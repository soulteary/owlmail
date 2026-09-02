package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestLatestAndEventDrivenWaitReturnBoundedSummaries(t *testing.T) {
	mailbox := newTestMailbox(t)
	service, err := New(mailbox, Options{
		SessionTimeout: time.Minute, ShutdownTimeout: time.Second,
		WebBaseURL: "https://mail.example.test/owlmail",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)
	session := connectTestClient(t, httpServer.URL)
	t.Cleanup(func() { _ = session.Close() })
	if err := mailbox.SaveEmailToStore("newest-with-stale-date", false,
		&mailserver.Envelope{To: []string{"inbox@example.test"}},
		&mailserver.Email{
			Time: time.Now().Add(-24 * time.Hour), Subject: "Newest delivery with stale Date",
			To: []*mail.Address{{Address: "inbox@example.test"}},
		}); err != nil {
		t.Fatal(err)
	}

	var latest latestEmailOutput
	callTool(t, session, "get_latest_email", map[string]any{"limit": 2}, &latest)
	if len(latest.Emails) != 2 {
		t.Fatalf("latest emails = %d, want 2", len(latest.Emails))
	}
	if latest.Emails[0].ID != "newest-with-stale-date" {
		t.Fatalf("latest email = %q, want newest delivery despite stale Date header", latest.Emails[0].ID)
	}
	for _, email := range latest.Emails {
		if email.WebURL != "https://mail.example.test/owlmail/?email="+email.ID {
			t.Fatalf("deep link = %q", email.WebURL)
		}
	}
	encodedLatest, _ := json.Marshal(latest)
	if !strings.Contains(string(encodedLatest), `"sizeHuman"`) || !strings.Contains(string(encodedLatest), `"hasAttachment"`) ||
		strings.Contains(string(encodedLatest), `"size_human"`) || strings.Contains(string(encodedLatest), `"has_attachment"`) {
		t.Fatalf("summary field compatibility changed: %s", encodedLatest)
	}

	type callResult struct {
		result *mcp.CallToolResult
		err    error
	}
	waited := make(chan callResult, 1)
	go func() {
		result, callErr := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "wait_for_email",
			Arguments: map[string]any{
				"to": "new@example.test", "subject": "Verify", "text": "code-42", "timeout_seconds": 5,
			},
		})
		waited <- callResult{result: result, err: callErr}
	}()
	waitForWaiterCount(t, service, 1)

	if err := mailbox.SaveEmailToStore("ignored", false,
		&mailserver.Envelope{To: []string{"other@example.test"}},
		&mailserver.Email{Subject: "Verify", Text: "code-42", To: []*mail.Address{{Address: "other@example.test"}}}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if service.waiters.count() != 1 {
		t.Fatal("non-matching delivery consumed the waiter")
	}
	if err := mailbox.SaveEmailToStore("matched", false,
		&mailserver.Envelope{To: []string{"new@example.test"}},
		&mailserver.Email{
			Subject: "Please Verify", Text: "use code-42", HTML: "<b>secret HTML code-42</b>",
			To:          []*mail.Address{{Address: "new@example.test"}},
			Attachments: []*mailserver.Attachment{{FileName: "secret.bin", GeneratedFileName: "secret.bin"}},
		}); err != nil {
		t.Fatal(err)
	}

	call := <-waited
	if call.err != nil || call.result == nil || call.result.IsError {
		t.Fatalf("wait_for_email result = %#v, error = %v", call.result, call.err)
	}
	encoded, _ := json.Marshal(call.result.StructuredContent)
	var output waitForEmailOutput
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if !output.Matched || output.TimedOut || output.Email == nil || output.Email.ID != "matched" {
		t.Fatalf("wait output = %#v", output)
	}
	if output.Email.WebURL != "https://mail.example.test/owlmail/?email=matched" {
		t.Fatalf("wait deep link = %q", output.Email.WebURL)
	}
	if strings.Contains(string(encoded), "secret HTML") || strings.Contains(string(encoded), "secret.bin") || strings.Contains(string(encoded), "source") {
		t.Fatalf("wait output crossed the bounded summary boundary: %s", encoded)
	}
	waitForWaiterCount(t, service, 0)
}

func TestWaitCancellationTimeoutSessionBoundAndShutdownCleanup(t *testing.T) {
	t.Run("client cancellation", func(t *testing.T) {
		mailbox := newTestMailbox(t)
		service := newTestService(t, mailbox, time.Minute)
		httpServer := httptest.NewServer(service)
		t.Cleanup(httpServer.Close)
		session := connectTestClient(t, httpServer.URL)
		t.Cleanup(func() { _ = session.Close() })

		ctx, cancel := context.WithCancel(context.Background())
		finished := make(chan error, 1)
		go func() {
			_, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "wait_for_email", Arguments: map[string]any{"to": "never", "timeout_seconds": 5}})
			finished <- err
		}()
		waitForWaiterCount(t, service, 1)
		cancel()
		if err := <-finished; err == nil {
			t.Fatal("canceled wait unexpectedly succeeded")
		}
		waitForWaiterCount(t, service, 0)
	})

	t.Run("explicit timeout", func(t *testing.T) {
		mailbox := newTestMailbox(t)
		service := newTestService(t, mailbox, time.Minute)
		httpServer := httptest.NewServer(service)
		t.Cleanup(httpServer.Close)
		session := connectTestClient(t, httpServer.URL)
		t.Cleanup(func() { _ = session.Close() })
		var output waitForEmailOutput
		callTool(t, session, "wait_for_email", map[string]any{"to": "never", "timeout_seconds": 1}, &output)
		if output.Matched || !output.TimedOut || output.WaitedMillis < 800 {
			t.Fatalf("timeout output = %#v", output)
		}
		waitForWaiterCount(t, service, 0)
	})

	t.Run("session timeout bounds active wait", func(t *testing.T) {
		mailbox := newTestMailbox(t)
		service := newTestService(t, mailbox, 80*time.Millisecond)
		started := time.Now()
		output, err := service.waitForEmail(context.Background(), nil, waitForEmailInput{To: "never"})
		if err != nil || !output.TimedOut || time.Since(started) > time.Second {
			t.Fatalf("session-bounded wait = %#v, %v", output, err)
		}
		waitForWaiterCount(t, service, 0)
	})

	t.Run("service shutdown", func(t *testing.T) {
		mailbox := newTestMailbox(t)
		service, err := New(mailbox, Options{SessionTimeout: time.Minute, ShutdownTimeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		finished := make(chan error, 1)
		go func() {
			_, waitErr := service.waitForEmail(context.Background(), nil, waitForEmailInput{To: "never", TimeoutSeconds: 5})
			finished <- waitErr
		}()
		waitForWaiterCount(t, service, 1)
		if err := service.Close(); err != nil {
			t.Fatal(err)
		}
		if err := <-finished; err == nil || !strings.Contains(err.Error(), "closed") {
			t.Fatalf("shutdown wait error = %v", err)
		}
		waitForWaiterCount(t, service, 0)
	})
}

func TestWaiterConcurrencyLimitsArePerSessionAndProcess(t *testing.T) {
	hub := newWaiterHub(2, 1)
	first, err := hub.add("session-a", emailWaitFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.add("session-a", emailWaitFilter{}); err == nil || !strings.Contains(err.Error(), "session") {
		t.Fatalf("per-session limit error = %v", err)
	}
	second, err := hub.add("session-b", emailWaitFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.add("session-c", emailWaitFilter{}); err == nil || !strings.Contains(err.Error(), "process") {
		t.Fatalf("process limit error = %v", err)
	}
	hub.cancelSession("session-a")
	select {
	case <-first.canceled:
	case <-time.After(time.Second):
		t.Fatal("session cancellation did not wake its waiter")
	}
	if hub.count() != 1 {
		t.Fatalf("waiters after session cancellation = %d", hub.count())
	}
	hub.remove(second)
	if hub.count() != 0 {
		t.Fatalf("waiters after cleanup = %d", hub.count())
	}
}

func TestWorkflowInputBoundsAndProtocolSessionLimit(t *testing.T) {
	mailbox := newTestMailbox(t)
	service, err := New(mailbox, Options{
		SessionTimeout: time.Minute, ShutdownTimeout: time.Second,
		MaxWaiters: 2, MaxWaitersPerSession: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)
	session := connectTestClient(t, httpServer.URL)
	t.Cleanup(func() { _ = session.Close() })

	for name, arguments := range map[string]map[string]any{
		"get_latest_email": {"limit": maximumLatestEmails + 1},
		"wait_for_email":   {"timeout_seconds": int(maximumWaitTimeout/time.Second) + 1},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil || result == nil || !result.IsError {
			t.Errorf("%s invalid input result = %#v, error = %v", name, result, err)
		}
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "wait_for_email", Arguments: map[string]any{"text": strings.Repeat("x", maximumWaitFilterBytes+1)},
	})
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("oversized filter result = %#v, error = %v", result, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	firstFinished := make(chan error, 1)
	go func() {
		_, callErr := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "wait_for_email", Arguments: map[string]any{"to": "never", "timeout_seconds": 5},
		})
		firstFinished <- callErr
	}()
	waitForWaiterCount(t, service, 1)
	second, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "wait_for_email", Arguments: map[string]any{"to": "also-never", "timeout_seconds": 5},
	})
	if err != nil || second == nil || !second.IsError {
		t.Fatalf("per-session protocol limit result = %#v, error = %v", second, err)
	}
	cancel()
	<-firstFinished
	waitForWaiterCount(t, service, 0)
}

func TestReadOnlyResourcesAndPrompts(t *testing.T) {
	mailbox := newTestMailbox(t)
	service, err := New(mailbox, Options{
		SessionTimeout: time.Minute, ShutdownTimeout: time.Second,
		WebBaseURL: "https://mail.example.test/base",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)
	session := connectTestClient(t, httpServer.URL)
	t.Cleanup(func() { _ = session.Close() })

	var resources []string
	for resource, err := range session.Resources(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		resources = append(resources, resource.URI)
	}
	sort.Strings(resources)
	if fmt.Sprint(resources) != "[owlmail://inbox owlmail://stats]" {
		t.Fatalf("resources = %v", resources)
	}

	var templates []string
	for template, err := range session.ResourceTemplates(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		templates = append(templates, template.URITemplate)
	}
	if fmt.Sprint(templates) != "[owlmail://email/{id}]" {
		t.Fatalf("resource templates = %v", templates)
	}

	for _, uri := range []string{"owlmail://inbox", "owlmail://stats", "owlmail://email/second"} {
		result, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			t.Fatalf("read %s: %v", uri, err)
		}
		if len(result.Contents) != 1 || result.Contents[0].MIMEType != "application/json" {
			t.Fatalf("resource %s = %#v", uri, result)
		}
		if strings.Contains(result.Contents[0].Text, "HTML needle") || strings.Contains(result.Contents[0].Text, "attachment bytes") {
			t.Fatalf("resource %s leaked excluded content: %s", uri, result.Contents[0].Text)
		}
	}
	if err := mailbox.SaveEmailToStore("large-text", false, &mailserver.Envelope{}, &mailserver.Email{
		Subject: "large", Text: strings.Repeat("€", maximumResourceTextBytes),
	}); err != nil {
		t.Fatal(err)
	}
	bounded, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "owlmail://email/large-text"})
	if err != nil {
		t.Fatal(err)
	}
	var boundedEmail resourceEmail
	if err := json.Unmarshal([]byte(bounded.Contents[0].Text), &boundedEmail); err != nil {
		t.Fatal(err)
	}
	if !boundedEmail.TextTruncated || len(boundedEmail.Email.Text) > maximumResourceTextBytes {
		t.Fatalf("bounded email resource = truncated %v, bytes %d", boundedEmail.TextTruncated, len(boundedEmail.Email.Text))
	}

	var prompts []string
	for prompt, err := range session.Prompts(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		prompts = append(prompts, prompt.Name)
	}
	sort.Strings(prompts)
	if fmt.Sprint(prompts) != "[password_reset_email registration_verification_email wait_for_delivery]" {
		t.Fatalf("prompts = %v", prompts)
	}
	prompt, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name: "registration_verification_email", Arguments: map[string]string{"recipient": "new@example.test"},
	})
	if err != nil || len(prompt.Messages) != 1 || !strings.Contains(prompt.Messages[0].Content.(*mcp.TextContent).Text, "wait_for_email") {
		t.Fatalf("verification prompt = %#v, %v", prompt, err)
	}
}

func TestPromptTimeoutUsesEffectiveServiceMaximum(t *testing.T) {
	mailbox := newTestMailbox(t)
	service, err := New(mailbox, Options{
		SessionTimeout: 2 * time.Second, ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)
	session := connectTestClient(t, httpServer.URL)
	t.Cleanup(func() { _ = session.Close() })

	prompt, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name: "registration_verification_email", Arguments: map[string]string{"recipient": "new@example.test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := prompt.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "timeout_seconds=2") || strings.Contains(text, "timeout_seconds=30") {
		t.Fatalf("prompt did not use effective timeout: %s", text)
	}
	if _, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{
		Name:      "registration_verification_email",
		Arguments: map[string]string{"recipient": "new@example.test", "timeout_seconds": "3"},
	}); err == nil {
		t.Fatal("prompt accepted timeout above the effective service maximum")
	}
}

func waitForWaiterCount(t *testing.T, service *Service, expected int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if service.waiters.count() == expected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("active waiters = %d, want %d", service.waiters.count(), expected)
}
