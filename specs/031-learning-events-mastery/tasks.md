# Tasks: F031 — Learning events, mastery v1, evaluation và re-evaluate

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `domain/learningrecord`, `domain/mastery`, `domain/evaluation`; wiring — file: `modules/learning/cmd/api/main.go`
- [ ] T002 [P] Đăng ký durable consumer `learning.mastery_on_graded` (`activity.graded`, `attempt.re_evaluated`) và `learning.progress_daily` (`activity.submitted`, `activity.graded`); job `learning-events-partitions`, `regrade-runner` — file: `modules/learning/cmd/worker/main.go`
- [ ] T003 [P] Seed `mastery_model_configs` `we-1.0` `active` từ LEARNING_ANALYTICS §3.1 — file: `modules/learning/seed/mastery_model_configs.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: payload `attempt.re_evaluated`, `learning_objective.progressed`, `learning_objective.mastered` v1; `getRegradeRequest`; mô tả quyền `getStudentProgress` — file: `specs/events/payloads/learning/`, `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`
- [ ] T005 ent schema `MasteryModelConfig`, `ObjectiveMastery` (cột mới), `MasteryHistory` (partition), `ProgressDaily`, `RegradeRequest` (H), `Evaluation` (cột mới), `MasteryRecomputeRun`, `ObjectiveMasteryShadow` — file: `modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f031_mastery`, `make migrate-lint MOD=learning`
- [ ] T006 Cập nhật DATABASE_DESIGN §2.6a, §2.7, §2.16 (bảng mới, cột mới, `re_evaluation`, `regrade_requests` H) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T007 Migration: GRANT `INSERT, SELECT` trên `learning.learning_events`, `learning.mastery_history` cho role app — file: `modules/learning/migrations/`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Ghi learning event làm bằng chứng học tập (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T008 [P] [US1] Contract test mọi learning event ghi ra hợp lệ `learning-event.schema.json`, đủ envelope (F031-AC-01) — file: `modules/learning/tests/contract/learning_event_schema_test.go` — phải FAIL trước khi code
- [ ] T009 [P] [US1] Integration test idempotency event + consumer `processed_events` (F031-AC-02) — file: `modules/learning/tests/integration/learningrecord/idempotency_test.go`
- [ ] T010 [P] [US1] Integration test role app không UPDATE/DELETE được (F031-AC-03) — file: `modules/learning/tests/integration/learningrecord/append_only_test.go`
- [ ] T011 [P] [US1] Integration test job partition ≥ 2 tháng + cảnh báo (F031-AC-04) — file: `modules/learning/tests/integration/learningrecord/partition_job_test.go`
- [ ] T012 [P] [US1] Unit test từ chối payload ngoài schema/PII (F031-AC-05) — file: `modules/learning/internal/domain/learningrecord/validate_test.go`
- [ ] T013 [P] [US1] Chaos integration test dừng Valkey khi nộp + cập nhật mastery (F031-AC-06) — file: `modules/learning/tests/integration/learningrecord/valkey_down_test.go`
- [ ] T014 [P] [US1] Test tenant isolation RLS/`TenantScope` cho learning event + mastery (F031-AC-07) — file: `modules/learning/tests/integration/learningrecord/tenant_isolation_test.go`

### Implementation cho User Story 1

- [ ] T015 [P] [US1] `LearningEventRecorder` (envelope, `class_id` từ `ref.enrollments`, kiểm schema, khóa idempotency) thay writer của F030 — file: `modules/learning/internal/domain/learningrecord/recorder.go`
- [ ] T016 [US1] Job `learning-events-partitions` + metric `eduworld_learning_events_partition_months_ahead` — file: `modules/learning/internal/presentation/job/learning_events_partitions.go`

## Phase 4: User Story 2 — Mastery theo LO từ nhiều bằng chứng (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Golden test `WeightedEvidence` bảng LEARNING_ANALYTICS §3.3 + ví dụ bổ sung (F031-AC-09) — file: `modules/learning/internal/domain/mastery/golden_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Unit test level `mastered` cần `n_min`/`d_min`; `Effective` decay không ghi đè (F031-AC-10, F031-AC-11) — file: `modules/learning/internal/domain/mastery/level_test.go`
- [ ] T019 [P] [US2] Integration test consumer áp evidence (`is_scored`, `rapid_guess`), lịch sử, `model_version` (F031-AC-08, F031-AC-12) — file: `modules/learning/tests/integration/mastery/consumer_test.go`
- [ ] T020 [P] [US2] Integration test `learning_objective.mastered` một lần, `progressed` mỗi thay đổi (F031-AC-13) — file: `modules/learning/tests/integration/mastery/objective_events_test.go`
- [ ] T021 [P] [US2] Integration test đồng thời cùng học sinh → kết quả tuần tự (F031-AC-14) — file: `modules/learning/tests/integration/mastery/concurrency_test.go`

### Implementation cho User Story 2

- [ ] T022 [P] [US2] Domain `mastery` (MasteryModel, WeightedEvidence, Level, Effective) — file: `modules/learning/internal/domain/mastery/weighted_evidence.go`
- [ ] T023 [US2] Command `ApplyMasteryEvidence` (đọc `question.answered` của attempt, khóa lạc quan, outbox `learning_objective.*`) — file: `modules/learning/internal/application/command/apply_mastery_evidence.go`
- [ ] T024 [US2] Consumer `learning.mastery_on_graded` + repo mastery — file: `modules/learning/internal/presentation/consumer/mastery_on_graded.go`

## Phase 5: User Story 3 — Tiến độ cho học sinh và nhà trường (Priority: P2)

### Tests cho User Story 3

- [ ] T025 [P] [US3] Integration test `getMyProgress` chỉ của mình, level sau decay (F031-AC-15) — file: `modules/learning/tests/integration/progress/my_progress_test.go`
- [ ] T026 [P] [US3] Test authorization + tenant `getStudentProgress` (trường khác 404, môn ngoài scope 403, audit) (F031-AC-16) — file: `modules/learning/tests/integration/progress/student_progress_authz_test.go`
- [ ] T027 [P] [US3] Integration test `progress_daily` idempotent (F031-AC-17) — file: `modules/learning/tests/integration/progress/progress_daily_test.go`

### Implementation cho User Story 3

- [ ] T028 [US3] Command `UpsertProgressDaily` + consumer `learning.progress_daily` — file: `modules/learning/internal/presentation/consumer/progress_daily.go`
- [ ] T029 [US3] Query `GetMyProgress`, `GetStudentProgress` (audit `learning.profile_viewed`) + handler — file: `modules/learning/internal/presentation/http/public/progress_handler.go`

## Phase 6: User Story 4 — Chấm lại khi nội dung có lỗi (Priority: P2)

### Tests cho User Story 4

- [ ] T030 [P] [US4] Unit test `evaluation.Regrade` cho `accept_all`, `use_new_key`, `drop_item` (F031-AC-20) — file: `modules/learning/internal/domain/evaluation/regrade_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US4] Integration test `createRegradeRequest` hợp lệ, `corrected_version_id`, audit (F031-AC-18) — file: `modules/learning/tests/integration/regrade/create_test.go`
- [ ] T032 [P] [US4] Test authorization + tenant/subject scope `createRegradeRequest` (F031-AC-19) — file: `modules/learning/tests/integration/regrade/authz_scope_test.go`
- [ ] T033 [P] [US4] Integration test job: evaluation mới/cũ superseded, attempt không đổi, `attempt.re_evaluated`, Bypass có audit (F031-AC-20) — file: `modules/learning/tests/integration/regrade/runner_test.go`
- [ ] T034 [P] [US4] Integration test mastery tính lại sau `attempt.re_evaluated` (F031-AC-21) — file: `modules/learning/tests/integration/regrade/mastery_adjust_test.go`
- [ ] T035 [P] [US4] Integration test Idempotency-Key, `409` yêu cầu trùng, `EVALUATION_NOT_EDITABLE` (F031-AC-22) — file: `modules/learning/tests/integration/regrade/idempotency_test.go`

### Implementation cho User Story 4

- [ ] T036 [P] [US4] Domain `evaluation` (Supersede, Regrade) — file: `modules/learning/internal/domain/evaluation/evaluation.go`
- [ ] T037 [US4] Command `CreateRegradeRequest`, query `GetRegradeRequest` + handler — file: `modules/learning/internal/presentation/http/public/cms_regrade_handler.go`
- [ ] T038 [US4] Command `RunRegradeRequest` + job `regrade-runner` (theo lô, `tenancy.Bypass` cho yêu cầu platform) — file: `modules/learning/internal/presentation/job/regrade_runner.go`

## Phase 7: User Story 5 — Tính lại mastery khi đổi mô hình (Priority: P3)

### Tests cho User Story 5

- [ ] T039 [P] [US5] Integration test recompute shadow không đổi mastery hiện hành, có báo cáo (F031-AC-23) — file: `modules/learning/tests/integration/mastery/recompute_shadow_test.go`
- [ ] T040 [P] [US5] Integration test kích hoạt (System Admin, audit), không phát lại `mastered` (F031-AC-24) — file: `modules/learning/tests/integration/mastery/activate_model_test.go`

### Implementation cho User Story 5

- [ ] T041 [US5] Command `RecomputeMastery`, `ActivateMasteryModel` + CLI `worker learning mastery-recompute|mastery-model activate` (`cmd/worker`) — file: `modules/learning/internal/presentation/job/mastery_recompute.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T042 [P] Metric `eduworld_mastery_updates_total`, `eduworld_mastery_update_lag_seconds`, `eduworld_regrade_attempts_total` — file: `modules/learning/internal/infrastructure/observability/mastery_metrics.go`
- [ ] T043 [P] Ghi CHANGELOG; đồng bộ LEARNING_ANALYTICS §8 (`/learning/mastery/me` dùng `getMyProgress`) — file: `docs/02-learning/LEARNING_ANALYTICS.md`
- [ ] T044 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/031-learning-events-mastery/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → (US3, US4 song song) → US5 → Polish.
- T015 thay `LearningEventWriter` của F030 (cùng interface port). T034 cần T024.

## Definition of Done

- [ ] Mọi tiêu chí F031-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `learning`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
