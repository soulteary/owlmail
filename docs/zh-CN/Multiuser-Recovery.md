# 多人模式备份、恢复与崩溃演练

状态：组件验收；公开多人启动门禁继续关闭。本文的测试使用合成身份、隔离目录和本地凭据，不代表真实 Stargate 或 Phorge/Gorge 部署已经验收。Gorge 无需代码修改。

关联：[实施清单 T15/T17](Multiuser-Implementation-Plan.md)、[运维指南](Multiuser-Operations.md)、[当前进度](Multiuser-Progress.md)。

## 备份边界

首版采用停机一致备份。先停止接收和所有其他写者，取消并等待维护 worker，关闭邮件服务和控制数据库，再复制完整状态。不能在服务运行时逐文件复制并声称得到一致备份；`-offline` 也不能代替停写。

必须作为同一恢复点保存：

- 整个 mail-directory，包括 EML、归属/已读/接收时间元数据、附件、模式标记、删除 fence、接收暂存目录与未完成 journal。不要用仅匹配 `*.eml` 的命令备份。
- control-database 及仍存在的 SQLite `-wal` / `-shm` / journal 配套文件；控制库不能根据 MIME 地址或首次登录者重建。
- 对应的应用配置、系统 SMTP 账号散列、TLS 证书/密钥，以及身份命名空间、可信代理与 Header 配置。外部保管的密钥需另有可用的恢复来源。
- 对应程序版本、目录布局、逐文件 SHA256/大小/权限和快照时间。镜像本身不包含持久数据；数据卷也不等于已备份。

控制库必须位于 mail-directory 外。建议将上述资源放在一个运维管理的状态根目录中，备份输出位于另一个目录或存储；外部挂载和符号链接指向的内容不能假定会随普通归档自动包含。备份和证据应分开：完整邮件、身份、账号散列及私钥不能提交到仓库或公开 CI artifacts。

正常关闭可能收拢 SQLite WAL；异常停止时不能手工删除 WAL/SHM 或应用 journal。复制前确认没有活跃写者，保留同一个时点的文件集合。备份校验副本应为只读，任何恢复都在另一份工作副本执行。

## 恢复与回滚顺序

1. 保持投递入口及访问入口关闭。验证归档或快照文件清单、SHA256、大小及权限；缺失、篡改或权限异常先拒绝恢复，不能先启动再检查。
2. 解包到新的空目录，恢复运行 UID/GID 和私有文件权限。配置明确指向这份工作副本的邮件、控制库和凭据，不能沿用指向原实例的绝对路径。
3. 只读诊断控制库及归属图。控制库缺失、损坏或不受支持的 schema 不能自动初始化；不要只恢复 mail-directory 后直接启用服务。
4. 恢复按既定顺序执行：接收 journal → 邮件加载 → pending 清理 journal → 归并 journal → 首轮维护。恢复完成前不得开放多人接口或 readiness。
5. 验证身份到个人箱的稳定绑定、路由、待认领隔离、邮件 ID/内容/附件、已读和接收时间、持久配额及待恢复任务。再次关闭并重开，确认恢复幂等，无重复、丢失或跨箱授权。
6. 验收失败时停用并保留失败工作副本诊断，从同一不可变备份新建另一工作副本，或恢复完整旧部署。不能只换旧二进制打开新格式目录，也不能将其他用户的箱合并为单机。

当前公开 CLI 仍拒绝启动多人服务。上述顺序通过测试组件验证，不提供生产门禁旁路；真实部署恢复演练必须在上游可信声明与发布条件满足后另行验收。离线 `doctor` 与显式单箱迁移保留原有约束。

## 验证方式与证据边界

停机备份恢复测试与真实进程崩溃测试分别记录。前者可准备明确的未完成 journal 状态来验证快照包含性；这不等同于在那个边界发生过真实崩溃。后者必须由独立测试进程执行持久写入，在命中明确边界后被父进程强制终止，不能用返回错误和正常 Close 替代。

进程强杀验证的是进程级中断及既有 fsync/SQLite 事务协议，不是断电、磁盘丢写、损坏控制器或跨文件系统在线快照验证。真实 Stargate 声明、Phorge/Gorge、Docker daemon/amd64、异机实际卷和远程备份介质恢复仍需独立记录。

## 可重复的组件演练

在仓库根目录执行：

```sh
go test -race -count=1 -v ./tests/recovery
```

`TestStoppedBackupRestoreWithPendingClaim` 使用真实 STARTTLS SMTP 和专用测试 CA 校验证书；为两个箱投递带附件邮件，产生待处理认领归并，再停止 listener、邮件服务和控制库。全状态复制包含账号散列、合成证书/私钥和部署描述；旁置 manifest 核对每个普通文件的 SHA256、大小和 mode。恢复到新目录后验证归并幂等、不重放投递、两用户隔离、附件字节、已读与 receivedAt，以及持久尝试额度和每箱容量拒绝。

测试还验证篡改或丢失私钥在启动前被 manifest 拒绝，控制库缺失时不能重建，并从未修改的备份重新复制回滚目录。该复制 helper 仅存在于 `_test.go`，不是应用备份命令：目录统一使用 0700，不宣称保留 ACL、扩展属性或 UID/GID；实际运维仍须按部署要求恢复这些属性。此测试准备的 pending merge 是有效未完成任务，不冒充真实进程崩溃。

测试使用默认内存索引从邮件重载；不据此宣称已演练可选 SQLite 索引删库重建。完整备份不得省略实际部署中的索引/配置文件，只读诊断及恢复核验之后才可决定是否重建可派生索引。

## 真实进程强杀场景

```sh
go test -race -count=1 -v ./internal/mailserver -run '^TestProcessCrash'
```

新增测试仅位于 `internal/mailserver/crash_recovery_test.go`，没有生产命令、环境开关或公开模式门禁旁路。父进程启动专用测试子进程；子进程到达持久边界后原子发布已 fsync 的 checkpoint 并阻塞。父进程校验 PID、磁盘元数据和 journal 后调用 `Process.Kill`（Unix 下为 SIGKILL），等待退出，并确认正常清理 defer 的标记不存在。父子双方均设时限，不能将 checkpoint 半写或正常 Close 误认为崩溃。

五个边界及验收：

- `after-install`：两个目标中第一个副本已安装，未写 COMMIT。恢复后整组为 0，重复重开仍为 0，临时 EML 与 journal 清除。
- `before-commit`：全部副本已安装，尚无 COMMIT。恢复同样回滚整组，不能部分入箱。
- `after-commit`：COMMIT 已持久化，DATA 调用尚未返回。恢复保留两个副本，核对原始邮件与附件哈希、箱归属及 ScopedView 跨箱拒绝。删除一个副本后重放该次真实旧 COMMIT journal，不能复活或复制邮件。此项不代表 SMTP 客户端已收到 250，也不承诺重试 exactly-once。
- `merge-sidecar-persisted`：第一封源邮件归属 sidecar 已更换，控制库 merge journal 尚为 pending。恢复迁完其余邮件，重复恢复与重新打开结果一致；另一用户个人箱不受影响，不产生新投递事件。
- `retention-message-deleted`：第一封到期 pending 邮件已完成持久删除，控制库清理 journal 尚为 pending。恢复前认领被阻止，恢复删除剩余到期邮件，个人箱保留；旧箱退休后同地址投递建立新 pending 箱，不重新激活旧箱。

每个场景均先复制完整已停写的崩溃目录为备份，再从备份生成另一份恢复副本；核验文件指纹一致、原崩溃现场及备份在恢复后未改。测试没有手工拼装 COMMIT 来冒充本轮强杀；删除后的 journal 重放使用该次真实崩溃留下的记录。

## 本轮结果与 CI

- macOS：停机备份测试 `-race` 通过；五个真实强杀边界 `-race` 通过。
- Linux/arm64：在现有生产镜像中以非 root `owlmail` 运行独立 Go 测试二进制，停机备份与五个真实强杀边界全部通过。测试二进制由 `CGO_ENABLED=0` 交叉构建；Linux 结果不包含 race，镜像没有为测试重建，也没有发布端口或解除模式门禁。
- 新测试为普通 Go 测试，现有全量 Go CI 自动包含；专用 helper 在普通测试运行时不启用。未推送或执行远程 CI。

运行日志及源码指纹随本轮交付保存。测试目录与合成密钥由测试清理，不将完整邮件快照或凭据上传为公开证据。

本轮最终仓库检查：`go test -race ./...` 通过（包括新增崩溃与备份包），`golangci-lint run ./...` 为 0 issues，219 项 Web/文档检查和 `git diff --check` 通过。单机/公开门禁专项回归亦通过。没有为当前演练修改生产运行代码。

## 实际容器命名卷、权限恢复与 SQLite 索引重建

后续补齐了 `tests/container/recovery.py` 与 `tests/recovery/sqlite_index_test.go`。前面的默认内存索引组件演练保留其原有范围；本节单独验证真实命名卷和可选 SQLite 派生索引。

脚本创建独立的 source、backup、restored 命名卷。测试 fixture 在 source 卷通过真实 TLS SMTP 接收 Alice/Bob 邮件，记录源 UUID，然后停止 writer。只读挂载源卷，将整个卷归档到 backup 卷；后续只读使用 backup。以容器命名空间 root 解包到新卷，逐项比较文件 SHA256、大小、数值 UID/GID 和 mode，包括目录。恢复后以 UID/GID 1000:1000 运行 fixture，核对原 UUID、认领/归并、附件和跨箱拒绝。

权限负向检查将 restored 卷改为 root:root 且根目录 0700，实际以 UID 1000 尝试读取控制库、打开密钥写入和创建文件，必须被拒绝。再从原归档恢复数值属主和权限，重复 manifest 比较，并确认同一非 root 身份可读写和启动组件。该演练没有使用 chmod 777。备份归档留在本轮私有卷内，不输出密钥或邮件快照；最后只清理本轮标签匹配的容器与卷。

凭据样本由本机 OpenSSL 临时生成，使用显式连接的容器标准输入写入；写后先检查非空和原始字节 SHA256，配置还需 JSON 解析及字段、字节哈希一致。早期复核曾发现缺少 interactive 导致样本为空，该轮证据已作废，不计凭据恢复通过。最终记录使用修复后的完整重跑。fixture 自身 TLS 仍为临时生成，本节证明独立 operator 凭据归档往返，不声称正在使用的 TLS 身份连续性。

SQLite 专项在独立命名卷内以非 root 测试二进制运行 `TestMultiuserSQLiteIndexDeleteAndRebuild`，完成两轮：

1. 使用真实 `NewSQLiteMailboxIndex`、控制库和 routed SMTP，为两个个人箱投递带附件邮件，并保存已读、receivedAt、额度与绑定。
2. 停止邮件服务并关闭索引/控制库，仅移除 `index.db`、`index.db-wal`、`index.db-shm`；删除后核验 `control.sqlite` 哈希未变。不能把 authority 当作派生索引删掉。
3. 重开后要求 index backend 为 sqlite 且 ready；直接只读 SQL 检查 `mailbox_index_v2` 的精确 ID、已读和 receivedAt 秒/纳秒，不能只依据页面结果判定成功。
4. 核验邮件、原文/附件、身份绑定、持久额度和跨箱 Get/批量操作拒绝；再次停机、删索引和重建仍一致。

当前 Scoped 查询会注入归属过滤回调，因此走权威内存数据过滤，而不是直接用全局 SQLite 行授权。上述两层验证分别证明派生索引正确重建和个人箱授权仍有效，不宣称实现了 SQL 租户索引加速。实际运维若需重建，应先停写并保存完整备份，确认 `mail-index-path` 与 `control-database` 的准确路径；只处理已配置的派生索引及其配套文件，保留全部 EML、归属、控制库与恢复 journal。恢复前后均核对权限和权威数据，失败则回滚整个工作副本。

### 复跑命令与平台边界

先按容器验证说明构建生产与独立 tagged fixture 镜像，再按目标容器架构编译测试二进制：

```sh
# Docker amd64 示例；本机 Podman Linux/arm64 实跑使用 GOARCH=arm64。
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o /tmp/owlmail-recovery.test ./tests/recovery
python3 tests/container/recovery.py \
  --image owlmail:container-check --fixture-image owlmail:container-fixture \
  --recovery-binary /tmp/owlmail-recovery.test \
  --evidence-directory /tmp/owlmail-volume-evidence
```

Podman 可增加 `--runtime podman --connection podman-machine-default`。必须传入 recovery binary，不能静默跳过索引验收。证据目录只输出 manifest、哈希、运行环境和测试日志；不输出归档内容。脚本异常返回失败，清理仅限本轮资源。

本机实际结果为 rootless Podman、Linux/arm64、非 root 1000:1000：完整命名卷恢复、错误权限拒绝及修复、两轮 SQLite 重建通过。容器内数值 UID/GID 不是宿主机 UID/GID；不据此宣称跨宿主用户映射、ACL、xattr、SELinux、外部存储或 Docker daemon/amd64 已验收。Docker PR verify 已加入同样流程，上传日志和元数据，不上传归档；未触发远程 CI。生产多人启动门禁继续保留，Gorge 未修改。

本轮最终检查：全量 Go race、全量 lint（0 issues）通过；新增 SQLite 专项 race 和单机 SQLite 兼容专项通过。最终卷演练 run-id `693edc1c1479`，非空 PEM/JSON、权限及内容恢复和 SQLite 均通过；本轮自有容器和卷清理后均为 0。

## 实际 SMTP TLS 证书恢复连续性

`tests/container/tls_recovery.py` 补齐此前“仅 operator 凭据归档往返”的缺口。独立 tagged fixture 新增成组的 `-tls-cert` / `-tls-key` / `-tls-ca` 测试参数，只接受 state 根内的常规文件，启动前校验证书私钥配对；不改变正式 OwlMail 配置或多人运行门禁。不提供这些参数时，原浏览器 fixture 仍使用其既有临时证书行为，不能作为本节的成功证据。

演练使用真实专用 CA、带 `127.0.0.1` IP SAN 的服务端证书和私钥。客户端在实际 STARTTLS 连接中验证 CA 与地址，并成功执行 SMTP AUTH。`/__fixture__/tls` 仅在本地测试代理中返回实际 TLS peer 的证书 DER 和公钥 SPKI SHA256；未配置验证时不会伪报通过。

source 卷完成投递后停写，全卷归档并恢复到新的卷；恢复后的服务实际加载该证书和密钥。两次真实连接的 peer DER/SPKI 与独立解析证书所得指纹均一致，SMTP 认证和恢复后投递继续成功，Alice/Bob 的原邮件、附件及跨用户 ID 拒绝也通过。只检查文件哈希或解析 PEM 不替代这项实际连接验证。

四个负向场景单独验证：错误私钥、缺失证书在开启 listener 前拒绝启动；错误 CA、错误主机名导致实际握手和投递被拒绝，不增加邮件。原始 source 和备份归档未改，私钥不导出为 artifact。本机使用已有生产镜像和仅 COPY 新 tagged fixture 二进制的测试镜像；没有重建或改造生产镜像以绕过门禁。

复跑需要包含新测试参数的 fixture 镜像（可用现有 `Dockerfile.multiuser-fixture` 正常构建）：

```sh
python3 tests/container/tls_recovery.py \
  --image owlmail:container-check --fixture-image owlmail:container-fixture \
  --evidence-directory /tmp/owlmail-tls-evidence
```

Podman 可传 `--runtime podman --connection podman-machine-default`。Docker PR verify 已增加此步骤，复用只上传日志/元数据的证据目录，不上传证书私钥归档；远程 Docker/amd64 CI 尚未执行。本地 rootless Podman Linux/arm64 实跑通过，成功及四项负向结果另附证据 JSON。

## 不同 UID/GID 映射环境恢复

用户已选择先验证不同映射环境，保留独立双宿主验收项。`tests/container/uid_mapping.py` 针对既有 rootless Podman machine，在同一 Linux VM 中创建两个同时存活、不相交的真实 user namespace；不创建新 VM、系统用户，也不修改 `/etc/subuid`、`/etc/subgid` 或 Podman 全局配置。

本次实际 VM host 映射为 source 的 `0 → 100000 / 65536` 与 target 的 `0 → 165536 / 65536`，UID、GID 范围均不相交。脚本同时读取容器及 VM host 看到的 `/proc` 映射，并通过只读 VM 探针核对真实卷 inode：容器内 1000:1000 在源对应 `101000:101000`，在目标对应 `166536:166536`。不能把同一 `--user 1000` 或一次 chown 当作跨映射验证。

负向步骤在祖先命名空间保留源的物理属主复制子项，验证目标控制库 inode 仍属 `101000:101000`，然后在目标映射内以 1000 分别测试读取和写入，均须拒绝。恢复步骤丢弃错误工作副本，依据目标真实映射仅修复测试卷根入口，再在目标命名空间中解包源的数值属主归档。解包后核对内容哈希、目标物理属主、非 root 实际读写、SMTP 服务、两用户原邮件 UUID、附件和 API 隔离，并确认源内容及数值元数据未变。

该脚本依赖既有 Podman machine、`--userns=auto` 能力、足够的已配置 subordinate ID 范围，以及到相应 VM 的只读 `sudo` `/proc`/`stat` 探针。它不是通用 Docker 或原生 Linux runner，不加入不具备这些条件的 Docker CI，也不会将不支持或跳过计为通过。

```sh
python3 tests/container/uid_mapping.py \
  --connection podman-machine-default --machine podman-machine-default \
  --fixture-image owlmail:container-fixture \
  --output /tmp/owlmail-uid-mapping-evidence.json
```

本机不同真实映射环境实跑通过；独立物理/虚拟宿主之间的复制、各自实际账号/subuid 配置、SELinux、ACL/xattr 和远端存储仍需部署环境另行验证。此项与 TLS 连续性为两个独立演练，不声称已经完成跨映射并同时保留 TLS 的生产部署联调。

本轮最终记录：TLS run `9261f4f3c276` 和不同映射 run `431da639ce6b` 均通过，相关自有容器/卷清理后均为 0。全量 Go race、普通及 tagged fixture lint（0 issues）、默认 fixture 的真实浏览器业务 2 项、Web/文档 219 项及 diff 检查通过。单机 TLS 专项 race 也通过。独立双宿主、真实 Stargate 已验证邮箱声明及正式多人发布验收继续保留。

## ACL、扩展属性与 SELinux：当前可验证范围

本轮使用现有 rootless Podman Linux/arm64 VM 的真实命名卷。应用和生产镜像没有增加备份命令或权限旁路；演练工具及包只在独立测试镜像中使用。此前仅比较字节、UID/GID 和 mode 的普通 tar/复制演练不能证明 ACL 或 xattr 完整，存在这些元数据时必须单独保存并核验。

### POSIX ACL 与 user.* xattr

生产 Alpine 镜像只有 BusyBox tar，未提供 getfacl/setfacl/getfattr/setfattr，也不支持完整 `--acls` / `--xattrs` 归档。新增 `Dockerfile.metadata-recovery` 仅基于生产镜像安装 acl、attr 和 GNU tar，供 `metadata_recovery.py` 演练使用，不发布、不进入正式镜像。

实际场景在源卷邮件文件和目录上设置非平凡 POSIX access ACL：额外用户1001的权限受 mask 限制，另一个用户1002无授权；同时保存目录字符串 `user.*` 属性和邮件文件上的非空二进制属性。脚本解析实际 UID/GID 映射，要求覆盖1000、1001、1002，并在 I/O 探针中核验实际身份。

停写后使用 GNU tar 的 `--numeric-owner --acls --xattrs --xattrs-include='user.*'` 创建私有归档，在新卷中用相同元数据选项恢复。除字节、大小、UID/GID/mode 外，逐项比较数值 ACL 和 xattr 的十六进制值；不能只比较 stat mode。真实授权检查包括：1001可读、mask为r--时不能以写方式打开，mask为rw-时可写打开，mask为---时不能读，再修复mask恢复读取；1002实际访问被拒绝。

负向对照使用 BusyBox 普通 tar：文件字节和数值权限完全相同，但额外 ACL/xattr 丢失，元数据校验必须拒绝。在仅补回目录 traverse ACL 后，1001仍不能读取缺失文件 ACL 的邮件，排除“只是目录进不去”的假通过。源与归档保持不变，完整归档留在本轮私有卷内并清理，不作为 CI artifact 输出。

```sh
docker build --build-arg BASE_IMAGE=owlmail:container-check \
  -f tests/container/Dockerfile.metadata-recovery -t owlmail:metadata-recovery .
python3 tests/container/metadata_recovery.py \
  --image owlmail:container-check --helper-image owlmail:metadata-recovery \
  --fixture-image owlmail:container-fixture \
  --evidence-directory /tmp/owlmail-metadata-evidence
```

Podman 可增加 `--runtime podman --connection podman-machine-default`。工具或文件系统能力不足时返回 unsupported/非零，而不是成功跳过。Docker PR verify 已加入独立工具镜像和相同演练，只上传日志、ACL/xattr元数据和指纹；本次没有执行远程 Docker CI。

范围限于本机卷上的 POSIX access ACL、ACL mask 和 `user.*` 属性，不宣称 default ACL 继承、NFSv4 ACL、SMB/macOS ACL、`trusted.*`、任意 `security.*` 或文件 capabilities 已验收。ACL中的数值身份仍需结合目标映射解释，本轮没有把 ACL 与双宿主用户迁移组合验证。演练授予1001读取权限仅针对合成测试数据，不是 OwlMail 推荐的生产邮件授权策略。

### SELinux Enforcing 与目标策略重标记

`selinux_recovery.py` 首先只读核验 `getenforce=Enforcing`、`/sys/fs/selinux/enforce=1` 及 Podman 的 `selinuxEnabled=true`。当前 VM 实际满足，因而执行了真实拒绝与恢复测试；Disabled/Permissive 或标签能力不足的环境返回 unsupported、退出2，不能当成通过。脚本不会关闭 enforcing、修改系统策略或以 `label=disable` 绕过验证。

源容器通过 `:Z` 使用私有 MCS 标签，完成邮件投递并停写。完整归档权限为0600，所在本轮备份目录0700，创建前使用umask077；归档明确排除 `security.selinux`，不把旧 MCS 原样复制当作恢复方案。测试只在自身目标卷中重现旧源 MCS 标签，并在具有不同实际 MCS 的目标容器里以非 root 分别尝试读取和写入，均被拒绝。

然后通过目标容器的 `:Z` 按目标策略重标记。实际 `/proc` 进程标签与目标文件标签一致后，原文件读取、创建文件、非 root SMTP服务、原邮件 UUID、附件和API隔离恢复。拒绝与成功之间的控制库 SHA256、文件 DAC 和目标根目录 DAC 均完全相同，以区别 MCS 问题和普通文件权限问题。测试前后系统一直 Enforcing，源内容不变；额外 `:z` 检查只验证一个合成文件可在共享标签下访问，不代表支持多人共享邮箱空间。

```sh
python3 tests/container/selinux_recovery.py \
  --connection podman-machine-default --machine podman-machine-default \
  --fixture-image owlmail:container-fixture \
  --output /tmp/owlmail-selinux-evidence.json
```

此脚本针对已有的 Podman machine，要求VM提供只作用于本轮卷的归档、getfattr和chcon能力；不在通用 Docker CI 中假设 SELinux enforcing 或静默跳过。当前内核日志没有暴露匹配本轮进程的 AVC 记录，证据标记 `audit_record_observable=false`：已验证实际 enforcing、MCS与DAC差分及拒绝/恢复，没有声称 AVC 日志链或审计管道完成验收。

该结果限于同一 VM、现有策略下的 `container_file_t`/MCS 和 `:Z`/`:z` 行为，不证明跨宿主 policy 版本、定制类型/模块、持久fcontext规则、策略恢复、SELinux布尔开关或远端文件系统的可移植性。跨环境恢复应由目标策略分配合法标签，不盲搬 `security.selinux`；独立双宿主验收仍保留。

本轮最终记录：ACL/xattr run `5c6268369fe0`、SELinux run `20ef6c487e51` 均通过，自有容器和卷清理后均为0。生产运行代码、单机行为、Gorge及公开多人门禁未改。

本轮仓库检查：单机配置/公开多人门禁专项 Go race、219项 Web/文档检查、两个新脚本 Python 语法及 diff 检查通过；本轮只改测试脚本、独立工具镜像、CI和文档，没有修改Go运行代码，不将此前全量Go检查误记为本轮重跑。

## default ACL 继承、NFSv4 ACL 与 AVC 审计：当前验收状态

本轮三项分开判定：POSIX default ACL 真实恢复通过；NFSv4 ACL 授权/恢复未通过验收；SELinux 实际拒绝仍可重现，但 AVC 审计日志关联未通过。后两项不能用 POSIX ACL、Enforcing 状态或一次 Permission denied 替代。

### default ACL：恢复后新对象继续继承

`default_acl_recovery.py` 使用已有 GNU tar/ACL 测试镜像和真实命名卷，先创建既有文件/目录，再设置目录 default ACL。验证既有对象不被追溯修改，随后在 umask077 下创建的新文件、子目录、孙项正确继承。实际覆盖 named user1001、named group1003及额外进程 UID/GID，并解析映射确认相关数值身份可用。

新增 `acl_creation` 小工具只在显式测试 build tag 下构建，直接通过 `OpenFile(...,0600)` 和 `Mkdir(...,0700)` 创建对象，没有 install/chmod 后置模拟。带 default ACL 的文件可以保留 named 条目rwx，但有效权限仍受创建模式对应的mask限制；0700子目录的access mask禁止外部search，其default ACL仍可传递给owner在其中新建的孙项。测试同时检查ACL原值和实际非所有者I/O，不把条目存在等同于可访问。

停写后 GNU tar `--acls`/`--xattrs` 完整归档，在新卷恢复，再创建新的文件、目录和孙项，要求继承与实际授权仍正确。普通 BusyBox tar 对照保持字节与stat，却丢失default ACL：新项不再继承，实际访问拒绝，不能判定完整备份通过。源与归档不变，自有资源全部清理。

```sh
# 按目标架构选择amd64或arm64；本机实际Linux/arm64。
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -tags owlmail_acl_creation_fixture -o /tmp/owlmail-acl-creation ./tests/container/acl_creation
python3 tests/container/default_acl_recovery.py \
  --image owlmail:container-check --helper-image owlmail:metadata-recovery \
  --creation-binary /tmp/owlmail-acl-creation \
  --evidence-directory /tmp/owlmail-default-acl-evidence
```

Podman增加 `--runtime podman --connection podman-machine-default`。creation binary为必填，不能静默用chmod模拟。Docker PR verify已加入真实创建helper编译与此演练；不发布helper，也不影响正式单机二进制。

### NFSv4 ACL：当前缺少真实目标，未验收

本机只读探测确认：没有已挂载NFSv4目标，主要本地文件系统为XFS/ext4等，缺少 `nfs4_getfacl` / `nfs4_setfacl`。`nfsstat`存在不代表已有可用NFSv4 ACL目标；服务线程探针不可用也不被误写成“确定未安装服务”。本地XFS的POSIX ACL不能证明NFSv4 ACE语义。

新增 `nfs4_acl_probe.py` 仅探测现有机器和可选的已挂载目录，不安装服务器、修改exports、创建挂载或写ACE。默认探测和选定现有XFS目录的负向探测均明确 unsupported、退出2；探测本身异常退出1，不把错误当能力不足。即使未来读到真实NFSv4 ACL，能力可见也不等于完整授权与恢复通过，`acceptancePassed`保持false。

```sh
python3 tests/container/nfs4_acl_probe.py --machine podman-machine-default \
  --output /tmp/owlmail-nfs4-capabilities.json
# 仅检查已有挂载；不创建或挂载远端存储。
python3 tests/container/nfs4_acl_probe.py --machine podman-machine-default \
  --target /existing/nfs4/test-directory --output /tmp/owlmail-nfs4-target.json
```

NFSv4授权与恢复验收仍需：真实支持相应ACL的服务端/客户端挂载、NFSv4 ACL工具、可控映射主体、ACE顺序/allow-deny/继承和真实I/O，以及恢复后的语义核验。不能用文本保存ACE或POSIX setfacl模拟来解除此项。当前交付是严格能力探针，不是未经实跑的完整NFSv4恢复实现。

### SELinux AVC：真实新拒绝已重现，日志链未验收

`selinux_avc.py` 独立创建本轮唯一对象及受控进程，在VM中核对host PID、inode、实际process/object contexts和epoch时间窗口。先在Enforcing下重现MCS拒绝；另用本轮私有文件的既有 `user_home_t` 类型重现TE拒绝，两者独立判定，不能用TE日志冒充MCS日志。恢复文件本地标签后可读，证明没有把普通访问错误当作审计事件。

现有auditd active，审计enabled=1，日志可sudo读，配置write_logs=yes；但本轮audit.log增量、ausearch、kernel journal和dmesg均没有符合PID、唯一对象/inode、上下文与时间窗口的可关联AVC。只读诊断还观察到旧日志没有增长、auditd进程未暴露匹配audit.log的打开fd，服务journal无对应条目。这些是诊断线索，不足以认定具体是dontaudit、日志路由或其他原因。

两轮严格返回unsupported/退出2，`audit_acceptance_passed=false`，不是AVC验收通过。脚本不写audit规则、不禁用dontaudit、不改变Enforcing、不重启auditd或更改策略来制造“通过”；本轮资源已清理。若实际拒绝或恢复断言失败，则记failed，不被unsupported掩盖。

```sh
python3 tests/container/selinux_avc.py --connection podman-machine-default \
  --machine podman-machine-default --fixture-image owlmail:container-fixture \
  --denial-kind mcs --output /tmp/owlmail-avc-mcs.json
python3 tests/container/selinux_avc.py --connection podman-machine-default \
  --machine podman-machine-default --fixture-image owlmail:container-fixture \
  --denial-kind type --output /tmp/owlmail-avc-type.json
```

该探针依赖已有Podman machine及只读审计入口。未来需要在批准的审计配置和可观察日志通道上重跑，取得本轮实际对应AVC后才能解除日志验收项。旧AVC、USER消息、模拟字符串和单纯MAC拒绝都不能替代。

本轮结果：default ACL run `49e2d80efbd8` passed；NFSv4默认及XFS目标为unsupported；AVC MCS run `a5757b2988e6`、TE run `5d755cd914ce`均unsupported。生产Go代码、Gorge、单机模式和多人发布门禁保持不变；独立双宿主验收仍保留。

本轮检查：单机配置/多人发布门禁专项Go race、tagged创建helper lint（0 issues）、219项Web/文档、三个Python脚本语法及diff检查通过。没有重跑或冒称重跑全量应用Go race/lint；生产运行Go代码没有修改。

## 后续排障：恢复 auditd 并取得真实 AVC

此前“未观察到日志”是当时的真实结果，旧 unsupported 证据保留。本轮进一步读取 auditd 内部状态后定位到实际原因：`logging suspended=yes`，文件系统/管理空间保护动作曾执行；当前有空闲空间，但既有 daemon 尚未恢复日志处理。不是因为没开启 Enforcing，也不能再仅凭旧日志推测 dontaudit。

用户已授权排障后，对重新核验 PID/starttime/executable 的既有 auditd 发送一次官方 `SIGUSR2`，尝试恢复日志和插件处理；没有重启 daemon，没有修改配置、审计规则、SELinux 策略或 Enforcing。`SIGCONT`只用于读取内部诊断状态，新生成诊断文件在读取后清理。两种信号语义见 [Linux Audit 官方 auditd 手册](https://github.com/linux-audit/audit-userspace/blob/master/docs/auditd.8)。恢复正常日志处理是本轮明确保留的修复状态，不反向制造原先的故障挂起。

恢复前先落盘内部状态、日志 inode/大小、daemon 身份、配置与规则 SHA256；恢复后同 PID 处于 `logging suspended=no` 且日志 FD 打开。再独立生成新的 MCS、TE 拒绝，实际 AVC 同时关联本轮 host PID、对象/inode、process/object context 和事件时间，且 `permissive=0`。两种场景均严格通过，并恢复本轮对象标签和清理测试资源。

现有8MiB轮转策略在恢复后正常触发。初版runner将“当前日志inode必须不变”设得过严，因此原始执行报告中的末尾断言失败完整保留；随后只读 finalizer 验证原inode出现在 `audit.log.1` 且大小不小于恢复前，并从实际 `audit.log*` 重读两条新事件。没有第二次resume，也没有改写旧失败报告。交付同时记录原实际执行脚本、修正runner、只读finalizer各自SHA，不能把只读补核称作又一次修复执行。

这证明采集时正常轮转保留原inode及新AVC落盘，不保证超出现有 `num_logs` 策略的永久历史保留；未截断日志或调整轮转配置。两场景的规则/配置散列、daemon身份和Enforcing前后均相同。

复跑常规 AVC 验证仍直接使用 `selinux_avc.py`。只有确认现有auditd确已挂起、且操作者明确授权恢复时，才使用显式修复工具；它不是CI常规步骤：

```sh
python3 tests/container/selinux_audit_resume.py \
  --connection podman-machine-default --machine podman-machine-default \
  --fixture-image owlmail:container-fixture \
  --output-directory /tmp/owlmail-new-audit-repair \
  --resume-existing-suspended-auditd
# 对已有修复运行进行只读复核，不再次发送resume：
python3 tests/container/selinux_audit_finalize.py \
  --connection podman-machine-default --machine podman-machine-default \
  --run-directory /tmp/owlmail-new-audit-repair
```

当前机器不应为“再跑修复”重新挂起daemon；健康状态应保留。常规验证与修复报告分开。生产 OwlMail/单机/Gorge及多人发布门禁不受该宿主审计修复改变。

原生 NFSv4 ACL 的隔离设施、构建命令、验收顺序及边界见[实验室说明](Multiuser-NFS4-Lab.md)。真实结果以本轮进度和证据为准，不能用能力探针代替完整验收。
