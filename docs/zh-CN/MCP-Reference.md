# MCP 参考

OwlMail 0.8.0 通过可选的有状态 Streamable HTTP 与 `owlmail mcp-stdio`
提供同一套只读 MCP。MCP 是检查接口，不是邮箱管理 API。

## Transport

| 模式 | 启用或启动 | 端点与行为 |
|---|---|---|
| HTTP | `-mcp-enabled` 或 `OWLMAIL_MCP_ENABLED=true` | `/mcp` 或 `<base-pathname>/mcp`；复用 Web Basic Auth 与 HTTPS |
| stdio | `owlmail mcp-stdio -mail-directory DIR` | 从已有目录读取已提交 EML；协议走 stdout，日志走 stderr |

HTTP 会话在 `-mcp-session-timeout` 后过期，默认 `30m`；关闭最多等待
`-mcp-shutdown-timeout`，默认 `5s`。

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
`desc`；日期格式为 `YYYY-MM-DD`。每个 wait 过滤器最多 1024 字节，每会话最多
4 个并发 wait，每进程最多 64 个。

`get_email_source.max_bytes` 计算解码后字节，因此返回的 base64 JSON 更大。结果包含
`returned_bytes`、完整 `size` 与 `truncated`。

## 资源

| URI | 内容边界 |
|---|---|
| `owlmail://inbox` | 最新 50 个紧凑摘要 |
| `owlmail://stats` | 总数、已读数、未读数 |
| `owlmail://email/{id}` | 独立详情，文本最多 32 KiB；省略 HTML、Header、source 与附件字节 |

## Prompts

| Prompt | 必填输入 | 用途 |
|---|---|---|
| `registration_verification_email` | `recipient` | 等待、检查并提取验证值，不修改邮件 |
| `password_reset_email` | `recipient` | 等待、检查并提取重置值，不修改邮件 |
| `wait_for_delivery` | 无 | 按可选收件人、主题或文本等待投递 |

所有 Prompt 只组合上述只读工具，不会增加权限。

## 明确不支持

MCP 不能删除邮件、修改已读状态、Relay/转发、下载附件字节、修改配置或重新加载邮箱。
测试确实需要写操作时使用原生 HTTP API，并尽量不要把写入凭据交给 Agent。

接入方式见 [AI Agent 测试](./AI-Agent-Testing.md)，会话、超时、外部 URL 与 stdio
刷新行为见[运维与排障](./Operations.md#供测试代理使用的只读-mcp)。
