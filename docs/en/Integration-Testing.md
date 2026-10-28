# Integration testing with OwlMail

OwlMail is a test boundary for applications that send email. Point the
application's SMTP client at OwlMail, trigger the behavior under test, then
assert the captured message through native HTTP read operations. Use a unique
recipient or subject for every test so parallel jobs cannot consume each
other's mail.

## Start a pinned test gateway

```bash
docker run --rm -d --name owlmail-test \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  ghcr.io/soulteary/owlmail:0.12.0

for attempt in $(seq 1 30); do
  if curl --fail --silent --connect-timeout 2 --max-time 3 \
    http://127.0.0.1:1080/readyz >/dev/null; then
    break
  fi
  if [ "$attempt" -eq 30 ]; then
    docker logs owlmail-test
    exit 1
  fi
  sleep 1
done
```

Configure the system under test with SMTP host `127.0.0.1`, port `1025`, and
no TLS or authentication for this local profile. CI that requires a
cryptographically fixed image should replace the tag with the manifest form
`ghcr.io/soulteary/owlmail@sha256:<digest>` recorded during release
verification.

OwlMail **0.12.0** includes local storage and outgoing readiness, cursor-based waits, test-run selectors, and `/check` assertions. All flows below run against the pinned stable instance above. Confirm ingress with a real test message.

## Bounded polling flow

1. Generate a collision-resistant recipient such as
   `signup+<test-run-id>@example.test`.
2. Trigger the application action that sends mail.
3. Poll `GET /api/v1/emails` with `to` and, when useful, `q` filters.
4. Fetch `GET /api/v1/emails/:id` and assert the subject, recipients, text,
   sanitized HTML, envelope, and attachment count.
5. Fetch attachment bytes or raw source only when the test needs them.
6. Delete only the matching email, or discard the whole isolated container.

Example polling request:

```bash
curl --fail --get http://127.0.0.1:1080/api/v1/emails \
  --data-urlencode 'to=signup+run-42@example.test' \
  --data-urlencode 'q=Verify your account' \
  --data-urlencode 'limit=10'
```

The collection response contains `total`, `limit`, `offset`, and `emails`.
An empty result is not a delivery failure until the test's own deadline has
expired. Prefer bounded retry with a short interval over a fixed sleep.

## Recommended cursor-based flow

1. Obtain `GET /api/v1/emails/cursor` **before** triggering application mail.
2. Send mail with a unique `X-OwlMail-Run-ID` and exact SMTP envelope recipient.
3. Call `POST /api/v1/emails/wait` with `after`, `run_id`, `recipient`, `count`,
   and a bounded `timeout_seconds`. It includes retained matches delivered
   after the cursor, even if they arrived before the wait began.
4. Check `timed_out` and `emails`; a timeout is a normal HTTP 200 response.
   `count` is an at-least threshold, not an exactly-once assertion.
5. Use `POST /api/v1/emails/:id/check` or inspect the detached detail.
6. Clean up with `DELETE /api/v1/test-runs/:runID/emails` after assertions;
   inspect the `failed` array and `more_possible` before considering it done.

Cursors expire on process restart. Test-run IDs are selectors, not tenant
authorization. The [delivery wait contract](./Delivery-Waits-and-Test-Runs.md)
defines count limits, quotas, pending Relay protection, and bounded cleanup.
The [README example](../../README.md#wait-check-and-clean-up-in-ci) shows a
complete stable-release test with Python's standard library.

## Isolation choices

| Scope | Recommended isolation |
|---|---|
| One local test process | Unique recipient and subject per test |
| Parallel test workers | One OwlMail container per worker, or a worker-specific recipient namespace |
| CI job | One disposable container per job |
| Persistent test environment | Configure retention and delete only IDs owned by the test |

Do not call `DELETE /api/v1/emails` from a shared environment: it clears mail
belonging to other tests. Reading a native v1 email does not mark it as read.

## Failure diagnosis

| Symptom | Check |
|---|---|
| SMTP connection refused | Container state, port mapping, and the application's SMTP host |
| API is ready but no message appears | Application SMTP logs, envelope recipient, and query filters |
| A test finds another test's message | Use a unique recipient and avoid subject-only matching |
| HTML assertion differs | Assert sanitized HTML, or use `/source` when exact RFC 5322 bytes matter |
| Intermittent timeout | Capture a cursor before triggering; for live MCP waits without a cursor, register the wait concurrently before the trigger; bound every REST poll |

Runnable dependency-free examples for JavaScript, Go, and Python are in
[examples/testing](../../examples/testing/README.md). For exact routes and
response shapes, see the [API reference](./API-Reference.md). For agent-driven
tests, see [AI agent testing](./AI-Agent-Testing.md).
