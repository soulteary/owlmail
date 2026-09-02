# sendmail 互換 CLI

`owlmail sendmail` は stdin から RFC 5322 メッセージを読み、OwlMail の通常の
SMTP リスナーへ送信します。AUTH、TLS、サイズ、受信者数、DATA 同時実行数の
制限を迂回しません。

```bash
owlmail sendmail -t -i < message.eml
```

`-t` は `To`、`Cc`、`Bcc` と `Resent-*` フィールドを抽出し、明示的な受信者に追加します。DATA の前に
Bcc、Resent-Bcc と継続行をすべて削除します。`-f ADDRESS` または `-fADDRESS` は envelope
sender を指定し、`-f '<>'` は空の reverse path を指定します。`-i`、`-oi`、
`--` に対応します。CRLF、dot-stuffing、RFC 2047/UTF-8 ヘッダーを保持し、必要時
は SMTPUTF8 を使用します。

## PHP

```ini
sendmail_path = "/usr/local/bin/owlmail sendmail -t -i"
```

```yaml
environment:
  OWLMAIL_SENDMAIL_HOST: owlmail
  OWLMAIL_SENDMAIL_PORT: "1025"
```

## 設定

| オプション | 環境変数 | 既定値 |
|---|---|---:|
| `--host` | `OWLMAIL_SENDMAIL_HOST` | `127.0.0.1` |
| `--port` | `OWLMAIL_SENDMAIL_PORT` | `1025` |
| `--starttls` | `OWLMAIL_SENDMAIL_STARTTLS` | `false` |
| `--smtps` | `OWLMAIL_SENDMAIL_SMTPS` | `false` |
| `--username` | `OWLMAIL_SENDMAIL_USERNAME` | - |
| `--password` | `OWLMAIL_SENDMAIL_PASSWORD` | - |
| `--timeout` | `OWLMAIL_SENDMAIL_TIMEOUT` | `30s` |

STARTTLS と SMTPS は同時に指定できません。ユーザー名とパスワードは両方必要で、
秘密情報は環境変数から渡すことを推奨します。TLS 証明書は OS の信頼ストアとホスト名
で検証され、検証を無効にする危険なオプションはありません。

安定した終了コードは、`0` 成功、`64` 引数、`65` メッセージデータ、`69` 恒久的
SMTP エラー、`74` ローカル I/O、`75` 一時的 SMTP/ネットワークエラーです。
パスワード、本文、AUTH の完全な交換は記録しません。完全な説明は
[英語](../en/Sendmail.md)または[中国語](../zh-CN/Sendmail.md)を参照してください。
