# Contracts: F040 — Course, learning path, lịch, nhóm học tập

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`), `x-service: learning`. Route `/lms/*`: `azp = eduworld-portal`, `user_type = staff`, header `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listCourses` | `GET /api/v1/lms/courses` | `learning.v1.yaml` (`Course`) | `lms.class.read` | Có sẵn |
| `updateCourse` | `PATCH /api/v1/lms/courses/{course_id}` | `learning.v1.yaml` | `lms.course.write` · `If-Match` | Có sẵn |
| `listLearningPaths` | `GET /api/v1/lms/courses/{course_id}/paths` | `learning.v1.yaml` (`LearningPath`) | `lms.class.read` | Có sẵn (thêm cờ `is_template`) |
| `createLearningPath` | `POST /api/v1/lms/courses/{course_id}/paths` | `learning.v1.yaml` (`LearningPathInput`) | `lms.learning_path.write` · `Idempotency-Key` | Có sẵn |
| `updateLearningPath` | `PATCH /api/v1/lms/courses/{course_id}/paths/{path_id}` | `learning.v1.yaml` | `lms.learning_path.write` · `If-Match` | Có sẵn |
| `unlockPathNodeForStudent` | `POST /api/v1/lms/courses/{course_id}/paths/{path_id}/nodes/{node_id}/students/{student_id}:unlock` | `learning.v1.yaml` (`ReasonRequest`) | `lms.learning_path.write` · `Idempotency-Key` | Có sẵn |
| `listScheduleItems` | `GET /api/v1/lms/schedules` | `learning.v1.yaml` (`ScheduleItem`) | `lms.class.read` | Có sẵn |
| `createScheduleItem` | `POST /api/v1/lms/schedules` | `learning.v1.yaml` | `lms.schedule.write` · `Idempotency-Key` | Có sẵn |
| `updateScheduleItem` | `PATCH /api/v1/lms/schedules/{schedule_item_id}` | `learning.v1.yaml` | `lms.schedule.write` · `If-Match` | Có sẵn |
| `deleteScheduleItem` | `DELETE /api/v1/lms/schedules/{schedule_item_id}` | `learning.v1.yaml` | `lms.schedule.write` | Có sẵn (soft-delete) |
| `listStudyGroups` | `GET /api/v1/lms/schools/{school_id}/study-groups` | `learning.v1.yaml` (`StudyGroup`) | `lms.class.read` | Có sẵn (ADR-0084: learning sở hữu) |
| `createStudyGroup` | `POST /api/v1/lms/schools/{school_id}/study-groups` | `learning.v1.yaml` | `lms.study_group.write` · `Idempotency-Key` | Có sẵn |
| `getStudyGroup` | `GET /api/v1/lms/schools/{school_id}/study-groups/{study_group_id}` | `learning.v1.yaml` | `lms.class.read` | Có sẵn |
| `updateStudyGroup` | `PATCH /api/v1/lms/schools/{school_id}/study-groups/{study_group_id}` | `learning.v1.yaml` | `lms.study_group.write` · `If-Match` | Có sẵn |
| `deleteStudyGroup` | `DELETE /api/v1/lms/schools/{school_id}/study-groups/{study_group_id}` | `learning.v1.yaml` | `lms.study_group.write` | Có sẵn (soft-delete) |
| `getSchoolCurriculumSettings` | `GET /api/v1/lms/schools/{school_id}/curriculum-settings?academic_year_id=` | — | `lms.class.read` | Cần thêm |
| `putSchoolCurriculumSettings` | `PUT /api/v1/lms/schools/{school_id}/curriculum-settings` | — | `school.settings.write` · `If-Match` | Cần thêm |
| `listMyLearningPaths` | `GET /api/v1/learning/my-learning-paths?subject_id=` | — | học sinh (self) | Cần thêm |
| `listMySchedule` | `GET /api/v1/learning/my-schedule?from=&to=` | — | học sinh (self) | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (study group không gọi module chat; module chat tiêu thụ event `study_group.*` ở F120 — ADR-0084) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `class.synced` | Tiêu thụ | `specs/events/payloads/identity/class.synced.v1.schema.json` | Có sẵn |
| `class.deleted`, `enrollment.changed` | Tiêu thụ | `specs/events/payloads/identity/<event>.v1.schema.json` | Cần thêm (thuộc identity) |
| `assignment.published` | Tiêu thụ (tạo lịch `assignment_due`) | `specs/events/payloads/learning/assignment.created.v1.schema.json` (dùng chung payload) | Có sẵn |
| `exam.opened` | Tiêu thụ (tạo lịch `exam`) | — | Cần thêm (có ở EXAM_ENGINE §5, chưa có ở EVENT_DESIGN §4.3 — thuộc F042) |
| `schedule_item.deleted`, `learning_path.deleted` | Phát | `specs/events/payloads/learning/<entity>.deleted.v1.schema.json` | Cần thêm |
| `study_group.created`, `study_group.updated`, `study_group.member_changed`, `study_group.deleted` | Phát (`ew.evt.learning.study_group.<action>`, outbox cùng transaction); consumer module chat ở F120 | `specs/events/payloads/learning/study_group.{created,updated,member_changed,deleted}.v1.schema.json` | Đã thêm theo ADR-0084 |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có (lộ trình không phải nội dung có JSON Schema riêng) | — |

## 6. Delta cần thực hiện

- Thêm `getSchoolCurriculumSettings`/`putSchoolCurriculumSettings` (`x-service: learning`): body `items[] {academic_year_id (uuid), grade_level (GradeLevel|null), curriculum_version_id (uuid)}`, `maxItems: 13`; response thêm `set_by`, `updated_at`.
- Thêm `listMyLearningPaths` (`x-service: learning`, route học sinh): `items[] {course_id, subject_id, subject_name (maxLength 255), path_id, title (maxLength 255), unlock_mode, nodes[] {node_id, position, content_type, content_id, content_version_id, title (maxLength 255), state (enum locked/unlocked/completed)}}`.
- Thêm `listMySchedule` (`x-service: learning`, route học sinh): query `from`, `to` (date, ≤ 62 ngày); `items[] {schedule_item_id, course_id, date, period, kind, ref_type (maxLength 64), ref_id, title (maxLength 255)}`.
- `LearningPath`: thêm `is_template` (boolean), `textbook_series_id`; `LearningPathInput.title` kiểm `maxLength: 255`.
- `Course`: thêm `curriculum_version_id` vào `required` khi trả về.
- Study group (ADR-0084): bỏ `StudyGroup.club_id`, mô tả `listStudyGroups`/`createStudyGroup` không còn nhắc club — Đã thêm theo ADR-0084 (spec gộp + `split-public.mjs`). Payload `study_group.*.v1` — Đã thêm theo ADR-0084. Kiểm lại khi code: `make specs-lint`, `make gen MOD=learning`.
- Payload schema `<entity>.deleted.v1` cho `schedule_item`, `learning_path`.
