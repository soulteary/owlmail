# API 重构记录

## 概述

本文档记录了 OwlMail 的 API 重构过程，记录了从 MailDev 兼容的 API 端点迁移到新的 RESTful API 设计（`/api/v1/`）的过程。重构在引入改进的 API 设计模式的同时，保持了完全的向后兼容性。

## 重构目标

重构旨在解决以下 API 设计问题：

1. **资源命名不一致** - 使用单数 `/email` 而不是复数 `/emails`
2. **非标准 RESTful 设计** - 使用 `/all` 后缀和不正确的 HTTP 方法
3. **操作命名不明确** - 操作未明确标识
4. **子资源命名不一致** - 使用单数 `attachment` 而不是复数
5. **路径命名语义性不足** - 使用通用术语如 `/download` 而不是 `/raw`
6. **缺少 API 版本控制** - 没有版本前缀，无法进行未来 API 演进
7. **HTTP 方法使用不当** - 使用 GET 进行状态更改操作

## API 设计问题分析

### 1. 资源命名不一致

**问题：**
- 使用单数 `/email` 而不是复数 `/emails`
- RESTful 最佳实践建议对资源集合使用复数形式

**示例：**
```
❌ GET /email          (单数)
✅ GET /emails         (复数)
```

### 2. 非标准 RESTful 设计

**问题：**
- `DELETE /email/all` - 使用 `/all` 后缀不符合 RESTful 规范
- `POST /email/batch/delete` - 使用 POST 进行删除操作不够语义化
- `PATCH /email/read-all` - 使用连字符命名不够清晰

**改进：**
```
❌ DELETE /email/all
✅ DELETE /emails

❌ POST /email/batch/delete
✅ DELETE /emails/batch

❌ PATCH /email/read-all
✅ PATCH /emails/read
```

### 3. 操作命名不明确

**问题：**
- `/email/:id/relay` - 操作不够明确
- 应该明确表示这是一个操作

**改进：**
```
❌ POST /email/:id/relay
✅ POST /emails/:id/actions/relay
```

### 4. 子资源命名不一致

**问题：**
- `/email/:id/attachment/:filename` - 使用单数 `attachment`
- 应该使用复数 `attachments` 来表示资源集合

**改进：**
```
❌ GET /email/:id/attachment/:filename
✅ GET /emails/:id/attachments/:filename
```

### 5. 路径命名语义性不足

**问题：**
- `/email/:id/download` - `download` 语义性不足
- `/config` - `config` 不如 `settings` 语义化
- `/healthz` - 非标准命名
- `/reloadMailsFromDirectory` - 驼峰命名不符合 RESTful 风格

**改进：**
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

### 6. HTTP 方法使用不当

**问题：**
- `GET /reloadMailsFromDirectory` - 重新加载是状态更改操作，应该使用 POST
- `POST /email/batch/delete` - 删除操作应该使用 DELETE

**改进：**
```
❌ GET /reloadMailsFromDirectory
✅ POST /emails/reload

❌ POST /email/batch/delete
✅ DELETE /emails/batch
```

### 7. 缺少 API 版本控制

**问题：**
- 没有 API 版本前缀
- 无法进行 API 版本演进

**改进：**
```
❌ GET /email
✅ GET /api/v1/emails
```

## 重构后的 API 设计

### MailDev 兼容 API（为向后兼容而保留）

保留所有原始 MailDev API 端点以确保向后兼容性：

| 方法 | 路径 | 描述 |
|--------|------|-------------|
| GET | `/email` | 获取所有邮件 |
| GET | `/email/:id` | 获取单个邮件 |
| GET | `/email/:id/html` | 获取邮件 HTML |
| GET | `/email/:id/attachment/:filename` | 下载附件 |
| GET | `/email/:id/download` | 下载原始 .eml 文件 |
| GET | `/email/:id/source` | 获取邮件原始源码 |
| DELETE | `/email/:id` | 删除单个邮件 |
| DELETE | `/email/all` | 删除所有邮件 |
| PATCH | `/email/read-all` | 标记所有邮件为已读 |
| PATCH | `/email/:id/read` | 标记单个邮件为已读 |
| POST | `/email/:id/relay` | 转发邮件 |
| POST | `/email/:id/relay/:relayTo` | 转发邮件到指定地址 |
| GET | `/email/stats` | 邮件统计 |
| GET | `/email/preview` | 邮件预览 |
| POST | `/email/batch/delete` | 批量删除 |
| POST | `/email/batch/read` | 批量标记为已读 |
| GET | `/email/export` | 导出邮件 |
| GET | `/config` | 获取配置 |
| GET | `/config/outgoing` | 获取发件配置 |
| PUT | `/config/outgoing` | 更新发件配置 |
| PATCH | `/config/outgoing` | 部分更新发件配置 |
| GET | `/healthz` | 健康检查 |
| GET | `/reloadMailsFromDirectory` | 重新加载邮件 |
| GET | `/socket.io` | WebSocket 连接 |

### 新的改进 API（推荐使用）

#### 邮件资源 (`/api/v1/emails`)

| 方法 | 路径 | 描述 | 改进 |
|--------|------|-------------|-------------|
| GET | `/api/v1/emails` | 获取所有邮件 | 使用复数资源 |
| GET | `/api/v1/emails/:id` | 获取单个邮件 | 使用复数资源 |
| DELETE | `/api/v1/emails/:id` | 删除单个邮件 | 使用复数资源 |
| DELETE | `/api/v1/emails` | 删除所有邮件 | 更符合 RESTful，无 `/all` 后缀 |
| DELETE | `/api/v1/emails/batch` | 批量删除 | 使用 DELETE 而不是 POST |
| PATCH | `/api/v1/emails/read` | 标记所有邮件为已读 | 命名更清晰 |
| PATCH | `/api/v1/emails/:id/read` | 标记单个邮件为已读 | 使用复数资源 |
| PATCH | `/api/v1/emails/batch/read` | 批量标记为已读 | 使用复数资源 |
| GET | `/api/v1/emails/stats` | 邮件统计 | 使用复数资源 |
| GET | `/api/v1/emails/preview` | 邮件预览 | 使用复数资源 |
| GET | `/api/v1/emails/export` | 导出邮件 | 使用复数资源 |
| POST | `/api/v1/emails/reload` | 重新加载邮件 | 使用 POST 而不是 GET |

#### 邮件内容资源

| 方法 | 路径 | 描述 | 改进 |
|--------|------|-------------|-------------|
| GET | `/api/v1/emails/:id/html` | 获取邮件 HTML | 使用复数资源 |
| GET | `/api/v1/emails/:id/source` | 获取邮件源码 | 使用复数资源 |
| GET | `/api/v1/emails/:id/raw` | 获取原始邮件 | 更语义化的命名（替代 `/download`） |
| GET | `/api/v1/emails/:id/attachments/:filename` | 下载附件 | 使用复数 `attachments` |

#### 邮件操作

| 方法 | 路径 | 描述 | 改进 |
|--------|------|-------------|-------------|
| POST | `/api/v1/emails/:id/actions/relay` | 转发邮件 | 明确表示操作 |
| POST | `/api/v1/emails/:id/actions/relay/:relayTo` | 转发邮件到指定地址 | 明确表示操作 |

#### 设置资源 (`/api/v1/settings`)

| 方法 | 路径 | 描述 | 改进 |
|--------|------|-------------|-------------|
| GET | `/api/v1/settings` | 获取所有设置 | 更语义化的命名（替代 `/config`） |
| GET | `/api/v1/settings/outgoing` | 获取发件配置 | 更语义化的命名 |
| PUT | `/api/v1/settings/outgoing` | 更新发件配置 | 更语义化的命名 |
| PATCH | `/api/v1/settings/outgoing` | 部分更新发件配置 | 更语义化的命名 |

#### 系统资源

| 方法 | 路径 | 描述 | 改进 |
|--------|------|-------------|-------------|
| GET | `/api/v1/health` | 健康检查 | 更标准的命名（替代 `/healthz`） |
| GET | `/api/v1/ws` | WebSocket 连接 | 更清晰的路径（替代 `/socket.io`） |

## 前端迁移

### 迁移概述

前端界面已从 MailDev 兼容的 API 迁移到新的 RESTful API 设计（`/api/v1/`）。

### API 基础路径迁移

**旧 API：**
```javascript
const API_BASE = window.location.origin;
```

**新 API：**
```javascript
const API_BASE = `${window.location.origin}/api/v1`;
```

### API 端点迁移参考

| 功能 | 旧 API（MailDev 兼容） | 新 API（推荐） | 说明 |
|---------------|------------------------------|----------------------|-------|
| 获取所有邮件 | `GET /email` | `GET /api/v1/emails` | 使用复数资源 |
| 获取单个邮件 | `GET /email/:id` | `GET /api/v1/emails/:id` | 使用复数资源 |
| 获取邮件 HTML | `GET /email/:id/html` | `GET /api/v1/emails/:id/html` | 使用复数资源 |
| 下载附件 | `GET /email/:id/attachment/:filename` | `GET /api/v1/emails/:id/attachments/:filename` | 使用复数 `attachments` |
| 下载原始邮件 | `GET /email/:id/download` | `GET /api/v1/emails/:id/raw` | 更语义化的命名 |
| 查看邮件源码 | `GET /email/:id/source` | `GET /api/v1/emails/:id/source` | 使用复数资源 |
| 删除单个邮件 | `DELETE /email/:id` | `DELETE /api/v1/emails/:id` | 使用复数资源 |
| 删除所有邮件 | `DELETE /email/all` | `DELETE /api/v1/emails` | 更符合 RESTful，无 `/all` |
| 标记所有为已读 | `PATCH /email/read-all` | `PATCH /api/v1/emails/read` | 命名更清晰 |
| 转发邮件 | `POST /email/:id/relay` | `POST /api/v1/emails/:id/actions/relay` | 明确表示操作 |
| WebSocket 连接 | `GET /socket.io` | `GET /api/v1/ws` | 路径更清晰 |

### 代码迁移示例

#### 1. API 基础路径

```javascript
// 旧代码
const API_BASE = window.location.origin;

// 新代码
const API_BASE = `${window.location.origin}/api/v1`;
```

#### 2. 获取所有邮件

```javascript
// 旧代码
const response = await fetch(`${API_BASE}/email?${params}`);

// 新代码
const response = await fetch(`${API_BASE}/emails?${params}`);
```

#### 3. 获取单个邮件

```javascript
// 旧代码
const response = await fetch(`${API_BASE}/email/${id}`);

// 新代码
const response = await fetch(`${API_BASE}/emails/${id}`);
```

#### 4. 删除所有邮件

```javascript
// 旧代码
const response = await fetch(`${API_BASE}/email/all`, {
    method: 'DELETE'
});

// 新代码
const response = await fetch(`${API_BASE}/emails`, {
    method: 'DELETE'
});
```

#### 5. 标记所有为已读

```javascript
// 旧代码
const response = await fetch(`${API_BASE}/email/read-all`, {
    method: 'PATCH'
});

// 新代码
const response = await fetch(`${API_BASE}/emails/read`, {
    method: 'PATCH'
});
```

#### 6. 转发邮件

```javascript
// 旧代码
const url = relayTo 
    ? `${API_BASE}/email/${id}/relay?relayTo=${encodeURIComponent(relayTo)}`
    : `${API_BASE}/email/${id}/relay`;

// 新代码
const url = relayTo 
    ? `${API_BASE}/emails/${id}/actions/relay/${encodeURIComponent(relayTo)}`
    : `${API_BASE}/emails/${id}/actions/relay`;
```

#### 7. 下载附件

```javascript
// 旧代码
const url = `${API_BASE}/email/${emailId}/attachment/${encodeURIComponent(att.generatedFileName)}`;

// 新代码
const url = `${API_BASE}/emails/${emailId}/attachments/${encodeURIComponent(att.generatedFileName)}`;
```

#### 8. 下载原始邮件

```javascript
// 旧代码
window.open(`${API_BASE}/email/${id}/download`, '_blank');

// 新代码
window.open(`${API_BASE}/emails/${id}/raw`, '_blank');
```

#### 9. WebSocket 连接

```javascript
// 旧代码
const wsUrl = `${protocol}//${window.location.host}/socket.io`;

// 新代码
const wsUrl = `${protocol}//${window.location.host}/api/v1/ws`;
```

## 迁移指南

### 从 MailDev 兼容 API 迁移到新 API

#### 1. 资源路径迁移

```javascript
// 旧 API（MailDev 兼容）
GET /email
GET /email/:id

// 新 API（推荐）
GET /api/v1/emails
GET /api/v1/emails/:id
```

#### 2. 批量操作迁移

```javascript
// 旧 API
DELETE /email/all
POST /email/batch/delete

// 新 API
DELETE /api/v1/emails
DELETE /api/v1/emails/batch
```

#### 3. 操作迁移

```javascript
// 旧 API
POST /email/:id/relay

// 新 API
POST /api/v1/emails/:id/actions/relay
```

#### 4. 子资源迁移

```javascript
// 旧 API
GET /email/:id/attachment/:filename
GET /email/:id/download

// 新 API
GET /api/v1/emails/:id/attachments/:filename
GET /api/v1/emails/:id/raw
```

#### 5. 配置 API 迁移

```javascript
// 旧 API
GET /config
GET /config/outgoing

// 新 API
GET /api/v1/settings
GET /api/v1/settings/outgoing
```

#### 6. 系统 API 迁移

```javascript
// 旧 API
GET /healthz
GET /reloadMailsFromDirectory
GET /socket.io

// 新 API
GET /api/v1/health
POST /api/v1/emails/reload
GET /api/v1/ws
```

## 重构收益

### 1. 更好的 RESTful 设计原则
- ✅ 使用复数资源：`/emails` 而不是 `/email`
- ✅ 更标准的批量操作：`DELETE /emails` 而不是 `DELETE /email/all`
- ✅ 更清晰的操作：`/actions/relay` 明确表示操作

### 2. 更语义化的命名
- ✅ `/raw` 替代 `/download`（更语义化）
- ✅ `/attachments` 使用复数（更标准）
- ✅ `/ws` 替代 `/socket.io`（更简洁）

### 3. API 版本控制
- ✅ 所有 API 使用 `/api/v1/` 前缀
- ✅ 支持未来 API 版本演进

### 4. 更好的可维护性
- ✅ 统一的 API 设计风格
- ✅ 清晰的资源层次结构
- ✅ 易于理解和扩展

### 5. 改进的 HTTP 方法使用
- ✅ 重新加载使用 `POST /emails/reload` 而不是 `GET /reloadMailsFromDirectory`
- ✅ 删除操作使用 `DELETE` 而不是 `POST`

### 6. 统一的命名风格
- ✅ 使用小写字母和连字符（kebab-case）
- ✅ 避免驼峰命名

## 兼容性保证

虽然前端已迁移到新 API，但后端仍保留所有 MailDev 兼容的原始 API 端点，确保：

- ✅ 现有客户端代码可以继续使用旧 API
- ✅ 新客户端可以使用改进的 API
- ✅ 两种 API 设计可以同时使用
- ✅ 平滑的迁移路径

## 测试建议

迁移后，应测试以下功能：

1. ✅ 邮件列表加载
2. ✅ 邮件详情查看
3. ✅ 邮件删除（单个和全部）
4. ✅ 标记为已读功能
5. ✅ 附件下载
6. ✅ 原始邮件下载
7. ✅ 邮件源码查看
8. ✅ WebSocket 实时更新
9. ✅ 邮件转发功能

## 最佳实践

1. **新项目**：推荐使用新的 `/api/v1/` API
2. **现有项目**：可以继续使用 MailDev 兼容的 API，逐步迁移
3. **混合使用**：两种 API 可以同时使用，根据需求选择

## 总结

通过这次 API 重构，我们实现了：

1. ✅ 保持完全的向后兼容性（所有 MailDev API 都保留）
2. ✅ 提供了更符合 RESTful 最佳实践的新 API 设计
3. ✅ 统一了资源命名约定（使用复数形式）
4. ✅ 改进了 HTTP 方法使用（更语义化）
5. ✅ 添加了 API 版本控制（支持未来演进）
6. ✅ 增强了 API 可读性和可维护性

这些改进使 API 更加标准和用户友好，同时保持与现有系统的兼容性。前端界面已成功迁移到新的 RESTful API 设计，所有 API 调用都使用 `/api/v1/` 前缀和更标准的资源命名。这提高了代码的可维护性和可扩展性，同时保持了与后端的完全兼容性。
