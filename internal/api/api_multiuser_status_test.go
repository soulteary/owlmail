package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/soulteary/owlmail/internal/mailbox"
)

func TestMultiuserMailboxStatusPrivacyAndAuthentication(t *testing.T) {
	f := newMultiFixture(t)
	f.api.multiuser.ClaimPolicy = mailbox.ExplicitHistory
	if _, err := f.store.EnsurePending(context.Background(), "main", "alice@example.test", 10); err != nil {
		t.Fatal(err)
	}
	request := func(user, email, verified string) (int, mailbox.PersonalStatus) {
		t.Helper()
		req := httptest.NewRequest("GET", "http://localhost/api/v1/me/mailbox", nil)
		if user != "" {
			req.Header.Set("X-Auth-User", user)
		}
		if email != "" {
			req.Header.Set("X-Auth-Email", email)
		}
		if verified != "" {
			req.Header.Set("X-Auth-Email-Verified", verified)
		}
		res, err := f.api.app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		status := mailbox.PersonalStatus{}
		if res.StatusCode == 200 {
			if res.Header.Get("Cache-Control") != "private, no-store" || res.Header.Get("X-OwlMail-Identity") == "" {
				t.Fatal("missing scoped policy")
			}
			var raw map[string]string
			if err = json.NewDecoder(res.Body).Decode(&raw); err != nil {
				t.Fatal(err)
			}
			if len(raw) != 4 {
				t.Fatal("unexpected identity details", raw)
			}
			status = mailbox.PersonalStatus{MailboxState: raw["mailbox_state"], DeliveryState: raw["delivery_state"], HistoryState: raw["history_state"], Action: raw["action"]}
		}
		return res.StatusCode, status
	}
	if code, _ := request("", "", ""); code != 401 {
		t.Fatal(code)
	}
	if code, _ := request("alice", "alice@example.test", "TRUE"); code != 401 {
		t.Fatal(code)
	}
	for _, address := range []string{"alice@example.test", "absent@example.test"} {
		code, status := request("alice", address, "false")
		if code != 200 || status != (mailbox.PersonalStatus{MailboxState: "ready", DeliveryState: "verification_required", HistoryState: "none", Action: "verify_email"}) {
			t.Fatal(code, status)
		}
	}
	code, status := request("alice", "alice@example.test", "true")
	if code != 200 || status != (mailbox.PersonalStatus{MailboxState: "ready", DeliveryState: "bound", HistoryState: "retained", Action: "none"}) {
		t.Fatal(code, status)
	}
	code, status = request("bob", "alice@example.test", "true")
	if code != 200 || status != (mailbox.PersonalStatus{MailboxState: "ready", DeliveryState: "conflict", HistoryState: "none", Action: "contact_operator"}) {
		t.Fatal(code, status)
	}
	code, status = request("new-user", "new@example.test", "true")
	if code != 200 || status != (mailbox.PersonalStatus{MailboxState: "ready", DeliveryState: "bound", HistoryState: "none", Action: "none"}) {
		t.Fatal(code, status)
	}
}

func TestMultiuserMailboxStatusControlFailureIsUnavailable(t *testing.T) {
	f := newMultiFixture(t)
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	res := f.request(t, "alice", "GET", "/api/v1/me/mailbox", "")
	if res.StatusCode != 503 {
		t.Fatal(res.StatusCode, bodyText(t, res))
	}
}
