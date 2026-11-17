# Tasks: F073 — Mini-game Engine: `MiniGameConfig`, plugin `minigame`, scene Godot 11 loại

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Seed 2 cấu hình (matching Toán lớp 3, true_false) + quest mẫu "Phiên chợ của bác Gấu Nâu" có bước `minigame`, gán 3 tenant demo — file: `modules/game/seed/minigames/`
- [ ] T002 [P] Fixture render payload 11 loại cho Godot — file: `godot/tests/fixtures/minigames/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Sửa `startMiniGame`, `MiniGameSession`, `ActivityResponseResult` (`game_points`, `streak`), kết quả submit (`game_score`, `completion_met`) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make gen MOD=learning`
- [ ] T004 [P] Thêm `validateMinigameQuestionSource` (`x-allowed-callers: [game]`), `ActivityRef.content_type` thêm `minigame_config` — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`, `make gen MOD=game`
- [ ] T005 [P] Payload event `minigame.completed`; đề xuất khối `minigame_config` cho `content.published` (game, owner F071) và khối `minigame` cho `activity.graded` (owner F030) — file: `specs/events/payloads/game/`, `specs/events/payloads/learning/activity.graded.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T006 ent schema game `MiniGameConfig`, `MiniGameConfigVersion` (cột tách), `MiniGamePlay` — file: `modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f073_minigame`, `make migrate-lint MOD=game`
- [ ] T007 [P] ent schema learning `RefMiniGameConfigVersion`, cột `assessment.attempts.game_score` — file: `modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f073_minigame`, `make migrate-lint MOD=learning`
- [ ] T008 Migration RLS `minigame_plays`, CHECK `minigame_type`/jsonb — file: `modules/game/migrations/`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Cấu hình mini-game hợp lệ và phát hành (Priority: P1) 🎯 MVP

**Goal**: Chỉ cấu hình hợp lệ được publish và đến được learning.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test bảng tương thích (sinh từ MINIGAME_ENGINE §2) + schema F073-AC-01, F073-AC-03 (không có trường câu hỏi trong payload) — file: `modules/game/internal/domain/minigame/compatibility_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Test F073-AC-02, F073-AC-05: nguồn không tương thích/thiếu câu/question set chưa publish → `ACTIVITY_CONFIG_INVALID` (TC-ACT-003); thiếu `en` → `TRANSLATION_INCOMPLETE`; chuỗi quá dài → 422 — file: `modules/game/tests/integration/minigame/publish_validator_test.go`
- [ ] T011 [P] [US1] Test learning `validateMinigameQuestionSource`: đúng/sai loại, đủ/thiếu câu; module ngoài `x-allowed-callers` (`game`) import facade → `make arch-check` fail (TC-SVC-005, TC-SVC-006) — file: `modules/learning/tests/integration/internal/validate_minigame_source_test.go`
- [ ] T012 [P] [US1] Facade test game ↔ learning `validateMinigameQuestionSource` (bên gọi với fake facade, bên nhận theo ví dụ spec internal) — file: `modules/game/tests/contract/learning_validate_minigame_source_test.go`
- [ ] T013 [P] [US1] Test F073-AC-04: publish → outbox `content.published` (`item_type = minigame_config`); consumer learning lưu snapshot bất biến, event lặp không thêm (TC-SVC-003) — file: `modules/learning/tests/integration/consumer/minigame_config_projection_test.go`

### Implementation cho User Story 1

- [ ] T014 [P] [US1] Domain `Compatibility`, `PublishValidator` — file: `modules/game/internal/domain/minigame/`
- [ ] T015 [US1] Đăng ký `item_type = minigame_config` vào `PublishGameContent` (F071) + adapter facade `learning` validate — file: `modules/game/internal/infrastructure/content/minigame_item_type.go`, `modules/game/internal/infrastructure/modules/learning.go`
- [ ] T016 [US1] Query + handler internal `validateMinigameQuestionSource`; consumer `learning.minigame_config_projection` — file: `modules/learning/internal/application/query/validate_minigame_question_source.go`, `modules/learning/internal/presentation/consumer/minigame_config_projection.go`

## Phase 4: User Story 2 — Chơi mini-game, server chấm (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Test F073-AC-06, F073-AC-12: `startMiniGame` ghim version, gọi `createAttempt` đúng mode/ActivityRef, `Idempotency-Key` gửi tiếp; `assignment` → 422; chưa gán/sai khối → 404 (TC-TEN-019) — file: `modules/game/tests/integration/minigame/start_minigame_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Contract test F073-AC-07: payload/cấu hình hiển thị 11 loại không có `answer_key`, `correct`, `scoring_rule`, feedback (TC-ACT-001, TC-LRN-007) — file: `modules/learning/tests/contract/minigame_render_payload_test.go`
- [ ] T019 [P] [US2] Test F073-AC-08, F073-AC-09, F073-AC-10: điểm từ server, bỏ trường client; lớp ≤ 5 không `time_bonus`, `wrong_penalty = 0`; `learning_score`/`game_score`/`completion_met` tách; evidence không dùng `game_score` — file: `modules/learning/tests/integration/plugins/minigame_scoring_test.go`
- [ ] T020 [P] [US2] Test F073-AC-11: trả lời sau `deadline_at` → `ATTEMPT_DEADLINE_PASSED`; hết giờ → `expired` + chấm phần đã trả lời — file: `modules/learning/tests/integration/plugins/minigame_deadline_test.go`
- [ ] T021 [P] [US2] Test authorization + tenant isolation F073-AC-13: `startMiniGame` × 5 role nhân sự → 403 (TC-AUTH-014); attempt/lượt chơi học sinh khác → 404; header tenant khác → `TENANT_MISMATCH` (TC-TEN-001) — file: `modules/game/tests/integration/minigame/authz_isolation_test.go`

### Implementation cho User Story 2

- [ ] T022 [P] [US2] Plugin `minigame`: `BuildRenderPayload`, `Respond`, `Evaluate`, `MinigameScorer` — file: `modules/learning/internal/domain/activity/plugins/minigame/`
- [ ] T023 [US2] Command `StartMiniGame` (ghi `minigame_plays`) + handler `startMiniGame` — file: `modules/game/internal/application/command/start_minigame.go`, `modules/game/internal/presentation/http/public/minigames.go`

## Phase 5: User Story 3 — Plugin `minigame` trong Activity Registry (Priority: P1)

### Tests cho User Story 3

- [ ] T024 [P] [US3] Test hợp đồng plugin chung F073-AC-14 (TC-ACT-001) — file: `modules/learning/internal/domain/activity/plugins/minigame/contract_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US3] Test F073-AC-15: activity `minigame` trỏ config chưa có / `question_set_override` không tương thích → `ACTIVITY_CONFIG_INVALID` (TC-ACT-003) — file: `modules/learning/tests/integration/plugins/minigame_activity_validate_test.go`
- [ ] T026 [P] [US3] Test F073-AC-16: bước quest `minigame` tạo attempt mode `quest`, hoàn thành khi `completion_met` (TC-ACT-002) — file: `modules/game/tests/integration/minigame/quest_minigame_step_test.go`
- [ ] T027 [P] [US3] Test F073-AC-17: publish config v2 giữa lượt → lượt giữ câu/luật v1 (TC-ACT-005) — file: `modules/learning/tests/integration/plugins/minigame_version_pinning_test.go`

### Implementation cho User Story 3

- [ ] T028 [US3] `ConfigSchema`/`ValidateConfig`/`EmitEvidence`, đăng ký plugin vào registry — file: `modules/learning/internal/domain/activity/plugins/minigame/plugin.go`
- [ ] T029 [US3] Bộ xử lý bước quest `minigame` đăng ký vào registry F072 — file: `modules/game/internal/domain/quest/handlers/minigame.go`

## Phase 6: User Story 4 — Kết thúc lượt chơi và tín hiệu phần thưởng (Priority: P2)

### Tests cho User Story 4

- [ ] T030 [P] [US4] Test F073-AC-18, F073-AC-19, F073-AC-20: `activity.graded` → cập nhật lượt chơi, `minigame.completed` một lần (TC-SVC-003); vượt `max_plays_per_day` → `rewards_eligible = false`, vẫn chơi; không ghi XP/Xu/item — file: `modules/game/tests/integration/minigame/record_result_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T031 [US4] Command `RecordMiniGameResult` + consumer `game.minigame_on_activity_graded` (lượt `assignment` tạo dòng khi nhận event) — file: `modules/game/internal/application/command/record_minigame_result.go`, `modules/game/internal/presentation/consumer/minigame_on_activity_graded.go`

## Phase 7: User Story 5 — Scene Godot cho 6 loại lõi (Priority: P2)

### Tests cho User Story 5

- [ ] T032 [P] [US5] GdUnit4 test F073-AC-21: registry chọn scene + `scene_variant`; loại thiếu → "Cần cập nhật game" + telemetry (TC-ACT-004) — file: `godot/tests/unit/minigames/minigame_registry_test.gd` — phải FAIL trước khi code
- [ ] T033 [P] [US5] GdUnit4 test F073-AC-22 cho 6 loại: `response_ready` đúng format (TC-LRN-016), không gọi mạng, hiệu ứng chỉ sau `show_feedback`, teardown không rò node — file: `godot/tests/unit/minigames/core_types_test.gd`
- [ ] T034 [P] [US5] GdUnit4 test F073-AC-23, F073-AC-24: đồng hồ theo `server_time`, hàng đợi gửi lại cùng `Idempotency-Key`; vùng chạm ≥ 64 px, âm thanh + biểu tượng — file: `godot/tests/unit/minigames/minigame_host_test.gd`

### Implementation cho User Story 5

- [ ] T035 [US5] `minigame_base.gd`, `minigame_registry.gd`, `MiniGameHost.tscn`, `MinigameRenderer.tscn` (cắm vào `ActivityRendererRegistry` F063) — file: `godot/scripts/learning/`, `godot/scenes/minigames/MiniGameHost.tscn`, `godot/scenes/learning/activity_renderers/minigame/MinigameRenderer.tscn`
- [ ] T036 [US5] Scene `multiple_choice`, `true_false`, `fill_in_blank`, `drag_and_drop`, `matching`, `ordering` (Theme token, khóa i18n) — file: `godot/scenes/minigames/types/`

## Phase 8: User Story 6 — 5 loại mở rộng (Priority: P3)

### Tests cho User Story 6

- [ ] T037 [P] [US6] GdUnit4 test F073-AC-25 cho 5 loại mở rộng — file: `godot/tests/unit/minigames/extended_types_test.gd` — phải FAIL trước khi code
- [ ] T038 [P] [US6] Test F073-AC-26: `timed_challenge` phát lô render payload, chấm từng câu ở server; `science_simulation` nộp trạng thái cuối như câu `interactive` — file: `modules/learning/tests/integration/plugins/minigame_extended_types_test.go`

### Implementation cho User Story 6

- [ ] T039 [US6] Scene `calculation`, `puzzle`, `word_game`, `science_simulation`, `timed_challenge`; prefetch lô trong plugin — file: `godot/scenes/minigames/types/`, `modules/learning/internal/domain/activity/plugins/minigame/render.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T040 [P] MINIGAME_ENGINE §5 + ACTIVITY_ENGINE §9 (hợp nhất đường bắt đầu), DATABASE_DESIGN (`minigame_plays`, `ref.minigame_config_versions`, `attempts.game_score`), EVENT_DESIGN §3.2/§4.5 (bỏ learning tiêu thụ `minigame.completed`, payload), CHANGELOG — file: `docs/08-game-client/MINIGAME_ENGINE.md`, `docs/02-learning/ACTIVITY_ENGINE.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`
- [ ] T041 [P] Metric `eduworld_game_minigame_*`, `eduworld_learning_minigame_score_seconds` — file: `modules/game/internal/infrastructure/observability/metrics.go`
- [ ] T042 [P] Benchmark scene mini-game trên preset Thấp/Trung bình (ngân sách PERFORMANCE_BUDGET) — file: `godot/tests/integration/minigame_benchmark_test.gd`
- [ ] T043 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 → US5 → US6 → Polish; Godot US5 có thể bắt đầu song song US2 trên fixture/Prism mock (gắn nhãn MOCK, R17).
- Phần module learning (T004, T007, T011, T016, T022, T024–T028) phối hợp nhóm learning (F030).

## Definition of Done

- [ ] Mọi tiêu chí F073-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (module game, module learning), `make test-contract`, `make godot-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
