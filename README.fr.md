# OwlMail

> 🦉 Un serveur Go de développement et de test d'e-mails, avec des workflows de style MailDev et des API propres à OwlMail

[![Go Version](https://img.shields.io/badge/Go-1.27.0+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Workflows](https://img.shields.io/badge/MailDev-Workflow%20Compatibility-blue.svg)](./docs/fr/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
[![Go Report Card](.github/goreportcard.svg)](.github/goreportcard-report.md)
[![codecov](https://codecov.io/gh/soulteary/owlmail/graph/badge.svg?token=AY59NGM1FV)](https://codecov.io/gh/soulteary/owlmail)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](README.md) | [简体中文](README.zh-CN.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Italiano](README.it.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

---

OwlMail est un serveur SMTP avec interface web pour les environnements de
développement et de test. Il couvre des workflows courants de
[MailDev](https://github.com/maildev/maildev), mais utilise ses propres réponses
API et un protocole WebSocket natif. Vérifiez les différences avant de migrer un
client API ou Socket.IO.

![](.github/assets/owlmail-banner.jpg)

## 📸 Aperçu

![Aperçu OwlMail](.github/assets/preview.png)

## 🎥 Vidéo de démonstration

![Vidéo de démonstration](.github/assets/realtime.gif)

## ✨ Features

### Core Features

- ✅ **SMTP Server** - Receives and stores all sent emails (default port 1025)
- ✅ **Web Interface** - View and manage emails through a browser (default port 1080)
- ✅ **Email Persistence** - Emails saved as `.eml` files, supports loading from directory
- ✅ **Email Relay** - Supports forwarding emails to real SMTP servers
- ✅ **Auto Relay** - Supports automatically forwarding all emails with rule filtering
- ✅ **Webhook Forwarding** - Sends matching new emails to HTTP webhooks with custom message templates
- ⚠️ **Authentification SMTP entrante** - Les paramètres existent, mais les expéditeurs non authentifiés ne sont actuellement pas refusés
- ✅ **TLS/STARTTLS** - Supports encrypted connections
- ✅ **SMTPS** - Supports direct TLS connection on port 465 when SMTP TLS is enabled

### Enhanced Features

- 🆕 **Batch Operations** - Batch delete, batch mark as read
- 🆕 **Notifications navigateur** - Notifications temps réel facultatives pour les nouveaux e-mails
- 🆕 **Email Statistics** - Get email statistics
- 🆕 **Email Preview** - Lightweight email preview API
- 🆕 **Email Export** - Export emails as ZIP files
- 🆕 **Configuration Management API** - Complete configuration management (GET/PUT/PATCH)
- 🆕 **Powerful Search** - Full-text search, date range filtering, sorting
- 🆕 **Improved RESTful API** - More standardized API design (`/api/v1/*`)
- 🆕 **Aide intégrée** - Guide local bilingue depuis la boîte de réception ou `/help`
- 🆕 **Configurateur Webhook** - Éditeur local intégré sous `/webhooks` pour créer, importer, valider, copier et télécharger les règles de transfert

### Compatibility

- ✅ **Routes de workflow de style MailDev** - Flux courants d'e-mail, de relais, de configuration et de santé
- ✅ **Alias MailDev sélectionnés** - Les noms `MAILDEV_*` pris en charge figurent dans le tableau de configuration
- ✅ **Règles d'auto-relais** - Règles JSON allow/deny de style MailDev
- ⚠️ **Différences documentées** - Préfixes API, payloads, état de lecture et protocole temps réel diffèrent

### Caractéristiques de déploiement

- ⚡ **Binaire unique** - L'interface et l'aide sont intégrées
- ⚡ **Sans runtime de langage** - Le binaire déployé ne requiert ni Go, ni Bun, ni Node.js
- ⚡ **Concurrence explicite** - Les Webhooks peuvent être limités ou volontairement illimités

Le dépôt ne publie pas de benchmark comparatif reproductible. Mesurez démarrage,
mémoire et débit avec votre propre charge.

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

### Utilisation Docker

#### Récupérer depuis GitHub Container Registry (Recommandé)

La façon la plus simple d'utiliser OwlMail est de récupérer l'image pré-construite depuis GitHub Container Registry :

```bash
# Récupérer la version 0.5.0
docker pull ghcr.io/soulteary/owlmail:0.5.0

# Récupérer l'image d'un commit exact (exemple)
docker pull ghcr.io/soulteary/owlmail:sha-b130f33

# Exécuter le conteneur
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  ghcr.io/soulteary/owlmail:0.5.0
```

**Tags disponibles :**
- `0.5.0` - Tag de version exact ; `0.5` et `0` évoluent avec les versions ultérieures de ces séries
- `sha-<commit>` - Image d'un SHA court précis (par exemple `sha-b130f33`)
- `main` - Image mobile issue du dernier build de `main`
- `latest` - Image mobile de la branche par défaut, pas un sélecteur de version stable

**Support multi-architecture :**
L'image prend en charge les architectures `linux/amd64` et `linux/arm64`. Docker récupérera automatiquement la bonne image pour votre plateforme.

**Voir toutes les images disponibles :** [GitHub Packages](https://github.com/users/soulteary/packages/container/package/owlmail)

#### Construire depuis les sources

##### Build de base (Architecture unique)

```bash
# Construire l'image pour l'architecture actuelle
docker build -t owlmail .

# Exécuter le conteneur
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  owlmail
```

##### Build multi-architecture

Pour aarch64 (ARM64) ou d'autres architectures, utilisez Docker Buildx :

```bash
# Activer buildx (si ce n'est pas déjà fait)
docker buildx create --use --name multiarch-builder

# Construire pour plusieurs architectures
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t owlmail:latest \
  --load .

# Ou construire et pousser vers le registre
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/owlmail:latest \
  --push .

# Construire pour une architecture spécifique (ex. aarch64/arm64)
docker buildx build \
  --platform linux/arm64 \
  -t owlmail:latest \
  --load .
```

**Note** : Le Dockerfile prend maintenant en charge les builds multi-architecture en utilisant les arguments de build `TARGETOS` et `TARGETARCH`, qui sont automatiquement définis par Docker Buildx.

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
| `-webhook-config` | `OWLMAIL_WEBHOOK_CONFIG` | - | JSON webhook forwarding configuration file |
| `-webhook-max-concurrency` | `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` | 8 | Livraisons Webhook simultanées ; `0` désactive la limite |
| `-webhook-redis-url` | `OWLMAIL_WEBHOOK_REDIS_URL` | - | Redis URL for durable webhook delivery |
| `-webhook-redis-prefix` | `OWLMAIL_WEBHOOK_REDIS_PREFIX` | owlmail:webhooks | Redis Streams key prefix |
| `-webhook-shutdown-timeout` | `OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT` | 15s | Graceful webhook drain deadline |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | Nom d’utilisateur SMTP entrant ; pas encore appliqué |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | Mot de passe SMTP entrant ; pas encore appliqué |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | Enable SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS certificate file |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS private key file |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | Log level |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | Use UUID for email IDs (default: 8-character random string) |

Une configuration incomplète de l’authentification Web n’est pas désactivée silencieusement :

| Valeurs configurées | Identifiants effectifs |
|---|---|
| Aucune | Authentification désactivée |
| Nom d’utilisateur uniquement | Le nom d’utilisateur et un mot de passe temporaire aléatoire cryptographiquement sûr de 32 caractères, affiché une fois sur stderr au démarrage |
| Mot de passe uniquement | Nom d’utilisateur `admin` et mot de passe configuré |
| Les deux valeurs | Nom d’utilisateur et mot de passe configurés |

Un mot de passe généré change à chaque redémarrage. Consultez la sortie du
processus (`docker logs owlmail` pour l’exemple avec conteneur), ou configurez
les deux valeurs pour conserver des identifiants stables. OwlMail ne démarre pas
si le mot de passe généré ne peut pas être écrit sur stderr. Utilisez Basic Auth
uniquement via localhost ou HTTPS.

### Environment Variable Compatibility

OwlMail prend en charge les alias MailDev listés dans le tableau et les préfère
aux variables `OWLMAIL_*` correspondantes. Les options MailDev absentes du
tableau ne sont pas automatiquement disponibles.

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

### Routes de compatibilité de style MailDev

OwlMail conserve des routes non versionnées pour les workflows courants, sans
garantir une équivalence exacte avec l'API MailDev actuelle. Consultez la
[référence API](./docs/en/API-Reference.md#maildev-migration-boundary).

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
- `GET /api/v1/ws` - WebSocket connection

Le contrat actuel, avec sous-ressources, authentification, réponses et événements
WebSocket, se trouve dans la [référence API](./docs/en/API-Reference.md).

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

### Limite de l’authentification SMTP entrante

> [!WARNING]
> `-smtp-user` et `-smtp-password` renseignent actuellement la configuration,
> mais la session SMTP ne refuse pas les expéditeurs non authentifiés. Isolez
> l’écoute SMTP avec une interface de confiance, un pare-feu ou un tunnel privé.

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

OwlMail couvre des workflows courants de MailDev, mais les clients actuels
peuvent nécessiter des adaptations explicites. Suivez le
[guide de migration](./docs/fr/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md).

### 1. Environment Variable Compatibility

OwlMail accepte les variables MailDev listées dans le tableau. Vérifiez chacune
de celles utilisées par votre déploiement :

```bash
# MailDev configuration
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# OwlMail peut aussi lire ces variables listées
./owlmail
```

### 2. API Compatibility

Les chemins et payloads API diffèrent. Utilisez l'API OwlMail versionnée pour
les nouvelles intégrations et adaptez explicitement les clients existants :

```bash
# API MailDev actuelle
curl http://localhost:1080/api/email

# API OwlMail
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

For detailed migration guide, see: [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/fr/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

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
- [Fiber](https://github.com/gofiber/fiber) - Web framework
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket library

## 📚 Related Documentation

- [Notes de version OwlMail 0.5.0](./docs/en/Release-0.5.0.md) ([中文](./docs/zh-CN/Release-0.5.0.md))
- [Journal des modifications](./CHANGELOG.md)
- [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/fr/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [Référence API (English)](./docs/en/API-Reference.md)
- [Exploitation et dépannage (English)](./docs/en/Operations.md)
- [Transmission Webhook (English)](./docs/en/Webhook-Forwarding.md)
- [Processus de publication (English)](./docs/en/Releasing.md)
- [Journal de refactorisation API (historique)](./docs/fr/internal/API_Refactoring_Record.md)

## 🐛 Issue Reporting

If you encounter any issues or have suggestions, please submit them in [GitHub Issues](https://github.com/soulteary/owlmail/issues).

## ⭐ Star History

If this project helps you, please give it a Star ⭐!

---

**OwlMail** - Un serveur Go de test d'e-mails avec des chemins de migration MailDev documentés 🦉
