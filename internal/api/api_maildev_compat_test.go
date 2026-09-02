package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

// These assertions mirror MailDev's current REST contract documented at
// https://github.com/maildev/maildev/blob/main/docs/rest.md and implemented by
// packages/api/src/server.ts. Socket.IO is deliberately outside this facade.
func TestMailDevRESTFacadeDisabledByDefault(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/email"},
		{http.MethodGet, "/api/email/summary"},
		{http.MethodGet, "/api/email/missing"},
		{http.MethodDelete, "/api/email/missing"},
		{http.MethodPost, "/api/email/delete"},
		{http.MethodDelete, "/api/email/all"},
		{http.MethodPatch, "/api/email/read-all"},
		{http.MethodGet, "/api/email/missing/html"},
		{http.MethodGet, "/api/email/missing/source"},
		{http.MethodGet, "/api/email/missing/download"},
		{http.MethodGet, "/api/email/missing/attachment/file.txt"},
		{http.MethodPost, "/api/email/missing/relay"},
		{http.MethodPost, "/api/email/missing/relay/to@example.com"},
		{http.MethodGet, "/api/config"},
		{http.MethodGet, "/api/healthz"},
		{http.MethodGet, "/api/reloadMailsFromDirectory"},
	}
	for _, route := range routes {
		req, _ := http.NewRequest(route.method, route.path, nil)
		resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
		if err != nil {
			t.Fatalf("%s %s: %v", route.method, route.path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want 404 while compatibility is disabled", route.method, route.path, resp.StatusCode)
		}
	}
}

func TestMailDevRESTFacadeContractAndReadSideEffect(t *testing.T) {
	api, server, mailDir := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	api.SetMailDevRESTCompat(true)

	received := time.Date(2026, time.January, 5, 19, 2, 9, 0, time.UTC)
	email := &types.Email{
		ID: "mail-1", Time: received, Read: false, Subject: "The ex-presidents are surfers",
		Text: "The wax at the bank was surfer wax!!!",
		HTML: `<p>The wax</p><img src="cid:logo@example.test">`,
		Size: 1024, SizeHuman: "1 KB",
		From: []*mail.Address{{Name: "Angelo Pappas", Address: "angelo.pappas@fbi.gov"}},
		To:   []*mail.Address{{Name: "Johnny Utah", Address: "johnny.utah@fbi.gov"}},
		Headers: map[string]interface{}{
			"Content-Type": "multipart/mixed", "Date": "Sun, 05 Jan 2026 19:02:09 +0000",
		},
		Attachments: []*types.Attachment{{
			FileName: "logo.png", GeneratedFileName: "safe-logo.png", ContentType: "image/png",
			ContentDisposition: "inline", ContentID: "logo@example.test", Size: 4,
		}},
	}
	envelope := &types.Envelope{
		From: "angelo.pappas@fbi.gov", To: []string{"johnny.utah@fbi.gov"},
		Host: "mail.test", RemoteAddress: "127.0.0.1:1234",
	}
	writeCompatFixture(t, server, mailDir, email, envelope, []byte("logo"))

	// OwlMail's native detail routes retain their explicit-read contract.
	for _, path := range []string{"/api/v1/emails/mail-1", "/email/mail-1"} {
		resp := compatRequest(t, api, http.MethodGet, path, nil, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d", path, resp.StatusCode)
		}
		_ = resp.Body.Close()
		stored, err := server.GetEmail("mail-1")
		if err != nil || stored.Read {
			t.Fatalf("native route %s changed read state: %#v, %v", path, stored, err)
		}
	}

	resp := compatRequest(t, api, http.MethodGet, "/api/email", nil, "")
	assertContentType(t, resp, "application/json")
	var list []map[string]interface{}
	decodeCompatJSON(t, resp, &list)
	if len(list) != 1 || list[0]["id"] != "mail-1" {
		t.Fatalf("unexpected MailDev list: %#v", list)
	}
	attachment := list[0]["attachments"].([]interface{})[0].(map[string]interface{})
	if attachment["filename"] != "logo.png" || attachment["contentDisposition"] != "inline" {
		t.Fatalf("unexpected attachment DTO: %#v", attachment)
	}
	if _, leaked := attachment["fileName"]; leaked {
		t.Fatalf("native attachment key leaked into facade: %#v", attachment)
	}

	// Arbitrary MailDev field filters must not traverse Go implementation
	// details such as time.Time's unexported fields or panic the server.
	resp = compatRequest(t, api, http.MethodGet, "/api/email?time.wall=0", nil, "")
	var rejectedTraversal []map[string]interface{}
	decodeCompatJSON(t, resp, &rejectedTraversal)
	if len(rejectedTraversal) != 0 {
		t.Fatalf("unexported field traversal unexpectedly matched: %#v", rejectedTraversal)
	}

	resp = compatRequest(t, api, http.MethodGet, "/api/email/summary?skip=0&limit=50&search=angelo&sort=desc&unread=true", nil, "")
	var summary struct {
		Items      []map[string]interface{} `json:"items"`
		Total      int                      `json:"total"`
		StoreTotal int                      `json:"storeTotal"`
		Unread     int                      `json:"unread"`
		Skip       int                      `json:"skip"`
		Limit      int                      `json:"limit"`
	}
	decodeCompatJSON(t, resp, &summary)
	if len(summary.Items) != 1 || summary.Total != 1 || summary.StoreTotal != 1 || summary.Unread != 1 || summary.Limit != 50 {
		t.Fatalf("unexpected MailDev summary page: %#v", summary)
	}
	for _, forbidden := range []string{"html", "text", "headers"} {
		if _, exists := summary.Items[0][forbidden]; exists {
			t.Fatalf("summary contains %s: %#v", forbidden, summary.Items[0])
		}
	}

	resp = compatRequest(t, api, http.MethodGet, "/api/email/mail-1", nil, "")
	var detail map[string]interface{}
	decodeCompatJSON(t, resp, &detail)
	if detail["read"] != true {
		t.Fatalf("compat detail did not return persisted read state: %#v", detail)
	}
	stored, err := server.GetEmail("mail-1")
	if err != nil || !stored.Read {
		t.Fatalf("compat detail did not mark stored email read: %#v, %v", stored, err)
	}

	resp = compatRequest(t, api, http.MethodGet, "/api/email/mail-1/html", nil, "")
	assertContentType(t, resp, "text/html")
	htmlBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(htmlBody), `src="/api/email/mail-1/attachment/safe-logo.png"`) {
		t.Fatalf("HTML did not embed facade attachment URL: %s", htmlBody)
	}

	resp = compatRequest(t, api, http.MethodGet, "/api/email/mail-1/source", nil, "")
	assertContentType(t, resp, "application/octet-stream")
	_ = resp.Body.Close()
	resp = compatRequest(t, api, http.MethodGet, "/api/email/mail-1/download", nil, "")
	assertContentType(t, resp, "message/rfc822")
	if got := resp.Header.Get("Content-Disposition"); got != "attachment; filename=mail-1.eml" {
		t.Fatalf("Content-Disposition = %q", got)
	}
	_ = resp.Body.Close()
	resp = compatRequest(t, api, http.MethodGet, "/api/email/mail-1/attachment/safe-logo.png", nil, "")
	assertContentType(t, resp, "image/png")
	attachmentBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(attachmentBody) != "logo" {
		t.Fatalf("attachment body = %q", attachmentBody)
	}

	resp = compatRequest(t, api, http.MethodGet, "/api/config", nil, "")
	var config map[string]interface{}
	decodeCompatJSON(t, resp, &config)
	if len(config) != 4 || config["smtpPort"] != float64(1025) || config["isOutgoingEnabled"] != false || config["outgoingHost"] != nil {
		t.Fatalf("unexpected config DTO: %#v", config)
	}
	resp = compatRequest(t, api, http.MethodGet, "/api/healthz", nil, "")
	var healthy bool
	decodeCompatJSON(t, resp, &healthy)
	if !healthy {
		t.Fatal("MailDev health response was not true")
	}
}

func TestMailDevRESTFacadeOperationsAndErrors(t *testing.T) {
	api, server, mailDir := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	api.SetMailDevRESTCompat(true)
	for _, id := range []string{"one", "two", "three"} {
		writeCompatFixture(t, server, mailDir, &types.Email{ID: id, Subject: id, Time: time.Now()}, &types.Envelope{}, nil)
	}

	resp := compatRequest(t, api, http.MethodPost, "/api/email/delete", []byte(`{"ids":["one","two","one","missing"]}`), "application/json")
	var deleted struct {
		Deleted []string `json:"deleted"`
		Missing []string `json:"notFound"`
	}
	decodeCompatJSON(t, resp, &deleted)
	if strings.Join(deleted.Deleted, ",") != "one,two" || strings.Join(deleted.Missing, ",") != "missing" {
		t.Fatalf("unexpected bulk-delete response: %#v", deleted)
	}

	resp = compatRequest(t, api, http.MethodPost, "/api/email/delete", []byte(`{"ids":"wrong"}`), "application/json")
	assertMailDevError(t, resp, http.StatusBadRequest, "Request body must include an ids array of email IDs")
	resp = compatRequest(t, api, http.MethodDelete, "/api/email/missing", nil, "")
	assertMailDevError(t, resp, http.StatusNotFound, "Email was not found")
	resp = compatRequest(t, api, http.MethodPost, "/api/email/three/relay", nil, "")
	assertMailDevError(t, resp, http.StatusInternalServerError, "Outgoing mail not configured")
	server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "smtp.example", Port: 25})
	resp = compatRequest(t, api, http.MethodPost, "/api/email/three/relay/not-an-address", nil, "")
	assertMailDevError(t, resp, http.StatusBadRequest, "Incorrect email address provided: not-an-address")

	resp = compatRequest(t, api, http.MethodPatch, "/api/email/read-all", nil, "")
	var count int
	decodeCompatJSON(t, resp, &count)
	if count != 1 {
		t.Fatalf("read-all count = %d, want 1", count)
	}
	resp = compatRequest(t, api, http.MethodGet, "/api/reloadMailsFromDirectory", nil, "")
	var reloaded bool
	decodeCompatJSON(t, resp, &reloaded)
	if !reloaded {
		t.Fatal("reload response was not true")
	}
	resp = compatRequest(t, api, http.MethodDelete, "/api/email/all", nil, "")
	var allDeleted bool
	decodeCompatJSON(t, resp, &allDeleted)
	if !allDeleted || len(server.GetAllEmail()) != 0 {
		t.Fatalf("delete-all response/state = %t/%d", allDeleted, len(server.GetAllEmail()))
	}
}

func TestMailDevRESTFacadeUsesBasePathAndBasicAuth(t *testing.T) {
	mailDir := t.TempDir()
	server, err := mailserver.NewMailServer(1025, "localhost", mailDir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPIWithAuth(server, 1080, "localhost", "admin", "secret")
	api.SetMailDevRESTCompat(true)
	if err := api.SetBasePathname("/owlmail"); err != nil {
		t.Fatal(err)
	}

	credentials := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req, _ := http.NewRequest(http.MethodGet, "/api/email", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unprefixed facade status = %d, want 404", resp.StatusCode)
	}
	_ = resp.Body.Close()
	resp = compatRequest(t, api, http.MethodGet, "/owlmail/api/email", nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated facade status = %d, want 401", resp.StatusCode)
	}
	_ = resp.Body.Close()
	req, _ = http.NewRequest(http.MethodGet, "/owlmail/api/email", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	resp, err = api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated facade status = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// MailDev's health route remains public, at the configured base path only.
	resp = compatRequest(t, api, http.MethodGet, "/owlmail/api/healthz", nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("compat health status = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestMailDevRESTListFiltersSortsAndPaginatesBeforeProjection(t *testing.T) {
	api, server, mailDir := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	api.SetMailDevRESTCompat(true)

	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	fixtures := []struct {
		id   string
		from string
		time time.Time
	}{
		{id: "old-alice", from: "alice@example.test", time: base},
		{id: "bob", from: "bob@example.test", time: base.Add(time.Hour)},
		{id: "new-alice", from: "alice@example.test", time: base.Add(2 * time.Hour)},
	}
	for _, fixture := range fixtures {
		email := &types.Email{
			ID: fixture.id, Time: fixture.time, Subject: fixture.id,
			From: []*mail.Address{{Address: fixture.from}},
			To:   []*mail.Address{{Address: "recipient@example.test"}},
		}
		writeCompatFixture(t, server, mailDir, email, &types.Envelope{
			From: fixture.from, To: []string{"recipient@example.test"},
		}, nil)
	}

	resp := compatRequest(t, api, http.MethodGet, "/api/email?from.address=alice@example.test&sort=desc&skip=1&limit=1", nil, "")
	var page []map[string]interface{}
	decodeCompatJSON(t, resp, &page)
	if len(page) != 1 || page[0]["id"] != "old-alice" {
		t.Fatalf("filtered page = %#v", page)
	}
}

func writeCompatFixture(t *testing.T, server *mailserver.MailServer, mailDir string, email *types.Email, envelope *types.Envelope, attachment []byte) {
	t.Helper()
	raw := []byte("From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: " + email.Subject + "\r\n\r\nbody")
	if err := os.WriteFile(filepath.Join(mailDir, email.ID+".eml"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if len(email.Attachments) > 0 {
		dir := filepath.Join(mailDir, email.ID)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, email.Attachments[0].GeneratedFileName), attachment, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := server.SaveEmailToStore(email.ID, email.Read, envelope, email); err != nil {
		t.Fatal(err)
	}
}

func compatRequest(t *testing.T, api *API, method, path string, body []byte, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeCompatJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	assertContentType(t, resp, "application/json")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
}

func assertContentType(t *testing.T, resp *http.Response, want string) {
	t.Helper()
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, want) {
		t.Fatalf("Content-Type = %q, want prefix %q", got, want)
	}
}

func assertMailDevError(t *testing.T, resp *http.Response, status int, message string) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	assertContentType(t, resp, "application/json")
	if resp.StatusCode != status {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, status, body)
	}
	var payload map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != message || len(payload) != 1 {
		t.Fatalf("error payload = %#v, want %q", payload, message)
	}
}
