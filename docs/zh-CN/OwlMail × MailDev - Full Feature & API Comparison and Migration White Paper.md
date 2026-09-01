# OwlMail × MailDev：功能、API 与迁移指南

> 面向选型与迁移的源码级对比，而不是未经验证的兼容性承诺。

**审查基线：** 2026-08-29。本文对照当前 OwlMail 源码与
[MailDev 官方 REST 文档](https://github.com/maildev/maildev/blob/main/docs/rest.md)。
两个项目都可能继续变化，部署前仍应核对实际版本。

## 执行摘要

OwlMail 与 MailDev 的核心目标一致：接收 SMTP 邮件、保存以供检查、通过浏览器
展示，并按需中继。因此，只使用 SMTP 接收功能的环境通常迁移较简单。

OwlMail 还提供单一 Go 二进制、版本化 `/api/v1`、原生 SMTPS、浏览器通知、
通用 Webhook 转发和内嵌本地帮助。这些是 OwlMail 的自身能力，不代表协议等价。

OwlMail **不是当前 MailDev 的精确无缝替代品**。需要关注 API 前缀与响应结构、
读取详情时的已读副作用、批量路由、WebSocket 协议与事件名、配置覆盖范围，以及
MailDev 的 MCP 接口。迁移应被视为一次小型集成改造并配套测试，而不是直接替换
二进制并假设完全兼容。

## 功能比较

| 能力 | MailDev | OwlMail | 迁移说明 |
|---|---|---|---|
| SMTP 捕获 | 支持 | 支持，默认 1025 | 通常只需修改主机名 |
| 浏览器收件箱 | 支持 | 支持，默认 1080 | UI 定制不能直接移植 |
| EML 目录 | 支持 | 支持 | 切换前先用归档副本验证 |
| 单封中继 | 支持 | 支持 | 需要出站 SMTP 设置 |
| 自动中继 | 支持 | 支持 | OwlMail 支持 allow/deny JSON 规则 |
| 入站 SMTP 鉴权 | 支持 | PLAIN/LOGIN；两项凭据同时配置时强制，否则为 NO AUTH | 迁移鉴权边界时同时配置用户名和密码 |
| Web Basic Auth | 支持 | 支持 | OwlMail 健康检查仍公开 |
| SMTP TLS / STARTTLS | 支持 | 支持 | 证书路径必须可读 |
| 直接 SMTPS | 随版本而异 | 开启 SMTP TLS 时监听 465 | OwlMail 特有行为 |
| REST API | 支持 | 支持 | 路由和载荷不完全相同 |
| 实时更新 | Socket.IO | 原生 WebSocket | 客户端代码需要修改 |
| 通用出站 Webhook | 随版本而异 | 支持 | OwlMail 提供模板、HMAC、重试和过滤 |
| 浏览器通知 | 随 UI/版本而异 | 浏览器内按需开启 | 需要权限和安全上下文 |
| MCP 服务 | 当前 MailDev 提供 | 不提供 | 依赖时需保留 MailDev 或另行集成 |
| 通用 JS/JSON 配置文件 | 当前 MailDev 提供 | 无通用配置文件 | OwlMail 使用参数和环境变量；Webhook 目标使用 JSON |
| 可配置基础路径 | 支持 | 不支持 | 需要路径前缀时使用反向代理 |

本文不提供性能星级，因为仓库中没有可复现的跨项目基准。编译后的 Go 二进制
可以简化部署，但吞吐和内存应按实际邮件、存储、TLS 与 Webhook 下游测量。

## API 兼容边界

### 当前 MailDev 接口

当前 MailDev 把路由放在 `/api` 下，包括 `/api/email`、
`/api/email/summary`、`/api/email/delete` 和 `/api/config`。调用
`GET /api/email/:id` 会把邮件标记为已读；实时事件使用 Socket.IO，事件名为
`newMail` 和 `deleteMail`。

### OwlMail 接口

OwlMail 提供两个接口面：

- `/api/v1/*`：推荐的新集成接口。
- 无版本的 `/email`、`/config`、`/healthz` 和 `/socket.io`：保留给已有
  OwlMail 客户端及常见 MailDev 风格工作流。

OwlMail 的 `/socket.io` 是原生 RFC 6455 WebSocket。路径名称并不意味着兼容
Socket.IO。

| 工作流 | 当前 MailDev | OwlMail |
|---|---|---|
| 邮件列表 | `GET /api/email` | `GET /email` 或 `GET /api/v1/emails` |
| 精简列表 | `GET /api/email/summary` | `GET /email/preview` 或 `GET /api/v1/emails/preview` |
| 获取详情 | `GET /api/email/:id`，同时标记已读 | `GET /email/:id` 或 `GET /api/v1/emails/:id`，无已读副作用 |
| 标记单封已读 | 获取详情时隐式完成 | `PATCH /email/:id/read` 或 `PATCH /api/v1/emails/:id/read` |
| 批量删除 | `POST /api/email/delete` | `POST /email/batch/delete` 或 `DELETE /api/v1/emails/batch` |
| 重载目录 | `GET /api/reloadMailsFromDirectory` | `GET /reloadMailsFromDirectory` 或 `POST /api/v1/emails/reload` |
| 配置信息 | `GET /api/config` | `GET /config` 或 `GET /api/v1/settings` |
| 健康检查 | `GET /api/healthz` | `GET /healthz` 或 `GET /api/v1/health` |
| 实时事件 | Socket.IO `newMail`、`deleteMail` | 原生 WS `{type:"new"}`、`{type:"delete"}` |

集合响应也不同。OwlMail 返回分页信封，例如
`{ "total": 3, "limit": 50, "offset": 0, "emails": [...] }`；客户端不能沿用
MailDev 列表结构的假设。

完整路由、载荷、状态行为、鉴权和 WebSocket 协议见
[OwlMail API 参考](./API-Reference.md)。

## 配置兼容边界

OwlMail 接受文档中列出的部分 `MAILDEV_*` 环境变量，显式 CLI 参数优先级更高；
OwlMail 特有设置还提供 `OWLMAIL_*` 名称。这是迁移便利层，并非支持当前
MailDev 的每个选项。

常见直接映射：

| 用途 | OwlMail 接受的 MailDev 风格变量 | OwlMail 变量 |
|---|---|---|
| SMTP 端口 | `MAILDEV_SMTP_PORT` | `OWLMAIL_SMTP_PORT` |
| Web 端口 | `MAILDEV_WEB_PORT` | `OWLMAIL_WEB_PORT` |
| 邮件目录 | `MAILDEV_MAIL_DIRECTORY` | `OWLMAIL_MAIL_DIR` |
| Web 用户名 | `MAILDEV_WEB_USER` | `OWLMAIL_WEB_USER` |
| Web 密码 | `MAILDEV_WEB_PASS` | `OWLMAIL_WEB_PASSWORD` |
| 出站主机 | `MAILDEV_OUTGOING_HOST` | `OWLMAIL_OUTGOING_HOST` |
| 入站用户名 | `MAILDEV_INCOMING_USER` | `OWLMAIL_SMTP_USER` |

完整支持范围以根 README 配置表为准。基础路径、MCP、通用配置文件等 MailDev
能力，不会因为 OwlMail 接受其他 `MAILDEV_*` 变量而自动可用。

## 需要规划的运行差异

### Web 凭据

- 用户名和密码都不设置：关闭 Basic Auth。
- 只设置用户名：生成 32 字符密码，只在 stderr 输出一次；重启后变化。
- 只设置密码：默认用户名为 `admin`。
- 两项都设置：使用明确配置的凭据。

自动化场景应同时配置两项。凭据离开 localhost 时应启用 HTTPS。

### Webhook 投递压力

OwlMail 对 Webhook 投递使用进程级并发上限，建议默认值为 8。只有在明确需要
无限并发且已验证下游容量时，才设置 `-webhook-max-concurrency 0`。有限上限在
所有槽位繁忙时对新邮件处理施加背压，避免慢下游造成无限等待 goroutine。

详见 [Webhook 消息转发](./Webhook-Forwarding.md)和
[场景示例](../../examples/webhooks/README.zh-CN.md)。

### 浏览器通知

通知默认关闭，由每个浏览器在收件箱内独立开启。只有实时到达的新消息才通知。
需要 HTTPS 或可信 localhost 来源，浏览器也可以独立撤销权限。

## 迁移手册

### 仅使用 SMTP 的应用

1. 在未占用的主机/端口启动 OwlMail。
2. 将预发布应用的 SMTP 主机指向 OwlMail 1025 端口。
3. 发送纯文本、HTML、多段、BCC 与附件邮件。
4. 使用持久化时核对信封收件人与 EML 文件。
5. 所有检查通过后再切换开发环境。

### REST 客户端

1. 盘点每个方法、路径、查询参数和预期响应结构。
2. 优先选择 `/api/v1`，不要新增对无版本别名的依赖。
3. 修改基础路径并解析 OwlMail 分页信封。
4. 将“读取详情即已读”改为显式 `PATCH`。
5. 验证未找到、参数错误、中继失败和鉴权响应。
6. 在 CI 或容器部署中固定 OwlMail 版本。

### 实时事件客户端

1. 用原生 WebSocket 客户端替换 Socket.IO 库。
2. 连接 `/api/v1/ws`。
3. 处理 `connected`、`new` 和 `delete` 类型。
4. 在客户端实现重连与退避。
5. 浏览器启用 Basic Auth 时，从 OwlMail 同源提供客户端；否则使用不携带
   `Origin` 的服务端桥接。

### EML 归档

1. 备份 MailDev 目录。
2. 让 OwlMail 先读取副本，绝不直接使用唯一归档。
3. 抽样核对数量、HTML、源码和附件。
4. 在不再需要回滚前保留原目录。

## 验收清单

- 纯文本、HTML、Unicode、BCC 与附件展示正确。
- API 客户端能解析集合信封和明确错误码。
- 只有客户端显式请求时才修改已读状态。
- 删除、批量、导出、中继和重载符合预期。
- WebSocket 重连和事件处理已经测试。
- 明确 Basic Auth、HTTPS、健康检查和同源行为。
- 对 Webhook 过滤、签名、重试、超时与并发做负载验证。
- 回滚方案保留原 EML 归档和旧配置。

## 结论

当 Go 部署模型、版本化 API、原生 WebSocket、Webhook、浏览器通知或内置帮助
符合需求时，可以选择 OwlMail；如果依赖当前 MailDev 的精确 REST、Socket.IO、
MCP、基础路径或配置行为，应继续使用 MailDev。混合环境中，反向代理或小型适配
层比依赖未文档化的“等价性”更安全。
