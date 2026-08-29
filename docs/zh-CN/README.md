# OwlMail 文档

欢迎来到 OwlMail 文档目录。本目录包含技术文档、迁移指南和 API 参考材料。

## 📸 预览

![OwlMail 预览](../../.github/assets/preview.png)

## 🎥 演示视频

![演示视频](../../.github/assets/realtime.gif)

## 🌍 Languages / 语言 / Sprachen / Langues / Lingue / 言語 / 언어

- [English](../README.md#english) | [简体中文](../README.md#简体中文) | [Deutsch](../README.md#deutsch) | [Français](../README.md#français) | [Italiano](../README.md#italiano) | [日本語](../README.md#日本語) | [한국어](../README.md#한국어)

---

## 📚 文档结构

### 主要文档

- **[API 参考](./API-Reference.md)**
  - 完整路由清单、请求与响应约定、鉴权、原生 WebSocket 事件、curl 示例，以及
    当前 MailDev 迁移边界。
  - **其他语言**：[English](../en/API-Reference.md)

- **[运维与排障](./Operations.md)**
  - 本地与 Docker 部署、持久化、安全默认值、TLS、就绪检查、Webhook 容量、
    备份升级、关闭边界和故障定位。
  - **其他语言**：[English](../en/Operations.md)

- **[Webhook 消息转发](./Webhook-Forwarding.md)**
  - 配置邮件过滤、自定义消息模板、HMAC 签名、失败重试，以及与 `soulteary/webhook` 的对接。
  - **可运行示例**：[最小、过滤、自定义、多目标、纯文本与 Compose 联动](../../examples/webhooks/README.zh-CN.md)
  - **其他语言**：[English](../en/Webhook-Forwarding.md)

- **[OwlMail × MailDev - 功能与 API 完整对比与迁移白皮书](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)**
  - OwlMail 与 MailDev 的全面对比，包括 API 兼容性、功能对等性和迁移指南。
  - **其他语言**: [English](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [Deutsch](../de/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [Français](../fr/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [Italiano](../it/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [日本語](../ja/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) | [한국어](../ko/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)

### 历史内部文档

- **[API 重构记录](./internal/API_Refactoring_Record.md)**
  - 记录无版本路由迁移到 `/api/v1/` 的历史实现过程；当前契约以 API 参考为准。
  - **其他语言**: [English](../en/internal/API_Refactoring_Record.md) | [Deutsch](../de/internal/API_Refactoring_Record.md) | [Français](../fr/internal/API_Refactoring_Record.md) | [Italiano](../it/internal/API_Refactoring_Record.md) | [日本語](../ja/internal/API_Refactoring_Record.md) | [한국어](../ko/internal/API_Refactoring_Record.md)

## 📖 如何阅读文档

文档按语言组织在不同的目录中。每个语言目录包含：
- `README.md` - 该语言的文档索引
- 主要文档（如迁移白皮书）
- `internal/` 子目录 - 历史实现记录

要切换语言，请使用本页顶部的语言选择器或访问[主文档索引](../README.md)。

## 🔄 贡献指南

添加新文档时：

1. 首先在 `en/` 目录中创建英文版本。
2. 在相应的语言目录中添加翻译版本。
3. 更新所有语言的 README 文件以包含新文档的链接。
4. 遵循现有的目录结构（文档在语言目录中，文件名不包含语言后缀）。

## 📝 文档分类

- **迁移指南**：帮助用户从 MailDev 迁移到 OwlMail
- **API 文档**：当前技术契约与历史重构记录
- **内部文档**：开发笔记和内部流程

---

有关 OwlMail 的更多信息，请访问[主 README](../../README.zh-CN.md)。
