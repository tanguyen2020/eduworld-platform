# Data Model: F054 — Workspace B — media, bộ câu hỏi/lesson/blueprint, version và rollback, hàng đợi duyệt, preview, import, thùng rác

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F054 là client (`web/apps/portal`), không tạo/sửa bảng hay migration. `content_items`, `content_versions`, `content_releases`, `media_assets`, review task, import job thuộc module learning (F024–F027); Thùng rác đọc bản ghi soft-delete qua API. Dưới đây là **model phía client**.

### Cache truy vấn (mọi key có `scopeKey`; System Admin thêm `tenantFilter`)

| Query key | operationId |
|---|---|
| `['content-studio','versions', scopeKey, type, id, status]` | `listContentVersions` |
| `['content-studio','releases', scopeKey, type, id]` | `listContentReleases` |
| `['content-studio','diff', scopeKey, type, id, v, with]` | `diffContentVersions` |
| `['content-studio','usages', scopeKey, type, id]` | `listContentUsages` |
| `['content-studio','review-tasks', scopeKey, lane, type, mine]` | `listContentReviewTasks` |
| `['content-studio','media', scopeKey, tenantFilter, kind, status, q]` | `listMediaAssets` |
| `['content-studio','list', scopeKey, tenantFilter, type, filters]` | `searchQuestions` / `listCmsQuestionSets` / `listCmsLessons` / `listCmsAssessmentBlueprints` |
| `['content-studio','question-set'\|'lesson'\|'blueprint', scopeKey, id]` | `getCmsQuestionSet` / `getCmsLesson` / `getCmsAssessmentBlueprint` |
| `['content-studio','import', scopeKey, importId]` | `getContentImport` (poll tới `succeeded`/`failed`) |
| `['content-studio','trash', scopeKey, resourceType]` | `listTenantTrash` |

Mọi mutation phát hành → invalidate `versions`, `releases`, `list`, `review-tasks` của item.

### Trạng thái cục bộ

| Model | Trường | Ghi chú |
|---|---|---|
| `ImportSession` (bộ nhớ) | `file`, `kind: questions_xlsx\|questions_csv`, `rows[]`, `mathErrors[{row, field, latex, message}]`, `mathAssets[]`, `assetId`, `dryRunImportId`, `runImportId`, `phase: parsing\|rendering\|uploading\|dry_run\|review\|running\|done` | Không lưu file/nội dung ra storage |
| `TryOutState` (bộ nhớ) | `previewSessionId`, `itemIndex`, `hintsShown`, `lastResult` | Không gửi event, không lưu |
| `BulkApproveRun` (bộ nhớ) | `items[{contentType, contentId, version, idempotencyKey, result}]` | Một key mỗi item |
| Bộ lọc danh sách | `localStorage` `ew.portal.<userSub>.content_studio.filters.<screen>` | Per-viewer, try/catch |

## 2. Vòng đời / trạng thái

- Version/release theo [CONTENT_MODEL §5–§6](../../docs/02-learning/CONTENT_MODEL.md); lựa chọn rollback = version `published` ≠ con trỏ của cùng item; archive = version không phải con trỏ.
- `ContentImportJob.status`: `queued → running → succeeded | failed`; luồng UI: chạy thử (`dry_run = true`) → xem lỗi → chạy thật.

## 3. Quy tắc kiểm tra dữ liệu

- Hẹn giờ ≥ hiện tại + `scheduled_publish_min_lead`; lý do theo `maxLength` OpenAPI; công thức import theo `latex_max_length`/`latex_max_depth`/`math_svg_max`; loại/dung lượng media kiểm sơ bộ theo [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md) (server quyết định cuối).

## 4. Event / outbox

Không áp dụng — client không phát/tiêu thụ event (R25).
