# OwlMail

> 🦉 一个用 Go 语言实现的邮件开发测试工具，完全兼容 MailDev，提供更好的性能和更丰富的功能

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Compatible](https://img.shields.io/badge/MailDev-Compatible-blue.svg)](https://github.com/maildev/maildev)
[![Go Report Card](https://goreportcard.com/badge/github.com/soulteary/owlmail)](https://goreportcard.com/report/github.com/soulteary/owlmail)
[![codecov](https://codecov.io/gh/soulteary/owlmail/graph/badge.svg?token=AY59NGM1FV)](https://codecov.io/gh/soulteary/owlmail)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](README.md) | [简体中文](README.zh-CN.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Italiano](README.it.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

---

OwlMail 是一个用于开发和测试环境的 SMTP 服务器和 Web 界面，可以捕获和查看所有发送的邮件。它是 [MailDev](https://github.com/maildev/maildev) 的 Go 语言实现，提供 100% API 兼容性，同时带来更好的性能、更低的资源占用和更丰富的功能。

![](.github/assets/owlmail-banner.jpg)

## ✨ 特性

### 核心功能

- ✅ **SMTP 服务器** - 接收和存储所有发送的邮件（默认端口 1025）
- ✅ **Web 界面** - 通过浏览器查看和管理邮件（默认端口 1080）
- ✅ **邮件持久化** - 邮件保存为 `.eml` 文件，支持从目录加载
- ✅ **邮件转发** - 支持将邮件转发到真实的 SMTP 服务器
- ✅ **自动中继** - 支持自动转发所有邮件，带规则过滤
- ✅ **SMTP 认证** - 支持 PLAIN/LOGIN 认证
- ✅ **TLS/STARTTLS** - 支持加密连接
- ✅ **SMTPS** - 支持端口 465 的直接 TLS 连接（OwlMail 独有）

### 增强功能

- 🆕 **批量操作** - 批量删除、批量标记已读
- 🆕 **邮件统计** - 获取邮件统计信息
- 🆕 **邮件预览** - 轻量级邮件预览 API
- 🆕 **邮件导出** - 导出邮件为 ZIP 文件
- 🆕 **配置管理 API** - 完整的配置管理（GET/PUT/PATCH）
- 🆕 **强大的搜索** - 全文搜索、日期范围过滤、排序
- 🆕 **改进的 RESTful API** - 更规范的 API 设计（`/api/v1/*`）

### 兼容性

- ✅ **100% MailDev API 兼容** - 所有 MailDev API 端点都得到支持
- ✅ **环境变量完全兼容** - 优先使用 MailDev 环境变量，无需修改配置
- ✅ **自动中继规则兼容** - JSON 配置文件格式完全兼容

### 性能优势

- ⚡ **单一二进制** - 编译为单一可执行文件，无需运行时
- ⚡ **低资源占用** - Go 语言编译，内存占用更低
- ⚡ **快速启动** - 启动速度更快
- ⚡ **高并发** - Go 协程，并发性能更好

## 🚀 快速开始

### 安装

#### 从源码编译

```bash
# 克隆仓库
git clone https://github.com/soulteary/owlmail.git
cd owlmail

# 编译
go build -o owlmail ./cmd/owlmail

# 运行
./owlmail
```

#### 使用 Go 安装

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@latest
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

### Docker 使用

#### 从 GitHub Container Registry 拉取镜像（推荐）

使用 OwlMail 最简单的方式是从 GitHub Container Registry 拉取预构建的镜像：

```bash
# 拉取最新镜像
docker pull ghcr.io/soulteary/owlmail:latest

# 拉取特定版本（使用提交 SHA）
docker pull ghcr.io/soulteary/owlmail:sha-49b5f35

# 运行容器
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  ghcr.io/soulteary/owlmail:latest
```

**可用标签：**
- `latest` - 最新稳定版本
- `sha-<commit>` - 特定提交 SHA（例如：`sha-49b5f35`）
- `main` - main 分支的最新版本

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
  -p 1025:1025 \
  -p 1080:1080 \
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

## 📖 配置选项

### 命令行参数

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP 端口 |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP 主机 |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web API 端口 |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web API 主机 |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | 邮件存储目录 |
| `-web-user` | `MAILDEV_WEB_USER` / `OWLMAIL_WEB_USER` | - | HTTP Basic Auth 用户名 |
| `-web-password` | `MAILDEV_WEB_PASS` / `OWLMAIL_WEB_PASSWORD` | - | HTTP Basic Auth 密码 |
| `-https` | `MAILDEV_HTTPS` / `OWLMAIL_HTTPS_ENABLED` | false | 启用 HTTPS |
| `-https-cert` | `MAILDEV_HTTPS_CERT` / `OWLMAIL_HTTPS_CERT` | - | HTTPS 证书文件 |
| `-https-key` | `MAILDEV_HTTPS_KEY` / `OWLMAIL_HTTPS_KEY` | - | HTTPS 私钥文件 |
| `-outgoing-host` | `MAILDEV_OUTGOING_HOST` / `OWLMAIL_OUTGOING_HOST` | - | 出站 SMTP 主机 |
| `-outgoing-port` | `MAILDEV_OUTGOING_PORT` / `OWLMAIL_OUTGOING_PORT` | 587 | 出站 SMTP 端口 |
| `-outgoing-user` | `MAILDEV_OUTGOING_USER` / `OWLMAIL_OUTGOING_USER` | - | 出站 SMTP 用户名 |
| `-outgoing-pass` | `MAILDEV_OUTGOING_PASS` / `OWLMAIL_OUTGOING_PASSWORD` | - | 出站 SMTP 密码 |
| `-outgoing-secure` | `MAILDEV_OUTGOING_SECURE` / `OWLMAIL_OUTGOING_SECURE` | false | 出站 SMTP TLS |
| `-auto-relay` | `MAILDEV_AUTO_RELAY` / `OWLMAIL_AUTO_RELAY` | false | 启用自动中继 |
| `-auto-relay-addr` | `MAILDEV_AUTO_RELAY_ADDR` / `OWLMAIL_AUTO_RELAY_ADDR` | - | 自动中继地址 |
| `-auto-relay-rules` | `MAILDEV_AUTO_RELAY_RULES` / `OWLMAIL_AUTO_RELAY_RULES` | - | 自动中继规则文件 |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | SMTP 认证用户名 |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | SMTP 认证密码 |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | 启用 SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS 证书文件 |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS 私钥文件 |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | 日志级别 |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | 使用 UUID 作为邮件 ID（默认使用 8 字符随机字符串） |

### 环境变量兼容性

OwlMail **完全支持 MailDev 环境变量**，优先使用 MailDev 环境变量，如果不存在则使用 OwlMail 环境变量。这意味着你可以直接使用 MailDev 的配置，无需修改。

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

### 邮件 ID 格式

OwlMail 支持两种邮件 ID 格式，所有 API 端点都兼容这两种格式：

- **8 字符随机字符串**：默认格式，例如 `aB3dEfGh`
- **UUID 格式**：36 字符标准 UUID，例如 `550e8400-e29b-41d4-a716-446655440000`

在 API 请求中使用 `:id` 参数时，可以使用任意一种格式。例如：
- `GET /email/aB3dEfGh` - 使用随机字符串 ID
- `GET /email/550e8400-e29b-41d4-a716-446655440000` - 使用 UUID ID

### MailDev 兼容 API

OwlMail 完全兼容 MailDev 的所有 API 端点：

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
    - `sortBy` - 排序字段（time, subject）
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
- `GET /healthz` - 健康检查
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
- `GET /api/v1/health` - 健康检查
- `GET /api/v1/ws` - WebSocket 连接

详细 API 文档请参考：[API 重构记录](./docs/zh-CN/internal/API_Refactoring_Record.md)

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
  -outgoing-secure
```

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

### 使用 HTTPS

```bash
./owlmail \
  -https \
  -https-cert /path/to/cert.pem \
  -https-key /path/to/key.pem \
  -web 1080
```

### 使用 SMTP 认证

```bash
./owlmail \
  -smtp-user admin \
  -smtp-password secret \
  -smtp 1025
```

### 使用 TLS

```bash
./owlmail \
  -tls \
  -tls-cert /path/to/cert.pem \
  -tls-key /path/to/key.pem \
  -smtp 1025
```

**注意**：启用 TLS 时，OwlMail 会自动在 465 端口启动 SMTPS 服务器，除了常规 SMTP 服务器外。SMTPS 服务器使用直接 TLS 连接（无需 STARTTLS）。这是 OwlMail 的独有功能。

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

OwlMail 完全兼容 MailDev，可以无缝替换：

### 1. 环境变量兼容

OwlMail 优先使用 MailDev 环境变量，无需修改配置：

```bash
# MailDev 配置
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# 直接使用 OwlMail（无需修改环境变量）
./owlmail
```

### 2. API 兼容

所有 MailDev API 端点都得到支持，现有客户端代码无需修改：

```bash
# MailDev API
curl http://localhost:1080/email

# OwlMail 完全兼容
curl http://localhost:1080/email
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

详细迁移指南请参考：[OwlMail × MailDev：功能与 API 完整对比与迁移白皮书](./docs/zh-CN/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

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
│   └── types/            # 类型定义
├── web/                  # Web 前端文件
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
- [Gin](https://github.com/gin-gonic/gin) - Web 框架
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket 库

## 📚 相关文档

- [OwlMail × MailDev：功能与 API 完整对比与迁移白皮书](./docs/zh-CN/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [API 重构记录](./docs/zh-CN/internal/API_Refactoring_Record.md)

## 🐛 问题反馈

如果遇到问题或有建议，请在 [GitHub Issues](https://github.com/soulteary/owlmail/issues) 中提交。

## ⭐ Star History

如果这个项目对你有帮助，请给一个 Star ⭐！

---

**OwlMail** - 用 Go 语言实现的邮件开发测试工具，完全兼容 MailDev 🦉

