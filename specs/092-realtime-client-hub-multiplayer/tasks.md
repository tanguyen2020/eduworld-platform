# Tasks: F092 — `RealtimeClient` Godot và Main Hub nhiều người chơi

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Thêm khóa dịch `ui.realtime.reconnecting`, `ui.realtime.degraded`, `ui.realtime.account_locked`, `ui.hub.room_full`, `ui.hub.join_denied`, `ui.hub.area_not_found`, `ui.hub.emote_rate_limited`, `ui.hub.emote_not_owned` — file: `src/game/locale/vi.po`, `src/game/locale/en.po`
- [ ] T002 [P] Interface `WsPeer` + `FakeWebSocketPeer` (kịch bản close code, độ trễ, mất gói) — file: `src/game/scripts/networking/ws_peer.gd`, `src/game/tests/fakes/fake_websocket_peer.gd`
- [ ] T003 [P] Lọc khóa `ticket`, `resume_token`, `access_token` trong `Log` — file: `src/game/scripts/core/logger.gd`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta AsyncAPI: thêm `examples` cho message system/session/hub (contracts §6) — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T005 Sinh fixture JSON từ example AsyncAPI + test parse mọi fixture — file: `scripts/godot/gen-realtime-fixtures.mjs`, `src/game/tests/fixtures/realtime/`, `src/game/tests/unit/networking/test_realtime_fixtures.gd`
- [ ] T006 `WsEnvelope` (parse/serialize, kiểm `v`, mẫu `type`, `seq`, `maxLength`, `max_frame_bytes`) và model typed (`RealtimeSession`, `PublicPlayerModel`, `SnapshotSample`) — file: `src/game/scripts/networking/ws_envelope.gd`, `src/game/scripts/multiplayer/models/`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Kết nối realtime ổn định (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T007 [P] [US1] Unit F092-AC-01, F092-AC-02: xin ticket kèm `X-Tenant-Id`, mở `wss` subprotocol, welcome → `Online`; 403 `GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` → `Stopped`, không retry — file: `src/game/tests/unit/networking/test_realtime_connect.gd` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Unit F092-AC-03, F092-AC-04: heartbeat theo `heartbeat_s`, im lặng 2× → reconnect; bảng close code 4401/4403/4408/4409 (có/không message)/4429/1012 sau `system.server.draining` → đúng trạng thái và signal (`session_expired`, `GameSessionService.session_replaced`) — file: `src/game/tests/unit/networking/test_realtime_close_codes.gd`
- [ ] T009 [P] [US1] Unit F092-AC-05, F092-AC-06: resume gửi `last_seq`, bỏ `seq` trùng; `resume_failed` → `getGameProfile` + join lại khu; `send_command` ghép `ref`, timeout `client_command_timeout`, `type` lạ bị bỏ, payload sai kiểu bị loại — file: `src/game/tests/unit/networking/test_realtime_resume_commands.gd`
- [ ] T010 [P] [US1] Unit F092-AC-07, F092-AC-09: log/telemetry không chứa ticket/token; build release từ chối `ws://`; Online → dừng heartbeat REST, gửi `presence.status.set` (`online`/`in_activity`/`in_match`/`away`) khi đổi hoạt động/idle 5 phút — file: `src/game/tests/unit/networking/test_realtime_security_presence.gd`
- [ ] T011 [P] [US1] Scene test F092-AC-08: banner hiện khi Backoff, ẩn khi Online, chỉ báo `system.service.degraded`, khóa lệnh ghi — file: `src/game/tests/scene/test_network_banner.gd`

### Implementation cho User Story 1

- [ ] T012 [US1] Autoload `RealtimeClient` (state machine data-model §2, poll `_process`, buffer 1 MB, `send_command`, dispatcher theo tiền tố, heartbeat + `Clock.sync`, resume) — file: `src/game/scripts/networking/realtime_client.gd`
- [ ] T013 [US1] Nối `GameSessionService` (F061): Online → tắt heartbeat REST; `session.session.replaced`/4409 → `session_replaced` — file: `src/game/scripts/authentication/game_session_service.gd`
- [ ] T014 [P] [US1] `PresenceService` (idle timer 5 phút, hoạt động từ `SceneRouter`) — file: `src/game/scripts/multiplayer/presence_service.gd`
- [ ] T015 [P] [US1] `NetworkBanner` (Theme token, khóa dịch) gắn vào `MainHud` — file: `src/game/scenes/ui/hud/NetworkBanner.tscn`, `src/game/scenes/ui/hud/network_banner.gd`

**Checkpoint**: kết nối realtime chạy và hồi phục.

## Phase 4: User Story 2 — Main Hub có người chơi khác (Priority: P1)

### Tests cho User Story 2

- [ ] T016 [P] [US2] Unit F092-AC-11: throttle `hub.player.move` (≤ `client_input_rate`, ngưỡng 2 cm/2°, keepalive 1 Hz, lượng tử 1 cm) — file: `src/game/tests/unit/multiplayer/test_hub_move_throttle.gd` — phải FAIL trước khi code
- [ ] T017 [P] [US2] Unit F092-AC-12, F092-AC-14: `Interpolator` (render_time, lerp/slerp, ngoại suy ≤ 250 ms, tăng delay khi jitter, blend hiệu chỉnh 200 ms); sai số trung bình ≤ 0,3 m khi mất 20% snapshot, jitter 80 ms — file: `src/game/tests/unit/world/test_interpolator.gd`
- [ ] T018 [P] [US2] Scene test F092-AC-10, F092-AC-13, F092-AC-15, F092-AC-16, F092-AC-17: assigned → spawn + RemotePlayer; joined/left/removed qua pool + fade; avatar từ `avatar_config` + appearance_changed; không hiển thị tên thật/trường/lớp; emote; migrated không reload scene; lỗi join → thông báo theo `code` — file: `src/game/tests/scene/test_remote_players.gd`
- [ ] T019 [P] [US2] Scene test F092-AC-18: chuỗi mới qua khóa dịch, `lint_theme_overrides.gd` không lỗi — file: `src/game/tests/scene/test_hub_i18n_theme.gd`
- [ ] T020 [US2] Integration F092-AC-19 + tenant isolation: 2 client headless cùng trường thấy nhau; `hs.dung.7a` (tenant khác) không bao giờ xuất hiện (TC-TEN-012) — file: `src/game/tests/integration/test_hub_multiplayer.gd`

### Implementation cho User Story 2

- [ ] T021 [P] [US2] `SnapshotBuffer`, `Interpolator` — file: `src/game/scripts/world/snapshot_buffer.gd`, `src/game/scripts/world/interpolator.gd`
- [ ] T022 [US2] `HubSync` (join, throttle move, snapshot, emote, corrected, migrated, lỗi join) — file: `src/game/scripts/multiplayer/hub_sync.gd`
- [ ] T023 [US2] `RemotePlayer.tscn` + `RemotePlayerSync` (pool ≤ `hub_instance_capacity`, `AvatarBuilder`, nickname label) — file: `src/game/scenes/character/RemotePlayer.tscn`, `src/game/scripts/world/remote_player_sync.gd`

## Phase 5: User Story 3 — Hub đông người chạy mượt trên máy tầm trung (Priority: P2)

### Tests cho User Story 3

- [ ] T024 [P] [US3] Benchmark headless F092-AC-21: 40 RemotePlayer giả lập, kiểm số nhân vật đầy đủ theo `quality_presets`, LOD animation > 30 m, frame time CPU trong ngân sách (TC-PERF-018) — file: `src/game/tests/perf/test_hub40_headless.gd` — phải FAIL trước khi code
- [ ] T025 [US3] Lab F092-AC-20, F092-AC-22, F092-AC-23: 39 bot + client thật trên DEV-AND-MID1/DEV-PC-MID/DEV-WEB-MID 10 phút; soak 60 phút (TC-PERF-009, TC-PERF-023, TC-PERF-021) — file: `src/game/tests/perf/lab_hub40_protocol.md` (quy trình đo + bảng kết quả)

### Implementation cho User Story 3

- [ ] T026 [US3] LOD/impostor RemotePlayer theo `QualityManager.quality_changed`, giảm tần suất animation ngoài 30 m — file: `src/game/scripts/world/remote_player_sync.gd`, `src/game/scenes/character/RemotePlayer.tscn`

## Phase 6: Polish & Cross-Cutting

- [ ] T027 [P] Cập nhật GAME_NETWORKING §4 (heartbeat REST chỉ khi WS chưa Online) nếu chưa nêu; CHANGELOG — file: `docs/08-game-client/GAME_NETWORKING.md`, `CHANGELOG.md`
- [ ] T028 [P] Thu `NetStats` (RTT p50/p95, reconnect, resume) vào `AppState` cho F080 — file: `src/game/scripts/core/app_state.gd`
- [ ] T029 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T030 **OQ chốt 2026-11-20** (pinning) `RealtimeClient` (`WebSocketPeer`) dùng `TLSOptions.client(trusted_chain)` với CA bundle chung; test chứng chỉ ngoài bundle bị từ chối — file: `src/game/scripts/networking/realtime_client.gd`, `src/game/tests/unit/networking/` — kiểm: `make godot-test` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → Polish. US1 dùng được fixture AsyncAPI (gắn nhãn MOCK) trước khi F090/F091 Done; T020, T025 cần backend thật.

## Definition of Done

- [ ] Mọi tiêu chí F092-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test`, `make godot-export-check` xanh; integration 2 client với backend thật xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
