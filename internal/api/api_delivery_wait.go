package api

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func (api *API) getDeliveryCursor(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"cursor": api.mailServer.DeliveryCursor()})
}
func (api *API) waitDeliveries(c fiber.Ctx) error {
	var input struct {
		After string `json:"after"`
		mailserver.DeliveryFilter
		Count          int `json:"count"`
		TimeoutSeconds int `json:"timeout_seconds"`
	}
	if err := c.Bind().Body(&input); err != nil {
		return invalidEmailQuery(c, err)
	}
	if input.Count == 0 {
		input.Count = 1
	}
	if input.TimeoutSeconds == 0 {
		input.TimeoutSeconds = 30
	}
	if input.TimeoutSeconds < 1 || input.TimeoutSeconds > 120 {
		return c.Status(400).JSON(ErrorResponse(ErrorCodeInvalidRequest, "timeout_seconds must be 1..120"))
	}
	result, err := api.mailServer.WaitDeliveries(c.Context(), input.After, input.DeliveryFilter, input.Count, time.Duration(input.TimeoutSeconds)*time.Second)
	if err != nil {
		status := 400
		if errors.Is(err, mailserver.ErrWaitCapacity) {
			status = 429
			c.Set("Retry-After", "1")
		}
		if errors.Is(err, mailserver.ErrMailboxClosed) {
			status = 503
		}
		return c.Status(status).JSON(ErrorResponse(ErrorCodeInvalidRequest, err.Error()))
	}
	return c.JSON(result)
}
func (api *API) deleteTestRunEmails(c fiber.Ctx) error {
	runID := c.Params("runID")
	if err := mailserver.ValidateRunID(runID); err != nil || runID == "" {
		return c.Status(400).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid run_id"))
	}
	emails, total := api.mailServer.QueryEmails(mailserver.EmailQuery{Limit: 1000, MatchStoreEmail: mailserver.DeliveryFilter{RunID: runID}.Matches})
	ids := make([]string, 0, len(emails))
	for _, e := range emails {
		ids = append(ids, e.ID)
	}
	deleted := make([]string, 0, len(ids))
	failed := make([]string, 0)
	// A nil/empty ID list means all sources to the protection helper; avoid it for an empty run.
	if len(ids) > 0 {
		err := api.relayService.ProtectSourceDeletion(ids, func() error {
			for _, id := range ids {
				if err := api.mailServer.DeleteEmail(id); err != nil {
					failed = append(failed, id)
				} else {
					deleted = append(deleted, id)
				}
			}
			return nil
		})
		if err != nil {
			return c.Status(409).JSON(ErrorResponse(ErrorCodeRelayFailed, "Run contains a pending relay source"))
		}
	}
	return c.JSON(fiber.Map{"run_id": runID, "deleted": deleted, "failed": failed, "more_possible": total > len(ids)})
}
