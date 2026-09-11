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

	// The router is neither strict about a trailing slash nor case sensitive, so
	// every spelling it dispatches must stay out of the wildcard CORS policy.
	for _, path := range []string{"/mcp", "/mcp/", "/MCP", "/Mcp/"} {
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

// mcpRequest issues one MCP request with an optional Origin and preflight
// headers, and returns the response for header assertions.
func mcpRequest(t *testing.T, api *API, method, path, origin string, preflightFor string) *http.Response {
	t.Helper()
	request, _ := http.NewRequest(method, path, nil)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if preflightFor != "" {
		request.Header.Set("Access-Control-Request-Method", preflightFor)
		request.Header.Set("Access-Control-Request-Headers", "content-type, mcp-protocol-version")
	}
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestMCPAllowedOriginReceivesUsableCORSHeaders(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	if err := api.SetMCPAllowedOrigins([]string{"https://inspector.example"}); err != nil {
		t.Fatal(err)
	}

	// Reaching the handler is not enough: without matching CORS response
	// headers the browser refuses to hand the response to the MCP client, which
	// would make -mcp-allowed-origins useless for the clients it exists for.
	response := mcpRequest(t, api, http.MethodPost, "/mcp", "https://inspector.example", "")
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("allowed origin status = %d, want 204", response.StatusCode)
	}
	if value := response.Header.Get("Access-Control-Allow-Origin"); value != "https://inspector.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the exact origin", value)
	}
	if value := response.Header.Get("Access-Control-Allow-Credentials"); value != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", value)
	}
	for _, header := range []string{"Mcp-Session-Id", "Mcp-Protocol-Version"} {
		if !strings.Contains(response.Header.Get("Access-Control-Expose-Headers"), header) {
			t.Fatalf("Access-Control-Expose-Headers = %q, want it to expose %s",
				response.Header.Get("Access-Control-Expose-Headers"), header)
		}
	}
	// An exact-origin policy must vary by Origin so a shared cache cannot serve
	// one origin's response to another.
	if !strings.Contains(response.Header.Get("Vary"), "Origin") {
		t.Fatalf("Vary = %q, want it to include Origin", response.Header.Get("Vary"))
	}
}

func TestMCPPreflightIsAnsweredForAllowedOriginsOnly(t *testing.T) {
	for _, credentials := range []bool{false, true} {
		user, password := "", ""
		if credentials {
			user, password = "agent", "secret"
		}
		api := newMCPOriginTestAPI(t, user, password)
		if err := api.SetMCPAllowedOrigins([]string{"https://inspector.example"}); err != nil {
			t.Fatal(err)
		}

		// A preflight carries no credentials, so Basic Auth must not answer it
		// with 401 before the endpoint's own policy replies.
		response := mcpRequest(t, api, http.MethodOptions, "/mcp", "https://inspector.example", http.MethodPost)
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("preflight status (auth=%v) = %d, want 204", credentials, response.StatusCode)
		}
		if value := response.Header.Get("Access-Control-Allow-Origin"); value != "https://inspector.example" {
			t.Fatalf("preflight Access-Control-Allow-Origin (auth=%v) = %q", credentials, value)
		}
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			if !strings.Contains(response.Header.Get("Access-Control-Allow-Methods"), method) {
				t.Fatalf("preflight Access-Control-Allow-Methods (auth=%v) = %q, want %s",
					credentials, response.Header.Get("Access-Control-Allow-Methods"), method)
			}
		}
		for _, header := range []string{"Authorization", "Content-Type", "Mcp-Session-Id", "Mcp-Protocol-Version"} {
			if !strings.Contains(response.Header.Get("Access-Control-Allow-Headers"), header) {
				t.Fatalf("preflight Access-Control-Allow-Headers (auth=%v) = %q, want %s",
					credentials, response.Header.Get("Access-Control-Allow-Headers"), header)
			}
		}

		// A preflight from any other origin is refused, and the exemption above
		// must not become an unauthenticated way through to the handler.
		response = mcpRequest(t, api, http.MethodOptions, "/mcp", "https://evil.example", http.MethodPost)
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("preflight status for a rejected origin (auth=%v) = %d, want 403", credentials, response.StatusCode)
		}
		if value := response.Header.Get("Access-Control-Allow-Origin"); value != "" {
			t.Fatalf("rejected preflight Access-Control-Allow-Origin (auth=%v) = %q, want empty", credentials, value)
		}
	}
}

func TestMCPOriginMatchingCanonicalizesDefaultPorts(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	// Browsers omit a default port when serializing Origin, so a configured
	// origin that spells it out must still match.
	if err := api.SetMCPAllowedOrigins([]string{"https://inspector.example:443", "http://plain.example:80"}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://inspector.example", "http://plain.example"} {
		if status, _ := mcpStatusForOrigin(t, api, origin); status != http.StatusNoContent {
			t.Fatalf("status for %q = %d, want 204", origin, status)
		}
	}
	// The reverse spelling matches too, and a non-default port still does not.
	if status, _ := mcpStatusForOrigin(t, api, "https://inspector.example:8443"); status != http.StatusForbidden {
		t.Fatalf("status for a non-default port = %d, want 403", status)
	}
}
