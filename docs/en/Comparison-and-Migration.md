# OwlMail × MailDev × MailCatcher × Mailpit: Feature, API, and Migration Guide

> A source-based comparison for choosing a development mail server. It describes
> verified behavior, not drop-in compatibility.

**Review baseline:** 2026-09-03. The Mailpit column was added from a separate
source review on 2026-09-12 and is pinned to its own commit below.

- OwlMail: 0.10.0 release baseline at
  8d3445dd5a4c5c14f8d73b2841d38efcbb7e2c7f. Changes after the 0.9.0 tag and
  through this baseline are user-visible and are reflected in the rows below:
  browser `Origin` validation on the Web, API, WebSocket, and MCP surfaces,
  tightened mailbox file modes, and a configurable SMTPS listener port.
- MailDev: release candidate maildev@3.0.0-rc.3; main at
  9d4141f42b0acedfa544a306f96a5373ded8c8a3. The latest stable 2.x release is
  2.2.1 and differs materially from the 3.x codebase.
- MailCatcher: latest GitHub release v0.10.0; main declares 0.11.0 at
  43e488e2a5692532c131a87d5bd16a973ee8db56.
- Mailpit: reviewed at 0bbbb233db56b185035ec3d1730228506dbb8f04; its `go.mod`
  declares Go 1.26.0. The tag `v1.31.1` (94445c801111689305625395d3543c07c50c7af8)
  is an ancestor of that commit, which sits three commits after it and carries no
  tag of its own, so the reviewed tree is that release plus `56d501b`,
  `39351ee`, and `0bbbb23`. v1.31.1 is the newest tag in the repository.

All four projects can evolve. Pin the version used by development and CI, and
validate the exact build before migration.

Every Mailpit claim below was read out of that checkout — `config/config.go`
and `cmd/root.go` for flags and defaults, `server/` for routes and handlers,
`internal/` for storage and SMTP, and `CHANGELOG.md` and `README.md` for
release history. Where the source did not settle a question, the cell says the
behavior was not verified rather than guessing.

## Executive summary

The four projects accept development SMTP mail and expose it for inspection,
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
- **Mailpit** emphasizes breadth inside one Go binary: SMTP capture, a Vue web
  UI, a documented `/api/v1` REST surface, optional POP3 retrieval, message
  tagging and search, SMTP release and forwarding, and built-in HTML
  compatibility, link, and SpamAssassin checks.

OwlMail is not a universal drop-in replacement for any of them. In 0.10.0, its
optional MailDev REST facade covers the current MailDev REST contract, but does
not implement Socket.IO or the Node API. Its separate MailCatcher facade covers
a bounded subset of the messages API without emulating MailCatcher's
live-update protocol. There is no Mailpit facade: OwlMail implements none of
Mailpit's routes and makes no compatibility claim about them.

## Feature comparison

| Capability | OwlMail 0.10.0 | MailDev 3.0.0-rc.3 | MailCatcher main 0.11.0 | Mailpit main (post-v1.31.1) |
|---|---|---|---|---|
| Runtime | Go single binary with embedded Web assets | Node.js 20+, TypeScript monorepo and React UI | Ruby 3.3+, EventMachine/Sinatra | Go single binary with embedded Vue 3 and Bootstrap 5 assets |
| Primary strength | AI-assisted testing, recoverable storage, automation, and explicit resource limits | Interactive email inspection and integration breadth | Minimal Ruby/sendmail workflow | Breadth of built-in message inspection and checking in one binary |
| Configuration | Flat layered YAML/JSON with strict file-shape validation, environment aliases, CLI flags, and component startup checks | TypeScript/JavaScript application configuration and environment variables | Command-line configuration | CLI flags and `MP_*` environment variables; YAML files only for relay, forwarding, and tag rules |
| SMTP capture | SMTP, STARTTLS, and direct SMTPS | Configurable SMTP/TLS behavior | Intentionally simple SMTP server | SMTP with optional STARTTLS, a required-STARTTLS mode, and a required SSL/TLS mode |
| Message size | Configurable; 100 MiB default | Configurable; 50 MiB default on current main | No equivalent documented control | Configurable `--max-message-size`; 50 MiB default (the flag help says MB, the code multiplies by 1024×1024), `0` unlimited, and it also caps the `/api/v1/send` body |
| DATA concurrency | Configurable per process; 8 default, 0 unlimited | No equivalent documented process-wide limiter | No equivalent documented limiter | No equivalent process-wide limiter found in the reviewed source; `--smtp-max-recipients` bounds recipients per message at 100 |
| Persistence | Atomic EML commit, recovery, quarantine, and an optional SQLite mailbox index | Optional EML and attachment directory with restore | SQLite in-memory database | One SQLite database holding zstd-compressed raw messages; without `--database` the file is temporary and deleted on exit |
| Retention | Age, count, and local-disk limits | Maximum email count | Maximum message count | Message count (500 default) and message age; no local-disk limit |
| Attachments | Streaming staging; local or optional S3 with cached readiness probes | Local attachment files when persistence is enabled | Stored with the in-memory message database | Parsed on demand from the stored raw message, with an image thumbnail route; no remote object store |
| REST API | Native versioned and historical routes, plus default-off MailDev and MailCatcher facades | Current API below /api | Messages API below /messages | `/api/v1` routes with a generated `swagger.json` and an embedded Swagger UI; no MailDev or MailCatcher facade |
| Live updates | Native RFC 6455 WebSocket | Socket.IO | WebSocket with polling fallback | Native RFC 6455 WebSocket at `/api/events` |
| UI | Lightweight multilingual inbox with secure HTML isolation, responsive widths, tabs, history, and keyboard navigation | Rich React UI, source/header views, and responsive preview | Simple HTML/plain/source UI with keyboard navigation | Vue/Bootstrap UI with search filters, tagging, a dark theme, mobile preview, HTML compatibility, link and SpamAssassin checks, and HTML screenshots; no translation layer found in the reviewed source |
| MCP | Default-off Streamable HTTP and stdio with seven read-only tools, resources, and prompts | HTTP and stdio MCP with broader tools, resources, and prompts | No built-in MCP | No built-in MCP |
| Webhooks | Generic filters, templates, HMAC, retry, local outbox, and optional Redis Streams | No equivalent generic durable webhook pipeline | No built-in generic webhook pipeline | One `--webhook-url` receives a JSON POST per new message, rate limited and optionally delayed; no filters, signing, retry, or durable outbox |
| Relay | Native v1 routes use persistent asynchronous jobs with streaming DATA, explicit TLS modes, and bounded retry; historical and compatibility routes retain their existing non-job behavior | Manual and automatic outgoing SMTP relay | No comparable outgoing relay workflow | Synchronous release through `POST /api/v1/message/{ID}/release` with allow and block recipient patterns, plus auto-relay and a separate forwarding configuration |
| sendmail analogue | `owlmail sendmail` | No bundled equivalent documented | `catchmail` | `mailpit sendmail` plus a standalone sendmail binary built from the same tree |
| Observability | Public liveness/readiness, optional Prometheus metrics, and console or JSON logs | Health endpoint and application logging | Basic application logging | `livez` and `readyz` endpoints, a `mailpit readyz` CLI probe that can wait for readiness, and optional Prometheus metrics on the Web listener or a separate port |
| Embedding | No stable Go library surface; internal packages remain internal | Public Node API | Primarily a standalone Ruby command | No documented embedding surface; storage, SMTP, and POP3 live under `internal/` |
| Base path | Configurable URL prefix across Web, APIs, WebSocket, and MCP | Yes | Yes through http-path | Yes through `--webroot` |
| Authentication | Web Basic Auth; real SMTP AUTH; optional TLS requirement | Web and incoming SMTP credentials | Intended for trusted development use | Separate htpasswd files for Web/API, SMTP, POP3, and the Send API; optional UI TLS; an accept-any SMTP mode |
| Multi-instance mailbox | No shared mailbox database | No | No | `--tenant-id` prefixes every table and an `http(s)` database value is opened with the rqlite driver, so instances can share one database; concurrent-writer behavior was not verified here |

No cross-project performance ranking is claimed. Runtime language, binary size,
or a synthetic microbenchmark does not establish end-to-end behavior under MIME
parsing, disk pressure, TLS, S3, webhook, or browser workloads.

## Where Mailpit covers ground OwlMail does not

These are verified gaps in OwlMail 0.10.0, not concessions. A reader who needs
any of them should prefer Mailpit:

- **Message checks.** Mailpit scores HTML against a bundled caniemail
  database, checks links in HTML and text parts, and can score spamminess
  through a running SpamAssassin server. OwlMail implements none of these.
- **POP3 retrieval.** Mailpit can serve captured mail to a real mail client
  over POP3, with its own TLS certificate and credentials. OwlMail has no POP3
  listener.
- **Tagging and search filters.** Mailpit tags messages manually, by filter
  rules, from plus-addressing, from an `X-Tags` header, or from the
  authenticated SMTP username, and exposes tags through the API. OwlMail has no
  tagging concept.
- **Fault injection.** Mailpit's Chaos feature returns configurable SMTP
  errors for senders, recipients, and authentication at a set probability, so
  an application's retry path can be exercised. OwlMail has no equivalent.
- **A send API.** `POST /api/v1/send` composes and stores a message from JSON,
  under its own credentials, so test fixtures need no SMTP client. OwlMail
  offers no such route.
- **A shared database.** A tenant prefix plus an rqlite endpoint lets several
  Mailpit instances address one database. OwlMail's mailbox is local to a
  single instance by design.

OwlMail's counterweights are its transactional on-disk EML storage with
recovery and quarantine, its durable filtered webhook pipeline, its
asynchronous relay jobs that survive restart, optional S3 attachment storage,
an explicit SMTP DATA concurrency limit, a multilingual UI, and its read-only
MCP surface. Which set matters is a workload question, not a ranking.

## API and real-time compatibility

| Workflow | MailDev | OwlMail | MailCatcher | Mailpit |
|---|---|---|---|---|
| List | GET /api/email | Same path only when MailDev facade is enabled; native GET /api/v1/emails | GET /messages | GET /api/v1/messages |
| Compact list | GET /api/email/summary | Same path and shape only with the facade | No equivalent documented summary contract | No separate summary route; the list response already carries per-message summaries and mailbox totals |
| Detail | GET /api/email/:id and mark read | Facade preserves that side effect; native detail does not | GET /messages/:id.json | GET /api/v1/message/{ID}, which marks the message read; `latest` is accepted in place of an ID |
| HTML/text/source | MailDev-specific /api routes | Facade plus native versioned routes | /messages/:id.html, .plain and .source | /view/{ID}.html and /view/{ID}.txt render the parts; GET /api/v1/message/{ID}/raw and /headers return source and headers |
| Attachments | MailDev attachment route | Facade plus native attachment route | /messages/:id/parts/:cid | GET /api/v1/message/{ID}/part/{PartID}, plus /thumb for image thumbnails |
| Live events | Socket.IO | Native WebSocket, not Socket.IO | Project-specific WebSocket/polling | Native WebSocket at /api/events |
| Embedded API | Node MailDev class | None | None | None |

In OwlMail 0.10.0, enable the MailDev facade explicitly with
OWLMAIL_MAILDEV_REST_COMPAT=true or -maildev-rest-compat. It shares the normal
Basic Auth, HTTPS, storage, and base-path boundary. It does not enable Socket.IO.

Do not point a MailCatcher API client at OwlMail without an adapter. There is
no Mailpit facade at all, so a Mailpit API client must be rewritten rather than
repointed: the route prefixes, identifier placement, part addressing, and
event-stream path all differ. SMTP-only applications are much easier to migrate
because all four accept ordinary SMTP delivery.

## Browser-origin protection

Mailpit v1.31.1 added `--allowed-hosts` (also `MP_ALLOWED_HOSTS`), a
comma-separated allowlist of `Host` header values checked ahead of every other
check on each middleware-wrapped route, and its CHANGELOG files the change
under Security as mitigating DNS rebinding against the API. The `livez` and
`readyz` probes are registered without that middleware
(`server/server.go:75-76`), so the allowlist does not run for them. The reasoning in `server/cors.go`
matches OwlMail's own: the same-origin branch of a CORS check compares two
client-supplied headers, so an attacker who rebinds a name they control
satisfies both sides of the comparison unless the decision is anchored on a
value the operator declared. Mailpit always admits loopback names and raw IP
literals, because neither can be produced by a rebinding attacker.

OwlMail addresses the same class of problem at a narrower surface: its
read-only MCP HTTP endpoint validates the browser `Origin` header on every
request, independently of Web Basic Auth, with `-mcp-allowed-origins` naming
additional browser origins. Requests carrying no `Origin` header stay allowed,
because non-browser clients never send one.

The two controls are not interchangeable. Mailpit gates its middleware-wrapped
routes on `Host` — every route except the `livez` and `readyz` probes — and is
off until an operator sets it; OwlMail gates one endpoint on `Origin` and is on
by default. An OwlMail deployment that exposes the Web UI or
REST API beyond loopback still needs a network boundary in front of it, which
`--allowed-hosts` would partly supply for Mailpit.

## Agent integration

OwlMail 0.10.0 provides a default-off MCP endpoint at `/mcp` for a root
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

Mailpit has no built-in MCP endpoint either: a case-insensitive search of the
reviewed tree finds no MCP transport, tool, or resource registration. Its
`/api/v1` surface is well documented and its `latest` message alias is
convenient for agents, but reaching it from an MCP client requires a separate
adapter, and that adapter would be read-write unless it is bounded by hand.

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

Mailpit stores everything in one SQLite database: raw messages are compressed
with zstd at a configurable level, a search index and tags live alongside them,
and WAL mode is on unless `--disable-wal` is set for NFS-mounted files. When
`--database` is unset the database is a temporary file that is deleted on exit,
so the default configuration is deliberately not durable. When it is set, the
mailbox survives restart. Recovery from a partially written message is the
database engine's, not a separate quarantine contract.

None of the projects should be described as a horizontally shared,
database-backed production mailbox service. Mailpit's tenant prefix and rqlite
option come closest, and even there the reviewed source does not establish how
concurrent writers behave.

## Selection guide

Choose **OwlMail 0.10.0** when a single binary, ARM/cross-platform deployment,
AI-assisted integration testing, durable webhook automation, recoverable disk
storage, optional S3 attachments, SMTP resource controls, or a bounded
read-only agent surface matters most.

Choose **MailDev** when the richest interactive UI, Node embedding, exact
Socket.IO behavior, or its broader MCP workflow matters most.

Choose **MailCatcher** when the smallest familiar Ruby workflow and catchmail
integration are more valuable than persistence, relaying, webhooks, or agent
integration.

Choose **Mailpit** when HTML compatibility, link, or SpamAssassin checks, POP3
retrieval, message tagging, SMTP fault injection, a send API, or a
tenant-partitioned shared database matter more than OwlMail's transactional
disk storage, durable webhook pipeline, or read-only MCP surface.

For an existing MailDev deployment, enable the OwlMail REST facade only after
inventorying REST and live-event consumers. For MailCatcher, treat SMTP capture
and the sendmail analogue as the portable concepts; adapt every HTTP or
WebSocket integration. For Mailpit, only SMTP capture and the sendmail
analogue carry over; every REST and WebSocket integration has to be rewritten,
and the checks, tags, and POP3 access listed above have no destination in
OwlMail at all.

## Known OwlMail 0.10.0 boundaries at this baseline

- The native WebSocket endpoint is not Socket.IO.
- There is no public stable Go embedding SDK; `internal/` packages are not a
  supported embedding surface.
- Native v1 relay acceptance is asynchronous; with a persistent mail directory,
  status survives restarts with documented at-least-once recovery rather than
  exactly-once delivery.
- The mailbox, SQLite index, Webhook outbox, and Relay job state are local to
  one OwlMail instance; there is no shared multi-instance mailbox database.
- MCP is intentionally read-only and bounded. It does not expose attachment
  bytes or mutation, relay, and configuration tools.
- There is no Mailpit compatibility facade, and no OwlMail equivalent to
  Mailpit's HTML, link, and SpamAssassin checks, POP3 retrieval, message
  tagging, Chaos fault injection, or send API.

These are roadmap observations, not claims that MailDev, MailCatcher, or
Mailpit necessarily implement the same behavior.

## Primary sources

- OwlMail API reference: [docs/en/API-Reference.md](./API-Reference.md)
- OwlMail operations guide: [docs/en/Operations.md](./Operations.md)
- MailDev README: https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/README.md
- MailDev REST documentation: https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/docs/rest.md
- MailDev MCP documentation: https://github.com/maildev/maildev/blob/9d4141f42b0acedfa544a306f96a5373ded8c8a3/docs/mcp.md
- MailCatcher README: https://github.com/sj26/mailcatcher/blob/43e488e2a5692532c131a87d5bd16a973ee8db56/README.md
- MailCatcher version: https://github.com/sj26/mailcatcher/blob/43e488e2a5692532c131a87d5bd16a973ee8db56/lib/mail_catcher/version.rb
- Mailpit README: https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/README.md
- Mailpit CHANGELOG: https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/CHANGELOG.md
- Mailpit flags and defaults: https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/cmd/root.go and https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/config/config.go
- Mailpit HTTP routes: https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/server/server.go
- Mailpit Host allowlist: https://github.com/axllent/mailpit/blob/0bbbb233db56b185035ec3d1730228506dbb8f04/server/cors.go
