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

type memoryAttachmentStore struct {
	objects   map[string][]byte
	putErr    error
	openErr   error
	deleteErr error
}

func newMemoryAttachmentStore() *memoryAttachmentStore {
	return &memoryAttachmentStore{objects: make(map[string][]byte)}
}

func (store *memoryAttachmentStore) Put(_ context.Context, emailID, filename, _ string, body io.Reader, _ int64) error {
	if store.putErr != nil {
		return store.putErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	store.objects[emailID+"/"+filename] = data
	return nil
}

func (store *memoryAttachmentStore) Open(_ context.Context, emailID, filename string) (*attachmentstore.Object, error) {
	if store.openErr != nil {
		return nil, store.openErr
	}
	data, ok := store.objects[emailID+"/"+filename]
	if !ok {
		return nil, errors.New("object not found")
	}
	return &attachmentstore.Object{
		Body: io.NopCloser(bytes.NewReader(data)),
		Size: int64(len(data)),
	}, nil
}

func (store *memoryAttachmentStore) DeleteEmail(_ context.Context, emailID string) error {
	if store.deleteErr != nil {
		return store.deleteErr
	}
	prefix := emailID + "/"
	for key := range store.objects {
		if strings.HasPrefix(key, prefix) {
			delete(store.objects, key)
		}
	}
	return nil
}

func TestRemoteAttachmentLifecycleSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	remote := newMemoryAttachmentStore()
	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}

	if err := server.storeIncomingEmail("remote-message", bytes.NewReader(multipartMessage()), nil); err != nil {
		t.Fatalf("storeIncomingEmail() error = %v", err)
	}
	email, err := server.GetEmail("remote-message")
	if err != nil || len(email.Attachments) != 1 {
		t.Fatalf("stored email = %#v, %v", email, err)
	}
	filename := email.Attachments[0].GeneratedFileName
	if filename == "" {
		t.Fatal("attachment generated filename is empty")
	}
	if got := string(remote.objects["remote-message/"+filename]); got != "data" {
		t.Fatalf("remote attachment = %q, want data", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "remote-message")); !os.IsNotExist(err) {
		t.Fatalf("remote attachment was also committed locally: %v", err)
	}

	opened, err := server.OpenEmailAttachment("remote-message", filename)
	if err != nil {
		t.Fatalf("OpenEmailAttachment() error = %v", err)
	}
	data, err := io.ReadAll(opened.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	_ = opened.Body.Close()
	if string(data) != "data" || opened.Size != 4 {
		t.Fatalf("opened attachment = %q (%d bytes)", data, opened.Size)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	restarted, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatalf("restart error = %v", err)
	}
	defer func() { _ = restarted.Close() }()
	restored, err := restarted.GetEmail("remote-message")
	if err != nil || len(restored.Attachments) != 1 {
		t.Fatalf("restored email = %#v, %v", restored, err)
	}
	if restored.Attachments[0].GeneratedFileName != filename {
		t.Fatalf("restored filename = %q, want %q", restored.Attachments[0].GeneratedFileName, filename)
	}

	if err := restarted.DeleteEmail("remote-message"); err != nil {
		t.Fatalf("DeleteEmail() error = %v", err)
	}
	if _, ok := remote.objects["remote-message/"+filename]; ok {
		t.Fatal("remote attachment survived email deletion")
	}
}

func TestRemoteAttachmentUploadFailureRollsBackMessage(t *testing.T) {
	dir := t.TempDir()
	remote := newMemoryAttachmentStore()
	remote.putErr = errors.New("injected remote upload failure")
	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	err = server.storeIncomingEmail("failed-remote", bytes.NewReader(multipartMessage()), nil)
	if err == nil || !strings.Contains(err.Error(), "injected remote upload failure") {
		t.Fatalf("storeIncomingEmail() error = %v", err)
	}
	if _, err := server.GetEmail("failed-remote"); err == nil {
		t.Fatal("failed message became visible")
	}
	if _, err := os.Stat(filepath.Join(dir, "failed-remote.eml")); !os.IsNotExist(err) {
		t.Fatalf("failed message EML survived: %v", err)
	}
	if len(remote.objects) != 0 {
		t.Fatalf("remote objects survived rollback: %#v", remote.objects)
	}
}

func TestS3ModeKeepsExistingLocalAttachmentsReadable(t *testing.T) {
	dir := t.TempDir()
	remote := newMemoryAttachmentStore()
	remote.openErr = errors.New("remote should not be opened")
	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	id := "legacy-local"
	filename := "legacy.txt"
	attachmentDirectory := filepath.Join(dir, id)
	if err := os.MkdirAll(attachmentDirectory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDirectory, filename), []byte("legacy"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".eml"), []byte("message"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := server.SaveEmailToStore(id, false, &Envelope{}, &Email{
		Attachments: []*Attachment{{GeneratedFileName: filename, ContentType: "text/plain", Size: 6}},
	}); err != nil {
		t.Fatal(err)
	}

	opened, err := server.OpenEmailAttachment(id, filename)
	if err != nil {
		t.Fatalf("OpenEmailAttachment() error = %v", err)
	}
	content, err := io.ReadAll(opened.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = opened.Body.Close()
	if string(content) != "legacy" {
		t.Fatalf("local fallback content = %q", content)
	}
	remote.openErr = nil
	if err := server.DeleteEmail(id); err != nil {
		t.Fatalf("DeleteEmail() error = %v", err)
	}
	if _, err := os.Stat(attachmentDirectory); !os.IsNotExist(err) {
		t.Fatalf("local fallback directory survived deletion: %v", err)
	}
}

func TestS3ModeDeleteAllCleansRemoteAttachments(t *testing.T) {
	dir := t.TempDir()
	remote := newMemoryAttachmentStore()
	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	for _, id := range []string{"remote-one", "remote-two"} {
		if err := server.storeIncomingEmail(id, bytes.NewReader(multipartMessage()), nil); err != nil {
			t.Fatalf("storeIncomingEmail(%q) error = %v", id, err)
		}
	}
	if len(remote.objects) != 2 {
		t.Fatalf("remote object count = %d, want 2", len(remote.objects))
	}
	if err := server.DeleteAllEmail(); err != nil {
		t.Fatalf("DeleteAllEmail() error = %v", err)
	}
	if len(remote.objects) != 0 {
		t.Fatalf("remote objects survived clear-all: %#v", remote.objects)
	}
	if got := len(server.GetAllEmail()); got != 0 {
		t.Fatalf("stored email count = %d, want 0", got)
	}
}

func TestS3ModeRetentionCleansRemoteAttachments(t *testing.T) {
	dir := t.TempDir()
	remote := newMemoryAttachmentStore()
	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	if err := server.storeIncomingEmail("remote-oldest", bytes.NewReader(multipartMessage()), nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := server.storeIncomingEmail("remote-newest", bytes.NewReader(multipartMessage()), nil); err != nil {
		t.Fatal(err)
	}
	if err := server.ConfigureStoragePolicy(StoragePolicy{MaxMessages: 1, CleanupInterval: time.Hour}); err != nil {
		t.Fatalf("ConfigureStoragePolicy() error = %v", err)
	}
	if _, err := server.GetEmail("remote-oldest"); err == nil {
		t.Fatal("oldest remote-backed email was not evicted")
	}
	if _, err := server.GetEmail("remote-newest"); err != nil {
		t.Fatalf("newest remote-backed email was evicted: %v", err)
	}
	for key := range remote.objects {
		if strings.HasPrefix(key, "remote-oldest/") {
			t.Fatalf("evicted email object survived retention cleanup: %s", key)
		}
	}
}

func TestS3ModeRestoresLegacyLocalAttachmentMetadata(t *testing.T) {
	dir := t.TempDir()
	id := "legacy-restart"
	filename := "old-generated.txt"
	if err := os.WriteFile(filepath.Join(dir, id+".eml"), multipartMessage(), 0644); err != nil {
		t.Fatal(err)
	}
	attachmentDirectory := filepath.Join(dir, id)
	if err := os.MkdirAll(attachmentDirectory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDirectory, filename), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	metadataDirectory := filepath.Join(dir, metadataDirectoryName)
	if err := os.MkdirAll(metadataDirectory, 0750); err != nil {
		t.Fatal(err)
	}
	legacyMetadata, err := json.Marshal(emailMetadata{
		Version:  1,
		ID:       id,
		Sequence: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadataDirectory, id+".json"), legacyMetadata, 0600); err != nil {
		t.Fatal(err)
	}

	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: newMemoryAttachmentStore()})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}
	defer func() { _ = server.Close() }()
	email, err := server.GetEmail(id)
	if err != nil || len(email.Attachments) != 1 {
		t.Fatalf("restored email = %#v, %v", email, err)
	}
	if email.Attachments[0].GeneratedFileName != filename {
		t.Fatalf("restored filename = %q, want %q", email.Attachments[0].GeneratedFileName, filename)
	}

	upgraded, err := server.loadEmailMetadata(id)
	if err != nil {
		t.Fatalf("load upgraded metadata: %v", err)
	}
	if upgraded.Version != currentMetadataVersion || len(upgraded.Attachments) != 1 || upgraded.Attachments[0].GeneratedFileName != filename {
		t.Fatalf("upgraded metadata = %#v", upgraded)
	}
}

func TestS3ModeQuarantinesLegacyLocalAttachmentsAndCleansRemoteObjects(t *testing.T) {
	dir := t.TempDir()
	remote := newMemoryAttachmentStore()
	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}
	defer func() { _ = server.Close() }()

	id := "legacy-corrupt"
	emlPath := filepath.Join(dir, id+".eml")
	if err := os.WriteFile(emlPath, []byte("corrupt message"), 0600); err != nil {
		t.Fatal(err)
	}
	attachmentDirectory := filepath.Join(dir, id)
	if err := os.MkdirAll(attachmentDirectory, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(attachmentDirectory, "evidence.txt"), []byte("evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	remote.objects[id+"/remote.txt"] = []byte("remote")

	if err := server.quarantineEmail(id, emlPath, "corrupt"); err != nil {
		t.Fatalf("quarantineEmail() error = %v", err)
	}
	if _, ok := remote.objects[id+"/remote.txt"]; ok {
		t.Fatal("remote object survived quarantine cleanup")
	}
	if _, err := os.Stat(attachmentDirectory); !os.IsNotExist(err) {
		t.Fatalf("legacy attachment directory remained live: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(dir, quarantineDirName))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("quarantine entries = %d, want 1", len(entries))
	}
	quarantined := filepath.Join(dir, quarantineDirName, entries[0].Name())
	if content, err := os.ReadFile(filepath.Join(quarantined, "message.eml")); err != nil || string(content) != "corrupt message" {
		t.Fatalf("quarantined EML = %q, %v", content, err)
	}
	if content, err := os.ReadFile(filepath.Join(quarantined, "attachments", "evidence.txt")); err != nil || string(content) != "evidence" {
		t.Fatalf("quarantined attachment = %q, %v", content, err)
	}
}
