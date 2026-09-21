# OwlMail 0.10.0 发布说明

OwlMail 0.10.0 是一个安全版本。它关闭了通过 Web UI、REST API、WebSocket 流以及
只读 MCP 端点跨源读取已捕获邮件的路径，不再把邮箱写进一个全局可列目录，并让
隐式 TLS 监听端口可配置——此前它固定在一个常常无法绑定的特权端口上。同时新增了
对四个解析器的模糊测试覆盖，本次修复的两个缺陷正是它找出来的。

0.10.0 版本日期为 2026-09-21。以下命令使用 `v0.10.0` 标签与 `0.10.0` 容器镜像，
请在标签和制品可用后执行。

**部署前请先阅读升级说明。** 本版本有三处升级可见的变化，其中一处会导致进程拒绝启动。

## 版本亮点

### 浏览器来源校验

Web UI 与 REST API 现在对每个请求校验浏览器 `Origin` 头，且不再依赖 Web Basic
Auth。此前该校验以 Basic Auth 开启为前提，而它默认是关闭的，因此默认部署会对任何
来源返回 `Access-Control-Allow-Origin: *`。只监听环回地址并不能缓解这一点：开发者
访问的那个无关页面，其浏览器就运行在同一台主机上，于是该页面可以读走整个邮箱——
包括密码重置链接和验证码——改写出站 SMTP 中继、把捕获的邮件转发到它指定的地址，
并清空邮箱。

只读 MCP HTTP 端点做了同样的处理，并拥有自己更严格的 CORS 策略，不再沿用 Web 的
那一套。这关闭了经由 `/mcp` 的跨源读取与 DNS 重绑定读取。

不带 `Origin` 头的请求保持原样放行，因为非浏览器客户端从不发送该头：`curl`、
HTTP 库、MCP SDK 的 HTTP 客户端、CI 脚本和服务端之间的调用都不受影响。

两个新选项用于追加允许的浏览器来源：

| 命令行参数 | 环境变量 | 作用范围 |
| --- | --- | --- |
| `-web-allowed-origins` | `OWLMAIL_WEB_ALLOWED_ORIGINS` | Web UI、REST API、WebSocket |
| `-mcp-allowed-origins` | `OWLMAIL_MCP_ALLOWED_ORIGINS` | 仅 `/mcp` |

被允许的来源会收到一份明确指名该来源的 CORS 策略，带凭据且预检不受质询，因此运维
有意放行的浏览器客户端确实能读到响应。单独一个 `*` 是文档化的关闭开关，会恢复此前
的通配行为；它不能与具名来源混用，因为这种组合只可能是笔误，会悄悄放宽一份本应保持
收紧的清单。

来源按浏览器序列化它们的方式比较：默认端口与补零端口、等价的 IP 写法、国际化域名
都会归一到同一形式。WebSocket 升级请求执行同一策略——浏览器不对 WebSocket 施加
CORS，而这条流承载着同样的邮件正文。

健康检查、就绪检查与指标端点位于该边界之内，而非豁免于它。真正会去探测它们的调用方
都不发送 `Origin`，不受影响；而此前能从无关来源读取 `/healthz` 的浏览器页面，是在
把它当作一个环回服务的指纹探测器用，而不是在做健康检查。

### 存储权限

已捕获的邮件不再写入全局可列目录，存储层提交的每一件产物现在都显式固定自己的权限位。

邮件目录此前以 `0755` 创建，因此本机任何其他账户都能列出邮箱内容：消息标识、`.eml`
文件名、大小、时间戳，以及哪些消息带有附件。邮件目录默认位于系统共享临时目录之下，
所以这是开箱即用的状态，而不是某种特殊配置。

| 产物 | 此前 | 现在 |
| --- | --- | --- |
| 邮件目录 | `0755` | `0750` |
| 每消息附件目录 | `0755` | `0700` |
| 附件文件 | `0644` | `0600` |

消息正文、元数据 sidecar，以及每条消息提交时经过的暂存文件，现在也都用显式 `chmod`
固定权限，因此它们都不再取决于 `os.CreateTemp` 的默认值或 OwlMail 启动时的 umask。
没有任何权限被放宽：这里的每一处改动要么是收紧，要么是把一个碰巧正确的权限固定下来。

### 可配置的 SMTPS 端口

`-smtps-port` 与 `OWLMAIL_SMTPS_PORT` 用于指定 `-tls` 所启动的隐式 TLS 监听端口，
设为 `0` 则完全不启动 SMTPS 监听，这样部署就可以只在 SMTP 端口上提供 STARTTLS，
而不必再绑定第二个端口。

此前该监听固定在特权端口 465，因此官方容器镜像以非 root 用户运行时根本无法绑定它，
同一主机上的两个 OwlMail 实例也无法同时启用 TLS。更糟的是，OwlMail 会在尝试绑定
**之前**就打印 "SMTPS Server running"，然后丢弃绑定错误，于是这种配置产生的进程会
报告启动成功、通过就绪检查，却没有任何东西在接受 SMTPS 连接。现在，SMTPS 监听无法
绑定的配置会直接启动失败，错误信息指明无法绑定的地址并指向 `-smtps-port`，而那行
日志只在监听真正绑定成功之后才会写出。

### 模糊测试，以及它找到的缺陷

Go 模糊测试目标现在覆盖了 OwlMail 解析完全由攻击者控制的输入的四个位置：每条捕获
消息都会经过的 MIME 入口、输出会被 Web UI 预览渲染的 HTML 消毒器、为消息提供排序键
与留存时长的 `Date` 头回退逻辑，以及决定一条消息会到达哪些 Webhook 目标的 glob
匹配器。每个目标断言的是性质，而不只是"没有 panic"。一个 CI 任务会在每个 Pull
Request 上重放全部种子，并对每个目标模糊测试三十秒。

它找到了两个缺陷，本版本一并修复。`Mon, 01 Jan 0001 00:00:00 +0000` 这样的 `Date`
头能被 OwlMail 尝试的第一个布局干净地解析出来，于是绕过了当前时间回退——而它来自
邮件头，任何发件方都可以选它，让该消息在邮箱的整个生命周期里排在所有真实消息之前。
另一个是：同一份正文被消毒两次时，HTML 消毒器会丢掉样式表 `<link>`，因为
bluemonday 会追加自己的 `rel` 标记，结果不再匹配刚刚产出它的那条策略。

### 本版本的其他变化

- 只读 MCP HTTP 端点在同一条已认证路由上并行提供现代无状态 `2026-07-28` 协议与
  旧版有状态协议修订。
- Web Basic Auth 凭据改为常数时间比较。
- `.golangci.yml` 固定了 CI 此前隐式运行的 lint 规则集并新增十四个 linter；
  `.github/dependabot.yml` 按周分组调度依赖更新。
- 修改容器镜像的 Pull Request 现在会构建并运行该镜像，校验内嵌的发布元数据，
  并完整地经 SMTP 收发一封邮件。
- 对比与迁移指南新增 Mailpit 一栏，与其他项目一样固定到评审时的提交。
- health-kit、logger-kit 与 version-kit 升级到各自最新主版本。路由、响应体、
  响应头与日志字段均无变化。

## 升级说明

- **来自其他来源的浏览器客户端现在默认被拦截。** 如果有状态页、仪表盘或本地工具在
  浏览器里从不同来源读取 OwlMail API，请在升级前把该来源写入 `-web-allowed-origins`
  （`/mcp` 用 `-mcp-allowed-origins`）。`*` 可恢复此前行为。非浏览器调用方不发送
  `Origin`，无需改动。
- **SMTPS 监听此前静默失效的部署，现在会拒绝启动。** 最常见的就是在容器镜像里使用
  `-tls`，而非 root 用户无法占用 465 端口。请用 `-smtps-port` 换一个端口，或用
  `-smtps-port 0` 关闭它——后者仍保留 SMTP 端口上的 STARTTLS。
- **收紧后的权限只作用于升级之后写入的产物。** 不会追溯性地 chmod，因为悄悄改写
  既有邮件目录的权限会破坏那些有意与其他容器共享该卷的部署。要收紧既有目录：

  ```bash
  chmod 0750 "$MAIL_DIR"
  find "$MAIL_DIR" -mindepth 1 -type d -exec chmod 0700 {} +
  find "$MAIL_DIR" -mindepth 1 -type f -exec chmod 0600 {} +
  ```

- 相比 0.9.0，没有 REST API、SMTP 协议或存储格式变更。
- MCP 仍然默认关闭，启用后保持只读。
- MailDev 与 MailCatcher 兼容 facade 仍需显式启用。
- CI 应固定 `0.10.0` 或记录的 manifest digest，不应依赖会移动的 `main` 或 `latest`。

## 包含的 Pull Request

- [#126](https://github.com/soulteary/owlmail/pull/126) 将安全策略移入 `.github`
- [#127](https://github.com/soulteary/owlmail/pull/127) 保持安全策略的多语言链接有效
- [#128](https://github.com/soulteary/owlmail/pull/128) 现代与旧版 MCP 协议修订
- [#129](https://github.com/soulteary/owlmail/pull/129) MCP 端点的浏览器来源校验
- [#130](https://github.com/soulteary/owlmail/pull/130) 固定 lint 规则集与依赖更新计划
- [#131](https://github.com/soulteary/owlmail/pull/131) Docker 基础镜像更新
- [#132](https://github.com/soulteary/owlmail/pull/132) AWS SDK 更新
- [#133](https://github.com/soulteary/owlmail/pull/133) GitHub Actions 更新
- [#134](https://github.com/soulteary/owlmail/pull/134) 消除 outbox 阻塞测试与刷写协程的竞态
- [#135](https://github.com/soulteary/owlmail/pull/135) 在 Pull Request 上构建并运行容器镜像
- [#136](https://github.com/soulteary/owlmail/pull/136) Web Basic Auth 常数时间比较
- [#137](https://github.com/soulteary/owlmail/pull/137) AWS SDK 更新
- [#138](https://github.com/soulteary/owlmail/pull/138) `golang.org/x` 更新
- [#139](https://github.com/soulteary/owlmail/pull/139) 固定存储权限，不再创建全局可列邮件目录
- [#140](https://github.com/soulteary/owlmail/pull/140) 可配置的 SMTPS 端口与显式的绑定失败
- [#141](https://github.com/soulteary/owlmail/pull/141) Web UI 与 API 的浏览器来源校验
- [#142](https://github.com/soulteary/owlmail/pull/142) MIME、HTML、日期与 Webhook 解析器的模糊测试
- [#143](https://github.com/soulteary/owlmail/pull/143) 将 Mailpit 作为固定提交的对比列
- [#144](https://github.com/soulteary/owlmail/pull/144) `/version` 继续提供构建详情
- [#146](https://github.com/soulteary/owlmail/pull/146) AWS SDK 更新
- [#149](https://github.com/soulteary/owlmail/pull/149) Go 模块更新
- [#150](https://github.com/soulteary/owlmail/pull/150) GitHub Actions 更新
- [#151](https://github.com/soulteary/owlmail/pull/151) health-kit、logger-kit 与 version-kit 主版本升级

## 安装

```bash
docker pull ghcr.io/soulteary/owlmail:0.10.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.10.0
```

为了可重复部署，请记录已发布的 manifest digest 并使用
`ghcr.io/soulteary/owlmail@sha256:<digest>`。

## 发布文件

- `checksums.txt`
- `checksums.txt.sigstore.json`
- `owlmail-linux-amd64` 与 `owlmail-linux-amd64.spdx.json`
- `owlmail-linux-arm64` 与 `owlmail-linux-arm64.spdx.json`
- `owlmail-darwin-amd64` 与 `owlmail-darwin-amd64.spdx.json`
- `owlmail-darwin-arm64` 与 `owlmail-darwin-arm64.spdx.json`
- `owlmail-windows-amd64.exe` 与
  `owlmail-windows-amd64.exe.spdx.json`

Linux amd64 下载、校验与启动示例：

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.10.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.10.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.10.0/checksums.txt.sigstore.json
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

```bash
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.10.0
```

## 已知限制

- 来源校验保护的是浏览器客户端，不能替代认证：任何能向 API 发出非浏览器请求的东西
  仍然可以读取邮箱，因此请把 OwlMail 放在可信网络内。
- 收紧后的权限不会追溯应用，详见升级说明。
- MCP 仍为只读，不能删除、标记或中继消息。
- MailCatcher facade 未实现 MailCatcher 的 WebSocket 事件总线。
- 中继恢复是至少一次，而非恰好一次。
- GHCR 上准确的 `0.10.0` 标签在发布后不可变；需要修正时请发布补丁版本，而不是删除
  并复用已发布的制品。
