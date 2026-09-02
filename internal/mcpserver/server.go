// Package mcpserver exposes OwlMail's mailbox through a read-only MCP server.
package mcpserver

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/soulteary/owlmail/internal/mailserver"
	"github.com/soulteary/owlmail/internal/types"
	"github.com/soulteary/version-kit/v2"
)

const (
	defaultPageSize       = 50
	maximumPageSize       = 1000
	defaultSourceMaxBytes = 1 << 20
	maximumSourceMaxBytes = 100 << 20
	defaultMaxWaiters     = 64
	defaultSessionWaiters = 4
	defaultWaitTimeout    = 30 * time.Second
	maximumWaitTimeout    = 2 * time.Minute
)

// Options configures the read-only MCP endpoint.
type Options struct {
	SessionTimeout       time.Duration
	ShutdownTimeout      time.Duration
	WebBaseURL           string
	MaxWaiters           int
	MaxWaitersPerSession int
	MaxWaitTimeout       time.Duration
}

// Service owns the SDK server, Streamable HTTP handler, and active sessions.
type Service struct {
	mailbox         *mailserver.MailServer
	handler         *mcp.StreamableHTTPHandler
	sessionTimeout  time.Duration
	shutdownTimeout time.Duration
	maxWaitTimeout  time.Duration
	webBaseURL      string
	waiters         *waiterHub

	mu        sync.Mutex
	closing   bool
	sessions  map[string]*time.Timer
	requests  sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

// New creates a read-only MCP service backed by the existing mailbox store.
func New(mailbox *mailserver.MailServer, options Options) (*Service, error) {
	if mailbox == nil {
		return nil, fmt.Errorf("mail server is nil")
	}
	if options.SessionTimeout <= 0 {
		return nil, fmt.Errorf("MCP session timeout must be positive")
	}
	if options.ShutdownTimeout <= 0 {
		return nil, fmt.Errorf("MCP shutdown timeout must be positive")
	}
	if options.MaxWaiters < 0 || options.MaxWaitersPerSession < 0 || options.MaxWaitTimeout < 0 {
		return nil, fmt.Errorf("MCP waiter limits cannot be negative")
	}
	if options.MaxWaitTimeout > maximumWaitTimeout {
		return nil, fmt.Errorf("MCP maximum wait timeout cannot exceed %s", maximumWaitTimeout)
	}
	if options.MaxWaiters == 0 {
		options.MaxWaiters = defaultMaxWaiters
	}
	if options.MaxWaitersPerSession == 0 {
		options.MaxWaitersPerSession = defaultSessionWaiters
	}
	if options.MaxWaitTimeout == 0 {
		options.MaxWaitTimeout = maximumWaitTimeout
	}
	if options.MaxWaitersPerSession > options.MaxWaiters {
		return nil, fmt.Errorf("MCP per-session waiter limit cannot exceed the process limit")
	}
	webBaseURL, err := normalizeWebBaseURL(options.WebBaseURL)
	if err != nil {
		return nil, err
	}

	service := &Service{
		mailbox:         mailbox,
		sessionTimeout:  options.SessionTimeout,
		shutdownTimeout: options.ShutdownTimeout,
		maxWaitTimeout:  options.MaxWaitTimeout,
		webBaseURL:      webBaseURL,
		sessions:        make(map[string]*time.Timer),
	}
	service.waiters = newWaiterHub(options.MaxWaiters, options.MaxWaitersPerSession)
	implementationVersion := version.Default().Version
	if implementationVersion == "" {
		implementationVersion = "development"
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "owlmail",
		Version: implementationVersion,
	}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{},
		Instructions: "Read-only access to the OwlMail test mailbox. No tool can delete, mark, relay, forward, or otherwise modify mail.",
		GetSessionID: rand.Text,
	})
	registerTools(server, mailbox, service)
	registerResources(server, mailbox, service)
	registerPrompts(server)
	// One bounded dispatcher is enough: notify only snapshots the bounded waiter
	// set, evaluates filters, and publishes an ID to buffered result channels.
	if err := mailbox.OnWithConcurrency("new", 1, service.waiters.notify); err != nil {
		return nil, fmt.Errorf("register MCP new-email listener: %w", err)
	}
	service.handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse:   true,
		SessionTimeout: options.SessionTimeout,
	})
	return service, nil
}

// ServeHTTP serves the official MCP Streamable HTTP transport.
func (service *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !service.beginRequest(writer, request) {
		return
	}
	defer service.requests.Done()

	response := &statusWriter{ResponseWriter: writer}
	service.handler.ServeHTTP(response, request)
	service.finishRequest(request, response.Header().Get("Mcp-Session-Id"), response.statusCode())
}

func (service *Service) beginRequest(writer http.ResponseWriter, request *http.Request) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closing {
		http.Error(writer, "MCP service is shutting down", http.StatusServiceUnavailable)
		return false
	}
	service.requests.Add(1)
	if request.Method == http.MethodPost {
		if timer := service.sessions[request.Header.Get("Mcp-Session-Id")]; timer != nil {
			timer.Stop()
		}
	}
	return true
}

func (service *Service) finishRequest(request *http.Request, responseSessionID string, status int) {
	sessionID := request.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		sessionID = responseSessionID
	}
	if sessionID == "" {
		return
	}
	if request.Method == http.MethodDelete || status == http.StatusNotFound {
		service.forgetSession(sessionID)
		return
	}
	if request.Method == http.MethodPost && status < http.StatusBadRequest {
		service.rememberSession(sessionID)
	}
}

func (service *Service) rememberSession(sessionID string) {
	if sessionID == "" {
		return
	}
	service.mu.Lock()
	if previous := service.sessions[sessionID]; previous != nil {
		previous.Stop()
	}
	if service.closing {
		service.mu.Unlock()
		service.terminateSession(sessionID)
		return
	}
	service.sessions[sessionID] = time.AfterFunc(service.sessionTimeout, func() {
		service.forgetSession(sessionID)
	})
	service.mu.Unlock()
}

func (service *Service) forgetSession(sessionID string) {
	service.mu.Lock()
	if timer := service.sessions[sessionID]; timer != nil {
		timer.Stop()
	}
	delete(service.sessions, sessionID)
	service.mu.Unlock()
	service.waiters.cancelSession(sessionID)
}

func (service *Service) sessionIDs() []string {
	service.mu.Lock()
	defer service.mu.Unlock()
	ids := make([]string, 0, len(service.sessions))
	for sessionID, timer := range service.sessions {
		timer.Stop()
		ids = append(ids, sessionID)
	}
	return ids
}

func (service *Service) terminateSession(sessionID string) {
	request, err := http.NewRequest(http.MethodDelete, "http://owlmail.invalid/mcp", nil)
	if err != nil {
		return
	}
	request.Header.Set("Mcp-Session-Id", sessionID)
	service.handler.ServeHTTP(&discardResponseWriter{header: make(http.Header)}, request)
	service.forgetSession(sessionID)
}

// Close rejects new work, terminates active sessions, and waits for in-flight
// requests up to the configured shutdown deadline.
func (service *Service) Close() error {
	service.closeOnce.Do(func() {
		service.mu.Lock()
		service.closing = true
		service.mu.Unlock()
		service.waiters.close()

		done := make(chan struct{})
		go func() {
			for _, sessionID := range service.sessionIDs() {
				service.terminateSession(sessionID)
			}
			service.requests.Wait()
			// A request that began before closing may have completed session
			// initialization after the first snapshot.
			for _, sessionID := range service.sessionIDs() {
				service.terminateSession(sessionID)
			}
			close(done)
		}()
		timer := time.NewTimer(service.shutdownTimeout)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			service.closeErr = fmt.Errorf("MCP shutdown timed out after %s", service.shutdownTimeout)
		}
	})
	return service.closeErr
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(data []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}
	return writer.ResponseWriter.Write(data)
}

func (writer *statusWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }

func (writer *statusWriter) statusCode() int {
	if writer.status == 0 {
		return http.StatusOK
	}
	return writer.status
}

type discardResponseWriter struct {
	header http.Header
}

func (writer *discardResponseWriter) Header() http.Header {
	if writer.header == nil {
		writer.header = make(http.Header)
	}
	return writer.header
}
func (*discardResponseWriter) Write(data []byte) (int, error) { return len(data), nil }
func (*discardResponseWriter) WriteHeader(int)                {}

type emailQueryInput struct {
	From      string `json:"from,omitempty" jsonschema:"sender name or address substring"`
	To        string `json:"to,omitempty" jsonschema:"recipient name or address substring"`
	DateFrom  string `json:"date_from,omitempty" jsonschema:"inclusive start date in YYYY-MM-DD form"`
	DateTo    string `json:"date_to,omitempty" jsonschema:"inclusive end date in YYYY-MM-DD form"`
	Read      *bool  `json:"read,omitempty" jsonschema:"filter by read state when provided"`
	SortBy    string `json:"sort_by,omitempty" jsonschema:"sort field: time, subject, from, or size"`
	SortOrder string `json:"sort_order,omitempty" jsonschema:"sort direction: asc or desc"`
	Offset    int    `json:"offset,omitempty" jsonschema:"zero-based result offset"`
	Limit     int    `json:"limit,omitempty" jsonschema:"page size from 1 to 1000; defaults to 50"`
}

type searchEmailsInput struct {
	Query string `json:"query" jsonschema:"required case-insensitive text found in subject, plain text, or HTML"`
	emailQueryInput
}

type emailPage struct {
	Total  int            `json:"total"`
	Offset int            `json:"offset"`
	Limit  int            `json:"limit"`
	Emails []emailSummary `json:"emails"`
}

type emailIDInput struct {
	ID string `json:"id" jsonschema:"OwlMail email ID"`
}

type getEmailInput struct {
	ID          string `json:"id" jsonschema:"OwlMail email ID"`
	IncludeHTML bool   `json:"include_html,omitempty" jsonschema:"include the sanitized HTML body; false by default"`
}

type address struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address"`
}

type envelope struct {
	From          string   `json:"from"`
	To            []string `json:"to"`
	CC            []string `json:"cc"`
	BCC           []string `json:"bcc"`
	CalculatedBCC []string `json:"calculated_bcc"`
	Host          string   `json:"host,omitempty"`
	RemoteAddress string   `json:"remote_address,omitempty"`
}

type emailDetail struct {
	ID              string    `json:"id"`
	Time            time.Time `json:"time"`
	Read            bool      `json:"read"`
	Subject         string    `json:"subject"`
	From            []address `json:"from"`
	To              []address `json:"to"`
	CC              []address `json:"cc"`
	BCC             []address `json:"bcc"`
	CalculatedBCC   []address `json:"calculated_bcc"`
	Text            string    `json:"text,omitempty"`
	HTML            string    `json:"html,omitempty"`
	Envelope        *envelope `json:"envelope,omitempty"`
	Size            int64     `json:"size"`
	SizeHuman       string    `json:"size_human"`
	AttachmentCount int       `json:"attachment_count"`
	WebURL          string    `json:"web_url,omitempty"`
}

type getEmailOutput struct {
	Email emailDetail `json:"email"`
}

type getSourceInput struct {
	ID       string `json:"id" jsonschema:"OwlMail email ID"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"maximum source bytes to return; defaults to 1048576 and is capped at 104857600"`
}

type getSourceOutput struct {
	ID            string `json:"id"`
	Encoding      string `json:"encoding"`
	SourceBase64  string `json:"source_base64"`
	ReturnedBytes int    `json:"returned_bytes"`
	Size          int64  `json:"size"`
	Truncated     bool   `json:"truncated"`
}

type attachment struct {
	ContentType       string `json:"content_type"`
	FileName          string `json:"file_name"`
	GeneratedFileName string `json:"generated_file_name"`
	ContentID         string `json:"content_id,omitempty"`
	Size              int64  `json:"size"`
	SHA256            string `json:"sha256,omitempty"`
	Storage           string `json:"storage,omitempty"`
}

type listAttachmentsOutput struct {
	ID          string       `json:"id"`
	Attachments []attachment `json:"attachments"`
}

func registerTools(server *mcp.Server, mailbox *mailserver.MailServer, service *Service) {
	annotations := &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		IdempotentHint:  true,
		DestructiveHint: boolPointer(false),
		OpenWorldHint:   boolPointer(false),
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_emails",
		Description: "List compact email summaries without full HTML, headers, source, or attachment content.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input emailQueryInput) (*mcp.CallToolResult, emailPage, error) {
		query, err := makeEmailQuery(input, "")
		if err != nil {
			return nil, emailPage{}, err
		}
		emails, total := mailbox.QueryEmailPreviews(query)
		return nil, emailPage{Total: total, Offset: query.Offset, Limit: query.Limit, Emails: service.makeSummaries(emails)}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_emails",
		Description: "Search compact email summaries by subject and body text with optional mailbox filters.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input searchEmailsInput) (*mcp.CallToolResult, emailPage, error) {
		if strings.TrimSpace(input.Query) == "" {
			return nil, emailPage{}, fmt.Errorf("query is required")
		}
		query, err := makeEmailQuery(input.emailQueryInput, input.Query)
		if err != nil {
			return nil, emailPage{}, err
		}
		emails, total := mailbox.QueryEmailPreviews(query)
		return nil, emailPage{Total: total, Offset: query.Offset, Limit: query.Limit, Emails: service.makeSummaries(emails)}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_email",
		Description: "Get one detached email snapshot. Sanitized HTML is omitted unless include_html is explicitly true.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input getEmailInput) (*mcp.CallToolResult, getEmailOutput, error) {
		email, err := getEmail(mailbox, input.ID)
		if err != nil {
			return nil, getEmailOutput{}, err
		}
		detail := makeEmailDetail(email, input.IncludeHTML)
		detail.WebURL = service.webURL(email.ID)
		return nil, getEmailOutput{Email: detail}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_email_source",
		Description: "Read the original RFC 5322 source as lossless base64. max_bytes limits decoded source bytes; attachment payloads may exist inside the raw source.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input getSourceInput) (*mcp.CallToolResult, getSourceOutput, error) {
		if strings.TrimSpace(input.ID) == "" {
			return nil, getSourceOutput{}, fmt.Errorf("email ID is required")
		}
		maxBytes := input.MaxBytes
		if maxBytes == 0 {
			maxBytes = defaultSourceMaxBytes
		}
		if maxBytes < 1 || maxBytes > maximumSourceMaxBytes {
			return nil, getSourceOutput{}, fmt.Errorf("max_bytes must be between 1 and %d", maximumSourceMaxBytes)
		}
		content, size, truncated, err := mailbox.GetRawEmailContentLimit(input.ID, int64(maxBytes))
		if err != nil {
			return nil, getSourceOutput{}, fmt.Errorf("email source was not found")
		}
		return nil, getSourceOutput{
			ID:            input.ID,
			Encoding:      "base64",
			SourceBase64:  base64.StdEncoding.EncodeToString(content),
			ReturnedBytes: len(content),
			Size:          size,
			Truncated:     truncated,
		}, nil
	})
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_attachments",
		Description: "List attachment metadata only. This tool never returns attachment bytes.",
		Annotations: annotations,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input emailIDInput) (*mcp.CallToolResult, listAttachmentsOutput, error) {
		email, err := getEmail(mailbox, input.ID)
		if err != nil {
			return nil, listAttachmentsOutput{}, err
		}
		attachments := make([]attachment, 0, len(email.Attachments))
		for _, item := range email.Attachments {
			if item == nil {
				continue
			}
			attachments = append(attachments, attachment{
				ContentType: item.ContentType, FileName: item.FileName,
				GeneratedFileName: item.GeneratedFileName, ContentID: item.ContentID,
				Size: item.Size, SHA256: item.ContentSHA256, Storage: item.Storage,
			})
		}
		sort.Slice(attachments, func(i, j int) bool {
			return attachments[i].GeneratedFileName < attachments[j].GeneratedFileName
		})
		return nil, listAttachmentsOutput{ID: input.ID, Attachments: attachments}, nil
	})
	registerWorkflowTools(server, mailbox, service, annotations)
}

func makeEmailQuery(input emailQueryInput, text string) (mailserver.EmailQuery, error) {
	limit := input.Limit
	if limit == 0 {
		limit = defaultPageSize
	}
	if limit < 1 || limit > maximumPageSize {
		return mailserver.EmailQuery{}, fmt.Errorf("limit must be between 1 and %d", maximumPageSize)
	}
	if input.Offset < 0 {
		return mailserver.EmailQuery{}, fmt.Errorf("offset cannot be negative")
	}
	sortBy := strings.ToLower(strings.TrimSpace(input.SortBy))
	if sortBy == "" {
		sortBy = "time"
	}
	if sortBy != "time" && sortBy != "subject" && sortBy != "from" && sortBy != "size" {
		return mailserver.EmailQuery{}, fmt.Errorf("sort_by must be time, subject, from, or size")
	}
	sortOrder := strings.ToLower(strings.TrimSpace(input.SortOrder))
	if sortOrder == "" {
		sortOrder = "desc"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		return mailserver.EmailQuery{}, fmt.Errorf("sort_order must be asc or desc")
	}
	query := mailserver.EmailQuery{
		Text: text, From: input.From, To: input.To, Read: input.Read,
		SortBy: sortBy, SortOrder: sortOrder, Offset: input.Offset, Limit: limit,
	}
	if input.DateFrom != "" {
		date, err := time.Parse("2006-01-02", input.DateFrom)
		if err != nil {
			return mailserver.EmailQuery{}, fmt.Errorf("date_from must use YYYY-MM-DD")
		}
		query.DateFrom = &date
	}
	if input.DateTo != "" {
		date, err := time.Parse("2006-01-02", input.DateTo)
		if err != nil {
			return mailserver.EmailQuery{}, fmt.Errorf("date_to must use YYYY-MM-DD")
		}
		// The shared mailbox query uses an inclusive DateTo comparison. Use
		// the last representable instant of the requested UTC date so that
		// midnight of the following day is not included.
		date = date.AddDate(0, 0, 1).Add(-time.Nanosecond)
		query.DateTo = &date
	}
	if query.DateFrom != nil && query.DateTo != nil && !query.DateFrom.Before(*query.DateTo) {
		return mailserver.EmailQuery{}, fmt.Errorf("date_from must not be after date_to")
	}
	return query, nil
}

func getEmail(mailbox *mailserver.MailServer, id string) (*types.Email, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("email ID is required")
	}
	email, err := mailbox.GetEmail(id)
	if err != nil {
		return nil, fmt.Errorf("email was not found")
	}
	return email, nil
}

func makeEmailDetail(email *types.Email, includeHTML bool) emailDetail {
	detail := emailDetail{
		ID: email.ID, Time: email.Time, Read: email.Read, Subject: email.Subject,
		From: addresses(email.From), To: addresses(email.To), CC: addresses(email.CC),
		BCC: addresses(email.BCC), CalculatedBCC: addresses(email.CalculatedBCC),
		Text: email.Text, Size: email.Size, SizeHuman: email.SizeHuman,
		AttachmentCount: len(email.Attachments),
	}
	if includeHTML {
		detail.HTML = email.HTML
	}
	if email.Envelope != nil {
		detail.Envelope = &envelope{
			From: email.Envelope.From, To: stringsCopy(email.Envelope.To),
			CC: stringsCopy(email.Envelope.CC), BCC: stringsCopy(email.Envelope.BCC),
			CalculatedBCC: stringsCopy(email.Envelope.CalculatedBCC), Host: email.Envelope.Host,
			RemoteAddress: email.Envelope.RemoteAddress,
		}
	}
	return detail
}

func addresses(values []*mail.Address) []address {
	result := make([]address, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, address{Name: value.Name, Address: value.Address})
		}
	}
	return result
}

func stringsCopy(values []string) []string {
	result := make([]string, len(values))
	copy(result, values)
	return result
}

func boolPointer(value bool) *bool { return &value }

var _ http.Handler = (*Service)(nil)
var _ interface{ Close() error } = (*Service)(nil)
