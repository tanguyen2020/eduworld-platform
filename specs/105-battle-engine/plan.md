# Implementation Plan: F105 — Battle engine server-authoritative và 8 chế độ thi đấu

**Branch**: `105-battle-engine` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Hai phần theo thứ tự bên nhận → bên gọi. (1) **module learning**: hiện thực facade `createQuestionPlan`, `scorePlanAnswer`, `revealPlanItem` (mới) dùng port `Delivery`/grader của F022/F030; plan lưu `assessment.question_plans` (ghim version), câu trả lời ghi `attempt_answers` `mode = competition`. (2) **module competition**: module `battle` gồm aggregate `MatchRoom` (state machine theo [REALTIME_ARCHITECTURE §7.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md#71-state-machine-knowledge-battle)) chạy trong room host (`src/backend/pkg/roomhost`: lease Valkey, checkpoint, input NATS), luật từng mode là `ModeRules` thuần (1v1, team, co-op, timed quiz, tournament, bot). Gọi learning qua typed client + `src/backend/pkg/modcall` (R29). Kết thúc trận: command `RecordMatchResult` ghi `match_results` + participant + outbox `match.completed` + audit trong một transaction (ADR-0038/0042).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server public; facade `src/backend/modules/learning/api` sinh từ `learning.internal.v1.yaml`), nats.go, go-redis, `src/backend/pkg/roomhost`, `src/backend/pkg/modcall`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, `src/backend/pkg/clock` |
| Storage | PostgreSQL `eduworld_competition` (`competition`, `multiplayer`); `eduworld_learning` (`assessment`); Valkey `ew:rt:room:*` (lease, checkpoint) |
| Testing | go test + testcontainers (PostgreSQL ×2, Valkey, NATS); bot WS; đồng hồ giả; facade test competition ↔ learning (fake facade phía competition, ví dụ spec phía learning) |
| Target Platform | Linux container (`core-api` nạp module `competition` + `learning`, `worker-events`, `realtime`) |
| Project Type | backend module (2 module) |
| Performance Goals | TC-PERF-013 (competition → learning p95); tick room không chặn bởi I/O DB |
| Constraints | AGENTS R5, R6, R9–R11, R13–R16, R23, R26–R29, R31 · ADR-0004, ADR-0016, ADR-0017, ADR-0020, ADR-0031, ADR-0040, ADR-0074, ADR-0042, ADR-0043 |
| Scale/Scope | 20% CCU trong trận/co-op (TC-PERF-014); battle 2–10 người, timed quiz ≤ 40 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta contract (`revealPlanItem`, `Match` review, `battle.match.ended` outcome, payload event match) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Phần learning nằm trong module learning; competition gọi facade `learning` (`x-allowed-callers: [competition]`); phản ứng phụ (XP, chat, evidence projection) qua event |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `match_participants`, `session_memberships`, `integrity_signals` có `tenant_id` + RLS; không hard-delete |
| IV Server-authoritative | ✅ | Giờ server, chấm ở learning, đáp án chỉ sau khi câu đóng; kết quả unique `match_id` |
| V Xác thực, phân quyền | ✅ | `game_player` cho lệnh/route người chơi; `lms.class.read` cho nhân sự xem trận |
| VI An toàn trẻ em | ✅ | Chỉ nickname/avatar; lớp 1–3 không thứ hạng cá nhân ở timed quiz; chat trận always-on; không log câu trả lời |
| VII Một nguồn sự thật, cấu hình | ✅ | Luật mode đọc từ cấu hình đã publish (tên CONFIG_DEFAULTS); grace theo MULTIPLAYER_GAMEPLAY |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi `F105-AC-*` có task test viết trước; facade test hai phía |

## Project Structure

### Documentation (feature này)

```text
specs/105-battle-engine/
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
src/backend/modules/learning/
├── ent/schema/question_plan.go                     # mới
├── internal/application/command/questionplan/      # create_question_plan.go, score_plan_answer.go
├── internal/application/query/questionplan/reveal_plan_item.go
├── internal/presentation/http/internal/question_plan_handler.go
└── tests/{integration,contract}/questionplan/

src/backend/modules/competition/
├── ent/schema/match_result.go, integrity_signal.go, room.go, session_membership.go  # mới/thêm cột
├── internal/domain/battle/
│   ├── room.go             # MatchRoom aggregate: trạng thái, câu hiện tại, deadline, điểm tạm
│   ├── rules.go            # ModeRules interface; rules_1v1.go, rules_team.go, rules_coop.go, rules_timed_quiz.go, rules_tournament.go
│   ├── scoring.go          # điểm, bonus tốc độ theo speed_bonus_min_grade, câu cuối ×2
│   ├── disconnect.go       # grace theo mode, abandon/forfeit
│   ├── bot.go              # BotPlayer (độ chính xác, thời gian)
│   └── integrity.go        # fast-answer detector
├── internal/application/
│   ├── command/battle/     # open_match_room.go, start_countdown.go, submit_answer.go, close_question.go, handle_disconnect.go, record_match_result.go, abort_match.go, apply_abandon_penalty.go
│   ├── query/battle/       # get_match.go, list_my_matches.go, get_state_snapshot.go
│   └── port/               # QuestionPlanClient, MatchRepository, RoomCheckpointStore, RealtimePublisher
├── internal/infrastructure/modules/learning.go     # adapter facade + modcall
├── internal/presentation/
│   ├── realtime/battle_cmd.go, coop_cmd.go          # ew.cmd.competition.battle.*, coop.*
│   ├── realtime/eventmap.go                         # match.completed → battle.match.ended
│   ├── http/public/match_handler.go                 # getMatch, listMyMatches
│   └── consumer/match_created_consumer.go           # mở room cho match.created (nội bộ)
└── tests/{unit,integration,contract}/battle/
```

**Structure Decision**: State machine và luật mode là domain thuần để unit test bằng đồng hồ giả; room host chỉ là adapter (lease/checkpoint). Plan câu hỏi và chấm ở module learning vì answer key không rời module sở hữu (ADR-0016).

## Phase 0: Research

Không có. State machine, chấm, checkpoint, grace đã định ở REALTIME_ARCHITECTURE §5.1, §7, COMPETITION_SYSTEM §3–§4, MULTIPLAYER_GAMEPLAY §9.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `MatchRoom` (`lobby → countdown → question_shown → reveal → round_end → finalizing → ended`, `paused`, `cancelled`, `aborted`), `ModeRules`, `Scoring`, `DisconnectPolicy`, `BotPlayer`, `FastAnswerDetector` | Invariant: một câu trả lời/người/câu; không gửi đáp án trước reveal |
| application | Command `OpenMatchRoom`, `StartCountdown`, `SubmitAnswer`, `CloseQuestion`, `HandleDisconnect`, `HandleReconnect`, `RecordMatchResult`, `AbortMatch`, `ApplyAbandonPenalty` · Query `GetMatch`, `ListMyMatches`, `GetStateSnapshot`; learning: `CreateQuestionPlan`, `ScorePlanAnswer`, `RevealPlanItem` | Command (`command/`, `UnitOfWork`) / query (`query/`, chỉ đọc) tách, gọi qua interface có kiểu |
| infrastructure | Adapter facade `learning` (deadline theo câu, `Idempotency-Key`, ánh xạ lỗi; không retry/circuit breaker), ent repo, checkpoint Valkey, outbox | `Idempotency-Key` = `<plan_id>:<index>:<student_id>` |
| presentation | NATS `battle.answer.submit`, `battle.state.request`, `coop.answer.submit`, `coop.action.perform`; handler `getMatch`, `listMyMatches`; consumer `match.created` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F104 (`CreateMatch`, `match.created`), F030/F022 (grader, `Delivery`), F090 (gateway, `ew:rt:room:<id>:out`), F091/F101 (`src/backend/pkg/roomhost`), F014 (takeover `game_session.replaced`).
- Facade: `learning` → `createQuestionPlan`, `scorePlanAnswer`, `revealPlanItem`.
- Event phát: `match.started`, `match.completed`, `match.aborted`, `audit.recorded`. Tiêu thụ: `match.created` (nội bộ), `game_session.replaced`, `user.access_revoked`.
- Bên dùng: F106 (rating, points, leaderboard), F107 (bracket), F117 (chat), module game (XP).

### Bảo mật, tenant, idempotency, audit

- Lệnh WS: principal từ gateway; kiểm member room + tenant room = tenant ticket (`PERMISSION_DENIED`).
- `getMatch`/`listMyMatches`: học sinh participant; nhân sự `lms.class.read` (portal) với participant thuộc trường trong scope.
- Idempotency: `RecordMatchResult` theo `match_id` (PK); `scorePlanAnswer` theo `(plan_id, index, student_id)`.
- Audit: `match_result.recorded`, `match.aborted`, `competition_restriction.created`.

### Quan sát

- Log không chứa `response`; chỉ `match_id`, `question_index`.
- Metric: `eduworld_rooms_active{room_type}`, `eduworld_battle_answer_rejected_total{code}`, `eduworld_battle_score_call_seconds`, `eduworld_room_abandon_total{mode}`, `eduworld_matches_aborted_total{reason}`, `eduworld_reconnect_total{result}`, `eduworld_integrity_signals_total{type}`.
- Span: `OpenMatchRoom`→`createQuestionPlan`, `SubmitAnswer`→`scorePlanAnswer`, `RecordMatchResult`.

Ước lượng: 3 sprint (backend 2 dev) [Proposed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
