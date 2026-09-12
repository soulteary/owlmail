# MCP reference

OwlMail 0.9.0 exposes the same read-only MCP server through dual-era
Streamable HTTP and through `owlmail mcp-stdio`. MCP is an inspection surface,
not a mailbox administration API.

## Transports

| Mode | Enable or launch | Endpoint and behavior |
|---|---|---|
| HTTP | `-mcp-enabled` or `OWLMAIL_MCP_ENABLED=true` | `/mcp` or `<base-pathname>/mcp`; modern stateless and legacy stateful clients share Web Basic Auth and HTTPS |
| stdio | `owlmail mcp-stdio -mail-directory DIR` | Reads committed EML files from an existing directory; protocol on stdout, logs on stderr |

## Protocol compatibility

The MCP specification uses date-based protocol versions. The informal names
“MCP 2.0” and “MCP 1.x” refer to two protocol eras, not official semantic
versions. JSON-RPC remains version 2.0 in both eras.

| Era | Protocol versions | HTTP behavior |
|---|---|---|
| Modern, often called “MCP 2.0” | `2026-07-28` | `server/discover`, per-request `_meta`, stateless `POST`; no protocol session, standalone `GET`, or session `DELETE` |
| Legacy, often called “MCP 1.x” | `2025-11-25` and earlier supported revisions | `initialize` / `notifications/initialized`, stateful `POST`, optional standalone `GET`, and session `DELETE` |

Both eras use the same HTTP path and tool catalog. Requests carrying
`Mcp-Protocol-Version: 2026-07-28` use the modern handler; legacy initialization
and session requests retain the existing stateful handler. The official SDK
negotiates the highest mutually supported version. The stdio transport also
supports modern discovery and legacy initialization on the same process.

Legacy HTTP sessions expire after `-mcp-session-timeout` (default `30m`). The
same value remains an upper bound for `wait_for_email` in either era. Modern
HTTP request cancellation is propagated when the response stream closes.
Shutdown waits up to `-mcp-shutdown-timeout` (default `5s`).

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
1024 bytes. A legacy or stdio session may hold four waits. Each modern HTTP
request has an independent quota scope, while the process-wide limit remains
64 waits.

`get_email_source.max_bytes` counts decoded bytes, so the returned base64 JSON
is larger. The result includes `returned_bytes`, full `size`, and `truncated`.

## Resources

| URI | Content boundary |
|---|---|
| `owlmail://inbox` | 50 newest compact summaries |
| `owlmail://stats` | Total, read, and unread counts |
| `owlmail://email/{id}` | Detached detail with text capped at 32 KiB; omits HTML, headers, source, and attachment bytes |

## Prompts

| Prompt | Required input | Optional input | Purpose |
|---|---|---|---|
| `registration_verification_email` | `recipient` | `subject`, `timeout_seconds` | Wait, inspect, and extract a verification value without mutation |
| `password_reset_email` | `recipient` | `subject`, `timeout_seconds` | Wait, inspect, and extract a reset value without mutation |
| `wait_for_delivery` | none | `recipient`, `subject`, `text`, `timeout_seconds` | Wait for an optional recipient, subject, or text match |

`subject` and `text` are substring matches. `timeout_seconds` must be an integer
from 1 through the effective service maximum, which is the smaller of the
configured wait timeout and MCP session timeout. When it is omitted, the prompt
uses the smaller of 30 seconds and that effective maximum.

All prompts compose the read-only tools. They do not grant capabilities beyond
the tool list.

## Browser origin validation

The HTTP endpoint validates the browser `Origin` header on every request,
independently of Web Basic Auth. The specification requires this check for
local HTTP servers: without it any page a developer visits can read the test
mailbox through `/mcp`, either directly when the deployment is unauthenticated
or by re-binding an attacker-controlled hostname to the loopback address.

| Request | Outcome |
|---|---|
| No `Origin` header | Allowed. Non-browser clients such as `curl`, MCP SDK HTTP clients, and server-to-server callers never send it |
| `Origin` matching an OwlMail origin | Allowed. The configured Web host and the loopback names at the Web port, on the scheme this listener itself serves, plus `-web-external-url` when it is set |
| `Origin` listed in `-mcp-allowed-origins` | Allowed. Comma-separated absolute `http` or `https` origins, added to the origins above rather than replacing them |
| Any other `Origin` | `403` with a plain-text reason |

On `/mcp` this check replaces, rather than follows, the global Web origin guard:
that guard accepts any `Origin` which echoes the request's own `Host`, so the
allow list above is strictly narrower, `-mcp-allowed-origins` keeps working, and
`-web-allowed-origins` does not open this endpoint.

`-mcp-allowed-origins '*'` turns the check off for deployments that control
browser access at another layer; it cannot be combined with an explicit origin,
so a typo never silently widens a narrow list.

Origins are compared the way a browser serializes them, so `https://host:443`
and `https://host` are the same value, and an IPv6 literal matches whichever of
its equivalent spellings is configured (`https://[2001:0db8::1]` and
`https://[2001:db8::1]` are one origin), and a Unicode domain matches the IDNA
ASCII origin a browser sends (`https://例え.テスト` and
`https://xn--r8jz45g.xn--zckzah` are one origin). Ports are compared as numbers
and IP addresses as addresses, so `:0443` and `:443` are one port and
`[::ffff:192.0.2.1]` and `[::ffff:c000:201]` are one host. Either spelling may
be used. A numeric spelling that only a browser normalizes -- a leading-zero
IPv4 such as `127.0.0.01`, or a zoned address -- is compared as written, so
configure those in the form the browser sends.
Startup logs the configured origins in that compared form, so a refused request
can be checked against them.

An allowed origin is answered with a CORS policy naming it exactly, never the
`Access-Control-Allow-Origin: *` the rest of the unauthenticated development API
still returns. Allowing an origin without those response headers would let the
request reach the handler but leave the browser refusing to hand the response to
the client, so the endpoint owns the whole policy:

| Response header | Value |
|---|---|
| `Access-Control-Allow-Origin` | the request's own origin, never a wildcard |
| `Access-Control-Allow-Credentials` | `true`, so Basic Auth works from an allowed origin |
| `Access-Control-Expose-Headers` | `Mcp-Session-Id, Mcp-Protocol-Version` |
| `Vary` | `Origin`, so a shared cache cannot serve one origin's response to another |

`Vary: Origin` is set on every response from this path, refusals included, so
a shared cache cannot reuse one origin's outcome for another.

A preflight `OPTIONS` from an allowed origin is answered with `204`, the
`GET, POST, DELETE, OPTIONS` method list, the MCP request headers, and a
ten-minute `Access-Control-Max-Age`. Basic Auth does not challenge it, because a
preflight carries no credentials by design; a preflight from any other origin is
still refused with `403` and no CORS headers.

Under `-mcp-allowed-origins '*'` validation is off for every browser context,
including the opaque `Origin: null` a page sends from a local file, a data URL,
or a sandboxed document. No origin has been vouched for, so the endpoint
returns the plain `Access-Control-Allow-Origin: *` and **no**
`Access-Control-Allow-Credentials`. Echoing the caller with credentials would
make the opt-out a stronger grant than the wildcard CORS this endpoint used to
fall under, which browsers refuse to use with credentials at all; turning the
check off must not be an upgrade.

The stdio transport opens no listener and has no origin to validate.

## Explicitly unsupported

MCP cannot delete mail, change read state, relay or forward a message, download
attachment bytes, change configuration, or reload the mailbox. Use the native
HTTP API only when a test explicitly needs one of those operations, and keep
mutation credentials outside the agent when possible.

For setup patterns see [AI agent testing](./AI-Agent-Testing.md). Operational
timeouts, session cleanup, external URL validation, and stdio refresh behavior
are covered in [Operations](./Operations.md#read-only-mcp-for-test-agents).
