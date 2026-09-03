# OwlMail 0.9.0 发布说明

OwlMail 0.9.0 聚焦开发者、CI 流水线与编码智能体所需的可重复集成测试和可信文档。
本版本让 JavaScript、Python 与 Go 示例持续接受端到端验证，强化失败清理路径，并通过
单一版本来源让发布元数据可核验。

0.9.0 版本日期为 2026-09-03。以下命令使用 `v0.9.0` 标签与 `0.9.0` 容器镜像，
请在标签和制品可用后执行。

## 版本亮点

### 可运行示例

无外部依赖的 JavaScript、Python 示例和 Go 示例现在会针对同一提交构建的 OwlMail
执行端到端测试：通过 SMTP 投递邮件，经 REST API 等待并校验消息，最后清理测试邮件。
工作流保留手动触发，但 push 和 Pull Request 只在 Go 源码、模块文件、示例或工作流
自身变化时运行。

三种语言的失败处理均已明确。JavaScript 在清理同时失败时保留原始校验异常，拒绝清理
重定向，归一化连续尾部斜线，并避免缓冲响应体；Python 会在断言失败后清理邮件，并有
专门的失败路径测试；Go 支持 IPv6 SMTP 主机、报告清理错误、排空有界的非成功响应，
并能在 SMTP 已接受邮件后重新查询消息 ID。

### 文档契约

文档测试现在从根目录 `VERSION` 派生当前版本、标签、镜像、发布说明路径和示例。测试
会验证可执行 shell 命令的结构，而不只检查字符串；覆盖所有已注册的机器接口路由、
MCP 的具体 HTTP 方法与 Prompt 参数，并拒绝未固定提交的 GitHub Actions 引用。

七份根 README 及中英文文档索引将 OwlMail 定位为 AI 原生集成测试网关，同时继续明确
安全边界：MCP 为可选功能，默认关闭、范围有界、保持只读，且不是 SMTP 或 Web 核心
服务的必要依赖。

### 发布一致性

`VERSION` 是当前稳定文档版本的单一事实来源。发布工作流在检出标签后检查请求版本，
版本不一致时拒绝发布。当前发布镜像、源码安装、API 示例、Issue 模板、发布说明链接与
运维命令会作为一组同步更新。

Go Report Card 徽章工作流也不再让仅更新生成徽章的 push 重复运行完整 CI 和镜像构建，
同时保留 Pull Request 的完整验证。

## 升级说明

- 相比 0.8.0，没有运行时 API、SMTP 协议、配置或存储格式变更，可直接升级。
- MCP 仍然默认关闭，启用后保持只读。
- MailDev 与 MailCatcher 兼容 facade 仍需显式启用。
- CI 应固定 `0.9.0` 或记录的 manifest digest，不应依赖会移动的 `main` 或 `latest`。

## 包含的 Pull Request

- [#109](https://github.com/soulteary/owlmail/pull/109) AI 原生 README 定位
- [#110](https://github.com/soulteary/owlmail/pull/110) 完整测试与 AI-first 文档
- [#111](https://github.com/soulteary/owlmail/pull/111) 发布和集成指南修正
- [#112](https://github.com/soulteary/owlmail/pull/112) Go 清理错误报告
- [#113](https://github.com/soulteary/owlmail/pull/113) Go 示例支持 IPv6 SMTP 主机
- [#114](https://github.com/soulteary/owlmail/pull/114) 完整 MCP Prompt 参数
- [#115](https://github.com/soulteary/owlmail/pull/115) Python 失败清理
- [#116](https://github.com/soulteary/owlmail/pull/116) 语义化文档测试
- [#117](https://github.com/soulteary/owlmail/pull/117) 徽章提交避免镜像构建
- [#118](https://github.com/soulteary/owlmail/pull/118) JavaScript 保留原始异常
- [#119](https://github.com/soulteary/owlmail/pull/119) 可靠的集成示例清理回退
- [#120](https://github.com/soulteary/owlmail/pull/120) 完整机器接口路由覆盖
- [#121](https://github.com/soulteary/owlmail/pull/121) 徽章提交避免完整 CI
- [#122](https://github.com/soulteary/owlmail/pull/122) 在线端到端示例 CI
- [#123](https://github.com/soulteary/owlmail/pull/123) 明确 MCP HTTP 方法
- [#124](https://github.com/soulteary/owlmail/pull/124) 发布元数据与 CI 范围一致性

## 安装

```bash
docker pull ghcr.io/soulteary/owlmail:0.9.0
docker run --rm \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  -v owlmail-data:/app/mail \
  ghcr.io/soulteary/owlmail:0.9.0
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

Linux amd64 下载、校验与启动示例：

```bash
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.9.0/owlmail-linux-amd64
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.9.0/checksums.txt
curl -fLO https://github.com/soulteary/owlmail/releases/download/v0.9.0/checksums.txt.sigstore.json
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
  ghcr.io/soulteary/owlmail:0.9.0
```

## 已知限制

- MCP 保持只读，不提供删除、标记或 Relay 邮件能力。
- MailCatcher facade 不实现 MailCatcher WebSocket 事件总线。
- Relay 恢复为 at-least-once，而不是 exactly-once。
- GHCR 的准确 `0.9.0` 标签发布后不可变；如需修正，应发布补丁版本，而不是删除并
  复用已发布制品。
