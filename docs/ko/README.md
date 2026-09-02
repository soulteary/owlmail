# OwlMail 문서

OwlMail 문서에 오신 것을 환영합니다. 문서는 언어별 디렉터리로 구성됩니다.

## 📸 미리보기

![OwlMail 미리보기](../../.github/assets/preview.png)

## 🎥 데모

![데모](../../.github/assets/realtime.gif)

## 🌍 언어

- [English](../en/README.md) | [简体中文](../zh-CN/README.md) | [Deutsch](../de/README.md) | [Français](../fr/README.md) | [Italiano](../it/README.md) | [日本語](../ja/README.md) | [한국어](./README.md)

## 📚 문서

### 운영 참조

- **[API 참조](../en/API-Reference.md)** (English, [中文](../zh-CN/API-Reference.md))
  - 경로, 인증, 응답 형식, WebSocket 이벤트 및 MailDev와의 차이점.
  - 기계 판독 가능: [OpenAPI 3.1 JSON](../../openapi/openapi.json) | [YAML](../../openapi/openapi.yaml)
- **[운영 및 문제 해결](../en/Operations.md)** (English, [中文](../zh-CN/Operations.md))
  - 배포, 영속성, 보안, TLS, 용량 및 장애 진단.
- **[Webhook 전달](../en/Webhook-Forwarding.md)** (English, [中文](../zh-CN/Webhook-Forwarding.md))
  - 필터, 사용자 지정 페이로드, HMAC 서명, 재시도 및 `soulteary/webhook` 연동.
- **[실행 가능한 Webhook 예제](../../examples/webhooks/README.md)** (English, [中文](../../examples/webhooks/README.zh-CN.md))

### 릴리스

- **[0.6.0 릴리스 노트](../en/Release-0.6.0.md)** (English, [中文](../zh-CN/Release-0.6.0.md))
- **[릴리스 절차](../en/Releasing.md)** (English, [中文](../zh-CN/Releasing.md))

### 비교 및 마이그레이션

- **[OwlMail × MailDev – 비교 및 마이그레이션 가이드](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)**
  - 이 번역은 미완성입니다. 전체 내용은
    [영어](../en/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)
    또는 [중국어](../zh-CN/OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md) 버전을 참조하세요.

### 과거 내부 문서

- **[API 리팩토링 기록](./internal/API_Refactoring_Record.md)**
  - 과거 구현 기록입니다. 현재 계약은 API 참조를 기준으로 합니다.

## 🔄 기여

새 문서는 먼저 `docs/en/`에 만들고, 번역은 해당 언어 디렉터리에서 동일한
파일 이름을 사용합니다. [기본 인덱스](../README.md)도 업데이트하세요. 자세한
내용은 [기여 가이드](../../.github/CONTRIBUTING.ko.md)를 참조하세요.

자세한 내용은 [메인 README](../../README.ko.md)를 참조하세요.
