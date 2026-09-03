package mailserver

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/soulteary/owlmail/internal/outgoing"
)

// RelayMail relays an email to the configured SMTP server
func (ms *MailServer) RelayMail(email *Email, isAutoRelay bool, callback func(error)) error {
	ms.outgoingMutex.RLock()
	outgoingRelay := ms.outgoing
	closed := ms.outgoingClosed
	ms.outgoingMutex.RUnlock()
	if closed {
		return outgoing.ErrClosed
	}
	if outgoingRelay == nil {
		return outgoing.ErrNotConfigured
	}

	emlPath := filepath.Join(ms.mailDir, email.ID+".eml")
	return outgoingRelay.RelayMail(email, emlPath, "", isAutoRelay, callback)
}

// RelayMailTo relays an email to a specific address
func (ms *MailServer) RelayMailTo(email *Email, relayTo string, callback func(error)) error {
	ms.outgoingMutex.RLock()
	outgoingRelay := ms.outgoing
	closed := ms.outgoingClosed
	ms.outgoingMutex.RUnlock()
	if closed {
		return outgoing.ErrClosed
	}
	if outgoingRelay == nil {
		return outgoing.ErrNotConfigured
	}

	emlPath := filepath.Join(ms.mailDir, email.ID+".eml")
	return outgoingRelay.RelayMail(email, emlPath, relayTo, false, callback)
}

// EffectiveRelayRecipients returns the outgoing rule-filtered recipient list
// used by a manual relay to the original SMTP envelope.
func (ms *MailServer) EffectiveRelayRecipients(email *Email) ([]string, error) {
	ms.outgoingMutex.RLock()
	outgoingRelay := ms.outgoing
	closed := ms.outgoingClosed
	ms.outgoingMutex.RUnlock()
	if closed {
		return nil, outgoing.ErrClosed
	}
	if outgoingRelay == nil {
		return nil, outgoing.ErrNotConfigured
	}
	return outgoingRelay.EffectiveRecipients(email)
}

// RelayMailConfirmed relays only when the current rule-filtered recipients
// match the list confirmed by the operator.
func (ms *MailServer) RelayMailConfirmed(email *Email, recipients []string, callback func(error)) error {
	ms.outgoingMutex.RLock()
	outgoingRelay := ms.outgoing
	closed := ms.outgoingClosed
	ms.outgoingMutex.RUnlock()
	if closed {
		return outgoing.ErrClosed
	}
	if outgoingRelay == nil {
		return outgoing.ErrNotConfigured
	}
	emlPath := filepath.Join(ms.mailDir, email.ID+".eml")
	return outgoingRelay.RelayMailConfirmed(email, emlPath, recipients, callback)
}

// RelayMailAndWait relays one message and waits for the outgoing worker's
// result. OwlMail's native API intentionally keeps its asynchronous enqueue
// semantics; the MailDev REST facade uses this method because MailDev only
// returns success after the relay attempt completes.
func (ms *MailServer) RelayMailAndWait(ctx context.Context, email *Email, relayTo string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ms.outgoingMutex.RLock()
	outgoingRelay := ms.outgoing
	closed := ms.outgoingClosed
	ms.outgoingMutex.RUnlock()
	if closed {
		return outgoing.ErrClosed
	}
	if outgoingRelay == nil {
		return outgoing.ErrNotConfigured
	}
	result := make(chan error, 1)
	callback := func(err error) {
		result <- err
	}
	emlPath := filepath.Join(ms.mailDir, email.ID+".eml")
	if err := outgoingRelay.RelayMailContext(ctx, email, emlPath, relayTo, false, callback); err != nil {
		return err
	}
	return waitRelayResult(ctx, result)
}

func waitRelayResult(ctx context.Context, result <-chan error) error {
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return fmt.Errorf("relay attempt canceled: %w", ctx.Err())
	}
}

// SetOutgoingConfig sets the outgoing mail configuration
func (ms *MailServer) SetOutgoingConfig(config *outgoing.OutgoingConfig) error {
	ms.outgoingMutex.Lock()
	defer ms.outgoingMutex.Unlock()
	if ms.outgoingClosed {
		return outgoing.ErrClosed
	}
	if ms.outgoing == nil {
		outgoingRelay := outgoing.NewOutgoingMail(nil)
		if err := outgoingRelay.UpdateConfig(config); err != nil {
			outgoingRelay.Close()
			return err
		}
		ms.outgoing = outgoingRelay
		return nil
	}
	return ms.outgoing.UpdateConfig(config)
}

// GetOutgoingConfig returns the outgoing mail configuration
func (ms *MailServer) GetOutgoingConfig() *outgoing.OutgoingConfig {
	ms.outgoingMutex.RLock()
	outgoingRelay := ms.outgoing
	ms.outgoingMutex.RUnlock()
	if outgoingRelay == nil {
		return nil
	}
	if config, ok := outgoingRelay.GetConfig().(*outgoing.OutgoingConfig); ok {
		return config
	}
	return nil
}

// IsAutoRelayEnabled reports whether the current immutable configuration
// enables automatic relay without exposing the relay implementation pointer.
func (ms *MailServer) IsAutoRelayEnabled() bool {
	ms.outgoingMutex.RLock()
	outgoingRelay := ms.outgoing
	closed := ms.outgoingClosed
	ms.outgoingMutex.RUnlock()
	return !closed && outgoingRelay != nil && outgoingRelay.IsAutoRelayEnabled()
}
