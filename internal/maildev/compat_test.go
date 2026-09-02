package maildev

import (
	"encoding/json"
	"net/mail"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/soulteary/owlmail/internal/types"
)

func TestFromEmailUsesMailDevDTOShape(t *testing.T) {
	received := time.Date(2026, time.January, 5, 19, 2, 9, 0, time.UTC)
	source := &types.Email{
		ID: "XwgKAxto", Time: received, Subject: "Surfers", Size: 1024,
		From: []*mail.Address{{Name: "Angelo Pappas", Address: "angelo@fbi.gov"}},
		To:   []*mail.Address{{Name: "Johnny Utah", Address: "johnny@fbi.gov"}},
		Headers: map[string]interface{}{
			"Date":        "Sun, 05 Jan 2026 19:02:09 +0000",
			"In-Reply-To": "<earlier@fbi.gov>",
			"X-Priority":  "1",
		},
		Attachments: []*types.Attachment{{
			FileName: "logo.png", GeneratedFileName: "safe.png", ContentType: "image/png",
			ContentDisposition: "attachment", ContentID: "logo@fbi.gov", Size: 24,
		}},
		Envelope: &types.Envelope{
			From: "angelo@fbi.gov", To: []string{"johnny@fbi.gov"},
			Host: "mail.test", RemoteAddress: "127.0.0.1:1234",
		},
	}

	dto := FromEmail(source, "/var/mail/owlmail")
	encoded, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	for _, fragment := range []string{
		`"source":"/var/mail/owlmail/XwgKAxto.eml"`,
		`"from":[{"address":"angelo@fbi.gov","name":"Angelo Pappas"}]`,
		`"calculatedBcc":[]`,
		`"date":"2026-01-05T19:02:09Z"`,
		`"priority":"high"`,
		`"filename":"logo.png"`,
		`"contentDisposition":"attachment"`,
		`"envelope":{"from":{"address":"angelo@fbi.gov"}`,
	} {
		if !strings.Contains(body, fragment) {
			t.Errorf("MailDev DTO %s does not contain %s", body, fragment)
		}
	}
	if strings.Contains(body, `"fileName"`) {
		t.Fatalf("OwlMail attachment field leaked into compatibility DTO: %s", body)
	}
	if dto.InReplyTo != "<earlier@fbi.gov>" {
		t.Fatalf("InReplyTo = %q", dto.InReplyTo)
	}
	if _, exists := dto.Headers["Date"]; exists {
		t.Fatalf("headers were not normalized to MailDev lowercase keys: %#v", dto.Headers)
	}
	if dto.Headers["date"] == nil {
		t.Fatalf("lowercase date header missing: %#v", dto.Headers)
	}
}

func TestToSummaryMatchesMailDevProjection(t *testing.T) {
	dto := ToSummary(&types.Email{
		ID: "summary", Subject: "Subject", Text: "  hello\n\tworld  ",
		From:        []*mail.Address{{Address: "from@example.com"}},
		To:          []*mail.Address{{Address: "to@example.com"}},
		Attachments: []*types.Attachment{{}, {}},
	})
	if dto.Preview != "hello world" || dto.AttachmentCount != 2 {
		t.Fatalf("unexpected summary: %#v", dto)
	}
	encoded, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"html"`, `"text"`, `"headers"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("body field leaked into summary: %s", encoded)
		}
	}
}

func TestToSummaryTruncatesOnUnicodeBoundaries(t *testing.T) {
	dto := ToSummary(&types.Email{Text: strings.Repeat("界", PreviewLength+1)})
	if !utf8.ValidString(dto.Preview) || len([]rune(dto.Preview)) != PreviewLength {
		t.Fatalf("preview was not truncated safely: %q", dto.Preview)
	}
}

func TestFilterAndPageUsesMailDevQueryRules(t *testing.T) {
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	emails := []Email{
		{ID: "first", Time: base, Read: false, Subject: "Welcome", From: []Address{{Address: "alice@example.com"}}},
		{ID: "second", Time: base.Add(time.Minute), Read: true, Subject: "Other", From: []Address{{Address: "bob@example.com"}}},
		{ID: "third", Time: base.Add(2 * time.Minute), Read: false, Subject: "Welcome back", From: []Address{{Address: "alice+new@example.com"}}},
	}
	limit := 1
	page := FilterAndPage(emails, map[string]string{"from.address": "alice@example.com", "read": "false"}, 0, &limit, "desc")
	if len(page) != 1 || page[0].ID != "first" {
		t.Fatalf("unexpected filtered page: %#v", page)
	}
	if got := FilterAndPage(emails, map[string]string{"subject": "welcome"}, 0, nil, ""); len(got) != 0 {
		t.Fatalf("MailDev filters must be case-sensitive exact matches: %#v", got)
	}
}

func TestEmbedAttachmentURLsUsesFacadeAndBasePath(t *testing.T) {
	html := `<img src="cid:logo@example.test"><img src="https://example.test/pixel">`
	result := EmbedAttachmentURLs(html, "/owlmail", "email-1", []Attachment{{
		ContentID: "logo@example.test", GeneratedFileName: "safe logo.png",
	}})
	if !strings.Contains(result, `src="/owlmail/api/email/email-1/attachment/safe%20logo.png"`) {
		t.Fatalf("CID URL was not rewritten through the facade: %s", result)
	}
	if !strings.Contains(result, `src="https://example.test/pixel"`) {
		t.Fatalf("unrelated image URL changed: %s", result)
	}
}

func TestParseNonNegativeIntMatchesJavaScriptPrefixParsing(t *testing.T) {
	for _, test := range []struct {
		input string
		want  int
		ok    bool
	}{{"3tail", 3, true}, {" 10 ", 10, true}, {"-1", -1, false}, {"nope", 0, false}} {
		got, ok := ParseNonNegativeInt(test.input)
		if got != test.want || ok != test.ok {
			t.Errorf("ParseNonNegativeInt(%q) = %d, %t; want %d, %t", test.input, got, ok, test.want, test.ok)
		}
	}
}
