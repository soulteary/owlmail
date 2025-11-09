# OwlMail × MailDev：功能与 API 完整对比与迁移白皮书

> **基于源代码的深度对比 + 面向用户与开发者的迁移指南**

---

## 📋 执行摘要

经过对两个项目源代码与接口的系统梳理，**OwlMail（Go）与 MailDev（Node.js）在核心功能与兼容 API 上完全一致**；在此基础上，OwlMail 还提供**更强的 REST 设计、批量操作、统计、导出、SMTPS(465)** 等增强能力，并以**单一二进制**形态带来更佳的性能与部署体验。

**核心结论**

* ✅ **API 兼容性：100%** — 覆盖所有 MailDev 端点
* ✅ **功能一致性：100%** — 收发/查看/删除/转发等核心能力等价
* ✅ **环境变量兼容：100%** — 优先识别 MailDev 变量；无缝迁移
* ✅ **增强能力** — 批量操作、统计、导出、改进 REST、SMTPS(465)
* ⚠️ **WebSocket 协议差异** — MailDev 用 Socket.IO，OwlMail 用原生 WS；功能一致，客户端需小幅适配

**可替换性结论**：在绝大多数场景可直接以 OwlMail 替换 MailDev，WebSocket 客户端按需从 Socket.IO 适配为标准 WebSocket 即可。

---

## 📖 项目概述与技术栈

| 项目          | 语言      | 版本    | 描述                             |
| ----------- | ------- | ----- | ------------------------------ |
| **OwlMail** | Go      | 1.0+  | 兼容 MailDev 的邮件开发/测试工具，附加大量增强能力 |
| **MailDev** | Node.js | 2.2.1 | 经典的邮件开发/测试工具，配套成熟前端            |

**技术栈对比**

| 方面        | OwlMail                  | MailDev        |
| --------- | ------------------------ | -------------- |
| 语言/运行时    | Go 1.24+（单一二进制）          | Node.js ≥ 18   |
| Web 框架    | Gin                      | Express        |
| SMTP 库    | emersion/go-smtp         | smtp-server    |
| 邮件解析      | emersion/go-message      | mailparser-mit |
| WebSocket | gorilla/websocket（原生 WS） | Socket.IO      |
| HTML 清理   | bluemonday               | DOMPurify      |
| 前端        | 原生 JS（轻量）                | AngularJS（完整）  |

---

## 🔍 兼容 API 端点（100% 覆盖）

> 下表按 MailDev 既有路径列示；OwlMail 在 **/api/v1/** 下还提供改进版 REST 路由（见后）。

| 端点                                | 方法     | MailDev | OwlMail | 兼容性要点                                                                                                                          |
| --------------------------------- | ------ | ------- | ------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `/email`                          | GET    | ✅       | ✅       | OwlMail 兼容 `skip`（映射 `offset`），并新增 `limit/q/from/to/dateFrom/dateTo/read/sort*`；返回 `{total, limit, offset, emails}` 更 RESTful。 |
| `/email/:id`                      | GET    | ✅       | ✅       | 功能一致；MailDev 取详情即置已读，OwlMail 需额外 `PATCH /email/:id/read`。                                                                      |
| `/email/:id/html`                 | GET    | ✅       | ✅       | 均返回 HTML；OwlMail 明确 `Content-Type`；baseUrl 处理策略不同但不影响主体功能。                                                                     |
| `/email/:id/attachment/:filename` | GET    | ✅       | ✅       | 均可下载附件，类型设置正确。                                                                                                                 |
| `/email/:id/download`             | GET    | ✅       | ✅       | 均下载 EML；OwlMail 以主题生成更友好的文件名。                                                                                                  |
| `/email/:id/source`               | GET    | ✅       | ✅       | 均返回原始源码（OwlMail 直接文本流）。                                                                                                        |
| `/email/:id`                      | DELETE | ✅       | ✅       | 功能一致；MailDev 删除不存在时常用 500，OwlMail 使用 404 更符合 REST。                                                                             |
| `/email/all`                      | DELETE | ✅       | ✅       | 均删除全部邮件；错误用 500。                                                                                                               |
| `/email/read-all`                 | PATCH  | ✅       | ✅       | 均支持；OwlMail 返回 `{message,count}`。                                                                                              |
| `/email/:id/relay/:relayTo?`      | POST   | ✅       | ✅       | 二者均支持 URL 参数；OwlMail 还支持请求体 `relayTo`。                                                                                         |
| `/config`                         | GET    | ✅       | ✅       | OwlMail 信息更丰富（嵌套结构）；可通过适配层输出 MailDev 扁平字段。                                                                                     |
| `/healthz`                        | GET    | ✅       | ✅       | MailDev 返回 `true`，OwlMail 返回 `{status:"ok"}`。                                                                                  |
| `/reloadMailsFromDirectory`       | GET    | ✅       | ✅       | 功能一致；OwlMail 新 API 倾向 `POST` 更语义化。                                                                                             |
| `/socket.io`                      | WS     | ✅       | ✅       | 路径兼容，协议不同：Socket.IO vs 原生 WS；事件语义一致。                                                                                           |

**代码定位（节选）**

* MailDev：`lib/routes.js`, `lib/mailserver.js`, `lib/outgoing.js`, `lib/web.js`
* OwlMail：`internal/api/api_*.go`, `internal/mailserver/*.go`, `internal/outgoing/*.go`

---

## 🆙 OwlMail 增强能力

### A. 新增/改进的 API（MailDev 未提供）

| 端点                    | 方法            | 说明                         |
| --------------------- | ------------- | -------------------------- |
| `/email/:id/read`     | PATCH         | 标记单封为已读（MailDev 逻辑为取详情即已读） |
| `/email/stats`        | GET           | 邮件统计信息                     |
| `/email/preview`      | GET           | 轻量预览列表                     |
| `/email/batch/delete` | POST          | 批量删除                       |
| `/email/batch/read`   | POST          | 批量已读                       |
| `/email/export`       | GET           | 导出所有邮件 ZIP                 |
| `/config/outgoing`    | GET/PUT/PATCH | 出站配置管理                     |

### B. 改进的 RESTful 版本路由（`/api/v1/*`）

* 复数资源、动作语义化、版本化治理：

  * `/api/v1/emails[/:id]`、`/api/v1/emails/batch`、`/api/v1/emails/:id/actions/relay`、`/api/v1/emails/stats|preview|export`、`/api/v1/settings*`、`/api/v1/health`、`/api/v1/ws` 等。

### C. SMTP/安全增强

* **SMTPS(465)**：OwlMail 原生支持（MailDev 无）。
* TLS/STARTTLS 配置与认证项更细化。

---

## 🔧 功能实现与差异要点

### 1) SMTP/收信

| 功能             | MailDev         | OwlMail       | 备注             |
| -------------- | --------------- | ------------- | -------------- |
| SMTP 服务器       | ✅ `smtp-server` | ✅ `go-smtp`   | 完全等价           |
| 端口/绑定/持久化      | ✅               | ✅             | `.eml` 文件存储、一致 |
| 从目录加载          | ✅               | ✅             | 一致             |
| SMTP 认证        | ✅ PLAIN/LOGIN   | ✅ PLAIN/LOGIN | 一致             |
| TLS/STARTTLS   | ✅               | ✅             | 一致             |
| **SMTPS(465)** | ❌               | ✅             | **OwlMail 独有** |

### 2) 过滤/搜索/分页

* **MailDev**：点号语法（如 `from.address=value`）、`skip` 偏移。
* **OwlMail**：在兼容基础上扩展 `q/from/to/dateFrom/dateTo/read/limit/offset/sortBy/sortOrder`，并返回 `{total,limit,offset}`。

### 3) 已读语义

* **MailDev**：`GET /email/:id` 即置已读。
* **OwlMail**：显式 `PATCH /email/:id/read`。
* **影响**：低；可通过一次额外调用对齐行为。

### 4) 状态码与返回体

* **删除不存在**：MailDev 常见 500；OwlMail 为 404（更 REST）。
* **返回体**：MailDev 偏 `true`/数字；OwlMail 多返回 JSON 对象（含 message/count）。

### 5) WebSocket 协议

* **MailDev**：Socket.IO（自动重连/房间等生态能力）。
* **OwlMail**：原生 WebSocket（消息如 `{type:"new"|"delete", email}`）。
* **兼容性**：事件含义一致；**客户端需改造**。

---

## 🌱 环境变量兼容（OwlMail 优先识别 MailDev 变量）

> OwlMail 全量映射 MailDev 环境变量；若未设置，则回退到 OwlMail 自有变量名。

| MailDev 环境变量               | OwlMail 环境变量                | 说明         |
| -------------------------- | --------------------------- | ---------- |
| `MAILDEV_SMTP_PORT`        | `OWLMAIL_SMTP_PORT`         | SMTP 端口    |
| `MAILDEV_IP`               | `OWLMAIL_SMTP_HOST`         | SMTP 主机    |
| `MAILDEV_MAIL_DIRECTORY`   | `OWLMAIL_MAIL_DIR`          | 邮件目录       |
| `MAILDEV_WEB_PORT`         | `OWLMAIL_WEB_PORT`          | Web 端口     |
| `MAILDEV_WEB_IP`           | `OWLMAIL_WEB_HOST`          | Web 主机     |
| `MAILDEV_WEB_USER`         | `OWLMAIL_WEB_USER`          | Web 认证用户   |
| `MAILDEV_WEB_PASS`         | `OWLMAIL_WEB_PASSWORD`      | Web 认证密码   |
| `MAILDEV_HTTPS`            | `OWLMAIL_HTTPS_ENABLED`     | 启用 HTTPS   |
| `MAILDEV_HTTPS_CERT`       | `OWLMAIL_HTTPS_CERT`        | HTTPS 证书   |
| `MAILDEV_HTTPS_KEY`        | `OWLMAIL_HTTPS_KEY`         | HTTPS 私钥   |
| `MAILDEV_OUTGOING_HOST`    | `OWLMAIL_OUTGOING_HOST`     | 出站 SMTP 主机 |
| `MAILDEV_OUTGOING_PORT`    | `OWLMAIL_OUTGOING_PORT`     | 出站 SMTP 端口 |
| `MAILDEV_OUTGOING_USER`    | `OWLMAIL_OUTGOING_USER`     | 出站 SMTP 用户 |
| `MAILDEV_OUTGOING_PASS`    | `OWLMAIL_OUTGOING_PASSWORD` | 出站 SMTP 密码 |
| `MAILDEV_OUTGOING_SECURE`  | `OWLMAIL_OUTGOING_SECURE`   | 出站 TLS     |
| `MAILDEV_AUTO_RELAY`       | `OWLMAIL_AUTO_RELAY`        | 自动中继开关     |
| `MAILDEV_AUTO_RELAY_ADDR`  | `OWLMAIL_AUTO_RELAY_ADDR`   | 自动中继地址     |
| `MAILDEV_AUTO_RELAY_RULES` | `OWLMAIL_AUTO_RELAY_RULES`  | 自动中继规则文件   |
| `MAILDEV_INCOMING_USER`    | `OWLMAIL_SMTP_USER`         | 入站 SMTP 用户 |
| `MAILDEV_INCOMING_PASS`    | `OWLMAIL_SMTP_PASSWORD`     | 入站 SMTP 密码 |
| `MAILDEV_INCOMING_SECURE`  | `OWLMAIL_TLS_ENABLED`       | 入站 TLS 开关  |
| `MAILDEV_INCOMING_CERT`    | `OWLMAIL_TLS_CERT`          | 入站 TLS 证书  |
| `MAILDEV_INCOMING_KEY`     | `OWLMAIL_TLS_KEY`           | 入站 TLS 私钥  |
| `MAILDEV_VERBOSE`          | `OWLMAIL_LOG_LEVEL=verbose` | 详细日志       |
| `MAILDEV_SILENT`           | `OWLMAIL_LOG_LEVEL=silent`  | 静默日志       |

**使用示例**

```bash
# 直接沿用 MailDev 环境变量（零改造）
export MAILDEV_SMTP_PORT=1025
export MAILDEV_WEB_PORT=1080
export MAILDEV_OUTGOING_HOST=smtp.gmail.com
./owlmail

# 或使用 OwlMail 自有变量（回退方案）
export OWLMAIL_SMTP_PORT=1025
export OWLMAIL_WEB_PORT=1080
./owlmail
```

---

## 📊 性能与部署

| 指标   | OwlMail | MailDev | 说明                |
| ---- | ------- | ------- | ----------------- |
| 启动速度 | ⚡ 快     | 🐢 较慢   | 二进制直启 vs 需 JS 运行时 |
| 内存占用 | 💚 低    | 🟡 中    | Go 运行时占用更低        |
| 并发处理 | 💚 优    | 🟡 良    | Goroutine 并发优势    |
| 资源消耗 | 💚 低    | 🟡 中    | 单一可执行文件           |
| 部署便捷 | 💚 优    | 🟡 良    | 无运行时依赖/镜像更小       |

---

## 🔄 迁移策略（含测试清单）

### 方案 1｜**完全替换（推荐）**

**适用**：主要经由 API/CLI 调用；可适配 WebSocket 客户端。

1. 停止 MailDev；以**相同环境变量**启动 OwlMail。
2. 验证 API：`/email`、`/healthz`、下载/附件/源码等。
3. WebSocket 客户端从 Socket.IO 适配为原生 **WS**。

### 方案 2｜渐进式替换

**适用**：需要原 MailDev 前端体验。

1. 复用 MailDev 前端静态文件；后端切换到 OwlMail。
2. 仅调整前端 WS 连接（`io()` → `new WebSocket()`）。

### 方案 3｜混合模式

**适用**：前端保留 MailDev，后端使用 OwlMail API。

### 测试清单（示例）

```bash
# API 端点冒烟
curl -s http://localhost:1080/healthz
curl -s http://localhost:1080/email
curl -s http://localhost:1080/email/:id
curl -s http://localhost:1080/email/:id/html
curl -s http://localhost:1080/email/:id/download

# 环境变量兼容
MAILDEV_SMTP_PORT=1025 MAILDEV_WEB_PORT=1080 ./owlmail

# 发信验证（任选）
echo "Test" | mail -s "Test" test@localhost
# 或 sendmail
sendmail -S localhost:1025 test@localhost <<'EOF'
Subject: Test Email
From: sender@example.com
To: test@localhost

This is a test email.
EOF

# 转发验证
EMAIL_ID=$(curl -s http://localhost:1080/email | jq -r '.[0].id // .emails[0].id')
curl -X POST "http://localhost:1080/email/${EMAIL_ID}/relay/recipient@example.com"
```

**WebSocket 客户端适配对照**

```js
// MailDev（Socket.IO）
const socket = io('/socket.io');
socket.on('newMail', (email) => { /* ... */ });
socket.on('deleteMail', (data) => { /* ... */ });

// OwlMail（原生 WebSocket）
const ws = new WebSocket('ws://localhost:1080/socket.io');
ws.onmessage = (ev) => {
  const data = JSON.parse(ev.data);
  if (data.type === 'new') { /* ... */ }
  if (data.type === 'delete') { /* ... */ }
};
```

---

## 🧩 自动中继（Relay）规则与转发

* **规则格式**：两者完全一致（JSON），**最后匹配生效**；支持 `*` 通配。

```json
[
  { "allow": "*" },
  { "deny": "*@test.com" },
  { "allow": "ok@test.com" }
]
```

* **能力等价**：外发配置/认证/TLS/按地址转发/自动中继均一致；OwlMail 额外支持以请求体传入 `relayTo`。

---

## 📝 兼容性矩阵（汇总）

### API 端点

| 端点                                | 兼容性        | 备注                               |         |       |         |
| --------------------------------- | ---------- | -------------------------------- | ------- | ----- | ------- |
| GET `/email`                      | ⭐⭐⭐⭐⭐      | OwlMail 功能更强，返回分页元数据             |         |       |         |
| GET `/email/:id`                  | ⭐⭐⭐⭐       | MailDev 取详情即已读；OwlMail 需额外 PATCH |         |       |         |
| DELETE `/email/:id`               | ⭐⭐⭐⭐       | 状态码/返回体轻微差异                      |         |       |         |
| DELETE `/email/all`               | ⭐⭐⭐⭐⭐      | 等价                               |         |       |         |
| PATCH `/email/read-all`           | ⭐⭐⭐⭐⭐      | 等价                               |         |       |         |
| GET `/email/:id/html              | attachment | download                         | source` | ⭐⭐⭐⭐⭐ | 等价或体验增强 |
| POST `/email/:id/relay/:relayTo?` | ⭐⭐⭐⭐⭐      | 等价；OwlMail 还支持 JSON 体            |         |       |         |
| GET `/config`                     | ⭐⭐⭐⭐       | OwlMail 字段更结构化（可做适配层）            |         |       |         |
| GET `/healthz`                    | ⭐⭐⭐⭐       | 返回体不同，不影响集成                      |         |       |         |
| GET `/reloadMailsFromDirectory`   | ⭐⭐⭐⭐⭐      | 等价；新 API 倾向 POST                 |         |       |         |
| `WS /socket.io`                   | ⭐⭐⭐        | 协议不同需适配，但事件等价                    |         |       |         |

### 功能

| 功能               | 兼容性   | 备注         |
| ---------------- | ----- | ---------- |
| SMTP/存储/解析/附件    | ⭐⭐⭐⭐⭐ | 等价         |
| 自动中继/转发          | ⭐⭐⭐⭐⭐ | 等价（规则完全兼容） |
| 认证/TLS/HTTPS     | ⭐⭐⭐⭐⭐ | 等价         |
| **SMTPS(465)**   | 🆕    | OwlMail 独有 |
| 批量/统计/导出/改进 REST | 🆕    | OwlMail 增强 |
| WebSocket        | ⚠️    | 协议不同、功能一致  |

---

## 🎯 推荐与落地

**优先推荐使用 OwlMail**（API 驱动、注重性能与部署、需要批量/导出/统计/SMTPS 的团队）。若前端强依赖 Socket.IO 或需 MailDev 完整 UI，可采用“渐进式/混合模式”过渡，渐进调整 WebSocket 客户端。

---

## 📚 代码位置参考（统一索引）

* **MailDev**：

  * `lib/routes.js`（REST 路由）
  * `lib/mailserver.js`（SMTP/解析）
  * `lib/outgoing.js`（转发/规则）
  * `lib/web.js`（Socket.IO）
* **OwlMail**：

  * `internal/api/api_emails.go`、`api_config.go`、`api_relay.go`、`api_websocket.go`
  * `internal/mailserver/session.go`、`store.go`
  * `internal/outgoing/outgoing.go`
  * 改进 REST：`/api/v1/*`（`api.go` / `setupImprovedAPIRoutes()`）

---

## 🏁 结论（合并版）

* **功能一致性：⭐⭐⭐⭐⭐** — 核心能力 100% 对齐；OwlMail 提供多项增强。
* **替换可行性：⭐⭐⭐⭐⭐** — 大多数场景可零配置替换；仅需 WS 客户端轻量适配。
* **环境变量兼容：⭐⭐⭐⭐⭐** — 直接沿用现有 MailDev 配置即可。
* **加分项** — 性能/部署优势显著；SMTPS 与批量/统计/导出与更规范的 REST 进一步提升工程体验。

---

**报告生成时间**：2025 年 11 月 10 日
**OwlMail 版本**：1.0+
**MailDev 版本**：2.2.1
