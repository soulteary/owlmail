package mailserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (ms *MailServer) uploadAttachments(id, stagingDirectory string, attachments []*Attachment) error {
	for _, attachment := range attachments {
		if attachment == nil || attachment.GeneratedFileName == "" {
			cleanupErr := ms.deleteRemoteAttachments(id)
			return errors.Join(fmt.Errorf("attachment metadata is incomplete"), cleanupErr)
		}
		attachmentPath := filepath.Join(stagingDirectory, attachment.GeneratedFileName)
		if err := validatePath(stagingDirectory, attachmentPath); err != nil {
			cleanupErr := ms.deleteRemoteAttachments(id)
			return errors.Join(fmt.Errorf("validate staged attachment path: %w", err), cleanupErr)
		}
		file, err := os.Open(attachmentPath)
		if err != nil {
			cleanupErr := ms.deleteRemoteAttachments(id)
			return errors.Join(fmt.Errorf("open staged attachment: %w", err), cleanupErr)
		}
		stat, statErr := file.Stat()
		if statErr == nil {
			err = ms.putRemoteAttachment(
				id,
				attachment.GeneratedFileName,
				attachment.ContentType,
				file,
				stat.Size(),
			)
		}
		closeErr := file.Close()
		if statErr != nil || err != nil || closeErr != nil {
			cleanupErr := ms.deleteRemoteAttachments(id)
			return errors.Join(statErr, err, closeErr, cleanupErr)
		}
	}
	return nil
}

// putRemoteAttachment bounds each S3 upload so a stalled endpoint cannot keep
// a storage read lock forever and indirectly block later transactions waiting
// behind a writer.
func (ms *MailServer) putRemoteAttachment(id, filename, contentType string, body io.Reader, size int64) error {
	timeout := ms.attachmentUploadTimeout
	if timeout <= 0 {
		timeout = defaultAttachmentUploadTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return ms.attachmentStore.Put(ctx, id, filename, contentType, body, size)
}

func (ms *MailServer) deleteStoredAttachments(id, localPath string) error {
	if ms.attachmentStore != nil {
		return errors.Join(
			ms.deleteRemoteAttachments(id),
			os.RemoveAll(localPath),
		)
	}
	return os.RemoveAll(localPath)
}

// deleteRemoteAttachments bounds S3 cleanup so a stalled endpoint cannot hold
// the storage transaction lock indefinitely. Prefix deletion is idempotent;
// callers retain their durable retry marker when this deadline expires.
func (ms *MailServer) deleteRemoteAttachments(id string) error {
	if ms.attachmentStore == nil {
		return nil
	}
	timeout := ms.attachmentDeleteTimeout
	if timeout <= 0 {
		timeout = defaultAttachmentDeleteTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return ms.attachmentStore.DeleteEmail(ctx, id)
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (body *cancelOnCloseReadCloser) Close() error {
	defer body.cancel()
	return body.ReadCloser.Close()
}

func (ms *MailServer) findAttachment(id, filename string) (*Attachment, error) {
	if err := validateEmailID(id); err != nil {
		return nil, fmt.Errorf("invalid email ID: %w", err)
	}
	if err := validateAttachmentFilename(filename); err != nil {
		return nil, err
	}

	email, err := ms.GetEmail(id)
	if err != nil {
		return nil, err
	}
	if len(email.Attachments) == 0 {
		return nil, fmt.Errorf("email has no attachments")
	}
	for _, attachment := range email.Attachments {
		if attachment != nil && attachment.GeneratedFileName == filename {
			return attachment, nil
		}
	}
	return nil, fmt.Errorf("attachment not found")
}

func validateAttachmentFilename(filename string) error {
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "\x00") {
		return fmt.Errorf("invalid filename: contains path traversal characters")
	}
	return nil
}

// restoreLegacyLocalAttachmentMetadata recovers generated filenames for mail
// written before attachment names were persisted in metadata. Matching by size
// and extension handles existing local mail without changing its files.
func (ms *MailServer) restoreLegacyLocalAttachmentMetadata(id string, attachments []*Attachment) error {
	directory := filepath.Join(ms.mailDir, id)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	type candidate struct {
		name          string
		size          int64
		contentSHA256 string
	}
	files := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := validateAttachmentFilename(entry.Name()); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		contentSHA256, err := attachmentFileSHA256(filepath.Join(directory, entry.Name()))
		if err != nil {
			return err
		}
		files = append(files, candidate{name: entry.Name(), size: info.Size(), contentSHA256: contentSHA256})
	}
	if len(files) != len(attachments) {
		return fmt.Errorf("legacy attachment file count does not match message")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	used := make([]bool, len(files))
	assignments := make([]int, len(attachments))
	for attachmentIndex, attachment := range attachments {
		if attachment == nil {
			return fmt.Errorf("legacy attachment metadata is incomplete")
		}
		if attachment.ContentSHA256 == "" {
			return fmt.Errorf("legacy attachment content digest is missing")
		}
		expectedExtension := attachmentExtension(attachment)
		matches := make([]int, 0, 1)
		for i, file := range files {
			if used[i] || file.size != attachment.Size || file.contentSHA256 != attachment.ContentSHA256 {
				continue
			}
			if expectedExtension != "" && !strings.EqualFold(filepath.Ext(file.name), expectedExtension) {
				continue
			}
			matches = append(matches, i)
		}
		if len(matches) != 1 {
			return fmt.Errorf("legacy attachment file cannot be uniquely matched")
		}
		match := matches[0]
		used[match] = true
		assignments[attachmentIndex] = match
	}
	for attachmentIndex, match := range assignments {
		attachment := attachments[attachmentIndex]
		attachment.GeneratedFileName = files[match].name
		attachment.Transformed = true
	}
	return nil
}

func attachmentFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func attachmentExtension(attachment *Attachment) string {
	extension := filepath.Ext(attachment.FileName)
	if extension != "" {
		return extension
	}
	if attachment.ContentType != "" {
		extensions, _ := mime.ExtensionsByType(attachment.ContentType)
		if len(extensions) > 0 {
			return extensions[0]
		}
	}
	return ".bin"
}

// OpenEmailAttachment opens a local or S3-backed attachment for streaming.
func (ms *MailServer) OpenEmailAttachment(id, filename string) (*AttachmentReader, error) {
	attachment, err := ms.findAttachment(id, filename)
	if err != nil {
		return nil, err
	}
	attachmentPath := filepath.Join(ms.mailDir, id, attachment.GeneratedFileName)
	if err := validatePath(ms.mailDir, attachmentPath); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}
	if ms.attachmentStore != nil {
		// Existing mail may predate S3 enablement. Prefer an existing local
		// attachment so enabling S3 does not make that mail unreadable.
		if file, err := os.Open(attachmentPath); err == nil {
			stat, statErr := file.Stat()
			if statErr != nil {
				_ = file.Close()
				return nil, fmt.Errorf("stat attachment: %w", statErr)
			}
			return &AttachmentReader{Body: file, ContentType: attachment.ContentType, Size: stat.Size()}, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("open attachment: %w", err)
		}

		timeout := ms.attachmentOpenTimeout
		if timeout <= 0 {
			timeout = defaultAttachmentOpenTimeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		object, err := ms.attachmentStore.Open(ctx, id, attachment.GeneratedFileName)
		if err != nil {
			cancel()
			return nil, err
		}
		if object == nil || object.Body == nil {
			cancel()
			return nil, fmt.Errorf("open remote attachment: response body is empty")
		}
		return &AttachmentReader{
			Body:        &cancelOnCloseReadCloser{ReadCloser: object.Body, cancel: cancel},
			ContentType: attachment.ContentType,
			Size:        object.Size,
		}, nil
	}
	file, err := os.Open(attachmentPath)
	if err != nil {
		return nil, fmt.Errorf("open attachment: %w", err)
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat attachment: %w", err)
	}
	return &AttachmentReader{
		Body:        file,
		ContentType: attachment.ContentType,
		Size:        stat.Size(),
	}, nil
}
