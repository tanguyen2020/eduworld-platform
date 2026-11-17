# Contracts: F111 — Godot — club trong game, Event center, nhóm "Bạn cùng lớp"/"Vừa chơi cùng", nút "Tham gia bạn"

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Client chỉ tiêu thụ contract; không sở hữu contract nào. Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (sinh `game.v1.yaml`, `chat.v1.yaml`, `identity.v1.yaml`), `specs/asyncapi/eduworld-realtime.v1.yaml`. Mọi operation: middleware `game_player`, header `X-Tenant-Id`.

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listFriendSuggestions` | `GET /api/v1/social/friend-suggestions?source=classmates\|recently_played` (cursor) | chat.v1.yaml | `game_player` | Cần thêm (owner F095) |
| `sendFriendRequest` | `POST /api/v1/social/friend-requests` (qua `SocialService` F110) | chat.v1.yaml | `game_player` | Có sẵn (F095) |
| `enterWorldArea` | `POST /api/v1/game/world/areas/{area_id}:enter` (qua `SceneRouter.request_area` F071; `403 AREA_LOCKED`) | game.v1.yaml | `game_player` | Cần sửa (F071/F091) |
| `joinFriendInWorld` | `POST /api/v1/game/world/friends/{user_id}:join` | game.v1.yaml | `game_player` | **Không dùng** — Cần sửa (F071/F091: bỏ hoặc đổi nghĩa) |
| `listClubs` | `GET /api/v1/social/clubs?scope=mine\|discoverable` | chat.v1.yaml | học sinh | Có sẵn (Cần sửa F120: bỏ `study_group`) |
| `getClub`, `leaveClub` | `GET /api/v1/social/clubs/{club_id}`, `POST …/{club_id}:leave` | chat.v1.yaml | thành viên / học sinh cùng phạm vi | Có sẵn |
| `createClub` | `POST /api/v1/social/clubs` (`Idempotency-Key`; `403`, `409`, `422`, `451 CHAT_CONTENT_BLOCKED`) | chat.v1.yaml | học sinh theo `student_club_creation` | Có sẵn (Cần sửa F120: khai báo `451`) |
| `listClubMembers`, `listClubGoals`, `listClubEvents` | `GET /api/v1/social/clubs/{club_id}/members\|goals\|events` | chat.v1.yaml | thành viên | Có sẵn |
| `requestJoinClub` | `POST /api/v1/social/clubs/{club_id}/join-requests` (`Idempotency-Key`) | chat.v1.yaml | học sinh cùng phạm vi | Có sẵn |
| `listClubJoinRequests`, `acceptClubJoinRequest`, `rejectClubJoinRequest` | `GET …/{club_id}/join-requests`, `POST /api/v1/social/club-join-requests/{request_id}:accept\|:reject` | chat.v1.yaml | owner/officer | Có sẵn |
| `inviteToClub` | `POST /api/v1/social/clubs/{club_id}/invitations` (`invitee_ids` ≤ 50, `Idempotency-Key`) | chat.v1.yaml | owner/officer | Có sẵn |
| `listMyClubInvitations` | `GET /api/v1/social/club-invitations?status=pending` | chat.v1.yaml | invitee | Cần thêm (owner F120) |
| `acceptClubInvitation`, `declineClubInvitation` | `POST /api/v1/social/club-invitations/{invitation_id}:accept\|:decline` | chat.v1.yaml | invitee | Có sẵn |
| `listGameEvents`, `getGameEvent` | `GET /api/v1/game/events?status=live,upcoming`, `GET …/{event_id}` | game.v1.yaml | `game_player` | Có sẵn (F077) |
| `joinGameEvent` | `POST /api/v1/game/events/{event_id}:join` (`Idempotency-Key`) | game.v1.yaml | `game_player` | Có sẵn (Cần sửa F077: `428`) |
| `claimEventMilestone` | `POST /api/v1/game/events/{event_id}/milestones/{milestone_id}:claim` (`Idempotency-Key`) | game.v1.yaml | `game_player` | Có sẵn |
| `getCommunityGoal` | `GET /api/v1/game/community-goals/{goal_id}` | game.v1.yaml | `game_player` | Có sẵn |
| `getSeasonTrack`, `claimSeasonTier`, `listSeasonTrackHistory` | `GET /api/v1/game/season-track`, `POST …/tiers/{tier}:claim` (`Idempotency-Key`), `GET …/history` | game.v1.yaml | `game_player` | Có sẵn |
| `listQuests`, `getQuest`, `acceptQuest` | `/api/v1/game/quests*` (qua `QuestService` F072) | game.v1.yaml | `game_player` | Có sẵn (F072) |
| `gameLogin` → `PlayerBootstrap.policies.social` | `POST /api/v1/auth/game/login` (bootstrap) | identity.v1.yaml | — | Cần sửa (owner F013, giá trị từ F015) |

`ProblemCode` hiển thị: `SOCIAL_SCOPE_VIOLATION`, `FRIEND_LIMIT_REACHED`, `CLUB_FULL`, `CLUB_LIMIT_REACHED`, `CLUB_NOT_APPROVED`, `CHAT_CONTENT_BLOCKED`, `AREA_LOCKED`, `EVENT_NOT_AVAILABLE`, `EVENT_ENDED`, `EVENT_MILESTONE_NOT_REACHED`, `SEASON_TIER_NOT_REACHED`, `PERMISSION_DENIED`, `NOT_FOUND`, `RATE_LIMITED`, `VALIDATION_FAILED`, `PRECONDITION_REQUIRED`, `GAME_ACCESS_STUDENT_ONLY`, `TENANT_MISMATCH`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi facade nội bộ giữa module | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `hub.instance.join` (`area_id`, `spawn_point`, `follow_user_id`) | client → competition (qua gateway) | Cần sửa (owner F091: thêm `follow_user_id`) |
| `hub.instance.assigned`, `system.command.failed` (`ROOM_FULL`, `PERMISSION_DENIED`, `NOT_FOUND`) | server → client | Có sẵn |
| `presence.friend.updated` (`status`, `activity`, `area_id`) | chat → bạn bè | Có sẵn (enum thống nhất — delta F091) |
| `event.status.changed`, `event.progress.updated`, `season.track.updated` | game → học sinh | Có sẵn |
| `event.community_goal.updated` | game → room hub trong phạm vi | Cần sửa (owner F077: nhịp theo `community_goal_progress_flush_seconds`) |
| `notification.item.created` (deep link club/sự kiện) | chat → học sinh | Cần sửa (owner F123 + F120: `kind` cho club) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không dùng NATS (R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Client chỉ dùng schema response OpenAPI/AsyncAPI ở trên | — |

## 6. Delta cần thực hiện

F111 không sở hữu contract. Delta do owner backend thực hiện (task theo dõi T004–T007):

- **F095** (module chat, social) — thêm `listFriendSuggestions`: query `source` enum `classmates|recently_played` (bắt buộc), `limit`, `cursor`; response `{data: FriendSuggestion[], page}`, `FriendSuggestion {player: PlayerCard, source, context_kind?: enum party|match|club|coop, last_played_at?: Timestamp}`; server loại bạn hiện có, lời mời đang chờ, quan hệ chặn hai chiều, người ngoài phạm vi (SOCIAL_SYSTEM §4.1, §7); không có tên thật/trường/lớp; lỗi `401`, `403`, `422`. Tham số mới [Proposed] `friend_suggestion_recent_window` (owner SOCIAL_SYSTEM, đăng ký CONFIG_DEFAULTS). Contract test với client fixture.
- **F120** (module chat, club) — thêm `listMyClubInvitations` (`GET /social/club-invitations`, query `status` mặc định `pending`, cursor; chỉ lời mời của người gọi); `ClubInvitation` thêm `club_name` (maxLength 64), `inviter_nickname` (maxLength 32); `createClub`/`updateClub` khai báo `451 CHAT_CONTENT_BLOCKED`; `ClubType` bỏ `study_group` (đã có trong delta F120).
- **F013/F015** (module identity) — `PlayerBootstrap.policies.social {student_club_creation: enum disabled|requires_approval|auto, club_creation_eligible: boolean, cross_school_enabled: boolean}` (chỉ để hiển thị; server vẫn kiểm).
- **F091/F071** — AsyncAPI `HubInstanceJoin.follow_user_id` (uuid, tùy chọn); quyết định bỏ hoặc đổi nghĩa `joinFriendInWorld` (F111 không gọi).
- **F123/F120** — `Notification.kind` thêm `club_invitation`, `club_join_request`, `club_approved`; enum `kind` chung REST/WS (RB-513) nên WS dùng cùng giá trị; `action` deep link `{"kind":"open_club","id":"<club_id>"}`, `{"kind":"open_event","id":"<event_id>"}`.
- **F077** — `event.community_goal.updated` theo `community_goal_progress_flush_seconds`; `joinGameEvent` khai báo `428`.
- Fixture `godot/tests/fixtures/{club,liveops,social_suggestions}/*.json` sinh lại từ example sau khi delta merge; test contract parse mọi example ([GODOT_ARCHITECTURE §12](../../../docs/08-game-client/GODOT_ARCHITECTURE.md)).
- Kiểm (owner backend): `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`, `make gen MOD=game`, `make gen MOD=identity`, `make test-contract`.
