# OwlMail Dokumentation

Willkommen im OwlMail-Dokumentationsverzeichnis. Die Dokumente sind nach Sprache
in eigenen Verzeichnissen organisiert.

## 📸 Vorschau

![OwlMail Vorschau](../../.github/assets/preview.png)

## 🎥 Demo-Video

![Demo-Video](../../.github/assets/realtime.gif)

## 🌍 Sprachen

- [English](../en/README.md) | [简体中文](../zh-CN/README.md) | [Deutsch](./README.md) | [Français](../fr/README.md) | [Italiano](../it/README.md) | [日本語](../ja/README.md) | [한국어](../ko/README.md)

## 📚 Dokumente

### Betriebsreferenzen

- **Testen und Agenten** (English, [中文](../zh-CN/Integration-Testing.md)):
  [Integration](../en/Integration-Testing.md), [CI](../en/CI-Quickstart.md),
  [AI-Agenten](../en/AI-Agent-Testing.md), [MCP](../en/MCP-Reference.md),
  [Rezepte](../en/Testing-Recipes.md), [Architektur](../en/Architecture.md) und
  [Sicherheitsmodell](../en/Security-Model.md).
- **[Ausführbare JavaScript-, Go-, Python- und Compose-Beispiele](../../examples/testing/README.md)**
- **[API-Referenz](../en/API-Reference.md)** (English, [中文](../zh-CN/API-Reference.md))
  - Endpunkte, Authentifizierung, Antwortformate, WebSocket-Ereignisse und
    dokumentierte MailDev-Unterschiede.
  - Maschinenlesbar: [OpenAPI 3.1 JSON](../../openapi/openapi.json) | [YAML](../../openapi/openapi.yaml)
- **[Betrieb und Fehlerbehebung](../en/Operations.md)** (English, [中文](../zh-CN/Operations.md))
  - Bereitstellungsprofile, Persistenz, Sicherheit, TLS, Kapazität und Diagnose.
- **[Sendmail-kompatible CLI](./Sendmail.md)**
  - PHP `sendmail_path`, Cron, SMTP/TLS/AUTH und stabile Statuswerte.
- **[Webhook-Weiterleitung](../en/Webhook-Forwarding.md)** (English, [中文](../zh-CN/Webhook-Forwarding.md))
  - Filter, benutzerdefinierte Payloads, HMAC-Signaturen, Wiederholungen und
    die Integration mit `soulteary/webhook`.
- **[Ausführbare Webhook-Beispiele](../../examples/webhooks/README.md)** (English, [中文](../../examples/webhooks/README.zh-CN.md))

### Veröffentlichungen

- **[Versionshinweise 0.9.0](../en/Release-0.9.0.md)** (English, [中文](../zh-CN/Release-0.9.0.md))
- **[Versionshinweise 0.7.0](../en/Release-0.7.0.md)** (English, [中文](../zh-CN/Release-0.7.0.md))
- **[Veröffentlichungsprozess](../en/Releasing.md)** (English, [中文](../zh-CN/Releasing.md))

### Vergleich und Migration

- **[OwlMail × MailDev – Vergleich und Migrations-Whitepaper](./Comparison-and-Migration.md)**
  - Diese Übersetzung ist noch unvollständig; für den aktuellen vollständigen
    Stand siehe [English](../en/Comparison-and-Migration.md)
    oder [中文](../zh-CN/Comparison-and-Migration.md).

### Historische interne Dokumentation

- **[API-Refactoring-Aufzeichnung](./internal/API_Refactoring_Record.md)**
  - Historische Implementierungsnotizen. Die aktuelle Schnittstelle wird durch
    die API-Referenz definiert.

## 🔄 Beitragen

Neue Dokumente werden zuerst unter `docs/en/` erstellt; Übersetzungen verwenden
das gleiche Dateinamenschema im jeweiligen Sprachverzeichnis. Aktualisieren Sie
beim Hinzufügen einer Übersetzung auch [den Hauptindex](../README.md). Weitere
Hinweise stehen im [Beitragsleitfaden](../../.github/CONTRIBUTING.de.md).

Weitere Informationen finden Sie im [Haupt-README](../../README.de.md).
