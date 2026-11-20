# Implementation Plan: F041 — Giao bài của nhà trường: assignment, `ResultRelease`, gia hạn/miễn, `my-assignments`

**Branch**: `041-school-assignments` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `lms` của module `learning` thêm aggregate `Assignment` (vòng đời `draft → scheduled → open → closed → results_released → archived`, `cancelled`), đối tượng snapshot, read model `AssignmentSubmission` cập nhật từ event attempt/exam, ngoại lệ cá nhân (gia hạn, miễn), nhắc có giới hạn, worker mở/đóng bài và công bố `after_close`. Attempt `mode = assignment` dùng API attempt của F030 với kiểm tra cửa sổ/lượt; F041 sở hữu port `ExamSessionCreator` với adapter mặc định "chưa khả dụng" nên bài `graded` + blueprint bị từ chối `422 VALIDATION_FAILED`; adapter gọi `CreateExamSession`/`CancelExamSession` do F042 cung cấp (thứ tự F040 → F041 → F042). Danh mục quest/mini-game đọc projection `ref.game_activities`. Rule R1 đăng ký vào rule engine F032. Mọi ghi qua command + outbox (ADR-0038, ADR-0042), có `Idempotency-Key` + audit (R10).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go, `src/backend/pkg/authz`, `src/backend/pkg/tenancy`, `src/backend/pkg/entx`, `src/backend/pkg/outbox`, `src/backend/pkg/idempotency` |
| Storage | PostgreSQL `eduworld_learning` (schema `lms`, `assessment`, `ref`); Valkey chỉ đếm nhắc tạm (nguồn sự thật bảng `lms.assignment_reminders`) |
| Testing | go test + testcontainers (PostgreSQL, NATS); contract test payload `assignment.*`; đồng hồ giả cho worker |
| Target Platform | Linux container (module `learning` api + worker) |
| Project Type | backend module |
| Performance Goals | Tiến độ bài giao gần thời gian thực qua event ([DASHBOARD_DATA §2](../../docs/07-backend/DASHBOARD_DATA.md#2-danh-mục-dashboard)); worker đóng bài chạy theo phút |
| Constraints | AGENTS R5–R11, R20, R26–R29 · ADR-0027, ADR-0043, ADR-0072 |
| Scale/Scope | Trường 1.500 học sinh, lớp 45 học sinh (SCHOOL_LMS §8.8); `targets` ≤ 50 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec.md Ready; delta contract (event payload, `ref.game_activities`) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Dữ liệu game qua projection từ event (không gọi module game đồng bộ); ExamSession là command cùng module |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới `tenant_id` + `deleted_at` + RLS; không hard-delete; test tenant isolation |
| IV Server-authoritative | ✅ | Điểm từ evaluation server; kết quả ẩn tới thời điểm `ResultRelease`; ghi có `Idempotency-Key` + audit |
| V Xác thực, phân quyền | ✅ | `lms.assignment.read`, `lms.assignment.write`, `lms.result.release`; `/lms/*` chỉ staff portal; route học sinh self-only |
| VI An toàn trẻ em | ✅ | Nhắc chỉ thông báo trong game, theo `assignment_remind_limit`; không xếp hạng học sinh (R19) |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; chuỗi `MaxLen`/`maxLength` |
| VIII UI tách logic, token | ✅ | Không có UI trong feature |
| IX Test có bằng chứng | ✅ | Mỗi `F041-AC-*` có task test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/041-school-assignments/
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
│   ├── assignment.go                     # lms.assignments (thêm cột)
│   ├── assignment_target.go              # lms.assignment_targets
│   ├── assignment_target_student.go      # mới: snapshot học sinh
│   ├── assignment_submission.go          # lms.assignment_submissions
│   ├── assignment_student_exception.go   # mới: gia hạn/miễn
│   ├── assignment_reminder.go            # mới
│   └── ref_game_activity.go              # mới: ref.game_activities
├── internal/domain/
│   ├── assignment/                       # assignment.go (vòng đời, bất biến sửa), release_policy.go, window.go (hạn cá nhân + muộn), score_policy.go
│   └── recommendation/r1_assignment_due.go
├── internal/application/
│   ├── command/assignment/               # create_assignment.go, update_assignment.go, publish_assignment.go, cancel_assignment.go,
│   │                                     # archive_assignment.go, release_assignment_results.go, remind_assignment.go,
│   │                                     # extend_for_student.go, excuse_for_student.go, open_due_assignments.go, close_due_assignments.go,
│   │                                     # apply_attempt_event.go, sync_targets_on_enrollment.go
│   ├── query/assignment/                 # list_assignments.go, get_assignment.go, list_submissions.go, list_my_assignments.go, list_game_activities.go
│   └── port/                             # AssignmentRepository, SubmissionRepository, TargetResolver (ClassReader, StudyGroupMemberReader, StudentReader),
│                                         # ContentAvailabilityReader, ExamSessionCreator (F041 sở hữu; adapter thật ở F042), GameActivityReader, Clock
├── internal/infrastructure/persistence/assignment/
├── internal/presentation/
│   ├── http/public/assignment_handler.go, my_assignment_handler.go, game_activity_catalog_handler.go
│   ├── consumer/                         # assignment_attempt_consumer.go, enrollment_assignment_consumer.go, game_activity_projection_consumer.go
│   └── job/                              # assignment_scheduler.go (open/close mỗi phút)
└── tests/{unit,integration,contract}/assignment/
```

**Structure Decision**: Assignment thuộc module `lms` của module `learning` (SCHOOL_LMS §1); attempt `mode = assignment` dùng chung Attempt của F030, chỉ thêm guard cửa sổ/lượt qua port `AssignmentGuard` gọi trong command start attempt.

## Phase 0: Research

Không có. Quy tắc ở SCHOOL_LMS §4, ACTIVITY_ENGINE §8.1, API_DESIGN §9.14.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `assignment.Assignment` (chuyển trạng thái, trường sửa được theo trạng thái), `ReleasePolicy` (FeedbackTiming ≤ ResultRelease), `Window` (hạn cá nhân, cửa sổ muộn), `ScorePolicy` (best/last/first), rule `R1AssignmentDue` | Thuần, có test bảng chuyển trạng thái |
| application | Command `CreateAssignment`, `UpdateAssignment`, `PublishAssignment`, `CancelAssignment`, `ArchiveAssignment`, `ReleaseAssignmentResults`, `RemindAssignment`, `ExtendAssignmentForStudent`, `ExcuseAssignmentForStudent`, `OpenDueAssignments`, `CloseDueAssignments`, `ApplyAttemptEvent`, `SyncTargetsOnEnrollment` · Query `ListAssignments`, `GetAssignment`, `ListAssignmentSubmissions`, `ListMyAssignments`, `ListGameActivities` | Authorization: `lms.assignment.write`/`read`, `lms.result.release`; scope trường/môn |
| infrastructure | Repository ent; SQL đọc `ref.classes`, `ref.enrollments`, `ref.game_activities`; outbox `assignment.*`, `audit.recorded` | SQL có `TenantScope` + `deleted_at IS NULL` |
| presentation | Handler `listAssignments`, `createAssignment`, `getAssignment`, `updateAssignment`, `publishAssignment`, `cancelAssignment`, `releaseAssignmentResults`, `archiveAssignment`, `remindAssignment`, `extendAssignmentForStudent`, `excuseAssignmentForStudent`, `listAssignmentSubmissions`, `listMyAssignments`, `listGameActivities` · consumer attempt/exam/enrollment/game catalog · job `assignment-scheduler` | Consumer idempotent `infra.processed_events` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F032 (rule engine), F040 (course, port thành viên study group), F030 (Attempt, `startActivityAttempt`), F026 (version published), F078 (event danh mục game, mốc sau).
- Không phụ thuộc F042: port `ExamSessionCreator` + adapter mặc định "chưa khả dụng" (`src/backend/modules/learning/internal/infrastructure/examsession/unavailable_creator.go`); F042 đăng ký adapter thật thay thế trong `module.go`.
- Facade: không gọi facade module khác (R29: phản ứng phụ qua event).
- Event phát: `assignment.created`, `assignment.published`, `assignment.results_released`, `assignment.completed` (EVENT_DESIGN §4.3), `assignment.closed`, `assignment.submission_updated`, `assignment.reminded` (Cần thêm), `audit.recorded`. Event tiêu thụ: `activity.started`, `activity.submitted`, `activity.graded` (owner F030), `exam.graded`, `attempt.re_evaluated`, `enrollment.changed`, `user.access_revoked`, event danh mục game của F078 (Cần thêm).

### Bảo mật, tenant, idempotency, audit

- Permission: `lms.assignment.read` (đọc, Tenant Admin chỉ đọc), `lms.assignment.write` (tạo/sửa/publish/cancel/archive/remind/extend/excuse), `lms.result.release` (công bố `manual`).
- `TenantScope`; System Admin ghi với `X-Tenant-Id` tenant đích, `created_by_role = system_admin`.
- `Idempotency-Key` cho mọi POST ghi; `If-Match` cho `updateAssignment`.
- Audit action: `assignment.created`, `assignment.updated`, `assignment.published`, `assignment.cancelled`, `assignment.archived`, `assignment.results_released`, `assignment.reminded`, `assignment.student_extended`, `assignment.student_excused`, `authz.denied`.

### Quan sát

- Log không chứa tên học sinh, câu trả lời (R14); metric `eduworld_lms_assignments_total{status}`, `eduworld_lms_submissions_updated_total{status}`, `eduworld_lms_assignment_reminders_total{result}`; span `PublishAssignment`, `ApplyAttemptEvent`, `CloseDueAssignments`.

Ước lượng: 2 sprint (backend 2 dev) [Confirmed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
