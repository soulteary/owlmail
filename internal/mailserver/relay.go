package mailserver

import (
	"fmt"
	"path/filepath"

	"github.com/soulteary/owlmail/internal/outgoing"
)

// RelayMail relays an email to the configured SMTP server
func (ms *MailServer) RelayMail(email *Email, isAutoRelay bool, callback func(error)) error {
	if ms.outgoing == nil {
		return fmt.Errorf("outgoing mail not configured")
	}

	emlPath := filepath.Join(ms.mailDir, email.ID+".eml")
	ms.outgoing.RelayMail(email, emlPath, "", isAutoRelay, callback)
	return nil
}

// RelayMailTo relays an email to a specific address
func (ms *MailServer) RelayMailTo(email *Email, relayTo string, callback func(error)) error {
	if ms.outgoing == nil {
		return fmt.Errorf("outgoing mail not configured")
	}

	emlPath := filepath.Join(ms.mailDir, email.ID+".eml")
	ms.outgoing.RelayMail(email, emlPath, relayTo, false, callback)
	return nil
}

// RelayMailAndWait relays one message and waits for the outgoing worker's
// result. OwlMail's native API intentionally keeps its asynchronous enqueue
// semantics; the MailDev REST facade uses this method because MailDev only
// returns success after the relay attempt completes.
func (ms *MailServer) RelayMailAndWait(email *Email, relayTo string) error {
	result := make(chan error, 1)
	callback := func(err error) {
		result <- err
	}
	var err error
	if relayTo == "" {
		err = ms.RelayMail(email, false, callback)
	} else {
		err = ms.RelayMailTo(email, relayTo, callback)
	}
	if err != nil {
		return err
	}
	return <-result
}

// SetOutgoingConfig sets the outgoing mail configuration
func (ms *MailServer) SetOutgoingConfig(config *outgoing.OutgoingConfig) {
	if ms.outgoing == nil {
		ms.outgoing = outgoing.NewOutgoingMail(config)
	} else {
		ms.outgoing.UpdateConfig(config)
	}
}

// GetOutgoingConfig returns the outgoing mail configuration
func (ms *MailServer) GetOutgoingConfig() *outgoing.OutgoingConfig {
	if ms.outgoing == nil {
		return nil
	}
	if config, ok := ms.outgoing.GetConfig().(*outgoing.OutgoingConfig); ok {
		return config
	}
	return nil
}
