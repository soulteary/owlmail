package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	clientsmtp "net/smtp"
	"strings"
	"sync"
	"testing"
	"time"

	smtp "github.com/emersion/go-smtp"
)

func TestHTTPFailuresAreIsolatedByDeliveryID(t *testing.T) {
	policy := &faultPolicy{attempts: make(map[string]int), failures: 2, ctx: context.Background()}
	receiver := httptest.NewServer(policy.httpHandler(503))
	defer receiver.Close()
	var wg sync.WaitGroup
	for _, id := range []string{"first", "second"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 1; n <= 3; n++ {
				req, err := http.NewRequest("POST", receiver.URL, strings.NewReader("{}"))
				if err != nil {
					t.Error(err)
					return
				}
				req.Header.Set("X-OwlMail-Delivery-ID", id)
				resp, err := receiver.Client().Do(req)
				if err != nil {
					t.Error(err)
					return
				}
				_ = resp.Body.Close()
				want := 503
				if n == 3 {
					want = 204
				}
				if resp.StatusCode != want || resp.Header.Get("X-Fault-Attempt") != fmt.Sprint(n) {
					t.Errorf("id=%s n=%d status=%d", id, n, resp.StatusCode)
				}
			}
		}()
	}
	wg.Wait()
}
func TestSMTPFailuresAtProtocolStages(t *testing.T) {
	for _, stage := range []string{"rcpt", "data"} {
		t.Run(stage, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			policy := &faultPolicy{attempts: make(map[string]int), failures: 1, ctx: context.Background()}
			server := smtp.NewServer(&faultBackend{policy: policy, code: 451, stage: stage})
			server.Domain = "localhost"
			server.ReadTimeout = time.Second
			server.WriteTimeout = time.Second
			done := make(chan error, 1)
			go func() { done <- server.Serve(listener) }()
			defer func() { _ = server.Close(); <-done }()
			for n := 1; n <= 2; n++ {
				err = clientsmtp.SendMail(listener.Addr().String(), nil, "sender@example.test", []string{"recipient@example.test"}, []byte("Subject: test\r\n\r\nhello\r\n"))
				if n == 1 && (err == nil || !strings.Contains(err.Error(), "451")) {
					t.Fatal("missing temporary failure", err)
				}
				if n == 2 && err != nil {
					t.Fatal("did not recover", err)
				}
			}
		})
	}
}
func TestDelayCanBeCanceledAndOptionsAreValidated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	policy := &faultPolicy{delay: 30 * time.Second, ctx: ctx}
	if err := policy.wait(context.Background()); err == nil {
		t.Fatal("delay ignored cancellation")
	}
	for _, args := range [][]string{{"-listen", "0.0.0.0:18081"}, {"-failures", "-1"}, {"-delay", "31s"}, {"-smtp-stage", "invalid"}} {
		if err := run(ctx, args); err == nil {
			t.Fatal(args)
		}
	}
}
