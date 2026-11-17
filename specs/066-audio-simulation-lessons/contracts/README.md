# Contracts: F066 — Bài nghe và mô phỏng: plugin `audio_lesson`, `simulation` + checker Go, template Godot

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (bundle) và `specs/openapi/modules/learning.v1.yaml`. Plugin cắm vào handler của F030; không thêm route.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `startActivityAttempt` | `POST /api/v1/learning/activities/{activity_id}/attempts` | eduworld-api | `game_player` | Có sẵn (`MEDIA_NOT_READY`) |
| `getActivityAttempt` | `GET /api/v1/learning/activity-attempts/{attempt_id}` | eduworld-api | `game_player` + owner | Có sẵn |
| `reportActivityProgress` | `POST /api/v1/learning/activity-attempts/{attempt_id}/progress` | eduworld-api | `game_player` + owner | Cần sửa (§6: `play_started`, `plays_used`, `max_plays`, `AUDIO_PLAY_LIMIT_REACHED`) |
| `submitActivityResponse` | `POST /api/v1/learning/activity-attempts/{attempt_id}/responses` | eduworld-api | `game_player` + owner | Có sẵn (`kind = question` cho bài nghe, `simulation_question` cho mô phỏng) |
| `revealNextHint` | `POST /api/v1/learning/activity-attempts/{attempt_id}/items/{position}/hints:next` | eduworld-api | `game_player` + owner | Có sẵn |
| `submitSimulationRun` | `POST /api/v1/learning/activity-attempts/{attempt_id}/simulation-runs` | eduworld-api | `game_player` + owner | Cần sửa (§6: thêm `401`/`403`/`404`, `SimulationRunResult` mở rộng, `SIMULATION_RUN_LIMIT_REACHED`) |
| `submitActivityAttempt` | `POST /api/v1/learning/activity-attempts/{attempt_id}:submit` | eduworld-api | `game_player` + owner | Có sẵn |
| `listActivityTypes` | `GET /api/v1/cms/learning/activity-types` | eduworld-api | `learning_content.read` | Có sẵn (thêm bản ghi `audio_lesson`, `simulation`) |
| `listSimulationTemplates` | `GET /api/v1/cms/learning/simulation-templates` | eduworld-api | `learning_content.read` | Cần sửa (§6: `SimulationTemplate` thêm `actions`, `metrics`, `status`, `simulation_kind` thống nhất enum) |
| `createCmsActivity` · `updateCmsActivityDraft` · `getCmsActivity` | `POST /api/v1/cms/learning/activities` · `PATCH`/`GET …/{activity_id}` | eduworld-api | `learning_content.activity.write` / `learning_content.read` | Có sẵn (`ValidateConfig` → `ACTIVITY_CONFIG_INVALID`, `ACTIVITY_TYPE_NOT_SUPPORTED`) |
| `listMediaAssets` · `presignMediaUpload` · `getMediaAsset` | `GET /api/v1/cms/media` · `POST /api/v1/cms/media:presign-upload` · `GET /api/v1/cms/media/{asset_id}` | eduworld-api | `learning_content.read` / `media.upload` | Có sẵn (F027) |

Schema dùng: `ActivityRender`, `ActivityProgressReport`, `ActivityProgressState`, `ActivityResponseSubmission`, `ActivityResponseResult`, `SimulationRunInput`, `SimulationRunResult`, `SimulationTemplate`, `ActivityTypeInfo`, `EvaluationStrategy`, `Attempt`, `Problem` (`MEDIA_NOT_READY`, `SIMULATION_LOG_INVALID`, `ACTIVITY_CONFIG_INVALID`, `ACTIVITY_TYPE_NOT_SUPPORTED`, `ATTEMPT_ALREADY_SUBMITTED`, `VALIDATION_FAILED`, `GAME_ACCESS_STUDENT_ONLY`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `createAttempt` (start theo quest/challenge) | module game → module learning (facade `learning`) | `specs/openapi/modules/learning.internal.v1.yaml` | `game` | Có sẵn — plugin dùng chung, không đổi |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `simulation.run_verified` | module learning phát; LR, AUD tiêu thụ | `specs/events/payloads/learning/simulation.run_verified.v1.schema.json` | Cần thêm |
| `simulation.run_rejected` | module learning phát; LR, AUD (integrity) tiêu thụ | `specs/events/payloads/learning/simulation.run_rejected.v1.schema.json` | Cần thêm |
| `activity.submitted`, `activity.graded` | module learning phát qua engine F030 (owner F030) | `specs/events/payloads/learning/activity.submitted.v1.schema.json`, `activity.graded.v1.schema.json` | Có sẵn |
| learning event `simulation.goal_met`, `question.answered`, `activity.started`, `activity.completed` | module learning ghi (F031) | `specs/schemas/learning-event.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/activities/audio-lesson.schema.json` | `ValidateConfig` bài nghe | Có sẵn; Cần thêm `$defs.render` (§6) |
| `specs/schemas/activities/simulation.schema.json` | `ValidateConfig` mô phỏng | Có sẵn; Cần thêm `$defs.render` (§6) |
| `specs/schemas/activities/activity-base.schema.json` (`completionRule` mode `min_watched_ratio`, `goals_met`) | Quy tắc hoàn thành | Có sẵn |
| `specs/schemas/question.schema.json` | Câu nghe hiểu/kết luận | Có sẵn |
| `specs/examples/simulation/physics.lever/v1/vectors.json` | Vector kiểm thử chung Go + GdUnit4 | Cần thêm (research R4) |

## 6. Delta cần thực hiện

Phase 2 task T004–T007.

- **Cần sửa** `ActivityProgressReport`: thêm `play_started: boolean` (default false). `ActivityProgressState`: thêm `plays_used: integer ≥ 0`, `max_plays: integer 1..20 | null`. `reportActivityProgress` thêm response `422` mô tả `AUDIO_PLAY_LIMIT_REACHED`. Task T004.
- **Cần thêm** `ProblemCode`: `AUDIO_PLAY_LIMIT_REACHED`, `SIMULATION_RUN_LIMIT_REACHED` (nhóm "Activity engine") ở mọi file liệt kê enum ProblemCode (bundle + `modules/*.v1.yaml`). Task T004.
- **Cần sửa** `submitSimulationRun`: thêm response `401`, `403` (`GAME_ACCESS_STUDENT_ONLY`), `404`, `428` (thiếu `Idempotency-Key`). `SimulationRunResult`: thêm `verification_status` enum `verified`/`rejected`/`not_verifiable`, `goals_met[]` (maxItems 20) `{goal_id maxLength 64, met boolean}`, `runs_used integer`, `max_runs integer | null`; `feedback` giữ `maxLength 1000`. Task T004.
- **Cần sửa** `SimulationTemplate`: `kind` → `simulation_kind` dùng đúng enum của `simulation.schema.json` (`science_simulation`, `physics`, `geometry_3d`, `virtual_lab`, `interactive_experiment`) — hiện lệch (`science`, `experiment`); thêm `actions[]` (maxItems 50) `{action maxLength 64, args_schema object}`, `metrics[]` (maxItems 50) `{metric maxLength 64, unit maxLength 16, display boolean}`, `checker_available boolean`, `status` enum `active`/`retired`. Task T004.
- **Cần thêm** `$defs.render` trong `audio-lesson.schema.json` (`audio`, `segments`, `questions[] {position, at_ms}`, `interaction`, `plays_used`, `resume_position_ms`, `transcript_asset_id` khi `show_transcript = always`) và `simulation.schema.json` (`template_code`, `template_version`, `simulation_kind`, `parameters`, `goals[] {goal_id, description}` — **không** `check`, `steps`, `interaction`, `seed`, `runs_used`, `questions[] {position}`); `additionalProperties: false`. Task T005.
- **Cần thêm** event payload `simulation.run_verified.v1` / `simulation.run_rejected.v1` (`run_id`, `attempt_id`, `student_id`, `template_code` maxLength 64, `template_version`, `goals_met[]`, `verification_status`, `reject_code?` maxLength 64) theo EVENT_DESIGN §4. Task T006.
- **Cần thêm** `specs/examples/simulation/physics.lever/v1/vectors.json` (≥ 20 vector: cân bằng, lệch, log rỗng, vật ngoài ô) và example `activity.audio-lesson.*.json`, `activity.simulation.physics-lever.json`. Task T007.
- Phụ thuộc delta của F065: `ActivityRender.media[].variant` (dùng giá trị `ogg_vorbis`).
