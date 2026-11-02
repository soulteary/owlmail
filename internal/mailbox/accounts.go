package mailbox

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
	"golang.org/x/crypto/bcrypt"
)

// Account configuration is loaded once at startup. There is no password or
// identity fallback when this file cannot be read or validated.
type Account struct {
	ID             string   `yaml:"id"`
	Username       string   `yaml:"username"`
	PasswordHash   string   `yaml:"password-hash"`
	Namespace      string   `yaml:"namespace"`
	AllowedDomains []string `yaml:"allowed-domains"`
	Enabled        bool     `yaml:"enabled"`
}
type accountDocument struct {
	Version  int       `yaml:"version"`
	Accounts []Account `yaml:"accounts"`
}
type Accounts struct {
	byUser map[string]Account
	dummy  []byte
}

const maxAccountsBytes = 1 << 20

func LoadAccounts(path string, namespace string, allowedDomains []string) (*Accounts, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open system SMTP accounts: %w", err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxAccountsBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAccountsBytes {
		return nil, errors.New("system SMTP accounts file is too large")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var doc accountDocument
	if err = dec.Decode(&doc); err != nil {
		return nil, errors.New("invalid system SMTP accounts document")
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("system SMTP accounts must contain exactly one document")
	}
	if doc.Version != 1 || len(doc.Accounts) == 0 || len(doc.Accounts) > 100 {
		return nil, errors.New("invalid system SMTP accounts version or count")
	}
	allowed := map[string]bool{}
	for _, d := range allowedDomains {
		normalized, err := normalizeDomain(d)
		if err != nil {
			return nil, err
		}
		allowed[normalized] = true
	}
	result := &Accounts{byUser: map[string]Account{}}
	ids := map[string]bool{}
	for _, a := range doc.Accounts {
		if !validIdentity(namespace, a.ID) || !validIdentity(namespace, a.Username) || a.Namespace != namespace || ids[a.ID] {
			return nil, errors.New("invalid or duplicate system SMTP account identity")
		}
		if _, exists := result.byUser[a.Username]; exists {
			return nil, errors.New("duplicate system SMTP username")
		}
		cost, err := bcrypt.Cost([]byte(a.PasswordHash))
		if err != nil || cost < 10 || cost > 14 {
			return nil, errors.New("system SMTP account requires bcrypt cost 10..14")
		}
		if len(a.AllowedDomains) == 0 {
			return nil, errors.New("system SMTP account requires allowed domains")
		}
		normalized := make([]string, 0, len(a.AllowedDomains))
		for _, d := range a.AllowedDomains {
			domain, err := normalizeDomain(d)
			if err != nil || !allowed[domain] {
				return nil, errors.New("account domain must be allowed by instance policy")
			}
			normalized = append(normalized, domain)
		}
		a.AllowedDomains = normalized
		result.byUser[a.Username] = a
		ids[a.ID] = true
	}
	// Unknown users still incur bcrypt work. Keep the dummy at the minimum
	// accepted cost; per-connection/rate limits must bound online attempts.
	result.dummy, err = bcrypt.GenerateFromPassword([]byte("owlmail-invalid-account"), 10)
	if err != nil {
		return nil, err
	}
	return result, nil
}
func normalizeDomain(domain string) (string, error) {
	address, err := NormalizeAddress("route@" + domain)
	if err != nil {
		return "", errors.New("invalid allowed recipient domain")
	}
	return strings.TrimPrefix(address, "route@"), nil
}
func (a *Accounts) Authenticate(username, password string) (Account, error) {
	if a == nil {
		return Account{}, errors.New("SMTP authentication failed")
	}
	account, exists := a.byUser[username]
	hash := a.dummy
	if exists {
		hash = []byte(account.PasswordHash)
	}
	err := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil || !exists || !account.Enabled {
		return Account{}, errors.New("SMTP authentication failed")
	}
	// Do not propagate a password verifier into session or logs.
	account.PasswordHash = ""
	return account, nil
}
func (a Account) Allows(address string) bool {
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return false
	}
	_, domain, _ := strings.Cut(normalized, "@")
	for _, allowed := range a.AllowedDomains {
		if domain == allowed {
			return true
		}
	}
	return false
}
