# Data Model: F032 — Chế độ luyện tập, hỗ trợ tự học (gợi ý, chẩn đoán, `rapid_guess`), gợi ý ôn tập tự động

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_learning`. Mọi bảng T dùng mixin `Base` (`tenant_id`, `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason varchar(64)`, `version`), RLS theo `app.tenant_id` (DATABASE_DESIGN §1, §4).

## 1. Entity

### AttemptAnswer — `assessment.attempt_answers` (Có sẵn, DDL §3.5)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `response_ms` | `int` | ≥ 0 | `min(client_elapsed_ms, server_interval_ms)` trừ pause |
| `hints_used` | `smallint` | NOT NULL DEFAULT 0, CHECK ≥ 0 | Số cấp mở trước lần trả lời |
| `rapid_guess` | `boolean` | NOT NULL DEFAULT false | F032-AC-16 |

Bảng append-only (TA), partition theo tháng; F032 chỉ ghi giá trị cho 3 cột đã có.

### AttemptItem — `assessment.attempt_items` (Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `hints_revealed` | `smallint` | NOT NULL DEFAULT 0, CHECK `0 ≤ hints_revealed ≤ 5` | ACTIVITY_ENGINE §6.4; tối đa 5 cấp theo `question.schema.json` |
| `presented_at` | `timestamptz` | NULL | Thời điểm giao item, dùng tính `server_interval_ms` |

### Attempt — `assessment.attempts` (Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `recommendation_id` | `uuid` | NULL | Gợi ý đã dẫn tới attempt (ACTIVITY_ENGINE §6.4) |
| `context_type` | `varchar(64)` | giá trị mới: `selection`, `retry_wrong`, `diagnostic` | Bổ sung danh sách giá trị cho attempt ad-hoc |

### WrongQuestionEntry — `learning.wrong_question_entries` (Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `entry_id` | UUIDv7 | PK | mixin `Base` |
| `tenant_id` | uuid | NOT NULL, RLS | R8 |
| `school_id` | uuid | NOT NULL | lọc báo cáo |
| `student_id` | uuid | NOT NULL | |
| `question_id` | uuid | NOT NULL | Câu gốc (định danh, không phải version) |
| `objective_ids` | `uuid[]` | CHECK `cardinality ≤ 20` | Snapshot LO |
| `question_type` | `varchar(64)` (`MaxLen(64)`) | NOT NULL | Dùng chọn câu anh em |
| `difficulty` | `smallint` | 1–5 | |
| `first_wrong_attempt_id` | uuid | NOT NULL | |
| `stage` | `smallint` | 0–2 | Bước trong `retry_wrong_schedule` |
| `next_due_at` | `timestamptz` | NOT NULL | |
| `correct_streak` | `smallint` | 0–2 | Đúng liên tiếp ở ngày khác nhau |
| `last_correct_on` | `date` | NULL | |
| `status` | `varchar(64)` (`MaxLen(64)`) | CHECK `IN ('active','cleared')` | Rời danh sách = `cleared` (không xóa) |
| `deleted_at`, `deleted_by` | | partial unique `(tenant_id, student_id, question_id) WHERE deleted_at IS NULL AND status = 'active'` | R21 |

**Index**: `(tenant_id, student_id, status, next_due_at) WHERE deleted_at IS NULL`.

### StudentRecommendation — `learning.rm_student_recommendations` (Mới; tên theo LEARNING_ANALYTICS §5)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `recommendation_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NOT NULL, RLS | |
| `school_id`, `student_id` | uuid | NOT NULL | |
| `rule_code` | `varchar(64)` | NOT NULL (`R1`…`R9`) | |
| `kind` | `varchar(64)` | CHECK `IN ('assignment','activity','lesson','objective_review','prerequisite_review','retry_wrong','diagnostic','interleave_practice','quest')` | |
| `target_type` | `varchar(64)` | NOT NULL | `activity`/`lesson`/`question_set`/`assignment`/`selection`/`quest` |
| `target_id` | uuid | NULL | NULL với gợi ý dạng `selection` |
| `objective_ids` | `uuid[]` | CHECK `cardinality ≤ 10` | |
| `reason_code` | `varchar(64)` | NOT NULL | Khóa bản dịch lý do (vi/en) |
| `reason_params` | `jsonb` | CHECK `octet_length ≤ 2048` | Tham số hiển thị (tên LO…) |
| `priority` | `smallint` | 1–9 | Theo bảng rule |
| `status` | `varchar(64)` | CHECK `IN ('active','started','dismissed','completed','expired','superseded')` | |
| `skip_count` | `smallint` | NOT NULL DEFAULT 0 | ≥ 3 → không gợi ý lại target |
| `started_attempt_id` | uuid | NULL | |
| `generated_at`, `expires_at` | `timestamptz` | NOT NULL | |
| `deleted_at`, `deleted_by` | | partial unique `(tenant_id, student_id, rule_code, target_type, coalesce(target_id, uuid_nil)) WHERE deleted_at IS NULL AND status = 'active'` | |

**Index**: `(tenant_id, student_id, status, priority) WHERE deleted_at IS NULL`.

### ObjectiveMastery — `learning.objective_masteries` (Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `prior_value` | `numeric(4,3)` | NULL, 0–1 | `M0` theo LO |
| `prior_source` | `varchar(64)` | CHECK `IN ('default','diagnostic','propagated')` DEFAULT `'default'` | LEARNING_ANALYTICS §3.5 |
| `prior_diagnostic_attempt_id` | uuid | NULL | |

Cấu hình `MasteryModel` mới (`w_hint`, `guess_coefficient`, loại `rapid_guess`) thêm thành một dòng `model_version` trong bảng cấu hình mô hình do F031 tạo (seed migration), không sửa dòng cũ.

## 2. Vòng đời / trạng thái

| Thực thể | Chuyển trạng thái |
|---|---|
| WrongQuestionEntry | `active` (sai lượt đầu) → `active` (đến hạn, sai: `stage` về 0) → `cleared` (đúng 2 lần ở 2 ngày khác nhau, hoặc re-evaluate thành đúng) |
| StudentRecommendation | `active` → `started` (attempt mang `recommendation_id`) → `completed` (attempt graded) · `active` → `dismissed` (bỏ qua; `skip_count++`) · `active` → `superseded` (lần tính lại không còn điều kiện) · `active` → `expired` (`expires_at`) |

## 3. Quy tắc kiểm tra dữ liệu

- `hints_used ≤ hints_revealed` tại thời điểm ghi; `hints_revealed ≤ total_levels` của version đã ghim.
- `rapid_guess` chỉ tính cho lần trả lời `is_scored = true`.
- Tối đa 3 gợi ý `active` hiển thị cho học sinh; nội dung gợi ý phải published và khả dụng cho tenant (MULTITENANCY §4.2).
- `prior_value` chỉ ghi khi `evidence_count = 0` (LEARNING_ANALYTICS §3.5 bước 4).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `question.answered` (learning event) | Mỗi lần trả lời có tính điểm | `specs/schemas/learning-event.schema.json` (đã có `hints_used`, `response_ms`, `rapid_guess`) |
| `activity.started` | Start attempt ad-hoc (phát bởi engine F030) | `specs/events/payloads/learning/activity.started.v1.schema.json` (owner F030, có `recommendation_id?`) |
| `learning.recommendation_generated` | Danh sách gợi ý của học sinh thay đổi | Cần thêm `specs/events/payloads/learning/learning.recommendation_generated.v1.schema.json` |
