# Feature Specification: F102 — Giao diện party trong Godot: panel, lời mời, lobby, ready

**Feature Branch**: `102-godot-party-ui` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `godot/` · **Mốc**: M4a · **Phụ thuộc**: F092, F101 (và F100; danh sách bạn bè từ F095)
**Input**: Học sinh tạo party, mời bạn bằng chạm avatar, nhận lời mời dạng popup, vào Party Lobby (thành viên, online, Ready, chọn hoạt động, Bắt đầu), thấy trạng thái leader/thành viên mất kết nối, chuyển vào phiên chơi chung và quay lại lobby. FR nguồn: FR-PTY-001, FR-PTY-006, FR-PTY-008, FR-PTY-012.

## Clarifications

### Session 2026-11-16

- Q: Tên scene theo PARTY_SYSTEM §12.4 (`party_lobby.tscn`, `party_hud.tscn`, `invite_dialog.tscn`) hay GODOT_ARCHITECTURE §3? → A: Theo GODOT_ARCHITECTURE §3 (sở hữu quy ước thư mục/đặt tên: scene `PascalCase.tscn`): `scenes/social/PartyLobby.tscn`, `scenes/social/InvitationToast.tscn`, `scenes/social/InviteFriendPicker.tscn`, `scenes/ui/hud/PartyHud.tscn`; script `scripts/multiplayer/party_service.gd`, `party_store.gd`.
- Q: Chat nhóm trong lobby làm ở đâu? → A: F102 chừa slot `EmbeddedChat` (always-on, thu gọn được) theo STUDENT_GAME_UX_SOCIAL §5; nội dung chat panel do F119. Khi F119 chưa có, slot ẩn.
- Q: Ô "Độ khó" trong mockup lobby? → A: Boss/quest không có tham số độ khó (`boss.schema.json`, `quest.schema.json`); `DifficultySelector` chỉ hiện với `coop_challenge`/`learning_challenge` (F105). M4a ẩn.
- Q: Màn chơi boss/quest co-op (thanh năng lượng, thẻ dữ kiện) có thuộc F102? → A: Không. F102 chỉ chuyển cảnh theo `party.session.started`; màn boss S06 và thẻ dữ kiện `interdependent` chưa có feature Godot trong danh mục (ghi Ngoài phạm vi).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tạo nhóm, mời bạn, nhận lời mời (Priority: P1)

Từ HUD (phím `P`) hoặc danh sách bạn bè, học sinh tạo nhóm và mời bạn bằng chạm avatar; người được mời thấy popup nhỏ góc phải và chấp nhận/từ chối (US-PTY-001).

**Why this priority**: Là đường vào duy nhất của mọi trải nghiệm party trên client.

**Independent Test**: Hai instance Godot (`hs.an.3a`, `hs.binh.3a`, `--device-id` khác nhau): A bấm `P` → "Mời bạn bè để cùng chơi" → tạo nhóm → mời B; B thấy `InvitationToast` có đếm ngược, bấm Chấp nhận → cả hai vào lobby.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F102-AC-01 | KHI học sinh chưa có party mở màn Party (phím `P`, nút `PartyChip` hoặc "Mời vào party" từ bạn bè), game client PHẢI hiện trạng thái "Một mình" với nút Mời lớn và tạo party qua `PartyService` (`createParty`, một `Idempotency-Key` cho mỗi ý định, giữ nguyên key khi thử lại) | FR-PTY-001 · [STUDENT_GAME_UX_SOCIAL §5](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | TC-PTY-001 |
| F102-AC-02 | game client PHẢI cho mời bạn từ `InviteFriendPicker` chỉ liệt kê bạn bè/bạn cùng lớp do server trả (nickname + avatar); lớp 1–3 PHẢI mời bằng chạm avatar, không có ô gõ tên tự do | FR-PTY-001 · [PARTY_SYSTEM §5](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-002 |
| F102-AC-03 | KHI nhận `party.invitation.received`, game client PHẢI hiện `InvitationToast` góc phải với nickname người mời, số thành viên và đếm ngược theo `expires_at` của server, tự ẩn sau 10 giây và mở lại được từ chuông thông báo; lời mời hết hạn PHẢI biến mất khỏi danh sách | FR-PTY-001 · AC-PTY-002 | TC-PTY-002 |
| F102-AC-04 | KHI người được mời bấm Chấp nhận mà đang ở party khác (`409 PARTY_ALREADY_IN_PARTY`), game client PHẢI hỏi "Rời nhóm hiện tại và tham gia?" và chỉ gọi `leaveParty` rồi `acceptPartyInvitation` khi học sinh xác nhận | FR-PTY-001 · [PARTY_SYSTEM §13.2](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-003 |
| F102-AC-05 | NẾU API party trả Problem, THÌ game client PHẢI hiển thị thông điệp thân thiện theo khóa `ui.party.error.<CODE>` (`PARTY_FULL` "Nhóm đã đủ người", `PARTY_INVITATION_EXPIRED` "Lời mời đã hết hạn", `PARTY_INVITE_RATE_LIMITED`, `PARTY_INVITE_NOT_ALLOWED`, `PARTY_POLICY_VIOLATION`, `PARTY_NOT_LEADER`, `PARTY_ACTIVITY_NOT_ELIGIBLE`, `PARTY_NOT_READY`) và không hiển thị mã thô | FR-PTY-001 · [STUDENT_GAME_UX_SOCIAL §5](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | — |

---

### User Story 2 — Party Lobby: thấy nhau, sẵn sàng, chọn hoạt động, bắt đầu (Priority: P1)

Các thành viên thấy thẻ của nhau (avatar 2D, level, online, Ready, vương miện leader); leader chọn hoạt động và bấm Bắt đầu; cả nhóm chuyển vào phiên chơi chung (US-PTY-002, US-PTY-003).

**Why this priority**: Màn chính của party; quyết định trải nghiệm chơi chung.

**Independent Test**: B bấm Sẵn sàng → thẻ B trên màn A đổi trong ≤ 1 s; A chọn quest → thông báo Ready reset; cả hai Ready → A bấm Bắt đầu → đếm ngược 3‑2‑1, cả hai tải `scene_ref` của phiên.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F102-AC-06 | game client PHẢI hiển thị mỗi thành viên bằng `PartyMemberCard` (thumbnail avatar 2D render sẵn, nickname, level, presence bằng biểu tượng + chữ, Ready, vương miện leader), không render nhân vật 3D trong lobby, và chỉ vẽ lại thẻ thay đổi | FR-PTY-006 · [STUDENT_GAME_UX_SOCIAL §5](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | TC-PTY-004 |
| F102-AC-07 | KHI nhận message party (`party.member.*`, `party.activity.selected`, `party.leader.transferred`, `party.state.snapshot`), `PartyStore` PHẢI áp theo `state_version` và hiển thị thay đổi trong ≤ 1 giây; NẾU phát hiện lệch phiên bản, THÌ PHẢI tải lại bằng `getParty` | FR-PTY-006 · AC-PTY-003 | TC-PTY-004 |
| F102-AC-08 | KHI học sinh bấm `ReadyToggle`, game client PHẢI gửi `party.member.ready_set` qua `RealtimeClient` (khi WebSocket không sẵn sàng thì dùng `setPartyReady`) và chỉ đổi hiển thị theo trạng thái server xác nhận | FR-PTY-006 · ADR-0016 | TC-PTY-004 |
| F102-AC-09 | game client PHẢI chỉ hiện điều khiển leader (chọn hoạt động, Bắt đầu, menu thẻ "Chuyển trưởng nhóm"/"Mời rời nhóm", sửa cài đặt, giải tán) cho leader hiện tại; nút Bắt đầu PHẢI xám kèm lý do bằng lời ("Còn 1 bạn chưa sẵn sàng") khi chưa đủ điều kiện; mọi kiểm tra quyền vẫn do server | FR-PTY-006 · AC-PTY-004 · AGENTS R1 | TC-PTY-005 |
| F102-AC-10 | game client PHẢI lấy danh sách hoạt động từ `listPartyEligibleActivities`: hoạt động mọi người đủ điều kiện chọn được; hoạt động chỉ một số đủ điều kiện hiện cảnh báo `ui.party.not_eligible` ("{nickname} chưa đủ điều kiện: {reason}") và không chọn được; lớp 1–3 PHẢI dùng 3–4 thẻ hình lớn, lớp 6–9 dùng dropdown | FR-PTY-006 · AC-PTY-005 | TC-PTY-005 |
| F102-AC-11 | KHI nhận `party.session.started`, game client PHẢI đếm ngược 3‑2‑1 rồi chuyển cảnh qua `SceneRouter` tới `scene_ref` và vào `room_id`; KHI nhận `party.session.ended`, PHẢI quay lại lobby với tóm tắt tiến độ chung, không hiển thị điểm học tập của bạn khác | FR-PTY-006 · FR-PTY-001 · AC-PTY-009 | TC-PTY-006 |
| F102-AC-12 | game client PHẢI cho rời nhóm, giải tán (leader), chuyển trưởng nhóm, mời rời nhóm qua hộp xác nhận; thu nhỏ lobby PHẢI giữ party và hiện `PartyHud` (avatar nhỏ + trạng thái từng thành viên) | FR-PTY-001 | TC-PTY-014 |

---

### User Story 3 — Mất kết nối của bạn, của leader và của chính mình (Priority: P1)

Bạn rớt mạng thì thẻ mờ đi; leader rớt mạng thì có băng "Trưởng nhóm đang kết nối lại…" và sau đó thông báo trưởng nhóm mới; chính mình rớt mạng thì băng vàng, thao tác ghi bị khóa, có mạng lại thì tự đồng bộ (US-PTY-004).

**Why this priority**: Mạng trường học không ổn định; người chơi phải hiểu chuyện gì đang xảy ra.

**Independent Test**: Ngắt mạng instance A (leader) quá grace: màn B hiện băng leader đang kết nối lại rồi toast "B là trưởng nhóm mới", điều khiển leader xuất hiện ở B; bật mạng A → A thấy B là leader.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F102-AC-13 | KHI thành viên có trạng thái `disconnected`, game client PHẢI làm mờ thẻ/khung HUD của người đó kèm nhãn chữ; trong phiên chơi chung PHẢI tiếp tục cho người khác chơi bình thường | FR-PTY-012 · AC-PTY-007 | TC-PTY-008 |
| F102-AC-14 | TRONG KHI leader mất kết nối, game client PHẢI hiện băng "Trưởng nhóm đang kết nối lại…" ở lobby và `PartyHud`; KHI nhận `party.leader.transferred` với reason `leader_disconnected`, PHẢI hiện toast "{nickname} là trưởng nhóm mới" và bật điều khiển leader nếu chính mình là leader mới | FR-PTY-008 · AC-PTY-008 | TC-PTY-009 |
| F102-AC-15 | TRONG KHI client mất mạng, game client PHẢI hiện băng vàng "Đang kết nối lại…", khóa mọi thao tác ghi party, giữ dữ liệu đã có để xem; KHI kết nối lại, PHẢI đồng bộ bằng `getMyParty` và, nếu party `in_activity` và mình còn là thành viên phiên, quay lại đúng `room_id` | FR-PTY-012 · AC-PTY-007 · ADR-0031 | TC-PTY-008 |

---

### User Story 4 — Song ngữ, trợ năng, giao diện theo token, an toàn (Priority: P2)

**Why this priority**: Bắt buộc theo constitution (VI, VIII) nhưng kiểm được sau khi luồng chính chạy.

**Independent Test**: Đổi `vi` ↔ `en` trong Cài đặt khi đang mở lobby; chạy `lint_theme_overrides.gd`; scene test bố cục mobile ở 2400×1080.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F102-AC-16 | Mọi chuỗi UI party PHẢI đi qua khóa `ui.party.*` trong `vi.po`/`en.po`, đổi ngôn ngữ áp dụng ngay; bố cục PHẢI chịu chuỗi dài hơn 35% (tối đa 2 dòng rồi rút gọn có tooltip) | NFR-I18N-001 · ADR-0056 | — |
| F102-AC-17 | game client PHẢI lấy màu, font, cỡ chữ, khoảng cách từ Theme sinh từ `design/game-tokens` theo density grade band (vùng chạm ≥ 48 dp, lớp 1–3 ≥ 64 dp), không override trong `.tscn` | NFR-A11Y-001 · ADR-0047 · AGENTS R32 | TC-UI-001, TC-UI-003 |
| F102-AC-18 | game client PHẢI có bố cục mobile ngang (thẻ thành viên ngang, nút Sẵn sàng/Bắt đầu lớn ở dưới, chat trượt từ phải 45% chiều rộng) và PC (hai cột, chat bên phải); trạng thái Ready có biểu tượng + chữ và thông báo vào/ra được đọc khi bật giọng đọc | FR-PTY-006 · NFR-A11Y-001 | — |
| F102-AC-19 | game client PHẢI chỉ hiển thị nickname/avatar/level của người khác (không tên thật, trường, lớp), không ghi dữ liệu party xuống đĩa, và xóa `PartyStore` khi đăng xuất hoặc nhận `session.session.replaced` | AGENTS R16 · ADR-0064 | — |
| F102-AC-20 | Script trong `scenes/**` PHẢI chỉ gọi method public của `PartyService` và đọc `AppState.party`/`PartyStore`; mọi gọi API/WS nằm trong `PartyService` (RefCounted, nhận `ApiClient`/`RealtimeClient` qua constructor) | AGENTS R1 · [GODOT_ARCHITECTURE §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | — |

### Edge Cases

- NẾU `party.party.disbanded` đến khi đang ở lobby, THÌ đóng lobby, toast "Nhóm đã giải tán", HUD bỏ `PartyHud`.
- NẾU bị kick (`party.member.left` reason `kicked` với chính mình), THÌ đóng lobby, toast thân thiện, không hiện lý do chi tiết.
- NẾU đổi hoạt động, THÌ hiện thông báo nhỏ "Đã đổi hoạt động, mọi người sẵn sàng lại".
- NẾU `startPartySession` trả `409 VERSION_MISMATCH`, THÌ tải lại snapshot rồi cho leader bấm lại (không tự gửi lại).
- NẾU nhận lời mời khi đang trong bài kiểm tra/activity player, THÌ chỉ tăng số trên chuông, không bật popup che màn học tập.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-PTY-001 | UI tạo, mời, chấp nhận/từ chối, rời, kick, chuyển leader, giải tán, cài đặt; slot chat nhóm | [PARTY_SYSTEM §2](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) · [STUDENT_GAME_UX_SOCIAL §5](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) |
| FR-PTY-006 | Party Lobby: thẻ thành viên, online, Ready, điều khiển leader, chọn hoạt động, Bắt đầu | [PARTY_SYSTEM §5](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-008 | Hiển thị leader đang kết nối lại và trưởng nhóm mới | [PARTY_SYSTEM §9](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-012 | Hiển thị thành viên mất kết nối, tự đồng bộ và quay lại phòng | [MULTIPLAYER_GAMEPLAY §9](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |

### Key Entities

- **PartyModel / PartyMemberModel / InvitationModel**: model typed parse từ response/message (bỏ qua trường lạ).
- **PartyStore**: trạng thái party phía client theo `state_version` (thành viên, presence, Ready, hoạt động, leader grace).
- **PartyService**: ý định của người chơi (tạo, mời, chấp nhận, ready, chọn, bắt đầu…) gọi REST/WS, phát signal.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `party_invite_ttl` | Chỉ để hiển thị; đếm ngược dùng `expires_at` server |
| `party_leader_grace` | Chỉ để hiển thị băng chờ (thời điểm do server) |
| `client_reconnect_backoff` | Kết nối lại WebSocket (của `RealtimeClient`, F092) |
| `supported_locales` | Ngôn ngữ UI |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Lobby cập nhật ≤ 1 s, không render 3D trong lobby | AC-PTY-003 · [STUDENT_GAME_UX_SOCIAL §12](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) (AC-UX-104) |
| Không business logic trong UI; service → state → UI | AGENTS R1 · [GODOT_ARCHITECTURE §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |
| Theme/token, không hardcode style | AGENTS R32 · ADR-0047 |
| Không hardcode chuỗi; song ngữ | AGENTS §7 · ADR-0056 |
| Không hiển thị tên thật/trường/lớp người khác | AGENTS R16 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh lớp 1–3 tạo nhóm và mời một bạn trong ≤ 3 thao tác từ HUD (thử nghiệm với kịch bản mẫu).
- **SC-002**: Thay đổi Ready của bạn hiển thị trong ≤ 1 giây (p95) trên thiết bị Android tầm trung tham chiếu.
- **SC-003**: Lobby 4 thành viên giữ ≥ 30 FPS trên thiết bị tầm trung tham chiếu (không render 3D trong lobby).
- **SC-004**: 0 màn party hiển thị mã lỗi thô hoặc tên thật/trường/lớp của người khác trong bộ scene test.

## Ngoài phạm vi

- Backend party/lobby/session → F100, F101. `RealtimeClient`, presence và Main Hub nhiều người → F092.
- Nội dung chat nhóm (panel, quick message, báo cáo) → F119; F102 chỉ chừa slot.
- Màn boss S06, HUD phase/năng lượng, thẻ dữ kiện bước `interdependent`, tiến độ quest chung: chưa có feature Godot trong danh mục (đề xuất bổ sung — backend ở F103).
- Thông báo lời mời khi offline (push/notification center) → F123. "Đi theo trưởng nhóm" khi chuyển khu → F091/F092.
- `DifficultySelector` cho `coop_challenge`/`learning_challenge` → F109 (cùng F105).

## Assumptions

- `listPartyEligibleActivities`, `expected_version`, `PARTY_NOT_READY` là delta contract của F101; trước khi F101 Done, client chạy trên Prism mock sinh từ `specs/` (profile `mock`, nhãn MOCK — R17).
- Danh sách bạn trong `InviteFriendPicker` lấy từ `listFriends` (F095) và bạn cùng lớp theo policy; server vẫn kiểm tra khi mời.
- Thumbnail avatar 2D lấy theo cấu hình avatar (cache) như F092/F070.
