# Testing recipes

These recipes assume OwlMail is reachable at `http://127.0.0.1:1080` and SMTP
at `127.0.0.1:1025`. Use a unique recipient for every scenario.

## Registration verification

1. Start a bounded `wait_for_email` call for the unique recipient, or begin a
   REST poll filtered by `to`.
2. Submit the registration request to the application.
3. Assert the subject and sender.
4. Fetch the message detail and extract the URL or code from `text`.
5. Open the URL against the application and assert the account transition.

For an AI client, the `registration_verification_email` MCP prompt performs the
read-only mail portion of this flow.

## Password reset

Use a recipient unique to the test and a subject filter specific enough to
exclude registration mail. Assert that the reset URL belongs to the expected
application origin before opening it. Never send real credentials or customer
addresses to a test inbox.

## Attachment delivery

Fetch `GET /api/v1/emails/:id` and assert `attachment_count`. Then request the
known attachment path from the message metadata and validate content type,
size, and checksum. The MCP `list_attachments` tool intentionally provides
metadata only; use the HTTP API when byte-level assertions are necessary.

## Exact MIME source

Use `GET /api/v1/emails/:id/source` when the test concerns headers, MIME
boundaries, transfer encoding, or DKIM input. HTML returned in message detail is
sanitized and is therefore the right target for display tests, not exact-source
tests.

## Negative delivery

To verify that an application does not send mail, poll with a unique recipient
until a short, explicit deadline and assert `total == 0`. An MCP
`wait_for_email` timeout is a normal result (`timed_out: true`), not proof that
all future delivery is impossible.

## Parallel suite isolation

Derive the address from suite, worker, and test IDs:

```text
checkout+<run-id>-<worker-id>-<test-id>@example.test
```

Match the full address. Do not clear a shared inbox. Prefer a disposable
container per CI job and delete only messages created by that job.

## Restart recovery

Mount a temporary mail directory, send a message, stop OwlMail cleanly, restart
with the same directory, and assert the message is still queryable. Do not run
two writable OwlMail instances against one directory. A separate `mcp-stdio`
process is read-only and can inspect committed EML files.

## Webhook handoff

Use the runnable [webhook examples](../../examples/webhooks/README.md). Treat
delivery as at least once, deduplicate by `X-OwlMail-Delivery-ID`, and assert
signatures before processing. SMTP acceptance means the durable local outbox
handoff succeeded; it does not mean every remote target already returned 2xx.

## Relay testing

Only native `POST /api/v1/emails/:id/actions/relay` routes create persistent
asynchronous jobs with status URLs and bounded retry. Historical and
compatibility relay routes keep their existing response behavior. Recovery is
at least once, so the downstream test receiver must tolerate duplicates.

See [Integration testing](./Integration-Testing.md) for the base lifecycle and
the [API reference](./API-Reference.md) for exact response shapes.
