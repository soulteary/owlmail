# Documentation OwlMail

Bienvenue dans la documentation OwlMail. Les documents sont organisés par
langue dans des répertoires distincts.

## 📸 Aperçu

![Aperçu OwlMail](../../.github/assets/preview.png)

## 🎥 Démonstration

![Démonstration](../../.github/assets/realtime.gif)

## 🌍 Langues

- [English](../en/README.md) | [简体中文](../zh-CN/README.md) | [Deutsch](../de/README.md) | [Français](./README.md) | [Italiano](../it/README.md) | [日本語](../ja/README.md) | [한국어](../ko/README.md)

## 📚 Documents

### Références opérationnelles

- **Tests et agents** (English, [中文](../zh-CN/Integration-Testing.md)) :
  [intégration](../en/Integration-Testing.md), [CI](../en/CI-Quickstart.md),
  [agents IA](../en/AI-Agent-Testing.md), [MCP](../en/MCP-Reference.md),
  [recettes](../en/Testing-Recipes.md), [architecture](../en/Architecture.md) et
  [modèle de sécurité](../en/Security-Model.md).
- **[Exemples JavaScript, Go, Python et Compose exécutables](../../examples/testing/README.md)**
- **[Référence API](../en/API-Reference.md)** (English, [中文](../zh-CN/API-Reference.md))
  - Routes, authentification, formats de réponse, événements WebSocket et
    différences documentées avec MailDev.
  - Lisible par machine : [OpenAPI 3.1 JSON](../../openapi/openapi.json) | [YAML](../../openapi/openapi.yaml)
- **[Exploitation et dépannage](../en/Operations.md)** (English, [中文](../zh-CN/Operations.md))
  - Déploiement, persistance, sécurité, TLS, capacité et diagnostic.
- **[CLI compatible sendmail](./Sendmail.md)**
  - PHP `sendmail_path`, Cron, SMTP/TLS/AUTH et codes de sortie stables.
- **[Transmission Webhook](../en/Webhook-Forwarding.md)** (English, [中文](../zh-CN/Webhook-Forwarding.md))
  - Filtres, charges utiles personnalisées, signatures HMAC, tentatives et
    intégration avec `soulteary/webhook`.
- **[Exemples Webhook exécutables](../../examples/webhooks/README.md)** (English, [中文](../../examples/webhooks/README.zh-CN.md))

### Versions

- **[Notes de version 0.9.0](../en/Release-0.9.0.md)** (English, [中文](../zh-CN/Release-0.9.0.md))
- **[Notes de version 0.7.0](../en/Release-0.7.0.md)** (English, [中文](../zh-CN/Release-0.7.0.md))
- **[Processus de publication](../en/Releasing.md)** (English, [中文](../zh-CN/Releasing.md))

### Comparaison et migration

- **[OwlMail × MailDev – Comparaison et guide de migration](./Comparison-and-Migration.md)**
  - Cette traduction est incomplète. Consultez la version complète en
    [anglais](../en/Comparison-and-Migration.md)
    ou en [chinois](../zh-CN/Comparison-and-Migration.md).

### Documentation interne historique

- **[Journal de refactorisation de l'API](./internal/API_Refactoring_Record.md)**
  - Notes d'implémentation historiques. La référence API définit le contrat actuel.

## 🔄 Contribuer

Les nouveaux documents sont d'abord créés dans `docs/en/`; les traductions
reprennent le même nom de fichier dans leur répertoire de langue. Mettez aussi à
jour [l'index principal](../README.md). Consultez le
[guide de contribution](../../.github/CONTRIBUTING.fr.md).

Pour plus d'informations, consultez le [README principal](../../README.fr.md).
