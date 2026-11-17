# Data Model: F104 — Matchmaking 8 chế độ: queue, ticket, proposal, ready check, ràng buộc khối

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_competition`. Bảng T dùng mixin `Base` (`tenant_id`, `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason varchar(64)`, `version`), RLS theo `app.tenant_id`. Ticket/proposal **không** có bảng (ADR-0004).

## 1. Entity

### Match — `competition.matches` (Có sẵn §2.10 + Thêm cột; G*: truy cập qua `match_participants`)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `match_id` | uuid | PK | = `proposal_id` (idempotent) |
| `mode` | `varchar(64)` | CHECK `IN ('battle_1v1','team_battle','coop_challenge','timed_quiz','tournament')` | Có sẵn (`MatchMode`) |
| `subject_id` | uuid | NOT NULL | Có sẵn |
| `grade_level` | smallint | 1–9 | Có sẵn — khối thấp nhất |
| `status` | `varchar(64)` | CHECK `IN ('lobby','running','completed','cancelled','aborted')` | Có sẵn; F104 tạo `lobby` |
| `tournament_id` | uuid | NULL | Có sẵn |
| `question_plan` | jsonb | CHECK `octet_length ≤ 65536` | Có sẵn (F105 ghi) |
| `matchmaking_type` | `varchar(64)` | CHECK `IN ('random','skill','grade','subject','topic','friend','team','tournament','bot')` | Thêm cột |
| `topic_id` | uuid | NULL | Thêm cột |
| `ranked` | boolean | NOT NULL | Thêm cột; `false` cho random/topic/friend/bot |
| `is_bot` | boolean | NOT NULL DEFAULT false | Thêm cột |
| `scope_kind` | `varchar(64)` | CHECK `IN ('school','tenant','platform')` | Thêm cột |
| `tenant_ids` | uuid[] | CHECK `cardinality ≤ 64` | Có sẵn (§4 RLS) |
| `duel_id`, `tournament_node_id` | uuid | NULL | Thêm cột |
| `ruleset_version_id` | uuid | NOT NULL | Thêm cột — `game_config_versions` matchmaking hiệu lực |
| `created_at`, `started_at`, `ended_at` | timestamptz | | Có sẵn |

**Index**: `(tournament_id) WHERE tournament_id IS NOT NULL`; `(status, created_at)`.

### MatchParticipant — `competition.match_participants` (Có sẵn §2.10, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `match_id`, `student_id` | uuid | PK `(match_id, student_id)` | Có sẵn |
| `tenant_id`, `school_id` | uuid | NOT NULL, RLS | Có sẵn |
| `team_no` | smallint | 0–1 | Có sẵn |
| `party_id` | uuid | NULL | Có sẵn |
| `outcome`, `score`, `connected_ms` | | NULL | Có sẵn (F105 ghi) |
| `rating_mu_at_match`, `rating_rd_at_match` | numeric(8,3) | NULL | Thêm cột — chỉ nội bộ, không trả client |

### Duel — `competition.duels` (Có sẵn §2.10, T)

`duel_id` PK · `tenant_id`, `school_id` · `challenger_id`, `opponent_id` uuid · `subject_id`, `topic_id` · `status varchar(32)` CHECK `IN ('pending','accepted','declined','expired','cancelled')` · `match_id` · `expires_at` (tạo + 60 s) · UNIQUE `(challenger_id, opponent_id) WHERE status = 'pending' AND deleted_at IS NULL`.

### CompetitionRestriction — `competition.competition_restrictions` (Mới, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `restriction_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id`, `student_id` | uuid | NOT NULL | |
| `kind` | `varchar(64)` | CHECK `IN ('abandon_penalty','restrict_competition')` | `abandon_penalty` ghi bởi F105 |
| `reason` | `varchar(1000)` | NOT NULL | |
| `starts_at`, `ends_at` | timestamptz | `ends_at > starts_at` | |
| `created_by` | uuid | NULL = system | |

**Index**: `(tenant_id, student_id, ends_at) WHERE deleted_at IS NULL`.

### GameConfig / GameConfigVersion / ContentRelease — `competition.game_configs`, `competition.game_config_versions`, `competition.content_releases` (Mới, G)

| Bảng | Cột chính | Ràng buộc |
|---|---|---|
| `game_configs` | `config_id` PK, `config_type varchar(64)` CHECK `IN ('matchmaking-configs','ranking-rules','seasons','tournament-templates','challenge-templates','leaderboard-definitions','competition-rewards')`, `code varchar(64)`, `current_version_id`, `latest_version_no int`, `owner_scope varchar(64)` = `platform` | UNIQUE `(config_type, code) WHERE deleted_at IS NULL` |
| `game_config_versions` | `config_version_id` PK, `config_id`, `version int`, `status varchar(64)` (`ContentStatus`), `title varchar(255)`, `payload jsonb` (CHECK ≤ 65536), `effective_from timestamptz`, `author_id`, `reviewer_id`, `change_note varchar(500)`, `based_on_version_id`, `published_at` | UNIQUE `(config_id, version)`; trigger chặn UPDATE khi `status = 'published'` (R11) |
| `content_releases` | `release_id`, `config_id`, `action` (`publish`/`rollback`/`unpublish`), `from_version_id`, `to_version_id`, `reason varchar(1000)`, `released_by`, `released_at` | append-only (mẫu DDL §3.13) |

F104 chỉ đọc (`ConfigReader.Active(type, at)` = version `published` có `effective_from ≤ at` mới nhất); seed `seed/matchmaking_config.json`. Ghi ở F108.

### Projection `ref` (cập nhật bởi consumer)

| Bảng | Trạng thái | Cột dùng |
|---|---|---|
| `ref.students` | Có sẵn | `grade_level`, `school_id`, `status`, `deleted_at` |
| `ref.enrollments` | Thêm vào competition (§2.17 chưa liệt kê) | `student_id`, `class_id`, `status` — cost cùng lớp, scope `class` (F106, F107) |
| `ref.schools` | Thêm cột `settings_subset jsonb` | `access_hours`, `disabled_leaderboard_definitions`, `allow_cross_school_listing` |
| `ref.tenants` | Có sẵn `settings_subset` | `cross_school_matchmaking` |
| `ref.player_profiles` | Có sẵn | `nickname`, `avatar_thumb_key` cho proposal |
| `ref.blocks` | Có sẵn (F091; consumer `user.blocked`/`user.unblocked` của F091 ghi) | `blocker_id`, `blocked_id`, `active` (H6) |

## 2. Vòng đời / trạng thái

Ticket (Valkey): `queued → proposed → matched`; `proposed → queued` (bên khác decline/timeout, giữ `enqueued_at`); `proposed → cancelled` (chính mình); `queued → cancelled | expired`. Duel: `pending → accepted | declined | expired | cancelled`. Match (F104 tạo): `lobby` (F105 chuyển tiếp).

Key Valkey (prefix thật `ew:competition:`, [VALKEY_DESIGN §3.1](../../docs/07-backend/VALKEY_DESIGN.md)): `mm:queue:<mode>:<scope>:<grade_key>:<subject>[:<topic>]` ZSET · `mm:ticket:<ticket_id>` HASH TTL 10 phút · `mm:user:<student_id>` STRING NX · `mm:proposal:<proposal_id>` HASH TTL 30 s · `mm:lock:<queue_key>` · `mm:queues` SET · `mm:cooldown:<student_id>` STRING TTL theo `mm_ticket_rate`. VALKEY_DESIGN chưa có đoạn `<scope>` và `cooldown` → task cập nhật.

## 3. Quy tắc kiểm tra dữ liệu

- H1–H7 theo [MATCHMAKING §2.1](../../docs/03-social-multiplayer/MATCHMAKING.md#21-hard-constraints-thành-phần-của-queue-key--không-nới-theo-thời-gian); `grade_key` không bao giờ gộp `g1…g5` với khối khác.
- `ranked = true` chỉ khi `matchmaking_type ∈ {skill, subject, grade(tùy cấu hình), team}` và không bot, không duel.
- Mọi người tham gia một trận cùng tenant, trừ `scope_kind = platform`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `match.created` | `CreateMatch` commit | `specs/events/payloads/competition/match.created.v1.schema.json` (Cần thêm) |
| `duel.sent` / `duel.accepted` / `duel.declined` / `duel.expired` | Chuyển trạng thái duel | `specs/events/payloads/competition/duel.<verb>.v1.schema.json` (Cần thêm) |
| `audit.recorded` | Tạo trận, duel | envelope chuẩn |
