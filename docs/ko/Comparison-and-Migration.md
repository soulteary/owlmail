# OwlMail × MailDev × MailCatcher: 전체 기능 및 API 비교 및 마이그레이션 화이트페이퍼

> **소스 코드 수준의 심층 비교 + 사용자 및 개발자를 위한 마이그레이션 가이드**

> ⚠️ **번역 진행 중**
> 
> 이 번역은 진행 중입니다. 현재는 [영어 버전](../en/Comparison-and-Migration.md) 또는 다른 사용 가능한 버전을 참조하세요:
> - [English](../en/Comparison-and-Migration.md)
> - [简体中文](../zh-CN/Comparison-and-Migration.md)
> 
> **기여를 환영합니다**: 이 번역에 기여하고 싶으시다면 [기여 가이드](../../.github/CONTRIBUTING.md)를 참조하세요.

---

## 📋 실행 요약

OwlMail, MailDev 및 MailCatcher는 기본 개발 워크플로를 공유하지만 **프로토콜이 동일하지
않으며 검증 없이 교체할 수 없습니다**. API 접두사, 응답 형식, 읽음 상태 및
실시간 프로토콜이 다릅니다. 현재 범위는
[API 참조](../en/API-Reference.md)를 확인하세요.

OwlMail 0.9.0은 기본적으로 비활성화된 읽기 전용 MCP 인터페이스를 Streamable HTTP와 stdio로 제공합니다. MailDev 3의 MCP 범위는 더 넓고 MailCatcher에는 내장 MCP가 없습니다. 기본적으로 비활성화된 `-maildev-rest-compat` 옵션은 현재 MailDev REST 경로를
`/api` 아래에 제공합니다. Socket.IO 호환성은 제공하지 않습니다.

> **참고**: 번역이 완료되면 전체 내용을 사용할 수 있습니다. 그동안 전체 세부사항은 영어 버전을 참조하세요.

---

## 기여 방법

이 문서의 번역을 돕고 싶으시다면:

1. 저장소 포크
2. 번역용 브랜치 생성
3. [Comparison-and-Migration.md](../en/Comparison-and-Migration.md)의 내용 번역
4. Pull Request 제출

자세한 내용은 [기여 가이드](../../.github/CONTRIBUTING.md)를 참조하세요.
