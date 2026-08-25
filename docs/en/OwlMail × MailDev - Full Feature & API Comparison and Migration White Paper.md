# OwlMail × MailDev: Full Feature & API Comparison and Migration White Paper

> **A Deep Source-Level Comparison + Migration Guide for Users and Developers**

---

## 📋 Executive Summary

After a systematic review of both source codes and APIs, **OwlMail (Go)** and **MailDev (Node.js)** are fully equivalent in their **core functionalities and compatible APIs**.
On top of that, OwlMail provides **stronger RESTful design, batch operations, statistics, export tools, and native SMTPS (465)** support — all delivered as a **single binary**, resulting in superior performance and deployment simplicity.

**Key Conclusions**

* ✅ **API Compatibility: 100%** — Covers all MailDev endpoints
* ✅ **Feature Parity: 100%** — Send / View / Delete / Relay, all core features equivalent
* ✅ **Environment Variable Compatibility: 100%** — Recognizes MailDev variables first for seamless migration
* ✅ **Enhanced Capabilities** — Batch operations, statistics, export, improved REST, SMTPS (465)
* ⚠️ **WebSocket Protocol Difference** — MailDev uses Socket.IO; OwlMail uses native WS; same semantics, minor client adaptation required

**Replaceability Conclusion:**
In nearly all scenarios, OwlMail can directly replace MailDev. Only WebSocket clients require slight adjustment from Socket.IO to standard WebSocket.

---

## 📖 Project Overview & Tech Stack

| Project     | Language | Version | Description                                                           |
| ----------- | -------- | ------- | --------------------------------------------------------------------- |
| **OwlMail** | Go       | 1.0+    | MailDev-compatible email dev/testing tool with extensive enhancements |
| **MailDev** | Node.js  | 2.2.1   | Classic email development/testing tool with a mature frontend         |

**Technical Stack Comparison**

| Aspect             | OwlMail                       | MailDev          |
| ------------------ | ----------------------------- | ---------------- |
| Language / Runtime | Go 1.26.6+ (single binary)    | Node.js ≥ 18     |
| Web Framework      | Fiber                         | Express          |
| SMTP Library       | emersion/go-smtp              | smtp-server      |
| Email Parser       | emersion/go-message           | mailparser-mit   |
| WebSocket          | gorilla/websocket (native WS) | Socket.IO        |
| HTML Sanitization  | bluemonday                    | DOMPurify        |
| Frontend           | Native JS (lightweight)       | AngularJS (full) |

---

## 🔍 Compatible API Endpoints (100% Coverage)

> Listed according to MailDev’s original routes.
> OwlMail also provides improved REST routes under **/api/v1/** (see below).

| Endpoint                          | Method | MailDev | OwlMail | Compatibility Notes                                                                                                                                  |
| --------------------------------- | ------ | ------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/email`                          | GET    | ✅       | ✅       | Supports `skip` (alias `offset`); adds `limit/q/from/to/dateFrom/dateTo/read/sort*`; returns `{total, limit, offset, emails}` for RESTful structure. |
| `/email/:id`                      | GET    | ✅       | ✅       | Same function; MailDev marks read on fetch, OwlMail uses separate `PATCH /email/:id/read`.                                                           |
| `/email/:id/html`                 | GET    | ✅       | ✅       | Both return HTML; OwlMail adds explicit `Content-Type`; different base URL handling (no impact).                                                     |
| `/email/:id/attachment/:filename` | GET    | ✅       | ✅       | Both support attachment download with correct MIME types.                                                                                            |
| `/email/:id/download`             | GET    | ✅       | ✅       | Both export `.eml`; OwlMail generates cleaner filenames.                                                                                             |
| `/email/:id/source`               | GET    | ✅       | ✅       | Returns raw source (text stream in OwlMail).                                                                                                         |
| `/email/:id`                      | DELETE | ✅       | ✅       | MailDev often returns 500 on missing ID; OwlMail returns 404 (more RESTful).                                                                         |
| `/email/all`                      | DELETE | ✅       | ✅       | Both clear mailbox.                                                                                                                                  |
| `/email/read-all`                 | PATCH  | ✅       | ✅       | OwlMail returns `{message,count}`.                                                                                                                   |
| `/email/:id/relay/:relayTo?`      | POST   | ✅       | ✅       | Both support URL param; OwlMail also supports JSON body.                                                                                             |
| `/config`                         | GET    | ✅       | ✅       | OwlMail provides richer nested data; can flatten for compatibility.                                                                                  |
| `/healthz`                        | GET    | ✅       | ✅       | MailDev: `true`; OwlMail: `{status:"ok"}`.                                                                                                           |
| `/reloadMailsFromDirectory`       | GET    | ✅       | ✅       | Same behavior; OwlMail prefers semantic `POST`.                                                                                                      |
| `/socket.io`                      | WS     | ✅       | ✅       | Same path; different protocol (Socket.IO vs native WS); same event semantics.                                                                        |

**Source Code References**

* MailDev: `lib/routes.js`, `lib/mailserver.js`, `lib/outgoing.js`, `lib/web.js`
* OwlMail: `internal/api/api_*.go`, `internal/mailserver/*.go`, `internal/outgoing/*.go`

---

## 🆙 OwlMail Enhancements

### A. New or Improved APIs (not in MailDev)

| Endpoint              | Method        | Description                                      |
| --------------------- | ------------- | ------------------------------------------------ |
| `/email/:id/read`     | PATCH         | Mark email as read (MailDev auto-marks on fetch) |
| `/email/stats`        | GET           | Message statistics                               |
| `/email/preview`      | GET           | Lightweight preview list                         |
| `/email/batch/delete` | POST          | Bulk delete                                      |
| `/email/batch/read`   | POST          | Bulk mark-read                                   |
| `/email/export`       | GET           | Export all emails as ZIP                         |
| `/config/outgoing`    | GET/PUT/PATCH | Manage outgoing relay config                     |

### B. Improved RESTful Routes (`/api/v1/*`)

Plural resources, action semantics, and versioning:

* `/api/v1/emails[/:id]`, `/api/v1/emails/batch`, `/api/v1/emails/:id/actions/relay`, `/api/v1/emails/stats|preview|export`, `/api/v1/settings*`, `/api/v1/health`, `/api/v1/ws`, etc.

### C. SMTP / Security Enhancements

* **SMTPS (465)**: native support (absent in MailDev).
* Fine-grained TLS/STARTTLS configuration and authentication.

---

## 🔧 Implementation & Difference Highlights

### 1) SMTP / Incoming Mail

| Feature                    | MailDev         | OwlMail       | Notes               |
| -------------------------- | --------------- | ------------- | ------------------- |
| SMTP Server                | ✅ `smtp-server` | ✅ `go-smtp`   | Equivalent          |
| Ports / Bind / Persistence | ✅               | ✅             | `.eml` file storage |
| Load from Directory        | ✅               | ✅             | Same                |
| SMTP Auth                  | ✅ PLAIN/LOGIN   | ✅ PLAIN/LOGIN | Same                |
| TLS/STARTTLS               | ✅               | ✅             | Same                |
| **SMTPS (465)**            | ❌               | ✅             | **OwlMail only**    |

### 2) Filtering / Search / Pagination

* **MailDev:** Dot-syntax (`from.address=value`), `skip` offset.
* **OwlMail:** Extends with `q/from/to/dateFrom/dateTo/read/limit/offset/sortBy/sortOrder`; returns `{total, limit, offset}`.

### 3) Read Semantics

* MailDev: `GET /email/:id` marks as read.
* OwlMail: explicit `PATCH /email/:id/read`.
* Impact: minimal — can align via one extra call.

### 4) Status Codes & Responses

* Non-existent delete → MailDev: 500; OwlMail: 404.
* MailDev often returns `true`/number; OwlMail returns structured JSON (`{message, count}`).

### 5) WebSocket Protocol

* MailDev: Socket.IO (auto-reconnect, rooms, etc.).
* OwlMail: native WebSocket (`{type:"new"|"delete", email}`).
* Compatibility: event semantics identical; client changes required.

---

## 🌱 Environment Variable Compatibility

> OwlMail fully maps all MailDev environment variables.
> If a MailDev variable is missing, it falls back to its own prefixed form.

| MailDev Variable           | OwlMail Variable            | Description            |
| -------------------------- | --------------------------- | ---------------------- |
| `MAILDEV_SMTP_PORT`        | `OWLMAIL_SMTP_PORT`         | SMTP Port              |
| `MAILDEV_IP`               | `OWLMAIL_SMTP_HOST`         | SMTP Host              |
| `MAILDEV_MAIL_DIRECTORY`   | `OWLMAIL_MAIL_DIR`          | Mail Directory         |
| `MAILDEV_WEB_PORT`         | `OWLMAIL_WEB_PORT`          | Web Port               |
| `MAILDEV_WEB_IP`           | `OWLMAIL_WEB_HOST`          | Web Host               |
| `MAILDEV_WEB_USER`         | `OWLMAIL_WEB_USER`          | Web Auth User          |
| `MAILDEV_WEB_PASS`         | `OWLMAIL_WEB_PASSWORD`      | Web Auth Password      |
| `MAILDEV_HTTPS`            | `OWLMAIL_HTTPS_ENABLED`     | Enable HTTPS           |
| `MAILDEV_HTTPS_CERT`       | `OWLMAIL_HTTPS_CERT`        | HTTPS Certificate      |
| `MAILDEV_HTTPS_KEY`        | `OWLMAIL_HTTPS_KEY`         | HTTPS Private Key      |
| `MAILDEV_OUTGOING_HOST`    | `OWLMAIL_OUTGOING_HOST`     | Outgoing SMTP Host     |
| `MAILDEV_OUTGOING_PORT`    | `OWLMAIL_OUTGOING_PORT`     | Outgoing SMTP Port     |
| `MAILDEV_OUTGOING_USER`    | `OWLMAIL_OUTGOING_USER`     | Outgoing SMTP User     |
| `MAILDEV_OUTGOING_PASS`    | `OWLMAIL_OUTGOING_PASSWORD` | Outgoing SMTP Password |
| `MAILDEV_OUTGOING_SECURE`  | `OWLMAIL_OUTGOING_SECURE`   | Outgoing TLS           |
| `MAILDEV_AUTO_RELAY`       | `OWLMAIL_AUTO_RELAY`        | Auto Relay Switch      |
| `MAILDEV_AUTO_RELAY_ADDR`  | `OWLMAIL_AUTO_RELAY_ADDR`   | Auto Relay Address     |
| `MAILDEV_AUTO_RELAY_RULES` | `OWLMAIL_AUTO_RELAY_RULES`  | Auto Relay Rules       |
| `MAILDEV_INCOMING_USER`    | `OWLMAIL_SMTP_USER`         | Incoming SMTP User     |
| `MAILDEV_INCOMING_PASS`    | `OWLMAIL_SMTP_PASSWORD`     | Incoming SMTP Password |
| `MAILDEV_INCOMING_SECURE`  | `OWLMAIL_TLS_ENABLED`       | Incoming TLS Switch    |
| `MAILDEV_INCOMING_CERT`    | `OWLMAIL_TLS_CERT`          | Incoming TLS Cert      |
| `MAILDEV_INCOMING_KEY`     | `OWLMAIL_TLS_KEY`           | Incoming TLS Key       |
| `MAILDEV_VERBOSE`          | `OWLMAIL_LOG_LEVEL=verbose` | Verbose Logging        |
| `MAILDEV_SILENT`           | `OWLMAIL_LOG_LEVEL=silent`  | Silent Logging         |

**Example**

```bash
# Use existing MailDev vars (zero migration)
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com
./owlmail

# or fallback to native OwlMail vars
export OWLMAIL_SMTP_PORT=1025
export OWLMAIL_WEB_PORT=1080
./owlmail
```

---

## 📊 Performance & Deployment

| Metric                | OwlMail      | MailDev   | Notes                        |
| --------------------- | ------------ | --------- | ---------------------------- |
| Startup Speed         | ⚡ Fast       | 🐢 Slower | Binary vs Node runtime       |
| Memory Usage          | 💚 Low       | 🟡 Medium | Go runtime footprint smaller |
| Concurrency           | 💚 Excellent | 🟡 Good   | Goroutines advantage         |
| Resource Usage        | 💚 Low       | 🟡 Medium | Single executable            |
| Deployment Simplicity | 💚 High      | 🟡 Medium | No runtime dependencies      |

---

## 🔄 Migration Strategies (with Test Checklist)

### Option 1｜**Full Replacement (Recommended)**

**Use Case:** Primarily API/CLI driven; WebSocket clients can adapt.

1. Stop MailDev; start OwlMail with **same environment vars**.
2. Verify endpoints: `/email`, `/healthz`, downloads, attachments, sources.
3. Update WebSocket client: Socket.IO → native **WS**.

### Option 2｜Progressive Replacement

**Use Case:** Need MailDev’s original frontend.

1. Reuse MailDev static frontend; switch backend to OwlMail.
2. Change WS connection only (`io()` → `new WebSocket()`).

### Option 3｜Hybrid Mode

**Use Case:** Retain MailDev UI, use OwlMail backend APIs.

### Sample Test Checklist

```bash
# API smoke tests
curl -s http://localhost:1080/healthz
curl -s http://localhost:1080/email
curl -s http://localhost:1080/email/:id
curl -s http://localhost:1080/email/:id/html
curl -s http://localhost:1080/email/:id/download

# Env var compatibility
MAILDEV_SMTP_PORT=1025 MAILDEV_WEB_PORT=1080 ./owlmail

# Send test mail
echo "Test" | mail -s "Test" test@localhost

# or via sendmail
sendmail -S localhost:1025 test@localhost <<'EOF'
Subject: Test Email
From: sender@example.com
To: test@localhost

This is a test email.
EOF

# Relay verification
EMAIL_ID=$(curl -s http://localhost:1080/email | jq -r '.[0].id // .emails[0].id')
curl -X POST "http://localhost:1080/email/${EMAIL_ID}/relay/recipient@example.com"
```

**WebSocket Client Adaptation**

```js
// MailDev (Socket.IO)
const socket = io('/socket.io');
socket.on('newMail', (email) => { /* ... */ });
socket.on('deleteMail', (data) => { /* ... */ });

// OwlMail (Native WS)
const ws = new WebSocket('ws://localhost:1080/socket.io');
ws.onmessage = (ev) => {
  const data = JSON.parse(ev.data);
  if (data.type === 'new') { /* ... */ }
  if (data.type === 'delete') { /* ... */ }
};
```

---

## 🧩 Relay Rules & Auto-Forwarding

* **Rule Format:** Identical JSON schema; **last match wins**; supports `*` wildcards.

```json
[
  { "allow": "*" },
  { "deny": "*@test.com" },
  { "allow": "ok@test.com" }
]
```

* **Feature Parity:** outbound config, authentication, TLS, address-based relay, and auto relay are all equivalent.
  OwlMail additionally supports passing `relayTo` via request body.

---

## 📝 Compatibility Matrix (Summary)

### API Endpoints

| Endpoint                                            | Compatibility | Notes                                       |
| --------------------------------------------------- | ------------- | ------------------------------------------- |
| GET `/email`                                        | ⭐⭐⭐⭐⭐         | OwlMail adds pagination metadata            |
| GET `/email/:id`                                    | ⭐⭐⭐⭐          | MailDev auto-reads; OwlMail explicit PATCH  |
| DELETE `/email/:id`                                 | ⭐⭐⭐⭐          | Slight status/response difference           |
| DELETE `/email/all`                                 | ⭐⭐⭐⭐⭐         | Equivalent                                  |
| PATCH `/email/read-all`                             | ⭐⭐⭐⭐⭐         | Equivalent                                  |
| GET `/email/:id/html, attachment, download, source` | ⭐⭐⭐⭐⭐         | Equivalent or improved                      |
| POST `/email/:id/relay/:relayTo?`                   | ⭐⭐⭐⭐⭐         | Equivalent; OwlMail adds JSON body          |
| GET `/config`                                       | ⭐⭐⭐⭐          | OwlMail structured (adapter layer possible) |
| GET `/healthz`                                      | ⭐⭐⭐⭐          | Different return format                     |
| GET `/reloadMailsFromDirectory`                     | ⭐⭐⭐⭐⭐         | Equivalent (prefers POST)                   |
| `WS /socket.io`                                     | ⭐⭐⭐           | Protocol differs, events equivalent         |

### Functional Comparison

| Feature                                | Compatibility | Notes                            |
| -------------------------------------- | ------------- | -------------------------------- |
| SMTP / Storage / Parsing / Attachments | ⭐⭐⭐⭐⭐         | Equivalent                       |
| Auto-relay / Forwarding                | ⭐⭐⭐⭐⭐         | Equivalent                       |
| Auth / TLS / HTTPS                     | ⭐⭐⭐⭐⭐         | Equivalent                       |
| **SMTPS (465)**                        | 🆕            | OwlMail only                     |
| Batch / Stats / Export / REST+         | 🆕            | OwlMail enhancements             |
| WebSocket                              | ⚠️            | Protocol differs, same semantics |

---

## 🎯 Recommendations

**Prefer OwlMail** — best suited for API-driven, high-performance, and production-grade email dev/testing environments.
If a project heavily relies on MailDev’s Socket.IO frontend, use a **progressive or hybrid migration** approach and adapt WS clients incrementally.

---

## 📚 Source Reference Index

* **MailDev**

  * `lib/routes.js` — REST routes
  * `lib/mailserver.js` — SMTP & parsing
  * `lib/outgoing.js` — relay logic
  * `lib/web.js` — Socket.IO

* **OwlMail**

  * `internal/api/api_emails.go`, `api_config.go`, `api_relay.go`, `api_websocket.go`
  * `internal/mailserver/session.go`, `store.go`
  * `internal/outgoing/outgoing.go`
  * Improved REST routes in `/api/v1/*` via `setupImprovedAPIRoutes()`

---

## 🏁 Final Conclusion

* **Feature Parity: ⭐⭐⭐⭐⭐** — 100% alignment on all core capabilities; OwlMail provides numerous enhancements.
* **Replaceability: ⭐⭐⭐⭐⭐** — Works as a drop-in replacement; only WS clients need light adaptation.
* **Env Var Compatibility: ⭐⭐⭐⭐⭐** — Zero-change migration from existing MailDev setups.
* **Added Value:** Superior performance, SMTPS, batch/analytics/export, and cleaner REST design greatly improve engineering usability.

---

**Report Date:** November 10, 2025
**OwlMail Version:** 1.0+
**MailDev Version:** 2.2.1
