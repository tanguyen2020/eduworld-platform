# Implementation Plan: F040 — Course, learning path, lịch, nhóm học tập

**Branch**: `040-lms-course-path-schedule` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `lms` trong module `learning`: (1) consumer `class.synced` → command `EnsureCoursesForClass` tạo course theo môn của khối trong curriculum version hiệu lực (cài đặt mới `lms.school_curriculum_settings`); (2) CRUD learning path + node ghim `content_version_id`, đánh giá khóa/mở theo `unlock_mode` (đọc `learning.objective_masteries` cùng module), mở khóa thủ công có audit; (3) lịch `lms.schedule_items` + consumer tạo mục `assignment_due`/`exam`; (4) API học sinh xem lộ trình và lịch (Cần thêm); (5) rule R6 đăng ký vào rule engine F032; (6) study group do module learning sở hữu (ADR-0084): `lms.study_groups` + `lms.study_group_members`, CRUD và event `study_group.*` qua outbox cho module chat (M5). Command (write context) và query (read context) tách; ghi qua ent + outbox trong `UnitOfWork` (ADR-0038, ADR-0042, ADR-0087).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go, `src/backend/pkg/authz`, `src/backend/pkg/tenancy`, `src/backend/pkg/entx`, `src/backend/pkg/outbox`, `src/backend/pkg/idempotency` |
| Storage | PostgreSQL `eduworld_learning` (schema `lms`, đọc `curriculum`, `content`, `learning`, `ref`) |
| Testing | go test + testcontainers (PostgreSQL, NATS); contract test với identity (event `class.synced`) |
| Target Platform | Linux container (module `learning` api + worker) |
| Project Type | backend module |
| Performance Goals | Danh sách phân trang cursor theo [API_DESIGN §5](../../docs/07-backend/API_DESIGN.md#5-pagination-filtering-sorting); tạo course cho một lớp trong một transaction |
| Constraints | AGENTS R7, R8, R10, R11, R13, R21, R26–R29 · ADR-0043, ADR-0060, ADR-0072, ADR-0084 |
| Scale/Scope | 50.000 học sinh, 1.500 lớp (DATABASE_DESIGN §8) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec.md Ready (sở hữu study group chốt ở ADR-0084); delta contract ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Lớp/học sinh từ projection `ref`; study group trong DB của module learning, module chat chỉ nhận event `study_group.*` (không gọi đồng bộ, R13/R29) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới có `tenant_id` + `deleted_at` + RLS; xóa = soft-delete; test tenant isolation |
| IV Server-authoritative | ✅ | Khóa/mở node tính ở server từ mastery; ghi có `Idempotency-Key` + audit |
| V Xác thực, phân quyền | ✅ | `lms.course.write`, `lms.learning_path.write`, `lms.schedule.write`, `lms.study_group.write`, `lms.class.read`, `school.settings.write`; route học sinh self-only |
| VI An toàn trẻ em | ✅ | Thành viên nhóm chỉ trong trường; không trả tên thật cho học sinh khác (R16) |
| VII Một nguồn sự thật, cấu hình | ✅ | Không hardcode môn/khối: đọc từ curriculum; chuỗi có `MaxLen` |
| VIII UI tách logic, token | ✅ | Không có UI (portal F055, game F062) |
| IX Test có bằng chứng | ✅ | Mỗi `F040-AC-*` có task test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/040-lms-course-path-schedule/
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
src/backend/modules/learning/
├── ent/schema/
│   ├── course.go                         # lms.courses
│   ├── school_curriculum_setting.go      # mới: lms.school_curriculum_settings
│   ├── learning_path.go                  # lms.learning_paths
│   ├── path_node.go                      # lms.path_nodes
│   ├── path_node_unlock.go               # mới: lms.path_node_unlocks
│   ├── schedule_item.go                  # lms.schedule_items
│   ├── study_group.go                    # mới: lms.study_groups (ADR-0084)
│   └── study_group_member.go             # mới: lms.study_group_members
├── internal/domain/
│   ├── course/                           # course.go (trạng thái), provisioning.go (lớp × môn)
│   ├── learningpath/                     # path.go, node.go, unlock_policy.go (sequential/free/mastery_gate)
│   ├── schedule/                         # schedule_item.go
│   ├── studygroup/                       # study_group.go (1–100 thành viên cùng trường, diff thêm/bớt), events.go
│   └── recommendation/r6_next_topic.go   # rule R6 (interface Rule của F032)
├── internal/application/
│   ├── command/lms/                      # ensure_courses_for_class.go, update_course.go, set_school_curriculum.go,
│   │                                     # create_learning_path.go, update_learning_path.go, unlock_path_node.go,
│   │                                     # create_schedule_item.go, update_schedule_item.go, delete_schedule_item.go,
│   │                                     # upsert_schedule_from_event.go, create_study_group.go, update_study_group.go, delete_study_group.go
│   ├── query/lms/                        # list_courses.go, list_learning_paths.go, list_schedule_items.go, list_study_groups.go,
│   │                                     # get_school_curriculum.go, list_my_learning_paths.go, list_my_schedule.go
│   └── port/                             # CourseRepository, LearningPathRepository, ScheduleRepository, ClassReader (ref),
│                                         # CurriculumReader, ContentAvailabilityReader, MasteryReader, StudyGroupRepository, StudentEnrollmentReader (ref)
├── internal/infrastructure/
│   └── persistence/lms/                  # ent repo + SQL đọc có scope (gồm study_group_repo.go)
├── internal/presentation/
│   ├── http/public/lms_course_handler.go, lms_path_handler.go, lms_schedule_handler.go, lms_study_group_handler.go, my_lms_handler.go
│   └── consumer/                         # class_synced_course_consumer.go, schedule_from_event_consumer.go
└── tests/{unit,integration,contract}/lms/
```

**Structure Decision**: LMS là module của module `learning` (SCHOOL_LMS §1, ADR-0074); khóa/mở node cần mastery cùng database nên không tách module. Study group cũng thuộc module `lms` (ADR-0084): đối tượng giao bài kiểm tra trong cùng module.

## Phase 0: Research

Không có quyết định kỹ thuật cần nghiên cứu. Sở hữu study group đã chốt ở ADR-0084 (module learning).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `course.Course`, `course.Provisioning`, `learningpath.Path` + `UnlockPolicy`, `schedule.Item`, `studygroup.Group` (`ReplaceMembers` trả diff), rule `R6NextTopic` | Bất biến: một course / (class, subject, academic_year); node ghim version; nhóm 1–100 thành viên cùng trường |
| application | Command `EnsureCoursesForClass`, `UpdateCourse`, `SetSchoolCurriculum`, `CreateLearningPath`, `UpdateLearningPath`, `UnlockPathNode`, `CreateScheduleItem`, `UpdateScheduleItem`, `DeleteScheduleItem`, `UpsertScheduleFromEvent`, `CreateStudyGroup`, `UpdateStudyGroup`, `DeleteStudyGroup` · Query `ListCourses`, `ListLearningPaths`, `ListScheduleItems`, `ListStudyGroups`, `GetStudyGroup`, `GetSchoolCurriculum`, `ListMyLearningPaths`, `ListMySchedule` | Mỗi handler gọi tường minh `authz.Require` theo scope tenant/school/subject ở đầu use case |
| infrastructure | Repository ent `lms.*`; SQL đọc `ref.classes`, `ref.students`, `ref.enrollments`, `curriculum.subject_grades`; publisher outbox `study_group.*` | SQL tường minh nhận `TenantScope` + `deleted_at IS NULL` |
| presentation | Handler `listCourses`, `updateCourse`, `listLearningPaths`, `createLearningPath`, `updateLearningPath`, `unlockPathNodeForStudent`, `listScheduleItems`, `createScheduleItem`, `updateScheduleItem`, `deleteScheduleItem`, `listStudyGroups`, `createStudyGroup`, `getStudyGroup`, `updateStudyGroup`, `deleteStudyGroup` + Cần thêm (`getSchoolCurriculumSettings`, `putSchoolCurriculumSettings`, `listMyLearningPaths`, `listMySchedule`) · consumer `class.synced`, `class.deleted`, `assignment.published`, `exam.opened` | Consumer idempotent `infra.processed_events` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F031 (mastery cho `mastery_gate`), F020 (projection `ref`, gán tenant), F021 (curriculum, bộ sách), F026 (nội dung published, version), F032 (rule engine).
- Facade: không gọi facade module khác (study group không gọi module chat; module chat tiêu thụ event ở F120).
- Event tiêu thụ: `class.synced`, `class.deleted`, `enrollment.changed` (identity), `assignment.published` (F041), `exam.opened` (F042). Event phát: `schedule_item.deleted`, `learning_path.deleted`, `study_group.created`, `study_group.updated`, `study_group.member_changed`, `study_group.deleted` (`ew.evt.learning.study_group.<action>`, outbox cùng transaction — ADR-0084), `audit.recorded`.

### Bảo mật, tenant, idempotency, audit

- Permission: `lms.class.read` (đọc), `lms.course.write`, `lms.learning_path.write`, `lms.schedule.write`, `lms.study_group.write`, `school.settings.write` (cài đặt curriculum); route học sinh self-only.
- `TenantScope` cho người dùng tenant; System Admin ghi với `X-Tenant-Id` tenant đích, đọc với `tenant_ids[]` qua `authz.CrossTenantScope` + audit `cross_tenant.accessed`.
- `Idempotency-Key` cho mọi POST tạo/mở khóa; `If-Match` cho PATCH/PUT.
- Audit action: `course.updated`, `school_curriculum.set`, `learning_path.created`, `learning_path.updated`, `path_node.unlocked`, `schedule_item.created`, `schedule_item.updated`, `schedule_item.deleted`, `study_group.created`, `study_group.updated`, `study_group.deleted` (qua outbox `audit.recorded`).

### Quan sát

- Log không có tên học sinh/PII (R14); metric `eduworld_lms_courses_provisioned_total{result}`, `eduworld_lms_path_node_unlocks_total{source}`; span `EnsureCoursesForClass`, `UnlockPolicy.evaluate`.

Ước lượng: 2 sprint (backend 1–2 dev) (gồm study group) [Confirmed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | Không có (Constitution Check đạt toàn bộ) | — |
