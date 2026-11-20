# Contracts: F120 — Channel bền vững (world, area, class, study group, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (sinh `specs/openapi/modules/chat.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listMyChannels` · `listChannelMessages` · `sendChannelMessage` | `GET /api/v1/chat/channels` · `GET/POST …/{channel_id}/messages` | eduworld-api (chat) | học sinh, thành viên channel | Có sẵn |
| `searchMessages` | `GET /api/v1/chat/search` | eduworld-api (chat) | học sinh, chỉ channel là thành viên | Có sẵn |
| `getChatPolicy` | `GET /api/v1/tenant/chat-policies/{scope_type}/{scope_id}` | eduworld-api (chat) | `tenant.settings.read` / `school.settings.read` | Cần thêm |
| `putChatPolicy` | `PUT /api/v1/tenant/chat-policies/{scope_type}/{scope_id}` | eduworld-api (chat) | `tenant.settings.write` (tenant) / `school.settings.write` (school, class) | Cần thêm |
| `getEffectiveChatPolicy` | `GET /api/v1/tenant/chat-policies:effective` | eduworld-api (chat) | `school.settings.read` | Cần thêm |
| `getPlatformChatPolicy` · `putPlatformChatPolicy` | `GET/PUT /api/v1/admin/chat/policies/platform` | eduworld-api (chat, `x-internal`) | `platform.settings.write` | Cần thêm |
| `listClubs` · `createClub` · `getClub` · `updateClub` | `GET/POST /api/v1/social/clubs` · `GET/PATCH …/{club_id}` | eduworld-api (chat) | học sinh (theo policy) / nhân sự trường | Có sẵn (`listClubs` Cần sửa) |
| `approveClub` · `leaveClub` · `archiveClub` | `POST /api/v1/social/clubs/{club_id}:approve|:leave|:archive` | eduworld-api (chat) | `social.club.approve` / thành viên / owner | Có sẵn |
| `listClubMembers` · `updateClubMemberRole` · `removeClubMember` | `GET/PATCH/DELETE /api/v1/social/clubs/{club_id}/members[/{user_id}]` | eduworld-api (chat) | owner/officer/School Admin | Có sẵn |
| `listClubJoinRequests` · `requestJoinClub` · `acceptClubJoinRequest` · `rejectClubJoinRequest` | `/api/v1/social/clubs/{club_id}/join-requests`, `/social/club-join-requests/{request_id}:accept|:reject` | eduworld-api (chat) | owner/officer | Có sẵn |
| `inviteToClub` · `acceptClubInvitation` · `declineClubInvitation` | `/api/v1/social/clubs/{club_id}/invitations`, `/social/club-invitations/{invitation_id}:accept|:decline` | eduworld-api (chat) | owner/officer; invitee | Có sẵn |
| `listClubGoals` · `createClubGoal` · `listClubEvents` · `createClubEvent` | `/api/v1/social/clubs/{club_id}/goals|events` | eduworld-api (chat) | thành viên / owner | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không gọi đồng bộ module khác (membership qua event, R29) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `chat.channel.join` · `chat.channel.leave` · `chat.message.send` · `chat.message.created` · `chat.history.fetch` · `chat.history.loaded` · `chat.unread.updated` | client ↔ server | Có sẵn |
| `chat.channel.membership_changed` (`channel_id`, `channel_type`, `op` added/removed, `status`) | server → client | Cần thêm |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `class.synced` | Tiêu thụ (identity) | `specs/events/payloads/identity/class.synced.v1.schema.json` | Có sẵn |
| `class.created` · `enrollment.changed` · `school.updated` | Tiêu thụ (identity) | `specs/events/payloads/identity/*.v1.schema.json` | Có sẵn (áp 2026-11-17, owner F010/F015) |
| `study_group.created` · `study_group.updated` · `study_group.member_changed` · `study_group.deleted` | Tiêu thụ (learning, ADR-0084) | `specs/events/payloads/learning/study_group.*.v1.schema.json` | Cần thêm (owner F040) |
| `user.blocked` · `user.unblocked` | Tiêu thụ (chat/social) | `specs/events/payloads/chat/user.blocked.v1.schema.json` | Cần thêm (owner F095) |
| `activity.graded` | Tiêu thụ (club goal) | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Có sẵn (Cần sửa: `duration_seconds`, owner F030) |
| `quest.completed` · `boss.encounter_completed` · `world.discovery_found` | Tiêu thụ (club goal) | `specs/events/payloads/game/*.v1.schema.json` | Cần thêm (owner F072/F103/F071) |
| `chat_policy.updated` | Phát | `specs/events/payloads/chat/chat_policy.updated.v1.schema.json` | Cần thêm |
| `club.created` · `club.approved` · `club.archived` · `club.member_joined` · `club.member_left` · `club.goal_completed` | Phát | `specs/events/payloads/chat/club.*.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/chat-policy.schema.json` (v5) | Validate rule mọi cấp, chặn `grade_*`, always-on | Có sẵn |

## 6. Delta cần thực hiện

Task Phase 2: T004–T008.

- **Cần thêm** `getChatPolicy`/`putChatPolicy` (`x-service: chat`, tag `tenant`): path param `scope_type` enum `tenant|school|class`, `scope_id` uuid; header `X-Tenant-Id`; GET trả `ChatPolicyDocument` {`policy_id`, `scope_type`, `scope_id`, `version`, `rules` ($ref `chat-policy.schema.json#/properties/rules`), `reason` maxLength 500, `updated_by`, `updated_at`} + `ETag`; PUT body `ChatPolicyPut` {`rules`, `reason` minLength 10 maxLength 500}, `If-Match`; lỗi `422 CHAT_POLICY_RELAX_NOT_ALLOWED`, `422 CHAT_POLICY_ALWAYS_ON`, `422 VALIDATION_FAILED`, `412`, `403`, `404`. `getEffectiveChatPolicy` query `school_id`, `class_id?`, `channel_type` → `EffectiveChatPolicy` {`channel_type`, `enabled`, `allowed_kinds[]`, `max_text_length`, `rate`, `slow_mode_seconds`, `allowed_hours[]`, `features` ($ref `ChatFeatures`), `sources[]` {scope_type, version}} (không có danh mục từ). Platform: `getPlatformChatPolicy`/`putPlatformChatPolicy` `x-internal: true`, security `keycloakAdminBearer`.
- **Cần sửa** `ClubType` bỏ `study_group` (ADR-0084); mô tả `ClubCreate.add_whole_class` không nhắc study group; `listClubs` thêm query `status` ($ref `ClubStatus`) và `school_id` cho nhân sự.
- **Cần thêm** AsyncAPI `chat.channel.membership_changed`.
- **Cần thêm** payload schema `chat_policy.updated` {`policy_id`, `tenant_id?`, `scope_type`, `scope_id?`, `version`, `actor_id`} (không chứa rule chi tiết), `club.*` theo [EVENT_DESIGN §4.9](../../../docs/07-backend/EVENT_DESIGN.md); thêm `chat_policy.updated` vào EVENT_DESIGN.
- **Phụ thuộc owner khác** (không sửa trong feature này, ghi để theo dõi): `study_group.*` (F040), `enrollment.changed`/`class.created`/`school.updated` (F010, F015), `user.blocked` (F095), `activity.graded.duration_seconds` (F031), event game (F071, F072, F103).
- ADR-0078: chat policy cấp trường thêm `chat_policy_preset` (enum `standard`, `quick_message_emote_only`); đường ghi policy từ chối `allowed_hours` vượt giới nghiêm (`CHAT_POLICY_RELAX_NOT_ALLOWED`); endpoint System Admin bật lại DM/World/kết bạn cho tenant `common` [Confirmed] (task T057, T059).
