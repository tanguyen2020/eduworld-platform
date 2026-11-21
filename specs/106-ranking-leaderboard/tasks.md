# Tasks: F106 — Ranking Glicko-2, tier, season và 12 bảng xếp hạng

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `ranking`, `points`, `leaderboard`, `season`; đăng ký consumer (durable `competition.<handler>`), job, handler trong composition root — file: `src/backend/modules/competition/module.go` (nối vào `src/backend/cmd/core-api`, `src/backend/cmd/worker`)
- [ ] T002 [P] Seed Ranking Rules, mùa "pre-season", danh mục leaderboard mặc định — file: `src/backend/modules/competition/seed/ranking_rules.json`, `src/backend/modules/competition/seed/leaderboard_definitions.json`, `src/backend/modules/competition/seed/seasons.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta OpenAPI: sửa `Rating`, `LeaderboardPage`, `getMyRatings`, `getLeaderboard`, `getMyLeaderboardPosition`; thêm `listLeaderboards`, `getLeaderboardHistory`, `listSeasons`, `getSeason` — file: `specs/openapi/modules/competition.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`
- [ ] T004 [P] AsyncAPI `competition.tier.updated` — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T005 [P] Payload schema `rating.updated.v1`, `competition_points.awarded.v1`, `leaderboard.period_closed.v1`, `season.started.v1`, `season.ended.v1`; catalog EVENT_DESIGN §4.8 — file: `specs/events/payloads/competition/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint`
- [ ] T006 [P] Delta `activity.graded.v1` thêm `rapid_guess_flagged`, `recommendation_id` (phối hợp nhóm learning) — file: `specs/events/payloads/learning/activity.graded.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T007 ent schema: cột mới `ratings`, `rating_changes`, `competition_point_ledger`, `leaderboard_definitions`, `leaderboard_snapshots` (partition tháng), `seasons`; bảng mới `leaderboard_contributions`, `season_results`; CHECK scope ≠ `xp` — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f106_ranking`, `make migrate-lint MOD=competition`
- [ ] T008 [P] Cập nhật DATABASE_DESIGN §2.10/§2.16, COMPETITION_SYSTEM §7.3/§9.3/§14.1 (tên bảng, key Valkey) — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/01-game-design/COMPETITION_SYSTEM.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Rating Glicko-2 sau trận ranked (Priority: P1) 🎯 MVP

**Goal**: Rating cập nhật đúng một lần sau trận ranked.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test F106-AC-01, F106-AC-04: vector Glickman (sai số < 0.01), Illinois hội tụ, team "đối thủ ảo", abandon/forfeit = 0 — file: `src/backend/modules/competition/tests/unit/ranking/glicko2_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Integration test F106-AC-02, F106-AC-03 (TC-CMP-008, TC-CMP-009): replay `match.completed` → 1 lần; unranked/bot/duel/giải/aborted không đổi; `rating.updated` sau commit — file: `src/backend/modules/competition/tests/integration/ranking/apply_match_rating_test.go`
- [ ] T011 [P] [US1] Unit test F106-AC-05, F106-AC-06: provisional theo `rating_provisional`; bước "no games" tăng RD không vượt max, không đổi rating — file: `src/backend/modules/competition/tests/unit/ranking/provisional_rd_test.go`

### Implementation cho User Story 1

- [ ] T012 [P] [US1] Domain `glicko2.Update`, `ProvisionalPolicy` — file: `src/backend/modules/competition/internal/domain/ranking/glicko2.go`, `src/backend/modules/competition/internal/domain/ranking/provisional.go`
- [ ] T013 [US1] Command `ApplyMatchRating` (tx: `ratings` + `rating_changes` + outbox) + consumer `match.completed` — file: `src/backend/modules/competition/internal/application/command/ranking/apply_match_rating.go`, `src/backend/modules/competition/internal/presentation/consumer/match_completed_ranking.go`
- [ ] T014 [US1] Command `InflateInactiveRD` + job tuần; `RatingReader` thật thay `DefaultRatingReader` (F104) — file: `src/backend/modules/competition/internal/application/command/ranking/inflate_inactive_rd.go`, `src/backend/modules/competition/internal/presentation/job/rd_inflation.go`, `src/backend/modules/competition/internal/infrastructure/persistence/rating_reader.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Tier thay số rating, tách khỏi năng lực học tập (Priority: P1)

**Goal**: Học sinh chỉ thấy tier; rating không rời module competition như chỉ số năng lực.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T015 [P] [US2] Unit test F106-AC-08: ngưỡng/tên tier từ `tiers`, hysteresis — file: `src/backend/modules/competition/tests/unit/ranking/tier_test.go` — phải FAIL trước khi code
- [ ] T016 [P] [US2] Contract test F106-AC-07, F106-AC-09: quét response mọi route học sinh + message realtime không có `rating`/`rd`; không có operation facade/trường event cấp rating cho learning — file: `src/backend/modules/competition/tests/contract/no_raw_rating_test.go`

### Implementation cho User Story 2

- [ ] T017 [P] [US2] Domain `TierPolicy` — file: `src/backend/modules/competition/internal/domain/ranking/tier.go`
- [ ] T018 [US2] Query `GetMyRatings` + handler `getMyRatings`; eventmap `rating.updated` → `competition.tier.updated` (chỉ tier) — file: `src/backend/modules/competition/internal/application/query/ranking/get_my_ratings.go`, `src/backend/modules/competition/internal/presentation/http/public/ranking_handler.go`, `src/backend/modules/competition/internal/presentation/realtime/eventmap.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Competition Points thưởng tham gia và nỗ lực (Priority: P1)

**Goal**: Sổ cái điểm thi đấu có trần.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T019 [P] [US3] Unit + integration test F106-AC-10, F106-AC-11: bảng điểm, thua không trừ, abandon không dưới 0, trần ngày/số trận (`capped`), unique nguồn, `competition_points.awarded` — file: `src/backend/modules/competition/tests/integration/points/award_points_test.go` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T020 [US3] Domain `PointTable` + command `AwardMatchPoints` + consumer `match.completed` (durable riêng) — file: `src/backend/modules/competition/internal/domain/points/table.go`, `src/backend/modules/competition/internal/application/command/points/award_match_points.go`, `src/backend/modules/competition/internal/presentation/consumer/match_completed_points.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Bảng xếp hạng an toàn theo khối (Priority: P1)

**Goal**: 12 loại bảng + mục tiêu tập thể của khối (`grade_goal`, ADR-0095), ZSET + sổ cái, visibility theo khối, policy trường, cô lập scope.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T021 [P] [US4] Unit test F106-AC-12, F106-AC-16: definition hợp lệ (12 loại), từ chối `xp` ở scope school/grade, từ chối scope `class`/metric `class_goal`; `effort_points` theo bảng, trần ngày, bỏ `rapid_guess`; `grade_goal` gộp theo (trường, khối tại thời điểm ghi), không rank/nickname, ô < 5 học sinh → `null` — file: `src/backend/modules/competition/tests/unit/leaderboard/definition_effort_test.go` — phải FAIL trước khi code
- [ ] T022 [P] [US4] Unit test F106-AC-17: `VisibilityPolicy` lớp 1–3/4–5/6–9 (lớp 1–3 chỉ `grade_goal` không tên, percentile, ±3) — file: `src/backend/modules/competition/tests/unit/leaderboard/visibility_test.go`
- [ ] T023 [P] [US4] Integration test F106-AC-13, F106-AC-14: dedupe `processed_events` trước ZINCRBY; xóa key → `stale = true` → rebuild khớp sổ cái; job đối chiếu sửa lệch — file: `src/backend/modules/competition/tests/integration/leaderboard/zset_rebuild_test.go`
- [ ] T024 [P] [US4] Integration test (đồng hồ giả) F106-AC-15: đóng kỳ ICT, snapshot top 100 + rank mọi thành viên trường/khối, dòng tổng `grade_goal`, `leaderboard.period_closed` idempotent, singleton lock — file: `src/backend/modules/competition/tests/integration/leaderboard/period_close_test.go`
- [ ] T025 [P] [US4] Integration test F106-AC-18, F106-AC-19: chỉ nickname/avatar (không tên lớp); `allow_cross_school_listing`; sanction bị loại; `disabled_leaderboard_definitions` → ẩn + `404` — file: `src/backend/modules/competition/tests/integration/leaderboard/privacy_policy_test.go`
- [ ] T026 [P] [US4] Test tenant isolation + authorization F106-AC-20, F106-AC-21: học sinh chỉ scope chứa mình; nhân sự `lms.student.read` theo scope; tenant khác → 404; header sai → `403 TENANT_MISMATCH`; nhân sự gọi route game → `403 GAME_ACCESS_STUDENT_ONLY`; tham số vượt `maxLength` → `422` — file: `src/backend/modules/competition/tests/integration/leaderboard/authz_tenant_test.go`

### Implementation cho User Story 4

- [ ] T027 [P] [US4] Domain `LeaderboardDefinition`, `VisibilityPolicy`, `GradeGoal` (gộp trường × khối, che ô < `analytics_min_cell_size`), `PeriodKey`, `EffortRules` — file: `src/backend/modules/competition/internal/domain/leaderboard/`
- [ ] T028 [US4] Adapter Valkey `LeaderboardStore` (ZINCRBY, ZREVRANGE, ZMSCORE friends, meta) — file: `src/backend/modules/competition/internal/infrastructure/cache/leaderboard_store.go`
- [ ] T029 [US4] Command `RecordContribution`, `ClosePeriod`, `RebuildLeaderboard`, `MaterializeDefinitions`; consumer `activity.graded`, `xp.awarded`, `quest.completed`, cộng điểm từ `competition_points.awarded`; job `period_close`, `leaderboard_rebuild` — file: `src/backend/modules/competition/internal/application/command/leaderboard/`, `src/backend/modules/competition/internal/presentation/consumer/activity_graded_effort.go`, `src/backend/modules/competition/internal/presentation/job/period_close.go`
- [ ] T030 [US4] Query `ListLeaderboards`, `GetLeaderboard`, `GetMyLeaderboardPosition`, `GetLeaderboardHistory` + handler (port `SchoolPolicyReader` đọc `ref.schools.settings_subset`) — file: `src/backend/modules/competition/internal/application/query/leaderboard/`, `src/backend/modules/competition/internal/presentation/http/public/leaderboard_handler.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Mùa giải 8 tuần và soft reset (Priority: P2)

**Goal**: Vòng đời mùa, soft reset, lịch sử mùa.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T031 [P] [US5] Unit test F106-AC-23: `SoftReset` theo `season_soft_reset` (RD ≥ ngưỡng, volatility giữ) — file: `src/backend/modules/competition/tests/unit/season/soft_reset_test.go` — phải FAIL trước khi code
- [ ] T032 [P] [US5] Integration test (đồng hồ giả) F106-AC-22, F106-AC-23, F106-AC-24: `season.started/ended` một lần; `season_results` tier cao nhất; rating mùa mới, points từ 0; `listSeasons`/`getSeason` — file: `src/backend/modules/competition/tests/integration/season/season_lifecycle_test.go`

### Implementation cho User Story 5

- [ ] T033 [US5] Domain `SoftReset`; command `StartSeason`, `EndSeason`; job `season_scheduler` — file: `src/backend/modules/competition/internal/domain/season/soft_reset.go`, `src/backend/modules/competition/internal/application/command/season/`, `src/backend/modules/competition/internal/presentation/job/season_scheduler.go`
- [ ] T034 [US5] Query `ListSeasons`, `GetSeason` + handler — file: `src/backend/modules/competition/internal/application/query/season/`, `src/backend/modules/competition/internal/presentation/http/public/season_handler.go`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: Polish & Cross-Cutting

- [ ] T035 [P] Metric/span theo plan.md §Quan sát; alert lệch bảng — file: `src/backend/modules/competition/internal/infrastructure/observability/ranking_metrics.go`
- [ ] T036 [P] Bài đo rebuild 50.000 học sinh (SC-005) — file: `tests/perf/seed_leaderboard.go`
- [ ] T037 [P] CHANGELOG, trạng thái F106 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/106-ranking-leaderboard/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T003–T008) → US1 → US2, US3 (song song) → US4 → US5 → Polish.
- T013, T020 cùng tiêu thụ `match.completed` nhưng durable khác nhau: song song được. T014 thay port của F104 (sau F104 T009). F107 dùng `RatingReader` (seeding); F108 phát `competition_config.published` cho T029/T033.

## Definition of Done

- [ ] Mọi tiêu chí F106-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
