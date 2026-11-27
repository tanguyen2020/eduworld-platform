# Contracts: F112 — Godot — channel chat bền vững (khu vực, DM) và công cụ chủ club (sửa club, phó nhóm, mục tiêu/sự kiện club) [Confirmed] Server theo SOCIAL_SYSTEM §6.2: sửa club, mục tiêu, sự kiện, bổ nhiệm chỉ `owner` (`officer` → `403 PERMISSION_DENIED`); `officer` duyệt yêu cầu + mời. Thay đổi hành vi (CHANGELOG) | [Confirmed] Không: `removeClubMember` luôn `403` cho học sinh (kể cả owner/officer), `archiveClub` từ học sinh `403`; nhân sự trường làm ở portal (CHILD_SAFETY §3.4) | [Confirmed] `internal_tournament`/`tournament_template_id` từ game → `422 VALIDATION_FAILED` `errors[0] = event_kind/VALUE_NOT_AVAILABLE`; không thêm operation liệt kê template (F107) | [Confirmed] Enum `ClubGoalAcceptedMetric` (`activities_completed`, `quests_completed`, `discoveries_found`) + `listClubGoals.accepted_metrics`; metric khác → `422 VALUE_NOT_AVAILABLE` | [Confirmed] `Channel.peer` (`PlayerCard`: `user_id`, `nickname`, `level`, `avatar_config`; R16) ở channel `direct` của `listMyChannels`, `openDirectChannel` | [Confirmed] `reopen_at` (giờ server UTC): REST trường `reopen_at` của Problem, WS `system.command.failed.details.reopen_at`; chỉ khi ngoài `allowed_hours` (cũng có cho `CURFEW_ACTIVE` trên WS) | [Confirmed] Identity điền từ bản chiếu `chat_policy.updated.direct_message_enabled` + `chat_policy.common_tenant_social_updated` (F013); vắng khi chưa chiếu cấp platform | [Confirmed] `highlight` = văn bản thuần (nội dung đã che, NFC, không thẻ) + `highlight_ranges` `[{start, length}]` theo điểm mã (CHAT_SYSTEM §8.4) | [Confirmed] `updateChannelMembership` (`PATCH /chat/channels/{channel_id}/membership` `{notification_level}`, `Idempotency-Key`, audit); `Channel.notification_level`, `muted` trong `listMyChannels` |

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Client gọi qua `ApiClient` (token game, `X-Tenant-Id` của phiên, `Accept-Language`). Tất cả đã có trong `specs/openapi/modules/chat.v1.yaml` (backend Done) — F112 không thêm operation.

| operationId | Method + path | Dùng cho | Trạng thái |
|---|---|---|---|
| `listMyChannels` | `GET /api/v1/chat/channels` | Tab channel bền vững, `status`, `features`, `sendable_kinds`, `max_text_length`, unread, `muted` | Có sẵn (F115, F120, delta F119) |
| `listChannelMessages` | `GET /api/v1/chat/channels/{channel_id}/messages` (`before_seq`/`after_seq`) | Lịch sử club/DM, nhảy tới kết quả tìm kiếm | Có sẵn (F115) |
| `openDirectChannel` | `POST /api/v1/chat/direct-channels` `{user_id}` → `200`/`201` `Channel`; `403 CHAT_NOT_ALLOWED`, `404` | Nhắn tin riêng | Có sẵn (F120) |
| `searchMessages` | `GET /api/v1/chat/search?q=&channel_id=&cursor=` | Bảng tìm kiếm | Có sẵn (F120) |
| `listQuickMessages`, `listStickerPacks`, `listGifs`, `createReport`, `blockUser`, `muteUser` | (như F119) | Ô soạn, báo cáo (`context.channel_id`), chặn/tắt tiếng | Có sẵn (F116–F118, F121, F095) |
| `listFriends` | `GET /api/v1/social/friends` | Tab "Bạn bè", thẻ người kia của DM | Có sẵn (F095), client ở F110 |
| `listClubs`, `getClub`, `listClubMembers`, `listClubGoals`, `listClubEvents` | `GET /api/v1/social/clubs…` | Tab club, màn Quản lý club | Có sẵn (F120), client ở F111 |
| `updateClub` | `PATCH /api/v1/social/clubs/{club_id}` (`If-Match`, `ClubUpdate`) → `Club`; `403`, `404`, `412`, `422`, `451` | Sửa thông tin | Có sẵn (F120) |
| `updateClubMemberRole` | `PATCH /api/v1/social/clubs/{club_id}/members/{user_id}` `{role: officer\|member}` → `ClubMember` | Phó nhóm | Có sẵn (F120) |
| `createClubGoal` | `POST /api/v1/social/clubs/{club_id}/goals` (`Idempotency-Key`, `ClubGoalCreate`) → `201 ClubGoal` | Mục tiêu tuần | Có sẵn (F120) |
| `createClubEvent` | `POST /api/v1/social/clubs/{club_id}/events` (`Idempotency-Key`, `ClubEventCreate`) → `201 ClubEvent` | Sự kiện club | Có sẵn (F120) |
| `PlayerBootstrap` (`feature_flags`, `policies.chat`, `policies.play_time.curfew`) — trả ở `createAuthSession` (đăng nhập game, F013/F061), client giữ trong `AppState` | — | Ẩn/hiện "Nhắn tin", giờ kết thúc giới nghiêm | Có sẵn (F013/F015) |

Không dùng trong F112 (dù có trong contract): `removeClubMember`, `archiveClub` (Clarifications — câu hỏi mở F112-2).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (client không gọi facade nội bộ giữa module) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `chat.channel.join`, `chat.channel.leave`, `chat.message.send` (+ lệnh tin của F119) | client → server | Có sẵn |
| `chat.message.created/acked/…` (như F119), `chat.unread.updated`, `chat.mention.created`, `chat.channel.membership_changed`, `chat.history.loaded`, `system.command.failed` (`CHAT_NOT_ALLOWED`, `CHAT_KIND_NOT_ALLOWED`, `CURFEW_ACTIVE`, `CHAT_MUTED`) | server → client | Có sẵn |
| `hub.instance.assigned` | server → client | Có sẵn (F091) — dùng để đổi channel `area` |
| `session.session.replaced` | server → client | Có sẵn — xóa store F112 |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không dùng NATS (R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- **Không có delta contract do F112 thiết kế** (client-side only). Các câu hỏi mở phía server F112-1…F112-9 đã được backend đóng ngày 2026-11-28 (cột cuối bảng dưới; F120/F115/F013/F107 Clarifications 2026-11-28) bằng delta additive (oasdiff 0 breaking; F112-1/2/3 là siết hành vi). Example đã thêm cho `openDirectChannel` `200`/`201`/`403`, `searchMessages` `200`, `updateClub` `200`/`412`/`451`, `updateClubMemberRole` `200`, `createClubGoal` `201`/`422`, `createClubEvent` `201`/`422`, `listClubGoals` `200`, `updateChannelMembership` `200`, AsyncAPI `chat.channel.membership_changed` (`dm_frozen`, `club_read_only`, `club_left`) và `system.command.failed` (`chat_not_allowed_hours`, `chat_not_allowed`, `curfew_active`). Fixture GdUnit4 **cần sinh lại** (`tools/gen_fixtures.mjs`, thêm các operation vào `RESPONSES`; lỗi `application/problem+json` chưa được generator đọc) — chưa chạy Godot.
- Fixture GdUnit4 sinh từ example OpenAPI/AsyncAPI vào `src/game/tests/fixtures/contract/` bằng `tools/gen_fixtures.mjs` (tiền tố `chat.`, `social.`): cần example cho `openDirectChannel` (`200`/`201`/`403`), `searchMessages`, `updateClub` (`200`/`412`/`451`), `updateClubMemberRole`, `createClubGoal`, `createClubEvent`, `chat.channel.membership_changed` (`frozen`, `read_only`, rời club), `system.command.failed` `CHAT_NOT_ALLOWED`/`CURFEW_ACTIVE`. Thiếu example nào thì thêm **chỉ ví dụ** (không đổi schema) trong `specs/openapi/eduworld-api.v1.yaml` / `specs/asyncapi/eduworld-realtime.v1.yaml` rồi tách lại module (như F119 2026-11-27) — task T002.
- **Câu hỏi mở cho backend/tài liệu** (không thiết kế ở F112; mỗi mục có đường dự phòng trong spec):

| Mã | Vấn đề | Đề xuất owner | Dự phòng ở client | Quyết định server (2026-11-28, delta backend) |
|---|---|---|---|---|
| F112-1 | SOCIAL_SYSTEM §6.2 chỉ cho `officer` duyệt yêu cầu + mời; contract/server F120 cho `officer` cả sửa club, tạo mục tiêu/sự kiện, gỡ member | SOCIAL_SYSTEM (owner tài liệu) + F120 | Công cụ quản lý chỉ cho `owner` |
| F112-2 | Học sinh owner có được gỡ thành viên, lưu trữ club trong game không (`removeClubMember`, `archiveClub` đã có) | Owner sản phẩm + CHILD_SAFETY | Không có trong game; nhân sự trường làm ở portal (F122) |
| F112-3 | Không có operation cho học sinh liệt kê template giải nội bộ (`tournament_template_id` của `ClubEventCreate`) | F107/F120 | Không có loại `internal_tournament` |
| F112-4 | Không có danh sách `metric` server chấp nhận (`minutes_learning`, `boss_defeated` → `422`) | F120 | Danh sách 3 chỉ số trong `club_form_options.gd` |
| F112-5 | `Channel` loại `direct` không có thẻ người kia (`peer`: `user_id`, nickname, avatar) | F120 (`listMyChannels`, `openDirectChannel`, additive) | Ánh xạ từ `openDirectChannel` + `listFriends` + `sender` |
| F112-6 | `CHAT_NOT_ALLOWED` (ngoài `allowed_hours`) không mang thời điểm mở lại | F115/F120 (`details.reopen_at`, additive) | Câu chung không có giờ |
| F112-7 | Identity chưa điền `PlayerBootstrap.policies.chat.direct_message_enabled` | F013/F015 | Vắng → vẫn hiện, `403` → câu chung + ẩn nút |
| F112-8 | Định dạng "đánh dấu từ khớp" của `MessageSearchHit.highlight` chưa mô tả | F120 | Bỏ mọi thẻ, tô từ khóa phía client |
| F112-9 | Chưa có operation đặt `notification_level` theo channel (CHAT_SYSTEM §9 "Mute channel") | F120 | Chỉ hiển thị `Channel.muted` |
