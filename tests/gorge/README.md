# Pinned Gorge SMTP integration

This test runs the **unchanged real Gorge mailer process** at commit
`e9ca3b7af2cc8914207fc22ffb499f5069d3364c` against OwlMail's isolated mailbox
components over actual implicit-TLS SMTP and HTTP listeners. All identities,
passwords, messages and addresses are synthetic; recipients use the reserved
`example.test` domain. No provider credentials or inherited Gorge configuration
reach the subprocess. Its private configuration contains exactly one loopback
SMTP adapter and zero retries, so there is no real-provider failover.

The Integration Examples workflow builds the pinned external repository in a
separate directory and runs this build-tagged test on Linux. It verifies the
binary's embedded source revision and clean VCS state. Gorge is not added to
OwlMail's Go module, modified, vendored or shipped. Missing/wrong binaries,
unsupported platforms, failed TLS, unavailable listeners and cleanup deadlines
fail the explicitly invoked suite; there is no skip path.

## Run on Linux

Go must satisfy both repositories' `go.mod`. Downloads and compilation need
network access; the running test needs only loopback listeners. Use a fresh
scratch directory, not an existing Gorge deployment:

```bash
scratch=$(mktemp -d)
git clone https://github.com/soulteary/gorge.git "$scratch/gorge"
git -C "$scratch/gorge" checkout --detach e9ca3b7af2cc8914207fc22ffb499f5069d3364c
go -C "$scratch/gorge/go" build -mod=readonly -o "$scratch/gorge-mailer" ./cmd/gorge-mailer
OWLMAIL_GORGE_BINARY="$scratch/gorge-mailer" \
  go test -race -tags gorge_integration ./tests/gorge \
  -run '^TestPinnedGorgeSMTP$' -count=1 -timeout=120s -v
```

The test uses short-lived private certificates and Linux `SSL_CERT_FILE` /
`SSL_CERT_DIR` to give the external, unmodified Gorge process a dedicated trust
root. It does not disable certificate or hostname verification and never
modifies system trust. Darwin's system verifier does not honor that trust
configuration; invoking this test there fails explicitly. On a non-Linux host,
run Linux binaries inside a disposable Linux container instead. Both endpoints
must be inside that container: its network can be disabled while retaining
loopback. Local Linux/arm64 container runs without `-race` are not race-test
results; CI executes the Linux race-enabled command above.

Each test listener and subprocess has a bounded startup/shutdown deadline.
The test signals and waits for its own Gorge processes, kills only those exact
processes if graceful exit times out, closes its SMTP/HTTP servers, and lets Go
remove its private temporary state. A port selected for Gorge or implicit SMTP
can be taken before bind; that causes a test failure rather than silently
selecting another service.

## Assertions and limits

- Gorge's real `ssl` adapter verifies TLS and authenticates with a single
  delivery-only system account. Its accepted receipt identifies `owlmail-only`.
- To and Cc become distinct inbox copies with different message IDs. Each API
  envelope lists only that inbox's matched recipient. Foreign raw IDs return
  404. SMTP credentials supplied as HTTP Basic authentication cannot list or
  read mail.
- An explicitly unverified synthetic identity cannot claim pending history.
  Later verified assertions in this test are synthetic component inputs; they
  do **not** prove a real upstream verification contract.
- A transaction containing an unknown-domain RCPT is permanently rejected before
  DATA. Invalid credentials, an untrusted CA and an OwlMail outage fail without
  publishing additional inbox copies. The mailer list contains exactly one
  adapter, and no alternate backend is configured.
- Original MIME is deliberately preserved, including **both** To/Cc addresses
  and body text mentioning the other recipient. Both copies retain identical
  raw bytes; multipart text/HTML, a chosen Message-ID and binary attachment
  payload are checked. Inbox isolation does not redact message content.
  **Phorge must authorize recipients and content before calling Gorge.**

See the [multiuser release-readiness checklist](../../docs/en/Multiuser-Release-Readiness.md) for the remaining production gates.

This is a Gorge SMTP compatibility test, not a running Phorge notification or
Stargate deployment. The HTTP request follows the pinned Phorge/Gorge contract;
it does not execute the PHP adapter, permission decisions, durable delivery
ledger, worker retries or unknown-outcome reconciliation. It does not prove
exactly-once delivery or the behavior of other Gorge backends.

Real Stargate has not supplied the trusted verified-email declaration needed
for automatic historical claiming. That remains a release blocker; a synthetic
verified header is not a substitute. Production OwlMail continues to reject
multiuser mode. This test constructs internal components only, is excluded
without `gorge_integration`, and adds no runtime flag or published fixture.
