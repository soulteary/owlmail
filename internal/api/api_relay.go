package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/common"
)

// relayEmail handles POST /api/v1/emails/:id/actions/relay
func (api *API) relayEmail(c fiber.Ctx) error {
	id := c.Params("id")

	relayTo := c.Query("relayTo")
	if relayTo == "" {
		var body struct {
			RelayTo string `json:"relayTo"`
		}
		if err := c.Bind().Body(&body); err == nil {
			relayTo = body.RelayTo
		}
	}

	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}

	var relayErr error
	if relayTo != "" {
		relayErr = api.mailServer.RelayMailTo(email, relayTo, func(err error) {
			if err != nil {
				common.Error("Error relaying email %s to %s: %v", id, relayTo, err)
			}
		})
	} else {
		relayErr = api.mailServer.RelayMail(email, false, func(err error) {
			if err != nil {
				common.Error("Error relaying email %s: %v", id, err)
			}
		})
	}

	if relayErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeRelayFailed, relayErr.Error()))
	}

	return c.JSON(SuccessResponse(SuccessCodeEmailRelayed, "Email relayed successfully", fiber.Map{"relayTo": relayTo}))
}

// relayEmailWithParam handles POST /api/v1/emails/:id/actions/relay/:relayTo
func (api *API) relayEmailWithParam(c fiber.Ctx) error {
	id := c.Params("id")
	relayTo := c.Params("relayTo")

	if relayTo == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidEmailAddress, "Invalid email address provided"))
	}

	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}

	relayErr := api.mailServer.RelayMailTo(email, relayTo, func(err error) {
		if err != nil {
			common.Error("Error relaying email %s to %s: %v", id, relayTo, err)
		}
	})

	if relayErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeRelayFailed, relayErr.Error()))
	}

	return c.JSON(SuccessResponse(SuccessCodeEmailRelayed, "Email relayed successfully", fiber.Map{"relayTo": relayTo}))
}
