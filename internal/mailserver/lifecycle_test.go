package mailserver

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/common"
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
		_ = conn.Close()
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

func TestListenWithReadyDoesNotSignalOnBindFailure(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	port := occupied.Addr().(*net.TCPAddr).Port
	server, err := NewMailServer(port, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	ready := false
	if err := server.ListenWithReady(func() { ready = true }); err == nil {
		t.Fatal("ListenWithReady succeeded on an occupied address")
	}
	if ready {
		t.Fatal("ready callback ran before the SMTP listener was bound")
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

	// The default SMTPS port is privileged, and this test binds it for real.
	server, err := NewMailServerWithOptions(0, "localhost", tmpDir, ServerOptions{
		TLSConfig: tlsConfig,
		SMTPSPort: freePort(t),
	})
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
		_ = conn.Close()
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

	server, err := NewMailServerWithOptions(0, "localhost", tmpDir, ServerOptions{
		AuthConfig: authConfig,
		TLSConfig:  tlsConfig,
		SMTPSPort:  freePort(t),
	})
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
		_ = conn.Close()
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

	server, err := NewMailServerWithOptions(0, "localhost", tmpDir, ServerOptions{
		TLSConfig: tlsConfig,
		SMTPSPort: freePort(t),
	})
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

// freePort reserves an ephemeral port and releases it again. Tests that need a
// bind to succeed use it instead of the privileged default SMTPS port, which a
// non-root test runner cannot bind at all.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func TestListenWithReadyReturnsSMTPSBindFailure(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	smtpsPort := occupied.Addr().(*net.TCPAddr).Port

	server, err := NewMailServerWithOptions(freePort(t), "127.0.0.1", t.TempDir(), ServerOptions{
		TLSConfig: &TLSConfig{Enabled: true},
		SMTPSPort: smtpsPort,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	// ListenWithReady runs off the test goroutine so a regression that goes
	// back to serving the plain listener fails on this deadline rather than
	// blocking in Serve until the package timeout kills the whole run.
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case err = <-result:
	case <-ready:
		t.Fatal("ready callback ran even though the SMTPS listener never bound")
	case <-time.After(5 * time.Second):
		t.Fatal("ListenWithReady neither returned nor reported ready")
	}
	if err == nil {
		t.Fatal("ListenWithReady succeeded with an occupied SMTPS address")
	}
	if !errors.Is(err, ErrSMTPSBind) {
		t.Fatalf("SMTPS bind failure = %v, want it to wrap ErrSMTPSBind", err)
	}
	if !strings.Contains(err.Error(), occupied.Addr().String()) {
		t.Fatalf("SMTPS bind error %q does not name the address it failed on", err)
	}

	reclaimed, err := net.Listen("tcp", server.smtpServer.Addr)
	if err != nil {
		t.Fatalf("plain SMTP listener stayed bound after the SMTPS bind failed: %v", err)
	}
	_ = reclaimed.Close()
}

func TestListenWithReadyStartsNoSMTPSListenerWhenPortIsZero(t *testing.T) {
	server, err := NewMailServerWithOptions(freePort(t), "127.0.0.1", t.TempDir(), ServerOptions{
		TLSConfig: &TLSConfig{Enabled: true},
		SMTPSPort: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if server.smtpsServer != nil {
		t.Fatal("SMTPS port 0 still configured an implicit-TLS server")
	}
	if server.smtpServer.TLSConfig == nil {
		t.Fatal("disabling SMTPS also removed STARTTLS from the SMTP port")
	}

	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-result:
		t.Fatalf("ListenWithReady returned before signaling ready: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("ListenWithReady never signaled ready")
	}
	if listener := server.takeSMTPSListener(); listener != nil {
		_ = listener.Close()
		t.Fatal("SMTPS port 0 still bound a listener")
	}
	if err := server.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	select {
	case <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("ListenWithReady did not return after Close")
	}
}

// Close runs inside ready(), which ListenWithReady invokes synchronously
// immediately before handing the listener to Serve. That puts the shutdown
// squarely in the window this guards: smtp.Server.Close only closes listeners
// Serve has already registered, so a Close arriving first used to close
// nothing, and Serve then blocked in Accept on a listener nobody owned for the
// life of the process. Calling Close from ready is what makes that ordering
// deterministic -- the sibling tests reach it only when the scheduler happens
// to, which is why it surfaced as a flaky "ListenWithReady did not return
// after Close" on loaded CI runners rather than a reliable failure.
func TestCloseBeforeServeRegistersListenerStillStopsListen(t *testing.T) {
	server, err := NewMailServerWithOptions(freePort(t), "127.0.0.1", t.TempDir(), ServerOptions{})
	if err != nil {
		t.Fatal(err)
	}

	closed := make(chan error, 1)
	result := make(chan error, 1)
	go func() {
		result <- server.ListenWithReady(func() { closed <- server.Close() })
	}()

	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ready was never called")
	}

	select {
	case <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("ListenWithReady did not return when Close ran before Serve registered the listener")
	}
}

func TestCloseReleasesSMTPSPortBoundByListen(t *testing.T) {
	smtpsPort := freePort(t)
	server, err := NewMailServerWithOptions(freePort(t), "127.0.0.1", t.TempDir(), ServerOptions{
		TLSConfig: &TLSConfig{Enabled: true},
		SMTPSPort: smtpsPort,
	})
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-result:
		t.Fatalf("ListenWithReady returned before signaling ready: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("ListenWithReady never signaled ready")
	}
	// Reclaiming the port is not on its own evidence that startup handed Close
	// something to release: when the listener is opened inside the serving
	// goroutine instead, that goroutine usually wins the race and registers it
	// with go-smtp, so Close frees the port anyway and the port check passes.
	// The registration below is what makes the release deterministic, and it
	// never happens under that arrangement.
	server.smtpsListenerMutex.Lock()
	registered := server.smtpsListener != nil
	server.smtpsListenerMutex.Unlock()
	if !registered {
		t.Fatal("startup did not register the SMTPS listener for Close to release")
	}
	if err := server.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	select {
	case <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("ListenWithReady did not return after Close")
	}

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(smtpsPort))
	reclaimed, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("SMTPS listener stayed bound after Close: %v", err)
	}
	_ = reclaimed.Close()
}

// TestListenWithReadyLogsSMTPSOnlyAfterBinding pins the half of the fix the
// port assertions cannot see. Announcing the listener before the bind, and
// announcing a port the code had hardcoded rather than the one it took, is how
// the old arrangement told operators a listener was running when none was: the
// log was the only evidence they had, and it was written unconditionally.
func TestListenWithReadyLogsSMTPSOnlyAfterBinding(t *testing.T) {
	captured := captureServerLog(t)

	smtpsPort := freePort(t)
	server, err := NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), ServerOptions{
		TLSConfig: &TLSConfig{Enabled: true},
		SMTPSPort: smtpsPort,
	})
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- server.ListenWithReady(func() { close(ready) }) }()
	select {
	case <-ready:
	case err := <-result:
		t.Fatalf("ListenWithReady returned before signaling ready: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("ListenWithReady never signaled ready")
	}
	if err := server.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	<-result

	logged := captured.String()
	if want := "SMTPS Server running at 127.0.0.1:" + strconv.Itoa(smtpsPort); !strings.Contains(logged, want) {
		t.Fatalf("startup log does not announce the bound SMTPS port %q:\n%s", want, logged)
	}

	captured.Reset()
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()

	failing, err := NewMailServerWithOptions(0, "127.0.0.1", t.TempDir(), ServerOptions{
		TLSConfig: &TLSConfig{Enabled: true},
		SMTPSPort: occupied.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = failing.Close() }()

	failed := make(chan error, 1)
	go func() { failed <- failing.ListenWithReady(func() {}) }()
	select {
	case err := <-failed:
		if err == nil {
			t.Fatal("ListenWithReady succeeded with an occupied SMTPS address")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ListenWithReady did not return after the SMTPS bind failed")
	}
	if strings.Contains(captured.String(), "SMTPS Server running") {
		t.Fatalf("a failed SMTPS bind still announced a running listener:\n%s", captured.String())
	}
}

// captureServerLog redirects the package-level logger for one test and puts it
// back afterwards. The logger is process-global, so this is safe only because
// no test in this package calls t.Parallel.
func captureServerLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	captured := &bytes.Buffer{}
	common.InitLoggerOutput(common.LogLevelNormal, captured)
	t.Cleanup(func() { common.InitLogger(common.LogLevelNormal) })
	return captured
}
