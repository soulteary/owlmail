package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/gofiber/fiber/v3"
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

	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response types.Email
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", response.ID)
	}

	// Test non-existent email
	req2, _ := http.NewRequest("GET", "/api/v1/emails/nonexistent", nil)
	resp2, err2 := api.app.Test(req2, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err2 != nil {
		t.Fatalf("Test request failed: %v", err2)
	}
	defer func() { _ = resp2.Body.Close() }()
	body2, _ := io.ReadAll(resp2.Body)
	_ = body2

	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp2.StatusCode)
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

	req, _ := http.NewRequest("DELETE", "/api/v1/emails/test-id", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify deleted
	_, err = server.GetEmail("test-id")
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

	req, _ := http.NewRequest("DELETE", "/api/v1/emails", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("PATCH", "/api/v1/emails/test-id/read", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("PATCH", "/api/v1/emails/read", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify all read
	emails := server.GetAllEmail()
	for _, email := range emails {
		if !email.Read {
			t.Error("All emails should be marked as read")
		}
	}
}

func TestAPIReadAllEmailsReportsPersistenceFailure(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() { _ = server.Close() }()

	id := "read-failure"
	if err := os.WriteFile(filepath.Join(tmpDir, id+".eml"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(id, false, &types.Envelope{To: []string{"to@example.com"}}, &types.Email{Subject: id}); err != nil {
		t.Fatal(err)
	}
	metadataDir := filepath.Join(tmpDir, ".owlmail-meta")
	if err := os.RemoveAll(metadataDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadataDir, []byte("blocks metadata directory"), 0600); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/emails/read", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	email, err := server.GetEmail(id)
	if err != nil || email.Read {
		t.Fatalf("failed read update became visible: %#v, %v", email, err)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/stats", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/html", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type text/html; charset=utf-8, got %s", resp.Header.Get("Content-Type"))
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

	req, _ := http.NewRequest("DELETE", "/api/v1/emails/batch", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("PATCH", "/api/v1/emails/batch/read", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/attachments/test.pdf", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if string(body) != "attachment content" {
		t.Errorf("Expected attachment content, got %q", body)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/source", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("Expected Content-Type text/plain; charset=utf-8, got %s", resp.Header.Get("Content-Type"))
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/raw", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?q=Subject&limit=10&offset=0", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test with from filter
	req2, _ := http.NewRequest("GET", "/api/v1/emails?from=from1", nil)
	resp2, err2 := api.app.Test(req2, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err2 != nil {
		t.Fatalf("Test request failed: %v", err2)
	}
	defer func() { _ = resp2.Body.Close() }()
	body2, _ := io.ReadAll(resp2.Body)
	_ = body2

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp2.StatusCode)
	}

	// Test with to filter
	req3, _ := http.NewRequest("GET", "/api/v1/emails?to=to1", nil)
	resp3, err3 := api.app.Test(req3, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err3 != nil {
		t.Fatalf("Test request failed: %v", err3)
	}
	defer func() { _ = resp3.Body.Close() }()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp3.StatusCode)
	}

	// Test with read filter
	req4, _ := http.NewRequest("GET", "/api/v1/emails?read=false", nil)
	resp4, err4 := api.app.Test(req4, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err4 != nil {
		t.Fatalf("Test request failed: %v", err4)
	}
	defer func() { _ = resp4.Body.Close() }()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp4.StatusCode)
	}

	// Test with sort
	req5, _ := http.NewRequest("GET", "/api/v1/emails?sortBy=subject&sortOrder=asc", nil)
	resp5, err5 := api.app.Test(req5, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err5 != nil {
		t.Fatalf("Test request failed: %v", err5)
	}
	defer func() { _ = resp5.Body.Close() }()
	if resp5.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp5.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/preview", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestAPIReloadMailsFromDirectory(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("POST", "/api/v1/emails/reload", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("POST", "/api/v1/emails/reload", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	// Should return 500 error when reload fails
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/nonexistent/html", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/attachments/nonexistent.pdf", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestAPIDownloadEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("GET", "/api/v1/emails/nonexistent/raw", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/raw", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/test-id/raw", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?q=Subject&limit=10&offset=0", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test with from filter
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?from=from1", nil)
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ = io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test with to filter
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?to=to1", nil)
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ = io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test with read filter
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?read=false", nil)
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ = io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test with dateFrom filter
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?dateFrom="+time.Now().Add(-48*time.Hour).Format("2006-01-02"), nil)
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ = io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test with dateTo filter
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?dateTo="+time.Now().Format("2006-01-02"), nil)
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ = io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test with sortBy
	req, _ = http.NewRequest("GET", "/api/v1/emails/preview?sortBy=subject&sortOrder=asc", nil)
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ = io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?limit=2&offset=1", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?limit=invalid", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestAPIGetEmailPreviewsWithInvalidOffset(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=invalid", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestAPIGetEmailPreviewsWithLimitTooLarge(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?limit=2000", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=-1", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=100&limit=10", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=1&limit=1", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=0", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=1", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/nonexistent/source", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestAPIDeleteEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("DELETE", "/api/v1/emails/nonexistent", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestAPIReadEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("PATCH", "/api/v1/emails/nonexistent/read", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestAPIBatchDeleteEmailsInvalidRequest(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("DELETE", "/api/v1/emails/batch", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("DELETE", "/api/v1/emails/batch", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("DELETE", "/api/v1/emails/batch", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("PATCH", "/api/v1/emails/batch/read", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("PATCH", "/api/v1/emails/batch/read", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("PATCH", "/api/v1/emails/batch/read", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("PATCH", "/api/v1/emails/batch/read", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=2&offset=1", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=invalid", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should default to 50
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails?limit=2000", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should cap at 1000
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=invalid", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should default to 0
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails?offset=-1", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should default to 0
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?sortBy=subject&sortOrder=asc", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?sortBy=from&sortOrder=asc", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?sortBy=size&sortOrder=asc", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	dateFrom := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	req, _ := http.NewRequest("GET", "/api/v1/emails?dateFrom="+dateFrom, nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test dateTo filter
	dateTo := time.Now().Format("2006-01-02")
	req2, _ := http.NewRequest("GET", "/api/v1/emails?dateTo="+dateTo, nil)
	resp2, err2 := api.app.Test(req2, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err2 != nil {
		t.Fatalf("Test request failed: %v", err2)
	}
	defer func() { _ = resp2.Body.Close() }()
	body2, _ := io.ReadAll(resp2.Body)
	_ = body2

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp2.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?to=cc", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails?to=bcc", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/preview", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/preview", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/export", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("Content-Type") != "application/zip" {
		t.Errorf("Expected Content-Type application/zip, got %s", resp.Header.Get("Content-Type"))
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=id1,id2", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/export?q=Test", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestAPIExportEmailsNoEmails(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("GET", "/api/v1/emails/export", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/export", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	// Should return 200 (ZIP created with available files, missing files are skipped)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=id1,id2", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	// Should return 200 (ZIP created with available files)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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

	// Test with IDs containing spaces (use url.Values so query is properly encoded)
	params := url.Values{}
	params.Set("ids", "id1, id2 , id3")
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?"+params.Encode(), nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	// Should return 200 (spaces are trimmed)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	// Should return 200 (empty ids param means use filter, which returns all emails)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails/export?ids=nonexistent1,nonexistent2", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	// Should return 400 (no emails found)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestAPIExportEmailsRejectsOversizedSourceBeforeStreaming(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	emlPath := filepath.Join(tmpDir, "large.eml")
	file, err := os.Create(emlPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxExportBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	_ = file.Close()
	if err := server.SaveEmailToStore("large", false, &types.Envelope{To: []string{"to@example.test"}}, &types.Email{Subject: "large"}); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/emails/export?ids=large", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON preflight error", contentType)
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
	req, _ := http.NewRequest("DELETE", "/api/v1/emails", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	// Should return 200 even with no emails
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
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
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=100&limit=10", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	previews := response["previews"].([]interface{})
	if len(previews) != 0 {
		t.Errorf("Expected 0 previews (offset > total), got %d", len(previews))
	}

	// Test with offset + limit > total (end > total case)
	req2, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=1&limit=10", nil)
	resp2, err2 := api.app.Test(req2, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err2 != nil {
		t.Fatalf("Test request failed: %v", err2)
	}
	defer func() { _ = resp2.Body.Close() }()
	body2, _ := io.ReadAll(resp2.Body)
	_ = body2

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp2.StatusCode)
	}

	var response2 map[string]interface{}
	if err := json.Unmarshal(body2, &response2); err != nil {
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

	req, _ := http.NewRequest("GET", "/api/v1/emails/preview", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
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
	req, _ := http.NewRequest("GET", "/api/v1/emails/preview?offset=1&limit=1", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	_ = body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	previews := response["previews"].([]interface{})
	// When start == end, should return empty array
	if len(previews) != 0 {
		t.Errorf("Expected 0 previews (start == end), got %d", len(previews))
	}
}
