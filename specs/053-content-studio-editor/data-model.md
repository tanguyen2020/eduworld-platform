# Data Model: F053 — Workspace B — cây curriculum, editor nội dung, công thức, song ngữ, trình soạn câu hỏi

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F053 là client (`web/apps/portal`, `web/packages/ops-ui`), không tạo/sửa bảng hay migration. Bảng nội dung (`content_items`, `content_versions`, `media_assets`, `math_svgs`, curriculum) thuộc module learning (F020–F027). Dưới đây là **model phía client**.

### `EditorDraft<T>` (bộ nhớ, `ops-ui/editor`)

| Trường | Kiểu TS | Ghi chú |
|---|---|---|
| `contentType` | `LearningContentType` | `question` ở F053 |
| `contentId`, `version` | `string`, `number` | |
| `etag` | `string` | Gửi `If-Match` |
| `status` | `ContentStatus` | `draft`/`in_review`/`approved`/`published`/`archived`/`rejected` |
| `value` | `T` (form value theo dạng) | Map sang `QuestionAuthoringInput` bằng `mappers/<type>.ts` |
| `dirty`, `saveState` | `boolean`, `'idle'\|'saving'\|'saved'\|'error'\|'conflict'` | Thanh trạng thái |
| `lastSavedAt` | `Date` | "Đã lưu lúc HH:mm" |
| `mathCache` | `Map<math_id, { latex, svg, speech_text, display }>` | Chỉ render lại công thức mới |

### Form câu hỏi → payload (`mappers/*.ts`)

| Phần form | Trường payload | Ràng buộc |
|---|---|---|
| Đề (rich content, song ngữ) | `stem` | `question.schema.json#/$defs/richContent` |
| Phương án/mục (id `itemId`) | `render_payload` | không chứa đáp án, feedback, hints |
| Đáp án | `answer_key` | ID phải tồn tại trong `render_payload` |
| Chấm | `scoring_rule` (`all_or_nothing`/`partial`) | |
| Phản hồi phương án sai, `misconception_tag` | phần đáp án (`options[].feedback`) | chỉ `single_choice`, `multiple_choice` |
| Gợi ý | `hints[]` (≤ 5) | |
| Giải thích | `explanation` | |
| Công thức | `math_assets[]` | sau delta T005 |
| Metadata | `grade_levels` (mảng khối 1–9, chọn nhiều — CONTENT_MODEL §3, `QuestionAuthoringInput.grade_levels` sau delta F022; khác `grade_level` đơn của chủ đề curriculum), `subject_id`, `topic_id`, `objective_ids`, `difficulty`, `content_language`, `owner_scope`, `tenant_id` | |

### Cache truy vấn (mọi key có `scopeKey`)

| Query key | operationId |
|---|---|
| `['content-studio','question', scopeKey, questionId]` | `getCmsQuestion` |
| `['content-studio','subjects'\|'curricula'\|'topics'\|'objectives', scopeKey, filters]` | `listCmsSubjects` / `listCmsCurricula` / `listCmsTopics` / `listCmsObjectives` |
| `['content-studio','media', scopeKey, filters]`, `['content-studio','media-asset', scopeKey, assetId]` | `listMediaAssets`, `getMediaAsset` (poll tới `ready`/`rejected`) |
| `['content-studio','translation-job', scopeKey, jobId]` | `getLearningTranslationJob` |
| `['content-studio','preview', scopeKey, contentId, version, lang]` | `createLearningPreviewSession` (mutation, kết quả giữ trong cache ngắn) |

Không lưu bản nháp vào `localStorage` (nội dung có đáp án); thay đổi chưa lưu chỉ nằm trong bộ nhớ và được `unsavedChangesGuard` (F051) bảo vệ.

## 2. Vòng đời / trạng thái

Theo [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md). Nút trên `ItemStatusBar` theo trạng thái:

| Trạng thái | Nút hiển thị (khi có quyền) |
|---|---|
| `draft`, `rejected` | Lưu, Gửi duyệt |
| `in_review` | Duyệt, Từ chối (ẩn với tác giả) |
| `approved` | Xuất bản ngay (hẹn giờ: F054) |
| `published` | Tạo bản nháp từ v{n} |

## 3. Quy tắc kiểm tra dữ liệu

- ajv với `question.schema.json`; `maxLength` từ OpenAPI; giới hạn công thức theo `latex_max_length`, `latex_max_depth`, `math_svg_max`; số asset ≤ `assets_per_version_max`.
- Quy tắc nghiệp vụ cuối cùng (validator theo dạng, reviewer ≠ author, media `ready`, audio `g1_3`) do server.

## 4. Event / outbox

Không áp dụng — client không phát/tiêu thụ event (R25).
