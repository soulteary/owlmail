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
}

// HealthChecker is implemented by stores that can verify their backing
// service without modifying it. Implementations must honor ctx deadlines.
type HealthChecker interface {
	CheckHealth(ctx context.Context) error
}

// HealthStatus is the cached, disclosure-safe state exposed to readiness
// handlers. Category never contains provider error text or configuration.
type HealthStatus struct {
	Ready    bool
	Category string
}

const (
	HealthOK               = "ok"
	HealthDisabled         = "disabled"
	HealthUnsupported      = "unsupported"
	HealthChecking         = "checking"
	HealthPermissionDenied = "permission_denied"
	HealthNotFound         = "not_found"
	HealthTimeout          = "timeout"
	HealthNetwork          = "network"
	HealthUnavailable      = "unavailable"
	HealthClosed           = "closed"
)

// ReadinessProvider exposes a cached health result. Calling Readiness must not
// perform network I/O.
type ReadinessProvider interface {
	Readiness() HealthStatus
}
