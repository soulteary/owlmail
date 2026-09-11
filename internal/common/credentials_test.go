package common

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"
)

func TestCredentialVerifier(t *testing.T) {
	verifier, err := NewCredentialVerifier("user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	if !verifier.CredentialsEqual("user", "pass") {
		t.Fatal("matching credentials were rejected")
	}
	for _, test := range []struct {
		username string
		password string
	}{
		{username: "uses", password: "pass"},
		{username: "other", password: "pass"},
		{username: "user", password: "fail"},
		{username: "user", password: "other"},
		{username: "", password: ""},
		// A prefix of either secret is a distinct credential, not a partial
		// match: nothing in the comparison stops early on the shared bytes.
		{username: "use", password: "pass"},
		{username: "user", password: "pas"},
		{username: "userr", password: "passs"},
	} {
		if verifier.CredentialsEqual(test.username, test.password) {
			t.Fatalf("credentials %q/%q unexpectedly matched", test.username, test.password)
		}
	}

	shortTag := verifier.tag("x")
	longTag := verifier.tag(strings.Repeat("x", 1024))
	if len(shortTag) != sha256.Size || len(longTag) != sha256.Size {
		t.Fatalf("credential tag lengths = %d/%d, want %d", len(shortTag), len(longTag), sha256.Size)
	}
	if bytes.Equal(shortTag[:], longTag[:]) {
		t.Fatal("different credentials produced matching tags")
	}
	if !verifier.StringsEqual("identity", "identity") {
		t.Fatal("matching authorization identities were rejected")
	}
	if verifier.StringsEqual("same", "different-length") {
		t.Fatal("different authorization identities unexpectedly matched")
	}
}

// TestCredentialVerifierNormalizesLength pins the property that closes the
// length leak. subtle.ConstantTimeCompare returns early when its operands
// differ in length, so comparing credentials directly would still time a
// wrong-length guess differently from a same-length one. Every candidate has to
// reach the comparison as the same number of bytes -- which the [sha256.Size]byte
// return type enforces -- and still be told apart, which is what this checks.
func TestCredentialVerifierNormalizesLength(t *testing.T) {
	verifier, err := NewCredentialVerifier("user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, length := range []int{0, 1, 3, 4, 5, 64, 4096} {
		candidate := strings.Repeat("a", length)
		tag := verifier.tag(candidate)
		if previous, duplicate := seen[string(tag[:])]; duplicate {
			t.Fatalf("values of length %d and %d produced the same tag", previous, length)
		}
		seen[string(tag[:])] = length
		if verifier.CredentialsEqual("user", candidate) {
			t.Fatalf("a %d-byte password was accepted", length)
		}
	}
}

// TestCredentialVerifierKeyIsPerInstance guards the tag key against being
// reduced to a plain hash. With a fixed key an attacker can compute the tag of
// a guess offline, which puts the search back within reach of the same
// byte-at-a-time strategy the tags exist to prevent.
func TestCredentialVerifierKeyIsPerInstance(t *testing.T) {
	first, err := NewCredentialVerifier("user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewCredentialVerifier("user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	if first.key == second.key {
		t.Fatal("two verifiers drew the same tag key")
	}
	firstTag := first.tag("user")
	secondTag := second.tag("user")
	if bytes.Equal(firstTag[:], secondTag[:]) {
		t.Fatal("the same value tagged identically under two keys")
	}
	// Both still accept the credentials they were built from.
	if !first.CredentialsEqual("user", "pass") || !second.CredentialsEqual("user", "pass") {
		t.Fatal("a verifier rejected the credentials it was constructed with")
	}
}

// TestCredentialVerifierChecksBothFields covers the short-circuit leak: a
// mismatched username must not spare the password comparison, so that the two
// secrets cannot be attacked one after the other.
func TestCredentialVerifierChecksBothFields(t *testing.T) {
	verifier, err := NewCredentialVerifier("user", "pass")
	if err != nil {
		t.Fatal(err)
	}
	if verifier.CredentialsEqual("wrong", "pass") {
		t.Fatal("a wrong username was accepted")
	}
	if verifier.CredentialsEqual("user", "wrong") {
		t.Fatal("a wrong password was accepted")
	}
	if verifier.CredentialsEqual("wrong", "wrong") {
		t.Fatal("wrong credentials were accepted")
	}
}
