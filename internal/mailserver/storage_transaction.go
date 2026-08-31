package mailserver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	storageTempPrefix   = ".owlmail-tmp-"
	rollbackFencePrefix = storageTempPrefix + "rollback-"
	rollbackFenceSuffix = ".fence"
	quarantineDirName   = "quarantine"
)

// storeIncomingEmail commits attachments first and the EML file last. The EML
// rename is the transaction marker observed by startup recovery.
func (ms *MailServer) storeIncomingEmail(id string, r io.Reader, session *Session) error {
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
			_ = os.RemoveAll(finalAttachments)
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
		if err := os.Rename(stagedAttachments, finalAttachments); err != nil {
			return fmt.Errorf("commit attachments: %w", err)
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

	if err := ms.SaveEmailToStore(id, false, envelope, email); err != nil {
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

// rollbackIncomingEmail durably fences the message before cleanup. If removal
// later fails, startup recovery will skip and retry cleanup for the fenced ID
// instead of loading a message that SMTP rejected.
func (ms *MailServer) rollbackIncomingEmail(id, emlPath, attachmentPath string) error {
	fencePath := rollbackFencePath(ms.mailDir, id)
	fence, err := os.OpenFile(fencePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create email rollback fence: %w", err)
	}
	if _, err := fence.WriteString(id + "\n"); err != nil {
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
	if err := os.RemoveAll(attachmentPath); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := syncDirectory(ms.mailDir); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if len(cleanupErrors) != 0 {
		// The durable fence is the rollback confirmation. Keep it so recovery
		// cannot publish any surviving EML after restart.
		return nil
	}
	if err := os.Remove(fencePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	// Failure to persist fence removal is harmless: a stale fence with no EML
	// is cleaned during recovery and cannot resurrect the rejected message.
	_ = syncDirectory(ms.mailDir)
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
		if id, ok := rollbackFenceID(entry.Name()); ok {
			if err := ms.cleanupRollbackFencedEmail(id, filepath.Join(ms.mailDir, entry.Name())); err != nil {
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

func rollbackFenceID(name string) (string, bool) {
	if !strings.HasPrefix(name, rollbackFencePrefix) || !strings.HasSuffix(name, rollbackFenceSuffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, rollbackFencePrefix), rollbackFenceSuffix)
	return id, validateEmailID(id) == nil
}

func (ms *MailServer) cleanupRollbackFencedEmail(id, fencePath string) error {
	var cleanupErrors []error
	if err := os.Remove(filepath.Join(ms.mailDir, id+".eml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := os.RemoveAll(filepath.Join(ms.mailDir, id)); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if err := ms.deleteEmailMetadata(id); err != nil {
		cleanupErrors = append(cleanupErrors, err)
	}
	if len(cleanupErrors) != 0 {
		return errors.Join(cleanupErrors...)
	}
	if err := os.Remove(fencePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDirectory(ms.mailDir)
}

func (ms *MailServer) quarantineEmail(id, emlPath, reason string) error {
	destination, err := ms.newQuarantineEntry(id, reason)
	if err != nil {
		return err
	}
	if err := os.Rename(emlPath, filepath.Join(destination, "message.eml")); err != nil {
		return err
	}
	attachmentPath := filepath.Join(ms.mailDir, id)
	if _, err := os.Stat(attachmentPath); err == nil {
		if err := os.Rename(attachmentPath, filepath.Join(destination, "attachments")); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
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
