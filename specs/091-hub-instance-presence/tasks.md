# Tasks: F091 — Hub instance, presence, đồng bộ vị trí

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Skeleton module competition (`module.go` đăng ký vào `src/backend/cmd/core-api` và `src/backend/cmd/worker`, `api/`, `internal/*`, `migrations/`, `src/backend/pkg/config`, outbox relay, rule depguard/arch-check, prefix Valkey `ew:competition:*` và quyền subscribe `ew.cmd.competition.>`, `ew.in.room.>` trong ACL của binary `core-api`) — file: `src/backend/modules/competition/` — **Bằng chứng (2026-11-21):** module.go đăng ký core-api (room host, 2 lệnh NATS) và worker (relay, 3 consumer, job); `TestWorkerProfilesRegisterRelaysConsumersAndJobs` pass (competition thêm vào `workerModules`); prefix Valkey `ew:competition:*`; ACL NATS/Valkey theo user vẫn là OQ F090-2
- [x] T002 [P] Gói `src/backend/pkg/roomhost`: `Lease` (SET NX/XX, watcher), `Runtime` (tick loop theo Clock, budget metric), hook checkpoint (rỗng cho hub) — file: `src/backend/pkg/roomhost/` — **Bằng chứng (2026-11-21):** `src/backend/pkg/roomhost` (`Tick`, `KeepLease`) — `go test ./pkg/roomhost` 2/2 pass
- [x] T003 [P] Cấu hình competition: `EW_HUB_TICK_HZ` (từ `hub_sync_rate`), `EW_HUB_INSTANCE_CAPACITY`, `EW_HUB_IDLE_CLOSE`, `EW_HUB_DISCONNECT_GRACE`, `EW_INSTANCE_ID` — file: `src/backend/modules/competition/internal/config/hub.go` — **Bằng chứng (2026-11-21):** `internal/infrastructure/config/hub.go` (`EW_HUB_TICK_HZ`, `EW_HUB_INSTANCE_CAPACITY`, `EW_HUB_IDLE_CLOSE`, `EW_HUB_DISCONNECT_GRACE`, `EW_INSTANCE_ID`)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta AsyncAPI: `follow_user_id` trong `HubInstanceJoin`, định tuyến `hub.*` → competition, `x-routing`, enum presence chung theo MULTIPLAYER_GAMEPLAY §5 + cờ `hidden` — file: `specs/asyncapi/eduworld-realtime.v1.yaml`, `specs/schemas/common/enums.schema.json` — kiểm: `make specs-lint` — **Bằng chứng (2026-11-21):** `follow_user_id`, `PresenceStatus` (enum chung + `enums.schema.json`, `check-enums` thêm tên), `hidden`; `PlayerCard.presence` cùng enum; `gen-routes`, `check-enums`, `check-maxlength` 0 lỗi
- [x] T005 [P] Xác nhận payload tiêu thụ đã có: khối `world_area` trong `content.published` của game (owner F071), `world.area_unlocked` (owner F071), `game_profile.updated` có `avatar_config` (owner F070); thiếu → đề xuất delta cho owner và chặn consumer tương ứng, không tự tạo — file: `specs/events/payloads/game/` (chỉ đọc) — **Bằng chứng (2026-11-21):** đã kiểm: `game_profile.updated` có `avatar_config` (dùng); `content.published` (`world_area`), `world.area_unlocked` (F071), `friend.*`, `user.blocked/unblocked` (F095) **chưa có** → không tự tạo, consumer tương ứng chặn (spec A-1)
- [x] T006 [P] Schema `presence-changed.schema.json`; action `presence.update` trong `rt-pubsub-envelope` — file: `specs/schemas/realtime/` — kiểm: `make specs-lint` — **Bằng chứng (2026-11-21):** `presence-changed.schema.json`; envelope thêm `kind = bundle` và control `presence.update`; `validate-schemas` 0 lỗi, `TestEnvelopeMatchesSchema` pass
- [x] T007 ent schema `Room` (+`grade_band`), `SessionMembership` (+cột §10), `RefWorldArea`, `RefAreaUnlock`, `RefFriendship`, `RefBlock`, `RefPlayerProfile` (+`avatar_config`), `RefTenant`, `RefStudent` với `MaxLen` — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f091_hub`, `make migrate-lint MOD=competition` — **Bằng chứng (2026-11-21):** ent `Room`, `SessionMembership`; `TestMigrationMatchesEnt` pass
- [x] T008 Migration: index partition, unique membership active, RLS bảng T, `REVOKE DELETE, TRUNCATE`, `infra.*` — file: `src/backend/modules/competition/migrations/` — **Bằng chứng (2026-11-21):** migration `20261225000001_f091_hub.sql` (ref.*, RLS, trigger leave-once, integrity_signals append-only, `_live`), `atlas.sum` cập nhật
- [x] T009 [P] Consumer projection identity (`tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up/restored`) + `content.published` (`item_type = world_area`), `world.area_unlocked`, `game_profile.updated`, `friend.*`, `user.blocked/unblocked` (áp `source_version` mới hơn, idempotent `processed_events`, TC-SVC-003) — file: `src/backend/modules/competition/internal/presentation/consumer/` — **Bằng chứng (2026-11-21):** `competition.projection_identity`, `competition.projection_game`, `competition.hub_access_revoked`; consumer world_area/unlock/friend/block chờ payload (A-1); `TestConsumersAcceptOwnerExamples` pass
- [x] T010 [P] Test RLS + tenant isolation bảng T của competition (không set tenant → 0 dòng; DELETE bị từ chối, TC-TEN-005, TC-TEN-016) — file: `src/backend/modules/competition/tests/integration/rls_test.go` — **Bằng chứng (2026-11-21):** `TestRLSAndNoDelete` pass (file `hub_presence_update_test.go`)

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Vào khu và được gán hub instance đúng phạm vi (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T011 [P] [US1] Unit test `AssignmentPolicy` F091-AC-02, F091-AC-03, F091-AC-04: không trộn partition, ưu tiên bạn bè → đông nhất < 90% → tạo mới; tránh người chặn; đầy → instance mới — file: `src/backend/modules/competition/tests/unit/hub_assignment_policy_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-21):** `TestChoose`, `TestFollow`, `TestPartitionAndCapacity` pass
- [x] T012 [P] [US1] Integration F091-AC-01, F091-AC-06: join trả `hub.instance.assigned` đủ trường + `subscribe`; area lạ/không cho khối/chưa mở khóa → `NOT_FOUND`/`PERMISSION_DENIED`; chuỗi quá `maxLength` → `VALIDATION_FAILED` — file: `src/backend/modules/competition/tests/integration/hub_join_test.go` — **Bằng chứng (2026-11-21):** `TestHubJoin`, `TestHubPartitionAndCapacity` pass
- [x] T013 [P] [US1] Integration F091-AC-05: `follow_user_id` bạn bè cùng partition → cùng room; không phải bạn/đã chặn/khác partition/instance đầy → lỗi, không lộ `room_id` — file: `src/backend/modules/competition/tests/integration/hub_join_friend_test.go` — **Bằng chứng (2026-11-21):** `TestHubJoinFriend` pass
- [x] T014 [P] [US1] Test authorization + tenant isolation F091-AC-07, F091-AC-08: lệnh vào room tenant khác/không là thành viên → `ROOM_NOT_MEMBER`; message `ew.cmd.competition.hub.*` từ user NATS khác `realtime` (binary `realtime`) bị bỏ qua (TC-TEN-012) — file: `src/backend/modules/competition/tests/integration/hub_authz_test.go` — **Bằng chứng (2026-11-21):** `TestHubAuthorization` pass (phần quyền NATS theo account: OQ F090-2)
- [x] T015 [P] [US1] Integration F091-AC-09, F091-AC-10, F091-AC-11: giữ chỗ trong grace, về instance cũ, hết grace → `hub.player.left`; instance rỗng quá idle → `closed`; `session_memberships` chỉ ghi khi bật cờ tenant — file: `src/backend/modules/competition/tests/integration/hub_lifecycle_test.go` — **Bằng chứng (2026-11-21):** `TestHubLifecycle` pass

### Implementation cho User Story 1

- [x] T016 [P] [US1] Domain `hub`: `HubInstance`, `Member`, `Partition`, `AssignmentPolicy` — file: `src/backend/modules/competition/internal/domain/hub/` — **Bằng chứng (2026-11-21):** `internal/domain/hub/{area,assignment,instance}.go`
- [x] T017 [US1] Registry Valkey + script Lua chọn/giữ chỗ — file: `src/backend/modules/competition/internal/infrastructure/cache/hub_registry.go`, `src/backend/modules/competition/internal/infrastructure/cache/scripts/hub_assign.lua` — **Bằng chứng (2026-11-21):** registry Valkey `internal/infrastructure/cache/hub_registry.go` (Lua lease/renew); chọn/giữ chỗ tuần tự ở owner room (single writer) thay script Lua chọn chỗ — unique membership PG vẫn là chốt chặn
- [x] T018 [US1] Command `JoinHubInstance`, `LeaveHubInstance`, `CloseIdleHubInstances` + job — file: `src/backend/modules/competition/internal/application/command/join_hub_instance.go`, `src/backend/modules/competition/internal/presentation/job/close_idle_hub_instances.go` — **Bằng chứng (2026-11-21):** `command.Hub` (`JoinHubInstance`, `MemberRemoved`, `RoomClosed`, `CloseOrphanRooms`) + job `close-idle-hub-instances`
- [x] T019 [US1] NATS handler `hub.instance.join` (danh tính từ `Ew-*`, reply `subscribe`) + consumer `user.access_revoked` gỡ khỏi instance — file: `src/backend/modules/competition/internal/presentation/natscmd/hub_instance_join.go`, `src/backend/modules/competition/internal/presentation/consumer/access_revoked.go` — **Bằng chứng (2026-11-21):** `natscmd/hub.go` + consumer `competition.hub_access_revoked`

**Checkpoint**: vào khu được gán instance đúng phạm vi.

## Phase 4: User Story 2 — Đồng bộ vị trí 10 Hz có kiểm tra phía server (Priority: P1)

### Tests cho User Story 2

- [x] T020 [P] [US2] Unit F091-AC-12, F091-AC-13: `MovementValidator` (Δt, hệ số 1,2, biên), clamp + `hub.player.corrected`, > 20 vi phạm/phút → integrity signal + audit — file: `src/backend/modules/competition/tests/unit/hub_movement_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-21):** `TestMovementValidator`, `TestEmote` pass
- [x] T021 [P] [US2] Unit F091-AC-14, F091-AC-15: `InterestGrid` 16 m + 8 ô; delta + `removed`; payload chỉ `eid`/`PublicPlayer`, không tên thật/trường/lớp — file: `src/backend/modules/competition/tests/unit/hub_interest_grid_test.go` — **Bằng chứng (2026-11-21):** `TestInterestGrid`, `TestTickAndPayloadPrivacy` pass
- [x] T022 [P] [US2] Integration F091-AC-17, F091-AC-18, F091-AC-19, F091-AC-20: không route REST chuyển động (quét OpenAPI); emote không sở hữu/vượt `emote_rate` bị từ chối; `game_profile.updated` → `hub.player.appearance_changed`; kill owner → `hub.instance.migrated` — file: `src/backend/modules/competition/tests/integration/hub_runtime_test.go` — **Bằng chứng (2026-11-21):** `TestHubRuntime`, `TestNoRESTMovementAndNoValkeySimulation`, `TestHubFailover` pass
- [x] T023 [P] [US2] Load test F091-AC-16: 40 bot × 10 Hz trong 10 phút, đo tick p95 và băng thông xuống/người (TC-PERF-003, TC-PERF-023) — file: `src/backend/modules/competition/tests/load/hub40_test.go` — **Bằng chứng (2026-11-21):** `TestHub40` (`-tags load`): 600 s mô phỏng, tick p95 1,50 ms / p99 1,61 ms (≤ 5 ms), người xem nặng nhất 5 597 B/s (≤ 30 KB/s), 0 hiệu chỉnh sai — lượt WebSocket đầu-cuối thuộc harness load (A-8)

### Implementation cho User Story 2

- [x] T024 [P] [US2] Domain `MovementValidator`, `InterestGrid` — file: `src/backend/modules/competition/internal/domain/hub/movement.go`, `src/backend/modules/competition/internal/domain/hub/interest_grid.go` — **Bằng chứng (2026-11-21):** `movement.go`, `interest.go`
- [x] T025 [US2] Room host hub: subscribe `ew.in.room.<id>`, tick `hub_sync_rate`, packer theo gateway node (research R4), publish `ew:rt:room:<id>:out` droppable — file: `src/backend/modules/competition/internal/infrastructure/roomhost/hub_runtime.go` — **Bằng chứng (2026-11-21):** `internal/infrastructure/roomhost/host.go` (lease, input, tick, bundle theo node, failover watcher, handover)
- [x] T026 [US2] Command `EmitEmote`, `RecordHubViolation` + NATS handler `hub.player.emote` + consumer appearance — file: `src/backend/modules/competition/internal/application/command/emit_emote.go`, `src/backend/modules/competition/internal/presentation/natscmd/hub_player_emote.go` — **Bằng chứng (2026-11-21):** `EmitEmote`, `Violation` (integrity signal + audit outbox), `ProfileChanged` + consumer appearance

## Phase 5: User Story 3 — Presence tạm thời (Priority: P2)

### Tests cho User Story 3

- [x] T027 [P] [US3] Test F091-AC-21, F091-AC-22, F091-AC-24: connect → `online` + tín hiệu `ew.rt.presence.<tenant>`; `presence.status.set` hợp lệ/ngoài enum; 4408 → `away` → `offline` sau `ws_timeout` (Clock giả) — file: `src/backend/modules/realtime/tests/integration/presence_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-21):** `TestPresence`, `TestRoomSnapshotBundle` pass
- [x] T028 [P] [US3] Test F091-AC-23, F091-AC-25: join/leave hub → control `presence.update` cập nhật `area_id`/`room_id`; không có message/endpoint nào trả presence cho client trong F091; không bảng PG presence — file: `src/backend/modules/competition/tests/integration/hub_presence_update_test.go` — **Bằng chứng (2026-11-21):** `TestHubPresenceUpdate` pass

### Implementation cho User Story 3

- [x] T029 [US3] Module presence gateway (store Valkey, handler `presence.status.set`, xử lý control `presence.update`, publisher NATS core) — file: `src/backend/modules/realtime/internal/presence/` — **Bằng chứng (2026-11-21):** `modules/realtime/internal/presence` + session/manager
- [x] T030 [US3] `PresenceNotifier` ở competition (publish control qua `src/backend/pkg/realtime`) — file: `src/backend/modules/competition/internal/infrastructure/realtime/presence_notifier.go` — **Bằng chứng (2026-11-21):** `internal/infrastructure/realtime/presence.go`

## Phase 6: Polish & Cross-Cutting

- [x] T031 [P] Cập nhật tài liệu: CODE_PLAN dòng F091 (module = competition + realtime); LOCAL_DEVELOPMENT §4 chuyển `EW_RT_HUB_*` sang competition; DATABASE_DESIGN §2.9 (cột mới), §2.17 (`ref.world_areas`, `ref.area_unlocks`, `ref.friendships`, `ref.blocks`, `avatar_config`); VALKEY_DESIGN §3.1 (`held`, field `hidden`); EVENT_DESIGN §4.5 (`content.published` (`item_type = world_area`)), REALTIME_ARCHITECTURE §1 (subject `ew.rt.presence.*`); đăng ký `audit_hub_membership` trong CONFIG_DEFAULTS; CHANGELOG — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`, `docs/11-operations/LOCAL_DEVELOPMENT.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`, `docs/06-architecture/REALTIME_ARCHITECTURE.md`, `docs/00-overview/CONFIG_DEFAULTS.md`, `CHANGELOG.md` — **Bằng chứng (2026-11-21):** LOCAL_DEVELOPMENT §4, DATABASE_DESIGN §2.9/§2.17, VALKEY_DESIGN §3.1, REALTIME_ARCHITECTURE §1/§5.1, CODE_PLAN, CHANGELOG; `audit_hub_membership` đã có trong CONFIG_DEFAULTS; EVENT_DESIGN `world_area` để F071
- [x] T032 [P] Metric `eduworld_rooms_active`, `eduworld_room_players`, `eduworld_instance_assign_duration_seconds`, `eduworld_room_tick_duration_seconds`, `eduworld_join_friend_total`, `eduworld_hub_violations_total` + alert tick p99 > 20 ms — file: `src/backend/modules/competition/internal/infrastructure/observability/metrics.go`, `deploy/observability/prometheus/alerts/competition.yml` — **Bằng chứng (2026-11-21):** `internal/infrastructure/observability/metrics.go` + `deploy/observability/prometheus/alerts/competition.yml` (rule_files)
- [x] T033 [P] Contract test consumer với schema event game/chat/identity — file: `src/backend/modules/competition/tests/contract/events_test.go` — **Bằng chứng (2026-11-21):** `tests/contract/events_test.go` pass
- [ ] T034 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — một phần: kịch bản 1–15 có test tự động tương ứng (integration + load); chạy tay trên stack `make up` chờ seed area của F071

## Nhận từ F071 (owner chốt 2026-11-21)

- [ ] T035 Consumer competition chiếu `content.published` (khối `world_area`) vào `ref.world_areas` và `world.area_unlocked` vào `ref.area_unlocks` (schema payload đã có ở `specs/events/payloads/game/`, F071); room host dùng area thật từ seed thay cho dữ liệu tạm — kiểm: `make test-integration MOD=competition`, `make test-contract` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational (T009 projection trước mọi story) → US1 → US2 → US3 → Polish. US3 phần gateway (T027, T029) song song được với US1.
- Cần F090 Done (gateway, `src/backend/pkg/realtime`); dữ liệu area thật cần F071 hoặc seed phát `content.published` (`item_type = world_area`).

## Definition of Done

- [x] Mọi tiêu chí F091-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `competition` và `realtime`; load test Hub40 đạt ngân sách
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
