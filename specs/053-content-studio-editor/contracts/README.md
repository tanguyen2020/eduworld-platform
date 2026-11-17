# Contracts: F053 — Workspace B — cây curriculum, editor nội dung, công thức, song ngữ, trình soạn câu hỏi

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Nguồn: [`specs/openapi/eduworld-api.v1.yaml`](../../openapi/eduworld-api.v1.yaml) (`x-service: learning`), gateway public cho dữ liệu tenant, `X-Tenant-Id` theo `TenantIdHeaderOptional` (người dùng tenant bắt buộc; role platform vắng = phạm vi platform).

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createQuestion` | `POST /api/v1/cms/learning/questions` (Idempotency-Key) | `eduworld-api.v1.yaml` | `learning_content.write` | Cần sửa (`QuestionAuthoringInput`) |
| `getCmsQuestion` | `GET /api/v1/cms/learning/questions/{question_id}` | như trên | `learning_content.read` | Có sẵn |
| `updateQuestionDraft` | `PATCH /api/v1/cms/learning/questions/{question_id}/versions/{version}` (`If-Match`) | như trên | `learning_content.write` | Cần sửa (`QuestionAuthoringInput`) |
| `createQuestionVersion` | `POST /api/v1/cms/learning/questions/{question_id}/versions` (Idempotency-Key) | như trên | `learning_content.write` | Có sẵn |
| `transitionContentVersion` | `POST /api/v1/cms/learning/content/{content_type}/{content_id}/versions/{version}:transition` (`If-Match`, Idempotency-Key) | như trên | `learning_content.write` (submit_review), `learning_content.review` (approve/reject) | Có sẵn |
| `publishContentVersion` | `POST /api/v1/cms/learning/content/{content_type}/{content_id}/versions/{version}:publish` (`If-Match`, Idempotency-Key) | như trên | `learning_content.publish` | Có sẵn |
| `createLearningPreviewSession` | `POST /api/v1/cms/learning/preview/sessions` (Idempotency-Key) | như trên | `learning_content.read` | Có sẵn |
| `presignMediaUpload` | `POST /api/v1/cms/media:presign-upload` (Idempotency-Key) | như trên | `media.upload` | Có sẵn |
| `listMediaAssets` / `getMediaAsset` | `GET /api/v1/cms/media`, `/{asset_id}` | như trên | `learning_content.read` | Có sẵn |
| `listCmsLessons` / `getCmsLesson` | `GET /api/v1/cms/learning/lessons`, `/{lesson_id}` | như trên | `learning_content.read` | Có sẵn (chọn `worked_example_ref`) |
| `listCmsSubjects` / `createSubject` / `updateSubject` | `/api/v1/cms/learning/subjects[/{subject_id}]` | như trên | `curriculum.read` / `curriculum.write` | Có sẵn |
| `listCmsCurricula` / `createCurriculum` / `getCmsCurriculum` / `updateCurriculum` | `/api/v1/cms/learning/curricula[/{curriculum_id}]` | như trên | `curriculum.read` / `curriculum.write` | Có sẵn |
| `listCmsTopics` / `createTopic` / `updateTopic` | `/api/v1/cms/learning/topics[/{topic_id}]` | như trên | `curriculum.read` / `curriculum.write` | Có sẵn |
| `listCmsObjectives` / `createObjective` / `updateObjective` | `/api/v1/cms/learning/objectives[/{objective_id}]` | như trên | `curriculum.read` / `curriculum.write` | Có sẵn |
| `listCurriculumVersions` | `GET /api/v1/cms/learning/curricula/{curriculum_id}/versions` | như trên | `curriculum.read` | Cần thêm |
| `transitionCurriculumVersion` | `POST /api/v1/cms/learning/curricula/{curriculum_id}/versions/{version}:transition` (`If-Match`, Idempotency-Key) | như trên | `curriculum.write` / `curriculum.publish` | Cần thêm |
| `publishCurriculumVersion` | `POST /api/v1/cms/learning/curricula/{curriculum_id}/versions/{version}:publish` (Idempotency-Key) | như trên | `curriculum.publish` | Cần thêm |
| `exportLearningTranslations` / `importLearningTranslations` | `POST /api/v1/cms/learning/translations:export`, `:import` (Idempotency-Key) | như trên | `learning_content.translate` | Có sẵn |
| `getLearningTranslationJob` | `GET /api/v1/cms/learning/translations/jobs/{job_id}` | như trên | `learning_content.translate` | Có sẵn |

Mã lỗi xử lý: `VALIDATION_FAILED`, `CONTENT_VERSION_CONFLICT`, `VERSION_MISMATCH`, `CONTENT_NOT_EDITABLE`, `INVALID_STATE_TRANSITION`, `MEDIA_NOT_READY`, `MEDIA_TYPE_NOT_ALLOWED`, `MEDIA_TOO_LARGE`, `MATH_RENDER_FAILED`, `TRANSLATION_INCOMPLETE`, `AUDIO_REQUIRED` (Cần thêm), `RESOURCE_NOT_ASSIGNED`, `PERMISSION_DENIED`, `TENANT_HEADER_REQUIRED`, `TENANT_MISMATCH`, `IDEMPOTENCY_KEY_REUSED`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không áp dụng | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không áp dụng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không áp dụng (client); `content.*` do module learning phát | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| [`specs/schemas/question.schema.json`](../../schemas/question.schema.json) | Validation client (ajv), cấu trúc `stem`, `render_payload`, `answer_key`, `hints`, `mathDisplay`, `richContent`, `worked_example` | Có sẵn |

## 6. Delta cần thực hiện

Owner backend: F021 (curriculum), F022/F023 (question), F025 (song ngữ), F027 (công thức/media).

- **Cần sửa** `QuestionAuthoringInput`: thay `stem`/`render_payload`/`answer_key`/`scoring_rule`/`explanation` dạng object tự do bằng `$ref` tới `question.schema.json`; thêm `hints` (≤ 5 `richContent`), `worked_example_ref` (uuid + `block_id` ≤ 64), `audio_exempt_reason` (≤ 255), `math_assets[]` (cùng shape `ActivityDefinitionInput.math_assets`). Task T005.
- **Cần sửa** `LessonAuthoringInput`: thêm `math_assets[]` (F054 dùng, khai báo cùng đợt). Task T006.
- **Cần thêm** `AUDIO_REQUIRED` vào enum `ProblemCode` (đang dùng trong [CONTENT_MODEL §8.1](../../../docs/02-learning/CONTENT_MODEL.md) và TC-CNT-010). Task T007.
- **Cần thêm** `listCurriculumVersions`, `transitionCurriculumVersion`, `publishCurriculumVersion` (schema `CurriculumVersion { version, status: ContentStatus, effective_from?, effective_to?, missing_yccd_objective_ids[] (maxItems 500) }`). Task T008.
- **Cần sửa** `ContentVersion`: thêm `outdated_fields[]` (string ≤ 256, maxItems 500, json-pointer) để hiện "Lỗi thời" theo trường. Task T009.
