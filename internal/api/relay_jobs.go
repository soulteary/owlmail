package api

import (
	"crypto/rand"
	"errors"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/common"
)

var errRelayJobCapacity = errors.New("relay job status capacity reached")

const (
	relayJobQueued    = "queued"
	relayJobSucceeded = "succeeded"
	relayJobFailed    = "failed"

	defaultRelayJobTTL   = 24 * time.Hour
	defaultRelayJobLimit = 1000
)

type relayJob struct {
	ID            string     `json:"id"`
	EmailID       string     `json:"emailId"`
	RelayTo       string     `json:"relayTo,omitempty"`
	Status        string     `json:"status"`
	ErrorCategory string     `json:"errorCategory,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

type relayJobStore struct {
	mu    sync.Mutex
	jobs  map[string]relayJob
	order []string
	now   func() time.Time
	newID func() (string, error)
	ttl   time.Duration
	limit int
}

func newRelayJobStore() *relayJobStore {
	return &relayJobStore{
		jobs: make(map[string]relayJob), now: time.Now, newID: randomRelayJobID,
		ttl: defaultRelayJobTTL, limit: defaultRelayJobLimit,
	}
}

func randomRelayJobID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate relay job ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func (store *relayJobStore) create(emailID, relayTo string) (relayJob, error) {
	id, err := store.newID()
	if err != nil {
		return relayJob{}, err
	}
	now := store.now().UTC()
	job := relayJob{ID: id, EmailID: emailID, RelayTo: relayTo, Status: relayJobQueued, CreatedAt: now, UpdatedAt: now}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(now)
	if !store.makeRoomLocked() {
		return relayJob{}, errRelayJobCapacity
	}
	store.jobs[id] = job
	store.order = append(store.order, id)
	return job, nil
}

func (store *relayJobStore) complete(id string, relayErr error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	job, ok := store.jobs[id]
	if !ok {
		return
	}
	now := store.now().UTC()
	job.UpdatedAt, job.CompletedAt = now, &now
	if relayErr == nil {
		job.Status, job.ErrorCategory = relayJobSucceeded, ""
	} else {
		job.Status, job.ErrorCategory = relayJobFailed, relayFailureCategory(relayErr)
	}
	store.jobs[id] = job
}

func (store *relayJobStore) remove(id string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.jobs, id)
	for index, item := range store.order {
		if item == id {
			store.order = append(store.order[:index], store.order[index+1:]...)
			return
		}
	}
}

func (store *relayJobStore) get(id string) (relayJob, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(store.now().UTC())
	job, ok := store.jobs[id]
	return job, ok
}

func (store *relayJobStore) pruneLocked(now time.Time) {
	kept := store.order[:0]
	for _, id := range store.order {
		job, ok := store.jobs[id]
		if !ok {
			continue
		}
		if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > store.ttl {
			delete(store.jobs, id)
			continue
		}
		kept = append(kept, id)
	}
	store.order = kept
}

func (store *relayJobStore) makeRoomLocked() bool {
	for store.limit > 0 && len(store.order) >= store.limit {
		removeIndex := -1
		for index, id := range store.order {
			if store.jobs[id].CompletedAt != nil {
				removeIndex = index
				break
			}
		}
		if removeIndex < 0 {
			return false
		}
		id := store.order[removeIndex]
		delete(store.jobs, id)
		store.order = append(store.order[:removeIndex], store.order[removeIndex+1:]...)
	}
	return store.limit > 0
}

func relayFailureCategory(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "queue is full"):
		return "queue_full"
	case strings.Contains(message, "not configured"):
		return "not_configured"
	case strings.Contains(message, "no recipients"):
		return "no_recipients"
	case strings.Contains(message, "authentication") || strings.Contains(message, "auth"):
		return "authentication"
	case strings.Contains(message, "open email") || strings.Contains(message, "read email"):
		return "source_unavailable"
	case strings.Contains(message, "canceled") || strings.Contains(message, "cancelled"):
		return "canceled"
	case strings.Contains(message, "timeout") || strings.Contains(message, "deadline"):
		return "timeout"
	case strings.Contains(message, "connect") || strings.Contains(message, "dial") || strings.Contains(message, "refused"):
		return "connection"
	default:
		return "downstream"
	}
}

func (api *API) relayEmailAsync(c fiber.Ctx) error {
	relayTo := c.Query("relayTo")
	if relayTo == "" {
		var body struct {
			RelayTo string `json:"relayTo"`
		}
		if err := c.Bind().Body(&body); err == nil {
			relayTo = body.RelayTo
		}
	}
	return api.enqueueRelayJob(c, relayTo)
}

func (api *API) relayEmailWithParamAsync(c fiber.Ctx) error {
	relayTo := c.Params("relayTo")
	if relayTo == "" {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidEmailAddress, "Invalid email address provided"))
	}
	return api.enqueueRelayJob(c, relayTo)
}

func (api *API) enqueueRelayJob(c fiber.Ctx, relayTo string) error {
	id := c.Params("id")
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}
	job, err := api.relayJobs.create(id, relayTo)
	if errors.Is(err, errRelayJobCapacity) {
		c.Set(fiber.HeaderRetryAfter, "1")
		return c.Status(http.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay status capacity reached; retry later"))
	}
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeRelayFailed, "Unable to create relay job"))
	}
	callback := func(relayErr error) {
		api.relayJobs.complete(job.ID, relayErr)
		if relayErr != nil {
			common.Error("Relay job %s failed for email %s (category: %s)", job.ID, id, relayFailureCategory(relayErr))
		}
	}
	if relayTo != "" {
		err = api.mailServer.RelayMailTo(email, relayTo, callback)
	} else {
		err = api.mailServer.RelayMail(email, false, callback)
	}
	if err != nil {
		api.relayJobs.remove(job.ID)
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeRelayFailed, err.Error()))
	}
	current, _ := api.relayJobs.get(job.ID)
	statusURL := api.route("/api/v1/relay-jobs/" + job.ID)
	c.Set(fiber.HeaderLocation, statusURL)
	return c.Status(http.StatusAccepted).JSON(SuccessResponse("RELAY_QUEUED", "Relay request accepted", fiber.Map{
		"job": current, "statusUrl": statusURL,
	}))
}

func (api *API) getRelayJob(c fiber.Ctx) error {
	job, ok := api.relayJobs.get(c.Params("jobID"))
	if !ok {
		return c.Status(http.StatusNotFound).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay job not found or expired"))
	}
	return c.JSON(SuccessResponse("RELAY_STATUS", "Relay job status", job))
}
