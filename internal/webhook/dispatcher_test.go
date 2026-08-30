package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

func testEmail() *types.Email {
	return &types.Email{
		ID:        "email-123",
		Time:      time.Date(2026, 8, 29, 12, 30, 0, 0, time.UTC),
		Subject:   "System Alert",
		From:      []*mail.Address{{Name: "Monitor", Address: "monitor@example.com"}},
		To:        []*mail.Address{{Address: "ops@example.net"}},
		CC:        []*mail.Address{{Address: "audit@example.net"}},
		Text:      "Verification code: 123456",
		HTML:      "<p>Verification code: 123456</p>",
		Size:      512,
		SizeHuman: "512 B",
		Envelope: &types.Envelope{
			From: "monitor@example.com",
			To:   []string{"ops@example.net"},
		},
		Source: "/private/mail/email-123.eml",
		Attachments: []*types.Attachment{{
			FileName:    "report.txt",
			ContentType: "text/plain",
			Size:        42,
		}},
	}
}

func TestDispatchDefaultPayload(t *testing.T) {
	var received eventPayload
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s", request.Method)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", request.Header.Get("Content-Type"))
		}
		if request.Header.Get("X-OwlMail-Event") != "email.received" || request.Header.Get("X-OwlMail-Email-ID") != "email-123" {
			t.Errorf("missing OwlMail headers")
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "primary", URL: server.URL}}}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results := dispatcher.Dispatch(context.Background(), testEmail())
	if len(results) != 1 || results[0].Err != nil || results[0].StatusCode != http.StatusNoContent || results[0].Attempts != 1 {
		t.Fatalf("Dispatch() = %#v", results)
	}
	if received.Event != "email.received" || received.Email.Subject != "System Alert" {
		t.Errorf("payload = %#v", received)
	}
	if strings.Contains(received.Message, "/private/mail") {
		t.Error("payload leaked local source path")
	}
	encoded, _ := json.Marshal(received)
	if strings.Contains(string(encoded), "/private/mail") {
		t.Error("payload JSON leaked local source path")
	}
}

func TestDispatchCustomTemplateHeadersAndSignature(t *testing.T) {
	const secret = "shared-secret"
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedBody, _ = io.ReadAll(request.Body)
		if request.Header.Get("X-Custom") != "owlmail" {
			t.Errorf("X-Custom = %q", request.Header.Get("X-Custom"))
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(receivedBody)
		wantSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if request.Header.Get("X-OwlMail-Signature") != wantSignature {
			t.Errorf("signature = %q, want %q", request.Header.Get("X-OwlMail-Signature"), wantSignature)
		}
		timestamp := request.Header.Get("X-OwlMail-Timestamp")
		nonce := request.Header.Get("X-OwlMail-Nonce")
		if timestamp == "" || nonce == "" {
			t.Fatal("timestamp and nonce headers are required")
		}
		mac = hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(timestamp + "." + nonce + "."))
		_, _ = mac.Write(receivedBody)
		wantV2 := "v2=" + hex.EncodeToString(mac.Sum(nil))
		if request.Header.Get("X-OwlMail-Signature-V2") != wantV2 {
			t.Errorf("v2 signature = %q, want %q", request.Header.Get("X-OwlMail-Signature-V2"), wantV2)
		}
		if request.Header.Get("X-OwlMail-Delivery-ID") != "email-123" {
			t.Errorf("delivery id = %q", request.Header.Get("X-OwlMail-Delivery-ID"))
		}
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	dispatcher, err := NewDispatcher(Config{Targets: []Target{{
		Name:         "custom",
		URL:          server.URL,
		Method:       "put",
		ContentType:  "application/vnd.owlmail+json",
		Headers:      map[string]string{"X-Custom": "owlmail"},
		Secret:       secret,
		BodyTemplate: `{"title":{{ json .Subject }},"message":{{ json (truncate .Text 17) }},"to":{{ json .To }}}`,
	}}}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results := dispatcher.Dispatch(context.Background(), testEmail())
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("Dispatch() = %#v", results)
	}
	if string(receivedBody) != `{"title":"System Alert","message":"Verification code","to":["ops@example.net"]}` {
		t.Errorf("body = %s", receivedBody)
	}
}

func TestDispatchRetriesUseFreshReplayProtection(t *testing.T) {
	var headers []http.Header
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		headers = append(headers, request.Header.Clone())
		if len(headers) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	dispatcher, err := NewDispatcher(Config{Targets: []Target{{
		Name: "retry", URL: server.URL, Secret: "secret", Retries: 1,
	}}}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results := dispatcher.DispatchDelivery(context.Background(), testEmail(), "delivery-123")
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %#v", results)
	}
	if len(headers) != 2 || headers[0].Get("X-OwlMail-Nonce") == headers[1].Get("X-OwlMail-Nonce") {
		t.Fatalf("retry nonces were not unique: %#v", headers)
	}
	for _, header := range headers {
		if header.Get("X-OwlMail-Delivery-ID") != "delivery-123" {
			t.Fatalf("delivery ID changed across retries: %q", header.Get("X-OwlMail-Delivery-ID"))
		}
	}
}

func TestDispatchAppliesMatchRules(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher, err := NewDispatcher(Config{Targets: []Target{{
		Name: "filtered",
		URL:  server.URL,
		Match: Match{
			From:    []string{"*@EXAMPLE.COM"},
			To:      []string{"ops@*"},
			Subject: []string{"*alert*"},
			Text:    []string{"*123456*"},
		},
	}}}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if results := dispatcher.Dispatch(context.Background(), testEmail()); len(results) != 1 || results[0].Err != nil {
		t.Fatalf("matching Dispatch() = %#v", results)
	}

	email := testEmail()
	email.Subject = "Routine report"
	if results := dispatcher.Dispatch(context.Background(), email); len(results) != 0 {
		t.Fatalf("non-matching Dispatch() = %#v", results)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}
}

func TestDispatchRetriesTransientFailure(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "retry", URL: server.URL, Retries: 1}}}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results := dispatcher.Dispatch(context.Background(), testEmail())
	if len(results) != 1 || results[0].Err != nil || results[0].Attempts != 2 || requests.Load() != 2 {
		t.Fatalf("Dispatch() = %#v, requests = %d", results, requests.Load())
	}
}

func TestDispatchDoesNotRetryPermanentFailure(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "bad", URL: server.URL, Retries: 3}}}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results := dispatcher.Dispatch(context.Background(), testEmail())
	if len(results) != 1 || results[0].Err == nil || results[0].Attempts != 1 || requests.Load() != 1 {
		t.Fatalf("Dispatch() = %#v, requests = %d", results, requests.Load())
	}
}

func TestDispatchDoesNotFollowRedirects(t *testing.T) {
	var redirectedRequests atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		redirectedRequests.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "redirect", URL: redirect.URL}}}, redirect.Client())
	if err != nil {
		t.Fatal(err)
	}
	results := dispatcher.Dispatch(context.Background(), testEmail())
	if len(results) != 1 || results[0].Err == nil || results[0].StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("Dispatch() = %#v", results)
	}
	if redirectedRequests.Load() != 0 {
		t.Fatalf("followed redirect %d time(s)", redirectedRequests.Load())
	}
}

func TestDispatchHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher, err := NewDispatcher(Config{Targets: []Target{{Name: "slow", URL: server.URL, Timeout: "10ms"}}}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results := dispatcher.Dispatch(context.Background(), testEmail())
	if len(results) != 1 || results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "deadline") {
		t.Fatalf("Dispatch() = %#v", results)
	}
}

func TestDispatchDoesNotLeakTargetURLInErrors(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: request.URL.String(), Err: errors.New("connection failed")}
	})}
	dispatcher, err := NewDispatcher(Config{Targets: []Target{{
		Name: "secret-url",
		URL:  "https://example.com/hooks/test?token=do-not-log",
	}}}, client)
	if err != nil {
		t.Fatal(err)
	}
	results := dispatcher.Dispatch(context.Background(), testEmail())
	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("Dispatch() = %#v", results)
	}
	if strings.Contains(results[0].Err.Error(), "do-not-log") || strings.Contains(results[0].Err.Error(), "example.com") {
		t.Fatalf("error leaked target URL: %v", results[0].Err)
	}
}

func TestDispatchRejectsOversizedAndNilEmail(t *testing.T) {
	dispatcher, err := NewDispatcher(Config{Targets: []Target{{
		Name:         "large",
		URL:          "https://example.com",
		BodyTemplate: "{{ .Text }}",
	}}}, &http.Client{Transport: roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("HTTP request should not be attempted")
		return nil, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	email := testEmail()
	email.Text = strings.Repeat("x", maxPayloadBytes+1)
	results := dispatcher.Dispatch(context.Background(), email)
	if len(results) != 1 || results[0].Err == nil || results[0].Attempts != 0 {
		t.Fatalf("oversized Dispatch() = %#v", results)
	}
	results = dispatcher.Dispatch(context.Background(), nil)
	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("nil Dispatch() = %#v", results)
	}
}

func TestTargetCountHandlesNilDispatcher(t *testing.T) {
	var dispatcher *Dispatcher
	if dispatcher.TargetCount() != 0 {
		t.Fatal("nil dispatcher should have zero targets")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
