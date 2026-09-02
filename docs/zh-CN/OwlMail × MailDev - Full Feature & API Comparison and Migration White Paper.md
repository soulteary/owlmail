# OwlMail × MailDev × MailCatcher：功能、API 与迁移指南

> 面向开发邮件服务器选型的源码级比较。本文记录已验证行为，不承诺无缝兼容。

**审查基线：2026-09-02。**

- OwlMail：正式版 0.6.0；已审查 main 为
  279571b62a5e4891f0a204837d8553b131b89b20。
- MailDev：候选版 maildev@3.0.0-rc.3；main 为
  9d4141f42b0acedfa544a306f96a5373ded8c8a3。最新稳定 2.x 为 2.2.1，
  与 3.x 主线架构存在明显差异。
- MailCatcher：GitHub 最新 Release 为 v0.10.0；main 已声明 0.11.0，
  审查提交为 43e488e2a5692532c131a87d5bd16a973ee8db56。

三个项目都会继续变化。开发和 CI 应固定版本，迁移前应按实际构建重新验证。

## 执行摘要

三者都能接收开发环境 SMTP 邮件并提供检查界面，但优化方向不同：

- **OwlMail** 侧重 Go 单二进制、可恢复的持久化、本地或 S3 附件、通用持久
  Webhook、版本化 API，以及默认关闭的只读 MCP。
- **MailDev 3** 侧重 React 邮件检查体验、Node 嵌入、真正的 Socket.IO、
  更完整的 MCP 工作流和 TypeScript 应用配置。
- **MailCatcher** 侧重简单 Ruby 工作流、轻量收件箱及 catchmail sendmail
  替代命令。

OwlMail 不是另外两者的通用无缝替代。已审查 main 上的可选 MailDev REST facade 能覆盖当前
MailDev REST 合约，但不实现 Socket.IO 或 Node API；MailCatcher 的 messages
API 和实时协议也不同。

## 功能对比

| 能力 | OwlMail 0.6.0 | OwlMail 已审查 main | MailDev 3.0.0-rc.3 | MailCatcher main 0.11.0 |
|---|---|---|---|---|
| 运行时 | Go 单二进制，内嵌 Web 资源 | 相同 | Node.js 20+、TypeScript monorepo、React | Ruby 3.3+、EventMachine/Sinatra |
| 核心优势 | 可恢复本地存储与持久 Webhook | 存储、自动化及更多集成能力 | 交互式邮件检查与集成广度 | 极简 Ruby/sendmail 工作流 |
| SMTP 捕获 | SMTP、STARTTLS、直接 SMTPS | 相同 | 可配置 SMTP/TLS | 简单 SMTP Server |
| 邮件大小 | 固定 1 MiB | 可配置，默认 100 MiB | 可配置，当前 main 默认 50 MiB | 未记录等价控制 |
| DATA 并发 | 无进程级限制 | 进程级可配置，默认 8，0 为无限制 | 未记录等价 DATA 限流 | 未记录等价限流 |
| 持久化 | EML 原子提交、恢复与 quarantine | 相同 | 可选 EML/附件目录并在启动时恢复 | SQLite 内存数据库 |
| 保留策略 | 按时间、数量和本地磁盘占用 | 相同 | 最大邮件数量 | 最大消息数量 |
| 附件 | 本地保存解码附件 | 流式 staging；本地或可选 S3 | 启用持久化时保存本地附件 | 随内存消息数据库保存 |
| REST API | 原生版本化及历史无版本路由 | 相同，另有可选 MailDev facade | 当前接口位于 /api | messages API 位于 /messages |
| 实时更新 | 原生 RFC 6455 WebSocket | 相同 | Socket.IO | WebSocket，浏览器可退化为轮询 |
| UI | 轻量多语言收件箱 | 相同，并增加安全 HTML 隔离和响应式宽度 | React UI、源码/Header 与响应式预览 | 简单 HTML/纯文本/源码视图及键盘导航 |
| MCP | 无内置端点 | 默认关闭的 Streamable HTTP，含七个只读工具、资源和 Prompts | HTTP 与 stdio，更丰富的工具、资源和 Prompts | 无内置 MCP |
| Webhook | 过滤、模板、HMAC、重试、本地 outbox、可选 Redis Streams | 相同 | 无等价通用持久 Webhook 管道 | 无内置通用 Webhook |
| Relay | 手动与自动出站 SMTP 中继 | 相同 | 手动与自动出站 SMTP 中继 | 无可比的出站中继流程 |
| sendmail 替代 | 无内置命令 | owlmail sendmail | 未记录内置等价命令 | catchmail |
| 嵌入能力 | 无稳定公共 Go SDK，internal 不是公共接口 | 相同 | 公共 Node API | 主要作为独立 Ruby 命令 |
| Base path | 无可配置 URL 前缀 | 支持可配置 URL 前缀 | 支持 | 通过 http-path 支持 |
| 鉴权 | Web Basic Auth；SMTP 凭据设置不强制验证 | Web Basic Auth；真实 SMTP AUTH；可选强制 TLS | Web 与入站 SMTP 凭据 | 面向可信开发环境 |
| 多实例共享邮箱 | 不支持 | 相同 | 不支持 | 不支持 |

本文不提供跨项目性能排名。运行语言、二进制大小或微基准不能代表 MIME 解析、
磁盘压力、TLS、S3、Webhook 下游和浏览器共同作用下的端到端性能。

## API 与实时兼容边界

| 工作流 | MailDev | OwlMail | MailCatcher |
|---|---|---|---|
| 列表 | GET /api/email | 仅开启 facade 后同路径；原生为 GET /api/v1/emails | GET /messages |
| 精简列表 | GET /api/email/summary | 仅 facade 保持同路径和形状 | 未记录等价 summary 合约 |
| 详情 | GET /api/email/:id，并标记已读 | facade 保留副作用；原生详情不标记 | GET /messages/:id.json |
| HTML/文本/源码 | MailDev 专用 /api 路径 | facade 与原生版本化路径 | /messages/:id.html、.plain、.source |
| 附件 | MailDev attachment 路径 | facade 与原生附件路径 | /messages/:id/parts/:cid |
| 实时事件 | Socket.IO | 原生 WebSocket，不是 Socket.IO | 项目专用 WebSocket/轮询 |
| 嵌入 API | Node MailDev 类 | 无 | 无 |

在已审查 OwlMail main 上，必须显式设置 OWLMAIL_MAILDEV_REST_COMPAT=true 或
-maildev-rest-compat 才会启用 OwlMail MailDev facade。它复用现有 Basic Auth、
HTTPS、存储和 base path，但不会启用 Socket.IO。

不要把 MailCatcher HTTP 客户端直接指向 OwlMail。仅使用 SMTP 的应用迁移更简单，
因为三者都接受普通 SMTP 投递。

## Agent 集成

已审查 OwlMail main 已提供默认关闭的 MCP：根路径部署使用 `/mcp`，配置 base
pathname 后使用 `<base-pathname>/mcp`。它包含七个封闭只读工具：列表、搜索、
独立详情快照、受限 base64 原始源码、附件元数据、按接收顺序取得最新邮件，以及
事件驱动且有界的投递等待。它还提供有界的收件箱、统计与单邮件资源，以及注册验证、
密码重置和投递等待 Prompts；生成的 Web 链接会保留 base path。它与 Web API 共用
监听器和鉴权边界，并明确不提供删除、已读修改、Relay、配置修改或附件二进制。

MailDev 3 的 MCP 范围更广，同时支持 HTTP 和 stdio。两者的工具名称和载荷不能
直接互换，已有 MailDev MCP 客户端仍需显式兼容验证。

MailCatcher 没有内置 MCP；Agent 只能通过单独的工具或适配器使用其 HTTP API。

## 存储与可靠性边界

OwlMail 在最终 EML 标记前提交附件，只在完整存储事务成功后将邮件暴露给 API，
并在启动恢复时隔离不完整或不可解析的文件。可选 S3 模式只远程保存解码附件；
EML、元数据、事务状态和 Webhook outbox 仍保留在本地。

MailDev 可保存并恢复 EML 和附件，但其存储模型与 OwlMail 的事务和 quarantine
保证并不相同。

MailCatcher 使用内存 SQLite。消息上限能限制活动收件箱，但它不是持久归档。

三者都不应被描述为支持水平扩展、共享数据库的生产邮箱系统。

## 选型建议

以下情况优先选择已审查的 **OwlMail main**：需要单文件部署、ARM/跨平台、持久 Webhook
自动化、磁盘异常恢复、可选 S3 附件、SMTP 资源控制或小型只读 Agent 接口。

以下情况优先选择 **MailDev**：需要更完整的交互 UI、Node 嵌入、精确
Socket.IO、配置文件或更广的 MCP 工作流。

以下情况优先选择 **MailCatcher**：熟悉 Ruby，且 catchmail 与最小部署流程
比持久化、Relay、Webhook 或 Agent 集成更重要。

从 MailDev 迁移时，应先盘点 REST 和实时客户端，再决定是否开启 facade。
从 MailCatcher 迁移时，应把 SMTP 捕获和 sendmail 替代视为可迁移概念，并对
HTTP 与 WebSocket 集成逐项适配。

## 本基线下 OwlMail main 仍有的缺口

- 原生 WebSocket 不是 Socket.IO。
- 没有稳定公共 Go 嵌入 SDK，也没有通用应用配置文件。
- SMTP 读写超时和收件人数仍使用固定默认值。
- 原生 Relay 仍为异步；配置持久邮件目录后状态可跨重启恢复，语义为“至少一次”而非
  “恰好一次”。
- Web 收件箱尚未提供完整浏览器历史与键盘导航语义。
- 即使 EML 已持久化，邮箱索引仍主要位于内存。
- 没有 Prometheus 指标端点。

以上是 OwlMail 路线观察，并不表示 MailDev 或 MailCatcher 一定实现同等能力。

## 主要源码

- OwlMail API：[docs/zh-CN/API-Reference.md](./API-Reference.md)
- OwlMail 运维：[docs/zh-CN/Operations.md](./Operations.md)
- [MailDev README](https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/README.md)
- [MailDev REST](https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/docs/rest.md)
- [MailDev MCP](https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/docs/mcp.md)
- [MailCatcher README](https://github.com/sj26/mailcatcher/blob/43e488e2a5692532c131a87d5bd16a973ee8db56/README.md)
- [MailCatcher 版本](https://github.com/sj26/mailcatcher/blob/43e488e2a5692532c131a87d5bd16a973ee8db56/lib/mail_catcher/version.rb)
