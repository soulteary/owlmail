# OwlMail API 参考

本文档描述当前 OwlMail 源码实际注册的 HTTP 与 WebSocket API。新集成建议使用
带版本号的 `/api/v1` 路由。无版本路由继续服务已有 OwlMail 客户端和常见的
MailDev 风格工作流，但不代表与 MailDev 协议的逐项、逐字节一致。

## 基础地址与鉴权

默认基础地址为 `http://localhost:1080`。除 HTML、纯文本、附件、EML 和 ZIP
下载端点外，响应均为 JSON。

使用 `-web-user` / `OWLMAIL_WEB_USER` 与 `-web-password` /
`OWLMAIL_WEB_PASSWORD` 配置 HTTP Basic Auth。两项 Web 凭据都未配置时认证
关闭；只配置一项时按下表补全：

| 配置 | 实际凭据 |
|---|---|
| 两项都未设置 | 关闭鉴权 |
| 只设置用户名 | 使用该用户名，并生成一个 32 字符密码；密码只在 stderr 输出一次 |
| 只设置密码 | 使用默认用户名 `admin` 和已配置密码 |
| 两项都设置 | 使用已配置用户名和密码 |

自动生成的密码会在每次重启时变化；需要固定凭据时请显式配置两项。如果无法
将该密码写入 stderr，OwlMail 会启动失败，因为此时不存在可恢复的有效凭据。
健康检查端点不要求鉴权。启用 Basic Auth 后，携带 `Origin` 的浏览器请求和
WebSocket 升级必须来自 OwlMail 自身源；此同源检查仍适用于无需鉴权的健康检查
端点，并可能返回纯文本 `403`。不携带 `Origin` 的服务端客户端仍可访问。离开
可信本地开发环境时应同时启用 HTTPS。

```bash
curl -u admin:secret http://localhost:1080/api/v1/emails
```

## OpenAPI 3.1 合约

版本控制中的规范合约提供
[JSON](../../openapi/openapi.json) 与 [YAML](../../openapi/openapi.yaml)
两种格式。运行中的服务也通过只读端点返回相同合约：

```bash
curl -u admin:secret http://localhost:1080/api/v1/openapi.json
curl -u admin:secret http://localhost:1080/api/v1/openapi.yaml
```

这两个合约端点遵循普通 Basic Auth 与浏览器同源策略。版本化 API 中只有
`/api/v1/health` 和 `/api/v1/ready` 公开。配置
`-base-pathname=/owlmail` 后，应访问
`/owlmail/api/v1/openapi.json`，返回值中的 `servers[0].url` 也会变为
`/owlmail/api/v1`。

合约只描述 OwlMail 原生 `/api/v1` 行为，明确排除无版本的 MailDev 风格兼容
路由。CI 会解析两种格式、验证二者语义一致、解析全部本地 `$ref`，并逐项
比较已注册的版本化方法/路径与合约，因此新增或删除 API 时会检测到合约漂移。

## 通用约定

- 默认邮件 ID 是八字符随机字符串；启用 `-use-uuid-for-email-id` 后，新邮件
  使用 UUID。两种格式均可查询。
- `GET /api/v1/emails/:id` 和 `GET /email/:id` **不会**自动标记已读；请显式
  调用对应的 `PATCH` 路由。
- 列表和预览端点默认 `limit=50`、`offset=0`，最大 `limit` 为 1000；非法值
  会回退到默认值。
- 时间由 Go `time.Time` 编码为 RFC 3339 格式。
- 修改成功通常返回 `code`、`message` 和可选的 `data`；API 处理器产生的
  错误会返回对应 HTTP 状态码，以及 `code`、`error`、`message`。Basic
  Auth 与浏览器同源中间件会在进入 API 处理器前直接返回纯文本 `401` 或
  `403`。启用后，同源检查先于 Basic Auth 执行，因此 `403` 不表示鉴权已经成功。

列表响应示例：

```json
{
  "total": 1,
  "limit": 50,
  "offset": 0,
  "emails": [
    {
      "id": "aB3dEfGh",
      "time": "2026-08-29T12:00:00Z",
      "read": false,
      "subject": "Welcome",
      "from": [{ "Address": "sender@example.com", "Name": "Sender" }],
      "to": [{ "Address": "recipient@example.com", "Name": "" }]
    }
  ]
}
```

错误示例：

```json
{
  "code": "EMAIL_NOT_FOUND",
  "error": "EMAIL_NOT_FOUND",
  "message": "Email not found"
}
```

## 筛选、分页与导出

`GET /api/v1/emails`、`GET /email` 和两个预览路由支持：

| 查询参数 | 含义 |
|---|---|
| `limit` | 每页数量，默认 50，最大 1000 |
| `offset` | 跳过的匹配记录数，从 0 开始 |
| `q` | 在主题、纯文本和 HTML 中进行不区分大小写的子串搜索 |
| `from` | 在发件人地址或名称中进行不区分大小写的子串搜索 |
| `to` | 在收件人地址或名称中进行不区分大小写的子串搜索 |
| `dateFrom` | `YYYY-MM-DD` 格式的包含式起始日期 |
| `dateTo` | `YYYY-MM-DD` 格式的包含式结束日期 |
| `read` | `true` 或 `false` |
| `sortBy` | `time`、`subject`、`from` 或 `size`；省略时默认按时间倒序 |
| `sortOrder` | `asc` 或 `desc` |

导出路由支持相同筛选条件。设置 `ids=id1,id2` 时优先按给定 ID 导出。

精简预览中的 `preview` 字符串按 200 个 UTF-8 字节（而非字符）截断，发生截断时
追加 `...`。

## 版本化 API

### 邮件集合

| 方法与路径 | 用途 |
|---|---|
| `GET /api/v1/emails` | 分页返回完整邮件对象 |
| `GET /api/v1/emails/stats` | 返回总数、未读、已读及按日期统计 |
| `GET /api/v1/emails/preview` | 分页返回精简预览 |
| `GET /api/v1/emails/export` | 将匹配邮件作为 EML 文件打包为 ZIP 下载 |
| `DELETE /api/v1/emails` | 删除全部邮件 |
| `PATCH /api/v1/emails/read` | 全部标记为已读 |
| `DELETE /api/v1/emails/batch` | 按 JSON 请求体中的 ID 批量删除 |
| `PATCH /api/v1/emails/batch/read` | 按 JSON 请求体中的 ID 批量标记已读 |
| `POST /api/v1/emails/reload` | 从已配置邮件目录重新加载 EML |

两个批量路由都接收：

```json
{ "ids": ["aB3dEfGh", "550e8400-e29b-41d4-a716-446655440000"] }
```

### 单封邮件

| 方法与路径 | 用途/响应类型 |
|---|---|
| `GET /api/v1/emails/:id` | 完整邮件 JSON |
| `DELETE /api/v1/emails/:id` | 删除单封邮件 |
| `PATCH /api/v1/emails/:id/read` | 标记单封邮件已读 |
| `GET /api/v1/emails/:id/html` | 清理后的 HTML，`text/html; charset=utf-8` |
| `GET /api/v1/emails/:id/source` | RFC 822 原始源码，`text/plain; charset=utf-8` |
| `GET /api/v1/emails/:id/raw` | 下载 EML，`message/rfc822` |
| `GET /api/v1/emails/:id/attachments/:filename` | 使用附件元数据 Content-Type 返回解码字节 |
| `POST /api/v1/emails/:id/actions/relay` | 按邮件原收件人中继 |
| `POST /api/v1/emails/:id/actions/relay/:relayTo` | 中继到一个明确地址 |

中继路由要求先配置出站 SMTP。成功响应只表示 OwlMail 收到了进程内中继请求，并
尝试将其交给出站工作器；**不保证**队列已经接受任务，也不表示下游 SMTP 已经投递
邮件。API 在异步处理前不会对 `relayTo` 做完整邮箱地址语法校验；队列饱和以及 HTTP
响应之后发生的下游错误只会记录到进程日志。

### 设置与系统

| 方法与路径 | 用途 |
|---|---|
| `GET /api/v1/settings` | 返回 SMTP、Web、出站、SMTP 鉴权和 TLS 运行时设置；不返回密钥 |
| `GET /api/v1/settings/outgoing` | 返回出站 SMTP 设置；不返回密码 |
| `PUT /api/v1/settings/outgoing` | 整体替换出站 SMTP 设置 |
| `PATCH /api/v1/settings/outgoing` | 更新部分出站 SMTP 字段 |
| `GET /api/v1/health` | 无需鉴权的存活检查 |
| `GET /api/v1/ready` | 无需鉴权、读取缓存的依赖 readiness 检查 |
| `GET /api/v1/version` | 构建/版本信息 |
| `GET /api/v1/ws` | 原生 WebSocket 端点 |
| `GET /api/v1/openapi.json` | 支持 base path 的 OpenAPI 3.1 JSON 合约 |
| `GET /api/v1/openapi.yaml` | 支持 base path 的 OpenAPI 3.1 YAML 合约 |

WebSocket upgrade header 或握手 key 格式错误时，会在建立 WebSocket 连接前返回
纯文本 `400`。

发布构建会返回类似以下的版本来源信息：

```json
{
  "version": "0.6.0",
  "commit": "<完整 Git 提交 SHA>",
  "build_date": "<UTC RFC 3339 时间>",
  "branch": "v0.6.0",
  "go_version": "go1.27.0",
  "platform": "linux/amd64",
  "compiler": "gc"
}
```

发布二进制与容器镜像会在构建时注入前四项，并冒烟验证版本与提交。普通本地
`go build` 对未注入字段使用开发默认值。

出站设置请求体支持 `host`、`port`、`user`、`password`、`secure`、
`autoRelay`、`autoRelayAddr`、`allowRules`、`denyRules`。`host` 必填，`port`
必须在 1 到 65535 之间。API 修改仅保存在内存，不会改写进程参数或环境变量。
PATCH 请求中的规则列表如出现必须是数组；要清空列表请传空数组，`null` 不会触发
更新。

NO AUTH 模式下，设置端点返回的 `smtpAuth` 为 `null`；启用强制认证后，该对象只
返回配置的用户名，不会返回密码。同时设置 `OWLMAIL_SMTP_USER` 与
`OWLMAIL_SMTP_PASSWORD` 后会强制执行 PLAIN/LOGIN 认证；只设置其中一项会启动
失败。设置 `OWLMAIL_SMTP_AUTH_REQUIRE_TLS=true` 后，AUTH 只能在 STARTTLS 后或
SMTPS 连接上执行，并要求启用 SMTP TLS；NO AUTH 模式下的匿名投递不受影响。该
启动策略不能通过设置 API 修改。详见
[运维与排障](./Operations.md#smtp-入口限制与鉴权模式)。

入站 DATA 处理默认限制为每进程 8 个并发事务，普通 SMTP、STARTTLS 与 SMTPS
共用上限。可通过 `OWLMAIL_SMTP_MAX_CONCURRENCY` 或 `-smtp-max-concurrency`
设置；`0` 表示不限制。达到上限时 SMTP 客户端收到可重试的 `451 4.3.2`，而不是
HTTP API 错误。该进程级启动配置不会由设置 API 返回或修改。

```bash
curl -u admin:secret \
  -H 'Content-Type: application/json' \
  -X PUT http://localhost:1080/api/v1/settings/outgoing \
  -d '{
    "host": "smtp.example.com",
    "port": 587,
    "user": "relay-user",
    "password": "relay-password",
    "secure": true
  }'
```

## 无版本兼容路由

这些路径沿用 MailDev 历史风格，供现有 OwlMail 集成继续使用。新代码应优先
使用 `/api/v1`。

| 方法与路径 | 版本化等价路径或用途 |
|---|---|
| `GET /email` | `GET /api/v1/emails` |
| `GET /email/:id` | `GET /api/v1/emails/:id` |
| `GET /email/:id/html` | `GET /api/v1/emails/:id/html` |
| `GET /email/:id/attachment/:filename` | `GET /api/v1/emails/:id/attachments/:filename` |
| `GET /email/:id/download` | `GET /api/v1/emails/:id/raw` |
| `GET /email/:id/source` | `GET /api/v1/emails/:id/source` |
| `DELETE /email/:id` | `DELETE /api/v1/emails/:id` |
| `DELETE /email/all` | `DELETE /api/v1/emails` |
| `PATCH /email/read-all` | `PATCH /api/v1/emails/read` |
| `PATCH /email/:id/read` | `PATCH /api/v1/emails/:id/read` |
| `POST /email/:id/relay` | `POST /api/v1/emails/:id/actions/relay` |
| `POST /email/:id/relay/:relayTo` | `POST /api/v1/emails/:id/actions/relay/:relayTo` |
| `GET /email/stats` | `GET /api/v1/emails/stats` |
| `GET /email/preview` | `GET /api/v1/emails/preview` |
| `POST /email/batch/delete` | 使用 `{ "ids": [...] }` 批量删除 |
| `POST /email/batch/read` | 使用 `{ "ids": [...] }` 批量标记已读 |
| `GET /email/export` | `GET /api/v1/emails/export` |
| `GET /socket.io` | 原生 WebSocket，**不是** Socket.IO 协议 |
| `GET /config` | `GET /api/v1/settings` |
| `GET /config/outgoing` | `GET /api/v1/settings/outgoing` |
| `PUT /config/outgoing` | `PUT /api/v1/settings/outgoing` |
| `PATCH /config/outgoing` | `PATCH /api/v1/settings/outgoing` |
| `GET /healthz` | `GET /api/v1/health` |
| `GET /readyz` | `GET /api/v1/ready` |
| `GET /reloadMailsFromDirectory` | 重新加载已配置邮件目录 |

## WebSocket 协议

两个 WebSocket 路径都实现标准 RFC 6455 WebSocket。连接成功后先收到：

```json
{ "type": "connected", "message": "WebSocket connection established" }
```

服务端事件：

```json
{ "type": "new", "email": { "id": "aB3dEfGh", "subject": "Welcome" } }
```

```json
{ "type": "delete", "id": "aB3dEfGh" }
```

客户端可发送 `{ "type": "ping" }` 并收到 `{ "type": "pong" }`。这里没有
Socket.IO 帧、事件协商或降级传输。

## MailDev 迁移边界

当前 MailDev 文档将 REST API 放在 `/api` 下，提供 `/api/email/summary` 和
`/api/email/delete`，读取详情时会标记已读，并使用 Socket.IO 事件。OwlMail
在这些方面有意采用不同设计。请对照
[MailDev 官方 REST 参考](https://github.com/maildev/maildev/blob/main/docs/rest.md)
和下表验证客户端，不要直接假设可以无缝替换。

| 范围 | 当前 MailDev | OwlMail |
|---|---|---|
| API 前缀 | `/api`，还可配置基础路径 | 无版本 `/email` 路由及 `/api/v1`，可统一挂载到 `-base-pathname` 下 |
| 列表结构 | MailDev 定义的邮件列表/摘要结构 | `{ total, limit, offset, emails }` 或 `{ ..., previews }` |
| 详情读取状态 | 获取详情即标记已读 | 只通过显式 `PATCH` 修改 |
| 批量删除 | `POST /api/email/delete` | `POST /email/batch/delete` 或 `DELETE /api/v1/emails/batch` |
| 实时协议 | Socket.IO，`newMail` / `deleteMail` | 原生 WebSocket，`new` / `delete` |
| 配置 | MailDev 当前 CLI/配置项 | OwlMail 已文档化参数和受支持的 MailDev 环境变量别名 |

迁移自动化客户端前：

1. 修改或代理 API 前缀；迁移时可继续使用 `MAILDEV_BASE_PATHNAME` 兼容别名。
2. 适配列表响应结构。
3. 将 Socket.IO 客户端替换为原生 WebSocket 客户端。
4. 需要时显式标记已读。
5. 在预发布环境验证删除、中继、附件、鉴权和错误路径。

更完整的功能比较见
[迁移白皮书](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)。
