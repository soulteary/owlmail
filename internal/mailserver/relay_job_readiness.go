package mailserver

import (
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
)

// SetRelayJobReadinessProvider registers the durable queue owner's cached health.
// The SMTP server never opens the queue or its records on behalf of a probe.
func (ms *MailServer) SetRelayJobReadinessProvider(provider attachmentstore.ReadinessProvider) {
	ms.readinessMutex.Lock()
	ms.relayJobHealth = provider
	ms.relayJobHealthRequired = true
	ms.readinessMutex.Unlock()
}

// GetRelayJobHealth omits the dependency when outgoing relay is not configured.
func (ms *MailServer) GetRelayJobHealth() (attachmentstore.HealthStatus, bool) {
	configuration := ms.GetOutgoingConfig()
	if configuration == nil || configuration.Host == "" {
		return attachmentstore.HealthStatus{}, false
	}
	ms.readinessMutex.RLock()
	provider := ms.relayJobHealth
	required := ms.relayJobHealthRequired
	ms.readinessMutex.RUnlock()
	if provider == nil {
		return attachmentstore.HealthStatus{State: attachmentstore.HealthChecking, ErrorCategory: attachmentstore.HealthErrorPending}, required
	}
	status := provider.Snapshot()
	if ms.shuttingDown.Load() {
		status.State, status.ErrorCategory = attachmentstore.HealthClosed, attachmentstore.HealthErrorClosed
	} else {
		status = freshReadinessStatus(status, time.Now())
	}
	return status, true
}
