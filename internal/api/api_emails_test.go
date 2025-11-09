package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/gin-gonic/gin"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

func TestAPIGetAllEmails(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now(), Read: false}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now(), Read: true}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test email
	email := &types.Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response types.Email
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test email
	email := &types.Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add unread email
	email := &types.Email{ID: "test-id", Subject: "Test Subject", Read: false, Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add unread emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Read: false, Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Read: true, Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email with HTML
	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		HTML:    "<html><body>Test</body></html>",
		Time:    time.Now(),
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add unread emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Read: false, Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email with attachment
	email := &types.Email{
		ID:      "test-id",
		Subject: "Test",
		Attachments: []*types.Attachment{
			{
				GeneratedFileName: "test.pdf",
				ContentType:       "application/pdf",
			},
		},
		Time: time.Now(),
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email
	email := &types.Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email source content")
	if err := os.WriteFile(emlPath, content, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email
	email := &types.Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	content := []byte("test email content")
	if err := os.WriteFile(emlPath, content, 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{
		ID:      "id1",
		Subject: "Test Subject 1",
		Text:    "Test content 1",
		Time:    time.Now(),
		Read:    false,
		From:    []*mail.Address{{Address: "from1@example.com"}},
		To:      []*mail.Address{{Address: "to1@example.com"}},
	}
	email2 := &types.Email{
		ID:      "id2",
		Subject: "Test Subject 2",
		Text:    "Test content 2",
		Time:    time.Now().Add(-24 * time.Hour),
		Read:    true,
		From:    []*mail.Address{{Address: "from2@example.com"}},
		To:      []*mail.Address{{Address: "to2@example.com"}},
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test email
	email := &types.Email{
		ID:      "test-id",
		Subject: "Test Subject",
		Text:    "Test content for preview",
		Time:    time.Now(),
		Read:    false,
		From:    []*mail.Address{{Address: "from@example.com"}},
		To:      []*mail.Address{{Address: "to@example.com"}},
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/reload", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIReloadMailsFromDirectoryError(t *testing.T) {
	// Create a server with an invalid/non-existent directory to trigger error
	tmpDir := t.TempDir()
	invalidDir := filepath.Join(tmpDir, "nonexistent")

	server, err := mailserver.NewMailServer(1025, "localhost", invalidDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPI(server, 1080, "localhost")

	// Remove the directory to make it inaccessible
	if err := os.RemoveAll(invalidDir); err != nil {
		t.Fatalf("Failed to remove directory: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/emails/reload", nil)
	api.router.ServeHTTP(w, req)

	// Should return 500 error when reload fails
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email without attachment
	email := &types.Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email without subject
	email := &types.Email{ID: "test-id", Subject: "", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/raw", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIDownloadEmailWithRawEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email but don't create the .eml file
	email := &types.Email{ID: "test-id", Subject: "Test Subject", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	// Don't create eml file
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/raw", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestAPIGetEmailPreviewsWithFilters(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{
		ID:      "id1",
		Subject: "Test Subject 1",
		Text:    "Test content 1",
		Time:    time.Now(),
		Read:    false,
		From:    []*mail.Address{{Address: "from1@example.com"}},
		To:      []*mail.Address{{Address: "to1@example.com"}},
	}
	email2 := &types.Email{
		ID:      "id2",
		Subject: "Test Subject 2",
		Text:    "Test content 2",
		Time:    time.Now().Add(-24 * time.Hour),
		Read:    true,
		From:    []*mail.Address{{Address: "from2@example.com"}},
		To:      []*mail.Address{{Address: "to2@example.com"}},
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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

	// Test with query filter
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?q=Subject&limit=10&offset=0", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with from filter
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?from=from1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with to filter
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?to=to1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with read filter
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?read=false", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with dateFrom filter
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?dateFrom="+time.Now().Add(-48*time.Hour).Format("2006-01-02"), nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with dateTo filter
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?dateTo="+time.Now().Format("2006-01-02"), nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test with sortBy
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?sortBy=subject&sortOrder=asc", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetEmailPreviewsWithPagination(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add multiple test emails
	for i := 0; i < 5; i++ {
		email := &types.Email{
			ID:      fmt.Sprintf("id%d", i),
			Subject: fmt.Sprintf("Subject %d", i),
			Time:    time.Now().Add(-time.Duration(i) * time.Hour),
		}
		envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
		emlPath := filepath.Join(tmpDir, fmt.Sprintf("id%d.eml", i))
		if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create email file %d: %v", i, err)
		}
		if err := server.SaveEmailToStore(fmt.Sprintf("id%d", i), false, envelope, email); err != nil {
			t.Fatalf("Failed to save email %d: %v", i, err)
		}
	}

	// Test with limit and offset
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?limit=2&offset=1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["limit"] != float64(2) {
		t.Errorf("Expected limit 2, got %v", response["limit"])
	}
	if response["offset"] != float64(1) {
		t.Errorf("Expected offset 1, got %v", response["offset"])
	}
}

func TestAPIGetEmailPreviewsWithInvalidLimit(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?limit=invalid", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetEmailPreviewsWithInvalidOffset(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=invalid", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIGetEmailPreviewsWithLimitTooLarge(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?limit=2000", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["limit"] != float64(1000) {
		t.Errorf("Expected limit 1000 (max), got %v", response["limit"])
	}
}

func TestAPIGetEmailPreviewsWithNegativeOffset(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=-1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["offset"] != float64(0) {
		t.Errorf("Expected offset 0 (min), got %v", response["offset"])
	}
}

func TestAPIGetAllEmailsWithOffsetBeyondTotal(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add only 2 emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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

	// Test with offset beyond total
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=100&limit=10", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	emails := response["emails"].([]interface{})
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails (offset beyond total), got %d", len(emails))
	}
}

func TestAPIGetAllEmailsWithStartEqualsEnd(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add only 1 email
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file 1: %v", err)
	}
	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}

	// Test with offset=1, limit=1 (start == end == 1)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=1&limit=1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	emails := response["emails"].([]interface{})
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails (start == end), got %d", len(emails))
	}
}

func TestAPIGetAllEmailsWithLimitZero(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file 1: %v", err)
	}
	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}

	// Test with limit=0 (should default to 50)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=0", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["limit"] != float64(50) {
		t.Errorf("Expected limit 50 (default), got %v", response["limit"])
	}
}

func TestAPIGetAllEmailsWithLimitOne(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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

	// Test with limit=1
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	emails := response["emails"].([]interface{})
	if len(emails) != 1 {
		t.Errorf("Expected 1 email, got %d", len(emails))
	}
}

func TestAPIGetEmailSourceNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add one email
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["success"] != float64(1) {
		t.Errorf("Expected 1 success, got %v", response["success"])
	}
	if response["failed"] != float64(1) {
		t.Errorf("Expected 1 failed, got %v", response["failed"])
	}
}

func TestAPIBatchReadEmailsInvalidRequest(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add one email
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["success"] != float64(1) {
		t.Errorf("Expected 1 success, got %v", response["success"])
	}
	if response["failed"] != float64(1) {
		t.Errorf("Expected 1 failed, got %v", response["failed"])
	}
}

func TestAPIBatchReadEmailsAlreadyRead(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add unread email first
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Read: false, Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Mark as read first
	if err := server.ReadEmail("id1"); err != nil {
		t.Fatalf("Failed to mark email as read: %v", err)
	}

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
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	// Should not count as success if already read
	if response["success"] != float64(0) {
		t.Errorf("Expected 0 success (already read), got %v", response["success"])
	}
}

func TestAPIGetAllEmailsPagination(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add multiple emails
	for i := 0; i < 5; i++ {
		email := &types.Email{ID: fmt.Sprintf("id%d", i), Subject: fmt.Sprintf("Subject %d", i), Time: time.Now().Add(time.Duration(i) * time.Hour)}
		envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
		emlPath := filepath.Join(tmpDir, fmt.Sprintf("id%d.eml", i))
		if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create email file %d: %v", i, err)
		}
		if err := server.SaveEmailToStore(fmt.Sprintf("id%d", i), false, envelope, email); err != nil {
			t.Fatalf("Failed to save email %d: %v", i, err)
		}
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
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["limit"] != float64(2) {
		t.Errorf("Expected limit 2, got %v", response["limit"])
	}
	if response["offset"] != float64(1) {
		t.Errorf("Expected offset 1, got %v", response["offset"])
	}
}

func TestAPIGetAllEmailsInvalidLimit(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=invalid", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should default to 50
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["limit"] != float64(50) {
		t.Errorf("Expected default limit 50, got %v", response["limit"])
	}
}

func TestAPIGetAllEmailsLargeLimit(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=2000", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should cap at 1000
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["limit"] != float64(1000) {
		t.Errorf("Expected capped limit 1000, got %v", response["limit"])
	}
}

func TestAPIGetAllEmailsInvalidOffset(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=invalid", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should default to 0
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["offset"] != float64(0) {
		t.Errorf("Expected default offset 0, got %v", response["offset"])
	}
}

func TestAPIGetAllEmailsNegativeOffset(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=-1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Should default to 0
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if response["offset"] != float64(0) {
		t.Errorf("Expected default offset 0, got %v", response["offset"])
	}
}

func TestAPIGetAllEmailsSorting(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add emails with different subjects
	email1 := &types.Email{ID: "id1", Subject: "A Subject", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "B Subject", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add emails with different from addresses
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", From: []*mail.Address{{Address: "a@example.com"}}, Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", From: []*mail.Address{{Address: "b@example.com"}}, Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add emails with different sizes
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Size: 100, Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Size: 200, Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add emails with different dates
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now().Add(-48 * time.Hour)}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email with CC
	email := &types.Email{
		ID:      "id1",
		Subject: "Subject 1",
		CC:      []*mail.Address{{Address: "cc@example.com", Name: "CC Name"}},
		Time:    time.Now(),
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("id1", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email with BCC
	email := &types.Email{
		ID:            "id1",
		Subject:       "Subject 1",
		CalculatedBCC: []*mail.Address{{Address: "bcc@example.com"}},
		Time:          time.Now(),
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("id1", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email with HTML but no text
	email := &types.Email{
		ID:      "test-id",
		Subject: "Test Subject",
		HTML:    "<html><body>Test content for preview</body></html>",
		Time:    time.Now(),
		Read:    false,
		From:    []*mail.Address{{Address: "from@example.com"}},
		To:      []*mail.Address{{Address: "to@example.com"}},
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email with very long text
	longText := string(make([]byte, 500))
	for i := range longText {
		longText = longText[:i] + "a" + longText[i+1:]
	}
	email := &types.Email{
		ID:      "test-id",
		Subject: "Test Subject",
		Text:    longText,
		Time:    time.Now(),
		Read:    false,
		From:    []*mail.Address{{Address: "from@example.com"}},
		To:      []*mail.Address{{Address: "to@example.com"}},
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Test Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Other Subject", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/export", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestAPIExportEmailsWithMissingFiles(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	// Create only one eml file
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}
	if err := server.SaveEmailToStore("id2", false, envelope, email2); err != nil {
		t.Fatalf("Failed to save email 2: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/export", nil)
	api.router.ServeHTTP(w, req)

	// Should return 200 (ZIP created with available files, missing files are skipped)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIExportEmailsWithIDsAndMissingFiles(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	// Create only one eml file
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email 1: %v", err)
	}
	if err := server.SaveEmailToStore("id2", false, envelope, email2); err != nil {
		t.Fatalf("Failed to save email 2: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=id1,id2", nil)
	api.router.ServeHTTP(w, req)

	// Should return 200 (ZIP created with available files)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIExportEmailsWithIDsWithSpaces(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

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

	// Test with IDs containing spaces
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=id1, id2 , id3", nil)
	api.router.ServeHTTP(w, req)

	// Should return 200 (spaces are trimmed)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIExportEmailsWithEmptyIDs(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Test with empty IDs list (should use filter instead, which returns all emails)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=", nil)
	api.router.ServeHTTP(w, req)

	// Should return 200 (empty ids param means use filter, which returns all emails)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAPIExportEmailsWithNonExistentIDs(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add test emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}

	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}

	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Test with non-existent IDs
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=nonexistent1,nonexistent2", nil)
	api.router.ServeHTTP(w, req)

	// Should return 400 (no emails found)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestApplyEmailSorting(t *testing.T) {
	now := time.Now()
	emails := []*types.Email{
		{ID: "1", Subject: "B Subject", Time: now.Add(-2 * time.Hour), Size: 200, From: []*mail.Address{{Address: "b@example.com"}}},
		{ID: "2", Subject: "A Subject", Time: now.Add(-1 * time.Hour), Size: 100, From: []*mail.Address{{Address: "a@example.com"}}},
		{ID: "3", Subject: "C Subject", Time: now, Size: 300, From: []*mail.Address{{Address: "c@example.com"}}},
	}

	// Test sorting by time (desc)
	applyEmailSorting(emails, "time", "desc")
	if emails[0].ID != "3" {
		t.Errorf("Expected first email ID '3', got '%s'", emails[0].ID)
	}

	// Test sorting by time (asc)
	applyEmailSorting(emails, "time", "asc")
	if emails[0].ID != "1" {
		t.Errorf("Expected first email ID '1', got '%s'", emails[0].ID)
	}

	// Test sorting by subject (asc)
	applyEmailSorting(emails, "subject", "asc")
	if emails[0].Subject != "A Subject" {
		t.Errorf("Expected first email subject 'A Subject', got '%s'", emails[0].Subject)
	}

	// Test sorting by subject (desc)
	applyEmailSorting(emails, "subject", "desc")
	if emails[0].Subject != "C Subject" {
		t.Errorf("Expected first email subject 'C Subject', got '%s'", emails[0].Subject)
	}

	// Test sorting by from (asc)
	applyEmailSorting(emails, "from", "asc")
	if emails[0].From[0].Address != "a@example.com" {
		t.Errorf("Expected first email from 'a@example.com', got '%s'", emails[0].From[0].Address)
	}

	// Test sorting by size (asc)
	applyEmailSorting(emails, "size", "asc")
	if emails[0].Size != 100 {
		t.Errorf("Expected first email size 100, got %d", emails[0].Size)
	}

	// Test sorting by size (desc)
	applyEmailSorting(emails, "size", "desc")
	if emails[0].Size != 300 {
		t.Errorf("Expected first email size 300, got %d", emails[0].Size)
	}

	// Test with empty from
	emails2 := []*types.Email{
		{ID: "1", Subject: "A", From: []*mail.Address{}},
		{ID: "2", Subject: "B", From: []*mail.Address{{Address: "b@example.com"}}},
	}
	applyEmailSorting(emails2, "from", "asc")
	// Should not panic

	// Test with empty from (desc)
	applyEmailSorting(emails2, "from", "desc")
	// Should not panic

	// Test with unknown sortBy (should not panic)
	emails3 := []*types.Email{
		{ID: "1", Subject: "A", Time: now},
		{ID: "2", Subject: "B", Time: now.Add(-time.Hour)},
	}
	applyEmailSorting(emails3, "unknown", "asc")
	// Should not panic, should not change order
}

func TestApplyEmailFilters(t *testing.T) {
	now := time.Now()
	emails := []*types.Email{
		{
			ID:            "1",
			Subject:       "Test Subject 1",
			Text:          "Test content 1",
			HTML:          "<html>Test HTML 1</html>",
			Time:          now,
			Read:          false,
			From:          []*mail.Address{{Address: "from1@example.com", Name: "From One"}},
			To:            []*mail.Address{{Address: "to1@example.com"}},
			CC:            []*mail.Address{{Address: "cc1@example.com"}},
			CalculatedBCC: []*mail.Address{{Address: "bcc1@example.com"}},
		},
		{
			ID:      "2",
			Subject: "Other Subject",
			Text:    "Other content",
			Time:    now.Add(-24 * time.Hour),
			Read:    true,
			From:    []*mail.Address{{Address: "from2@example.com"}},
			To:      []*mail.Address{{Address: "to2@example.com"}},
		},
	}

	// Test with query filter
	filtered := applyEmailFilters(emails, "Test", "", "", "", "", "")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test with from filter
	filtered = applyEmailFilters(emails, "", "from1", "", "", "", "")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test with from filter by name
	filtered = applyEmailFilters(emails, "", "From One", "", "", "", "")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test with to filter
	filtered = applyEmailFilters(emails, "", "", "to1", "", "", "")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test with to filter by CC
	filtered = applyEmailFilters(emails, "", "", "cc1", "", "", "")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test with to filter by BCC
	filtered = applyEmailFilters(emails, "", "", "bcc1", "", "", "")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test with dateFrom filter
	filtered = applyEmailFilters(emails, "", "", "", now.Add(-48*time.Hour).Format("2006-01-02"), "", "")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 emails, got %d", len(filtered))
	}

	// Test with dateTo filter
	filtered = applyEmailFilters(emails, "", "", "", "", now.Format("2006-01-02"), "")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 emails, got %d", len(filtered))
	}

	// Test with read filter (false)
	filtered = applyEmailFilters(emails, "", "", "", "", "", "false")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test with read filter (true)
	filtered = applyEmailFilters(emails, "", "", "", "", "", "true")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test with invalid dateFrom
	filtered = applyEmailFilters(emails, "", "", "", "invalid-date", "", "")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 emails (no filter applied), got %d", len(filtered))
	}

	// Test with invalid dateTo
	filtered = applyEmailFilters(emails, "", "", "", "", "invalid-date", "")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 emails (no filter applied), got %d", len(filtered))
	}

	// Test with no filters
	filtered = applyEmailFilters(emails, "", "", "", "", "", "")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 emails, got %d", len(filtered))
	}

	// Test with empty email (no From, To, etc.)
	emails3 := []*types.Email{
		{
			ID:      "3",
			Subject: "Empty Email",
			Text:    "Content",
			Time:    now,
			Read:    false,
			From:    []*mail.Address{},
			To:      []*mail.Address{},
		},
	}

	// Test query filter with empty email
	filtered = applyEmailFilters(emails3, "Content", "", "", "", "", "")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 email, got %d", len(filtered))
	}

	// Test from filter with empty From
	filtered = applyEmailFilters(emails3, "", "test", "", "", "", "")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 emails (no match), got %d", len(filtered))
	}

	// Test to filter with empty To
	filtered = applyEmailFilters(emails3, "", "", "test", "", "", "")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 emails (no match), got %d", len(filtered))
	}

	// Test dateFrom filter with email before date
	filtered = applyEmailFilters(emails3, "", "", "", now.Add(24*time.Hour).Format("2006-01-02"), "", "")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 emails (before date), got %d", len(filtered))
	}

	// Test dateTo filter with email after date
	filtered = applyEmailFilters(emails3, "", "", "", "", now.Add(-48*time.Hour).Format("2006-01-02"), "")
	if len(filtered) != 0 {
		t.Errorf("Expected 0 emails (after date), got %d", len(filtered))
	}
}

// TestAPIDeleteAllEmailsError tests the error path in deleteAllEmails
// Note: DeleteAllEmail() currently doesn't return errors in normal cases,
// but we test the API endpoint structure
func TestAPIDeleteAllEmailsError(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Test with empty server (should still work)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/emails", nil)
	api.router.ServeHTTP(w, req)

	// Should return 200 even with no emails
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestAPIGetEmailPreviewsBoundaryConditions tests boundary conditions in getEmailPreviews
func TestAPIGetEmailPreviewsBoundaryConditions(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add only 2 emails
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Text: "Content 1", Time: time.Now()}
	email2 := &types.Email{ID: "id2", Subject: "Subject 2", Text: "Content 2", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
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

	// Test with offset > total (start > total case)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=100&limit=10", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	previews := response["previews"].([]interface{})
	if len(previews) != 0 {
		t.Errorf("Expected 0 previews (offset > total), got %d", len(previews))
	}

	// Test with offset + limit > total (end > total case)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=1&limit=10", nil)
	api.router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	var response2 map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &response2); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	previews2 := response2["previews"].([]interface{})
	// Should return 1 email (total is 2, offset is 1, so end would be 11 but capped at 2)
	if len(previews2) != 1 {
		t.Errorf("Expected 1 preview (end > total case), got %d", len(previews2))
	}
}

// TestAPIGetEmailPreviewsMultipleSpaces tests text processing with multiple spaces
func TestAPIGetEmailPreviewsMultipleSpaces(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add email with HTML containing multiple spaces
	email := &types.Email{
		ID:      "test-id",
		Subject: "Test Subject",
		HTML:    "<html><body>Text   with    multiple     spaces</body></html>",
		Time:    time.Now(),
		Read:    false,
		From:    []*mail.Address{{Address: "from@example.com"}},
		To:      []*mail.Address{{Address: "to@example.com"}},
	}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	previews := response["previews"].([]interface{})
	if len(previews) > 0 {
		preview := previews[0].(map[string]interface{})
		previewText := preview["preview"].(string)
		// Check that multiple spaces are collapsed
		if strings.Contains(previewText, "     ") {
			t.Errorf("Multiple spaces should be collapsed, got: %s", previewText)
		}
	}
}

// TestAPIGetEmailPreviewsStartEqualsEnd tests the case where start == end
func TestAPIGetEmailPreviewsStartEqualsEnd(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Add only 1 email
	email1 := &types.Email{ID: "id1", Subject: "Subject 1", Text: "Content 1", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath1 := filepath.Join(tmpDir, "id1.eml")
	if err := os.WriteFile(emlPath1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("id1", false, envelope, email1); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	// Test with offset=1, limit=1 (start == end == 1)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=1&limit=1", nil)
	api.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	previews := response["previews"].([]interface{})
	// When start == end, should return empty array
	if len(previews) != 0 {
		t.Errorf("Expected 0 previews (start == end), got %d", len(previews))
	}
}
