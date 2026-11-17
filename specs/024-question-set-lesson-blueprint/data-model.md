# Data Model: F024 — Question set, lesson, assessment blueprint, worked example, import hàng loạt, tìm kiếm

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Mọi bảng item dùng `ContentItemMixin`, bảng version dùng `ContentVersionMixin` (F020; loại H). Cột riêng dưới đây chưa có DDL ở DATABASE_DESIGN §2.5 → **Mới** (task T040).

## 1. Entity

### QuestionSet — `content.question_sets`, `content.question_set_versions`, `content.question_set_items` (Có sẵn tên — cột Mới)

| Bảng.Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `question_set_versions.title` | jsonb (`LearningName`) | CHECK ≤ 2048 byte | |
| `.description` | jsonb NULL | CHECK ≤ 8192 byte | |
| `.kind` | `varchar(64)` | CHECK IN (`static`,`rule_based`,`mixed`) | |
| `.purpose` | `varchar(64)` | CHECK IN (`exercise`,`minigame`,`quest`,`competition`,`general`) | |
| `.selection_rules` | jsonb NULL | CHECK `jsonb_array_length ≤ 20`, ≤ 65536 byte | DATABASE_DESIGN §2.5 |
| `.shuffle_items`, `.shuffle_options` | bool | DEFAULT true | |
| `.default_feedback_timing` | `varchar(64)` | CHECK `FeedbackTiming` | |
| `.subject_id` | uuid | NOT NULL | |
| `.grade_levels` | smallint[] | CHECK 1–9 phần tử, giá trị 1–9 | |
| `.topic_ids`, `.objective_ids` | uuid[] | CHECK `cardinality ≤ 50` | |
| `.estimated_duration_sec` | int NULL | > 0 | |
| `.search_text` | `varchar(20000)` | | R1 research |
| `.search_tsv` | tsvector GENERATED | GIN | |
| `question_set_items.question_set_version_id` | uuid | PK ghép với `position` | |
| `.position` | smallint | 1–200 | |
| `.question_version_id` | uuid | NOT NULL, idx | tham chiếu bất biến |
| `.max_score` | numeric(7,2) NULL | > 0 | |
| `.tenant_id` | uuid NULL | RLS loại H | |

UNIQUE `(question_set_version_id, question_version_id)`.

### Lesson — `content.lessons`, `content.lesson_versions` (Có sẵn tên `lesson_versions` — `lessons` Mới)

| Trường (`lesson_versions`) | Kiểu | Ràng buộc |
|---|---|---|
| `title` | jsonb (`LearningName`) | ≤ 2048 byte |
| `summary` | jsonb NULL (`LearningText`) | ≤ 8192 byte |
| `subject_id` | uuid | NOT NULL |
| `grade_levels` | smallint[] | 1–9 phần tử |
| `topic_id` | uuid | NOT NULL |
| `objective_ids` | uuid[] | `cardinality ≤ 50` |
| `estimated_minutes` | smallint NULL | 1–180 |
| `completion_rule` | `varchar(64)` | CHECK IN (`all_required_activities`,`any_activity`,`view_only`) |
| `blocks` | jsonb | `jsonb_array_length` 1–100, ≤ 262144 byte |
| `media_refs` | uuid[] | `cardinality ≤ 50` |
| `worked_example_ids` | `varchar(64)[]` | `cardinality ≤ 100`; GIN — tra `block_id` nhanh |
| `search_text`, `search_tsv` | như trên | |

### LearningMaterial — `content.learning_materials`, `content.learning_material_versions` (Có sẵn tên `learning_materials` — version Mới)

Cột version như `lesson_versions` trừ `completion_rule`, `estimated_minutes`; thêm `material_kind varchar(64)` CHECK IN (`rich_text`,`pdf`,`slides`,`worked_example`).

### AssessmentBlueprint — `content.assessment_blueprints`, `content.assessment_blueprint_versions` (Có sẵn — cột Thêm)

| Trường (version) | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `title` | jsonb | ≤ 2048 byte | |
| `exam_type` | `varchar(64)` | CHECK IN (`practice_test`,`mock`,`chapter`,`semester`,`subject`,`custom`) | |
| `subject_id`, `grade_levels` | | | |
| `sections` | jsonb | ≤ 10 section, ≤ 131072 byte | DATABASE_DESIGN §2.5 |
| `time_limit_seconds` | int | 60–14400 | `duration_sec` của schema |
| `total_points` | numeric(7,2) | > 0 | |
| `scoring` | jsonb | `grading_scale`, `attempt_policy` (≤ 8192 byte) | DATABASE_DESIGN §2.5 |
| `variants` | jsonb | ≤ 4096 byte | Thêm cột |
| `shuffle_policy` | jsonb | `shuffle_options`, `navigation` (≤ 1024 byte) | DATABASE_DESIGN §2.5 (`varchar` → jsonb: Thêm cột thay thế) |
| `result_visibility` | jsonb | `result_release` (`mode`, `feedback_timing` ≠ `immediate`, cờ hiển thị) | DATABASE_DESIGN §2.5 |
| `search_text`, `search_tsv` | | | |

### ImportJob — `content.import_jobs` (Mới ở F021 — mở rộng CHECK `kind`)

`kind` thêm `questions_csv`, `questions_xlsx`, `questions_qti_lite`, `content_json`.

## 2. Vòng đời / trạng thái

Theo `contentmodel.Lifecycle` (F020) cho cả bốn loại; F024 chỉ tạo/sửa `draft`, tạo version mới, soft-delete item chưa có con trỏ.

## 3. Quy tắc kiểm tra dữ liệu

- Set: `items` tham chiếu `question_version_id` trong phạm vi; set platform không tham chiếu câu tenant; luật: `sum(distribution.count) = count`, `difficulty.min ≤ max`.
- Lesson/material: khối `rich_text`/`media`/`worked_example` (`activity` khi F030 đăng ký); `worked_example.id` khớp `^we_[a-z0-9_]{1,40}$`, duy nhất trong version, 1–8 `steps`.
- `worked_example_ref` của câu hỏi: `{content_item_id, block_id}` (`content_item_id` là lesson hoặc learning material, tra theo id), khối tồn tại trong version hiện hành hoặc version nháp mới nhất trong phạm vi, có LO chung.
- Blueprint: `Σ count × points_each = total_points` (sai số 0,001); `feedback_timing ≠ immediate`; `fixed_question_version_ids` dài bằng `count`, không trùng giữa luật.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `<entity>.deleted` / `<entity>.restored` (entity: `question_set`, `lesson`, `learning_material`, `assessment_blueprint`) | Soft-delete/khôi phục | `resource.deleted.v1` / `resource.restored.v1` (F020) |
| `audit.recorded` | Tạo version, xóa, khôi phục, clone, import | envelope |
