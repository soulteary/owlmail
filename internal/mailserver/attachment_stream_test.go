package mailserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/soulteary/owlmail/internal/attachmentstore"
)

type repeatedByteReader struct {
	remaining int64
	value     byte
	maxRead   int
	err       error
}

func (reader *repeatedByteReader) Read(buffer []byte) (int, error) {
	if reader.remaining == 0 {
		if reader.err != nil {
			err := reader.err
			reader.err = nil
			return 0, err
		}
		return 0, io.EOF
	}
	if len(buffer) > reader.maxRead {
		reader.maxRead = len(buffer)
	}
	count := int64(len(buffer))
	if count > reader.remaining {
		count = reader.remaining
	}
	for index := int64(0); index < count; index++ {
		buffer[index] = reader.value
	}
	reader.remaining -= count
	return int(count), nil
}

func streamingAttachmentMessage(size int64, value byte, filename string) io.Reader {
	header := "From: from@example.com\r\n" +
		"To: to@example.com\r\n" +
		"Subject: streaming attachment\r\n" +
		"Content-Type: multipart/mixed; boundary=stream-boundary\r\n\r\n" +
		"--stream-boundary\r\nContent-Type: text/plain\r\n\r\nbody\r\n" +
		"--stream-boundary\r\nContent-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=\"" + filename + "\"\r\n\r\n"
	footer := "\r\n--stream-boundary--\r\n"
	return io.MultiReader(
		strings.NewReader(header),
		&repeatedByteReader{remaining: size, value: value},
		strings.NewReader(footer),
	)
}

func repeatedByteSHA256(size int64, value byte) string {
	digest := sha256.New()
	chunk := bytes.Repeat([]byte{value}, attachmentCopyBufferSize)
	for remaining := size; remaining > 0; {
		count := int64(len(chunk))
		if count > remaining {
			count = remaining
		}
		_, _ = digest.Write(chunk[:count])
		remaining -= count
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func TestStoreIncomingEmailStreamsLargeAttachment(t *testing.T) {
	const attachmentSize = int64(8 << 20)
	const attachmentByte = byte('L')
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "large-stream"
	if err := server.storeIncomingEmail(id, streamingAttachmentMessage(attachmentSize, attachmentByte, "large.bin"), nil); err != nil {
		t.Fatalf("storeIncomingEmail() error = %v", err)
	}
	email, err := server.GetEmail(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(email.Attachments) != 1 {
		t.Fatalf("attachments = %d, want 1", len(email.Attachments))
	}
	attachment := email.Attachments[0]
	if attachment.Size != attachmentSize {
		t.Fatalf("attachment size = %d, want %d", attachment.Size, attachmentSize)
	}
	wantDigest := repeatedByteSHA256(attachmentSize, attachmentByte)
	if attachment.ContentSHA256 != wantDigest {
		t.Fatalf("attachment digest = %q, want %q", attachment.ContentSHA256, wantDigest)
	}
	path := filepath.Join(dir, id, attachment.GeneratedFileName)
	if stat, err := os.Stat(path); err != nil || stat.Size() != attachmentSize {
		t.Fatalf("staged attachment stat = %#v, %v", stat, err)
	}
}

func TestStoreIncomingEmailStreamsMultipleAttachments(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	const id = "multi-stream"
	if err := server.storeIncomingEmail(id, bytes.NewReader(twoTextAttachmentMessage("first payload", "second payload")), nil); err != nil {
		t.Fatal(err)
	}
	email, err := server.GetEmail(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(email.Attachments) != 2 {
		t.Fatalf("attachments = %d, want 2", len(email.Attachments))
	}
	for index, want := range []string{"first payload", "second payload"} {
		attachment := email.Attachments[index]
		if attachment.Size != int64(len(want)) {
			t.Fatalf("attachment %d size = %d, want %d", index, attachment.Size, len(want))
		}
		if attachment.ContentSHA256 != repeatedByteStringSHA256(want) {
			t.Fatalf("attachment %d digest = %q", index, attachment.ContentSHA256)
		}
		content, err := os.ReadFile(filepath.Join(dir, id, attachment.GeneratedFileName))
		if err != nil || string(content) != want {
			t.Fatalf("attachment %d content = %q, %v", index, content, err)
		}
	}
}

func repeatedByteStringSHA256(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func TestStreamingAttachmentReadFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	readErr := errors.New("injected attachment read failure")
	attachment := &Attachment{FileName: "failed.bin", ContentType: "application/octet-stream"}
	reader := &repeatedByteReader{remaining: 64 << 10, value: 'R', err: readErr}
	err = server.saveAttachmentReaderInDirectory(filepath.Join(dir, "staging"), attachment, reader)
	if !errors.Is(err, readErr) {
		t.Fatalf("saveAttachmentReaderInDirectory() error = %v", err)
	}
	if attachment.Size != 0 || attachment.ContentSHA256 != "" {
		t.Fatalf("failed attachment exposed partial metadata: %#v", attachment)
	}
	assertDirectoryHasNoAttachmentFiles(t, filepath.Join(dir, "staging"))
}

type failAfterWriter struct {
	destination io.Writer
	remaining   int
	err         error
}

func (writer *failAfterWriter) Write(buffer []byte) (int, error) {
	if writer.remaining <= 0 {
		return 0, writer.err
	}
	if len(buffer) <= writer.remaining {
		written, err := writer.destination.Write(buffer)
		writer.remaining -= written
		return written, err
	}
	written, err := writer.destination.Write(buffer[:writer.remaining])
	writer.remaining -= written
	if err != nil {
		return written, err
	}
	return written, writer.err
}

func TestStreamingAttachmentDiskFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	diskErr := errors.New("injected attachment disk failure")
	server.wrapAttachmentWriter = func(destination io.Writer) io.Writer {
		return &failAfterWriter{destination: destination, remaining: 1024, err: diskErr}
	}
	err = server.storeIncomingEmail("disk-failure", streamingAttachmentMessage(64<<10, 'D', "disk.bin"), nil)
	if !errors.Is(err, diskErr) {
		t.Fatalf("storeIncomingEmail() error = %v", err)
	}
	if _, err := server.GetEmail("disk-failure"); err == nil {
		t.Fatal("disk-failed email became visible")
	}
	assertNoCommittedOrTemporaryArtifacts(t, dir)
}

func TestMalformedAttachmentTransferEncodingRollsBack(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	message := "From: from@example.com\r\nTo: to@example.com\r\n" +
		"Subject: broken attachment\r\n" +
		"Content-Type: multipart/mixed; boundary=broken\r\n\r\n" +
		"--broken\r\nContent-Type: application/octet-stream\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"Content-Disposition: attachment; filename=broken.bin\r\n\r\n" +
		"aGVsbG8=%%%\r\n--broken--\r\n"
	err = server.storeIncomingEmail("read-failure", strings.NewReader(message), nil)
	if err == nil {
		t.Fatal("malformed transfer encoding was silently truncated")
	}
	if _, err := server.GetEmail("read-failure"); err == nil {
		t.Fatal("partially decoded attachment became visible")
	}
	assertNoCommittedOrTemporaryArtifacts(t, dir)
}

func TestTextAndHTMLReadFailuresAreNotSilentlyTruncated(t *testing.T) {
	for _, mediaType := range []string{"text/plain", "text/html"} {
		t.Run(mediaType, func(t *testing.T) {
			server, err := NewMailServer(1025, "localhost", t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = server.Close() }()

			readErr := errors.New("injected body read failure")
			message := io.MultiReader(
				strings.NewReader("From: from@example.com\r\nTo: to@example.com\r\nContent-Type: "+mediaType+"\r\n\r\n"),
				&repeatedByteReader{remaining: 64 << 10, value: 'T', err: readErr},
			)
			_, _, err = server.parseEmailMessage("body-read-failure", message, nil, false, "")
			if !errors.Is(err, readErr) {
				t.Fatalf("parseEmailMessage() error = %v", err)
			}
		})
	}
}

func assertDirectoryHasNoAttachmentFiles(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("partial attachment files survived: %#v", entries)
	}
}

func TestStreamingAttachmentUsesBoundedReadBuffer(t *testing.T) {
	const size = int64(8 << 20)
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	reader := &repeatedByteReader{remaining: size, value: 'M'}
	attachment := &Attachment{FileName: "bounded.bin", ContentType: "application/octet-stream"}
	if err := server.saveAttachmentReaderInDirectory(filepath.Join(dir, "staging"), attachment, reader); err != nil {
		t.Fatal(err)
	}
	if reader.maxRead > attachmentCopyBufferSize {
		t.Fatalf("largest read buffer = %d, want <= %d", reader.maxRead, attachmentCopyBufferSize)
	}
	if attachment.Size != size {
		t.Fatalf("attachment size = %d, want %d", attachment.Size, size)
	}
}

type blockingWriter struct {
	destination io.Writer
	started     chan struct{}
	release     chan struct{}
	once        sync.Once
}

func (writer *blockingWriter) Write(buffer []byte) (int, error) {
	writer.once.Do(func() { close(writer.started) })
	<-writer.release
	return writer.destination.Write(buffer)
}

func TestStreamingAttachmentRemainsInvisibleUntilTransactionCommit(t *testing.T) {
	dir := t.TempDir()
	server, err := NewMailServer(1025, "localhost", dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	started := make(chan struct{})
	release := make(chan struct{})
	server.wrapAttachmentWriter = func(destination io.Writer) io.Writer {
		return &blockingWriter{destination: destination, started: started, release: release}
	}
	const id = "invisible-stream"
	result := make(chan error, 1)
	go func() {
		result <- server.storeIncomingEmail(id, streamingAttachmentMessage(1<<20, 'I', "invisible.bin"), nil)
	}()
	<-started

	if _, err := server.GetEmail(id); err == nil {
		t.Fatal("email became API-visible while its attachment was still streaming")
	}
	if _, err := os.Stat(filepath.Join(dir, id)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final attachment directory became visible before commit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".eml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final EML became visible before commit: %v", err)
	}

	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if _, err := server.GetEmail(id); err != nil {
		t.Fatalf("committed email is not visible: %v", err)
	}
}

type failSecondPutStore struct {
	*memoryAttachmentStore
	calls int
	err   error
}

func (store *failSecondPutStore) CheckHealth(context.Context) error { return nil }

func (store *failSecondPutStore) Put(ctx context.Context, emailID, filename, contentType string, body io.Reader, size int64) error {
	store.calls++
	if store.calls == 2 {
		return store.err
	}
	return store.memoryAttachmentStore.Put(ctx, emailID, filename, contentType, body, size)
}

func TestStreamingMultipleAttachmentsS3FailureRollsBackUploadedObjects(t *testing.T) {
	remote := &failSecondPutStore{
		memoryAttachmentStore: newMemoryAttachmentStore(),
		err:                   errors.New("injected second S3 upload failure"),
	}
	dir := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	err = server.storeIncomingEmail("s3-stream-fail", bytes.NewReader(twoTextAttachmentMessage("first", "second")), nil)
	if !errors.Is(err, remote.err) {
		t.Fatalf("storeIncomingEmail() error = %v", err)
	}
	if len(remote.objects) != 0 {
		t.Fatalf("uploaded objects survived rollback: %#v", remote.objects)
	}
	if _, err := server.GetEmail("s3-stream-fail"); err == nil {
		t.Fatal("S3-failed email became visible")
	}
	assertNoCommittedOrTemporaryArtifacts(t, dir)
}

func TestStreamingAttachmentRollbackFailureRecoversFromFence(t *testing.T) {
	remote := newMemoryAttachmentStore()
	dir := t.TempDir()
	server, err := NewMailServerWithOptions(1025, "localhost", dir, ServerOptions{AttachmentStore: remote})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()

	commitErr := errors.New("injected store commit failure")
	remote.deleteErr = errors.New("injected rollback cleanup failure")
	server.beforeStoreCommit = func(*Email) error { return commitErr }
	err = server.storeIncomingEmail("rollback-failure", bytes.NewReader(multipartMessage()), nil)
	if !errors.Is(err, commitErr) {
		t.Fatalf("storeIncomingEmail() error = %v", err)
	}
	if len(remote.objects) != 1 {
		t.Fatalf("fault injection did not retain remote retry evidence: %#v", remote.objects)
	}
	if state, err := readRollbackFenceState(rollbackFencePath(dir, "rollback-failure")); err != nil || state != rollbackFenceState {
		t.Fatalf("rollback fence = %q, %v", state, err)
	}
	if _, err := server.GetEmail("rollback-failure"); err == nil {
		t.Fatal("rollback-failed email became visible")
	}

	remote.deleteErr = nil
	if err := server.recoverStorageArtifacts(); err != nil {
		t.Fatalf("recoverStorageArtifacts() error = %v", err)
	}
	if len(remote.objects) != 0 {
		t.Fatalf("recovery retained remote objects: %#v", remote.objects)
	}
}

var _ attachmentstore.Store = (*failSecondPutStore)(nil)

func BenchmarkSaveAttachmentStreaming(b *testing.B) {
	for _, size := range []int64{1 << 20, 32 << 20} {
		b.Run(fmt.Sprintf("%dMiB", size>>20), func(b *testing.B) {
			dir := b.TempDir()
			server, err := NewMailServer(1025, "localhost", dir)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = server.Close() }()
			b.SetBytes(size)
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				attachment := &Attachment{FileName: fmt.Sprintf("bench-%d.bin", index), ContentType: "application/octet-stream"}
				reader := &repeatedByteReader{remaining: size, value: 'B'}
				if err := server.saveAttachmentReaderInDirectory(dir, attachment, reader); err != nil {
					b.Fatal(err)
				}
				if err := os.Remove(filepath.Join(dir, attachment.GeneratedFileName)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
