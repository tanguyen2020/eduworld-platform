# Implementation Plan: F090 — realtime: ticket, WebSocket, heartbeat/reconnect, định tuyến lệnh, cắt kết nối khi thu hồi

**Branch**: `090-realtime-gateway` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Dựng module `realtime` và binary `src/backend/cmd/realtime` (Go mỏng, không database, ADR-0074) theo [REALTIME_ARCHITECTURE §2–§11](../../docs/06-architecture/REALTIME_ARCHITECTURE.md): `ConnManager` nhận WS qua APISIX, `GETDEL` ticket ở Valkey, đăng ký `ew:rt:conn:<user_id>` (thay kết nối cũ `4409`, xuyên node qua `ew:rt:node:<node>:in`), `Session` per connection (hàng đợi gửi có giới hạn, `seq`, buffer resume Valkey, rate limiter), `Router` sinh từ AsyncAPI (`type` → `ew.cmd.<module>.<type>`, handler trong `core-api`), `Subscriptions` đếm tham chiếu topic Valkey, `Revocation` xử lý message điều khiển. module game bổ sung `createRealtimeTicket` và NATS handler `ew.cmd.game.session.heartbeat`. Thêm `src/backend/pkg/realtime` (envelope Pub/Sub + publisher) để mọi module (trong `core-api`/`worker`) đẩy sự kiện/điều khiển theo một schema.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | `coder/websocket` (hoặc `gorilla/websocket`) [Confirmed, research R2], `valkey-io/valkey-go`, `nats.go`, `src/backend/pkg/natsx`, `src/backend/pkg/config`, `src/backend/pkg/observability`, `src/backend/pkg/health`; module game: chi, ent, oapi-codegen, `src/backend/pkg/httpx` (`game_player`, `tenant`) |
| Storage | Không có PostgreSQL ở gateway; Valkey: `ew:game:rt:ticket:*` (game ghi), `ew:gateway:rt:*` (conn, node, resume); module game đọc/ghi `game.game_sessions` (F014) |
| Testing | go test; testcontainers Valkey + NATS; client WS headless; toxiproxy cho reconnect |
| Target Platform | Linux container (binary `realtime`; cổng public và cổng quản trị theo LOCAL_DEVELOPMENT §3.2) |
| Project Type | backend module + binary `realtime` (gateway) + phần nhỏ module game |
| Performance Goals | `ws_per_gateway_node` ở CPU ≤ 50%; lệnh NATS p95 theo [REALTIME_ARCHITECTURE §12](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) |
| Constraints | AGENTS R3, R9, R13, R14, R23, R28, R29 · ADR-0004, ADR-0008, ADR-0011, ADR-0031, ADR-0074, ADR-0065 |
| Scale/Scope | 10.000 CCU, 4–6 replica (REALTIME_ARCHITECTURE §11) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta AsyncAPI/OpenAPI/schema Pub/Sub ở Phase 2 (T004–T007) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Gateway không domain, không DB ([BACKEND_DESIGN §3.1](../../docs/07-backend/BACKEND_DESIGN.md)); lệnh tới module đích qua NATS request/reply (handler trong `core-api`); module game heartbeat gọi command handler qua interface có kiểu |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Tenant cố định theo ticket; Valkey chỉ trạng thái tạm có TTL; test TC-TEN-012 |
| IV Server-authoritative | ✅ | Gateway không quyết định kết quả; `seq`, `server_time` do server cấp |
| V Xác thực, phân quyền | ✅ | Ticket chỉ cho học sinh (`game_player`); module đích tự kiểm quyền join; chỉ user NATS `realtime` (binary `realtime`) publish `ew.cmd.>` |
| VI An toàn trẻ em | ✅ | Không log payload/ticket; không PII trong envelope |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo CONFIG_DEFAULTS; tham số chưa có tên đăng ký ở Polish |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test cho mọi F090-AC-*, test đa node, chaos Valkey/NATS |

## Project Structure

### Documentation (feature này)

```text
specs/090-realtime-gateway/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/realtime/
├── module.go                           # Register(...) cho composition root `src/backend/cmd/realtime/main.go` (HTTP /realtime/v1, cổng quản trị)
├── internal/config/config.go           # EW_WS_*, EW_CMD_TIMEOUT, allowlist Origin
├── internal/protocol/                  # envelope, validator sinh từ AsyncAPI (maxLength, frame size)
├── internal/router/routes_gen.go       # bảng type → subject (sinh bởi scripts/realtime/gen-routes.mjs)
├── internal/conn/                      # ConnManager, Session, sendqueue (droppable), seq
├── internal/ticket/consumer.go         # GETDEL ew:game:rt:ticket:<sha256>
├── internal/registry/                  # ew:gateway:rt:conn, rt:node:*, rt:nodes, kênh rt:node:<id>:in
├── internal/subs/                      # Valkey Pub/Sub ref-count
├── internal/resume/                    # buffer ew:gateway:rt:resume:<token>, dedupe id lệnh
├── internal/revocation/                # control access_revoked, session.session.replaced
├── internal/ratelimit/                 # token bucket theo kết nối
├── internal/audit/publisher.go         # audit.recorded → JetStream EVT_REALTIME
├── internal/observability/metrics.go   # eduworld_rt_*
└── tests/{unit,integration,e2e}/
src/backend/modules/game/
├── internal/application/command/issue_realtime_ticket.go
├── internal/application/port/ticket_store.go
├── internal/infrastructure/realtime/ticket_store.go
├── internal/presentation/http/public/realtime_tickets.go
├── internal/presentation/natscmd/session_heartbeat.go
└── tests/integration/realtime_ticket_test.go
src/backend/cmd/realtime/main.go                    # composition root binary realtime (F001)
src/backend/pkg/realtime/                           # Envelope Pub/Sub (message | control), Publisher, topic builder
scripts/realtime/gen-routes.mjs         # sinh routes_gen.go từ AsyncAPI
```

**Structure Decision**: Gateway theo bảng cổng [LOCAL_DEVELOPMENT §3](../../docs/11-operations/LOCAL_DEVELOPMENT.md) (`src/backend/modules/realtime/`, binary `src/backend/cmd/realtime`, ADR-0074); không có tầng domain/application vì không có nghiệp vụ; không nạp module domain nào. Ticket nằm ở module game vì gắn game session (ADR-0008, ADR-0031).

## Phase 0: Research

→ [research.md](research.md): R1 schema Pub/Sub và message điều khiển; R2 thư viện WebSocket; R3 buffer resume và `seq`; R4 audit của gateway không có outbox.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — (gateway); module game: dùng aggregate `GameSession` của F014 | |
| application | module game: Command `IssueRealtimeTicket` (kiểm session active của chính mình, tenant), `HeartbeatGameSession` (F014, tái dùng) | Route `game_player` |
| infrastructure | `TicketStore` (Valkey `SET NX EX`), gateway: Valkey client, NATS client, audit publisher | |
| presentation | `createRealtimeTicket` · NATS `ew.cmd.game.session.heartbeat` · gateway `/realtime/v1` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F003 (`src/backend/pkg/natsx`, key builder), F013 (token game), F014 (game session, `session.session.replaced`), F004 (route APISIX `/realtime/v1`, idle timeout 120 s).
- Không facade, không HTTP nội bộ: gateway chỉ dùng NATS + Valkey. Module khác đẩy sự kiện bằng `src/backend/pkg/realtime.Publisher`.
- Message điều khiển `access_revoked` do module identity publish (F016); `session.session.replaced` do module game (F014).

### Bảo mật, tenant, idempotency, audit

- Ticket: 32 byte `crypto/rand`, base64url; lưu `sha256(ticket)`; payload không PII ngoài ID.
- `Ew-*` header chỉ gateway đặt; user NATS `realtime` (binary `realtime`) là nguồn duy nhất của `ew.cmd.>`/`ew.in.>`.
- Lệnh trùng `id` trả kết quả cũ (F090-AC-16); `createRealtimeTicket` không cần `Idempotency-Key` (mỗi lần cấp vé mới, rate limit).
- Audit: `game_access_denied_non_student` (module game outbox), `realtime.rate_limit_closed` (gateway → `EVT_REALTIME`).

### Quan sát

- Metric: `eduworld_rt_connections{node}`, `eduworld_rt_messages_total{direction,type}`, `eduworld_rt_send_queue_dropped_total{type}`, `eduworld_rt_cmd_latency_seconds{module,type}`, `eduworld_rt_cmd_failed_total{module,code}`, `eduworld_rt_resume_total{result}`, `eduworld_rt_ticket_rejected_total{reason}`, `eduworld_rt_message_delivery_latency_seconds`.
- Span cho lệnh, truyền `traceparent` qua header NATS; log không chứa payload.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
