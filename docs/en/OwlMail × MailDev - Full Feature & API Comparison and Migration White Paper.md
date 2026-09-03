# OwlMail × MailDev × MailCatcher: Feature, API, and Migration Guide

> A source-based comparison for choosing a development mail server. It describes
> verified behavior, not drop-in compatibility.

**Review baseline:** 2026-09-03.

- OwlMail: release 0.8.0 at
  e3d2cfcaf5580a7d914d1d27142a9edf43eaf8e9; the reviewed main commit
  1ef3cb32063a12a73fdc68f8c434d7ba143cefc4 adds documentation only.
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

- **OwlMail** emphasizes a single Go binary, AI-assisted integration testing,
  durable and recoverable local storage, optional S3 attachments, generic
  durable webhooks, versioned and compatibility APIs, and default-off read-only
  MCP over Streamable HTTP or stdio.
- **MailDev 3** emphasizes a rich React inspection experience, Node embedding,
  exact Socket.IO integration, a broad MCP workflow, and a configurable
  TypeScript application surface.
- **MailCatcher** emphasizes a small Ruby workflow, a simple browser inbox, and
  the catchmail sendmail analogue.

OwlMail is not a universal drop-in replacement for either project. In 0.8.0,
its optional MailDev REST facade covers the current MailDev REST contract, but
does not implement Socket.IO or the Node API. Its separate MailCatcher facade
covers a bounded subset of the messages API without emulating MailCatcher's
live-update protocol.

## Feature comparison

| Capability | OwlMail 0.8.0 | MailDev 3.0.0-rc.3 | MailCatcher main 0.11.0 |
|---|---|---|---|
| Runtime | Go single binary with embedded Web assets | Node.js 20+, TypeScript monorepo and React UI | Ruby 3.3+, EventMachine/Sinatra |
| Primary strength | AI-assisted testing, recoverable storage, automation, and explicit resource limits | Interactive email inspection and integration breadth | Minimal Ruby/sendmail workflow |
| Configuration | Flat layered YAML/JSON with strict file-shape validation, environment aliases, CLI flags, and component startup checks | TypeScript/JavaScript application configuration and environment variables | Command-line configuration |
| SMTP capture | SMTP, STARTTLS, and direct SMTPS | Configurable SMTP/TLS behavior | Intentionally simple SMTP server |
| Message size | Configurable; 100 MiB default | Configurable; 50 MiB default on current main | No equivalent documented control |
| DATA concurrency | Configurable per process; 8 default, 0 unlimited | No equivalent documented process-wide limiter | No equivalent documented limiter |
| Persistence | Atomic EML commit, recovery, quarantine, and an optional SQLite mailbox index | Optional EML and attachment directory with restore | SQLite in-memory database |
| Retention | Age, count, and local-disk limits | Maximum email count | Maximum message count |
| Attachments | Streaming staging; local or optional S3 with cached readiness probes | Local attachment files when persistence is enabled | Stored with the in-memory message database |
| REST API | Native versioned and historical routes, plus default-off MailDev and MailCatcher facades | Current API below /api | Messages API below /messages |
| Live updates | Native RFC 6455 WebSocket | Socket.IO | WebSocket with polling fallback |
| UI | Lightweight multilingual inbox with secure HTML isolation, responsive widths, tabs, history, and keyboard navigation | Rich React UI, source/header views, and responsive preview | Simple HTML/plain/source UI with keyboard navigation |
| MCP | Default-off Streamable HTTP and stdio with seven read-only tools, resources, and prompts | HTTP and stdio MCP with broader tools, resources, and prompts | No built-in MCP |
| Webhooks | Generic filters, templates, HMAC, retry, local outbox, and optional Redis Streams | No equivalent generic durable webhook pipeline | No built-in generic webhook pipeline |
| Relay | Persistent asynchronous jobs, streaming DATA, explicit TLS modes, and bounded retry | Manual and automatic outgoing SMTP relay | No comparable outgoing relay workflow |
| sendmail analogue | `owlmail sendmail` | No bundled equivalent documented | `catchmail` |
| Observability | Public liveness/readiness, optional Prometheus metrics, and console or JSON logs | Health endpoint and application logging | Basic application logging |
| Embedding | No stable Go library surface; internal packages remain internal | Public Node API | Primarily a standalone Ruby command |
| Base path | Configurable URL prefix across Web, APIs, WebSocket, and MCP | Yes | Yes through http-path |
| Authentication | Web Basic Auth; real SMTP AUTH; optional TLS requirement | Web and incoming SMTP credentials | Intended for trusted development use |
| Multi-instance mailbox | No shared mailbox database | No | No |

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

In OwlMail 0.8.0, enable the MailDev facade explicitly with
OWLMAIL_MAILDEV_REST_COMPAT=true or -maildev-rest-compat. It shares the normal
Basic Auth, HTTPS, storage, and base-path boundary. It does not enable Socket.IO.

Do not point a MailCatcher API client at OwlMail without an adapter. SMTP-only
applications are much easier to migrate because all three accept ordinary SMTP
delivery.

## Agent integration

OwlMail 0.8.0 provides a default-off MCP endpoint at `/mcp` for a root
deployment and `<base-pathname>/mcp` when a base pathname is configured. Local
clients can instead launch `owlmail mcp-stdio -mail-directory DIR`; both
transports expose the same seven closed-world, read-only tools: list, search,
detached detail, bounded base64 source, attachment metadata, latest-email
lookup ordered by receipt, and an event-driven bounded delivery wait. They also
expose bounded inbox, statistics, and email resources plus
registration-verification, password-reset, and delivery prompts. The HTTP
transport shares the Web listener and authentication boundary, and generated
Web links honor the configured external origin and base path. Both transports
deliberately exclude deletion, read-state mutation, relay, configuration
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

Choose **OwlMail 0.8.0** when a single binary, ARM/cross-platform deployment,
AI-assisted integration testing, durable webhook automation, recoverable disk
storage, optional S3 attachments, SMTP resource controls, or a bounded
read-only agent surface matters most.

Choose **MailDev** when the richest interactive UI, Node embedding, exact
Socket.IO behavior, or its broader MCP workflow matters most.

Choose **MailCatcher** when the smallest familiar Ruby workflow and catchmail
integration are more valuable than persistence, relaying, webhooks, or agent
integration.

For an existing MailDev deployment, enable the OwlMail REST facade only after
inventorying REST and live-event consumers. For MailCatcher, treat SMTP capture
and the sendmail analogue as the portable concepts; adapt every HTTP or
WebSocket integration.

## Known OwlMail 0.8.0 boundaries at this baseline

- The native WebSocket endpoint is not Socket.IO.
- There is no public stable Go embedding SDK; `internal/` packages are not a
  supported embedding surface.
- Native relay acceptance is asynchronous; with a persistent mail directory,
  status survives restarts with documented at-least-once recovery rather than
  exactly-once delivery.
- The mailbox, SQLite index, Webhook outbox, and Relay job state are local to
  one OwlMail instance; there is no shared multi-instance mailbox database.
- MCP is intentionally read-only and bounded. It does not expose attachment
  bytes or mutation, relay, and configuration tools.

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
