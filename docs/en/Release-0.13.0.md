# OwlMail 0.13.0

Release date: 2026-11-02 (Asia/Shanghai).

## Changes since 0.12.0

- Native API, automatic forwarding, historical Relay routes and MailDev compatibility calls share one durable Relay task owner. Envelope recipients, retry status and source protection survive restart, including SMTP-only operation.
- Automatic forwarding persists prepared intent, a held task and submitted acknowledgement so interrupted handoffs can resume safely.
- Per-message metadata retains the last downstream SMTP DATA acceptance time and actual recipients independently of bounded Relay history; native/MailDev/MCP details and the inbox expose this receipt.
- Recovery repairs successful-task receipts without resending SMTP and keeps readiness failed until durable repair succeeds. Legacy queued JSON freezes recipients before delivery; invalid recovery state remains actionable rather than being silently discarded.
- Refresh published kit patch dependencies and required SQLite/libc checksums; Go remains 1.27.3. Existing Reply-To, MIME/CID handling, dynamic listener ports, REST and read-only MCP behavior remain available.

## Upgrade, compatibility and rollback

1. Stop incoming SMTP traffic and the old process. Back up the complete mail directory, including Relay records, metadata, local EML/attachments and outbox state. Back up Redis/S3 state separately when configured; record the exact old image digest and flags/environment.
2. Start 0.13.0 against an isolated copy first. Disable real outbound destinations for rehearsal: recovery may resume pending Relay work. Keep SMTP-only flags if used; recovery no longer relies on the Web UI being enabled.
3. Check `/healthz` and readiness, retained messages/attachments, a new SMTP capture, native and MailDev details, MCP reads, delivery receipts and restart recovery before admitting traffic. Investigate queued/held/handoff/receipt repair state instead of deleting records to force readiness.
4. Delivery remains at least once. A crash after downstream acceptance but before durable acknowledgement can duplicate delivery; a successful acceptance receipt does not promise exactly-once delivery or final inbox placement. Manual retries can resend mail.
5. Rollback is not a reverse migration: stop 0.13.0 and restore the complete pre-upgrade data and external stores before running 0.12.0. Do not run both versions against the same writable state. Restoring a snapshot may reintroduce pending deliveries, so reconcile downstream receipts before enabling outbound traffic. New captures after the snapshot require separate preservation and reconciliation.

Previous release: [0.12.0](Release-0.12.0.md). The tag workflow reruns race tests, vulnerability checks, binary metadata checks, license bundling and signed multi-platform publication; use its actual result as publication evidence.

## Install

```bash
docker pull ghcr.io/soulteary/owlmail:0.13.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.13.0
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
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.13.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.13.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.13.0/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.13.0/owlmail-licenses.tar.gz
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
  ghcr.io/soulteary/owlmail:0.13.0
```
