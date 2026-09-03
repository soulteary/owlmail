# OwlMail ドキュメント

OwlMail のドキュメントへようこそ。文書は言語別のディレクトリで管理されています。

## 📸 プレビュー

![OwlMail プレビュー](../../.github/assets/preview.png)

## 🎥 デモ

![デモ](../../.github/assets/realtime.gif)

## 🌍 言語

- [English](../en/README.md) | [简体中文](../zh-CN/README.md) | [Deutsch](../de/README.md) | [Français](../fr/README.md) | [Italiano](../it/README.md) | [日本語](./README.md) | [한국어](../ko/README.md)

## 📚 ドキュメント

### 運用リファレンス

- **テストと Agent** (English、[中文](../zh-CN/Integration-Testing.md))：
  [統合テスト](../en/Integration-Testing.md)、[CI](../en/CI-Quickstart.md)、
  [AI Agent](../en/AI-Agent-Testing.md)、[MCP](../en/MCP-Reference.md)、
  [レシピ](../en/Testing-Recipes.md)、[アーキテクチャ](../en/Architecture.md)、
  [セキュリティモデル](../en/Security-Model.md)。
- **[実行可能な JavaScript、Go、Python、Compose 例](../../examples/testing/README.md)**
- **[API リファレンス](../en/API-Reference.md)** (English、[中文](../zh-CN/API-Reference.md))
  - ルート、認証、レスポンス形式、WebSocket イベント、および MailDev との相違点。
  - 機械可読：[OpenAPI 3.1 JSON](../../openapi/openapi.json) | [YAML](../../openapi/openapi.yaml)
- **[運用・トラブルシューティング](../en/Operations.md)** (English、[中文](../zh-CN/Operations.md))
  - デプロイ、永続化、セキュリティ、TLS、容量、障害診断。
- **[sendmail 互換 CLI](./Sendmail.md)**
  - PHP `sendmail_path`、Cron、SMTP/TLS/AUTH、安定した終了コード。
- **[Webhook 転送](../en/Webhook-Forwarding.md)** (English、[中文](../zh-CN/Webhook-Forwarding.md))
  - フィルター、カスタムペイロード、HMAC 署名、再試行、`soulteary/webhook` 連携。
- **[実行可能な Webhook 例](../../examples/webhooks/README.md)** (English、[中文](../../examples/webhooks/README.zh-CN.md))

### リリース

- **[0.9.0 リリースノート](../en/Release-0.9.0.md)** (English、[中文](../zh-CN/Release-0.9.0.md))
- **[0.8.0 リリースノート](../en/Release-0.8.0.md)** (English、[中文](../zh-CN/Release-0.8.0.md))
- **[リリース手順](../en/Releasing.md)** (English、[中文](../zh-CN/Releasing.md))

### 比較と移行

- **[OwlMail × MailDev – 比較・移行ガイド](./Comparison-and-Migration.md)**
  - この翻訳は未完成です。完全な内容は
    [英語版](../en/Comparison-and-Migration.md)
    または[中国語版](../zh-CN/Comparison-and-Migration.md)を参照してください。

### 過去の内部資料

- **[API リファクタリング記録](./internal/API_Refactoring_Record.md)**
  - 過去の実装記録です。現在の契約は API リファレンスを参照してください。

## 🔄 コントリビューション

新規文書はまず `docs/en/` に作成し、翻訳は各言語ディレクトリで同じファイル名を
使用します。[メインインデックス](../README.md)も更新してください。詳細は
[コントリビューションガイド](../../.github/CONTRIBUTING.ja.md)を参照してください。

詳細は[メイン README](../../README.ja.md)をご覧ください。
