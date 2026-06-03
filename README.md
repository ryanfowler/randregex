# randregex

`randregex` is a Go library for generating pseudo-random strings that match
regular expressions. It is intended for test data, identifiers, fixtures,
property-style checks, and other workflows where a compact regexp is a clearer
way to describe valid sample strings than handwritten generation code.

## Install

```sh
go get github.com/ryanfowler/randregex
```

## Quick Start

```go
package main

import (
	"fmt"
	"log"

	"github.com/ryanfowler/randregex"
)

func main() {
	g, err := randregex.Compile(`[a-z]{8}\d{2}`, randregex.DefaultMaxRepeat)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(g.String())
}
```

## API Overview

The public API is intentionally small:

- `Compile(pattern string, maxRepeat int) (*Generator, error)` parses and
  validates a regexp pattern.
- `MustCompile(pattern string, maxRepeat int) *Generator` is suitable for
  package-level generators and panics on invalid input.
- `FromRegexp(re *syntax.Regexp, maxRepeat int) (*Generator, error)` compiles
  an existing `regexp/syntax.Regexp` without mutating it.
- `(*Generator).String() string` returns a generated string using the default
  pseudo-random source.
- `(*Generator).StringWithRand(r Rand) string` uses a caller-provided random
  source.
- `(*Generator).Append(dst []byte) []byte` appends generated output to a buffer.
- `(*Generator).AppendWithRand(dst []byte, r Rand) []byte` combines buffer reuse
  with a caller-provided random source.
- `CryptoRand` is a `Rand` value backed by Go's `crypto/rand` source.

`DefaultMaxRepeat` is the recommended bound for unbounded repetitions:

```go
const DefaultMaxRepeat = 32
```

The main API keeps this policy explicit:

```go
g, err := randregex.Compile(pattern, randregex.DefaultMaxRepeat)
```

## Compile Once, Reuse Often

Compile patterns once and reuse the generator:

```go
var userID = randregex.MustCompile(`user-[a-z0-9]{12}`, randregex.DefaultMaxRepeat)

func newUserID() string {
	return userID.String()
}
```

`*Generator` is immutable after construction and safe for concurrent use.

## Reproducible Output

Use `StringWithRand` or `AppendWithRand` with any value that satisfies:

```go
type Rand interface {
	IntN(n int) int
}
```

This interface is satisfied by `*math/rand/v2.Rand`:

```go
r := rand.New(rand.NewPCG(1, 2))
g := randregex.MustCompile(`[a-z]{8}`, randregex.DefaultMaxRepeat)

fmt.Println(g.StringWithRand(r))
```

If a `Rand` value is shared across goroutines, the `Rand` implementation must
provide its own synchronization.

## Cryptographic Randomness

For security-sensitive output, pass `CryptoRand` to `StringWithRand` or
`AppendWithRand`:

```go
g := randregex.MustCompile(`[a-zA-Z0-9_-]{32}`, randregex.DefaultMaxRepeat)

token := g.StringWithRand(randregex.CryptoRand)
```

`CryptoRand` uses `crypto/rand.Reader` and panics if the system cryptographic
source fails.

## Buffer Reuse

`Append` and `AppendWithRand` are the allocation-conscious APIs:

```go
g := randregex.MustCompile(`[a-zA-Z0-9_-]{24}`, randregex.DefaultMaxRepeat)
buf := make([]byte, 0, 64)

for range 1000 {
	buf = buf[:0]
	buf = g.Append(buf)
	use(buf)
}
```

For common ASCII patterns, `AppendWithRand` allocates zero times when the
provided buffer has enough capacity.

## Supported Syntax

Patterns are parsed with Go's `regexp/syntax` package using `syntax.Perl`.

Supported:

- Empty expressions
- Literal strings and escaped literal characters
- Literal Unicode characters
- Concatenation
- Alternation, such as `foo|bar`
- Capturing and non-capturing groups
- Character classes, such as `[a-z]`, `[abc]`, and `[a-zA-Z0-9_]`
- Predefined ASCII classes: `\d`, `\D`, `\w`, `\W`, `\s`, `\S`
- Repetition: `?`, `*`, `+`, `{n}`, `{n,m}`, `{n,}`
- Dot `.`
- Anchors as zero-width nodes: `^`, `$`, `\A`, `\z`, `\b`, `\B`

Unsupported expressions return compile-time errors. Go's regexp syntax does not
support lookaround or backreferences, so `randregex` does not either.

Word-boundary assertions are accepted only when the adjacent generated
characters make the assertion guaranteed. For example, `\b[a-z]{4}\b` is valid,
while `a?\b` is rejected because one random branch would violate the assertion.

## Repetition Bounds

`maxRepeat` controls unbounded repetitions:

- `a*` generates 0 through `maxRepeat` repetitions.
- `a+` generates 1 through `maxRepeat` repetitions, or exactly 1 when
  `maxRepeat` is 0.
- `a{3,}` generates 3 through `maxRepeat` repetitions when `maxRepeat > 3`.
- If the minimum is greater than or equal to `maxRepeat`, an unbounded repeat
  generates exactly the minimum.

`maxRepeat` must be greater than or equal to zero.

## Character Generation

Character generation is intentionally ASCII-first for performance,
predictability, and testability.

- Literal Unicode characters are supported and emitted literally.
- ASCII character classes are sampled directly.
- Dot `.` samples from printable ASCII, from space through tilde.
- Negated and very broad character classes sample from printable ASCII after
  applying the class.
- `\d` is `[0-9]`.
- `\w` is `[0-9A-Za-z_]`.
- `\s` is tab, newline, vertical tab, form feed, carriage return, and space.

Full Unicode character-class sampling is intentionally out of scope. Use
Unicode literals outside character classes when exact Unicode characters are
needed.

## Randomness and Security

The default methods use Go's pseudo-random `math/rand/v2` default source.
Generated strings are not cryptographic secrets.

For reproducible output, pass a seeded `math/rand/v2.Rand`. For
security-sensitive use, pass `randregex.CryptoRand`.

`randregex` samples choices locally at each regexp node. It does not attempt to
provide a uniform distribution over all strings accepted by a pattern.

## Performance Characteristics

`randregex` compiles regular expressions into an immutable internal generator
tree. Generation does not re-parse patterns or walk `regexp/syntax` trees.

The implementation:

- Appends directly into caller-provided buffers.
- Precomputes sampleable character sets.
- Chooses alternation branches without generating unused branches.
- Generates repetition counts once per repeat node.
- Avoids reflection and external runtime dependencies.

Benchmarks are included in `randregex_benchmark_test.go` and can be run with:

```sh
go test -bench=. -benchmem ./...
```

## Error Handling Model

Invalid patterns, unsupported regexp nodes, unsafe word-boundary assertions, and
unsampleable character classes are rejected during compilation. Generation
methods assume a valid compiled generator and therefore return only generated
output.

This design makes errors explicit at setup time and keeps hot-path generation
simple.
