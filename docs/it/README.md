# Documentazione OwlMail

Benvenuto nella documentazione di OwlMail. I documenti sono organizzati per
lingua in directory separate.

## 📸 Anteprima

![Anteprima OwlMail](../../.github/assets/preview.png)

## 🎥 Video dimostrativo

![Video dimostrativo](../../.github/assets/realtime.gif)

## 🌍 Lingue

- [English](../en/README.md) | [简体中文](../zh-CN/README.md) | [Deutsch](../de/README.md) | [Français](../fr/README.md) | [Italiano](./README.md) | [日本語](../ja/README.md) | [한국어](../ko/README.md)

## 📚 Documenti

### Riferimenti operativi

- **Test e agenti** (English, [中文](../zh-CN/Integration-Testing.md)):
  [integrazione](../en/Integration-Testing.md), [CI](../en/CI-Quickstart.md),
  [agenti AI](../en/AI-Agent-Testing.md), [MCP](../en/MCP-Reference.md),
  [ricette](../en/Testing-Recipes.md), [architettura](../en/Architecture.md) e
  [modello di sicurezza](../en/Security-Model.md).
- **[Esempi JavaScript, Go, Python e Compose eseguibili](../../examples/testing/README.md)**
- **[Riferimento API](../en/API-Reference.md)** (English, [中文](../zh-CN/API-Reference.md))
  - Route, autenticazione, formati di risposta, eventi WebSocket e differenze
    documentate rispetto a MailDev.
  - Leggibile da strumenti: [OpenAPI 3.1 JSON](../../openapi/openapi.json) | [YAML](../../openapi/openapi.yaml)
- **[Operazioni e risoluzione problemi](../en/Operations.md)** (English, [中文](../zh-CN/Operations.md))
  - Distribuzione, persistenza, sicurezza, TLS, capacità e diagnostica.
- **[CLI compatibile con sendmail](./Sendmail.md)**
  - PHP `sendmail_path`, Cron, SMTP/TLS/AUTH e codici di uscita stabili.
- **[Inoltro Webhook](../en/Webhook-Forwarding.md)** (English, [中文](../zh-CN/Webhook-Forwarding.md))
  - Filtri, payload personalizzati, firme HMAC, nuovi tentativi e integrazione
    con `soulteary/webhook`.
- **[Esempi Webhook eseguibili](../../examples/webhooks/README.md)** (English, [中文](../../examples/webhooks/README.zh-CN.md))

### Release

- **[Note di rilascio 0.8.0](../en/Release-0.8.0.md)** (English, [中文](../zh-CN/Release-0.8.0.md))
- **[Note di rilascio 0.7.0](../en/Release-0.7.0.md)** (English, [中文](../zh-CN/Release-0.7.0.md))
- **[Processo di rilascio](../en/Releasing.md)** (English, [中文](../zh-CN/Releasing.md))

### Confronto e migrazione

- **[OwlMail × MailDev – Confronto e guida alla migrazione](./Comparison-and-Migration.md)**
  - Questa traduzione è incompleta. Consulta la versione completa in
    [inglese](../en/Comparison-and-Migration.md)
    o in [cinese](../zh-CN/Comparison-and-Migration.md).

### Documentazione interna storica

- **[Registro della refactorizzazione API](./internal/API_Refactoring_Record.md)**
  - Note storiche di implementazione. Il contratto corrente è definito dal
    riferimento API.

## 🔄 Contribuire

I nuovi documenti vengono creati prima in `docs/en/`; le traduzioni usano lo
stesso nome di file nella rispettiva directory. Aggiorna anche
[l'indice principale](../README.md). Consulta la
[guida ai contributi](../../.github/CONTRIBUTING.it.md).

Per maggiori informazioni, consulta il [README principale](../../README.it.md).
