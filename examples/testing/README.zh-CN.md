# 集成测试示例

这些零依赖示例会发送一封使用唯一收件人的 SMTP 邮件，在有界截止时间内轮询
OwlMail 原生 v1 API，断言内容，并且只删除这一封邮件。

## 启动 OwlMail

```bash
docker compose -f examples/testing/compose.yaml up -d --wait
```

Compose 固定 OwlMail 0.8.0，并只在 loopback 发布两个端口。需要严格可复现的 CI
应把镜像标签替换为发布 manifest digest。

## 运行示例

```bash
# Node.js 18+
node examples/testing/javascript/email-test.mjs

# Python 3.10+
python3 examples/testing/python/email_test.py

# 使用仓库 go.mod 声明的 Go 版本
OWLMAIL_RUN_INTEGRATION_TEST=1 go test ./examples/testing/go -v
```

Go 示例要求显式设置该变量，避免常规的仓库级 `go test ./...` 依赖已启动的
OwlMail 实例。

OwlMail 位于其他地址时可覆盖端点：

```bash
TEST_SMTP_HOST=owlmail \
TEST_SMTP_PORT=1025 \
TEST_MAIL_API=http://owlmail:1080 \
node examples/testing/javascript/email-test.mjs
```

示例不会清空邮箱。每次运行都会创建唯一收件人、按完整地址过滤，并只删除返回 ID。
完整模式见[集成测试](../../docs/zh-CN/Integration-Testing.md)与
[测试配方](../../docs/zh-CN/Testing-Recipes.md)。

```bash
docker compose -f examples/testing/compose.yaml down
```
