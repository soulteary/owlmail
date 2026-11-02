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
	return ms.retainPending(ctx, item, nil)
}
func (ms *MailServer) retainPending(ctx context.Context, item mailbox.PendingRetention, afterDelete func(string) error) (mailbox.PendingRetentionResult, error) {
	result := mailbox.PendingRetentionResult{}
	if !ms.requireOwnership || ms.readOnly || item.Cutoff.IsZero() {
		return result, errors.New("pending retention requires writable isolated storage and cutoff")
	}
	parsed, err := uuid.Parse(item.MailboxID)
	if err != nil || parsed.String() != item.MailboxID {
		return result, errors.New("invalid pending mailbox identifier")
	}
	ms.storageTransactionMutex.Lock()
	defer ms.storageTransactionMutex.Unlock()
	ms.storeMutex.RLock()
	ids := make([]string, 0)
	for _, id := range ms.storeOrder {
		email := ms.storeByID[id]
		if email == nil || email.Ownership == nil || email.Ownership.MailboxID != item.MailboxID {
			continue
		}
		received := ms.receivedAtByID[id]
		if received.IsZero() {
			ms.storeMutex.RUnlock()
			return result, fmt.Errorf("pending message %s has no durable received time", id)
		}
		if !received.After(item.Cutoff) {
			ids = append(ids, id)
		} else {
			result.Remaining++
		}
	}
	ms.storeMutex.RUnlock()
	// Pending content should not be leased by user routes. Check before any
	// deletion anyway, so an internal source reader cannot lose its files.
	for _, id := range ids {
		if ms.protectedEmailSources[id] > 0 || ms.pendingAutomaticRelay(id) {
			return result, ErrEmailSourceInUse
		}
	}
	for _, id := range ids {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		if ms.beforeEmailDelete != nil {
			if err = ms.beforeEmailDelete(id); err != nil {
				return result, err
			}
		}
		if err = ms.ensureDeletionFence(id); err != nil {
			return result, err
		}
		if err = ms.cleanupDeletionFencedEmail(id); err != nil {
			return result, err
		}
		ms.deleteEmailFromRuntimeState(id)
		ms.deletedMessages.Add(1)
		result.Deleted++
		if afterDelete != nil {
			if err = afterDelete(id); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}
