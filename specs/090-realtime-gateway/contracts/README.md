# Contracts: F090 — realtime: ticket, WebSocket, heartbeat/reconnect, định tuyến lệnh, cắt kết nối khi thu hồi

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createRealtimeTicket` | `POST /api/v1/game/sessions/{game_session_id}/realtime-tickets` → `201 RealtimeTicket` | `specs/openapi/modules/game.v1.yaml` (nguồn `eduworld-api.v1.yaml`) | `game_player` (học sinh, session của chính mình) | Cần sửa — thêm response `401`, `403` (`GAME_ACCESS_STUDENT_ONLY`, `TENANT_MISMATCH`), `409`, `503` |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Gateway không gọi facade hay HTTP của module khác (binary `realtime` không nạp module domain); lệnh đi NATS request/reply (`ew.cmd.*`) tới handler trong `core-api` (ADR-0074) | — | — | — |

Subject NATS (không phải OpenAPI, ghi ở [specs/events/README](../../../specs/events/README.md) §NATS core):

| Subject | Gọi từ → tới | Trạng thái |
|---|---|---|
| `ew.cmd.<module>.<type>` | gateway → module sở hữu (handler NATS trong `core-api`) (request/reply, reply `{ok, result?, subscribe?, unsubscribe?}` hoặc `{ok:false, problem:{code, retryable, retry_after_ms?}}`) | Có sẵn (khung) |
| `ew.cmd.game.session.heartbeat` | gateway → module game, payload `{game_session_id}` | Cần thêm (liệt kê trong AsyncAPI phần định tuyến nội bộ) |
| `ew.in.room.<room_id>` | gateway → room host (fire-and-forget) | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `system.heartbeat.ping` / `system.heartbeat.pong` | client ↔ server | Có sẵn |
| `system.session.welcome` | server → client | Có sẵn |
| `system.session.resume` / `system.session.resumed` / `system.session.resume_failed` | client ↔ server | Có sẵn |
| `system.command.acked` / `system.command.failed` | server → client | Có sẵn (`ErrorCode` có `UNKNOWN_TYPE`, `VALIDATION_FAILED`, `RATE_LIMITED`, `UNAVAILABLE`) |
| `system.service.degraded` | server → client | Có sẵn |
| `system.notice.broadcasted` | server → client (từ `ew:rt:broadcast`) | Có sẵn |
| `session.session.replaced` | server → thiết bị cũ, sau đó close `4409` | Có sẵn |
| `system.server.draining` (`reconnect_after_ms`) | server → client, trước khi đóng `1012` | Cần thêm |
| Close codes `1012/4400/4401/4403/4408/4409/4429` | server → client | Cần sửa (thay `4500` bằng `1012` sau `system.server.draining`) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `audit.recorded` | gateway → identity (AUD) qua `EVT_REALTIME` | `specs/events/payloads/common/audit.recorded.v1.schema.json` (schema chung, owner F003 T004) | Cần thêm (F003) |
| `user.access_revoked` | identity → (identity tự publish message điều khiển lên Valkey, F016) | `specs/events/payloads/identity/user.access_revoked.v1.schema.json` | Cần thêm (F016) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/realtime/rt-pubsub-envelope.schema.json` | Envelope message/điều khiển trên Valkey Pub/Sub ([research R1](../research.md)) | Cần thêm |

## 6. Delta cần thực hiện

- `eduworld-api.v1.yaml` → `createRealtimeTicket`: thêm `401`, `403` (`GAME_ACCESS_STUDENT_ONLY`, `TENANT_MISMATCH`), `409` (session đã kết thúc/bị thay), `503 UNAVAILABLE`; chạy `split-public` sinh lại `game.v1.yaml`.
- AsyncAPI `eduworld-realtime.v1.yaml`: sửa mô tả định tuyến (`hub.instance.*`, `hub.player.*` → module competition theo REALTIME_ARCHITECTURE §4.1; header NATS `Ew-*`); thêm `x-routing` cho mỗi operation receive (module đích) để sinh `routes_gen.go`; ghi subject `ew.cmd.game.session.heartbeat`; thêm message `system.server.draining` (`reconnect_after_ms` integer 0–120000, `reason` enum `deploy`/`maintenance` — cùng delta F130 T004) và đổi close code tắt node `4500` → `1012` trong mô tả channel (RB-401, REALTIME_ARCHITECTURE §3.1).
- Thêm `specs/schemas/realtime/rt-pubsub-envelope.schema.json` (mọi string có `maxLength`).
- Thêm stream `EVT_REALTIME`; cập nhật `specs/events/README.md`. Payload `audit.recorded` dùng schema chung `specs/events/payloads/common/audit.recorded.v1.schema.json` (F003), không tạo bản riêng cho realtime.
- Kiểm: `make specs-lint`, `make gen MOD=game`, `make test-contract`.
- ADR-0078: `createRealtimeTicket` khai báo `403 CURFEW_ACTIVE` (task T041).
