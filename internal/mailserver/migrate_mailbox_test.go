package mailserver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/soulteary/owlmail/internal/mailbox"
)

const migrationTestMIME = "From: sender@example.com\r\nTo: alice@example.com\r\nSubject: migrate\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/plain\r\n\r\nhello\r\n--x\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=test.txt\r\nContent-Transfer-Encoding: base64\r\n\r\naGVsbG8=\r\n--x--\r\n"

func standaloneMigrationSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	server, err := NewMailServer(0, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = server.storeIncomingEmail("source-email", strings.NewReader(migrationTestMIME), &Session{from: "sender@example.com", to: []string{"alice@example.com"}}); err != nil {
		t.Fatal(err)
	}
	if err = server.ReadEmail("source-email"); err != nil {
		t.Fatal(err)
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}
func migrationImportOptions(t *testing.T, source string) MailboxMigrationOptions {
	t.Helper()
	return MailboxMigrationOptions{Source: source, Destination: filepath.Join(t.TempDir(), "bundle"), Direction: "standalone-to-multiuser", Namespace: "main", Subject: "stable-alice", Offline: true}
}
func TestMailboxMigrationRoundTripAndIdempotence(t *testing.T) {
	ctx := context.Background()
	source := standaloneMigrationSource(t)
	before, err := migrationInventory(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	options := migrationImportOptions(t, source)
	result, err := MigrateMailbox(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Emails != 1 || result.Attachments != 1 || result.MailboxID == "" {
		t.Fatalf("unexpected result %+v", result)
	}
	after, err := migrationInventory(ctx, source)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("migration changed source")
	}
	archive, err := migrationInventory(ctx, filepath.Join(options.Destination, "source-archive"))
	if err != nil || !reflect.DeepEqual(before, archive) {
		t.Fatal("backup did not preserve complete original source")
	}
	isolated := &MailServer{mailDir: filepath.Join(options.Destination, "mail"), requireOwnership: true}
	metadata, err := isolated.loadEmailMetadata("source-email")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Ownership.MailboxID != result.MailboxID || !metadata.Read || metadata.Version != 4 {
		t.Fatal("ownership or read state not preserved")
	}
	control, err := mailbox.OpenReadOnly(filepath.Join(options.Destination, "control.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	personal, err := control.PersonalMailbox(ctx, "main", "stable-alice")
	if err != nil || personal != result.MailboxID {
		t.Fatal("explicit owner missing")
	}
	_ = control.Close()
	repeated, err := MigrateMailbox(ctx, options)
	if err != nil || !repeated.AlreadyComplete || repeated.MailboxID != result.MailboxID {
		t.Fatal("migration rerun not idempotent", err)
	}
	export := MailboxMigrationOptions{Source: filepath.Join(options.Destination, "mail"), Destination: filepath.Join(t.TempDir(), "export"), Direction: "multiuser-to-standalone", MailboxID: result.MailboxID, ControlDatabase: filepath.Join(options.Destination, "control.sqlite"), Offline: true}
	exported, err := MigrateMailbox(ctx, export)
	if err != nil {
		t.Fatal(err)
	}
	if exported.Emails != 1 || exported.Attachments != 1 {
		t.Fatal(exported)
	}
	reader := &MailServer{mailDir: filepath.Join(export.Destination, "mail")}
	plain, err := reader.loadEmailMetadata("source-email")
	if err != nil {
		t.Fatal(err)
	}
	if plain.Ownership != nil || plain.Version != 3 || !plain.Read {
		t.Fatal("standalone export did not remove ownership")
	}
	raw, err := os.ReadFile(filepath.Join(export.Destination, "mail", "source-email.eml"))
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(source, "source-email.eml"))
	if err != nil || string(raw) != string(original) {
		t.Fatal("raw message changed")
	}
	// The full authority database is not copied into a single-person export.
	if _, err = os.Stat(filepath.Join(export.Destination, "control.sqlite")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("global authority leaked into export")
	}
	if err = os.WriteFile(filepath.Join(options.Destination, "mail", "source-email.eml"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = MigrateMailbox(ctx, options); err == nil {
		t.Fatal("idempotence trusted a tampered destination")
	}
}
func TestMailboxMigrationSourceChangeAndFailureRollback(t *testing.T) {
	source := standaloneMigrationSource(t)
	options := migrationImportOptions(t, source)
	options.afterCopy = func() error {
		return os.WriteFile(filepath.Join(source, "concurrent.eml"), []byte("Subject: concurrent\r\n\r\nbody"), 0600)
	}
	if _, err := MigrateMailbox(context.Background(), options); err == nil {
		t.Fatal("concurrent source modification accepted")
	}
	if _, err := os.Stat(options.Destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed destination was published")
	}
	entries, err := os.ReadDir(filepath.Dir(options.Destination))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("staging/lock left behind after failure")
	}
	if _, err := os.Stat(filepath.Join(source, "source-email.eml")); err != nil {
		t.Fatal("original removed")
	}
}
func TestMailboxMigrationRejectsUnsafeInputs(t *testing.T) {
	for _, scenario := range []string{"offline", "nested", "symlink", "queue", "foreign-direction", "tampered-attachment"} {
		t.Run(scenario, func(t *testing.T) {
			source := standaloneMigrationSource(t)
			options := migrationImportOptions(t, source)
			switch scenario {
			case "offline":
				options.Offline = false
			case "nested":
				options.Destination = filepath.Join(source, "nested")
			case "symlink":
				if err := os.Symlink(filepath.Join(source, "source-email.eml"), filepath.Join(source, "unsafe")); err != nil {
					t.Fatal(err)
				}
			case "queue":
				dir := filepath.Join(source, metadataDirectoryName, relayJobMetadataDirectoryName)
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "pending.json"), []byte(`{"status":"queued"}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "foreign-direction":
				options.Direction = "multiuser-to-standalone"
				options.Subject = ""
				options.MailboxID = uuid.NewString()
				options.ControlDatabase = "/does/not/exist"
			case "tampered-attachment":
				reader := &MailServer{mailDir: source}
				metadata, err := reader.loadEmailMetadata("source-email")
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(source, "source-email", metadata.Attachments[0].GeneratedFileName)
				if err = os.WriteFile(path, []byte("wrong"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := MigrateMailbox(context.Background(), options); err == nil {
				t.Fatal("unsafe migration accepted")
			}
		})
	}
}
func TestMailboxExportDoesNotIncludeOtherPersonalMailboxes(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	server := isolatedServer(t, source)
	controlPath := filepath.Join(t.TempDir(), "control.sqlite")
	control, err := mailbox.Open(controlPath, true)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, user := range []string{"alice", "bob"} {
		owner, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: user}, mailbox.ExplicitHistory)
		if err != nil {
			t.Fatal(err)
		}
		ids[user] = owner.MailboxID
		ownership := sampleOwnership()
		ownership.MailboxID = owner.MailboxID
		if err = server.storeIncomingEmail(user, strings.NewReader("Subject: "+user+"\r\n\r\nsecret-"+user), &Session{ownership: ownership}); err != nil {
			t.Fatal(err)
		}
	}
	if err = server.Close(); err != nil {
		t.Fatal(err)
	}
	if err = control.Close(); err != nil {
		t.Fatal(err)
	}
	options := MailboxMigrationOptions{Source: source, Destination: filepath.Join(t.TempDir(), "bundle"), Direction: "multiuser-to-standalone", MailboxID: ids["alice"], ControlDatabase: controlPath, Offline: true}
	result, err := MigrateMailbox(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Emails != 1 {
		t.Fatal(result)
	}
	for _, directory := range []string{"mail", "source-archive"} {
		if _, err = os.Stat(filepath.Join(options.Destination, directory, "bob.eml")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("foreign email exported")
		}
	}
	data, err := os.ReadFile(filepath.Join(options.Destination, migrationManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "bob") {
		t.Fatal("foreign identity leaked into manifest")
	}
	var manifest MailboxMigrationResult
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.MailboxID != ids["alice"] {
		t.Fatal("export lost selected identity")
	}
}

func TestMailboxMigrationInterruptedCopyLeavesSourceAndNoDestination(t *testing.T) {
	source := standaloneMigrationSource(t)
	before, err := migrationInventory(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	options := migrationImportOptions(t, source)
	options.afterCopy = func() error { return errors.New("simulated interruption before publication") }
	if _, err = MigrateMailbox(context.Background(), options); err == nil {
		t.Fatal("fault was ignored")
	}
	after, err := migrationInventory(context.Background(), source)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed migration mutated source")
	}
	if _, err = os.Stat(options.Destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("incomplete destination exposed")
	}
	entries, err := os.ReadDir(filepath.Dir(options.Destination))
	if err != nil || len(entries) != 0 {
		t.Fatal("failed migration left temporary data")
	}
	options.afterCopy = nil
	if _, err = MigrateMailbox(context.Background(), options); err != nil {
		t.Fatal("clean retry after failure did not succeed", err)
	}
}

func TestMailboxMigrationRequiresExactOwnerAndRefusesGlobalHistoryExport(t *testing.T) {
	source := standaloneMigrationSource(t)
	options := migrationImportOptions(t, source)
	imported, err := MigrateMailbox(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	export := MailboxMigrationOptions{Source: filepath.Join(options.Destination, "mail"), Destination: filepath.Join(t.TempDir(), "export"), Direction: "multiuser-to-standalone", MailboxID: uuid.NewString(), ControlDatabase: filepath.Join(options.Destination, "control.sqlite"), Offline: true}
	if _, err = MigrateMailbox(context.Background(), export); err == nil {
		t.Fatal("non-personal mailbox accepted")
	}
	export.MailboxID = imported.MailboxID
	history := filepath.Join(export.Source, metadataDirectoryName, "webhook-history")
	if err = os.MkdirAll(history, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(history, "history.json"), []byte(`{"records":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = MigrateMailbox(context.Background(), export); err == nil {
		t.Fatal("global history silently dropped or copied into single-person export")
	}
}

func TestMailboxExportRejectsInterruptedOrQuotaDeferredMergeOnlyForSelectedBox(t *testing.T) {
	for _, scenario := range []string{"interrupted-file-merge", "quota-deferred-merge"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			source := t.TempDir()
			server := isolatedServer(t, source)
			controlPath := filepath.Join(t.TempDir(), "control.sqlite")
			control, err := mailbox.Open(controlPath, true)
			if err != nil {
				t.Fatal(err)
			}
			alice, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice"}, mailbox.VerifiedEmail)
			if err != nil {
				t.Fatal(err)
			}
			bob, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "bob"}, mailbox.VerifiedEmail)
			if err != nil {
				t.Fatal(err)
			}
			route, err := control.EnsurePending(ctx, "main", "alice@example.com", 10)
			if err != nil {
				t.Fatal(err)
			}
			for id, owner := range map[string]string{"alice-original": alice.MailboxID, "alice-pending-first": route.MailboxID, "alice-pending-second": route.MailboxID, "bob-only": bob.MailboxID} {
				ownership := sampleOwnership()
				ownership.MailboxID = owner
				if err = server.storeIncomingEmail(id, strings.NewReader("Subject: "+id+"\r\n\r\noriginal"), &Session{ownership: ownership}); err != nil {
					t.Fatal(err)
				}
			}
			claimed, err := control.BindLogin(ctx, mailbox.Login{Namespace: "main", Subject: "alice", Email: "alice@example.com", Verified: true}, mailbox.VerifiedEmail)
			if err != nil || claimed.MergeID == "" {
				t.Fatal("merge not planned", err)
			}
			interruption := errors.New("interrupted after one sidecar commit")
			moved := 0
			err = control.RunPendingMerges(ctx, func(ctx context.Context, item mailbox.Merge) error {
				if scenario == "quota-deferred-merge" {
					return mailbox.ErrQuotaExceeded
				}
				return server.mergeOwnership(ctx, item.SourceID, item.TargetID, func(string) error { moved++; return interruption })
			})
			if scenario == "interrupted-file-merge" {
				if !errors.Is(err, interruption) || moved != 1 {
					t.Fatal("interrupted merge was not modeled", err, moved)
				}
			} else if err != nil {
				t.Fatal("quota deferral should retain journal without a fatal error", err)
			}
			pending, err := control.PendingMerges(ctx)
			if err != nil || len(pending) != 1 {
				t.Fatal("merge journal was not retained", pending, err)
			}
			if err = server.Close(); err != nil {
				t.Fatal(err)
			}
			if err = control.Close(); err != nil {
				t.Fatal(err)
			}
			before, err := migrationInventory(ctx, source)
			if err != nil {
				t.Fatal(err)
			}
			options := MailboxMigrationOptions{Source: source, Destination: filepath.Join(t.TempDir(), "alice-export"), Direction: "multiuser-to-standalone", MailboxID: alice.MailboxID, ControlDatabase: controlPath, Offline: true}
			if _, err = MigrateMailbox(ctx, options); !errors.Is(err, mailbox.ErrMailboxUnavailable) {
				t.Fatal("incomplete personal history was exported", err)
			}
			if _, err = os.Stat(options.Destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("partial-history export was published")
			}
			after, err := migrationInventory(ctx, source)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("rejected export changed source")
			}
			options.MailboxID = bob.MailboxID
			options.Destination = filepath.Join(t.TempDir(), "bob-export")
			exported, err := MigrateMailbox(ctx, options)
			if err != nil || exported.Emails != 1 {
				t.Fatal("unrelated pending merge blocked Bob export", err)
			}
			if _, err = os.Stat(filepath.Join(options.Destination, "mail", "bob-only.eml")); err != nil {
				t.Fatal(err)
			}
		})
	}
}
