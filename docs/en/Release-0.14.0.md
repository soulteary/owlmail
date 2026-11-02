# OwlMail 0.14.0

Stable standalone release notes. Normal binaries retain the multiuser startup gate. Artifact availability and verification results are established by the tagged release workflow; these notes do not assert a published digest or completed artifact verification.

## Changes since 0.13.0

- Add independent personal-mailbox infrastructure: system SMTP credentials and recipient routing, trusted-delivery pending inboxes, stable-identity binding, scoped APIs, durable multi-copy ingress, merge/retention recovery, and persistent admission quotas.
- Add a polling inbox and mode-specific OpenAPI contracts. Multiuser components exclude MCP, WebSocket, shared spaces, outbound relay and global mailbox operations.
- Add offline doctor/migration tooling and recovery validation, including real process interruption, immutable backup restore, derived SQLite index rebuild, and isolated browser, volume, TLS, ACL and SELinux labs.
- Drain normal Relay readiness probes during shutdown so scratch-file IO does not outlive successful closure; blocked filesystem IO produces an explicit bounded shutdown error.
- Add explicit-history SMTP/HTTP coverage for recycled addresses, competing subjects and delivery across login.
- Verify the existing Gorge SMTP transport and the pinned original Phorge PHP mail adapter against OwlMail in isolated component tests, including MIME/attachment preservation and permanent-recipient rejection. Phorge notification authorization, databases and workers are not deployed by these tests.
- Bound transient native-Chromium profile cleanup retries while preserving functional assertions and permanent-error failures.

## Availability and known limitations

Standalone remains the default and retains the existing SMTP, Web, MailDev, MCP and Relay behavior. **Normal release binaries still reject multiuser mode.** This version supports standalone operation; multiuser infrastructure remains gated and is not an enabled multiuser service. There is no published runtime bypass. Release binaries and normal container images exclude the fixture executable and external PHP/Gorge runtimes. GitHub source archives contain the repository’s test sources; that does not enable a production fixture entry point.

Stargate has not yet provided the trusted verified-email declaration required for automatic inbox claiming. Gorge’s existing SMTP transport has isolated component coverage; deployed Phorge → Gorge → OwlMail delivery together with Stargate-authenticated personal-inbox access and claiming remains unaccepted. Gorge is unchanged and uses system-level SMTP credentials rather than user passwords. Real upstream identity acceptance remains pending. Recovery on two physical hosts and production NFS support are outside this release scope, rather than universal prerequisites for independent personal inboxes.

NFSv4/RichACL experiments used an isolated, patched lab stack and numeric AUTH_SYS on one VM. They do not establish production NFS support, Kerberos/domain identity support, arbitrary server compatibility, or SQLite-on-NFS deployment support. Process-kill tests do not prove hardware power-loss durability or exactly-once SMTP delivery.

## Upgrade, compatibility and rollback

1. Stop SMTP ingress and the old process. Back up the complete standalone mail directory, including EML, attachments, metadata, Relay/outbox records, and any external Redis/S3 state. Record flags and the old image digest.
2. Rehearse this version against an isolated copy, with real outbound destinations disabled. Check health/readiness, retained messages and attachments, new SMTP capture and restart recovery. Existing Relay delivery remains at least once.
3. Do not switch a production directory to multiuser mode. An isolated directory requires its complete control database and ownership state; it cannot be opened as standalone or restored from only EML files. Explicit migrations must remain offline and use a complete backup.
4. Roll back by stopping this version and restoring the complete pre-upgrade snapshot before running 0.13.0. Do not run both versions against one writable directory. Reconcile pending outbound work and preserve newer captures separately.

The release workflow publishes the exact version and eligible `latest`, `0`, and `0.14` aliases. Record the actual manifest digest and artifact verification results after the workflow completes.

Previous release: [0.13.0](Release-0.13.0.md).

## Install

Use the following commands once the tagged release workflow has made the artifacts available.

```bash
docker pull ghcr.io/soulteary/owlmail:0.14.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.14.0
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
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0/owlmail-licenses.tar.gz
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
  ghcr.io/soulteary/owlmail:0.14.0
```
