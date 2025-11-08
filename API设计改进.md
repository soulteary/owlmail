# API 设计改进文档

## 概述

本文档分析了当前 API 设计的问题，并提供了更合理的 RESTful API 设计。同时，为了保持向后兼容性，所有 MailDev 兼容的原始 API 端点都得到保留。

## 当前 API 设计问题分析

### 1. 资源命名不统一

**问题：**
- 使用单数 `/email` 而不是复数 `/emails`
- RESTful 最佳实践建议使用复数形式表示资源集合

**示例：**
```
❌ GET /email          (单数)
✅ GET /emails         (复数)
```

### 2. RESTful 设计不规范

**问题：**
- `DELETE /email/all` - 使用 `/all` 后缀不够 RESTful
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

### 3. 动作命名不够清晰

**问题：**
- `/email/:id/relay` - 动作不够明确
- 应该明确表示这是一个动作操作

**改进：**
```
❌ POST /email/:id/relay
✅ POST /emails/:id/actions/relay
```

### 4. 子资源命名不一致

**问题：**
- `/email/:id/attachment/:filename` - 使用单数 `attachment`
- 应该使用复数 `attachments` 表示资源集合

**改进：**
```
❌ GET /email/:id/attachment/:filename
✅ GET /emails/:id/attachments/:filename
```

### 5. 路径命名不够语义化

**问题：**
- `/email/:id/download` - `download` 不够语义化
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
- `GET /reloadMailsFromDirectory` - 重新加载是修改操作，应该使用 POST
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

## 改进后的 API 设计

### MailDev 兼容 API（保持向后兼容）

所有原始 MailDev API 端点都得到保留，确保向后兼容性：

| 方法 | 路径 | 说明 |
|------|------|------|
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
| POST | `/email/batch/read` | 批量标记已读 |
| GET | `/email/export` | 导出邮件 |
| GET | `/config` | 获取配置 |
| GET | `/config/outgoing` | 获取出站配置 |
| PUT | `/config/outgoing` | 更新出站配置 |
| PATCH | `/config/outgoing` | 部分更新出站配置 |
| GET | `/healthz` | 健康检查 |
| GET | `/reloadMailsFromDirectory` | 重新加载邮件 |
| GET | `/socket.io` | WebSocket 连接 |

### 新的改进 API（推荐使用）

#### 邮件资源 (`/api/v1/emails`)

| 方法 | 路径 | 说明 | 改进点 |
|------|------|------|--------|
| GET | `/api/v1/emails` | 获取所有邮件 | 使用复数资源 |
| GET | `/api/v1/emails/:id` | 获取单个邮件 | 使用复数资源 |
| DELETE | `/api/v1/emails/:id` | 删除单个邮件 | 使用复数资源 |
| DELETE | `/api/v1/emails` | 删除所有邮件 | 更 RESTful，不使用 `/all` |
| DELETE | `/api/v1/emails/batch` | 批量删除 | 使用 DELETE 而不是 POST |
| PATCH | `/api/v1/emails/read` | 标记所有邮件为已读 | 更清晰的命名 |
| PATCH | `/api/v1/emails/:id/read` | 标记单个邮件为已读 | 使用复数资源 |
| PATCH | `/api/v1/emails/batch/read` | 批量标记已读 | 使用复数资源 |
| GET | `/api/v1/emails/stats` | 邮件统计 | 使用复数资源 |
| GET | `/api/v1/emails/preview` | 邮件预览 | 使用复数资源 |
| GET | `/api/v1/emails/export` | 导出邮件 | 使用复数资源 |
| POST | `/api/v1/emails/reload` | 重新加载邮件 | 使用 POST 而不是 GET |

#### 邮件内容资源

| 方法 | 路径 | 说明 | 改进点 |
|------|------|------|--------|
| GET | `/api/v1/emails/:id/html` | 获取邮件 HTML | 使用复数资源 |
| GET | `/api/v1/emails/:id/source` | 获取邮件源码 | 使用复数资源 |
| GET | `/api/v1/emails/:id/raw` | 获取原始邮件 | 更语义化的命名（替代 `/download`） |
| GET | `/api/v1/emails/:id/attachments/:filename` | 下载附件 | 使用复数 `attachments` |

#### 邮件动作

| 方法 | 路径 | 说明 | 改进点 |
|------|------|------|--------|
| POST | `/api/v1/emails/:id/actions/relay` | 转发邮件 | 明确表示这是动作操作 |
| POST | `/api/v1/emails/:id/actions/relay/:relayTo` | 转发邮件到指定地址 | 明确表示这是动作操作 |

#### 设置资源 (`/api/v1/settings`)

| 方法 | 路径 | 说明 | 改进点 |
|------|------|------|--------|
| GET | `/api/v1/settings` | 获取所有设置 | 更语义化的命名（替代 `/config`） |
| GET | `/api/v1/settings/outgoing` | 获取出站配置 | 更语义化的命名 |
| PUT | `/api/v1/settings/outgoing` | 更新出站配置 | 更语义化的命名 |
| PATCH | `/api/v1/settings/outgoing` | 部分更新出站配置 | 更语义化的命名 |

#### 系统资源

| 方法 | 路径 | 说明 | 改进点 |
|------|------|------|--------|
| GET | `/api/v1/health` | 健康检查 | 更标准的命名（替代 `/healthz`） |
| GET | `/api/v1/ws` | WebSocket 连接 | 更清晰的路径（替代 `/socket.io`） |

## API 设计改进总结

### 1. 资源命名统一
- ✅ 使用复数形式：`/emails` 而不是 `/email`
- ✅ 子资源使用复数：`/attachments` 而不是 `/attachment`

### 2. RESTful 设计更规范
- ✅ 批量删除使用 `DELETE /emails` 而不是 `DELETE /email/all`
- ✅ 批量操作使用 `DELETE /emails/batch` 而不是 `POST /email/batch/delete`
- ✅ 动作操作使用 `/actions/` 前缀明确标识

### 3. HTTP 方法使用更合理
- ✅ 重新加载使用 `POST /emails/reload` 而不是 `GET /reloadMailsFromDirectory`
- ✅ 删除操作使用 `DELETE` 而不是 `POST`

### 4. 路径命名更语义化
- ✅ `/raw` 替代 `/download`（更语义化）
- ✅ `/settings` 替代 `/config`（更语义化）
- ✅ `/health` 替代 `/healthz`（更标准）
- ✅ `/ws` 替代 `/socket.io`（更简洁）

### 5. API 版本控制
- ✅ 所有新 API 使用 `/api/v1/` 前缀
- ✅ 支持未来 API 版本演进

### 6. 命名风格统一
- ✅ 使用小写字母和连字符（kebab-case）
- ✅ 避免驼峰命名（camelCase）

## 迁移指南

### 从 MailDev 兼容 API 迁移到新 API

#### 1. 资源路径迁移

```javascript
// 旧 API (MailDev 兼容)
GET /email
GET /email/:id

// 新 API (推荐)
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

#### 3. 动作操作迁移

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

## 兼容性保证

- ✅ 所有 MailDev 兼容的原始 API 端点都得到保留
- ✅ 现有客户端代码无需修改即可继续工作
- ✅ 新客户端可以使用改进后的 API 设计
- ✅ 两种 API 设计可以同时使用

## 最佳实践建议

1. **新项目**：推荐使用新的 `/api/v1/` API
2. **现有项目**：可以继续使用 MailDev 兼容 API，逐步迁移
3. **混合使用**：两种 API 可以同时使用，根据需求选择

## 总结

通过这次 API 设计改进，我们：

1. ✅ 保持了完全的向后兼容性（所有 MailDev API 都保留）
2. ✅ 提供了更符合 RESTful 最佳实践的新 API 设计
3. ✅ 统一了资源命名规范（使用复数形式）
4. ✅ 改进了 HTTP 方法的使用（更语义化）
5. ✅ 添加了 API 版本控制（支持未来演进）
6. ✅ 提高了 API 的可读性和可维护性

这些改进使得 API 更加规范、易用，同时保持了与现有系统的兼容性。

