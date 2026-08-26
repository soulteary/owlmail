package api

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v3"
)

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
