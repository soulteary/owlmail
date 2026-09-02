package mcpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/soulteary/owlmail/internal/mailserver"
)

func TestReadOnlyToolsUseMailboxSnapshots(t *testing.T) {
	mailbox := newTestMailbox(t)
	service := newTestService(t, mailbox, time.Minute)
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)

	session := connectTestClient(t, httpServer.URL)
	t.Cleanup(func() { _ = session.Close() })

	var tools []*mcp.Tool
	for tool, err := range session.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		tools = append(tools, tool)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	wantNames := []string{"get_email", "get_email_source", "get_latest_email", "list_attachments", "list_emails", "search_emails", "wait_for_email"}
	if len(tools) != len(wantNames) {
		t.Fatalf("tools = %d, want %d", len(tools), len(wantNames))
	}
	for index, tool := range tools {
		if tool.Name != wantNames[index] {
			t.Fatalf("tool[%d] = %q, want %q", index, tool.Name, wantNames[index])
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint || !tool.Annotations.IdempotentHint ||
			tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint ||
			tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Fatalf("tool %q does not declare a closed-world read-only boundary: %#v", tool.Name, tool.Annotations)
		}
	}

	var page emailPage
	callTool(t, session, "list_emails", map[string]any{"limit": 1}, &page)
	if page.Total != 2 || len(page.Emails) != 1 {
		t.Fatalf("unexpected compact list result: %#v", page)
	}

	callTool(t, session, "search_emails", map[string]any{"query": "needle"}, &page)
	if page.Total != 1 || len(page.Emails) != 1 || page.Emails[0].ID != "second" {
		t.Fatalf("unexpected search result: %#v", page)
	}

	var detail map[string]any
	callTool(t, session, "get_email", map[string]any{"id": "second"}, &detail)
	email := detail["email"].(map[string]any)
	if _, ok := email["html"]; ok {
		t.Fatalf("get_email returned HTML without include_html: %#v", email)
	}
	if email["text"] != "plain needle body" || int(email["attachment_count"].(float64)) != 1 {
		t.Fatalf("unexpected email detail: %#v", email)
	}
	callTool(t, session, "get_email", map[string]any{"id": "second", "include_html": true}, &detail)
	if !strings.Contains(detail["email"].(map[string]any)["html"].(string), "HTML needle") {
		t.Fatalf("include_html did not return sanitized HTML: %#v", detail)
	}

	var attachments listAttachmentsOutput
	callTool(t, session, "list_attachments", map[string]any{"id": "second"}, &attachments)
	if len(attachments.Attachments) != 1 || attachments.Attachments[0].FileName != "report.txt" {
		t.Fatalf("unexpected attachment metadata: %#v", attachments)
	}
	encodedAttachments, _ := json.Marshal(attachments)
	if strings.Contains(string(encodedAttachments), "attachment bytes") {
		t.Fatalf("attachment payload leaked into metadata: %s", encodedAttachments)
	}

	var source getSourceOutput
	callTool(t, session, "get_email_source", map[string]any{"id": "second", "max_bytes": 16}, &source)
	decodedSource, err := base64.StdEncoding.DecodeString(source.SourceBase64)
	if err != nil {
		t.Fatal(err)
	}
	if source.Encoding != "base64" || !source.Truncated || source.ReturnedBytes != 16 ||
		len(decodedSource) != source.ReturnedBytes || source.Size <= int64(source.ReturnedBytes) {
		t.Fatalf("source limit was not enforced: %#v", source)
	}

	stored, err := mailbox.GetEmail("second")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Read || stored.Subject != "Needle result" || len(mailbox.GetAllEmail()) != 2 {
		t.Fatalf("MCP calls mutated the mailbox: %#v", stored)
	}
}

func TestEmailSourceUsesLosslessBase64(t *testing.T) {
	mailbox := newTestMailbox(t)
	prefix := []byte("From: raw@example.test\r\nTo: inbox@example.test\r\nSubject: Raw bytes\r\n\r\n")
	rawSource := append(append(append([]byte{}, prefix...), []byte("valid UTF-8: \xe2\x82\xac; 8-bit: ")...), 0xff, 0xfe)
	if err := os.WriteFile(filepath.Join(mailbox.GetMailDir(), "raw.eml"), rawSource, 0600); err != nil {
		t.Fatal(err)
	}
	if err := mailbox.SaveEmailToStore("raw", false,
		&mailserver.Envelope{From: "raw@example.test", To: []string{"inbox@example.test"}},
		&mailserver.Email{Subject: "Raw bytes"}); err != nil {
		t.Fatal(err)
	}

	service := newTestService(t, mailbox, time.Minute)
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)
	session := connectTestClient(t, httpServer.URL)
	t.Cleanup(func() { _ = session.Close() })

	tests := []struct {
		name     string
		maxBytes int
	}{
		{name: "truncated in UTF-8 sequence", maxBytes: len(prefix) + len("valid UTF-8: ") + 2},
		{name: "complete source with invalid UTF-8", maxBytes: len(rawSource)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output getSourceOutput
			callTool(t, session, "get_email_source", map[string]any{"id": "raw", "max_bytes": test.maxBytes}, &output)
			decoded, err := base64.StdEncoding.DecodeString(output.SourceBase64)
			if err != nil {
				t.Fatal(err)
			}
			if output.Encoding != "base64" || output.ReturnedBytes != test.maxBytes ||
				!bytes.Equal(decoded, rawSource[:test.maxBytes]) {
				t.Fatalf("source bytes changed: output=%#v decoded=%x want=%x", output, decoded, rawSource[:test.maxBytes])
			}
			if output.Size != int64(len(rawSource)) || output.Truncated != (test.maxBytes < len(rawSource)) {
				t.Fatalf("unexpected source metadata: %#v", output)
			}
		})
	}
}

func TestEmailQueryNormalizesSortAndInclusiveDateTo(t *testing.T) {
	query, err := makeEmailQuery(emailQueryInput{
		SortOrder: "asc",
		DateTo:    "2026-09-02",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if query.SortBy != "time" || query.SortOrder != "asc" {
		t.Fatalf("sort = %q %q, want time asc", query.SortBy, query.SortOrder)
	}
	wantDateTo := time.Date(2026, time.September, 2, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if query.DateTo == nil || !query.DateTo.Equal(wantDateTo) {
		t.Fatalf("date_to = %v, want %v", query.DateTo, wantDateTo)
	}

	sameDay, err := makeEmailQuery(emailQueryInput{
		DateFrom: "2026-09-02",
		DateTo:   "2026-09-02",
	}, "")
	if err != nil {
		t.Fatalf("same-day range was rejected: %v", err)
	}
	if sameDay.DateFrom == nil || sameDay.DateTo == nil || !sameDay.DateFrom.Before(*sameDay.DateTo) {
		t.Fatalf("unexpected same-day range: %#v", sameDay)
	}
}

func TestConcurrentSessionsUnknownIDsAndClientClose(t *testing.T) {
	mailbox := newTestMailbox(t)
	service := newTestService(t, mailbox, time.Minute)
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)

	const sessionCount = 8
	sessions := make([]*mcp.ClientSession, sessionCount)
	var wait sync.WaitGroup
	errorsBySession := make(chan error, sessionCount)
	for index := range sessions {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			client := mcp.NewClient(&mcp.Implementation{Name: fmt.Sprintf("test-%d", index), Version: "1"}, nil)
			session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
				Endpoint: httpServer.URL, DisableStandaloneSSE: true,
			}, nil)
			if err == nil {
				sessions[index] = session
			}
			errorsBySession <- err
		}(index)
	}
	wait.Wait()
	close(errorsBySession)
	for err := range errorsBySession {
		if err != nil {
			t.Fatal(err)
		}
	}
	waitForSessionCount(t, service, sessionCount)

	request, _ := http.NewRequest(http.MethodDelete, httpServer.URL, nil)
	request.Header.Set("Mcp-Session-Id", "unknown-session")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown session status = %d, want 404", response.StatusCode)
	}

	for _, session := range sessions {
		wait.Add(1)
		go func(session *mcp.ClientSession) {
			defer wait.Done()
			if err := session.Close(); err != nil {
				t.Errorf("close session: %v", err)
			}
		}(session)
	}
	wait.Wait()
	waitForSessionCount(t, service, 0)
}

func TestIdleTimeoutAndServiceCloseCleanup(t *testing.T) {
	mailbox := newTestMailbox(t)
	service := newTestService(t, mailbox, 40*time.Millisecond)
	httpServer := httptest.NewServer(service)
	t.Cleanup(httpServer.Close)

	first := connectTestClient(t, httpServer.URL)
	second := connectTestClient(t, httpServer.URL)
	waitForSessionCount(t, service, 2)
	time.Sleep(100 * time.Millisecond)
	waitForSessionCount(t, service, 0)
	if result, err := first.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_emails"}); err == nil && !result.IsError {
		t.Fatal("timed-out MCP session unexpectedly remained usable")
	}

	third := connectTestClient(t, httpServer.URL)
	waitForSessionCount(t, service, 1)
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	waitForSessionCount(t, service, 0)

	request, _ := http.NewRequest(http.MethodPost, httpServer.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","clientInfo":{"name":"late","version":"1"},"capabilities":{}}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("post-close status = %d, want 503", response.StatusCode)
	}
	_ = first.Close()
	_ = second.Close()
	_ = third.Close()
}

func TestServiceCloseHonorsShutdownTimeout(t *testing.T) {
	mailbox := newTestMailbox(t)
	service, err := New(mailbox, Options{SessionTimeout: time.Minute, ShutdownTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	body := &blockingBody{started: make(chan struct{}, 1), release: make(chan struct{})}
	request, _ := http.NewRequest(http.MethodPost, "http://owlmail.invalid/mcp", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	finished := make(chan struct{})
	go func() {
		service.ServeHTTP(httptest.NewRecorder(), request)
		close(finished)
	}()
	<-body.started

	started := time.Now()
	err = service.Close()
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Close() error = %v, want timeout", err)
	}
	if elapsed := time.Since(started); elapsed < 15*time.Millisecond || elapsed > 250*time.Millisecond {
		t.Fatalf("Close() elapsed = %s, want configured bound", elapsed)
	}
	close(body.release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("blocked MCP request did not finish after release")
	}
}

type blockingBody struct {
	started chan struct{}
	release chan struct{}
}

func (body *blockingBody) Read([]byte) (int, error) {
	select {
	case body.started <- struct{}{}:
	default:
	}
	<-body.release
	return 0, io.EOF
}

func (*blockingBody) Close() error { return nil }

func newTestMailbox(t *testing.T) *mailserver.MailServer {
	t.Helper()
	directory := t.TempDir()
	mailbox, err := mailserver.NewMailServer(1025, "localhost", directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mailbox.Close() })

	messages := []struct {
		id       string
		source   string
		email    *mailserver.Email
		envelope *mailserver.Envelope
	}{
		{
			id: "first", source: "From: Alpha <alpha@example.test>\r\nTo: inbox@example.test\r\nSubject: First\r\n\r\nfirst body",
			email:    &mailserver.Email{Subject: "First", Text: "first body", From: []*mail.Address{{Name: "Alpha", Address: "alpha@example.test"}}, To: []*mail.Address{{Address: "inbox@example.test"}}},
			envelope: &mailserver.Envelope{From: "alpha@example.test", To: []string{"inbox@example.test"}},
		},
		{
			id: "second", source: "From: Beta <beta@example.test>\r\nTo: inbox@example.test\r\nSubject: Needle result\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain\r\n\r\nplain needle body\r\n--x\r\nContent-Disposition: attachment; filename=report.txt\r\n\r\nattachment bytes\r\n--x--",
			email: &mailserver.Email{
				Subject: "Needle result", Text: "plain needle body", HTML: "<p>HTML needle body</p>",
				From: []*mail.Address{{Name: "Beta", Address: "beta@example.test"}}, To: []*mail.Address{{Address: "inbox@example.test"}},
				Attachments: []*mailserver.Attachment{{ContentType: "text/plain", FileName: "report.txt", GeneratedFileName: "report.txt", Size: 16}},
			},
			envelope: &mailserver.Envelope{From: "beta@example.test", To: []string{"inbox@example.test"}},
		},
	}
	for _, message := range messages {
		if err := os.WriteFile(filepath.Join(directory, message.id+".eml"), []byte(message.source), 0600); err != nil {
			t.Fatal(err)
		}
		if err := mailbox.SaveEmailToStore(message.id, false, message.envelope, message.email); err != nil {
			t.Fatal(err)
		}
	}
	return mailbox
}

func newTestService(t *testing.T, mailbox *mailserver.MailServer, sessionTimeout time.Duration) *Service {
	t.Helper()
	service, err := New(mailbox, Options{SessionTimeout: sessionTimeout, ShutdownTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	return service
}

func connectTestClient(t *testing.T, endpoint string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "owlmail-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: endpoint, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any, output any) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool %s returned an error: %#v", name, result.Content)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, output); err != nil {
		t.Fatal(err)
	}
}

func waitForSessionCount(t *testing.T, service *Service, expected int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		service.mu.Lock()
		count := len(service.sessions)
		service.mu.Unlock()
		if count == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	service.mu.Lock()
	count := len(service.sessions)
	service.mu.Unlock()
	t.Fatalf("active sessions = %d, want %d", count, expected)
}
