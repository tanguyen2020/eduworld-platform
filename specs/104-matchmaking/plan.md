# Implementation Plan: F104 — Matchmaking 8 chế độ: queue, ticket, proposal, ready check, ràng buộc khối

**Branch**: `104-matchmaking` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `matchmaking` trong module `competition`. API nhận ticket (REST, `Idempotency-Key`), kiểm tra H1–H7 bằng projection `ref.*` + bảng hạn chế, ghi ticket/queue vào Valkey (ADR-0004). Worker `matchmaker` lặp theo `EW_MM_TICK`, khóa per-queue, ghép greedy theo cost và cửa sổ rating, tạo proposal, nhận accept/decline qua NATS request `ew.cmd.competition.matchmaking.*` (từ realtime) hoặc REST fallback. Khi đủ accept, command `CreateMatch` ghi `competition.matches` + `match_participants` + outbox `match.created` trong một transaction (CQRS, ADR-0038/0042), `match_id = proposal_id`. Friend duel là aggregate `Duel` trong PostgreSQL. Cấu hình matchmaking đọc từ bảng cấu hình phiên bản hóa (seed) qua port `ConfigReader`; rating đọc qua port `RatingReader` (mặc định `glicko2_params` cho tới F106).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go (JetStream + core request/reply), go-redis (Valkey), `pkg/authz`, `pkg/tenancy`, `pkg/entx`, `pkg/outbox`, `pkg/idempotency`, `pkg/natsx`, `pkg/clock`, `pkg/ids` |
| Storage | PostgreSQL `eduworld_competition` (schema `competition`, `ref`, `infra`); Valkey `ew:competition:mm:*` (ephemeral) |
| Testing | go test + testcontainers (PostgreSQL, Valkey, NATS); đồng hồ giả `pkg/clock`; k6 WS cho TC-PERF-004 |
| Target Platform | Linux container (module `competition` nạp trong `core-api` + `worker-events`, `worker-jobs` cho matchmaker) |
| Project Type | backend module |
| Performance Goals | NFR-PERF-007; vòng matchmaker < 1 s ([MATCHMAKING §10](../../docs/03-social-multiplayer/MATCHMAKING.md#10-metrics--alert)) |
| Constraints | AGENTS R5, R7, R8, R9, R10, R13, R14, R16, R20, R23, R26–R29 · ADR-0004, ADR-0027, ADR-0031, ADR-0037–ADR-0042, ADR-0074, ADR-0054 |
| Scale/Scope | 20% CCU vào queue (TC-PERF-004); `mm_tickets_per_user`, `mm_ticket_rate` ([CONFIG_DEFAULTS §10](../../docs/00-overview/CONFIG_DEFAULTS.md)) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec.md Ready; delta contract (`declineMatchProposal`, `listCompetitionModes`, `startBotMatch`, field ticket, payload event) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module `competition`; dữ liệu module khác qua projection `ref.*` (event); không gọi đồng bộ module khác trên đường tạo ticket |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Khóa queue chứa tenant/trường; bảng mới có `tenant_id` + soft-delete/RLS; `matches` (G*) chỉ truy cập qua `match_participants` |
| IV Server-authoritative | ✅ | Khối/rating/trường do server lấy; client chỉ tạo/hủy ticket, accept/decline; tạo trận idempotent theo `proposal_id` |
| V Xác thực, phân quyền | ✅ | Middleware `game_player` mọi route; chủ ticket/duel; leader party |
| VI An toàn trẻ em | ✅ | Proposal/lobby chỉ nickname/avatar/tier; block H6; không label PII trong metric |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS, giá trị từ cấu hình đã publish; key Valkey theo VALKEY_DESIGN |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi `F104-AC-*` có task test viết trước; test tenant isolation + authorization mọi endpoint |

## Project Structure

### Documentation (feature này)

```text
specs/104-matchmaking/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
modules/competition/
├── ent/schema/
│   ├── match.go, match_participant.go       # mới (cột theo data-model)
│   ├── duel.go, competition_restriction.go  # mới
│   ├── game_config.go, game_config_version.go, content_release.go  # mới (đọc ở F104, ghi ở F108)
│   └── ref_enrollment.go, ref_school.go (thêm settings_subset), ref_block.go (nếu F095/F100 chưa có)
├── internal/domain/matchmaking/
│   ├── constraints.go      # H1–H7, QueueKey (mode, scope, grade_key, subject, topic)
│   ├── cost.go, window.go  # soft scoring, expanding window
│   ├── pairing.go          # greedy theo thời gian chờ, team snake draft, bucket khi > 500
│   ├── ticket.go, proposal.go
│   └── duel/duel.go        # aggregate Duel
├── internal/application/
│   ├── command/matchmaking/  # create_ticket.go, cancel_ticket.go, accept_proposal.go, decline_proposal.go, run_matchmaker_tick.go, create_match.go, create_direct_proposal.go, start_bot_match.go, expire_tickets.go
│   ├── command/duel/         # create_duel.go, accept_duel.go, decline_duel.go, expire_duels.go
│   ├── query/matchmaking/    # get_ticket.go, list_competition_modes.go
│   └── port/                 # TicketStore, QueueStore, ProposalStore, RatingReader, ConfigReader, EligibilityReader (ref.*), RestrictionReader, BlockReader, RealtimePublisher
├── internal/infrastructure/
│   ├── cache/mm_store.go     # Valkey: queue ZSET, ticket HASH, player NX, proposal HASH, lock
│   ├── persistence/          # ent repo match, duel, restriction, config; SQL đọc ref.*
│   └── messaging/            # outbox writer; realtime publisher PUBLISH ew:rt:user:<id>
├── internal/presentation/
│   ├── http/public/matchmaking_handler.go, duel_handler.go
│   ├── realtime/matchmaking_cmd.go   # NATS ew.cmd.competition.matchmaking.{ticket.cancel,proposal.accept,proposal.decline}
│   ├── consumer/             # party_member_left (hủy ticket), user_blocked, student/enrollment/tenant/school projection
│   └── job/matchmaker.go, ticket_expiry.go, duel_expiry.go
├── seed/matchmaking_config.json
└── tests/{unit,integration,contract}/matchmaking/
```

**Structure Decision**: Matchmaking, battle, ranking, tournament cùng module `competition` (ADR-0074). Domain `matchmaking` thuần (không Valkey) để unit test H1–H7 và cost; Valkey là adapter sau port `QueueStore`.

## Phase 0: Research

Không có. Thuật toán (hard constraint → soft scoring, expanding window, greedy, lock per-queue) đã định ở [MATCHMAKING §2–§6](../../docs/03-social-multiplayer/MATCHMAKING.md#2-mô-hình-hai-tầng-hard-constraints--soft-scoring).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `QueueKey`, `HardConstraints` (H1–H7), `Cost`, `Window(t)`, `Pairing`, `TeamDraft`, `Proposal` (deadline, accept flags), `Duel` | Invariant: không nới H1–H7; lớp 1–5 `grade_key = g<n>`; lớp 6–9 `b6_7`/`b8_9` |
| application | Command `CreateTicket`, `CancelTicket`, `AcceptProposal`, `DeclineProposal`, `RunMatchmakerTick`, `CreateMatch`, `CreateDirectProposal` (cho F107), `StartBotMatch`, `ExpireTickets`, `CreateDuel`, `AcceptDuel`, `DeclineDuel`, `ExpireDuels` · Query `GetTicket`, `ListCompetitionModes` | Command/query tách (ADR-0087); log/trace/idempotency ở middleware `pkg/httpx`, authz tường minh đầu handler, validation trong DTO/handler |
| infrastructure | Valkey store (Lua cho tạo proposal nguyên tử: ZREM + HSET status), ent repo, outbox, realtime publisher | Key `ew:competition:mm:*` |
| presentation | Handler `createMatchmakingTicket`, `getMatchmakingTicket`, `cancelMatchmakingTicket`, `acceptMatchProposal`, `declineMatchProposal`, `startBotMatch`, `listCompetitionModes`, `createDuel`, `acceptDuel`, `declineDuel`; NATS command handler; consumer; job | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F090 (realtime định tuyến `matchmaking.*`, push `ew:rt:user:<id>`), F100 (party `competition_team`, event `party.member_left`), F095 (`user.blocked`), F010/F011 (event `student.synced`, `enrollment.changed`, `school.updated`, `tenant.updated`), F014 (game session, middleware `game_player`).
- Facade: không gọi (R30: đường tạo ticket không gọi đồng bộ).
- Event phát: `match.created`, `duel.sent`, `duel.accepted`, `duel.declined`, `duel.expired`, `audit.recorded`. Tiêu thụ: `party.member_left`, `party.disbanded` (nội bộ module), `user.blocked`/`user.unblocked`, `student.synced`, `enrollment.changed`, `school.updated`, `tenant.updated`, `game_profile.updated`.
- Bên dùng: F105 nhận `match.created` (nội bộ, command `OpenMatchRoom`), F107 gọi `CreateDirectProposal`, F106 cung cấp `RatingReader` thật.

### Bảo mật, tenant, idempotency, audit

- Mọi route: `game_player`; owner check ticket (`student_ids` chứa principal), duel (challenger/opponent), proposal (participant).
- TenantScope từ principal; khóa queue `scope = s:<school_id>` | `t:<tenant_id>` | `p` (chỉ `CreateDirectProposal` của giải platform).
- `Idempotency-Key`: `createMatchmakingTicket`, `createDuel`, `acceptDuel`, `startBotMatch` (bảng `infra.idempotency_records`).
- Audit: `match.created`, `duel.sent/accepted/declined/expired`; ticket không audit (ephemeral).

### Quan sát

- Log: `ticket_id`, `queue_key` băm, không nickname/student_id ở INFO.
- Metric: `eduworld_mm_queue_size{mode,grade_key,subject}`, `eduworld_mm_wait_seconds{mode,result}`, `eduworld_mm_proposals_total{result}`, `eduworld_mm_rating_gap`, `eduworld_mm_matches_created_total{mode}`, `eduworld_mm_loop_duration_seconds`.
- Span: `CreateTicket`, `matchmaker.tick`, `CreateMatch`, `CreateDuel`.

Ước lượng: 2 sprint (backend 2 dev) [Proposed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
