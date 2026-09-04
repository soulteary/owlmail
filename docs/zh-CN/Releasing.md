# 发布流程

本文面向 OwlMail 维护者。每个发布版本由签名或附注的语义化版本标签标识，例如
`v0.9.0`。GitHub Release 二进制与容器镜像必须从同一个标签构建。

## 事实来源

- `VERSION` 记录不带 `v` 前缀的当前稳定文档版本；发布标签必须与该值一致。
- `CHANGELOG.md` 记录用户可感知的改动。
- `docs/en/Release-X.Y.Z.md` 是 GitHub Release 使用的英文发布正文。
- `docs/zh-CN/Release-X.Y.Z.md` 是对应的中文发布说明。
- `.github/workflows/release.yml` 从标签构建正式二进制、校验和、逐二进制 SBOM、
  来源证明、签名及多架构发布镜像。
- `.github/workflows/docker.yml` 只为默认分支发布会移动的 `main`、`latest` 与提交
  快照。

发布工作流会把英文策划版说明放在 GitHub 自动生成的变更列表之前。手动运行时，
如果版本标签或对应发布说明文件不存在，工作流会失败。

## 发布前检查

- [ ] 功能、修复、依赖、Go Report Card 与发布文档 PR 均已合并。
- [ ] `VERSION` 与准备创建的标签及 README 中的当前版本示例一致。
- [ ] `CHANGELOG.md` 及中英文发布说明覆盖从上一个标签开始的最终差异。
- [ ] 发布说明中没有未处理占位符或未经验证的兼容性承诺。
- [ ] 精确 `main` 提交上的必要检查均通过。
- [ ] `go test -race ./...`、`go vet ./...`、`go mod verify`、浏览器与文档测试通过。
- [ ] `govulncheck ./...` 没有可达漏洞，或每个例外都已记录。
- [ ] `.bun-version` 与发布工作流中固定的 Go、Bun、`govulncheck` 版本符合本次
  发布使用的工具链。
- [ ] 多架构 Docker 构建成功。
- [ ] 使用持久数据进行升级冒烟测试前，已备份完整邮件目录。

对于 0.9.0，还需要确认三种可运行集成示例均通过在线工作流，失败与清理路径已有
测试覆盖，API 与 MCP 文档契约通过，并且所有当前版本示例均由 `VERSION` 派生或已在
打标签前完成更新。

## 创建发布标签

更新本地 `main`，记录准确提交，然后创建一个附注标签：

```bash
git switch main
git pull --ff-only
git status --short
git rev-parse HEAD
git tag -a v0.9.0 -m "OwlMail v0.9.0"
git push origin v0.9.0
```

不要移动或复用已经发布的标签。如需修正，应创建新的补丁版本。

推送标签会启动统一发布工作流。手动运行只用于重试，并且必须在同一个标签引用
上执行，使 OIDC 身份与标签绑定：

```bash
gh workflow run release.yml --ref v0.9.0 -f version=v0.9.0
```

工作流会拒绝引用和请求版本不一致的手动运行，然后检出该标签再构建。发布文件
上传前，任务会对该
标签重新执行依赖校验、格式检查、`go vet`、带竞态检测的 Go 测试、
`govulncheck` 以及 Bun 浏览器/文档检查，随后生成 SPDX SBOM、GitHub Artifact
Attestation 与 Sigstore 无密钥签名，再正式发布。

构建前，工作流会查询 GHCR；如果准确版本标签已经存在，则以失败关闭方式停止。
因此，手动重试只适用于镜像尚未发布的阶段；镜像发布后如需修正，应创建新的补丁
版本，而不是覆盖已有制品。发布工作流也不会重新发布默认分支使用的 `sha-*` 别名。

只有请求标签仍是仓库中最新的稳定 SemVer 标签时，工作流才会更新 `latest`、主版本
和次版本别名，避免运维重试导致使用移动镜像标签的部署被降级。生成镜像标签前，
工作流会重新拉取远端标签并再次检查这个条件；同时禁用 metadata action 自动生成的
`latest`，因此只有显式守卫的别名能够发布。

## 验证发布文件

除 GitHub 自动生成的源码归档外，使用当前工作流发布的版本应包含：

- `checksums.txt`
- `checksums.txt.sigstore.json`
- `owlmail-linux-amd64`
- `owlmail-linux-amd64.spdx.json`
- `owlmail-linux-arm64`
- `owlmail-linux-arm64.spdx.json`
- `owlmail-darwin-amd64`
- `owlmail-darwin-amd64.spdx.json`
- `owlmail-darwin-arm64`
- `owlmail-darwin-arm64.spdx.json`
- `owlmail-windows-amd64.exe`
- `owlmail-windows-amd64.exe.spdx.json`

将全部文件下载到空目录并校验：

```bash
sha256sum -c checksums.txt
gh attestation verify owlmail-linux-amd64 --repo soulteary/owlmail
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

校验和清单与 GitHub provenance 同时覆盖 5 个可执行文件及其 5 个
`*.spdx.json` SBOM。

至少冒烟测试一个二进制：

```bash
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64 -smtp 11025 -web 11080
curl --fail http://localhost:11080/healthz
curl --fail http://localhost:11080/api/v1/version
```

两个端点及一次 SMTP 收件均成功后，停止冒烟测试进程。

## 验证容器发布

对于 `v0.9.0`，检查 `0.9.0`、`0.9`、`0` 标签和两个目标架构。`main`、`latest`
与 `sha-*` 都是默认分支别名，不能用于证明发布可复现性。记录正式发布的清单 digest，
并在可重复部署中使用该 digest。

```bash
docker buildx imagetools inspect ghcr.io/soulteary/owlmail:0.9.0
cosign verify \
  --certificate-identity-regexp '^https://github.com/soulteary/owlmail/.github/workflows/release.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/soulteary/owlmail:0.9.0
docker run --rm \
  -p 127.0.0.1:11025:1025 \
  -p 127.0.0.1:11080:1080 \
  ghcr.io/soulteary/owlmail:0.9.0
```

确认清单包含 `linux/amd64` 与 `linux/arm64`，再对容器重复健康、版本和 SMTP
冒烟测试。镜像清单还应包含 BuildKit SBOM、最高模式 provenance、GitHub
provenance、Cosign 签名，以及明确的 OCI 来源、修订、版本和 MIT 许可证标签。

## 发布后检查

- [ ] GitHub 将 `v0.9.0` 标记为最新非预发布版本。
- [ ] 策划版说明显示在自动生成的 PR 列表之前。
- [ ] 所有二进制、SBOM、校验和、签名包与 GitHub Attestation 均可下载或发现，
  并通过校验。
- [ ] 容器版本标签和移动标签指向预期清单。
- [ ] 已记录正式多架构清单的 digest，供精确部署使用。
- [ ] 发布镜像的 Cosign 签名与 OCI Attestation 通过验证。
- [ ] 使用文档要求的 Go 版本执行 `@v0.9.0` 安装成功。
- [ ] README 与发布说明中的安装命令均可解析。
- [ ] 所有发布失败或已知限制均已补充到发布正文与变更日志。
