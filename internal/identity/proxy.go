// Package identity validates identity assertions from a trusted reverse proxy.
package identity

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"unicode"
)

var ErrUntrusted = errors.New("trusted upstream identity required")

// Assertion is authenticated by the deployment's proxy trust boundary, not by
// email content. Verified must only be set by an upstream that verifies email ownership.
type Assertion struct {
	Namespace, Subject, Email string
	EmailVerified             bool
}
type Proxy struct {
	namespace                                  string
	peers                                      []netip.Prefix
	subjectHeader, emailHeader, verifiedHeader string
}

func NewProxy(namespace string, peers []string, subjectHeader, emailHeader, verifiedHeader string) (*Proxy, error) {
	if !validValue(namespace, 256) || len(peers) == 0 {
		return nil, errors.New("namespace and trusted proxies required")
	}
	headers := []string{subjectHeader, emailHeader, verifiedHeader}
	seen := map[string]bool{}
	for _, h := range headers {
		canonical := strings.ToLower(h)
		if h == "" || seen[canonical] {
			return nil, errors.New("distinct identity headers required")
		}
		for _, c := range h {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
				return nil, errors.New("invalid identity header")
			}
		}
		seen[canonical] = true
	}
	p := &Proxy{namespace: namespace, subjectHeader: subjectHeader, emailHeader: emailHeader, verifiedHeader: verifiedHeader}
	for _, peer := range peers {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(peer))
		if err != nil {
			return nil, errors.New("trusted proxies must be CIDR prefixes")
		}
		if prefix.Bits() == 0 {
			return nil, errors.New("unrestricted proxy trust is not allowed")
		}
		p.peers = append(p.peers, prefix.Masked())
	}
	return p, nil
}
func validValue(v string, limit int) bool {
	if v == "" || len(v) > limit || strings.TrimSpace(v) != v {
		return false
	}
	for _, c := range v {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}
func oneHeader(h http.Header, name string) (string, error) {
	// Iterate instead of Header.Values alone: callers may construct mixed-case
	// map keys. Count all variants, including repeated empty values.
	var values []string
	for k, v := range h {
		if strings.EqualFold(k, name) {
			values = append(values, v...)
		}
	}
	if len(values) > 1 {
		return "", ErrUntrusted
	}
	if len(values) == 0 {
		return "", nil
	}
	return values[0], nil
}

// Verify uses the actual connection peer. Forwarded headers never influence trust.
func (p *Proxy) Verify(remoteAddr string, h http.Header) (Assertion, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return Assertion{}, ErrUntrusted
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return Assertion{}, ErrUntrusted
	}
	addr = addr.Unmap()
	trusted := false
	for _, peer := range p.peers {
		if peer.Contains(addr) {
			trusted = true
			break
		}
	}
	if !trusted {
		return Assertion{}, ErrUntrusted
	}
	subject, err := oneHeader(h, p.subjectHeader)
	if err != nil || !validValue(subject, 256) {
		return Assertion{}, ErrUntrusted
	}
	email, err := oneHeader(h, p.emailHeader)
	if err != nil || (email != "" && !validValue(email, 254)) {
		return Assertion{}, ErrUntrusted
	}
	verified, err := oneHeader(h, p.verifiedHeader)
	if err != nil {
		return Assertion{}, ErrUntrusted
	}
	if verified != "" && verified != "true" && verified != "false" {
		return Assertion{}, ErrUntrusted
	}
	if verified == "true" && email == "" {
		return Assertion{}, ErrUntrusted
	}
	return Assertion{Namespace: p.namespace, Subject: subject, Email: email, EmailVerified: verified == "true"}, nil
}
