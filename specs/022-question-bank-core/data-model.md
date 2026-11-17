# Data Model: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### Question — `content.questions` (Có sẵn, DATABASE_DESIGN §2.5, §3.13) · loại H · mixin `ContentItemMixin` (F020)

`question_id` PK, `owner_scope`, `tenant_id`, `current_version_id`, `latest_version_no`, `created_by`, `cloned_*`, `deleted_*`. Index `(owner_scope, tenant_id)`; trigger `questions_release_target` (DATABASE_DESIGN §3.13).

### QuestionVersion — `content.question_versions` (Có sẵn — Thêm cột) · loại H · mixin `ContentVersionMixin` (F020)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `question_version_id` | UUIDv7 | PK | |
| `question_id` | uuid | FK; UNIQUE `(question_id, version)` | |
| `type` | `varchar(64)` | CHECK 11 giá trị `QuestionType` | |
| `stem` | jsonb | NOT NULL, CHECK `octet_length ≤ 65536` | richContent |
| `render_payload` | jsonb | NOT NULL, CHECK `≤ 65536` | **không** chứa `feedback`/`misconception_tag` (tách khi lưu) |
| `answer_key` | jsonb | NOT NULL, CHECK `≤ 65536` | chỉ server |
| `scoring_rule` | jsonb | NOT NULL, CHECK `≤ 4096` | |
| `explanation` | jsonb NULL | CHECK `≤ 65536` | |
| `option_feedback` | jsonb NULL | CHECK `≤ 65536` | Thêm cột: `{<option_id>: {feedback, misconception_tag}}` |
| `hints` | jsonb NULL | CHECK `jsonb_array_length ≤ 5`, `≤ 32768` | Thêm cột |
| `worked_example_ref` | jsonb NULL | CHECK `≤ 1024` | Thêm cột |
| `min_response_ms` | int NULL | CHECK 0–600000 | Thêm cột |
| `max_score` | numeric(7,2) | NOT NULL DEFAULT 1, CHECK > 0 | Thêm cột |
| `exam_only` | bool | NOT NULL DEFAULT false | Thêm cột |
| `math_assets` | jsonb NULL | CHECK `jsonb_array_length ≤ 50`, `≤ 65536` | Thêm cột (ADR-0055) |
| `tags` | `varchar(64)[]` | CHECK `cardinality ≤ 20` | Thêm cột |
| `difficulty` | smallint | CHECK 1–5 | |
| `grade_levels` | smallint[] | CHECK `cardinality BETWEEN 1 AND 9`, phần tử 1–9 | Thay `grade_level smallint` (expand: thêm cột, backfill, contract sau) |
| `subject_id` | uuid | NOT NULL | |
| `topic_id` | uuid | NOT NULL | DATABASE_DESIGN để NULL; CONTENT_MODEL §3 bắt buộc |
| `media_refs` | uuid[] | CHECK `cardinality ≤ 50` (`assets_per_version_max`) | |
| `content_hash` | bytea | CHECK `octet_length = 32` | sha256 JSON chuẩn hóa |
| cột mixin version | | `status`, `author_id`, `reviewer_id`, `published_at`, `content_language`, `curriculum_version_id`, `change_note`, `publish_at`, `snapshot_key`, `snapshot_sha256`, `based_on_version_id` | F020 |

**Index**: `question_versions_delivery_idx (tenant_id, subject_id, topic_id, difficulty) WHERE status='published' AND deleted_at IS NULL`; GIN `grade_levels`; `(question_id, status) WHERE status IN ('draft','in_review') AND deleted_at IS NULL` (UNIQUE — một bản nháp mở).

### QuestionVersionObjective — `content.question_version_objectives` (Có sẵn)

PK `(question_version_id, objective_id)`; idx `objective_id`. Thêm cột `tenant_id` (NULL = platform) để RLS loại H (Thêm cột).

### View — `content.question_delivery_v` (Có sẵn — Sửa)

Cột: `question_version_id`, `question_id`, `tenant_id`, `owner_scope`, `version`, `type`, `stem`, `render_payload`, `math_assets`, `max_score`, `difficulty`, `grade_levels`, `subject_id`, `topic_id`, `media_refs`, `content_language`, `exam_only`, `worked_example_ref`. **Không** có `answer_key`, `scoring_rule`, `explanation`, `option_feedback`, `hints`, `min_response_ms`. Điều kiện: `status='published' AND deleted_at IS NULL`.

### Value object (không lưu bảng riêng)

- `Presentation {question_version_id, option_order[], item_order[], seed}` — F030 lưu vào `attempt_items.presentation`.
- `GradeResult {score, max_score, is_correct, parts[{id, correct}], selected_option_ids[]}`.

## 2. Vòng đời / trạng thái

Theo `contentmodel.Lifecycle` (F020). F022 chỉ tạo `draft`, sửa `draft`, tạo version mới; chuyển trạng thái do F025/F026.

## 3. Quy tắc kiểm tra dữ liệu

- `itemId` khớp `^[a-z]_[a-z0-9_]{1,24}$`, duy nhất trong version; cấm `A`–`D`, chỉ số thuần.
- Theo loại: `single_choice` đúng 1 `correct_option_id`; `multiple_choice` ≥ 1, ≤ số phương án; `true_false` `value` hoặc `statement_values` phủ mọi statement; `fill_in_blank` mỗi `{{blank:id}}` trong `stem` có trong `blanks` và `answer_key.blanks`; `numeric` `value` parse được decimal (hoặc biểu thức khi tham số hóa — F023), `tolerance.value ≥ 0`; `matching` mỗi cặp tham chiếu `left`/`right` có thật, không trùng `left`; `ordering` ≥ 3 mục, `order` là hoán vị đủ; `drag_drop` mỗi key là draggable có thật, value là zone có thật.
- `partial_method` theo loại: `multiple_choice` {`right_minus_wrong`,`proportional`}; `ordering` {`position`,`adjacent_pairs`}; `true_false` (statements), `fill_in_blank`, `matching`, `drag_drop` {`proportional`}; loại khác chỉ `all_or_nothing`.
- Khối nội dung cho phép trong câu hỏi: `paragraph`, `heading`, `list`, `math`, `image`, `audio`, `video`, `table`, `callout`.
- Mọi text NFC.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `question.deleted` / `question.restored` | Soft-delete/khôi phục | `resource.deleted.v1` / `resource.restored.v1` (F020) |
| `audit.recorded` | Tạo version, xóa, khôi phục | envelope |
