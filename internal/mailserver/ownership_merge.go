package mailserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// MergeOwnership idempotently moves a frozen pending mailbox's committed
// messages to a personal mailbox. The control Store must hold its mutation lock
// and validate the merge journal before calling this method. This method does
// not bind identities, publish new-email events, create Relay/Webhook work, or
// replay historical delivery. Control commit happens only after this succeeds.
// Lock order: control Store -> storageTransactionMutex -> storeMutex.
func (ms *MailServer) MergeOwnership(ctx context.Context, sourceID, targetID string) error {
	return ms.mergeOwnership(ctx, sourceID, targetID, nil)
}

func (ms *MailServer) mergeOwnership(ctx context.Context, sourceID, targetID string, afterPersist func(string) error) error {
	if !ms.requireOwnership || ms.readOnly {
		return errors.New("mailbox merge requires writable isolated storage")
	}
	for _, id := range []string{sourceID, targetID} {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed.String() != id {
			return errors.New("invalid mailbox merge identifier")
		}
	}
	if sourceID == targetID {
		return errors.New("mailbox merge source and target must differ")
	}
	ms.storageTransactionMutex.Lock()
	defer ms.storageTransactionMutex.Unlock()
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()
	if err := ms.checkOwnershipMergeQuotaLocked(sourceID, targetID); err != nil {
		return err
	}
	for _, id := range ms.storeOrder {
		if err := ctx.Err(); err != nil {
			return err
		}
		email, ok := ms.storeByID[id]
		if !ok {
			continue
		}
		if email.Ownership == nil {
			return fmt.Errorf("isolated message %s has no ownership", id)
		}
		if email.Ownership.MailboxID != sourceID {
			continue
		}
		updated := cloneEmail(email)
		updated.Ownership.MailboxID = targetID
		// Atomic sidecar replacement is the per-message durability fence. If the
		// process crashes here, startup restores the new owner. The durable control
		// journal keeps the source frozen and repeats the remaining messages.
		if err := ms.persistEmailMetadataAt(updated, ms.receivedAtByID[id]); err != nil {
			return fmt.Errorf("persist merged message %s: %w", id, err)
		}
		if afterPersist != nil {
			if err := afterPersist(id); err != nil {
				return err
			}
		}
		ms.storeByID[id] = updated
		ms.upsertMailboxIndexLocked(updated, ms.receivedAtByID[id])
	}
	return nil
}
