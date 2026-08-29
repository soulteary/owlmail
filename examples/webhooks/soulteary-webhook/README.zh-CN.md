# OwlMail + `soulteary/webhook`

这个可运行示例会把 OwlMail 新邮件发送到
[`soulteary/webhook`](https://github.com/soulteary/webhook)，使用 HMAC-SHA256
校验完整请求体，把 JSON 字段映射为命令环境变量，并执行 `print-email.sh`。

## 文件说明

| 文件 | 使用方 | 作用 |
|---|---|---|
| [`owlmail.json`](./owlmail.json) | OwlMail | 环境变量目标 URL 与 HMAC 密钥、重试策略和自定义 JSON 请求体。 |
| [`hooks.json.tmpl`](./hooks.json.tmpl) | `soulteary/webhook` | Hook、字段映射和 `X-OwlMail-Signature` 校验。 |
| [`print-email.sh`](./print-email.sh) | `soulteary/webhook` | 无外部副作用、仅输出映射字段的演示命令。 |
| [`compose.yaml`](./compose.yaml) | Docker Compose | 构建 OwlMail，并在同一私有网络启动两个服务。 |

## 启动服务

从该目录执行：

```bash
cd examples/webhooks/soulteary-webhook
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
docker compose up --build
```

Compose 文件提供了仅用于本地演示的默认密钥，因此不设置变量也能启动；只要端口
可能被其他机器访问，就应设置自己的长随机密钥。同一个密钥会传给两个容器。
Compose 还会传入 `webhook` 容器内的命令和工作目录路径。`soulteary/webhook`
使用模板模式启动，使 `getenv` 能把这三个值写入 Hook 定义。Hook 会等待演示命令
完成并捕获输出，因此命令失败时会向 OwlMail 返回非 2xx 状态。
本示例开启了调试日志以显示命令摘要；生产环境应关闭 `DEBUG`（通常也关闭
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

打开 `http://localhost:1080` 可以查看已保存邮件。`webhook` 服务的 Compose
日志会输出类似内容：

```text
OwlMail webhook event
  event: email.received
  id: Ab12Cd34
  title: Demo alert
  from: monitor@example.test
  to: ops@example.test
  received: 2026-08-29T12:00:00Z
  message: The integration works.
```

如果两边密钥不同，或请求体在签名后被修改，`soulteary/webhook` 会拒绝触发，
命令不会执行。OwlMail 会记录非 2xx 结果，并按照 `owlmail.json` 进行重试。

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

在另一个终端传入本机 Hook 地址和相同密钥，再启动 OwlMail：

```bash
export OWLMAIL_WEBHOOK_URL='http://127.0.0.1:9000/hooks/owlmail'
export OWLMAIL_WEBHOOK_SECRET='replace-with-a-long-random-secret'
owlmail -webhook-config owlmail.json
```

[English](./README.md) ·
[Webhook 消息转发参考](../../../docs/zh-CN/Webhook-Forwarding.md)
