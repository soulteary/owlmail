package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/gin-gonic/gin"
)

func setupTestAPI(t *testing.T) (*API, *MailServer, string) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	api := NewAPI(server, 1080, "localhost")
	return api, server, tmpDir
}

func TestNewAPI(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPI(server, 1080, "localhost")
	if api == nil {
		t.Error("NewAPI should not return nil")
	}
	if api.mailServer != server {
		t.Error("API should have correct mail server")
	}
	if api.port != 1080 {
		t.Errorf("Expected port 1080, got %d", api.port)
	}
	if api.host != "localhost" {
		t.Errorf("Expected host localhost, got %s", api.host)
	}
}

func TestNewAPIWithAuth(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	if api == nil {
		t.Error("NewAPIWithAuth should not return nil")
	}
	if api.authUser != "user" {
		t.Errorf("Expected auth user 'user', got '%s'", api.authUser)
	}
	if api.authPassword != "pass" {
		t.Errorf("Expected auth password 'pass', got '%s'", api.authPassword)
	}
}

func TestNewAPIWithHTTPS(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPIWithHTTPS(server, 1080, "localhost", "user", "pass", true, "cert.pem", "key.pem")
	if api == nil {
		t.Error("NewAPIWithHTTPS should not return nil")
	}
	if !api.httpsEnabled {
		t.Error("HTTPS should be enabled")
	}
	if api.httpsCertFile != "cert.pem" {
		t.Errorf("Expected cert file 'cert.pem', got '%s'", api.httpsCertFile)
	}
	if api.httpsKeyFile != "key.pem" {
		t.Errorf("Expected key file 'key.pem', got '%s'", api.httpsKeyFile)
	}
}

func TestAPIHealthCheck(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", response["status"])
	}
}

func TestAPIGetAllEmails(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Time: time.Now(), Read: false}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Time: time.Now(), Read: true}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", true, envelope, email2)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response["total"] == nil {
		t.Error("Response should have total field")
	}
}

func TestAPIGetEmailByID(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response Email
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", response.ID)
	}

	// Test non-existent email
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/emails/nonexistent", nil)
	api.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w2.Code)
	}
}

func TestAPIDeleteEmail(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/emails/test-id", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify deleted
	_, err := server.GetEmail("test-id")
	if err == nil {
		t.Error("Email should be deleted")
	}
}

func TestAPIDeleteAllEmails(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/emails", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify all deleted
	emails := server.GetAllEmail()
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails, got %d", len(emails))
	}
}

func TestAPIReadEmail(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add unread email
	email := &Email{ID: "test-id", Subject: "Test Subject", Read: false, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/emails/test-id/read", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
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

func TestAPIReadAllEmails(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
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

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/emails/read", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify all read
	emails := server.GetAllEmail()
	for _, email := range emails {
		if !email.Read {
			t.Error("All emails should be marked as read")
		}
	}
}

func TestAPIGetEmailStats(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Read: true, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", true, envelope, email2)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/stats", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response["total"] == nil {
		t.Error("Response should have total field")
	}
}

func TestAPIGetConfig(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/settings", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response["version"] == nil {
		t.Error("Response should have version field")
	}
}

func TestAPIGetEmailHTML(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email with HTML
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		HTML:    "<html><body>Test</body></html>",
		Time:    time.Now(),
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/html", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type text/html; charset=utf-8, got %s", w.Header().Get("Content-Type"))
	}
}

func TestAPIBatchDeleteEmails(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test emails
	email1 := &Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

	// Batch delete
	requestBody := map[string]interface{}{
		"ids": []string{"id1", "id2"},
	}
	jsonBody, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/emails/batch", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify deleted
	emails := server.GetAllEmail()
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails, got %d", len(emails))
	}
}

func TestAPIBatchReadEmails(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
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

	// Batch read
	requestBody := map[string]interface{}{
		"ids": []string{"id1", "id2"},
	}
	jsonBody, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/emails/batch/read", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify all read
	emails := server.GetAllEmail()
	for _, email := range emails {
		if !email.Read {
			t.Error("All emails should be marked as read")
		}
	}
}

func TestAPISanitizeFilename(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"test.pdf", "test.pdf"},
		{"test/file.pdf", "test_file.pdf"},
		{"test\\file.pdf", "test_file.pdf"},
		{"test:file.pdf", "test_file.pdf"},
		{"test*file.pdf", "test_file.pdf"},
		{"test?file.pdf", "test_file.pdf"},
		{"test\"file.pdf", "test_file.pdf"},
		{"test<file.pdf", "test_file.pdf"},
		{"test>file.pdf", "test_file.pdf"},
		{"test|file.pdf", "test_file.pdf"},
	}

	for _, tc := range testCases {
		result := sanitizeFilename(tc.input)
		if result != tc.expected {
			t.Errorf("For input '%s': Expected '%s', got '%s'", tc.input, tc.expected, result)
		}
	}

	// Test long filename
	longName := string(make([]byte, 150))
	result := sanitizeFilename(longName)
	if len(result) > 100 {
		t.Errorf("Long filename should be truncated to 100 chars, got %d", len(result))
	}
}

func TestCorsMiddleware(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/v1/health", nil)
	req.Header.Set("Origin", "http://example.com")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS headers should be set")
	}
}

func TestBasicAuthMiddleware(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAPIGetAttachment(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email with attachment
	email := &Email{
		ID:      "test-id",
		Subject: "Test",
		Attachments: []*Attachment{
			{
				GeneratedFileName: "test.pdf",
				ContentType:       "application/pdf",
			},
		},
		Time: time.Now(),
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

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/attachments/test.pdf", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetEmailSource(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email
	email := &Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email source content")
	os.WriteFile(emlPath, content, 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/source", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("Expected Content-Type text/plain; charset=utf-8, got %s", w.Header().Get("Content-Type"))
	}
}

func TestAPIDownloadEmail(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email
	email := &Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email content")
	os.WriteFile(emlPath, content, 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/raw", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetOutgoingConfig(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/settings/outgoing", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIUpdateOutgoingConfig(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	config := map[string]interface{}{
		"host":   "smtp.example.com",
		"port":   587,
		"user":   "user",
		"secure": true,
	}
	jsonBody, _ := json.Marshal(config)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/settings/outgoing", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIPatchOutgoingConfig(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	// First set a config
	config := map[string]interface{}{
		"host": "smtp.example.com",
		"port": 587,
	}
	jsonBody, _ := json.Marshal(config)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/settings/outgoing", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	// Then patch it
	patch := map[string]interface{}{
		"port": 465,
	}
	patchBody, _ := json.Marshal(patch)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("PATCH", "/api/v1/settings/outgoing", bytes.NewBuffer(patchBody))
	req2.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}
}

func TestAPIGetAllEmailsWithFilters(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test emails
	email1 := &Email{
		ID:      "id1",
		Subject: "Test Subject 1",
		Text:    "Test content 1",
		Time:    time.Now(),
		Read:    false,
		From:    []*mail.Address{{Address: "from1@example.com"}},
		To:      []*mail.Address{{Address: "to1@example.com"}},
	}
	email2 := &Email{
		ID:      "id2",
		Subject: "Test Subject 2",
		Text:    "Test content 2",
		Time:    time.Now().Add(-24 * time.Hour),
		Read:    true,
		From:    []*mail.Address{{Address: "from2@example.com"}},
		To:      []*mail.Address{{Address: "to2@example.com"}},
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)

	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", true, envelope, email2)

	// Test with query filter
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?q=Subject&limit=10&offset=0", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with from filter
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/emails?from=from1", nil)
	api.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	// Test with to filter
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/api/v1/emails?to=to1", nil)
	api.router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w3.Code)
	}

	// Test with read filter
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/api/v1/emails?read=false", nil)
	api.router.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w4.Code)
	}

	// Test with sort
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("GET", "/api/v1/emails?sortBy=subject&sortOrder=asc", nil)
	api.router.ServeHTTP(w5, req5)

	if w5.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w5.Code)
	}
}

func TestAPIGetEmailPreviews(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test email
	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		Text:    "Test content for preview",
		Time:    time.Now(),
		Read:    false,
		From:    []*mail.Address{{Address: "from@example.com"}},
		To:      []*mail.Address{{Address: "to@example.com"}},
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)

	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIReloadMailsFromDirectory(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/reload", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestBasicAuthMiddlewareSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer server.Close()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	req.SetBasicAuth("user", "pass")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Note: WebSocket tests are more complex and would require a WebSocket client
// For now, we'll skip comprehensive WebSocket testing

