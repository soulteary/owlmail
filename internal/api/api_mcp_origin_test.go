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

// credentialedMCPRequest issues an authenticated cross-origin MCP request. The
// credentials matter: without them Basic Auth answers 401 before the endpoint's
// own policy runs, and an assertion about CORS headers would pass vacuously.
func credentialedMCPRequest(t *testing.T, api *API, origin string) *http.Response {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPost, "/mcp", nil)
	request.Header.Set("Origin", origin)
	request.SetBasicAuth("agent", "secret")
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("credentialed request for %q reached status %d, want 204 from the stub handler", origin, response.StatusCode)
	}
	return response
}

func TestMCPWildcardOptOutNeverGrantsCredentialedCORS(t *testing.T) {
	api := newMCPOriginTestAPI(t, "agent", "secret")
	if err := api.SetMCPAllowedOrigins([]string{"*"}); err != nil {
		t.Fatal(err)
	}

	// Turning validation off must not be an upgrade on the wildcard CORS this
	// endpoint used to fall under. Echoing the caller with credentials would
	// hand every site a credentialed grant, which a wildcard cannot carry: the
	// opt-out would then be a bigger hole than the one origin validation closes.
	response := credentialedMCPRequest(t, api, "https://evil.example")
	if value := response.Header.Get("Access-Control-Allow-Origin"); value != "*" {
		t.Fatalf("wildcard Access-Control-Allow-Origin = %q, want *", value)
	}
	if value := response.Header.Get("Access-Control-Allow-Credentials"); value != "" {
		t.Fatalf("wildcard Access-Control-Allow-Credentials = %q, want empty", value)
	}

	// A named origin still gets the stronger, exact-origin grant.
	named := newMCPOriginTestAPI(t, "agent", "secret")
	if err := named.SetMCPAllowedOrigins([]string{"https://inspector.example"}); err != nil {
		t.Fatal(err)
	}
	response = credentialedMCPRequest(t, named, "https://inspector.example")
	if value := response.Header.Get("Access-Control-Allow-Origin"); value != "https://inspector.example" {
		t.Fatalf("named Access-Control-Allow-Origin = %q", value)
	}
	if value := response.Header.Get("Access-Control-Allow-Credentials"); value != "true" {
		t.Fatalf("named Access-Control-Allow-Credentials = %q, want true", value)
	}
}

func TestMCPResponsesAlwaysVaryByOrigin(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")

	// A refusal and a pass-through depend on the Origin header just as much as
	// an allowed response does, so a shared cache must not reuse either.
	for _, test := range []struct {
		name   string
		origin string
		status int
	}{
		{name: "rejected", origin: "https://evil.example", status: http.StatusForbidden},
		{name: "non-browser", origin: "", status: http.StatusNoContent},
		{name: "own origin", origin: "http://localhost:1080", status: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := mcpRequest(t, api, http.MethodPost, "/mcp", test.origin, "")
			if response.StatusCode != test.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.status)
			}
			if !strings.Contains(response.Header.Get("Vary"), "Origin") {
				t.Fatalf("Vary = %q, want it to include Origin", response.Header.Get("Vary"))
			}
		})
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

func TestMCPOriginMatchingCanonicalizesIPv6Literals(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()

	// A browser serializes an IPv6 origin in its compressed form, so a listen
	// address written out in full must still match its own origin.
	api := NewAPI(mailbox, 1080, "[2001:0db8::1]")
	if !originAllowed("http://[2001:db8::1]:1080", api.mcpOriginAllowList()) {
		t.Fatalf("derived allow list rejected its own compressed origin: %v", api.mcpOriginAllowList())
	}

	// The same holds for a configured entry, in either direction and either case.
	configured := NewAPIWithAuth(mailbox, 1080, "localhost", "", "")
	if err := configured.SetMCPAllowedOrigins([]string{"https://[2001:0DB8::1]"}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://[2001:db8::1]", "https://[2001:0db8:0000::1]"} {
		if !originAllowed(origin, configured.mcpOriginAllowList()) {
			t.Fatalf("configured allow list rejected %q: %v", origin, configured.mcpOriginAllowList())
		}
	}
	// A different address is still a different origin.
	if originAllowed("https://[2001:db8::2]", configured.mcpOriginAllowList()) {
		t.Fatal("configured allow list accepted an unrelated IPv6 address")
	}
	// An IPv4 host is left exactly as written rather than rewritten.
	if !originAllowed("http://127.0.0.1:1080", api.mcpOriginAllowList()) {
		t.Fatalf("derived allow list rejected the loopback IPv4 origin: %v", api.mcpOriginAllowList())
	}
}

func TestMCPWildcardOptOutAcceptsOpaqueBrowserOrigins(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	if err := api.SetMCPAllowedOrigins([]string{"*"}); err != nil {
		t.Fatal(err)
	}

	// A page from a local file, a data URL, or a sandboxed document sends
	// "null". Turning validation off has to cover those too, or the opt-out
	// does not opt out; recognizing the wildcard inside originAllowed put it
	// behind a parse gate that refused such values first.
	for _, origin := range []string{"null", "http://evil.example"} {
		if status, _ := mcpStatusForOrigin(t, api, origin); status != http.StatusNoContent {
			t.Fatalf("wildcard status for Origin %q = %d, want 204", origin, status)
		}
	}

	// Without the opt-out, "null" stays refused.
	strict := newMCPOriginTestAPI(t, "", "")
	if status, _ := mcpStatusForOrigin(t, strict, "null"); status != http.StatusForbidden {
		t.Fatalf("strict status for Origin null = %d, want 403", status)
	}
}

func TestMCPOriginMatchingCanonicalizesInternationalizedHostnames(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")

	// A browser serializes a Unicode domain in its IDNA ASCII form, so an
	// operator who configures the Unicode spelling must still match the origin
	// the browser actually sends.
	if err := api.SetMCPAllowedOrigins([]string{"https://例え.テスト"}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://xn--r8jz45g.xn--zckzah", "https://例え.テスト"} {
		if status, _ := mcpStatusForOrigin(t, api, origin); status != http.StatusNoContent {
			t.Fatalf("status for %q = %d, want 204", origin, status)
		}
	}

	// Configuring the ASCII form matches both spellings too.
	ascii := newMCPOriginTestAPI(t, "", "")
	if err := ascii.SetMCPAllowedOrigins([]string{"https://xn--r8jz45g.xn--zckzah"}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://例え.テスト", "https://xn--r8jz45g.xn--zckzah"} {
		if status, _ := mcpStatusForOrigin(t, ascii, origin); status != http.StatusNoContent {
			t.Fatalf("ascii-configured status for %q = %d, want 204", origin, status)
		}
	}
	// A different domain is still a different origin.
	if status, _ := mcpStatusForOrigin(t, ascii, "https://evil.example"); status != http.StatusForbidden {
		t.Fatalf("status for an unrelated origin = %d, want 403", status)
	}
}

func TestMCPOriginMatchingKeepsHostsTheIDNAProfileRejects(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	// An underscore label is refused by the IDNA lookup profile but occurs in
	// container and CI hostnames. Such a host must keep the spelling it was
	// given rather than being dropped, so nothing that matches today stops
	// matching to gain IDNA support.
	if err := api.SetMCPAllowedOrigins([]string{"http://under_score.example:8080"}); err != nil {
		t.Fatal(err)
	}
	if status, _ := mcpStatusForOrigin(t, api, "http://under_score.example:8080"); status != http.StatusNoContent {
		t.Fatalf("status for an underscore host = %d, want 204", status)
	}
	// IP origins keep working alongside it.
	if status, _ := mcpStatusForOrigin(t, api, "http://127.0.0.1:1080"); status != http.StatusNoContent {
		t.Fatalf("status for the loopback origin = %d, want 204", status)
	}
}

func TestMCPAllowedOriginsAccessorReportsTheComparedForm(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	if err := api.SetMCPAllowedOrigins([]string{"https://例え.テスト", "https://Inspector.Example:443"}); err != nil {
		t.Fatal(err)
	}

	// Startup logs this so an operator can compare it against a refused
	// request. It has to be the form a browser sends, not the configured
	// spelling, which may be percent-encoded or carry a default port.
	origins := api.MCPAllowedOrigins()
	want := []string{"https://xn--r8jz45g.xn--zckzah", "https://inspector.example"}
	if len(origins) != len(want) {
		t.Fatalf("MCPAllowedOrigins() = %v, want %v", origins, want)
	}
	for index, origin := range origins {
		if origin != want[index] {
			t.Fatalf("MCPAllowedOrigins() = %v, want %v", origins, want)
		}
	}

	// The caller gets a copy; mutating it must not change the policy.
	origins[0] = "https://evil.example"
	if status, _ := mcpStatusForOrigin(t, api, "https://evil.example"); status != http.StatusForbidden {
		t.Fatal("mutating the returned slice changed the allow list")
	}
}

func TestMCPAllowedOriginsRejectsMixedWildcardLists(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	if err := api.SetMCPAllowedOrigins([]string{"https://inspector.example"}); err != nil {
		t.Fatal(err)
	}

	// The wildcard turns validation off for everything, so pairing it with a
	// named origin silently opens a list meant to stay narrow. The setter is
	// reachable without the configuration parser that refuses this, so it has
	// to refuse it too -- and must not half-apply the bad list.
	if err := api.SetMCPAllowedOrigins([]string{"*", "https://inspector.example"}); err == nil {
		t.Fatal("SetMCPAllowedOrigins accepted a wildcard combined with an explicit origin")
	}
	if status, _ := mcpStatusForOrigin(t, api, "https://evil.example"); status != http.StatusForbidden {
		t.Fatalf("a refused list still widened the policy: status = %d, want 403", status)
	}
	if status, _ := mcpStatusForOrigin(t, api, "https://inspector.example"); status != http.StatusNoContent {
		t.Fatalf("a refused list disturbed the previous policy: status = %d, want 204", status)
	}

	// The wildcard on its own, in any spelling, still opts out.
	if err := api.SetMCPAllowedOrigins([]string{"*", "", "  "}); err != nil {
		t.Fatalf("SetMCPAllowedOrigins rejected a lone wildcard: %v", err)
	}
	if status, _ := mcpStatusForOrigin(t, api, "https://evil.example"); status != http.StatusNoContent {
		t.Fatalf("lone wildcard status = %d, want 204", status)
	}
}

func TestMCPOriginAllowListUsesTheListenerScheme(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	// TLS terminated at a reverse proxy: the browser-visible scheme is https,
	// but this listener still answers plain HTTP on the loopback. Deriving the
	// listener's own origins from the external scheme locks a browser out of
	// the listener it is actually talking to.
	if err := api.SetExternalScheme("https"); err != nil {
		t.Fatal(err)
	}
	if err := api.SetMCPAllowedOrigins([]string{"https://mail.example"}); err != nil {
		t.Fatal(err)
	}
	if status, _ := mcpStatusForOrigin(t, api, "http://localhost:1080"); status != http.StatusNoContent {
		t.Fatalf("direct listener origin status = %d, want 204", status)
	}
	// The browser-visible origin still works, from configuration.
	if status, _ := mcpStatusForOrigin(t, api, "https://mail.example"); status != http.StatusNoContent {
		t.Fatalf("external origin status = %d, want 204", status)
	}
	// The scheme still has to match: an https loopback origin is not this
	// listener's, and is not silently accepted for being "close enough".
	if status, _ := mcpStatusForOrigin(t, api, "https://localhost:1080"); status != http.StatusForbidden {
		t.Fatalf("mismatched-scheme loopback status = %d, want 403", status)
	}
}

func TestMCPOriginAllowListFollowsAnHTTPSListener(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()

	api := NewAPIWithHTTPS(mailbox, 1080, "localhost", "", "", true, "cert.pem", "key.pem")
	if !originAllowed("https://localhost:1080", api.mcpOriginAllowList()) {
		t.Fatalf("HTTPS listener rejected its own origin: %v", api.mcpOriginAllowList())
	}
	if originAllowed("http://localhost:1080", api.mcpOriginAllowList()) {
		t.Fatalf("HTTPS listener accepted a plain HTTP origin: %v", api.mcpOriginAllowList())
	}
}

func TestMCPOriginMatchingCanonicalizesNumericPorts(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	// A browser renders a port as a number, so a zero-padded spelling has to
	// reduce before the default-port rule is applied.
	if err := api.SetMCPAllowedOrigins([]string{"https://padded.example:0443", "https://other.example:08443"}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://padded.example", "https://padded.example:443", "https://other.example:8443"} {
		if status, _ := mcpStatusForOrigin(t, api, origin); status != http.StatusNoContent {
			t.Fatalf("status for %q = %d, want 204", origin, status)
		}
	}
	// A genuinely different port is still a different origin.
	if status, _ := mcpStatusForOrigin(t, api, "https://other.example:8444"); status != http.StatusForbidden {
		t.Fatalf("status for a different port = %d, want 403", status)
	}
}

func TestMCPOriginMatchingCanonicalizesIPv4MappedLiterals(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	// "::ffff:192.0.2.1" and "::ffff:c000:201" are one address written two
	// ways, and a browser picks the hex form. Both sides of the comparison are
	// canonicalized, so either spelling may be configured.
	if err := api.SetMCPAllowedOrigins([]string{"https://[::ffff:192.0.2.1]"}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://[::ffff:c000:201]", "https://[::ffff:192.0.2.1]"} {
		if status, _ := mcpStatusForOrigin(t, api, origin); status != http.StatusNoContent {
			t.Fatalf("status for %q = %d, want 204", origin, status)
		}
	}
	// The dotted form is a different origin to a browser -- an IPv4 host rather
	// than an IPv6 one -- so configuring the mapped literal must not also admit
	// it. Rendering the mapped address in dotted form would merge the two.
	if status, _ := mcpStatusForOrigin(t, api, "https://192.0.2.1"); status != http.StatusForbidden {
		t.Fatalf("the plain IPv4 origin was admitted by a mapped IPv6 entry: status = %d, want 403", status)
	}
	if status, _ := mcpStatusForOrigin(t, api, "https://192.0.2.2"); status != http.StatusForbidden {
		t.Fatalf("status for a different address = %d, want 403", status)
	}
	// Configuring the dotted form admits it, and not the mapped literal.
	dotted := newMCPOriginTestAPI(t, "", "")
	if err := dotted.SetMCPAllowedOrigins([]string{"https://192.0.2.1"}); err != nil {
		t.Fatal(err)
	}
	if status, _ := mcpStatusForOrigin(t, dotted, "https://192.0.2.1"); status != http.StatusNoContent {
		t.Fatalf("dotted origin status = %d, want 204", status)
	}
	if status, _ := mcpStatusForOrigin(t, dotted, "https://[::ffff:c000:201]"); status != http.StatusForbidden {
		t.Fatalf("mapped literal admitted by a dotted entry: status = %d, want 403", status)
	}
	// A plain IPv4 origin is unchanged by the canonicalization.
	plain := newMCPOriginTestAPI(t, "", "")
	if status, _ := mcpStatusForOrigin(t, plain, "http://127.0.0.1:1080"); status != http.StatusNoContent {
		t.Fatalf("loopback IPv4 status = %d, want 204", status)
	}
}

func TestMCPOriginMatchingDoesNotWidenOnUnicodeCaseMapping(t *testing.T) {
	api := newMCPOriginTestAPI(t, "", "")
	// strings.ToLower uses simple, per-rune case mapping, which collapses
	// "İ" (U+0130) to a plain "i". Applying it before IDNA would store this
	// origin as "https://i.com": the intended browser, which sends
	// "xn--i-9bb.com", would be refused, and an unrelated domain anyone can
	// register would be accepted in its place. The widening is the point of
	// this test, not the missed match.
	if err := api.SetMCPAllowedOrigins([]string{"https://İ.com"}); err != nil {
		t.Fatal(err)
	}
	if got := api.MCPAllowedOrigins(); len(got) != 1 || got[0] != "https://xn--i-9bb.com" {
		t.Fatalf("MCPAllowedOrigins() = %v, want [https://xn--i-9bb.com]", got)
	}
	if status, _ := mcpStatusForOrigin(t, api, "https://xn--i-9bb.com"); status != http.StatusNoContent {
		t.Fatalf("status for the browser's spelling = %d, want 204", status)
	}
	if status, _ := mcpStatusForOrigin(t, api, "https://i.com"); status != http.StatusForbidden {
		t.Fatalf("an unrelated domain was accepted: status = %d, want 403", status)
	}

	// Ordinary ASCII hosts are still matched case-insensitively, which the IDNA
	// profile does itself.
	mixed := newMCPOriginTestAPI(t, "", "")
	if err := mixed.SetMCPAllowedOrigins([]string{"https://Inspector.EXAMPLE"}); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://inspector.example", "https://INSPECTOR.example"} {
		if status, _ := mcpStatusForOrigin(t, mixed, origin); status != http.StatusNoContent {
			t.Fatalf("status for %q = %d, want 204", origin, status)
		}
	}

	// An upper-case IP literal and a host the IDNA profile rejects both still
	// compare case-insensitively, since each branch lower-cases its own output.
	if !originAllowed("http://[2001:0DB8::1]:1080", []string{"http://[2001:db8::1]:1080"}) {
		t.Fatal("upper-case IPv6 literal did not match its canonical form")
	}
	rejected := newMCPOriginTestAPI(t, "", "")
	if err := rejected.SetMCPAllowedOrigins([]string{"http://Under_Score.example:8080"}); err != nil {
		t.Fatal(err)
	}
	if status, _ := mcpStatusForOrigin(t, rejected, "http://under_score.example:8080"); status != http.StatusNoContent {
		t.Fatalf("status for an underscore host = %d, want 204", status)
	}
}

func TestMCPOriginAllowListDerivesListenHostsWithoutPreLowering(t *testing.T) {
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mailbox.Close() }()

	// The listen address takes the same path as a configured origin, so it must
	// not be lower-cased before IDNA either: "İ" collapses to a plain "i" under
	// simple case mapping, which would derive an origin for a domain the
	// operator never named.
	api := NewAPI(mailbox, 1080, "İ.example")
	allowed := api.mcpOriginAllowList()
	if !originAllowed("http://xn--i-9bb.example:1080", allowed) {
		t.Fatalf("derived allow list rejected the browser's spelling: %v", allowed)
	}
	if originAllowed("http://i.example:1080", allowed) {
		t.Fatalf("derived allow list accepted an unrelated domain: %v", allowed)
	}

	// Ordinary mixed-case and bracketed spellings still derive correctly.
	mixed := NewAPI(mailbox, 1080, "LocalHost")
	if !originAllowed("http://localhost:1080", mixed.mcpOriginAllowList()) {
		t.Fatalf("mixed-case listen host rejected its own origin: %v", mixed.mcpOriginAllowList())
	}
	bracketed := NewAPI(mailbox, 1080, "[2001:0DB8::1]")
	if !originAllowed("http://[2001:db8::1]:1080", bracketed.mcpOriginAllowList()) {
		t.Fatalf("bracketed upper-case IPv6 rejected its own origin: %v", bracketed.mcpOriginAllowList())
	}
	// A wildcard bind is still recognized whatever its case.
	for _, host := range []string{"0.0.0.0", "::", "[::]"} {
		if !isWildcardBindHost(host) {
			t.Fatalf("isWildcardBindHost(%q) = false", host)
		}
	}
}
