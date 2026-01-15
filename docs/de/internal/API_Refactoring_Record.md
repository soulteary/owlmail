# API-Refactoring-Aufzeichnung

## Übersicht

Dieses Dokument zeichnet den API-Refactoring-Prozess von OwlMail auf und dokumentiert die Migration von MailDev-kompatiblen API-Endpunkten zu einem neuen RESTful API-Design (`/api/v1/`). Das Refactoring behält vollständige Rückwärtskompatibilität bei und führt gleichzeitig verbesserte API-Designmuster ein.

## Refactoring-Ziele

Das Refactoring wurde initiiert, um mehrere API-Designprobleme zu beheben:

1. **Inkonsistente Ressourcennamen** - Verwendung von Singular `/email` statt Plural `/emails`
2. **Nicht standardmäßiges RESTful-Design** - Verwendung von `/all`-Suffixen und falschen HTTP-Methoden
3. **Unklare Aktionsnamen** - Aktionen nicht explizit identifiziert
4. **Inkonsistente Unterressourcen-Namen** - Verwendung von Singular `attachment` statt Plural
5. **Weniger semantische Pfadnamen** - Verwendung generischer Begriffe wie `/download` statt `/raw`
6. **Fehlende API-Versionierung** - Kein Versionspräfix für zukünftige API-Evolution
7. **Falsche HTTP-Methodenverwendung** - Verwendung von GET für zustandsändernde Operationen

## API-Designprobleme-Analyse

### 1. Inkonsistente Ressourcennamen

**Problem:**
- Verwendung von Singular `/email` statt Plural `/emails`
- RESTful-Best-Practices empfehlen die Verwendung von Pluralformen für Ressourcensammlungen

**Beispiel:**
```
❌ GET /email          (Singular)
✅ GET /emails         (Plural)
```

### 2. Nicht standardmäßiges RESTful-Design

**Problem:**
- `DELETE /email/all` - Verwendung von `/all`-Suffix ist nicht RESTful
- `POST /email/batch/delete` - Verwendung von POST für Löschoperationen ist nicht semantisch
- `PATCH /email/read-all` - Verwendung von Bindestrich-Namen ist nicht klar

**Verbesserung:**
```
❌ DELETE /email/all
✅ DELETE /emails

❌ POST /email/batch/delete
✅ DELETE /emails/batch

❌ PATCH /email/read-all
✅ PATCH /emails/read
```

### 3. Unklare Aktionsnamen

**Problem:**
- `/email/:id/relay` - Aktion ist nicht explizit
- Sollte klar anzeigen, dass dies eine Aktionsoperation ist

**Verbesserung:**
```
❌ POST /email/:id/relay
✅ POST /emails/:id/actions/relay
```

### 4. Inkonsistente Unterressourcen-Namen

**Problem:**
- `/email/:id/attachment/:filename` - Verwendung von Singular `attachment`
- Sollte Plural `attachments` verwenden, um Ressourcensammlung darzustellen

**Verbesserung:**
```
❌ GET /email/:id/attachment/:filename
✅ GET /emails/:id/attachments/:filename
```

### 5. Weniger semantische Pfadnamen

**Problem:**
- `/email/:id/download` - `download` ist nicht semantisch genug
- `/config` - `config` ist weniger semantisch als `settings`
- `/healthz` - Nicht standardmäßige Benennung
- `/reloadMailsFromDirectory` - CamelCase-Benennung entspricht nicht dem RESTful-Stil

**Verbesserung:**
```
❌ GET /email/:id/download
✅ GET /emails/:id/raw

❌ GET /config
✅ GET /settings

❌ GET /healthz
✅ GET /health

❌ GET /reloadMailsFromDirectory
✅ POST /emails/reload
```

### 6. Falsche HTTP-Methodenverwendung

**Problem:**
- `GET /reloadMailsFromDirectory` - Neuladen ist eine zustandsändernde Operation, sollte POST verwenden
- `POST /email/batch/delete` - Löschoperationen sollten DELETE verwenden

**Verbesserung:**
```
❌ GET /reloadMailsFromDirectory
✅ POST /emails/reload

❌ POST /email/batch/delete
✅ DELETE /emails/batch
```

### 7. Fehlende API-Versionierung

**Problem:**
- Kein API-Versionspräfix
- Kann API-Versionen nicht entwickeln

**Verbesserung:**
```
❌ GET /email
✅ GET /api/v1/emails
```

## Refactored API-Design

### MailDev-kompatible API (für Rückwärtskompatibilität beibehalten)

Alle ursprünglichen MailDev API-Endpunkte werden beibehalten, um Rückwärtskompatibilität zu gewährleisten:

| Methode | Pfad | Beschreibung |
|--------|------|-------------|
| GET | `/email` | Alle E-Mails abrufen |
| GET | `/email/:id` | Einzelne E-Mail abrufen |
| GET | `/email/:id/html` | E-Mail-HTML abrufen |
| GET | `/email/:id/attachment/:filename` | Anhang herunterladen |
| GET | `/email/:id/download` | Originale .eml-Datei herunterladen |
| GET | `/email/:id/source` | E-Mail-Rohquelle abrufen |
| DELETE | `/email/:id` | Einzelne E-Mail löschen |
| DELETE | `/email/all` | Alle E-Mails löschen |
| PATCH | `/email/read-all` | Alle E-Mails als gelesen markieren |
| PATCH | `/email/:id/read` | Einzelne E-Mail als gelesen markieren |
| POST | `/email/:id/relay` | E-Mail weiterleiten |
| POST | `/email/:id/relay/:relayTo` | E-Mail an angegebene Adresse weiterleiten |
| GET | `/email/stats` | E-Mail-Statistiken |
| GET | `/email/preview` | E-Mail-Vorschau |
| POST | `/email/batch/delete` | Batch-Löschen |
| POST | `/email/batch/read` | Batch als gelesen markieren |
| GET | `/email/export` | E-Mails exportieren |
| GET | `/config` | Konfiguration abrufen |
| GET | `/config/outgoing` | Ausgehende Konfiguration abrufen |
| PUT | `/config/outgoing` | Ausgehende Konfiguration aktualisieren |
| PATCH | `/config/outgoing` | Ausgehende Konfiguration teilweise aktualisieren |
| GET | `/healthz` | Gesundheitsprüfung |
| GET | `/reloadMailsFromDirectory` | E-Mails neu laden |
| GET | `/socket.io` | WebSocket-Verbindung |

### Neue verbesserte API (empfohlen)

#### E-Mail-Ressourcen (`/api/v1/emails`)

| Methode | Pfad | Beschreibung | Verbesserung |
|--------|------|-------------|-------------|
| GET | `/api/v1/emails` | Alle E-Mails abrufen | Plural-Ressource verwenden |
| GET | `/api/v1/emails/:id` | Einzelne E-Mail abrufen | Plural-Ressource verwenden |
| DELETE | `/api/v1/emails/:id` | Einzelne E-Mail löschen | Plural-Ressource verwenden |
| DELETE | `/api/v1/emails` | Alle E-Mails löschen | RESTfuler, kein `/all`-Suffix |
| DELETE | `/api/v1/emails/batch` | Batch-Löschen | DELETE statt POST verwenden |
| PATCH | `/api/v1/emails/read` | Alle E-Mails als gelesen markieren | Klarere Benennung |
| PATCH | `/api/v1/emails/:id/read` | Einzelne E-Mail als gelesen markieren | Plural-Ressource verwenden |
| PATCH | `/api/v1/emails/batch/read` | Batch als gelesen markieren | Plural-Ressource verwenden |
| GET | `/api/v1/emails/stats` | E-Mail-Statistiken | Plural-Ressource verwenden |
| GET | `/api/v1/emails/preview` | E-Mail-Vorschau | Plural-Ressource verwenden |
| GET | `/api/v1/emails/export` | E-Mails exportieren | Plural-Ressource verwenden |
| POST | `/api/v1/emails/reload` | E-Mails neu laden | POST statt GET verwenden |

#### E-Mail-Inhaltsressourcen

| Methode | Pfad | Beschreibung | Verbesserung |
|--------|------|-------------|-------------|
| GET | `/api/v1/emails/:id/html` | E-Mail-HTML abrufen | Plural-Ressource verwenden |
| GET | `/api/v1/emails/:id/source` | E-Mail-Quelle abrufen | Plural-Ressource verwenden |
| GET | `/api/v1/emails/:id/raw` | Rohe E-Mail abrufen | Semantischere Benennung (ersetzt `/download`) |
| GET | `/api/v1/emails/:id/attachments/:filename` | Anhang herunterladen | Plural `attachments` verwenden |

#### E-Mail-Aktionen

| Methode | Pfad | Beschreibung | Verbesserung |
|--------|------|-------------|-------------|
| POST | `/api/v1/emails/:id/actions/relay` | E-Mail weiterleiten | Zeigt explizit Aktionsoperation an |
| POST | `/api/v1/emails/:id/actions/relay/:relayTo` | E-Mail an angegebene Adresse weiterleiten | Zeigt explizit Aktionsoperation an |

#### Einstellungsressourcen (`/api/v1/settings`)

| Methode | Pfad | Beschreibung | Verbesserung |
|--------|------|-------------|-------------|
| GET | `/api/v1/settings` | Alle Einstellungen abrufen | Semantischere Benennung (ersetzt `/config`) |
| GET | `/api/v1/settings/outgoing` | Ausgehende Konfiguration abrufen | Semantischere Benennung |
| PUT | `/api/v1/settings/outgoing` | Ausgehende Konfiguration aktualisieren | Semantischere Benennung |
| PATCH | `/api/v1/settings/outgoing` | Ausgehende Konfiguration teilweise aktualisieren | Semantischere Benennung |

#### Systemressourcen

| Methode | Pfad | Beschreibung | Verbesserung |
|--------|------|-------------|-------------|
| GET | `/api/v1/health` | Gesundheitsprüfung | Standardmäßigere Benennung (ersetzt `/healthz`) |
| GET | `/api/v1/ws` | WebSocket-Verbindung | Klarerer Pfad (ersetzt `/socket.io`) |

## Frontend-Migration

### Migrationsübersicht

Die Frontend-Oberfläche wurde von der MailDev-kompatiblen API zum neuen RESTful API-Design (`/api/v1/`) migriert.

### API-Basis-Pfad-Migration

**Alte API:**
```javascript
const API_BASE = window.location.origin;
```

**Neue API:**
```javascript
const API_BASE = `${window.location.origin}/api/v1`;
```

### API-Endpunkt-Migrationsreferenz

| Funktionalität | Alte API (MailDev-kompatibel) | Neue API (empfohlen) | Hinweise |
|---------------|------------------------------|----------------------|-------|
| Alle E-Mails abrufen | `GET /email` | `GET /api/v1/emails` | Plural-Ressource verwenden |
| Einzelne E-Mail abrufen | `GET /email/:id` | `GET /api/v1/emails/:id` | Plural-Ressource verwenden |
| E-Mail-HTML abrufen | `GET /email/:id/html` | `GET /api/v1/emails/:id/html` | Plural-Ressource verwenden |
| Anhang herunterladen | `GET /email/:id/attachment/:filename` | `GET /api/v1/emails/:id/attachments/:filename` | Plural `attachments` verwenden |
| Rohe E-Mail herunterladen | `GET /email/:id/download` | `GET /api/v1/emails/:id/raw` | Semantischere Benennung |
| E-Mail-Quelle anzeigen | `GET /email/:id/source` | `GET /api/v1/emails/:id/source` | Plural-Ressource verwenden |
| Einzelne E-Mail löschen | `DELETE /email/:id` | `DELETE /api/v1/emails/:id` | Plural-Ressource verwenden |
| Alle E-Mails löschen | `DELETE /email/all` | `DELETE /api/v1/emails` | RESTfuler, kein `/all` |
| Alle als gelesen markieren | `PATCH /email/read-all` | `PATCH /api/v1/emails/read` | Klarere Benennung |
| E-Mail weiterleiten | `POST /email/:id/relay` | `POST /api/v1/emails/:id/actions/relay` | Zeigt explizit Aktion an |
| WebSocket-Verbindung | `GET /socket.io` | `GET /api/v1/ws` | Klarerer Pfad |

## Refactoring-Vorteile

### 1. Bessere RESTful-Designprinzipien
- ✅ Plural-Ressourcen verwenden: `/emails` statt `/email`
- ✅ Standardmäßigere Batch-Operationen: `DELETE /emails` statt `DELETE /email/all`
- ✅ Klarere Aktionsoperationen: `/actions/relay` zeigt explizit Aktionen an

### 2. Semantischere Benennung
- ✅ `/raw` ersetzt `/download` (semantischer)
- ✅ `/attachments` verwendet Plural (standardmäßiger)
- ✅ `/ws` ersetzt `/socket.io` (prägnanter)

### 3. API-Versionierung
- ✅ Alle APIs verwenden `/api/v1/`-Präfix
- ✅ Unterstützung für zukünftige API-Versionsentwicklung

### 4. Bessere Wartbarkeit
- ✅ Einheitlicher API-Designstil
- ✅ Klare Ressourcenhierarchie
- ✅ Einfach zu verstehen und zu erweitern

### 5. Verbesserte HTTP-Methodenverwendung
- ✅ Neuladen verwendet `POST /emails/reload` statt `GET /reloadMailsFromDirectory`
- ✅ Löschoperationen verwenden `DELETE` statt `POST`

### 6. Einheitlicher Benennungsstil
- ✅ Kleinbuchstaben und Bindestriche verwenden (kebab-case)
- ✅ CamelCase-Benennung vermeiden

## Kompatibilitätsgarantie

Während das Frontend zur neuen API migriert wurde, behält das Backend alle MailDev-kompatiblen ursprünglichen API-Endpunkte bei, um sicherzustellen:

- ✅ Bestehender Client-Code kann weiterhin die alte API verwenden
- ✅ Neue Clients können die verbesserte API verwenden
- ✅ Beide API-Designs können gleichzeitig verwendet werden
- ✅ Sanfter Migrationspfad

## Testempfehlungen

Nach der Migration sollten die folgenden Funktionalitäten getestet werden:

1. ✅ E-Mail-Listenladung
2. ✅ E-Mail-Detailanzeige
3. ✅ E-Mail-Löschung (einzeln und alle)
4. ✅ Als-gelesen-markieren-Funktionalität
5. ✅ Anhang-Download
6. ✅ Rohe E-Mail-Download
7. ✅ E-Mail-Quellenanzeige
8. ✅ WebSocket-Echtzeitaktualisierungen
9. ✅ E-Mail-Weiterleitungsfunktionalität

## Best Practices

1. **Neue Projekte**: Empfohlen, die neue `/api/v1/`-API zu verwenden
2. **Bestehende Projekte**: Können weiterhin die MailDev-kompatible API verwenden, schrittweise migrieren
3. **Gemischte Verwendung**: Beide APIs können gleichzeitig verwendet werden, je nach Bedarf wählen

## Zusammenfassung

Durch dieses API-Refactoring haben wir:

1. ✅ Vollständige Rückwärtskompatibilität beibehalten (alle MailDev-APIs sind erhalten)
2. ✅ Ein neues API-Design bereitgestellt, das besser den RESTful-Best-Practices entspricht
3. ✅ Ressourcennamenkonventionen vereinheitlicht (Pluralformen verwenden)
4. ✅ HTTP-Methodenverwendung verbessert (semantischer)
5. ✅ API-Versionierung hinzugefügt (Unterstützung für zukünftige Evolution)
6. ✅ API-Lesbarkeit und Wartbarkeit verbessert

Diese Verbesserungen machen die API standardmäßiger und benutzerfreundlicher, während die Kompatibilität mit bestehenden Systemen erhalten bleibt. Die Frontend-Oberfläche wurde erfolgreich zum neuen RESTful API-Design migriert, wobei alle API-Aufrufe das `/api/v1/`-Präfix und standardmäßigere Ressourcennamen verwenden. Dies verbessert die Code-Wartbarkeit und Erweiterbarkeit, während die vollständige Kompatibilität mit dem Backend erhalten bleibt.
