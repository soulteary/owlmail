# Configuration reference

[English](./Configuration-Reference.md) · [简体中文](../zh-CN/Configuration-Reference.md)

This reference lists the current main branch's flags, environment aliases, and
built-in defaults. `-disable-web` requires main; the original `v0.10.1` release
does not include SMTP-only mode. See the [README version guide](../../README.md#choose-a-version)
before using development-branch commands with a release binary.

## Configuration precedence

Command-line flags override supported MailDev environment aliases, then OwlMail
environment variables, then the flat YAML/JSON file and built-in defaults.
Keys in a config file use flag names without the leading `-`. See
[operations](./Operations.md) for examples and startup behavior.

## Flags and environment variables

| Argument | Environment Variable | Default | Description |
|----------|---------------------|---------|-------------|
| `-config` | `OWLMAIL_CONFIG_FILE` | - | Flat YAML or JSON file whose keys use CLI option names |
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP port |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP host |
| `-smtp-max-message-mb` | `OWLMAIL_SMTP_MAX_MESSAGE_MB` | 100 | Maximum inbound message size in MiB |
| `-smtp-max-concurrency` | `OWLMAIL_SMTP_MAX_CONCURRENCY` | 8 | Concurrent DATA transactions per process across SMTP, STARTTLS, and SMTPS; `0` is unlimited; a full limit returns retryable `451 4.3.2` |
| `-smtp-read-timeout` | `OWLMAIL_SMTP_READ_TIMEOUT` | 10s | SMTP command and DATA read timeout |
| `-smtp-write-timeout` | `OWLMAIL_SMTP_WRITE_TIMEOUT` | 10s | SMTP response write timeout |
| `-smtp-max-recipients` | `OWLMAIL_SMTP_MAX_RECIPIENTS` | 50 | Maximum recipients accepted per message |
| `-disable-web` | `OWLMAIL_DISABLE_WEB` | false | Run SMTP without the HTTP listener (SMTP-only mode) |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web API port |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web API host |
| `-web-external-url` | `OWLMAIL_WEB_EXTERNAL_URL` | - | Browser-visible HTTP(S) origin used in generated email deep links; configure reverse-proxy paths separately with `-base-pathname` |
| `-web-allowed-origins` | `OWLMAIL_WEB_ALLOWED_ORIGINS` | - | Extra browser origins accepted on the Web UI and REST API, in addition to OwlMail's own; `*` disables origin validation |
| `-base-pathname` | `MAILDEV_BASE_PATHNAME` / `OWLMAIL_BASE_PATHNAME` | - | URL path prefix such as `/owlmail`; root remains the default |
| `-maildev-rest-compat` | `OWLMAIL_MAILDEV_REST_COMPAT` | false | Enable the opt-in MailDev `/api` REST facade; Socket.IO remains unsupported |
| `-mailcatcher-rest-compat` | `OWLMAIL_MAILCATCHER_REST_COMPAT` | false | Enable the opt-in MailCatcher `/messages` REST facade |
| `-metrics-enabled` | `OWLMAIL_METRICS_ENABLED` | false | Expose Prometheus metrics at the base-path-aware `/metrics` endpoint; protected by Web Basic Auth when configured |
| `-mcp-enabled` | `OWLMAIL_MCP_ENABLED` | false | Enable the read-only MCP Streamable HTTP endpoint at `/mcp` |
| `-mcp-session-timeout` | `OWLMAIL_MCP_SESSION_TIMEOUT` | 30m | Close idle legacy MCP sessions and cap waits |
| `-mcp-shutdown-timeout` | `OWLMAIL_MCP_SHUTDOWN_TIMEOUT` | 5s | Deadline for closing MCP work during shutdown |
| `-mcp-allowed-origins` | `OWLMAIL_MCP_ALLOWED_ORIGINS` | - | Extra browser origins accepted on `/mcp`, in addition to OwlMail's own; `*` disables origin validation |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | Mail storage directory |
| `-mail-retention-days` | `OWLMAIL_MAIL_RETENTION_DAYS` | 0 | Mail retention days; `0` is unlimited |
| `-mail-max-messages` | `OWLMAIL_MAIL_MAX_MESSAGES` | 0 | Maximum stored messages; `0` is unlimited |
| `-mail-max-disk-mb` | `OWLMAIL_MAIL_MAX_DISK_MB` | 0 | Maximum mailbox MiB; `0` is unlimited |
| `-mail-cleanup-interval` | `OWLMAIL_MAIL_CLEANUP_INTERVAL` | 1h | Background cleanup interval |
| `-mail-index-path` | `OWLMAIL_MAIL_INDEX_PATH` | - | Optional path for a rebuildable SQLite mailbox query index; EML files remain authoritative |
| `-s3-enabled` | `OWLMAIL_S3_ENABLED` | false | Store decoded attachments in S3-compatible object storage |
| `-s3-endpoint` | `OWLMAIL_S3_ENDPOINT` | - | Custom S3-compatible endpoint; empty uses AWS S3 |
| `-s3-region` | `OWLMAIL_S3_REGION` | us-east-1 | S3 signing region |
| `-s3-bucket` | `OWLMAIL_S3_BUCKET` | - | Existing bucket for attachments |
| `-s3-prefix` | `OWLMAIL_S3_PREFIX` | owlmail/attachments | Attachment object-key prefix |
| `-s3-access-key` | `OWLMAIL_S3_ACCESS_KEY` | - | Optional static access key; otherwise use the AWS credential chain |
| `-s3-secret-key` | `OWLMAIL_S3_SECRET_KEY` | - | Optional static secret key |
| `-s3-session-token` | `OWLMAIL_S3_SESSION_TOKEN` | - | Optional static credential session token |
| `-s3-use-path-style` | `OWLMAIL_S3_USE_PATH_STYLE` | false | Use path-style bucket addressing for compatible services |
| `-s3-startup-check` | `OWLMAIL_S3_STARTUP_CHECK` | false | Fail startup if the initial read-only S3 bucket check fails |
| `-s3-health-check-interval` | `OWLMAIL_S3_HEALTH_CHECK_INTERVAL` | 30s | Background S3 readiness refresh interval |
| `-s3-health-check-timeout` | `OWLMAIL_S3_HEALTH_CHECK_TIMEOUT` | 5s | Deadline for each S3 readiness probe |
| `-web-user` | `MAILDEV_WEB_USER` / `OWLMAIL_WEB_USER` | - | HTTP Basic Auth username |
| `-web-password` | `MAILDEV_WEB_PASS` / `OWLMAIL_WEB_PASSWORD` | - | HTTP Basic Auth password |
| `-https` | `MAILDEV_HTTPS` / `OWLMAIL_HTTPS_ENABLED` | false | Enable HTTPS |
| `-https-cert` | `MAILDEV_HTTPS_CERT` / `OWLMAIL_HTTPS_CERT` | - | HTTPS certificate file |
| `-https-key` | `MAILDEV_HTTPS_KEY` / `OWLMAIL_HTTPS_KEY` | - | HTTPS private key file |
| `-outgoing-host` | `MAILDEV_OUTGOING_HOST` / `OWLMAIL_OUTGOING_HOST` | - | Outgoing SMTP host |
| `-outgoing-port` | `MAILDEV_OUTGOING_PORT` / `OWLMAIL_OUTGOING_PORT` | 587 | Outgoing SMTP port |
| `-outgoing-user` | `MAILDEV_OUTGOING_USER` / `OWLMAIL_OUTGOING_USER` | - | Outgoing SMTP username |
| `-outgoing-pass` | `MAILDEV_OUTGOING_PASS` / `OWLMAIL_OUTGOING_PASSWORD` | - | Outgoing SMTP password |
| `-outgoing-secure` | `MAILDEV_OUTGOING_SECURE` / `OWLMAIL_OUTGOING_SECURE` | false | MailDev-compatible alias for implicit TLS/SMTPS |
| `-outgoing-tls-mode` | `OWLMAIL_OUTGOING_TLS_MODE` | - | Transport mode: unset is `plain`; or choose mandatory `starttls` or implicit `smtps` |
| `-outgoing-insecure-skip-verify` | `OWLMAIL_OUTGOING_INSECURE_SKIP_VERIFY` | false | Disable certificate and hostname verification (unsafe, explicit opt-in) |
| `-outgoing-connect-timeout` | `OWLMAIL_OUTGOING_CONNECT_TIMEOUT` | 10s | TCP connection and SMTP greeting deadline |
| `-outgoing-tls-handshake-timeout` | `OWLMAIL_OUTGOING_TLS_HANDSHAKE_TIMEOUT` | 10s | TLS/STARTTLS handshake deadline |
| `-outgoing-auth-timeout` | `OWLMAIL_OUTGOING_AUTH_TIMEOUT` | 10s | AUTH deadline |
| `-outgoing-envelope-timeout` | `OWLMAIL_OUTGOING_ENVELOPE_TIMEOUT` | 10s | MAIL/RCPT deadline |
| `-outgoing-data-timeout` | `OWLMAIL_OUTGOING_DATA_TIMEOUT` | 30s | DATA write and acknowledgement deadline |
| `-outgoing-quit-timeout` | `OWLMAIL_OUTGOING_QUIT_TIMEOUT` | 5s | QUIT deadline |
| `-auto-relay` | `MAILDEV_AUTO_RELAY` / `OWLMAIL_AUTO_RELAY` | false | Enable auto relay |
| `-auto-relay-addr` | `MAILDEV_AUTO_RELAY_ADDR` / `OWLMAIL_AUTO_RELAY_ADDR` | - | Auto relay address |
| `-auto-relay-rules` | `MAILDEV_AUTO_RELAY_RULES` / `OWLMAIL_AUTO_RELAY_RULES` | - | Auto relay rules file |
| `-webhook-config` | `OWLMAIL_WEBHOOK_CONFIG` | - | JSON webhook forwarding configuration file |
| `-webhook-max-concurrency` | `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` | 8 | Concurrent email webhook deliveries; `0` disables the limit |
| `-webhook-redis-url` | `OWLMAIL_WEBHOOK_REDIS_URL` | - | Redis URL for durable, restart-safe webhook delivery |
| `-webhook-redis-prefix` | `OWLMAIL_WEBHOOK_REDIS_PREFIX` | owlmail:webhooks | Redis Streams key prefix |
| `-webhook-shutdown-timeout` | `OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT` | 15s | Graceful webhook drain deadline |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | Inbound SMTP username; configure together with the password to require AUTH |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | Inbound SMTP password; configure together with the username to require AUTH |
| `-smtp-auth-require-tls` | `OWLMAIL_SMTP_AUTH_REQUIRE_TLS` | false | Reject PLAIN/LOGIN before TLS; requires SMTP TLS to be enabled |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | Enable SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS certificate file |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS private key file |
| `-smtps-port` | `OWLMAIL_SMTPS_PORT` | 465 | Implicit TLS (SMTPS) listener port used when TLS is enabled; `0` starts no SMTPS listener |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | Log level |
| `-log-format` | `OWLMAIL_LOG_FORMAT` | console | Log output format: `console` or `json` |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | Use UUID for email IDs (default: 8-character random string) |

`OWLMAIL_WEB_EXTERNAL_SCHEME` selects the externally visible scheme when TLS
terminates at a reverse proxy. Use `-web-external-url` for an explicit public
origin and `-base-pathname` for a proxy subpath.

MCP, metrics, REST facades, S3, Webhooks, and Relay are opt-in. `0` means unlimited
only for the options whose descriptions say so. Default SMTP is NO AUTH;
configure both `-smtp-user` and `-smtp-password` to require authentication.

[Operations and troubleshooting](./Operations.md) covers TLS, authentication,
storage, retention, startup diagnosis, and graceful shutdown.
