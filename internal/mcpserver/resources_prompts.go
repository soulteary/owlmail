package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/soulteary/owlmail/internal/mailserver"
)

const maximumResourceTextBytes = 32 << 10

type resourceEmail struct {
	Email         emailDetail `json:"email"`
	TextTruncated bool        `json:"text_truncated"`
}

type inboxStats struct {
	Total  int `json:"total"`
	Unread int `json:"unread"`
	Read   int `json:"read"`
}

func registerResources(server *mcp.Server, mailbox *mailserver.MailServer, service *Service) {
	server.AddResource(&mcp.Resource{
		URI: "owlmail://inbox", Name: "inbox", Title: "OwlMail inbox",
		Description: "The 50 newest compact email summaries. Full HTML, source, headers, and attachment bytes are omitted.",
		MIMEType:    "application/json",
	}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		previews, total := mailbox.QueryEmailPreviews(mailserver.EmailQuery{SortBy: "time", SortOrder: "desc", Limit: defaultPageSize})
		return jsonResource(request.Params.URI, emailPageResource{
			Total: total, Emails: service.makeSummaries(previews),
		})
	})

	server.AddResource(&mcp.Resource{
		URI: "owlmail://stats", Name: "stats", Title: "OwlMail inbox statistics",
		Description: "Bounded read-only total, read, and unread mailbox counts.",
		MIMEType:    "application/json",
	}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		stats := mailbox.GetEmailStats()
		total, _ := stats["total"].(int)
		unread, _ := stats["unread"].(int)
		read, _ := stats["read"].(int)
		return jsonResource(request.Params.URI, inboxStats{
			Total: total, Unread: unread, Read: read,
		})
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "owlmail://email/{id}", Name: "email", Title: "OwlMail email",
		Description: "A detached email snapshot with bounded plain text and without HTML, source, headers, or attachment bytes.",
		MIMEType:    "application/json",
	}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		id, err := emailIDFromResourceURI(request.Params.URI)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(request.Params.URI)
		}
		email, err := getEmail(mailbox, id)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(request.Params.URI)
		}
		detail := makeEmailDetail(email, false)
		detail.WebURL = service.webURL(id)
		detail.Text, err = boundedUTF8(detail.Text, maximumResourceTextBytes)
		return jsonResource(request.Params.URI, resourceEmail{
			Email: detail, TextTruncated: err != nil,
		})
	})
}

type emailPageResource struct {
	Total  int            `json:"total"`
	Emails []emailSummary `json:"emails"`
}

func jsonResource(uri string, value any) (*mcp.ReadResourceResult, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode resource: %w", err)
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
		URI: uri, MIMEType: "application/json", Text: string(encoded),
	}}}, nil
}

func emailIDFromResourceURI(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "owlmail" || parsed.Host != "email" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid email resource URI")
	}
	id := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if id == "" || strings.Contains(id, "/") {
		return "", fmt.Errorf("invalid email resource ID")
	}
	return url.PathUnescape(id)
}

// boundedUTF8 returns a valid UTF-8 prefix and uses a non-nil sentinel error to
// report truncation without exposing the discarded content.
func boundedUTF8(value string, maximum int) (string, error) {
	if len(value) <= maximum {
		return value, nil
	}
	end := maximum
	for end > 0 && !utf8Start(value[end]) {
		end--
	}
	return value[:end], fmt.Errorf("truncated")
}

func utf8Start(value byte) bool { return value&0xc0 != 0x80 }

func registerPrompts(server *mcp.Server, service *Service) {
	maximum := service.maxWaitTimeout
	if service.sessionTimeout < maximum {
		maximum = service.sessionTimeout
	}
	timeoutDescription := fmt.Sprintf("Optional wait timeout from 1 to %d seconds", int(maximum/time.Second))
	if maximum < time.Second {
		timeoutDescription = "Omit to use the configured sub-second service timeout"
	}
	server.AddPrompt(&mcp.Prompt{
		Name: "registration_verification_email", Title: "Inspect a registration verification email",
		Description: "Wait for and inspect a registration or account-verification email without changing the inbox.",
		Arguments: []*mcp.PromptArgument{
			{Name: "recipient", Description: "Expected recipient email address", Required: true},
			{Name: "subject", Description: "Optional expected subject substring"},
			{Name: "timeout_seconds", Description: timeoutDescription},
		},
	}, verificationPrompt("registration or account-verification", maximum))

	server.AddPrompt(&mcp.Prompt{
		Name: "password_reset_email", Title: "Inspect a password reset email",
		Description: "Wait for and inspect a password-reset email without changing the inbox.",
		Arguments: []*mcp.PromptArgument{
			{Name: "recipient", Description: "Expected recipient email address", Required: true},
			{Name: "subject", Description: "Optional expected subject substring"},
			{Name: "timeout_seconds", Description: timeoutDescription},
		},
	}, verificationPrompt("password-reset", maximum))

	server.AddPrompt(&mcp.Prompt{
		Name: "wait_for_delivery", Title: "Wait for email delivery",
		Description: "Wait for a new matching test email, then report its compact metadata and Web UI deep link.",
		Arguments: []*mcp.PromptArgument{
			{Name: "recipient", Description: "Optional recipient substring"},
			{Name: "subject", Description: "Optional subject substring"},
			{Name: "text", Description: "Optional body-text substring"},
			{Name: "timeout_seconds", Description: timeoutDescription},
		},
	}, func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := request.Params.Arguments
		timeout, err := promptTimeoutInstruction(args["timeout_seconds"], maximum)
		if err != nil {
			return nil, err
		}
		return promptResult("Wait for the next matching test email by calling wait_for_email with to=" +
			strconv.Quote(args["recipient"]) + ", subject=" + strconv.Quote(args["subject"]) + ", text=" +
			strconv.Quote(args["text"]) + timeout +
			". Report whether it matched and include the returned Web UI link. Do not modify, relay, or download attachment bytes."), nil
	})
}

func verificationPrompt(kind string, maximum time.Duration) mcp.PromptHandler {
	return func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		args := request.Params.Arguments
		recipient := strings.TrimSpace(args["recipient"])
		if recipient == "" {
			return nil, fmt.Errorf("recipient is required")
		}
		timeout, err := promptTimeoutInstruction(args["timeout_seconds"], maximum)
		if err != nil {
			return nil, err
		}
		return promptResult("Wait for a new " + kind + " email by calling wait_for_email with to=" +
			strconv.Quote(recipient) + ", subject=" + strconv.Quote(args["subject"]) + timeout +
			". If it matches, call get_email with the returned ID and include_html=false; request sanitized HTML only if the plain text lacks the verification URL or code. Extract that value and include the Web UI link. Do not mark, delete, relay, or download attachment bytes."), nil
	}
}

func promptTimeoutInstruction(value string, maximum time.Duration) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		selected := defaultWaitTimeout
		if selected > maximum {
			selected = maximum
		}
		seconds := int(selected / time.Second)
		if seconds < 1 {
			return ", omitting timeout_seconds so the service applies its configured maximum", nil
		}
		return ", and timeout_seconds=" + strconv.Itoa(seconds), nil
	}
	maximumSeconds := int(maximum / time.Second)
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 1 || seconds > maximumSeconds {
		if maximumSeconds < 1 {
			return "", fmt.Errorf("timeout_seconds must be omitted because the configured maximum is below one second")
		}
		return "", fmt.Errorf("timeout_seconds must be an integer between 1 and %d", maximumSeconds)
	}
	return ", and timeout_seconds=" + strconv.Itoa(seconds), nil
}

func promptResult(text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{Messages: []*mcp.PromptMessage{{
		Role: "user", Content: &mcp.TextContent{Text: text},
	}}}
}
