package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/common"
)

// relayEmail handles the historical POST /email/:id/relay route. Native v1
// routes use relayEmailAsync and return a tracked asynchronous job instead.
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

	email, releaseSource, err := api.mailServer.AcquireEmailSource(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}

	var relayErr error
	if relayTo != "" {
		relayErr = api.mailServer.RelayMailTo(email, relayTo, func(err error) {
			releaseSource()
			if err != nil {
				common.Error("Error relaying email %s to %s: %v", id, relayTo, err)
			}
		})
	} else {
		relayErr = api.mailServer.RelayMail(email, false, func(err error) {
			releaseSource()
			if err != nil {
				common.Error("Error relaying email %s: %v", id, err)
			}
		})
	}

	if relayErr != nil {
		releaseSource()
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeRelayFailed, relayErr.Error()))
	}

	return c.JSON(SuccessResponse(SuccessCodeEmailRelayed, "Email relayed successfully", fiber.Map{"relayTo": relayTo}))
}

// relayEmailWithParam handles the historical POST /email/:id/relay/:relayTo route.
func (api *API) relayEmailWithParam(c fiber.Ctx) error {
	id := c.Params("id")
	relayTo := c.Params("relayTo")

	if relayTo == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidEmailAddress, "Invalid email address provided"))
	}

	email, releaseSource, err := api.mailServer.AcquireEmailSource(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}

	relayErr := api.mailServer.RelayMailTo(email, relayTo, func(err error) {
		releaseSource()
		if err != nil {
			common.Error("Error relaying email %s to %s: %v", id, relayTo, err)
		}
	})

	if relayErr != nil {
		releaseSource()
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse(ErrorCodeRelayFailed, relayErr.Error()))
	}

	return c.JSON(SuccessResponse(SuccessCodeEmailRelayed, "Email relayed successfully", fiber.Map{"relayTo": relayTo}))
}
