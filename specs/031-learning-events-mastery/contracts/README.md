# Contracts: F031 — Learning events, mastery v1, evaluation và re-evaluate

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/learning.v1.yaml` (gộp trong `specs/openapi/eduworld-api.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getMyProgress` | `GET /api/v1/learning/progress/me` | learning.v1 | `game_player` (self) | Có sẵn (`LearningProgress`) |
| `getStudentProgress` | `GET /api/v1/students/{student_id}/progress` | learning.v1 | `analytics.school.read` (gộp analytics.class.read — ADR-0095 mục 6) + scope trường/môn [Confirmed] | Có sẵn |
| `createRegradeRequest` | `POST /api/v1/cms/learning/regrade-requests` | learning.v1 | `learning_content.re_evaluate` | Có sẵn (`RegradeRequestCreate`, `RegradeRequest`) |
| `getLesson` | `GET /api/v1/learning/lessons/{lesson_id}` | learning.v1 | học sinh (self; nhân sự `403 PERMISSION_DENIED`) | Có sẵn (`Lesson` — version ghim theo lesson session, `activities[].completed`, `completed_at`) |
| `completeLesson` | `POST /api/v1/learning/lessons/{lesson_id}:complete` | learning.v1 | học sinh (self), `Idempotency-Key` | Có sẵn (`LessonCompletion`, mô tả `409`) |
| `getRegradeRequest` | `GET /api/v1/cms/learning/regrade-requests/{regrade_request_id}` | learning.v1 | `learning_content.re_evaluate` | Cần thêm |

ProblemCode: `PERMISSION_DENIED`, `TENANT_MISMATCH`, `CONFLICT`, `EVALUATION_NOT_EDITABLE`, `VALIDATION_FAILED`, `NOT_FOUND`, `IDEMPOTENCY_KEY_REQUIRED`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `activity.graded`, `activity.submitted` | F030 phát / mastery, progress (tiêu thụ nội bộ) | `specs/events/payloads/learning/activity.graded.v1.schema.json`, `activity.submitted.v1.schema.json` | Có sẵn |
| `attempt.re_evaluated` | learning phát / mastery (nội bộ), LMS, NTF, game | `specs/events/payloads/learning/attempt.re_evaluated.v1.schema.json` | Cần thêm |
| `learning_objective.progressed` | learning / game (achievement), RT | `…/learning_objective.progressed.v1.schema.json` | Cần thêm |
| `learning_objective.mastered` | learning / game (XP), NTF, LMS | `…/learning_objective.mastered.v1.schema.json` | Cần thêm |
| `lesson.completed` | learning phát (owner F031) / game (`game.reward_engine`, F074), mastery/progress (nội bộ `learning.progress_daily`) | `specs/events/payloads/learning/lesson.completed.v1.schema.json` | Có sẵn |
| learning event (mọi `event_type`) | ghi `learning.learning_events` | `specs/schemas/learning-event.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/learning-event.schema.json` | Kiểm envelope + payload khi ghi (F031-AC-01, F031-AC-05) | Có sẵn |
| `specs/schemas/common/enums.schema.json` (`AttemptMode`) | `context.mode` | Có sẵn |

## 6. Delta cần thực hiện

- Payload domain `attempt.re_evaluated.v1`, `learning_objective.progressed.v1`, `learning_objective.mastered.v1` khớp `$defs` tương ứng trong `learning-event.schema.json` (task T004).
- `getRegradeRequest` (trạng thái, `affected_attempts`, `re_evaluated_attempts`) (task T004).
- `completeLesson`, `getLesson`: mô tả lesson session, `409 CONFLICT` và hoàn thành idempotent đã ghi vào OpenAPI (task T045 — xong).
- `getStudentProgress`: ghi rõ permission/scope và audit `learning.profile_viewed` trong mô tả (task T004).
- DATABASE_DESIGN: `mastery_model_configs`, cột mới `objective_masteries`, `mastery_history`, `regrade_requests` (T → H), `evaluations.source` = `re_evaluation` (task T006).
