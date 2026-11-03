package mailserver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/owlmail/internal/common"
)

const privateSubject = "subject-private-OTP-694281"
const privateSender = "sender-private@example.test"
const privateBody = "body-private-password-token-382197"
const privateDate = "date-private-cookie-session-937126"
const privateCredential = "credential-private-Cookie-Authorization-821349"

func privateMessage() string {
	return "From: " + privateSender + "\r\nTo: alice@example.com\r\nSubject: " + privateSubject +
		"\r\nDate: " + privateDate + "\r\nCookie: " + privateCredential +
		"\r\nX-Auth-User: " + privateCredential + "\r\n\r\n" + privateBody
}

func captureOwnedMailLog(t *testing.T) *processCrashOutput {
	t.Helper()
	output := &processCrashOutput{}
	common.InitLoggerOutputWithFormat(common.LogLevelVerbose, "json", output)
	t.Cleanup(func() { common.InitLogger(common.LogLevelNormal) })
	return output
}

func assertPrivateMailAbsent(t *testing.T, output string) {
	t.Helper()
	for _, secret := range []string{privateSubject, privateSender, privateBody, privateDate, privateCredential, "system-secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("mail or credential content reached the log: %q", secret)
		}
	}
}

func TestOwnedMailLogsRedactStartupRestoreAndScopedDelete(t *testing.T) {
	output := captureOwnedMailLog(t)
	dir := t.TempDir()
	server := isolatedServer(t, dir)
	ownership := sampleOwnership()
	if err := server.storeIncomingEmail("owned-private-id", strings.NewReader(privateMessage()), &Session{mailServer: server, ownership: ownership, to: []string{"alice@example.com"}}); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	// Startup reparses EML before attaching persisted ownership. The server's
	// mode must protect those diagnostics even before Email.Ownership exists.
	server = isolatedServer(t, dir)
	mail, err := server.GetEmail("owned-private-id")
	if err != nil || mail.Subject != privateSubject || mail.Text != privateBody || mail.From[0].Address != privateSender {
		t.Fatal("privacy logging changed stored message content", err)
	}
	view, err := server.MailboxView(ownership.MailboxID)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := view.Delete([]string{"owned-private-id"}); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	logs := output.String()
	assertPrivateMailAbsent(t, logs)
	for _, operation := range []string{"Saving owned email: id: owned-private-id", "Restored owned email: id: owned-private-id", "Deleting owned email: id: owned-private-id", "class: invalid_date"} {
		if !strings.Contains(logs, operation) {
			t.Fatalf("missing safe diagnostic %q", operation)
		}
	}
}

type privateErrorReader struct{ failure error }

func (r privateErrorReader) Read([]byte) (int, error) { return 0, r.failure }

func TestOwnedParserErrorsRemainClassifiableWithoutQuotingContent(t *testing.T) {
	output := captureOwnedMailLog(t)
	server := isolatedServer(t, t.TempDir())
	failure := errors.New(privateCredential)
	_, _, err := server.parseEmailMessage("safe-internal-id", privateErrorReader{failure}, nil, false, "")
	if err == nil || !errors.Is(err, failure) {
		t.Fatal("safe diagnostic lost the original error identity", err)
	}
	var parsed *ownedMessageParseError
	if !errors.As(err, &parsed) {
		t.Fatal("missing safe parser diagnostic")
	}
	common.Error("Forwarded diagnostic: %v", err)
	assertPrivateMailAbsent(t, output.String()+fmt.Sprintf("%v %+v", err, err))
	if !strings.Contains(err.Error(), "safe-internal-id") || !strings.Contains(err.Error(), "class: message_parse") {
		t.Fatal("missing internal ID/classification", err)
	}
}

func TestRoutedSMTPLogsDoNotQuoteHeadersOrReaderErrors(t *testing.T) {
	output := captureOwnedMailLog(t)
	options, _ := routedOptions(t)
	server := newRoutedTestServer(t, t.TempDir(), options)
	session := routedTestSession(t, server, "alice@example.com")
	if err := session.Mail(privateSender, nil); err != nil {
		t.Fatal(err)
	}
	if err := session.Data(strings.NewReader(privateMessage())); err != nil {
		t.Fatal(err)
	}
	for _, reader := range []io.Reader{
		privateErrorReader{errors.New(privateCredential)},
		strings.NewReader(privateCredential + "\r\n\r\n" + privateBody),
	} {
		session = routedTestSession(t, server, "alice@example.com")
		if err := session.Data(reader); err == nil {
			t.Fatal("invalid DATA was accepted")
		}
	}
	logs := output.String()
	assertPrivateMailAbsent(t, logs)
	for _, classification := range []string{"class: invalid_date", "class: staging", "class: submission"} {
		if !strings.Contains(logs, classification) {
			t.Fatalf("missing safe routed diagnostic %q", classification)
		}
	}
}

func TestOwnedCorruptMIMERestoreLogsOnlyInternalDiagnostic(t *testing.T) {
	output := captureOwnedMailLog(t)
	dir := t.TempDir()
	server := isolatedServer(t, dir)
	ownership := sampleOwnership()
	if err := server.storeIncomingEmail("corrupt-private-id", strings.NewReader(privateMessage()), &Session{mailServer: server, ownership: ownership, to: []string{"alice@example.com"}}); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corrupt-private-id.eml"), []byte(privateCredential+"\r\n\r\n"+privateBody), 0600); err != nil {
		t.Fatal(err)
	}
	server = isolatedServer(t, dir)
	if _, err := server.GetEmail("corrupt-private-id"); !errors.Is(err, ErrEmailNotFound) {
		t.Fatal("corrupt message was not quarantined", err)
	}
	logs := output.String()
	assertPrivateMailAbsent(t, logs)
	if !strings.Contains(logs, "class: message_parse") {
		t.Fatal("missing generic corrupt-MIME diagnostic")
	}
}

func TestOwnedHeaderBackfillDoesNotLogMalformedMIME(t *testing.T) {
	output := captureOwnedMailLog(t)
	dir := t.TempDir()
	server := isolatedServer(t, dir)
	if err := server.storeIncomingEmail("backfill-private-id", strings.NewReader(privateMessage()), &Session{mailServer: server, ownership: sampleOwnership(), to: []string{"alice@example.com"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "backfill-private-id.eml"), []byte(privateCredential+"\r\n\r\n"+privateBody), 0600); err != nil {
		t.Fatal(err)
	}
	// Public multiuser configuration disables compatibility projections, but
	// the ownership-aware core must also protect explicit header backfills.
	server.SetRetainAllHeaders(true)
	logs := output.String()
	assertPrivateMailAbsent(t, logs)
	if !strings.Contains(logs, "Failed to parse owned email headers: id: backfill-private-id, class: message_parse") {
		t.Fatal("missing safe header-backfill diagnostic")
	}
}

func TestStandaloneMailLoggingPreservesExistingDiagnostics(t *testing.T) {
	output := captureOwnedMailLog(t)
	server, err := NewMailServer(0, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if err := server.storeIncomingEmail("standalone-log-id", strings.NewReader(privateMessage()), &Session{mailServer: server, to: []string{"alice@example.com"}}); err != nil {
		t.Fatal(err)
	}
	if err := server.DeleteEmail("standalone-log-id"); err != nil {
		t.Fatal(err)
	}
	logs := output.String()
	for _, expected := range []string{"Saving email: " + privateSubject, "Deleting email - " + privateSubject, privateDate} {
		if !strings.Contains(logs, expected) {
			t.Fatalf("standalone diagnostic changed: missing %q", expected)
		}
	}
}
