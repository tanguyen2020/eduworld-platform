# Contracts: F054 — Workspace B — media, bộ câu hỏi/lesson/blueprint, version và rollback, hàng đợi duyệt, preview, import, thùng rác

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Nguồn: [`specs/openapi/eduworld-api.v1.yaml`](../../openapi/eduworld-api.v1.yaml) (`x-service: learning`; Thùng rác `x-service: identity`). `X-Tenant-Id` theo `TenantIdHeaderOptional`/`TenantIdHeader`.

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listContentVersions` | `GET /api/v1/cms/learning/content/{content_type}/{content_id}/versions` | `eduworld-api.v1.yaml` | `learning_content.read` | Có sẵn |
| `diffContentVersions` | `GET …/versions/{version}:diff?with=` | như trên | `learning_content.read` (`answer_key`: `learning_content.review`) | Có sẵn |
| `listContentReleases` | `GET …/{content_id}/releases` | như trên | `learning_content.read` | Có sẵn |
| `publishContentVersion` | `POST …/versions/{version}:publish` (`publish_at`, `If-Match`, Idempotency-Key) | như trên | `learning_content.publish` | Có sẵn |
| `cancelScheduledPublish` | `POST …/versions/{version}:cancel-schedule` (Idempotency-Key) | như trên | `learning_content.publish` | Cần thêm |
| `rollbackContent` | `POST …/{content_id}:rollback` (Idempotency-Key) | như trên | `content.release.rollback` | Có sẵn |
| `unpublishContent` | `POST …/{content_id}:unpublish` (Idempotency-Key) | như trên | `learning_content.publish` | Có sẵn |
| `transitionContentVersion` | `POST …/versions/{version}:transition` (`archive`, `approve`, `reject`) | như trên | `learning_content.write` / `learning_content.review` | Cần sửa (`field_comments`) |
| `listContentReviewTasks` | `GET /api/v1/cms/learning/review-tasks` | như trên | `learning_content.review` | Cần sửa (enum `lane`) |
| `createLearningPreviewSession` | `POST /api/v1/cms/learning/preview/sessions` | như trên | `learning_content.read` | Có sẵn |
| `submitLearningPreviewResponse` | `POST /api/v1/cms/learning/preview/sessions/{preview_session_id}/responses` | như trên | `learning_content.read` | Cần thêm |
| `listContentUsages` | `GET /api/v1/cms/learning/content/{content_type}/{content_id}/usages` | như trên | `learning_content.read` | Cần thêm |
| `listMediaAssets` / `getMediaAsset` / `presignMediaUpload` | `/api/v1/cms/media…` | như trên | `learning_content.read` / `media.upload` | Có sẵn |
| `searchQuestions` | `GET /api/v1/cms/learning/questions` | như trên | `learning_content.read` | Cần sửa (bộ lọc) |
| `listCmsQuestionSets` / `createCmsQuestionSet` / `getCmsQuestionSet` / `createCmsQuestionSetVersion` / `updateCmsQuestionSetDraft` | `/api/v1/cms/learning/question-sets…` | như trên | `learning_content.read` / `learning_content.write` | Có sẵn (Cần sửa `pool_sizes`) |
| `listCmsLessons` / `createCmsLesson` / `getCmsLesson` / `createCmsLessonVersion` / `updateCmsLessonDraft` | `/api/v1/cms/learning/lessons…` | như trên | như trên | Có sẵn (`math_assets` ở F053 T006) |
| `listCmsAssessmentBlueprints` / `createCmsAssessmentBlueprint` / `getCmsAssessmentBlueprint` / `createCmsAssessmentBlueprintVersion` / `updateCmsAssessmentBlueprintDraft` | `/api/v1/cms/learning/assessment-blueprints…` | như trên | như trên | Có sẵn |
| `deleteQuestion` | `DELETE /api/v1/cms/learning/questions/{question_id}` (`If-Match`) | như trên | `learning_content.write` | Có sẵn |
| `deleteContent` | `DELETE /api/v1/cms/learning/content/{content_type}/{content_id}` (`If-Match`, `reason`) | như trên | `learning_content.write` | Cần thêm |
| `createContentImport` / `getContentImport` | `POST /api/v1/cms/learning/imports`, `GET …/{import_id}` | như trên | `learning_content.import` | Có sẵn (Cần sửa `math_assets`) |
| `listTenantTrash` / `restoreTenantTrashItem` | `GET /api/v1/tenant/trash`, `POST /api/v1/tenant/trash/{resource_type}/{resource_id}:restore` | như trên | `tenant.trash.read` / `tenant.trash.restore` | Có sẵn |

Mã lỗi xử lý: `CONTENT_VERSION_CONFLICT`, `CONTENT_NOT_EDITABLE`, `CONTENT_NOT_PUBLISHED`, `ROLLBACK_TARGET_INVALID`, `INVALID_STATE_TRANSITION`, `MEDIA_NOT_READY`, `MEDIA_TYPE_NOT_ALLOWED`, `MEDIA_TOO_LARGE`, `MATH_RENDER_FAILED`, `TRANSLATION_INCOMPLETE`, `QUESTION_POOL_INSUFFICIENT`, `CONFLICT`, `PERMISSION_DENIED`, `VALIDATION_FAILED`, `IDEMPOTENCY_KEY_REUSED`, `IDEMPOTENCY_IN_PROGRESS`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không áp dụng (nơi sử dụng do module learning dựng từ event, F026) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không áp dụng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không áp dụng (client) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| [`specs/schemas/question-set.schema.json`](../../schemas/question-set.schema.json) | Question Set editor (tĩnh/luật chọn) | Có sẵn |
| [`specs/schemas/assessment-blueprint.schema.json`](../../schemas/assessment-blueprint.schema.json) | Blueprint editor | Có sẵn |
| [`specs/schemas/question.schema.json`](../../schemas/question.schema.json) | `richContent`, `worked_example` trong lesson | Có sẵn |

## 6. Delta cần thực hiện

Owner backend: F024 (question set/lesson/blueprint, import, tìm kiếm), F025 (duyệt), F026 (phát hành, nơi sử dụng), F027 (preview).

- **Cần thêm** `listContentUsages` — item `{ consumer_type: question_set|lesson|assessment_blueprint|activity|learning_path|assignment|exam|quest|minigame_config|challenge, consumer_id, consumer_version?, title (≤255), owner_service: learning|game|competition, tenant_id? }`, phân trang cursor. Task T005.
- **Cần thêm** `deleteContent` (soft-delete theo `content_type`, `If-Match`, `ReasonQuery`; published → reason bắt buộc). Task T006.
- **Cần thêm** `submitLearningPreviewResponse` — body `{ position, response: QuestionResponse, hints_requested? }`, trả `ActivityResponseResult` (feedback, explanation, hint kế tiếp); không lưu attempt, không phát event. Task T007.
- **Cần thêm** `cancelScheduledPublish` (chỉ khi `publish_at` chưa tới; audit `content.publish_schedule_cancelled`). Task T008.
- **Cần sửa** `ContentReviewTask.lane` và tham số `lane` của `listContentReviewTasks`: `tenant | platform_learning` theo [ADMIN_PORTAL §3.5](../../../docs/04-admin-portal/ADMIN_PORTAL.md) (D8). Body `transitionContentVersion` thêm `field_comments[] { path (≤256), comment (≤1000) }` (maxItems 100). Task T009.
- **Cần sửa** `searchQuestions` thêm `topic_id`, `objective_id`, `difficulty`, `translation_status`; `listCmsQuestionSets`/`listCmsLessons`/`listCmsAssessmentBlueprints` thêm `topic_id`, `objective_id` (FR-CNT-021). Task T010.
- **Cần sửa** response `getCmsQuestionSet` thêm `pool_sizes[] { rule_index, available_count }` (readOnly). Task T011.
- **Cần sửa** `ContentImportRequest` thêm `math_assets[]` (shape `ActivityDefinitionInput.math_assets`, maxItems theo `request_body_max`) để gửi công thức đã render trên trình duyệt (ADR-0055). Task T012.
