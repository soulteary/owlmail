# Stargate deployment and operations contract

This is a deployment preview. Normal OwlMail binaries still reject multiuser
startup. Passing `doctor`, rendering Compose, or running synthetic identity tests
does not open that gate. Standalone needs none of the services described here.

## One supported integration shape

Users open one inbox URL, authenticate with Stargate, and receive their own
personal inbox. They do not create OwlMail passwords, select spaces, or configure
SMTP. Warden supplies stable directory identities; Stargate authenticates and
asserts identity; Phorge decides notification recipients and content permissions;
Gorge uses its existing SMTP transport; OwlMail enforces mailbox ownership.

Use Warden's existing `REQUIRE_EXPLICIT_USER_ID=true` and include `user_id` in its
response fields. The authoritative source must preserve each ID across contact
changes and never reuse it for another person. Fix OwlMail's identity namespace
per authority. Neither hashing an address nor changing a hash strategy supplies
this lifecycle guarantee. See the [identity contract](Stargate-Identity-Contract.md).

The current deployed verified-email contract remains unaccepted. Never inject
`X-Auth-Email-Verified: true` unconditionally. Listed mail, password, SMS, TOTP and
generic AMR are not evidence that the exact asserted email belongs to the subject.
Old sessions without evidence must remain unverified after any upstream upgrade.

Prefer `explicit-history`: trusted current-email evidence binds future delivery,
while old pending history stays isolated. Use `verified-email` only when the
operator guarantees continuous ownership throughout the pending-history window.
Current verification alone cannot establish historical ownership.

## Small configuration surface, strict network boundary

Configure one site URL, a fixed identity namespace, a private proxy peer, allowed
recipient domains, system SMTP credentials, certificate paths and a persistent
state volume. Keep the existing bounded quota and retention defaults initially.
The [multiuser layout](../../examples/multiuser/README.zh-CN.md) is a preview, not a
production launch command or a bundled Stargate deployment.

The proxy must strip incoming identity headers and copy only successful Stargate
response headers. Missing, rejected or timed-out authentication must never reach
protected OwlMail routes. API authentication failure should return 401 rather than
redirect a fetch to a successful HTML login page. Browser navigation may redirect
to login. Do not expose the backend Web port publicly; restrict access by other
containers on the same network. OwlMail checks the actual connection peer, not
X-Forwarded-For. A trusted CIDR alone does not authenticate a process on a shared
network. Use transport protection when crossing an untrusted host/network boundary.

Use a reliable local filesystem. Put mail and the authority database in separate
subdirectories of the same state volume; the database must remain outside the mail
directory. Mount configuration, SMTP accounts and keys read-only with permissions
for the runtime UID. Never solve a mapping error by making private keys public.

## SMTP integration and failure behavior

Use system credentials per sending application, not per end user. Restrict each
account to the deployment namespace and intended recipient domains. Validate the
server certificate and hostname. Gorge needs no new adapter or identity password.
Recipients are SMTP envelope recipients; message headers never authorize reading.
A compromised sender may inject mail within its allowed scope even though it
cannot read inboxes. Keep persistent admission quotas and bounded pending retention.

A dedicated capture domain is useful only if notification recipient mapping and
Stargate's trusted asserted address are consistent. Changing only the SMTP domain
breaks claiming. Never fall back to real external recipients when capture fails.
Phorge remains responsible for the notification's business authorization.

## Credential rotation and emergency revocation

Account files are loaded at startup. Editing the file alone does not revoke a
credential in the running process. Use a controlled restart; do not assume reload.

1. Add a new enabled system account while keeping the old account temporarily.
2. Restart the capture service using the approved deployment procedure to load it.
3. Update Gorge's existing SMTP configuration, then verify delivery with synthetic
   content into the intended test identity. Do not print passwords in logs.
4. Remove or disable the old account and restart again; verify old authentication
   is rejected and new delivery still works.

For emergency revocation, stop or restrict SMTP ingress and close active sessions
before restarting with the compromised account disabled. Verify rejection before
reopening ingress. Do not promise that editing credentials cancels an already
accepted DATA transaction. Document the durable acceptance boundary and preserve
accepted messages; clients may retry after disconnect and produce duplicates.
Message-ID is untrusted and must not become a global cross-user deduplication key.

## Address changes and reassignment

An email change must not change a stable subject's personal inbox. Removal from an
assertion does not automatically release an old route. Before an address is given
to another person, block capture for that address at the sending boundary and
pause affected delivery. Preserve the old owner's history. Do not modify the live
control database with ad hoc SQL or automatically transfer an existing binding.

Use the local [offline recipient-route workflow](Recipient-Route-Operations.md)
for read-only inspection, exact plans, audited blocking and reassignment of future
delivery to an existing personal mailbox. Apply requires stopped writers, the
expected route ID/revision and the exact confirmation token, and refuses affected
pending recovery work. A mistaken block can be explicitly resumed to the original
owner with the same checks. Historical mail never moves. There is no online
administration endpoint; this command does not enable gated multiuser serving or
replace real upstream acceptance.

## Diagnostics and recovery

`owlmail doctor` is offline and read-only. It cannot prove proxy isolation,
network reachability, storage write access, upstream evidence, or real notification
delivery. Keep readiness local: unavailable identity verification denies reads,
but must not trigger a restart loop or discard safely authenticated SMTP delivery.

Back up the authority database, mail and required configuration at one consistent
point. Preserve or separately protect the matching certificate/key versions. Use a
verified coordinated snapshot or stop writes; copying only a live SQLite main file
is not a backup. Restore and inspect offline before reopening ingress. Losing the
authority database is a hard recovery failure; mail indexes cannot reconstruct it.

Audit binding, conflict, reassignment, migration, rotation and recovery with bounded
internal IDs and request correlation. Do not log bodies, OTPs, passwords, cookies,
Authorization or raw identity headers; do not use arbitrary users/addresses as
metric labels. Downloaded or exported content cannot be revoked retroactively.

Real identity and notification acceptance is still required: [T02, T16 and T17](
Multiuser-Release-Readiness.md). Adapter tests, source review and the published
standalone release do not substitute for that acceptance. Multiuser MCP,
WebSocket, Relay, Webhook and shared spaces remain unavailable.
