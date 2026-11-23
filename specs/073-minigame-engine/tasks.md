# Tasks: F073 — Mini-game Engine: `MiniGameConfig`, plugin `minigame`, scene Godot 11 loại

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Seed 2 cấu hình (matching Toán lớp 3, true_false) + quest mẫu "Phiên chợ của bác Gấu Nâu" có bước `minigame`, gán 3 tenant demo — file: `src/backend/modules/game/seed/minigames/` — đã làm: `seed/minigames/` (2 cấu hình, quest "Phiên chợ của bác Gấu Nâu", gán 3 tenant demo); `cmd/seed` không publish (cần facade learning — Clarifications 2026-11-23), `TestMiniGameSeedFiles` publish với facade
- [x] T002 [P] Fixture render payload 11 loại cho Godot — file: `src/game/tests/fixtures/minigames/`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Sửa `startMiniGame`, `MiniGameSession`, `ActivityResponseResult` (`game_points`, `streak`), kết quả submit (`game_score`, `completion_met`) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make gen MOD=learning`
- [x] T004 [P] Thêm `validateMinigameQuestionSource` (`x-allowed-callers: [game]`), `ActivityRef.content_type` thêm `minigame_config` — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`, `make gen MOD=game`
- [x] T005 [P] Payload event `minigame.completed`; đề xuất khối `minigame_config` cho `content.published` (game, owner F071) và khối `minigame` cho `activity.graded` (owner F030) — file: `specs/events/payloads/game/`, `specs/events/payloads/learning/activity.graded.v1.schema.json` — kiểm: `make specs-lint`
- [x] T006 ent schema game `MiniGameConfig`, `MiniGameConfigVersion` (cột tách), `MiniGamePlay` — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f073_minigame`, `make migrate-lint MOD=game`
- [x] T007 [P] ent schema learning `RefMiniGameConfigVersion`, cột `assessment.attempts.game_score` — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f073_minigame`, `make migrate-lint MOD=learning`
- [x] T008 Migration RLS `minigame_plays`, CHECK `minigame_type`/jsonb — file: `src/backend/modules/game/migrations/` — đã làm trong `20261230000773_f073_minigame.sql`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Cấu hình mini-game hợp lệ và phát hành (Priority: P1) 🎯 MVP

**Goal**: Chỉ cấu hình hợp lệ được publish và đến được learning.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit test bảng tương thích (sinh từ MINIGAME_ENGINE §2) + schema F073-AC-01, F073-AC-03 (không có trường câu hỏi trong payload) — file: `src/backend/modules/game/internal/domain/minigame/compatibility_test.go` — phải FAIL trước khi code — đã làm: bảng tương thích ở plugin learning (`plugins/minigame/compat_test.go`, `TestCompatibilityTableMatchesDoc`); game: `internal/domain/minigame/minigame_test.go`, `internal/infrastructure/minigameschema/minigameschema_test.go`
- [x] T010 [P] [US1] Test F073-AC-02, F073-AC-05: nguồn không tương thích/thiếu câu/question set chưa publish → `ACTIVITY_CONFIG_INVALID` (TC-ACT-003); thiếu `en` → `TRANSLATION_INCOMPLETE`; chuỗi quá dài → 422 — file: `src/backend/modules/game/tests/integration/minigame/publish_validator_test.go` — đã làm: `tests/integration/minigame_test.go` `TestMiniGamePublish`
- [x] T011 [P] [US1] Test learning `validateMinigameQuestionSource`: đúng/sai loại, đủ/thiếu câu; module ngoài `x-allowed-callers` (`game`) import facade → `make arch-check` fail (TC-SVC-005, TC-SVC-006) — file: `src/backend/modules/learning/tests/integration/internal/validate_minigame_source_test.go` — đã làm: `tests/integration/minigame_validate_test.go` `TestValidateMinigameQuestionSource` (learning)
- [x] T012 [P] [US1] Facade test game ↔ learning `validateMinigameQuestionSource` (bên gọi với fake facade, bên nhận theo ví dụ spec internal) — file: `src/backend/modules/game/tests/contract/learning_validate_minigame_source_test.go` — đã làm: `tests/contract/learning_validate_minigame_source_test.go`
- [x] T013 [P] [US1] Test F073-AC-04: publish → outbox `content.published` (`item_type = minigame_config`); consumer learning lưu snapshot bất biến, event lặp không thêm (TC-SVC-003) — file: `src/backend/modules/learning/tests/integration/consumer/minigame_config_projection_test.go` — đã làm: `tests/integration/minigame_projection_test.go` `TestMinigameConfigProjection` (learning)

### Implementation cho User Story 1

- [x] T014 [P] [US1] Domain `Compatibility`, `PublishValidator` — file: `src/backend/modules/game/internal/domain/minigame/` — đã làm: `internal/domain/minigame/minigame.go` (parse, dịch, kiểm, display, rewards_eligible); bảng tương thích không sao chép (ở learning)
- [x] T015 [US1] Đăng ký `item_type = minigame_config` vào `PublishGameContent` (F071) + adapter facade `learning` validate — file: `src/backend/modules/game/internal/infrastructure/content/minigame_item_type.go`, `src/backend/modules/game/internal/infrastructure/modules/learning.go` — đã làm: `application/command/publish_minigame.go`, `infrastructure/modules/learning_minigame.go`, `persistence/write/world_repo.go` (bảng nội dung)
- [x] T016 [US1] Query + handler internal `validateMinigameQuestionSource`; consumer `learning.minigame_config_projection` — file: `src/backend/modules/learning/internal/application/query/validate_minigame_question_source.go`, `src/backend/modules/learning/internal/presentation/consumer/minigame_config_projection.go`

## Phase 4: User Story 2 — Chơi mini-game, server chấm (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Test F073-AC-06, F073-AC-12: `startMiniGame` ghim version, gọi `createAttempt` đúng mode/ActivityRef, `Idempotency-Key` gửi tiếp; `assignment` → 422; chưa gán/sai khối → 404 (TC-TEN-019) — file: `src/backend/modules/game/tests/integration/minigame/start_minigame_test.go` — phải FAIL trước khi code — đã làm: `tests/integration/minigame_test.go` `TestStartMiniGame` + `tests/contract/learning_validate_minigame_source_test.go` `TestLearningCreateMinigameAttemptFacade`
- [x] T018 [P] [US2] Contract test F073-AC-07: payload/cấu hình hiển thị 11 loại không có `answer_key`, `correct`, `scoring_rule`, feedback (TC-ACT-001, TC-LRN-007) — file: `src/backend/modules/learning/tests/contract/minigame_render_payload_test.go` — đã làm: `tests/contract/minigame_render_payload_test.go` (learning)
- [x] T019 [P] [US2] Test F073-AC-08, F073-AC-09, F073-AC-10: điểm từ server, bỏ trường client; lớp ≤ 5 không `time_bonus`, `wrong_penalty = 0`; `learning_score`/`game_score`/`completion_met` tách; evidence không dùng `game_score` — file: `src/backend/modules/learning/tests/integration/plugins/minigame_scoring_test.go` — đã làm: `tests/integration/minigame_scoring_test.go` `TestMinigameScoring` (learning)
- [x] T020 [P] [US2] Test F073-AC-11: trả lời sau `deadline_at` → `ATTEMPT_DEADLINE_PASSED`; hết giờ → `expired` + chấm phần đã trả lời — file: `src/backend/modules/learning/tests/integration/plugins/minigame_deadline_test.go` — đã làm: `tests/integration/minigame_scoring_test.go` `TestMinigameDeadline` (learning)
- [x] T021 [P] [US2] Test authorization + tenant isolation F073-AC-13: `startMiniGame` × 5 role nhân sự → 403 (TC-AUTH-014); attempt/lượt chơi học sinh khác → 404; header tenant khác → `TENANT_MISMATCH` (TC-TEN-001) — file: `src/backend/modules/game/tests/integration/minigame/authz_isolation_test.go` — đã làm: `tests/integration/minigame_test.go` `TestMiniGameAuthzIsolation` + `TestPlayerRoutesStaffDenied`

### Implementation cho User Story 2

- [x] T022 [P] [US2] Plugin `minigame`: `BuildRenderPayload`, `Respond`, `Evaluate`, `MinigameScorer` — file: `src/backend/modules/learning/internal/domain/activity/plugins/minigame/`
- [x] T023 [US2] Command `StartMiniGame` (ghi `minigame_plays`) + handler `startMiniGame` — file: `src/backend/modules/game/internal/application/command/start_minigame.go`, `src/backend/modules/game/internal/presentation/http/public/minigames.go` — đã làm: `application/command/minigame.go`, `presentation/http/minigames.go`

## Phase 5: User Story 3 — Plugin `minigame` trong Activity Registry (Priority: P1)

### Tests cho User Story 3

- [x] T024 [P] [US3] Test hợp đồng plugin chung F073-AC-14 (TC-ACT-001) — file: `src/backend/modules/learning/internal/domain/activity/plugins/minigame/contract_test.go` — phải FAIL trước khi code — đã làm: `plugins/minigame/contract_test.go` + `tests/contract` `TestPluginContract` (learning)
- [x] T025 [P] [US3] Test F073-AC-15: activity `minigame` trỏ config chưa có / `question_set_override` không tương thích → `ACTIVITY_CONFIG_INVALID` (TC-ACT-003) — file: `src/backend/modules/learning/tests/integration/plugins/minigame_activity_validate_test.go` — đã làm: `TestMinigameActivityValidate` (learning)
- [x] T026 [P] [US3] Test F073-AC-16: bước quest `minigame` tạo attempt mode `quest`, hoàn thành khi `completion_met` (TC-ACT-002) — file: `src/backend/modules/game/tests/integration/minigame/quest_minigame_step_test.go` — đã làm: `tests/integration/minigame_test.go` `TestQuestMiniGameStep`
- [x] T027 [P] [US3] Test F073-AC-17: publish config v2 giữa lượt → lượt giữ câu/luật v1 (TC-ACT-005) — file: `src/backend/modules/learning/tests/integration/plugins/minigame_version_pinning_test.go` — đã làm: `TestMinigameVersionPinning` (learning)

### Implementation cho User Story 3

- [x] T028 [US3] `ConfigSchema`/`ValidateConfig`/`EmitEvidence`, đăng ký plugin vào registry — file: `src/backend/modules/learning/internal/domain/activity/plugins/minigame/plugin.go`
- [x] T029 [US3] Bộ xử lý bước quest `minigame` đăng ký vào registry F072 — file: `src/backend/modules/game/internal/domain/quest/handlers/minigame.go` — đã làm: `internal/domain/quest/minigame.go` (`MiniGameStep`, trong `DefaultRegistry`)

## Phase 6: User Story 4 — Kết thúc lượt chơi và tín hiệu phần thưởng (Priority: P2)

### Tests cho User Story 4

- [x] T030 [P] [US4] Test F073-AC-18, F073-AC-19, F073-AC-20: `activity.graded` → cập nhật lượt chơi, `minigame.completed` một lần (TC-SVC-003); vượt `max_plays_per_day` → `rewards_eligible = false`, vẫn chơi; không ghi XP/Xu/item — file: `src/backend/modules/game/tests/integration/minigame/record_result_test.go` — phải FAIL trước khi code — đã làm: `tests/integration/minigame_test.go` `TestMiniGameRecordResult` + `internal/presentation/consumer/reward_engine_minigame_test.go`

### Implementation cho User Story 4

- [x] T031 [US4] Command `RecordMiniGameResult` + consumer `game.minigame_on_activity_graded` (lượt `assignment` tạo dòng khi nhận event) — file: `src/backend/modules/game/internal/application/command/record_minigame_result.go`, `src/backend/modules/game/internal/presentation/consumer/minigame_on_activity_graded.go` — đã làm: `application/command/minigame.go` (`RecordResult`), `presentation/consumer/minigame_on_activity_graded.go`

## Phase 7: User Story 5 — Scene Godot cho 6 loại lõi (Priority: P2)

### Tests cho User Story 5

- [x] T032 [P] [US5] GdUnit4 test F073-AC-21: registry chọn scene + `scene_variant`; loại thiếu → "Cần cập nhật game" + telemetry (TC-ACT-004) — file: `src/game/tests/unit/minigames/minigame_registry_test.gd` — phải FAIL trước khi code
- [x] T033 [P] [US5] GdUnit4 test F073-AC-22 cho 6 loại: `response_ready` đúng format (TC-LRN-016), không gọi mạng, hiệu ứng chỉ sau `show_feedback`, teardown không rò node — file: `src/game/tests/unit/minigames/core_types_test.gd`
- [x] T034 [P] [US5] GdUnit4 test F073-AC-23, F073-AC-24: đồng hồ theo `server_time`, hàng đợi gửi lại cùng `Idempotency-Key`; vùng chạm ≥ 64 px, âm thanh + biểu tượng — file: `src/game/tests/unit/minigames/minigame_host_test.gd`

### Implementation cho User Story 5

- [x] T035 [US5] `minigame_base.gd`, `minigame_registry.gd`, `MiniGameHost.tscn`, `MinigameRenderer.tscn` (cắm vào `ActivityRendererRegistry` F063) — file: `src/game/scripts/learning/`, `src/game/scenes/minigames/MiniGameHost.tscn`, `src/game/scenes/learning/activity_renderers/minigame/MinigameRenderer.tscn`
- [x] T036 [US5] Scene `multiple_choice`, `true_false`, `fill_in_blank`, `drag_and_drop`, `matching`, `ordering` (Theme token, khóa i18n) — file: `src/game/scenes/minigames/types/`

## Phase 8: User Story 6 — 5 loại mở rộng (Priority: P3)

### Tests cho User Story 6

- [x] T037 [P] [US6] GdUnit4 test F073-AC-25 cho 5 loại mở rộng — file: `src/game/tests/unit/minigames/extended_types_test.gd` — phải FAIL trước khi code
- [x] T038 [P] [US6] Test F073-AC-26: `timed_challenge` phát lô render payload, chấm từng câu ở server; `science_simulation` nộp trạng thái cuối như câu `interactive` — file: `src/backend/modules/learning/tests/integration/plugins/minigame_extended_types_test.go` — đã làm: `TestMinigameExtendedTypes` (learning)

### Implementation cho User Story 6

- [x] T039 [US6] Scene `calculation`, `puzzle`, `word_game`, `science_simulation`, `timed_challenge`; prefetch lô trong plugin — file: `src/game/scenes/minigames/types/`, `src/backend/modules/learning/internal/domain/activity/plugins/minigame/render.go`

## Phase 9: Polish & Cross-Cutting

- [x] T040 [P] MINIGAME_ENGINE §5 + ACTIVITY_ENGINE §9 (hợp nhất đường bắt đầu), DATABASE_DESIGN (`minigame_plays`, `ref.minigame_config_versions`, `attempts.game_score`), EVENT_DESIGN §3.2/§4.5 (bỏ learning tiêu thụ `minigame.completed`, payload), CHANGELOG — file: `docs/08-game-client/MINIGAME_ENGINE.md`, `docs/02-learning/ACTIVITY_ENGINE.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md` — đã làm (MINIGAME_ENGINE §4–§6.1, ACTIVITY_ENGINE §6.3/§9, DATABASE_DESIGN, EVENT_DESIGN, CHANGELOG)
- [x] T041 [P] Metric `eduworld_game_minigame_*`, `eduworld_learning_minigame_score_seconds` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go` — đã làm: `infrastructure/observability/minigame.go` (`eduworld_game_minigame_start_total`, `eduworld_game_minigame_completed_total`)
- [x] T042 [P] Benchmark scene mini-game trên preset Thấp/Trung bình (ngân sách PERFORMANCE_BUDGET) — file: `src/game/tests/integration/minigame_benchmark_test.gd`
- [ ] T043 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — chưa làm: cần stack compose riêng (quickstart tay, video DEV-AND-MID1)

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 → US5 → US6 → Polish; Godot US5 có thể bắt đầu song song US2 trên fixture/Prism mock (gắn nhãn MOCK, R17).
- Phần module learning (T004, T007, T011, T016, T022, T024–T028) phối hợp nhóm learning (F030).

## Definition of Done

- [x] Mọi tiêu chí F073-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (module game, module learning), `make test-contract`, `make godot-test` xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Bằng chứng: tiêu chí → test (2026-11-23)

| Tiêu chí | Test (đường dẫn tương đối module) | Kết quả |
|---|---|---|
| F073-AC-01 | game `internal/domain/minigame` `TestParseValidate`, `internal/infrastructure/minigameschema` `TestValidate`; integration `TestMiniGamePublish` | pass |
| F073-AC-02 | integration `TestMiniGamePublish` (incompatible, insufficient, set not published → `ACTIVITY_CONFIG_INVALID`); contract `TestLearningValidateMinigameSourceFacade`; learning `TestValidateMinigameQuestionSource`, `minigame.TestCheckSource` | pass |
| F073-AC-03 | `minigameschema.TestValidate` (trường câu hỏi bị schema từ chối), `TestMiniGamePublish` (snapshot không có câu hỏi, "embedded question" → 422) | pass |
| F073-AC-04 | game `TestMiniGamePublish` (outbox `content.published` khối `minigame_config`, schema event); learning `TestMinigameConfigProjection` (JetStream thật, event lặp không thêm dòng) | pass |
| F073-AC-05 | `TestTranslations`, `TestMiniGamePublish` (thiếu `en` → `TRANSLATION_INCOMPLETE`, chuỗi > 2000 → `VALIDATION_FAILED`) | pass |
| F073-AC-06 | `TestStartMiniGame` (ghim version, `createAttempt` mode `minigame`/`minigame_session`, Idempotency-Key chuyển tiếp, replay), `TestLearningCreateMinigameAttemptFacade`; learning `TestMinigameScoring` (createAttempt phía nhận, 409 khi chưa projection, `QUESTION_POOL_INSUFFICIENT`) | pass |
| F073-AC-07 | learning contract `TestMinigameRenderPayloadContract` (11 loại × 4 khối, schema `MiniGameDisplayConfig`); game `TestDisplay`, `TestStartMiniGame` (không `answer_key`/`correct`/`scoring_rule`) | pass |
| F073-AC-08 | learning `TestMinigameScoring` (trường client giả bị bỏ qua), `minigame.TestScorer` | pass |
| F073-AC-09 | learning `minigame.TestScorer`, `TestMinigameScoring` (khối 3 không `time_bonus`, `wrong_penalty = 0`; khối 7 có) | pass |
| F073-AC-10 | learning `TestMinigameScoring`, `TestLearningScoreAndCompletion`, `attempt.TestMinigameAttempt` (learning event/mastery không có `game_score`) | pass |
| F073-AC-11 | learning `TestMinigameDeadline` (`ATTEMPT_DEADLINE_PASSED` 422; worker → `expired` + chấm phần đã trả lời) | pass |
| F073-AC-12 | `TestStartMiniGame` (`assignment` → 422; chưa gán/sai khối/không tồn tại → 404) | pass |
| F073-AC-13 | `TestMiniGameAuthzIsolation` (4 role nhân sự → 403, tenant khác → `TENANT_MISMATCH`, RLS), `TestPlayerRoutesStaffDenied` (5 role) | pass |
| F073-AC-14 | learning `minigame.TestMinigamePluginContract`, contract `TestPluginContract`, `TestRegistryResolve` | pass |
| F073-AC-15 | learning `TestMinigameActivityValidate` | pass |
| F073-AC-16 | `TestQuestMiniGameStep` (mode `quest`, version ghim, hoàn thành theo `completion_met`) | pass |
| F073-AC-17 | learning `TestMinigameVersionPinning` | pass |
| F073-AC-18 | `TestMiniGameRecordResult` (một `minigame.completed`, event lặp không phát lại, schema event) | pass |
| F073-AC-19 | `TestMiniGameRecordResult` (lượt thứ 6 > `max_plays_per_day` vẫn chơi, `rewards_eligible = false`), `TestRewardsEligible`, `TestTriggerMiniGameCompleted` | pass |
| F073-AC-20 | `TestMiniGameRecordResult` (không dòng XP/Xu/item), `TestTriggerMiniGameCompleted` (chỉ `reward_rule_id`, `activity.graded` mode `minigame` không thưởng) | pass |
| F073-AC-21 | Godot `tests/unit/minigames/minigame_registry_test.gd` | pass |
| F073-AC-22 | Godot `tests/unit/minigames/core_types_test.gd` | pass |
| F073-AC-23 | Godot `tests/unit/minigames/minigame_host_test.gd` (đồng hồ theo `server_time`, gửi lại cùng key) | pass |
| F073-AC-24 | Godot `minigame_host_test.gd` (vùng chạm ≥ 64 px cả 11 loại, âm thanh + biểu tượng + chữ, bàn phím/chuột/chạm) | pass |
| F073-AC-25 | Godot `tests/unit/minigames/extended_types_test.gd` | pass |
| F073-AC-26 | learning `TestMinigameExtendedTypes`; Godot `extended_types_test.gd` (prefetch `timed_challenge`, trạng thái cuối `science_simulation`) | pass |
