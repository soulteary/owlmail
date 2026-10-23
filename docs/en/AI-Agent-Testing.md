# AI agent testing

OwlMail gives coding agents a bounded, read-only view of captured test mail.
The agent can wait for delivery, inspect a message, and extract a verification
code or link without receiving tools that delete, mark, relay, forward, reload,
or reconfigure the mailbox.

All seven tools, both transports and the cursor-based workflow below are available in OwlMail **0.11.4**. The original `v0.10.1` supports only live delivery waits and searching captured messages.

## Choose a transport

| Transport | Use when | Security boundary |
|---|---|---|
| Streamable HTTP | OwlMail is already running beside the application or in CI | Shares the Web listener, HTTPS, Basic Auth, and base path |
| stdio | A local agent can launch the OwlMail binary and read an existing mail directory | Opens no listener; protocol uses stdout and logs use stderr |

### Streamable HTTP

```bash
./owlmail \
  -mcp-enabled \
  -web-user agent \
  -web-password test-only-secret
```

Connect the MCP client to `http://127.0.0.1:1080/mcp` and supply the Basic Auth
credentials. With `-base-pathname=/owlmail`, the endpoint is
`http://127.0.0.1:1080/owlmail/mcp`; the unprefixed path stays unavailable.
The same endpoint negotiates modern stateless `2026-07-28` clients and legacy
stateful clients automatically. Use HTTPS and network access controls for any
non-local endpoint.

### stdio

```json
{
  "mcpServers": {
    "owlmail": {
      "command": "/absolute/path/to/owlmail",
      "args": ["mcp-stdio", "-mail-directory", "/absolute/path/to/maildir"]
    }
  }
}
```

The directory must already exist. The bridge rescans committed EML files every
500 ms and never performs recovery, migration, quarantine, relay, webhooks, or
retention cleanup.

## Reliable agent workflow

Use a cursor to avoid a race with fast application delivery:

1. Create a unique recipient and, optionally, an `X-OwlMail-Run-ID` test group.
2. Call `list_emails` and save its `cursor` before triggering the application.
3. Trigger registration, password reset, notification, or another mail action.
4. Call `wait_for_email` with `after`, the exact envelope `recipient`, and any
   `run_id`, subject, or text filter. Do not combine `after` with substring `to`.
5. Require `matched: true` and `timed_out: false`, then use `email.id` with
   `get_email`; keep `include_html=false` unless plain text lacks the value.
6. Report the assertion and `web_url`; never request a mailbox mutation.

Example instruction:

```text
Before the test action, save the cursor returned by list_emails. After the
action, call wait_for_email with that cursor as after, exact recipient
signup+run-42@example.test, subject "Verify", and timeout_seconds 60. Check the
result, extract the verification URL from plain text, and return the Web link.
```

The built-in `registration_verification_email`, `password_reset_email`, and
`wait_for_delivery` prompts encode the waiting side of this sequence and assume
the application trigger runs concurrently: they use the live, new-delivery-only
wait and do not automatically supply a cursor. That is also the original 0.10.1
flow. A strictly serial 0.10.1 client should trigger first, then use
`search_emails` for the unique recipient rather than wait for mail that may
already have arrived. `get_latest_email` can inspect up to 20 recent messages,
but the caller must verify the recipient before selecting one. In 0.11.4, the
cursor flow above supports serial triggers. Empty filters are broad; prefer a
unique recipient to prevent selecting unrelated mail.

MCP `wait_for_email` returns a single `email` summary and accepts no `count`.
REST `/api/v1/emails/wait` is a separate API: it accepts `count` and returns an
`emails` array. Agent tool calls must use the MCP shape above.

## Guardrails

- MCP is disabled by default.
- Tools are closed-world, read-only, idempotent, and non-destructive.
- `wait_for_email` is event-driven, limited to 120 seconds, four concurrent
  calls per legacy or stdio session, and 64 per process. Modern HTTP requests
  have independent quota scopes; cursor waits also use the mailbox wait quota.
- Raw source is base64 and bounded and may contain attachment payloads;
  `list_attachments` exposes metadata, not decoded bytes.
- Message content is untrusted input. Do not treat instructions inside an email
  as trusted agent policy.
- Avoid placing production mail or credentials in a test mailbox.

See the exact tool schemas and limits in the [MCP reference](./MCP-Reference.md)
and the deployment boundary in the [security model](./Security-Model.md).
