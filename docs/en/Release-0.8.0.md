# OwlMail 0.8.0 release notes

OwlMail 0.8.0 turns the project into a more complete email-testing gateway for
developers, CI pipelines, and coding agents. The release adds durable and
observable relay delivery, layered configuration, an optional SQLite mailbox
index, a read-only MCP stdio bridge, an opt-in MailCatcher REST facade, and a
safer, faster inbox workflow.

The commands below become valid after the `v0.8.0` tag is published.

## Highlights

### Persistent relay jobs

Manual and API-triggered relay requests now return an asynchronous job ID.
Accepted jobs are persisted under the mail directory, restored after restart,
and retried for bounded transient failures with safe status categories.
Source EML files remain protected while a relay job still needs them, and
startup orders SMTP, the Web API, storage cleanup, and relay recovery so queued
work is not exposed or deleted prematurely.

Outgoing runtime configuration now uses immutable per-job snapshots. Enabling,
disabling, updating credentials, enqueueing, and closing are coordinated without
data races or producer-side channel closes. Already accepted jobs drain with
their original configuration while new submissions receive deterministic
disabled, full, or closed errors.

### Layered configuration and indexing

OwlMail can load a flat YAML or JSON configuration file through `-config` or
`OWLMAIL_CONFIG_FILE`. Precedence is explicit: CLI, MailDev environment
aliases, OwlMail environment, configuration file, then built-in defaults.
Unknown, duplicate, nested, null, oversized, and type-invalid values fail
closed.

An optional SQLite mailbox index accelerates restart and mailbox queries while
keeping stored EML and sidecar files authoritative. Prometheus metrics and
structured JSON logging are available for production-style test environments.

### MCP stdio bridge and client compatibility

`owlmail mcp-stdio` exposes the existing read-only MCP tools, resources,
prompts, limits, and mailbox store over the official stdio transport. Protocol
frames stay on stdout and logs stay on stderr, making the command suitable for
local coding-agent configuration.

The default-off MailCatcher REST facade covers message list and detail, HTML,
plain text, source, EML, CID parts, and deletion routes while sharing OwlMail's
Basic Auth, HTTPS, storage, and base path. OwlMail IDs remain opaque strings,
and the facade does not emulate MailCatcher's WebSocket event bus.

### Inbox and operator workflow

The inbox adds HTML, plain-text, header, and raw-source tabs, browser history,
keyboard navigation, and guarded manual relay controls. Relay actions show
their asynchronous job ID, suppress duplicate submission, and require explicit
confirmation before sending real mail.

SMTP greeting length, command length, recipients, message size, read timeout,
write timeout, and DATA concurrency are independently configurable. API list
queries and bulk mutations now reject malformed or oversized inputs instead of
silently normalizing them.

## Security and performance

- Outbound SMTP uses explicit `plain`, mandatory `starttls`, or implicit
  `smtps` transport. STARTTLS and certificate failures never fall back to
  plaintext, hostname verification is enabled by default, and credentials are
  rejected on cleartext connections.
- Potentially active HTML, SVG, XML, and JavaScript attachments are forced to
  download with `nosniff` protection and a sanitized filename.
- Relay DATA and raw-source responses stream from validated EML paths instead
  of allocating message-sized buffers. Filtered exports use lightweight
  summaries and preserve the existing message-count and byte limits.
- Relay configuration, queue submission, recovery, deletion protection, and
  shutdown have race and failure-path coverage.

## Upgrade notes

- Existing CLI and environment configuration remains supported. Configuration
  files are optional and lower precedence than environment variables and CLI
  arguments.
- SQLite indexing, Prometheus metrics, the MailCatcher facade, and both MCP
  transports remain opt-in.
- Clients that relied on invalid list parameters being silently ignored must
  send valid dates, read filters, sort fields, sort orders, and bounded ID
  batches.
- Persistent relay recovery is at-least-once. A crash after the remote SMTP
  server accepts DATA but before OwlMail records completion can cause a
  duplicate delivery.
- Back up the complete mail directory before testing an upgrade with persistent
  data.

## Included pull requests

- [#91](https://github.com/soulteary/owlmail/pull/91) Web history and keyboard navigation
- [#90](https://github.com/soulteary/owlmail/pull/90) configurable SMTP protocol limits
- [#89](https://github.com/soulteary/owlmail/pull/89) source-pinned MailDev and MailCatcher comparison
- [#93](https://github.com/soulteary/owlmail/pull/93) asynchronous relay status
- [#92](https://github.com/soulteary/owlmail/pull/92) Prometheus metrics
- [#94](https://github.com/soulteary/owlmail/pull/94) optional SQLite mailbox index
- [#97](https://github.com/soulteary/owlmail/pull/97) lifecycle-safe outgoing configuration
- [#95](https://github.com/soulteary/owlmail/pull/95) streaming SMTP relay
- [#96](https://github.com/soulteary/owlmail/pull/96) fail-closed outbound SMTP TLS
- [#98](https://github.com/soulteary/owlmail/pull/98) attachment content isolation
- [#100](https://github.com/soulteary/owlmail/pull/100) streaming source and bounded exports
- [#101](https://github.com/soulteary/owlmail/pull/101) structured JSON logging
- [#99](https://github.com/soulteary/owlmail/pull/99) strict API query and bulk bounds
- [#102](https://github.com/soulteary/owlmail/pull/102) email content tabs
- [#107](https://github.com/soulteary/owlmail/pull/107) persistent relay jobs
- [#106](https://github.com/soulteary/owlmail/pull/106) layered YAML and JSON configuration
- [#105](https://github.com/soulteary/owlmail/pull/105) MailCatcher REST facade
- [#104](https://github.com/soulteary/owlmail/pull/104) read-only MCP stdio bridge
- [#103](https://github.com/soulteary/owlmail/pull/103) safe manual relay controls

## Install

```bash
docker pull ghcr.io/soulteary/owlmail:0.8.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.8.0
```

For repeatable deployment, record the published manifest digest and use
`ghcr.io/soulteary/owlmail@sha256:<digest>`.

## Release artifacts

- `checksums.txt`
- `checksums.txt.sigstore.json`
- `owlmail-linux-amd64` and `owlmail-linux-amd64.spdx.json`
- `owlmail-linux-arm64` and `owlmail-linux-arm64.spdx.json`
- `owlmail-darwin-amd64` and `owlmail-darwin-amd64.spdx.json`
- `owlmail-darwin-arm64` and `owlmail-darwin-arm64.spdx.json`
- `owlmail-windows-amd64.exe` and
  `owlmail-windows-amd64.exe.spdx.json`

```bash
sha256sum -c checksums.txt
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.8.0
```

## Known limitations

- MCP remains read-only and does not delete, mark, or relay messages.
- The MailCatcher facade does not implement MailCatcher's WebSocket event bus.
- Relay recovery is at-least-once rather than exactly-once.
- The exact GHCR `0.8.0` tag is immutable after publication; use a patch
  release instead of deleting and reusing published artifacts.
