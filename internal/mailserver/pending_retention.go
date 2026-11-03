package mailserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/mailbox"
)

// RetainPending applies a control-store-authorized pending cleanup. The caller
// must hold Store.mu via RunPendingRetention/CleanupPending. It never emits
// deletion events, creates outbound work, or removes personal mailbox data.
// The control journal is authoritative until this idempotent callback succeeds.
func (ms *MailServer) RetainPending(ctx context.Context, item mailbox.PendingRetention) (mailbox.PendingRetentionResult, error) {
	result, _, err := ms.retainPendingBatch(ctx, item, 0, nil)
	return result, err
}

// RetainPendingBatch removes at most limit expired messages. It has the same
// control-store authorization requirements as RetainPending. Completion is
// separate from Remaining: newer content does not prevent a sweep completing.
func (ms *MailServer) RetainPendingBatch(ctx context.Context, item mailbox.PendingRetention, limit int) (mailbox.PendingRetentionResult, bool, error) {
	if limit < 1 || limit > 128 {
		return mailbox.PendingRetentionResult{}, false, errors.New("pending retention batch limit must be between 1 and 128")
	}
	return ms.retainPendingBatch(ctx, item, limit, nil)
}
func (ms *MailServer) retainPending(ctx context.Context, item mailbox.PendingRetention, afterDelete func(string) error) (mailbox.PendingRetentionResult, error) {
	result, _, err := ms.retainPendingBatch(ctx, item, 0, afterDelete)
	return result, err
}
func (ms *MailServer) retainPendingBatch(ctx context.Context, item mailbox.PendingRetention, limit int, afterDelete func(string) error) (mailbox.PendingRetentionResult, bool, error) {
	result := mailbox.PendingRetentionResult{}
	if !ms.requireOwnership || ms.readOnly || item.Cutoff.IsZero() {
		return result, false, errors.New("pending retention requires writable isolated storage and cutoff")
	}
	parsed, err := uuid.Parse(item.MailboxID)
	if err != nil || parsed.String() != item.MailboxID {
		return result, false, errors.New("invalid pending mailbox identifier")
	}
	ms.storageTransactionMutex.Lock()
	defer ms.storageTransactionMutex.Unlock()
	ms.storeMutex.RLock()
	ids := make([]string, 0)
	complete := true
	for _, id := range ms.storeOrder {
		email := ms.storeByID[id]
		if email == nil || email.Ownership == nil || email.Ownership.MailboxID != item.MailboxID {
			continue
		}
		received := ms.receivedAtByID[id]
		if received.IsZero() {
			ms.storeMutex.RUnlock()
			return result, false, fmt.Errorf("pending message %s has no durable received time", id)
		}
		if !received.After(item.Cutoff) && (limit == 0 || len(ids) < limit) {
			ids = append(ids, id)
		} else {
			if !received.After(item.Cutoff) {
				complete = false
			}
			result.Remaining++
		}
	}
	ms.storeMutex.RUnlock()
	// Pending content should not be leased by user routes. Check before any
	// deletion anyway, so an internal source reader cannot lose its files.
	for _, id := range ids {
		if ms.protectedEmailSources[id] > 0 || ms.pendingAutomaticRelay(id) {
			return result, false, ErrEmailSourceInUse
		}
	}
	for _, id := range ids {
		if err = ctx.Err(); err != nil {
			return result, false, err
		}
		if ms.beforeEmailDelete != nil {
			if err = ms.beforeEmailDelete(id); err != nil {
				return result, false, err
			}
		}
		if err = ms.ensureDeletionFence(id); err != nil {
			return result, false, err
		}
		if err = ms.cleanupDeletionFencedEmail(id); err != nil {
			return result, false, err
		}
		ms.deleteEmailFromRuntimeState(id)
		ms.deletedMessages.Add(1)
		result.Deleted++
		if afterDelete != nil {
			if err = afterDelete(id); err != nil {
				return result, false, err
			}
		}
	}
	return result, complete, nil
}
