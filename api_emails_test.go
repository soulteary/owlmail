package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/gin-gonic/gin"
)

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

func TestAPIGetEmailHTMLNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/nonexistent/html", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIGetAttachmentNotFound(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email without attachment
	email := &Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/attachments/nonexistent.pdf", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIDownloadEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/nonexistent/raw", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIDownloadEmailWithoutSubject(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email without subject
	email := &Email{ID: "test-id", Subject: "", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	os.WriteFile(emlPath, []byte("content"), 0644)
	server.saveEmailToStore("test-id", false, envelope, email)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/raw", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetEmailSourceNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/nonexistent/source", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIDeleteEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/emails/nonexistent", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIReadEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/emails/nonexistent/read", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIBatchDeleteEmailsInvalidRequest(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/emails/batch", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIBatchDeleteEmailsEmptyIDs(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	requestBody := map[string]interface{}{
		"ids": []string{},
	}
	jsonBody, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/emails/batch", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIBatchDeleteEmailsPartialFailure(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add one email
	email1 := &Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	server.saveEmailToStore("id1", false, envelope, email1)

	// Try to delete both existing and non-existing
	requestBody := map[string]interface{}{
		"ids": []string{"id1", "nonexistent"},
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

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["success"] != float64(1) {
		t.Errorf("Expected 1 success, got %v", response["success"])
	}
	if response["failed"] != float64(1) {
		t.Errorf("Expected 1 failed, got %v", response["failed"])
	}
}

func TestAPIBatchReadEmailsInvalidRequest(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/emails/batch/read", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIBatchReadEmailsEmptyIDs(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	requestBody := map[string]interface{}{
		"ids": []string{},
	}
	jsonBody, _ := json.Marshal(requestBody)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/emails/batch/read", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIBatchReadEmailsPartialFailure(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add one email
	email1 := &Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	server.saveEmailToStore("id1", false, envelope, email1)

	// Try to read both existing and non-existing
	requestBody := map[string]interface{}{
		"ids": []string{"id1", "nonexistent"},
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

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["success"] != float64(1) {
		t.Errorf("Expected 1 success, got %v", response["success"])
	}
	if response["failed"] != float64(1) {
		t.Errorf("Expected 1 failed, got %v", response["failed"])
	}
}

func TestAPIBatchReadEmailsAlreadyRead(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add unread email first
	email1 := &Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	server.saveEmailToStore("id1", false, envelope, email1)

	// Mark as read first
	server.ReadEmail("id1")

	// Now try to read it again
	requestBody := map[string]interface{}{
		"ids": []string{"id1"},
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

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	// Should not count as success if already read
	if response["success"] != float64(0) {
		t.Errorf("Expected 0 success (already read), got %v", response["success"])
	}
}

func TestAPIGetAllEmailsPagination(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add multiple emails
	for i := 0; i < 5; i++ {
		email := &Email{ID: fmt.Sprintf("id%d", i), Subject: fmt.Sprintf("Subject %d", i), Time: time.Now().Add(time.Duration(i) * time.Hour)}
		envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
		emlPath := filepath.Join(tmpDir, fmt.Sprintf("id%d.eml", i))
		os.WriteFile(emlPath, []byte("content"), 0644)
		server.saveEmailToStore(fmt.Sprintf("id%d", i), false, envelope, email)
	}

	// Test pagination
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=2&offset=1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["limit"] != float64(2) {
		t.Errorf("Expected limit 2, got %v", response["limit"])
	}
	if response["offset"] != float64(1) {
		t.Errorf("Expected offset 1, got %v", response["offset"])
	}
}

func TestAPIGetAllEmailsInvalidLimit(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=invalid", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should default to 50
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["limit"] != float64(50) {
		t.Errorf("Expected default limit 50, got %v", response["limit"])
	}
}

func TestAPIGetAllEmailsLargeLimit(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=2000", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should cap at 1000
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["limit"] != float64(1000) {
		t.Errorf("Expected capped limit 1000, got %v", response["limit"])
	}
}

func TestAPIGetAllEmailsInvalidOffset(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=invalid", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should default to 0
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["offset"] != float64(0) {
		t.Errorf("Expected default offset 0, got %v", response["offset"])
	}
}

func TestAPIGetAllEmailsNegativeOffset(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=-1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should default to 0
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if response["offset"] != float64(0) {
		t.Errorf("Expected default offset 0, got %v", response["offset"])
	}
}

func TestAPIGetAllEmailsSorting(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add emails with different subjects
	email1 := &Email{ID: "id1", Subject: "A Subject", Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "B Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)
	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

	// Test sorting by subject ascending
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?sortBy=subject&sortOrder=asc", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetAllEmailsSortingByFrom(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add emails with different from addresses
	email1 := &Email{ID: "id1", Subject: "Subject 1", From: []*mail.Address{{Address: "a@example.com"}}, Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", From: []*mail.Address{{Address: "b@example.com"}}, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)
	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

	// Test sorting by from
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?sortBy=from&sortOrder=asc", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetAllEmailsSortingBySize(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add emails with different sizes
	email1 := &Email{ID: "id1", Subject: "Subject 1", Size: 100, Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Size: 200, Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)
	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

	// Test sorting by size
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?sortBy=size&sortOrder=asc", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetAllEmailsDateFilters(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add emails with different dates
	email1 := &Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Subject 2", Time: time.Now().Add(-48 * time.Hour)}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)
	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

	// Test dateFrom filter
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	dateFrom := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	req, _ := http.NewRequest("GET", "/api/v1/emails?dateFrom="+dateFrom, nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test dateTo filter
	w2 := httptest.NewRecorder()
	dateTo := time.Now().Format("2006-01-02")
	req2, _ := http.NewRequest("GET", "/api/v1/emails?dateTo="+dateTo, nil)
	api.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}
}

func TestAPIGetAllEmailsFilterByCC(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email with CC
	email := &Email{
		ID:      "id1",
		Subject: "Subject 1",
		CC:      []*mail.Address{{Address: "cc@example.com", Name: "CC Name"}},
		Time:    time.Now(),
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "id1.eml")
	os.WriteFile(emlPath, []byte("content1"), 0644)
	server.saveEmailToStore("id1", false, envelope, email)

	// Test filter by CC
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?to=cc", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetAllEmailsFilterByBCC(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email with BCC
	email := &Email{
		ID:            "id1",
		Subject:       "Subject 1",
		CalculatedBCC: []*mail.Address{{Address: "bcc@example.com"}},
		Time:          time.Now(),
	}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "id1.eml")
	os.WriteFile(emlPath, []byte("content1"), 0644)
	server.saveEmailToStore("id1", false, envelope, email)

	// Test filter by BCC
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?to=bcc", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetEmailPreviewsWithHTML(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email with HTML but no text
	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		HTML:    "<html><body>Test content for preview</body></html>",
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

func TestAPIGetEmailPreviewsLongText(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add email with very long text
	longText := string(make([]byte, 500))
	for i := range longText {
		longText = longText[:i] + "a" + longText[i+1:]
	}
	email := &Email{
		ID:      "test-id",
		Subject: "Test Subject",
		Text:    longText,
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

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	previews := response["previews"].([]interface{})
	if len(previews) > 0 {
		preview := previews[0].(map[string]interface{})
		previewText := preview["preview"].(string)
		if len(previewText) > 203 { // 200 chars + "..."
			t.Errorf("Preview text should be truncated to 200 chars, got %d", len(previewText))
		}
	}
}

func TestAPIExportEmails(t *testing.T) {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails/export", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/zip" {
		t.Errorf("Expected Content-Type application/zip, got %s", w.Header().Get("Content-Type"))
	}
}

func TestAPIExportEmailsWithIDs(t *testing.T) {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=id1,id2", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIExportEmailsWithFilters(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer server.Close()

	// Add test emails
	email1 := &Email{ID: "id1", Subject: "Test Subject 1", Time: time.Now()}
	email2 := &Email{ID: "id2", Subject: "Other Subject", Time: time.Now()}
	envelope := &Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	emlPath2 := filepath.Join(tmpDir, "id2.eml")
	os.WriteFile(emlPath1, []byte("content1"), 0644)
	os.WriteFile(emlPath2, []byte("content2"), 0644)
	server.saveEmailToStore("id1", false, envelope, email1)
	server.saveEmailToStore("id2", false, envelope, email2)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?q=Test", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIExportEmailsNoEmails(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer server.Close()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/export", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
