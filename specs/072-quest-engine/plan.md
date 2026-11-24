# Implementation Plan: F072 — Quest Engine: 12 loại quest, bước, tiến độ qua event, lọc theo khối

**Branch**: `072-quest-engine` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `quest` của module game: quest là item nội dung có version trên lõi phát hành F071 (`item_type = quest`, payload `quest.schema.json` v2, cột tách để lọc). `QuestAvailability` (domain thuần) lọc theo khối, gán tenant, availability, prerequisites, learning requirements (`ref.mastery_summary`), môn bật. Aggregate `QuestProgress` giữ `step_states`; `StepHandler` registry (F072: `talk_npc`, `go_to`, `collect`, `discover`, `activity`). Bước học tập gọi `createAttempt` của module learning qua facade `learning` (`src/backend/pkg/modcall`); consumer `game.quest_on_activity_graded` đánh giá `pass_rule`; consumer `game.quest_on_discovery_found`; consumer `game.periodic_quests_on_session_started` cấp daily/weekly; job `quest-period-expiry`, `quest-failure-timer`. Mọi đổi trạng thái ghi outbox (`quest.*`) và publish Valkey `ew:rt:user:<id>` (`quest.progress.updated`). module learning thêm consumer ghi learning event `quest.completed`. Godot: `QuestService`, Quest Journal (S05), Quest Tracker, `QuestMarker3D`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 · GDScript (Godot 4.x) |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server + client `learning.internal`), `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, `src/backend/pkg/idempotency`, `src/backend/pkg/modcall`, nats.go, Valkey client, JSON Schema validator · GdUnit4 |
| Storage | PostgreSQL `eduworld_game` (`game.quest_definitions`, `game.quest_versions`, `game.quest_progress`, `game.quest_step_attempts`, `game.periodic_quest_grants`, `infra.tenant_resource_assignments`); `eduworld_learning` (learning events — F031) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); facade test với module learning (`createAttempt`); GdUnit4 |
| Target Platform | Linux container (`core-api` + `worker`) · Android, iOS, Web (ADR-0101) |
| Project Type | backend module + game client |
| Performance Goals | `listQuests` p95 ≤ 300 ms; cập nhật tiến độ sau nộp ≤ 2 s p95; lời gọi `createAttempt` p95 < 50 ms (TC-PERF-013) |
| Constraints | AGENTS R2, R5, R8, R10, R13, R19, R26, R29, R31; ADR-0016, ADR-0019, ADR-0029, ADR-0040, ADR-0042, ADR-0043, ADR-0048, ADR-0056, ADR-0059 |
| Scale/Scope | `max_active_quests`/học sinh; 10.000 CCU (luồng quest 70% hub) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Sửa `QuestType`, `QuestProgressStatus`, `QuestDetail`, ProblemCode, `quest.schema.json`, `ActivityRef`; payload `quest.*` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | learning chỉ qua `createAttempt` (facade `learning`) và event `activity.graded`; learning tự ghi learning event từ `quest.completed` |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `quest_progress` T + RLS; quest platform qua gán tenant; test isolation |
| IV Server-authoritative | ✅ | Bước học tập chỉ hoàn thành từ kết quả chấm; `Idempotency-Key` cho accept/abandon/report/start; audit `quest.completed` |
| V Xác thực, phân quyền | ✅ | `game_player`; gọi facade `learning` chỉ vì `game` có trong `x-allowed-callers` (`make arch-check`) |
| VI An toàn trẻ em | ✅ | Không streak phạt, không phạt thất bại; không PII trong event |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo CONFIG_DEFAULTS; quest là dữ liệu version; `MaxLen` |
| VIII UI tách logic, token | ✅ | Journal/Tracker chỉ gọi `QuestService`; điều kiện quest ở server |
| IX Test có bằng chứng | ✅ | Mỗi F072-AC có task test; facade test `createAttempt` |

## Project Structure

### Documentation (feature này)

```text
specs/072-quest-engine/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/                         # questdefinition.go, questversion.go, questprogress.go, queststepattempt.go, periodicquestgrant.go
├── internal/domain/quest/              # QuestVersion (parse v2), QuestAvailability, QuestProgress, StepHandler + registry,
│                                       # handlers/{talk_npc,go_to,collect,discover,activity}.go, PassRule, CompletionRule,
│                                       # FailurePolicy, PeriodKey (ICT), QuestPublishValidator
├── internal/application/command/       # accept_quest.go, abandon_quest.go, report_quest_step.go, start_quest_activity.go,
│                                       # apply_activity_result.go, apply_discovery.go, grant_periodic_quests.go,
│                                       # expire_period_quests.go, apply_quest_failure.go
├── internal/application/query/         # list_quests.go, get_quest.go
├── internal/application/port/          # LearningAttemptClient, PresenceReader, MasteryReader, AssignmentReader, RealtimePublisher
├── internal/infrastructure/modules/learning.go         # adapter facade learning (createAttempt)
├── internal/presentation/http/public/quests.go
├── internal/presentation/consumer/     # quest_on_activity_graded.go, quest_on_discovery_found.go, periodic_quests_on_session_started.go
├── internal/presentation/job/          # quest_period_expiry.go, quest_failure_timer.go
└── seed/quests/                        # 3 quest v1 + gán tenant demo
src/backend/modules/learning/internal/presentation/consumer/learning_event_on_quest_completed.go
src/game/
├── scenes/quest/{QuestJournal.tscn, QuestTracker.tscn, QuestMarker3D.tscn}
├── scripts/quest/{quest_service.gd, quest_journal.gd, quest_tracker.gd}
└── tests/unit/quest/
```

**Structure Decision**: Step handler registry trong domain cho phép F073/F076/F103 thêm loại bước không sửa engine (ADR-0048 tinh thần "không sửa quest"). Thư mục Godot `scripts/quest/` thêm vào cây GODOT_ARCHITECTURE §3 (hiện chỉ có `scenes/quest/`).

## Phase 0: Research

Không có — mô hình và luồng ở QUEST_SYSTEM §2–§8, ACTIVITY_ENGINE §9; quyết định ghi ở Clarifications.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `QuestAvailability.Check(student, version, facts) (ok, reasons[])`; `QuestProgress.Accept/ReportStep/ApplyActivityResult/Complete/Abandon/Expire/Fail`; `StepHandler{Validate, OnReport, OnEvent}`; `PassRule.Evaluate(result)`; `PeriodKey(now, quest_reset_time)`; `QuestPublishValidator` | Thuần; Clock inject |
| application | Command `AcceptQuest`, `AbandonQuest`, `ReportQuestStep`, `StartQuestActivity`, `ApplyActivityResult`, `ApplyDiscovery`, `GrantPeriodicQuests`, `ExpirePeriodQuests`, `ApplyQuestFailure` · Query `ListQuests`, `GetQuest` | Command/query tách (ADR-0087); `SELECT … FOR UPDATE` trên progress |
| infrastructure | ent repo; adapter facade `learning` (`src/backend/pkg/modcall`: deadline từ context, không retry/circuit breaker — ADR-0074); `PresenceReader` (Valkey F071); `MasteryReader` (`ref.mastery_summary`); `AssignmentReader` (`infra.tenant_resource_assignments`) | |
| presentation | `listQuests`, `getQuest`, `acceptQuest`, `abandonQuest`, `reportQuestStep`, `startQuestActivity` · 3 consumer · 2 job; learning consumer `learning.learning_event_on_quest_completed` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F030 (`createAttempt`, `activity.graded`), F071 (lõi phát hành, presence, `world.discovery_found`, `ref.mastery_summary`), F014 (`game_session.started`), F031 (learning events).
- Event phát: `quest.accepted`, `quest.step_completed`, `quest.completed`, `quest.abandoned`, `audit.recorded`; tiêu thụ: `activity.graded`, `world.discovery_found`, `game_session.started`.
- Facade: `createAttempt` (facade `learning`, caller `game`).

### Bảo mật, tenant, idempotency, audit

- `game_player`; `TenantScope`; quest platform đọc qua `AssignmentReader` (không `Bypass`).
- `Idempotency-Key` bắt buộc: `acceptQuest`, `abandonQuest`, `reportQuestStep`, `startQuestActivity`; `Idempotency-Key` gửi tiếp xuống `createAttempt`.
- Audit: `quest.completed`, `quest.failed` (actor student, không PII).

### Quan sát

- Metric `eduworld_game_quest_accept_total{result}`, `eduworld_game_quest_step_report_total{type,result}`, `eduworld_game_quest_completed_total{quest_type}`, `eduworld_game_quest_progress_lag_seconds` (graded → step completed); span `quest.start_activity` có `traceparent` sang learning.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
