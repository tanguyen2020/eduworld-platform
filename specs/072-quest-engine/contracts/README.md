# Contracts: F072 — Quest Engine: 12 loại quest, bước, tiến độ qua event, lọc theo khối

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (sinh `modules/game.v1.yaml`). Mọi route `game_player`, header `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listQuests` | `GET /api/v1/game/quests?quest_type=&status=&area_id=&cursor=` | eduworld-api.v1.yaml | `game_player` | Cần sửa (thêm `area_id`; enum `QuestType` 12 loại; `status`) |
| `getQuest` | `GET /api/v1/game/quests/{quest_id}` | eduworld-api.v1.yaml | `game_player` | Cần sửa (`QuestDetail.steps[]`: `step_type` theo schema v2, thêm `hint`, `optional`, `depends_on`, `count`, `area_id`, `npc_id`; `narrative`, `giver_npc_id`, `area_id`) |
| `acceptQuest` | `POST /api/v1/game/quests/{quest_id}:accept` (`Idempotency-Key`, body `{party_id?}`) | eduworld-api.v1.yaml | `game_player` | Cần sửa (requestBody tùy chọn; `404`; `409` `QUEST_ACTIVE_LIMIT`/`QUEST_ALREADY_COMPLETED`/`QUEST_COOLDOWN`; `403` `QUEST_PREREQUISITE_NOT_MET`) |
| `abandonQuest` | `POST /api/v1/game/quests/{quest_id}:abandon` (`Idempotency-Key`) | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `reportQuestStep` | `POST /api/v1/game/quests/{quest_id}/steps/{step_id}:report` (`Idempotency-Key`, `QuestStepReport`) | eduworld-api.v1.yaml | `game_player` | Có sẵn |
| `startQuestActivity` | `POST /api/v1/game/quests/{quest_id}/steps/{step_id}/activities:start` (`Idempotency-Key`) | eduworld-api.v1.yaml | `game_player` | Cần sửa (`409` `QUEST_STEP_ATTEMPT_LIMIT`, `503`) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `createAttempt` | game → learning | `specs/openapi/modules/learning.internal.v1.yaml` | `[game]` | Cần sửa (`ActivityRef.content_type` thêm `activity` với `content_id = activity_id`, `content_version_id`) |
| `getAttemptResult` | game → learning (đối soát) | learning.internal.v1.yaml | `[game]` | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `quest.progress.updated` | server → học sinh (Valkey `ew:rt:user:<id>`) | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `quest.accepted`, `quest.step_completed`, `quest.abandoned` | game → analytics, RT | `specs/events/payloads/game/quest.*.v1.schema.json` | Cần thêm |
| `quest.completed` | game → game (F071 unlock/chương, F074 reward), learning (learning event), analytics | `specs/events/payloads/game/quest.completed.v1.schema.json` | Cần thêm |
| `activity.graded` | learning → game | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Có sẵn, owner F030 (dùng `attempt_id`, `correct_count`, `item_count`, `passed`; tra bước qua `game.quest_step_attempts`) |
| `world.discovery_found` | game (F071) → game | `specs/events/payloads/game/world.discovery_found.v1.schema.json` | Cần thêm (F071) |
| `game_session.started` | game (F014) → game | `specs/events/payloads/game/game_session.started.v1.schema.json` | Cần thêm (F014) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/quest.schema.json` (v2) | Payload `QuestVersion` | Cần sửa (`content_ref.kind` thêm `activity`; `start_trigger.interactable_ids[]` ≤ 20 cho `discovery`) |

## 6. Delta cần thực hiện

- `eduworld-api.v1.yaml`: `QuestType` = 12 giá trị của `quest.schema.json`; `QuestProgressStatus` = `active`, `completed`, `expired`, `abandoned`, `failed` và `QuestSummary.status` thêm `available`; `QuestProgress.step_states` thành object `{status: locked|active|completed, count?, target?}`; `QuestDetail.steps[]` bổ sung trường; `listQuests` thêm `area_id`; `acceptQuest` body tùy chọn `{party_id}`; khai báo lỗi.
- `ProblemCode` thêm `QUEST_PREREQUISITE_NOT_MET`, `QUEST_ACTIVE_LIMIT`, `QUEST_STEP_ATTEMPT_LIMIT`.
- `learning.internal.v1.yaml` `ActivityRef.content_type` thêm `activity` (FR-CNT-013); `lesson` của quest chưa hỗ trợ (validator từ chối) — ghi chú trong spec.
- `quest.schema.json`: `content_ref.kind` thêm `activity`; `start_trigger.interactable_ids`.
- Payload event `quest.accepted`, `quest.step_completed`, `quest.completed`, `quest.abandoned`.
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make gen MOD=learning`, `make test-contract`.
