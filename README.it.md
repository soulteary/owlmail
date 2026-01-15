# OwlMail

> 🦉 Un'implementazione in Go di uno strumento di sviluppo e test per email, completamente compatibile con MailDev, che offre prestazioni migliori e funzionalità più ricche

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![MailDev Compatible](https://img.shields.io/badge/MailDev-Compatible-blue.svg)](https://github.com/maildev/maildev)
[![Go Report Card](https://goreportcard.com/badge/github.com/soulteary/owlmail)](https://goreportcard.com/report/github.com/soulteary/owlmail)
[![codecov](https://codecov.io/gh/soulteary/owlmail/graph/badge.svg?token=AY59NGM1FV)](https://codecov.io/gh/soulteary/owlmail)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](README.md) | [简体中文](README.zh-CN.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Italiano](README.it.md) | [日本語](README.ja.md) | [한국어](README.ko.md)

---

OwlMail è un server SMTP e un'interfaccia web per ambienti di sviluppo e test che cattura e visualizza tutte le email inviate. È un'implementazione in Go di [MailDev](https://github.com/maildev/maildev) con compatibilità API al 100%, offrendo prestazioni migliori, minore utilizzo di risorse e funzionalità più ricche.

![](.github/assets/owlmail-banner.jpg)

## ✨ Funzionalità

### Funzionalità Core

- ✅ **Server SMTP** - Riceve e memorizza tutte le email inviate (porta predefinita 1025)
- ✅ **Interfaccia Web** - Visualizza e gestisci le email tramite un browser (porta predefinita 1080)
- ✅ **Persistenza Email** - Le email vengono salvate come file `.eml`, supporta il caricamento da directory
- ✅ **Inoltro Email** - Supporta l'inoltro di email a server SMTP reali
- ✅ **Inoltro Automatico** - Supporta l'inoltro automatico di tutte le email con filtri basati su regole
- ✅ **Autenticazione SMTP** - Supporta autenticazione PLAIN/LOGIN
- ✅ **TLS/STARTTLS** - Supporta connessioni crittografate
- ✅ **SMTPS** - Supporta connessione TLS diretta sulla porta 465 (esclusivo OwlMail)

### Funzionalità Avanzate

- 🆕 **Operazioni Batch** - Eliminazione batch, segnatura batch come lette
- 🆕 **Statistiche Email** - Ottieni statistiche sulle email
- 🆕 **Anteprima Email** - API leggera per l'anteprima delle email
- 🆕 **Esportazione Email** - Esporta email come file ZIP
- 🆕 **API di Gestione Configurazione** - Gestione completa della configurazione (GET/PUT/PATCH)
- 🆕 **Ricerca Potente** - Ricerca full-text, filtri per intervallo di date, ordinamento
- 🆕 **API RESTful Migliorata** - Design API più standardizzato (`/api/v1/*`)

### Compatibilità

- ✅ **100% Compatibile con API MailDev** - Tutti gli endpoint API di MailDev sono supportati
- ✅ **Variabili d'Ambiente Completamente Compatibili** - Priorizza le variabili d'ambiente MailDev, nessuna modifica alla configurazione necessaria
- ✅ **Regole di Inoltro Automatico Compatibili** - Formato file di configurazione JSON completamente compatibile

### Vantaggi Prestazionali

- ⚡ **Binario Singolo** - Compilato come un singolo eseguibile, nessun runtime richiesto
- ⚡ **Basso Utilizzo di Risorse** - Compilato in Go, minore footprint di memoria
- ⚡ **Avvio Rapido** - Tempo di avvio più veloce
- ⚡ **Alta Concorrenza** - Goroutine Go, migliore prestazione concorrente

## 🚀 Quick Start

### Installazione

#### Compilazione da Sorgente

```bash
# Clona il repository
git clone https://github.com/soulteary/owlmail.git
cd owlmail

# Compila
go build -o owlmail ./cmd/owlmail

# Esegui
./owlmail
```

#### Installa con Go

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@latest
owlmail
```

### Utilizzo Base

```bash
# Avvia con configurazione predefinita (SMTP: 1025, Web: 1080)
./owlmail

# Porte personalizzate
./owlmail -smtp 1025 -web 1080

# Usa variabili d'ambiente
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
./owlmail
```

### Utilizzo Docker

#### Scarica da GitHub Container Registry (Consigliato)

Il modo più semplice per usare OwlMail è scaricare l'immagine pre-costruita da GitHub Container Registry:

```bash
# Scarica l'ultima immagine
docker pull ghcr.io/soulteary/owlmail:latest

# Scarica una versione specifica (usando il SHA del commit)
docker pull ghcr.io/soulteary/owlmail:sha-49b5f35

# Esegui container
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  ghcr.io/soulteary/owlmail:latest
```

**Tag disponibili:**
- `latest` - Ultima versione stabile
- `sha-<commit>` - SHA del commit specifico (es. `sha-49b5f35`)
- `main` - Ultima versione dal branch main

**Supporto multi-architettura:**
L'immagine supporta sia le architetture `linux/amd64` che `linux/arm64`. Docker scaricherà automaticamente l'immagine corretta per la tua piattaforma.

**Visualizza tutte le immagini disponibili:** [GitHub Packages](https://github.com/users/soulteary/packages/container/package/owlmail)

#### Costruisci dal sorgente

##### Build Base (Architettura Singola)

```bash
# Crea immagine per l'architettura corrente
docker build -t owlmail .

# Esegui container
docker run -d \
  -p 1025:1025 \
  -p 1080:1080 \
  --name owlmail \
  owlmail
```

##### Build Multi-Architettura

Per aarch64 (ARM64) o altre architetture, usa Docker Buildx:

```bash
# Abilita buildx (se non già abilitato)
docker buildx create --use --name multiarch-builder

# Compila per più architetture
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t owlmail:latest \
  --load .

# Oppure compila e invia al registry
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t your-registry/owlmail:latest \
  --push .

# Compila per architettura specifica (es. aarch64/arm64)
docker buildx build \
  --platform linux/arm64 \
  -t owlmail:latest \
  --load .
```

**Nota**: Il Dockerfile ora supporta build multi-architettura usando gli argomenti di build `TARGETOS` e `TARGETARCH`, che vengono impostati automaticamente da Docker Buildx.

## 📖 Opzioni di Configurazione

### Argomenti da Riga di Comando

| Argomento | Variabile d'Ambiente | Predefinito | Descrizione |
|-----------|---------------------|-------------|-------------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | Porta SMTP |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | Host SMTP |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Porta API Web |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Host API Web |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | Directory di archiviazione email |
| `-web-user` | `MAILDEV_WEB_USER` / `OWLMAIL_WEB_USER` | - | Nome utente HTTP Basic Auth |
| `-web-password` | `MAILDEV_WEB_PASS` / `OWLMAIL_WEB_PASSWORD` | - | Password HTTP Basic Auth |
| `-https` | `MAILDEV_HTTPS` / `OWLMAIL_HTTPS_ENABLED` | false | Abilita HTTPS |
| `-https-cert` | `MAILDEV_HTTPS_CERT` / `OWLMAIL_HTTPS_CERT` | - | File certificato HTTPS |
| `-https-key` | `MAILDEV_HTTPS_KEY` / `OWLMAIL_HTTPS_KEY` | - | File chiave privata HTTPS |
| `-outgoing-host` | `MAILDEV_OUTGOING_HOST` / `OWLMAIL_OUTGOING_HOST` | - | Host SMTP in uscita |
| `-outgoing-port` | `MAILDEV_OUTGOING_PORT` / `OWLMAIL_OUTGOING_PORT` | 587 | Porta SMTP in uscita |
| `-outgoing-user` | `MAILDEV_OUTGOING_USER` / `OWLMAIL_OUTGOING_USER` | - | Nome utente SMTP in uscita |
| `-outgoing-pass` | `MAILDEV_OUTGOING_PASS` / `OWLMAIL_OUTGOING_PASSWORD` | - | Password SMTP in uscita |
| `-outgoing-secure` | `MAILDEV_OUTGOING_SECURE` / `OWLMAIL_OUTGOING_SECURE` | false | TLS SMTP in uscita |
| `-auto-relay` | `MAILDEV_AUTO_RELAY` / `OWLMAIL_AUTO_RELAY` | false | Abilita inoltro automatico |
| `-auto-relay-addr` | `MAILDEV_AUTO_RELAY_ADDR` / `OWLMAIL_AUTO_RELAY_ADDR` | - | Indirizzo inoltro automatico |
| `-auto-relay-rules` | `MAILDEV_AUTO_RELAY_RULES` / `OWLMAIL_AUTO_RELAY_RULES` | - | File regole inoltro automatico |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | Nome utente autenticazione SMTP |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | Password autenticazione SMTP |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | Abilita TLS SMTP |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | File certificato TLS SMTP |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | File chiave privata TLS SMTP |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | Livello di log |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | Usa UUID per ID email (predefinito: stringa casuale di 8 caratteri) |

### Compatibilità Variabili d'Ambiente

OwlMail **supporta completamente le variabili d'ambiente MailDev**, dando priorità alle variabili d'ambiente MailDev e utilizzando quelle OwlMail se non presenti. Ciò significa che puoi utilizzare la configurazione MailDev direttamente senza modifiche.

```bash
# Usa direttamente le variabili d'ambiente MailDev (consigliato)
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com
./owlmail

# Oppure usa le variabili d'ambiente OwlMail
export OWLMAIL_SMTP_PORT=1025
export OWLMAIL_WEB_PORT=1080
./owlmail
```

## 📡 Documentazione API

### Formato Risposta API

OwlMail utilizza un formato di risposta API standardizzato:

**Risposta di Successo:**
```json
{
  "code": "EMAIL_DELETED",
  "message": "Email deleted",
  "data": { ... }
}
```

**Risposta di Errore:**
```json
{
  "code": "EMAIL_NOT_FOUND",
  "error": "EMAIL_NOT_FOUND",
  "message": "Email not found"
}
```

Il campo `code` contiene codici di errore/successo standardizzati che possono essere utilizzati per l'internazionalizzazione. Il campo `message` fornisce testo in inglese per la compatibilità con le versioni precedenti.

### Formato ID Email

OwlMail supporta due formati di ID email e tutti gli endpoint API sono compatibili con entrambi:

- **Stringa casuale di 8 caratteri**: Formato predefinito, es. `aB3dEfGh`
- **Formato UUID**: UUID standard di 36 caratteri, es. `550e8400-e29b-41d4-a716-446655440000`

Quando usi il parametro `:id` nelle richieste API, puoi usare entrambi i formati. Ad esempio:
- `GET /email/aB3dEfGh` - Usando ID stringa casuale
- `GET /email/550e8400-e29b-41d4-a716-446655440000` - Usando ID UUID

### API Compatibile MailDev

OwlMail è completamente compatibile con tutti gli endpoint API di MailDev:

#### Operazioni Email

- `GET /email` - Ottieni tutte le email (supporta paginazione e filtri)
  - Parametri di query:
    - `limit` (predefinito: 50, max: 1000) - Numero di email da restituire
    - `offset` (predefinito: 0) - Numero di email da saltare
    - `q` - Query di ricerca full-text
    - `from` - Filtra per indirizzo email mittente
    - `to` - Filtra per indirizzo email destinatario
    - `dateFrom` - Filtra per data da (formato YYYY-MM-DD)
    - `dateTo` - Filtra per data fino a (formato YYYY-MM-DD)
    - `read` - Filtra per stato di lettura (true/false)
    - `sortBy` - Ordina per campo (time, subject)
    - `sortOrder` - Ordine di ordinamento (asc, desc, predefinito: desc)
  - Esempio: `GET /email?limit=20&offset=0&q=test&sortBy=time&sortOrder=desc`
- `GET /email/:id` - Ottieni singola email
- `DELETE /email/:id` - Elimina singola email
- `DELETE /email/all` - Elimina tutte le email
- `PATCH /email/read-all` - Segna tutte le email come lette
- `PATCH /email/:id/read` - Segna singola email come letta

#### Contenuto Email

- `GET /email/:id/html` - Ottieni contenuto HTML email
- `GET /email/:id/attachment/:filename` - Scarica allegato
- `GET /email/:id/download` - Scarica file .eml grezzo
- `GET /email/:id/source` - Ottieni sorgente grezza email

#### Inoltro Email

- `POST /email/:id/relay` - Inoltra email al server SMTP configurato
- `POST /email/:id/relay/:relayTo` - Inoltra email a indirizzo specifico

#### Configurazione e Sistema

- `GET /config` - Ottieni informazioni di configurazione
- `GET /healthz` - Controllo salute
- `GET /reloadMailsFromDirectory` - Ricarica email da directory
- `GET /socket.io` - Connessione WebSocket (WebSocket standard, non Socket.IO)

### API Avanzata OwlMail

#### Statistiche e Anteprima Email

- `GET /email/stats` - Ottieni statistiche email
- `GET /email/preview` - Ottieni anteprima email (leggera)

#### Operazioni Batch

- `POST /email/batch/delete` - Elimina email in batch
- `POST /email/batch/read` - Segna come lette in batch

#### Esportazione Email

- `GET /email/export` - Esporta email come file ZIP

#### Gestione Configurazione

- `GET /config/outgoing` - Ottieni configurazione in uscita
- `PUT /config/outgoing` - Aggiorna configurazione in uscita
- `PATCH /config/outgoing` - Aggiorna parzialmente configurazione in uscita

### API RESTful Migliorata (`/api/v1/*`)

OwlMail fornisce un design API RESTful più standardizzato:

- `GET /api/v1/emails` - Ottieni tutte le email (risorsa plurale)
  - Parametri di query: Stessi di `GET /email` (limit, offset, q, from, to, dateFrom, dateTo, read, sortBy, sortOrder)
  - Esempio: `GET /api/v1/emails?limit=20&offset=0&q=test&sortBy=time&sortOrder=desc`
- `GET /api/v1/emails/:id` - Ottieni singola email
- `DELETE /api/v1/emails/:id` - Elimina singola email
- `DELETE /api/v1/emails` - Elimina tutte le email
- `DELETE /api/v1/emails/batch` - Eliminazione batch
- `PATCH /api/v1/emails/read` - Segna tutte le email come lette
- `PATCH /api/v1/emails/:id/read` - Segna singola email come letta
- `PATCH /api/v1/emails/batch/read` - Segna come lette in batch
- `GET /api/v1/emails/stats` - Statistiche email
- `GET /api/v1/emails/preview` - Anteprima email
- `GET /api/v1/emails/export` - Esporta email
- `POST /api/v1/emails/reload` - Ricarica email
- `GET /api/v1/settings` - Ottieni tutte le impostazioni
- `GET /api/v1/settings/outgoing` - Ottieni configurazione in uscita
- `PUT /api/v1/settings/outgoing` - Aggiorna configurazione in uscita
- `PATCH /api/v1/settings/outgoing` - Aggiorna parzialmente configurazione in uscita
- `GET /api/v1/health` - Controllo salute
- `GET /api/v1/ws` - Connessione WebSocket

Per documentazione API dettagliata, vedi: [Registro Refactoring API](./docs/it/internal/API_Refactoring_Record.md)

## 🔧 Esempi di Utilizzo

### Utilizzo Base

```bash
# Avvia OwlMail
./owlmail -smtp 1025 -web 1080

# Configura SMTP nella tua applicazione
SMTP_HOST=localhost
SMTP_PORT=1025
```

### Configura Inoltro Email

```bash
# Inoltra a SMTP Gmail
./owlmail \
  -outgoing-host smtp.gmail.com \
  -outgoing-port 587 \
  -outgoing-user your-email@gmail.com \
  -outgoing-pass your-password \
  -outgoing-secure
```

### Modalità Inoltro Automatico

```bash
# Crea file regole inoltro automatico (relay-rules.json)
cat > relay-rules.json <<EOF
[
  { "allow": "*" },
  { "deny": "*@test.com" },
  { "allow": "ok@test.com" }
]
EOF

# Avvia inoltro automatico
./owlmail \
  -outgoing-host smtp.gmail.com \
  -outgoing-port 587 \
  -outgoing-user your-email@gmail.com \
  -outgoing-pass your-password \
  -auto-relay \
  -auto-relay-rules relay-rules.json
```

### Usa HTTPS

```bash
./owlmail \
  -https \
  -https-cert /path/to/cert.pem \
  -https-key /path/to/key.pem \
  -web 1080
```

### Usa Autenticazione SMTP

```bash
./owlmail \
  -smtp-user admin \
  -smtp-password secret \
  -smtp 1025
```

### Usa TLS

```bash
./owlmail \
  -tls \
  -tls-cert /path/to/cert.pem \
  -tls-key /path/to/key.pem \
  -smtp 1025
```

**Nota**: Quando TLS è abilitato, OwlMail avvia automaticamente un server SMTPS sulla porta 465 oltre al server SMTP regolare. Il server SMTPS utilizza una connessione TLS diretta (non è richiesto STARTTLS). Questa è una funzionalità esclusiva di OwlMail.

### Usa UUID per ID Email

OwlMail supporta due formati di ID email:

1. **Formato predefinito**: Stringa casuale di 8 caratteri (es. `aB3dEfGh`)
2. **Formato UUID**: UUID standard di 36 caratteri (es. `550e8400-e29b-41d4-a716-446655440000`)

L'uso del formato UUID fornisce migliore unicità e tracciabilità, particolarmente utile per l'integrazione con sistemi esterni.

```bash
# Abilita UUID usando flag da riga di comando
./owlmail -use-uuid-for-email-id

# Abilita UUID usando variabile d'ambiente
export OWLMAIL_USE_UUID_FOR_EMAIL_ID=true
./owlmail

# Usa con altre configurazioni
./owlmail \
  -use-uuid-for-email-id \
  -smtp 1025 \
  -web 1080
```

**Note**:
- Il predefinito usa stringa casuale di 8 caratteri, compatibile con il comportamento MailDev
- Quando UUID è abilitato, tutte le email appena ricevute useranno ID in formato UUID
- L'API supporta entrambi i formati ID, permettendo normale query, eliminazione e operazione delle email
- I formati ID email esistenti non cambieranno; solo le nuove email useranno il nuovo formato ID

## 🔄 Migrazione da MailDev

OwlMail è completamente compatibile con MailDev e può essere usato come sostituto diretto:

### 1. Compatibilità Variabili d'Ambiente

OwlMail dà priorità alle variabili d'ambiente MailDev, nessuna modifica alla configurazione necessaria:

```bash
# Configurazione MailDev
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com

# Usa OwlMail direttamente (nessun bisogno di cambiare variabili d'ambiente)
./owlmail
```

### 2. Compatibilità API

Tutti gli endpoint API MailDev sono supportati, il codice client esistente non richiede modifiche:

```bash
# API MailDev
curl http://localhost:1080/email

# OwlMail completamente compatibile
curl http://localhost:1080/email
```

### 3. Adattamento WebSocket

Se usi WebSocket, devi cambiare da Socket.IO a WebSocket standard:

```javascript
// MailDev (Socket.IO)
const socket = io('/socket.io');
socket.on('newMail', (email) => { /* ... */ });

// OwlMail (WebSocket Standard)
const ws = new WebSocket('ws://localhost:1080/socket.io');
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'new') { /* ... */ }
};
```

Per guida di migrazione dettagliata, vedi: [OwlMail × MailDev: Confronto Completo Funzionalità e API e White Paper di Migrazione](./docs/it/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

## 🧪 Test

```bash
# Esegui tutti i test
go test ./...

# Esegui test con copertura
go test -cover ./...

# Esegui test per pacchetti specifici
go test ./internal/api/...
go test ./internal/mailserver/...
```

## 📦 Struttura Progetto

```
OwlMail/
├── cmd/
│   └── owlmail/          # Punto di ingresso programma principale
├── internal/
│   ├── api/              # Implementazione API Web
│   ├── common/           # Utility comuni (logging, gestione errori)
│   ├── maildev/          # Livello compatibilità MailDev
│   ├── mailserver/       # Implementazione server SMTP
│   ├── outgoing/         # Implementazione inoltro email
│   └── types/            # Definizioni di tipo
├── web/                  # File frontend web
├── go.mod                # Definizione modulo Go
└── README.md             # Questo documento
```

## 🤝 Contribuire

I contributi sono benvenuti! Segui questi passaggi:

1. Fai un fork del repository
2. Crea un branch per la funzionalità (`git checkout -b feature/AmazingFeature`)
3. Committa le tue modifiche (`git commit -m 'Add some AmazingFeature'`)
4. Invia al branch (`git push origin feature/AmazingFeature`)
5. Apri una Pull Request

## 📄 Licenza

Questo progetto è concesso in licenza sotto la Licenza MIT - vedi il file [LICENSE](LICENSE) per i dettagli.

## 🙏 Ringraziamenti

- [MailDev](https://github.com/maildev/maildev) - Ispirazione progetto originale
- [emersion/go-smtp](https://github.com/emersion/go-smtp) - Libreria server SMTP
- [emersion/go-message](https://github.com/emersion/go-message) - Libreria parsing email
- [Gin](https://github.com/gin-gonic/gin) - Framework web
- [gorilla/websocket](https://github.com/gorilla/websocket) - Libreria WebSocket

## 📚 Documentazione Correlata

- [OwlMail × MailDev: Confronto Completo Funzionalità e API e White Paper di Migrazione](./docs/it/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [Registro Refactoring API](./docs/it/internal/API_Refactoring_Record.md)

## 🐛 Segnalazione Problemi

Se riscontri problemi o hai suggerimenti, inviali in [GitHub Issues](https://github.com/soulteary/owlmail/issues).

## ⭐ Storia Star

Se questo progetto ti aiuta, per favore dagli una Star ⭐!

---

**OwlMail** - Un'implementazione in Go di uno strumento di sviluppo e test per email, completamente compatibile con MailDev 🦉
