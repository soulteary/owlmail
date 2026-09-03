package mailserver

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/emersion/go-smtp"
)

type gatedDataReader struct {
	reader  io.Reader
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (reader *gatedDataReader) Read(buffer []byte) (int, error) {
	reader.once.Do(func() { close(reader.entered) })
	<-reader.release
	return reader.reader.Read(buffer)
}

func newDataTestSession(server *MailServer) *Session {
	return &Session{
		mailServer:    server,
		from:          "sender@example.test",
		to:            []string{"recipient@example.test"},
		authenticated: true,
	}
}

func assertDataLimitError(t *testing.T, err error) {
	t.Helper()
	var smtpErr *smtp.SMTPError
	if !errors.As(err, &smtpErr) {
		t.Fatalf("DATA error = %v, want SMTP error", err)
	}
	if smtpErr.Code != 451 || smtpErr.EnhancedCode != (smtp.EnhancedCode{4, 3, 2}) {
		t.Fatalf("DATA status = %d %v, want 451 4.3.2", smtpErr.Code, smtpErr.EnhancedCode)
	}
	if smtpErr.Message != "temporary resource limit reached; try again later" {
		t.Fatalf("DATA message = %q", smtpErr.Message)
	}
}

func TestDataConcurrencyRejectsImmediatelyAndRecovers(t *testing.T) {
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{MaxDataConcurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	releaseFirst := make(chan struct{})
	firstEntered := make(chan struct{})
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- newDataTestSession(server).Data(&gatedDataReader{
			reader:  bytes.NewReader(validMessage("first")),
			entered: firstEntered,
			release: releaseFirst,
		})
	}()
	<-firstEntered

	secondErr := newDataTestSession(server).Data(bytes.NewReader(validMessage("rejected")))
	assertDataLimitError(t, secondErr)
	if got := len(server.dataLimiter.slots); got != 1 {
		t.Fatalf("occupied DATA slots = %d, want 1", got)
	}

	close(releaseFirst)
	if err := <-firstResult; err != nil {
		t.Fatalf("first DATA failed: %v", err)
	}
	if got := len(server.dataLimiter.slots); got != 0 {
		t.Fatalf("occupied DATA slots after completion = %d, want 0", got)
	}
	if err := newDataTestSession(server).Data(bytes.NewReader(validMessage("after release"))); err != nil {
		t.Fatalf("DATA after release failed: %v", err)
	}
	if got := len(server.GetAllEmail()); got != 2 {
		t.Fatalf("stored messages = %d, want 2", got)
	}
}

func TestDataConcurrencyNeverExceedsLimit(t *testing.T) {
	const (
		limit = 3
		total = 24
	)
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{MaxDataConcurrency: limit})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	var active atomic.Int32
	var peak atomic.Int32
	entered := make(chan struct{}, limit)
	server.afterDataAcquire = func() {
		current := active.Add(1)
		for {
			previous := peak.Load()
			if current <= previous || peak.CompareAndSwap(previous, current) {
				entered <- struct{}{}
				return
			}
		}
	}
	server.beforeDataRelease = func() { active.Add(-1) }

	release := make(chan struct{})
	results := make(chan error, total)
	for index := 0; index < total; index++ {
		go func() {
			results <- newDataTestSession(server).Data(&gatedDataReader{
				reader:  bytes.NewReader(validMessage("pressure")),
				entered: make(chan struct{}),
				release: release,
			})
		}()
	}
	for index := 0; index < limit; index++ {
		<-entered
	}
	for index := 0; index < total-limit; index++ {
		assertDataLimitError(t, <-results)
	}
	close(release)
	for index := 0; index < limit; index++ {
		if err := <-results; err != nil {
			t.Fatalf("accepted DATA failed: %v", err)
		}
	}
	if got := peak.Load(); got != limit {
		t.Fatalf("peak DATA concurrency = %d, want %d", got, limit)
	}
	if got := active.Load(); got != 0 {
		t.Fatalf("active DATA count after completion = %d, want 0", got)
	}
}

func TestUnlimitedDataConcurrency(t *testing.T) {
	const total = 8
	server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{MaxDataConcurrency: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if server.dataLimiter != nil {
		t.Fatal("unlimited mode created a limiter")
	}

	release := make(chan struct{})
	entered := make(chan struct{}, total)
	server.afterDataAcquire = func() { entered <- struct{}{} }
	results := make(chan error, total)
	for index := 0; index < total; index++ {
		go func() {
			results <- newDataTestSession(server).Data(&gatedDataReader{
				reader:  bytes.NewReader(validMessage("unlimited")),
				entered: make(chan struct{}),
				release: release,
			})
		}()
	}
	for index := 0; index < total; index++ {
		<-entered
	}
	close(release)
	for index := 0; index < total; index++ {
		if err := <-results; err != nil {
			t.Fatalf("unlimited DATA failed: %v", err)
		}
	}
}

func TestDataSlotReleasedAfterTransactionFailures(t *testing.T) {
	readFailure := errors.New("injected DATA read failure")
	brokenMIME := []byte("From: from@example.test\r\nTo: recipient@example.test\r\n" +
		"Content-Type: multipart/mixed; boundary=broken\r\n\r\n" +
		"--broken\r\nContent-Type: application/octet-stream\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"Content-Disposition: attachment; filename=broken.bin\r\n\r\n%%%\r\n--broken--\r\n")

	tests := []struct {
		name      string
		message   func() io.Reader
		configure func(*MailServer) func()
	}{
		{
			name: "message reader",
			message: func() io.Reader {
				return io.MultiReader(bytes.NewReader([]byte("From: sender@example.test\r\n\r\n")), errorReader{err: readFailure})
			},
			configure: func(*MailServer) func() { return func() {} },
		},
		{
			name:      "MIME parsing",
			message:   func() io.Reader { return bytes.NewReader(brokenMIME) },
			configure: func(*MailServer) func() { return func() {} },
		},
		{
			name:    "attachment staging write",
			message: func() io.Reader { return bytes.NewReader(multipartMessage()) },
			configure: func(server *MailServer) func() {
				server.beforeAttachmentWrite = func(string) error { return errors.New("injected staging failure") }
				return func() { server.beforeAttachmentWrite = nil }
			},
		},
		{
			name:    "atomic commit",
			message: func() io.Reader { return bytes.NewReader(validMessage("commit failure")) },
			configure: func(server *MailServer) func() {
				server.beforeStoreCommit = func(*Email) error { return errors.New("injected commit failure") }
				return func() { server.beforeStoreCommit = nil }
			},
		},
		{
			name:    "S3 upload and rollback",
			message: func() io.Reader { return bytes.NewReader(multipartMessage()) },
			configure: func(server *MailServer) func() {
				remote := newMemoryAttachmentStore()
				remote.putErr = errors.New("injected S3 failure")
				server.attachmentStore = remote
				return func() { remote.putErr = nil }
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, err := NewMailServerWithOptions(1025, "localhost", t.TempDir(), ServerOptions{MaxDataConcurrency: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = server.Close() }()
			recover := test.configure(server)
			if err := newDataTestSession(server).Data(test.message()); err == nil {
				t.Fatal("failing DATA transaction succeeded")
			}
			if got := len(server.dataLimiter.slots); got != 0 {
				t.Fatalf("DATA failure retained %d slot(s)", got)
			}
			recover()
			if err := newDataTestSession(server).Data(bytes.NewReader(validMessage("recovered"))); err != nil {
				t.Fatalf("DATA after failure did not recover: %v", err)
			}
		})
	}
}

func TestRejectedDataCreatesNoLocalOrRemoteArtifacts(t *testing.T) {
	directory := t.TempDir()
	remote := newMemoryAttachmentStore()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{
		MaxDataConcurrency: 1,
		AttachmentStore:    remote,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if !server.dataLimiter.tryAcquire() {
		t.Fatal("failed to occupy the only DATA slot")
	}
	defer server.dataLimiter.release()

	err = newDataTestSession(server).Data(bytes.NewReader(multipartMessage()))
	assertDataLimitError(t, err)
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("rejected DATA created %d index record(s)", got)
	}
	if artifacts := dataArtifactPaths(t, directory); len(artifacts) != 0 {
		t.Fatalf("rejected DATA left local artifacts: %v", artifacts)
	}
	if len(remote.objects) != 0 {
		t.Fatalf("rejected DATA left S3 objects: %v", remote.objects)
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

func startDataProtocolServers(t *testing.T, server *MailServer) (plainAddress, smtpsAddress string) {
	t.Helper()
	plainListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = plainListener.Close()
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() { done <- server.smtpServer.Serve(plainListener) }()
	go func() { done <- server.smtpsServer.Serve(tls.NewListener(tlsListener, server.smtpsServer.TLSConfig)) }()
	t.Cleanup(func() {
		_ = server.Close()
		for index := 0; index < 2; index++ {
			<-done
		}
	})
	return plainListener.Addr().String(), tlsListener.Addr().String()
}

func beginData(t *testing.T, client *smtp.Client, from string) io.WriteCloser {
	t.Helper()
	if err := client.Mail(from, nil); err != nil {
		t.Fatalf("MAIL FROM failed: %v", err)
	}
	if err := client.Rcpt("recipient@example.test", nil); err != nil {
		t.Fatalf("RCPT TO failed: %v", err)
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatalf("DATA command failed: %v", err)
	}
	return writer
}

func finishData(writer io.WriteCloser, subject string) error {
	_, writeErr := writer.Write(validMessage(subject))
	return errors.Join(writeErr, writer.Close())
}

func dataArtifactPaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, relative)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

func TestSMTPTransportsShareDataLimitAndRejectedBodyIsDrained(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "127.0.0.1", directory, ServerOptions{
		TLSConfig:          &TLSConfig{Enabled: true},
		MaxDataConcurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	plainAddress, smtpsAddress := startDataProtocolServers(t, server)

	holderEntered := make(chan struct{})
	releaseHolder := make(chan struct{})
	var blockFirst sync.Once
	server.afterDataAcquire = func() {
		blockFirst.Do(func() {
			close(holderEntered)
			<-releaseHolder
		})
	}

	plainClient, err := smtp.Dial(plainAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = plainClient.Close() }()
	holderResult := make(chan error, 1)
	holder := beginData(t, plainClient, "plain-holder@example.test")
	go func() { holderResult <- finishData(holder, "plain holder") }()
	<-holderEntered

	startTLSClient, err := smtp.DialStartTLS(plainAddress, insecureSMTPTestTLSConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = startTLSClient.Close() }()
	startTLSWriter := beginData(t, startTLSClient, "starttls-rejected@example.test")
	assertDataLimitError(t, finishData(startTLSWriter, "STARTTLS rejected"))

	smtpsClient, err := smtp.DialTLS(smtpsAddress, insecureSMTPTestTLSConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = smtpsClient.Close() }()
	smtpsWriter := beginData(t, smtpsClient, "smtps-rejected@example.test")
	assertDataLimitError(t, finishData(smtpsWriter, "SMTPS rejected"))

	if artifacts := dataArtifactPaths(t, directory); len(artifacts) != 0 {
		t.Fatalf("rejected DATA left artifacts: %v", artifacts)
	}
	close(releaseHolder)
	if err := <-holderResult; err != nil {
		t.Fatalf("plain holder failed: %v", err)
	}

	// Both rejected connections remain protocol-synchronized after go-smtp
	// drains their message bodies, so they can deliver the next transaction.
	if err := finishData(beginData(t, startTLSClient, "starttls-retry@example.test"), "STARTTLS retry"); err != nil {
		t.Fatalf("STARTTLS connection reuse failed: %v", err)
	}
	if err := finishData(beginData(t, smtpsClient, "smtps-retry@example.test"), "SMTPS retry"); err != nil {
		t.Fatalf("SMTPS connection reuse failed: %v", err)
	}
	if got := len(server.GetAllEmail()); got != 3 {
		t.Fatalf("stored messages = %d, want 3", got)
	}
}

func TestMessageSizeFailureReleasesDataSlot(t *testing.T) {
	server, err := NewMailServerWithOptions(1025, "127.0.0.1", t.TempDir(), ServerOptions{
		MaxMessageBytes:    512,
		MaxDataConcurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.smtpServer.Serve(listener) }()
	defer func() {
		_ = server.Close()
		<-done
	}()

	client, err := smtp.Dial(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	oversized := beginData(t, client, "oversized@example.test")
	if _, err := io.WriteString(oversized, "From: sender@example.test\r\nTo: recipient@example.test\r\n\r\n"+strings.Repeat("x", 1024)); err != nil {
		t.Fatal(err)
	}
	if err := oversized.Close(); err == nil {
		t.Fatal("oversized DATA succeeded")
	}
	if got := len(server.dataLimiter.slots); got != 0 {
		t.Fatalf("oversized DATA retained %d slot(s)", got)
	}
	if err := finishData(beginData(t, client, "after-size@example.test"), "after size failure"); err != nil {
		t.Fatalf("DATA after size failure failed: %v", err)
	}
}

func TestClientAbortAndServerCloseReleaseDataSlot(t *testing.T) {
	for _, closeServer := range []bool{false, true} {
		name := "client abort"
		if closeServer {
			name = "server close"
		}
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			server, err := NewMailServerWithOptions(1025, "127.0.0.1", directory, ServerOptions{MaxDataConcurrency: 1})
			if err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- server.smtpServer.Serve(listener) }()
			acquired := make(chan struct{})
			released := make(chan struct{})
			server.afterDataAcquire = func() { close(acquired) }
			server.afterDataRelease = func() { close(released) }

			client, err := smtp.Dial(listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			writer := beginData(t, client, "interrupted@example.test")
			if _, err := io.WriteString(writer, "From: sender@example.test\r\n\r\npartial"); err != nil {
				t.Fatal(err)
			}
			<-acquired
			if closeServer {
				_ = server.Close()
			} else {
				_ = client.Close()
			}
			<-released
			if got := len(server.dataLimiter.slots); got != 0 {
				t.Fatalf("interrupted DATA retained %d slot(s)", got)
			}
			if artifacts := dataArtifactPaths(t, directory); len(artifacts) != 0 {
				t.Fatalf("interrupted DATA left artifacts: %v", artifacts)
			}
			_ = client.Close()
			_ = server.Close()
			<-done
		})
	}
}
