package mailserver

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/outgoing"
)

const runtimeReadinessPrefix = ".owlmail-runtime-readiness-"

// RuntimeReadinessPath keeps independent listeners in a shared directory separate.
func RuntimeReadinessPath(directory, host string, port int) string {
	identity := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", host, port)))
	return filepath.Join(directory, fmt.Sprintf("%s%x.json", runtimeReadinessPrefix, identity[:8]))
}

const RuntimeReadinessMaxAge = 5 * time.Second
const serviceReadinessMaxAge = 45 * time.Second

// RuntimeReadiness is a credential-free snapshot published by the SMTP owner.
// It describes cached health, not final delivery or index integrity.
type RuntimeReadiness struct {
	Version    int                           `json:"version"`
	PID        int                           `json:"pid"`
	Host       string                        `json:"host"`
	Port       int                           `json:"port"`
	UpdatedAt  time.Time                     `json:"updated_at"`
	SMTP       bool                          `json:"smtp"`
	Storage    attachmentstore.HealthStatus  `json:"mail_storage"`
	Attachment *attachmentstore.HealthStatus `json:"attachment_store,omitempty"`
	Relay      *outgoing.RuntimeReadiness    `json:"relay,omitempty"`
	RelayJobs  *attachmentstore.HealthStatus `json:"relay_job_storage,omitempty"`
	Transport  *attachmentstore.HealthStatus `json:"outgoing_transport,omitempty"`
}

func (status RuntimeReadiness) Validate(host string, port int, now time.Time) error {
	if status.Version != 1 || status.PID <= 0 || status.Host != host || status.Port != port {
		return fmt.Errorf("runtime readiness does not match the configured SMTP service")
	}
	age := now.Sub(status.UpdatedAt)
	if status.UpdatedAt.IsZero() || age < -time.Second || age > RuntimeReadinessMaxAge {
		return fmt.Errorf("runtime readiness is stale or unavailable")
	}
	return status.ValidateService(now)
}

// ValidateService applies the same dependency policy to HTTP readiness and
// snapshots. Snapshot ownership and publication freshness are checked by Validate.
func (status RuntimeReadiness) ValidateService(now time.Time) error {
	if !status.SMTP {
		return fmt.Errorf("runtime SMTP listeners are not ready")
	}
	if !status.Storage.Ready() {
		return fmt.Errorf("runtime mail storage is not writable")
	}
	if !readinessFresh(status.Storage, now) {
		return fmt.Errorf("runtime mail storage check is stale")
	}
	if status.Attachment != nil && !status.Attachment.Ready() {
		return fmt.Errorf("runtime attachment storage is not ready")
	}
	if status.Relay != nil && !status.Relay.Ready() {
		return fmt.Errorf("runtime relay workers or queue are not ready")
	}
	if status.RelayJobs != nil && (!status.RelayJobs.Ready() || !readinessFresh(*status.RelayJobs, now)) {
		return fmt.Errorf("runtime durable relay storage is not ready")
	}
	if (status.Relay != nil && status.Relay.Enabled && status.Transport == nil) || (status.Transport != nil && (!status.Transport.Ready() || !readinessFresh(*status.Transport, now))) {
		return fmt.Errorf("runtime outgoing SMTP transport or authentication is not ready")
	}
	return nil
}

func readinessFresh(status attachmentstore.HealthStatus, now time.Time) bool {
	age := now.Sub(status.CheckedAt)
	return !status.CheckedAt.IsZero() && age >= -time.Second && age <= serviceReadinessMaxAge
}

func freshReadinessStatus(status attachmentstore.HealthStatus, now time.Time) attachmentstore.HealthStatus {
	if status.State == "" {
		status.State = attachmentstore.HealthChecking
		status.ErrorCategory = attachmentstore.HealthErrorPending
	} else if status.Ready() && !readinessFresh(status, now) {
		status.State = attachmentstore.HealthUnready
		status.ErrorCategory = attachmentstore.HealthErrorTimeout
	}
	return status
}

// GetRuntimeReadiness returns credential-free cached state for either mode.
// It never touches disk or connects to a dependency.
func (ms *MailServer) GetRuntimeReadiness() RuntimeReadiness {
	smtp, storage := ms.GetServiceReadiness()
	now := time.Now()
	result := RuntimeReadiness{Version: 1, PID: os.Getpid(), Host: ms.host, Port: ms.GetPort(), UpdatedAt: now, SMTP: smtp, Storage: freshReadinessStatus(storage, now)}
	if attachment, enabled := ms.GetAttachmentHealth(); enabled {
		result.Attachment = &attachment
	}
	if relayJobs, enabled := ms.GetRelayJobHealth(); enabled {
		result.RelayJobs = &relayJobs
	}
	ms.outgoingMutex.RLock()
	relay := ms.outgoing
	closed := ms.outgoingClosed
	defer ms.outgoingMutex.RUnlock()
	if relay != nil {
		status := outgoing.RuntimeReadiness{Closed: true}
		if provider, ok := relay.(interface {
			RuntimeReadiness() outgoing.RuntimeReadiness
		}); ok {
			status = provider.RuntimeReadiness()
		}
		status.Closed = status.Closed || closed
		result.Relay = &status
		if status.Enabled {
			ms.readinessMutex.RLock()
			transport := freshReadinessStatus(ms.outgoingTransportHealth, now)
			ms.readinessMutex.RUnlock()
			result.Transport = &transport
		}
	}
	return result
}

func (ms *MailServer) runtimeSnapshot() RuntimeReadiness {
	return ms.GetRuntimeReadiness()
}

func (ms *MailServer) publishRuntimeReadiness() error {
	data, err := json.Marshal(ms.runtimeSnapshot())
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(ms.mailDir, runtimeReadinessPrefix+"*")
	if err != nil {
		return err
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(path, RuntimeReadinessPath(ms.mailDir, ms.host, ms.GetPort()))
}

// One publisher can block on slow storage without spawning repeated I/O.
// Its last snapshot then expires; CLI reads have their own bounded wait.
func (ms *MailServer) startRuntimeReadiness() func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = ms.publishRuntimeReadiness()
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	return func() { close(stop); <-done }
}
