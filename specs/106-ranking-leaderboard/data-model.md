# Data Model: F106 — Ranking Glicko-2, tier, season và 13 bảng xếp hạng

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_competition`, schema `competition`. T = mixin `Base` + RLS; TA = tenant append-only; G = platform.

## 1. Entity

### Rating — `competition.ratings` (Có sẵn §2.10, T + Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `student_id`, `mode`, `subject_id`, `season_id` | uuid / `varchar(64)` | PK `(student_id, mode, subject_id, season_id)` | Có sẵn; `mode` CHECK `IN ('battle_1v1','team_battle','timed_quiz')` |
| `tenant_id`, `school_id` | uuid | NOT NULL, RLS | Có sẵn |
| `rating`, `rd`, `volatility` | numeric(9,4) | RD trong [RD min, RD max] | Có sẵn — không bao giờ trả học sinh |
| `games_played` | int | ≥ 0 | Có sẵn |
| `tier_code` | `varchar(64)` | NULL khi provisional | Thêm cột |
| `peak_tier_code` | `varchar(64)` | NULL | Thêm cột (thưởng cuối mùa) |
| `last_played_at` | timestamptz | | Thêm cột (job RD) |
| `rule_version_id` | uuid | NOT NULL | Thêm cột |
| `updated_at`, `version` | | | mixin |

### RatingChange — `competition.rating_changes` (Có sẵn §2.10, TA)

`match_id`, `student_id` PK · `tenant_id`, `school_id` · `mode varchar(64)`, `subject_id`, `season_id` (Thêm) · `before jsonb`, `after jsonb` (`{rating, rd, volatility, tier_code}`, ≤ 2048 byte) · `rule_version_id` (Thêm) · `created_at`.

### CompetitionPointEntry — `competition.competition_point_ledger` (Có sẵn §2.16, TA)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `entry_id` | UUIDv7 | PK |
| `tenant_id`, `school_id`, `student_id`, `season_id`, `subject_id` | uuid | NOT NULL |
| `mode` | `varchar(64)` | |
| `amount` | int | có thể 0 (capped) hoặc âm (abandon), tổng ngày không dưới 0 |
| `source_type` | `varchar(64)` | CHECK `IN ('match','tournament')` |
| `source_id` | uuid | **UNIQUE `(source_type, source_id, student_id)`** |
| `reason` | `varchar(64)` | CHECK `IN ('completed','win','draw','correct','upset','abandon','capped','tournament')` — Thêm cột |
| `rule_version_id` | uuid | Thêm cột |
| `day_key` | date (ICT) | Thêm cột — trần ngày |
| `created_at` | timestamptz | |

`competition.competition_points` (Có sẵn, T): read model tổng `(student_id, season_id, scope_key)`.

### LeaderboardContribution — `competition.leaderboard_contributions` (Mới, TA)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `contribution_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id`, `student_id` | uuid | NOT NULL | |
| `class_id` | uuid | NULL | Lớp tại thời điểm (từ `ref.enrollments`) |
| `metric` | `varchar(64)` | CHECK `IN ('effort_points','xp','quest_completed','team_points')` | |
| `amount` | int | ≥ 0 | sau trần |
| `source_type` | `varchar(64)` | | `activity_completed`, `review_activity`, `active_day`, `xp_awarded`, `quest_completed`, `team_match` |
| `source_id` | `varchar(64)` | UNIQUE `(metric, source_type, source_id, student_id)` | |
| `day_key` | date | | |
| `rule_version_id` | uuid | NULL | |
| `occurred_at` | timestamptz | | |

Index `(tenant_id, metric, day_key, student_id)`.

### LeaderboardDefinition — `competition.leaderboard_definitions` (Có sẵn §2.10, G + Thêm cột)

`leaderboard_id` PK · `code varchar(64)` (Thêm, UNIQUE partial) · `metric varchar(64)` · `scope_type varchar(64)` CHECK `IN ('global','tenant','school','class','grade','friends','team','tournament')` · `scope_id` (NULL = theo người xem) · `period varchar(64)` CHECK `IN ('daily','weekly','monthly','season','all_time')` · `filters jsonb` (`subject_id`, `topic_id`, `mode`; ≤ 2048) (Thêm) · `grade_levels smallint[]` (Thêm) · `display jsonb` (`class_goal_target`, `highlight_threshold`) (Thêm) · `config_version_id` (Thêm) · `status varchar(64)` (`active`/`retired`) (Thêm). CHECK: `NOT (scope_type IN ('class','school','grade') AND metric = 'xp')`; `period = 'all_time'` chỉ với `xp`/`quest_completed`.

### LeaderboardSnapshot — `competition.leaderboard_snapshots` (Có sẵn §2.10, T/G, P tháng)

`leaderboard_id`, `period_key varchar(64)`, `rank int`, `student_id`, `value numeric(12,2)` (Có sẵn) · `tenant_id`, `school_id`, `scope_id`, `captured_at` (Thêm). UNIQUE `(leaderboard_id, scope_id, period_key, student_id)`. Giữ 3 năm.

### Season — `competition.seasons` (Có sẵn §2.10, G + Thêm cột) · SeasonResult — `competition.season_results` (Mới, TA)

| Bảng | Cột |
|---|---|
| `seasons` | `season_id`, `name varchar(128)`, `starts_at`, `ends_at`, `rules jsonb` (Có sẵn) · `code varchar(64)`, `theme_code varchar(64)`, `status varchar(64)` CHECK `IN ('scheduled','active','ended')`, `config_version_id` (Thêm) |
| `season_results` | `season_id`, `student_id`, `mode`, `subject_id` PK · `tenant_id`, `school_id` · `final_rating`, `final_rd` numeric(9,4) · `peak_tier_code varchar(64)` · `games_played int` · `competition_points int` · `created_at` |

## 2. Vòng đời / trạng thái

Season: `scheduled → active → ended`. Period bảng: mở (Valkey) → đóng (snapshot PG, key TTL hết kỳ + 7 ngày). Rating: provisional → có tier.

Valkey (Rebuildable, [VALKEY_DESIGN §3.1](../../docs/07-backend/VALKEY_DESIGN.md)): `ew:competition:lb:<tenant_id>:<leaderboard_id>:<scope_id>:<period_key>` ZSET (score = điểm × 1e6 − thời điểm đạt để phá hòa) + `:meta` HASH `{rebuilt_at, source_version, checksum}`. Key theo VALKEY_DESIGN (tenant đứng đầu, RB-412).

## 3. Quy tắc kiểm tra dữ liệu

- Glicko-2 và tham số: [COMPETITION_SYSTEM §7.2–§7.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#72-tham-số-confirmed); rating chỉ cập nhật khi `match.completed.rated = true`.
- Bảng điểm, trần: [COMPETITION_SYSTEM §8.3, §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#83-competition-points-confirmed); `effort_points` bỏ attempt `rapid_guess_flagged`.
- Visibility: [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#94-privacy-và-an-toàn-trẻ-em) theo khối người xem.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `rating.updated` | `ApplyMatchRating` commit | `specs/events/payloads/competition/rating.updated.v1.schema.json` (Cần thêm) — `student_id`, `mode`, `subject_id`, `season_id`, `rating`, `rd`, `tier_code`, `provisional` (chỉ consumer server) |
| `competition_points.awarded` | Sau ghi sổ cái | `.../competition_points.awarded.v1.schema.json` (Cần thêm) |
| `leaderboard.period_closed` | Đóng kỳ | `.../leaderboard.period_closed.v1.schema.json` (Cần thêm) — `leaderboard_id`, `scope_id`, `period_key`, `top[]` (maxItems 100) |
| `season.started` / `season.ended` | Job mùa | `.../season.started.v1.schema.json`, `.../season.ended.v1.schema.json` (Cần thêm) |
