# Data Model: F030 — Activity Registry và Attempt API (`quiz`, `interactive_exercise`)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### LearningActivity / ActivityVersion — `content.activities`, `content.activity_versions` (Có sẵn §2.6a)

Theo DATABASE_DESIGN §2.6a (`activity_type varchar(32)`, `config jsonb ≤ 256 KB`, `math_assets jsonb`, UNIQUE `(activity_id, version)`, trigger bất biến) + cột release (F026) + cột workflow/ngôn ngữ (F025). Thêm: `feedback_timing varchar(64) NULL`, `time_limit_seconds int NULL`, `instructions jsonb` (i18n ≤ 2.000 ký tự/bản) nếu chưa nằm trong `config` (Thêm cột).

### Attempt — `assessment.attempts` (Có sẵn §3.4 — Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `attempt_id`, `tenant_id`, `school_id`, `student_id` | uuid | NOT NULL | T |
| `mode` | `varchar(64)` | CHECK `AttemptMode` | |
| `context_type` | `varchar(64)` | CHECK IN (`free_play`,`lesson`,`learning_path`,`recommendation`,`question_set`,`selection`,`assignment`,`quest_step`,`minigame_session`,`exam_session`,`match`,`challenge_attempt`,`boss_encounter`) | Sửa danh sách §3.4; hai giá trị cuối cho mode `challenge` [Confirmed] |
| `context_id` | uuid | NOT NULL | free_play: `activity_id` |
| `activity_id` | uuid NULL | | Mới; NULL với ad-hoc |
| `activity_type` | `varchar(32)` | CHECK `ActivityType` | Mới |
| `source_version_id` | uuid | NOT NULL | ghim (ADR-0043) |
| `status` | `varchar(64)` | CHECK `AttemptStatus` | |
| `seed` | bigint | NOT NULL | |
| `started_at`, `deadline_at`, `submitted_at`, `graded_at` | timestamptz | | server |
| `paused_at` | timestamptz NULL | | Mới |
| `paused_ms` | bigint | DEFAULT 0 | Mới |
| `last_activity_at` | timestamptz | NOT NULL | Mới (idle/abandon) |
| `feedback_timing` | `varchar(64)` | CHECK `FeedbackTiming` | Mới |
| `display_language` | `varchar(64)` | `vi`/`en` | Mới (ADR-0056) |
| `submit_reason` | `varchar(64)` NULL | `manual`/`timeout`/`auto_complete` | Mới |
| `score`, `max_score` | numeric(7,2) | | |
| `duration_ms` | bigint | | `submitted_at − started_at − paused_ms` |
| `client_elapsed_ms` | bigint NULL | | Mới, tham khảo |
| `idempotency_key` | `varchar(64)` | UNIQUE `(student_id, idempotency_key)` | Mới (start) |
| `version`, `deleted_at`, `deleted_by` | | CHECK không xóa attempt `submitted/graded` | |

**Index**: theo §3.4 + partial UNIQUE `(student_id, source_version_id) WHERE status='in_progress' AND mode='practice' AND deleted_at IS NULL` (Mới, F030-AC-04); `(status, deadline_at) WHERE status='in_progress'`; `(status, last_activity_at) WHERE status='in_progress'`.

### AttemptItem — `assessment.attempt_items` (Có sẵn §2.6 — Thêm cột)

PK `(attempt_id, position)`; `question_version_id`, `presented_payload_hash`, `max_score` + Mới: `tenant_id`, `question_type varchar(64)`, `objective_ids uuid[]` (snapshot, CHECK cardinality ≤ 50), `presentation jsonb` (thứ tự option, tham số; ≤ 16 KB), `answer_count smallint`, `final_is_correct bool`, `final_score numeric(6,2)`, `hints_revealed smallint DEFAULT 0` (F032 ghi).

### AttemptAnswer — `assessment.attempt_answers` (Có sẵn §3.5, partitioned, append-only)

Theo DDL §3.5 + Mới: `is_scored bool NOT NULL DEFAULT true`, `attempt_no smallint NOT NULL`. `hints_used`, `rapid_guess` giữ mặc định (F032 điền).

### Evaluation — `assessment.evaluations` (Có sẵn §2.6a)

F030 tạo `source = 'system'`, `status = 'final'`, `effective = true`. Giá trị re-evaluate theo OpenAPI là `re_evaluation` (F031); `source varchar(16)` đủ độ dài.

### LearningEvent — `learning.learning_events` (Có sẵn §3.6, partitioned tháng)

F030 tạo bảng + partition tháng hiện tại và tháng kế; ghi `activity.started`, `question.answered`, `activity.submitted`, `activity.graded` theo [`learning-event.schema.json`](../../specs/schemas/learning-event.schema.json). Khóa idempotency: `start:<attempt_id>`, `qa:<attempt_id>:<position>:<attempt_no>`, `sub:<attempt_id>`, `grd:<evaluation_id>`.

## 2. Vòng đời / trạng thái

| Từ | Sự kiện | Tới |
|---|---|---|
| — | start | `in_progress` |
| `in_progress` | respond / pause / resume | `in_progress` (`paused_at` bật/tắt) |
| `in_progress` | submit | `submitted` → `graded` (cùng transaction) |
| `in_progress` | quá `deadline_at` | `expired` → `graded` nếu có câu trả lời (`timeout`) |
| `in_progress` | quá `attempt_resume_window` | `abandoned` |

## 3. Quy tắc kiểm tra dữ liệu

- Vòng đời, resume, phản hồi, chấm: [ACTIVITY_ENGINE §6–§8, §11](../../docs/02-learning/ACTIVITY_ENGINE.md).
- `CHECK (status <> 'graded' OR (score IS NOT NULL AND graded_at IS NOT NULL))` (§3.4); một evaluation hiệu lực: UNIQUE partial `(attempt_id) WHERE effective` (§2.6a).
- `answer_key` chỉ đọc qua scorer; query phục vụ học sinh dùng `content.question_delivery_v` (§2.5).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `activity.submitted` | submit / auto-submit | `specs/events/payloads/learning/activity.submitted.v1.schema.json` |
| `activity.graded` | evaluation hiệu lực tạo xong | `specs/events/payloads/learning/activity.graded.v1.schema.json` |
| `activity.started` | Start attempt (mọi mode; `recommendation_id?` khi bắt đầu từ gợi ý của F032) | Cần thêm `specs/events/payloads/learning/activity.started.v1.schema.json` (T052) |
| `activity.abandoned` | `abandoned` | Cần thêm `activity.abandoned.v1.schema.json` |
| `audit.recorded` | CMS activity create/update | envelope |
