# Sendmail-kompatible CLI

`owlmail sendmail` liest eine RFC-5322-Nachricht von stdin und übergibt sie über
den normalen OwlMail-SMTP-Listener. AUTH, TLS, Größen-, Empfänger- und
DATA-Parallelitätsgrenzen werden dadurch nicht umgangen.

```bash
owlmail sendmail -t -i < message.eml
```

`-t` liest `To`, `Cc` und `Bcc`; explizite Empfänger werden ergänzt. Vor DATA
werden alle Bcc-Felder einschließlich Fortsetzungszeilen entfernt. `-f ADDRESS`
und `-fADDRESS` setzen den Envelope-Absender, `-f '<>'` den leeren Reverse Path.
`-i`, `-oi` und `--` werden unterstützt. CRLF, Dot-Stuffing sowie RFC-2047- und
UTF-8-Header werden korrekt behandelt; bei Bedarf wird SMTPUTF8 verwendet.

## PHP

```ini
sendmail_path = "/usr/local/bin/owlmail sendmail -t -i"
```

```yaml
environment:
  OWLMAIL_SENDMAIL_HOST: owlmail
  OWLMAIL_SENDMAIL_PORT: "1025"
```

## Konfiguration

| Option | Umgebungsvariable | Standard |
|---|---|---:|
| `--host` | `OWLMAIL_SENDMAIL_HOST` | `127.0.0.1` |
| `--port` | `OWLMAIL_SENDMAIL_PORT` | `1025` |
| `--starttls` | `OWLMAIL_SENDMAIL_STARTTLS` | `false` |
| `--smtps` | `OWLMAIL_SENDMAIL_SMTPS` | `false` |
| `--username` | `OWLMAIL_SENDMAIL_USERNAME` | - |
| `--password` | `OWLMAIL_SENDMAIL_PASSWORD` | - |

STARTTLS und SMTPS schließen sich aus. Kennungen müssen gemeinsam gesetzt
werden; das Passwort sollte aus der Umgebung kommen. TLS-Zertifikate werden
gegen den System-Truststore und den Hostnamen geprüft. Es gibt keinen unsicheren
Schalter zum Überspringen der Prüfung.

Stabile Statuswerte: `0` Erfolg, `64` Argumentfehler, `65` Nachrichtendaten,
`69` permanenter SMTP-Fehler, `74` lokaler I/O-Fehler und `75` temporärer
SMTP-/Netzwerkfehler. Kennwort, Nachrichtentext und vollständiger AUTH-Dialog
werden nicht protokolliert. Die vollständige Referenz steht auf
[Englisch](../en/Sendmail.md) und [Chinesisch](../zh-CN/Sendmail.md).

