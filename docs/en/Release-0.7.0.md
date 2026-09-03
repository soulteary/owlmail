# OwlMail 0.7.0 release notes

OwlMail 0.7.0 expands the test gateway with read-only MCP workflows, explicit
MailDev REST compatibility, a complete native OpenAPI contract, sendmail-style
submission, S3 attachment storage, SMTP resource controls, and secure HTML
preview isolation.

OwlMail 0.7.0 was published on 2026-09-02.

## Highlights

### Agent and compatibility interfaces

- Optional Streamable HTTP MCP with bounded read-only list, search, detail,
  source, attachment metadata, latest-mail, and delivery-wait workflows.
- Optional MailDev REST facade below `/api`; Socket.IO and the Node embedding
  API remain outside the compatibility contract.
- Native OpenAPI 3.1 JSON and YAML with route-drift checks.
- `owlmail sendmail` for PHP, cron, and legacy programs.

### Storage and ingress

- Optional S3-compatible decoded attachment storage with transactional upload,
  verification, rollback, readiness, and offline migration.
- Configurable message size, recipient count, SMTP read/write timeouts, and a
  process-wide DATA concurrency limit.
- Real PLAIN and LOGIN SMTP AUTH with optional TLS requirement.
- Reverse-proxy base paths across Web assets, API, WebSocket, attachments, and
  MCP.

### Browser security and scale

HTML previews combine server-side sanitization, a zero-permission iframe,
no-referrer policy, restrictive CSP, and default blocking of remote content.
Mailbox list paths use compact previews and bounded snapshot queries.

## Upgrade notes

- The default maximum message size changes from 1 MiB to 100 MiB.
- Setting both SMTP credentials requires AUTH; setting only one fails startup.
- MCP and both compatibility facades remain disabled unless explicitly enabled.
- S3 migration must run while the writable OwlMail process is stopped.
- Remote content loading is explicit and per message.

## Installation

```bash
docker pull ghcr.io/soulteary/owlmail:0.7.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  ghcr.io/soulteary/owlmail:0.7.0
```

Registry tags are names, not content identities. Use the published manifest
form `ghcr.io/soulteary/owlmail@sha256:<digest>` for an exact deployment.

## Known boundaries

- MCP in 0.7.0 uses Streamable HTTP; the stdio bridge is introduced in 0.8.0.
- MailDev compatibility does not include Socket.IO.
- Mailbox and storage state belong to one writable OwlMail instance.
- Redis-backed Webhook delivery is at least once around failure and recovery.

See the [0.7.0 changelog](../../CHANGELOG.md#070---2026-09-02),
[API reference](./API-Reference.md), and [Operations](./Operations.md).
