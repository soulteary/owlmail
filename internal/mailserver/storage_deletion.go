package mailserver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	deletionFencePrefix = storageTempPrefix + "delete-"
	deletionFenceSuffix = ".fence"
	deletionFenceState  = "delete"
)

func deletionFencePath(mailDir, id string) string {
	return filepath.Join(mailDir, deletionFencePrefix+id+deletionFenceSuffix)
}

func deletionFenceID(name string) (string, bool) {
	if !strings.HasPrefix(name, deletionFencePrefix) || !strings.HasSuffix(name, deletionFenceSuffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, deletionFencePrefix), deletionFenceSuffix)
	return id, validateEmailID(id) == nil
}

// ensureDeletionFence records deletion intent before any local or remote
// artifact is removed. An existing fence makes retries idempotent.
func (ms *MailServer) ensureDeletionFence(id string) error {
	if err := validateEmailID(id); err != nil {
		return err
	}
	fencePath := deletionFencePath(ms.mailDir, id)
	fence, err := os.OpenFile(fencePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create email deletion fence: %w", err)
	}
	committed := false
	defer func() {
		_ = fence.Close()
		if !committed {
			_ = os.Remove(fencePath)
			_ = syncDirectory(ms.mailDir)
		}
	}()
	if _, err := fence.WriteString(deletionFenceState + "\n"); err != nil {
		return fmt.Errorf("write email deletion fence: %w", err)
	}
	if err := fence.Sync(); err != nil {
		return fmt.Errorf("sync email deletion fence: %w", err)
	}
	if err := fence.Close(); err != nil {
		return fmt.Errorf("close email deletion fence: %w", err)
	}
	if err := syncDirectory(ms.mailDir); err != nil {
		return fmt.Errorf("sync email deletion fence directory: %w", err)
	}
	committed = true
	return nil
}

// cleanupDeletionFencedEmail completes an idempotent delete. Remote cleanup is
// attempted first and no local evidence is removed if it fails. The durable
// fence remains until every artifact deletion has been synced.
func (ms *MailServer) cleanupDeletionFencedEmail(id string) error {
	if ms.attachmentStore != nil {
		if err := ms.deleteRemoteAttachments(id); err != nil {
			return fmt.Errorf("delete remote attachments: %w", err)
		}
	}

	var cleanupErrors []error
	if err := os.RemoveAll(filepath.Join(ms.mailDir, id)); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete local attachments: %w", err))
	}
	if err := os.Remove(filepath.Join(ms.mailDir, id+".eml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete email file: %w", err))
	}
	if err := ms.deleteEmailMetadata(id); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete email metadata: %w", err))
	}
	if err := syncDirectoryIfExists(filepath.Join(ms.mailDir, metadataDirectoryName)); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("sync email metadata directory: %w", err))
	}
	if err := syncDirectory(ms.mailDir); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("sync mail directory: %w", err))
	}
	if err := errors.Join(cleanupErrors...); err != nil {
		return err
	}

	fencePath := deletionFencePath(ms.mailDir, id)
	if err := os.Remove(fencePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove email deletion fence: %w", err)
	}
	if err := syncDirectory(ms.mailDir); err != nil {
		return fmt.Errorf("sync deleted email fence: %w", err)
	}
	return nil
}

func syncDirectoryIfExists(path string) error {
	err := syncDirectory(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (ms *MailServer) deletionCandidates() ([]string, error) {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	add := func(id string) {
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	ms.storeMutex.RLock()
	for _, id := range ms.storeOrder {
		add(id)
	}
	for id := range ms.storeByID {
		add(id)
	}
	ms.storeMutex.RUnlock()

	entries, err := os.ReadDir(ms.mailDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if id, ok := deletionFenceID(entry.Name()); ok {
			add(id)
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".eml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".eml")
		if validateEmailID(id) == nil {
			add(id)
		}
	}
	return ids, nil
}

func (ms *MailServer) removeEmailFromMemory(id string) {
	ms.storeMutex.Lock()
	delete(ms.storeByID, id)
	delete(ms.receivedAtByID, id)
	for i, storedID := range ms.storeOrder {
		if storedID == id {
			ms.storeOrder = append(ms.storeOrder[:i], ms.storeOrder[i+1:]...)
			break
		}
	}
	ms.storeMutex.Unlock()
}
