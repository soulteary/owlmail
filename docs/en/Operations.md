# OwlMail Operations and Troubleshooting

This guide covers repeatable local and container deployments, security defaults,
readiness, persistence, webhook capacity, shutdown behavior, and common failure
modes. See the [API Reference](./API-Reference.md) and
[Webhook Forwarding](./Webhook-Forwarding.md) for protocol details.

## Mailbox retention and state

OwlMail can enforce age, message-count, and disk-usage limits together:

```bash
owlmail \
  -mail-retention-days 14 \
  -mail-max-messages 10000 \
  -mail-max-disk-mb 2048 \
  -mail-cleanup-interval 10m
```

Zero disables an individual limit. Cleanup runs once at startup and then in
the background. It removes the oldest messages until every configured limit is
met. The existing email stats response includes current disk bytes, configured
limits, cleanup runs, deleted messages, reclaimed bytes, and the last error.

Read state is stored atomically in `.owlmail-meta/<id>.json`; legacy messages
without metadata recover as unread. ZIP export is streamed to the client and is
bounded to 1,000 source messages and 256 MiB of raw EML data, avoiding a second
mailbox-sized in-memory buffer.

## Deployment profiles

### 1. Minimal local inbox

```bash
./owlmail
```

This listens on localhost by default: SMTP on 1025 and HTTP on 1080. Without
`-mail-directory`, raw messages and attachments are written under the
process-specific temporary directory `owlmail-<pid>` while parsed state is held
in memory. That location is not a durable archive. This profile is intended for
one developer on a trusted machine.

Readiness check:

```bash
curl --fail http://localhost:1080/healthz
```

### 2. Local inbox with persistence

```bash
mkdir -p ./owlmail-data
./owlmail -mail-directory ./owlmail-data
```

Back up the directory before changing OwlMail versions or testing an archive
created by another product. Attachment files are stored alongside OwlMail's
mail data; copying only selected files may produce incomplete messages.

OwlMail stages and syncs raw messages and attachments in the same directory,
commits attachments first, and uses the final atomic `.eml` rename as the
complete-message marker. Parse, attachment-write, or in-memory commit failures
roll back the current transaction. Startup recovery moves temporary artifacts,
orphan attachment directories, and unparseable `.eml` files into
`<mail-directory>/quarantine/` instead of loading or silently deleting damaged
data. Operators may remove reviewed entries; do not move them directly back
into the live mail directory while troubleshooting.

### Optional S3-compatible attachment storage

Decoded attachments can be stored in AWS S3 or an S3-compatible service while
raw messages, metadata, transaction markers, temporary staging files, and the
webhook outbox remain in `-mail-directory`. Local attachment storage remains the
default.

```bash
OWLMAIL_S3_ENABLED=true \
OWLMAIL_S3_ENDPOINT=http://minio:9000 \
OWLMAIL_S3_REGION=us-east-1 \
OWLMAIL_S3_BUCKET=owlmail \
OWLMAIL_S3_PREFIX=owlmail/attachments \
OWLMAIL_S3_ACCESS_KEY=replace-me \
OWLMAIL_S3_SECRET_KEY=replace-me \
OWLMAIL_S3_USE_PATH_STYLE=true \
./owlmail -mail-directory ./owlmail-data
```

The bucket must exist before OwlMail starts receiving attachments. Leave the
endpoint empty for AWS S3. Static credentials are optional: when omitted, the
AWS SDK default credential chain is used, including environment credentials,
shared configuration, and workload roles. Use path-style addressing only when
the selected S3-compatible service requires it.

OwlMail stages attachments locally, writes a durable rollback marker, uploads
every object under `<prefix>/<email-id>/`, and only then commits the `.eml`
marker. A failed upload rejects the SMTP transaction and triggers prefix
cleanup; startup recovery retries cleanup after an interrupted transaction.
Single-email deletion, clear-all, and retention cleanup delete remote objects as
well as any matching legacy local attachment directory. Before deletion OwlMail
syncs a per-message deletion fence. Remote cleanup runs first; if it fails, the
raw message, metadata, local attachments, and fence remain available for a
same-process retry or automatic startup recovery. A deletion-fenced message is
never republished while cleanup is pending. Corrupt-message quarantine follows
the same remote-first rule and leaves the live `.eml` retry marker in place when
S3 is unavailable. Each remote deletion attempt has a 30-second deadline, so a
stalled endpoint releases the storage transaction lock and leaves cleanup for a
later retry.

Enabling S3 does not migrate existing attachments. Existing local attachments
remain readable and are removed normally, while newly received attachments use
S3. During startup, legacy attachment filenames are recovered only when file
size, extension, and SHA-256 content digest identify one unique local file;
ambiguous metadata is not persisted. Disabling S3 or changing its bucket/prefix
does not download or relocate objects, so migrate those objects before changing
the configuration. The `-mail-max-disk-mb` limit and `storage.diskBytes`
statistic cover local files only and do not include remote object bytes.

### 3. Persistent Docker deployment

```bash
docker volume create owlmail-data
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.6.0
```

This guide pins the `0.6.0` release image. The `main` and `latest` tags move with
default-branch builds and should not be used for a repeatable deployment.

The image configures OwlMail to listen on `0.0.0.0` inside the container. Bind
published ports to `127.0.0.1` as shown unless other machines must connect. The
Dockerfile runs as a non-root user and stores mail in `/app/mail`.

### 4. Web UI protected with fixed credentials

```bash
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -e OWLMAIL_WEB_USER=admin \
  -e OWLMAIL_WEB_PASSWORD='replace-with-a-secret' \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.6.0
```

Configure both values for stable automation. A username alone causes OwlMail to
generate a new password at each process start and print it once to stderr. A
password alone uses `admin` as the username. Startup fails if a generated
password cannot be printed. Retrieve startup output with:

```bash
docker logs owlmail
```

Basic Auth protects the UI, API, assets, and WebSocket endpoints, but
`/healthz` and `/api/v1/health` intentionally remain public for probes. Use
HTTPS or a trusted reverse proxy when credentials cross a network.

When TLS terminates at that proxy, set `OWLMAIL_WEB_EXTERNAL_SCHEME=https` so
authenticated HTTP and WebSocket same-origin checks use the browser-visible
scheme. Configure it explicitly instead of trusting client-supplied forwarded
headers.

## HTTPS and TLS

Web HTTPS and SMTP TLS are separate settings:

- `-https`, `-https-cert`, and `-https-key` protect the Web UI/API.
- `-tls`, `-tls-cert`, and `-tls-key` enable SMTP STARTTLS and direct SMTPS on
  container/process port 465.

Example web HTTPS:

```bash
./owlmail \
  -https \
  -https-cert ./certs/web-cert.pem \
  -https-key ./certs/web-key.pem
```

The image's built-in healthcheck uses plain HTTP on port 1080. If HTTPS is
enabled, replace the container healthcheck with one that uses HTTPS and the
appropriate CA policy; otherwise Docker can report an unhealthy container even
when OwlMail is serving correctly.

Publish port 465 explicitly when direct SMTPS is needed:

```bash
docker run -d \
  -p 1025:1025 -p 1080:1080 -p 465:465 \
  -v "$PWD/certs:/certs:ro" \
  -e OWLMAIL_TLS_ENABLED=true \
  -e OWLMAIL_TLS_CERT=/certs/smtp-cert.pem \
  -e OWLMAIL_TLS_KEY=/certs/smtp-key.pem \
  ghcr.io/soulteary/owlmail:0.6.0
```

Verify that the container runtime permits the non-root process to bind port 465.
If it does not, grant only the required bind-service capability according to the
runtime's security policy.

## SMTP ingress limits and authentication status

The SMTP and SMTPS servers accept at most 100 MiB per message by default. Set
`-smtp-max-message-mb` or `OWLMAIL_SMTP_MAX_MESSAGE_MB` to a positive MiB value
to change the limit. The recipient limit remains 50 and read/write timeouts
remain 10 seconds.

> [!WARNING]
> `-smtp-user` / `-smtp-password` and their environment aliases populate SMTP
> authentication configuration, but the current SMTP session does **not**
> reject unauthenticated senders. Do not use these options as an access-control
> boundary. Keep the SMTP listener on a trusted interface or protect it with
> network policy, a firewall, or a private tunnel.

For SMTP TLS, OwlMail uses the configured certificate only when both
`-tls-cert` and `-tls-key` are present; otherwise it generates a self-signed
certificate and logs a warning. Web HTTPS behaves differently: both
`-https-cert` and `-https-key` are required, and missing files prevent the Web
server from starting.

## Webhook capacity profiles

`-webhook-max-concurrency` is process-wide across all targets and messages and
is acquired for each individual target HTTP request. A finite limit bounds
active downstream requests while the local outbox absorbs accepted handoffs.

| Profile | Value | Use when |
|---|---:|---|
| recommended starting point | `8` | ordinary development and moderate downstream latency |
| conservative | `2`–`4` | fragile, rate-limited, or resource-constrained receivers |
| higher bounded throughput | `16`–`64` | load tests confirm downstream and file-descriptor capacity |
| unlimited | `0` | burst size is controlled and maximum machine utilization is intentional |

```bash
# Recommended bounded delivery
./owlmail \
  -webhook-config ./webhooks.json \
  -webhook-max-concurrency 8

# Explicitly unlimited
./owlmail \
  -webhook-config ./webhooks.json \
  -webhook-max-concurrency 0
```

The limit is not a queue size. SMTP `DATA` completion waits for the event to be
synced to `.owlmail-webhook-outbox`, not for a target slot, Redis append, or HTTP
response. Include target timeout and retry duration when estimating drain time,
and monitor receiver latency and error rate before raising the limit. Also
monitor free space on the mail volume because a prolonged downstream or Redis
outage can accumulate local outbox files.

## Shutdown and delivery guarantees

On `SIGINT` or `SIGTERM`, OwlMail stops SMTP intake and new webhook handoffs,
then drains the local outbox, queued work, and active webhook requests for up to
`-webhook-shutdown-timeout`. When the deadline expires, remaining operations
are canceled and shutdown reports an error. For deployments that require
stronger delivery guarantees:

1. Stop upstream SMTP traffic before terminating OwlMail.
2. Set the shutdown deadline to cover the expected queue and retry window.
3. Use Redis for restart-safe queued delivery and persist the complete mail
   directory so pre-queue outbox entries survive a restart.
4. Make receivers idempotent and deduplicate `X-OwlMail-Delivery-ID`.

Without Redis, only the handoff before the in-memory queue is durable; a job is
not restart-safe after it leaves the local outbox. Redis delivery is durable but
at least once, so duplicates remain possible around crashes.

Outgoing relay is also asynchronous. An API success response acknowledges the
in-process request, not delivery by the downstream SMTP server; inspect logs and
the destination system when delivery confirmation matters.

Configuration is not yet validated by one uniform startup pass. S3 option shape
is checked when that backend is enabled, but reachability and credentials can
still fail when an object operation is first attempted. Other components may
normalize values or fail later during listener setup. Treat startup and operation
warnings as actionable, and verify both health endpoints, SMTP receipt, and
attachment download after configuration changes.

## Backup and upgrade procedure

1. Stop new SMTP traffic.
2. Copy or snapshot the complete mail directory/volume.
3. Record the image tag or binary version and effective flags/environment.
4. Start the new version against a copy when the archive is important.
5. Check `/healthz`, open representative HTML messages and attachments, then
   send a new test message.
6. Keep the backup until rollback is no longer required.

Record and deploy the published `ghcr.io/soulteary/owlmail@sha256:<digest>` in
repeatable environments. Tags are aliases; `main`, `latest`, major, and minor
tags are intentionally moving.

## Troubleshooting

| Symptom | Checks and action |
|---|---|
| Web UI is unreachable | Verify `-web-ip`/`OWLMAIL_WEB_HOST`, port publication, and `/healthz`; inspect startup logs for bind or certificate errors |
| Browser repeatedly asks for credentials | Confirm the effective username/password; a generated password changes on restart; clear stale browser credentials if needed |
| SMTP is still open after setting SMTP credentials | Current inbound SMTP authentication is not enforced; isolate the listener with interface binding and network controls |
| Container is unhealthy with HTTPS | Override the image's HTTP healthcheck with an HTTPS probe and correct certificate trust |
| Browser notification does not appear | Enable it from the inbox, use HTTPS or localhost, and restore site permission in browser settings |
| Webhook delivery is slow | Check receiver latency, timeout and retry settings; lower retries or fix the receiver before raising concurrency |
| SMTP works but direct SMTPS does not | Publish port 465, check certificate paths and runtime permission to bind a privileged port |
| Relay API returned success but no message arrived | The relay is asynchronous; inspect OwlMail logs, outgoing SMTP connectivity, recipient syntax, and receiver logs |
| Messages disappear after container recreation | Mount a volume at `/app/mail`; an unmounted container filesystem is disposable |
| API client breaks after MailDev migration | Adapt the `/api` versus `/api/v1` prefix, pagination envelope, explicit read operation, and native WebSocket protocol |

Useful checks:

```bash
docker logs --tail 200 owlmail
curl --fail http://localhost:1080/healthz
curl -u admin:secret http://localhost:1080/api/v1/version
```
