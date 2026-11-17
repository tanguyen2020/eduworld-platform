# Implementation Plan: F103 — Boss battle nhiều phase và quest co-op (shared progress, bước `interdependent`)

**Branch**: `103-boss-coop-quest` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Trong module game: aggregate `BossEncounter` chạy trên `pkg/roomhost` (F076) với lệnh WS `ew.cmd.game.boss.*`, bộ câu hỏi và chấm điểm qua question plan của module learning (research R1); encounter solo qua REST, encounter party qua facade `startPartyActivity` do F101 gọi (R2). Aggregate `PartyQuestSession` giữ tiến độ chung (R4) cạnh `QuestProgress` của F072: bước khung chia sẻ, `collective_target`, phân vai và thẻ dữ kiện `interdependent` (R3). Kết thúc encounter/phiên chốt trong một transaction, gọi Reward Engine (F074) idempotent, phát `boss.encounter_completed`/`quest.party_completed` qua outbox cho module competition (F101) và reward/LA.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server `game.v1.yaml`; facade `modules/game/api` sinh từ `game.internal.v1.yaml`; gọi facade `learning`), `pkg/roomhost`, `pkg/entx` (`UnitOfWork`), `pkg/outbox`, `pkg/idempotency`, `pkg/modcall`, `pkg/natsx`, Valkey client |
| Storage | PostgreSQL `eduworld_game` (`game.boss_encounters`, `boss_encounter_members`, `boss_encounter_contributions`, `boss_phase_plans`, `boss_answer_results`, `party_quest_sessions`, `party_quest_contributions`, `quest_progress`); Valkey `ew:rt:room:<encounter_id>*`, `ew:game:clue:*`, `ew:rt:user:<id>` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); facade test phía bên nhận (bên gọi `competition`) và phía bên gọi (fake facade `learning`); E2E boss 2 client headless |
| Target Platform | Linux container (module `game` api + worker) |
| Project Type | backend module |
| Performance Goals | Cập nhật năng lượng ≤ 1 s p95; `nats_request_timeout` cho lệnh WS |
| Constraints | AGENTS R5, R6, R8, R9, R10, R13, R16, R19, R23, R26, R29, R31; ADR-0012, ADR-0016, ADR-0031, ADR-0043, ADR-0048, ADR-0049 |
| Scale/Scope | Encounter ≤ 8 người, ≤ 8 phase (`boss.schema.json`); bước `interdependent` theo `interdependent_roles` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta game public/internal, learning internal, AsyncAPI, event ở Phase 2 trước code |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chấm điểm/dữ kiện qua facade `learning`; party chỉ qua facade `game` + event; không đọc DB competition |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới T + RLS + `deleted_at`; test isolation REST, internal, WS |
| IV Server-authoritative | ✅ | Giờ server, module learning chấm, thưởng idempotent `(encounter_id, student_id)`, version ghim |
| V Xác thực, phân quyền | ✅ | `game_player` route người chơi; internal `x-allowed-callers: [competition]`; leader kiểm qua dữ liệu phiên |
| VI An toàn trẻ em | ✅ | Không lộ đúng/sai, xếp hạng; chỉ nickname; không phạt khi sai |
| VII Một nguồn sự thật, cấu hình | ✅ | Luật từ boss/quest version và CONFIG_DEFAULTS; enum `shared_progress_policy` theo QUEST_SYSTEM §6 |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi F103-AC có task test; stub learning chỉ trong test, gắn nhãn MOCK |

## Project Structure

### Documentation (feature này)

```text
specs/103-boss-coop-quest/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
modules/game/
├── ent/schema/        # bossencounter.go (cột mới), bossencountermember.go, bossencountercontribution.go,
│                      # bossphaseplan.go, bossanswerresult.go, partyquestsession.go, partyquestcontribution.go
├── migrations/
├── internal/domain/boss/            # encounter.go (state machine §3.3), phase.go (completion_rule), contribution.go, eligibility.go
├── internal/domain/partyquest/      # session.go, shared_progress.go (policy), roles.go (role_assignment, allow_multi_role), clues.go
├── internal/application/command/    # create_boss_encounter.go, start_party_activity.go, mark_encounter_ready.go, submit_boss_answer.go,
│                                    # advance_boss_phase.go, complete_boss_encounter.go, leave_boss_encounter.go,
│                                    # apply_shared_step.go, record_collective_contribution.go, assign_step_roles.go,
│                                    # share_step_clue.go, complete_party_quest.go, apply_party_membership_change.go
├── internal/application/query/      # list_party_activity_candidates.go, check_party_activity_eligibility.go,
│                                    # get_boss_encounter.go, get_step_clues.go
├── internal/application/port/       # LearningPlanClient, MaterialRenderClient, RoomHost, RewardEngine, RealtimePublisher, Clock
├── internal/infrastructure/client/learningclient/   # createQuestionPlan, scorePlanAnswer, getLearningMaterialRenders
├── internal/infrastructure/persistence/
├── internal/infrastructure/cache/{keys.go, clue_cache.go}
├── internal/presentation/http/public/{bosses.go, quest_clues.go}
├── internal/presentation/http/internal/party_activities.go
├── internal/presentation/natscmd/boss.go            # ew.cmd.game.boss.ready, ew.cmd.game.boss.answer
├── internal/presentation/consumer/{party_membership.go, boss_quest_step.go}   # party.*, boss.encounter_completed (bước boss)
└── tests/{unit,integration,contract,e2e}/
```

**Structure Decision**: hai package domain mới (`boss`, `partyquest`) cạnh `quest` (F072) và `challenge` (F076) trong module game theo BACKEND_DESIGN §3.1; consumer `activity.graded` của F072 mở rộng để gọi `RecordCollectiveContribution`.

## Phase 0: Research

Xem [research.md](research.md): R1 chấm qua question plan, R2 encounter party qua Party Lobby, R3 thẻ dữ kiện, R4 bảng tiến độ chung, R5 room host.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `BossEncounter` (Ready, StartPhase, RecordAnswer, EvaluatePhase, RetryPhase, End(outcome)), `PhaseRule` (`all`/`any_n`/`collective_count` + `min_contribution`), `BossEligibility`; `PartyQuestSession` (ApplySharedStep, AddContribution, AssignRoles, ReassignRole, ShareClue, Complete) | Thuần; Clock inject |
| application | Command/query ở cây trên; `SubmitBossAnswer` gọi `scorePlanAnswer` ngoài transaction rồi ghi kết quả idempotent; `CompleteBossEncounter`/`CompletePartyQuest` gọi `RewardEngine.Grant` trong transaction | `game_player`; facade: `x-allowed-callers: [competition]` (arch-check) |
| infrastructure | adapter facade `learning`, ent repo, roomhost adapter, Valkey clue cache, publisher `ew:rt:user`/`ew:rt:room:<id>:out` | |
| presentation | public `listBosses`, `createBossEncounter`, `getBossEncounter`, `leaveBossEncounter`, `getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles`; internal `listPartyActivityCandidates`, `checkPartyActivityEligibility`, `startPartyActivity`; NATS `boss.encounter.ready`, `boss.answer.submit`; consumer party/boss; timer roomhost | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F076 (`pkg/roomhost`, challenge), F072 (`QuestProgress`, consumer `activity.graded`), F074 (Reward Engine), F101 (gọi internal, tiêu thụ event), F030/F026 (learning activity, version).
- Event phát: `boss.encounter_started`, `boss.phase_completed`, `boss.encounter_completed`, `quest.party_completed`, `quest.completed`/`quest.step_completed` (F072), `audit.recorded`.
- Event tiêu thụ: `party.member_left`, `party.member_kicked`, `party.disbanded`, `party.leader_transferred` (competition); `activity.graded` (learning).

### Bảo mật, tenant, idempotency, audit

- `game_player` cho route người chơi; facade party-activity chỉ module `competition` được import (`x-allowed-callers`, `make arch-check`); kiểm TenantScope trong context khớp thành viên (`ref.students`).
- `Idempotency-Key`: `createBossEncounter`, `leaveBossEncounter`, `shareQuestStepClue`, `assignQuestStepRoles`, `startPartyActivity`; câu trả lời idempotent theo `(plan_id, index, student_id)`; thưởng theo `(encounter_id, student_id)` / `(quest_progress_id, rule_id)`.
- Audit: `boss.encounter_completed` (thưởng), `boss.integrity_flagged` (trả lời quá nhanh).

### Quan sát

- Metric `eduworld_boss_encounters_active`, `eduworld_boss_answer_total{result}`, `eduworld_boss_phase_retry_total`, `eduworld_boss_encounter_end_total{outcome}`, `eduworld_party_quest_step_shared_total{policy}`, span/metric gọi facade `learning` (`pkg/modcall`); log không có đáp án/nội dung câu trả lời (R14).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
