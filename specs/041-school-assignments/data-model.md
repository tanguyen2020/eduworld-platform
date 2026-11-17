# Data Model: F041 — Giao bài của nhà trường: assignment, `ResultRelease`, gia hạn/miễn, `my-assignments`

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_learning`. Bảng T dùng mixin `Base` (`tenant_id`, `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason varchar(64)`, `version`), RLS theo `app.tenant_id`.

**Mâu thuẫn cần sửa:** DATABASE_DESIGN §2.14 ghi `assignments.result_visibility`; spec (`AssignmentCreate`), SCHOOL_LMS §2 và ACTIVITY_ENGINE §8.1 dùng `feedback_timing` + `result_release`. Data model theo spec; task T007 sửa DATABASE_DESIGN.

## 1. Entity

### Assignment — `lms.assignments` (Có sẵn tên · Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `assignment_id` | UUIDv7 | PK | mixin `Base` |
| `tenant_id` | uuid | NOT NULL, RLS | R8 |
| `school_id`, `course_id` | uuid | NOT NULL | |
| `created_by` | uuid | NOT NULL | |
| `created_by_role` | `varchar(64)` | CHECK `IN ('school_admin','academic_manager','system_admin')` | |
| `title` | `varchar(120)` (`MaxLen(120)`) | NOT NULL | Khớp `AssignmentCreate.title maxLength 120` |
| `instructions` | `varchar(2000)` (`MaxLen(2000)`) | NULL | |
| `content_type` | `varchar(64)` | CHECK `IN ('activity','question_set','assessment_blueprint','lesson','quest','minigame_config')` | |
| `content_id` | uuid | NOT NULL | |
| `pinned_version_id` | uuid | NULL khi `follow_latest` | DATABASE_DESIGN §3.13 |
| `follow_latest` | `boolean` | NOT NULL DEFAULT false; CHECK `NOT (follow_latest AND mode = 'graded')` | |
| `mode` | `varchar(64)` | CHECK `IN ('practice','graded')` | |
| `open_at`, `due_at` | `timestamptz` | NOT NULL, CHECK `due_at > open_at` | |
| `late_window_hours` | `smallint` | NOT NULL DEFAULT 0, CHECK 0–168 | `late_window_hours` |
| `max_attempts` | `smallint` | 1–10, DEFAULT 1 | |
| `score_policy` | `varchar(64)` | CHECK `IN ('best','last','first')` | |
| `feedback_timing` | `varchar(64)` | CHECK `IN ('immediate','after_submit','after_close','never')` | FeedbackTiming |
| `result_release` | `varchar(64)` | CHECK `IN ('immediate','after_close','manual')` | ResultRelease |
| `status` | `varchar(64)` | CHECK `IN ('draft','scheduled','open','closed','results_released','archived','cancelled')` | |
| `exam_session_id` | uuid | NULL | `mode = graded` + blueprint; F041 tạo cột, F042 ghi giá trị khi publish (F042-AC-31) |
| `objective_ids` | `uuid[]` | CHECK `cardinality ≤ 50` | Snapshot cho R1/dashboard |
| `published_at`, `closed_at`, `results_released_at` | `timestamptz` | NULL | |

**Index**: `(tenant_id, school_id, status, due_at) WHERE deleted_at IS NULL`; `(tenant_id, course_id) WHERE deleted_at IS NULL`.

### AssignmentTarget — `lms.assignment_targets` (Có sẵn tên · cột Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `target_id_pk` | UUIDv7 | PK | |
| `tenant_id`, `school_id`, `assignment_id` | uuid | NOT NULL | |
| `target_type` | `varchar(64)` | CHECK `IN ('class','study_group','students')` | DATABASE_DESIGN ghi `class/group/student` → theo spec |
| `target_ref_id` | uuid | NULL khi `students` | |

### AssignmentTargetStudent — `lms.assignment_target_students` (Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `tenant_id`, `school_id`, `assignment_id`, `student_id` | uuid | PK `(assignment_id, student_id)` | Snapshot khi publish, thêm khi học sinh mới vào lớp |
| `class_id_at_assign` | uuid | NULL | Giữ lớp cũ khi chuyển lớp |
| `source` | `varchar(64)` | CHECK `IN ('publish','enrollment_added')` | |
| `deleted_at`, `deleted_by` | | | |

### AssignmentSubmission — `lms.assignment_submissions` (Có sẵn, DATABASE_DESIGN §2.16 · Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `assignment_id`, `student_id` | uuid | PK | |
| `tenant_id`, `school_id` | uuid | NOT NULL | |
| `status` | `varchar(64)` | CHECK `IN ('not_started','in_progress','submitted','late','missed','excused')` | |
| `best_attempt_id` | uuid | NULL | |
| `score` | `numeric(7,2)` | NULL | Theo `score_policy` |
| `attempts_used` | `smallint` | NOT NULL DEFAULT 0 | Thêm cột |
| `submitted_at` | `timestamptz` | NULL | |
| `rapid_guess_count` | `int` | NOT NULL DEFAULT 0 | Thêm cột (SCHOOL_LMS §2) |
| `last_event_id` | uuid | NULL | Thêm cột: chống áp event cũ |

### AssignmentStudentException — `lms.assignment_student_exceptions` (Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `exception_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id`, `assignment_id`, `student_id` | uuid | NOT NULL | |
| `kind` | `varchar(64)` | CHECK `IN ('extend','excuse')` | |
| `due_at` | `timestamptz` | NULL (bắt buộc khi `extend`) | |
| `reason` | `varchar(1000)` | NOT NULL | |
| `created_by` | uuid | NOT NULL | |
| `deleted_at`, `deleted_by` | | partial unique `(assignment_id, student_id, kind) WHERE deleted_at IS NULL` | Gia hạn lần sau thay dòng cũ (soft-delete) |

### AssignmentReminder — `lms.assignment_reminders` (Mới, TA append-only)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `reminder_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id`, `assignment_id` | uuid | NOT NULL | |
| `reminded_on` | `date` | UNIQUE `(assignment_id, reminded_on)` khi `assignment_remind_limit` = 1/ngày | Đếm theo ngày (ICT) |
| `recipients` | `int` | NOT NULL | |
| `created_by` | uuid | NOT NULL | |

### GameActivityCatalogEntry — `ref.game_activities` (Mới, projection)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `item_type` | `varchar(64)` | CHECK `IN ('quest','minigame_config')` | PK `(item_type, item_id, tenant_id)` |
| `item_id`, `current_version_id` | uuid | NOT NULL | |
| `tenant_id` | uuid | NOT NULL | Một dòng mỗi tenant được gán (hoặc sở hữu) |
| `title_i18n` | `jsonb` | CHECK `octet_length ≤ 2048` | |
| `objective_ids` | `uuid[]` | CHECK `cardinality ≤ 50` | |
| `grade_levels` | `smallint[]` | | |
| `thumbnail_key` | `varchar(512)` | NULL | |
| `status` | `varchar(64)` | `published`/`unpublished`/`unassigned` | |
| `source_version` | `bigint` | NOT NULL | DATABASE_DESIGN §2.17 |

## 2. Vòng đời / trạng thái

Theo [SCHOOL_LMS §4](../../docs/04-admin-portal/SCHOOL_LMS.md#4-assignment): `draft → scheduled|open` (publish) · `scheduled → open` (`open_at`) · `scheduled|open(chưa có submission) → cancelled` · `open → closed` (`due_at` + cửa sổ muộn) · `closed → results_released` (`after_close` tự động / `manual`) · `closed → archived` (`immediate`) · `results_released → archived`.

Submission: `not_started → in_progress → submitted|late`; `not_started|in_progress → missed` (khi đóng); bất kỳ → `excused`; `missed → in_progress` khi gia hạn cá nhân.

## 3. Quy tắc kiểm tra dữ liệu

- `feedback_timing` không sớm hơn `result_release` (ACTIVITY_ENGINE §8.1); graded mặc định `graded_assignment_release_default`.
- Đối tượng và học sinh trong cùng `school_id`; course thuộc scope môn của Academic Manager.
- Nội dung `published`, khả dụng cho tenant; `activity_type` trong Release; `quest`/`minigame_config` có dòng `ref.game_activities` `published` cho tenant.
- Sửa ở `open` chỉ `due_at`, `instructions`, `late_window_hours`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `assignment.created` | Tạo draft | `specs/events/payloads/learning/assignment.created.v1.schema.json` |
| `assignment.published` | Publish | Dùng payload `assignment.created.v1` với `status` `scheduled`/`open` (EVENT_DESIGN §4.3) |
| `assignment.results_released` | Công bố | Cần thêm `assignment.results_released.v1.schema.json` |
| `assignment.completed` | Học sinh hoàn thành | Cần thêm `assignment.completed.v1.schema.json` |
| `assignment.closed` | Worker đóng bài | Cần thêm (SCHOOL_LMS §12; chưa có ở EVENT_DESIGN) |
| `assignment.submission_updated` | Submission đổi | Cần thêm (SCHOOL_LMS §12; chưa có ở EVENT_DESIGN) |
| `assignment.reminded` | Nhắc | Cần thêm (consumer thông báo F123) |
