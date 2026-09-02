package outgoing

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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

var ErrSTARTTLSUnsupported = errors.New("outgoing SMTP server does not support STARTTLS")

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

// sendMailTLS preserves the historical helper name while giving Secure the
// MailDev-compatible meaning: implicit TLS from the first byte on the wire.
func sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	return sendMailWithConfig(context.Background(), addr, auth, from, to, msg, (&OutgoingConfig{Secure: true}).withDefaults())
}

// sendMailContext preserves the legacy call shape. secure=true means SMTPS;
// callers that need mandatory STARTTLS use sendMailWithConfig with TLSModeSTARTTLS.
func sendMailContext(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte, secure bool) error {
	return sendMailWithConfig(ctx, addr, auth, from, to, msg, (&OutgoingConfig{Secure: secure}).withDefaults())
}

func sendMailWithConfig(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, msg []byte, rawConfig *OutgoingConfig) error {
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
	stopCancel := context.AfterFunc(ctx, func() { _ = conn.Close() })
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
		if err := setPhaseDeadline(ctx, smtpConn, timeouts.connect); err != nil {
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
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return relayContextError(ctx, err)
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

func setAbsoluteDeadline(ctx context.Context, conn net.Conn, deadline time.Time) error {
	if err := conn.SetDeadline(deadline); err != nil {
		return relayContextError(ctx, err)
	}
	return nil
}

func setPhaseDeadline(ctx context.Context, conn net.Conn, timeout time.Duration) error {
	return setAbsoluteDeadline(ctx, conn, phaseDeadline(ctx, timeout))
}

func relayContextError(ctx context.Context, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	return err
}
