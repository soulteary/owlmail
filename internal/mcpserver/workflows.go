package mcpserver

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
)

const (
	maximumLatestEmails    = 20
	maximumWaitFilterBytes = 1024
)

type latestEmailInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"number of newest emails to return from 1 to 20; defaults to 1"`
}

type waitForEmailInput struct {
	To             string `json:"to,omitempty" jsonschema:"recipient name or address substring"`
	Subject        string `json:"subject,omitempty" jsonschema:"case-insensitive subject substring"`
	Text           string `json:"text,omitempty" jsonschema:"case-insensitive plain-text or sanitized HTML substring"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"maximum wait in seconds; defaults to 30 and cannot exceed 120"`
}

type emailSummary struct {
	ID            string    `json:"id"`
	Time          time.Time `json:"time"`
	Read          bool      `json:"read"`
	Subject       string    `json:"subject"`
	From          string    `json:"from"`
	To            []string  `json:"to"`
	Size          int64     `json:"size"`
	SizeHuman     string    `json:"sizeHuman"`
	HasAttachment bool      `json:"hasAttachment"`
	Preview       string    `json:"preview"`
	WebURL        string    `json:"web_url,omitempty"`
}

type latestEmailOutput struct {
	Emails []emailSummary `json:"emails"`
}

type waitForEmailOutput struct {
	Matched      bool          `json:"matched"`
	TimedOut     bool          `json:"timed_out"`
	WaitedMillis int64         `json:"waited_milliseconds"`
	Email        *emailSummary `json:"email,omitempty"`
}

type emailWaitFilter struct {
	to      *regexp.Regexp
	subject *regexp.Regexp
	text    *regexp.Regexp
}

func (filter emailWaitFilter) matches(email *types.Email) bool {
	if email == nil {
		return false
	}
	if filter.to != nil && !addressesMatchRecipient(email, filter.to) {
		return false
	}
	if filter.subject != nil && !filter.subject.MatchString(email.Subject) {
		return false
	}
	if filter.text != nil && !filter.text.MatchString(email.Text) && !filter.text.MatchString(email.HTML) {
		return false
	}
	return true
}

func addressesMatchRecipient(email *types.Email, pattern *regexp.Regexp) bool {
	for _, values := range [][]*mail.Address{email.To, email.CC, email.CalculatedBCC} {
		for _, address := range values {
			if address != nil && (pattern.MatchString(address.Address) || pattern.MatchString(address.Name)) {
				return true
			}
		}
	}
	if email.Envelope != nil {
		for _, values := range [][]string{email.Envelope.To, email.Envelope.CalculatedBCC} {
			for _, address := range values {
				if pattern.MatchString(address) {
					return true
				}
			}
		}
	}
	return false
}

func foldPattern(value string) *regexp.Regexp {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return regexp.MustCompile("(?i)" + regexp.QuoteMeta(value))
}

type emailWaiter struct {
	id        uint64
	sessionID string
	filter    emailWaitFilter
	result    chan string
	canceled  chan struct{}
}

// waiterHub is an event-driven, bounded fan-out. Waiting calls do not poll and
// do not create background goroutines; their request goroutine blocks directly
// on an event, cancellation, timeout, or service shutdown.
type waiterHub struct {
	mu            sync.Mutex
	nextID        uint64
	max           int
	maxPerSession int
	waiters       map[uint64]*emailWaiter
	perSession    map[string]int
	closing       bool
	done          chan struct{}
}

func newWaiterHub(maximum, maximumPerSession int) *waiterHub {
	return &waiterHub{
		max: maximum, maxPerSession: maximumPerSession,
		waiters: make(map[uint64]*emailWaiter), perSession: make(map[string]int),
		done: make(chan struct{}),
	}
}

func (hub *waiterHub) add(sessionID string, filter emailWaitFilter) (*emailWaiter, error) {
	if sessionID == "" {
		sessionID = "sessionless"
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.closing {
		return nil, fmt.Errorf("MCP service is shutting down")
	}
	if len(hub.waiters) >= hub.max {
		return nil, fmt.Errorf("process wait_for_email limit reached")
	}
	if hub.perSession[sessionID] >= hub.maxPerSession {
		return nil, fmt.Errorf("session wait_for_email limit reached")
	}
	hub.nextID++
	waiter := &emailWaiter{
		id: hub.nextID, sessionID: sessionID, filter: filter,
		result: make(chan string, 1), canceled: make(chan struct{}),
	}
	hub.waiters[waiter.id] = waiter
	hub.perSession[sessionID]++
	return waiter, nil
}

func (hub *waiterHub) remove(waiter *emailWaiter) {
	if waiter == nil {
		return
	}
	hub.mu.Lock()
	hub.removeLocked(waiter.id)
	hub.mu.Unlock()
}

func (hub *waiterHub) removeLocked(id uint64) *emailWaiter {
	waiter := hub.waiters[id]
	if waiter == nil {
		return nil
	}
	delete(hub.waiters, id)
	hub.perSession[waiter.sessionID]--
	if hub.perSession[waiter.sessionID] == 0 {
		delete(hub.perSession, waiter.sessionID)
	}
	return waiter
}

func (hub *waiterHub) notify(email *types.Email) {
	if email == nil {
		return
	}
	hub.mu.Lock()
	if hub.closing {
		hub.mu.Unlock()
		return
	}
	waiters := make([]*emailWaiter, 0, len(hub.waiters))
	for _, waiter := range hub.waiters {
		waiters = append(waiters, waiter)
	}
	hub.mu.Unlock()
	for _, waiter := range waiters {
		if waiter.filter.matches(email) {
			hub.mu.Lock()
			claimed := hub.waiters[waiter.id] == waiter
			if claimed {
				hub.removeLocked(waiter.id)
			}
			hub.mu.Unlock()
			if claimed {
				waiter.result <- email.ID
			}
		}
	}
}

func (hub *waiterHub) cancelSession(sessionID string) {
	if sessionID == "" {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	for id, waiter := range hub.waiters {
		if waiter.sessionID == sessionID {
			hub.removeLocked(id)
			close(waiter.canceled)
		}
	}
}

func (hub *waiterHub) close() {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.closing {
		return
	}
	hub.closing = true
	close(hub.done)
	hub.waiters = make(map[uint64]*emailWaiter)
	hub.perSession = make(map[string]int)
}

func (hub *waiterHub) count() int {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return len(hub.waiters)
}

func registerWorkflowTools(server *mcp.Server, mailbox *mailserver.MailServer, service *Service, annotations *mcp.ToolAnnotations) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_latest_email",
		Description: "Return the newest one or a bounded number of compact email summaries, including Web UI deep links when configured.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input latestEmailInput) (*mcp.CallToolResult, latestEmailOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = 1
		}
		if limit < 1 || limit > maximumLatestEmails {
			return nil, latestEmailOutput{}, fmt.Errorf("limit must be between 1 and %d", maximumLatestEmails)
		}
		previews, _ := mailbox.QueryEmailPreviews(mailserver.EmailQuery{SortBy: "store", SortOrder: "desc", Limit: limit})
		return nil, latestEmailOutput{Emails: service.makeSummaries(previews)}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "wait_for_email",
		Description: "Wait for a newly delivered email matching recipient, subject, and/or text using bounded event subscription; never polls or changes mail.",
		Annotations: annotations,
	}, func(ctx context.Context, request *mcp.CallToolRequest, input waitForEmailInput) (*mcp.CallToolResult, waitForEmailOutput, error) {
		output, err := service.waitForEmail(ctx, request, input)
		return nil, output, err
	})
}

func (service *Service) waitForEmail(ctx context.Context, request *mcp.CallToolRequest, input waitForEmailInput) (waitForEmailOutput, error) {
	timeout := defaultWaitTimeout
	maximum := service.maxWaitTimeout
	if service.sessionTimeout < maximum {
		maximum = service.sessionTimeout
	}
	if input.TimeoutSeconds != 0 {
		maximumSeconds := int(maximum / time.Second)
		if input.TimeoutSeconds < 1 || input.TimeoutSeconds > maximumSeconds {
			return waitForEmailOutput{}, fmt.Errorf("timeout_seconds must be between 1 and %d", maximumSeconds)
		}
		timeout = time.Duration(input.TimeoutSeconds) * time.Second
	} else if timeout > maximum {
		timeout = maximum
	}
	filters := []struct {
		name  string
		value string
	}{{"to", input.To}, {"subject", input.Subject}, {"text", input.Text}}
	for _, filter := range filters {
		if len(filter.value) > maximumWaitFilterBytes {
			return waitForEmailOutput{}, fmt.Errorf("%s filter cannot exceed %d bytes", filter.name, maximumWaitFilterBytes)
		}
	}
	filter := emailWaitFilter{
		to:      foldPattern(input.To),
		subject: foldPattern(input.Subject),
		text:    foldPattern(input.Text),
	}
	sessionID := ""
	if request != nil && request.Session != nil {
		sessionID = request.Session.ID()
	}
	waiter, err := service.waiters.add(sessionID, filter)
	if err != nil {
		return waitForEmailOutput{}, err
	}
	defer service.waiters.remove(waiter)

	started := time.Now()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	output := waitForEmailOutput{}
	var waitErr error
	select {
	case emailID := <-waiter.result:
		preview, exists := service.mailbox.GetEmailPreview(emailID)
		if !exists {
			return waitForEmailOutput{}, fmt.Errorf("matched email was not found")
		}
		summary := service.makeSummaries([]mailserver.EmailPreview{preview})[0]
		output.Matched = true
		output.Email = &summary
	case <-timer.C:
		output.TimedOut = true
	case <-ctx.Done():
	case <-waiter.canceled:
		waitErr = fmt.Errorf("MCP session closed while waiting for email")
	case <-service.waiters.done:
		waitErr = fmt.Errorf("MCP service closed while waiting for email")
	}
	output.WaitedMillis = time.Since(started).Milliseconds()
	if ctx.Err() != nil {
		return waitForEmailOutput{}, ctx.Err()
	}
	return output, waitErr
}

func (service *Service) makeSummaries(previews []mailserver.EmailPreview) []emailSummary {
	result := make([]emailSummary, 0, len(previews))
	for _, preview := range previews {
		result = append(result, emailSummary{
			ID: preview.ID, Time: preview.Time, Read: preview.Read, Subject: preview.Subject,
			From: preview.From, To: stringsCopy(preview.To), Size: preview.Size, SizeHuman: preview.SizeHuman,
			HasAttachment: preview.HasAttachment, Preview: preview.Preview, WebURL: service.webURL(preview.ID),
		})
	}
	return result
}

func normalizeWebBaseURL(value string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("MCP Web base URL must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" || parsed.RawPath != "" {
		return "", fmt.Errorf("MCP Web base URL must not contain credentials, query, or fragment")
	}
	normalizedPath, err := config.NormalizeBasePathname(parsed.Path)
	if err != nil || normalizedPath != parsed.Path {
		return "", fmt.Errorf("MCP Web base URL contains an invalid base pathname")
	}
	return value, nil
}

func (service *Service) webURL(emailID string) string {
	if service.webBaseURL == "" || emailID == "" {
		return ""
	}
	return service.webBaseURL + "/?email=" + url.QueryEscape(emailID)
}
