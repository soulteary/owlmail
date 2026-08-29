# Webhook 消息转发

OwlMail 可以把符合规则的新邮件发送到一个或多个 HTTP 端点。该能力在服务端运行，不要求浏览器保持打开，也不会改变 MailDev 兼容 API。

## 开启转发

创建 JSON 配置文件，并在启动时传给 OwlMail：

```bash
export OWLMAIL_WEBHOOK_SECRET='请替换为随机密钥'
./owlmail -webhook-config ./webhooks.json
```

也可以使用等价的环境变量：

```bash
export OWLMAIL_WEBHOOK_CONFIG=./webhooks.json
```

完整示例见 [`examples/webhooks.json`](../../examples/webhooks.json)。

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
| `name` | 是 | 仅用于日志的安全标识；完整 URL 和配置密钥不会写入日志。 |
| `url` | 是 | 固定的 `http` 或 `https` 地址；不会跟随重定向。 |
| `method` | 否 | 默认 `POST`，支持 `POST`、`PUT`、`PATCH`。 |
| `headers` | 否 | 静态请求头；不能覆盖 `Host` 和 `Content-Length`。 |
| `contentType` | 否 | 默认 `application/json`；显式 `Content-Type` 请求头优先。 |
| `secret` | 否 | 对最终请求体生成 `X-OwlMail-Signature: sha256=<hex>`。 |
| `timeout` | 否 | 每次请求的超时，默认 `5s`，最大 `1m`。 |
| `retries` | 否 | 0～5 次额外尝试；仅网络错误、408、425、429 和 5xx 会重试。 |
| `match` | 否 | 不区分大小写的通配符规则，语义见下文。 |
| `bodyTemplate` | 否 | 自定义请求体的 Go 文本模板；省略时发送默认事件结构。 |

`url`、请求头值和 `secret` 支持 `${VARIABLE}` 占位符。如果变量不存在，OwlMail 会在启动阶段报错，不会静默发送缺少鉴权信息的请求。

## 匹配规则

`from`、`to`、`subject`、`text` 支持含 `*`、`?` 的通配符数组：

- 同一个字段内的多个规则是“或”关系；
- 多个非空字段之间是“且”关系；
- 空字段匹配所有邮件；
- 匹配不区分大小写；
- `to` 同时检查 SMTP 信封收件人，以及 To、Cc、Bcc 邮件头。

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

把邮件内容写进 JSON 时，应始终使用 `json`：

```json
"bodyTemplate": "{\"title\":{{ json .Subject }},\"message\":{{ json (truncate .Text 4000) }}}"
```

未配置模板时，默认发送：

```json
{
  "event": "email.received",
  "message": "主题和纯文本正文",
  "email": {
    "id": "email-id",
    "subject": "示例",
    "from": ["sender@example.com"],
    "to": ["recipient@example.com"],
    "text": "邮件正文"
  }
}
```

请求头还会包含 `X-OwlMail-Email-ID`。接收端可用它作为幂等键，处理重试造成的重复请求。

## 发送到 `soulteary/webhook`

把目标地址设置为 [`soulteary/webhook`](https://github.com/soulteary/webhook) 提供的 Hook 地址：

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

对应的 `soulteary/webhook` Hook 配置示例：

```json
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
        "secret": "{{ getenv \"OWLMAIL_WEBHOOK_SECRET\" | js }}",
        "parameter": { "source": "header", "name": "X-OwlMail-Signature" }
      }
    }
  }
]
```

Hook 文件使用 `getenv` 时，需要用 `-template` 参数启动 `soulteary/webhook`。两个容器应配置相同的 `OWLMAIL_WEBHOOK_SECRET`。

## 运行与安全边界

- Webhook 目标会收到邮件内容，只能配置可信目的地；
- 容器私网之外优先使用 HTTPS，并使用 HMAC 签名；
- 配置文件建议设置为 `0600`，Token 和密钥通过环境变量传入；
- 任意 2xx 都视为成功；重定向和其他非 2xx 响应视为失败；
- Webhook 发送相对于 SMTP 存储是异步的，发送失败不会拒收或删除邮件；
- 重试可能造成重复请求，执行非幂等动作前应使用 `email.id` 或 `X-OwlMail-Email-ID` 去重。
