# OwlMail 0.8.0 发布说明

OwlMail 0.8.0 将项目进一步完善为面向开发者、CI 流水线与编码智能体的邮件测试
网关。本版本新增可持久化、可观测的 Relay 投递、分层配置、可选 SQLite 邮箱索引、
只读 MCP stdio 桥接、默认关闭的 MailCatcher REST 兼容层，以及更安全高效的收件箱
工作流。

以下命令需在 `v0.8.0` 标签正式发布后使用。

## 版本亮点

### 持久化中继任务

收件箱中的手动 Relay，以及原生
`/api/v1/emails/:id/actions/relay` 端点请求现在返回异步任务 ID。保留的无版本
`/email/:id/relay` 兼容路由继续返回历史 HTTP 200 响应，不返回任务 ID。原生 API
接受的任务会持久化到邮件目录，在重启后恢复，并针对有界的瞬时故障进行重试；状态仅
暴露安全错误类别。任务仍需要源 EML
时，删除与清理流程会保护该文件。启动顺序也会协调 SMTP、Web API、存储清理与 Relay
恢复，避免排队任务过早可见或源文件被删除。

运行时出站配置使用每个任务独立的不可变快照。启用、禁用、凭据更新、入队与关闭均
避免数据竞争和生产者侧关闭 channel；已经接受的任务继续使用原配置，新请求则获得
确定的 disabled、full 或 closed 错误。

### 分层配置与索引

OwlMail 可通过 `-config` 或 `OWLMAIL_CONFIG_FILE` 加载扁平 YAML/JSON 配置。
优先级明确为：CLI、MailDev 环境变量别名、OwlMail 环境变量、配置文件、内置默认值。
未知、重复、嵌套、null、超限及类型错误的配置都会失败关闭。

可选 SQLite 邮箱索引可以加快重启和查询，同时仍以 EML 与 sidecar 文件为权威数据源。
Prometheus 指标与结构化 JSON 日志可用于更接近生产环境的测试部署。

### MCP stdio 桥接与客户端兼容

`owlmail mcp-stdio` 通过官方 stdio transport 暴露既有的只读 MCP tools、resources、
prompts、限制和邮箱存储。协议帧仅写入 stdout，日志仅写入 stderr，适合本地编码智能体
配置。

默认关闭的 MailCatcher REST facade 覆盖邮件列表、详情、HTML、纯文本、源码、EML、
CID part 与删除接口，并复用 OwlMail 的 Basic Auth、HTTPS、存储和 base path。
OwlMail ID 仍是 opaque string，该 facade 不模拟 MailCatcher WebSocket 事件总线。

### 收件箱与运维工作流

收件箱新增 HTML、纯文本、邮件头和原始源码标签页、浏览器历史、键盘导航，以及带保护
的手动 Relay 控件。Relay 操作会展示异步任务 ID、防止重复提交，并要求用户明确确认
将发送真实邮件。

SMTP greeting 长度、命令长度、收件人数、邮件大小、读写超时和 DATA 并发可以独立
配置。API 列表查询和批量修改会拒绝非法或超限输入，不再静默归一化。

## 安全与性能

- 出站 SMTP 明确区分 `plain`、强制 `starttls` 和隐式 `smtps`。STARTTLS、
  证书及主机名校验失败时不会降级到明文，明文连接也不得发送凭据。
- 可能主动执行的 HTML、SVG、XML 与 JavaScript 附件强制下载，并设置 `nosniff`
  与安全文件名。
- Relay DATA 与原始源码响应从经过验证的 EML 路径流式读取，不再分配邮件等大的缓冲；
  过滤导出使用轻量摘要并保留消息数和总字节限制。
- Relay 配置、队列提交、恢复、删除保护与关闭流程增加竞态和失败路径覆盖。

## 升级说明

- 原有 CLI 与环境变量继续受支持；配置文件为可选项，优先级低于环境变量和 CLI。
- SQLite 索引、Prometheus、MailCatcher facade 与两种 MCP transport 均保持显式启用。
- 依赖非法列表参数被静默忽略的客户端，需要改为发送合法日期、已读条件、排序字段、
  排序方向和有界 ID 批次。
- Relay 持久化恢复为 at-least-once：远端 SMTP 已接受 DATA、但 OwlMail 尚未记录完成
  时发生崩溃，可能产生重复投递。
- 使用持久化数据验证升级前，请备份完整邮件目录。

## 包含的 Pull Request

- [#91](https://github.com/soulteary/owlmail/pull/91) Web 历史与键盘导航
- [#90](https://github.com/soulteary/owlmail/pull/90) SMTP 协议限制配置
- [#89](https://github.com/soulteary/owlmail/pull/89) MailDev/MailCatcher 对比文档
- [#93](https://github.com/soulteary/owlmail/pull/93) 异步 Relay 状态
- [#92](https://github.com/soulteary/owlmail/pull/92) Prometheus 指标
- [#94](https://github.com/soulteary/owlmail/pull/94) 可选 SQLite 邮箱索引
- [#97](https://github.com/soulteary/owlmail/pull/97) 生命周期安全的出站配置
- [#95](https://github.com/soulteary/owlmail/pull/95) 流式 SMTP Relay
- [#96](https://github.com/soulteary/owlmail/pull/96) 出站 SMTP TLS 失败关闭
- [#98](https://github.com/soulteary/owlmail/pull/98) 附件内容隔离
- [#100](https://github.com/soulteary/owlmail/pull/100) 流式源码与有界导出
- [#101](https://github.com/soulteary/owlmail/pull/101) 结构化 JSON 日志
- [#99](https://github.com/soulteary/owlmail/pull/99) 严格 API 查询与批量边界
- [#102](https://github.com/soulteary/owlmail/pull/102) 邮件内容标签页
- [#107](https://github.com/soulteary/owlmail/pull/107) 持久化 Relay 任务
- [#106](https://github.com/soulteary/owlmail/pull/106) 分层 YAML/JSON 配置
- [#105](https://github.com/soulteary/owlmail/pull/105) MailCatcher REST facade
- [#104](https://github.com/soulteary/owlmail/pull/104) 只读 MCP stdio 桥接
- [#103](https://github.com/soulteary/owlmail/pull/103) 安全手动 Relay 控件

## 安装

```bash
docker pull ghcr.io/soulteary/owlmail:0.8.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.8.0
```

可重复部署应记录正式 manifest digest，并使用
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

```bash
sha256sum -c checksums.txt
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.8.0
```

## 已知限制

- MCP 保持只读，不提供删除、标记或 Relay 邮件能力。
- MailCatcher facade 不实现 MailCatcher WebSocket 事件总线。
- Relay 恢复为 at-least-once，而不是 exactly-once。
- GHCR 的准确 `0.8.0` 标签发布后不可变；如需修正，应发布补丁版本，而不是删除并
  复用已发布制品。
