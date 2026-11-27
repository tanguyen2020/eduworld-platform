# Contracts: F110 — Godot — màn boss co-op, thẻ dữ kiện bước `interdependent`, danh sách bạn bè

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Client chỉ tiêu thụ contract; không sở hữu contract nào. Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (sinh `game.v1.yaml`, `chat.v1.yaml`, `competition.v1.yaml`), `specs/asyncapi/eduworld-realtime.v1.yaml`. Mọi operation: middleware `game_player`, header `X-Tenant-Id`.

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listBosses` | `GET /api/v1/game/bosses?area_id=` | game.v1.yaml | `game_player` | Có sẵn |
| `createBossEncounter` | `POST /api/v1/game/bosses/{boss_id}/encounters` (`Idempotency-Key`, `mode = solo`) | game.v1.yaml | `game_player` | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần sửa (F103: chỉ `solo`, `422`)) |
| `getBossEncounter` | `GET /api/v1/game/boss-encounters/{encounter_id}` | game.v1.yaml | thành viên encounter | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần sửa (F103: `state_version`, `deadline_at`, `phase_retries_left`, `outcome`, `my_questions`)) |
| `leaveBossEncounter` | `POST /api/v1/game/boss-encounters/{encounter_id}:leave` (`Idempotency-Key`) | game.v1.yaml | thành viên encounter | Có sẵn |
| `getQuest`, `startQuestActivity` | `/api/v1/game/quests/{quest_id}…` | game.v1.yaml | `game_player` | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần sửa (F103: `step_type` `interdependent`/`boss`, `QuestProgress.party_progress`)) |
| `getQuestStepClues` | `GET /api/v1/game/quests/{quest_id}/steps/{step_id}/clues` | game.v1.yaml | thành viên phiên | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần thêm (F103)) |
| `shareQuestStepClue` | `POST …/steps/{step_id}/clues/{role}:share` (`Idempotency-Key`) | game.v1.yaml | người giữ vai | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần thêm (F103)) |
| `assignQuestStepRoles` | `PUT …/steps/{step_id}/roles` (`Idempotency-Key`) | game.v1.yaml | leader party | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần thêm (F103)) |
| `listFriends`, `removeFriend` | `GET /api/v1/social/friends`, `DELETE …/friends/{user_id}` | chat.v1.yaml | `game_player` | Có sẵn (F095; enum `presence` thống nhất ở F091) |
| `listFriendRequests`, `sendFriendRequest`, `acceptFriendRequest`, `rejectFriendRequest` | `/api/v1/social/friend-requests*` | chat.v1.yaml | `game_player` | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Có sẵn / Cần sửa (F095: mã lỗi)) |
| `createFriendCode`, `redeemFriendCode` | `/api/v1/social/friend-codes*` | chat.v1.yaml | `game_player` | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Có sẵn / Cần sửa (F095: `404 FRIEND_CODE_INVALID`)) |
| `blockUser`, `unblockUser`, `listBlocks`, `muteUser`, `unmuteUser`, `listMutes` | `/api/v1/social/blocks*`, `/api/v1/social/mutes*` | chat.v1.yaml | `game_player` | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Có sẵn / Cần sửa (F095: `403`, `422`)) |
| `createParty`, `getMyParty`, `inviteToParty` | `/api/v1/multiplayer/parties*` (qua `PartyService` F102) | competition.v1.yaml | `game_player` | Có sẵn (F100) |
| `createReport` | `POST /api/v1/chat/reports` (qua `ReportDialog` F119) | chat.v1.yaml | `game_player` | Có sẵn (F121) |

`ProblemCode` hiển thị: `CHALLENGE_NOT_ELIGIBLE`, `CHALLENGE_ATTEMPT_LIMIT`, `BOSS_ENCOUNTER_FULL`, `BOSS_PHASE_CLOSED`, `ROOM_NOT_MEMBER`, `PARTY_NOT_LEADER`, `PERMISSION_DENIED`, `SOCIAL_SCOPE_VIOLATION`, `FRIEND_LIMIT_REACHED`, `FRIEND_CODE_INVALID`, `RATE_LIMITED`, `NOT_FOUND`, `VALIDATION_FAILED`, `GAME_ACCESS_STUDENT_ONLY`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi facade nội bộ giữa module | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `boss.encounter.ready`, `boss.answer.submit` | client → game | Có sẵn |
| `boss.encounter.updated`, `boss.phase.started`, `boss.phase.completed` | game → thành viên | Có sẵn |
| `boss.encounter.ended` | game → thành viên | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần sửa (F103: `outcome` thống nhất với event)) |
| `boss.question.assigned`, `boss.answer.evaluated` | game → một thành viên | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần thêm (F103)) |
| `quest.progress.updated` (`party_progress`) | game → thành viên | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần sửa (F103)) |
| `quest.roles.assigned`, `quest.clue.shared` | game → thành viên phiên | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần thêm (F103)) |
| `party.session.started` (`room_type = boss`), `party.session.ended` | competition → thành viên | Có sẵn trên main (đối soát 2026-11-28, §6; trước: Cần sửa (F101: `room_type` thêm `boss`)) |
| `presence.friend.updated` | chat → bạn bè | Có sẵn (enum `status` thống nhất — delta F091) |
| `presence.status.set` | client → gateway | Có sẵn |
| `system.command.acked`, `system.command.failed`, `session.session.replaced` | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không dùng NATS (R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` (phần render) | Parse `QuestionRender` trong `boss.question.assigned` (renderer F063) | Có sẵn |

## 6. Delta cần thực hiện

Không có delta do F110 sở hữu. F110 phụ thuộc delta của:

- **F103** (module game): `createBossEncounter` chỉ `solo`; `BossEncounter` thêm `state_version`, `deadline_at`, `phase_retries_left`, `outcome`, `my_questions`; `getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles`; AsyncAPI `boss.question.assigned`, `boss.answer.evaluated`, `quest.roles.assigned`, `quest.clue.shared`, `quest.progress.updated.party_progress`, `BossEncounterEnded.outcome`.
- **F101**: `party.session.started.room_type` thêm `boss`. **F095**: mã lỗi social, `404 FRIEND_CODE_INVALID`. **F091**: enum presence chung (MULTIPLAYER_GAMEPLAY §5) giữa `PlayerCard.presence` và `presence.friend.updated.status`; ẩn trạng thái là cờ `hidden` của `presence.status.set`.
- **Đề xuất mới [Confirmed] (owner F095)**: `PlayerCard` thêm `title` (`LocalizedName`, danh hiệu) để hiển thị trong danh sách bạn như wireframe S11; khi chưa có, client ẩn danh hiệu.

**Trạng thái trên main (đối soát 2026-11-28, owner "chốt hết" — [Confirmed]):**

| Delta | Owner | Trạng thái trên main | Bằng chứng / ghi chú |
|---|---|---|---|
| `createBossEncounter` chỉ `solo` (`422` cho `party`); `BossEncounter.state_version`, `deadline_at`, `phase_retries_left`, `outcome`, `my_questions` | F103 | Đã có | `game.v1` `createBossEncounter`, schema `BossEncounter`; `modules/game/internal/application/partyrules/views.go` |
| `getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles` | F103 | Đã có | `game.v1`; `query/party_activities.go`, `command/party_quest_roles.go` |
| AsyncAPI `boss.question.assigned`, `boss.answer.evaluated`, `quest.roles.assigned`, `quest.clue.shared`, `quest.progress.updated.party_progress`, `boss.encounter.ended.outcome` | F103 | Đã có | `eduworld-realtime.v1.yaml`; `command/boss.go`, `command/party_quest.go` |
| `party.session.started.room_type` có `boss` | F101 | Đã có | AsyncAPI `PartySessionStarted`; `competition/internal/domain/party/activity.go` `RoomBoss` |
| Mã lỗi social, `404 FRIEND_CODE_INVALID`, `403`/`422` của chặn/tắt tiếng | F095 | Đã có | `chat.v1` `redeemFriendCode`, `blockUser`…`unmuteUser` |
| Enum presence chung `PlayerCard.presence` = `presence.friend.updated.status`; ẩn trạng thái = cờ `hidden` của `presence.status.set` | F091 | Đã có | AsyncAPI `PresenceStatus`, `PresenceStatusSet.hidden`; `modules/realtime/internal/presence` |
| `PlayerCard.title` (`LocalizedName`, danh hiệu level) | F095 (+F074 `level.reached.title`) | **Làm 2026-11-28** | `chat.v1` `PlayerCard.title`; consumer `chat.projection_game` nhận `level.reached`; migration `20270203000001_f110_f111_social_deltas.sql`; test `TestF095PlayerCardTitle`. Vắng tới khi học sinh lên một level có danh hiệu → client ẩn |
| Fixture Godot `src/game/tests/fixtures/{boss,coop_quest,social}` | Agent Godot | **Chưa** (không thuộc lượt server) | Sinh lại từ example OpenAPI/AsyncAPI sau khi merge |

- Fixture `src/game/tests/fixtures/{boss,coop_quest,social}/*.json` sinh lại từ example sau khi delta merge; test contract parse mọi example ([GODOT_ARCHITECTURE §12](../../../docs/08-game-client/GODOT_ARCHITECTURE.md)).
- Kiểm (owner backend): `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make gen MOD=chat`.
