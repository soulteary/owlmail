# OwlMail 0.14.0

单机稳定版发布说明，正常二进制保留多人启动门禁。制品可用性与核验结果由带标签的发布工作流确认；本文不预先声明发布摘要或制品核验已完成。

## 相对 0.13.0 的变化

- 新增独立个人收件箱基础设施：系统级 SMTP 凭据与收件地址路由、可信投递建立待认领收件箱、稳定身份绑定、全接口隔离、多副本持久接收、归并与保留策略恢复、持久准入配额。
- 新增轮询界面与模式化 OpenAPI。多人组件停用 MCP、WebSocket、共享空间、出站 Relay 和全局收件箱操作。
- 新增离线 doctor/迁移工具，以及真实进程中断、不可变备份恢复、SQLite 派生索引重建、浏览器、卷、TLS、ACL 和 SELinux 隔离实验验证。
- Relay 正常关闭会等待 readiness 探针完成，避免临时文件写入越过成功关闭边界；文件系统持续阻塞时返回明确的有界关闭错误。
- 补充 explicit-history 的真实 SMTP/HTTP 组件测试，覆盖回收地址、竞争身份和跨登录投递。
- 通过隔离组件测试验证 Gorge 现有 SMTP 传输和固定版本的原始 Phorge PHP 邮件适配器，覆盖 MIME/附件保持与永久收件人拒绝；这些测试不部署 Phorge 通知授权、数据库和 worker。
- 为原生 Chromium 测试目录清理提供有界暂态重试，业务断言和永久错误失败保持。

## 可用范围与已知限制

单机仍为默认模式，保留现有 SMTP、Web、MailDev、MCP 与 Relay 行为。**正常发布二进制仍拒绝启动多人模式。** 此版本支持单机运行，多人基础设施仍保留门禁，不是已开放的多人服务；没有发布运行时绕过开关。正常发布二进制和容器镜像不包含 fixture 可执行入口及外部 PHP/Gorge 运行时。GitHub 源码归档包含仓库测试源码，这不构成生产 fixture 入口。

Stargate 尚未提供自动认领所需的可信“邮箱已验证”声明。Gorge 现有 SMTP 传输已有隔离组件测试；真实部署的 Phorge → Gorge → OwlMail 投递，以及 Stargate 认证后的个人箱访问/认领闭环仍未验收。Gorge 保持不变，采用系统级 SMTP 凭据而非每用户密码。真实上游身份验收仍待完成。双物理宿主恢复和生产 NFS 支持不在本次范围，不是独立个人收件箱正式版的统一前置。

NFSv4/RichACL 证据来自同一 VM 内的隔离补丁实验栈与 numeric AUTH_SYS，不能据此宣称生产 NFS、Kerberos/域身份、任意服务端或 SQLite-on-NFS 部署支持。进程硬退出验证不等于硬件掉电耐久性或 SMTP 恰好一次投递证明。

## 升级与回滚

1. 停止 SMTP 入口和旧进程，完整备份单机邮件目录，包括 EML、附件、元数据、Relay/outbox；外部 Redis/S3 状态另行备份，记录启动参数与旧镜像 digest。
2. 先在隔离副本上演练，停用真实出站目的地，检查健康/就绪、历史邮件附件、新 SMTP 捕获和重启恢复。Relay 仍采用至少一次投递语义。
3. 不要将生产目录切换为多人模式。隔离目录必须保留完整控制数据库和所有权状态，不能作为单机目录打开，也不能只用 EML 恢复；显式迁移应离线并先完整备份。
4. 回滚时停止此版本，恢复完整升级前快照后再运行 0.13.0。不要让两个版本共用可写目录；核对待发送任务并另行保留快照后的新邮件。

发布工作流发布精确版本及适用的 `latest`、`0`、`0.14` 别名。实际 manifest 摘要及制品核验结果在工作流完成后记录。

上一版：[0.13.0](Release-0.13.0.md)。

## 安装

在带标签的发布工作流提供制品后使用以下命令。

```bash
docker pull ghcr.io/soulteary/owlmail:0.14.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.14.0
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
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0/checksums.txt.sigstore.json
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.14.0/owlmail-licenses.tar.gz
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
  ghcr.io/soulteary/owlmail:0.14.0
```
