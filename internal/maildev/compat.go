// Package maildev contains the opt-in MailDev REST compatibility contract.
package maildev

import (
	"fmt"
	"net/mail"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/soulteary/owlmail/internal/types"
)

const (
	DefaultPageSize = 50
	MaxPageSize     = 200
	PreviewLength   = 140
)

var integerPrefix = regexp.MustCompile(`^[+-]?\d+`)

type Address struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

type EnvelopeAddress struct {
	Address string `json:"address"`
}

type Envelope struct {
	From          EnvelopeAddress   `json:"from"`
	To            []EnvelopeAddress `json:"to"`
	Host          string            `json:"host,omitempty"`
	RemoteAddress string            `json:"remoteAddress,omitempty"`
}

type Attachment struct {
	Filename           string `json:"filename"`
	GeneratedFileName  string `json:"generatedFileName"`
	ContentType        string `json:"contentType"`
	ContentDisposition string `json:"contentDisposition"`
	ContentID          string `json:"contentId,omitempty"`
	Size               int64  `json:"size,omitempty"`
}

// Email is deliberately independent from OwlMail's native DTO. Its JSON names,
// optional fields, envelope shape, and attachment shape follow MailDev's REST
// contract without changing /api/v1.
type Email struct {
	ID            string                 `json:"id"`
	Time          time.Time              `json:"time"`
	Read          bool                   `json:"read"`
	Subject       string                 `json:"subject"`
	Source        string                 `json:"source"`
	Size          int64                  `json:"size"`
	SizeHuman     string                 `json:"sizeHuman"`
	From          []Address              `json:"from"`
	To            []Address              `json:"to"`
	CC            []Address              `json:"cc,omitempty"`
	BCC           []Address              `json:"bcc,omitempty"`
	CalculatedBCC []Address              `json:"calculatedBcc"`
	Date          *time.Time             `json:"date,omitempty"`
	HTML          string                 `json:"html,omitempty"`
	Text          string                 `json:"text,omitempty"`
	Headers       map[string]interface{} `json:"headers"`
	InReplyTo     string                 `json:"inReplyTo,omitempty"`
	Priority      string                 `json:"priority,omitempty"`
	Attachments   []Attachment           `json:"attachments"`
	Envelope      Envelope               `json:"envelope"`
}

type Summary struct {
	ID              string    `json:"id"`
	Time            time.Time `json:"time"`
	Read            bool      `json:"read"`
	Subject         string    `json:"subject"`
	Size            int64     `json:"size"`
	SizeHuman       string    `json:"sizeHuman"`
	From            []Address `json:"from"`
	To              []Address `json:"to"`
	CC              []Address `json:"cc,omitempty"`
	AttachmentCount int       `json:"attachmentCount"`
	Preview         string    `json:"preview"`
}

type SummaryResponse struct {
	Items      []Summary `json:"items"`
	Total      int       `json:"total"`
	StoreTotal int       `json:"storeTotal"`
	Unread     int       `json:"unread"`
	Skip       int       `json:"skip"`
	Limit      int       `json:"limit"`
}

type BulkDeleteResponse struct {
	Deleted  []string `json:"deleted"`
	NotFound []string `json:"notFound"`
}

type ConfigResponse struct {
	Version         string  `json:"version"`
	SMTPPort        int     `json:"smtpPort"`
	OutgoingEnabled bool    `json:"isOutgoingEnabled"`
	OutgoingHost    *string `json:"outgoingHost"`
}

func FromEmail(email *types.Email, mailDir string) Email {
	result := Email{
		ID:            email.ID,
		Time:          email.Time,
		Read:          email.Read,
		Subject:       email.Subject,
		Source:        filepath.Join(mailDir, email.ID+".eml"),
		Size:          email.Size,
		SizeHuman:     email.SizeHuman,
		From:          addresses(email.From),
		To:            addresses(email.To),
		CC:            addressesOrNil(email.CC),
		BCC:           addressesOrNil(email.BCC),
		CalculatedBCC: addresses(email.CalculatedBCC),
		HTML:          email.HTML,
		Text:          email.Text,
		Headers:       normalizeHeaders(email.Headers),
		Attachments:   make([]Attachment, 0, len(email.Attachments)),
		Envelope: Envelope{
			To: make([]EnvelopeAddress, 0),
		},
	}
	if result.SizeHuman == "" {
		result.SizeHuman = formatBytes(result.Size)
	}
	if email.Envelope != nil {
		result.Envelope.From = EnvelopeAddress{Address: email.Envelope.From}
		result.Envelope.To = envelopeAddresses(email.Envelope.To)
		result.Envelope.Host = email.Envelope.Host
		result.Envelope.RemoteAddress = email.Envelope.RemoteAddress
		if len(result.CalculatedBCC) == 0 {
			result.CalculatedBCC = stringAddresses(email.Envelope.CalculatedBCC)
		}
	}
	for _, attachment := range email.Attachments {
		if attachment == nil {
			continue
		}
		disposition := "attachment"
		if attachment.ContentID != "" {
			disposition = "inline"
		}
		result.Attachments = append(result.Attachments, Attachment{
			Filename: attachment.FileName, GeneratedFileName: attachment.GeneratedFileName,
			ContentType: attachment.ContentType, ContentDisposition: disposition,
			ContentID: attachment.ContentID, Size: attachment.Size,
		})
	}
	if value := headerValue(result.Headers, "date"); value != "" {
		if parsed, err := mail.ParseDate(value); err == nil {
			parsed = parsed.UTC()
			result.Date = &parsed
		}
	}
	result.InReplyTo = headerValue(result.Headers, "in-reply-to")
	result.Priority = priority(result.Headers)
	return result
}

func ToSummary(email *types.Email) Summary {
	preview := ""
	if email.Text != "" {
		preview = strings.Join(strings.Fields(email.Text), " ")
		if len(preview) > PreviewLength {
			preview = preview[:PreviewLength]
		}
	}
	result := Summary{
		ID: email.ID, Time: email.Time, Read: email.Read, Subject: email.Subject,
		Size: email.Size, SizeHuman: email.SizeHuman, From: addresses(email.From),
		To: addresses(email.To), CC: addressesOrNil(email.CC),
		AttachmentCount: len(email.Attachments), Preview: preview,
	}
	if result.SizeHuman == "" {
		result.SizeHuman = formatBytes(result.Size)
	}
	return result
}

func FilterAndPage(emails []Email, filters map[string]string, skip int, limit *int, order string) []Email {
	filtered := make([]Email, 0, len(emails))
	for _, email := range emails {
		if matchesFilters(email, filters) {
			filtered = append(filtered, email)
		}
	}
	if order == "asc" || order == "desc" {
		sort.SliceStable(filtered, func(i, j int) bool {
			if order == "asc" {
				return filtered[i].Time.Before(filtered[j].Time)
			}
			return filtered[i].Time.After(filtered[j].Time)
		})
	}
	if skip < 0 {
		skip = 0
	}
	if skip >= len(filtered) {
		return []Email{}
	}
	filtered = filtered[skip:]
	if limit != nil && *limit < len(filtered) {
		filtered = filtered[:*limit]
	}
	return filtered
}

// EmbedAttachmentURLs replaces MailDev-style CID references with facade URLs.
func EmbedAttachmentURLs(html, basePath, emailID string, attachments []Attachment) string {
	for _, attachment := range attachments {
		if attachment.ContentID == "" {
			continue
		}
		pattern := regexp.MustCompile(`src=("|')cid:` + regexp.QuoteMeta(attachment.ContentID) + `("|')`)
		target := basePath + "/api/email/" + url.PathEscape(emailID) + "/attachment/" + url.PathEscape(attachment.GeneratedFileName)
		html = pattern.ReplaceAllString(html, `src="`+target+`"`)
	}
	return html
}

func ParseNonNegativeInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	prefix := integerPrefix.FindString(value)
	if prefix == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(prefix)
	return parsed, err == nil && parsed >= 0
}

func addresses(source []*mail.Address) []Address {
	result := make([]Address, 0, len(source))
	for _, address := range source {
		if address != nil {
			result = append(result, Address{Address: address.Address, Name: address.Name})
		}
	}
	return result
}

func addressesOrNil(source []*mail.Address) []Address {
	if len(source) == 0 {
		return nil
	}
	return addresses(source)
}

func envelopeAddresses(source []string) []EnvelopeAddress {
	result := make([]EnvelopeAddress, 0, len(source))
	for _, address := range source {
		result = append(result, EnvelopeAddress{Address: address})
	}
	return result
}

func stringAddresses(source []string) []Address {
	result := make([]Address, 0, len(source))
	for _, address := range source {
		result = append(result, Address{Address: address})
	}
	return result
}

func normalizeHeaders(headers map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(headers))
	for key, value := range headers {
		switch typed := value.(type) {
		case string:
			result[strings.ToLower(key)] = typed
		case []string:
			result[strings.ToLower(key)] = append([]string(nil), typed...)
		}
	}
	return result
}

func headerValue(headers map[string]interface{}, key string) string {
	value, ok := headers[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		if len(typed) > 0 {
			return typed[0]
		}
	}
	return ""
}

func priority(headers map[string]interface{}) string {
	importance := strings.ToLower(headerValue(headers, "importance"))
	if importance == "high" || importance == "low" {
		return importance
	}
	value := strings.TrimSpace(headerValue(headers, "x-priority"))
	if value == "1" || value == "2" {
		return "high"
	}
	if value == "4" || value == "5" {
		return "low"
	}
	if value != "" || headerValue(headers, "priority") != "" {
		return "normal"
	}
	return ""
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == units[len(units)-1] {
			if value == float64(int64(value)) {
				return fmt.Sprintf("%.0f %s", value, unit)
			}
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}

func matchesFilters(email Email, filters map[string]string) bool {
	for path, expected := range filters {
		values := nestedValues(reflect.ValueOf(email), strings.Split(path, "."))
		matched := false
		for _, value := range values {
			if matchesValue(value, expected) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func nestedValues(value reflect.Value, path []string) []interface{} {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil
	}
	if len(path) == 0 {
		return []interface{}{value.Interface()}
	}
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		if index, err := strconv.Atoi(path[0]); err == nil {
			if index < 0 || index >= value.Len() {
				return nil
			}
			return nestedValues(value.Index(index), path[1:])
		}
		var result []interface{}
		for i := 0; i < value.Len(); i++ {
			result = append(result, nestedValues(value.Index(i), path)...)
		}
		return result
	}
	if value.Kind() == reflect.Map && value.Type().Key().Kind() == reflect.String {
		entry := value.MapIndex(reflect.ValueOf(path[0]).Convert(value.Type().Key()))
		return nestedValues(entry, path[1:])
	}
	if value.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" {
			name = field.Name
		}
		if name == path[0] {
			return nestedValues(value.Field(i), path[1:])
		}
	}
	return nil
}

func matchesValue(actual interface{}, expected string) bool {
	switch typed := actual.(type) {
	case string:
		return typed == expected
	case bool:
		return strconv.FormatBool(typed) == expected
	case int:
		return strconv.Itoa(typed) == expected
	case int64:
		return strconv.FormatInt(typed, 10) == expected
	case time.Time:
		return typed.String() == expected
	default:
		return fmt.Sprint(actual) == expected
	}
}
