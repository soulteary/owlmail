// Package relay owns durable SMTP delivery tasks independently of HTTP transports.
package relay

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

// Mailbox supplies low-level transport and source ownership, not task scheduling.
type Mailbox interface {
	GetEmail(string) (*types.Email, error)
	AcquireEmailSource(string) (*types.Email, func(), error)
	GetOutgoingConfig() *outgoing.OutgoingConfig
	EffectiveRelayRecipients(*types.Email) ([]string, error)
	RelayMail(*types.Email, bool, func(error)) error
	RelayMailTo(*types.Email, string, func(error)) error
	RelayMailConfirmed(*types.Email, []string, func(error)) error
	RecordRelaySuccess(string, time.Time, []string) error
}

// Request describes one durable delivery. Key makes handoff promotion idempotent.
type Request struct {
	DeferDispatch       bool
	EmailID             string
	RelayTo             string
	ConfirmedRecipients []string
	Origin              string
	Key                 string
	Envelope            *types.Envelope
}

// RequestID returns the stable job identifier for an idempotent handoff.
func RequestID(origin, key string) string {
	sum := sha256.Sum256([]byte(origin + "\x00" + key))
	return hex.EncodeToString(sum[:16])
}

// Options bounds task retention and supplies storage dependencies.
type Options struct {
	Limit            int
	TTL              time.Duration
	MinimumRetention time.Duration
	Now              func() time.Time
	NewID            func() (string, error)
	SyncDirectory    func(string) error
}

var (
	ErrCapacity          = errRelayJobCapacity
	ErrRecipientTooLong  = errRelayRecipientTooLong
	ErrAlreadyPending    = errRelayAlreadyPending
	ErrJobTooLarge       = errRelayJobTooLarge
	ErrSourceInUse       = errRelaySourceInUse
	ErrAttemptsExhausted = errRelayAttemptsExhausted
	ErrPersistence       = errRelayJobPersistence
	ErrRetained          = errRelayJobRetained
	ErrNotFound          = errors.New("relay job not found or expired")
)

const (
	Queued    = relayJobQueued
	Succeeded = relayJobSucceeded
	Failed    = relayJobFailed
)

// Service is the single task owner shared by every application entry point.
type Service struct {
	mailbox              Mailbox
	store                *relayJobStore
	loadErr              error
	readiness            *relayJobReadinessMonitor
	mu                   sync.Mutex
	started, closed      bool
	stop, done, wake     chan struct{}
	changed              chan struct{}
	active               map[string]bool
	leases               map[string]func()
	startOnce, closeOnce sync.Once
	receiptPending       atomic.Int64
	admissions           sync.WaitGroup
}

// New restores tasks without starting delivery. Malformed persistence fails closed.
func New(mailbox Mailbox, mailDirectory string) *Service {
	return NewWithOptions(mailbox, mailDirectory, Options{})
}

// NewWithOptions constructs a task owner with explicit retention/storage dependencies.
func NewWithOptions(mailbox Mailbox, mailDirectory string, options Options) *Service {
	store, err := newPersistentRelayJobStoreWithOptions(mailDirectory, options)
	if err != nil {
		store = newRelayJobStore()
	}
	service := &Service{mailbox: mailbox, store: store, loadErr: err, stop: make(chan struct{}), done: make(chan struct{}), wake: make(chan struct{}, 1), changed: make(chan struct{}), active: make(map[string]bool), leases: make(map[string]func())}
	service.readiness = newRelayJobReadinessMonitor(service)
	for _, job := range store.jobs {
		if job.ReceiptPending {
			service.receiptPending.Add(1)
		}
	}
	for _, job := range store.queued() {
		if _, err := service.acquireSource(job); err != nil {
			common.Error("Protect recovered relay source %s: %v", job.ID, err)
		}
	}
	return service
}

func applyOptions(store *relayJobStore, options Options) {
	if options.Limit != 0 {
		store.limit = options.Limit
	}
	if options.TTL != 0 {
		store.ttl = options.TTL
	}
	if options.MinimumRetention != 0 {
		store.minimumRetention = options.MinimumRetention
	}
	if options.Now != nil {
		store.now = options.Now
	}
	if options.NewID != nil {
		store.newID = options.NewID
	}
	if options.SyncDirectory != nil {
		store.syncDirectory = options.SyncDirectory
	}
}

func cloneEnvelope(envelope *types.Envelope) *types.Envelope {
	if envelope == nil {
		return nil
	}
	result := *envelope
	result.To = append([]string(nil), envelope.To...)
	result.CC = append([]string(nil), envelope.CC...)
	result.BCC = append([]string(nil), envelope.BCC...)
	result.CalculatedBCC = append([]string(nil), envelope.CalculatedBCC...)
	return &result
}
func cloneJob(job Job) Job {
	if job.CompletedAt != nil {
		value := *job.CompletedAt
		job.CompletedAt = &value
	}
	if job.NextAttemptAt != nil {
		value := *job.NextAttemptAt
		job.NextAttemptAt = &value
	}
	job.Recipients = append([]string(nil), job.Recipients...)
	job.Envelope = cloneEnvelope(job.Envelope)
	return job
}

// Submit durably accepts a task before scheduling. An unstarted service only persists.
func (s *Service) Submit(request Request) (Job, error) {
	if s == nil {
		return Job{}, ErrPersistence
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Job{}, outgoing.ErrClosed
	}
	s.admissions.Add(1)
	s.mu.Unlock()
	defer s.admissions.Done()
	request.EmailID = strings.Clone(request.EmailID)
	request.RelayTo = strings.Clone(request.RelayTo)
	request.Origin = strings.Clone(request.Origin)
	request.Key = strings.Clone(request.Key)
	if request.ConfirmedRecipients != nil {
		request.ConfirmedRecipients = append([]string{}, request.ConfirmedRecipients...)
	}
	for i, recipient := range request.ConfirmedRecipients {
		request.ConfirmedRecipients[i] = strings.Clone(recipient)
	}
	if s.loadErr != nil {
		return Job{}, fmt.Errorf("%w: %v", ErrPersistence, s.loadErr)
	}
	if request.Origin == "" {
		request.Origin = "native"
	}
	if request.Key != "" {
		if job, ok := s.Get(RequestID(request.Origin, request.Key)); ok {
			if job.EmailID != request.EmailID {
				return Job{}, fmt.Errorf("request key belongs to another email")
			}
			return job, nil
		}
	}
	if len(request.RelayTo) > defaultRelayRecipientMaxBytes {
		return Job{}, ErrRecipientTooLong
	}
	if request.ConfirmedRecipients != nil && len(request.ConfirmedRecipients) == 0 {
		return Job{}, fmt.Errorf("confirmed relay recipients cannot be empty")
	}
	email, release, err := s.mailbox.AcquireEmailSource(request.EmailID)
	if err != nil {
		return Job{}, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	if request.Envelope != nil {
		copy := *email
		copy.Envelope = cloneEnvelope(request.Envelope)
		email = &copy
	}
	configuration := s.mailbox.GetOutgoingConfig()
	if configuration == nil || configuration.Host == "" {
		release()
		return Job{}, outgoing.ErrNotConfigured
	}
	if err := configuration.Validate(); err != nil {
		release()
		return Job{}, err
	}
	var recipients []string
	if request.RelayTo != "" {
		address, err := mail.ParseAddress(request.RelayTo)
		if err != nil || address.Address != request.RelayTo {
			release()
			return Job{}, fmt.Errorf("invalid relay address")
		}
		recipients = []string{request.RelayTo}
	} else {
		recipients, err = s.mailbox.EffectiveRelayRecipients(email)
		if err != nil {
			release()
			return Job{}, err
		}
		if request.ConfirmedRecipients != nil && !reflect.DeepEqual(request.ConfirmedRecipients, recipients) {
			release()
			return Job{}, outgoing.ErrConfigChanged
		}
	}
	if len(recipients) == 0 {
		release()
		return Job{}, fmt.Errorf("email had no recipients")
	}
	request.ConfirmedRecipients = append([]string(nil), recipients...)
	if request.Envelope == nil {
		request.Envelope = cloneEnvelope(email.Envelope)
	}
	if request.Origin == "" {
		request.Origin = "native"
	}
	job, err := s.store.createRequest(request)
	retained := errors.Is(err, ErrRetained)
	if err != nil && !retained {
		release()
		return Job{}, err
	}
	s.mu.Lock()
	if previous := s.leases[job.ID]; previous != nil {
		release()
	} else {
		s.leases[job.ID] = release
	}
	s.signalLocked()
	s.mu.Unlock()
	s.notify()
	return cloneJob(job), nil
}

// CommitHandoff acknowledges a durable producer handoff before dispatch.
func (s *Service) CommitHandoff(id string) error {
	s.store.mu.Lock()
	job, ok := s.store.jobs[id]
	if !ok {
		s.store.mu.Unlock()
		return ErrNotFound
	}
	if !job.HandoffPending {
		s.store.mu.Unlock()
		return nil
	}
	job.HandoffPending = false
	job.UpdatedAt = s.store.now()
	if err := s.store.persistLocked(job); err != nil {
		s.store.mu.Unlock()
		return err
	}
	s.store.jobs[id] = job
	s.store.mu.Unlock()
	s.signal()
	s.notify()
	return nil
}

// Start enables recovery and dispatch once. No delivery runs before this gate.
func (s *Service) Start() {
	s.startOnce.Do(func() {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		s.started = true
		s.mu.Unlock()
		s.readiness.Start()
		go s.run()
	})
}
func (s *Service) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *Service) signalLocked() { close(s.changed); s.changed = make(chan struct{}) }
func (s *Service) signal()       { s.mu.Lock(); s.signalLocked(); s.mu.Unlock() }
func (s *Service) run() {
	defer close(s.done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.dispatch()
		select {
		case <-s.stop:
			return
		case <-s.wake:
		case <-ticker.C:
		}
	}
}
func (s *Service) dispatch() {
	s.repairReceipts()
	configuration := s.mailbox.GetOutgoingConfig()
	if configuration == nil || configuration.Host == "" {
		return
	}
	for _, job := range s.store.queued() {
		if job.HandoffPending {
			continue
		}
		s.mu.Lock()
		if s.closed || len(s.active) >= 8 {
			s.mu.Unlock()
			return
		}
		if s.active[job.ID] {
			s.mu.Unlock()
			continue
		}
		if job.NextAttemptAt != nil && time.Now().Before(*job.NextAttemptAt) {
			s.mu.Unlock()
			continue
		}
		s.active[job.ID] = true
		s.mu.Unlock()
		if job.Attempts >= defaultRelayMaxAttempts {
			s.finish(job, ErrAttemptsExhausted)
			continue
		}
		email, err := s.acquireSource(job)
		if err != nil {
			s.finish(job, err)
			continue
		}
		previous := job
		job, err = s.store.beginAttempt(job.ID)
		if err != nil {
			s.deactivate(previous.ID)
			continue
		}
		var callbackOnce sync.Once
		callback := func(err error) {
			callbackOnce.Do(func() {
				if errors.Is(err, outgoing.ErrClosed) || errors.Is(err, outgoing.ErrNotConfigured) {
					_ = s.store.restoreQueuedAfterShutdown(previous)
					s.deactivate(job.ID)
					return
				}
				s.finish(job, err)
			})
		}

		// An explicit override freezes that address; confirmed envelope delivery
		// revalidates the current rule-filtered list before entering the transport.
		if job.RelayTo != "" {
			err = s.mailbox.RelayMailTo(email, job.RelayTo, callback)
		} else if job.Confirmed {
			err = s.mailbox.RelayMailConfirmed(email, job.Recipients, callback)
		} else {
			err = s.mailbox.RelayMail(email, false, callback)
		}
		// Outgoing's rejection path invokes callback synchronously. Its accepted
		// path invokes callback exactly once, so err must not finish a second time.
		if err != nil {
			callback(err)
		}
	}
}
func (s *Service) deactivate(id string) {
	s.mu.Lock()
	delete(s.active, id)
	s.signalLocked()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		s.release(id)
	}
	s.notify()
}
func (s *Service) release(id string) {
	s.mu.Lock()
	release := s.leases[id]
	delete(s.leases, id)
	s.mu.Unlock()
	if release != nil {
		release()
	}
}
func (s *Service) acquireSource(job Job) (*types.Email, error) {
	s.mu.Lock()
	tracked := s.leases[job.ID] != nil
	s.mu.Unlock()
	var email *types.Email
	var err error
	if tracked {
		email, err = s.mailbox.GetEmail(job.EmailID)
	} else {
		var release func()
		email, release, err = s.mailbox.AcquireEmailSource(job.EmailID)
		if err == nil {
			s.mu.Lock()
			if s.leases[job.ID] == nil {
				s.leases[job.ID] = release
			} else {
				release()
			}
			s.mu.Unlock()
		}
	}
	if err != nil {
		return nil, fmt.Errorf("open email source: %w", err)
	}
	copy := *email
	if job.Envelope != nil {
		copy.Envelope = cloneEnvelope(job.Envelope)
	}
	return &copy, nil
}
func (s *Service) finish(job Job, err error) {
	if err != nil && relayFailureIsRetryable(err) && job.Attempts < defaultRelayMaxAttempts {
		if _, ok := s.store.queueRetry(job.ID, err, relayRetryDelay(job.Attempts)); ok {
			s.deactivate(job.ID)
			return
		}
	}
	if err == nil {
		s.store.mu.Lock()
		current, ok := s.store.jobs[job.ID]
		if ok {
			if !current.ReceiptPending {
				s.receiptPending.Add(1)
			}
			current.ReceiptPending = true
			s.store.jobs[job.ID] = current
		}
		s.store.mu.Unlock()
	}
	s.store.complete(job.ID, err)
	s.release(job.ID)
	s.deactivate(job.ID)

}
func (s *Service) repairReceipts() {
	s.store.mu.Lock()
	jobs := make([]Job, 0)
	for _, job := range s.store.jobs {
		if job.Status == Succeeded && job.ReceiptPending && job.CompletedAt != nil {
			jobs = append(jobs, cloneJob(job))
		}
	}
	s.store.mu.Unlock()
	for _, job := range jobs {
		_, sourceErr := s.mailbox.GetEmail(job.EmailID)
		if sourceErr == nil {
			if err := s.mailbox.RecordRelaySuccess(job.EmailID, *job.CompletedAt, job.Recipients); err != nil {
				continue
			}
		}
		s.store.mu.Lock()
		current, ok := s.store.jobs[job.ID]
		if ok && current.ReceiptPending {
			current.ReceiptPending = false
			if err := s.store.persistLocked(current); err == nil {
				s.store.jobs[job.ID] = current
				s.receiptPending.Add(-1)
			}
		}
		s.store.mu.Unlock()
	}
}

// Get returns a detached status snapshot.
func (s *Service) Get(id string) (Job, bool) {
	if s == nil {
		return Job{}, false
	}
	return s.store.get(id)
}

// List returns newest matching jobs and whether additional records exist.
func (s *Service) List(status, emailID string, limit int) ([]Job, bool) {
	if s == nil {
		return []Job{}, false
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	s.store.pruneLocked(s.store.now().UTC())
	jobs := make([]Job, 0)
	more := false
	for i := len(s.store.order) - 1; i >= 0; i-- {
		job := s.store.jobs[s.store.order[i]]
		if status != "" && job.Status != status || emailID != "" && job.EmailID != emailID {
			continue
		}
		if len(jobs) == limit {
			more = true
			break
		}
		jobs = append(jobs, cloneJob(job))
	}
	return jobs, more
}

// Wait observes durable completion. Canceling a waiter does not cancel delivery.
func (s *Service) Wait(ctx context.Context, id string) (Job, error) {
	if s == nil {
		return Job{}, ErrPersistence
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		s.mu.Lock()
		changed := s.changed
		closed := s.closed
		s.mu.Unlock()
		job, ok := s.Get(id)
		if !ok {
			return Job{}, ErrNotFound
		}
		if job.CompletedAt != nil {
			if job.Status == Succeeded {
				return job, nil
			}
			return job, fmt.Errorf("relay failed: %s", job.ErrorCategory)
		}
		if closed {
			return job, outgoing.ErrClosed
		}
		select {
		case <-ctx.Done():
			return job, ctx.Err()
		case <-changed:
		}
	}
}

// Retry creates a new durable task from a failed record and its frozen targets.
func (s *Service) Retry(id string) (Job, error) {
	job, ok := s.Get(id)
	if !ok {
		return Job{}, ErrNotFound
	}
	if job.Status != Failed {
		return Job{}, fmt.Errorf("only failed relay jobs may be replayed")
	}
	return s.Submit(Request{EmailID: job.EmailID, RelayTo: job.RelayTo, ConfirmedRecipients: job.Recipients, Origin: job.Origin, Envelope: job.Envelope})
}

// ProtectSourceDeletion serializes source cleanup against accepted jobs.
func (s *Service) ProtectSourceDeletion(ids []string, remove func() error) error {
	if s == nil || s.loadErr != nil {
		return fmt.Errorf("%w: persisted relay work is unavailable", ErrSourceInUse)
	}
	return s.store.protectSourceDeletion(ids, remove)
}

// PersistenceError reports a load or unresolved durable-storage failure.
func (s *Service) PersistenceError() error {
	if s == nil {
		return ErrPersistence
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	return errors.Join(s.loadErr, s.store.lastPersistenceErr)
}

// Snapshot returns cached readiness without waiting for filesystem operations.
func (s *Service) Snapshot() attachmentstore.HealthStatus {
	if s == nil {
		return attachmentstore.HealthStatus{State: attachmentstore.HealthUnready, ErrorCategory: attachmentstore.HealthErrorUnavailable}
	}
	status := s.readiness.Snapshot()
	pending := s.receiptPending.Load() > 0
	if pending {
		status.State = attachmentstore.HealthUnready
		status.ErrorCategory = attachmentstore.HealthErrorUnavailable
	}
	return status
}

// Close stops scheduling and releases source leases without deleting queued work.
func (s *Service) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		started := s.started
		close(s.stop)
		s.signalLocked()
		s.mu.Unlock()
		_ = s.readiness.Close()
		s.admissions.Wait()
		if started {
			<-s.done
		}
		s.mu.Lock()
		leases := make(map[string]func())
		for id, release := range s.leases {
			if !s.active[id] {
				leases[id] = release
				delete(s.leases, id)
			}
		}
		s.mu.Unlock()
		for _, release := range leases {
			release()
		}
	})
	return nil
}

func relayFailureIsRetryable(err error) bool {
	switch relayFailureCategory(err) {
	case "connection", "timeout", "queue_full", "temporary_smtp":
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

// CheckPersistence runs an explicit storage probe; normal readiness reads remain cached.
func (s *Service) CheckPersistence() {
	if s != nil {
		s.readiness.probe()
	}
}
