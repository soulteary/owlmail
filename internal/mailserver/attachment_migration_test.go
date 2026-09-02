package mailserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/owlmail/internal/attachmentstore"
)

type migrationFakeStore struct {
	objects     map[string][]byte
	putCalls    map[string]int
	openCalls   map[string]int
	putFailures map[string]int
	blockPut    bool
}

func newMigrationFakeStore() *migrationFakeStore {
	return &migrationFakeStore{
		objects: make(map[string][]byte), putCalls: make(map[string]int),
		openCalls: make(map[string]int), putFailures: make(map[string]int),
	}
}

func (store *migrationFakeStore) Put(ctx context.Context, emailID, filename, _ string, body io.Reader, _ int64) error {
	key := emailID + "/" + filename
	store.putCalls[key]++
	if store.blockPut {
		<-ctx.Done()
		return ctx.Err()
	}
	if remaining := store.putFailures[key]; remaining != 0 {
		if remaining > 0 {
			store.putFailures[key]--
		}
		return errors.New("injected upload failure")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	store.objects[key] = data
	return nil
}

func (store *migrationFakeStore) Open(_ context.Context, emailID, filename string) (*attachmentstore.Object, error) {
	key := emailID + "/" + filename
	store.openCalls[key]++
	data, ok := store.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return &attachmentstore.Object{Body: io.NopCloser(bytes.NewReader(data)), Size: int64(len(data))}, nil
}

func (store *migrationFakeStore) DeleteEmail(_ context.Context, emailID string) error {
	for key := range store.objects {
		if strings.HasPrefix(key, emailID+"/") {
			delete(store.objects, key)
		}
	}
	return nil
}

func (store *migrationFakeStore) CheckHealth(context.Context) error { return nil }

func createLocalMigrationMessage(t *testing.T, directory, id string, message []byte) (string, string) {
	t.Helper()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.storeIncomingEmail(id, bytes.NewReader(message), nil); err != nil {
		_ = server.Close()
		t.Fatal(err)
	}
	email, err := server.GetEmail(id)
	if err != nil || len(email.Attachments) != 1 {
		_ = server.Close()
		t.Fatalf("stored email = %#v, %v", email, err)
	}
	filename := email.Attachments[0].GeneratedFileName
	path := filepath.Join(directory, id, filename)
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	return filename, path
}

func migrationTestOptions() AttachmentMigrationOptions {
	return AttachmentMigrationOptions{AttemptTimeout: time.Second, RetryDelay: 0}
}

func migrationMetadata(t *testing.T, directory, id string) emailMetadata {
	t.Helper()
	metadata, err := (&MailServer{mailDir: directory}).loadEmailMetadata(id)
	if err != nil {
		t.Fatal(err)
	}
	return metadata
}

func TestAttachmentMigrationPartialSuccessAndResume(t *testing.T) {
	directory := t.TempDir()
	firstFilename, firstPath := createLocalMigrationMessage(t, directory, "a-first", multipartMessage())
	secondFilename, secondPath := createLocalMigrationMessage(t, directory, "b-second", multipartMessage())
	store := newMigrationFakeStore()
	secondKey := "b-second/" + secondFilename
	store.putFailures[secondKey] = -1

	summary, err := MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions())
	if err == nil || summary.Uploaded != 1 || summary.Failed != 1 {
		t.Fatalf("first migration summary = %#v, error = %v", summary, err)
	}
	if got := migrationMetadata(t, directory, "a-first").Attachments[0].Storage; got != attachmentStorageS3 {
		t.Fatalf("first attachment storage = %q", got)
	}
	if got := migrationMetadata(t, directory, "b-second").Attachments[0].Storage; got != attachmentStorageLocal {
		t.Fatalf("second attachment storage = %q", got)
	}
	for _, path := range []string{firstPath, secondPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("default migration removed %s: %v", path, err)
		}
	}

	store.putFailures[secondKey] = 0
	summary, err = MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	if summary.AlreadyMigrated != 1 || summary.Uploaded != 1 || summary.Failed != 0 {
		t.Fatalf("resumed migration summary = %#v", summary)
	}
	if calls := store.putCalls["a-first/"+firstFilename]; calls != 1 {
		t.Fatalf("already migrated attachment upload calls = %d, want 1", calls)
	}
}

func TestAttachmentMigrationRetriesTimeout(t *testing.T) {
	directory := t.TempDir()
	filename, path := createLocalMigrationMessage(t, directory, "timeout", multipartMessage())
	store := newMigrationFakeStore()
	store.blockPut = true
	options := migrationTestOptions()
	options.Retries = 1
	options.AttemptTimeout = 10 * time.Millisecond

	summary, err := MigrateLocalAttachments(context.Background(), directory, store, options)
	if err == nil || !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("migration error = %v", err)
	}
	if summary.RetryAttempts != 1 || summary.Failed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if calls := store.putCalls["timeout/"+filename]; calls != 2 {
		t.Fatalf("upload calls = %d, want 2", calls)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("timed-out migration removed source: %v", err)
	}
}

func TestAttachmentMigrationRepeatedRunIsIdempotent(t *testing.T) {
	directory := t.TempDir()
	filename, path := createLocalMigrationMessage(t, directory, "repeat", multipartMessage())
	store := newMigrationFakeStore()

	if _, err := MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions()); err != nil {
		t.Fatal(err)
	}
	summary, err := MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	if summary.AlreadyMigrated != 1 || summary.Uploaded != 0 || summary.Verified != 1 {
		t.Fatalf("second summary = %#v", summary)
	}
	if calls := store.putCalls["repeat/"+filename]; calls != 1 {
		t.Fatalf("upload calls = %d, want 1", calls)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("default migration removed source: %v", err)
	}
}

func TestAttachmentMigrationUpgradesVersionTwoMetadata(t *testing.T) {
	directory := t.TempDir()
	filename, _ := createLocalMigrationMessage(t, directory, "legacy-v2", multipartMessage())
	metadata := migrationMetadata(t, directory, "legacy-v2")
	metadata.Version = 2
	metadata.Attachments[0].ContentSHA256 = ""
	metadata.Attachments[0].Storage = ""
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, metadataDirectoryName, "legacy-v2.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	store := newMigrationFakeStore()

	if _, err := MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions()); err != nil {
		t.Fatal(err)
	}
	upgraded := migrationMetadata(t, directory, "legacy-v2")
	if upgraded.Version != currentMetadataVersion || upgraded.Attachments[0].ContentSHA256 == "" || upgraded.Attachments[0].Storage != attachmentStorageS3 {
		t.Fatalf("upgraded metadata = %#v", upgraded)
	}
	if calls := store.putCalls["legacy-v2/"+filename]; calls != 1 {
		t.Fatalf("upload calls = %d, want 1", calls)
	}
}

func TestAttachmentMigrationRestoresVersionOneMetadata(t *testing.T) {
	directory := t.TempDir()
	filename, _ := createLocalMigrationMessage(t, directory, "legacy-v1", multipartMessage())
	metadata := migrationMetadata(t, directory, "legacy-v1")
	metadata.Version = 1
	metadata.Attachments = nil
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, metadataDirectoryName, "legacy-v1.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	store := newMigrationFakeStore()

	if _, err := MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions()); err != nil {
		t.Fatal(err)
	}
	upgraded := migrationMetadata(t, directory, "legacy-v1")
	if upgraded.Version != currentMetadataVersion || len(upgraded.Attachments) != 1 {
		t.Fatalf("upgraded metadata = %#v", upgraded)
	}
	attachment := upgraded.Attachments[0]
	if attachment.GeneratedFileName != filename || attachment.ContentSHA256 == "" || attachment.Storage != attachmentStorageS3 {
		t.Fatalf("upgraded attachment metadata = %#v", attachment)
	}
}

func TestAttachmentMigrationRecognizesVersionTwoRemoteObject(t *testing.T) {
	directory := t.TempDir()
	filename, path := createLocalMigrationMessage(t, directory, "legacy-remote", multipartMessage())
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	metadata := migrationMetadata(t, directory, "legacy-remote")
	metadata.Version = 2
	metadata.Attachments[0].ContentSHA256 = ""
	metadata.Attachments[0].Storage = ""
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, metadataDirectoryName, "legacy-remote.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	store := newMigrationFakeStore()
	store.objects["legacy-remote/"+filename] = data

	summary, err := MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	if summary.AlreadyMigrated != 1 || summary.Uploaded != 0 || summary.Verified != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	upgraded := migrationMetadata(t, directory, "legacy-remote")
	if upgraded.Version != currentMetadataVersion || upgraded.Attachments[0].ContentSHA256 == "" || upgraded.Attachments[0].Storage != attachmentStorageS3 {
		t.Fatalf("upgraded metadata = %#v", upgraded)
	}
}

func TestAttachmentMigrationDryRunDoesNotWrite(t *testing.T) {
	directory := t.TempDir()
	filename, path := createLocalMigrationMessage(t, directory, "dry-run", multipartMessage())
	store := newMigrationFakeStore()
	before, err := os.ReadFile(filepath.Join(directory, metadataDirectoryName, "dry-run.json"))
	if err != nil {
		t.Fatal(err)
	}
	options := migrationTestOptions()
	options.DryRun = true
	options.DeleteLocal = true

	summary, err := MigrateLocalAttachments(context.Background(), directory, store, options)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Planned != 1 || summary.Uploaded != 0 || len(store.objects) != 0 {
		t.Fatalf("dry-run summary = %#v, objects = %#v", summary, store.objects)
	}
	after, err := os.ReadFile(filepath.Join(directory, metadataDirectoryName, "dry-run.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("dry-run changed metadata")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("dry-run removed local attachment %s: %v", filename, err)
	}
}

func TestAttachmentMigrationRejectsChecksumMismatchBeforeUpload(t *testing.T) {
	directory := t.TempDir()
	_, path := createLocalMigrationMessage(t, directory, "checksum", multipartMessage())
	if err := os.WriteFile(path, []byte("evil"), 0644); err != nil {
		t.Fatal(err)
	}
	store := newMigrationFakeStore()

	_, err := MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions())
	if err == nil || !strings.Contains(err.Error(), "verify local attachment") {
		t.Fatalf("migration error = %v", err)
	}
	if len(store.putCalls) != 0 {
		t.Fatalf("preflight failure uploaded objects: %#v", store.putCalls)
	}
}

func TestAttachmentMigrationRejectsAmbiguousMetadataBeforeUpload(t *testing.T) {
	directory := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", directory, ServerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const id = "ambiguous"
	if err := server.storeIncomingEmail(id, bytes.NewReader(twoTextAttachmentMessage("same", "same")), nil); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	metadata := migrationMetadata(t, directory, id)
	metadata.Attachments[1].GeneratedFileName = metadata.Attachments[0].GeneratedFileName
	if err := persistMigrationMetadata(directory, metadata); err != nil {
		t.Fatal(err)
	}
	store := newMigrationFakeStore()

	_, err = MigrateLocalAttachments(context.Background(), directory, store, migrationTestOptions())
	if err == nil || !strings.Contains(err.Error(), "ambiguous attachment mapping") {
		t.Fatalf("migration error = %v", err)
	}
	if len(store.putCalls) != 0 {
		t.Fatalf("ambiguous preflight uploaded objects: %#v", store.putCalls)
	}
}

func TestAttachmentMigrationCrashRecoveryBoundaries(t *testing.T) {
	t.Run("before metadata commit reuploads safely", func(t *testing.T) {
		directory := t.TempDir()
		filename, path := createLocalMigrationMessage(t, directory, "before-metadata", multipartMessage())
		store := newMigrationFakeStore()
		options := migrationTestOptions()
		options.DeleteLocal = true
		options.afterRemoteVerified = func(_, _ string) error { return errors.New("simulated crash") }

		if _, err := MigrateLocalAttachments(context.Background(), directory, store, options); err == nil {
			t.Fatal("migration succeeded across simulated crash")
		}
		if got := migrationMetadata(t, directory, "before-metadata").Attachments[0].Storage; got != attachmentStorageLocal {
			t.Fatalf("storage = %q, want local", got)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("source removed before metadata commit: %v", err)
		}
		options.afterRemoteVerified = nil
		if _, err := MigrateLocalAttachments(context.Background(), directory, store, options); err != nil {
			t.Fatal(err)
		}
		if calls := store.putCalls["before-metadata/"+filename]; calls != 2 {
			t.Fatalf("upload calls = %d, want safe overwrite on retry", calls)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("source survived completed delete-local migration: %v", err)
		}
	})

	t.Run("after metadata commit verifies then deletes", func(t *testing.T) {
		directory := t.TempDir()
		filename, path := createLocalMigrationMessage(t, directory, "after-metadata", multipartMessage())
		store := newMigrationFakeStore()
		options := migrationTestOptions()
		options.DeleteLocal = true
		options.afterMetadataCommit = func(_, _ string) error { return errors.New("simulated crash") }

		if _, err := MigrateLocalAttachments(context.Background(), directory, store, options); err == nil {
			t.Fatal("migration succeeded across simulated crash")
		}
		if got := migrationMetadata(t, directory, "after-metadata").Attachments[0].Storage; got != attachmentStorageS3 {
			t.Fatalf("storage = %q, want s3", got)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("source removed before post-commit hook: %v", err)
		}
		options.afterMetadataCommit = nil
		summary, err := MigrateLocalAttachments(context.Background(), directory, store, options)
		if err != nil {
			t.Fatal(err)
		}
		if summary.AlreadyMigrated != 1 || summary.LocalFilesDeleted != 1 {
			t.Fatalf("recovery summary = %#v", summary)
		}
		if calls := store.putCalls["after-metadata/"+filename]; calls != 1 {
			t.Fatalf("upload calls = %d, want remote verification without overwrite", calls)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("source survived recovered delete-local migration: %v", err)
		}
	})
}
