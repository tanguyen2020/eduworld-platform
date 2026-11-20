# Feature Specification: F119 — Giao diện chat trong Godot: panel, quick message, tab, tắt tiếng, báo cáo

**Feature Branch**: `119-godot-chat-ui` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/game/` · **Mốc**: M4b · **Phụ thuộc**: F092, F117
**Input**: Giao diện chat của học sinh trong game Godot cho channel gameplay (party lobby, trận 1v1, trận đội, boss): panel luôn có (thu gọn được, không tắt), hàng quick message một chạm, tab "Đội"/"Cả trận", menu Tắt tiếng/Chặn/Báo cáo trên từng tin và từ màn hình kết quả trận, bảng emoji/sticker/GIF, menu thao tác tin (trả lời, trích dẫn, chuyển tiếp, sao chép, ghim, sửa, thu hồi, cảm xúc), trạng thái gửi/nhận/xem, phản hồi kiểm duyệt thân thiện. FR nguồn: FR-CHAT-003, FR-CHAT-010, FR-CHAT-014.

## Clarifications

### Session 2026-11-16

- Q: `createReport` do F121 (M5) làm, F119 ở M4b? → A: F119 gọi `createReport` theo contract có sẵn; trước khi F121 Done chạy trên Prism mock gắn nhãn MOCK (CODE_PLAN §2, R17); F119 chỉ `Done` khi chạy với backend thật.
- Q: Panel chat ngoài trận (HUD, tab Khu vực/Lớp/Bạn bè, tìm kiếm)? → A: Các channel bền vững và tìm kiếm thuộc F120; F119 dựng thành phần dùng chung (`ChatPanel`, `ChatStore`, `ChatService`) và áp cho channel gameplay; tab channel bền vững bật khi F120 Done.
- Q: STUDENT_GAME_UX_SOCIAL §7 vẽ tin ghim "Nhà trường: …" trong chat? → A: Không làm: nhân sự không là thành viên channel (ADR-0072, CHAT_SYSTEM §2); thông báo nhà trường đi qua bảng tin lớp. Cần sửa wireframe.
- Q: Client có ẩn/che tin theo danh mục không? → A: Không. Mọi quyết định kiểm duyệt ở server; client chỉ hiển thị kết quả (CHILD_SAFETY §6).

### Session 2026-11-17

- Q: Tên khóa idempotency tin nhắn (RB-508)? → A: `client_message_id` (một tên cho REST, realtime, DB) thay `client_msg_id`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Panel chat luôn có trong party và trận (Priority: P1)

Vào party lobby hay trận đấu, học sinh thấy ngay khung chat; có thể thu gọn để tập trung vào câu hỏi nhưng không tắt được; trận đội có tab Đội và Cả trận; mất mạng thì tin chờ gửi lại.

**Why this priority**: FR-CHAT-010 và ADR-0020 mục 6 (chat gameplay luôn có) — mọi tương tác chat trong M4b đi qua panel này.

**Independent Test**: Chạy Godot với backend thật (hoặc Prism mock có nhãn) cho 2 client: tạo party → panel có ngay; thu gọn → tin mới hiện badge; team battle → 2 tab; ngắt mạng client A, gửi tin → biểu tượng đồng hồ, có mạng lại tự gửi đúng một lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F119-AC-01 | KHI vào party lobby, trận 1v1, trận đội hoặc boss co-op, game client PHẢI hiển thị panel chat của channel gameplay tương ứng; người chơi PHẢI thu gọn/ẩn được panel nhưng không có tùy chọn tắt chat, và tin mới khi thu gọn PHẢI hiện badge | FR-CHAT-010 · AC-CHAT-008 · [CHAT_SYSTEM §4.2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-017 |
| F119-AC-02 | KHI ở trận đội, game client PHẢI có hai tab "Đội" (channel `team` của mình) và "Cả trận" (channel `match`) với badge chưa đọc riêng | FR-CHAT-010 · [STUDENT_GAME_UX_SOCIAL §7](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | TC-CHAT-017 |
| F119-AC-03 | TRONG KHI câu hỏi đang hiển thị ở Knowledge Battle hoặc boss, game client PHẢI tự thu nhỏ panel để không che vùng câu hỏi/đáp án (mobile: dải 2 dòng), vẫn nhận và lưu tin | FR-CHAT-010 · AC-UX-108 | — |
| F119-AC-04 | game client PHẢI sắp tin theo `seq`, hiển thị trạng thái tin của mình chờ gửi → đã gửi (chỉ khi nhận `chat.message.acked`) → đã nhận/đã xem; KHI phát hiện khoảng trống `seq` PHẢI gửi `chat.channel.sync`, quá `chat_recent_buffer` tin thì tải lịch sử REST, và khoảng trống không bù được PHẢI chỉ thử một lần | FR-CHAT-003 · [GODOT_ARCHITECTURE §6.1](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-CHAT-028 |
| F119-AC-05 | NẾU mất kết nối hoặc nhận `CHAT_TEMPORARILY_UNAVAILABLE`, THÌ game client PHẢI giữ tin ở trạng thái chờ (biểu tượng đồng hồ), hiện "Đang kết nối lại chat…", và gửi lại với cùng `client_message_id` theo `client_reconnect_backoff` mà không tạo tin trùng | FR-CHAT-003 · AC-CHAT-011 | TC-CHAT-026 |
| F119-AC-06 | KHI channel gameplay chưa sẵn sàng ngay sau khi trận/party bắt đầu (`NOT_FOUND` khi join), game client PHẢI thử join lại theo `client_reconnect_backoff`; KHI channel đã đóng (`CHAT_CHANNEL_CLOSED`) PHẢI khóa ô nhập và hiện thông báo thời hạn lưu tính từ `purge_after` | FR-CHAT-003 · FR-CHAT-010 | TC-CHAT-020 |
| F119-AC-07 | KHI xem lịch sử trận/party, game client PHẢI hiển thị "Tin nhắn trong trận được giữ {N} ngày" với N lấy từ dữ liệu server, không hardcode | FR-CHAT-010 · [CHAT_SYSTEM §4.2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-021 |

---

### User Story 2 — Quick message một chạm và phản hồi kiểm duyệt thân thiện (Priority: P1)

Học sinh lớp 2 chạm "Chúc may mắn!" mà không cần gõ; bạn chọn tiếng Anh thấy câu tiếng Anh. Tin bị chặn hiện lời nhắc dễ hiểu, không nói từ nào sai.

**Why this priority**: US-CHAT-001; trẻ nhỏ chat chủ yếu bằng quick message; phản hồi giáo dục là yêu cầu an toàn.

**Independent Test**: Hàng quick message luôn hiển thị trên ô nhập; chạm → gửi `kind = quick`; đổi locale người nhận `en` → hiển thị bản `en`; server trả `CHAT_CONTENT_BLOCKED` (pii) → hiện thông điệp khóa `ui.chat.feedback.pii_blocked`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F119-AC-08 | game client PHẢI luôn hiển thị hàng quick message (dữ liệu từ `listQuickMessages`) phía trên ô nhập, gửi bằng một chạm với `quick_message_id`, và hiển thị câu theo ngôn ngữ của người xem (`text.vi`/`text.en`); với `g1_3` hàng quick message PHẢI to hơn và bảng emoji mở mặc định | FR-CHAT-003 · FR-CHAT-010 · US-CHAT-001 · ADR-0056 | TC-I18N-008 |
| F119-AC-09 | game client PHẢI đếm ký tự theo `max_text_length` của channel, `Enter` gửi và `Shift+Enter` xuống dòng trên PC, giữ `client_message_id` cho mỗi tin | FR-CHAT-003 · [STUDENT_GAME_UX_SOCIAL §7](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | — |
| F119-AC-10 | KHI nhận quyết định kiểm duyệt (`masked`, `CHAT_CONTENT_BLOCKED` theo `category_code`, `chat.message.held`, `CHAT_RATE_LIMITED`), game client PHẢI hiển thị đúng thông điệp của bảng CHAT_SYSTEM §4.3 qua khóa `ui.chat.feedback.*`, và PHẢI không hiển thị từ đã khớp hay vị trí khớp | FR-CHAT-010 · AC-UX-106 · AGENTS R15 | TC-CHAT-015 |
| F119-AC-11 | KHI nhận `chat.sanction.applied` hoặc `CHAT_MUTED`, game client PHẢI khóa ô nhập kèm thời điểm mở lại, vẫn cho đọc tin và vẫn ở trong trận/party | FR-CHAT-010 · [CHAT_SYSTEM §4.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-019 |

---

### User Story 3 — Tắt tiếng, chặn, báo cáo trong ≤ 2 chạm (Priority: P1)

Bị nói khó nghe trong trận, học sinh nhấn giữ tin → Báo cáo → chọn lý do bằng biểu tượng; hoặc chặn ngay. Sau trận vẫn báo cáo được từ màn hình kết quả.

**Why this priority**: Biện pháp an toàn thay thế cho việc không tắt được chat gameplay (CHILD_SAFETY §3.2b); US-CHAT-002.

**Independent Test**: Trên tin của người khác: nhấn giữ (mobile)/chuột phải (PC) → menu có Tắt tiếng, Chặn, Báo cáo; Báo cáo → chọn lý do → gửi `createReport` có `match_id`; đếm thao tác ≤ 2 tới khi mở hộp báo cáo.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F119-AC-12 | game client PHẢI cho mọi tin của người khác có menu Tắt tiếng · Chặn · Báo cáo mở được trong ≤ 2 thao tác (nhấn giữ trên mobile, chuột phải hoặc nút ⋯ trên PC); tin của chính mình PHẢI không có ba mục này | FR-CHAT-010 · AC-MOD-023 · AC-UX-106 | TC-CHAT-019 |
| F119-AC-13 | KHI học sinh gửi báo cáo, game client PHẢI cho chọn `reason` bằng biểu tượng (`rude_language`, `bullying`, `asked_personal_info`, `spam`, `inappropriate_content`, `cheating`, `other`), gọi `createReport` với `Idempotency-Key` và ngữ cảnh `match_id`/`party_id`, rồi hiện lời cảm ơn kèm gợi ý Chặn | FR-CHAT-010 · US-CHAT-002 · [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-006 |
| F119-AC-14 | KHI ở màn hình kết quả trận hoặc lịch sử party (trong thời hạn lưu), game client PHẢI có nút "Báo cáo người chơi" và "Chặn" đối thủ/thành viên | FR-CHAT-010 · AC-UX-110 | TC-CHAT-019 |
| F119-AC-15 | KHI học sinh chặn hoặc tắt tiếng một người, game client PHẢI gọi `blockUser`/`muteUser` và ẩn ngay tin của người đó trong panel (server vẫn là nơi thực thi) | FR-CHAT-010 · AC-CHAT-005 | TC-SOC-003 |

---

### User Story 4 — Emoji, sticker, GIF và thao tác trên tin (Priority: P2)

Học sinh mở bảng mặt cười chọn emoji/sticker/GIF, thả cảm xúc, trả lời, trích dẫn, chuyển tiếp, sao chép, ghim, sửa, thu hồi — mục nào hiện theo quyền và loại channel.

**Why this priority**: FR-CHAT-014 và bộ tính năng ADR-0034 trên client; không ảnh hưởng an toàn nếu chậm hơn US1–US3.

**Independent Test**: Picker 3 tab: pack khóa hiện ổ khóa; GIF chỉ tìm theo chủ đề trong thư viện nội bộ; thu hồi luôn hỏi lại; sao chép một tin đã che → clipboard chỉ có bản đã che.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F119-AC-16 | game client PHẢI có bảng 3 tab Emoji/Sticker/GIF: sticker theo `listStickerPacks` (pack khóa hiện ổ khóa + gợi ý cách mở), GIF theo chủ đề từ `listGifs`, và PHẢI không có ô tìm kiếm Internet hay URL ngoài CDN EduWorld | FR-CHAT-014 · AC-CHAT-013 | TC-CHAT-033 |
| F119-AC-17 | game client PHẢI phát tối đa 3 ảnh động cùng lúc, chỉ khi trong khung nhìn, và cache asset theo version | FR-CHAT-014 · [STUDENT_GAME_UX_SOCIAL §7](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | TC-PERF-019 |
| F119-AC-18 | game client PHẢI hiển thị hàng 6 emoji cảm xúc nhanh, gửi `chat.reaction.toggle`, hiển thị số đếm từ `chat.reaction.updated` và báo lỗi thân thiện khi vượt giới hạn | FR-CHAT-014 · FR-CHAT-003 | TC-CHAT-034 |
| F119-AC-19 | game client PHẢI hiển thị các mục Trả lời, Trích dẫn, Chuyển tiếp, Sao chép, Ghim, Sửa, Thu hồi theo `Channel.features` và quyền (sửa/thu hồi chỉ tin của mình trong thời hạn); Thu hồi PHẢI hỏi xác nhận; Sao chép PHẢI chép nội dung đang hiển thị (bản đã che hoặc placeholder) | FR-CHAT-010 · FR-CHAT-014 · AC-CHAT-012 | TC-CHAT-041 |
| F119-AC-20 | game client PHẢI hiển thị nhãn "đã chỉnh sửa", "Đã chuyển tiếp từ {nickname}", placeholder thu hồi, "Tin nhắn gốc không còn" cho reply/quote, typing tối đa 3 nickname và ✓/✓✓/✓✓ xanh theo `chat.receipt.updated` | FR-CHAT-003 · AC-CHAT-014 | TC-CHAT-038 |

---

### User Story 5 — Hiệu năng, song ngữ, theme và ranh giới an toàn của client (Priority: P2)

Chat chạy mượt trên máy tầm trung, đủ tiếng Việt/Anh, không chứa logic kiểm duyệt hay kết nối hạ tầng nội bộ.

**Why this priority**: Ràng buộc chung (R1, R25, R32) cho mọi màn chat.

**Independent Test**: Gửi 500 tin vào một channel → FPS trong ngân sách thiết bị tầm trung; pseudo-locale không tràn chữ; quét mã không có kết nối NATS/Valkey, không chuỗi hardcode.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F119-AC-21 | game client PHẢI ảo hóa danh sách tin (chỉ tạo bubble trong khung nhìn), giữ tối đa `chat_recent_buffer` tin trong bộ nhớ mỗi channel, gộp cập nhật typing/receipt ≤ 1 lần/giây, và giữ FPS trong ngân sách thiết bị tầm trung khi chat trong trận | FR-CHAT-003 · [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-009 |
| F119-AC-22 | game client PHẢI lấy mọi chuỗi UI chat từ khóa dịch `ui.chat.*` (vi/en), mọi màu/cỡ chữ từ Theme sinh từ `design/game-tokens/`, không ghép câu bằng code | FR-CHAT-010 · AGENTS R32 · NFR-I18N-004 | TC-I18N-006 |
| F119-AC-23 | game client PHẢI chỉ dùng WebSocket `/realtime/v1` và REST `/api/v1/chat/*`, không kết nối NATS/Valkey, không chứa danh mục hay logic kiểm duyệt, và không có tùy chọn tắt chat gameplay | FR-CHAT-010 · AGENTS R1 · AGENTS R25 | — |

### Edge Cases

- NẾU người chơi đổi thiết bị giữa trận (takeover), THÌ client mới join lại channel và `chat.channel.sync` từ `last_seen_seq` = 0 tải lịch sử.
- NẾU tin của người bị chặn nằm trong reply/quote, THÌ hiển thị placeholder thay nội dung.
- NẾU asset sticker/GIF tải lỗi, THÌ hiển thị `alt_text` theo locale.
- NẾU bàn phím ảo mobile mở, THÌ panel đẩy lên, không che hàng quick message.
- NẾU locale đổi khi đang chat, THÌ quick message/tin hệ thống đổi ngôn ngữ ngay; tin tự do giữ nguyên văn.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHAT-003 | Hiển thị text, emoji, reaction, quick message, lịch sử, unread, trạng thái tin trong channel gameplay | [CHAT_SYSTEM §4.3, §8](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-010 | Panel thu gọn không tắt, quick message một chạm, tab Đội/Cả trận, menu Tắt tiếng/Chặn/Báo cáo, báo cáo từ màn kết quả | [CHAT_SYSTEM §4.2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · [STUDENT_GAME_UX_SOCIAL §7](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) |
| FR-CHAT-014 | Bảng emoji/sticker/GIF, reaction trên client | [CHAT_SYSTEM §7.5–§7.6](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |

### Key Entities

- **ChatStore**: trạng thái chat trong RAM theo channel (tin sắp theo `seq`, tin chờ gửi, con trỏ, unread, mention).
- **ChatPanel**: khung chat gắn với ngữ cảnh party/trận, có tab và trạng thái thu gọn.
- **QuickMessageCatalog**: danh mục câu soạn sẵn song ngữ tải từ server.
- **ChatAssetCache**: cache sticker/GIF theo version.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `client_reconnect_backoff` | Thử join/gửi lại khi mất kết nối |
| `client_command_timeout` | Hết thời gian chờ phản hồi lệnh WS |
| `chat_recent_buffer` | Số tin tối đa giữ trong bộ nhớ/bù qua `chat.channel.sync` |
| `supported_locales` | Ngôn ngữ hiển thị quick message/tin hệ thống |
| `chat_reactions_per_user` | Hiển thị lỗi khi vượt giới hạn reaction |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic kiểm duyệt trong scene/script UI | AGENTS R1 · CHILD_SAFETY §6 |
| Client không kết nối NATS/Valkey | AGENTS R25 |
| Theme/token từ `design/game-tokens/`, chuỗi qua gettext | AGENTS R32 · ADR-0047 · ADR-0056 |
| GDScript static typing, test GdUnit4 | AGENTS §7 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh mở được hộp báo cáo/chặn từ một tin trong ≤ 2 thao tác ở 100% màn có chat gameplay.
- **SC-002**: Học sinh lớp 1–3 gửi được quick message trong ≤ 1 chạm trong test khả dụng với 5 học sinh.
- **SC-003**: 0 tin trùng sau 20 lần ngắt/nối mạng khi đang gửi.
- **SC-004**: FPS khi chat trong trận đạt ngân sách thiết bị tầm trung tham chiếu (PERFORMANCE_BUDGET).

## Ngoài phạm vi

- Backend gửi/nhận, kiểm duyệt, channel gameplay, tính năng tin → F115–F118; xử lý báo cáo/case → F121.
- Tab channel bền vững (Khu vực, Lớp, Bạn bè, DM), tìm kiếm, cài đặt thông báo channel → F120 (+ UI phần đó khi F120 Done).
- Hộp thông báo trong game → F123; UI party lobby/arena (khung chứa panel) → F102, F109; `RealtimeClient` → F092.

## Assumptions

- `RealtimeClient` (F092) cung cấp gửi lệnh có `id`, nhận `system.command.failed`, resume với `last_seq`; F119 chỉ dùng API của nó.
- Trước khi F121 Done, `createReport` dùng Prism mock gắn nhãn MOCK trong build debug; build phát hành không có mock (R17).
- Thời điểm thu nhỏ panel lấy từ sự kiện của màn câu hỏi (F063/F109) qua `EventBus`, không từ logic chat [Confirmed].
