package webhook

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

// Attachment is the safe attachment metadata exposed to webhook templates.
type Attachment struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

// EmailPayload is the stable email representation used by default payloads and
// custom body templates. It intentionally omits local file-system paths.
type EmailPayload struct {
	ID              string                 `json:"id"`
	Time            time.Time              `json:"time"`
	Subject         string                 `json:"subject"`
	From            []string               `json:"from"`
	To              []string               `json:"to"`
	CC              []string               `json:"cc,omitempty"`
	BCC             []string               `json:"bcc,omitempty"`
	EnvelopeFrom    string                 `json:"envelopeFrom,omitempty"`
	EnvelopeTo      []string               `json:"envelopeTo,omitempty"`
	Text            string                 `json:"text"`
	HTML            string                 `json:"html,omitempty"`
	Size            int64                  `json:"size"`
	SizeHuman       string                 `json:"sizeHuman"`
	Attachments     []Attachment           `json:"attachments,omitempty"`
	Headers         map[string]interface{} `json:"-"`
	AttachmentCount int                    `json:"attachmentCount"`
}

type eventPayload struct {
	Event   string       `json:"event"`
	Message string       `json:"message"`
	Email   EmailPayload `json:"email"`
}

func newEmailPayload(email *types.Email) EmailPayload {
	payload := EmailPayload{
		ID:              email.ID,
		Time:            email.Time,
		Subject:         email.Subject,
		From:            addressStrings(email.From),
		To:              addressStrings(email.To),
		CC:              addressStrings(email.CC),
		BCC:             addressStrings(email.BCC),
		Text:            email.Text,
		HTML:            email.HTML,
		Size:            email.Size,
		SizeHuman:       email.SizeHuman,
		Headers:         email.Headers,
		AttachmentCount: len(email.Attachments),
	}
	if email.Envelope != nil {
		payload.EnvelopeFrom = email.Envelope.From
		payload.EnvelopeTo = append([]string(nil), email.Envelope.To...)
	}
	if len(email.Attachments) > 0 {
		payload.Attachments = make([]Attachment, 0, len(email.Attachments))
		for _, attachment := range email.Attachments {
			if attachment == nil {
				continue
			}
			payload.Attachments = append(payload.Attachments, Attachment{
				FileName:    attachment.FileName,
				ContentType: attachment.ContentType,
				Size:        attachment.Size,
			})
		}
	}
	return payload
}

func addressStrings(addresses []*mail.Address) []string {
	if len(addresses) == 0 {
		return nil
	}
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address != nil && address.Address != "" {
			result = append(result, address.Address)
		}
	}
	return result
}

func defaultMessage(payload EmailPayload) string {
	subject := strings.TrimSpace(payload.Subject)
	text := strings.TrimSpace(payload.Text)
	switch {
	case subject != "" && text != "":
		return subject + "\n" + text
	case subject != "":
		return subject
	case text != "":
		return text
	default:
		return "New email received"
	}
}

func templateFunctions() template.FuncMap {
	return template.FuncMap{
		"json": func(value any) (string, error) {
			encoded, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("encode template value as JSON: %w", err)
			}
			return string(encoded), nil
		},
		"join": strings.Join,
		"truncate": func(value string, length int) string {
			if length <= 0 {
				return ""
			}
			runes := []rune(value)
			if len(runes) <= length {
				return value
			}
			return string(runes[:length])
		},
	}
}

func (target compiledTarget) matches(payload EmailPayload) bool {
	fromValues := append(append([]string(nil), payload.From...), payload.EnvelopeFrom)
	toValues := append(append([]string(nil), payload.To...), payload.EnvelopeTo...)
	toValues = append(toValues, payload.CC...)
	toValues = append(toValues, payload.BCC...)

	return matchesField(target.match.From, fromValues) &&
		matchesField(target.match.To, toValues) &&
		matchesField(target.match.Subject, []string{payload.Subject}) &&
		matchesField(target.match.Text, []string{payload.Text})
}

func matchesField(patterns, values []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		lowerPattern := strings.ToLower(pattern)
		for _, value := range values {
			matched, _ := pathMatch(lowerPattern, strings.ToLower(value))
			if matched {
				return true
			}
		}
	}
	return false
}

// pathMatch is a variable to keep the wildcard matcher easy to exercise in
// tests without exposing it as part of the public package API.
var pathMatch = func(pattern, value string) (bool, error) {
	// path.Match treats '/' as a directory separator, but webhook filters apply
	// to arbitrary email text. Replace slashes with a private-use rune that is
	// absent from both inputs so the standard glob grammar is retained without
	// giving slash special path semantics.
	const privateUseStart = rune(0xE000)
	const privateUseEnd = rune(0xF8FF)
	replacement := privateUseStart
	for ; replacement <= privateUseEnd; replacement++ {
		if !strings.ContainsRune(pattern, replacement) && !strings.ContainsRune(value, replacement) {
			mapped := string(replacement)
			return path.Match(strings.ReplaceAll(pattern, "/", mapped), strings.ReplaceAll(value, "/", mapped))
		}
	}
	return false, fmt.Errorf("cannot allocate wildcard slash sentinel")
}
