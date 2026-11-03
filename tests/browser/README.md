# Browser regressions

Install the locked runner with `bun install --frozen-lockfile` in this directory.
Use a Playwright Chromium installation, or set `CHROME_BIN` to an isolated test
browser executable. The suites never use an existing browser profile.

## Standalone

Build the normal binary from the repository root, then run the original suite:

```sh
go build -o owlmail ./cmd/owlmail
cd tests/browser
bun x playwright test --config playwright.config.js
```

`OWLMAIL_BINARY` may point to an absolute alternative binary path. The standalone
suite keeps its own SMTP/Web ports and uses a fresh temporary mail directory.

## Multiuser component suite

The public CLI multiuser release gate remains closed. The fixture below is
excluded from normal builds and is available only with an explicit test build tag:

```sh
go build -tags owlmail_browser_fixture -o owlmail-browser-fixture ./tests/browser/fixture
cd tests/browser
OWLMAIL_BROWSER_FIXTURE="$(pwd)/../../owlmail-browser-fixture" \
  bun x playwright test --config multiuser.playwright.config.js --grep-invert 'real background visibility'
```

The fixture uses real TLS SMTP and the same isolated storage, control-store and
HTTP components as the application. Its loopback proxy issues **synthetic**
Alice/Bob identity and verified-email assertions. Passing this suite does not
prove that a deployed Stargate supplies verified-email evidence.

The fixture binds only `127.0.0.1:21180`. Its test-only endpoints can send synthetic
mail and expose request counts; do not deploy it as an application server.
Optional fixture flags `-state-directory /absolute/test/path` and
`-listen 127.0.0.1:21180` support isolated container/restart tests. Without an
explicit state directory it creates temporary state. A nonempty mail directory
cannot recreate a missing control database.

The suite checks mail-before-login, unverified versus verified claims, account
switching, per-person lists/details/attachments/ZIP/batch mutations, disabled
legacy/MCP/WebSocket routes, real manual/timed polling, selection preservation,
maximum in-flight refresh concurrency and clearing/stopping on HTTP 401.

## Real background pause and resume

The background test requires a headed browser. It starts a temporary native
Chromium profile and connects to its existing default context using Playwright
1.62.1 `connectOverCDP({noDefaults:true})`. Ordinary managed pages enable focus
emulation, which can report every tab as visible despite a real tab switch.
The test never rewrites visibility properties or freezes the page.

On Linux, from this directory (Node, Xvfb and the locked browser dependencies must
be installed):

```sh
OWLMAIL_BROWSER_FIXTURE="$(pwd)/../../owlmail-browser-fixture" \
  ../../tests/container/run-browser-linux.sh bun x playwright test \
  --config multiuser.playwright.config.js --grep 'real background visibility'
```

The runner enables `OWLMAIL_REQUIRE_REAL_BACKGROUND=1` and requires actual
`hidden → visible`; failure to hide fails the test, never skips. It starts Xvfb
with a real X11 readiness check, stage logs and bounded cleanup of its owned descendant processes.
For an existing supported desktop display (including macOS), run the same test
command directly with `OWLMAIL_REQUIRE_REAL_BACKGROUND=1` and optionally
`CHROME_BIN`; no Xvfb runner is needed.

While hidden, real TLS SMTP delivers another message. Over 16 seconds the page
heartbeat must continue, preview request count must remain unchanged and the
list must retain its old state. On return, the new message must appear within
five seconds with the selected detail preserved and exactly one maximum
in-flight refresh. No application timer or visibility value is mocked.

Without strict mode an environment that actually keeps the page visible may
explicitly skip; that skip is not acceptance evidence. Linux CI always runs the
background case separately in strict mode. The ordinary headless multiuser step
excludes it. Neither step changes the production multiuser release gate.

Successful business screenshots are written to `multiuser-evidence/` and attached
to the report. Standard results/report use `multiuser-test-results/` and
`multiuser-playwright-report/`. Strict background results/report use separate
`multiuser-background-results/` and `multiuser-background-report/` directories,
including `real_background_evidence.json` and `background_resume_selection.png`.
Failed screenshots/traces are diagnostics, not successful walkthrough evidence.

Protected multiuser responses also require the server identity marker. Unexpected
login redirects and successful login HTML clear the current private view and stop
polling instead of being consumed as mailbox data. On foreground resume the UI
checks settings through the same authenticated response path before refreshing;
a persisted `pageshow` clears the view and reloads the document. The regression
suite uses real browser requests with intercepted proxy-style responses; its
persisted-page test dispatches the lifecycle event and does not certify that a
particular browser actually admitted the document into its back-forward cache.

A proxy `403` without the protected server identity marker also expires the
multiuser view. A same-identity authenticated `403` remains an operation error.
Successful JSON API responses must have the `application/json` media type even
when a login page incorrectly retains an identity header. Raw mail source keeps
its explicit text format; standalone response and error behavior is preserved.

Multiuser scheduled polling adds an independent uniform integer delay of 0–1500 ms
on top of the existing 15-second interval (and its existing failure backoff).
Foreground synchronization remains immediate after session validation; manual
refresh and standalone behavior do not use jitter. No extra configuration is
required. The maximum-in-flight and actual background-pause tests apply unchanged.

Expired sessions show a localized notice and turn the existing Refresh button into
“Sign in again”. This explicit action reloads the same protected page so Stargate
can authenticate it; background polling never retries login automatically. The
notice and action follow all seven UI languages, including live language changes.
