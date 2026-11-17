# Data Model: F065 — Bài học video: plugin `video_lesson` và renderer Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) §2.6, §2.6a, §3.4, §3.5

DB `eduworld_learning`. Mọi bảng có cột chung `tenant_id`, `created_at/by`, `updated_at/by`, `deleted_at/by`, `version` (mixin `src/backend/pkg/entx`, ADR-0028/0039); RLS theo `tenant_id` (ADR-0005).

## 1. Entity

### VideoProgress — `assessment.video_progress` (Có sẵn trong DATABASE_DESIGN §2.6a · Mới trong code)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `attempt_id` | uuid | PK; tham chiếu `assessment.attempts` (cùng DB) | một bản ghi mỗi attempt; dùng chung cho audio (F066) |
| `tenant_id` | uuid | NOT NULL, RLS | R8 |
| `position_ms` | `int` | NOT NULL, ≥ 0 | vị trí cuối đã chấp nhận |
| `watched_ranges` | `jsonb` | CHECK `jsonb_array_length ≤ 64`; mỗi phần tử `{start_ms, end_ms}` | đã gộp, sắp xếp, không chồng |
| `watched_ratio` | `numeric(5,4)` | 0 ≤ x ≤ 1 | server tính |
| `last_heartbeat_at` | timestamptz | NOT NULL | thời gian server |
| `client_seq` | `bigint` | NOT NULL, ≥ 0 | heartbeat lớn nhất đã nhận |
| `milestones_emitted` | `smallint` | NOT NULL DEFAULT 0, bitmask 25/50/75/100 | **Mới** — chống phát trùng `video.progressed`; cập nhật DATABASE_DESIGN (T040) |
| `deleted_at`, `deleted_by` | timestamptz, uuid | | R21 |

**Index**: PK `attempt_id`; idx `(tenant_id, last_heartbeat_at)` cho job phân tích.

### VideoBranchChoice — `assessment.video_branch_choices` (Có sẵn trong DATABASE_DESIGN §2.6a · Mới trong code)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `id` | UUIDv7 | PK | mixin `Base` (**Mới**: DATABASE_DESIGN chưa nêu PK) |
| `tenant_id` | uuid | NOT NULL, RLS | |
| `attempt_id` | uuid | NOT NULL | |
| `branch_point_id` | `varchar(64)` (`MaxLen(64)`) | NOT NULL | slug |
| `option_id` | `varchar(64)` (`MaxLen(64)`) | NOT NULL | slug |
| `chosen_at` | timestamptz | NOT NULL | thời gian server |
| `deleted_at`, `deleted_by` | | partial UNIQUE `(attempt_id, branch_point_id) WHERE deleted_at IS NULL` | một lựa chọn mỗi điểm rẽ |

### Dùng lại (F030, không đổi schema)

| Bảng | Dùng cho |
|---|---|
| `assessment.attempts` | attempt `activity_type = video_lesson`, `source_version_id` ghim |
| `assessment.attempt_items` | mỗi câu chèn một item (`position` theo `at_ms`, `question_version_id` ghim) — research R1 |
| `assessment.attempt_answers` | câu trả lời câu chèn (`hints_used`, `response_ms`, `rapid_guess`, `idempotency_key`) |
| `assessment.evaluations` | evaluation tự động khi submit |
| `assessment.integrity_signals` | tín hiệu `video_progress_implausible` (DATABASE_DESIGN §2.16); nếu chưa có ent schema ở F030 thì tạo trong T008 |

## 2. Vòng đời / trạng thái

Attempt theo `AttemptStatus` của engine. `video_progress`: tạo ở heartbeat đầu (hoặc lúc start với `position_ms = 0`), cập nhật mỗi heartbeat hợp lệ; sau `graded` vẫn nhận heartbeat xem lại nhưng không đổi evaluation.

## 3. Quy tắc kiểm tra dữ liệu

- `0 ≤ start_ms < end_ms ≤ duration_ms`; gộp khoảng chồng/kề; tối đa 64 khoảng (gộp khoảng gần nhất khi vượt).
- Phần mới chấp nhận ≤ `(now − last_heartbeat_at) × max(playback_rates)` (research R2).
- Hoàn thành theo `completion_rule` của version (`min_watched_ratio`, `require_required_cues`, `min_score_ratio`); thiếu thì dùng `video_completion`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `video.progressed` (learning event) | `watched_ratio` vượt 25/50/75/100% | `specs/schemas/learning-event.schema.json#/$defs/payload_video_progressed` |
| `video.cue_answered` (learning event) | Chấm một câu chèn | `specs/schemas/learning-event.schema.json#/$defs/payload_video_cue_answered` |
| `question.answered`, `activity.started`, `activity.completed` (learning event) | Như EVENT_DESIGN §5.2 | `specs/schemas/learning-event.schema.json` |
| `activity.submitted`, `activity.graded` (domain, outbox của engine — owner F030) | Submit/graded | `specs/events/payloads/learning/activity.submitted.v1.schema.json`, `activity.graded.v1.schema.json` |
