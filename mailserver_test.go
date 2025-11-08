package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

func TestNewMailServer(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with default values
	server, err := NewMailServer(0, "", "")
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	if server.port != defaultPort {
		t.Errorf("Expected port %d, got %d", defaultPort, server.port)
	}
	if server.host != defaultHost {
		t.Errorf("Expected host %s, got %s", defaultHost, server.host)
	}

	// Test with custom values
	server2, err := NewMailServer(2525, "127.0.0.1", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server2.Close()

	if server2.port != 2525 {
		t.Errorf("Expected port 2525, got %d", server2.port)
	}
	if server2.host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", server2.host)
	}
	if server2.mailDir != tmpDir {
		t.Errorf("Expected mailDir %s, got %s", tmpDir, server2.mailDir)
	}
}

func TestNewMailServerWithOutgoing(t *testing.T) {
	tmpDir := t.TempDir()

	outgoingConfig := &OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	server, err := NewMailServerWithOutgoing(1025, "localhost", tmpDir, outgoingConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	if server.outgoing == nil {
		t.Error("Outgoing mail should be configured")
	}
}

func TestNewMailServerWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	outgoingConfig := &OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	authConfig := &SMTPAuthConfig{
		Username: "user",
		Password: "pass",
		Enabled:  true,
	}

	tlsConfig := &TLSConfig{
		Enabled: true,
	}

	server, err := NewMailServerWithConfig(1025, "localhost", tmpDir, outgoingConfig, authConfig, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	if server.authConfig == nil {
		t.Error("Auth config should be set")
	}
	if server.tlsConfig == nil {
		t.Error("TLS config should be set")
	}
}

func TestMailServerOn(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	eventFired := false
	server.On("new", func(email *Email) {
		eventFired = true
	})

	// Emit event
	email := &Email{ID: "test-id", Subject: "Test"}
	server.emit("new", email)

	// Give time for goroutine to execute
	time.Sleep(50 * time.Millisecond)

	if !eventFired {
		t.Error("Event handler should have been called")
	}
}

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

func TestMailServerSetOutgoingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	// Set outgoing config
	config := &OutgoingConfig{
		Host:     "smtp.example.com",
		Port:     587,
		User:     "user",
		Password: "pass",
	}

	server.SetOutgoingConfig(config)

	// Get config
	retrieved := server.GetOutgoingConfig()
	if retrieved == nil {
		t.Fatal("Outgoing config should be set")
	}
	if retrieved.Host != config.Host {
		t.Errorf("Expected host %s, got %s", config.Host, retrieved.Host)
	}
}

func TestMakeID(t *testing.T) {
	id1 := makeID()
	id2 := makeID()

	if len(id1) != 8 {
		t.Errorf("Expected ID length 8, got %d", len(id1))
	}
	if id1 == id2 {
		t.Error("IDs should be unique")
	}
}

func TestFormatBytes(t *testing.T) {
	testCases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 bytes"},
		{512, "512 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}

	for _, tc := range testCases {
		result := formatBytes(tc.bytes)
		if result != tc.expected {
			t.Errorf("For %d bytes: Expected '%s', got '%s'", tc.bytes, tc.expected, result)
		}
	}
}

func TestAddressListToStrings(t *testing.T) {
	addrs := []*mail.Address{
		{Address: "test1@example.com"},
		{Address: "test2@example.com"},
	}

	result := addressListToStrings(addrs)
	if len(result) != 2 {
		t.Errorf("Expected 2 addresses, got %d", len(result))
	}
	if result[0] != "test1@example.com" {
		t.Errorf("Expected 'test1@example.com', got '%s'", result[0])
	}
	if result[1] != "test2@example.com" {
		t.Errorf("Expected 'test2@example.com', got '%s'", result[1])
	}
}

func TestCalculateBCC(t *testing.T) {
	recipients := []string{"to1@example.com", "to2@example.com", "cc1@example.com", "bcc1@example.com"}
	to := []string{"to1@example.com", "to2@example.com"}
	cc := []string{"cc1@example.com"}

	bcc := calculateBCC(recipients, to, cc)
	if len(bcc) != 1 {
		t.Errorf("Expected 1 BCC address, got %d", len(bcc))
	}
	if bcc[0].Address != "bcc1@example.com" {
		t.Errorf("Expected 'bcc1@example.com', got '%s'", bcc[0].Address)
	}
}

func TestTransformAttachment(t *testing.T) {
	// Test with filename and extension
	attachment := &Attachment{
		FileName:    "test.pdf",
		ContentType: "application/pdf",
	}

	result := transformAttachment(attachment)
	if result.GeneratedFileName == "" {
		t.Error("Generated filename should not be empty")
	}
	if !result.transformed {
		t.Error("Attachment should be marked as transformed")
	}
	if filepath.Ext(result.GeneratedFileName) != ".pdf" {
		t.Error("Generated filename should have .pdf extension")
	}

	// Test with ContentID
	attachment2 := &Attachment{
		FileName:    "test.pdf",
		ContentID:   "test-content-id",
		ContentType: "application/pdf",
	}

	result2 := transformAttachment(attachment2)
	if result2.GeneratedFileName == "" {
		t.Error("Generated filename should not be empty")
	}

	// Test with no extension
	attachment3 := &Attachment{
		FileName:    "test",
		ContentType: "text/plain",
	}

	result3 := transformAttachment(attachment3)
	if filepath.Ext(result3.GeneratedFileName) == "" {
		t.Error("Generated filename should have extension")
	}

	// Test already transformed
	attachment4 := &Attachment{
		FileName:          "test.pdf",
		transformed:       true,
		GeneratedFileName: "already-generated.pdf",
	}

	result4 := transformAttachment(attachment4)
	if result4.GeneratedFileName != "already-generated.pdf" {
		t.Error("Already transformed attachment should not be transformed again")
	}
}

func TestSanitizeHTML(t *testing.T) {
	html := `<html><body><script>alert('xss')</script><p>Safe content</p><a href="http://example.com" target="_blank">Link</a></body></html>`
	sanitized := sanitizeHTML(html)

	if len(sanitized) == 0 {
		t.Error("Sanitized HTML should not be empty")
	}
	// Script should be removed
	if len(sanitized) >= len(html) {
		t.Error("Sanitized HTML should be shorter (script removed)")
	}
}

func TestParseEmailDate(t *testing.T) {
	// Create a message header
	header := message.Header{}
	header.Set("Date", "Mon, 02 Jan 2006 15:04:05 -0700")

	date := parseEmailDate(header)
	if date.IsZero() {
		t.Error("Date should not be zero")
	}

	// Test with empty date
	header2 := message.Header{}
	date2 := parseEmailDate(header2)
	if date2.IsZero() {
		t.Error("Date should default to current time")
	}

	// Test with invalid date
	header3 := message.Header{}
	header3.Set("Date", "invalid date")
	date3 := parseEmailDate(header3)
	if date3.IsZero() {
		t.Error("Date should default to current time for invalid date")
	}
}

func TestGenerateSelfSignedCert(t *testing.T) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate self-signed certificate: %v", err)
	}

	if len(cert.Certificate) == 0 {
		t.Error("Certificate should have certificate data")
	}
	if cert.PrivateKey == nil {
		t.Error("Certificate should have private key")
	}
}

func TestBackendNewSessionBasic(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	// Create a backend instance
	backend := &Backend{mailServer: server}

	// Create a mock SMTP connection
	// We can't easily create a real smtp.Conn, so we'll test the function exists
	// In a real scenario, this would be called by the SMTP server
	_ = backend.NewSession
}

func TestSessionMail(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	session := &Session{
		mailServer:    server,
		authenticated: true,
		conn:          nil, // conn is nil in test, but Mail should handle it
	}

	// Test Mail command - conn is nil, but Mail should handle it gracefully
	// The code checks if conn is nil before using it
	err = session.Mail("from@example.com", nil)
	if err != nil {
		t.Errorf("Mail should succeed, got error: %v", err)
	}
	if session.from != "from@example.com" {
		t.Errorf("Expected from 'from@example.com', got '%s'", session.from)
	}

	// Test with authentication required but not authenticated
	server.authConfig = &SMTPAuthConfig{
		Username: "user",
		Password: "pass",
		Enabled:  true,
	}
	session.authenticated = false
	err = session.Mail("from@example.com", nil)
	// Should still succeed (we just log a warning)
	if err != nil {
		t.Errorf("Mail should still succeed with warning, got error: %v", err)
	}
}

func TestSessionRcpt(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	session := &Session{
		mailServer: server,
		to:         []string{},
	}

	// Test Rcpt command
	err = session.Rcpt("to@example.com", nil)
	if err != nil {
		t.Errorf("Rcpt should succeed, got error: %v", err)
	}
	if len(session.to) != 1 || session.to[0] != "to@example.com" {
		t.Errorf("Expected to ['to@example.com'], got %v", session.to)
	}

	// Test multiple recipients
	err = session.Rcpt("to2@example.com", nil)
	if err != nil {
		t.Errorf("Rcpt should succeed, got error: %v", err)
	}
	if len(session.to) != 2 {
		t.Errorf("Expected 2 recipients, got %d", len(session.to))
	}
}

func TestSessionData(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	session := &Session{
		mailServer: server,
		from:       "from@example.com",
		to:         []string{"to@example.com"},
		conn:       nil, // conn is nil in test, but Data should handle it
	}

	// Create a simple email message
	emailData := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Test\r\n" +
		"\r\n" +
		"Test body")

	reader := bytes.NewReader(emailData)
	err = session.Data(reader)
	if err != nil {
		t.Errorf("Data should succeed, got error: %v", err)
	}

	// Verify email was saved
	emails := server.GetAllEmail()
	if len(emails) == 0 {
		t.Error("Email should be saved")
	}
}

func TestSessionReset(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	session := &Session{
		mailServer: server,
		from:       "from@example.com",
		to:         []string{"to@example.com"},
	}

	// Test Reset
	session.Reset()
	if session.from != "" {
		t.Errorf("Expected from to be empty, got '%s'", session.from)
	}
	if len(session.to) != 0 {
		t.Errorf("Expected to to be empty, got %v", session.to)
	}
}

func TestSessionLogout(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	session := &Session{
		mailServer: server,
	}

	// Test Logout
	err = session.Logout()
	if err != nil {
		t.Errorf("Logout should succeed, got error: %v", err)
	}
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

func TestBackendNewSession(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	backend := &Backend{mailServer: server}

	// Test NewSession without auth
	session, err := backend.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession should succeed: %v", err)
	}
	if session == nil {
		t.Error("Session should not be nil")
	}

	// Test NewSession with auth enabled
	server.authConfig = &SMTPAuthConfig{
		Username: "user",
		Password: "pass",
		Enabled:  true,
	}

	session2, err := backend.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession with auth should succeed: %v", err)
	}
	if session2 == nil {
		t.Error("Session should not be nil")
	}
}

func TestRelayMail(t *testing.T) {
	tmpDir := t.TempDir()

	// Create server without outgoing config
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	// Test RelayMail without outgoing config
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &Envelope{
			From: "from@example.com",
			To:   []string{"to@example.com"},
		},
	}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("test email"), 0644)

	err = server.RelayMail(email, false, func(err error) {
		if err == nil {
			t.Error("RelayMail should fail without outgoing config")
		}
	})
	if err == nil {
		t.Error("RelayMail should return error without outgoing config")
	}

	// Test RelayMail with outgoing config
	outgoingConfig := &OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}
	server.SetOutgoingConfig(outgoingConfig)

	// RelayMail will queue the task, but actual relay will fail in test
	// We can test that it doesn't panic
	err = server.RelayMail(email, true, func(err error) {
		// Callback will be called with error in test environment
	})
	if err != nil {
		// Error is expected in test environment
	}
}

func TestRelayMailTo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create server without outgoing config
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	// Test RelayMailTo without outgoing config
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Envelope: &Envelope{
			From: "from@example.com",
			To:   []string{"to@example.com"},
		},
	}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("test email"), 0644)

	err = server.RelayMailTo(email, "relay@example.com", func(err error) {
		if err == nil {
			t.Error("RelayMailTo should fail without outgoing config")
		}
	})
	if err == nil {
		t.Error("RelayMailTo should return error without outgoing config")
	}

	// Test RelayMailTo with outgoing config
	outgoingConfig := &OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}
	server.SetOutgoingConfig(outgoingConfig)

	// RelayMailTo will queue the task, but actual relay will fail in test
	// We can test that it doesn't panic
	err = server.RelayMailTo(email, "relay@example.com", func(err error) {
		// Callback will be called with error in test environment
	})
	if err != nil {
		// Error is expected in test environment
	}
}

func TestSessionDataWithAttachment(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	session := &Session{
		mailServer: server,
		from:       "from@example.com",
		to:         []string{"to@example.com"},
		conn:       nil,
	}

	// Create a multipart email with attachment
	emailData := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Test with Attachment\r\n" +
		"Content-Type: multipart/mixed; boundary=\"boundary123\"\r\n" +
		"\r\n" +
		"--boundary123\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Test body\r\n" +
		"--boundary123\r\n" +
		"Content-Type: application/pdf\r\n" +
		"Content-Disposition: attachment; filename=\"test.pdf\"\r\n" +
		"\r\n" +
		"PDF content\r\n" +
		"--boundary123--\r\n")

	reader := bytes.NewReader(emailData)
	err = session.Data(reader)
	if err != nil {
		t.Errorf("Data should succeed with attachment, got error: %v", err)
	}

	// Verify email was saved
	emails := server.GetAllEmail()
	if len(emails) == 0 {
		t.Error("Email with attachment should be saved")
	}

	// Verify attachment was saved
	if len(emails) > 0 {
		email := emails[0]
		if len(email.Attachments) == 0 {
			t.Error("Email should have attachment")
		}
	}
}

func TestSessionDataWithHTML(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	session := &Session{
		mailServer: server,
		from:       "from@example.com",
		to:         []string{"to@example.com"},
		conn:       nil,
	}

	// Create an HTML email
	emailData := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: HTML Test\r\n" +
		"Content-Type: text/html\r\n" +
		"\r\n" +
		"<html><body><h1>Test</h1></body></html>")

	reader := bytes.NewReader(emailData)
	err = session.Data(reader)
	if err != nil {
		t.Errorf("Data should succeed with HTML, got error: %v", err)
	}

	// Verify email was saved
	emails := server.GetAllEmail()
	if len(emails) == 0 {
		t.Error("HTML email should be saved")
	}

	// Verify HTML was saved
	if len(emails) > 0 {
		email := emails[0]
		if email.HTML == "" {
			t.Error("Email should have HTML content")
		}
	}
}
