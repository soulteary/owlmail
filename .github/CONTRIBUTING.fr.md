# Contribuer à OwlMail

Cette page est un résumé en français. Le
[guide anglais](./CONTRIBUTING.md) est la référence complète et normative ; une
[version chinoise complète](./CONTRIBUTING.zh-CN.md) est également disponible.

## Parcours

1. Rechercher dans les [Issues](https://github.com/soulteary/owlmail/issues) et
   la [documentation](../docs/README.md).
2. Créer une branche ciblée, avec tests et documentation à jour.
3. Exécuter les contrôles avant d'ouvrir la pull request :

```bash
go test -race ./...
go vet ./...
bun build ./web/*.js --target=browser --outdir=./.bun-check
bun test ./tests/web ./tests/docs
```

Ne publiez jamais de mots de passe, identifiants SMTP, clés S3, secrets
Webhook ou contenu d'e-mail sensible dans les Issues, journaux ou données de
test. Les contributions suivent le [code de conduite](./CODE_OF_CONDUCT.fr.md)
et la [licence MIT](../LICENSE).
