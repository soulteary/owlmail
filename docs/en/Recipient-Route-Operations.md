# Offline recipient route operations

The public multiuser serving gate remains closed. This local operator command
prepares and applies future-delivery routing changes; it starts no listeners,
creates no users, grants no read permissions and moves no historical messages.
Normal standalone serving remains unchanged.

## Inspect and plan

Use the exact control database, namespace and recipient address. The default
`inspect` action reads the selected route and its current personal owner, including
its route UUID and revision. It creates no writer fence or authority state.
Inspection can coexist with a live upgraded writer but is only a point-in-time
view, not a consistent mail backup or permission to mutate live state.

```sh
owlmail recipient-route \
  -control-database /data/control/control.sqlite \
  -identity-namespace main -address alice@example.test
```

Operator output contains the selected address, stable subject and mailbox IDs.
Protect this output; it is not intended for browser users, metrics or ordinary
application logs. Never enter passwords, OTPs or other secrets in `-reason` or
any command arguments. Reasons are nonempty, at most 256 UTF-8 bytes, with no
control characters or surrounding whitespace.

Before applying any change, stop **all** writers, including older OwlMail
versions that do not honor the advisory fence, and take a coordinated complete
backup. Follow [the writer-fence contract](Control-Writer-Lock.md). Never delete
a live `.writer-lock` file to bypass an active-writer refusal.

## Block first

Copy the exact route UUID and revision from inspect. `block` without `-apply`
produces a read-only plan with a `confirm_plan` SHA256 token. Replace the example
revision below with the actual inspected value.

```sh
owlmail recipient-route \
  -control-database /data/control/control.sqlite \
  -identity-namespace main -address alice@example.test \
  -action block -expected-route-id "$ROUTE_ID" -expected-revision 3 \
  -reason 'Address reassigned by identity authority'
```

Rerun the same command with `-apply -offline -confirm-plan "$PLAN_HASH"` to apply
that exact plan. The token binds the operation, namespace, address, route ID,
revision, source/target and reason. A changed route or different decision requires
a fresh plan. A mistyped token is rejected before writable state is opened.

Only a bound personal route can be blocked. The command appends operator audit
evidence and increments its revision. Historical mail and personal ownership do
not change. New delivery to the blocked route is refused; no real-mail fallback
is enabled. A crash between block and reassignment deliberately leaves it blocked.

## Reassign future delivery

Inspect again and use the **new** blocked revision. The new stable subject must
already have a personal mailbox in the same namespace. Confirm its lifecycle and
current address ownership in the upstream authority; this command cannot prove
those external facts and never creates an identity from an email address.

```sh
owlmail recipient-route \
  -control-database /data/control/control.sqlite \
  -identity-namespace main -address alice@example.test \
  -action reassign -expected-route-id "$ROUTE_ID" -expected-revision 4 \
  -target-subject "$EXISTING_STABLE_SUBJECT" \
  -reason 'New address ownership confirmed'
```

Review the plan and repeat with `-apply -offline -confirm-plan "$PLAN_HASH"`.
Reassignment requires matching operator-block evidence for the exact blocked
revision; ordinary deny routes cannot silently become new bindings. The explicit
existing target must be valid, affected pending merge/retention recovery must finish
first, and both route and audit commit in one SQLite transaction. No `trusted_login`
evidence is fabricated and no merge task is created. The old personal inbox and
all its historical messages remain with its original stable identity.

Apply revalidates under the exclusive writer fence and transaction. No changed
plan is silently retried. After an uncertain command result, inspect the route
and verify the revision/target before making another decision. A stale SMTP RCPT
snapshot cannot follow a reassignment into a different mailbox.

## Undo an accidental block

Use the same `reassign` planning flow with `-target-subject` set to the original
stable subject. Review a fresh plan and apply with its exact confirmation token
and stopped-writer declaration. This restores future delivery to the same owner
and writes distinct `operator_route_resume` audit evidence. It does not move
history or weaken the blocked-revision/recovery checks; do not use SQL or a
third-party mailbox as an intermediate workaround.

## Resume and recover

Check the new route, reopen the authority for normal integrity validation, and
verify new delivery goes only to the intended identity while old history remains
isolated. Do not resume an unaccepted multiuser deployment or bypass its gate.
If a graph/recovery check fails, stop and use the recovery procedure rather than
editing SQLite directly.

A backup rollback can restore an old route even though the upstream address has
already been reassigned. Keep ingress stopped until the restored routing has
been reconciled with the current upstream authority. Never restore only the
control database against a different mail snapshot, or blindly resume old routing.
