package api

import (
	"encoding/json"
	"io"
	"net"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	smtp "github.com/emersion/go-smtp"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/outgoing"
)

type retryFaultBackend struct {
	attempts atomic.Int32
	code     int
}

func (b *retryFaultBackend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &retryFaultSession{backend: b}, nil
}

type retryFaultSession struct{ backend *retryFaultBackend }

func (s *retryFaultSession) Mail(_ string, _ *smtp.MailOptions) error { return nil }
func (s *retryFaultSession) Rcpt(_ string, _ *smtp.RcptOptions) error { return nil }
func (s *retryFaultSession) Data(reader io.Reader) error {
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return err
	}
	n := s.backend.attempts.Add(1)
	if n <= 2 {
		return &smtp.SMTPError{Code: s.backend.code, Message: "injected fault"}
	}
	return nil
}
func (s *retryFaultSession) Reset()        {}
func (s *retryFaultSession) Logout() error { return nil }

func TestRelayQueueRecoversSMTP4xxButDoesNotRetry5xx(t *testing.T) {
	for _, code := range []int{451, 550} {
		t.Run((&textproto.Error{Code: code}).Error(), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			backend := &retryFaultBackend{code: code}
			receiver := smtp.NewServer(backend)
			receiver.Domain = "localhost"
			receiver.ReadTimeout = time.Second
			receiver.WriteTimeout = time.Second
			done := make(chan error, 1)
			go func() { done <- receiver.Serve(listener) }()
			defer func() { _ = receiver.Close(); <-done }()
			api, server, dir := setupTestAPI(t)
			defer func() { _ = server.Close() }()
			if err = server.SetOutgoingConfig(&outgoing.OutgoingConfig{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port}); err != nil {
				t.Fatal(err)
			}
			email := &mailserver.Email{Subject: "Fault recovery"}
			if err = server.SaveEmailToStore("fault", false, &mailserver.Envelope{From: "sender@example.test", To: []string{"recipient@example.test"}}, email); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "fault.eml"), []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: Fault recovery\r\n\r\nBody\r\n"), 0600); err != nil {
				t.Fatal(err)
			}
			resp, err := api.app.Test(httptest.NewRequest("POST", "/api/v1/emails/fault/actions/relay", nil))
			if err != nil {
				t.Fatal(err)
			}
			var accepted struct {
				Data struct {
					Job relayJob `json:"job"`
				} `json:"data"`
			}
			err = json.NewDecoder(resp.Body).Decode(&accepted)
			_ = resp.Body.Close()
			if err != nil || resp.StatusCode != 202 {
				t.Fatal(err, resp.StatusCode)
			}
			deadline := time.Now().Add(4 * time.Second)
			for time.Now().Before(deadline) {
				job, _ := api.relayService.Get(accepted.Data.Job.ID)
				if job.CompletedAt != nil {
					if code == 451 && (job.Status != relayJobSucceeded || job.Attempts != 3 || backend.attempts.Load() != 3) {
						t.Fatal(job, backend.attempts.Load())
					}
					if code == 550 && (job.Status != relayJobFailed || job.ErrorCategory != "permanent_smtp" || backend.attempts.Load() != 1) {
						t.Fatal(job, backend.attempts.Load())
					}
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("relay did not reach terminal state")
		})
	}
}
