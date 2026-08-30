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
	storageTempPrefix = ".owlmail-tmp-"
	quarantineDirName = "quarantine"
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
		_ = os.Remove(finalEML)
		_ = os.RemoveAll(finalAttachments)
		return fmt.Errorf("sync mail directory: %w", err)
	}

	if err := ms.SaveEmailToStore(id, false, envelope, email); err != nil {
		_ = os.Remove(finalEML)
		_ = os.RemoveAll(finalAttachments)
		_ = syncDirectory(ms.mailDir)
		committedEML = false
		committedAttachments = false
		return fmt.Errorf("commit email to memory: %w", err)
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
		if !strings.HasPrefix(entry.Name(), storageTempPrefix) {
			continue
		}
		if err := ms.quarantinePath(filepath.Join(ms.mailDir, entry.Name()), "incomplete"); err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("quarantine incomplete artifact %s: %w", entry.Name(), err))
		}
	}
	return errors.Join(recoveryErrors...)
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
