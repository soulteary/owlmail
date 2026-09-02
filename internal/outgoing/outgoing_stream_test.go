package outgoing

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

var errRelaySource = errors.New("relay source failed")

type relaySMTPResult struct {
	commands []string
	data     []byte
	bytes    int64
	accepted bool
	err      error
}

type relaySMTPDataHandler func(net.Conn, *bufio.Reader) ([]byte, int64, bool, error)

func startRelaySMTPServer(t *testing.T, handleData relaySMTPDataHandler) (string, <-chan relaySMTPResult) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	result := make(chan relaySMTPResult, 1)
	go func() {
		serverResult := relaySMTPResult{}
		defer func() { result <- serverResult }()
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverResult.err = acceptErr
			return
		}
		defer func() { _ = conn.Close() }()
		reader := bufio.NewReader(conn)
		if _, writeErr := fmt.Fprint(conn, "220 localhost ESMTP\r\n"); writeErr != nil {
			serverResult.err = writeErr
			return
		}
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				serverResult.err = readErr
				return
			}
			command := strings.TrimSpace(line)
			serverResult.commands = append(serverResult.commands, command)
			switch {
			case strings.HasPrefix(command, "EHLO "), strings.HasPrefix(command, "HELO "):
				_, serverResult.err = fmt.Fprint(conn, "250 localhost\r\n")
			case strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
				_, serverResult.err = fmt.Fprint(conn, "250 ok\r\n")
			case command == "DATA":
				if _, serverResult.err = fmt.Fprint(conn, "354 continue\r\n"); serverResult.err != nil {
					return
				}
				serverResult.data, serverResult.bytes, serverResult.accepted, serverResult.err = handleData(conn, reader)
				if serverResult.err != nil || !serverResult.accepted {
					return
				}
				_, serverResult.err = fmt.Fprint(conn, "250 queued\r\n")
			case command == "QUIT":
				_, serverResult.err = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, serverResult.err = fmt.Fprint(conn, "500 unexpected command\r\n")
			}
			if serverResult.err != nil {
				return
			}
		}
	}()
	return listener.Addr().String(), result
}

func readRelaySMTPData(_ net.Conn, reader *bufio.Reader) ([]byte, int64, bool, error) {
	var data bytes.Buffer
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return data.Bytes(), int64(data.Len()), false, err
		}
		if bytes.Equal(line, []byte(".\r\n")) {
			return data.Bytes(), int64(data.Len()), true, nil
		}
		_, _ = data.Write(line)
	}
}

func countRelaySMTPData(_ net.Conn, reader *bufio.Reader) ([]byte, int64, bool, error) {
	var total int64
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, total, false, err
		}
		if bytes.Equal(line, []byte(".\r\n")) {
			return nil, total, true, nil
		}
		total += int64(len(line))
	}
}

func TestSendMailContextStreamsThroughSMTPData(t *testing.T) {
	addr, result := startRelaySMTPServer(t, readRelaySMTPData)
	source := &trackingRelayReadCloser{Reader: strings.NewReader("Subject: stream\n\n.leading dot\nbody")}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := sendMailContext(ctx, addr, nil, "", []string{"one@example.test", "two@example.test"}, source, false, false); err != nil {
		t.Fatalf("sendMailContext() error = %v", err)
	}
	serverResult := <-result
	if serverResult.err != nil {
		t.Fatalf("SMTP server error = %v", serverResult.err)
	}
	wantData := "Subject: stream\r\n\r\n..leading dot\r\nbody\r\n"
	if got := string(serverResult.data); got != wantData {
		t.Fatalf("SMTP DATA = %q, want %q", got, wantData)
	}
	if !serverResult.accepted {
		t.Fatal("SMTP server did not receive the DATA terminator")
	}
	if source.closes.Load() != 1 {
		t.Fatalf("source close count = %d, want 1", source.closes.Load())
	}
	assertRelayCommand(t, serverResult.commands, "MAIL FROM:<>")
	assertRelayCommand(t, serverResult.commands, "RCPT TO:<one@example.test>")
	assertRelayCommand(t, serverResult.commands, "RCPT TO:<two@example.test>")
}

func TestSendMailContextStreamsLargeMessage(t *testing.T) {
	addr, result := startRelaySMTPServer(t, countRelaySMTPData)
	line := strings.Repeat("x", 1022) + "\r\n"
	message := strings.Repeat(line, 8*1024)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sendMailContext(ctx, addr, nil, "sender@example.test", []string{"recipient@example.test"}, io.NopCloser(strings.NewReader(message)), false, false); err != nil {
		t.Fatalf("sendMailContext() error = %v", err)
	}
	serverResult := <-result
	if serverResult.err != nil {
		t.Fatalf("SMTP server error = %v", serverResult.err)
	}
	if serverResult.bytes != int64(len(message)) {
		t.Fatalf("streamed bytes = %d, want %d", serverResult.bytes, len(message))
	}
}

func TestRelayEmailPreservesHELOOnlyAuthBehavior(t *testing.T) {
	addr, result := startHELOOnlyRelaySMTPServer(t)
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	messagePath := t.TempDir() + "/message.eml"
	if err := os.WriteFile(messagePath, []byte("Subject: HELO only\r\n\r\nbody"), 0o600); err != nil {
		t.Fatal(err)
	}
	om := &OutgoingMail{
		config:  &OutgoingConfig{Host: host, Port: port, User: "configured", Password: "secret"},
		enabled: true,
	}
	err = om.relayEmail(&RelayTask{
		Email: &types.Email{
			Subject:  "HELO only",
			Envelope: &types.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}},
		},
		EmailPath: messagePath,
	})
	if err != nil {
		t.Fatalf("relayEmail() error = %v", err)
	}
	serverResult := <-result
	if serverResult.err != nil {
		t.Fatalf("SMTP server error = %v", serverResult.err)
	}
	for _, command := range serverResult.commands {
		if strings.HasPrefix(command, "AUTH ") {
			t.Fatalf("HELO-only SMTP commands unexpectedly contain %q", command)
		}
	}
	if !serverResult.accepted {
		t.Fatal("HELO-only server did not accept the message")
	}
}

func TestSendMailContextSourceFailureAbortsData(t *testing.T) {
	addr, result := startRelaySMTPServer(t, readRelaySMTPData)
	source := &failingRelayReadCloser{data: []byte("Subject: partial\r\n\r\nbody")}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := sendMailContext(ctx, addr, nil, "sender@example.test", []string{"recipient@example.test"}, source, false, false)
	if !errors.Is(err, errRelaySource) {
		t.Fatalf("sendMailContext() error = %v, want source failure", err)
	}
	serverResult := <-result
	if serverResult.accepted {
		t.Fatal("partial message was terminated and accepted")
	}
	if source.closes.Load() != 1 {
		t.Fatalf("source close count = %d, want 1", source.closes.Load())
	}
}

func TestSendMailContextServerClosesDuringData(t *testing.T) {
	addr, result := startRelaySMTPServer(t, func(conn net.Conn, _ *bufio.Reader) ([]byte, int64, bool, error) {
		_ = conn.Close()
		return nil, 0, false, errors.New("server closed during DATA")
	})
	message := strings.Repeat("message data\r\n", 64*1024)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := sendMailContext(ctx, addr, nil, "sender@example.test", []string{"recipient@example.test"}, io.NopCloser(strings.NewReader(message)), false, false)
	if err == nil {
		t.Fatal("sendMailContext() error = nil, want DATA write failure")
	}
	<-result
}

func TestSendMailContextCancellationInterruptsSourceRead(t *testing.T) {
	addr, result := startRelaySMTPServer(t, readRelaySMTPData)
	source := newBlockingRelayReadCloser()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- sendMailContext(ctx, addr, nil, "sender@example.test", []string{"recipient@example.test"}, source, false, false)
	}()
	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("relay source read did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("sendMailContext() error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled DATA stream did not stop")
	}
	if source.closes.Load() != 1 {
		t.Fatalf("source close count = %d, want 1", source.closes.Load())
	}
	<-result
}

func TestSendMailContextDeadlineInterruptsSourceRead(t *testing.T) {
	addr, result := startRelaySMTPServer(t, readRelaySMTPData)
	source := newBlockingRelayReadCloser()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := sendMailContext(ctx, addr, nil, "sender@example.test", []string{"recipient@example.test"}, source, false, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sendMailContext() error = %v, want deadline exceeded", err)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("deadline took too long: %s", time.Since(started))
	}
	<-result
}

func TestCopyRelayMessageWriteFailure(t *testing.T) {
	_, err := copyRelayMessage(context.Background(), errorRelayWriter{}, strings.NewReader("message"))
	if !errors.Is(err, errRelayWrite) {
		t.Fatalf("copyRelayMessage() error = %v, want write failure", err)
	}
}

func TestSendMailContextClosesSourceWhenDialFails(t *testing.T) {
	source := &trackingRelayReadCloser{Reader: strings.NewReader("message")}
	err := sendMailContext(context.Background(), "invalid:address", nil, "", nil, source, false, false)
	if err == nil {
		t.Fatal("sendMailContext() error = nil, want address error")
	}
	if source.closes.Load() != 1 {
		t.Fatalf("source close count = %d, want 1", source.closes.Load())
	}
}

func BenchmarkCopyRelayMessage(b *testing.B) {
	for _, size := range []int64{1 << 20, 64 << 20} {
		b.Run(fmt.Sprintf("%dMiB", size>>20), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(size)
			for range b.N {
				if _, err := copyRelayMessage(context.Background(), io.Discard, io.LimitReader(zeroRelayReader{}, size)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func assertRelayCommand(t *testing.T, commands []string, want string) {
	t.Helper()
	for _, command := range commands {
		if command == want {
			return
		}
	}
	t.Fatalf("SMTP commands = %v, missing %q", commands, want)
}

func startHELOOnlyRelaySMTPServer(t *testing.T) (string, <-chan relaySMTPResult) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	result := make(chan relaySMTPResult, 1)
	go func() {
		serverResult := relaySMTPResult{}
		defer func() { result <- serverResult }()
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverResult.err = acceptErr
			return
		}
		defer func() { _ = conn.Close() }()
		reader := bufio.NewReader(conn)
		if _, serverResult.err = fmt.Fprint(conn, "220 localhost SMTP\r\n"); serverResult.err != nil {
			return
		}
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				serverResult.err = readErr
				return
			}
			command := strings.TrimSpace(line)
			serverResult.commands = append(serverResult.commands, command)
			switch {
			case strings.HasPrefix(command, "EHLO "):
				_, serverResult.err = fmt.Fprint(conn, "502 EHLO unsupported\r\n")
			case strings.HasPrefix(command, "HELO "), strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
				_, serverResult.err = fmt.Fprint(conn, "250 ok\r\n")
			case strings.HasPrefix(command, "AUTH "):
				_, serverResult.err = fmt.Fprint(conn, "504 AUTH unsupported\r\n")
			case command == "DATA":
				if _, serverResult.err = fmt.Fprint(conn, "354 continue\r\n"); serverResult.err != nil {
					return
				}
				serverResult.data, serverResult.bytes, serverResult.accepted, serverResult.err = readRelaySMTPData(conn, reader)
				if serverResult.err != nil {
					return
				}
				_, serverResult.err = fmt.Fprint(conn, "250 queued\r\n")
			case command == "QUIT":
				_, serverResult.err = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, serverResult.err = fmt.Fprint(conn, "500 unexpected command\r\n")
			}
			if serverResult.err != nil {
				return
			}
		}
	}()
	return listener.Addr().String(), result
}

type trackingRelayReadCloser struct {
	io.Reader
	closes atomic.Int32
}

func (reader *trackingRelayReadCloser) Close() error {
	reader.closes.Add(1)
	return nil
}

type failingRelayReadCloser struct {
	data    []byte
	emitted bool
	closes  atomic.Int32
}

func (reader *failingRelayReadCloser) Read(buffer []byte) (int, error) {
	if reader.emitted {
		return 0, errRelaySource
	}
	reader.emitted = true
	return copy(buffer, reader.data), nil
}

func (reader *failingRelayReadCloser) Close() error {
	reader.closes.Add(1)
	return nil
}

type blockingRelayReadCloser struct {
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
	closes  atomic.Int32
}

func newBlockingRelayReadCloser() *blockingRelayReadCloser {
	return &blockingRelayReadCloser{started: make(chan struct{}), closed: make(chan struct{})}
}

func (reader *blockingRelayReadCloser) Read([]byte) (int, error) {
	reader.once.Do(func() { close(reader.started) })
	<-reader.closed
	return 0, errors.New("relay source closed")
}

func (reader *blockingRelayReadCloser) Close() error {
	if reader.closes.Add(1) == 1 {
		close(reader.closed)
	}
	return nil
}

var errRelayWrite = errors.New("relay DATA write failed")

type errorRelayWriter struct{}

func (errorRelayWriter) Write([]byte) (int, error) {
	return 0, errRelayWrite
}

type zeroRelayReader struct{}

func (zeroRelayReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 'x'
	}
	return len(buffer), nil
}
