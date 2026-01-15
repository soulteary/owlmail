# OwlMail 문서

OwlMail 문서 디렉토리에 오신 것을 환영합니다. 이 디렉토리에는 기술 문서, 마이그레이션 가이드 및 API 참조 자료가 포함되어 있습니다.

## 📸 미리보기

![OwlMail 미리보기](../../.github/assets/preview.png)

## 🎥 데모 비디오

![데모 비디오](../../.github/assets/realtime.gif)

## 📚 문서 구조

### 주요 문서

- **[OwlMail × MailDev - 전체 기능 및 API 비교 및 마이그레이션 백서](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.md)** (영어)
  - [中文版本](./OwlMail%20×%20MailDev%20-%20Full%20Feature%20&%20API%20Comparison%20and%20Migration%20White%20Paper.zh-CN.md)
  - OwlMail과 MailDev 간의 포괄적인 비교, API 호환성, 기능 패리티 및 마이그레이션 가이드를 포함합니다.

### 내부 문서

- **[API 리팩토링 기록](./internal/API_Refactoring_Record.md)** (영어)
  - [中文版本](./internal/API_Refactoring_Record.zh-CN.md)
  - MailDev 호환 엔드포인트에서 새로운 RESTful API 디자인(`/api/v1/`)으로의 API 리팩토링 프로세스를 문서화합니다.

## 🌍 다국어 지원

모든 문서는 명명 규칙을 따릅니다: `filename.md` (영어, 기본값) 및 다른 언어의 `filename.LANG.md`.

### 지원되는 언어

- **English** (`en`) - 기본값, 언어 코드 접미사 없음
- **简体中文** (`zh-CN`) - 중국어 (간체)
- **한국어** (`ko`) - 한국어

### 언어 코드 형식

언어 코드는 [ISO 639-1](https://en.wikipedia.org/wiki/ISO_639-1) 표준을 따릅니다:
- `zh-CN` - 중국어 (간체)
- `de` - 독일어 (향후)
- `fr` - 프랑스어 (향후)
- `it` - 이탈리아어 (향후)
- `ja` - 일본어 (향후)
- `ko` - 한국어

## 📖 문서 읽는 방법

1. **기본 언어**: 언어 코드 접미사가 없는 문서는 영어(기본값)입니다.
2. **다른 언어**: 언어 코드 접미사가 있는 문서(예: `.zh-CN.md`)는 번역본입니다.
3. **디렉토리 구조**: 문서는 주제별로 구성되며, 내부 문서는 `internal/` 하위 디렉토리에 있습니다.

## 🔄 기여하기

새 문서를 추가할 때:

1. 먼저 영어 버전을 만듭니다(기본값, 언어 코드 없음).
2. 적절한 언어 코드 접미사로 번역을 추가합니다.
3. 이 README를 업데이트하여 새 문서에 대한 링크를 포함합니다.
4. 기존 명명 규칙을 따릅니다.

## 📝 문서 카테고리

- **마이그레이션 가이드**: 사용자가 MailDev에서 OwlMail로 마이그레이션하는 데 도움
- **API 문서**: 기술 API 참조 및 리팩토링 기록
- **내부 문서**: 개발 노트 및 내부 프로세스

---

OwlMail에 대한 자세한 내용은 [메인 README](../README.ko.md)를 방문하세요.
