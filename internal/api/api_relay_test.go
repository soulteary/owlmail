package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/types"
)

func TestAPIRelayEmail(t *testing.T) {
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

	// Test relay with query parameter
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay?relayTo=relay@example.com", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should return 200 or 400 depending on relay configuration
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailWithBody(t *testing.T) {
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

	// Test relay with body parameter
	requestBody := map[string]interface{}{
		"relayTo": "relay@example.com",
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should return 200 or 400 depending on relay configuration
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailWithoutRelayTo(t *testing.T) {
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

	// Test relay without relayTo (uses configured SMTP server)
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should return 200 or 400 depending on relay configuration
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("POST", "/api/v1/emails/nonexistent/actions/relay", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailWithParam(t *testing.T) {
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

	// Test relay with param
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay/relay@example.com", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should return 200 or 400 depending on relay configuration
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailWithParamEmpty(t *testing.T) {
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

	// Test relay with empty param (using empty string as param)
	// Use a route that will have empty relayTo param
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay/ ", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// The route might redirect or return 400, both are acceptable
	// The important thing is that it doesn't succeed with empty param
	if resp.StatusCode == http.StatusOK {
		t.Errorf("Expected status not 200 for empty param, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailWithParamNotFound(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("POST", "/api/v1/emails/nonexistent/actions/relay/relay@example.com", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailWithBodyButNoRelayTo(t *testing.T) {
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

	// Test relay with body but no relayTo field
	requestBody := map[string]interface{}{
		"other": "value",
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should return 200 or 400 depending on relay configuration
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailWithInvalidBody(t *testing.T) {
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

	// Test relay with invalid JSON body
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should return 200 or 400 depending on relay configuration (invalid JSON is ignored)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", resp.StatusCode)
	}
}

func TestAPIRelayEmailWithParamEmptyString(t *testing.T) {
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

	// Test relay with empty string param (using empty string as param value)
	// Note: Fiber router may redirect trailing slashes, so we test with actual empty param
	// Use a route that will have empty relayTo param - need to use a different approach
	req, _ := http.NewRequest("POST", "/api/v1/emails/test-id/actions/relay/%20", nil) // space character
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should return 400 for empty/invalid email address
	// Note: The route might redirect or return different status, but validation should catch it
	if resp.StatusCode == http.StatusOK {
		t.Errorf("Expected status not 200 for empty/invalid email address, got %d", resp.StatusCode)
	}
}
