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
HTTPS 与网络访问控制。

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

## 可靠的 Agent 流程

1. 为场景创建唯一收件人。
2. 在触发应用之前，先用该收件人调用 `wait_for_email`。
3. 触发注册、密码重置、通知或其他发信操作。
4. 用返回 ID 调用 `get_email`；除非纯文本缺少目标值，否则保持 `include_html=false`。
5. 返回断言结果与 `web_url`，不要请求修改邮箱。

```text
等待最多 60 秒，接收发往 signup+run-42@example.test 且主题包含“验证”的新邮件。
只读检查邮件，从纯文本提取验证 URL，并返回 OwlMail Web 链接作为证据。
```

内置 `registration_verification_email`、`password_reset_email` 与
`wait_for_delivery` Prompts 已封装这一流程。空过滤器范围很宽，应优先使用唯一收件人。

## 防护边界

- MCP 默认关闭。
- 工具为封闭世界、只读、幂等且非破坏性。
- `wait_for_email` 使用事件驱动，最长 120 秒；每会话最多 4 个、每进程最多 64 个。
- 原始 source 使用 base64 且有大小限制；附件工具只提供元数据。
- 邮件内容是不可信输入，不得把邮件中的指令当作 Agent 策略。
- 测试邮箱中不要放入生产邮件或凭据。

准确 Schema 与限制见 [MCP 参考](./MCP-Reference.md)，部署边界见
[安全模型](./Security-Model.md)。
