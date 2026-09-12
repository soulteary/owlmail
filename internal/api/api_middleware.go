package api

import (
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/common"
)

// basicAuthMiddleware creates HTTP Basic Auth middleware for Fiber.
//
// The credential check is delegated to a common.CredentialVerifier rather than
// comparing the decoded values directly: Go's == returns at the first differing
// byte, which times a near-miss differently from a first-byte miss and lets the
// configured password be recovered one byte at a time. A nil verifier means the
// process failed to build one, and every authenticated request is refused
// rather than falling back to a comparison that leaks.
func basicAuthMiddleware(verifier *common.CredentialVerifier, skippedPaths ...string) fiber.Handler {
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

		if verifier == nil || !verifier.CredentialsEqual(credentials[0], credentials[1]) {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		return c.Next()
	}
}
