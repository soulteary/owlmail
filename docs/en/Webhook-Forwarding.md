# Webhook forwarding

OwlMail can send every matching new email to one or more HTTP endpoints. The feature is configured at startup, runs on the server even when no browser is open, and does not alter MailDev-compatible APIs.

## Build or import a configuration in the browser

Open `http://localhost:1080/webhooks`, or use the **Webhooks** button in the
inbox, to build a version 1 configuration. The embedded English/Chinese editor
can also import an existing JSON file by picker, drag and drop, or pasted text.
It validates the same documented target limits and can copy or download the
normalized result.

The editor is local-only: it does not upload the configuration and cannot
change the running server. After downloading the JSON, place it where OwlMail
can read it, select it with `-webhook-config` or
`OWLMAIL_WEBHOOK_CONFIG`, and restart OwlMail. Values such as
`${OWLMAIL_WEBHOOK_SECRET}` remain placeholders and must exist in the server's
environment at startup.

## Enable forwarding

The smallest valid configuration needs only a target name and an HTTP(S) URL:

```json
{
  "version": 1,
  "targets": [
    {
      "name": "local-receiver",
      "url": "http://127.0.0.1:18080/owlmail"
    }
  ]
}
```

Save it and pass it to OwlMail:

```bash
./owlmail -webhook-config ./webhooks.json -webhook-max-concurrency 8
```

The environment variable form is equivalent:

```bash
export OWLMAIL_WEBHOOK_CONFIG=./webhooks.json
export OWLMAIL_WEBHOOK_MAX_CONCURRENCY=8
```

If neither the flag nor environment variable is set, webhook forwarding is
disabled. The configuration is validated once at startup; restart OwlMail after
editing it.

The recommended default permits eight emails to be delivered concurrently.
Set `-webhook-max-concurrency 0` (or the environment variable to `0`) for
unlimited concurrency when all receivers are trusted, fast, and the incoming
load is controlled.

## Choose a runnable example

| Scenario | Configuration |
|---|---|
| Minimal forwarding with the default payload | [`examples/webhooks/minimal.json`](../../examples/webhooks/minimal.json) |
| Recipient and subject filtering | [`examples/webhooks/filtered-alerts.json`](../../examples/webhooks/filtered-alerts.json) |
| Authenticated custom JSON and HMAC | [`examples/webhooks/custom-json.json`](../../examples/webhooks/custom-json.json) |
| Archive plus filtered incident fan-out | [`examples/webhooks/multiple-targets.json`](../../examples/webhooks/multiple-targets.json) |
| Plain-text request body | [`examples/webhooks/plain-text.json`](../../examples/webhooks/plain-text.json) |
| Complete OwlMail + `soulteary/webhook` Compose stack | [`examples/webhooks/soulteary-webhook/`](../../examples/webhooks/soulteary-webhook/) |
| Most options in one target | [`examples/webhooks.json`](../../examples/webhooks.json) |

The [example walkthrough](../../examples/webhooks/README.md) includes a
loopback-only receiver, exact startup commands, and a test SMTP message.

## Configuration format

```json
{
  "version": 1,
  "targets": [
    {
      "name": "notifications",
      "url": "https://notify.example.com/hooks/owlmail",
      "method": "POST",
      "headers": {
        "Authorization": "Bearer ${OWLMAIL_WEBHOOK_TOKEN}"
      },
      "contentType": "application/json",
      "secret": "${OWLMAIL_WEBHOOK_SECRET}",
      "timeout": "5s",
      "retries": 2,
      "match": {
        "from": ["*@example.com"],
        "to": ["alerts@*"],
        "subject": ["*alert*", "*verification*"],
        "text": ["*code*"]
      },
      "bodyTemplate": "{\"title\":{{ json .Subject }},\"message\":{{ json .Text }}}"
    }
  ]
}
```

| Field | Required | Behavior |
|---|---:|---|
| `version` | No | Configuration version. Omitted and `1` both mean version 1. |
| `targets` | Yes | One to 32 destinations. Target names must be unique. |
| `name` | Yes | Safe identifier used in logs; maximum 100 UTF-8 bytes with no newline. The full URL and configured secrets are never logged. |
| `url` | Yes | Fixed `http` or `https` URL. User information and fragments are rejected; redirects are not followed. |
| `method` | No | `POST` by default; `POST`, `PUT`, and `PATCH` are accepted. |
| `headers` | No | Static request headers. Header names and values are validated; `Host` and `Content-Length` cannot be overridden. |
| `contentType` | No | `application/json` by default. An explicit `Content-Type` header takes precedence. |
| `secret` | No | Generates `X-OwlMail-Signature: sha256=<hex>` over the exact request body. |
| `timeout` | No | Per-attempt timeout; default `5s`, maximum `1m`. |
| `retries` | No | Extra attempts from zero to five. Only network errors, 408, 425, 429, and 5xx responses are retried. |
| `match` | No | Case-insensitive wildcard rules. See the rule semantics below. |
| `bodyTemplate` | No | Go text template for a custom body. When omitted, OwlMail sends the default event payload. |

`${VARIABLE}` placeholders are supported in `url`, header values, and `secret`. OwlMail fails at startup if a referenced variable is missing or empty, rather than silently sending an unauthenticated request.

Only the braced `${VARIABLE}` form is expanded; `$VARIABLE` is treated as
literal text. Environment expansion does not apply to `name`, `contentType`,
matching rules, or `bodyTemplate`.

### Validation and limits

- The configuration file is limited to 1 MiB, must contain exactly one JSON
  value, and rejects unknown fields.
- Version 1 accepts 1–32 uniquely named targets.
- A rendered request body is limited to 2 MiB. Oversized bodies fail before an
  HTTP request is made.
- Target timeout must be greater than zero and no more than one minute. The
  default is five seconds per attempt.
- `retries` counts extra attempts. For example, `2` means at most three total
  requests for that target.
- All targets and templates are compiled before SMTP and Web servers start, so
  a configuration error fails fast without partially enabling forwarding.

## Concurrency and backpressure

`-webhook-max-concurrency` limits concurrent email delivery jobs across all
targets. The default of `8` is appropriate for local development and small CI
environments. A useful starting estimate is peak emails per second multiplied
by the receiver's p95 response time in seconds, rounded up.

The limit is acquired before OwlMail creates the webhook handler goroutine. If
all slots are busy, the email is already persisted and event processing waits
for a slot; lightweight logging and WebSocket listeners are still started
first. SMTP `DATA` completion can therefore wait for a slot even though the
message has already been stored. This intentional backpressure prevents slow
receivers from creating an unbounded goroutine pile. Within one job, matching
targets are delivered sequentially.

Use `0` only when unlimited fan-out is intentional. It preserves the previous
behavior and can consume large amounts of memory and sockets during a burst.

## Matching rules

The `from`, `to`, `subject`, and `text` fields accept arrays of Go
shell-style patterns: `*`, `?`, character classes such as `[a-z]`, and `\`
escapes. As with Go's `path.Match`, `*` does not match `/`.

- Patterns inside one field are ORed.
- Different non-empty fields are ANDed.
- Empty fields match every email.
- Matching is case-insensitive.
- `to` considers envelope recipients as well as To, Cc, and Bcc headers.
- `from` considers parsed From addresses and the SMTP envelope sender.
- `text` matches the parsed plain-text body only; it does not search HTML or
  attachments. An HTML-only email normally has an empty text value.
- Matching by arbitrary headers is not supported. Use `.Headers` in a custom
  template when the receiver needs header values.

For example, this rule forwards alerts from any `example.com` sender only when their text contains a verification code:

```json
{
  "from": ["*@example.com"],
  "subject": ["*alert*", "*verification*"],
  "text": ["*code*"]
}
```

## Custom body templates

Templates receive these fields:

| Field | Type | Notes |
|---|---|---|
| `.ID`, `.Subject`, `.Text`, `.HTML` | string | Email identifier and content. |
| `.Time` | time | Parsed or received timestamp. |
| `.From`, `.To`, `.CC`, `.BCC` | string arrays | Parsed addresses without display names. |
| `.EnvelopeFrom`, `.EnvelopeTo` | string / string array | SMTP envelope addresses. |
| `.Size`, `.SizeHuman` | number / string | Stored message size. |
| `.Attachments`, `.AttachmentCount` | array / number | Safe attachment metadata; attachment bodies and local paths are not exposed. |
| `.Headers` | map | Parsed headers, available only to an explicit template. |

Available helpers are:

- `json VALUE`: JSON-encode a value so quotes and newlines cannot break the request body.
- `join STRINGS SEPARATOR`: join an address array.
- `truncate STRING LENGTH`: truncate by Unicode code points.

Templates use `missingkey=error`; a missing map key or template execution error
fails that target delivery. Parsed addresses contain mailbox addresses without
display names. `.Headers` may contain strings, arrays, or other parsed values,
so pass it through `json` instead of assuming every value is a string.

Always use `json` when placing email-controlled text into JSON:

```json
"bodyTemplate": "{\"title\":{{ json .Subject }},\"message\":{{ json (truncate .Text 4000) }}}"
```

Without a template, OwlMail sends:

```json
{
  "event": "email.received",
  "message": "Example\nMessage body",
  "email": {
    "id": "email-id",
    "time": "2026-08-29T12:00:00Z",
    "subject": "Example",
    "from": ["sender@example.com"],
    "to": ["recipient@example.com"],
    "envelopeFrom": "sender@example.com",
    "envelopeTo": ["recipient@example.com"],
    "text": "Message body",
    "html": "<p>Message body</p>",
    "size": 123,
    "sizeHuman": "123 B",
    "attachments": [
      {
        "fileName": "report.txt",
        "contentType": "text/plain",
        "size": 42
      }
    ],
    "attachmentCount": 1
  }
}
```

Empty optional arrays and strings such as `cc`, `bcc`, `html`, envelope fields,
and `attachments` are omitted. `headers` is intentionally excluded from the
default payload and is exposed only to explicit templates. Attachment bytes and
local storage paths are never included.

## Request headers

| Header | Value |
|---|---|
| `Content-Type` | Target `contentType`, defaulting to `application/json`; an explicit custom header wins. |
| `User-Agent` | `OwlMail-Webhook/1`, unless overridden by a custom header. |
| `X-OwlMail-Event` | Always `email.received`. |
| `X-OwlMail-Email-ID` | Exact OwlMail email ID; use it as an idempotency key. |
| `X-OwlMail-Signature` | Present only when `secret` is set; `sha256=` plus lowercase hex HMAC over the exact body bytes. |

Retries send the same body and email ID, so receivers should deduplicate before
performing a non-idempotent action.

## Send to `soulteary/webhook`

The runnable example starts both projects, validates HMAC, maps payload fields
to environment variables, and executes a visible demo command:

```bash
cd examples/webhooks/soulteary-webhook
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
docker compose up --build
```

Follow the [complete integration walkthrough](../../examples/webhooks/soulteary-webhook/README.md)
to send a test message and inspect the result.

For a manual deployment, use the hook URL exposed by
[`soulteary/webhook`](https://github.com/soulteary/webhook):

```json
{
  "version": 1,
  "targets": [
    {
      "name": "automation",
      "url": "http://webhook:9000/hooks/owlmail",
      "secret": "${OWLMAIL_WEBHOOK_SECRET}",
      "bodyTemplate": "{\"message\":{{ json .Text }},\"title\":{{ json .Subject }},\"emailId\":{{ json .ID }}}"
    }
  ]
}
```

One matching `soulteary/webhook` hook configuration is shown below. This is a
Go-template source file, so it is intentionally not valid JSON until
`soulteary/webhook -template` renders it:

```text
[
  {
    "id": "owlmail",
    "execute-command": "/app/notify.sh",
    "incoming-payload-content-type": "application/json",
    "http-methods": ["POST"],
    "pass-environment-to-command": [
      { "source": "payload", "name": "message", "envname": "OWLMAIL_MESSAGE" },
      { "source": "payload", "name": "title", "envname": "OWLMAIL_TITLE" },
      { "source": "payload", "name": "emailId", "envname": "OWLMAIL_EMAIL_ID" }
    ],
    "trigger-rule": {
      "match": {
        "type": "payload-hmac-sha256",
        "secret": "{{ getenv "OWLMAIL_WEBHOOK_SECRET" | js }}",
        "parameter": { "source": "header", "name": "X-OwlMail-Signature" }
      }
    }
  }
]
```

Start `soulteary/webhook` with its `-template` option when using `getenv` in the
hook file. Give both containers the same `OWLMAIL_WEBHOOK_SECRET` value.

## Delivery lifecycle and troubleshooting

1. OwlMail parses and stores the SMTP message first.
2. The `new` event starts the HTTP delivery asynchronously, so a slow or failed
   target does not change the stored message or eventual SMTP result. If the
   process-wide concurrency limit is saturated, SMTP acknowledgment can wait
   for a delivery slot as described above.
3. Targets matching one email are called sequentially in configuration order.
   Different email events may be delivered concurrently.
4. A 2xx response completes the target. Network errors, 408, 425, 429, and 5xx
   responses retry after approximately 100 ms, 200 ms, 400 ms, 800 ms, and
   1.6 s as needed. `Retry-After` is not interpreted.
5. Other 3xx/4xx responses fail immediately. Response bodies are not used.

Forwarding has no persistent delivery queue or dead-letter store. After the
configured attempts are exhausted, OwlMail keeps the email and logs the failure;
it does not retry that event after a restart. Process shutdown does not drain
in-flight webhook deliveries; see the
[shutdown guidance](./Operations.md#shutdown-and-delivery-guarantees).

For a local check, run the bundled receiver and the minimal configuration in
separate terminals:

```bash
go run ./examples/webhooks/receiver
go run ./cmd/owlmail -webhook-config ./examples/webhooks/minimal.json
```

When debugging:

- A startup error normally means invalid JSON, an unknown field, a missing
  `${VARIABLE}`, an invalid wildcard/duration, or a template parse error.
- No request usually means the target did not match. Temporarily remove `match`
  or start with `minimal.json`.
- In containers, `127.0.0.1` points back to OwlMail. Use the receiver's service
  name and container port.
- HTTP 401/403 usually means the custom token or HMAC secret differs. HMAC covers
  the exact body bytes; do not reformat the body before verifying it.
- Successful deliveries are logged at verbose level. Failures are logged with
  the safe target name, status, and attempt count, without the full URL or secret.

## Operational and security notes

- Webhook targets receive email content. Only configure destinations you trust.
- Prefer HTTPS outside a private container network and HMAC-sign every request.
- Keep the configuration file private (for example, mode `0600`) and put tokens in environment variables.
- A successful response is any 2xx status. Redirects and other non-2xx responses are failures.
- Delivery is asynchronous relative to SMTP storage. A failed webhook does not reject or delete the received email.
- Retries can duplicate a request. Deduplicate with `email.id` or `X-OwlMail-Email-ID` before performing non-idempotent actions.
- Default payloads may contain both plain-text and HTML email bodies. Use a
  custom template to minimize data when a receiver needs only a title or code.
- Webhook delivery is intended for notifications and automation, not as a
  guaranteed message queue. Use a durable downstream endpoint when delivery
  guarantees are required.
