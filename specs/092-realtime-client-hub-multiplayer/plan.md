# Implementation Plan: F092 — `RealtimeClient` Godot và Main Hub nhiều người chơi

**Branch**: `092-realtime-client-hub-multiplayer` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Hiện thực autoload `RealtimeClient` (#7, [GODOT_ARCHITECTURE §4](../../docs/08-game-client/GODOT_ARCHITECTURE.md)) theo state machine [GAME_NETWORKING §4.4](../../docs/08-game-client/GAME_NETWORKING.md): xin ticket qua `ApiClient`, `WebSocketPeer` poll trong `_process`, envelope typed (`WsEnvelope`), `send_command` với bảng chờ ack, dispatcher theo tiền tố `type`, heartbeat + đồng hồ (`Clock` của F061), resume theo `last_seq`, xử lý close code. Thêm `HubSync` (service `RefCounted`) gửi `hub.player.move` có ngưỡng, nhận snapshot vào `SnapshotBuffer` + `Interpolator`, quản lý pool `RemotePlayer` dựng bằng `AvatarBuilder` (F070), emote, hiệu chỉnh vị trí. UI chỉ có banner mạng và thông báo lỗi hub.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim ở `src/game/.godot-version`), static typing (ADR-0014) |
| Primary Dependencies | GdUnit4; autoload F060 (`Config`, `Log`, `EventBus`, `AppState`, `SceneRouter`, `Settings`), F061 (`ApiClient`, `AuthService`, `Clock`, `GameSessionService`), F062 (`main_hub`, `PlayerController`, `QualityManager`), F070 (`AvatarBuilder`) |
| Storage | Không lưu dữ liệu nghiệp vụ; `resume_token` chỉ trong RAM |
| Testing | GdUnit4 unit (`FakeWebSocketPeer`, `FakeApiClient`), scene test, integration headless 2 client + backend compose, toxiproxy; lab thiết bị |
| Target Platform | Windows, Android, iOS, Web (ADR-0013) |
| Project Type | game client |
| Performance Goals | AC-PERF-001/002/003/006/009 ([PERFORMANCE_BUDGET §16](../../docs/08-game-client/PERFORMANCE_BUDGET.md)); `ui_cpu_budget` |
| Constraints | AGENTS R1, R14, R16, R17, R32 · ADR-0008, ADR-0011, ADR-0031, ADR-0047, ADR-0057, ADR-0064 |
| Scale/Scope | 1 kết nối, ≤ `hub_instance_capacity` người chơi khác |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Dùng contract F090/F091; thêm example AsyncAPI cho fixture ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public (`createRealtimeTicket`) và WS; tách Service ← Gameplay ← UI |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Ticket theo tenant phiên; integration kiểm không thấy người tenant khác |
| IV Server-authoritative | ✅ | Vị trí hợp lệ do server; client không tự quyết định hết giờ/kết quả |
| V Xác thực, phân quyền | ✅ | Xử lý `GAME_ACCESS_STUDENT_ONLY`, `TENANT_MISMATCH`, `4403`, `4409` |
| VI An toàn trẻ em | ✅ | Chỉ nickname + avatar; không log token/ticket |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số từ welcome/client-config/CONFIG_DEFAULTS, không hardcode URL |
| VIII UI tách logic, token | ✅ | Banner/thông báo chỉ đọc state; Theme sinh từ token; chuỗi `ui.realtime.*` |
| IX Test có bằng chứng | ✅ | Test mọi F092-AC-*, integration 2 client, lab FPS |

## Project Structure

### Documentation (feature này)

```text
specs/092-realtime-client-hub-multiplayer/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/game/
├── scripts/networking/realtime_client.gd     # autoload RealtimeClient: state machine, send_command, dispatcher, resume
├── scripts/networking/ws_envelope.gd         # parse/serialize + validate (maxLength, frame ≤ ws_max_frame)
├── scripts/networking/ws_peer.gd             # interface bọc WebSocketPeer (để thay FakeWebSocketPeer)
├── scripts/networking/backoff.gd             # tái dùng của F061 (client_reconnect_backoff)
├── scripts/multiplayer/hub_sync.gd           # HubSync: join, move throttle, snapshot → buffer, emote, migrate
├── scripts/multiplayer/presence_service.gd   # presence.status.set theo hoạt động/idle
├── scripts/world/snapshot_buffer.gd          # buffer theo eid (giữ 1 s)
├── scripts/world/interpolator.gd             # render_time, lerp/slerp, ngoại suy 250 ms, jitter adapt
├── scripts/world/remote_player_sync.gd       # pool RemotePlayer, LOD theo quality preset
├── scenes/character/RemotePlayer.tscn        # AvatarRig + NicknameLabel (Theme)
├── scenes/ui/hud/NetworkBanner.tscn (+ .gd)  # "Đang kết nối lại…", chỉ báo suy giảm
├── locale/{vi,en}.po                         # ui.realtime.*, ui.hub.*
└── tests/{unit,scene,integration}/ , tests/fixtures/realtime/*.json
```

**Structure Decision**: Đúng cây GODOT_ARCHITECTURE §3 (`scripts/networking`, `scripts/multiplayer`, `scripts/world`, `scenes/character`); `ws_peer.gd` thêm để test bằng fake (G-5, §12).

## Phase 0: Research

Không có — GAME_NETWORKING §4–§6 và REALTIME_ARCHITECTURE §3, §6, §9 đã mô tả đủ thuật toán và tham số.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — (client không có domain nghiệp vụ) | |
| application | `RealtimeClient` (signal `connection_state_changed(state)`, `message_received(type, payload)`), `HubSync` (signal `room_assigned`, `remote_player_joined`, `remote_player_left`, `position_corrected`), `PresenceService` | Service `RefCounted` tạo bởi `ServiceLocator`, nhận `ApiClient`/`RealtimeClient` qua constructor |
| infrastructure | `WsPeer` (bọc `WebSocketPeer`), `WsEnvelope`, `Backoff`, `Clock` | |
| presentation | `RemotePlayer.tscn`, `NetworkBanner.tscn`, toast lỗi hub | Chỉ đọc `AppState`/signal |

### Tích hợp và phụ thuộc

- API: `createRealtimeTicket` (F090). WS: message `system.*`, `session.session.replaced`, `hub.*`, `presence.status.set` (F090, F091).
- `GameSessionService.session_replaced` (F061) cho luồng takeover; `EventBus.session_expired` cho `4403`.

### Bảo mật, tenant, idempotency, audit

- Ticket/access token chỉ trong RAM (refresh token do F061 quản lý qua `SecureStorage`, ADR-0086); `ApiClient` tắt debug log cho ticket; `Log` lọc khóa `ticket`, `resume_token`.
- `wss` bắt buộc ở build release (kiểm `Config.realtime_url` scheme); `X-Tenant-Id` gắn bởi `ApiClient`.
- Lệnh WS dùng `id` UUIDv7 duy nhất (gateway chống lặp, F090-AC-16).

### Quan sát

- Telemetry mạng gộp ≤ 1 lần/5 phút qua `POST /game/telemetry` (F080): RTT p50/p95, số reconnect, resume ok/fail — không PII ([GAME_NETWORKING §10](../../docs/08-game-client/GAME_NETWORKING.md)); trong F092 chỉ thu số liệu vào `AppState.net_stats`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
