# MCP 参考

OwlMail 0.10.0 通过兼容两个协议时代的 Streamable HTTP 与 `owlmail mcp-stdio`
提供同一套只读 MCP。MCP 是检查接口，不是邮箱管理 API。

## Transport

| 模式 | 启用或启动 | 端点与行为 |
|---|---|---|
| HTTP | `-mcp-enabled` 或 `OWLMAIL_MCP_ENABLED=true` | `/mcp` 或 `<base-pathname>/mcp`；现代无状态与旧版有状态客户端复用 Web Basic Auth 与 HTTPS |
| stdio | `owlmail mcp-stdio -mail-directory DIR` | 从已有目录读取已提交 EML；协议走 stdout，日志走 stderr |

## 协议兼容性

MCP 规范使用日期作为协议版本。社区常说的“MCP 2.0”和“MCP 1.x”指两个协议时代，
并不是官方语义版本；两个时代底层都使用 JSON-RPC 2.0。

| 时代 | 协议版本 | HTTP 行为 |
|---|---|---|
| 现代，常被称为“MCP 2.0” | `2026-07-28` | 使用 `server/discover` 与逐请求 `_meta`；仅使用无状态 `POST`，没有协议会话、独立 `GET` 或会话 `DELETE` |
| 旧版，常被称为“MCP 1.x” | `2025-11-25` 及更早的受支持修订 | 使用 `initialize` / `notifications/initialized`、有状态 `POST`、可选独立 `GET` 与会话 `DELETE` |

两个时代共用同一 HTTP 路径和工具目录。携带
`Mcp-Protocol-Version: 2026-07-28` 的请求进入现代 handler；旧版初始化与会话请求
继续进入现有有状态 handler。官方 SDK 会协商双方支持的最高版本。stdio transport
也可在同一进程中处理现代 discovery 与旧版 initialization。

旧版 HTTP 会话在 `-mcp-session-timeout` 后过期，默认 `30m`；该值在两个时代中
仍作为 `wait_for_email` 的等待上限。现代 HTTP 客户端关闭响应流时，请求取消会向下
传递。进程关闭最多等待 `-mcp-shutdown-timeout`，默认 `5s`。

## 工具

| 工具 | 输入 | 结果与限制 |
|---|---|---|
| `list_emails` | `from`、`to`、`date_from`、`date_to`、`read`、`sort_by`、`sort_order`、`offset`、`limit` | 紧凑分页；`limit` 默认 50，范围 1–1000 |
| `search_emails` | 必填 `query`，以及列表过滤器 | 在主题、纯文本与 HTML 中不区分大小写搜索 |
| `get_email` | 必填 `id`，可选 `include_html` | 独立详情；默认省略安全化 HTML |
| `get_email_source` | 必填 `id`，可选 `max_bytes` | 无损 base64 RFC 5322 source；默认 1 MiB，解码后最大 100 MiB |
| `list_attachments` | 必填 `id` | 文件名、类型、Content-ID、大小、SHA-256、存储元数据；不返回字节 |
| `get_latest_email` | 可选 `limit` | 按邮箱顺序返回最新 1–20 个摘要 |
| `wait_for_email` | 可选 `to`、`subject`、`text`、`timeout_seconds` | 只匹配新投递；事件驱动；默认 30 秒，最长 120 秒 |

`sort_by` 接受 `time`、`subject`、`from`、`size`；`sort_order` 接受 `asc`、
`desc`；日期格式为 `YYYY-MM-DD`。每个 wait 过滤器最多 1024 字节；旧版或 stdio
会话最多 4 个并发 wait，现代 HTTP 的每个请求使用独立配额范围，全进程仍最多
64 个 wait。

`get_email_source.max_bytes` 计算解码后字节，因此返回的 base64 JSON 更大。结果包含
`returned_bytes`、完整 `size` 与 `truncated`。

## 资源

| URI | 内容边界 |
|---|---|
| `owlmail://inbox` | 最新 50 个紧凑摘要 |
| `owlmail://stats` | 总数、已读数、未读数 |
| `owlmail://email/{id}` | 独立详情，文本最多 32 KiB；省略 HTML、Header、source 与附件字节 |

## Prompts

| Prompt | 必填输入 | 可选输入 | 用途 |
|---|---|---|---|
| `registration_verification_email` | `recipient` | `subject`、`timeout_seconds` | 等待、检查并提取验证值，不修改邮件 |
| `password_reset_email` | `recipient` | `subject`、`timeout_seconds` | 等待、检查并提取重置值，不修改邮件 |
| `wait_for_delivery` | 无 | `recipient`、`subject`、`text`、`timeout_seconds` | 按可选收件人、主题或文本等待投递 |

`subject` 与 `text` 使用子串匹配。`timeout_seconds` 必须是 1 到服务有效上限之间的
整数；有效上限取配置的等待超时与 MCP 会话超时中的较小值。省略时，Prompt 使用
30 秒与该有效上限中的较小值。

所有 Prompt 只组合上述只读工具，不会增加权限。

## 浏览器 Origin 校验

HTTP 端点在每个请求上校验浏览器的 `Origin` 头，且与 Web Basic Auth 无关。规范
要求本地 HTTP 服务器做这项检查：没有它，开发者访问的任意页面都能通过 `/mcp`
读取测试邮箱——未启用认证时可直接读取，启用后也可以把自己控制的域名重绑定到
回环地址绕过仅比对 Host 的同源检查。

| 请求 | 结果 |
|---|---|
| 不带 `Origin` | 放行。`curl`、MCP SDK 的 HTTP 客户端和服务端到服务端的调用方都不会发送该头 |
| `Origin` 是 OwlMail 自身来源 | 放行。包括配置的 Web 主机与 Web 端口上的回环名称（按本监听器自身实际提供的 scheme），以及设置了 `-web-external-url` 时的该来源 |
| `Origin` 在 `-mcp-allowed-origins` 中 | 放行。逗号分隔的绝对 `http`/`https` 来源，与上述来源相加而非替换 |
| 其他 `Origin` | 返回 `403` 与纯文本原因 |

在 `/mcp` 上，该校验取代而非叠加于全局的 Web 来源守卫：那个守卫接受任何与请求
自身 `Host` 相同的 `Origin`，因此上述允许列表严格更窄，`-mcp-allowed-origins`
依然有效，且 `-web-allowed-origins` 不会打开该端点。

`-mcp-allowed-origins '*'` 供浏览器访问已由其他层控制的部署关闭该校验；它不能与
具体来源同时出现，因此一个笔误不会悄悄放宽一份收紧过的列表。

来源按浏览器序列化 `Origin` 的写法比较，因此 `https://host:443` 与
`https://host` 是同一个值；IPv6 字面量也会匹配其各种等价写法
（`https://[2001:0db8::1]` 与 `https://[2001:db8::1]` 是同一个来源）；Unicode 域名
也会匹配浏览器实际发送的 IDNA ASCII 来源（`https://例え.テスト` 与
`https://xn--r8jz45g.xn--zckzah` 是同一个来源）。端口按数值比较、IP 按地址比较，
因此 `:0443` 与 `:443` 是同一个端口，`[::ffff:192.0.2.1]` 与
`[::ffff:c000:201]` 是同一个主机。配置成任一写法都能匹配。仅浏览器会归一化的
数字写法——例如带前导零的 IPv4 `127.0.0.01`，或带 zone 的地址——按字面比较，
请按浏览器实际发送的形式配置。启动日志
按该比较形式打印已配置的来源，便于与被拒绝的请求对照。

被放行的来源会得到一份精确指名该来源的 CORS 策略，而不是未启用认证的其余开发
API 仍会返回的 `Access-Control-Allow-Origin: *`。只放行而不给出这些响应头，请求
虽然能到达 handler，浏览器却仍会拒绝把响应交给客户端，因此该端点自行承担完整
策略：

| 响应头 | 值 |
|---|---|
| `Access-Control-Allow-Origin` | 请求自身的来源，绝不使用通配符 |
| `Access-Control-Allow-Credentials` | `true`，使 Basic Auth 能从放行来源使用 |
| `Access-Control-Expose-Headers` | `Mcp-Session-Id, Mcp-Protocol-Version` |
| `Vary` | `Origin`，避免共享缓存把一个来源的响应发给另一个来源 |

该路径的**每个**响应都会设置 `Vary: Origin`，包括被拒绝的响应，避免共享缓存把
一个来源的结果复用给另一个来源。

来自放行来源的 `OPTIONS` 预检返回 `204`，附带 `GET, POST, DELETE, OPTIONS`
方法列表、MCP 所需请求头，以及十分钟的 `Access-Control-Max-Age`。Basic Auth
不会对预检发起质询——预检按设计不携带凭据；其他来源的预检仍然返回 `403` 且不
带任何 CORS 头。

在 `-mcp-allowed-origins '*'` 下，校验对所有浏览器上下文一并关闭，包括本地文件、
data URL 或沙箱文档发出的不透明 `Origin: null`。此时没有任何来源被担保，因此该端点返回朴素的
`Access-Control-Allow-Origin: *`，且**不**返回 `Access-Control-Allow-Credentials`。
回显调用方并允许携带凭据，会让这个"关闭校验"的选项比该端点原本所处的通配 CORS
授权更强——浏览器根本不允许通配符携带凭据。关闭校验不应该成为一次升级。

stdio 传输不监听端口，没有需要校验的 Origin。

## 明确不支持

MCP 不能删除邮件、修改已读状态、Relay/转发、下载附件字节、修改配置或重新加载邮箱。
测试确实需要写操作时使用原生 HTTP API，并尽量不要把写入凭据交给 Agent。

接入方式见 [AI Agent 测试](./AI-Agent-Testing.md)，会话、超时、外部 URL 与 stdio
刷新行为见[运维与排障](./Operations.md#供测试代理使用的只读-mcp)。
