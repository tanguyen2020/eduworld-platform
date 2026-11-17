# Contracts: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `searchQuestions` | `GET /api/v1/cms/learning/questions` | `specs/openapi/eduworld-api.v1.yaml` | `learning_content.read` | Cần sửa (thêm query `topic_id`, `objective_id`, `difficulty`) |
| `createQuestion` | `POST /api/v1/cms/learning/questions` | như trên | `learning_content.write` | Cần sửa (`QuestionAuthoringInput`) |
| `getCmsQuestion` | `GET /api/v1/cms/learning/questions/{question_id}` | như trên | `learning_content.read` | Có sẵn |
| `getCmsQuestionVersion` | `GET /api/v1/cms/learning/questions/{question_id}/versions/{version}` | như trên | `learning_content.read` | Cần thêm |
| `updateQuestionDraft` | `PATCH /api/v1/cms/learning/questions/{question_id}/versions/{version}` | như trên | `learning_content.write` | Có sẵn (schema theo delta) |
| `createQuestionVersion` | `POST /api/v1/cms/learning/questions/{question_id}/versions` | như trên | `learning_content.write` | Có sẵn |
| `deleteQuestion` | `DELETE /api/v1/cms/learning/questions/{question_id}` | như trên | `learning_content.write` | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listTrash`, `restoreTrashItem`, `executeTenantCloneStep`, `applyTenantAssignments` (`resource_type = question`) | identity → learning | `specs/openapi/modules/learning.internal.v1.yaml` | `identity` | Có sẵn / Cần thêm (F020) |
| `createQuestionPlan`, `scorePlanAnswer` | competition → learning (dùng port `Delivery` của F022; operation facade làm ở F105) | như trên | `competition` | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng trực tiếp (`AnswerResponse` của AsyncAPI dùng cùng shape response) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `content.published` | Tiêu thụ (xóa cache pool) | `specs/events/payloads/learning/content.published.v1.schema.json` | Có sẵn |
| `content.rolled_back`, `content.unpublished` | Tiêu thụ | `specs/events/payloads/learning/content.rolled_back.v1.schema.json`, `content.unpublished.v1.schema.json` | Cần thêm (owner F026) |
| `tenant_data.assigned` / `tenant_data.unassigned` | Tiêu thụ | payload identity | Cần thêm (owner F045) |
| `question.deleted` / `question.restored` | Phát | `resource.deleted.v1` / `resource.restored.v1` | Cần thêm (F020) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` (`$defs` 8 loại, `option`, `scoringRule`, `textMatch`, `richContent`, `mathDisplay`) | Validate dạng soạn | Có sẵn |
| `specs/schemas/question.schema.json#/$defs/response` + `<type>Response` | Validate response trước khi chấm | Có sẵn |
| `specs/schemas/question-set.schema.json#/$defs/selectionRule` | Đầu vào `Select` | Có sẵn |
| `specs/schemas/common/enums.schema.json#/$defs/QuestionType` | Registry loại | Có sẵn |

## 6. Delta cần thực hiện

- `QuestionAuthoringInput`: đổi `grade_level` → `grade_levels` (mảng `GradeLevel`, 1–9 phần tử, uniqueItems); thêm `curriculum_version_id`, `hints` (≤ 5, richContent), `worked_example_ref`, `min_response_ms` (0–600000), `max_score` (> 0), `exam_only`, `math_assets` (≤ 50, `$ref` `mathDisplay`), `tags` (≤ 20, chuỗi ≤ 50); `stem`/`explanation` `$ref` `richContent`; `answer_key`, `render_payload` mô tả rõ "validate theo `question.schema.json` theo `type`" và giới hạn `maxProperties`.
- Thêm operation `getCmsQuestionVersion` (`GET …/versions/{version}`, `ETag`, trả `QuestionAuthoring`, 403/404).
- `searchQuestions`: thêm query `topic_id` (uuid), `objective_id` (uuid), `difficulty` (integer 1–5, lặp được).
- Cập nhật DATABASE_DESIGN §3.3 (cột thêm, `grade_levels`, `topic_id NOT NULL`, view delivery) — task Polish.
