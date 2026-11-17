# Contracts: F024 — Question set, lesson, assessment blueprint, worked example, import hàng loạt, tìm kiếm

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (`x-service: learning`) → `specs/openapi/modules/learning.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listCmsQuestionSets` · `createCmsQuestionSet` | `GET/POST /api/v1/cms/learning/question-sets` | eduworld-api | `learning_content.read` / `.write` | Cần sửa (query `topic_id`, `objective_id`) |
| `getCmsQuestionSet` · `createCmsQuestionSetVersion` · `updateCmsQuestionSetDraft` | `/cms/learning/question-sets/{question_set_id}` (+ `/versions`, `/versions/{version}`) | eduworld-api | `.read` / `.write` | Có sẵn |
| `listCmsLessons` · `createCmsLesson` · `getCmsLesson` · `createCmsLessonVersion` · `updateCmsLessonDraft` | `/cms/learning/lessons*` | eduworld-api | `.read` / `.write` | Cần sửa (query `topic_id`, `objective_id`; schema khối `worked_example`) |
| `listCmsAssessmentBlueprints` · `createCmsAssessmentBlueprint` · `getCmsAssessmentBlueprint` · `createCmsAssessmentBlueprintVersion` · `updateCmsAssessmentBlueprintDraft` | `/cms/learning/assessment-blueprints*` | eduworld-api | `.read` / `.write` | Cần sửa (query `topic_id`, `objective_id`) |
| `listCmsLearningMaterials` · `createCmsLearningMaterial` · `getCmsLearningMaterial` · `createCmsLearningMaterialVersion` · `updateCmsLearningMaterialDraft` | `/cms/learning/learning-materials` (+ `/{learning_material_id}`, `/versions`, `/versions/{version}`) | eduworld-api | `.read` / `.write` | Cần thêm |
| `deleteCmsQuestionSet` · `deleteCmsLesson` · `deleteCmsLearningMaterial` · `deleteCmsAssessmentBlueprint` | `DELETE` trên path item tương ứng | eduworld-api | `.write` | Cần thêm |
| `getQuestionSetPoolStats` | `GET /cms/learning/question-sets/{question_set_id}/versions/{version}:pool-stats` | eduworld-api | `.read` | Cần thêm |
| `getAssessmentBlueprintPoolStats` | `GET /cms/learning/assessment-blueprints/{assessment_blueprint_id}/versions/{version}:pool-stats` | eduworld-api | `.read` | Cần thêm |
| `createContentImport` · `getContentImport` | `POST /cms/learning/imports`, `GET /cms/learning/imports/{import_id}` | eduworld-api | `learning_content.import` | Cần sửa (`kind`) |
| `searchQuestions` | `GET /cms/learning/questions` (`q`) | eduworld-api | `.read` | Có sẵn (`q` hiện thực ở F024) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listTrash`, `restoreTrashItem`, `executeTenantCloneStep`, `applyTenantAssignments` (`resource_type` `question_set`, `lesson`, `assessment_blueprint`) | identity → learning | `specs/openapi/modules/learning.internal.v1.yaml` | `identity` | Có sẵn / Cần thêm (F020) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `<resource_type>.deleted` / `.restored` | Phát | `resource.deleted.v1` / `resource.restored.v1` | Cần thêm (F020) |
| `audit.recorded` | Phát | envelope | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question-set.schema.json` | Validate `definition` của set | Có sẵn |
| `specs/schemas/assessment-blueprint.schema.json` | Validate `definition` của blueprint | Có sẵn |
| `specs/schemas/lesson.schema.json` (`$defs.block`, `$defs.workedExample`) | Validate khối lesson/learning material | Cần thêm |
| `specs/schemas/question.schema.json#/properties/worked_example_ref` | Tham chiếu ví dụ mẫu | Cần sửa |
| `specs/schemas/activities/interactive-exercise.schema.json` (`worked_example_ref`) | Đồng bộ shape | Cần sửa |
| `specs/schemas/import/questions-row.schema.json` | Một dòng CSV/Excel sau khi ánh xạ cột | Cần thêm |

## 6. Delta cần thực hiện

- Thêm `lesson.schema.json`: `block` oneOf `rich_text` (richContent), `media` (`media_ref`), `worked_example` (`id` pattern `^we_[a-z0-9_]{1,40}$`, `objective_ids` 1–20, `problem` richContent, `steps` 1–8 `{content, audio_ref?}`, `self_explain_prompt?`), `activity` (`activity_id`, `required`); `LessonAuthoringInput.blocks.items` `$ref` tới đó.
- Sửa `worked_example_ref` (question, interactive-exercise): object `{content_item_id uuid (lesson hoặc learning material), block_id string ≤ 64 pattern}` (QUESTION_BANK §2, RB-203).
- Thêm schema OpenAPI `LearningMaterialAuthoringInput`/`LearningMaterialAuthoring` (như lesson, thêm `material_kind`), `PoolStats { rules[] {section_id?, rule_index, count, required, available, insufficient, warning} }` và các operation "Cần thêm" ở §1 (`TenantIdHeaderOptional`, `IdempotencyKey` cho POST, `IfMatch` cho PATCH/DELETE).
- `ContentImportRequest.kind`: thêm `questions_qti_lite`, `content_json`; ghi chú `lessons_docx` chưa hỗ trợ (`IMPORT_KIND_NOT_SUPPORTED` trong `errors[].code`).
- List endpoint question set/lesson/blueprint/learning material: thêm query `topic_id`, `objective_id`.
- Thêm `specs/schemas/import/questions-row.schema.json` và mẫu `specs/examples/import/questions.template.csv`.
