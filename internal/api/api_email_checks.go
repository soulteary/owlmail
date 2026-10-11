package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/emailcheck"
)

func (api *API) checkEmail(c fiber.Ctx) error {
	email, err := api.mailServer.GetEmail(c.Params("id"))
	if err != nil {
		return c.Status(404).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}
	var request emailcheck.Request
	if len(c.Body()) > 64<<10 {
		return c.Status(413).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Inspection request cannot exceed 64 KiB"))
	}
	if err := c.Bind().Body(&request); err != nil {
		return c.Status(400).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid inspection request"))
	}
	report, err := emailcheck.Inspect(email, request)
	if err != nil {
		return c.Status(400).JSON(ErrorResponse(ErrorCodeInvalidRequest, err.Error()))
	}
	return c.JSON(report)
}
