# Feature Specification: F095 — Bạn bè, block, mute (module social)

**Feature Branch**: `095-friends-block-mute` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `chat` (module social, schema `social`) · **Mốc**: M4a · **Phụ thuộc**: F090 (F091 cho tín hiệu presence)
**Input**: Học sinh kết bạn với bạn cùng trường (lời mời, mã bạn bè cho bạn ngồi cạnh), xem danh sách bạn kèm trạng thái online, hủy kết bạn lặng lẽ; chặn và tắt tiếng người khác. Quan hệ chặn/tắt tiếng được phát thành event để chat, party, ghép trận, hub tôn trọng. Không có tìm kiếm người lạ. FR nguồn: FR-SOC-001, FR-SOC-002, FR-SOC-003 (US-SOC-001); phần hiển thị presence cho bạn bè của FR-MP-003.

## Clarifications

### Session 2026-11-16

- Q: Route `/social/friends*`, `/social/blocks*`, `/social/mutes*`, `/social/friend-codes*` có qua middleware `game_player`? → A: Có — chỉ học sinh dùng các route này trong game (API_DESIGN §9.10 cột Role = STU); nhân sự nhận `403 GAME_ACCESS_STUDENT_ONLY`. AGENTS R23 chưa liệt kê `/social/*` — bổ sung khi review [Confirmed].
- Q: Giới hạn lời mời theo ngày: `friend_limits` (SOCIAL_SYSTEM §4.1) hay rate limit "10/giờ, 30/ngày" (API_DESIGN §8)? → A: Theo `friend_limits` (SOCIAL_SYSTEM là tài liệu sở hữu, R33); sửa API_DESIGN §8 (task Polish).
- Q: Tenant bật kết bạn liên trường bằng cài đặt nào? → A: `cross_school_social_enabled` trong `platform.tenants.settings` (chỉ áp dụng tenant `unit`), đọc từ projection `ref.tenants.settings_subset` — [Confirmed, RB-003].
- Q: Chặn có hủy kết bạn? → A: Có — chặn kết thúc tình bạn (lặng lẽ, phát `friend.removed`) và hủy lời mời đang chờ hai chiều; bỏ chặn không khôi phục tình bạn — [Confirmed, RB-002].
- Q: Ẩn tin/emote của người bị chặn/tắt tiếng làm ở đâu? → A: Chat ở F115/F118, emote hub phía client ở F119; ghép trận/party ở F100/F104; hub instance ở F091. F095 chỉ sở hữu quan hệ và phát event.
- Q: Ai gửi presence cho bạn bè? → A: module chat (sở hữu quan hệ bạn bè) nghe tín hiệu presence của gateway (F091 research R3) và gửi `presence.friend.updated`.

### Session 2026-11-17

- Q: Xác nhận sau đối soát? → A: Chặn kết thúc tình bạn (RB-002) và cài đặt `cross_school_social_enabled` (RB-003, [CONFIG_DEFAULTS](../../docs/00-overview/CONFIG_DEFAULTS.md), [SOCIAL_SYSTEM §7](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md)) đã chốt; nickname `maxLength` 32 mọi nơi (RB-302); presence theo enum chung MULTIPLAYER_GAMEPLAY §5, ẩn trạng thái là cờ `hidden` (RB-404).
- Q: Tenant `common` tắt kết bạn và mã bạn bè (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

- Q: `common_tenant_social_defaults` bật lại bằng gì khi F120 T059 (`SetCommonTenantSocialOverride`) chưa có? → A: Module chat đọc khóa `common_social_enabled` trong `ref.tenants.settings_subset` (vắng = tắt) [Confirmed]; client đọc cờ `social.friends_enabled` của `PlayerBootstrap.feature_flags` (module identity: `false` ở tenant `common`, cờ cùng tên của System Admin quyết định khi có) [Confirmed]. F120 T059 ghi cả hai nguồn.
- Q: Module identity chưa phát `settings_subset` trong `tenant.updated` (F015) — `cross_school_social_enabled` đến module chat bằng cách nào? → A: Consumer `chat.projection_identity` đã chiếu `settings_subset` khi có; phía phát thuộc F015 (câu hỏi mở cho F015, ghi ở mục Assumptions). Test seed projection trực tiếp.
- Q: Presence của bạn đang thi (`activity = exam`) hiển thị gì? → A: `status` giữ nguyên, `activity = learning`, không có `area_id` [Confirmed].
- Q: `presence_ttl` 90 s nhưng gateway chỉ phát tín hiệu khi đổi trạng thái — học sinh online lâu có bị coi `offline`? → A: Gateway phát lại trạng thái không đổi với `reason = refresh` tối đa mỗi `presence_ttl`/3 khi kết nối còn sống (heartbeat); module chat chỉ gia hạn TTL, không gửi `presence.friend.updated` khi bạn bè không thấy thay đổi [Confirmed]; enum `reason` của `presence-changed.schema.json` thêm `refresh`.
- Q: Consumer, relay và fan-out presence chạy ở binary nào? → A: Route `/social/*` và subscriber `ew.rt.presence.*` (queue group `chat.presence_fanout`) trong `core-api`; relay + consumer projection trong `worker --profile=events`; job `expire-friend-requests` trong `--profile=jobs` — như các module khác; profile `chat` để dành cho `chat-persister`/purge (F115/F117) [Confirmed].
- Q: `unmuteUser` khi chưa tắt tiếng? → A: `204` (idempotent như AC-17), không event; `unblockUser` khi chưa chặn vẫn `404` theo AC-16.
- Q: Giới hạn số bạn khi chấp nhận? → A: Kiểm cả hai người (người chấp nhận hoặc người gửi đã đủ `friend_limits` → `409 FRIEND_LIMIT_REACHED`) [Confirmed].
- Q: Lời mời/ngày đếm ở đâu? → A: Đếm trong `social.friend_requests` (mọi lời mời đã tạo từ đầu ngày platform `Asia/Ho_Chi_Minh`), không dùng counter Valkey; `Retry-After` = số giây tới ngày mới [Confirmed].
- Q: Hủy lời mời khi học sinh bị dọn/chuyển? → A: Consumer `chat.projection_identity` hủy lời mời `pending` của học sinh khi nhận `student.cleaned_up` hoặc `student.synced` có `status` khác `active` (module identity chưa gọi facade `cleanupStudentData` của chat) [Confirmed].

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Gợi ý "Bạn cùng lớp" và ngoại lệ tên thật cho bạn cùng lớp? → A: bỏ (ADR-0095 mục 5); `PlayerCard` giữ nguyên chỉ nickname/avatar/level/presence; không thêm gợi ý theo khối. Không đổi code/AC của F095 (chưa từng hiện thực ngoại lệ này).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Gửi và nhận lời mời kết bạn trong phạm vi trường (Priority: P1)

Học sinh lớp 7 gửi lời mời kết bạn tới bạn cùng trường gặp ở hub, hoặc đọc mã bạn bè cho bạn ngồi cạnh nhập trong phòng máy; người nhận chấp nhận hoặc từ chối lặng lẽ (US-SOC-001).

**Why this priority**: Bạn bè là nền cho mời party (P4), DM và "Tham gia bạn".

**Independent Test**: `hs.an.3a` gửi lời mời tới `hs.binh.3a` → `hs.binh.3a` chấp nhận → cả hai thấy nhau trong `listFriends`; `hs.an.3a` gửi tới `hs.dung.7a` (tenant khác) → `404`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F095-AC-01 | KHI học sinh gọi `sendFriendRequest` (`addressee_id`, `Idempotency-Key`) tới học sinh cùng trường chưa là bạn, module chat PHẢI tạo lời mời `pending` hết hạn sau `friend_request_ttl`, trả `201 FriendRequest`, phát `friend_request.sent` và audit trong cùng transaction | FR-SOC-001 · US-SOC-001 | TC-SOC-001 |
| F095-AC-02 | NẾU người nhận khác trường và tenant không bật kết bạn liên trường, hoặc hai người đã chặn nhau (bất kỳ chiều), THÌ module chat PHẢI trả `403 SOCIAL_SCOPE_VIOLATION` mà không tiết lộ chiều chặn; NẾU người nhận thuộc tenant khác hoặc không phải học sinh hoạt động, THÌ PHẢI trả `404` | FR-SOC-002 · AC-SOC-001 · ADR-0054 | TC-SOC-001, TC-TEN-001 |
| F095-AC-03 | NẾU người gửi đã có tối đa lời mời đang chờ hoặc tối đa số bạn theo `friend_limits`, THÌ module chat PHẢI trả `409 FRIEND_LIMIT_REACHED`; NẾU vượt số lời mời/ngày của `friend_limits`, THÌ PHẢI trả `429 RATE_LIMITED` kèm `Retry-After` | FR-SOC-001 · [SOCIAL_SYSTEM §4.1](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-002 |
| F095-AC-04 | NẾU đã là bạn, đã có lời mời `pending` cùng cặp, hoặc gửi cho chính mình, THÌ module chat PHẢI trả `409 CONFLICT` (hai trường hợp đầu) hoặc `422 VALIDATION_FAILED`; gửi lại cùng `Idempotency-Key` PHẢI trả cùng kết quả, không tạo lời mời thứ hai | FR-SOC-001 · NFR-SEC-004 | — |
| F095-AC-05 | KHI người nhận gọi `acceptFriendRequest` (`Idempotency-Key`) với lời mời `pending` còn hạn của mình, module chat PHẢI tạo đúng một tình bạn (cặp chuẩn hóa thứ tự), đặt lời mời `accepted` và phát `friend.added`; người khác gọi hoặc lời mời đã hết hạn/đã xử lý PHẢI nhận `404` | FR-SOC-001 | — |
| F095-AC-06 | KHI người nhận gọi `rejectFriendRequest`, module chat PHẢI đặt `rejected` mà không phát event hay thông báo tới người gửi; người gửi PHẢI tiếp tục thấy lời mời `pending` trong `listFriendRequests?direction=outgoing` tới khi hết hạn | FR-SOC-001 · AC-SOC-003 | — |
| F095-AC-07 | TRONG KHI lời mời `pending` quá `friend_request_ttl`, job hết hạn PHẢI đặt `expired` và không gửi thông báo | FR-SOC-001 · AC-SOC-003 | — |
| F095-AC-08 | KHI học sinh gọi `createFriendCode`, module chat PHẢI trả mã khớp `^[A-Z0-9]{6,12}$` hết hạn sau `friend_code_ttl` (mã mới thay mã cũ của người đó); KHI học sinh khác gọi `redeemFriendCode` với mã còn hạn, PHẢI tạo lời mời như F095-AC-01 với cùng kiểm tra phạm vi; mã sai/hết hạn PHẢI trả `404 FRIEND_CODE_INVALID`; mã sai định dạng PHẢI trả `422 VALIDATION_FAILED` | FR-SOC-001 · FR-SOC-002 · [SOCIAL_SYSTEM §4.1](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-001 |
| F095-AC-09 | HỆ THỐNG PHẢI không có API tìm người chơi theo tên, username hay mã định danh tự do | FR-SOC-002 · AC-SOC-002 | — |
| F095-AC-10 | NẾU token không phải token game của học sinh (`school_admin`, `academic_manager`, `tenant_admin`, `safety_moderator`, `system_admin`), THÌ mọi route bạn bè/chặn/tắt tiếng PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` | AGENTS R23 · ADR-0031 | TC-AUTH-014 |
| F095-AC-23 | TRONG KHI tenant của học sinh là `common` và System Admin chưa bật lại kết bạn cho tenant đó (`common_tenant_social_defaults`), module chat PHẢI từ chối `sendFriendRequest`, tạo/đổi mã bạn bè và thêm bạn bằng mã với `403 SOCIAL_SCOPE_VIOLATION`, không tạo lời mời hay thông báo; client-config/bootstrap PHẢI báo tính năng tắt để UI ẩn nút | [CHILD_SAFETY §3.6](../../docs/10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078) · AC-CHAT-015 · ADR-0078 | TC-CHAT-047 |

---

### User Story 2 — Danh sách bạn bè và hủy kết bạn (Priority: P1)

Học sinh xem danh sách bạn (nickname, avatar, level, trạng thái) và hủy kết bạn bằng một chạm mà người kia không được báo.

**Why this priority**: Điểm vào của mời party, nhắn tin, "Tham gia bạn".

**Independent Test**: `listFriends` của `hs.an.3a` chỉ có bạn của mình, không chứa tên thật/trường/lớp/điểm; `removeFriend` → hai bên không còn thấy nhau, không có thông báo.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F095-AC-11 | KHI học sinh gọi `listFriends`, module chat PHẢI trả (cursor) chỉ bạn bè của chính mình trong tenant phiên dạng `PlayerCard` (`user_id`, `nickname`, `avatar_config`, `level`, `presence`), không chứa tên thật, trường, lớp, điểm học tập, mastery hay kết quả kiểm tra | FR-SOC-001 · AC-SOC-010 · AGENTS R16 | TC-TEN-001 |
| F095-AC-12 | KHI học sinh gọi `removeFriend` với một người bạn, module chat PHẢI soft-delete tình bạn, phát `friend.removed` và không gửi thông báo cho người kia; người không phải bạn PHẢI nhận `404` | FR-SOC-001 · [SOCIAL_SYSTEM §4.1](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | — |
| F095-AC-13 | KHI học sinh gọi `listFriendRequests` với `direction`, module chat PHẢI trả lời mời đến (`pending` còn hạn) hoặc đi của chính mình kèm `PlayerCard` của bên kia; lời mời của người khác PHẢI không bao giờ xuất hiện | FR-SOC-001 · NFR-SEC-006 | TC-TEN-001 |

---

### User Story 3 — Chặn và tắt tiếng (Priority: P1)

Học sinh bị làm phiền chặn người kia (không còn nhận tin, lời mời, không bị ghép cùng) hoặc chỉ tắt tiếng (ẩn tin/emote của người đó với mình); người bị chặn/tắt tiếng không được báo.

**Why this priority**: An toàn trẻ em (FR-SOC-003, CHILD_SAFETY); block cần sẵn trước party P4.

**Independent Test**: `hs.an.3a` chặn `hs.binh.3a` → tình bạn kết thúc, lời mời hai chiều bị từ chối `SOCIAL_SCOPE_VIOLATION`, event `user.blocked` có trên NATS; `muteUser` → `user.muted`, `listMutes` có `hs.binh.3a`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F095-AC-14 | KHI học sinh gọi `blockUser` với học sinh cùng tenant, module chat PHẢI ghi quan hệ chặn (idempotent: gọi lại trả `204`), kết thúc tình bạn nếu có, hủy lời mời kết bạn đang chờ hai chiều, phát `user.blocked` (và `friend.removed` khi có) cùng audit trong một transaction; người bị chặn PHẢI không nhận thông báo | FR-SOC-003 · AC-SOC-004 | TC-CHAT-005 |
| F095-AC-15 | TRONG KHI quan hệ chặn còn hiệu lực (bất kỳ chiều), module chat PHẢI từ chối `sendFriendRequest`/`redeemFriendCode` giữa hai người với `403 SOCIAL_SCOPE_VIOLATION` | FR-SOC-003 · AC-SOC-004 | TC-CHAT-005 |
| F095-AC-16 | KHI học sinh gọi `unblockUser`, module chat PHẢI soft-delete quan hệ chặn, phát `user.unblocked` và không khôi phục tình bạn; bỏ chặn người chưa chặn PHẢI trả `404` | FR-SOC-003 | — |
| F095-AC-17 | KHI học sinh gọi `muteUser`/`unmuteUser`, module chat PHẢI ghi/soft-delete quan hệ tắt tiếng (idempotent) và phát `user.muted`/`user.unmuted`; quan hệ chỉ ảnh hưởng người tắt tiếng, người bị tắt tiếng không được báo | FR-SOC-003 · [SOCIAL_SYSTEM §5](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-003 |
| F095-AC-18 | NẾU `user_id` đích là chính mình, THÌ module chat PHẢI trả `422 VALIDATION_FAILED`; NẾU đích không phải học sinh trong tenant phiên, THÌ PHẢI trả `404` | FR-SOC-003 · NFR-SEC-006 | TC-TEN-001 |
| F095-AC-19 | KHI học sinh gọi `listBlocks`/`listMutes`, module chat PHẢI chỉ trả quan hệ do chính học sinh tạo (`PlayerRelation`: nickname, avatar, thời điểm), không bao giờ cho biết ai đã chặn mình | FR-SOC-003 · AGENTS R16 | — |

---

### User Story 4 — Bạn bè thấy trạng thái online (Priority: P2)

Học sinh thấy bạn nào đang online, đang học hay đang thi đấu để rủ chơi cùng; bật "Ẩn trạng thái" thì bạn bè thấy mình offline.

**Why this priority**: FR-MP-003 (phần hiển thị), làm danh sách bạn bè có giá trị.

**Independent Test**: `hs.binh.3a` kết nối → `hs.an.3a` (bạn, đang online) nhận `presence.friend.updated {status: online}`; `hs.chi.3b` (không phải bạn) không nhận gì.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F095-AC-20 | KHI nhận tín hiệu presence của học sinh U, module chat PHẢI gửi `presence.friend.updated` (`user_id`, `status`, `activity`, `area_id`) tới bạn bè đang online của U không có quan hệ chặn với U, và không gửi tới bất kỳ ai khác | FR-MP-003 · FR-SOC-001 · [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |
| F095-AC-21 | TRONG KHI U ẩn trạng thái, module chat PHẢI gửi và trả về U ở trạng thái `offline` cho bạn bè; TRONG KHI U đang làm bài kiểm tra, `activity` PHẢI ở dạng chung không lộ chi tiết bài | FR-MP-003 · [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |
| F095-AC-22 | `listFriends` PHẢI trả `presence` theo trạng thái gần nhất đã nhận (TTL `presence_ttl`), mặc định `offline` khi không có dữ liệu | FR-MP-003 · ADR-0004 | — |

### Edge Cases

- NẾU A và B gửi lời mời cho nhau gần như đồng thời, THÌ cả hai lời mời tồn tại; chấp nhận một lời mời tạo tình bạn và đặt lời mời ngược lại `cancelled` (unique cặp tình bạn là chốt chặn).
- NẾU học sinh chuyển trường khác tenant hoặc bị dọn (`student.cleaned_up`), THÌ lời mời đang chờ bị `cancelled`; tình bạn khác tenant không còn hiển thị (lọc theo tenant phiên).
- NẾU bị chặn giữa lúc lời mời đang chờ, THÌ lời mời bị `cancelled` (F095-AC-14).
- NẾU tín hiệu presence mất (NATS core), THÌ trạng thái tự về `offline` sau `presence_ttl`; không ảnh hưởng dữ liệu bền.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-SOC-001 | Lời mời (gửi/nhận/chấp nhận/từ chối lặng lẽ/hết hạn), mã bạn bè, danh sách bạn, hủy kết bạn | [SOCIAL_SYSTEM §4](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |
| FR-SOC-002 | Phạm vi cùng trường mặc định; liên trường trong tenant `unit` khi bật; không khác tenant; không tìm kiếm tự do | [ADR-0054](../../docs/adr/ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) · [SOCIAL_SYSTEM §7](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |
| FR-SOC-003 | Quan hệ chặn/tắt tiếng, event cho chat/competition/hub; chặn lời mời kết bạn | [SOCIAL_SYSTEM §5](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |
| FR-MP-003 (phần hiển thị) | Gửi presence cho bạn bè, ẩn trạng thái | [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |

### Key Entities

- **FriendRequest**: lời mời giữa hai học sinh, trạng thái `pending/accepted/rejected/cancelled/expired`, hạn.
- **Friendship**: quan hệ đối xứng giữa hai học sinh cùng tenant.
- **FriendCode**: mã ngắn hạn của một học sinh để bạn ngồi cạnh nhập.
- **Block**: quan hệ một chiều, hiệu lực hai chiều với tương tác.
- **Mute**: quan hệ một chiều, chỉ ảnh hưởng người tắt tiếng.
- **Hồ sơ công khai, học sinh, tenant**: bản sao tối thiểu từ module game/identity để hiển thị và kiểm phạm vi.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `friend_limits` | Số lời mời đang chờ, lời mời/ngày, số bạn tối đa |
| `friend_request_ttl` / `friend_code_ttl` | Hạn lời mời; hạn mã bạn bè |
| `presence_ttl` | Hạn trạng thái presence đã nhận |
| `idempotency_record_ttl` | Lưu kết quả request có `Idempotency-Key` |
| `api_rate_default` | Rate limit chung các route đọc/ghi |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Bảng `social.*` có `tenant_id`, soft-delete, RLS; unique partial `WHERE deleted_at IS NULL` | AGENTS R8, R21, R27 · ADR-0028 |
| Lời mời idempotent + audit; event qua outbox cùng transaction | AGENTS R10, R26 · ADR-0042 |
| Chỉ nickname + avatar; không tên thật/trường/lớp; không lộ ai chặn mình | AGENTS R16 · ADR-0054 |
| Không log PII; event chỉ ID | AGENTS R14 · NFR-PRIV-002 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 lời mời kết bạn thành công giữa học sinh khác tenant hoặc khác trường (khi tenant chưa bật) trong bộ test phạm vi.
- **SC-002**: 100% hành động chặn làm lời mời kết bạn giữa hai người bị từ chối ngay sau khi chặn, và event `user.blocked` được phát đúng một lần.
- **SC-003**: Người gửi không phân biệt được lời mời bị từ chối với lời mời chưa trả lời trước khi hết hạn.
- **SC-004**: Bạn bè nhận thay đổi trạng thái online trong ≤ 2 giây; 0 trường hợp người không phải bạn nhận presence.

## Ngoài phạm vi

- Ẩn tin chat của người bị chặn/tắt tiếng, DM chỉ giữa bạn bè → F115/F118/F120; chặn mời party, ghép trận, thách đấu → F100, F104; tránh cùng hub instance → F091 (tiêu thụ event).
- Thông báo trong game cho lời mời kết bạn → F123; báo cáo người dùng → F121.
- Club/guild, study group → F120 (FR-SOC-004).
- UI bạn bè trong Godot: chưa có feature trong danh mục (đề xuất gắn F102 hoặc thêm feature mới).
- ~~Hiển thị tên thật cho bạn cùng lớp~~ — bỏ theo ADR-0095 (không còn ngoại lệ hiển thị cho "bạn cùng lớp"; mọi người chơi khác chỉ thấy nickname + avatar, R16).

## Assumptions

- F095 là feature đầu tiên của module chat: dựng skeleton module và projection `ref.tenants`, `ref.students`, `ref.player_profiles` (F070 phát `game_profile.updated`).
- Cài đặt liên trường `cross_school_social_enabled` [Confirmed] — tenant chưa có cài đặt thì coi là tắt.
- Tín hiệu presence của F091 (`ew.rt.presence.<tenant_id>`) sẵn sàng; nếu chưa, US4 chờ F091 Done (US1–US3 độc lập).
- A-1 (triển khai) Module identity chưa đưa `settings_subset` vào `tenant.updated` (owner F015): tới khi có, tenant `unit` chưa bật được `cross_school_social_enabled` và System Admin chưa bật lại được kết bạn cho tenant `common` qua sự kiện (mặc định an toàn: tắt). Logic và test phía chat đã đủ. **Đã xử lý 2026-11-21 (F015 T044):** `updateTenantSettings` ghi `cross_school_social_enabled` và `tenant.updated` mang `settings_subset` đủ hai khóa; `common_social_enabled` vẫn chờ F120 T059.
- A-2 (triển khai) Giao diện bạn bè Godot không thuộc F095 (đề xuất gắn F110/F102). AGENTS R23 nên liệt kê `/social/*` phía người chơi — đề xuất khi review, chưa sửa AGENTS.
- A-3 (triển khai) `listFriendRequests` không có cursor trong contract: trả tối đa 100 lời mời mới nhất [Confirmed]. Rate limit `api_rate_default` cho redeem mã là việc của gateway (APISIX), không trong module.
