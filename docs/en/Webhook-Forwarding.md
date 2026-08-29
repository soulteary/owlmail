# Webhook forwarding

OwlMail can send every matching new email to one or more HTTP endpoints. The feature is configured at startup, runs on the server even when no browser is open, and does not alter MailDev-compatible APIs.

## Enable forwarding

Create a JSON file and pass it to OwlMail:

```bash
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-random-secret'
./owlmail -webhook-config ./webhooks.json
```

The environment variable form is equivalent:

```bash
export OWLMAIL_WEBHOOK_CONFIG=./webhooks.json
```

See [`examples/webhooks.json`](../../examples/webhooks.json) for a complete configuration.

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
| `name` | Yes | Safe identifier used in logs; the full URL and configured secrets are never logged. |
| `url` | Yes | Fixed `http` or `https` URL. Redirects are not followed. |
| `method` | No | `POST` by default; `POST`, `PUT`, and `PATCH` are accepted. |
| `headers` | No | Static request headers. `Host` and `Content-Length` cannot be overridden. |
| `contentType` | No | `application/json` by default. An explicit `Content-Type` header takes precedence. |
| `secret` | No | Generates `X-OwlMail-Signature: sha256=<hex>` over the exact request body. |
| `timeout` | No | Per-attempt timeout; default `5s`, maximum `1m`. |
| `retries` | No | Extra attempts from zero to five. Only network errors, 408, 425, 429, and 5xx responses are retried. |
| `match` | No | Case-insensitive wildcard rules. See the rule semantics below. |
| `bodyTemplate` | No | Go text template for a custom body. When omitted, OwlMail sends the default event payload. |

`${VARIABLE}` placeholders are supported in `url`, header values, and `secret`. OwlMail fails at startup if a referenced variable is missing, rather than silently sending an unauthenticated request.

## Matching rules

The `from`, `to`, `subject`, and `text` fields accept arrays of shell-style patterns using `*` and `?`.

- Patterns inside one field are ORed.
- Different non-empty fields are ANDed.
- Empty fields match every email.
- Matching is case-insensitive.
- `to` considers envelope recipients as well as To, Cc, and Bcc headers.

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

Always use `json` when placing email-controlled text into JSON:

```json
"bodyTemplate": "{\"title\":{{ json .Subject }},\"message\":{{ json (truncate .Text 4000) }}}"
```

Without a template, OwlMail sends:

```json
{
  "event": "email.received",
  "message": "Subject followed by plain-text content",
  "email": {
    "id": "email-id",
    "subject": "Example",
    "from": ["sender@example.com"],
    "to": ["recipient@example.com"],
    "text": "Message body"
  }
}
```

The exact email ID is also sent as `X-OwlMail-Email-ID`, which receivers can use as an idempotency key when retries produce a duplicate request.

## Send to `soulteary/webhook`

Use the hook URL exposed by [`soulteary/webhook`](https://github.com/soulteary/webhook):

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

One matching `soulteary/webhook` hook configuration is:

```json
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
        "secret": "{{ getenv \"OWLMAIL_WEBHOOK_SECRET\" | js }}",
        "parameter": { "source": "header", "name": "X-OwlMail-Signature" }
      }
    }
  }
]
```

Start `soulteary/webhook` with its `-template` option when using `getenv` in the hook file. Give both containers the same `OWLMAIL_WEBHOOK_SECRET` value.

## Operational and security notes

- Webhook targets receive email content. Only configure destinations you trust.
- Prefer HTTPS outside a private container network and HMAC-sign every request.
- Keep the configuration file private (for example, mode `0600`) and put tokens in environment variables.
- A successful response is any 2xx status. Redirects and other non-2xx responses are failures.
- Delivery is asynchronous relative to SMTP storage. A failed webhook does not reject or delete the received email.
- Retries can duplicate a request. Deduplicate with `email.id` or `X-OwlMail-Email-ID` before performing non-idempotent actions.
