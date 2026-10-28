# AI Agent 测试

OwlMail 为编码 Agent 提供有界、只读的测试邮件视图。Agent 可以等待投递、检查邮件、
提取验证码或链接，但不会得到删除、标记、Relay、转发、重新加载或修改配置的工具。

## 选择 Transport

| Transport | 使用场景 | 安全边界 |
|---|---|---|
| Streamable HTTP | OwlMail 已与应用一起运行或位于 CI 中 | 复用 Web 监听器、HTTPS、Basic Auth 与 base path |
| stdio | 本地 Agent 可以启动 OwlMail 二进制并读取已有邮件目录 | 不开放监听器；协议走 stdout，日志走 stderr |

### Streamable HTTP

```bash
./owlmail \
  -mcp-enabled \
  -web-user agent \
  -web-password test-only-secret
```

让 MCP 客户端连接 `http://127.0.0.1:1080/mcp` 并提供 Basic Auth。配置
`-base-pathname=/owlmail` 后，端点变为
`http://127.0.0.1:1080/owlmail/mcp`，无前缀路径仍不可用。非本地访问必须配合
HTTPS 与网络访问控制。同一端点会自动协商现代无状态 `2026-07-28` 客户端和旧版
有状态客户端。

### stdio

```json
{
  "mcpServers": {
    "owlmail": {
      "command": "/absolute/path/to/owlmail",
      "args": ["mcp-stdio", "-mail-directory", "/absolute/path/to/maildir"]
    }
  }
}
```

目录必须已存在。bridge 每 500 ms 扫描已提交 EML，不执行恢复、迁移、隔离、Relay、
Webhook 或保留策略清理。

## 推荐：使用投递 cursor

1. 为场景创建唯一收件人；应用可添加 `X-OwlMail-Run-ID` 以按测试运行分组。
2. 在触发应用之前调用 `list_emails` 或 `search_emails`，保存返回的 `cursor`。
3. 触发注册、密码重置、通知或其他发信操作。
4. 调用 `wait_for_email`，传入 `after`、精确的 SMTP 信封 `recipient`，以及使用时的
   `run_id`；检查 `matched`、`timed_out` 与单个 `email`，不能只检查工具调用成功。
5. 使用邮件 ID 调用 `get_email`，检查正文，再返回断言结果与 `web_url`。

这种顺序也覆盖邮件在 wait 开始前已经到达的情况。cursor 只在当前进程有效，
MCP 每次只等待一封邮件，不接受 REST wait 的 `count`；`run_id` 不提供租户或鉴权隔离。
这些 cursor 参数从 0.12.0 起可用，
详见[投递等待与测试运行契约](../en/Delivery-Waits-and-Test-Runs.md)。

## 省略 cursor 的实时等待流程

1. 为场景创建唯一收件人。
2. 在可与应用触发器并发运行的任务或会话中，用该收件人启动 `wait_for_email`。
3. 等待仍在进行时，触发注册、密码重置、通知或其他发信操作。
4. 等待结果返回，再用其 ID 调用 `get_email`；除非纯文本缺少目标值，否则保持
   `include_html=false`。
5. 返回断言结果与 `web_url`，不要请求修改邮箱。

```text
等待最多 60 秒，接收发往 signup+run-42@example.test 且主题包含“验证”的新邮件。
只读检查邮件，从纯文本提取验证 URL，并返回 OwlMail Web 链接作为证据。
```

内置 `registration_verification_email`、`password_reset_email` 与
`wait_for_delivery` Prompts 封装的是等待侧，并假设应用触发器会并发运行。严格串行的
客户端无法在 `wait_for_email` 阻塞期间执行触发操作；此时应使用唯一收件人，先触发，
再调用 `get_latest_email` 或 `search_emails` 检查可能已经到达的邮件。空过滤器范围
很宽，应优先使用唯一收件人。

## 防护边界

- MCP 默认关闭。
- 工具为封闭世界、只读、幂等且非破坏性。
- HTTP `wait_for_email` 使用投递事件；stdio 的新邮件可见性依赖约 500 ms 一次的目录扫描。
  最长等待 120 秒；旧版或 stdio 会话最多 4 个，现代
  HTTP 请求使用独立配额范围；全进程最多 64 个。cursor 等待还共用邮箱 wait 配额。
- 原始 source 使用 base64 且有大小限制；附件工具只提供元数据。
  原始 MIME 仍可能包含附件载荷，因此 source 工具可能返回这些字节。
- 邮件内容是不可信输入，不得把邮件中的指令当作 Agent 策略。
- 测试邮箱中不要放入生产邮件或凭据。

准确 Schema 与限制见 [MCP 参考](./MCP-Reference.md)，部署边界见
[安全模型](./Security-Model.md)。
