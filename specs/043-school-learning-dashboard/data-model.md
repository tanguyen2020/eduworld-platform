# Data Model: F043 — Rollup analytics và School Learning Dashboard API

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_learning`, schema `learning`. Cấu trúc chung theo [DASHBOARD_DATA §3.1](../../docs/07-backend/DASHBOARD_DATA.md): bảng dẫn xuất có `tenant_id` + RLS, **không** `deleted_at` (dựng lại được, ADR-0028 áp cho nguồn); ghi bằng role `learning_app` (worker), đọc bằng `learning_dashboard` trên replica. DATABASE_DESIGN §2.7 hiện ghi `progress_daily(date…)` và `class_objective_rollups` — lệch với DASHBOARD_DATA; task T008 cập nhật DATABASE_DESIGN theo bảng dưới.

## 1. Entity

### Cột chung của mọi rollup chuỗi thời gian (Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `tenant_id` | uuid | NOT NULL, RLS | R8 |
| `bucket_date` | `date` | NOT NULL | Tuần = thứ Hai ISO; tháng = ngày 1 |
| `metric_version` | `smallint` | NOT NULL | Đổi công thức → version mới |
| `count_*`, `sum_*` | `bigint` | NOT NULL DEFAULT 0 | Cộng dồn được |
| `*_num`, `*_den` | `bigint` | NOT NULL DEFAULT 0 | Tỷ lệ (accuracy, rapid_guess, on_time) |
| `time_hist` | `int[]` | CHECK `cardinality ≤ 32` | Histogram bucket cố định |
| `active_students` | `int` | NULL | Distinct, chỉ đúng khi `is_final` |
| `is_final` | `boolean` | NOT NULL DEFAULT false | |
| `needs_reclose` | `boolean` | NOT NULL DEFAULT false | Event muộn trong cửa sổ |
| `computed_at` | `timestamptz` | NOT NULL | Nguồn `data_as_of` |

### `learning.progress_daily` (Thêm cột / đổi khóa)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `student_id`, `class_id` (snapshot lúc event), `subject_id`, `school_id` | uuid | NOT NULL | |
| `activities_completed`, `items_answered`, `items_correct`, `items_rapid_guess`, `items_with_hint`, `active_seconds`, `lessons_completed` | `int` | ≥ 0 | LEARNING_ANALYTICS §4 |

PK `(tenant_id, student_id, subject_id, bucket_date, metric_version)`; partition tháng; index `(tenant_id, class_id, bucket_date)`, `(tenant_id, student_id, bucket_date)`, BRIN `(bucket_date)`.

### `learning.rollup_class_daily` / `_weekly` / `_monthly` (Mới)

Chiều `class_id`, `subject_id`, `school_id`; đo: `active_students`, `activities_completed`, `accuracy_num/den`, `rapid_guess_num/den`, `active_seconds`, `assignments_on_time_num/den`, `time_hist`. PK `(tenant_id, class_id, subject_id, bucket_date, metric_version)`; partition: daily theo tháng, weekly theo năm, monthly không partition.

### `learning.rollup_class_objective_weekly` (Mới)

Chiều `class_id`, `objective_id`; đo: `evidence_count`, `correct_num/den`, `level_dist int[5]` (cuối tuần). PK `(tenant_id, class_id, objective_id, bucket_date, metric_version)`.

### `learning.rollup_school_daily` / `_weekly` / `_monthly` (Mới)

Chiều `school_id`, `grade_level smallint`, `subject_id`; đo như cấp lớp + `active_classes`. PK `(tenant_id, school_id, grade_level, subject_id, bucket_date, metric_version)`.

### `learning.rollup_tenant_daily` / `_weekly` / `_monthly` (Mới)

Chiều `grade_level`, `subject_id`; đo như cấp trường + `active_schools`. PK `(tenant_id, grade_level, subject_id, bucket_date, metric_version)`.

### Read model trạng thái hiện tại (Mới, thay `class_objective_rollups` của DATABASE_DESIGN §2.7)

| Bảng | Khóa | Cột chính |
|---|---|---|
| `learning.rm_class_objective_stats` | `(tenant_id, class_id, objective_id)` | `school_id`, `subject_id`, `level_dist int[5]`, `students_with_evidence`, `accuracy_num/den`, `computed_at` |
| `learning.rm_class_subject_summary` | `(tenant_id, class_id, subject_id)` | `school_id`, `student_count`, `weak_objective_ids uuid[]` (≤ 20), `assignment_completion_num/den`, `rapid_guess_students`, `computed_at` |
| `learning.rm_school_grade_summary` | `(tenant_id, school_id, grade_level, subject_id)` | `curriculum_coverage_num/den`, `participation_num/den`, `computed_at` |
| `learning.rm_student_attention` | `(tenant_id, student_id)` | `school_id`, `class_id`, `reasons varchar(64)[]` (≤ 8: `missed_assignments`, `beginning_objectives`, `inactive_game`, `rapid_guess`, `scheduled_cleanup`), `computed_at` |
| `learning.mastery_snapshots_weekly` | `(tenant_id, student_id, subject_id, bucket_date)` | `mean_mastery numeric(4,3)`, `mastered_count`, `level_dist int[5]`; partition năm |

Không có cột `xp`, `rating`, nội dung chat trong mọi bảng trên (F043-AC-11).

### DB role (Mới)

`learning_dashboard`: `SELECT` chỉ trên bảng rollup/read model trên; `statement_timeout = dashboard_statement_timeout`; không quyền trên `learning_events`, `assessment.*`, `lms.*` trừ `lms.assignment_submissions` (read model trạng thái, DASHBOARD_DATA §2).

## 2. Vòng đời / trạng thái

| Trạng thái dòng rollup | Chuyển |
|---|---|
| `is_final = false` | Upsert trong ngày; HLL Valkey cho distinct tạm tính |
| `is_final = true` | Job `dashboard-day-close` sau `dashboard_day_close_time` |
| `needs_reclose = true` | Event muộn ≤ `dashboard_late_window_hours` vào ngày đã chốt → chốt lại → `false` |

## 3. Quy tắc kiểm tra dữ liệu

- Mọi truy vấn đọc có `tenant_id = ANY(@tenant_ids)` + phạm vi (trường/lớp/môn) + `bucket_date BETWEEN @from AND @to` + `metric_version = @current`.
- Tuần/tháng dựng từ bảng ngày (cộng dồn) và từ `progress_daily` cho distinct, không từ dữ liệu thô.
- Báo cáo cấp tenant/platform ẩn ô có số học sinh < `analytics_min_cell_size`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — (chỉ tiêu thụ) | `question.answered`, `activity.*`, `exam.graded`, `attempt.re_evaluated`, `assignment.*`, `learning_objective.*`, `student.game_logged_in/out` | `specs/schemas/learning-event.schema.json`, `specs/events/payloads/learning/*.v1.schema.json` (một số Cần thêm — contracts §4) |
