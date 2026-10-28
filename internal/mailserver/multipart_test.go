package mailserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nestedMIMEMessage(depth int) string {
	var body strings.Builder
	body.WriteString("From: sender@example.test\r\nTo: recipient@example.test\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=level-0\r\n\r\n")
	for i := 0; i < depth; i++ {
		fmt.Fprintf(&body, "--level-%d\r\nContent-Type: multipart/related; boundary=level-%d\r\n\r\n", i, i+1)
	}
	fmt.Fprintf(&body, "--level-%d\r\nContent-Type: text/html\r\n\r\n<p>nested</p>\r\n", depth)
	for i := depth; i >= 0; i-- {
		fmt.Fprintf(&body, "--level-%d--\r\n", i)
	}
	return body.String()
}

func TestNestedMIMEBudgetsAndRollback(t *testing.T) {
	var manyParts strings.Builder
	manyParts.WriteString("From: sender@example.test\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=parts\r\n\r\n")
	// Write an attachment before the part limit to exercise staged-file rollback.
	manyParts.WriteString("--parts\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=first.bin\r\n\r\nattachment\r\n")
	for i := 0; i < maximumMIMEParts; i++ {
		manyParts.WriteString("--parts\r\nContent-Type: text/plain\r\n\r\nx\r\n")
	}
	manyParts.WriteString("--parts--\r\n")
	for _, test := range []struct{ name, input, wantError string }{
		{"nesting", nestedMIMEMessage(maximumMIMEDepth), "MIME nesting exceeds"},
		{"parts", manyParts.String(), "MIME part count exceeds"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			server, err := NewMailServer(1025, "127.0.0.1", directory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = server.Close() }()
			err = server.storeIncomingEmail("mime-budget", strings.NewReader(test.input), nil)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error %v want %s", err, test.wantError)
			}
			if len(server.GetAllEmail()) != 0 {
				t.Fatal("rejected MIME became visible")
			}
			for _, path := range []string{"mime-budget.eml", "mime-budget"} {
				if _, err := os.Stat(filepath.Join(directory, path)); !os.IsNotExist(err) {
					t.Fatalf("rejected MIME artifact %s: %v", path, err)
				}
			}
		})
	}
}

func TestNestedMIMEAcceptedBelowDepthLimit(t *testing.T) {
	server, err := NewMailServer(1025, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	email, err := server.parseEmail("nested", strings.NewReader(nestedMIMEMessage(maximumMIMEDepth-1)), nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if email.HTML != "<p>nested</p>" {
		t.Fatalf("nested HTML %q", email.HTML)
	}
}

func TestMultipartAttachmentIsNotPromotedToMessageBody(t *testing.T) {
	server, err := NewMailServer(1025, "127.0.0.1", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	input := "From: sender@example.test\r\nContent-Type: multipart/mixed; boundary=outer\r\n\r\n--outer\r\nContent-Type: text/plain\r\n\r\nActual body\r\n--outer\r\nContent-Type: multipart/alternative; boundary=attached\r\nContent-Disposition: attachment; filename=attached.mime\r\n\r\n--attached\r\nContent-Type: text/html\r\n\r\n<p>Attached body</p>\r\n--attached--\r\n--outer--\r\n"
	email, err := server.parseEmail("attached", strings.NewReader(input), nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if email.Text != "Actual body" || email.HTML != "" || len(email.Attachments) != 1 || email.Attachments[0].FileName != "attached.mime" {
		t.Fatalf("multipart attachment changed message: %+v", email)
	}
}
