# Browser and container regression checks

CI builds a live OwlMail server and runs Chromium tests for sandboxed long
messages, scrolling and widths, persisted/system themes, focus-safe shortcuts,
blocked storage, and the `/preview` base path. Failures retain traces and
screenshots for seven days. To run locally, build `owlmail`, then run
`bun install --frozen-lockfile`, `bun x playwright install chromium`, and
`bun run test` from `tests/browser`. `CHROME_BIN` can select an installed browser.

The image verification job also runs `tests/container/healthcheck.py --image IMAGE`
against environment and CLI overrides, config-file HTTPS with a self-signed
certificate, IPv6, and the non-server stdio mode. It checks Docker's actual
health state as well as the standalone probe. Containers are removed afterwards.

The local Docker-compatible runtime may build OCI images without HEALTHCHECK
metadata. For local testing only, pass
`--health-command '/app/owlmail healthcheck --container'` to the scenario script.
CI never supplies this override: a packaged image missing its healthcheck fails.
The IPv6 scenario also guards the web listener's host/port formatting.
