# 前端 API 迁移说明

## 概述

前端界面已从 MailDev 兼容 API 迁移到新的 RESTful API 设计（`/api/v1/`）。

## 迁移详情

### API 基础路径

**旧 API：**
```javascript
const API_BASE = window.location.origin;
```

**新 API：**
```javascript
const API_BASE = `${window.location.origin}/api/v1`;
```

### API 端点迁移对照表

| 功能 | 旧 API (MailDev 兼容) | 新 API (推荐) | 说明 |
|------|----------------------|--------------|------|
| 获取所有邮件 | `GET /email` | `GET /api/v1/emails` | 使用复数资源 |
| 获取单个邮件 | `GET /email/:id` | `GET /api/v1/emails/:id` | 使用复数资源 |
| 获取邮件 HTML | `GET /email/:id/html` | `GET /api/v1/emails/:id/html` | 使用复数资源 |
| 下载附件 | `GET /email/:id/attachment/:filename` | `GET /api/v1/emails/:id/attachments/:filename` | 使用复数 `attachments` |
| 下载原始邮件 | `GET /email/:id/download` | `GET /api/v1/emails/:id/raw` | 更语义化的命名 |
| 查看邮件源码 | `GET /email/:id/source` | `GET /api/v1/emails/:id/source` | 使用复数资源 |
| 删除单个邮件 | `DELETE /email/:id` | `DELETE /api/v1/emails/:id` | 使用复数资源 |
| 删除所有邮件 | `DELETE /email/all` | `DELETE /api/v1/emails` | 更 RESTful，不使用 `/all` |
| 标记所有已读 | `PATCH /email/read-all` | `PATCH /api/v1/emails/read` | 更清晰的命名 |
| 转发邮件 | `POST /email/:id/relay` | `POST /api/v1/emails/:id/actions/relay` | 明确表示动作操作 |
| WebSocket 连接 | `GET /socket.io` | `GET /api/v1/ws` | 更清晰的路径 |

### 代码变更详情

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

#### 5. 标记所有已读

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

## 改进优势

### 1. 更符合 RESTful 设计原则
- ✅ 使用复数资源：`/emails` 而不是 `/email`
- ✅ 批量操作更规范：`DELETE /emails` 而不是 `DELETE /email/all`
- ✅ 动作操作更清晰：`/actions/relay` 明确表示动作

### 2. 更语义化的命名
- ✅ `/raw` 替代 `/download`（更语义化）
- ✅ `/attachments` 使用复数（更规范）
- ✅ `/ws` 替代 `/socket.io`（更简洁）

### 3. API 版本控制
- ✅ 所有 API 使用 `/api/v1/` 前缀
- ✅ 支持未来 API 版本演进

### 4. 更好的可维护性
- ✅ 统一的 API 设计风格
- ✅ 清晰的资源层次结构
- ✅ 易于理解和扩展

## 兼容性说明

虽然前端已迁移到新 API，但后端仍然保留所有 MailDev 兼容的原始 API 端点，确保：

- ✅ 现有客户端代码可以继续使用旧 API
- ✅ 新客户端可以使用改进后的 API
- ✅ 两种 API 设计可以同时使用
- ✅ 平滑的迁移路径

## 测试建议

迁移后建议测试以下功能：

1. ✅ 邮件列表加载
2. ✅ 邮件详情查看
3. ✅ 邮件删除（单个和全部）
4. ✅ 标记已读功能
5. ✅ 附件下载
6. ✅ 原始邮件下载
7. ✅ 邮件源码查看
8. ✅ WebSocket 实时更新
9. ✅ 邮件转发功能

## 总结

前端界面已成功迁移到新的 RESTful API 设计，所有 API 调用都使用 `/api/v1/` 前缀和更规范的资源命名。这提高了代码的可维护性和可扩展性，同时保持了与后端的完全兼容性。

