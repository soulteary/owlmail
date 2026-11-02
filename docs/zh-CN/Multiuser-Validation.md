# 多人模式契约、浏览器和容器验证

日期：2026-11-02。当前状态：0.14.0-alpha.1 已发布，公开多人启动门禁仍关闭。Gorge 未修改。

发布提交 `39e3ced315a027f6e5938146387179fa683fe7b1` 的 [CI](https://github.com/soulteary/owlmail/actions/runs/41457095521)、[Docker/Linux amd64 验证](https://github.com/soulteary/owlmail/actions/runs/41457095598) 和 [Release](https://github.com/soulteary/owlmail/actions/runs/41459011723) 已成功；后文 Podman/macOS 记录描述对应本地环境，不替代或否认后续远程结果。正式多人版本仍需真实 T02/T16 与正常二进制 T17 验收，见[当前进度](Multiuser-Progress.md)和[发布检查单](../en/Multiuser-Release-Readiness.md)。

## 模式化 OpenAPI

新增 `openapi/multiuser.json` / `multiuser.yaml`，25 个路径、27 个操作，精确覆盖多人 versioned 白名单。旧 `openapi/openapi.json` / `openapi.yaml` 保持不变。

`/api/v1/openapi.json` 和 `/api/v1/openapi.yaml` 根据服务构造模式选择契约，不能由请求参数切换服务模式。多人契约端点具有同样的可信身份要求。实际 base pathname 和经校验的 subject/email/verified Header 名称会同时反映在 JSON/YAML 中。

契约明确：可信实际代理连接、剥除客户端身份头、稳定 subject、已验证邮箱语义、历史地址归属限制、发布门禁、禁用能力、分页、个人箱不透明 cursor、等待容量和 SMTP 持久配额。身份头不是用户可自行获得的 API 密钥，也不构成 OIDC/Bearer 登录实现。

验证包括：OpenAPI 机器校验、两种格式语义等价、生成可重现性、实际注册路由集合、实际 JSON 响应 schema、自定义头、认证与 base path。操作 ID 无重复。

```sh
go generate ./openapi
go test -race ./openapi/... ./internal/api ./internal/identity
```

## 真实 Chrome

使用项目锁定的 Playwright 和隔离浏览器 profile，运行真实页面，不以 JS harness 替代。原单机套件 6/6 通过。

多人套件使用显式 `owlmail_browser_fixture` build tag 的独立命令，真实 TLS SMTP、隔离 HTTP 后端和 loopback 测试代理。身份和已验证邮箱声明为合成 Alice/Bob 数据，不能据此声称真实 Stargate 已满足验证契约。

已通过多人业务测试：

- 邮件先到、未验证不认领、可信声明认领。
- 账号切换后的列表/详情/附件及跨用户 ID、ZIP、批量请求隔离。
- 无 WebSocket，旧全局接口/MCP 不可用；个人清空不影响另一用户。
- 自然 15 秒定时刷新、关闭自动刷新、手动刷新、最多一个在途刷新。
- 保持选中详情，401 清空旧状态并停止轮询。

### 后台验证修正与超时定位

原 Linux 60 秒无输出超时发生在 Alpine `xvfb-run` 的 Xvfb 就绪信号握手：容器中只有包装脚本和已创建 X11 socket 的 Xvfb，没有 Node、fixture 或浏览器进程。该记录不代表浏览器用例已执行，也不能靠延长超时计为通过。

另一个独立问题是 Playwright 默认给受管页面启用焦点模拟，真实切换标签页仍可能报告 `visible`。后台专项改用临时独立 profile 启动原生 Chromium，使用锁定 Playwright 版本的 `connectOverCDP({noDefaults:true})` 连接已有默认 context，不创建会重新应用默认值的新 context。测试不覆盖 `document.hidden` / `visibilityState`，不冻结页面，也不使用用户浏览器 profile。

严格模式 `OWLMAIL_REQUIRE_REAL_BACKGROUND=1` 要求实际 `hidden → visible`，未隐藏直接失败，不能跳过。隐藏期间真实 TLS SMTP 投入新邮件，等待 16 秒，验证刷新请求数不增长、列表未增加；页面定时心跳继续运行，排除冻结页面造成的假通过。返回前台后 5 秒内必须显示新邮件、保留已选详情，且最大在途刷新数为 1。证据 JSON 和恢复后的截图分别保存在独立 background results 目录。

本轮 macOS Chrome 严格单项通过（1 passed、无 skip）：后台心跳 16 次，preview 请求数 `1 → 1`，前台恢复 64ms，最大在途刷新 1；新邮件出现且原详情保持。无效浏览器路径负向测试在 103ms 内明确报 ENOENT，不再无限等待。

Linux/arm64（rootless Podman、Chromium 156、Xvfb）严格单项通过（1 passed、无 skip）：真实隐藏 16 秒、心跳 16 次、preview `1 → 1`，前台恢复 28ms，详情保持、最大在途刷新 1。将旧包装脚本作为 PID 1 运行时卡住；加入 `--init` 的对照可完成 X11 握手，进一步定位了启动问题。

真实超时负向检查另外发现并修复了 detached fixture 服务残留：仅清理 Playwright 进程组不足。现在清理开始前捕获 runner 后代，按每个 PID 的 Linux starttime 复核后定向 TERM/KILL，不按全局进程名清理。30 秒实际超时返回 `124`，保持诊断容器运行后检查 `/proc`，无非 Z 状态的 fixture/Chromium/Xvfb；已退出的 zombie 明确交给容器 init 回收，不误报为仍在运行。清理范围限于开始时捕获的 owned 后代。

Linux runner 显式启动 Xvfb，验证 X11 就绪后才运行测试，并提供阶段日志、总时限和 owned 后代进程清理；不再使用阻塞的 `xvfb-run` 握手。复跑说明见 [容器验证说明](../../tests/container/README.md)。

成功截图位于运行生成的 `tests/browser/multiuser-evidence/`，由本次交付另附；失败报告不作为成功证据。真实截图还发现并修复了 Webhooks 按钮 hidden 被作者 CSS 覆盖的问题，最新单机套件再次通过。

构建及运行说明见 [浏览器测试说明](../../tests/browser/README.md)。普通 CLI 不接收测试身份选择参数；production binary 不包含测试 fixture 的 main。

## 真实容器

本机通过 rootless Podman 执行 Linux/arm64 构建和运行；不是 Docker daemon 的实跑结果。production 与 test-only fixture 镜像分开构建、分开报告。

正式 Dockerfile 构建通过，`go mod verify` 成功。构建使用校验过的模块缓存，未关闭 go.sum 校验。运行时源码输入已与当前工作树核对。修复 `.dockerignore` 对 go:embed HTML 的误排除，并排除浏览器依赖与报告产物。

正式镜像已验证：

- 非 root UID 1000、私有 EML 权限、嵌入 HTML。
- SMTP 附件投递、HTTP 读取、持久化重启、打包 healthcheck。
- 离线双向迁移、源 hash 保持、控制库与邮件分离。
- 正式 multiuser 和 stdio MCP 门禁。
- 只读 credential 卷：实际 write-open 被拒绝，而非仅依赖权限位测试。

独立测试镜像已验证真实 TLS SMTP、未验证待认领隔离、声明绑定、跨 ID/旧路由拒绝和持久重启。该镜像使用独立测试入口，不发布端口，不进入生产发布流程。

```sh
# 按目标平台设置 TARGETARCH；本机验证为 arm64，PR CI 使用 amd64。
docker build --build-arg TARGETARCH=arm64 -t owlmail:check .
docker build --build-arg BASE_IMAGE=owlmail:check --build-arg TARGETARCH=arm64 \
  -f tests/container/Dockerfile.multiuser-fixture -t owlmail:fixture .
python3 tests/container/multiuser.py --image owlmail:check --fixture-image owlmail:fixture
```

使用 Podman 时可给脚本传入 `--runtime podman --connection podman-machine-default`。脚本只清理自身创建的唯一容器/数据卷；不清理用户既有资源。

## CI 与发布边界

- 普通 Go CI 覆盖新旧模式的契约、路由和 schema 测试。
- 浏览器 job 保留单机套件，构建显式 tagged fixture；多人业务用例 headless 执行，真实后台用例由 Linux/Xvfb 独立严格执行。两步使用不同结果目录，保存成功截图、证据 JSON 和失败诊断。
- Docker PR verify job 构建本地 fixture 镜像并执行生产门禁/组件验证；不推送该测试镜像。正式发布仍只使用生产 Dockerfile。
- 上述本地验证记录形成时尚未推送远程 CI；后续 PR #214 的远程 CI、Docker 和 Alpha Release 已通过，见本页顶部工作流链接。旧失败与本地环境限制保留，不记为当前远程检查失败。

仍未满足真实 Stargate 已验证邮箱声明和真实 Phorge/Gorge 联调，不能解除多人发布门禁。单机兼容回归、组件测试与真实容器验证不等于已批准发布多人服务。

## 本轮最终本地检查

前轮全量 Go race、生产代码 lint、显式 tagged fixture lint、go vet 和正式构建通过。本轮正式二进制重新构建，模式/正式入口 Go 回归、单机浏览器 6 项、多人业务浏览器 2 项、macOS 与 Linux/Xvfb 严格后台单项各 1 项、219 项 Web/文档测试及 diff 检查通过。只读复核确认测试构建/CI 不给生产程序增加门禁旁路，测试镜像不推送。成功运行日志和截图随交付保存，不把早期失败或超时记录作为成功演示。

## 备份恢复与真实进程强杀

后续补充的一致备份、五个持久边界强杀及 macOS/Linux 验证见 [恢复演练](Multiuser-Recovery.md)。原有返回错误和人工 journal 测试仍是有用的故障测试，但不冒充真实进程崩溃。正式发布门禁继续保留。
