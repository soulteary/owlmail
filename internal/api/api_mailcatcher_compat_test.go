package api

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/owlmail/internal/types"
)

func TestMailCatcherRESTFacadeIsOptIn(t *testing.T) {
	api, server, _ := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	req, _ := http.NewRequest(http.MethodGet, "/messages", nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled facade returned %d", resp.StatusCode)
	}
}

func TestMailCatcherRESTFacadeContract(t *testing.T) {
	api, server, mailDir := setupTestAPI(t)
	defer func() { _ = server.Close() }()
	api.SetMailCatcherRESTCompat(true)
	email := &types.Email{
		ID: "mail-1", Subject: "MailCatcher", Text: "plain body", HTML: `<img src="cid:logo@example.test"><img src="cid:folder/logo?theme#1@example.test"><img src="cid:foo&amp;bar@example.test"><img src="cid:foo&amp;copy;@example.test">`,
		Time: time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC), Size: 123,
		Envelope: &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}},
		Attachments: []*types.Attachment{
			{ContentID: "logo@example.test", ContentType: "image/png", ContentDisposition: "attachment", FileName: "logo.png", GeneratedFileName: "safe-logo.png", Size: 4},
			{ContentID: "folder/logo?theme#1@example.test", ContentType: "image/png", ContentDisposition: "inline", FileName: "special.png", GeneratedFileName: "safe-special.png", Size: 7},
			{ContentID: "foo&bar@example.test", ContentType: "image/png", ContentDisposition: "inline", FileName: "amp.png", GeneratedFileName: "safe-amp.png", Size: 3},
			{ContentID: "foo&copy;@example.test", ContentType: "image/png", ContentDisposition: "inline", FileName: "entity.png", GeneratedFileName: "safe-entity.png", Size: 6},
		},
	}
	if err := os.WriteFile(filepath.Join(mailDir, "mail-1.eml"), []byte("Subject: MailCatcher\r\n\r\nsource"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(mailDir, "mail-1"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mailDir, "mail-1", "safe-logo.png"), []byte("logo"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mailDir, "mail-1", "safe-special.png"), []byte("special"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mailDir, "mail-1", "safe-amp.png"), []byte("amp"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mailDir, "mail-1", "safe-entity.png"), []byte("entity"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(email.ID, false, email.Envelope, email); err != nil {
		t.Fatal(err)
	}
	second := &types.Email{
		ID: "mail-2", Subject: "Newest capture", Text: "second",
		Time:     time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		Envelope: &types.Envelope{From: "new@example.test", To: []string{"recipient@example.test"}},
	}
	if err := os.WriteFile(filepath.Join(mailDir, "mail-2.eml"), []byte("Subject: Newest capture\r\n\r\nsecond"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(second.ID, false, second.Envelope, second); err != nil {
		t.Fatal(err)
	}

	resp := mailCatcherRequest(t, api, http.MethodGet, "/messages")
	var list []map[string]interface{}
	decodeMailCatcherJSON(t, resp, &list)
	if len(list) != 2 || list[0]["id"] != "mail-2" || list[1]["sender"] != "<sender@example.test>" {
		t.Fatalf("unexpected list: %#v", list)
	}
	resp = mailCatcherRequest(t, api, http.MethodGet, "/messages/mail-1.json")
	var detail map[string]interface{}
	decodeMailCatcherJSON(t, resp, &detail)
	if len(detail["formats"].([]interface{})) != 3 || len(detail["attachments"].([]interface{})) != 4 {
		t.Fatalf("unexpected detail: %#v", detail)
	}
	if detail["created_at"] == email.Time.Format(time.RFC3339) {
		t.Fatalf("created_at used sender-controlled Date header: %#v", detail)
	}
	if detail["created_at"] == (time.Time{}).Format(time.RFC3339) {
		t.Fatalf("created_at is zero: %#v", detail)
	}
	restored := mailCatcherMessageDTO(&types.Email{
		ID: "restored", Envelope: &types.Envelope{},
		From: []*mail.Address{{Address: "header-sender@example.test"}},
		To:   []*mail.Address{{Address: "header-recipient@example.test"}},
	}, time.Now(), true)
	if restored["sender"] != "<header-sender@example.test>" || len(restored["recipients"].([]string)) != 1 {
		t.Fatalf("restored envelope fallback = %#v", restored)
	}

	resp = mailCatcherRequest(t, api, http.MethodGet, "/messages/mail-1.html")
	htmlBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || !strings.Contains(string(htmlBody), url.PathEscape("folder/logo?theme#1@example.test")) {
		t.Fatalf("HTML CID rewrite = %q, %v", htmlBody, err)
	}
	for _, path := range []string{"/messages/mail-1.plain", "/messages/mail-1.source", "/messages/mail-1.eml", "/messages/mail-1/parts/logo@example.test", "/messages/mail-1/parts/" + url.PathEscape("folder/logo?theme#1@example.test"), "/messages/mail-1/parts/" + url.PathEscape("foo&bar@example.test"), "/messages/mail-1/parts/" + url.PathEscape("foo&copy;@example.test")} {
		resp = mailCatcherRequest(t, api, http.MethodGet, path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s returned %d", path, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}
	if strings.Contains(string(htmlBody), "foo&amp;amp;bar") {
		t.Fatalf("HTML CID rewrite retained a serialized entity: %s", htmlBody)
	}
	entityURL := html.EscapeString("/messages/mail-1/parts/" + url.PathEscape("foo&copy;@example.test"))
	if !strings.Contains(string(htmlBody), entityURL) {
		t.Fatalf("HTML CID rewrite did not escape entity-like ampersand: %s", htmlBody)
	}
	if err := os.Remove(filepath.Join(mailDir, "mail-1", "safe-special.png")); err != nil {
		t.Fatal(err)
	}
	resp = mailCatcherRequest(t, api, http.MethodGet, "/messages/mail-1/parts/"+url.PathEscape("folder/logo?theme#1@example.test"))
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("unreadable part returned %d, want 500", resp.StatusCode)
	}
	_ = resp.Body.Close()
	resp = mailCatcherRequest(t, api, http.MethodDelete, "/messages/mail-1")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE returned %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	resp = mailCatcherRequest(t, api, http.MethodDelete, "/messages/missing")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing DELETE returned %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func mailCatcherRequest(t *testing.T, api *API, method, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, path, nil)
	resp, err := api.app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeMailCatcherJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
}
