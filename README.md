# OwlMail

> 🦉 A Go mail development and testing server with MailDev-style workflow compatibility and OwlMail-specific APIs

[![Go Version](https://img.shields.io/badge/Go-1.27.0+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Workflows](https://img.shields.io/badge/MailDev-Workflow%20Compatibility-blue.svg)](./docs/en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
[![Go Report Card](.github/goreportcard.svg)](.github/goreportcard-report.md)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](README.md) | [简体中文](README.zh-CN.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Italiano](README.it.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

---

OwlMail is an SMTP server and web interface for development and testing
environments. It supports common [MailDev](https://github.com/maildev/maildev)
workflows while providing its own versioned API, native WebSocket protocol,
webhooks, and browser notifications. Review the documented compatibility
boundary before migrating API or Socket.IO clients.

![](.github/assets/owlmail-banner.jpg)

## 📸 Preview

![OwlMail Preview](.github/assets/preview.png)

## 🎥 Demo Video

![Demo Video](.github/assets/realtime.gif)

## ✨ Features

### Core Features

- ✅ **SMTP Server** - Receives and stores all sent emails (default port 1025)
- ✅ **Web Interface** - View and manage emails through a browser (default port 1080)
- ✅ **Email Persistence** - Emails saved as `.eml` files, supports loading from directory
- ✅ **S3-compatible Attachments** - Optional object storage for decoded attachments; local storage remains the default
- ✅ **Email Relay** - Supports forwarding emails to real SMTP servers
- ✅ **Auto Relay** - Supports automatically forwarding all emails with rule filtering
- ✅ **Webhook Forwarding** - Sends matching new emails to generic HTTP webhooks with custom payload templates
- ⚠️ **Inbound SMTP Authentication** - Configuration flags exist, but unauthenticated senders are not currently rejected
- ✅ **TLS/STARTTLS** - Supports encrypted connections
- ✅ **SMTPS** - Supports direct TLS connection on port 465 when SMTP TLS is enabled

### Enhanced Features

- 🆕 **Batch Operations** - Batch delete, batch mark as read
- 🆕 **Browser Notifications** - Optional live notifications for newly received email
- 🆕 **Email Statistics** - Get email statistics
- 🆕 **Email Preview** - Lightweight email preview API
- 🆕 **Email Export** - Export emails as ZIP files
- 🆕 **Configuration Management API** - Complete configuration management (GET/PUT/PATCH)
- 🆕 **Powerful Search** - Full-text search, date range filtering, sorting
- 🆕 **Improved RESTful API** - More standardized API design (`/api/v1/*`)
- 🆕 **Built-in Help** - Local bilingual guide available from the inbox or at `/help`
- 🆕 **Webhook Configurator** - Embedded local editor at `/webhooks` for building, importing, validating, copying, and downloading forwarding rules

### Compatibility

- ✅ **MailDev-style Workflow Routes** - Common email, relay, configuration, and health workflows have OwlMail routes
- ✅ **Selected MailDev Environment Aliases** - Supported `MAILDEV_*` names are listed in the configuration table
- ✅ **Auto Relay Rules** - Supports MailDev-style JSON allow/deny rules
- ⚠️ **Documented Differences** - API prefixes and payloads, read side effects, and live-event protocols are not identical

### Deployment Characteristics

- ⚡ **Single Binary** - Compiled executable with the UI and help assets embedded
- ⚡ **No Language Runtime** - The deployed binary does not require Go, Bun, or Node.js
- ⚡ **Explicit Concurrency Controls** - Webhook delivery can be bounded or intentionally unlimited

The repository does not publish a reproducible cross-project benchmark. Measure
startup, memory, and throughput with your own mail volume, storage, TLS, and
webhook targets before making capacity claims.

## 🚀 Quick Start

### Installation

#### Build from Source

```bash
# Clone repository
git clone https://github.com/soulteary/owlmail.git
cd owlmail

# Build
go build -o owlmail ./cmd/owlmail

# Run
./owlmail
```

#### Install with Go

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@latest
owlmail
```

### Basic Usage

```bash
# Start with default configuration (SMTP: 1025, Web: 1080)
./owlmail

# Custom ports
./owlmail -smtp 1025 -web 1080

# Use environment variables
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
./owlmail
```

Open `http://localhost:1080` for the inbox. The **Help** button opens the local
guide at `http://localhost:1080/help`, while **Webhooks** opens the local
configurator at `http://localhost:1080/webhooks`. The configurator generates and
downloads JSON; it does not change the running server. Select the file with
`-webhook-config` and restart OwlMail to activate it. All pages and assets are
embedded in the executable, so installed binaries do not need a separate `web`
folder.

### Docker Usage

#### Pull from GitHub Container Registry (Recommended)

The easiest way to use OwlMail is to pull the pre-built image from GitHub Container Registry:

```bash
# Pull release 0.6.0
docker pull ghcr.io/soulteary/owlmail:0.6.0

# Pull an image for one exact commit (example)
docker pull ghcr.io/soulteary/owlmail:sha-b130f33

# Run container
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  ghcr.io/soulteary/owlmail:0.6.0
```

**Available Tags:**
- `0.6.0` - Exact release tag; `0.6` and `0` move with later releases in those series
- `sha-<commit>` - Image for a specific short commit SHA (for example, `sha-b130f33`)
- `main` - Moving image from the latest `main` branch build
- `latest` - Moving default-branch image; it is not a stable-release selector

**Multi-Architecture Support:**
The image supports both `linux/amd64` and `linux/arm64` architectures. Docker will automatically pull the correct image for your platform.

**View all available images:** [GitHub Packages](https://github.com/users/soulteary/packages/container/package/owlmail)

#### Build from Source

##### Basic Build (Single Architecture)

```bash
# Build image for current architecture
docker build -t owlmail .

# Run container
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  owlmail
```

##### Multi-Architecture Build

For aarch64 (ARM64) or other architectures, use Docker Buildx:

```bash
# Enable buildx (if not already enabled)
docker buildx create --use --name multiarch-builder

# Build for multiple architectures
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t owlmail:latest \
  --load .

# Or build and push to registry
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/owlmail:latest \
  --push .

# Build for specific architecture (e.g., aarch64/arm64)
docker buildx build \
  --platform linux/arm64 \
  -t owlmail:latest \
  --load .
```

**Note**: The Dockerfile now supports multi-architecture builds using `TARGETOS` and `TARGETARCH` build arguments, which are automatically set by Docker Buildx.

### Browser Notifications

Browser notifications are off by default. Click **Notifications off** in the
inbox header to request permission and enable them. The preference is stored in
that browser and can be switched off from the same button. Only messages arriving
through the live WebSocket after notifications are enabled create a notification;
loading existing messages does not.

The Notifications API requires HTTPS or a trusted local origin such as
`http://localhost`. If permission was denied, allow OwlMail in the browser's site
settings before trying again. Notifications show the subject and sender but not
the message body; clicking one focuses OwlMail and opens the message.

## 📖 Configuration Options

### Command Line Arguments

| Argument | Environment Variable | Default | Description |
|----------|---------------------|---------|-------------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP port |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP host |
| `-smtp-max-message-mb` | `OWLMAIL_SMTP_MAX_MESSAGE_MB` | 100 | Maximum inbound message size in MiB |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web API port |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web API host |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | Mail storage directory |
| `-mail-retention-days` | `OWLMAIL_MAIL_RETENTION_DAYS` | 0 | Mail retention days; `0` is unlimited |
| `-mail-max-messages` | `OWLMAIL_MAIL_MAX_MESSAGES` | 0 | Maximum stored messages; `0` is unlimited |
| `-mail-max-disk-mb` | `OWLMAIL_MAIL_MAX_DISK_MB` | 0 | Maximum mailbox MiB; `0` is unlimited |
| `-mail-cleanup-interval` | `OWLMAIL_MAIL_CLEANUP_INTERVAL` | 1h | Background cleanup interval |
| `-s3-enabled` | `OWLMAIL_S3_ENABLED` | false | Store decoded attachments in S3-compatible object storage |
| `-s3-endpoint` | `OWLMAIL_S3_ENDPOINT` | - | Custom S3-compatible endpoint; empty uses AWS S3 |
| `-s3-region` | `OWLMAIL_S3_REGION` | us-east-1 | S3 signing region |
| `-s3-bucket` | `OWLMAIL_S3_BUCKET` | - | Existing bucket for attachments |
| `-s3-prefix` | `OWLMAIL_S3_PREFIX` | owlmail/attachments | Attachment object-key prefix |
| `-s3-access-key` | `OWLMAIL_S3_ACCESS_KEY` | - | Optional static access key; otherwise use the AWS credential chain |
| `-s3-secret-key` | `OWLMAIL_S3_SECRET_KEY` | - | Optional static secret key |
| `-s3-session-token` | `OWLMAIL_S3_SESSION_TOKEN` | - | Optional static credential session token |
| `-s3-use-path-style` | `OWLMAIL_S3_USE_PATH_STYLE` | false | Use path-style bucket addressing for compatible services |
| `-web-user` | `MAILDEV_WEB_USER` / `OWLMAIL_WEB_USER` | - | HTTP Basic Auth username |
| `-web-password` | `MAILDEV_WEB_PASS` / `OWLMAIL_WEB_PASSWORD` | - | HTTP Basic Auth password |
| `-https` | `MAILDEV_HTTPS` / `OWLMAIL_HTTPS_ENABLED` | false | Enable HTTPS |
| `-https-cert` | `MAILDEV_HTTPS_CERT` / `OWLMAIL_HTTPS_CERT` | - | HTTPS certificate file |
| `-https-key` | `MAILDEV_HTTPS_KEY` / `OWLMAIL_HTTPS_KEY` | - | HTTPS private key file |
| `-outgoing-host` | `MAILDEV_OUTGOING_HOST` / `OWLMAIL_OUTGOING_HOST` | - | Outgoing SMTP host |
| `-outgoing-port` | `MAILDEV_OUTGOING_PORT` / `OWLMAIL_OUTGOING_PORT` | 587 | Outgoing SMTP port |
| `-outgoing-user` | `MAILDEV_OUTGOING_USER` / `OWLMAIL_OUTGOING_USER` | - | Outgoing SMTP username |
| `-outgoing-pass` | `MAILDEV_OUTGOING_PASS` / `OWLMAIL_OUTGOING_PASSWORD` | - | Outgoing SMTP password |
| `-outgoing-secure` | `MAILDEV_OUTGOING_SECURE` / `OWLMAIL_OUTGOING_SECURE` | false | Outgoing SMTP TLS |
| `-auto-relay` | `MAILDEV_AUTO_RELAY` / `OWLMAIL_AUTO_RELAY` | false | Enable auto relay |
| `-auto-relay-addr` | `MAILDEV_AUTO_RELAY_ADDR` / `OWLMAIL_AUTO_RELAY_ADDR` | - | Auto relay address |
| `-auto-relay-rules` | `MAILDEV_AUTO_RELAY_RULES` / `OWLMAIL_AUTO_RELAY_RULES` | - | Auto relay rules file |
| `-webhook-config` | `OWLMAIL_WEBHOOK_CONFIG` | - | JSON webhook forwarding configuration file |
| `-webhook-max-concurrency` | `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` | 8 | Concurrent email webhook deliveries; `0` disables the limit |
| `-webhook-redis-url` | `OWLMAIL_WEBHOOK_REDIS_URL` | - | Redis URL for durable, restart-safe webhook delivery |
| `-webhook-redis-prefix` | `OWLMAIL_WEBHOOK_REDIS_PREFIX` | owlmail:webhooks | Redis Streams key prefix |
| `-webhook-shutdown-timeout` | `OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT` | 15s | Graceful webhook drain deadline |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | Inbound SMTP username setting; not currently enforced |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | Inbound SMTP password setting; not currently enforced |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | Enable SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS certificate file |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS private key file |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | Log level |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | Use UUID for email IDs (default: 8-character random string) |

When TLS terminates at a reverse proxy, set `OWLMAIL_WEB_EXTERNAL_SCHEME` to `https`.

When HTTP Basic Auth is enabled, browser API and WebSocket requests are limited
to OwlMail's own origin. Command-line and server-to-server clients that omit the
browser `Origin` header continue to work normally.

Web authentication also fails closed when only one credential is configured:

| Configured values | Effective credentials |
|---|---|
| Neither value | Authentication disabled |
| Username only | The username plus a cryptographically random 32-character temporary password, printed once to stderr at startup |
| Password only | Username `admin` plus the configured password |
| Both values | The configured username and password |

A generated password changes on every restart. Read it from the process output
(`docker logs owlmail` for the container example), or configure both values for
stable credentials. Startup fails if the generated password cannot be written
to stderr. Basic Auth credentials should only be used over localhost or HTTPS.

### Environment Variable Compatibility

OwlMail supports the MailDev environment aliases shown in the table above,
preferring them over the corresponding `OWLMAIL_*` variables. Options that are
not listed are not supported automatically.

```bash
# Use MailDev environment variables directly (recommended)
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com
./owlmail

# Or use OwlMail environment variables
export OWLMAIL_SMTP_PORT=1025
export OWLMAIL_WEB_PORT=1080
./owlmail
```

### S3-compatible attachment storage

S3 storage is disabled by default. OwlMail otherwise keeps decoded attachments
under `-mail-directory`. Enabling S3 moves only decoded attachments to object
storage; raw `.eml` files, metadata, transaction markers, and webhook outbox
data remain local and still require a persistent mail directory.

```bash
export OWLMAIL_S3_ENABLED=true
export OWLMAIL_S3_ENDPOINT=http://minio:9000
export OWLMAIL_S3_REGION=us-east-1
export OWLMAIL_S3_BUCKET=owlmail
export OWLMAIL_S3_PREFIX=owlmail/attachments
export OWLMAIL_S3_ACCESS_KEY=replace-me
export OWLMAIL_S3_SECRET_KEY=replace-me
export OWLMAIL_S3_USE_PATH_STYLE=true
./owlmail -mail-directory ./owlmail-data
```

The bucket must already exist. Omit the endpoint to use AWS S3. Omit OwlMail's
static key settings to use the AWS SDK credential chain, including workload
roles. Attachment keys use
`<prefix>/<email-id>/<generated-filename>`; email deletion and retention cleanup
remove that email's object prefix. Failed remote deletion retains a durable
per-message fence and all local recovery evidence, then retries on the next
request or startup; pending deletions are not republished. Upload must finish
before SMTP accepts the message transaction. `OWLMAIL_MAIL_MAX_DISK_MB` measures
local files and does not include S3 object bytes.

## 📡 API Documentation

### API Response Format

OwlMail uses a standardized API response format:

**Success Response:**
```json
{
  "code": "EMAIL_DELETED",
  "message": "Email deleted",
  "data": { ... }
}
```

**Error Response:**
```json
{
  "code": "EMAIL_NOT_FOUND",
  "error": "EMAIL_NOT_FOUND",
  "message": "Email not found"
}
```

The `code` field contains standardized error/success codes that can be used for internationalization. The `message` field provides English text for backward compatibility.

### Email ID Format

OwlMail supports two email ID formats, and all API endpoints are compatible with both:

- **8-character random string**: Default format, e.g., `aB3dEfGh`
- **UUID format**: 36-character standard UUID, e.g., `550e8400-e29b-41d4-a716-446655440000`

When using the `:id` parameter in API requests, you can use either format. For example:
- `GET /email/aB3dEfGh` - Using random string ID
- `GET /email/550e8400-e29b-41d4-a716-446655440000` - Using UUID ID

### MailDev-style Compatibility API

OwlMail retains unversioned routes for common MailDev-style workflows. They are
not exact current MailDev API equivalents; see the compatibility boundary in
the [API reference](./docs/en/API-Reference.md#maildev-migration-boundary).

#### Email Operations

- `GET /email` - Get all emails (supports pagination and filtering)
  - Query parameters:
    - `limit` (default: 50, max: 1000) - Number of emails to return
    - `offset` (default: 0) - Number of emails to skip
    - `q` - Full-text search query
    - `from` - Filter by sender email address
    - `to` - Filter by recipient email address
    - `dateFrom` - Filter by date from (YYYY-MM-DD format)
    - `dateTo` - Filter by date to (YYYY-MM-DD format)
    - `read` - Filter by read status (true/false)
    - `sortBy` - Sort by field (time, subject, from, size)
    - `sortOrder` - Sort order (asc, desc, default: desc)
  - Example: `GET /email?limit=20&offset=0&q=test&sortBy=time&sortOrder=desc`
- `GET /email/:id` - Get single email
- `DELETE /email/:id` - Delete single email
- `DELETE /email/all` - Delete all emails
- `PATCH /email/read-all` - Mark all emails as read
- `PATCH /email/:id/read` - Mark single email as read

#### Email Content

- `GET /email/:id/html` - Get email HTML content
- `GET /email/:id/attachment/:filename` - Download attachment
- `GET /email/:id/download` - Download raw .eml file
- `GET /email/:id/source` - Get email raw source

#### Email Relay

- `POST /email/:id/relay` - Relay email to configured SMTP server
- `POST /email/:id/relay/:relayTo` - Relay email to specific address

#### Configuration and System

- `GET /config` - Get configuration information
- `GET /healthz` - Health check
- `GET /reloadMailsFromDirectory` - Reload emails from directory
- `GET /socket.io` - WebSocket connection (standard WebSocket, not Socket.IO)

### OwlMail Enhanced API

#### Email Statistics and Preview

- `GET /email/stats` - Get email statistics
- `GET /email/preview` - Get email preview (lightweight)

#### Batch Operations

- `POST /email/batch/delete` - Batch delete emails
- `POST /email/batch/read` - Batch mark as read

#### Email Export

- `GET /email/export` - Export emails as ZIP file

#### Configuration Management

- `GET /config/outgoing` - Get outgoing configuration
- `PUT /config/outgoing` - Update outgoing configuration
- `PATCH /config/outgoing` - Partially update outgoing configuration

### Improved RESTful API (`/api/v1/*`)

OwlMail provides a more standardized RESTful API design:

- `GET /api/v1/emails` - Get all emails (plural resource)
  - Query parameters: Same as `GET /email` (limit, offset, q, from, to, dateFrom, dateTo, read, sortBy, sortOrder)
  - Example: `GET /api/v1/emails?limit=20&offset=0&q=test&sortBy=time&sortOrder=desc`
- `GET /api/v1/emails/:id` - Get single email
- `DELETE /api/v1/emails/:id` - Delete single email
- `DELETE /api/v1/emails` - Delete all emails
- `DELETE /api/v1/emails/batch` - Batch delete
- `PATCH /api/v1/emails/read` - Mark all emails as read
- `PATCH /api/v1/emails/:id/read` - Mark single email as read
- `PATCH /api/v1/emails/batch/read` - Batch mark as read
- `GET /api/v1/emails/stats` - Email statistics
- `GET /api/v1/emails/preview` - Email preview
- `GET /api/v1/emails/export` - Export emails
- `POST /api/v1/emails/reload` - Reload emails
- `GET /api/v1/settings` - Get all settings
- `GET /api/v1/settings/outgoing` - Get outgoing configuration
- `PUT /api/v1/settings/outgoing` - Update outgoing configuration
- `PATCH /api/v1/settings/outgoing` - Partially update outgoing configuration
- `GET /api/v1/health` - Health check
- `GET /api/v1/version` - Version info
- `GET /api/v1/ws` - WebSocket connection

For the current contract, including sub-resources, authentication, response
shapes, and WebSocket events, see the [API Reference](./docs/en/API-Reference.md).

## 🔧 Usage Examples

### Basic Usage

```bash
# Start OwlMail
./owlmail -smtp 1025 -web 1080

# Configure SMTP in your application
SMTP_HOST=localhost
SMTP_PORT=1025
```

### Configure Email Relay

```bash
# Relay to Gmail SMTP
./owlmail \
  -outgoing-host smtp.gmail.com \
  -outgoing-port 587 \
  -outgoing-user your-email@gmail.com \
  -outgoing-pass your-password \
  -outgoing-secure
```

### Auto Relay Mode

```bash
# Create auto relay rules file (relay-rules.json)
cat > relay-rules.json <<EOF
[
  { "allow": "*" },
  { "deny": "*@test.com" },
  { "allow": "ok@test.com" }
]
EOF

# Start auto relay
./owlmail \
  -outgoing-host smtp.gmail.com \
  -outgoing-port 587 \
  -outgoing-user your-email@gmail.com \
  -outgoing-pass your-password \
  -auto-relay \
  -auto-relay-rules relay-rules.json
```

### Webhook Forwarding

Use the embedded configurator at `http://localhost:1080/webhooks` to build a new
version 1 configuration or import and validate an existing one. Editing stays
inside the browser. Download the resulting JSON, then select it with
`-webhook-config` and restart OwlMail; downloading a file does not activate it.

```bash
# Terminal 1: local test receiver
go run ./examples/webhooks/receiver

# Terminal 2: forward every new email with the default JSON payload
./owlmail -webhook-config ./examples/webhooks/minimal.json
```

Webhook targets support case-insensitive wildcard rules, custom JSON-safe body templates, environment-backed secrets, HMAC-SHA256 signatures, timeouts, and bounded retries. See the [scenario examples](./examples/webhooks/README.md) for filtering, custom APIs, multiple targets, plain text, and a runnable `soulteary/webhook` stack. The [Webhook forwarding guide](./docs/en/Webhook-Forwarding.md) is the complete reference.

### Using HTTPS

```bash
./owlmail \
  -https \
  -https-cert /path/to/cert.pem \
  -https-key /path/to/key.pem \
  -web 1080
```

### Inbound SMTP Authentication Limitation

> [!WARNING]
> `-smtp-user` and `-smtp-password` currently populate configuration, but the
> SMTP session does not reject unauthenticated senders. Do not expose the SMTP
> listener to untrusted networks or rely on these flags as an access-control
> boundary; use interface binding, firewall rules, or a private tunnel.

### Using TLS

```bash
./owlmail \
  -tls \
  -tls-cert /path/to/cert.pem \
  -tls-key /path/to/key.pem \
  -smtp 1025
```

**Note**: When TLS is enabled, OwlMail automatically starts an SMTPS server on port 465 in addition to the regular SMTP server. The SMTPS server uses direct TLS connection (no STARTTLS required).

### Using UUID for Email IDs

OwlMail supports two email ID formats:

1. **Default format**: 8-character random string (e.g., `aB3dEfGh`)
2. **UUID format**: 36-character standard UUID (e.g., `550e8400-e29b-41d4-a716-446655440000`)

Using UUID format provides better uniqueness and traceability, especially useful for integration with external systems.

```bash
# Enable UUID using command line flag
./owlmail -use-uuid-for-email-id

# Enable UUID using environment variable
export OWLMAIL_USE_UUID_FOR_EMAIL_ID=true
./owlmail

# Use with other configurations
./owlmail \
  -use-uuid-for-email-id \
  -smtp 1025 \
  -web 1080
```

**Notes**:
- Default uses 8-character random string, compatible with MailDev behavior
- When UUID is enabled, all newly received emails will use UUID format IDs
- The API supports both ID formats, allowing normal query, delete, and operation of emails
- Existing email ID formats will not change; only new emails will use the new ID format

## 🔄 Migrating from MailDev

OwlMail covers common MailDev workflows, but current MailDev clients may require
small, explicit adaptations. Follow the
[migration guide](./docs/en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md).

### 1. Environment Variable Compatibility

OwlMail accepts the MailDev environment variables listed in the configuration
table. Verify every variable used by your deployment:

```bash
# MailDev configuration
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# These listed variables can also be read by OwlMail
./owlmail
```

### 2. API Compatibility

API paths and payloads differ in current MailDev. Use OwlMail's versioned API for
new integrations and adapt existing clients deliberately:

```bash
# Current MailDev API
curl http://localhost:1080/api/email

# OwlMail API
curl http://localhost:1080/api/v1/emails
```

### 3. WebSocket Adaptation

If using WebSocket, you need to change from Socket.IO to standard WebSocket:

```javascript
// MailDev (Socket.IO)
const socket = io('/socket.io');
socket.on('newMail', (email) => { /* ... */ });

// OwlMail (Standard WebSocket)
const ws = new WebSocket('ws://localhost:1080/socket.io');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'new') { /* ... */ }
};
```

For detailed migration guide, see: [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific packages
go test ./internal/api/...
go test ./internal/mailserver/...
```

## 📦 Project Structure

```
OwlMail/
├── cmd/
│   └── owlmail/          # Main program entry
├── internal/
│   ├── api/              # Web API implementation
│   ├── common/           # Common utilities (logging, error handling)
│   ├── maildev/          # MailDev compatibility layer
│   ├── mailserver/       # SMTP server implementation
│   ├── outgoing/         # Email relay implementation
│   ├── types/            # Type definitions
│   └── webhook/          # Webhook filtering, templates, signing, and delivery
├── docs/                 # API, operations, webhook, and migration documentation
├── examples/             # Runnable integration examples
├── tests/                # Browser and documentation contract tests
├── web/                  # Embedded web frontend and local help assets
├── go.mod                # Go module definition
└── README.md             # This document
```

## 🤝 Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [MailDev](https://github.com/maildev/maildev) - Original project inspiration
- [emersion/go-smtp](https://github.com/emersion/go-smtp) - SMTP server library
- [emersion/go-message](https://github.com/emersion/go-message) - Email parsing library
- [Fiber](https://github.com/gofiber/fiber) - Web framework
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket library

## 📚 Related Documentation

- [OwlMail 0.6.0 Release Notes](./docs/en/Release-0.6.0.md)
- [Changelog](./CHANGELOG.md)
- [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [API Reference](./docs/en/API-Reference.md)
- [Operations and Troubleshooting](./docs/en/Operations.md)
- [Webhook Forwarding](./docs/en/Webhook-Forwarding.md)
- [Runnable Webhook Scenarios](./examples/webhooks/README.md)
- [Release Process (maintainers)](./docs/en/Releasing.md)
- [API Refactoring Record (historical)](./docs/en/internal/API_Refactoring_Record.md)

## 🐛 Issue Reporting

If you encounter any issues or have suggestions, please submit them in [GitHub Issues](https://github.com/soulteary/owlmail/issues).

## ⭐ Star History

If this project helps you, please give it a Star ⭐!

---

**OwlMail** - A Go mail development and testing server with documented MailDev migration paths 🦉
