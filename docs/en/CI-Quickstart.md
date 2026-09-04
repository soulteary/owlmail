# CI quickstart

Run OwlMail as a disposable sidecar, wait for readiness, execute the test
suite, and preserve logs when the job fails. The example uses the 0.9.0 image
tag for readability; record and pin its manifest digest when exact
reproducibility is required.

## GitHub Actions

```yaml
name: integration
on: [push, pull_request]

jobs:
  email:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6
        with:
          go-version-file: go.mod
      - name: Start OwlMail 0.9.0
        run: |
          docker run -d --name owlmail-ci \
            -p 127.0.0.1:1025:1025 \
            -p 127.0.0.1:1080:1080 \
            ghcr.io/soulteary/owlmail:0.9.0
          for attempt in $(seq 1 30); do
            curl --fail --silent --connect-timeout 2 --max-time 3 \
              http://127.0.0.1:1080/readyz && exit 0
            sleep 1
          done
          docker logs owlmail-ci
          exit 1
      - name: Run integration tests
        env:
          TEST_SMTP_HOST: 127.0.0.1
          TEST_SMTP_PORT: "1025"
          TEST_MAIL_API: http://127.0.0.1:1080
        run: OWLMAIL_RUN_INTEGRATION_TEST=1 go test ./examples/testing/go -v
      - name: Preserve OwlMail logs
        if: failure()
        run: docker logs owlmail-ci > owlmail.log 2>&1
      - uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6
        if: failure()
        with:
          name: owlmail-log
          path: owlmail.log
      - name: Stop OwlMail
        if: always()
        run: docker rm --force owlmail-ci || true
```

Pin third-party actions to full commit SHAs in security-sensitive repositories.
If the job runs inside another container, `127.0.0.1` is not the Docker host;
place both containers on one network and use the OwlMail service name.

## CI contract

| Boundary | Recommended check |
|---|---|
| Process alive | `GET /healthz` |
| Dependencies ready | `GET /readyz` |
| SMTP ingress | One unique test message accepted on port 1025 |
| Message assertion | Native `/api/v1/emails` and `/api/v1/emails/:id` |
| Failure evidence | OwlMail logs plus application mailer logs |

Use readiness rather than a fixed startup sleep. Apply a timeout to every poll,
and let the job fail with useful logs instead of waiting indefinitely.

## Parallel jobs and cleanup

- Prefer one disposable OwlMail per CI job.
- Generate recipients from the job and worker IDs.
- Do not share a writable mail directory between OwlMail processes.
- Do not expose SMTP or the Web API on a public runner interface.
- Delete the container after each job; use a volume only when the test is
  specifically exercising restart recovery.

The ready-to-run local Compose profile and three language examples are in
[examples/testing](../../examples/testing/README.md). See
[Integration testing](./Integration-Testing.md) for the assertion lifecycle and
[Operations](./Operations.md) for persistent, authenticated, TLS, and S3
profiles.
