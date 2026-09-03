# 测试配方

以下配方假设 OwlMail Web 位于 `http://127.0.0.1:1080`，SMTP 位于
`127.0.0.1:1025`。每个场景都应使用唯一收件人。

## 注册验证

1. 先为唯一收件人启动 `wait_for_email`，或开始使用 `to` 过滤 REST 轮询。
2. 向被测应用提交注册请求。
3. 断言主题与发件人。
4. 获取邮件详情，从 `text` 提取 URL 或验证码。
5. 对应用打开 URL，并断言账户状态变化。

AI 客户端可以使用 `registration_verification_email` MCP Prompt 完成只读邮件步骤。

## 密码重置

使用测试专属收件人，并让主题过滤条件能够排除注册邮件。打开重置 URL 前，确认其
属于预期应用 Origin。不要向测试邮箱发送真实凭据或客户地址。

## 附件投递

获取 `GET /api/v1/emails/:id` 并断言 `attachment_count`。再使用邮件元数据中的
附件路径读取已知附件，验证类型、大小与摘要。MCP `list_attachments` 有意只提供
元数据；需要字节级断言时使用 HTTP API。

## 精确 MIME Source

测试 Header、MIME boundary、传输编码或 DKIM 输入时，使用
`GET /api/v1/emails/:id/source`。邮件详情中的 HTML 已经过安全化，适合显示测试，
不适合精确 Source 测试。

## 负向投递

用唯一收件人轮询到一个短且明确的截止时间，然后断言 `total == 0`。MCP
`wait_for_email` 超时是正常结果（`timed_out: true`），不代表将来永远不会投递。

## 并行套件隔离

```text
checkout+<run-id>-<worker-id>-<test-id>@example.test
```

匹配完整地址，不要清空共享邮箱。每个 CI Job 优先使用一次性容器，并只删除当前
Job 创建的邮件。

## 重启恢复

挂载临时邮件目录，发送邮件，正常停止 OwlMail，再用同一目录重启并断言邮件仍可
查询。不要让两个可写 OwlMail 实例共享目录。单独的 `mcp-stdio` 进程是只读的，
可以检查已提交 EML。

## Webhook Handoff

使用可运行的 [Webhook 示例](../../examples/webhooks/README.zh-CN.md)。按至少一次
投递处理，使用 `X-OwlMail-Delivery-ID` 去重，并先验签再处理。SMTP 接受表示本地
持久 outbox handoff 成功，不表示每个远端目标已返回 2xx。

## Relay 测试

只有原生 `POST /api/v1/emails/:id/actions/relay` 路由会创建带状态 URL 与有界重试的
持久异步任务。历史与兼容 Relay 路由保留原有响应行为。恢复语义为至少一次，因此
下游测试接收端必须容忍重复。

基础生命周期见[集成测试](./Integration-Testing.md)，准确响应结构见
[API 参考](./API-Reference.md)。
