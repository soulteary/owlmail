// Package emailcheck implements bounded, deterministic inspection without network access.
package emailcheck

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/soulteary/owlmail/internal/types"
	"golang.org/x/net/html"
)

const maxBodyBytes = 2 << 20
const maxResults = 200

var textURL = regexp.MustCompile(`(?i:https?)://[^\s<>"']+`)

// Extraction identifies a bounded RE2 expression and its source.
type Extraction struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Source  string `json:"source,omitempty"`
}

// AttachmentExpectation verifies one attachment by its original filename.
type AttachmentExpectation struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	MaxBytes    *int64 `json:"max_bytes,omitempty"`
}

// Request contains explicit assertions; empty requests only inspect links.
type Request struct {
	SubjectContains  []string                `json:"subject_contains,omitempty"`
	TextContains     []string                `json:"text_contains,omitempty"`
	AttachmentCount  *int                    `json:"attachment_count,omitempty"`
	Attachments      []AttachmentExpectation `json:"attachments,omitempty"`
	AllowedLinkHosts []string                `json:"allowed_link_hosts,omitempty"`
	Extract          []Extraction            `json:"extract,omitempty"`
}

// Link is a parsed reference and where it was found; no URL is fetched.
type Link struct {
	URL    string `json:"url"`
	Source string `json:"source"`
	Scheme string `json:"scheme"`
	Host   string `json:"host,omitempty"`
}

// Assertion records a stable name and an explanation for CI diagnostics.
type Assertion struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// Report is a deterministic result over one detached, sanitized email snapshot.
type Report struct {
	Passed               bool                `json:"passed"`
	Checks               []Assertion         `json:"checks"`
	Links                []Link              `json:"links"`
	Extractions          map[string][]string `json:"extractions"`
	LinksTruncated       bool                `json:"links_truncated"`
	ExtractionsTruncated bool                `json:"extractions_truncated"`
}

// Inspect verifies explicit assertions without modifying mail or contacting remote services.
func Inspect(email *types.Email, request Request) (Report, error) {
	r := Report{Passed: true, Checks: make([]Assertion, 0), Links: make([]Link, 0), Extractions: make(map[string][]string)}
	if email == nil {
		return r, fmt.Errorf("email is nil")
	}
	if len(email.Text) > maxBodyBytes || len(email.HTML) > maxBodyBytes || len(email.Subject) > maxBodyBytes {
		return r, fmt.Errorf("inspection bodies must not exceed 2 MiB")
	}
	if len(request.SubjectContains)+len(request.TextContains)+len(request.Attachments)+len(request.Extract)+len(request.AllowedLinkHosts) > 100 {
		return r, fmt.Errorf("inspection accepts at most 100 selectors")
	}
	add := func(name string, passed bool, message string) {
		r.Checks = append(r.Checks, Assertion{Name: name, Passed: passed, Message: message})
		r.Passed = r.Passed && passed
	}
	for i, s := range request.SubjectContains {
		if len(s) > 1024 {
			return r, fmt.Errorf("assertion cannot exceed 1024 bytes")
		}
		add(fmt.Sprintf("subject_contains[%d]", i), strings.Contains(email.Subject, s), "Case-sensitive subject substring")
	}
	for i, s := range request.TextContains {
		if len(s) > 1024 {
			return r, fmt.Errorf("assertion cannot exceed 1024 bytes")
		}
		add(fmt.Sprintf("text_contains[%d]", i), strings.Contains(email.Text, s), "Case-sensitive plain-text substring")
	}
	if request.AttachmentCount != nil {
		if *request.AttachmentCount < 0 {
			return r, fmt.Errorf("attachment_count cannot be negative")
		}
		add("attachment_count", len(email.Attachments) == *request.AttachmentCount, fmt.Sprintf("Found %d attachments", len(email.Attachments)))
	}
	for i, want := range request.Attachments {
		if want.Filename == "" || len(want.Filename) > 1024 || want.MaxBytes != nil && *want.MaxBytes < 0 {
			return r, fmt.Errorf("invalid attachment expectation")
		}
		var matches []*types.Attachment
		for _, a := range email.Attachments {
			if a != nil && a.FileName == want.Filename {
				matches = append(matches, a)
			}
		}
		passed := len(matches) == 1
		if passed {
			a := matches[0]
			passed = (want.ContentType == "" || a.ContentType == want.ContentType) && (want.SHA256 == "" || strings.EqualFold(a.ContentSHA256, want.SHA256)) && (want.MaxBytes == nil || a.Size <= *want.MaxBytes)
		}
		add(fmt.Sprintf("attachment[%d]", i), passed, "Filename must identify exactly one attachment with the requested metadata")
	}
	seen := make(map[string]bool)
	appendLink := func(raw, source string) {
		u, err := url.Parse(raw)
		if raw == "" {
			return
		}
		var scheme, host string
		if err != nil {
			prefix, _, _ := strings.Cut(raw, ":")
			scheme = strings.ToLower(prefix)
			if scheme != "http" && scheme != "https" {
				return
			}
			// Keep malformed HTTP candidates visible and fail host assertions closed.
		} else {
			scheme, host = strings.ToLower(u.Scheme), strings.ToLower(u.Hostname())
		}
		key := source + "\x00" + raw
		if seen[key] {
			return
		}
		if len(r.Links) >= maxResults {
			r.LinksTruncated = true
			return
		}
		seen[key] = true
		r.Links = append(r.Links, Link{URL: raw, Source: source, Scheme: scheme, Host: host})
	}
	for remaining := email.Text; remaining != ""; {
		index := textURL.FindStringIndex(remaining)
		if index == nil {
			break
		}
		appendLink(strings.TrimRight(remaining[index[0]:index[1]], ".,;!?)"), "text")
		remaining = remaining[index[1]:]
	}
	tokenizer := html.NewTokenizer(strings.NewReader(email.HTML))
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			if err := tokenizer.Err(); err != nil && err != io.EOF {
				return r, err
			}
			break
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		for _, a := range token.Attr {
			if token.Data == "a" && a.Key == "href" {
				appendLink(a.Val, "html_link")
			}
			if token.Data == "img" && a.Key == "src" {
				appendLink(a.Val, "html_image")
			}
		}
	}
	if len(request.AllowedLinkHosts) > 0 {
		allowed := make(map[string]bool)
		for _, host := range request.AllowedLinkHosts {
			if len(host) > 253 || host == "" || strings.ContainsAny(host, "/:@ ") {
				return r, fmt.Errorf("allowed_link_hosts must contain bare hostnames")
			}
			allowed[strings.ToLower(host)] = true
		}
		passed := !r.LinksTruncated
		for _, link := range r.Links {
			if link.Scheme == "http" || link.Scheme == "https" {
				passed = passed && link.Host != "" && allowed[link.Host]
			}
		}
		add("allowed_link_hosts", passed, "Every discovered HTTP(S) link must use an exact allowed hostname; truncated inspection fails closed")
	}
	for _, ex := range request.Extract {
		if ex.Name == "" || len(ex.Name) > 64 || len(ex.Pattern) == 0 || len(ex.Pattern) > 1024 {
			return r, fmt.Errorf("extraction requires a name (1..64 bytes) and pattern (1..1024 bytes)")
		}
		if _, exists := r.Extractions[ex.Name]; exists {
			return r, fmt.Errorf("extraction names must be unique")
		}
		pattern, err := regexp.Compile(ex.Pattern)
		if err != nil {
			return r, fmt.Errorf("invalid extraction pattern %q", ex.Name)
		}
		source := email.Text
		switch ex.Source {
		case "", "text":
		case "subject":
			source = email.Subject
		case "html":
			source = email.HTML
		default:
			return r, fmt.Errorf("extraction source must be text, subject, or html")
		}
		matches := pattern.FindAllStringSubmatch(source, maxResults+1)
		if len(matches) > maxResults {
			matches = matches[:maxResults]
			r.ExtractionsTruncated = true
		}
		values := make([]string, 0, len(matches))
		for _, match := range matches {
			value := match[0]
			if len(match) > 1 {
				value = match[1]
			}
			values = append(values, value)
		}
		r.Extractions[ex.Name] = values
	}
	return r, nil
}
