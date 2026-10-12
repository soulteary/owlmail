package api

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestAPIReadyCertificateUsesLoadedKeyPairAfterRotation(t *testing.T) {
	donor := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	pair := donor.TLS.Certificates[0]
	donor.Close()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	template, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	template.SerialNumber = big.NewInt(123456)
	template.NotBefore = time.Now().Add(-time.Minute)
	template.NotAfter = time.Now().Add(time.Hour)
	signer := pair.PrivateKey.(crypto.Signer)
	rotatedDER, err := x509.CreateCertificate(rand.Reader, template, template, signer.Public(), signer)
	if err != nil {
		t.Fatal(err)
	}
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reserved.Addr().(*net.TCPAddr).Port
	server, err := mailserver.NewMailServer(0, "127.0.0.1", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	blocked := NewAPIWithHTTPSDeferredRecovery(server, port, "127.0.0.1", "", "", true, certPath, keyPath)
	called := false
	if err := blocked.StartWithCertificateReady(func(_ [][]byte) error { called = true; return nil }); err == nil || called {
		_ = reserved.Close()
		t.Fatalf("HTTPS bind failure invoked readiness: %v, called=%v", err, called)
	}
	_ = reserved.Close()
	api := NewAPIWithHTTPSDeferredRecovery(server, port, "127.0.0.1", "", "", true, certPath, keyPath)
	ready := make(chan []byte, 1)
	finished := make(chan error, 1)
	go func() {
		finished <- api.StartWithCertificateReady(func(chain [][]byte) error {
			if len(chain) != 1 || !bytes.Equal(chain[0], pair.Certificate[0]) {
				return fmt.Errorf("callback certificate differs from loaded keypair")
			}
			// Rotate the mounted certificate before the listener starts serving.
			if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rotatedDER}), 0600); err != nil {
				return err
			}
			ready <- append([]byte(nil), chain[0]...)
			// The caller owns its copy and cannot corrupt the serving certificate.
			chain[0][0] ^= 1
			return nil
		})
	}()
	var loadedDER []byte
	select {
	case loadedDER = <-ready:
	case err := <-finished:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("HTTPS listener did not become ready")
	}
	defer func() {
		_ = api.app.Shutdown()
		select {
		case err := <-finished:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("HTTPS listener did not stop")
		}
	}()
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: loadedDER})) {
		t.Fatal("invalid callback certificate")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	response, err := client.Get(fmt.Sprintf("https://127.0.0.1:%d/healthz", port))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Equal(response.TLS.PeerCertificates[0].Raw, loadedDER) {
		t.Fatal("listener did not serve the exact certificate provided to readiness")
	}
}

func TestAPIReadyCertificateFailureClosesListener(t *testing.T) {
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reserved.Addr().(*net.TCPAddr).Port
	_ = reserved.Close()
	server, err := mailserver.NewMailServer(0, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	api := NewAPIWithHTTPSDeferredRecovery(server, port, "127.0.0.1", "", "", false, "", "")
	failed := errors.New("startup record unavailable")
	if err := api.StartWithCertificateReady(func(chain [][]byte) error {
		if len(chain) != 0 {
			t.Error("HTTP listener returned certificates")
		}
		return failed
	}); !errors.Is(err, failed) {
		t.Fatalf("callback failure: %v", err)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal("failed callback retained the listener:", err)
	}
	_ = listener.Close()
}
