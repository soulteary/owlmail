package webhook

import (
	"path"
	"runtime"
	"strings"
	"testing"
)

// The matcher backtracks in O(pattern x value), so unbounded fuzzer-grown
// inputs would spend the whole budget on length instead of on grammar shapes,
// and the seed corpus would stop being a fast unit test. Oversized inputs are
// truncated rather than skipped so no generated shape is thrown away.
const (
	fuzzPatternLimit = 128
	fuzzValueLimit   = 1024
)

// FuzzMatchTextPattern exercises the only pattern engine in this tree that is
// written by hand rather than taken from the standard library. Both of its
// inputs are untrusted: the value is a subject line or address straight off
// the wire, and a pattern reaches it from configuration that an operator may
// have pasted out of a captured message.
//
// matchTextPattern is path.Match's algorithm with the separator role removed,
// which makes path.Match a genuine oracle rather than a restatement of the
// code under test: when neither input holds a slash, every separator-specific
// branch in the standard library is inert, so the two must agree on the
// verdict and on whether the pattern is well-formed. A divergence is the copy
// drifting from the grammar it claims to implement, and the visible
// consequence is a webhook target that silently stops forwarding the mail an
// operator configured it to match.
func FuzzMatchTextPattern(f *testing.F) {
	seeds := []struct {
		pattern string
		value   string
	}{
		{pattern: "*code*", value: "visit https://example.com/code"},
		{pattern: "https://*/code", value: "https://example.com/code"},
		{pattern: "https://example.com/*", value: "https://example.com/a/b"},
		{pattern: "https://example.com/?", value: "https://example.com/a/b"},
		{pattern: "[.-0]", value: "/"},
		{pattern: "[^.-0]", value: "/"},
		{pattern: "[-]", value: ""},
		{pattern: "", value: ""},
		{pattern: "", value: "nonempty"},
		{pattern: "*", value: ""},
		// Unterminated constructs: the escape and the character class are the
		// two places where the scanner and the matcher have to agree on where
		// a chunk ends.
		{pattern: `\`, value: `\`},
		{pattern: `a\`, value: "a"},
		{pattern: "[", value: "["},
		{pattern: "[]", value: "]"},
		{pattern: "[^", value: "a"},
		{pattern: "[a-", value: "a"},
		{pattern: "[-]", value: "-"},
		{pattern: "[a-]", value: "a"},
		{pattern: "**********a", value: strings.Repeat("a", 64)},
		{pattern: "*a*a*a*b", value: strings.Repeat("a", 64)},
		{pattern: "\xff", value: "\xff"},
		{pattern: "[\xff]", value: "\xff"},
		{pattern: "user@example.com", value: "USER@EXAMPLE.COM"},
	}
	for _, seed := range seeds {
		f.Add(seed.pattern, seed.value)
	}

	f.Fuzz(func(t *testing.T, pattern, value string) {
		if len(pattern) > fuzzPatternLimit {
			pattern = pattern[:fuzzPatternLimit]
		}
		if len(value) > fuzzValueLimit {
			value = value[:fuzzValueLimit]
		}

		matched, err := matchTextPattern(pattern, value)

		repeated, repeatedErr := matchTextPattern(pattern, value)
		if matched != repeated || (err == nil) != (repeatedErr == nil) {
			t.Fatalf("matchTextPattern(%q, %q) is not deterministic: (%v, %v) then (%v, %v)",
				pattern, value, matched, err, repeated, repeatedErr)
		}

		// A rejected pattern must not also be reported as matching: matchesField
		// discards the error and keeps only the boolean, so a true alongside an
		// error would deliver mail to a target whose pattern is unusable.
		if err != nil && matched {
			t.Fatalf("matchTextPattern(%q, %q) reported a match alongside %v", pattern, value, err)
		}

		// A pattern carrying no metacharacter is a literal. This holds without
		// reference to path.Match, so it still covers inputs the differential
		// check below has to leave alone.
		if !strings.ContainsAny(pattern, `*?[\`) {
			if err != nil {
				t.Fatalf("matchTextPattern(%q, %q) rejected a literal pattern: %v", pattern, value, err)
			}
			if matched != (pattern == value) {
				t.Fatalf("matchTextPattern(%q, %q) = %v, want %v for a literal pattern", pattern, value, matched, pattern == value)
			}
		}

		// path.Match drops backslash escaping entirely on Windows while this
		// matcher honors it on every platform, so the oracle only holds
		// elsewhere. A slash in either input reaches the separator handling
		// that this matcher deliberately does not have.
		if runtime.GOOS == "windows" || strings.Contains(pattern, "/") || strings.Contains(value, "/") {
			return
		}
		wantMatched, wantErr := path.Match(pattern, value)
		if matched != wantMatched || (err == nil) != (wantErr == nil) {
			t.Fatalf("matchTextPattern(%q, %q) = (%v, %v); path.Match = (%v, %v)",
				pattern, value, matched, err, wantMatched, wantErr)
		}
	})
}
