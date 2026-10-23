# OwlMail

在本地捕获应用邮件，检查内容、链接与附件，并在 CI 和 AI Agent 中验证发信流程。

[![Go Test Coverage](.github/coverage.svg)](.github/go-test-report.md) [![Go Report Card](.github/goreportcard.svg)](.github/goreportcard-report.md) [![Release](https://img.shields.io/github/v/release/soulteary/owlmail)](https://github.com/soulteary/owlmail/releases) [![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) · [简体中文](README.zh-CN.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Italiano](README.it.md) · [日本語](README.ja.md) · [한국어](README.ko.md)

将应用的 SMTP 指向 OwlMail，注册、密码重置和通知邮件就会进入测试邮箱。你可以在浏览器查看，同一封已提交邮件也可以通过 REST API、WebSocket 或只读 MCP 交给测试程序和编码 Agent。

OwlMail 以一个内嵌 Web 界面的 Go 可执行文件运行，提供 Docker 镜像，支持 Linux amd64 和 arm64。运行二进制无需安装 Go、Node.js 或 Bun。

![OwlMail](.github/assets/owlmail-banner.jpg)

## 📸 预览

![OwlMail 预览](.github/assets/preview.png)

## 🎥 演示视频

![演示视频](.github/assets/realtime.gif)

## 为什么使用 OwlMail

- **看到邮件，也能验证结果。** 查看 HTML、文本、Header、源码和附件；在浏览器验证主题、正文和链接域名，也可通过 API 验证附件数量、文件名、类型、大小和 SHA-256。
- **让异步邮件进入测试流程。** 先记录投递游标，再触发业务、等待指定测试运行的邮件，最后只清理本次测试的数据。
- **给 Agent 一个只读邮箱。** MCP 支持搜索、读取和有界等待；OwlMail 本身不依赖 LLM。
- **发现问题后查得到投递过程。** 支持 Webhook 与手动、自动 Relay；界面可查看尝试次数、失败类别和投递记录。
- **按需要增加部署能力。** 本地 EML 持久化、恢复与隔离；可选 SQLite 查询索引、S3 附件存储、Redis Streams 和 Prometheus 指标。

## 选择版本

下面的 Docker 快速开始使用正式版 **0.11.4**，可以直接体验邮件捕获、浏览器验证、cursor 测试、只读 MCP 和投递诊断。

下文的完整工作流均已包含在 **0.11.4**。固定 CI 环境使用明确的发布版本或镜像 digest；`main`、`latest` 是移动的开发标签。参见 [0.11.4 发布说明](docs/zh-CN/Release-0.11.4.md)和[上一版 0.11.3](docs/zh-CN/Release-0.11.3.md)。

## 快速开始：收到第一封邮件

### 1. 启动 OwlMail

```bash
docker run -d --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.11.4

curl --fail --silent --retry 10 --retry-connrefused --retry-delay 1 \
  --max-time 2 --output /dev/null http://127.0.0.1:1080/readyz
```

打开 [http://localhost:1080](http://localhost:1080)。邮件保存到 `owlmail-data` 卷，重建容器时继续挂载该卷即可保留数据。

端口只发布到本机。默认 SMTP 使用 NO AUTH；需要团队访问时，使用私有网络，并配置 Web 认证和 HTTPS。

### 2. 配置应用 SMTP

本机运行的应用使用：

```text
SMTP host: 127.0.0.1
SMTP port: 1025
TLS: off
Authentication: none
```

如果应用运行在同一 Compose 网络中，主机名改为 OwlMail 的服务名，例如 `owlmail`，端口仍是 **1025**；容器中的 `localhost` 指向该容器自己。

也可以先用 curl 发一封邮件：

```bash
curl --url smtp://127.0.0.1:1025 \
  --mail-from sender@example.test \
  --mail-rcpt developer@example.test \
  --upload-file - <<'MAIL'
From: sender@example.test
To: developer@example.test
Subject: Hello OwlMail
Content-Type: text/plain; charset=utf-8

Your test code is 123456.
MAIL
```

### 3. 查看捕获结果

浏览器中打开新邮件，检查正文和附件，或查询 API：

```bash
curl --fail http://127.0.0.1:1080/api/v1/emails
```

界面支持暗色主题和预览宽度控制。列表返回分页结果。读取原生 `/api/v1` 邮件详情不会自动标记为已读。完整接口见 [API 参考](docs/zh-CN/API-Reference.md)；服务提供 [OpenAPI JSON](http://localhost:1080/api/v1/openapi.json) / [YAML](http://localhost:1080/api/v1/openapi.yaml)，也可直接读取仓库中的 [YAML 合约](openapi/openapi.yaml)。

## 在浏览器中检查邮件

- 在 HTML、文本、Header 和源码之间切换，查看附件和内联图片。
- 用七种预览宽度检查手机、平板和桌面布局；切换宽度保留预览滚动位置。
- 使用暗色主题、搜索、结构化筛选、批量已读 / 删除和 ZIP 导出。
- 按 `Ctrl/Cmd + K` 或 `?` 打开命令面板；`j/k` 或 `↑/↓` 导航，`/` 搜索，`r` 刷新。
- 在 **Verify** 中设置主题、正文和链接域名条件；在 **Deliveries** 中手动刷新 Relay / Webhook 记录，查看结果或确认重试。

主题与正文条件使用大小写敏感的子串匹配。链接检查只解析邮件内容中的 URL，不访问链接；六位数字提取是候选值，不代表验证码有效。新 Verify / Deliveries 控件目前可能使用英文回退。

HTML 预览使用隔离 iframe，远程图片、字体和样式默认阻止，需要时可对当前邮件手动加载。

## 在 CI 中等待、验证和清理

下面使用 Python 3 标准库完成一个测试，不需要额外安装依赖。保存为 `email-check.py`，在上述正式版实例启动后运行 `python3 email-check.py`：

```python
import json
import smtplib
import uuid
from email.message import EmailMessage
from urllib.request import ProxyHandler, Request, build_opener

API = "http://127.0.0.1:1080"
HTTP = build_opener(ProxyHandler({}))
RUN_ID = "readme-" + uuid.uuid4().hex
RECIPIENT = "signup+" + RUN_ID + "@example.test"


def api(method, path, body=None):
    data = None if body is None else json.dumps(body).encode()
    request = Request(
        API + path,
        data=data,
        method=method,
        headers={"Content-Type": "application/json"},
    )
    with HTTP.open(request, timeout=35) as response:
        return json.load(response)


cursor = api("GET", "/api/v1/emails/cursor")["cursor"]

message = EmailMessage()
message["From"] = "sender@example.test"
message["To"] = RECIPIENT
message["Subject"] = "Verify your account"
message["X-OwlMail-Run-ID"] = RUN_ID
message.set_content("Your test code is 123456.")
with smtplib.SMTP("127.0.0.1", 1025, timeout=10) as smtp:
    smtp.send_message(message)

result = api("POST", "/api/v1/emails/wait", {
    "after": cursor,
    "run_id": RUN_ID,
    "recipient": RECIPIENT,
    "count": 1,
    "timeout_seconds": 10,
})
if result["timed_out"] or not result["emails"]:
    raise RuntimeError("Expected email did not arrive")

email_id = result["emails"][0]["id"]
report = api("POST", "/api/v1/emails/" + email_id + "/check", {
    "subject_contains": ["Verify your account"],
    "text_contains": ["123456"],
    "attachment_count": 0,
})
if not report["passed"]:
    raise RuntimeError(json.dumps(report["checks"]))

cleanup = api("DELETE", "/api/v1/test-runs/" + RUN_ID + "/emails")
if cleanup["failed"] or cleanup["more_possible"]:
    raise RuntimeError("Test-run cleanup is incomplete")
print("PASS: received, checked, and cleaned up", RUN_ID)
```

接入真实应用时，将 `smtp.send_message` 替换为注册、密码重置等业务触发操作，并让应用邮件携带 `X-OwlMail-Run-ID`。保留“取得 cursor → 触发 → 等待”的顺序，可覆盖邮件在开始等待前已经到达的情况。

示例在断言通过后清理，失败时保留邮件便于排查。`run_id` 用于测试分组，不是租户或鉴权隔离；cursor 只在当前进程有效。等待超时会返回 `timed_out: true`，调用方需要检查它，不能只看 HTTP 状态。等待的 `count` 表示至少收到指定数量。

更多内容见 [等待与测试运行](docs/en/Delivery-Waits-and-Test-Runs.md)、[JavaScript / Go / Python 示例](examples/testing)和 [CI 快速入门](docs/zh-CN/CI-Quickstart.md)。

## 让编码 Agent 检查邮件

MCP 默认关闭。下面是独立的二进制部署示例。先按[二进制安装](#使用二进制)准备 `./owlmail`，并确认本机 1025 / 1080 未被其他实例占用。启动前设置一个非空的 `OWLMAIL_WEB_PASSWORD`，再启用 HTTP 接口：

```bash
./owlmail -mcp-enabled \
  -web-user agent \
  -web-password "$OWLMAIL_WEB_PASSWORD"
```

让支持 Streamable HTTP 的客户端连接 `http://127.0.0.1:1080/mcp`，并提供上述 Basic Auth 凭据。服务已在 Docker 中运行时，可以添加 `-e OWLMAIL_MCP_ENABLED=true` 及对应 Web 凭据后重建容器。

也可以让本地客户端通过 stdio 读取已有邮箱目录：

```json
{
  "mcpServers": {
    "owlmail": {
      "command": "/absolute/path/to/owlmail",
      "args": ["mcp-stdio", "-mail-directory", "/absolute/path/to/mail"]
    }
  }
}
```

MCP 的七个工具支持列表、搜索、详情、原始源码、附件元数据、最新邮件和等待投递。HTTP 模式直接读取运行实例；stdio 模式读取已提交 EML，并定期扫描目录。

Agent 可以返回注册邮件是否到达、正文中的验证链接及浏览器查看地址。MCP 不提供删除、Relay 或配置修改；原始 MIME 可能包含附件载荷，但没有独立的解码附件下载工具。使用列表 / 搜索返回的 cursor 配合 `wait_for_email`，具体参数见 [MCP 参考](docs/zh-CN/MCP-Reference.md)和[测试运行契约](docs/en/Delivery-Waits-and-Test-Runs.md)。

## 把邮件接入自动化或转发到真实邮箱

### Webhook

内置 `/webhooks` 页面可以生成、导入和下载配置。它不修改正在运行的服务；保存文件后，通过 `-webhook-config` 启动或重启 OwlMail。

例如，将所有新邮件发送给本机接收器。先让接收器监听 `18080`，把下面 JSON 保存为 `webhooks.json`：

```json
{
  "version": 1,
  "targets": [
    {
      "name": "local-receiver",
      "url": "http://127.0.0.1:18080/owlmail"
    }
  ]
}
```

```bash
./owlmail -mail-directory ./mail -webhook-config ./webhooks.json
```

这是独立的本机二进制示例，需要先安装二进制，并确保默认监听端口未被其他实例占用。容器中的 `127.0.0.1` 指向 OwlMail 容器本身；容器部署请使用同一网络中的接收器服务名，并挂载配置文件。

支持收件人 / 主题过滤、自定义请求体、HMAC 签名和重试。需要跨重启恢复排队投递时，使用持久邮件目录、持久化的 Redis Streams，并让接收器按 `X-OwlMail-Delivery-ID` 去重。仅本地 outbox、没有 Redis 时，事件进入内存队列后不再保证跨重启恢复。

参见 [Webhook 指南](docs/zh-CN/Webhook-Forwarding.md)和[可运行 Compose 示例](examples/webhooks/soulteary-webhook)。

### SMTP Relay

配置出站 SMTP 后，可以手动转发选中的测试邮件，或按规则自动转发。出站传输支持 `plain`、强制 `starttls` 和 `smtps`，默认验证证书。

原生 Relay API 返回异步任务；提交成功表示任务被接受，应继续查询状态。持久邮件目录可保存任务并恢复未完成投递。恢复可能重复发送，发送成功也不证明远端最终进入用户收件箱。

需要先配置接收地址与 TLS 策略，参见 [Relay 与运维配置](docs/zh-CN/Operations.md)。

## 安装和日常配置

### 使用二进制

从 [Releases](https://github.com/soulteary/owlmail/releases)下载适合平台的二进制，将文件重命名为 `owlmail`，Unix 平台执行 `chmod +x owlmail`。本文的 `./owlmail` 表示这个下载或本地构建的文件。

也可以使用 Go 1.27.3+ 安装正式版：

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@v0.11.4
owlmail -mail-directory ./mail
```

通过 `go install` 安装后，将示例中的 `./owlmail` 改为 `owlmail`，并确保 Go 安装目录已加入 `PATH`。

本机默认监听 `localhost`；需要持久邮箱时显式指定 `-mail-directory`。

构建当前主分支：

```bash
git clone --branch main --depth 1 https://github.com/soulteary/owlmail.git
cd owlmail
go build -o owlmail ./cmd/owlmail
./owlmail -mail-directory ./mail
```

### 使用配置文件

支持平面 YAML / JSON，键名使用 CLI 选项。把下面内容保存为 `owlmail.yaml`：

```yaml
smtp: 1025
web: 1080
mail-directory: ./mail
mail-retention-days: 7
smtp-max-concurrency: 8
log-format: json
```

```bash
./owlmail -config ./owlmail.yaml
```

该配置保存本地邮件，并按七天保留策略清理。设置 `mail-max-messages` 或 `mail-max-disk-mb` 可进一步控制容量。环境变量和命令行可以覆盖文件配置；全部参数、默认值和环境变量别名见[配置参考](docs/zh-CN/Configuration-Reference.md)，部署方式见[运维指南](docs/zh-CN/Operations.md)。

### 诊断与健康检查

```bash
./owlmail doctor -config ./owlmail.yaml
./owlmail healthcheck -config ./owlmail.yaml
./owlmail healthcheck --ready -config ./owlmail.yaml
```

`doctor` 做离线配置和恢复记录检查，不连接下游，也不证明服务可用。`healthcheck` 检查正在运行的服务；`--ready` 检查缓存的运行时就绪状态。容器内置探针使用启动配置快照，不要求你把端口和 HTTPS 设置再写一遍。

只需要 SMTP 时，可使用 `-disable-web`；它不会提供 Web、REST 或 HTTP MCP。SMTP-only 的独立 readiness 探针需要明确的绝对邮件目录，见[运维指南](docs/en/Operations.md)。

## 从 MailDev 或 MailCatcher 迁移

普通 SMTP 发信应用通常只需修改主机和端口。已有 REST 客户端可显式启用默认关闭的兼容层：

```bash
./owlmail -maildev-rest-compat
# 或
./owlmail -mailcatcher-rest-compat
```

兼容层覆盖文档列出的 HTTP 工作流，不包含 MailDev 的 Node 嵌入 API、Socket.IO 或 MCP 工具契约。新集成使用原生 `/api/v1` 与标准 WebSocket。

详细差异见 [MailDev / MailCatcher / Mailpit 比较与迁移](docs/zh-CN/Comparison-and-Migration.md)。

## 使用边界

OwlMail 面向开发、CI 和受控测试网络。它保存完整邮件，可能包含验证码、重置链接和测试凭据；默认绑定本机，团队部署应设置固定认证、HTTPS 和网络访问范围。

- 一个邮箱目录由一个可写 OwlMail 实例使用，不支持共享多写者邮箱或租户权限隔离。
- S3 只保存解码附件；EML、元数据和投递状态仍需本地持久化。
- Webhook 和恢复后的 Relay 可能重复，接收端需要幂等处理。
- OwlMail 不提供生产 MTA、IMAP / POP3、垃圾邮件评分或真实邮件客户端兼容性认证。

更多细节见[安全模型](docs/zh-CN/Security-Model.md)。

## 文档与反馈

- [0.11.4 发布说明](docs/zh-CN/Release-0.11.4.md) · [上一版 0.11.3 发布说明](docs/zh-CN/Release-0.11.3.md)
- [集成测试](docs/zh-CN/Integration-Testing.md) · [Sendmail 兼容 CLI](docs/zh-CN/Sendmail.md)
- [配置参考](docs/zh-CN/Configuration-Reference.md)
- [API 与 OpenAPI](docs/zh-CN/API-Reference.md)
- [Agent 接入](docs/zh-CN/AI-Agent-Testing.md)
- [部署、持久化、TLS 和监控](docs/zh-CN/Operations.md)
- [架构](docs/zh-CN/Architecture.md) · [变更记录](CHANGELOG.md)
- [贡献指南](.github/CONTRIBUTING.zh-CN.md) · [Issues](https://github.com/soulteary/owlmail/issues)

反馈问题时附上版本或提交、启动方式、脱敏配置、错误信息和最小复现。漏洞请通过[安全报告渠道](.github/SECURITY.md)提交。

## 许可证与致谢

[MIT](LICENSE)。感谢 [MailDev](https://github.com/maildev/maildev) 的启发，以及 Go SMTP / MIME、Fiber 和 WebSocket 等上游项目。

Previous release: [OwlMail 0.11.3](docs/en/Release-0.11.3.md).
