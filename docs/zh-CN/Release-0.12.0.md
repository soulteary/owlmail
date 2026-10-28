# OwlMail 0.12.0

发布日期：2026-10-29

本版完善内联附件兼容性，提供结构化回复地址，并允许并行测试直接绑定独立的 SMTP 与 HTTP 随机端口。包含 PR #206，以及 0.11.4 之后的 health-kit 4.0.4 依赖更新。

## 主要改动

- CID 重写覆盖图片 `src` 和附件链接 `href`，支持正确的 API 路径与部署子路径、带尖括号的 Content-ID、HTML 实体和单次 URI 解码；未知或存在歧义的引用保持原值。
- 递归解析嵌套 MIME alternative/related，限制为最多 32 层、10,000 个部件；保留附件流式存储和无效邮件回滚。
- 原生 REST 与 MailDev 详情新增可选 `replyTo`，只读 MCP 详情新增可选 `reply_to`，浏览器以安全转义文本显示回复地址。缺省、格式错误或重复头不生成结构化字段，原始头仍保留；`In-Reply-To` 继续表示 Message-ID。
- `-smtp 0`、`-web 0` 由操作系统分配端口；`-listen-info-file` 原子发布实际地址，配置接口、Origin 校验、MCP 链接和健康探针使用实际端口。
- 并发关闭会等待同一次资源清理完成，主服务退出前等待 API 启动协程结束，并清理发现文件。

## 兼容性与升级

升级前请备份完整邮件目录。存量 Content-ID 与附件文件名不迁移，重启恢复保持原值。Reply-To 为可选新增字段，列表摘要和搜索契约不变。旧 Go SMTP 构造函数保持 `0 = 默认端口`，Go 调用方通过 `ServerOptions{AllowEphemeralPort: true}` 显式启用随机端口。`-smtps-port 0` 仍表示禁用隐式 TLS，下游 SMTP 端口必须为 1–65535。

每个实例使用独立的发现文件。异常退出后，确认原 owner 已退出再清理残留锁；发现文件只提供本地地址元数据，不承担鉴权。CID CSS `url()` 和 `srcset` 不在本版范围，预览 sandbox 权限及远程资源默认策略保持不变。

## 验证

PR #206 的 race、vet、lint、浏览器回归、fuzz、CodeQL 和镜像验证均已通过。本地验证覆盖 20 个 Go 包、205 项前端/文档单元测试、5 项 Chromium 回归，以及双实例随机端口收信、原生/MailDev 附件内容、健康检查、关闭和重启恢复。Windows amd64 完成交叉编译，本地运行时测试在 macOS 完成。标签发布流程会在发布产物前重新执行预检和可达漏洞扫描。

上一版本：[0.11.4](Release-0.11.4.md)。

## 安装

```bash
docker pull ghcr.io/soulteary/owlmail:0.12.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.12.0
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
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.12.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.12.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.12.0/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.12.0/owlmail-licenses.tar.gz
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
  ghcr.io/soulteary/owlmail:0.12.0
```
