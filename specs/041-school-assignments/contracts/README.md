# Contracts: F041 — Giao bài của nhà trường: assignment, `ResultRelease`, gia hạn/miễn, `my-assignments`

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`), `x-service: learning`. `/lms/*` chỉ staff portal (`azp = eduworld-portal`); `/learning/*` route học sinh self-only.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listAssignments` | `GET /api/v1/lms/assignments` | `learning.v1.yaml` (`Assignment`) | `lms.assignment.read` | Có sẵn |
| `createAssignment` | `POST /api/v1/lms/assignments` | `learning.v1.yaml` (`AssignmentCreate`) | `lms.assignment.write` · `Idempotency-Key` | Có sẵn |
| `getAssignment` | `GET /api/v1/lms/assignments/{assignment_id}` | `learning.v1.yaml` | `lms.assignment.read` | Có sẵn |
| `updateAssignment` | `PATCH /api/v1/lms/assignments/{assignment_id}` | `learning.v1.yaml` | `lms.assignment.write` · `If-Match` | Có sẵn |
| `publishAssignment` | `POST /api/v1/lms/assignments/{assignment_id}:publish` | `learning.v1.yaml` | `lms.assignment.write` · `Idempotency-Key` | Có sẵn |
| `cancelAssignment` | `POST /api/v1/lms/assignments/{assignment_id}:cancel` | `learning.v1.yaml` | `lms.assignment.write` · `Idempotency-Key` | Có sẵn |
| `releaseAssignmentResults` | `POST /api/v1/lms/assignments/{assignment_id}:release-results` | `learning.v1.yaml` | `lms.result.release` · `Idempotency-Key` | Có sẵn |
| `archiveAssignment` | `POST /api/v1/lms/assignments/{assignment_id}:archive` | `learning.v1.yaml` | `lms.assignment.write` · `Idempotency-Key` | Có sẵn |
| `remindAssignment` | `POST /api/v1/lms/assignments/{assignment_id}:remind` | `learning.v1.yaml` (`202 {recipients}`, `429 RateLimited`) | `lms.assignment.write` · `Idempotency-Key` | Có sẵn |
| `extendAssignmentForStudent` | `POST /api/v1/lms/assignments/{assignment_id}/students/{student_id}:extend` | `learning.v1.yaml` | `lms.assignment.write` · `Idempotency-Key` | Có sẵn |
| `excuseAssignmentForStudent` | `POST /api/v1/lms/assignments/{assignment_id}/students/{student_id}:excuse` | `learning.v1.yaml` | `lms.assignment.write` · `Idempotency-Key` | Có sẵn |
| `listAssignmentSubmissions` | `GET /api/v1/lms/assignments/{assignment_id}/submissions` | `learning.v1.yaml` (`AssignmentSubmission`) | `lms.assignment.read` | Có sẵn (thêm `attempts_used`) |
| `listGameActivities` | `GET /api/v1/lms/catalog/game-activities` | `learning.v1.yaml` | `lms.assignment.read` | Có sẵn (thêm filter `objective_ids`) |
| `listMyAssignments` | `GET /api/v1/learning/my-assignments?status=` | `learning.v1.yaml` (`MyAssignment`) | học sinh (self) | Có sẵn |
| `startActivityAttempt` | `POST /api/v1/learning/activities/{activity_id}/attempts` | `learning.v1.yaml` | học sinh (self) · `context.assignment_id` | Có sẵn (F030) |
| `getActivityAttemptReview` | `GET /api/v1/learning/activity-attempts/{attempt_id}/review` | `learning.v1.yaml` | học sinh (self) | Có sẵn (gating theo `ResultRelease`) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (ExamSession là command cùng module; dữ liệu game qua event) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có (thông báo bài mới qua event `assignment.published` → F123) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `assignment.created`, `assignment.published` | Phát | `specs/events/payloads/learning/assignment.created.v1.schema.json` | Có sẵn |
| `assignment.results_released`, `assignment.completed` | Phát | `specs/events/payloads/learning/<event>.v1.schema.json` | Cần thêm |
| `assignment.closed`, `assignment.submission_updated` | Phát | `specs/events/payloads/learning/<event>.v1.schema.json` | Cần thêm (SCHOOL_LMS §12; bổ sung vào EVENT_DESIGN §4.3) |
| `assignment.reminded` | Phát | `specs/events/payloads/learning/assignment.reminded.v1.schema.json` | Cần thêm |
| `activity.graded`, `activity.submitted` | Tiêu thụ | `specs/events/payloads/learning/activity.graded.v1.schema.json`, `activity.submitted.v1.schema.json` | Có sẵn |
| `activity.started`, `activity.submitted`, `exam.graded`, `attempt.re_evaluated` | Tiêu thụ (owner: `activity.*` F030) | `specs/events/payloads/learning/<event>.v1.schema.json` | Cần thêm (thuộc F030/F031/F042) |
| `enrollment.changed`, `user.access_revoked` | Tiêu thụ | `specs/events/payloads/identity/<event>.v1.schema.json` | Cần thêm (thuộc identity) |
| `content.published`, `content.rolled_back`, `content.unpublished` (module game; `item_type` `quest`/`minigame_config`, khối `catalog`) | Tiêu thụ → `ref.game_activities` | `specs/events/payloads/game/content.published.v1.schema.json` (+ `content.rolled_back`, `content.unpublished`) | Cần thêm (owner F071 cho `content.published`, owner F078 cho `rolled_back`/`unpublished`) |
| `tenant_data.assigned`, `tenant_data.unassigned` (`resource_type` `quest`/`minigame_config`) | Tiêu thụ → `ref.game_activities` (tenant được gán) | `specs/events/payloads/identity/tenant_data.assigned.v1.schema.json` (+ `tenant_data.unassigned`) | Cần thêm (owner F045) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có (bài giao tham chiếu nội dung đã có schema ở F022–F026) | — |

## 6. Delta cần thực hiện

- Payload schema mới: `assignment.results_released.v1` (`assignment_id`, `released_at`, `released_by?`), `assignment.completed.v1` (`assignment_id`, `student_id`, `attempt_id`), `assignment.closed.v1` (`assignment_id`, `closed_at`, `missed_count`), `assignment.submission_updated.v1` (`assignment_id`, `student_id`, `status`, `score?`, `rapid_guess_count`), `assignment.reminded.v1` (`assignment_id`, `student_ids[]` maxItems 2000, `reminded_at`); mọi string có `maxLength`.
- Bổ sung `assignment.closed`, `assignment.submission_updated`, `assignment.reminded` vào EVENT_DESIGN §4.3 (đang chỉ có ở SCHOOL_LMS §12).
- Danh mục giao bài dựng từ `content.published`/`content.rolled_back`/`content.unpublished` của module game (khối `catalog`: `item_type`, `item_id`, `current_version_id`, `status`, `objective_ids[]`, `title_i18n` — F078 đề xuất cho schema của owner F071) và `tenant_data.assigned`/`tenant_data.unassigned` (owner F045); rebuild qua facade `listAssignedGameActivities` (F078). Không có event riêng cho catalog.
- `AssignmentSubmission`: thêm `attempts_used` (integer). `listGameActivities`: thêm query `objective_ids` (array uuid, maxItems 20).
- Không có mã lỗi mới: nhắc vượt giới hạn dùng `429 RATE_LIMITED` (response `RateLimited` sẵn có).
