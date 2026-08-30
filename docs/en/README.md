# OwlMail Documentation

Welcome to the OwlMail documentation directory. This directory contains technical documentation, migration guides, and API reference materials.

## 📸 Preview

![OwlMail Preview](../../.github/assets/preview.png)

## 🎥 Demo Video

![Demo Video](../../.github/assets/realtime.gif)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](../README.md#english) | [简体中文](../README.md#简体中文) | [Deutsch](../README.md#deutsch) | [Français](../README.md#français) | [Italiano](../README.md#italiano) | [日本語](../README.md#日本語) | [한국어](../README.md#한국어)

---

## 📚 Documentation Structure

### Main Documents

- **[OwlMail 0.5.0 Release Notes](./Release-0.5.0.md)**
  - Highlights, upgrade-sensitive behavior, installation commands, asset names,
    known limitations, and links to the detailed references.
  - **Other languages**: [简体中文](../zh-CN/Release-0.5.0.md)

- **[API Reference](./API-Reference.md)**
  - Complete route inventory, request/response conventions, authentication,
    native WebSocket events, curl examples, and the current MailDev migration boundary.
  - **Other languages**: [简体中文](../zh-CN/API-Reference.md)

- **[Operations and Troubleshooting](./Operations.md)**
  - Local and Docker profiles, persistence, security defaults, TLS, readiness,
    webhook capacity, backup/upgrade, shutdown boundaries, and failure diagnosis.
  - **Other languages**: [简体中文](../zh-CN/Operations.md)

- **[Webhook Forwarding](./Webhook-Forwarding.md)**
  - Configure filtering, custom payload templates, HMAC signatures, retries, and integration with `soulteary/webhook`.
  - **Runnable examples**: [minimal, filtered, custom, multi-target, plain text, and Compose](../../examples/webhooks/README.md)
  - **Other languages**: [简体中文](../zh-CN/Webhook-Forwarding.md)

- **[OwlMail × MailDev - Full Feature & API Comparison and Migration White Paper](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)**
  - A source-checked comparison of capability differences, API incompatibilities,
    and the migration checklist.
  - **Other languages**: [简体中文](../zh-CN/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [Deutsch](../de/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [Français](../fr/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [Italiano](../it/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [日本語](../ja/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [한국어](../ko/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

### Historical Internal Documentation

- **[API Refactoring Record](./internal/API_Refactoring_Record.md)**
  - Historical implementation notes for the move from unversioned routes to `/api/v1/`.
    Use the API Reference above as the current contract.
  - **Other languages**: [简体中文](../zh-CN/internal/API_Refactoring_Record.md) | [Deutsch](../de/internal/API_Refactoring_Record.md) | [Français](../fr/internal/API_Refactoring_Record.md) | [Italiano](../it/internal/API_Refactoring_Record.md) | [日本語](../ja/internal/API_Refactoring_Record.md) | [한국어](../ko/internal/API_Refactoring_Record.md)

### Maintainer Documentation

- **[Release Process](./Releasing.md)**
  - Tagging, binary/checksum and container verification, smoke tests, and the
    pre/post-release checklist.
  - **Other languages**: [简体中文](../zh-CN/Releasing.md)

## 📖 How to Read Documentation

Documents are organized by language in separate directories. Each language directory contains:
- `README.md` - Documentation index for that language
- Main documents (e.g., Migration White Paper)
- `internal/` subdirectory - Historical implementation records

To switch languages, use the language selector at the top of this page or visit the [main documentation index](../README.md).

## 🔄 Contributing

When adding new documentation:

1. Create the English version first in the `en/` directory.
2. Add translations in the corresponding language directories.
3. Update all language README files to include links to new documents.
4. Follow the existing directory structure (documents in language directories, no language suffix in filenames).

## 📝 Document Categories

- **Migration Guides**: Help users migrate from MailDev to OwlMail
- **Release Notes**: Summarize versioned changes, upgrade behavior, and known limitations
- **Operations**: Cover deployment, troubleshooting, and the maintainer release process
- **API Documentation**: Current technical contract and historical refactoring records
- **Internal Documentation**: Development notes and internal processes

---

For more information about OwlMail, please visit the [main README](../../README.md).
