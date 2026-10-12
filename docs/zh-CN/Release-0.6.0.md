# OwlMail 0.6.0 发布说明

OwlMail 0.6.0 显著增强了邮件持久化与 Webhook 投递的可靠性。本版本新增原子邮件
事务与启动恢复、可配置存储治理、持久化 Webhook 队列交接、Redis 租约保护和移动端
兼容的浏览器通知，同时加强已鉴权请求的同源检查与发布供应链。

OwlMail 0.6.0 已于 2026-09-01 发布。以下命令使用已发布的 `v0.6.0` 标签与
`0.6.0` 容器镜像。

## 主要更新

### 原子邮件持久化与恢复

原始邮件与附件会先在同一目录暂存并同步，只有最终 `.eml` 原子重命名完成后才会对
内存可见。解析、附件写入或内存提交失败都会回滚当前事务。启动时，不完整、损坏和
由 OwlMail 生成的孤立文件会移入 `<mail-directory>/quarantine/`；无关目录和已有
隔离证据不会被删除。

邮件存储改为按 ID 建立索引，并向 API、WebSocket、Webhook 和事件消费者返回深度
快照。已读状态会原子写入 `.owlmail-meta/`，成功记录的状态不会在重启后重置。

### 存储治理

可以组合使用三个独立限制：

```bash
owlmail \
  -mail-retention-days 14 \
  -mail-max-messages 10000 \
  -mail-max-disk-mb 2048 \
  -mail-cleanup-interval 10m
```

三个限制默认均为 `0`，即不启用。启用后，清理会在启动时和后台周期执行，按最旧
邮件优先删除，直到满足全部限制。统计 API 会报告当前磁盘占用、清理次数、删除邮件
数、回收字节数和最近错误。ZIP 导出改为流式处理，并限制最多 1,000 封源邮件和
256 MiB 原始 EML。

### 持久化 Webhook 交接与 Redis 投递

每个已接受的 Webhook 事件都会先写入邮件目录中的本地 outbox。Redis 或内存队列
暂时无法接受任务时，outbox 会保留交接记录；清空收件箱不会删除它。如果需要依赖
这一恢复边界，应配置持久化的 `-mail-directory`。

配置 `-webhook-redis-url` 后，Redis Streams 提供重启恢复、稳定投递 ID、消费者组
pending 恢复、死信记录和活动租约续期。租约续期会原子验证所有权，延迟 worker
无法从其他消费者手中抢回条目。投递仍是至少一次语义；接收方应根据
`X-OwlMail-Delivery-ID` 去重。

关闭时会停止新的交接，并在 `-webhook-shutdown-timeout` 内排空 outbox、排队任务
和活动请求。不使用 Redis 时，任务从本地 outbox 进入进程内内存队列后将不再具备
重启持久性。

### Webhook 与浏览器正确性

- 并发限制作用于单个目标请求，单封邮件不会占用容量并阻塞无关目标；
- 文本 glob 可匹配包含 `/` 的 URL 类字符串，同时保留 Go 风格通配符与字符范围语法；
- HMAC nonce 在有效期边界使用排他的过期判定；
- 浏览器通知会等待 Service Worker 激活，只聚焦收件箱客户端，并在新窗口深链接中
  保留所选邮件；
- 在可信代理终止 TLS 时，`OWLMAIL_WEB_EXTERNAL_SCHEME=https` 让已鉴权 HTTP 与
  WebSocket 同源检查使用浏览器看到的协议。

## 升级前需要确认的行为

| 范围 | 0.6.0 行为 | 运维建议 |
|---|---|---|
| 邮件目录 | 新增 `.owlmail-meta`、`.owlmail-webhook-outbox` 和 `quarantine` | 备份与恢复完整目录，包括隐藏项 |
| 保留策略 | 任意非零限制都可能在启动时删除最旧邮件 | 先使用邮箱副本验证策略 |
| 启动恢复 | 损坏的 OwlMail 文件会被隔离，不再静默跳过 | 人工检查隔离证据，不要直接移回活动目录 |
| Webhook 投递 | 本地交接持久化；完整的重启安全排队仍需要 Redis | 需要端到端恢复时同时使用持久邮件目录和 Redis |
| 投递语义 | Redis 投递为至少一次 | 接收方根据稳定投递 ID 去重 |
| 反向代理 | 已鉴权浏览器同源检查包含协议 | 外部可信 TLS 终止时设置 `OWLMAIL_WEB_EXTERNAL_SCHEME=https` |

## 安装方式

### 发布二进制

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.6.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.6.0/checksums.txt
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

发布内容包含 Linux amd64/arm64、macOS amd64/arm64 和 Windows amd64 二进制。
每个可执行文件都有对应的 SPDX SBOM；校验和清单包含 Sigstore bundle，GitHub
provenance attestation 覆盖二进制和 SBOM。

### Go 安装

从源码安装需要 Go 1.27.0 或更高版本：

```bash
go install github.com/soulteary/owlmail/cmd/owlmail@v0.6.0
```

下载的发布二进制在运行时不需要 Go、Bun 或 Node.js。

### 容器镜像

```bash
docker pull ghcr.io/soulteary/owlmail:0.6.0
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.6.0
```

发布工作流会拒绝覆盖已经存在的 `0.6.0` 镜像；`0.6`、`0`、`main` 与 `latest`
仍是移动别名。但 Registry 标签本质上仍是名称，不是内容身份。需要密码学意义上的
精确部署时，应固定正式发布清单的 digest：

```text
ghcr.io/soulteary/owlmail@sha256:<digest>
```

## 已知限制

- 入站 SMTP 用户名/密码设置仍不会拒绝未认证发送方；请将 SMTP 限制在可信接口或
  使用网络控制隔离；
- 不使用 Redis 时，任务离开本地 outbox 进入进程内内存队列后不具备重启安全性；
- Redis 投递为至少一次语义，在崩溃边界可能产生重复；
- 启用 Web Basic Auth 后，健康检查端点仍保持公开；
- 未配置 `-mail-directory` 时，进程专属临时邮件目录不是持久归档。

## 相关文档

- [运维与排障](./Operations.md)
- [Webhook 消息转发参考](./Webhook-Forwarding.md)
- [Webhook 场景示例](../../examples/webhooks/README.zh-CN.md)
- [API 参考](./API-Reference.md)
- [发布验证](./Releasing.md)
- [完整变更日志](../../CHANGELOG.md)
