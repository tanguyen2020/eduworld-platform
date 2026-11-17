# Tasks: F091 — Hub instance, presence, đồng bộ vị trí

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Skeleton module competition (`module.go` đăng ký vào `cmd/core-api` và `cmd/worker`, `api/`, `internal/*`, `migrations/`, `pkg/config`, outbox relay, rule depguard/arch-check, prefix Valkey `ew:competition:*` và quyền subscribe `ew.cmd.competition.>`, `ew.in.room.>` trong ACL của binary `core-api`) — file: `modules/competition/`
- [ ] T002 [P] Gói `pkg/roomhost`: `Lease` (SET NX/XX, watcher), `Runtime` (tick loop theo Clock, budget metric), hook checkpoint (rỗng cho hub) — file: `pkg/roomhost/`
- [ ] T003 [P] Cấu hình competition: `EW_HUB_TICK_HZ` (từ `hub_sync_rate`), `EW_HUB_INSTANCE_CAPACITY`, `EW_HUB_IDLE_CLOSE`, `EW_HUB_DISCONNECT_GRACE`, `EW_INSTANCE_ID` — file: `modules/competition/internal/config/hub.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta AsyncAPI: `follow_user_id` trong `HubInstanceJoin`, định tuyến `hub.*` → competition, `x-routing`, enum presence chung theo MULTIPLAYER_GAMEPLAY §5 + cờ `hidden` — file: `specs/asyncapi/eduworld-realtime.v1.yaml`, `specs/schemas/common/enums.schema.json` — kiểm: `make specs-lint`
- [ ] T005 [P] Xác nhận payload tiêu thụ đã có: khối `world_area` trong `content.published` của game (owner F071), `world.area_unlocked` (owner F071), `game_profile.updated` có `avatar_config` (owner F070); thiếu → đề xuất delta cho owner và chặn consumer tương ứng, không tự tạo — file: `specs/events/payloads/game/` (chỉ đọc)
- [ ] T006 [P] Schema `presence-changed.schema.json`; action `presence.update` trong `rt-pubsub-envelope` — file: `specs/schemas/realtime/` — kiểm: `make specs-lint`
- [ ] T007 ent schema `Room` (+`grade_band`), `SessionMembership` (+cột §10), `RefWorldArea`, `RefAreaUnlock`, `RefFriendship`, `RefBlock`, `RefPlayerProfile` (+`avatar_config`), `RefTenant`, `RefStudent` với `MaxLen` — file: `modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f091_hub`, `make migrate-lint MOD=competition`
- [ ] T008 Migration: index partition, unique membership active, RLS bảng T, `REVOKE DELETE, TRUNCATE`, `infra.*` — file: `modules/competition/migrations/`
- [ ] T009 [P] Consumer projection identity (`tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up/restored`) + `content.published` (`item_type = world_area`), `world.area_unlocked`, `game_profile.updated`, `friend.*`, `user.blocked/unblocked` (áp `source_version` mới hơn, idempotent `processed_events`, TC-SVC-003) — file: `modules/competition/internal/presentation/consumer/`
- [ ] T010 [P] Test RLS + tenant isolation bảng T của competition (không set tenant → 0 dòng; DELETE bị từ chối, TC-TEN-005, TC-TEN-016) — file: `modules/competition/tests/integration/rls_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Vào khu và được gán hub instance đúng phạm vi (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T011 [P] [US1] Unit test `AssignmentPolicy` F091-AC-02, F091-AC-03, F091-AC-04: không trộn partition, ưu tiên bạn bè → đông nhất < 90% → tạo mới; tránh người chặn; đầy → instance mới — file: `modules/competition/tests/unit/hub_assignment_policy_test.go` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Integration F091-AC-01, F091-AC-06: join trả `hub.instance.assigned` đủ trường + `subscribe`; area lạ/không cho khối/chưa mở khóa → `NOT_FOUND`/`PERMISSION_DENIED`; chuỗi quá `maxLength` → `VALIDATION_FAILED` — file: `modules/competition/tests/integration/hub_join_test.go`
- [ ] T013 [P] [US1] Integration F091-AC-05: `follow_user_id` bạn bè cùng partition → cùng room; không phải bạn/đã chặn/khác partition/instance đầy → lỗi, không lộ `room_id` — file: `modules/competition/tests/integration/hub_join_friend_test.go`
- [ ] T014 [P] [US1] Test authorization + tenant isolation F091-AC-07, F091-AC-08: lệnh vào room tenant khác/không là thành viên → `ROOM_NOT_MEMBER`; message `ew.cmd.competition.hub.*` từ user NATS khác `realtime` (binary `realtime`) bị bỏ qua (TC-TEN-012) — file: `modules/competition/tests/integration/hub_authz_test.go`
- [ ] T015 [P] [US1] Integration F091-AC-09, F091-AC-10, F091-AC-11: giữ chỗ trong grace, về instance cũ, hết grace → `hub.player.left`; instance rỗng quá idle → `closed`; `session_memberships` chỉ ghi khi bật cờ tenant — file: `modules/competition/tests/integration/hub_lifecycle_test.go`

### Implementation cho User Story 1

- [ ] T016 [P] [US1] Domain `hub`: `HubInstance`, `Member`, `Partition`, `AssignmentPolicy` — file: `modules/competition/internal/domain/hub/`
- [ ] T017 [US1] Registry Valkey + script Lua chọn/giữ chỗ — file: `modules/competition/internal/infrastructure/cache/hub_registry.go`, `modules/competition/internal/infrastructure/cache/scripts/hub_assign.lua`
- [ ] T018 [US1] Command `JoinHubInstance`, `LeaveHubInstance`, `CloseIdleHubInstances` + job — file: `modules/competition/internal/application/command/join_hub_instance.go`, `modules/competition/internal/presentation/job/close_idle_hub_instances.go`
- [ ] T019 [US1] NATS handler `hub.instance.join` (danh tính từ `Ew-*`, reply `subscribe`) + consumer `user.access_revoked` gỡ khỏi instance — file: `modules/competition/internal/presentation/natscmd/hub_instance_join.go`, `modules/competition/internal/presentation/consumer/access_revoked.go`

**Checkpoint**: vào khu được gán instance đúng phạm vi.

## Phase 4: User Story 2 — Đồng bộ vị trí 10 Hz có kiểm tra phía server (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Unit F091-AC-12, F091-AC-13: `MovementValidator` (Δt, hệ số 1,2, biên), clamp + `hub.player.corrected`, > 20 vi phạm/phút → integrity signal + audit — file: `modules/competition/tests/unit/hub_movement_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Unit F091-AC-14, F091-AC-15: `InterestGrid` 16 m + 8 ô; delta + `removed`; payload chỉ `eid`/`PublicPlayer`, không tên thật/trường/lớp — file: `modules/competition/tests/unit/hub_interest_grid_test.go`
- [ ] T022 [P] [US2] Integration F091-AC-17, F091-AC-18, F091-AC-19, F091-AC-20: không route REST chuyển động (quét OpenAPI); emote không sở hữu/vượt `emote_rate` bị từ chối; `game_profile.updated` → `hub.player.appearance_changed`; kill owner → `hub.instance.migrated` — file: `modules/competition/tests/integration/hub_runtime_test.go`
- [ ] T023 [P] [US2] Load test F091-AC-16: 40 bot × 10 Hz trong 10 phút, đo tick p95 và băng thông xuống/người (TC-PERF-003, TC-PERF-023) — file: `modules/competition/tests/load/hub40_test.go`

### Implementation cho User Story 2

- [ ] T024 [P] [US2] Domain `MovementValidator`, `InterestGrid` — file: `modules/competition/internal/domain/hub/movement.go`, `modules/competition/internal/domain/hub/interest_grid.go`
- [ ] T025 [US2] Room host hub: subscribe `ew.in.room.<id>`, tick `hub_sync_rate`, packer theo gateway node (research R4), publish `ew:rt:room:<id>:out` droppable — file: `modules/competition/internal/infrastructure/roomhost/hub_runtime.go`
- [ ] T026 [US2] Command `EmitEmote`, `RecordHubViolation` + NATS handler `hub.player.emote` + consumer appearance — file: `modules/competition/internal/application/command/emit_emote.go`, `modules/competition/internal/presentation/natscmd/hub_player_emote.go`

## Phase 5: User Story 3 — Presence tạm thời (Priority: P2)

### Tests cho User Story 3

- [ ] T027 [P] [US3] Test F091-AC-21, F091-AC-22, F091-AC-24: connect → `online` + tín hiệu `ew.rt.presence.<tenant>`; `presence.status.set` hợp lệ/ngoài enum; 4408 → `away` → `offline` sau `ws_timeout` (Clock giả) — file: `modules/realtime/tests/integration/presence_test.go` — phải FAIL trước khi code
- [ ] T028 [P] [US3] Test F091-AC-23, F091-AC-25: join/leave hub → control `presence.update` cập nhật `area_id`/`room_id`; không có message/endpoint nào trả presence cho client trong F091; không bảng PG presence — file: `modules/competition/tests/integration/hub_presence_update_test.go`

### Implementation cho User Story 3

- [ ] T029 [US3] Module presence gateway (store Valkey, handler `presence.status.set`, xử lý control `presence.update`, publisher NATS core) — file: `modules/realtime/internal/presence/`
- [ ] T030 [US3] `PresenceNotifier` ở competition (publish control qua `pkg/realtime`) — file: `modules/competition/internal/infrastructure/realtime/presence_notifier.go`

## Phase 6: Polish & Cross-Cutting

- [ ] T031 [P] Cập nhật tài liệu: CODE_PLAN dòng F091 (module = competition + realtime); LOCAL_DEVELOPMENT §4 chuyển `EW_RT_HUB_*` sang competition; DATABASE_DESIGN §2.9 (cột mới), §2.17 (`ref.world_areas`, `ref.area_unlocks`, `ref.friendships`, `ref.blocks`, `avatar_config`); VALKEY_DESIGN §3.1 (`held`, field `hidden`); EVENT_DESIGN §4.5 (`content.published` (`item_type = world_area`)), REALTIME_ARCHITECTURE §1 (subject `ew.rt.presence.*`); đăng ký `audit_hub_membership` trong CONFIG_DEFAULTS; CHANGELOG — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`, `docs/11-operations/LOCAL_DEVELOPMENT.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`, `docs/06-architecture/REALTIME_ARCHITECTURE.md`, `docs/00-overview/CONFIG_DEFAULTS.md`, `CHANGELOG.md`
- [ ] T032 [P] Metric `eduworld_rooms_active`, `eduworld_room_players`, `eduworld_instance_assign_duration_seconds`, `eduworld_room_tick_duration_seconds`, `eduworld_join_friend_total`, `eduworld_hub_violations_total` + alert tick p99 > 20 ms — file: `modules/competition/internal/infrastructure/observability/metrics.go`, `deploy/observability/prometheus/alerts/competition.yml`
- [ ] T033 [P] Contract test consumer với schema event game/chat/identity — file: `modules/competition/tests/contract/events_test.go`
- [ ] T034 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T009 projection trước mọi story) → US1 → US2 → US3 → Polish. US3 phần gateway (T027, T029) song song được với US1.
- Cần F090 Done (gateway, `pkg/realtime`); dữ liệu area thật cần F071 hoặc seed phát `content.published` (`item_type = world_area`).

## Definition of Done

- [ ] Mọi tiêu chí F091-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `competition` và `realtime`; load test Hub40 đạt ngân sách
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
