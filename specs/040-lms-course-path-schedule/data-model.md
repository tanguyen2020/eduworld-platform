# Data Model: F040 — Course, learning path, lịch, nhóm học tập

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_learning`, schema `lms`. Mọi bảng T dùng mixin `Base` (`tenant_id`, `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason varchar(64)`, `version`), RLS theo `app.tenant_id`. DATABASE_DESIGN §2.14 liệt kê `courses`, `learning_paths` và (ADR-0084) `study_groups`, `study_group_members`; cột dưới đây là **Mới** theo SCHOOL_LMS §2 và phải bổ sung vào DATABASE_DESIGN.

## 1. Entity

### Course — `lms.courses` (Có sẵn tên · cột Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `course_id` | UUIDv7 | PK | mixin `Base` |
| `tenant_id` | uuid | NOT NULL, RLS | R8 |
| `school_id`, `class_id`, `subject_id`, `academic_year_id` | uuid | NOT NULL | `class_id` từ `ref.classes` (không FK chéo DB) |
| `curriculum_version_id` | uuid | NOT NULL | FR-CUR-008 |
| `textbook_series_id` | uuid | NULL | Chọn lộ trình mẫu |
| `status` | `varchar(64)` (`MaxLen(64)`) | CHECK `IN ('active','hidden','archived')` | |
| `deleted_at`, `deleted_by` | | partial unique `(tenant_id, class_id, subject_id, academic_year_id) WHERE deleted_at IS NULL` | R21 |

**Index**: `(tenant_id, school_id, class_id) WHERE deleted_at IS NULL`, `(tenant_id, school_id, subject_id) WHERE deleted_at IS NULL`.

### SchoolCurriculumSetting — `lms.school_curriculum_settings` (Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `setting_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id`, `academic_year_id` | uuid | NOT NULL | |
| `grade_level` | `smallint` | NULL (= mọi khối), 1–12 | Nhiều version song song theo khối |
| `curriculum_version_id` | uuid | NOT NULL | Version đã publish |
| `set_by` | uuid | NOT NULL | |
| `deleted_at`, `deleted_by` | | partial unique `(tenant_id, school_id, academic_year_id, coalesce(grade_level, 0)) WHERE deleted_at IS NULL` | |

### LearningPath — `lms.learning_paths` (Có sẵn tên · cột Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `path_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NULL khi `owner_scope = platform` | Bảng loại H (lộ trình mẫu platform) |
| `owner_scope` | `varchar(64)` | CHECK `IN ('platform','tenant')`, CHECK `(owner_scope='platform') = (tenant_id IS NULL)` | DATABASE_DESIGN §2 loại H |
| `course_id` | uuid | NULL (lộ trình mẫu) | |
| `school_id` | uuid | NULL (mẫu) | |
| `textbook_series_id` | uuid | NULL | Lộ trình mẫu theo bộ sách |
| `title` | `varchar(255)` (`MaxLen(255)`) | NOT NULL | |
| `unlock_mode` | `varchar(64)` | CHECK `IN ('sequential','free','mastery_gate')` | |
| `status` | `varchar(64)` | CHECK `IN ('active','archived')` | |
| `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by` | | NULL | ADR-0029 |
| `deleted_at`, `deleted_by` | | partial unique `(tenant_id, course_id) WHERE deleted_at IS NULL AND status = 'active'` | một lộ trình active/course |

### PathNode — `lms.path_nodes` (Có sẵn, DATABASE_DESIGN §2.16)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `node_id` | UUIDv7 | PK | |
| `path_id`, `school_id` | uuid | NOT NULL (`school_id` NULL với mẫu) | |
| `position` | `int` | UNIQUE `(path_id, position) WHERE deleted_at IS NULL` | |
| `content_ref` | `jsonb` | CHECK `octet_length ≤ 2048` | `{content_type, content_id, content_version_id}` |
| `unlock_rule` | `jsonb` | CHECK `octet_length ≤ 4096` | `{objective_ids[], min_level}` cho `mastery_gate` |

### PathNodeUnlock — `lms.path_node_unlocks` (Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `unlock_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id`, `node_id`, `student_id` | uuid | NOT NULL | |
| `reason` | `varchar(1000)` (`MaxLen(1000)`) | NOT NULL | |
| `unlocked_by` | uuid | NOT NULL | |
| `deleted_at`, `deleted_by` | | partial unique `(tenant_id, node_id, student_id) WHERE deleted_at IS NULL` | |

### ScheduleItem — `lms.schedule_items` (Có sẵn, DATABASE_DESIGN §2.16)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `schedule_item_id` | UUIDv7 | PK | |
| `course_id`, `school_id` | uuid | NOT NULL | |
| `date` | `date` | NOT NULL | |
| `period` | `smallint` | NULL, 1–15 | Tiết |
| `kind` | `varchar(64)` | CHECK `IN ('lesson','assignment_due','exam','event')` | |
| `ref` | `jsonb` | CHECK `octet_length ≤ 2048` | `{ref_type, ref_id}` |
| `source_event_id` | uuid | NULL, partial unique `WHERE deleted_at IS NULL AND source_event_id IS NOT NULL` | Mới: chống tạo trùng từ event |

**Index**: `(tenant_id, course_id, date) WHERE deleted_at IS NULL`.

### StudyGroup — `lms.study_groups` (Mới · ADR-0084, DATABASE_DESIGN §2.14 đã cập nhật)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `study_group_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id` | uuid | NOT NULL, RLS | R8 |
| `class_id` | uuid | NULL (= nhóm theo khối) | Từ `ref.classes`, không FK chéo DB |
| `grade_level` | `smallint` | NOT NULL, 1–12 | Khối của lớp hoặc khối được chọn |
| `name` | `varchar(64)` (`MaxLen(64)`) | NOT NULL, UNIQUE `(tenant_id, school_id, lower(name)) WHERE deleted_at IS NULL` | Trùng → `409 CONFLICT` |
| `member_count` | `int` | NOT NULL, 1–100 | Cập nhật cùng transaction với thành viên |
| `version` | `int` | NOT NULL | ETag / `If-Match`; ghi vào payload event |
| `created_by`, `updated_by` | uuid | NOT NULL | |
| `created_at`, `updated_at`, `deleted_at`, `deleted_by` | | | R21 |

**Index**: `(tenant_id, school_id, class_id) WHERE deleted_at IS NULL`.

### StudyGroupMember — `lms.study_group_members` (Mới · ADR-0084, DATABASE_DESIGN §2.14 đã cập nhật)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `study_group_id` | uuid | NOT NULL, FK `lms.study_groups` | |
| `student_id` | uuid | NOT NULL, UNIQUE `(study_group_id, student_id) WHERE deleted_at IS NULL` | Học sinh active của trường (`ref.students`) |
| `tenant_id`, `school_id` | uuid | NOT NULL, RLS | |
| `added_by` | uuid | NOT NULL | |
| `added_at` | timestamptz | NOT NULL | |
| `deleted_at`, `deleted_by` | | | Bỏ khỏi nhóm = soft-delete |

**Index**: `(tenant_id, student_id) WHERE deleted_at IS NULL` (tra nhóm của học sinh cho F041).

## 2. Vòng đời / trạng thái

| Thực thể | Chuyển trạng thái |
|---|---|
| Course | `active` ⇄ `hidden` (nhà trường) → `archived` (hết năm học hoặc lớp bị xóa) |
| LearningPath | `active` → `archived` (thay bằng lộ trình mới); sửa node tăng `version` (chỉ khóa lạc quan/ETag — path **không** có version nội dung hay workflow phát hành, RB-216; tiến độ giữ theo `node_id`) |
| StudyGroup | tạo → sửa (tăng `version`) → soft-delete (nhóm + thành viên); không có trạng thái khác |
| Node theo học sinh (tính khi đọc) | `locked` → `unlocked` (điều kiện `unlock_mode` hoặc `path_node_unlocks`) → `completed` (attempt graded/lesson completed) |

## 3. Quy tắc kiểm tra dữ liệu

- Môn của course thuộc `curriculum.subject_grades` của curriculum version cho khối của lớp.
- `content_ref` chỉ trỏ version `published` khả dụng cho tenant (MULTITENANCY §4.2); loại `activity` chỉ `activity_type` trong Release (ADR-0072).
- Thành viên study group: 1–100, học sinh active của đúng `school_id` (`ref.students`); có `class_id` thì phải có enrollment active ở lớp đó (`ref.enrollments`); sai → `422 VALIDATION_FAILED`.
- `updateStudyGroup` thay toàn bộ `member_ids`: thành viên mới chèn dòng, thành viên bị bỏ soft-delete; diff dùng cho `study_group.member_changed`.
- `period`, `date` trong năm học của course.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `schedule_item.deleted`, `learning_path.deleted` (mẫu `<entity>.deleted`) | Soft-delete | Cần thêm `specs/events/payloads/learning/<entity>.deleted.v1.schema.json` (mẫu chung EVENT_DESIGN §4.1) |
| `study_group.created` | Tạo nhóm | `specs/events/payloads/learning/study_group.created.v1.schema.json` |
| `study_group.updated` | Đổi tên/lớp/khối | `specs/events/payloads/learning/study_group.updated.v1.schema.json` |
| `study_group.member_changed` | Đổi thành viên (`added_member_ids`, `removed_member_ids`, `member_count`) | `specs/events/payloads/learning/study_group.member_changed.v1.schema.json` |
| `study_group.deleted` | Soft-delete nhóm | `specs/events/payloads/learning/study_group.deleted.v1.schema.json` |
| `audit.recorded` | Mọi thao tác ghi | Envelope chuẩn EVENT_DESIGN §2 |
| Tiêu thụ `class.synced` | Lớp mới/đổi | `specs/events/payloads/identity/class.synced.v1.schema.json` |
