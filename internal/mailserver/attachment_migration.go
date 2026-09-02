package mailserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
)

const (
	attachmentMigrationCopyBufferSize = 32 * 1024
	maxAttachmentMigrationRetries     = 100
)

// AttachmentMigrationOptions configures the offline local-to-S3 migration.
// Retries is the number of attempts after the initial attempt.
type AttachmentMigrationOptions struct {
	DryRun         bool
	DeleteLocal    bool
	Retries        int
	AttemptTimeout time.Duration
	RetryDelay     time.Duration
	Progress       func(AttachmentMigrationProgress)

	// Fault hooks are intentionally unexported and nil in production. They
	// model process interruption around the durable metadata boundary.
	afterRemoteVerified func(emailID, filename string) error
	afterMetadataCommit func(emailID, filename string) error
}

// AttachmentMigrationProgress reports one observable migration transition.
type AttachmentMigrationProgress struct {
	EmailID  string
	Filename string
	Status   string
	Attempt  int
	Err      error
}

// AttachmentMigrationSummary reports the final migration counters.
type AttachmentMigrationSummary struct {
	EmailsScanned      int `json:"emailsScanned"`
	AttachmentsScanned int `json:"attachmentsScanned"`
	Planned            int `json:"planned"`
	Uploaded           int `json:"uploaded"`
	Verified           int `json:"verified"`
	AlreadyMigrated    int `json:"alreadyMigrated"`
	LocalFilesDeleted  int `json:"localFilesDeleted"`
	RetryAttempts      int `json:"retryAttempts"`
	Failed             int `json:"failed"`
}

type attachmentMigrationPlan struct {
	emailID     string
	metadata    emailMetadata
	attachments []attachmentMigrationItem
}

type attachmentMigrationItem struct {
	index       int
	filename    string
	contentType string
	size        int64
	sha256      string
	localPath   string
	localExists bool
}

// MigrateLocalAttachments migrates decoded local attachments to the configured
// external store without starting SMTP or HTTP services. The caller must stop
// OwlMail before invoking it. Every source is preflighted before the first
// remote write, and migration state is committed atomically per attachment.
func MigrateLocalAttachments(ctx context.Context, mailDir string, store attachmentstore.Store, options AttachmentMigrationOptions) (AttachmentMigrationSummary, error) {
	var summary AttachmentMigrationSummary
	if ctx == nil {
		return summary, fmt.Errorf("migration context is nil")
	}
	if strings.TrimSpace(mailDir) == "" {
		return summary, fmt.Errorf("mail directory is required")
	}
	if store == nil {
		return summary, fmt.Errorf("attachment store is required")
	}
	if options.Retries < 0 {
		return summary, fmt.Errorf("migration retries cannot be negative")
	}
	if options.Retries > maxAttachmentMigrationRetries {
		return summary, fmt.Errorf("migration retries cannot exceed %d", maxAttachmentMigrationRetries)
	}
	if options.AttemptTimeout <= 0 {
		return summary, fmt.Errorf("migration attempt timeout must be positive")
	}
	if options.RetryDelay < 0 {
		return summary, fmt.Errorf("migration retry delay cannot be negative")
	}

	plans, err := preflightAttachmentMigration(ctx, mailDir)
	if err != nil {
		return summary, err
	}
	var migrationErrors []error
	for _, plan := range plans {
		summary.EmailsScanned++
		summary.AttachmentsScanned += len(plan.attachments)
		for _, item := range plan.attachments {
			alreadyMigrated := plan.metadata.Attachments[item.index].Storage == attachmentStorageS3
			if !alreadyMigrated {
				summary.Planned++
			}
			if !options.DryRun {
				continue
			}
			if !alreadyMigrated && item.localExists {
				reportAttachmentMigration(options, item, plan.emailID, "planned", 0, nil)
				continue
			}

			attempts, verifyErr := retryAttachmentMigration(ctx, options, item, plan.emailID, false, store)
			summary.RetryAttempts += attempts - 1
			if verifyErr != nil {
				summary.Failed++
				migrationErrors = append(migrationErrors, fmt.Errorf("dry-run verify remote attachment %s/%s: %w", plan.emailID, item.filename, verifyErr))
				reportAttachmentMigration(options, item, plan.emailID, "failed", attempts, verifyErr)
				continue
			}
			summary.AlreadyMigrated++
			summary.Verified++
			status := "remote-verified"
			if alreadyMigrated {
				status = "already-migrated"
			}
			reportAttachmentMigration(options, item, plan.emailID, status, attempts, nil)
		}
	}
	if options.DryRun {
		return summary, errors.Join(migrationErrors...)
	}

	for planIndex := range plans {
		plan := &plans[planIndex]
		for itemIndex := range plan.attachments {
			item := &plan.attachments[itemIndex]
			if err := ctx.Err(); err != nil {
				return summary, errors.Join(errors.Join(migrationErrors...), err)
			}

			alreadyMigrated := plan.metadata.Attachments[item.index].Storage == attachmentStorageS3
			verified := false
			if alreadyMigrated || !item.localExists {
				attempts, verifyErr := retryAttachmentMigration(ctx, options, *item, plan.emailID, false, store)
				summary.RetryAttempts += attempts - 1
				if verifyErr == nil {
					verified = true
					summary.AlreadyMigrated++
				} else if !item.localExists {
					summary.Failed++
					migrationErrors = append(migrationErrors, fmt.Errorf("verify migrated attachment %s/%s: %w", plan.emailID, item.filename, verifyErr))
					reportAttachmentMigration(options, *item, plan.emailID, "failed", attempts, verifyErr)
					continue
				}
			}

			if !verified {
				attempts, uploadErr := retryAttachmentMigration(ctx, options, *item, plan.emailID, true, store)
				summary.RetryAttempts += attempts - 1
				if uploadErr != nil {
					summary.Failed++
					migrationErrors = append(migrationErrors, fmt.Errorf("migrate attachment %s/%s: %w", plan.emailID, item.filename, uploadErr))
					reportAttachmentMigration(options, *item, plan.emailID, "failed", attempts, uploadErr)
					continue
				}
				summary.Uploaded++
			}
			summary.Verified++
			reportAttachmentMigration(options, *item, plan.emailID, "verified", 0, nil)

			if options.afterRemoteVerified != nil {
				if err := options.afterRemoteVerified(plan.emailID, item.filename); err != nil {
					summary.Failed++
					migrationErrors = append(migrationErrors, fmt.Errorf("after remote verification for %s/%s: %w", plan.emailID, item.filename, err))
					continue
				}
			}

			metadataAttachment := &plan.metadata.Attachments[item.index]
			metadataAttachment.ContentSHA256 = item.sha256
			metadataAttachment.Storage = attachmentStorageS3
			plan.metadata.Version = currentMetadataVersion
			if err := persistMigrationMetadata(mailDir, plan.metadata); err != nil {
				summary.Failed++
				migrationErrors = append(migrationErrors, fmt.Errorf("commit migration metadata for %s/%s: %w", plan.emailID, item.filename, err))
				continue
			}
			reportAttachmentMigration(options, *item, plan.emailID, "metadata-committed", 0, nil)

			if options.afterMetadataCommit != nil {
				if err := options.afterMetadataCommit(plan.emailID, item.filename); err != nil {
					summary.Failed++
					migrationErrors = append(migrationErrors, fmt.Errorf("after metadata commit for %s/%s: %w", plan.emailID, item.filename, err))
					continue
				}
			}
			if options.DeleteLocal && item.localExists {
				if err := deleteMigratedLocalAttachment(mailDir, plan.emailID, item.localPath); err != nil {
					summary.Failed++
					migrationErrors = append(migrationErrors, fmt.Errorf("delete migrated local attachment %s/%s: %w", plan.emailID, item.filename, err))
					continue
				}
				item.localExists = false
				summary.LocalFilesDeleted++
				reportAttachmentMigration(options, *item, plan.emailID, "local-deleted", 0, nil)
			}
		}
	}
	return summary, errors.Join(migrationErrors...)
}

func preflightAttachmentMigration(ctx context.Context, mailDir string) ([]attachmentMigrationPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stat, err := os.Stat(mailDir)
	if err != nil {
		return nil, fmt.Errorf("inspect mail directory: %w", err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("mail directory is not a directory")
	}
	entries, err := os.ReadDir(mailDir)
	if err != nil {
		return nil, fmt.Errorf("read mail directory: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reader := &MailServer{mailDir: mailDir}
	plans := make([]attachmentMigrationPlan, 0)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".eml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".eml")
		if err := validateEmailID(id); err != nil {
			return nil, fmt.Errorf("invalid email filename %q: %w", entry.Name(), err)
		}
		if migrationFenceExists(mailDir, id) {
			return nil, fmt.Errorf("email %s has a pending storage transaction; run normal startup recovery before migration", id)
		}

		metadata, metadataErr := reader.loadEmailMetadata(id)
		eml, err := os.Open(filepath.Join(mailDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("open email %s: %w", id, err)
		}
		email, _, parseErr := reader.parseEmailMessage(id, contextReader{ctx: ctx, reader: eml}, nil, false, "")
		closeErr := eml.Close()
		if parseErr != nil || closeErr != nil {
			return nil, fmt.Errorf("parse email %s: %w", id, errors.Join(parseErr, closeErr))
		}
		if err := validateLocalAttachmentDirectory(mailDir, id); err != nil {
			return nil, fmt.Errorf("validate local attachment directory for %s: %w", id, err)
		}
		if len(email.Attachments) == 0 {
			if metadataErr != nil && !errors.Is(metadataErr, os.ErrNotExist) {
				return nil, fmt.Errorf("load attachment metadata for %s: %w", id, metadataErr)
			}
			if metadataErr == nil && len(metadata.Attachments) != 0 {
				return nil, fmt.Errorf("ambiguous attachment mapping for %s: metadata count %d does not match MIME count 0", id, len(metadata.Attachments))
			}
			if err := rejectUnmappedLocalFiles(ctx, mailDir, id, map[string]struct{}{}); err != nil {
				return nil, err
			}
			plans = append(plans, attachmentMigrationPlan{emailID: id})
			continue
		}
		metadataMissing := errors.Is(metadataErr, os.ErrNotExist)
		if metadataErr != nil && !metadataMissing {
			return nil, fmt.Errorf("load attachment metadata for %s: %w", id, metadataErr)
		}
		if metadataMissing {
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("inspect legacy email %s: %w", id, err)
			}
			metadata = emailMetadata{Version: 1, ID: id, Sequence: info.ModTime().UTC()}
		}
		if len(metadata.Attachments) == 0 && metadata.Version < currentMetadataVersion {
			if err := reader.restoreLegacyLocalAttachmentMetadataContext(ctx, id, email.Attachments); err != nil {
				return nil, fmt.Errorf("ambiguous legacy attachment mapping for %s: %w", id, err)
			}
			metadata.Attachments = make([]attachmentMetadata, 0, len(email.Attachments))
			for _, attachment := range email.Attachments {
				metadata.Attachments = append(metadata.Attachments, attachmentMetadata{
					GeneratedFileName: attachment.GeneratedFileName,
					Size:              attachment.Size,
					ContentSHA256:     attachment.ContentSHA256,
					Storage:           attachmentStorageLocal,
				})
			}
		}
		if len(metadata.Attachments) != len(email.Attachments) {
			return nil, fmt.Errorf("ambiguous attachment mapping for %s: metadata count %d does not match MIME count %d", id, len(metadata.Attachments), len(email.Attachments))
		}

		plan := attachmentMigrationPlan{emailID: id, metadata: metadata}
		seenFilenames := make(map[string]struct{}, len(metadata.Attachments))
		referencedFiles := make(map[string]struct{}, len(metadata.Attachments))
		for index, saved := range metadata.Attachments {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			parsed := email.Attachments[index]
			if parsed == nil {
				return nil, fmt.Errorf("ambiguous attachment mapping for %s: MIME attachment %d is missing", id, index)
			}
			if err := validateAttachmentFilename(saved.GeneratedFileName); err != nil {
				return nil, fmt.Errorf("ambiguous attachment mapping for %s: %w", id, err)
			}
			if _, duplicate := seenFilenames[saved.GeneratedFileName]; duplicate {
				return nil, fmt.Errorf("ambiguous attachment mapping for %s: generated filename %q is duplicated", id, saved.GeneratedFileName)
			}
			seenFilenames[saved.GeneratedFileName] = struct{}{}
			referencedFiles[saved.GeneratedFileName] = struct{}{}
			if saved.Size < 0 || saved.Size != parsed.Size {
				return nil, fmt.Errorf("attachment size verification failed for %s/%s: metadata=%d MIME=%d", id, saved.GeneratedFileName, saved.Size, parsed.Size)
			}
			expectedSHA256 := parsed.ContentSHA256
			if saved.ContentSHA256 != "" && !strings.EqualFold(saved.ContentSHA256, expectedSHA256) {
				return nil, fmt.Errorf("attachment SHA-256 verification failed for %s/%s: metadata and MIME differ", id, saved.GeneratedFileName)
			}
			localPath := filepath.Join(mailDir, id, saved.GeneratedFileName)
			if err := validatePath(mailDir, localPath); err != nil {
				return nil, fmt.Errorf("validate attachment path for %s/%s: %w", id, saved.GeneratedFileName, err)
			}
			localExists, err := verifyLocalMigrationSource(ctx, localPath, saved.Size, expectedSHA256)
			if err != nil {
				return nil, fmt.Errorf("verify local attachment %s/%s: %w", id, saved.GeneratedFileName, err)
			}
			plan.attachments = append(plan.attachments, attachmentMigrationItem{
				index: index, filename: saved.GeneratedFileName, contentType: parsed.ContentType,
				size: saved.Size, sha256: expectedSHA256, localPath: localPath, localExists: localExists,
			})
		}
		if err := rejectUnmappedLocalFiles(ctx, mailDir, id, referencedFiles); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func migrationFenceExists(mailDir, id string) bool {
	for _, path := range []string{rollbackFencePath(mailDir, id), deletionFencePath(mailDir, id)} {
		if _, err := os.Stat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}

func validateLocalAttachmentDirectory(mailDir, id string) error {
	directory := filepath.Join(mailDir, id)
	if err := validatePath(mailDir, directory); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("attachment directory is a symbolic link")
	}
	if !info.IsDir() {
		return fmt.Errorf("attachment path is not a directory")
	}
	return nil
}

func verifyLocalMigrationSource(ctx context.Context, path string, expectedSize int64, expectedSHA256 string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	stat, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !stat.Mode().IsRegular() {
		return false, fmt.Errorf("source is not a regular file")
	}
	if stat.Size() != expectedSize {
		return false, fmt.Errorf("size mismatch: metadata=%d local=%d", expectedSize, stat.Size())
	}
	digest, err := attachmentFileSHA256Context(ctx, path)
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(digest, expectedSHA256) {
		return false, fmt.Errorf("SHA-256 mismatch")
	}
	return true, nil
}

func rejectUnmappedLocalFiles(ctx context.Context, mailDir, id string, referenced map[string]struct{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	directory := filepath.Join(mailDir, id)
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read local attachment directory for %s: %w", id, err)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return fmt.Errorf("ambiguous attachment mapping for %s: unexpected directory %q", id, entry.Name())
		}
		if _, ok := referenced[entry.Name()]; !ok {
			return fmt.Errorf("ambiguous attachment mapping for %s: local file %q is not referenced by metadata", id, entry.Name())
		}
	}
	return nil
}

func retryAttachmentMigration(ctx context.Context, options AttachmentMigrationOptions, item attachmentMigrationItem, emailID string, upload bool, store attachmentstore.Store) (int, error) {
	maxAttempts := options.Retries + 1
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, options.AttemptTimeout)
		if upload {
			lastErr = uploadAndVerifyMigrationAttachment(attemptCtx, store, emailID, item)
		} else {
			lastErr = verifyRemoteMigrationAttachment(attemptCtx, store, emailID, item)
		}
		cancel()
		if lastErr == nil {
			return attempt, nil
		}
		if attempt == maxAttempts {
			break
		}
		reportAttachmentMigration(options, item, emailID, "retrying", attempt, lastErr)
		if err := waitForMigrationRetry(ctx, options.RetryDelay); err != nil {
			return attempt, errors.Join(lastErr, err)
		}
	}
	return maxAttempts, lastErr
}

func uploadAndVerifyMigrationAttachment(ctx context.Context, store attachmentstore.Store, emailID string, item attachmentMigrationItem) error {
	if !item.localExists {
		return fmt.Errorf("local source is unavailable")
	}
	file, err := os.Open(item.localPath)
	if err != nil {
		return fmt.Errorf("open local source: %w", err)
	}
	putErr := store.Put(ctx, emailID, item.filename, item.contentType, file, item.size)
	closeErr := file.Close()
	if err := errors.Join(putErr, closeErr); err != nil {
		return err
	}
	return verifyRemoteMigrationAttachment(ctx, store, emailID, item)
}

func verifyRemoteMigrationAttachment(ctx context.Context, store attachmentstore.Store, emailID string, item attachmentMigrationItem) error {
	object, err := store.Open(ctx, emailID, item.filename)
	if err != nil {
		return err
	}
	if object == nil || object.Body == nil {
		return fmt.Errorf("remote attachment response body is empty")
	}
	if object.Size >= 0 && object.Size != item.size {
		_ = object.Body.Close()
		return fmt.Errorf("remote attachment size mismatch: metadata=%d remote=%d", item.size, object.Size)
	}
	digest := sha256.New()
	written, copyErr := io.CopyBuffer(digest, object.Body, make([]byte, attachmentMigrationCopyBufferSize))
	closeErr := object.Body.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return err
	}
	if written != item.size {
		return fmt.Errorf("remote attachment size mismatch: metadata=%d remote=%d", item.size, written)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); !strings.EqualFold(got, item.sha256) {
		return fmt.Errorf("remote attachment SHA-256 mismatch")
	}
	return nil
}

func persistMigrationMetadata(mailDir string, metadata emailMetadata) error {
	if err := validateEmailID(metadata.ID); err != nil {
		return err
	}
	server := &MailServer{mailDir: mailDir}
	// Reuse the normal atomic temp-write, fsync, rename, and directory-fsync
	// path without constructing or starting a MailServer.
	email := &Email{ID: metadata.ID, Read: metadata.Read, Attachments: make([]*Attachment, 0, len(metadata.Attachments))}
	for _, saved := range metadata.Attachments {
		email.Attachments = append(email.Attachments, &Attachment{
			GeneratedFileName: saved.GeneratedFileName,
			Size:              saved.Size,
			ContentSHA256:     saved.ContentSHA256,
			Storage:           saved.Storage,
		})
	}
	return server.persistEmailMetadataAt(email, metadata.Sequence)
}

func deleteMigratedLocalAttachment(mailDir, emailID, path string) error {
	if err := validatePath(filepath.Join(mailDir, emailID), path); err != nil {
		return err
	}
	if err := validateLocalAttachmentDirectory(mailDir, emailID); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	directory := filepath.Dir(path)
	if err := syncDirectory(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		if err := os.Remove(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return syncDirectory(mailDir)
	}
	return nil
}

func waitForMigrationRetry(ctx context.Context, delay time.Duration) error {
	if delay == 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func reportAttachmentMigration(options AttachmentMigrationOptions, item attachmentMigrationItem, emailID, status string, attempt int, err error) {
	if options.Progress == nil {
		return
	}
	options.Progress(AttachmentMigrationProgress{
		EmailID: emailID, Filename: item.filename, Status: status, Attempt: attempt, Err: err,
	})
}
