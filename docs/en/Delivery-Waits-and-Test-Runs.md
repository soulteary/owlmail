# Delivery waits and test runs

A delivery cursor closes the race between triggering application mail and
registering a waiter. Obtain it **before** triggering the application:

```sh
curl http://localhost:1080/api/v1/emails/cursor
# {"cursor":"<process-epoch>:<sequence>"}
```

Send mail with `X-OwlMail-Run-ID: checkout-42`, then wait:

```sh
curl -H 'Content-Type: application/json' \
  -d '{"after":"<cursor>","run_id":"checkout-42","recipient":"user@example.test","count":1,"timeout_seconds":30}' \
  http://localhost:1080/api/v1/emails/wait
```

`recipient` matches the complete SMTP envelope recipient **case-sensitively**,
not the visible To header. `run_id` matches the header exactly. Run IDs contain
1–128 ASCII letters, digits, `.`, `_`, `:`, or `-`; omission leaves mail ungrouped.
A duplicate or invalid run header rejects the SMTP transaction. Grouping survives
EML reloads without changing the sidecar format. It is a selector, not tenant
isolation or an authorization boundary.

The response contains `emails`, `cursor`, `timed_out`, and
`waited_milliseconds`. `count` defaults to 1 and is an **at-least** threshold
from 1 through 100. The first matching retained messages are returned; this is
not an exactly-one assertion. Timeout defaults to 30 seconds, maximum 120. A
normal timeout is HTTP 200 with `timed_out: true` and any partial matches. Reuse
the **original** cursor when retrying an incomplete count. A timeout does not
promise that mail will never arrive later.

At most 64 mailbox waits run concurrently; excess requests receive 429 and
`Retry-After: 1`. Shutdown cancels waits with 503. Invalid or previous-process
cursors receive 400. Cursors are process-scoped and do not survive a restart.
Deletion/retention can remove replayable messages: obtain a fresh cursor and do
not clean up mail while a test still needs it. A wait reads detached snapshots
and never changes read state. HTTP client disconnection is subject to the
server framework's cancellation behavior; the bounded timeout always applies.

Lists, previews, and exports accept `run_id` and `recipient`. Both selectors
compose with existing filters. Run-scoped cleanup is explicit:

```sh
curl -X DELETE http://localhost:1080/api/v1/test-runs/checkout-42/emails
```

Cleanup reports `deleted`, `failed`, and `more_possible`. It takes a snapshot of
at most 1000 IDs and preserves pending relay sources; repeat if `more_possible`
is true. Concurrently arriving messages are not part of that snapshot. Never
use global mailbox deletion to clean up a shared test run.

MCP `list_emails` and `search_emails` return a `cursor` and accept `run_id` and
`recipient`. Capture that cursor before triggering the application, then pass
`after`, `run_id`, and `recipient` to `wait_for_email`. With `after`, use exact
`recipient` rather than the legacy substring `to`. Omitting `after` preserves
the existing new-delivery-only contract. Existing MCP session/process quotas
still apply; cursor-based waits also share the mailbox wait quota. MCP stays
read-only and exposes no cleanup tool.
