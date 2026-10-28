package relay

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

var (
	errRelayJobCapacity       = errors.New("relay job status capacity reached")
	errRelayRecipientTooLong  = errors.New("relay recipient exceeds size limit")
	errRelayAlreadyPending    = errors.New("email already has a pending relay job")
	errRelayJobTooLarge       = errors.New("relay job exceeds persistence size limit")
	errRelaySourceInUse       = errors.New("email has a pending relay job")
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

type Job struct {
	HandoffPending bool            `json:"handoffPending,omitempty"`
	Envelope       *types.Envelope `json:"-"`
	Origin         string          `json:"origin,omitempty"`
	Key            string          `json:"key,omitempty"`
	ReceiptPending bool            `json:"receiptPending,omitempty"`
	ID             string          `json:"id"`
	EmailID        string          `json:"emailId"`
	RelayTo        string          `json:"relayTo,omitempty"`
	Recipients     []string        `json:"confirmedRecipients,omitempty"`
	Confirmed      bool            `json:"recipientsConfirmed,omitempty"`
	Status         string          `json:"status"`
	ErrorCategory  string          `json:"errorCategory,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
	Attempts       int             `json:"attempts"`
	NextAttemptAt  *time.Time      `json:"nextAttemptAt,omitempty"`
	retainUntil    time.Time
}

type relayJob = Job

// protectSourceDeletion serializes acceptance and deletion so a relay cannot
// become pending after the deletion check but before its source is removed.
// A nil ID set protects deletion of the entire mailbox.
func (store *relayJobStore) protectSourceDeletion(ids []string, deleteSource func() error) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.pruneLocked(store.now().UTC())
	targets := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		targets[id] = struct{}{}
	}
	for _, job := range store.jobs {
		if job.CompletedAt != nil {
			continue
		}
		if ids == nil {
			return errRelaySourceInUse
		}
		if _, targeted := targets[job.EmailID]; targeted {
			return errRelaySourceInUse
		}
	}
	return deleteSource()
}

type relayJobStore struct {
	mu                 sync.Mutex
	jobs               map[string]relayJob
	order              []string
	now                func() time.Time
	newID              func() (string, error)
	ttl                time.Duration
	limit              int
	minimumRetention   time.Duration
	directory          string
	syncDirectory      func(string) error
	lastPersistenceErr error
	persistenceErrors  map[string]error
	readiness          *relayJobReadinessMonitor
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
	return store.createConfirmed(emailID, relayTo, nil)
}

func (store *relayJobStore) createConfirmed(emailID, relayTo string, recipients []string) (relayJob, error) {
	return store.createRequest(Request{EmailID: emailID, RelayTo: relayTo, ConfirmedRecipients: recipients})
}

func (store *relayJobStore) createRequest(request Request) (relayJob, error) {
	emailID, relayTo, recipients := request.EmailID, request.RelayTo, request.ConfirmedRecipients
	if len(relayTo) > defaultRelayRecipientMaxBytes {
		return relayJob{}, errRelayRecipientTooLong
	}
	id, err := store.newID()
	if request.Key != "" {
		id, err = RequestID(request.Origin, request.Key), nil
	}
	if err != nil {
		return relayJob{}, err
	}
	now := store.now().UTC()
	job := relayJob{HandoffPending: request.DeferDispatch, Origin: request.Origin, Key: request.Key, Envelope: cloneEnvelope(request.Envelope), ID: id, EmailID: emailID, RelayTo: relayTo, Status: relayJobQueued, CreatedAt: now, UpdatedAt: now}
	if recipients != nil {
		job.Recipients = append([]string(nil), recipients...)
		job.Confirmed = true
	}
	if store.directory != "" {
		if err := validateRelayJobPersistenceBudget(job); err != nil {
			return relayJob{}, err
		}
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	defer store.refreshReadinessCapacityLocked()
	store.pruneLocked(now)
	if request.Key != "" {
		if existing, ok := store.jobs[id]; ok {
			if existing.EmailID != emailID {
				return relayJob{}, fmt.Errorf("request key belongs to another email")
			}
			return cloneJob(existing), nil
		}
	}
	for _, existing := range store.jobs {
		if existing.EmailID == emailID && existing.CompletedAt == nil {
			return relayJob{}, errRelayAlreadyPending
		}
	}
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
	defer store.refreshReadinessCapacityLocked()
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
	defer store.refreshReadinessCapacityLocked()
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
	return cloneJob(job), ok
}

func (store *relayJobStore) pruneLocked(now time.Time) {
	kept := store.order[:0]
	for _, id := range store.order {
		job, ok := store.jobs[id]
		if !ok {
			continue
		}
		if job.CompletedAt != nil && !job.ReceiptPending && now.Sub(*job.CompletedAt) > store.ttl {
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
			if job.CompletedAt != nil && !job.ReceiptPending && !now.Before(job.retainUntil) {
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
	var smtpError *textproto.Error
	if errors.As(err, &smtpError) {
		if smtpError.Code >= 400 && smtpError.Code < 500 {
			return "temporary_smtp"
		}
		if smtpError.Code >= 500 && smtpError.Code < 600 {
			return "permanent_smtp"
		}
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "queue is full"):
		return "queue_full"
	case strings.Contains(message, "not configured"):
		return "not_configured"
	case errors.Is(err, outgoing.ErrConfigChanged):
		return "configuration_changed"
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
