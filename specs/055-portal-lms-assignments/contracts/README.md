# Contracts: F055 — Workspace C — LMS: course, learning path, wizard giao bài, theo dõi nộp bài, lịch, nhóm học tập

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (x-service `learning` trừ khi ghi khác). Mọi route gửi `X-Tenant-Id`.

> **ADR-0095 (WP-L1, 2026-11-21):** `createAssignment`/`updateAssignment` nhận `school_id`, `subject_id` (không `course_id`) và `targets[]` mới: `{target_type: grade, grade_level, include_student_ids[], exclude_student_ids[]}` hoặc `{target_type: students, student_ids[]}` (1–9 target, exclude thắng include, học sinh khác trường → 422); `Assignment.targets[].student_count` sau publish; `listAssignments` lọc `school_id`, `subject_id`, `grade_level` (bỏ `class_id`, `course_id`); `listAssignmentSubmissions` bỏ `class_id`; `createExamSession` cần `school_id` (bỏ `course_id`). Wizard portal cập nhật ở WP-P1.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listAssignments` | `GET /api/v1/lms/assignments` | eduworld-api | `lms.assignment.read` | Có sẵn |
| `createAssignment` | `POST /api/v1/lms/assignments` | eduworld-api | `lms.assignment.write` | Có sẵn |
| `getAssignment` · `updateAssignment` | `GET`/`PATCH /api/v1/lms/assignments/{assignment_id}` | eduworld-api | `lms.assignment.read` / `.write` | Có sẵn |
| `publishAssignment` | `POST …/{assignment_id}:publish` | eduworld-api | `lms.assignment.write` | Có sẵn |
| `cancelAssignment` · `archiveAssignment` | `POST …:cancel` · `…:archive` | eduworld-api | `lms.assignment.write` | Có sẵn |
| `releaseAssignmentResults` | `POST …:release-results` | eduworld-api | `lms.result.release` | Có sẵn |
| `remindAssignment` | `POST …:remind` | eduworld-api | `lms.assignment.write` | Có sẵn |
| `extendAssignmentForStudent` · `excuseAssignmentForStudent` | `POST …/students/{student_id}:extend` · `:excuse` | eduworld-api | `lms.assignment.write` | Có sẵn |
| `listAssignmentSubmissions` | `GET …/{assignment_id}/submissions` | eduworld-api | `lms.assignment.read` | Có sẵn |
| `listCourses` · `updateCourse` | `GET /api/v1/lms/courses` · `PATCH …/{course_id}` | eduworld-api | `lms.class.read` / `lms.course.write` | **Removed (ADR-0095, WP-L1 2026-11-21)** — op đã xóa khỏi OpenAPI; màn hình gỡ ở WP-P1 |
| `listLearningPaths` · `createLearningPath` · `updateLearningPath` | `…/courses/{course_id}/paths[/{path_id}]` | eduworld-api | `lms.learning_path.write` (ghi) | **Removed (ADR-0095, WP-L1 2026-11-21)** — op đã xóa khỏi OpenAPI; màn hình gỡ ở WP-P1 |
| `unlockPathNodeForStudent` | `POST …/nodes/{node_id}/students/{student_id}:unlock` | eduworld-api | `lms.learning_path.write` | **Removed (ADR-0095, WP-L1 2026-11-21)** — op đã xóa khỏi OpenAPI; màn hình gỡ ở WP-P1 |
| `listScheduleItems` · `createScheduleItem` · `updateScheduleItem` · `deleteScheduleItem` | `/api/v1/lms/schedules[/{schedule_item_id}]` | eduworld-api | `lms.schedule.write` (ghi) | **Removed (ADR-0095, WP-L1 2026-11-21)** — op đã xóa khỏi OpenAPI; màn hình gỡ ở WP-P1 |
| `listStudyGroups` · `createStudyGroup` · `getStudyGroup` · `updateStudyGroup` · `deleteStudyGroup` | `/api/v1/lms/schools/{school_id}/study-groups[/{study_group_id}]` | eduworld-api | `lms.study_group.write` (ghi) | **Removed (ADR-0095, WP-L1 2026-11-21)** — op đã xóa khỏi OpenAPI; màn hình gỡ ở WP-P1 |
| `listGameActivities` | `GET /api/v1/lms/catalog/game-activities` | eduworld-api | `lms.assignment.write` | Có sẵn |
| `listCmsActivities` · `listActivityTypes` · `listCmsQuestionSets` · `listCmsAssessmentBlueprints` · `listCmsLessons` · `listCmsObjectives` | `GET /api/v1/cms/learning/…` | eduworld-api | `learning_content.read` | Có sẵn |
| `listClasses` | `GET /api/v1/schools/{school_id}/classes` | eduworld-api (identity) | `lms.class.read` | Có sẵn |
| `getExamSession` | `GET /api/v1/exams/sessions/{exam_session_id}` | eduworld-api | `lms.assignment.read` | Có sẵn |
| `putExamAccommodations` | `PUT /api/v1/exams/sessions/{exam_session_id}/accommodations` | eduworld-api | `lms.assignment.write` [Confirmed] | Có sẵn (delta D2, F055 2026-11-21) |
| `createRegradeRequest` | `POST /api/v1/cms/learning/regrade-requests` (`scope = exam_session`) | eduworld-api | `learning_content.re_evaluate` · `Idempotency-Key` | Có sẵn |
| `previewRegradeRequest` · `getRegradeRequest` | `POST …/regrade-requests:preview` · `GET …/regrade-requests/{regrade_request_id}` | eduworld-api | `learning_content.re_evaluate` | Có sẵn (F031/F058) |
| `getMyPermissions` | `GET /api/v1/me/permissions` | eduworld-api (identity) | đã đăng nhập portal | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (frontend) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Portal không phát/tiêu thụ event; `assignment.published` do module learning phát (F041) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không | — |

## 6. Delta cần thực hiện

- **Cần thêm** (owner F042, Phase 2 task T004): schema `ExamAccommodation` `{student_id: Uuid, extra_time_percent: integer (min 1, max theo EXAM_ENGINE §5), reason: string maxLength 500}`; trường `accommodations[]` (maxItems 200) trong `ExamSession`; operation `putExamAccommodations` `PUT /exams/sessions/{exam_session_id}/accommodations` (`Idempotency-Key`, `If-Match`, 403/404/409/422). Cập nhật [EXAM_ENGINE §11](../../../docs/02-learning/EXAM_ENGINE.md) cùng PR.
- **Đã làm (2026-11-21):** D2 như trên với body `ExamAccommodationsUpdate {accommodations[], reason}` (lý do chung, `ExamAccommodation` giữ `additionalProperties: false`), 200 `ExamSession` + `ETag`, 409 khi session đã đóng.
- **D1 (2026-11-21, [Confirmed]):** `Course.version`, `ScheduleItem.version` (readOnly) — `If-Match` của `updateCourse`/`updateScheduleItem` từ danh sách (không có GET đơn lẻ).
