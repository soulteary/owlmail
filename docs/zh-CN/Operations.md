# OwlMail 运维与排障

本文覆盖可复现的本地与容器部署、安全默认值、就绪检查、持久化、Webhook 容量、
关闭行为和常见故障。协议细节见 [API 参考](./API-Reference.md)与
[Webhook 消息转发](./Webhook-Forwarding.md)。

## 分层配置文件

使用 `-config PATH` 或 `OWLMAIL_CONFIG_FILE` 加载扁平的 YAML 或 JSON 对象。
键名使用不带开头短横线的 CLI 参数名：

```yaml
smtp: 1025
web: 1080
web-ip: 127.0.0.1
mail-directory: ./data/mail
metrics-enabled: true
log-level: normal
```

优先级依次为 CLI 参数、MailDev 兼容环境变量、`OWLMAIL_*` 环境变量、配置文件、
内置默认值。未知、重复、嵌套、null、超大或类型错误的文件值会使启动失败。文件上限
为 1 MiB。包含密码或对象存储凭据的配置文件不应提交到源码仓库，并应限制文件权限。

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
返回 ready。启用 S3 后，OwlMail 在后台优先执行只读 `HeadBucket`；若最小权限策略
拒绝 bucket 级操作，则回退到附件前缀下 `MaxKeys=1` 的 `ListObjectsV2`。最近结果会
被缓存，默认每 30 秒刷新一次，单次探测最多等待 5 秒。HTTP readiness 请求只读取缓存，
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

仅开启 S3 不会自动迁移已有附件：旧本地附件仍可读取并正常删除，新邮件附件写入
S3。启动恢复旧附件文件名时，只有文件大小、扩展名和 SHA-256 内容摘要共同唯一匹配
一个本地文件才会写入升级后的元数据；存在歧义时不会持久化映射。关闭 S3 或修改
bucket/prefix 也不会自动下载或搬迁对象。`-mail-max-disk-mb` 与
`storage.diskBytes` 只统计本地文件，不包含远端对象占用。

#### 离线本地附件迁移到 S3

迁移前应停止 OwlMail，并备份 `-mail-directory`。迁移命令复用服务端的
`OWLMAIL_S3_*` 环境变量和 S3 参数。先执行只读预检：

```bash
OWLMAIL_S3_ENABLED=true \
OWLMAIL_S3_REGION=us-east-1 \
OWLMAIL_S3_BUCKET=owlmail \
./owlmail migrate-attachments \
  -mail-directory ./owlmail-data \
  -dry-run
```

去掉 `-dry-run` 即开始上传。本地附件默认保留：

```bash
OWLMAIL_S3_ENABLED=true \
OWLMAIL_S3_REGION=us-east-1 \
OWLMAIL_S3_BUCKET=owlmail \
./owlmail migrate-attachments -mail-directory ./owlmail-data
```

只有确认要让已验证的 S3 对象成为唯一解码附件副本时，才显式增加
`-delete-local`：

```bash
./owlmail migrate-attachments \
  -s3-enabled -s3-region us-east-1 -s3-bucket owlmail \
  -mail-directory ./owlmail-data \
  -delete-local
```

预检会在第一次上传前读取所有已提交 EML 和 sidecar，使用 generated filename、
解码后 size 与 SHA-256 校验映射；遇到重复或未被元数据引用的候选文件会停止整个
任务，不会猜测。版本 1/2 sidecar 只有在 MIME 附件顺序以及现有本地或远端内容完全
匹配后，才会随成功迁移升级为版本 3。原始 EML、邮箱索引、已读状态和排序时间均不会
改写。dry-run 会对已记录为 S3 以及仅远端存在的恢复候选执行只读 size 和 SHA-256
校验；远端对象缺失或损坏会让 dry-run 失败，但不会写入任何数据。
预检的 EML 扫描和本地文件哈希会响应命令取消信号，因此操作者可以在上传开始前安全
中断耗时较长的只读校验。

每个对象都会流式上传，再从 S3 重新打开并校验精确大小和 SHA-256，之后才会原子
更新 sidecar；只有元数据提交成功后，`-delete-local` 才可删除对应本地文件。默认在
首次尝试后重试 3 次，每次截止时间为 5 分钟；可通过 `-retries`（0～100）、
`-migration-attempt-timeout` 和 `-migration-retry-delay` 调整。命令逐附件输出进度，
最后一行 `summary` 为 JSON 统计。

对象 key 是确定的：若在元数据提交前中断，重复运行会安全覆盖并重新验证同一个对象；
若在元数据提交后中断，重复运行会验证已记录的 S3 对象，并在指定参数时继续删除本地
文件，而不会再次上传。因此命令可幂等重复执行。若存在待处理存储 fence，应先正常
启动一次 OwlMail 完成恢复。本命令只支持本地到 S3，不实现 S3 到本地的反向迁移。

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

### 5. 在 Nginx 子路径后部署 Docker

容器端口保持在内部网络，OwlMail 与代理使用相同的外部前缀，并保留 WebSocket
升级请求头：

```yaml
services:
  owlmail:
    image: ghcr.io/soulteary/owlmail:0.6.0
    environment:
      OWLMAIL_BASE_PATHNAME: /owlmail
      OWLMAIL_WEB_EXTERNAL_SCHEME: https
    volumes:
      - owlmail-data:/app/mail
    expose:
      - "1080"

volumes:
  owlmail-data:
```

```nginx
location = /owlmail {
    return 308 /owlmail/;
}

location /owlmail/ {
    proxy_pass http://owlmail:1080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

访问 `https://example.com/owlmail/`。不要在 `proxy_pass` 中剥离 `/owlmail`：
OwlMail 会把 UI、REST API、附件、兼容路由和原生 WebSocket 都注册到此前缀下。
兼容路径 `/owlmail/socket.io` 仍是原生 RFC 6455 WebSocket，不是 Socket.IO
协议。健康探针相应改为 `/owlmail/healthz` 或 `/owlmail/api/v1/health`，
Service Worker scope 为 `/owlmail/`。

`OWLMAIL_BASE_PATHNAME=owlmail`、`/owlmail` 与 `/owlmail/` 会规范为同一值；
留空或 `/` 保持历史根路径行为。启动时会拒绝路径穿越段、query、fragment、scheme、
反斜线、编码斜线及内部空段。迁移时也可使用 `MAILDEV_BASE_PATHNAME` 别名；与其他
兼容变量相同，显式 CLI 参数的优先级最高。

## 供测试代理使用的只读 MCP

MCP 默认关闭，使用官方 Go SDK 的有状态 Streamable HTTP transport。通过
`-mcp-enabled` 或 `OWLMAIL_MCP_ENABLED=true` 开启。根路径部署的端点为
`/mcp`；配置 base pathname 后，端点为 `<base-pathname>/mcp`。

MCP 被挂载在现有 Web router 内，而不是额外启动独立监听器，因此继承以下边界：

- `-web-user` 与 `-web-password` 使用和 UI、REST API 相同的 HTTP Basic Auth
  保护每个 MCP 请求。MCP 不属于健康检查的免认证路径。
- `-https` 在同一个 HTTPS listener 上保护 MCP。若 TLS 在反向代理终止，代理必须
  转发 MCP 路径并执行预期的 HTTPS 策略；不要通过不可信明文 HTTP 发送 Basic Auth。
- `-base-pathname` 会同时移动 MCP 端点。此时无前缀 `/mcp` 保持 404，不会产生
  根路径认证旁路。

MCP 服务严格只提供七个封闭域、只读工具：

| 工具 | 输出边界 |
|---|---|
| `list_emails` | 复用现有邮箱查询 API 的分页轻量摘要 |
| `search_emails` | 按主题、文本和 HTML 过滤的同类摘要 |
| `get_email` | 单封邮件的深复制快照；清洗后的 HTML 需要显式请求 |
| `get_email_source` | 使用无损 base64 返回 RFC 5322 原始 source；原始字节默认最多 1 MiB，最大 100 MiB |
| `list_attachments` | 名称、类型、大小、哈希和存储元数据；从不返回附件字节 |
| `get_latest_email` | 按邮箱时间返回最新 1 至 20 封邮件的轻量摘要 |
| `wait_for_email` | 通过投递事件流等待新的收件人、主题或文本匹配；不进行轮询 |

不存在删除、标记已读、转发、中继、下载附件、修改配置或重新加载邮箱的 MCP
工具。`list_emails` 与 `search_emails` 复用 `QueryEmailPreviews`；`get_email` 和
`list_attachments` 复用邮箱的深复制 `GetEmail` 边界，不维护第二套索引，也不
返回内部可变指针。

`wait_for_email` 默认等待 30 秒，硬上限为 2 分钟；若 MCP session timeout
更短，则以后者为上限。每个 session 最多同时等待 4 个调用，全进程最多 64 个。
客户端取消、session 删除或超时以及进程关闭都会立即移除 waiter。waiter 表有界，
并通过单个邮箱 listener 接收已提交的 `new` 事件，不会高频轮询，也不会为每个
waiter 启动后台 goroutine。

同一服务还提供 `owlmail://inbox`、`owlmail://stats` Resources 和
`owlmail://email/{id}` Resource template。inbox 最多返回 50 条摘要；单封邮件
Resource 的文本最多 32 KiB 并标明是否截断，且不包含 HTML、headers、source 或
附件字节。`registration_verification_email`、`password_reset_email` 和
`wait_for_delivery` Prompts 只组合上述只读能力。

轻量摘要和邮件详情包含 `web_url`。默认根据 Web listener、HTTPS 或
`OWLMAIL_WEB_EXTERNAL_SCHEME` 以及规范化后的 base pathname 生成。通过反向代理
对外服务时，可设置 `-web-external-url https://mail.example.com` 或
`OWLMAIL_WEB_EXTERNAL_URL=https://mail.example.com`；该值必须是无凭据、query、
fragment 和 path 的 HTTP(S) origin。代理子路径继续通过 `-base-pathname` 单独
设置，例如 `/owlmail`。

多个客户端可以维持相互独立的会话。未知 session ID 返回 HTTP 404；客户端
`DELETE` 会关闭会话；空闲会话在 `-mcp-session-timeout` 后关闭（默认 `30m`）。
进程关闭时拒绝新的 MCP 工作、清理活动会话，并最多等待
`-mcp-shutdown-timeout`（默认 `5s`）。

`get_email_source` 返回 `encoding: "base64"`、`source_base64`、解码后的
`returned_bytes` 数量、完整 source 的 `size` 和 `truncated` 状态。
`max_bytes` 约束的是 base64 解码后的原始 source 字节，因此 JSON 表示会更大；
该格式能够无损保留任意 8-bit MIME 数据以及截断后的不完整 UTF-8 序列。

受保护的本地端点示例：

```bash
./owlmail \
  -mcp-enabled \
  -web-user agent \
  -web-password test-only-secret \
  -mcp-session-timeout 10m
```

将 MCP 客户端 URL 配置为 `http://localhost:1080/mcp` 并携带上述 Basic Auth。
远程可访问时，除认证外还应使用 HTTPS 和网络访问控制。

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
`OWLMAIL_SMTP_MAX_MESSAGE_MB` 设置为其他正整数 MiB。收件人上限默认为 50，可用
`-smtp-max-recipients` 或 `OWLMAIL_SMTP_MAX_RECIPIENTS` 调整。读写超时默认为
10 秒，可分别用 `-smtp-read-timeout` / `OWLMAIL_SMTP_READ_TIMEOUT` 和
`-smtp-write-timeout` / `OWLMAIL_SMTP_WRITE_TIMEOUT` 调整。收件人上限必须为正数，
超时必须为正数的 Go duration 字符串。

OwlMail 默认还会把每个进程同时处理的 SMTP 邮件正文事务限制为 8 个。可通过
`-smtp-max-concurrency` 或 `OWLMAIL_SMTP_MAX_CONCURRENCY` 调整；显式设置为
`0` 会恢复不限制并发的原有行为。普通 SMTP、STARTTLS 与直接 SMTPS 共用邮件
服务器持有的同一个 limiter。多副本部署中每个实例独立执行上限，这不是集群级或
分布式配额。

SMTP 库进入 DATA handler 时会以非阻塞方式获取名额，名额覆盖原始 EML 暂存、
MIME 与正文处理、附件流式写入和摘要计算、S3 上传，以及原子提交或失败回滚。
该上限不限制 TCP 连接、SMTP session、AUTH、MAIL FROM 或 RCPT TO。名额已满时，
OwlMail 会安全排空但不存储本次邮件正文，并返回
`451 4.3.2 temporary resource limit reached; try again later`。该临时状态允许合规
SMTP 客户端重试；被拒绝的事务不会留下 EML、附件、索引记录或 S3 对象。

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

## 出站 Relay TLS 与阶段超时

通过 `-outgoing-tls-mode` 或 `OWLMAIL_OUTGOING_TLS_MODE` 明确选择一种传输：

| 模式 | 线上行为 | 凭据 |
| --- | --- | --- |
| `plain` | SMTP 全程明文 | 配置用户名/密码时直接拒绝 |
| `starttls` | 必须声明 STARTTLS 并完成通过验证的握手 | 仅在 TLS 后发送 |
| `smtps` | TCP 建连后立即进行 TLS 握手 | 仅在 TLS 后发送 |

默认是 `plain`。兼容 MailDev 的 `-outgoing-secure` 和
`MAILDEV_OUTGOING_SECURE=true` 等同于 `smtps`，不再表示机会式 STARTTLS。
默认验证证书和主机名。`-outgoing-insecure-skip-verify` 默认关闭，只应在明确
接受风险的测试环境中显式开启。

连接/问候、TLS 握手、AUTH、MAIL/RCPT、DATA 与 QUIT 分别设置 deadline。
使用对应的 `-outgoing-{connect,tls-handshake,auth,envelope,data,quit}-timeout`
参数或 `OWLMAIL_OUTGOING_*_TIMEOUT` 环境变量配置。连接、TLS、AUTH 和信封阶段
默认 10 秒，DATA 默认 30 秒，QUIT 默认 5 秒。调用方 context 更早到期时，以其
为准并关闭连接。

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

出站中继同样是异步的。配置持久邮件目录后，原生中继任务会在入队前写入
`.owlmail-meta/relay-jobs`，未完成任务在启动时重新提交。客户端应轮询返回的状态地址
直到终态。连接与超时错误采用指数退避和有界抖动，最多尝试三次。恢复语义为“至少
一次”，因此下游 SMTP 已接受但终态尚未写入时崩溃可能
产生一次重复尝试。

所有出站中继入口共用同一套流式 SMTP 事务。worker 开始投递时才打开已存储的 EML，
使用固定 32 KiB 缓冲区复制到 SMTP `DATA` 返回的 writer，并继续由 `net/smtp` 负责
CRLF 规范化与 dot-stuffing。因此，普通中继、指定地址中继、自动中继及同步的
MailDev 兼容路由都不会再创建与邮件大小相当的额外字节切片。context 取消、deadline、
源文件读取失败、DATA 写入失败或下游连接中断时，EML reader 与 SMTP 连接都会关闭；
若 `DATA` 开始后流式复制失败，会先中止连接再关闭 DATA writer，避免下游接受截断邮件。

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
