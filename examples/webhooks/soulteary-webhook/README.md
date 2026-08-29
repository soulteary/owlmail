# OwlMail + `soulteary/webhook`

This runnable example sends each new OwlMail message to
[`soulteary/webhook`](https://github.com/soulteary/webhook), verifies the exact
request body with HMAC-SHA256, maps JSON fields into command environment
variables, and runs `print-email.sh`.

## Files

| File | Used by | Purpose |
|---|---|---|
| [`owlmail.json`](./owlmail.json) | OwlMail | Environment-backed target URL and HMAC secret, retry policy, and custom JSON body. |
| [`hooks.json.tmpl`](./hooks.json.tmpl) | `soulteary/webhook` | Hook, payload mapping, and `X-OwlMail-Signature` verification. |
| [`print-email.sh`](./print-email.sh) | `soulteary/webhook` | Side-effect-free demo command that prints mapped fields. |
| [`compose.yaml`](./compose.yaml) | Docker Compose | Builds OwlMail and starts both services on one private network. |

## Start the stack

From this directory:

```bash
cd examples/webhooks/soulteary-webhook
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
docker compose up --build
```

The Compose file has a development-only fallback secret so the demo can start,
but set your own value whenever the ports are reachable by another machine.
The same secret is passed to both containers. Compose also supplies the command
and working-directory paths used inside the `webhook` container.
`soulteary/webhook` starts in template mode so `getenv` can inject all three
values into the hook definition. The Hook waits for the demo command and
captures its output, so command failures become non-2xx responses to OwlMail.
Debug logging is enabled so the command summary is visible in this demo; turn
off `DEBUG` (and normally `VERBOSE`) before production because mapped email
values can appear in logs.

## Send a message

In another terminal:

```bash
printf 'From: monitor@example.test\r\nTo: ops@example.test\r\nSubject: Demo alert\r\n\r\nThe integration works.\r\n' \
  | curl --url smtp://127.0.0.1:1025 \
      --mail-from monitor@example.test \
      --mail-rcpt ops@example.test \
      --upload-file -
```

Open `http://localhost:1080` to inspect the stored email. The Compose output for
the `webhook` service prints a summary similar to:

```text
OwlMail webhook event
  event: email.received
  id: Ab12Cd34
  title: Demo alert
  from: monitor@example.test
  to: ops@example.test
  received: 2026-08-29T12:00:00Z
  message: The integration works.
```

If the secrets differ or the request body changes after signing,
`soulteary/webhook` rejects the trigger and the command does not run. OwlMail
logs the non-2xx delivery result and retries according to `owlmail.json`.

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
owlmail -webhook-config owlmail.json
```

[中文说明](./README.zh-CN.md) ·
[Webhook forwarding reference](../../../docs/en/Webhook-Forwarding.md)
