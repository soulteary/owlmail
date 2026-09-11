package api

import (
	"net/http"
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

	_, header := mcpStatusForOrigin(t, api, "http://evil.example")
	if value := header.Get("Access-Control-Allow-Origin"); value != "" {
		t.Fatalf("MCP Access-Control-Allow-Origin = %q, want empty", value)
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
