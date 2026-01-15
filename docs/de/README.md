# OwlMail Dokumentation

Willkommen im OwlMail-Dokumentationsverzeichnis. Dieses Verzeichnis enthält technische Dokumentation, Migrationsleitfäden und API-Referenzmaterialien.

## 📚 Dokumentationsstruktur

### Hauptdokumente

- **[OwlMail × MailDev - Vollständiger Funktions- und API-Vergleich sowie Migrations-Whitepaper](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)** (Englisch)
  - [中文版本](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.zh-CN.md)
  - Ein umfassender Vergleich zwischen OwlMail und MailDev, einschließlich API-Kompatibilität, Funktionsparität und Migrationsleitfaden.

### Interne Dokumentation

- **[API-Refactoring-Aufzeichnung](./internal/API_Refactoring_Record.md)** (Englisch)
  - [中文版本](./internal/API_Refactoring_Record.zh-CN.md)
  - Dokumentiert den API-Refactoring-Prozess von MailDev-kompatiblen Endpunkten zum neuen RESTful API-Design (`/api/v1/`).

## 🌍 Mehrsprachige Unterstützung

Alle Dokumente folgen der Namenskonvention: `filename.md` (Englisch, Standard) und `filename.LANG.md` für andere Sprachen.

### Unterstützte Sprachen

- **English** (`en`) - Standard, kein Sprachcode-Suffix
- **简体中文** (`zh-CN`) - Chinesisch (Vereinfacht)
- **Deutsch** (`de`) - Deutsch

### Sprachcode-Format

Die Sprachcodes folgen dem [ISO 639-1](https://en.wikipedia.org/wiki/ISO_639-1)-Standard:
- `zh-CN` - Chinesisch (Vereinfacht)
- `de` - Deutsch
- `fr` - Französisch (zukünftig)
- `it` - Italienisch (zukünftig)
- `ja` - Japanisch (zukünftig)
- `ko` - Koreanisch (zukünftig)

## 📖 So lesen Sie die Dokumentation

1. **Standardsprache**: Dokumente ohne Sprachcode-Suffix sind auf Englisch (Standard).
2. **Andere Sprachen**: Dokumente mit Sprachcode-Suffix (z. B. `.zh-CN.md`) sind Übersetzungen.
3. **Verzeichnisstruktur**: Dokumente sind nach Themen organisiert, interne Dokumentation befindet sich im Unterverzeichnis `internal/`.

## 🔄 Beitragen

Beim Hinzufügen neuer Dokumentation:

1. Erstellen Sie zuerst die englische Version (Standard, kein Sprachcode).
2. Fügen Sie Übersetzungen mit dem entsprechenden Sprachcode-Suffix hinzu.
3. Aktualisieren Sie dieses README, um Links zu neuen Dokumenten aufzunehmen.
4. Befolgen Sie die bestehenden Namenskonventionen.

## 📝 Dokumentkategorien

- **Migrationsleitfäden**: Helfen Benutzern bei der Migration von MailDev zu OwlMail
- **API-Dokumentation**: Technische API-Referenz und Refactoring-Aufzeichnungen
- **Interne Dokumentation**: Entwicklungsnotizen und interne Prozesse

---

Weitere Informationen zu OwlMail finden Sie im [Haupt-README](../README.de.md).
