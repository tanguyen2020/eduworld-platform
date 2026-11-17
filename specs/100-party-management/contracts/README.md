# Contracts: F100 — Party: tạo/tham gia/mời/kick/chuyển leader/giải tán, một party active, giới hạn

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → sinh `specs/openapi/modules/competition.v1.yaml`. Mọi operation: `game_player` (học sinh), `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createParty` | `POST /api/v1/multiplayer/parties` (`Idempotency-Key`, `PartyCreate`) | competition.v1.yaml | `game_player` | Có sẵn |
| `getMyParty` | `GET /api/v1/multiplayer/parties/me` | competition.v1.yaml | `game_player` | Có sẵn |
| `getParty` | `GET /api/v1/multiplayer/parties/{party_id}` | competition.v1.yaml | `game_player` (thành viên) | Có sẵn |
| `updatePartySettings` | `PATCH /api/v1/multiplayer/parties/{party_id}` (`PartySettingsUpdate`) | competition.v1.yaml | leader | Cần sửa (thêm `422`, `404`) |
| `inviteToParty` | `POST /api/v1/multiplayer/parties/{party_id}/invitations` | competition.v1.yaml | leader / member theo policy | Cần sửa (thêm `422`) |
| `listMyInvitations` | `GET /api/v1/multiplayer/invitations?status=` | competition.v1.yaml | invitee | Có sẵn |
| `acceptPartyInvitation` | `POST /api/v1/multiplayer/invitations/{invitation_id}:accept` | competition.v1.yaml | invitee | Cần sửa (thêm `403`) |
| `rejectPartyInvitation` | `POST /api/v1/multiplayer/invitations/{invitation_id}:reject` | competition.v1.yaml | invitee | Có sẵn |
| `cancelPartyInvitation` | `POST /api/v1/multiplayer/invitations/{invitation_id}:cancel` | competition.v1.yaml | inviter / leader | Có sẵn |
| `joinParty` | `POST /api/v1/multiplayer/parties/{party_id}:join` | competition.v1.yaml | học sinh thỏa policy | Có sẵn |
| `leaveParty` | `POST /api/v1/multiplayer/parties/{party_id}:leave` | competition.v1.yaml | thành viên | Có sẵn |
| `kickPartyMember` | `POST /api/v1/multiplayer/parties/{party_id}/members/{student_id}:kick` | competition.v1.yaml | leader | Cần sửa (thêm `404`) |
| `transferPartyLeader` | `POST /api/v1/multiplayer/parties/{party_id}:transfer-leader` | competition.v1.yaml | leader | Cần sửa (thêm `409` `PARTY_MEMBER_NOT_ONLINE`) |
| `disbandParty` | `POST /api/v1/multiplayer/parties/{party_id}:disband` | competition.v1.yaml | leader | Có sẵn |

Schema dùng: `Party`, `PartyCreate`, `PartySettingsUpdate`, `PartyMember`, `PartyInvitation`, `InvitationStatus`, `PartyType`, `PartyVisibility`, `Problem` (`ProblemCode`: `PARTY_ALREADY_IN_PARTY`, `PARTY_FULL`, `PARTY_NOT_FOUND`, `PARTY_NOT_LEADER`, `PARTY_INVITE_NOT_ALLOWED`, `PARTY_INVITE_RATE_LIMITED`, `PARTY_INVITATION_EXPIRED`, `PARTY_POLICY_VIOLATION`, `PARTY_MEMBER_NOT_ONLINE`, `GAME_ACCESS_STUDENT_ONLY`, `TENANT_MISMATCH`, `VALIDATION_FAILED`, `PRECONDITION_REQUIRED`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `moderateDisplayText` | competition → chat (facade `chat`, gọi trong tiến trình) | chat.internal.v1.yaml | `competition` | Cần thêm (F116 hiện thực) |
| `resolvePrincipal` | competition → identity | identity.internal.v1.yaml | gồm `competition` | Có sẵn |

## 3. Realtime (AsyncAPI)

Giao qua Valkey `ew:rt:user:<user_id>` (research R1).

| Message | Hướng | Trạng thái |
|---|---|---|
| `party.invitation.received` | server → invitee | Có sẵn |
| `party.invitation.updated` | server → inviter/invitee | Có sẵn |
| `party.member.joined`, `party.member.left` (reason `left`/`kicked`/`timeout`/…) | server → thành viên | Có sẵn |
| `party.leader.transferred` | server → thành viên | Có sẵn |
| `party.state.snapshot` | server → thành viên (sau sửa cài đặt) | Có sẵn |
| `party.party.disbanded` | server → thành viên | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `party.created`, `party.disbanded` | competition → chat (channel `party`, `ref.parties`), AUD, ANA | `specs/events/payloads/competition/party.created.v1.schema.json`, `party.disbanded.v1.schema.json` | Cần thêm |
| `party.member_joined` / `_left` / `_kicked` | competition → chat, game (F103), AUD | `party.member_joined.v1`, `party.member_left.v1`, `party.member_kicked.v1` | Cần thêm |
| `party.invitation_*` (5 event) | competition → NTF, AUD | `party.invitation_sent.v1` … `party.invitation_cancelled.v1` | Cần thêm |
| `party.leader_transferred`, `party.settings_updated` | competition → AUD, game (F103) | `party.leader_transferred.v1`, `party.settings_updated.v1` | Cần thêm |
| `friend.added`/`removed`, `user.blocked`/`unblocked`, `sanction.applied`/`lifted` | chat → competition (projection) | `specs/events/payloads/chat/` | Cần thêm (owner F095 cho `friend.*`/`user.*`, owner F116 cho `sanction.applied`, owner F121 cho `sanction.lifted`) |
| `tenant.*`, `student.synced`, `user.status_changed`, `enrollment.changed`, `student.cleaned_up` | identity → competition | `specs/events/payloads/identity/` | Cần thêm (F010, F011, F016) |
| `game_profile.updated` | game → competition (`ref.player_profiles`) | `specs/events/payloads/game/game_profile.updated.v1.schema.json` | Cần thêm (F070) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- `chat.internal.v1.yaml`: thêm operation facade `moderateDisplayText` (body `{text ≤ 255, context: party_name|team_name, tenant_id, author_id}`; response `{decision: allowed|blocked}` không trả từ khớp; `x-allowed-callers: [competition]`).
- `eduworld-api.v1.yaml` (party): `Party` thêm `state_version` (integer); `PartyInvitation` thêm `created_at`; `Party.name` `maxLength` 255 → 128 (khớp DB); `PlayerCard`/`PartyMember.nickname` và AsyncAPI `PublicPlayer.nickname` thống nhất `maxLength` 32 (RB-302); thêm response `422` cho `updatePartySettings`, `inviteToParty`; `403` cho `acceptPartyInvitation`; `404` cho `kickPartyMember`, `updatePartySettings`; `409` cho `transferPartyLeader`; `403 GAME_ACCESS_STUDENT_ONLY` cho mọi operation party. Chạy `node scripts/openapi/split-public.mjs`.
- `specs/events/payloads/competition/`: 14 payload schema party ở §4 (mọi chuỗi có `maxLength`).
- Tài liệu (task Polish): API_DESIGN §5 (block → `PARTY_INVITE_NOT_ALLOWED`), rate lời mời `party_invite_rate` = 10/phút, 50/ngày theo CONFIG_DEFAULTS C-3 (RB-405, đã sửa); REALTIME §5 + VALKEY_DESIGN §3.2 (fan-out `ew:rt:user`); DATABASE_DESIGN §2.17 (`ref.friendships`, `ref.blocks`, `ref.sanctions`, `ref.enrollments` cho competition), §2.9 `state_version`; EVENT_DESIGN §3 consumer competition.
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`, `make test-contract`.
