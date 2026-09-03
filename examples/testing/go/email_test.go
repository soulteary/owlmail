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

func sendSMTPMessage(address, from string, recipients []string, message []byte, timeout time.Duration, onAccepted func()) error {
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
	onAccepted()
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
		if _, drainErr := io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10)); drainErr != nil {
			err = errors.Join(err, fmt.Errorf("drain cleanup response: %w", drainErr))
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close cleanup response: %w", closeErr))
		}
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("delete email: unexpected status %d", response.StatusCode)
	}
	return nil
}

func findMatchingEmailIDs(client *http.Client, apiBase, recipient, subject string) (ids []string, err error) {
	endpoint := apiBase + "/api/v1/emails?to=" + url.QueryEscape(recipient) + "&limit=10"
	response, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("list emails: %w", err)
	}
	defer func() {
		if _, drainErr := io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10)); drainErr != nil {
			err = errors.Join(err, fmt.Errorf("drain email list response: %w", drainErr))
		}
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close email list response: %w", closeErr))
		}
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list emails: unexpected status %d", response.StatusCode)
	}
	var page emailPage
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode email list: %w", err)
	}
	for _, item := range page.Emails {
		if item.Subject == subject {
			ids = append(ids, item.ID)
		}
	}
	return ids, nil
}

func cleanupCapturedEmail(client *http.Client, apiBase, recipient, subject, knownID string) error {
	ids := []string{knownID}
	if knownID == "" {
		var err error
		ids, err = findMatchingEmailIDs(client, apiBase, recipient, subject)
		if err != nil {
			return err
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("cleanup could not locate the accepted message for %s", recipient)
	}
	var cleanupErr error
	for _, id := range ids {
		messageURL := apiBase + "/api/v1/emails/" + url.PathEscape(id)
		if err := deleteEmail(client, messageURL); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
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
	client := &http.Client{Timeout: 5 * time.Second}
	accepted := false
	var id string
	t.Cleanup(func() {
		if !accepted {
			return
		}
		if err := cleanupCapturedEmail(client, apiBase, recipient, subject, id); err != nil {
			t.Errorf("cleanup captured email: %v", err)
		}
	})
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
		func() { accepted = true },
	); err != nil {
		t.Fatalf("send test email: %v", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		ids, err := findMatchingEmailIDs(client, apiBase, recipient, subject)
		if err != nil {
			t.Fatalf("find email: %v", err)
		}
		if len(ids) != 0 {
			id = ids[0]
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if id == "" {
		t.Fatalf("timed out waiting for %s", recipient)
	}

	messageURL := apiBase + "/api/v1/emails/" + url.PathEscape(id)
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

type trackingBody struct {
	reader *strings.Reader
	reads  int
}

func (body *trackingBody) Read(buffer []byte) (int, error) {
	body.reads++
	return body.reader.Read(buffer)
}

func (*trackingBody) Close() error { return nil }

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

func TestDeleteEmailDrainsNonSuccessResponse(t *testing.T) {
	body := &trackingBody{reader: strings.NewReader("cleanup failure details")}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: body}, nil
	})}

	err := deleteEmail(client, "http://owlmail.test/api/v1/emails/id")
	if err == nil || !strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("deleteEmail() error = %v", err)
	}
	if body.reads == 0 || body.reader.Len() != 0 {
		t.Fatalf("error response body was not drained: reads=%d remaining=%d", body.reads, body.reader.Len())
	}
}

func TestCleanupCapturedEmailFindsUnknownID(t *testing.T) {
	var deletedPath string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.Method {
		case http.MethodGet:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"emails":[{"id":"other-id","subject":"other"},{"id":"captured-id","subject":"subject"}]}`)),
			}, nil
		case http.MethodDelete:
			deletedPath = request.URL.Path
			return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
		default:
			return nil, fmt.Errorf("unexpected method %s", request.Method)
		}
	})}

	if err := cleanupCapturedEmail(
		client,
		"http://owlmail.test",
		"recipient@example.test",
		"subject",
		"",
	); err != nil {
		t.Fatalf("cleanupCapturedEmail() error = %v", err)
	}
	if deletedPath != "/api/v1/emails/captured-id" {
		t.Fatalf("deleted path = %q", deletedPath)
	}
}
