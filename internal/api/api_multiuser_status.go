package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/identity"
	"github.com/soulteary/owlmail/internal/mailbox"
)

func (api *API) multiMailboxStatus(c fiber.Ctx) error {
	assertion, ok := c.Locals(multiuserAssertionKey).(identity.Assertion)
	if !ok {
		return c.SendStatus(401)
	}
	status, err := api.multiuser.Store.LoginStatus(c.Context(), mailbox.Login{Namespace: assertion.Namespace, Subject: assertion.Subject, Email: assertion.Email, Verified: assertion.EmailVerified})
	if err != nil {
		return c.Status(503).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Mailbox status unavailable"))
	}
	return c.JSON(status)
}
