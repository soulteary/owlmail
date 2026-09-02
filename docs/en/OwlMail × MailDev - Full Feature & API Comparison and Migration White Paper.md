# OwlMail × MailDev × MailCatcher: Feature, API, and Migration Guide

> A source-based comparison for choosing a development mail server. It describes
> verified behavior, not drop-in compatibility.

**Review baseline:** 2026-09-02.

- OwlMail: release 0.6.0; reviewed main at 279571b62a5e4891f0a204837d8553b131b89b20.
- MailDev: release candidate maildev@3.0.0-rc.3; main at
  9d4141f42b0acedfa544a306f96a5373ded8c8a3. The latest stable 2.x release is
  2.2.1 and differs materially from the 3.x codebase.
- MailCatcher: latest GitHub release v0.10.0; main declares 0.11.0 at
  43e488e2a5692532c131a87d5bd16a973ee8db56.

All three projects can evolve. Pin the version used by development and CI, and
validate the exact build before migration.

## Executive summary

The three projects accept development SMTP mail and expose it for inspection,
but optimize for different workflows:

- **OwlMail** emphasizes a single Go binary, durable and recoverable local
  storage, optional S3 attachments, generic durable webhooks, a versioned API,
  and a default-off read-only MCP endpoint.
- **MailDev 3** emphasizes a rich React inspection experience, Node embedding,
  exact Socket.IO integration, a broad MCP workflow, and a configurable
  TypeScript application surface.
- **MailCatcher** emphasizes a small Ruby workflow, a simple browser inbox, and
  the catchmail sendmail analogue.

OwlMail is not a universal drop-in replacement for either project. On the reviewed main branch, its optional
MailDev REST facade covers the current MailDev REST contract, but does not
implement Socket.IO or the Node API. MailCatcher uses a different messages API
and live-update contract.

## Feature comparison

| Capability | OwlMail 0.6.0 | OwlMail reviewed main | MailDev 3.0.0-rc.3 | MailCatcher main 0.11.0 |
|---|---|---|---|---|
| Runtime | Go single binary with embedded Web assets | Same | Node.js 20+, TypeScript monorepo and React UI | Ruby 3.3+, EventMachine/Sinatra |
| Primary strength | Recoverable local storage and durable webhooks | Storage, automation, and broader integrations | Interactive email inspection and integration breadth | Minimal Ruby/sendmail workflow |
| SMTP capture | SMTP, STARTTLS and direct SMTPS | Same | Configurable SMTP/TLS behavior | Intentionally simple SMTP server |
| Message size | Fixed 1 MiB | Configurable; 100 MiB default | Configurable; 50 MiB default on current main | No equivalent documented control |
| DATA concurrency | No process-wide limiter | Configurable per process; 8 default, 0 unlimited | No equivalent documented process-wide limiter | No equivalent documented limiter |
| Persistence | Atomic EML commit, recovery and quarantine | Same | Optional EML and attachment directory with restore | SQLite in-memory database |
| Retention | Age, count and local-disk limits | Same | Maximum email count | Maximum message count |
| Attachments | Local decoded attachments | Streaming staging; local or optional S3 | Local attachment files when persistence is enabled | Stored with the in-memory message database |
| REST API | Native versioned and historical unversioned routes | Same, plus an optional MailDev REST facade | Current API below /api | Messages API below /messages |
| Live updates | Native RFC 6455 WebSocket | Same | Socket.IO | WebSocket with polling fallback |
| UI | Lightweight multilingual inbox | Same, plus secure HTML isolation and responsive widths | Rich React UI, source/header views and responsive preview | Simple HTML/plain/source UI with keyboard navigation |
| MCP | No built-in endpoint | Optional, default-off Streamable HTTP with seven read-only tools, resources and prompts | HTTP and stdio MCP with broader tools, resources and prompts | No built-in MCP |
| Webhooks | Generic filters, templates, HMAC, retry, local outbox and optional Redis Streams | Same | No equivalent generic durable webhook pipeline | No built-in generic webhook pipeline |
| Relay | Manual and automatic outgoing SMTP relay | Same | Manual and automatic outgoing SMTP relay | No comparable outgoing relay workflow |
| sendmail analogue | No bundled command | owlmail sendmail | No bundled equivalent documented | catchmail |
| Embedding | No stable Go library surface; internal packages remain internal | Same | Public Node API | Primarily a standalone Ruby command |
| Base path | No configurable URL prefix | Configurable URL prefix | Yes | Yes through http-path |
| Authentication | Web Basic Auth; SMTP credential settings are not enforced | Web Basic Auth; real SMTP AUTH; optional TLS requirement | Web and incoming SMTP credentials | Intended for trusted development use |
| Multi-instance mailbox | No shared mailbox database | Same | No | No |

No cross-project performance ranking is claimed. Runtime language, binary size,
or a synthetic microbenchmark does not establish end-to-end behavior under MIME
parsing, disk pressure, TLS, S3, webhook, or browser workloads.

## API and real-time compatibility

| Workflow | MailDev | OwlMail | MailCatcher |
|---|---|---|---|
| List | GET /api/email | Same path only when MailDev facade is enabled; native GET /api/v1/emails | GET /messages |
| Compact list | GET /api/email/summary | Same path and shape only with the facade | No equivalent documented summary contract |
| Detail | GET /api/email/:id and mark read | Facade preserves that side effect; native detail does not | GET /messages/:id.json |
| HTML/text/source | MailDev-specific /api routes | Facade plus native versioned routes | /messages/:id.html, .plain and .source |
| Attachments | MailDev attachment route | Facade plus native attachment route | /messages/:id/parts/:cid |
| Live events | Socket.IO | Native WebSocket, not Socket.IO | Project-specific WebSocket/polling |
| Embedded API | Node MailDev class | None | None |

On the reviewed OwlMail main branch, enable the MailDev facade explicitly with
OWLMAIL_MAILDEV_REST_COMPAT=true or -maildev-rest-compat. It shares the normal
Basic Auth, HTTPS, storage, and base-path boundary. It does not enable Socket.IO.

Do not point a MailCatcher API client at OwlMail without an adapter. SMTP-only
applications are much easier to migrate because all three accept ordinary SMTP
delivery.

## Agent integration

The reviewed OwlMail main branch provides a default-off MCP endpoint at /mcp
with seven closed-world, read-only tools: list, search, detached detail,
bounded base64 source, attachment metadata, latest-email lookup ordered by
receipt, and an event-driven bounded delivery wait. It also exposes bounded
inbox, statistics, and email resources plus registration-verification,
password-reset, and delivery prompts. It shares the Web listener and
authentication boundary and generates base-path-aware Web links. It
deliberately excludes deletion, read-state mutation, relay, configuration
changes, and attachment bytes.

MailDev 3 exposes a broader MCP server and supports both HTTP and stdio
workflows. Tool and payload names are not interchangeable with OwlMail. An
existing MailDev MCP client therefore requires an explicit compatibility check.

MailCatcher has no built-in MCP endpoint. Agents can use its HTTP API only
through a separate tool or adapter.

## Storage and reliability boundary

OwlMail commits attachments before the final EML marker, makes a message visible
only after the storage transaction completes, and quarantines incomplete or
unparseable recovery artifacts. Its optional S3 mode stores decoded attachments
remotely while retaining EML, metadata, transaction state, and the webhook
outbox locally.

MailDev can persist EML and attachments and restore them at startup, but its
storage model and operational guarantees are not the same as OwlMail's
transaction and quarantine contract.

MailCatcher stores messages in an in-memory SQLite database. Its message limit
bounds the active inbox, but it is not a durable archive.

None of the projects should be described as a horizontally shared,
database-backed production mailbox service.

## Selection guide

Choose the reviewed **OwlMail main** when a single binary, ARM/cross-platform deployment, durable
webhook automation, recoverable disk storage, optional S3 attachments, SMTP
resource controls, or a small read-only agent surface matters most.

Choose **MailDev** when the richest interactive UI, Node embedding, exact
Socket.IO behavior, configuration files, or its broader MCP workflow matters
most.

Choose **MailCatcher** when the smallest familiar Ruby workflow and catchmail
integration are more valuable than persistence, relaying, webhooks, or agent
integration.

For an existing MailDev deployment, enable the OwlMail REST facade only after
inventorying REST and live-event consumers. For MailCatcher, treat SMTP capture
and the sendmail analogue as the portable concepts; adapt every HTTP or
WebSocket integration.

## Known OwlMail main-branch gaps at this baseline

- The native WebSocket endpoint is not Socket.IO.
- There is no public stable Go embedding SDK or general application config file.
- SMTP read/write timeouts and the recipient count are still fixed defaults.
- Native relay acceptance is asynchronous and does not expose durable delivery
  status.
- The Web inbox does not yet provide complete browser-history and keyboard
  navigation semantics.
- The mailbox index remains in memory even when EML files are durable.
- There is no Prometheus metrics endpoint.

These are roadmap observations, not claims that MailDev or MailCatcher
necessarily implement the same behavior.

## Primary sources

- OwlMail API reference: [docs/en/API-Reference.md](./API-Reference.md)
- OwlMail operations guide: [docs/en/Operations.md](./Operations.md)
- MailDev README: https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/README.md
- MailDev REST documentation: https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/docs/rest.md
- MailDev MCP documentation: https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/docs/mcp.md
- MailCatcher README: https://github.com/sj26/mailcatcher/blob/43e488e2a5692532c131a87d5bd16a973ee8db56/README.md
- MailCatcher version: https://github.com/sj26/mailcatcher/blob/43e488e2a5692532c131a87d5bd16a973ee8db56/lib/mail_catcher/version.rb
