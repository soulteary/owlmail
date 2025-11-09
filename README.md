# OwlMail

> 🦉 A Go implementation of a mail development and testing tool, fully compatible with MailDev, providing better performance and richer features

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Compatible](https://img.shields.io/badge/MailDev-Compatible-blue.svg)](https://github.com/maildev/maildev)
[![Go Report Card](https://goreportcard.com/badge/github.com/soulteary/owlmail)](https://goreportcard.com/report/github.com/soulteary/owlmail)

OwlMail is an SMTP server and web interface for development and testing environments that captures and displays all sent emails. It's a Go implementation of [MailDev](https://github.com/maildev/maildev) with 100% API compatibility, while providing better performance, lower resource usage, and richer features.

## ✨ Features

### Core Features

- ✅ **SMTP Server** - Receives and stores all sent emails (default port 1025)
- ✅ **Web Interface** - View and manage emails through a browser (default port 1080)
- ✅ **Email Persistence** - Emails saved as `.eml` files, supports loading from directory
- ✅ **Email Relay** - Supports forwarding emails to real SMTP servers
- ✅ **Auto Relay** - Supports automatically forwarding all emails with rule filtering
- ✅ **SMTP Authentication** - Supports PLAIN/LOGIN authentication
- ✅ **TLS/STARTTLS** - Supports encrypted connections
- ✅ **SMTPS** - Supports direct TLS connection on port 465 (OwlMail exclusive)

### Enhanced Features

- 🆕 **Batch Operations** - Batch delete, batch mark as read
- 🆕 **Email Statistics** - Get email statistics
- 🆕 **Email Preview** - Lightweight email preview API
- 🆕 **Email Export** - Export emails as ZIP files
- 🆕 **Configuration Management API** - Complete configuration management (GET/PUT/PATCH)
- 🆕 **Powerful Search** - Full-text search, date range filtering, sorting
- 🆕 **Improved RESTful API** - More standardized API design (`/api/v1/*`)

### Compatibility

- ✅ **100% MailDev API Compatible** - All MailDev API endpoints are supported
- ✅ **Environment Variables Fully Compatible** - Prioritizes MailDev environment variables, no configuration changes needed
- ✅ **Auto Relay Rules Compatible** - JSON configuration file format fully compatible

### Performance Advantages

- ⚡ **Single Binary** - Compiled as a single executable, no runtime required
- ⚡ **Low Resource Usage** - Go compiled, lower memory footprint
- ⚡ **Fast Startup** - Faster startup time
- ⚡ **High Concurrency** - Go goroutines, better concurrent performance

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

### Docker Usage

```bash
# Build image
docker build -t owlmail .

# Run container
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  owlmail
```

## 📖 Configuration Options

### Command Line Arguments

| Argument | Environment Variable | Default | Description |
|----------|---------------------|---------|-------------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP port |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP host |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web API port |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web API host |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | Mail storage directory |
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
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | SMTP authentication username |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | SMTP authentication password |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | Enable SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS certificate file |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS private key file |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | Log level |

### Environment Variable Compatibility

OwlMail **fully supports MailDev environment variables**, prioritizing MailDev environment variables, and falling back to OwlMail environment variables if not present. This means you can use MailDev's configuration directly without modification.

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

## 📡 API Documentation

### MailDev Compatible API

OwlMail is fully compatible with all MailDev API endpoints:

#### Email Operations

- `GET /email` - Get all emails (supports pagination and filtering)
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
- `GET /api/v1/ws` - WebSocket connection

For detailed API documentation, see: [API Refactoring Record](./docs/internal/API_Refactoring_Record.md)

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

### Using HTTPS

```bash
./owlmail \
  -https \
  -https-cert /path/to/cert.pem \
  -https-key /path/to/key.pem \
  -web 1080
```

### Using SMTP Authentication

```bash
./owlmail \
  -smtp-user admin \
  -smtp-password secret \
  -smtp 1025
```

### Using TLS

```bash
./owlmail \
  -tls \
  -tls-cert /path/to/cert.pem \
  -tls-key /path/to/key.pem \
  -smtp 1025
```

## 🔄 Migrating from MailDev

OwlMail is fully compatible with MailDev and can be used as a drop-in replacement:

### 1. Environment Variable Compatibility

OwlMail prioritizes MailDev environment variables, no configuration changes needed:

```bash
# MailDev configuration
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# Use OwlMail directly (no need to change environment variables)
./owlmail
```

### 2. API Compatibility

All MailDev API endpoints are supported, existing client code requires no changes:

```bash
# MailDev API
curl http://localhost:1080/email

# OwlMail fully compatible
curl http://localhost:1080/email
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

For detailed migration guide, see: [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

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
│   └── types/            # Type definitions
├── web/                  # Web frontend files
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
- [Gin](https://github.com/gin-gonic/gin) - Web framework
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket library

## 📚 Related Documentation

- [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [API Refactoring Record](./docs/internal/API_Refactoring_Record.md)

## 🐛 Issue Reporting

If you encounter any issues or have suggestions, please submit them in [GitHub Issues](https://github.com/soulteary/owlmail/issues).

## ⭐ Star History

If this project helps you, please give it a Star ⭐!

---

**OwlMail** - A Go implementation of a mail development and testing tool, fully compatible with MailDev 🦉
