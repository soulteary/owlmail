package mailserver

import (
	"context"
	"errors"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/outgoing"
)

const outgoingReadinessInterval = 30 * time.Second
const outgoingReadinessTimeout = 5 * time.Second

// Called while outgoingMutex is held. A result for a previous configuration
// must not make the newly configured transport ready.
func (ms *MailServer) invalidateOutgoingReadiness() {
	ms.outgoingGeneration++
	ms.readinessMutex.Lock()
	ms.outgoingTransportHealth = attachmentstore.HealthStatus{State: attachmentstore.HealthChecking, ErrorCategory: attachmentstore.HealthErrorPending}
	ms.readinessMutex.Unlock()
	select {
	case ms.outgoingReadinessWake <- struct{}{}:
	default:
	}
}

func (ms *MailServer) probeOutgoingReadiness(parent context.Context) {
	ms.outgoingMutex.RLock()
	generation := ms.outgoingGeneration
	var cfg *outgoing.OutgoingConfig
	if ms.outgoing != nil && !ms.outgoingClosed {
		cfg, _ = ms.outgoing.GetConfig().(*outgoing.OutgoingConfig)
	}
	ms.outgoingMutex.RUnlock()
	if cfg == nil || cfg.Host == "" {
		return
	}
	ctx, cancel := context.WithTimeout(parent, outgoingReadinessTimeout)
	err := outgoing.ProbeSMTP(ctx, cfg)
	cancel()
	status := attachmentstore.HealthStatus{State: attachmentstore.HealthReady, CheckedAt: time.Now()}
	if err != nil {
		status.State = attachmentstore.HealthUnready
		status.ErrorCategory = attachmentstore.HealthErrorUnavailable
		if errors.Is(err, context.DeadlineExceeded) {
			status.ErrorCategory = attachmentstore.HealthErrorTimeout
		}
	}
	ms.outgoingMutex.RLock()
	defer ms.outgoingMutex.RUnlock()
	if generation != ms.outgoingGeneration || ms.outgoingClosed || parent.Err() != nil {
		return
	}
	ms.readinessMutex.Lock()
	ms.outgoingTransportHealth = status
	ms.readinessMutex.Unlock()
}

// One owner performs bounded probes; readiness requests only read its cache.
func (ms *MailServer) startOutgoingReadiness() func() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ms.probeOutgoingReadiness(ctx)
		ticker := time.NewTicker(outgoingReadinessInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-ms.outgoingReadinessWake:
			}
			ms.probeOutgoingReadiness(ctx)
		}
	}()
	return cancel
}
