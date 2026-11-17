# Tasks: F090 — realtime: ticket, WebSocket, heartbeat/reconnect, định tuyến lệnh, cắt kết nối khi thu hồi

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Skeleton module `realtime` (`src/backend/modules/realtime/module.go`, composition root `src/backend/cmd/realtime` của F001, `src/backend/pkg/config` với `EW_WS_MAX_CONNECTIONS`, `EW_WS_HEARTBEAT`, `EW_WS_TIMEOUT`, `EW_CMD_TIMEOUT`, `EW_NODE_ID`, `EW_CORS_ALLOWED_ORIGINS`; health `/livez` `/readyz` trên cổng quản trị; rule depguard/arch-check) — file: `src/backend/modules/realtime/`
- [ ] T002 [P] Thêm Swarm/compose service `realtime` (`src/backend/cmd/realtime`, F001), Makefile `run-realtime`, ACL Valkey user `realtime` (`~ew:gateway:*`, đọc + `GETDEL` `~ew:game:rt:ticket:*`, kênh `&ew:rt:*`, `&ew:chat:fanout:*`), user NATS `realtime` (publish `ew.cmd.>`, `ew.in.>`, `ew.evt.realtime.>`) — file: `deploy/compose/docker-compose.yml`, `deploy/valkey/users.acl`, `deploy/nats/accounts.conf`, `Makefile`
- [ ] T003 [P] Gói `src/backend/pkg/realtime`: `Envelope` (message/control theo research R1), `Publisher.ToUser/ToRoom/ToParty/Broadcast`, topic builder — file: `src/backend/pkg/realtime/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta OpenAPI `createRealtimeTicket` (401, 403, 409, 503) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game`
- [ ] T005 [P] Delta AsyncAPI: mô tả định tuyến (`hub.*` → competition, header `Ew-*`), `x-routing` từng operation receive, subject `ew.cmd.game.session.heartbeat`, message `system.server.draining` + close `1012` thay `4500` — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T006 [P] JSON Schema `rt-pubsub-envelope.schema.json` (mọi string có `maxLength`) — file: `specs/schemas/realtime/rt-pubsub-envelope.schema.json` — kiểm: `make specs-lint`
- [ ] T007 [P] Stream `EVT_REALTIME` (payload `audit.recorded` dùng schema chung của F003 T004) — file: `specs/events/README.md`
- [ ] T008 Script sinh bảng định tuyến + validator envelope từ AsyncAPI; CI fail khi lệch — file: `scripts/realtime/gen-routes.mjs`, `src/backend/modules/realtime/internal/router/routes_gen.go`, `src/backend/modules/realtime/internal/protocol/validate_gen.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Cấp ticket và mở kết nối WebSocket (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T009 [P] [US1] Test F090-AC-01, F090-AC-04: ticket ≥ 256 bit, lưu SHA-256, TTL `realtime_ticket_ttl`, `url` từ cấu hình; vượt `ws_ticket_rate` → 429; Valkey down → 503 — file: `src/backend/modules/game/tests/integration/realtime_ticket_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Test authorization + tenant isolation F090-AC-02, F090-AC-03: token nhân sự (5 role) → `GAME_ACCESS_STUDENT_ONLY` + audit; session người khác/tenant khác → 404; session đã kết thúc → 409; header tenant khác → `TENANT_MISMATCH` — file: `src/backend/modules/game/tests/integration/realtime_ticket_authz_test.go`
- [ ] T011 [P] [US1] Test F090-AC-05, F090-AC-06, F090-AC-07: welcome đủ trường; ticket dùng lại/hết hạn → 4401 + metric; thiếu subprotocol/Origin lạ → từ chối; quét log không có ticket/payload — file: `src/backend/modules/realtime/tests/integration/handshake_test.go`
- [ ] T012 [P] [US1] Test F090-AC-08, F090-AC-09: kết nối mới thay cũ (cùng node, khác node) → 4409 im lặng; header `Ew-Tenant-Id` luôn từ ticket kể cả khi payload chứa `tenant_id` khác (TC-TEN-012) — file: `src/backend/modules/realtime/tests/integration/single_connection_test.go`
- [ ] T040 [P] [US1] Test F090-AC-31 (Clock giả 21:35): `createRealtimeTicket` → `403 CURFEW_ACTIVE`, không có khóa ticket trong Valkey — file: `src/backend/modules/game/tests/integration/realtime_ticket_curfew_test.go` — phải FAIL trước khi code

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Command `IssueRealtimeTicket` + port `TicketStore` + adapter Valkey `SET NX EX` — file: `src/backend/modules/game/internal/application/command/issue_realtime_ticket.go`, `src/backend/modules/game/internal/infrastructure/realtime/ticket_store.go`
- [ ] T014 [US1] Handler `createRealtimeTicket` (`game_player`, `tenant`, rate limit `ws_ticket_rate`) — file: `src/backend/modules/game/internal/presentation/http/public/realtime_tickets.go`
- [ ] T015 [US1] `ConnManager` (handshake, subprotocol, Origin, `GETDEL`, welcome), `Session` (identity cố định) — file: `src/backend/modules/realtime/internal/conn/manager.go`, `src/backend/modules/realtime/internal/ticket/consumer.go`
- [ ] T016 [US1] Registry một kết nối/học sinh (Lua đọc-ghi `ew:gateway:rt:conn`, kênh `ew:rt:node:<id>:in`, node alive/nodes) — file: `src/backend/modules/realtime/internal/registry/`
- [ ] T041 [US1] Kiểm giới nghiêm (projection của F014 T041) trong `CreateRealtimeTicket`; contract `403 CURFEW_ACTIVE` — file: `src/backend/modules/game/internal/application/command/create_realtime_ticket.go`, `specs/openapi/eduworld-api.v1.yaml`

**Checkpoint**: mở được kết nối đã xác thực.

## Phase 4: User Story 2 — Heartbeat, timeout, reconnect và resume (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Test F090-AC-10, F090-AC-11: ping/pong có `server_time`; WS ping mỗi `ws_ping`; im lặng quá `ws_timeout` → 4408 (Clock giả) — file: `src/backend/modules/realtime/tests/integration/heartbeat_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Test F090-AC-12: gateway gửi `ew.cmd.game.session.heartbeat` mỗi `ws_heartbeat`; module game cập nhật `last_heartbeat_at`; session đã kết thúc → reply lỗi → gateway đóng 4409 — file: `src/backend/modules/game/tests/integration/nats_session_heartbeat_test.go`
- [ ] T019 [P] [US2] Test F090-AC-13, F090-AC-14, F090-AC-15, F090-AC-16: `seq` liên tục qua resume khác node; replay đúng thứ tự; token hết hạn/khác học sinh → `resume_failed`; lệnh trùng `id` → cùng kết quả, không chuyển tiếp lần hai — file: `src/backend/modules/realtime/tests/integration/resume_test.go`

### Implementation cho User Story 2

- [ ] T020 [US2] Heartbeat, WS ping, timeout 4408, gửi heartbeat NATS định kỳ — file: `src/backend/modules/realtime/internal/conn/heartbeat.go`
- [ ] T021 [US2] NATS handler `session.heartbeat` → command handler `HeartbeatGameSession` (F014) qua interface có kiểu — file: `src/backend/modules/game/internal/presentation/natscmd/session_heartbeat.go`
- [ ] T022 [US2] Buffer resume + meta + dedupe `cmd_id` (research R3) — file: `src/backend/modules/realtime/internal/resume/`

## Phase 5: User Story 3 — Định tuyến lệnh và giao sự kiện (Priority: P1)

### Tests cho User Story 3

- [ ] T023 [P] [US3] Test F090-AC-17, F090-AC-18: subject đúng theo bảng, đủ header `Ew-*` + `traceparent`, đúng một ack/failed có `ref`; `hub.player.move` → `ew.in.room.<room_id>` không ack; service im lặng → `UNAVAILABLE` sau `nats_request_timeout` — file: `src/backend/modules/realtime/tests/integration/router_test.go` — phải FAIL trước khi code
- [ ] T024 [P] [US3] Test F090-AC-19, F090-AC-21: chỉ subscribe topic theo reply; message topic chỉ tới kết nối đã đăng ký; tenant B không nhận topic của tenant A (TC-TEN-012, TC-CHAT-008) — file: `src/backend/modules/realtime/tests/integration/subscriptions_test.go`
- [ ] T025 [P] [US3] Test F090-AC-20 (giới hạn độ dài/frame, `UNKNOWN_TYPE`, 4400 khi lặp), F090-AC-22 (rate limit, 4429, audit `EVT_REALTIME`), F090-AC-23 (drop droppable, 4408 khi message quan trọng kẹt) — file: `src/backend/modules/realtime/tests/integration/limits_test.go`

### Implementation cho User Story 3

- [ ] T026 [US3] `Router` (NATS request/reply, header, timeout, map reply → ack/failed) — file: `src/backend/modules/realtime/internal/router/router.go`
- [ ] T027 [US3] `Subscriptions` ref-count + giao message theo `RtPubSubEnvelope` (droppable, target) — file: `src/backend/modules/realtime/internal/subs/`
- [ ] T028 [US3] Rate limiter, send queue có giới hạn, audit publisher — file: `src/backend/modules/realtime/internal/ratelimit/`, `src/backend/modules/realtime/internal/conn/sendqueue.go`, `src/backend/modules/realtime/internal/audit/publisher.go`

## Phase 6: User Story 4 — Cắt kết nối khi thu hồi quyền hoặc bị thay thiết bị (Priority: P1)

### Tests cho User Story 4

- [ ] T029 [P] [US4] Test F090-AC-24: control `access_revoked` (mỗi lý do, gồm `password_changed`) → `session.session.ended` đúng `reason` rồi 4403 ≤ 5 s, buffer resume bị xóa, resume sau đó thất bại (TC-AUTH-006, TC-TEN-009) — file: `src/backend/modules/realtime/tests/integration/revocation_test.go` — phải FAIL trước khi code
- [ ] T042 [P] [US4] Test F090-AC-32: `session.session.ended` (`curfew`, `reopen_at`) khớp session → giao rồi `4410` (không 4403), buffer resume bị xóa, resume thất bại; khác session → giữ kết nối (TC-AUTH-092) — file: `src/backend/modules/realtime/tests/integration/session_ended_test.go` — phải FAIL trước khi code
- [ ] T030 [P] [US4] Test F090-AC-25, F090-AC-26: `session.session.replaced` khớp session → giao rồi 4409; khác session → giữ kết nối (E2E với module game takeover, TC-AUTH-015) — file: `tests/e2e/realtime_takeover_test.go`

### Implementation cho User Story 4

- [ ] T031 [US4] `Revocation` xử lý control, `session.session.replaced` (4409) và `session.session.ended` (4410, F090-AC-32) — file: `src/backend/modules/realtime/internal/revocation/handler.go`

## Phase 7: User Story 5 — Scale ngang và tắt êm (Priority: P2)

### Tests cho User Story 5

- [ ] T032 [P] [US5] Test F090-AC-27: SIGTERM → readiness 503, gửi `system.server.draining` rồi đóng `1012` rải ≤ 30 s, resume ở node khác đủ message — file: `src/backend/modules/realtime/tests/integration/graceful_shutdown_test.go` — phải FAIL trước khi code
- [ ] T033 [P] [US5] Test F090-AC-28, F090-AC-29: 2 node không sticky, giao đúng, thay kết nối xuyên node; vượt tốc độ nhận/`ws_per_gateway_node` → 4429 — file: `tests/e2e/realtime_multinode_test.go`
- [ ] T034 [P] [US5] Test F090-AC-30: Valkey down → readiness fail + `system.service.degraded`; phục hồi → tự subscribe lại (TC-PERF-008) — file: `src/backend/modules/realtime/tests/integration/valkey_degraded_test.go`

### Implementation cho User Story 5

- [ ] T035 [US5] Drain + readiness + giới hạn tốc độ nhận kết nối + watcher Valkey reconnect — file: `src/backend/modules/realtime/internal/conn/drain.go`, `src/backend/cmd/realtime/main.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T036 [P] Đăng ký `ws_frame_rate`, `ws_inflight_commands`, `ws_send_queue`, `ws_accept_rate`; làm rõ `ws_connections_per_user`; kiểm close code `4403` (EVENT_DESIGN §4.1) và `system.server.draining` + `1012` (BACKEND_DESIGN §13) khớp REALTIME_ARCHITECTURE §3.1; thêm key `rt:resume:*:meta`, `rt:cmd:*`, field `game_session_id` (VALKEY_DESIGN §3.1); stream `EVT_REALTIME` (EVENT_DESIGN §3.1); CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/07-backend/EVENT_DESIGN.md`, `docs/07-backend/BACKEND_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `CHANGELOG.md`
- [ ] T037 [P] Metric `eduworld_rt_*` + dashboard Grafana realtime — file: `src/backend/modules/realtime/internal/observability/metrics.go`, `deploy/observability/grafana/realtime.json`
- [ ] T038 [P] WebSocket fuzz (envelope ngẫu nhiên, frame lớn) theo SECURITY_ARCHITECTURE §16 — file: `src/backend/modules/realtime/tests/fuzz/envelope_fuzz_test.go`
- [ ] T039 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T008 trước mọi story gateway) → US1 → US2, US3 song song → US4 → US5 → Polish.
- T030 cần F014 (takeover) chạy thật; message điều khiển `access_revoked` thật cần F016 (trước đó test bằng publish thủ công, không phải mock trong đường chạy thật).

## Definition of Done

- [ ] Mọi tiêu chí F090-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho `realtime` và phần module `game`; `make test-e2e` đa node xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
