# Tasks: F030 — Activity Registry và Attempt API (`quiz`, `interactive_exercise`)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `domain/activity`, `domain/activity/plugins/{quiz,interactive}`, `domain/attempt`; wiring registry ở composition root — file: `modules/learning/cmd/api/main.go`
- [ ] T002 [P] Middleware `game_player` cho `/learning/activities/*`, `/learning/activity-attempts/*`; route internal trên cổng internal — file: `modules/learning/internal/presentation/http/router.go`
- [ ] T003 [P] Job `attempt-lifecycle` đăng ký trong worker (mỗi phút, khóa phân tán) — file: `modules/learning/cmd/worker/main.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta OpenAPI: responses `410`/`428` cho `submitActivityResponse`/`submitActivityAttempt`; `ActivityTypeInfo.min_client_version` bắt buộc khi `enabled` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`
- [ ] T005 [P] Payload `activity.abandoned.v1.schema.json` — file: `specs/events/payloads/learning/activity.abandoned.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T006 ent schema `Attempt`, `AttemptItem`, `AttemptAnswer` (partition), `Evaluation`, `LearningEvent` (partition tháng hiện tại + kế); cột mới `ActivityVersion` — file: `modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f030_attempt`, `make migrate-lint MOD=learning`
- [ ] T007 Cập nhật DATABASE_DESIGN §2.6, §3.4, §3.5 (cột mới, `context_type`, unique practice in_progress) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T008 Port `AttemptRepository`, `LearningEventWriter`, `QuestionScorer`, `QuestionSelector`; adapter tới F022/F026 — file: `modules/learning/internal/application/port/attempt_repo.go`
- [ ] T052 [P] Delta contract: `learning-event.schema.json` đổi `exercise.started`/`exercise.submitted` sang `activity.*` + thêm `activity.abandoned`; payload domain event `activity.started.v1` (có `recommendation_id?` cho F032) và `activity.abandoned.v1` ở `specs/events/payloads/learning/`; `AttemptCreateRequest.context.context_type` thêm `challenge_attempt`, `boss_encounter` — file: `specs/schemas/learning-event.schema.json`, `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Bắt đầu luyện tập từ activity đã xuất bản (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test `Attempt.Start` ghim version, `display_language`, `feedback_timing` mặc định theo mode (F030-AC-01) — file: `modules/learning/internal/domain/attempt/start_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Contract test quét `ActivityRender`/`QuestionRender`/`Attempt` không có trường đáp án (F030-AC-02) — file: `modules/learning/tests/contract/attempt_dto_no_answer_test.go`
- [ ] T011 [P] [US1] Integration test unpublished → 409, tenant khác/platform chưa gán → 404 (F030-AC-03) — file: `modules/learning/tests/integration/attempt/start_scope_test.go`
- [ ] T012 [P] [US1] Integration test trả lại attempt dở, Idempotency-Key, `attempt_start_rate` (F030-AC-04, F030-AC-05) — file: `modules/learning/tests/integration/attempt/start_idempotency_test.go`
- [ ] T013 [P] [US1] Test authorization `game_player` + owner check cho mọi endpoint attempt (F030-AC-06) — file: `modules/learning/tests/integration/attempt/authz_test.go`
- [ ] T014 [P] [US1] Integration test `listMyActivityAttempts` chỉ của mình + lọc (F030-AC-07) — file: `modules/learning/tests/integration/attempt/list_my_test.go`

### Implementation cho User Story 1

- [ ] T015 [P] [US1] Domain `activity.Plugin`, `Registry`; plugin `quiz`, `interactive_exercise` (BuildRenderPayload qua F022 delivery view) — file: `modules/learning/internal/domain/activity/registry.go`
- [ ] T016 [P] [US1] Domain `attempt.Attempt` (Start, FeedbackPolicy) — file: `modules/learning/internal/domain/attempt/attempt.go`
- [ ] T017 [US1] Command `StartActivityAttempt` (resolver F026, learning event `activity.started`) + query `GetLearningActivity`, `ListMyActivityAttempts` — file: `modules/learning/internal/application/command/start_attempt.go`
- [ ] T018 [US1] Repository attempt + `LearningEventWriter` (`ON CONFLICT DO NOTHING`) — file: `modules/learning/internal/infrastructure/persistence/attempt_repo.go`
- [ ] T019 [US1] Handler `startActivityAttempt`, `getLearningActivity`, `listMyActivityAttempts` — file: `modules/learning/internal/presentation/http/public/activity_attempt_handler.go`

## Phase 4: User Story 2 — Trả lời, chấm và nộp bài (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Unit test chấm + tính điểm §7.2 (final_score, câu bỏ trống, is_scored) và feedback theo `feedback_timing` (F030-AC-08, F030-AC-12) — file: `modules/learning/internal/domain/attempt/scoring_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Security test trường client `score`/`is_correct`/`duration_ms` bị bỏ qua (F030-AC-09) — file: `modules/learning/tests/integration/attempt/client_fields_ignored_test.go`
- [ ] T022 [P] [US2] Integration test `INVALID_RESPONSE`, position sai, `ANSWER_LIMIT_REACHED`, `ATTEMPT_NOT_IN_PROGRESS` (F030-AC-10, F030-AC-11) — file: `modules/learning/tests/integration/attempt/respond_errors_test.go`
- [ ] T023 [P] [US2] Integration test submit: một transaction, evaluation hiệu lực, outbox + learning event (F030-AC-12) — file: `modules/learning/tests/integration/attempt/submit_test.go`
- [ ] T024 [P] [US2] Integration test nộp trùng cùng/khác key/đồng thời (F030-AC-13) — file: `modules/learning/tests/integration/attempt/submit_idempotency_test.go`
- [ ] T025 [P] [US2] Integration test làm lại + publish version mới không đổi attempt cũ (F030-AC-14) — file: `modules/learning/tests/integration/attempt/history_immutability_test.go`
- [ ] T026 [P] [US2] Integration test review sau `graded`, `409` khi chưa nộp, `feedback_timing = never` (F030-AC-15) — file: `modules/learning/tests/integration/attempt/review_test.go`

### Implementation cho User Story 2

- [ ] T027 [US2] Command `SubmitActivityResponse` (scorer F022, `question.answered`) — file: `modules/learning/internal/application/command/submit_response.go`
- [ ] T028 [US2] Command `SubmitActivityAttempt` (UPDATE có điều kiện, evaluation, `activity.submitted`/`activity.graded`) — file: `modules/learning/internal/application/command/submit_attempt.go`
- [ ] T029 [US2] Query `GetActivityAttemptReview` + handler `submitActivityResponse`, `submitActivityAttempt`, `getActivityAttemptReview` — file: `modules/learning/internal/presentation/http/public/activity_attempt_handler.go`

## Phase 5: User Story 3 — Tạm dừng, resume và hết hạn (Priority: P1)

### Tests cho User Story 3

- [ ] T030 [P] [US3] Integration test resume trên version ghim (F030-AC-16) — file: `modules/learning/tests/integration/attempt/resume_test.go`
- [ ] T031 [P] [US3] Integration test pause/idle loại khỏi `duration_ms` (F030-AC-17) — file: `modules/learning/tests/integration/attempt/pause_test.go`
- [ ] T032 [P] [US3] Integration test job abandon + `activity.abandoned` + `410` (F030-AC-18) — file: `modules/learning/tests/integration/attempt/abandon_job_test.go`
- [ ] T033 [P] [US3] Integration test expire + auto-submit + `ATTEMPT_EXPIRED`/`ATTEMPT_DEADLINE_PASSED` với clock giả (F030-AC-19) — file: `modules/learning/tests/integration/attempt/expire_job_test.go`

### Implementation cho User Story 3

- [ ] T034 [US3] Command `PauseActivityAttempt`, `ExpireAttempt`, `AbandonAttempt`; query `GetActivityAttempt` — file: `modules/learning/internal/application/command/expire_attempt.go`
- [ ] T035 [US3] Job `attempt-lifecycle` (`FOR UPDATE SKIP LOCKED`, idle → pause, resume window → abandoned, deadline → expired/auto-submit) — file: `modules/learning/internal/presentation/job/attempt_lifecycle.go`

## Phase 6: User Story 4 — Activity Registry và soạn activity (Priority: P2)

### Tests cho User Story 4

- [ ] T036 [P] [US4] Unit test registry từ chối loại ngoài Release/chưa có plugin (F030-AC-20) — file: `modules/learning/internal/domain/activity/registry_test.go` — phải FAIL trước khi code
- [ ] T037 [P] [US4] Integration test `ACTIVITY_CONFIG_INVALID` khi lưu draft và publish (F030-AC-21) — file: `modules/learning/tests/integration/activity/config_schema_test.go`
- [ ] T038 [P] [US4] Integration test `listActivityTypes` (F030-AC-22) — file: `modules/learning/tests/integration/activity/list_types_test.go`
- [ ] T039 [P] [US4] Contract test chung cho mọi plugin + readiness khi thiếu plugin (F030-AC-23) — file: `modules/learning/tests/contract/plugin_contract_test.go`
- [ ] T040 [P] [US4] Test tenant isolation + authorization + `If-Match` + `maxLength` cho CMS activity (F030-AC-24) — file: `modules/learning/tests/integration/activity/cms_authz_tenant_test.go`

### Implementation cho User Story 4

- [ ] T041 [US4] Command `CreateCmsActivity`, `UpdateCmsActivityDraft` (ValidateConfig theo `specs/schemas/activities/*`); query `ListActivityTypes`, `GetCmsActivity`, `ListCmsActivities` + handler — file: `modules/learning/internal/presentation/http/public/cms_activity_handler.go`
- [ ] T042 [US4] Readiness check registry ↔ `activity_type` có nội dung published; thay bảng metadata tạm của F026 bằng `MinClientVersion()` — file: `modules/learning/internal/infrastructure/health/registry_check.go`

## Phase 7: User Story 5 — Attempt cho bước quest/mini-game từ module game (Priority: P2)

### Tests cho User Story 5

- [ ] T043 [P] [US5] Integration test `createAttempt` mode quest/minigame, mode khác 422, idempotent (F030-AC-25) — file: `modules/learning/tests/integration/facade/create_attempt_test.go`
- [ ] T044 [P] [US5] Test kiến trúc + tenant (fixture module `chat` import facade → `make arch-check` fail; `getAttemptResult` khác tenant → 404) (F030-AC-26) — file: `modules/learning/tests/integration/facade/authz_test.go`
- [ ] T045 [P] [US5] Facade test module game ↔ module learning (game với fake facade; learning với ví dụ spec internal) — file: `modules/game/tests/contract/learning_attempts_test.go`

### Implementation cho User Story 5

- [ ] T046 [US5] Command `CreateInternalAttempt`, query `GetAttemptResult` + hiện thực facade (`modules/learning/api`) — file: `modules/learning/internal/presentation/facade/attempt_facade.go`

## Phase 8: User Story 6 — Luyện tập ad-hoc từ bộ câu hỏi hoặc điều kiện chọn (Priority: P2)

### Tests cho User Story 6

- [ ] T047 [P] [US6] Integration test `question_set`/`selection`, seed tái lập, `QUESTION_POOL_INSUFFICIENT`, tenant + nội dung gán (F030-AC-27) — file: `modules/learning/tests/integration/attempt/ad_hoc_test.go`

### Implementation cho User Story 6

- [ ] T048 [US6] Command `StartAdHocAttempt` (selector F022) + handler `startAdHocAttempt` — file: `modules/learning/internal/application/command/start_ad_hoc_attempt.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T049 [P] Metric `eduworld_attempt_*`, span `attempt.*`; gắn cờ `suspicious_speed_flag` gửi ANTI_CHEAT (không phạt) — file: `modules/learning/internal/infrastructure/observability/attempt_metrics.go`
- [ ] T050 [P] Đồng bộ tên event (`activity.submitted`) và mã lỗi trong ACTIVITY_ENGINE §9–§10, TEST_STRATEGY TC-LRN-004; CHANGELOG — file: `docs/02-learning/ACTIVITY_ENGINE.md`
- [ ] T051 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/030-activity-attempt-api/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → (US4, US5, US6 song song) → Polish.
- T008 cần F022 (scorer/selector) và F026 (resolver/reader); T042 cập nhật lại T045 của F026.

## Definition of Done

- [ ] Mọi tiêu chí F030-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
