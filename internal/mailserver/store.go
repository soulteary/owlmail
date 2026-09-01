package mailserver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/soulteary/owlmail/internal/common"
)

const webhookOutboxDirectoryName = ".owlmail-webhook-outbox"

// SaveEmailToStore saves a parsed email to the store (exported for testing)
func (ms *MailServer) SaveEmailToStore(id string, isRead bool, envelope *Envelope, parsedEmail *Email) error {
	ms.storageTransactionMutex.RLock()
	defer ms.storageTransactionMutex.RUnlock()
	return ms.saveEmailToStore(id, isRead, envelope, parsedEmail, true, false)
}

func (ms *MailServer) saveEmailToStore(id string, isRead bool, envelope *Envelope, parsedEmail *Email, persistMetadata, finalizeRollbackFence bool) error {
	emlPath := filepath.Join(ms.mailDir, id+".eml")

	parsedEmail.ID = id
	// Only set time if not already set (from header parsing)
	if parsedEmail.Time.IsZero() {
		parsedEmail.Time = time.Now()
	}
	parsedEmail.Read = isRead
	parsedEmail.Envelope = envelope
	parsedEmail.Source = emlPath

	// Try to get file size, but don't fail if file doesn't exist
	stat, err := os.Stat(emlPath)
	if err != nil {
		// File doesn't exist, set size to 0
		parsedEmail.Size = 0
		parsedEmail.SizeHuman = formatBytes(0)
	} else {
		parsedEmail.Size = stat.Size()
		parsedEmail.SizeHuman = formatBytes(stat.Size())
	}

	// Calculate BCC
	envelopeTo := append([]string{}, envelope.To...)
	parsedEmail.CalculatedBCC = calculateBCC(
		envelopeTo,
		addressListToStrings(parsedEmail.To),
		addressListToStrings(parsedEmail.CC),
	)

	// Sanitize HTML if present
	if parsedEmail.HTML != "" {
		parsedEmail.HTML = strings.TrimSpace(sanitizeHTML(parsedEmail.HTML))
	}
	if ms.beforeStoreCommit != nil {
		if err := ms.beforeStoreCommit(parsedEmail); err != nil {
			return fmt.Errorf("storage commit rejected: %w", err)
		}
	}

	storedEmail := cloneEmail(parsedEmail)
	ms.storeMutex.Lock()
	previousEmail, existed := ms.storeByID[id]
	previousEmail = cloneEmail(previousEmail)
	receivedAt := ms.receivedAtByID[id]
	previousReceivedAt := receivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	if persistMetadata {
		// New mail must have durable initial metadata before it becomes visible.
		if err := ms.persistEmailMetadataAt(storedEmail, receivedAt); err != nil {
			ms.storeMutex.Unlock()
			return fmt.Errorf("persist email metadata: %w", err)
		}
	}
	hasTransactionalHandoff := ms.hasSynchronousListener("new")
	handoffErr := ms.emitSynchronous("new", storedEmail)
	if handoffErr == nil && hasTransactionalHandoff {
		if finalizeRollbackFence {
			handoffErr = ms.acceptRollbackFence(id)
		} else {
			handoffErr = ms.createAcceptedHandoffFence(id)
		}
	} else if handoffErr == nil && finalizeRollbackFence {
		handoffErr = ms.completeLocalRollbackFence(id)
	}
	if handoffErr != nil {
		var rollbackErr error
		if persistMetadata {
			if existed {
				rollbackErr = ms.persistEmailMetadataAt(previousEmail, previousReceivedAt)
			} else {
				rollbackErr = ms.deleteEmailMetadata(id)
				if rollbackErr == nil {
					rollbackErr = syncStorageDirectory(filepath.Join(ms.mailDir, metadataDirectoryName))
				}
			}
		}
		ms.storeMutex.Unlock()
		if hasTransactionalHandoff {
			// Complete local outbox cleanup before exposing the failure to a
			// caller that may immediately retry the same email ID.
			ms.emitNotificationAndWait("new-rollback", storedEmail)
		}
		if rollbackErr != nil {
			return errors.Join(handoffErr, fmt.Errorf("roll back email metadata: %w", rollbackErr))
		}
		return handoffErr
	}
	if !existed {
		ms.storeOrder = append(ms.storeOrder, id)
	}
	ms.receivedAtByID[id] = receivedAt
	ms.storeByID[id] = storedEmail
	ms.storeMutex.Unlock()

	common.Log("Saving email: %s, id: %s", parsedEmail.Subject, id)

	// Notify asynchronous listeners only after the durable handoff and in-memory
	// commit both succeed.
	ms.emitAsynchronous("new", storedEmail)

	// Auto relay if enabled
	if ms.outgoing != nil && ms.outgoing.IsAutoRelayEnabled() {
		if err := ms.RelayMail(cloneEmail(storedEmail), true, func(err error) {
			if err != nil {
				common.Error("Error when auto-relaying email: %v", err)
			}
		}); err != nil {
			common.Error("Error when initiating auto-relay: %v", err)
		}
	}

	return nil
}

// saveAttachment saves an attachment to disk
func (ms *MailServer) saveAttachment(id string, attachment *Attachment, data []byte) error {
	attachmentDir := filepath.Join(ms.mailDir, id)
	return ms.saveAttachmentInDirectory(attachmentDir, attachment, data)
}

// saveAttachmentInDirectory writes an attachment atomically inside dir. The
// caller may pass a staging directory so a complete message can be committed
// by rename only after every attachment is durable.
func (ms *MailServer) saveAttachmentInDirectory(attachmentDir string, attachment *Attachment, data []byte) error {
	if err := os.MkdirAll(attachmentDir, 0755); err != nil {
		return fmt.Errorf("failed to create attachment directory: %w", err)
	}

	// Transform attachment filename
	attachment = transformAttachment(attachment)

	attachmentPath := filepath.Join(attachmentDir, attachment.GeneratedFileName)
	if ms.beforeAttachmentWrite != nil {
		if err := ms.beforeAttachmentWrite(attachmentPath); err != nil {
			return fmt.Errorf("attachment write rejected: %w", err)
		}
	}
	tmp, err := os.CreateTemp(attachmentDir, ".owlmail-attachment-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create attachment temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0644); err != nil {
		return fmt.Errorf("failed to set attachment permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("failed to save attachment: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("failed to sync attachment: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close attachment: %w", err)
	}
	if err := os.Rename(tmpPath, attachmentPath); err != nil {
		return fmt.Errorf("failed to commit attachment: %w", err)
	}
	committed = true
	if err := syncDirectory(attachmentDir); err != nil {
		return fmt.Errorf("failed to sync attachment directory: %w", err)
	}

	attachment.Size = int64(len(data))
	return nil
}

// GetEmail retrieves an email by ID
func (ms *MailServer) GetEmail(id string) (*Email, error) {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	if email, exists := ms.storeByID[id]; exists {
		return cloneEmail(email), nil
	}

	return nil, fmt.Errorf("email was not found")
}

// GetAllEmail returns all emails
func (ms *MailServer) GetAllEmail() []*Email {
	ms.storeMutex.RLock()
	defer ms.storeMutex.RUnlock()

	// Return a copy to prevent external modification
	emails := make([]*Email, 0, len(ms.storeOrder))
	for _, id := range ms.storeOrder {
		if email, exists := ms.storeByID[id]; exists {
			emails = append(emails, cloneEmail(email))
		}
	}
	return emails
}

// DeleteEmail deletes an email by ID
func (ms *MailServer) DeleteEmail(id string) error {
	// Validate email ID to prevent path traversal
	if err := validateEmailID(id); err != nil {
		return fmt.Errorf("invalid email ID: %w", err)
	}
	ms.storeMutex.RLock()
	email, exists := ms.storeByID[id]
	if !exists {
		ms.storeMutex.RUnlock()
		return fmt.Errorf("email not found")
	}
	email = cloneEmail(email)
	ms.storeMutex.RUnlock()
	if ms.beforeEmailDelete != nil {
		if err := ms.beforeEmailDelete(id); err != nil {
			return fmt.Errorf("delete email rejected: %w", err)
		}
	}

	// Delete raw email file
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, emlPath); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}
	var deletionErrors []error
	if err := os.Remove(emlPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		deletionErrors = append(deletionErrors, fmt.Errorf("delete email file: %w", err))
	}

	// Delete attachments directory
	attachmentDir := filepath.Join(ms.mailDir, id)
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, attachmentDir); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}
	if err := ms.deleteStoredAttachments(id, attachmentDir); err != nil {
		deletionErrors = append(deletionErrors, fmt.Errorf("delete attachment directory: %w", err))
	}
	if err := ms.deleteEmailMetadata(id); err != nil {
		deletionErrors = append(deletionErrors, fmt.Errorf("delete email metadata: %w", err))
	}
	if err := errors.Join(deletionErrors...); err != nil {
		return err
	}

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

	common.Log("Deleting email - %s, id: %s", email.Subject, email.ID)

	// Emit delete event
	if err := ms.emit("delete", email); err != nil {
		common.Error("Failed to emit delete event: %v", err)
	}

	return nil
}

// DeleteAllEmail deletes all emails
func (ms *MailServer) DeleteAllEmail() error {
	ms.storageTransactionMutex.Lock()
	defer ms.storageTransactionMutex.Unlock()
	common.Log("Deleting all email")

	if ms.attachmentStore != nil {
		ms.storeMutex.RLock()
		ids := append([]string(nil), ms.storeOrder...)
		ms.storeMutex.RUnlock()
		var deletionErrors []error
		for _, id := range ids {
			if err := ms.deleteStoredAttachments(id, filepath.Join(ms.mailDir, id)); err != nil {
				deletionErrors = append(deletionErrors, fmt.Errorf("delete attachments for %s: %w", id, err))
			}
		}
		if err := errors.Join(deletionErrors...); err != nil {
			return err
		}
	}

	ms.storeMutex.Lock()
	ms.storeByID = make(map[string]*Email)
	ms.storeOrder = make([]string, 0)
	ms.receivedAtByID = make(map[string]time.Time)
	ms.storeMutex.Unlock()

	// Clear mail directory
	files, err := os.ReadDir(ms.mailDir)
	if err == nil {
		for _, file := range files {
			if file.IsDir() && (file.Name() == quarantineDirName || file.Name() == webhookOutboxDirectoryName) {
				continue
			}
			if _, fenced := rollbackFenceID(file.Name()); fenced {
				continue
			}
			if err := os.RemoveAll(filepath.Join(ms.mailDir, file.Name())); err != nil {
				common.Verbose("Failed to remove file: %v", err)
			}
		}
	}

	return nil
}

// GetRawEmail returns the raw email file path
func (ms *MailServer) GetRawEmail(id string) (string, error) {
	// Validate email ID to prevent path traversal
	if err := validateEmailID(id); err != nil {
		return "", fmt.Errorf("invalid email ID: %w", err)
	}
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, emlPath); err != nil {
		return "", fmt.Errorf("path validation failed: %w", err)
	}
	if _, err := os.Stat(emlPath); err != nil {
		return "", fmt.Errorf("email file not found")
	}
	return emlPath, nil
}

// GetRawEmailContent returns the raw email file content
func (ms *MailServer) GetRawEmailContent(id string) ([]byte, error) {
	// Validate email ID to prevent path traversal
	if err := validateEmailID(id); err != nil {
		return nil, fmt.Errorf("invalid email ID: %w", err)
	}
	emlPath := filepath.Join(ms.mailDir, id+".eml")
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, emlPath); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}
	content, err := os.ReadFile(emlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read email file: %w", err)
	}
	return content, nil
}

// GetEmailHTML returns the HTML content of an email
func (ms *MailServer) GetEmailHTML(id string) (string, error) {
	email, err := ms.GetEmail(id)
	if err != nil {
		return "", err
	}
	return email.HTML, nil
}

// GetEmailAttachment returns attachment file path
func (ms *MailServer) GetEmailAttachment(id, filename string) (string, string, error) {
	attachment, err := ms.findAttachment(id, filename)
	if err != nil {
		return "", "", err
	}
	if ms.attachmentStore != nil {
		return "", "", fmt.Errorf("attachment is stored remotely; use OpenEmailAttachment")
	}

	attachmentPath := filepath.Join(ms.mailDir, id, attachment.GeneratedFileName)
	// Validate path is within mail directory
	if err := validatePath(ms.mailDir, attachmentPath); err != nil {
		return "", "", fmt.Errorf("path validation failed: %w", err)
	}
	return attachmentPath, attachment.ContentType, nil
}

// ReadAllEmail marks all emails as read, acknowledging only state that was
// durably persisted.
func (ms *MailServer) ReadAllEmail() (int, error) {
	ms.storeMutex.Lock()
	defer ms.storeMutex.Unlock()

	count := 0
	for _, id := range ms.storeOrder {
		email, exists := ms.storeByID[id]
		if !exists {
			continue
		}
		if !email.Read {
			snapshot := cloneEmail(email)
			snapshot.Read = true
			if err := ms.persistEmailMetadataAt(snapshot, ms.receivedAtByID[id]); err != nil {
				return count, fmt.Errorf("persist read state for %s: %w", id, err)
			}
			email.Read = true
			count++
		}
	}
	return count, nil
}

// ReadEmail marks a single email as read
func (ms *MailServer) ReadEmail(id string) error {
	ms.storeMutex.Lock()
	if email, exists := ms.storeByID[id]; exists {
		previous := email.Read
		email.Read = true
		snapshot := cloneEmail(email)
		receivedAt := ms.receivedAtByID[id]
		if err := ms.persistEmailMetadataAt(snapshot, receivedAt); err != nil {
			email.Read = previous
			ms.storeMutex.Unlock()
			return fmt.Errorf("persist read state: %w", err)
		}
		ms.storeMutex.Unlock()
		return nil
	}
	ms.storeMutex.Unlock()
	return fmt.Errorf("email not found")
}

// GetEmailStats returns email statistics
func (ms *MailServer) GetEmailStats() map[string]interface{} {
	ms.storeMutex.RLock()

	stats := make(map[string]interface{})
	total := len(ms.storeByID)
	unread := 0
	byDate := make(map[string]int)

	for _, email := range ms.storeByID {
		if !email.Read {
			unread++
		}

		// Group by date (YYYY-MM-DD)
		dateKey := email.Time.Format("2006-01-02")
		byDate[dateKey]++
	}

	stats["total"] = total
	stats["unread"] = unread
	stats["read"] = total - unread
	stats["byDate"] = byDate
	ms.storeMutex.RUnlock()
	stats["storage"] = ms.storageStats()

	return stats
}

// parseEmail parses email from given reader
func (ms *MailServer) parseEmail(id string, r io.Reader, s *Session, saveAttachments, markAsRead bool) (*Email, error) {
	email, envelope, err := ms.parseEmailMessage(id, r, s, saveAttachments, filepath.Join(ms.mailDir, id))
	if err != nil {
		return nil, err
	}
	if err := ms.SaveEmailToStore(id, markAsRead, envelope, email); err != nil {
		return nil, fmt.Errorf("failed to store email into memory: %w", err)
	}
	return email, nil
}

// parseEmailMessage parses a message without publishing it to the in-memory
// store. This separation lets SMTP DATA finish all durable filesystem work
// before the new-email event becomes visible.
func (ms *MailServer) parseEmailMessage(id string, r io.Reader, s *Session, saveAttachments bool, attachmentDir string) (*Email, *Envelope, error) {
	msg, err := message.Read(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Parse email content
	email := &Email{
		Attachments: make([]*Attachment, 0),
		Headers:     make(map[string]interface{}),
	}

	// Extract headers
	// Wrap in mail.Header to get decoding support
	headers := mail.Header{Header: msg.Header}

	// Parse all headers into Headers map
	// Common headers to parse
	commonHeaders := []string{
		"From", "To", "Cc", "Bcc", "Subject", "Date", "Message-ID",
		"Reply-To", "In-Reply-To", "References", "Content-Type",
		"Content-Transfer-Encoding", "MIME-Version", "X-Mailer",
		"X-Priority", "Priority", "Importance",
	}
	for _, headerName := range commonHeaders {
		if headerValue := headers.Get(headerName); headerValue != "" {
			if headerValues := headers.Values(headerName); len(headerValues) > 1 {
				email.Headers[headerName] = headerValues
			} else {
				email.Headers[headerName] = headerValue
			}
		}
	}
	// Note: Additional custom headers can be added here if needed
	// For now, we parse the most common headers listed above

	// Parse date from headers
	if email.Time, err = headers.Date(); err != nil {
		email.Time = parseEmailDate(headers.Header)
	}

	if email.Subject, err = headers.Subject(); err != nil {
		// Fallback to raw subject if decoding fails
		email.Subject = headers.Get("Subject")
	}

	// Parse addresses
	// TODO : handle error cases
	email.From, _ = headers.AddressList("From")
	email.To, _ = headers.AddressList("To")
	email.CC, _ = headers.AddressList("Cc")
	email.BCC, _ = headers.AddressList("Bcc")

	// Parse body
	mediaType, _, err := headers.ContentType()
	if err != nil {
		mediaType = "text/plain"
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		mr := msg.MultipartReader()
		if mr != nil {
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					common.Verbose("Error reading multipart: %v", err)
					continue
				}

				partMediaType, _, _ := p.Header.ContentType()
				if partMediaType == "" {
					partMediaType = "text/plain"
				}

				disposition, params, _ := p.Header.ContentDisposition()
				contentID := strings.Trim(p.Header.Get("Content-ID"), "<>")

				body, _ := io.ReadAll(p.Body)

				if partMediaType == "text/plain" && disposition != "attachment" {
					email.Text = strings.TrimSpace(string(body))
				} else if partMediaType == "text/html" && disposition != "attachment" {
					email.HTML = strings.TrimSpace(string(body))
				} else if disposition == "attachment" || contentID != "" {
					// Handle attachment
					filename := params["filename"]
					if filename == "" {
						filename = partMediaType
					}

					attachment := &Attachment{
						ContentType: partMediaType,
						FileName:    filename,
						ContentID:   contentID,
						Size:        int64(len(body)),
					}

					if saveAttachments {
						err = ms.saveAttachmentInDirectory(attachmentDir, attachment, body)
						if err != nil {
							return nil, nil, fmt.Errorf("failed to save attachment: %w", err)
						}
					}
					email.Attachments = append(email.Attachments, attachment)
				}
			}
		}
	} else {
		// Simple message
		body, _ := io.ReadAll(msg.Body)
		if strings.HasPrefix(mediaType, "text/html") {
			email.HTML = strings.TrimSpace(string(body))
		} else {
			email.Text = strings.TrimSpace(string(body))
		}
	}

	// Create envelope
	envelope := &Envelope{
		From:          "",
		To:            addressListToStrings(email.To),
		Host:          "unknown",
		RemoteAddress: "unknown",
	}
	if s != nil {
		if s.conn != nil {
			if conn := s.conn.Conn(); conn != nil {
				envelope.RemoteAddress = conn.RemoteAddr().String()
			}
			envelope.Host = s.conn.Hostname()
		}
		envelope.From = s.from
		envelope.To = s.to
	}

	return email, envelope, nil
}

// LoadMailsFromDirectory loads emails from the mail directory
func (ms *MailServer) LoadMailsFromDirectory() error {
	ms.storageTransactionMutex.Lock()
	defer ms.storageTransactionMutex.Unlock()
	var loadErrors []error
	fencedIDs := make(map[string]struct{})
	if entries, err := os.ReadDir(ms.mailDir); err == nil {
		for _, entry := range entries {
			if id, ok := rollbackFenceID(entry.Name()); ok {
				state, stateErr := readRollbackFenceState(filepath.Join(ms.mailDir, entry.Name()))
				if stateErr != nil || (state != acceptedFenceState && state != localFenceState) {
					fencedIDs[id] = struct{}{}
				}
			}
		}
	}
	if err := ms.recoverStorageArtifacts(); err != nil {
		common.Error("Storage recovery completed with errors: %v", err)
		loadErrors = append(loadErrors, err)
	}
	files, err := os.ReadDir(ms.mailDir)
	if err != nil {
		return fmt.Errorf("failed to read mail directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only process .eml files
		if !strings.HasSuffix(file.Name(), ".eml") {
			continue
		}

		// Extract ID from filename
		id := strings.TrimSuffix(file.Name(), ".eml")
		if _, fenced := fencedIDs[id]; fenced {
			continue
		}
		emlPath := filepath.Join(ms.mailDir, file.Name())

		// Check if email already loaded
		ms.storeMutex.RLock()
		_, alreadyLoaded := ms.storeByID[id]
		ms.storeMutex.RUnlock()

		if alreadyLoaded {
			continue
		}

		// Read and parse email file
		emailFile, err := os.Open(emlPath)
		if err != nil {
			common.Verbose("Error opening email file %s: %v", emlPath, err)
			continue
		}

		markAsRead := false
		metadataLoaded := false
		metadata := emailMetadata{}
		if loadedMetadata, metadataErr := ms.loadEmailMetadata(id); metadataErr == nil {
			metadata = loadedMetadata
			metadataLoaded = true
			markAsRead = loadedMetadata.Read
			ms.storeMutex.Lock()
			ms.receivedAtByID[id] = loadedMetadata.Sequence
			ms.storeMutex.Unlock()
		} else if !os.IsNotExist(metadataErr) {
			common.Error("Ignoring invalid metadata for %s: %v", id, metadataErr)
		} else if stat, statErr := file.Info(); statErr == nil {
			ms.storeMutex.Lock()
			ms.receivedAtByID[id] = stat.ModTime().UTC()
			ms.storeMutex.Unlock()
		}

		// Parse email. Legacy messages without metadata start unread instead of
		// being silently marked read during recovery. Close the file before a
		// possible quarantine rename because Windows cannot rename an open file.
		email, envelope, parseErr := ms.parseEmailMessage(id, emailFile, nil, false, filepath.Join(ms.mailDir, id))
		closeErr := emailFile.Close()
		if parseErr == nil && closeErr == nil {
			persistRestoredMetadata := !metadataLoaded || (metadata.Version < currentMetadataVersion && len(email.Attachments) == 0)
			if metadataLoaded {
				if metadataErr := restoreAttachmentMetadata(email, metadata); metadataErr != nil {
					loadErrors = append(loadErrors, fmt.Errorf("restore attachment metadata for %s: %w", id, metadataErr))
					continue
				}
			}
			if len(email.Attachments) > 0 && (!metadataLoaded || len(metadata.Attachments) == 0) {
				if metadataErr := ms.restoreLegacyLocalAttachmentMetadata(id, email.Attachments); metadataErr != nil {
					common.Verbose("Could not restore legacy attachment names for %s: %v", id, metadataErr)
				} else {
					persistRestoredMetadata = true
				}
			}
			if storeErr := ms.saveEmailToStore(id, markAsRead, envelope, email, false, false); storeErr != nil {
				loadErrors = append(loadErrors, fmt.Errorf("restore email %s: %w", id, storeErr))
				continue
			}
			if persistRestoredMetadata {
				if metadataErr := ms.persistEmailMetadata(email); metadataErr != nil {
					common.Error("Restored email %s without metadata: %v", id, metadataErr)
					loadErrors = append(loadErrors, fmt.Errorf("persist restored metadata for %s: %w", id, metadataErr))
				}
			}
			common.Verbose("Restored email: %s (id: %s)", email.Subject, id)
		} else {
			loadErr := parseErr
			if loadErr == nil {
				loadErr = fmt.Errorf("failed to close email file: %w", closeErr)
			}
			common.Error("Quarantining corrupt email %s: %v", file.Name(), loadErr)
			if quarantineErr := ms.quarantineEmail(id, emlPath, "corrupt"); quarantineErr != nil {
				common.Error("Failed to quarantine corrupt email %s: %v", file.Name(), quarantineErr)
			}
		}
	}
	ms.storeMutex.Lock()
	sort.SliceStable(ms.storeOrder, func(i, j int) bool {
		return ms.receivedAtByID[ms.storeOrder[i]].Before(ms.receivedAtByID[ms.storeOrder[j]])
	})
	ms.storeMutex.Unlock()
	if err := ms.quarantineOrphanAttachmentDirectories(); err != nil {
		loadErrors = append(loadErrors, err)
	}

	return errors.Join(loadErrors...)
}
