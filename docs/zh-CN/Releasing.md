# 发布流程

本文面向 OwlMail 维护者。每个发布版本由签名或附注的语义化版本标签标识，例如
`v0.5.0`。GitHub Release 二进制与容器镜像必须从同一个标签构建。

## 事实来源

- `CHANGELOG.md` 记录用户可感知的改动。
- `docs/en/Release-X.Y.Z.md` 是 GitHub Release 使用的英文发布正文。
- `docs/zh-CN/Release-X.Y.Z.md` 是对应的中文发布说明。
- `.github/workflows/release.yml` 构建二进制与校验和。
- `.github/workflows/docker.yml` 发布多架构镜像。

发布工作流会把英文策划版说明放在 GitHub 自动生成的变更列表之前。手动运行时，
如果版本标签或对应发布说明文件不存在，工作流会失败。

## 发布前检查

- [ ] 功能、修复、依赖、Go Report Card 与发布文档 PR 均已合并。
- [ ] `CHANGELOG.md` 及中英文发布说明覆盖从上一个标签开始的最终差异。
- [ ] 发布说明中没有未处理占位符或未经验证的兼容性承诺。
- [ ] 精确 `main` 提交上的必要检查均通过。
- [ ] `go test -race ./...`、`go vet ./...`、`go mod verify`、浏览器与文档测试通过。
- [ ] `govulncheck ./...` 没有可达漏洞，或每个例外都已记录。
- [ ] 多架构 Docker 构建成功。
- [ ] 使用持久数据进行升级冒烟测试前，已备份完整邮件目录。

对于 0.5.0，还需要确认 Go 1.27 依赖升级与仓库本地 Go Report Card 改动已在
打标签前合并。

## 创建发布标签

更新本地 `main`，记录准确提交，然后创建一个附注标签：

```bash
git switch main
git pull --ff-only
git status --short
git rev-parse HEAD
git tag -a v0.5.0 -m "OwlMail v0.5.0"
git push origin v0.5.0
```

不要移动或复用已经发布的标签。如需修正，应创建新的补丁版本。

推送标签会启动二进制与容器发布工作流。手动运行二进制发布工作流只用于重试：
必须提供已存在标签，工作流会先检出该标签再构建。

## 验证发布文件

除 GitHub 自动生成的源码归档外，0.5.0 的 GitHub Release 应包含以下 6 个上传文件：

- `checksums.txt`
- `owlmail-linux-amd64`
- `owlmail-linux-arm64`
- `owlmail-darwin-amd64`
- `owlmail-darwin-arm64`
- `owlmail-windows-amd64.exe`

将全部文件下载到空目录并校验：

```bash
sha256sum -c checksums.txt
```

至少冒烟测试一个二进制：

```bash
chmod +x owlmail-linux-amd64
./owlmail-linux-amd64 -smtp 11025 -web 11080
curl --fail http://localhost:11080/healthz
curl --fail http://localhost:11080/api/v1/version
```

两个端点及一次 SMTP 收件均成功后，停止冒烟测试进程。

## 验证容器发布

对于 `v0.5.0`，检查 `0.5.0`、`0.5`、`0` 与提交 SHA 标签，以及两个目标架构。
`main` 和 `latest` 是随默认分支移动的标签，不能用于证明发布可复现性。

```bash
docker buildx imagetools inspect ghcr.io/soulteary/owlmail:0.5.0
docker run --rm \
  -p 127.0.0.1:11025:1025 \
  -p 127.0.0.1:11080:1080 \
  ghcr.io/soulteary/owlmail:0.5.0
```

确认清单包含 `linux/amd64` 与 `linux/arm64`，再对容器重复健康、版本和 SMTP
冒烟测试。

## 发布后检查

- [ ] GitHub 将 `v0.5.0` 标记为最新非预发布版本。
- [ ] 策划版说明显示在自动生成的 PR 列表之前。
- [ ] 所有二进制与 `checksums.txt` 均可下载并通过校验。
- [ ] 容器版本标签和提交标签指向预期清单。
- [ ] 使用文档要求的 Go 版本执行 `@v0.5.0` 安装成功。
- [ ] README 与发布说明中的安装命令均可解析。
- [ ] 所有发布失败或已知限制均已补充到发布正文与变更日志。
