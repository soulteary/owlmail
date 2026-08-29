# Webhook 消息转发

OwlMail 可以把符合规则的新邮件发送到一个或多个 HTTP 端点。该能力在服务端运行，不要求浏览器保持打开，也不会改变 MailDev 兼容 API。

## 开启转发

最小有效配置只需要目标名称和 HTTP(S) 地址：

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

保存配置，并在启动时传给 OwlMail：

```bash
./owlmail -webhook-config ./webhooks.json -webhook-max-concurrency 8
```

也可以使用等价的环境变量：

```bash
export OWLMAIL_WEBHOOK_CONFIG=./webhooks.json
export OWLMAIL_WEBHOOK_MAX_CONCURRENCY=8
```

没有设置参数或环境变量时，Webhook 转发保持关闭。配置只在启动时校验一次；
修改文件后需要重启 OwlMail。

建议默认值允许同时投递 8 封邮件。仅当所有接收端可信、响应足够快且入口负载
可控时，才使用 `-webhook-max-concurrency 0`（或把环境变量设为 `0`）取消限制。

## 选择可运行示例

| 场景 | 配置 |
|---|---|
| 使用默认请求体转发所有新邮件 | [`examples/webhooks/minimal.json`](../../examples/webhooks/minimal.json) |
| 按收件人和主题过滤 | [`examples/webhooks/filtered-alerts.json`](../../examples/webhooks/filtered-alerts.json) |
| 带鉴权的自定义 JSON 和 HMAC | [`examples/webhooks/custom-json.json`](../../examples/webhooks/custom-json.json) |
| 全量归档与事故目标多路分发 | [`examples/webhooks/multiple-targets.json`](../../examples/webhooks/multiple-targets.json) |
| 纯文本请求体 | [`examples/webhooks/plain-text.json`](../../examples/webhooks/plain-text.json) |
| 完整 OwlMail + `soulteary/webhook` Compose 联动 | [`examples/webhooks/soulteary-webhook/`](../../examples/webhooks/soulteary-webhook/) |
| 单个目标展示大部分参数 | [`examples/webhooks.json`](../../examples/webhooks.json) |

[示例操作指南](../../examples/webhooks/README.zh-CN.md)包含仅监听本机的接收器、
完整启动命令和测试 SMTP 邮件。

## 配置格式

```json
{
  "version": 1,
  "targets": [
    {
      "name": "notifications",
      "url": "https://notify.example.com/hooks/owlmail",
      "method": "POST",
      "headers": {
        "Authorization": "Bearer ${OWLMAIL_WEBHOOK_TOKEN}"
      },
      "contentType": "application/json",
      "secret": "${OWLMAIL_WEBHOOK_SECRET}",
      "timeout": "5s",
      "retries": 2,
      "match": {
        "from": ["*@example.com"],
        "to": ["alerts@*"],
        "subject": ["*alert*", "*验证码*"],
        "text": ["*code*", "*验证码*"]
      },
      "bodyTemplate": "{\"title\":{{ json .Subject }},\"message\":{{ json .Text }}}"
    }
  ]
}
```

| 字段 | 必需 | 行为 |
|---|---:|---|
| `version` | 否 | 配置版本；省略或设置为 `1` 都表示版本 1。 |
| `targets` | 是 | 1～32 个目标，目标名称必须唯一。 |
| `name` | 是 | 仅用于日志的安全标识，最长 100 字符且不能包含换行；完整 URL 和配置密钥不会写入日志。 |
| `url` | 是 | 固定的 `http` 或 `https` 地址；拒绝用户信息和片段，不会跟随重定向。 |
| `method` | 否 | 默认 `POST`，支持 `POST`、`PUT`、`PATCH`。 |
| `headers` | 否 | 静态请求头；名称和值会被校验，不能覆盖 `Host` 和 `Content-Length`。 |
| `contentType` | 否 | 默认 `application/json`；显式 `Content-Type` 请求头优先。 |
| `secret` | 否 | 对最终请求体生成 `X-OwlMail-Signature: sha256=<hex>`。 |
| `timeout` | 否 | 每次请求的超时，默认 `5s`，最大 `1m`。 |
| `retries` | 否 | 0～5 次额外尝试；仅网络错误、408、425、429 和 5xx 会重试。 |
| `match` | 否 | 不区分大小写的通配符规则，语义见下文。 |
| `bodyTemplate` | 否 | 自定义请求体的 Go 文本模板；省略时发送默认事件结构。 |

`url`、请求头值和 `secret` 支持 `${VARIABLE}` 占位符。如果变量不存在或值为空，OwlMail 会在启动阶段报错，不会静默发送缺少鉴权信息的请求。

只有带花括号的 `${VARIABLE}` 会展开，`$VARIABLE` 会作为普通文本。环境变量
不会展开到 `name`、`contentType`、匹配规则或 `bodyTemplate`。

### 校验与限制

- 配置文件最大 1 MiB，只能包含一个 JSON 值，并拒绝未知字段；
- 版本 1 支持 1～32 个名称唯一的目标；
- 模板渲染后的请求体最大 2 MiB，超出时不会发出 HTTP 请求；
- 单次超时必须大于零且不超过一分钟，默认每次尝试五秒；
- `retries` 表示额外尝试次数，例如 `2` 表示最多请求三次；
- SMTP 和 Web 服务启动前，会先编译全部目标与模板，因此错误配置不会造成部分启用。

## 并发与背压

`-webhook-max-concurrency` 限制所有目标共享的并发邮件投递任务数。默认值 `8`
适合本地开发和小型 CI 环境。估算初始值时，可以用“峰值每秒邮件数 × 接收端
p95 响应秒数”，然后向上取整。

OwlMail 会在创建 Webhook 处理 goroutine 之前获取并发槽位。槽位耗尽时，邮件已经
持久化，事件处理会等待空闲槽位；轻量日志和 WebSocket 监听器仍会优先启动。
因此慢接收端不会制造无限增长的 goroutine。单个任务内的匹配目标仍按顺序投递。

`0` 会保留旧版的无限并发行为，只应在明确需要无限制分发时使用；突发流量下可能
消耗大量内存和连接。

## 匹配规则

`from`、`to`、`subject`、`text` 支持 Go shell 风格通配符数组：`*`、`?`、
`[a-z]` 等字符组以及 `\` 转义。与 Go 的 `path.Match` 一致，`*` 不匹配 `/`。

- 同一个字段内的多个规则是“或”关系；
- 多个非空字段之间是“且”关系；
- 空字段匹配所有邮件；
- 匹配不区分大小写；
- `to` 同时检查 SMTP 信封收件人，以及 To、Cc、Bcc 邮件头。
- `from` 同时检查解析后的 From 地址与 SMTP 信封发件人；
- `text` 只匹配解析后的纯文本正文，不搜索 HTML 或附件；只有 HTML 的邮件通常
  没有可匹配的 `text`；
- 不支持按任意邮件头过滤。如果接收端需要邮件头，可在自定义模板中读取 `.Headers`。

例如，下面的规则只转发来自 `example.com`、主题为告警或验证码、正文包含验证码的邮件：

```json
{
  "from": ["*@example.com"],
  "subject": ["*alert*", "*验证码*"],
  "text": ["*code*", "*验证码*"]
}
```

## 自定义请求体

模板可使用以下字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `.ID`、`.Subject`、`.Text`、`.HTML` | 字符串 | 邮件标识和内容。 |
| `.Time` | 时间 | 邮件头时间或接收时间。 |
| `.From`、`.To`、`.CC`、`.BCC` | 字符串数组 | 不带显示名称的解析后地址。 |
| `.EnvelopeFrom`、`.EnvelopeTo` | 字符串 / 字符串数组 | SMTP 信封地址。 |
| `.Size`、`.SizeHuman` | 数值 / 字符串 | 已保存邮件的大小。 |
| `.Attachments`、`.AttachmentCount` | 数组 / 数值 | 安全的附件元数据；不暴露附件正文和本地路径。 |
| `.Headers` | 映射 | 解析后的邮件头，仅在显式模板中可用。 |

模板函数：

- `json VALUE`：把值安全编码为 JSON，避免引号或换行破坏请求结构；
- `join STRINGS SEPARATOR`：连接地址数组；
- `truncate STRING LENGTH`：按 Unicode 字符数截断文本。

模板使用 `missingkey=error`；映射键不存在或模板执行错误会使该目标发送失败。
解析后的地址只包含邮箱地址，不包含显示名称。`.Headers` 的值可能是字符串、数组
或其他解析类型，因此应通过 `json` 传递，不要假设所有值都是字符串。

把邮件内容写进 JSON 时，应始终使用 `json`：

```json
"bodyTemplate": "{\"title\":{{ json .Subject }},\"message\":{{ json (truncate .Text 4000) }}}"
```

未配置模板时，默认发送：

```json
{
  "event": "email.received",
  "message": "示例\n邮件正文",
  "email": {
    "id": "email-id",
    "time": "2026-08-29T12:00:00Z",
    "subject": "示例",
    "from": ["sender@example.com"],
    "to": ["recipient@example.com"],
    "envelopeFrom": "sender@example.com",
    "envelopeTo": ["recipient@example.com"],
    "text": "邮件正文",
    "html": "<p>邮件正文</p>",
    "size": 123,
    "sizeHuman": "123 B",
    "attachments": [
      {
        "fileName": "report.txt",
        "contentType": "text/plain",
        "size": 42
      }
    ],
    "attachmentCount": 1
  }
}
```

`cc`、`bcc`、`html`、信封字段和 `attachments` 等可选值为空时会省略。
默认请求体有意不包含 `headers`，只有显式模板可以读取。附件正文和本地存储路径
永远不会发送。

## 请求头

| 请求头 | 值 |
|---|---|
| `Content-Type` | 目标的 `contentType`，默认为 `application/json`；显式自定义请求头优先。 |
| `User-Agent` | 默认为 `OwlMail-Webhook/1`，可用自定义请求头覆盖。 |
| `X-OwlMail-Event` | 固定为 `email.received`。 |
| `X-OwlMail-Email-ID` | OwlMail 邮件 ID，可作为幂等键。 |
| `X-OwlMail-Signature` | 只在配置 `secret` 时发送；格式为 `sha256=` 加上对原始请求体计算的十六进制小写 HMAC。 |

重试会使用相同的请求体和邮件 ID。接收端在执行非幂等操作前应先去重。

## 发送到 `soulteary/webhook`

可运行示例会同时启动两个项目，验证 HMAC，把请求字段映射为环境变量，并执行
一个可以直接观察结果的演示命令：

```bash
cd examples/webhooks/soulteary-webhook
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
docker compose up --build
```

发送测试邮件和查看结果的步骤见[完整联动指南](../../examples/webhooks/soulteary-webhook/README.zh-CN.md)。

手动部署时，把目标地址设置为
[`soulteary/webhook`](https://github.com/soulteary/webhook) 提供的 Hook 地址：

```json
{
  "version": 1,
  "targets": [
    {
      "name": "automation",
      "url": "http://webhook:9000/hooks/owlmail",
      "secret": "${OWLMAIL_WEBHOOK_SECRET}",
      "bodyTemplate": "{\"message\":{{ json .Text }},\"title\":{{ json .Subject }},\"emailId\":{{ json .ID }}}"
    }
  ]
}
```

对应的 `soulteary/webhook` Hook 配置如下。这是 Go 模板源文件，在
`soulteary/webhook -template` 渲染之前有意不是合法 JSON：

```text
[
  {
    "id": "owlmail",
    "execute-command": "/app/notify.sh",
    "incoming-payload-content-type": "application/json",
    "http-methods": ["POST"],
    "pass-environment-to-command": [
      { "source": "payload", "name": "message", "envname": "OWLMAIL_MESSAGE" },
      { "source": "payload", "name": "title", "envname": "OWLMAIL_TITLE" },
      { "source": "payload", "name": "emailId", "envname": "OWLMAIL_EMAIL_ID" }
    ],
    "trigger-rule": {
      "match": {
        "type": "payload-hmac-sha256",
        "secret": "{{ getenv "OWLMAIL_WEBHOOK_SECRET" | js }}",
        "parameter": { "source": "header", "name": "X-OwlMail-Signature" }
      }
    }
  }
]
```

Hook 文件使用 `getenv` 时，需要用 `-template` 参数启动 `soulteary/webhook`。
两个容器应配置相同的 `OWLMAIL_WEBHOOK_SECRET`。

## 发送生命周期与问题排查

1. OwlMail 先解析并保存 SMTP 邮件；
2. `new` 事件异步启动 Webhook 发送，因此目标缓慢或失败不会改变 SMTP 接收结果；
3. 同一封邮件命中的目标按照配置顺序依次调用，不同邮件事件可能并发发送；
4. 2xx 表示目标完成。网络错误、408、425、429 和 5xx 会根据需要等待约
   100 ms、200 ms、400 ms、800 ms、1.6 s 后重试；不会解析 `Retry-After`；
5. 其他 3xx/4xx 会立即失败，响应正文不会参与处理。

转发没有持久化发送队列或死信存储。配置的尝试次数耗尽后，OwlMail 会保留邮件并
记录失败，但重启后不会再次发送该事件。

本地检查时，可在不同终端运行内置接收器和最小配置：

```bash
go run ./examples/webhooks/receiver
go run ./cmd/owlmail -webhook-config ./examples/webhooks/minimal.json
```

排查建议：

- 启动错误通常来自非法 JSON、未知字段、缺失 `${VARIABLE}`、错误通配符/时长
  或模板解析失败；
- 没有收到请求通常表示规则未命中，可暂时移除 `match` 或改用 `minimal.json`；
- 容器内的 `127.0.0.1` 指向 OwlMail 自身，应使用接收器服务名和容器端口；
- HTTP 401/403 通常表示自定义 Token 或 HMAC 密钥不同。HMAC 覆盖请求体的精确
  字节，验证前不要重新格式化请求体；
- 成功发送记录在 verbose 日志中；失败日志包含安全目标名、状态和尝试次数，
  不包含完整 URL 或密钥。

## 运行与安全边界

- Webhook 目标会收到邮件内容，只能配置可信目的地；
- 容器私网之外优先使用 HTTPS，并使用 HMAC 签名；
- 配置文件建议设置为 `0600`，Token 和密钥通过环境变量传入；
- 任意 2xx 都视为成功；重定向和其他非 2xx 响应视为失败；
- Webhook 发送相对于 SMTP 存储是异步的，发送失败不会拒收或删除邮件；
- 重试可能造成重复请求，执行非幂等动作前应使用 `email.id` 或 `X-OwlMail-Email-ID` 去重。
- 默认请求体可能同时包含纯文本和 HTML 正文；接收端只需要标题或验证码时，
  应使用自定义模板减少数据；
- Webhook 转发适合通知和自动化，不是可靠消息队列；需要投递保证时，应把目标
  设置为具备持久化能力的下游服务。
