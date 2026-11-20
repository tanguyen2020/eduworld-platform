# Implementation Plan: F055 — Workspace C — LMS: course, learning path, wizard giao bài, theo dõi nộp bài, lịch, nhóm học tập

**Branch**: `055-portal-lms-assignments` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Xây các màn LMS của workspace C trong Admin Portal (một ứng dụng React, ADR-0027 mục 7): danh sách/chi tiết course, learning path builder, wizard giao bài 5 bước (Nội dung → Đối tượng → Thời gian → Chính sách → Xem lại), chi tiết bài với bảng submission và thao tác extend/excuse/remind/release/cancel, lịch, nhóm học tập, tab gia hạn thời gian thi. Mọi dữ liệu và quyết định nằm ở module learning (F040–F042); Portal chỉ gọi API sinh từ OpenAPI qua TanStack Query, validation sinh từ schema, hiển thị theo `getMyPermissions`. Một delta contract (accommodations) đưa vào Phase 2.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) |
| Primary Dependencies | Ant Design 5 + Pro (ProTable, StepsForm, ProForm, Calendar), TanStack Query, React Router, React Hook Form + Zod (schema sinh từ OpenAPI), openapi-typescript (`pnpm gen:api`), i18next (vi/en), dayjs `vi` |
| Storage | Không có; per-viewer `sessionStorage` chỉ cho bản nháp wizard chưa gửi |
| Testing | Vitest + Testing Library + MSW (handler sinh từ OpenAPI, gắn nhãn `MOCK` chỉ trong test), Playwright E2E với backend compose, axe |
| Target Platform | Trình duyệt desktop/tablet/mobile ([ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md)) |
| Project Type | web app (workspace C của `src/cms/frontend/apps/portal`) |
| Performance Goals | Danh sách phân trang cursor; mỗi trang ≤ 4 truy vấn song song khi tải |
| Constraints | AGENTS R1, R3, R7, R8, R10, R20, R28, R32; ADR-0022, ADR-0027, ADR-0047, ADR-0065, ADR-0072 |
| Scale/Scope | Trường 1.500 học sinh, lớp ≤ 45 học sinh; `targets` ≤ `maxItems` của `AssignmentCreate` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta accommodations là task Phase 2 (T004); không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Portal chỉ gọi API public qua APISIX; không logic nghiệp vụ ở client |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` mọi request; System Admin chọn tenant đích; xóa nhóm/lịch = soft-delete ở server; test F055-AC-21 |
| IV Server-authoritative | ✅ | Điểm, trạng thái submission, mặc định release do server; Portal không gửi điểm |
| V Xác thực, phân quyền | ✅ | Menu/nút theo permission; mọi chặn ở backend; test ma trận quyền |
| VI An toàn trẻ em | ✅ | Chỉ hiển thị họ tên/mã học sinh trong scope; không chat, không XP/rating |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn lấy từ schema OpenAPI; tham số theo tên CONFIG_DEFAULTS, không hardcode |
| VIII UI tách logic, token | ✅ | Component chỉ render; hook gọi API; theme `admin-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F055-AC có test Vitest/Playwright viết trước; MSW chỉ trong test |

## Project Structure

### Documentation (feature này)

```text
specs/055-portal-lms-assignments/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/cms/frontend/apps/portal/src/workspaces/school-lms/lms/
├── routes.tsx                         # /portal/school-lms/lms/{courses,assignments,schedule,study-groups}
├── api/                               # hook TanStack Query bọc client sinh
│   ├── assignments.ts                 # useAssignments, useCreateAndPublishAssignment, useSubmissions, useExtend, useExcuse, useRemind, useReleaseResults, useCancel
│   ├── courses.ts                     # useCourses, useUpdateCourse, useLearningPaths, useSaveLearningPath, useUnlockNode
│   ├── schedules.ts · studyGroups.ts · examAccommodations.ts · contentPicker.ts
├── model/
│   ├── assignmentForm.ts              # Zod từ AssignmentCreate + quy tắc hiển thị (graded → tắt follow_latest)
│   └── problemMessages.ts             # ánh xạ ProblemCode → khóa i18n
├── assignments/
│   ├── AssignmentListPage.tsx · AssignmentDetailPage.tsx · SubmissionTable.tsx
│   ├── wizard/{AssignmentWizard,StepContent,StepTargets,StepSchedule,StepPolicy,StepReview}.tsx
│   └── dialogs/{ExtendDialog,ExcuseDialog,CancelDialog,ReleaseResultsConfirm}.tsx
├── courses/{CourseListPage,CourseDetailPage,LearningPathBuilder,UnlockNodeDialog}.tsx
├── schedule/SchedulePage.tsx
├── study-groups/{StudyGroupListPage,StudyGroupForm}.tsx
├── exams/AccommodationsTab.tsx
├── i18n/{vi,en}.json
└── __tests__/*.test.tsx
src/cms/frontend/apps/portal/e2e/lms/*.spec.ts
```

**Structure Decision**: Workspace C là chunk lazy-load `src/workspaces/school-lms` (ADR-0027 mục 7); module `lms` tách khỏi phần quản trị trường của F052. Hook trong `api/` là lớp duy nhất gọi mạng (R1).

## Phase 0: Research

Không có. ADR-0022/0047, [ADMIN_PORTAL](../../docs/04-admin-portal/ADMIN_PORTAL.md), [SCHOOL_LMS](../../docs/04-admin-portal/SCHOOL_LMS.md) và [DESIGN_SYSTEM §4](../../docs/05-ux/DESIGN_SYSTEM.md) đã đủ.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — | Không có domain phía client; quy tắc nghiệp vụ ở module learning |
| application | Hook `api/*.ts` (query/mutation) | Query key có `tenantId`; mutation sinh `Idempotency-Key` UUIDv7 một lần/thao tác (helper của `apiClient` F050; không dùng `crypto.randomUUID` vì sinh UUIDv4 — ADR-0089) |
| infrastructure | `apiClient` (F050), client sinh `eduworld-api.d.ts` | Gắn header, parse Problem Details |
| presentation | Page/wizard/dialog trong `assignments/`, `courses/`, `schedule/`, `study-groups/`, `exams/` | Gọi `listAssignments`, `createAssignment`, `publishAssignment`, `updateAssignment`, `cancelAssignment`, `releaseAssignmentResults`, `remindAssignment`, `extendAssignmentForStudent`, `excuseAssignmentForStudent`, `listAssignmentSubmissions`, `listCourses`, `updateCourse`, `listLearningPaths`, `createLearningPath`, `updateLearningPath`, `unlockPathNodeForStudent`, `listScheduleItems`, `createScheduleItem`, `updateScheduleItem`, `deleteScheduleItem`, `listStudyGroups`, `createStudyGroup`, `updateStudyGroup`, `deleteStudyGroup` |

### Wizard giao bài

| Bước | Dữ liệu | API |
|---|---|---|
| 1 Nội dung | Tab Activity / Bộ câu hỏi / Đề (blueprint) / Bài học / Hoạt động game; lọc khối, môn, LO; chỉ bản đã publish | `listCmsActivities`, `listActivityTypes`, `listCmsQuestionSets`, `listCmsAssessmentBlueprints`, `listCmsLessons`, `listGameActivities`, `listCmsObjectives` |
| 2 Đối tượng | Lớp (nhiều), nhóm học tập, danh sách học sinh | `listCourses`, `listClasses`, `listStudyGroups` |
| 3 Thời gian | `open_at`, `due_at`, `late_policy` | — |
| 4 Chính sách | `mode`, `max_attempts`, `score_policy`, `feedback_timing`, `result_release`, giữ version / `follow_latest` | — |
| 5 Xem lại | Tóm tắt, số học sinh dự kiến | `createAssignment` → `publishAssignment` |

Lối tắt từ F056 ("Giao ôn tập theo LO") mở wizard với query `?objective_id=&class_id=&student_ids=&mode=practice` (đã điền sẵn bước 1–2).

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F040 (course/path/schedule/study group API), F041 (assignment API), F042 (exam session, accommodations), F052 (khung workspace C, danh sách lớp/học sinh), F051 (shell, `getMyPermissions`), F050 (`apiClient`, `ops-ui`).
- Typed client: client TS sinh từ OpenAPI (R29 áp cho Go; web dùng client sinh tương đương). Event: không.

### Bảo mật, tenant, idempotency, audit

- Permission: `lms.class.read`, `lms.assignment.read`, `lms.assignment.write`, `lms.course.write`, `lms.learning_path.write`, `lms.schedule.write`, `lms.study_group.write`, `lms.result.release` ([ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md)).
- Tenant: `X-Tenant-Id` = tenant phiên; System Admin: list gửi `tenant_ids[]` (bộ lọc F057), ghi gửi `tenant_id` + `X-Tenant-Id` tenant đích.
- Idempotency-Key cho `createAssignment`, `publishAssignment`, `cancelAssignment`, `releaseAssignmentResults`, `archiveAssignment`, `remindAssignment`, `extendAssignmentForStudent`, `excuseAssignmentForStudent`, `createLearningPath`, `unlockPathNodeForStudent`, `createScheduleItem`, `createStudyGroup`. Audit ghi ở module learning.

### Quan sát

- Log client chỉ `code`, `trace_id`, route (không họ tên học sinh, không lý do nhập tay); `traceparent` do `apiClient` gắn; lỗi gửi error reporter đã lọc PII (R14).


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.school_lms.assignments`, `menu.school_lms.courses`, `menu.school_lms.schedules`, `menu.school_lms.study_groups`, `menu.school_lms.game_activities` |
| Action dùng | `action.school_lms.assignments.view`, `action.school_lms.assignments.edit`, `action.school_lms.assignments.manage_students`, `action.school_lms.assignments.release_results`, `action.school_lms.courses.view`, `action.school_lms.courses.edit`, `action.school_lms.courses.edit_paths`, `action.school_lms.courses.unlock_node`, `action.school_lms.schedules.view`, `action.school_lms.schedules.edit`, `action.school_lms.study_groups.view`, `action.school_lms.study_groups.edit`, `action.school_lms.game_activities.view`, `action.content_studio.regrade.request` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- ~~`putExamAccommodations`, `previewRegradeRequest`, `getRegradeRequest` cần `x-portal-action`~~ — đã đóng 2026-11-21: `putExamAccommodations` → `action.school_lms.assignments.manage_students` (delta D2); `previewRegradeRequest`/`getRegradeRequest` → `action.content_studio.regrade.request`
- ~~`getExamSession` thiếu `x-portal-action`~~ — đã đóng: `getExamSession`, `listExamSessions` → `action.school_lms.assignments.view`, `createExamSession` → `action.school_lms.assignments.edit`, `releaseExamResults` → `action.school_lms.assignments.release_results`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
