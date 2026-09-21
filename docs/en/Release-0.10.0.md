# OwlMail 0.10.0 release notes

OwlMail 0.10.0 is a security release. It closes cross-origin reads of captured
mail from the Web UI, the REST API, the WebSocket stream, and the read-only MCP
endpoint; it stops writing the mailbox into a world-listable directory; and it
makes the implicit-TLS listener configurable instead of fixed at a privileged
port it often could not bind. Alongside that it adds fuzz coverage for the four
parsers that read fully attacker-controlled input, which found two of the bugs
fixed here.

The 0.10.0 release is dated 2026-09-21. The commands below use the `v0.10.0` tag
and `0.10.0` container image; run them after the tag and artifacts are available.

**Read the upgrade notes before deploying.** Three changes are visible on
upgrade, and one of them can stop a process from starting.

## Highlights

### Browser origin validation

The Web UI and REST API now validate the browser `Origin` header on every
request, independently of Web Basic Auth. Validation used to be conditional on
Basic Auth, which is off by default, so the default deployment answered every
origin with `Access-Control-Allow-Origin: *`. Binding to loopback did not
mitigate that: the browser running an unrelated page the developer visits is on
the same host, so that page could read the whole mailbox — including
password-reset links and verification codes — repoint the outgoing SMTP relay,
forward captured mail to an address it chose, and empty the mailbox.

The read-only MCP HTTP endpoint gets the same treatment and now owns its own,
stricter CORS policy rather than inheriting the Web one. This closes
cross-origin and DNS-rebinding reads of captured mail through `/mcp`.

Requests without an `Origin` header remain allowed and unchanged, because
non-browser clients never send one: `curl`, HTTP libraries, MCP SDK HTTP
clients, CI scripts, and server-to-server callers are unaffected.

Two new options list additional browser origins that should be allowed:

| Flag | Environment variable | Scope |
| --- | --- | --- |
| `-web-allowed-origins` | `OWLMAIL_WEB_ALLOWED_ORIGINS` | Web UI, REST API, WebSocket |
| `-mcp-allowed-origins` | `OWLMAIL_MCP_ALLOWED_ORIGINS` | `/mcp` only |

An allowed origin is answered with a CORS policy naming it exactly, with
credentials and an unchallenged preflight, so a browser client allowed on
purpose can read the response. A single `*` is the documented opt-out and
restores the previous wildcard behavior; it cannot be combined with a named
origin, because that combination can only be a typo that silently widens a list
meant to stay narrow.

Origins are compared the way a browser serializes them: default and zero-padded
ports, equivalent IP spellings, and internationalized domain names all match
their canonical form. The WebSocket upgrade enforces the same policy, since
browsers do not apply CORS to WebSockets and that stream carries the same mail
bodies.

The health, readiness, and metrics endpoints are inside the boundary rather than
exempt from it. Every caller that actually probes them sends no `Origin` and is
unaffected, while a browser page on an unrelated origin that used to read
`/healthz` was using it as a fingerprinting oracle for a loopback service.

### Storage permissions

Captured mail is no longer written into a world-listable directory, and every
artifact the storage layer commits now pins its own mode.

The mail directory was created `0755`, so any other local account could list the
mailbox: message identifiers, `.eml` filenames, sizes, timestamps, and which
messages carried attachments. The mail directory defaults to a path under the
shared system temporary directory, so this was the out-of-the-box arrangement
rather than an unusual one.

| Artifact | Before | After |
| --- | --- | --- |
| Mail directory | `0755` | `0750` |
| Per-message attachment directory | `0755` | `0700` |
| Attachment file | `0644` | `0600` |

The message body, the metadata sidecar, and the staging file each message is
committed through are now pinned with an explicit `chmod` as well, so none of
them depends on `os.CreateTemp`'s default or on the umask OwlMail was started
under. No mode is widened: every change here is a tightening, or pins a mode
that was already correct by accident.

### Configurable SMTPS port

`-smtps-port` and `OWLMAIL_SMTPS_PORT` select the port of the implicit-TLS
listener that `-tls` starts, and `0` starts no SMTPS listener at all, so a
deployment can offer STARTTLS on the SMTP port without a second bind.

The listener was fixed at the privileged port 465, so the published container
image could never bind it as the non-root user it runs as, and two OwlMail
instances on one host could not both enable TLS. Worse, OwlMail logged
"SMTPS Server running" *before* it attempted the bind and then discarded the
bind error, so that configuration produced a process that reported a successful
start, passed readiness, and had nothing accepting SMTPS. A configuration whose
SMTPS listener cannot bind now fails to start, naming the address and pointing
at `-smtps-port`, and the log line is written only after the listener is bound.

### Fuzzing, and the bugs it found

Go fuzz targets now cover the four places where OwlMail parses fully
attacker-controlled input: the MIME entry point every captured message goes
through, the HTML sanitizer whose output the Web UI preview renders, the `Date`
header fallback that supplies a message's sort key and retention age, and the
glob matcher that decides which webhook targets a message reaches. Each target
asserts properties rather than absence of panics. A CI job replays every seed
and fuzzes each target for thirty seconds on every pull request.

It found two bugs, both fixed here. A `Date` header of
`Mon, 01 Jan 0001 00:00:00 +0000` parses cleanly against the first layout
OwlMail tries, so it bypassed the current-time fallback — and since it arrives
in a header, any sender could choose it and sort ahead of every real message for
as long as the mailbox lives. And the HTML sanitizer discarded a stylesheet
`<link>` when a body was sanitized twice, because bluemonday appends its own
`rel` tokens and the result no longer matched the policy that had just produced
it.

### Also in this release

- The read-only MCP HTTP endpoint serves the modern stateless `2026-07-28`
  protocol and legacy stateful revisions concurrently on the same authenticated
  route.
- Web Basic Auth credentials are compared in constant time.
- `.golangci.yml` pins the lint rule set CI had been running implicitly and adds
  fourteen linters; `.github/dependabot.yml` schedules weekly grouped updates.
- Pull requests that change the container image now build and run it, check the
  embedded release metadata, and capture a message over SMTP end to end.
- The comparison guide covers Mailpit alongside MailDev and MailCatcher, pinned
  to a reviewed commit the same way the other projects are.
- health-kit, logger-kit, and version-kit move to their latest major versions.
  Routes, response bodies, response headers, and log fields are unchanged.

## Upgrade notes

- **A browser client on another origin is now blocked by default.** If a status
  page, dashboard, or local tool reads the OwlMail API from a different origin
  in a browser, name that origin in `-web-allowed-origins` (or
  `-mcp-allowed-origins` for `/mcp`) before upgrading. `*` restores the previous
  behavior. Non-browser callers send no `Origin` and need no change.
- **A deployment with a silently dead SMTPS listener will now refuse to
  start.** This is the common case of `-tls` inside the container image, where
  the non-root user cannot take port 465. Move the listener with
  `-smtps-port`, or turn it off with `-smtps-port 0`, which keeps STARTTLS
  available on the SMTP port.
- **The tightened file modes apply only to artifacts written after the
  upgrade.** Nothing is chmod-ed retroactively, because silently rewriting the
  modes of an existing mail directory would break a deployment that
  deliberately shares the volume with another container. To tighten an existing
  directory:

  ```bash
  chmod 0750 "$MAIL_DIR"
  find "$MAIL_DIR" -mindepth 1 -type d -exec chmod 0700 {} +
  find "$MAIL_DIR" -mindepth 1 -type f -exec chmod 0600 {} +
  ```

- There are no REST API, SMTP protocol, or storage-format changes from 0.9.0.
- MCP remains disabled by default and read-only when enabled.
- The MailDev and MailCatcher compatibility facades remain opt-in.
- CI configurations should pin `0.10.0` or a recorded manifest digest rather
  than moving `main` or `latest` tags.

## Included pull requests

- [#126](https://github.com/soulteary/owlmail/pull/126) move security policies into `.github`
- [#127](https://github.com/soulteary/owlmail/pull/127) keep security language links valid
- [#128](https://github.com/soulteary/owlmail/pull/128) modern and legacy MCP protocol revisions
- [#129](https://github.com/soulteary/owlmail/pull/129) browser origin validation on the MCP endpoint
- [#130](https://github.com/soulteary/owlmail/pull/130) pinned lint rule set and dependency schedule
- [#131](https://github.com/soulteary/owlmail/pull/131) Docker base image updates
- [#132](https://github.com/soulteary/owlmail/pull/132) AWS SDK updates
- [#133](https://github.com/soulteary/owlmail/pull/133) GitHub Actions updates
- [#134](https://github.com/soulteary/owlmail/pull/134) stop the outbox-block tests racing the flush worker
- [#135](https://github.com/soulteary/owlmail/pull/135) build and run the container image on pull requests
- [#136](https://github.com/soulteary/owlmail/pull/136) constant-time Web Basic Auth comparison
- [#137](https://github.com/soulteary/owlmail/pull/137) AWS SDK updates
- [#138](https://github.com/soulteary/owlmail/pull/138) `golang.org/x` updates
- [#139](https://github.com/soulteary/owlmail/pull/139) pinned storage modes and no world-listable mail directory
- [#140](https://github.com/soulteary/owlmail/pull/140) configurable SMTPS port with a loud bind failure
- [#141](https://github.com/soulteary/owlmail/pull/141) browser origin validation on the Web UI and API
- [#142](https://github.com/soulteary/owlmail/pull/142) fuzz targets for the MIME, HTML, date, and webhook parsers
- [#143](https://github.com/soulteary/owlmail/pull/143) Mailpit as a pinned comparison column
- [#144](https://github.com/soulteary/owlmail/pull/144) keep serving build details on `/version`
- [#146](https://github.com/soulteary/owlmail/pull/146) AWS SDK updates
- [#149](https://github.com/soulteary/owlmail/pull/149) Go module updates
- [#150](https://github.com/soulteary/owlmail/pull/150) GitHub Actions updates
- [#151](https://github.com/soulteary/owlmail/pull/151) health-kit, logger-kit, and version-kit major upgrades

## Install

```bash
docker pull ghcr.io/soulteary/owlmail:0.10.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.10.0
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

Linux amd64 download, verification, and launch example:

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.10.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.10.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.10.0/checksums.txt.sigstore.json
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

```bash
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.10.0
```

## Known limitations

- Origin validation protects browser clients. It is not a substitute for
  authentication: anything that can make a non-browser request to the API still
  reads the mailbox, so keep OwlMail on a trusted network.
- The tightened file modes are not applied retroactively; see the upgrade notes.
- MCP remains read-only and does not delete, mark, or relay messages.
- The MailCatcher facade does not implement MailCatcher's WebSocket event bus.
- Relay recovery is at-least-once rather than exactly-once.
- The exact GHCR `0.10.0` tag is immutable after publication; use a patch
  release instead of deleting and reusing published artifacts.
