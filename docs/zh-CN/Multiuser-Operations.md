# 多人模式：配额、待认领清理与离线迁移

状态：保留多人门禁的 0.14.0-alpha.1 已发布，远程 CI 与组件验证已通过。多人服务的公开启动门禁仍关闭；离线迁移工具可用于准备和校验数据，不会启动 SMTP、HTTP 或外发任务。

关联：[技术设计](Multiuser-Design.md)、[实施清单](Multiuser-Implementation-Plan.md)、[当前进度](Multiuser-Progress.md)。

地址重新分配的只读检查、精确计划、阻断及未来路由改绑见 [离线收件路由操作](Recipient-Route-Operations.md)。该流程不转移历史邮件，也不解除多人门禁。

完整停机备份、恢复顺序及进程崩溃验证见 [备份恢复与崩溃演练](Multiuser-Recovery.md)。迁移工具的 source-archive 不能代替包含控制库、应用 journal、配置与凭据的完整部署备份。

## 配额与保留配置

所有新增选项只约束 `multiuser`，默认单机不使用这些策略。使用现有平面 YAML/JSON、环境变量和 CLI 优先级。

```yaml
mode: multiuser
multiuser-max-pending: 1000
multiuser-rcpt-per-minute: 60
multiuser-auth-per-minute: 120
multiuser-max-messages: 10000
multiuser-max-disk-mb: 1024
pending-retention-days: 7
pending-empty-retention-hours: 24
pending-cleanup-interval: 1h
```

其余必需身份、TLS、系统账号和控制库配置见设计。所有额度和期限要求正值，不能以 0 绕过；过大的整数、时长和无效间隔会被拒绝。

- `multiuser-max-pending`：持久控制库中的有效待认领路由所引用箱数量；归并冻结源、已退休箱不占用创建额度。
- `multiuser-rcpt-per-minute`：每个已认证系统账号的 RCPT 尝试额度，包含重复和放弃事务。
- `multiuser-auth-per-minute`：全实例 AUTH 尝试额度；未知用户名不会制造新的配额键。
- `multiuser-max-messages`、`multiuser-max-disk-mb`：每箱已提交邮件数量与逻辑文件字节上限。字节包括 EML、归属 sidecar 和解码附件，不按文件系统已分配 block 计费。

实例总量使用既有 `mail-max-messages` / `mail-max-disk-mb` 参数作为硬准入额度。多人模式下，这两项为 0 时分别选用 100000 封和 10240 MiB 的有限默认上限；正值显式覆盖。所有箱的已提交用量加整组新副本必须同时满足总额与每箱额。超限不驱逐既有邮件，删除后自然释放。多人拒绝非零 `mail-retention-days`，只用 pending 保留策略；单机原参数及清理行为不变。

AUTH/RCPT 额度记录在控制库，重启不重置；持久非递减分钟水位防止时钟回拨重新开放额度。旧窗口按水位清理。

邮件用量不维护容易失配的独立收费账本，而是在控制锁和存储提交锁内，从权威已提交邮件及实际持久文件计算；全部目标通过预检后才提交一组副本。失败、恢复、删除自然反映在下次用量中，不存在只计费未收信或部分目标接受。

归并也先统一检查目标容量。目标满额时保留源和待处理 journal，不让该任务阻断个人箱访问；用户可删除旧邮件，后台维护周期会重试。磁盘/元数据错误仍按持久化失败处理。

## 待认领生命周期

启动顺序为：接收 journal 恢复 → EML 加载 → 清理 journal 恢复 → 归并恢复 → 首轮清理 → 周期维护 → 开放接口。多人发布门禁位于此流程之前，尚未解除。

清理只选择未绑定、未退休、非归并源的 pending 箱，包括 `explicit-history` 策略下仍隔离的历史源。按服务端持久 `receivedAt` 清理，不能使用发信人可修改的 MIME Date。

每箱先写持久清理意图，再删除到期邮件，最后完成控制状态与审计。中途失败保留意图，重启继续。持有同一控制锁，认领和投递不能把正在清理的箱重新激活；其他用户数据不会进入候选集合。

空箱达到空箱期限且确认无剩余邮件后，解除普通 pending 路由，保留用于审计的退休 tombstone；同一地址未来投递可建立新箱。显式 blocked 路由保持拒绝，不会被清理自动解禁。个人箱永不由该策略删除。

周期 worker 随服务关闭先取消并等待，再关闭控制数据库。失败会记录日志；未完成 journal 保持冻结，不能用重启绕过。

## 离线迁移约束

必须停止所有写者并使用 `-offline` 声明。该参数是运维停写确认，不是对旧程序的自动排他锁；工具额外检查源前后文件集合与 SHA256，检测到变化时不发布目标。

迁移始终写入一个新的绝对目标目录。源和目标不能相同或相互包含；拒绝符号链接和特殊文件。目标侧锁防止同时迁入同一路径。进程被强杀后，只有确认迁移已停止才能人工处理遗留锁和暂存目录。

以下状态必须先处理，不能靠强制参数静默丢弃：

- 未排空的 Relay、自动转发、Webhook outbox、接收恢复等记录；工具对非空相关目录保守拒绝，可能包括需要先归档的终态记录。
- 未解决的删除/回滚/运行时标记。
- 远端附件；应在受支持的单机流程中先完成本地化。
- 单箱导出时无法安全归属的全局投递历史；必须先单独归档。
- 源控制库存在 WAL/SHM/journal 等未收拢状态。

迁移不会重发邮件或历史任务。本期隔离目录不支持旧的外部附件迁移命令，避免绕过多人外部存储门禁。

## 单机迁入多人

显式指定已确认的稳定身份，不能使用显示名、临时邮箱值或“首个登录者”。

```bash
./owlmail migrate-mailbox \
  -direction standalone-to-multiuser \
  -source /srv/owlmail/mail \
  -destination /srv/owlmail/migrations/import-001 \
  -identity-namespace main \
  -subject stable-user-id \
  -offline
```

输出 bundle：

```text
import-001/
  mail/                     带归属的邮件及附件
  control.sqlite            显式身份与个人箱
  source-archive/           完整源目录原件和历史备份
  migration-manifest.json   数量、身份、源指纹及逐文件 SHA256
```

原 EML、已读、接收时间及附件内容保持；归属写入新 sidecar。不会根据原 MIME 收件人自动为其他地址授权。

将来允许启用多人服务后，其 `mail-directory` 应指向 bundle 的 `mail/`，`control-database` 指向旁边的 `control.sqlite`。当前公开多人服务仍不能启动。

## 多人导出为单机

仅允许显式选择一个已验证为个人箱的内部 UUID，不支持“导出全部后合并公开”。

```bash
./owlmail migrate-mailbox \
  -direction multiuser-to-standalone \
  -source /srv/owlmail/multiuser/mail \
  -control-database /srv/owlmail/multiuser/control.sqlite \
  -mailbox-id 11111111-1111-4111-8111-111111111111 \
  -destination /srv/owlmail/migrations/export-001 \
  -offline
```

输出 `mail/`、仅该箱的 `source-archive/` 和 manifest，不复制其他用户或全局控制数据库。导出 sidecar 移除多人归属、恢复单机格式；源码及附件保持一致。不要将导出目录覆盖原多人目录。

## 幂等、核验与回滚

工具先在目标所在文件系统暂存，校验邮件/附件、数量、归属及源一致性，再原子发布新 bundle。失败清理暂存，源不变。

同一命令重跑时，不仅比较参数和源指纹，还校验目标全部文件。目标已被运行中的服务修改或被篡改时，重跑会拒绝；不要把幂等功能当成运行目录修复器。

控制库可写打开会将 schema v1 事务升级至 v2；只读导出兼容 v1/v2，不偷偷升级或修改源权限/内容。回滚方式是停用新 bundle、恢复原部署指向未修改的源及其配套备份；不能只降级程序去打开新格式目录。

归档和 manifest 含完整邮件及身份信息，应按原邮箱数据权限保存。验收证据与尚未完成的真实部署项见实施进度。

## 只读诊断

`owlmail doctor -config /absolute/owlmail.yaml` 在多人配置下只读检查控制库完整性与归属图，并返回个人/待认领/退休箱及待恢复任务的聚合数量，不输出身份、邮箱或密钥。它不会升级库、重建丢失库、执行清理或解除发布门禁。


## 在线归并分批执行

登录只推进本人目标箱的一条待归并日志，每批最多替换 32 封邮件的归属 sidecar。维护 worker 每 250ms 推进一条日志，并按 ID 游标轮转，单个箱失败不会一直挡住其他箱。每批重新验证权威归属图，保留控制库、存储事务、邮件索引的原有锁顺序；批次间释放锁，未完成源箱保持冻结，重启按持久日志恢复。启动前仍校验和恢复权威数据。

后台每批使用两秒协作取消上下文，这不是文件系统 I/O 的硬超时。单次阻塞写入或大规模配额扫描仍可能持锁；本次限制写入数量并移除登录触发的全局回放，不宣称消除了所有最坏延迟。未增加外部队列、schema 变更或门禁绕过。配额不足的归并保持待处理，容量可用后继续。
