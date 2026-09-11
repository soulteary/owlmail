package common

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
)

// CredentialVerifier compares a presented credential against a configured one
// in time that does not depend on the configured value. Go's own string
// comparison is not safe for a secret: it returns as soon as two bytes differ,
// so the time a rejection takes grows with the length of the shared prefix,
// and an attacker who can measure it recovers the secret one byte at a time
// instead of guessing it whole.
//
// Three separate leaks have to be closed, and closing only the obvious one
// leaves a working oracle:
//
//   - The prefix leak. subtle.ConstantTimeCompare reads both operands to the
//     end regardless of where they differ.
//   - The length leak. subtle.ConstantTimeCompare does not close this one: it
//     returns immediately when the lengths differ, so a wrong-length guess is
//     still measurably faster and the secret's length stays readable. Hashing
//     to a fixed-size tag first removes it, because every candidate -- one byte
//     or one megabyte -- is compared as the same 32 bytes.
//   - The short-circuit leak. Rejecting on the username before looking at the
//     password lets the two secrets be attacked one after the other rather than
//     jointly. Both tags are therefore always computed and always compared, and
//     the two results are combined with a bitwise & rather than &&.
//
// The tag is keyed with a random value drawn once at startup, so an attacker
// cannot precompute the tag of a guess and mount the same search against the
// tags themselves. The expected tags are computed once at construction, so
// per-request work does not depend on the configured username or password
// length either.
type CredentialVerifier struct {
	key                 [sha256.Size]byte
	expectedUsernameTag [sha256.Size]byte
	expectedPasswordTag [sha256.Size]byte
}

// NewCredentialVerifier binds a verifier to the configured credential pair.
func NewCredentialVerifier(username, password string) (*CredentialVerifier, error) {
	verifier := &CredentialVerifier{}
	if _, err := rand.Read(verifier.key[:]); err != nil {
		return nil, err
	}
	verifier.expectedUsernameTag = verifier.tag(username)
	verifier.expectedPasswordTag = verifier.tag(password)
	return verifier, nil
}

// CredentialsEqual reports whether both presented values match the configured
// pair. It does the same work for every input.
func (v *CredentialVerifier) CredentialsEqual(username, password string) bool {
	usernameTag := v.tag(username)
	passwordTag := v.tag(password)
	usernameMatches := subtle.ConstantTimeCompare(usernameTag[:], v.expectedUsernameTag[:])
	passwordMatches := subtle.ConstantTimeCompare(passwordTag[:], v.expectedPasswordTag[:])
	return usernameMatches&passwordMatches == 1
}

// StringsEqual compares two values that are not the configured credentials --
// an SMTP authorization identity, say -- under the same tag construction, so a
// differing length is not distinguishable from a differing first byte.
func (v *CredentialVerifier) StringsEqual(value, expected string) bool {
	valueTag := v.tag(value)
	expectedTag := v.tag(expected)
	return subtle.ConstantTimeCompare(valueTag[:], expectedTag[:]) == 1
}

func (v *CredentialVerifier) tag(value string) [sha256.Size]byte {
	mac := hmac.New(sha256.New, v.key[:])
	_, _ = mac.Write([]byte(value))
	var tag [sha256.Size]byte
	mac.Sum(tag[:0])
	return tag
}
