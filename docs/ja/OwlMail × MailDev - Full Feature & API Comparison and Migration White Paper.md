# OwlMail × MailDev × MailCatcher: 完全な機能と API の比較および移行ホワイトペーパー

> **ソースコードレベルの詳細比較 + ユーザーと開発者向けの移行ガイド**

> ⚠️ **翻訳中**
> 
> この翻訳は進行中です。現在は、[英語版](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)または他の利用可能なバージョンを参照してください：
> - [English](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
> - [简体中文](../zh-CN/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
> 
> **貢献を歓迎します**：この翻訳に貢献したい場合は、[貢献ガイド](../../.github/CONTRIBUTING.md)を参照してください。

---

## 📋 エグゼクティブサマリー

OwlMail、MailDev、MailCatcher は基本的な開発ワークフローを共有しますが、**プロトコルが
同一ではなく、検証なしに置き換えることはできません**。API プレフィックス、
レスポンス、既読状態、リアルタイム通信が異なります。現在の境界は
[API リファレンス](../en/API-Reference.md)を参照してください。

OwlMail 0.8.0 は、既定で無効の読み取り専用 MCP インターフェースを Streamable HTTP と stdio で提供します。MailDev 3 の MCP はより広範で、MailCatcher に組み込み MCP はありません。既定で無効の `-maildev-rest-compat` を有効にすると、現在の MailDev REST
ルートが `/api` 配下に追加されます。Socket.IO 互換性は提供されません。

> **注**：翻訳が完了すると、完全なコンテンツが利用可能になります。それまでの間、完全な詳細については英語版を参照してください。

---

## 貢献方法

このドキュメントの翻訳を支援したい場合：

1. リポジトリをフォーク
2. 翻訳用のブランチを作成
3. [OwlMail × MailDev - Full Feature & API Comparison and Migration White Paper.md](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) の内容を翻訳
4. プルリクエストを送信

詳細については、[貢献ガイド](../../.github/CONTRIBUTING.md)を参照してください。
