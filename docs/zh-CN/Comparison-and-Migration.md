# OwlMail × MailDev × MailCatcher × Mailpit：功能、API 与迁移指南

> 面向开发邮件服务器选型的源码级比较。本文记录已验证行为，不承诺无缝兼容。

**审查基线：2026-09-03。** Mailpit 一栏来自 2026-09-12 的独立源码审查，
其提交单独固定在下方。

- OwlMail：0.10.0 发布基线对应提交
  8d3445dd5a4c5c14f8d73b2841d38efcbb7e2c7f。从 0.9.0 标签到该基线的变化对用户可见，
  并已反映在下表中：Web、API、WebSocket 与 MCP 界面的浏览器 `Origin` 校验，
  收紧后的邮箱文件权限，以及可配置的 SMTPS 监听端口。
- MailDev：候选版 maildev@3.0.0-rc.3；main 为
  9d4141f42b0acedfa544a306f96a5373ded8c8a3。最新稳定 2.x 为 2.2.1，
  与 3.x 主线架构存在明显差异。
- MailCatcher：GitHub 最新 Release 为 v0.10.0；main 已声明 0.11.0，
  审查提交为 43e488e2a5692532c131a87d5bd16a973ee8db56。
- Mailpit：审查提交为 0bbbb233db56b185035ec3d1730228506dbb8f04，`go.mod` 声明
  Go 1.26.0。tag `v1.31.1`（94445c801111689305625395d3543c07c50c7af8）是该提交的
  祖先，该提交位于其后第三个提交且自身不带 tag，因此被审查的代码树即该版本加上
  `56d501b`、`39351ee`、`0bbbb23` 三个提交。v1.31.1 是仓库中最新的 tag。

四个项目都会继续变化。开发和 CI 应固定版本，迁移前应按实际构建重新验证。

下文关于 Mailpit 的每一条结论都来自该检出：`config/config.go` 与 `cmd/root.go`
对应参数与默认值，`server/` 对应路由与处理器，`internal/` 对应存储与 SMTP，
`CHANGELOG.md` 与 `README.md` 对应发布历史。源码无法确认的项目直接写明未验证，
而不是猜测。

## 执行摘要

四者都能接收开发环境 SMTP 邮件并提供检查界面，但优化方向不同：

- **OwlMail** 侧重 Go 单二进制、AI 辅助集成测试、可恢复的持久化、本地或 S3
  附件、通用持久 Webhook、版本化及兼容 API，以及默认关闭、通过 Streamable
  HTTP 或 stdio 使用的只读 MCP。
- **MailDev 3** 侧重 React 邮件检查体验、Node 嵌入、真正的 Socket.IO、
  更完整的 MCP 工作流和 TypeScript 应用配置。
- **MailCatcher** 侧重简单 Ruby 工作流、轻量收件箱及 catchmail sendmail
  替代命令。
- **Mailpit** 侧重单个 Go 二进制内的功能广度：SMTP 捕获、Vue Web UI、
  有文档的 `/api/v1` REST 接口、可选 POP3 取信、标签与搜索、SMTP release 与
  转发，以及内置的 HTML 兼容性、链接和 SpamAssassin 检查。

OwlMail 不是其中任何一个的通用无缝替代。0.10.0 的可选 MailDev REST facade 能覆盖
当前 MailDev REST 合约，但不实现 Socket.IO 或 Node API；独立的 MailCatcher
facade 只覆盖有限的 messages API，不模拟 MailCatcher 的实时协议。OwlMail 完全
没有 Mailpit facade：不实现 Mailpit 的任何路由，也不作任何兼容性承诺。

## 功能对比

| 能力 | OwlMail 0.10.0 | MailDev 3.0.0-rc.3 | MailCatcher main 0.11.0 | Mailpit main（v1.31.1 之后）|
|---|---|---|---|---|
| 运行时 | Go 单二进制，内嵌 Web 资源 | Node.js 20+、TypeScript monorepo、React | Ruby 3.3+、EventMachine/Sinatra | Go 单二进制，内嵌 Vue 3 与 Bootstrap 5 资源 |
| 核心优势 | AI 辅助测试、可恢复存储、自动化和明确资源限制 | 交互式邮件检查与集成广度 | 极简 Ruby/sendmail 工作流 | 单个二进制内的邮件检查与校验功能广度 |
| 配置 | 扁平分层 YAML/JSON（严格校验文件结构）、环境变量兼容别名、CLI 及组件启动检查 | TypeScript/JavaScript 应用配置及环境变量 | 命令行配置 | CLI 参数与 `MP_*` 环境变量；只有 relay、转发和标签规则使用 YAML 文件 |
| SMTP 捕获 | SMTP、STARTTLS、直接 SMTPS | 可配置 SMTP/TLS | 简单 SMTP Server | SMTP，可选 STARTTLS、强制 STARTTLS 模式与强制 SSL/TLS 模式 |
| 邮件大小 | 可配置，默认 100 MiB | 可配置，当前 main 默认 50 MiB | 未记录等价控制 | `--max-message-size` 可配置，默认 50 MiB（flag 帮助写作 MB，代码按 1024×1024 换算），`0` 表示不限制，同时限制 `/api/v1/send` 请求体 |
| DATA 并发 | 进程级可配置，默认 8，0 为无限制 | 未记录等价 DATA 限流 | 未记录等价限流 | 被审查源码中未发现进程级限流；`--smtp-max-recipients` 将单封收件人数限制为 100 |
| 持久化 | EML 原子提交、恢复、quarantine 及可选 SQLite 邮箱索引 | 可选 EML/附件目录并在启动时恢复 | SQLite 内存数据库 | 单个 SQLite 数据库保存 zstd 压缩后的原始邮件；未设置 `--database` 时使用退出即删除的临时文件 |
| 保留策略 | 按时间、数量和本地磁盘占用 | 最大邮件数量 | 最大消息数量 | 按邮件数量（默认 500）和邮件时间；没有本地磁盘占用上限 |
| 附件 | 流式 staging；本地或可选 S3，并提供缓存式 readiness 探测 | 启用持久化时保存本地附件 | 随内存消息数据库保存 | 从已保存的原始邮件按需解析，并提供图片缩略图路由；没有远程对象存储 |
| REST API | 原生版本化及历史路由，以及默认关闭的 MailDev 和 MailCatcher facade | 当前接口位于 /api | messages API 位于 /messages | `/api/v1` 路由，附带生成的 `swagger.json` 与内嵌 Swagger UI；没有 MailDev 或 MailCatcher facade |
| 实时更新 | 原生 RFC 6455 WebSocket | Socket.IO | WebSocket，浏览器可退化为轮询 | 原生 RFC 6455 WebSocket，位于 `/api/events` |
| UI | 轻量多语言收件箱，包含安全 HTML 隔离、响应式宽度、标签页、历史和键盘导航 | React UI、源码/Header 与响应式预览 | 简单 HTML/纯文本/源码视图及键盘导航 | Vue/Bootstrap 界面，含搜索过滤、标签、深色主题、移动端预览、HTML 兼容性、链接与 SpamAssassin 检查以及 HTML 截图；被审查源码中未发现多语言层 |
| MCP | 默认关闭的 Streamable HTTP 与 stdio，含七个只读工具、资源和 Prompts | HTTP 与 stdio，更丰富的工具、资源和 Prompts | 无内置 MCP | 无内置 MCP |
| Webhook | 过滤、模板、HMAC、重试、本地 outbox、可选 Redis Streams | 无等价通用持久 Webhook 管道 | 无内置通用 Webhook | 单个 `--webhook-url` 在新邮件到达时收到一次 JSON POST，带限速和可选延迟；没有过滤、签名、重试或持久 outbox |
| Relay | 原生 v1 路由使用持久异步任务、流式 DATA、明确 TLS 模式和有界重试；历史与兼容路由保留原有的非任务行为 | 手动与自动出站 SMTP 中继 | 无可比的出站中继流程 | 通过 `POST /api/v1/message/{ID}/release` 同步投递，支持收件人允许与阻止正则，另有自动中继和独立的转发配置 |
| sendmail 替代 | `owlmail sendmail` | 未记录内置等价命令 | `catchmail` | `mailpit sendmail`，以及同一代码树构建的独立 sendmail 二进制 |
| 可观测性 | 公共 liveness/readiness、可选 Prometheus 指标、console 或 JSON 日志 | 健康端点与应用日志 | 基础应用日志 | `livez` 与 `readyz` 端点、可等待就绪的 `mailpit readyz` 命令，以及可挂在 Web 监听器或独立端口上的 Prometheus 指标 |
| 嵌入能力 | 无稳定公共 Go SDK，internal 不是公共接口 | 公共 Node API | 主要作为独立 Ruby 命令 | 无文档化嵌入接口；存储、SMTP 和 POP3 都位于 `internal/` |
| Base path | Web、API、WebSocket 与 MCP 均支持可配置 URL 前缀 | 支持 | 通过 http-path 支持 | 通过 `--webroot` 支持 |
| 鉴权 | Web Basic Auth；真实 SMTP AUTH；可选强制 TLS | Web 与入站 SMTP 凭据 | 面向可信开发环境 | Web/API、SMTP、POP3 与 Send API 各自使用独立 htpasswd 文件；可选 UI TLS；提供接受任意 SMTP 凭据的模式 |
| 多实例共享邮箱 | 不支持 | 不支持 | 不支持 | `--tenant-id` 为所有表加前缀，数据库值为 `http(s)` 时使用 rqlite 驱动打开，因此多实例可共享同一数据库；并发写入行为未在此验证 |

本文不提供跨项目性能排名。运行语言、二进制大小或微基准不能代表 MIME 解析、
磁盘压力、TLS、S3、Webhook 下游和浏览器共同作用下的端到端性能。

## Mailpit 覆盖而 OwlMail 没有的部分

以下都是 OwlMail 0.10.0 已验证的缺口，不是客套话。需要其中任何一项的读者应当
选择 Mailpit：

- **邮件检查。** Mailpit 基于内置的 caniemail 数据给 HTML 打分，检查 HTML 与
  纯文本部分中的链接，并可通过运行中的 SpamAssassin 服务评估垃圾邮件评分。
  OwlMail 均未实现。
- **POP3 取信。** Mailpit 可以通过 POP3 把捕获的邮件交给真实邮件客户端，并有
  独立的 TLS 证书与凭据。OwlMail 没有 POP3 监听器。
- **标签与搜索过滤。** Mailpit 支持手动打标签，也支持按过滤规则、plus 地址、
  `X-Tags` 头或已认证的 SMTP 用户名自动打标签，并通过 API 暴露标签。OwlMail
  没有标签概念。
- **故障注入。** Mailpit 的 Chaos 功能可按设定概率对发件人、收件人和认证返回
  可配置的 SMTP 错误，用于验证应用的重试路径。OwlMail 没有等价能力。
- **发送 API。** `POST /api/v1/send` 用独立凭据从 JSON 组装并保存邮件，测试
  夹具不需要 SMTP 客户端。OwlMail 没有这样的路由。
- **共享数据库。** 租户前缀加 rqlite 端点可以让多个 Mailpit 实例指向同一数据库。
  OwlMail 的邮箱按设计属于单实例。

OwlMail 的对应优势是：带恢复与 quarantine 的事务式磁盘 EML 存储、可过滤的持久
Webhook 管道、可跨重启的异步 Relay 任务、可选 S3 附件、明确的 SMTP DATA 并发
上限、多语言 UI，以及只读 MCP 接口。哪一组更重要取决于工作负载，而不是排名。

## API 与实时兼容边界

| 工作流 | MailDev | OwlMail | MailCatcher | Mailpit |
|---|---|---|---|---|
| 列表 | GET /api/email | 仅开启 facade 后同路径；原生为 GET /api/v1/emails | GET /messages | GET /api/v1/messages |
| 精简列表 | GET /api/email/summary | 仅 facade 保持同路径和形状 | 未记录等价 summary 合约 | 没有独立的 summary 路由；列表响应已包含逐封摘要与邮箱总计 |
| 详情 | GET /api/email/:id，并标记已读 | facade 保留副作用；原生详情不标记 | GET /messages/:id.json | GET /api/v1/message/{ID}，并标记已读；ID 位置可写 `latest` |
| HTML/文本/源码 | MailDev 专用 /api 路径 | facade 与原生版本化路径 | /messages/:id.html、.plain、.source | /view/{ID}.html 与 /view/{ID}.txt 渲染正文；GET /api/v1/message/{ID}/raw 与 /headers 返回源码和 Header |
| 附件 | MailDev attachment 路径 | facade 与原生附件路径 | /messages/:id/parts/:cid | GET /api/v1/message/{ID}/part/{PartID}，图片另有 /thumb |
| 实时事件 | Socket.IO | 原生 WebSocket，不是 Socket.IO | 项目专用 WebSocket/轮询 | 原生 WebSocket，位于 /api/events |
| 嵌入 API | Node MailDev 类 | 无 | 无 | 无 |

在 OwlMail 0.10.0 中，必须显式设置 OWLMAIL_MAILDEV_REST_COMPAT=true 或
-maildev-rest-compat 才会启用 OwlMail MailDev facade。它复用现有 Basic Auth、
HTTPS、存储和 base path，但不会启用 Socket.IO。

不要把 MailCatcher HTTP 客户端直接指向 OwlMail。Mailpit 则完全没有 facade，
其 API 客户端必须重写而不是改地址：路由前缀、标识符位置、附件寻址和事件流路径
都不一样。仅使用 SMTP 的应用迁移更简单，因为四者都接受普通 SMTP 投递。

## 浏览器来源防护

Mailpit v1.31.1 新增了 `--allowed-hosts`（同时支持 `MP_ALLOWED_HOSTS`）：一个
逗号分隔的 `Host` 头允许列表，在每条经过中间件的路由上先于其他检查执行，其
CHANGELOG 把这项改动归入 Security，并说明用于缓解针对 API 的 DNS 重绑定。
`livez` 与 `readyz` 探针在注册时没有套用该中间件（`server/server.go:75-76`），
因此该允许列表对它们不生效。`server/cors.go` 中的理由
与 OwlMail 一致：CORS 的同源分支比较的是两个由客户端提供的头，除非把判断锚定在
运维人员声明过的值上，否则重绑定自己控制的域名的攻击者可以同时满足比较的两侧。
Mailpit 始终放行回环名称和裸 IP 字面量，因为二者都不可能由重绑定攻击产生。

OwlMail 在更窄的面上处理同一类问题：只读 MCP HTTP 端点在每个请求上独立于 Web
Basic Auth 校验浏览器 `Origin` 头，`-mcp-allowed-origins` 可追加浏览器来源。
不带 `Origin` 头的请求仍然放行，因为非浏览器客户端从不发送该头。

两种控制不可互换。Mailpit 按 `Host` 约束经过中间件的路由——即除 `livez` 与
`readyz` 探针之外的每条路由——但在运维人员设置之前不生效；OwlMail 按 `Origin`
约束单个端点，且默认开启。把 Web UI 或 REST API 暴露
到回环之外的 OwlMail 部署仍然需要前置网络边界，而 `--allowed-hosts` 能为 Mailpit
提供其中一部分。

## Agent 集成

OwlMail 0.10.0 提供默认关闭的 MCP：根路径部署使用 `/mcp`，配置 base pathname
后使用 `<base-pathname>/mcp`；本地客户端也可以运行
`owlmail mcp-stdio -mail-directory DIR`。两个 transport 提供相同的七个封闭只读
工具：列表、搜索、独立详情快照、受限 base64 原始源码、附件元数据、按接收顺序取得
最新邮件，以及事件驱动且有界的投递等待。它们还提供有界的收件箱、统计与单邮件
资源，以及注册验证、密码重置和投递等待 Prompts。HTTP transport 与 Web API
共用监听器和鉴权边界；生成的 Web 链接会保留外部地址与 base path。两个 transport
都明确不提供删除、已读修改、Relay、配置修改或附件二进制。

MailDev 3 的 MCP 范围更广，同时支持 HTTP 和 stdio。两者的工具名称和载荷不能
直接互换，已有 MailDev MCP 客户端仍需显式兼容验证。

MailCatcher 没有内置 MCP；Agent 只能通过单独的工具或适配器使用其 HTTP API。

Mailpit 同样没有内置 MCP：对被审查代码树做大小写不敏感检索，找不到任何 MCP
transport、工具或资源注册。它的 `/api/v1` 接口文档完整，`latest` 别名对 Agent
也很方便，但 MCP 客户端访问它需要单独的适配器，而这种适配器除非手工加以限制，
否则是可读写的。

## 存储与可靠性边界

OwlMail 在最终 EML 标记前提交附件，只在完整存储事务成功后将邮件暴露给 API，
并在启动恢复时隔离不完整或不可解析的文件。可选 S3 模式只远程保存解码附件；
EML、元数据、事务状态和 Webhook outbox 仍保留在本地。

MailDev 可保存并恢复 EML 和附件，但其存储模型与 OwlMail 的事务和 quarantine
保证并不相同。

MailCatcher 使用内存 SQLite。消息上限能限制活动收件箱，但它不是持久归档。

Mailpit 把全部数据放进单个 SQLite 数据库：原始邮件按可配置等级用 zstd 压缩，
搜索索引与标签同库保存，除非用 `--disable-wal` 适配 NFS 挂载，否则启用 WAL。
未设置 `--database` 时数据库是退出即删除的临时文件，因此默认配置刻意不持久；
设置之后邮箱可跨重启保留。写入中断后的恢复依赖数据库引擎本身，而不是独立的
quarantine 合约。

四者都不应被描述为支持水平扩展、共享数据库的生产邮箱系统。Mailpit 的租户前缀
与 rqlite 选项最接近，但即便如此，被审查源码也没有说明并发写入的行为。

## 选型建议

以下情况优先选择 **OwlMail 0.10.0**：需要单文件部署、ARM/跨平台、AI 辅助集成
测试、持久 Webhook 自动化、磁盘异常恢复、可选 S3 附件、SMTP 资源控制或有界
只读 Agent 接口。

以下情况优先选择 **MailDev**：需要更完整的交互 UI、Node 嵌入、精确
Socket.IO 或更广的 MCP 工作流。

以下情况优先选择 **MailCatcher**：熟悉 Ruby，且 catchmail 与最小部署流程
比持久化、Relay、Webhook 或 Agent 集成更重要。

以下情况优先选择 **Mailpit**：HTML 兼容性、链接或 SpamAssassin 检查、POP3
取信、邮件标签、SMTP 故障注入、发送 API 或按租户切分的共享数据库，比 OwlMail
的事务式磁盘存储、持久 Webhook 管道或只读 MCP 接口更重要。

从 MailDev 迁移时，应先盘点 REST 和实时客户端，再决定是否开启 facade。
从 MailCatcher 迁移时，应把 SMTP 捕获和 sendmail 替代视为可迁移概念，并对
HTTP 与 WebSocket 集成逐项适配。从 Mailpit 迁移时，只有 SMTP 捕获和 sendmail
替代能够平移，所有 REST 与 WebSocket 集成都必须重写，而上文列出的检查、标签和
POP3 访问在 OwlMail 中没有落点。

## 本基线下 OwlMail 0.10.0 的已知边界

- 原生 WebSocket 不是 Socket.IO。
- 没有稳定公共 Go 嵌入 SDK；`internal/` 包不是受支持的嵌入接口。
- 原生 v1 Relay 仍为异步；配置持久邮件目录后状态可跨重启恢复，语义为“至少一次”而非
  “恰好一次”。
- 邮箱、SQLite 索引、Webhook outbox 和 Relay 任务状态都属于单个 OwlMail
  实例；不提供共享的多实例邮箱数据库。
- MCP 有意保持只读和有界，不提供附件二进制，也不提供修改、Relay 或配置工具。
- 没有 Mailpit 兼容 facade，也没有与 Mailpit 的 HTML、链接和 SpamAssassin
  检查、POP3 取信、邮件标签、Chaos 故障注入或发送 API 等价的能力。

以上是 OwlMail 路线观察，并不表示 MailDev、MailCatcher 或 Mailpit 一定实现
同等能力。

## 主要源码

- OwlMail API：[docs/zh-CN/API-Reference.md](./API-Reference.md)
- OwlMail 运维：[docs/zh-CN/Operations.md](./Operations.md)
- [MailDev README](https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/README.md)
- [MailDev REST](https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/docs/rest.md)
- [MailDev MCP](https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/docs/mcp.md)
- [MailCatcher README](https://github.com/sj26/mailcatcher/blob/43e488e2a5692532c131a87d5bd16a973ee8db56/README.md)
- [MailCatcher 版本](https://github.com/sj26/mailcatcher/blob/43e488e2a5692532c131a87d5bd16a973ee8db56/lib/mail_catcher/version.rb)
- [Mailpit README](https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/README.md)
- [Mailpit CHANGELOG](https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/CHANGELOG.md)
- Mailpit 参数与默认值：[cmd/root.go](https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/cmd/root.go) 与 [config/config.go](https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/config/config.go)
- [Mailpit HTTP 路由](https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/server/server.go)
- [Mailpit Host 允许列表](https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/server/cors.go)
