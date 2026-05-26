// Package randregex generates pseudo-random strings that match regular
// expressions.
//
// The package is compile-first: parsing, validation, and conversion to an
// immutable internal representation happen in Compile or FromRegexp. Once a
// Generator has been created successfully, its generation methods do not return
// errors.
//
// Generated strings are pseudo-random and are not suitable for secrets unless
// callers provide an appropriate Rand implementation. Generator values are safe
// for concurrent use. If a Rand is shared across goroutines, the Rand
// implementation must provide its own synchronization.
//
// Character generation is ASCII-first. Literal Unicode characters are emitted
// as literals, but dot and negated or very broad character classes sample from
// printable ASCII, from space through tilde. The predefined Perl classes \d,
// \w, and \s use conventional ASCII definitions.
package randregex
