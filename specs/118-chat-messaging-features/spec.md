# Feature Specification: F118 — Bộ tính năng nhắn tin đầy đủ (sửa/thu hồi, reply/forward, mention, receipts) và sticker/GIF

**Feature Branch**: `118-chat-messaging-features` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `chat` · **Mốc**: M4b · **Phụ thuộc**: F117
**Input**: Bộ tính năng tin nhắn ADR-0034 trên lõi F115–F117: sửa tin (chạy lại kiểm duyệt, lưu revision), thu hồi (placeholder, giữ bằng chứng), reply/quote (từ bản đã che), forward (cùng tenant, chạy lại kiểm duyệt channel đích), emoji, sticker (pack platform, mở khóa bằng phần thưởng), GIF (thư viện nội bộ), reaction, mention `@nickname`, pin, typing, Sent/Delivered/Read theo con trỏ, unread + mention count. Kèm backend soạn sticker pack/GIF cho Game Studio. FR nguồn: FR-CHAT-012…016.

## Clarifications

### Session 2026-11-16

- Q: Mention người không phải thành viên/đã chặn người gửi: từ chối cả tin (`CHAT_MENTION_LIMIT`, API_DESIGN §5) hay loại khỏi `mentions` (CHAT_SYSTEM §7.7, TC-CHAT-035)? → A: Loại khỏi `mentions` và vẫn gửi tin; chỉ `> chat_mentions_max` mới `CHAT_MENTION_LIMIT` (theo tài liệu sở hữu CHAT_SYSTEM).
- Q: Tìm kiếm full-text (FR-CHAT-016)? → A: P5, làm ở F120; F118 làm Sent/Delivered/Read, unread + mention count, lịch sử phân trang dùng lại F115.
- Q: Quyền pin của leader party lấy từ đâu? → A: `channel_members.role = leader` cập nhật từ `party.created.leader_id` và `party.leader_transferred` (module chat thêm consumer — EVENT_DESIGN §4.7 cần bổ sung chat làm consumer).
- Q: Học sinh "đã sở hữu" pack sticker khóa xác định thế nào? → A: Qua projection từ event mở khóa của module game (F075); chưa có event thì pack khóa không chọn được (fail-closed), pack mặc định vẫn dùng được.
- Q: Copy tin? → A: Tính năng client (F119), không có API.

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F118-AC-17: "(gateway nội bộ, MFA)" đọc là gateway nội bộ + `game.chat_assets.write`, không MFA; plan/contracts nhắc MFA là lịch sử. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Receipts và ghim còn nhánh cho channel `class`/`study_group`? → A: Không — hai loại channel này bị bỏ ([ADR-0095 §5](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md), FR-CHAT-001). `getMessageReceipts` trả danh sách từng người với channel ≤ 8 thành viên (`party`, `match`, `team` theo trận, `direct`) và chỉ số đếm với channel nhiều thành viên còn lại là club (`team` cố định, tới `club_max_members`) [Proposed — WP-S: thay nhánh `class`/`study_group` cũ bằng club; mô tả operation trong `chat.v1` do WP-C1 sửa]. Ghim ở `world`/`area` vẫn `403 CHAT_FEATURE_DISABLED`. F118-AC-20, F118-AC-23 viết lại.

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: F118-AC-17. Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Sửa và thu hồi tin mà không xóa được bằng chứng (Priority: P1)

Học sinh gõ sai thì sửa trong thời hạn ngắn; gửi nhầm thì thu hồi. Mọi người thấy nhãn "đã chỉnh sửa" hoặc "Tin nhắn đã được thu hồi"; Safety Team vẫn xem được bản gốc khi có báo cáo.

**Why this priority**: Rủi ro bắt nạt rồi xóa dấu vết (CHILD_SAFETY §3.3b); sửa là nội dung mới nên phải qua kiểm duyệt (R25).

**Independent Test**: An gửi tin, sửa sau 1 phút (hợp lệ) → Bình nhận `chat.message.edited`, `revision = 2`, bản cũ trong revision; sửa sang nội dung vi phạm → `CHAT_CONTENT_BLOCKED`, bản cũ giữ; sửa sau quá hạn → `CHAT_EDIT_WINDOW_EXPIRED`; thu hồi → placeholder, nội dung gốc còn trong DB.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F118-AC-01 | KHI người gửi sửa tin `text` của mình trong `chat_edit_window` với `revision_expected` đúng, module chat PHẢI chạy lại toàn bộ pipeline, lưu bản cũ vào revision, tăng `revision`/`edit_count`, đặt `edited_at` và phát `chat.message.edited` (nội dung đã che) chỉ sau khi stream bền vững ack | FR-CHAT-012 · AGENTS R25 · ADR-0034 | TC-CHAT-029 |
| F118-AC-02 | NẾU quá `chat_edit_window`, người gọi không phải người gửi, hoặc tin không phải `text`, THÌ module chat PHẢI trả `409 CHAT_EDIT_WINDOW_EXPIRED`; NẾU `revision_expected`/`If-Match` lệch, THÌ PHẢI trả `412 PRECONDITION_FAILED` | FR-CHAT-012 · AC-CHAT-012 | TC-CHAT-029 |
| F118-AC-03 | NẾU bản sửa bị `block`, THÌ module chat PHẢI giữ nguyên bản cũ và trả `CHAT_CONTENT_BLOCKED`; NẾU bản sửa bị `hold_for_review`, THÌ người khác PHẢI vẫn thấy bản cũ đến khi được duyệt | FR-CHAT-012 · [MODERATION §2.1](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-029 |
| F118-AC-04 | KHI người gửi thu hồi tin trong `chat_recall_window`, module chat PHẢI đặt `recalled_at`/`recalled_by`, giữ nguyên nội dung gốc và revision làm bằng chứng, tự bỏ ghim, phát `chat.message.recalled` (không nội dung); NẾU quá hạn, THÌ PHẢI trả `409 CHAT_RECALL_WINDOW_EXPIRED` | FR-CHAT-012 · AC-CHAT-012 · ADR-0028 | TC-CHAT-030 |
| F118-AC-05 | HỆ THỐNG PHẢI không trả nội dung tin đã thu hồi hoặc đã gỡ qua lịch sử, quote, reply, forward, mention preview hay receipts; các nơi đó PHẢI hiển thị placeholder ("Tin nhắn gốc không còn") | FR-CHAT-012 · AGENTS R25 | TC-CHAT-031 |
| F118-AC-06 | KHI tin đã thu hồi hoặc đã sửa vẫn trong thời hạn lưu của channel, HỆ THỐNG PHẢI còn đủ nội dung gốc và mọi revision để module moderation dùng khi có báo cáo | FR-CHAT-012 · AC-CHAT-012 | TC-CHAT-030 |

---

### User Story 2 — Trả lời, trích dẫn, chuyển tiếp an toàn (Priority: P1)

Học sinh trả lời đúng tin cần trả lời, trích một đoạn, hoặc chuyển tiếp lời mời làm quest sang nhóm khác; không thể dùng chuyển tiếp để phát tán tin riêng ra ngoài trường.

**Why this priority**: Forward là nội dung mới ở channel đích; phải chặn phát tán liên tenant (CHILD_SAFETY §3.3b).

**Independent Test**: Quote một tin đã bị che → excerpt chỉ có `***`; forward sang channel tenant khác hoặc channel không có quyền → `CHAT_FORWARD_NOT_ALLOWED`; forward hợp lệ tới 2 channel → mỗi tin mới qua pipeline của channel đích.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F118-AC-07 | KHI gửi tin có `reply_to_message_id`, module chat PHẢI kiểm tin gốc cùng channel và người gửi được xem, và trả kèm trích ngắn (≤ 80 ký tự) lấy từ nội dung đã che | FR-CHAT-013 · [CHAT_SYSTEM §7.4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-031 |
| F118-AC-08 | KHI gửi tin có `quote` (`message_id`, `excerpt_start`, `excerpt_end`), module chat PHẢI tự cắt excerpt (≤ 200 ký tự) từ nội dung đã che của tin gốc, không nhận nội dung trích từ client, và đưa excerpt qua pipeline | FR-CHAT-013 · AC-CHAT-013 | TC-CHAT-031 |
| F118-AC-09 | KHI forward tới tối đa `chat_forward_targets_max` channel, module chat PHẢI tạo tin mới ở mỗi channel đích với `forwarded_from` (`message_id`, `channel_type`, `sender_nickname`), mỗi tin chạy lại pipeline và policy của channel/tenant đích | FR-CHAT-013 · AGENTS R25 | TC-CHAT-032 |
| F118-AC-10 | NẾU channel đích khác tenant, người gửi không có quyền gửi ở đích, nguồn là `world`/`area`, hoặc tin nguồn đã thu hồi/bị che/bị gỡ/đang held, THÌ module chat PHẢI từ chối `CHAT_FORWARD_NOT_ALLOWED`; vượt số đích PHẢI trả `VALIDATION_FAILED` | FR-CHAT-013 · AC-CHAT-013 | TC-CHAT-032 |
| F118-AC-11 | KHI gọi `forwardMessage` lặp lại cùng `Idempotency-Key`, module chat PHẢI trả cùng danh sách tin, không tạo tin trùng; thiếu khóa PHẢI trả `428 IDEMPOTENCY_KEY_REQUIRED` | FR-CHAT-013 · AGENTS R10 | TC-CHAT-032 |

---

### User Story 3 — Emoji, sticker, GIF và reaction từ thư viện đã duyệt (Priority: P2)

Học sinh lớp nhỏ biểu đạt bằng sticker, GIF chủ đề học tập, thả cảm xúc; mọi hình ảnh đến từ thư viện EduWorld đã duyệt, không có ảnh do học sinh tải lên hay dịch vụ GIF Internet.

**Why this priority**: Kênh giao tiếp không cần gõ chữ cho trẻ nhỏ; hình ảnh không qua kiểm tra văn bản nên phải đóng nguồn.

**Independent Test**: Gửi `sticker_id` của pack chưa gán tenant/chưa mở khóa, `gif_id` lạ → `CHAT_ASSET_NOT_AVAILABLE`; thả 4 reaction trên một tin → lần 4 bị từ chối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F118-AC-12 | KHI gửi `kind = sticker`, module chat PHẢI kiểm sticker thuộc pack `published`, pack được gán cho tenant của người gửi, và người gửi sở hữu pack khi pack có `unlock_rule`; ngược lại PHẢI trả `CHAT_ASSET_NOT_AVAILABLE` | FR-CHAT-014 · AC-CHAT-013 · ADR-0029 | TC-CHAT-033 |
| F118-AC-13 | KHI gửi `kind = gif`, module chat PHẢI chỉ chấp nhận `gif_id` `published` trong thư viện nội bộ; `listGifs` PHẢI chỉ trả GIF đã publish theo tag, không có URL hay endpoint dịch vụ GIF bên thứ ba | FR-CHAT-014 · AC-CHAT-013 | TC-CHAT-033 |
| F118-AC-14 | KHI học sinh gọi `listStickerPacks`, module chat PHẢI trả pack được gán cho tenant kèm trạng thái mở khóa của chính học sinh, nhãn song ngữ | FR-CHAT-014 | TC-CHAT-033 |
| F118-AC-15 | HỆ THỐNG PHẢI giới hạn sticker/GIF chung theo rate của channel và trần sticker/GIF mỗi phút; emoji chỉ từ bộ cho phép cấu hình platform | FR-CHAT-014 · [CHAT_SYSTEM §7.5](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-033 |
| F118-AC-16 | KHI thành viên `chat.reaction.toggle` (hoặc REST) một reaction trong bộ cho phép, module chat PHẢI bật/tắt reaction, giới hạn `chat_reactions_per_user` mỗi người mỗi tin, từ chối trên tin đã thu hồi, và phát `chat.reaction.updated` với số đếm đồng bộ mọi client | FR-CHAT-014 · FR-CHAT-003 | TC-CHAT-034 |
| F118-AC-17 | KHI người có `game.chat_assets.write` (gateway nội bộ, role platform) tạo/sửa/duyệt/publish sticker pack hoặc GIF, module chat PHẢI kiểm tệp (sticker webp/png ≤ 512 KB; GIF ≤ 2 MB, ≤ 480 px, có preview tĩnh), bắt buộc người duyệt khác người upload với GIF, version hóa, ghi audit; role tenant PHẢI nhận `403 PERMISSION_DENIED` | FR-CHAT-014 · AGENTS R20 · [ADMIN_PORTAL §3.3](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CHAT-033 |

---

### User Story 4 — Mention, ghim, đang soạn tin (Priority: P2)

Học sinh gọi tên bạn bằng `@nickname`, ghim tin quan trọng của nhóm, thấy bạn đang soạn tin.

**Why this priority**: Phối hợp nhóm trong trận/party; phải chống spam mention và không lộ username đăng nhập.

**Independent Test**: Tin có 6 mention → `CHAT_MENTION_LIMIT`; mention hợp lệ → người được mention nhận `chat.mention.created`, `unread_mentions` tăng; ghim tin thứ 4 → `CHAT_PIN_LIMIT_REACHED`; typing không ghi DB, tự hết sau 5 giây.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F118-AC-18 | NẾU tin có nhiều hơn `chat_mentions_max` mention, THÌ module chat PHẢI trả `422 CHAT_MENTION_LIMIT`; mention tới người không phải thành viên hoặc có quan hệ chặn với người gửi PHẢI bị loại khỏi tin | FR-CHAT-015 · [CHAT_SYSTEM §7.7](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-035 |
| F118-AC-19 | KHI tin có mention hợp lệ được ack, module chat PHẢI tăng `unread_mentions` của người được mention, gửi `chat.mention.created` (nickname từ hồ sơ game, preview đã che ≤ 80 ký tự) và payload PHẢI không chứa username đăng nhập | FR-CHAT-015 · AGENTS R16 | TC-CHAT-035 |
| F118-AC-20 | KHI ghim/bỏ ghim (`chat.pin.add/remove`, `pinMessage`/`unpinMessage`), module chat PHẢI kiểm quyền theo loại channel (leader ở `party`; mọi người chơi ở `match`/`team`; cả hai ở `direct`), tối đa `chat_pins_max` (vượt → `409 CHAT_PIN_LIMIT_REACHED`), ghi audit, phát `chat.pin.updated`; ở `world`/`area` PHẢI trả `403 CHAT_FEATURE_DISABLED` | FR-CHAT-015 · ADR-0072 · ADR-0095 | TC-CHAT-036 |
| F118-AC-21 | KHI nhận `chat.typing.start/stop`, module chat PHẢI throttle 1 lần/3 giây, tự hết sau 5 giây, phát `chat.typing.updated` (tối đa 3 nickname hiển thị) chỉ qua pub/sub tạm thời, không ghi DB, không gửi tới người có quan hệ chặn, không hỗ trợ ở `world`/`area` | FR-CHAT-015 · [CHAT_SYSTEM §7.10](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-037 |

---

### User Story 5 — Đã gửi / đã nhận / đã xem và bộ đếm chưa đọc (Priority: P2)

Trong party 4 người, người gửi thấy ai đã nhận, ai đã xem; trong club đông người chỉ thấy "N bạn đã xem"; số chưa đọc và số mention đúng trên mọi thiết bị.

**Why this priority**: FR-CHAT-016 P4; giữ đúng ranh giới riêng tư (không lộ trạng thái giữa người chặn nhau).

**Independent Test**: Party 4 người: Bình nhận (delivered), Chi đọc (read) → An gọi `getMessageReceipts` thấy đúng từng người; channel club (> 8 thành viên) chỉ trả số đếm; Dũng chặn An → An không thấy trạng thái của Dũng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F118-AC-22 | KHI thiết bị gửi `chat.receipt.delivered`/`chat.receipt.read`, module chat PHẢI gộp con trỏ (≤ 1 lần/2 giây/thành viên/channel), lưu `last_delivered_seq`/`last_read_seq` không lùi, không lưu dòng theo từng tin × người, và phát `chat.receipt.updated` cho thành viên channel có receipts | FR-CHAT-016 · [CHAT_SYSTEM §8.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-038 |
| F118-AC-23 | KHI gọi `getMessageReceipts`, module chat PHẢI trả danh sách từng người với channel ≤ 8 thành viên (`party`, `match`, `team` theo trận, `direct`), chỉ số đếm với channel club (`team` cố định) có > 8 thành viên, và `403 CHAT_FEATURE_DISABLED` với `world`/`area` | FR-CHAT-016 · AC-CHAT-014 · ADR-0095 | TC-CHAT-038 |
| F118-AC-24 | HỆ THỐNG PHẢI không cho hai người có quan hệ chặn thấy trạng thái nhận/xem hay typing của nhau | FR-CHAT-016 · AGENTS R16 | TC-CHAT-038 |
| F118-AC-25 | KHI con trỏ đọc thay đổi trên một thiết bị, module chat PHẢI cập nhật `unread_count` và `unread_mentions` (reset mention khi đọc qua tin chứa mention) và phát `chat.unread.updated` tới mọi thiết bị của người đó | FR-CHAT-016 · AC-CHAT-014 | TC-CHAT-039 |

---

### Phạm vi chung (áp cho mọi story)

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F118-AC-26 | HỆ THỐNG PHẢI cho mọi endpoint/lệnh mới của feature: học sinh không phải thành viên hoặc khác tenant nhận `404` (không lộ tồn tại), token nhân sự nhận `403 GAME_ACCESS_STUDENT_ONLY`, `X-Tenant-Id` lệch nhận `403 TENANT_MISMATCH` | AC-CHAT-007 · AGENTS R8 · AGENTS R23 | TC-TEN-001 |
| F118-AC-27 | NẾU tính năng bị tắt trong `features` của effective policy cho loại channel, THÌ module chat PHẢI trả `403 CHAT_FEATURE_DISABLED`; với `match`/`team`/`party` các tính năng "luôn" của CHAT_SYSTEM §7.1 PHẢI luôn bật | FR-CHAT-012 · FR-CHAT-015 · ADR-0020 | TC-CHAT-036 |

### Edge Cases

- NẾU sửa tin đang được trích dẫn/trả lời, THÌ trích dẫn giữ excerpt đã cắt lúc gửi (bất biến), không cập nhật theo bản sửa.
- NẾU sự kiện sửa/thu hồi đến persister muộn hơn bản mới, THÌ bỏ qua theo `revision` (EVENT_DESIGN §4.10).
- NẾU tin bị thu hồi đang được ghim, THÌ tự bỏ ghim và phát `chat.pin.updated` (F118-AC-04).
- NẾU pack sticker bị gỡ gán khỏi tenant, THÌ tin cũ vẫn hiển thị sticker (asset bất biến), không gửi mới được.
- NẾU channel gameplay đã đóng, THÌ sửa/thu hồi/reaction/pin trả `CHAT_CHANNEL_CLOSED`; receipts đọc vẫn được ghi.
- NẾU người được mention offline, THÌ chỉ tăng bộ đếm; thông báo trong game do F123.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHAT-012 | Sửa (chạy lại kiểm duyệt, revision), thu hồi (placeholder, giữ bằng chứng) | [CHAT_SYSTEM §7.2–§7.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · ADR-0034 |
| FR-CHAT-013 | Reply, Quote (từ bản đã che), Forward (cùng tenant, ≤ `chat_forward_targets_max`, chạy lại kiểm duyệt) | [CHAT_SYSTEM §7.4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-014 | Emoji, sticker (pack platform, mở khóa), GIF nội bộ, reaction; backend soạn sticker/GIF | [CHAT_SYSTEM §7.5–§7.6](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-015 | Mention, pin, typing (copy ở client F119) | [CHAT_SYSTEM §7.7–§7.10](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-016 | Sent/Delivered/Read theo con trỏ, unread + mention count (tìm kiếm → F120) | [CHAT_SYSTEM §8.1–§8.2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |

### Key Entities

- **MessageRevision**: bản cũ của tin khi sửa, kèm quyết định kiểm duyệt (bằng chứng).
- **MessageReaction**: reaction của một người trên một tin.
- **PinnedMessage**: tin được ghim trong channel (≤ `chat_pins_max`).
- **StickerPack / Sticker**: thư viện sticker platform, gán tenant, có thể khóa bằng phần thưởng.
- **GifAsset / GifTag**: thư viện GIF nội bộ đã duyệt, tag song ngữ.
- **StickerUnlock**: quyền dùng pack khóa của một học sinh (projection từ module game).
- **MemberCursor**: con trỏ đã nhận/đã xem và bộ đếm mention của thành viên.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `chat_edit_window` / `chat_recall_window` | Thời hạn sửa / thu hồi |
| `chat_pins_max` / `chat_mentions_max` / `chat_reactions_per_user` | Giới hạn ghim, mention, reaction |
| `chat_forward_targets_max` | Số channel đích tối đa khi forward |
| `chat_max_length` | Độ dài bản sửa |
| `idempotency_record_ttl` | Giữ kết quả `forwardMessage` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Sửa/forward/quote là nội dung mới → qua pipeline; thu hồi giữ bằng chứng; không trả nội dung gốc đã che | AGENTS R25 · ADR-0034 |
| Mọi thay đổi tin đi qua stream bền vững (`evt`) trước khi phát; typing/receipt broadcast chỉ qua pub/sub tạm thời | ADR-0035 |
| Không upload ảnh/file của học sinh; không dịch vụ GIF bên thứ ba | ADR-0034 · CHILD_SAFETY §3.3b |
| Endpoint soạn sticker/GIF chỉ gateway nội bộ, role platform | AGENTS R20 · ADR-0027 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% bản sửa/forward/quote trong bộ test đi qua kiểm duyệt; 0 nội dung đã thu hồi xuất hiện trong mọi API người dùng.
- **SC-002**: Trong party 4 người, trạng thái nhận/xem hiển thị đúng cho người gửi trong ≤ 2 giây ở mạng bình thường.
- **SC-003**: 0 sticker/GIF ngoài thư viện đã duyệt được gửi thành công trong bộ test bảo mật.
- **SC-004**: Unread + mention count khớp nguồn sự thật sau khi đọc trên thiết bị khác ở 100% lần kiểm.

## Ngoài phạm vi

- Tìm kiếm full-text, channel bền vững (team cố định của club, DM) → F120; `show_read_receipts` cho DM [Confirmed] → F120.
- Báo cáo tin (kể cả tin đã thu hồi), moderator xem nội dung gốc/revision, gỡ tin → F121; thông báo mention khi offline → F123.
- UI Godot (menu thao tác, picker, copy, receipts) → F119; UI Game Studio cho sticker/GIF trong Portal workspace A → chưa có feature trong danh mục (đề xuất bổ sung vào F079).
- Phần thưởng mở khóa sticker (reward rule, inventory) → F075.

## Assumptions

- Gán sticker pack/GIF cho tenant dùng `infra.tenant_resource_assignments` của `eduworld_chat`, ghi qua saga gán của System Admin (F045, `executeTenantCloneStep` hoặc bước gán tương đương) [Confirmed].
- module game (F075) phát event mở khóa vật phẩm mang `item_id`/`reward_rule_id`; chưa có thì pack khóa không dùng được (fail-closed).
- Trần sticker/GIF 4/phút/channel, throttle typing 3 s/hết hạn 5 s, gộp receipt 2 s, giới hạn tệp sticker/GIF chưa có tên trong CONFIG_DEFAULTS → task Polish đề xuất đăng ký [Confirmed].
- Thời hạn sửa/thu hồi và các giới hạn số là [Confirmed] theo PRD FR-CHAT-012…015.
