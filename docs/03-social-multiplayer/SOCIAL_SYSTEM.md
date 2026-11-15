# Social System — Bạn bè, Club/Guild, quan hệ xã hội

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: Game Director + UX/UI Designer · Cập nhật: 2026-11-15
> Liên quan: [ADR-0050 Party vs Guild/Club](../adr/ADR-0050-party-tam-thoi-va-guild-club-ben-vung-la-hai-khai-niem-tach.md) · [ADR-0054 Liên trường](../adr/ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) · [ADR-0020](../adr/ADR-0020-chat-thong-nhat-cho-moi-lop-moi-tin-nhan-phai-qua-kiem-tra-h.md) · [CHAT_SYSTEM](CHAT_SYSTEM.md) · [PARTY_SYSTEM](PARTY_SYSTEM.md) · [MULTIPLAYER_GAMEPLAY](MULTIPLAYER_GAMEPLAY.md) · [MODERATION](MODERATION.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)

## 1. Mục đích & phạm vi

Tài liệu **sở hữu** các quan hệ xã hội bền vững giữa người chơi: **bạn bè**, **block/mute** (quan hệ), **Club/Guild** (nhóm bền vững), **quy tắc liên trường** và **thông báo xã hội**. Không lặp lại:

| Chủ đề | Tài liệu sở hữu |
|---|---|
| Kênh chat, tính năng tin nhắn, UX chat, quick message | [CHAT_SYSTEM](CHAT_SYSTEM.md) |
| Kiểm duyệt, report flow, sanction, danh mục từ nhạy cảm | [MODERATION](MODERATION.md) |
| Party (nhóm chơi tạm thời), Party Lobby, co-op session | [PARTY_SYSTEM](PARTY_SYSTEM.md) |
| Instance, presence, gặp nhau trong thế giới, mất kết nối | [MULTIPLAYER_GAMEPLAY](MULTIPLAYER_GAMEPLAY.md) |
| Emote 3D (danh sách, emote wheel, mở khóa) | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |

Domain thuộc **chat-service** (Social & Chat, ADR-0036), database `eduworld_chat`.

## 2. Mục tiêu trải nghiệm (game-first)

- Học sinh cảm thấy "có bạn trong thế giới": gặp nhau ở hub, chào bằng emote, kết bạn, rủ vào party, cùng làm quest/boss, cùng đội thi đấu, có club để thuộc về **[Confirmed — Master Prompt §13]**.
- Giao tiếp phục vụ **hợp tác học tập và vui chơi**, không phải mạng xã hội mở: không tìm người lạ tự do, không hồ sơ công khai, không feed **[Confirmed]**.
- Mọi học sinh lớp 1–9 có **cùng quyền xã hội** (ADR-0020); khác biệt theo khối chỉ ở UX (nút to, chọn bằng avatar cho lớp nhỏ) **[Confirmed]**.
- Người ngoài trường chỉ thấy **nickname + avatar**, không thấy tên thật, trường, lớp (R16) **[Confirmed]**.

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
| Gửi lời mời kết bạn | Tới học sinh **cùng trường**; khác trường trong cùng tenant `unit` khi tenant bật (ADR-0054); không bao giờ khác tenant | [Proposed] |
| Tìm bạn | **Không** có tìm kiếm tự do theo tên/ID. Chỉ chọn từ: người chơi gần (cùng instance), bạn cùng lớp, thành viên party/trận/club vừa chơi cùng (trong phạm vi được phép) | [Proposed] |
| Mã bạn bè | Mã ngắn hết hạn 10 phút do một người tạo để bạn ngồi cạnh nhập (phòng máy), vẫn kiểm tra phạm vi trường | [Proposed] |
| Giới hạn | ≤ 10 lời mời đang chờ; ≤ 20 lời mời/ngày; ≤ 100 bạn | [Proposed] |
| Hết hạn lời mời | 7 ngày | [Proposed] |
| Từ chối | Lặng lẽ: người gửi chỉ thấy "đang chờ" đến khi hết hạn, tránh tổn thương | [Proposed] |
| Hủy kết bạn | Một chạm, không thông báo cho người kia | [Proposed] |
| Hiển thị trong danh sách | Nickname, avatar, level, trạng thái presence ([MULTIPLAYER_GAMEPLAY §4](MULTIPLAYER_GAMEPLAY.md#5-presence)); tên thật chỉ hiện với bạn cùng lớp | [Proposed] |

### 4.2. Hành động từ danh sách bạn bè

| Hành động | Điều kiện | Kết quả |
|---|---|---|
| Mời vào party | Bạn online, không trong party khác | Lời mời party ([PARTY_SYSTEM §2](PARTY_SYSTEM.md#2-party-management--thao-tác)) |
| **Tham gia bạn (Join friend)** | Bạn đang ở hub/zone công khai (không trong trận/bài kiểm tra/instance riêng) | Dịch chuyển tới instance của bạn nếu còn chỗ và cùng partition ([MULTIPLAYER_GAMEPLAY §6](MULTIPLAYER_GAMEPLAY.md#6-gán-instance)); không thì báo "Khu vực của bạn đã đầy" |
| Thách đấu (friend duel) | Cả hai online, cùng grade band | Trận unranked ([MATCHMAKING §8](MATCHMAKING.md#8-party-queue--friend-duel)) |
| Nhắn tin | Là bạn bè, không block | Channel `direct` |
| Xem thành tích | Bạn bè | Level, danh hiệu, huy hiệu, collection công khai; **không** hiển thị điểm học tập, mastery, kết quả kiểm tra |

### 4.3. Dữ liệu & API

- Bảng (`eduworld_chat`, schema `social`): `friend_requests`, `friendships` (cặp đối xứng), `blocks`, `mutes` — đều có `tenant_id`, soft-delete (ADR-0028).
- API (đã có trong OpenAPI): `GET/POST /api/v1/social/friends`, `/social/friend-requests` (+ `:accept`), `/social/blocks/{user_id}`.
- Event (catalog: [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md)): `friend.added`, `friend.removed`, `user.blocked`, `user.unblocked`, `user.muted`, `user.unmuted` (consumer: chat, competition — tránh ghép trận/instance).

## 5. Block, mute, report (điểm vào)

| Hành động | Hiệu lực | Nơi xử lý |
|---|---|---|
| **Block** | Ẩn tin hai chiều trong channel chung (trừ channel lớp: tin hiển thị thu gọn), chặn DM, lời mời party/kết bạn, thách đấu; không ghép cùng trận; cố gắng không cùng hub instance; người bị block không được thông báo | Quan hệ: tài liệu này. Hiệu lực trong chat: [CHAT_SYSTEM §9](CHAT_SYSTEM.md#9-mute-block-report-phía-chat). Ghép trận: [MATCHMAKING](MATCHMAKING.md) |
| **Mute** người chơi | Ẩn tin/emote của người đó với mình | [CHAT_SYSTEM §9](CHAT_SYSTEM.md#9-mute-block-report-phía-chat) |
| **Report** | Tạo report + moderation case | [MODERATION §5](MODERATION.md#5-report-flow) |

- Mọi nickname/tin nhắn/avatar có menu **Báo cáo · Chặn · Tắt tiếng**, ≤ 2 chạm trong trận.
- Không chặn được giáo viên trong channel lớp **[Proposed]**.

## 6. Club / Guild (nhóm bền vững) — ADR-0050 [Proposed]

Club là nhóm **bền vững** (khác Party là nhóm chơi tạm thời). Team cố định và Study Group là **loại club**, không phải khái niệm riêng.

### 6.1. Loại club

| `club_type` | Mục đích | Ai tạo | Duyệt |
|---|---|---|---|
| `study_group` | Nhóm học tập theo môn/chủ đề | Giáo viên; học sinh đề xuất | Giáo viên lớp/bộ môn duyệt khi học sinh tạo |
| `school_club` | Câu lạc bộ trường (Toán, Khoa học, Tiếng Anh…) | Giáo viên, School Admin | Không cần |
| `subject_club` | Câu lạc bộ theo môn, có thể liên lớp | Giáo viên | Không cần |
| `competition_team` | Đội thi đấu bền vững (giải trường/lớp, mùa) | Học sinh, giáo viên | Giáo viên duyệt khi học sinh tạo |
| `interest_group` | Nhóm sở thích trong game (khám phá, sưu tầm) | Học sinh | Giáo viên duyệt |

### 6.2. Thành viên & vai trò

| Vai trò | Quyền |
|---|---|
| `owner` (giáo viên hoặc học sinh tạo) | Sửa thông tin, duyệt yêu cầu tham gia, mời, xóa thành viên, bổ nhiệm officer, tạo sự kiện club, ghim tin |
| `officer` | Duyệt yêu cầu tham gia, mời, ghim tin |
| `member` | Tham gia hoạt động, chat club, rời club |
| `teacher_supervisor` | Giáo viên giám sát mọi club do học sinh tạo trong lớp/trường: xem, khóa chat, giải tán, gỡ thành viên |

- Tham gia: **yêu cầu tham gia** (owner/officer duyệt) hoặc **lời mời**. Club do giáo viên tạo cho lớp có thể tự thêm cả lớp.
- Kích thước tối đa: 50 (cấu hình theo loại, platform-only) **[Proposed]**.
- Mỗi học sinh tối đa 5 club **[Proposed]**.
- Phạm vi: **một tenant**; mặc định cùng trường, liên trường trong tenant `unit` khi tenant bật (ADR-0054).

### 6.3. Nội dung club

| Thành phần | Mô tả |
|---|---|
| Thông tin | Tên (qua kiểm duyệt [MODERATION §2](MODERATION.md#2-phạm-vi-áp-dụng)), biểu tượng chọn từ thư viện, mô tả ngắn (qua kiểm duyệt), loại, môn |
| Chat club | Channel bền vững (`study_group` hoặc `team` cố định — [CHAT_SYSTEM §2](CHAT_SYSTEM.md#2-channel-types)); **không** purge, theo retention chung |
| Mục tiêu club | Mục tiêu tập thể theo tuần (ví dụ "cả club hoàn thành 30 bài Toán"); đếm đóng góp từng người từ event học tập, không chia sẻ điểm/mastery cá nhân |
| Hoạt động | Rủ party từ thành viên club (Party có `club_id` tham chiếu), co-op quest, boss nhóm, luyện tập chung |
| Thành tích club | Huy hiệu club khi đạt mục tiêu; thành tích cá nhân nhận riêng theo [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| Sự kiện club | Lịch hoạt động (buổi luyện tập, giải nội bộ từ template — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)) do owner/giáo viên tạo |

### 6.4. Quan hệ Party ↔ Club

- Party có thể được tạo **từ** club (`parties.club_id`): chỉ mời thành viên club; party vẫn tạm thời, chat `party` vẫn purge (ADR-0033); chat club không bị ảnh hưởng.
- `party_type = study_group` của PARTY_SYSTEM là **phiên học nhóm tạm thời**; khi tạo từ club `study_group` thì gắn `club_id`.
- `competition_team` dạng party (theo giải/mùa) có thể liên kết club `competition_team` để giữ đội qua nhiều giải.

### 6.5. Dữ liệu, API, event

- Bảng (`eduworld_chat`, schema `social`): `clubs` (`club_id`, `tenant_id`, `school_id?`, `club_type`, `name` varchar(64), `description` varchar(500), `icon_id`, `owner_id`, `status` `pending_approval|active|locked|archived`, `max_members`, audit cột chuẩn), `club_members` (`club_id`, `user_id`, `role`, `joined_at`, `left_at`; unique active), `club_join_requests`, `club_invitations`, `club_goals`, `club_goal_progress`, `club_events`.
- API (chi tiết trong [OpenAPI](../../specs/openapi/eduworld-api.v1.yaml) và [API_DESIGN §9.10](../07-backend/API_DESIGN.md)): `GET/POST /api/v1/social/clubs`, `GET/PATCH /social/clubs/{club_id}`, `POST /social/clubs/{club_id}:approve` (giáo viên), `:leave`, `:archive`; `GET /social/clubs/{club_id}/members`, `PATCH/DELETE /social/clubs/{club_id}/members/{user_id}`; `POST /social/clubs/{club_id}/join-requests`, `POST /social/club-join-requests/{request_id}:accept|:reject`; `POST /social/clubs/{club_id}/invitations`, `POST /social/club-invitations/{invitation_id}:accept|:decline`; `GET/POST /social/clubs/{club_id}/goals`, `GET/POST /social/clubs/{club_id}/events`.
- Event (catalog: [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md)): `club.created`, `club.approved`, `club.archived`, `club.member_joined`, `club.member_left`, `club.goal_completed`.
- Lỗi: `CLUB_FULL`, `CLUB_LIMIT_REACHED`, `CLUB_NOT_APPROVED`, `SOCIAL_SCOPE_VIOLATION` (dùng chung cho mọi vi phạm phạm vi xã hội).

## 7. Quy tắc liên trường — ADR-0054

| Hoạt động | Cùng trường | Khác trường, cùng tenant đơn vị | Khác tenant |
|---|---|---|---|
| Kết bạn, DM | Có | Khi tenant bật | Không |
| Party, club | Có | Khi tenant bật | Không |
| World / Area chat | Có (World mặc định theo trường) | World mở rộng toàn tenant khi tenant bật | Không |
| Ghép trận, chat `match` | Có (mặc định) | Khi tenant bật | Chỉ giải đấu/sự kiện liên tenant do platform tổ chức |
| Leaderboard | Có | Có | Chỉ bảng toàn platform, nickname |

Hoạt động liên tenant chỉ tồn tại trong ngữ cảnh do platform tạo; khi kết thúc, kênh liên lạc đóng.

## 8. Thông báo xã hội

| Loại | Kênh | Ghi chú |
|---|---|---|
| Lời mời party / kết bạn / club | Popup trong game (tắt được) + hộp thông báo | |
| Tin nhắn mới, mention | Badge unread trên HUD ([CHAT_SYSTEM §8.2](CHAT_SYSTEM.md#82-unread-count)) | Âm thanh tắt mặc định |
| Thông báo giáo viên, bài được giao | Bảng tin lớp ở Main Hub + hộp thông báo | Ưu tiên cao |
| Kết quả xử lý báo cáo | Hộp thông báo ("Báo cáo của em đã được xử lý") | Không nêu chi tiết hình phạt |

**Không có push notification ngoài game cho học sinh trong Release 1.0** [Proposed]: tránh kéo trẻ quay lại game ([GAME_DESIGN](../01-game-design/GAME_DESIGN.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)). Thông báo chỉ hiển thị trong game; giáo viên nhận thông báo trong Admin Portal. API: `GET /api/v1/notifications`.

## 9. Acceptance criteria

| ID | Tiêu chí |
|---|---|
| AC-SOC-001 | Học sinh gửi lời mời kết bạn tới học sinh khác trường (tenant chưa bật liên trường) → bị từ chối `SOCIAL_SCOPE_VIOLATION` |
| AC-SOC-002 | Không có API tìm người chơi theo tên/username tự do |
| AC-SOC-003 | Từ chối lời mời kết bạn không gửi thông báo cho người gửi; lời mời hết hạn sau 7 ngày |
| AC-SOC-004 | A block B → B không mời A vào party/club, không DM, không thách đấu, không cùng proposal ghép trận |
| AC-SOC-005 | "Tham gia bạn" chỉ thành công khi bạn ở hub/zone công khai và instance còn chỗ; không vào được instance riêng/trận/bài kiểm tra |
| AC-SOC-006 | Học sinh tạo club `interest_group` → `pending_approval`, chỉ hoạt động sau khi giáo viên duyệt |
| AC-SOC-007 | Club đạt 50 thành viên → yêu cầu tham gia mới bị từ chối `CLUB_FULL` |
| AC-SOC-008 | Mục tiêu club chỉ hiển thị số đóng góp, không hiển thị điểm/mastery cá nhân của thành viên khác |
| AC-SOC-009 | Party tạo từ club chỉ mời được thành viên club; party giải tán không ảnh hưởng chat club |
| AC-SOC-010 | Hồ sơ bạn bè không hiển thị điểm học tập, mastery, kết quả kiểm tra |
