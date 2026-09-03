package testingexample

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type emailPage struct {
	Emails []struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
	} `json:"emails"`
}

type emailDetail struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func joinSMTPAddress(host, port string) string {
	return net.JoinHostPort(host, port)
}

func sendSMTPMessage(address, from string, recipients []string, message []byte, timeout time.Duration) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("parse SMTP address: %w", err)
	}
	connection, err := (&net.Dialer{Timeout: timeout}).Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("connect to SMTP: %w", err)
	}
	if err := connection.SetDeadline(time.Now().Add(timeout)); err != nil {
		_ = connection.Close()
		return fmt.Errorf("set SMTP deadline: %w", err)
	}

	client, err := smtp.NewClient(connection, host)
	if err != nil {
		_ = connection.Close()
		return fmt.Errorf("read SMTP greeting: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("send MAIL command: %w", err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("send RCPT command: %w", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP DATA: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		return fmt.Errorf("write SMTP DATA: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP DATA: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit SMTP session: %w", err)
	}
	return nil
}

func deleteEmail(client *http.Client, messageURL string) (err error) {
	request, err := http.NewRequest(http.MethodDelete, messageURL, nil)
	if err != nil {
		return fmt.Errorf("create cleanup request: %w", err)
	}
	cleanupClient := *client
	cleanupClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := cleanupClient.Do(request)
	if err != nil {
		return fmt.Errorf("delete email: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close cleanup response: %w", closeErr))
		}
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("delete email: unexpected status %d", response.StatusCode)
	}
	return nil
}

func TestCapturedEmail(t *testing.T) {
	if os.Getenv("OWLMAIL_RUN_INTEGRATION_TEST") != "1" {
		t.Skip("set OWLMAIL_RUN_INTEGRATION_TEST=1 to run against a live OwlMail instance")
	}

	smtpAddress := joinSMTPAddress(
		environment("TEST_SMTP_HOST", "127.0.0.1"),
		environment("TEST_SMTP_PORT", "1025"),
	)
	apiBase := strings.TrimRight(environment("TEST_MAIL_API", "http://127.0.0.1:1080"), "/")
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	recipient := "signup+" + runID + "@example.test"
	subject := "OwlMail integration " + runID
	token := "token-" + runID
	message := strings.Join([]string{
		"From: sender@example.test",
		"To: " + recipient,
		"Subject: " + subject,
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Verification token: " + token,
	}, "\r\n")
	if err := sendSMTPMessage(
		smtpAddress,
		"sender@example.test",
		[]string{recipient},
		[]byte(message),
		5*time.Second,
	); err != nil {
		t.Fatalf("send test email: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(15 * time.Second)
	var id string
	for time.Now().Before(deadline) {
		endpoint := apiBase + "/api/v1/emails?to=" + url.QueryEscape(recipient) + "&limit=10"
		response, err := client.Get(endpoint)
		if err != nil {
			t.Fatalf("list emails: %v", err)
		}
		var page emailPage
		err = json.NewDecoder(response.Body).Decode(&page)
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Fatalf("close email list response: %v", closeErr)
		}
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("decode email list: status=%d err=%v", response.StatusCode, err)
		}
		for _, item := range page.Emails {
			if item.Subject == subject {
				id = item.ID
				break
			}
		}
		if id != "" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if id == "" {
		t.Fatalf("timed out waiting for %s", recipient)
	}

	messageURL := apiBase + "/api/v1/emails/" + url.PathEscape(id)
	t.Cleanup(func() {
		if err := deleteEmail(client, messageURL); err != nil {
			t.Errorf("cleanup captured email: %v", err)
		}
	})
	response, err := client.Get(messageURL)
	if err != nil {
		t.Fatalf("get email: %v", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("close email response: %v", closeErr)
		}
	}()
	var detail emailDetail
	if err := json.NewDecoder(response.Body).Decode(&detail); err != nil {
		t.Fatalf("decode email: %v", err)
	}
	if detail.Subject != subject || !strings.Contains(detail.Text, token) {
		t.Fatalf("unexpected email: %#v", detail)
	}
}

func TestSMTPAddressSupportsHostnamesAndIPAddresses(t *testing.T) {
	tests := map[string]struct {
		host string
		want string
	}{
		"hostname": {host: "owlmail", want: "owlmail:1025"},
		"IPv4":     {host: "127.0.0.1", want: "127.0.0.1:1025"},
		"IPv6":     {host: "::1", want: "[::1]:1025"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := joinSMTPAddress(test.host, "1025"); got != test.want {
				t.Fatalf("joinSMTPAddress(%q, %q) = %q, want %q", test.host, "1025", got, test.want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type closeErrorBody struct{}

func (closeErrorBody) Read([]byte) (int, error) { return 0, io.EOF }
func (closeErrorBody) Close() error             { return errors.New("close failed") }

func TestDeleteEmailReportsCleanupFailures(t *testing.T) {
	tests := map[string]struct {
		messageURL string
		transport  roundTripFunc
		wantError  string
	}{
		"invalid URL": {
			messageURL: "://invalid",
			wantError:  "create cleanup request",
		},
		"transport failure": {
			messageURL: "http://owlmail.test/api/v1/emails/id",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("connection lost")
			},
			wantError: "connection lost",
		},
		"non-success status": {
			messageURL: "http://owlmail.test/api/v1/emails/id",
			transport: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
			wantError: "unexpected status 500",
		},
		"redirect": {
			messageURL: "http://owlmail.test/api/v1/emails/id",
			transport: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusFound,
					Header:     http.Header{"Location": []string{"/login"}},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
			wantError: "unexpected status 302",
		},
		"response close failure": {
			messageURL: "http://owlmail.test/api/v1/emails/id",
			transport: func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       closeErrorBody{},
				}, nil
			},
			wantError: "close cleanup response: close failed",
		},
		"success": {
			messageURL: "http://owlmail.test/api/v1/emails/id",
			transport: func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodDelete {
					return nil, fmt.Errorf("method = %s", request.Method)
				}
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := &http.Client{}
			if test.transport != nil {
				client.Transport = test.transport
			}
			err := deleteEmail(client, test.messageURL)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("deleteEmail() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("deleteEmail() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}
