package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
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

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/owlmail", bytes.NewReader(body))
	request.Header.Set("X-OwlMail-Signature", signature)
	receive(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
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
