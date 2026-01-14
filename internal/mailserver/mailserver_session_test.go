package mailserver

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestBackendNewSessionBasic(t *testing.T) {
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

	// Create a backend instance
	backend := &Backend{mailServer: server}

	// Create a mock SMTP connection
	// We can't easily create a real smtp.Conn, so we'll test the function exists
	// In a real scenario, this would be called by the SMTP server
	_ = backend.NewSession
}

func TestBackendNewSession(t *testing.T) {
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

func TestSessionMail(t *testing.T) {
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

// TestSessionMailWithConnNilConn tests Mail method when conn is nil (covers the else branch)
func TestSessionMailWithConnNilConn(t *testing.T) {
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

	// Create a session with a nil conn pointer - this tests the else branch (conn == nil)
	session := &Session{
		mailServer:    server,
		authenticated: false,
		conn:          nil, // This tests the else branch (conn == nil)
	}

	server.authConfig = &SMTPAuthConfig{
		Username: "user",
		Password: "pass",
		Enabled:  true,
	}

	err = session.Mail("from@example.com", nil)
	if err != nil {
		t.Errorf("Mail should succeed, got error: %v", err)
	}
}

func TestSessionRcpt(t *testing.T) {
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	session := &Session{
		mailServer: server,
		from:       "from@example.com",
		to:         []string{"to@example.com"},
		conn:       nil, // conn is nil in test, but Data should handle it
	}

	// Create a simple email message
	emailData := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: =?UTF-8?Q?=E6=B5=8B=E8=AF=95=E6=B6=88=E6=81=AF?=\r\n" +
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
	if emails[0].Subject != "测试消息" {
		t.Error("Email subject should be properly decoded")
	}
}

func TestSessionReset(t *testing.T) {
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	session := &Session{
		mailServer: server,
	}

	// Test Logout
	err = session.Logout()
	if err != nil {
		t.Errorf("Logout should succeed, got error: %v", err)
	}
}

func TestSessionDataWithAttachment(t *testing.T) {
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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

// TestSessionDataFileCreationError tests Data method when file creation fails
func TestSessionDataFileCreationError(t *testing.T) {
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

	session := &Session{
		mailServer: server,
		from:       "from@example.com",
		to:         []string{"to@example.com"},
		conn:       nil,
	}

	// Create a simple email message
	emailData := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Test\r\n" +
		"\r\n" +
		"Test body")

	reader := bytes.NewReader(emailData)

	// Set mailDir to a non-existent path to trigger file creation error
	originalMailDir := server.mailDir
	// Use a path that doesn't exist and can't be created (parent doesn't exist)
	server.mailDir = filepath.Join(tmpDir, "nonexistent", "subdir", "path")

	err = session.Data(reader)
	if err == nil {
		t.Error("Data should fail when file creation fails")
	}

	// Restore original mailDir
	server.mailDir = originalMailDir
}

// TestSessionDataWithInvalidMailDir tests Data method with invalid mail directory
func TestSessionDataWithInvalidMailDir(t *testing.T) {
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

	session := &Session{
		mailServer: server,
		from:       "from@example.com",
		to:         []string{"to@example.com"},
		conn:       nil,
	}

	// Create a simple email message
	emailData := []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: Test\r\n" +
		"\r\n" +
		"Test body")

	reader := bytes.NewReader(emailData)

	// Set mailDir to an invalid path (too long path on some systems)
	originalMailDir := server.mailDir
	// Create a path that's likely to be invalid
	invalidPath := filepath.Join(tmpDir, string(make([]byte, 300))) // Very long path
	server.mailDir = invalidPath

	err = session.Data(reader)
	if err == nil {
		t.Error("Data should fail with invalid mail directory")
	}

	// Restore original mailDir
	server.mailDir = originalMailDir
}

// TestSessionNewSessionWithAuthConfigNil tests NewSession when authConfig is nil
func TestSessionNewSessionWithAuthConfigNil(t *testing.T) {
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

	// Ensure authConfig is nil
	server.authConfig = nil

	backend := &Backend{mailServer: server}
	session, err := backend.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession should succeed: %v", err)
	}
	if session == nil {
		t.Error("Session should not be nil")
	}
	// When authConfig is nil, authenticated should be true
	if s, ok := session.(*Session); ok {
		if !s.authenticated {
			t.Error("Session should be authenticated when authConfig is nil")
		}
	}
}

// TestSessionNewSessionWithAuthConfigDisabled tests NewSession when auth is disabled
func TestSessionNewSessionWithAuthConfigDisabled(t *testing.T) {
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

	// Set authConfig but with Enabled = false
	server.authConfig = &SMTPAuthConfig{
		Username: "user",
		Password: "pass",
		Enabled:  false,
	}

	backend := &Backend{mailServer: server}
	session, err := backend.NewSession(nil)
	if err != nil {
		t.Fatalf("NewSession should succeed: %v", err)
	}
	if session == nil {
		t.Error("Session should not be nil")
	}
	// When auth is disabled, authenticated should be true
	if s, ok := session.(*Session); ok {
		if !s.authenticated {
			t.Error("Session should be authenticated when auth is disabled")
		}
	}
}
