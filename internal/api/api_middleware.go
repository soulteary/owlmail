package api

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
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

// normalizeOrigin reduces a browser Origin header or a configured allow-list
// entry to its canonical scheme://host[:port] form. Values that are not an
// absolute http or https origin are rejected rather than partially matched.
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
	return scheme + "://" + strings.ToLower(parsed.Host), true
}

// originAllowed reports whether a browser Origin may reach the MCP endpoint.
// An absent Origin identifies a non-browser client such as curl, the MCP SDK's
// HTTP client, or another server, and stays allowed; browsers always send the
// header on cross-origin requests.
func originAllowed(origin string, allowed []string) bool {
	if strings.TrimSpace(origin) == "" {
		return true
	}
	normalized, ok := normalizeOrigin(origin)
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == mcpAllowAnyOrigin {
			return true
		}
		if candidate == normalized {
			return true
		}
	}
	return false
}
