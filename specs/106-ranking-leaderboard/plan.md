# Implementation Plan: F106 — Ranking Glicko-2, tier, season và 13 bảng xếp hạng

**Branch**: `106-ranking-leaderboard` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Ba module trong module `competition`: `ranking` (hàm thuần `glicko2.Update`, tier, provisional; consumer `match.completed` cập nhật `ratings` + `rating_changes` + outbox `rating.updated` trong một transaction; job RD hằng tuần), `points` (sổ cái Competition Points theo Ranking Rules, trần ngày), `leaderboard` (definition materialize từ cấu hình publish; consumer cộng điểm vào Valkey ZSET sau khi ghi sổ cái/`processed_events`; query áp `leaderboard_visibility` + policy trường; job đóng kỳ/snapshot; job rebuild/đối chiếu) và `season` (job chuyển mùa, soft reset, `season_results`). Event nguồn: `match.completed` (F105), `activity.graded` (learning — `effort_points`), `xp.awarded`, `quest.completed` (game). F106 thay `DefaultRatingReader` của F104 bằng reader đọc `ratings`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go (JetStream consumer), go-redis, `src/backend/pkg/outbox`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/tenancy`, `src/backend/pkg/clock` |
| Storage | PostgreSQL `eduworld_competition` (`competition`, `ref`); Valkey `ew:competition:lb:*` (Rebuildable) |
| Testing | go test (vector Glickman, property test tier/visibility), testcontainers (PostgreSQL, Valkey, NATS), đồng hồ giả cho kỳ/mùa |
| Target Platform | Linux container (module `competition` nạp trong `core-api` + `worker-events`, `worker-jobs`) |
| Project Type | backend module |
| Performance Goals | `getLeaderboard` p95 theo baseline API ([SYSTEM_ARCHITECTURE §7](../../docs/06-architecture/SYSTEM_ARCHITECTURE.md#7-non-functional-baseline), đọc ZSET); rebuild 50.000 học sinh ≤ 5 phút (SC-005) |
| Constraints | AGENTS R8–R10, R14, R16, R19, R20, R26–R28 · ADR-0004, ADR-0018, ADR-0027, ADR-0042, ADR-0054 |
| Scale/Scope | `leaderboard_visibility`, friends ≤ 200; snapshot partition tháng, giữ 3 năm ([DATABASE_DESIGN §5](../../docs/07-backend/DATABASE_DESIGN.md#5-partitioning--retention)) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta (`Rating`, `LeaderboardPage`, `listLeaderboards`, `getLeaderboardHistory`, `listSeasons`, `getSeason`, event payload, `activity.graded` thêm trường) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module competition; dữ liệu module khác qua event (`activity.graded`, `xp.awarded`, `quest.completed`, `school.updated`) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới T/TA có `tenant_id` + RLS; key Valkey bắt đầu bằng tenant; không hard-delete |
| IV Server-authoritative | ✅ | Rating/points chỉ từ event server; consumer idempotent; không endpoint nhận điểm |
| V Xác thực, phân quyền | ✅ | Học sinh qua `game_player` chỉ scope chứa mình; nhân sự `lms.class.read` theo scope |
| VI An toàn trẻ em | ✅ | Tier thay số; lớp 1–3 không thứ hạng; chỉ nickname; không XP ở bảng lớp; không thông báo "bị vượt" |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số Ranking Rules/definition từ cấu hình publish, tên theo CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi `F106-AC-*` có task test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/106-ranking-leaderboard/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/modules/competition/
├── ent/schema/rating.go, rating_change.go, competition_point_entry.go, competition_points.go,
│   leaderboard_contribution.go, leaderboard_definition.go, leaderboard_snapshot.go, season.go, season_result.go
├── internal/domain/
│   ├── ranking/glicko2.go       # Update(player, opponents, results) — thuần, Illinois
│   ├── ranking/tier.go          # ngưỡng + hysteresis; provisional
│   ├── points/table.go          # bảng Competition Points, trần ngày
│   ├── leaderboard/definition.go, visibility.go, period.go, effort.go
│   └── season/soft_reset.go
├── internal/application/
│   ├── command/ranking/apply_match_rating.go, inflate_inactive_rd.go
│   ├── command/points/award_match_points.go
│   ├── command/leaderboard/record_contribution.go, close_period.go, rebuild_leaderboard.go, materialize_definitions.go
│   ├── command/season/start_season.go, end_season.go
│   ├── query/ranking/get_my_ratings.go
│   ├── query/leaderboard/list_leaderboards.go, get_leaderboard.go, get_my_position.go, get_history.go
│   ├── query/season/list_seasons.go, get_season.go
│   └── port/RatingReader (thật, thay F104), LeaderboardStore, SchoolPolicyReader
├── internal/infrastructure/cache/leaderboard_store.go   # ZINCRBY, ZREVRANGE, ZMSCORE, meta
├── internal/presentation/
│   ├── consumer/match_completed_ranking.go, match_completed_points.go, activity_graded_effort.go, xp_awarded.go, quest_completed.go
│   ├── job/rd_inflation.go, period_close.go, leaderboard_rebuild.go, season_scheduler.go
│   └── http/public/ranking_handler.go, leaderboard_handler.go, season_handler.go
└── tests/{unit,integration,contract}/{ranking,leaderboard,season}/
```

**Structure Decision**: Glicko-2, tier, visibility, soft reset là hàm thuần (unit test vector chuẩn). Mỗi metric một consumer riêng (durable `competition.<handler>`) để lỗi một nguồn không chặn nguồn khác.

## Phase 0: Research

Không có. Công thức Glicko-2, tier, point table, effort points, visibility, kiến trúc ZSET + sổ cái đã định ở [COMPETITION_SYSTEM §7–§9](../../docs/01-game-design/COMPETITION_SYSTEM.md#7-game-rating--glicko-2-adr-0018).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `glicko2.Update`, `TierPolicy`, `ProvisionalPolicy`, `PointTable`, `EffortRules`, `LeaderboardDefinition` (invariant: scope class/school/grade ≠ `xp`), `VisibilityPolicy`, `PeriodKey` (ICT), `SoftReset` | Thuần |
| application | Command `ApplyMatchRating`, `InflateInactiveRD`, `AwardMatchPoints`, `RecordContribution`, `ClosePeriod`, `RebuildLeaderboard`, `MaterializeDefinitions`, `StartSeason`, `EndSeason` · Query `GetMyRatings`, `ListLeaderboards`, `GetLeaderboard`, `GetMyLeaderboardPosition`, `GetLeaderboardHistory`, `ListSeasons`, `GetSeason` | Command (`command/`, `UnitOfWork`) / query (`query/`, chỉ đọc) tách, gọi qua interface có kiểu |
| infrastructure | ent repo; Valkey store; SQL đọc snapshot | Key `ew:competition:lb:<tenant_id>:<leaderboard_id>:<scope_id>:<period_key>` |
| presentation | Handler `getMyRatings`, `listLeaderboards`, `getLeaderboard`, `getMyLeaderboardPosition`, `getLeaderboardHistory`, `listSeasons`, `getSeason`; consumer; job | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F105 (`match.completed`), F104 (port `RatingReader`), F031 (`activity.graded` có cờ rapid guess — delta), F074/F072 (`xp.awarded`, `quest.completed`), F015/F052 (`school.updated` có `settings_subset`).
- Facade: không gọi.
- Event phát: `rating.updated`, `competition_points.awarded`, `leaderboard.period_closed`, `season.started`, `season.ended`. Tiêu thụ: `match.completed`, `activity.graded`, `xp.awarded`, `quest.completed`, `competition_config.published` (F108, materialize definition/season).
- `rating.updated` chứa `rating`, `rd` cho consumer server; eventmap realtime chỉ đẩy tier tới client.

### Bảo mật, tenant, idempotency, audit

- Học sinh: `game_player`, chỉ scope chứa mình (lớp từ `ref.enrollments`, trường từ `ref.students`, friends từ projection bạn bè).
- Nhân sự: `lms.class.read`, scope trường/tenant của UserRole; `show_real_names` chỉ trong trường thuộc scope, audit `leaderboard.real_names_viewed`.
- Idempotency: consumer qua `processed_events`; sổ cái unique theo nguồn; snapshot unique `(leaderboard_id, scope_id, period_key, student_id)`.
- Audit: `season.started/ended`, `leaderboard.period_closed` (hệ thống).

### Quan sát

- Metric: `eduworld_rating_updates_total{mode}`, `eduworld_points_capped_total`, `eduworld_ldb_rebuild_total{reason}`, `eduworld_ldb_drift_detected_total`, `eduworld_ldb_period_close_seconds`.
- Span: `ApplyMatchRating`, `RecordContribution`, `ClosePeriod`, `RebuildLeaderboard`.

Ước lượng: 2–3 sprint (backend 2 dev) [Confirmed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
