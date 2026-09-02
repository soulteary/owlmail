// Package sendmail implements the sendmail-compatible OwlMail CLI client.
package sendmail

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-sasl"
	smtp "github.com/emersion/go-smtp"
)

// Exit statuses follow sysexits(3), as traditional sendmail-compatible
// programs do. Keep these values stable for callers such as PHP and cron.
const (
	ExitOK          = 0
	ExitUsage       = 64
	ExitDataError   = 65
	ExitUnavailable = 69
	ExitIOError     = 74
	ExitTempFailure = 75
)

const (
	defaultHost    = "127.0.0.1"
	defaultPort    = 1025
	defaultTimeout = 30 * time.Second
)

type errorKind int

const (
	kindUsage errorKind = iota
	kindData
	kindIO
)

type commandError struct {
	kind errorKind
	err  error
}

func (e *commandError) Error() string { return e.err.Error() }
func (e *commandError) Unwrap() error { return e.err }

func newCommandError(kind errorKind, format string, args ...any) error {
	return &commandError{kind: kind, err: fmt.Errorf(format, args...)}
}

type config struct {
	host       string
	port       int
	startTLS   bool
	smtps      bool
	username   string
	password   string
	timeout    time.Duration
	readHeader bool
	from       string
	fromSet    bool
	recipients []string
	tlsConfig  *tls.Config
}

type preparedMessage struct {
	reader           io.Reader
	headerRecipients []string
	headerSender     string
	requiresUTF8     bool
}

// Run reads an RFC 5322 message from stdin and submits it through SMTP. It
// never prints message content, credentials, or an SMTP authentication trace.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	cfg, help, err := parseArgs(args, os.LookupEnv)
	if help {
		_, _ = io.WriteString(stdout, usageText)
		return ExitOK
	}
	if err != nil {
		printError(stderr, err)
		return exitCode(err)
	}

	message, err := prepareMessage(stdin)
	if err != nil {
		printError(stderr, err)
		return exitCode(err)
	}

	recipients := append([]string(nil), cfg.recipients...)
	if cfg.readHeader {
		recipients = append(recipients, message.headerRecipients...)
	}
	if len(recipients) == 0 {
		if cfg.readHeader {
			err = newCommandError(kindData, "message has no To, Cc, Bcc, or Resent recipients")
		} else {
			err = newCommandError(kindUsage, "no recipients; pass recipients or use -t")
		}
		printError(stderr, err)
		return exitCode(err)
	}

	from := message.headerSender
	if cfg.fromSet {
		from = cfg.from
	}
	if err := submit(cfg, from, recipients, message); err != nil {
		printError(stderr, err)
		return exitCode(err)
	}
	return ExitOK
}

func printError(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "owlmail sendmail: %v\n", err)
}

func exitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		switch commandErr.kind {
		case kindUsage:
			return ExitUsage
		case kindData:
			return ExitDataError
		case kindIO:
			return ExitIOError
		}
	}
	var smtpErr *smtp.SMTPError
	if errors.As(err, &smtpErr) {
		if smtpErr.Temporary() {
			return ExitTempFailure
		}
		return ExitUnavailable
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ExitTempFailure
	}
	return ExitUnavailable
}

func parseArgs(args []string, lookupEnv func(string) (string, bool)) (config, bool, error) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "-h" || arg == "--help" {
			return config{}, true, nil
		}
	}
	cfg, err := configFromEnvironment(lookupEnv)
	if err != nil {
		return config{}, false, err
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cfg.recipients = append(cfg.recipients, args[i+1:]...)
			break
		}
		switch arg {
		case "-h", "--help":
			return cfg, true, nil
		case "-t":
			cfg.readHeader = true
		case "-i", "-oi":
			// Dot termination is handled by the SMTP client, so these accepted
			// sendmail compatibility options do not change input processing.
		case "-f":
			value, next, valueErr := nextValue(args, i, "-f")
			if valueErr != nil {
				return config{}, false, valueErr
			}
			i = next
			cfg.from, err = parseMailbox(value, true)
			if err != nil {
				return config{}, false, newCommandError(kindUsage, "invalid -f envelope sender")
			}
			cfg.fromSet = true
		case "--host", "--smtp-host", "--smtp-ip":
			cfg.host, i, err = nextValue(args, i, arg)
			if err != nil {
				return config{}, false, err
			}
		case "--port", "--smtp-port":
			var value string
			value, i, err = nextValue(args, i, arg)
			if err == nil {
				cfg.port, err = parsePort(value, arg)
			}
			if err != nil {
				return config{}, false, err
			}
		case "--username":
			cfg.username, i, err = nextValue(args, i, arg)
			if err != nil {
				return config{}, false, err
			}
		case "--password":
			cfg.password, i, err = nextValue(args, i, arg)
			if err != nil {
				return config{}, false, err
			}
		case "--timeout":
			var value string
			value, i, err = nextValue(args, i, arg)
			if err == nil {
				cfg.timeout, err = parseTimeout(value, arg)
			}
			if err != nil {
				return config{}, false, err
			}
		case "--starttls":
			cfg.startTLS = true
		case "--smtps":
			cfg.smtps = true
		default:
			switch {
			case strings.HasPrefix(arg, "-f") && len(arg) > 2:
				cfg.from, err = parseMailbox(arg[2:], true)
				if err != nil {
					return config{}, false, newCommandError(kindUsage, "invalid -f envelope sender")
				}
				cfg.fromSet = true
			case strings.HasPrefix(arg, "--") && strings.Contains(arg, "="):
				if err := applyLongOption(&cfg, arg); err != nil {
					return config{}, false, err
				}
			case strings.HasPrefix(arg, "-"):
				return config{}, false, newCommandError(kindUsage, "unknown option %q; use -- before recipients beginning with '-'", arg)
			default:
				cfg.recipients = append(cfg.recipients, arg)
			}
		}
	}

	cfg.host = strings.TrimSpace(cfg.host)
	if cfg.host == "" {
		return config{}, false, newCommandError(kindUsage, "SMTP host must not be empty")
	}
	if cfg.startTLS && cfg.smtps {
		return config{}, false, newCommandError(kindUsage, "--starttls and --smtps are mutually exclusive")
	}
	if (cfg.username == "") != (cfg.password == "") {
		return config{}, false, newCommandError(kindUsage, "SMTP username and password must be configured together")
	}
	for i, recipient := range cfg.recipients {
		cfg.recipients[i], err = parseMailbox(recipient, false)
		if err != nil {
			return config{}, false, newCommandError(kindUsage, "invalid recipient argument")
		}
	}
	return cfg, false, nil
}

func configFromEnvironment(lookupEnv func(string) (string, bool)) (config, error) {
	if lookupEnv == nil {
		lookupEnv = func(string) (string, bool) { return "", false }
	}
	cfg := config{host: defaultHost, port: defaultPort, timeout: defaultTimeout}
	if value, ok := lookupEnv("OWLMAIL_SENDMAIL_HOST"); ok {
		cfg.host = value
	}
	if value, ok := lookupEnv("OWLMAIL_SENDMAIL_PORT"); ok {
		port, err := parsePort(value, "OWLMAIL_SENDMAIL_PORT")
		if err != nil {
			return config{}, err
		}
		cfg.port = port
	}
	var err error
	if cfg.startTLS, err = envBool(lookupEnv, "OWLMAIL_SENDMAIL_STARTTLS"); err != nil {
		return config{}, err
	}
	if cfg.smtps, err = envBool(lookupEnv, "OWLMAIL_SENDMAIL_SMTPS"); err != nil {
		return config{}, err
	}
	if value, ok := lookupEnv("OWLMAIL_SENDMAIL_USERNAME"); ok {
		cfg.username = value
	}
	if value, ok := lookupEnv("OWLMAIL_SENDMAIL_PASSWORD"); ok {
		cfg.password = value
	}
	if value, ok := lookupEnv("OWLMAIL_SENDMAIL_TIMEOUT"); ok {
		timeout, err := parseTimeout(value, "OWLMAIL_SENDMAIL_TIMEOUT")
		if err != nil {
			return config{}, err
		}
		cfg.timeout = timeout
	}
	return cfg, nil
}

func envBool(lookupEnv func(string) (string, bool), name string) (bool, error) {
	value, ok := lookupEnv(name)
	if !ok || value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, newCommandError(kindUsage, "%s must be a boolean", name)
	}
	return parsed, nil
}

func nextValue(args []string, index int, name string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, newCommandError(kindUsage, "%s requires a value", name)
	}
	return args[index+1], index + 1, nil
}

func applyLongOption(cfg *config, arg string) error {
	name, value, _ := strings.Cut(arg, "=")
	switch name {
	case "--host", "--smtp-host", "--smtp-ip":
		cfg.host = value
	case "--port", "--smtp-port":
		port, err := parsePort(value, name)
		if err != nil {
			return err
		}
		cfg.port = port
	case "--username":
		cfg.username = value
	case "--password":
		cfg.password = value
	case "--timeout":
		timeout, err := parseTimeout(value, name)
		if err != nil {
			return err
		}
		cfg.timeout = timeout
	case "--starttls":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return newCommandError(kindUsage, "%s must be a boolean", name)
		}
		cfg.startTLS = parsed
	case "--smtps":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return newCommandError(kindUsage, "%s must be a boolean", name)
		}
		cfg.smtps = parsed
	default:
		return newCommandError(kindUsage, "unknown option %q", name)
	}
	return nil
}

func parsePort(value, name string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, newCommandError(kindUsage, "%s must be an integer from 1 to 65535", name)
	}
	return port, nil
}

func parseTimeout(value, name string) (time.Duration, error) {
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, newCommandError(kindUsage, "%s must be a positive duration", name)
	}
	return timeout, nil
}

func parseMailbox(value string, allowEmpty bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "<>" {
		value = ""
	}
	if value == "" {
		if allowEmpty {
			return "", nil
		}
		return "", errors.New("empty mailbox")
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address == "" || strings.ContainsAny(address.Address, "\r\n") {
		return "", errors.New("invalid mailbox")
	}
	return address.Address, nil
}

func prepareMessage(input io.Reader) (preparedMessage, error) {
	reader := bufio.NewReader(input)
	var sanitized bytes.Buffer
	var currentName string
	var currentValue strings.Builder
	var recipientValues []string
	var senderValues []string
	requiresUTF8 := false

	flush := func() {
		value := strings.TrimSpace(currentValue.String())
		switch strings.ToLower(currentName) {
		case "to", "cc", "bcc", "resent-to", "resent-cc", "resent-bcc":
			if value != "" {
				recipientValues = append(recipientValues, value)
			}
		case "sender":
			if value != "" {
				senderValues = append([]string{value}, senderValues...)
			}
		case "from":
			if value != "" {
				senderValues = append(senderValues, value)
			}
		}
		currentName = ""
		currentValue.Reset()
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return preparedMessage{}, newCommandError(kindIO, "read message headers: %v", err)
		}
		withoutEnding := strings.TrimSuffix(line, "\n")
		withoutEnding = strings.TrimSuffix(withoutEnding, "\r")
		if withoutEnding == "" {
			flush()
			if line == "" {
				return preparedMessage{}, newCommandError(kindData, "message is missing the header/body separator")
			}
			sanitized.WriteString(line)
			break
		}
		if errors.Is(err, io.EOF) {
			return preparedMessage{}, newCommandError(kindData, "message is missing the header/body separator")
		}

		if withoutEnding[0] == ' ' || withoutEnding[0] == '\t' {
			if currentName == "" {
				return preparedMessage{}, newCommandError(kindData, "message contains an invalid folded header")
			}
			if !isBlindRecipientHeader(currentName) {
				sanitized.WriteString(line)
				if !isASCII(line) {
					requiresUTF8 = true
				}
			}
			currentValue.WriteByte(' ')
			currentValue.WriteString(strings.TrimSpace(withoutEnding))
			continue
		}

		flush()
		colon := strings.IndexByte(withoutEnding, ':')
		if colon <= 0 {
			return preparedMessage{}, newCommandError(kindData, "message contains an invalid header field")
		}
		currentName = strings.TrimSpace(withoutEnding[:colon])
		if !validHeaderFieldName(currentName) {
			return preparedMessage{}, newCommandError(kindData, "message contains an invalid header field name")
		}
		currentValue.WriteString(strings.TrimSpace(withoutEnding[colon+1:]))
		if !isBlindRecipientHeader(currentName) {
			sanitized.WriteString(line)
			if !isASCII(line) {
				requiresUTF8 = true
			}
		}
	}

	headerRecipients, err := parseHeaderAddresses(recipientValues)
	if err != nil {
		return preparedMessage{}, newCommandError(kindData, "message contains an invalid recipient header address")
	}
	headerSender := ""
	if len(senderValues) > 0 {
		addresses, parseErr := mail.ParseAddressList(senderValues[0])
		if parseErr != nil || len(addresses) == 0 {
			return preparedMessage{}, newCommandError(kindData, "message contains an invalid Sender or From address")
		}
		headerSender = addresses[0].Address
	}
	return preparedMessage{
		reader:           io.MultiReader(bytes.NewReader(sanitized.Bytes()), reader),
		headerRecipients: headerRecipients,
		headerSender:     headerSender,
		requiresUTF8:     requiresUTF8,
	}, nil
}

func parseHeaderAddresses(values []string) ([]string, error) {
	var recipients []string
	for _, value := range values {
		addresses, err := mail.ParseAddressList(value)
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			if address == nil || address.Address == "" || strings.ContainsAny(address.Address, "\r\n") {
				return nil, errors.New("invalid mailbox")
			}
			recipients = append(recipients, address.Address)
		}
	}
	return recipients, nil
}

func isBlindRecipientHeader(name string) bool {
	return strings.EqualFold(name, "bcc") || strings.EqualFold(name, "resent-bcc")
}

func submit(cfg config, from string, recipients []string, message preparedMessage) error {
	address := net.JoinHostPort(cfg.host, strconv.Itoa(cfg.port))
	if cfg.timeout <= 0 {
		cfg.timeout = defaultTimeout
	}
	tlsConfig := cfg.tlsConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.host}
	} else {
		tlsConfig = tlsConfig.Clone()
		if tlsConfig.ServerName == "" {
			tlsConfig.ServerName = cfg.host
		}
	}

	client, err := dialSMTP(address, cfg, tlsConfig)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer func() { _ = client.Close() }()

	if cfg.username != "" {
		if err := client.Auth(sasl.NewPlainClient("", cfg.username, cfg.password)); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	utf8Required := message.requiresUTF8 || !isASCII(from)
	for _, recipient := range recipients {
		utf8Required = utf8Required || !isASCII(recipient)
	}
	var mailOptions *smtp.MailOptions
	if utf8Required {
		mailOptions = &smtp.MailOptions{UTF8: true}
	}
	if err := client.Mail(from, mailOptions); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient, nil); err != nil {
			return fmt.Errorf("SMTP RCPT TO failed: %w", err)
		}
	}
	data, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if err := copyMessage(data, message.reader); err != nil {
		// Do not close the DATA writer: closing sends the terminating dot and
		// could accept a truncated message after a local input error.
		return err
	}
	if err := data.Close(); err != nil {
		return fmt.Errorf("SMTP message submission failed: %w", err)
	}
	// A 250 response to DATA is the acceptance boundary. A later QUIT failure
	// must not make PHP or cron resend an already accepted message.
	_ = client.Quit()
	return nil
}

type cappedDeadlineConn struct {
	net.Conn
	deadline time.Time
}

func (conn *cappedDeadlineConn) SetDeadline(deadline time.Time) error {
	return conn.Conn.SetDeadline(conn.cap(deadline))
}

func (conn *cappedDeadlineConn) SetReadDeadline(deadline time.Time) error {
	return conn.Conn.SetReadDeadline(conn.cap(deadline))
}

func (conn *cappedDeadlineConn) SetWriteDeadline(deadline time.Time) error {
	return conn.Conn.SetWriteDeadline(conn.cap(deadline))
}

func (conn *cappedDeadlineConn) cap(deadline time.Time) time.Time {
	if deadline.IsZero() || deadline.After(conn.deadline) {
		return conn.deadline
	}
	return deadline
}

func dialSMTP(address string, cfg config, tlsConfig *tls.Config) (*smtp.Client, error) {
	connection, err := (&net.Dialer{Timeout: cfg.timeout}).Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	bounded := &cappedDeadlineConn{Conn: connection, deadline: time.Now().Add(cfg.timeout)}
	if err := bounded.SetDeadline(bounded.deadline); err != nil {
		_ = connection.Close()
		return nil, err
	}

	var client *smtp.Client
	switch {
	case cfg.smtps:
		client = smtp.NewClient(tls.Client(bounded, tlsConfig))
	case cfg.startTLS:
		client, err = smtp.NewClientStartTLS(bounded, tlsConfig)
	default:
		client = smtp.NewClient(bounded)
	}
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	client.CommandTimeout = cfg.timeout
	client.SubmissionTimeout = cfg.timeout
	return client, nil
}

func copyMessage(destination io.Writer, source io.Reader) error {
	buffer := make([]byte, 32*1024)
	for {
		read, readErr := source.Read(buffer)
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			if writeErr != nil {
				return fmt.Errorf("transmit message data: %w", writeErr)
			}
			if written != read {
				return fmt.Errorf("transmit message data: %w", io.ErrShortWrite)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return newCommandError(kindIO, "read message data: %v", readErr)
		}
		if read == 0 {
			return newCommandError(kindIO, "read message data: no progress")
		}
	}
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			return false
		}
	}
	return true
}

func validHeaderFieldName(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		// RFC 5322 field names are printable US-ASCII except colon.
		if value[i] < 33 || value[i] > 126 || value[i] == ':' {
			return false
		}
	}
	return true
}

const usageText = `Usage: owlmail sendmail [options] [recipient ...]

Read an RFC 5322 message from stdin and submit it to OwlMail through SMTP.

Sendmail-compatible options:
  -t                       Read To/Cc/Bcc and Resent recipients from headers
  -f ADDRESS               Set the envelope sender (-fADDRESS is also accepted)
  -i, -oi                  Accepted compatibility options; dots are handled safely
  --                       End options (required for recipients beginning with '-')

SMTP options:
  --host HOST              SMTP host (default 127.0.0.1)
  --port PORT              SMTP port (default 1025)
  --starttls               Require STARTTLS
  --smtps                  Use implicit TLS
  --username USER          SMTP AUTH username
  --password PASSWORD      SMTP AUTH password (prefer the environment variable)
  --timeout DURATION       Whole SMTP submission deadline (default 30s)
  -h, --help               Show this help

Environment:
  OWLMAIL_SENDMAIL_HOST, OWLMAIL_SENDMAIL_PORT,
  OWLMAIL_SENDMAIL_STARTTLS, OWLMAIL_SENDMAIL_SMTPS,
  OWLMAIL_SENDMAIL_USERNAME, OWLMAIL_SENDMAIL_PASSWORD,
  OWLMAIL_SENDMAIL_TIMEOUT

Exit status: 0 success, 64 usage, 65 message data, 69 permanent SMTP,
74 local I/O, 75 temporary SMTP or network failure.
`
