# OwlMail Operations and Troubleshooting

This guide covers repeatable local and container deployments, security defaults,
readiness, persistence, webhook capacity, shutdown behavior, and common failure
modes. See the [API Reference](./API-Reference.md) and
[Webhook Forwarding](./Webhook-Forwarding.md) for protocol details.

## Deployment profiles

### 1. Minimal local inbox

```bash
./owlmail
```

This listens on localhost by default: SMTP on 1025 and HTTP on 1080. It keeps
mail in memory unless `-mail-directory` is set. This profile is intended for one
developer on a trusted machine.

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

### 3. Persistent Docker deployment

```bash
docker volume create owlmail-data
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:latest
```

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
  ghcr.io/soulteary/owlmail:latest
```

Configure both values for stable automation. A username alone causes OwlMail to
generate a new password at each process start and print it once to stderr. A
password alone uses `admin` as the username. Retrieve startup output with:

```bash
docker logs owlmail
```

Basic Auth protects the UI, API, assets, and WebSocket endpoints, but
`/healthz` and `/api/v1/health` intentionally remain public for probes. Use
HTTPS or a trusted reverse proxy when credentials cross a network.

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
  ghcr.io/soulteary/owlmail:latest
```

Verify that the container runtime permits the non-root process to bind port 465.
If it does not, grant only the required bind-service capability according to the
runtime's security policy.

## Webhook capacity profiles

`-webhook-max-concurrency` is process-wide across all targets and messages.
Acquiring a delivery slot happens before a handler goroutine is created, so a
finite limit provides actual backpressure rather than leaving an unbounded set
of waiting goroutines.

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

The limit is not a queue size. When all slots are occupied, new-message event
processing waits for a slot. Include target timeout and retry duration when
estimating the worst-case hold time. Monitor receiver latency and error rate
before raising the limit.

## Shutdown and delivery guarantees

On `SIGINT` or `SIGTERM`, OwlMail closes its SMTP server and the process exits.
Do not assume that an in-flight webhook or relay request completes during this
window. For deployments that require stronger delivery guarantees:

1. Stop upstream SMTP traffic before terminating OwlMail.
2. Allow the longest configured webhook timeout/retry window to elapse.
3. Make receivers idempotent and retain their request logs.
4. Use HMAC signatures and a stable event identifier in custom payloads when
   deduplication matters.

Webhook forwarding is an integration notification mechanism, not a durable
message queue.

## Backup and upgrade procedure

1. Stop new SMTP traffic.
2. Copy or snapshot the complete mail directory/volume.
3. Record the image tag or binary version and effective flags/environment.
4. Start the new version against a copy when the archive is important.
5. Check `/healthz`, open representative HTML messages and attachments, then
   send a new test message.
6. Keep the backup until rollback is no longer required.

Use immutable release or commit tags in repeatable environments instead of
`latest`.

## Troubleshooting

| Symptom | Checks and action |
|---|---|
| Web UI is unreachable | Verify `-web-ip`/`OWLMAIL_WEB_HOST`, port publication, and `/healthz`; inspect startup logs for bind or certificate errors |
| Browser repeatedly asks for credentials | Confirm the effective username/password; a generated password changes on restart; clear stale browser credentials if needed |
| Container is unhealthy with HTTPS | Override the image's HTTP healthcheck with an HTTPS probe and correct certificate trust |
| Browser notification does not appear | Enable it from the inbox, use HTTPS or localhost, and restore site permission in browser settings |
| Webhook delivery is slow | Check receiver latency, timeout and retry settings; lower retries or fix the receiver before raising concurrency |
| SMTP works but direct SMTPS does not | Publish port 465, check certificate paths and runtime permission to bind a privileged port |
| Messages disappear after container recreation | Mount a volume at `/app/mail`; an unmounted container filesystem is disposable |
| API client breaks after MailDev migration | Adapt the `/api` versus `/api/v1` prefix, pagination envelope, explicit read operation, and native WebSocket protocol |

Useful checks:

```bash
docker logs --tail 200 owlmail
curl --fail http://localhost:1080/healthz
curl -u admin:secret http://localhost:1080/api/v1/version
```

