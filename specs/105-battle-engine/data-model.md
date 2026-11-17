# Data Model: F105 — Battle engine server-authoritative và 8 chế độ thi đấu

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Hai database: `eduworld_competition` (`competition`, `multiplayer`) và `eduworld_learning` (`assessment`). Bảng T dùng mixin `Base`; bảng TA append-only. `matches`, `match_participants` do F104 tạo.

## 1. Entity

### Match — `competition.matches` (Có sẵn, F104 tạo + Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `question_plan_id` | uuid | Đặt khi tạo trận (trước `countdown`) | Thêm cột — plan ở module learning |
| `question_plan_version` | `varchar(64)` | NULL | Có sẵn theo §3.13 (ghim, ADR-0043) |
| `question_plan` | jsonb | ≤ 65536 byte | Có sẵn — chỉ `items[] {index, question_instance_id, question_version_id, time_limit_ms, points_multiplier}`, không đáp án |
| `status` | `varchar(64)` | CHECK `IN ('lobby','running','completed','cancelled','aborted')` | Có sẵn |
| `end_reason` | `varchar(64)` | CHECK `IN ('finished','abandon','forfeit','all_left','server_failure','plan_failure')` | Thêm cột |
| `room_id` | uuid | NULL | Thêm cột |

### MatchParticipant — `competition.match_participants` (Có sẵn, Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `outcome` | `varchar(64)` | CHECK `IN ('win','loss','draw','abandoned','forfeit','no_contest')` | Có sẵn (thêm giá trị) |
| `score` | int | ≥ 0 | Có sẵn |
| `correct_count` | smallint | ≥ 0 | Thêm cột |
| `connected_ms` | int | | Có sẵn |
| `disconnects` | smallint | ≥ 0 | Thêm cột (giải: tối đa 2) |

Bot không có dòng participant (không phải học sinh); thông tin bot lưu trong `match_results.summary.bot`.

### MatchResult — `competition.match_results` (Có sẵn §2.10, G — truy cập qua participant)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `match_id` | uuid | PK | **Một kết quả/trận** |
| `winner_team` | smallint | NULL = hòa/không kết quả | Có sẵn |
| `summary` | jsonb | ≤ 65536 byte | Có sẵn: `questions[] {index, per_student[] {student_id, answered, is_correct, points, time_bonus, received_ms}}`, `bot` (nếu có) |
| `verified_at` | timestamptz | NOT NULL | Có sẵn |
| `recorded_by` | `varchar(64)` | | Có sẵn (instance id room host) |
| `rated` | boolean | NOT NULL | Thêm cột |
| `rule_version_id` | uuid | NOT NULL | Thêm cột — config version luật mode |
| `tenant_ids` | uuid[] | | Thêm cột (giống `matches`, §4 RLS) |

### IntegritySignal — `competition.integrity_signals` (Có sẵn §2.16, T, P tháng)

`signal_id` · `tenant_id`, `school_id`, `student_id` · `match_id` · `signal_type varchar(64)` CHECK `IN ('fast_answer','answer_before_shown','answer_closed_question','win_trading','device_sharing')` · `score numeric(5,2)` · `evidence jsonb` (≤ 8192 byte, không có câu trả lời thô) · `created_at`. Index `(tenant_id, student_id, created_at DESC)`. Append-only.

### Room — `multiplayer.rooms` (Có sẵn §2.9, T) · SessionMembership — `multiplayer.session_memberships` (Có sẵn §2.9, Thêm cột)

| Bảng | Cột F105 dùng / thêm |
|---|---|
| `rooms` | `room_id`, `room_type` (CHECK thêm `lobby`,`battle`,`coop`), `school_id` (NULL khi liên trường), `capacity`, `status`, `created_at`, `closed_at`; Thêm `match_id` |
| `session_memberships` | Có sẵn `room_id`, `student_id`, `school_id`, `joined_at`, `left_at`, `disconnects`; Thêm `tenant_id` (nếu thiếu), `game_session_id`, `party_id`, `match_id`, `leave_reason varchar(64)` CHECK `IN ('left','disconnected','kicked','timeout','room_closed','transferred','replaced_by_other_device')` ([MULTIPLAYER_GAMEPLAY §10](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#10-session-membership)) |

### CompetitionRestriction — `competition.competition_restrictions` (F104 tạo) — F105 ghi `kind = abandon_penalty`.

### QuestionPlan — `assessment.question_plans` (Mới, T, DB `eduworld_learning`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `plan_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NOT NULL, RLS | Tenant của trận (giải platform: tenant của từng học sinh khi chấm) |
| `match_id` | uuid | UNIQUE `WHERE deleted_at IS NULL` | Idempotent `createQuestionPlan` |
| `mode` | `varchar(64)` | CHECK theo `MatchMode` | |
| `grade_level` | smallint | 1–9 | |
| `items` | jsonb | ≤ 65536 byte | `[{index, question_version_id, time_limit_ms}]` — answer key đọc từ `question_versions`, không chép |
| `participants` | uuid[] | `cardinality ≤ 40` | |
| `expires_at` | timestamptz | NOT NULL | |

Câu trả lời ghi `assessment.attempt_answers` (Có sẵn §3.5) với attempt `mode = competition`, `context.match_id`; UNIQUE `(plan_id, index, student_id)` qua cột `idempotency_key varchar(64)` của attempt answer hoặc bảng `infra.idempotency_records`.

## 2. Vòng đời / trạng thái

Room (Valkey + checkpoint): `lobby → countdown → question_shown ⇄ reveal → round_end → finalizing → ended`; `question_shown → paused → question_shown`; mọi trạng thái → `aborted` (không checkpoint / plan lỗi). Match (PG): `lobby → running → completed | cancelled | aborted`. Checkpoint `ew:rt:room:<room_id>:ckpt` sau mỗi chuyển trạng thái ([REALTIME_ARCHITECTURE §5.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md#51-room-registry--failover-module competition)).

## 3. Quy tắc kiểm tra dữ liệu

- Điểm/câu theo `battle_1v1_format`, `team_battle_format`; bonus chỉ khi `matches.grade_level ≥ speed_bonus_min_grade` ([COMPETITION_SYSTEM §3.1](../../docs/01-game-design/COMPETITION_SYSTEM.md#31-1v1-knowledge-battle-proposed)).
- `rated = matches.ranked AND status = completed AND NOT is_bot AND tournament_id IS NULL AND end_reason ≠ server_failure`.
- `outcome` người rời quá grace: `abandoned` (thường) / `forfeit` (giải).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `match.started` | `countdown` xong, câu 1 hiện | `specs/events/payloads/competition/match.started.v1.schema.json` (Cần thêm) |
| `match.completed` | `RecordMatchResult` commit | `specs/events/payloads/competition/match.completed.v1.schema.json` (Cần thêm): `match_id`, `mode`, `matchmaking_type`, `ranked`, `rated`, `is_bot`, `subject_id`, `grade_level`, `tournament_id`, `tournament_node_id`, `end_reason`, `results[] {student_id, school_id, team_no, outcome, score, correct_count}` |
| `match.aborted` | `AbortMatch` | `specs/events/payloads/competition/match.aborted.v1.schema.json` (Cần thêm) |
| `audit.recorded` | Kết quả, abort, hạn chế | envelope chuẩn |
