# OwlMail 0.6.0 release notes

OwlMail 0.6.0 makes persisted mail and webhook delivery substantially safer.
The release adds atomic mailbox transactions and recovery, configurable storage
governance, durable webhook queue handoffs, Redis lease protection, and
mobile-compatible browser notifications. It also hardens authenticated origin
checks and the release supply chain.

OwlMail 0.6.0 was published on 2026-09-01.

Commands that reference `v0.6.0` or the `0.6.0` container tag work only after
the release tag has been published.

## Highlights

### Atomic mailbox persistence and recovery

Raw messages and attachments are staged and synced before the final `.eml`
rename makes a message visible. Failures during parsing, attachment writes, or
the in-memory commit roll back the transaction. At startup, incomplete,
corrupt, and generated orphan artifacts move to `<mail-directory>/quarantine/`
while unrelated directories and existing evidence are preserved.

The store now indexes mail by ID and returns deep snapshots to API, WebSocket,
webhook, and event consumers. Read state is persisted atomically under
`.owlmail-meta/`, so restart no longer resets successfully recorded state.

### Storage governance

Operators can combine independent limits:

```bash
owlmail \
  -mail-retention-days 14 \
  -mail-max-messages 10000 \
  -mail-max-disk-mb 2048 \
  -mail-cleanup-interval 10m
```

All three limits default to `0` and are therefore disabled. When enabled,
cleanup runs at startup and periodically removes the oldest mail until every
configured limit is satisfied. The stats API reports current disk use, cleanup
runs, deleted messages, reclaimed bytes, and the last cleanup error. ZIP export
streams data and is bounded to 1,000 source messages and 256 MiB of raw EML.

### Durable webhook handoff and Redis delivery

Every accepted webhook event is first written to a local outbox under the mail
directory. The outbox survives temporary Redis or in-memory queue handoff
failure and is preserved by inbox clear operations. Use a persistent
`-mail-directory` if this recovery boundary matters.

With `-webhook-redis-url`, Redis Streams provide restart recovery, stable
delivery IDs, consumer-group pending recovery, dead-letter records, and active
lease renewal. Lease renewal atomically verifies ownership so a delayed worker
cannot steal an entry back from another consumer. Delivery remains at least
once; receivers should deduplicate `X-OwlMail-Delivery-ID`.

Shutdown stops new handoffs and drains the outbox, queued work, and active
requests for up to `-webhook-shutdown-timeout`. Without Redis, a job is no
longer durable after it has moved from the local outbox into the process-local
memory queue.

### Webhook and browser correctness

- Concurrency applies to individual target requests instead of letting one
  message monopolize capacity across unrelated targets.
- Text glob rules match arbitrary strings, including URL-like values that
  contain `/`, while retaining Go-style wildcard and character-range grammar.
- HMAC nonce expiry is exclusive at the validity boundary.
- Browser notifications use an active service worker where supported, focus
  only an inbox client, and preserve the selected email in a new-window deep
  link.
- `OWLMAIL_WEB_EXTERNAL_SCHEME=https` lets authenticated HTTP and WebSocket
  origin checks use the browser-visible scheme behind trusted TLS termination.

## Behavior to review before upgrading

| Area | 0.6.0 behavior | Operator action |
|---|---|---|
| Mail directory | OwlMail creates `.owlmail-meta`, `.owlmail-webhook-outbox`, and `quarantine` | Back up and restore the complete directory, including hidden entries |
| Retention | Any non-zero limit can delete oldest messages at startup | Test policies against a copied mailbox before production use |
| Recovery | Damaged OwlMail artifacts are quarantined instead of silently skipped | Inspect quarantine evidence manually; do not move it directly into the live directory |
| Webhook delivery | Local handoff is durable; full restart-safe queued delivery requires Redis | Use a persistent mail directory and Redis where end-to-end restart recovery is required |
| Delivery semantics | Redis delivery is at least once | Deduplicate the stable delivery ID at receivers |
| Reverse proxy | Authenticated browser origins include the scheme | Set `OWLMAIL_WEB_EXTERNAL_SCHEME=https` when trusted TLS termination is external |

## Installation

### Release binaries

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.6.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.6.0/checksums.txt
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

The release includes Linux amd64/arm64, macOS amd64/arm64, and Windows amd64
binaries. Each executable has an adjacent SPDX SBOM. The checksum manifest has
a Sigstore bundle, and GitHub provenance attestations cover the binaries and
SBOMs.

### Go install

Source installation requires Go 1.27.0 or newer:

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@v0.6.0
```

Downloaded release binaries do not require Go, Bun, or Node.js at runtime.

### Container image

```bash
docker pull ghcr.io/soulteary/owlmail:0.6.0
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.6.0
```

The release workflow refuses to overwrite an existing `0.6.0` image, while
`0.6`, `0`, `main`, and `latest` remain moving aliases. Registry tags are still
names rather than content identities. Pin the published manifest digest for a
cryptographically exact deployment:

```text
ghcr.io/soulteary/owlmail@sha256:<digest>
```

## Known limitations

- Incoming SMTP username/password settings still do not reject unauthenticated
  senders. Keep SMTP on a trusted interface or behind network controls.
- Without Redis, delivery is not restart-safe after a job leaves the local
  outbox for the process-local memory queue.
- Redis delivery is at least once and can produce duplicates around crashes.
- Health endpoints remain public when Web Basic Auth is enabled.
- Without `-mail-directory`, the process-specific temporary mail directory is
  not a durable archive.

## Documentation

- [Operations and troubleshooting](./Operations.md)
- [Webhook forwarding reference](./Webhook-Forwarding.md)
- [Webhook scenarios](../../examples/webhooks/README.md)
- [API reference](./API-Reference.md)
- [Release verification](./Releasing.md)
- [Full changelog](../../CHANGELOG.md)
