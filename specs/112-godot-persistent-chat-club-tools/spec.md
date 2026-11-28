# Feature Specification: F112 — Godot — channel chat bền vững (khu vực, DM) và công cụ chủ club (sửa club, phó nhóm, mục tiêu/sự kiện club)

**Feature Branch**: `112-godot-persistent-chat-club-tools` · **Created**: 2026-11-28 · **Status**: In progress · **Module**: `src/game/` · **Mốc**: M5 · **Phụ thuộc**: F111, F119, F120 (và F110 danh sách bạn bè/`FriendProfilePanel`, F071 `SceneRouter`/khu vực, F091/F092 `hub.instance.assigned`/`RealtimeClient`, F123 hộp thông báo, F125 chế độ `compact`, F114 phong cách tầng)
**Input**: Học sinh chat ngoài trận trong các channel bền vững mà server cho phép — "Khu vực" (`area` của instance đang đứng), "Cả trường" (`world`), channel club (`team` cố định) và tin nhắn riêng với bạn bè (`direct`) — từ khung chat HUD (S12) dùng lại toàn bộ thành phần chat của F119; nhắn riêng từ hồ sơ bạn (F110); tìm lại tin trong club và DM; học sinh là chủ club (`owner`) sửa thông tin club, chọn phó nhóm (`officer`), tạo mục tiêu tuần và sự kiện club ngay trong game. Màn S12 và phần "Club" của S11 trong [STUDENT_GAME_UX](../../docs/05-ux/STUDENT_GAME_UX.md), chi tiết ở [STUDENT_GAME_UX_SOCIAL §6–§7](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md). FR nguồn (phía client): FR-CHAT-001, FR-CHAT-003, FR-CHAT-004, FR-CHAT-006, FR-CHAT-016, FR-SOC-003, FR-SOC-004, FR-MOD-002, FR-GAME-010.

## Clarifications

### Session 2026-11-28 (speckit.clarify — [Confirmed] bởi orchestrator theo lệnh "chốt hết" của owner: chọn phương án an toàn nhất, không thiết kế endpoint server mới)

- Q: Ranh giới với F119 và F111? → A: F119 sở hữu thành phần chat dùng chung (`ChatService`, `ChatStore`, `ChatDock`, bong bóng, ô soạn, menu tin, báo cáo, ảo hóa) và channel gameplay; F112 chỉ thêm **khung chat HUD** (`ChatHub`) cho channel bền vững, chế độ mở một channel bền vững theo `channel_id` (để nút "Chat club" của F111-AC-24 dùng), DM, tìm kiếm và các trạng thái riêng của channel bền vững. F111 sở hữu tab Club, trang club, xin vào/lời mời/tạo club, duyệt yêu cầu và mời của owner/officer (F111-AC-19…26); F112 thêm màn "Quản lý club" của owner. Không làm lại hành vi đã có ở F119/F111. [Confirmed]
- Q: Ai thấy công cụ quản lý club trong game? → A: Chỉ `my_role = owner`, club `active`. [SOCIAL_SYSTEM §6.2](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#62-thành-viên--vai-trò) (tài liệu sở hữu) chỉ cho `officer` duyệt yêu cầu và mời (đã có ở F111), trong khi contract/server F120 cho cả `officer` sửa club, tạo mục tiêu/sự kiện. Client chọn phạm vi hẹp hơn của tài liệu sở hữu; lệch giữa tài liệu và server ghi thành câu hỏi mở F112-1 (không đổi server ở F112). [Confirmed]
- Q: Có làm "gỡ thành viên" và "giải tán/lưu trữ club" cho chủ club là học sinh không (contract có `removeClubMember`, `archiveClub`)? → A: Không trong F112. Đây là thao tác loại người/đóng nhóm giữa trẻ em với nhau (dễ thành công cụ cô lập bạn); việc này để nhân sự trường làm ở portal (`removeSchoolClubMember`, khóa/lưu trữ — F122). Học sinh vẫn có Tắt tiếng/Chặn/Báo cáo (F119). Ghi câu hỏi mở F112-2 để owner sản phẩm quyết định sau. [Confirmed]
- Q: Loại sự kiện club nào được tạo trong game? → A: `practice_session`, `coop_quest`, `boss_raid`, `other`. Không có `internal_tournament` vì cần `tournament_template_id` mà contract chưa có cách cho học sinh liệt kê template (câu hỏi mở F112-3). [Confirmed]
- Q: Chỉ số (`metric`) nào được chọn khi tạo mục tiêu tuần? → A: Chỉ ba chỉ số server đang đếm được: `activities_completed`, `quests_completed`, `discoveries_found` (F120 Clarifications 2026-11-23: `minutes_learning`, `boss_defeated` trả `422`). Danh sách này nằm trong một file dữ liệu client (không trong scene), có thể gỡ khi server công bố danh sách chỉ số hợp lệ (câu hỏi mở F112-4). Không gửi `subject_id`. [Confirmed]
- Q: Tab DM lấy nickname/avatar của bạn ở đâu khi `Channel` của `listMyChannels` không có thông tin người kia? → A: Từ danh sách bạn bè (`listFriends`, F110) theo ánh xạ `channel_id ↔ user_id` biết được khi chính học sinh mở DM bằng `openDirectChannel`, hoặc từ `sender` của tin của người kia trong channel đó; ánh xạ chỉ giữ trong RAM của phiên. Channel `direct` chưa biết người kia chỉ được tính vào badge của tab "Bạn bè", không hiện tên đoán. Không gọi `openDirectChannel` hàng loạt (lệnh này tạo channel). Câu hỏi mở F112-5: thêm `peer` (thẻ công khai) vào `Channel` loại `direct`. [Confirmed]
- Q: Khi nào join channel `world`/`area`? → A: Chỉ khi tab tương ứng đang mở trên khung chat HUD; đóng khung hoặc đổi tab thì `chat.channel.leave`. World/Area không có unread (CHAT_SYSTEM §7.1), lịch sử chỉ là bộ đệm gần nhất do server trả khi join; giảm băng thông và giảm tiếp xúc với tin của người lạ trong trường. Channel `team` cố định của club và `direct` join khi tab mở; badge lấy từ `listMyChannels`/`chat.unread.updated`. [Confirmed]
- Q: Khung chat HUD có mở được khi đang làm bài kiểm tra/thi không? → A: Không: TRONG KHI `AppState.blocking_activity` là bài kiểm tra/thi (F064), nút chat HUD ẩn và mọi channel bền vững đang join được rời; chat gameplay (party/trận/boss) không bị ảnh hưởng vì không chạy cùng lúc với bài thi. Lý do: giữ tính trung thực của bài thi và để học sinh tập trung; không phải điều kiện theo khối (R15). [Confirmed]
- Q: Câu báo khi không gửi/mở được (ngoài giờ chat của trường, policy tắt DM, tenant `common`, không còn là bạn, bị chặn)? → A: Một câu chung trung tính theo loại lỗi, không nêu ai chặn ai, không nêu lý do chính sách: `CHAT_NOT_ALLOWED` → "Kênh này đang nghỉ, em quay lại sau nhé" (channel) / "Em chưa nhắn riêng được với bạn này lúc này" (DM); `frozen` → "Cuộc trò chuyện này đang tạm đóng". Không hiện giờ mở lại vì lỗi `CHAT_NOT_ALLOWED` chưa mang thời điểm (câu hỏi mở F112-6); `CURFEW_ACTIVE` dùng giờ kết thúc giới nghiêm có sẵn trong `PlayerBootstrap.policies.play_time.curfew`. [Confirmed]
- Q: Nút "Nhắn tin" hiện khi nào? → A: Chỉ với người đang là bạn bè, khi `feature_flags.social.friends_enabled` không `false` và `policies.chat.direct_message_enabled` không `false` (identity hiện chưa điền trường này — câu hỏi mở F112-7; vắng thì vẫn hiện, server quyết định và lỗi `403` dùng câu chung ở trên rồi ẩn nút tới hết phiên). Cờ chỉ để hiển thị (R7). [Confirmed]
- Q: Đoạn trích kết quả tìm kiếm (`MessageSearchHit.highlight`) có "đánh dấu từ khớp" theo định dạng chưa mô tả? → A: Client hiển thị `highlight` (thiếu thì `message.body`) là **văn bản thuần**: bỏ mọi thẻ đánh dấu, không phân tích BBCode/markup từ server hay người dùng; tô từ khóa bằng so khớp không dấu phía client trên chính chuỗi đó. Câu hỏi mở F112-8: ghi định dạng đánh dấu vào contract. [Confirmed]
- Q: Cài đặt thông báo theo channel (`notification_level`, CHAT_SYSTEM §9 "Mute channel")? → A: Ngoài phạm vi F112: contract chưa có operation (`PATCH …/membership` chỉ có trong tài liệu). Client chỉ đọc `Channel.muted` để hiện biểu tượng chuông gạch kèm chữ. Câu hỏi mở F112-9. [Confirmed]
- Q: Bố cục trên điện thoại (F125 `compact`) và tablet? → A: Theo STUDENT_GAME_UX_SOCIAL §7: PC/tablet — khung nổi dưới-trái mở rộng được (tablet ≤ 40% chiều rộng); điện thoại — tấm phủ cao hết màn trượt từ trái, nút "✕ Đóng" đỏ (F125-AC-06), hàng tab cuộn ngang bằng mũi tên. Quy tắc "≥ 3 tin" (`chat_list_min_visible`) áp cho mọi tab channel bền vững như F119. [Confirmed]

### Session 2026-11-28 (backend đóng câu hỏi mở F112-1…F112-9 — [Confirmed], owner "chốt hết": phương án an toàn nhất)

Delta server đã áp (F120/F115/F013/F107 Clarifications 2026-11-28; contracts §6 cột "Quyết định server"). Các quyết định phía client ở trên **giữ nguyên** (client vẫn hẹp hơn hoặc bằng server); dùng thêm trường/operation mới là quyết định của F112 khi triển khai, không mở rộng phạm vi ở đây.

- F112-1 → [Confirmed] Server theo SOCIAL_SYSTEM §6.2: chỉ `owner` sửa club, tạo mục tiêu/sự kiện, bổ nhiệm; `officer` chỉ duyệt yêu cầu và mời (`403 PERMISSION_DENIED`) — khớp "chỉ owner thấy công cụ quản lý".
- F112-2 → [Confirmed] Học sinh không gỡ thành viên, không lưu trữ club (server `403`); nhân sự trường làm ở portal — khớp "không có trong game".
- F112-3 → [Confirmed] `internal_tournament` không mở cho học sinh (server `422 VALUE_NOT_AVAILABLE`) — khớp bốn loại sự kiện của client.
- F112-4 → [Confirmed] Contract công bố `ClubGoalAcceptedMetric` và `listClubGoals.accepted_metrics` (cùng ba chỉ số); client có thể dựng danh sách từ `accepted_metrics` thay file dữ liệu.
- F112-5 → [Confirmed] `Channel.peer` (thẻ công khai, R16) ở channel `direct`; ánh xạ RAM vẫn là đường dự phòng khi `peer` vắng.
- F112-6 → [Confirmed] `CHAT_NOT_ALLOWED` ngoài giờ chat mang `reopen_at` (REST) / `details.reopen_at` (WS); câu trung tính hiện tại vẫn đúng, giờ mở lại là bổ sung tùy chọn.
- F112-7 → [Confirmed] Bootstrap có `policies.chat.direct_message_enabled` từ bản chiếu policy (vắng khi chưa chiếu cấp platform) — khớp quy tắc nút "Nhắn tin".
- F112-8 → [Confirmed] `highlight` là văn bản thuần + `highlight_ranges` theo điểm mã — khớp "hiển thị văn bản thuần"; client có thể tô theo `highlight_ranges` thay so khớp phía client.
- F112-9 → [Confirmed] Có `updateChannelMembership` + `Channel.notification_level`/`muted`; cài đặt thông báo theo channel trong game vẫn ngoài phạm vi F112 cho tới khi spec này được cập nhật.
- Fixture GdUnit4 (`tools/gen_fixtures.mjs`) cần sinh lại từ example mới (T002); chưa chạy Godot.

### Session 2026-11-28 (triển khai client — [Confirmed — owner "chốt hết"]: điểm còn ngỏ chốt theo phương án an toàn nhất, không mở rộng phạm vi, không đổi Go/OpenAPI)

- Q: Đoạn trích tìm kiếm tô từ khóa theo `highlight_ranges` hay so khớp phía client? → A: Dùng `highlight_ranges` của server khi mọi đoạn nằm trong `highlight`; thiếu/sai thì so khớp không dấu phía client (mỗi từ của `q` là đầu một từ). Đoạn trích luôn là văn bản thuần: bỏ thẻ dạng `<…>`, `[…]` bằng phép quét ký tự (không biểu thức chính quy — ranh giới F119), hiển thị bằng `RichTextLabel.add_text` (BBCode tắt), từ khóa in đậm + gạch chân (không chỉ dựa vào màu). [Confirmed — owner "chốt hết"]
- Q: Danh sách chỉ số mục tiêu lấy từ đâu? → A: Ba chỉ số `ClubGoalAcceptedMetric` **giao** với `listClubGoals.accepted_metrics` (server chỉ có thể thu hẹp, không mở rộng — chỉ số lạ bị bỏ); vắng `accepted_metrics` thì cả ba. [Confirmed — owner "chốt hết"]
- Q: Có hiện giờ mở lại khi `CHAT_NOT_ALLOWED` mang `reopen_at` (F112-6)? → A: Không — giữ câu trung tính "Kênh này đang nghỉ, em quay lại sau nhé" (an toàn nhất, không lộ khung giờ của trường); giới nghiêm hiện giờ kết thúc từ `policies.play_time.curfew` của bootstrap như spec. [Confirmed — owner "chốt hết"]
- Q: Vị trí nút chat HUD? → A: Hàng nút dưới-phải, trước "Về Quảng trường" (không đè toast trên cùng, không đè joystick dưới-trái điện thoại); badge số (≥ 100 → "99+") và badge "@" tĩnh ở góc nút; điện thoại chỉ còn biểu tượng (chữ ở tooltip + tên trợ năng); PC `Enter`/`T` mở khi không ô nào đang giữ bàn phím. [Confirmed — owner "chốt hết"]
- Q: Bố cục `ChatHub`? → A: PC/tablet: khung bên trái cao gần hết màn (40% bề ngang), hàng tab dưới thanh tiêu đề; điện thoại (`compact`): tấm phủ cả màn hình, hàng tab nằm trên dòng thanh tiêu đề (biểu tượng chat, tab sáng — tab đang chọn vàng + ✓ —, kính lúp, "✕ Đóng" đỏ) để giữ "≥ 3 tin". Tab đang mở được cuộn vào tầm nhìn. [Confirmed — owner "chốt hết"]
- Q: Khi channel bị khóa (DM `frozen`, club `read_only`, `CHAT_NOT_ALLOWED`, giới nghiêm, `sendable_kinds` rỗng) ô soạn hiển thị gì? → A: Chỉ băng khóa với câu trung tính; hàng tin nhanh và ô gõ ẩn (nhường chỗ cho tin). `allows(kind)` = `allowed_kinds` ∩ `sendable_kinds` (khi server gửi `sendable_kinds`) — cùng một quy tắc mọi khối. [Confirmed — owner "chốt hết"]
- Q: Tab "Khu vực" hiện khi nào? → A: Khi server liệt kê channel `area` VÀ học sinh đang ở khu hub (HubSync có khu/room) VÀ không có hoạt động toàn khung (`AppState.blocking_activity` rỗng). [Confirmed — owner "chốt hết"]
- Q: Form "Sự kiện club" chọn thời điểm thế nào cho trẻ? → A: Chip ngày (hôm nay + 6 ngày tới, ICT), giờ bằng nút "Sớm hơn"/"Muộn hơn" bước 15 phút, độ dài tùy chọn (Không đặt / 30 / 60 / 120 phút); dòng "Bắt đầu: {ngày} lúc {giờ}" theo locale; "Tạo sự kiện" mờ khi thời điểm đã qua theo đồng hồ server. Mục tiêu: chip chỉ số có hình, số lượng bằng −/+ (bước 5, kẹp 1…100 000). [Confirmed — owner "chốt hết"]
- Q: Telemetry T034? → A: Chỉ `client_error` theo mã lỗi với `scene_code` `chat_dm` / `club_manage` (không nội dung, không id, không nickname); độ trễ mở DM **không** gửi vì danh mục `TelemetryBatch` chưa có loại sự kiện phù hợp (không tự thêm loại — ghi ở báo cáo). [Confirmed — owner "chốt hết"]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Khung chat HUD với các channel bền vững (Priority: P1) 🎯 MVP

An (lớp 3, TH Demo A) đứng ở Quảng trường Tia Sáng, bấm nút chat dưới-trái: thấy tab "Khu vực" (bạn cùng instance), "Cả trường", "CLB Toán" (club của em) và "Bạn bè"; chạm hàng tin nhanh để chào; đi sang Vương quốc Số thì tab "Khu vực" đổi sang khu mới.

**Why this priority**: Đường vào duy nhất cho channel bền vững của F120 (FR-CHAT-001, FR-CHAT-003); DM, tìm kiếm và chat club đều dựng trên khung này.

**Independent Test**: Hai instance Godot `hs.an.3a`, `hs.binh.3a` cùng `main_hub`: cả hai mở khung chat → tab Khu vực có tin của nhau; An vào `learning_zone.math` → tab Khu vực trống của khu mới, không còn tin khu cũ; tenant `common` → không có tab "Cả trường".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F112-AC-01 | KHI học sinh mở chat từ HUD (PC: khung dưới-trái, `Enter`/`T`; điện thoại: nút chat có badge), game client PHẢI mở `ChatHub` với tab dựng từ `listMyChannels`: "Khu vực" (`area`), "Cả trường" (`world`), mỗi club có `chat_channel_id` trùng một club của em (`listClubs` `scope = mine`, F111) và "Bạn bè" (`direct`); PHẢI chỉ có tab cho channel server trả, bỏ qua `channel_type` lạ và PHẢI không có tab theo lớp/khối (ADR-0095) | FR-CHAT-001 · FR-CHAT-003 · [STUDENT_GAME_UX_SOCIAL §7](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md#7-chat) · ADR-0095 | TC-CHAT-002 |
| F112-AC-02 | game client PHẢI dùng lại thành phần chat của F119 cho mọi tab channel bền vững (danh sách ảo hóa, bong bóng, hàng tin nhanh, ô soạn theo `max_text_length`, bảng mặt cười, menu tin theo `Channel.features`, trạng thái gửi theo `chat.message.acked`, phản hồi kiểm duyệt `ui.chat.feedback.*`, Tắt tiếng · Chặn · Báo cáo ≤ 2 thao tác) và PHẢI gửi `createReport` với `context.channel_id` của channel đang mở | FR-CHAT-003 · FR-MOD-002 · F119-AC-08…F119-AC-20 · AC-UX-106 | TC-CHAT-006 |
| F112-AC-03 | KHI nhận `hub.instance.assigned` (vào khu hoặc đổi instance), game client PHẢI làm mới `listMyChannels`, gửi `chat.channel.leave` cho channel `area` cũ, xóa tin của nó khỏi RAM và chỉ hiện channel `area` mới; TRONG KHI không ở hub instance (trận, màn toàn khung), tab "Khu vực" PHẢI ẩn | FR-CHAT-001 · [CHAT_SYSTEM §4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#4-membership) | TC-CHAT-008 |
| F112-AC-04 | game client PHẢI chỉ gửi `chat.channel.join` cho channel bền vững của tab đang mở và gửi `chat.channel.leave` khi đóng khung chat hoặc đổi tab; channel `world`/`area` PHẢI không hiện số chưa đọc (chỉ badge `@` khi có `chat.mention.created`), channel `team` cố định/`direct` PHẢI hiện số chưa đọc từ `listMyChannels`/`chat.unread.updated` | FR-CHAT-003 · [CHAT_SYSTEM §7.1, §8.2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#82-unread-count) | TC-CHAT-039 |
| F112-AC-05 | Badge trên nút chat HUD PHẢI bằng tổng số chưa đọc do server trả (hiển thị "99+" khi vượt), badge `@` riêng; badge PHẢI tĩnh (không nhấp nháy, không âm thanh mặc định) và mất khi đã đọc | FR-CHAT-003 · [SOCIAL_SYSTEM §8](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#8-thông-báo-xã-hội) · AGENTS R19 | TC-CHAT-039 |
| F112-AC-06 | TRONG KHI `AppState.blocking_activity` là bài kiểm tra/thi, game client PHẢI ẩn nút chat HUD và rời mọi channel bền vững đang join; chat gameplay của F119 PHẢI không bị ảnh hưởng | FR-CHAT-003 · [STUDENT_GAME_UX §2](../../docs/05-ux/STUDENT_GAME_UX.md) | — |

---

### User Story 2 — Nhắn tin riêng với bạn bè (Priority: P1)

Dũng (lớp 7) mở hồ sơ bạn CáHeoVui07, bấm "Nhắn tin", trò chuyện riêng; khi bạn ấy chặn Dũng, cuộc trò chuyện chỉ còn đọc được với câu "Cuộc trò chuyện này đang tạm đóng" — không nói ai chặn ai.

**Why this priority**: FR-CHAT-004 (DM chỉ giữa bạn bè không block); kênh riêng là nơi rủi ro an toàn cao nhất nên trạng thái và câu báo phải đúng ngay từ đầu.

**Independent Test**: `hs.dung.7a` và `hs.em.7a` là bạn: Dũng bấm "Nhắn tin" → `openDirectChannel` (`201`), gửi tin → Em thấy ở tab Bạn bè; Em chặn Dũng → tab của Dũng khóa ô nhập với câu trung tính; người không là bạn → không có nút; tenant `common` → không có nút.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F112-AC-07 | KHI học sinh bấm "Nhắn tin" (hồ sơ bạn F110 hoặc danh sách trong tab "Bạn bè"), game client PHẢI gọi `openDirectChannel` (`user_id`; `200` và `201` xử lý như nhau), mở tab DM với nickname + avatar của bạn lấy từ danh sách bạn bè và giữ ánh xạ `channel_id ↔ user_id` chỉ trong RAM; nút PHẢI chỉ hiện với bạn bè hiện tại, khi `feature_flags.social.friends_enabled` và `policies.chat.direct_message_enabled` không `false` | FR-CHAT-004 · FR-SOC-001 · AC-CHAT-005 · AC-SOC-011 · ADR-0078 | TC-CHAT-005 |
| F112-AC-08 | NẾU `openDirectChannel` hoặc lệnh gửi ở channel `direct` trả `403 CHAT_NOT_ALLOWED` hoặc `404`, THÌ game client PHẢI hiện câu chung "Em chưa nhắn riêng được với bạn này lúc này" (không nêu: không còn là bạn, bị chặn, trường tắt, tenant `common`), không tự thử lại và ẩn nút "Nhắn tin" của bạn đó tới hết phiên | FR-CHAT-004 · AC-SOC-004 · AGENTS R16 · [CHILD_SAFETY §3.6](../../docs/10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078) | TC-CHAT-047 |
| F112-AC-09 | KHI channel `direct` có `status = frozen` (từ `listMyChannels`, `openDirectChannel` hoặc `chat.channel.membership_changed`), game client PHẢI khóa ô nhập, hàng tin nhanh và bảng mặt cười với câu "Cuộc trò chuyện này đang tạm đóng", vẫn cho đọc lịch sử, và PHẢI không cho biết ai chặn ai; KHI trở lại `open`, PHẢI mở khóa | FR-SOC-003 · FR-CHAT-004 · [CHAT_SYSTEM §4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#4-membership) | TC-CHAT-005 |
| F112-AC-10 | Tab "Bạn bè" PHẢI liệt kê bạn bè (`listFriends`, F110) kèm số chưa đọc của channel `direct` đã biết người kia; channel `direct` chưa biết người kia PHẢI chỉ cộng vào badge của tab, không hiện tên đoán; game client PHẢI không gọi `openDirectChannel` cho bạn mà học sinh chưa chọn | FR-CHAT-004 · FR-CHAT-003 · AGENTS R16 | TC-CHAT-039 |

---

### User Story 3 — Trạng thái và giới hạn của channel bền vững (Priority: P1)

Ngoài giờ chat của trường, kênh "Cả trường" khóa ô nhập với lời nhẹ nhàng; trường chọn "chỉ tin nhanh + biểu cảm" thì mọi học sinh chỉ thấy hàng tin nhanh; club bị nhà trường khóa thì chỉ đọc; em rời club thì tab club biến mất.

**Why this priority**: Phản ánh đúng quyết định của server (ADR-0020, ADR-0078) là yêu cầu an toàn; không làm thì học sinh gõ rồi bị từ chối mà không hiểu vì sao.

**Independent Test**: Fake server trả lần lượt `CHAT_NOT_ALLOWED`, `CURFEW_ACTIVE`, `sendable_kinds = [quick, emoji]`, `sendable_kinds = []`, club `read_only`, `chat.channel.membership_changed` rời club → scene test kiểm ô nhập/thông điệp từng trường hợp ở cả 4 khối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F112-AC-11 | NẾU gửi ở channel bền vững trả `CHAT_NOT_ALLOWED`, THÌ game client PHẢI gỡ tin chưa gửi khỏi danh sách (không gửi lại tự động), khóa ô nhập của channel đó với "Kênh này đang nghỉ, em quay lại sau nhé" tới lần làm mới `listMyChannels` kế tiếp; NẾU trả `CURFEW_ACTIVE`, THÌ PHẢI khóa mọi ô nhập với giờ kết thúc giới nghiêm từ `policies.play_time.curfew` | FR-CHAT-004 · AC-CHAT-015 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) | TC-CHAT-046 |
| F112-AC-12 | game client PHẢI chỉ hiện điều khiển soạn cho loại tin có trong `sendable_kinds` của channel (vắng thì `allowed_kinds`): chỉ `quick` + `emoji` → ẩn ô gõ chữ, nhãn dán, ảnh động; rỗng → khóa ô soạn như F119-AC-11; quy tắc PHẢI như nhau ở mọi khối (không đọc `grade_band` để quyết định quyền chat) | FR-CHAT-004 · AC-CHAT-015 · [CHILD_SAFETY §3.5](../../docs/10-security/CHILD_SAFETY.md#35-preset-chỉ-quick-message--emote-adr-0078) · AGENTS R15 | TC-CHAT-048 |
| F112-AC-13 | KHI channel club có `status = read_only` (club bị khóa/lưu trữ), game client PHẢI khóa ô soạn với "Club đang tạm khóa, em vẫn đọc được tin cũ"; KHI nhận `chat.channel.membership_changed` cho biết em không còn là thành viên (rời/bị gỡ khỏi club) hoặc lịch sử trả `404`, PHẢI gỡ tab, xóa tin của channel khỏi RAM và không hiển thị lý do | FR-SOC-004 · FR-CHAT-006 · [CHAT_SYSTEM §8.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#83-lịch-sử--đồng-bộ) | TC-CHAT-008 |
| F112-AC-14 | game client PHẢI ẩn các mục menu mà `Channel.features` của channel không bật (World/Area mặc định không có chuyển tiếp, ghim, đang soạn, đã xem) và PHẢI không có danh sách thành viên hay người đang online cho channel `world`/`area` | FR-CHAT-001 · [CHAT_SYSTEM §7.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#71-ma-trận-tính-năng--loại-channel-mặc-định-platform) · AGENTS R16 | TC-CHAT-036 |

---

### User Story 4 — Tìm lại tin trong club và DM (Priority: P2)

Học sinh lớp 6 nhớ bạn hẹn giờ đi boss trong club, bấm kính lúp, gõ "di boss" (không dấu) và nhảy tới tin "đi boss lúc 19h".

**Why this priority**: FR-CHAT-016 (tìm kiếm, P5); cần US1–US2 có channel.

**Independent Test**: Fake `searchMessages` trả 3 kết quả (một có thẻ đánh dấu, một chứa ký tự `[b]`): bảng kết quả hiện văn bản thuần, chạm kết quả → mở tab club cuộn tới tin; `q` 1 ký tự → nút Tìm mờ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F112-AC-15 | KHI học sinh bấm kính lúp trong `ChatHub`, game client PHẢI mở bảng tìm kiếm trên channel bền vững có `features.search` (club, DM; không World/Area), phạm vi "Tất cả" hoặc channel đang mở (`channel_id`), ô nhập có bộ đếm theo giới hạn `q` của contract (nút Tìm mờ khi ngoài giới hạn), gọi `searchMessages` phân trang theo `cursor`; mỗi kết quả PHẢI hiện nickname, avatar, tên kênh (tên club hoặc nickname bạn), thời gian theo locale | FR-CHAT-016 · AC-CHAT-014 · [CHAT_SYSTEM §8.4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#84-tìm-kiếm) | TC-CHAT-040 |
| F112-AC-16 | game client PHẢI hiển thị đoạn trích (`highlight`, thiếu thì `message.body`) là văn bản thuần (bỏ thẻ đánh dấu, không phân tích BBCode/markup), tô từ khóa bằng so khớp không dấu phía client, và PHẢI không lưu, không log, không gửi telemetry chứa `q` hay đoạn trích | FR-CHAT-016 · AGENTS R14 · [CHILD_SAFETY §6](../../docs/10-security/CHILD_SAFETY.md#6-điểm-thực-thi-enforcement-points) | TC-CHAT-040 |
| F112-AC-17 | KHI chạm một kết quả, game client PHẢI mở tab của channel đó, tải lịch sử quanh `seq` của tin (`listChannelMessages` `before_seq`/`after_seq`), cuộn tới và tô sáng tin trong 2 giây; NẾU `404` (không còn là thành viên), THÌ PHẢI hiện "Tin này không còn xem được" và bỏ kết quả khỏi danh sách; kết quả rỗng PHẢI hiện trạng thái rỗng có gợi ý | FR-CHAT-016 · FR-CHAT-006 · [STUDENT_GAME_UX_SOCIAL §2](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md#2-quy-ước-áp-dụng) | TC-CHAT-040 |

---

### User Story 5 — Chủ club sửa thông tin và chọn phó nhóm (Priority: P2)

Dũng là chủ club "Nhà thám hiểm" (do em tạo, nhà trường đã duyệt): đổi mô tả và biểu tượng, chọn bạn MèoXanh42 làm phó nhóm để cùng duyệt yêu cầu tham gia.

**Why this priority**: FR-SOC-004 phía owner; club do học sinh tạo cần người quản lý trong game (SOCIAL_SYSTEM §6.2). Ít người dùng hơn US1–US3.

**Independent Test**: `hs.dung.7a` (owner, club `active`) thấy nút "Quản lý club"; sửa mô tả → `updateClub` với `If-Match` = `version`; fake trả `412` → form tải lại; chọn phó nhóm → `updateClubMemberRole`; `hs.em.7a` (member/officer) không thấy nút.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F112-AC-18 | TRONG KHI `my_role = owner` và club `active`, trang club (F111) PHẢI có nút "Quản lý club" mở màn gồm Sửa thông tin · Phó nhóm · Mục tiêu tuần · Sự kiện club; `officer`/`member` và club `pending_approval`/`locked`/`archived` PHẢI không có nút này; vai trò ở client chỉ quyết định hiển thị — NẾU server trả `403`, THÌ PHẢI ẩn công cụ, làm mới `getClub` và hiện "Em không còn quyền quản lý club này" | FR-SOC-004 · [SOCIAL_SYSTEM §6.2](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#62-thành-viên--vai-trò) · AGENTS R7 | TC-SOC-004 |
| F112-AC-19 | KHI owner lưu "Sửa thông tin", game client PHẢI gửi `updateClub` chỉ với trường đã đổi (tên 2–64 ký tự, mô tả ≤ 500 ký tự có bộ đếm và chặn vượt, biểu tượng chọn từ thư viện của F111) với `If-Match` là `version` của lần đọc gần nhất, và cập nhật trang theo `Club` server trả; NẾU `412`, THÌ PHẢI tải lại `getClub`, điền lại form bằng giá trị mới và hiện "Thông tin club vừa thay đổi, em xem lại rồi lưu nhé" | FR-SOC-004 · ADR-0041 · [SOCIAL_SYSTEM §6.3](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#63-nội-dung-club) | TC-SOC-004 |
| F112-AC-20 | NẾU lưu thông tin, mục tiêu hoặc sự kiện club trả `451 CHAT_CONTENT_BLOCKED`, THÌ game client PHẢI hiện thông điệp kiểm duyệt chung (không nêu từ khớp, không echo nội dung) và giữ form để sửa; `422 VALIDATION_FAILED` PHẢI hiện lỗi cạnh ô tương ứng; `409 CLUB_NOT_APPROVED` PHẢI hiện "Club đang chờ nhà trường duyệt" | AGENTS R15 · [MODERATION §2](../../docs/03-social-multiplayer/MODERATION.md#2-phạm-vi-áp-dụng) · AC-SOC-006 | TC-SOC-004 |
| F112-AC-21 | KHI owner mở "Phó nhóm", game client PHẢI liệt kê thành viên (`listClubMembers`, tải đủ mọi trang; nickname, avatar, vai trò) với "Chọn làm phó nhóm" cho `member` và "Thôi làm phó nhóm" cho `officer` (xác nhận một bước), gọi `updateClubMemberRole` (`role` = `officer`/`member`) và cập nhật dòng theo `ClubMember` server trả; dòng của owner PHẢI không có thao tác; PHẢI không hiện tên thật, trường, khối, điểm hay mastery của thành viên | FR-SOC-004 · AC-SOC-008 · AGENTS R16 | TC-SOC-006 |

---

### User Story 6 — Chủ club tạo mục tiêu tuần và sự kiện club (Priority: P3)

Dũng đặt mục tiêu "cả club hoàn thành 30 hoạt động" tuần này và hẹn "Luyện tập chung thứ Bảy 9:00"; thành viên thấy ở trang club (F111).

**Why this priority**: Nội dung club (SOCIAL_SYSTEM §6.3); club vẫn dùng được khi chưa có công cụ này.

**Independent Test**: `hs.dung.7a` tạo mục tiêu → `createClubGoal` có `Idempotency-Key` (gửi lại cùng key khi mất mạng → một mục tiêu); tạo sự kiện thời điểm đã qua → nút Tạo mờ; fake `451` → giữ form.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F112-AC-22 | KHI owner tạo mục tiêu tuần, game client PHẢI cho chọn `metric` bằng hình + chữ trong ba chỉ số `activities_completed`, `quests_completed`, `discoveries_found`, `target` bằng nút −/+ trong giới hạn của contract, tiêu đề tùy chọn ≤ 128 ký tự, gửi `createClubGoal` với `Idempotency-Key` giữ nguyên khi thử lại, rồi làm mới `listClubGoals`; PHẢI không có đóng góp từng người khác, xếp hạng hay phần thưởng mua được | FR-SOC-004 · AC-SOC-008 · AGENTS R19 · [SOCIAL_SYSTEM §6.3](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#63-nội-dung-club) | TC-SOC-007 |
| F112-AC-23 | KHI owner tạo sự kiện club, game client PHẢI cho chọn `event_kind` trong `practice_session`, `coop_quest`, `boss_raid`, `other` (không `internal_tournament`), tiêu đề 1–128 ký tự, mô tả ≤ 1000 ký tự, ngày + giờ bắt đầu (giờ ICT, không cho chọn thời điểm đã qua theo đồng hồ server), giờ kết thúc tùy chọn sau giờ bắt đầu; gửi `createClubEvent` với `Idempotency-Key` giữ khi thử lại, rồi làm mới `listClubEvents` | FR-SOC-004 · [SOCIAL_SYSTEM §6.3](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#63-nội-dung-club) · ADR-0041 | TC-SOC-004 |
| F112-AC-24 | Thời điểm sự kiện club và mục tiêu PHẢI hiển thị dạng ngày + giờ theo locale (không đếm ngược giờ/phút/giây, không nhấp nháy, không câu "chỉ còn…"); KHI tạo xong, PHẢI hiện toast thành công 2 giây và PHẢI không gửi thông báo ngoài game | AGENTS R19 · NFR-SAFE-003 · [SOCIAL_SYSTEM §8](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#8-thông-báo-xã-hội) | TC-EVT-009 |

---

### User Story 7 — Song ngữ, giao diện theo khối và điện thoại, hiệu năng, ranh giới an toàn (Priority: P2)

**Why this priority**: Bắt buộc theo constitution (VI, VIII, IX) và chuẩn giao diện học sinh (F114, F125); kiểm sau luồng chính.

**Independent Test**: Ma trận khối (`tests/scene/test_grade_band_matrix.gd`) cho mọi màn mới; pseudo-locale `xx-PS`; ảnh chụp 4 khối × 4 cửa sổ; quét mã `src/game/` (NATS/Valkey, `grade_band` trong code chat, BBCode).

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F112-AC-25 | Mọi chuỗi UI mới PHẢI qua khóa `ui.chat.hub.*`, `ui.chat.dm.*`, `ui.chat.search.*`, `ui.club.manage.*`, `ui.error.<CODE>` (vi/en, không ghép câu bằng code); ngày giờ theo ICT, định dạng theo locale; bố cục PHẢI chịu chuỗi dài hơn 35% (pseudo-locale) không cắt, không tràn | NFR-I18N-001 · NFR-I18N-003 · NFR-I18N-004 · ADR-0056 | TC-I18N-006 |
| F112-AC-26 | game client PHẢI lấy mọi màu/cỡ chữ/khoảng cách từ Theme sinh từ `design/game-tokens/` theo phong cách tầng của F114 (nút bấm được tầng 1 viền mực + cạnh dưới, khung thông tin tầng 2 viền mảnh, chip đang chọn vàng + ✓); mỗi cửa sổ mới PHẢI có đúng một nút "✕ Đóng" đỏ ở phải thanh tiêu đề, hộp xác nhận giữ "Hủy" trung tính; trạng thái (chưa đọc, đã tắt thông báo, khóa, phó nhóm) PHẢI có chữ hoặc biểu tượng, không chỉ màu | NFR-A11Y-001 · ADR-0047 · AGENTS R32 · F114-AC-02 · F125-AC-06 | TC-UI-001, TC-UI-003 |
| F112-AC-27 | TRONG chế độ `compact` (F125), `ChatHub`, bảng tìm kiếm và màn Quản lý club PHẢI là tấm phủ cao hết màn hình, vùng chạm ≥ `density.compact.button.min_height`, chữ ≥ 12 CSS px và vùng chạm ≥ 40 CSS px vật lý ở 844×390 và 740×360, form cuộn dọc khi không vừa (không thu chữ dưới token khối); mỗi tab channel bền vững PHẢI giữ quy tắc "≥ 3 tin" (`chat_list_min_visible`) như F119 | FR-GAME-010 · NFR-A11Y-001 · F125-AC-03 · F125-AC-05 · [STUDENT_GAME_UX_SOCIAL §7](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md#7-chat) | TC-UI-003 |
| F112-AC-28 | Ma trận khối PHẢI dựng mọi màn mới (`ChatHub` mỗi loại tab, DM đang mở và `frozen`, bảng tìm kiếm có kết quả và rỗng, màn Quản lý club, form sửa club, Phó nhóm, form mục tiêu, form sự kiện) × 4 khối (`g1_3`, `g4_5`, `g6_7`, `g8_9`) × cửa sổ 1280×720, 1024×600, 844×390, 740×360 × `vi`/`en`/pseudo và PHẢI pass probe không cắt chữ, không tràn, nút ≥ ngưỡng, probe vật lý (kiểm ngược như F125-AC-04); ảnh trước/sau PHẢI được chụp cho mỗi màn mới ở 844×390 và 740×360 × 4 khối `vi`, 844×390 `en` `g4_5`, 1280×720 `vi` `g1_3`/`g8_9`, xem từng ảnh và ghi bảng màn × khối trong quickstart §5 | FR-GAME-010 · F125-AC-04 · F125-AC-08 · [DESIGN_SYSTEM §9](../../docs/05-ux/DESIGN_SYSTEM.md#9-đảm-bảo-chất-lượng) | TC-UI-003, TC-I18N-006 |
| F112-AC-29 | `ChatHub` và màn Quản lý club PHẢI giữ ≥ 30 FPS trên điện thoại tầm trung preset Trung bình khi channel `world` nhận tin dồn dập: danh sách ảo hóa, giữ tối đa `chat_recent_buffer` tin/channel, gộp cập nhật badge/typing ≤ 1 lần/giây, thế giới nền giảm tần số cập nhật khi khung chat che màn | NFR-COMP-001 · [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-009 |
| F112-AC-30 | game client PHẢI chỉ dùng WebSocket `/realtime/v1` và REST `/api/v1/chat/*`, `/api/v1/social/*` qua `ApiClient` (gắn `X-Tenant-Id` của phiên), không kết nối NATS/Valkey, không chứa danh mục hay logic kiểm duyệt, không có nhánh code chat đọc `grade_level`/`grade_band` (khối chỉ đổi density/Theme), hiển thị mọi chuỗi do người dùng hoặc server gửi là văn bản thuần (không BBCode) | FR-CHAT-004 · NFR-SAFE-001 · AGENTS R1 · AGENTS R15 · AGENTS R25 | TC-CHAT-010 |
| F112-AC-31 | Mọi màn của F112 PHẢI chỉ hiển thị nickname, avatar, level, vai trò club của người khác (không tên thật, trường, khối); store DM/tìm kiếm/ánh xạ bạn chỉ trong RAM, xóa khi đăng xuất hoặc `session.session.replaced`; NẾU nhận `403 GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH`, THÌ PHẢI theo luồng F061; `404` của club/channel PHẢI đóng màn liên quan và làm mới, không hiển thị chi tiết | AGENTS R16 · AGENTS R23 · ADR-0064 · ADR-0065 | TC-AUTH-014 |

### Edge Cases

- NẾU học sinh đổi khu khi tab "Khu vực" đang mở, THÌ áp F112-AC-03: tab giữ vị trí, nội dung đổi sang channel `area` mới; tin đang soạn giữ lại trong ô nhập, tin chờ gửi của channel cũ bị bỏ.
- NẾU server cũ còn trả channel `class`/`study_group`, THÌ bỏ qua như F119 (ADR-0095).
- NẾU học sinh vừa chặn bạn trong DM (menu Chặn của F119), THÌ tin của bạn ẩn ngay; trạng thái `frozen` đến từ server sau đó (F112-AC-09).
- NẾU bạn hết là bạn bè (hủy kết bạn), THÌ DM vẫn đọc được theo retention; gửi bị từ chối → F112-AC-08.
- NẾU học sinh là owner bị nhà trường khóa club trong lúc đang sửa, THÌ lần lưu nhận `403`/`409` → F112-AC-18, F112-AC-20; lần làm mới hiện chế độ chỉ xem (F111-AC-24).
- NẾU nhận cùng `chat.unread.updated` hai lần, THÌ thay giá trị (không cộng dồn).
- NẾU mất mạng khi tạo mục tiêu/sự kiện, THÌ thử lại với cùng `Idempotency-Key` (một bản ghi); băng vàng "Đang kết nối lại…" và nút Tạo khóa trong lúc chờ.
- NẾU deep link thông báo (F123) trỏ tới tin/club không còn, THÌ áp F112-AC-31 (`404`).
- NẾU học sinh `g1_3` mở khung chat, THÌ cùng tab và quyền như mọi khối; chỉ khác cỡ chữ/nút và bảng mặt cười mở sẵn ngoài `compact` (F119-AC-08).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHAT-001 | Tab client cho channel `area`, `world`, `team` cố định (club), `direct` mà server trả; không tab lớp/khối | [CHAT_SYSTEM §2, §4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-003 | Gửi/nhận, lịch sử, unread, badge HUD cho channel bền vững (dùng lại F119) | [CHAT_SYSTEM §8](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#8-lịch-sử-receipts-unread-tìm-kiếm) |
| FR-CHAT-004 | DM chỉ với bạn bè; một bộ quy tắc cho mọi khối; câu báo trung tính khi bị từ chối | [CHAT_SYSTEM §1, §3.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-006 (phần hiển thị) | Lịch sử theo quyền: rời club/không còn thành viên thì gỡ tab | [CHAT_SYSTEM §6, §8.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-016 (phần tìm kiếm) | Bảng tìm kiếm club + DM, có/không dấu | [CHAT_SYSTEM §8.4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md#84-tìm-kiếm) |
| FR-SOC-003 (phần hiển thị) | DM `frozen` khi có block, không lộ ai chặn | [SOCIAL_SYSTEM §5](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#5-block-mute-report-điểm-vào) |
| FR-SOC-004 (phần owner) | Sửa club, phó nhóm, mục tiêu tuần, sự kiện club | [SOCIAL_SYSTEM §6.2–§6.3](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md#62-thành-viên--vai-trò) |
| FR-MOD-002 (phần client) | Báo cáo tin/người từ channel bền vững với `context.channel_id` | [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-GAME-010 | Bố cục theo khối và điện thoại (`compact`) | [STUDENT_GAME_UX_SOCIAL §2, §6–§7](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) |

NFR: NFR-I18N-001, NFR-I18N-003, NFR-I18N-004, NFR-A11Y-001, NFR-COMP-001, NFR-SAFE-001, NFR-SAFE-003.

### Key Entities

- **ChatHub**: khung chat HUD cho channel bền vững: danh sách tab theo channel server trả, tab đang mở, badge tổng.
- **PersistentChannelTab**: một channel bền vững trên client (loại, tên hiển thị — tên club hoặc nickname bạn, trạng thái `open`/`read_only`/`frozen`, số chưa đọc, có join hay không).
- **DirectPeerMap**: ánh xạ `channel_id ↔ bạn` (thẻ công khai) chỉ trong RAM của phiên.
- **SearchQuery / SearchHit**: truy vấn tìm kiếm và kết quả (thẻ người gửi, kênh, thời gian, đoạn trích văn bản thuần).
- **ClubManageState**: dữ liệu màn Quản lý club (club + `version`, thành viên và vai trò, form mục tiêu/sự kiện đang soạn, khóa idempotency đang chờ).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `chat_recent_buffer` | Giới hạn tin trong RAM mỗi channel bền vững |
| `chat_list_min_visible` | Quy tắc "≥ 3 tin" ở mọi tab |
| `chat_allowed_hours`, `platform_curfew` | Chỉ để hiểu lỗi `CHAT_NOT_ALLOWED`/`CURFEW_ACTIVE` (server kiểm; giờ kết thúc giới nghiêm lấy từ bootstrap) |
| `world_chat_scope`, `common_tenant_social_defaults` | Chỉ để hiểu vì sao server không trả World/DM (client không đọc giá trị) |
| `persistent_chat_retention_days`, `ephemeral_chat_retention` | Chỉ để hiểu lịch sử còn/không còn (client không hiện số ngày) |
| `club_max_members`, `clubs_per_student_max`, `student_club_creation` | Chỉ hiển thị (giá trị trong `Club` và lỗi server) |
| `compact_short_side`, `compact_logical_height`, `compact_density_floors`, `compact_physical_floor` | Chế độ điện thoại (F125) |
| `client_reconnect_backoff`, `supported_locales` | Kết nối lại, ngôn ngữ UI |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic trong UI; quyền/trạng thái chat do server quyết định, client chỉ hiển thị | AGENTS R1, R7 · CHILD_SAFETY §6 |
| Một bộ quy tắc chat cho mọi khối; mọi tin qua pipeline server | AGENTS R15 · ADR-0020 |
| Chỉ nickname + avatar; không lộ ai chặn ai, không lộ lý do chính sách | AGENTS R16 · ADR-0054 · ADR-0078 |
| Client không kết nối NATS/Valkey | AGENTS R25 · ADR-0035 |
| Không dark pattern (đếm ngược, nhấp nháy, thông báo ngoài game) | AGENTS R19 · NFR-SAFE-003 |
| Theme/token, phong cách tầng F114, `compact` F125, song ngữ | AGENTS R32 · ADR-0047 · ADR-0056 |
| GDScript static typing, test GdUnit4 | AGENTS §7 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh mở được một channel bền vững từ HUD và gửi tin nhanh trong ≤ 2 thao tác ở mọi khối.
- **SC-002**: 0 màn trong bộ scene test hiển thị tên thật, trường, khối của người khác, hoặc câu cho biết ai đã chặn ai / vì sao bị từ chối.
- **SC-003**: Học sinh lớp 6–9 tìm lại được một tin cũ trong club bằng từ khóa không dấu trong ≤ 4 thao tác.
- **SC-004**: Chủ club đổi phó nhóm hoặc tạo mục tiêu tuần trong ≤ 5 thao tác từ trang club.
- **SC-005**: `ChatHub` ≥ 30 FPS trên thiết bị Android tầm trung tham chiếu, preset Trung bình, khi channel World nhận 10 tin/giây trong 60 giây.
- **SC-006**: 100% màn mới pass ma trận khối 4 khối × 4 cửa sổ × 3 ngôn ngữ và có ảnh đã xem trong quickstart §5.

## Ngoài phạm vi

- Backend channel bền vững, policy, DM, club, tìm kiếm, retention → F120; kiểm duyệt → F116; báo cáo/case → F121; mọi delta contract của câu hỏi mở F112-1…F112-9 → feature backend tương ứng (F120/F095/F015), không thiết kế ở đây.
- Thành phần chat dùng chung và channel gameplay → F119; tab Club, trang club, tạo/xin vào/lời mời club, duyệt yêu cầu và mời của owner/officer → F111; danh sách bạn, hồ sơ bạn, chặn/tắt tiếng → F110; hộp thông báo, deep link → F123.
- Gỡ thành viên, giải tán/lưu trữ club bởi học sinh; chuyển quyền chủ club; sửa/xóa mục tiêu hoặc sự kiện đã tạo (contract chưa có); sự kiện `internal_tournament` → chưa làm (Clarifications, câu hỏi mở F112-2, F112-3).
- Cài đặt thông báo theo channel (`notification_level`) → chờ contract (câu hỏi mở F112-9).
- Voice chat: không có (FR-CHAT-007, ADR-0070). Kênh lớp/nhóm học tập/khối: không có (ADR-0095).

## Assumptions

- F111 cung cấp `ClubService` (`listClubs`, `getClub`, `listClubMembers`, `listClubGoals`, `listClubEvents`) và trang club; F110 cung cấp `SocialService.list_friends` và `FriendProfilePanel`; F112 thêm vào đó, không viết lại. Nếu F110/F111 chưa xong khi bắt đầu F112, làm F110/F111 trước (thứ tự phụ thuộc CODE_PLAN).
- F120, F121 đã Done: client gọi backend thật; không tạo Prism mock (R17). Test dùng fake + fixture sinh từ example contract, gắn nhãn TEST DOUBLE.
- Câu hỏi mở cho backend/tài liệu (không chặn Ready vì có đường dự phòng): F112-1 quyền `officer` (SOCIAL_SYSTEM §6.2 ↔ contract/server), F112-2 gỡ thành viên/lưu trữ bởi học sinh, F112-3 danh sách template giải nội bộ, F112-4 danh sách `metric` hợp lệ, F112-5 `peer` trong `Channel` loại `direct`, F112-6 `reopen_at` trong lỗi `CHAT_NOT_ALLOWED`, F112-7 identity điền `policies.chat.direct_message_enabled`, F112-8 định dạng đánh dấu của `highlight`, F112-9 operation đặt `notification_level` — chi tiết ở [contracts §6](contracts/README.md).
