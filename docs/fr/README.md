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

- **[Référence API](../en/API-Reference.md)** (English, [中文](../zh-CN/API-Reference.md))
  - Routes, authentification, formats de réponse, événements WebSocket et
    différences documentées avec MailDev.
- **[Exploitation et dépannage](../en/Operations.md)** (English, [中文](../zh-CN/Operations.md))
  - Déploiement, persistance, sécurité, TLS, capacité et diagnostic.
- **[Transmission Webhook](../en/Webhook-Forwarding.md)** (English, [中文](../zh-CN/Webhook-Forwarding.md))
  - Filtres, charges utiles personnalisées, signatures HMAC, tentatives et
    intégration avec `soulteary/webhook`.
- **[Exemples Webhook exécutables](../../examples/webhooks/README.md)** (English, [中文](../../examples/webhooks/README.zh-CN.md))

### Versions

- **[Notes de version 0.6.0](../en/Release-0.6.0.md)** (English, [中文](../zh-CN/Release-0.6.0.md))
- **[Processus de publication](../en/Releasing.md)** (English, [中文](../zh-CN/Releasing.md))

### Comparaison et migration

- **[OwlMail × MailDev – Comparaison et guide de migration](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)**
  - Cette traduction est incomplète. Consultez la version complète en
    [anglais](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
    ou en [chinois](../zh-CN/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md).

### Documentation interne historique

- **[Journal de refactorisation de l'API](./internal/API_Refactoring_Record.md)**
  - Notes d'implémentation historiques. La référence API définit le contrat actuel.

## 🔄 Contribuer

Les nouveaux documents sont d'abord créés dans `docs/en/`; les traductions
reprennent le même nom de fichier dans leur répertoire de langue. Mettez aussi à
jour [l'index principal](../README.md). Consultez le
[guide de contribution](../../.github/CONTRIBUTING.fr.md).

Pour plus d'informations, consultez le [README principal](../../README.fr.md).
