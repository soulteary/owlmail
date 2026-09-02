# Sendmail 兼容 CLI

`owlmail sendmail` 为 PHP、Cron 和传统程序提供 sendmail 风格的进程接口，
但邮件仍通过 OwlMail 的正常 SMTP 监听器投递，不会直接写入邮箱。因此服务端配置的
SMTP AUTH、TLS 策略、邮件大小、收件人数和 DATA 并发限制都会正常生效。

## 基本用法

显式指定收件人：

```bash
printf 'From: app@example.test\nSubject: Job finished\n\nDone.\n' |
  owlmail sendmail operator@example.test
```

也可以从 `To`、`Cc`、`Bcc` 及对应的 `Resent-*` 字段提取信封收件人：

```bash
owlmail sendmail -t -i < message.eml
```

显式收件人与 `-t` 提取的收件人会合并。发送 DATA 前会删除所有 `Bcc`、
`Resent-Bcc` 字段及其
折行续行。`-i` 和 `-oi` 作为兼容参数接受；SMTP 客户端始终正确处理 CRLF 和
dot-stuffing。`-f '<>'` 表示空 envelope sender。RFC 2047 与 UTF-8 头保持原样；
原始 UTF-8 头或国际化信封地址需要时会协商 SMTPUTF8。

## PHP `sendmail_path`

先把 OwlMail 二进制放入 PHP 容器或主机，再在 `php.ini` 中配置：

```ini
sendmail_path = "/usr/local/bin/owlmail sendmail -t -i"
```

通过 PHP 进程继承的环境变量指向 OwlMail。假设 Compose 服务名为 `owlmail`：

```yaml
environment:
  OWLMAIL_SENDMAIL_HOST: owlmail
  OWLMAIL_SENDMAIL_PORT: "1025"
```

此后 PHP `mail()` 也会通过同一 SMTP 安全边界投递：

```php
<?php
mail('developer@example.test', 'OwlMail test', 'It works!', [
    'From' => 'php@example.test',
]);
```

## SMTP 配置

命令行参数优先于环境变量。

| 参数 | 环境变量 | 默认值 | 作用 |
|---|---|---:|---|
| `--host` | `OWLMAIL_SENDMAIL_HOST` | `127.0.0.1` | OwlMail SMTP 主机 |
| `--port` | `OWLMAIL_SENDMAIL_PORT` | `1025` | OwlMail SMTP 端口 |
| `--starttls` | `OWLMAIL_SENDMAIL_STARTTLS` | `false` | 要求服务端声明并完成 STARTTLS 升级 |
| `--smtps` | `OWLMAIL_SENDMAIL_SMTPS` | `false` | 连接建立时直接使用 TLS |
| `--username` | `OWLMAIL_SENDMAIL_USERNAME` | - | PLAIN AUTH 用户名 |
| `--password` | `OWLMAIL_SENDMAIL_PASSWORD` | - | PLAIN AUTH 密码 |
| `--timeout` | `OWLMAIL_SENDMAIL_TIMEOUT` | `30s` | 整次 SMTP 投递时限，包括连接和 DATA 接受 |

同时支持 `--smtp-host`、`--smtp-ip` 和 `--smtp-port` 别名。STARTTLS 与 SMTPS
互斥，用户名和密码必须同时配置。建议使用 `OWLMAIL_SENDMAIL_PASSWORD`，避免
命令行密码出现在进程列表中。

连接要求 TLS 的 OwlMail：

```bash
export OWLMAIL_SENDMAIL_HOST=owlmail.example.test
export OWLMAIL_SENDMAIL_PORT=1025
export OWLMAIL_SENDMAIL_STARTTLS=true
export OWLMAIL_SENDMAIL_USERNAME=app
export OWLMAIL_SENDMAIL_PASSWORD='replace-me'
owlmail sendmail -t < message.eml
```

SMTPS 通常使用 465 端口，并设置 `OWLMAIL_SENDMAIL_SMTPS=true`。证书会根据操作
系统信任库和目标主机名进行校验。使用私有证书时，应把私有 CA 安装到该信任库；
命令有意不提供跳过证书校验的选项。

## 兼容参数

- `-t`：加入所有 `To`、`Cc`、`Bcc`、`Resent-To`、`Resent-Cc`、
  `Resent-Bcc` 地址。
- `-f ADDRESS` 或 `-fADDRESS`：设置 envelope sender；`-f '<>'` 使用空反向路径。
- `-i`、`-oi`：接受但无需额外处理的兼容形式。
- `--`：结束参数解析，后续值一律作为收件人。

未知 sendmail 参数会明确失败，不会静默忽略。

## 退出码

以下稳定值遵循 `sysexits(3)` 约定：

| 退出码 | 含义 |
|---:|---|
| `0` | 服务端已经接受 DATA |
| `64` | 参数或客户端配置错误 |
| `65` | RFC 5322 头无效，或 `-t` 未找到收件人 |
| `69` | 永久 SMTP 错误，包括 5xx 响应 |
| `74` | 本地 stdin/读取错误 |
| `75` | 临时 SMTP 4xx 或网络错误，可以重试 |

服务端在 DATA 后返回 `250` 即构成接受边界；此后的 QUIT 失败仍返回成功，避免调用方
重复投递。诊断信息不会包含密码、邮件正文或完整认证交换。
