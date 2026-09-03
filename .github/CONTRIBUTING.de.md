# Beiträge zu OwlMail

Dies ist eine kurze deutsche Orientierung. Die vollständige und verbindliche
Anleitung ist der [englische Beitragsleitfaden](./CONTRIBUTING.md); eine
vollständige chinesische Fassung ist ebenfalls
[verfügbar](./CONTRIBUTING.zh-CN.md).

## Ablauf

1. Vorhandene [Issues](https://github.com/soulteary/owlmail/issues) und die
   [Dokumentation](../docs/README.md) durchsuchen.
2. Einen kleinen Branch mit Tests und aktualisierter Dokumentation erstellen.
3. Vor dem Pull Request folgende Prüfungen ausführen:

```bash
go test -race ./...
go vet ./...
bun build ./web/*.js --target=browser --outdir=./.bun-check
bun test ./tests/web ./tests/docs
```

Keine Passwörter, SMTP-Zugangsdaten, S3-Schlüssel, Webhook-Secrets oder
vertrauliche E-Mail-Inhalte in Issues, Logs oder Testdaten veröffentlichen.
Beiträge unterliegen dem [Verhaltenskodex](./CODE_OF_CONDUCT.de.md) und der
[MIT-Lizenz](../LICENSE).
