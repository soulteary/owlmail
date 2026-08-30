package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestCorsMiddleware(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// CORS middleware adds headers to actual requests; with AllowOriginsFunc
	// returning true it sets Access-Control-Allow-Origin to the request origin.
	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	req.Header.Set("Origin", "http://example.com")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin != "*" && allowOrigin != "http://example.com" {
		t.Errorf("CORS Access-Control-Allow-Origin should be set, got %q", allowOrigin)
	}
}

func TestAuthenticatedAPIRejectsCrossOriginBrowserRequest(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	req, _ := http.NewRequest(http.MethodGet, "http://owlmail.test/api/v1/emails", nil)
	req.Header.Set("Origin", "https://attacker.example")
	req.SetBasicAuth("user", "pass")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", origin)
	}
}

func TestAuthenticatedAPIRejectsCrossSchemeBrowserRequest(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	req, _ := http.NewRequest(http.MethodGet, "http://owlmail.test/api/v1/emails", nil)
	req.Header.Set("Origin", "https://owlmail.test")
	req.SetBasicAuth("user", "pass")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAuthenticatedAPIAllowsSameOriginAndNonBrowserRequests(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	for _, origin := range []string{"http://owlmail.test", ""} {
		req, _ := http.NewRequest(http.MethodGet, "http://owlmail.test/api/v1/emails", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.SetBasicAuth("user", "pass")
		resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("origin %q: status = %d, want %d", origin, resp.StatusCode, http.StatusOK)
		}
	}
}

func TestAuthenticatedWebSocketOriginPolicy(t *testing.T) {
	server, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	tests := []struct {
		origin string
		want   bool
	}{
		{origin: "", want: true},
		{origin: "http://owlmail.test", want: true},
		{origin: "https://owlmail.test", want: false},
		{origin: "https://attacker.example", want: false},
		{origin: "://invalid", want: false},
	}
	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, "http://owlmail.test/api/v1/ws", nil)
		if test.origin != "" {
			req.Header.Set("Origin", test.origin)
		}
		if got := api.wsUpgrader.CheckOrigin(req); got != test.want {
			t.Errorf("origin %q: CheckOrigin() = %v, want %v", test.origin, got, test.want)
		}
	}
}

func TestBasicAuthMiddleware(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestHelpPageRequiresBasicAuth(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	req, _ := http.NewRequest(http.MethodGet, "/help", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected help page status 401, got %d", resp.StatusCode)
	}
}

func TestBasicAuthMiddlewareSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	req.SetBasicAuth("user", "pass")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestBasicAuthMiddlewareInvalidPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestBasicAuthMiddlewareInvalidBase64(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	req.Header.Set("Authorization", "Basic invalid-base64!")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestBasicAuthMiddlewareInvalidCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	req.SetBasicAuth("wronguser", "wrongpass")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestBasicAuthMiddlewareInvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")

	req, _ := http.NewRequest("GET", "/api/v1/emails", nil)
	req.Header.Set("Authorization", "Basic dXNlcg==") // base64("user")
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}
}

func TestHealthCheckSkippedAuth(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()
	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHealthzSkippedAuth(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()
	api := NewAPIWithAuth(server, 1080, "localhost", "user", "pass")
	req, _ := http.NewRequest("GET", "/healthz", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
