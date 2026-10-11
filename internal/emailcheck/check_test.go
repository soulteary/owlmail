package emailcheck

import (
	"fmt"
	"strings"
	"testing"

	"github.com/soulteary/owlmail/internal/types"
)

func TestDeterministicInspection(t *testing.T) {
	count := 1
	max := int64(10)
	email := &types.Email{Subject: "Verify account", Text: "Code 123456. Open https://app.example.test/verify", HTML: `<a href="https://app.example.test/?a=1&amp;b=2">Verify</a><img src="cid:logo">`, Attachments: []*types.Attachment{{FileName: "invoice.pdf", ContentType: "application/pdf", ContentSHA256: "abcd", Size: 5}}}
	req := Request{SubjectContains: []string{"Verify"}, TextContains: []string{"Code"}, AttachmentCount: &count, Attachments: []AttachmentExpectation{{Filename: "invoice.pdf", ContentType: "application/pdf", SHA256: "ABCD", MaxBytes: &max}}, AllowedLinkHosts: []string{"app.example.test"}, Extract: []Extraction{{Name: "code", Pattern: `Code (\d{6})`}}}
	r, err := Inspect(email, req)
	if err != nil || !r.Passed || len(r.Links) != 3 || r.Extractions["code"][0] != "123456" {
		t.Fatalf("%+v %v", r, err)
	}
	if r.Links[1].URL != "https://app.example.test/?a=1&b=2" {
		t.Fatal(r.Links)
	}
	req.TextContains = []string{"missing"}
	r, err = Inspect(email, req)
	if err != nil || r.Passed {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestInspectionRejectsAmbiguityAndBounds(t *testing.T) {
	email := &types.Email{Attachments: []*types.Attachment{{FileName: "same"}, {FileName: "same"}}}
	r, err := Inspect(email, Request{Attachments: []AttachmentExpectation{{Filename: "same"}}})
	if err != nil || r.Passed {
		t.Fatal(r, err)
	}
	for _, req := range []Request{{Extract: []Extraction{{Name: "x", Pattern: "["}}}, {Extract: []Extraction{{Name: "x", Pattern: "a"}, {Name: "x", Pattern: "b"}}}, {AllowedLinkHosts: []string{"https://example.test"}}} {
		if _, err := Inspect(email, req); err == nil {
			t.Fatal("accepted invalid selectors")
		}
	}
	email.Text = strings.Repeat("x", maxBodyBytes+1)
	if _, err := Inspect(email, Request{}); err == nil {
		t.Fatal("accepted oversized body")
	}
	email.Text = ""
	email.HTML = strings.Repeat(`<a href="https://evil.example.test/`, 1) + `">x</a>`
	r, err = Inspect(email, Request{AllowedLinkHosts: []string{"example.test"}})
	if err != nil || r.Passed {
		t.Fatal("accepted subdomain", r, err)
	}
	email.HTML = ""
	email.Text = strings.Repeat("a", maxResults+1)
	r, err = Inspect(email, Request{Extract: []Extraction{{Name: "a", Pattern: "a"}}})
	if err != nil || !r.ExtractionsTruncated || len(r.Extractions["a"]) != maxResults {
		t.Fatal(r, err)
	}
}

func TestRepeatedAndUppercaseURLsCannotBypassHostChecks(t *testing.T) {
	for _, text := range []string{strings.Repeat("https://allowed.example.test/a ", 201) + "https://evil.example.test/b", "HTTPS://evil.example.test/path"} {
		report, err := Inspect(&types.Email{Text: text}, Request{AllowedLinkHosts: []string{"allowed.example.test"}})
		if err != nil || report.Passed || report.LinksTruncated {
			t.Fatal(report, err)
		}
	}
	text := ""
	for i := 0; i < 201; i++ {
		text += fmt.Sprintf("https://allowed.example.test/%d ", i)
	}
	report, err := Inspect(&types.Email{Text: text}, Request{AllowedLinkHosts: []string{"allowed.example.test"}})
	if err != nil || report.Passed || !report.LinksTruncated || len(report.Links) != 200 {
		t.Fatal(report, err)
	}
}

func TestMalformedHTTPCandidatesFailHostAssertionsClosed(t *testing.T) {
	for _, email := range []*types.Email{{Text: "https://evil.example.test/%zz"}, {HTML: `<a href="https://evil.example.test/%zz">link</a>`}} {
		result, err := Inspect(email, Request{AllowedLinkHosts: []string{"allowed.example.test"}})
		if err != nil || result.Passed || len(result.Links) != 1 || result.Links[0].Host != "" {
			t.Fatal(result, err)
		}
	}
}

func TestHostAllowlistChecksNetworkPathReferences(t *testing.T) {
	for _, html := range []string{`<a href="//evil.example.test/path">link</a>`, `<img src=" //evil.example.test/image ">`, `<a href="//evil.example.test/%zz">link</a>`} {
		result, err := Inspect(&types.Email{HTML: html}, Request{AllowedLinkHosts: []string{"allowed.example.test"}})
		if err != nil || result.Passed {
			t.Fatal(result, err)
		}
	}
	result, err := Inspect(&types.Email{HTML: `<a href="//allowed.example.test/path">link</a><a href="/local">local</a>`}, Request{AllowedLinkHosts: []string{"allowed.example.test"}})
	if err != nil || !result.Passed {
		t.Fatal(result, err)
	}
}

func TestHostAllowlistFailsBrowserNetworkFormsClosed(t *testing.T) {
	for _, reference := range []string{`///evil.example.test/path`, `////evil.example.test/path`, `\\evil.example.test/path`, `/\evil.example.test/path`, "ht\ttps://evil.example.test/path", `//`} {
		result, err := Inspect(&types.Email{HTML: `<a href="` + reference + `">link</a>`}, Request{AllowedLinkHosts: []string{"allowed.example.test"}})
		if err != nil || result.Passed || len(result.Links) != 1 {
			t.Fatal(reference, result, err)
		}
	}
}
