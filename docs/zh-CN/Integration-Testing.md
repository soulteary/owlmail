# 使用 OwlMail 进行集成测试

OwlMail 可以作为应用发信流程的测试边界：让被测系统通过 SMTP 把邮件投递到
OwlMail，触发业务行为，再通过原生只读 HTTP API 断言捕获结果。每个测试应使用
唯一收件人或主题，避免并行任务互相读取邮件。

## 启动固定版本的测试网关

```bash
docker run --rm -d --name owlmail-test \
  -p 127.0.0.1:1025:1025 \
  -p 127.0.0.1:1080:1080 \
  ghcr.io/soulteary/owlmail:0.8.0

until curl --fail --silent http://127.0.0.1:1080/readyz >/dev/null; do sleep 1; done
```

本地测试配置为 SMTP `127.0.0.1:1025`，不启用 TLS 与认证。要求密码学级固定镜像
时，应把标签替换为发布验证阶段记录的
`ghcr.io/soulteary/owlmail@sha256:<digest>`。

## 确定性测试流程

1. 生成 `signup+<test-run-id>@example.test` 一类不会碰撞的收件人。
2. 触发被测应用发信。
3. 使用 `to`，必要时再使用 `q`，轮询 `GET /api/v1/emails`。
4. 获取 `GET /api/v1/emails/:id`，断言主题、收件人、文本、安全化 HTML、信封与附件数。
5. 仅在需要时读取附件或原始 source。
6. 只删除当前测试的邮件，或直接销毁隔离容器。

```bash
curl --fail --get http://127.0.0.1:1080/api/v1/emails \
  --data-urlencode 'to=signup+run-42@example.test' \
  --data-urlencode 'q=验证账户' \
  --data-urlencode 'limit=10'
```

集合响应包含 `total`、`limit`、`offset` 与 `emails`。在测试自身截止时间之前，
空结果不代表投递失败。应使用有界重试与短间隔，避免固定长时间 sleep。

## 隔离选择

| 范围 | 建议隔离方式 |
|---|---|
| 单个本地测试进程 | 每个测试使用唯一收件人和主题 |
| 并行测试 Worker | 每个 Worker 一个容器，或使用 Worker 专属收件人命名空间 |
| CI Job | 每个 Job 一个一次性容器 |
| 持久测试环境 | 配置保留策略，只删除当前测试拥有的 ID |

不要在共享环境调用 `DELETE /api/v1/emails`，它会清除其他测试的邮件。读取原生 v1
邮件不会自动标记为已读。

## 常见失败

| 现象 | 检查项 |
|---|---|
| SMTP 拒绝连接 | 容器状态、端口映射和应用 SMTP 主机 |
| API 就绪但没有邮件 | 应用 Mailer 日志、信封收件人和查询过滤器 |
| 读到其他测试邮件 | 使用唯一收件人，不要只匹配主题 |
| HTML 断言不同 | 断言安全化 HTML；精确 MIME 字节使用 `/source` |
| 偶发超时 | 先开始等待再触发投递，并设置明确截止时间 |

JavaScript、Go、Python 的零依赖可运行示例见
[examples/testing](../../examples/testing/README.zh-CN.md)。准确路由与响应见
[API 参考](./API-Reference.md)，Agent 场景见 [AI Agent 测试](./AI-Agent-Testing.md)。
