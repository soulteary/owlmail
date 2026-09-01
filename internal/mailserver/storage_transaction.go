package mailserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/common"
)

const (
	storageTempPrefix   = ".owlmail-tmp-"
	rollbackFencePrefix = storageTempPrefix + "rollback-"
	rollbackFenceSuffix = ".fence"
	activeFenceState    = "active"
	rollbackFenceState  = "rollback"
	acceptedFenceState  = "accepted"
	localFenceState     = "accepted-local"
	quarantineDirName   = "quarantine"
)

// storeIncomingEmail commits attachments first and the EML file last. The EML
// rename is the transaction marker observed by startup recovery.
func (ms *MailServer) storeIncomingEmail(id string, r io.Reader, session *Session) error {
	ms.storageTransactionMutex.RLock()
	defer ms.storageTransactionMutex.RUnlock()
	if err := validateEmailID(id); err != nil {
		return err
	}
	finalEML := filepath.Join(ms.mailDir, id+".eml")
	finalAttachments := filepath.Join(ms.mailDir, id)
	if _, err := os.Stat(finalEML); err == nil {
		return fmt.Errorf("email already exists: %s", id)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check final email path: %w", err)
	}

	raw, err := os.CreateTemp(ms.mailDir, storageTempPrefix+id+"-*.eml.tmp")
	if err != nil {
		return fmt.Errorf("create temporary email: %w", err)
	}
	rawPath := raw.Name()
	stagedAttachments, err := os.MkdirTemp(ms.mailDir, storageTempPrefix+id+"-attachments-")
	if err != nil {
		_ = raw.Close()
		_ = os.Remove(rawPath)
		return fmt.Errorf("create temporary attachment directory: %w", err)
	}
	committedAttachments := false
	committedEML := false
	defer func() {
		_ = raw.Close()
		_ = os.Remove(rawPath)
		_ = os.RemoveAll(stagedAttachments)
		if !committedEML && committedAttachments {
			_ = ms.deleteStoredAttachments(id, finalAttachments)
		}
	}()

	if _, err := io.Copy(raw, r); err != nil {
		return fmt.Errorf("write temporary email: %w", err)
	}
	if err := raw.Sync(); err != nil {
		return fmt.Errorf("sync temporary email: %w", err)
	}
	if err := raw.Close(); err != nil {
		return fmt.Errorf("close temporary email: %w", err)
	}

	reader, err := os.Open(rawPath)
	if err != nil {
		return fmt.Errorf("open temporary email for parsing: %w", err)
	}
	email, envelope, parseErr := ms.parseEmailMessage(id, reader, session, true, stagedAttachments)
	closeErr := reader.Close()
	if parseErr != nil {
		return parseErr
	}
	if closeErr != nil {
		return fmt.Errorf("close parsed email: %w", closeErr)
	}

	entries, err := os.ReadDir(stagedAttachments)
	if err != nil {
		return fmt.Errorf("read staged attachments: %w", err)
	}
	if len(entries) > 0 {
		if err := syncDirectory(stagedAttachments); err != nil {
			return fmt.Errorf("sync staged attachments: %w", err)
		}
	}
	// Persist the rollback decision before any final path becomes visible. If
	// a later handoff or cleanup fails, recovery can never mistake the EML for
	// an accepted message.
	if err := ms.createRollbackFence(id); err != nil {
		return err
	}
	if len(entries) > 0 {
		if ms.attachmentStore != nil {
			if err := ms.uploadAttachments(id, stagedAttachments, email.Attachments); err != nil {
				return fmt.Errorf("commit S3 attachments: %w", err)
			}
		} else {
			if err := os.Rename(stagedAttachments, finalAttachments); err != nil {
				return fmt.Errorf("commit attachments: %w", err)
			}
		}
		committedAttachments = true
	}
	if err := os.Rename(rawPath, finalEML); err != nil {
		return fmt.Errorf("commit email: %w", err)
	}
	committedEML = true
	if err := syncDirectory(ms.mailDir); err != nil {
		rollbackErr := ms.rollbackIncomingEmail(id, finalEML, finalAttachments)
		committedEML = false
		committedAttachments = false
		return errors.Join(fmt.Errorf("sync mail directory: %w", err), rollbackErr)
	}

	if err := ms.saveEmailToStore(id, false, envelope, email, true, true); err != nil {
		rollbackErr := ms.rollbackIncomingEmail(id, finalEML, finalAttachments)
		committedEML = false
		committedAttachments = false
		return errors.Join(fmt.Errorf("commit email to memory: %w", err), rollbackErr)
	}
	return nil
}

func rollbackFencePath(mailDir, id string) string {
	return filepath.Join(mailDir, rollbackFencePrefix+id+rollbackFenceSuffix)
}

func (ms *MailServer) createRollbackFence(id string) error {
	fencePath := rollbackFencePath(ms.mailDir, id)
	fence, err := os.OpenFile(fencePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create email rollback fence: %w", err)
	}
	if _, err := fence.WriteString(activeFenceState + "\n"); err != nil {
		_ = fence.Close()
		return fmt.Errorf("write email rollback fence: %w", err)
	}
	if err := fence.Sync(); err != nil {
		_ = fence.Close()
		return fmt.Errorf("sync email rollback fence: %w", err)
	}
	if err := fence.Close(); err != nil {
		return fmt.Errorf("close email rollback fence: %w", err)
	}
	if err := syncDirectory(ms.mailDir); err != nil {
		return fmt.Errorf("sync email rollback fence: %w", err)
	}
	return nil
}

func (ms *MailServer) acceptRollbackFence(id string) error {
	// Keep the accepted fence until the staged webhook job is promoted. It is
	// the durable recovery key even if the user deletes the email meanwhile.
	return ms.writeRollbackFenceState(id, acceptedFenceState)
}

func (ms *MailServer) createAcceptedHandoffFence(id string) error {
	fencePath := rollbackFencePath(ms.mailDir, id)
	fence, err := os.OpenFile(fencePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			state, stateErr := readRollbackFenceState(fencePath)
			if stateErr == nil && state == acceptedFenceState {
				return nil
			}
		}
		return fmt.Errorf("create accepted webhook handoff fence: %w", err)
	}
	if _, err := fence.WriteString(acceptedFenceState + "\n"); err != nil {
		_ = fence.Close()
		return fmt.Errorf("write accepted webhook handoff fence: %w", err)
	}
	if err := fence.Sync(); err != nil {
		_ = fence.Close()
		return fmt.Errorf("sync accepted webhook handoff fence: %w", err)
	}
	if err := fence.Close(); err != nil {
		return fmt.Errorf("close accepted webhook handoff fence: %w", err)
	}
	syncFenceDirectory := syncDirectory
	if ms.syncAcceptedFenceDirectory != nil {
		syncFenceDirectory = ms.syncAcceptedFenceDirectory
	}
	if err := syncFenceDirectory(ms.mailDir); err != nil {
		// The accepted marker is already visible and its contents are durable.
		// Reporting rollback now would allow a caller retry to duplicate the
		// staged webhook. Preserve commit semantics and surface the durability
		// degradation operationally instead.
		common.Error("Failed to sync accepted webhook handoff directory: %v", err)
	}
	return nil
}

func (ms *MailServer) completeLocalRollbackFence(id string) error {
	if err := ms.writeRollbackFenceState(id, localFenceState); err != nil {
		return err
	}
	_ = os.Remove(rollbackFencePath(ms.mailDir, id))
	_ = syncDirectory(ms.mailDir)
	return nil
}

func (ms *MailServer) writeRollbackFenceState(id, state string) error {
	fencePath := rollbackFencePath(ms.mailDir, id)
	fence, err := os.CreateTemp(ms.mailDir, storageTempPrefix+"fence-"+id+"-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := fence.Name()
	replaced := false
	defer func() {
		_ = fence.Close()
		if !replaced {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := fence.Chmod(0600); err != nil {
		return err
	}
	if _, err := fence.WriteString(state + "\n"); err != nil {
		return err
	}
	if err := fence.Sync(); err != nil {
		return err
	}
	if err := fence.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, fencePath); err != nil {
		return err
	}
	replaced = true
	return syncDirectory(ms.mailDir)
}

// rollbackIncomingEmail cleans final artifacts while retaining the previously
// persisted rollback fence. Recovery therefore keeps the ID rejected even if
// any unlink or directory sync is not durable.
func (ms *MailServer) rollbackIncomingEmail(id, emlPath, attachmentPath string) error {
	if err := ms.writeRollbackFenceState(id, rollbackFenceState); err != nil {
		return fmt.Errorf("persist rejected email rollback fence: %w", err)
	}
	var cleanupErrors []error
	if ms.beforeEmailRollback != nil {
		if err := ms.beforeEmailRollback(emlPath); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		} else if err := os.Remove(emlPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, err)
		}
	} else if err := os.Remove(emlPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := ms.deleteStoredAttachments(id, attachmentPath); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := syncDirectory(ms.mailDir); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if len(cleanupErrors) != 0 {
		// The durable fence is the rollback confirmation.
		return nil
	}
	return nil
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		// Windows does not provide the directory fsync semantics used by Unix.
		// File contents are still synced before every atomic rename.
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

func (ms *MailServer) recoverStorageArtifacts() error {
	entries, err := os.ReadDir(ms.mailDir)
	if err != nil {
		return fmt.Errorf("read mail directory for recovery: %w", err)
	}
	var recoveryErrors []error
	for _, entry := range entries {
		if id, ok := deletionFenceID(entry.Name()); ok {
			if err := ms.cleanupDeletionFencedEmail(id); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("clean deletion-fenced email %s: %w", id, err))
			}
			continue
		}
		if id, ok := rollbackFenceID(entry.Name()); ok {
			fencePath := filepath.Join(ms.mailDir, entry.Name())
			state, err := readRollbackFenceState(fencePath)
			if err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("read rollback fence for %s: %w", id, err))
				continue
			}
			if state == acceptedFenceState {
				// Webhook startup recovery owns accepted-fence cleanup.
				continue
			}
			if state == localFenceState {
				_ = os.Remove(fencePath)
				continue
			}
			if state == activeFenceState {
				if err := ms.writeRollbackFenceState(id, rollbackFenceState); err != nil {
					recoveryErrors = append(recoveryErrors, fmt.Errorf("finalize interrupted rollback fence for %s: %w", id, err))
					continue
				}
				state = rollbackFenceState
			}
			if state != rollbackFenceState {
				// Older interrupted state writes may have left a partial marker.
				// Treat unknown states conservatively as rejection so the email
				// cannot remain permanently hidden behind an invalid fence.
				if err := ms.writeRollbackFenceState(id, rollbackFenceState); err != nil {
					recoveryErrors = append(recoveryErrors, fmt.Errorf("recover invalid rollback fence for %s: %w", id, err))
					continue
				}
			}
			if err := ms.cleanupRollbackFencedEmail(id); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("clean rollback-fenced email %s: %w", id, err))
			}
			continue
		}
		if !strings.HasPrefix(entry.Name(), storageTempPrefix) {
			continue
		}
		if err := ms.quarantinePath(filepath.Join(ms.mailDir, entry.Name()), "incomplete"); err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("quarantine incomplete artifact %s: %w", entry.Name(), err))
		}
	}
	return errors.Join(recoveryErrors...)
}

func readRollbackFenceState(path string) (string, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(encoded)), nil
}

func rollbackFenceID(name string) (string, bool) {
	if !strings.HasPrefix(name, rollbackFencePrefix) || !strings.HasSuffix(name, rollbackFenceSuffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, rollbackFencePrefix), rollbackFenceSuffix)
	return id, validateEmailID(id) == nil
}

func (ms *MailServer) cleanupRollbackFencedEmail(id string) error {
	var cleanupErrors []error
	if err := os.Remove(filepath.Join(ms.mailDir, id+".eml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := ms.deleteStoredAttachments(id, filepath.Join(ms.mailDir, id)); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := ms.deleteEmailMetadata(id); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if len(cleanupErrors) != 0 {
		return errors.Join(cleanupErrors...)
	}
	// Retain the durable rollback fence after cleanup. This retires the ID and
	// prevents a crash from exposing an EML unlink whose durability was unknown.
	return syncDirectory(ms.mailDir)
}

func (ms *MailServer) quarantineEmail(id, emlPath, reason string) error {
	// Keep the live EML as the startup retry marker until remote cleanup has
	// succeeded. S3 prefix deletion is idempotent, so a later retry is safe.
	if ms.attachmentStore != nil {
		if err := ms.attachmentStore.DeleteEmail(context.Background(), id); err != nil {
			return err
		}
	}
	destination, err := ms.newQuarantineEntry(id, reason)
	if err != nil {
		return err
	}
	attachmentPath := filepath.Join(ms.mailDir, id)
	attachmentsMoved := false
	if _, err := os.Stat(attachmentPath); err == nil {
		if err := os.Rename(attachmentPath, filepath.Join(destination, "attachments")); err != nil {
			_ = os.Remove(destination)
			return err
		}
		attachmentsMoved = true
	} else if !os.IsNotExist(err) {
		_ = os.Remove(destination)
		return err
	}
	if err := os.Rename(emlPath, filepath.Join(destination, "message.eml")); err != nil {
		var rollbackErr error
		if attachmentsMoved {
			rollbackErr = os.Rename(filepath.Join(destination, "attachments"), attachmentPath)
		}
		if rollbackErr == nil {
			_ = os.Remove(destination)
		}
		return errors.Join(err, rollbackErr)
	}
	return syncDirectory(ms.mailDir)
}

func (ms *MailServer) quarantinePath(source, reason string) error {
	if ms.beforeQuarantineMove != nil {
		if err := ms.beforeQuarantineMove(source); err != nil {
			return err
		}
	}
	destination, err := ms.newQuarantineEntry(filepath.Base(source), reason)
	if err != nil {
		return err
	}
	if err := os.Rename(source, filepath.Join(destination, filepath.Base(source))); err != nil {
		return err
	}
	return syncDirectory(ms.mailDir)
}

// isGeneratedEmailID deliberately recognizes only the two ID formats OwlMail
// creates. validateEmailID is a path-safety check and is intentionally broader,
// so using it here would mistake arbitrary user directories for attachments.
func isGeneratedEmailID(value string) bool {
	if len(value) == 8 {
		for _, char := range value {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
				return false
			}
		}
		return true
	}
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == strings.ToLower(value)
}

func (ms *MailServer) newQuarantineEntry(id, reason string) (string, error) {
	root := filepath.Join(ms.mailDir, quarantineDirName)
	if err := os.MkdirAll(root, 0750); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), reason, sanitizeQuarantineName(id))
	destination := filepath.Join(root, name)
	if err := os.Mkdir(destination, 0750); err != nil {
		return "", err
	}
	return destination, nil
}

func sanitizeQuarantineName(value string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, value)
}

func (ms *MailServer) quarantineOrphanAttachmentDirectories() error {
	entries, err := os.ReadDir(ms.mailDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == quarantineDirName || strings.HasPrefix(entry.Name(), storageTempPrefix) {
			continue
		}
		if !isGeneratedEmailID(entry.Name()) {
			continue
		}
		if _, err := os.Stat(filepath.Join(ms.mailDir, entry.Name()+".eml")); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := ms.quarantinePath(filepath.Join(ms.mailDir, entry.Name()), "orphan-attachments"); err != nil {
			return err
		}
	}
	return nil
}
