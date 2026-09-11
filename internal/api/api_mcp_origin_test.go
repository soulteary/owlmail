package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/soulteary/owlmail/internal/mailserver"
)

func newMCPOriginTestAPI(t *testing.T, user, password string) *API {
	t.Helper()
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mailbox.Close() })

	api := NewAPIWithAuth(mailbox, 1080, "localhost", user, password)
	if err := api.SetMCPHandler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})); err != nil {
		t.Fatal(err)
	}
	return api
}

func mcpStatusForOrigin(t *testing.T, api *API, origin string) (int, http.Header) {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPost, "/mcp", nil)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	return response.StatusCode, response.Header
}

func TestMCPOriginValidationAppliesWithoutBasicAuth(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")

	// A non-browser client (curl, the MCP SDK's HTTP client, another server)
	// sends no Origin and must keep working.
	if status, _ := mcpStatusForOrigin(t, api, ""); status != http.StatusNoContent {
		t.Fatalf("MCP status without Origin = %d, want 204", status)
	}

	// Any page the developer visits, including one that re-binds a hostname it
	// controls to the loopback address, must not be able to read the mailbox.
	for _, origin := range []string{
		"http://evil.example",
		"https://evil.example",
		"http://localhost:9999",
		"https://localhost:1080",
		"null",
		"http://user:pass@localhost:1080",
	} {
		if status, _ := mcpStatusForOrigin(t, api, origin); status != http.StatusForbidden {
			t.Fatalf("MCP status for Origin %q = %d, want 403", origin, status)
		}
	}

	for _, origin := range []string{
		"http://localhost:1080",
		"http://127.0.0.1:1080",
		"http://[::1]:1080",
		"HTTP://LOCALHOST:1080",
	} {
		if status, _ := mcpStatusForOrigin(t, api, origin); status != http.StatusNoContent {
			t.Fatalf("MCP status for own Origin %q = %d, want 204", origin, status)
		}
	}
}

func TestMCPEndpointNeverAdvertisesWildcardCORS(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")

	// The router is not strict about a trailing slash, so both spellings reach
	// the MCP handler and both must stay out of the wildcard CORS policy.
	for _, path := range []string{"/mcp", "/mcp/"} {
		request, _ := http.NewRequest(http.MethodPost, path, nil)
		request.Header.Set("Origin", "http://evil.example")
		response, err := api.app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("MCP status for %q = %d, want 403", path, response.StatusCode)
		}
		if value := response.Header.Get("Access-Control-Allow-Origin"); value != "" {
			t.Fatalf("MCP Access-Control-Allow-Origin for %q = %q, want empty", path, value)
		}
	}

	// The rest of the unauthenticated API keeps its open development CORS.
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/emails", nil)
	request.Header.Set("Origin", "http://evil.example")
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if value := response.Header.Get("Access-Control-Allow-Origin"); value != "*" {
		t.Fatalf("versioned API Access-Control-Allow-Origin = %q, want *", value)
	}
}

func TestMCPOriginValidationAppliesWithBasicAuth(t *testing.T) {
	api := newMCPOriginTestAPI(t, "agent", "secret")

	request, _ := http.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set("Origin", "http://evil.example")
	request.SetBasicAuth("agent", "secret")
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("authenticated cross-origin MCP status = %d, want 403", response.StatusCode)
	}
}

func TestMCPAllowedOriginsSurviveBasicAuthSameOriginMiddleware(t *testing.T) {
	api := newMCPOriginTestAPI(t, "agent", "secret")
	if err := api.SetMCPAllowedOrigins([]string{"https://inspector.example"}); err != nil {
		t.Fatal(err)
	}

	// The global same-origin middleware would reject this before the guard is
	// consulted, making -mcp-allowed-origins inert on authenticated deployments.
	request, _ := http.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set("Origin", "https://inspector.example")
	request.SetBasicAuth("agent", "secret")
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("allowed cross-origin MCP status with auth = %d, want 204", response.StatusCode)
	}

	// Every other route keeps the same-origin middleware it had.
	request, _ = http.NewRequest(http.MethodGet, "/api/v1/emails", nil)
	request.Header.Set("Origin", "https://inspector.example")
	request.SetBasicAuth("agent", "secret")
	response, err = api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("versioned API cross-origin status = %d, want 403", response.StatusCode)
	}
}

func TestMCPOriginAllowListAcceptsBracketedIPv6WebHost(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()

	// net.JoinHostPort adds its own brackets, so a bracketed listen address must
	// not derive "http://[[2001:db8::1]]:1080" and reject the browser's real
	// origin. A non-loopback address is used deliberately: ::1 would be covered
	// by the loopback entries even if the derivation were wrong.
	api := NewAPI(mailbox, 1080, "[2001:db8::1]")
	allowed := api.mcpOriginAllowList()
	if !originAllowed("http://[2001:db8::1]:1080", allowed) {
		t.Fatalf("derived allow list rejected its own origin: %v", allowed)
	}
	for _, origin := range allowed {
		if strings.Contains(origin, "[[") {
			t.Fatalf("derived allow list double-bracketed a host: %v", allowed)
		}
	}
}

func TestMCPOriginAllowListHasNoDuplicates(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()

	api := NewAPI(mailbox, 1080, "localhost")
	seen := make(map[string]struct{})
	for _, origin := range api.mcpOriginAllowList() {
		if _, exists := seen[origin]; exists {
			t.Fatalf("derived allow list repeats %q: %v", origin, api.mcpOriginAllowList())
		}
		seen[origin] = struct{}{}
	}
}

func TestMCPAllowedOriginsAcceptsConfiguredBrowserOrigins(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	if err := api.SetMCPAllowedOrigins([]string{"https://inspector.example"}); err != nil {
		t.Fatal(err)
	}

	if status, _ := mcpStatusForOrigin(t, api, "https://inspector.example"); status != http.StatusNoContent {
		t.Fatalf("configured Origin status = %d, want 204", status)
	}
	if status, _ := mcpStatusForOrigin(t, api, "https://other.example"); status != http.StatusForbidden {
		t.Fatalf("unconfigured Origin status = %d, want 403", status)
	}
	// Configured extras add to, and never replace, OwlMail's own origins.
	if status, _ := mcpStatusForOrigin(t, api, "http://localhost:1080"); status != http.StatusNoContent {
		t.Fatalf("own Origin status = %d, want 204", status)
	}

	if err := api.SetMCPAllowedOrigins([]string{"not-an-origin"}); err == nil {
		t.Fatal("SetMCPAllowedOrigins accepted a value that is not an origin")
	}
}

func TestMCPAllowedOriginsWildcardDisablesValidation(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	if err := api.SetMCPAllowedOrigins([]string{"*"}); err != nil {
		t.Fatal(err)
	}
	if status, _ := mcpStatusForOrigin(t, api, "http://evil.example"); status != http.StatusNoContent {
		t.Fatalf("wildcard Origin status = %d, want 204", status)
	}
}

func TestMCPOriginAllowListCoversDefaultPortsAndWildcardBinds(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()

	api := NewAPI(mailbox, 80, "0.0.0.0")
	allowed := api.mcpOriginAllowList()

	// A wildcard bind names no browser-visible host, so only the loopback
	// spellings are derived, in both port-less and explicit-port form.
	for _, origin := range []string{"http://localhost", "http://localhost:80", "http://127.0.0.1", "http://[::1]:80"} {
		if !originAllowed(origin, allowed) {
			t.Fatalf("derived allow list rejected %q", origin)
		}
	}
	if originAllowed("http://0.0.0.0:80", allowed) {
		t.Fatal("derived allow list accepted the wildcard bind address as an origin")
	}
}
