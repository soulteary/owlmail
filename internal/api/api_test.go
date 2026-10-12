package api

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

func setupTestAPI(t *testing.T) (*API, *mailserver.MailServer, string) {
	tmpDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	api := NewAPI(server, 1080, "localhost")
	return api, server, tmpDir
}

func TestNewAPI(t *testing.T) {
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

	api := NewAPI(server, 1080, "localhost")
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
	if api.authUser != "user" {
		t.Errorf("Expected auth user 'user', got '%s'", api.authUser)
	}
	if api.authPassword != "pass" {
		t.Errorf("Expected auth password 'pass', got '%s'", api.authPassword)
	}
}

func TestNewAPIWithHTTPS(t *testing.T) {
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

	api := NewAPIWithHTTPS(server, 1080, "localhost", "user", "pass", true, "cert.pem", "key.pem")
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

func TestAPIStartWithReadyDoesNotSignalOnBindFailure(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	port := occupied.Addr().(*net.TCPAddr).Port

	server, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPIWithHTTPSDeferredRecovery(server, port, "127.0.0.1", "", "", false, "", "")
	ready := false
	if err := api.StartWithReady(func() { ready = true }); err == nil {
		t.Fatal("StartWithReady succeeded on an occupied address")
	}
	if ready {
		t.Fatal("ready callback ran before the API listener was bound")
	}
}

func TestAPIHealthCheck(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", response["status"])
	}
	if response["service"] != "owlmail" {
		t.Errorf("Expected service 'owlmail', got '%v'", response["service"])
	}
}

type staticReadinessProvider struct {
	status attachmentstore.HealthStatus
}

func (provider staticReadinessProvider) Snapshot() attachmentstore.HealthStatus {
	return provider.status
}

func TestAPIReadinessUsesCachedStoreHealthWithoutAffectingLiveness(t *testing.T) {
	server, err := mailserver.NewMailServerWithOptions(1025, "localhost", t.TempDir(), mailserver.ServerOptions{
		AttachmentHealth: staticReadinessProvider{status: attachmentstore.HealthStatus{
			State: attachmentstore.HealthUnready, ErrorCategory: attachmentstore.HealthErrorPermission,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPI(server, 1080, "localhost")

	for _, path := range []string{"/readyz", "/api/v1/ready"} {
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		resp, requestErr := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d, body = %s", path, resp.StatusCode, body)
		}
		if !strings.Contains(string(body), `"error_category":"permission"`) || strings.Contains(string(body), "secret") || strings.Contains(string(body), "endpoint") || strings.Contains(string(body), "token") {
			t.Fatalf("unsafe or incomplete readiness response: %s", body)
		}
	}

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("liveness status = %d", resp.StatusCode)
	}
}

func TestAPISetupEventListeners(t *testing.T) {
	api, server, tmpDir := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	api.mailServer.On("new", func(email *types.Email) {})

	email := &types.Email{ID: "test-id", Subject: "Test", Time: time.Now()}
	envelope := &types.Envelope{From: "from@example.com", To: []string{"to@example.com"}}
	emlPath := filepath.Join(tmpDir, "test-id.eml")
	if err := os.WriteFile(emlPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create email file: %v", err)
	}
	if err := server.SaveEmailToStore("test-id", false, envelope, email); err != nil {
		t.Fatalf("Failed to save email: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if api.mailServer == nil {
		t.Error("Mail server should be set")
	}
}

func TestAPISetupRoutes(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	if api.app == nil {
		t.Error("App should be set up")
	}

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if api.app == nil {
		t.Error("App should be configured")
	}

	req2, _ := http.NewRequest("GET", "/some-page", nil)
	_, _ = api.app.Test(req2, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})

	req3, _ := http.NewRequest("GET", "/api/v1/health", nil)
	resp3, err := api.app.Test(req3, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("Test request failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("API route should work, got status %d", resp3.StatusCode)
	}

	testCases := []string{"/email", "/config", "/healthz", "/socket.io", "/api/", "/style.css", "/app.js", "/webhooks", "/webhooks.css", "/webhooks.js"}
	for _, path := range testCases {
		req, _ := http.NewRequest("GET", path, nil)
		_, _ = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	}
}

func TestEmbeddedWebAssets(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Failed to close server: %v", err)
		}
	}()

	// Embedded assets must not depend on the process being started from the
	// repository root (for example, after `go install`).
	t.Chdir(t.TempDir())

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html", contains: `href="/help"`},
		{path: "/some-page", contentType: "text/html", contains: "OwlMail"},
		{path: "/help", contentType: "text/html", contains: "OwlMail Help"},
		{path: "/help/", contentType: "text/html", contains: "快速上手 OwlMail"},
		{path: "/style.css", contentType: "text/css", contains: ".header"},
		{path: "/app.js", contentType: "text/javascript", contains: "connectWebSocket"},
		{path: "/service-worker.js", contentType: "text/javascript", contains: "notificationclick"},
		{path: "/help.css", contentType: "text/css", contains: ".help-shell"},
		{path: "/help.js", contentType: "text/javascript", contains: "applyLanguage"},
		{path: "/webhooks", contentType: "text/html", contains: "Webhook Configurator"},
		{path: "/webhooks/", contentType: "text/html", contains: `id="targetList"`},
		{path: "/webhooks.css", contentType: "text/css", contains: ".webhook-workspace"},
		{path: "/webhooks.js", contentType: "text/javascript", contains: "parseConfigText"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.path, nil)
			if err != nil {
				t.Fatalf("NewRequest(%q) error: %v", tt.path, err)
			}
			resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatalf("GET %s failed: %v", tt.path, err)
			}
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				t.Fatalf("reading %s response: %v", tt.path, readErr)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tt.path, resp.StatusCode, http.StatusOK)
			}
			if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, tt.contentType) {
				t.Errorf("GET %s Content-Type = %q, want prefix %q", tt.path, got, tt.contentType)
			}
			if !strings.Contains(string(body), tt.contains) {
				t.Errorf("GET %s body does not contain %q", tt.path, tt.contains)
			}
		})
	}
}

func TestBasePathRouting(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		prefix string
	}{
		{name: "root", input: "/", prefix: ""},
		{name: "subpath", input: "/owlmail/", prefix: "/owlmail"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, server, _ := setupTestAPI(t)
			defer func() { _ = server.Close() }()
			if err := api.SetBasePathname(tt.input); err != nil {
				t.Fatal(err)
			}
			if tt.prefix != "" {
				for _, redirect := range []struct {
					request string
					want    string
				}{
					{request: tt.prefix, want: tt.prefix + "/"},
					{request: tt.prefix + "?email=message%2Fid&tab=html", want: tt.prefix + "/?email=message%2Fid&tab=html"},
				} {
					req, _ := http.NewRequest(http.MethodGet, redirect.request, nil)
					resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
					if err != nil {
						t.Fatal(err)
					}
					_ = resp.Body.Close()
					if resp.StatusCode != http.StatusPermanentRedirect || resp.Header.Get("Location") != redirect.want {
						t.Errorf("GET %s redirect = (%d, %q), want (308, %q)", redirect.request, resp.StatusCode, resp.Header.Get("Location"), redirect.want)
					}
				}
			}

			for _, route := range []string{
				"/", "/help", "/webhooks", "/style.css", "/app.js",
				"/favicon.svg", "/api/v1/health", "/healthz", "/config",
			} {
				req, _ := http.NewRequest(http.MethodGet, tt.prefix+route, nil)
				resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
				if err != nil {
					t.Fatalf("GET %s: %v", tt.prefix+route, err)
				}
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Errorf("GET %s status = %d, want 200", tt.prefix+route, resp.StatusCode)
				}
			}

			req, _ := http.NewRequest(http.MethodGet, tt.prefix+"/", nil)
			resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			for _, asset := range []string{"/style.css", "/app.js", "/favicon.svg", "/help", "/webhooks"} {
				if !strings.Contains(string(body), tt.prefix+asset) {
					t.Errorf("index for %q does not reference %q", tt.prefix, tt.prefix+asset)
				}
			}

			req, _ = http.NewRequest(http.MethodGet, tt.prefix+"/service-worker.js", nil)
			resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if got := resp.Header.Get("Service-Worker-Allowed"); got != tt.prefix+"/" {
				t.Errorf("Service-Worker-Allowed = %q, want %q", got, tt.prefix+"/")
			}

			for _, route := range []string{"/api/v1/ws", "/socket.io", "/api/v1/emails/missing/attachments/file.txt"} {
				req, _ = http.NewRequest(http.MethodGet, tt.prefix+route, nil)
				resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
				if err != nil {
					t.Fatal(err)
				}
				body, _ = io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusNotFound && (route == "/api/v1/ws" || route == "/socket.io") {
					t.Errorf("WebSocket route %s was not registered", tt.prefix+route)
				}
				if strings.Contains(string(body), "Email Development Testing Tool") {
					t.Errorf("GET %s unexpectedly fell through to the browser UI", tt.prefix+route)
				}
			}

			if tt.prefix != "" {
				req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
				resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
				if err != nil {
					t.Fatal(err)
				}
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Errorf("image health check status = %d, want 200", resp.StatusCode)
				}

				for _, route := range []string{"/", "/api/v1/health", "/style.css"} {
					req, _ = http.NewRequest(http.MethodGet, route, nil)
					resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
					if err != nil {
						t.Fatal(err)
					}
					_ = resp.Body.Close()
					if resp.StatusCode != http.StatusNotFound {
						t.Errorf("out-of-scope GET %s status = %d, want 404", route, resp.StatusCode)
					}
				}
			}
		})
	}
}

func TestBasePathHealthzKeepsImageHealthCheckReachable(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	if err := api.SetBasePathname("/healthz"); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("GET /healthz returned non-health response: %s", body)
	}
}

func TestEscapedBasePathRouting(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	if err := api.SetBasePathname("/team%20mail"); err != nil {
		t.Fatal(err)
	}

	for _, route := range []string{"/team%20mail/", "/team%20mail/style.css", "/team%20mail/api/v1/health"} {
		req, _ := http.NewRequest(http.MethodGet, route, nil)
		resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("GET %s failed: %v", route, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", route, resp.StatusCode)
		}
	}
}

func TestAPIStart(t *testing.T) {
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

	api := NewAPI(server, 0, "localhost")
	if api == nil {
		t.Fatal("NewAPI should not return nil")
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- api.Start()
	}()

	time.Sleep(100 * time.Millisecond)

	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("Server start error (expected in some cases): %v", err)
		}
	default:
	}

	apiHTTPS := NewAPIWithHTTPS(server, 0, "localhost", "", "", true, "nonexistent.pem", "nonexistent.key")
	errChan2 := make(chan error, 1)
	go func() {
		errChan2 <- apiHTTPS.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-errChan2:
		if err == nil {
			t.Error("Expected error when cert files don't exist")
		}
	default:
		t.Error("Expected error when cert files don't exist")
	}

	apiHTTPS2 := NewAPIWithHTTPS(server, 0, "localhost", "", "", true, "", "key.pem")
	errChan3 := make(chan error, 1)
	go func() {
		errChan3 <- apiHTTPS2.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-errChan3:
		if err == nil {
			t.Error("Expected error when cert file is empty")
		}
	default:
		t.Error("Expected error when cert file is empty")
	}

	apiHTTPS3 := NewAPIWithHTTPS(server, 0, "localhost", "", "", true, "cert.pem", "")
	errChan4 := make(chan error, 1)
	go func() {
		errChan4 <- apiHTTPS3.Start()
	}()

	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-errChan4:
		if err == nil {
			t.Error("Expected error when key file is empty")
		}
	default:
		t.Error("Expected error when key file is empty")
	}
}
