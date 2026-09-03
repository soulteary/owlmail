# CI 快速入门

将 OwlMail 作为一次性 sidecar 运行：等待就绪、执行测试，并在失败时保留日志。
示例使用 0.8.0 标签以便阅读；需要严格可复现时，请固定已记录的 manifest digest。

## GitHub Actions

```yaml
name: integration
on: [push, pull_request]

jobs:
  email:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6
        with:
          go-version-file: go.mod
      - name: Start OwlMail 0.8.0
        run: |
          docker run -d --name owlmail-ci \
            -p 127.0.0.1:1025:1025 \
            -p 127.0.0.1:1080:1080 \
            ghcr.io/soulteary/owlmail:0.8.0
          for attempt in $(seq 1 30); do
            curl --fail --silent --connect-timeout 2 --max-time 3 \
              http://127.0.0.1:1080/readyz && exit 0
            sleep 1
          done
          docker logs owlmail-ci
          exit 1
      - name: Run integration tests
        env:
          TEST_SMTP_HOST: 127.0.0.1
          TEST_SMTP_PORT: "1025"
          TEST_MAIL_API: http://127.0.0.1:1080
        run: OWLMAIL_RUN_INTEGRATION_TEST=1 go test ./examples/testing/go -v
      - name: Preserve OwlMail logs
        if: failure()
        run: docker logs owlmail-ci > owlmail.log 2>&1
      - uses: actions/upload-artifact@b7c566a772e6b6bfb58ed0dc250532a479d7789f # v6
        if: failure()
        with:
          name: owlmail-log
          path: owlmail.log
      - name: Stop OwlMail
        if: always()
        run: docker rm --force owlmail-ci || true
```

安全敏感仓库应把第三方 Action 固定到完整提交 SHA。如果测试运行在另一个容器中，
`127.0.0.1` 并不是 Docker 主机；应把两个容器放进同一网络并使用 OwlMail 服务名。

## CI 契约

| 边界 | 建议检查 |
|---|---|
| 进程存活 | `GET /healthz` |
| 依赖就绪 | `GET /readyz` |
| SMTP 入口 | 端口 1025 接受一封唯一测试邮件 |
| 邮件断言 | 原生 `/api/v1/emails` 与 `/api/v1/emails/:id` |
| 失败证据 | OwlMail 日志与应用 Mailer 日志 |

应等待 readiness，而不是固定睡眠。每次轮询都要设置超时，超时时输出日志并失败，
不要无限等待。

## 并行与清理

- 每个 CI Job 优先运行一个一次性 OwlMail。
- 使用 Job、Worker 和测试 ID 生成收件人。
- 不要让多个 OwlMail 进程共享可写邮件目录。
- 不要把 SMTP 或 Web API 暴露到公共 Runner 接口。
- 每个 Job 后删除容器；只有测试重启恢复时才挂载卷。

本地 Compose 和三种语言的示例见
[examples/testing](../../examples/testing/README.zh-CN.md)。测试生命周期见
[集成测试](./Integration-Testing.md)，持久化、认证、TLS 与 S3 部署见
[运维与排障](./Operations.md)。
