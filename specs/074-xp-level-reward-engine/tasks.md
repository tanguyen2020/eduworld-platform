# Tasks: F074 — XP ledger, level, đường cong XP, danh hiệu, Reward Engine có cap

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `internal/domain/reward`, `internal/domain/progression`, đấu nối handler vào HTTP handler/consumer qua interface có kiểu và depguard cho package mới — file: `src/backend/modules/game/internal/domain/reward/doc.go`, `src/backend/modules/game/.golangci.yml` — đã làm: `internal/domain/reward`, `internal/domain/progression` (depguard/archgen sinh theo tầng, 0 vi phạm)
- [x] T002 [P] Cấu hình đọc tham số `xp_daily_cap`, `xp_repeat_multipliers`, `xp_below_grade_multiplier`, `xp_objective_mastered` từ platform settings (ref `settings_subset`) có giá trị seed — file: `src/backend/modules/game/internal/config/progression.go` — đã làm: `internal/infrastructure/config/progression.go` (env `EW_GAME_XP_*` + giá trị CONFIG_DEFAULTS; chưa có khóa platform settings — Clarifications 2026-11-23)
- [x] T003 [P] Seed config platform: curve mặc định + reward rule cho mỗi `source_code` của PROGRESSION_SYSTEM §3.1 (dạng YAML, nạp qua lệnh `seed-config`) — file: `deploy/seed/game-config/progression-curves/main.yaml`, `deploy/seed/game-config/reward-rules/` — đã làm ở `src/backend/modules/game/seed/progression/` (`curve.default.v1.json`, `reward-rules.v1.json`, 9 rule; JSON như seed hiện hành) nạp bằng `cmd/seed`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Sửa `reward-rule.schema.json`: `source_code`, trigger `activity.graded`/`world.discovery_found`/`world.area_completed`, grant `title`, `caps.bucket` — file: `specs/schemas/reward-rule.schema.json` — kiểm: `make specs-lint`
- [x] T005 [P] Thêm payload `xp.awarded.v1`, `level.reached.v1`, `reward.granted.v1`; cập nhật bảng payload — file: `specs/events/payloads/game/`, `specs/events/README.md` — kiểm: `make specs-lint`
- [x] T006 [P] Phối hợp module learning sửa `activity.graded.v1` (`content_version_id`, `first_try_correct_count`, `recommendation_id`, `content_grade_levels`) (owner F030); xác nhận `learning_objective.mastered.v1` (owner F031) đã có, thiếu → chặn consumer; `lesson.completed.v1` (owner F031) đã có — file: `specs/events/payloads/learning/` — kiểm: `make specs-lint`, `make test-contract` — đã làm: schema + module learning điền `content_version_id`, `recommendation_id`, `first_try_correct_count`; `content_grade_levels` chưa điền (Câu hỏi mở F074-2)
- [x] T007 ent schema `XpLedger` (AppendOnly, thêm `reason_code`, `content_version_id`), `XpDailyCap`, `RewardRule`, `RewardRuleVersion`, `RewardGrant` (thêm `trigger_kind`, `idempotency_key`), `ProgressionCurve`, `ProgressionCurveVersion`, `ProgressionLevel` (thêm `xp_cumulative`), `LevelHistory`; `game_profiles.grade_transition_year` — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f074_progression`, `make migrate-lint MOD=game`
- [x] T008 Migration: unique/index theo data-model, RLS bảng T/TA, `REVOKE UPDATE, DELETE, TRUNCATE` trên `xp_ledger`, `level_history` — file: `src/backend/modules/game/migrations/`
- [x] T009 [P] Test RLS + append-only F074-AC-12: tenant A không đọc ledger B; `UPDATE`/`DELETE` ledger bị từ chối — file: `src/backend/modules/game/tests/integration/progression_rls_test.go`
- [x] T010 [P] `GrantApplierRegistry` (port) + applier `xp`, `title`; lỗi khi loại chưa đăng ký — file: `src/backend/modules/game/internal/application/port/write/grant_applier.go`, `src/backend/modules/game/internal/infrastructure/reward/appliers.go` — đã làm: `port/write/progression.go` (`GrantApplierRegistry`), `infrastructure/reward/appliers.go`
- [x] T039 [P] Delta contract `GameConfigType`: `levels` → `progression-levels` (enum + mọi path `/game-admin/{config_type}`) — file: `specs/openapi/modules/game.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Cộng XP server-side từ sự kiện đã kiểm chứng (Priority: P1) 🎯 MVP

**Goal**: XP từ event, idempotent, có giảm dần, cap, miễn giảm ôn LO tiên quyết.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T011 [P] [US1] Unit test `reward.Engine` cho F074-AC-04, F074-AC-05, F074-AC-06, F074-AC-07 (bảng ca: lặp 1–5 lần, cap còn 0/một phần, câu `rapid_guess`, nội dung thấp 2 khối có/không `recommendation_id`/`assignment`) — file: `src/backend/modules/game/internal/domain/reward/engine_test.go` — phải FAIL trước khi code
- [x] T012 [P] [US1] Integration test F074-AC-01, F074-AC-02, F074-AC-03: mỗi trigger tạo bút toán đúng `source_code`; giao lại cùng `event_id`/cùng khóa nguồn không thêm dòng; ledger + `xp_total` + outbox cùng transaction (rollback → không có gì) — file: `src/backend/modules/game/tests/integration/reward_engine_consumer_test.go`
- [x] T013 [P] [US1] Integration test F074-AC-08: `learning_objective.mastered` lặp và sau decay → một bút toán — file: `src/backend/modules/game/tests/integration/objective_mastered_xp_test.go`
- [x] T040 [P] [US1] Integration test F074-AC-27: `lesson.completed` → bút toán `source_code = lesson.completed`, `source_type = lesson`, `source_id = lesson_version_id`; giao lại không thêm dòng; nội dung thấp 2 khối × `xp_below_grade_multiplier`, `mode = assignment` đầy đủ — file: `src/backend/modules/game/tests/integration/lesson_completed_xp_test.go`
- [x] T014 [P] [US1] Test race cap: 20 event đồng thời của một học sinh không vượt `xp_daily_cap` — file: `src/backend/modules/game/tests/integration/xp_cap_race_test.go`
- [x] T015 [P] [US1] Test tenant isolation consumer F074-AC-11: envelope tenant khác `ref.students` → không ghi, DLQ, metric — file: `src/backend/modules/game/tests/integration/reward_engine_tenant_test.go`
- [x] T016 [P] [US1] Contract/static test F074-AC-09, F074-AC-10: quét `game.v1.yaml` không request học sinh nào có `xp|amount|score`; quét module learning không subscribe `xp.awarded`/`level.reached`; reward rule seed không có trigger đăng nhập — file: `src/backend/modules/game/tests/contract/no_client_xp_test.go`

### Implementation cho User Story 1

- [x] T017 [P] [US1] Domain `reward`: `Rule`, `Engine.Evaluate`, `CapPolicy`, `RepeatPolicy`, `GradePolicy` — file: `src/backend/modules/game/internal/domain/reward/`
- [x] T018 [US1] Command `AwardFromEvent` (khóa `xp_daily_caps`, insert ledger, cập nhật profile, outbox `xp.awarded`) — file: `src/backend/modules/game/internal/application/command/award_from_event.go` — đã làm trong `command/progression.go`
- [x] T019 [US1] Repository ledger/cap + reader rule có cache con trỏ — file: `src/backend/modules/game/internal/infrastructure/persistence/write/xp_ledger_repo.go`, `src/backend/modules/game/internal/infrastructure/persistence/write/reward_rule_reader.go` — đã làm: `persistence/write/progression_repo.go`, `persistence/progressionsql` (không cache Valkey cho rule — Clarifications 2026-11-23)
- [x] T020 [US1] Consumer `game.reward_engine` (FilterSubjects các trigger gồm `ew.evt.learning.lesson.completed`, ánh xạ payload → `Trigger`, NAK khi thiếu projection) + publisher realtime `progression.xp.updated` sau commit — file: `src/backend/modules/game/internal/presentation/consumer/reward_engine.go` — đã làm: hai durable `game.reward_engine` (learning) + `game.reward_engine_game` (game); `minigame.completed` chờ F073 (Câu hỏi mở F074-1)
- [x] T021 [US1] Job `xp-reconcile` hằng đêm (so `xp_total` với Σ ledger, metric + `audit.recorded` khi lệch) — file: `src/backend/modules/game/internal/presentation/job/xp_reconcile.go` — đã làm: `ReconcileXP` (`command/progression_ops.go`), job `presentation/job/progression.go`

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Level, danh hiệu, phần thưởng lên cấp (Priority: P1)

### Tests cho User Story 2

- [x] T022 [P] [US2] Unit test `progression.LevelTable` cho F074-AC-13, F074-AC-15, F074-AC-16 (vượt nhiều cấp, danh hiệu theo khoảng, `max_level`) — file: `src/backend/modules/game/internal/domain/progression/level_table_test.go` — phải FAIL trước khi code
- [x] T023 [P] [US2] Integration test F074-AC-13, F074-AC-14: hai `level.reached`, hai `level_history`, RewardGrant một lần theo `(student_id, level, curve_id)`, đạt lại không nhận lại — file: `src/backend/modules/game/tests/integration/level_up_test.go`

### Implementation cho User Story 2

- [x] T024 [P] [US2] Domain `progression`: `Curve`, `LevelTable`, `Title`, `LevelUp` — file: `src/backend/modules/game/internal/domain/progression/`
- [x] T025 [US2] Mở rộng `AwardFromEvent`: lên cấp → `level_history`, `reward_grants` (`trigger_kind = level_up`), outbox `level.reached`, `reward.granted`, realtime `progression.level.reached` — file: `src/backend/modules/game/internal/application/command/award_from_event.go`

## Phase 5: User Story 3 — Học sinh xem tiến trình và lịch sử XP (Priority: P1)

### Tests cho User Story 3

- [x] T026 [P] [US3] API test F074-AC-17, F074-AC-18: đủ trường, lịch sử không lộ id kỹ thuật, phân trang cursor — file: `src/backend/modules/game/tests/integration/progression_api_test.go` — phải FAIL trước khi code
- [x] T027 [P] [US3] Test authorization + tenant isolation F074-AC-19, F074-AC-20: 4 route × token `school_admin`, `academic_manager`, `tenant_admin`, `safety_moderator`, `system_admin` → 403; header tenant khác → `TENANT_MISMATCH`; `limit`/`cursor` sai → 422 — file: `src/backend/modules/game/tests/integration/progression_authz_test.go`

### Implementation cho User Story 3

- [x] T028 [US3] Query `GetProgression`, `ListXpHistory` (map `source_code` → `source_label` qua i18n), `ListProgressionLevels`, `GetXpStatus` + handler — file: `src/backend/modules/game/internal/application/query/get_progression.go`, `src/backend/modules/game/internal/presentation/http/public/progression.go` — đã làm: `query/progression.go`, `presentation/http/progression.go` (nhãn nguồn vi/en theo `Accept-Language`)

## Phase 6: User Story 4 — Đường cong XP và reward rule có version (Priority: P2)

### Tests cho User Story 4

- [x] T029 [P] [US4] Test module validate F074-AC-23: payload sai schema, ngưỡng không đơn điệu, cap ≤ 0, `source_code` lạ, loại thưởng chưa có applier → 422 chỉ rõ field — file: `src/backend/modules/game/tests/unit/gameconfig_progression_module_test.go` — phải FAIL trước khi code — làm dưới dạng integration `TestProgressionPublishChecks` + unit domain (validate chạy trong lõi phát hành)
- [x] T030 [P] [US4] Integration test F074-AC-21, F074-AC-22: publish curve tăng ngưỡng giữ level 12; curve giảm ngưỡng → `curve-rollout` phát lần lượt `level.reached`, chạy lại không trùng — file: `src/backend/modules/game/tests/integration/curve_rollout_test.go`
- [x] T031 [P] [US4] Integration test F074-AC-24: event trước `effective_from` dùng rule cũ; đổi rule không thu hồi XP — file: `src/backend/modules/game/tests/integration/reward_rule_versioning_test.go`

### Implementation cho User Story 4

- [x] T032 [US4] Module `progression-curves` (validate, `OnPublished` sinh `progression_levels`), `progression-levels` (đọc/ghi đè theo level, permission `game.progression.write`) và `reward-rules` (validate, cập nhật cache) đăng ký `ConfigTypeRegistry` — file: `src/backend/modules/game/internal/infrastructure/gameconfig/progression_curve_module.go`, `src/backend/modules/game/internal/infrastructure/gameconfig/reward_rule_module.go` — đã làm trên lõi phát hành F071 (`command/publish_progression.go`, item type `progression_curve`, `reward_rule`; `ConfigTypeRegistry` thuộc F078)
- [x] T033 [US4] Command `RolloutCurve` + job `curve-rollout` (theo lô, theo tenant, idempotent) — file: `src/backend/modules/game/internal/application/command/rollout_curve.go`, `src/backend/modules/game/internal/presentation/job/curve_rollout.go` — đã làm: `RolloutCurve` (`command/progression_ops.go`), job `presentation/job/progression.go`

## Phase 7: User Story 5 — Chuyển năm học (Priority: P2)

### Tests cho User Story 5

- [x] T034 [P] [US5] Integration test F074-AC-25, F074-AC-26: đổi khối giữ XP/level, quest khối cũ `expired`, lặp event không áp lại; không đổi khối → không thay đổi — file: `src/backend/modules/game/tests/integration/grade_transition_test.go` — phải FAIL trước khi code — đã làm: `TestGradeTransition` (`tests/integration/progression_ops_test.go`)

### Implementation cho User Story 5

- [x] T035 [US5] Command `ApplyGradeTransition` (gọi `QuestProgress.ExpireForGradeChange` của F072, ghi `grade_transition_year`) + consumer `game.grade_transition` — file: `src/backend/modules/game/internal/application/command/apply_grade_transition.go`, `src/backend/modules/game/internal/presentation/consumer/grade_transition.go` — đã làm: `ApplyGradeTransition` (`command/progression_ops.go`) + `Quests.ExpireForGrade` + consumer `game.grade_transition` (`consumer/reward_engine.go`)

## Phase 8: Polish & Cross-Cutting

- [x] T036 [P] Cập nhật DATABASE_DESIGN §2.8a/§2.13 (cột thêm: `reason_code`, `content_version_id`, `trigger_kind`, `idempotency_key`, `xp_cumulative`, `cap_date`, `grade_transition_year`); PROGRESSION_SYSTEM §3.2 (ngày cap ICT), kiểm §6 (`xp_required`), §7 (`progression-curves`, `progression-levels`) khớp; EVENT_DESIGN §4.6; CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/01-game-design/PROGRESSION_SYSTEM.md`, `docs/07-backend/EVENT_DESIGN.md`, `CHANGELOG.md`
- [x] T037 [P] Metric `eduworld_game_xp_*`, `eduworld_game_level_reached_total`, `eduworld_game_ledger_mismatch_total` + span — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go` — đã làm: `observability/progression_metrics.go`
- [ ] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **chưa chạy tay**: cần stack compose riêng (không dùng stack local của owner); các kịch bản 1–13 đã có test tự động tương ứng (bảng dưới)

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Đã làm 2026-11-23.

- [x] T041 **OQ chốt 2026-11-20** (OQ F016-2) Bước gộp ledger XP bằng bản ghi bù trong `mergeUserData` của module game, idempotent theo `merge_id` — file: `src/backend/modules/game/internal/application/command/` — kiểm: `make test-integration MOD=game` — cỡ S — đã làm: `Progression.MergeXP` gọi trong `StudentLifecycle.Merge` (test `TestMergeXPLedger`)

## Nhận từ F106 (owner chốt 2026-11-22)

- [x] T042 Khi có schema `xp.awarded` (F074) và `quest.completed` (F072): thêm consumer competition ghi contribution cho bảng `friends_xp_weekly` (F106 đã seed definition, hiện rỗng) — integration test competition — cỡ S — đã làm: consumer `competition.leaderboard_xp` + `Ranking.RecordXP` (test `TestF074FriendsXPWeekly`)

## Bằng chứng: tiêu chí → test (2026-11-23)

| Tiêu chí | Test (file) | Kết quả |
|---|---|---|
| F074-AC-01 | `TestRewardEngineTriggers` (tests/integration/reward_engine_consumer_test.go) — 6/7 trigger; `minigame.completed`: `TestTriggerMiniGameCompleted` (internal/presentation/consumer, F073 — F074-1 đã đóng) | pass |
| F074-AC-02 | `TestRewardEngineReplay`; `TestRewardEngineTenantIsolation` (processed_events qua NATS thật) | pass |
| F074-AC-03 | `TestRewardEngineOneTransaction` (rollback, cache = Σ ledger, outbox), `TestXPReconcile` (cảnh báo, không tự sửa) | pass |
| F074-AC-04 | `TestRepeatMultipliers` (domain/reward/engine_test.go), `TestRewardEngineRepeatMultipliers` | pass |
| F074-AC-05 | `TestCaps` (unit), `TestRewardEngineDailyCap` (bút toán 0 XP `capped`, push `progression.xp.updated`), `TestXPCapRace` (20 event đồng thời) | pass |
| F074-AC-06 | `TestFirstTryCorrect` (unit), `TestRewardEngineFirstTryCorrect` | pass |
| F074-AC-07 | `TestBelowGrade` (unit), `TestRewardEngineBelowGrade` | pass |
| F074-AC-08 | `TestObjectiveMasteredOnce` (tests/integration/objective_mastered_xp_test.go) | pass |
| F074-AC-09 | `TestNoClientXP` (tests/contract/no_client_xp_test.go), `TestRuleValidate` (unit) | pass |
| F074-AC-10 | `TestProgressionDoesNotTouchLearning` (tests/contract/no_client_xp_test.go) | pass |
| F074-AC-11 | `TestRewardEngineTenantIsolation` (NATS JetStream thật: retry → DLQ, metric `eduworld_game_consumer_student_unknown_total`) | pass |
| F074-AC-12 | `TestProgressionRLSAndAppendOnly` (tests/integration/progression_rls_test.go) | pass |
| F074-AC-13 | `TestApplyMultipleLevels` (unit), `TestLevelUpMultipleLevelsAndRewards` | pass |
| F074-AC-14 | `TestLevelUpMultipleLevelsAndRewards` (đạt lại sau điều chỉnh không nhận lại) | pass |
| F074-AC-15 | `TestTitles` (unit), `TestProgressionAPI` (nhãn vi/en) | pass |
| F074-AC-16 | `TestMaxLevel` (unit + integration) | pass |
| F074-AC-17 | `TestProgressionAPI` | pass |
| F074-AC-18 | `TestProgressionAPI` (phân trang cursor, không id kỹ thuật) | pass |
| F074-AC-19 | `TestProgressionAuthzAndValidation`, `TestPlayerRoutesStaffDenied` | pass |
| F074-AC-20 | `TestProgressionAuthzAndValidation` | pass |
| F074-AC-21 | `TestFloorProtection` (unit), `TestCurveRollout` | pass |
| F074-AC-22 | `TestCurveRollout` (lần lượt từng `level.reached`, chạy lại không trùng) | pass |
| F074-AC-23 | `TestValidate` (domain/progression), `TestRuleValidate` (domain/reward), `TestValidate` (progressionschema), `TestProgressionPublishChecks` (422 chỉ rõ field, curve thứ hai 409) | pass |
| F074-AC-24 | `TestRewardRuleVersioning` | pass |
| F074-AC-25 | `TestGradeTransition` | pass |
| F074-AC-26 | `TestGradeTransition` (không đổi khối: không thay đổi, không thông điệp) | pass |
| F074-AC-27 | `TestLessonCompletedXP` | pass |
| T041 | `TestMergeXPLedger` | pass |
| T042 | `TestF074FriendsXPWeekly` (modules/competition/tests/integration) | pass |

## Dependencies & Execution Order

- Setup → Foundational (T006 cần module learning; T007–T008 trước mọi test integration) → US1 → US2 → US3 (song song US2 sau T018) → US4 → US5 → Polish.
- US5 cần domain quest của F072; US1 cần event payload của F071–F073.

## Definition of Done

- [x] Mọi tiêu chí F074-AC-* có test pass (ghi mã test/TC) — bảng trên; F074-AC-01 phần `minigame.completed` chờ F073
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game`; `make test-contract` xanh với module learning
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — test double chỉ trong test (`progPush`), có nhãn
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
