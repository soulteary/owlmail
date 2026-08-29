// Command receiver runs a loopback-only HTTP endpoint for exercising OwlMail
// webhook configurations during local development.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	listenAddress  = "127.0.0.1:18080"
	maxRequestBody = 2 << 20
)

func main() {
	server := &http.Server{
		Addr:              listenAddress,
		Handler:           http.HandlerFunc(receive),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("OwlMail example receiver listening on http://%s", listenAddress)
	log.Fatal(server.ListenAndServe())
}

func receive(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost && request.Method != http.MethodPut && request.Method != http.MethodPatch {
		writer.Header().Set("Allow", "POST, PUT, PATCH")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBody)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, "request body is too large or unreadable", http.StatusBadRequest)
		return
	}

	verified, err := verifySignature(request.Header.Get("X-OwlMail-Signature"), body, os.Getenv("OWLMAIL_WEBHOOK_SECRET"))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusUnauthorized)
		return
	}

	log.Printf(
		"path=%s event=%q email_id=%q content_type=%q authorization=%t signature_verified=%t body=%s",
		request.URL.Path,
		request.Header.Get("X-OwlMail-Event"),
		request.Header.Get("X-OwlMail-Email-ID"),
		request.Header.Get("Content-Type"),
		request.Header.Get("Authorization") != "",
		verified,
		body,
	)
	writer.WriteHeader(http.StatusNoContent)
}

func verifySignature(signature string, body []byte, secret string) (bool, error) {
	if signature == "" {
		if secret != "" {
			return false, fmt.Errorf("X-OwlMail-Signature is required when OWLMAIL_WEBHOOK_SECRET is set")
		}
		return false, nil
	}
	if secret == "" {
		return false, nil
	}

	const prefix = "sha256="
	if len(signature) <= len(prefix) || signature[:len(prefix)] != prefix {
		return false, fmt.Errorf("invalid X-OwlMail-Signature format")
	}
	provided, err := hex.DecodeString(signature[len(prefix):])
	if err != nil {
		return false, fmt.Errorf("invalid X-OwlMail-Signature encoding")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return false, fmt.Errorf("X-OwlMail-Signature verification failed")
	}
	return true, nil
}
