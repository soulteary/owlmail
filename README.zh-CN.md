# OwlMail

> 🦉 面向开发者、CI 流水线、自动化系统与 AI Agent 的自托管 AI 原生邮件测试网关。

[![Go Version](https://img.shields.io/badge/Go-1.27.0+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Latest Release](https://img.shields.io/github/v/release/soulteary/owlmail)](https://github.com/soulteary/owlmail/releases)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Workflows](https://img.shields.io/badge/MailDev-Workflow%20Compatibility-blue.svg)](./docs/zh-CN/Comparison-and-Migration.md)
[![Go Report Card](.github/goreportcard.svg)](.github/goreportcard-report.md)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](README.md) | [简体中文](README.zh-CN.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Italiano](README.it.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

---

OwlMail 在应用邮件进入真实邮箱前将其捕获，并转换为确定、可检查的测试数据。
开发者可以通过 Web UI 查看邮件，测试套件可以使用版本化 REST API 和 OpenAPI
合约，自动化系统可以消费持久事件，AI Agent 则可以通过有明确边界的只读 MCP
等待并检查邮件投递。

只需捕获一次，即可通过适合不同工作流的接口验证同一封已完整提交的邮件：

| 使用者 | 接口 | 典型工作流 |
|---|---|---|
| 开发者 | Web UI、浏览器通知 | 检查 HTML、纯文本、Header、源码、链接与附件 |
| 测试套件与 CI | REST API、OpenAPI 3.1、WebSocket | 验证注册、密码重置和通知邮件 |
| 自动化系统 | 签名 Webhook、可选 Redis Streams | 将已提交邮件转换为可跨重启恢复的下游事件 |
| AI 编码 Agent | 基于 Streamable HTTP 或 stdio 的只读 MCP | 搜索、获取和等待邮件，不开放破坏性操作 |
| SMTP 运维方 | 手动与自动 Relay | 按明确的 TLS 与重试策略转发选定测试邮件 |

![](.github/assets/owlmail-banner.jpg)

## 📸 预览

![OwlMail 预览](.github/assets/preview.png)

## 🎥 演示视频

![演示视频](.github/assets/realtime.gif)

## ✨ 为什么选择 OwlMail

- **确定性捕获** —— EML、元数据和附件先进入 staging，原子提交后才会对 API
  与事件消费者可见。
- **面向集成测试** —— 提供版本化 `/api/v1`、OpenAPI 3.1、原生 WebSocket、
  健康与就绪检查、搜索、过滤及导出。
- **AI 原生但不依赖 AI** —— 默认关闭的 MCP 提供七个封闭集合的只读工具、
  有界 Resources、Prompts 和事件驱动的 `wait_for_email`；运行 OwlMail 不需要 LLM。
- **持久化自动化** —— 本地 Webhook outbox、可选 Redis Streams、HMAC 签名、
  稳定投递 ID、有限重试和优雅排空。
- **明确的运维边界** —— SMTP 容量限制、持久化、可选 S3 附件、SQLite 索引、
  Prometheus Metrics、JSON 日志与故障恢复。
- **受控投递** —— 原生异步 Relay 任务仅在配置邮件目录时持久化（否则保存在
  内存中），并使用不可变配置快照、流式 DATA、明确 TLS 模式与状态查询。
- **清晰迁移路径** —— 默认关闭的 MailDev 与 MailCatcher REST facade 支持
  部分现有工作流，但不宣称 Socket.IO 或完全等价兼容。

- **本地工具** —— 使用内嵌的 `/webhooks` 编辑器生成 Webhook 规则，并通过
  [sendmail 指南](./docs/zh-CN/Sendmail.md)接入传统程序。源码和浏览器测试使用 Bun；
  部署后的二进制不需要 Go、Bun 或 Node.js 运行时。

## 🆕 OwlMail 0.8.0

`v0.8.0` 是当前稳定版本，新增配置邮件目录后可持久化的原生 Relay 任务、分层
YAML/JSON 配置、可选 SQLite 索引、Prometheus Metrics、结构化日志、
MailCatcher REST facade、MCP
stdio 桥接、更完整的 Web 收件箱导航、更严格的 API 校验、SMTP 容量控制及附件
下载安全加固。

以下安装示例统一固定为 `ghcr.io/soulteary/owlmail:0.8.0`。需要可重复的 CI
或接近生产的测试环境时，应使用完整版本号或
`ghcr.io/soulteary/owlmail@sha256:<digest>`，不要依赖移动标签。
完整内容参阅 [0.8.0 发布说明](./docs/zh-CN/Release-0.8.0.md)和
[CHANGELOG.md](./CHANGELOG.md)。

> [!IMPORTANT]
> OwlMail 面向开发、测试、CI 与可信内部网络，不是公网生产 MTA、
> exactly-once 消息队列或多租户邮件服务。

## 🚀 快速开始

### 安装

#### 从源码编译

```bash
# 克隆仓库
git clone --branch v0.8.0 --depth 1 https://github.com/soulteary/owlmail.git
cd owlmail

# 编译
go build -o owlmail ./cmd/owlmail

# 运行
./owlmail
```

#### 使用 Go 安装

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@v0.8.0
owlmail
```

### 基本使用

```bash
# 使用默认配置启动（SMTP: 1025, Web: 1080）
./owlmail

# 自定义端口
./owlmail -smtp 1025 -web 1080

# 使用环境变量
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
./owlmail
```

打开 `http://localhost:1080` 访问收件箱。点击**帮助**可打开
`http://localhost:1080/help`，点击 **Webhook 配置**可打开
`http://localhost:1080/webhooks`。配置器只生成和下载 JSON，不会修改正在运行的
服务；需要使用 `-webhook-config` 指定文件并重启 OwlMail 后才会生效。所有页面及
静态资源均已嵌入可执行文件，安装后的二进制无需额外携带 `web` 目录。

### Docker 使用

#### 从 GitHub Container Registry 拉取镜像（推荐）

使用 OwlMail 最简单的方式是从 GitHub Container Registry 拉取预构建的镜像：

```bash
# 拉取 0.8.0 发布镜像
docker pull ghcr.io/soulteary/owlmail:0.8.0

# 拉取某个准确提交的镜像（示例）
docker pull ghcr.io/soulteary/owlmail:sha-e3d2cfc

# 运行容器
docker run -d \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  --name owlmail \
  ghcr.io/soulteary/owlmail:0.8.0
```

**可用标签：**
- `0.8.0` - 准确发布标签；`0.8` 与 `0` 会随对应版本系列的后续发布移动
- `sha-<commit>` - 指定短提交 SHA 的镜像（例如 `sha-e3d2cfc`）
- `main` - 随 `main` 分支最新构建移动的镜像
- `latest` - 随默认分支移动的镜像，不是稳定版本选择器

**多架构支持：**
镜像支持 `linux/amd64` 和 `linux/arm64` 两种架构。Docker 会自动为您的平台拉取正确的镜像。

**查看所有可用镜像：** [GitHub Packages](https://github.com/users/soulteary/packages/container/package/owlmail)

#### 从源码构建

##### 基础构建（单架构）

```bash
# 为当前架构构建镜像
docker build -t owlmail .

# 运行容器
docker run -d \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  --name owlmail \
  owlmail
```

##### 多架构构建

对于 aarch64 (ARM64) 或其他架构，请使用 Docker Buildx：

```bash
# 启用 buildx（如果尚未启用）
docker buildx create --use --name multiarch-builder

# 为多个架构构建
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t owlmail:latest \
  --load .

# 或构建并推送到镜像仓库
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/owlmail:latest \
  --push .

# 为特定架构构建（例如 aarch64/arm64）
docker buildx build \
  --platform linux/arm64 \
  -t owlmail:latest \
  --load .
```

**注意**：Dockerfile 现在支持使用 `TARGETOS` 和 `TARGETARCH` 构建参数进行多架构构建，这些参数由 Docker Buildx 自动设置。

### 浏览器通知

浏览器通知默认关闭。点击收件箱顶部的**通知已关闭**按钮后，浏览器才会请求
权限并开启通知；设置保存在当前浏览器中，也可以随时通过同一按钮关闭。
只有开启后通过 WebSocket 实时到达的新邮件会触发通知，加载已有邮件不会通知。

Notifications API 需要 HTTPS，或 `http://localhost` 等受信任的本地来源。
如果曾拒绝权限，需要先在浏览器的网站设置中允许 OwlMail 通知。通知只显示
主题和发件人，不包含邮件正文；点击通知会聚焦 OwlMail 并打开对应邮件。

## 📖 配置选项

### 命令行参数

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP 端口 |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP 主机 |
| `-smtp-max-message-mb` | `OWLMAIL_SMTP_MAX_MESSAGE_MB` | 100 | 单封入站邮件上限，单位 MiB |
| `-smtp-max-concurrency` | `OWLMAIL_SMTP_MAX_CONCURRENCY` | 8 | 每进程跨 SMTP、STARTTLS 与 SMTPS 的并发 DATA 事务数；`0` 表示不限制；达到上限返回可重试的 `451 4.3.2` |
| `-smtp-read-timeout` | `OWLMAIL_SMTP_READ_TIMEOUT` | 10s | SMTP 命令与 DATA 读取超时 |
| `-smtp-write-timeout` | `OWLMAIL_SMTP_WRITE_TIMEOUT` | 10s | SMTP 响应写入超时 |
| `-smtp-max-recipients` | `OWLMAIL_SMTP_MAX_RECIPIENTS` | 50 | 单封邮件允许的最大收件人数 |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web API 端口 |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web API 主机 |
| `-web-external-url` | `OWLMAIL_WEB_EXTERNAL_URL` | - | 生成邮件深链接时使用的浏览器可见 HTTP(S) origin；反向代理子路径仍通过 `-base-pathname` 配置 |
| `-base-pathname` | `MAILDEV_BASE_PATHNAME` / `OWLMAIL_BASE_PATHNAME` | - | URL 子路径前缀，例如 `/owlmail`；默认仍为根路径 |
| `-maildev-rest-compat` | `OWLMAIL_MAILDEV_REST_COMPAT` | false | 显式启用 MailDev `/api` REST 兼容 facade；仍不支持 Socket.IO |
| `-metrics-enabled` | `OWLMAIL_METRICS_ENABLED` | false | 在跟随基础路径的 `/metrics` 端点暴露 Prometheus 指标；配置 Web Basic Auth 后同样受其保护 |
| `-mcp-enabled` | `OWLMAIL_MCP_ENABLED` | false | 在 `/mcp` 启用只读 MCP Streamable HTTP 端点 |
| `-mcp-session-timeout` | `OWLMAIL_MCP_SESSION_TIMEOUT` | 30m | 关闭空闲 MCP 会话 |
| `-mcp-shutdown-timeout` | `OWLMAIL_MCP_SHUTDOWN_TIMEOUT` | 5s | 关闭时清理 MCP 会话的期限 |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | 邮件存储目录 |
| `-mail-retention-days` | `OWLMAIL_MAIL_RETENTION_DAYS` | 0 | 删除超过 N 天的邮件；`0` 表示不限 |
| `-mail-max-messages` | `OWLMAIL_MAIL_MAX_MESSAGES` | 0 | 最大邮件封数；`0` 表示不限 |
| `-mail-max-disk-mb` | `OWLMAIL_MAIL_MAX_DISK_MB` | 0 | 邮箱最大磁盘 MiB；`0` 表示不限 |
| `-mail-cleanup-interval` | `OWLMAIL_MAIL_CLEANUP_INTERVAL` | 1h | 后台保留策略清理间隔 |
| `-mail-index-path` | `OWLMAIL_MAIL_INDEX_PATH` | - | 可选的 SQLite 邮箱查询索引路径；索引可重建，EML 文件仍是权威数据源 |
| `-s3-enabled` | `OWLMAIL_S3_ENABLED` | false | 将解析后的附件存入 S3 兼容对象存储 |
| `-s3-endpoint` | `OWLMAIL_S3_ENDPOINT` | - | 自定义 S3 兼容端点；留空使用 AWS S3 |
| `-s3-region` | `OWLMAIL_S3_REGION` | us-east-1 | S3 签名区域 |
| `-s3-bucket` | `OWLMAIL_S3_BUCKET` | - | 用于附件的已有存储桶 |
| `-s3-prefix` | `OWLMAIL_S3_PREFIX` | owlmail/attachments | 附件对象键前缀 |
| `-s3-access-key` | `OWLMAIL_S3_ACCESS_KEY` | - | 可选静态访问密钥；留空使用 AWS 凭据链 |
| `-s3-secret-key` | `OWLMAIL_S3_SECRET_KEY` | - | 可选静态秘密密钥 |
| `-s3-session-token` | `OWLMAIL_S3_SESSION_TOKEN` | - | 可选静态凭据会话令牌 |
| `-s3-use-path-style` | `OWLMAIL_S3_USE_PATH_STYLE` | false | 为兼容服务使用路径式存储桶寻址 |
| `-s3-startup-check` | `OWLMAIL_S3_STARTUP_CHECK` | false | 首次只读 S3 存储桶探测失败时终止启动 |
| `-s3-health-check-interval` | `OWLMAIL_S3_HEALTH_CHECK_INTERVAL` | 30s | 后台刷新 S3 readiness 的间隔 |
| `-s3-health-check-timeout` | `OWLMAIL_S3_HEALTH_CHECK_TIMEOUT` | 5s | 单次 S3 readiness 探测超时 |
| `-web-user` | `MAILDEV_WEB_USER` / `OWLMAIL_WEB_USER` | - | HTTP Basic Auth 用户名 |
| `-web-password` | `MAILDEV_WEB_PASS` / `OWLMAIL_WEB_PASSWORD` | - | HTTP Basic Auth 密码 |
| `-https` | `MAILDEV_HTTPS` / `OWLMAIL_HTTPS_ENABLED` | false | 启用 HTTPS |
| `-https-cert` | `MAILDEV_HTTPS_CERT` / `OWLMAIL_HTTPS_CERT` | - | HTTPS 证书文件 |
| `-https-key` | `MAILDEV_HTTPS_KEY` / `OWLMAIL_HTTPS_KEY` | - | HTTPS 私钥文件 |
| `-outgoing-host` | `MAILDEV_OUTGOING_HOST` / `OWLMAIL_OUTGOING_HOST` | - | 出站 SMTP 主机 |
| `-outgoing-port` | `MAILDEV_OUTGOING_PORT` / `OWLMAIL_OUTGOING_PORT` | 587 | 出站 SMTP 端口 |
| `-outgoing-user` | `MAILDEV_OUTGOING_USER` / `OWLMAIL_OUTGOING_USER` | - | 出站 SMTP 用户名 |
| `-outgoing-pass` | `MAILDEV_OUTGOING_PASS` / `OWLMAIL_OUTGOING_PASSWORD` | - | 出站 SMTP 密码 |
| `-outgoing-secure` | `MAILDEV_OUTGOING_SECURE` / `OWLMAIL_OUTGOING_SECURE` | false | MailDev 兼容的隐式 TLS/SMTPS 开关 |
| `-outgoing-tls-mode` | `OWLMAIL_OUTGOING_TLS_MODE` | - | 传输模式：未设置时为 `plain`，也可选择强制 `starttls` 或隐式 `smtps` |
| `-outgoing-insecure-skip-verify` | `OWLMAIL_OUTGOING_INSECURE_SKIP_VERIFY` | false | 跳过证书和主机名验证（不安全，必须显式开启） |
| `-outgoing-connect-timeout` | `OWLMAIL_OUTGOING_CONNECT_TIMEOUT` | 10s | TCP 连接和 SMTP 问候超时 |
| `-outgoing-tls-handshake-timeout` | `OWLMAIL_OUTGOING_TLS_HANDSHAKE_TIMEOUT` | 10s | TLS/STARTTLS 握手超时 |
| `-outgoing-auth-timeout` | `OWLMAIL_OUTGOING_AUTH_TIMEOUT` | 10s | AUTH 超时 |
| `-outgoing-envelope-timeout` | `OWLMAIL_OUTGOING_ENVELOPE_TIMEOUT` | 10s | MAIL/RCPT 超时 |
| `-outgoing-data-timeout` | `OWLMAIL_OUTGOING_DATA_TIMEOUT` | 30s | DATA 写入和确认超时 |
| `-outgoing-quit-timeout` | `OWLMAIL_OUTGOING_QUIT_TIMEOUT` | 5s | QUIT 超时 |
| `-auto-relay` | `MAILDEV_AUTO_RELAY` / `OWLMAIL_AUTO_RELAY` | false | 启用自动中继 |
| `-auto-relay-addr` | `MAILDEV_AUTO_RELAY_ADDR` / `OWLMAIL_AUTO_RELAY_ADDR` | - | 自动中继地址 |
| `-auto-relay-rules` | `MAILDEV_AUTO_RELAY_RULES` / `OWLMAIL_AUTO_RELAY_RULES` | - | 自动中继规则文件 |
| `-webhook-config` | `OWLMAIL_WEBHOOK_CONFIG` | - | Webhook 消息转发 JSON 配置文件 |
| `-webhook-max-concurrency` | `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` | 8 | Webhook 邮件并发投递数；`0` 表示不限制 |
| `-webhook-redis-url` | `OWLMAIL_WEBHOOK_REDIS_URL` | - | 用于持久、可跨重启投递的 Redis URL |
| `-webhook-redis-prefix` | `OWLMAIL_WEBHOOK_REDIS_PREFIX` | owlmail:webhooks | Redis Streams 键前缀 |
| `-webhook-shutdown-timeout` | `OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT` | 15s | Webhook 优雅排空截止时间 |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | 入站 SMTP 用户名；与密码同时配置后强制 AUTH |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | 入站 SMTP 密码；与用户名同时配置后强制 AUTH |
| `-smtp-auth-require-tls` | `OWLMAIL_SMTP_AUTH_REQUIRE_TLS` | false | TLS 建立前拒绝 PLAIN/LOGIN；必须同时启用 SMTP TLS |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | 启用 SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS 证书文件 |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS 私钥文件 |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | 日志级别 |
| `-mailcatcher-rest-compat` | `OWLMAIL_MAILCATCHER_REST_COMPAT` | false | 启用可选的 MailCatcher REST 兼容接口 |
| `-config` | `OWLMAIL_CONFIG_FILE` | - | 启动时读取 YAML 或 JSON 配置文件；命令行和环境变量优先 |
| `-log-format` | `OWLMAIL_LOG_FORMAT` | console | 日志输出格式：`console` 或 `json` |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | 使用 UUID 作为邮件 ID（默认使用 8 字符随机字符串） |

若 TLS 在反向代理终止，请将 `OWLMAIL_WEB_EXTERNAL_SCHEME` 设置为 `https`。

启用 HTTP Basic Auth 后，浏览器 API 与 WebSocket 请求仅允许来自 OwlMail
自身源。命令行和服务端客户端不携带浏览器 `Origin` 请求头时仍可正常访问。

只配置一项 Web 认证凭据时，OwlMail 也会安全补全，而不会静默关闭认证：

| 已配置内容 | 最终凭据 |
|---|---|
| 用户名和密码均未配置 | 关闭认证 |
| 只配置用户名 | 保留用户名，生成 32 字符密码，并在启动时向 stderr 输出一次 |
| 只配置密码 | 使用默认用户名 `admin` 和已配置密码 |
| 用户名和密码均已配置 | 原样使用 |

自动生成的密码会在每次重启后变化。可从进程输出中读取（容器示例使用
`docker logs owlmail`），需要稳定凭据时应同时配置用户名和密码；如果无法将
自动生成的密码写入 stderr，OwlMail 会启动失败。Basic Auth 只应在 localhost
或 HTTPS 上使用。

### 只读 MCP

MCP 默认关闭。使用 `-mcp-enabled` 或 `OWLMAIL_MCP_ENABLED=true` 启用官方
Streamable HTTP 端点，然后连接 `http://localhost:1080/mcp`。若设置
`-base-pathname /owlmail`，端点随之变为 `/owlmail/mcp`。MCP 与 Web 共用监听器、
HTTPS 配置和 HTTP Basic Auth；因此 Web 已启用认证时，每个 MCP 请求也必须携带
同一组 Basic Auth 凭据。

服务提供七个只读工具：在原有轻量查询、深复制详情、有界 source 和附件元数据
能力上，增加 `get_latest_email` 与事件驱动的 `wait_for_email`。同时提供有界的
`owlmail://inbox`、`owlmail://stats`、`owlmail://email/{id}` Resources，以及
注册验证、密码重置和等待投递 Prompts。邮件结果包含 Web UI 深链接；反向代理
部署可设置 `-web-external-url https://mail.example.com`（或
`OWLMAIL_WEB_EXTERNAL_URL`），并使用 `-base-pathname` 单独配置子路径。任何工具
都不会返回附件字节，只有显式调用 `get_email(include_html=true)` 才返回 HTML。
完整安全及会话生命周期说明见[运维文档](./docs/zh-CN/Operations.md#供测试代理使用的只读-mcp)。

### 环境变量兼容性

OwlMail 支持上表列出的 MailDev 环境变量别名，并优先于对应的 `OWLMAIL_*`
变量；未列出的 MailDev 选项不会自动生效。

```bash
# 直接使用 MailDev 环境变量（推荐）
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com
./owlmail

# 或使用 OwlMail 环境变量
export OWLMAIL_SMTP_PORT=1025
export OWLMAIL_WEB_PORT=1080
./owlmail
```

### S3 兼容附件存储

S3 默认关闭，解析后的附件继续存放在 `-mail-directory` 下。开启 S3 后，只有解析
后的附件进入对象存储；原始 `.eml`、元数据、事务标记及 Webhook outbox 仍保存在
本地，因此生产环境仍需持久化邮件目录。

```bash
export OWLMAIL_S3_ENABLED=true
export OWLMAIL_S3_ENDPOINT=http://minio:9000
export OWLMAIL_S3_REGION=us-east-1
export OWLMAIL_S3_BUCKET=owlmail
export OWLMAIL_S3_PREFIX=owlmail/attachments
export OWLMAIL_S3_ACCESS_KEY=replace-me
export OWLMAIL_S3_SECRET_KEY=replace-me
export OWLMAIL_S3_USE_PATH_STYLE=true
# 可选：首次 HeadBucket 失败时终止启动
export OWLMAIL_S3_STARTUP_CHECK=true
./owlmail -mail-directory ./owlmail-data
```

存储桶需要预先创建。留空 endpoint 时使用 AWS S3；不设置 OwlMail 静态密钥时，
使用 AWS SDK 默认凭据链，可直接使用工作负载角色。附件对象键格式为
`<prefix>/<email-id>/<generated-filename>`；删除邮件和执行保留策略时，会同步删除
该邮件对应的对象前缀。只有附件上传完成后 SMTP 才会接受本次邮件事务。
`OWLMAIL_MAIL_MAX_DISK_MB` 只统计本地文件，不包含 S3 对象占用。

OwlMail 优先使用只读 `HeadBucket` 探测 S3；若最小权限策略不允许 bucket 级探测，
则回退到最多读取一个键、限定附件前缀的 `ListObjectsV2`。默认异步执行首次探测，以保持既有启动行为；
在探测成功前，`GET /readyz` 与 `GET /api/v1/ready` 返回 `503`。设置
`OWLMAIL_S3_STARTUP_CHECK=true` 后，仅首次探测失败会终止启动。运行期间 S3
暂时不可用只会让 readiness 失败，不会使进程退出；后台探测恢复后 readiness
自动恢复。readiness 请求只读取缓存，不会同步等待 S3。`/healthz` 和
`/api/v1/health` 始终用于进程 liveness。

## 📡 API 文档

### API 响应格式

OwlMail 使用标准化的 API 响应格式：

**成功响应：**
```json
{
  "code": "EMAIL_DELETED",
  "message": "Email deleted",
  "data": { ... }
}
```

**错误响应：**
```json
{
  "code": "EMAIL_NOT_FOUND",
  "error": "EMAIL_NOT_FOUND",
  "message": "Email not found"
}
```

`code` 字段包含标准化的错误/成功代码，可用于国际化。`message` 字段提供英文文本以保持向后兼容。
Basic Auth 与浏览器同源中间件错误发生在进入 API 处理器之前，因此会返回纯文本
`401` 或 `403`。

### 邮件 ID 格式

OwlMail 支持两种邮件 ID 格式，所有 API 端点都兼容这两种格式：

- **8 字符随机字符串**：默认格式，例如 `aB3dEfGh`
- **UUID 格式**：36 字符标准 UUID，例如 `550e8400-e29b-41d4-a716-446655440000`

在 API 请求中使用 `:id` 参数时，可以使用任意一种格式。例如：
- `GET /email/aB3dEfGh` - 使用随机字符串 ID
- `GET /email/550e8400-e29b-41d4-a716-446655440000` - 使用 UUID ID

### 可选 MailDev REST facade

现有 MailDev REST 客户端可显式启用其 `/api` 路由和载荷合约：

```bash
owlmail -maildev-rest-compat
# 或：OWLMAIL_MAILDEV_REST_COMPAT=true owlmail
```

启用后提供 `/api/email`、`/api/email/summary`、`/api/email/delete`、详情、
HTML、source、EML 下载、附件、relay、`/api/config`、`/api/healthz` 和
`/api/reloadMailsFromDirectory`。facade 复用 Basic Auth、HTTPS 与 base path；
health 与 MailDev 一样公开。只有 `GET /api/email/:id` 会将邮件标记已读。
关闭选项时所有新增 `/api` 路由均不存在。这里**不实现 Socket.IO**，实时客户端
仍需迁移到 OwlMail 原生 WebSocket 协议。详见
[API 参考](./docs/zh-CN/API-Reference.md#可选-maildev-rest-facade)。

### MailDev 风格兼容路由

OwlMail 保留无版本路由以覆盖常见 MailDev 风格工作流，但它们不是当前 MailDev
API 的精确等价实现；差异见 [API 参考](./docs/zh-CN/API-Reference.md#maildev-迁移边界)。

#### 邮件操作

- `GET /email` - 获取所有邮件（支持分页和过滤）
  - 查询参数：
    - `limit` (默认: 50, 最大: 1000) - 返回邮件数量
    - `offset` (默认: 0) - 跳过的邮件数量
    - `q` - 全文搜索查询
    - `from` - 按发件人邮箱地址过滤
    - `to` - 按收件人邮箱地址过滤
    - `dateFrom` - 按起始日期过滤（YYYY-MM-DD 格式）
    - `dateTo` - 按结束日期过滤（YYYY-MM-DD 格式）
    - `read` - 按已读状态过滤（true/false）
    - `sortBy` - 排序字段（time、subject、from、size）
    - `sortOrder` - 排序顺序（asc, desc，默认: desc）
  - 示例：`GET /email?limit=20&offset=0&q=test&sortBy=time&sortOrder=desc`
- `GET /email/:id` - 获取单个邮件
- `DELETE /email/:id` - 删除单个邮件
- `DELETE /email/all` - 删除所有邮件
- `PATCH /email/read-all` - 标记所有邮件为已读
- `PATCH /email/:id/read` - 标记单个邮件为已读

#### 邮件内容

- `GET /email/:id/html` - 获取邮件 HTML 内容
- `GET /email/:id/attachment/:filename` - 下载附件
- `GET /email/:id/download` - 下载原始 .eml 文件
- `GET /email/:id/source` - 获取邮件原始源码

#### 邮件转发

- `POST /email/:id/relay` - 转发邮件到配置的 SMTP 服务器
- `POST /email/:id/relay/:relayTo` - 转发邮件到指定地址

#### 配置和系统

- `GET /config` - 获取配置信息
- `GET /healthz` - 进程存活检查
- `GET /readyz` - 缓存的依赖 readiness 检查
- `GET /reloadMailsFromDirectory` - 重新加载邮件目录
- `GET /socket.io` - WebSocket 连接（标准 WebSocket，非 Socket.IO）

### OwlMail 增强 API

#### 邮件统计和预览

- `GET /email/stats` - 获取邮件统计信息
- `GET /email/preview` - 获取邮件预览（轻量级）

#### 批量操作

- `POST /email/batch/delete` - 批量删除邮件
- `POST /email/batch/read` - 批量标记已读

#### 邮件导出

- `GET /email/export` - 导出邮件为 ZIP 文件

#### 配置管理

- `GET /config/outgoing` - 获取出站配置
- `PUT /config/outgoing` - 更新出站配置
- `PATCH /config/outgoing` - 部分更新出站配置

### 改进的 RESTful API (`/api/v1/*`)

OwlMail 提供了更规范的 RESTful API 设计：

- `GET /api/v1/emails` - 获取所有邮件（复数资源）
  - 查询参数：与 `GET /email` 相同（limit, offset, q, from, to, dateFrom, dateTo, read, sortBy, sortOrder）
  - 示例：`GET /api/v1/emails?limit=20&offset=0&q=test&sortBy=time&sortOrder=desc`
- `GET /api/v1/emails/:id` - 获取单个邮件
- `DELETE /api/v1/emails/:id` - 删除单个邮件
- `DELETE /api/v1/emails` - 删除所有邮件
- `DELETE /api/v1/emails/batch` - 批量删除
- `PATCH /api/v1/emails/read` - 标记所有邮件为已读
- `PATCH /api/v1/emails/:id/read` - 标记单个邮件为已读
- `PATCH /api/v1/emails/batch/read` - 批量标记已读
- `GET /api/v1/emails/stats` - 邮件统计
- `GET /api/v1/emails/preview` - 邮件预览
- `GET /api/v1/emails/export` - 导出邮件
- `POST /api/v1/emails/reload` - 重新加载邮件
- `GET /api/v1/settings` - 获取所有设置
- `GET /api/v1/settings/outgoing` - 获取出站配置
- `PUT /api/v1/settings/outgoing` - 更新出站配置
- `PATCH /api/v1/settings/outgoing` - 部分更新出站配置
- `GET /api/v1/health` - 进程存活检查
- `GET /api/v1/ready` - 缓存的依赖 readiness 检查
- `GET /api/v1/version` - 版本信息
- `GET /api/v1/ws` - WebSocket 连接
- `GET /api/v1/openapi.json` - OpenAPI 3.1 合约（JSON）
- `GET /api/v1/openapi.yaml` - OpenAPI 3.1 合约（YAML）

完整的子资源、鉴权、响应结构和 WebSocket 事件见
[API 参考](./docs/zh-CN/API-Reference.md)或版本控制中的
[OpenAPI 合约](./openapi/openapi.yaml)。服务端返回的合约会自动包含配置的
base pathname。

## 🔧 使用示例

### 基本使用

```bash
# 启动 OwlMail
./owlmail -smtp 1025 -web 1080

# 在应用中配置 SMTP
SMTP_HOST=localhost
SMTP_PORT=1025
```

### 配置邮件转发

```bash
# 转发到 Gmail SMTP
./owlmail \
  -outgoing-host smtp.gmail.com \
  -outgoing-port 587 \
  -outgoing-user your-email@gmail.com \
  -outgoing-pass your-password \
  -outgoing-tls-mode starttls
```

`starttls` 模式在服务端未声明 STARTTLS、握手失败或证书/主机名验证失败时
直接终止，不会降级到明文。隐式 TLS（通常为 465 端口）请使用 `smtps`，或使用
兼容 MailDev 的旧参数 `-outgoing-secure`。`plain` 模式禁止出站 AUTH。

### 自动中继模式

```bash
# 创建自动中继规则文件 (relay-rules.json)
cat > relay-rules.json <<EOF
[
  { "allow": "*" },
  { "deny": "*@test.com" },
  { "allow": "ok@test.com" }
]
EOF

# 启动自动中继
./owlmail \
  -outgoing-host smtp.gmail.com \
  -outgoing-port 587 \
  -outgoing-user your-email@gmail.com \
  -outgoing-pass your-password \
  -auto-relay \
  -auto-relay-rules relay-rules.json
```

### Webhook 消息转发

可在 `http://localhost:1080/webhooks` 使用内置配置器生成版本 1 配置，或导入并
校验已有配置。编辑过程只在浏览器本地完成。下载 JSON 后，需要使用
`-webhook-config` 指定文件并重启 OwlMail；仅下载文件不会自动启用规则。

```bash
# 终端 1：启动本地测试接收器
go run ./examples/webhooks/receiver

# 终端 2：使用默认 JSON 请求体转发所有新邮件
./owlmail -webhook-config ./examples/webhooks/minimal.json
```

Webhook 目标支持不区分大小写的通配规则、JSON 安全的自定义请求体模板、环境变量密钥、HMAC-SHA256 签名、超时和有限重试。[场景示例](./examples/webhooks/README.zh-CN.md)覆盖过滤、自定义 API、多目标、纯文本和可直接运行的 `soulteary/webhook` 联动；完整参考见 [Webhook 消息转发指南](./docs/zh-CN/Webhook-Forwarding.md)。

### 使用 HTTPS

```bash
./owlmail \
  -https \
  -https-cert /path/to/cert.pem \
  -https-key /path/to/key.pem \
  -web 1080
```

### 入站 SMTP 认证模式

不配置 `-smtp-user` 和 `-smtp-password` 时，OwlMail 默认使用 **NO AUTH**
模式：既允许客户端不认证直接投递，也会声明支持 PLAIN/LOGIN，并接受任意凭据，
方便对接那些必须填写 SMTP 凭据、但本地开发和测试时不希望额外配置服务的应用。

同时配置两项后启用真正的强制 SMTP AUTH。未认证事务会收到 `530 5.7.0`，错误
凭据会收到 `535 5.7.8`。只配置其中一项时启动失败，不会静默回退到 NO AUTH。

若希望保留默认开发体验，同时避免凭据经过明文连接，可在启用 SMTP TLS 的同时
设置 `-smtp-auth-require-tls`（或 `OWLMAIL_SMTP_AUTH_REQUIRE_TLS=true`）。此时
明文 SMTP 既不会声明也不会接受 PLAIN/LOGIN；STARTTLS 后及 SMTPS 连接仍可正常
认证。NO AUTH 模式下的匿名投递不受影响。开启此选项但没有启用且可用的 SMTP TLS
配置时，服务会直接启动失败。

> [!WARNING]
> NO AUTH 有意不提供访问控制；为兼容开发环境，PLAIN/LOGIN 也允许在未启用 TLS
> 时使用。请只在本机或可信网络使用 NO AUTH，使用真实凭据前应启用 TLS。

### 使用 TLS

```bash
./owlmail \
  -tls \
  -tls-cert /path/to/cert.pem \
  -tls-key /path/to/key.pem \
  -smtp 1025
```

**注意**：启用 TLS 时，OwlMail 会在常规 SMTP 服务器之外自动监听 465 端口提供 SMTPS。SMTPS 使用直接 TLS 连接（无需 STARTTLS）。

### 使用 UUID 作为邮件 ID

OwlMail 支持两种邮件 ID 格式：

1. **默认格式**：8 字符随机字符串（例如：`aB3dEfGh`）
2. **UUID 格式**：36 字符标准 UUID（例如：`550e8400-e29b-41d4-a716-446655440000`）

使用 UUID 格式可以提供更好的唯一性和可追溯性，特别适合需要与外部系统集成的场景。

```bash
# 使用命令行参数启用 UUID
./owlmail -use-uuid-for-email-id

# 使用环境变量启用 UUID
export OWLMAIL_USE_UUID_FOR_EMAIL_ID=true
./owlmail

# 结合其他配置使用
./owlmail \
  -use-uuid-for-email-id \
  -smtp 1025 \
  -web 1080
```

**注意事项**：
- 默认使用 8 字符随机字符串，兼容 MailDev 的行为
- 启用 UUID 后，所有新接收的邮件将使用 UUID 格式的 ID
- API 同时支持两种格式的 ID，可以正常查询、删除和操作邮件
- 已存在的邮件 ID 格式不会改变，只有新邮件会使用新的 ID 格式

## 🔄 从 MailDev 迁移

OwlMail 覆盖常见 MailDev 工作流，但当前 MailDev 客户端可能需要少量明确适配。
请遵循[迁移指南](./docs/zh-CN/Comparison-and-Migration.md)。

### 1. 环境变量兼容

OwlMail 接受配置表中列出的 MailDev 环境变量；请逐项核对部署实际使用的变量：

```bash
# MailDev 配置
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# 上述已列出的变量也可被 OwlMail 读取
./owlmail
```

### 2. API 兼容

现有 REST 客户端可显式启用默认关闭的 MailDev facade；新集成应使用 OwlMail
版本化 API。该 facade 不提供 Socket.IO 兼容：

```bash
# 现有 MailDev REST 客户端
OWLMAIL_MAILDEV_REST_COMPAT=true ./owlmail
curl http://localhost:1080/api/email

# 新 OwlMail 集成
curl http://localhost:1080/api/v1/emails
```

### 3. WebSocket 适配

如果使用 WebSocket，需要从 Socket.IO 改为标准 WebSocket：

```javascript
// MailDev (Socket.IO)
const socket = io('/socket.io');
socket.on('newMail', (email) => { /* ... */ });

// OwlMail (标准 WebSocket)
const ws = new WebSocket('ws://localhost:1080/socket.io');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'new') { /* ... */ }
};
```

详细迁移指南请参考：[OwlMail × MailDev：功能与 API 完整对比与迁移白皮书](./docs/zh-CN/Comparison-and-Migration.md)

## 🧪 测试

```bash
# 运行所有测试
go test ./...

# 运行测试并查看覆盖率
go test -cover ./...

# 运行特定包的测试
go test ./internal/api/...
go test ./internal/mailserver/...
```

## 📦 项目结构

```
OwlMail/
├── cmd/
│   └── owlmail/          # 主程序入口
├── internal/
│   ├── api/              # Web API 实现
│   ├── common/           # 通用工具（日志、错误处理）
│   ├── maildev/          # MailDev 兼容层
│   ├── mailserver/       # SMTP 服务器实现
│   ├── outgoing/         # 邮件转发实现
│   ├── types/            # 类型定义
│   └── webhook/          # Webhook 过滤、模板、签名与投递
├── docs/                 # API、运维、Webhook 与迁移文档
├── examples/             # 可运行的集成示例
├── tests/                # 浏览器与文档契约测试
├── web/                  # 嵌入式 Web 前端和本地帮助资源
├── go.mod                # Go 模块定义
└── README.md             # 本文档
```

## 🤝 贡献

欢迎贡献！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙏 致谢

- [MailDev](https://github.com/maildev/maildev) - 原始项目灵感
- [emersion/go-smtp](https://github.com/emersion/go-smtp) - SMTP 服务器库
- [emersion/go-message](https://github.com/emersion/go-message) - 邮件解析库
- [Fiber](https://github.com/gofiber/fiber) - Web 框架
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket 库

## 📚 相关文档

- [OwlMail 0.8.0 发布说明](./docs/zh-CN/Release-0.8.0.md)
- [集成测试](./docs/zh-CN/Integration-Testing.md)与 [CI 快速入门](./docs/zh-CN/CI-Quickstart.md)
- [AI Agent 测试](./docs/zh-CN/AI-Agent-Testing.md)与 [MCP 参考](./docs/zh-CN/MCP-Reference.md)
- [测试配方与可运行示例](./docs/zh-CN/Testing-Recipes.md)
- [架构](./docs/zh-CN/Architecture.md)与[安全模型](./docs/zh-CN/Security-Model.md)
- [OwlMail 0.7.0 发布说明](./docs/zh-CN/Release-0.7.0.md)
- [变更日志](./CHANGELOG.md)
- [OwlMail × MailDev：功能与 API 完整对比与迁移白皮书](./docs/zh-CN/Comparison-and-Migration.md)
- [API 参考](./docs/zh-CN/API-Reference.md)
- [运维与排障](./docs/zh-CN/Operations.md)
- [Webhook 消息转发](./docs/zh-CN/Webhook-Forwarding.md)
- [可运行 Webhook 场景](./examples/webhooks/README.zh-CN.md)
- [发布流程（维护者）](./docs/zh-CN/Releasing.md)
- [API 重构记录（历史资料）](./docs/zh-CN/internal/API_Refactoring_Record.md)

## 🐛 问题反馈

如果遇到问题或有建议，请在 [GitHub Issues](https://github.com/soulteary/owlmail/issues) 中提交。

## ⭐ Star History

如果这个项目对你有帮助，请给一个 Star ⭐！

---

**OwlMail** —— 为开发者、CI、自动化系统与 AI Agent 提供统一的自托管邮件测试网关。🦉
