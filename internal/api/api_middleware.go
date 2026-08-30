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
func originMatchesRequest(origin, scheme, host string) bool {
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return parsed.User == nil &&
		strings.EqualFold(parsed.Scheme, scheme) &&
		strings.EqualFold(parsed.Host, host)
}

func requestScheme(request *http.Request) string {
	if request == nil {
		return ""
	}
	if request.URL != nil && request.URL.Scheme != "" {
		return strings.ToLower(request.URL.Scheme)
	}
	if request.TLS != nil {
		return "https"
	}
	return "http"
}

func sameOriginMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if !originMatchesRequest(c.Get(fiber.HeaderOrigin), c.Protocol(), c.Host()) {
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
