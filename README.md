# randregex

`randregex` is a Go library for generating pseudo-random strings that match
regular expressions.

It is designed around one rule:

> Compile can fail. Generate cannot.

Parsing, validation, and conversion to an immutable generator happen up front.
Once you have a `*randregex.Generator`, generation methods do not return
errors.

## Install

```sh
go get github.com/ryanfowler/randregex
```

## Quick Start

```go
g, err := randregex.Compile(`[a-z]{8}\d{2}`, randregex.DefaultMaxRepeat)
if err != nil {
	log.Fatal(err)
}

fmt.Println(g.String())
```

## Precompiled Generator

Compile once and reuse the generator:

```go
var userID = randregex.MustCompile(`user-[a-z0-9]{12}`, randregex.DefaultMaxRepeat)

func newUserID() string {
	return userID.String()
}
```

`*Generator` is immutable after construction and safe for concurrent use.

## Deterministic Random

Use `StringWithRand` or `AppendWithRand` with any value that satisfies:

```go
type Rand interface {
	IntN(n int) int
}
```

This works with `math/rand/v2.Rand`:

```go
r := rand.New(rand.NewPCG(1, 2))

g := randregex.MustCompile(`[a-z]{8}`, randregex.DefaultMaxRepeat)

fmt.Println(g.StringWithRand(r))
```

If a `Rand` is shared across goroutines, the `Rand` implementation must be
safe for concurrent use.

## Append and Buffer Reuse

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

Unsupported expressions return clear compile-time errors. Go's regexp syntax
does not support lookaround or backreferences, so `randregex` does not either.
Word-boundary assertions are accepted when the adjacent generated characters
make the assertion guaranteed; patterns such as `a?\b` are rejected because one
random branch would violate the assertion.

## maxRepeat

`maxRepeat` controls unbounded repetitions:

- `a*` generates 0 through `maxRepeat` repetitions.
- `a+` generates 1 through `maxRepeat` repetitions, or exactly 1 when
  `maxRepeat` is 0.
- `a{3,}` generates 3 through `maxRepeat` repetitions when `maxRepeat > 3`.
- If the minimum is greater than or equal to `maxRepeat`, an unbounded repeat
  generates exactly the minimum.

`maxRepeat` must be greater than or equal to zero.

The package exposes:

```go
const DefaultMaxRepeat = 32
```

The main API stays explicit:

```go
g, err := randregex.Compile(pattern, randregex.DefaultMaxRepeat)
```

## ASCII-First Behavior

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

Full Unicode category sampling is intentionally out of scope for the first
version.

## Randomness and Security

The default methods use Go's pseudo-random `math/rand/v2` default source.
Generated strings are not cryptographic secrets.

For reproducible output, pass a seeded `math/rand/v2.Rand`. For security-sensitive
use, provide a `Rand` implementation backed by an appropriate cryptographic
source.

## Performance Notes

`randregex` compiles regular expressions into an immutable internal generator
tree. Generation does not re-parse patterns or walk `regexp/syntax` trees.

The implementation:

- Appends directly into caller-provided buffers.
- Precomputes sampleable character sets.
- Chooses alternation branches without generating unused branches.
- Generates repetition counts once per repeat node.
- Avoids reflection and required external runtime dependencies.
