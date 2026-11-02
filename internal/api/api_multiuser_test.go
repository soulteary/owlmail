package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/identity"
	"github.com/soulteary/owlmail/internal/mailbox"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

type multiFixture struct {
	api    *API
	server *mailserver.MailServer
	store  *mailbox.Store
	boxes  map[string]string
	dir    string
}

func newMultiFixture(t *testing.T) *multiFixture {
	t.Helper()
	dir := t.TempDir()
	server, err := mailserver.NewMailServerWithOptions(0, "localhost", dir, mailserver.ServerOptions{RequireOwnership: true})
	if err != nil {
		t.Fatal(err)
	}
	store, err := mailbox.Open(filepath.Join(t.TempDir(), "control.sqlite"), true)
	if err != nil {
		t.Fatal(err)
	}
	proxy, err := identity.NewProxy("main", []string{"0.0.0.0/32"}, "X-Auth-User", "X-Auth-Email", "X-Auth-Email-Verified")
	if err != nil {
		t.Fatal(err)
	}
	api, err := NewMultiuserAPIWithHTTPS(server, 0, "localhost", false, "", "", MultiuserOptions{Proxy: proxy, Store: store, ClaimPolicy: mailbox.VerifiedEmail})
	if err != nil {
		t.Fatal(err)
	}
	f := &multiFixture{api: api, server: server, store: store, boxes: map[string]string{}, dir: dir}
	for _, user := range []string{"alice", "bob"} {
		result, err := store.BindLogin(context.Background(), mailbox.Login{Namespace: "main", Subject: user}, mailbox.VerifiedEmail)
		if err != nil {
			t.Fatal(err)
		}
		f.boxes[user] = result.MailboxID
	}
	t.Cleanup(func() { _ = server.Close(); _ = store.Close() })
	return f
}
func (f *multiFixture) mail(t *testing.T, user, id string) {
	t.Helper()
	raw := "From: sender@example.com\r\nTo: " + user + "@example.com\r\nSubject: shared\r\n\r\nsecret-" + user
	if err := os.WriteFile(filepath.Join(f.dir, id+".eml"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	ownership := &types.Ownership{MailboxID: f.boxes[user], IngestAccountID: "gorge", TransactionID: uuid.NewString(), MatchedRecipients: []string{user + "@example.com"}, RouteRevisions: []int64{1}}
	if err := f.server.SaveEmailToStore(id, false, &types.Envelope{To: []string{user + "@example.com", "other@example.com"}}, &types.Email{Subject: "shared", Text: "secret-" + user, HTML: "<b>secret-" + user + "</b>", RunID: "same", Ownership: ownership}); err != nil {
		t.Fatal(err)
	}
}
func (f *multiFixture) request(t *testing.T, user, method, path, body string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(method, "http://localhost"+path, strings.NewReader(body))
	if user != "" {
		request.Header.Set("X-Auth-User", user)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := f.api.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}
func bodyText(t *testing.T, r *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func TestMultiuserRouterScopesEveryReadableExit(t *testing.T) {
	f := newMultiFixture(t)
	f.mail(t, "alice", "alice-id")
	f.mail(t, "bob", "bob-id")
	for _, path := range []string{"/api/v1/emails", "/api/v1/emails/preview", "/api/v1/emails/stats"} {
		r := f.request(t, "alice", "GET", path, "")
		text := bodyText(t, r)
		if r.StatusCode != 200 || strings.Contains(text, "bob-id") || strings.Contains(text, "secret-bob") || strings.Contains(text, "mailDir") {
			t.Fatalf("%s leaked data: %d %s", path, r.StatusCode, text)
		}
	}
	for _, suffix := range []string{"", "/html", "/source", "/raw", "/attachments/test.txt"} {
		r := f.request(t, "alice", "GET", "/api/v1/emails/bob-id"+suffix, "")
		if r.StatusCode != 404 {
			t.Fatalf("foreign %s: %d", suffix, r.StatusCode)
		}
	}
	own := f.request(t, "alice", "GET", "/api/v1/emails/alice-id", "")
	text := bodyText(t, own)
	if own.StatusCode != 200 || strings.Contains(text, "other@example.com") || strings.Contains(text, f.dir) {
		t.Fatal("unsafe personal projection: " + text)
	}
	raw := f.request(t, "alice", "GET", "/api/v1/emails/alice-id/raw", "")
	if raw.StatusCode != 200 || !strings.Contains(bodyText(t, raw), "secret-alice") {
		t.Fatal("own raw unavailable")
	}
	for _, path := range []string{"/email", "/email/bob-id", "/config", "/reloadMailsFromDirectory", "/api/v1/ws", "/socket.io", "/mcp", "/messages", "/api/emails", "/api/v1/emails/reload", "/metrics", "/webhooks"} {
		r := f.request(t, "alice", "GET", path, "")
		if r.StatusCode != 404 {
			t.Fatalf("unexpected unscoped route %s: %d", path, r.StatusCode)
		}
	}
	settings := bodyText(t, f.request(t, "alice", "GET", "/api/v1/settings", ""))
	if !strings.Contains(settings, "multiuser") || strings.Contains(settings, f.dir) || strings.Contains(settings, "password") {
		t.Fatal("unsafe settings: " + settings)
	}
	if r := f.request(t, "", "GET", "/api/v1/emails", ""); r.StatusCode != 401 {
		t.Fatal("anonymous mailbox access")
	}
}
func TestMultiuserBatchPreflightAndPersonalCleanup(t *testing.T) {
	f := newMultiFixture(t)
	f.mail(t, "alice", "alice-id")
	f.mail(t, "bob", "bob-id")
	for _, method := range []string{"DELETE", "PATCH"} {
		path := "/api/v1/emails/batch"
		if method == "PATCH" {
			path += "/read"
		}
		r := f.request(t, "alice", method, path, `{"ids":["alice-id","bob-id"]}`)
		if r.StatusCode != 404 {
			t.Fatal("foreign batch accepted")
		}
	}
	own, err := f.server.GetEmail("alice-id")
	if err != nil || own.Read {
		t.Fatal("batch partially changed own item")
	}
	r := f.request(t, "alice", "DELETE", "/api/v1/test-runs/same/emails", "")
	if r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
	if _, err = f.server.GetEmail("alice-id"); err == nil {
		t.Fatal("own run not cleaned")
	}
	if _, err = f.server.GetEmail("bob-id"); err != nil {
		t.Fatal("foreign run cleaned")
	}
}
func TestMultiuserExportAndOpaqueCursor(t *testing.T) {
	f := newMultiFixture(t)
	f.mail(t, "alice", "alice-id")
	f.mail(t, "bob", "bob-id")
	denied := f.request(t, "alice", "GET", "/api/v1/emails/export?ids=alice-id,bob-id", "")
	if denied.StatusCode != 404 {
		t.Fatal("foreign export accepted")
	}
	exported := f.request(t, "alice", "GET", "/api/v1/emails/export", "")
	data := []byte(bodyText(t, exported))
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != "alice-id.eml" {
		t.Fatal("export not scoped")
	}
	var cursor struct {
		Cursor string `json:"cursor"`
	}
	r := f.request(t, "alice", "GET", "/api/v1/emails/cursor", "")
	if err = json.NewDecoder(r.Body).Decode(&cursor); err != nil {
		t.Fatal(err)
	}
	if cursor.Cursor == "" || strings.Contains(cursor.Cursor, ":") {
		t.Fatal("cursor exposes global sequence")
	}
	rejected := f.request(t, "bob", "POST", "/api/v1/emails/wait", fmt.Sprintf(`{"after":%q,"timeout_seconds":1}`, cursor.Cursor))
	if rejected.StatusCode != 400 {
		t.Fatal("cross-mailbox cursor accepted")
	}
	f.mail(t, "bob", "bob-new")
	f.mail(t, "alice", "alice-new")
	own := f.request(t, "alice", "POST", "/api/v1/emails/wait", fmt.Sprintf(`{"after":%q,"run_id":"same","timeout_seconds":1}`, cursor.Cursor))
	text := bodyText(t, own)
	if own.StatusCode != 200 || !strings.Contains(text, "alice-new") || strings.Contains(text, "bob-new") {
		t.Fatalf("wait isolation: %d %s", own.StatusCode, text)
	}
}
func TestMultiuserConstructorAndRebuildStayClosed(t *testing.T) {
	f := newMultiFixture(t)
	standard := NewAPI(f.server, 0, "localhost")
	request := httptest.NewRequest("GET", "http://localhost/api/v1/emails", nil)
	r, err := standard.app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Body.Close()
	if r.StatusCode != 503 {
		t.Fatal("standard constructor opened isolated store")
	}
	if err = f.api.SetBasePathname("/inbox"); err != nil {
		t.Fatal(err)
	}
	if r = f.request(t, "alice", "GET", "/inbox/api/v1/settings", ""); r.StatusCode != 200 {
		t.Fatal("route rebuild lost isolated settings")
	}
	if r = f.request(t, "alice", "GET", "/inbox/email", ""); r.StatusCode != 404 {
		t.Fatal("route rebuild added legacy routes")
	}
	if err = f.api.SetMCPHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})); err == nil {
		t.Fatal("MCP enabled")
	}
	// A missing trusted email declaration still allows a personal empty mailbox.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = f.store.PersonalMailbox(ctx, "main", "alice"); err != nil {
		t.Fatal(err)
	}
}

func TestMultiuserStatsReadAllAndClearUseCompleteOwnedSnapshot(t *testing.T) {
	f := newMultiFixture(t)
	f.mail(t, "alice", "alice-first")
	f.mail(t, "alice", "alice-second")
	f.mail(t, "bob", "bob-only")
	r := f.request(t, "alice", "GET", "/api/v1/emails/stats", "")
	var stats struct {
		Total  int `json:"total"`
		Unread int `json:"unread"`
	}
	if err := json.NewDecoder(r.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.Total != 2 || stats.Unread != 2 {
		t.Fatalf("scoped stats: %+v", stats)
	}
	r = f.request(t, "alice", "PATCH", "/api/v1/emails/read", "")
	if r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
	for _, id := range []string{"alice-first", "alice-second"} {
		e, err := f.server.GetEmail(id)
		if err != nil || !e.Read {
			t.Fatal("read-all skipped own message")
		}
	}
	bob, err := f.server.GetEmail("bob-only")
	if err != nil || bob.Read {
		t.Fatal("read-all changed foreign message")
	}
	r = f.request(t, "alice", "DELETE", "/api/v1/emails", "")
	if r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
	for _, id := range []string{"alice-first", "alice-second"} {
		if _, err = f.server.GetEmail(id); err == nil {
			t.Fatal("clear skipped own message")
		}
	}
	if _, err = f.server.GetEmail("bob-only"); err != nil {
		t.Fatal("clear removed foreign message")
	}
}

func TestIdentityMarkerPrecedesControlStoreFailures(t *testing.T) {
	f := newMultiFixture(t)
	first := f.request(t, "alice", "GET", "/api/v1/settings", "")
	marker := first.Header.Get("X-OwlMail-Identity")
	if marker == "" {
		t.Fatal("missing identity marker")
	}
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	changed := f.request(t, "bob", "GET", "/api/v1/settings", "")
	if changed.StatusCode != 503 || changed.Header.Get("X-OwlMail-Identity") == "" || changed.Header.Get("X-OwlMail-Identity") == marker {
		t.Fatal("failed new identity retained old session marker")
	}
}
