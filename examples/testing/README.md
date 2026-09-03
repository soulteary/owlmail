# Integration testing examples

These dependency-free examples send one uniquely addressed SMTP message, poll
the OwlMail native v1 API with a bounded deadline, assert its content, and
delete only that message.

## Start OwlMail

```bash
docker compose -f examples/testing/compose.yaml up -d --wait
```

The Compose profile pins OwlMail 0.8.0 and publishes both ports on loopback.
For exact CI reproducibility, replace the image tag with the release manifest
digest.

## Run an example

```bash
# Node.js 18 or newer
node examples/testing/javascript/email-test.mjs

# Python 3.10 or newer
python3 examples/testing/python/email_test.py

# Go version declared by the repository go.mod
OWLMAIL_RUN_INTEGRATION_TEST=1 go test ./examples/testing/go -v
```

The Go example requires the opt-in variable so a normal repository-wide
`go test ./...` does not depend on a running OwlMail instance.

Override the endpoints when OwlMail runs elsewhere:

```bash
TEST_SMTP_HOST=owlmail \
TEST_SMTP_PORT=1025 \
TEST_MAIL_API=http://owlmail:1080 \
node examples/testing/javascript/email-test.mjs
```

The examples intentionally avoid clearing the mailbox. Each creates a unique
recipient, filters by the complete address, and removes only the returned ID.
See [Integration testing](../../docs/en/Integration-Testing.md) and the
[testing recipes](../../docs/en/Testing-Recipes.md) for production test-suite
patterns.

Stop the profile after testing:

```bash
docker compose -f examples/testing/compose.yaml down
```
