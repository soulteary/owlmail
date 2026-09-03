# OwlMail 0.7.0 发布说明

OwlMail 0.7.0 为测试网关增加只读 MCP 工作流、明确的 MailDev REST 兼容层、完整
原生 OpenAPI 契约、sendmail 风格提交、S3 附件存储、SMTP 资源控制与安全 HTML
预览隔离。

OwlMail 0.7.0 已于 2026-09-02 发布。

## 重点能力

### Agent 与兼容接口

- 可选 Streamable HTTP MCP，提供有界只读列表、搜索、详情、Source、附件元数据、
  最新邮件和投递等待。
- 可选 `/api` MailDev REST Facade；Socket.IO 与 Node 嵌入 API 不属于兼容契约。
- 原生 OpenAPI 3.1 JSON/YAML 与路由漂移测试。
- 面向 PHP、Cron 和传统程序的 `owlmail sendmail`。

### 存储与入口

- 可选 S3 兼容解码附件存储，支持事务上传、校验、回滚、就绪探测与离线迁移。
- 可配置邮件大小、收件人数、SMTP 读写超时和进程级 DATA 并发。
- 真实 PLAIN/LOGIN SMTP AUTH 与可选 TLS 强制策略。
- Base path 覆盖 Web、API、WebSocket、附件和 MCP。

### 浏览器安全与规模

HTML 预览组合服务端安全化、零权限 iframe、no-referrer、严格 CSP 与默认阻止远程
内容。邮箱列表使用紧凑预览和有界快照查询。

## 升级注意

- 默认邮件上限由 1 MiB 调整为 100 MiB。
- 同时设置两项 SMTP 凭据后强制 AUTH；只设置一项会启动失败。
- MCP 与两个兼容 Facade 都保持默认关闭。
- S3 迁移应在可写 OwlMail 进程停止时执行。
- 远程内容按邮件显式加载。

## 安装

```bash
docker pull ghcr.io/soulteary/owlmail:0.7.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  ghcr.io/soulteary/owlmail:0.7.0
```

Registry 标签是名称，不是内容标识。精确部署请使用已发布的
`ghcr.io/soulteary/owlmail@sha256:<digest>`。

## 已知边界

- 0.7.0 的 MCP 使用 Streamable HTTP；stdio bridge 在 0.8.0 引入。
- MailDev 兼容层不包含 Socket.IO。
- 邮箱和存储状态属于单个可写 OwlMail 实例。
- Redis 支持的 Webhook 在故障与恢复边界使用至少一次投递。

参阅 [0.7.0 变更日志](../../CHANGELOG.md#070---2026-09-02)、
[API 参考](./API-Reference.md)与[运维与排障](./Operations.md)。
