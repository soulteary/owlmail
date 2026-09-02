package attachmentstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/aws/smithy-go"
)

// MonitoredStore decorates a health-capable Store with a background probe and
// a cached readiness result. Store operations retain their original behavior.
type MonitoredStore struct {
	Store
	checker  HealthChecker
	interval time.Duration
	timeout  time.Duration

	mu     sync.RWMutex
	status HealthStatus
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// NewMonitoredStore starts a background probe. The first probe runs
// asynchronously so non-strict startup remains compatible when S3 is down.
func NewMonitoredStore(store Store, interval, timeout time.Duration, initial ...HealthStatus) (*MonitoredStore, error) {
	if store == nil {
		return nil, fmt.Errorf("attachment store cannot be nil")
	}
	checker, ok := store.(HealthChecker)
	if !ok {
		return nil, fmt.Errorf("attachment store does not support health checks")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("attachment health check interval must be positive")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("attachment health check timeout must be positive")
	}
	ctx, cancel := context.WithCancel(context.Background())
	status := HealthStatus{Category: HealthChecking}
	if len(initial) > 0 {
		status = initial[0]
	}
	monitored := &MonitoredStore{
		Store:    store,
		checker:  checker,
		interval: interval,
		timeout:  timeout,
		status:   status,
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	go monitored.run(ctx)
	return monitored, nil
}

func (store *MonitoredStore) run(ctx context.Context) {
	defer close(store.done)
	store.probe(ctx)
	ticker := time.NewTicker(store.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			store.probe(ctx)
		}
	}
}

func (store *MonitoredStore) probe(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, store.timeout)
	err := store.checker.CheckHealth(ctx)
	cancel()
	status := HealthStatus{Ready: err == nil, Category: ClassifyHealthError(err)}
	store.mu.Lock()
	store.status = status
	store.mu.Unlock()
}

// Readiness returns the most recent probe result without contacting S3.
func (store *MonitoredStore) Readiness() HealthStatus {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.status
}

// Close stops future probes and waits for an in-flight probe to observe
// cancellation. It is safe to call more than once.
func (store *MonitoredStore) Close() error {
	store.once.Do(func() {
		store.cancel()
		<-store.done
		store.mu.Lock()
		store.status = HealthStatus{Category: HealthClosed}
		store.mu.Unlock()
		if closer, ok := store.Store.(io.Closer); ok {
			_ = closer.Close()
		}
	})
	return nil
}

// CheckHealth performs one bounded caller-controlled probe and returns only a
// categorized result. Callers must supply a context with an appropriate
// deadline when using this for strict startup.
func CheckHealth(ctx context.Context, store Store) HealthStatus {
	checker, ok := store.(HealthChecker)
	if !ok {
		return HealthStatus{Ready: true, Category: HealthUnsupported}
	}
	err := checker.CheckHealth(ctx)
	return HealthStatus{Ready: err == nil, Category: ClassifyHealthError(err)}
}

// ClassifyHealthError maps provider errors to a stable, disclosure-safe
// category suitable for readiness responses and startup messages.
func ClassifyHealthError(err error) string {
	if err == nil {
		return HealthOK
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return HealthTimeout
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch strings.ToLower(apiErr.ErrorCode()) {
		case "accessdenied", "forbidden", "invalidaccesskeyid", "signaturedoesnotmatch", "expiredtoken", "invalidtoken":
			return HealthPermissionDenied
		case "nosuchbucket", "notfound", "404":
			return HealthNotFound
		case "requesttimeout", "requesttimeoutexception", "timeout":
			return HealthTimeout
		}
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) {
		if networkErr.Timeout() {
			return HealthTimeout
		}
		return HealthNetwork
	}
	return HealthUnavailable
}
