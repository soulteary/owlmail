package mailserver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/soulteary/owlmail/internal/relay"
)

// InitializeRelayService loads the single durable relay owner and automatic
// handoffs before retention policies or SMTP listeners may expose this mailbox.
// It does not start delivery. Read-only observers never create relay state.
func (ms *MailServer) InitializeRelayService() error {
	ms.relayInitOnce.Do(func() {
		if ms.readOnly {
			return
		}
		manager, err := newAutomaticRelayManager(ms)
		if err != nil {
			ms.relayInitErr = err
			return
		}
		service := relay.New(ms, ms.mailDir)
		ms.automaticRelay.Store(manager)
		ms.relayService.Store(service)
		manager.recoverAcknowledged(service)
		ms.SetRelayJobReadinessProvider(relayReadiness{service: service, automatic: manager})
		// Stop promotion before closing the task service. Closing the parent has
		// already rejected new transport submissions before these closers run.
		if err := ms.AddCloser(manager); err != nil {
			_ = manager.Close()
			_ = service.Close()
			ms.relayInitErr = err
			return
		}
		if err := ms.AddCloser(service); err != nil {
			_ = service.Close()
			ms.relayInitErr = err
			return
		}
	})
	return ms.relayInitErr
}

// RelayService returns the shared durable task owner. It is nil for read-only
// observers or when automatic handoff recovery could not be loaded safely.
func (ms *MailServer) RelayService() *relay.Service {
	if err := ms.InitializeRelayService(); err != nil {
		return nil
	}
	return ms.relayService.Load()
}

// StartRelayRecovery begins persisted task recovery and automatic intent
// promotion. CLI callers invoke it only after all required listeners bind.
func (ms *MailServer) StartRelayRecovery() error {
	if err := ms.InitializeRelayService(); err != nil {
		return err
	}
	service := ms.relayService.Load()
	if service == nil {
		return fmt.Errorf("relay is unavailable in read-only mode")
	}
	service.Start()
	ms.automaticRelay.Load().Start()
	return nil
}

func (ms *MailServer) needsRelayInitialization() bool {
	if ms.readOnly {
		return false
	}
	if configuration := ms.GetOutgoingConfig(); configuration != nil && configuration.Host != "" {
		return true
	}
	// A legacy mailbox may retain mail even when its optional metadata path
	// cannot be written. Without outgoing configuration, that is not evidence
	// of an existing durable Relay owner.
	if info, err := os.Stat(filepath.Join(ms.mailDir, metadataDirectoryName)); err == nil && !info.IsDir() {
		return false
	}
	for _, name := range []string{relayJobMetadataDirectoryName, relayIntentMetadataDirectoryName} {
		if _, err := os.Stat(filepath.Join(ms.mailDir, metadataDirectoryName, name)); err == nil || !os.IsNotExist(err) {
			return true
		}
	}
	return false
}
