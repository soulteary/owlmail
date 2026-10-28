package mailserver

import (
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"
)

// Limits apply across the entire MIME tree, not separately to each container.
// Streaming leaf attachments still use the transaction's existing writer and
// rollback path; nesting never introduces an in-memory attachment buffer.
const (
	maximumMIMEDepth = 32
	maximumMIMEParts = 10000
)

func (ms *MailServer) parseMultipartBody(entity *message.Entity, email *Email, saveAttachments bool, attachmentDir string, depth int, parts *int) error {
	if depth >= maximumMIMEDepth {
		return fmt.Errorf("MIME nesting exceeds %d levels", maximumMIMEDepth)
	}
	reader := entity.MultipartReader()
	if reader == nil {
		return nil
	}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to read multipart: %w", err)
		}
		*parts += 1
		if *parts > maximumMIMEParts {
			return fmt.Errorf("MIME part count exceeds %d", maximumMIMEParts)
		}
		mediaType, _, _ := part.Header.ContentType()
		if mediaType == "" {
			mediaType = "text/plain"
		}
		disposition, params, _ := part.Header.ContentDisposition()
		if strings.HasPrefix(mediaType, "multipart/") && disposition != "attachment" {
			if err := ms.parseMultipartBody(part, email, saveAttachments, attachmentDir, depth+1, parts); err != nil {
				return err
			}
			continue
		}
		contentID := strings.Trim(part.Header.Get("Content-ID"), "<>")
		switch {
		case mediaType == "text/plain" && disposition != "attachment":
			body, err := io.ReadAll(part.Body)
			if err != nil {
				return fmt.Errorf("failed to read text body: %w", err)
			}
			email.Text = strings.TrimSpace(string(body))
		case mediaType == "text/html" && disposition != "attachment":
			body, err := io.ReadAll(part.Body)
			if err != nil {
				return fmt.Errorf("failed to read HTML body: %w", err)
			}
			email.HTML = strings.TrimSpace(string(body))
		case disposition == "attachment" || contentID != "":
			filename := params["filename"]
			if filename == "" {
				filename = mediaType
			}
			attachment := &Attachment{ContentType: mediaType, ContentDisposition: disposition, FileName: filename, ContentID: contentID}
			if saveAttachments {
				err = ms.saveAttachmentReaderInDirectory(attachmentDir, attachment, part.Body)
			} else {
				err = measureAttachment(attachment, part.Body)
			}
			if err != nil {
				return fmt.Errorf("failed to read attachment: %w", err)
			}
			email.Attachments = append(email.Attachments, attachment)
		default:
			if _, err := io.Copy(io.Discard, part.Body); err != nil {
				return fmt.Errorf("failed to discard MIME part: %w", err)
			}
		}
	}
}
