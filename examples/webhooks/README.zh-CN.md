# OwlMail Webhook 示例

示例按场景拆分。建议从最接近需求的最小配置开始，而不是从一份大配置中逐项删除。

| 场景 | 配置 | 重点 |
|---|---|---|
| 转发所有新邮件 | [`minimal.json`](./minimal.json) | 目标所需的两个必填字段，以及 OwlMail 默认 JSON 事件。 |
| 只转发指定告警 | [`filtered-alerts.json`](./filtered-alerts.json) | 收件人和主题过滤、超时、重试。 |
| 调用带鉴权的 JSON API | [`custom-json.json`](./custom-json.json) | 环境变量 URL/Token/密钥、自定义请求头、HMAC 和 JSON 安全模板。 |
| 同时发送多个目标 | [`multiple-targets.json`](./multiple-targets.json) | 全量归档目标和按规则触发的事故目标；同一邮件可以命中两者。 |
| 发送纯文本消息 | [`plain-text.json`](./plain-text.json) | 非 JSON Content-Type 和文本模板。 |
| 联动 `soulteary/webhook` | [`soulteary-webhook/`](./soulteary-webhook/) | 完整 Docker Compose、HMAC 校验、字段映射和命令执行。 |
| 单文件完整参考 | [`../webhooks.json`](../webhooks.json) | 在一个目标中同时展示大部分配置项。 |

## 五分钟本地验证

仓库内置的示例接收器只监听 `127.0.0.1:18080`，会打印请求并返回 HTTP
204；配置密钥后还可以验证 OwlMail 的 HMAC 签名。

终端 1——启动接收器：

```bash
go run ./examples/webhooks/receiver
```

终端 2——使用最小配置启动 OwlMail：

```bash
go run ./cmd/owlmail -webhook-config ./examples/webhooks/minimal.json
```

终端 3——发送测试邮件（curl 需要包含 SMTP 协议支持）：

```bash
printf 'From: sender@example.test\r\nTo: inbox@example.test\r\nSubject: Minimal webhook\r\n\r\nHello from OwlMail.\r\n' \
  | curl --url smtp://127.0.0.1:1025 \
      --mail-from sender@example.test \
      --mail-rcpt inbox@example.test \
      --upload-file -
```

接收器会输出请求路径、事件类型、邮件 ID、Content-Type、签名状态和请求体。
使用 Ctrl+C 停止两个进程。

## 各场景使用方法

### 最小配置：转发全部邮件

```bash
./owlmail -webhook-config ./examples/webhooks/minimal.json
```

配置不包含 `match`、`bodyTemplate`、密钥或自定义请求头，因此每封新邮件都会
使用 OwlMail 默认的 `email.received` JSON 结构发送一次。

### 过滤告警

```bash
./owlmail -webhook-config ./examples/webhooks/filtered-alerts.json
```

邮件必须至少命中一个 `to` 规则，**并且**至少命中一个 `subject` 规则。
请把示例地址和关键词替换为实际告警规则。

### 带鉴权的自定义 JSON

先在接收器终端开启签名校验：

```bash
export OWLMAIL_WEBHOOK_SECRET='replace-this-development-secret'
go run ./examples/webhooks/receiver
```

在 OwlMail 终端设置配置引用的全部环境变量：

```bash
export OWLMAIL_WEBHOOK_URL='http://127.0.0.1:18080/custom'
export OWLMAIL_WEBHOOK_TOKEN='development-token'
export OWLMAIL_WEBHOOK_SECRET='replace-this-development-secret'
./owlmail -webhook-config ./examples/webhooks/custom-json.json
```

接收器会校验 `X-OwlMail-Signature-V2`、限制签名时间为五分钟，并拒绝重复 nonce。
对于 `Authorization`，它只记录请求头
是否存在，不会输出 Token 内容。

### 多目标分发

```bash
./owlmail -webhook-config ./examples/webhooks/multiple-targets.json
```

每封邮件都会发送到 `/archive`；同时，收件人为 `ops@*` 且主题包含
`critical` 或 `outage` 的邮件也会发送到 `/incidents`。每封邮件的目标按照
配置文件中的顺序依次处理。

### 纯文本消息

```bash
./owlmail -webhook-config ./examples/webhooks/plain-text.json
```

该配置发送 `text/plain; charset=utf-8`，并把邮件正文截断为 2,000 个 Unicode
字符，而不是发送 JSON。

## 在容器中运行 OwlMail

容器内的 `127.0.0.1` 指向容器自身。请把示例 URL 换成 Compose 服务名（例如
`http://receiver:18080/owlmail`），或换成 OwlMail 容器能够访问的地址。
[`soulteary-webhook`](./soulteary-webhook/) 示例展示了服务名寻址方式。

## 部署前验证

OwlMail 会在启动时校验并编译整个配置文件。环境变量缺失或值为空、未知 JSON 字段、
非法时长、无效通配符或错误模板都会阻止服务启动。修改配置后需要重启 OwlMail；
Webhook 配置不会热加载。

OwlMail 默认最多并发处理 8 个邮件 Webhook 任务，适合多数开发和 CI 负载。
仅在明确需要无限并发时使用 `-webhook-max-concurrency 0`。

所有字段、默认请求体、请求头、规则、重试行为和安全边界见
[中文完整参考](../../docs/zh-CN/Webhook-Forwarding.md) 或
[English reference](../../docs/en/Webhook-Forwarding.md)。
