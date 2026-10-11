// Command faults runs a loopback-only deterministic HTTP or SMTP fault receiver.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	smtp "github.com/emersion/go-smtp"
)

type faultPolicy struct {
	mu       sync.Mutex
	attempts map[string]int
	failures int
	delay    time.Duration
	ctx      context.Context
}

func (p *faultPolicy) attempt(key string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(key) > 1024 {
		return 0, fmt.Errorf("fault key exceeds 1024 bytes")
	}
	if _, exists := p.attempts[key]; !exists && len(p.attempts) >= 10000 {
		return 0, fmt.Errorf("fault key capacity reached; restart receiver")
	}
	p.attempts[key]++
	return p.attempts[key], nil
}
func (p *faultPolicy) wait(ctx context.Context) error {
	if p.delay == 0 {
		return nil
	}
	timer := time.NewTimer(p.delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}
func (p *faultPolicy) httpHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.Header().Set("Allow", "POST")
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		body := http.MaxBytesReader(w, r.Body, 2<<20)
		defer func() { _ = body.Close() }()
		if _, err := io.Copy(io.Discard, body); err != nil {
			http.Error(w, "unreadable or oversized body", http.StatusRequestEntityTooLarge)
			return
		}
		key := r.Header.Get("X-OwlMail-Delivery-ID")
		if key == "" {
			http.Error(w, "X-OwlMail-Delivery-ID required", 400)
			return
		}
		attempt, err := p.attempt(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
			return
		}
		w.Header().Set("X-Fault-Attempt", fmt.Sprint(attempt))
		if err = p.wait(r.Context()); err != nil {
			return
		}
		if attempt <= p.failures {
			w.WriteHeader(status)
			return
		}
		w.WriteHeader(204)
	})
}

type faultBackend struct {
	policy *faultPolicy
	code   int
	stage  string
}

func (b *faultBackend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &faultSession{backend: b}, nil
}

type faultSession struct {
	backend   *faultBackend
	recipient string
}

func (s *faultSession) Mail(_ string, _ *smtp.MailOptions) error { return nil }
func (s *faultSession) Rcpt(to string, _ *smtp.RcptOptions) error {
	if s.backend.stage == "rcpt" {
		if err := s.apply(to); err != nil {
			return err
		}
	}
	s.recipient = to
	return nil
}
func (s *faultSession) Data(r io.Reader) error {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return err
	}
	if s.backend.stage == "data" {
		return s.apply(s.recipient)
	}
	return nil
}
func (s *faultSession) apply(key string) error {
	n, err := s.backend.policy.attempt(key)
	if err != nil {
		return &smtp.SMTPError{Code: 452, Message: "fault receiver capacity reached"}
	}
	if err = s.backend.policy.wait(s.backend.policy.ctx); err != nil {
		return &smtp.SMTPError{Code: 421, Message: "fault receiver shutting down"}
	}
	if n <= s.backend.policy.failures {
		return &smtp.SMTPError{Code: s.backend.code, Message: fmt.Sprintf("injected failure at attempt %d", n)}
	}
	return nil
}
func (s *faultSession) Reset()        { s.recipient = "" }
func (s *faultSession) Logout() error { return nil }

func run(ctx context.Context, args []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	fs := flag.NewFlagSet("faults", flag.ContinueOnError)
	protocol := fs.String("protocol", "http", "http or smtp")
	listen := fs.String("listen", "127.0.0.1:18081", "loopback address and port")
	failures := fs.Int("failures", 2, "first N attempts per delivery ID or SMTP recipient to reject")
	delay := fs.Duration("delay", 0, "delay every response (0..30s)")
	httpStatus := fs.Int("http-status", 503, "injected HTTP response (400..599)")
	smtpCode := fs.Int("smtp-code", 451, "injected SMTP response (400..599)")
	stage := fs.String("smtp-stage", "data", "SMTP failure stage: rcpt or data (data keys the last recipient)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *failures < 0 || *failures > 1000 || *delay < 0 || *delay > 30*time.Second || *httpStatus < 400 || *httpStatus > 599 || *smtpCode < 400 || *smtpCode > 599 || (*stage != "rcpt" && *stage != "data") || (*protocol != "http" && *protocol != "smtp") {
		return fmt.Errorf("invalid fault receiver options")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("listen host must be a literal loopback IP")
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	policy := &faultPolicy{attempts: make(map[string]int), failures: *failures, delay: *delay, ctx: ctx}
	log.Printf("fault receiver protocol=%s address=%s first_failures=%d delay=%s", *protocol, listener.Addr(), *failures, *delay)
	if *protocol == "http" {
		server := &http.Server{Handler: policy.httpHandler(*httpStatus), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 35 * time.Second}
		go func() { <-ctx.Done(); _ = server.Close() }()
		err = server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
	server := smtp.NewServer(&faultBackend{policy: policy, code: *smtpCode, stage: *stage})
	server.Domain = "faults.local"
	server.MaxMessageBytes = 2 << 20
	server.MaxRecipients = 50
	server.ReadTimeout = 10 * time.Second
	server.WriteTimeout = 35 * time.Second
	go func() { <-ctx.Done(); _ = server.Close() }()
	err = server.Serve(listener)
	if ctx.Err() != nil {
		return nil
	}
	return err
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
