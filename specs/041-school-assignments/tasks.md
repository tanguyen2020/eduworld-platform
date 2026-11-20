# Tasks: F041 — Giao bài của nhà trường: assignment, `ResultRelease`, gia hạn/miễn, `my-assignments`

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `assignment` (domain, application, persistence, handler, job) và đăng ký route/consumer/job — file: `src/backend/modules/learning/cmd/api/main.go`, `src/backend/modules/learning/cmd/worker/main.go` — bằng chứng: package `assignment` theo cấu trúc phẳng của module learning (domain `internal/domain/assignment`, command `school_assignments*.go`, query `school_assignments.go`, repo `school_assignment_repo.go`, read model `read/assignment.go`, handler `http/school_assignments.go`, consumer `consumer/school_assignments.go`, job `job/assignment_scheduler.go`), nối ở `module.go` (route, consumer `learning.assignment_submissions`/`learning.assignment_enrollment`, job `assignment-scheduler`); `cmd/internal/wiring` PASS
- [x] T002 [P] Đọc tham số `graded_assignment_release_default`, `late_window_hours`, `assignment_remind_limit`, `score_scale` từ cấu hình platform/tenant — file: `src/backend/modules/learning/internal/application/port/lms_config.go`, `src/backend/modules/learning/internal/infrastructure/persistence/lms_config_reader.go` — bằng chứng: `command.AssignmentConfig`/`DefaultAssignmentConfig()` theo CONFIG_DEFAULTS [Confirmed — đọc settings khi module learning có, như `attempt_start_rate`]

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Delta contract OpenAPI: `AssignmentSubmission.attempts_used`, query `objective_ids` của `listGameActivities` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — bằng chứng: `AssignmentSubmission.attempts_used` đã có; `listGameActivities` thêm `objective_ids` (array ≤ 20); `MyAssignment.subject_name` (T046); split + portal `gen:api`/`gen:check`, fixture Godot `gen_fixtures`
- [x] T004 [P] Payload schema `assignment.results_released.v1`, `assignment.completed.v1`, `assignment.closed.v1`, `assignment.submission_updated.v1`, `assignment.reminded.v1`; bổ sung 3 event mới vào EVENT_DESIGN §4.3 — file: `specs/events/payloads/learning/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint` — bằng chứng: schema `assignment.results_released.v1`, `assignment.completed.v1`, `assignment.closed.v1`, `assignment.submission_updated.v1`, `assignment.reminded.v1` (có `examples`, `validate-schemas` 0 lỗi); EVENT_DESIGN §4.3 cập nhật trường
- [x] T005 [P] Xác nhận payload tiêu thụ cho `ref.game_activities` đã có: `content.published` (game, khối `catalog` — owner F071, F078 bổ sung), `content.rolled_back`/`content.unpublished` (game, owner F078), `tenant_data.assigned`/`tenant_data.unassigned` (owner F045); thiếu → chặn T041, không tự tạo — file: `specs/events/payloads/game/`, `specs/events/payloads/identity/` (chỉ đọc) — bằng chứng: thiếu payload catalog của module game (`content.published` khối `catalog`, `content.rolled_back`/`unpublished`) và `tenant_data.assigned/unassigned` cho `quest`/`minigame_config` → không tự tạo; Câu hỏi mở F041-2
- [x] T006 ent schema `lms.assignments` (thêm cột), `lms.assignment_targets`, `lms.assignment_target_students`, `lms.assignment_submissions`, `lms.assignment_student_exceptions`, `lms.assignment_reminders`, `ref.game_activities` (mọi chuỗi `MaxLen`, CHECK, partial unique, RLS) — file: `src/backend/modules/learning/ent/schema/assignment.go`, `assignment_target.go`, `assignment_target_student.go`, `assignment_submission.go`, `assignment_student_exception.go`, `assignment_reminder.go`, `ref_game_activity.go` — kiểm: `make migrate-diff MOD=learning name=f041_assignments`, `make migrate-lint MOD=learning` — bằng chứng: migration viết tay `20261216000001_f041_assignments.sql` (lms.assignments, assignment_targets, assignment_target_students, assignment_submissions, assignment_student_exceptions, assignment_reminders TA, ref.game_activities; CHECK, partial unique, RLS, view `_live`, REVOKE DELETE/TRUNCATE) [Confirmed: như F040, không ent]; `atlas.sum` cập nhật
- [x] T007 [P] Cập nhật DATABASE_DESIGN §2.14/§2.16 (cột assignment theo spec, bỏ `result_visibility`, bảng mới, `target_type` class/study_group/students) — file: `docs/07-backend/DATABASE_DESIGN.md` — bằng chứng: DATABASE_DESIGN §2.14 thêm bảng F041 (`result_release` thay `result_visibility`)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Tạo và giao bài (Priority: P1) 🎯 MVP

**Goal**: Tạo, sửa, publish, hủy bài giao.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T008 [P] [US1] Unit test bảng chuyển trạng thái và trường sửa được F041-AC-10, F041-AC-11; `ReleasePolicy` F041-AC-06 — file: `src/backend/modules/learning/tests/unit/assignment/assignment_state_test.go` — phải FAIL trước khi code — bằng chứng: `internal/domain/assignment/assignment_test.go` (TestReleasePolicy, TestValidateLimits, TestLifeCycle, TestWindowAndScore) PASS
- [x] T009 [P] [US1] API test F041-AC-01, F041-AC-02, F041-AC-03: tạo draft + `assignment.created`; `CONTENT_NOT_PUBLISHED`, `RESOURCE_NOT_ASSIGNED`, `ACTIVITY_TYPE_NOT_SUPPORTED` — file: `src/backend/modules/learning/tests/integration/assignment/create_assignment_test.go` — bằng chứng: `TestAssignmentCreate` PASS + unit `TestAssignmentContentActivityType` (AC-03: loại ngoài Release không lưu được — CHECK `activity_versions_type_ck`)
- [x] T010 [P] [US1] API test F041-AC-04, F041-AC-05: AM ngoài môn `403 PERMISSION_DENIED` + audit; đối tượng trường/tenant khác `404` — file: `src/backend/modules/learning/tests/integration/assignment/assignment_scope_test.go` — bằng chứng: `TestAssignmentScope` PASS (AM môn khác 403 + `authz.denied`, đối tượng tenant khác 404)
- [x] T011 [P] [US1] API test F041-AC-07: validation độ dài/khoảng → `422 VALIDATION_FAILED` — file: `src/backend/modules/learning/tests/integration/assignment/assignment_validation_test.go` — bằng chứng: `TestAssignmentCreate` PASS (title > 120, instructions > 2000, max_attempts 11, late 169, due ≤ open)
- [x] T012 [P] [US1] Integration test F041-AC-08, F041-AC-09: publish snapshot đối tượng, `scheduled`/`open`, `assignment.published`, audit; tạo/sửa bài graded + blueprint khi chỉ có adapter mặc định → `422 VALIDATION_FAILED`, không ghi DB (nửa "publish → ExamSession" chuyển sang F042 T051) — file: `src/backend/modules/learning/tests/integration/assignment/publish_assignment_test.go` — bằng chứng: `TestAssignmentPublishUpdateCancel` (snapshot 2 học sinh, `open`/`scheduled`, `assignment.published`, audit) + `TestAssignmentCreate` (graded + blueprint → 422 `/content_ref`, không ghi) PASS

### Implementation cho User Story 1

- [x] T013 [P] [US1] Domain `assignment.Assignment`, `ReleasePolicy`, `Window`, `ScorePolicy` — file: `src/backend/modules/learning/internal/domain/assignment/assignment.go`, `release_policy.go`, `window.go`, `score_policy.go` — bằng chứng: `domain/assignment/assignment.go` (Fields/Validate/Defaults, Editable/CanEdit, PublishStatus, CanCancel/CanArchive, AfterClose, ResultsVisible, Window, Pick)
- [x] T014 [US1] Port `TargetResolver` (lớp từ `ref.enrollments`, học sinh, `StudyGroupMemberReader` của F040) và port `ExamSessionCreator` (F041 sở hữu: `Available`, `Create`, `Cancel`) + adapter mặc định `unavailableExamSessionCreator` đăng ký ở `module.go` (adapter gọi `CreateExamSession`/`CancelExamSession` thuộc F042 T052) — file: `src/backend/modules/learning/internal/application/port/assignment_ports.go`, `src/backend/modules/learning/internal/infrastructure/examsession/unavailable_creator.go` — bằng chứng: port `write.SchoolAssignmentStore` (lớp từ `ref.enrollments`, nhóm qua `LMSRepo.Group`, học sinh active) và `port.ExamSessionCreator` (`Available`/`Create`/`Cancel`) + adapter `infrastructure/examsession.Unavailable` nối ở `module.go`
- [x] T015 [US1] Command `CreateAssignment`, `UpdateAssignment`, `PublishAssignment`, `CancelAssignment`, `ArchiveAssignment` (outbox + audit) — file: `src/backend/modules/learning/internal/application/command/assignment/create_assignment.go`, `update_assignment.go`, `publish_assignment.go`, `cancel_assignment.go`, `archive_assignment.go` — bằng chứng: `command.Assignments`: CreateAssignment, UpdateAssignment, PublishAssignment, CancelAssignment, ArchiveAssignment (outbox + audit, `authz.denied` ghi ngoài transaction)
- [x] T016 [US1] Repository ent + query `ListAssignments`, `GetAssignment` có scope — file: `src/backend/modules/learning/internal/infrastructure/persistence/assignment/assignment_repo.go`, `src/backend/modules/learning/internal/application/query/assignment/list_assignments.go` — bằng chứng: repo `SchoolAssignmentRepo`, read model `SchoolAssignmentModel`, query `SchoolAssignments.Assignments/Assignment` (lọc theo cặp trường × môn của grant)
- [x] T017 [US1] Handler `listAssignments`, `createAssignment`, `getAssignment`, `updateAssignment`, `publishAssignment`, `cancelAssignment`, `archiveAssignment` — file: `src/backend/modules/learning/internal/presentation/http/public/assignment_handler.go` — bằng chứng: handler `listAssignments`, `createAssignment`, `getAssignment`, `updateAssignment` (merge patch), `publishAssignment`, `cancelAssignment`, `archiveAssignment`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Học sinh nhận và làm bài trong game (Priority: P1)

**Goal**: `listMyAssignments`, guard attempt `assignment`, submission read model.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T018 [P] [US2] API test F041-AC-12: `listMyAssignments` chỉ bài có học sinh trong snapshot, có `attempt_id`, lượt còn — file: `src/backend/modules/learning/tests/integration/assignment/my_assignments_test.go` — phải FAIL trước khi code — bằng chứng: `TestAssignmentStudentFlow` PASS (chỉ học sinh trong snapshot, `attempt_id` đang dở, `attempts_left`, `subject_name`)
- [x] T019 [P] [US2] Integration test F041-AC-13, F041-AC-14: `ASSIGNMENT_NOT_OPEN`, `ASSIGNMENT_MAX_ATTEMPTS`, ngoài đối tượng `404`; version ghim vs `follow_latest` — file: `src/backend/modules/learning/tests/integration/assignment/assignment_attempt_guard_test.go` — bằng chứng: `TestAssignmentStudentFlow` (404 ngoài đối tượng, 403 `ASSIGNMENT_NOT_OPEN`, 409 `ASSIGNMENT_MAX_ATTEMPTS`) + `TestAssignmentVersionPinning` (ghim v1 vs `follow_latest` v2, lượt đang làm giữ v1) PASS
- [x] T020 [P] [US2] Integration test F041-AC-15: áp event attempt/exam/re-evaluate một lần, `score_policy`, `late`, `rapid_guess_count`, `assignment.completed` — file: `src/backend/modules/learning/tests/integration/assignment/submission_projection_test.go` — bằng chứng: `TestAssignmentStudentFlow` (mỗi event áp hai lần, `assignment.completed` một lần), `TestAssignmentStudentExceptions` (`late`), `TestAssignmentReEvaluated` (`attempt.re_evaluated`) PASS
- [x] T021 [P] [US2] Integration test F041-AC-16: học sinh mới vào lớp, chuyển lớp, bị khóa → `excused` — file: `src/backend/modules/learning/tests/integration/assignment/enrollment_change_test.go` — bằng chứng: `TestAssignmentEnrollmentChanges` PASS (học sinh mới vào lớp, khóa tài khoản → `excused`)
- [x] T022 [P] [US2] Contract test F041-AC-17: payload `assignment.published` khớp `assignment.created.v1` (consumer module game, thông báo) — file: `src/backend/modules/learning/tests/contract/assignment/assignment_published_payload_test.go` — bằng chứng: `TestAssignmentStudentFlow` kiểm `assignment.published` theo schema `assignment.created.v1` (jsonschema) PASS

### Implementation cho User Story 2

- [x] T023 [US2] `AssignmentGuard` gọi trong start attempt `mode = assignment` (cửa sổ, lượt, đối tượng, version) — file: `src/backend/modules/learning/internal/application/command/assignment/assignment_guard.go` — bằng chứng: `Assignments.Admit` (gate) gọi trong `startActivityAttempt` mode `assignment` (`attempt_assignment.go`); `feedback_timing` của bài; `activity.submitted/graded` có `assignment_id` + `result_release` của bài
- [x] T024 [US2] Command `ApplyAttemptEvent`, `SyncTargetsOnEnrollment` + consumer — file: `src/backend/modules/learning/internal/application/command/assignment/apply_attempt_event.go`, `src/backend/modules/learning/internal/presentation/consumer/assignment_attempt_consumer.go`, `src/backend/modules/learning/internal/presentation/consumer/enrollment_assignment_consumer.go` — bằng chứng: `ApplyAttemptEvent` (tính lại từ attempt + evaluation hiệu lực), `SyncEnrollment`, `AccessRevoked` + consumer `AssignmentSubmissions`, `AssignmentEnrollment`
- [x] T025 [US2] Query `ListMyAssignments` + handler — file: `src/backend/modules/learning/internal/application/query/assignment/list_my_assignments.go`, `src/backend/modules/learning/internal/presentation/http/public/my_assignment_handler.go` — bằng chứng: query `MyAssignments` + handler `listMyAssignments` (guard `game_player`)

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Đóng bài và công bố kết quả (Priority: P1)

**Goal**: Worker mở/đóng, `missed`, `ResultRelease`.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T026 [P] [US3] Integration test (đồng hồ giả) F041-AC-18: đóng bài, `missed` trừ `excused`, `assignment.closed`; chạy 2 worker không đóng trùng — file: `src/backend/modules/learning/tests/integration/assignment/close_due_assignments_test.go` — phải FAIL trước khi code — bằng chứng: `TestAssignmentCloseAndRelease` PASS (đồng hồ giả, `missed` trừ `excused`, `assignment.closed`, chạy lần 2 không đóng lại)
- [x] T027 [P] [US3] API test F041-AC-19, F041-AC-20, F041-AC-21: review ẩn kết quả trước công bố; `after_close` tự công bố; `manual` cần `lms.result.release`, trước đóng `409 CONFLICT`, audit — file: `src/backend/modules/learning/tests/integration/assignment/result_release_test.go` — bằng chứng: `TestAssignmentCloseAndRelease` PASS (review/attempt/list không có kết quả trước công bố; `after_close` tự công bố; `manual` cần `lms.result.release`, 409 trước khi đóng, audit)

### Implementation cho User Story 3

- [x] T028 [US3] Command `OpenDueAssignments`, `CloseDueAssignments` (`FOR UPDATE SKIP LOCKED`) + job `assignment-scheduler` — file: `src/backend/modules/learning/internal/application/command/assignment/close_due_assignments.go`, `src/backend/modules/learning/internal/presentation/job/assignment_scheduler.go` — bằng chứng: `RunScheduler` (khóa dòng `FOR UPDATE`, scope từng tenant, `tenancy.Bypass(platform_job)` khi liệt kê) + job `assignment-scheduler` mỗi phút
- [x] T029 [US3] Command `ReleaseAssignmentResults` + gating review theo `ResultRelease` trong `getActivityAttemptReview` — file: `src/backend/modules/learning/internal/application/command/assignment/release_assignment_results.go`, `src/backend/modules/learning/internal/application/query/attempt/get_attempt_review.go` — bằng chứng: `ReleaseAssignmentResults` + gating ở `query.Attempts` (`Review`, `Get`, `ListMine`) và `Submit` (không trả evaluation khi chưa công bố)

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Gia hạn, miễn, nhắc từng học sinh (Priority: P2)

**Goal**: Ngoại lệ cá nhân và nhắc có giới hạn.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T030 [P] [US4] API test F041-AC-22, F041-AC-23: gia hạn mở lại học sinh `missed`; miễn loại khỏi tỉ lệ; lý do bắt buộc; audit; replay key — file: `src/backend/modules/learning/tests/integration/assignment/student_exception_test.go` — phải FAIL trước khi code — bằng chứng: `TestAssignmentStudentExceptions` PASS (gia hạn mở lại `missed`, sớm hơn hạn → 422, miễn có lý do, audit, replay key, key dùng lại → `IDEMPOTENCY_KEY_REUSED`)
- [x] T031 [P] [US4] API test F041-AC-24: nhắc chỉ học sinh chưa nộp, `recipients`, lần vượt `assignment_remind_limit` → `429 RATE_LIMITED` + `Retry-After`, audit — file: `src/backend/modules/learning/tests/integration/assignment/remind_assignment_test.go` — bằng chứng: `TestAssignmentStudentExceptions` PASS (`recipients` = 2, lần 2 cùng ngày → 429 + `Retry-After`, `assignment.reminded`)
- [x] T032 [P] [US4] API test F041-AC-25: `listAssignmentSubmissions` đủ trường, cursor, chỉ trong scope — file: `src/backend/modules/learning/tests/integration/assignment/list_submissions_test.go` — bằng chứng: `TestAssignmentStudentExceptions` (cursor, lọc `status`) + `TestAssignmentStudentFlow` (tenant B 404) PASS

### Implementation cho User Story 4

- [x] T033 [US4] Command `ExtendAssignmentForStudent`, `ExcuseAssignmentForStudent`, `RemindAssignment` (outbox `assignment.reminded`) — file: `src/backend/modules/learning/internal/application/command/assignment/extend_for_student.go`, `excuse_for_student.go`, `remind_assignment.go` — bằng chứng: `ExtendAssignmentForStudent`, `ExcuseAssignmentForStudent`, `RemindAssignment` (outbox `assignment.reminded`, `RetryAfter` tới 00:00 ICT)
- [x] T034 [US4] Query `ListAssignmentSubmissions`; handler `extendAssignmentForStudent`, `excuseAssignmentForStudent`, `remindAssignment`, `listAssignmentSubmissions`, `releaseAssignmentResults` — file: `src/backend/modules/learning/internal/application/query/assignment/list_submissions.go`, `src/backend/modules/learning/internal/presentation/http/public/assignment_handler.go` — bằng chứng: query `Submissions`; handler `extendAssignmentForStudent`, `excuseAssignmentForStudent`, `remindAssignment`, `listAssignmentSubmissions`, `releaseAssignmentResults`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Phân quyền và cô lập tenant (Priority: P1)

**Goal**: Mọi endpoint F041 đúng vai trò, phạm vi, tenant, idempotent.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T035 [P] [US5] Test tenant isolation + authorization cho mọi endpoint mới (sinh từ OpenAPI) F041-AC-26, F041-AC-27: `ad.truong.b` → 404, `tenant_ids` bỏ qua, `ad.donvi` ghi → 403, `sm.antoan` → 403, học sinh `/lms/*` → `PORTAL_ACCESS_STAFF_ONLY`, audit `authz.denied` — file: `src/backend/modules/learning/tests/integration/assignment/authz_tenant_test.go` — phải FAIL trước khi code — bằng chứng: `TestAssignmentScope` PASS (`ad.truong.b`/`ad.truong.a2` 404, `tenant_ids` bỏ qua, tenant admin ghi 403, `safety_moderator` 403, học sinh `/lms` 403, `authz.denied`); `authztest.Cover` cho mọi operation F041 (matrix `cross_tenant`, `role_denied`)
- [x] T036 [P] [US5] API test F041-AC-28: System Admin ghi thiếu `X-Tenant-Id` → 400; có header → `created_by_role = system_admin`, audit tenant đích — file: `src/backend/modules/learning/tests/integration/assignment/system_admin_assignment_test.go` — bằng chứng: `TestAssignmentScope` PASS (System Admin thiếu `X-Tenant-Id` → 400, có header → `created_by_role = system_admin`)
- [x] T037 [P] [US5] API test F041-AC-29: replay mọi thao tác ghi cùng key không ghi lần hai; khác payload → `409 IDEMPOTENCY_KEY_REUSED` — file: `src/backend/modules/learning/tests/integration/assignment/assignment_idempotency_test.go` — bằng chứng: `TestAssignmentPublishUpdateCancel` (publish replay, một `assignment.published`) + `TestAssignmentStudentExceptions` (extend replay, key khác payload) PASS

### Implementation cho User Story 5

- [x] T038 [US5] Guard phạm vi trường/môn cho command/query assignment, audit qua outbox `audit.recorded` — file: `src/backend/modules/learning/internal/application/command/assignment/scope_guard.go` — bằng chứng: `AuthorizeLMS` (trường 404, môn 403) ở mọi command; query kiểm `lms.assignment.read` theo trường × môn; `deniedOutside` ghi `authz.denied` ngoài transaction

**Checkpoint**: User Story 5 đạt.

## Phase 8: User Story 6 — Giao quest/mini-game theo LO yếu và nhắc trong gợi ý (Priority: P3)

**Goal**: Danh mục game chỉ đọc, rule R1.

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [x] T039 [P] [US6] Integration test F041-AC-30: projection rỗng → danh sách rỗng; event mẫu → chỉ quest published được gán, lọc `objective_ids`; giao quest chưa gán → 403; không route sửa game trong `/lms/*` — file: `src/backend/modules/learning/tests/integration/assignment/game_activity_catalog_test.go` — phải FAIL trước khi code — bằng chứng: `TestAssignmentGameCatalog` PASS (projection rỗng → rỗng, lọc `objective_ids`, mục chưa publish → 403, `/lms/*` không có route ghi game); consumer projection chờ F041-2
- [x] T040 [P] [US6] Unit test F041-AC-31: rule R1 (hạn cá nhân < 48 giờ, chưa nộp) ưu tiên 1 — file: `src/backend/modules/learning/tests/unit/recommendation/r1_assignment_due_test.go` — bằng chứng: `domain/recommendation/r1_test.go` (TestR1AssignmentDue) PASS

### Implementation cho User Story 6

- [x] T041 [US6] Consumer projection `ref.game_activities` + query/handler `listGameActivities` — file: `src/backend/modules/learning/internal/presentation/consumer/game_activity_projection_consumer.go`, `src/backend/modules/learning/internal/application/query/assignment/list_game_activities.go`, `src/backend/modules/learning/internal/presentation/http/public/game_activity_catalog_handler.go` — bằng chứng: query/handler `listGameActivities` trên `ref.game_activities`; consumer projection chưa làm (thiếu schema event, Câu hỏi mở F041-2)
- [x] T042 [US6] Rule R1 và đăng ký vào `RuleRegistry` — file: `src/backend/modules/learning/internal/domain/recommendation/r1_assignment_due.go`, `src/backend/modules/learning/cmd/worker/main.go` — bằng chứng: rule `recommendation.R1` (hạn cá nhân < 48 giờ, ưu tiên 1) + SnapshotSource `command.AssignmentsDue`, đăng ký ở `module.go`

**Checkpoint**: User Story 6 chạy độc lập.

## Phase 9: Polish & Cross-Cutting

- [x] T043 [P] Metric/span theo plan.md §Quan sát; kiểm log không có tên học sinh — file: `src/backend/modules/learning/internal/infrastructure/observability/assignment_metrics.go` — bằng chứng: metric `eduworld_lms_assignments_total{status}`, `eduworld_lms_submissions_updated_total{status}`, `eduworld_lms_assignment_reminders_total{result}`; span qua `observability.UseCase`; log không có tên học sinh
- [x] T044 [P] Đồng bộ mã lỗi AM ngoài scope giữa TEST_STRATEGY TC-TEN-002 và API_DESIGN §4.1 (đề xuất sửa tài liệu) + CHANGELOG + trạng thái F041 trong CODE_PLAN — file: `docs/12-project/TEST_STRATEGY.md`, `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — bằng chứng: CHANGELOG, CODE_PLAN; mã lỗi AM ngoài scope = `403 PERMISSION_DENIED` khớp API_DESIGN §4.1 (TEST_STRATEGY không đổi)
- [x] T045 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/041-school-assignments/quickstart.md` — bằng chứng: quickstart §5 ghi bằng chứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [x] T046 **OQ chốt 2026-11-20** (OQ F062-1) `MyAssignment.subject_name` (contract + read model) — file: `specs/openapi/modules/learning.v1.yaml`, `src/backend/modules/learning/internal/infrastructure/persistence/lms/` — kiểm: `make specs-breaking`, integration — cỡ S — bằng chứng: `MyAssignment.subject_name` (`LearningName` từ `curriculum.subjects.name`) trong contract + read model; `TestAssignmentStudentFlow` PASS
- [x] T048 **OQ chốt 2026-11-20** (OQ F041-1) Event `assignment.cancelled` (payload schema, EVENT_DESIGN §4.3, outbox trong `CancelAssignment`); consumer `learning.lms_schedule` (F040) soft-delete mục `assignment_due` của bài, phát `schedule_item.deleted`, idempotent — file: `specs/events/payloads/learning/assignment.cancelled.v1.schema.json`, `src/backend/modules/learning/internal/application/command/school_assignments.go`, `src/backend/modules/learning/internal/application/command/lms.go`, `src/backend/modules/learning/internal/presentation/consumer/lms.go` — kiểm: integration — cỡ S — bằng chứng: `TestAssignmentCancelRemovesSchedule` PASS (payload khớp schema, mục lịch soft-delete `delete_reason = cancelled`, một `schedule_item.deleted` khi giao hai lần)
- [ ] T049 **OQ chốt 2026-11-20** (OQ F041-2, phụ thuộc) Consumer projection `ref.game_activities` — làm khi F071/F078 định nghĩa payload `content.published` khối `catalog`, `content.rolled_back`/`content.unpublished` (F078 T006, T034) và `tenant_data.assigned/unassigned` (F045); đến lúc đó `listGameActivities` trả rỗng — file: `src/backend/modules/learning/internal/presentation/consumer/` — kiểm: integration — cỡ S
- [x] T047 **OQ chốt 2026-11-20** (OQ F062-2) `startAdHocAttempt` nhận `mode = assignment` cho bài `question_set` với kiểm tra F041-AC-13; test cửa sổ/số lần/không thuộc lớp — file: `src/backend/modules/learning/internal/application/command/attempt_commands.go` — kiểm: `make test-integration MOD=learning` — cỡ S — bằng chứng: `startAdHocAttempt` mode `assignment` + `source.kind = question_set` + `context_type = assignment` (`startAssignmentSet`, version ghim); `TestAssignmentQuestionSet` PASS (ngoài đối tượng 404, thiếu context 422, hết lượt 409, quá hạn 403)

## Dependencies & Execution Order

- Setup → Foundational (T003–T007) → US1 → US2 → US3 → US4, US5 (song song) → US6 → Polish.
- T014 cần F040 (`StudyGroupMemberReader`); khi chưa có, đối tượng `study_group` trả `422` theo Assumptions của spec. F041 không phụ thuộc F042: graded + blueprint trả `422` (F041-AC-09) cho tới khi F042 T052 thay adapter (thứ tự F040 → F041 → F042, CODE_PLAN §4.1).
- T041 nhận dữ liệu thật khi F078 phát event (T049 chờ F078 T006/T034, OQ F041-2); T042 cần interface `Rule` của F032.

## Definition of Done

- [x] Mọi tiêu chí F041-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
