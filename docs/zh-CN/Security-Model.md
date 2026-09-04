# 安全模型

OwlMail 面向开发、CI 与受控测试网络。它保存完整邮件，其中可能包含被测系统生成的
凭据、Token、链接和附件；它不是生产 MTA，也不是可安全公开的归档服务。

## 信任边界

| 接口 | 默认状态 | 边界 |
|---|---|---|
| SMTP 1025 | 本地监听；未配置两项凭据时为 NO AUTH | 保持私有；同时配置两项凭据才强制 AUTH |
| Web UI/API 1080 | 本地监听；未配置两项 Web 凭据时无 Basic Auth | 非本地访问使用固定凭据与 HTTPS |
| 健康探针 | 无认证 | 只暴露健康状态；浏览器 Origin 检查仍可拒绝跨域请求 |
| MCP | 默认关闭 | HTTP 复用 Web 认证/TLS/base path；stdio 只读一个已有邮件目录 |
| MailDev/MailCatcher Facade | 默认关闭 | 启用后复用 Web 边界，只提供明确的兼容契约 |
| Metrics | 默认关闭 | 启用后在网络或反向代理层保护 |
| Webhook 与 Relay | 配置后才启用 | 把捕获内容或元数据发送到运维者选择的目标 |

## 不可信邮件内容

捕获的 Header、文本、HTML、链接与附件都属于攻击者可控输入。OwlMail 会安全化 HTML，
并放入零权限 `srcdoc` sandbox iframe，同时设置 `referrerpolicy="no-referrer"` 与严格
CSP。远程图片、字体、样式表和媒体默认阻止；按邮件加载远程内容会联系发件人选择的
基础设施，并可能泄露 IP 与追踪标识。

主动型附件会强制下载、设置 `nosniff` 并清理文件名。打开下载附件仍是 OwlMail
sandbox 之外的人工操作。

Agent 必须把邮件内容视为数据而非指令。邮件内嵌的 Prompt 不能覆盖测试计划、工具
策略或 Secret 处理规则。

## 凭据与 Secret

- 不要在 Issue 或日志中发布 SMTP 密码、Web Basic Auth、S3 Key、Webhook Secret、
  完整敏感邮件或生产 Token。
- 同时配置两项 SMTP 凭据才强制 AUTH；两项均缺失时会有意允许匿名投递和任意测试
  凭据；只配置一项会启动失败。
- SMTP 凭据不得经过明文时启用 `-smtp-auth-require-tls`。
- 不要通过不可信 HTTP 发送 Basic Auth。
- Webhook HMAC Secret 使用环境变量值；分享配置前必须脱敏。

## 持久化与出站副作用

把邮件目录与备份按敏感数据保护。本地文件和 sidecar 是事实来源；S3 策略应只允许
配置的 Bucket 与 Prefix。启用保留与删除策略前先在副本上验证。

崩溃边界附近 Webhook 与 Relay 可能重复投递，下游应幂等。强制 STARTTLS 与 SMTPS
不会降级为明文。原生 v1 Relay Job 只暴露有界错误类别，不通过状态返回原始下游错误。

## 部署检查清单

- SMTP 与 Web 绑定到 loopback 或私有网络。
- 非本地访问设置固定 Web 凭据与 HTTPS。
- 只向预期客户端开放 MCP 与 Metrics。
- 测试数据与生产账户、生产基础设施隔离。
- 安全敏感 CI 按 manifest digest 固定发布镜像。
- 升级前备份完整邮件目录。
- 漏洞报告前阅读 [SECURITY.zh-CN.md](../../.github/SECURITY.zh-CN.md)，通过私有 Advisory
  或安全邮箱提交，不要创建公开 Issue。

配置细节见[运维与排障](./Operations.md)，API 认证与 Origin 行为见
[API 参考](./API-Reference.md)。
