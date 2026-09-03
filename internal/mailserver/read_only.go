package mailserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ReadOnlyRefreshPartialError reports files that could not be inspected while
// preserving every mailbox entry whose state could not be determined safely.
// The top-level mailbox directory was still readable.
type ReadOnlyRefreshPartialError struct {
	err error
}

func (err *ReadOnlyRefreshPartialError) Error() string { return err.err.Error() }
func (err *ReadOnlyRefreshPartialError) Unwrap() error { return err.err }

// RefreshReadOnlyMailbox synchronizes the in-memory snapshot with committed
// EML files without recovery, metadata migration, quarantine, or any write to
// the mailbox. Newly observed messages emit the ordinary in-process new event.
func (ms *MailServer) RefreshReadOnlyMailbox() error {
	files, err := os.ReadDir(ms.mailDir)
	if err != nil {
		return fmt.Errorf("read mail directory: %w", err)
	}

	type candidate struct {
		entry             os.DirEntry
		receivedAt        time.Time
		read              bool
		metadata          *emailMetadata
		metadataUncertain bool
	}
	candidates := make(map[string]candidate)
	var refreshErrors []error
	blockedIDs := make(map[string]struct{})
	uncertainIDs := make(map[string]struct{})
	for _, file := range files {
		if id, ok := deletionFenceID(file.Name()); ok {
			blockedIDs[id] = struct{}{}
			continue
		}
		if id, ok := rollbackFenceID(file.Name()); ok {
			state, stateErr := readRollbackFenceState(filepath.Join(ms.mailDir, file.Name()))
			if stateErr != nil {
				blockedIDs[id] = struct{}{}
				uncertainIDs[id] = struct{}{}
				refreshErrors = append(refreshErrors, fmt.Errorf("read rollback fence for %s: %w", id, stateErr))
				continue
			}
			if state != acceptedFenceState && state != localFenceState {
				blockedIDs[id] = struct{}{}
			}
		}
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".eml") {
			continue
		}
		id := strings.TrimSuffix(file.Name(), ".eml")
		if err := validateEmailID(id); err != nil {
			refreshErrors = append(refreshErrors, fmt.Errorf("ignore invalid email filename %q: %w", file.Name(), err))
			continue
		}
		if _, blocked := blockedIDs[id]; blocked {
			continue
		}
		info, err := file.Info()
		if err != nil {
			uncertainIDs[id] = struct{}{}
			refreshErrors = append(refreshErrors, fmt.Errorf("stat email %s: %w", id, err))
			continue
		}
		item := candidate{entry: file, receivedAt: info.ModTime().UTC()}
		if metadata, metadataErr := ms.loadEmailMetadata(id); metadataErr == nil {
			item.read = metadata.Read
			item.receivedAt = metadata.Sequence
			item.metadata = &metadata
		} else if !os.IsNotExist(metadataErr) {
			item.metadataUncertain = true
			refreshErrors = append(refreshErrors, fmt.Errorf("read metadata for %s: %w", id, metadataErr))
		}
		candidates[id] = item
	}

	ms.storeMutex.RLock()
	existing := make(map[string]struct{}, len(ms.storeByID))
	for id := range ms.storeByID {
		existing[id] = struct{}{}
	}
	ms.storeMutex.RUnlock()

	loaded := make(map[string]*Email)
	for id, item := range candidates {
		if _, ok := existing[id]; ok {
			continue
		}
		path := filepath.Join(ms.mailDir, item.entry.Name())
		file, err := os.Open(path)
		if err != nil {
			refreshErrors = append(refreshErrors, fmt.Errorf("open email %s: %w", id, err))
			continue
		}
		email, envelope, parseErr := ms.parseEmailMessage(id, file, nil, false, filepath.Join(ms.mailDir, id))
		closeErr := file.Close()
		if parseErr != nil || closeErr != nil {
			if parseErr != nil {
				refreshErrors = append(refreshErrors, fmt.Errorf("parse email %s: %w", id, parseErr))
			}
			if closeErr != nil {
				refreshErrors = append(refreshErrors, fmt.Errorf("close email %s: %w", id, closeErr))
			}
			continue
		}
		if !ms.retainAllHeaders.Load() {
			email.AllHeaders = nil
		}
		email.ID = id
		if email.Time.IsZero() {
			email.Time = item.receivedAt
		}
		email.Read = item.read
		if item.metadata != nil && item.metadata.Envelope != nil {
			envelope = cloneEnvelope(item.metadata.Envelope)
		}
		email.Envelope = envelope
		email.Source = path
		if info, statErr := item.entry.Info(); statErr == nil {
			email.Size = info.Size()
			email.SizeHuman = formatBytes(info.Size())
		}
		if envelope != nil {
			email.CalculatedBCC = calculateBCC(append([]string(nil), envelope.To...), addressListToStrings(email.To), addressListToStrings(email.CC))
		}
		if email.HTML != "" {
			email.HTML = strings.TrimSpace(sanitizeHTML(email.HTML))
		}
		if item.metadataUncertain && len(email.Attachments) > 0 {
			continue
		}
		if item.metadata != nil {
			if err := restoreAttachmentMetadata(email, *item.metadata); err != nil {
				refreshErrors = append(refreshErrors, fmt.Errorf("restore attachment metadata for %s: %w", id, err))
				continue
			}
		}
		if len(email.Attachments) > 0 && !item.metadataUncertain && (item.metadata == nil || len(item.metadata.Attachments) == 0) {
			if err := ms.restoreLegacyLocalAttachmentMetadata(id, email.Attachments); err != nil {
				refreshErrors = append(refreshErrors, fmt.Errorf("restore legacy attachment metadata for %s: %w", id, err))
				continue
			}
		}
		loaded[id] = email
	}

	// A writer in another process may start or complete deletion while this
	// observer parses an EML from the initial directory snapshot. Take one fresh
	// directory snapshot immediately before publication: an in-progress delete
	// has a fence, while a completed delete no longer has its source EML.
	for id := range candidates {
		if ms.beforeReadOnlyPublish != nil {
			ms.beforeReadOnlyPublish(id)
		}
	}
	visible, visibilityErrors, visibilityErr := ms.readOnlyVisibilitySnapshot()
	if visibilityErr != nil {
		refreshErrors = append(refreshErrors, fmt.Errorf("rescan mail directory before publication: %w", visibilityErr))
		for id := range candidates {
			uncertainIDs[id] = struct{}{}
			delete(candidates, id)
			delete(loaded, id)
		}
	} else {
		for id := range candidates {
			if itemErr := visibilityErrors[id]; itemErr != nil {
				uncertainIDs[id] = struct{}{}
				refreshErrors = append(refreshErrors, fmt.Errorf("recheck email %s before publication: %w", id, itemErr))
			}
			if !visible[id] {
				delete(candidates, id)
				delete(loaded, id)
			}
		}
	}

	newEmails := make([]*Email, 0, len(loaded))
	ms.storeMutex.Lock()
	for id := range ms.storeByID {
		if _, ok := candidates[id]; !ok {
			if _, uncertain := uncertainIDs[id]; uncertain {
				continue
			}
			delete(ms.storeByID, id)
			delete(ms.receivedAtByID, id)
		}
	}
	for id, item := range candidates {
		if email := ms.storeByID[id]; email != nil {
			if item.metadataUncertain {
				continue
			}
			updated := cloneEmail(email)
			if item.metadata != nil {
				if err := restoreAttachmentMetadata(updated, *item.metadata); err != nil {
					refreshErrors = append(refreshErrors, fmt.Errorf("restore attachment metadata for %s: %w", id, err))
					continue
				}
				if item.metadata.Envelope != nil {
					updated.Envelope = cloneEnvelope(item.metadata.Envelope)
					updated.CalculatedBCC = calculateBCC(append([]string(nil), updated.Envelope.To...), addressListToStrings(updated.To), addressListToStrings(updated.CC))
				}
			}
			updated.Read = item.read
			ms.storeByID[id] = updated
			ms.receivedAtByID[id] = item.receivedAt
		}
	}
	for id, email := range loaded {
		ms.storeByID[id] = cloneEmail(email)
		ms.receivedAtByID[id] = candidates[id].receivedAt
	}
	ms.storeOrder = ms.storeOrder[:0]
	for id := range ms.storeByID {
		ms.storeOrder = append(ms.storeOrder, id)
	}
	sort.SliceStable(ms.storeOrder, func(i, j int) bool {
		left, right := ms.storeOrder[i], ms.storeOrder[j]
		leftTime, rightTime := ms.receivedAtByID[left], ms.receivedAtByID[right]
		if leftTime.Equal(rightTime) {
			return left < right
		}
		return leftTime.Before(rightTime)
	})
	for _, id := range ms.storeOrder {
		if email := loaded[id]; email != nil {
			newEmails = append(newEmails, cloneEmail(email))
		}
	}
	ms.resetMailboxStorePositionsLocked()
	ms.storeMutex.Unlock()

	for _, email := range newEmails {
		ms.emitAsynchronous("new", email)
	}
	if err := errors.Join(refreshErrors...); err != nil {
		return &ReadOnlyRefreshPartialError{err: err}
	}
	return nil
}

func (ms *MailServer) readOnlyVisibilitySnapshot() (map[string]bool, map[string]error, error) {
	entries, err := os.ReadDir(ms.mailDir)
	if err != nil {
		return nil, nil, err
	}
	visible := make(map[string]bool)
	visibilityErrors := make(map[string]error)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".eml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".eml")
		if validateEmailID(id) == nil {
			visible[id] = true
		}
	}
	for _, entry := range entries {
		if id, ok := deletionFenceID(entry.Name()); ok {
			delete(visible, id)
			continue
		}
		id, ok := rollbackFenceID(entry.Name())
		if !ok {
			continue
		}
		state, stateErr := readRollbackFenceState(filepath.Join(ms.mailDir, entry.Name()))
		if stateErr != nil {
			delete(visible, id)
			visibilityErrors[id] = stateErr
			continue
		}
		if state != acceptedFenceState && state != localFenceState {
			delete(visible, id)
		}
	}
	return visible, visibilityErrors, nil
}
