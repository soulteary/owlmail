# OwlMail 0.13.0

发布日期：2026-11-02（Asia/Shanghai）。

## 相对 0.12.0 的变化

- 原生 API、自动转发、历史 Relay 路由和 MailDev 兼容调用共用持久化 Relay 任务所有者，保存信封收件人、重试状态与源邮件保护；SMTP-only 模式也能恢复。
- 自动转发依次持久化准备意图、暂挂任务和提交确认，中断后可安全继续交接。
- 邮件元数据独立保留最近一次下游 SMTP DATA 接受时间和实际收件人，不再依赖有上限的 Relay 历史；原生、MailDev、MCP 详情和收件箱均展示该记录。
- 成功任务的接受记录可以只修复而不重新发送；持久化修复完成前保持就绪失败。旧版排队 JSON 在投递前冻结收件人，异常恢复状态不会静默丢弃。
- 更新已发布的 kit 补丁依赖及其所需 SQLite/libc 校验和，继续使用 Go 1.27.3。保留 Reply-To、MIME/CID、随机端口、REST 与只读 MCP 功能。

## 升级、兼容与回退

1. 停止新 SMTP 流量及旧进程，备份完整邮件目录，包含 Relay 任务、元数据、EML/附件和 outbox；使用 Redis/S3 时另行备份外部状态，记录原镜像摘要和有效配置。
2. 先在隔离副本上验证，并禁用真实下游投递：恢复会继续处理待发送任务。SMTP-only 部署保留原参数，恢复不再依赖 Web 界面启用。
3. 检查 `/healthz` 与就绪状态、历史邮件及附件、新 SMTP 捕获、原生/MailDev/MCP 读取、投递接受记录和重启恢复。遇到排队、暂挂、交接或接受记录修复状态时排查原因，不要删除数据来强行通过就绪检查。
4. 投递仍是至少一次。下游接受后、持久化确认前崩溃仍可能重复发送；接受记录不等于恰好一次或最终送达收件箱，手动重试也可能再次发送。
5. 回退不是反向迁移：停止 0.13.0，恢复升级前完整目录与外部存储后再启动 0.12.0。两个版本不得共用可写状态。恢复快照可能重新引入待发送任务，重新启用出站前核对下游记录；快照后的新邮件需另行保留并核对。

上一版：[0.12.0](Release-0.12.0.md)。标签发布流程会重新执行 race、漏洞检查、二进制元数据检查、许可证打包及签名多平台发布；以实际工作流结果作为发布完成证据。

## 安装

```bash
docker pull ghcr.io/soulteary/owlmail:0.13.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.13.0
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
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.13.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.13.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.13.0/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.13.0/owlmail-licenses.tar.gz
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
  ghcr.io/soulteary/owlmail:0.13.0
```
