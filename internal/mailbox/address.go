// Package mailbox owns identities and recipient routes. It never authenticates users.
package mailbox

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
)

// NormalizeAddress is deliberately conservative: domains are case-insensitive,
// local parts are exact, and plus tags are never stripped.
func NormalizeAddress(value string) (string, error) {
	if value == "" || len(value) > 254 || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n\x00") {
		return "", errors.New("invalid recipient address")
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Name != "" || parsed.Address != value {
		return "", errors.New("recipient must be a bare mailbox address")
	}
	at := strings.LastIndexByte(value, '@')
	if at < 1 || at == len(value)-1 {
		return "", errors.New("invalid recipient address")
	}
	local, domain := value[:at], value[at+1:]
	// Restrict this first version to unquoted ASCII SMTP mailboxes; SMTPUTF8 and
	// quoted local parts require an explicit normalization migration contract.
	if len(local) > 64 || strings.ContainsAny(local, "\"\\ ()<>[]:;,") {
		return "", errors.New("unsupported local part")
	}
	for _, c := range local {
		if c > unicode.MaxASCII || unicode.IsControl(c) {
			return "", errors.New("unsupported local part")
		}
	}
	ascii, err := idna.Lookup.ToASCII(domain)
	if err != nil || strings.HasSuffix(ascii, ".") || len(ascii) > 253 {
		return "", errors.New("invalid recipient domain")
	}
	for _, label := range strings.Split(ascii, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid recipient domain")
		}
		for _, c := range label {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
				return "", errors.New("invalid recipient domain")
			}
		}
	}
	return local + "@" + strings.ToLower(ascii), nil
}
