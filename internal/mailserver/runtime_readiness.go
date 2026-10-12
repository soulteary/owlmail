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
}

func (status RuntimeReadiness) Validate(host string, port int, now time.Time) error {
	if status.Version != 1 || status.PID <= 0 || status.Host != host || status.Port != port {
		return fmt.Errorf("runtime readiness does not match the configured SMTP service")
	}
	age := now.Sub(status.UpdatedAt)
	if status.UpdatedAt.IsZero() || age < -time.Second || age > RuntimeReadinessMaxAge {
		return fmt.Errorf("runtime readiness is stale or unavailable")
	}
	if !status.SMTP {
		return fmt.Errorf("runtime SMTP listeners are not ready")
	}
	if !status.Storage.Ready() {
		return fmt.Errorf("runtime mail storage is not writable")
	}
	if status.Storage.CheckedAt.IsZero() || now.Sub(status.Storage.CheckedAt) > 45*time.Second {
		return fmt.Errorf("runtime mail storage check is stale")
	}
	if status.Attachment != nil && !status.Attachment.Ready() {
		return fmt.Errorf("runtime attachment storage is not ready")
	}
	if status.Relay != nil && !status.Relay.Ready() {
		return fmt.Errorf("runtime relay workers or queue are not ready")
	}
	return nil
}

func (ms *MailServer) runtimeSnapshot() RuntimeReadiness {
	smtp, storage := ms.GetServiceReadiness()
	result := RuntimeReadiness{Version: 1, PID: os.Getpid(), Host: ms.host, Port: ms.port, UpdatedAt: time.Now(), SMTP: smtp, Storage: storage}
	if attachment, enabled := ms.GetAttachmentHealth(); enabled {
		result.Attachment = &attachment
	}
	ms.outgoingMutex.RLock()
	relay := ms.outgoing
	closed := ms.outgoingClosed
	ms.outgoingMutex.RUnlock()
	if relay != nil {
		status := outgoing.RuntimeReadiness{Closed: true}
		if provider, ok := relay.(interface {
			RuntimeReadiness() outgoing.RuntimeReadiness
		}); ok {
			status = provider.RuntimeReadiness()
		}
		status.Closed = status.Closed || closed
		result.Relay = &status
	}
	return result
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
	return os.Rename(path, RuntimeReadinessPath(ms.mailDir, ms.host, ms.port))
}

// One publisher can block on slow storage without spawning repeated I/O.
// Its last snapshot then expires; CLI reads have their own bounded wait.
func (ms *MailServer) startRuntimeReadiness() func() {
	stop := make(chan struct{})
	go func() {
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
	return func() { close(stop) }
}
