package mailserver

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validMessage(subject string) []byte {
	return []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		"body")
}

func multipartMessage() []byte {
	return []byte("From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: attachment\r\n" +
		"Content-Type: multipart/mixed; boundary=boundary123\r\n\r\n" +
		"--boundary123\r\nContent-Type: text/plain\r\n\r\nbody\r\n" +
		"--boundary123\r\nContent-Type: text/plain\r\n" +
		"Content-Disposition: attachment; filename=test.txt\r\n\r\ndata\r\n" +
		"--boundary123--\r\n")
}

func assertNoCommittedOrTemporaryArtifacts(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == quarantineDirName {
			continue
		}
		if _, ok := rollbackFenceID(entry.Name()); ok {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".eml") || strings.HasPrefix(entry.Name(), storageTempPrefix) {
			t.Fatalf("unexpected storage artifact after rollback: %s", entry.Name())
		}
	}
}

func TestStoreIncomingEmailRollsBackAfterMemoryCommitFailure(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	server.beforeStoreCommit = func(*Email) error { return errors.New("injected commit failure") }

	err = server.storeIncomingEmail("atomic-message", bytes.NewReader(validMessage("atomic")), nil)
	if err == nil || !strings.Contains(err.Error(), "injected commit failure") {
		t.Fatalf("expected injected commit failure, got %v", err)
	}
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("memory store contains %d email(s) after rollback", got)
	}
	assertNoCommittedOrTemporaryArtifacts(t, dir)
}

func TestStoreIncomingEmailRollsBackDurableHandoffFailure(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.OnSynchronous("new", func(*Email) error {
		return errors.New("injected outbox failure")
	}); err != nil {
		t.Fatal(err)
	}

	err = server.storeIncomingEmail("handoff-message", bytes.NewReader(multipartMessage()), nil)
	if err == nil || !strings.Contains(err.Error(), "injected outbox failure") {
		t.Fatalf("expected durable handoff failure, got %v", err)
	}
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("memory store contains %d email(s) after handoff rollback", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "handoff-message")); !os.IsNotExist(err) {
		t.Fatalf("attachment directory survived handoff rollback: %v", err)
	}
	if _, err := os.Stat(server.metadataPath("handoff-message")); !os.IsNotExist(err) {
		t.Fatalf("metadata survived handoff rollback: %v", err)
	}
	assertNoCommittedOrTemporaryArtifacts(t, dir)
}

func TestFailedHandoffFencesEMLWhenImmediateCleanupFails(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.OnSynchronous("new", func(*Email) error {
		return errors.New("injected outbox failure")
	}); err != nil {
		t.Fatal(err)
	}
	server.beforeEmailRollback = func(string) error {
		return errors.New("injected EML cleanup failure")
	}

	err = server.storeIncomingEmail("fenced-handoff", bytes.NewReader(validMessage("fenced")), nil)
	if err == nil || !strings.Contains(err.Error(), "injected outbox failure") {
		t.Fatalf("expected durable handoff failure, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fenced-handoff.eml")); err != nil {
		t.Fatalf("fault injection did not retain the EML: %v", err)
	}
	if _, err := os.Stat(rollbackFencePath(dir, "fenced-handoff")); err != nil {
		t.Fatalf("rollback fence missing: %v", err)
	}

	restarted, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = restarted.Close() }()
	if got := len(restarted.GetAllEmail()); got != 0 {
		t.Fatalf("restart loaded %d rollback-fenced email(s)", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "fenced-handoff.eml")); !os.IsNotExist(err) {
		t.Fatalf("recovery retained rollback-fenced EML: %v", err)
	}
	if state, err := readRollbackFenceState(rollbackFencePath(dir, "fenced-handoff")); err != nil || state != rollbackFenceState {
		t.Fatalf("durable rollback fence after recovery = %q, %v", state, err)
	}
}

func TestRecoveryLoadsEmailWithStaleAcceptedFence(t *testing.T) {
	dir := t.TempDir()
	id := "accepted-fence"
	if err := os.WriteFile(filepath.Join(dir, id+".eml"), validMessage("accepted"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackFencePath(dir, id), []byte(acceptedFenceState+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if got := len(server.GetAllEmail()); got != 1 {
		t.Fatalf("restart loaded %d email(s) with accepted fence, want 1", got)
	}
	if _, err := os.Stat(rollbackFencePath(dir, id)); !os.IsNotExist(err) {
		t.Fatalf("stale accepted fence survived recovery: %v", err)
	}
}

func TestDeleteAllPreservesTransactionFences(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	fencePath := rollbackFencePath(dir, "deleted-accepted")
	if err := os.WriteFile(fencePath, []byte(acceptedFenceState+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.DeleteAllEmail(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fencePath); err != nil {
		t.Fatalf("bulk deletion removed transaction fence: %v", err)
	}
}

func TestStoreIncomingEmailRollsBackAttachmentFailure(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	server.beforeAttachmentWrite = func(string) error { return errors.New("injected attachment failure") }

	err = server.storeIncomingEmail("attachment-message", bytes.NewReader(multipartMessage()), nil)
	if err == nil || !strings.Contains(err.Error(), "injected attachment failure") {
		t.Fatalf("expected injected attachment failure, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "attachment-message")); !os.IsNotExist(statErr) {
		t.Fatalf("attachment directory survived rollback: %v", statErr)
	}
	assertNoCommittedOrTemporaryArtifacts(t, dir)
}

func TestStoreIncomingEmailCommitsCompleteMessage(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := server.storeIncomingEmail("complete-message", bytes.NewReader(multipartMessage()), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "complete-message.eml")); err != nil {
		t.Fatalf("committed EML missing: %v", err)
	}
	attachments, err := os.ReadDir(filepath.Join(dir, "complete-message"))
	if err != nil {
		t.Fatalf("committed attachment directory missing: %v", err)
	}
	if len(attachments) != 1 || strings.HasSuffix(attachments[0].Name(), ".tmp") {
		t.Fatalf("unexpected committed attachments: %#v", attachments)
	}
	if got := len(server.GetAllEmail()); got != 1 {
		t.Fatalf("expected one published email, got %d", got)
	}
}

func TestLoadMailsQuarantinesCorruptIncompleteAndOrphanArtifacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "corrupt.eml"), []byte("not a message"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, storageTempPrefix+"partial.eml.tmp"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Ab12Cd34"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "archive"), 0755); err != nil {
		t.Fatal(err)
	}

	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("corrupt email was published: %d", got)
	}
	for _, name := range []string{"corrupt.eml", storageTempPrefix + "partial.eml.tmp", "Ab12Cd34"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("artifact %s was not moved to quarantine: %v", name, err)
		}
	}
	quarantined, err := os.ReadDir(filepath.Join(dir, quarantineDirName))
	if err != nil {
		t.Fatal(err)
	}
	if len(quarantined) != 3 {
		t.Fatalf("expected three quarantine entries, got %d", len(quarantined))
	}
	if _, err := os.Stat(filepath.Join(dir, "archive")); err != nil {
		t.Fatalf("unrelated directory was treated as an orphan: %v", err)
	}
}

func TestLoadMailsContinuesAfterQuarantineFailure(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}

	validID := "Ef56Gh78"
	if err := os.WriteFile(filepath.Join(dir, validID+".eml"), validMessage("recoverable"), 0644); err != nil {
		t.Fatal(err)
	}
	temporaryName := storageTempPrefix + "blocked.eml.tmp"
	if err := os.WriteFile(filepath.Join(dir, temporaryName), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	server.beforeQuarantineMove = func(path string) error {
		if filepath.Base(path) == temporaryName {
			return errors.New("injected quarantine failure")
		}
		return nil
	}

	err = server.LoadMailsFromDirectory()
	if err == nil || !strings.Contains(err.Error(), "injected quarantine failure") {
		t.Fatalf("expected recovery error, got %v", err)
	}
	if _, err := server.GetEmail(validID); err != nil {
		t.Fatalf("valid email was not restored after recovery error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, temporaryName)); err != nil {
		t.Fatalf("failed artifact should remain available for a later retry: %v", err)
	}
}
