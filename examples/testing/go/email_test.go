package testingexample

import (
	"encoding/json"
	"fmt"
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

func TestCapturedEmail(t *testing.T) {
	smtpAddress := environment("TEST_SMTP_HOST", "127.0.0.1") + ":" + environment("TEST_SMTP_PORT", "1025")
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
	if err := smtp.SendMail(smtpAddress, nil, "sender@example.test", []string{recipient}, []byte(message)); err != nil {
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
		response.Body.Close()
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
		request, _ := http.NewRequest(http.MethodDelete, messageURL, nil)
		if response, err := client.Do(request); err == nil {
			response.Body.Close()
		}
	})
	response, err := client.Get(messageURL)
	if err != nil {
		t.Fatalf("get email: %v", err)
	}
	defer response.Body.Close()
	var detail emailDetail
	if err := json.NewDecoder(response.Body).Decode(&detail); err != nil {
		t.Fatalf("decode email: %v", err)
	}
	if detail.Subject != subject || !strings.Contains(detail.Text, token) {
		t.Fatalf("unexpected email: %#v", detail)
	}
}
