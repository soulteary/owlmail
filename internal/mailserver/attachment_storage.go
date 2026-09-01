package mailserver

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (ms *MailServer) uploadAttachments(id, stagingDirectory string, attachments []*Attachment) error {
	for _, attachment := range attachments {
		if attachment == nil || attachment.GeneratedFileName == "" {
			cleanupErr := ms.attachmentStore.DeleteEmail(context.Background(), id)
			return errors.Join(fmt.Errorf("attachment metadata is incomplete"), cleanupErr)
		}
		attachmentPath := filepath.Join(stagingDirectory, attachment.GeneratedFileName)
		if err := validatePath(stagingDirectory, attachmentPath); err != nil {
			cleanupErr := ms.attachmentStore.DeleteEmail(context.Background(), id)
			return errors.Join(fmt.Errorf("validate staged attachment path: %w", err), cleanupErr)
		}
		file, err := os.Open(attachmentPath)
		if err != nil {
			cleanupErr := ms.attachmentStore.DeleteEmail(context.Background(), id)
			return errors.Join(fmt.Errorf("open staged attachment: %w", err), cleanupErr)
		}
		stat, statErr := file.Stat()
		if statErr == nil {
			err = ms.attachmentStore.Put(
				context.Background(),
				id,
				attachment.GeneratedFileName,
				attachment.ContentType,
				file,
				stat.Size(),
			)
		}
		closeErr := file.Close()
		if statErr != nil || err != nil || closeErr != nil {
			cleanupErr := ms.attachmentStore.DeleteEmail(context.Background(), id)
			return errors.Join(statErr, err, closeErr, cleanupErr)
		}
	}
	return nil
}

func (ms *MailServer) deleteStoredAttachments(id, localPath string) error {
	if ms.attachmentStore != nil {
		return errors.Join(
			ms.attachmentStore.DeleteEmail(context.Background(), id),
			os.RemoveAll(localPath),
		)
	}
	return os.RemoveAll(localPath)
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
		name string
		size int64
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
		files = append(files, candidate{name: entry.Name(), size: info.Size()})
	}
	if len(files) != len(attachments) {
		return fmt.Errorf("legacy attachment file count does not match message")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	used := make([]bool, len(files))
	for _, attachment := range attachments {
		if attachment == nil {
			return fmt.Errorf("legacy attachment metadata is incomplete")
		}
		expectedExtension := attachmentExtension(attachment)
		match := -1
		for i, file := range files {
			if used[i] || file.size != attachment.Size {
				continue
			}
			if expectedExtension != "" && !strings.EqualFold(filepath.Ext(file.name), expectedExtension) {
				continue
			}
			match = i
			break
		}
		if match == -1 {
			return fmt.Errorf("legacy attachment file cannot be matched")
		}
		used[match] = true
		attachment.GeneratedFileName = files[match].name
		attachment.Transformed = true
	}
	return nil
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

		object, err := ms.attachmentStore.Open(context.Background(), id, attachment.GeneratedFileName)
		if err != nil {
			return nil, err
		}
		return &AttachmentReader{
			Body:        object.Body,
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
