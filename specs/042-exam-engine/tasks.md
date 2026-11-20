# Tasks: F042 — Exam Engine: 7 hình thức đề, random theo blueprint, thi có giờ, chấm lại đề

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `exam`, `exam/paper`, `exam/regrade`; đăng ký handler, consumer, job trong composition root — file: `src/backend/modules/learning/cmd/api/main.go`, `src/backend/modules/learning/cmd/worker/main.go`
- [x] T002 [P] Đọc tham số `score_scale`, `suspicious_speed_flag`, `graded_assignment_release_default` từ cấu hình platform/tenant — file: `src/backend/modules/learning/internal/application/port/learning_config.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Delta contract: sửa `ExamSessionCreate`/`ExamSession` (accommodations, late_start_policy, audience, show_*), `startExamAttempt` (device_token, take_over, `409 EXAM_ACTIVE_ON_OTHER_DEVICE`); thêm `listExamLibrary`, `listExamSessionPapers`, `getExamAttemptAnalysis` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [x] T004 [P] Payload schema `exam.started/submitted/graded/results_released/regraded/opened/closed.v1` (`attempt.re_evaluated.v1` do F031 sở hữu — chỉ dùng) — file: `specs/events/payloads/learning/` — kiểm: `make specs-lint`
- [x] T005 ent schema: cột mới `exam_sessions`, `exam_papers`, `attempts` (cột `regrade_requests` do F031 thêm); bảng mới `exam_participations` (mọi chuỗi `MaxLen`) — file: `src/backend/modules/learning/ent/schema/exam_session.go`, `src/backend/modules/learning/ent/schema/exam_paper.go`, `src/backend/modules/learning/ent/schema/exam_participation.go`, `src/backend/modules/learning/ent/schema/attempt.go` — kiểm: `make migrate-diff MOD=learning name=f042_exam_engine`, `make migrate-lint MOD=learning`
- [x] T006 RLS, partial unique, trigger bất biến `exam_papers.items` trong migration — file: `src/backend/modules/learning/migrations/<ts>_f042_exam_engine.sql`
- [x] T007 [P] Cập nhật DATABASE_DESIGN §2.6, §2.16 (cột/bảng mới, ghi chú `type`/`duration_seconds`) — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Tạo đề từ blueprint (Priority: P1) 🎯 MVP

**Goal**: Nhà trường tạo ExamSession, đề được sinh và khóa.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T008 [P] [US1] Unit test F042-AC-01, F042-AC-02: mặc định theo loại đề; sinh `fixed`/`independent_draw` đúng ma trận; cùng seed cho cùng kết quả — file: `src/backend/modules/learning/tests/unit/exam/paper_generator_test.go` — phải FAIL trước khi code
- [x] T009 [P] [US1] Unit test F042-AC-03, F042-AC-08: blueprint `immediate`, tổng điểm lệch, pool thiếu (`QUESTION_POOL_INSUFFICIENT`), câu trùng, loại không tự chấm; maxLength/cửa sổ sai → `422` — file: `src/backend/modules/learning/tests/unit/exam/session_validation_test.go`
- [x] T010 [P] [US1] Integration test F042-AC-05: `deadline_at` cộng gia hạn đúng học sinh; học sinh ngoài đối tượng bị từ chối — file: `src/backend/modules/learning/tests/integration/exam/accommodation_test.go`
- [x] T011 [P] [US1] Integration test F042-AC-06, F042-AC-07: scope `school_admin`/`academic_manager` (`PERMISSION_DENIED`, `404` khác trường/tenant), System Admin cần `X-Tenant-Id`; idempotent create; audit `exam_session.created` — file: `src/backend/modules/learning/tests/integration/exam/create_session_authz_test.go`
- [x] T012 [P] [US1] API test F042-AC-04: `listExamSessionPapers` cho người tạo đề; học sinh `403` — file: `src/backend/modules/learning/tests/integration/exam/papers_preview_test.go`

### Implementation cho User Story 1

- [x] T013 [P] [US1] Domain `exam.Session`, `exam.ReleasePolicy`, `exam.Accommodation` — file: `src/backend/modules/learning/internal/domain/exam/session.go`, `src/backend/modules/learning/internal/domain/exam/release_policy.go`, `src/backend/modules/learning/internal/domain/exam/accommodation.go`
- [x] T014 [P] [US1] Domain `paper.Generator`, interface `paper.VariantBalancer` + `NoopBalancer`, `paper.Shuffle` — file: `src/backend/modules/learning/internal/domain/exam/paper/generator.go`, `src/backend/modules/learning/internal/domain/exam/paper/variant_balancer.go`
- [x] T015 [US1] Port `BlueprintReader`, `QuestionPoolReader` (published, khả dụng cho tenant) + repository ent — file: `src/backend/modules/learning/internal/application/port/exam.go`, `src/backend/modules/learning/internal/infrastructure/persistence/exam_session_repo.go`
- [x] T016 [US1] Command `CreateExamSession` (snapshot target, sinh paper, outbox, audit) và `CancelExamSession` (cho F041) — file: `src/backend/modules/learning/internal/application/command/exam/create_exam_session.go`, `src/backend/modules/learning/internal/application/command/exam/cancel_exam_session.go`
- [x] T051 [P] [US1] Integration test F042-AC-31 (nửa sau F041 T012 chuyển sang): với adapter thật, tạo + publish bài `graded` blueprint → ExamSession ghim blueprint version, cùng đối tượng/cửa sổ, `exam_session_id` lưu trên bài; hủy bài → session `cancelled`; publish lỗi sinh đề → rollback cả bài — file: `src/backend/modules/learning/tests/integration/exam/assignment_exam_session_test.go` — phải FAIL trước khi code
- [x] T052 [US1] Adapter `ExamSessionCreator` (port của F041) gọi command `CreateExamSession`/`CancelExamSession`; thay adapter mặc định "chưa khả dụng" của F041 trong `module.go`; `PublishAssignment`/`CancelAssignment` của F041 lưu `exam_session_id` qua port — file: `src/backend/modules/learning/internal/infrastructure/examsession/exam_session_creator.go`, `src/backend/modules/learning/module.go`
- [x] T017 [US1] Query `ListExamSessions`, `GetExamSession`, `ListExamSessionPapers` + handler — file: `src/backend/modules/learning/internal/application/query/exam/`, `src/backend/modules/learning/internal/presentation/http/public/exam_handler.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Làm bài có giờ theo server (Priority: P1)

**Goal**: Start, lưu câu, resume, một thiết bị, hết giờ tự nộp.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T018 [P] [US2] Integration test F042-AC-09, F042-AC-13: `deadline_at` theo `late_start_policy`, chọn biến thể `hash mod N`, `server_time`; start lại trả attempt cũ — file: `src/backend/modules/learning/tests/integration/exam/start_attempt_test.go` — phải FAIL trước khi code
- [x] T019 [P] [US2] Contract test F042-AC-10, F042-AC-12: payload/response không đáp án, không gợi ý; media signed URL private — file: `src/backend/modules/learning/tests/contract/exam/exam_payload_test.go`
- [x] T020 [P] [US2] API test F042-AC-11: `EXAM_NOT_ASSIGNED`, `EXAM_NOT_OPEN`, `EXAM_WINDOW_CLOSED`, `EXAM_MAX_ATTEMPTS` — file: `src/backend/modules/learning/tests/integration/exam/start_errors_test.go`
- [x] T021 [P] [US2] Integration test F042-AC-14: `EXAM_ACTIVE_ON_OTHER_DEVICE`, `take_over` vô hiệu thiết bị cũ + audit — file: `src/backend/modules/learning/tests/integration/exam/device_switch_test.go`
- [x] T022 [P] [US2] Integration test (đồng hồ giả) F042-AC-15, F042-AC-16: `EXAM_TIME_OVER`; job `exam.autosubmit` `auto_timeout`; đóng cửa sổ → auto-submit + `absent`; hai worker không nộp trùng — file: `src/backend/modules/learning/tests/integration/exam/autosubmit_test.go`
- [x] T023 [P] [US2] Test tenant isolation + authorization F042-AC-17 cho mọi endpoint exam (staff/học sinh khác/tenant khác/header sai) — file: `src/backend/modules/learning/tests/integration/exam/authz_tenant_test.go`

### Implementation cho User Story 2

- [x] T024 [P] [US2] Domain `exam.Deadline` (late start, gia hạn, ân hạn) — file: `src/backend/modules/learning/internal/domain/exam/deadline.go`
- [x] T025 [US2] Command `StartExamAttempt`, `SaveExamAnswer`, `SwitchExamDevice` (participation, append-only answer) — file: `src/backend/modules/learning/internal/application/command/exam/start_exam_attempt.go`, `src/backend/modules/learning/internal/application/command/exam/save_exam_answer.go`, `src/backend/modules/learning/internal/application/command/exam/switch_exam_device.go`
- [x] T026 [US2] Adapter ký URL media private — file: `src/backend/modules/learning/internal/infrastructure/storage/media_signer.go`
- [x] T027 [US2] Job `exam.autosubmit` (`FOR UPDATE SKIP LOCKED`) và `exam-window` (scheduled→open→closed, absent) — file: `src/backend/modules/learning/internal/application/command/exam/auto_submit_expired.go`, `src/backend/modules/learning/internal/presentation/job/exam_autosubmit.go`, `src/backend/modules/learning/internal/presentation/job/exam_window.go`
- [x] T028 [US2] Handler `startExamAttempt`, `saveExamAnswer` — file: `src/backend/modules/learning/internal/presentation/http/public/exam_handler.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Nộp bài, chấm và công bố kết quả (Priority: P1)

**Goal**: Nộp idempotent, chấm worker, công bố theo chính sách, phân tích + R5.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T029 [P] [US3] Integration test F042-AC-18: nộp cùng key / khác key / sau auto-submit / đồng thời — file: `src/backend/modules/learning/tests/integration/exam/submit_idempotency_test.go` — phải FAIL trước khi code
- [x] T030 [P] [US3] Integration test F042-AC-19, F042-AC-24: chấm theo version ghim, bỏ qua điểm client, `exam.graded`; kết quả còn sau `FLUSHALL` Valkey — file: `src/backend/modules/learning/tests/integration/exam/grade_durability_test.go`
- [x] T031 [P] [US3] Integration test F042-AC-20, F042-AC-21: `EXAM_RESULTS_NOT_RELEASED` theo `immediate`/`after_close`/`manual`; `releaseExamResults` quyền, idempotent, audit — file: `src/backend/modules/learning/tests/integration/exam/result_release_test.go`
- [x] T032 [P] [US3] Integration test F042-AC-22, F042-AC-23: `getExamResult` điểm/section, `getExamAttemptAnalysis` theo LO, câu sai, `show_*`, 1–3 LO yếu; gợi ý R5 được tạo — file: `src/backend/modules/learning/tests/integration/exam/result_analysis_test.go`
- [x] T033 [P] [US3] Unit test F042-AC-25: cờ `suspicious_speed` không đổi kết quả — file: `src/backend/modules/learning/tests/unit/exam/suspicious_speed_test.go`

### Implementation cho User Story 3

- [x] T034 [US3] Command `SubmitExamAttempt` (UPDATE có điều kiện) + `GradeExamAttempt` + consumer chấm — file: `src/backend/modules/learning/internal/application/command/exam/submit_exam_attempt.go`, `src/backend/modules/learning/internal/application/command/exam/grade_exam_attempt.go`, `src/backend/modules/learning/internal/presentation/consumer/exam_grade_consumer.go`
- [x] T035 [US3] Command `ReleaseExamResults` + công bố `after_close` trong job `exam-window` — file: `src/backend/modules/learning/internal/application/command/exam/release_exam_results.go`
- [x] T036 [US3] Query `GetExamResult` (điểm, section) và `GetExamAttemptAnalysis` (phân tích LO, câu sai, đề xuất ôn tập) — file: `src/backend/modules/learning/internal/application/query/exam/get_exam_result.go`, `src/backend/modules/learning/internal/application/query/exam/get_exam_attempt_analysis.go`
- [x] T037 [P] [US3] Rule gợi ý R5 đăng ký vào `RuleRegistry` (F032) — file: `src/backend/modules/learning/internal/domain/recommendation/r5_exam_followup.go`
- [x] T038 [US3] Ghi integrity signal `suspicious_speed` khi chấm — file: `src/backend/modules/learning/internal/application/command/exam/grade_exam_attempt.go`
- [x] T039 [US3] Handler `submitExamAttempt`, `getExamResult`, `getExamAttemptAnalysis`, `releaseExamResults` — file: `src/backend/modules/learning/internal/presentation/http/public/exam_handler.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Luyện đề tự do (Priority: P2)

**Goal**: Thư viện đề luyện, rút đề riêng mỗi lượt.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T040 [P] [US4] API test F042-AC-26: `listExamLibrary` chỉ session luyện đang mở, đúng khối/đối tượng/tenant — file: `src/backend/modules/learning/tests/integration/exam/library_test.go` — phải FAIL trước khi code
- [x] T041 [P] [US4] Unit test F042-AC-27: `per_student` seed theo session/học sinh/lượt, làm lại theo giới hạn, kết quả `immediate` — file: `src/backend/modules/learning/tests/unit/exam/per_student_draw_test.go`

### Implementation cho User Story 4

- [x] T042 [US4] `paper.Generator` nhánh `per_student` trong `StartExamAttempt` — file: `src/backend/modules/learning/internal/domain/exam/paper/generator.go`
- [x] T043 [US4] Query `ListExamLibrary` + handler `listExamLibrary` — file: `src/backend/modules/learning/internal/application/query/exam/list_exam_library.go`, `src/backend/modules/learning/internal/presentation/http/public/exam_handler.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Chấm lại đề khi câu hỏi lỗi (Priority: P2)

**Goal**: RegradeRequest `scope = exam_session` chạy tự động, lưu lịch sử điểm.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T044 [P] [US5] Integration test F042-AC-28, F042-AC-30: `accept_all`, `use_new_key` (version sửa lỗi đã publish), `drop_item`; revision điểm cũ/mới; `exam.regraded` + `attempt.re_evaluated`; version cũ không bị sửa — file: `src/backend/modules/learning/tests/integration/exam/regrade_test.go` — phải FAIL trước khi code
- [x] T045 [P] [US5] API test F042-AC-29: `school_admin` → 403; trùng yêu cầu đang chạy → `409 CONFLICT`; cùng key không bù hai lần; audit `content.re_evaluated` — file: `src/backend/modules/learning/tests/integration/exam/regrade_authz_test.go`

### Implementation cho User Story 5

- [x] T046 [P] [US5] Domain `regrade.Plan` (tính điểm mới, loại câu, chia section an toàn) — file: `src/backend/modules/learning/internal/domain/exam/regrade/regrade.go`
- [x] T047 [US5] Command `RunExamRegrade` + consumer, nhánh `scope = exam_session` của handler `createRegradeRequest` — file: `src/backend/modules/learning/internal/application/command/exam/run_exam_regrade.go`, `src/backend/modules/learning/internal/presentation/consumer/regrade_consumer.go`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: Polish & Cross-Cutting

- [x] T048 [P] Metric, span theo plan.md §Quan sát; kiểm log không chứa câu trả lời (R14) — file: `src/backend/modules/learning/internal/infrastructure/observability/exam_metrics.go`
- [x] T049 [P] Kiểm EXAM_ENGINE §11 và API_DESIGN §9.5 khớp (`/analysis`, `/report`, `/library`, `/papers` đã chốt RB-213); cập nhật CHANGELOG, trạng thái F042 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [x] T050 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/042-exam-engine/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T003–T007) → US1 → US2 → US3 → US4, US5 (song song) → Polish.
- T025, T042 cùng chạm luồng start: làm tuần tự. T034, T038 cùng sửa `grade_exam_attempt.go`.
- F044 thay `NoopBalancer` sau khi T014 xong. F041 phải `Done` trước (thứ tự F040 → F041 → F042, CODE_PLAN §4.1); T052 cần T016 và port `ExamSessionCreator` của F041 T014.

## Definition of Done

- [x] Mọi tiêu chí F042-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Bằng chứng triển khai (2026-11-20)

Đường dẫn thực tế khác gợi ý của task (repo dùng file test cạnh code và thư mục `tests/integration` phẳng):

| Task | Hiện thực | Test |
|---|---|---|
| T001, T002 | `module.go` (Exams, ExamSessionCreator, consumer `learning.exam_grade`, job `exam-window`/`exam-autosubmit` 15 s, R5); `command.ExamConfig` (`exam_offline_sync_grace` 30 s, `score_scale` 10, `suspicious_speed_flag` 1,5 s/câu, `graded_assignment_release_default`) | `cmd/internal/wiring` compose test |
| T003 | `specs/openapi/eduworld-api.v1.yaml`: `ExamSessionCreate`/`ExamSession` (accommodations, late_start_policy, audience, show_*, score_policy, duration_seconds, server_time), `ExamDeviceToken`, `ExamAttemptStart`, `listExamLibrary`, `listExamSessionPapers`, `getExamAttemptAnalysis`, `AttemptReview.score_scaled/section_scores`, `answered_at_client` | split-public, portal `gen:api`, routematrix |
| T004 | `specs/events/payloads/learning/exam.{opened,closed,started,submitted,graded,results_released,regraded}.v1.schema.json` | validate-schemas; `outboxPayloads` kiểm payload trong integration |
| T005, T006 | `migrations/20261222000001_f042_exam_engine.sql` (exam_sessions, exam_papers + trigger bất biến, exam_participations, cột đề của attempts qua entddl, RLS, `_live`); ent `Attempt` thêm 4 cột | `TestMigrationMatchesEnt`, trigger kiểm trong `TestExamSessionCreate` |
| T007 | DATABASE_DESIGN §2.6 | — |
| T008, T009, T013, T014, T024, T041, T042 | `internal/domain/exam`, `internal/domain/exam/paper` | `TestDefaultsOf`, `TestSettingsValidate`, `TestDeadline`, `TestVariantAndSeed`, `TestResultsVisible`, `TestGenerateMatrixAndSeed`, `TestGeneratePoolInsufficient`, `TestGenerateFixedAndExamOnly`, `TestOrder`, `TestCheckMatrix` |
| T010–T012, T015–T017 | `command/exams.go`, `persistence/write/exam_repo.go`, `query/exams.go`, `persistence/read/exam.go`, `http/exams.go` | `TestExamSessionCreate` |
| T051, T052 | `command/exam_session_creator.go` (adapter cùng package command), `PublishAssignment` gọi `examSession` | `TestExamSessionOfAssignment` |
| T018–T028 | `command/exam_attempts.go` (start/save/submit/auto-submit), `job/exams.go`; media đề đi qua render của F030 (URL ký, không CDN) | `TestExamAttemptFlow` |
| T029–T039 | grading consumer `consumer/exams.go`, release, result/analysis, R5 (`recommendation.R5`, `command.ExamFollowups`), integrity signal | `TestExamResultRelease` |
| T040, T043 | `query.Exams.Library` | `TestExamLibraryPerStudent` |
| T044–T047 | `command/regrade.go` nhánh `exam_session`, `RegradeRepo.AffectedAttempts`, `Exams.RegradeTarget/Regraded` | `TestExamRegrade` (+ `TestRegrade*` F031 không đổi) |
| T048 | `observability/exam_metrics.go` (4 metric, nhãn không ID); log use case không chứa câu trả lời | gate (golangci, ewlint) |
| T049, T050 | CHANGELOG, CODE_PLAN, quickstart §5 | — |

Phụ thuộc: `ExamAttemptAnalysis.mastery_level` trả `null` (spec Clarifications 2026-11-20 [Proposed]); cân bằng biến thể (`VariantBalancer`) và báo cáo đề thuộc F044.
