package api

import (
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/net/idna"
)

// originMatchesRequest implements the browser same-origin check used for
// authenticated HTTP and WebSocket requests. Requests without an Origin header
// are non-browser clients and remain allowed.
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

func sameOriginMiddleware(scheme string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !originMatchesRequest(c.Get(fiber.HeaderOrigin), c.Host(), scheme) {
			return c.SendStatus(http.StatusForbidden)
		}
		return c.Next()
	}
}

// basicAuthMiddleware creates HTTP Basic Auth middleware for Fiber
func basicAuthMiddleware(username, password string, skippedPaths ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		for _, p := range skippedPaths {
			if path == p {
				return c.Next()
			}
		}

		auth := c.Get("Authorization")
		if auth == "" {
			c.Set("WWW-Authenticate", `Basic realm="OwlMail"`)
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		const prefix = "Basic "
		if !strings.HasPrefix(auth, prefix) {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
		if err != nil {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		credentials := strings.SplitN(string(decoded), ":", 2)
		if len(credentials) != 2 {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		if credentials[0] != username || credentials[1] != password {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		return c.Next()
	}
}

// mcpAllowAnyOrigin disables MCP browser origin validation. It is an explicit,
// documented opt-out for deployments that control browser access elsewhere.
const mcpAllowAnyOrigin = "*"

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

// originAllowed reports whether a browser Origin may reach the MCP endpoint.
// An absent Origin identifies a non-browser client such as curl, the MCP SDK's
// HTTP client, or another server, and stays allowed; browsers always send the
// header on cross-origin requests.
//
// The "*" opt-out is deliberately not handled here. Recognizing it in two
// places is what let it sit behind this parse gate, where an opaque browser
// origin such as "null" was refused before the opt-out was ever consulted.
// mcpAllowsAnyOrigin is the single authority, and its caller answers before
// reaching this function.
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
