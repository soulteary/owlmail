package outgoing

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/soulteary/owlmail/internal/common"
)

type TLSMode string

const (
	TLSModePlain    TLSMode = "plain"
	TLSModeSTARTTLS TLSMode = "starttls"
	TLSModeSMTPS    TLSMode = "smtps"

	DefaultConnectTimeout      = 10 * time.Second
	DefaultTLSHandshakeTimeout = 10 * time.Second
	DefaultAuthTimeout         = 10 * time.Second
	DefaultEnvelopeTimeout     = 10 * time.Second
	DefaultDataTimeout         = 30 * time.Second
	DefaultQuitTimeout         = 5 * time.Second
)

var (
	ErrSTARTTLSUnsupported  = errors.New("outgoing SMTP server does not support STARTTLS")
	ErrAUTHPlainUnsupported = errors.New("outgoing SMTP server does not advertise AUTH PLAIN")
)

type smtpTimeouts struct {
	connect      time.Duration
	tlsHandshake time.Duration
	auth         time.Duration
	envelope     time.Duration
	data         time.Duration
	quit         time.Duration
}

func (config *OutgoingConfig) withDefaults() *OutgoingConfig {
	if config == nil {
		config = &OutgoingConfig{}
	}
	result := *config
	if result.TLSMode == "" {
		if result.Secure {
			result.TLSMode = TLSModeSMTPS
		} else {
			result.TLSMode = TLSModePlain
		}
	}
	result.TLSMode = TLSMode(strings.ToLower(strings.TrimSpace(string(result.TLSMode))))
	result.Secure = result.TLSMode == TLSModeSMTPS
	setDefault := func(value *string, fallback time.Duration) {
		if strings.TrimSpace(*value) == "" {
			*value = fallback.String()
		}
	}
	setDefault(&result.ConnectTimeout, DefaultConnectTimeout)
	setDefault(&result.TLSHandshakeTimeout, DefaultTLSHandshakeTimeout)
	setDefault(&result.AuthTimeout, DefaultAuthTimeout)
	setDefault(&result.EnvelopeTimeout, DefaultEnvelopeTimeout)
	setDefault(&result.DataTimeout, DefaultDataTimeout)
	setDefault(&result.QuitTimeout, DefaultQuitTimeout)
	result.AllowRules = append([]string(nil), config.AllowRules...)
	result.DenyRules = append([]string(nil), config.DenyRules...)
	return &result
}

func (config *OutgoingConfig) Validate() error {
	config = config.withDefaults()
	switch config.TLSMode {
	case TLSModePlain, TLSModeSTARTTLS, TLSModeSMTPS:
	default:
		return fmt.Errorf("outgoing TLS mode must be plain, starttls, or smtps")
	}
	if (config.User == "") != (config.Password == "") {
		return fmt.Errorf("outgoing username and password must be configured together")
	}
	if config.User != "" && config.TLSMode == TLSModePlain {
		return fmt.Errorf("outgoing SMTP authentication requires starttls or smtps")
	}
	_, err := config.timeouts()
	return err
}

func (config *OutgoingConfig) timeouts() (smtpTimeouts, error) {
	config = config.withDefaults()
	parse := func(name, value string) (time.Duration, error) {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return 0, fmt.Errorf("outgoing %s timeout must be a positive duration", name)
		}
		return duration, nil
	}
	var result smtpTimeouts
	var err error
	if result.connect, err = parse("connect", config.ConnectTimeout); err != nil {
		return result, err
	}
	if result.tlsHandshake, err = parse("TLS handshake", config.TLSHandshakeTimeout); err != nil {
		return result, err
	}
	if result.auth, err = parse("AUTH", config.AuthTimeout); err != nil {
		return result, err
	}
	if result.envelope, err = parse("MAIL/RCPT", config.EnvelopeTimeout); err != nil {
		return result, err
	}
	if result.data, err = parse("DATA", config.DataTimeout); err != nil {
		return result, err
	}
	if result.quit, err = parse("QUIT", config.QuitTimeout); err != nil {
		return result, err
	}
	return result, nil
}

// sendMailContext preserves the streaming helper call shape from the relay
// performance work. secure=true has the fail-closed SMTPS meaning. The final
// argument is retained for source compatibility; plaintext authentication is
// now rejected by configuration validation instead of conditionally skipped.
func sendMailContext(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, source io.ReadCloser, secure, authOnlyWhenAdvertised bool) error {
	_ = authOnlyWhenAdvertised
	return sendMailStreamWithConfig(ctx, addr, auth, from, to, source, (&OutgoingConfig{Secure: secure}).withDefaults())
}

func sendMailWithConfig(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte, rawConfig *OutgoingConfig) error {
	return sendMailStreamWithConfig(ctx, addr, auth, from, to, io.NopCloser(bytes.NewReader(msg)), rawConfig)
}

// sendMailStreamWithConfig performs a bounded SMTP transaction while streaming
// the message through net/smtp's DATA writer. It always closes source.
func sendMailStreamWithConfig(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, source io.ReadCloser, rawConfig *OutgoingConfig) error {
	if source == nil {
		return fmt.Errorf("email source is nil")
	}
	source = &relayReadCloser{ReadCloser: source}
	defer func() {
		if err := source.Close(); err != nil {
			common.Verbose("Failed to close email source: %v", err)
		}
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	config := rawConfig.withDefaults()
	if err := config.Validate(); err != nil {
		return err
	}
	timeouts, err := config.timeouts()
	if err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	connectDeadline := phaseDeadline(ctx, timeouts.connect)
	dialer := &net.Dialer{Deadline: connectDeadline}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return relayContextError(ctx, err)
	}
	// SMTPS performs a separately budgeted TLS handshake before the server
	// greeting. Preserve only the connect budget left after dialing so the
	// handshake cannot reset the dial-and-greeting limit.
	connectRemaining := remainingPhaseBudget(connectDeadline)
	stopCancel := context.AfterFunc(ctx, func() {
		_ = source.Close()
		_ = conn.Close()
	})
	defer stopCancel()
	defer func() {
		if err := conn.Close(); err != nil {
			common.Verbose("Failed to close outgoing SMTP connection: %v", err)
		}
	}()

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         host,
		InsecureSkipVerify: config.InsecureSkipVerify, // #nosec G402 -- explicit opt-in compatibility setting.
	}
	if config.tlsConfig != nil {
		tlsConfig = config.tlsConfig.Clone()
		tlsConfig.MinVersion = tls.VersionTLS12
		tlsConfig.ServerName = host
		tlsConfig.InsecureSkipVerify = config.InsecureSkipVerify
	}

	smtpConn := conn
	if config.TLSMode == TLSModeSMTPS {
		if err := setPhaseDeadline(ctx, conn, timeouts.tlsHandshake); err != nil {
			return err
		}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return relayContextError(ctx, fmt.Errorf("outgoing SMTP TLS handshake failed: %w", err))
		}
		smtpConn = tlsConn
	}

	if config.TLSMode == TLSModeSMTPS {
		if err := setPhaseDeadline(ctx, smtpConn, connectRemaining); err != nil {
			return err
		}
	} else if err := setAbsoluteDeadline(ctx, smtpConn, connectDeadline); err != nil {
		return err
	}
	client, err := smtp.NewClient(smtpConn, host)
	if err != nil {
		return relayContextError(ctx, err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			common.Verbose("Failed to close outgoing SMTP client: %v", err)
		}
	}()

	if config.TLSMode == TLSModeSTARTTLS {
		if err := setPhaseDeadline(ctx, smtpConn, timeouts.tlsHandshake); err != nil {
			return err
		}
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return ErrSTARTTLSUnsupported
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return relayContextError(ctx, fmt.Errorf("outgoing SMTP STARTTLS failed: %w", err))
		}
	}

	if auth != nil {
		if config.TLSMode == TLSModePlain {
			return fmt.Errorf("outgoing SMTP authentication requires TLS")
		}
		ok, mechanisms := client.Extension("AUTH")
		if !ok || !supportsAuthMechanism(mechanisms, "PLAIN") {
			return ErrAUTHPlainUnsupported
		}
		if err := setPhaseDeadline(ctx, smtpConn, timeouts.auth); err != nil {
			return err
		}
		if err := client.Auth(auth); err != nil {
			return relayContextError(ctx, err)
		}
	}

	// MAIL and every RCPT share one absolute deadline so the configured
	// envelope timeout bounds the whole phase, independent of recipient count.
	if err := setPhaseDeadline(ctx, smtpConn, timeouts.envelope); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return relayContextError(ctx, err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return relayContextError(ctx, err)
		}
	}

	if err := setPhaseDeadline(ctx, smtpConn, timeouts.data); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return relayContextError(ctx, err)
	}
	if _, err := copyRelayMessage(ctx, w, source); err != nil {
		// Closing a DATA writer normally sends the terminating dot. Abort the
		// connection first so a source or write failure cannot submit a
		// truncated message, then close the writer to release its resources.
		_ = client.Close()
		if closeErr := w.Close(); closeErr != nil {
			common.Verbose("Failed to close aborted SMTP DATA writer: %v", closeErr)
		}
		return relayContextError(ctx, fmt.Errorf("stream email data: %w", err))
	}
	if err := w.Close(); err != nil {
		return relayContextError(ctx, err)
	}

	if err := setPhaseDeadline(ctx, smtpConn, timeouts.quit); err != nil {
		return err
	}
	if err := client.Quit(); err != nil {
		return relayContextError(ctx, err)
	}
	return nil
}

func phaseDeadline(ctx context.Context, timeout time.Duration) time.Time {
	deadline := time.Now().Add(timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		return contextDeadline
	}
	return deadline
}

func remainingPhaseBudget(deadline time.Time) time.Duration {
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func supportsAuthMechanism(advertised, required string) bool {
	for _, mechanism := range strings.Fields(advertised) {
		if strings.EqualFold(mechanism, required) {
			return true
		}
	}
	return false
}

func setAbsoluteDeadline(ctx context.Context, conn net.Conn, deadline time.Time) error {
	if err := conn.SetDeadline(deadline); err != nil {
		return relayContextError(ctx, err)
	}
	return nil
}

func setPhaseDeadline(ctx context.Context, conn net.Conn, timeout time.Duration) error {
	return setAbsoluteDeadline(ctx, conn, phaseDeadline(ctx, timeout))
}
