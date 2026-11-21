# Tasks: F055 — Workspace C — LMS: course, learning path, wizard giao bài, theo dõi nộp bài, lịch, nhóm học tập

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo module `lms` trong workspace C, khai báo route lazy `/portal/school-lms/lms/*` gắn guard permission từ `getMyPermissions` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/routes.tsx` — **2026-11-21:** `school-lms/index.tsx` gắn 5 màn LMS vào khóa menu catalog; thư mục `school-lms/lms/` (route theo catalog `/school-lms/<menu>`, guard của shell F051)
- [x] T002 [P] Tạo file dịch vi/en cho LMS (khóa `lms.*`, `error.<ProblemCode>`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/i18n/vi.json`, `en.json` — **2026-11-21:** `src/i18n/lms.{vi,en}.json` (khóa `lms.*`, `lms.errors.<CODE>`), test parity trong `lms/model/model.test.ts`
- [x] T003 [P] Ánh xạ ProblemCode LMS (`VALIDATION_FAILED`, `CONTENT_NOT_PUBLISHED`, `RESOURCE_NOT_ASSIGNED`, `PERMISSION_DENIED`, `ASSIGNMENT_NOT_EDITABLE`, `VERSION_MISMATCH`, `CONFLICT`, `INVALID_STATE_TRANSITION`, `RATE_LIMITED`, `TENANT_MISMATCH`) sang khóa i18n — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/model/problemMessages.ts` — **2026-11-21:** `lms/model/problemMessages.ts` + test `model.test.ts`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract (owner F042): thêm `ExamAccommodation`, `ExamSession.accommodations[]`, operation `putExamAccommodations`; cập nhật EXAM_ENGINE §11 — file: `specs/openapi/eduworld-api.v1.yaml`, `docs/02-learning/EXAM_ENGINE.md` — kiểm: `make specs-lint`, `make specs-breaking` — **2026-11-21:** delta D2 `putExamAccommodations` + `ExamAccommodationsUpdate` trong `specs/openapi/eduworld-api.v1.yaml` (split `learning.v1.yaml`), EXAM_ENGINE §5/§11; backend `command/exam_accommodations.go`, integration `exam_accommodations_test.go`
- [x] T005 Sinh lại client TS và kiểm khớp spec — file: `src/cms/frontend/apps/portal/src/api/generated/eduworld-api.d.ts` — kiểm: `pnpm gen:api` + CI diff — **2026-11-21:** `pnpm gen:api` → `schema.gen.d.ts`, `operation-meta.gen.ts`; `pnpm gen:check` xanh
- [x] T006 [P] Zod schema form giao bài sinh từ `AssignmentCreate`/`AssignmentUpdate` + quy tắc hiển thị (graded → tắt `follow_latest`, không gửi release khi không đổi) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/model/assignmentForm.ts` — **2026-11-21:** `lms/model/assignmentForm.ts` — giới hạn/enum mirror từ contract qua `lms/model/fieldSchemas.ts` (mẫu `fieldSchemas` của repo, test so với spec) thay cho Zod; test `model.test.ts`
- [x] T007 [P] Hook query/mutation dùng chung: query key chứa `tenantId`, mutation sinh `Idempotency-Key` một lần/thao tác — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/api/useLmsMutation.ts` — **2026-11-21:** `lms/api/lmsCommon.ts` (query key `[school-lms, lms, name, tenant]`, `useIdempotentMutation`)
- [x] T008 [P] MSW handler sinh từ OpenAPI cho test (gắn nhãn `MOCK`, chỉ dùng trong Vitest) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/__tests__/msw/handlers.ts` — **2026-11-21:** MOCK `src/test/lms-mock.ts` + `lms-harness.tsx` (stand-in trong bộ nhớ theo contract, chỉ test, nhãn MOCK)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Wizard giao bài (Priority: P1) 🎯 MVP

**Goal**: Giao bài cho lớp/nhóm/học sinh và publish.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Test F055-AC-01: bộ chọn nội dung chỉ loại tự chấm đã publish, quest/mini-game từ `listGameActivities` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/__tests__/StepContent.test.tsx` — phải FAIL trước khi code — **2026-11-21:** `lms/assignments/wizard.test.tsx` › content
- [x] T010 [P] [US1] Test F055-AC-02: đối tượng chỉ trong scope API trả — file: `…/__tests__/StepTargets.test.tsx` — **2026-11-21:** `wizard.test.tsx` › targets
- [x] T011 [P] [US1] Test F055-AC-03 và F055-AC-04: graded tắt `follow_latest`, payload không có release mặc định; giới hạn `maxLength`/`maxItems`/`due_at > open_at` — file: `…/__tests__/assignmentForm.test.ts` — **2026-11-21:** `wizard.test.tsx` › limits / graded + `model.test.ts`
- [x] T012 [P] [US1] Test F055-AC-05: double click và retry dùng cùng `Idempotency-Key`, `publishAssignment` gửi `If-Match` — file: `…/__tests__/AssignmentWizard.test.tsx` — **2026-11-21:** `wizard.test.tsx` › double click / network retry
- [x] T013 [P] [US1] Test F055-AC-06: ánh xạ `errors[]` về trường, thông điệp theo `code` + `trace_id` — file: `…/__tests__/AssignmentWizard.errors.test.tsx` — **2026-11-21:** `wizard.test.tsx` › 422 / CONTENT_NOT_PUBLISHED
- [x] T014 [P] [US1] Test F055-AC-07: System Admin bắt buộc tenant đích, header `X-Tenant-Id` = tenant đích — file: `…/__tests__/AssignmentWizard.sysadmin.test.tsx` — **2026-11-21:** `wizard.test.tsx` › System Admin
- [ ] T015 [US1] E2E giao bài `ad.truong.a` → `hs.an.3a` thấy, `hs.chi.3b` không thấy (TC-LMS-001) — file: `src/cms/frontend/apps/portal/e2e/lms/assign.spec.ts`

### Implementation cho User Story 1

- [x] T016 [P] [US1] Hook `useAssignments`, `useCreateAndPublishAssignment` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/api/assignments.ts` — **2026-11-21:** `lms/api/assignments.ts`
- [x] T017 [P] [US1] Hook bộ chọn nội dung (`listCmsActivities`, `listActivityTypes`, `listCmsQuestionSets`, `listCmsAssessmentBlueprints`, `listCmsLessons`, `listGameActivities`, `listCmsObjectives`) — file: `…/lms/api/contentPicker.ts` — **2026-11-21:** `lms/api/contentPicker.ts`
- [x] T018 [US1] Wizard 5 bước + nhận query điền sẵn từ F056 — file: `…/lms/assignments/wizard/AssignmentWizard.tsx`, `StepContent.tsx`, `StepTargets.tsx`, `StepSchedule.tsx`, `StepPolicy.tsx`, `StepReview.tsx` — **2026-11-21:** `lms/assignments/wizard/*` (StepContent, StepTargets, StepDetails = Thời gian + Chính sách; Xem lại trong AssignmentWizard; điền sẵn `?mode=&student_ids=&objective_id=`)
- [x] T019 [US1] Danh sách bài (ProTable, cursor, cột Tenant khi System Admin) — file: `…/lms/assignments/AssignmentListPage.tsx` — **2026-11-21:** `lms/assignments/AssignmentsScreen.tsx` (ResponsiveTable cursor; System Admin làm trên một tenant đích — Clarifications 2026-11-21)

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Theo dõi nộp bài và thao tác cá nhân (Priority: P1)

**Goal**: Theo dõi submission, extend/excuse/remind/release/cancel/sửa.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T020 [P] [US2] Test F055-AC-08: cột bảng submission, nhãn "làm quá nhanh", không cột XP/rating — file: `…/__tests__/SubmissionTable.test.tsx` — phải FAIL trước khi code — **2026-11-21:** `lms/assignments/detail.test.tsx` › table
- [x] T021 [P] [US2] Test F055-AC-09: lý do bắt buộc, `Idempotency-Key`, cập nhật dòng — file: `…/__tests__/ExtendExcuse.test.tsx` — **2026-11-21:** `detail.test.tsx` › extend/excuse
- [x] T022 [P] [US2] Test F055-AC-10: `429 RATE_LIMITED` → thông điệp + khóa nút theo `Retry-After` — file: `…/__tests__/Remind.test.tsx` — **2026-11-21:** `detail.test.tsx` › remind
- [x] T023 [P] [US2] Test F055-AC-11: nút Công bố chỉ khi `manual` + `closed` + `lms.result.release`; hủy báo `409` — file: `…/__tests__/AssignmentActions.test.tsx` — **2026-11-21:** `detail.test.tsx` › release / cancel
- [x] T024 [P] [US2] Test F055-AC-12: `If-Match`, trường khóa khi `open`, `412`/`ASSIGNMENT_NOT_EDITABLE` → tải lại — file: `…/__tests__/AssignmentEdit.test.tsx` — **2026-11-21:** `detail.test.tsx` › edit

### Implementation cho User Story 2

- [x] T025 [P] [US2] Hook `useSubmissions`, `useExtend`, `useExcuse`, `useRemind`, `useReleaseResults`, `useCancel`, `useUpdateAssignment` — file: `…/lms/api/assignments.ts` — **2026-11-21:** `lms/api/assignments.ts`
- [x] T026 [US2] Trang chi tiết bài + `SubmissionTable` (ResponsiveTable thẻ trên mobile) — file: `…/lms/assignments/AssignmentDetailPage.tsx`, `SubmissionTable.tsx` — **2026-11-21:** `AssignmentDetailPage.tsx`, `SubmissionTable.tsx`
- [x] T027 [US2] Dialog gia hạn/miễn/hủy/công bố — file: `…/lms/assignments/dialogs/ExtendDialog.tsx`, `ExcuseDialog.tsx`, `CancelDialog.tsx`, `ReleaseResultsConfirm.tsx` — **2026-11-21:** dialog trong `SubmissionTable.tsx` (gia hạn/miễn), `AssignmentDetailPage.tsx` (hủy, công bố), `AssignmentEditDialog.tsx`

**Checkpoint**: User Story 2 chạy độc lập trên bài đã có.

## Phase 5: User Story 3 — Course và learning path (Priority: P2)

### Tests cho User Story 3

- [x] T028 [P] [US3] Test F055-AC-13: danh sách course, đổi `status` chỉ với `lms.course.write` — file: `…/__tests__/CourseList.test.tsx` — phải FAIL trước khi code — **2026-11-21:** `lms/courses/courses.test.tsx` › course list
- [x] T029 [P] [US3] Test F055-AC-14 và F055-AC-15: node chỉ version đã publish, `maxItems`, path mẫu chỉ đọc; unlock cần lý do + `Idempotency-Key` — file: `…/__tests__/LearningPathBuilder.test.tsx` — **2026-11-21:** `courses.test.tsx` › path builder + unlock

### Implementation cho User Story 3

- [x] T030 [P] [US3] Hook course/path/unlock — file: `…/lms/api/courses.ts` — **2026-11-21:** `lms/api/courses.ts` (If-Match từ `version` — delta D1)
- [x] T031 [US3] Trang course + builder kéo-thả + dialog mở khóa — file: `…/lms/courses/CourseListPage.tsx`, `CourseDetailPage.tsx`, `LearningPathBuilder.tsx`, `UnlockNodeDialog.tsx` — **2026-11-21:** `lms/courses/CoursesScreen.tsx`, `CourseDetailPage.tsx` (sắp xếp bằng nút lên/xuống, dùng được bằng bàn phím)

## Phase 6: User Story 4 — Lịch và nhóm học tập (Priority: P2)

### Tests cho User Story 4

- [x] T032 [P] [US4] Test F055-AC-16: thành viên trong scope, giới hạn `StudyGroupInput`, xóa có xác nhận — file: `…/__tests__/StudyGroupForm.test.tsx` — phải FAIL trước khi code — **2026-11-21:** `courses.test.tsx` › study group
- [x] T033 [P] [US4] Test F055-AC-17: lịch CRUD, `note` `maxLength` — file: `…/__tests__/SchedulePage.test.tsx` — **2026-11-21:** `courses.test.tsx` › schedule

### Implementation cho User Story 4

- [x] T034 [P] [US4] Hook schedule/study group — file: `…/lms/api/schedules.ts`, `…/lms/api/studyGroups.ts` — **2026-11-21:** `lms/api/schedules.ts`
- [x] T035 [US4] Trang lịch (Calendar tháng/tuần) và nhóm học tập — file: `…/lms/schedule/SchedulePage.tsx`, `…/lms/study-groups/StudyGroupListPage.tsx`, `StudyGroupForm.tsx` — **2026-11-21:** `lms/schedule/SchedulesScreen.tsx`, `lms/study-groups/StudyGroupsScreen.tsx`

## Phase 7: User Story 5 — Gia hạn thời gian thi (Priority: P3)

### Tests cho User Story 5

- [x] T036 [P] [US5] Test F055-AC-18: tab chỉ khi có `exam_session_id`, chọn học sinh trong đối tượng, gửi `putExamAccommodations` với `Idempotency-Key` + lý do — file: `…/__tests__/AccommodationsTab.test.tsx` — phải FAIL trước khi code — **2026-11-21:** `lms/exams/exams.test.tsx` › Hỗ trợ thời gian
- [x] T047 [P] [US5] Test F055-AC-23: nút "Chấm lại đề" chỉ khi có `learning_content.re_evaluate` và bài có `exam_session_id` đã chấm; `RegradeDialog` gửi `scope = exam_session`, `Idempotency-Key` giữ khi retry, polling `getRegradeRequest` (TC-LRN-015) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/__tests__/ExamRegradeButton.test.tsx` — phải FAIL trước khi code — **2026-11-21:** `exams.test.tsx` › Chấm lại đề

### Implementation cho User Story 5

- [x] T037 [US5] Hook + tab Hỗ trợ thời gian (sau T004) — file: `…/lms/api/examAccommodations.ts`, `…/lms/exams/AccommodationsTab.tsx` — **2026-11-21:** `lms/api/exams.ts`, `lms/exams/AccommodationsTab.tsx`
- [x] T048 [US5] `ExamRegradeButton` gắn `RegradeDialog` (ops-ui, F058) vào màn bài giao có đề, hook `useExamRegrade` (`previewRegradeRequest`, `createRegradeRequest`, `getRegradeRequest`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/exams/ExamRegradeButton.tsx`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/api/examRegrade.ts` — **2026-11-21:** `lms/exams/ExamRegradeButton.tsx`; `RegradeDialog` (content-studio/regrade, F058) mở rộng `scope = exam_session`

## Phase 8: User Story 6 — Phân quyền và cô lập tenant (Priority: P1)

### Tests cho User Story 6

- [x] T038 [P] [US6] Test F055-AC-19: ma trận role × nút ghi (`tenant_admin`, `school_admin`, `academic_manager` scope môn, System Admin); `403` giữ dữ liệu (TC-TEN-002) — file: `…/__tests__/permissions.matrix.test.tsx` — phải FAIL trước khi code — **2026-11-21:** `lms/permissions.test.tsx` › tenant_admin / 403
- [ ] T039 [P] [US6] E2E F055-AC-20: `sm.antoan` mở URL LMS → trang 403, network log không có chunk workspace C (TC-CMS-WS-01) — file: `src/cms/frontend/apps/portal/e2e/lms/authz.spec.ts`
- [ ] T040 [P] [US6] E2E tenant isolation F055-AC-21: `ad.truong.b` mở bài TH Demo A → "Không tìm thấy"; header cũ → `TENANT_MISMATCH` về chọn đơn vị; người dùng tenant không gửi `tenant_ids` (TC-CMS-AUTHZ-08, TC-TEN-011) — file: `src/cms/frontend/apps/portal/e2e/lms/tenant-isolation.spec.ts`
- [x] T041 [P] [US6] Test F055-AC-22: route/menu workspace C không chứa màn sửa quest/mini-game — file: `…/__tests__/routes.test.ts` — **2026-11-21:** `permissions.test.tsx` › AC-22

### Implementation cho User Story 6

- [x] T042 [US6] Guard permission theo route/nút và xử lý `TENANT_MISMATCH` (xóa cache, điều hướng) — file: `…/lms/routes.tsx`, `…/lms/api/useLmsMutation.ts` — **2026-11-21:** guard route + `TENANT_MISMATCH`/`FEATURE_DISABLED` của shell F051 (sự kiện apiClient); màn LMS dùng `SingleTenantGate`

## Phase N: Polish & Cross-Cutting

- [ ] T043 [P] axe + visual regression sáng/tối/compact cho wizard, chi tiết bài, lịch (TC-UI-004, TC-UI-006) — file: `src/cms/frontend/apps/portal/e2e/lms/a11y-visual.spec.ts`
- [x] T044 [P] Lint chặn hardcode style (TC-UI-003) — kiểm: `make web-lint` — **2026-11-21:** `pnpm lint` (ESLint `ew/no-hardcoded-style` + stylelint)
- [x] T045 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md` — **2026-11-21:** `CHANGELOG.md`
- [ ] T046 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng
- [x] T049 [P] Test F055-AC-24: tắt `menu.school_lms.study_groups` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/lms-feature-toggle.spec.ts` — **2026-11-21:** `permissions.test.tsx` › study groups tắt + FEATURE_DISABLED (vitest với MOCK; bản Playwright chờ backend compose)
- [x] T050 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F055-AC-24) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx` — **2026-11-21:** `school-lms/index.tsx` gắn màn vào khóa catalog; nút bọc `<ActionGate actionKey>`

## ADR-0095 — giao bài theo khối, gỡ course/lộ trình/lịch/nhóm học tập (WP-P1, 2026-11-21)

Nguồn: Clarifications "Session 2026-11-21 (ADR-0095 — [Confirmed])" của spec; CODE_PLAN §4.2 WP-P1; contract WP-L1 (`AssignmentTarget` `grade`/`students`, `school_id`, `subject_id`).

- [x] T051 Gỡ màn Course/Learning Path (US3), Lịch, Nhóm học tập (US4): `lms/courses/`, `lms/schedule/`, `lms/study-groups/`, `lms/api/{courses,schedules}.ts`, gán màn trong `school-lms/index.tsx`, khóa i18n `lms.course|path|schedule|group|courseStatus|unlockMode|unlockRule|scheduleKind`, nhãn catalog, MOCK `lms-mock.ts` (F055-AC-13…AC-17 Removed) — kiểm: `permissions.test.tsx` (không route/menu/lời gọi course, lịch, nhóm)
- [x] T052 Wizard bước "Đối tượng" theo khối: trường (`SchoolSelect`), môn (`listSubjects`, lọc môn của role theo môn), "Theo khối" (khối trường dạy ∩ 1–9, thêm học sinh khối khác cùng trường, bớt học sinh, cảnh báo exclude thắng include) hoặc "Học sinh cụ thể"; `toTargets` (mỗi khối một target, include ở khối đầu, exclude ở mọi khối); giới hạn từ `fieldSchemas` (≤ 9 target, khối 1–9, ≤ 200 học sinh); `422` tại `/targets/i/…` → đúng ô và bước; lối tắt `?school_id&subject_id&grade&student_ids` (F055-AC-02, AC-04, AC-05, AC-06) — file: `lms/model/assignmentForm.ts`, `lms/model/fieldSchemas.ts`, `lms/api/targets.ts`, `lms/assignments/wizard/{StepTargets,AssignmentWizard}.tsx` — kiểm: `model.test.ts`, `wizard.test.tsx`
- [x] T053 Danh sách bài giao lọc trường/môn/khối/trạng thái (`listAssignments` `school_id`, `subject_id`, `grade_level`), cột "Đối tượng" (`TargetsText`); bảng nộp bài không cột lớp — file: `lms/api/assignments.ts`, `lms/assignments/{AssignmentsScreen,AssignmentFilterBar}.tsx`, `lms/LmsUi.tsx`
- [x] T054 Kiểm thử: `pnpm typecheck` 0 lỗi, `pnpm lint` sạch, `pnpm gen:check` khớp; Vitest `wizard.test.tsx` 12/12, `model.test.ts`, `permissions.test.tsx` 7/7; toàn bộ portal 2 worker (2026-11-22): 847/851 lần đầu, 4 lỗi đã sửa (nhãn menu cũ, MOCK `listTenantUsers`) hoặc chạy lại pass (chậm do tải máy)
- [x] T055 Sau WP-I1: ô chọn học sinh của wizard dùng `listTenantGradeStudents` (bộ chọn khối + tìm trên server) thay tìm tạm thời `listTenantUsers`; test `wizard.test.tsx` (lời gọi `/tenant/schools/{school_id}/grades/{grade}/students`, không gọi `/tenant/users`) — file: `lms/api/targets.ts`, `lms/assignments/wizard/StepTargets.tsx`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 (cần bài đã tạo) → US6 (song song từ khi có US1) → US3, US4 (song song) → US5 (cần T004 merge) → Polish.
- T004 do owner F042 duyệt; T005 chạy lại sau mọi đổi spec.

## Definition of Done

- [x] Mọi tiêu chí F055-AC-* có test pass (ghi mã test/TC) — **2026-11-21:** Vitest 79/79 (`school-lms/lms/**`, `content-studio/regrade`, i18n) + Go integration learning 4/4; E2E Playwright còn mở (T015, T039, T040, T043)
- [ ] `make web-lint`, `make web-test`, Playwright E2E xanh với backend thật
- [x] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17) — MOCK chỉ ở `src/test/lms-mock.ts`
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — EXAM_ENGINE §5/§11, CHANGELOG; CONFIG_DEFAULTS không đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
