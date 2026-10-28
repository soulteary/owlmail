# 配置参考

[English](../en/Configuration-Reference.md) · [简体中文](./Configuration-Reference.md)

本页列出 OwlMail **0.12.0** 的参数、环境变量别名和内置默认值，包括 `-disable-web` SMTP-only 模式。安装方式见
[README 快速开始](../../README.zh-CN.md#快速开始收到第一封邮件)。

## 配置优先级

命令行优先于已支持的 MailDev 环境变量别名，其次是 OwlMail 环境变量、平面
YAML / JSON 配置文件和内置默认值。文件键名使用去掉开头 `-` 的参数名。
示例和启动行为见[运维指南](./Operations.md)。

## 参数与环境变量

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `-smtp` | `MAILDEV_SMTP_PORT` / `OWLMAIL_SMTP_PORT` | 1025 | SMTP 端口；支持 `0` 自动分配端口 |
| `-ip` | `MAILDEV_IP` / `OWLMAIL_SMTP_HOST` | localhost | SMTP 主机 |
| `-smtp-max-message-mb` | `OWLMAIL_SMTP_MAX_MESSAGE_MB` | 100 | 单封入站邮件上限，单位 MiB |
| `-smtp-max-concurrency` | `OWLMAIL_SMTP_MAX_CONCURRENCY` | 8 | 每进程跨 SMTP、STARTTLS 与 SMTPS 的并发 DATA 事务数；`0` 表示不限制；达到上限返回可重试的 `451 4.3.2` |
| `-smtp-read-timeout` | `OWLMAIL_SMTP_READ_TIMEOUT` | 10s | SMTP 命令与 DATA 读取超时 |
| `-smtp-write-timeout` | `OWLMAIL_SMTP_WRITE_TIMEOUT` | 10s | SMTP 响应写入超时 |
| `-smtp-max-recipients` | `OWLMAIL_SMTP_MAX_RECIPIENTS` | 50 | 单封邮件允许的最大收件人数 |
| `-disable-web` | `OWLMAIL_DISABLE_WEB` | false | 仅运行 SMTP，不启动 HTTP 监听器 |
| `-web` | `MAILDEV_WEB_PORT` / `OWLMAIL_WEB_PORT` | 1080 | Web API 端口；支持 `0` 自动分配端口 |
| `-web-ip` | `MAILDEV_WEB_IP` / `OWLMAIL_WEB_HOST` | localhost | Web API 主机 |
| `-web-external-url` | `OWLMAIL_WEB_EXTERNAL_URL` | - | 生成邮件深链接时使用的浏览器可见 HTTP(S) origin；反向代理子路径仍通过 `-base-pathname` 配置 |
| `-web-allowed-origins` | `OWLMAIL_WEB_ALLOWED_ORIGINS` | - | 除 OwlMail 自身来源外，Web 界面与 REST API 额外接受的浏览器来源；`*` 关闭来源校验 |
| `-base-pathname` | `MAILDEV_BASE_PATHNAME` / `OWLMAIL_BASE_PATHNAME` | - | URL 子路径前缀，例如 `/owlmail`；默认仍为根路径 |
| `-maildev-rest-compat` | `OWLMAIL_MAILDEV_REST_COMPAT` | false | 显式启用 MailDev `/api` REST 兼容 facade；仍不支持 Socket.IO |
| `-metrics-enabled` | `OWLMAIL_METRICS_ENABLED` | false | 在跟随基础路径的 `/metrics` 端点暴露 Prometheus 指标；配置 Web Basic Auth 后同样受其保护 |
| `-mcp-enabled` | `OWLMAIL_MCP_ENABLED` | false | 在 `/mcp` 启用只读 MCP Streamable HTTP 端点 |
| `-mcp-session-timeout` | `OWLMAIL_MCP_SESSION_TIMEOUT` | 30m | 关闭空闲旧版 MCP 会话并限制 wait |
| `-mcp-shutdown-timeout` | `OWLMAIL_MCP_SHUTDOWN_TIMEOUT` | 5s | 关闭时清理 MCP 工作的期限 |
| `-mcp-allowed-origins` | `OWLMAIL_MCP_ALLOWED_ORIGINS` | - | 除 OwlMail 自身来源外,`/mcp` 额外接受的浏览器来源;`*` 关闭来源校验 |
| `-mail-directory` | `MAILDEV_MAIL_DIRECTORY` / `OWLMAIL_MAIL_DIR` | - | 邮件存储目录 |
| `-mail-retention-days` | `OWLMAIL_MAIL_RETENTION_DAYS` | 0 | 删除超过 N 天的邮件；`0` 表示不限 |
| `-mail-max-messages` | `OWLMAIL_MAIL_MAX_MESSAGES` | 0 | 最大邮件封数；`0` 表示不限 |
| `-mail-max-disk-mb` | `OWLMAIL_MAIL_MAX_DISK_MB` | 0 | 邮箱最大磁盘 MiB；`0` 表示不限 |
| `-mail-cleanup-interval` | `OWLMAIL_MAIL_CLEANUP_INTERVAL` | 1h | 后台保留策略清理间隔 |
| `-mail-index-path` | `OWLMAIL_MAIL_INDEX_PATH` | - | 可选的 SQLite 邮箱查询索引路径；索引可重建，EML 文件仍是权威数据源 |
| `-s3-enabled` | `OWLMAIL_S3_ENABLED` | false | 将解析后的附件存入 S3 兼容对象存储 |
| `-s3-endpoint` | `OWLMAIL_S3_ENDPOINT` | - | 自定义 S3 兼容端点；留空使用 AWS S3 |
| `-s3-region` | `OWLMAIL_S3_REGION` | us-east-1 | S3 签名区域 |
| `-s3-bucket` | `OWLMAIL_S3_BUCKET` | - | 用于附件的已有存储桶 |
| `-s3-prefix` | `OWLMAIL_S3_PREFIX` | owlmail/attachments | 附件对象键前缀 |
| `-s3-access-key` | `OWLMAIL_S3_ACCESS_KEY` | - | 可选静态访问密钥；留空使用 AWS 凭据链 |
| `-s3-secret-key` | `OWLMAIL_S3_SECRET_KEY` | - | 可选静态秘密密钥 |
| `-s3-session-token` | `OWLMAIL_S3_SESSION_TOKEN` | - | 可选静态凭据会话令牌 |
| `-s3-use-path-style` | `OWLMAIL_S3_USE_PATH_STYLE` | false | 为兼容服务使用路径式存储桶寻址 |
| `-s3-startup-check` | `OWLMAIL_S3_STARTUP_CHECK` | false | 首次只读 S3 存储桶探测失败时终止启动 |
| `-s3-health-check-interval` | `OWLMAIL_S3_HEALTH_CHECK_INTERVAL` | 30s | 后台刷新 S3 readiness 的间隔 |
| `-s3-health-check-timeout` | `OWLMAIL_S3_HEALTH_CHECK_TIMEOUT` | 5s | 单次 S3 readiness 探测超时 |
| `-web-user` | `MAILDEV_WEB_USER` / `OWLMAIL_WEB_USER` | - | HTTP Basic Auth 用户名 |
| `-web-password` | `MAILDEV_WEB_PASS` / `OWLMAIL_WEB_PASSWORD` | - | HTTP Basic Auth 密码 |
| `-https` | `MAILDEV_HTTPS` / `OWLMAIL_HTTPS_ENABLED` | false | 启用 HTTPS |
| `-https-cert` | `MAILDEV_HTTPS_CERT` / `OWLMAIL_HTTPS_CERT` | - | HTTPS 证书文件 |
| `-https-key` | `MAILDEV_HTTPS_KEY` / `OWLMAIL_HTTPS_KEY` | - | HTTPS 私钥文件 |
| `-outgoing-host` | `MAILDEV_OUTGOING_HOST` / `OWLMAIL_OUTGOING_HOST` | - | 出站 SMTP 主机 |
| `-outgoing-port` | `MAILDEV_OUTGOING_PORT` / `OWLMAIL_OUTGOING_PORT` | 587 | 出站 SMTP 端口 |
| `-outgoing-user` | `MAILDEV_OUTGOING_USER` / `OWLMAIL_OUTGOING_USER` | - | 出站 SMTP 用户名 |
| `-outgoing-pass` | `MAILDEV_OUTGOING_PASS` / `OWLMAIL_OUTGOING_PASSWORD` | - | 出站 SMTP 密码 |
| `-outgoing-secure` | `MAILDEV_OUTGOING_SECURE` / `OWLMAIL_OUTGOING_SECURE` | false | MailDev 兼容的隐式 TLS/SMTPS 开关 |
| `-outgoing-tls-mode` | `OWLMAIL_OUTGOING_TLS_MODE` | - | 传输模式：未设置时为 `plain`，也可选择强制 `starttls` 或隐式 `smtps` |
| `-outgoing-insecure-skip-verify` | `OWLMAIL_OUTGOING_INSECURE_SKIP_VERIFY` | false | 跳过证书和主机名验证（不安全，必须显式开启） |
| `-outgoing-connect-timeout` | `OWLMAIL_OUTGOING_CONNECT_TIMEOUT` | 10s | TCP 连接和 SMTP 问候超时 |
| `-outgoing-tls-handshake-timeout` | `OWLMAIL_OUTGOING_TLS_HANDSHAKE_TIMEOUT` | 10s | TLS/STARTTLS 握手超时 |
| `-outgoing-auth-timeout` | `OWLMAIL_OUTGOING_AUTH_TIMEOUT` | 10s | AUTH 超时 |
| `-outgoing-envelope-timeout` | `OWLMAIL_OUTGOING_ENVELOPE_TIMEOUT` | 10s | MAIL/RCPT 超时 |
| `-outgoing-data-timeout` | `OWLMAIL_OUTGOING_DATA_TIMEOUT` | 30s | DATA 写入和确认超时 |
| `-outgoing-quit-timeout` | `OWLMAIL_OUTGOING_QUIT_TIMEOUT` | 5s | QUIT 超时 |
| `-auto-relay` | `MAILDEV_AUTO_RELAY` / `OWLMAIL_AUTO_RELAY` | false | 启用自动中继 |
| `-auto-relay-addr` | `MAILDEV_AUTO_RELAY_ADDR` / `OWLMAIL_AUTO_RELAY_ADDR` | - | 自动中继地址 |
| `-auto-relay-rules` | `MAILDEV_AUTO_RELAY_RULES` / `OWLMAIL_AUTO_RELAY_RULES` | - | 自动中继规则文件 |
| `-webhook-config` | `OWLMAIL_WEBHOOK_CONFIG` | - | Webhook 消息转发 JSON 配置文件 |
| `-webhook-max-concurrency` | `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` | 8 | Webhook 邮件并发投递数；`0` 表示不限制 |
| `-webhook-redis-url` | `OWLMAIL_WEBHOOK_REDIS_URL` | - | 用于持久、可跨重启投递的 Redis URL |
| `-webhook-redis-prefix` | `OWLMAIL_WEBHOOK_REDIS_PREFIX` | owlmail:webhooks | Redis Streams 键前缀 |
| `-webhook-shutdown-timeout` | `OWLMAIL_WEBHOOK_SHUTDOWN_TIMEOUT` | 15s | Webhook 优雅排空截止时间 |
| `-smtp-user` | `MAILDEV_INCOMING_USER` / `OWLMAIL_SMTP_USER` | - | 入站 SMTP 用户名；与密码同时配置后强制 AUTH |
| `-smtp-password` | `MAILDEV_INCOMING_PASS` / `OWLMAIL_SMTP_PASSWORD` | - | 入站 SMTP 密码；与用户名同时配置后强制 AUTH |
| `-smtp-auth-require-tls` | `OWLMAIL_SMTP_AUTH_REQUIRE_TLS` | false | TLS 建立前拒绝 PLAIN/LOGIN；必须同时启用 SMTP TLS |
| `-tls` | `MAILDEV_INCOMING_SECURE` / `OWLMAIL_TLS_ENABLED` | false | 启用 SMTP TLS |
| `-tls-cert` | `MAILDEV_INCOMING_CERT` / `OWLMAIL_TLS_CERT` | - | SMTP TLS 证书文件 |
| `-tls-key` | `MAILDEV_INCOMING_KEY` / `OWLMAIL_TLS_KEY` | - | SMTP TLS 私钥文件 |
| `-smtps-port` | `OWLMAIL_SMTPS_PORT` | 465 | 启用 TLS 时隐式 TLS（SMTPS）监听端口；设为 `0` 则不启动 SMTPS 监听 |
| `-log-level` | `MAILDEV_VERBOSE` / `MAILDEV_SILENT` / `OWLMAIL_LOG_LEVEL` | normal | 日志级别 |
| `-mailcatcher-rest-compat` | `OWLMAIL_MAILCATCHER_REST_COMPAT` | false | 启用可选的 MailCatcher REST 兼容接口 |
| `-config` | `OWLMAIL_CONFIG_FILE` | - | 启动时读取 YAML 或 JSON 配置文件；命令行和环境变量优先 |
| `-listen-info-file` | `OWLMAIL_LISTEN_INFO_FILE` | - | 将实际监听地址原子写入私有 JSON 文件；每个实例使用独立路径 |
| `-log-format` | `OWLMAIL_LOG_FORMAT` | console | 日志输出格式：`console` 或 `json` |
| `-use-uuid-for-email-id` | `OWLMAIL_USE_UUID_FOR_EMAIL_ID` | false | 使用 UUID 作为邮件 ID（默认使用 8 字符随机字符串） |

由反向代理终止 TLS 时，可用 `OWLMAIL_WEB_EXTERNAL_SCHEME` 指定对外协议。
明确的公网 origin 使用 `-web-external-url`，代理子路径使用 `-base-pathname`。

MCP、指标、REST 兼容层、S3、Webhook 和 Relay 均需显式启用。只有说明中明确指出
的参数才以 `0` 表示不限。默认 SMTP 使用 NO AUTH；同时配置 `-smtp-user` 与
`-smtp-password` 才会要求认证。

[运维与排障](./Operations.md)说明 TLS、认证、存储、保留策略、启动诊断与有序关闭。
