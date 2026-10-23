# Reproducible delivery faults

The loopback-only receiver in `examples/faults` exercises Webhook and Relay failures without modifying OwlMail's SMTP acceptance or production configuration. It discards all payloads and does not relay messages.

This drill uses OwlMail **0.11.3** and its manual delivery retry workflow. Run the receiver from a `v0.11.3` source checkout; it is a test fixture, not a command bundled in the installed OwlMail binary.

## Webhook retries, timeouts, and terminal failure

```sh
go run ./examples/faults -protocol http -listen 127.0.0.1:18081 -failures 2 -http-status 503
```

Point a webhook target at `http://127.0.0.1:18081/`. Its first two requests per `X-OwlMail-Delivery-ID` fail with 503; subsequent requests return 204. `X-Fault-Attempt` makes the ordinal observable. Different delivery IDs have independent counters, including concurrent runs. Counters reset when the receiver restarts. Set target retries to zero to exercise terminal failure and explicit replay, or two to verify automatic recovery. For timeouts, add `-delay 2s` and set the webhook timeout below two seconds. Use `-http-status 400` to verify permanent HTTP errors.

## Relay temporary and permanent failures

```sh
go run ./examples/faults -protocol smtp -listen 127.0.0.1:12525 -failures 2 -smtp-code 451 -smtp-stage data
```

Configure outgoing SMTP to host `127.0.0.1`, port `12525`, with TLS and authentication disabled. Confirm the test recipient, then submit a relay. The first two transactions per last SMTP recipient fail after DATA; the third succeeds through the bounded relay retry queue. Typed SMTP 4xx responses are categorized as `temporary_smtp` and retried; 5xx responses are `permanent_smtp` and terminal. Use one recipient per transaction for an unambiguous counter. `-smtp-stage rcpt` injects failure at RCPT instead. `-smtp-code 550` verifies permanent failures; `-delay` exercises stage deadlines. The selected stage's counters are independent per exact recipient, so use distinct recipients for parallel tests.

## Acceptance and restart checklist

1. Run temporary faults and assert attempts, queued retry state, then terminal success.
2. Run permanent faults and assert a categorized terminal failure.
3. Stop OwlMail with a queued durable job, restart with the same spool and configuration, and verify recovery without losing its source.
4. Replay a retained failed job with the current [delivery workflow API](./API-Reference.md#delivery-workflows) after the fault ordinal has been exhausted; verify success and that successful webhook targets are not resent. The source email and target configuration must still exist.
5. Treat remote acknowledgements as potentially ambiguous: receivers must deduplicate webhook delivery IDs and relay may deliver more than once.

The receiver allows only literal loopback IP addresses, at most 1,000 initial failures, a 30-second delay, 2 MiB payloads, and 10,000 distinct keys. When key capacity is exhausted, restart it. Do not use it as a general SMTP server or authenticated webhook verifier. Shutdown cancels injected delays.
