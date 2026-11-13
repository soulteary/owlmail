# OwlMail 0.15.0

单机稳定版发布说明，正常二进制保留多人启动门禁。制品可用性与核验结果由带标签的发布工作流确认；本文不预先声明发布摘要或制品核验已完成。

## 相对 0.14.0 的变化

- 直接打开邮件 HTML 接口地址时不再加载远程图片和样式，适用于单机、MailDev、MailCatcher 和多人 HTML 视图。链接打开时不携带 opener 和 referrer，CID 附件链接仍可下载，预览中显式加载远程内容的选项保持不变。
- 要求 Go 1.27.4，并更新 Fiber 至 v3.7.0、go-smtp 至 v0.25.1、modernc.org/sqlite 至 v1.62.0、soulteary kits（cli-kit v1.9.4、health-kit/v4 v4.0.7、logger-kit/v3 v3.0.6、version-kit/v4 v4.0.5）以及 AWS SDK 模块（aws-sdk-go-v2 v1.47.7、config v1.33.15、credentials v1.20.15、service/s3 v1.114.8）。
- 多人组件新增 `GET /api/v1/me/mailbox` 和多语言收件箱提示，说明个人收件箱为空的原因，不返回地址、所有者 ID 或邮件数量。
- 新增离线 `recipient-route` 命令，用于查看、阻断、重新分配和恢复后续收件地址路由，变更需要精确的计划令牌并写入审计记录，见[收件地址路由操作](Recipient-Route-Operations.md)。
- 通过持久的 `<database>.writer-lock` 旁路文件拒绝第二个控制库写入者，并以 SQLite 文件 URI 编码控制库路径，见[控制库写入锁（English）](../en/Control-Writer-Lock.md)。
- 多人界面拒绝代理登录跳转、缺少身份标记或非 JSON 的受保护响应：清空私有状态、停止轮询，并提供显式的本地化重新登录操作。
- 多人收件箱刷新进行中的搜索、筛选和翻页变更不再丢失；定时轮询增加 0–1500 毫秒抖动；自动刷新控件完成本地化。
- 限定多人维护工作量：认领缓存上限 4,096 个主体，在线归并恢复与待认领收件箱保留清理每批最多 32 封邮件，路由 SMTP 在分发前预检配额。
- 收件地址在 IDNA 规范化之后再校验 254 字节长度上限。
- 服务端要求收件箱归属时，诊断日志不再包含邮件主题和提交的内容。单机日志保持不变。

## 可用范围与已知限制

单机仍为默认模式，保留现有 SMTP、Web、MailDev、MCP 与 Relay 行为。**正常发布二进制仍拒绝启动多人模式。** 上述多人相关变化作用于保留门禁的组件，并使用合成可信身份验证；没有发布运行时绕过开关。

Stargate 尚未提供自动认领所需的可信“邮箱已验证”声明；真实部署的 Phorge → Gorge → OwlMail 投递，以及 Stargate 认证后的个人箱访问闭环仍未验收。Gorge 保持不变。

控制库写入锁是建议性锁，旧版二进制和直接执行的 SQL 不会遵守；升级或离线变更前必须停止所有旧版写入进程。分批上限约束的是每一步的邮件变更数量，不是文件系统最坏情况下的延迟。

## 升级与回滚

1. 停止 SMTP 入口和旧进程，完整备份单机邮件目录，包括 EML、附件、元数据、Relay/outbox；外部 Redis/S3 状态另行备份，记录启动参数与旧镜像 digest。
2. 先在隔离副本上演练，停用真实出站目的地，检查健康/就绪、历史邮件附件、新 SMTP 捕获和重启恢复。Relay 仍采用至少一次投递语义。
3. 直接打开邮件 HTML 接口地址的客户端不再加载远程图片和样式；需要渲染远程资源时，请使用收件箱预览及其加载远程内容的选项。
4. 从源码构建需要 Go 1.27.4 或更高版本。
5. 不要将生产目录切换为多人模式。隔离目录必须保留完整控制数据库和所有权状态，不能作为单机目录打开，也不能只用 EML 恢复；显式迁移和收件地址路由变更应离线并先完整备份。
6. 回滚时停止此版本，恢复完整升级前快照后再运行 0.14.0。不要让两个版本共用可写目录；核对待发送任务并另行保留快照后的新邮件。

发布工作流发布精确版本及适用的 `latest`、`0`、`0.15` 别名。实际 manifest 摘要及制品核验结果在工作流完成后记录。

上一版：[0.14.0](Release-0.14.0.md)。

## 安装

在带标签的发布工作流提供制品后使用以下命令。

```bash
docker pull ghcr.io/soulteary/owlmail:0.15.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.15.0
```

为确保可重复部署，请记录发布镜像的 manifest 摘要，并使用
`ghcr.io/soulteary/owlmail@sha256:<digest>` 固定镜像。

## 发布产物

- `checksums.txt` 和 `checksums.txt.sigstore.json`
- `owlmail-linux-amd64` 和 `owlmail-linux-amd64.spdx.json`
- `owlmail-linux-arm64` 和 `owlmail-linux-arm64.spdx.json`
- `owlmail-darwin-amd64` 和 `owlmail-darwin-amd64.spdx.json`
- `owlmail-darwin-arm64` 和 `owlmail-darwin-arm64.spdx.json`
- `owlmail-windows-amd64.exe` 和 `owlmail-windows-amd64.exe.spdx.json`
- `owlmail-licenses.tar.gz`

Linux amd64 下载、校验和启动示例：

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.15.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.15.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.15.0/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.15.0/owlmail-licenses.tar.gz
grep ' owlmail-linux-amd64$' checksums.txt | sha256sum -c -
grep ' owlmail-licenses.tar.gz$' checksums.txt | sha256sum -c -
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
tar -xzf owlmail-licenses.tar.gz
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64
```

重新分发二进制文件时，请一并保留解压后的 `owlmail-licenses/` 目录。
该许可证包覆盖 OwlMail 及其 Go 依赖；镜像中的操作系统软件包遵循各自的许可条款。

```bash
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.15.0
```
