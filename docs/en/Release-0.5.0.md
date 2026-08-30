# OwlMail 0.5.0 release notes

OwlMail 0.5.0 expands the server from a local inbox into a more complete
integration endpoint while retaining its single-binary deployment model. The
release adds configurable outgoing webhooks, embedded help, opt-in browser
notifications, explicit webhook capacity controls, and safer partial Web Basic
Auth behavior.

Commands that reference `v0.5.0` or the `0.5.0` container tag work only after
the release tag has been published.

## Highlights

### Webhook forwarding

Newly stored messages can be delivered to generic HTTP endpoints. A version 1
configuration supports 1–32 named targets with:

- case-insensitive wildcard filters for sender, recipient, and subject;
- default, custom JSON-safe, or plain-text request bodies;
- values loaded from environment variables;
- configurable headers, HMAC-SHA256 signatures, timeouts, and retries;
- multiple independent targets; and
- a runnable `soulteary/webhook` Compose integration.

Use `-webhook-config` or `OWLMAIL_WEBHOOK_CONFIG` to select the JSON file. The
process-wide `-webhook-max-concurrency` /
`OWLMAIL_WEBHOOK_MAX_CONCURRENCY` setting defaults to `8`; set it to `0` only
when unlimited delivery is intentional.

### Embedded operator help

The inbox now links to a bilingual local guide at `/help`. The HTML, CSS, and
JavaScript are embedded in the executable, so binary and container deployments
do not need a separate `web` directory.

### Browser notifications

Notifications remain off by default and require an explicit click in each
browser. They apply only to new messages received through the live WebSocket,
require HTTPS or a trusted local origin, and never include the message body.

### Web authentication defaults

Partial Web Basic Auth configuration no longer disables authentication:

| Configured values | Effective behavior |
|---|---|
| neither | authentication disabled |
| username only | keep the username, generate a random 32-character password, and print it once to stderr |
| password only | use username `admin` and the configured password |
| both | use both configured values unchanged |

Authenticated requests carrying a browser `Origin` header must come from
OwlMail's own origin. Basic Auth should still be used only on localhost or over
HTTPS.

## Behavior to review before upgrading

| Area | 0.5.0 behavior | Operator action |
|---|---|---|
| Webhook saturation | A finite limit applies backpressure before handler goroutines start and can delay SMTP `DATA` completion | Start with `8`; size timeouts, retries, and concurrency together |
| Webhook shutdown | In-flight delivery is not drained before process exit | Stop new SMTP traffic and allow the longest retry window before termination |
| Web credentials | One configured value now produces usable credentials instead of silently disabling auth | Read generated credentials from stderr or configure both values explicitly |
| Browser notifications | Permission and preference are browser-local | Enable per browser under HTTPS or localhost |
| MailDev clients | OwlMail keeps MailDev-style workflow routes but does not implement the current MailDev API or Socket.IO protocol exactly | Validate paths, payloads, read side effects, and WebSocket clients |

Back up the complete mail directory before changing versions. Test an important
archive against a copy first.

## Installation after publication

### Release binaries

The release workflow publishes five executables and `checksums.txt`:

| Platform | Asset |
|---|---|
| Linux amd64 | `owlmail-linux-amd64` |
| Linux arm64 | `owlmail-linux-arm64` |
| macOS amd64 | `owlmail-darwin-amd64` |
| macOS arm64 | `owlmail-darwin-arm64` |
| Windows amd64 | `owlmail-windows-amd64.exe` |

Linux amd64 example:

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.5.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.5.0/checksums.txt
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

### Go install

Source installation requires Go 1.27.0 or newer:

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@v0.5.0
```

Downloaded release binaries do not require Go or Node.js at runtime.

Release binaries and images embed `version`, `commit`, `build_date`, and the
source tag. Inspect them through `GET /api/v1/version`; the release workflow
smoke-tests the embedded version and commit before uploading assets.

### Container image

Use the release tag for a release deployment:

```bash
docker pull ghcr.io/soulteary/owlmail:0.5.0
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  ghcr.io/soulteary/owlmail:0.5.0
```

The `main` and `latest` tags move whenever the default branch is built; they are
not stable-release selectors. `0.5.0` selects this release, while
`sha-<short-commit>` selects one repository commit.

## Known limitations

- Incoming SMTP username/password settings are present but unauthenticated
  senders are not rejected. Keep the SMTP listener on a trusted network.
- Webhook forwarding is an integration notification mechanism, not a durable
  queue. Receivers should be idempotent.
- Health endpoints remain public when Web Basic Auth is enabled so probes can
  run without credentials.
- The compiled SMTP defaults are 1 MiB per message, 50 recipients, and
  10-second read/write timeouts.

## Documentation

- [Webhook forwarding reference](./Webhook-Forwarding.md)
- [Webhook scenarios](../../examples/webhooks/README.md)
- [API reference](./API-Reference.md)
- [Operations and troubleshooting](./Operations.md)
- [MailDev comparison and migration guide](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [Full changelog](../../CHANGELOG.md)
