# OwlMail

> 🦉 MailDev 스타일 워크플로와 OwlMail 전용 API를 제공하는 Go 이메일 개발·테스트 서버

[![Go Version](https://img.shields.io/badge/Go-1.27.0+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Workflows](https://img.shields.io/badge/MailDev-Workflow%20Compatibility-blue.svg)](./docs/ko/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
[![Go Report Card](.github/goreportcard.svg)](.github/goreportcard-report.md)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](README.md) | [简体中文](README.zh-CN.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Italiano](README.it.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

---

OwlMail은 개발 및 테스트 환경을 위한 SMTP 서버와 Web UI입니다. 일반적인
[MailDev](https://github.com/maildev/maildev) 워크플로를 지원하지만 API 응답과
WebSocket 프로토콜은 OwlMail 고유 형식입니다. API 또는 Socket.IO 클라이언트를
마이그레이션하기 전에 문서화된 차이를 확인하세요.

![](.github/assets/owlmail-banner.jpg)

## 📸 미리보기

![OwlMail 미리보기](.github/assets/preview.png)

## 🎥 데모 비디오

![데모 비디오](.github/assets/realtime.gif)

## ✨ Features

### Core Features

- ✅ **SMTP Server** - Receives and stores all sent emails (default port 1025)
- ✅ **Web Interface** - View and manage emails through a browser (default port 1080)
- ✅ **Email Persistence** - Emails saved as `.eml` files, supports loading from directory
- ✅ **Email Relay** - Supports forwarding emails to real SMTP servers
- ✅ **Auto Relay** - Supports automatically forwarding all emails with rule filtering
- ✅ **Webhook Forwarding** - Sends matching new emails to HTTP webhooks with custom message templates
- ✅ **인바운드 SMTP 인증** - 필수 PLAIN/LOGIN 인증과 설정이 필요 없는 NO AUTH 테스트 모드 지원
- ✅ **TLS/STARTTLS** - Supports encrypted connections
- ✅ **SMTPS** - Supports direct TLS connection on port 465 when SMTP TLS is enabled

### Enhanced Features

- 🆕 **Batch Operations** - Batch delete, batch mark as read
- 🆕 **브라우저 알림** - 새 이메일에 대한 선택적 실시간 알림
- 🆕 **Email Statistics** - Get email statistics
- 🆕 **Email Preview** - Lightweight email preview API
- 🆕 **Email Export** - Export emails as ZIP files
- 🆕 **Configuration Management API** - Complete configuration management (GET/PUT/PATCH)
- 🆕 **Powerful Search** - Full-text search, date range filtering, sorting
- 🆕 **Improved RESTful API** - More standardized API design (`/api/v1/*`)
- 🆕 **내장 도움말** - 받은 편지함 또는 `/help`에서 여는 로컬 이중 언어 가이드
- 🆕 **Webhook 구성 도구** - `/webhooks`에서 전달 규칙을 생성, 가져오기, 검증, 복사 및 다운로드하는 내장 로컬 편집기
- 🆕 **sendmail 호환 CLI** - [`owlmail sendmail`](./docs/ko/Sendmail.md)은 PHP, Cron 및 기존 프로그램의 메일을 일반 SMTP 경계를 통해 전달

### Compatibility

- ✅ **MailDev 스타일 워크플로 경로** - 일반적인 이메일, 릴레이, 설정 및 상태 확인 흐름
- ✅ **선택된 MailDev 환경 변수 별칭** - 지원되는 `MAILDEV_*` 이름은 설정 표에 명시
- ✅ **자동 릴레이 규칙** - MailDev 스타일 JSON allow/deny 규칙 지원
- ⚠️ **문서화된 차이** - API 접두사, 페이로드, 읽음 상태 및 실시간 프로토콜이 동일하지 않음

### 배포 특성

- ⚡ **단일 바이너리** - UI와 도움말이 내장됨
- ⚡ **언어 런타임 불필요** - 배포 바이너리는 Go, Bun 또는 Node.js가 필요하지 않음
- ⚡ **명시적 동시성 제어** - Webhook 전달은 제한하거나 의도적으로 무제한 설정 가능

저장소에는 재현 가능한 프로젝트 간 벤치마크가 없습니다. 실제 부하로 시작 시간,
메모리 및 처리량을 측정하세요.

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

### Docker 사용

#### GitHub Container Registry에서 가져오기 (권장)

OwlMail을 사용하는 가장 쉬운 방법은 GitHub Container Registry에서 사전 빌드된 이미지를 가져오는 것입니다:

```bash
# 0.6.0 릴리스 가져오기
docker pull ghcr.io/soulteary/owlmail:0.6.0

# 정확한 커밋 이미지 가져오기 (예시)
docker pull ghcr.io/soulteary/owlmail:sha-b130f33

# 컨테이너 실행
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  ghcr.io/soulteary/owlmail:0.6.0
```

**사용 가능한 태그:**
- `0.6.0` - 정확한 릴리스 태그. `0.6`와 `0`은 해당 계열의 후속 릴리스에 따라 이동
- `sha-<commit>` - 특정 짧은 커밋 SHA의 이미지(예: `sha-b130f33`)
- `main` - 최신 `main` 빌드를 가리키는 이동 태그
- `latest` - 기본 브랜치를 가리키는 이동 태그이며 안정 릴리스 선택자가 아님

**다중 아키텍처 지원:**
이미지는 `linux/amd64` 및 `linux/arm64` 아키텍처를 모두 지원합니다. Docker는 플랫폼에 맞는 올바른 이미지를 자동으로 가져옵니다.

**사용 가능한 모든 이미지 보기:** [GitHub Packages](https://github.com/users/soulteary/packages/container/package/owlmail)

#### 소스에서 빌드

##### 기본 빌드 (단일 아키텍처)

```bash
# 현재 아키텍처용 이미지 빌드
docker build -t owlmail .

# 컨테이너 실행
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  owlmail
```

##### 다중 아키텍처 빌드

aarch64 (ARM64) 또는 다른 아키텍처의 경우 Docker Buildx를 사용하세요:

```bash
# buildx 활성화 (아직 활성화되지 않은 경우)
docker buildx create --use --name multiarch-builder

# 여러 아키텍처용 빌드
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t owlmail:latest \
  --load .

# 또는 빌드하고 레지스트리에 푸시
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/owlmail:latest \
  --push .

# 특정 아키텍처용 빌드 (예: aarch64/arm64)
docker buildx build \
  --platform linux/arm64 \
  -t owlmail:latest \
  --load .
```

**참고**: Dockerfile은 이제 Docker Buildx에 의해 자동으로 설정되는 `TARGETOS` 및 `TARGETARCH` 빌드 인수를 사용하여 다중 아키텍처 빌드를 지원합니다.

## 📖 Configuration Options

### Command Line Arguments

| Argument | Environment Variable | Default | Description |
|----------|---------------------|---------|-------------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP port |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP host |
| `-smtp-max-message-mb` | `OWLMAIL_SMTP_MAX_MESSAGE_MB` | 100 | 수신 메시지 최대 크기(MiB) |
| `-smtp-max-concurrency` | `OWLMAIL_SMTP_MAX_CONCURRENCY` | 8 | SMTP, STARTTLS, SMTPS 전체에서 프로세스별 동시 DATA 트랜잭션 수; `0`은 무제한; 한도 도달 시 재시도 가능한 `451 4.3.2` 반환 |
| `-smtp-read-timeout` | `OWLMAIL_SMTP_READ_TIMEOUT` | 10s | SMTP 명령 및 DATA 읽기 제한 시간 |
| `-smtp-write-timeout` | `OWLMAIL_SMTP_WRITE_TIMEOUT` | 10s | SMTP 응답 쓰기 제한 시간 |
| `-smtp-max-recipients` | `OWLMAIL_SMTP_MAX_RECIPIENTS` | 50 | 메시지당 최대 수신자 수 |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web API port |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web API host |
| `-web-external-url` | `OWLMAIL_WEB_EXTERNAL_URL` | - | 생성된 이메일 딥 링크에 사용할 브라우저 공개 HTTP(S) origin. 프록시 하위 경로는 `-base-pathname`으로 별도 설정 |
| `-base-pathname` | `MAILDEV_BASE_PATHNAME` / `OWLMAIL_BASE_PATHNAME` | - | `/owlmail` 같은 URL 경로 접두사. 기본값은 루트 경로 |
| `-maildev-rest-compat` | `OWLMAIL_MAILDEV_REST_COMPAT` | false | MailDev `/api` REST facade를 명시적으로 활성화하며 Socket.IO는 계속 호환되지 않음 |
| `-metrics-enabled` | `OWLMAIL_METRICS_ENABLED` | false | 기본 경로를 따르는 `/metrics`에서 Prometheus 메트릭을 노출하며 Web Basic Auth 설정 시 동일하게 보호 |
| `-mcp-enabled` | `OWLMAIL_MCP_ENABLED` | false | `/mcp`에서 읽기 전용 MCP Streamable HTTP 엔드포인트 활성화 |
| `-mcp-session-timeout` | `OWLMAIL_MCP_SESSION_TIMEOUT` | 30m | 유휴 MCP 세션 종료 |
| `-mcp-shutdown-timeout` | `OWLMAIL_MCP_SHUTDOWN_TIMEOUT` | 5s | 종료 시 MCP 세션 정리 제한 시간 |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | Mail storage directory |
| `-mail-retention-days` | `OWLMAIL_MAIL_RETENTION_DAYS` | 0 | Mail retention days; `0` is unlimited |
| `-mail-max-messages` | `OWLMAIL_MAIL_MAX_MESSAGES` | 0 | Maximum stored messages; `0` is unlimited |
| `-mail-max-disk-mb` | `OWLMAIL_MAIL_MAX_DISK_MB` | 0 | Maximum mailbox MiB; `0` is unlimited |
| `-mail-cleanup-interval` | `OWLMAIL_MAIL_CLEANUP_INTERVAL` | 1h | Background cleanup interval |
| `-mail-index-path` | `OWLMAIL_MAIL_INDEX_PATH` | - | 재구축 가능한 SQLite 사서함 쿼리 인덱스의 선택 경로이며 EML 파일이 계속 원본 데이터 |
| `-s3-enabled` | `OWLMAIL_S3_ENABLED` | false | 디코딩된 첨부 파일을 S3 호환 스토리지에 저장 |
| `-s3-endpoint` | `OWLMAIL_S3_ENDPOINT` | - | 사용자 지정 S3 엔드포인트. 비우면 AWS S3 사용 |
| `-s3-region` | `OWLMAIL_S3_REGION` | us-east-1 | S3 리전 |
| `-s3-bucket` | `OWLMAIL_S3_BUCKET` | - | 첨부 파일용 기존 버킷 |
| `-s3-prefix` | `OWLMAIL_S3_PREFIX` | owlmail/attachments | 객체 키 접두사 |
| `-s3-access-key` | `OWLMAIL_S3_ACCESS_KEY` | - | 선택적 정적 액세스 키 |
| `-s3-secret-key` | `OWLMAIL_S3_SECRET_KEY` | - | 선택적 정적 시크릿 키 |
| `-s3-session-token` | `OWLMAIL_S3_SESSION_TOKEN` | - | 선택적 세션 토큰 |
| `-s3-use-path-style` | `OWLMAIL_S3_USE_PATH_STYLE` | false | 경로 스타일 버킷 주소 사용 |
| `-s3-startup-check` | `OWLMAIL_S3_STARTUP_CHECK` | false | 최초 읽기 전용 S3 버킷 확인 실패 시 시작 중단 |
| `-s3-health-check-interval` | `OWLMAIL_S3_HEALTH_CHECK_INTERVAL` | 30s | 백그라운드 S3 readiness 확인 간격 |
| `-s3-health-check-timeout` | `OWLMAIL_S3_HEALTH_CHECK_TIMEOUT` | 5s | 각 S3 readiness 확인 제한 시간 |
| `-web-user` | `MAILDEV_WEB_USER` / `OWLMAIL_WEB_USER` | - | HTTP Basic Auth username |
| `-web-password` | `MAILDEV_WEB_PASS` / `OWLMAIL_WEB_PASSWORD` | - | HTTP Basic Auth password |
| `-https` | `MAILDEV_HTTPS` / `OWLMAIL_HTTPS_ENABLED` | false | Enable HTTPS |
| `-https-cert` | `MAILDEV_HTTPS_CERT` / `OWLMAIL_HTTPS_CERT` | - | HTTPS certificate file |
| `-https-key` | `MAILDEV_HTTPS_KEY` / `OWLMAIL_HTTPS_KEY` | - | HTTPS private key file |
| `-outgoing-host` | `MAILDEV_OUTGOING_HOST` / `OWLMAIL_OUTGOING_HOST` | - | Outgoing SMTP host |
| `-outgoing-port` | `MAILDEV_OUTGOING_PORT` / `OWLMAIL_OUTGOING_PORT` | 587 | Outgoing SMTP port |
| `-outgoing-user` | `MAILDEV_OUTGOING_USER` / `OWLMAIL_OUTGOING_USER` | - | Outgoing SMTP username |
| `-outgoing-pass` | `MAILDEV_OUTGOING_PASS` / `OWLMAIL_OUTGOING_PASSWORD` | - | Outgoing SMTP password |
| `-outgoing-secure` | `MAILDEV_OUTGOING_SECURE` / `OWLMAIL_OUTGOING_SECURE` | false | MailDev 호환 implicit TLS/SMTPS 별칭 |
| `-outgoing-tls-mode` | `OWLMAIL_OUTGOING_TLS_MODE` | - | 미설정 시 `plain`, 또는 필수 `starttls`/implicit `smtps` |
| `-outgoing-insecure-skip-verify` | `OWLMAIL_OUTGOING_INSECURE_SKIP_VERIFY` | false | 인증서/호스트 이름 검증 비활성화(위험) |
| `-outgoing-connect-timeout` | `OWLMAIL_OUTGOING_CONNECT_TIMEOUT` | 10s | 연결/인사 제한 시간 |
| `-outgoing-tls-handshake-timeout` | `OWLMAIL_OUTGOING_TLS_HANDSHAKE_TIMEOUT` | 10s | TLS 핸드셰이크 제한 시간 |
| `-outgoing-auth-timeout` | `OWLMAIL_OUTGOING_AUTH_TIMEOUT` | 10s | AUTH 제한 시간 |
| `-outgoing-envelope-timeout` | `OWLMAIL_OUTGOING_ENVELOPE_TIMEOUT` | 10s | MAIL/RCPT 제한 시간 |
| `-outgoing-data-timeout` | `OWLMAIL_OUTGOING_DATA_TIMEOUT` | 30s | DATA 제한 시간 |
| `-outgoing-quit-timeout` | `OWLMAIL_OUTGOING_QUIT_TIMEOUT` | 5s | QUIT 제한 시간 |
| `-auto-relay` | `MAILDEV_AUTO_RELAY` / `OWLMAIL_AUTO_RELAY` | false | Enable auto relay |
| `-auto-relay-addr` | `MAILDEV_AUTO_RELAY_ADDR` / `OWLMAIL_AUTO_RELAY_ADDR` | - | Auto relay address |
| `-auto-relay-rules` | `MAILDEV_AUTO_RELAY_RULES` / `OWLMAIL_AUTO_RELAY_RULES` | - | Auto relay rules file |
| `-webhook-config` | `OWLMAIL_WEBHOOK_CONFIG` | - | JSON webhook forwarding configuration file |
| `-webhook-max-concurrency` | `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` | 8 | 동시 Webhook 전달 수; `0`은 제한 없음 |
| `-webhook-redis-url` | `OWLMAIL_WEBHOOK_REDIS_URL` | - | Redis URL for durable webhook delivery |
| `-webhook-redis-prefix` | `OWLMAIL_WEBHOOK_REDIS_PREFIX` | owlmail:webhooks | Redis Streams key prefix |
| `-webhook-shutdown-timeout` | `OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT` | 15s | Graceful webhook drain deadline |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | 인바운드 SMTP 사용자 이름; 비밀번호와 함께 설정하면 AUTH 필수 |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | 인바운드 SMTP 비밀번호; 사용자 이름과 함께 설정하면 AUTH 필수 |
| `-smtp-auth-require-tls` | `OWLMAIL_SMTP_AUTH_REQUIRE_TLS` | false | TLS 전 PLAIN/LOGIN 거부; SMTP TLS 활성화 필요 |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | Enable SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS certificate file |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS private key file |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | Log level |
| `-mailcatcher-rest-compat` | `OWLMAIL_MAILCATCHER_REST_COMPAT` | false | 선택적 MailCatcher REST 호환 API 활성화 |
| `-log-format` | `OWLMAIL_LOG_FORMAT` | console | 로그 출력 형식: `console` 또는 `json` |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | Use UUID for email IDs (default: 8-character random string) |

When TLS terminates at a reverse proxy, set `OWLMAIL_WEB_EXTERNAL_SCHEME` to `https`.

웹 인증 값 중 하나만 설정해도 인증이 조용히 비활성화되지 않습니다.

| 설정된 값 | 실제 자격 증명 |
|---|---|
| 둘 다 없음 | 인증 비활성화 |
| 사용자 이름만 | 지정한 사용자 이름과 암호학적으로 안전한 32자 임시 비밀번호. 시작할 때 stderr에 한 번 출력됩니다 |
| 비밀번호만 | 사용자 이름 `admin`과 지정한 비밀번호 |
| 둘 다 | 지정한 사용자 이름과 비밀번호 |

생성된 비밀번호는 재시작할 때마다 바뀝니다. 프로세스 출력(컨테이너
예제에서는 `docker logs owlmail`)에서 확인하거나, 고정된 자격 증명이
필요하면 두 값을 모두 설정하세요. 생성된 비밀번호를 stderr에 쓸 수 없으면
OwlMail은 시작에 실패합니다. Basic Auth는 localhost 또는 HTTPS를 통해서만
사용하세요.

### Environment Variable Compatibility

OwlMail은 표에 나열된 MailDev 환경 변수 별칭을 지원하며 해당 `OWLMAIL_*`
변수보다 우선합니다. 표에 없는 MailDev 옵션은 자동으로 지원되지 않습니다.

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
Basic Auth 및 브라우저 same-origin 미들웨어 오류는 API 핸들러보다 먼저
발생하므로 일반 텍스트 `401` 또는 `403` 응답입니다.

### Email ID Format

OwlMail supports two email ID formats, and all API endpoints are compatible with both:

- **8-character random string**: Default format, e.g., `aB3dEfGh`
- **UUID format**: 36-character standard UUID, e.g., `550e8400-e29b-41d4-a716-446655440000`

When using the `:id` parameter in API requests, you can use either format. For example:
- `GET /email/aB3dEfGh` - Using random string ID
- `GET /email/550e8400-e29b-41d4-a716-446655440000` - Using UUID ID

### MailDev 스타일 호환 경로

OwlMail은 일반적인 워크플로를 위해 버전 없는 경로를 유지하지만 현재 MailDev API와
완전히 같지는 않습니다.
[API 참조](./docs/en/API-Reference.md#maildev-migration-boundary)를 확인하세요.

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
- `GET /api/v1/ready` - 캐시된 종속성 readiness 확인
- `GET /api/v1/version` - 버전 정보
- `GET /api/v1/ws` - WebSocket connection
- `GET /api/v1/openapi.json` - OpenAPI 3.1 계약 (JSON)
- `GET /api/v1/openapi.yaml` - OpenAPI 3.1 계약 (YAML)

하위 리소스, 인증, 응답 및 WebSocket 이벤트를 포함한 현재 계약은
[API 참조](./docs/en/API-Reference.md) 또는
[OpenAPI 계약](./openapi/openapi.yaml)을 확인하세요. 제공되는 계약에는 설정된
기본 경로가 자동으로 포함됩니다.

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
  -outgoing-tls-mode starttls
```

`starttls`는 서버가 STARTTLS를 알리지 않거나 TLS/인증서 검증에 실패하면
평문으로 다운그레이드하지 않고 실패합니다. `smtps`는 연결 시작부터 TLS를
사용하며 `plain` 모드에서는 AUTH를 허용하지 않습니다.

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

### 인바운드 SMTP 인증 모드

`-smtp-user`와 `-smtp-password`를 모두 생략하면 기본 **NO AUTH** 모드가
사용됩니다. 인증 없는 전송과 함께 SMTP 자격 증명을 요구하는 애플리케이션을 위해
임의의 PLAIN/LOGIN 자격 증명도 허용합니다. 두 값을 모두 설정하면 SMTP AUTH가
필수가 됩니다. 하나만 설정하면 NO AUTH로 조용히 돌아가지 않고 시작에 실패합니다.

자격 증명이 평문 연결을 통과하지 않도록 하려면 SMTP TLS와 함께
`-smtp-auth-require-tls`(또는 `OWLMAIL_SMTP_AUTH_REQUIRE_TLS=true`)를
활성화하세요. 평문 SMTP는 PLAIN/LOGIN을 광고하거나 허용하지 않지만 STARTTLS 후와
SMTPS에서는 AUTH가 정상 작동합니다. NO AUTH 모드의 익명 전송은 그대로 유지됩니다.
활성화되고 사용 가능한 SMTP TLS 설정이 없으면 이 옵션을 켠 상태로 시작할 수
없습니다.

> [!WARNING]
> NO AUTH는 의도적으로 접근 제어를 제공하지 않습니다. 개발 호환성을 위해
> PLAIN/LOGIN을 TLS 없이도 사용할 수 있으므로 실제 자격 증명을 사용하기 전에
> TLS를 활성화하고 SMTP 리스너를 격리하세요.

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

OwlMail은 일반적인 MailDev 워크플로를 다루지만 현재 클라이언트에는 명시적인
수정이 필요할 수 있습니다.
[마이그레이션 가이드](./docs/ko/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)를 따르세요.

### 1. Environment Variable Compatibility

OwlMail은 설정 표에 나열된 MailDev 변수를 허용합니다. 배포에서 사용하는 변수를
모두 확인하세요:

```bash
# MailDev configuration
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# OwlMail도 위에 나열된 변수를 읽을 수 있음
./owlmail
```

### 2. API Compatibility

기존 REST 클라이언트는 기본적으로 비활성화된 MailDev facade를 명시적으로
활성화할 수 있습니다. 새 연동은 버전이 지정된 OwlMail API를 사용해야 합니다.
facade는 Socket.IO 호환성을 추가하지 않습니다:

```bash
# 기존 MailDev REST 클라이언트
OWLMAIL_MAILDEV_REST_COMPAT=true ./owlmail
curl http://localhost:1080/api/email

# 새 OwlMail 연동
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

For detailed migration guide, see: [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/ko/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

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

- [OwlMail 0.6.0 릴리스 노트](./docs/en/Release-0.6.0.md) ([中文](./docs/zh-CN/Release-0.6.0.md))
- [변경 기록](./CHANGELOG.md)
- [OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper](./docs/ko/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [API 참조 (English)](./docs/en/API-Reference.md)
- [운영 및 문제 해결 (English)](./docs/en/Operations.md)
- [Webhook 전달 (English)](./docs/en/Webhook-Forwarding.md)
- [릴리스 절차 (English)](./docs/en/Releasing.md)
- [API 리팩토링 기록(과거 자료)](./docs/ko/internal/API_Refactoring_Record.md)

## 🐛 Issue Reporting

If you encounter any issues or have suggestions, please submit them in [GitHub Issues](https://github.com/soulteary/owlmail/issues).

## ⭐ Star History

If this project helps you, please give it a Star ⭐!

---

**OwlMail** - MailDev 마이그레이션 경로가 문서화된 Go 이메일 개발·테스트 서버 🦉
