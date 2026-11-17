# Contracts: F092 — `RealtimeClient` Godot và Main Hub nhiều người chơi

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createRealtimeTicket` | `POST /api/v1/game/sessions/{game_session_id}/realtime-tickets` → `RealtimeTicket` (`ticket`, `expires_at`, `url`, `protocol_version`) | `specs/openapi/modules/game.v1.yaml` | `game_player` | Có sẵn (delta lỗi ở F090) |
| `getGameProfile` | `GET /api/v1/game/profile` (resync sau `resume_failed`) | `specs/openapi/modules/game.v1.yaml` | `game_player` | Có sẵn |
| `heartbeatGameSession` | `POST /api/v1/game/sessions/{id}:heartbeat` (chỉ khi WS chưa Online) | `specs/openapi/modules/game.v1.yaml` | `game_player` | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi facade nội bộ giữa module | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `system.heartbeat.ping` / `system.heartbeat.pong` | client ↔ server | Có sẵn |
| `system.session.welcome`, `system.session.resume`, `system.session.resumed`, `system.session.resume_failed` | client ↔ server | Có sẵn |
| `system.command.acked`, `system.command.failed`, `system.service.degraded`, `system.notice.broadcasted` | server → client | Có sẵn |
| `session.session.replaced` | server → client | Có sẵn |
| `presence.status.set` | client → server | Có sẵn (enum chung — delta F091) |
| `hub.instance.join` | client → server | Cần sửa (F091: `follow_user_id`) |
| `hub.player.move`, `hub.player.emote` | client → server | Có sẵn |
| `hub.instance.assigned`, `hub.snapshot.updated`, `hub.player.joined`, `hub.player.left`, `hub.player.emoted`, `hub.player.appearance_changed`, `hub.player.corrected`, `hub.instance.migrated` | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không dùng NATS (AGENTS R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- AsyncAPI: thêm `examples` cho `system.session.welcome`, `system.session.resumed`, `system.session.resume_failed`, `session.session.replaced`, `hub.instance.assigned`, `hub.snapshot.updated`, `hub.player.joined`, `hub.player.corrected`, `hub.instance.migrated` để sinh fixture `godot/tests/fixtures/realtime/` (CI kiểm fixture khớp schema).
- Phụ thuộc delta của F090 (lỗi `createRealtimeTicket`, message `system.server.draining` + close `1012` thay `4500`) và F091 (`follow_user_id`, enum presence theo MULTIPLAYER_GAMEPLAY §5, cờ `hidden`).
- Kiểm: `make specs-lint`, `make godot-test`.
