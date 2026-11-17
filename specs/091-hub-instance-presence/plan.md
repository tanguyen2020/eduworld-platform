# Implementation Plan: F091 — Hub instance, presence, đồng bộ vị trí

**Branch**: `091-hub-instance-presence` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Feature đầu tiên của module competition: dựng skeleton module, projection `ref.*` cần cho hub, và **room host** `hub_instance` trên `pkg/roomhost` (mới, dùng lại cho lobby/battle/co-op/boss). Lệnh `hub.instance.join`/`hub.player.emote` đến qua NATS request `ew.cmd.competition.hub.*`; input `hub.player.move` qua NATS core `ew.in.room.<room_id>`. Command `JoinHubInstance` chạy thuật toán gán instance ([MULTIPLAYER_GAMEPLAY §6](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md)) trên registry Valkey `ew:competition:rt:area:*:instances` + lease `ew:competition:rt:room:<id>:lease`, ghi `multiplayer.rooms`/`session_memberships` trong `eduworld_competition`. Instance giữ lease chạy tick `hub_sync_rate`: validate tốc độ/biên, interest grid 16 m, delta snapshot gom theo gateway node, publish `ew:rt:room:<id>:out`. realtime thêm module presence (Valkey `ew:gateway:presence:*`, NATS core `ew.rt.presence.<tenant_id>`).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `pkg/natsx`, `pkg/outbox`, `pkg/realtime` (F090), `pkg/roomhost` (mới), `valkey-go` |
| Storage | PostgreSQL `eduworld_competition` (`multiplayer.rooms`, `multiplayer.session_memberships`, `competition.integrity_signals`, `ref.*`, `infra.*`); Valkey registry/lease/presence (tạm) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); bot Go WS cho TC-PERF-003/023 |
| Target Platform | Linux container (module `competition` api + worker; `realtime`) |
| Project Type | backend module |
| Performance Goals | Tick ≤ 5 ms/tick hub 40 người, p99 < 20 ms ([REALTIME_ARCHITECTURE §8](../../docs/06-architecture/REALTIME_ARCHITECTURE.md)); `hub_sync_bandwidth` |
| Constraints | AGENTS R4, R8, R9, R13, R16, R21, R26, R27, R28, R29 · ADR-0004, ADR-0011, ADR-0012, ADR-0074, ADR-0037, ADR-0042, ADR-0054 |
| Scale/Scope | ~175 instance hub ở 10.000 CCU, 4–6 replica `core-api` chạy room host của module competition (handler NATS `ew.cmd.competition.*` và subscriber `ew.in.room.*`, trạng thái phân phối qua Valkey; binary `realtime` chỉ kết thúc WebSocket, kiểm ticket, presence, định tuyến) ([REALTIME_ARCHITECTURE §11](../../docs/06-architecture/REALTIME_ARCHITECTURE.md)) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta AsyncAPI (`follow_user_id`, định tuyến), event payload, schema presence ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Room host ở module competition (research R1); dữ liệu area/bạn bè/chặn/hồ sơ qua projection từ event; không gọi đồng bộ trên đường vào khu |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `rooms`, `session_memberships` có `tenant_id` + RLS; registry key chứa tenant; test TC-TEN-012 |
| IV Server-authoritative | ✅ | Vị trí hợp lệ do server quyết định; không phần thưởng trong feature |
| V Xác thực, phân quyền | ✅ | Chỉ lệnh qua NATS account gateway; danh tính từ `Ew-*`; membership room kiểm mọi lệnh |
| VI An toàn trẻ em | ✅ | Partition trường + grade band; snapshot chỉ `PublicPlayer`; tránh xếp chung người chặn nhau |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số CONFIG_DEFAULTS; area runtime từ module game; `audit_hub_membership` đăng ký ở Polish |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mọi F091-AC-*, load test bot 40 người |

## Project Structure

### Documentation (feature này)

```text
specs/091-hub-instance-presence/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
modules/competition/
├── cmd/{api,worker,migrate}/main.go
├── ent/schema/          # room.go, sessionmembership.go, refworldarea.go, refareaunlock.go, reffriendship.go,
│                        # refblock.go, refplayerprofile.go, reftenant.go, refstudent.go
├── migrations/
├── internal/domain/hub/                 # HubInstance, Member, AssignmentPolicy, MovementValidator, InterestGrid, Partition
├── internal/application/command/        # JoinHubInstance, LeaveHubInstance, EmitEmote, CloseIdleHubInstances, RecordHubViolation
├── internal/application/port/           # RoomRegistry, RoomHostRuntime, RealtimePublisher, PresenceNotifier, AreaCatalog, SocialGraph, Clock
├── internal/infrastructure/roomhost/    # wiring pkg/roomhost cho hub: lease, subscribe ew.in.room.<id>, tick, packer theo node
├── internal/infrastructure/cache/       # keys.go, registry (area instances ZSET, room HASH)
├── internal/infrastructure/persistence/ # ent repo rooms, memberships, projection readers
├── internal/presentation/natscmd/       # hub_instance_join.go, hub_player_emote.go
├── internal/presentation/consumer/      # projection_identity.go, projection_world_areas.go, projection_area_unlocks.go,
│                                        # projection_social.go, projection_player_profiles.go, access_revoked.go
├── internal/presentation/job/close_idle_hub_instances.go
└── tests/{unit,integration,contract,load}/
modules/realtime/internal/presence/   # presence store, handler presence.status.set, control presence.update, NATS publisher
pkg/roomhost/                                  # Lease, Runtime (tick loop), Checkpoint (dùng ở F104/F105/F103)
```

**Structure Decision**: Room host trong module competition theo REALTIME_ARCHITECTURE §1, §5.1 và AGENTS §6 ([research R1](research.md)); presence online/offline ở gateway vì gateway giữ kết nối ([REALTIME_ARCHITECTURE §1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md)).

## Phase 0: Research

→ [research.md](research.md): R1 module sở hữu hub; R2 dữ liệu area runtime và mở khóa; R3 tín hiệu presence cho module khác; R4 đóng gói snapshot theo node.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `HubInstance` (Join, Leave, HoldSeat, Expire), `AssignmentPolicy` (partition, bạn bè, chặn, đông nhất < 90%), `MovementValidator` (tốc độ × 1,2, biên), `InterestGrid` (ô 16 m + 8 ô) | Thuần, Clock inject |
| application | `JoinHubInstance` (command, `UnitOfWork`, ghi `rooms`/`session_memberships` + outbox audit), `LeaveHubInstance`, `EmitEmote`, `CloseIdleHubInstances`, `RecordHubViolation` | Danh tính từ `Ew-*`; không permission catalog (route người chơi qua ticket học sinh) |
| infrastructure | Registry Valkey (Lua chọn/giữ chỗ), roomhost runtime, publisher `pkg/realtime`, projection readers | |
| presentation | NATS `ew.cmd.competition.hub.instance.join`, `ew.cmd.competition.hub.player.emote`, input `ew.in.room.<room_id>`, consumer projection, job đóng instance | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F090 (gateway, `pkg/realtime`), F070 (`game_profile.updated`), F071 (`content.published` (`item_type = world_area`), `world.area_unlocked`), F095 (`friend.*`, `user.blocked/unblocked`), F010/F011 (`tenant.*`, `student.synced`), F016 (`user.access_revoked`).
- Event phát: `audit.recorded` (vi phạm vượt ngưỡng). Realtime: `hub.instance.assigned`, `hub.snapshot.updated`, `hub.player.joined/left/emoted/appearance_changed/corrected`, `hub.instance.migrated`; control `presence.update` tới gateway qua `ew:rt:user:<id>`.

### Bảo mật, tenant, idempotency, audit

- Membership room kiểm mọi lệnh/input; room có `tenant_id` + `school_id` + `grade_band` khớp partition của người gửi.
- `JoinHubInstance` idempotent theo `(student_id, room_id)` membership active (lệnh trùng `id` gateway đã chặn).
- Audit `hub.violation_threshold_exceeded` (outbox `audit.recorded`), integrity signal `hub_speed`.

### Quan sát

- Metric: `eduworld_rooms_active{room_type}`, `eduworld_room_players{room_type}`, `eduworld_instance_assign_duration_seconds`, `eduworld_room_tick_duration_seconds`, `eduworld_join_friend_total{result}`, `eduworld_hub_violations_total{reason}`, `eduworld_projection_lag_seconds{table}`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
