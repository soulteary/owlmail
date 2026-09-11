package mailserver

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// assertPermissionsWithin fails when path grants any access the storage layer
// does not intend. The bound is used instead of equality wherever the mode
// comes from a create call, because the umask of the host running the test can
// only clear bits: a stricter host is not a regression, a wider mode is the
// exposure being guarded.
//
// The bound is therefore only as strong as the host umask is weak. Under a
// umask of 0027 or stricter the group bits are already cleared for every
// caller, so a regression that widened one of these create calls back to 0755
// would still pass here. The chmod-pinned artifacts are the ones checked
// exactly, and those hold on any host.
func assertPermissionsWithin(t *testing.T, path string, allowed os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got&^allowed != 0 {
		t.Fatalf("%s mode = %04o, want no access beyond %04o", path, got, allowed)
	}
}

// assertPermissionsExactly is used for artifacts whose mode is pinned by an
// explicit chmod, which the umask cannot narrow.
func assertPermissionsExactly(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %04o, want %04o", path, got, want)
	}
}

func TestCommittedMailArtifactPermissions(t *testing.T) {
	mailDir := filepath.Join(t.TempDir(), "maildir")
	server, err := NewMailServer(1025, "localhost", mailDir)
	if err != nil {
		t.Fatalf("NewMailServer() error = %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	const id = "permission-message"
	if err := server.storeIncomingEmail(id, bytes.NewReader(multipartMessage()), nil); err != nil {
		t.Fatalf("storeIncomingEmail() error = %v", err)
	}
	email, err := server.GetEmail(id)
	if err != nil {
		t.Fatalf("GetEmail() error = %v", err)
	}
	if len(email.Attachments) != 1 {
		t.Fatalf("stored attachments = %d, want 1", len(email.Attachments))
	}

	tests := []struct {
		name   string
		path   string
		mode   os.FileMode
		pinned bool
	}{
		{name: "mail directory", path: mailDir, mode: 0750},
		{name: "message body", path: filepath.Join(mailDir, id+".eml"), mode: 0600, pinned: true},
		// The per-message attachment directory is stricter than the mail
		// directory that holds it, which is intended: only OwlMail's own
		// account ever traverses it.
		{name: "attachment directory", path: filepath.Join(mailDir, id), mode: 0700, pinned: true},
		{
			name:   "attachment file",
			path:   filepath.Join(mailDir, id, email.Attachments[0].GeneratedFileName),
			mode:   0600,
			pinned: true,
		},
		{
			name:   "metadata sidecar",
			path:   filepath.Join(mailDir, ".owlmail-meta", id+".json"),
			mode:   0600,
			pinned: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.pinned {
				assertPermissionsExactly(t, test.path, test.mode)
				return
			}
			assertPermissionsWithin(t, test.path, test.mode)
		})
	}
}

// TestSaveAttachmentPermissions covers saveAttachment, which has no non-test
// caller: the live capture path is storeIncomingEmail, which stages into its
// own directory and promotes it. The branch is covered because it is exported
// to the rest of the package and creates the attachment directory itself, so a
// future caller must not reintroduce a wider mode.
func TestSaveAttachmentPermissions(t *testing.T) {
	mailDir := filepath.Join(t.TempDir(), "maildir")
	server, err := NewMailServer(1025, "localhost", mailDir)
	if err != nil {
		t.Fatalf("NewMailServer() error = %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	attachment := &Attachment{FileName: "report.pdf", ContentType: "application/pdf"}
	if err := server.saveAttachment("direct-message", attachment, []byte("payload")); err != nil {
		t.Fatalf("saveAttachment() error = %v", err)
	}

	attachmentDir := filepath.Join(mailDir, "direct-message")
	assertPermissionsWithin(t, attachmentDir, 0700)
	assertPermissionsExactly(t, filepath.Join(attachmentDir, attachment.GeneratedFileName), 0600)
}

// stagingPermissionStore records the mode of each attachment as the upload
// reads it, which is the only point at which an S3-bound attachment exists on
// local disk. It follows the same embed-and-override shape as
// failSecondPutStore in attachment_stream_test.go; the two cannot share one
// type because they intercept Put for opposite reasons.
type stagingPermissionStore struct {
	*memoryAttachmentStore
	modes map[string]os.FileMode
}

func (store *stagingPermissionStore) Put(ctx context.Context, emailID, filename, contentType string, body io.Reader, size int64) error {
	if file, ok := body.(interface{ Stat() (os.FileInfo, error) }); ok {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		store.modes[filename] = info.Mode().Perm()
	}
	return store.memoryAttachmentStore.Put(ctx, emailID, filename, contentType, body, size)
}

func TestRemoteAttachmentStagingPermissions(t *testing.T) {
	mailDir := filepath.Join(t.TempDir(), "maildir")
	remote := &stagingPermissionStore{
		memoryAttachmentStore: newMemoryAttachmentStore(),
		modes:                 make(map[string]os.FileMode),
	}
	server, err := NewMailServerWithOptions(1025, "localhost", mailDir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatalf("NewMailServerWithOptions() error = %v", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	// The staging directory is removed once the upload succeeds, so its mode
	// has to be read while the attachment is still being written into it.
	var stagingModes []os.FileMode
	server.beforeAttachmentWrite = func(path string) error {
		info, err := os.Stat(filepath.Dir(path))
		if err != nil {
			return err
		}
		stagingModes = append(stagingModes, info.Mode().Perm())
		return nil
	}

	const id = "remote-permission-message"
	if err := server.storeIncomingEmail(id, bytes.NewReader(multipartMessage()), nil); err != nil {
		t.Fatalf("storeIncomingEmail() error = %v", err)
	}
	email, err := server.GetEmail(id)
	if err != nil {
		t.Fatalf("GetEmail() error = %v", err)
	}
	if len(email.Attachments) != 1 {
		t.Fatalf("stored attachments = %d, want 1", len(email.Attachments))
	}
	if len(stagingModes) != 1 {
		t.Fatalf("staged attachment writes = %d, want 1", len(stagingModes))
	}
	if stagingModes[0] != 0700 {
		t.Fatalf("staging directory mode = %04o, want 0700", stagingModes[0])
	}

	mode, recorded := remote.modes[email.Attachments[0].GeneratedFileName]
	if !recorded {
		t.Fatalf("upload did not read the staged attachment from disk")
	}
	if mode != 0600 {
		t.Fatalf("staged attachment mode = %04o, want 0600", mode)
	}
}
