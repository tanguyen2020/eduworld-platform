# Implementation Plan: F107 — Tournament: 6 format, bracket, lịch, thử thách lớp/giải trường từ template

**Branch**: `107-tournament-challenges` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `tournament` và `challenge` trong module `competition`. `CreateChallenge` (route `/lms/challenges`, chuyển sang module competition) kiểm tra template đã publish + gán (`infra.tenant_resource_assignments`), lọc `params` theo `allowed_params`, ghim `template_version_id` (bảng `game_config_versions` của F104/F108) và tạo `Tournament` hoặc thử thách Timed Quiz. Thuật toán bracket (single/double elim, round robin, vòng bảng, seeding) là hàm thuần trong `domain/tournament/bracket`. Job `tournament-scheduler` chuyển trạng thái theo thời gian, check-in, gọi `CreateDirectProposal` (F104) cho nút tới giờ, xử no-show. Consumer `match.completed` đẩy người thắng (idempotent theo `match_id`). Hoàn thành giải → `tournament_results`, Competition Points `source_type = tournament` (port của F106), `tournament_rewards` + outbox `tournament.rewarded`. Bên nhận internal `assignTenantResources` cho template.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go, go-redis (lock singleton job), `pkg/entx` (`UnitOfWork`), `pkg/authz`, `pkg/tenancy`, `pkg/outbox`, `pkg/idempotency`, `pkg/clock` |
| Storage | PostgreSQL `eduworld_competition` (`competition`, `infra`, `ref`) |
| Testing | go test (property test bracket N = 2..64), testcontainers (PostgreSQL, NATS, Valkey), đồng hồ giả cho lịch/check-in/no-show |
| Target Platform | Linux container (module `competition` nạp trong `core-api` + `worker-events`, `worker-jobs`) |
| Project Type | backend module |
| Performance Goals | Sinh bracket 64 entry < 1 s; job lịch chạy mỗi 15 s, trễ tạo trận ≤ 30 s |
| Constraints | AGENTS R7–R10, R15, R16, R19–R23, R26–R29 · ADR-0004, ADR-0020, ADR-0027, ADR-0029, ADR-0072 |
| Scale/Scope | `max_entries` ≤ 256 ([COMPETITION_SYSTEM §10.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#102-mô-hình)); cả trường cho School Tournament |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta (`x-service` `/lms/challenges`, endpoint tournament, `listMyChallenges`, `joinChallenge`, `getChallenge`, `cancelChallenge`, internal assignment, payload event) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module competition; identity gọi facade assignment `competition` (adapter của F045); thưởng vật phẩm qua event cho module game |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới T có `tenant_id` + RLS + soft-delete; giải liên tenant (G) chỉ mô hình, không API tenant |
| IV Server-authoritative | ✅ | Kết quả nút chỉ từ `match.completed`; override chỉ người tổ chức có lý do + audit; thưởng idempotent |
| V Xác thực, phân quyền | ✅ | `lms.challenge.write` (chỉ `school_admin` trong role tenant) + scope trường; học sinh qua `game_player` |
| VI An toàn trẻ em | ✅ | Chỉ nickname với học sinh; không tắt chat trận; thưởng cosmetic, không pay-to-win |
| VII Một nguồn sự thật, cấu hình | ✅ | Luật/thưởng từ template version; thời hạn theo `tournament_timers` |
| VIII UI tách logic, token | ✅ | Không có UI (F108) |
| IX Test có bằng chứng | ✅ | Mỗi `F107-AC-*` có task test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/107-tournament-challenges/
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
├── ent/schema/challenge.go, tournament.go, tournament_entry.go, tournament_group.go,
│   bracket_node.go, tournament_result.go, tournament_reward.go
├── internal/domain/
│   ├── challenge/challenge.go, params.go          # lọc allowed_params, ghim template version
│   └── tournament/
│       ├── tournament.go                          # vòng đời, invariant
│       ├── entry.go, eligibility.go
│       ├── bracket/single_elim.go, double_elim.go, round_robin.go, groups.go, seeding.go, standings.go
│       └── advance.go, forfeit.go, override.go
├── internal/application/
│   ├── command/challenge/   # create_challenge.go, cancel_challenge.go, join_challenge.go
│   ├── command/tournament/  # publish.go, cancel.go, register_entry.go, withdraw_entry.go, confirm_entries.go, check_in.go,
│   │                        # generate_bracket.go, schedule_due_nodes.go, apply_no_show.go, advance_on_match.go,
│   │                        # confirm_node.go, override_node.go, complete_tournament.go, award_rewards.go, transition_by_time.go
│   ├── command/assignment/  # assign_template.go, unassign_template.go
│   ├── query/               # list_challenges.go, get_challenge.go, list_challenge_templates.go, list_my_challenges.go,
│   │                        # list_tournaments.go, get_tournament.go, get_bracket.go, list_assignments.go
│   └── port/                # TemplateReader, AssignmentRepo, DirectProposalCreator (F104), TimedQuizRoomJoiner (F105), RatingReader (F106), PointsWriter (F106)
├── internal/presentation/
│   ├── http/public/challenge_handler.go, tournament_handler.go
│   ├── http/internal/assignment_handler.go
│   ├── consumer/match_completed_tournament.go
│   └── job/tournament_scheduler.go
├── seed/tournament_templates.json
└── tests/{unit,integration,contract}/tournament/
```

**Structure Decision**: Thuật toán bracket thuần (property test) tách khỏi lịch/trận; Challenge là aggregate riêng vì Class Challenge Timed Quiz không có bracket.

## Phase 0: Research

Không có. Thuật toán và luật đã định ở [COMPETITION_SYSTEM §10](../../docs/01-game-design/COMPETITION_SYSTEM.md#10-tournament); ánh xạ losers bracket dùng bảng chuẩn double elimination (ghi trong code + test).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `Challenge`, `ParamFilter`, `Tournament` (state machine), `Eligibility`, bracket builders, `Seeding`, `Standings`, `Advance`, `Forfeit`, `Override` | Invariant: không đổi luật ngoài template; override chỉ trước khi nút kế bắt đầu |
| application | Command liệt kê ở cây trên · Query `ListChallenges`, `GetChallenge`, `ListChallengeTemplates`, `ListMyChallenges`, `ListTournaments`, `GetTournament`, `GetTournamentBracket`, `ListTenantResourceAssignments` | Command (`command/`, `UnitOfWork`) / query (`query/`, chỉ đọc) tách, gọi qua interface có kiểu |
| infrastructure | ent repo; lock singleton job (Valkey); outbox | `FOR UPDATE SKIP LOCKED` cho nút tới giờ |
| presentation | Handler operationId ở contracts §1–§2; consumer `match.completed`; job `tournament-scheduler` (15 s) | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F104 (`CreateDirectProposal`), F105 (`JoinTimedQuizRoom`, luật trận giải, `match.completed`), F106 (`RatingReader`, ghi points `tournament`), F045 (bên gọi assignment), F010 (`ref.classes`, `ref.enrollments`).
- Facade: không gọi (bên nhận: hiện thực facade assignment).
- Event phát: `tournament.match_scheduled`, `tournament.completed`, `tournament.rewarded`, `audit.recorded`. Tiêu thụ: `match.completed`, `student.cleaned_up`.

### Bảo mật, tenant, idempotency, audit

- Người tổ chức: `lms.challenge.write`, scope trường của UserRole; System Admin cần tenant đích.
- Học sinh: `game_player`, phạm vi giải (lớp/trường/tenant) từ `ref.enrollments`/`ref.students`.
- Internal: `x-allowed-callers: [identity]`.
- `Idempotency-Key`: `createChallenge`, `registerTournamentEntry`, `confirmTournamentEntries`, `generateTournamentBracket`, `overrideBracketNode`, `joinChallenge`, `assignTenantResources`.
- Audit: `challenge.created/cancelled`, `tournament.published/cancelled/bracket_generated/node_overridden/rewarded`, `tenant_resource.assigned/unassigned`.

### Quan sát

- Metric: `eduworld_tournament_matches_scheduled_total`, `eduworld_tournament_forfeits_total{reason}`, `eduworld_tournament_overrides_total`, `eduworld_tournament_scheduler_lag_seconds`.
- Span: `CreateChallenge`, `GenerateBracket`, `ScheduleDueNodes`, `AdvanceOnMatch`, `AwardRewards`.

Ước lượng: 2–3 sprint (backend 2 dev) [Proposed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
