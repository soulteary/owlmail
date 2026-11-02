package mailserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/mailbox"
	"golang.org/x/crypto/bcrypt"
)

// These env names are consumed only by this test binary, never by production
// code. The parent kills the blocked child process; no error-return path or
// graceful shutdown is used to model a crash.
const crashHelperEnv = "OWLMAIL_TEST_PROCESS_CRASH_HELPER"
const crashRootEnv = "OWLMAIL_TEST_PROCESS_CRASH_ROOT"
const crashCaseEnv = "OWLMAIL_TEST_PROCESS_CRASH_CASE"
const crashCheckpointFile = "process-crash-checkpoint.json"
const crashCleanupFile = "graceful-cleanup-ran"

type processCrashFixture struct {
	Kind         string    `json:"kind"`
	SourceID     string    `json:"sourceId,omitempty"`
	TargetID     string    `json:"targetId,omitempty"`
	PersonalID   string    `json:"personalId,omitempty"`
	RetentionNow time.Time `json:"retentionNow,omitempty"`
}
type processCrashCheckpoint struct {
	Boundary      string            `json:"boundary"`
	PID           int               `json:"pid"`
	MessageID     string            `json:"messageId,omitempty"`
	Owners        map[string]string `json:"owners"`
	TransactionID string            `json:"transactionId,omitempty"`
	JournalIDs    []string          `json:"journalIds,omitempty"`
	Committed     bool              `json:"committed"`
}

type processCrashOutput struct {
	mu      sync.Mutex
	content bytes.Buffer
}

func (o *processCrashOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	remaining := (64 << 10) - o.content.Len()
	if remaining > 0 {
		if remaining > len(p) {
			remaining = len(p)
		}
		_, _ = o.content.Write(p[:remaining])
	}
	return len(p), nil
}
func (o *processCrashOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.content.String()
}

func writeCrashJSON(t *testing.T, path string, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path+".tmp", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(encoded); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(path+".tmp", path); err != nil {
		t.Fatal(err)
	}
	if err = syncDirectory(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
}
func readCrashFixture(t *testing.T, root string) processCrashFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture processCrashFixture
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}
func crashDurableOwners(t *testing.T, root string) map[string]string {
	t.Helper()
	reader := &MailServer{mailDir: filepath.Join(root, "mail"), requireOwnership: true}
	entries, err := os.ReadDir(reader.mailDir)
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".eml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".eml")
		metadata, err := reader.loadEmailMetadata(id)
		if err != nil {
			t.Fatal(err)
		}
		if metadata.Ownership == nil {
			t.Fatal("durable email has no ownership")
		}
		owners[id] = metadata.Ownership.MailboxID
	}
	return owners
}
func processCrashBoundary(t *testing.T, root, boundary, messageID string) {
	t.Helper()
	checkpoint := processCrashCheckpoint{Boundary: boundary, PID: os.Getpid(), MessageID: messageID, Owners: crashDurableOwners(t, root)}
	stages, err := os.ReadDir(filepath.Join(root, "mail", ingressDirectory))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(stages) > 0 {
		if len(stages) != 1 {
			t.Fatal("unexpected staged transaction count")
		}
		stage := filepath.Join(root, "mail", ingressDirectory, stages[0].Name())
		journal, err := readRoutedJournal(stage)
		if err != nil {
			t.Fatal(err)
		}
		checkpoint.TransactionID = journal.TransactionID
		checkpoint.JournalIDs = journal.IDs
		_, err = os.Stat(filepath.Join(stage, ingressCommit))
		checkpoint.Committed = err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	writeCrashJSON(t, filepath.Join(root, crashCheckpointFile), checkpoint)
	// Remain inside the durable boundary with every lock and defer intact until
	// the parent uses Process.Kill. A timer keeps this a bounded child helper
	// even under the Go runtime's deadlock detector.
	time.Sleep(45 * time.Second)
	t.Fatal("parent failed to kill crash helper at durable boundary")
}
func openProcessCrashComponents(t *testing.T, root string) (*MailServer, *mailbox.Store) {
	t.Helper()
	control, err := mailbox.Open(filepath.Join(root, "control.sqlite"), false)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := mailbox.LoadAccounts(filepath.Join(root, "accounts.yaml"), "main", []string{"example.com"})
	if err != nil {
		_ = control.Close()
		t.Fatal(err)
	}
	options := &RoutedIngressOptions{Accounts: accounts, Control: control, MaxPending: 20, MaxCreatesPerMinute: 1000, MaxAuthPerMinute: 1000}
	server, err := NewMailServerWithOptions(0, "127.0.0.1", filepath.Join(root, "mail"), ServerOptions{RequireOwnership: true, AuthRequireTLS: true, TLSConfig: &TLSConfig{Enabled: true}, RoutedIngress: options})
	if err != nil {
		_ = control.Close()
		t.Fatal(err)
	}
	return server, control
}
func closeProcessCrashComponents(t *testing.T, server *MailServer, control *mailbox.Store) {
	t.Helper()
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if err := control.Close(); err != nil {
		t.Fatal(err)
	}
}
func makeProcessCrashFixture(t *testing.T, kind string) string {
	t.Helper()
	root := t.TempDir()
	control, err := mailbox.Open(filepath.Join(root, "control.sqlite"), true)
	if err != nil {
		t.Fatal(err)
	}
	if err = control.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("system-secret"), 10)
	if err != nil {
		t.Fatal(err)
	}
	account := fmt.Sprintf("version: 1\naccounts:\n  - id: gorge\n    username: gorge\n    password-hash: %s\n    namespace: main\n    enabled: true\n    allowed-domains: [example.com]\n", hash)
	if err = os.WriteFile(filepath.Join(root, "accounts.yaml"), []byte(account), 0600); err != nil {
		t.Fatal(err)
	}
	server, control := openProcessCrashComponents(t, root)
	fixture := processCrashFixture{Kind: kind}
	if kind == "merge" || kind == "retention" {
		ctx := context.Background()
		route, err := control.EnsurePending(ctx, "main", "alice@example.com", 20)
		if err != nil {
			t.Fatal(err)
		}
		fixture.SourceID = route.MailboxID
		personal, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "bob"}, mailbox.VerifiedEmail)
		if err != nil {
			t.Fatal(err)
		}
		fixture.PersonalID = personal.MailboxID
		for _, id := range []string{"pending-first", "pending-second", "personal-stable"} {
			ownership := sampleOwnership()
			ownership.MailboxID = fixture.SourceID
			if id == "personal-stable" {
				ownership.MailboxID = fixture.PersonalID
			}
			if err = server.storeIncomingEmail(id, strings.NewReader(routedMessage), &Session{mailServer: server, ownership: ownership, to: []string{"alice@example.com"}}); err != nil {
				t.Fatal(err)
			}
		}
		if kind == "merge" {
			target, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice"}, mailbox.VerifiedEmail)
			if err != nil {
				t.Fatal(err)
			}
			fixture.TargetID = target.MailboxID
			claimed, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail)
			if err != nil || claimed.MergeID == "" {
				t.Fatal("durable merge was not planned", err)
			}
		} else {
			fixture.RetentionNow = time.Now().UTC().Add(10 * 24 * time.Hour)
		}
	}
	closeProcessCrashComponents(t, server, control)
	writeCrashJSON(t, filepath.Join(root, "fixture.json"), fixture)
	return root
}

// TestProcessCrashHelper executes only in the child test process. It is not an
// application command or a release-gate bypass.
func TestProcessCrashHelper(t *testing.T) {
	if os.Getenv(crashHelperEnv) != "1" {
		return
	}
	root := os.Getenv(crashRootEnv)
	point := os.Getenv(crashCaseEnv)
	if !filepath.IsAbs(root) {
		t.Fatal("absolute crash fixture root required")
	}
	fixture := readCrashFixture(t, root)
	defer func() {
		_ = os.WriteFile(filepath.Join(root, crashCleanupFile), []byte("clean shutdown occurred"), 0600)
	}()
	server, control := openProcessCrashComponents(t, root)
	defer func() { _ = server.Close(); _ = control.Close() }()
	switch fixture.Kind {
	case "ingress":
		server.routedIngress.fault = func(stage string, index int) error {
			if stage == point && (point != "after-install" || index == 0) {
				processCrashBoundary(t, root, stage, "")
			}
			return nil
		}
		session := routedTestSession(t, server, "alice@example.com", "bob@example.com")
		if err := session.Data(strings.NewReader(routedMessage)); err != nil {
			t.Fatal(err)
		}
	case "merge":
		err := control.RunPendingMerges(context.Background(), func(ctx context.Context, item mailbox.Merge) error {
			return server.mergeOwnership(ctx, item.SourceID, item.TargetID, func(id string) error { processCrashBoundary(t, root, "merge-sidecar-persisted", id); return nil })
		})
		if err != nil {
			t.Fatal(err)
		}
	case "retention":
		err := control.CleanupPending(context.Background(), fixture.RetentionNow, 7*24*time.Hour, 24*time.Hour, func(ctx context.Context, item mailbox.PendingRetention) (mailbox.PendingRetentionResult, error) {
			return server.retainPending(ctx, item, func(id string) error { processCrashBoundary(t, root, "retention-message-deleted", id); return nil })
		})
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("unknown child crash fixture")
	}
	t.Fatal("child did not reach the requested durable crash boundary")
}

func killProcessAtBoundary(t *testing.T, root, point string) processCrashCheckpoint {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessCrashHelper$", "-test.v", "-test.timeout=40s")
	command.Env = append(os.Environ(), crashHelperEnv+"=1", crashRootEnv+"="+root, crashCaseEnv+"="+point)
	output := &processCrashOutput{}
	command.Stdout = output
	command.Stderr = output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	waited := false
	defer func() {
		if !waited {
			_ = command.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("crash child did not terminate")
			}
		}
	}()
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	var checkpoint processCrashCheckpoint
	for {
		data, err := os.ReadFile(filepath.Join(root, crashCheckpointFile))
		if err == nil {
			if err = json.Unmarshal(data, &checkpoint); err != nil {
				t.Fatal(err)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			waited = true
			t.Fatalf("child exited before durable checkpoint: %v\n%s", err, output.String())
		case <-deadline.C:
			t.Fatalf("child checkpoint timeout\n%s", output.String())
		case <-tick.C:
		}
	}
	if checkpoint.PID != command.Process.Pid || checkpoint.PID <= 0 {
		t.Fatal("checkpoint came from a different process")
	}
	// Marker file contents and the inspected disk state are durable before kill.
	// Kill is portable; on Unix it delivers SIGKILL and no Go defer can run.
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		waited = true
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || command.ProcessState.Success() {
			t.Fatal("child was not abruptly terminated", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("killed child did not exit")
	}
	if _, err := os.Stat(filepath.Join(root, crashCleanupFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("child ran graceful cleanup instead of crashing")
	}
	return checkpoint
}
func copyProcessCrashSnapshot(t *testing.T, source string) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "snapshot")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	files, err := migrationInventory(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if err = copyMigrationInventory(context.Background(), source, destination, files); err != nil {
		t.Fatal(err)
	}
	copied, err := migrationInventory(context.Background(), destination)
	if err != nil {
		t.Fatal(err)
	}
	originalHash, err := migrationFingerprint(files)
	if err != nil {
		t.Fatal(err)
	}
	copyHash, err := migrationFingerprint(copied)
	if err != nil || copyHash != originalHash {
		t.Fatal("crash snapshot differs from stopped process state")
	}
	return destination
}
func assertProcessCrashMail(t *testing.T, server *MailServer, id, owner string) {
	t.Helper()
	email, err := server.GetEmail(id)
	if err != nil || email.Ownership == nil || email.Ownership.MailboxID != owner {
		t.Fatal("restored message has incorrect owner", id, err)
	}
	source, err := server.GetRawEmailContent(id)
	if err != nil || string(source) != routedMessage {
		t.Fatal("restored raw MIME changed", id, err)
	}
	if len(email.Attachments) != 1 {
		t.Fatal("restored attachment missing", id)
	}
	path := filepath.Join(server.mailDir, id, email.Attachments[0].GeneratedFileName)
	hash, err := migrationHashFile(context.Background(), path)
	if err != nil || hash != email.Attachments[0].ContentSHA256 {
		t.Fatal("restored attachment checksum mismatch", id, err)
	}
}
func crashSnapshotFingerprint(t *testing.T, root string) string {
	t.Helper()
	files, err := migrationInventory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := migrationFingerprint(files)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func TestProcessCrashRoutedJournalRecoveryFromImmutableBackup(t *testing.T) {
	for _, point := range []string{"after-install", "before-commit", "after-commit"} {
		t.Run(point, func(t *testing.T) {
			root := makeProcessCrashFixture(t, "ingress")
			checkpoint := killProcessAtBoundary(t, root, point)
			wantInstalled := 2
			if point == "after-install" {
				wantInstalled = 1
			}
			if checkpoint.Boundary != point || len(checkpoint.JournalIDs) != 2 || len(checkpoint.Owners) != wantInstalled || checkpoint.Committed != (point == "after-commit") {
				t.Fatalf("wrong durable checkpoint: %+v", checkpoint)
			}
			originalHash := crashSnapshotFingerprint(t, root)
			backup := copyProcessCrashSnapshot(t, root)
			backupHash := crashSnapshotFingerprint(t, backup)
			restored := copyProcessCrashSnapshot(t, backup)
			server, control := openProcessCrashComponents(t, restored)
			expected := 0
			if point == "after-commit" {
				expected = 2
			}
			emails := server.GetAllEmail()
			if len(emails) != expected {
				t.Fatalf("recovery published %d messages, want %d", len(emails), expected)
			}
			for _, email := range emails {
				assertProcessCrashMail(t, server, email.ID, checkpoint.Owners[email.ID])
				view, viewErr := server.MailboxView(checkpoint.Owners[email.ID])
				if viewErr != nil {
					t.Fatal(viewErr)
				}
				for _, other := range emails {
					if other.ID != email.ID {
						if _, viewErr = view.Get(other.ID); !errors.Is(viewErr, ErrEmailNotFound) {
							t.Fatal("recovery exposed another recipient replica", viewErr)
						}
					}
				}
			}
			stages, err := os.ReadDir(filepath.Join(restored, "mail", ingressDirectory))
			if err != nil || len(stages) != 0 {
				t.Fatal("recovery journal was not completed", err)
			}
			closeProcessCrashComponents(t, server, control)
			if point == "after-commit" {
				// Retain a stale real COMMIT record from the killed process, delete one
				// accepted replica, and replay that record. It must never recreate mail.
				server, control = openProcessCrashComponents(t, restored)
				removed := checkpoint.JournalIDs[0]
				if err = server.DeleteEmail(removed); err != nil {
					t.Fatal(err)
				}
				closeProcessCrashComponents(t, server, control)
				journalSource := filepath.Join(backup, "mail", ingressDirectory, checkpoint.TransactionID)
				journalTarget := filepath.Join(restored, "mail", ingressDirectory, checkpoint.TransactionID)
				if err = os.Mkdir(journalTarget, 0700); err != nil {
					t.Fatal(err)
				}
				files, err := migrationInventory(context.Background(), journalSource)
				if err != nil {
					t.Fatal(err)
				}
				if err = copyMigrationInventory(context.Background(), journalSource, journalTarget, files); err != nil {
					t.Fatal(err)
				}
				server, control = openProcessCrashComponents(t, restored)
				if _, err = server.GetEmail(removed); !errors.Is(err, ErrEmailNotFound) {
					t.Fatal("stale accepted journal resurrected a deleted replica")
				}
				if len(server.GetAllEmail()) != 1 {
					t.Fatal("stale committed journal duplicated mail")
				}
				closeProcessCrashComponents(t, server, control)
			} else {
				server, control = openProcessCrashComponents(t, restored)
				if len(server.GetAllEmail()) != 0 {
					t.Fatal("prepared recovery changed on reopen")
				}
				closeProcessCrashComponents(t, server, control)
				for _, id := range checkpoint.JournalIDs {
					if _, err = os.Stat(filepath.Join(restored, "mail", id+".eml")); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("prepared EML survived rollback")
					}
				}
			}
			if crashSnapshotFingerprint(t, root) != originalHash || crashSnapshotFingerprint(t, backup) != backupHash {
				t.Fatal("recovery modified original crash evidence or immutable backup")
			}
			evidence, _ := json.Marshal(map[string]any{"boundary": point, "termination": "parent Process.Kill; no deferred cleanup", "installed_before_kill": wantInstalled, "committed_before_kill": checkpoint.Committed, "restored_visible_messages": expected, "raw_and_attachment_hashes_verified": true, "immutable_backup_verified": true})
			t.Logf("PROCESS_CRASH_EVIDENCE %s", evidence)
		})
	}
}

func TestProcessCrashMergeAndRetentionRecoverFromImmutableBackup(t *testing.T) {
	for _, kind := range []string{"merge", "retention"} {
		t.Run(kind, func(t *testing.T) {
			root := makeProcessCrashFixture(t, kind)
			fixture := readCrashFixture(t, root)
			checkpoint := killProcessAtBoundary(t, root, kind)
			if checkpoint.MessageID == "" {
				t.Fatal("crash did not interrupt an individual durable update")
			}
			if checkpoint.Owners["personal-stable"] != fixture.PersonalID {
				t.Fatal("personal mailbox was changed at crash checkpoint")
			}
			if kind == "merge" {
				sourceCount, targetCount := 0, 0
				for _, owner := range checkpoint.Owners {
					if owner == fixture.SourceID {
						sourceCount++
					}
					if owner == fixture.TargetID {
						targetCount++
					}
				}
				if sourceCount != 1 || targetCount != 1 {
					t.Fatal("merge checkpoint must contain one old and one new owner")
				}
			} else {
				if _, exists := checkpoint.Owners[checkpoint.MessageID]; exists || len(checkpoint.Owners) != 2 {
					t.Fatal("retention checkpoint must contain one durably deleted pending message")
				}
			}
			originalHash := crashSnapshotFingerprint(t, root)
			backup := copyProcessCrashSnapshot(t, root)
			backupHash := crashSnapshotFingerprint(t, backup)
			restored := copyProcessCrashSnapshot(t, backup)
			server, control := openProcessCrashComponents(t, restored)
			ctx := context.Background()
			if kind == "merge" {
				pending, err := control.PendingMerges(ctx)
				if err != nil || len(pending) != 1 {
					t.Fatal("merge authority journal was not retained", err)
				}
				cursor := server.DeliveryCursor()
				apply := func(ctx context.Context, item mailbox.Merge) error {
					return server.MergeOwnership(ctx, item.SourceID, item.TargetID)
				}
				if err = control.RunPendingMerges(ctx, apply); err != nil {
					t.Fatal(err)
				}
				if err = control.RunPendingMerges(ctx, apply); err != nil {
					t.Fatal(err)
				}
				for _, id := range []string{"pending-first", "pending-second"} {
					assertProcessCrashMail(t, server, id, fixture.TargetID)
				}
				if server.DeliveryCursor() != cursor {
					t.Fatal("merge replayed history as a new delivery")
				}
				pending, err = control.PendingMerges(ctx)
				if err != nil || len(pending) != 0 {
					t.Fatal("merge recovery did not durably complete", err)
				}
			} else {
				if _, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail); !errors.Is(err, mailbox.ErrRetentionPending) {
					t.Fatal("claim bypassed an interrupted cleanup", err)
				}
				if err := control.RunPendingRetention(ctx, server.RetainPending); err != nil {
					t.Fatal(err)
				}
				if err := control.RunPendingRetention(ctx, server.RetainPending); err != nil {
					t.Fatal(err)
				}
				for _, id := range []string{"pending-first", "pending-second"} {
					if _, err := server.GetEmail(id); !errors.Is(err, ErrEmailNotFound) {
						t.Fatal("pending message resurrected after retention recovery")
					}
				}
				route, err := control.EnsurePending(ctx, "main", "alice@example.com", 20)
				if err != nil || route.MailboxID == fixture.SourceID {
					t.Fatal("retired pending mailbox was reused", err)
				}
			}
			personalView, viewErr := server.MailboxView(fixture.PersonalID)
			if viewErr != nil {
				t.Fatal(viewErr)
			}
			for _, id := range []string{"pending-first", "pending-second"} {
				if _, viewErr = personalView.Get(id); !errors.Is(viewErr, ErrEmailNotFound) {
					t.Fatal("recovery exposed pending history to another user", viewErr)
				}
			}
			if kind == "merge" {
				targetView, targetErr := server.MailboxView(fixture.TargetID)
				if targetErr != nil {
					t.Fatal(targetErr)
				}
				if _, targetErr = targetView.Get("personal-stable"); !errors.Is(targetErr, ErrEmailNotFound) {
					t.Fatal("merge target exposed another personal mailbox", targetErr)
				}
			}
			assertProcessCrashMail(t, server, "personal-stable", fixture.PersonalID)
			expected := 3
			if kind == "retention" {
				expected = 1
			}
			if len(server.GetAllEmail()) != expected {
				t.Fatal("unexpected recovered message count")
			}
			closeProcessCrashComponents(t, server, control)
			// A second fresh process-equivalent open must observe the same durable end state.
			server, control = openProcessCrashComponents(t, restored)
			if len(server.GetAllEmail()) != expected {
				t.Fatal("completed recovery was not stable on reopen")
			}
			closeProcessCrashComponents(t, server, control)
			if crashSnapshotFingerprint(t, root) != originalHash || crashSnapshotFingerprint(t, backup) != backupHash {
				t.Fatal("recovery mutated crash source or backup")
			}
			evidence, _ := json.Marshal(map[string]any{"boundary": checkpoint.Boundary, "termination": "parent Process.Kill; no deferred cleanup", "durable_mutated_message": checkpoint.MessageID, "personal_untouched": true, "restored_visible_messages": expected, "recovery_idempotent": true, "immutable_backup_verified": true})
			t.Logf("PROCESS_CRASH_EVIDENCE %s", evidence)
		})
	}
}
