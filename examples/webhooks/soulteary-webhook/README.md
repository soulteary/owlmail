# OwlMail + `soulteary/webhook`

This runnable example sends each new OwlMail message to
[`soulteary/webhook`](https://github.com/soulteary/webhook), verifies the exact
request body with HMAC-SHA256, maps JSON fields into command environment
variables, and runs `print-email.sh`.

The Compose demo intentionally pins both released components: OwlMail `0.5.0`
and WebHook `7.0.0`. This keeps the example reproducible instead of rebuilding
OwlMail from a moving `main` branch.

## Files

| File | Used by | Purpose |
|---|---|---|
| [`owlmail.json`](./owlmail.json) | OwlMail | Environment-backed target URL and HMAC secret, retry policy, and custom JSON body. |
| [`hooks.json.tmpl`](./hooks.json.tmpl) | `soulteary/webhook` | Hook, payload mapping, and `X-OwlMail-Signature` verification. |
| [`print-email.sh`](./print-email.sh) | `soulteary/webhook` | Side-effect-free demo command that prints mapped fields. |
| [`compose.yaml`](./compose.yaml) | Docker Compose | Starts released OwlMail and WebHook images on one private network. |

## Start the stack

From this directory:

```bash
cd examples/webhooks/soulteary-webhook
export OWLMAIL_WEBHOOK_SECRET="$(openssl rand -hex 32)"
docker compose up
```

The Compose file has a development-only fallback secret so the demo can start,
but set your own random value whenever the ports are reachable by another
machine. The same secret is passed to both containers.

OwlMail explicitly uses `OWLMAIL_WEBHOOK_MAX_CONCURRENCY=8`, its safe default.
Keep the limit sized to downstream capacity. Set it to `0` only when unlimited
webhook delivery is intentional and the receiving system can absorb it.

`soulteary/webhook` starts in template mode so `getenv` can inject the secret,
command, and working directory into the hook definition. The Hook waits for the
demo command and captures its output, so command failures become non-2xx
responses to OwlMail. Debug logging is enabled for this local demo; disable
`DEBUG` (and normally `VERBOSE`) in production because mapped email values can
appear in logs.

## Send a message

In another terminal:

```bash
printf 'From: monitor@example.test\r\nTo: ops@example.test\r\nSubject: Demo alert\r\n\r\nThe integration works.\r\n' \
  | curl --url smtp://127.0.0.1:1025 \
      --mail-from monitor@example.test \
      --mail-rcpt ops@example.test \
      --upload-file -
```

Open `http://localhost:1080` to inspect the stored email and
`http://localhost:1080/webhooks` to inspect, import, validate, or generate the
OwlMail webhook configuration. The Compose output for the `webhook` service
prints the verified mapped event.

If the secrets differ or the request body changes after signing,
`soulteary/webhook` rejects the trigger and the command does not run. OwlMail
logs the non-2xx delivery result and retries according to `owlmail.json`. Keep
real handlers idempotent and use the email ID as a deduplication key.

## Stop and clean up

```bash
docker compose down
```

No mail volume is configured, so the demo's captured mail is removed with its
container. Add a volume at `/app/mail` when persistence is wanted.

For a non-container setup, run `soulteary/webhook` from this directory:

```bash
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
export OWLMAIL_WEBHOOK_COMMAND="$PWD/print-email.sh"
export OWLMAIL_WEBHOOK_WORKDIR="$PWD"
webhook -template -hooks hooks.json.tmpl -verbose -debug
```

In another terminal, pass OwlMail the local Hook URL and the same secret:

```bash
export OWLMAIL_WEBHOOK_URL='http://127.0.0.1:9000/hooks/owlmail'
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
export OWLMAIL_WEBHOOK_MAX_CONCURRENCY=8
owlmail -webhook-config owlmail.json
```

For production, keep WebHook private or behind an authenticated reverse proxy,
retain HMAC verification, restrict allowed command paths, bound execution and
concurrency, and avoid logging full email bodies.

[中文说明](./README.zh-CN.md) ·
[Webhook forwarding reference](../../../docs/en/Webhook-Forwarding.md) ·
[WebHook-side integration guide](https://github.com/soulteary/webhook/blob/main/docs/en-US/OwlMail-Integration.md)
