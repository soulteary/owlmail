package webhook

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"path"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

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
	return matchTextPattern(pattern, value)
}

// matchTextPattern implements path.Match's glob grammar for arbitrary text.
// Unlike path.Match, slash has no separator role: *, ?, and character ranges
// treat it exactly like every other rune.
func matchTextPattern(pattern, value string) (bool, error) {
Pattern:
	for len(pattern) > 0 {
		star, chunk, rest := scanTextChunk(pattern)
		pattern = rest
		if star && chunk == "" {
			return true, nil
		}
		remainder, matched, err := matchTextChunk(chunk, value)
		if matched && (len(remainder) == 0 || len(pattern) > 0) {
			value = remainder
			continue
		}
		if err != nil {
			return false, err
		}
		if star {
			for index := 0; index < len(value); index++ {
				remainder, matched, err = matchTextChunk(chunk, value[index+1:])
				if matched {
					if len(pattern) == 0 && len(remainder) > 0 {
						continue
					}
					value = remainder
					continue Pattern
				}
				if err != nil {
					return false, err
				}
			}
		}
		for len(pattern) > 0 {
			_, chunk, pattern = scanTextChunk(pattern)
			if _, _, err := matchTextChunk(chunk, ""); err != nil {
				return false, err
			}
		}
		return false, nil
	}
	return len(value) == 0, nil
}

func scanTextChunk(pattern string) (star bool, chunk, rest string) {
	for len(pattern) > 0 && pattern[0] == '*' {
		pattern = pattern[1:]
		star = true
	}
	inRange := false
	for index := 0; index < len(pattern); index++ {
		switch pattern[index] {
		case '\\':
			if index+1 < len(pattern) {
				index++
			}
		case '[':
			inRange = true
		case ']':
			inRange = false
		case '*':
			if !inRange {
				return star, pattern[:index], pattern[index:]
			}
		}
	}
	return star, pattern, ""
}

func matchTextChunk(chunk, value string) (rest string, matched bool, err error) {
	failed := false
	for len(chunk) > 0 {
		failed = failed || len(value) == 0
		switch chunk[0] {
		case '[':
			var current rune
			if !failed {
				var size int
				current, size = utf8.DecodeRuneInString(value)
				value = value[size:]
			}
			chunk = chunk[1:]
			negated := false
			if len(chunk) > 0 && chunk[0] == '^' {
				negated = true
				chunk = chunk[1:]
			}
			classMatched := false
			ranges := 0
			for {
				if len(chunk) > 0 && chunk[0] == ']' && ranges > 0 {
					chunk = chunk[1:]
					break
				}
				var low, high rune
				if low, chunk, err = getTextEsc(chunk); err != nil {
					return "", false, err
				}
				high = low
				if chunk[0] == '-' {
					if high, chunk, err = getTextEsc(chunk[1:]); err != nil {
						return "", false, err
					}
				}
				classMatched = classMatched || low <= current && current <= high
				ranges++
			}
			failed = failed || classMatched == negated
		case '?':
			if !failed {
				_, size := utf8.DecodeRuneInString(value)
				value = value[size:]
			}
			chunk = chunk[1:]
		case '\\':
			chunk = chunk[1:]
			if len(chunk) == 0 {
				return "", false, path.ErrBadPattern
			}
			fallthrough
		default:
			if !failed {
				failed = chunk[0] != value[0]
				value = value[1:]
			}
			chunk = chunk[1:]
		}
	}
	if failed {
		return "", false, nil
	}
	return value, true, nil
}

func getTextEsc(chunk string) (r rune, rest string, err error) {
	if len(chunk) == 0 || chunk[0] == '-' || chunk[0] == ']' {
		return 0, "", path.ErrBadPattern
	}
	if chunk[0] == '\\' {
		chunk = chunk[1:]
		if len(chunk) == 0 {
			return 0, "", path.ErrBadPattern
		}
	}
	r, size := utf8.DecodeRuneInString(chunk)
	if r == utf8.RuneError && size == 1 {
		return 0, "", path.ErrBadPattern
	}
	rest = chunk[size:]
	if len(rest) == 0 {
		return 0, "", path.ErrBadPattern
	}
	return r, rest, nil
}
