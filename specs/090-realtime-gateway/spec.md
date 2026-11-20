# Feature Specification: F090 — realtime: ticket, WebSocket, heartbeat/reconnect, định tuyến lệnh, cắt kết nối khi thu hồi

**Feature Branch**: `090-realtime-gateway` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `realtime` (+ module `game`: cấp ticket, heartbeat qua NATS) · **Mốc**: M2 · **Phụ thuộc**: F003, F013 (F014 cho game session)
**Input**: Một kết nối WebSocket `/realtime/v1` duy nhất cho mỗi học sinh, mở bằng ticket dùng một lần; heartbeat, timeout, reconnect + resume không mất message quan trọng; gateway không có logic nghiệp vụ, chỉ chuyển lệnh tới module sở hữu qua NATS và giao sự kiện từ Valkey Pub/Sub; cắt kết nối khi tài khoản bị thu hồi hoặc bị thay bởi thiết bị khác; scale ngang, tắt êm. FR nguồn: FR-AUTH-010, FR-AUTH-011, FR-AUTH-019, FR-MP-005; NFR-AVAIL-004, NFR-SCAL-003.

## Clarifications

### Session 2026-11-16

- Q: Ai cấp realtime ticket? → A: module game `createRealtimeTicket` (F014 ghi "Ngoài phạm vi → F090"); gateway chỉ đọc + `GETDEL` ticket ([VALKEY_DESIGN §1](../../docs/07-backend/VALKEY_DESIGN.md)).
- Q: Close code khi thu hồi quyền: `4003` ([EVENT_DESIGN §4.1](../../docs/07-backend/EVENT_DESIGN.md)) hay `4403`? → A: `4403` — [REALTIME_ARCHITECTURE §3.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) là nguồn chuẩn close code; sửa EVENT_DESIGN (task Polish).
- Q: Tắt node dùng `4500` hay `1012` + `system.server.draining` ([BACKEND_DESIGN §13](../../docs/07-backend/BACKEND_DESIGN.md))? → A: Thay bởi Session 2026-11-17 (`system.server.draining` + `1012`).
- Q: Mỗi học sinh 1 kết nối (REALTIME_ARCHITECTURE §3, ADR-0031) hay `ws_connections_per_user` = 2 ([CONFIG_DEFAULTS](../../docs/00-overview/CONFIG_DEFAULTS.md))? → A: Một kết nối hiệu lực; kết nối mới thay kết nối cũ (đóng `4409`, không kèm message). Ghi chú CONFIG_DEFAULTS cần làm rõ (task Polish).
- Q: Ai đẩy lệnh đóng khi `user.access_revoked`? → A: module identity publish message điều khiển lên `ew:rt:user:<id>` (REALTIME_ARCHITECTURE §10, phần publish thuộc F016); gateway không là consumer JetStream ([EVENT_DESIGN §3.2](../../docs/07-backend/EVENT_DESIGN.md)).
- Q: Định tuyến `hub.*`: AsyncAPI ghi module game, REALTIME_ARCHITECTURE §4.1 ghi module competition? → A: Theo REALTIME_ARCHITECTURE §4.1 (tài liệu sở hữu); bảng định tuyến sinh từ AsyncAPI nên phải sửa phần mô tả định tuyến của AsyncAPI (contracts §6, quyết định ở F091 research R1).
- Q: Gateway ghi audit thế nào khi không có database/outbox? → A: Publish trực tiếp `audit.recorded` lên JetStream (stream mới `EVT_REALTIME`) — at-most-once, chấp nhận cho audit vi phạm rate limit [Confirmed]; contracts §6.

### Session 2026-11-17

- Q: Close code tắt node sau đối soát (RB-401)? → A: Gửi `system.server.draining` (`reconnect_after_ms`) rồi đóng `1012` theo [REALTIME_ARCHITECTURE §3.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md); không dùng `4500`. Thu hồi quyền đóng `4403`. AsyncAPI cần thêm message `system.server.draining` (delta contract).
- Q: Không cấp realtime ticket trong giờ giới nghiêm (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Cấp ticket và mở kết nối WebSocket (Priority: P1)

Sau khi tạo game session (F014), game xin ticket rồi mở WebSocket; nhận lời chào kèm thông số heartbeat và resume token.

**Why this priority**: Mọi tính năng realtime (hub, party, chat, thông báo) cần kết nối đã xác thực.

**Independent Test**: Client headless `hs.an.3a`: `createRealtimeTicket` → mở `wss /realtime/v1?ticket=` → nhận `system.session.welcome`; dùng lại ticket → close `4401`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F090-AC-01 | KHI học sinh có game session active của chính mình gọi `createRealtimeTicket` với `X-Tenant-Id` khớp tenant phiên, module game PHẢI sinh ticket ngẫu nhiên ≥ 256 bit, lưu băm SHA-256 kèm `user_id`, `tenant_id`, `school_id`, `game_session_id`, `device_id` với TTL `realtime_ticket_ttl` và trả `201` (`ticket`, `expires_at`, `url` từ cấu hình, `protocol_version`) | FR-AUTH-011 · ADR-0008 | — |
| F090-AC-02 | NẾU token không phải token game của học sinh, THÌ module game PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY`, không tạo ticket và ghi audit | AGENTS R23 · ADR-0031 | TC-AUTH-013, TC-AUTH-014 |
| F090-AC-03 | NẾU game session không thuộc người gọi hoặc thuộc tenant khác, THÌ module game PHẢI trả `404`; NẾU session đã kết thúc hoặc đã bị thay, THÌ PHẢI trả `409`; NẾU `X-Tenant-Id` khác tenant phiên, THÌ PHẢI trả `403 TENANT_MISMATCH` | FR-AUTH-019 · NFR-SEC-006 | TC-TEN-001, TC-TEN-011 |
| F090-AC-04 | NẾU người gọi vượt `ws_ticket_rate`, THÌ module game PHẢI trả `429 RATE_LIMITED` kèm `Retry-After`; NẾU Valkey không sẵn sàng, THÌ PHẢI trả `503 UNAVAILABLE` (REST vẫn dùng được) | FR-AUTH-011 · [VALKEY_DESIGN §6](../../docs/07-backend/VALKEY_DESIGN.md) | TC-PERF-008 |
| F090-AC-05 | KHI client mở `/realtime/v1?ticket=` với subprotocol `eduworld.v1`, gateway PHẢI tiêu thụ ticket bằng một thao tác nguyên tử, gắn cố định danh tính (user, tenant, school, game session, device) cho kết nối và gửi `system.session.welcome` (`conn_id`, `server_time`, `heartbeat_s`, `timeout_s`, `resume_token`, `resume_window_s`, `protocol_version`, `limits`) | FR-AUTH-011 · [REALTIME_ARCHITECTURE §3](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F090-AC-06 | NẾU ticket không tồn tại, hết hạn hoặc đã dùng, THÌ gateway PHẢI đóng `4401` và tăng `eduworld_rt_ticket_rejected_total{reason}`; NẾU thiếu subprotocol `eduworld.v1` hoặc `Origin` (Web) ngoài allowlist cấu hình, THÌ PHẢI từ chối handshake | ADR-0008 · [REALTIME_ARCHITECTURE §10](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F090-AC-07 | HỆ THỐNG PHẢI không nhận JWT trong query string hay frame WebSocket và không ghi ticket, resume token, nội dung payload vào log | ADR-0008 · AGENTS R14 | — |
| F090-AC-08 | KHI cùng học sinh mở kết nối mới (cùng hoặc khác node), gateway PHẢI đóng kết nối cũ với `4409` không kèm `session.session.replaced` và chỉ giữ một kết nối hiệu lực | ADR-0031 · [REALTIME_ARCHITECTURE §3](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F090-AC-09 | TRONG KHI kết nối mở, gateway PHẢI gắn `Ew-Tenant-Id` lấy từ ticket vào mọi lệnh chuyển tiếp và không bao giờ đổi tenant theo dữ liệu client gửi; đổi tenant bắt buộc ticket mới | FR-AUTH-019 · ADR-0065 | TC-TEN-012 |
| F090-AC-31 | TRONG KHI thời điểm server nằm trong giới nghiêm hiệu lực của học sinh, module game PHẢI từ chối `createRealtimeTicket` bằng `403 CURFEW_ACTIVE` kèm `reopen_at` và không lưu ticket | NFR-SAFE-005 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 | TC-AUTH-092 |

---

### User Story 2 — Heartbeat, timeout, reconnect và resume (Priority: P1)

Mạng chập chờn ở phòng máy: game tự kết nối lại và nhận bù các message quan trọng bị lỡ, không phải đăng nhập lại.

**Why this priority**: FR-MP-005; thiếu resume thì lời mời, kết quả, thông báo bị mất khi rớt mạng ngắn.

**Independent Test**: Ngắt mạng client 10 s (toxiproxy) → reconnect với ticket mới + `system.session.resume` → nhận lại message `seq > last_seq` rồi `system.session.resumed`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F090-AC-10 | KHI nhận `system.heartbeat.ping`, gateway PHẢI trả `system.heartbeat.pong` (`client_time`, `server_time`); gateway PHẢI gửi WebSocket ping control frame mỗi `ws_ping` | FR-MP-005 | — |
| F090-AC-11 | NẾU không nhận frame nào quá `ws_timeout`, THÌ gateway PHẢI đóng kết nối với `4408` | FR-MP-005 · [REALTIME_ARCHITECTURE §3.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F090-AC-12 | TRONG KHI kết nối mở, gateway PHẢI gửi lệnh `session.heartbeat` tới module game qua NATS mỗi `ws_heartbeat`, và module game PHẢI cập nhật `last_heartbeat_at` của game session như `heartbeatGameSession` (session đã kết thúc → reply lỗi, gateway đóng `4409`) | FR-MP-005 · FR-GAME-014 | TC-AUTH-017 |
| F090-AC-13 | Gateway PHẢI gán `seq` tăng dần cho mọi message server→client theo kết nối logic (giữ qua resume) và lưu message không droppable vào buffer resume trong `resume_window` | FR-MP-005 · [REALTIME_ARCHITECTURE §9](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F090-AC-14 | KHI client gửi `system.session.resume` (`resume_token`, `last_seq`) trên kết nối mới trong `resume_window`, gateway PHẢI phát lại theo thứ tự các message có `seq > last_seq`, khôi phục subscription đã lưu, rồi gửi `system.session.resumed` (`replayed`, `rejoined_rooms`) | FR-MP-005 | TC-AUTH-047 |
| F090-AC-15 | NẾU `resume_token` hết hạn, không tồn tại, thiếu đoạn `seq` hoặc thuộc học sinh khác, THÌ gateway PHẢI gửi `system.session.resume_failed` với `reason` (`expired`/`unknown_token`/`seq_gap`) và không tiết lộ dữ liệu của phiên khác | FR-MP-005 · NFR-SEC-006 | — |
| F090-AC-16 | NẾU client gửi lại lệnh có cùng `id` trong `resume_window`, THÌ gateway PHẢI trả lại đúng `system.command.acked`/`system.command.failed` đã trả và không chuyển tiếp lần hai | FR-MP-005 · NFR-SEC-004 | — |

---

### User Story 3 — Định tuyến lệnh và giao sự kiện (Priority: P1)

Gateway chuyển lệnh của client tới đúng module (handler NATS trong `core-api`) và giao sự kiện của module tới đúng kết nối, không tự quyết định quyền.

**Why this priority**: Là đường đi chung của chat, party, hub, thông báo (ADR-0074).

**Independent Test**: Stub handler NATS trả `{ok:true, subscribe:[…]}` → client nhận `system.command.acked` có `ref`; publish lên topic đã đăng ký → chỉ kết nối đã đăng ký nhận.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F090-AC-17 | KHI nhận lệnh có `type` trong bảng định tuyến, gateway PHẢI gửi NATS request `ew.cmd.<module>.<type>` kèm header `Ew-Tenant-Id`, `Ew-User-Id`, `Ew-Game-Session-Id`, `Ew-Device-Id`, `Ew-Conn-Id`, `Ew-Node-Id`, `traceparent` và trả đúng một `system.command.acked` hoặc `system.command.failed` có `ref` = `id`; `hub.player.move` PHẢI đi NATS core `ew.in.room.<room_id>` không ack | [REALTIME_ARCHITECTURE §4.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) · ADR-0074 | — |
| F090-AC-18 | NẾU module đích không trả lời trong `nats_request_timeout`, THÌ gateway PHẢI trả `system.command.failed` mã `UNAVAILABLE` kèm `retry_after_ms` | FR-MP-005 · NFR-AVAIL-005 | TC-SVC-012 |
| F090-AC-19 | KHI reply của module có `subscribe`/`unsubscribe`, gateway PHẢI cập nhật đăng ký Valkey Pub/Sub (đếm tham chiếu theo node) trước khi gửi ack; gateway PHẢI không tự đăng ký topic nào ngoài `ew:rt:user:<id>` của chính kết nối | [REALTIME_ARCHITECTURE §10](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | TC-CHAT-008, TC-TEN-012 |
| F090-AC-20 | NẾU envelope sai schema, `type` không có trong bảng định tuyến, chuỗi vượt `maxLength` của AsyncAPI hoặc frame vượt `ws_max_frame`, THÌ gateway PHẢI trả `system.command.failed` (`VALIDATION_FAILED`/`UNKNOWN_TYPE`), không chuyển tiếp; lỗi giao thức lặp lại PHẢI đóng `4400` | ADR-0041 · [AsyncAPI](../../specs/asyncapi/eduworld-realtime.v1.yaml) | — |
| F090-AC-21 | KHI có message trên `ew:rt:user:<id>`, `ew:rt:party:<id>`, `ew:rt:room:<id>:out`, `ew:chat:fanout:<id>` hoặc `ew:rt:broadcast`, gateway PHẢI giao tới đúng các kết nối local đã đăng ký topic đó và không tới kết nối khác | [REALTIME_ARCHITECTURE §5](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | TC-TEN-012 |
| F090-AC-22 | NẾU kết nối vượt giới hạn tần suất frame hoặc số lệnh đang chờ ([REALTIME_ARCHITECTURE §8](../../docs/06-architecture/REALTIME_ARCHITECTURE.md)), THÌ gateway PHẢI trả `RATE_LIMITED` kèm `retry_after_ms`; vượt quá 3 lần/phút PHẢI đóng `4429` và phát `audit.recorded` | [REALTIME_ARCHITECTURE §8.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F090-AC-23 | TRONG KHI hàng đợi gửi của một kết nối đầy, gateway PHẢI bỏ message `droppable` cũ (snapshot, presence) trước; NẾU message không droppable không đẩy được trong thời hạn cấu hình, THÌ PHẢI đóng `4408` để client resume | [REALTIME_ARCHITECTURE §8](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |

---

### User Story 4 — Cắt kết nối khi thu hồi quyền hoặc bị thay thiết bị (Priority: P1)

Học sinh bị khóa hoặc chuyển sang trường thuộc tenant khác thì bị ngắt ngay; đăng nhập ở máy khác thì máy cũ nhận thông báo rồi bị ngắt.

**Why this priority**: An toàn và quy tắc một thiết bị (ADR-0031); FR-AUTH-010.

**Independent Test**: Publish message điều khiển `access_revoked` lên `ew:rt:user:<id>` → kết nối đóng `4403` ≤ 5 s; takeover (F014) → máy cũ nhận `session.session.replaced` rồi `4409`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F090-AC-24 | KHI nhận message điều khiển thu hồi (`access_revoked`, kèm lý do `account_locked`/`access_revoked`/`cleanup`/`admin`/`password_changed`) trên `ew:rt:user:<id>`, gateway PHẢI gửi `session.session.ended` với `reason` tương ứng rồi đóng mọi kết nối của học sinh với `4403` trong ≤ 5 giây và xóa buffer resume của học sinh | FR-AUTH-010 · AC-AUTH-006 · [REALTIME_ARCHITECTURE §3.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | TC-AUTH-006, TC-AUTH-036, TC-TEN-009 |
| F090-AC-25 | KHI nhận `session.session.replaced` cho `game_session_id` mà kết nối đang giữ, gateway PHẢI giao message đó rồi đóng `4409` và vô hiệu resume token cũ | FR-AUTH-023 · ADR-0031 | TC-AUTH-015 |
| F090-AC-26 | NẾU message điều khiển/`session.session.replaced` nhắm `game_session_id` khác với kết nối hiện tại, THÌ gateway PHẢI không đóng kết nối đó | ADR-0031 | TC-AUTH-015 |
| F090-AC-32 | KHI nhận `session.session.ended` với `reason = curfew` trên `ew:rt:user:<id>` cho `game_session_id` mà kết nối đang giữ, gateway PHẢI giao message đó rồi đóng kết nối với `4410` (không phải `4403`), xóa buffer resume và vô hiệu resume token; message nhắm `game_session_id` khác PHẢI không đóng kết nối | ADR-0078 · [REALTIME_ARCHITECTURE §3.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md) | TC-AUTH-092 |

---

### User Story 5 — Scale ngang và tắt êm (Priority: P2)

Vận hành tăng/giảm replica theo lịch học, deploy không làm học sinh mất message.

**Why this priority**: NFR-SCAL-003, NFR-AVAIL-004; cần trước load test 10.000 CCU (F132).

**Independent Test**: 2 node gateway; client trên node 1, publish từ module → nhận đúng; SIGTERM node 1 → client nhận `system.server.draining` rồi close `1012`, resume trên node 2 không mất message không droppable.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F090-AC-27 | KHI node nhận SIGTERM, gateway PHẢI chuyển readiness sang `503`, ngừng nhận kết nối mới, gửi `system.server.draining` (kèm `reconnect_after_ms` ngẫu nhiên để giãn tải) rồi đóng dần các kết nối với close code `1012` trong tối đa 30 giây và hủy subscribe Valkey; client resume ở node khác PHẢI nhận đủ message không droppable trong `resume_window` | NFR-AVAIL-004 · [REALTIME_ARCHITECTURE §3.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F090-AC-28 | HỆ THỐNG PHẢI chạy nhiều replica gateway không cần sticky session: kết nối ở node bất kỳ nhận đúng message của mình và thay thế kết nối khác node qua kênh `ew:rt:node:<node_id>:in` | NFR-SCAL-003 · ADR-0074 | TC-PERF-014 |
| F090-AC-29 | NẾU node vượt tốc độ nhận kết nối mới cấu hình hoặc `ws_per_gateway_node`, THÌ gateway PHẢI đóng kết nối mới với `4429` kèm thời gian thử lại | [REALTIME_ARCHITECTURE §11](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | TC-PERF-014 |
| F090-AC-30 | NẾU Valkey mất kết nối, THÌ gateway PHẢI báo readiness thất bại (không nhận kết nối mới) và gửi `system.service.degraded` tới kết nối hiện có; khi Valkey phục hồi PHẢI tự đăng ký lại topic | NFR-AVAIL-003 · [VALKEY_DESIGN §6](../../docs/07-backend/VALKEY_DESIGN.md) | TC-PERF-008 |

### Edge Cases

- NẾU access token hết hạn khi WS đang mở, THÌ kết nối không bị cắt (ADR-0008); chỉ cắt khi thu hồi/thay thế (TC-AUTH-002).
- NẾU NATS mất, THÌ lệnh trả `UNAVAILABLE` (F090-AC-18), giao sự kiện qua Valkey vẫn chạy (TC-SVC-012).
- NẾU hai kết nối của cùng học sinh đến hai node cùng lúc, THÌ kết nối đăng ký sau vào `ew:rt:conn:<user_id>` thắng, kết nối kia đóng `4409` (F090-AC-08).
- NẾU học sinh đổi tenant (token mới) khi WS đang mở, THÌ WS giữ tenant cũ tới khi client xin ticket mới (TC-TEN-011d).
- NẾU message điều khiển đến node không giữ kết nối, THÌ node bỏ qua (chỉ node có subscriber mới nhận).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-010 | Cắt kết nối realtime khi nhận lệnh thu hồi (`4403`, ≤ 5 s); phần publish do module identity (F016) | [REALTIME_ARCHITECTURE §10](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) |
| FR-AUTH-011 | Ticket dùng một lần, TTL, băm khi lưu, tiêu thụ nguyên tử | [ADR-0008](../../docs/adr/ADR-0008-access-token-cua-keycloak-la-credential-api-realtime-dung-ti.md) |
| FR-AUTH-019 | Tenant cố định theo ticket; đổi tenant cần ticket mới | [ADR-0065](../../docs/adr/ADR-0065-tenant-id-tren-header-moi-request.md) |
| FR-MP-005 | Heartbeat, timeout, reconnect, resume, xử lý mất kết nối | [REALTIME_ARCHITECTURE §3, §9](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) |
| NFR-AVAIL-004 | Tắt êm: readiness, `system.server.draining` + đóng `1012`, client resume | [REALTIME_ARCHITECTURE §3.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) |
| NFR-SCAL-003 | Gateway không trạng thái nghiệp vụ, scale ngang, định tuyến xuyên node | [REALTIME_ARCHITECTURE §11](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) |

### Key Entities

- **RealtimeTicket**: vé một lần, ngắn hạn, gắn học sinh + tenant + game session + thiết bị.
- **Connection**: kết nối WebSocket đã xác thực; danh tính cố định, `seq` outbound, hàng đợi gửi, rate limiter.
- **ResumeBuffer**: các message không droppable gần nhất của một kết nối logic, dùng khi reconnect.
- **RoutingTable**: ánh xạ tiền tố `type` → module đích, sinh từ AsyncAPI.
- **ControlMessage**: lệnh điều khiển từ module tới gateway qua Pub/Sub (thu hồi, thay thế).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `realtime_ticket_ttl` | TTL và số lần dùng ticket |
| `ws_ticket_rate` / `ws_connections_per_user` | Giới hạn xin ticket; số kết nối mỗi học sinh |
| `ws_heartbeat` / `ws_timeout` / `ws_ping` | Heartbeat client, timeout không nhận frame, WS ping |
| `ws_max_frame` | Kích thước frame client→server tối đa |
| `resume_window` | Thời gian và số message giữ để resume |
| `nats_request_timeout` | Timeout lệnh NATS request/reply |
| `ws_per_gateway_node` | Số kết nối mục tiêu mỗi node |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Gateway không có database, không logic nghiệp vụ; quyền join/subscribe do module sở hữu quyết định | ADR-0074 · [REALTIME_ARCHITECTURE §1, §10](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) |
| Chỉ user NATS `realtime` (binary `realtime`) được publish `ew.cmd.>`/`ew.in.>` | [SECURITY_ARCHITECTURE §8.1](../../docs/10-security/SECURITY_ARCHITECTURE.md) |
| Ticket chỉ học sinh; không JWT trong URL; không log token/ticket/payload | AGENTS R14, R23 · ADR-0008 |
| Valkey chỉ trạng thái tạm; mọi key có TTL, prefix theo module | AGENTS R9 · ADR-0004 |
| Mọi chuỗi trong envelope có `maxLength` | AGENTS R28 · ADR-0041 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% ticket chỉ mở được đúng một kết nối; 0 kết nối được chấp nhận bằng ticket đã dùng/hết hạn trong test 1.000 lần.
- **SC-002**: Học sinh bị khóa mất kết nối realtime trong ≤ 5 giây sau khi lệnh thu hồi được publish (đạt AC-AUTH-006 ≤ 60 s toàn tuyến).
- **SC-003**: Rớt mạng ≤ `resume_window`: 0 message không droppable bị mất hoặc trùng sau resume trong 100 lần thử.
- **SC-004**: Rolling restart toàn bộ node gateway khi đang có tải: 0 lỗi 5xx dây chuyền; 100% client kết nối lại được.
- **SC-005**: Mỗi node giữ `ws_per_gateway_node` kết nối ở CPU ≤ 50% trong load test (TC-PERF-014).

## Ngoài phạm vi

- Publish lệnh thu hồi khi khóa/chuyển trường → F016 (module identity).
- Hub instance, presence, đồng bộ vị trí → F091; `RealtimeClient` Godot → F092.
- Nghiệp vụ chat, party, matchmaking, battle, thông báo → F115, F100/F101, F104, F105, F123.
- Load test 10.000 CCU, chaos test → F132; cấu hình APISIX WS route → F004 (chỉ kiểm lại idle timeout).

## Assumptions

- F003 cung cấp `src/backend/pkg/natsx` (request/reply, header ngữ cảnh) và key builder Valkey; F014 đã có game session và `ew:game:active_session:<student_id>`.
- Giới hạn tần suất frame, số lệnh đang chờ, hàng đợi gửi, tốc độ nhận kết nối mới chưa có tên trong CONFIG_DEFAULTS; đề xuất `ws_frame_rate`, `ws_inflight_commands`, `ws_send_queue`, `ws_accept_rate` (task Polish) [Confirmed].
- Stream `EVT_REALTIME` cho `audit.recorded` của gateway [Confirmed] — cần cập nhật EVENT_DESIGN §3.1.
