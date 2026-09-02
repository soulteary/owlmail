# Changelog

All notable changes to OwlMail are documented in this file. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and release tags use
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- An optional, default-off read-only MCP server using the official Go SDK's
  Streamable HTTP transport. It exposes compact list/search, detached email,
  bounded raw source, and attachment-metadata tools; shares Web Basic Auth,
  HTTPS, and base-path routing; and bounds idle-session and shutdown cleanup.
- An idempotent `migrate-attachments` offline command for resumable local-to-S3
  attachment migration, with dry-run planning and read-only remote validation,
  bounded retries, per-attachment progress, exact filename/size/SHA-256
  verification, atomic metadata commits, and opt-in local deletion after remote
  verification.
- Responsive HTML email preview width presets for 100%, 1440, 1024, 768,
  425, 375, and 320 px without reloading the message or replacing its secure
  iframe.
- Configurable reverse-proxy subpath deployment with `-base-pathname`,
  `OWLMAIL_BASE_PATHNAME`, and the compatible `MAILDEV_BASE_PATHNAME` alias.
  The prefix covers embedded pages and assets, REST and compatibility routes,
  attachment downloads, native WebSockets, and the Service Worker scope while
  preserving the existing root-path default.
- Real inbound SMTP AUTH with PLAIN and LOGIN mechanisms, constant-time
  credential checks, and protocol-level rejection of unauthenticated mail
  transactions.
- Optional `-smtp-auth-require-tls` /
  `OWLMAIL_SMTP_AUTH_REQUIRE_TLS` policy that rejects PLAIN/LOGIN on cleartext
  connections while preserving STARTTLS, SMTPS, and anonymous NO AUTH delivery.
- Optional S3-compatible object storage for decoded attachments, including
  bounded transactional uploads and rollback, restart-safe metadata, bounded
  streaming downloads, recoverable deletion and quarantine cleanup,
  content-verified legacy attachment mapping, retention cleanup, and
  compatibility with existing local attachments.
- Cached S3 readiness checks using preferred read-only `HeadBucket` with a
  bounded prefix-scoped fallback for least-privilege policies, separate
  liveness and readiness endpoints, bounded probe timeouts, automatic recovery,
  and an opt-in strict startup check that preserves the non-strict default.

### Changed

- HTML email previews now combine server-side sanitization with a zero-permission
  iframe sandbox, a no-referrer policy, and a restrictive per-preview CSP.
  Remote images, fonts, stylesheets, and media are blocked by default and can
  be loaded only through an explicit, non-persistent per-message action. CID
  images continue to resolve through OwlMail's local attachment endpoint.
- Decoded MIME attachments now stream through a fixed-size buffer into private
  transaction staging files while size and SHA-256 are calculated. Large
  attachments no longer require an additional attachment-sized byte slice;
  body read, staging write, S3 upload, and rollback failures reject the message
  before it becomes visible.
- Mailbox list and preview queries now filter, sort, and paginate under a
  thread-safe store snapshot before cloning results. Full list responses clone
  only the selected page, while preview responses build lightweight summaries
  without cloning complete message bodies, headers, or attachments.
- The Web inbox now uses compact email previews for mailbox lists and only
  fetches complete message content after a message is selected.
- The zero-credential default is now documented as NO AUTH mode. It accepts
  unauthenticated delivery and arbitrary PLAIN/LOGIN credentials for test
  clients that require SMTP credentials. Configuring both inbound credentials
  enables required AUTH; configuring only one now fails startup.
- The SMTP and SMTPS message-size limit is configurable with
  `-smtp-max-message-mb` / `OWLMAIL_SMTP_MAX_MESSAGE_MB` and now defaults to
  100 MiB instead of 1 MiB.

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
