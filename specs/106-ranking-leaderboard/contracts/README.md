# Contracts: F106 — Ranking Glicko-2, tier, season và 12 bảng xếp hạng

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/competition.v1.yaml`, `x-service: competition`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getMyRatings` | `GET /api/v1/competition/ratings/me?season_id=` | `competition.v1.yaml` (`Rating`) | học sinh (`game_player`) | Cần sửa |
| `listSeasons` | `GET /api/v1/competition/seasons` | — | học sinh / nhân sự (mọi role) | Cần thêm |
| `getSeason` | `GET /api/v1/competition/seasons/{season_id}` | — | học sinh / nhân sự | Cần thêm |
| `listLeaderboards` | `GET /api/v1/leaderboards` | — | học sinh (`game_player`) / nhân sự `lms.student.read` | Cần thêm |
| `getLeaderboard` | `GET /api/v1/leaderboards/{leaderboard_id}?scope_id=&period_key=&limit=&cursor=` | `competition.v1.yaml` (`LeaderboardPage`) | học sinh (scope chứa mình) / nhân sự `lms.student.read` | Cần sửa |
| `getMyLeaderboardPosition` | `GET /api/v1/leaderboards/{leaderboard_id}/me?scope_id=&period_key=` | `competition.v1.yaml` | học sinh | Cần sửa |
| `getLeaderboardHistory` | `GET /api/v1/leaderboards/{leaderboard_id}/history?scope_id=` | — | học sinh (scope chứa mình) / nhân sự `lms.student.read` | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (cố ý: không cấp Game Rating cho module khác, FR-RNK-002) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `competition.tier.updated` (`mode`, `subject_id`, `tier_code`, `provisional`, `games_played`; không số rating) | server → client | Cần thêm |
| `leaderboard.position.updated` | — | Không làm (không đẩy thông báo "bị vượt", COMPETITION_SYSTEM §9.4) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `rating.updated` | Phát | `specs/events/payloads/competition/rating.updated.v1.schema.json` | Cần thêm |
| `competition_points.awarded` | Phát | `specs/events/payloads/competition/competition_points.awarded.v1.schema.json` | Cần thêm (+ catalog EVENT_DESIGN §4.8) |
| `leaderboard.period_closed` | Phát | `specs/events/payloads/competition/leaderboard.period_closed.v1.schema.json` | Cần thêm (+ catalog) |
| `season.started`, `season.ended` | Phát (module game tiêu thụ) | `specs/events/payloads/competition/season.*.v1.schema.json` | Cần thêm |
| `match.completed` | Tiêu thụ (F105) | `specs/events/payloads/competition/match.completed.v1.schema.json` | Cần thêm (F105) |
| `activity.graded` | Tiêu thụ (learning) | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Cần sửa (thêm trường, §6) |
| `xp.awarded`, `quest.completed` | Tiêu thụ (game) | `specs/events/payloads/game/…` | Cần thêm (thuộc F074/F072) |
| `competition_config.published` | Tiêu thụ nội bộ (F108) | `specs/events/payloads/competition/competition_config.published.v1.schema.json` | Cần thêm (F108) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/ranking-rules.schema.json`, `leaderboard-definition.schema.json`, `season.schema.json` | Payload cấu hình F106 đọc | Cần thêm (F108 làm; F106 dùng seed hợp lệ) |

## 6. Delta cần thực hiện

- `competition.v1.yaml`:
  - `Rating`: bỏ `rating`, `rd`; thêm `tier_code` (string, maxLength 64), `tier_name` (string, maxLength 128, theo `Accept-Language`), `provisional` (boolean), `provisional_games_required` (integer), `peak_tier_code`; `getMyRatings` thêm query `season_id`.
  - `LeaderboardPage.scope_type` bỏ `class` (ADR-0095) — **Đã áp 2026-11-21 (WP-S)** bởi lead trong `competition.v1.yaml` và bundle `eduworld-api.v1.yaml`.
  - `LeaderboardPage` (còn lại, chưa áp): `metric` enum = `competition_points`, `effort_points`, `grade_goal`, `xp`, `quest_completed` (đổi từ `quests_completed`), `rating`, `team_points`; `scope_type` enum = `global`, `tenant`, `school`, `grade`, `friends`, `team`, `tournament` (bỏ `organization`, `subject`, `topic`, `game_mode` — chuyển sang `filters`); thêm `visibility` (enum `ranked`, `top_n_percentile`, `grade_goal`), `stale` (boolean), `me` (`{rank, value, percentile_bucket}`), `around_me[]`, `grade_goal {grade_level, current, target, my_contribution, contributors_count (nullable — `null` khi < 5 học sinh)}`, `filters {subject_id, topic_id, mode}`; `entries[].player` chỉ `PlayerCard` (nickname/avatar); với `visibility = grade_goal` thì `entries` rỗng. **Bỏ** so với bản trước ADR-0095: metric/visibility `class_goal`, `class_goal {current, target}`, `highlight_players[]`, `class_name` (ADR-0095 mục 5).
  - Thêm param `scope_id` (uuid) cho `getLeaderboard`, `getMyLeaderboardPosition`; thêm `listLeaderboards` (`{data: LeaderboardSummary[]}` — `leaderboard_id`, `code`, `title` maxLength 128, `metric`, `scope_type`, `period`, `scope_id`), `getLeaderboardHistory` (`{data: {period_key, entries[]}[] , page}`), `listSeasons`, `getSeason` (`Season {season_id, code, name maxLength 128, theme_code, starts_at, ends_at, status, my_peak_tier_code}`).
- `eduworld-realtime.v1.yaml`: thêm `competition.tier.updated`.
- `activity.graded.v1`: thêm `rapid_guess_flagged` (boolean), `recommendation_id` (uuid|null) — cần nhóm learning (F031/F032) đồng ý.
- Event payload schema mới (§4); bổ sung catalog EVENT_DESIGN §4.8 (`competition_points.awarded`, `leaderboard.period_closed`).
- `leaderboard-definition.schema.json` (F108): `scope_type` không có `class`, metric `grade_goal` thay `class_goal`, `display.grade_goal_target` thay `class_goal_target`/`highlight_threshold` (ADR-0095).
- Tài liệu: DATABASE_DESIGN §2.10/§2.16 (cột mới, `leaderboard_contributions.grade_level`, `season_results`). Tên `rating_changes`, `leaderboard_id`, key Valkey theo VALKEY_DESIGN đã thống nhất trong tài liệu sở hữu (RB-412).
