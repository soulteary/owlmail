package mailserver

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
)

func TestMailServerGetEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test with non-existent email
	_, err = server.GetEmail("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent email")
	}

	// Create and save an email
	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		Time:    time.Now(),
		Read:    false,
	}

	envelope := &Envelope{
		From: "from@example.com",
		To:   []string{"to@example.com"},
	}

	// Save email to store
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("test email content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Retrieve email
	retrieved, err := server.GetEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to get email: %v", err)
	}

	if retrieved.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", retrieved.ID)
	}
	if retrieved.Subject != "Test Subject" {
		t.Errorf("Expected subject 'Test Subject', got '%s'", retrieved.Subject)
	}
}

func TestMailServerGetAllEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Initially should be empty
	emails := server.GetAllEmail()
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails, got %d", len(emails))
	}

	// Add emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}

	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file 1: %v", err)
	}
	if err := os.WriteFile(emlPath2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create email file 2: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}
	if err := server.SaveEmailToStore("id2", false, envelope, email2); err != nil {
		t.Fatalf("Failed to save email 2: %v", err)
	}

	emails = server.GetAllEmail()
	if len(emails) != 2 {
		t.Errorf("Expected 2 emails, got %d", len(emails))
	}
}

func TestMailServerDeleteEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email
	email := &Email{ID: "test-id", Subject: "Test"}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Delete email
	err = server.DeleteEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to delete email: %v", err)
	}

	// Verify deleted
	_, err = server.GetEmail("test-id")
	if err == nil {
		t.Error("Email should be deleted")
	}
}

func TestMailServerDeleteAllEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file 1: %v", err)
	}
	if err := os.WriteFile(emlPath2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create email file 2: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}
	if err := server.SaveEmailToStore("id2", false, envelope, email2); err != nil {
		t.Fatalf("Failed to save email 2: %v", err)
	}

	// Delete all
	err = server.DeleteAllEmail()
	if err != nil {
		t.Fatalf("Failed to delete all emails: %v", err)
	}

	// Verify all deleted
	emails := server.GetAllEmail()
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails, got %d", len(emails))
	}
}

func TestMailServerDeleteAllEmailWithEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Delete all when directory is empty (tests the error path in ReadDir)
	err = server.DeleteAllEmail()
	if err != nil {
		t.Fatalf("DeleteAllEmail should succeed even with empty directory: %v", err)
	}

	// Verify still empty
	emails := server.GetAllEmail()
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails, got %d", len(emails))
	}
}

func TestMailServerReadEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create unread email
	email := &Email{ID: "test-id", Subject: "Test", Read: false}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Mark as read
	err = server.ReadEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to read email: %v", err)
	}

	// Verify read
	retrieved, err := server.GetEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to get email: %v", err)
	}
	if !retrieved.Read {
		t.Error("Email should be marked as read")
	}
}

func TestMailServerReadAllEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add unread emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Read: false, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file 1: %v", err)
	}
	if err := os.WriteFile(emlPath2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create email file 2: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}
	if err := server.SaveEmailToStore("id2", false, envelope, email2); err != nil {
		t.Fatalf("Failed to save email 2: %v", err)
	}

	// Mark all as read
	count := server.ReadAllEmail()
	if count != 2 {
		t.Errorf("Expected 2 emails marked as read, got %d", count)
	}

	// Verify all read
	emails := server.GetAllEmail()
	for _, email := range emails {
		if !email.Read {
			t.Error("All emails should be marked as read")
		}
	}
}

func TestMailServerGetEmailStats(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Read: true, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file 1: %v", err)
	}
	if err := os.WriteFile(emlPath2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create email file 2: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}
	if err := server.SaveEmailToStore("id2", true, envelope, email2); err != nil {
		t.Fatalf("Failed to save email 2: %v", err)
	}

	// Get stats
	stats := server.GetEmailStats()
	if stats["total"] != 2 {
		t.Errorf("Expected total 2, got %v", stats["total"])
	}
	if stats["unread"] != 1 {
		t.Errorf("Expected unread 1, got %v", stats["unread"])
	}
	if stats["read"] != 1 {
		t.Errorf("Expected read 1, got %v", stats["read"])
	}
}

func TestMailServerGetRawEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test with non-existent email
	_, err = server.GetRawEmail("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent email")
	}

	// Create email file
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email content")
	if err := os.WriteFile(emlPath, content, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Get raw email
	path, err := server.GetRawEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to get raw email: %v", err)
	}
	if path != emlPath {
		t.Errorf("Expected path %s, got %s", emlPath, path)
	}
}

func TestMailServerGetRawEmailContent(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email file
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email content")
	if err := os.WriteFile(emlPath, content, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Get raw email content
	retrieved, err := server.GetRawEmailContent("test-id")
	if err != nil {
		t.Fatalf("Failed to get raw email content: %v", err)
	}
	if string(retrieved) != string(content) {
		t.Errorf("Expected content %s, got %s", string(content), string(retrieved))
	}
}

func TestMailServerGetEmailHTML(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email with HTML
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		HTML:    "<html><body>Test</body></html>",
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Get HTML (will be sanitized)
	html, err := server.GetEmailHTML("test-id")
	if err != nil {
		t.Fatalf("Failed to get HTML: %v", err)
	}
	if html == "" {
		t.Error("HTML should not be empty")
	}
	// HTML is sanitized, so we just check it's not empty
}

func TestSaveAttachment(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test saveAttachment
	attachment := &Attachment{
		FileName:    "test.pdf",
		ContentType: "application/pdf",
	}
	data := []byte("test attachment data")

	err = server.saveAttachment("test-id", attachment, data)
	if err != nil {
		t.Fatalf("Failed to save attachment: %v", err)
	}

	// Verify attachment was saved
	attachmentDir := filepath.Join(tmpDir, "test-id")
	attachmentPath := filepath.Join(attachmentDir, attachment.GeneratedFileName)

	if _, err := os.Stat(attachmentPath); err != nil {
		t.Errorf("Attachment file should exist: %v", err)
	}

	// Verify attachment size
	if attachment.Size != int64(len(data)) {
		t.Errorf("Expected attachment size %d, got %d", len(data), attachment.Size)
	}

	// Test with ContentID
	attachment2 := &Attachment{
		FileName:    "test2.pdf",
		ContentType: "application/pdf",
		ContentID:   "test-content-id",
	}
	data2 := []byte("test attachment data 2")

	err = server.saveAttachment("test-id-2", attachment2, data2)
	if err != nil {
		t.Fatalf("Failed to save attachment with ContentID: %v", err)
	}

	// Verify attachment was saved
	attachmentDir2 := filepath.Join(tmpDir, "test-id-2")
	attachmentPath2 := filepath.Join(attachmentDir2, attachment2.GeneratedFileName)

	if _, err := os.Stat(attachmentPath2); err != nil {
		t.Errorf("Attachment file with ContentID should exist: %v", err)
	}
}

func TestLoadMailsFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create a simple email file
	emailContent := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Test Email\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"\r\n" +
		"Test body")

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Load emails from directory
	err = server.LoadMailsFromDirectory()
	if err != nil {
		t.Fatalf("Failed to load emails from directory: %v", err)
	}

	// Verify email was loaded
	emails := server.GetAllEmail()
	if len(emails) == 0 {
		t.Error("Email should be loaded from directory")
	}

	// Verify email content
	if len(emails) > 0 {
		email := emails[0]
		if email.Subject != "Test Email" {
			t.Errorf("Expected subject 'Test Email', got '%s'", email.Subject)
		}
	}

	// Test with invalid email file
	invalidPath := filepath.Join(tmpDir, "invalid.eml")
	if err := os.WriteFile(invalidPath, []byte("invalid email content"), 0644); err != nil {
		t.Fatalf("Failed to create invalid email file: %v", err)
	}

	// Load should continue even with invalid files
	err = server.LoadMailsFromDirectory()
	if err != nil {
		t.Fatalf("LoadMailsFromDirectory should handle invalid files gracefully: %v", err)
	}

	// Test with non-.eml file (should be skipped)
	nonEmlPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(nonEmlPath, []byte("not an email"), 0644); err != nil {
		t.Fatalf("Failed to create non-email file: %v", err)
	}

	// Load should skip non-.eml files
	err = server.LoadMailsFromDirectory()
	if err != nil {
		t.Fatalf("LoadMailsFromDirectory should skip non-.eml files: %v", err)
	}

	// Test with already loaded email (should be skipped)
	err = server.LoadMailsFromDirectory()
	if err != nil {
		t.Fatalf("LoadMailsFromDirectory should handle already loaded emails: %v", err)
	}
}

func TestLoadMailsFromDirectoryWithMultipart(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create a multipart email file
	emailContent := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Multipart Test\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"Content-Type: multipart/alternative; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Plain text body\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<html><body>HTML body</body></html>\r\n" +
		"--boundary123--\r\n")

	emlPath := filepath.Join(tmpDir, "multipart-id.eml")
	if err := os.WriteFile(emlPath, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create multipart email file: %v", err)
	}

	// Load emails from directory
	err = server.LoadMailsFromDirectory()
	if err != nil {
		t.Fatalf("Failed to load multipart email from directory: %v", err)
	}

	// Verify email was loaded
	emails := server.GetAllEmail()
	if len(emails) == 0 {
		t.Error("Multipart email should be loaded from directory")
	}
}

func TestGetEmailAttachment(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test with non-existent email
	_, _, err = server.GetEmailAttachment("nonexistent", "file.pdf")
	if err == nil {
		t.Error("Expected error for non-existent email")
	}

	// Create email with attachment
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Attachments: []*Attachment{
			{
				GeneratedFileName: "test.pdf",
				ContentType:       "application/pdf",
			},
		},
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Create attachment directory and file
	attachmentDir := filepath.Join(tmpDir, "test-id")
	if err := os.MkdirAll(attachmentDir, 0755); err != nil {
		t.Fatalf("Failed to create attachment directory: %v", err)
	}
	attachmentPath := filepath.Join(attachmentDir, "test.pdf")
	if err := os.WriteFile(attachmentPath, []byte("attachment content"), 0644); err != nil {
		t.Fatalf("Failed to create attachment file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Get attachment
	path, contentType, err := server.GetEmailAttachment("test-id", "test.pdf")
	if err != nil {
		t.Fatalf("Failed to get attachment: %v", err)
	}
	if path != attachmentPath {
		t.Errorf("Expected path %s, got %s", attachmentPath, path)
	}
	if contentType != "application/pdf" {
		t.Errorf("Expected content type 'application/pdf', got '%s'", contentType)
	}

	// Test with non-existent attachment
	_, _, err = server.GetEmailAttachment("test-id", "nonexistent.pdf")
	if err == nil {
		t.Error("Expected error for non-existent attachment")
	}

	// Test with email without attachments
	email2 := &Email{
		ID:      "test-id-2",
		Subject: "Test 2",
	}
	emlPath2 := filepath.Join(tmpDir, "test-id-2.eml")
	if err := os.WriteFile(emlPath2, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file 2: %v", err)
	}
	if err := server.SaveEmailToStore("test-id-2", false, envelope, email2); err != nil {
		t.Fatalf("Failed to save email 2: %v", err)
	}

	_, _, err = server.GetEmailAttachment("test-id-2", "file.pdf")
	if err == nil {
		t.Error("Expected error for email without attachments")
	}
}

// TestSaveEmailToStoreWithZeroTime tests SaveEmailToStore when Time is zero
func TestSaveEmailToStoreWithZeroTime(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email with zero time
	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		Time:    time.Time{}, // Zero time
		Read:    false,
	}

	envelope := &Envelope{
		From: "from@example.com",
		To:   []string{"to@example.com"},
	}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("test email content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Verify time was set
	retrieved, err := server.GetEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to get email: %v", err)
	}
	if retrieved.Time.IsZero() {
		t.Error("Time should be set when it was zero")
	}
}

// TestSaveEmailToStoreWithHTML tests SaveEmailToStore with HTML content
func TestSaveEmailToStoreWithHTML(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email with HTML
	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		HTML:    "  <html><body>Test</body></html>  ", // With whitespace
		Time:    time.Now(),
	}

	envelope := &Envelope{
		From: "from@example.com",
		To:   []string{"to@example.com"},
	}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("test email content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Verify HTML was sanitized and trimmed
	retrieved, err := server.GetEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to get email: %v", err)
	}
	if retrieved.HTML == "" {
		t.Error("HTML should not be empty")
	}
}

// TestGetEmailWithoutHTML tests GetEmail when HTML is empty
func TestGetEmailWithoutHTML(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email without HTML
	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		HTML:    "",
		Time:    time.Now(),
	}

	envelope := &Envelope{
		From: "from@example.com",
		To:   []string{"to@example.com"},
	}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("test email content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Get email
	retrieved, err := server.GetEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to get email: %v", err)
	}
	if retrieved.HTML != "" {
		t.Error("HTML should be empty")
	}
}

// TestDeleteEmailWithInvalidID tests DeleteEmail with invalid email ID
func TestDeleteEmailWithInvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test with invalid ID containing path traversal
	err = server.DeleteEmail("../invalid")
	if err == nil {
		t.Error("Expected error for invalid email ID")
	}

	// Test with empty ID
	err = server.DeleteEmail("")
	if err == nil {
		t.Error("Expected error for empty email ID")
	}
}

// TestDeleteEmailNotFound tests DeleteEmail when email is not found
func TestDeleteEmailNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Try to delete non-existent email
	err = server.DeleteEmail("nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent email")
	}
}

// TestGetRawEmailWithInvalidID tests GetRawEmail with invalid email ID
func TestGetRawEmailWithInvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test with invalid ID containing path traversal
	_, err = server.GetRawEmail("../invalid")
	if err == nil {
		t.Error("Expected error for invalid email ID")
	}

	// Test with empty ID
	_, err = server.GetRawEmail("")
	if err == nil {
		t.Error("Expected error for empty email ID")
	}
}

// TestGetRawEmailContentWithInvalidID tests GetRawEmailContent with invalid email ID
func TestGetRawEmailContentWithInvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test with invalid ID containing path traversal
	_, err = server.GetRawEmailContent("../invalid")
	if err == nil {
		t.Error("Expected error for invalid email ID")
	}

	// Test with empty ID
	_, err = server.GetRawEmailContent("")
	if err == nil {
		t.Error("Expected error for empty email ID")
	}
}

// TestGetEmailAttachmentWithInvalidFilename tests GetEmailAttachment with invalid filename
func TestGetEmailAttachmentWithInvalidFilename(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email with attachment
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Attachments: []*Attachment{
			{
				GeneratedFileName: "test.pdf",
				ContentType:       "application/pdf",
			},
		},
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Test with empty filename
	_, _, err = server.GetEmailAttachment("test-id", "")
	if err == nil {
		t.Error("Expected error for empty filename")
	}

	// Test with path traversal in filename
	_, _, err = server.GetEmailAttachment("test-id", "../file.pdf")
	if err == nil {
		t.Error("Expected error for filename with path traversal")
	}

	// Test with slash in filename
	_, _, err = server.GetEmailAttachment("test-id", "path/file.pdf")
	if err == nil {
		t.Error("Expected error for filename with slash")
	}

	// Test with backslash in filename
	_, _, err = server.GetEmailAttachment("test-id", "path\\file.pdf")
	if err == nil {
		t.Error("Expected error for filename with backslash")
	}

	// Test with invalid ID
	_, _, err = server.GetEmailAttachment("../invalid", "file.pdf")
	if err == nil {
		t.Error("Expected error for invalid email ID")
	}
}

// TestReadEmailNotFound tests ReadEmail when email is not found
func TestReadEmailNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Try to mark non-existent email as read
	err = server.ReadEmail("nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent email")
	}
}

// TestReadAllEmailWithMixedReadStatus tests ReadAllEmail with mixed read/unread emails
func TestReadAllEmailWithMixedReadStatus(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add mixed read/unread emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Read: true, Time: time.Now()}
	email3 := &Email{ID: "id3", Subject: "Subject 3", Read: false, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	emlPath3 := filepath.Join(tmpDir, "id3.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file 1: %v", err)
	}
	if err := os.WriteFile(emlPath2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create email file 2: %v", err)
	}
	if err := os.WriteFile(emlPath3, []byte("content3"), 0644); err != nil {
		t.Fatalf("Failed to create email file 3: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}
	if err := server.SaveEmailToStore("id2", true, envelope, email2); err != nil {
		t.Fatalf("Failed to save email 2: %v", err)
	}
	if err := server.SaveEmailToStore("id3", false, envelope, email3); err != nil {
		t.Fatalf("Failed to save email 3: %v", err)
	}

	// Mark all as read
	count := server.ReadAllEmail()
	if count != 2 {
		t.Errorf("Expected 2 emails marked as read, got %d", count)
	}

	// Verify all read
	emails := server.GetAllEmail()
	for _, email := range emails {
		if !email.Read {
			t.Error("All emails should be marked as read")
		}
	}
}

// TestSaveEmailToStoreWithFileSize tests SaveEmailToStore with existing file
func TestSaveEmailToStoreWithFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email file first
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email content with some length")
	if err := os.WriteFile(emlPath, content, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		Time:    time.Now(),
	}

	envelope := &Envelope{
		From: "from@example.com",
		To:   []string{"to@example.com"},
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Verify file size was set
	retrieved, err := server.GetEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to get email: %v", err)
	}
	if retrieved.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), retrieved.Size)
	}
	if retrieved.SizeHuman == "" {
		t.Error("SizeHuman should not be empty")
	}
}

// TestGetEmailStatsWithEmptyStore tests GetEmailStats with empty store
func TestGetEmailStatsWithEmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Get stats from empty store
	stats := server.GetEmailStats()
	if stats["total"] != 0 {
		t.Errorf("Expected total 0, got %v", stats["total"])
	}
	if stats["unread"] != 0 {
		t.Errorf("Expected unread 0, got %v", stats["unread"])
	}
	if stats["read"] != 0 {
		t.Errorf("Expected read 0, got %v", stats["read"])
	}
}

// TestGetEmailHTMLNotFound tests GetEmailHTML when email is not found
func TestGetEmailHTMLNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Try to get HTML from non-existent email
	_, err = server.GetEmailHTML("nonexistent-id")
	if err == nil {
		t.Error("Expected error for non-existent email")
	}
}

// TestSaveEmailToStoreWithBCC tests SaveEmailToStore with BCC calculation
func TestSaveEmailToStoreWithBCC(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email with To, CC, and envelope To
	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		Time:    time.Now(),
		To: []*mail.Address{
			{Address: "to1@example.com"},
			{Address: "to2@example.com"},
		},
		CC: []*mail.Address{
			{Address: "cc1@example.com"},
		},
	}

	envelope := &Envelope{
		From: "from@example.com",
		To:   []string{"to1@example.com", "to2@example.com", "cc1@example.com", "bcc1@example.com"},
	}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("test email content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Verify BCC was calculated
	retrieved, err := server.GetEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to get email: %v", err)
	}
	if len(retrieved.CalculatedBCC) == 0 {
		t.Error("BCC should be calculated")
	}
}

// TestParseEmailWithSimpleText tests parseEmail with simple text email
func TestParseEmailWithSimpleText(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create simple text email
	emailContent := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Simple Text Email\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"This is a simple text email body")

	emlPath := filepath.Join(tmpDir, "simple-text.eml")
	if err := os.WriteFile(emlPath, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Parse email
	emailFile, err := os.Open(emlPath)
	if err != nil {
		t.Fatalf("Failed to open email file: %v", err)
	}
	defer emailFile.Close()

	email, err := server.parseEmail("simple-text", emailFile, nil, false, false)
	if err != nil {
		t.Fatalf("Failed to parse email: %v", err)
	}

	if email.Subject != "Simple Text Email" {
		t.Errorf("Expected subject 'Simple Text Email', got '%s'", email.Subject)
	}
	if email.Text == "" {
		t.Error("Text body should not be empty")
	}
}

// TestParseEmailWithHTMLOnly tests parseEmail with HTML-only email
func TestParseEmailWithHTMLOnly(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create HTML-only email
	emailContent := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: HTML Email\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<html><body>This is HTML content</body></html>")

	emlPath := filepath.Join(tmpDir, "html-only.eml")
	if err := os.WriteFile(emlPath, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Parse email
	emailFile, err := os.Open(emlPath)
	if err != nil {
		t.Fatalf("Failed to open email file: %v", err)
	}
	defer emailFile.Close()

	email, err := server.parseEmail("html-only", emailFile, nil, false, false)
	if err != nil {
		t.Fatalf("Failed to parse email: %v", err)
	}

	if email.Subject != "HTML Email" {
		t.Errorf("Expected subject 'HTML Email', got '%s'", email.Subject)
	}
	if email.HTML == "" {
		t.Error("HTML body should not be empty")
	}
}

// TestParseEmailWithAttachment tests parseEmail with attachment
func TestParseEmailWithAttachment(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email with attachment
	emailContent := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Email with Attachment\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Email body\r\n" +
		"--boundary123\r\n" +
		"Content-Type: application/pdf\r\n" +
		"Content-Disposition: attachment; filename=\"test.pdf\"\r\n" +
		"\r\n" +
		"PDF content here\r\n" +
		"--boundary123--\r\n")

	emlPath := filepath.Join(tmpDir, "with-attachment.eml")
	if err := os.WriteFile(emlPath, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Parse email with saveAttachments=true
	emailFile, err := os.Open(emlPath)
	if err != nil {
		t.Fatalf("Failed to open email file: %v", err)
	}
	defer emailFile.Close()

	email, err := server.parseEmail("with-attachment", emailFile, nil, true, false)
	if err != nil {
		t.Fatalf("Failed to parse email: %v", err)
	}

	if len(email.Attachments) == 0 {
		t.Error("Email should have attachments")
	}
}

// TestParseEmailWithContentID tests parseEmail with Content-ID
func TestParseEmailWithContentID(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email with Content-ID
	emailContent := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Email with Content-ID\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"Content-Type: multipart/related; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<html><body><img src=\"cid:image123\"></body></html>\r\n" +
		"--boundary123\r\n" +
		"Content-Type: image/png\r\n" +
		"Content-ID: <image123>\r\n" +
		"\r\n" +
		"PNG content here\r\n" +
		"--boundary123--\r\n")

	emlPath := filepath.Join(tmpDir, "with-contentid.eml")
	if err := os.WriteFile(emlPath, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Parse email
	emailFile, err := os.Open(emlPath)
	if err != nil {
		t.Fatalf("Failed to open email file: %v", err)
	}
	defer emailFile.Close()

	email, err := server.parseEmail("with-contentid", emailFile, nil, true, false)
	if err != nil {
		t.Fatalf("Failed to parse email: %v", err)
	}

	if len(email.Attachments) == 0 {
		t.Error("Email should have attachments with Content-ID")
	}
}

// TestParseEmailWithInvalidContent tests parseEmail with invalid content
func TestParseEmailWithInvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create invalid email content
	invalidContent := []byte("This is not a valid email")

	emlPath := filepath.Join(tmpDir, "invalid.eml")
	if err := os.WriteFile(emlPath, invalidContent, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Parse email should fail
	emailFile, err := os.Open(emlPath)
	if err != nil {
		t.Fatalf("Failed to open email file: %v", err)
	}
	defer emailFile.Close()

	_, err = server.parseEmail("invalid", emailFile, nil, false, false)
	if err == nil {
		t.Error("Expected error for invalid email content")
	}
}

// TestDeleteEmailWithAttachmentDirectory tests DeleteEmail with attachment directory
func TestDeleteEmailWithAttachmentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create email
	email := &Email{ID: "test-id", Subject: "Test"}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Create attachment directory
	attachmentDir := filepath.Join(tmpDir, "test-id")
	if err := os.MkdirAll(attachmentDir, 0755); err != nil {
		t.Fatalf("Failed to create attachment directory: %v", err)
	}
	attachmentPath := filepath.Join(attachmentDir, "test.pdf")
	if err := os.WriteFile(attachmentPath, []byte("attachment content"), 0644); err != nil {
		t.Fatalf("Failed to create attachment file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Delete email
	err = server.DeleteEmail("test-id")
	if err != nil {
		t.Fatalf("Failed to delete email: %v", err)
	}

	// Verify attachment directory was deleted
	if _, err := os.Stat(attachmentDir); err == nil {
		t.Error("Attachment directory should be deleted")
	}
}

// TestLoadMailsFromDirectoryWithSubdirectory tests LoadMailsFromDirectory with subdirectory
func TestLoadMailsFromDirectoryWithSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Create a subdirectory (should be skipped)
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create email file
	emailContent := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Test Email\r\n" +
		"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
		"\r\n" +
		"Test body")

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, emailContent, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	// Load emails from directory
	err = server.LoadMailsFromDirectory()
	if err != nil {
		t.Fatalf("Failed to load emails from directory: %v", err)
	}

	// Verify email was loaded (subdirectory should be skipped)
	emails := server.GetAllEmail()
	if len(emails) == 0 {
		t.Error("Email should be loaded from directory")
	}
}
