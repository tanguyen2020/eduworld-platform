# Contracts: F063 — Activity player: renderer registry, renderer 11 loại câu hỏi, gợi ý, kết quả, công thức SVG

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (bundle) và `specs/openapi/modules/learning.v1.yaml`. Mọi route người chơi qua middleware `game_player` + kiểm tra attempt thuộc học sinh (404 nếu không).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `startActivityAttempt` | `POST /api/v1/learning/activities/{activity_id}/attempts` | eduworld-api | `game_player` | Có sẵn |
| `startAdHocAttempt` | `POST /api/v1/learning/activity-attempts` | eduworld-api | `game_player` | Có sẵn (nút luyện tập từ gợi ý) |
| `getActivityAttempt` | `GET /api/v1/learning/activity-attempts/{attempt_id}` | eduworld-api | `game_player` + owner | Có sẵn |
| `submitActivityResponse` | `POST /api/v1/learning/activity-attempts/{attempt_id}/responses` | eduworld-api | `game_player` + owner | Có sẵn |
| `revealNextHint` | `POST /api/v1/learning/activity-attempts/{attempt_id}/items/{position}/hints:next` | eduworld-api | `game_player` + owner | Có sẵn |
| `pauseActivityAttempt` | `POST /api/v1/learning/activity-attempts/{attempt_id}:pause` | eduworld-api | `game_player` + owner | Có sẵn |
| `submitActivityAttempt` | `POST /api/v1/learning/activity-attempts/{attempt_id}:submit` | eduworld-api | `game_player` + owner | Có sẵn |
| `getActivityAttemptReview` | `GET /api/v1/learning/activity-attempts/{attempt_id}/review` | eduworld-api | `game_player` + owner | Có sẵn |
| `getLesson` | `GET /api/v1/learning/lessons/{lesson_id}` | eduworld-api | `game_player` | Có sẵn (worked example) |
| `getMyProgress` | `GET /api/v1/learning/progress/me` | eduworld-api | `game_player` | Có sẵn (mức mastery LO) |
| `getContentManifest` | `GET /api/v1/game/content-manifest` | eduworld-api | `game_player` | Đã sửa (§6) |
| `postTelemetry` | `POST /api/v1/game/telemetry` | eduworld-api | `game_player` | Có sẵn |

Schema dùng: `Attempt`, `QuestionRender`, `ActivityRender`, `MathAssetRef`, `ActivityAttemptStart`, `AdHocAttemptStart`, `ActivityResponseSubmission`, `QuestionResponse` (11 loại), `ActivityResponseResult`, `AnswerFeedback`, `HintResult`, `AttemptResult`, `AttemptReview`, `Problem` (`ProblemCode`: `ATTEMPT_ALREADY_SUBMITTED`, `ATTEMPT_EXPIRED`, `ATTEMPT_NOT_IN_PROGRESS`, `INVALID_RESPONSE`, `HINTS_NOT_ALLOWED`, `HINTS_EXHAUSTED`, `ASSIGNMENT_NOT_OPEN`, `ASSIGNMENT_MAX_ATTEMPTS`, `ACTIVITY_TYPE_NOT_SUPPORTED`, `GAME_ACCESS_STUDENT_ONLY`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi internal API | | | |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng ở F063 | |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không phát/tiêu thụ | | |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` (`$defs.<type>.render_payload`, `$defs.response`) | Sinh fixture, kiểm answer model | Có sẵn (đóng chặt render + response theo loại: Bước 0 S1, S4) |
| `specs/schemas/activities/quiz.schema.json`, `interactive-exercise.schema.json` | Fixture activity | Có sẵn |

## 6. Delta cần thực hiện

- **Đã thêm (T004, 2026-11-20)** `QuestionRender.hint_count` (`integer`, 0–5): số cấp gợi ý của item, không lộ nội dung (GODOT_ARCHITECTURE §6.2). Task T004.
- **Đã thêm (T004, 2026-11-20)** `ContentManifest.activity_min_client_versions` (object, key = `ActivityType`, value `string maxLength 32`, SemVer) cho FR-EXR-015. Task T004.
- **Đã thêm (T004, 2026-11-20) [Proposed]** `AttemptReview.recommendations[].recommendation_id` (tùy chọn) cho F063-AC-24 (spec Clarifications 2026-11-20), cùng `example` cho `Attempt` (11 loại câu), `ActivityResponseResult`, `HintResult`, `AttemptReview` (Prism MOCK + fixture game).
- Phụ thuộc Bước 0 (không làm ở đây): S1 response schema theo loại câu, S3 hints API/`hints_used`, S4 đóng chặt `render_payload`.
