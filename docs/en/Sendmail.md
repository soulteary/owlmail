# Sendmail-compatible CLI

`owlmail sendmail` gives PHP, cron jobs, and legacy applications a
sendmail-style process interface while delivering through OwlMail's normal
SMTP listener. It does not write to the mailbox directly. SMTP AUTH, TLS
policy, the message-size limit, recipient limit, and DATA concurrency limit
are therefore enforced by the server exactly as they are for any other SMTP
client.

## Basic use

Pass recipients explicitly:

```bash
printf 'From: app@example.test\nSubject: Job finished\n\nDone.\n' |
  owlmail sendmail operator@example.test
```

Or extract envelope recipients from `To`, `Cc`, and `Bcc`:

```bash
owlmail sendmail -t -i < message.eml
```

Explicit recipients and recipients found with `-t` are combined. Every `Bcc`
field, including folded continuation lines, is removed before DATA is sent.
`-i` and `-oi` are accepted for compatibility; the SMTP client always performs
CRLF normalization and dot-stuffing. Use `-f '<>'` for an empty envelope sender.
RFC 2047 and UTF-8 headers remain intact, and SMTPUTF8 is negotiated when raw
UTF-8 headers or internationalized envelope addresses require it.

## PHP `sendmail_path`

Install the OwlMail binary in the PHP container or host, then add this to
`php.ini`:

```ini
sendmail_path = "/usr/local/bin/owlmail sendmail -t -i"
```

Point the inherited process environment at the OwlMail service. For a Compose
service named `owlmail`:

```yaml
environment:
  OWLMAIL_SENDMAIL_HOST: owlmail
  OWLMAIL_SENDMAIL_PORT: "1025"
```

PHP's `mail()` function can now submit through the same SMTP boundary:

```php
<?php
mail('developer@example.test', 'OwlMail test', 'It works!', [
    'From' => 'php@example.test',
]);
```

## SMTP configuration

Command-line options override environment variables.

| Option | Environment variable | Default | Purpose |
|---|---|---:|---|
| `--host` | `OWLMAIL_SENDMAIL_HOST` | `127.0.0.1` | OwlMail SMTP host |
| `--port` | `OWLMAIL_SENDMAIL_PORT` | `1025` | OwlMail SMTP port |
| `--starttls` | `OWLMAIL_SENDMAIL_STARTTLS` | `false` | Require an advertised STARTTLS upgrade |
| `--smtps` | `OWLMAIL_SENDMAIL_SMTPS` | `false` | Use implicit TLS from connection start |
| `--username` | `OWLMAIL_SENDMAIL_USERNAME` | - | PLAIN AUTH username |
| `--password` | `OWLMAIL_SENDMAIL_PASSWORD` | - | PLAIN AUTH password |

`--smtp-host`, `--smtp-ip`, and `--smtp-port` are accepted aliases. STARTTLS
and SMTPS are mutually exclusive. Credentials must be provided together;
prefer `OWLMAIL_SENDMAIL_PASSWORD` over a command-line password so the secret
does not appear in a process listing.

For a TLS-required OwlMail listener:

```bash
export OWLMAIL_SENDMAIL_HOST=owlmail.example.test
export OWLMAIL_SENDMAIL_PORT=1025
export OWLMAIL_SENDMAIL_STARTTLS=true
export OWLMAIL_SENDMAIL_USERNAME=app
export OWLMAIL_SENDMAIL_PASSWORD='replace-me'
owlmail sendmail -t < message.eml
```

SMTPS normally uses port 465 and `OWLMAIL_SENDMAIL_SMTPS=true`. Certificates
are verified against the operating system trust store and the configured host
name. Install a private CA in that trust store when OwlMail uses a private
certificate; the command intentionally has no insecure-skip-verification mode.

## Compatible arguments

- `-t`: add addresses from all `To`, `Cc`, and `Bcc` fields.
- `-f ADDRESS` or `-fADDRESS`: select the envelope sender. `-f '<>'` selects
  the null reverse path.
- `-i` and `-oi`: accepted no-op compatibility forms.
- `--`: stop option parsing so a later value is always treated as a recipient.

Unknown sendmail flags fail instead of being silently ignored.

## Exit status

The stable values follow `sysexits(3)` conventions:

| Status | Meaning |
|---:|---|
| `0` | Server accepted DATA |
| `64` | Invalid option or client configuration |
| `65` | Invalid RFC 5322 header data or no recipients found with `-t` |
| `69` | Permanent SMTP failure, including a 5xx reply |
| `74` | Local stdin/read failure |
| `75` | Temporary SMTP 4xx reply or network failure; retry is appropriate |

Once the server returns `250` after DATA, a later QUIT failure still returns
success so callers do not resend an already accepted message. Diagnostics
never include the password, message body, or full authentication exchange.

