package api

// Browser origin machinery shared by the Web origin guard and the MCP origin
// guard. One canonicalizer and one allow-list builder serve both, so a value an
// operator writes for one option cannot be refused by the other.

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

// wildcardBindHosts are listen addresses that name every local interface
// rather than one browser-visible hostname. They cannot contribute a usable
// origin, so the loopback names are used instead.
var wildcardBindHosts = map[string]struct{}{
	"":        {},
	"0.0.0.0": {},
	"::":      {},
	"[::]":    {},
	"*":       {},
}

// loopbackOriginHosts are always accepted because OwlMail is documented as a
// loopback-first development and CI service.
var loopbackOriginHosts = []string{"localhost", "127.0.0.1", "::1"}

// originHost prepares a listen address for use in an origin. An operator may
// spell an IPv6 address with brackets, and net.JoinHostPort adds its own, so
// they are stripped here rather than deriving "[[::1]]:1080".
//
// Case is deliberately left alone: canonicalOrigin owns it, and lowering it
// early is the same mistake that made a configured "İ.com" resolve to the
// unrelated "i.com". Applying the rule in one path and not the other is how
// that class of defect survives a fix.
func originHost(host string) string {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	return host
}

// normalizeAllowedOrigins reduces a configured allow list to the canonical form
// the guards compare against. The Web API and the MCP endpoint share it so one
// spelling cannot be accepted on one endpoint and refused on the other; subject
// names the option in the error an operator reads.
func normalizeAllowedOrigins(origins []string, subject string) ([]string, error) {
	normalized := make([]string, 0, len(origins))
	wildcard := false
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == allowAnyOrigin {
			wildcard = true
			continue
		}
		canonical, ok := normalizeOrigin(origin)
		if !ok {
			return nil, fmt.Errorf("%s allowed origin %q must be an absolute http or https origin", subject, origin)
		}
		normalized = append(normalized, canonical)
	}
	// The wildcard turns validation off for every origin, so pairing it with a
	// named one can only be a mistake -- and exactly the mistake that silently
	// opens a list its author meant to keep narrow. The configuration parser
	// refuses the combination; so must the setter that applies the policy,
	// which is reachable without going through that parser.
	if wildcard {
		if len(normalized) > 0 {
			return nil, fmt.Errorf("%s allowed origins cannot combine %q with an explicit origin", subject, allowAnyOrigin)
		}
		normalized = []string{allowAnyOrigin}
	}
	return normalized, nil
}

// allowsAnyOrigin reports the documented opt-out, where the operator has turned
// origin validation off entirely rather than naming any origin.
func allowsAnyOrigin(allowed []string) bool {
	for _, origin := range allowed {
		if origin == allowAnyOrigin {
			return true
		}
	}
	return false
}

// listenerScheme is the scheme this process actually answers on. It is
// deliberately not requestScheme: when TLS terminates at a reverse proxy,
// -web-external-url makes the browser-visible scheme https while this listener
// still serves plain HTTP, and the origins derived from it describe the
// listener. The browser-visible origin is added separately, from configuration.
func (api *API) listenerScheme() string {
	if api.httpsEnabled {
		return "https"
	}
	return "http"
}

// originAllowList joins a configured allow list to the origins this listener
// answers on, which are accepted by every endpoint and need not be configured.
// It is computed per request because the external scheme and base pathname are
// configured after the router is built, and only when a request actually
// carries an Origin header.
func (api *API) originAllowList(configured []string) []string {
	allowed := make([]string, 0, len(configured)+8)
	seen := make(map[string]struct{}, len(configured)+8)
	add := func(origin string) {
		if _, exists := seen[origin]; exists {
			return
		}
		seen[origin] = struct{}{}
		allowed = append(allowed, origin)
	}
	for _, origin := range configured {
		add(origin)
	}

	scheme := api.listenerScheme()
	hosts := make([]string, 0, len(loopbackOriginHosts)+1)
	if host := originHost(api.host); !isWildcardBindHost(host) {
		hosts = append(hosts, host)
	}
	hosts = append(hosts, loopbackOriginHosts...)

	for _, host := range hosts {
		add(canonicalOrigin(scheme, host, strconv.Itoa(api.port)))
	}
	return allowed
}

func isWildcardBindHost(host string) bool {
	// The keys hold no letters today, but folding here keeps this correct
	// without depending on a caller having lower-cased first.
	_, wildcard := wildcardBindHosts[strings.ToLower(host)]
	return wildcard
}

// originMatchesRequest implements the browser same-origin check used by the Web
// origin guard and the WebSocket handshake. Requests without an Origin header
// are non-browser clients and remain allowed.
//
// Comparing against the request's own Host is what keeps the browser UI working
// on a hostname OwlMail was never configured with, a reverse proxy's included.
// It is therefore not a defense on its own: a hostname an attacker re-binds to
// the loopback address produces a matching pair. The guard's configured allow
// list is the part that is not derived from the request.
func originMatchesRequest(origin, host, scheme string) bool {
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return parsed.User == nil && strings.EqualFold(parsed.Host, host) && strings.EqualFold(parsed.Scheme, scheme)
}

// allowAnyOrigin disables browser origin validation on the endpoint whose allow
// list carries it. It is an explicit, documented opt-out for deployments that
// control browser access elsewhere. Both the Web API and the MCP endpoint spell
// it the same way so an operator does not have to learn two notations.
const allowAnyOrigin = "*"

// canonicalOrigin renders one origin the way a browser serializes it: lower
// case, and with no port when the port is the scheme's default. Comparing
// canonical forms means a configured "https://host:443" still matches the
// "https://host" a browser actually sends. The host is expected unbracketed;
// an IPv6 literal is bracketed here.
func canonicalOrigin(scheme, host, port string) string {
	scheme = strings.ToLower(scheme)
	// The host is deliberately not lower-cased before this point.
	// strings.ToLower applies simple, per-rune case mapping, which destroys
	// information the IDNA profile needs: it collapses "İ" (U+0130) to a plain
	// "i", so "İ.com" would be stored as "i.com" -- refusing the browser, which
	// sends "xn--i-9bb.com", while accepting an unrelated domain anyone can
	// register. IDNA folds case correctly as part of its own mapping, and
	// net.ParseIP accepts upper-case hex, so each branch below lower-cases only
	// what it produces.
	if ip := net.ParseIP(host); ip != nil {
		// An IP address has many equivalent spellings and a browser serializes
		// one of them, so "[2001:0db8::1]" must not compare as a different
		// origin from "[2001:db8::1]", nor "[::ffff:192.0.2.1]" from
		// "[::ffff:c000:201]". Rewriting is safe because both sides of the
		// comparison pass through here, so it can only make genuinely equal
		// addresses match. A spelling Go will not parse -- a leading-zero IPv4
		// such as "127.0.0.01", or a zoned address -- is left alone and must be
		// configured as written.
		//
		// An IPv4-mapped address written in IPv6 notation stays an IPv6 host.
		// net.IP.String renders it in dotted form, which would make
		// "[::ffff:192.0.2.1]" and "192.0.2.1" one value, but a browser keeps
		// them apart: one is an IPv6 host and the other an IPv4 host, so they
		// are different origins. Compression is unambiguous for this shape --
		// five leading zero groups are always the longest run -- so it is
		// rendered directly rather than through a general IPv6 serializer.
		if v4 := ip.To4(); v4 != nil && strings.Contains(host, ":") {
			host = "::ffff:" +
				strconv.FormatUint(uint64(v4[0])<<8|uint64(v4[1]), 16) + ":" +
				strconv.FormatUint(uint64(v4[2])<<8|uint64(v4[3]), 16)
		} else {
			host = ip.String()
		}
	} else if ascii, err := idna.Lookup.ToASCII(host); err == nil && ascii != "" {
		// A browser serializes a domain in its IDNA ASCII form, so a Unicode
		// spelling such as "例え.テスト" has to match the "xn--" origin it
		// actually sends. The profile also folds case, so an already-ASCII host
		// comes back lower-cased and unchanged otherwise.
		host = strings.ToLower(ascii)
	} else {
		// A host the profile rejects -- an underscore label or a zoned address,
		// say -- keeps the spelling it was given rather than being dropped, so
		// nothing that matches today stops matching.
		host = strings.ToLower(host)
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	// A port is a number, not a string: a browser drops "0443" to the default
	// and renders "08443" as "8443", so the padding must go before the default
	// is recognized. url.Parse has already rejected a non-numeric port, so the
	// error path only guards a caller passing one directly.
	if number, err := strconv.Atoi(port); err == nil {
		port = strconv.Itoa(number)
	}
	if port == "" || (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		return scheme + "://" + host
	}
	return scheme + "://" + host + ":" + port
}

// normalizeOrigin reduces a browser Origin header or a configured allow-list
// entry to its canonical form. Values that are not an absolute http or https
// origin are rejected rather than partially matched.
func normalizeOrigin(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", false
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", false
	}
	if parsed.Hostname() == "" {
		return "", false
	}
	return canonicalOrigin(scheme, parsed.Hostname(), parsed.Port()), true
}

// originAllowed reports whether a browser Origin appears in an endpoint's
// allow list. Both the Web API guard and the MCP guard call it, so it decides
// membership only and leaves the policy around that answer to each caller.
// An absent Origin identifies a non-browser client such as curl, an SDK HTTP
// client, or another server, and stays allowed; browsers always send the
// header on cross-origin requests.
//
// The "*" opt-out is deliberately not handled here. Recognizing it in more than
// one place is what let it sit behind this parse gate, where an opaque browser
// origin such as "null" was refused before the opt-out was ever consulted. Each
// endpoint's own allows-any predicate is the single authority for that endpoint
// and answers before this function is reached.
func originAllowed(origin string, allowed []string) bool {
	if strings.TrimSpace(origin) == "" {
		return true
	}
	normalized, ok := normalizeOrigin(origin)
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == normalized {
			return true
		}
	}
	return false
}
