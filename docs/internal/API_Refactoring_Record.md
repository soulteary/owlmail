# API Refactoring Record

## Overview

This document records the API refactoring process of OwlMail, documenting the migration from MailDev-compatible API endpoints to a new RESTful API design (`/api/v1/`). The refactoring maintains full backward compatibility while introducing improved API design patterns.

## Refactoring Objectives

The refactoring was initiated to address several API design issues:

1. **Inconsistent resource naming** - Using singular `/email` instead of plural `/emails`
2. **Non-standard RESTful design** - Using `/all` suffixes and improper HTTP methods
3. **Unclear action naming** - Actions not explicitly identified
4. **Inconsistent sub-resource naming** - Using singular `attachment` instead of plural
5. **Less semantic path naming** - Using generic terms like `/download` instead of `/raw`
6. **Missing API versioning** - No version prefix for future API evolution
7. **Improper HTTP method usage** - Using GET for state-changing operations

## API Design Issues Analysis

### 1. Inconsistent Resource Naming

**Problem:**
- Using singular `/email` instead of plural `/emails`
- RESTful best practices recommend using plural forms for resource collections

**Example:**
```
❌ GET /email          (singular)
✅ GET /emails         (plural)
```

### 2. Non-standard RESTful Design

**Problem:**
- `DELETE /email/all` - Using `/all` suffix is not RESTful
- `POST /email/batch/delete` - Using POST for delete operations is not semantic
- `PATCH /email/read-all` - Using hyphenated naming is not clear

**Improvement:**
```
❌ DELETE /email/all
✅ DELETE /emails

❌ POST /email/batch/delete
✅ DELETE /emails/batch

❌ PATCH /email/read-all
✅ PATCH /emails/read
```

### 3. Unclear Action Naming

**Problem:**
- `/email/:id/relay` - Action is not explicit
- Should clearly indicate this is an action operation

**Improvement:**
```
❌ POST /email/:id/relay
✅ POST /emails/:id/actions/relay
```

### 4. Inconsistent Sub-resource Naming

**Problem:**
- `/email/:id/attachment/:filename` - Using singular `attachment`
- Should use plural `attachments` to represent resource collection

**Improvement:**
```
❌ GET /email/:id/attachment/:filename
✅ GET /emails/:id/attachments/:filename
```

### 5. Less Semantic Path Naming

**Problem:**
- `/email/:id/download` - `download` is not semantic enough
- `/config` - `config` is less semantic than `settings`
- `/healthz` - Non-standard naming
- `/reloadMailsFromDirectory` - CamelCase naming doesn't conform to RESTful style

**Improvement:**
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

### 6. Improper HTTP Method Usage

**Problem:**
- `GET /reloadMailsFromDirectory` - Reloading is a state-changing operation, should use POST
- `POST /email/batch/delete` - Delete operations should use DELETE

**Improvement:**
```
❌ GET /reloadMailsFromDirectory
✅ POST /emails/reload

❌ POST /email/batch/delete
✅ DELETE /emails/batch
```

### 7. Missing API Versioning

**Problem:**
- No API version prefix
- Unable to evolve API versions

**Improvement:**
```
❌ GET /email
✅ GET /api/v1/emails
```

## Refactored API Design

### MailDev-Compatible API (Maintained for Backward Compatibility)

All original MailDev API endpoints are preserved to ensure backward compatibility:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/email` | Get all emails |
| GET | `/email/:id` | Get single email |
| GET | `/email/:id/html` | Get email HTML |
| GET | `/email/:id/attachment/:filename` | Download attachment |
| GET | `/email/:id/download` | Download original .eml file |
| GET | `/email/:id/source` | Get email raw source |
| DELETE | `/email/:id` | Delete single email |
| DELETE | `/email/all` | Delete all emails |
| PATCH | `/email/read-all` | Mark all emails as read |
| PATCH | `/email/:id/read` | Mark single email as read |
| POST | `/email/:id/relay` | Relay email |
| POST | `/email/:id/relay/:relayTo` | Relay email to specified address |
| GET | `/email/stats` | Email statistics |
| GET | `/email/preview` | Email preview |
| POST | `/email/batch/delete` | Batch delete |
| POST | `/email/batch/read` | Batch mark as read |
| GET | `/email/export` | Export emails |
| GET | `/config` | Get configuration |
| GET | `/config/outgoing` | Get outgoing configuration |
| PUT | `/config/outgoing` | Update outgoing configuration |
| PATCH | `/config/outgoing` | Partially update outgoing configuration |
| GET | `/healthz` | Health check |
| GET | `/reloadMailsFromDirectory` | Reload emails |
| GET | `/socket.io` | WebSocket connection |

### New Improved API (Recommended)

#### Email Resources (`/api/v1/emails`)

| Method | Path | Description | Improvement |
|--------|------|-------------|-------------|
| GET | `/api/v1/emails` | Get all emails | Use plural resource |
| GET | `/api/v1/emails/:id` | Get single email | Use plural resource |
| DELETE | `/api/v1/emails/:id` | Delete single email | Use plural resource |
| DELETE | `/api/v1/emails` | Delete all emails | More RESTful, no `/all` suffix |
| DELETE | `/api/v1/emails/batch` | Batch delete | Use DELETE instead of POST |
| PATCH | `/api/v1/emails/read` | Mark all emails as read | Clearer naming |
| PATCH | `/api/v1/emails/:id/read` | Mark single email as read | Use plural resource |
| PATCH | `/api/v1/emails/batch/read` | Batch mark as read | Use plural resource |
| GET | `/api/v1/emails/stats` | Email statistics | Use plural resource |
| GET | `/api/v1/emails/preview` | Email preview | Use plural resource |
| GET | `/api/v1/emails/export` | Export emails | Use plural resource |
| POST | `/api/v1/emails/reload` | Reload emails | Use POST instead of GET |

#### Email Content Resources

| Method | Path | Description | Improvement |
|--------|------|-------------|-------------|
| GET | `/api/v1/emails/:id/html` | Get email HTML | Use plural resource |
| GET | `/api/v1/emails/:id/source` | Get email source | Use plural resource |
| GET | `/api/v1/emails/:id/raw` | Get raw email | More semantic naming (replaces `/download`) |
| GET | `/api/v1/emails/:id/attachments/:filename` | Download attachment | Use plural `attachments` |

#### Email Actions

| Method | Path | Description | Improvement |
|--------|------|-------------|-------------|
| POST | `/api/v1/emails/:id/actions/relay` | Relay email | Explicitly indicates action operation |
| POST | `/api/v1/emails/:id/actions/relay/:relayTo` | Relay email to specified address | Explicitly indicates action operation |

#### Settings Resources (`/api/v1/settings`)

| Method | Path | Description | Improvement |
|--------|------|-------------|-------------|
| GET | `/api/v1/settings` | Get all settings | More semantic naming (replaces `/config`) |
| GET | `/api/v1/settings/outgoing` | Get outgoing configuration | More semantic naming |
| PUT | `/api/v1/settings/outgoing` | Update outgoing configuration | More semantic naming |
| PATCH | `/api/v1/settings/outgoing` | Partially update outgoing configuration | More semantic naming |

#### System Resources

| Method | Path | Description | Improvement |
|--------|------|-------------|-------------|
| GET | `/api/v1/health` | Health check | More standard naming (replaces `/healthz`) |
| GET | `/api/v1/ws` | WebSocket connection | Clearer path (replaces `/socket.io`) |

## Frontend Migration

### Migration Overview

The frontend interface has been migrated from MailDev-compatible API to the new RESTful API design (`/api/v1/`).

### API Base Path Migration

**Old API:**
```javascript
const API_BASE = window.location.origin;
```

**New API:**
```javascript
const API_BASE = `${window.location.origin}/api/v1`;
```

### API Endpoint Migration Reference

| Functionality | Old API (MailDev Compatible) | New API (Recommended) | Notes |
|---------------|------------------------------|----------------------|-------|
| Get all emails | `GET /email` | `GET /api/v1/emails` | Use plural resource |
| Get single email | `GET /email/:id` | `GET /api/v1/emails/:id` | Use plural resource |
| Get email HTML | `GET /email/:id/html` | `GET /api/v1/emails/:id/html` | Use plural resource |
| Download attachment | `GET /email/:id/attachment/:filename` | `GET /api/v1/emails/:id/attachments/:filename` | Use plural `attachments` |
| Download raw email | `GET /email/:id/download` | `GET /api/v1/emails/:id/raw` | More semantic naming |
| View email source | `GET /email/:id/source` | `GET /api/v1/emails/:id/source` | Use plural resource |
| Delete single email | `DELETE /email/:id` | `DELETE /api/v1/emails/:id` | Use plural resource |
| Delete all emails | `DELETE /email/all` | `DELETE /api/v1/emails` | More RESTful, no `/all` |
| Mark all as read | `PATCH /email/read-all` | `PATCH /api/v1/emails/read` | Clearer naming |
| Relay email | `POST /email/:id/relay` | `POST /api/v1/emails/:id/actions/relay` | Explicitly indicates action |
| WebSocket connection | `GET /socket.io` | `GET /api/v1/ws` | Clearer path |

### Code Migration Examples

#### 1. API Base Path

```javascript
// Old code
const API_BASE = window.location.origin;

// New code
const API_BASE = `${window.location.origin}/api/v1`;
```

#### 2. Get All Emails

```javascript
// Old code
const response = await fetch(`${API_BASE}/email?${params}`);

// New code
const response = await fetch(`${API_BASE}/emails?${params}`);
```

#### 3. Get Single Email

```javascript
// Old code
const response = await fetch(`${API_BASE}/email/${id}`);

// New code
const response = await fetch(`${API_BASE}/emails/${id}`);
```

#### 4. Delete All Emails

```javascript
// Old code
const response = await fetch(`${API_BASE}/email/all`, {
    method: 'DELETE'
});

// New code
const response = await fetch(`${API_BASE}/emails`, {
    method: 'DELETE'
});
```

#### 5. Mark All as Read

```javascript
// Old code
const response = await fetch(`${API_BASE}/email/read-all`, {
    method: 'PATCH'
});

// New code
const response = await fetch(`${API_BASE}/emails/read`, {
    method: 'PATCH'
});
```

#### 6. Relay Email

```javascript
// Old code
const url = relayTo 
    ? `${API_BASE}/email/${id}/relay?relayTo=${encodeURIComponent(relayTo)}`
    : `${API_BASE}/email/${id}/relay`;

// New code
const url = relayTo 
    ? `${API_BASE}/emails/${id}/actions/relay/${encodeURIComponent(relayTo)}`
    : `${API_BASE}/emails/${id}/actions/relay`;
```

#### 7. Download Attachment

```javascript
// Old code
const url = `${API_BASE}/email/${emailId}/attachment/${encodeURIComponent(att.generatedFileName)}`;

// New code
const url = `${API_BASE}/emails/${emailId}/attachments/${encodeURIComponent(att.generatedFileName)}`;
```

#### 8. Download Raw Email

```javascript
// Old code
window.open(`${API_BASE}/email/${id}/download`, '_blank');

// New code
window.open(`${API_BASE}/emails/${id}/raw`, '_blank');
```

#### 9. WebSocket Connection

```javascript
// Old code
const wsUrl = `${protocol}//${window.location.host}/socket.io`;

// New code
const wsUrl = `${protocol}//${window.location.host}/api/v1/ws`;
```

## Migration Guide

### From MailDev-Compatible API to New API

#### 1. Resource Path Migration

```javascript
// Old API (MailDev Compatible)
GET /email
GET /email/:id

// New API (Recommended)
GET /api/v1/emails
GET /api/v1/emails/:id
```

#### 2. Batch Operations Migration

```javascript
// Old API
DELETE /email/all
POST /email/batch/delete

// New API
DELETE /api/v1/emails
DELETE /api/v1/emails/batch
```

#### 3. Action Operations Migration

```javascript
// Old API
POST /email/:id/relay

// New API
POST /api/v1/emails/:id/actions/relay
```

#### 4. Sub-resource Migration

```javascript
// Old API
GET /email/:id/attachment/:filename
GET /email/:id/download

// New API
GET /api/v1/emails/:id/attachments/:filename
GET /api/v1/emails/:id/raw
```

#### 5. Configuration API Migration

```javascript
// Old API
GET /config
GET /config/outgoing

// New API
GET /api/v1/settings
GET /api/v1/settings/outgoing
```

#### 6. System API Migration

```javascript
// Old API
GET /healthz
GET /reloadMailsFromDirectory
GET /socket.io

// New API
GET /api/v1/health
POST /api/v1/emails/reload
GET /api/v1/ws
```

## Refactoring Benefits

### 1. Better RESTful Design Principles
- ✅ Use plural resources: `/emails` instead of `/email`
- ✅ More standard batch operations: `DELETE /emails` instead of `DELETE /email/all`
- ✅ Clearer action operations: `/actions/relay` explicitly indicates actions

### 2. More Semantic Naming
- ✅ `/raw` replaces `/download` (more semantic)
- ✅ `/attachments` uses plural (more standard)
- ✅ `/ws` replaces `/socket.io` (more concise)

### 3. API Versioning
- ✅ All APIs use `/api/v1/` prefix
- ✅ Support for future API version evolution

### 4. Better Maintainability
- ✅ Unified API design style
- ✅ Clear resource hierarchy
- ✅ Easy to understand and extend

### 5. Improved HTTP Method Usage
- ✅ Reload uses `POST /emails/reload` instead of `GET /reloadMailsFromDirectory`
- ✅ Delete operations use `DELETE` instead of `POST`

### 6. Unified Naming Style
- ✅ Use lowercase letters and hyphens (kebab-case)
- ✅ Avoid camelCase naming

## Compatibility Guarantee

While the frontend has been migrated to the new API, the backend still maintains all MailDev-compatible original API endpoints, ensuring:

- ✅ Existing client code can continue using the old API
- ✅ New clients can use the improved API
- ✅ Both API designs can be used simultaneously
- ✅ Smooth migration path

## Testing Recommendations

After migration, the following functionality should be tested:

1. ✅ Email list loading
2. ✅ Email detail viewing
3. ✅ Email deletion (single and all)
4. ✅ Mark as read functionality
5. ✅ Attachment download
6. ✅ Raw email download
7. ✅ Email source viewing
8. ✅ WebSocket real-time updates
9. ✅ Email relay functionality

## Best Practices

1. **New Projects**: Recommended to use the new `/api/v1/` API
2. **Existing Projects**: Can continue using MailDev-compatible API, migrate gradually
3. **Mixed Usage**: Both APIs can be used simultaneously, choose based on needs

## Summary

Through this API refactoring, we have:

1. ✅ Maintained full backward compatibility (all MailDev APIs are preserved)
2. ✅ Provided a new API design that better conforms to RESTful best practices
3. ✅ Unified resource naming conventions (using plural forms)
4. ✅ Improved HTTP method usage (more semantic)
5. ✅ Added API versioning (support for future evolution)
6. ✅ Enhanced API readability and maintainability

These improvements make the API more standard and user-friendly while maintaining compatibility with existing systems. The frontend interface has been successfully migrated to the new RESTful API design, with all API calls using the `/api/v1/` prefix and more standard resource naming. This improves code maintainability and extensibility while maintaining full compatibility with the backend.

