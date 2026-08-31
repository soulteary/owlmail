package mailserver

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestFailedHandoffWaitsForRollbackCleanup(t *testing.T) {
	server, err := NewMailServer(1025, "localhost", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.OnSynchronous("new", func(*Email) error {
		return errors.New("injected handoff failure")
	}); err != nil {
		t.Fatal(err)
	}
	cleanupStarted := make(chan struct{})
	releaseCleanup := make(chan struct{})
	server.On("new-rollback", func(*Email) {
		close(cleanupStarted)
		<-releaseCleanup
	})

	completed := make(chan error, 1)
	go func() {
		completed <- server.SaveEmailToStore(
			"retry-ordering",
			false,
			&Envelope{From: "sender@example.com", To: []string{"recipient@example.com"}},
			&Email{Subject: "failed handoff"},
		)
	}()
	<-cleanupStarted
	select {
	case err := <-completed:
		t.Fatalf("failed save returned before rollback cleanup finished: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(releaseCleanup)
	if err := <-completed; err == nil || !strings.Contains(err.Error(), "injected handoff failure") {
		t.Fatalf("failed save error = %v", err)
	}
}

func TestRecoveryConvertsMalformedFenceToRollback(t *testing.T) {
	dir := t.TempDir()
	id := "malformed-fence"
	if err := os.WriteFile(filepath.Join(dir, id+".eml"), validMessage("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackFencePath(dir, id), []byte("acc"), 0600); err != nil {
		t.Fatal(err)
	}

	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("recovery loaded %d malformed-fenced email(s)", got)
	}
	if state, err := readRollbackFenceState(rollbackFencePath(dir, id)); err != nil || state != rollbackFenceState {
		t.Fatalf("recovered malformed fence = %q, %v", state, err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".eml")); !os.IsNotExist(err) {
		t.Fatalf("malformed-fenced EML survived conservative recovery: %v", err)
	}
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

func TestRecoveryLoadsEmailAndPreservesAcceptedFenceForWebhookRecovery(t *testing.T) {
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
	if state, err := readRollbackFenceState(rollbackFencePath(dir, id)); err != nil || state != acceptedFenceState {
		t.Fatalf("accepted fence needed by webhook recovery = %q, %v", state, err)
	}
}

func TestRecoveryRemovesCompletedLocalFence(t *testing.T) {
	dir := t.TempDir()
	id := "local-fence"
	if err := os.WriteFile(filepath.Join(dir, id+".eml"), validMessage("local"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollbackFencePath(dir, id), []byte(localFenceState+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	if got := len(server.GetAllEmail()); got != 1 {
		t.Fatalf("restart loaded %d email(s) with local fence, want 1", got)
	}
	if _, err := os.Stat(rollbackFencePath(dir, id)); !os.IsNotExist(err) {
		t.Fatalf("completed local fence survived recovery: %v", err)
	}
}

func TestSaveEmailToStorePersistsAcceptedWebhookHandoff(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.OnSynchronous("new", func(*Email) error { return nil }); err != nil {
		t.Fatal(err)
	}

	id := "non-smtp-handoff"
	if err := server.SaveEmailToStore(
		id,
		false,
		&Envelope{From: "sender@example.com", To: []string{"recipient@example.com"}},
		&Email{Subject: "accepted outside SMTP"},
	); err != nil {
		t.Fatal(err)
	}
	if state, err := readRollbackFenceState(rollbackFencePath(dir, id)); err != nil || state != acceptedFenceState {
		t.Fatalf("non-SMTP accepted handoff fence = %q, %v", state, err)
	}
}

func TestSaveEmailToStoreKeepsCommitAfterAcceptedFenceDirectorySyncFailure(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.OnSynchronous("new", func(*Email) error { return nil }); err != nil {
		t.Fatal(err)
	}
	server.syncAcceptedFenceDirectory = func(string) error {
		return errors.New("injected accepted-fence directory sync failure")
	}

	id := "accepted-sync-error"
	if err := server.SaveEmailToStore(
		id,
		false,
		&Envelope{From: "sender@example.com", To: []string{"recipient@example.com"}},
		&Email{Subject: "accepted despite directory sync error"},
	); err != nil {
		t.Fatalf("accepted handoff was reported as rolled back: %v", err)
	}
	if _, err := server.GetEmail(id); err != nil {
		t.Fatalf("accepted email was not published: %v", err)
	}
	if state, err := readRollbackFenceState(rollbackFencePath(dir, id)); err != nil || state != acceptedFenceState {
		t.Fatalf("accepted handoff fence after directory sync error = %q, %v", state, err)
	}
}

func TestReloadPersistsAcceptedWebhookHandoff(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.OnSynchronous("new", func(*Email) error { return nil }); err != nil {
		t.Fatal(err)
	}

	id := "reload-handoff"
	if err := os.WriteFile(filepath.Join(dir, id+".eml"), validMessage("reload handoff"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := server.LoadMailsFromDirectory(); err != nil {
		t.Fatal(err)
	}
	if state, err := readRollbackFenceState(rollbackFencePath(dir, id)); err != nil || state != acceptedFenceState {
		t.Fatalf("reloaded accepted handoff fence = %q, %v", state, err)
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
	if _, err := os.Stat(rollbackFencePath(dir, "complete-message")); !os.IsNotExist(err) {
		t.Fatalf("local-only delivery retained a transaction fence: %v", err)
	}
}

func TestReloadWaitsForActiveStorageTransaction(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server.beforeStoreCommit = func(*Email) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	}
	stored := make(chan error, 1)
	go func() {
		stored <- server.storeIncomingEmail("reload-race", bytes.NewReader(validMessage("reload")), nil)
	}()
	<-started

	reloaded := make(chan error, 1)
	go func() { reloaded <- server.LoadMailsFromDirectory() }()
	select {
	case err := <-reloaded:
		t.Fatalf("reload completed during an active storage transaction: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-stored; err != nil {
		t.Fatal(err)
	}
	if err := <-reloaded; err != nil {
		t.Fatal(err)
	}
	if _, err := server.GetEmail("reload-race"); err != nil {
		t.Fatalf("committed email missing after serialized reload: %v", err)
	}
}

func TestDeleteAllWaitsForActiveStorageTransaction(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server.beforeStoreCommit = func(*Email) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	}
	stored := make(chan error, 1)
	go func() {
		stored <- server.storeIncomingEmail("delete-all-race", bytes.NewReader(validMessage("delete all")), nil)
	}()
	<-started

	deleted := make(chan error, 1)
	go func() { deleted <- server.DeleteAllEmail() }()
	select {
	case err := <-deleted:
		t.Fatalf("delete-all completed during an active storage transaction: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-stored; err != nil {
		t.Fatal(err)
	}
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("delete-all retained %d committed email(s)", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "delete-all-race.eml")); !os.IsNotExist(err) {
		t.Fatalf("delete-all retained the committed EML: %v", err)
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
