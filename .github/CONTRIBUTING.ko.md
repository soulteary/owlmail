# OwlMail 기여 안내

이 문서는 한국어 요약입니다. 완전하고 규범적인 절차는
[영문 기여 가이드](./CONTRIBUTING.md)를 따르며,
[중국어 전체판](./CONTRIBUTING.zh-CN.md)도 제공됩니다.

## 절차

1. 기존 [Issues](https://github.com/soulteary/owlmail/issues)와
   [문서](../docs/README.md)를 검색합니다.
2. 범위가 작은 브랜치를 만들고 테스트와 문서를 함께 갱신합니다.
3. Pull Request 전에 다음 검사를 실행합니다.

```bash
go test -race ./...
go vet ./...
bun build ./web/*.js --target=browser --outdir=./.bun-check
bun test ./tests/web ./tests/docs
```

비밀번호, SMTP 자격 증명, S3 키, Webhook Secret, 민감한 이메일 내용을 Issue,
로그 또는 테스트 데이터에 게시하지 마세요. 기여에는
[행동 강령](./CODE_OF_CONDUCT.ko.md)과 [MIT License](../LICENSE)가 적용됩니다.
