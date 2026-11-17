# Tasks: F041 — Giao bài của nhà trường: assignment, `ResultRelease`, gia hạn/miễn, `my-assignments`

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `assignment` (domain, application, persistence, handler, job) và đăng ký route/consumer/job — file: `modules/learning/cmd/api/main.go`, `modules/learning/cmd/worker/main.go`
- [ ] T002 [P] Đọc tham số `graded_assignment_release_default`, `late_window_hours`, `assignment_remind_limit`, `score_scale` từ cấu hình platform/tenant — file: `modules/learning/internal/application/port/lms_config.go`, `modules/learning/internal/infrastructure/persistence/lms_config_reader.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta contract OpenAPI: `AssignmentSubmission.attempts_used`, query `objective_ids` của `listGameActivities` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T004 [P] Payload schema `assignment.results_released.v1`, `assignment.completed.v1`, `assignment.closed.v1`, `assignment.submission_updated.v1`, `assignment.reminded.v1`; bổ sung 3 event mới vào EVENT_DESIGN §4.3 — file: `specs/events/payloads/learning/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint`
- [ ] T005 [P] Xác nhận payload tiêu thụ cho `ref.game_activities` đã có: `content.published` (game, khối `catalog` — owner F071, F078 bổ sung), `content.rolled_back`/`content.unpublished` (game, owner F078), `tenant_data.assigned`/`tenant_data.unassigned` (owner F045); thiếu → chặn T041, không tự tạo — file: `specs/events/payloads/game/`, `specs/events/payloads/identity/` (chỉ đọc)
- [ ] T006 ent schema `lms.assignments` (thêm cột), `lms.assignment_targets`, `lms.assignment_target_students`, `lms.assignment_submissions`, `lms.assignment_student_exceptions`, `lms.assignment_reminders`, `ref.game_activities` (mọi chuỗi `MaxLen`, CHECK, partial unique, RLS) — file: `modules/learning/ent/schema/assignment.go`, `assignment_target.go`, `assignment_target_student.go`, `assignment_submission.go`, `assignment_student_exception.go`, `assignment_reminder.go`, `ref_game_activity.go` — kiểm: `make migrate-diff MOD=learning name=f041_assignments`, `make migrate-lint MOD=learning`
- [ ] T007 [P] Cập nhật DATABASE_DESIGN §2.14/§2.16 (cột assignment theo spec, bỏ `result_visibility`, bảng mới, `target_type` class/study_group/students) — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Tạo và giao bài (Priority: P1) 🎯 MVP

**Goal**: Tạo, sửa, publish, hủy bài giao.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T008 [P] [US1] Unit test bảng chuyển trạng thái và trường sửa được F041-AC-10, F041-AC-11; `ReleasePolicy` F041-AC-06 — file: `modules/learning/tests/unit/assignment/assignment_state_test.go` — phải FAIL trước khi code
- [ ] T009 [P] [US1] API test F041-AC-01, F041-AC-02, F041-AC-03: tạo draft + `assignment.created`; `CONTENT_NOT_PUBLISHED`, `RESOURCE_NOT_ASSIGNED`, `ACTIVITY_TYPE_NOT_SUPPORTED` — file: `modules/learning/tests/integration/assignment/create_assignment_test.go`
- [ ] T010 [P] [US1] API test F041-AC-04, F041-AC-05: AM ngoài môn `403 PERMISSION_DENIED` + audit; đối tượng trường/tenant khác `404` — file: `modules/learning/tests/integration/assignment/assignment_scope_test.go`
- [ ] T011 [P] [US1] API test F041-AC-07: validation độ dài/khoảng → `422 VALIDATION_FAILED` — file: `modules/learning/tests/integration/assignment/assignment_validation_test.go`
- [ ] T012 [P] [US1] Integration test F041-AC-08, F041-AC-09: publish snapshot đối tượng, `scheduled`/`open`, `assignment.published`, audit; tạo/sửa bài graded + blueprint khi chỉ có adapter mặc định → `422 VALIDATION_FAILED`, không ghi DB (nửa "publish → ExamSession" chuyển sang F042 T051) — file: `modules/learning/tests/integration/assignment/publish_assignment_test.go`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Domain `assignment.Assignment`, `ReleasePolicy`, `Window`, `ScorePolicy` — file: `modules/learning/internal/domain/assignment/assignment.go`, `release_policy.go`, `window.go`, `score_policy.go`
- [ ] T014 [US1] Port `TargetResolver` (lớp từ `ref.enrollments`, học sinh, `StudyGroupMemberReader` của F040) và port `ExamSessionCreator` (F041 sở hữu: `Available`, `Create`, `Cancel`) + adapter mặc định `unavailableExamSessionCreator` đăng ký ở `module.go` (adapter gọi `CreateExamSession`/`CancelExamSession` thuộc F042 T052) — file: `modules/learning/internal/application/port/assignment_ports.go`, `modules/learning/internal/infrastructure/examsession/unavailable_creator.go`
- [ ] T015 [US1] Command `CreateAssignment`, `UpdateAssignment`, `PublishAssignment`, `CancelAssignment`, `ArchiveAssignment` (outbox + audit) — file: `modules/learning/internal/application/command/assignment/create_assignment.go`, `update_assignment.go`, `publish_assignment.go`, `cancel_assignment.go`, `archive_assignment.go`
- [ ] T016 [US1] Repository ent + query `ListAssignments`, `GetAssignment` có scope — file: `modules/learning/internal/infrastructure/persistence/assignment/assignment_repo.go`, `modules/learning/internal/application/query/assignment/list_assignments.go`
- [ ] T017 [US1] Handler `listAssignments`, `createAssignment`, `getAssignment`, `updateAssignment`, `publishAssignment`, `cancelAssignment`, `archiveAssignment` — file: `modules/learning/internal/presentation/http/public/assignment_handler.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Học sinh nhận và làm bài trong game (Priority: P1)

**Goal**: `listMyAssignments`, guard attempt `assignment`, submission read model.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T018 [P] [US2] API test F041-AC-12: `listMyAssignments` chỉ bài có học sinh trong snapshot, có `attempt_id`, lượt còn — file: `modules/learning/tests/integration/assignment/my_assignments_test.go` — phải FAIL trước khi code
- [ ] T019 [P] [US2] Integration test F041-AC-13, F041-AC-14: `ASSIGNMENT_NOT_OPEN`, `ASSIGNMENT_MAX_ATTEMPTS`, ngoài đối tượng `404`; version ghim vs `follow_latest` — file: `modules/learning/tests/integration/assignment/assignment_attempt_guard_test.go`
- [ ] T020 [P] [US2] Integration test F041-AC-15: áp event attempt/exam/re-evaluate một lần, `score_policy`, `late`, `rapid_guess_count`, `assignment.completed` — file: `modules/learning/tests/integration/assignment/submission_projection_test.go`
- [ ] T021 [P] [US2] Integration test F041-AC-16: học sinh mới vào lớp, chuyển lớp, bị khóa → `excused` — file: `modules/learning/tests/integration/assignment/enrollment_change_test.go`
- [ ] T022 [P] [US2] Contract test F041-AC-17: payload `assignment.published` khớp `assignment.created.v1` (consumer module game, thông báo) — file: `modules/learning/tests/contract/assignment/assignment_published_payload_test.go`

### Implementation cho User Story 2

- [ ] T023 [US2] `AssignmentGuard` gọi trong start attempt `mode = assignment` (cửa sổ, lượt, đối tượng, version) — file: `modules/learning/internal/application/command/assignment/assignment_guard.go`
- [ ] T024 [US2] Command `ApplyAttemptEvent`, `SyncTargetsOnEnrollment` + consumer — file: `modules/learning/internal/application/command/assignment/apply_attempt_event.go`, `modules/learning/internal/presentation/consumer/assignment_attempt_consumer.go`, `modules/learning/internal/presentation/consumer/enrollment_assignment_consumer.go`
- [ ] T025 [US2] Query `ListMyAssignments` + handler — file: `modules/learning/internal/application/query/assignment/list_my_assignments.go`, `modules/learning/internal/presentation/http/public/my_assignment_handler.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Đóng bài và công bố kết quả (Priority: P1)

**Goal**: Worker mở/đóng, `missed`, `ResultRelease`.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T026 [P] [US3] Integration test (đồng hồ giả) F041-AC-18: đóng bài, `missed` trừ `excused`, `assignment.closed`; chạy 2 worker không đóng trùng — file: `modules/learning/tests/integration/assignment/close_due_assignments_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US3] API test F041-AC-19, F041-AC-20, F041-AC-21: review ẩn kết quả trước công bố; `after_close` tự công bố; `manual` cần `lms.result.release`, trước đóng `409 CONFLICT`, audit — file: `modules/learning/tests/integration/assignment/result_release_test.go`

### Implementation cho User Story 3

- [ ] T028 [US3] Command `OpenDueAssignments`, `CloseDueAssignments` (`FOR UPDATE SKIP LOCKED`) + job `assignment-scheduler` — file: `modules/learning/internal/application/command/assignment/close_due_assignments.go`, `modules/learning/internal/presentation/job/assignment_scheduler.go`
- [ ] T029 [US3] Command `ReleaseAssignmentResults` + gating review theo `ResultRelease` trong `getActivityAttemptReview` — file: `modules/learning/internal/application/command/assignment/release_assignment_results.go`, `modules/learning/internal/application/query/attempt/get_attempt_review.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Gia hạn, miễn, nhắc từng học sinh (Priority: P2)

**Goal**: Ngoại lệ cá nhân và nhắc có giới hạn.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T030 [P] [US4] API test F041-AC-22, F041-AC-23: gia hạn mở lại học sinh `missed`; miễn loại khỏi tỉ lệ; lý do bắt buộc; audit; replay key — file: `modules/learning/tests/integration/assignment/student_exception_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US4] API test F041-AC-24: nhắc chỉ học sinh chưa nộp, `recipients`, lần vượt `assignment_remind_limit` → `429 RATE_LIMITED` + `Retry-After`, audit — file: `modules/learning/tests/integration/assignment/remind_assignment_test.go`
- [ ] T032 [P] [US4] API test F041-AC-25: `listAssignmentSubmissions` đủ trường, cursor, chỉ trong scope — file: `modules/learning/tests/integration/assignment/list_submissions_test.go`

### Implementation cho User Story 4

- [ ] T033 [US4] Command `ExtendAssignmentForStudent`, `ExcuseAssignmentForStudent`, `RemindAssignment` (outbox `assignment.reminded`) — file: `modules/learning/internal/application/command/assignment/extend_for_student.go`, `excuse_for_student.go`, `remind_assignment.go`
- [ ] T034 [US4] Query `ListAssignmentSubmissions`; handler `extendAssignmentForStudent`, `excuseAssignmentForStudent`, `remindAssignment`, `listAssignmentSubmissions`, `releaseAssignmentResults` — file: `modules/learning/internal/application/query/assignment/list_submissions.go`, `modules/learning/internal/presentation/http/public/assignment_handler.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Phân quyền và cô lập tenant (Priority: P1)

**Goal**: Mọi endpoint F041 đúng vai trò, phạm vi, tenant, idempotent.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T035 [P] [US5] Test tenant isolation + authorization cho mọi endpoint mới (sinh từ OpenAPI) F041-AC-26, F041-AC-27: `ad.truong.b` → 404, `tenant_ids` bỏ qua, `ad.donvi` ghi → 403, `sm.antoan` → 403, học sinh `/lms/*` → `PORTAL_ACCESS_STAFF_ONLY`, audit `authz.denied` — file: `modules/learning/tests/integration/assignment/authz_tenant_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US5] API test F041-AC-28: System Admin ghi thiếu `X-Tenant-Id` → 400; có header → `created_by_role = system_admin`, audit tenant đích — file: `modules/learning/tests/integration/assignment/system_admin_assignment_test.go`
- [ ] T037 [P] [US5] API test F041-AC-29: replay mọi thao tác ghi cùng key không ghi lần hai; khác payload → `409 IDEMPOTENCY_KEY_REUSED` — file: `modules/learning/tests/integration/assignment/assignment_idempotency_test.go`

### Implementation cho User Story 5

- [ ] T038 [US5] Guard phạm vi trường/môn cho command/query assignment, audit qua outbox `audit.recorded` — file: `modules/learning/internal/application/command/assignment/scope_guard.go`

**Checkpoint**: User Story 5 đạt.

## Phase 8: User Story 6 — Giao quest/mini-game theo LO yếu và nhắc trong gợi ý (Priority: P3)

**Goal**: Danh mục game chỉ đọc, rule R1.

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [ ] T039 [P] [US6] Integration test F041-AC-30: projection rỗng → danh sách rỗng; event mẫu → chỉ quest published được gán, lọc `objective_ids`; giao quest chưa gán → 403; không route sửa game trong `/lms/*` — file: `modules/learning/tests/integration/assignment/game_activity_catalog_test.go` — phải FAIL trước khi code
- [ ] T040 [P] [US6] Unit test F041-AC-31: rule R1 (hạn cá nhân < 48 giờ, chưa nộp) ưu tiên 1 — file: `modules/learning/tests/unit/recommendation/r1_assignment_due_test.go`

### Implementation cho User Story 6

- [ ] T041 [US6] Consumer projection `ref.game_activities` + query/handler `listGameActivities` — file: `modules/learning/internal/presentation/consumer/game_activity_projection_consumer.go`, `modules/learning/internal/application/query/assignment/list_game_activities.go`, `modules/learning/internal/presentation/http/public/game_activity_catalog_handler.go`
- [ ] T042 [US6] Rule R1 và đăng ký vào `RuleRegistry` — file: `modules/learning/internal/domain/recommendation/r1_assignment_due.go`, `modules/learning/cmd/worker/main.go`

**Checkpoint**: User Story 6 chạy độc lập.

## Phase 9: Polish & Cross-Cutting

- [ ] T043 [P] Metric/span theo plan.md §Quan sát; kiểm log không có tên học sinh — file: `modules/learning/internal/infrastructure/observability/assignment_metrics.go`
- [ ] T044 [P] Đồng bộ mã lỗi AM ngoài scope giữa TEST_STRATEGY TC-TEN-002 và API_DESIGN §4.1 (đề xuất sửa tài liệu) + CHANGELOG + trạng thái F041 trong CODE_PLAN — file: `docs/12-project/TEST_STRATEGY.md`, `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T045 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/041-school-assignments/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T003–T007) → US1 → US2 → US3 → US4, US5 (song song) → US6 → Polish.
- T014 cần F040 (`StudyGroupMemberReader`); khi chưa có, đối tượng `study_group` trả `422` theo Assumptions của spec. F041 không phụ thuộc F042: graded + blueprint trả `422` (F041-AC-09) cho tới khi F042 T052 thay adapter (thứ tự F040 → F041 → F042, CODE_PLAN §4.1).
- T041 nhận dữ liệu thật khi F078 phát event; T042 cần interface `Rule` của F032.

## Definition of Done

- [ ] Mọi tiêu chí F041-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
