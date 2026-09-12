package mailserver

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Every fuzz target in this file truncates rather than skips oversized input.
// SMTP DATA is already capped by the protocol limiter, and the shapes worth
// finding here are grammatical -- an unbalanced boundary, a class that never
// closes -- not long ones. Truncating keeps the seed corpus a fast unit test
// on the ordinary `go test ./...` path while still exercising the mutation the
// fuzzer produced.
const (
	fuzzMessageLimit = 32 << 10
	fuzzHTMLLimit    = 16 << 10
	fuzzHeaderLimit  = 1 << 10
)

// fuzzActiveElements execute, submit, fetch, or reframe on their own. None has
// ever been on sanitizeHTML's allowlist, so one reaching the output is a
// bypass of the allowlist rather than a loosening of it -- and the sanitized
// HTML is what the web UI renders in its preview.
var fuzzActiveElements = map[string]struct{}{
	"script": {}, "iframe": {}, "object": {}, "embed": {}, "applet": {},
	"form": {}, "input": {}, "button": {}, "select": {}, "textarea": {},
	"style": {}, "base": {}, "meta": {}, "frame": {}, "frameset": {},
	"svg": {}, "math": {}, "template": {}, "portal": {},
}

// fuzzURLAttributes carry exactly one URL that a browser will dereference or
// navigate to. srcset is deliberately absent: its value is a comma-separated
// candidate list with descriptors, so the single-URL check below would inspect
// only the first candidate and report an assertion stronger than it makes.
// bluemonday's policy drops srcset outright, so nothing is lost by leaving the
// weaker check unwritten rather than writing one that reads as complete.
var fuzzURLAttributes = map[string]struct{}{
	"href": {}, "src": {}, "action": {}, "formaction": {},
	"background": {}, "poster": {}, "data": {}, "codebase": {},
	"cite": {}, "longdesc": {}, "manifest": {}, "xlink:href": {},
}

// fuzzActiveURLSchemes run code or smuggle a document in place of a fetch.
// sanitizeHTML allows only mailto, http, https, cid, and relative references.
var fuzzActiveURLSchemes = []string{"javascript:", "vbscript:", "data:", "blob:", "file:"}

// FuzzSanitizeHTML drives the sanitizer that stands between a captured HTML
// body and the web UI preview. The body is attacker-controlled in full, and
// the preview renders the sanitizer's output, so a single surviving handler or
// scheme is a stored cross-site scripting hole in a mailbox that routinely
// holds password-reset links and verification codes.
//
// The assertions read the sanitized output as a browser does rather than
// searching it for substrings: bluemonday escapes text, so a message whose
// body is the literal words "javascript:" or "<script>" legitimately keeps
// them as text, and a substring check would both miss an obfuscated attribute
// and reject a harmless one.
//
// Idempotence is asserted even though no path sanitizes an already-sanitized
// body today: both call sites work from HTML freshly parsed out of the raw
// .eml, and the raw bytes are what storeIncomingEmail persists. It is asserted
// because a sanitizer whose second pass differs from its first is producing
// output that is not in the language its own policy accepts, which is the shape
// of every mutation-XSS bug -- markup that survives one pass and is re-parsed
// into something else on the next. It is also the precondition any future
// re-render path would silently depend on.
//
// Two known gaps are deliberately not asserted, because they describe current
// behavior and an assertion would be a failing gate rather than a finding:
// `style="border: url(/track.png)"` and `style="width: expression(alert(1))"`
// both survive the inline-style policy. They are carried as seeds instead, so a
// change to either the property allowlist or the value pattern is measured
// against them. sanitizeHTML's own comment now states this boundary; it used to
// claim url() and expression() were excluded, which they are not.
func FuzzSanitizeHTML(f *testing.F) {
	seeds := []string{
		"",
		"plain text with no markup at all",
		"javascript:alert(1)",
		"<",
		"<<<>>>",
		`<script>top.location='https://attacker.test'</script>`,
		`<form action="https://attacker.test"><input name="secret"></form>`,
		`<a href="javascript:alert(1)" target="_top">bad</a>`,
		`<img src="javascript:alert(2)" onerror="alert(3)">`,
		`<a href=" javascript:alert(1)">leading space</a>`,
		`<a href="java&#09;script:alert(1)">entity split</a>`,
		`<a href="https://example.com/path" target="_self" rel="opener">external</a>`,
		`<link rel="stylesheet" href="https://cdn.example.test/mail.css" type="text/css" media="screen">`,
		`<link rel="preload" href="https://tracker.example.test/pixel">`,
		`<table style="width: 100%; border-collapse: collapse"><tr><td style="color: #123456; padding: 8px">` +
			`<img src="cid:logo@example.test" style="max-width: 240px"></td></tr></table>`,
		`<div style="background-image: url(https://tracker.example.test/pixel); color: red; position: fixed">safe</div>`,
		// The two inline-style values the policy lets through: a relative url()
		// and a bare function call, both inside an allowed property. They are
		// seeds rather than assertions because they describe what the sanitizer
		// does today, and a change to the property allowlist or to
		// safeInlineStyleValue has to be measured against them.
		`<div style="border: url(/track.png)">relative url in an allowed property</div>`,
		`<div style="width: expression(alert(1))">bare function call</div>`,
		`<svg><script>alert(1)</script></svg>`,
		`<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>`,
		`<noscript><p title="</noscript><img src=x onerror=alert(1)>">`,
		`<div><p>unterminated`,
		`<a href="//example.com">protocol relative</a>`,
		`<base href="https://attacker.test/">`,
		"&lt;script&gt;alert(1)&lt;/script&gt;",
		"&amp;lt;",
		strings.Repeat("<div>", 200) + "deep" + strings.Repeat("</div>", 200),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > fuzzHTMLLimit {
			raw = raw[:fuzzHTMLLimit]
		}

		sanitized := sanitizeHTML(raw)
		assertNoActiveContent(t, sanitized)

		if again := sanitizeHTML(sanitized); again != sanitized {
			t.Fatalf("sanitizeHTML is not idempotent for %q:\nfirst:  %q\nsecond: %q", raw, sanitized, again)
		}
	})
}

// assertNoActiveContent parses sanitized markup the way a browser would and
// reports any element or attribute that could still run or fetch.
func assertNoActiveContent(t *testing.T, sanitized string) {
	t.Helper()

	context := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	roots, err := html.ParseFragment(strings.NewReader(sanitized), context)
	if err != nil {
		t.Fatalf("sanitized HTML does not parse: %v\noutput: %q", err, sanitized)
	}

	pending := append([]*html.Node(nil), roots...)
	for len(pending) > 0 {
		node := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			pending = append(pending, child)
		}
		if node.Type != html.ElementNode {
			continue
		}
		name := strings.ToLower(node.Data)
		if _, active := fuzzActiveElements[name]; active {
			t.Fatalf("sanitized HTML kept the <%s> element: %q", name, sanitized)
		}
		for _, attribute := range node.Attr {
			attributeName := strings.ToLower(attribute.Key)
			if attribute.Namespace != "" {
				attributeName = strings.ToLower(attribute.Namespace) + ":" + attributeName
			}
			// No HTML attribute outside the event handlers begins with "on",
			// so the prefix is a sound test and covers handlers this list
			// would otherwise have to enumerate.
			if strings.HasPrefix(attributeName, "on") {
				t.Fatalf("sanitized HTML kept the %q event handler: %q", attributeName, sanitized)
			}
			if _, dereferenced := fuzzURLAttributes[attributeName]; !dereferenced {
				continue
			}
			normalized := normalizeFuzzURL(attribute.Val)
			for _, scheme := range fuzzActiveURLSchemes {
				if strings.HasPrefix(normalized, scheme) {
					t.Fatalf("sanitized HTML kept a %s URL in %q: %q", scheme, attributeName, sanitized)
				}
			}
		}
	}
}

// normalizeFuzzURL folds an attribute value the way a browser does before it
// picks a scheme: whitespace and control characters are dropped wherever they
// appear, so "java\tscript:" and " javascript:" are the same URL, and case is
// not significant in a scheme.
func normalizeFuzzURL(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r <= ' ' || r == 0x7f {
			continue
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}

// FuzzParseEmailMessage drives the MIME entry point that every captured
// message goes through: SMTP DATA on the way in, and every stored .eml on the
// way back out during recovery. Its input is fully attacker-controlled, and a
// panic here does not merely lose one message -- it takes down the SMTP
// listener for every other session on the process.
//
// The assertions are about the contract the callers depend on rather than
// about any particular parse. Of the five call sites -- storeIncomingEmail,
// parseEmail, LoadMailsFromDirectory, the read-only loader, and the attachment
// migration -- four branch on the error and then dereference the result without
// a nil check, so a nil-with-no-error return is an immediate crash rather than
// a handled case. Attachments are asserted to carry a digest because the
// storage and deduplication layers key on it.
func FuzzParseEmailMessage(f *testing.F) {
	server, err := NewMailServer(0, "localhost", f.TempDir())
	if err != nil {
		f.Fatalf("NewMailServer() error = %v", err)
	}
	f.Cleanup(func() { _ = server.Close() })
	// The complete header projection is off by default, so leaving it off here
	// would leave collectAllHeaders -- which walks every field of an untrusted
	// header block -- outside the target.
	server.SetRetainAllHeaders(true)

	seeds := []string{
		"",
		"not an email at all",
		"From: from@example.com\r\n",
		"\r\n\r\nbody with no headers at all\r\n",
		"From: from@example.com\r\nTo: to@example.com\r\nSubject: Plain\r\n\r\nPlain body\r\n",
		"From: from@example.com\r\nTo: to@example.com\r\nContent-Type: text/html\r\n\r\n" +
			"<html><body><img src=x onerror=alert(1)></body></html>\r\n",
		"From: from@example.com\r\n" +
			"To: to@example.com\r\n" +
			"Subject: Multipart Test\r\n" +
			"Date: Mon, 02 Jan 2006 15:04:05 -0700\r\n" +
			"Content-Type: multipart/alternative; boundary=\"boundary123\"\r\n\r\n" +
			"--boundary123\r\nContent-Type: text/plain\r\n\r\nPlain text body\r\n" +
			"--boundary123\r\nContent-Type: text/html\r\n\r\n<html><body>HTML body</body></html>\r\n" +
			"--boundary123--\r\n",
		"From: from@example.com\r\n" +
			"To: to@example.com\r\n" +
			"Content-Type: multipart/related; boundary=\"boundary123\"\r\n\r\n" +
			"--boundary123\r\nContent-Type: text/html\r\n\r\n<img src=\"cid:image123\">\r\n" +
			"--boundary123\r\nContent-Type: image/png\r\nContent-ID: <image123>\r\n\r\nPNG content here\r\n" +
			"--boundary123--\r\n",
		// A boundary that is announced and never closed, and one that never
		// appears at all: the two ways a truncated transfer ends.
		"Content-Type: multipart/mixed; boundary=broken\r\n\r\n--broken\r\nContent-Type: text/plain\r\n\r\nhalf",
		"Content-Type: multipart/mixed; boundary=absent\r\n\r\nno part ever starts\r\n",
		// Base64 that stops being base64 part way through, which must surface
		// as an error rather than as a silently truncated attachment.
		"Content-Type: multipart/mixed; boundary=broken\r\n\r\n--broken\r\n" +
			"Content-Type: application/octet-stream\r\n" +
			"Content-Transfer-Encoding: base64\r\n" +
			"Content-Disposition: attachment; filename=broken.bin\r\n\r\naGVsbG8=%%%\r\n--broken--\r\n",
		"Content-Transfer-Encoding: cosmic-ray\r\n\r\nbody\r\n",
		"Content-Type: multipart/mixed\r\n\r\nboundary parameter is missing entirely\r\n",
		"Content-Type: multipart/mixed; boundary=\"unterminated\r\n\r\nbody\r\n",
		"Content-Type: text/plain; charset=\"unterminated\r\n\r\nbody\r\n",
		"From: \"unterminated quote <from@example.com>\r\nTo: to@example.com\r\n\r\nbody\r\n",
		"From: =?utf-8?q?broken?=\r\nSubject: =?utf-8?b?!!!!?=\r\n\r\nbody\r\n",
		"Subject: " + strings.Repeat("folded\r\n ", 64) + "\r\n\r\nbody\r\n",
		// Nested multipart: the parser descends one level only, so the inner
		// document has to be consumed as an opaque part without stalling.
		"Content-Type: multipart/mixed; boundary=outer\r\n\r\n--outer\r\n" +
			"Content-Type: multipart/mixed; boundary=inner\r\n\r\n--inner\r\n" +
			"Content-Type: text/plain\r\n\r\nnested\r\n--inner--\r\n--outer--\r\n",
		"Content-Type: multipart/mixed; boundary=b\r\n\r\n" +
			strings.Repeat("--b\r\nContent-Type: text/plain\r\n\r\npart\r\n", 32) + "--b--\r\n",
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > fuzzMessageLimit {
			raw = raw[:fuzzMessageLimit]
		}

		email, envelope, err := server.parseEmailMessage("fuzz", bytes.NewReader(raw), nil, false, "")
		if err != nil {
			if email != nil || envelope != nil {
				t.Fatalf("parseEmailMessage() returned (%v, %v) alongside %v", email, envelope, err)
			}
			return
		}
		if email == nil || envelope == nil {
			t.Fatalf("parseEmailMessage() returned email=%v envelope=%v with no error", email, envelope)
		}
		if email.Headers == nil {
			t.Fatal("parseEmailMessage() returned an email with a nil header map")
		}
		// A message parsed off disk during recovery must never be labeled as
		// having arrived over SMTP: the webhook payload and the stored
		// metadata both report that flag as provenance.
		if envelope.SMTPTransaction {
			t.Fatal("parseEmailMessage() marked an SMTP transaction without a session")
		}
		if len(envelope.To) != len(email.To) {
			t.Fatalf("envelope lists %d recipients for %d parsed To addresses", len(envelope.To), len(email.To))
		}
		for name, addresses := range map[string][]*mail.Address{
			"From": email.From, "To": email.To, "Cc": email.CC, "Bcc": email.BCC,
		} {
			for index, address := range addresses {
				if address == nil {
					t.Fatalf("%s address %d is nil, which every address consumer dereferences", name, index)
				}
			}
		}
		for index, attachment := range email.Attachments {
			if attachment == nil {
				t.Fatalf("attachment %d is nil", index)
			}
			if attachment.ContentType == "" || attachment.FileName == "" {
				t.Fatalf("attachment %d has no content type or file name: %+v", index, attachment)
			}
			if attachment.Size < 0 {
				t.Fatalf("attachment %d reports a negative size: %d", index, attachment.Size)
			}
			// Deduplication and the attachment migration both key on this
			// digest, so an attachment that reaches the store without one
			// corrupts the storage layer rather than the message.
			if len(attachment.ContentSHA256) != 64 {
				t.Fatalf("attachment %d carries digest %q, want 64 hex characters", index, attachment.ContentSHA256)
			}
		}

		// The same bytes must parse to the same message. Header handling walks
		// maps, and a result that depends on that iteration order would make a
		// recovered mailbox differ from the one that was captured.
		repeated, _, repeatedErr := server.parseEmailMessage("fuzz", bytes.NewReader(raw), nil, false, "")
		if repeatedErr != nil {
			t.Fatalf("parseEmailMessage() succeeded then failed for the same input: %v", repeatedErr)
		}
		// Without this the nil-with-no-error return -- the failure this target
		// exists to report -- would surface on the second parse as a nil
		// dereference panic in the comparison below, which is the least legible
		// output the target can produce for its own headline case.
		if repeated == nil {
			t.Fatalf("parseEmailMessage() returned a nil email with a nil error on the second parse of %q", raw)
		}
		if repeated.Subject != email.Subject || repeated.Text != email.Text || repeated.HTML != email.HTML {
			t.Fatalf("parseEmailMessage() is not deterministic for %q", raw)
		}
		if len(repeated.Attachments) != len(email.Attachments) {
			t.Fatalf("parseEmailMessage() found %d attachments then %d", len(email.Attachments), len(repeated.Attachments))
		}
	})
}

// FuzzParseEmailDate drives the Date fallback that runs whenever go-message
// refuses a header. Its input is a raw header value off the wire, and the
// result becomes the message's sort key, its retention age, and the timestamp
// the API and webhooks publish.
//
// The fallback to time.Now is the only source of nondeterminism in the
// function, so requiring every other input to parse identically twice is a
// real constraint rather than a restatement: it fails if any layout in the
// list resolves against the current clock instead of against the header.
func FuzzParseEmailDate(f *testing.F) {
	seeds := []string{
		"",
		" ",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999999999+07:00",
		"Mon, 02 Jan 2006 15:04:05 -0700 (GMT)",
		"Mon, 02 Jan 2006 15:04:05 +0000 (UTC)",
		"Tue, 30 Feb 2006 25:61:61 -0700",
		"Mon, 02 Jan 200600000000000 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -9999",
		"not a date",
		"0000-00-00T00:00:00Z",
		// Both of these parse cleanly -- the first against RFC1123Z, the first
		// layout tried -- and both used to return the zero time, which the
		// storage layer reads as "no time recorded". They are permanent seeds
		// because they are the crasher this target found, and because they are
		// the shape an attacker would send: a sender choosing a sentinel that
		// means something else downstream.
		"Mon, 01 Jan 0001 00:00:00 +0000",
		"0001-01-01T00:00:00Z",
		"\x00\x01\x02",
		strings.Repeat("Mon, ", 64),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > fuzzHeaderLimit {
			raw = raw[:fuzzHeaderLimit]
		}
		// A header value is a single line by construction; a value carrying a
		// line ending would describe a different header block, not a different
		// date, and go-message refuses to set one.
		if strings.ContainsAny(raw, "\r\n") {
			return
		}

		headers := message.Header{}
		headers.Set("Date", raw)
		stored := headers.Get("Date")

		before := time.Now()
		parsed := parseEmailDate(headers)
		repeated := parseEmailDate(headers)
		after := time.Now()

		// The zero time sorts before every real message and reads as year 1 in
		// the API, so the fallback must never produce one.
		if parsed.IsZero() {
			t.Fatalf("parseEmailDate(%q) returned the zero time", raw)
		}
		if parsed.Location() == nil {
			t.Fatalf("parseEmailDate(%q) returned a time with no location", raw)
		}
		if !parsed.Equal(repeated) {
			if parsed.Before(before) || parsed.After(after) || repeated.Before(before) || repeated.After(after) {
				t.Fatalf("parseEmailDate(%q) returned %v then %v without falling back to the clock", raw, parsed, repeated)
			}
		}

		// RFC 1123 with a numeric zone is the first layout tried and the form
		// every conforming sender emits, so a value in that form must be
		// returned exactly rather than approximated by a later layout. The one
		// exception is a value that parses to the zero time: the storage layer
		// spells "no time recorded" that way, so parseEmailDate routes it to the
		// clock fallback rather than handing a sender that sentinel.
		if stored != "" {
			expected, parseErr := time.Parse(time.RFC1123Z, stored)
			switch {
			case parseErr != nil:
			case expected.IsZero():
				if parsed.Before(before) || parsed.After(after) {
					t.Fatalf("parseEmailDate(%q) = %v, want the clock fallback for a zero-time header", raw, parsed)
				}
			case !parsed.Equal(expected):
				t.Fatalf("parseEmailDate(%q) = %v, want the RFC 1123 value %v", raw, parsed, expected)
			}
		}
	})
}
