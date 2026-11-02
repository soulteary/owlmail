# OwlMail 0.14.0-alpha.1

Alpha infrastructure preview, prepared 2026-11-02 (Asia/Shanghai). Publication is conditional on the exact merged commit passing CI and the release workflow; these notes do not claim artifacts have been published.

## Changes since 0.13.0

- Add independent personal-mailbox infrastructure: system SMTP credentials and recipient routing, trusted-delivery pending inboxes, stable-identity binding, scoped APIs, durable multi-copy ingress, merge/retention recovery, and persistent admission quotas.
- Add a polling inbox and mode-specific OpenAPI contracts. Multiuser components exclude MCP, WebSocket, shared spaces, outbound relay and global mailbox operations.
- Add offline doctor/migration tooling and recovery validation, including real process interruption, immutable backup restore, derived SQLite index rebuild, and isolated browser, volume, TLS, ACL and SELinux labs.
- Drain normal Relay readiness probes during shutdown so scratch-file IO does not outlive successful closure; blocked filesystem IO produces an explicit bounded shutdown error.

## Availability and known limitations

Standalone remains the default and retains the existing SMTP, Web, MailDev, MCP and Relay behavior. **Normal release binaries still reject multiuser mode.** This Alpha previews infrastructure for review and standalone regression testing; it is not an enabled multiuser service. There is no published runtime bypass. Test-only fixtures and their synthetic identities are not release artifacts.

Stargate has not yet provided the trusted verified-email declaration required for automatic inbox claiming. Real Phorge/Gorge integration remains untested; Gorge is unchanged and the design uses system-level SMTP credentials rather than user passwords. Two-physical-host recovery and production identity acceptance remain pending.

NFSv4/RichACL experiments used an isolated, patched lab stack and numeric AUTH_SYS on one VM. They do not establish production NFS support, Kerberos/domain identity support, arbitrary server compatibility, or SQLite-on-NFS deployment support. Process-kill tests do not prove hardware power-loss durability or exactly-once SMTP delivery.

## Upgrade, compatibility and rollback

1. Stop SMTP ingress and the old process. Back up the complete standalone mail directory, including EML, attachments, metadata, Relay/outbox records, and any external Redis/S3 state. Record flags and the old image digest.
2. Rehearse this Alpha against an isolated copy, with real outbound destinations disabled. Check health/readiness, retained messages and attachments, new SMTP capture and restart recovery. Existing Relay delivery remains at least once.
3. Do not switch a production directory to multiuser mode. An isolated directory requires its complete control database and ownership state; it cannot be opened as standalone or restored from only EML files. Explicit migrations must remain offline and use a complete backup.
4. Roll back by stopping the Alpha and restoring the complete pre-upgrade snapshot before running 0.13.0. Do not run both versions against one writable directory. Reconcile pending outbound work and preserve newer captures separately.

Only the exact prerelease image tag is intended for this Alpha; `latest`, `0`, and `0.14` must not move. Record the actual published digest, checksums, signatures and workflow result after publication.

Previous release: [0.13.0](Release-0.13.0.md).

## Install

```bash
docker pull ghcr.io/soulteary/owlmail:0.14.0-alpha.1
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.14.0-alpha.1
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
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0-alpha.1/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0-alpha.1/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0-alpha.1/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0-alpha.1/owlmail-licenses.tar.gz
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
  ghcr.io/soulteary/owlmail:0.14.0-alpha.1
```
