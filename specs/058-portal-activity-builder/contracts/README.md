# Contracts: F058 — Workspace B — Learning Activity Builder (quiz, interactive_exercise, video_lesson), UI re-evaluate

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (gộp trong `specs/openapi/eduworld-api.v1.yaml`). Dữ liệu tenant qua gateway public; dữ liệu platform qua gateway nội bộ.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listActivityTypes` | `GET /api/v1/cms/learning/activity-types` | learning.v1 | `learning_content.read` | Có sẵn (`ActivityTypeInfo`) |
| `listCmsActivities` · `getCmsActivity` | `GET /api/v1/cms/learning/activities[/{activity_id}]` | learning.v1 | `learning_content.read` | Có sẵn |
| `createCmsActivity` | `POST /api/v1/cms/learning/activities` | learning.v1 | `learning_content.activity.write` | Có sẵn (`ActivityDefinitionInput`; `ACTIVITY_CONFIG_INVALID`, `MATH_RENDER_FAILED`) |
| `updateCmsActivityDraft` | `PATCH /api/v1/cms/learning/activities/{activity_id}` | learning.v1 | `learning_content.activity.write` | Có sẵn (`If-Match`; `CONTENT_NOT_EDITABLE`) |
| `createCmsActivityVersion` | `POST /api/v1/cms/learning/activities/{activity_id}/versions` | learning.v1 | `learning_content.activity.write` | Cần thêm |
| `searchQuestions` | `GET /api/v1/cms/learning/questions` | learning.v1 | `learning_content.read` | Có sẵn (`status=published`, `type`, `source`) |
| `listCmsQuestionSets` | `GET /api/v1/cms/learning/question-sets` | learning.v1 | `learning_content.read` | Có sẵn |
| `listMediaAssets` · `getMediaAsset` · `presignMediaUpload` | `GET /api/v1/cms/media` · `GET …/{asset_id}` · `POST /api/v1/cms/media:presign-upload` | learning.v1 | `learning_content.read` / `media.upload` | Có sẵn; `MediaKind` Cần sửa (phụ đề) |
| `createLearningPreviewSession` | `POST /api/v1/cms/learning/preview/sessions` | learning.v1 | `learning_content.read` | Có sẵn (`content_type = activity`) |
| `transitionContentVersion` | `POST /api/v1/cms/learning/content/{content_type}/{content_id}/versions/{version}:transition` | learning.v1 | `learning_content.activity.write` (submit) / `learning_content.activity.review` (approve, reject) | Có sẵn |
| `publishContentVersion` | `POST …/versions/{version}:publish` | learning.v1 | `learning_content.publish` | Có sẵn |
| `listContentVersions` | `GET …/{content_type}/{content_id}/versions` | learning.v1 | `learning_content.read` | Có sẵn (chọn `corrected_version_id`) |
| `createRegradeRequest` | `POST /api/v1/cms/learning/regrade-requests` | learning.v1 | `learning_content.re_evaluate` | Có sẵn (`RegradeRequestCreate` → `202 RegradeRequest`) |
| `previewRegradeRequest` | `POST /api/v1/cms/learning/regrade-requests:preview` | learning.v1 | `learning_content.re_evaluate` | Cần thêm |
| `getRegradeRequest` | `GET /api/v1/cms/learning/regrade-requests/{regrade_request_id}` | learning.v1 | `learning_content.re_evaluate` | Cần thêm (đã ghi ở F031) |
| `listRegradeRequests` | `GET /api/v1/cms/learning/regrade-requests` | learning.v1 | `learning_content.re_evaluate` | Cần thêm |
| `getMyPermissions` | `GET /api/v1/me/permissions` | eduworld-api | đã đăng nhập | Có sẵn |

ProblemCode hiển thị: `ACTIVITY_TYPE_NOT_SUPPORTED`, `ACTIVITY_CONFIG_INVALID`, `MATH_RENDER_FAILED`, `CONTENT_NOT_EDITABLE`, `CONTENT_VERSION_CONFLICT`, `MEDIA_NOT_READY`, `VIDEO_NOT_READY`, `RESOURCE_NOT_ASSIGNED`, `PERMISSION_DENIED`, `VALIDATION_FAILED`, `IDEMPOTENCY_KEY_REUSED`, `EVALUATION_NOT_EDITABLE`; `AUDIO_REQUIRED` (delta của F053).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (frontend) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Portal không phát/tiêu thụ; `content.published`, `attempt.re_evaluated` do module learning phát (F026, F031) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/activities/activity-base.schema.json` | Widget metadata, `questionSource`, `evaluation`, `feedback`, `completionRule` | Có sẵn |
| `specs/schemas/activities/quiz.schema.json` | Form `quiz` | Có sẵn |
| `specs/schemas/activities/interactive-exercise.schema.json` | Form `interactive_exercise` | Có sẵn |
| `specs/schemas/activities/video-lesson.schema.json` | Form `video_lesson` | Có sẵn |
| `specs/schemas/common/enums.schema.json` | `ActivityType`, `FeedbackTiming` | Có sẵn |

## 6. Delta cần thực hiện

Phase 2 task T005–T008. Không sửa contract trong đợt viết spec.

- **Cần thêm** (owner F030) `createCmsActivityVersion` `POST /cms/learning/activities/{activity_id}/versions`: body `NewVersionRequest` (như `createQuestionVersion`), `Idempotency-Key`, `TenantIdHeaderOptional` → `201 ActivityDefinition` + `ETag`; `403`, `404`, `422`.
- **Cần thêm** (owner F031) `previewRegradeRequest` `POST /cms/learning/regrade-requests:preview`: body `RegradeRequestCreate` (cho phép thiếu `reason`) → `200 RegradeImpact` {`affected_attempts` integer ≥ 0, `affected_tenant_count` integer ≥ 0, `affected_assignment_count` integer ≥ 0}; không ghi dữ liệu, không audit nội dung; `403 PERMISSION_DENIED`.
- **Cần thêm** (owner F031) `getRegradeRequest` `GET /cms/learning/regrade-requests/{regrade_request_id}` → `RegradeRequest` (đã có trong delta F031) và `listRegradeRequests` `GET /cms/learning/regrade-requests` lọc `question_version_id`, `activity_version_id`, `status`, `Limit`, `Cursor` → `{data: RegradeRequest[], page: PageInfo}`; thêm `question_version_id`, `activity_version_id`, `corrected_version_id` vào `RegradeRequest`.
- **Cần sửa** (owner F027) `MediaKind` thêm `subtitle` (WebVTT `text/vtt`, kích thước tối đa theo media pipeline) để upload phụ đề cho `video-lesson.schema.json` `subtitles[].subtitle_asset_id`; cập nhật CONTENT_MODEL §7.
- **Tài liệu** ADMIN_PORTAL §3.13: permission Learning Activity Builder/Video Manager sửa thành `learning_content.activity.write` cho khớp ACTIVITY_ENGINE §13 và OpenAPI.
