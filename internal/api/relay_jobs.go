package api

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/relay"
)

type relayJob = relay.Job

const (
	relayJobQueued    = relay.Queued
	relayJobSucceeded = relay.Succeeded
	relayJobFailed    = relay.Failed
)

func (api *API) relayEmailAsync(c fiber.Ctx) error {
	relayTo := c.Query("relayTo")
	var confirmedRecipients []string
	if len(c.Request().Body()) > 0 {
		var body struct {
			RelayTo             string    `json:"relayTo"`
			ConfirmedRecipients *[]string `json:"confirmedRecipients"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid relay request body"))
		}
		if relayTo == "" {
			relayTo = body.RelayTo
			if body.ConfirmedRecipients != nil {
				confirmedRecipients = *body.ConfirmedRecipients
			}
		}
	}
	return api.enqueueRelayJob(c, relayTo, confirmedRecipients)
}

func (api *API) relayEmailWithParamAsync(c fiber.Ctx) error {
	relayTo := c.Params("relayTo")
	if relayTo == "" {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidEmailAddress, "Invalid email address provided"))
	}
	return api.enqueueRelayJob(c, relayTo, nil)
}

func (api *API) relayEmailPreflight(c fiber.Ctx) error {
	email, err := api.mailServer.GetEmail(c.Params("id"))
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}
	recipients, err := api.mailServer.EffectiveRelayRecipients(email)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeRelayFailed, err.Error()))
	}
	return c.JSON(SuccessResponse("RELAY_PREFLIGHT", "Effective relay recipients", fiber.Map{"recipients": recipients}))
}

func (api *API) enqueueRelayJob(c fiber.Ctx, relayTo string, confirmedRecipients []string) error {
	return api.enqueueRelaySource(c, c.Params("id"), relayTo, confirmedRecipients)
}

func (api *API) enqueueRelaySource(c fiber.Ctx, id, relayTo string, confirmedRecipients []string) error {
	job, err := api.relayService.Submit(relay.Request{EmailID: id, RelayTo: relayTo, ConfirmedRecipients: confirmedRecipients, Origin: "native"})
	if err != nil {
		return relaySubmitError(c, err)
	}
	statusURL := api.route("/api/v1/relay-jobs/" + job.ID)
	c.Set(fiber.HeaderLocation, statusURL)
	return c.Status(http.StatusAccepted).JSON(SuccessResponse("RELAY_QUEUED", "Relay request accepted", fiber.Map{"job": job, "statusUrl": statusURL}))
}
func relaySubmitError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, relay.ErrCapacity):
		c.Set("Retry-After", "1")
		return c.Status(http.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay status capacity reached"))
	case errors.Is(err, relay.ErrPersistence), errors.Is(err, outgoing.ErrClosed):
		c.Set("Retry-After", "1")
		return c.Status(http.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay job persistence or capacity is unavailable"))
	case errors.Is(err, relay.ErrNotFound):
		return c.Status(http.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	case errors.Is(err, relay.ErrAlreadyPending):
		return c.Status(http.StatusConflict).JSON(ErrorResponse(ErrorCodeRelayFailed, "Email already has a pending relay job"))
	case errors.Is(err, relay.ErrRecipientTooLong):
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidEmailAddress, "Relay recipient exceeds 1024 UTF-8 bytes"))
	default:
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeRelayFailed, err.Error()))
	}
}
func (api *API) getRelayJob(c fiber.Ctx) error {
	job, ok := api.relayService.Get(c.Params("jobID"))
	if !ok {
		return c.Status(http.StatusNotFound).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay job not found or expired"))
	}
	return c.JSON(SuccessResponse("RELAY_STATUS", "Relay job status", job))
}
