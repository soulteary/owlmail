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
origin; server-to-server clients that omit `Origin` are accepted. Use HTTPS
outside a trusted local development machine.

```bash
curl -u admin:secret http://localhost:1080/api/v1/emails
```

## Common conventions

- Email IDs are eight-character random strings by default. New messages use
  UUIDs when `-use-uuid-for-email-id` is enabled; both formats can be queried.
- `GET /api/v1/emails/:id` and `GET /email/:id` do **not** mark an email as
  read. Use the corresponding `PATCH` route explicitly.
- Collection and preview endpoints default to `limit=50` and `offset=0`. The
  maximum limit is 1000; invalid values fall back to their defaults.
- Timestamps are JSON-encoded Go `time.Time` values in RFC 3339 form.
- Successful mutations usually return `code`, `message`, and optional `data`.
  Errors return an HTTP error status plus `code`, `error`, and `message`.

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
      "from": [{ "address": "sender@example.com", "name": "Sender" }],
      "to": [{ "address": "recipient@example.com", "name": "" }]
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
| `sortBy` | `time`, `subject`, `from`, or `size`; time descending is the default when omitted |
| `sortOrder` | `asc` or `desc` |

Export routes accept the same filters. `ids=id1,id2` takes precedence and
exports only the listed IDs.

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

### Individual email

| Method and path | Purpose / response type |
|---|---|
| `GET /api/v1/emails/:id` | full email JSON |
| `DELETE /api/v1/emails/:id` | delete one email |
| `PATCH /api/v1/emails/:id/read` | mark one email as read |
| `GET /api/v1/emails/:id/html` | sanitized HTML, `text/html` |
| `GET /api/v1/emails/:id/source` | raw RFC 822 source, `text/plain` |
| `GET /api/v1/emails/:id/raw` | downloadable EML file |
| `GET /api/v1/emails/:id/attachments/:filename` | one decoded attachment |
| `POST /api/v1/emails/:id/actions/relay` | relay using the message recipients |
| `POST /api/v1/emails/:id/actions/relay/:relayTo` | relay to one explicit address |

Relay routes require outgoing SMTP configuration. A success response confirms
that OwlMail accepted the in-process relay request; it does **not** confirm that
the downstream SMTP server delivered the message. The API does not
syntactically validate `relayTo` before queueing it, and downstream failures are
reported in process logs after the HTTP response.

### Settings and system

| Method and path | Purpose |
|---|---|
| `GET /api/v1/settings` | runtime SMTP, web, outgoing, SMTP-auth, and TLS settings; secrets are omitted |
| `GET /api/v1/settings/outgoing` | current outgoing SMTP settings; password is omitted |
| `PUT /api/v1/settings/outgoing` | replace outgoing SMTP settings |
| `PATCH /api/v1/settings/outgoing` | update selected outgoing SMTP fields |
| `GET /api/v1/health` | unauthenticated liveness check |
| `GET /api/v1/version` | build/version information |
| `GET /api/v1/ws` | native WebSocket endpoint |

A release build returns version provenance similar to:

```json
{
  "version": "0.6.0",
  "commit": "<full Git commit SHA>",
  "build_date": "<UTC RFC 3339 timestamp>",
  "branch": "v0.6.0",
  "go_version": "go1.27.0",
  "platform": "linux/amd64",
  "compiler": "gc"
}
```

Release binaries and container images inject the first four values during the
build and smoke-test the version and commit. An ordinary local `go build` uses
development defaults for values that were not injected.

The outgoing settings body supports `host`, `port`, `user`, `password`,
`secure`, `autoRelay`, `autoRelayAddr`, `allowRules`, and `denyRules`. `host` is
required and `port` must be between 1 and 65535. Changes are in memory; they do
not rewrite the process flags or environment.

The `smtpAuth` object returned by the settings endpoint reflects configured
values only. Inbound SMTP authentication is not currently enforced; see
[Operations](./Operations.md#smtp-ingress-limits-and-authentication-status).

```bash
curl -u admin:secret \
  -H 'Content-Type: application/json' \
  -X PUT http://localhost:1080/api/v1/settings/outgoing \
  -d '{
    "host": "smtp.example.com",
    "port": 587,
    "user": "relay-user",
    "password": "relay-password",
    "secure": true
  }'
```

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

## MailDev migration boundary

Current MailDev documents its REST API under `/api`, provides
`/api/email/summary` and `/api/email/delete`, marks a message read when its detail
is fetched, and uses Socket.IO events. OwlMail intentionally differs in these
areas. See the [upstream MailDev REST reference](https://github.com/maildev/maildev/blob/main/docs/rest.md)
and validate clients against the following matrix rather than assuming a
drop-in replacement.

| Area | Current MailDev | OwlMail |
|---|---|---|
| API prefix | `/api` (plus optional base pathname) | unversioned `/email` routes and `/api/v1`; no configurable base pathname |
| list shape | MailDev-defined email list/summary shapes | `{ total, limit, offset, emails }` or `{ ..., previews }` |
| detail read state | detail fetch marks read | explicit `PATCH` only |
| batch delete | `POST /api/email/delete` | `POST /email/batch/delete` or `DELETE /api/v1/emails/batch` |
| live protocol | Socket.IO, `newMail` / `deleteMail` | native WebSocket, `new` / `delete` |
| configuration | MailDev's current CLI/config surface | documented OwlMail flags and supported MailDev environment aliases |

Before migrating an automated client:

1. Update or proxy the API prefix.
2. Adapt collection response parsing.
3. Replace Socket.IO code with a native WebSocket client.
4. Mark messages read explicitly when required.
5. Exercise delete, relay, attachment, authentication, and error paths in a
   staging environment.

For the broader comparison, see the
[migration white paper](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md).
