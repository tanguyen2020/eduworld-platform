# Feature Specification: F092 — `RealtimeClient` Godot và Main Hub nhiều người chơi

**Feature Branch**: `092-realtime-client-hub-multiplayer` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/game/` · **Mốc**: M2 · **Phụ thuộc**: F091 (cùng F061, F062, F070; F090 qua F091)
**Input**: Game client mở một kết nối realtime sau khi tạo game session, tự kết nối lại và resume khi rớt mạng, xử lý đăng xuất do thiết bị khác; trong Main Hub thấy các bạn cùng instance di chuyển mượt (nội suy), đúng avatar và emote, chạy đạt FPS trên máy tầm trung và trong ngân sách băng thông. FR nguồn: FR-MP-004, FR-MP-005, FR-GAME-004; NFR-PERF-008.

## Clarifications

### Session 2026-11-16

- Q: Heartbeat REST của F061 có còn chạy khi có WebSocket? → A: Không. Khi `RealtimeClient` ở trạng thái `Online`, heartbeat đi qua WS (gateway chuyển cho module game, F090-AC-12); quay về heartbeat REST (F061-AC-24) chỉ khi WS chưa kết nối được.
- Q: Hộp thoại "đã đăng xuất vì thiết bị khác" do ai làm? → A: Scene và luồng `ReplacedNotice` đã có ở F061; F092 kích hoạt signal `session_replaced` của `GameSessionService` (F061) khi nhận `session.session.replaced`/close `4409`.
- Q: Avatar người khác dựng từ đâu? → A: `PublicPlayer.avatar_config` trong `hub.instance.assigned`/`hub.player.joined` và `hub.player.appearance_changed`; dựng bằng `AvatarBuilder` của F070 (không gọi REST hồ sơ người khác).
- Q: Client có tự quyết định vị trí hợp lệ? → A: Không; client di chuyển cục bộ ngay, server hiệu chỉnh qua `hub.player.corrected` (ADR-0011, không prediction/rollback).

### Session 2026-11-17

- Q: Close code tắt node và enum presence sau đối soát? → A: Tắt node là `system.server.draining` + `1012` (RB-401, [GAME_NETWORKING §4.4](../../docs/08-game-client/GAME_NETWORKING.md)), không còn `4500`; presence theo [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) (RB-404).

### Session 2026-11-20 (owner chốt — certificate pinning)

- Certificate pinning cho `wss` → **Chốt:** `RealtimeClient` dùng cùng CA bundle với `ApiClient` (`TLSOptions.client(trusted_chain)`, CA chính + dự phòng do F133 đóng gói) trên Android/iOS ở build release (ADR-0101); Web dùng TLS trình duyệt.

### Session 2026-11-21 (quyết định khi triển khai, [Confirmed] chờ owner xác nhận)

- Autoload `RealtimeClient` (#7, sau `ApiClient`) có `class_name GameRealtimeClient` (tên class không được trùng tên autoload, như `GameAuthService`/`RestApiClient`). Service `HubSync`, `PresenceService` do `ServiceLocator` tạo với client đó. [Confirmed]
- Fixture realtime sinh bằng bộ sinh có sẵn `src/game/tools/gen_fixtures.mjs` (đã có `--check` trong gate Godot) vào `tests/fixtures/contract/asyncapi/<type>.<name>.json`, mỗi example kiểm bằng ajv theo payload schema của message — thay `scripts/godot/gen-realtime-fixtures.mjs` + `fixtures/realtime/` của plan. [Confirmed]
- Sau resume thành công client giữ resume token cũ (gateway tiếp tục kết nối logic cũ, F090-AC-14); resume thất bại → token và số `seq` của kết nối mới. Lệnh WS được phép trong `Resync` (để join lại khu), UI chỉ ghi khi `Online`. [Confirmed]
- `4409` không kèm `session.session.replaced` → không hiện gì và kết nối lại im lặng (backoff, không banner). `4403`: `account_locked` → màn AccountBlocked (`ui.error.ACCOUNT_LOCKED` của F061, không thêm khóa `ui.realtime.account_locked`); lý do khác/không có message → xóa phiên, `EventBus.session_expired`, form đăng nhập. `4410` (`curfew`, `daily_limit`) → CurfewNotice giữ refresh token. Ticket `404`/`409` → như heartbeat 409/404 (ReplacedNotice); `403 CURFEW_ACTIVE` → CurfewNotice. [Confirmed]
- Im lặng 2 × `heartbeat_s`: client tự đóng (close 1000) rồi backoff + resume. Mobile ở nền ≥ 5 phút: khi quay lại đóng socket, xin ticket mới và resume (không chạy timer khi app bị treo nền). [Confirmed]
- Presence: hoạt động lấy từ màn UI mà WorldRoot chuyển tiếp (`activity_host` → `in_activity`/`learning`) thay `SceneRouter`; `in_match` do feature trận/boss gọi `set_activity`; input lấy ở `WorldRoot._input`. Sau mỗi kết nối mới (gateway đặt `online`) client gửi lại trạng thái hiện tại nếu khác. [Confirmed]
- LOD: N người gần nhất (`quality_presets`: 10 / 20 (PC 30) / 40, thêm `full_characters`/`full_characters_pc` vào `QualityPolicy.PRESETS`) vẽ đầy đủ, còn lại impostor (capsule placeholder có nhãn, OQ-16); > 30 m animation cập nhật 1/4 khung. Kiểm CI headless: script hub ≤ 6 ms/khung (PERFORMANCE_BUDGET §2 "game logic + script"), băng thông lên ≤ `hub_sync_bandwidth`; FPS, băng thông xuống và soak 60 phút đo trên lab. [Confirmed]
- CA bundle cho `wss` (OQ pinning): `res://config/tls/ca_bundle.crt` (F133 đóng gói) → `TLSOptions.client(chain)`; build chưa có bundle dùng CA của engine. [Confirmed]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Kết nối realtime ổn định (Priority: P1)

Học sinh vào game; kết nối realtime tự mở; mạng phòng máy chập chờn thì hiện banner "Đang kết nối lại…" rồi tự hồi phục, không phải đăng nhập lại.

**Why this priority**: FR-MP-005; mọi tính năng realtime sau (party, chat, thông báo) dùng chung `RealtimeClient`.

**Independent Test**: GdUnit4 với `FakeWebSocketPeer`: welcome → mất kết nối → backoff → ticket mới → `system.session.resume` → `system.session.resumed`; mỗi close code dẫn tới đúng trạng thái.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F092-AC-01 | KHI có game session, game client PHẢI gọi `createRealtimeTicket` (kèm `X-Tenant-Id`), mở `url` của ticket qua `wss` với subprotocol `eduworld.v1`, chờ `system.session.welcome` rồi lưu `resume_token`, `heartbeat_s`, `server_time` và chuyển trạng thái `Online` | FR-MP-005 · [GAME_NETWORKING §4.1](../../docs/08-game-client/GAME_NETWORKING.md) | — |
| F092-AC-02 | NẾU `createRealtimeTicket` trả `403 GAME_ACCESS_STUDENT_ONLY` hoặc `403 TENANT_MISMATCH`, THÌ game client PHẢI dừng kết nối và đi theo luồng lỗi của F061 (màn hướng dẫn/đăng nhập lại), không thử lại | AGENTS R23 · ADR-0065 | TC-AUTH-013, TC-TEN-011 |
| F092-AC-03 | TRONG KHI `Online`, game client PHẢI gửi `system.heartbeat.ping` mỗi `heartbeat_s`; NẾU không nhận frame nào trong 2 × `heartbeat_s`, THÌ PHẢI đóng và kết nối lại | FR-MP-005 · [GAME_NETWORKING §4.3](../../docs/08-game-client/GAME_NETWORKING.md) | — |
| F092-AC-04 | KHI kết nối đóng, game client PHẢI xử lý theo close code: `4401` → xin ticket mới; `4408`/đóng bất thường → backoff `client_reconnect_backoff` rồi resume; `1012` sau `system.server.draining` → chờ `reconnect_after_ms` rồi resume ở node khác, không hiện lỗi; `4429` → chờ ≥ 30 giây; `4403` → dừng, phát `EventBus.session_expired` với lý do khóa tài khoản; `4409` sau `session.session.replaced` → dừng, không reconnect, xóa token, kích hoạt `GameSessionService.session_replaced`; `4409` không kèm message → im lặng | FR-MP-005 · FR-AUTH-023 · [GAME_NETWORKING §4.4](../../docs/08-game-client/GAME_NETWORKING.md) | TC-AUTH-015, TC-AUTH-006 |
| F092-AC-05 | KHI kết nối lại có `resume_token` cũ, game client PHẢI gửi `system.session.resume` (`resume_token`, `last_seq`), bỏ qua message có `seq` đã xử lý; NẾU nhận `system.session.resume_failed`, THÌ PHẢI resync qua REST (`getGameProfile`) và gửi lại `hub.instance.join` cho khu hiện tại | FR-MP-005 · [GAME_NETWORKING §4.4](../../docs/08-game-client/GAME_NETWORKING.md) | TC-AUTH-047 |
| F092-AC-06 | game client PHẢI gửi lệnh qua `send_command(type, payload)` với `id` UUIDv7 và hoàn tất khi nhận `system.command.acked`/`system.command.failed` có `ref` khớp hoặc hết `client_command_timeout`; `hub.player.move` gửi không chờ ack; message có `type` lạ PHẢI bị bỏ qua, payload sai kiểu PHẢI bị loại trước khi tới service | [GAME_NETWORKING §4.2, §9](../../docs/08-game-client/GAME_NETWORKING.md) | — |
| F092-AC-07 | game client PHẢI không ghi ticket, token, resume token vào log/telemetry, không gửi access token qua WebSocket và chỉ dùng `wss` ở build release | ADR-0008 · ADR-0064 · AGENTS R14 | TC-AUTH-010 |
| F092-AC-08 | TRONG KHI đang backoff/kết nối lại, game client PHẢI hiện banner "Đang kết nối lại…" (khóa dịch), khóa lệnh ghi qua WS và đóng băng avatar người khác ở vị trí cuối; KHI nhận `system.service.degraded`, PHẢI hiện chỉ báo suy giảm | FR-MP-005 · [GODOT_ARCHITECTURE §9](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | — |
| F092-AC-09 | TRONG KHI `Online`, game client PHẢI ngừng heartbeat REST của F061 và gửi `presence.status.set` theo enum presence chung khi hoạt động đổi (`online` ở hub/menu, `in_activity` khi làm bài/quest/mini-game, `in_match` khi thi đấu/đánh boss) hoặc không có input quá 5 phút (`away`) | FR-MP-005 · [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |

---

### User Story 2 — Main Hub có người chơi khác (Priority: P1)

Học sinh vào Quảng trường Tia Sáng thấy các bạn cùng instance đi lại, đổi trang phục, vẫy tay; bị kéo lại nhẹ nhàng nếu vị trí bị server hiệu chỉnh.

**Why this priority**: "Thế giới có người" là trải nghiệm cốt lõi của hub (MULTIPLAYER_GAMEPLAY §2).

**Independent Test**: Hai client headless `hs.an.3a`, `hs.binh.3a` với backend compose: cả hai cùng `room_id`, A di chuyển → B thấy `RemotePlayer` của A di chuyển mượt (sai số vị trí trong ngưỡng khi mất 20% snapshot, jitter 80 ms).

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F092-AC-10 | KHI vào một khu, game client PHẢI gửi `hub.instance.join` (`area_id`, `spawn_point` nếu có) và KHI nhận `hub.instance.assigned`, PHẢI đặt nhân vật tại `spawn`, ghi `self_eid`, tạo `RemotePlayer` cho mỗi phần tử `players[]` | FR-MP-004 · [GAME_NETWORKING §5](../../docs/08-game-client/GAME_NETWORKING.md) | — |
| F092-AC-11 | game client PHẢI gửi `hub.player.move` (`room_id`, `client_seq`, `p` lượng tử 1 cm, `r`, `a`) tối đa `client_input_rate` chỉ khi Δvị trí ≥ 2 cm hoặc Δhướng ≥ 2° hoặc animation đổi, và 1 Hz khi đứng yên | FR-MP-004 · [GAME_NETWORKING §5.1](../../docs/08-game-client/GAME_NETWORKING.md) | TC-PERF-023 |
| F092-AC-12 | KHI nhận `hub.snapshot.updated`, game client PHẢI hiển thị người chơi khác tại `render_time = Clock.server_now() − client_interpolation` bằng nội suy giữa hai snapshot, tự tăng độ trễ khi jitter cao (không vượt trần), ngoại suy tối đa 250 ms khi thiếu snapshot rồi đứng yên | FR-MP-004 · NFR-PERF-008 · [GAME_NETWORKING §5.2](../../docs/08-game-client/GAME_NETWORKING.md) | — |
| F092-AC-13 | KHI nhận `hub.player.joined`/`hub.player.left` hoặc `removed[]`, game client PHẢI lấy/trả `RemotePlayer` từ pool (tối đa `hub_instance_capacity`) và fade-out 0,3 giây khi rời | FR-MP-004 | — |
| F092-AC-14 | KHI nhận `hub.player.corrected` cho chính mình, game client PHẢI đưa nhân vật về vị trí server trong 200 ms bằng nội suy, không dịch chuyển giật | FR-MP-004 · ADR-0011 | — |
| F092-AC-15 | game client PHẢI dựng `RemotePlayer` từ `PublicPlayer.avatar_config` (thân, tóc, trang phục, phụ kiện) bằng `AvatarBuilder`, hiển thị nickname và cập nhật khi nhận `hub.player.appearance_changed`; PHẢI không hiển thị tên thật, trường, lớp của người khác | FR-GAME-004 · AGENTS R16 | — |
| F092-AC-16 | KHI học sinh chọn emote đã sở hữu, game client PHẢI gửi `hub.player.emote` (giới hạn `emote_rate` phía client) và phát animation emote cho mọi `hub.player.emoted` nhận được; lỗi `PERMISSION_DENIED`/`RATE_LIMITED` PHẢI hiện thông báo thân thiện | FR-GAME-004 · [CHARACTER_SYSTEM](../../docs/01-game-design/CHARACTER_SYSTEM.md) | — |
| F092-AC-17 | KHI nhận `hub.instance.migrated`, game client PHẢI chuyển sang `room_id` mới mà không tải lại scene; KHI join thất bại (`ROOM_FULL`, `PERMISSION_DENIED`, `NOT_FOUND`), PHẢI ở lại khu hiện tại và hiện thông báo theo `code` | [REALTIME_ARCHITECTURE §5.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F092-AC-18 | Mọi chuỗi UI mới (banner mạng, lỗi hub, emote) PHẢI qua khóa dịch `ui.realtime.*`/`ui.hub.*` trong `vi.po`/`en.po` và style lấy từ Theme sinh từ token | NFR-I18N-001 · ADR-0047 | TC-I18N-005, TC-UI-003 |
| F092-AC-19 | game client PHẢI chỉ hiển thị người chơi do server gửi trong instance của mình; học sinh tenant khác không bao giờ xuất hiện (kiểm end-to-end) | NFR-SEC-006 · ADR-0054 | TC-TEN-012 |

---

### User Story 3 — Hub đông người chạy mượt trên máy tầm trung (Priority: P2)

Phòng máy của trường dùng PC iGPU và điện thoại 4 GB RAM: hub 40 người vẫn đạt FPS mục tiêu, không ngốn băng thông.

**Why this priority**: NFR-COMP-001 là yêu cầu bắt buộc; NFR-PERF-008.

**Independent Test**: 39 bot + 1 client thật trên DEV-AND-MID1 và DEV-WEB-MID (PC chạy bản Web — ADR-0101), preset Trung bình, 10 phút: đo FPS p10 và băng thông.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F092-AC-20 | TRONG KHI ở Hub 40 người trên thiết bị hạng Tầm trung (preset Trung bình), game client PHẢI đạt FPS theo AC-PERF-001 (Android), AC-PERF-002 (PC), AC-PERF-003 (Web) | NFR-COMP-001 · [PERFORMANCE_BUDGET §16](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-009 |
| F092-AC-21 | game client PHẢI giới hạn số nhân vật hiển thị đầy đủ theo `quality_presets`, dùng LOD/impostor cho phần còn lại và giảm tần suất cập nhật animation của người chơi ngoài 30 m | NFR-COMP-001 · [PERFORMANCE_BUDGET §5, §13](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-018 |
| F092-AC-22 | Băng thông của client ở Hub 40 người PHẢI ≤ `hub_sync_bandwidth` (xuống và lên) trong 10 phút đo | NFR-PERF-008 · AC-PERF-009 | TC-PERF-023 |
| F092-AC-23 | Sau 60 phút ở Hub có người vào/ra liên tục, bộ nhớ tiến trình PHẢI không tăng quá ngưỡng rò rỉ của AC-PERF-006 (pool `RemotePlayer` được tái sử dụng) | NFR-COMP-001 · AC-PERF-006 | TC-PERF-021 |

### Edge Cases

- NẾU token hết hạn khi WS đang mở, THÌ WS không bị cắt; chỉ refresh khi xin ticket mới (TC-AUTH-002).
- NẾU ứng dụng mobile vào nền > 5 phút, THÌ đóng WS (best effort) theo GODOT_ARCHITECTURE §5; quay lại trong 30 phút → ticket mới + resume.
- NẾU nhận snapshot của `room_id` khác room hiện tại (trễ sau migrate), THÌ bỏ qua.
- NẾU `seq` nhảy (gap) mà không resume được, THÌ resync REST và join lại khu (F092-AC-05).
- NẾU ngôn ngữ đổi khi đang ở Hub, THÌ banner/thông báo đổi ngay (TC-I18N-004).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-MP-004 | Gửi input ≤ 10 Hz có ngưỡng, nội suy snapshot, hiệu chỉnh từ server | [GAME_NETWORKING §5](../../docs/08-game-client/GAME_NETWORKING.md) |
| FR-MP-005 | `RealtimeClient`: kết nối, heartbeat, reconnect theo close code, resume, takeover | [GAME_NETWORKING §4](../../docs/08-game-client/GAME_NETWORKING.md) |
| FR-GAME-004 | Hiển thị avatar (thân, tóc, trang phục, phụ kiện) và emote của người chơi khác | [CHARACTER_SYSTEM](../../docs/01-game-design/CHARACTER_SYSTEM.md) |
| NFR-PERF-008 | Ngân sách băng thông hub phía client; tần suất đồng bộ | [PERFORMANCE_BUDGET §10](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |

### Key Entities

- **RealtimeConnection**: trạng thái kết nối (Disconnected, FetchingTicket, Connecting, Welcomed, Resuming, Resync, Online, Backoff, Stopped), `resume_token`, `last_seq`.
- **PendingCommand**: lệnh đang chờ ack (id, type, hạn).
- **HubRoomState**: `room_id`, `area_id`, `self_eid`, danh sách người chơi khác.
- **RemotePlayerView**: hiển thị một người chơi khác (eid, nickname, avatar, buffer snapshot).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `ws_heartbeat` | Mặc định `heartbeat_s` khi welcome không gửi |
| `client_reconnect_backoff` | Backoff kết nối lại |
| `client_command_timeout` / `client_input_rate` | Timeout chờ ack; trần tần suất gửi di chuyển |
| `client_interpolation` | Độ trễ nội suy và trần |
| `hub_instance_capacity` | Kích thước pool `RemotePlayer` |
| `emote_rate` | Giới hạn emote phía client |
| `quality_presets` | Số nhân vật hiển thị đầy đủ theo preset |
| `hub_sync_bandwidth` | Ngưỡng kiểm băng thông |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Luồng UI → Service → `AppState` → signal; không gọi mạng từ scene | AGENTS R1 · [GODOT_ARCHITECTURE §2, §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |
| Access token, ticket chỉ trong RAM (refresh token theo F061); không log token/ticket | ADR-0086 · AGENTS R14 |
| Không hardcode màu/font/chuỗi | AGENTS R32 · NFR-I18N-001 |
| Static typing GDScript, signal thì quá khứ | AGENTS §7 · ADR-0014 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Mất mạng ≤ 2 phút: 100% lần thử client tự hồi phục không hỏi mật khẩu (TC-AUTH-047).
- **SC-002**: Thiết bị cũ về màn đăng nhập trong ≤ 5 giây sau takeover, không tự kết nối lại (TC-AUTH-015).
- **SC-003**: Người chơi khác thấy chuyển động mượt: sai số vị trí hiển thị trung bình ≤ 0,3 m khi mất 20% snapshot, jitter 80 ms.
- **SC-004**: Hub 40 người đạt AC-PERF-001/002/003 và AC-PERF-009 trên lab thiết bị.

## Ngoài phạm vi

- Server gateway, hub instance, presence → F090, F091.
- Hộp thoại takeover/`ReplacedNotice`, heartbeat REST, `ApiClient` → F061; tạo avatar/trang phục, `AvatarBuilder` → F070.
- Danh sách bạn bè, "Tham gia bạn" từ UI, hiển thị presence bạn bè → F095 (backend); UI bạn bè trong Godot chưa có feature trong danh mục (đề xuất gắn F102).
- Party/lobby, chat, thông báo trong game → F102, F119, F123; battle client → F109.

## Assumptions

- F061 cung cấp `ApiClient`, `AuthService`, `Clock` (offset theo `server_time`), `EventBus.session_expired`, luồng `ReplacedNotice`; F062 cung cấp zone `main_hub`, `PlayerController`, `QualityManager`; F070 cung cấp `AvatarBuilder` và `ItemVisual`.
- Ngưỡng 2 cm/2°, ngoại suy 250 ms, pha blend 200 ms, fade 0,3 s, LOD > 30 m là quy tắc ở GAME_NETWORKING §5 (chưa có tên tham số) — giữ link nguồn.
- Trước khi F091 Done, client chạy với AsyncAPI fixture (gắn nhãn MOCK, R17); feature chỉ `Done` khi chạy với backend thật.
