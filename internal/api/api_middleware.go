package api

import (
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
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
	host = strings.ToLower(host)
	if ip := net.ParseIP(host); ip != nil {
		// An IPv6 literal has many equivalent spellings and a browser
		// serializes the compressed one, so "[2001:0db8::1]" must not compare
		// as a different origin from "[2001:db8::1]". IPv4 and IPv4-mapped
		// addresses are left exactly as written rather than rewritten into a
		// different notation.
		if ip.To4() == nil {
			host = ip.String()
		}
	} else if ascii, err := idna.Lookup.ToASCII(host); err == nil && ascii != "" {
		// A browser serializes a domain in its IDNA ASCII form, so a Unicode
		// spelling such as "例え.テスト" has to match the "xn--" origin it
		// actually sends. Already-ASCII hosts are unchanged by this. A host the
		// profile rejects -- an underscore label or a zoned address, say --
		// keeps the spelling it was given rather than being dropped, so nothing
		// that matches today stops matching.
		host = ascii
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
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
