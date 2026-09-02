# CLI compatibile con sendmail

`owlmail sendmail` legge un messaggio RFC 5322 da stdin e lo consegna al normale
listener SMTP di OwlMail. AUTH, TLS e i limiti per dimensione, destinatari e
concorrenza DATA rimangono quindi attivi.

```bash
owlmail sendmail -t -i < message.eml
```

`-t` estrae `To`, `Cc`, `Bcc` e i campi `Resent-*`, aggiungendoli ai destinatari espliciti. Tutti i
campi Bcc/Resent-Bcc e le continuazioni vengono rimossi prima di DATA. `-f ADDRESS` o
`-fADDRESS` imposta il mittente dell'envelope; `-f '<>'` usa il reverse path
vuoto. Sono supportati `-i`, `-oi` e `--`. CRLF, dot-stuffing e header RFC
2047/UTF-8 restano corretti, con SMTPUTF8 quando necessario.

## PHP

```ini
sendmail_path = "/usr/local/bin/owlmail sendmail -t -i"
```

```yaml
environment:
  OWLMAIL_SENDMAIL_HOST: owlmail
  OWLMAIL_SENDMAIL_PORT: "1025"
```

## Configurazione

| Opzione | Variabile d'ambiente | Predefinito |
|---|---|---:|
| `--host` | `OWLMAIL_SENDMAIL_HOST` | `127.0.0.1` |
| `--port` | `OWLMAIL_SENDMAIL_PORT` | `1025` |
| `--starttls` | `OWLMAIL_SENDMAIL_STARTTLS` | `false` |
| `--smtps` | `OWLMAIL_SENDMAIL_SMTPS` | `false` |
| `--username` | `OWLMAIL_SENDMAIL_USERNAME` | - |
| `--password` | `OWLMAIL_SENDMAIL_PASSWORD` | - |
| `--timeout` | `OWLMAIL_SENDMAIL_TIMEOUT` | `30s` |

STARTTLS e SMTPS sono mutuamente esclusivi. Username e password vanno forniti
insieme; per il segreto è preferibile la variabile d'ambiente. I certificati TLS
sono verificati con il trust store di sistema e il nome host; non esiste
un'opzione insicura per saltare la verifica.

Codici stabili: `0` successo, `64` argomenti, `65` dati messaggio, `69` errore
SMTP permanente, `74` I/O locale e `75` errore SMTP temporaneo o di rete.
Password, corpo e scambio AUTH completo non vengono registrati. Riferimento
completo in [inglese](../en/Sendmail.md) o [cinese](../zh-CN/Sendmail.md).
