package api

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/relay"
	"github.com/soulteary/owlmail/internal/webhook"
)

// SetWebhookService connects delivery diagnostics before the API starts serving.
func (api *API) SetWebhookService(service *webhook.Service) { api.webhookService = service }
func workflowLimit(c fiber.Ctx) (int, error) {
	if c.Query("limit") == "" {
		return 100, nil
	}
	n, err := strconv.Atoi(c.Query("limit"))
	if err != nil || n < 1 || n > 1000 {
		return 0, fiber.NewError(400, "limit must be between 1 and 1000")
	}
	return n, nil
}
func (api *API) listRelayJobs(c fiber.Ctx) error {
	limit, err := workflowLimit(c)
	if err != nil {
		return err
	}
	status := c.Query("status")
	if status != "" && status != relayJobQueued && status != relayJobFailed && status != relayJobSucceeded {
		return c.Status(400).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid status"))
	}
	jobs, more := api.relayService.List(status, c.Query("email_id"), limit)
	return c.JSON(fiber.Map{"jobs": jobs, "more_possible": more, "persistence_healthy": api.relayService.PersistenceError() == nil})
}
func (api *API) replayRelayJob(c fiber.Ctx) error {
	job, err := api.relayService.Retry(c.Params("jobID"))
	if err != nil {
		if errors.Is(err, relay.ErrNotFound) {
			return c.Status(404).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Relay job not found"))
		}
		if errors.Is(err, relay.ErrPersistence) || errors.Is(err, relay.ErrCapacity) {
			return relaySubmitError(c, err)
		}
		return c.Status(409).JSON(ErrorResponse(ErrorCodeRelayFailed, err.Error()))
	}
	statusURL := api.route("/api/v1/relay-jobs/" + job.ID)
	c.Set(fiber.HeaderLocation, statusURL)
	return c.Status(202).JSON(SuccessResponse("RELAY_QUEUED", "Relay request accepted", fiber.Map{"job": job, "statusUrl": statusURL}))
}

func (api *API) listWebhookDeliveries(c fiber.Ctx) error {
	limit, err := workflowLimit(c)
	if err != nil {
		return err
	}
	status := c.Query("status")
	if status != "" && status != "failed" && status != "succeeded" {
		return c.Status(400).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid status"))
	}
	if api.webhookService == nil {
		return c.JSON(fiber.Map{"enabled": false, "deliveries": []webhook.DeliveryRecord{}})
	}
	records, healthy := api.webhookService.DeliveryHistory()
	result := make([]webhook.DeliveryRecord, 0)
	more := false
	for _, r := range records {
		if status != "" && r.Status != status || c.Query("email_id") != "" && r.EmailID != c.Query("email_id") {
			continue
		}
		if len(result) == limit {
			more = true
			break
		}
		result = append(result, r)
	}
	return c.JSON(fiber.Map{"enabled": true, "deliveries": result, "more_possible": more, "persistence_healthy": healthy})
}
func (api *API) replayWebhookDelivery(c fiber.Ctx) error {
	if api.webhookService == nil {
		return c.Status(503).JSON(ErrorResponse(ErrorCodeRelayFailed, "Webhooks are disabled"))
	}
	records, _ := api.webhookService.DeliveryHistory()
	id := c.Params("jobID")
	for _, r := range records {
		if r.ID != id {
			continue
		}
		email, release, err := api.mailServer.AcquireEmailSource(r.EmailID)
		if err != nil {
			return c.Status(404).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email source unavailable"))
		}
		defer release()
		if err = api.webhookService.RetryDelivery(id, email); err != nil {
			if errors.Is(err, webhook.ErrDeliveryReplayConflict) {
				return c.Status(409).JSON(ErrorResponse(ErrorCodeRelayFailed, err.Error()))
			}
			return c.Status(503).JSON(ErrorResponse(ErrorCodeRelayFailed, "Webhook replay infrastructure unavailable"))
		}
		return c.Status(202).JSON(SuccessResponse("WEBHOOK_REPLAY_QUEUED", "Failed targets queued for replay", fiber.Map{"delivery_id": id}))
	}
	return c.Status(404).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Delivery not found"))
}
