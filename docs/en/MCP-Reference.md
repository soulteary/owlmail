# MCP reference

OwlMail 0.8.0 exposes the same read-only MCP server through optional stateful
Streamable HTTP and through `owlmail mcp-stdio`. MCP is an inspection surface,
not a mailbox administration API.

## Transports

| Mode | Enable or launch | Endpoint and behavior |
|---|---|---|
| HTTP | `-mcp-enabled` or `OWLMAIL_MCP_ENABLED=true` | `/mcp` or `<base-pathname>/mcp`; shares Web Basic Auth and HTTPS |
| stdio | `owlmail mcp-stdio -mail-directory DIR` | Reads committed EML files from an existing directory; protocol on stdout, logs on stderr |

HTTP sessions expire after `-mcp-session-timeout` (default `30m`). Shutdown
waits up to `-mcp-shutdown-timeout` (default `5s`).

## Tools

| Tool | Inputs | Result and limits |
|---|---|---|
| `list_emails` | `from`, `to`, `date_from`, `date_to`, `read`, `sort_by`, `sort_order`, `offset`, `limit` | Compact page; `limit` defaults to 50 and is 1–1000 |
| `search_emails` | required `query`, plus list filters | Case-insensitive subject, plain-text, and HTML search |
| `get_email` | required `id`, optional `include_html` | Detached detail; sanitized HTML is omitted by default |
| `get_email_source` | required `id`, optional `max_bytes` | Lossless base64 RFC 5322 source; default 1 MiB, maximum 100 MiB decoded bytes |
| `list_attachments` | required `id` | Filename, content type, content ID, size, SHA-256, and storage metadata; no bytes |
| `get_latest_email` | optional `limit` | One to 20 newest compact summaries in mailbox order |
| `wait_for_email` | optional `to`, `subject`, `text`, `timeout_seconds` | New delivery only; event-driven; default 30 seconds, maximum 120 seconds |

`sort_by` accepts `time`, `subject`, `from`, or `size`; `sort_order` accepts
`asc` or `desc`. Date filters use `YYYY-MM-DD`. Every wait filter is limited to
1024 bytes. A session may hold four waits and the process may hold 64.

`get_email_source.max_bytes` counts decoded bytes, so the returned base64 JSON
is larger. The result includes `returned_bytes`, full `size`, and `truncated`.

## Resources

| URI | Content boundary |
|---|---|
| `owlmail://inbox` | 50 newest compact summaries |
| `owlmail://stats` | Total, read, and unread counts |
| `owlmail://email/{id}` | Detached detail with text capped at 32 KiB; omits HTML, headers, source, and attachment bytes |

## Prompts

| Prompt | Required input | Purpose |
|---|---|---|
| `registration_verification_email` | `recipient` | Wait, inspect, and extract a verification value without mutation |
| `password_reset_email` | `recipient` | Wait, inspect, and extract a reset value without mutation |
| `wait_for_delivery` | none | Wait for an optional recipient, subject, or text match |

All prompts compose the read-only tools. They do not grant capabilities beyond
the tool list.

## Explicitly unsupported

MCP cannot delete mail, change read state, relay or forward a message, download
attachment bytes, change configuration, or reload the mailbox. Use the native
HTTP API only when a test explicitly needs one of those operations, and keep
mutation credentials outside the agent when possible.

For setup patterns see [AI agent testing](./AI-Agent-Testing.md). Operational
timeouts, session cleanup, external URL validation, and stdio refresh behavior
are covered in [Operations](./Operations.md#read-only-mcp-for-test-agents).
