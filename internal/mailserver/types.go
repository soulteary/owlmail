package mailserver

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/soulteary/owlmail/internal/attachmentstore"
	"github.com/soulteary/owlmail/internal/common"
	"github.com/soulteary/owlmail/internal/outgoing"
	"github.com/soulteary/owlmail/internal/types"
)

const (
	defaultPort    = 1025
	defaultHost    = "localhost"
	defaultMailDir = "owlmail"

	// DefaultMaxMessageBytes is the default SMTP DATA limit (100 MiB).
	DefaultMaxMessageBytes   int64 = 100 << 20
	defaultSMTPReadTimeout         = 10 * time.Second
	defaultSMTPWriteTimeout        = 10 * time.Second
	defaultSMTPMaxRecipients       = 50

	// DefaultSMTPSPort is the registered implicit-TLS submission port. It is
	// privileged, so a deployment that cannot bind below 1024 has to move or
	// disable the listener. It has to stay equal to config.DefaultSMTPSPort,
	// which this package deliberately does not import: the mail server is
	// usable without the CLI configuration layer. A test in cmd/owlmail, which
	// already sees both, pins the equality.
	DefaultSMTPSPort = 465

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
// MaxDataConcurrency leaves SMTP DATA concurrency unlimited. Zero protocol
// timeout and recipient fields select their established defaults. Zero
// SMTPSPort is the exception: it leaves the implicit-TLS listener unstarted
// instead of selecting a default, so a caller that wants the historical
// listener has to name DefaultSMTPSPort and accept its privileged bind.
type ServerOptions struct {
	// ReadOnly requires an existing mail directory and prevents constructor
	// writes. It is used by observer processes such as the MCP stdio bridge.
	ReadOnly           bool
	OutgoingConfig     *outgoing.OutgoingConfig
	AuthConfig         *SMTPAuthConfig
	AuthRequireTLS     bool
	TLSConfig          *TLSConfig
	UseUUIDForID       bool
	MaxMessageBytes    int64
	MaxDataConcurrency int
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	MaxRecipients      int
	// SMTPSPort is the port of the implicit-TLS (SMTPS) listener started
	// alongside the plain listener when TLSConfig is enabled. Zero starts no
	// implicit-TLS listener, which leaves STARTTLS on the main port as the
	// only encrypted path and avoids a second, possibly privileged, bind.
	SMTPSPort        int
	RetainAllHeaders bool
	AttachmentStore  attachmentstore.Store
	AttachmentHealth attachmentstore.ReadinessProvider
	MailboxIndex     MailboxIndex
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
	storePositionByID       map[string]int
	nextStorePosition       int
	storeMutex              sync.RWMutex
	storageTransactionMutex sync.RWMutex
	protectedEmailSources   map[string]int
	mailDir                 string
	port                    int
	host                    string
	maxMessageBytes         int64
	maxDataConcurrency      int
	readTimeout             time.Duration
	writeTimeout            time.Duration
	maxRecipients           int
	retainAllHeaders        atomic.Bool
	dataLimiter             *dataLimiter
	attachmentStore         attachmentstore.Store
	attachmentHealth        attachmentstore.ReadinessProvider
	attachmentUploadTimeout time.Duration
	attachmentOpenTimeout   time.Duration
	attachmentDeleteTimeout time.Duration
	smtpServer              *smtp.Server
	// smtpListener is the bound plain-SMTP listener, recorded for the same
	// reason as smtpsListener below: smtp.Server.Close only closes the
	// listeners Serve has already registered, and ListenWithReady signals
	// ready before it calls Serve.
	smtpListener      net.Listener
	smtpListenerMutex sync.Mutex
	smtpsServer       *smtp.Server // SMTPS server (implicit TLS)
	smtpsPort         int
	// smtpsListener is the bound implicit-TLS listener. Shutdown closes it
	// directly because smtp.Server only knows the listeners Serve has already
	// registered, and Serve receives this one from a goroutine.
	smtpsListener      net.Listener
	smtpsListenerMutex sync.Mutex
	eventChan          chan Event
	listeners          map[string][]eventListener
	listenersMutex     sync.RWMutex
	closers            []io.Closer
	closersMutex       sync.Mutex
	outgoing           interface {
		RelayMail(email *types.Email, emlPath, relayTo string, isAutoRelay bool, callback func(error)) error
		RelayMailConfirmed(email *types.Email, emlPath string, recipients []string, callback func(error)) error
		RelayMailContext(ctx context.Context, email *types.Email, emlPath, relayTo string, isAutoRelay bool, callback func(error)) error
		EffectiveRecipients(email *types.Email) ([]string, error)
		UpdateConfig(config interface{}) error
		GetConfig() interface{}
		IsAutoRelayEnabled() bool
		Close()
	}
	outgoingMutex       sync.RWMutex
	outgoingClosed      bool
	authConfig          *SMTPAuthConfig
	authVerifier        *common.CredentialVerifier
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
	receivedMessages    atomic.Uint64
	deletedMessages     atomic.Uint64

	// Storage hooks are intentionally unexported and nil in production. They
	// provide deterministic fault injection for transaction boundary tests.
	beforeStoreCommit          func(*types.Email) error
	beforeAttachmentWrite      func(string) error
	wrapAttachmentWriter       func(io.Writer) io.Writer
	beforeQuarantineMove       func(string) error
	beforeEmailRollback        func(string) error
	beforeEmailDelete          func(string) error
	beforeReadOnlyPublish      func(string)
	syncAcceptedFenceDirectory func(string) error

	// DATA hooks are nil in production and provide deterministic synchronization
	// for concurrency and shutdown tests.
	afterDataAcquire  func()
	beforeDataRelease func()
	afterDataRelease  func()
}

// GetHost returns the SMTP server host
func (ms *MailServer) GetHost() string {
	return ms.host
}

// GetPort returns the SMTP server port
func (ms *MailServer) GetPort() int {
	return ms.port
}

// GetSMTPSPort returns the port of the implicit-TLS listener, or zero when no
// implicit-TLS listener is configured. Callers cannot infer the port from the
// TLS settings alone: STARTTLS on the main port stays available when the
// implicit-TLS listener is switched off.
func (ms *MailServer) GetSMTPSPort() int {
	if ms.smtpsServer == nil {
		return 0
	}
	return ms.smtpsPort
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

// GetReadTimeout returns the SMTP read timeout shared by SMTP and SMTPS.
func (ms *MailServer) GetReadTimeout() time.Duration {
	return ms.readTimeout
}

// GetWriteTimeout returns the SMTP write timeout shared by SMTP and SMTPS.
func (ms *MailServer) GetWriteTimeout() time.Duration {
	return ms.writeTimeout
}

// GetMaxRecipients returns the maximum recipients accepted per message.
func (ms *MailServer) GetMaxRecipients() int {
	return ms.maxRecipients
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
