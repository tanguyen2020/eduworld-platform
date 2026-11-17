# Tasks: F072 — Quest Engine: 12 loại quest, bước, tiến độ qua event, lọc theo khối

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Adapter facade `learning` (`createAttempt`, `getAttemptResult`) qua `src/backend/pkg/modcall` (deadline, `TenantScope`, Principal gốc, `Idempotency-Key`, span truyền qua `context`; không retry/circuit breaker) — file: `src/backend/modules/game/internal/infrastructure/modules/learning.go`
- [ ] T002 [P] Seed 3 quest v1 (chuỗi main quest chương 0, quest Toán lớp 3, một daily) + gán 3 tenant demo — file: `src/backend/modules/game/seed/quests/`
- [ ] T003 [P] Cấu hình tham số quest đọc từ platform settings (`max_active_quests`, `daily_quests_per_day`, `weekly_quests_per_week`, `quest_reset_time`, `quest_pass_rule`, `learning_block_max_items`) — file: `src/backend/modules/game/internal/config/quest.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Sửa `QuestType` (12 loại), `QuestProgressStatus`, `QuestSummary`, `QuestDetail`, `QuestProgress.step_states`; `listQuests` thêm `area_id`; `acceptQuest` body `{party_id?}` + lỗi; `startQuestActivity` `409`/`503`; ProblemCode `QUEST_PREREQUISITE_NOT_MET`, `QUEST_ACTIVE_LIMIT`, `QUEST_STEP_ATTEMPT_LIMIT` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [ ] T005 [P] `ActivityRef.content_type` thêm `activity` — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`, `make gen MOD=game`
- [ ] T006 [P] `quest.schema.json`: `content_ref.kind` thêm `activity`, `start_trigger.interactable_ids` (≤ 20, `maxLength`) — file: `specs/schemas/quest.schema.json` — kiểm: `make specs-lint`
- [ ] T007 [P] Payload event `quest.accepted`, `quest.step_completed`, `quest.completed`, `quest.abandoned` — file: `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [ ] T008 ent schema `QuestDefinition`, `QuestVersion` (cột tách), `QuestProgress`, `QuestStepAttempt`, `PeriodicQuestGrant` với `MaxLen`; `infra.tenant_resource_assignments` nếu chưa có — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f072_quest`, `make migrate-lint MOD=game`
- [ ] T009 Migration: unique partial một active/quest, unique `period_key`, GIN `grade_levels`, CHECK `quest_type`/`status`/jsonb, RLS — file: `src/backend/modules/game/migrations/`
- [ ] T010 Đăng ký `item_type = quest` vào lõi phát hành F071 (snapshot delivery, manifest tenant theo gán) — file: `src/backend/modules/game/internal/infrastructure/content/quest_item_type.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Thấy và nhận quest đúng khối (Priority: P1) 🎯 MVP

**Goal**: Học sinh thấy và nhận quest đúng khối, đúng tenant.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T011 [P] [US1] Unit test `QuestAvailability` (khối, gán tenant, availability, prerequisites, learning requirements, môn bật) — file: `src/backend/modules/game/internal/domain/quest/availability_test.go` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Test F072-AC-01, F072-AC-02, F072-AC-03: lọc `status`/`quest_type`/`area_id`; sai khối hoặc chưa gán → 404 (TC-GAM-012, TC-TEN-019); delivery vi/en — file: `src/backend/modules/game/tests/integration/quest/list_get_quest_test.go`
- [ ] T013 [P] [US1] Test F072-AC-04, F072-AC-05, F072-AC-06: accept ghim version + `quest.accepted`, cùng `Idempotency-Key` → cùng kết quả; thiếu điều kiện → `QUEST_PREREQUISITE_NOT_MET`, quest học tập không chặn bởi level; vượt `max_active_quests` → 409, đang active → trả progress cũ — file: `src/backend/modules/game/tests/integration/quest/accept_quest_test.go`
- [ ] T014 [P] [US1] Test F072-AC-07: abandon giữ attempt, `quest.abandoned` `player`, nhận lại tạo progress mới — file: `src/backend/modules/game/tests/integration/quest/abandon_quest_test.go`
- [ ] T015 [P] [US1] Test authorization + tenant isolation F072-AC-08: mọi route quest × 5 role nhân sự → 403 (TC-AUTH-014); progress học sinh/tenant khác → 404; header tenant khác → `TENANT_MISMATCH` (TC-TEN-001) — file: `src/backend/modules/game/tests/integration/quest/authz_isolation_test.go`

### Implementation cho User Story 1

- [ ] T016 [P] [US1] Domain `QuestVersion` (parse v2), `QuestAvailability`, `QuestProgress` (Accept/Abandon), `PeriodKey` — file: `src/backend/modules/game/internal/domain/quest/`
- [ ] T017 [US1] Query `ListQuests`, `GetQuest` (+ `AssignmentReader`, `MasteryReader`) — file: `src/backend/modules/game/internal/application/query/list_quests.go`, `src/backend/modules/game/internal/application/query/get_quest.go`
- [ ] T018 [US1] Command `AcceptQuest`, `AbandonQuest` (outbox, idempotency, audit) + handler (middleware `game_player`, `tenant`) — file: `src/backend/modules/game/internal/application/command/accept_quest.go`, `src/backend/modules/game/internal/presentation/http/public/quests.go`
- [ ] T019 [US1] Godot `QuestService`, `QuestJournal.tscn` (S05, tab theo grade band, khóa `ui.quest.*`) — file: `src/game/scripts/quest/quest_service.gd`, `src/game/scenes/quest/QuestJournal.tscn`

## Phase 4: User Story 2 — Bước tương tác do server xác minh (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Test F072-AC-09, F072-AC-10: `talk_npc`/`go_to`/`collect` kiểm session, presence, ID; mở bước kế theo `depends_on`; `quest.step_completed` + Valkey `quest.progress.updated`; sai area/ID → 422; bước locked/completed → 409 (TC-WLD-006, TC-GAM-010) — file: `src/backend/modules/game/tests/integration/quest/report_step_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Test F072-AC-11: cùng `Idempotency-Key` và cùng `interactable_id` của `collect` tính một lần — file: `src/backend/modules/game/tests/integration/quest/report_step_idempotency_test.go`
- [ ] T022 [P] [US2] Test F072-AC-12, F072-AC-13: `world.discovery_found` tăng bước `discover`, `reportQuestStep` cho `discover` → 422; bước khung không phát evidence/learning event — file: `src/backend/modules/game/tests/integration/quest/discover_step_test.go`

### Implementation cho User Story 2

- [ ] T023 [P] [US2] `StepHandler` registry + handler `talk_npc`, `go_to`, `collect`, `discover` — file: `src/backend/modules/game/internal/domain/quest/handlers/`
- [ ] T024 [US2] Command `ReportQuestStep`, `ApplyDiscovery` + consumer `game.quest_on_discovery_found` + `RealtimePublisher` — file: `src/backend/modules/game/internal/application/command/report_quest_step.go`, `src/backend/modules/game/internal/presentation/consumer/quest_on_discovery_found.go`
- [ ] T025 [US2] Godot `QuestTracker.tscn`, `QuestMarker3D.tscn`, `Interactable` gọi `QuestService.report_step`, nhận `quest.progress.updated` — file: `src/game/scenes/quest/QuestTracker.tscn`, `src/game/scripts/quest/quest_tracker.gd`

## Phase 5: User Story 3 — Bước học tập tạo attempt riêng (Priority: P1)

### Tests cho User Story 3

- [ ] T026 [P] [US3] Test F072-AC-14: `startQuestActivity` gọi `createAttempt` đúng mode/context/nội dung đã khóa, lưu `quest_step_attempts`, `Idempotency-Key` gửi tiếp (learning giả qua httptest của client sinh) — file: `src/backend/modules/game/tests/integration/quest/start_activity_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US3] Facade test `createAttempt`: bên gọi game với fake facade `learning`, bên nhận learning theo ví dụ spec internal (mode `quest`, `ActivityRef` `question_set`/`assessment_blueprint`/`activity`) — file: `src/backend/modules/game/tests/contract/learning_create_attempt_test.go`
- [ ] T028 [P] [US3] Test F072-AC-15, F072-AC-16: `activity.graded` đạt → bước completed; không đạt → active + gợi ý; replay xử lý một lần (TC-SVC-003); `reportQuestStep` cho bước học tập → 422 (TC-LRN-007) — file: `src/backend/modules/game/tests/integration/quest/activity_result_test.go`
- [ ] T029 [P] [US3] Test F072-AC-17, F072-AC-18: vượt `max_attempts_per_day` → `QUEST_STEP_ATTEMPT_LIMIT`; learning 5xx/timeout → 503 + `Retry-After`, tiến độ không đổi (TC-SVC-004) — file: `src/backend/modules/game/tests/integration/quest/start_activity_failure_test.go`

### Implementation cho User Story 3

- [ ] T030 [P] [US3] Handler bước `activity`, `PassRule` — file: `src/backend/modules/game/internal/domain/quest/handlers/activity.go`, `src/backend/modules/game/internal/domain/quest/pass_rule.go`
- [ ] T031 [US3] Command `StartQuestActivity`, `ApplyActivityResult` + consumer `game.quest_on_activity_graded` — file: `src/backend/modules/game/internal/application/command/start_quest_activity.go`, `src/backend/modules/game/internal/presentation/consumer/quest_on_activity_graded.go`

## Phase 6: User Story 4 — Hoàn thành quest và tín hiệu phần thưởng (Priority: P1)

### Tests cho User Story 4

- [ ] T032 [P] [US4] Test F072-AC-19, F072-AC-20: `all_required_steps`/`any_n_of`, bước `optional` không chặn; `quest.completed` một lần cùng transaction; không ghi XP/Xu/item ở module quest (TC-GAM-004 phần quest) — file: `src/backend/modules/game/tests/integration/quest/complete_quest_test.go` — phải FAIL trước khi code
- [ ] T033 [P] [US4] Test F072-AC-21: learning consumer ghi learning event `quest.completed` một lần/`quest_progress_id` (TC-SVC-003) — file: `src/backend/modules/learning/tests/integration/consumer/learning_event_on_quest_completed_test.go`

### Implementation cho User Story 4

- [ ] T034 [US4] `CompletionRule` + chuyển `completed` trong `ReportQuestStep`/`ApplyActivityResult`/`ApplyDiscovery`; audit `quest.completed` — file: `src/backend/modules/game/internal/domain/quest/completion_rule.go`
- [ ] T035 [US4] Consumer `learning.learning_event_on_quest_completed` — file: `src/backend/modules/learning/internal/presentation/consumer/learning_event_on_quest_completed.go`

## Phase 7: User Story 5 — Daily và weekly quest không áp lực (Priority: P2)

### Tests cho User Story 5

- [ ] T036 [P] [US5] Test F072-AC-22: session đầu kỳ cấp đúng số daily/weekly theo khối, môn bật, LO yếu; lặp trong kỳ không thêm; `period_key` ICT — file: `src/backend/modules/game/tests/integration/quest/periodic_grant_test.go` — phải FAIL trước khi code
- [ ] T037 [P] [US5] Test F072-AC-23: job qua `quest_reset_time` (Clock giả) → `expired`, `quest.abandoned` `expired`, không trừ XP/Xu, không streak — file: `src/backend/modules/game/tests/integration/quest/period_expiry_job_test.go`

### Implementation cho User Story 5

- [ ] T038 [US5] Command `GrantPeriodicQuests` + consumer `game.periodic_quests_on_session_started`; command `ExpirePeriodQuests` + job `quest-period-expiry` — file: `src/backend/modules/game/internal/application/command/grant_periodic_quests.go`, `src/backend/modules/game/internal/presentation/job/quest_period_expiry.go`

## Phase 8: User Story 6 — Version, lặp lại, thất bại và kiểm định khi phát hành (Priority: P2)

### Tests cho User Story 6

- [ ] T039 [P] [US6] Test F072-AC-24: đổi con trỏ (publish/rollback) khi đang làm → giữ version ghim; người nhận mới theo con trỏ (TC-CNT-001) — file: `src/backend/modules/game/tests/integration/quest/version_pinning_test.go` — phải FAIL trước khi code
- [ ] T040 [P] [US6] Test F072-AC-25: `once` → `QUEST_ALREADY_COMPLETED`; `repeatable` trong cooldown → `QUEST_COOLDOWN` + thời điểm; vượt `max_completions` → 409 — file: `src/backend/modules/game/tests/integration/quest/repeatability_test.go`
- [ ] T041 [P] [US6] Test F072-AC-26: `reset_step` chỉ đặt lại bước; `fail_quest` → `failed`, nhận lại ngay; không phạt — file: `src/backend/modules/game/tests/integration/quest/failure_conditions_test.go`
- [ ] T042 [P] [US6] Test F072-AC-27, F072-AC-28: publish sai schema/thiếu `en`/tham chiếu không tồn tại/thiếu LO/chuỗi câu hỏi vượt `learning_block_max_items`/bước chưa có handler → 422 đúng code; chuỗi body > 64 ký tự → 422 — file: `src/backend/modules/game/tests/integration/quest/quest_publish_validator_test.go`

### Implementation cho User Story 6

- [ ] T043 [US6] `FailurePolicy`, command `ApplyQuestFailure` + job `quest-failure-timer`; luật repeatability trong `AcceptQuest` — file: `src/backend/modules/game/internal/application/command/apply_quest_failure.go`, `src/backend/modules/game/internal/presentation/job/quest_failure_timer.go`
- [ ] T044 [US6] `QuestPublishValidator` gắn vào `PublishGameContent` cho `item_type = quest` — file: `src/backend/modules/game/internal/domain/quest/publish_validator.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T045 [P] DATABASE_DESIGN §2.8 (`quest_progress` khóa mới, cột tách `quest_versions`, `quest_step_attempts`, `periodic_quest_grants`); EVENT_DESIGN §4.5 payload `quest.*`; GODOT_ARCHITECTURE §3 thêm `scripts/quest/`; CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`, `docs/08-game-client/GODOT_ARCHITECTURE.md`
- [ ] T046 [P] Metric `eduworld_game_quest_*` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`
- [ ] T047 [P] Kịch bản tải luồng quest game → learning (TC-PERF-013) — file: `tests/perf/k6/quest_flow.js`
- [ ] T048 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T004–T010) → US1 → US2, US3 song song → US4 → US5, US6 song song → Polish.
- T005 (learning internal) phối hợp nhóm learning (F030); learning consumer T035 cần F031.

## Definition of Done

- [ ] Mọi tiêu chí F072-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (module game, phần module learning liên quan), `make test-contract`, `make godot-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
