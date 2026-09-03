package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

var (
	errRelayJobCapacity       = errors.New("relay job status capacity reached")
	errRelayRecipientTooLong  = errors.New("relay recipient exceeds size limit")
	errRelayAttemptsExhausted = errors.New("relay job attempt limit reached")
	errRelayJobPersistence    = errors.New("relay job persistence unavailable")
	errRelayJobRetained       = errors.New("relay job retained after an indeterminate persistence failure")
)

const (
	relayJobQueued    = "queued"
	relayJobSucceeded = "succeeded"
	relayJobFailed    = "failed"

	defaultRelayJobTTL              = 24 * time.Hour
	defaultRelayJobLimit            = 1000
	defaultRelayJobMinimumRetention = time.Minute
	defaultRelayRecipientMaxBytes   = 1024
	defaultRelayMaxAttempts         = 3
	defaultRelayRetryBaseDelay      = 250 * time.Millisecond
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
	Attempts      int        `json:"attempts"`
	NextAttemptAt *time.Time `json:"nextAttemptAt,omitempty"`
	retainUntil   time.Time
}

type relayJobStore struct {
	mu               sync.Mutex
	jobs             map[string]relayJob
	order            []string
	now              func() time.Time
	newID            func() (string, error)
	ttl              time.Duration
	limit            int
	minimumRetention time.Duration
	directory        string
	syncDirectory    func(string) error
}

func newRelayJobStore() *relayJobStore {
	return &relayJobStore{
		jobs:             make(map[string]relayJob),
		now:              time.Now,
		newID:            randomRelayJobID,
		ttl:              defaultRelayJobTTL,
		limit:            defaultRelayJobLimit,
		minimumRetention: defaultRelayJobMinimumRetention,
		syncDirectory:    syncRelayJobDirectory,
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
	if len(relayTo) > defaultRelayRecipientMaxBytes {
		return relayJob{}, errRelayRecipientTooLong
	}
	id, err := store.newID()
	if err != nil {
		return relayJob{}, err
	}
	now := store.now().UTC()
	job := relayJob{ID: id, EmailID: emailID, RelayTo: relayTo, Status: relayJobQueued, CreatedAt: now, UpdatedAt: now}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(now)
	if !store.makeRoomLocked(now) {
		return relayJob{}, errRelayJobCapacity
	}
	store.jobs[id] = job
	store.order = append(store.order, id)
	if err := store.persistLocked(job); err != nil {
		if cleanupErr := store.removePersistedLocked(id); cleanupErr != nil {
			return job, fmt.Errorf("%w: persist: %v; cleanup: %v", errRelayJobRetained, err, cleanupErr)
		}
		delete(store.jobs, id)
		store.order = store.order[:len(store.order)-1]
		return relayJob{}, fmt.Errorf("%w: %v", errRelayJobPersistence, err)
	}
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
	job.NextAttemptAt = nil
	job.retainUntil = now.Add(store.minimumRetention)
	if relayErr == nil {
		job.Status, job.ErrorCategory = relayJobSucceeded, ""
	} else {
		job.Status, job.ErrorCategory = relayJobFailed, relayFailureCategory(relayErr)
	}
	store.jobs[id] = job
	if err := store.persistLocked(job); err != nil {
		common.Error("Persist relay job %s completion: %v", id, err)
	}
}

func (store *relayJobStore) beginAttempt(id string) (relayJob, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	job, ok := store.jobs[id]
	if !ok || job.CompletedAt != nil {
		return relayJob{}, fmt.Errorf("relay job is not queued")
	}
	if job.Attempts >= defaultRelayMaxAttempts {
		return relayJob{}, errRelayAttemptsExhausted
	}
	job.Attempts++
	job.UpdatedAt = store.now().UTC()
	job.NextAttemptAt = nil
	if err := store.persistLocked(job); err != nil {
		return relayJob{}, fmt.Errorf("%w: %v", errRelayJobPersistence, err)
	}
	store.jobs[id] = job
	return job, nil
}

func (store *relayJobStore) queueRetry(id string, relayErr error, delay time.Duration) (relayJob, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	job, ok := store.jobs[id]
	if !ok || job.CompletedAt != nil || job.Attempts >= defaultRelayMaxAttempts {
		return relayJob{}, false
	}
	now := store.now().UTC()
	next := now.Add(delay)
	job.Status = relayJobQueued
	job.ErrorCategory = relayFailureCategory(relayErr)
	job.UpdatedAt = now
	job.NextAttemptAt = &next
	if err := store.persistLocked(job); err != nil {
		common.Error("Persist relay job %s retry: %v", id, err)
		return relayJob{}, false
	}
	store.jobs[id] = job
	return job, true
}

// restoreQueuedAfterShutdown rolls back the attempt reservation made just
// before the outgoing relay rejected work because shutdown had started. A
// process shutdown must not consume an attempt or make recoverable work
// terminal; the next process will retry the durable queued record.
func (store *relayJobStore) restoreQueuedAfterShutdown(previous relayJob) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	current, ok := store.jobs[previous.ID]
	if !ok || current.CompletedAt != nil {
		return nil
	}
	previous.Status = relayJobQueued
	previous.CompletedAt = nil
	previous.NextAttemptAt = nil
	previous.UpdatedAt = store.now().UTC()
	if err := store.persistLocked(previous); err != nil {
		return fmt.Errorf("%w: %v", errRelayJobPersistence, err)
	}
	store.jobs[previous.ID] = previous
	return nil
}

func (store *relayJobStore) remove(id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.removePersistedLocked(id); err != nil {
		return fmt.Errorf("%w: %v", errRelayJobPersistence, err)
	}
	delete(store.jobs, id)
	for index, item := range store.order {
		if item == id {
			store.order = append(store.order[:index], store.order[index+1:]...)
			return nil
		}
	}
	return nil
}

func (store *relayJobStore) get(id string) (relayJob, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(store.now().UTC())
	job, ok := store.jobs[id]
	return job, ok
}

func (store *relayJobStore) hasQueued() bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, job := range store.jobs {
		if job.Status == relayJobQueued && job.CompletedAt == nil {
			return true
		}
	}
	return false
}

func (store *relayJobStore) pruneLocked(now time.Time) {
	kept := store.order[:0]
	for _, id := range store.order {
		job, ok := store.jobs[id]
		if !ok {
			continue
		}
		if job.CompletedAt != nil && now.Sub(*job.CompletedAt) > store.ttl {
			if err := store.removePersistedLocked(id); err != nil {
				common.Error("Remove expired relay job %s: %v", id, err)
				kept = append(kept, id)
				continue
			}
			delete(store.jobs, id)
			continue
		}
		kept = append(kept, id)
	}
	store.order = kept
}

func (store *relayJobStore) makeRoomLocked(now time.Time) bool {
	for store.limit > 0 && len(store.order) >= store.limit {
		removeIndex := -1
		for index, id := range store.order {
			job := store.jobs[id]
			if job.CompletedAt != nil && !now.Before(job.retainUntil) {
				removeIndex = index
				break
			}
		}
		if removeIndex < 0 {
			return false
		}
		id := store.order[removeIndex]
		if err := store.removePersistedLocked(id); err != nil {
			common.Error("Remove relay job %s for capacity: %v", id, err)
			return false
		}
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
	if api.relayJobsPersistenceErr != nil {
		c.Set("Retry-After", "1")
		return c.Status(http.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay job persistence is unavailable"))
	}
	id := c.Params("id")
	email, err := api.mailServer.GetEmail(id)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(ErrorResponse(ErrorCodeEmailNotFound, "Email not found"))
	}
	job, err := api.relayJobs.create(id, relayTo)
	retained := errors.Is(err, errRelayJobRetained)
	if errors.Is(err, errRelayRecipientTooLong) {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeInvalidEmailAddress, "Relay recipient exceeds 1024 UTF-8 bytes"))
	}
	if errors.Is(err, errRelayJobCapacity) {
		c.Set("Retry-After", "1")
		return c.Status(http.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay status capacity reached; retry later"))
	}
	if errors.Is(err, errRelayJobPersistence) {
		c.Set("Retry-After", "1")
		return c.Status(http.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay job persistence is temporarily unavailable"))
	}
	if retained {
		common.Error("Relay job %s was accepted after an indeterminate persistence failure: %v", job.ID, err)
		err = nil
	}
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse(ErrorCodeRelayFailed, "Unable to create relay job"))
	}
	if retained {
		time.AfterFunc(defaultRelayRetryBaseDelay, func() { api.retryRelayJob(job.ID) })
	} else {
		err, handled := api.submitRelayJob(job, email)
		if err != nil && !handled {
			if errors.Is(err, errRelayJobPersistence) {
				if removeErr := api.relayJobs.remove(job.ID); removeErr == nil {
					c.Set("Retry-After", "1")
					return c.Status(http.StatusServiceUnavailable).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay job persistence is temporarily unavailable"))
				}
				common.Error("Relay job %s remains accepted after its attempt could not be persisted or durably removed: %v", job.ID, err)
				time.AfterFunc(defaultRelayRetryBaseDelay, func() { api.retryRelayJob(job.ID) })
			} else {
				if removeErr := api.relayJobs.remove(job.ID); removeErr != nil {
					common.Error("Relay job %s remains accepted after synchronous rejection could not be made durable: %v", job.ID, removeErr)
					api.relayJobs.complete(job.ID, err)
				} else {
					return c.Status(http.StatusBadRequest).JSON(ErrorResponse(ErrorCodeRelayFailed, err.Error()))
				}
			}
		}
	}
	current, _ := api.relayJobs.get(job.ID)
	statusURL := api.route("/api/v1/relay-jobs/" + job.ID)
	c.Set(fiber.HeaderLocation, statusURL)
	return c.Status(http.StatusAccepted).JSON(SuccessResponse("RELAY_QUEUED", "Relay request accepted", fiber.Map{
		"job": current, "statusUrl": statusURL,
	}))
}

func (api *API) submitRelayJob(job relayJob, message *types.Email) (error, bool) {
	if message == nil {
		return fmt.Errorf("email is unavailable"), false
	}
	previous := job
	job, err := api.relayJobs.beginAttempt(job.ID)
	if err != nil {
		return fmt.Errorf("persist relay attempt: %w", err), false
	}
	var callbackHandled atomic.Bool
	callback := func(relayErr error) {
		callbackHandled.Store(true)
		if errors.Is(relayErr, outgoing.ErrClosed) {
			if restoreErr := api.relayJobs.restoreQueuedAfterShutdown(previous); restoreErr != nil {
				common.Error("Restore relay job %s after shutdown rejection: %v", job.ID, restoreErr)
			}
			return
		}
		api.finishRelayAttempt(job.ID, relayErr)
	}
	if job.RelayTo != "" {
		err = api.mailServer.RelayMailTo(message, job.RelayTo, callback)
	} else {
		err = api.mailServer.RelayMail(message, false, callback)
	}
	return err, callbackHandled.Load()
}

func (api *API) finishRelayAttempt(jobID string, relayErr error) {
	if relayErr == nil {
		api.relayJobs.complete(jobID, nil)
		return
	}
	job, ok := api.relayJobs.get(jobID)
	if ok && relayFailureIsRetryable(relayErr) && job.Attempts < defaultRelayMaxAttempts {
		delay := relayRetryDelay(job.Attempts)
		if _, queued := api.relayJobs.queueRetry(jobID, relayErr, delay); queued {
			time.AfterFunc(delay, func() { api.retryRelayJob(jobID) })
			return
		}
	}
	api.relayJobs.complete(jobID, relayErr)
	if ok {
		common.Error("Relay job %s failed for email %s after %d attempt(s) (category: %s)", job.ID, job.EmailID, job.Attempts, relayFailureCategory(relayErr))
	}
}

func relayFailureIsRetryable(err error) bool {
	switch relayFailureCategory(err) {
	case "connection", "timeout", "queue_full":
		return true
	default:
		return false
	}
}

func relayRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := defaultRelayRetryBaseDelay << (attempts - 1)
	spread := delay / 5
	if spread <= 0 {
		return delay
	}
	random, err := rand.Int(rand.Reader, big.NewInt(int64(spread*2+1)))
	if err != nil {
		return delay
	}
	return delay - spread + time.Duration(random.Int64())
}

func (api *API) retryRelayJob(jobID string) {
	job, ok := api.relayJobs.get(jobID)
	if !ok || job.CompletedAt != nil {
		return
	}
	if job.Attempts >= defaultRelayMaxAttempts {
		api.relayJobs.complete(jobID, errRelayAttemptsExhausted)
		return
	}
	email, err := api.mailServer.GetEmail(job.EmailID)
	if err == nil {
		var handled bool
		err, handled = api.submitRelayJob(job, email)
		if handled {
			return
		}
	}
	if errors.Is(err, outgoing.ErrClosed) {
		if restoreErr := api.relayJobs.restoreQueuedAfterShutdown(job); restoreErr != nil {
			common.Error("Restore relay job %s after shutdown rejection: %v", job.ID, restoreErr)
		}
		return
	}
	if err != nil {
		api.finishRelayAttempt(jobID, err)
	}
}

func (api *API) recoverRelayJobs() {
	for _, job := range api.relayJobs.queued() {
		if job.Attempts >= defaultRelayMaxAttempts {
			api.relayJobs.complete(job.ID, errRelayAttemptsExhausted)
			continue
		}
		if job.NextAttemptAt != nil {
			delay := time.Until(*job.NextAttemptAt)
			if delay > 0 {
				time.AfterFunc(delay, func() { api.retryRelayJob(job.ID) })
				continue
			}
		}
		email, err := api.mailServer.GetEmail(job.EmailID)
		if err == nil {
			var handled bool
			err, handled = api.submitRelayJob(job, email)
			if handled {
				continue
			}
		}
		if errors.Is(err, outgoing.ErrClosed) {
			if restoreErr := api.relayJobs.restoreQueuedAfterShutdown(job); restoreErr != nil {
				common.Error("Restore relay job %s after shutdown rejection: %v", job.ID, restoreErr)
			}
			continue
		}
		if err != nil {
			api.finishRelayAttempt(job.ID, err)
		}
	}
}

func (api *API) getRelayJob(c fiber.Ctx) error {
	job, ok := api.relayJobs.get(c.Params("jobID"))
	if !ok {
		return c.Status(http.StatusNotFound).JSON(ErrorResponse(ErrorCodeRelayFailed, "Relay job not found or expired"))
	}
	return c.JSON(SuccessResponse("RELAY_STATUS", "Relay job status", job))
}
