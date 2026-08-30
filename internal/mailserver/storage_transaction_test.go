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
	if err := os.Mkdir(filepath.Join(dir, "orphan01"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "backups"), 0755); err != nil {
		t.Fatal(err)
	}

	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("corrupt email was published: %d", got)
	}
	for _, name := range []string{"corrupt.eml", storageTempPrefix + "partial.eml.tmp", "orphan01"} {
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
	if _, err := os.Stat(filepath.Join(dir, "backups")); err != nil {
		t.Fatalf("unrelated directory was quarantined: %v", err)
	}
}

func TestRecoveryFailureDoesNotHideCommittedMail(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "valid001.eml"), validMessage("valid"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, storageTempPrefix+"partial.eml.tmp"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, quarantineDirName), []byte("blocks quarantine directory"), 0600); err != nil {
		t.Fatal(err)
	}

	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if got := len(server.GetAllEmail()); got != 1 {
		t.Fatalf("valid committed mail count = %d, want 1", got)
	}
}
