# Social System — Bạn bè, Club/Guild, quan hệ xã hội

> Trạng thái: Draft · Phiên bản: 0.4 · Owner: Game Director + UX/UI Designer · Cập nhật: 2026-11-21 (ADR-0095 — WP-S: bỏ lớp/nhóm học tập: không gợi ý "Bạn cùng lớp", không ngoại lệ tên thật, club không "thêm cả lớp", party `study_party`) · 2026-11-17 (ADR-0078: tenant `common` tắt kết bạn, DM, World chat; 2026-11-16 ADR-0084: study group không còn là loại club)
> Liên quan: [ADR-0050 Party vs Guild/Club](../adr/ADR-0050-party-tam-thoi-va-guild-club-ben-vung-la-hai-khai-niem-tach.md) · [ADR-0054 Liên trường](../adr/ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) · [ADR-0020](../adr/ADR-0020-chat-thong-nhat-cho-moi-lop-moi-tin-nhan-phai-qua-kiem-tra-h.md) · [CHAT_SYSTEM](CHAT_SYSTEM.md) · [PARTY_SYSTEM](PARTY_SYSTEM.md) · [MULTIPLAYER_GAMEPLAY](MULTIPLAYER_GAMEPLAY.md) · [MODERATION](MODERATION.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)

> [!IMPORTANT]
> **Bỏ khái niệm lớp — [ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) (owner chốt 2026-11-21).** Không còn Study Group (ADR-0084 bị thay toàn phần) và club do trường lập không có tùy chọn "thêm cả lớp"; bỏ gợi ý kết bạn "Bạn cùng lớp" (còn "Vừa chơi cùng"); mọi người chơi khác chỉ thấy nickname + avatar (không còn ngoại lệ tên thật cho bạn cùng lớp); presence chỉ cho bạn bè/party/club. Nội dung bên dưới **đã viết lại theo ADR-0095 ở WP-S (2026-11-21)** ([CODE_PLAN §4.2](../12-project/CODE_PLAN_CMS_LMS_FIRST.md#42-kế-hoạch-bỏ-khái-niệm-lớp-adr-0095)); "lớp 1–3", "lớp 4–9"… còn lại là cách gọi **khối**; khi mâu thuẫn, ADR-0095 thắng.

## 1. Mục đích & phạm vi

Tài liệu **sở hữu** các quan hệ xã hội bền vững giữa người chơi: **bạn bè**, **block/mute** (quan hệ), **Club/Guild** (nhóm bền vững), **quy tắc liên trường** và **thông báo xã hội**. Không lặp lại:

| Chủ đề | Tài liệu sở hữu |
|---|---|
| Kênh chat, tính năng tin nhắn, UX chat, quick message | [CHAT_SYSTEM](CHAT_SYSTEM.md) |
| Kiểm duyệt, report flow, sanction, danh mục từ nhạy cảm | [MODERATION](MODERATION.md) |
| Party (nhóm chơi tạm thời), Party Lobby, co-op session | [PARTY_SYSTEM](PARTY_SYSTEM.md) |
| Instance, presence, gặp nhau trong thế giới, mất kết nối | [MULTIPLAYER_GAMEPLAY](MULTIPLAYER_GAMEPLAY.md) |
| Emote 3D (danh sách, emote wheel, mở khóa) | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |

Domain thuộc **module chat** (Social & Chat, ADR-0036), database `eduworld_chat`.

## 2. Mục tiêu trải nghiệm (game-first)

- Học sinh cảm thấy "có bạn trong thế giới": gặp nhau ở hub, chào bằng emote, kết bạn, rủ vào party, cùng làm quest/boss, cùng đội thi đấu, có club để thuộc về **[Confirmed — Master Prompt §13]**.
- Giao tiếp phục vụ **hợp tác học tập và vui chơi**, không phải mạng xã hội mở: không tìm người lạ tự do, không hồ sơ công khai, không feed **[Confirmed]**.
- Mọi học sinh lớp 1–9 có **cùng quyền xã hội** (ADR-0020); khác biệt theo khối chỉ ở UX (nút to, chọn bằng avatar cho lớp nhỏ) **[Confirmed]**.
- Mọi người chơi khác (kể cả cùng trường, cùng khối) chỉ thấy **nickname + avatar**, không thấy tên thật, trường, khối (R16; ADR-0095 bỏ ngoại lệ bạn cùng lớp) **[Confirmed]**.

## 3. Bậc thang giao tiếp

Mọi học sinh dùng được tất cả; khác nhau ở rủi ro và cách kiểm tra.

| Bậc | Hình thức | Tài liệu chi tiết | Rủi ro |
|---|---|---|---|
| 0 | Emote 3D (vẫy tay, vỗ tay, nhảy, ăn mừng, cảm ơn, xin lỗi) | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | Rất thấp |
| 1 | Emoji / reaction / sticker / GIF đã duyệt | [CHAT_SYSTEM §7.5](CHAT_SYSTEM.md#75-emoji-sticker-gif) | Rất thấp |
| 2 | Quick message (câu soạn sẵn theo ngữ cảnh) | [CHAT_SYSTEM §8.5](CHAT_SYSTEM.md#85-quick-messages) | Thấp |
| 3 | Text đã kiểm tra hợp lệ | [CHAT_SYSTEM](CHAT_SYSTEM.md), [MODERATION](MODERATION.md) | Trung bình |
| 4 | Direct message (chỉ giữa bạn bè) | [CHAT_SYSTEM §2](CHAT_SYSTEM.md#2-channel-types) | Cao hơn: kiểm tra như mọi tin + phân tích hội thoại bất đồng bộ |
| — | Voice | [ADR-0051](../adr/ADR-0051-pham-vi-voice-chat.md): không có trong Release 1.0 ([ADR-0070](../adr/ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md), OQ-19 đã đóng) | — |

## 4. Bạn bè (Friend)

### 4.1. Quy tắc

| Hành động | Quy tắc mặc định | Nhãn |
|---|---|---|
| Gửi lời mời kết bạn | Tới học sinh **cùng trường**; khác trường trong cùng tenant `unit` khi tenant bật `cross_school_social_enabled` (§7, ADR-0054); không bao giờ khác tenant; hai người đã chặn nhau (một trong hai chiều) không gửi được. **Tenant `common`: tắt mặc định** (`common_tenant_social_defaults`, [CHILD_SAFETY §3.6](../10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078)) — API trả `SOCIAL_SCOPE_VIOLATION`, UI ẩn nút kết bạn và mã bạn bè | [Confirmed] · tenant `common` [Confirmed — Technical Architect 2026-11-17] (ADR-0078) |
| Tìm bạn | **Không** có tìm kiếm tự do theo tên/ID. Chỉ chọn từ: người chơi gần (cùng instance), thành viên party/trận/club vừa chơi cùng ("Vừa chơi cùng"), Mã bạn bè (trong phạm vi được phép); không có gợi ý "Bạn cùng lớp" hay theo khối (ADR-0095) | [Confirmed] |
| Mã bạn bè | Mã ngắn hết hạn sau `friend_code_ttl` (10 phút; mỗi người một mã hiệu lực) do một người tạo để bạn ngồi cạnh nhập (phòng máy), vẫn kiểm tra phạm vi trường | [Confirmed] |
| Giới hạn (`friend_limits`) | ≤ 10 lời mời đang chờ; ≤ 20 lời mời/ngày; ≤ 100 bạn. Đây là giới hạn duy nhất cho lời mời kết bạn (rate limit API áp theo cùng tham số) | [Confirmed] |
| Hết hạn lời mời (`friend_request_ttl`) | 7 ngày | [Confirmed] |
| Từ chối | Lặng lẽ: người gửi chỉ thấy "đang chờ" đến khi hết hạn, tránh tổn thương | [Confirmed] |
| Hủy kết bạn | Một chạm, không thông báo cho người kia | [Confirmed] |
| Hiển thị trong danh sách | Nickname, avatar, level, trạng thái presence ([MULTIPLAYER_GAMEPLAY §4](MULTIPLAYER_GAMEPLAY.md#5-presence)); không hiện tên thật, trường, khối (R16, ADR-0095) | [Confirmed] |

### 4.2. Hành động từ danh sách bạn bè

| Hành động | Điều kiện | Kết quả |
|---|---|---|
| Mời vào party | Bạn online, không trong party khác | Lời mời party ([PARTY_SYSTEM §2](PARTY_SYSTEM.md#2-party-management--thao-tác)) |
| **Tham gia bạn (Join friend)** | Bạn đang ở hub/zone công khai (không trong trận/bài kiểm tra/instance riêng) | Dịch chuyển tới instance của bạn nếu còn chỗ và cùng partition ([MULTIPLAYER_GAMEPLAY §6](MULTIPLAYER_GAMEPLAY.md#6-gán-instance)); không thì báo "Khu vực của bạn đã đầy" |
| Thách đấu (friend duel) | Cả hai online, cùng grade band | Trận unranked ([MATCHMAKING §8](MATCHMAKING.md#8-party-queue--friend-duel)) |
| Nhắn tin | Là bạn bè, không block | Channel `direct` |
| Xem thành tích | Bạn bè | Level, danh hiệu, huy hiệu, collection công khai; **không** hiển thị điểm học tập, mastery, kết quả kiểm tra |

### 4.3. Dữ liệu & API

- Bảng (`eduworld_chat`, schema `social`): `friend_requests`, `friendships` (cặp đối xứng), `blocks`, `mutes` — đều có `tenant_id`, soft-delete (ADR-0028), khóa chính UUID và unique partial `WHERE deleted_at IS NULL` trên cặp người dùng, để kết bạn/chặn lại sau khi đã hủy vẫn tạo được dòng mới ([DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md)).
- API (đã có trong OpenAPI): `GET/POST /api/v1/social/friends`, `/social/friend-requests` (+ `:accept`), `/social/blocks/{user_id}`. Mọi route `/social/*` phía người chơi qua middleware `game_player` (chỉ học sinh — AGENTS R23, ADR-0031).
- Event (catalog: [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md)): `friend.added`, `friend.removed`, `user.blocked`, `user.unblocked`, `user.muted`, `user.unmuted` (consumer: chat, competition — tránh ghép trận/instance).

## 5. Block, mute, report (điểm vào)

| Hành động | Hiệu lực | Nơi xử lý |
|---|---|---|
| **Block** | **Kết thúc tình bạn** nếu đang là bạn (phát `friend.removed`, không thông báo cho người kia) và hủy lời mời kết bạn/party đang chờ giữa hai người; ẩn tin hai chiều trong channel chung, chặn DM, lời mời party/kết bạn, thách đấu; không ghép cùng trận; cố gắng không cùng hub instance; người bị block không được thông báo | Quan hệ: tài liệu này. Hiệu lực trong chat: [CHAT_SYSTEM §9](CHAT_SYSTEM.md#9-mute-block-report-phía-chat). Ghép trận: [MATCHMAKING](MATCHMAKING.md) |
| **Mute** người chơi | Ẩn tin/emote của người đó với mình | [CHAT_SYSTEM §9](CHAT_SYSTEM.md#9-mute-block-report-phía-chat) |
| **Report** | Tạo report + moderation case | [MODERATION §5](MODERATION.md#5-report-flow) |

- Mọi nickname/tin nhắn/avatar có menu **Báo cáo · Chặn · Tắt tiếng**, ≤ 2 chạm trong trận.
- Report được Platform Safety Team xử lý tập trung ([MODERATION §6](MODERATION.md#6-moderation-queue--quyền-moderator), ADR-0072).

## 6. Club / Guild (nhóm bền vững) — ADR-0050 [Confirmed]

Club là nhóm **bền vững** (khác Party là nhóm chơi tạm thời). Team cố định là **loại club**, không phải khái niệm riêng. Không có nhóm học tập LMS (Study Group) và không có channel `study_group` ([ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) thay toàn phần ADR-0084); club không gắn với lớp hay khối.

### 6.1. Loại club

| `club_type` | Mục đích | Ai tạo | Duyệt |
|---|---|---|---|
| `school_club` | Câu lạc bộ trường (Toán, Khoa học, Tiếng Anh…) | `school_admin` | Không cần |
| `subject_club` | Câu lạc bộ theo môn, có thể gồm học sinh nhiều khối của trường | `school_admin`, `academic_manager` (môn trong phạm vi) | Không cần |
| `competition_team` | Đội thi đấu bền vững (giải trường/khối, mùa) | `school_admin`; học sinh lớp 4–9 khi policy trường cho phép | Theo `student_club_creation` |
| `interest_group` | Nhóm sở thích trong game (khám phá, sưu tầm) | Học sinh lớp 4–9 khi policy trường cho phép | Theo `student_club_creation` |

Policy trường `student_club_creation` (do `school_admin` đặt, như nhau cho mọi học sinh của trường) **[Confirmed — Technical Architect 2026-11-15]**: `disabled` (mặc định) · `requires_approval` (club ở `pending_approval` đến khi `school_admin` duyệt trong portal) · `auto` (club hoạt động ngay sau khi tên/mô tả qua kiểm duyệt). Giới hạn lớp 4–9 là điều kiện tạo nhóm, không phải điều kiện chat (chat club theo ChatPolicy chung, R15).

### 6.2. Thành viên & vai trò

| Vai trò | Quyền |
|---|---|
| `owner` (nhân sự tạo trong portal, hoặc học sinh tạo) | Sửa thông tin, duyệt yêu cầu tham gia, mời, xóa thành viên, bổ nhiệm officer, tạo sự kiện club. Owner là nhân sự quản lý club trong portal, **không** là thành viên chat |
| `officer` (học sinh) | Duyệt yêu cầu tham gia, mời |
| `member` (học sinh) | Tham gia hoạt động, chat club, rời club |

- Tham gia: **yêu cầu tham gia** (owner/officer duyệt) hoặc **lời mời**. Club do nhà trường tạo có thể thêm trực tiếp danh sách học sinh của trường (trong giới hạn thành viên); **không** có tùy chọn "thêm cả lớp" (ADR-0095).
- `school_admin` quản lý mọi club của trường trong portal: khóa (`locked`), lưu trữ, gỡ thành viên; **không** đọc nội dung chat club (vi phạm chat do Safety Team xử lý, [MODERATION](MODERATION.md)).
- Kích thước tối đa: 50 (cấu hình theo loại, platform-only) **[Confirmed]**.
- Mỗi học sinh tối đa 5 club **[Confirmed]**.
- Phạm vi: **một tenant**; mặc định cùng trường, liên trường trong tenant `unit` khi tenant bật `cross_school_social_enabled` (§7, ADR-0054).

### 6.3. Nội dung club

| Thành phần | Mô tả |
|---|---|
| Thông tin | Tên (qua kiểm duyệt [MODERATION §2](MODERATION.md#2-phạm-vi-áp-dụng)), biểu tượng chọn từ thư viện, mô tả ngắn (qua kiểm duyệt), loại, môn |
| Chat club | Channel bền vững (`team` cố định — [CHAT_SYSTEM §2](CHAT_SYSTEM.md#2-channel-types)); **không** purge, theo retention chung |
| Mục tiêu club | Mục tiêu tập thể theo tuần (ví dụ "cả club hoàn thành 30 bài Toán"); đếm đóng góp từng người từ event học tập, không chia sẻ điểm/mastery cá nhân |
| Hoạt động | Rủ party từ thành viên club (Party có `club_id` tham chiếu), co-op quest, boss nhóm, luyện tập chung |
| Thành tích club | Huy hiệu club khi đạt mục tiêu; thành tích cá nhân nhận riêng theo [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| Sự kiện club | Lịch hoạt động (buổi luyện tập, giải nội bộ từ template — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)) do owner tạo |

### 6.4. Quan hệ Party ↔ Club

- Party có thể được tạo **từ** club (`parties.club_id`): chỉ mời thành viên club; party vẫn tạm thời, chat `party` vẫn purge (ADR-0033); chat club không bị ảnh hưởng.
- `party_type = study_party` ("Nhóm cùng học", trước là `study_group` — ADR-0095 D7) của [PARTY_SYSTEM](PARTY_SYSTEM.md) là **phiên học nhóm tạm thời** do học sinh lập, không gắn `club_id`; không có nhóm học tập LMS.
- `competition_team` dạng party (theo giải/mùa) có thể liên kết club `competition_team` để giữ đội qua nhiều giải.

### 6.5. Dữ liệu, API, event

- Bảng (`eduworld_chat`, schema `social`): `clubs` (`club_id`, `tenant_id`, `school_id?`, `club_type`, `name` varchar(64), `description` varchar(500), `icon_id`, `owner_id`, `status` `pending_approval|active|locked|archived`, `max_members`, audit cột chuẩn), `club_members` (`club_id`, `user_id`, `role`, `joined_at`, `left_at`; unique active), `club_join_requests`, `club_invitations`, `club_goals`, `club_goal_progress`, `club_events`.
- API (chi tiết trong [OpenAPI](../../specs/openapi/eduworld-api.v1.yaml) và [API_DESIGN §9.10](../07-backend/API_DESIGN.md)): `GET/POST /api/v1/social/clubs`, `GET/PATCH /social/clubs/{club_id}`, `POST /social/clubs/{club_id}:approve` (`school_admin`), `:leave`, `:archive`; `GET /social/clubs/{club_id}/members`, `PATCH/DELETE /social/clubs/{club_id}/members/{user_id}`; `POST /social/clubs/{club_id}/join-requests`, `POST /social/club-join-requests/{request_id}:accept|:reject`; `POST /social/clubs/{club_id}/invitations`, `POST /social/club-invitations/{invitation_id}:accept|:decline`; `GET/POST /social/clubs/{club_id}/goals`, `GET/POST /social/clubs/{club_id}/events`.
- Event (catalog: [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md)): `club.created`, `club.approved`, `club.archived`, `club.member_joined`, `club.member_left`, `club.goal_completed`.
- Lỗi: `CLUB_FULL`, `CLUB_LIMIT_REACHED`, `CLUB_NOT_APPROVED`, `SOCIAL_SCOPE_VIOLATION` (dùng chung cho mọi vi phạm phạm vi xã hội: khác trường khi tenant chưa bật, khác tenant, đã chặn); policy trường không cho học sinh tạo club → `403 PERMISSION_DENIED`.

## 7. Quy tắc liên trường — ADR-0054

| Hoạt động | Cùng trường | Khác trường, cùng tenant đơn vị | Khác tenant |
|---|---|---|---|
| Kết bạn, DM | Có | Khi tenant bật | Không |
| Party, club | Có | Khi tenant bật | Không |
| World / Area chat | Có (World mặc định theo trường) | World mở rộng toàn tenant khi tenant bật | Không |
| Ghép trận, chat `match` | Có (mặc định) | Khi tenant bật | Chỉ giải đấu/sự kiện liên tenant do platform tổ chức |
| Leaderboard | Có | Có | Chỉ bảng toàn platform, nickname |

**Tenant `common`** (ADR-0078, [Confirmed — Technical Architect 2026-11-17]): không có khái niệm "cùng trường" có ý nghĩa an toàn, nên kết bạn, DM và World chat **tắt mặc định**; chỉ System Admin bật lại có lý do + audit ([CHILD_SAFETY §3.6](../10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078)).

"Khi tenant bật" là cài đặt tenant `cross_school_social_enabled` (`platform.tenants.settings`, chỉ có nghĩa với tenant `unit`; chưa đặt = tắt) **[Confirmed — Technical Architect 2026-11-16]**; Tenant Admin bật/tắt có audit; module khác đọc qua projection `ref.tenants.settings_subset` ([MULTITENANCY §7](../06-architecture/MULTITENANCY.md#7-các-tình-huống-đặc-biệt)). Hoạt động liên tenant chỉ tồn tại trong ngữ cảnh do platform tạo; khi kết thúc, kênh liên lạc đóng.

## 8. Thông báo xã hội

| Loại | Kênh | Ghi chú |
|---|---|---|
| Lời mời party / kết bạn / club | Popup trong game (tắt được) + hộp thông báo | |
| Tin nhắn mới, mention | Badge unread trên HUD ([CHAT_SYSTEM §8.2](CHAT_SYSTEM.md#82-unread-count)) | Âm thanh tắt mặc định |
| Thông báo trường: **chỉ** bài được giao và kết quả bài giao đã công bố | Bảng tin trường ở Main Hub + hộp thông báo (tab "Nhà trường giao") | Ưu tiên cao; không có thông báo tự do của nhân sự nhà trường tới học sinh |
| Kết quả xử lý báo cáo | Hộp thông báo ("Báo cáo của em đã được xử lý") | Không nêu chi tiết hình phạt |

**Không có push notification ngoài game cho học sinh trong Release 1.0** [Confirmed]: tránh kéo trẻ quay lại game ([GAME_DESIGN](../01-game-design/GAME_DESIGN.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)). Thông báo chỉ hiển thị trong game; nhân sự nhà trường nhận thông báo trong Admin Portal. API: `GET /api/v1/notifications`; đẩy realtime qua `notification.item.created`. Loại thông báo dùng **một enum `kind`** chung cho REST và WS (contract ở `specs/openapi`, `specs/asyncapi`).

## 9. Acceptance criteria

| ID | Tiêu chí |
|---|---|
| AC-SOC-001 | Học sinh gửi lời mời kết bạn tới học sinh khác trường (tenant chưa bật liên trường) → bị từ chối `SOCIAL_SCOPE_VIOLATION` |
| AC-SOC-002 | Không có API tìm người chơi theo tên/username tự do |
| AC-SOC-003 | Từ chối lời mời kết bạn không gửi thông báo cho người gửi; lời mời hết hạn sau 7 ngày |
| AC-SOC-004 | A block B → tình bạn A–B (nếu có) kết thúc; B không mời A vào party/club, không kết bạn lại, không DM, không thách đấu, không cùng proposal ghép trận |
| AC-SOC-005 | "Tham gia bạn" chỉ thành công khi bạn ở hub/zone công khai và instance còn chỗ; không vào được instance riêng/trận/bài kiểm tra |
| AC-SOC-006 | Trường đặt `student_club_creation = requires_approval`: học sinh tạo club `interest_group` → `pending_approval`, chỉ hoạt động sau khi `school_admin` duyệt; `disabled` → `403 PERMISSION_DENIED` (cùng mã cho mọi khối; khác biệt theo khối chỉ ở UX); `club_type` ngoài 4 loại §6.1 (kể cả `study_group` cũ) → `422 VALIDATION_FAILED` |
| AC-SOC-007 | Club đạt 50 thành viên → yêu cầu tham gia mới bị từ chối `CLUB_FULL` |
| AC-SOC-008 | Mục tiêu club chỉ hiển thị số đóng góp, không hiển thị điểm/mastery cá nhân của thành viên khác |
| AC-SOC-009 | Party tạo từ club chỉ mời được thành viên club; party giải tán không ảnh hưởng chat club |
| AC-SOC-010 | Hồ sơ bạn bè không hiển thị điểm học tập, mastery, kết quả kiểm tra |
| AC-SOC-011 | Học sinh tenant `common` gửi lời mời kết bạn, tạo mã bạn bè hoặc mở DM → bị từ chối `SOCIAL_SCOPE_VIOLATION`; UI không hiện các nút này (ADR-0078) |
