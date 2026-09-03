# OwlMail

> 🦉 Ein selbst gehostetes, AI-natives E-Mail-Test-Gateway für Entwicklung, CI, Automatisierung und Coding Agents.

[![Go Version](https://img.shields.io/badge/Go-1.27.0+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Latest Release](https://img.shields.io/github/v/release/soulteary/owlmail)](https://github.com/soulteary/owlmail/releases)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Workflows](https://img.shields.io/badge/MailDev-Workflow%20Compatibility-blue.svg)](./docs/de/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
[![Go Report Card](.github/goreportcard.svg)](.github/goreportcard-report.md)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](README.md) | [简体中文](README.zh-CN.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Italiano](README.it.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

---

OwlMail fängt Anwendungs-E-Mails vor einem echten Postfach ab und macht sie zu
deterministischen, prüfbaren Testdaten. Entwickler nutzen die Weboberfläche,
Tests die versionierte REST-API und OpenAPI, Automatisierungen dauerhafte
Ereignisse und AI Agents eine begrenzte, schreibgeschützte MCP-Schnittstelle.

| Nutzer | Schnittstelle | Typischer Ablauf |
|---|---|---|
| Entwickler | Weboberfläche und Browser-Benachrichtigungen | HTML, Text, Header, Quelle, Links und Anhänge prüfen |
| Tests und CI | REST API, OpenAPI 3.1 und WebSocket | Registrierungs-, Reset- und Benachrichtigungs-E-Mails verifizieren |
| Automatisierung | Signierte Webhooks und optional Redis Streams | Committete E-Mails in wiederherstellbare Ereignisse umwandeln |
| AI Coding Agents | Schreibgeschütztes MCP über Streamable HTTP oder stdio | E-Mails suchen, abrufen und ereignisbasiert erwarten |
| SMTP-Betrieb | Manuelles und automatisches Relay | Ausgewählte Test-E-Mails mit klaren TLS-Regeln weiterleiten |

![](.github/assets/owlmail-banner.jpg)

## 📸 Vorschau

![OwlMail Vorschau](.github/assets/preview.png)

## 🎥 Demo-Video

![Demo-Video](.github/assets/realtime.gif)

## ✨ Warum OwlMail

- **Deterministische Erfassung** — EML, Metadaten und Anhänge werden vor der
  atomaren Sichtbarkeit vollständig bereitgestellt.
- **Für Integrationstests** — `/api/v1`, OpenAPI 3.1, natives WebSocket,
  Health/Readiness, Suche, Filterung und Export.
- **AI-nativ, nicht AI-abhängig** — Das standardmäßig deaktivierte MCP bietet
  sieben geschlossene Nur-Lese-Tools, begrenzte Ressourcen, Prompts und
  ereignisbasiertes `wait_for_email`; OwlMail benötigt selbst kein LLM.
- **Dauerhafte Automatisierung** — Lokale Webhook-Outbox, optional Redis Streams,
  HMAC, stabile Zustell-IDs, begrenzte Wiederholungen und geordnetes Beenden.
- **Expliziter Betrieb** — SMTP-Kapazitätsgrenzen, Persistenz, optionale
  S3-Anhänge, SQLite-Index, Prometheus-Metriken und JSON-Protokolle.
- **Kontrollierte Zustellung** — Persistente asynchrone Relay-Aufträge verwenden
  unveränderliche Konfiguration, Streaming-DATA und explizite TLS-Modi.
- **Migrationspfade** — Optionale MailDev- und MailCatcher-REST-Fassaden, ohne
  Socket.IO oder vollständige Gleichwertigkeit zu versprechen.

- **Lokale Werkzeuge** — Webhook-Regeln im eingebetteten `/webhooks`-Editor\n  erstellen und Altprogramme über den [Sendmail-Leitfaden](./docs/de/Sendmail.md)\n  anbinden. Quell- und Browsertests verwenden Bun; die Binärdatei benötigt keine Laufzeit.\n\n## 🆕 OwlMail 0.8.0

`v0.8.0` ist die aktuelle stabile Version. Sie ergänzt persistente Relay-Aufträge,
geschichtete YAML/JSON-Konfiguration, optionales SQLite-Indexing, Prometheus-
Metriken, strukturierte Logs, MailCatcher REST, MCP stdio und strengere Grenzen.

Alle Installationsbeispiele verwenden `ghcr.io/soulteary/owlmail:0.8.0`.
Für reproduzierbare CI sollte die vollständige Version oder
`ghcr.io/soulteary/owlmail@sha256:<digest>` verwendet werden.
Details stehen in den [Versionshinweisen 0.8.0](./docs/en/Release-0.8.0.md).

> [!IMPORTANT]
> OwlMail ist für Entwicklung, Tests, CI und vertrauenswürdige interne Netze
> gedacht, nicht als öffentliches Produktions-MTA oder mandantenfähiger Maildienst.

## 🚀 Schnellstart

### Installation

#### Aus Quellcode kompilieren

```bash
# Repository klonen
git clone https://github.com/soulteary/owlmail.git
cd owlmail

# Kompilieren
go build -o owlmail ./cmd/owlmail

# Ausführen
./owlmail
```

#### Mit Go installieren

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@latest
owlmail
```

### Grundlegende Verwendung

```bash
# Mit Standardkonfiguration starten (SMTP: 1025, Web: 1080)
./owlmail

# Benutzerdefinierte Ports
./owlmail -smtp 1025 -web 1080

# Umgebungsvariablen verwenden
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
./owlmail
```

### Docker-Verwendung

#### Von GitHub Container Registry abrufen (Empfohlen)

Der einfachste Weg, OwlMail zu verwenden, ist das Abrufen des vorgefertigten Images von GitHub Container Registry:

```bash
# Release 0.8.0 abrufen
docker pull ghcr.io/soulteary/owlmail:0.8.0

# Image für einen exakten Commit abrufen (Beispiel)
docker pull ghcr.io/soulteary/owlmail:sha-b130f33

# Container ausführen
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  ghcr.io/soulteary/owlmail:0.8.0
```

**Verfügbare Tags:**
- `0.8.0` - Exaktes Release-Tag; `0.8` und `0` werden mit späteren Releases der Reihe aktualisiert
- `sha-<commit>` - Image für einen bestimmten kurzen Commit-SHA (z. B. `sha-b130f33`)
- `main` - Veränderliches Image des neuesten `main`-Builds
- `latest` - Veränderliches Standard-Branch-Image, kein stabiles Release-Tag

**Multi-Architektur-Unterstützung:**
Das Image unterstützt sowohl `linux/amd64` als auch `linux/arm64` Architekturen. Docker lädt automatisch das richtige Image für Ihre Plattform herunter.

**Alle verfügbaren Images anzeigen:** [GitHub Packages](https://github.com/users/soulteary/packages/container/package/owlmail)

#### Aus Quellcode erstellen

##### Grundlegender Build (Einzelarchitektur)

```bash
# Image für aktuelle Architektur erstellen
docker build -t owlmail .

# Container ausführen
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  owlmail
```

##### Multi-Architektur-Build

Für aarch64 (ARM64) oder andere Architekturen verwenden Sie Docker Buildx:

```bash
# Buildx aktivieren (falls noch nicht aktiviert)
docker buildx create --use --name multiarch-builder

# Für mehrere Architekturen erstellen
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t owlmail:latest \
  --load .

# Oder erstellen und in Registry pushen
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/owlmail:latest \
  --push .

# Für spezifische Architektur erstellen (z.B. aarch64/arm64)
docker buildx build \
  --platform linux/arm64 \
  -t owlmail:latest \
  --load .
```

**Hinweis**: Das Dockerfile unterstützt jetzt Multi-Architektur-Builds mit `TARGETOS`- und `TARGETARCH`-Build-Argumenten, die automatisch von Docker Buildx gesetzt werden.

## 📖 Konfigurationsoptionen

### Befehlszeilenargumente

| Argument | Umgebungsvariable | Standard | Beschreibung |
|----------|------------------|---------|--------------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP-Port |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP-Host |
| `-smtp-max-message-mb` | `OWLMAIL_SMTP_MAX_MESSAGE_MB` | 100 | Maximale Größe eingehender Nachrichten in MiB |
| `-smtp-max-concurrency` | `OWLMAIL_SMTP_MAX_CONCURRENCY` | 8 | Gleichzeitige DATA-Transaktionen pro Prozess über SMTP, STARTTLS und SMTPS; `0` ist unbegrenzt; bei vollem Limit folgt ein wiederholbarer Fehler `451 4.3.2` |
| `-smtp-read-timeout` | `OWLMAIL_SMTP_READ_TIMEOUT` | 10s | Lese-Timeout für SMTP-Befehle und DATA |
| `-smtp-write-timeout` | `OWLMAIL_SMTP_WRITE_TIMEOUT` | 10s | Schreib-Timeout für SMTP-Antworten |
| `-smtp-max-recipients` | `OWLMAIL_SMTP_MAX_RECIPIENTS` | 50 | Maximale Empfängerzahl pro Nachricht |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web-API-Port |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web-API-Host |
| `-web-external-url` | `OWLMAIL_WEB_EXTERNAL_URL` | - | Browser-sichtbarer HTTP(S)-Origin für generierte E-Mail-Deep-Links; Proxy-Unterpfade werden separat mit `-base-pathname` konfiguriert |
| `-base-pathname` | `MAILDEV_BASE_PATHNAME` / `OWLMAIL_BASE_PATHNAME` | - | URL-Pfadpräfix wie `/owlmail`; standardmäßig wird der Root-Pfad verwendet |
| `-maildev-rest-compat` | `OWLMAIL_MAILDEV_REST_COMPAT` | false | Aktiviert die optionale MailDev-REST-Fassade unter `/api`; Socket.IO bleibt inkompatibel |
| `-metrics-enabled` | `OWLMAIL_METRICS_ENABLED` | false | Prometheus-Metriken am Basispfad-abhängigen Endpunkt `/metrics` bereitstellen; vorhandene Web-Basic-Auth gilt ebenfalls |
| `-mcp-enabled` | `OWLMAIL_MCP_ENABLED` | false | Aktiviert den schreibgeschützten MCP-Streamable-HTTP-Endpunkt unter `/mcp` |
| `-mcp-session-timeout` | `OWLMAIL_MCP_SESSION_TIMEOUT` | 30m | Schließt inaktive MCP-Sitzungen |
| `-mcp-shutdown-timeout` | `OWLMAIL_MCP_SHUTDOWN_TIMEOUT` | 5s | Frist zum Schließen von MCP-Sitzungen beim Herunterfahren |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | E-Mail-Speicherverzeichnis |
| `-mail-retention-days` | `OWLMAIL_MAIL_RETENTION_DAYS` | 0 | Mail retention days; `0` is unlimited |
| `-mail-max-messages` | `OWLMAIL_MAIL_MAX_MESSAGES` | 0 | Maximum stored messages; `0` is unlimited |
| `-mail-max-disk-mb` | `OWLMAIL_MAIL_MAX_DISK_MB` | 0 | Maximum mailbox MiB; `0` is unlimited |
| `-mail-cleanup-interval` | `OWLMAIL_MAIL_CLEANUP_INTERVAL` | 1h | Background cleanup interval |
| `-mail-index-path` | `OWLMAIL_MAIL_INDEX_PATH` | - | Optionaler Pfad für einen wiederherstellbaren SQLite-Postfachindex; EML-Dateien bleiben maßgeblich |
| `-s3-enabled` | `OWLMAIL_S3_ENABLED` | false | Dekodierte Anhänge in S3-kompatiblem Objektspeicher ablegen |
| `-s3-endpoint` | `OWLMAIL_S3_ENDPOINT` | - | Benutzerdefinierter S3-Endpunkt; leer verwendet AWS S3 |
| `-s3-region` | `OWLMAIL_S3_REGION` | us-east-1 | S3-Region |
| `-s3-bucket` | `OWLMAIL_S3_BUCKET` | - | Vorhandener Bucket für Anhänge |
| `-s3-prefix` | `OWLMAIL_S3_PREFIX` | owlmail/attachments | Objektschlüssel-Präfix für Anhänge |
| `-s3-access-key` | `OWLMAIL_S3_ACCESS_KEY` | - | Optionaler statischer Zugriffsschlüssel |
| `-s3-secret-key` | `OWLMAIL_S3_SECRET_KEY` | - | Optionaler statischer geheimer Schlüssel |
| `-s3-session-token` | `OWLMAIL_S3_SESSION_TOKEN` | - | Optionales Sitzungstoken |
| `-s3-use-path-style` | `OWLMAIL_S3_USE_PATH_STYLE` | false | Pfadbasierte Bucket-Adressierung verwenden |
| `-s3-startup-check` | `OWLMAIL_S3_STARTUP_CHECK` | false | Start abbrechen, wenn die erste schreibgeschützte S3-Bucket-Prüfung fehlschlägt |
| `-s3-health-check-interval` | `OWLMAIL_S3_HEALTH_CHECK_INTERVAL` | 30s | Intervall für S3-Readiness-Prüfungen im Hintergrund |
| `-s3-health-check-timeout` | `OWLMAIL_S3_HEALTH_CHECK_TIMEOUT` | 5s | Zeitlimit pro S3-Readiness-Prüfung |
| `-web-user` | `MAILDEV_WEB_USER` / `OWLMAIL_WEB_USER` | - | HTTP Basic Auth Benutzername |
| `-web-password` | `MAILDEV_WEB_PASS` / `OWLMAIL_WEB_PASSWORD` | - | HTTP Basic Auth Passwort |
| `-https` | `MAILDEV_HTTPS` / `OWLMAIL_HTTPS_ENABLED` | false | HTTPS aktivieren |
| `-https-cert` | `MAILDEV_HTTPS_CERT` / `OWLMAIL_HTTPS_CERT` | - | HTTPS-Zertifikatsdatei |
| `-https-key` | `MAILDEV_HTTPS_KEY` / `OWLMAIL_HTTPS_KEY` | - | HTTPS-Private-Key-Datei |
| `-outgoing-host` | `MAILDEV_OUTGOING_HOST` / `OWLMAIL_OUTGOING_HOST` | - | Ausgehender SMTP-Host |
| `-outgoing-port` | `MAILDEV_OUTGOING_PORT` / `OWLMAIL_OUTGOING_PORT` | 587 | Ausgehender SMTP-Port |
| `-outgoing-user` | `MAILDEV_OUTGOING_USER` / `OWLMAIL_OUTGOING_USER` | - | Ausgehender SMTP-Benutzername |
| `-outgoing-pass` | `MAILDEV_OUTGOING_PASS` / `OWLMAIL_OUTGOING_PASSWORD` | - | Ausgehendes SMTP-Passwort |
| `-outgoing-secure` | `MAILDEV_OUTGOING_SECURE` / `OWLMAIL_OUTGOING_SECURE` | false | MailDev-kompatibler Alias für implizites TLS/SMTPS |
| `-outgoing-tls-mode` | `OWLMAIL_OUTGOING_TLS_MODE` | - | Ohne Wert `plain`; sonst verpflichtendes `starttls` oder implizites `smtps` |
| `-outgoing-insecure-skip-verify` | `OWLMAIL_OUTGOING_INSECURE_SKIP_VERIFY` | false | Zertifikats-/Hostnamenprüfung deaktivieren (unsicher) |
| `-outgoing-connect-timeout` | `OWLMAIL_OUTGOING_CONNECT_TIMEOUT` | 10s | Connect-/Greeting-Frist |
| `-outgoing-tls-handshake-timeout` | `OWLMAIL_OUTGOING_TLS_HANDSHAKE_TIMEOUT` | 10s | TLS-Handshake-Frist |
| `-outgoing-auth-timeout` | `OWLMAIL_OUTGOING_AUTH_TIMEOUT` | 10s | AUTH-Frist |
| `-outgoing-envelope-timeout` | `OWLMAIL_OUTGOING_ENVELOPE_TIMEOUT` | 10s | MAIL/RCPT-Frist |
| `-outgoing-data-timeout` | `OWLMAIL_OUTGOING_DATA_TIMEOUT` | 30s | DATA-Frist |
| `-outgoing-quit-timeout` | `OWLMAIL_OUTGOING_QUIT_TIMEOUT` | 5s | QUIT-Frist |
| `-auto-relay` | `MAILDEV_AUTO_RELAY` / `OWLMAIL_AUTO_RELAY` | false | Auto-Relay aktivieren |
| `-auto-relay-addr` | `MAILDEV_AUTO_RELAY_ADDR` / `OWLMAIL_AUTO_RELAY_ADDR` | - | Auto-Relay-Adresse |
| `-auto-relay-rules` | `MAILDEV_AUTO_RELAY_RULES` / `OWLMAIL_AUTO_RELAY_RULES` | - | Auto-Relay-Regeldatei |
| `-webhook-config` | `OWLMAIL_WEBHOOK_CONFIG` | - | JSON-Konfigurationsdatei für Webhook-Weiterleitung |
| `-webhook-max-concurrency` | `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` | 8 | Gleichzeitige Webhook-Zustellungen; `0` deaktiviert die Begrenzung |
| `-webhook-redis-url` | `OWLMAIL_WEBHOOK_REDIS_URL` | - | Redis URL for durable webhook delivery |
| `-webhook-redis-prefix` | `OWLMAIL_WEBHOOK_REDIS_PREFIX` | owlmail:webhooks | Redis Streams key prefix |
| `-webhook-shutdown-timeout` | `OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT` | 15s | Graceful webhook drain deadline |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | Eingehender SMTP-Benutzername; zusammen mit dem Passwort erzwingt er AUTH |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | Eingehendes SMTP-Passwort; zusammen mit dem Benutzernamen erzwingt es AUTH |
| `-smtp-auth-require-tls` | `OWLMAIL_SMTP_AUTH_REQUIRE_TLS` | false | PLAIN/LOGIN vor TLS ablehnen; SMTP TLS muss aktiviert sein |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | SMTP TLS aktivieren |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS-Zertifikatsdatei |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS-Private-Key-Datei |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | Protokollierungsstufe |
| `-mailcatcher-rest-compat` | `OWLMAIL_MAILCATCHER_REST_COMPAT` | false | Optionale MailCatcher-REST-Kompatibilität aktivieren |
| `-config` | `OWLMAIL_CONFIG_FILE` | - | YAML- oder JSON-Konfigurationsdatei beim Start laden; CLI und Umgebung haben Vorrang |
| `-log-format` | `OWLMAIL_LOG_FORMAT` | console | Protokollformat: `console` oder `json` |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | UUID für E-Mail-IDs verwenden (Standard: 8-Zeichen-Zufallszeichenfolge) |

When TLS terminates at a reverse proxy, set `OWLMAIL_WEB_EXTERNAL_SCHEME` to `https`.

Eine unvollständige Web-Authentifizierung wird nicht stillschweigend deaktiviert:

| Konfigurierte Werte | Effektive Zugangsdaten |
|---|---|
| Keine | Authentifizierung deaktiviert |
| Nur Benutzername | Der Benutzername und ein kryptografisch zufälliges temporäres Passwort mit 32 Zeichen; das Passwort wird beim Start einmal auf stderr ausgegeben |
| Nur Passwort | Benutzername `admin` und das konfigurierte Passwort |
| Beide Werte | Der konfigurierte Benutzername und das konfigurierte Passwort |

Ein generiertes Passwort ändert sich bei jedem Neustart. Lesen Sie es aus der
Prozessausgabe (`docker logs owlmail` beim Container-Beispiel), oder konfigurieren
Sie beide Werte für stabile Zugangsdaten. OwlMail startet nicht, wenn das
generierte Passwort nicht auf stderr geschrieben werden kann. Verwenden Sie
Basic Auth nur über localhost oder HTTPS.

### Umgebungsvariablen-Kompatibilität

OwlMail unterstützt die in der Tabelle aufgeführten MailDev-Umgebungsaliase und
priorisiert sie vor den entsprechenden `OWLMAIL_*`-Variablen. Nicht aufgeführte
MailDev-Optionen werden nicht automatisch unterstützt.

```bash
# MailDev-Umgebungsvariablen direkt verwenden (empfohlen)
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com
./owlmail

# Oder OwlMail-Umgebungsvariablen verwenden
export OWLMAIL_SMTP_PORT=1025
export OWLMAIL_WEB_PORT=1080
./owlmail
```

## 📡 API-Dokumentation

### API-Antwortformat

OwlMail verwendet ein standardisiertes API-Antwortformat:

**Erfolgreiche Antwort:**
```json
{
  "code": "EMAIL_DELETED",
  "message": "Email deleted",
  "data": { ... }
}
```

**Fehlerantwort:**
```json
{
  "code": "EMAIL_NOT_FOUND",
  "error": "EMAIL_NOT_FOUND",
  "message": "Email not found"
}
```

Das Feld `code` enthält standardisierte Fehler-/Erfolgscodes, die für die Internationalisierung verwendet werden können. Das Feld `message` bietet englischen Text für Rückwärtskompatibilität.
Fehler der Basic-Auth- und Browser-Same-Origin-Middleware sind einfache
Textantworten mit `401` beziehungsweise `403`, da sie vor dem API-Handler auftreten.

### E-Mail-ID-Format

OwlMail unterstützt zwei E-Mail-ID-Formate, und alle API-Endpunkte sind mit beiden kompatibel:

- **8-Zeichen-Zufallszeichenfolge**: Standardformat, z.B. `aB3dEfGh`
- **UUID-Format**: 36-Zeichen-Standard-UUID, z.B. `550e8400-e29b-41d4-a716-446655440000`

Bei Verwendung des `:id`-Parameters in API-Anfragen können Sie beide Formate verwenden. Zum Beispiel:
- `GET /email/aB3dEfGh` - Zufallszeichenfolgen-ID verwenden
- `GET /email/550e8400-e29b-41d4-a716-446655440000` - UUID-ID verwenden

### MailDev-ähnliche Kompatibilitätsrouten

OwlMail behält unversionierte Routen für gängige Workflows bei. Sie sind keine
exakten Entsprechungen der aktuellen MailDev-API; siehe
[API-Referenz](./docs/en/API-Reference.md#maildev-migration-boundary).

#### E-Mail-Operationen

- `GET /email` - Alle E-Mails abrufen (unterstützt Paginierung und Filterung)
  - Abfrageparameter:
    - `limit` (Standard: 50, Max: 1000) - Anzahl der zurückzugebenden E-Mails
    - `offset` (Standard: 0) - Anzahl der zu überspringenden E-Mails
    - `q` - Volltextsuchabfrage
    - `from` - Nach Absender-E-Mail-Adresse filtern
    - `to` - Nach Empfänger-E-Mail-Adresse filtern
    - `dateFrom` - Nach Datum von filtern (YYYY-MM-DD Format)
    - `dateTo` - Nach Datum bis filtern (YYYY-MM-DD Format)
    - `read` - Nach Lesestatus filtern (true/false)
    - `sortBy` - Nach Feld sortieren (time, subject, from, size)
    - `sortOrder` - Sortierreihenfolge (asc, desc, Standard: desc)
  - Beispiel: `GET /email?limit=20&offset=0&q=test&sortBy=time&sortOrder=desc`
- `GET /email/:id` - Einzelne E-Mail abrufen
- `DELETE /email/:id` - Einzelne E-Mail löschen
- `DELETE /email/all` - Alle E-Mails löschen
- `PATCH /email/read-all` - Alle E-Mails als gelesen markieren
- `PATCH /email/:id/read` - Einzelne E-Mail als gelesen markieren

#### E-Mail-Inhalt

- `GET /email/:id/html` - E-Mail-HTML-Inhalt abrufen
- `GET /email/:id/attachment/:filename` - Anhang herunterladen
- `GET /email/:id/download` - Rohe .eml-Datei herunterladen
- `GET /email/:id/source` - Rohe E-Mail-Quelle abrufen

#### E-Mail-Weiterleitung

- `POST /email/:id/relay` - E-Mail an konfigurierten SMTP-Server weiterleiten
- `POST /email/:id/relay/:relayTo` - E-Mail an bestimmte Adresse weiterleiten

#### Konfiguration und System

- `GET /config` - Konfigurationsinformationen abrufen
- `GET /healthz` - Gesundheitsprüfung
- `GET /reloadMailsFromDirectory` - E-Mails aus Verzeichnis neu laden
- `GET /socket.io` - WebSocket-Verbindung (Standard WebSocket, nicht Socket.IO)

### OwlMail erweiterte API

#### E-Mail-Statistiken und Vorschau

- `GET /email/stats` - E-Mail-Statistiken abrufen
- `GET /email/preview` - E-Mail-Vorschau abrufen (leichtgewichtig)

#### Batch-Operationen

- `POST /email/batch/delete` - E-Mails im Batch löschen
- `POST /email/batch/read` - Im Batch als gelesen markieren

#### E-Mail-Export

- `GET /email/export` - E-Mails als ZIP-Datei exportieren

#### Konfigurationsverwaltung

- `GET /config/outgoing` - Ausgehende Konfiguration abrufen
- `PUT /config/outgoing` - Ausgehende Konfiguration aktualisieren
- `PATCH /config/outgoing` - Ausgehende Konfiguration teilweise aktualisieren

### Verbesserte RESTful API (`/api/v1/*`)

OwlMail bietet ein standardisierteres RESTful API-Design:

- `GET /api/v1/emails` - Alle E-Mails abrufen (Plural-Ressource)
  - Abfrageparameter: Gleich wie `GET /email` (limit, offset, q, from, to, dateFrom, dateTo, read, sortBy, sortOrder)
  - Beispiel: `GET /api/v1/emails?limit=20&offset=0&q=test&sortBy=time&sortOrder=desc`
- `GET /api/v1/emails/:id` - Einzelne E-Mail abrufen
- `DELETE /api/v1/emails/:id` - Einzelne E-Mail löschen
- `DELETE /api/v1/emails` - Alle E-Mails löschen
- `DELETE /api/v1/emails/batch` - Batch-Löschen
- `PATCH /api/v1/emails/read` - Alle E-Mails als gelesen markieren
- `PATCH /api/v1/emails/:id/read` - Einzelne E-Mail als gelesen markieren
- `PATCH /api/v1/emails/batch/read` - Im Batch als gelesen markieren
- `GET /api/v1/emails/stats` - E-Mail-Statistiken
- `GET /api/v1/emails/preview` - E-Mail-Vorschau
- `GET /api/v1/emails/export` - E-Mails exportieren
- `POST /api/v1/emails/reload` - E-Mails neu laden
- `GET /api/v1/settings` - Alle Einstellungen abrufen
- `GET /api/v1/settings/outgoing` - Ausgehende Konfiguration abrufen
- `PUT /api/v1/settings/outgoing` - Ausgehende Konfiguration aktualisieren
- `PATCH /api/v1/settings/outgoing` - Ausgehende Konfiguration teilweise aktualisieren
- `GET /api/v1/health` - Gesundheitsprüfung
- `GET /api/v1/ready` - Zwischengespeicherte Readiness-Prüfung
- `GET /api/v1/version` - Versionsinformationen
- `GET /api/v1/ws` - WebSocket-Verbindung
- `GET /api/v1/openapi.json` - OpenAPI-3.1-Vertrag (JSON)
- `GET /api/v1/openapi.yaml` - OpenAPI-3.1-Vertrag (YAML)

Die aktuelle Schnittstelle einschließlich Unterressourcen, Authentifizierung,
Antwortformen und WebSocket-Ereignissen beschreibt die
[API-Referenz](./docs/en/API-Reference.md) beziehungsweise der
[OpenAPI-Vertrag](./openapi/openapi.yaml). Der ausgelieferte Vertrag enthält
automatisch den konfigurierten Basispfad.

## 🔧 Verwendungsbeispiele

### Grundlegende Verwendung

```bash
# OwlMail starten
./owlmail -smtp 1025 -web 1080

# SMTP in Ihrer Anwendung konfigurieren
SMTP_HOST=localhost
SMTP_PORT=1025
```

### E-Mail-Weiterleitung konfigurieren

```bash
# An Gmail SMTP weiterleiten
./owlmail \
  -outgoing-host smtp.gmail.com \
  -outgoing-port 587 \
  -outgoing-user your-email@gmail.com \
  -outgoing-pass your-password \
  -outgoing-tls-mode starttls
```

`starttls` schlägt ohne angekündigtes STARTTLS oder bei TLS-/Zertifikatsfehlern
fehl und fällt nie auf Klartext zurück. `smtps` verwendet TLS ab Verbindungsaufbau;
AUTH ist im Modus `plain` nicht erlaubt.

### Auto-Relay-Modus

```bash
# Auto-Relay-Regeldatei erstellen (relay-rules.json)
cat > relay-rules.json <<EOF
[
  { "allow": "*" },
  { "deny": "*@test.com" },
  { "allow": "ok@test.com" }
]
EOF

# Auto-Relay starten
./owlmail \
  -outgoing-host smtp.gmail.com \
  -outgoing-port 587 \
  -outgoing-user your-email@gmail.com \
  -outgoing-pass your-password \
  -auto-relay \
  -auto-relay-rules relay-rules.json
```

### HTTPS verwenden

```bash
./owlmail \
  -https \
  -https-cert /path/to/cert.pem \
  -https-key /path/to/key.pem \
  -web 1080
```

### Modi der eingehenden SMTP-Authentifizierung

Ohne `-smtp-user` und `-smtp-password` verwendet OwlMail standardmäßig **NO
AUTH**. Nicht authentifizierte Zustellung ist erlaubt; PLAIN/LOGIN werden
dennoch angeboten und beliebige Zugangsdaten akzeptiert, damit Anwendungen mit
obligatorischen SMTP-Zugangsdaten ohne Serverkonfiguration getestet werden
können. Sind beide Werte gesetzt, ist SMTP AUTH erforderlich. Ein einzelner
Wert verhindert den Start, statt unbemerkt auf NO AUTH zurückzufallen.

Aktivieren Sie `-smtp-auth-require-tls` (oder
`OWLMAIL_SMTP_AUTH_REQUIRE_TLS=true`) zusammen mit SMTP TLS, damit Zugangsdaten
nicht über eine unverschlüsselte Verbindung übertragen werden. SMTP im Klartext
bietet PLAIN/LOGIN dann weder an noch akzeptiert es diese Verfahren; nach
STARTTLS und über SMTPS funktioniert AUTH weiterhin. Die anonyme Zustellung im
NO-AUTH-Modus bleibt erhalten. Ohne aktivierte, nutzbare SMTP-TLS-Konfiguration
verhindert diese Option den Start.

> [!WARNING]
> NO AUTH bietet absichtlich keine Zugriffskontrolle. PLAIN/LOGIN sind für die
> Entwicklung auch ohne TLS erlaubt; verwenden Sie echte Zugangsdaten nur mit
> TLS und isolieren Sie den Listener.

### TLS verwenden

```bash
./owlmail \
  -tls \
  -tls-cert /path/to/cert.pem \
  -tls-key /path/to/key.pem \
  -smtp 1025
```

**Hinweis**: Wenn TLS aktiviert ist, startet OwlMail automatisch zusätzlich zum regulären SMTP-Server einen SMTPS-Server auf Port 465. Der SMTPS-Server verwendet eine direkte TLS-Verbindung (kein STARTTLS erforderlich).

### UUID für E-Mail-IDs verwenden

OwlMail unterstützt zwei E-Mail-ID-Formate:

1. **Standardformat**: 8-Zeichen-Zufallszeichenfolge (z.B. `aB3dEfGh`)
2. **UUID-Format**: 36-Zeichen-Standard-UUID (z.B. `550e8400-e29b-41d4-a716-446655440000`)

Die Verwendung des UUID-Formats bietet bessere Eindeutigkeit und Nachverfolgbarkeit, besonders nützlich für die Integration mit externen Systemen.

```bash
# UUID mit Befehlszeilenflag aktivieren
./owlmail -use-uuid-for-email-id

# UUID mit Umgebungsvariable aktivieren
export OWLMAIL_USE_UUID_FOR_EMAIL_ID=true
./owlmail

# Mit anderen Konfigurationen verwenden
./owlmail \
  -use-uuid-for-email-id \
  -smtp 1025 \
  -web 1080
```

**Hinweise**:
- Standard verwendet 8-Zeichen-Zufallszeichenfolge, kompatibel mit MailDev-Verhalten
- Wenn UUID aktiviert ist, verwenden alle neu empfangenen E-Mails UUID-Format-IDs
- Die API unterstützt beide ID-Formate, ermöglicht normale Abfrage, Löschung und Operation von E-Mails
- Bestehende E-Mail-ID-Formate ändern sich nicht; nur neue E-Mails verwenden das neue ID-Format

## 🔄 Migration von MailDev

OwlMail deckt gängige MailDev-Workflows ab; aktuelle MailDev-Clients können jedoch
gezielte Anpassungen benötigen. Folgen Sie dem
[Migrationsleitfaden](./docs/de/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md).

### 1. Umgebungsvariablen-Kompatibilität

OwlMail akzeptiert die in der Konfigurationstabelle aufgeführten MailDev-Variablen.
Prüfen Sie jede in Ihrer Bereitstellung verwendete Variable:

```bash
# MailDev-Konfiguration
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# Diese aufgeführten Variablen kann auch OwlMail lesen
./owlmail
```

### 2. API-Kompatibilität

Bestehende REST-Clients können die standardmäßig deaktivierte MailDev-Fassade
explizit aktivieren. Neue Integrationen sollten die versionierte OwlMail-API
verwenden. Die Fassade bietet keine Socket.IO-Kompatibilität:

```bash
# Bestehender MailDev-REST-Client
OWLMAIL_MAILDEV_REST_COMPAT=true ./owlmail
curl http://localhost:1080/api/email

# Neue OwlMail-Integration
curl http://localhost:1080/api/v1/emails
```

### 3. WebSocket-Anpassung

Wenn Sie WebSocket verwenden, müssen Sie von Socket.IO auf Standard WebSocket umstellen:

```javascript
// MailDev (Socket.IO)
const socket = io('/socket.io');
socket.on('newMail', (email) => { /* ... */ });

// OwlMail (Standard WebSocket)
const ws = new WebSocket('ws://localhost:1080/socket.io');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'new') { /* ... */ }
};
```

Detaillierte Migrationsanleitung finden Sie unter: [OwlMail × MailDev: Vollständiger Funktions- und API-Vergleich und Migrations-Whitepaper](./docs/de/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

## 🧪 Tests

```bash
# Alle Tests ausführen
go test ./...

# Tests mit Abdeckung ausführen
go test -cover ./...

# Tests für spezifische Pakete ausführen
go test ./internal/api/...
go test ./internal/mailserver/...
```

## 📦 Projektstruktur

```
OwlMail/
├── cmd/
│   └── owlmail/          # Hauptprogrammeinstieg
├── internal/
│   ├── api/              # Web-API-Implementierung
│   ├── common/           # Gemeinsame Utilities (Protokollierung, Fehlerbehandlung)
│   ├── maildev/          # MailDev-Kompatibilitätsschicht
│   ├── mailserver/       # SMTP-Server-Implementierung
│   ├── outgoing/         # E-Mail-Weiterleitungsimplementierung
│   ├── types/            # Typdefinitionen
│   └── webhook/          # Webhook-Filterung, Vorlagen, Signaturen und Zustellung
├── docs/                 # API-, Betriebs-, Webhook- und Migrationsdokumentation
├── examples/             # Ausführbare Integrationsbeispiele
├── tests/                # Browser- und Dokumentationsvertragstests
├── web/                  # Web-Frontend-Dateien
├── go.mod                # Go-Moduldefinition
└── README.md             # Dieses Dokument
```

## 🤝 Beitragen

Beiträge sind willkommen! Bitte folgen Sie diesen Schritten:

1. Repository forken
2. Feature-Branch erstellen (`git checkout -b feature/AmazingFeature`)
3. Änderungen committen (`git commit -m 'Add some AmazingFeature'`)
4. Auf Branch pushen (`git push origin feature/AmazingFeature`)
5. Pull Request öffnen

## 📄 Lizenz

Dieses Projekt ist unter der MIT-Lizenz lizenziert - siehe [LICENSE](LICENSE)-Datei für Details.

## 🙏 Danksagungen

- [MailDev](https://github.com/maildev/maildev) - Originalprojekt-Inspiration
- [emersion/go-smtp](https://github.com/emersion/go-smtp) - SMTP-Server-Bibliothek
- [emersion/go-message](https://github.com/emersion/go-message) - E-Mail-Parsing-Bibliothek
- [Fiber](https://github.com/gofiber/fiber) - Web-Framework
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket-Bibliothek

## 📚 Verwandte Dokumentation

- [Versionshinweise zu OwlMail 0.8.0](./docs/en/Release-0.8.0.md) ([中文](./docs/zh-CN/Release-0.8.0.md))
- [Änderungsprotokoll](./CHANGELOG.md)
- [OwlMail × MailDev: Vollständiger Funktions- und API-Vergleich und Migrations-Whitepaper](./docs/de/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [API-Referenz (English)](./docs/en/API-Reference.md)
- [Betrieb und Fehlerbehebung (English)](./docs/en/Operations.md)
- [Webhook-Weiterleitung (English)](./docs/en/Webhook-Forwarding.md)
- [Veröffentlichungsprozess (English)](./docs/en/Releasing.md)
- [API-Refactoring-Aufzeichnung (historisch)](./docs/de/internal/API_Refactoring_Record.md)

## 🐛 Problemberichterstattung

Wenn Sie auf Probleme stoßen oder Vorschläge haben, senden Sie diese bitte in [GitHub Issues](https://github.com/soulteary/owlmail/issues).

## ⭐ Star-Verlauf

Wenn dieses Projekt Ihnen hilft, geben Sie bitte einen Star ⭐!

---

**OwlMail** — ein selbst gehostetes E-Mail-Test-Gateway für Entwicklung, CI, Automatisierung und AI Agents. 🦉
