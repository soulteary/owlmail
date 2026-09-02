# OwlMail

> 🦉 MailDev 風ワークフローと OwlMail 独自 API を備えた Go 製メール開発・テストサーバー

[![Go Version](https://img.shields.io/badge/Go-1.27.0+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Workflows](https://img.shields.io/badge/MailDev-Workflow%20Compatibility-blue.svg)](./docs/ja/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
[![Go Report Card](.github/goreportcard.svg)](.github/goreportcard-report.md)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](README.md) | [简体中文](README.zh-CN.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Italiano](README.it.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

---

OwlMail は開発・テスト環境向けの SMTP サーバーと Web UI です。
[MailDev](https://github.com/maildev/maildev) の一般的なワークフローを扱いますが、
API レスポンスと WebSocket プロトコルは独自です。API や Socket.IO クライアントを
移行する前に、文書化された相違点を確認してください。

![](.github/assets/owlmail-banner.jpg)

## 📸 プレビュー

![OwlMail プレビュー](.github/assets/preview.png)

## 🎥 デモ動画

![デモ動画](.github/assets/realtime.gif)

## ✨ Features

### Core Features

- ✅ **SMTP Server** - Receives and stores all sent emails (default port 1025)
- ✅ **Web Interface** - View and manage emails through a browser (default port 1080)
- ✅ **Email Persistence** - Emails saved as `.eml` files, supports loading from directory
- ✅ **Email Relay** - Supports forwarding emails to real SMTP servers
- ✅ **Auto Relay** - Supports automatically forwarding all emails with rule filtering
- ✅ **Webhook Forwarding** - Sends matching new emails to HTTP webhooks with custom message templates
- ✅ **受信 SMTP 認証** - PLAIN/LOGIN の必須認証と、設定不要の NO AUTH テストモードに対応
- ✅ **TLS/STARTTLS** - Supports encrypted connections
- ✅ **SMTPS** - Supports direct TLS connection on port 465 when SMTP TLS is enabled

### Enhanced Features

- 🆕 **Batch Operations** - Batch delete, batch mark as read
- 🆕 **ブラウザ通知** - 新着メールの任意リアルタイム通知
- 🆕 **Email Statistics** - Get email statistics
- 🆕 **Email Preview** - Lightweight email preview API
- 🆕 **Email Export** - Export emails as ZIP files
- 🆕 **Configuration Management API** - Complete configuration management (GET/PUT/PATCH)
- 🆕 **Powerful Search** - Full-text search, date range filtering, sorting
- 🆕 **Improved RESTful API** - More standardized API design (`/api/v1/*`)
- 🆕 **内蔵ヘルプ** - 受信トレイまたは `/help` から開けるローカル二言語ガイド
- 🆕 **Webhook 設定ツール** - `/webhooks` で転送ルールの作成、インポート、検証、コピー、ダウンロードができる組み込みローカルエディター

### Compatibility

- ✅ **MailDev 風ワークフロールート** - 一般的なメール、リレー、設定、ヘルスチェックを提供
- ✅ **選択された MailDev 環境変数エイリアス** - 対応する `MAILDEV_*` は設定表に記載
- ✅ **自動リレールール** - MailDev 風 JSON allow/deny ルールをサポート
- ⚠️ **文書化された相違点** - API プレフィックス、ペイロード、既読状態、リアルタイム通信は同一ではない

### デプロイ特性

- ⚡ **単一バイナリ** - UI とヘルプを埋め込み
- ⚡ **言語ランタイム不要** - 配布バイナリは Go、Bun、Node.js を必要としない
- ⚡ **明示的な同時実行制御** - Webhook は上限付き、または意図的に無制限に設定可能

リポジトリには再現可能なプロジェクト間ベンチマークはありません。実際の負荷で
起動時間、メモリ、スループットを測定してください。

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

### Docker の使用

#### GitHub Container Registry から取得（推奨）

OwlMail を使用する最も簡単な方法は、GitHub Container Registry から事前に構築されたイメージを取得することです：

```bash
# リリース 0.6.0 を取得
docker pull ghcr.io/soulteary/owlmail:0.6.0

# 特定コミットのイメージを取得（例）
docker pull ghcr.io/soulteary/owlmail:sha-b130f33

# コンテナを実行
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  ghcr.io/soulteary/owlmail:0.6.0
```

**利用可能なタグ：**
- `0.6.0` - 正確なリリースタグ。`0.6` と `0` は同じ系列の後続リリースで更新
- `sha-<commit>` - 特定の短いコミット SHA のイメージ（例：`sha-b130f33`）
- `main` - 最新の `main` ビルドに追随する可変イメージ
- `latest` - デフォルトブランチに追随する可変イメージで、安定版の指定には使用不可

**マルチアーキテクチャサポート：**
イメージは `linux/amd64` と `linux/arm64` の両方のアーキテクチャをサポートしています。Docker は自動的にプラットフォームに適したイメージを取得します。

**利用可能なすべてのイメージを表示：** [GitHub Packages](https://github.com/users/soulteary/packages/container/package/owlmail)

#### ソースからビルド

##### 基本ビルド（単一アーキテクチャ）

```bash
# 現在のアーキテクチャ用のイメージをビルド
docker build -t owlmail .

# コンテナを実行
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  owlmail
```

##### マルチアーキテクチャビルド

aarch64 (ARM64) またはその他のアーキテクチャの場合、Docker Buildx を使用してください：

```bash
# buildx を有効化（まだ有効でない場合）
docker buildx create --use --name multiarch-builder

# 複数のアーキテクチャ用にビルド
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t owlmail:latest \
  --load .

# またはビルドしてレジストリにプッシュ
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/owlmail:latest \
  --push .

# 特定のアーキテクチャ用にビルド（例：aarch64/arm64）
docker buildx build \
  --platform linux/arm64 \
  -t owlmail:latest \
  --load .
```

**注意**：Dockerfile は、Docker Buildx によって自動的に設定される `TARGETOS` と `TARGETARCH` ビルド引数を使用して、マルチアーキテクチャビルドをサポートするようになりました。

## 📖 Configuration Options

### Command Line Arguments

| Argument | Environment Variable | Default | Description |
|----------|---------------------|---------|-------------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP port |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP host |
| `-smtp-max-message-mb` | `OWLMAIL_SMTP_MAX_MESSAGE_MB` | 100 | 受信メッセージの最大サイズ（MiB） |
| `-smtp-max-concurrency` | `OWLMAIL_SMTP_MAX_CONCURRENCY` | 8 | SMTP・STARTTLS・SMTPS 全体でのプロセス単位の同時 DATA トランザクション数。`0` は無制限。上限時は再試行可能な `451 4.3.2` を返す |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web API port |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web API host |
| `-web-external-url` | `OWLMAIL_WEB_EXTERNAL_URL` | - | 生成するメール深層リンク用のブラウザー公開 HTTP(S) origin。プロキシのサブパスは `-base-pathname` で別途設定 |
| `-base-pathname` | `MAILDEV_BASE_PATHNAME` / `OWLMAIL_BASE_PATHNAME` | - | `/owlmail` などの URL パス接頭辞。既定はルートパス |
| `-mcp-enabled` | `OWLMAIL_MCP_ENABLED` | false | `/mcp` で読み取り専用 MCP Streamable HTTP エンドポイントを有効化 |
| `-mcp-session-timeout` | `OWLMAIL_MCP_SESSION_TIMEOUT` | 30m | アイドル状態の MCP セッションを終了 |
| `-mcp-shutdown-timeout` | `OWLMAIL_MCP_SHUTDOWN_TIMEOUT` | 5s | シャットダウン時に MCP セッションを終了する期限 |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | Mail storage directory |
| `-mail-retention-days` | `OWLMAIL_MAIL_RETENTION_DAYS` | 0 | Mail retention days; `0` is unlimited |
| `-mail-max-messages` | `OWLMAIL_MAIL_MAX_MESSAGES` | 0 | Maximum stored messages; `0` is unlimited |
| `-mail-max-disk-mb` | `OWLMAIL_MAIL_MAX_DISK_MB` | 0 | Maximum mailbox MiB; `0` is unlimited |
| `-mail-cleanup-interval` | `OWLMAIL_MAIL_CLEANUP_INTERVAL` | 1h | Background cleanup interval |
| `-s3-enabled` | `OWLMAIL_S3_ENABLED` | false | デコード済み添付ファイルを S3 互換ストレージに保存 |
| `-s3-endpoint` | `OWLMAIL_S3_ENDPOINT` | - | カスタム S3 エンドポイント。空の場合は AWS S3 |
| `-s3-region` | `OWLMAIL_S3_REGION` | us-east-1 | S3 リージョン |
| `-s3-bucket` | `OWLMAIL_S3_BUCKET` | - | 添付ファイル用の既存バケット |
| `-s3-prefix` | `OWLMAIL_S3_PREFIX` | owlmail/attachments | オブジェクトキープレフィックス |
| `-s3-access-key` | `OWLMAIL_S3_ACCESS_KEY` | - | 任意の静的アクセスキー |
| `-s3-secret-key` | `OWLMAIL_S3_SECRET_KEY` | - | 任意の静的シークレットキー |
| `-s3-session-token` | `OWLMAIL_S3_SESSION_TOKEN` | - | 任意のセッショントークン |
| `-s3-use-path-style` | `OWLMAIL_S3_USE_PATH_STYLE` | false | パス形式のバケットアドレスを使用 |
| `-s3-startup-check` | `OWLMAIL_S3_STARTUP_CHECK` | false | 最初の読み取り専用 S3 バケット確認に失敗した場合は起動を中止 |
| `-s3-health-check-interval` | `OWLMAIL_S3_HEALTH_CHECK_INTERVAL` | 30s | バックグラウンド S3 readiness 確認間隔 |
| `-s3-health-check-timeout` | `OWLMAIL_S3_HEALTH_CHECK_TIMEOUT` | 5s | 各 S3 readiness 確認のタイムアウト |
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
| `-webhook-max-concurrency` | `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` | 8 | 同時 Webhook 配信数。`0` は上限なし |
| `-webhook-redis-url` | `OWLMAIL_WEBHOOK_REDIS_URL` | - | Redis URL for durable webhook delivery |
| `-webhook-redis-prefix` | `OWLMAIL_WEBHOOK_REDIS_PREFIX` | owlmail:webhooks | Redis Streams key prefix |
| `-webhook-shutdown-timeout` | `OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT` | 15s | Graceful webhook drain deadline |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | 受信 SMTP ユーザー名。パスワードと同時設定すると AUTH を必須化 |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | 受信 SMTP パスワード。ユーザー名と同時設定すると AUTH を必須化 |
| `-smtp-auth-require-tls` | `OWLMAIL_SMTP_AUTH_REQUIRE_TLS` | false | TLS 確立前の PLAIN/LOGIN を拒否。SMTP TLS の有効化が必要 |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | Enable SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS certificate file |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS private key file |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | Log level |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | Use UUID for email IDs (default: 8-character random string) |

When TLS terminates at a reverse proxy, set `OWLMAIL_WEB_EXTERNAL_SCHEME` to `https`.

Web 認証の設定値が片方だけの場合でも、認証が暗黙に無効になることはありません。

| 設定値 | 実際の認証情報 |
|---|---|
| どちらも未設定 | 認証は無効 |
| ユーザー名のみ | 指定したユーザー名と、暗号学的に安全な 32 文字の一時パスワード。起動時に一度だけ stderr に出力されます |
| パスワードのみ | ユーザー名 `admin` と指定したパスワード |
| 両方 | 指定したユーザー名とパスワード |

生成されたパスワードは再起動ごとに変わります。プロセス出力
（コンテナ例では `docker logs owlmail`）で確認するか、固定した認証情報が
必要な場合は両方を設定してください。生成したパスワードを stderr に出力
できない場合、OwlMail は起動に失敗します。Basic Auth は localhost または
HTTPS 経由でのみ使用してください。

### Environment Variable Compatibility

OwlMail は表に記載された MailDev 環境変数エイリアスをサポートし、対応する
`OWLMAIL_*` より優先します。記載のない MailDev オプションは自動的には使えません。

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

### MailDev 風互換ルート

OwlMail は一般的なワークフロー用に非バージョンルートを残していますが、現在の
MailDev API と完全に同じではありません。
[API リファレンス](./docs/en/API-Reference.md#maildev-migration-boundary)を参照してください。

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

サブリソース、認証、レスポンス、WebSocket イベントを含む現在の仕様は
[API リファレンス](./docs/en/API-Reference.md)を参照してください。

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

### 受信 SMTP 認証モード

`-smtp-user` と `-smtp-password` をどちらも設定しない場合、既定の **NO
AUTH** モードになります。未認証の配送に加え、SMTP 設定を必須とするアプリの
ために任意の PLAIN/LOGIN 認証情報も受け入れます。両方を設定すると SMTP AUTH
が必須になります。片方だけの設定は NO AUTH に戻らず、起動エラーになります。

認証情報を平文接続で送信させない場合は、SMTP TLS とともに
`-smtp-auth-require-tls`（または `OWLMAIL_SMTP_AUTH_REQUIRE_TLS=true`）を
有効にします。平文 SMTP では PLAIN/LOGIN を通知せず、受け付けませんが、
STARTTLS 後および SMTPS では AUTH を利用できます。NO AUTH モードの匿名配送は
変わりません。有効かつ利用可能な SMTP TLS 設定がない場合、このオプションを
有効にすると起動に失敗します。

> [!WARNING]
> NO AUTH は意図的にアクセス制御を提供しません。開発互換性のため
> PLAIN/LOGIN は TLS なしでも使用できるため、実際の認証情報を使う前に TLS を
> 有効化し、SMTP リスナーを分離してください。

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

OwlMail は一般的な MailDev ワークフローを扱いますが、現在のクライアントには
明示的な修正が必要な場合があります。
[移行ガイド](./docs/ja/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)に従ってください。

### 1. Environment Variable Compatibility

OwlMail は設定表に記載された MailDev 変数を受け付けます。実際に使用する変数を
すべて確認してください：

```bash
# MailDev configuration
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# OwlMail もこれらの記載済み変数を読み取れる
./owlmail
```

### 2. API Compatibility

API パスとペイロードは異なります。新規連携ではバージョン付き OwlMail API を使い、
既存クライアントは明示的に修正してください：

```bash
# 現在の MailDev API
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

For detailed migration guide, see: [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/ja/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

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

- [OwlMail 0.6.0 リリースノート](./docs/en/Release-0.6.0.md) ([中文](./docs/zh-CN/Release-0.6.0.md))
- [変更履歴](./CHANGELOG.md)
- [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/ja/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [API リファレンス (English)](./docs/en/API-Reference.md)
- [運用・トラブルシューティング (English)](./docs/en/Operations.md)
- [Webhook 転送 (English)](./docs/en/Webhook-Forwarding.md)
- [リリース手順 (English)](./docs/en/Releasing.md)
- [API リファクタリング記録（履歴）](./docs/ja/internal/API_Refactoring_Record.md)

## 🐛 Issue Reporting

If you encounter any issues or have suggestions, please submit them in [GitHub Issues](https://github.com/soulteary/owlmail/issues).

## ⭐ Star History

If this project helps you, please give it a Star ⭐!

---

**OwlMail** - MailDev 移行手順を明示した Go 製メール開発・テストサーバー 🦉
