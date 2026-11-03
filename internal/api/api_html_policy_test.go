package api

import (
	"net/http"
	"strings"
	"testing"
)

func assertEmailHTMLPolicy(t *testing.T, response *http.Response) {
	t.Helper()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTML status %d", response.StatusCode)
	}
	policy := response.Header.Get("Content-Security-Policy")
	if !strings.Contains(policy, "default-src 'none'") || !strings.HasPrefix(policy, "sandbox allow-popups allow-popups-to-escape-sandbox allow-downloads;") || strings.Contains(policy, "https:") || strings.Contains(policy, "http:") {
		t.Fatalf("HTML response permits remote or unsandboxed content: %q", policy)
	}
	if response.Header.Get("Referrer-Policy") != "no-referrer" || response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("HTML response missing resource headers: %v", response.Header)
	}
}

func TestMultiuserDirectHTMLPolicy(t *testing.T) {
	f := newMultiFixture(t)
	f.mail(t, "alice", "html-policy")
	response := f.request(t, "alice", "GET", "/api/v1/emails/html-policy/html", "")
	assertEmailHTMLPolicy(t, response)
	if body := bodyText(t, response); !strings.Contains(body, "secret-alice") {
		t.Fatalf("HTML body was changed: %q", body)
	}
	foreign := f.request(t, "bob", "GET", "/api/v1/emails/html-policy/html", "")
	if foreign.StatusCode != http.StatusNotFound {
		t.Fatalf("foreign HTML status %d", foreign.StatusCode)
	}
}
