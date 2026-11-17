# Data Model: F066 — Bài nghe và mô phỏng: plugin `audio_lesson`, `simulation` + checker Go, template Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) §2.6, §2.6a, §2.16

DB `eduworld_learning`. Bảng T có cột chung `tenant_id`, `created_at/by`, `updated_at/by`, `deleted_at/by`, `version` (mixin `pkg/entx`, ADR-0028/0039) và RLS theo `tenant_id` (ADR-0005). Bảng G (platform) không có `tenant_id`.

## 1. Entity

### SimulationTemplate — `content.simulation_templates` (Có sẵn trong DATABASE_DESIGN §2.6a · Mới trong code · Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `template_code` | `varchar(64)` (`MaxLen(64)`) | PK phần 1; pattern `^[a-z0-9_]+(\.[a-z0-9_]+)+$` | ví dụ `physics.lever` |
| `template_version` | `int` | PK phần 2, ≥ 1 | |
| `simulation_kind` | `varchar(32)` (`MaxLen(32)`) | NOT NULL, enum của `simulation.schema.json` | |
| `title` | `jsonb` (i18n, mỗi ngôn ngữ ≤ 255) | NOT NULL | **Mới** — `SimulationTemplate.title` có trong OpenAPI nhưng chưa có cột |
| `parameter_schema` | `jsonb` | NOT NULL, CHECK ≤ 32 KB | JSON Schema tham số |
| `actions` | `jsonb` | NOT NULL, ≤ 50 phần tử `{action ≤ 64, args_schema}` | **Mới** — dùng để kiểm action log và editor |
| `metrics` | `jsonb` | NOT NULL, ≤ 50 phần tử `{metric ≤ 64, unit ≤ 16, display bool}` | **Mới** — editor chọn metric cho `goals[].check` |
| `checker_available` | `bool` | NOT NULL | |
| `min_client_version` | `varchar(32)` (`MaxLen(32)`) | NOT NULL | **Mới** — có trong OpenAPI |
| `status` | `varchar(16)` (`MaxLen(16)`) | `active` / `retired` | do job `simtemplates.Sync` đặt |
| `deleted_at`, `deleted_by` | | | không dùng (template gỡ → `retired`) |

**Index**: PK `(template_code, template_version)`; idx `(status)`.

### SimulationRun — `assessment.simulation_runs` (Có sẵn trong DATABASE_DESIGN §2.6a · Mới trong code)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `run_id` | UUIDv7 | PK | mixin `Base` |
| `tenant_id` | uuid | NOT NULL, RLS | R8 |
| `attempt_id` | uuid | NOT NULL | cùng DB, tham chiếu `assessment.attempts` |
| `template_code` | `varchar(64)` (`MaxLen(64)`) | NOT NULL | giá trị đã ghim |
| `template_version` | `int` | NOT NULL | |
| `seed` | `bigint` | NOT NULL | seed server cấp |
| `action_log` | `jsonb` | NOT NULL, CHECK `octet_length(action_log::text) ≤ 65536` | |
| `client_outcome` | `jsonb` | NULL | chỉ tham khảo |
| `verified_outcome` | `jsonb` | NULL khi `rejected` | |
| `goals_met` | `jsonb` | mảng `{goal_id, met}` | |
| `verification_status` | `varchar(16)` (`MaxLen(16)`) | `verified` / `rejected` / `not_verifiable` | |
| `reject_code` | `varchar(64)` (`MaxLen(64)`) | NULL | **Mới** — lý do từ chối (không chứa dữ liệu học sinh) |
| `idempotency_key` | `varchar(64)` (`MaxLen(64)`) | partial UNIQUE `(attempt_id, idempotency_key) WHERE deleted_at IS NULL` | **Mới** (DATABASE_DESIGN ghi "Idempotency-Key") |
| `run_no` | `smallint` | NOT NULL, UNIQUE `(attempt_id, run_no) WHERE deleted_at IS NULL` | **Mới** — đếm `max_runs` |
| `created_at` | timestamptz | NOT NULL | thời gian server |
| `deleted_at`, `deleted_by` | | | R21 |

**Index**: idx `(tenant_id, attempt_id, run_no DESC) WHERE deleted_at IS NULL`.

### MediaProgress — `assessment.video_progress` (Có sẵn · Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `plays_used` | `smallint` | NOT NULL DEFAULT 0, ≥ 0 | **Mới** — lượt nghe đã dùng (research R1); video luôn 0 |

Các cột khác giữ theo F065 (`position_ms`, `watched_ranges`, `watched_ratio`, `last_heartbeat_at`, `client_seq`).

### Dùng lại (F030/F065, không đổi schema)

| Bảng | Dùng cho |
|---|---|
| `content.activity_versions` | `config` của `audio_lesson`/`simulation` (gồm `goals[].check`, chỉ server đọc) |
| `assessment.attempts` | `seed` của attempt mô phỏng; `source_version_id` ghim |
| `assessment.attempt_items`, `assessment.attempt_answers` | câu nghe hiểu/nghe chép, câu kết luận mô phỏng |
| `assessment.evaluations` | evaluation tự động khi submit |
| `assessment.integrity_signals` | `simulation_log_invalid`, `simulation_outcome_mismatch`, `audio_progress_implausible` |

## 2. Vòng đời / trạng thái

| Thực thể | Trạng thái |
|---|---|
| SimulationTemplate | `active` (descriptor có trong bản phát hành) → `retired` (descriptor gỡ); không quay lại `active` với cùng version |
| SimulationRun | tạo một lần, bất biến: `verified` / `rejected` / `not_verifiable` |
| MediaProgress | tạo khi start (`position_ms = 0`, `plays_used = 0`), cập nhật mỗi heartbeat hợp lệ |

## 3. Quy tắc kiểm tra dữ liệu

- Audio: đoạn `0 ≤ start_ms < end_ms ≤ duration_ms`, không chồng; `at_ms < duration_ms`; câu hỏi loại tự chấm; publish cần `transcript_asset_id` ([AUDIO_LANGUAGE_LEARNING §2](../../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md)).
- `plays_used ≤ max_plays` (nếu đặt).
- Mô phỏng: `parameters` hợp lệ theo `parameter_schema` của template đã chọn; `goals[].check.metric` ∈ `metrics` của template; action log ≤ 500 phần tử, ≤ 64 KB, `t_ms` không giảm, `action` ∈ `actions` ([SIMULATION_ENGINE §5](../../docs/02-learning/SIMULATION_ENGINE.md)).
- `run_no ≤ interaction.max_runs`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `simulation.run_verified` (domain, outbox) | Run `verified`/`not_verifiable` | `specs/events/payloads/learning/simulation.run_verified.v1.schema.json` (Cần thêm) |
| `simulation.run_rejected` (domain, outbox) | Run `rejected` | `specs/events/payloads/learning/simulation.run_rejected.v1.schema.json` (Cần thêm) |
| `simulation.goal_met` (learning event) | Mỗi mục tiêu đạt trong run `verified` | `specs/schemas/learning-event.schema.json#/$defs/payload_simulation_goal_met` |
| `question.answered`, `activity.started`, `activity.completed` (learning event) | Như EVENT_DESIGN §5.2 | `specs/schemas/learning-event.schema.json` |
| `activity.submitted`, `activity.graded` (domain, outbox của engine — owner F030) | Submit/graded | `specs/events/payloads/learning/activity.submitted.v1.schema.json`, `activity.graded.v1.schema.json` |
