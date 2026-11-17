# Contracts: F102 — Giao diện party trong Godot: panel, lời mời, lobby, ready

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Client chỉ tiêu thụ contract; không sở hữu contract nào.

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createParty`, `getMyParty`, `getParty`, `updatePartySettings` | `/api/v1/multiplayer/parties*` | competition.v1.yaml | `game_player` | Có sẵn (delta `state_version` ở F100) |
| `inviteToParty`, `listMyInvitations`, `acceptPartyInvitation`, `rejectPartyInvitation`, `cancelPartyInvitation` | `/api/v1/multiplayer/parties/{party_id}/invitations`, `/api/v1/multiplayer/invitations*` | competition.v1.yaml | `game_player` | Có sẵn |
| `joinParty`, `leaveParty`, `kickPartyMember`, `transferPartyLeader`, `disbandParty` | `/api/v1/multiplayer/parties/{party_id}:*` | competition.v1.yaml | `game_player` | Có sẵn |
| `setPartyReady`, `selectPartyActivity`, `startPartySession` | `/api/v1/multiplayer/parties/{party_id}/…` | competition.v1.yaml | `game_player` | Cần sửa (F101: `expected_version`, `PARTY_NOT_READY`) |
| `listPartyEligibleActivities` | `GET /api/v1/multiplayer/parties/{party_id}/eligible-activities` | competition.v1.yaml | `game_player` | Cần thêm (F101) |
| `listFriends` | `GET /api/v1/social/friends` (nguồn `InviteFriendPicker`) | chat.v1.yaml | `game_player` | Có sẵn (F095) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi facade nội bộ giữa module | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `party.member.ready_set`, `party.activity.select`, `party.session.start` | client → server | Có sẵn / Cần sửa (`expected_version`, F101) |
| `party.state.snapshot`, `party.member.joined`, `party.member.left`, `party.member.status_changed`, `party.member.ready_changed`, `party.leader.transferred`, `party.activity.selected`, `party.session.ended`, `party.party.disbanded`, `party.invitation.received`, `party.invitation.updated` | server → client | Có sẵn |
| `party.session.started` | server → client | Cần sửa (`room_type` thêm `boss`, F101) |
| `system.command.acked` / `system.command.failed`, `session.session.replaced` | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không dùng NATS (R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- Không có delta riêng của F102. Phụ thuộc delta của F100 (`state_version`) và F101 (`listPartyEligibleActivities`, `expected_version`, `PARTY_NOT_READY`, `room_type: boss`). Fixture `godot/tests/fixtures/multiplayer/*.json` sinh lại từ example sau khi delta merge; test contract parse mọi example (GODOT_ARCHITECTURE §12).
