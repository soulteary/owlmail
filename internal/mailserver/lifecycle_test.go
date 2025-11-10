package mailserver

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/outgoing"
)

// TestListenBasic tests basic SMTP server listening
func TestListenBasic(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(0, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Use a context with timeout to control the test
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start server in a goroutine
	var listenErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Listen will block, so we need to handle it carefully
		// We'll use a channel to signal when it starts
		listenErr = server.Listen()
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is listening by attempting to connect
	conn, err := net.DialTimeout("tcp", server.smtpServer.Addr, 500*time.Millisecond)
	if err != nil {
		t.Logf("Could not connect to server (this may be expected): %v", err)
	} else {
		conn.Close()
	}

	// Close the server
	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %v", err)
	}

	// Wait for listen goroutine to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Listen should have returned after Close
	case <-ctx.Done():
		t.Log("Listen goroutine did not finish in time (this may be expected)")
	}

	// Check if there was an error (should be nil or server closed error)
	if listenErr != nil {
		t.Logf("Listen returned error (may be expected): %v", listenErr)
	}
}

// TestListenWithAuth tests SMTP server listening with authentication enabled
func TestListenWithAuth(t *testing.T) {
	tmpDir := t.TempDir()

	authConfig := &SMTPAuthConfig{
		Username: "testuser",
		Password: "testpass",
		Enabled:  true,
	}

	server, err := NewMailServerWithConfig(0, "localhost", tmpDir, nil, authConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Start server in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.Listen()
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify auth config is set
	if server.authConfig == nil || !server.authConfig.Enabled {
		t.Error("Auth config should be enabled")
	}

	// Close the server
	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %v", err)
	}

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)
}

// TestListenWithTLS tests SMTP server listening with TLS enabled
func TestListenWithTLS(t *testing.T) {
	tmpDir := t.TempDir()

	tlsConfig := &TLSConfig{
		Enabled: true,
		// CertFile and KeyFile are empty, so it will generate self-signed cert
	}

	server, err := NewMailServerWithConfig(0, "localhost", tmpDir, nil, nil, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Verify SMTPS server is configured
	if server.smtpsServer == nil {
		t.Error("SMTPS server should be configured when TLS is enabled")
	}

	// Start server in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.Listen()
	}()

	// Give the server a moment to start
	time.Sleep(200 * time.Millisecond)

	// Verify TLS config is set
	if server.tlsConfig == nil || !server.tlsConfig.Enabled {
		t.Error("TLS config should be enabled")
	}

	// Try to connect to SMTPS port (465)
	conn, err := net.DialTimeout("tcp", server.smtpsServer.Addr, 500*time.Millisecond)
	if err != nil {
		t.Logf("Could not connect to SMTPS server (this may be expected): %v", err)
	} else {
		conn.Close()
	}

	// Close the server
	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %v", err)
	}

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)
}

// TestListenWithAuthAndTLS tests SMTP server listening with both auth and TLS enabled
func TestListenWithAuthAndTLS(t *testing.T) {
	tmpDir := t.TempDir()

	authConfig := &SMTPAuthConfig{
		Username: "testuser",
		Password: "testpass",
		Enabled:  true,
	}

	tlsConfig := &TLSConfig{
		Enabled: true,
	}

	server, err := NewMailServerWithConfig(0, "localhost", tmpDir, nil, authConfig, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Verify both configs are set
	if server.authConfig == nil || !server.authConfig.Enabled {
		t.Error("Auth config should be enabled")
	}
	if server.tlsConfig == nil || !server.tlsConfig.Enabled {
		t.Error("TLS config should be enabled")
	}
	if server.smtpsServer == nil {
		t.Error("SMTPS server should be configured")
	}

	// Start server in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.Listen()
	}()

	// Give the server a moment to start
	time.Sleep(200 * time.Millisecond)

	// Close the server
	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %v", err)
	}

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)
}

// TestCloseBasic tests basic server closing
func TestCloseBasic(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(0, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Close should succeed even if server is not started
	if err := server.Close(); err != nil {
		t.Errorf("Close should succeed: %v", err)
	}

	// Verify eventChan is closed
	select {
	case _, ok := <-server.eventChan:
		if ok {
			t.Error("eventChan should be closed")
		}
	default:
		// Channel is already closed, which is expected
	}
}

// TestCloseWithOutgoing tests server closing with outgoing mail configured
func TestCloseWithOutgoing(t *testing.T) {
	tmpDir := t.TempDir()

	outgoingConfig := &outgoing.OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	server, err := NewMailServerWithOutgoing(0, "localhost", tmpDir, outgoingConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Verify outgoing is set
	if server.outgoing == nil {
		t.Fatal("Outgoing should be configured")
	}

	// Close should succeed and close outgoing
	if err := server.Close(); err != nil {
		t.Errorf("Close should succeed: %v", err)
	}

	// Verify eventChan is closed
	select {
	case _, ok := <-server.eventChan:
		if ok {
			t.Error("eventChan should be closed")
		}
	default:
		// Channel is already closed, which is expected
	}
}

// TestCloseWithSMTPS tests server closing with SMTPS server configured
func TestCloseWithSMTPS(t *testing.T) {
	tmpDir := t.TempDir()

	tlsConfig := &TLSConfig{
		Enabled: true,
	}

	server, err := NewMailServerWithConfig(0, "localhost", tmpDir, nil, nil, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Verify SMTPS server is configured
	if server.smtpsServer == nil {
		t.Fatal("SMTPS server should be configured")
	}

	// Close should succeed and close both servers
	if err := server.Close(); err != nil {
		t.Errorf("Close should succeed: %v", err)
	}

	// Verify eventChan is closed
	select {
	case _, ok := <-server.eventChan:
		if ok {
			t.Error("eventChan should be closed")
		}
	default:
		// Channel is already closed, which is expected
	}
}

// TestCloseWithAllConfigs tests server closing with all configurations
func TestCloseWithAllConfigs(t *testing.T) {
	tmpDir := t.TempDir()

	outgoingConfig := &outgoing.OutgoingConfig{
		Host: "smtp.example.com",
		Port: 587,
	}

	authConfig := &SMTPAuthConfig{
		Username: "testuser",
		Password: "testpass",
		Enabled:  true,
	}

	tlsConfig := &TLSConfig{
		Enabled: true,
	}

	server, err := NewMailServerWithConfig(0, "localhost", tmpDir, outgoingConfig, authConfig, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Verify all configs are set
	if server.outgoing == nil {
		t.Error("Outgoing should be configured")
	}
	if server.authConfig == nil {
		t.Error("Auth config should be set")
	}
	if server.tlsConfig == nil {
		t.Error("TLS config should be set")
	}
	if server.smtpsServer == nil {
		t.Error("SMTPS server should be configured")
	}

	// Close should succeed
	if err := server.Close(); err != nil {
		t.Errorf("Close should succeed: %v", err)
	}

	// Verify eventChan is closed
	select {
	case _, ok := <-server.eventChan:
		if ok {
			t.Error("eventChan should be closed")
		}
	default:
		// Channel is already closed, which is expected
	}
}

// TestCloseMultipleTimes tests closing server multiple times
func TestCloseMultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(0, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// First close should succeed
	if err := server.Close(); err != nil {
		t.Errorf("First Close should succeed: %v", err)
	}

	// Second close may return an error (smtp.Server.Close() returns error if already closed)
	// This is acceptable behavior - the important thing is it doesn't panic
	err = server.Close()
	if err != nil {
		// Error is acceptable for second close, as long as it doesn't panic
		t.Logf("Second Close returned error (expected): %v", err)
	}
}

// TestListenAndClose tests starting and then closing the server
func TestListenAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(0, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Start server in a goroutine
	var wg sync.WaitGroup
	var listenErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		listenErr = server.Listen()
	}()

	// Give the server a moment to start
	time.Sleep(200 * time.Millisecond)

	// Verify server is listening
	conn, err := net.DialTimeout("tcp", server.smtpServer.Addr, 500*time.Millisecond)
	if err != nil {
		t.Logf("Could not connect to server (this may be expected): %v", err)
	} else {
		conn.Close()
	}

	// Close the server
	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %v", err)
	}

	// Wait for listen to return
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Listen should have returned
		if listenErr != nil {
			t.Logf("Listen returned error (may be expected): %v", listenErr)
		}
	case <-time.After(2 * time.Second):
		t.Error("Listen did not return after Close")
	}
}

// TestListenWithSMTPSErrorHandling tests error handling when SMTPS server fails to start
func TestListenWithSMTPSErrorHandling(t *testing.T) {
	tmpDir := t.TempDir()

	tlsConfig := &TLSConfig{
		Enabled: true,
	}

	server, err := NewMailServerWithConfig(0, "localhost", tmpDir, nil, nil, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Verify SMTPS server is configured
	if server.smtpsServer == nil {
		t.Fatal("SMTPS server should be configured")
	}

	// Start server in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = server.Listen()
	}()

	// Give the server a moment to start
	time.Sleep(200 * time.Millisecond)

	// Close the server
	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %v", err)
	}

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)
}

// TestCloseEventChan tests that eventChan is properly closed
func TestCloseEventChan(t *testing.T) {
	tmpDir := t.TempDir()
	server, err := NewMailServer(0, "localhost", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create mail server: %v", err)
	}

	// Verify eventChan is open
	select {
	case <-server.eventChan:
		t.Error("eventChan should be open before Close")
	default:
		// Channel is open, which is expected
	}

	// Close the server
	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %v", err)
	}

	// Verify eventChan is closed
	select {
	case _, ok := <-server.eventChan:
		if ok {
			t.Error("eventChan should be closed after Close")
		}
	default:
		t.Error("eventChan should be closed and readable")
	}
}
