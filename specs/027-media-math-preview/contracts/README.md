# Contracts: F027 — Media pipeline, công thức SVG, preview session

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/learning.v1.yaml` (gộp trong `specs/openapi/eduworld-api.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `presignMediaUpload` | `POST /api/v1/cms/media:presign-upload` | learning.v1 | `media.upload` | Có sẵn (`MediaPresignRequest`, `MediaPresignResponse`) |
| `completeMediaUpload` | `POST /api/v1/cms/media/{asset_id}:complete-upload` | learning.v1 | `media.upload` | Cần thêm |
| `listMediaAssets` | `GET /api/v1/cms/media` | learning.v1 | `learning_content.read` | Có sẵn |
| `getMediaAsset` | `GET /api/v1/cms/media/{asset_id}` | learning.v1 | `learning_content.read` | Có sẵn (`MediaAsset.used_in`) |
| `createLearningPreviewSession` | `POST /api/v1/cms/learning/preview/sessions` | learning.v1 | `learning_content.read` | Có sẵn (`LearningPreviewRequest`, `LearningPreviewSession`) |
| `submitPreviewResponse` | `POST /api/v1/cms/learning/preview/sessions/{preview_session_id}/responses` | learning.v1 | `learning_content.read` | Cần thêm |
| `updateQuestionDraft`, `updateCmsActivityDraft`, `updateCmsLessonDraft` … | `PATCH …` | learning.v1 | `learning_content.write` | Có sẵn — F027 thêm kiểm `MATH_RENDER_FAILED` qua pipeline validate |

ProblemCode: `MEDIA_TYPE_NOT_ALLOWED` (415), `MEDIA_TOO_LARGE` (413), `MEDIA_NOT_READY`, `MATH_RENDER_FAILED`, `VALIDATION_FAILED`, `PERMISSION_DENIED`, `NOT_FOUND`. Schema: `MediaKind`, `MediaStatus`, `MathAssetRef`, `ActivityRender`, `QuestionRender`.

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
| `media.uploaded` | module learning / media worker (learning) | `specs/events/payloads/learning/media.uploaded.v1.schema.json` | Cần thêm |
| `media.processed` | media worker / learning, NTF | `…/media.processed.v1.schema.json` | Cần thêm |
| `media.rejected` | media worker / learning, NTF | `…/media.rejected.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` (khối `math`, `media_ref`, `audio_ref`) | Tìm LaTeX/media trong nội dung câu hỏi | Có sẵn |
| `specs/schemas/activities/activity-base.schema.json` (`math_assets`, `media_refs`) | Activity | Có sẵn |

## 6. Delta cần thực hiện

- `completeMediaUpload`: request rỗng, header `Idempotency-Key`; response `MediaAsset`; lỗi `404`, `409` (đã complete khác trạng thái), `422` (task T004).
- `submitPreviewResponse`: request `{position, response: QuestionResponse}`; response `ActivityResponseResult` + `hint_levels[]`; `404` khi phiên hết hạn (task T004).
- `MediaPresignRequest`: ghi rõ `alt_text` bắt buộc khi `kind = image`; thêm `is_exam_material` (boolean) (task T004).
- Payload `media.uploaded/processed/rejected.v1` theo EVENT_DESIGN §4.2 (task T005).
- `license` `maxLength` 64 → 255 (D17 đã chốt; DATABASE_DESIGN §3.13, CONTENT_MODEL §7) (task T004).
