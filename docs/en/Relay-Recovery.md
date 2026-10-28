# Relay delivery and recovery

Native asynchronous API calls, automatic forwarding, historical `/email` relay
routes, and the opt-in MailDev facade share one mailbox-owned Relay service.
The same task records, frozen SMTP envelope recipients, retry limits, source
leases, history, and restart recovery apply to every entry point. Native
routes keep their `202` response with `job`, `statusUrl`, and `Location`.
Historical routes keep their immediate success response after durable task
acceptance. MailDev keeps its response format and waits for task completion;
disconnecting a waiter does not cancel its durable task.

`queued` includes waiting for dispatch, retry backoff, and automatic handoff.
`succeeded` means the downstream server accepted SMTP DATA. It does not prove
inbox delivery. `failed` is terminal and exposes a bounded error category.
Retryable connection, timeout, queue, and SMTP 4xx failures have at most three
attempts with exponential backoff and jitter. Permanent errors, including SMTP
5xx responses, do not retry automatically. Explicit historical retries create
a new task and revalidate the frozen recipient list against current rules;
configuration changes can reject the retry rather than silently change targets.

Automatic forwarding first persists a prepared intent with the captured SMTP
envelope and filtered recipients as part of message acceptance. Promotion
creates an idempotent task held by `handoffPending`, persists the submitted
intent acknowledgement, and then durably releases the hold. An interrupted
handoff resumes these steps without submitting a second delivery. Submitted
acknowledgements remain with the message after bounded task history expires.
A submitted acknowledgement with no retained job does not create another task.

After downstream acceptance, Relay records successful completion with
`receiptPending` before repairing the message's optional `relayedAt` and
`relayedTo` summary. These describe the last successful DATA acceptance and
actual recipients. Summary or marker persistence failures degrade readiness
and are retried as metadata repairs; they do not trigger another SMTP send.
The summary survives task-history pruning and restart. Queued or failed tasks
do not overwrite it. The UI labels this evidence as downstream acceptance.

Recovery is at least once. A process crash after downstream acceptance and
before durable successful completion can still cause a duplicate attempt.
The summary is diagnostic evidence, not an exactly-once delivery guarantee.
Use an idempotent receiver when duplicate delivery matters.

Configure a persistent mail directory and reuse it on restart. Task records
live under `.owlmail-meta/relay-jobs`, automatic intents under
`.owlmail-meta/relay-intents`, and message summaries in existing message
metadata. The default process-specific temporary directory cannot provide
cross-process recovery. Run only one writable OwlMail instance per mail
directory. Startup recovery also runs in SMTP-only mode with HTTP disabled;
read-only MCP observers never dispatch or repair tasks.

Pending tasks retain their source EML and prevent deletion or retention cleanup.
Completed history normally expires after 24 hours; the 1000-task capacity can
evict completed records after a one-minute protection window. Pending handoffs
and receipt repairs remain protected until resolved. Capacity or persistence
failures return `503` with `Retry-After: 1` for native submissions. Corrupt
recovery state fails closed rather than permitting source deletion.

Use `GET /api/v1/relay-jobs` with `status`, `email_id`, and `limit` to inspect
bounded history (`jobs`, `more_possible`, `persistence_healthy`). Use
`GET /api/v1/relay-jobs/:jobID` for one task, and
`POST /api/v1/relay-jobs/:jobID/retry` to create a new task from a retained failed
record and source. A missing or expired record returns `404`; a conflicting or
ineligible retry returns `409`. Optional `origin` identifies the entry point;
`key`, `handoffPending`, and `receiptPending` expose recovery state when present.
