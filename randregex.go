package randregex

import (
	"fmt"
	"math/rand/v2"
	"regexp/syntax"
	"strings"
)

// DefaultMaxRepeat is the recommended upper bound for unbounded repetitions.
//
// It is used for patterns such as a*, a+, and a{3,}. The primary APIs remain
// explicit, so callers pass this value as Compile(pattern, DefaultMaxRepeat).
const DefaultMaxRepeat = 32

// Rand is the random-number interface used by Generator.
//
// It is satisfied by *math/rand/v2.Rand. The package-level CryptoRand value
// provides a cryptographic implementation. Implementations must return a value
// in [0, n), and may panic when n <= 0. If a Rand is shared concurrently, the
// Rand implementation is responsible for synchronization.
type Rand interface {
	IntN(n int) int
}

// Generator is an immutable compiled regular-expression string generator.
//
// Generator values are safe for concurrent use. Generation methods using an
// explicit Rand require the caller's Rand to be safe when shared concurrently.
type Generator struct {
	root node
}

// Compile parses pattern using regexp/syntax.Perl and compiles it into a
// Generator.
//
// maxRepeat controls the maximum used for unbounded repetitions. It must be
// non-negative. For a{n,}, the upper bound is maxRepeat when maxRepeat > n;
// otherwise generation emits exactly n repetitions.
func Compile(pattern string, maxRepeat int) (*Generator, error) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil, err
	}
	return FromRegexp(re, maxRepeat)
}

// FromRegexp compiles re into a Generator without mutating re.
//
// maxRepeat must be non-negative and controls unbounded repetitions as
// described by Compile. Passing an already simplified regexp is supported, but
// regexp/syntax.Simplify may rewrite counted unbounded repetitions such as
// a{3,} into forms that no longer preserve the original minimum for randregex's
// maxRepeat policy.
func FromRegexp(re *syntax.Regexp, maxRepeat int) (*Generator, error) {
	if re == nil {
		return nil, fmt.Errorf("randregex: regexp must not be nil")
	}
	if maxRepeat < 0 {
		return nil, fmt.Errorf("randregex: maxRepeat must be >= 0")
	}
	if _, err := validateRootZeroWidth(re); err != nil {
		return nil, err
	}

	root, err := compileNode(re, maxRepeat)
	if err != nil {
		return nil, err
	}
	return &Generator{root: root}, nil
}

// MustCompile is like Compile but panics if pattern cannot be compiled.
func MustCompile(pattern string, maxRepeat int) *Generator {
	g, err := Compile(pattern, maxRepeat)
	if err != nil {
		panic(err)
	}
	return g
}

// String generates a pseudo-random string matching the compiled regexp.
func (g *Generator) String() string {
	return g.StringWithRand(defaultRand{})
}

// Append appends a pseudo-random string matching the compiled regexp to dst and
// returns the extended buffer.
func (g *Generator) Append(dst []byte) []byte {
	return g.AppendWithRand(dst, defaultRand{})
}

// StringWithRand generates a string matching the compiled regexp using r.
//
// If r is shared concurrently, it must provide its own synchronization.
func (g *Generator) StringWithRand(r Rand) string {
	return string(g.AppendWithRand(nil, r))
}

// AppendWithRand appends a generated string matching the compiled regexp to dst
// using r, and returns the extended buffer.
//
// This is the lowest-allocation public API. If dst has sufficient capacity, it
// does not allocate for common ASCII patterns. If r is shared concurrently, it
// must provide its own synchronization.
func (g *Generator) AppendWithRand(dst []byte, r Rand) []byte {
	return g.root.append(dst, r)
}

type defaultRand struct{}

func (defaultRand) IntN(n int) int {
	return rand.IntN(n)
}

type node interface {
	append(dst []byte, r Rand) []byte
}

type emptyNode struct{}

func (emptyNode) append(dst []byte, _ Rand) []byte {
	return dst
}

type literalNode struct {
	s string
}

func (n literalNode) append(dst []byte, _ Rand) []byte {
	return append(dst, n.s...)
}

type concatNode struct {
	nodes []node
}

func (n concatNode) append(dst []byte, r Rand) []byte {
	for _, child := range n.nodes {
		dst = child.append(dst, r)
	}
	return dst
}

type alternateNode struct {
	nodes []node
}

func (n alternateNode) append(dst []byte, r Rand) []byte {
	return n.nodes[r.IntN(len(n.nodes))].append(dst, r)
}

type repeatNode struct {
	child node
	min   int
	max   int
}

func (n repeatNode) append(dst []byte, r Rand) []byte {
	count := n.min
	if n.max > n.min {
		count += r.IntN(n.max - n.min + 1)
	}
	for range count {
		dst = n.child.append(dst, r)
	}
	return dst
}

type charClassNode struct {
	chars string
}

func (n charClassNode) append(dst []byte, r Rand) []byte {
	return append(dst, n.chars[r.IntN(len(n.chars))])
}

var (
	printableASCII = asciiRange(' ', '~')
	dotChars       = printableASCII
)

func compileNode(re *syntax.Regexp, maxRepeat int) (node, error) {
	switch re.Op {
	case syntax.OpNoMatch:
		return nil, fmt.Errorf("randregex: unsupported regexp node %s: expression matches no strings", re.Op)
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return emptyNode{}, nil
	case syntax.OpLiteral:
		return literalNode{s: string(re.Rune)}, nil
	case syntax.OpCharClass:
		return compileCharClass(re.Rune)
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		return charClassNode{chars: dotChars}, nil
	case syntax.OpCapture:
		if len(re.Sub) != 1 {
			return nil, fmt.Errorf("randregex: invalid capture node with %d children", len(re.Sub))
		}
		return compileNode(re.Sub[0], maxRepeat)
	case syntax.OpStar:
		return compileRepeat(re, 0, maxRepeat, maxRepeat)
	case syntax.OpPlus:
		upper := maxRepeat
		if upper < 1 {
			upper = 1
		}
		return compileRepeat(re, 1, upper, maxRepeat)
	case syntax.OpQuest:
		return compileRepeat(re, 0, 1, maxRepeat)
	case syntax.OpRepeat:
		min, max := re.Min, re.Max
		if max < 0 {
			max = min
			if maxRepeat > min {
				max = maxRepeat
			}
		}
		return compileRepeat(re, min, max, maxRepeat)
	case syntax.OpConcat:
		return compileList(re.Sub, maxRepeat, false)
	case syntax.OpAlternate:
		return compileList(re.Sub, maxRepeat, true)
	default:
		return nil, fmt.Errorf("randregex: unsupported regexp node %s", re.Op)
	}
}

func compileList(subs []*syntax.Regexp, maxRepeat int, alternate bool) (node, error) {
	if len(subs) == 0 {
		return emptyNode{}, nil
	}

	nodes := make([]node, 0, len(subs))
	for _, sub := range subs {
		child, err := compileNode(sub, maxRepeat)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, child)
	}

	if len(nodes) == 1 {
		return nodes[0], nil
	}
	if alternate {
		return alternateNode{nodes: nodes}, nil
	}
	return concatNode{nodes: nodes}, nil
}

func compileRepeat(re *syntax.Regexp, min, max, maxRepeat int) (node, error) {
	if len(re.Sub) != 1 {
		return nil, fmt.Errorf("randregex: invalid repeat node %s with %d children", re.Op, len(re.Sub))
	}
	if min < 0 || max < min {
		return nil, fmt.Errorf("randregex: invalid repeat bounds {%d,%d}", min, max)
	}

	child, err := compileNode(re.Sub[0], maxRepeat)
	if err != nil {
		return nil, err
	}
	if min == 0 && max == 0 {
		return emptyNode{}, nil
	}
	if min == 1 && max == 1 {
		return child, nil
	}
	return repeatNode{child: child, min: min, max: max}, nil
}

func compileCharClass(ranges []rune) (node, error) {
	chars, err := sampleableChars(ranges)
	if err != nil {
		return nil, err
	}
	return charClassNode{chars: chars}, nil
}

func byteClassDomain(ranges []rune) string {
	for i := 0; i+1 < len(ranges); i += 2 {
		if ranges[i] < 0 || ranges[i+1] > 127 {
			return printableASCII
		}
	}
	return asciiRange(0, 127)
}

func runeInClass(r rune, ranges []rune) bool {
	for i := 0; i+1 < len(ranges); i += 2 {
		if ranges[i] <= r && r <= ranges[i+1] {
			return true
		}
	}
	return false
}

func asciiRange(lo, hi byte) string {
	var b strings.Builder
	b.Grow(int(hi - lo + 1))
	for c := lo; c <= hi; c++ {
		b.WriteByte(c)
		if c == hi {
			break
		}
	}
	return b.String()
}

const (
	charWord uint8 = 1 << iota
	charNonWord
)

type regexpProps struct {
	canEmpty bool
	first    uint8
	last     uint8
}

func validateRootZeroWidth(re *syntax.Regexp) (regexpProps, error) {
	if re.Op == syntax.OpWordBoundary || re.Op == syntax.OpNoWordBoundary {
		if !boundaryGuaranteed(charNonWord, charNonWord, re.Op == syntax.OpWordBoundary) {
			return regexpProps{}, fmt.Errorf("randregex: %s assertion is not guaranteed by adjacent generated characters", re.Op)
		}
		return regexpProps{canEmpty: true}, nil
	}
	return validateZeroWidth(re)
}

func validateZeroWidth(re *syntax.Regexp) (regexpProps, error) {
	switch re.Op {
	case syntax.OpNoMatch:
		return regexpProps{}, nil
	case syntax.OpEmptyMatch, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText:
		return regexpProps{canEmpty: true}, nil
	case syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return regexpProps{}, fmt.Errorf("randregex: %s assertion is not guaranteed by adjacent generated characters", re.Op)
	case syntax.OpLiteral:
		if len(re.Rune) == 0 {
			return regexpProps{canEmpty: true}, nil
		}
		return regexpProps{
			first: charKind(re.Rune[0]),
			last:  charKind(re.Rune[len(re.Rune)-1]),
		}, nil
	case syntax.OpCharClass:
		chars, err := sampleableChars(re.Rune)
		if err != nil {
			return regexpProps{}, nil
		}
		return charStringProps(chars), nil
	case syntax.OpAnyCharNotNL, syntax.OpAnyChar:
		return charStringProps(dotChars), nil
	case syntax.OpCapture:
		if len(re.Sub) != 1 {
			return regexpProps{}, nil
		}
		return validateZeroWidth(re.Sub[0])
	case syntax.OpStar, syntax.OpQuest:
		if len(re.Sub) != 1 {
			return regexpProps{}, nil
		}
		p, err := validateZeroWidth(re.Sub[0])
		if err != nil {
			return regexpProps{}, err
		}
		p.canEmpty = true
		return p, nil
	case syntax.OpPlus:
		if len(re.Sub) != 1 {
			return regexpProps{}, nil
		}
		return validateZeroWidth(re.Sub[0])
	case syntax.OpRepeat:
		if len(re.Sub) != 1 {
			return regexpProps{}, nil
		}
		p, err := validateZeroWidth(re.Sub[0])
		if err != nil {
			return regexpProps{}, err
		}
		if re.Min == 0 {
			p.canEmpty = true
		}
		return p, nil
	case syntax.OpAlternate:
		return validateAlternate(re.Sub)
	case syntax.OpConcat:
		return validateConcat(re.Sub)
	default:
		return regexpProps{}, nil
	}
}

func validateAlternate(subs []*syntax.Regexp) (regexpProps, error) {
	if len(subs) == 0 {
		return regexpProps{canEmpty: true}, nil
	}
	var out regexpProps
	for _, sub := range subs {
		p, err := validateZeroWidth(sub)
		if err != nil {
			return regexpProps{}, err
		}
		out.canEmpty = out.canEmpty || p.canEmpty
		out.first |= p.first
		out.last |= p.last
	}
	return out, nil
}

func validateConcat(subs []*syntax.Regexp) (regexpProps, error) {
	if len(subs) == 0 {
		return regexpProps{canEmpty: true}, nil
	}

	props := make([]regexpProps, len(subs))
	for i, sub := range subs {
		if sub.Op == syntax.OpWordBoundary || sub.Op == syntax.OpNoWordBoundary {
			props[i] = regexpProps{canEmpty: true}
			continue
		}
		p, err := validateZeroWidth(sub)
		if err != nil {
			return regexpProps{}, err
		}
		props[i] = p
	}

	for i, sub := range subs {
		if sub.Op != syntax.OpWordBoundary && sub.Op != syntax.OpNoWordBoundary {
			continue
		}
		left := boundaryLeftKinds(props[:i])
		right := boundaryRightKinds(props[i+1:])
		if !boundaryGuaranteed(left, right, sub.Op == syntax.OpWordBoundary) {
			return regexpProps{}, fmt.Errorf("randregex: %s assertion is not guaranteed by adjacent generated characters", sub.Op)
		}
	}

	return concatProps(props), nil
}

func concatProps(props []regexpProps) regexpProps {
	out := regexpProps{canEmpty: true}

	prefixCanEmpty := true
	for _, p := range props {
		if prefixCanEmpty {
			out.first |= p.first
		}
		prefixCanEmpty = prefixCanEmpty && p.canEmpty
	}

	suffixCanEmpty := true
	for i := len(props) - 1; i >= 0; i-- {
		if suffixCanEmpty {
			out.last |= props[i].last
		}
		suffixCanEmpty = suffixCanEmpty && props[i].canEmpty
	}

	out.canEmpty = prefixCanEmpty
	return out
}

func boundaryLeftKinds(prefix []regexpProps) uint8 {
	kinds := uint8(0)
	canEmpty := true
	for i := len(prefix) - 1; i >= 0; i-- {
		if canEmpty {
			kinds |= prefix[i].last
		}
		canEmpty = canEmpty && prefix[i].canEmpty
	}
	if canEmpty {
		kinds |= charNonWord
	}
	return kinds
}

func boundaryRightKinds(suffix []regexpProps) uint8 {
	kinds := uint8(0)
	canEmpty := true
	for _, p := range suffix {
		if canEmpty {
			kinds |= p.first
		}
		canEmpty = canEmpty && p.canEmpty
	}
	if canEmpty {
		kinds |= charNonWord
	}
	return kinds
}

func boundaryGuaranteed(left, right uint8, wantBoundary bool) bool {
	for _, l := range []uint8{charWord, charNonWord} {
		if left&l == 0 {
			continue
		}
		for _, rr := range []uint8{charWord, charNonWord} {
			if right&rr == 0 {
				continue
			}
			isBoundary := l != rr
			if isBoundary != wantBoundary {
				return false
			}
		}
	}
	return true
}

func sampleableChars(ranges []rune) (string, error) {
	domain := byteClassDomain(ranges)
	chars := make([]byte, 0, len(domain))
	for i := range domain {
		if runeInClass(rune(domain[i]), ranges) {
			chars = append(chars, domain[i])
		}
	}
	if len(chars) == 0 {
		return "", fmt.Errorf("randregex: character class has no sampleable ASCII characters")
	}
	return string(chars), nil
}

func charStringProps(chars string) regexpProps {
	var kinds uint8
	for i := range chars {
		kinds |= charKind(rune(chars[i]))
	}
	return regexpProps{first: kinds, last: kinds}
}

func charKind(r rune) uint8 {
	if ('0' <= r && r <= '9') || ('A' <= r && r <= 'Z') || r == '_' || ('a' <= r && r <= 'z') {
		return charWord
	}
	return charNonWord
}
