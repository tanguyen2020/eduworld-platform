# Feature Specification: F091 — Hub instance, presence, đồng bộ vị trí

**Feature Branch**: `091-hub-instance-presence` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` (room host hub) + `realtime` (presence) · **Mốc**: M2 · **Phụ thuộc**: F070, F090 (dữ liệu area runtime và mở khóa từ F071)
**Input**: Học sinh vào một khu chung (Main Hub, khu học tập, Social Zone…) được gán vào bản sao (hub instance) tối đa 40 người cùng trường và cùng nhóm khối, ưu tiên ở cùng bạn bè; thấy người khác di chuyển mượt nhờ snapshot 10 Hz do server kiểm tra tốc độ/biên; trạng thái online/khu vực được ghi tạm để tính năng xã hội dùng. FR nguồn: FR-MP-001, FR-MP-002, FR-MP-003, FR-MP-004, FR-MP-006; NFR-PERF-008.

## Clarifications

### Session 2026-11-16

- Q: Hub instance chạy ở module nào? CODE_PLAN ghi `realtime` + module `game`, AsyncAPI ghi `hub.*` → module game, còn REALTIME_ARCHITECTURE v0.3 §1/§4.1/§6, MULTIPLAYER_GAMEPLAY §4, VALKEY_DESIGN (owner `competition · room`), DATABASE_DESIGN §2.9 (`multiplayer.rooms`) và AGENTS §6 ghi module competition → A: **module competition** là room host (tài liệu sở hữu, R33); gateway chỉ định tuyến và giữ presence online/offline. CODE_PLAN và AsyncAPI cần sửa ([research R1](research.md)).
- Q: Lấy capacity/tốc độ tối đa/biên/khối được vào của area ở đâu khi area thuộc module game (F071)? → A: Projection `ref.world_areas` và `ref.area_unlocks` trong `eduworld_competition` từ event của module game; không gọi đồng bộ trên đường vào khu ([research R2](research.md)).
- Q: REST `enterWorldArea`, `joinFriendInWorld` (game.v1) gán instance? → A: Gán instance thuộc room host; "Tham gia bạn" làm qua `hub.instance.join` có `follow_user_id` (delta AsyncAPI). Hai endpoint REST cần thống nhất với F071 (contracts §6).
- Q: Ai hiển thị presence cho bạn bè? → A: F091 chỉ ghi presence tạm và phát tín hiệu thay đổi; gửi `presence.friend.updated` cho bạn bè thuộc F095 (module chat sở hữu quan hệ bạn bè).
- Q: Tập trạng thái presence khác nhau giữa MULTIPLAYER_GAMEPLAY §5 (`in_activity`, `in_match`) và AsyncAPI (`busy`, `invisible`)? → A: Thay bởi Session 2026-11-17 (enum theo MULTIPLAYER_GAMEPLAY §5).

### Session 2026-11-17

- Q: Enum presence sau đối soát (RB-404)? → A: Một enum duy nhất theo [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md): `offline`, `online`, `in_activity`, `in_match`, `away` (dùng chung AsyncAPI, `PlayerCard`, REST); bỏ `busy`/`invisible`. "Ẩn trạng thái" là cờ `hidden` riêng, người không được thấy nhận `offline`. Delta AsyncAPI ở contracts §6.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Vào khu và được gán hub instance đúng phạm vi (Priority: P1)

Học sinh lớp 3 vào Main Hub thấy các bạn cùng trường, cùng nhóm khối; bạn bè đang online được xếp chung khi còn chỗ; không bao giờ gặp học sinh lớp lớn ở khu chung.

**Why this priority**: An toàn trẻ em (ADR-0012, ADR-0054) và là nền cho mọi tương tác trong thế giới.

**Independent Test**: Bot WS `hs.an.3a`, `hs.binh.3a` (bạn bè, fixture `friend.added`) gửi `hub.instance.join {area_id: main_hub}` → cùng `room_id`; 40 bot khác cùng partition → bot thứ 41 sang instance mới.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F091-AC-01 | KHI học sinh gửi `hub.instance.join` với `area_id` hợp lệ, module competition PHẢI gán một hub instance thuộc partition `(tenant, school_id, grade_band)` của học sinh và trả `hub.instance.assigned` (`room_id`, `area_id`, `self_eid`, `spawn`, `tick_rate_hz`, `max_speed_mps`, `players[]`) kèm yêu cầu subscribe output của room | FR-MP-001 · FR-MP-002 · [MULTIPLAYER_GAMEPLAY §6](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |
| F091-AC-02 | module competition PHẢI không bao giờ đặt hai học sinh khác trường hoặc khác `grade_band` vào cùng hub instance theo cấu hình mặc định (`g1_3` và `g8_9` không bao giờ chung) | FR-MP-002 · AC-MP-002 · ADR-0054 | — |
| F091-AC-03 | KHI chọn instance, module competition PHẢI ưu tiên instance có nhiều bạn bè online nhất còn chỗ, rồi instance đông nhất dưới 90% sức chứa, nếu không có thì tạo instance mới; PHẢI tránh xếp chung người đã chặn nhau khi còn lựa chọn khác | FR-MP-002 · AC-MP-001 · [SOCIAL_SYSTEM §5](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | — |
| F091-AC-04 | KHI instance đạt `hub_instance_capacity`, module competition PHẢI gán người tiếp theo vào instance khác hoặc instance mới của cùng partition | FR-MP-002 · AC-MP-003 | TC-PERF-003 |
| F091-AC-05 | KHI `hub.instance.join` có `follow_user_id` (Tham gia bạn), module competition PHẢI chỉ đưa học sinh vào instance của người đó nếu hai người là bạn bè, không chặn nhau, người đó ở hub instance công khai cùng partition còn chỗ; nếu không PHẢI trả `ROOM_FULL` hoặc `PERMISSION_DENIED` mà không tiết lộ vị trí của người đó | AC-SOC-005 · AC-MP-010 · [SOCIAL_SYSTEM §4.2](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | — |
| F091-AC-06 | NẾU `area_id` không tồn tại, không active, không cho khối của học sinh hoặc cần mở khóa mà học sinh chưa mở, THÌ module competition PHẢI trả `system.command.failed` (`NOT_FOUND`/`PERMISSION_DENIED`) và không tạo room; chuỗi `area_id`, `spawn_point` vượt `maxLength` PHẢI bị từ chối `VALIDATION_FAILED` | FR-MP-001 · ADR-0041 | — |
| F091-AC-07 | NẾU lệnh `hub.*` tham chiếu room mà người gửi không phải thành viên hoặc room thuộc tenant khác tenant của kết nối, THÌ module competition PHẢI trả `ROOM_NOT_MEMBER` và room không bị ảnh hưởng | AC-MP-008 · NFR-SEC-006 | TC-TEN-012 |
| F091-AC-08 | module competition PHẢI chỉ xử lý lệnh `hub.*` đến từ NATS account của gateway và lấy danh tính, tenant từ header `Ew-*`, không từ payload | AGENTS R7 · [REALTIME_ARCHITECTURE §2.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F091-AC-09 | KHI thành viên mất kết nối, module competition PHẢI giữ chỗ trong `disconnect_grace` của hub (avatar đứng yên); quay lại trong thời gian đó PHẢI về đúng instance cũ; hết thời gian PHẢI gỡ khỏi instance và phát `hub.player.left` | FR-MP-005 · AC-MP-004 · [MULTIPLAYER_GAMEPLAY §9.2](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |
| F091-AC-10 | KHI instance rỗng quá `hub_instance_idle_close`, module competition PHẢI đóng instance, ghi thời điểm đóng vào bản ghi room tóm tắt và dừng tick | FR-MP-001 · ADR-0004 | — |
| F091-AC-11 | TRONG KHI ghi nhận thành viên hub được bật cho tenant, module competition PHẢI ghi lượt vào/ra hub của học sinh (room, thời điểm, lý do rời) để phục vụ điều tra báo cáo | [MULTIPLAYER_GAMEPLAY §10](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |

---

### User Story 2 — Đồng bộ vị trí 10 Hz có kiểm tra phía server (Priority: P1)

Học sinh thấy bạn đi lại, vẫy tay mượt mà; ai cố "dịch chuyển tức thời" bị đưa về vị trí hợp lệ.

**Why this priority**: FR-MP-004, NFR-PERF-008; thiếu đồng bộ thì hub không có người.

**Independent Test**: 40 bot gửi `hub.player.move` 10 Hz → mỗi bot nhận `hub.snapshot.updated` 10 Hz chỉ gồm thực thể trong ô lân cận; bot gửi vị trí nhảy 50 m → nhận `hub.player.corrected {reason: speed}`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F091-AC-12 | KHI nhận `hub.player.move`, room host PHẢI kiểm quãng đường ≤ `max_speed_mps` × Δt × 1,2 và vị trí nằm trong biên area; hợp lệ thì cập nhật trạng thái, không hợp lệ thì giữ vị trí hợp lệ cuối, gửi `hub.player.corrected` (`reason` = `speed`/`out_of_bounds`) cho người gửi và tăng đếm vi phạm | FR-MP-004 · AC-MP-007 · ADR-0011 | — |
| F091-AC-13 | NẾU một học sinh vượt 20 vi phạm/phút, THÌ room host PHẢI ghi tín hiệu gian lận (integrity signal) và audit, không tự động phạt | [REALTIME_ARCHITECTURE §6](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) · [ANTI_CHEAT](../../docs/10-security/ANTI_CHEAT.md) | — |
| F091-AC-14 | room host PHẢI tick theo `hub_sync_rate` và gửi mỗi người xem `hub.snapshot.updated` (`tick`, `entities` delta, `removed`, `ack_client_seq`) chỉ gồm thực thể trong ô interest hiện tại và 8 ô lân cận, đóng gói theo gateway node, đánh dấu droppable | FR-MP-004 · NFR-PERF-008 · [REALTIME_ARCHITECTURE §6](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | TC-PERF-003 |
| F091-AC-15 | Snapshot và `hub.player.joined` PHẢI chỉ chứa `eid`, vị trí, hướng, animation và `PublicPlayer` (`nickname`, `avatar_config`, `level`), không chứa tên thật, trường, lớp hay `user_id` ngoài `PublicPlayer` | AGENTS R16 · [MULTIPLAYER_GAMEPLAY §11](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |
| F091-AC-16 | Ở instance 40 người, băng thông xuống mỗi người PHẢI ≤ `hub_sync_bandwidth` và thời gian tick p95 PHẢI đạt ngân sách room tick ([REALTIME_ARCHITECTURE §8](../../docs/06-architecture/REALTIME_ARCHITECTURE.md)) | NFR-PERF-008 · AC-PERF-009 | TC-PERF-003, TC-PERF-023 |
| F091-AC-17 | HỆ THỐNG PHẢI không có endpoint REST nhận chuyển động 3D và không dùng Valkey để mô phỏng game: trạng thái tick nằm trong bộ nhớ room host, Valkey chỉ giữ registry/lease/vị trí gần nhất có TTL | FR-MP-006 · ADR-0004 | — |
| F091-AC-18 | KHI nhận `hub.player.emote`, room host PHẢI chỉ phát `hub.player.emoted` nếu `emote_id` thuộc avatar của người gửi và không vượt `emote_rate`; ngược lại trả `PERMISSION_DENIED`/`RATE_LIMITED` | FR-MP-004 · [CHARACTER_SYSTEM](../../docs/01-game-design/CHARACTER_SYSTEM.md) | — |
| F091-AC-19 | KHI nhận `game_profile.updated` của học sinh đang ở instance, room host PHẢI phát `hub.player.appearance_changed` (slot → item) cho người cùng instance | FR-GAME-004 · [EVENT_DESIGN §3.2](../../docs/07-backend/EVENT_DESIGN.md) | — |
| F091-AC-20 | NẾU instance room host mất lease, THÌ instance module competition khác PHẢI nhận quyền, dựng lại trạng thái từ input kế tiếp và gửi `hub.instance.migrated` cho thành viên | [REALTIME_ARCHITECTURE §5.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |

---

### User Story 3 — Presence tạm thời (Priority: P2)

Hệ thống biết học sinh đang online, đang ở khu nào, đang làm gì để bạn bè (F095) và party (F101) dùng; không ai khác thấy.

**Why this priority**: FR-MP-003; cần cho danh sách bạn bè và "Tham gia bạn".

**Independent Test**: Mở kết nối → presence `online`; vào hub → `area_id` cập nhật; đóng kết nối quá `ws_timeout` → `offline`; mỗi thay đổi có một tín hiệu presence trên NATS.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F091-AC-21 | KHI kết nối được chấp nhận hoặc kết thúc, gateway PHẢI ghi presence `online`/`offline` với TTL `presence_ttl` và phát một tín hiệu thay đổi presence (không PII: `user_id`, `tenant_id`, `school_id`, `status`, `activity`, `area_id`, `ts`) | FR-MP-003 · [VALKEY_DESIGN §4.2](../../docs/07-backend/VALKEY_DESIGN.md) | — |
| F091-AC-22 | KHI nhận `presence.status.set`, gateway PHẢI cập nhật `status` theo enum presence chung (`online`/`in_activity`/`in_match`/`away`; `offline` chỉ do gateway đặt), `activity` và cờ `hidden` (ẩn trạng thái) rồi trả ack; giá trị ngoài enum PHẢI bị từ chối `VALIDATION_FAILED` | FR-MP-003 · [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |
| F091-AC-23 | KHI room host gán hoặc gỡ học sinh khỏi instance, presence PHẢI cập nhật `area_id`/`room_id` tương ứng | FR-MP-003 | — |
| F091-AC-24 | KHI kết nối đóng `4408`, presence PHẢI chuyển `away`; NẾU không có kết nối mới trong `ws_timeout`, THÌ PHẢI chuyển `offline` | FR-MP-003 · [REALTIME_ARCHITECTURE §9](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F091-AC-25 | HỆ THỐNG PHẢI không trả presence của học sinh cho client nào qua F091 và không ghi presence vào PostgreSQL | FR-MP-003 · AGENTS R9, R16 | — |

### Edge Cases

- NẾU học sinh vào lại cùng khu trong `disconnect_grace`, THÌ về instance cũ kể cả khi instance đã đầy (chỗ được giữ) (F091-AC-09).
- NẾU partition của học sinh đổi (chuyển trường cùng tenant `unit`), THÌ lần `hub.instance.join` kế tiếp dùng partition mới; instance hiện tại giữ tới khi rời.
- NẾU Valkey mất, THÌ room đang chạy giữ trạng thái trong bộ nhớ nhưng không giao được snapshot xuyên node; client nhận `system.service.degraded` (F090), snapshot đầy đủ khi phục hồi.
- NẾU projection `ref.world_areas` chưa có area, THÌ trả `NOT_FOUND` (không tạo room với cấu hình mặc định).
- NẾU học sinh bị thu hồi quyền giữa chừng, THÌ consumer `user.access_revoked` gỡ khỏi instance ngay, không chờ grace.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-MP-001 | Room loại `hub_instance` (vòng đời, bản ghi tóm tắt, membership); các loại room khác ở F100–F105 | [MULTIPLAYER_GAMEPLAY §4](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| FR-MP-002 | Gán instance ≤ 40 theo trường + grade band, ưu tiên bạn bè, tránh người đã chặn, Tham gia bạn | [MULTIPLAYER_GAMEPLAY §6](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [ADR-0012](../../docs/adr/ADR-0012-the-gioi-game-hybrid-hub-instance-private-instance-match-roo.md) |
| FR-MP-003 | Presence tạm (online/away/offline, area, activity, ẩn); hiển thị cho bạn bè ở F095 | [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| FR-MP-004 | Input ≤ 10 Hz, snapshot 10 Hz theo interest grid, kiểm tốc độ/biên | [REALTIME_ARCHITECTURE §6](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) · [ADR-0011](../../docs/adr/ADR-0011-realtime-transport-websocket-qua-go-realtime-server.md) |
| FR-MP-006 | Không REST cho chuyển động; Valkey không làm game simulation | [REALTIME_ARCHITECTURE §1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) |
| NFR-PERF-008 | Tick 10 Hz, băng thông theo ngân sách | [PERFORMANCE_BUDGET §10](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |

### Key Entities

- **HubInstance (Room)**: bản sao một khu chung, thuộc một partition (tenant, trường, nhóm khối), có sức chứa, thành viên, chủ sở hữu (instance room host), trạng thái mở/đóng.
- **HubMember**: học sinh trong instance với `eid`, vị trí/hướng/animation hiện tại, đếm vi phạm, hạn giữ chỗ khi mất kết nối.
- **SessionMembership**: bản ghi vào/ra room để điều tra.
- **WorldAreaRuntime**: dữ liệu runtime của khu (sức chứa, tốc độ tối đa, biên, điểm xuất hiện, nhóm khối được vào, điều kiện mở khóa) sao chép từ module game.
- **Presence**: trạng thái tạm của học sinh (status, activity, area, room, ẩn).
- **Quan hệ tham chiếu**: bạn bè, chặn, hồ sơ công khai (nickname, avatar) sao chép từ module sở hữu.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `hub_instance_capacity` | Sức chứa tối đa một instance (giá trị area có thể nhỏ hơn) |
| `hub_instance_idle_close` | Đóng instance rỗng |
| `disconnect_grace` | Giữ chỗ hub khi mất kết nối |
| `hub_sync_rate` | Tần suất tick/snapshot |
| `client_input_rate` | Trần tần suất `hub.player.move` |
| `emote_rate` | Giới hạn emote |
| `hub_sync_bandwidth` | Ngân sách băng thông kiểm trong load test |
| `presence_ttl` | TTL presence |
| `ws_timeout` | Chuyển `offline` khi không kết nối lại |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Room host trong module competition, lease Valkey, input qua NATS core `ew.in.room.*` | ADR-0074 · [REALTIME_ARCHITECTURE §5.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) |
| Không đọc DB/Valkey key của module khác; dữ liệu area/bạn bè/chặn/hồ sơ qua projection từ event | AGENTS R13, R29 · ADR-0037, ADR-0042, ADR-0074 |
| Không PII trong snapshot; chỉ nickname + avatar | AGENTS R16 · ADR-0054 |
| Bảng mới có `tenant_id` + soft-delete/append-only, RLS | AGENTS R8, R21 · ADR-0005, ADR-0028 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 trường hợp hai học sinh khác trường hoặc khác nhóm khối cùng một hub instance trong mô phỏng 10.000 lượt vào khu.
- **SC-002**: ≥ 95% lượt vào khu của học sinh có bạn bè online cùng partition (còn chỗ) được xếp cùng ít nhất một bạn.
- **SC-003**: Instance 40 người: tick p95 trong ngân sách, băng thông xuống ≤ `hub_sync_bandwidth` trong 10 phút đo.
- **SC-004**: Mất mạng 10 s ở hub → thấy lại người chơi khác trong ≤ 2 giây sau khi kết nối lại (AC-MP-004).
- **SC-005**: 100% vị trí vượt tốc độ tối đa bị hiệu chỉnh trong cùng tick.

## Ngoài phạm vi

- Client Godot nội suy, remote player → F092.
- Hiển thị presence cho bạn bè, ẩn trạng thái với bạn bè, sự kiện `friend.*`/`user.blocked` → F095 (F091 chỉ tiêu thụ).
- Party ưu tiên cùng instance, kéo party khi chuyển khu, private instance → F100/F101; lobby/battle/co-op room → F104/F105; boss room → F103.
- Kiểm tra khoảng cách cho tương tác có giá trị (mở quest, nhận item) và dữ liệu area/mở khóa gốc → F071.
- Gộp grade band theo cấu hình trường → chưa có tham số, mặc định không gộp [Proposed].

## Assumptions

- F071 (module game) phát `content.published` (`item_type = world_area`) và `world.area_unlocked`; trước khi F071 Done, quickstart dùng seed `main_hub` phát qua cùng event (không mock trong đường chạy thật).
- F070 phát `game_profile.updated` có `avatar_config`; F095 phát `friend.added/removed`, `user.blocked/unblocked` (trước đó projection rỗng: không có ưu tiên bạn bè).
- Bật ghi nhận hub membership theo tenant chưa có tên trong CONFIG_DEFAULTS; đề xuất `audit_hub_membership` (task Polish) [Proposed].
- Ánh xạ `grade_level` → `grade_band` (`g1_3`, `g4_5`, `g6_7`, `g8_9`) theo [GLOSSARY](../../docs/00-overview/GLOSSARY.md) (mục Grade band).
