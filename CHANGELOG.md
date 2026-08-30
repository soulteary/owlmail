# Changelog

All notable changes to OwlMail are documented in this file. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and release tags use
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Optional Redis Streams-backed Webhook delivery with restart recovery,
  dead-letter records, stable delivery IDs, graceful drain, and replay-aware
  HMAC headers containing a timestamp and nonce.

### Changed

- The in-memory mail store now uses an ID map with ordered IDs and returns deep
  snapshots to API, WebSocket, webhook, and other event consumers.
- Mailboxes can enforce age, count, and disk limits with background cleanup;
  read state uses atomic sidecar metadata and ZIP exports stream with hard
  source-count and byte limits.

### Fixed

- Incoming messages and attachments are staged, synced, and atomically renamed
  before they become visible in memory. Startup recovery now quarantines
  incomplete, corrupt, and orphaned storage artifacts.

## [0.5.0]

### Added

- An embedded English/Chinese Webhook Configurator at `/webhooks` for building,
  importing, validating, copying, and downloading version 1 forwarding rules.
  Configuration is processed locally in the browser and is not activated until
  the downloaded file is selected with `-webhook-config` and OwlMail is
  restarted.
- Generic outgoing webhook delivery for newly received email, including
  multiple targets, wildcard filters, JSON-safe and plain-text templates,
  environment-backed values, HMAC-SHA256 signatures, bounded retries, and
  per-target timeouts.
- Runnable webhook scenarios for a minimal receiver, filtered notifications,
  custom payloads, multiple targets, plain text, and an end-to-end
  `soulteary/webhook` Compose stack.
- An embedded English/Chinese help page available from the inbox and at
  `/help` without separate runtime assets.
- Opt-in browser notifications for messages received over the live WebSocket.
- A process-wide webhook concurrency setting with a recommended default of
  `8`; `0` explicitly selects unlimited delivery concurrency.
- Browser and documentation contract tests, including API-route, configuration,
  local-link, and notification checks.

### Changed

- Migrated the HTTP implementation to Fiber v3.
- A lone Web Basic Auth username now receives a generated 32-character
  temporary password printed once to stderr. A lone password now uses the
  default username `admin`; neither value still disables authentication.
- Authenticated browser API and WebSocket requests are restricted to OwlMail's
  own origin. Server-to-server clients without a browser `Origin` header are
  unaffected.
- Webhook capacity is acquired before starting a handler goroutine, preventing
  unbounded goroutine creation when a finite concurrency limit is configured.
- Source builds move to Go 1.27.0 and refresh the project's Go, container, and
  CI dependencies.
- Project documentation now distinguishes MailDev-style workflows from exact
  API compatibility and records the current SMTP-authentication limitation.

### Release engineering

- Browser and documentation checks now use the repository-pinned Bun 1.4.0
  toolchain, and the release workflow repeats those checks against the exact
  release tag before publishing assets.
- The release workflow also gates publication on dependency verification,
  formatting, `go vet`, race-enabled Go tests, and `govulncheck`.
- Release notes are versioned in the repository and prepended to GitHub's
  generated change list.
- Manual release workflow runs must reference an existing semantic-version tag
  and build that tag instead of the default branch.
- Release binaries and container images embed their version, commit, build
  timestamp, and source tag; the workflow smoke-tests the version endpoint.
- Go Report Card output is generated in the repository with
  `soulteary/goreportcard-action` rather than loaded from the external service.

## [0.4.0] - 2026-01-27

- Adopted `soulteary/cli-kit` for command-line and environment configuration.

Earlier release notes remain available on the
[GitHub Releases page](https://github.com/soulteary/owlmail/releases).

[Unreleased]: https://github.com/soulteary/owlmail/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/soulteary/owlmail/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/soulteary/owlmail/releases/tag/v0.4.0
