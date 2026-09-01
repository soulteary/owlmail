# Changelog

All notable changes to OwlMail are documented in this file. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and release tags use
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] - 2026-08-31

### Added

- Mailbox governance with independent age, message-count, and disk-usage
  limits, periodic cleanup metrics, atomic read-state sidecars, and bounded
  streaming ZIP exports.
- Optional Redis Streams-backed webhook delivery with restart recovery,
  dead-letter records, stable delivery IDs, active lease renewal, graceful
  drain, and replay-aware HMAC headers containing a timestamp and nonce.
- A local webhook outbox that durably records queue handoffs before Redis or
  the in-memory worker accepts them.
- Service-worker browser notifications with inbox-safe click routing and
  email deep links for mobile browsers.

### Changed

- The in-memory mail store now uses an ID map with ordered IDs and returns deep
  snapshots to API, WebSocket, webhook, and other event consumers.
- Webhook target concurrency is enforced per HTTP request, text globs match
  arbitrary strings including slashes, and shutdown drains queued and active
  deliveries up to the configured deadline.
- Authenticated HTTP and WebSocket origin checks can use an explicitly
  configured browser-facing scheme when TLS terminates at a trusted proxy.
- Locale initialization now renders translated empty-state content before the
  first API response and recognizes `zh-SG` as Simplified Chinese.

### Release engineering

- CI now runs the race-enabled coverage suite once and reuses its output for
  the Job Summary and a seven-day HTML artifact. The duplicate Codecov workflow
  and silently ignored external upload failures were removed.
- Pull requests now build once on Ubuntu, main cross-compiles snapshots on
  Ubuntu with seven-day retention, and only the release workflow handles tag
  binaries and versioned container images. Build workflows cancel superseded
  branch runs and skip documentation-only changes.
- Formal releases now publish per-binary SPDX SBOMs, GitHub provenance
  attestations, a keyless Sigstore signature for the checksum manifest, signed
  container manifests, BuildKit SBOM/provenance attestations, and explicit OCI
  source, revision, version, and license labels.
- Stable moving image aliases are disabled for older release retries and are
  revalidated against freshly fetched tags immediately before publication.

### Fixed

- Incoming messages and attachments are staged, synced, and atomically renamed
  before they become visible in memory. Startup recovery now quarantines
  incomplete, corrupt, and orphaned storage artifacts without deleting
  unrelated directories or previously quarantined evidence.
- Mail deletion, cleanup, startup restoration, and bulk read operations now
  surface persistence failures instead of reporting success after disk state
  diverges from memory.
- Redis webhook lease renewal verifies ownership atomically, webhook nonce
  expiry is exclusive at the validity boundary, and individual target
  schedules no longer block unrelated targets.
- Browser notifications wait for an active service worker and preserve the
  selected email when focusing or opening an inbox window.

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

[Unreleased]: https://github.com/soulteary/owlmail/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/soulteary/owlmail/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/soulteary/owlmail/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/soulteary/owlmail/releases/tag/v0.4.0
