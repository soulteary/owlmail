# sendmail 호환 CLI

`owlmail sendmail`은 stdin에서 RFC 5322 메시지를 읽어 OwlMail의 일반 SMTP
리스너로 전달합니다. 따라서 AUTH, TLS, 크기, 수신자 수 및 DATA 동시성 제한을
우회하지 않습니다.

```bash
owlmail sendmail -t -i < message.eml
```

`-t`는 `To`, `Cc`, `Bcc` 및 `Resent-*` 필드를 추출하여 명시적 수신자에 추가합니다. DATA 전송 전에
Bcc/Resent-Bcc 필드와 연속 줄을 모두 제거합니다. `-f ADDRESS` 또는 `-fADDRESS`는 envelope
sender를 설정하고 `-f '<>'`는 빈 reverse path를 사용합니다. `-i`, `-oi`, `--`를
지원합니다. CRLF, dot-stuffing, RFC 2047/UTF-8 헤더를 보존하며 필요하면 SMTPUTF8을
사용합니다.

## PHP

```ini
sendmail_path = "/usr/local/bin/owlmail sendmail -t -i"
```

```yaml
environment:
  OWLMAIL_SENDMAIL_HOST: owlmail
  OWLMAIL_SENDMAIL_PORT: "1025"
```

## 설정

| 옵션 | 환경 변수 | 기본값 |
|---|---|---:|
| `--host` | `OWLMAIL_SENDMAIL_HOST` | `127.0.0.1` |
| `--port` | `OWLMAIL_SENDMAIL_PORT` | `1025` |
| `--starttls` | `OWLMAIL_SENDMAIL_STARTTLS` | `false` |
| `--smtps` | `OWLMAIL_SENDMAIL_SMTPS` | `false` |
| `--username` | `OWLMAIL_SENDMAIL_USERNAME` | - |
| `--password` | `OWLMAIL_SENDMAIL_PASSWORD` | - |
| `--timeout` | `OWLMAIL_SENDMAIL_TIMEOUT` | `30s` |

STARTTLS와 SMTPS는 함께 사용할 수 없습니다. 사용자 이름과 암호를 함께 제공해야 하며,
암호는 환경 변수로 전달하는 것이 좋습니다. TLS 인증서는 시스템 신뢰 저장소와 호스트
이름으로 검증되며 검증을 건너뛰는 안전하지 않은 옵션은 없습니다.

안정적인 종료 코드는 `0` 성공, `64` 인수, `65` 메시지 데이터, `69` 영구 SMTP 오류,
`74` 로컬 I/O, `75` 임시 SMTP/네트워크 오류입니다. 암호, 메시지 본문 및 전체 AUTH
교환은 기록하지 않습니다. 전체 문서는 [영어](../en/Sendmail.md) 또는
[중국어](../zh-CN/Sendmail.md)를 참조하세요.
