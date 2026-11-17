# Contracts: F025 — Workflow duyệt nội dung, song ngữ, lồng tiếng `g1_3`

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/learning.v1.yaml` (gộp trong `specs/openapi/eduworld-api.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `transitionContentVersion` | `POST /api/v1/cms/learning/content/{content_type}/{content_id}/versions/{version}:transition` | learning.v1 | `learning_content.write` (submit_review, revise) · `learning_content.review` (approve, reject); activity: `learning_content.activity.write/review` | Cần sửa (action `revise`, field `audio_exemption_acknowledged`, response 409/428 đủ mã) |
| `listContentReviewTasks` | `GET /api/v1/cms/learning/review-tasks` | learning.v1 | `learning_content.review` | Cần sửa (enum `lane` → `tenant`, `platform_learning`) |
| `exportLearningTranslations` | `POST /api/v1/cms/learning/translations:export` | learning.v1 | `learning_content.translate` | Có sẵn |
| `importLearningTranslations` | `POST /api/v1/cms/learning/translations:import` | learning.v1 | `learning_content.translate` | Có sẵn |
| `getLearningTranslationJob` | `GET /api/v1/cms/learning/translations/jobs/{job_id}` | learning.v1 | `learning_content.translate` | Có sẵn |
| `updateQuestionDraft` (và `update*Draft` khác) | `PATCH …/versions/{version}` | learning.v1 | `learning_content.write` | Có sẵn — trả `translation_status` (F022/F024 sở hữu endpoint, F025 thêm tính toán) |

Schema dùng: `ContentVersion` (`translation_status`, `content_language`), `ContentReviewTask`, `TranslationExportRequest`, `TranslationImportRequest`, `TranslationJob`, `Problem` (`ProblemCode`: `VALIDATION_FAILED`, `MEDIA_NOT_READY`, `INVALID_STATE_TRANSITION`, `TRANSLATION_INCOMPLETE`, `PERMISSION_DENIED`, `VERSION_MISMATCH`, `PRECONDITION_REQUIRED`, `IDEMPOTENCY_KEY_REUSED`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `content.version_submitted` | learning phát / NTF (sau) | `specs/events/payloads/learning/content.version_submitted.v1.schema.json` | Cần thêm |
| `content.version_approved` | learning phát / NTF, AUD | `…/content.version_approved.v1.schema.json` | Cần thêm |
| `content.version_rejected` | learning phát / NTF, AUD | `…/content.version_rejected.v1.schema.json` | Cần thêm |
| `audit.recorded` | learning phát / identity | `specs/events/envelope.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` (`stem.audio_ref`, `i18nText`) | Rule audio/ngôn ngữ cho câu hỏi | Có sẵn — Cần sửa: thêm `audio_exempt_reason` (`maxLength 255`) nếu chưa có |
| `specs/schemas/activities/activity-base.schema.json` (`instructions_audio_ref`) | Rule audio cho activity | Có sẵn — kiểm `instructions_audio_ref`, `audio_exempt_reason` |
| `specs/schemas/common/enums.schema.json` (`LearningContentType`) | `content_type` | Có sẵn |

## 6. Delta cần thực hiện

- `transitionContentVersion`: thêm `revise` vào enum `action`; thêm `audio_exemption_acknowledged: boolean`; liệt kê `409 IDEMPOTENCY_KEY_REUSED`, `428 PRECONDITION_REQUIRED`, `422` với `AUDIO_REQUIRED`/`TRANSLATION_INCOMPLETE`/`MEDIA_NOT_READY` (task T004).
- `ContentReviewTask.lane` và query `lane`: đổi enum thành `tenant`, `platform_learning` (D8) (task T004).
- `ProblemCode` (learning.v1 + eduworld-api.v1 + bảng API_DESIGN §4.1): thêm `AUDIO_REQUIRED` (422) (task T004).
- Event payload: `content.version_submitted/approved/rejected.v1.schema.json` (task T005).
- `question.schema.json` / `activity-base.schema.json`: `audio_exempt_reason` (`maxLength 255`) nếu chưa có (task T005).
