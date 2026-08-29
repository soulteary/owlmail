# OwlMail webhook examples

These examples are intentionally separate: start with the smallest file that
matches your use case instead of deleting fields from one large configuration.

| Scenario | Configuration | What it demonstrates |
|---|---|---|
| Send every new email | [`minimal.json`](./minimal.json) | The two required target fields and OwlMail's default JSON event. |
| Route selected alerts | [`filtered-alerts.json`](./filtered-alerts.json) | Recipient and subject matching, timeout, and retries. |
| Call an authenticated JSON API | [`custom-json.json`](./custom-json.json) | Environment-backed URL/token/secret, custom headers, HMAC, and a JSON-safe template. |
| Fan out to several destinations | [`multiple-targets.json`](./multiple-targets.json) | An archive target plus a filtered incident target. One email may match both. |
| Send a plain-text message | [`plain-text.json`](./plain-text.json) | A non-JSON content type and a text template. |
| Integrate with `soulteary/webhook` | [`soulteary-webhook/`](./soulteary-webhook/) | A complete Docker Compose stack, HMAC validation, payload mapping, and command execution. |
| Full single-file reference | [`../webhooks.json`](../webhooks.json) | Most target options together in one configuration. |

## Five-minute local smoke test

The bundled receiver listens only on `127.0.0.1:18080`, prints each request,
returns HTTP 204, and can optionally verify OwlMail's HMAC signature.

Terminal 1 — start the receiver:

```bash
go run ./examples/webhooks/receiver
```

Terminal 2 — start OwlMail with the minimal configuration:

```bash
go run ./cmd/owlmail -webhook-config ./examples/webhooks/minimal.json
```

Terminal 3 — send a test email (curl must include SMTP protocol support):

```bash
printf 'From: sender@example.test\r\nTo: inbox@example.test\r\nSubject: Minimal webhook\r\n\r\nHello from OwlMail.\r\n' \
  | curl --url smtp://127.0.0.1:1025 \
      --mail-from sender@example.test \
      --mail-rcpt inbox@example.test \
      --upload-file -
```

The receiver output includes the request path, event type, email ID, content
type, signature state, and body. Stop both processes with Ctrl+C.

## Run each scenario

### Minimal: forward everything

```bash
./owlmail -webhook-config ./examples/webhooks/minimal.json
```

There is no `match`, `bodyTemplate`, secret, or custom header, so every new
email is sent once using OwlMail's default `email.received` JSON payload.

### Filter alerts

```bash
./owlmail -webhook-config ./examples/webhooks/filtered-alerts.json
```

An email must match at least one `to` pattern **and** at least one `subject`
pattern. Change the example addresses and words to match your own alerts.

### Authenticated custom JSON

Start the example receiver with signature verification:

```bash
export OWLMAIL_WEBHOOK_SECRET='replace-this-development-secret'
go run ./examples/webhooks/receiver
```

In the OwlMail terminal, set every placeholder used by the configuration:

```bash
export OWLMAIL_WEBHOOK_URL='http://127.0.0.1:18080/custom'
export OWLMAIL_WEBHOOK_TOKEN='development-token'
export OWLMAIL_WEBHOOK_SECRET='replace-this-development-secret'
./owlmail -webhook-config ./examples/webhooks/custom-json.json
```

The receiver verifies `X-OwlMail-Signature`. It deliberately logs only whether
the `Authorization` header is present, never its value.

### Multiple targets

```bash
./owlmail -webhook-config ./examples/webhooks/multiple-targets.json
```

Every message goes to `/archive`. Messages addressed to `ops@*` whose subject
contains `critical` or `outage` also go to `/incidents`. Destinations are
evaluated in file order for each email.

### Plain text

```bash
./owlmail -webhook-config ./examples/webhooks/plain-text.json
```

This sends `text/plain; charset=utf-8` instead of JSON and truncates the message
body to 2,000 Unicode characters.

## Running OwlMail in a container

`127.0.0.1` inside a container refers to that container. Replace the example
URL with a Compose service name such as `http://receiver:18080/owlmail`, or with
an address reachable from the OwlMail container. The
[`soulteary-webhook`](./soulteary-webhook/) example shows service-name routing.

## Validate before deployment

OwlMail validates and compiles the complete file at startup. A missing or empty
environment variable, unknown JSON field, bad duration, invalid wildcard, or
invalid template prevents the service from starting. After changing a file,
restart OwlMail; webhook configuration is not hot-reloaded.

OwlMail runs at most eight email webhook jobs concurrently by default. Keep the
recommended limit for most development and CI workloads; use
`-webhook-max-concurrency 0` only for deliberately unlimited concurrency.

See the [full English reference](../../docs/en/Webhook-Forwarding.md) or
[中文参考](../../docs/zh-CN/Webhook-Forwarding.md) for every field, payload,
header, matching rule, retry behavior, and security boundary.
