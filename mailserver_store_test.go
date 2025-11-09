package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMailServerGetEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

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

	if err := server.saveEmailToStore("test-id", false, envelope, email); err != nil {
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
	defer server.Close()

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
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

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
	defer server.Close()

	// Create email
	email := &Email{ID: "test-id", Subject: "Test"}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

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
	defer server.Close()

	// Add emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

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

func TestMailServerReadEmail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	// Create unread email
	email := &Email{ID: "test-id", Subject: "Test", Read: false}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

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
	defer server.Close()

	// Add unread emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Read: false, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

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
	defer server.Close()

	// Add emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Read: true, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", true, envelope, email2)

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
	defer server.Close()

	// Test with non-existent email
	_, err = server.GetRawEmail("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent email")
	}

	// Create email file
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email content")
	os.WriteFile(emlPath, content, 0644)

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
	defer server.Close()

	// Create email file
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email content")
	os.WriteFile(emlPath, content, 0644)

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
	defer server.Close()

	// Create email with HTML
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		HTML:    "<html><body>Test</body></html>",
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

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
	defer server.Close()

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
	defer server.Close()

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
	defer server.Close()

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
	defer server.Close()

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
	os.WriteFile(emlPath, []byte("content"), 0644)

	// Create attachment directory and file
	attachmentDir := filepath.Join(tmpDir, "test-id")
	os.MkdirAll(attachmentDir, 0755)
	attachmentPath := filepath.Join(attachmentDir, "test.pdf")
	os.WriteFile(attachmentPath, []byte("attachment content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

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
	os.WriteFile(emlPath2, []byte("content"), 0644)
	server.saveEmailToStore("test-id-2", false, envelope, email2)

	_, _, err = server.GetEmailAttachment("test-id-2", "file.pdf")
	if err == nil {
		t.Error("Expected error for email without attachments")
	}
}
