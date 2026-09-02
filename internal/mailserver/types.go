package mailserver

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

const (
	defaultPort    = 1025
	defaultHost    = "localhost"
	defaultMailDir = "owlmail"

	// DefaultMaxMessageBytes is the default SMTP DATA limit (100 MiB).
	DefaultMaxMessageBytes int64 = 100 << 20

	defaultAttachmentUploadTimeout = 5 * time.Minute
	defaultAttachmentOpenTimeout   = 5 * time.Minute
	defaultAttachmentDeleteTimeout = 30 * time.Second
)

// Email is an alias for types.Email
type Email = types.Email

// Attachment is an alias for types.Attachment
type Attachment = types.Attachment

// Envelope is an alias for types.Envelope
type Envelope = types.Envelope

// SMTPAuthConfig represents required SMTP authentication configuration. A nil
// or disabled config selects NO AUTH mode.
type SMTPAuthConfig struct {
	Username string
	Password string
	Enabled  bool
}

// TLSConfig represents TLS configuration for SMTP server
type TLSConfig struct {
	CertFile string
	KeyFile  string
	Enabled  bool
}

// ServerOptions contains optional runtime integrations and SMTP behavior.
// Zero MaxMessageBytes selects DefaultMaxMessageBytes. Zero
// MaxDataConcurrency leaves SMTP DATA concurrency unlimited.
type ServerOptions struct {
	OutgoingConfig     *outgoing.OutgoingConfig
	AuthConfig         *SMTPAuthConfig
	AuthRequireTLS     bool
	TLSConfig          *TLSConfig
	UseUUIDForID       bool
	MaxMessageBytes    int64
	MaxDataConcurrency int
	RetainAllHeaders   bool
	AttachmentStore    attachmentstore.Store
	AttachmentHealth   attachmentstore.ReadinessProvider
	MailboxIndex       MailboxIndex
}

// AttachmentReader describes an attachment opened for HTTP streaming.
type AttachmentReader struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

type eventListener struct {
	handler            func(*types.Email)
	synchronousHandler func(*types.Email) error
	slots              chan struct{}
}

// MailServer represents the SMTP mail server
type MailServer struct {
	storeByID               map[string]*types.Email
	storeOrder              []string
	receivedAtByID          map[string]time.Time
	storeMutex              sync.RWMutex
	storageTransactionMutex sync.RWMutex
	mailDir                 string
	port                    int
	host                    string
	maxMessageBytes         int64
	maxDataConcurrency      int
	retainAllHeaders        atomic.Bool
	dataLimiter             *dataLimiter
	attachmentStore         attachmentstore.Store
	attachmentHealth        attachmentstore.ReadinessProvider
	attachmentUploadTimeout time.Duration
	attachmentOpenTimeout   time.Duration
	attachmentDeleteTimeout time.Duration
	smtpServer              *smtp.Server
	smtpsServer             *smtp.Server // SMTPS server (direct TLS on 465)
	eventChan               chan Event
	listeners               map[string][]eventListener
	listenersMutex          sync.RWMutex
	closers                 []io.Closer
	closersMutex            sync.Mutex
	outgoing                interface {
		RelayMail(email *types.Email, emlPath, relayTo string, isAutoRelay bool, callback func(error))
		RelayMailContext(ctx context.Context, email *types.Email, emlPath, relayTo string, isAutoRelay bool, callback func(error))
		UpdateConfig(config interface{})
		GetConfig() interface{}
		IsAutoRelayEnabled() bool
		Close()
	}
	authConfig          *SMTPAuthConfig
	authVerifier        *credentialVerifier
	authRequireTLS      bool
	tlsConfig           *TLSConfig
	useUUIDForID        bool
	storagePolicy       StoragePolicy
	cleanupCancel       context.CancelFunc
	cleanupWG           sync.WaitGroup
	storageMetricsMutex sync.RWMutex
	storageMetrics      StorageMetrics
	mailboxIndex        MailboxIndex
	mailboxIndexReady   atomic.Bool

	// Storage hooks are intentionally unexported and nil in production. They
	// provide deterministic fault injection for transaction boundary tests.
	beforeStoreCommit          func(*types.Email) error
	beforeAttachmentWrite      func(string) error
	wrapAttachmentWriter       func(io.Writer) io.Writer
	beforeQuarantineMove       func(string) error
	beforeEmailRollback        func(string) error
	beforeEmailDelete          func(string) error
	syncAcceptedFenceDirectory func(string) error

	// DATA hooks are nil in production and provide deterministic synchronization
	// for concurrency and shutdown tests.
	afterDataAcquire  func()
	beforeDataRelease func()
}

// GetHost returns the SMTP server host
func (ms *MailServer) GetHost() string {
	return ms.host
}

// GetPort returns the SMTP server port
func (ms *MailServer) GetPort() int {
	return ms.port
}

// GetMaxMessageBytes returns the configured inbound SMTP message-size limit.
func (ms *MailServer) GetMaxMessageBytes() int64 {
	return ms.maxMessageBytes
}

// GetMaxDataConcurrency returns the per-process SMTP DATA transaction limit.
// Zero means unlimited.
func (ms *MailServer) GetMaxDataConcurrency() int {
	return ms.maxDataConcurrency
}

// GetMailDir returns the mail directory path
func (ms *MailServer) GetMailDir() string {
	return ms.mailDir
}

// GetAttachmentHealth returns the latest cached attachment-store readiness.
// A nil provider means external attachment storage is disabled.
func (ms *MailServer) GetAttachmentHealth() (attachmentstore.HealthStatus, bool) {
	if ms.attachmentHealth == nil {
		return attachmentstore.HealthStatus{}, false
	}
	return ms.attachmentHealth.Snapshot(), true
}

// GetAuthConfig returns the SMTP authentication configuration
func (ms *MailServer) GetAuthConfig() *SMTPAuthConfig {
	return ms.authConfig
}

// GetAuthRequireTLS reports whether SMTP AUTH is restricted to encrypted
// connections. Anonymous delivery in NO AUTH mode is unaffected.
func (ms *MailServer) GetAuthRequireTLS() bool {
	return ms.authRequireTLS
}

// GetTLSConfig returns the TLS configuration
func (ms *MailServer) GetTLSConfig() *TLSConfig {
	return ms.tlsConfig
}

// Event represents a server event
type Event struct {
	Type  string
	Email *types.Email
	ID    string
}
