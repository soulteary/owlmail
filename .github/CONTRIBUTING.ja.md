# OwlMail へのコントリビューション

これは日本語の概要です。完全かつ正式な手順は
[英語版ガイド](./CONTRIBUTING.md)です。
[完全な中国語版](./CONTRIBUTING.zh-CN.md)も利用できます。

## 手順

1. 既存の [Issues](https://github.com/soulteary/owlmail/issues) と
   [ドキュメント](../docs/README.md)を検索します。
2. 変更範囲を絞ったブランチを作り、テストと文書を更新します。
3. Pull Request の前に次を実行します。

```bash
go test -race ./...
go vet ./...
bun build ./web/*.js --target=browser --outdir=./.bun-check
bun test ./tests/web ./tests/docs
```

パスワード、SMTP 認証情報、S3 キー、Webhook Secret、機密メール本文を Issue、
ログ、テストデータに掲載しないでください。コントリビューションには
[行動規範](./CODE_OF_CONDUCT.ja.md)と [MIT License](../LICENSE) が適用されます。
