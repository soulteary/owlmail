# OwlMail 运维与排障

本文覆盖可复现的本地与容器部署、安全默认值、就绪检查、持久化、Webhook 容量、
关闭行为和常见故障。协议细节见 [API 参考](./API-Reference.md)与
[Webhook 消息转发](./Webhook-Forwarding.md)。

## 邮箱保留策略与状态

OwlMail 可以同时限制邮件年龄、封数和磁盘占用：

```bash
owlmail \
  -mail-retention-days 14 \
  -mail-max-messages 10000 \
  -mail-max-disk-mb 2048 \
  -mail-cleanup-interval 10m
```

单项设置为零表示不限制。服务会在启动时执行一次清理，之后由后台任务定期删除
最旧邮件，直到全部限制都满足。现有邮件统计响应会包含磁盘字节数、限制值、清理
次数、删除封数、回收字节数和最近错误。

已读状态通过 `.owlmail-meta/<id>.json` 原子保存；没有元数据的旧邮件在恢复时保持
未读。ZIP 导出直接流式写给客户端，并限制为最多 1,000 个源邮件及 256 MiB 原始
EML 数据，避免再创建一个邮箱规模的内存缓冲区。

## 部署场景

### 1. 最简本地收件箱

```bash
./owlmail
```

默认只监听 localhost：SMTP 1025、HTTP 1080。未设置 `-mail-directory` 时，
原始邮件和附件写入当前进程专用的临时目录 `owlmail-<pid>`，解析后的状态保存在
内存中；该目录不是持久归档。该档位适合可信机器上的单开发者场景。

存活检查：

```bash
curl --fail http://localhost:1080/healthz
```

### 2. 带持久化的本地收件箱

```bash
mkdir -p ./owlmail-data
./owlmail -mail-directory ./owlmail-data
```

更换 OwlMail 版本或测试其他产品生成的归档前，应先备份目录。附件文件与邮件数据
一同保存，只复制部分文件可能产生不完整邮件。

OwlMail 在同一目录中暂存并同步原始邮件与附件，附件先提交，最后以 `.eml` 原子
重命名作为完整邮件的提交标记。解析、附件写入或内存提交失败时，当前事务会回滚。
启动恢复会把残留临时文件、孤立附件目录和无法解析的 `.eml` 移入
`<mail-directory>/quarantine/`，而不是把损坏数据静默载入或删除。确认内容不再
需要后，可由运维人员清理该目录；排障前不要把其中的文件直接移回邮件目录。

解析 MIME 时，每个解码后的附件使用固定 32 KiB 缓冲区，直接流入该邮件私有事务
暂存目录中的附件临时文件；同一次流式处理同步计算解码后大小和 SHA-256。只有完整
读取 MIME part 后才会同步并重命名临时文件；读取、写入、同步或关闭失败都会删除
局部文件。附件 API 无法访问事务暂存目录。本地存储会在 `.eml` 提交标记之前原子
提升整个附件目录；S3 模式则从已完成的暂存文件流式上传。只有原始邮件、附件和
元数据事务全部完成后，邮件才进入内存索引并可通过 API 访问。

为保持现有 API 无需额外存储读取即可返回正文，纯文本与 HTML 正文仍保存为内存
字符串。正文不会被静默截断：任何 MIME 正文读取错误都会拒绝当前事务。SMTP 接收
场景中的最大内存边界来自整封邮件大小限制（`OWLMAIL_SMTP_MAX_MESSAGE_MB`，默认
100 MiB），目前没有单独的正文上限。为兼容旧数据，启动时载入的现有 `.eml` 不受
SMTP 限制，运维人员应只恢复可信且大小合理的文件。正文流式持久化属于后续独立
优化。

### 可选 S3 兼容附件存储

可以把解析后的附件存入 AWS S3 或 S3 兼容服务；原始邮件、元数据、事务标记、临时
暂存文件和 Webhook outbox 仍保存在 `-mail-directory`。默认继续使用本地附件存储。

```bash
OWLMAIL_S3_ENABLED=true \
OWLMAIL_S3_ENDPOINT=http://minio:9000 \
OWLMAIL_S3_REGION=us-east-1 \
OWLMAIL_S3_BUCKET=owlmail \
OWLMAIL_S3_PREFIX=owlmail/attachments \
OWLMAIL_S3_ACCESS_KEY=replace-me \
OWLMAIL_S3_SECRET_KEY=replace-me \
OWLMAIL_S3_USE_PATH_STYLE=true \
./owlmail -mail-directory ./owlmail-data
```

OwlMail 接收附件前，存储桶必须已经存在。endpoint 留空时使用 AWS S3。静态密钥
可以不配置，此时使用 AWS SDK 默认凭据链，包括环境凭据、共享配置和工作负载角色。
只有选定的 S3 兼容服务要求时，才开启路径式寻址。

### Liveness、readiness 与 S3 启动策略

`GET /healthz` 与 `GET /api/v1/health` 是进程 liveness 检查，不依赖 S3。
因此对象存储暂时不可用时不会造成容器反复重启；镜像内置 Docker 健康检查也刻意
使用 `/healthz`。

`GET /readyz` 与 `GET /api/v1/ready` 是 readiness 检查。本地附件存储会立即
返回 ready。启用 S3 后，OwlMail 在后台执行只读 `HeadBucket`，缓存最近结果，
默认每 30 秒刷新一次，单次探测最多等待 5 秒。HTTP readiness 请求只读取缓存，
不会同步访问或等待 S3。

```bash
OWLMAIL_S3_STARTUP_CHECK=false \
OWLMAIL_S3_HEALTH_CHECK_INTERVAL=30s \
OWLMAIL_S3_HEALTH_CHECK_TIMEOUT=5s \
./owlmail
```

默认 `OWLMAIL_S3_STARTUP_CHECK=false`，保持已有启动兼容性：readiness 起初为
`checking` 并返回 HTTP `503`，首次探测成功后转为 ready。若希望 bucket、凭据、
权限或网络路径错误立即阻止部署，可将其设置为 `true`。只有首次失败会终止启动；
运行期间 S3 故障只让 readiness 返回 `503`，liveness 仍为 `200`，后台探测成功后
自动恢复。

readiness 响应只包含组件状态、探测时间和 `pending`、`permission`、
`credentials`、`not_found`、`timeout`、`unavailable`、`unknown`、`closed` 之一；
不会包含 SDK 原始错误、endpoint、access key、secret key 或 session token。

Kubernetes 应分别配置重启判断和流量接入：

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 1080
readinessProbe:
  httpGet:
    path: /readyz
    port: 1080
```

OwlMail 会把附件流式写入本地暂存文件并写入持久回滚标记，再把全部对象上传至
`<prefix>/<email-id>/`，最后提交 `.eml` 标记。上传失败会拒绝 SMTP 事务并清理
对象前缀；若进程在事务中断，启动恢复会继续重试清理。每个附件的上传和远端下载流
截止时间均为 5 分钟，失去响应的端点不会无限占用存储事务或请求。删除单封邮件、
清空邮箱和保留策略清理都会删除远端对象，同时清理可能存在的旧本地附件目录。
删除前会为每封
邮件同步写入删除标记，并优先清理远端对象；如果远端清理失败，原始邮件、元数据、
本地附件和删除标记都会保留，可在当前进程重试或由下次启动自动恢复。清理完成前，
带删除标记的邮件不会重新载入。损坏邮件隔离也会先清理远端对象；S3 不可用时保留
原位置的 `.eml` 作为重试标记。每次远端删除尝试的截止时间为 30 秒，失去响应的
端点不会无限占用存储事务锁，未完成的清理会留待后续重试。

开启 S3 不会自动迁移已有附件：旧本地附件仍可读取并正常删除，新邮件附件写入
S3。启动恢复旧附件文件名时，只有文件大小、扩展名和 SHA-256 内容摘要共同唯一匹配
一个本地文件才会写入升级后的元数据；存在歧义时不会持久化映射。关闭 S3 或修改
bucket/prefix 也不会自动下载或搬迁对象，变更前应先完成迁移。
`-mail-max-disk-mb` 与 `storage.diskBytes` 只统计本地文件，不包含远端对象占用。

### 3. 持久化 Docker 部署

```bash
docker volume create owlmail-data
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.6.0
```

本文固定使用 `0.6.0` 发布镜像。`main` 与 `latest` 会随默认分支构建移动，不应用于
可复现部署。

镜像内部默认绑定 `0.0.0.0`。除非其他机器必须访问，否则应像示例一样把宿主机
端口限制到 `127.0.0.1`。Dockerfile 使用非 root 用户，邮件目录为 `/app/mail`。

### 4. 使用固定凭据保护 Web UI

```bash
docker run -d \
  --name owlmail \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -e OWLMAIL_WEB_USER=admin \
  -e OWLMAIL_WEB_PASSWORD='replace-with-a-secret' \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.6.0
```

自动化场景应同时配置用户名和密码。只配置用户名时，每次启动会生成新密码并只在
stderr 输出一次；只配置密码时用户名默认为 `admin`。如果无法输出生成的密码，
OwlMail 会启动失败。查看启动输出：

```bash
docker logs owlmail
```

Basic Auth 保护 UI、API、静态资源和 WebSocket，但 `/healthz` 与
`/api/v1/health` 会有意保持公开，便于探针检查。凭据通过网络传输时，应启用
HTTPS 或可信反向代理。

若 TLS 在反向代理终止，请设置 `OWLMAIL_WEB_EXTERNAL_SCHEME=https`，让已鉴权的
HTTP 与 WebSocket 同源检查使用浏览器可见协议。应显式配置该值，不要信任客户端
可以伪造的转发请求头。

## HTTPS 与 TLS

Web HTTPS 与 SMTP TLS 是两组独立设置：

- `-https`、`-https-cert`、`-https-key` 保护 Web UI/API。
- `-tls`、`-tls-cert`、`-tls-key` 启用 SMTP STARTTLS，并在进程/容器 465
  端口提供直接 SMTPS。

Web HTTPS 示例：

```bash
./owlmail \
  -https \
  -https-cert ./certs/web-cert.pem \
  -https-key ./certs/web-key.pem
```

镜像内置 healthcheck 使用 1080 端口的明文 HTTP。启用 HTTPS 后，应覆盖为 HTTPS
探针并配置正确的 CA 策略；否则即使 OwlMail 正常服务，Docker 仍可能显示 unhealthy。

需要直接 SMTPS 时显式发布 465 端口：

```bash
docker run -d \
  -p 1025:1025 -p 1080:1080 -p 465:465 \
  -v "$PWD/certs:/certs:ro" \
  -e OWLMAIL_TLS_ENABLED=true \
  -e OWLMAIL_TLS_CERT=/certs/smtp-cert.pem \
  -e OWLMAIL_TLS_KEY=/certs/smtp-key.pem \
  ghcr.io/soulteary/owlmail:0.6.0
```

确认容器运行时允许非 root 进程绑定 465；如不允许，应按运行时安全策略只授予所需
的 bind-service 能力。

## SMTP 入口限制与鉴权模式

SMTP 与 SMTPS 默认单封邮件上限为 100 MiB。可通过 `-smtp-max-message-mb` 或
`OWLMAIL_SMTP_MAX_MESSAGE_MB` 设置为其他正整数 MiB。收件人上限仍为 50，读写
超时仍为 10 秒。

同时省略 `-smtp-user` 与 `-smtp-password` 时使用 NO AUTH 模式：允许不认证直接
投递，也接受任意 PLAIN/LOGIN 凭据，便于必须填写凭据的测试客户端零配置接入。
同时设置两项后强制 SMTP AUTH；未认证事务返回 `530 5.7.0`，错误凭据返回
`535 5.7.8`。只设置其中一项会启动失败。

如果部署环境不允许认证载荷经过明文连接，请设置 `-smtp-auth-require-tls` 或
`OWLMAIL_SMTP_AUTH_REQUIRE_TLS=true`，并启用 SMTP TLS。此时明文监听器会拒绝
PLAIN/LOGIN，STARTTLS 会话和直接 SMTPS 连接仍可正常 AUTH；NO AUTH 模式仍允许
匿名投递。开启策略但未启用 TLS 配置时，服务会在打开监听器前启动失败。

> [!WARNING]
> NO AUTH 有意保持开放。为兼容开发环境，OwlMail 也允许在非 TLS 连接上使用
> PLAIN/LOGIN。请将监听器限制在可信接口，并在使用真实凭据前启用 TLS。

SMTP TLS 只有在 `-tls-cert` 与 `-tls-key` 同时存在时才使用指定证书，否则会生成
自签名证书并记录警告。Web HTTPS 行为不同：`-https-cert` 与 `-https-key` 两项
都必须存在，缺失时 Web 服务无法启动。

## Webhook 容量档位

`-webhook-max-concurrency` 是跨全部目标和邮件的进程级上限，并按每个目标 HTTP
请求获取。有限值会限制活动下游请求，本地 outbox 则承接已经接受的事件交接。

| 档位 | 值 | 适用场景 |
|---|---:|---|
| 建议起点 | `8` | 常规开发和中等下游延迟 |
| 保守 | `2`–`4` | 脆弱、限流或资源受限的接收器 |
| 较高有界吞吐 | `16`–`64` | 压测确认下游及文件描述符容量 |
| 无限 | `0` | 突发规模可控，并明确希望充分使用机器资源 |

```bash
# 建议的有限投递
./owlmail \
  -webhook-config ./webhooks.json \
  -webhook-max-concurrency 8

# 显式无限并发
./owlmail \
  -webhook-config ./webhooks.json \
  -webhook-max-concurrency 0
```

该值不是队列长度。SMTP `DATA` 命令会等待事件同步写入
`.owlmail-webhook-outbox`，但不等待目标槽位、Redis append 或 HTTP 响应。估算
排空时间时应包含目标超时与重试时长；提高上限前先监控接收器延迟与错误率。长期
下游或 Redis 故障会积累本地 outbox 文件，因此还应监控邮件卷剩余空间。

## 关闭与投递保证

收到 `SIGINT` 或 `SIGTERM` 后，OwlMail 会停止 SMTP 入口和新的 Webhook 交接，
并在 `-webhook-shutdown-timeout` 内排空本地 outbox、排队任务及活动 Webhook 请求。
超时后会取消剩余操作并报告关闭错误。需要更强投递保证时：

1. 终止 OwlMail 前先停止上游 SMTP 流量。
2. 将关闭期限设置为可覆盖预期队列和重试窗口。
3. 使用 Redis 获得可跨重启的排队投递，并持久化完整邮件目录，使队列接收前的
   outbox 条目也能跨重启恢复。
4. 接收器实现幂等，并使用 `X-OwlMail-Delivery-ID` 去重。

未配置 Redis 时，只有进入内存队列前的交接是持久的；条目离开本地 outbox 后不再
具备跨重启能力。Redis 投递是持久但“至少一次”的，崩溃边界仍可能产生重复。

出站中继同样是异步的。API 成功响应只确认进程内请求已被接受，不代表下游 SMTP
已经完成投递；需要确认投递时，应检查 OwlMail 日志与目标系统。

项目尚未通过一次统一启动校验覆盖所有配置。启用 S3 时会先检查其配置结构，但
可达性和凭据仍可能在首次对象操作时才暴露错误；其他组件也可能在监听器启动阶段
才归一化取值或失败。请把启动及运行告警视为需要处理的问题，并在修改配置后同时
验证健康端点、SMTP 收件和附件下载。

## 备份与升级流程

1. 停止新 SMTP 流量。
2. 完整复制或快照邮件目录/卷。
3. 记录镜像标签或二进制版本及有效参数/环境变量。
4. 重要归档应先让新版本读取副本。
5. 检查 `/healthz`、抽样打开 HTML 和附件，再发送一封新测试邮件。
6. 在不再需要回滚前保留备份。

可复现环境应记录并部署正式发布的
`ghcr.io/soulteary/owlmail@sha256:<digest>`。标签都是别名，其中 `main`、
`latest`、主版本和次版本标签会有意移动。

## 故障排查

| 现象 | 检查与处理 |
|---|---|
| Web UI 无法访问 | 核对 `-web-ip`/`OWLMAIL_WEB_HOST`、端口发布和 `/healthz`；查看绑定或证书错误 |
| 浏览器反复要求凭据 | 核对实际用户名/密码；自动密码重启后会变化；必要时清除浏览器缓存凭据 |
| 设置 SMTP 凭据后入口仍然开放 | 当前入站 SMTP 鉴权未强制执行；使用接口绑定与网络控制隔离监听器 |
| HTTPS 容器显示 unhealthy | 用 HTTPS 探针和正确证书信任覆盖镜像的 HTTP healthcheck |
| 浏览器没有通知 | 在收件箱中开启，使用 HTTPS 或 localhost，并在浏览器网站设置中恢复权限 |
| Webhook 投递慢 | 检查接收器延迟、超时和重试；提高并发前先修复接收器或减少重试 |
| SMTP 正常但直接 SMTPS 失败 | 发布 465 端口，核对证书路径及运行时绑定特权端口的权限 |
| 中继 API 成功但邮件未到达 | 中继是异步的；检查 OwlMail 日志、出站 SMTP 连通性、收件人语法和接收端日志 |
| 重建容器后邮件消失 | 将卷挂载到 `/app/mail`；未挂载的容器文件系统可随时丢弃 |
| 从 MailDev 迁移后 API 客户端失败 | 适配 `/api` 与 `/api/v1`、分页信封、显式已读操作和原生 WebSocket |

常用检查：

```bash
docker logs --tail 200 owlmail
curl --fail http://localhost:1080/healthz
curl -u admin:secret http://localhost:1080/api/v1/version
```
