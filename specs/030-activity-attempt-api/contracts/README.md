# Contracts: F030 — Activity Registry và Attempt API (`quiz`, `interactive_exercise`)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/learning.v1.yaml` (gộp trong `specs/openapi/eduworld-api.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getLearningActivity` | `GET /api/v1/learning/activities/{activity_id}` | learning.v1 | `game_player` | Có sẵn |
| `startActivityAttempt` | `POST /api/v1/learning/activities/{activity_id}/attempts` | learning.v1 | `game_player` | Có sẵn (`ActivityAttemptStart`, `Attempt`) |
| `startAdHocAttempt` | `POST /api/v1/learning/activity-attempts` | learning.v1 | `game_player` | Có sẵn (`AdHocAttemptStart`) |
| `listMyActivityAttempts` | `GET /api/v1/learning/activity-attempts` | learning.v1 | `game_player` (self) | Có sẵn |
| `getActivityAttempt` | `GET /api/v1/learning/activity-attempts/{attempt_id}` | learning.v1 | `game_player` (owner) | Có sẵn |
| `submitActivityResponse` | `POST /api/v1/learning/activity-attempts/{attempt_id}/responses` | learning.v1 | `game_player` (owner) | Có sẵn (`ActivityResponseSubmission`, `ActivityResponseResult`) |
| `pauseActivityAttempt` | `POST /api/v1/learning/activity-attempts/{attempt_id}:pause` | learning.v1 | `game_player` (owner) | Có sẵn |
| `submitActivityAttempt` | `POST /api/v1/learning/activity-attempts/{attempt_id}:submit` | learning.v1 | `game_player` (owner) | Có sẵn |
| `getActivityAttemptReview` | `GET /api/v1/learning/activity-attempts/{attempt_id}/review` | learning.v1 | `game_player` (owner) | Có sẵn (`AttemptReview`) |
| `listActivityTypes` | `GET /api/v1/cms/learning/activity-types` | learning.v1 | `learning_content.read` | Có sẵn |
| `listCmsActivities`, `getCmsActivity` | `GET /api/v1/cms/learning/activities[/{activity_id}]` | learning.v1 | `learning_content.read` | Có sẵn |
| `createCmsActivity`, `updateCmsActivityDraft` | `POST /api/v1/cms/learning/activities`, `PATCH …/{activity_id}` | learning.v1 | `learning_content.activity.write` | Có sẵn |

ProblemCode: `ATTEMPT_NOT_FOUND`, `ATTEMPT_NOT_IN_PROGRESS`, `ATTEMPT_ALREADY_SUBMITTED`, `ATTEMPT_DEADLINE_PASSED`, `ATTEMPT_EXPIRED`, `ANSWER_LIMIT_REACHED`, `INVALID_RESPONSE`, `QUESTION_POOL_INSUFFICIENT`, `ACTIVITY_TYPE_NOT_SUPPORTED`, `ACTIVITY_CONFIG_INVALID`, `CONTENT_NOT_PUBLISHED`, `GAME_ACCESS_STUDENT_ONLY`, `IDEMPOTENCY_KEY_REQUIRED`, `RATE_LIMITED`, `VALIDATION_FAILED`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `createAttempt` | module game → module learning | `specs/openapi/modules/learning.internal.v1.yaml` | `game` | Có sẵn (`AttemptCreateRequest`, `AttemptCreated`) |
| `getAttemptResult` | module game → module learning | learning.internal.v1 | `game` | Có sẵn (`AttemptResult`) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có (attempt dùng REST) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `activity.submitted` | learning / game (quest, XP), learning (mastery F031), LMS | `specs/events/payloads/learning/activity.submitted.v1.schema.json` | Có sẵn |
| `activity.graded` | learning / learning (mastery), LMS, NTF, game | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Có sẵn |
| `activity.started` | learning / LA, LMS | `specs/events/payloads/learning/activity.started.v1.schema.json` (`recommendation_id?`) | Cần thêm (T052) |
| `activity.abandoned` | learning / LA, LMS | `…/activity.abandoned.v1.schema.json` | Cần thêm |
| learning event `activity.started`, `question.answered`, `activity.submitted`, `activity.graded` | ghi `learning.learning_events` | `specs/schemas/learning-event.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/activities/activity-base.schema.json` | Trường chung activity | Có sẵn |
| `specs/schemas/activities/quiz.schema.json` | `ConfigSchema()` plugin `quiz` | Có sẵn |
| `specs/schemas/activities/interactive-exercise.schema.json` | `ConfigSchema()` plugin `interactive_exercise` | Có sẵn |
| `specs/schemas/question.schema.json` (`$defs.<type>.response`) | Validate `response` (`INVALID_RESPONSE`) | Có sẵn (S1 của Bước 0) |
| `specs/schemas/common/enums.schema.json` | `ActivityType`, `AttemptMode`, `AttemptStatus`, `FeedbackTiming` | Có sẵn |

## 6. Delta cần thực hiện

- Payload `activity.abandoned.v1.schema.json` (`attempt_id`, `student_id`, `mode`, `reason` ∈ {`expired`,`user`,`superseded`}) theo EVENT_DESIGN (RB-206) (task T005).
- Domain event `activity.started` (`specs/events/payloads/learning/activity.started.v1.schema.json`, trường theo EVENT_DESIGN §4.3 + `recommendation_id?` cho F032) và `activity.abandoned`: Cần thêm (task T052). F030 là owner duy nhất của domain event `activity.*`; feature khác (F032, F041, F065, F066) chỉ tiêu thụ hoặc phát qua engine.
- `specs/schemas/learning-event.schema.json`: đổi `exercise.started`/`exercise.submitted` (enum `event_type`, các nhánh `if`, mô tả) sang `activity.started`/`activity.submitted`, thêm `activity.abandoned` (RB-206) (task T052).
- `learning.internal.v1.yaml` `AttemptCreateRequest.context.context_type`: thêm `challenge_attempt` (F076) và `boss_encounter` (+ `phase`, F103) cho mode `challenge` [Confirmed] (task T052).
- `ActivityTypeInfo`/registry: bảo đảm `min_client_version` bắt buộc với loại `enabled` (task T004).
- `submitActivityAttempt`, `submitActivityResponse`: liệt kê `410` (`ATTEMPT_EXPIRED`) và `428` trong responses (task T004).
- DATABASE_DESIGN §3.4: danh sách `context_type` (gồm `challenge_attempt`, `boss_encounter`), cột mới của `attempts`/`attempt_items`/`attempt_answers` (task T007).
