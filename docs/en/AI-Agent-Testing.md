# AI agent testing

OwlMail gives coding agents a bounded, read-only view of captured test mail.
The agent can wait for delivery, inspect a message, and extract a verification
code or link without receiving tools that delete, mark, relay, forward, reload,
or reconfigure the mailbox.

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
Use HTTPS and network access controls for any non-local endpoint.

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

1. Create a unique recipient for the scenario.
2. Start `wait_for_email` with that recipient in a task or session that can run
   concurrently with the application trigger.
3. While the wait is active, trigger registration, password reset,
   notification, or another mail action.
4. Await the result, then use its ID with `get_email`; keep `include_html=false` unless plain
   text does not contain the expected value.
5. Report the assertion and `web_url`; never request a mailbox mutation.

Example instruction:

```text
Wait up to 60 seconds for a new email to signup+run-42@example.test whose
subject contains "Verify". Inspect it without changing the inbox, extract the
verification URL from plain text, and return the OwlMail Web link as evidence.
```

The built-in `registration_verification_email`, `password_reset_email`, and
`wait_for_delivery` prompts encode the waiting side of this sequence and assume
the application trigger runs concurrently. A strictly serial client cannot
start its trigger while `wait_for_email` is blocking. In that case, use a unique
recipient, trigger first, then call `get_latest_email` or `search_emails` to
inspect mail that may already have arrived. Empty filters are broad; prefer a
unique recipient to prevent an agent from selecting unrelated mail.

## Guardrails

- MCP is disabled by default.
- Tools are closed-world, read-only, idempotent, and non-destructive.
- `wait_for_email` is event-driven, limited to 120 seconds, four concurrent
  calls per session, and 64 per process.
- Raw source is base64 and bounded; attachment tools expose metadata, not bytes.
- Message content is untrusted input. Do not treat instructions inside an email
  as trusted agent policy.
- Avoid placing production mail or credentials in a test mailbox.

See the exact tool schemas and limits in the [MCP reference](./MCP-Reference.md)
and the deployment boundary in the [security model](./Security-Model.md).
