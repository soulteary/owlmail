package mailserver

import (
	"fmt"
	"time"
)

// RecordRelaySuccess records the last SMTP DATA acceptance independently from
// bounded job history. It must be called only after the downstream accepted
// the complete message, with the recipients used by that transaction.
// A persistence error never changes the fact that SMTP delivery succeeded:
// the in-memory receipt is retained and callers must repair metadata, not resend.
func (ms *MailServer) RecordRelaySuccess(id string, acceptedAt time.Time, recipients []string) error {
	if ms.readOnly {
		return fmt.Errorf("read-only mailbox cannot record relay delivery")
	}
	if err := validateEmailID(id); err != nil {
		return err
	}
	if acceptedAt.IsZero() || len(recipients) == 0 {
		return fmt.Errorf("relay receipt requires acceptance time and actual recipients")
	}
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()
	email := ms.storeByID[id]
	if email == nil {
		return ErrEmailNotFound
	}
	// Recovery can replay an earlier completed task. Never replace a newer
	// observed acceptance with an older one merely because record order differs.
	if email.RelayedAt != nil && email.RelayedAt.After(acceptedAt) {
		return nil
	}
	acceptedAt = acceptedAt.UTC()
	email.RelayedAt = &acceptedAt
	email.RelayedTo = append([]string(nil), recipients...)
	if err := ms.persistEmailMetadataAt(email, ms.receivedAtByID[id]); err != nil {
		return fmt.Errorf("persist relay receipt: %w", err)
	}
	return nil
}

func restoreRelayReceipt(email *Email, metadata emailMetadata) {
	email.RelayedAt = cloneTime(metadata.RelayedAt)
	email.RelayedTo = append([]string(nil), metadata.RelayedTo...)
}
