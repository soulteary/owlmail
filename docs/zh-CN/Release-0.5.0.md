# OwlMail 0.5.0 发布说明

OwlMail 0.5.0 在保持单一二进制部署方式的同时，把本地收件箱扩展为更完整的
集成端点。本版本新增可配置 Webhook 转发、内置帮助、按需浏览器通知、明确的
Webhook 容量控制，并改进了 Web Basic Auth 只配置一项凭据时的行为。

只有在发布 `v0.5.0` 标签后，引用 `v0.5.0` 或容器标签 `0.5.0` 的命令才会生效。

## 主要更新

### Webhook 消息转发

新存储的邮件可发送到通用 HTTP 端点。版本 1 配置支持 1～32 个具名目标，包括：

- 针对发件人、收件人和主题的不区分大小写通配符过滤；
- 默认载荷、自定义 JSON 安全模板或纯文本请求体；
- 从环境变量加载值；
- 自定义请求头、HMAC-SHA256 签名、超时与重试；
- 多个相互独立的目标；
- 可直接运行的 `soulteary/webhook` Compose 联动示例。

使用 `-webhook-config` 或 `OWLMAIL_WEBHOOK_CONFIG` 指定 JSON 文件。进程级
`-webhook-max-concurrency` / `OWLMAIL_WEBHOOK_MAX_CONCURRENCY` 默认值为 `8`；
只有明确需要无限投递并发时才设置为 `0`。

### 内置运维帮助

收件箱新增入口，可打开 `/help` 的中英文本地指南。HTML、CSS 与 JavaScript
均嵌入可执行文件，二进制和容器部署无需额外携带 `web` 目录。

### 浏览器通知

通知默认关闭，每个浏览器都必须由用户点击开启。只有通过实时 WebSocket 到达的
新邮件会触发通知；功能要求 HTTPS 或可信本地来源，且通知不会包含邮件正文。

### Web 鉴权默认策略

Web Basic Auth 只配置一项时不再静默关闭鉴权：

| 已配置内容 | 最终行为 |
|---|---|
| 两项均未配置 | 关闭鉴权 |
| 只配置用户名 | 保留用户名，生成 32 字符随机密码，并在 stderr 输出一次 |
| 只配置密码 | 使用用户名 `admin` 和已配置密码 |
| 两项均配置 | 原样使用两项配置 |

带浏览器 `Origin` 请求头的已鉴权请求必须来自 OwlMail 自身源。Basic Auth 仍只应
用于 localhost 或 HTTPS。

## 升级前需要确认的行为

| 范围 | 0.5.0 行为 | 运维建议 |
|---|---|---|
| Webhook 饱和 | 有限并发会在创建处理 goroutine 前施加背压，可能延迟 SMTP `DATA` 完成 | 从 `8` 开始，并一起评估超时、重试和并发 |
| Webhook 关闭 | 进程退出前不会排空正在进行的投递 | 先停止新 SMTP 流量，再等待最长重试窗口 |
| Web 凭据 | 只配置一项会生成可用凭据，不再静默关闭鉴权 | 从 stderr 读取临时凭据，或明确配置两项 |
| 浏览器通知 | 权限与偏好保存在单个浏览器 | 在 HTTPS 或 localhost 下逐个浏览器开启 |
| MailDev 客户端 | 保留 MailDev 风格工作流路由，但不完全实现当前 MailDev API 或 Socket.IO 协议 | 验证路径、载荷、已读副作用和 WebSocket 客户端 |

更换版本前应备份完整邮件目录；重要归档应先使用副本验证。

## 发布后的安装方式

### 发布二进制

发布工作流会上传 5 个可执行文件与 `checksums.txt`：

| 平台 | 文件 |
|---|---|
| Linux amd64 | `owlmail-linux-amd64` |
| Linux arm64 | `owlmail-linux-arm64` |
| macOS amd64 | `owlmail-darwin-amd64` |
| macOS arm64 | `owlmail-darwin-arm64` |
| Windows amd64 | `owlmail-windows-amd64.exe` |

Linux amd64 示例：

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.5.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.5.0/checksums.txt
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

### Go 安装

从源码安装需要 Go 1.27.0 或更高版本：

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@v0.5.0
```

下载的发布二进制在运行时不需要 Go 或 Node.js。

发布二进制与镜像会嵌入 `version`、`commit`、`build_date` 和来源标签，可通过
`GET /api/v1/version` 检查；发布工作流会在上传文件前冒烟验证版本与提交。

### 容器镜像

发布部署应使用对应版本标签：

```bash
docker pull ghcr.io/soulteary/owlmail:0.5.0
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  ghcr.io/soulteary/owlmail:0.5.0
```

`main` 与 `latest` 会随默认分支构建而移动，不是稳定版本选择器。`0.5.0` 指向本次
发布，`sha-<short-commit>` 指向一个仓库提交。

## 已知限制

- 入站 SMTP 用户名/密码设置已经存在，但不会拒绝未认证发送方；请将 SMTP
  监听器限制在可信网络。
- Webhook 转发是集成通知机制，不是持久消息队列；接收器应实现幂等。
- 启用 Web Basic Auth 后，健康检查端点仍保持公开，便于无凭据探针使用。
- SMTP 编译默认值为：单封邮件 1 MiB、最多 50 个收件人、读写超时 10 秒。

## 相关文档

- [Webhook 消息转发参考](./Webhook-Forwarding.md)
- [Webhook 场景示例](../../examples/webhooks/README.zh-CN.md)
- [API 参考](./API-Reference.md)
- [运维与排障](./Operations.md)
- [MailDev 对比与迁移指南](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
- [完整变更日志](../../CHANGELOG.md)
