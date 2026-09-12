package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func newWebOriginTestAPI(t *testing.T, user, password string) *API {
	t.Helper()
	mailbox, err := mailserver.NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mailbox.Close() })
	return NewAPIWithAuth(mailbox, 1080, "localhost", user, password)
}

// webRequest issues one browser-shaped request against the Web API. The target
// carries an absolute URL so the same-origin comparison has a Host to work
// with, which is what distinguishes OwlMail's own page from every other one.
func webRequest(t *testing.T, api *API, method, target, origin string, credentials bool) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if credentials {
		request.SetBasicAuth("user", "pass")
	}
	response, err := api.app.Test(request, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestWebOriginValidationAppliesWithoutBasicAuth(t *testing.T) {
	api := newWebOriginTestAPI(t, "", "")

	// Basic Auth is off by default, so this is the default deployment. Every one
	// of these requests used to be answered with Access-Control-Allow-Origin: *,
	// which let any page the developer visited read the captured mailbox,
	// repoint the outgoing relay, forward mail, and empty the mailbox.
	for _, test := range []struct {
		name   string
		method string
		target string
	}{
		{name: "read the mailbox", method: http.MethodGet, target: "http://owlmail.test/api/v1/emails"},
		{name: "read the outgoing relay", method: http.MethodGet, target: "http://owlmail.test/api/v1/settings/outgoing"},
		{name: "empty the mailbox", method: http.MethodDelete, target: "http://owlmail.test/api/v1/emails"},
		{name: "historical route", method: http.MethodGet, target: "http://owlmail.test/email"},
		{name: "browser UI", method: http.MethodGet, target: "http://owlmail.test/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := webRequest(t, api, test.method, test.target, "https://evil.example", false)
			if response.StatusCode != http.StatusForbidden {
				t.Fatalf("cross-origin status = %d, want 403", response.StatusCode)
			}
			if value := response.Header.Get("Access-Control-Allow-Origin"); value != "" {
				t.Fatalf("Access-Control-Allow-Origin = %q, want empty", value)
			}
		})
	}
}

func TestWebOriginValidationAllowsOwnOrigin(t *testing.T) {
	api := newWebOriginTestAPI(t, "", "")

	for _, origin := range []string{
		"http://owlmail.test",
		"HTTP://OWLMAIL.TEST",
		"http://localhost:1080",
		"http://127.0.0.1:1080",
	} {
		response := webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", origin, false)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("own Origin %q status = %d, want 200", origin, response.StatusCode)
		}
	}

	// A page served over https by a host that spells its name the same way is a
	// different origin, and so is one on another port.
	for _, origin := range []string{
		"https://owlmail.test",
		"http://owlmail.test:8080",
		"null",
		"http://user:pass@owlmail.test",
	} {
		response := webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", origin, false)
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("Origin %q status = %d, want 403", origin, response.StatusCode)
		}
	}
}

// TestWebOriginValidationAllowsClientsWithoutAnOrigin pins the compatibility
// contract that makes validation safe to turn on by default: curl, HTTP
// libraries, CI scripts and server-to-server callers never send Origin, and
// only a browser page does. If this breaks, the change is not a hardening but
// an outage.
func TestWebOriginValidationAllowsClientsWithoutAnOrigin(t *testing.T) {
	for _, auth := range []struct {
		name        string
		user        string
		password    string
		credentials bool
	}{
		{name: "unauthenticated", user: "", password: "", credentials: false},
		{name: "basic auth", user: "user", password: "pass", credentials: true},
	} {
		t.Run(auth.name, func(t *testing.T) {
			api := newWebOriginTestAPI(t, auth.user, auth.password)
			for _, target := range []string{
				"http://owlmail.test/api/v1/emails",
				"http://owlmail.test/api/v1/emails/stats",
				"http://owlmail.test/email",
				"http://owlmail.test/",
			} {
				response := webRequest(t, api, http.MethodGet, target, "", auth.credentials)
				if response.StatusCode != http.StatusOK {
					t.Fatalf("%s without Origin status = %d, want 200", target, response.StatusCode)
				}
			}
		})
	}
}

// TestWebOriginValidationLeavesHealthEndpointsReachable keeps the probes that
// gate a container restart out of the policy's way; a health checker sends no
// Origin and must not start failing because a browser rule was added.
func TestWebOriginValidationLeavesHealthEndpointsReachable(t *testing.T) {
	for _, auth := range []struct {
		name     string
		user     string
		password string
	}{
		{name: "unauthenticated", user: "", password: ""},
		{name: "basic auth", user: "user", password: "pass"},
	} {
		t.Run(auth.name, func(t *testing.T) {
			api := newWebOriginTestAPI(t, auth.user, auth.password)
			for _, target := range []string{
				"http://owlmail.test/healthz",
				"http://owlmail.test/readyz",
				"http://owlmail.test/api/v1/health",
				"http://owlmail.test/api/v1/ready",
			} {
				// No credentials are sent: health routes are exempt from Basic
				// Auth, and the origin guard must not reintroduce a gate there.
				response := webRequest(t, api, http.MethodGet, target, "", false)
				if response.StatusCode != http.StatusOK {
					t.Fatalf("%s status = %d, want 200", target, response.StatusCode)
				}
			}
		})
	}
}

func TestWebAllowedOriginsAcceptsConfiguredBrowserOrigins(t *testing.T) {
	api := newWebOriginTestAPI(t, "", "")
	if err := api.SetWebAllowedOrigins([]string{"https://console.example"}); err != nil {
		t.Fatal(err)
	}

	// An allowed origin needs the response headers that let the browser hand the
	// body to the page. Without them the operator's own allow list would answer
	// 200 and still show an empty response in the console.
	response := webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", "https://console.example", false)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("configured Origin status = %d, want 200", response.StatusCode)
	}
	if value := response.Header.Get("Access-Control-Allow-Origin"); value != "https://console.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the exact origin", value)
	}
	if value := response.Header.Get("Access-Control-Allow-Credentials"); value != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want true", value)
	}
	if value := response.Header.Get("Vary"); value != "Origin" {
		t.Fatalf("Vary = %q, want Origin", value)
	}

	response = webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", "https://other.example", false)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("unconfigured Origin status = %d, want 403", response.StatusCode)
	}

	// A configured extra adds to, and never replaces, OwlMail's own origin.
	response = webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", "http://owlmail.test", false)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("own Origin status = %d, want 200", response.StatusCode)
	}
	// Same-origin needs no CORS grant, and issuing one would widen nothing while
	// suggesting the policy had.
	if value := response.Header.Get("Access-Control-Allow-Origin"); value != "" {
		t.Fatalf("same-origin Access-Control-Allow-Origin = %q, want empty", value)
	}

	if err := api.SetWebAllowedOrigins([]string{"not-an-origin"}); err == nil {
		t.Fatal("SetWebAllowedOrigins accepted a value that is not an origin")
	}
}

func TestWebAllowedOriginsCanonicalizeLikeTheMCPList(t *testing.T) {
	api := newWebOriginTestAPI(t, "", "")
	// Default ports, zero padding, equivalent IP spellings and IDN all go
	// through one canonicalizer, so a value that works for -mcp-allowed-origins
	// works here too.
	if err := api.SetWebAllowedOrigins([]string{"https://Console.Example:443", "https://例え.テスト", "http://[2001:0DB8::1]:08080"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"https://console.example", "https://xn--r8jz45g.xn--zckzah", "http://[2001:db8::1]:8080"}
	origins := api.WebAllowedOrigins()
	if len(origins) != len(want) {
		t.Fatalf("WebAllowedOrigins() = %v, want %v", origins, want)
	}
	for index, origin := range origins {
		if origin != want[index] {
			t.Fatalf("WebAllowedOrigins() = %v, want %v", origins, want)
		}
	}
	for _, origin := range []string{"https://console.example", "https://例え.テスト", "http://[2001:db8:0:0::1]:8080"} {
		response := webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", origin, false)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("equivalent spelling %q status = %d, want 200", origin, response.StatusCode)
		}
	}
}

func TestWebAllowedOriginsWildcardRestoresTheOpenPolicy(t *testing.T) {
	api := newWebOriginTestAPI(t, "", "")
	if err := api.SetWebAllowedOrigins([]string{"*"}); err != nil {
		t.Fatal(err)
	}

	response := webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", "https://evil.example", false)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("wildcard Origin status = %d, want 200", response.StatusCode)
	}
	// No origin has been vouched for under the opt-out, so the grant stays the
	// weaker uncredentialed wildcard rather than an echo a browser may use with
	// credentials.
	if value := response.Header.Get("Access-Control-Allow-Origin"); value != "*" {
		t.Fatalf("wildcard Access-Control-Allow-Origin = %q, want *", value)
	}
	if value := response.Header.Get("Access-Control-Allow-Credentials"); value != "" {
		t.Fatalf("wildcard Access-Control-Allow-Credentials = %q, want empty", value)
	}
}

func TestWebAllowedOriginsRejectsMixedWildcardLists(t *testing.T) {
	api := newWebOriginTestAPI(t, "", "")
	if err := api.SetWebAllowedOrigins([]string{"https://console.example"}); err != nil {
		t.Fatal(err)
	}

	// Pairing the opt-out with a named origin can only be a mistake, and it is
	// exactly the mistake that silently opens a list its author meant to keep
	// narrow. The refusal must leave the previous list in place rather than
	// applying half of the new one.
	if err := api.SetWebAllowedOrigins([]string{"*", "https://console.example"}); err == nil {
		t.Fatal("SetWebAllowedOrigins accepted a wildcard combined with an origin")
	}
	if api.webAllowsAnyOrigin() {
		t.Fatal("a refused allow list disabled origin validation")
	}
	response := webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", "https://evil.example", false)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin status after a refused list = %d, want 403", response.StatusCode)
	}
}

func TestWebOriginGuardAnswersPreflight(t *testing.T) {
	for _, test := range []struct {
		name    string
		allowed []string
		origin  string
		status  int
		allow   string
	}{
		{name: "configured origin", allowed: []string{"https://console.example"}, origin: "https://console.example", status: http.StatusNoContent, allow: "https://console.example"},
		{name: "unknown origin", allowed: nil, origin: "https://evil.example", status: http.StatusForbidden, allow: ""},
		{name: "wildcard opt-out", allowed: []string{"*"}, origin: "https://evil.example", status: http.StatusNoContent, allow: "*"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Basic Auth is configured deliberately: a preflight carries no
			// credentials, so a 401 here would stop an allowed browser client
			// from ever issuing the real request.
			api := newWebOriginTestAPI(t, "user", "pass")
			if test.allowed != nil {
				if err := api.SetWebAllowedOrigins(test.allowed); err != nil {
					t.Fatal(err)
				}
			}
			request, err := http.NewRequest(http.MethodOptions, "http://owlmail.test/api/v1/settings/outgoing", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Access-Control-Request-Method", http.MethodPut)
			request.Header.Set("Access-Control-Request-Headers", "Content-Type")
			response, err := api.app.Test(request, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()

			if response.StatusCode != test.status {
				t.Fatalf("preflight status = %d, want %d", response.StatusCode, test.status)
			}
			if value := response.Header.Get("Access-Control-Allow-Origin"); value != test.allow {
				t.Fatalf("preflight Access-Control-Allow-Origin = %q, want %q", value, test.allow)
			}
			if test.status != http.StatusNoContent {
				return
			}
			if value := response.Header.Get("Access-Control-Allow-Methods"); value != webCORSAllowedMethods {
				t.Fatalf("preflight Access-Control-Allow-Methods = %q", value)
			}
			if value := response.Header.Get("Access-Control-Allow-Headers"); value != webCORSAllowedHeaders {
				t.Fatalf("preflight Access-Control-Allow-Headers = %q", value)
			}
		})
	}
}

func TestWebOriginGuardLeavesMCPToItsOwnPolicy(t *testing.T) {
	api := newWebOriginTestAPI(t, "", "")
	if err := api.SetMCPHandler(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})); err != nil {
		t.Fatal(err)
	}
	if err := api.SetMCPAllowedOrigins([]string{"https://inspector.example"}); err != nil {
		t.Fatal(err)
	}

	// The MCP allow list is the operator's decision about /mcp. A Web guard that
	// answered first would overrule it, which is the mistake the Basic Auth
	// branch made before this policy became the default.
	response := webRequest(t, api, http.MethodPost, "http://owlmail.test/mcp", "https://inspector.example", false)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("MCP status for its own allowed origin = %d, want 204", response.StatusCode)
	}

	// The reverse holds too: a Web allow list does not open /mcp.
	if err := api.SetWebAllowedOrigins([]string{"https://console.example"}); err != nil {
		t.Fatal(err)
	}
	response = webRequest(t, api, http.MethodPost, "http://owlmail.test/mcp", "https://console.example", false)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("MCP status for a Web-only origin = %d, want 403", response.StatusCode)
	}
	response = webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", "https://console.example", false)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Web status for its own allowed origin = %d, want 200", response.StatusCode)
	}
}

func TestWebOriginGuardKeepsBasicAuthBehavior(t *testing.T) {
	api := newWebOriginTestAPI(t, "user", "pass")

	for _, test := range []struct {
		name        string
		origin      string
		credentials bool
		status      int
	}{
		{name: "cross-origin with credentials", origin: "https://evil.example", credentials: true, status: http.StatusForbidden},
		{name: "same-origin without credentials", origin: "http://owlmail.test", credentials: false, status: http.StatusUnauthorized},
		{name: "same-origin with credentials", origin: "http://owlmail.test", credentials: true, status: http.StatusOK},
		{name: "no Origin without credentials", origin: "", credentials: false, status: http.StatusUnauthorized},
		{name: "no Origin with credentials", origin: "", credentials: true, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := webRequest(t, api, http.MethodGet, "http://owlmail.test/api/v1/emails", test.origin, test.credentials)
			if response.StatusCode != test.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.status)
			}
		})
	}
}

func TestWebSocketOriginPolicyDoesNotDependOnBasicAuth(t *testing.T) {
	// A WebSocket upgrade is not governed by CORS, so this handshake check is
	// the only thing keeping a page the developer visits off the live mail
	// stream, which carries the same bodies the REST API returns.
	api := newWebOriginTestAPI(t, "", "")
	for _, test := range []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "non-browser client", origin: "", want: true},
		{name: "same origin", origin: "http://owlmail.test", want: true},
		{name: "own listener origin", origin: "http://localhost:1080", want: true},
		{name: "scheme mismatch", origin: "https://owlmail.test", want: false},
		{name: "attacker page", origin: "https://evil.example", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://owlmail.test/api/v1/ws", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if got := api.wsUpgrader.CheckOrigin(request); got != test.want {
				t.Fatalf("CheckOrigin() = %v, want %v", got, test.want)
			}
		})
	}

	allowed := newWebOriginTestAPI(t, "", "")
	if err := allowed.SetWebAllowedOrigins([]string{"https://console.example"}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://owlmail.test/api/v1/ws", nil)
	request.Header.Set("Origin", "https://console.example")
	if !allowed.wsUpgrader.CheckOrigin(request) {
		t.Fatal("configured browser origin was refused the WebSocket upgrade")
	}

	wildcard := newWebOriginTestAPI(t, "", "")
	if err := wildcard.SetWebAllowedOrigins([]string{"*"}); err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "http://owlmail.test/api/v1/ws", nil)
	request.Header.Set("Origin", "https://evil.example")
	if !wildcard.wsUpgrader.CheckOrigin(request) {
		t.Fatal("the documented opt-out did not disable WebSocket origin validation")
	}
}

// TestWebOriginGuardExposesRetryAfterToAllowedOrigins pins the difference
// between a response a browser receives and one it can read. Retry-After is not
// CORS-safelisted, so without an explicit expose header an operator-allowed
// client gets the 429 or 503 body telling it to back off and no way to read how
// long to wait for.
func TestWebOriginGuardExposesRetryAfterToAllowedOrigins(t *testing.T) {
	api := newWebOriginTestAPI(t, "", "")
	if err := api.SetWebAllowedOrigins([]string{"https://console.example"}); err != nil {
		t.Fatal(err)
	}

	request, _ := http.NewRequest(http.MethodGet, "/api/v1/emails", nil)
	request.Header.Set("Origin", "https://console.example")
	response, err := api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()

	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "https://console.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the named origin", got)
	}
	if got := response.Header.Get("Access-Control-Expose-Headers"); !strings.Contains(got, "Retry-After") {
		t.Fatalf("Access-Control-Expose-Headers = %q, want it to name Retry-After", got)
	}
}
