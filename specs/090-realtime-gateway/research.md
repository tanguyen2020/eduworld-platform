# Research: F090 — realtime: ticket, WebSocket, heartbeat/reconnect, định tuyến lệnh, cắt kết nối khi thu hồi

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Schema message trên Valkey Pub/Sub và message điều khiển

- **Decision**: Mọi publish lên `ew:rt:*`/`ew:chat:fanout:*` dùng một envelope JSON `RtPubSubEnvelope` (JSON Schema mới `specs/schemas/realtime/rt-pubsub-envelope.schema.json`): `{ v: 1, kind: "message" | "control", target?: { user_ids?: uuid[≤ 500], game_session_id?: uuid, exclude_conn_id?: string≤64 }, droppable: bool, message?: <ServerEnvelope không có seq>, control?: { action: "close", code: 4403|4409, reason: "access_revoked"|"replaced"|"transferred" } }`. Gateway gán `seq` khi giao. `session.session.replaced` gửi dạng `kind=message` kèm `control` close `4409` và `target.game_session_id`.
- **Rationale**: REALTIME_ARCHITECTURE chỉ nói module `PUBLISH` message; cần một hợp đồng chung để gateway biết droppable, lọc theo game session (F090-AC-26) và đóng kết nối. Gộp snapshot theo node (F091) dùng `target.user_ids`.
- **Alternatives**: Kênh riêng cho điều khiển (`ew:rt:control:<user>`) — thêm subscribe mỗi kết nối, bị loại; dùng NATS cho điều khiển — gateway không là consumer JetStream (EVENT_DESIGN §3.2), bị loại.

## R2. Thư viện WebSocket

- **Decision**: `github.com/coder/websocket` (context-aware, hỗ trợ subprotocol, close code tùy ý, nén tắt mặc định) [Confirmed].
- **Rationale**: API theo `context.Context` khớp quy ước Go của repo (AGENTS §7); dễ đặt write deadline 10 s.
- **Alternatives**: `gorilla/websocket` (ổn định nhưng API cũ, cần goroutine ghi riêng); `gobwas/ws` (hiệu năng cao, code phức tạp) — xem lại nếu load test F132 cho thấy cần.

## R3. Buffer resume và `seq`

- **Decision**: `seq` tăng trong bộ nhớ `Session`, đồng thời ghi message không droppable vào `ew:gateway:rt:resume:<token>` (LIST, `RPUSH` + `LTRIM` giữ `resume_window` message, TTL theo `resume_window`); kèm `HSET ew:gateway:rt:resume:<token>:meta {user_id, game_session_id, last_seq, subs}`. Resume trên node khác đọc meta, kiểm `user_id` + `game_session_id` khớp ticket mới, phát lại phần `seq > last_seq`, tiếp tục `seq` từ `last_seq` meta. ID lệnh đã xử lý lưu `ew:gateway:rt:cmd:<user_id>:<id>` (kết quả ack/failed, TTL `resume_window`).
- **Rationale**: Resume phải chạy trên node bất kỳ (không sticky, NFR-SCAL-003); `seq` liên tục giúp client phát hiện gap.
- **Alternatives**: Giữ buffer chỉ trong RAM node — mất khi node chết/deploy (trái F090-AC-27), bị loại.

## R4. Audit của gateway khi không có database

- **Decision**: Gateway publish `audit.recorded` trực tiếp lên JetStream subject `ew.evt.realtime.audit.recorded` (stream mới `EVT_REALTIME`, 7 ngày), `Nats-Msg-Id = event_id`; module identity mở rộng consumer audit sang `EVT_REALTIME`.
- **Rationale**: [REALTIME_ARCHITECTURE §8.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) yêu cầu ghi audit khi đóng `4429`; không có outbox vì không có database (ADR-0074). Mất một bản audit khi JetStream lỗi được chấp nhận (sự kiện vi phạm rate limit, không phải thao tác nghiệp vụ).
- **Alternatives**: Gọi module identity HTTP — trái R29 (phản ứng phụ dùng event), bị loại.
