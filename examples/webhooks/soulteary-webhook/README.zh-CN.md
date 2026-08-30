# OwlMail + `soulteary/webhook`

这个可运行示例会把 OwlMail 新邮件发送到
[`soulteary/webhook`](https://github.com/soulteary/webhook)，使用 HMAC-SHA256
校验完整请求体，把 JSON 字段映射为命令环境变量，并执行 `print-email.sh`。

Compose 示例固定使用已经发布的 OwlMail `0.5.0` 和 WebHook `7.0.0`，避免从
持续变化的 `main` 分支重新构建 OwlMail，使示例更容易复现和长期维护。

## 文件说明

| 文件 | 使用方 | 作用 |
|---|---|---|
| [`owlmail.json`](./owlmail.json) | OwlMail | 环境变量目标 URL 与 HMAC 密钥、重试策略和自定义 JSON 请求体。 |
| [`hooks.json.tmpl`](./hooks.json.tmpl) | `soulteary/webhook` | Hook、字段映射和 `X-OwlMail-Signature` 校验。 |
| [`print-email.sh`](./print-email.sh) | `soulteary/webhook` | 无外部副作用、仅输出映射字段的演示命令。 |
| [`compose.yaml`](./compose.yaml) | Docker Compose | 在同一私有网络启动已发布的 OwlMail 和 WebHook 镜像。 |

## 启动服务

从该目录执行：

```bash
cd examples/webhooks/soulteary-webhook
export OWLMAIL_WEBHOOK_SECRET="$(openssl rand -hex 32)"
docker compose up
```

Compose 文件保留了仅用于本地演示的默认密钥，因此不设置变量也能启动；只要端口
可能被其他机器访问，就应该使用自己的随机密钥。同一个密钥会传给两个容器。

OwlMail 显式设置 `OWLMAIL_WEBHOOK_MAX_CONCURRENCY=8`，与安全默认值一致。生产
环境应根据下游处理能力设置上限；只有明确希望不限制并发，并确认接收端能够承受时，
才应设置为 `0`。

`soulteary/webhook` 使用模板模式启动，使 `getenv` 能把密钥、命令和工作目录写入
Hook 定义。Hook 会等待演示命令完成并捕获输出，因此命令失败时会向 OwlMail 返回
非 2xx 状态。本地 Demo 开启了调试日志；生产环境应关闭 `DEBUG`（通常也关闭
`VERBOSE`），因为日志中可能出现映射后的邮件内容。

## 发送邮件

在另一个终端执行：

```bash
printf 'From: monitor@example.test\r\nTo: ops@example.test\r\nSubject: Demo alert\r\n\r\nThe integration works.\r\n' \
  | curl --url smtp://127.0.0.1:1025 \
      --mail-from monitor@example.test \
      --mail-rcpt ops@example.test \
      --upload-file -
```

打开 `http://localhost:1080` 可以查看已保存邮件；打开
`http://localhost:1080/webhooks` 可以查看、导入、校验或生成 OwlMail Webhook
配置。`webhook` 服务日志会输出通过签名验证并完成字段映射后的事件。

如果两边密钥不同，或请求体在签名后被修改，`soulteary/webhook` 会拒绝触发，
命令不会执行。OwlMail 会记录非 2xx 结果，并按照 `owlmail.json` 进行重试。
实际业务处理器应保持幂等，并使用邮件 ID 作为去重键。

## 停止并清理

```bash
docker compose down
```

示例未配置邮件数据卷，因此容器被移除后，捕获的邮件也会删除。如需持久化，
请为 `/app/mail` 添加数据卷。

如果不使用容器，先在当前目录启动 `soulteary/webhook`：

```bash
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
export OWLMAIL_WEBHOOK_COMMAND="$PWD/print-email.sh"
export OWLMAIL_WEBHOOK_WORKDIR="$PWD"
webhook -template -hooks hooks.json.tmpl -verbose -debug
```

在另一个终端传入本机 Hook 地址、相同密钥和并发限制，再启动 OwlMail：

```bash
export OWLMAIL_WEBHOOK_URL='http://127.0.0.1:9000/hooks/owlmail'
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
export OWLMAIL_WEBHOOK_MAX_CONCURRENCY=8
owlmail -webhook-config owlmail.json
```

示例将所有宿主机端口绑定到 `127.0.0.1`，在共享主机上不要移除此边界。
生产环境中，HTTP 界面可以置于经过身份验证的反向代理之后，并应继续启用 OwlMail Web
Basic Auth；但这不能保护 SMTP。请将 1025 端口限制在可信接口，或通过网络策略、防火墙
或私有隧道进行隔离。同时保留 HMAC 校验，限制允许执行的命令路径、执行时间与并发，
并避免记录完整邮件正文。

[English](./README.md) ·
[Webhook 消息转发参考](../../../docs/zh-CN/Webhook-Forwarding.md) ·
[WebHook 侧联动指南](https://github.com/soulteary/webhook/blob/main/docs/zh-CN/OwlMail-Integration.md)
