# Feature Specification: F123 — Thông báo trong game: realtime, lưu bền vững, không push

**Feature Branch**: `123-in-game-notifications` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `chat` (module notification) + `src/game/` · **Mốc**: M5 · **Phụ thuộc**: F090, F115 (nguồn sự kiện: F095, F100, F041, F074)
**Input**: Học sinh nhận thông báo ngay trong game khi có lời mời kết bạn/party, bài nhà trường giao, kết quả được công bố, phần thưởng; thông báo được lưu để xem lại trong hộp thông báo, có badge chưa đọc; không gửi push ngoài game tới học sinh. FR nguồn: FR-NTF-001, FR-NTF-002, FR-NTF-003.

## Clarifications

### Session 2026-11-16

- Q: "Thông báo trường" gồm gì khi chưa có contract soạn thông báo? → A: Trong Release 1.0 là thông báo phát sinh từ hoạt động của nhà trường: bài được giao (`assignment.published`) và kết quả được công bố (`assignment.results_released`). Soạn thông báo tự do của nhà trường chưa có FR/contract — ngoài phạm vi.
- Q: Danh sách học sinh nhận thông báo bài giao lấy ở đâu khi payload `assignment.published` không chứa danh sách học sinh? → A: Facade `learning` `listAssignmentRecipients` (operation mới) gọi từ profile `worker-chat` — learning là nguồn duy nhất về đối tượng giao bài (lớp, nhóm, học sinh, miễn làm); không dựng lại từ projection (AGENTS R29, chỉ đọc).
- Q: Enum loại thông báo khác nhau: REST `Notification.kind` vs WS `notification.item.created.category`? → A: Thay bởi Session 2026-11-17 (một enum `kind`).
- Q: "Kết quả" là sự kiện nào? → A: `assignment.results_released` (kết quả bài giao được công bố theo `ResultRelease`); kết quả trận/giải → F106/F107 bổ sung nguồn sau. Phần thưởng: `level.reached`, `achievement.unlocked` (consumer NTF trong EVENT_DESIGN §4.6).
- Q: Thông báo kết quả xử lý báo cáo (SOCIAL_SYSTEM §8)? → A: F121 phát event khi có; F123 cung cấp cơ chế (giá trị `kind` tương ứng thêm khi có event), không tự thêm nguồn.

### Session 2026-11-17

- Q: Enum loại thông báo sau đối soát (RB-513)? → A: **Một enum `kind`** dùng chung REST (`Notification.kind`) và WS (`notification.item.created`) theo [API_DESIGN §9.11](../../docs/07-backend/API_DESIGN.md); bỏ `category`. Delta contract: gỡ `category` khỏi AsyncAPI, thêm `kind` (contracts §6).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Nhận thông báo realtime khi có sự kiện (Priority: P1)

Bạn gửi lời mời kết bạn, trưởng nhóm mời vào party, nhà trường giao bài mới: học sinh đang chơi thấy thông báo ngay; nếu offline thì thấy khi vào game.

**Why this priority**: FR-NTF-001/002; lời mời party và bài giao mất giá trị nếu học sinh không biết.

**Independent Test**: Phát `friend_request.sent` (A → B) → B (đang kết nối) nhận `notification.item.created` một lần; phát lại cùng event → không có thông báo thứ hai; B offline → thông báo có trong `listNotifications?unread=true`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F123-AC-01 | KHI nhận `friend_request.sent`, `party.invitation_sent`, `level.reached` hoặc `achievement.unlocked`, module chat PHẢI tạo đúng một thông báo cho học sinh nhận (`kind`, `message_code`, `params`, `ref_type`, `ref_id`, `action`) và ghi bền vững trước khi đẩy realtime | FR-NTF-001 · FR-NTF-002 · [SOCIAL_SYSTEM §8](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | — |
| F123-AC-02 | KHI nhận `assignment.published` (trạng thái `open`) hoặc `assignment.results_released`, module chat PHẢI lấy danh sách học sinh nhận từ module learning và tạo một thông báo `kind` = `assignment_new`/`assignment_results_released` cho mỗi học sinh, bỏ qua học sinh được miễn | FR-NTF-001 · FR-LMS-006 · [EVENT_DESIGN §4.3](../../docs/07-backend/EVENT_DESIGN.md) | — |
| F123-AC-03 | NẾU cùng một event được giao lại, THÌ module chat PHẢI không tạo thông báo thứ hai (khóa người nhận + `event_id` + `kind`) | FR-NTF-002 · NFR-SEC-004 | TC-SVC-003 |
| F123-AC-04 | KHI thông báo được ghi, module chat PHẢI đẩy `notification.item.created` tới học sinh qua kênh realtime của người đó (không droppable, được giữ trong buffer resume) | FR-NTF-002 · [REALTIME_ARCHITECTURE §1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F123-AC-05 | `params` và nội dung thông báo PHẢI chỉ chứa nickname, tên nội dung và mã định danh cần cho deep link; PHẢI không chứa tên thật, trường, lớp của học sinh khác, nội dung chat, điểm của người khác hay chi tiết hình phạt; mọi chuỗi PHẢI trong `maxLength` của contract | AGENTS R14, R16 · ADR-0041 | — |
| F123-AC-06 | NẾU người nhận không còn là học sinh hoạt động trong tenant của event, hoặc event thuộc tenant khác tenant của người nhận, THÌ module chat PHẢI không tạo thông báo | NFR-SEC-006 · AGENTS R8 | TC-TEN-001 |
| F123-AC-07 | NẾU module learning không phản hồi khi lấy người nhận, THÌ consumer PHẢI thử lại theo chính sách consumer (không ack) và chuyển DLQ sau số lần quy định, không tạo thông báo thiếu người | FR-NTF-002 · NFR-AVAIL-005 | TC-SVC-004 |

---

### User Story 2 — Hộp thông báo lưu bền vững và đánh dấu đã đọc (Priority: P1)

Học sinh mở hộp thông báo xem lại các thông báo cũ, thấy số chưa đọc; đọc trên một thiết bị thì thiết bị sau cũng thấy đã đọc.

**Why this priority**: FR-NTF-002 (`/api/v1/notifications`); học sinh không online lúc sự kiện vẫn phải biết.

**Independent Test**: `listNotifications?unread=true` trả đúng thông báo của mình kèm `unread_count`; `markNotificationRead` → `unread_count` giảm; học sinh khác gọi với `notification_id` này → `404`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F123-AC-08 | KHI người dùng gọi `listNotifications` (lọc `unread`, cursor), module chat PHẢI trả chỉ thông báo của chính người gọi trong tenant phiên, mới nhất trước, kèm `unread_count` | FR-NTF-002 | TC-TEN-001 |
| F123-AC-09 | KHI người nhận gọi `markNotificationRead` hoặc gửi WS `notification.item.mark_read` (≤ 100 id), module chat PHẢI đặt thời điểm đọc một lần (gọi lại không đổi) và trả trạng thái mới; id không thuộc người gọi PHẢI bị bỏ qua (WS) hoặc trả `404` (REST) | FR-NTF-002 · NFR-SEC-004 | TC-TEN-001 |
| F123-AC-10 | KHI người nhận gọi `markAllNotificationsRead`, module chat PHẢI đánh dấu mọi thông báo chưa đọc của chính người đó tạo trước thời điểm gọi và trả `unread_count = 0` | FR-NTF-002 | — |
| F123-AC-11 | NẾU thiếu `X-Tenant-Id`/header khác tenant phiên, THÌ module chat PHẢI trả `400 TENANT_HEADER_REQUIRED`/`403 TENANT_MISMATCH`; không có token hợp lệ PHẢI trả `401` | FR-AUTH-019 · ADR-0065 | TC-TEN-011 |
| F123-AC-12 | HỆ THỐNG PHẢI không xóa vật lý thông báo; trạng thái đã đọc lưu ở PostgreSQL (Valkey chỉ cache số chưa đọc) | AGENTS R9, R21 · ADR-0070 | — |

---

### User Story 3 — Không push ngoài game tới học sinh (Priority: P1)

Thông báo chỉ hiện khi học sinh đang trong game; không có thông báo đẩy kéo trẻ quay lại game.

**Why this priority**: FR-NTF-003, an toàn trẻ em (CHILD_SAFETY, chống áp lực gây nghiện R19).

**Independent Test**: Quét OpenAPI/AsyncAPI và phụ thuộc build: không có endpoint đăng ký token thiết bị, không có thư viện FCM/APNs/Web Push trong backend và Godot.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F123-AC-13 | HỆ THỐNG PHẢI không có endpoint đăng ký token push và không gửi thông báo tới học sinh qua push di động, Web Push, email hay SMS; thông báo chỉ giao qua kết nối realtime trong game và API hộp thông báo | FR-NTF-003 · AGENTS R19 | — |
| F123-AC-14 | CI PHẢI thất bại nếu module chat hoặc bản build Godot thêm thư viện/SDK push (FCM, APNs, OneSignal, Web Push) | FR-NTF-003 | — |

---

### User Story 4 — Hiển thị thông báo trong game (Priority: P2)

Học sinh thấy badge số chưa đọc trên HUD, popup ngắn (tắt được) khi có lời mời, mở hộp thông báo và bấm để đi tới nơi liên quan (bài giao, lời mời).

**Why this priority**: Hoàn tất trải nghiệm FR-NTF-001 trong game.

**Independent Test**: GdUnit4: nhận `notification.item.created` → badge +1, toast hiện (trừ khi tắt); mở inbox → gửi `notification.item.mark_read`; bấm thông báo bài giao → `EventBus.ui_requested("quest_journal", {tab: "assigned"})`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F123-AC-15 | KHI vào game hoặc resume thất bại, game client PHẢI tải `listNotifications?unread=true` và hiển thị badge số chưa đọc trên HUD; KHI nhận `notification.item.created`, PHẢI cập nhật badge ngay | FR-NTF-001 · FR-NTF-002 | — |
| F123-AC-16 | KHI nhận thông báo `kind` = `friend_request`/`party_invitation`/`assignment_new`/`assignment_results_released`, game client PHẢI hiện toast ngắn không chặn thao tác; TRONG KHI học sinh tắt popup trong Cài đặt hoặc đang trong trận/bài kiểm tra, PHẢI chỉ cập nhật badge; âm thanh thông báo PHẢI tắt mặc định | FR-NTF-001 · [SOCIAL_SYSTEM §8](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | — |
| F123-AC-17 | game client PHẢI hiển thị nội dung từ `message_code` (`ui.notification.<code>`) + `params` theo ngôn ngữ đang chọn, dùng `title`/`body` khi thiếu khóa dịch | NFR-I18N-001 · ADR-0056 | TC-I18N-005 |
| F123-AC-18 | KHI học sinh mở hộp thông báo hoặc bấm một thông báo, game client PHẢI gửi `notification.item.mark_read` cho các mục đã xem và mở màn theo `action` qua `EventBus.ui_requested`, không gọi API nghiệp vụ trực tiếp từ scene | FR-NTF-002 · AGENTS R1 | — |
| F123-AC-19 | Mọi thành phần UI thông báo PHẢI dùng Theme sinh từ token theo grade band và chuỗi qua file dịch | AGENTS R32 · NFR-A11Y-001 | TC-UI-003 |

### Edge Cases

- NẾU học sinh offline lâu, THÌ khi vào game chỉ tải trang đầu (cursor) và badge theo `unread_count`; không phát toast hàng loạt cho thông báo cũ.
- NẾU lời mời party đã hết hạn khi học sinh bấm thông báo, THÌ màn đích hiển thị trạng thái hết hạn từ API của F100 (thông báo không tự xóa).
- NẾU Valkey mất, THÌ thông báo vẫn ghi PostgreSQL; client lấy qua REST khi nhận `system.service.degraded` hoặc khi resync.
- NẾU một bài giao nhắm 1 lớp 40 học sinh, THÌ tạo 40 thông báo trong một lô; lặp event không nhân đôi (F123-AC-03).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-NTF-001 | Tạo thông báo từ lời mời kết bạn/party, bài giao, kết quả công bố, phần thưởng; hiển thị trong game | [SOCIAL_SYSTEM §8](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |
| FR-NTF-002 | Đẩy realtime qua WebSocket; lưu bền vững; hộp thông báo, đã đọc | [CHAT_SYSTEM §10](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · [API_DESIGN §9.11](../../docs/07-backend/API_DESIGN.md) |
| FR-NTF-003 | Không push ngoài game tới học sinh | [SOCIAL_SYSTEM §8](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |

### Key Entities

- **Notification**: thông báo gửi một người nhận: loại (`kind`, enum chung REST và WS), mã thông điệp + tham số, tham chiếu tài nguyên, hành động mở màn, thời điểm tạo, thời điểm đọc, event nguồn.
- **NotificationSource**: event domain sinh thông báo và quy tắc chọn người nhận.
- **UnreadCounter**: số chưa đọc của một người (cache, tính lại được).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `api_rate_default` | Rate limit `listNotifications`, đánh dấu đã đọc |
| `resume_window` | Thông báo realtime giữ trong buffer resume |
| `idempotency_record_ttl` | Kết quả request đánh dấu đã đọc có `Idempotency-Key` (tùy chọn) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Ghi PostgreSQL trước, đẩy Valkey sau commit; consumer idempotent | AGENTS R9 · ADR-0004, ADR-0042 |
| Lấy người nhận bài giao qua facade `learning`, không đọc DB learning | AGENTS R13, R29 · ADR-0074 |
| Bảng mới `tenant_id` + RLS; không xóa vật lý | AGENTS R8, R21 · ADR-0028, ADR-0070 |
| Không push ngoài game | FR-NTF-003 · AGENTS R19 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh đang online thấy thông báo lời mời trong ≤ 2 giây sau khi lời mời được tạo (p95).
- **SC-002**: 0 thông báo trùng khi event được giao lại 3 lần; 0 thông báo gửi sai tenant trong test cô lập.
- **SC-003**: 100% thông báo tạo khi học sinh offline xuất hiện trong hộp thông báo khi vào game.
- **SC-004**: 0 kênh push ngoài game (kiểm bằng quét contract và phụ thuộc build).

## Ngoài phạm vi

- Nguồn sự kiện: lời mời kết bạn (F095), lời mời party (F100), bài giao/kết quả (F041), level/achievement (F074, F075); toast lời mời party trong lobby → F102; thông báo bài thi mới ở Exam Zone → F064.
- Thông báo vụ việc an toàn cho School Admin trong portal → F121/F122; thông báo kết quả xử lý báo cáo (nguồn) → F121; badge tin chat/mention → F118/F119.
- Soạn thông báo tự do của nhà trường/bảng tin lớp; trung tâm thông báo cho nhân sự trong Admin Portal; push ngoài game (cần ADR mới, OQ-17).

## Assumptions

- `listNotifications` "Mọi role" (API_DESIGN §9.11): nhân sự gọi được nhưng F123 chỉ tạo thông báo cho học sinh.
- Popup bật/tắt là cài đặt cục bộ của thiết bị (`Settings`), không lưu server [Confirmed].
- F115 đã dựng consumer/worker `chat-notifier` của module chat; F041 phát `assignment.published`/`assignment.results_released`.
