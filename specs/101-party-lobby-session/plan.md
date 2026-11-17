# Implementation Plan: F101 — Party Lobby, Start Session, leader/thành viên mất kết nối

**Branch**: `101-party-lobby-session` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Mở rộng bounded context `party` của module competition (F100): lobby state trong Valkey (`ew:party:<pid>`, `ew:party:<pid>:members`), lệnh lobby qua REST và NATS request `ew.cmd.competition.party.*` từ realtime; command `StartPartySession` theo luồng 9 bước (research R2) với module game internal cho `quest`/`boss` (research R1) và room allocator nội bộ cho `explore`; `EndPartyActivity` khi nhận event từ module game hoặc khi worker phát hiện thiếu người. Worker `party-presence-sweeper` (research R3) xử lý presence, grace leader, dormant. Bảng `party_activities`, `rooms`, `session_memberships` của `eduworld_competition`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server `competition.v1.yaml`), facade `game` (`modules/game/api`), `pkg/entx` (`UnitOfWork`), `pkg/outbox`, `pkg/idempotency`, `pkg/modcall`, `pkg/natsx` (`HandleCommand`), Valkey client |
| Storage | PostgreSQL `eduworld_competition` (`multiplayer.party_activities`, `rooms`, `session_memberships`; bảng F100); Valkey `ew:party:*`, `ew:presence:user:*` (chỉ đọc), `ew:rt:user:<id>` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); facade test với module game (fake facade phía competition); E2E WS 3 client headless |
| Target Platform | Linux container (module `competition` nạp trong `core-api` + `worker-events`) |
| Project Type | backend module |
| Performance Goals | Lobby ≤ 1 s p95 (AC-PTY-003); `nats_request_timeout` cho lệnh WS |
| Constraints | AGENTS R5, R8, R9, R10, R13, R16, R23, R26, R29; ADR-0004, ADR-0012, ADR-0016, ADR-0031, ADR-0040, ADR-0074, ADR-0042 |
| Scale/Scope | Party ≤ 8; `private_instance_capacity` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta OpenAPI/AsyncAPI/game.internal/event ở Phase 2 trước code |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Điều kiện quest/boss qua facade `game`; kết thúc phiên qua event; không đọc DB game |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới có `tenant_id` + RLS + `deleted_at`; test isolation REST + WS |
| IV Server-authoritative | ✅ | Leader/Ready/số lượng kiểm ở server; không lưu điểm; `Idempotency-Key` cho start; audit chuyển leader tự động |
| V Xác thực, phân quyền | ✅ | `game_player` REST; lệnh WS chỉ qua user NATS của binary `realtime`; gọi facade `game` mang TenantScope + Principal gốc trong context |
| VI An toàn trẻ em | ✅ | Không lộ khối/lớp/điểm của bạn; lý do không đủ điều kiện dạng mã |
| VII Một nguồn sự thật, cấu hình | ✅ | Enum `PartyActivityType` chung; tham số theo tên CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi F101-AC có task test; stub module game gắn nhãn MOCK chỉ trong test |

## Project Structure

### Documentation (feature này)

```text
specs/101-party-lobby-session/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
modules/competition/
├── ent/schema/            # partyactivity.go, room.go, sessionmembership.go
├── migrations/
├── internal/domain/party/                   # lobby.go (LobbyState, ReadyRule), activity.go (PartyActivity, EndReason), leader_grace.go
├── internal/application/command/            # set_ready.go, select_activity.go, start_party_session.go, end_party_activity.go,
│                                            # mark_presence.go, auto_transfer_leader.go, disband_dormant.go
├── internal/application/query/              # list_eligible_activities.go, get_lobby_snapshot.go
├── internal/application/port/               # GameActivityPort, RoomAllocator, PresenceReader, LobbyStore
├── internal/infrastructure/client/gameclient/        # listPartyActivityCandidates, checkPartyActivityEligibility, startPartyActivity
├── internal/infrastructure/cache/lobby_store.go      # ew:party:<pid>, :members, :lock, :leader_grace, ew:party:active
├── internal/infrastructure/room/allocator.go         # private_instance (ew:rt:room:<id>, rooms)
├── internal/presentation/http/public/lobby.go        # setPartyReady, selectPartyActivity, startPartySession, listPartyEligibleActivities
├── internal/presentation/natscmd/party.go            # ew.cmd.competition.party.member.ready_set|activity.select|session.start
├── internal/presentation/consumer/party_activity_end.go   # boss.encounter_completed, quest.party_completed
├── internal/presentation/job/presence_sweeper.go
└── tests/{unit,integration,contract,e2e}/
```

**Structure Decision**: cùng package `party` với F100 (cùng aggregate Party); presentation thêm `natscmd/` cho lệnh WS theo BACKEND_DESIGN §9.

## Phase 0: Research

Xem [research.md](research.md): R1 facade module game, R2 thứ tự gọi/commit, R3 presence sweeper, R4 enum chung.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `LobbyState` (SetReady, SelectActivity → reset ready, CanStart), `PartyActivity` (Start, End(reason)), `LeaderGrace` (chọn leader mới = online gia nhập sớm nhất) | Thuần; Clock inject |
| application | `SetReady`, `SelectActivity` (`expected_version`), `StartPartySession` (R2), `EndPartyActivity`, `MarkPresence`, `AutoTransferLeader`, `DisbandDormantParty`; query `ListEligibleActivities`, `GetLobbySnapshot` | Kiểm leader/thành viên trong handler |
| infrastructure | adapter facade `game`, `LobbyStore` Valkey, `RoomAllocator` (`ew:rt:room:<id>`, bảng `rooms`), `PresenceReader` | |
| presentation | REST `setPartyReady`, `selectPartyActivity`, `startPartySession`, `listPartyEligibleActivities`; NATS `party.member.ready_set`, `party.activity.select`, `party.session.start`; consumer kết thúc phiên; job `party-presence-sweeper` | Reply NATS theo REALTIME §4.1 |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F100 (party, fan-out), F014 (`game_session_id` cho membership), F090 (presence, lệnh WS), F091 (room host `private_instance`), F103 (provider game.internal, event kết thúc).
- Facade: `listPartyActivityCandidates`, `checkPartyActivityEligibility`, `startPartyActivity` (game.internal).
- Event phát: `party.activity_selected`, `party.activity_started`, `party.activity_ended`, `party.activity_completed`, `party.leader_transferred` (reason `leader_disconnected`), `party.disbanded` (reason `inactivity`), `audit.recorded`.
- Event tiêu thụ: `boss.encounter_completed`, `quest.party_completed` (game); `party.disbanded` nội bộ để kết thúc activity.

### Bảo mật, tenant, idempotency, audit

- `game_player` REST; lệnh WS dựng Principal từ header NATS của gateway; `TenantScope` + membership kiểm tra trong command.
- `Idempotency-Key` bắt buộc `startPartySession`; chuyển tiếp key tới module game bằng `party_activity_id`.
- Audit: `party.leader_transferred` (auto), `party.disbanded` (inactivity), `party.session_start_denied` (không đủ điều kiện).

### Quan sát

- Metric `eduworld_party_lobby_update_seconds`, `eduworld_party_session_start_total{activity_type,result}`, `eduworld_party_leader_auto_transfer_total`, `eduworld_party_activity_end_total{reason}`, span/metric gọi facade `game` (`pkg/modcall`); span `StartPartySession` gồm lời gọi module game.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
