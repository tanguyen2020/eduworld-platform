# Data Model: F107 — Tournament: 6 format, bracket, lịch, thử thách khối/giải trường từ template

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_competition`, schema `competition`. T = mixin `Base` + RLS; TA = append-only. DATABASE_DESIGN §2.10 chỉ liệt kê tên `tournaments`, `tournament_entries`, `bracket_nodes` — cột dưới đây theo [COMPETITION_SYSTEM §10.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#102-mô-hình) và cần ghi vào DATABASE_DESIGN.

## 1. Entity

### Challenge — `competition.challenges` (Mới, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `challenge_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id` | uuid | NOT NULL, RLS | |
| `template_config_id`, `template_version_id` | uuid | NOT NULL | `game_config_versions` (ghim) |
| `kind` | `varchar(64)` | CHECK `IN ('grade_challenge','school_tournament')` | ADR-0095: thay `class_challenge`, `class_tournament` (chưa có dòng nào — feature chưa code) |
| `name` | `varchar(128)` | NOT NULL | API `maxLength 80` |
| `targets` | jsonb | ≤ 8192 byte | `[{target_type, grade_level?, student_ids?}]` — `target_type` ∈ `grade`, `students`; `student_ids` ≤ 500 — mọi học sinh thuộc `school_id` (ADR-0095 mục 3, 5) |
| `exclude_student_ids` | uuid[] | ≤ 500, NULL | Loại trừ; **exclude thắng** include |
| `participation` | `varchar(64)` | CHECK `IN ('individual')` | Bỏ `study_group` (ADR-0095) [Proposed] |
| `question_set_version_id` | uuid | NULL | |
| `params` | jsonb | ≤ 8192 byte | Chỉ khóa trong `allowed_params` |
| `starts_at`, `ends_at` | timestamptz | `ends_at > starts_at` | |
| `status` | `varchar(64)` | CHECK `IN ('scheduled','running','completed','cancelled')` | |
| `tournament_id`, `match_id` | uuid | NULL | giải / trận Timed Quiz |
| `cancel_reason` | `varchar(1000)` | NULL | |
| `created_by` | uuid | NOT NULL | |

Index `(tenant_id, school_id, status, starts_at) WHERE deleted_at IS NULL`.

### Tournament — `competition.tournaments` (Có sẵn tên, Thêm cột, T; giải platform G có `tenant_ids`)

`tournament_id` PK · `tenant_id`, `school_id` · `owner_scope varchar(64)` CHECK `IN ('school','tenant','platform')` (giải khối của trường = `school` + `grade_levels` một phần tử — ADR-0095), `owner_id` · `challenge_id` · `name varchar(128)` · `format varchar(64)` CHECK `IN ('single_elim','double_elim','round_robin','groups_then_single_elim')` · `participant_type varchar(64)` (`individual`/`team`), `team_size smallint` · `mode varchar(64)` (`battle_1v1`/`team_battle`) · `subject_id`, `topic_ids uuid[]` (≤ 50), `grade_levels smallint[]` · `question_policy jsonb` · `registration_open_at`, `registration_close_at`, `check_in_opens_at`, `starts_at` · `max_entries smallint` (2–256) · `seeding_policy varchar(64)` CHECK `IN ('rating','random','manual')` (bỏ `separate_class` — ADR-0095), `seed_value bigint` · `auto_confirm boolean` · `reward_rule_refs jsonb` · `status varchar(64)` CHECK `IN ('draft','published','registration_open','registration_closed','seeding','scheduled','in_progress','completed','rewarded','archived','cancelled')` · `rule_version_id` · `tenant_ids uuid[]` · `version`.

### TournamentEntry — `competition.tournament_entries` (Có sẵn, T)

`entry_id` PK · `tournament_id` · `tenant_id`, `school_id` · `student_id` (NULL khi đội) · `team_ref jsonb` (`{party_id, member_ids[]}` ≤ 8) · `seed smallint` · `group_id` · `status varchar(64)` CHECK `IN ('registered','confirmed','checked_in','withdrawn','disqualified','eliminated','champion')` · `registered_by` · `checked_in_at`. UNIQUE `(tournament_id, student_id) WHERE deleted_at IS NULL AND student_id IS NOT NULL` (Có sẵn).

### TournamentGroup — `competition.tournament_groups` (Mới, T)

`group_id` PK · `tournament_id` · `tenant_id` · `code varchar(16)` · `standings jsonb` (≤ 16384). UNIQUE `(tournament_id, code)`.

### BracketNode — `competition.bracket_nodes` (Có sẵn tên, Thêm cột, T)

`node_id` PK · `tournament_id` · `tenant_id` · `bracket varchar(64)` CHECK `IN ('winners','losers','grand_final','group')` · `round smallint`, `position smallint` · `group_id` · `entry_a_id`, `entry_b_id`, `winner_entry_id` · `match_id` · `scheduled_at` · `next_win_node_id`, `next_lose_node_id` · `reschedule_count smallint` (≤ 2) · `status varchar(64)` CHECK `IN ('pending','scheduled','in_match','awaiting_confirm','confirmed','bye','overridden','void')` · `override_reason varchar(1000)`, `overridden_by` · `version`. UNIQUE `(tournament_id, bracket, round, position)`; UNIQUE `(match_id) WHERE match_id IS NOT NULL`.

### TournamentResult — `competition.tournament_results` (Mới, TA) · TournamentReward — `competition.tournament_rewards` (Mới, TA)

| Bảng | Cột | Ràng buộc |
|---|---|---|
| `tournament_results` | `tournament_id`, `entry_id`, `tenant_id`, `school_id`, `final_rank smallint`, `wins`, `losses`, `draws smallint`, `points int`, `created_at` | PK `(tournament_id, entry_id)` |
| `tournament_rewards` | `tournament_id`, `entry_id`, `rule_id varchar(64)`, `tenant_id`, `student_ids uuid[]`, `reward_ref jsonb`, `granted_at` | PK `(tournament_id, entry_id, rule_id)` |

### Gán template — `infra.tenant_resource_assignments` (Có sẵn §2.17)

`resource_type` ∈ (`tournament_template`, `challenge_template`) · `resource_id` = `game_configs.config_id` · `tenant_id` · soft-delete khi gỡ gán.

## 2. Vòng đời / trạng thái

Tournament theo [COMPETITION_SYSTEM §10.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#103-vòng-đời). Challenge: `scheduled → running → completed`, hủy → `cancelled`. Nút: `pending → scheduled → in_match → awaiting_confirm → confirmed` (hoặc `bye`, `overridden`, `void`).

## 3. Quy tắc kiểm tra dữ liệu

- Tối thiểu entry: single elim ≥ 4, double elim ≥ 4, round robin ≥ 3 ([COMPETITION_SYSTEM §10.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#104-đăng-ký-và-xác-nhận-confirmed-trừ-khi-ghi-khác)).
- Đối tượng ([COMPETITION_SYSTEM §10.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#104-đăng-ký-và-xác-nhận-confirmed-trừ-khi-ghi-khác), ADR-0095): (học sinh hiệu lực của `grade_level` trong `school_id` ∪ `student_ids`) − `exclude_student_ids`; học sinh không thuộc `school_id` → `422`; khối và trường đọc từ `ref.students`.
- `params` ⊆ `allowed_params` của template version; `reward_rule_refs`, `format`, `mode` chỉ từ template.
- Bracket sinh trong một transaction; trận giải không đổi rating.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `tournament.match_scheduled` | Trước giờ đấu 10 phút và 1 phút | `specs/events/payloads/competition/tournament.match_scheduled.v1.schema.json` (Cần thêm) |
| `tournament.completed` | Chung kết xác nhận | `.../tournament.completed.v1.schema.json` (Cần thêm) |
| `tournament.rewarded` | Mỗi thưởng trao | `.../tournament.rewarded.v1.schema.json` (Cần thêm, + catalog EVENT_DESIGN §4.8) — `tournament_id`, `entry_id`, `rule_id`, `student_ids[]`, `reward_ref` |
| `audit.recorded` | Thao tác người tổ chức, gán template | envelope |

## 5. Ghi chú triển khai (2026-11-24, migration `20261231000006_f107_tournament.sql`)

- `challenges`: thêm `format` (`timed_quiz`/`tournament`), `grade_level` (khối của target `grade`, lọc `listChallenges`), `subject_id`, `join_opens_at`; `exclude_student_ids` là `jsonb` (≤ 500) thay `uuid[]`; `match_id` thành `match_ids jsonb` (một thử thách có thể nhiều phòng Timed Quiz ≤ 40 học sinh).
- `tournaments`: thêm `template_config_id`, `min_entries`, `round_gap_s`, `reward_summary`, `cancel_reason`, `completed_at`, `rewarded_at`; `topic_ids`/`grade_levels` là `jsonb`; `tenant_ids uuid[]` viết tay (chỉ chuẩn bị giải platform).
- `tournament_entries`: `team_ref` thay bằng `party_id` + `member_ids jsonb` (luôn có, kể cả entry cá nhân) để kiểm tra "mỗi học sinh một entry" bằng GIN `jsonb_path_ops`; UNIQUE thêm `(tournament_id, party_id)`.
- `bracket_nodes`: thêm `is_draw`, `result jsonb` (log khiếu nại không chứa chat), `started_at`, `next_win_slot`, `next_lose_slot`, `wave`, `attempts`, `notices`, `confirm_after`, `decision_required`.
- `tournament_results`/`tournament_rewards`: PK kỹ thuật `result_id`/`reward_id` (ent cần một cột id) + UNIQUE nghiệp vụ `(tournament_id, entry_id)` / `(tournament_id, entry_id, rule_id)`; thêm `matches_played`, `points`.
- `infra.tenant_resource_assignments` của `eduworld_competition` (DATABASE_DESIGN §3.0): `resource_type` ∈ (`tournament_template`, `challenge_template`), FK `resource_id → game_configs`.
