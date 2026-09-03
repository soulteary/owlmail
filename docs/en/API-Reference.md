# OwlMail API Reference

This reference describes the HTTP and WebSocket API implemented by the current
OwlMail source tree. For new integrations, prefer the versioned `/api/v1`
routes. The unversioned routes remain available for existing OwlMail clients and
for common MailDev-style workflows, but they are not a promise of exact MailDev
protocol compatibility.

## Base URL and authentication

The default base URL is `http://localhost:1080`. JSON is used unless an endpoint
explicitly returns HTML, plain text, an attachment, an EML file, or a ZIP file.

Configure HTTP Basic Auth with `-web-user` / `OWLMAIL_WEB_USER` and
`-web-password` / `OWLMAIL_WEB_PASSWORD`. It is disabled when neither web
credential is configured. The effective behavior for partial credentials is:

| Configuration | Effective credentials |
|---|---|
| neither value | authentication disabled |
| username only | that username plus a generated 32-character password printed once to stderr |
| password only | username `admin` plus the configured password |
| both values | the configured username and password |

A generated password changes whenever OwlMail restarts. Configure both values
for stable credentials. Startup fails if that generated password cannot be
written to stderr, because no recoverable credential would remain. The health
endpoints remain unauthenticated. When Basic Auth is enabled, browser requests
carrying an `Origin` header and WebSocket upgrades must come from OwlMail's own
origin; this same-origin check still applies to the unauthenticated health
endpoints and can return plain-text `403`. Server-to-server clients that omit
`Origin` are accepted. Use HTTPS outside a trusted local development machine.

```bash
curl -u admin:secret http://localhost:1080/api/v1/emails
```

## OpenAPI 3.1 contract

The canonical, version-controlled contract is available as
[JSON](../../openapi/openapi.json) and [YAML](../../openapi/openapi.yaml).
The running server exposes the same read-only documents:

```bash
curl -u admin:secret http://localhost:1080/api/v1/openapi.json
curl -u admin:secret http://localhost:1080/api/v1/openapi.yaml
```

These contract endpoints follow the normal Basic Auth and browser same-origin
policy. Only `/api/v1/health` and `/api/v1/ready` are public within the
versioned API. With `-base-pathname=/owlmail`, use
`/owlmail/api/v1/openapi.json`; the returned `servers[0].url` is likewise
`/owlmail/api/v1`.

The contract covers only OwlMail's native `/api/v1` behavior. The unversioned
MailDev-style compatibility routes are intentionally excluded. CI parses both
serializations, checks that they are semantically equivalent, resolves every
local `$ref`, and compares every registered versioned method/path with the
contract so route additions and removals cannot silently drift.

## Common conventions

- Email IDs are eight-character random strings by default. New messages use
  UUIDs when `-use-uuid-for-email-id` is enabled; both formats can be queried.
- `GET /api/v1/emails/:id` and `GET /email/:id` do **not** mark an email as
  read. Use the corresponding `PATCH` route explicitly.
- When the optional MailDev REST facade is enabled, only
  `GET /api/email/:id` reproduces MailDev's read-on-fetch side effect.
- Collection and preview endpoints default to `limit=50` and `offset=0`. The
  maximum limit is 1000; invalid values return `400`.
- Timestamps are JSON-encoded Go `time.Time` values in RFC 3339 form.
- Successful mutations usually return `code`, `message`, and optional `data`.
  Handler-level API errors return an HTTP error status plus `code`, `error`,
  and `message`. Basic Auth and browser same-origin middleware reject requests
  with plain-text `401` or `403` responses before an API handler runs. When
  installed, the same-origin check runs before Basic Auth, so `403` does not
  imply that authentication succeeded.

Example collection response:

```json
{
  "total": 1,
  "limit": 50,
  "offset": 0,
  "emails": [
    {
      "id": "aB3dEfGh",
      "time": "2026-08-29T12:00:00Z",
      "read": false,
      "subject": "Welcome",
      "from": [{ "Address": "sender@example.com", "Name": "Sender" }],
      "to": [{ "Address": "recipient@example.com", "Name": "" }]
    }
  ]
}
```

Example error:

```json
{
  "code": "EMAIL_NOT_FOUND",
  "error": "EMAIL_NOT_FOUND",
  "message": "Email not found"
}
```

## Filtering, pagination, and export

`GET /api/v1/emails`, `GET /email`, and both preview routes accept:

| Query | Meaning |
|---|---|
| `limit` | page size, default 50, maximum 1000 |
| `offset` | zero-based number of matching records to skip |
| `q` | case-insensitive substring search in subject, text, and HTML |
| `from` | case-insensitive substring search in sender address or name |
| `to` | case-insensitive substring search in recipient address or name |
| `dateFrom` | inclusive lower date boundary in `YYYY-MM-DD` form |
| `dateTo` | inclusive upper date boundary in `YYYY-MM-DD` form |
| `read` | `true` or `false` |
| `sortBy` | `time`, `subject`, `from`, `size`, or `store`; time descending is the default when omitted |
| `sortOrder` | `asc` or `desc` |

Malformed, negative, out-of-range, or unknown query values return `400`
instead of being silently replaced by defaults.

Export routes accept the same filters. `ids=id1,id2` takes precedence and
exports only the listed IDs.

The compact `preview` string is truncated after 200 UTF-8 bytes, rather than
200 characters, and receives `...` when truncated.

## Versioned API

### Email collection

| Method and path | Purpose |
|---|---|
| `GET /api/v1/emails` | list full email objects with pagination |
| `GET /api/v1/emails/stats` | return total, unread, read, and per-date counts |
| `GET /api/v1/emails/preview` | list compact previews with pagination |
| `GET /api/v1/emails/export` | download matching messages as a ZIP of EML files |
| `DELETE /api/v1/emails` | delete every stored message |
| `PATCH /api/v1/emails/read` | mark every message as read |
| `DELETE /api/v1/emails/batch` | delete the IDs in a JSON request body |
| `PATCH /api/v1/emails/batch/read` | mark the IDs in a JSON request body as read |
| `POST /api/v1/emails/reload` | reload EML files from the configured mail directory |

Both batch routes accept:

```json
{ "ids": ["aB3dEfGh", "550e8400-e29b-41d4-a716-446655440000"] }
```

A batch is limited to 1000 IDs and larger requests return `413` before any
mailbox mutation occurs.

### Individual email

| Method and path | Purpose / response type |
|---|---|
| `GET /api/v1/emails/:id` | full email JSON |
| `DELETE /api/v1/emails/:id` | delete one email |
| `PATCH /api/v1/emails/:id/read` | mark one email as read |
| `GET /api/v1/emails/:id/html` | sanitized HTML, `text/html; charset=utf-8` |
| `GET /api/v1/emails/:id/source` | raw RFC 822 source, `text/plain; charset=utf-8` |
| `GET /api/v1/emails/:id/raw` | downloadable EML, `message/rfc822` |
| `GET /api/v1/emails/:id/attachments/:filename` | decoded bytes using the attachment metadata Content-Type |
| `GET /api/v1/emails/:id/actions/relay/preflight` | preview the rule-filtered SMTP envelope recipients before manual relay |
| `POST /api/v1/emails/:id/actions/relay` | relay using the message recipients |
| `POST /api/v1/emails/:id/actions/relay/:relayTo` | relay to one explicit address |
| `GET /api/v1/relay-jobs/:jobID` | inspect an asynchronous relay job |

Native v1 relay routes require outgoing SMTP configuration and return `202`
with an opaque job ID, a base-path-aware `statusUrl`, and the current state.
Poll that URL until the state is `succeeded` or `failed`. Failed jobs expose a
bounded `errorCategory`, never the raw downstream error. When a persistent
mail directory is configured, job records are atomically stored under
`.owlmail-meta/relay-jobs` before enqueueing; queued jobs are resubmitted after
restart. Connection and timeout failures retry up to three total attempts with
exponential backoff and bounded jitter; authentication and other classified
permanent failures become terminal immediately. The response exposes the
attempt count and next scheduled attempt without raw downstream errors.
Completed records expire after 24 hours. The status store keeps at most
1000 jobs; completed records have a one-minute post-completion protection
window and may be evicted under capacity pressure after that window. Active
jobs are never evicted. If all slots are active or protected, a new request
returns `503` with `Retry-After: 1`. Explicit `relayTo` values are limited to
1024 UTF-8 bytes before a status record is created. Persistence failures make
new relay requests return `503` rather than silently accepting volatile jobs.
Recovery is at least once: a crash after downstream acceptance but before the
terminal status commit can cause one duplicate attempt.
Historical `/email` aliases keep their existing response behavior; the opt-in
MailDev facade continues to wait for its relay attempt.

### Settings and system

| Method and path | Purpose |
|---|---|
| `GET /api/v1/settings` | runtime SMTP, web, outgoing, SMTP-auth, and TLS settings; secrets are omitted |
| `GET /api/v1/settings/outgoing` | current outgoing SMTP settings; password is omitted |
| `PUT /api/v1/settings/outgoing` | replace outgoing SMTP settings |
| `PATCH /api/v1/settings/outgoing` | update selected outgoing SMTP fields |
| `GET /api/v1/health` | unauthenticated liveness check |
| `GET /api/v1/ready` | unauthenticated cached dependency readiness check |
| `GET /api/v1/version` | build/version information |
| `GET /api/v1/ws` | native WebSocket endpoint |
| `GET /api/v1/openapi.json` | base-path-aware OpenAPI 3.1 JSON contract |
| `GET /api/v1/openapi.yaml` | base-path-aware OpenAPI 3.1 YAML contract |

### Optional service endpoints

These endpoints are registered only when their corresponding feature is
enabled. They follow the configured base pathname and Web Basic Auth policy.

| Method and path | Purpose |
|---|---|
| `GET /metrics` | Prometheus metrics when `-metrics-enabled` is set |
| `GET /mcp` | Open the optional standalone SSE stream for the MCP Streamable HTTP transport |
| `POST /mcp` | Send MCP JSON-RPC messages over the Streamable HTTP transport |
| `DELETE /mcp` | Terminate an MCP Streamable HTTP session |

Malformed WebSocket upgrade headers or handshake keys return a plain-text
`400` response before a WebSocket connection is established.

The liveness response is independent of remote storage. Readiness returns
HTTP `200` only when every enabled dependency is ready and HTTP `503` while the
cached S3 probe is checking or failing:

```json
{
  "status": "unready",
  "checks": {
    "attachment_store": {
      "status": "unready",
      "error_category": "permission",
      "checked_at": "2026-09-02T05:45:00Z"
    }
  }
}
```

The response never includes raw SDK errors, endpoints, access keys, secret
keys, or session tokens. A readiness request reads a background-refreshed cache
and does not synchronously contact S3.

A release build returns version provenance similar to:

```json
{
  "version": "0.9.0",
  "commit": "<full Git commit SHA>",
  "build_date": "<UTC RFC 3339 timestamp>",
  "branch": "v0.9.0",
  "go_version": "go1.27.0",
  "platform": "linux/amd64",
  "compiler": "gc"
}
```

Release binaries and container images inject the first four values during the
build and smoke-test the version and commit. An ordinary local `go build` uses
development defaults for values that were not injected.

The outgoing settings body supports `host`, `port`, `user`, `password`,
`tlsMode` (`plain`, mandatory `starttls`, or implicit `smtps`),
`insecureSkipVerify`, the six phase timeout fields, `autoRelay`,
`autoRelayAddr`, `allowRules`, and `denyRules`. The legacy `secure=true` field
selects `smtps` for MailDev compatibility. Certificates and hostnames are
verified by default; `insecureSkipVerify` is an explicit unsafe opt-out.
Credentials are rejected in `plain` mode, and `starttls` never falls back when
the extension or handshake is unavailable. `host` is required and `port` must
be between 1 and 65535. Changes are in memory; they do not rewrite process flags
or environment. In PATCH requests, rule lists must be arrays when present; use
an empty array to clear a list because `null` is not an update.

The `smtpAuth` object returned by the settings endpoint is `null` in NO AUTH
mode and reflects the configured username (never the password) when required
authentication is enabled. With both `OWLMAIL_SMTP_USER` and
`OWLMAIL_SMTP_PASSWORD` set, PLAIN/LOGIN authentication is enforced. Supplying
only one credential fails startup. `OWLMAIL_SMTP_AUTH_REQUIRE_TLS=true` makes
AUTH available only after STARTTLS or over SMTPS and requires SMTP TLS to be
enabled; it does not change anonymous delivery in NO AUTH mode. This startup
policy is not writable through the settings API. See
[Operations](./Operations.md#smtp-ingress-limits-and-authentication-modes).

Inbound DATA processing is limited to eight concurrent transactions per process
by default across SMTP, STARTTLS, and SMTPS. Set `OWLMAIL_SMTP_MAX_CONCURRENCY`
or `-smtp-max-concurrency`; `0` means unlimited. At capacity the SMTP client
receives retryable `451 4.3.2`, not an HTTP API error. This process-level startup
setting is not exposed or modified by the settings API.

```bash
curl -u admin:secret \
  -H 'Content-Type: application/json' \
  -X PUT http://localhost:1080/api/v1/settings/outgoing \
  -d '{
    "host": "smtp.example.com",
    "port": 587,
    "user": "relay-user",
    "password": "relay-password",
    "tlsMode": "starttls",
    "connectTimeout": "10s",
    "tlsHandshakeTimeout": "10s",
    "authTimeout": "10s",
    "envelopeTimeout": "10s",
    "dataTimeout": "30s",
    "quitTimeout": "5s"
  }'
```

## Optional MailDev REST facade

The current MailDev REST contract is available only when explicitly enabled:

```bash
owlmail -maildev-rest-compat
# equivalent environment setting:
OWLMAIL_MAILDEV_REST_COMPAT=true owlmail
```

The default is `false`; with the option off, the routes below return `404`.
`-base-pathname /owlmail` moves them below `/owlmail/api`. They share OwlMail's
listener, HTTPS, CORS/origin protection, and Basic Auth. As in MailDev,
`/api/healthz` remains unauthenticated. The facade delegates to the existing
mailbox, durable read metadata, transactional deletion, attachment store, and
outgoing relay worker; it does not create a second index or storage format.

| Method and path | MailDev-compatible result |
|---|---|
| `GET /api/email` | Full email JSON array; supports `skip`, optional `limit`, optional `sort=asc\|desc`, and field/dot-path filters |
| `GET /api/email/summary` | `{items,total,storeTotal,unread,skip,limit}`; defaults to 50, clamps to 200, and supports `search`, `sort`, and `unread=true` |
| `GET /api/email/:id` | Full MailDev DTO and, only here, persistently marks an unread message read |
| `DELETE /api/email/:id` | JSON boolean `true`; missing ID is `404 {"error":"Email was not found"}` |
| `POST /api/email/delete` | Accepts `{"ids":[...]}` and returns `{"deleted":[...],"notFound":[...]}` |
| `DELETE /api/email/all` | JSON boolean `true` |
| `PATCH /api/email/read-all` | Number of messages changed |
| `GET /api/email/:id/html` | HTML with CID sources rewritten to compatibility attachment URLs |
| `GET /api/email/:id/source` | Raw RFC 822 bytes |
| `GET /api/email/:id/download` | `message/rfc822` download named `<id>.eml` |
| `GET /api/email/:id/attachment/:filename` | Streamed attachment with its stored media type |
| `POST /api/email/:id/relay` | Relays to original recipients; JSON boolean `true` after the attempt succeeds |
| `POST /api/email/:id/relay/:relayTo` | Relays to an explicit validated recipient |
| `GET /api/config` | MailDev-shaped `version`, `smtpPort`, `isOutgoingEnabled`, and `outgoingHost` |
| `GET /api/healthz` | Public JSON boolean `true` |
| `GET /api/reloadMailsFromDirectory` | Reloads the configured directory and returns JSON boolean `true` |

Relay requests wait for the outgoing attempt so their success value matches
MailDev, but the HTTP wait follows request cancellation and is capped at 30
seconds; the facade continues to use OwlMail's existing relay worker.

Compatibility errors have the single-field shape `{"error":"..."}`. JSON,
HTML, source, EML, and attachment responses preserve their corresponding
Content-Type. The facade changes neither routes nor DTOs under `/api/v1`.

This option provides **REST compatibility only**. OwlMail does not expose a
Socket.IO server, namespace handshake, polling transport, or MailDev events such
as `newMail` and `deleteMail`. `/socket.io` remains OwlMail's historical native
RFC 6455 alias and is not part of this facade.

## Unversioned compatibility routes

These routes use names historically associated with MailDev and are retained
for existing OwlMail integrations. Prefer `/api/v1` for new code.

| Method and path | Versioned equivalent or purpose |
|---|---|
| `GET /email` | `GET /api/v1/emails` |
| `GET /email/:id` | `GET /api/v1/emails/:id` |
| `GET /email/:id/html` | `GET /api/v1/emails/:id/html` |
| `GET /email/:id/attachment/:filename` | `GET /api/v1/emails/:id/attachments/:filename` |
| `GET /email/:id/download` | `GET /api/v1/emails/:id/raw` |
| `GET /email/:id/source` | `GET /api/v1/emails/:id/source` |
| `DELETE /email/:id` | `DELETE /api/v1/emails/:id` |
| `DELETE /email/all` | `DELETE /api/v1/emails` |
| `PATCH /email/read-all` | `PATCH /api/v1/emails/read` |
| `PATCH /email/:id/read` | `PATCH /api/v1/emails/:id/read` |
| `POST /email/:id/relay` | `POST /api/v1/emails/:id/actions/relay` |
| `POST /email/:id/relay/:relayTo` | `POST /api/v1/emails/:id/actions/relay/:relayTo` |
| `GET /email/stats` | `GET /api/v1/emails/stats` |
| `GET /email/preview` | `GET /api/v1/emails/preview` |
| `POST /email/batch/delete` | batch delete using `{ "ids": [...] }` |
| `POST /email/batch/read` | batch mark-read using `{ "ids": [...] }` |
| `GET /email/export` | `GET /api/v1/emails/export` |
| `GET /socket.io` | native WebSocket, **not** the Socket.IO protocol |
| `GET /config` | `GET /api/v1/settings` |
| `GET /config/outgoing` | `GET /api/v1/settings/outgoing` |
| `PUT /config/outgoing` | `PUT /api/v1/settings/outgoing` |
| `PATCH /config/outgoing` | `PATCH /api/v1/settings/outgoing` |
| `GET /healthz` | `GET /api/v1/health` |
| `GET /readyz` | `GET /api/v1/ready` |
| `GET /reloadMailsFromDirectory` | reload the configured mail directory |

## WebSocket protocol

Both WebSocket paths implement standard RFC 6455 WebSockets. A successful
connection first receives:

```json
{ "type": "connected", "message": "WebSocket connection established" }
```

Server events are:

```json
{ "type": "new", "email": { "id": "aB3dEfGh", "subject": "Welcome" } }
```

```json
{ "type": "delete", "id": "aB3dEfGh" }
```

Clients may send `{ "type": "ping" }` and receive `{ "type": "pong" }`.
There is no Socket.IO framing, event negotiation, or fallback transport.

## Optional MailCatcher REST facade

Set `-mailcatcher-rest-compat` or `OWLMAIL_MAILCATCHER_REST_COMPAT=true` to
expose MailCatcher-style message routes. The facade is disabled by default and
follows OwlMail's base path and Web Basic Auth policy.

| Method and path | Purpose |
|---|---|
| `GET /messages` | list message metadata |
| `DELETE /messages` | delete all messages |
| `GET /messages/:id.json` | message metadata, formats, and attachments |
| `GET /messages/:id.html` | HTML body with CID URLs rewritten |
| `GET /messages/:id.plain` | plain-text body |
| `GET /messages/:id.source` | RFC 822 source |
| `GET /messages/:id.eml` | downloadable RFC 822 message |
| `GET /messages/:id/parts/*` | attachment or inline MIME part selected by the wildcard content ID path |
| `DELETE /messages/:id` | delete one message |

This is a REST migration aid. It does not emulate MailCatcher's WebSocket bus
or integer identifier allocation; clients must treat IDs as opaque strings.

## MailDev migration boundary

Current MailDev documents its REST API under `/api`, provides
`/api/email/summary` and `/api/email/delete`, marks a message read when its detail
is fetched, and uses Socket.IO events. OwlMail's opt-in REST facade covers the
documented HTTP contract, while native APIs and the live protocol intentionally
differ. See the [upstream MailDev REST reference](https://github.com/maildev/maildev/blob/main/docs/rest.md)
and validate non-REST clients rather than assuming a complete drop-in replacement.

| Area | Current MailDev | OwlMail |
|---|---|---|
| API prefix | `/api` (plus optional base pathname) | same when the facade is enabled; native routes remain available |
| list shape | MailDev-defined email list/summary shapes | same in the facade; native `/api/v1` uses OwlMail envelopes |
| detail read state | detail fetch marks read | same only at `/api/email/:id`; native detail remains side-effect free |
| batch delete | `POST /api/email/delete` | same in the facade; native batch routes remain available |
| live protocol | Socket.IO, `newMail` / `deleteMail` | native WebSocket, `new` / `delete` |
| configuration | MailDev's current CLI/config surface | documented OwlMail flags and supported MailDev environment aliases |

Before migrating an automated client:

1. Enable the REST facade for an unchanged MailDev REST client, or migrate new code to `/api/v1`.
2. Reuse `MAILDEV_BASE_PATHNAME` as an OwlMail base-path alias if needed.
3. Replace Socket.IO code with a native WebSocket client; the REST switch does not cover it.
4. When using native detail routes, continue to mark messages read explicitly.
5. Exercise delete, relay, attachment, authentication, and error paths in a
   staging environment.

For the broader comparison, see the
[migration white paper](./Comparison-and-Migration.md).
