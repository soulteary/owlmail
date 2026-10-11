package api

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"
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
	store := api.relayJobs
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(store.now().UTC())
	jobs := make([]relayJob, 0)
	more := false
	for i := len(store.order) - 1; i >= 0; i-- {
		j := store.jobs[store.order[i]]
		if status != "" && j.Status != status || c.Query("email_id") != "" && j.EmailID != c.Query("email_id") {
			continue
		}
		if len(jobs) == limit {
			more = true
			break
		}
		jobs = append(jobs, j)
	}
	return c.JSON(fiber.Map{"jobs": jobs, "more_possible": more, "persistence_healthy": api.relayJobsPersistenceErr == nil && store.lastPersistenceErr == nil})
}
func (api *API) replayRelayJob(c fiber.Ctx) error {
	job, ok := api.relayJobs.get(c.Params("jobID"))
	if !ok {
		return c.Status(404).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Relay job not found"))
	}
	if job.Status != relayJobFailed {
		return c.Status(409).JSON(ErrorResponse(ErrorCodeRelayFailed, "Only failed relay jobs may be replayed"))
	}
	var recipients []string
	if job.Confirmed {
		recipients = job.Recipients
	}
	err := api.enqueueRelaySource(c, job.EmailID, job.RelayTo, recipients)
	if c.Response().StatusCode() == fiber.StatusBadRequest {
		c.Status(fiber.StatusConflict)
	}
	return err
}
func (api *API) listWebhookDeliveries(c fiber.Ctx) error {
	limit, err := workflowLimit(c)
	if err != nil {
		return err
	}
	if api.webhookService == nil {
		return c.JSON(fiber.Map{"enabled": false, "deliveries": []webhook.DeliveryRecord{}})
	}
	status := c.Query("status")
	if status != "" && status != "failed" && status != "succeeded" {
		return c.Status(400).JSON(ErrorResponse(ErrorCodeInvalidRequest, "Invalid status"))
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
