# OwlMail 多人模式实施进度

更新日期：2026-11-02。实现已通过 [PR #214](https://github.com/soulteary/owlmail/pull/214) 合并到 `main`，合并提交为 `39e3ced315a027f6e5938146387179fa683fe7b1`。

## 当前发布状态

[0.14.0-alpha.1](https://github.com/soulteary/owlmail/releases/tag/v0.14.0-alpha.1) 已发布为预发布版本，[Release 工作流](https://github.com/soulteary/owlmail/actions/runs/41459011723) 全部成功。发布提交的 [CI](https://github.com/soulteary/owlmail/actions/runs/41457095521)、[Docker 验证](https://github.com/soulteary/owlmail/actions/runs/41457095598)、[CodeQL](https://github.com/soulteary/owlmail/actions/runs/41457095388)、[Build](https://github.com/soulteary/owlmail/actions/runs/41457095513) 和 [Integration Examples](https://github.com/soulteary/owlmail/actions/runs/41457095793) 均通过。Docker/Linux amd64 与远程 CI 不再属于未执行项。

发布镜像为 `ghcr.io/soulteary/owlmail@sha256:05e730b44a2d614cc50f2f2e18764f4942f58450f6bf5e71486a827127393f95`，发布包含 13 项制品。该 Alpha 保留正常二进制的多人启动门禁，测试 fixture 不作为发布入口；单机默认兼容，Gorge 未改动，继续使用其现有 SMTP 能力，不需要新增适配器。

英文检查单见 [Multiuser release readiness](../en/Multiuser-Release-Readiness.md)。

[0.14.0 的发布范围](Release-0.14.0.md)是保留多人门禁的单机运行。制品发布时间与核验结果另按发布工作流记录，不意味着以下真实多人验收已完成。原始 Phorge PHP 适配器与 Gorge 的隔离组件测试已补齐，但不是完整通知部署或真实 Stargate 身份契约验收。

### 上游邮箱验证声明实现进展

Stargate [PR #229](https://github.com/soulteary/stargate/pull/229) 已合并，源码提交为 `67d1e1c8dc458433a2cb8df41e4ce4d9081ae547`。实现将成功的 Herald 邮箱通道 challenge 与明确用户 ID、精确邮箱和会话证据绑定，旧会话、短信、TOTP、密码或通用 AMR 不自动获得已验证声明。证据不随滑动访问延长，目录刷新观察到邮箱变化时清除。上游 CI 的 lint、race/覆盖率、镜像打包和 Windows 启动检查通过；PR 工作流不包含漏洞扫描，不能额外声称无漏洞。

本轮没有真实隔离身份源及通知部署环境，用户确认继续实现并保留验收。此进展仅补齐上游源码实现，不关闭 T02/T16/T17，不解除正常二进制多人门禁。详细契约见[身份契约](../en/Stargate-Identity-Contract.md)。后文历史记录中“未提供声明”描述当时固定版本。

### 正式多人版本剩余验收

- **T02：真实身份契约。** 在真实 Stargate 部署确认稳定 subject/namespace、代理剥头和可信邮箱验证声明，验证伪造声明拒绝、未验证邮箱不能认领；合法稳定 subject 缺少已验证邮箱时仍只建立空个人箱。已验证当前邮箱不等于历史归属：邮箱回收或身份重分配必须有明确策略，无连续归属保证时采用 `explicit-history`，不自动公开旧历史。合成声明通过不能替代此项。
- **T16：真实通知闭环。** 用真实 Phorge 两个身份和 Gorge 现有 SMTP 路径，验证先投递后登录认领、跨身份隔离、系统账号不能读信、TLS/域限制和不可用时不意外投向真实邮箱；保存脱敏配置与结果。
- **T17：正常二进制验收与门禁决策。** T02/T16 通过后，用候选版本的正常发布二进制执行实际启动、配置拒绝、认领、隔离、轮询、重启恢复和单机兼容回归；关闭阻断项后，单独评审是否解除门禁。当前 Alpha 的成功发布不等于此项完成。

Beta/RC 是可选发布节奏，不是必须追加的阶段。本期不扩展团队管理、共享空间、生产邮件外发，也不把固定压测天数、NFS 生产部署或双宿主实验作为个人收件箱正式版的统一前置条件；仅在实际部署使用对应能力时验收其适用边界。

## 历史实施记录

以下按实施顺序保留当时的结果、失败与未执行状态。后文“未推送/未执行远程 CI”等表述描述该轮时间点，以本页顶部当前状态为准；旧失败证据不因后续通过而删除。

## 本轮并行实施

三个 agent 分别负责 SMTP、归并与接口隔离，主 agent 集成启动和资源生命周期，并增加跨模块真实网络测试。归并 agent 另行只读审查接口隔离，修复发现的问题。

### SMTP 数据通路：T05 / T06

- `internal/mailserver/auth.go`、`session.go`、新增 `routed_ingress.go`：系统账号认证、TLS、允许域、RCPT 动态待认领建箱。
- 控制库新增 `WithExpectedRouteSnapshot`：保存 RCPT 决策，在 DATA 提交前复核。仅允许具有绑定审计的合法认领迁移；阻止地址被重新分配后静默投给新身份。
- 多收件人按最终收件箱去重，分别生成独立邮件和附件；不修改原始 MIME，也不把 MIME 收件人作为权限依据。
- 隐藏暂存 → prepared journal → 全部文件安装和同步 → COMMIT → 同一内存锁发布。
- 提交前失败回滚全部副本；清理失败进入阻断状态。恢复先处理接收 journal，再加载 EML。
- 已提交 journal 仅清理，不重建后来已删除的邮件。
- 不经过全局邮件事件/外发链路；本期多人明确不支持 S3、Relay、Webhook。

### 归并与恢复：T09

- 新增 `internal/mailbox/merge.go`、`merge_route_snapshot.go` 和 `internal/mailserver/ownership_merge.go`。
- `PersonalMailbox` / `WithPersonalMailbox` 验证个人身份与归属图，不能选取 pending 或冻结源。
- 先将未来路由切换到个人箱，再在持久 merge journal 下迁移历史归属。
- 每封邮件原子替换 sidecar；中途失败保留 journal，重启后幂等继续。控制库完成状态仅在文件迁移成功后提交。
- 迁移不生成新邮件事件，不重新发送历史邮件。
- 待认领创建额度只统计仍有 active pending 路由的箱，冻结归并源及显式历史源不占用此额度。

### 接口隔离：T10 / T11 / T13

- 新增 `internal/mailserver/scoped.go` 与 `internal/api/api_multiuser.go`。
- 多人使用独立路由白名单，标准 `NewAPI` 误配隔离存储时全局拒绝，避免复用旧 handler 漏过滤。
- 个人列表、预览、统计、详情、HTML、源码、附件、ZIP、已读、删除、批量、run_id 清理、检查、cursor 与等待按箱隔离。
- 批量先验证全部 ID；其他用户 ID 和不存在 ID 一致返回 404。
- 源码与附件通过 source lease 保护，流式响应关闭时释放。
- cursor 使用箱绑定的不透明认证加密，不能跨箱复用；等待每箱最多 8、全局最多 64，长轮询不持有控制锁。
- `/email`、`/config`、reload、旧兼容路由、MCP、WebSocket、metrics 等不加入多人白名单。
- Relay/Webhook 不提供提交和重试能力；查询只返回空的 disabled 能力结果，不暴露全局数据。
- 探针仅返回聚合状态。响应在可信身份确认后立即给出不透明身份标记，即使后续数据库失败也能让浏览器识别账号变化。

### 前端：T14

- 多人模式不建立 WebSocket；采用可关闭的 15 秒局部轮询。
- 后台暂停、重新可见刷新、请求不重叠、失败退避。
- 401 和身份变化清空旧账号邮件状态；不支持的投递控件隐藏。
- 单机继续使用原实时逻辑。

### 主线集成

- `cmd/owlmail/multiuser.go`、`main.go`：加载系统账号与控制库，挂接 routed ingress 和多人 API。
- 有内容的邮件目录不得自动重建丢失的控制数据库；控制库要求放在邮件目录外。
- 启动恢复顺序固定为：接收 journal → EML → pending 清理 journal → merge journal → 首轮维护 → 接口开放。
- 多人模式不初始化全局 Relay 管理器，不要求其 readiness；单机保持原行为。
- 多人关闭顺序先停止 HTTP consumer，再关闭 authority，保留单机历史资源顺序。
- 所有组件集成均位于仍关闭的公开 CLI 发布门禁之下。

## 验证与审查发现

增加跨模块 `TestMultiuserSMTPToHTTPClaimAndIsolation`：使用真实本地 TLS SMTP 和 HTTP listener，先投 Alice/Bob，再证明未验证声明不认领、可信声明认领、跨用户 ID 拒绝、个人统计及清空不影响另一用户，并验证正常关闭。

独立审查及闭环测试发现并修复：

1. 零 Limit 的空页语义导致个人统计和清空不工作：改为显式个人快照。
2. 切换身份后数据库 503 缺少身份标记，可能保留旧界面：标记提前到可信身份校验之后。
3. 全局 Relay 初始化造成 routed SMTP 拒收：隔离模式不初始化全局外发链路。
4. RCPT 到 DATA 之间地址重分配可能错投：使用带审计校验的期待路由快照。
5. 单个收件箱可能耗尽全局等待槽：补每箱 8 槽限制。

验证记录随交付提供；最终以对应日志为准。

## 本轮补齐：持久配额、保留清理和迁移

继续由三个 agent 分工实施，主 agent 集成配置、生命周期和故障路径。

### T07 配额与待认领清理：代码及组件验收完成

- AUTH/RCPT 尝试额度写入控制库，固定认证 global key，非减分钟水位防止重启或时钟回拨绕过。
- 每箱消息数/逻辑文件字节和全实例总量在同一控制/存储锁内，依据权威已提交文件加整组副本统一预检。无独立收费账本，回滚、删除、恢复不会产生虚计费。
- 实例总量不足只拒绝新投递，不驱逐其他用户。归并超额延后，源保持冻结，个人箱仍能进入并删除旧邮件腾空间。
- schema v1→v2 事务升级统一加入配额表、保留 journal/audit、退休标记；v1/v2 只读打开不修改源。
- 按持久 receivedAt 清理 pending；个人箱和归并源不进入候选。显式历史策略遗留源使用同一期限。
- 逐箱规划持久清理意图，使用删除 fence，失败只冻结相关源并可重启继续；不发删除/新邮件事件或外部投递。
- 旧空箱退休后普通 pending 地址可重新投递建箱，blocked 拒绝规则保留。
- 启动恢复、首轮清理、周期维护与关闭取消均已接入；周期同时重试因容量延期的归并。
- 多人跳过旧全局驱逐策略，禁止旧 mail-retention-days 自动删除个人数据；单机保留原有策略。

### T15 离线迁移与只读诊断：主要功能及组件验收完成

- 新增 `migrate-mailbox`，必须 `-offline` 和显式方向/身份或个人箱 ID；向新 bundle 暂存、核验后原子发布，源保持不变。
- 正向生成 mail/、control.sqlite、完整 source-archive/ 和 SHA256 manifest；反向只导出一个个人箱，不复制其他用户或全局控制库。
- 重跑验证目标全部文件；篡改、源并发变化、symlink、嵌套路径、未处理队列/远端附件/不安全全局历史均拒绝。
- 选定箱参与未完成/配额延期归并时拒绝半套历史导出，不阻塞无关箱。
- 失败清理暂存，原部署可继续指向未修改源，完成往返、幂等及故障回滚测试。
- doctor 只读报告控制库完整性和聚合状态，不做清理/迁移；缺失库不会创建，旧 schema 不被只读诊断升级。
- 修复旧附件迁移 sidecar helper 保留 Ownership，并拒绝隔离目录执行本期不支持的外部附件迁移。

具体配置、命令和回滚步骤见 [多人运维指南](Multiuser-Operations.md)。

## 历史阶段：当时仍未开放或未执行

- T02：真实 Stargate 已验证邮箱声明尚未提供。
- T12：多人 Relay/Webhook/S3 仍明确禁用，单机原功能保留。
- T16：尚未运行真实 Phorge/Gorge 与真实 Stargate 部署联调。
- T17：本轮已完成模式化 OpenAPI、真实 Chrome 和 rootless Podman 容器验证；真实后台暂停和恢复已在 macOS Chrome 与 Linux/Xvfb 严格验证，见验证报告。当时 Docker daemon/amd64 实跑、远程 CI 和真实部署备份恢复演练尚未执行；远程 CI 和 Docker 后续已通过，见顶部当前状态。真实部署验收仅按实际部署范围执行。
- `-offline` 是调用者已停写声明及额外一致性检查，不是对旧二进制的自动排他锁。

多人运行门禁保持关闭，不因离线工具可用而开启；Gorge 未修改。

## 前轮最终检查

- `go test -race ./...`：通过，包含真实 TLS SMTP→HTTP 认领与隔离闭环。
- `golangci-lint run ./...`：0 issues。
- `go vet ./...`：通过。
- `go build -o ../owlmail-multiuser-check ./cmd/owlmail`：通过。
- `bun test ./tests/web ./tests/docs`：217 pass，0 fail。
- `git diff --check`：通过。
- 公开 CLI 的 multiuser 门禁与 stdio MCP 禁用均有测试覆盖。

## 本轮验证范围

覆盖：跨重启/回拨额度、并发额度、跨箱整组拒绝、删除释放、容量归并延期、清理恢复与认领互斥、伪造 MIME Date 不影响保留、启动与周期清理、旧 schema 只读/升级、双向迁移完整性/幂等/源不变，以及默认单机回归。最终执行结果随补丁提供。

本轮结果：完整 `go test -race ./...` 通过；最后补充只读诊断和导出边界后，`go test -race ./cmd/owlmail ./internal/mailbox ./internal/mailserver` 再次通过（18.198s / 9.743s / 51.998s）。全量 lint 为 0 issues，go vet 通过，最终构建通过，Web/文档 217 pass / 0 fail，diff 检查通过。

## 本轮补齐：模式契约、真实浏览器与容器验证

- OpenAPI 独立多人 JSON/YAML，25 paths / 27 operations；真实路由白名单与实际响应 schema、生成重现性、自定义头/base、认证测试通过；旧单机契约文件未改。
- 实际 Chrome：单机 6 pass，多人业务/自然轮询 2 pass；真实后台专项已修正 Playwright 焦点模拟影响，macOS 与 Linux/Xvfb 严格单项各 1 pass、无 skip。结果见 [专项报告](Multiuser-Validation.md)。
- 实际 rootless Podman Linux/arm64：正式 Dockerfile 构建、非 root/私有文件、SMTP附件/HTTP、持久化重启、healthcheck、双向离线迁移、只读 secrets 和正式模式门禁通过。独立 test-only fixture 的 TLS/认领/隔离/模式契约/重启也通过。
- 修复 `.dockerignore` 误排除嵌入 HTML，增加模块 verify；排除浏览器依赖/报告进入生产 context。
- 测试 fixture 为独立显式 build-tag 入口，仅 loopback 合成身份，不进入正式二进制；PR CI 构建/运行测试镜像但不推送，生产发布路径不变。
- 浏览器 job 已加入多人套件及截图产物；Docker verify 已加入生产门禁与组件脚本。未推送/触发远程 CI。
- 公开多人发布门禁仍关闭；真实 Stargate 已验证邮箱声明尚未提供，Gorge 未修改。

详细证据边界和复跑说明见 [专项报告](Multiuser-Validation.md)。

前轮全量 race、生产/tagged fixture lint（均0 issues）、vet、正式构建及219项Web/文档检查通过；rootless Podman production和fixture分别PASS。原 Linux/Xvfb TIMEOUT60s 已定位为 xvfb-run 的 PID 1 信号握手，并修正测试启动流程。

本轮继续完成：严格真实后台用例（macOS 1 pass、Linux/Xvfb 1 pass），后台 16 秒请求不增长且 JS 心跳继续，前台立即获得后台 SMTP 新邮件、保持详情、单一在途刷新；单机 6 pass、多人业务 2 pass、模式/正式入口 Go 回归通过。CI 将 headless 业务与 strict Xvfb 后台分开执行并隔离证据目录。正式多人发布门禁未解除，Gorge 未修改，未推送或执行远程 CI。

真实超时负向检查补齐：修复 Playwright detached fixture 脱离进程组后的存活残留；清理前捕获后代并核对 PID starttime。实际 30 秒超时返回124，诊断容器继续存活时核验无 live fixture/Chromium/Xvfb，Z 状态退出进程单独说明。

## 本轮补齐：一致备份与真实进程崩溃恢复

- 新增 `tests/recovery/backup_test.go` / `backup_snapshot_test.go`：停写完整状态备份，manifest 核验、新目录恢复、待处理归并、隔离、附件/已读/receivedAt、持久额度、缺库拒绝、篡改拒绝及不可变备份回滚。
- 新增 `internal/mailserver/crash_recovery_test.go`：五个真实父进程强杀边界，覆盖 SMTP 部分安装/提交前/COMMIT 后、归并 sidecar 已换一封、pending 清理已删一封。逐例备份完整崩溃现场后在副本恢复，核对原件和备份不变、恢复幂等、跨箱拒绝与删除不复活。
- macOS 专项 race 与 Linux/arm64 非 root 容器实跑通过；Linux 为 CGO=0 测试二进制，无 race。没有为测试修改生产代码、重建生产镜像或增加发布门禁旁路，Gorge 未修改。
- 修正文档中 COMMIT 恢复语义和清理/归并启动顺序。现有 Go CI 自动包含新增测试；未推送或触发远程 CI。
- 这不是断电/硬件故障、在线快照或真实 Stargate 部署恢复验收；正式多人门禁继续保留。详细边界、命令和结果见 [恢复演练](Multiuser-Recovery.md)。

## 本轮补齐：真实命名卷、权限恢复和 SQLite 重建

- 新增 `tests/container/recovery.py`：停写后 source→私有 backup→新 restored 命名卷，比较内容及 UID/GID/mode，保留邮件 UUID，验证认领、附件和隔离；错误 root 属主下非 root 读写实际拒绝，再由归档恢复权限。最终非空凭据及配置已核对原始哈希，早期空样本证据不计通过。
- 新增 `tests/recovery/sqlite_index_test.go`：两轮只删除派生索引及 WAL/SHM 后重建，直接核验 SQL 行与 ready 状态，权威控制库不删不重建；验证 ID/已读/receivedAt/附件、绑定、配额和跨箱拒绝。Scoped 查询的内存授权边界保持不变。
- rootless Podman Linux/arm64 命名卷实跑通过；本轮测试容器和卷已清理。数值权限是容器命名空间内的 UID/GID，不扩展到宿主映射、ACL/xattr/SELinux 或真实 TLS 连续性结论。
- Docker PR verify 增加 test-only Go binary 和完整演练及脱敏证据 artifact，正式镜像发布路径未变；未推送/执行远程 CI，多人门禁保留，Gorge 未改。

详细命令、验证层级和限制见 [恢复演练](Multiuser-Recovery.md)。

## 本轮补齐：实际 TLS 连续性与不同用户映射恢复

- 独立 tagged fixture 新增 state 内持久 TLS 配置及真实 peer 探针；`tests/container/tls_recovery.py` 实际 STARTTLS/CA/IP SAN/AUTH 验证恢复前后 DER/SPKI 相同，投递与隔离继续有效。错密钥、缺证书、错 CA、错主机名拒绝均有实际记录。生产运行代码和多人门禁未改。
- `tests/container/uid_mapping.py` 实测同 Linux VM 两个不相交的真实 user namespace，核对 `/proc` 映射与 VM inode 101000→166536，错误物理属主复制的独立读写失败，目标数值归档恢复后非 root 服务、邮件/附件/隔离正常，源未变。
- 用户明确选择先完成不同映射环境，独立双宿主实跑继续保留；不将同 host userns 称为跨宿主完成。脚本不改系统用户或 subuid/global config，只清理自身资源。
- Docker PR verify 新增 TLS 恢复步骤并仅保存非私密证据；映射脚本依赖已有 Podman machine 能力，不伪称通用 Docker/CI 验收。未推送/执行远程 CI，Gorge 未改，公开多人门禁保留。

运行说明及验收边界见 [恢复演练](Multiuser-Recovery.md)。

## 本轮补齐：ACL/xattr 与 SELinux 权限恢复

- 独立 metadata helper 镜像提供GNU tar/ACL/attr工具，生产镜像不改。真实命名卷中完整恢复POSIX access ACL和非空user.*属性；普通tar即使内容与stat一致，丢失扩展元数据也被检测。实际UID1001/1002和ACL mask允许、拒绝、修复均验证，源及归档未变。
- 现有VM实际SELinux Enforcing，旧MCS在不同目标process context下读写被拒，目标:Z重标记后非root服务、邮件/附件及隔离恢复；前后根/文件DAC和控制库hash相同。归档排除security.selinux、模式0600，不复制旧MCS作为恢复方案。
- 已观察实际enforcing和标签差分，未取得匹配AVC，审计日志验证仍保留；不扩展成跨host策略移植、NFSv4/default ACL或任意security/trusted xattr结论。
- 两项演练资源已清理。Docker PR verify加入ACL/xattr工具与演练；SELinux脚本明确需要已有enforcing Podman machine，unsupported非0不算pass。未推送或执行远程CI，生产模式门禁、单机和Gorge保持不变。

详见 [恢复演练](Multiuser-Recovery.md) 的当前支持范围与复跑命令。

## 本轮补齐：default ACL 与严格 NFSv4/AVC 探测

- POSIX default ACL实跑通过：非追溯继承、umask077下新文件/目录/孙项、真实0600/0700创建mode、named-user/group I/O、恢复后新项继续继承、普通tar丢失负向。只增显式tag测试创建helper，生产运行代码不改。
- NFSv4能力探测真实记录unsupported：当前无实际NFSv4挂载且缺ACL工具，XFS目标负向也拒绝。只读probe不称完整NFSv4恢复实现，不把POSIX ACL替代NFSv4。
- MCS与TE各重现本轮唯一对象的真实拒绝并恢复，但四种审计通道未提供可关联AVC，两轮严格unsupported/退出2；保留审计链验收，不修改全局audit/SELinux配置。
- Docker PR verify增加default ACL helper与演练；NFSv4和AVC专用能力探针不伪装通用CI通过。未推送/执行远程CI，Gorge及多人门禁不变。

详细已验证/未验证范围见 [恢复演练](Multiuser-Recovery.md)。

## 后续隔离实验室与 AVC 排障

- AVC已从此前unsupported推进到真实通过：内部state确认既有auditd因空间保护一直suspended，按本轮授权仅一次官方resume恢复；配置、规则、Enforcing和daemon身份不变。新MCS/TE事件从实际日志严格关联，正常轮转经独立复核，旧失败报告保留。
- NFSv4原生测试栈采用LizardFS RichACL和Ganesha FSAL_LIZARDFS，工具/原生镜像已实际构建；为ARM64固定源码兼容问题保留最小测试补丁。协议和新卷恢复仍以实际客户端结果判定，不把构建成功记为验收。
- 宿主现有exports/fstab/用户/subuid和SELinux策略未改；应用运行代码、Gorge及多人发布门禁不变。具体实验室验收结果随最后运行证据更新。

## 本轮最终验收：原生 NFSv4 ACL 与真实 AVC

- 隔离 LizardFS RichACL/Ganesha 实验室真实通过（run `c72c26a6c6d8`）：9 对象快照、137 长名分页文件、有序 ACE、数字身份/零 capabilities 的实际读写、固定内容和直接偏移读取、原始 ACL/子孙继承及恢复后新项继承。完整后端停机备份恢复到独立新卷/新端口，源与归档未变，所有本轮临时资源已清理。
- 真实 AVC 已通过：auditd 空间保护挂起经一次官方 resume 恢复，新 MCS/TE 事件严格关联；旧 unsupported、轮转末尾断言失败与只读补核报告分别保留。没有修改审计配置/规则或 SELinux 策略/Enforcing。
- 保留第三次恢复初始化失败的具体原因未复现这一限制；加强真实后端/导出就绪检查不冒称已定位该次底层原因。保留旧 FSAL 停止计数警告，专用测试修补不构成生产认证。双宿主、Kerberos、会话 reclaim 和 OwlMail SQLite 的 NFS 部署不在本次通过范围。
- 单机/门禁专项 Go race 已通过，Web/文档 219 项通过；最终源文件语法、文档链接、补丁应用检查见交付记录。生产运行 Go 代码及 Gorge 本轮没有新增改动，多人发布门禁保持关闭。

复跑方式、实际环境权限与边界见[隔离实验室](Multiuser-NFS4-Lab.md)和[恢复演练](Multiuser-Recovery.md)。
