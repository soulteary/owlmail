// Package attachmentstore provides optional external storage for decoded email
// attachments. Raw messages and OwlMail metadata remain in the configured mail
// directory regardless of the attachment backend.
package attachmentstore

import (
	"context"
	"io"
)

// Object is an attachment opened for streaming.
type Object struct {
	Body io.ReadCloser
	Size int64
}

// Store persists decoded attachments for one email ID.
type Store interface {
	Put(ctx context.Context, emailID, filename, contentType string, body io.Reader, size int64) error
	Open(ctx context.Context, emailID, filename string) (*Object, error)
	DeleteEmail(ctx context.Context, emailID string) error
	CheckHealth(ctx context.Context) error
}
