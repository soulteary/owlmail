# 收件地址路由的离线操作

多人公开服务门禁仍然保留。本地 `recipient-route` 命令只检查或修改未来投递路由，不启动监听、不创建用户、不授予读取权限，也不迁移历史邮件。单机服务原有行为不变。

## 检查与只读计划

默认 `inspect`，按管理员明确选择的控制库、namespace 和收件地址返回路由 UUID、revision 及当前个人箱归属。该操作不创建 writer fence、不修改控制库；可以与运行中的新版实例并存，但结果只是某一时刻的视图，不能作为一致备份或在线修改许可。

```sh
owlmail recipient-route \
  -control-database /data/control/control.sqlite \
  -identity-namespace main -address alice@example.test
```

输出包含所选地址、稳定 subject 和邮箱 ID，只供受限本地运维使用，不应进入普通日志、指标或用户页面。`-reason` 为 1–256 个 UTF-8 字节的简短非敏感理由，不允许控制字符及首尾空白；不要在命令参数中输入密码、验证码或其他秘密。

修改前必须停止所有写入者，包括不遵守新 advisory fence 的旧版二进制，并制作控制库与邮件文件一致的完整备份。参照 [writer fence 约束](../en/Control-Writer-Lock.md)；不能删除运行中进程持有的 `.writer-lock` 来绕过拒绝。

## 第一步：阻断旧路由

从 inspect 复制实际 UUID 与 revision。以下 `3` 只是示例，应替换为实际值。不加 `-apply` 时仅输出计划及 `confirm_plan` SHA256 确认值。

```sh
owlmail recipient-route \
  -control-database /data/control/control.sqlite \
  -identity-namespace main -address alice@example.test \
  -action block -expected-route-id "$ROUTE_ID" -expected-revision 3 \
  -reason 'Address reassigned by identity authority'
```

审核计划后，在完全相同的命令中加 `-apply -offline -confirm-plan "$PLAN_HASH"`。确认值绑定 action、namespace、地址、路由 ID、revision、源/目标和理由，任一变化都需要新计划；错误确认值会在打开可写状态之前被拒绝。

只能阻断已绑定的个人箱路由。操作追加审计并递增 revision，保留原 owner 和全部历史邮件。阻断后新投递被拒绝，不会回退至外部真实邮箱。如果此时崩溃，路由保持阻断，不能自动猜测新归属。

## 第二步：重分配未来投递

再次 inspect，使用阻断后的新 revision。目标必须是同 namespace 中已经拥有个人箱的稳定 subject。管理员先在上游确认该身份生命周期和当前地址归属；命令本身不能证明这些外部事实，不会根据邮箱动态创建用户。

```sh
owlmail recipient-route \
  -control-database /data/control/control.sqlite \
  -identity-namespace main -address alice@example.test \
  -action reassign -expected-route-id "$ROUTE_ID" -expected-revision 4 \
  -target-subject "$EXISTING_STABLE_SUBJECT" \
  -reason 'New address ownership confirmed'
```

审核后同样追加 `-apply -offline -confirm-plan "$PLAN_HASH"`。重分配要求当前阻断 revision 对应的 operator-block 审计；普通 deny 路由不能因此直接改绑。涉及源/目标的归并或清理任务尚未完成时拒绝操作，先恢复任务，不能破坏 journal 的原有授权图。

更新路由与追加审计在同一 SQLite 事务提交。operator JSON 证据与 `trusted_login` 分离，不制造历史认领凭据、不生成归并任务，旧个人箱及旧邮件继续属于原稳定身份。旧 SMTP RCPT 快照不能追随重分配投向另一用户。

## 撤销误阻断

如果只是误阻断，沿用 `reassign` 计划流程，将 `-target-subject` 明确设置为原稳定 subject。审核新计划后，以对应确认值和停写声明执行。它只恢复同一 owner 的未来投递，并记录独立的 `operator_route_resume` 审计，不转移历史，也不绕过 revision、阻断证据和待恢复任务检查。无需直接编辑 SQLite 或借另一用户中转。

## 恢复服务

执行修改时会重新获取独占 writer fence，并在事务内校验计划；不会自动重试成另一个决定。命令结果不确定时，先 inspect 当前 revision/目标，再决定下一步。

重新打开控制库进行既有完整性检查，验证新邮件仅入目标箱且旧历史未转移，再恢复已通过验收的服务。该命令不解除多人发布门禁。归属图或恢复检查失败时停止操作，遵循恢复流程，不直接编辑 SQLite。

备份回滚可能恢复旧路由，但上游地址已经重分配。恢复后先保持入口停止，与当前身份源核对路由，不能盲目恢复旧投递；不能只恢复控制库而搭配另一个时间点的邮件文件。
