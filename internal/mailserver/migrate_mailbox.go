package mailserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/types"
)

const migrationManifestName = "migration-manifest.json"

// MailboxMigrationOptions describes an offline copy, never an in-place upgrade.
// Offline is the operator's assertion that every writer has been stopped. The
// migration additionally compares source inventories before publishing output.
type MailboxMigrationOptions struct {
	Source          string
	Destination     string
	Direction       string
	Namespace       string
	Subject         string
	MailboxID       string
	ControlDatabase string
	Offline         bool
	afterCopy       func() error
}

type migrationFile struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Size   int64  `json:"size,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// MailboxMigrationResult is also the private, checksummed bundle receipt.
type MailboxMigrationResult struct {
	Version           int             `json:"version"`
	Direction         string          `json:"direction"`
	Namespace         string          `json:"namespace,omitempty"`
	Subject           string          `json:"subject,omitempty"`
	MailboxID         string          `json:"mailboxId"`
	SourceFingerprint string          `json:"sourceFingerprint"`
	SourceControlHash string          `json:"sourceControlHash,omitempty"`
	Emails            int             `json:"emails"`
	Attachments       int             `json:"attachments"`
	Files             []migrationFile `json:"files"`
	AlreadyComplete   bool            `json:"-"`
}

type mailboxMigrationItem struct {
	id       string
	metadata emailMetadata
}

// MigrateMailbox creates destination/{mail,control.sqlite,source-archive} for
// imports, or destination/{mail,source-archive} for a single-person export.
// Publication is one rename on the destination filesystem. Rollback means
// discarding this new bundle: source content is never modified.
func MigrateMailbox(ctx context.Context, options MailboxMigrationOptions) (MailboxMigrationResult, error) {
	var result MailboxMigrationResult
	if ctx == nil || !options.Offline {
		return result, errors.New("offline migration requires an explicit stopped-writer declaration")
	}
	if options.Direction != "standalone-to-multiuser" && options.Direction != "multiuser-to-standalone" {
		return result, errors.New("invalid migration direction")
	}
	source, destination, err := migrationPaths(options.Source, options.Destination)
	if err != nil {
		return result, err
	}
	options.Source = source
	options.Destination = destination
	lockPath := destination + ".migration-lock"
	lock, lockErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if lockErr != nil {
		return result, fmt.Errorf("acquire destination migration lock (remove a stale lock only after verifying no migration is running): %w", lockErr)
	}
	defer func() { _ = lock.Close(); _ = os.Remove(lockPath) }()
	forward := options.Direction == "standalone-to-multiuser"
	if forward && (options.Namespace == "" || options.Subject == "" || options.MailboxID != "" || options.ControlDatabase != "") {
		return result, errors.New("import requires explicit namespace/subject and no source control database or mailbox ID")
	}
	if !forward && (options.MailboxID == "" || options.ControlDatabase == "" || options.Subject != "") {
		return result, errors.New("export requires exactly one mailbox ID and its source control database")
	}
	if err = checkMigrationMode(source, !forward); err != nil {
		return result, err
	}
	if err = rejectMigrationQueues(source, !forward); err != nil {
		return result, err
	}
	before, err := migrationInventory(ctx, source)
	if err != nil {
		return result, err
	}
	fingerprint, err := migrationFingerprint(before)
	if err != nil {
		return result, err
	}
	result = MailboxMigrationResult{Version: 1, Direction: options.Direction, Namespace: options.Namespace, Subject: options.Subject, MailboxID: options.MailboxID, SourceFingerprint: fingerprint}
	if !forward {
		if !filepath.IsAbs(options.ControlDatabase) {
			return result, errors.New("source control database must be absolute")
		}
		if err = rejectSQLiteSidecars(options.ControlDatabase); err != nil {
			return result, err
		}
		result.SourceControlHash, err = migrationHashFile(ctx, options.ControlDatabase)
		if err != nil {
			return result, err
		}
		authority, openErr := mailbox.OpenReadOnly(options.ControlDatabase)
		if openErr != nil {
			return result, openErr
		}
		checkErr := authority.CheckPersonalMailboxID(ctx, options.MailboxID)
		closeErr := authority.Close()
		if checkErr != nil {
			return result, checkErr
		}
		if closeErr != nil {
			return result, closeErr
		}
	}
	if info, statErr := os.Lstat(destination); statErr == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return result, errors.New("migration destination already exists")
		}
		completed, verifyErr := verifyCompletedMigration(ctx, destination, result)
		if verifyErr != nil {
			return result, verifyErr
		}
		completed.AlreadyComplete = true
		return completed, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return result, statErr
	}
	plans, err := planMailboxMigration(ctx, source, options.MailboxID, forward)
	if err != nil {
		return result, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(destination), ".owlmail-migrate-*")
	if err != nil {
		return result, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stage)
		}
	}()
	mailDir := filepath.Join(stage, "mail")
	archive := filepath.Join(stage, "source-archive")
	if err = os.Mkdir(mailDir, 0700); err != nil {
		return result, err
	}
	if err = os.Mkdir(archive, 0700); err != nil {
		return result, err
	}
	if forward {
		if err = copyMigrationInventory(ctx, source, archive, before); err != nil {
			return result, err
		}
		authority, openErr := mailbox.Open(filepath.Join(stage, "control.sqlite"), true)
		if openErr != nil {
			return result, openErr
		}
		binding, bindErr := authority.BindLogin(ctx, mailbox.Login{Namespace: options.Namespace, Subject: options.Subject}, mailbox.ExplicitHistory)
		closeErr := authority.Close()
		if bindErr != nil {
			return result, bindErr
		}
		if closeErr != nil {
			return result, closeErr
		}
		result.MailboxID = binding.MailboxID
		if err = prepareOwnershipDirectory(mailDir, true); err != nil {
			return result, err
		}
	}
	transactionID := uuid.NewString()
	for _, plan := range plans {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		if err = copyMigrationFile(ctx, filepath.Join(source, plan.id+".eml"), filepath.Join(mailDir, plan.id+".eml")); err != nil {
			return result, err
		}
		attachmentDir := filepath.Join(source, plan.id)
		if info, statErr := os.Lstat(attachmentDir); statErr == nil {
			if !info.IsDir() {
				return result, errors.New("attachment path must be a directory")
			}
			files, walkErr := migrationInventory(ctx, attachmentDir)
			if walkErr != nil {
				return result, walkErr
			}
			if err = os.Mkdir(filepath.Join(mailDir, plan.id), 0700); err != nil {
				return result, err
			}
			if err = copyMigrationInventory(ctx, attachmentDir, filepath.Join(mailDir, plan.id), files); err != nil {
				return result, err
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return result, statErr
		}
		// Reverse export archives only the selected person's original material.
		// Never include the full source control DB or another person's files.
		if !forward {
			if err = copyMigrationFile(ctx, filepath.Join(source, plan.id+".eml"), filepath.Join(archive, plan.id+".eml")); err != nil {
				return result, err
			}
			if info, statErr := os.Stat(attachmentDir); statErr == nil && info.IsDir() {
				files, walkErr := migrationInventory(ctx, attachmentDir)
				if walkErr != nil {
					return result, walkErr
				}
				if err = os.Mkdir(filepath.Join(archive, plan.id), 0700); err != nil {
					return result, err
				}
				if err = copyMigrationInventory(ctx, attachmentDir, filepath.Join(archive, plan.id), files); err != nil {
					return result, err
				}
			}
			if err = os.MkdirAll(filepath.Join(archive, metadataDirectoryName), 0700); err != nil {
				return result, err
			}
			if err = copyMigrationFile(ctx, filepath.Join(source, metadataDirectoryName, plan.id+".json"), filepath.Join(archive, metadataDirectoryName, plan.id+".json")); err != nil {
				return result, err
			}
		}
		metadata := plan.metadata
		if forward {
			recipients := []string{}
			if metadata.Envelope != nil {
				for _, address := range metadata.Envelope.To {
					if canonical, normalizeErr := mailbox.NormalizeAddress(address); normalizeErr == nil && !containsMigrationAddress(recipients, canonical) {
						recipients = append(recipients, canonical)
					}
				}
			}
			// Legacy EML without an SMTP envelope has no trustworthy RCPT evidence.
			// This reserved audit address is not installed as a future recipient route.
			if len(recipients) == 0 {
				recipients = []string{"offline-migration@owlmail.invalid"}
			}
			revisions := make([]int64, len(recipients))
			for i := range revisions {
				revisions[i] = 1
			}
			metadata.Ownership = &types.Ownership{MailboxID: result.MailboxID, IngestAccountID: "offline-migration", TransactionID: transactionID, MatchedRecipients: recipients, RouteRevisions: revisions}
		} else {
			metadata.Ownership = nil
		}
		if err = writeMailboxMigrationMetadata(mailDir, metadata, forward); err != nil {
			return result, err
		}
		result.Emails++
		result.Attachments += len(metadata.Attachments)
	}
	// Verify the exported EML, attachment count and hashes with the same parser
	// used for the source. No SMTP, HTTP or delivery recovery is started.
	verified, err := planMailboxMigration(ctx, mailDir, func() string {
		if forward {
			return result.MailboxID
		}
		return ""
	}(), !forward)
	if err != nil {
		return result, fmt.Errorf("verify migrated mail: %w", err)
	}
	if len(verified) != result.Emails {
		return result, errors.New("migration email count mismatch")
	}
	attachmentCount := 0
	for _, item := range verified {
		attachmentCount += len(item.metadata.Attachments)
	}
	if attachmentCount != result.Attachments {
		return result, errors.New("migration attachment count mismatch")
	}
	if options.afterCopy != nil {
		if err = options.afterCopy(); err != nil {
			return result, err
		}
	}
	after, err := migrationInventory(ctx, source)
	if err != nil {
		return result, err
	}
	if !reflect.DeepEqual(before, after) {
		return result, errors.New("source changed during migration; destination was not published")
	}
	if !forward {
		hash, hashErr := migrationHashFile(ctx, options.ControlDatabase)
		if hashErr != nil {
			return result, hashErr
		}
		if hash != result.SourceControlHash {
			return result, errors.New("source control database changed during export")
		}
		if err = rejectSQLiteSidecars(options.ControlDatabase); err != nil {
			return result, err
		}
	}
	result.Files, err = migrationInventory(ctx, stage)
	if err != nil {
		return result, err
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return result, err
	}
	if err = writeMigrationFile(filepath.Join(stage, migrationManifestName), encoded); err != nil {
		return result, err
	}
	if err = syncMigrationDirectories(stage); err != nil {
		return result, err
	}
	if _, err = os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return result, errors.New("migration destination appeared during copy")
	}
	if err = os.Rename(stage, destination); err != nil {
		return result, err
	}
	published = true
	if err = syncDirectory(filepath.Dir(destination)); err != nil {
		return result, fmt.Errorf("migration published but parent sync failed; verify existing bundle before retry: %w", err)
	}
	return result, nil
}
func containsMigrationAddress(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func migrationPaths(source, destination string) (string, string, error) {
	if !filepath.IsAbs(source) || !filepath.IsAbs(destination) {
		return "", "", errors.New("source and destination must be absolute")
	}
	info, err := os.Lstat(source)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("source must be a real directory")
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return "", "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		return "", "", err
	}
	destination = filepath.Join(parent, filepath.Base(destination))
	for _, pair := range [][2]string{{source, destination}, {destination, source}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err != nil {
			return "", "", err
		}
		if relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", "", errors.New("source and destination cannot contain each other")
		}
	}
	return source, destination, nil
}
func checkMigrationMode(source string, isolated bool) error {
	info, err := os.Lstat(filepath.Join(source, ownershipMarker))
	if isolated {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("invalid source mode marker")
		}
		data, err := os.ReadFile(filepath.Join(source, ownershipMarker))
		if err != nil {
			return err
		}
		if string(data) != ownershipMarkerContent {
			return errors.New("unsupported source mode")
		}
		return nil
	}
	if err == nil {
		return errors.New("standalone import cannot read a multiuser directory")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
func rejectMigrationQueues(source string, export bool) error {
	for _, name := range []string{filepath.Join(metadataDirectoryName, relayJobMetadataDirectoryName), filepath.Join(metadataDirectoryName, relayIntentMetadataDirectoryName), webhookOutboxDirectoryName, ingressDirectory} {
		entries, err := os.ReadDir(filepath.Join(source, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return fmt.Errorf("mailbox migration requires drained delivery/recovery state: %s; archive or settle these records explicitly first", name)
		}
	}
	// Global history can safely remain in the complete standalone backup, but
	// cannot be copied into one person's export without mixing other users.
	if export {
		entries, err := os.ReadDir(filepath.Join(source, metadataDirectoryName, "webhook-history"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if len(entries) > 0 {
			return errors.New("single-mailbox export requires separately archived global delivery history")
		}
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), storageTempPrefix) || strings.HasPrefix(e.Name(), runtimeReadinessPrefix) {
			return errors.New("source has live or unresolved runtime artifacts; stop writers and recover before migration")
		}
	}
	return nil
}
func rejectSQLiteSidecars(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("source control database must be a regular file")
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		info, err := os.Lstat(path + suffix)
		if err == nil && info.Size() > 0 {
			return errors.New("source control database has live journal files; close/checkpoint writers before export")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
func planMailboxMigration(ctx context.Context, source, selected string, standalone bool) ([]mailboxMigrationItem, error) {
	reader := &MailServer{mailDir: source, requireOwnership: !standalone}
	entries, err := os.ReadDir(source)
	if err != nil {
		return nil, err
	}
	var result []mailboxMigrationItem
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".eml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".eml")
		if err = validateEmailID(id); err != nil {
			return nil, err
		}
		metadata, metaErr := reader.loadEmailMetadata(id)
		if !standalone {
			if metaErr != nil {
				return nil, metaErr
			}
			if metadata.Ownership == nil {
				return nil, errors.New("isolated source contains unowned email")
			}
			if metadata.Ownership.MailboxID != selected {
				continue
			}
		}
		if standalone && metaErr != nil && !errors.Is(metaErr, os.ErrNotExist) {
			return nil, metaErr
		}
		if standalone && metadata.Ownership != nil {
			return nil, errors.New("standalone source contains isolated ownership")
		}
		file, err := os.Open(filepath.Join(source, entry.Name()))
		if err != nil {
			return nil, err
		}
		email, envelope, parseErr := reader.parseEmailMessage(id, file, nil, false, filepath.Join(source, id))
		closeErr := file.Close()
		if parseErr != nil {
			return nil, parseErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if errors.Is(metaErr, os.ErrNotExist) {
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			metadata = emailMetadata{ID: id, Version: currentMetadataVersion, Sequence: info.ModTime().UTC(), Envelope: envelope}
		}
		if metadata.Envelope == nil {
			metadata.Envelope = envelope
		}
		if len(metadata.Attachments) == 0 && len(email.Attachments) > 0 {
			if err = reader.restoreLegacyLocalAttachmentMetadataContext(ctx, id, email.Attachments); err != nil {
				return nil, err
			}
			for _, attachment := range email.Attachments {
				metadata.Attachments = append(metadata.Attachments, attachmentMetadata{GeneratedFileName: attachment.GeneratedFileName, Size: attachment.Size, ContentSHA256: attachment.ContentSHA256, Storage: attachmentStorageLocal})
			}
		}
		if len(metadata.Attachments) != len(email.Attachments) {
			return nil, errors.New("MIME and attachment metadata counts differ")
		}
		referenced := map[string]struct{}{}
		for index, attachment := range metadata.Attachments {
			if attachment.Storage != "" && attachment.Storage != attachmentStorageLocal {
				return nil, errors.New("remote attachments must be materialized and verified locally before mailbox migration")
			}
			if err = validateAttachmentFilename(attachment.GeneratedFileName); err != nil {
				return nil, err
			}
			if _, exists := referenced[attachment.GeneratedFileName]; exists {
				return nil, errors.New("duplicate attachment filename")
			}
			referenced[attachment.GeneratedFileName] = struct{}{}
			path := filepath.Join(source, id, attachment.GeneratedFileName)
			info, err := os.Lstat(path)
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() || info.Size() != attachment.Size {
				return nil, errors.New("attachment size or type mismatch")
			}
			hash, err := migrationHashFile(ctx, path)
			if err != nil {
				return nil, err
			}
			if attachment.ContentSHA256 != "" && hash != attachment.ContentSHA256 {
				return nil, errors.New("attachment checksum mismatch")
			}
			parsed := email.Attachments[index]
			if parsed == nil || parsed.Size != attachment.Size || parsed.ContentSHA256 != hash {
				return nil, errors.New("local attachment differs from original MIME")
			}
			metadata.Attachments[index].ContentSHA256 = hash
			metadata.Attachments[index].Storage = attachmentStorageLocal

		}
		if err = rejectUnmappedLocalFiles(ctx, source, id, referenced); err != nil {
			return nil, err
		}
		result = append(result, mailboxMigrationItem{id: id, metadata: metadata})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].id < result[j].id })
	return result, nil
}
func writeMailboxMigrationMetadata(directory string, metadata emailMetadata, isolated bool) error {
	writer := &MailServer{mailDir: directory, requireOwnership: isolated}
	email := &Email{ID: metadata.ID, Read: metadata.Read, Envelope: restoreMetadataEnvelope(metadata), RelayedAt: cloneTime(metadata.RelayedAt), RelayedTo: append([]string(nil), metadata.RelayedTo...), Ownership: cloneOwnership(metadata.Ownership)}
	for _, saved := range metadata.Attachments {
		email.Attachments = append(email.Attachments, &Attachment{GeneratedFileName: saved.GeneratedFileName, Size: saved.Size, ContentSHA256: saved.ContentSHA256, Storage: saved.Storage})
	}
	return writer.persistEmailMetadataAt(email, metadata.Sequence)
}
