# Tasks: F101 — Party Lobby, Start Session, leader/thành viên mất kết nối

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Adapter facade `game` (`listPartyActivityCandidates`, `checkPartyActivityEligibility`, `startPartyActivity`) qua `src/backend/pkg/modcall` (`Idempotency-Key` cho `startPartyActivity`) — file: `src/backend/modules/competition/internal/infrastructure/modules/game.go`
- [ ] T002 [P] Key builder `ew:party:<pid>:lock`, `ew:party:<pid>:leader_grace`, `ew:party:active`; đọc `ew:presence:user:<id>` — file: `src/backend/modules/competition/internal/infrastructure/cache/keys.go`
- [ ] T003 [P] Đăng ký handler NATS `ew.cmd.competition.party.*` qua `src/backend/pkg/natsx` (nạp trong `core-api`) — file: `src/backend/modules/competition/module.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Thêm `PartyActivityType` vào enum chung; `$ref` từ OpenAPI/AsyncAPI — file: `specs/schemas/common/enums.schema.json` — kiểm: `make specs-lint`
- [ ] T005 Delta OpenAPI: `expected_version`, requestBody + `409 ineligible_members` + `503` cho `startPartySession`, `listPartyEligibleActivities`, `PARTY_NOT_READY`, `409/429` cho `setPartyReady`; sinh file theo module — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`
- [ ] T006 [P] Delta AsyncAPI: `expected_version` cho `party.activity.select`, `party.session.start`; `room_type` thêm `boss`; `ActivityRef.kind` — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T007 [P] Ba operation `game.internal` (schema theo F103 contracts) + facade test phía bên gọi (fake facade) — file: `specs/openapi/modules/game.internal.v1.yaml`, `src/backend/modules/competition/tests/contract/game_party_activities_test.go` — kiểm: `make specs-lint`, `make test-contract`
- [ ] T008 [P] Payload schema `party.activity_selected`, `party.activity_started`, `party.activity_ended`, `party.activity_completed` — file: `specs/events/payloads/competition/` — kiểm: `make specs-lint`
- [ ] T009 ent schema `PartyActivity`, `Room`, `SessionMembership` (cột mới, `MaxLen`) — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f101_lobby_session`, `make migrate-lint MOD=competition`
- [ ] T010 Migration: unique một activity `running`/party, unique membership mở/phòng, RLS, CHECK enum — file: `src/backend/modules/competition/migrations/`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Party Lobby: sẵn sàng, chọn hoạt động (Priority: P1) 🎯 MVP

**Goal**: Lobby realtime với Ready và chọn hoạt động.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T011 [P] [US1] Test F101-AC-01, F101-AC-02: Ready qua REST và NATS cập nhật Valkey, fan-out `party.member.ready_changed` (`all_ready`), không outbox; party không `open` → `409`; người ngoài → `404`; > tần suất → `429` — file: `src/backend/modules/competition/tests/integration/lobby_ready_test.go` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Test F101-AC-03, F101-AC-04: chọn hoạt động reset Ready, `state_version++`, `party.activity.selected` có `ineligible_members`; không phải leader → `403`; version lệch → `409 VERSION_MISMATCH` — file: `src/backend/modules/competition/tests/integration/select_activity_test.go`
- [ ] T013 [P] [US1] Test F101-AC-05 (module game stub MOCK): đủ/một phần điều kiện, mã lý do không lộ khối/lớp — file: `src/backend/modules/competition/tests/integration/eligible_activities_test.go`
- [ ] T014 [P] [US1] Test F101-AC-06: snapshot đầy đủ; xóa Valkey → dựng lại từ PG, Ready = false — file: `src/backend/modules/competition/tests/integration/lobby_snapshot_rebuild_test.go`
- [ ] T015 [P] [US1] E2E đo p95 Ready ≤ 1 s với 4 client WS (TC-PTY-004) — file: `tests/e2e/party_lobby_latency_test.go`

### Implementation cho User Story 1

- [ ] T016 [P] [US1] Domain `LobbyState`, `ReadyRule` — file: `src/backend/modules/competition/internal/domain/party/lobby.go`
- [ ] T017 [US1] Command `SetReady`, `SelectActivity`; query `ListEligibleActivities`, `GetLobbySnapshot`; `LobbyStore` Valkey — file: `src/backend/modules/competition/internal/application/command/set_ready.go`, `src/backend/modules/competition/internal/application/command/select_activity.go`, `src/backend/modules/competition/internal/application/query/list_eligible_activities.go`, `src/backend/modules/competition/internal/infrastructure/cache/lobby_store.go`
- [ ] T018 [US1] Handler REST `setPartyReady`, `selectPartyActivity`, `listPartyEligibleActivities` và NATS `party.member.ready_set`, `party.activity.select` — file: `src/backend/modules/competition/internal/presentation/http/public/lobby.go`, `src/backend/modules/competition/internal/presentation/natscmd/party.go`

**Checkpoint**: lobby chạy độc lập.

## Phase 4: User Story 2 — Bắt đầu và kết thúc phiên chơi chung (Priority: P1)

### Tests cho User Story 2

- [ ] T019 [P] [US2] Test F101-AC-07, F101-AC-09: start hợp lệ (activity, membership, `in_activity`, outbox, `party.session.started` cùng `room_id`); không phải leader → `403`; chưa Ready → `409 PARTY_NOT_READY`; thiếu/lặp `Idempotency-Key` — file: `src/backend/modules/competition/tests/integration/start_party_session_test.go` — phải FAIL trước khi code
- [ ] T020 [P] [US2] Test F101-AC-08: thành viên không đủ điều kiện → `409 PARTY_ACTIVITY_NOT_ELIGIBLE` + lý do, không ghi gì — file: `src/backend/modules/competition/tests/integration/start_ineligible_test.go`
- [ ] T021 [P] [US2] Test F101-AC-10: `explore` cấp `private_instance`; module game lỗi/timeout → `503`, party `open`; retry cùng key → cùng phòng — file: `src/backend/modules/competition/tests/integration/start_game_dependency_test.go`
- [ ] T022 [P] [US2] Test F101-AC-11: người không thuộc membership vào phòng → `ROOM_NOT_MEMBER` — file: `src/backend/modules/competition/tests/integration/room_membership_check_test.go`
- [ ] T023 [P] [US2] Test F101-AC-12, F101-AC-13: kết thúc theo 5 lý do (consumer `boss.encounter_completed`/`quest.party_completed` idempotent — TC-SVC-003), party về `open`, `party.activity_completed` khi completed; `result_summary`/`party.session.ended` không có khóa điểm/attempt/mastery — file: `src/backend/modules/competition/tests/integration/end_party_activity_test.go`

### Implementation cho User Story 2

- [ ] T024 [P] [US2] Domain `PartyActivity`, `EndReason` — file: `src/backend/modules/competition/internal/domain/party/activity.go`
- [ ] T025 [US2] Command `StartPartySession` (R2: lock, version, eligibility, `startPartyActivity`, commit, publish) — file: `src/backend/modules/competition/internal/application/command/start_party_session.go`
- [ ] T026 [US2] `RoomAllocator` `private_instance` + kiểm membership phòng — file: `src/backend/modules/competition/internal/infrastructure/room/allocator.go`
- [ ] T027 [US2] Command `EndPartyActivity` + consumer `party_activity_end` — file: `src/backend/modules/competition/internal/application/command/end_party_activity.go`, `src/backend/modules/competition/internal/presentation/consumer/party_activity_end.go`
- [ ] T028 [US2] Handler `startPartySession` REST + NATS `party.session.start` — file: `src/backend/modules/competition/internal/presentation/http/public/lobby.go`, `src/backend/modules/competition/internal/presentation/natscmd/party.go`

## Phase 5: User Story 3 — Mất kết nối: thành viên và leader (Priority: P1)

### Tests cho User Story 3

- [ ] T029 [P] [US3] Test F101-AC-14: presence hết hạn → `disconnected` trong một chu kỳ, `party.member.status_changed`, Ready reset ở lobby — file: `src/backend/modules/competition/tests/integration/presence_sweeper_test.go` — phải FAIL trước khi code
- [ ] T030 [P] [US3] Test F101-AC-15, F101-AC-16 (Clock giả): leader quay lại trong grace giữ vai trò; hết grace → leader mới đúng thứ tự + audit; dormant → leader là người online lại đầu tiên; quá `party_dormant_timeout` → `disbanded` `inactivity` — file: `src/backend/modules/competition/tests/integration/leader_grace_dormant_test.go`
- [ ] T031 [P] [US3] Test F101-AC-17, F101-AC-18: trong phiên `explore`/`quest` giữ chỗ trong `disconnect_grace`, quay lại (thiết bị khác) về đúng `room_id`; hết grace đóng membership `disconnected`; dưới tối thiểu → `timeout` — file: `src/backend/modules/competition/tests/integration/in_activity_disconnect_test.go`
- [ ] T032 [P] [US3] E2E ngắt kết nối thành viên và leader 2 client headless (TC-PTY-008, TC-PTY-009) — file: `tests/e2e/party_disconnect_test.go`

### Implementation cho User Story 3

- [ ] T033 [P] [US3] Domain `LeaderGrace` (chọn leader mới) — file: `src/backend/modules/competition/internal/domain/party/leader_grace.go`
- [ ] T034 [US3] Command `MarkPresence`, `AutoTransferLeader`, `DisbandDormantParty` + job `party-presence-sweeper` (khóa `ew:lock:job:party-presence-sweeper`) — file: `src/backend/modules/competition/internal/application/command/mark_presence.go`, `src/backend/modules/competition/internal/presentation/job/presence_sweeper.go`

## Phase 6: User Story 4 — Phân quyền và cô lập tenant (Priority: P1)

### Tests cho User Story 4

- [ ] T035 [P] [US4] Test tenant isolation + authorization F101-AC-19, F101-AC-20: 4 operation × token nhân sự → `403 GAME_ACCESS_STUDENT_ONLY`; REST/NATS với party tenant khác hoặc không là thành viên → `404`/`PERMISSION_DENIED`; gọi facade `game` mang `TenantScope` của party trong context — file: `src/backend/modules/competition/tests/integration/lobby_authz_tenant_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T036 [US4] Middleware Principal cho NATS command (header gateway, kiểm tenant + membership) — file: `src/backend/modules/competition/internal/presentation/natscmd/principal.go`

## Phase 7: Polish & Cross-Cutting

- [ ] T037 [P] Đăng ký `party_activity_min_members_timeout`, `party_lobby_cmd_rate` (CONFIG_DEFAULTS + PARTY_SYSTEM §9, §10); sửa API_DESIGN §5 (409), VALKEY_DESIGN §3.1 (key mới thay `ew:pty:*`), DATABASE_DESIGN §2.9 (cột mới), PRD AC-PTY-008 (tên message `party.leader.transferred`); CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/03-social-multiplayer/PARTY_SYSTEM.md`, `docs/07-backend/API_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T038 [P] Metric `eduworld_party_lobby_update_seconds`, `eduworld_party_session_start_total`, `eduworld_party_leader_auto_transfer_total`, `eduworld_party_activity_end_total` — file: `src/backend/modules/competition/internal/infrastructure/observability/metrics.go`
- [ ] T039 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3, US4 song song → Polish.
- `quest`/`boss` của US2 cần F103 hiện thực T007; trước đó test dùng stub contract gắn nhãn MOCK (R17). `explore` cần room host F091.

## Definition of Done

- [ ] Mọi tiêu chí F101-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` xanh cho module `competition`; `make test-contract`, `make test-e2e` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
