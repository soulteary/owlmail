package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/soulteary/owlmail/internal/emailcheck"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestEmailCheckIsReadOnlyAndReportsFailedAssertions(t *testing.T) {
	api, s, _ := setupTestAPI(t)
	defer func() { _ = s.Close() }()
	if err := s.SaveEmailToStore("check", false, &mailserver.Envelope{}, &mailserver.Email{Text: "Code 123456"}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/v1/emails/check/check", strings.NewReader(`{"text_contains":["absent"],"extract":[{"name":"code","pattern":"[0-9]{6}"}]}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := api.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var r emailcheck.Report
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || r.Passed || r.Extractions["code"][0] != "123456" {
		t.Fatal(r)
	}
	e, _ := s.GetEmail("check")
	if e.Read {
		t.Fatal("inspection marked mail read")
	}
}
