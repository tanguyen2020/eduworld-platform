# Contracts: F023 — Question Bank nâng cao: short_answer, image_based, interactive, câu tham số hóa

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createQuestion`, `updateQuestionDraft`, `createQuestionVersion`, `getCmsQuestion`, `getCmsQuestionVersion`, `searchQuestions`, `deleteQuestion` | `/api/v1/cms/learning/questions*` (F022) | `specs/openapi/eduworld-api.v1.yaml` | `learning_content.read/write` | Có sẵn / Cần thêm (F022) |
| `listQuestionWidgets` | `GET /api/v1/cms/learning/question-widgets` | như trên | `learning_content.read` | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có operation facade mới | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `AnswerResponse` (oneOf theo loại, gồm `shortAnswer`, `imageBased`, `interactive`) | client → server (dùng ở F030/F105) | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không có | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json#/$defs/shortAnswer`, `textMatch`, `shortAnswerResponse` | Validate/chấm `short_answer` | Có sẵn |
| `specs/schemas/question.schema.json#/$defs/imageBased`, `imageBasedResponse` | Validate/chấm `image_based` | Cần sửa (giới hạn mảng) |
| `specs/schemas/question.schema.json#/$defs/interactive`, `interactiveResponse` | Validate/chấm `interactive` | Có sẵn |
| `specs/schemas/question.schema.json#/$defs/params` | Câu tham số hóa | Cần sửa |
| `specs/schemas/question-widgets/balance_scale.v1.schema.json` | `config` + `answer_key.params` của widget `balance_scale@1.0` | Cần thêm |

## 6. Delta cần thực hiện

- `question.schema.json`: `textMatch.accepted` thêm `maxItems: 50`; `imageBased.render_payload.labels` và `drop_points` `maxItems: 16`; `answer_key.regions` `maxItems: 20`, `regions[].points` `minItems: 3`, `maxItems: 64`; `params` thêm `maxProperties: 11` (10 biến + `constraints`), `constraints.maxItems: 10`, `values.maxItems: 20`, `propertyNames.pattern` `^([a-z][a-z0-9_]{0,15}|constraints)$`; `numeric.answer_key.value` (biểu thức) `maxLength: 255`.
- Thêm thư mục `specs/schemas/question-widgets/` với `balance_scale.v1.schema.json` (`config {left_weights[] ≤ 10, available_weights[] ≤ 10, max_moves 1–50}`, `params {target}`) và cập nhật `specs/README.md` bảng JSON Schema.
- Thêm operation `listQuestionWidgets` → schema `QuestionWidgetList { data[] {widget (≤ 64), widget_version (≤ 64), config_schema (object), validators[] (≤ 64)} }`, lỗi 401/403.
