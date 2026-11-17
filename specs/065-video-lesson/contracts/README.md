# Contracts: F065 — Bài học video: plugin `video_lesson` và renderer Godot

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (bundle) và `specs/openapi/modules/learning.v1.yaml`. Không thêm endpoint; plugin cắm vào handler của F030.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `startActivityAttempt` | `POST /api/v1/learning/activities/{activity_id}/attempts` | eduworld-api | `game_player` | Có sẵn (`VIDEO_NOT_READY`) |
| `getActivityAttempt` | `GET /api/v1/learning/activity-attempts/{attempt_id}` | eduworld-api | `game_player` + owner | Có sẵn |
| `reportActivityProgress` | `POST /api/v1/learning/activity-attempts/{attempt_id}/progress` | eduworld-api | `game_player` + owner | Cần sửa (§6: `watched_ranges` trong `ActivityProgressState`) |
| `submitActivityResponse` | `POST /api/v1/learning/activity-attempts/{attempt_id}/responses` | eduworld-api | `game_player` + owner | Có sẵn (`kind = cue_answer | branch_choice`, `next_segment_id`) |
| `revealNextHint` | `POST /api/v1/learning/activity-attempts/{attempt_id}/items/{position}/hints:next` | eduworld-api | `game_player` + owner | Có sẵn (cue là item) |
| `submitActivityAttempt` | `POST /api/v1/learning/activity-attempts/{attempt_id}:submit` | eduworld-api | `game_player` + owner | Có sẵn (`CUE_ANSWER_REQUIRED`) |
| `listActivityTypes` | `GET /api/v1/cms/learning/activity-types` | eduworld-api | `learning_content.read` | Có sẵn (thêm bản ghi `video_lesson`) |
| `createCmsActivity` / `updateCmsActivityDraft` | `POST /api/v1/cms/learning/activities`, `PATCH /api/v1/cms/learning/activities/{activity_id}` | eduworld-api | `learning_content.activity.write` | Có sẵn (gọi `ValidateConfig` → `ACTIVITY_CONFIG_INVALID`) |

Schema dùng: `ActivityRender`, `ActivityProgressReport`, `ActivityProgressState`, `ActivityResponseSubmission`, `ActivityResponseResult`, `Attempt`, `QuestionRender`, `Problem` (`VIDEO_NOT_READY`, `VIDEO_SEEK_NOT_ALLOWED`, `CUE_ANSWER_REQUIRED`, `ACTIVITY_CONFIG_INVALID`, `ATTEMPT_ALREADY_SUBMITTED`, `VALIDATION_FAILED`, `GAME_ACCESS_STUDENT_ONLY`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `createAttempt` (start quest/challenge) | module game → module learning (facade `learning`) | `specs/openapi/modules/learning.internal.v1.yaml` | `game` | Có sẵn — plugin dùng chung, không đổi |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `activity.started`, `activity.submitted`, `activity.graded` | module learning phát qua engine F030 (owner F030) | `specs/events/payloads/learning/activity.submitted.v1.schema.json`, `activity.graded.v1.schema.json` | Có sẵn |
| learning event `video.progressed`, `video.cue_answered`, `activity.started`, `activity.completed`, `question.answered` | module learning ghi (F031) | `specs/schemas/learning-event.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/activities/video-lesson.schema.json` | `ValidateConfig` | Có sẵn; Cần thêm `$defs.render` (§6) |
| `specs/schemas/activities/activity-base.schema.json` (`completionRule`) | Quy tắc hoàn thành | Có sẵn |
| `specs/schemas/question.schema.json` | Item câu chèn | Có sẵn |

## 6. Delta cần thực hiện

- **Cần thêm** `$defs.render` trong `specs/schemas/activities/video-lesson.schema.json` (đóng chặt, `additionalProperties: false`): `duration_ms`, `chapters[] {chapter_id, title, start_ms, end_ms}`, `cues[] {cue_id, at_ms, position, required, allow_skip_after_wrong}`, `branches[] {branch_point_id, at_ms, prompt, options[] {option_id, label}}` (không `target_segment_id`), `segments[] {segment_id, is_ending}`, `interaction`; `ActivityRender.render_payload` của `video_lesson` `$ref` tới đó (cùng hướng Bước 0 S4). Task T004.
- **Cần thêm** vào `ActivityRender.media[]`: `variant` (`string maxLength 32`, ví dụ `theora_480p`, `theora_720p`, `vtt`), `language` (`string maxLength 8`, phụ đề), `segment_id` (`string maxLength 64`). Task T004.
- **Cần thêm** `ActivityProgressState.watched_ranges` (`array maxItems 64` of `{start_ms, end_ms}`) để client khóa tua sau resume. Task T004.
- Không cần `ProblemCode` mới.
