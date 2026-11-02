# Container verification

The production Dockerfile remains the only release image. The multiuser CLI
release gate is intentionally closed. No test image enables a production bypass.

## Production and component checks

Build the regular image, then the explicitly tagged synthetic fixture:

```sh
docker build --build-arg TARGETARCH=amd64 -t owlmail:container-check .
docker build --build-arg BASE_IMAGE=owlmail:container-check --build-arg TARGETARCH=amd64 \
  -f tests/container/Dockerfile.multiuser-fixture -t owlmail:container-fixture .
python3 tests/container/multiuser.py --image owlmail:container-check \
  --fixture-image owlmail:container-fixture
```

Use the architecture of the runtime (`amd64` or `arm64`). The script supports
`--runtime podman --connection <name>` and removes only its own containers and
labelled data volumes. It reports production and test-only component results
separately. Production checks cover non-root permissions, embedded HTML, SMTP
attachments, HTTP, restart persistence, packaged health probes, offline migration,
read-only credential mounts and both multiuser/stdio gates. The fixture binds
only loopback inside the container and tests actual TLS SMTP, unverified pending
mail, claims, cross-user denial and persistent restart.

The existing `healthcheck.py` exercises additional packaged probe combinations.
Its default runner is Docker; do not describe Podman verification as a Docker
engine result.

## Strict Linux headed background-tab check

This uses only a separate test image. It installs Chromium, Xvfb and Node inside
that image, and copies public Playwright dependencies from the lockfile-based
browser installation. It does not install a new host runtime or change production.

After installing `tests/browser` dependencies from its lockfile:

```sh
browser_context_dir="$(mktemp -d)"
cp tests/container/Dockerfile.browser-linux "$browser_context_dir/Dockerfile"
cp tests/container/run-browser-linux.sh tests/container/run-browser-linux.mjs "$browser_context_dir/"
cp tests/browser/multiuser.playwright.config.js tests/browser/multiuser.spec.js "$browser_context_dir/"
tar -C tests/browser -cf "$browser_context_dir/playwright-node-modules.tar" node_modules
docker build --build-arg FIXTURE_IMAGE=owlmail:container-fixture \
  -t owlmail:linux-browser "$browser_context_dir"
docker run --rm --shm-size=256m owlmail:linux-browser
rm -rf "$browser_context_dir"
```

The image default runs only `real background visibility` with
`OWLMAIL_REQUIRE_REAL_BACKGROUND=1`: actual hidden visibility is mandatory and
cannot skip. It starts a private Xvfb server, checks a real X11 setup handshake,
then launches the test with stage logs and a default 240-second total test budget
(`OWLMAIL_BROWSER_TOTAL_SECONDS`, accepted range 30..600). Browser/fixture startup
also has bounded timeouts. The runner snapshots its Linux `/proc` descendants before termination and
checks each PID start time before signalling it, including the detached fixture
process. Cleanup has a three-second TERM grace and one-second KILL bound. Exited
zombies are distinguished from live processes and left for container init to
reap. This is bounded cleanup of the owned test tree, not a global process sweep. It does not touch unrelated processes or production settings.

The previous Alpine `xvfb-run` entrypoint stalled before Node started while its
PID-1 shell waited for Xvfb's SIGUSR1 readiness handshake. The X11 socket existed,
but no fixture or browser process had begun. An `--init` control completed that
handshake. The new entrypoint avoids this wrapper and checks actual X11 readiness
instead of treating a socket file as sufficient.

Playwright-managed pages normally enable focus emulation. The background test
therefore launches one temporary native Chromium profile and attaches to its
existing default context with `connectOverCDP({noDefaults:true})`. It does not
modify `document.hidden`/visibility or freeze the page. A real JS heartbeat must
continue while hidden; over 16 seconds a new TLS SMTP message must not trigger a
preview request or list refresh. Foregrounding must show it within five seconds,
keep the selected detail and have exactly one maximum in-flight refresh.

For Linux CI or an existing Linux checkout, no test image is required. After the
usual fixture build, locked browser install and Xvfb/Node install, run from
`tests/browser`:

```sh
../../tests/container/run-browser-linux.sh bun x playwright test \
  --config multiuser.playwright.config.js --grep 'real background visibility'
```

Strict result and HTML report directories are `multiuser-background-results/`
and `multiuser-background-report/`, separate from the ordinary headless suite.
The results contain JSON measurements and a successful foreground screenshot.
To export container evidence, create a named container, start it, inspect its
exit code and use `docker cp <name>:/checks/multiuser-background-results ./...`
before removing that container. A success log must show **1 passed, no skip**;
a timeout or skip is not acceptance.

Local validation uses rootless Podman Linux/arm64 and is reported as such, not
as Docker daemon/amd64 validation. Test fixtures and browser images must not be
published or deployed as services. The production multiuser release gate stays
closed until real Stargate identity evidence and release acceptance are complete.

## Named-volume backup, numeric permissions and SQLite index recovery

Use the same production and test-only fixture images above. Build a test binary
for the runtime architecture (`amd64` for standard Docker CI, `arm64` for the
local Podman VM):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o /tmp/owlmail-recovery.test ./tests/recovery
python3 tests/container/recovery.py --image owlmail:container-check \
  --fixture-image owlmail:container-fixture \
  --recovery-binary /tmp/owlmail-recovery.test \
  --evidence-directory /tmp/owlmail-volume-evidence
```

Podman supports `--runtime podman --connection podman-machine-default`. The binary
is required: this command cannot silently skip the SQLite acceptance test. No
ports are published. The synthetic proxy fixture is below the unchanged public
production gate; the ordinary production binary still cannot start multiuser.

The drill creates separate labelled source, backup and restored named volumes,
stops the writer, archives the complete source and restores numeric UID/GID,
modes and contents into a fresh volume. A wrong-owner negative test must deny
actual reads/writes as UID 1000, followed by restoration from archive metadata.
It verifies original message UUIDs, pending claims/merge, attachments and foreign
resource rejection. Original source and backup hashes must remain unchanged.

Synthetic PEM credentials are parsed, nonempty and compared against source
SHA256; deployment JSON is parsed and checked before backup. Fixture TLS is
ephemeral, so this is not a claim of deployed TLS continuity. Rootless numeric
IDs belong to the container namespace, not the host; ACL/xattr/SELinux and
cross-host mappings require separate deployment validation.

A separate named volume runs the nonroot test binary
`TestMultiuserSQLiteIndexDeleteAndRebuild`. It removes only the derived index and
WAL/SHM after shutdown, rebuilds twice, checks actual SQL IDs/read/receivedAt and
`sqlite` ready status, and separately tests scoped isolation. The authority
control database is never deleted. Scoped queries currently use ownership
filtering of authoritative in-memory mail, not SQL tenant indexing.

Evidence includes JSON manifests, hashes and logs. Archive contents, mail and
private keys stay in owned volumes and are deleted during labelled resource
cleanup. Never upload those archives. Docker PR CI runs the same command and
uploads only the evidence directory; local rootless Podman/arm64 success does
not imply Docker daemon/amd64 or remote CI has been executed.

## Restored SMTP TLS identity

Build the latest explicitly tagged fixture image first. Then run:

```sh
python3 tests/container/tls_recovery.py --image owlmail:container-check \
  --fixture-image owlmail:container-fixture --evidence-directory /tmp/owlmail-tls-evidence
```

The fixture's new persistent certificate/key/CA options are test-only and limited
to its state root. The test verifies real STARTTLS CA/IP SAN trust, SMTP AUTH,
actual peer DER/SPKI before and after a complete stopped-volume restore, continued
mail delivery and user isolation. Wrong key/missing certificate must refuse
startup; wrong CA/hostname must reject handshake and delivery. Keys and archives
remain private and are removed. Docker PR CI runs this drill without publishing
the fixture or changing the production multiuser gate.

## Different rootless UID/GID mappings (Podman machine only)

With an existing capable Podman machine and sufficient configured subordinate IDs:

```sh
python3 tests/container/uid_mapping.py --connection podman-machine-default \
  --machine podman-machine-default --fixture-image owlmail:container-fixture \
  --output /tmp/owlmail-uid-mapping.json
```

This uses two simultaneously live, disjoint `--userns=auto` environments in the
same Linux VM. It reads actual container/VM `/proc` maps and physical volume inode
owners, verifies a wrong physical-owner copy denies separate nonroot reads/writes,
and restores numeric ownership in the target map. Content/IDs/attachments and
API isolation must survive; source content and numeric metadata must not change.
Only owned test volume roots are repaired; no global subuid or user configuration
is changed. VM `sudo` probes are read-only. This is not independent-host acceptance
or a generic Docker/native-Linux script. The user explicitly retained the
independent-host gate. Do not treat unsupported environments or skips as passes.

## POSIX ACL and user.* extended attributes

BusyBox tar in the production image is not a metadata-complete archive tool.
Build only a separate test helper; do not add these packages to production:

```sh
docker build --build-arg BASE_IMAGE=owlmail:container-check \
  -f tests/container/Dockerfile.metadata-recovery -t owlmail:metadata-recovery .
python3 tests/container/metadata_recovery.py --image owlmail:container-check \
  --helper-image owlmail:metadata-recovery --fixture-image owlmail:container-fixture \
  --evidence-directory /tmp/owlmail-metadata-evidence
```

The test compares explicit numeric POSIX access ACLs and nonempty binary/string
user xattrs, and exercises actual nonowner reads/write-open plus ACL mask changes.
The ordinary-tar negative preserves bytes/stat but loses metadata and must be
rejected. GNU tar uses numeric owner, --acls and --xattrs with user.* inclusion.
Missing tools/filesystem capabilities are unsupported/nonzero, never a successful
skip. Only logs/metadata/hashes are exported, not archives. This does not validate
NFSv4/default ACL inheritance, trusted.* or arbitrary security.* namespaces.

## Enforcing SELinux and destination-policy labels (Podman machine)

```sh
python3 tests/container/selinux_recovery.py --connection podman-machine-default \
  --machine podman-machine-default --fixture-image owlmail:container-fixture \
  --output /tmp/owlmail-selinux-evidence.json
```

Requires actual Enforcing SELinux and enabled container labeling; unsupported or
permissive environments return exit2 rather than passing. The test does not alter
system enforcement or policy. An old source MCS label causes real nonroot reads
and writes to fail; destination :Z relabel restores service and mailbox access
with unchanged file/root DAC and file hashes. The private archive excludes
security.selinux instead of treating old MCS labels as portable. :z tests only a
synthetic shared filesystem probe, not shared mailboxes. All writes/chcon apply
to uniquely owned test volumes and resources are removed.

The local VM exposes no matching AVC in the existing kernel journal. Evidence
states that limitation; no audit-pipeline acceptance is claimed. This is same-VM
existing-policy behavior, not cross-host policy, custom-type or remote-storage
portability. Do not run this machine-specific probe as a silently skipped Docker
CI step. ACL/xattr is integrated separately in Docker PR verification.

## Default ACL inheritance after restore

Build the required Linux syscall helper (select the runtime architecture):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags owlmail_acl_creation_fixture \
  -o /tmp/owlmail-acl-creation ./tests/container/acl_creation
python3 tests/container/default_acl_recovery.py --image owlmail:container-check \
  --helper-image owlmail:metadata-recovery --creation-binary /tmp/owlmail-acl-creation \
  --evidence-directory /tmp/owlmail-default-acl-evidence
```

This verifies existing objects are not changed retroactively, new child/grandchild
inheritance despite umask077, true creation mode0600/0700 limits, actual named-user
and named-group I/O, and inheritance of newly created objects after restore.
There is no post-creation chmod substitute. Ordinary tar must lose default ACL
and fail the new-object test. Docker PR verification includes this drill.

## Strict NFSv4 and AVC probes (existing Podman machine)

```sh
python3 tests/container/nfs4_acl_probe.py --machine podman-machine-default \
  --output /tmp/owlmail-nfs4-capabilities.json
python3 tests/container/selinux_avc.py --connection podman-machine-default \
  --machine podman-machine-default --fixture-image owlmail:container-fixture \
  --denial-kind mcs --output /tmp/owlmail-avc-mcs.json
```

NFSv4 `--target` selects only an existing mounted directory for read-only
inspection; no server, exports, mounts or ACE changes are created. Local POSIX
ACL is not a substitute. This is a capability probe, not an untested full NFSv4
restore implementation; acceptance remains false until separately exercised.

AVC `--denial-kind type` separately tests an owned user_home_t object; it never
stands in for MCS audit acceptance. Records must correlate a newly generated
actual denial by host PID, unique object/inode, contexts and time. No rules,
dontaudit, enforcement or service configuration are changed. In the current VM
both genuine MCS/type denials have no correlated log through existing channels;
NFSv4 has no mounted target/tools. These return unsupported/exit2, not passing
skips; actual probe errors or denial failures are failed. Do not add these as
silently skipped Docker CI checks. See the Chinese recovery guide for evidence
and the still-open acceptance requirements.
