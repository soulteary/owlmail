# OwlMail 单机与多人模式技术设计

- 状态：Draft，供实现与评审；本文不代表功能已实现。
- 日期：2026-11-02。
- 配套任务：[分阶段实施清单](Multiuser-Implementation-Plan.md)。
- OwlMail 源码基线：`41a10e75d726c194d9cd418e3d44fd3e069ac061`。
- Stargate 基线：`da26b02411fdb588b74971f31a59df05e24f9dee`。
- Gorge 基线：`e9ca3b7af2cc8914207fc22ffb499f5069d3364c`。
- 文中配置和接口均为拟议契约，落地时需更新 OpenAPI、配置参考和运行时校验。

## 1. 目标和非目标

默认 `standalone` 保持现有配置、启动方式、邮件目录、认证、API、WebSocket 和可选 MCP 行为。新增显式启用的 `multiuser`，每个 Stargate 稳定身份独享一个收件箱；不建设团队、共享空间、成员或协作角色。

多人模式使用系统 SMTP 凭据和信封收件地址路由。可信投递可建立待认领箱，可信登录可将其绑定到稳定身份。多人模式关闭 MCP 和 WebSocket，页面通过 HTTP 手动或定时刷新。

本期不要求 Gorge 改动代码，不新增专用适配器、个人 SMTP 密码、个人 API Token、外部数据库服务或多实例共享写入。继续遵守一个邮件目录只有一个可写实例的边界。

## 2. 架构和信任边界

```text
Phorge 决定收件地址并完成内容/附件授权
  → Gorge 现有 SMTP 适配器（系统账号，TLS）
  → OwlMail 地址路由 → 待认领箱 / 个人箱

浏览器 → 反向代理 → Stargate 身份认证
  → OwlMail 可信身份适配 → 本地身份绑定 → 个人箱访问
```

Phorge 本期确定的是地址，OwlMail 将地址映射为收件箱；Gorge 不携带最终用户身份。Warden 等目录服务由 Stargate 集成，OwlMail 的投递和查询链路不逐次访问目录。

系统 SMTP 账号只允许投递，不能登录、查信或修改绑定。邮件内容与 `To/Cc/From`、自定义 Header、`run_id` 均不能授予权限。默认单机用户不自动成为多人模式的超级用户。

### 2.1 Stargate 声明契约

必填：稳定、不可复用的 `subject`。邮箱认领还要求一个可信的 `verified_email` 及明确的验证状态。`namespace` 固定于部署配置，不能来自客户端。

拟议 Header 为 `X-Auth-User`、`X-Auth-Email`、`X-Auth-Email-Verified`。现有 Stargate 有用户和邮箱响应头，但最后一项及邮箱验证语义尚不能视为已有能力。实施前必须完成联调契约；允许通过上游受控配置保证邮箱来自已验证目录，但禁止将所有邮箱无条件标记为已验证。

代理必须移除客户端身份头，只复制 Stargate 成功认证结果。后端限制可信代理网络访问；OwlMail 验证实际连接对端，不依赖客户端 `X-Forwarded-For`。关键头为空、重复、过长或格式错误时拒绝。不能回退到 Basic Auth 或匿名。

缺少可信邮箱声明但 `subject` 有效时，允许创建个人空箱，禁止地址绑定和历史认领。后续可用运维受信映射补足。

## 3. 模式与配置

`mode` 为实例级枚举，缺省 `standalone`。模式选择必须先于路由、SMTP、MCP 和后台服务初始化。单机不得被迫创建控制数据库或配置代理。

```yaml
mode: multiuser
identity:
  namespace: main
  subject-header: X-Auth-User
  email-header: X-Auth-Email
  email-verified-header: X-Auth-Email-Verified
  trusted-proxies:
    - 172.20.0.2/32
recipient-routing:
  auto-create-pending: true
  allowed-domains:
    - example.com
  claim-policy: verified-email
  pending-retention-days: 7
system-smtp-accounts-file: /run/secrets/smtp-accounts.yaml
control-database: /data/control.sqlite
mail-directory: /data/mail
refresh-interval-seconds: 15
```

以上嵌套 YAML 是目标表达，不是当前配置解析器已支持的格式。实现应遵循仓库现有平面配置风格，采用等价的平面键，或单独 ADR 明确嵌套配置兼容方案；不得悄然改变既有解析优先级。

多人模式必须明确选择认领策略，并配置可信代理、系统账号、允许域和配额。启用 MCP、匿名 SMTP 等冲突配置时启动报错。静态文件、健康探针的豁免保持最小范围，不能豁免邮件资源；保留现有探针契约。

首版配置变更通过重启生效。系统账号文件保存密码散列，不记录明文；Gorge 持有明文密钥。控制库损坏或缺失时，多人模式拒绝启动，不自动创建空库绕过已有归属。

## 4. 权威数据模型

新增独立 SQLite 控制库；身份、路由、绑定和迁移日志属于权威数据。现有可选邮件索引仍为可重建缓存。所有 ID 均由服务端生成，不能把原始用户 ID 或邮箱拼接为路径。

### Identity

`id`、`provider_namespace`、`external_subject`、`personal_mailbox_id`、`created_at`、`last_seen_at`。

唯一约束：`(provider_namespace, external_subject)`、`personal_mailbox_id`。一身份一有效个人箱。邮箱、姓名只是可更新展示信息。

### Mailbox

`id`、`state(pending/personal/merged)`、`owner_identity_id`、`merged_into_id`、`created_at`、`claimed_at`。

pending 无 owner；personal 有唯一 owner；merged 有合法目标。目标必须是同 namespace 的个人箱，禁止循环和多级重定向。外部请求不能指定箱 ID 选择归属。

### RecipientRoute

`id`、`namespace`、`normalized_address`、`mailbox_id`、`state(pending/bound/blocked)`、`revision`、`created_at`、`bound_at`。

唯一约束：`(namespace, normalized_address)`。多个地址可指向同一箱。域名规范化，本地部分精确匹配；不去除 `+tag`、不自动合并别名、不猜测全局大小写等价。校验、规范化函数供 SMTP、登录和运维映射共用。

### SystemSMTPAccount

`id`、`name`、`secret_hash`、`enabled`、`allowed_namespace`、`allowed_recipient_domains`、`creation_limits`、`created_at`。

账号允许域与实例允许域取交集；未知地址创建同时受账号和全局限额约束。账号撤销后拒绝新的 SMTP 事务，已有 DATA 按明确的事务边界处理。

### BindingAudit / MergeJournal

审计保存 `identity_id`、`route_id`、来源与目标箱、`evidence_type(trusted_login/operator_mapping)`、映射版本、时间和结果。不得记录 Cookie 或密码。

归并日志保存操作 ID、源/目标箱、状态、进度与错误；记录必须足够支持幂等恢复，不能只依赖内存队列。

### 邮件权威元数据

增加 `mailbox_id`、`ingest_account_id`、`matched_recipients`、`route_revisions`、`receive_transaction_id`、元数据版本。EML 保持不变。附件、Relay/Webhook 任务与历史均保留可验证的箱归属。

控制库与文件系统没有共同事务。所有跨库/文件变更必须用持久 journal 和可恢复提交协议，禁止用一次 SQLite commit 假装完成跨存储原子提交。

## 5. 创建与认领状态机

### 5.1 未知地址投递

系统账号认证通过后，在 RCPT 阶段校验域名和配额。无路由时，在数据库事务内创建唯一 pending 箱和路由。并发使用唯一约束冲突重读，不重复建箱。

RCPT 后未执行 DATA 或提交失败，可能留下空 pending 箱；通过独立保留策略清理，避免 RCPT 探测耗尽资源。创建速率按尝试计费，不能靠失败事务绕过。

### 5.2 首次登录与个人箱

可信 subject 查找或创建 Identity。无个人箱时：若可认领 pending 箱，直接升级为 personal；否则创建空个人箱。个人箱与身份的创建在一个控制库事务内完成。

认领多个地址时，只接受上游可信声明或运维显式映射。首版可仅处理一个已验证邮箱，不从 MIME、浏览器输入或未验证的邮箱列表扩展。

### 5.3 认领策略

`verified-email`：可信邮箱登录自动认领保留期内历史，并绑定未来路由。仅适用于待认领期间地址不会被回收/重新分配的环境；部署必须明确接受该前提。

`explicit-history`：登录仅把未来路由指向个人箱，历史 pending 箱保留隔离状态；通过运维确认历史归属后迁移。无法确认当前与历史地址归属一致时使用此策略。

当前邮箱所有权不能证明历史所有权。该限制是产品语义，不应通过配置默认“已验证”掩盖。

### 5.4 已有个人箱的历史归并

已有个人箱时，不创建第二个可见收件箱。建议分为两步：控制库事务先将未来路由指向个人箱并创建归并日志；后台在持久 journal 下迁移源箱邮件元数据和索引，完成后标记源箱 merged。

归并期间源箱进入冻结状态，普通用户暂时不可读；新邮件进入目标个人箱。前端可以显示个人认领处理中，不允许全局搜索绕过。任务按邮件 ID 幂等更新，崩溃后继续，失败不撤销已正确绑定的新路由。归并不能重发 Webhook/Relay。

### 5.5 冲突和地址重新分配

同身份重复登录为幂等成功。不同身份请求认领已绑定地址时拒绝该绑定，保留其自身个人箱访问，返回不泄漏旧身份的冲突提示并审计。

邮箱从上游声明消失不自动释放旧地址。地址重新分配必须运维显式阻断旧路由、递增版本、建立新路由目标；旧邮件保留原 owner。稳定 subject 必须不可回收。

## 6. SMTP 提交、并发和恢复

RCPT 保存路由 ID/版本，在提交时重新解析。认领引起的合法路由迁移可跟随目标；blocked、跨身份重分配或版本含义不明确时临时拒绝，不使用过期归属。

实现先固定锁顺序：路由/归并协调锁 → 接收提交协调 → 邮件存储锁。锁内不调用网络。禁止同时在不同顺序获取控制库事务和邮件锁；用确定性并发测试验证。

同事务按最终箱 ID 去重：同用户多个别名保存一份；不同箱各保存独立副本及邮件 ID。API 信封只显示本箱匹配地址，原始 MIME 不重写，因此其中原有 To/Cc 或正文可能暴露其他收件人。按人内容保密仍由 Phorge 负责。

提交协议：暂存全部副本/附件/归属 → 写持久接收 journal → 安装并持久化全部最终文件 → 持久提交标记 → 发布可见状态/索引 → SMTP 250。索引失败不推翻已接受结果；恢复后可重建。无提交标记时回滚整组副本；有提交标记时保留现存已提交文件，仅清理 journal 并重建内存索引，不能从旧暂存副本复活随后已删除的邮件。

未认证返回认证错误；未映射且禁止创建、域名不允许返回永久拒绝；临时存储/配额阻塞返回临时错误。不得声称未接受已提交的邮件。SMTP 接受回执丢失仍可能重复；不按 Message-ID 盲目去重，不承诺 exactly-once。

## 7. 查询与操作隔离

新增带身份的 mailbox service，所有用户访问通过它获取作用域，不向 handler 暴露全局存储操作。先限定箱，再过滤、排序、分页、统计；内存与 SQLite 回退行为一致。

覆盖列表、详情、HTML、源码、附件/内联图片、ZIP、已读、删除/清空、run_id 清理、cursor/等待、Relay/Webhook 历史和重试。跨用户 ID 统一不存在；批量操作预检查全部目标；cursor 绑定箱和实例 epoch。待认领箱和冻结归并源不进入用户查询。

旧 MailDev/MailCatcher API 逐路由审计；多人首版未完成隔离的接口关闭。全局 reload、配置、存储诊断仅运维使用，不新增可读所有邮件的默认管理员 Web 入口。

## 8. 前端与能力开关

多人模式不注册 MCP、stdio、WebSocket 或历史别名，不初始化广播连接表和监听。模式与能力由不泄漏身份的启动契约传给前端。

开页立即获取列表/统计；默认 15 秒局部轮询，可关闭；后台暂停，恢复可见刷新；不重叠，失败退避，401 停止。保留筛选、选中邮件和滚动，未变化不重新取正文。ETag/版本仅反映本箱变化。

API、源码、附件使用禁止持久缓存的响应策略，不进入 Service Worker。账号切换或认证失效清空前端邮件内存；偏好按身份分区。前端不能仅依赖自身清理保障服务器权限。

## 9. 投递出口、配额和运维

pending 邮件默认不触发 Webhook/Relay；认领和历史归并不重放。已绑定箱的规则由运维显式配置范围，后台任务恢复后仍验证归属。不能继承全局规则为多人默认外发规则。

限制全局磁盘、并发、账号投递/建箱速率、pending 数量/容量/期限、个人邮件数量/容量、导出和等待请求。subject/邮箱不作为无限基数指标标签。健康检查提供聚合状态；日志脱敏地址并记录操作 ID。

身份、控制库、邮件 journal 恢复完成前多人 readiness 为失败；健康接口不得暴露地址、邮件或账号配置。

## 10. Gorge 部署兼容

复用 `SMTP_HOST/PORT/USER/PASSWORD/PROTOCOL`，使用受信证书和隐式 TLS：`SMTP_PROTOCOL=ssl`。当前 Gorge 的 `tls` 也走隐式 TLS，不能按名字误配为 STARTTLS。

OwlMail 为 Gorge 创建投递专用系统账号。只需改 Gorge SMTP 后端配置，不改请求契约。若 OwlMail 为唯一预期目的地，限制候选后端，避免 failover 到真实邮箱服务。保持 Gorge 的未知投递结果不自动重发语义。

## 11. 迁移与回滚

默认单机兼容是逐阶段发布门槛。不得静默升级单机目录为多人目录。模式/目录格式标记用于阻止错模式打开；旧二进制不会识别新标记，因此禁止用旧程序直接打开多人数据。

单机到多人：停写 → 完整备份 → 显式指定历史稳定身份 → 生成控制库和归属 → 校验数量/附件/任务 → 重建索引 → 启动验证。不得归给首个登录者。

备份控制库一致性快照、EML、元数据、附件及未完成 journal。首版提供停机备份流程；不承诺逐文件在线复制一致性。恢复前完成路由、归并和接收事务恢复。回滚恢复完整迁移前备份，不能只降级二进制。多人到单机需显式导出指定个人箱，禁止自动合并所有用户数据。

## 12. 发布条件与未决项

必须解决：Stargate 可信邮箱声明契约；认领策略选择；控制库/文件 journal 协议；配置平面键契约；所有数据出口审计。未满足时不得公开多人模式。

验收必须覆盖邮件先到后认领、并发首次登录/投递、已有个人箱归并、认领与 DATA 竞争、地址重分配、跨用户资源访问、崩溃恢复、索引回退、能力禁用以及单机回归。详见配套任务清单。

## 13. 参考源码

- [OwlMail 当前边界](https://github.com/soulteary/owlmail/blob/41a10e75d726c194d9cd418e3d44fd3e069ac061/README.zh-CN.md)
- [SMTP 认证](https://github.com/soulteary/owlmail/blob/41a10e75d726c194d9cd418e3d44fd3e069ac061/internal/mailserver/auth.go)
- [邮件模型](https://github.com/soulteary/owlmail/blob/41a10e75d726c194d9cd418e3d44fd3e069ac061/internal/types/types.go)
- [Stargate 身份输出](https://github.com/soulteary/stargate/blob/da26b02411fdb588b74971f31a59df05e24f9dee/src/internal/handlers/forwardauth.go)
- [Gorge SMTP](https://github.com/soulteary/gorge/blob/e9ca3b7af2cc8914207fc22ffb499f5069d3364c/go/internal/mailer/smtp.go)
- [Gorge 投递职责](https://github.com/soulteary/gorge/blob/e9ca3b7af2cc8914207fc22ffb499f5069d3364c/docs/modules/mailer.md)
