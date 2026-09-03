# Contribuire a OwlMail

Questa pagina è un riepilogo in italiano. La guida completa e normativa è la
[versione inglese](./CONTRIBUTING.md); è disponibile anche una
[versione cinese completa](./CONTRIBUTING.zh-CN.md).

## Procedura

1. Cercare negli [Issue](https://github.com/soulteary/owlmail/issues) e nella
   [documentazione](../docs/README.md).
2. Creare un branch mirato con test e documentazione aggiornati.
3. Prima della pull request eseguire:

```bash
go test -race ./...
go vet ./...
bun build ./web/*.js --target=browser --outdir=./.bun-check
bun test ./tests/web ./tests/docs
```

Non pubblicare password, credenziali SMTP, chiavi S3, segreti Webhook o
contenuti email sensibili in Issue, log o dati di test. I contributi seguono il
[codice di condotta](./CODE_OF_CONDUCT.it.md) e la
[licenza MIT](../LICENSE).
