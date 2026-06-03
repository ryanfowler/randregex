// Package randregex generates pseudo-random strings that match regular
// expressions.
//
// It is intended for test data, identifiers, fixtures, property-style checks,
// and other workflows where a regexp is a compact description of valid sample
// strings. Patterns are parsed with regexp/syntax using syntax.Perl.
//
// Generator values are immutable and safe for concurrent use. The String and
// Append methods use the default pseudo-random source. StringWithRand and
// AppendWithRand let callers provide any Rand implementation, including a
// seeded *math/rand/v2.Rand for reproducible output. If a Rand is shared across
// goroutines, the Rand implementation must provide its own synchronization.
//
// Compile and FromRegexp parse, validate, and convert patterns into an
// immutable internal representation. Use MustCompile for package-level
// generators when invalid patterns should be treated as programmer errors.
//
// Generated strings are not cryptographic secrets unless callers provide a Rand
// implementation backed by an appropriate cryptographic source, such as
// CryptoRand.
//
// Character generation is ASCII-first. Literal Unicode characters are emitted
// as literals, but dot and negated or very broad character classes sample from
// printable ASCII, from space through tilde. The predefined Perl classes \d,
// \w, and \s use conventional ASCII definitions. Full Unicode character-class
// sampling and regex features unsupported by Go's regexp engine, such as
// lookaround and backreferences, are out of scope.
package randregex
