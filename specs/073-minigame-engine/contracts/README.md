# Contracts: F073 — Mini-game Engine: `MiniGameConfig`, plugin `minigame`, scene Godot 11 loại

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (sinh `specs/openapi/modules/game.v1.yaml`, `specs/openapi/modules/learning.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `startMiniGame` | `POST /api/v1/game/minigames/{minigame_config_id}:start` (`Idempotency-Key`, body `{context_type, context_id?}`) | eduworld-api.v1.yaml | `game_player` | Cần sửa (`context_type` bỏ `assignment`; `MiniGameSession.minigame_type` `drag_drop` → `drag_and_drop`; thêm `deadline_at`, `server_time`, `minigame_config_version_id`; `403`, `409`, `422`, `503`) |
| `startQuestActivity` | `POST /api/v1/game/quests/{quest_id}/steps/{step_id}/activities:start` | eduworld-api.v1.yaml | `game_player` | Có sẵn (F072; bước `minigame` trả `Attempt`) |
| `submitActivityResponse` | `POST /api/v1/learning/activity-attempts/{attempt_id}/responses` | eduworld-api.v1.yaml | `game_player` | Cần sửa (`ActivityResponseResult` thêm `game_points`, `streak` cho plugin `minigame`) |
| `submitActivityAttempt` | `POST /api/v1/learning/activity-attempts/{attempt_id}:submit` | eduworld-api.v1.yaml | `game_player` | Cần sửa (kết quả thêm `game_score`, `completion_met` khi `activity_type = minigame`) |
| `startActivityAttempt` | `POST /api/v1/learning/activities/{activity_id}/attempts` | eduworld-api.v1.yaml | `game_player` | Có sẵn (activity `minigame`, F041/F063 dùng) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `createAttempt` | game → learning | `specs/openapi/modules/learning.internal.v1.yaml` | `[game]` | Cần sửa (`ActivityRef.content_type` thêm `minigame_config`: `content_id = minigame_config_id`, `content_version_id` = version ghim) |
| `validateMinigameQuestionSource` | game → learning (chỉ khi publish) | `specs/openapi/modules/learning.internal.v1.yaml` — operation facade `validateMinigameQuestionSource` (input `minigame_type`, `question_source`, `question_count`; trả `compatible`, `question_types[]`, `available_count`, `reasons[]`) | `[game]` | Cần thêm |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Mini-game dùng REST (không realtime ở Release) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `content.published` (`item_type = minigame_config`) | game → learning (projection), AUD | `specs/events/payloads/game/content.published.v1.schema.json` (khối `minigame_config`) | Cần sửa (owner F071) |
| `minigame.completed` | game → game (F072 bước, F074 reward), analytics | `specs/events/payloads/game/minigame.completed.v1.schema.json` | Cần thêm |
| `activity.graded` | learning → game | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Cần sửa (khối `minigame`; owner F030) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/minigame-config.schema.json` | Payload `MiniGameConfigVersion` | Có sẵn |
| `specs/schemas/activities/minigame.schema.json` | Activity `minigame` (bọc config, `question_set_override`) | Có sẵn |
| `specs/schemas/common/enums.schema.json` | `QuestionType`, `FeedbackTiming`, `ActivityType` | Có sẵn |

## 6. Delta cần thực hiện

- `eduworld-api.v1.yaml`: `startMiniGame` (enum `context_type` = `free_play`, `quest_step`; lỗi); `MiniGameSession` (`minigame_type` theo `minigame-config.schema.json`, `deadline_at`, `server_time`, `minigame_config_version_id`; `config` thay `additionalProperties: true` bằng schema hiển thị đóng không có đáp án); `ActivityResponseResult` thêm `game_points`, `streak`; kết quả submit thêm `game_score`, `completion_met`.
- `learning.internal.v1.yaml`: thêm `validateMinigameQuestionSource` (`x-allowed-callers: [game]`, mọi chuỗi có `maxLength`); `ActivityRef.content_type` thêm `minigame_config`.
- Payload event: thêm `minigame.completed`; đề xuất khối `minigame_config` cho `content.published` (game, owner F071); sửa `activity.graded` (khối `minigame` tùy chọn).
- EVENT_DESIGN §3.2 ghi learning tiêu thụ `minigame.completed` để ghi evidence — trùng với evidence từ attempt; đề xuất bỏ (task Polish).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make gen MOD=learning`, `make test-contract`.
