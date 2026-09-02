package attachmentstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type fakeS3Client struct {
	objects      map[string][]byte
	contentTypes map[string]string
	putErr       error
	getErr       error
	listErr      error
	deleteErr    error
	headMu       sync.Mutex
	headErr      error
	headBlock    bool
	headCalls    int
}

func (client *fakeS3Client) HeadBucket(ctx context.Context, _ *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	client.headMu.Lock()
	client.headCalls++
	err := client.headErr
	block := client.headBlock
	client.headMu.Unlock()
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return &s3.HeadBucketOutput{}, nil
}

func (client *fakeS3Client) setHeadResult(err error, block bool) {
	client.headMu.Lock()
	client.headErr = err
	client.headBlock = block
	client.headMu.Unlock()
}

func (client *fakeS3Client) headCallCount() int {
	client.headMu.Lock()
	defer client.headMu.Unlock()
	return client.headCalls
}

func newFakeS3Client() *fakeS3Client {
	return &fakeS3Client{
		objects:      make(map[string][]byte),
		contentTypes: make(map[string]string),
	}
}

func (client *fakeS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if client.putErr != nil {
		return nil, client.putErr
	}
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	key := aws.ToString(input.Key)
	client.objects[key] = data
	client.contentTypes[key] = aws.ToString(input.ContentType)
	return &s3.PutObjectOutput{}, nil
}

func (client *fakeS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if client.getErr != nil {
		return nil, client.getErr
	}
	data, ok := client.objects[aws.ToString(input.Key)]
	if !ok {
		return nil, errors.New("not found")
	}
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: aws.Int64(int64(len(data))),
	}, nil
}

func (client *fakeS3Client) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if client.listErr != nil {
		return nil, client.listErr
	}
	prefix := aws.ToString(input.Prefix)
	keys := make([]string, 0)
	for key := range client.objects {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	objects := make([]s3types.Object, 0, len(keys))
	for _, key := range keys {
		objects = append(objects, s3types.Object{Key: aws.String(key)})
	}
	return &s3.ListObjectsV2Output{
		Contents:    objects,
		IsTruncated: aws.Bool(false),
	}, nil
}

func (client *fakeS3Client) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if client.deleteErr != nil {
		return nil, client.deleteErr
	}
	delete(client.objects, aws.ToString(input.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func TestS3StorePutOpenAndDeleteEmail(t *testing.T) {
	client := newFakeS3Client()
	store := newS3Store(client, "mail-bucket", "/owlmail/attachments/")
	ctx := context.Background()

	content := []byte("attachment data")
	if err := store.Put(ctx, "mail-1", "document.pdf", "application/pdf", bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	key := "owlmail/attachments/mail-1/document.pdf"
	if got := string(client.objects[key]); got != string(content) {
		t.Fatalf("stored content = %q, want %q", got, content)
	}
	if got := client.contentTypes[key]; got != "application/pdf" {
		t.Fatalf("stored content type = %q", got)
	}

	object, err := store.Open(ctx, "mail-1", "document.pdf")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	opened, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if err := object.Body.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !bytes.Equal(opened, content) || object.Size != int64(len(content)) {
		t.Fatalf("opened object = %q (%d bytes)", opened, object.Size)
	}

	client.objects["owlmail/attachments/mail-1/image.png"] = []byte("image")
	client.objects["owlmail/attachments/mail-10/keep.txt"] = []byte("keep")
	if err := store.DeleteEmail(ctx, "mail-1"); err != nil {
		t.Fatalf("DeleteEmail() error = %v", err)
	}
	if _, ok := client.objects[key]; ok {
		t.Fatal("first email object survived deletion")
	}
	if _, ok := client.objects["owlmail/attachments/mail-1/image.png"]; ok {
		t.Fatal("second email object survived deletion")
	}
	if _, ok := client.objects["owlmail/attachments/mail-10/keep.txt"]; !ok {
		t.Fatal("neighboring email prefix was deleted")
	}
}

func TestS3StoreCheckHealth(t *testing.T) {
	client := newFakeS3Client()
	store := newS3Store(client, "mail-bucket", "attachments")
	if err := store.CheckHealth(context.Background()); err != nil {
		t.Fatalf("CheckHealth() error = %v", err)
	}
	client.setHeadResult(&smithy.GenericAPIError{Code: "AccessDenied", Message: "secret provider detail"}, false)
	status := CheckHealth(context.Background(), store)
	if status.Ready || status.Category != HealthPermissionDenied {
		t.Fatalf("permission status = %#v", status)
	}
	client.setHeadResult(&smithy.GenericAPIError{Code: "NoSuchBucket", Message: "missing"}, false)
	if status := CheckHealth(context.Background(), store); status.Ready || status.Category != HealthNotFound {
		t.Fatalf("missing bucket status = %#v", status)
	}
	client.setHeadResult(&net.DNSError{Err: "dial failed", Name: "objects.example.test"}, false)
	if status := CheckHealth(context.Background(), store); status.Ready || status.Category != HealthNetwork {
		t.Fatalf("network status = %#v", status)
	}
	if got := client.headCallCount(); got != 4 {
		t.Fatalf("HeadBucket calls = %d, want 4", got)
	}
}

func TestMonitoredS3StoreTimeoutRecoveryAndClose(t *testing.T) {
	client := newFakeS3Client()
	client.setHeadResult(nil, true)
	store := newS3Store(client, "mail-bucket", "attachments")
	monitored, err := NewMonitoredStore(store, 10*time.Millisecond, 15*time.Millisecond)
	if err != nil {
		t.Fatalf("NewMonitoredStore() error = %v", err)
	}
	waitForHealthStatus(t, monitored, HealthTimeout)

	client.setHeadResult(nil, false)
	waitForHealthStatus(t, monitored, HealthOK)
	if !monitored.Readiness().Ready {
		t.Fatalf("recovered readiness = %#v", monitored.Readiness())
	}

	if err := monitored.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if status := monitored.Readiness(); status.Ready || status.Category != HealthClosed {
		t.Fatalf("closed readiness = %#v", status)
	}
	calls := client.headCallCount()
	time.Sleep(30 * time.Millisecond)
	if got := client.headCallCount(); got != calls {
		t.Fatalf("HeadBucket called after Close: before=%d after=%d", calls, got)
	}
	if err := monitored.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func waitForHealthStatus(t *testing.T, provider ReadinessProvider, category string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if status := provider.Readiness(); status.Category == category {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("health category = %q, want %q", provider.Readiness().Category, category)
}

func TestS3StorePropagatesClientErrors(t *testing.T) {
	client := newFakeS3Client()
	store := newS3Store(client, "bucket", "attachments")
	ctx := context.Background()

	client.putErr = errors.New("put failed")
	if err := store.Put(ctx, "mail", "file.txt", "text/plain", bytes.NewReader(nil), 0); err == nil {
		t.Fatal("Put() succeeded with client error")
	}
	client.putErr = nil

	client.getErr = errors.New("get failed")
	if _, err := store.Open(ctx, "mail", "file.txt"); err == nil {
		t.Fatal("Open() succeeded with client error")
	}
	client.getErr = nil

	client.listErr = errors.New("list failed")
	if err := store.DeleteEmail(ctx, "mail"); err == nil {
		t.Fatal("DeleteEmail() succeeded with list error")
	}
	client.listErr = nil
	client.objects["attachments/mail/file.txt"] = []byte("data")
	client.deleteErr = errors.New("delete failed")
	if err := store.DeleteEmail(ctx, "mail"); err == nil {
		t.Fatal("DeleteEmail() succeeded with delete error")
	}
	if err := store.DeleteEmail(ctx, ""); err == nil {
		t.Fatal("DeleteEmail() accepted an empty email ID")
	}
}

func TestNewS3ValidatesRequiredConfiguration(t *testing.T) {
	ctx := context.Background()
	if _, err := NewS3(ctx, S3Config{Bucket: "bucket"}); err == nil {
		t.Fatal("NewS3() accepted an empty region")
	}
	if _, err := NewS3(ctx, S3Config{Region: "us-east-1"}); err == nil {
		t.Fatal("NewS3() accepted an empty bucket")
	}
	if _, err := NewS3(ctx, S3Config{Region: "us-east-1", Bucket: "bucket", AccessKeyID: "access"}); err == nil {
		t.Fatal("NewS3() accepted partial static credentials")
	}
	if _, err := NewS3(ctx, S3Config{Region: "us-east-1", Bucket: "bucket", Endpoint: "ftp://example.test"}); err == nil {
		t.Fatal("NewS3() accepted a non-HTTP endpoint")
	}
	if _, err := NewS3(ctx, S3Config{Region: "us-east-1", Bucket: "bucket", AccessKeyID: "access", SecretAccessKey: "secret"}); err != nil {
		t.Fatalf("NewS3() valid static configuration error = %v", err)
	}
}
