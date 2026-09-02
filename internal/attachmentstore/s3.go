package attachmentstore

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config configures an Amazon S3 or S3-compatible attachment backend.
type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	UsePathStyle    bool
}

type s3Client interface {
	HeadBucket(context.Context, *s3.HeadBucketInput, ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// CheckHealth verifies that the configured credentials can address the bucket.
// HeadBucket is preferred. A bounded prefix-scoped ListObjectsV2 fallback
// supports least-privilege policies that deny bucket-wide HeadBucket while
// permitting OwlMail's actual attachment prefix.
func (store *S3Store) CheckHealth(ctx context.Context) error {
	_, err := store.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(store.bucket)})
	if err == nil {
		return nil
	}
	prefix := store.prefix
	if prefix != "" {
		prefix += "/"
	}
	_, fallbackErr := store.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(store.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if fallbackErr != nil {
		return fmt.Errorf("check S3 attachment prefix: %w", fallbackErr)
	}
	return nil
}

// S3Store stores attachments as <prefix>/<email-id>/<generated-filename>.
type S3Store struct {
	client s3Client
	bucket string
	prefix string
}

// NewS3 creates an S3-compatible attachment store. The bucket must already
// exist. When static credentials are omitted, the AWS SDK default credential
// chain is used.
func NewS3(ctx context.Context, cfg S3Config) (*S3Store, error) {
	region := strings.TrimSpace(cfg.Region)
	bucket := strings.TrimSpace(cfg.Bucket)
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if region == "" {
		return nil, fmt.Errorf("S3 region cannot be empty")
	}
	if bucket == "" {
		return nil, fmt.Errorf("S3 bucket cannot be empty")
	}
	if (cfg.AccessKeyID == "") != (cfg.SecretAccessKey == "") {
		return nil, fmt.Errorf("S3 access key and secret key must be configured together")
	}
	if cfg.SessionToken != "" && cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("S3 session token requires static access key credentials")
	}
	if endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("S3 endpoint must be an HTTP or HTTPS URL without credentials, query, or fragment")
		}
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if cfg.AccessKeyID != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
	})
	return newS3Store(client, bucket, cfg.Prefix), nil
}

func newS3Store(client s3Client, bucket, prefix string) *S3Store {
	return &S3Store{
		client: client,
		bucket: strings.TrimSpace(bucket),
		prefix: strings.Trim(strings.TrimSpace(prefix), "/"),
	}
}

// Put uploads an attachment to S3.
func (store *S3Store) Put(ctx context.Context, emailID, filename, contentType string, body io.Reader, size int64) error {
	if err := validateObjectComponent("email ID", emailID); err != nil {
		return err
	}
	if err := validateObjectComponent("filename", filename); err != nil {
		return err
	}
	input := &s3.PutObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(store.objectKey(emailID, filename)),
		Body:   body,
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if _, err := store.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put S3 attachment %q: %w", filename, err)
	}
	return nil
}

// Open opens an attachment for streaming from S3.
func (store *S3Store) Open(ctx context.Context, emailID, filename string) (*Object, error) {
	if err := validateObjectComponent("email ID", emailID); err != nil {
		return nil, err
	}
	if err := validateObjectComponent("filename", filename); err != nil {
		return nil, err
	}
	output, err := store.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(store.objectKey(emailID, filename)),
	})
	if err != nil {
		return nil, fmt.Errorf("get S3 attachment %q: %w", filename, err)
	}
	if output.Body == nil {
		return nil, fmt.Errorf("get S3 attachment %q: response body is empty", filename)
	}
	size := int64(-1)
	if output.ContentLength != nil {
		size = *output.ContentLength
	}
	return &Object{Body: output.Body, Size: size}, nil
}

// DeleteEmail removes every attachment below one exact email prefix.
func (store *S3Store) DeleteEmail(ctx context.Context, emailID string) error {
	if err := validateObjectComponent("email ID", emailID); err != nil {
		return err
	}
	prefix := store.emailPrefix(emailID)
	paginator := s3.NewListObjectsV2Paginator(store.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(store.bucket),
		Prefix: aws.String(prefix),
	})
	var keys []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list S3 attachments for email %q: %w", emailID, err)
		}
		for _, object := range page.Contents {
			if object.Key == nil || !strings.HasPrefix(*object.Key, prefix) {
				continue
			}
			keys = append(keys, *object.Key)
		}
	}
	for _, key := range keys {
		if _, err := store.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(store.bucket),
			Key:    aws.String(key),
		}); err != nil {
			return fmt.Errorf("delete S3 attachment %q: %w", key, err)
		}
	}
	return nil
}

func (store *S3Store) objectKey(emailID, filename string) string {
	return store.emailPrefix(emailID) + filename
}

func (store *S3Store) emailPrefix(emailID string) string {
	if store.prefix == "" {
		return emailID + "/"
	}
	return store.prefix + "/" + emailID + "/"
}

func validateObjectComponent(name, value string) error {
	if value == "" || value == "." || value == ".." || strings.Contains(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, "\x00") {
		return fmt.Errorf("invalid S3 attachment %s", name)
	}
	return nil
}
