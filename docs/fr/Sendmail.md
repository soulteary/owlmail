# CLI compatible sendmail

`owlmail sendmail` lit un message RFC 5322 sur stdin et l'envoie au listener
SMTP normal d'OwlMail. AUTH, TLS, les limites de taille, de destinataires et de
concurrence DATA restent donc appliqués.

```bash
owlmail sendmail -t -i < message.eml
```

`-t` extrait `To`, `Cc` et `Bcc` et les ajoute aux destinataires explicites.
Tous les champs Bcc et leurs continuations sont retirés avant DATA. `-f ADDRESS`
ou `-fADDRESS` choisit l'expéditeur d'enveloppe; `-f '<>'` utilise un reverse
path vide. `-i`, `-oi` et `--` sont acceptés. CRLF, dot-stuffing et les en-têtes
RFC 2047/UTF-8 sont préservés, avec SMTPUTF8 lorsque nécessaire.

## PHP

```ini
sendmail_path = "/usr/local/bin/owlmail sendmail -t -i"
```

```yaml
environment:
  OWLMAIL_SENDMAIL_HOST: owlmail
  OWLMAIL_SENDMAIL_PORT: "1025"
```

## Configuration

| Option | Variable d'environnement | Défaut |
|---|---|---:|
| `--host` | `OWLMAIL_SENDMAIL_HOST` | `127.0.0.1` |
| `--port` | `OWLMAIL_SENDMAIL_PORT` | `1025` |
| `--starttls` | `OWLMAIL_SENDMAIL_STARTTLS` | `false` |
| `--smtps` | `OWLMAIL_SENDMAIL_SMTPS` | `false` |
| `--username` | `OWLMAIL_SENDMAIL_USERNAME` | - |
| `--password` | `OWLMAIL_SENDMAIL_PASSWORD` | - |

STARTTLS et SMTPS sont exclusifs. L'identifiant et le mot de passe doivent être
fournis ensemble; préférez la variable d'environnement pour le secret. Les
certificats TLS sont vérifiés avec le magasin système et le nom d'hôte; aucune
option non sûre ne désactive cette vérification.

Codes stables : `0` succès, `64` arguments, `65` données du message, `69` erreur
SMTP permanente, `74` E/S locale, `75` erreur SMTP temporaire ou réseau. Le mot
de passe, le corps et l'échange AUTH complet ne sont jamais journalisés. Voir la
référence complète en [anglais](../en/Sendmail.md) ou en
[chinois](../zh-CN/Sendmail.md).

