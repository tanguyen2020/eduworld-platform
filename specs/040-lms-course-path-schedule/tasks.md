# Tasks: F040 — Course, learning path, lịch, nhóm học tập

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package module `lms` (domain, application, persistence, handler) và đăng ký route/consumer trong composition root — file: `src/backend/modules/learning/cmd/api/main.go`, `src/backend/modules/learning/cmd/worker/main.go` — ✅ nối trong `src/backend/modules/learning/module.go` (command.LMS, query.LMS, consumer `learning.lms_courses`, `learning.lms_schedule`, R6); `go test ./cmd/internal/wiring/` ok
- [x] T002 [P] Đăng ký permission dùng trong module (`lms.class.read`, `lms.course.write`, `lms.learning_path.write`, `lms.schedule.write`, `lms.study_group.write`, `school.settings.write`) vào kiểm tra catalog lúc khởi động — file: `src/backend/modules/learning/internal/application/port/permissions.go` — ✅ permission đã có trong `src/backend/pkg/authz/catalog.go` (kiểm lúc khởi động); use case gọi `command.AuthorizeLMS`/`authz.Require` tường minh

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Delta contract: thêm `getSchoolCurriculumSettings`, `putSchoolCurriculumSettings`, `listMyLearningPaths`, `listMySchedule`; `LearningPath.is_template`, `textbook_series_id`; `Course.curriculum_version_id` required — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — ✅ merged spec → `split-public.mjs`; 4 operation mới, `LearningPath.textbook_series_id/version`, `Course.curriculum_version_id` required; menu catalog (settings view/edit_school); portal `gen:api`
- [x] T004 [P] Payload schema `<entity>.deleted.v1` cho entity LMS — file: `specs/events/payloads/learning/schedule_item.deleted.v1.schema.json` — kiểm: `make specs-lint` — ✅ `specs/events/payloads/learning/schedule_item.deleted.v1.schema.json` (validate-schemas 0 lỗi)
- [x] T005 ent schema `lms.courses`, `lms.school_curriculum_settings`, `lms.learning_paths`, `lms.path_nodes`, `lms.path_node_unlocks`, `lms.schedule_items` (mọi chuỗi `MaxLen`, partial unique) — file: `src/backend/modules/learning/ent/schema/course.go`, `school_curriculum_setting.go`, `learning_path.go`, `path_node.go`, `path_node_unlock.go`, `schedule_item.go` — kiểm: `make migrate-diff MOD=learning name=f040_lms_core`, `make migrate-lint MOD=learning` — ✅ [Confirmed] bảng viết tay, repository SQL (`persistence/write/lms_repo.go`, `read/lms.go`) — migration `20261206000001_f040_lms.sql`
- [x] T006 RLS + policy cho bảng mới, loại H cho `learning_paths` (platform + gán) trong migration — file: `src/backend/modules/learning/migrations/<ts>_f040_lms_core.sql` — ✅ RLS T + hybrid_read cho `learning_paths`/`path_nodes`, view `_live`, REVOKE DELETE/TRUNCATE trong cùng migration; tenancyguard pass
- [x] T007 [P] Cập nhật DATABASE_DESIGN §2.14/§2.16 (cột course, learning_paths, bảng mới; `lms.study_groups`/`lms.study_group_members` đã ghi theo ADR-0084 — chỉ đối chiếu) — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ `docs/07-backend/DATABASE_DESIGN.md` §2.14, §2.16

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Course tự tạo theo lớp × môn × năm học (Priority: P1) 🎯 MVP

**Goal**: Course tự tạo, cài đặt curriculum, ẩn/hiện course.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T008 [P] [US1] Integration test F040-AC-01, F040-AC-02: phát `class.synced` hai lần → một course/môn, đúng `curriculum_version_id` (có/không cài đặt) — file: `src/backend/modules/learning/tests/integration/lms/course_provisioning_test.go` — phải FAIL trước khi code — ✅ TestLMSCoursesAndSettings (giao lại `class.synced` → 1 course; version mới nhất khi chưa cài đặt; lớp đóng → archived)
- [x] T009 [P] [US1] API test F040-AC-03: version chưa publish `409`, chưa gán `403 RESOURCE_NOT_ASSIGNED`, audit, chỉ áp course mới — file: `src/backend/modules/learning/tests/integration/lms/school_curriculum_settings_test.go` — ✅ TestLMSCoursesAndSettings (chưa gán 403 RESOURCE_NOT_ASSIGNED, chưa publish 409, 428/412, audit `school_curriculum.set`, chỉ course tạo sau)
- [x] T010 [P] [US1] API test F040-AC-04, F040-AC-05: phạm vi `listCourses` theo SA/AM/TA; `updateCourse` `428`/`412`/thành công + audit — file: `src/backend/modules/learning/tests/integration/lms/course_api_test.go` — ✅ TestLMSCoursesAndSettings (SA trường, trường khác, tenant B, AM môn khác, TA đọc toàn tenant; 428/412/200 + audit `course.updated`)

### Implementation cho User Story 1

- [x] T011 [P] [US1] Domain `course.Course`, `course.Provisioning` (lớp × môn × năm học) — file: `src/backend/modules/learning/internal/domain/course/course.go`, `src/backend/modules/learning/internal/domain/course/provisioning.go` — ✅ `internal/domain/lms/lms.go` (`SetCourseStatus`, `AcademicYearID`); TestCourseAndPathRules
- [x] T012 [US1] Command `EnsureCoursesForClass`, `UpdateCourse`, `SetSchoolCurriculum` — file: `src/backend/modules/learning/internal/application/command/lms/ensure_courses_for_class.go`, `update_course.go`, `set_school_curriculum.go` — ✅ `command/lms.go` (`EnsureCoursesForClass`, `UpdateCourse`, `PutCurriculumSettings`)
- [x] T013 [US1] Query `ListCourses`, `GetSchoolCurriculum` + repository có scope — file: `src/backend/modules/learning/internal/application/query/lms/list_courses.go`, `src/backend/modules/learning/internal/infrastructure/persistence/lms/course_repo.go` — ✅ `query/lms.go` (`Courses`, `CurriculumSettings`) + `persistence/read/lms.go`
- [x] T014 [US1] Consumer `class.synced`, `class.deleted` → course; handler `listCourses`, `updateCourse`, `getSchoolCurriculumSettings`, `putSchoolCurriculumSettings` — file: `src/backend/modules/learning/internal/presentation/consumer/class_synced_course_consumer.go`, `src/backend/modules/learning/internal/presentation/http/public/lms_course_handler.go` — ✅ consumer `presentation/consumer/lms.go` (`LMSCourses`: class.synced, `status = closed` → archived); handler `presentation/http/lms.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Learning path cho học sinh tự học (Priority: P1)

**Goal**: CRUD lộ trình, khóa/mở node, mở khóa thủ công, R6.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T015 [P] [US2] API test F040-AC-06, F040-AC-07: tạo/sửa path ghim version; nội dung chưa publish `409`, chưa gán `403` — file: `src/backend/modules/learning/tests/integration/lms/learning_path_api_test.go` — phải FAIL trước khi code — ✅ TestLMSLearningPaths (nháp 409, platform chưa gán 403, tạo + replay key, path thứ hai 409)
- [x] T016 [P] [US2] Unit test F040-AC-09: `UnlockPolicy` cho `sequential`/`free`/`mastery_gate` và mở khóa thủ công — file: `src/backend/modules/learning/tests/unit/learningpath/unlock_policy_test.go` — ✅ `domain/lms` TestNodeStates
- [x] T017 [P] [US2] Integration test F040-AC-08, F040-AC-10, F040-AC-11: lộ trình mẫu; unlock thiếu lý do `422`, học sinh ngoài lớp `404`, audit; `listMyLearningPaths` bỏ course hidden — file: `src/backend/modules/learning/tests/integration/lms/my_learning_paths_test.go` — ✅ TestLMSLearningPaths (mẫu theo môn + khối, mở khóa thiếu lý do 422, học sinh ngoài lớp 404, audit, course hidden bị bỏ)
- [x] T018 [P] [US2] Unit test F040-AC-12: rule R6 tạo gợi ý ưu tiên 4 — file: `src/backend/modules/learning/tests/unit/recommendation/r6_next_topic_test.go` — ✅ `domain/recommendation` TestR6

### Implementation cho User Story 2

- [x] T019 [P] [US2] Domain `learningpath.Path`, `Node`, `UnlockPolicy` — file: `src/backend/modules/learning/internal/domain/learningpath/path.go`, `src/backend/modules/learning/internal/domain/learningpath/unlock_policy.go` — ✅ `domain/lms/lms.go` (`ValidatePath`, `NodeStates`)
- [x] T020 [US2] Command `CreateLearningPath`, `UpdateLearningPath`, `UnlockPathNode` (kiểm nội dung published + gán qua `ContentAvailabilityReader`) — file: `src/backend/modules/learning/internal/application/command/lms/create_learning_path.go`, `update_learning_path.go`, `unlock_path_node.go` — ✅ `command/lms.go` (`CreatePath`, `UpdatePath`, `UnlockNode`, `visibleVersion`)
- [x] T021 [US2] Query `ListLearningPaths`, `ListMyLearningPaths` (đọc `MasteryReader`) + handler — file: `src/backend/modules/learning/internal/application/query/lms/list_my_learning_paths.go`, `src/backend/modules/learning/internal/presentation/http/public/lms_path_handler.go`, `src/backend/modules/learning/internal/presentation/http/public/my_lms_handler.go` — ✅ `query/lms.go` (`Paths`, `MyLearningPaths`) + handler
- [x] T022 [US2] Rule R6 và đăng ký vào `RuleRegistry` — file: `src/backend/modules/learning/internal/domain/recommendation/r6_next_topic.go`, `src/backend/modules/learning/cmd/worker/main.go` — ✅ `domain/recommendation/rules.go` R6 + `command.LMSUpcoming` (SnapshotSource) đăng ký trong `module.go`; TestLMSLearningPaths (R6 ưu tiên 4)

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Lịch học, hạn nộp, đề (Priority: P2)

**Goal**: CRUD lịch, lịch từ event, lịch học sinh.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T023 [P] [US3] API test F040-AC-13, F040-AC-14: tạo/sửa trong phạm vi; xóa = soft-delete, audit, `schedule_item.deleted`, biến mất khỏi API đọc — file: `src/backend/modules/learning/tests/integration/lms/schedule_api_test.go` — phải FAIL trước khi code — ✅ TestLMSScheduleAndStudyGroups (tạo/sửa trong phạm vi, trường khác 404, xóa = soft-delete + `schedule_item.deleted` + audit, biến mất)
- [x] T024 [P] [US3] Integration test F040-AC-15: `assignment.published`/`exam.opened` giao 2 lần → một mục lịch — file: `src/backend/modules/learning/tests/integration/lms/schedule_from_event_test.go` — ✅ TestLMSScheduleAndStudyGroups (`assignment.published` giao 2 lần → 1 mục); `exam.opened` cùng consumer (payload F042 [Confirmed])
- [x] T025 [P] [US3] API test F040-AC-16: `listMySchedule` chỉ course active của lớp học sinh, đúng khoảng — file: `src/backend/modules/learning/tests/integration/lms/my_schedule_test.go` — ✅ TestLMSScheduleAndStudyGroups (`listMySchedule` course active của lớp, > 62 ngày 422, tenant B rỗng)

### Implementation cho User Story 3

- [x] T026 [P] [US3] Domain `schedule.Item` — file: `src/backend/modules/learning/internal/domain/schedule/schedule_item.go` — ✅ kind và giới hạn trong `domain/lms/lms.go`
- [x] T027 [US3] Command `CreateScheduleItem`, `UpdateScheduleItem`, `DeleteScheduleItem`, `UpsertScheduleFromEvent` — file: `src/backend/modules/learning/internal/application/command/lms/create_schedule_item.go`, `delete_schedule_item.go`, `upsert_schedule_from_event.go` — ✅ `command/lms.go` (`CreateSchedule`, `UpdateSchedule`, `DeleteSchedule`, `UpsertScheduleFromEvent`)
- [x] T028 [US3] Query `ListScheduleItems`, `ListMySchedule`; handler; consumer `assignment.published`, `exam.opened` — file: `src/backend/modules/learning/internal/presentation/http/public/lms_schedule_handler.go`, `src/backend/modules/learning/internal/presentation/consumer/schedule_from_event_consumer.go` — ✅ `query/lms.go` (`Schedule`, `MySchedule`), handler, consumer `LMSSchedule`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Nhóm học tập (Priority: P3)

**Goal**: CRUD nhóm học tập do module learning sở hữu (ADR-0084), event `study_group.*` qua outbox.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T029 [P] [US4] Unit test domain F040-AC-19, F040-AC-22: `ReplaceMembers` trả đúng diff thêm/bớt, chặn 0 hoặc > 100 thành viên, `name` rỗng/> 64 — file: `src/backend/modules/learning/tests/unit/studygroup/study_group_test.go` — phải FAIL trước khi code — ✅ `domain/lms` TestStudyGroupRules
- [x] T030 [P] [US4] API test F040-AC-17, F040-AC-18, F040-AC-19, F040-AC-20: tạo nhóm theo lớp/khối + audit + replay `Idempotency-Key`; list lọc `class_id`/`grade_level`, get có `member_ids` + `ETag`; update `428`/`412`/thay danh sách; delete = soft-delete nhóm + thành viên, biến mất khỏi API đọc — file: `src/backend/modules/learning/tests/integration/lms/study_group_api_test.go` — phải FAIL trước khi code — ✅ TestLMSScheduleAndStudyGroups (tạo theo lớp + replay key, list lọc lớp, get ETag, update 428/412/thay danh sách, delete soft-delete)
- [x] T031 [P] [US4] API test F040-AC-21, F040-AC-22: trùng tên khác hoa thường `409 CONFLICT`; học sinh trường B, học sinh không thuộc `class_id`, 101 thành viên, tên 65 ký tự → `422 VALIDATION_FAILED` không lộ học sinh trường khác — file: `src/backend/modules/learning/tests/integration/lms/study_group_validation_test.go` — ✅ TestLMSScheduleAndStudyGroups (trùng tên khác hoa thường 409, học sinh tenant B/không tồn tại, tên 65, rỗng, thiếu khối → 422 không lộ id)
- [x] T032 [P] [US4] Integration test F040-AC-23: mỗi thao tác ghi đúng một dòng `infra.outbox_events` cùng transaction (`study_group.created/updated/member_changed/deleted`), payload validate với `specs/events/payloads/learning/study_group.*.v1.schema.json`, không có tên học sinh; rollback → không có event — file: `src/backend/modules/learning/tests/integration/lms/study_group_outbox_test.go` — ✅ TestLMSScheduleAndStudyGroups (`study_group.created/updated/member_changed/deleted` mỗi loại 1, hợp lệ schema, chỉ ID)

### Implementation cho User Story 4

- [x] T033 [US4] ent schema `lms.study_groups`, `lms.study_group_members` (`name` `MaxLen(64)`, partial unique `(tenant_id, school_id, lower(name))`, `(study_group_id, student_id)`, index `(tenant_id, school_id, class_id)`, `(tenant_id, student_id)`) + migration Atlas + RLS — file: `src/backend/modules/learning/ent/schema/study_group.go`, `src/backend/modules/learning/ent/schema/study_group_member.go`, `src/backend/modules/learning/migrations/<ts>_f040_study_groups.sql` — kiểm: `make migrate-diff MOD=learning name=f040_study_groups`, `make migrate-lint MOD=learning` — ✅ [Confirmed] bảng viết tay trong migration F040 (unique `lower(name)`, `(study_group_id, student_id)`, index học sinh)
- [x] T034 [P] [US4] Domain `studygroup.Group` (1–100 thành viên, `ReplaceMembers` trả diff, event domain) — file: `src/backend/modules/learning/internal/domain/studygroup/study_group.go`, `src/backend/modules/learning/internal/domain/studygroup/events.go` — ✅ `domain/lms/lms.go` (`ValidateGroup`, `Diff`)
- [x] T035 [US4] Command `CreateStudyGroup`, `UpdateStudyGroup`, `DeleteStudyGroup`: kiểm thành viên qua `StudentEnrollmentReader` (`ref.students`, `ref.enrollments`), lưu ent + ghi outbox `study_group.*` + `audit.recorded` cùng transaction, map unique violation → `409 CONFLICT` — file: `src/backend/modules/learning/internal/application/command/lms/create_study_group.go`, `update_study_group.go`, `delete_study_group.go` — ✅ `command/lms.go` (`CreateStudyGroup`, `UpdateStudyGroup`, `DeleteStudyGroup`; unique → 409 trong savepoint)
- [x] T036 [US4] Query `ListStudyGroups`, `GetStudyGroup` + repository ent + handler 5 operation — file: `src/backend/modules/learning/internal/application/query/lms/list_study_groups.go`, `src/backend/modules/learning/internal/infrastructure/persistence/lms/study_group_repo.go`, `src/backend/modules/learning/internal/presentation/http/public/lms_study_group_handler.go` — ✅ `query/lms.go` (`StudyGroups`, `StudyGroup`) + handler 5 operation

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Phân quyền và cô lập tenant (Priority: P1)

**Goal**: Mọi endpoint F040 đúng vai trò, phạm vi, tenant.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T037 [P] [US5] Test tenant isolation + authorization cho mọi endpoint mới (sinh từ OpenAPI) F040-AC-24, F040-AC-25, F040-AC-26: `ad.truong.b` → 404, `tenant_ids` bị bỏ qua, AM ngoài môn `403 PERMISSION_DENIED`, `sm.antoan`/TA ghi `403 PERMISSION_DENIED`, học sinh `403 PORTAL_ACCESS_STAFF_ONLY`, audit `authz.denied` — file: `src/backend/modules/learning/tests/integration/lms/authz_tenant_test.go` — phải FAIL trước khi code — ✅ TestLMSCoursesAndSettings, TestLMSLearningPaths, TestLMSScheduleAndStudyGroups (404 trường khác/tenant B, `tenant_ids` bỏ qua, AM ngoài môn 403, TA ghi 403, học sinh 403 PORTAL_ACCESS_STAFF_ONLY; `authztest.Cover` 19 operation)
- [x] T038 [P] [US5] API test F040-AC-27: System Admin ghi thiếu `X-Tenant-Id` → 400, có header → audit tenant đích — file: `src/backend/modules/learning/tests/integration/lms/system_admin_write_test.go` — ✅ TestLMSCoursesAndSettings (SA thiếu `X-Tenant-Id` 400, có header đọc tenant A)
- [x] T039 [P] [US5] API test F040-AC-28, F040-AC-29: replay `Idempotency-Key` không ghi lần hai; audit mọi thao tác ghi; chuỗi vượt `maxLength` → 422 — file: `src/backend/modules/learning/tests/integration/lms/idempotency_validation_test.go` — ✅ replay `Idempotency-Key` (path, group); audit mọi thao tác ghi; 422 do OpenAPI `maxLength`/enum

### Implementation cho User Story 5

- [x] T040 [US5] Guard authorization (command handler gọi tường minh `authz.Require`) phạm vi `school`/`subject` cho module `lms` + ghi audit qua outbox `audit.recorded` — file: `src/backend/modules/learning/internal/application/command/lms/scope_guard.go` — ✅ `command.AuthorizeLMS` (404 trường ngoài grant, 403 môn/thiếu quyền, audit `authz.denied`)

**Checkpoint**: User Story 5 đạt cho US1–US4.

## Phase 8: Polish & Cross-Cutting

- [x] T041 [P] Metric/span theo plan.md §Quan sát — file: `src/backend/modules/learning/internal/infrastructure/observability/lms_metrics.go` — ✅ `infrastructure/observability/practice_metrics.go` (`eduworld_lms_courses_provisioned_total`, `eduworld_lms_path_node_unlocks_total`)
- [x] T042 [P] Sửa câu chữ CURRICULUM_MODEL §5 về nơi lưu curriculum version theo năm học (sau khi duyệt) + CHANGELOG + trạng thái F040 trong CODE_PLAN — file: `docs/02-learning/CURRICULUM_MODEL.md`, `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — ✅ CHANGELOG, CODE_PLAN (CURRICULUM_MODEL §5 đã trỏ `lms.school_curriculum_settings`)
- [x] T043 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/040-lms-course-path-schedule/quickstart.md` — ✅ quickstart §5

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Đã làm 2026-11-20.

- [x] T044 **OQ chốt 2026-11-20** (OQ F021-5) Resolver version mặc định lọc theo năm học của lớp — file: `src/backend/modules/learning/internal/infrastructure/persistence/curriculum/tree_read_model.go` — kiểm: `make test-integration MOD=learning` — cỡ S — ✅ `read.SchoolSettings.DefaultVersion` lọc theo năm học của lớp trong projection; TestLMSCoursesAndSettings (không năm học → quy tắc cũ, năm khác → không áp, đúng năm → áp)

## Dependencies & Execution Order

- Setup → Foundational (T003–T007) → US1 → US2, US3, US4 (song song) → US5 (test chạy trên US1–US4) → Polish.
- T022 cần rule engine F032 (interface `Rule`); T024 cần F041/F042 phát event (test dùng event mẫu theo schema).
- US4 cần projection `ref.students`, `ref.enrollments` (F020); payload `study_group.*.v1` đã có trong `specs/events/payloads/learning/` (ADR-0084). Channel chat `study_group` thuộc F120.

## Definition of Done

- [x] Mọi tiêu chí F040-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
