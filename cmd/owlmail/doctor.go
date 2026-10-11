package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/soulteary/owlmail/internal/api"
	"github.com/soulteary/owlmail/internal/config"
	"github.com/soulteary/owlmail/internal/mailserver"
	webhooknotify "github.com/soulteary/owlmail/internal/webhook"
)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
type doctorReport struct {
	Ready    bool                        `json:"ready"`
	Scope    string                      `json:"scope"`
	Checks   []doctorCheck               `json:"checks"`
	Recovery api.RelayRecoveryInspection `json:"relay_recovery"`
}

func doctorConfig(args []string) (*config.Config, error) {
	scan := flag.NewFlagSet("doctor", flag.ContinueOnError)
	scan.SetOutput(io.Discard)
	refs := config.DefineFlags(scan)
	if config.HelpRequested(args, scan) {
		return nil, flag.ErrHelp
	}
	if err := scan.Parse(args); err != nil || scan.NArg() != 0 {
		return nil, fmt.Errorf("invalid doctor options")
	}
	defaults := config.DefaultConfig()
	path := strings.TrimSpace(config.ResolveConfig(scan, refs).ConfigFile)
	if path != "" {
		var err error
		defaults, err = config.LoadConfigFile(path)
		if err != nil {
			return nil, fmt.Errorf("configuration file could not be loaded; check its path and schema")
		}
	}
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	refs = config.DefineFlagsWithDefaults(fs, defaults)
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("invalid doctor options")
	}
	cfg := config.ResolveConfig(fs, refs)
	cfg.SMTPSPortConfigured = cfg.SMTPSPortConfigured || defaults.SMTPSPortConfigured
	return cfg, nil
}
func runDoctor(args []string, stdout, stderr io.Writer) error {
	cfg, err := doctorConfig(args)
	if errors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprintln(stderr, "Usage: owlmail doctor [normal OwlMail configuration flags]\nRead-only, offline JSON diagnostics. Exit 0 means local checks pass; exit 1 means a check failed. No listeners, deliveries, probes, or storage writes.")
		return err
	}
	report := doctorReport{Ready: true, Scope: "offline; network reachability and storage write access are not verified", Checks: make([]doctorCheck, 0)}
	if err != nil {
		report.Ready = false
		report.Checks = append(report.Checks, doctorCheck{Name: "configuration", Status: "fail", Message: err.Error()})
	} else {
		report = inspectDoctorConfig(cfg)
	}
	if err = json.NewEncoder(stdout).Encode(report); err != nil {
		return err
	}
	if !report.Ready {
		return fmt.Errorf("offline diagnostics failed")
	}
	return nil
}
func inspectDoctorConfig(cfg *config.Config) doctorReport {
	r := doctorReport{Ready: true, Scope: "offline; network reachability and storage write access are not verified", Checks: make([]doctorCheck, 0)}
	add := func(name string, ok bool, message string) {
		status := "pass"
		if !ok {
			status = "fail"
			r.Ready = false
		}
		r.Checks = append(r.Checks, doctorCheck{Name: name, Status: status, Message: message})
	}
	note := func(name, message string) {
		r.Checks = append(r.Checks, doctorCheck{Name: name, Status: "warning", Message: message})
	}
	err := config.ValidateConfig(cfg)
	message := "Resolved configuration satisfies the normal startup value contract"
	if err != nil {
		message = "Resolved configuration is invalid"
		for _, category := range []string{"SMTP", "outgoing", "web", "base pathname", "mail", "TLS", "HTTPS", "S3", "auto relay", "webhook", "log", "MCP"} {
			if strings.HasPrefix(strings.ToLower(err.Error()), strings.ToLower(category)) {
				message = category + " configuration is invalid; check this group of settings"
				break
			}
		}
	}
	add("configuration", err == nil, message)
	add("web_auth", (cfg.WebUser == "") == (cfg.WebPassword == ""), "Web username and password must be configured together")
	collision, uncertain := doctorListenerOverlap(cfg.SMTPHost, cfg.SMTPPort, cfg.WebHost, cfg.WebPort)
	if cfg.TLSEnabled && cfg.SMTPSPort != 0 {
		sameSMTP, _ := doctorListenerOverlap(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPHost, cfg.SMTPSPort)
		sameWeb, unknownWeb := doctorListenerOverlap(cfg.SMTPHost, cfg.SMTPSPort, cfg.WebHost, cfg.WebPort)
		collision = collision || sameSMTP || sameWeb
		uncertain = uncertain || unknownWeb
	}
	add("listener_ports", !collision, "Enabled listeners must not overlap in bind address and port")
	if uncertain && !collision {
		note("listener_addresses", "Hostname overlap requires runtime resolution; doctor does not resolve DNS")
	}
	for _, item := range []struct {
		name      string
		enabled   bool
		cert, key string
	}{{"smtp_tls", cfg.TLSEnabled, cfg.TLSCertFile, cfg.TLSKeyFile}, {"web_https", cfg.HTTPSEnabled, cfg.HTTPSCertFile, cfg.HTTPSKeyFile}} {
		if !item.enabled {
			continue
		}
		pair, loadErr := tls.LoadX509KeyPair(item.cert, item.key)
		valid := loadErr == nil
		if valid {
			leaf, parseErr := x509.ParseCertificate(pair.Certificate[0])
			valid = parseErr == nil && time.Now().After(leaf.NotBefore) && time.Now().Before(leaf.NotAfter)
		}
		add(item.name, valid, "Enabled TLS requires readable matching certificate/key files and a currently valid leaf certificate")
	}
	if cfg.WebhookConfig != "" {
		_, err = webhooknotify.Load(cfg.WebhookConfig, nil)
		add("webhook_config", err == nil, "Webhook configuration must be readable and compile successfully")
	}
	if cfg.WebhookRedisURL != "" {
		_, err = redis.ParseURL(cfg.WebhookRedisURL)
		add("webhook_redis_url", err == nil, "Redis URL must have a valid supported format; no connection is attempted")
	}
	if cfg.AutoRelayRules != "" {
		info, statErr := os.Stat(cfg.AutoRelayRules)
		ok := statErr == nil && info.Mode().IsRegular() && info.Size() <= 1<<20
		if ok {
			_, _, err = loadAutoRelayRules(cfg.AutoRelayRules)
			ok = err == nil
		}
		add("relay_rules", ok, "Relay rules must be readable valid JSON within 1 MiB")
	}
	mailDir := mailserver.ResolveMailDirectory(cfg.MailDir)
	info, statErr := os.Stat(mailDir)
	if os.IsNotExist(statErr) {
		note("mail_directory", "Mail directory does not exist yet; doctor does not create it")
	} else {
		add("mail_directory", statErr == nil && info.IsDir(), "Mail directory must be an accessible directory")
	}
	if cfg.MailIndexPath != "" {
		add("mail_index_namespace", mailserver.ValidateMailboxIndexPath(mailDir, cfg.MailIndexPath) == nil, "SQLite index must be outside managed mailbox namespaces")
		parent, statErr := os.Stat(filepath.Dir(cfg.MailIndexPath))
		add("mail_index_parent", statErr == nil && parent.IsDir(), "SQLite index parent directory must exist; doctor does not create or open the index")
	}
	recovery, recoveryErr := api.InspectRelayRecovery(mailDir)
	r.Recovery = recovery
	add("relay_recovery", recoveryErr == nil && recovery.MissingSources == 0 && recovery.DuplicateSources == 0, "Durable relay records must be valid, unique per pending source, and retain regular source EML files")
	if recovery.Pending > 0 {
		note("pending_relay", "Queued relay jobs are present; normal startup will resume them. Doctor does not send mail")
	}
	if cfg.AutoRelay && cfg.OutgoingHost == "" {
		add("auto_relay", false, "Automatic relay requires an outgoing SMTP host")
	}
	if cfg.OutgoingInsecureSkipVerify {
		note("outgoing_tls", "Outgoing certificate verification is disabled")
	}
	return r
}

// Hostnames are not resolved because doctor is offline. Distinct literal addresses
// can share a port; wildcard addresses conservatively overlap other binds.
func doctorListenerOverlap(leftHost string, leftPort int, rightHost string, rightPort int) (bool, bool) {
	if leftPort != rightPort {
		return false, false
	}
	leftHost, rightHost = strings.Trim(leftHost, "[]"), strings.Trim(rightHost, "[]")
	if strings.EqualFold(leftHost, rightHost) {
		return true, false
	}
	left, right := net.ParseIP(leftHost), net.ParseIP(rightHost)
	if leftHost == "" || rightHost == "" || left != nil && left.IsUnspecified() || right != nil && right.IsUnspecified() {
		return true, false
	}
	if left != nil && right != nil {
		return left.Equal(right), false
	}
	return false, true
}
