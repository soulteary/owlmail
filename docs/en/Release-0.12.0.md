# OwlMail 0.12.0

Released: 2026-10-29

This release improves inline attachment compatibility, exposes structured reply destinations, and lets parallel tests bind independent SMTP and HTTP ports directly. It includes PR #206 and the health-kit 4.0.4 dependency update since 0.11.4.

## Changes

- CID references in both image sources and attachment links resolve through the correct API and deployment base path. Matching handles bracketed Content-IDs, HTML entities, and one URI decoding pass; unknown or ambiguous references remain unchanged.
- Nested MIME alternatives and related inline attachments are parsed with a maximum depth of 32 and 10,000 total parts. Attachment storage remains streamed, and invalid-message rollback is preserved.
- Native REST and MailDev details expose optional `replyTo`; read-only MCP details expose optional `reply_to`. The inbox displays escaped reply addresses. Missing, malformed, or repeated headers retain their raw values without producing a structured field. `In-Reply-To` remains a Message-ID.
- `-smtp 0` and `-web 0` select OS-assigned ports. `-listen-info-file` publishes actual addresses atomically; settings, Origin checks, MCP browser links, and health probes use the bound ports.
- Concurrent shutdown waits for owned resources and joins API startup before returning, including discovery-file cleanup.

## Compatibility and upgrade

Back up the complete mail directory before upgrading. Existing Content-IDs and attachment filenames are not migrated; restart recovery preserves them. Reply-To fields are optional additions, and summary/search contracts are unchanged. Legacy Go SMTP constructors retain `0 = default SMTP port`; Go callers opt into dynamic ports with `ServerOptions{AllowEphemeralPort: true}`. `-smtps-port 0` still disables implicit TLS, and outgoing SMTP ports must be 1–65535.

Use a unique discovery file per instance. After an ungraceful crash, verify that the old owner has exited before removing a stale discovery lock. Discovery is local address metadata, not authentication. CID CSS `url()` and `srcset` are outside this release. Existing preview sandbox permissions and remote-resource defaults remain unchanged.

## Validation

PR #206 passed race-enabled tests, vet, lint, browser regressions, fuzzing, CodeQL, and packaged-image verification. Local checks covered 20 Go packages, 205 browser/documentation unit tests, five Chromium regressions, dual-instance dynamic-port capture, native/MailDev attachment bytes, health checks, shutdown, and restart recovery. Windows amd64 was cross-compiled; local runtime tests used macOS. The tag release workflow repeats its preflight checks and reachable-vulnerability scan before publishing artifacts.

Previous release: [0.11.4](Release-0.11.4.md).

## Install

```bash
docker pull ghcr.io/soulteary/owlmail:0.12.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.12.0
```

For repeatable deployment, record the published manifest digest and use
`ghcr.io/soulteary/owlmail@sha256:<digest>`.

## Release artifacts

- `checksums.txt` and `checksums.txt.sigstore.json`
- `owlmail-linux-amd64` and `owlmail-linux-amd64.spdx.json`
- `owlmail-linux-arm64` and `owlmail-linux-arm64.spdx.json`
- `owlmail-darwin-amd64` and `owlmail-darwin-amd64.spdx.json`
- `owlmail-darwin-arm64` and `owlmail-darwin-arm64.spdx.json`
- `owlmail-windows-amd64.exe` and `owlmail-windows-amd64.exe.spdx.json`
- `owlmail-licenses.tar.gz`

Linux amd64 download, verification, and launch example:

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.12.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.12.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.12.0/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.12.0/owlmail-licenses.tar.gz
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
grep ' owlmail-licenses.tar.gz$' checksums.txt | sha256sum -c -
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
tar -xzf owlmail-licenses.tar.gz
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

Keep the extracted `owlmail-licenses/` directory with redistributed binaries.
The bundle covers OwlMail and its Go dependencies; image operating-system
packages retain their own terms.

```bash
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.12.0
```
