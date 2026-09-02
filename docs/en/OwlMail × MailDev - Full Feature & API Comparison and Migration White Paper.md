# OwlMail × MailDev: Feature, API, and Migration Guide

> A source-based comparison for users deciding whether and how to migrate.

**Review baseline:** 2026-08-29. This guide compares the OwlMail source tree
with the current [MailDev REST documentation](https://github.com/maildev/maildev/blob/main/docs/rest.md).
Both projects can evolve; validate the exact versions you deploy.

## Executive summary

OwlMail and MailDev solve the same central development problem: accept SMTP
mail, retain it for inspection, show it in a browser, and optionally relay it.
That overlap makes SMTP-only migrations straightforward in many environments.

OwlMail also provides a single Go binary, a versioned `/api/v1` surface, native
SMTPS, browser notifications, generic webhook forwarding, and embedded local
help. Those are OwlMail features, not proof of protocol equivalence.

OwlMail is **not an exact drop-in replacement for current MailDev**. Important
differences include API prefixes and response shapes, read-on-fetch behavior,
batch routes, WebSocket protocol and event names, configuration breadth, and
MailDev's MCP interface. Treat migration as a small integration change followed
by tests, not as a binary swap with a compatibility guarantee.

## Feature comparison

| Capability | MailDev | OwlMail | Migration note |
|---|---|---|---|
| SMTP capture | Yes | Yes, default port 1025 | Usually only the hostname changes |
| Browser inbox | Yes | Yes, default port 1080 | UI customizations are not portable |
| EML directory | Yes | Yes | Test an archive copy before switching |
| Individual relay | Yes | Yes | Requires outgoing SMTP settings |
| Automatic relay | Yes | Yes | OwlMail supports allow/deny JSON rules |
| Inbound SMTP auth | Yes | PLAIN/LOGIN; required with both credentials, otherwise NO AUTH | Configure both credentials when migrating an authentication boundary |
| Web Basic Auth | Yes | Yes | OwlMail health endpoints remain public |
| SMTP TLS / STARTTLS | Yes | Yes | Certificate paths must be readable |
| Direct SMTPS | Version-dependent | Yes, port 465 when SMTP TLS is enabled | OwlMail-specific behavior |
| REST API | Yes | Yes | Routes and payloads are not identical |
| Live updates | Socket.IO | Native WebSocket | Client code must change |
| Generic outgoing webhooks | Version-dependent | Yes | OwlMail supports templates, HMAC, retry, and filters |
| Browser notifications | UI/version-dependent | Opt-in per browser | Requires permission and a secure context |
| MCP server | Current MailDev provides one | No | Keep MailDev or add another integration if required |
| General JS/JSON config file | Current MailDev provides one | No general config file | OwlMail uses flags and environment variables; webhook targets use JSON |
| Configurable base pathname | Yes | Yes | OwlMail prefixes every UI, API, native WebSocket, compatibility, and Service Worker route |

No performance rating is included here because the repository does not contain
a reproducible cross-project benchmark. A compiled Go binary can simplify
deployment, but throughput and memory claims should be measured against the
actual workload, storage, TLS, and webhook downstreams.

## API compatibility boundary

### Current MailDev interface

Current MailDev documents routes below `/api`, including `/api/email`,
`/api/email/summary`, `/api/email/delete`, and `/api/config`. It marks a message
as read when `GET /api/email/:id` is called. Live events use Socket.IO and the
event names `newMail` and `deleteMail`.

### OwlMail interface

OwlMail exposes two surfaces:

- `/api/v1/*`: the preferred, versioned OwlMail API.
- Unversioned `/email`, `/config`, `/healthz`, and `/socket.io` paths retained
  for existing OwlMail clients and common MailDev-style workflows.

When `-base-pathname /owlmail` (or `OWLMAIL_BASE_PATHNAME=/owlmail`) is set,
prepend `/owlmail` to every path in this section. The compatible
`MAILDEV_BASE_PATHNAME` variable is also accepted. This changes route location,
not protocol: `/owlmail/socket.io` is still native RFC 6455 WebSocket.

The `/socket.io` OwlMail path is a native RFC 6455 WebSocket endpoint. Its name
does not make it Socket.IO compatible.

| Workflow | Current MailDev | OwlMail |
|---|---|---|
| list emails | `GET /api/email` | `GET /email` or `GET /api/v1/emails` |
| compact list | `GET /api/email/summary` | `GET /email/preview` or `GET /api/v1/emails/preview` |
| get detail | `GET /api/email/:id`, marks read | `GET /email/:id` or `GET /api/v1/emails/:id`, no read side effect |
| mark one read | implicit on detail | `PATCH /email/:id/read` or `PATCH /api/v1/emails/:id/read` |
| delete many | `POST /api/email/delete` | `POST /email/batch/delete` or `DELETE /api/v1/emails/batch` |
| reload directory | `GET /api/reloadMailsFromDirectory` | `GET /reloadMailsFromDirectory` or `POST /api/v1/emails/reload` |
| configuration | `GET /api/config` | `GET /config` or `GET /api/v1/settings` |
| health | `GET /api/healthz` | `GET /healthz` or `GET /api/v1/health` |
| live events | Socket.IO `newMail`, `deleteMail` | native WS `{type:"new"}`, `{type:"delete"}` |

Collection shapes also differ. OwlMail returns a pagination envelope such as
`{ "total": 3, "limit": 50, "offset": 0, "emails": [...] }`; clients must not
assume the MailDev list shape.

See the [OwlMail API reference](./API-Reference.md) for the complete route list,
payloads, status behavior, authentication, and WebSocket protocol.

## Configuration compatibility boundary

OwlMail accepts a documented subset of familiar `MAILDEV_*` environment
variables and gives explicitly supplied CLI flags priority. It also provides
`OWLMAIL_*` names for OwlMail-specific settings. This is a convenience layer,
not full compatibility with every current MailDev option.

Common direct mappings include:

| Purpose | MailDev-style variable accepted by OwlMail | OwlMail variable |
|---|---|---|
| SMTP port | `MAILDEV_SMTP_PORT` | `OWLMAIL_SMTP_PORT` |
| Web port | `MAILDEV_WEB_PORT` | `OWLMAIL_WEB_PORT` |
| Mail directory | `MAILDEV_MAIL_DIRECTORY` | `OWLMAIL_MAIL_DIR` |
| Web user | `MAILDEV_WEB_USER` | `OWLMAIL_WEB_USER` |
| Web password | `MAILDEV_WEB_PASS` | `OWLMAIL_WEB_PASSWORD` |
| Base pathname | `MAILDEV_BASE_PATHNAME` | `OWLMAIL_BASE_PATHNAME` |
| Outgoing host | `MAILDEV_OUTGOING_HOST` | `OWLMAIL_OUTGOING_HOST` |
| Incoming user | `MAILDEV_INCOMING_USER` | `OWLMAIL_SMTP_USER` |

Review the root README configuration table for the complete OwlMail-supported
set. MailDev features such as MCP or a general configuration file do not become
available merely because other `MAILDEV_*` names are recognized.

## Operational differences to plan for

### Web credentials

- Neither username nor password: Basic Auth is disabled.
- Username only: OwlMail generates a 32-character password and prints it once
  to stderr. It changes on restart.
- Password only: OwlMail uses `admin` as the username.
- Both: OwlMail uses the supplied pair.

Configure both values for stable automation. Use HTTPS when credentials travel
outside localhost.

### Webhook delivery pressure

OwlMail's webhook deliveries use a process-wide concurrency limit. The
recommended default is 8. Set `-webhook-max-concurrency 0` only when unlimited
delivery is intentional and downstream capacity has been verified. A finite
limit applies backpressure to new-message processing when every delivery slot is
busy; it prevents an unbounded population of goroutines waiting on slow targets.

See [Webhook Forwarding](./Webhook-Forwarding.md) and the
[scenario examples](../../examples/webhooks/README.md).

### Browser notifications

Notifications are off by default and enabled per browser from the inbox. Only
new live events produce notifications. HTTPS or a trusted localhost origin is
required, and browser permission can be revoked independently of OwlMail.

## Migration playbooks

### SMTP-only application

1. Start OwlMail on an unused host/port.
2. Point a staging application's SMTP host to OwlMail port 1025.
3. Send plain text, HTML, multipart, BCC, and attachment messages.
4. Verify envelope recipients and persisted EML files if persistence is used.
5. Switch the development environment after the checks pass.

### REST client

1. Inventory every method, path, query parameter, and expected response shape.
2. Choose `/api/v1` instead of depending on the unversioned aliases.
3. Update the base path and unwrap OwlMail's pagination envelope.
4. Replace detail-fetch read side effects with an explicit `PATCH` request.
5. Test not-found, validation, relay failure, and authentication responses.
6. Pin the OwlMail version used by CI or container deployments.

### Live-event client

1. Replace the Socket.IO library with a native WebSocket client.
2. Connect to `/api/v1/ws`.
3. Handle `connected`, `new`, and `delete` message types.
4. Add reconnect/backoff logic in the client.
5. If Basic Auth is enabled in a browser, serve the client from OwlMail's own
   origin or use a server-side bridge that omits `Origin`.

### EML archive

1. Back up the MailDev directory.
2. Start OwlMail against a copy, never the only archive.
3. Verify counts, HTML, source, and attachments for representative messages.
4. Keep the original until rollback is no longer required.

## Acceptance checklist

- SMTP plain text, HTML, Unicode, BCC, and attachments display correctly.
- API clients parse collection envelopes and explicit error codes.
- Read state changes only when the client requests it.
- Delete, batch, export, relay, and reload operations behave as expected.
- WebSocket reconnect and event handling are tested.
- Basic Auth, HTTPS, health checks, and same-origin behavior are understood.
- Webhook filters, signatures, retry, timeout, and concurrency are load-tested.
- Rollback preserves the original EML archive and prior configuration.

## Conclusion

Choose OwlMail when its Go deployment model, versioned API, native WebSocket,
webhooks, browser notifications, or built-in help fit the workflow. Keep or
choose MailDev when exact current MailDev REST, Socket.IO, MCP, base-path, or
configuration behavior is required. For mixed environments, a reverse proxy or
small adapter is safer than relying on undocumented equivalence.
