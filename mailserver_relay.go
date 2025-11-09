package main

import (
	"fmt"
	"path/filepath"
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

// SetOutgoingConfig sets the outgoing mail configuration
func (ms *MailServer) SetOutgoingConfig(config *OutgoingConfig) {
	if ms.outgoing == nil {
		ms.outgoing = NewOutgoingMail(config)
	} else {
		ms.outgoing.UpdateConfig(config)
	}
}

// GetOutgoingConfig returns the outgoing mail configuration
func (ms *MailServer) GetOutgoingConfig() *OutgoingConfig {
	if ms.outgoing == nil {
		return nil
	}
	return ms.outgoing.GetConfig()
}
