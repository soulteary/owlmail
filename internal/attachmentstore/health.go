package attachmentstore

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// HealthState is the cached availability state of an attachment store.
type HealthState string

const (
	HealthChecking HealthState = "checking"
	HealthReady    HealthState = "ready"
	HealthUnready  HealthState = "unready"
	HealthClosed   HealthState = "closed"
)

// HealthErrorCategory is a safe, bounded classification for probe failures.
// Raw SDK errors are deliberately kept out of readiness responses.
type HealthErrorCategory string

const (
	HealthErrorNone        HealthErrorCategory = ""
	HealthErrorPending     HealthErrorCategory = "pending"
	HealthErrorPermission  HealthErrorCategory = "permission"
	HealthErrorCredentials HealthErrorCategory = "credentials"
	HealthErrorNotFound    HealthErrorCategory = "not_found"
	HealthErrorTimeout     HealthErrorCategory = "timeout"
	HealthErrorUnavailable HealthErrorCategory = "unavailable"
	HealthErrorUnknown     HealthErrorCategory = "unknown"
	HealthErrorClosed      HealthErrorCategory = "closed"
)

// HealthStatus is safe to expose through operational readiness endpoints.
type HealthStatus struct {
	State         HealthState         `json:"status"`
	ErrorCategory HealthErrorCategory `json:"error_category,omitempty"`
	CheckedAt     time.Time           `json:"checked_at,omitempty"`
}

// Ready reports whether the most recent cached probe succeeded.
func (status HealthStatus) Ready() bool {
	return status.State == HealthReady
}

// ReadinessProvider supplies cached health without performing remote I/O.
type ReadinessProvider interface {
	Snapshot() HealthStatus
}

// HealthMonitor probes a Store in the background and caches the most recent
// result. Snapshot never performs remote I/O.
type HealthMonitor struct {
	store    Store
	interval time.Duration
	timeout  time.Duration

	mu      sync.RWMutex
	status  HealthStatus
	started bool
	closed  bool
	cancel  context.CancelFunc
	done    chan struct{}
	probeMu sync.Mutex
}

// NewHealthMonitor builds a stopped monitor. Start begins the initial
// asynchronous probe; ProbeNow can be used first for strict startup checks.
func NewHealthMonitor(store Store, interval, timeout time.Duration) (*HealthMonitor, error) {
	if store == nil {
		return nil, fmt.Errorf("attachment store cannot be nil")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("attachment health interval must be positive")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("attachment health timeout must be positive")
	}
	return &HealthMonitor{
		store: store, interval: interval, timeout: timeout,
		status: HealthStatus{State: HealthChecking, ErrorCategory: HealthErrorPending},
		done:   make(chan struct{}),
	}, nil
}

// Start begins background probes. It is safe to call more than once.
func (monitor *HealthMonitor) Start(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	monitor.mu.Lock()
	if monitor.started || monitor.closed {
		monitor.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	monitor.started = true
	monitor.cancel = cancel
	monitor.mu.Unlock()
	go monitor.run(ctx)
}

func (monitor *HealthMonitor) run(ctx context.Context) {
	defer close(monitor.done)
	monitor.probe(ctx)
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			monitor.probe(ctx)
		}
	}
}

// ProbeNow performs one bounded probe and updates the cache. Startup code uses
// this method only when the explicit strict-startup option is enabled.
func (monitor *HealthMonitor) ProbeNow(parent context.Context) HealthStatus {
	if parent == nil {
		parent = context.Background()
	}
	return monitor.probe(parent)
}

func (monitor *HealthMonitor) probe(parent context.Context) HealthStatus {
	monitor.probeMu.Lock()
	defer monitor.probeMu.Unlock()

	ctx, cancel := context.WithTimeout(parent, monitor.timeout)
	err := monitor.store.CheckHealth(ctx)
	cancel()
	status := HealthStatus{State: HealthReady, CheckedAt: time.Now().UTC()}
	if err != nil {
		status.State = HealthUnready
		status.ErrorCategory = classifyHealthError(err)
	}
	monitor.mu.Lock()
	if !monitor.closed {
		monitor.status = status
	}
	monitor.mu.Unlock()
	return status
}

// Snapshot returns the most recent cached result without remote I/O.
func (monitor *HealthMonitor) Snapshot() HealthStatus {
	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	return monitor.status
}

// Close cancels an in-flight probe and stops future probes.
func (monitor *HealthMonitor) Close() error {
	monitor.mu.Lock()
	if monitor.closed {
		monitor.mu.Unlock()
		return nil
	}
	monitor.closed = true
	cancel := monitor.cancel
	started := monitor.started
	monitor.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if started {
		<-monitor.done
	}
	monitor.mu.Lock()
	monitor.status = HealthStatus{
		State: HealthClosed, ErrorCategory: HealthErrorClosed, CheckedAt: time.Now().UTC(),
	}
	monitor.mu.Unlock()
	return nil
}

type codedError interface {
	ErrorCode() string
}

func classifyHealthError(err error) HealthErrorCategory {
	if err == nil {
		return HealthErrorNone
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return HealthErrorTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return HealthErrorTimeout
		}
		return HealthErrorUnavailable
	}
	var apiErr codedError
	if errors.As(err, &apiErr) {
		switch strings.ToLower(apiErr.ErrorCode()) {
		case "accessdenied", "forbidden", "allaccessdisabled":
			return HealthErrorPermission
		case "invalidaccesskeyid", "signaturedoesnotmatch", "expiredtoken", "invalidtoken", "tokenrefreshrequired", "requesttimetoolskewed":
			return HealthErrorCredentials
		case "nosuchbucket", "notfound":
			return HealthErrorNotFound
		case "requesttimeout", "requesttimeoutexception":
			return HealthErrorTimeout
		case "internalerror", "serviceunavailable", "slowdown":
			return HealthErrorUnavailable
		}
	}
	if errors.Is(err, context.Canceled) {
		return HealthErrorUnavailable
	}
	return HealthErrorUnknown
}
