# Tasks: F030 — Activity Registry và Attempt API (`quiz`, `interactive_exercise`)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `domain/activity`, `domain/activity/plugins/{quiz,interactive}`, `domain/attempt`; wiring registry ở composition root — file: `src/backend/modules/learning/module.go` — ✅ package đã có; `module.go` dựng `plugins.Registry()`, `qbschema.NewActivities()` cho `command.Docs`/`command.Releases`, `query.CmsActivities`, `query.ActivityTypes`, `command.Attempts`/`query.Attempts` (adapter thật, `query.Delivery` có cache Valkey), facade `AttemptOpener`/`AttemptResults` (`createAttempt`, `getAttemptResult` trong `learning.Facade`), readiness `activity_registry:learning`, `observability.RegisterAttempts` (2026-11-20): `go test ./cmd/internal/wiring/` TestCoreAPIMountsEveryImplementedOperation pass (240 operation mount đúng một lần, check `activity_registry:learning`); smoke stack `ewf030` `/readyz` core-api + worker có `activity_registry:learning` ok
- [x] T002 [P] Middleware `game_player` cho `/learning/activities/*`, `/learning/activity-attempts/*`; route internal trên cổng internal — file: `src/backend/modules/learning/internal/presentation/http/router.go` — ✅ Operation `player()` = `httpx.GuardGamePlayer` + `TenantRequired` cho 9 route học sinh (`presentation/http/activity_attempts.go`); facade không có route (R29); TestAttemptStart (403 GAME_ACCESS_STUDENT_ONLY)
- [x] T003 [P] Job `attempt-lifecycle` đăng ký trong worker (mỗi phút, khóa phân tán) — file: `src/backend/modules/learning/module.go` — ✅ `r.Job(learning, "attempt-lifecycle", 1 phút, job.AttemptLifecycle.Count)` chạy trong worker profile `jobs`/`all` (cùng các job khác của `pkg/module.Registry`, khóa leader chưa có — OQ F010-2 [Confirmed]; job an toàn khi chạy song song nhờ row version) (2026-11-20): TestWorkerProfilesRegisterRelaysConsumersAndJobs pass (events/jobs/all); smoke `ewf030` metric `eduworld_job_runs_total{job="attempt-lifecycle",result="ok"}`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta OpenAPI: responses `410`/`428` cho `submitActivityResponse`/`submitActivityAttempt`; `ActivityTypeInfo.min_client_version` bắt buộc khi `enabled` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning` — ✅ responses 403/404/409/410/422/428/429 cho operation attempt, `ActivityTypeInfo.required += min_client_version, status`; `split-public --check`, `check-maxlength`, `check-problem-codes`, portal `gen:check` xanh (2026-11-20)
- [x] T005 [P] Payload `activity.abandoned.v1.schema.json` — file: `specs/events/payloads/learning/activity.abandoned.v1.schema.json` — kiểm: `make specs-lint` — ✅ `activity.abandoned.v1.schema.json` (+ `activity.started.v1`); `validate-schemas` 97 schema 0 lỗi
- [x] T006 ent schema `Attempt`, `AttemptItem`, `AttemptAnswer` (partition), `Evaluation`, `LearningEvent` (partition tháng hiện tại + kế); cột mới `ActivityVersion` — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f030_attempt`, `make migrate-lint MOD=learning` — ✅ ent `Activity`, `ActivityVersion` (loại nội dung ghép F024), `Attempt`, `AttemptItem`, `Evaluation`; `attempt_answers`, `learning_events` partition tháng viết tay — migration `20261130000001_f030_activity_attempt.sql`, golden `TestMigrationMatchesEnt` pass, `atlas-sum`
- [x] T007 Cập nhật DATABASE_DESIGN §2.6, §3.4, §3.5 (cột mới, `context_type`, unique practice in_progress) — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ DATABASE_DESIGN §2.6, §2.6a, §2.7, §3.4 cập nhật
- [x] T008 Port `AttemptRepository`, `LearningEventWriter`, `QuestionScorer`, `QuestionSelector`; adapter tới F022/F026 — file: `src/backend/modules/learning/internal/application/port/attempt_repo.go` — ✅ `port/write/attempt.go` (AttemptStore, LearningEventWriter), `port/attempt.go` (QuestionDelivery, StudentDirectory), `port/read/attempt.go`; adapter `persistence/write/attempt_repo.go`, `learning_event_writer.go`, `persistence/read/attempt.go`; `query.Delivery.OptionFeedback`
- [x] T052 [P] Delta contract: `learning-event.schema.json` đổi `exercise.started`/`exercise.submitted` sang `activity.*` + thêm `activity.abandoned`; payload domain event `activity.started.v1` (có `recommendation_id?` cho F032) và `activity.abandoned.v1` ở `specs/events/payloads/learning/`; `AttemptCreateRequest.context.context_type` thêm `challenge_attempt`, `boss_encounter` — file: `specs/schemas/learning-event.schema.json`, `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — ✅ `learning-event.schema.json` bỏ `exercise.*`, thêm `activity.abandoned` (+ `payload_activity_abandoned`); internal `context_type` + `challenge_attempt`, `boss_encounter`, `phase`; `make gen` facade

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Bắt đầu luyện tập từ activity đã xuất bản (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit test `Attempt.Start` ghim version, `display_language`, `feedback_timing` mặc định theo mode (F030-AC-01) — file: `src/backend/modules/learning/internal/domain/attempt/start_test.go` — phải FAIL trước khi code — ✅ `internal/domain/attempt/attempt_test.go` TestStart pass
- [x] T010 [P] [US1] Contract test quét `ActivityRender`/`QuestionRender`/`Attempt` không có trường đáp án (F030-AC-02) — file: `src/backend/modules/learning/tests/contract/attempt_dto_no_answer_test.go` — ✅ quét khóa đáp án trên response thật: `answerKeysIn` trong TestAttemptStart/TestAttemptAdHoc (Attempt, ActivityRender, QuestionRender) + `tests/contract/plugin_contract_test.go` (render payload plugin) pass — không tạo file contract riêng
- [x] T011 [P] [US1] Integration test unpublished → 409, tenant khác/platform chưa gán → 404 (F030-AC-03) — file: `src/backend/modules/learning/tests/integration/attempt/start_scope_test.go` — ✅ `tests/integration/attempt_test.go` TestAttemptStart (404 tenant B, 409 CONTENT_NOT_PUBLISHED) pass
- [x] T012 [P] [US1] Integration test trả lại attempt dở, Idempotency-Key, `attempt_start_rate` (F030-AC-04, F030-AC-05) — file: `src/backend/modules/learning/tests/integration/attempt/start_idempotency_test.go` — ✅ TestAttemptStart (replay cùng key, resume khác key, 428, 429 `attempt_start_rate`) pass
- [x] T013 [P] [US1] Test authorization `game_player` + owner check cho mọi endpoint attempt (F030-AC-06) — file: `src/backend/modules/learning/tests/integration/attempt/authz_test.go` — ✅ TestAttemptStart + TestAttemptRespondSubmitReview (403 portal, 404 ATTEMPT_NOT_FOUND học sinh khác/tenant khác; authztest.Cover 9 operation) pass
- [x] T014 [P] [US1] Integration test `listMyActivityAttempts` chỉ của mình + lọc (F030-AC-07) — file: `src/backend/modules/learning/tests/integration/attempt/list_my_test.go` — ✅ TestAttemptStart (listMyActivityAttempts lọc status/activity_type, chỉ của mình) pass

### Implementation cho User Story 1

- [x] T015 [P] [US1] Domain `activity.Plugin`, `Registry`; plugin `quiz`, `interactive_exercise` (BuildRenderPayload qua F022 delivery view) — file: `src/backend/modules/learning/internal/domain/activity/registry.go` — ✅ `domain/activity` (Plugin, Registry, Config, QuestionBased), `plugins/quiz`, `plugins/interactive`
- [x] T016 [P] [US1] Domain `attempt.Attempt` (Start, FeedbackPolicy) — file: `src/backend/modules/learning/internal/domain/attempt/attempt.go` — ✅ `domain/attempt/attempt.go`, `start.go` (Start, FeedbackPolicy, Respond, Pause, Submit, Expire, Abandon, IdlePause)
- [x] T017 [US1] Command `StartActivityAttempt` (resolver F026, learning event `activity.started`) + query `GetLearningActivity`, `ListMyActivityAttempts` — file: `src/backend/modules/learning/internal/application/command/start_attempt.go` — ✅ `command/attempts.go`, `attempt_commands.go` (StartActivity, learning event + outbox `activity.started`); `query/attempts.go` (Activity, ListMine)
- [x] T018 [US1] Repository attempt + `LearningEventWriter` (`ON CONFLICT DO NOTHING`) — file: `src/backend/modules/learning/internal/infrastructure/persistence/attempt_repo.go` — ✅ `persistence/write/attempt_repo.go`, `learning_event_writer.go` (NOT EXISTS + ON CONFLICT DO NOTHING, snapshot `class_id`)
- [x] T019 [US1] Handler `startActivityAttempt`, `getLearningActivity`, `listMyActivityAttempts` — file: `src/backend/modules/learning/internal/presentation/http/public/activity_attempt_handler.go` — ✅ `presentation/http/activity_attempts.go`

## Phase 4: User Story 2 — Trả lời, chấm và nộp bài (Priority: P1)

### Tests cho User Story 2

- [x] T020 [P] [US2] Unit test chấm + tính điểm §7.2 (final_score, câu bỏ trống, is_scored) và feedback theo `feedback_timing` (F030-AC-08, F030-AC-12) — file: `src/backend/modules/learning/internal/domain/attempt/scoring_test.go` — phải FAIL trước khi code — ✅ `internal/domain/attempt/attempt_test.go` TestRespondAndSubmit, TestFeedbackPolicyOfAnswers pass
- [x] T021 [P] [US2] Security test trường client `score`/`is_correct`/`duration_ms` bị bỏ qua (F030-AC-09) — file: `src/backend/modules/learning/tests/integration/attempt/client_fields_ignored_test.go` — ✅ TestAttemptRespondSubmitReview (client gửi score/is_correct/duration_ms/max_score bị bỏ qua; client_elapsed_ms không lưu vào attempt) pass
- [x] T022 [P] [US2] Integration test `INVALID_RESPONSE`, position sai, `ANSWER_LIMIT_REACHED`, `ATTEMPT_NOT_IN_PROGRESS` (F030-AC-10, F030-AC-11) — file: `src/backend/modules/learning/tests/integration/attempt/respond_errors_test.go` — ✅ TestAttemptRespondSubmitReview (INVALID_RESPONSE, position 99 → VALIDATION_FAILED, ANSWER_LIMIT_REACHED, ATTEMPT_NOT_IN_PROGRESS) pass
- [x] T023 [P] [US2] Integration test submit: một transaction, evaluation hiệu lực, outbox + learning event (F030-AC-12) — file: `src/backend/modules/learning/tests/integration/attempt/submit_test.go` — ✅ TestAttemptRespondSubmitReview (graded, 1 evaluation hiệu lực, 4 loại learning event, outbox activity.submitted + activity.graded) pass
- [x] T024 [P] [US2] Integration test nộp trùng cùng/khác key/đồng thời (F030-AC-13) — file: `src/backend/modules/learning/tests/integration/attempt/submit_idempotency_test.go` — ✅ TestAttemptRespondSubmitReview (replay cùng key, khác key 409 ATTEMPT_ALREADY_SUBMITTED; UPDATE theo row version) pass
- [x] T025 [P] [US2] Integration test làm lại + publish version mới không đổi attempt cũ (F030-AC-14) — file: `src/backend/modules/learning/tests/integration/attempt/history_immutability_test.go` — ✅ TestAttemptRespondSubmitReview (publish v2, attempt cũ giữ v1 + evaluation, làm lại ghim v2) pass
- [x] T026 [P] [US2] Integration test review sau `graded`, `409` khi chưa nộp, `feedback_timing = never` (F030-AC-15) — file: `src/backend/modules/learning/tests/integration/attempt/review_test.go` — ✅ TestAttemptRespondSubmitReview (409 trước khi chấm, review đúng/sai/feedback/weak_objectives, 404 học sinh khác) pass

### Implementation cho User Story 2

- [x] T027 [US2] Command `SubmitActivityResponse` (scorer F022, `question.answered`) — file: `src/backend/modules/learning/internal/application/command/submit_response.go` — ✅ `command.Attempts.Respond`
- [x] T028 [US2] Command `SubmitActivityAttempt` (UPDATE có điều kiện, evaluation, `activity.submitted`/`activity.graded`) — file: `src/backend/modules/learning/internal/application/command/submit_attempt.go` — ✅ `command.Attempts.Submit` / `gradeAttempt`
- [x] T029 [US2] Query `GetActivityAttemptReview` + handler `submitActivityResponse`, `submitActivityAttempt`, `getActivityAttemptReview` — file: `src/backend/modules/learning/internal/presentation/http/public/activity_attempt_handler.go` — ✅ `query.Attempts.Review` + handler

## Phase 5: User Story 3 — Tạm dừng, resume và hết hạn (Priority: P1)

### Tests cho User Story 3

- [x] T030 [P] [US3] Integration test resume trên version ghim (F030-AC-16) — file: `src/backend/modules/learning/tests/integration/attempt/resume_test.go` — ✅ `tests/integration/attempt_test.go` TestAttemptPauseResumeExpire (saved_responses, version ghim) pass
- [x] T031 [P] [US3] Integration test pause/idle loại khỏi `duration_ms` (F030-AC-17) — file: `src/backend/modules/learning/tests/integration/attempt/pause_test.go` — ✅ TestAttemptPauseResumeExpire (paused_ms = 30000, duration_ms = 0) + domain TestLifecycle pass
- [x] T032 [P] [US3] Integration test job abandon + `activity.abandoned` + `410` (F030-AC-18) — file: `src/backend/modules/learning/tests/integration/attempt/abandon_job_test.go` — ✅ TestAttemptPauseResumeExpire (idle pause, abandoned sau 25 h, outbox activity.abandoned, 410) pass
- [x] T033 [P] [US3] Integration test expire + auto-submit + `ATTEMPT_EXPIRED`/`ATTEMPT_DEADLINE_PASSED` với clock giả (F030-AC-19) — file: `src/backend/modules/learning/tests/integration/attempt/expire_job_test.go` — ✅ TestAttemptPauseResumeExpire (clock giả: 410 sau grace, 422 ATTEMPT_DEADLINE_PASSED, auto-submit timeout, expired không chấm) pass

### Implementation cho User Story 3

- [x] T034 [US3] Command `PauseActivityAttempt`, `ExpireAttempt`, `AbandonAttempt`; query `GetActivityAttempt` — file: `src/backend/modules/learning/internal/application/command/expire_attempt.go` — ✅ `command.Attempts.Pause`, `attempt_lifecycle.go`; `query.Attempts.Get`
- [x] T035 [US3] Job `attempt-lifecycle` (`FOR UPDATE SKIP LOCKED`, idle → pause, resume window → abandoned, deadline → expired/auto-submit) — file: `src/backend/modules/learning/internal/presentation/job/attempt_lifecycle.go` — ✅ `command.Attempts.RunLifecycle` (Bypass platform_job, mỗi attempt một transaction theo tenant) + `presentation/job.AttemptLifecycle` — đăng ký lịch mỗi phút trong worker thuộc agent wiring

## Phase 6: User Story 4 — Activity Registry và soạn activity (Priority: P2)

### Tests cho User Story 4

- [x] T036 [P] [US4] Unit test registry từ chối loại ngoài Release/chưa có plugin (F030-AC-20) — file: `src/backend/modules/learning/internal/domain/activity/registry_test.go` — phải FAIL trước khi code — ✅ `internal/domain/activity/registry_test.go` (TestRegistryResolve, TestRegistryRefusesDuplicatesAndOutOfRelease) pass
- [x] T037 [P] [US4] Integration test `ACTIVITY_CONFIG_INVALID` khi lưu draft và publish (F030-AC-21) — file: `src/backend/modules/learning/tests/integration/activity/config_schema_test.go` — ✅ `tests/integration/activity_cms_test.go` TestActivityRegistryAndConfig (draft + publish `ACTIVITY_CONFIG_INVALID` có JSON pointer) pass
- [x] T038 [P] [US4] Integration test `listActivityTypes` (F030-AC-22) — file: `src/backend/modules/learning/tests/integration/activity/list_types_test.go` — ✅ TestActivityRegistryAndConfig (listActivityTypes: 2 loại, schema_url, min_client_version, status) pass
- [x] T039 [P] [US4] Contract test chung cho mọi plugin + readiness khi thiếu plugin (F030-AC-23) — file: `src/backend/modules/learning/tests/contract/plugin_contract_test.go` — ✅ `tests/contract/plugin_contract_test.go` TestPluginContract + `internal/infrastructure/health/registry_check_test.go` pass
- [x] T040 [P] [US4] Test tenant isolation + authorization + `If-Match` + `maxLength` cho CMS activity (F030-AC-24) — file: `src/backend/modules/learning/tests/integration/activity/cms_authz_tenant_test.go` — ✅ `tests/integration/activity_cms_test.go` TestActivityCmsAuthzTenant (404 tenant B, 403, 428/412, maxLength 422, CONTENT_NOT_EDITABLE) pass

### Implementation cho User Story 4

- [x] T041 [US4] Command `CreateCmsActivity`, `UpdateCmsActivityDraft` (ValidateConfig theo `specs/schemas/activities/*`); query `ListActivityTypes`, `GetCmsActivity`, `ListCmsActivities` + handler — file: `src/backend/modules/learning/internal/presentation/http/public/cms_activity_handler.go` — ✅ `presentation/http/cms_activities.go`, `query/activities.go`, activity là loại nội dung ghép trong `command.Docs` (ValidateConfig theo `qbschema.NewActivities`, plugin), workflow/release dùng chung
- [x] T042 [US4] Readiness check registry ↔ `activity_type` có nội dung published; thay bảng metadata tạm của F026 bằng `MinClientVersion()` — file: `src/backend/modules/learning/internal/infrastructure/health/registry_check.go` — ✅ `infrastructure/health/registry_check.go` (RegistryCheck + SQLPublishedTypes); `command.MinClientVersions` thay bằng `Activities.MinClientVersion` — đăng ký check vào `/health/ready` trong `module.go` thuộc agent wiring

## Phase 7: User Story 5 — Attempt cho bước quest/mini-game từ module game (Priority: P2)

### Tests cho User Story 5

- [x] T043 [P] [US5] Integration test `createAttempt` mode quest/minigame, mode khác 422, idempotent (F030-AC-25) — file: `src/backend/modules/learning/tests/integration/facade/create_attempt_test.go` — ✅ `tests/integration/attempt_facade_test.go` TestAttemptFacade (quest, idempotent, practice → 422) pass
- [x] T044 [P] [US5] Test kiến trúc + tenant (fixture module `chat` import facade → `make arch-check` fail; `getAttemptResult` khác tenant → 404) (F030-AC-26) — file: `src/backend/modules/learning/tests/integration/facade/authz_test.go` — ✅ TestAttemptFacade (chat/competition → CALLER_NOT_ALLOWED qua modcall, tenant B → 404, không đáp án); `archgen --check` 0 vi phạm (depguard chat ↛ learning/api) ; facade không có route
- [ ] T045 [P] [US5] Facade test module game ↔ module learning (game với fake facade; learning với ví dụ spec internal) — file: `src/backend/modules/game/tests/contract/learning_attempts_test.go` — ⏳ phía learning: TestAttemptFacade với dữ liệu thật; phía game (adapter + fake facade) thuộc feature game tiêu thụ (F072) — chưa làm

### Implementation cho User Story 5

- [x] T046 [US5] Command `CreateInternalAttempt`, query `GetAttemptResult` + hiện thực facade (`src/backend/modules/learning/api`) — file: `src/backend/modules/learning/internal/presentation/facade/attempt_facade.go` — ✅ `command/attempt_internal.go` CreateInternal, `query.Attempts.Result`, `presentation/facade/attempts.go`; `ActivityRef.content_type += activity`

## Phase 8: User Story 6 — Luyện tập ad-hoc từ bộ câu hỏi hoặc điều kiện chọn (Priority: P2)

### Tests cho User Story 6

- [x] T047 [P] [US6] Integration test `question_set`/`selection`, seed tái lập, `QUESTION_POOL_INSUFFICIENT`, tenant + nội dung gán (F030-AC-27) — file: `src/backend/modules/learning/tests/integration/attempt/ad_hoc_test.go` — ✅ `tests/integration/attempt_facade_test.go` TestAttemptAdHoc (selection 3 câu, seed, pool thiếu 422, tenant B, question_set + resume) pass

### Implementation cho User Story 6

- [x] T048 [US6] Command `StartAdHocAttempt` (selector F022) + handler `startAdHocAttempt` — file: `src/backend/modules/learning/internal/application/command/start_ad_hoc_attempt.go` — ✅ `command/attempt_adhoc.go` StartAdHoc + handler

## Phase 9: Polish & Cross-Cutting

- [ ] T049 [P] Metric `eduworld_attempt_*`, span `attempt.*`; gắn cờ `suspicious_speed_flag` gửi ANTI_CHEAT (không phạt) — file: `src/backend/modules/learning/internal/infrastructure/observability/attempt_metrics.go` — ⏳ đã có: `observability.AttemptCounter` (`eduworld_attempt_started_total`, `_submitted_total`, `_expired_total`), span qua `observability.UseCase`; chưa có `suspicious_speed_flag` → ANTI_CHEAT và `eduworld_attempt_response_latency_seconds` — OQ chốt 2026-11-20: cờ trên attempt + metric `eduworld_attempt_suspicious_speed_total`, không phạt (Session owner chốt)
- [x] T050 [P] Đồng bộ tên event (`activity.submitted`) và mã lỗi trong ACTIVITY_ENGINE §9–§10, TEST_STRATEGY TC-LRN-004; CHANGELOG — file: `docs/02-learning/ACTIVITY_ENGINE.md` — ✅ ACTIVITY_ENGINE §6.4 (context_type), §9 (`CONTENT_NOT_PUBLISHED`); TEST_STRATEGY TC-LRN-004 đã dùng `activity.submitted`; CHANGELOG
- [ ] T051 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/030-activity-attempt-api/quickstart.md` — ⏳ bằng chứng tự động thay kịch bản tay (bảng ở quickstart §4); chạy trên stack thật sau khi agent wiring nối module

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [x] T053 **OQ chốt 2026-11-20** `answer_rate`: bộ đếm Valkey theo học sinh (trả lời, nộp, khoảng cách tối thiểu theo thời điểm server), `429 RATE_LIMITED` + `Retry-After`, giá trị từ cấu hình; test vượt ngưỡng + Valkey lỗi cho qua có metric — file: `src/backend/modules/learning/internal/application/command/attempt_commands.go`, `internal/infrastructure/cache/` — kiểm: `make test-integration MOD=learning` — cỡ S — bằng chứng: `command/answer_rate.go` (`checkAnswerRate` trong `Respond`/`Submit`, `RetryAfter`), `cache/rate.go` (khóa `ew:learning:rate:<kind>:<student>:<phút>` INCR + khóa khoảng cách `gap:<student>:<attempt>` lưu thời điểm server, lệnh có thời hạn `PoolTimeout`), `fail` của HTTP learning gửi `Retry-After`; metric `eduworld_answer_rate_limited_total{kind}`, `eduworld_answer_rate_counter_failures_total{kind}`; `TestAnswerRate` PASS (khoảng cách → 429 `Retry-After: 1`, câu thứ 4 trong phút → 429, submit thứ 2 → 429, học sinh khác không bị ảnh hưởng, Valkey dừng → 3 câu + nộp đều 200 và metric lỗi = 3/1).
- [x] T054 **OQ chốt 2026-11-20** `createCmsActivityVersion` (contract cùng mẫu `createQuestionVersion` + handler dùng `command.Docs.NewVersion`); test version published → draft mới, idempotency, authorization, tenant isolation — file: `specs/openapi/modules/learning.v1.yaml`, `src/backend/modules/learning/internal/presentation/http/` — kiểm: `make specs-breaking`, `make test-integration MOD=learning` — cỡ S — bằng chứng: `POST /cms/learning/activities/{activity_id}/versions` (`NewVersionRequest` → `201 ActivityDefinition` + `ETag`, `x-portal-action: action.content_studio.activities.edit`, menu catalog `2026.11.20.3`), handler `createCmsActivityVersion`; body replay của `createCmsActivity`/`createCmsActivityVersion` là `ActivityDefinition` (`command.ActivityJSON`, trước đây replay trả dạng authoring chung); `TestActivityNewVersion` PASS (draft mở → 409, thiếu key → 428, `school_admin`/không role → 403, tenant B → 404, version nguồn không có → 404, v2 draft dựa trên v1 + replay cùng version, draft thứ hai → 409, v1 vẫn published). `make specs-breaking` không chạy được (máy không có `oasdiff`/`make`); thay đổi chỉ thêm operation.
- [x] T055 **OQ chốt 2026-11-20** Facade platform-ops learning nhận `ResourceType = activity` (gán, gỡ gán, clone, Thùng rác, khôi phục) — file: `src/backend/modules/learning/internal/presentation/facade/`, `specs/openapi/modules/internal-common.v1.yaml` — kiểm: facade test bên nhận — cỡ S — bằng chứng: `contentmodel.ResourceActivity` (`ParseResourceType`, `ResourceOf(activity)`), `command.DocResources` thêm handler `activity` (clone sao câu hỏi tenant nguồn và question set tenant được tham chiếu trong `config.content.question_source`, giữ nội dung platform); `internal-common` đã có `activity` (F045 T059); `TestActivityResourcePlatformOps` PASS (gán activity platform → tenant A đọc được, B 404, gỡ gán → 404; clone A → B: câu hỏi A sao sang B, câu platform giữ nguyên, `cloned_from_id`; xóa mềm → Thùng rác có `activity` → khôi phục) + unit `contentmodel`.
- [x] T056 **OQ chốt 2026-11-20** (OQ F016-2, F045-3) `mergeUserData` của learning: liên kết `student_id` gốc → user đích, idempotent theo `merge_id`, `nothing_to_do` khi không có dữ liệu; nối `LearningMerge` của identity — file: `src/backend/modules/learning/internal/application/command/merge_user_data.go`, migration learning, `src/backend/modules/identity/module.go` — kiểm: facade test hai phía — cỡ M — bằng chứng: migration `20261207000001_f030_student_merge_links.sql` (`learning.student_merge_links`, RLS T, view `_live`, không DELETE/TRUNCATE), `command.MergeUserData` (`platform.user.merge`, liên kết một lần mỗi `merge_id`, `nothing_to_do` khi không có attempt/mastery/lesson session trong tenant, `blocked` khi `student_id` gốc đã liên kết user khác, audit `student.merge_linked`), facade `MergeUserData` (idempotency theo `Idempotency-Key = merge_id`, 503 khi chưa nối); identity `wire.go` nối `Gateway.LearningMerge` (interface `LearningFacade` thêm `modules.LearningMergeOps`). Test bên nhận `TestMergeUserDataLinksStudent` PASS (nothing_to_do, done + attempt giữ `student_id` gốc + audit, replay cùng merge_id không thêm dòng, đích khác → blocked, tenant B → nothing_to_do); contract `TestPlatformOpsOnlyForIdentity`/`TestPlatformOpsAuthorizePrincipal` thêm `mergeUserData`; bên gọi `TestOwnerGatewayCallContext` (identity) PASS.
- [x] T057 (F063, contract b058746) `QuestionRender.hint_count` (0–5) = số cấp `hints[]` của version đã ghim, không trả nội dung gợi ý: đếm ở `read.Delivery.Deliverable` → `question.Content/Rendered.HintCount` → `attemptview.Item` — file: `src/backend/modules/learning/internal/infrastructure/persistence/read/question.go`, `internal/application/attemptview/view.go` — kiểm: `make test-integration MOD=learning` — bằng chứng: `TestRenderHintCountAndReviewRecommendations` PASS (2 và 1 cấp, không lộ `hints`)

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → (US4, US5, US6 song song) → Polish.
- T008 cần F022 (scorer/selector) và F026 (resolver/reader); T042 cập nhật lại T045 của F026.

## Definition of Done

- [x] Mọi tiêu chí F030-AC-* có test pass (ghi mã test/TC) — ✅ 27/27 AC: unit domain activity/attempt, contract plugin, integration activity_cms/attempt/attempt_facade (TC ghi ở tasks)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` xanh — ✅ gofmt, `go vet -tags integration`, golangci-lint (default + arch) 0 issues, tenancyguard, ewlint, entlint, `archgen --check`, `routematrix check` 0 problems, facadegen, unit learning 32 gói ok, contract + facade ok, integration learning đầy đủ 72/73 lần 1 (TestAttemptFacade quá deadline modcall 2 s khi máy tải) → sửa deadline test, chạy lại pass
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — ✅ không mock trong đường chạy thật; clock giả chỉ trong test
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — ✅ DATABASE_DESIGN, ACTIVITY_ENGINE, CHANGELOG
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — ✅ Done 2026-11-20
