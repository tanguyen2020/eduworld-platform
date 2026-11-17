# Data Model: F031 — Learning events, mastery v1, evaluation và re-evaluate

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### LearningEvent — `learning.learning_events` (Có sẵn §3.6, tạo ở F030)

DDL §3.6; F031 thêm: GRANT chỉ `INSERT, SELECT` cho role app; job tạo partition `learning_events_YYYY_MM` trước ≥ 2 tháng; RLS theo `tenant_id`.

### MasteryModelConfig — `learning.mastery_model_configs` (Mới, G)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `config_id` | UUIDv7 | PK | |
| `model_version` | `varchar(64)` | UNIQUE | ví dụ `we-1.0` |
| `algorithm` | `varchar(64)` | `weighted_evidence_ewma` | |
| `params` | jsonb | CHECK ≤ 16384 B | `M0, α_base, α_max, w_ctx, w_diff, w_rep, w_hint, guess_coefficient, g, h, n_min, d_min, level_thresholds` (giá trị: LEARNING_ANALYTICS §3.1) |
| `status` | `varchar(64)` | `draft`/`shadow`/`active`/`retired`; partial unique `WHERE status='active'` | |
| `activated_at`, `activated_by` | | | |
| `created_at`, `updated_at`, `deleted_at`, `deleted_by` | | | |

### ObjectiveMastery — `learning.objective_masteries` (Có sẵn §2.7 — Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `tenant_id`, `school_id` | uuid | NOT NULL; RLS | T |
| `student_id`, `objective_id` | uuid | PK `(tenant_id, student_id, objective_id)` | §2.7 PK `(student_id, objective_id)` → thêm `tenant_id` đầu |
| `value` | numeric(4,3) | 0..1 | giá trị gốc (chưa decay) |
| `level` | `varchar(64)` | `MasteryLevel` | level tại lần cập nhật (đọc dùng `Effective`) |
| `evidence_count`, `distinct_days` | int | ≥ 0 | |
| `last_evidence_at` | timestamptz | | |
| `last_evidence_date` | date | | Mới — tính `distinct_days` |
| `first_mastered_at` | timestamptz NULL | | Mới — chỉ phát `mastered` một lần |
| `model_version` | `varchar(64)` | NOT NULL | |
| `prior_source` | `varchar(64)` | DEFAULT `default` | Mới (F032 ghi `diagnostic`) |
| `version` | bigint | | Mới — khóa lạc quan |
| `updated_at`, `deleted_at`, `deleted_by` | | | |

### MasteryHistory — `learning.mastery_history` (Có sẵn §2.7, P tháng — Thêm cột chi tiết)

`history_id`, `tenant_id`, `student_id`, `objective_id`, `value_before`, `value_after` numeric(4,3), `level_before`, `level_after` `varchar(64)`, `source_event_id` uuid, `source_kind varchar(64)` (`evidence`/`re_evaluation`/`recompute`), `model_version varchar(64)`, `occurred_at`; append-only, partition theo `occurred_at`.

### ProgressDaily — `learning.progress_daily` (Có sẵn §2.7)

PK `(tenant_id, student_id, date, subject_id)`; `items_answered`, `items_correct` (lần đầu), `active_seconds`, `lessons_completed`, `activities_completed`, `class_id` snapshot. Cột rollup khác (rapid_guess, hint) thêm ở F032/F043.

### LessonSession — `learning.lesson_sessions` (Có sẵn §2.5 — Thêm cột; F031 sở hữu)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `lesson_session_id` | UUIDv7 | PK | |
| `tenant_id`, `student_id` | uuid | NOT NULL | RLS + `TenantScope` |
| `lesson_id` | uuid | NOT NULL | Mới (tra cứu session mở theo bài) |
| `lesson_version_id` | uuid | NOT NULL | Ghim khi tạo (R31) |
| `status` | `varchar(64)` | CHECK IN (`in_progress`,`completed`) | |
| `context_mode` | `varchar(64)` | `AttemptMode` của attempt mở session (`practice`/`assignment`) | Mới |
| `assignment_id`, `learning_path_id`, `recommendation_id` | uuid NULL | | Mới — chép vào payload `lesson.completed` |
| `completed_activity_ids` | uuid[] | ≤ 50 phần tử | Mới — activity đã có attempt `graded` trong session |
| `active_ms` | bigint | ≥ 0 | Mới — cộng dồn thời gian active của attempt trong session |
| `started_at`, `ended_at` | timestamptz | `ended_at` = `completed_at` | |
| `created_at`, `updated_at`, `deleted_at`, `deleted_by` | | | soft-delete (R21) |

**Index**: partial UNIQUE `(tenant_id, student_id, lesson_id) WHERE status = in_progress AND deleted_at IS NULL` (một session mở mỗi bài); UNIQUE `(tenant_id, student_id, lesson_version_id) WHERE status = completed AND deleted_at IS NULL` (hoàn thành một lần mỗi version, F031-AC-27). Cột `assessment.attempts` không đổi: attempt trong ngữ cảnh bài có `context_type = lesson`, `context_id = lesson_id`.

### Evaluation — `assessment.evaluations` (Có sẵn §2.6a — Thêm cột)

Thêm `regrade_request_id uuid NULL`, `supersedes_evaluation_id uuid NULL`, `details jsonb` (điểm từng item sau chấm lại, ≤ 64 KB); `source` dùng `system`/`re_evaluation` (sửa giá trị `system_reevaluation` ở §2.6a).

### RegradeRequest — `assessment.regrade_requests` (Có sẵn §2.16 — Thêm cột, đổi T → H)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `regrade_request_id` | UUIDv7 | PK | |
| `tenant_id` | uuid NULL | NULL = yêu cầu platform (content_admin) | đổi loại |
| `school_id`, `subject_id` | uuid NULL | | scope academic_manager |
| `scope` | `varchar(64)` | `question_version`/`activity_version`/`exam_session` | Mới |
| `question_version_id`, `activity_version_id`, `exam_session_id`, `corrected_version_id` | uuid NULL | CHECK theo `scope` | |
| `action` | `varchar(64)` | `accept_all`/`use_new_key`/`drop_item` | |
| `reason` | `varchar(1000)` | NOT NULL, ≥ 10 | |
| `status` | `varchar(64)` | `queued`/`running`/`completed`/`failed` | thay `pending` |
| `affected_attempts`, `re_evaluated_attempts` | int | ≥ 0 | Mới |
| `requested_by` | uuid | | |
| `requested_role` | `varchar(64)` | `content_admin`/`academic_manager` | Mới |
| `idempotency_key` | `varchar(64)` | UNIQUE `(requested_by, idempotency_key)` | Mới |
| `created_at`, `started_at`, `finished_at`, `deleted_at`, `deleted_by` | | partial unique `(coalesce(question_version_id, activity_version_id), coalesce(tenant_id, …)) WHERE status IN ('queued','running')` | |

### Recompute — `learning.mastery_recompute_runs`, `learning.objective_mastery_shadows` (Mới)

`mastery_recompute_runs`: `run_id`, `model_version`, `status` (`running`/`completed`/`failed`/`activated`), `report jsonb` (≤ 64 KB), `started_at`, `finished_at`, `started_by`. `objective_mastery_shadows`: cùng cột trạng thái với `objective_masteries` + `run_id`; PK `(run_id, tenant_id, student_id, objective_id)`; dọn bằng soft-delete theo run.

## 2. Vòng đời / trạng thái

- `MasteryModelConfig`: `draft → shadow → active → retired`.
- `RegradeRequest`: `queued → running → completed | failed`.
- `Evaluation`: `final (effective) → superseded`.
- `LessonSession`: `in_progress → completed` (một chiều; version mới của bài mở session mới).

## 3. Quy tắc kiểm tra dữ liệu

- Công thức, tham số, level, decay: [LEARNING_ANALYTICS §3](../../docs/02-learning/LEARNING_ANALYTICS.md).
- Evidence hợp lệ: `question.answered` có `is_scored = true` và `rapid_guess = false`.
- Một evaluation hiệu lực mỗi attempt (§2.6a); attempt gốc không đổi khi chấm lại.
- Hoàn thành bài: `completion_rule` của version đã ghim (`content.lesson_versions`, F024) — `all_required_activities`/`any_activity`; activity "hoàn thành" = có attempt `graded` trong session (hoàn thành ≠ đạt, ACTIVITY_ENGINE §7). `view_only` chưa hỗ trợ (spec Ngoài phạm vi).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `learning_objective.progressed` | mastery đổi | `learning-event.schema.json#/$defs/payload_objective_progressed`; domain: Cần thêm `specs/events/payloads/learning/learning_objective.progressed.v1.schema.json` |
| `learning_objective.mastered` | lần đầu `mastered` | `#/$defs/payload_objective_mastered`; domain: Cần thêm `learning_objective.mastered.v1.schema.json` |
| `lesson.completed` | lesson session chuyển `completed` (F031-AC-26) — phát learning event và domain event | learning event: `learning-event.schema.json#/$defs/payload_lesson_completed`; domain: `specs/events/payloads/learning/lesson.completed.v1.schema.json` |
| `attempt.re_evaluated` | mỗi attempt chấm lại | `#/$defs/payload_attempt_re_evaluated`; domain: Cần thêm `attempt.re_evaluated.v1.schema.json` |
| `audit.recorded` | regrade, xem hồ sơ, kích hoạt mô hình | envelope |
