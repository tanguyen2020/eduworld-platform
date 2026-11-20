# Tasks: F031 — Learning events, mastery v1, evaluation và re-evaluate

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `domain/learningrecord`, `domain/mastery`, `domain/evaluation`; wiring — file: `src/backend/modules/learning/cmd/api/main.go` — ✅ `domain/mastery` (WeightedEvidence, Replay, Effective), envelope + kiểm schema trong `persistence/write.LearningEventWriter` (`qbschema.ValidateLearningEvent`) thay `domain/learningrecord`; nối trong `module.go` (2026-11-20)
- [x] T002 [P] Đăng ký durable consumer `learning.mastery_on_graded` (`activity.graded`, `attempt.re_evaluated`) và `learning.progress_daily` (`activity.submitted`, `activity.graded`, `lesson.completed`); job `learning-events-partitions`, `regrade-runner` — file: `src/backend/modules/learning/cmd/worker/main.go` — ✅ `module.go`: consumer `learning.mastery_on_graded`, `learning.progress_daily` (`activity.graded`; `attempt.re_evaluated`, `lesson.completed` thêm ở US4/US6), job `learning-events-partitions` (24 h, role `learning_partition_admin`); `regrade-runner` thuộc US4 — TestWorkerProfilesRegisterRelaysConsumersAndJobs pass
- [x] T003 [P] Seed `mastery_model_configs` `we-1.0` `active` từ LEARNING_ANALYTICS §3.1 — file: `src/backend/modules/learning/seed/mastery_model_configs.json` — ✅ seed trong migration `20261201000001_f031_mastery.sql` (`we-1.0` `active`, tham số LEARNING_ANALYTICS §3.1) thay file JSON [Proposed] — TestMasteryOnGraded (`model_version = we-1.0`)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: payload `attempt.re_evaluated`, `learning_objective.progressed`, `learning_objective.mastered` v1; `getRegradeRequest`; mô tả quyền `getStudentProgress` — file: `specs/events/payloads/learning/`, `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning` — ✅ `learning_objective.progressed.v1`, `learning_objective.mastered.v1`, `attempt.re_evaluated.v1`; `getRegradeRequest` (+ `409`, trường mới của `RegradeRequest`, menu catalog `2026.11.20.2`); mô tả quyền/scope/audit `getStudentProgress`, `getMyProgress`, `createRegradeRequest` (split-public, portal `gen:api`, routematrix)
- [x] T005 ent schema `MasteryModelConfig`, `ObjectiveMastery` (cột mới), `MasteryHistory` (partition), `ProgressDaily`, `RegradeRequest` (H), `Evaluation` (cột mới), `MasteryRecomputeRun`, `ObjectiveMasteryShadow` — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f031_mastery`, `make migrate-lint MOD=learning` — ✅ phần US1–US3: `mastery_model_configs`, `objective_masteries`, `mastery_history` (P), `progress_daily` (P) viết tay (Clarifications 2026-11-20 [Proposed]); `RegradeRequest`, `Evaluation` (cột đã có từ F030), `MasteryRecomputeRun`, `ObjectiveMasteryShadow` thuộc US4/US5 — chưa làm; `atlas migrate hash`
- [x] T006 Cập nhật DATABASE_DESIGN §2.6a, §2.7, §2.16 (bảng mới, cột mới, `re_evaluation`, `regrade_requests` H) — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ phần US1–US3: DATABASE_DESIGN §2.7 (`learning_events`, `mastery_model_configs`, `objective_masteries`, `mastery_history`, `progress_daily`); §2.6a/§2.16 (regrade) thuộc US4
- [x] T045 Delta contract US6: payload `lesson.completed.v1` (owner F031), mô tả `completeLesson` (`409 CONFLICT`, idempotent) và `getLesson` (version ghim theo lesson session); EVENT_DESIGN §3.2, §4.4 — file: `specs/events/payloads/learning/lesson.completed.v1.schema.json`, `specs/openapi/eduworld-api.v1.yaml`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `node scripts/specs/validate-schemas.mjs`, `node scripts/specs/check-cross-features.mjs`
- [x] T046 ent schema `LessonSession` (cột mới `lesson_id`, `context_mode`, `assignment_id`, `learning_path_id`, `recommendation_id`, `completed_activity_ids`, `active_ms`; hai partial unique theo data-model); cập nhật DATABASE_DESIGN §2.5 — file: `src/backend/modules/learning/ent/schema/lesson_session.go`, `docs/07-backend/DATABASE_DESIGN.md` — kiểm: `make migrate-diff MOD=learning name=f031_lesson_sessions`, `make migrate-lint MOD=learning` — ✅ `learning.lesson_sessions` viết tay (migration `20261204000001_f031_lesson_sessions.sql`, hai partial unique open/completed, RLS T) thay ent [Proposed]; DATABASE_DESIGN §2.5
- [x] T007 Migration: GRANT `INSERT, SELECT` trên `learning.learning_events`, `learning.mastery_history` cho role app — file: `src/backend/modules/learning/migrations/` — ✅ `learning_events`/`mastery_history`: chỉ `SELECT, INSERT` cho app/platform, quyền trực tiếp trên partition bị thu hồi — TestLearningRecordAppendOnlyAndIsolated

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Ghi learning event làm bằng chứng học tập (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T008 [P] [US1] Contract test mọi learning event ghi ra hợp lệ `learning-event.schema.json`, đủ envelope (F031-AC-01) — file: `src/backend/modules/learning/tests/contract/learning_event_schema_test.go` — phải FAIL trước khi code — ✅ `persistence/write/learning_event_envelope_test.go` TestLearningEventEnvelopeValidation + TestMasteryOnGraded (`validateStoredEvents`: mọi event của attempt dựng lại envelope hợp lệ, `event_id` UUIDv7)
- [x] T009 [P] [US1] Integration test idempotency event + consumer `processed_events` (F031-AC-02) — file: `src/backend/modules/learning/tests/integration/learningrecord/idempotency_test.go` — ✅ TestMasteryOnGraded (giao lại `activity.graded`: `processed_events` một dòng mỗi consumer, history và `progress_daily` không đổi); idempotency key learning event: F030 TestAttemptStart
- [x] T010 [P] [US1] Integration test role app không UPDATE/DELETE được (F031-AC-03) — file: `src/backend/modules/learning/tests/integration/learningrecord/append_only_test.go` — ✅ TestLearningRecordAppendOnlyAndIsolated (UPDATE/DELETE `learning_events`, partition `learning_events_2026_11`, `mastery_history`, `attempt_answers` bằng role app/platform → permission denied)
- [x] T011 [P] [US1] Integration test job partition ≥ 2 tháng + cảnh báo (F031-AC-04) — file: `src/backend/modules/learning/tests/integration/learningrecord/partition_job_test.go` — ✅ TestLearningPartitionsJob (clock 2027-11: tạo 8 partition 2028-01/02, gauge = 3, chạy lại 0, partition mới không có quyền trực tiếp, role admin không đọc được bảng)
- [x] T012 [P] [US1] Unit test từ chối payload ngoài schema/PII (F031-AC-05) — file: `src/backend/modules/learning/internal/domain/learningrecord/validate_test.go` — ✅ TestLearningEventEnvelopeValidation (`nickname`, `email`, `world_position`, `chat_text` bị từ chối, lỗi không chứa giá trị)
- [x] T013 [P] [US1] Chaos integration test dừng Valkey khi nộp + cập nhật mastery (F031-AC-06) — file: `src/backend/modules/learning/tests/integration/learningrecord/valkey_down_test.go` — ✅ `tests/integration/valkey_down_test.go` TestLearningRecordWithoutValkey (Valkey dừng: practice từ pool bắt đầu, chấm, nộp; 6 learning event, 1 evaluation, mastery 3 evidence); phát hiện và sửa: lệnh cache pool không có timeout → treo khi Valkey dừng, nay `cache.PoolTimeout` 200 ms [Proposed]
- [x] T014 [P] [US1] Test tenant isolation RLS/`TenantScope` cho learning event + mastery (F031-AC-07) — file: `src/backend/modules/learning/tests/integration/learningrecord/tenant_isolation_test.go` — ✅ TestLearningRecordAppendOnlyAndIsolated (không scope → 0 dòng ở 4 bảng; repository trong scope tenant B không thấy mastery/evidence tenant A)

### Implementation cho User Story 1

- [x] T015 [P] [US1] `LearningEventRecorder` (envelope, `class_id` từ `ref.enrollments`, kiểm schema, khóa idempotency) thay writer của F030 — file: `src/backend/modules/learning/internal/domain/learningrecord/recorder.go` — ✅ `persistence/write.LearningEventWriter` kiểm `learning-event.schema.json` (bản nhúng `qbschema`, TestEmbeddedSchemas) + `Envelope` (actor, subject, producer); payload F030 `score` → `{raw, max, scaled}`; `ActorID` cho staff
- [x] T016 [US1] Job `learning-events-partitions` + metric `eduworld_learning_events_partition_months_ahead` — file: `src/backend/modules/learning/internal/presentation/job/learning_events_partitions.go` — ✅ `command.LearningPartitions` + `presentation/job.LearningEventsPartitions` + `persistence/write.PartitionMaint` + metric `eduworld_learning_events_partition_months_ahead`, alert `LearningPartitionsMissing` (MONITORING)

## Phase 4: User Story 2 — Mastery theo LO từ nhiều bằng chứng (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Golden test `WeightedEvidence` bảng LEARNING_ANALYTICS §3.3 + ví dụ bổ sung (F031-AC-09) — file: `src/backend/modules/learning/internal/domain/mastery/golden_test.go` — phải FAIL trước khi code — ✅ `internal/domain/mastery/golden_test.go` TestGoldenTable, TestGoldenAdditional (sai số ≤ 0,0005) pass
- [x] T018 [P] [US2] Unit test level `mastered` cần `n_min`/`d_min`; `Effective` decay không ghi đè (F031-AC-10, F031-AC-11) — file: `src/backend/modules/learning/internal/domain/mastery/level_test.go` — ✅ TestMasteredNeedsEvidenceOnDays, TestEffectiveDecay pass
- [x] T019 [P] [US2] Integration test consumer áp evidence (`is_scored`, `rapid_guess`), lịch sử, `model_version` (F031-AC-08, F031-AC-12) — file: `src/backend/modules/learning/tests/integration/mastery/consumer_test.go` — ✅ TestMasteryOnGraded (M = 0,720 sau 3 evidence, `model_version`, `distinct_days`, `mastery_history`)
- [x] T020 [P] [US2] Integration test `learning_objective.mastered` một lần, `progressed` mỗi thay đổi (F031-AC-13) — file: `src/backend/modules/learning/tests/integration/mastery/objective_events_test.go` — ✅ TestMasteryOnGraded (mastered đúng một learning event + một outbox; tính lại không phát lại; `progressed` 2 lần)
- [x] T021 [P] [US2] Integration test đồng thời cùng học sinh → kết quả tuần tự (F031-AC-14) — file: `src/backend/modules/learning/tests/integration/mastery/concurrency_test.go` — ✅ TestMasteryConcurrentEvidence (2 attempt áp đồng thời → M = 0,837, n = 6, 2 ngày, một `mastered`)

### Implementation cho User Story 2

- [x] T022 [P] [US2] Domain `mastery` (MasteryModel, WeightedEvidence, Level, Effective) — file: `src/backend/modules/learning/internal/domain/mastery/weighted_evidence.go` — ✅ `internal/domain/mastery/mastery.go`
- [x] T023 [US2] Command `ApplyMasteryEvidence` (đọc `question.answered` của attempt, khóa lạc quan, outbox `learning_objective.*`) — file: `src/backend/modules/learning/internal/application/command/apply_mastery_evidence.go` — ✅ `command.Mastery.ApplyGraded`/`Recompute` (replay, `UPDATE … WHERE version`, outbox + learning event `learning_objective.*`)
- [x] T024 [US2] Consumer `learning.mastery_on_graded` + repo mastery — file: `src/backend/modules/learning/internal/presentation/consumer/mastery_on_graded.go` — ✅ `presentation/consumer/mastery_on_graded.go` + `persistence/write.MasteryRepo`

## Phase 5: User Story 3 — Tiến độ cho học sinh và nhà trường (Priority: P2)

### Tests cho User Story 3

- [x] T025 [P] [US3] Integration test `getMyProgress` chỉ của mình, level sau decay (F031-AC-15) — file: `src/backend/modules/learning/tests/integration/progress/my_progress_test.go` — ✅ TestMasteryOnGraded (`getMyProgress`: level, evidence, totals, chỉ của mình, decay sau 104 ngày → proficient, giá trị lưu không đổi, nhân sự → 403)
- [x] T026 [P] [US3] Test authorization + tenant `getStudentProgress` (trường khác 404, môn ngoài scope 403, audit) (F031-AC-16) — file: `src/backend/modules/learning/tests/integration/progress/student_progress_authz_test.go` — ✅ TestStudentProgressAuthz (school_admin cùng trường/academic_manager 200 + 3 audit `learning.profile_viewed`; trường khác, tenant B → 404; môn ngoài scope, thiếu `subject_id` với grant theo môn, không quyền, token học sinh → 403)
- [x] T027 [P] [US3] Integration test `progress_daily` idempotent (F031-AC-17) — file: `src/backend/modules/learning/tests/integration/progress/progress_daily_test.go` — ✅ TestMasteryOnGraded (`progress_daily` 1 activity, 3 câu, 3 đúng; giao lại không cộng đôi)

### Implementation cho User Story 3

- [x] T028 [US3] Command `UpsertProgressDaily` + consumer `learning.progress_daily` — file: `src/backend/modules/learning/internal/presentation/consumer/progress_daily.go` — ✅ `command.Progress.RecordGraded` + `presentation/consumer/progress_daily.go` (`lesson.completed` thêm ở US6)
- [x] T029 [US3] Query `GetMyProgress`, `GetStudentProgress` (audit `learning.profile_viewed`) + handler — file: `src/backend/modules/learning/internal/presentation/http/public/progress_handler.go` — ✅ `query.Progress` (Mine, Student), `command.ProfileViews`, handler `presentation/http/progress.go`; OpenAPI mô tả quyền/audit + `403`

## Phase 6: User Story 4 — Chấm lại khi nội dung có lỗi (Priority: P2)

### Tests cho User Story 4

- [x] T030 [P] [US4] Unit test `evaluation.Regrade` cho `accept_all`, `use_new_key`, `drop_item` (F031-AC-20) — file: `src/backend/modules/learning/internal/domain/evaluation/regrade_test.go` — phải FAIL trước khi code — ✅ `internal/domain/evaluation/regrade_test.go` TestRegradeActions, TestSupersede pass
- [x] T031 [P] [US4] Integration test `createRegradeRequest` hợp lệ, `corrected_version_id`, audit (F031-AC-18) — file: `src/backend/modules/learning/tests/integration/regrade/create_test.go` — ✅ `tests/integration/regrade_test.go` TestRegradeUseNewKey (202 `queued`, `corrected_version_id` câu khác/scope sai/`exam_session`/lý do ngắn → 422, audit `content.re_evaluated`) + TestRegradePlatformContent (content_admin, yêu cầu không tenant)
- [x] T032 [P] [US4] Test authorization + tenant/subject scope `createRegradeRequest` (F031-AC-19) — file: `src/backend/modules/learning/tests/integration/regrade/authz_scope_test.go` — ✅ TestRegradeUseNewKey (school_admin, không quyền, academic_manager môn khác → 403; tenant B, content_admin trên câu của tenant → 404; `getRegradeRequest` tenant B 404, school_admin 403, tenant đọc yêu cầu platform 404)
- [x] T033 [P] [US4] Integration test job: evaluation mới/cũ superseded, attempt không đổi, `attempt.re_evaluated`, Bypass có audit (F031-AC-20) — file: `src/backend/modules/learning/tests/integration/regrade/runner_test.go` — ✅ TestRegradeUseNewKey (2 attempt: evaluation cũ `superseded`, mới `re_evaluation` hiệu lực, điểm đổi đúng chiều, attempt và câu trả lời gốc không đổi, 2 learning event `attempt.re_evaluated` actor staff, audit theo tenant) + TestRegradePlatformContent (Bypass `re_evaluation`, audit `cross_tenant.accessed`, accept_all)
- [x] T034 [P] [US4] Integration test mastery tính lại sau `attempt.re_evaluated` (F031-AC-21) — file: `src/backend/modules/learning/tests/integration/regrade/mastery_adjust_test.go` — ✅ TestRegradeUseNewKey (giao `attempt.re_evaluated` → mastery tính lại thấp hơn, 1 dòng history `re_evaluation`, `question.answered` gốc giữ nguyên)
- [x] T035 [P] [US4] Integration test Idempotency-Key, `409` yêu cầu trùng, `EVALUATION_NOT_EDITABLE` (F031-AC-22) — file: `src/backend/modules/learning/tests/integration/regrade/idempotency_test.go` — ✅ TestRegradeUseNewKey (cùng key → cùng yêu cầu, key khác khi `queued` → 409 CONFLICT, `Supersede` evaluation đã superseded → `EVALUATION_NOT_EDITABLE`, trigger DB từ chối sửa)

### Implementation cho User Story 4

- [x] T036 [P] [US4] Domain `evaluation` (Supersede, Regrade) — file: `src/backend/modules/learning/internal/domain/evaluation/evaluation.go` — ✅ `internal/domain/evaluation/evaluation.go` (Supersede, CheckEditable, Regrade accept_all/use_new_key/drop_item)
- [x] T037 [US4] Command `CreateRegradeRequest`, query `GetRegradeRequest` + handler — file: `src/backend/modules/learning/internal/presentation/http/public/cms_regrade_handler.go` — ✅ `command.Regrades.Create`, `query.Regrades.Get`, `presentation/http/regrade.go`; migration `20261202000001_f031_regrade.sql` (`assessment.regrade_requests` RLS T + hàng platform qua `learning_platform`, trigger `evaluations_superseded_immutable`)
- [x] T038 [US4] Command `RunRegradeRequest` + job `regrade-runner` (theo lô, `tenancy.Bypass` cho yêu cầu platform) — file: `src/backend/modules/learning/internal/presentation/job/regrade_runner.go` — ✅ `command.Regrades.RunOnce` (claim `FOR UPDATE SKIP LOCKED` dưới `tenancy.Bypass(re_evaluation)`, mỗi attempt một transaction theo tenant, `Delivery.Grade` với key mới) + job `regrade-runner` 30 s trong `module.go`

## Phase 7: User Story 5 — Tính lại mastery khi đổi mô hình (Priority: P3)

### Tests cho User Story 5

- [x] T039 [P] [US5] Integration test recompute shadow không đổi mastery hiện hành, có báo cáo (F031-AC-23) — file: `src/backend/modules/learning/tests/integration/mastery/recompute_shadow_test.go` — ✅ `tests/integration/recompute_test.go` TestMasteryRecomputeAndActivate (shadow `we-1.1`: báo cáo 2 cặp, 1 đổi level, phân bố level; 2 dòng shadow; `objective_masteries` giữ `we-1.0`)
- [x] T040 [P] [US5] Integration test kích hoạt (System Admin, audit), không phát lại `mastered` (F031-AC-24) — file: `src/backend/modules/learning/tests/integration/mastery/activate_model_test.go` — ✅ TestMasteryRecomputeAndActivate (kích hoạt với id System Admin: `we-1.0` retired, đúng một active, mastery mang `we-1.1`, LO đã đạt không phát lại `mastered`, LO mới đạt phát đúng một, audit `mastery_model.activated`, kích hoạt lần hai/shadow version active bị từ chối)

### Implementation cho User Story 5

- [x] T041 [US5] Command `RecomputeMastery`, `ActivateMasteryModel` + CLI `worker learning mastery-recompute|mastery-model activate` (`src/backend/cmd/worker`) — file: `src/backend/modules/learning/internal/presentation/job/mastery_recompute.go` — ✅ `command.Recompute` (AddModel, Shadow, Activate) + `persistence/write.RecomputeRepo` + CLI `modules/learning/cmd/mastery` (`add-model`, `recompute`, `activate`) thay `worker learning …` [Proposed]; migration `20261203000001_f031_recompute.sql`

## Phase 8: User Story 6 — Hoàn thành bài học (Priority: P2)

**Goal**: Ghim version bài khi bắt đầu, xác định hoàn thành theo `completion_rule` ở server, phát `lesson.completed` đúng một lần.

**Independent Test**: Như spec.md US6.

### Tests cho User Story 6

- [x] T047 [P] [US6] Unit test `lesson.CompletionRule` (`all_required_activities`, `any_activity`, activity tùy chọn không chặn, hoàn thành ≠ đạt) (F031-AC-26) — file: `src/backend/modules/learning/internal/domain/lesson/completion_rule_test.go` — phải FAIL trước khi code — ✅ `internal/domain/lesson/completion_test.go` TestCompletionRule pass
- [x] T048 [P] [US6] Integration test ghim version: attempt `context_type = lesson` tạo session; publish version mới giữa chừng không đổi session; activity ngoài version ghim → `422 VALIDATION_FAILED`; `getLesson` trả version ghim + `completed` (F031-AC-25) — file: `src/backend/modules/learning/tests/integration/lesson/session_pin_test.go` — ✅ `tests/integration/lesson_completion_test.go` TestLessonCompletion (session ghim v1, publish v2 không đổi session, activity ngoài version → 422, `getLesson` trả v1 + `completed`)
- [x] T049 [P] [US6] Integration test hoàn thành: activity bắt buộc cuối được chấm → session `completed`, learning event + outbox `lesson.completed` cùng transaction, payload hợp lệ `lesson.completed.v1`; giao lại `activity.graded`, attempt thêm, `completeLesson` lặp → không phát lần hai; version mới → session mới (F031-AC-26, F031-AC-27) — file: `src/backend/modules/learning/tests/integration/lesson/completion_test.go` — ✅ TestLessonCompletion (activity bắt buộc cuối → session `completed`, một learning event + một outbox `lesson.completed` đủ trường, không PII; giao lại `activity.graded`, `completeLesson` lặp → không phát lần hai; version mới → session mới, event thứ hai)
- [x] T050 [P] [US6] Integration test `completeLesson`: đã hoàn thành → cùng `completed_at`; chưa thỏa → `409 CONFLICT` + `errors[]`; chưa có session → `409 CONFLICT`; trường client `duration_ms`/`score` bị bỏ qua (F031-AC-28) — file: `src/backend/modules/learning/tests/integration/lesson/complete_lesson_api_test.go` — ✅ TestLessonCompletion (chưa có session → 409; chưa thỏa → 409 + `errors[]` chỉ activity bắt buộc còn thiếu; trường client `duration_ms`/`score` bỏ qua; đã hoàn thành → cùng `completed_at`)
- [x] T051 [P] [US6] Test authorization + tenant `getLesson`/`completeLesson` (nhân sự 403, `TENANT_MISMATCH`, bài chưa gán 404) và quét payload không PII (F031-AC-29) — file: `src/backend/modules/learning/tests/integration/lesson/lesson_authz_tenant_test.go` — ✅ TestLessonCompletion (nhân sự → 403, `X-Tenant-Id` khác → 403 TENANT_MISMATCH, tenant B và bài chưa publish → 404, quét khóa PII payload)
- [x] T052 [P] [US6] Integration test `progress_daily.lessons_completed` +1 idempotent khi nhận `lesson.completed` (F031-AC-30) — file: `src/backend/modules/learning/tests/integration/progress/lessons_completed_test.go` — ✅ TestLessonCompletion (`lesson.completed` → `progress_daily.lessons_completed` = 1, giao lại vẫn 1)

### Implementation cho User Story 6

- [x] T053 [P] [US6] Domain `lesson` (`Session`, `CompletionRule`, `IdempotencyKey`) thuần — file: `src/backend/modules/learning/internal/domain/lesson/session.go` — ✅ `internal/domain/lesson/completion.go` (Body, Activities, Satisfied, trạng thái session)
- [x] T054 [US6] `LessonSessionPinner` gọi trong transaction start attempt của F030 khi `context_type = lesson` (port `port/write`), kiểm activity thuộc version ghim — file: `src/backend/modules/learning/internal/application/command/pin_lesson_session.go` — ✅ `command.Lessons.Pin` gọi trong transaction `StartActivity` khi `context_type = lesson` (`Attempts.Lessons`), kiểm activity thuộc version ghim; `Docs.ActivityEnabled` bật (khối activity của lesson)
- [x] T055 [US6] Command `CompleteLesson` (dùng chung cho consumer nội bộ `activity.graded` và `completeLesson`): `UnitOfWork` cập nhật session, `LearningEventRecorder` ghi `lesson.completed`, outbox `lesson.completed`; consumer `learning.lesson_completion` (`activity.graded` có `context_type = lesson`) — file: `src/backend/modules/learning/internal/application/command/complete_lesson.go`, `src/backend/modules/learning/internal/presentation/consumer/lesson_completion.go` — ✅ `command.Lessons` (OnGraded, Complete, `lesson.completed` learning event + outbox, khóa `lsn:<UUIDv5(student, version)>`) + consumer `learning.lesson_completion`; `progress_daily` nhận `lesson.completed`
- [x] T056 [US6] Handler `getLesson` (query version ghim + `completed`), `completeLesson` (Idempotency-Key, Problem `409`) — file: `src/backend/modules/learning/internal/presentation/http/public/lesson_handler.go` — ✅ `presentation/http/lessons.go` (`getLesson`, `completeLesson`) + `query.Lessons` + `persistence/read.LessonModel`

**Checkpoint**: US6 chạy và kiểm độc lập được; module game (F074) tiêu thụ `lesson.completed`.

## Phase 9: Polish & Cross-Cutting

- [x] T042 [P] Metric `eduworld_mastery_updates_total`, `eduworld_mastery_update_lag_seconds`, `eduworld_regrade_attempts_total`, `eduworld_lesson_completions_total` — file: `src/backend/modules/learning/internal/infrastructure/observability/mastery_metrics.go` — ✅ `observability/mastery_metrics.go`: `eduworld_mastery_updates_total`, `eduworld_mastery_update_lag_seconds`, `eduworld_learning_events_partition_months_ahead`, `eduworld_regrade_attempts_total{action}`, `eduworld_lesson_completions_total{trigger}`
- [x] T043 [P] Ghi CHANGELOG; đồng bộ LEARNING_ANALYTICS §8 (`/learning/mastery/me` dùng `getMyProgress`) — file: `docs/02-learning/LEARNING_ANALYTICS.md` — ✅ CHANGELOG; LEARNING_ANALYTICS §8 (`getMyProgress` thay `/learning/mastery/me`, level sau decay); DATABASE_DESIGN §2.7; MONITORING (metric + alert `LearningPartitionsMissing`)
- [x] T044 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/031-learning-events-mastery/quickstart.md` — ✅ quickstart §5–§7: bằng chứng tự động cho kịch bản 1–27 (test tích hợp trên PostgreSQL/NATS/Valkey thật); chạy tay trên stack compose chưa làm (đăng nhập game bị chặn quyền ở phiên agent, như F030 T051)

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → (US3, US4 song song) → US5 → US6 (sau US1, song song US2–US5) → Polish.
- T015 thay `LearningEventWriter` của F030 (cùng interface port). T034 cần T024.
- US6 sau US1 (T015) và T046; T054 sửa transaction start attempt của F030 (cùng module). T052 cần T028.

## Definition of Done

- [x] Mọi tiêu chí F031-AC-* có test pass (ghi mã test/TC) — ✅ 30/30 AC: domain mastery/evaluation/lesson, persistence envelope, integration mastery_test, regrade_test, recompute_test, lesson_completion_test, valkey_down_test
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `learning` — ✅ gate trước mỗi lần push (quickstart §5–§8)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — ✅ clock giả chỉ trong test
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — ✅ DATABASE_DESIGN, MONITORING, LEARNING_ANALYTICS, CHANGELOG; không đổi giá trị CONFIG_DEFAULTS
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — ✅ Done 2026-11-20
