# Tasks: F040 — Course, learning path, lịch, nhóm học tập

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package module `lms` (domain, application, persistence, handler) và đăng ký route/consumer trong composition root — file: `modules/learning/cmd/api/main.go`, `modules/learning/cmd/worker/main.go`
- [ ] T002 [P] Đăng ký permission dùng trong module (`lms.class.read`, `lms.course.write`, `lms.learning_path.write`, `lms.schedule.write`, `lms.study_group.write`, `school.settings.write`) vào kiểm tra catalog lúc khởi động — file: `modules/learning/internal/application/port/permissions.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta contract: thêm `getSchoolCurriculumSettings`, `putSchoolCurriculumSettings`, `listMyLearningPaths`, `listMySchedule`; `LearningPath.is_template`, `textbook_series_id`; `Course.curriculum_version_id` required — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T004 [P] Payload schema `<entity>.deleted.v1` cho entity LMS — file: `specs/events/payloads/learning/schedule_item.deleted.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T005 ent schema `lms.courses`, `lms.school_curriculum_settings`, `lms.learning_paths`, `lms.path_nodes`, `lms.path_node_unlocks`, `lms.schedule_items` (mọi chuỗi `MaxLen`, partial unique) — file: `modules/learning/ent/schema/course.go`, `school_curriculum_setting.go`, `learning_path.go`, `path_node.go`, `path_node_unlock.go`, `schedule_item.go` — kiểm: `make migrate-diff MOD=learning name=f040_lms_core`, `make migrate-lint MOD=learning`
- [ ] T006 RLS + policy cho bảng mới, loại H cho `learning_paths` (platform + gán) trong migration — file: `modules/learning/migrations/<ts>_f040_lms_core.sql`
- [ ] T007 [P] Cập nhật DATABASE_DESIGN §2.14/§2.16 (cột course, learning_paths, bảng mới; `lms.study_groups`/`lms.study_group_members` đã ghi theo ADR-0084 — chỉ đối chiếu) — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Course tự tạo theo lớp × môn × năm học (Priority: P1) 🎯 MVP

**Goal**: Course tự tạo, cài đặt curriculum, ẩn/hiện course.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T008 [P] [US1] Integration test F040-AC-01, F040-AC-02: phát `class.synced` hai lần → một course/môn, đúng `curriculum_version_id` (có/không cài đặt) — file: `modules/learning/tests/integration/lms/course_provisioning_test.go` — phải FAIL trước khi code
- [ ] T009 [P] [US1] API test F040-AC-03: version chưa publish `409`, chưa gán `403 RESOURCE_NOT_ASSIGNED`, audit, chỉ áp course mới — file: `modules/learning/tests/integration/lms/school_curriculum_settings_test.go`
- [ ] T010 [P] [US1] API test F040-AC-04, F040-AC-05: phạm vi `listCourses` theo SA/AM/TA; `updateCourse` `428`/`412`/thành công + audit — file: `modules/learning/tests/integration/lms/course_api_test.go`

### Implementation cho User Story 1

- [ ] T011 [P] [US1] Domain `course.Course`, `course.Provisioning` (lớp × môn × năm học) — file: `modules/learning/internal/domain/course/course.go`, `modules/learning/internal/domain/course/provisioning.go`
- [ ] T012 [US1] Command `EnsureCoursesForClass`, `UpdateCourse`, `SetSchoolCurriculum` — file: `modules/learning/internal/application/command/lms/ensure_courses_for_class.go`, `update_course.go`, `set_school_curriculum.go`
- [ ] T013 [US1] Query `ListCourses`, `GetSchoolCurriculum` + repository có scope — file: `modules/learning/internal/application/query/lms/list_courses.go`, `modules/learning/internal/infrastructure/persistence/lms/course_repo.go`
- [ ] T014 [US1] Consumer `class.synced`, `class.deleted` → course; handler `listCourses`, `updateCourse`, `getSchoolCurriculumSettings`, `putSchoolCurriculumSettings` — file: `modules/learning/internal/presentation/consumer/class_synced_course_consumer.go`, `modules/learning/internal/presentation/http/public/lms_course_handler.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Learning path cho học sinh tự học (Priority: P1)

**Goal**: CRUD lộ trình, khóa/mở node, mở khóa thủ công, R6.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T015 [P] [US2] API test F040-AC-06, F040-AC-07: tạo/sửa path ghim version; nội dung chưa publish `409`, chưa gán `403` — file: `modules/learning/tests/integration/lms/learning_path_api_test.go` — phải FAIL trước khi code
- [ ] T016 [P] [US2] Unit test F040-AC-09: `UnlockPolicy` cho `sequential`/`free`/`mastery_gate` và mở khóa thủ công — file: `modules/learning/tests/unit/learningpath/unlock_policy_test.go`
- [ ] T017 [P] [US2] Integration test F040-AC-08, F040-AC-10, F040-AC-11: lộ trình mẫu; unlock thiếu lý do `422`, học sinh ngoài lớp `404`, audit; `listMyLearningPaths` bỏ course hidden — file: `modules/learning/tests/integration/lms/my_learning_paths_test.go`
- [ ] T018 [P] [US2] Unit test F040-AC-12: rule R6 tạo gợi ý ưu tiên 4 — file: `modules/learning/tests/unit/recommendation/r6_next_topic_test.go`

### Implementation cho User Story 2

- [ ] T019 [P] [US2] Domain `learningpath.Path`, `Node`, `UnlockPolicy` — file: `modules/learning/internal/domain/learningpath/path.go`, `modules/learning/internal/domain/learningpath/unlock_policy.go`
- [ ] T020 [US2] Command `CreateLearningPath`, `UpdateLearningPath`, `UnlockPathNode` (kiểm nội dung published + gán qua `ContentAvailabilityReader`) — file: `modules/learning/internal/application/command/lms/create_learning_path.go`, `update_learning_path.go`, `unlock_path_node.go`
- [ ] T021 [US2] Query `ListLearningPaths`, `ListMyLearningPaths` (đọc `MasteryReader`) + handler — file: `modules/learning/internal/application/query/lms/list_my_learning_paths.go`, `modules/learning/internal/presentation/http/public/lms_path_handler.go`, `modules/learning/internal/presentation/http/public/my_lms_handler.go`
- [ ] T022 [US2] Rule R6 và đăng ký vào `RuleRegistry` — file: `modules/learning/internal/domain/recommendation/r6_next_topic.go`, `modules/learning/cmd/worker/main.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Lịch học, hạn nộp, đề (Priority: P2)

**Goal**: CRUD lịch, lịch từ event, lịch học sinh.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T023 [P] [US3] API test F040-AC-13, F040-AC-14: tạo/sửa trong phạm vi; xóa = soft-delete, audit, `schedule_item.deleted`, biến mất khỏi API đọc — file: `modules/learning/tests/integration/lms/schedule_api_test.go` — phải FAIL trước khi code
- [ ] T024 [P] [US3] Integration test F040-AC-15: `assignment.published`/`exam.opened` giao 2 lần → một mục lịch — file: `modules/learning/tests/integration/lms/schedule_from_event_test.go`
- [ ] T025 [P] [US3] API test F040-AC-16: `listMySchedule` chỉ course active của lớp học sinh, đúng khoảng — file: `modules/learning/tests/integration/lms/my_schedule_test.go`

### Implementation cho User Story 3

- [ ] T026 [P] [US3] Domain `schedule.Item` — file: `modules/learning/internal/domain/schedule/schedule_item.go`
- [ ] T027 [US3] Command `CreateScheduleItem`, `UpdateScheduleItem`, `DeleteScheduleItem`, `UpsertScheduleFromEvent` — file: `modules/learning/internal/application/command/lms/create_schedule_item.go`, `delete_schedule_item.go`, `upsert_schedule_from_event.go`
- [ ] T028 [US3] Query `ListScheduleItems`, `ListMySchedule`; handler; consumer `assignment.published`, `exam.opened` — file: `modules/learning/internal/presentation/http/public/lms_schedule_handler.go`, `modules/learning/internal/presentation/consumer/schedule_from_event_consumer.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Nhóm học tập (Priority: P3)

**Goal**: CRUD nhóm học tập do module learning sở hữu (ADR-0084), event `study_group.*` qua outbox.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T029 [P] [US4] Unit test domain F040-AC-19, F040-AC-22: `ReplaceMembers` trả đúng diff thêm/bớt, chặn 0 hoặc > 100 thành viên, `name` rỗng/> 64 — file: `modules/learning/tests/unit/studygroup/study_group_test.go` — phải FAIL trước khi code
- [ ] T030 [P] [US4] API test F040-AC-17, F040-AC-18, F040-AC-19, F040-AC-20: tạo nhóm theo lớp/khối + audit + replay `Idempotency-Key`; list lọc `class_id`/`grade_level`, get có `member_ids` + `ETag`; update `428`/`412`/thay danh sách; delete = soft-delete nhóm + thành viên, biến mất khỏi API đọc — file: `modules/learning/tests/integration/lms/study_group_api_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US4] API test F040-AC-21, F040-AC-22: trùng tên khác hoa thường `409 CONFLICT`; học sinh trường B, học sinh không thuộc `class_id`, 101 thành viên, tên 65 ký tự → `422 VALIDATION_FAILED` không lộ học sinh trường khác — file: `modules/learning/tests/integration/lms/study_group_validation_test.go`
- [ ] T032 [P] [US4] Integration test F040-AC-23: mỗi thao tác ghi đúng một dòng `infra.outbox_events` cùng transaction (`study_group.created/updated/member_changed/deleted`), payload validate với `specs/events/payloads/learning/study_group.*.v1.schema.json`, không có tên học sinh; rollback → không có event — file: `modules/learning/tests/integration/lms/study_group_outbox_test.go`

### Implementation cho User Story 4

- [ ] T033 [US4] ent schema `lms.study_groups`, `lms.study_group_members` (`name` `MaxLen(64)`, partial unique `(tenant_id, school_id, lower(name))`, `(study_group_id, student_id)`, index `(tenant_id, school_id, class_id)`, `(tenant_id, student_id)`) + migration Atlas + RLS — file: `modules/learning/ent/schema/study_group.go`, `modules/learning/ent/schema/study_group_member.go`, `modules/learning/migrations/<ts>_f040_study_groups.sql` — kiểm: `make migrate-diff MOD=learning name=f040_study_groups`, `make migrate-lint MOD=learning`
- [ ] T034 [P] [US4] Domain `studygroup.Group` (1–100 thành viên, `ReplaceMembers` trả diff, event domain) — file: `modules/learning/internal/domain/studygroup/study_group.go`, `modules/learning/internal/domain/studygroup/events.go`
- [ ] T035 [US4] Command `CreateStudyGroup`, `UpdateStudyGroup`, `DeleteStudyGroup`: kiểm thành viên qua `StudentEnrollmentReader` (`ref.students`, `ref.enrollments`), lưu ent + ghi outbox `study_group.*` + `audit.recorded` cùng transaction, map unique violation → `409 CONFLICT` — file: `modules/learning/internal/application/command/lms/create_study_group.go`, `update_study_group.go`, `delete_study_group.go`
- [ ] T036 [US4] Query `ListStudyGroups`, `GetStudyGroup` + repository ent + handler 5 operation — file: `modules/learning/internal/application/query/lms/list_study_groups.go`, `modules/learning/internal/infrastructure/persistence/lms/study_group_repo.go`, `modules/learning/internal/presentation/http/public/lms_study_group_handler.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Phân quyền và cô lập tenant (Priority: P1)

**Goal**: Mọi endpoint F040 đúng vai trò, phạm vi, tenant.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T037 [P] [US5] Test tenant isolation + authorization cho mọi endpoint mới (sinh từ OpenAPI) F040-AC-24, F040-AC-25, F040-AC-26: `ad.truong.b` → 404, `tenant_ids` bị bỏ qua, AM ngoài môn `403 PERMISSION_DENIED`, `sm.antoan`/TA ghi `403 PERMISSION_DENIED`, học sinh `403 PORTAL_ACCESS_STAFF_ONLY`, audit `authz.denied` — file: `modules/learning/tests/integration/lms/authz_tenant_test.go` — phải FAIL trước khi code
- [ ] T038 [P] [US5] API test F040-AC-27: System Admin ghi thiếu `X-Tenant-Id` → 400, có header → audit tenant đích — file: `modules/learning/tests/integration/lms/system_admin_write_test.go`
- [ ] T039 [P] [US5] API test F040-AC-28, F040-AC-29: replay `Idempotency-Key` không ghi lần hai; audit mọi thao tác ghi; chuỗi vượt `maxLength` → 422 — file: `modules/learning/tests/integration/lms/idempotency_validation_test.go`

### Implementation cho User Story 5

- [ ] T040 [US5] Guard authorization (command handler gọi tường minh `authz.Require`) phạm vi `school`/`subject` cho module `lms` + ghi audit qua outbox `audit.recorded` — file: `modules/learning/internal/application/command/lms/scope_guard.go`

**Checkpoint**: User Story 5 đạt cho US1–US4.

## Phase 8: Polish & Cross-Cutting

- [ ] T041 [P] Metric/span theo plan.md §Quan sát — file: `modules/learning/internal/infrastructure/observability/lms_metrics.go`
- [ ] T042 [P] Sửa câu chữ CURRICULUM_MODEL §5 về nơi lưu curriculum version theo năm học (sau khi duyệt) + CHANGELOG + trạng thái F040 trong CODE_PLAN — file: `docs/02-learning/CURRICULUM_MODEL.md`, `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T043 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/040-lms-course-path-schedule/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T003–T007) → US1 → US2, US3, US4 (song song) → US5 (test chạy trên US1–US4) → Polish.
- T022 cần rule engine F032 (interface `Rule`); T024 cần F041/F042 phát event (test dùng event mẫu theo schema).
- US4 cần projection `ref.students`, `ref.enrollments` (F020); payload `study_group.*.v1` đã có trong `specs/events/payloads/learning/` (ADR-0084). Channel chat `study_group` thuộc F120.

## Definition of Done

- [ ] Mọi tiêu chí F040-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
