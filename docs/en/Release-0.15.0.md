# OwlMail 0.15.0

Stable standalone release notes. Normal binaries retain the multiuser startup gate. Artifact availability and verification results are established by the tagged release workflow; these notes do not assert a published digest or completed artifact verification.

## Changes since 0.14.0

- Block remote images and styles when an email's HTML API URL is opened directly, in the standalone, MailDev, MailCatcher and multiuser HTML views. Links open without an opener or referrer, CID attachment links still download, and the preview's explicit remote-content opt-in is unchanged.
- Require Go 1.27.4 and update Fiber to v3.7.0, go-smtp to v0.25.1, modernc.org/sqlite to v1.62.0, the soulteary kits (cli-kit v1.9.4, health-kit/v4 v4.0.7, logger-kit/v3 v3.0.6, version-kit/v4 v4.0.5) and the AWS SDK modules (aws-sdk-go-v2 v1.47.7, config v1.33.15, credentials v1.20.15, service/s3 v1.114.8).
- Add `GET /api/v1/me/mailbox` and a translated inbox notice to the multiuser components, explaining an empty personal inbox without returning addresses, owner IDs or message counts.
- Add the offline `recipient-route` command for inspecting, blocking, reassigning and resuming future recipient routing with exact plan tokens and audit evidence. See [recipient-route operations](Recipient-Route-Operations.md).
- Refuse a second control-store writer through a persistent `<database>.writer-lock` sidecar, and encode control database paths as SQLite file URIs. See [control writer lock](Control-Writer-Lock.md).
- Reject proxy login redirects, markerless or non-JSON protected responses in the multiuser UI; clear private state, stop polling and offer an explicit localized login reload.
- Keep the latest search, filter or pagination change made during an active multiuser refresh, add 0–1500 ms of polling jitter, and translate the automatic-refresh control.
- Bound multiuser maintenance work: a 4,096-subject claim cache, live merge recovery and online pending-mailbox retention in batches of at most 32 messages, and a routed SMTP quota check before fan-out.
- Apply the 254-byte recipient address limit after IDNA normalization.
- Keep message subjects and supplied content out of diagnostics when the server requires mailbox ownership. Standalone logging is unchanged.

## Availability and known limitations

Standalone remains the default and retains the existing SMTP, Web, MailDev, MCP and Relay behavior. **Normal release binaries still reject multiuser mode.** The multiuser changes above apply to gated components and were validated with synthetic trusted identities; there is no published runtime bypass.

Stargate has not yet provided the trusted verified-email declaration required for automatic inbox claiming, and deployed Phorge → Gorge → OwlMail delivery with Stargate-authenticated personal-inbox access remains unaccepted. Gorge is unchanged.

The control-store writer lock is advisory. Earlier binaries and raw SQL do not honor it; stop every earlier writer before an upgrade or an offline change. Batch limits bound message mutations per step, not worst-case filesystem latency.

## Upgrade, compatibility and rollback

1. Stop SMTP ingress and the old process. Back up the complete standalone mail directory, including EML, attachments, metadata, Relay/outbox records, and any external Redis/S3 state. Record flags and the old image digest.
2. Rehearse this version against an isolated copy, with real outbound destinations disabled. Check health/readiness, retained messages and attachments, new SMTP capture and restart recovery. Existing Relay delivery remains at least once.
3. Clients that open an email's HTML API URL directly no longer load remote images or styles. Use the inbox preview and its remote-content opt-in when remote resources must be rendered.
4. Building from source requires Go 1.27.4 or later.
5. Do not switch a production directory to multiuser mode. An isolated directory requires its complete control database and ownership state; it cannot be opened as standalone or restored from only EML files. Explicit migrations and recipient-route changes must remain offline and use a complete backup.
6. Roll back by stopping this version and restoring the complete pre-upgrade snapshot before running 0.14.0. Do not run both versions against one writable directory. Reconcile pending outbound work and preserve newer captures separately.

The release workflow publishes the exact version and eligible `latest`, `0`, and `0.15` aliases. Record the actual manifest digest and artifact verification results after the workflow completes.

Previous release: [0.14.0](Release-0.14.0.md).

## Install

Use the following commands once the tagged release workflow has made the artifacts available.

```bash
docker pull ghcr.io/soulteary/owlmail:0.15.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.15.0
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
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.15.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.15.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.15.0/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.15.0/owlmail-licenses.tar.gz
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
  ghcr.io/soulteary/owlmail:0.15.0
```
