# Contracts: F028 — Import Word/MathType (`lessons_docx`) qua Pandoc

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (tách ra `specs/openapi/modules/learning.v1.yaml` bằng `scripts/openapi/split-public.mjs`).

| operationId | Method + path | Permission | Trạng thái |
|---|---|---|---|
| `createContentImport` | `POST /api/v1/cms/learning/imports` | `learning_content.import` | Có sẵn — `kind = lessons_docx` được hỗ trợ (không còn `IMPORT_KIND_NOT_SUPPORTED`); `ContentImportJob.result` thêm `items_total`, `items_importable` |
| `getContentImport` | `GET /api/v1/cms/learning/imports/{import_id}` | `learning_content.import` | Có sẵn |
| `listContentImportItems` | `GET /api/v1/cms/learning/imports/{import_id}/items` | `learning_content.import` | Mới — `status`, `limit`, `cursor`; `{data: ContentImportItem[], page: PageInfo}` |
| `createDraftFromImportItem` | `POST /api/v1/cms/learning/imports/{import_id}/items/{item_no}:create-draft` | `learning_content.import` | Mới — `Idempotency-Key` bắt buộc; body `ContentImportItemDraftRequest {math_assets[]}`; `201 ContentImportItem`; `404`, `409 CONFLICT`, `422 VALIDATION_FAILED`/`MATH_RENDER_FAILED` |
| `presignMediaUpload` | `POST /api/v1/cms/media:presign-upload` | `media.upload` | Có sẵn — `kind = document` nhận MIME `application/vnd.openxmlformats-officedocument.wordprocessingml.document` |

Schema mới: `ContentImportItem`, `ContentImportItemStatus`, `ContentImportItemIssue`, `ContentImportItemDraftRequest`. Mọi `string` có `maxLength` (R28). ProblemCode dùng lại: `CONFLICT` (sub-code `IMPORT_ITEM_NOT_IMPORTABLE`, `IMPORT_ITEM_ALREADY_CREATED`), `MATH_RENDER_FAILED`, `VALIDATION_FAILED`, `PERMISSION_DENIED`, `NOT_FOUND`. `x-portal-action`: `action.content_studio.question_bank.import`.

Mã vấn đề của câu (`ContentImportItemIssue.code`, `ContentImportJob.errors[].code`): `STEM_EMPTY`, `OPTIONS_TOO_FEW`, `OPTIONS_TOO_MANY`, `OPTION_ORDER`, `OPTION_EMPTY`, `ANSWER_MISSING`, `ANSWER_CONFLICT`, `ANSWER_UNKNOWN_OPTION`, `METADATA_MISSING`, `DIFFICULTY_INVALID`, `UNKNOWN_CODE`, `MATH_NOT_ALLOWED`, `FORMULA_NEEDS_REWRITE`, `FORMULAS_TOO_MANY`, `IMAGE_ALT_MISSING`, `IMAGE_TYPE_NOT_ALLOWED`, `IMAGE_NOT_FOUND`, `IMAGES_TOO_MANY`, `UNSUPPORTED_CONTENT`, `UNSUPPORTED_QUESTION_FORMAT`, `TEXT_TOO_LONG`, `HINTS_TOO_MANY` (lỗi); `EXTERNAL_LINK_REMOVED`, `FOOTNOTE_REMOVED`, `LIST_FLATTENED`, `TEXT_SCRIPT_CONVERTED`, `QUESTION_NUMBER_REPEATED` (cảnh báo); lỗi validator khi tạo Draft giữ mã của `createQuestion` với `source = validator`. Cấp file: `FILE_TOO_LARGE`, `INVALID_FORMAT`, `MACRO_NOT_ALLOWED`, `DOCX_TOO_COMPLEX`, `CONVERSION_TIMEOUT`, `CONVERTER_UNAVAILABLE`, `NO_QUESTIONS`, `TOO_MANY_QUESTIONS`, `ASSET_NOT_FOUND`.

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
| `media.uploaded` | module learning (ảnh nhúng) / media worker | `specs/events/payloads/learning/media.uploaded.v1.schema.json` | Có sẵn (F027) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` (`singleChoice`, `multipleChoice`, `richContent`, `mathDisplay`) | Nội dung Draft dựng từ Word | Có sẵn |

## 6. Delta cần thực hiện

- `ContentImportRequest.kind` mô tả: `lessons_docx` được hỗ trợ (F028), job chỉ phân tích (task T004).
- `ContentImportJob.result`: thêm `items_total`, `items_importable` (integer, optional — không breaking).
- Thêm `listContentImportItems`, `createDraftFromImportItem` và schema mới; chạy `split-public.mjs`, sinh lại `schema.gen.d.ts`/`operation-meta.gen.ts` của portal.
- Không có internal API, AsyncAPI, event mới.
