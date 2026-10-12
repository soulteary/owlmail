package api

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
)

// The queue owner maintains this cache independently of the store mutex. A
// blocked filesystem operation must never block HTTP or runtime snapshot reads.
type relayJobReadinessMonitor struct {
	api       *API
	directory string
	loadErr   error
	check     func(string) error

	mu                  sync.RWMutex
	storage             attachmentstore.HealthStatus
	persistenceFailures int
	persistenceEpoch    uint64
	capacityBlocked     bool
	capacityAvailableAt time.Time
	started             bool
	closed              bool
	stop                chan struct{}
	startOnce           sync.Once
	closeOnce           sync.Once
}

func newRelayJobReadinessMonitor(api *API) *relayJobReadinessMonitor {
	monitor := &relayJobReadinessMonitor{
		api: api, directory: api.relayJobs.directory, loadErr: api.relayJobsPersistenceErr,
		check:   checkRelayJobDirectory,
		storage: attachmentstore.HealthStatus{State: attachmentstore.HealthChecking, ErrorCategory: attachmentstore.HealthErrorPending},
		stop:    make(chan struct{}),
	}
	api.relayJobs.mu.Lock()
	api.relayJobs.readiness = monitor
	monitor.persistenceFailures = len(api.relayJobs.persistenceErrors)
	api.relayJobs.refreshReadinessCapacityLocked()
	api.relayJobs.mu.Unlock()
	return monitor
}

func (monitor *relayJobReadinessMonitor) Snapshot() attachmentstore.HealthStatus {
	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	status := monitor.storage
	switch {
	case monitor.closed:
		status.State, status.ErrorCategory = attachmentstore.HealthClosed, attachmentstore.HealthErrorClosed
	case monitor.loadErr != nil || monitor.persistenceFailures > 0:
		status.State, status.ErrorCategory = attachmentstore.HealthUnready, attachmentstore.HealthErrorUnavailable
	case !monitor.started:
		status.State, status.ErrorCategory = attachmentstore.HealthChecking, attachmentstore.HealthErrorPending
	case monitor.capacityBlocked && (monitor.capacityAvailableAt.IsZero() || time.Now().Before(monitor.capacityAvailableAt)):
		status.State, status.ErrorCategory = attachmentstore.HealthUnready, attachmentstore.HealthErrorUnavailable
	}
	return status
}

func (monitor *relayJobReadinessMonitor) recordPersistenceErrors(count int, err error) {
	monitor.mu.Lock()
	monitor.persistenceFailures = count
	if err != nil {
		monitor.persistenceEpoch++
		monitor.storage = attachmentstore.HealthStatus{State: attachmentstore.HealthUnready, ErrorCategory: attachmentstore.HealthErrorUnavailable, CheckedAt: time.Now()}
	}
	monitor.mu.Unlock()
}

// Full status retention is not necessarily backpressure: an expired completed
// record can be evicted by the next request. Pending/minimum-retention records
// can prevent acceptance and are reflected without mutating them in a probe.
func (store *relayJobStore) refreshReadinessCapacityLocked() {
	if store.readiness == nil {
		return
	}
	blocked := store.limit <= 0 || len(store.order) >= store.limit
	var availableAt time.Time
	if blocked && store.limit > 0 {
		for _, job := range store.jobs {
			if job.CompletedAt != nil && (availableAt.IsZero() || job.retainUntil.Before(availableAt)) {
				availableAt = job.retainUntil
				if availableAt.IsZero() {
					availableAt = time.Now().Add(-time.Nanosecond)
				}
			}
		}
	}
	store.readiness.mu.Lock()
	store.readiness.capacityBlocked, store.readiness.capacityAvailableAt = blocked, availableAt
	store.readiness.mu.Unlock()
}

func (monitor *relayJobReadinessMonitor) Start() {
	monitor.startOnce.Do(func() {
		monitor.mu.Lock()
		if monitor.closed {
			monitor.mu.Unlock()
			return
		}
		monitor.started = true
		monitor.mu.Unlock()
		go monitor.run()
	})
}

func (monitor *relayJobReadinessMonitor) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastProbe time.Time
	for {
		select {
		case <-monitor.stop:
			return
		default:
		}
		configuration := monitor.api.mailServer.GetOutgoingConfig()
		if configuration != nil && configuration.Host != "" && (lastProbe.IsZero() || time.Since(lastProbe) >= 30*time.Second) {
			monitor.probe()
			lastProbe = time.Now()
		}
		select {
		case <-monitor.stop:
			return
		case <-ticker.C:
		}
	}
}

func (monitor *relayJobReadinessMonitor) probe() {
	monitor.mu.Lock()
	if monitor.closed {
		monitor.mu.Unlock()
		return
	}
	monitor.storage = attachmentstore.HealthStatus{State: attachmentstore.HealthChecking, ErrorCategory: attachmentstore.HealthErrorPending, CheckedAt: time.Now()}
	epoch := monitor.persistenceEpoch
	monitor.mu.Unlock()
	if monitor.loadErr != nil {
		return // A failed load cannot be recovered by reopening the live queue.
	}
	err := monitor.check(monitor.directory)
	if err == nil {
		// Retry the authoritative in-memory records, not a new queue owner.
		// A successful scratch write alone must not erase an unresolved job error.
		store := monitor.api.relayJobs
		store.mu.Lock()
		select {
		case <-monitor.stop:
			store.mu.Unlock()
			return
		default:
		}
		store.pruneLocked(store.now().UTC())
		for id := range store.persistenceErrors {
			if job, exists := store.jobs[id]; exists {
				_ = store.persistLocked(job)
			} else {
				_ = store.removePersistedLocked(id)
			}
		}
		store.refreshReadinessCapacityLocked()
		store.mu.Unlock()
	}
	status := attachmentstore.HealthStatus{State: attachmentstore.HealthReady, CheckedAt: time.Now()}
	if err != nil {
		status.State, status.ErrorCategory = attachmentstore.HealthUnready, attachmentstore.HealthErrorUnavailable
		if errors.Is(err, os.ErrPermission) {
			status.ErrorCategory = attachmentstore.HealthErrorPermission
		} else if errors.Is(err, os.ErrNotExist) {
			status.ErrorCategory = attachmentstore.HealthErrorNotFound
		}
	}
	monitor.mu.Lock()
	if !monitor.closed && (err != nil || monitor.persistenceEpoch == epoch) {
		monitor.storage = status
	}
	monitor.mu.Unlock()
}

func checkRelayJobDirectory(directory string) error {
	file, err := os.CreateTemp(directory, ".relay-readiness-*")
	if err != nil {
		return err
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := file.Write([]byte("owlmail relay readiness\n")); err != nil {
		_ = file.Close()
		return err
	}
	if err := errors.Join(file.Sync(), file.Close()); err != nil {
		return err
	}
	committed := path + ".committed"
	defer func() { _ = os.Remove(committed) }()
	if err := os.Rename(path, committed); err != nil {
		return err
	}
	if err := syncRelayJobDirectory(directory); err != nil {
		return err
	}
	if err := os.Remove(committed); err != nil {
		return err
	}
	return syncRelayJobDirectory(directory)
}

// Filesystem syscalls cannot be canceled portably. One background owner may
// remain blocked, but closing the service and answering probes never join it.
func (monitor *relayJobReadinessMonitor) Close() error {
	monitor.closeOnce.Do(func() {
		monitor.mu.Lock()
		monitor.closed = true
		monitor.mu.Unlock()
		close(monitor.stop)
	})
	return nil
}
