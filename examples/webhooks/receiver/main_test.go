package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReceiveUnsignedRequest(t *testing.T) {
	t.Setenv("OWLMAIL_WEBHOOK_SECRET", "")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/owlmail", bytes.NewBufferString(`{"event":"email.received"}`))

	receive(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestReceiveVerifiesSignature(t *testing.T) {
	const secret = "example-secret"
	body := []byte(`{"event":"email.received"}`)
	t.Setenv("OWLMAIL_WEBHOOK_SECRET", secret)

	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := "test-nonce"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "." + nonce + "."))
	_, _ = mac.Write(body)
	signature := "v2=" + hex.EncodeToString(mac.Sum(nil))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/owlmail", bytes.NewReader(body))
	request.Header.Set("X-OwlMail-Signature-V2", signature)
	request.Header.Set("X-OwlMail-Timestamp", timestamp)
	request.Header.Set("X-OwlMail-Nonce", nonce)
	receive(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestReceiveRejectsReplayedSignature(t *testing.T) {
	const secret = "example-secret"
	body := []byte(`{"event":"email.received"}`)
	t.Setenv("OWLMAIL_WEBHOOK_SECRET", secret)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := "replayed-nonce"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "." + nonce + "."))
	_, _ = mac.Write(body)
	signature := "v2=" + hex.EncodeToString(mac.Sum(nil))
	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/owlmail", bytes.NewReader(body))
		request.Header.Set("X-OwlMail-Signature-V2", signature)
		request.Header.Set("X-OwlMail-Timestamp", timestamp)
		request.Header.Set("X-OwlMail-Nonce", nonce)
		receive(recorder, request)
		want := http.StatusNoContent
		if attempt == 1 {
			want = http.StatusUnauthorized
		}
		if recorder.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", attempt+1, recorder.Code, want)
		}
	}
}

func TestReceiveRejectsMissingSignature(t *testing.T) {
	t.Setenv("OWLMAIL_WEBHOOK_SECRET", "example-secret")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/owlmail", bytes.NewBufferString(`{}`))

	receive(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestReceiveRejectsUnsupportedMethod(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/owlmail", nil)

	receive(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}
