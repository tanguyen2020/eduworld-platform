# Tasks: F105 — Battle engine server-authoritative và 8 chế độ thi đấu

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `battle` (domain/application/presentation) và đăng ký NATS command `ew.cmd.competition.battle.*`, `coop.*`, consumer `match.created` trong composition root — file: `src/backend/modules/competition/module.go` (nối vào `src/backend/cmd/core-api`, `src/backend/cmd/worker`)
- [ ] T002 [P] Biến `EW_BATTLE_ROOM_CHECKPOINT`, deadline gọi facade `learning` — file: `src/backend/modules/competition/internal/infrastructure/config/config.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta internal learning: thêm `revealPlanItem` + `PlanItemReveal` — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`, `make gen MOD=competition`
- [ ] T004 [P] Delta public: `Match` (`end_reason`, `rated`, `review[]`, bỏ `rating_delta`, outcome mới) — file: `specs/openapi/modules/competition.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=competition`
- [ ] T005 [P] Delta AsyncAPI `battle.match.ended` (outcome `abandoned`/`forfeit`, `end_reason`, `top[]`, `my_rank`, `grade_goal` thay `class_goal` — ADR-0095) — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T006 [P] Payload schema `match.started.v1`, `match.completed.v1`, `match.aborted.v1`; cập nhật EVENT_DESIGN §4.8 — file: `specs/events/payloads/competition/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint`
- [ ] T007 ent schema learning `question_plans` (RLS, unique `match_id`) — file: `src/backend/modules/learning/ent/schema/question_plan.go` — kiểm: `make migrate-diff MOD=learning name=f105_question_plans`, `make migrate-lint MOD=learning`
- [ ] T008 ent schema competition: cột mới `matches`, `match_participants`; bảng `match_results`, `integrity_signals` (partition tháng); cột `rooms`, `session_memberships` — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f105_battle`, `make migrate-lint MOD=competition`
- [ ] T009 [P] Cập nhật DATABASE_DESIGN §2.6, §2.9, §2.10 — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Trận 1v1 Knowledge Battle do server điều khiển (Priority: P1) 🎯 MVP

**Goal**: Plan server, câu không đáp án, nhận/chấm câu trả lời, reveal đồng thời.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Integration test learning F105-AC-02: `createQuestionPlan` chọn câu publish/gán tenant, đúng khối/môn, loại câu đã gặp 7 ngày, payload không đáp án; idempotent theo `match_id`; module ngoài `x-allowed-callers` import facade → `make arch-check` fail — file: `src/backend/modules/learning/tests/integration/questionplan/create_plan_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Facade test competition ↔ learning (fake facade phía competition; hiện thực facade với ví dụ spec phía learning) (`createQuestionPlan`, `scorePlanAnswer`, `revealPlanItem`) — file: `src/backend/modules/competition/tests/contract/learningclient_test.go`, `src/backend/modules/learning/tests/contract/questionplan_provider_test.go`
- [ ] T012 [P] [US1] Unit test F105-AC-04, F105-AC-06: điểm 100/0, câu cuối ×2, bonus chỉ khi `grade_level ≥ speed_bonus_min_grade` (bảng khối 1–9), thắng/hòa — file: `src/backend/modules/competition/tests/unit/battle/scoring_test.go`
- [ ] T013 [P] [US1] Unit test F105-AC-03: state machine từ chối trả lời trễ, trùng, câu chưa phát/đã đóng, không thuộc plan; ghi signal — file: `src/backend/modules/competition/tests/unit/battle/room_answer_test.go`
- [ ] T014 [P] [US1] Integration test (bot WS) F105-AC-01, F105-AC-05: cùng câu, cùng `shown_at`; plan ghim version; reveal đồng thời; không đáp án trước reveal — file: `src/backend/modules/competition/tests/integration/battle/battle_1v1_flow_test.go`

### Implementation cho User Story 1

- [ ] T015 [US1] learning: command `CreateQuestionPlan`, `ScorePlanAnswer` (ghi attempt answer `competition`), query `RevealPlanItem` + handler internal — file: `src/backend/modules/learning/internal/application/command/questionplan/`, `src/backend/modules/learning/internal/application/query/questionplan/reveal_plan_item.go`, `src/backend/modules/learning/internal/presentation/http/internal/question_plan_handler.go`
- [ ] T016 [P] [US1] Domain `MatchRoom`, `ModeRules` 1v1, `Scoring` — file: `src/backend/modules/competition/internal/domain/battle/room.go`, `src/backend/modules/competition/internal/domain/battle/rules_1v1.go`, `src/backend/modules/competition/internal/domain/battle/scoring.go`
- [ ] T017 [US1] Adapter facade `learning` qua `src/backend/pkg/modcall` (port `QuestionPlanClient`) — file: `src/backend/modules/competition/internal/infrastructure/modules/learning.go`
- [ ] T018 [US1] Command `OpenMatchRoom` (gọi `createQuestionPlan` khi tạo trận), `StartCountdown`, `SubmitAnswer`, `CloseQuestion`; consumer `match.created`; NATS handler `battle.answer.submit`, `battle.state.request`; eventmap → `ew:rt:room:<id>:out` — file: `src/backend/modules/competition/internal/application/command/battle/`, `src/backend/modules/competition/internal/presentation/realtime/battle_cmd.go`, `src/backend/modules/competition/internal/presentation/consumer/match_created_consumer.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Kết quả ghi một lần và bằng chứng học tập (Priority: P1)

**Goal**: `match_results` duy nhất, event, audit, attempt answer `competition`.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T019 [P] [US2] Integration test F105-AC-07, F105-AC-08 (TC-CMP-009): ghi kết quả 2 lần/2 node → 1 dòng, 1 event; audit; không có REST ghi kết quả (quét OpenAPI) — file: `src/backend/modules/competition/tests/integration/battle/record_result_test.go` — phải FAIL trước khi code
- [ ] T020 [P] [US2] Integration test learning F105-AC-09: `scorePlanAnswer` idempotent `(plan_id, index, student_id)`, `late` không điểm, attempt `mode = competition` — file: `src/backend/modules/learning/tests/integration/questionplan/score_answer_test.go`
- [ ] T021 [P] [US2] Integration test F105-AC-10: kết quả còn sau `FLUSHALL` Valkey — file: `src/backend/modules/competition/tests/integration/battle/result_durability_test.go`

### Implementation cho User Story 2

- [ ] T022 [US2] Command `RecordMatchResult` (tx: `match_results` + participant + outbox `match.completed` + audit) — file: `src/backend/modules/competition/internal/application/command/battle/record_match_result.go`, `src/backend/modules/competition/internal/infrastructure/persistence/match_result_repo.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Mất kết nối, kết nối lại, đổi thiết bị, sự cố server (Priority: P1)

**Goal**: Grace theo mode, abandon/forfeit, takeover, checkpoint/failover, hạn chế bỏ trận.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T023 [P] [US3] Unit test F105-AC-11, F105-AC-12, F105-AC-13: `DisconnectPolicy` theo `disconnect_grace` cho 1v1/đội/timed quiz/giải; cả hai rời → `cancelled` — file: `src/backend/modules/competition/tests/unit/battle/disconnect_test.go` — phải FAIL trước khi code
- [ ] T024 [P] [US3] Integration test F105-AC-11, F105-AC-14: ngắt/nối lại → `status_changed` + `state.snapshot`; takeover vào lại bằng principal, `leave_reason = replaced_by_other_device` — file: `src/backend/modules/competition/tests/integration/battle/reconnect_takeover_test.go`
- [ ] T025 [P] [US3] Integration test F105-AC-15: kill room host → khôi phục checkpoint, replay câu; không checkpoint → `aborted`, không rating/abandon — file: `src/backend/modules/competition/tests/integration/battle/failover_test.go`
- [ ] T026 [P] [US3] Integration test F105-AC-16: 3 `abandoned`/24 h → `competition_restrictions` + audit; trận `aborted` không tính — file: `src/backend/modules/competition/tests/integration/battle/abandon_penalty_test.go`

### Implementation cho User Story 3

- [ ] T027 [P] [US3] Domain `DisconnectPolicy` — file: `src/backend/modules/competition/internal/domain/battle/disconnect.go`
- [ ] T028 [US3] Command `HandleDisconnect`, `HandleReconnect`, `AbortMatch`, `ApplyAbandonPenalty`; checkpoint store; consumer `game_session.replaced` — file: `src/backend/modules/competition/internal/application/command/battle/handle_disconnect.go`, `src/backend/modules/competition/internal/application/command/battle/abort_match.go`, `src/backend/modules/competition/internal/application/command/battle/apply_abandon_penalty.go`, `src/backend/modules/competition/internal/infrastructure/cache/room_checkpoint.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Team Battle, Co-op Challenge, Timed Quiz, bot và thử thách khối (Priority: P2)

**Goal**: Luật các mode còn lại.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T029 [P] [US4] Unit test F105-AC-17: xoay vòng lượt chính, lượt hỗ trợ theo `team_battle_format`, khối ≥ 4 — file: `src/backend/modules/competition/tests/unit/battle/rules_team_test.go` — phải FAIL trước khi code
- [ ] T030 [P] [US4] Integration test F105-AC-18: room `coop`, attempt riêng từng người, không thứ hạng — file: `src/backend/modules/competition/tests/integration/battle/coop_test.go`
- [ ] T031 [P] [US4] Unit + integration test F105-AC-19: timed quiz 40 người, top 5 + `my_rank`, lớp 1–3 chỉ `grade_goal` (không tên, ô < 5 ẩn), lớp 1–5 không bonus — file: `src/backend/modules/competition/tests/integration/battle/timed_quiz_test.go`
- [ ] T032 [P] [US4] Unit test F105-AC-20, F105-AC-21: bot (nhãn, `rated = false`, độ chính xác cấu hình); trận giải grace 2 lần → `forfeit`, `rated = false` — file: `src/backend/modules/competition/tests/unit/battle/bot_tournament_rules_test.go`

### Implementation cho User Story 4

- [ ] T033 [P] [US4] Domain `rules_team.go`, `rules_coop.go`, `rules_timed_quiz.go`, `rules_tournament.go`, `bot.go` — file: `src/backend/modules/competition/internal/domain/battle/`
- [ ] T034 [US4] NATS handler `coop.answer.submit`, `coop.action.perform`; join timed quiz cho thử thách khối (command `JoinTimedQuizRoom` dùng bởi F107) — file: `src/backend/modules/competition/internal/presentation/realtime/coop_cmd.go`, `src/backend/modules/competition/internal/application/command/battle/join_timed_quiz_room.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Room, lịch sử trận, phân quyền và chống gian lận (Priority: P1)

**Goal**: Room/session membership, xem trận, authz, signal, event cho chat.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T035 [P] [US5] Integration test F105-AC-22: `rooms`, `session_memberships` đầy đủ; non-member → `ROOM_NOT_MEMBER`; ticket tenant khác → `PERMISSION_DENIED` (TC-TEN-012) — file: `src/backend/modules/competition/tests/integration/battle/room_membership_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US5] Test tenant isolation + authorization F105-AC-23, F105-AC-25: `getMatch`/`listMyMatches` participant/không participant/nhân sự trường khác/tenant khác; staff token game → `403 GAME_ACCESS_STUDENT_ONLY`; header sai → `403 TENANT_MISMATCH`; không trả số rating — file: `src/backend/modules/competition/tests/integration/battle/match_authz_tenant_test.go`
- [ ] T037 [P] [US5] Unit test F105-AC-24: `FastAnswerDetector` theo `cheat_fast_answer_flag`, bỏ bonus, không đổi kết quả — file: `src/backend/modules/competition/tests/unit/battle/integrity_test.go`
- [ ] T038 [P] [US5] Integration test F105-AC-26, F105-AC-27: event `match.started/completed/aborted` có participants/đội, không cờ tắt chat (quét cấu hình); lệnh sai schema/maxLength → `VALIDATION_FAILED`, room không đổi — file: `src/backend/modules/competition/tests/integration/battle/events_validation_test.go`

### Implementation cho User Story 5

- [ ] T039 [US5] Query `GetMatch` (review sau trận qua `revealPlanItem`), `ListMyMatches` + handler — file: `src/backend/modules/competition/internal/application/query/battle/get_match.go`, `src/backend/modules/competition/internal/presentation/http/public/match_handler.go`
- [ ] T040 [US5] Repo `rooms`/`session_memberships`; `FastAnswerDetector` + ghi `integrity_signals` — file: `src/backend/modules/competition/internal/infrastructure/persistence/room_repo.go`, `src/backend/modules/competition/internal/domain/battle/integrity.go`

**Checkpoint**: User Story 5 đạt.

## Phase 8: Polish & Cross-Cutting

- [ ] T041 [P] Metric/span theo plan.md §Quan sát; kiểm log không chứa `response` (R14) — file: `src/backend/modules/competition/internal/infrastructure/observability/battle_metrics.go`
- [ ] T042 [P] Kịch bản tải battle → learning (TC-PERF-013) — file: `tests/perf/k6/battle_learning.js`
- [ ] T043 [P] Ghi lệch COMPETITION_SYSTEM §3.1/MULTIPLAYER_GAMEPLAY §9.2 và REALTIME §7.1 để cập nhật; CHANGELOG; trạng thái F105 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T044 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/105-battle-engine/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T003–T009) → US1 (T015 learning trước T017/T018) → US2 → US3 → US4, US5 (song song) → Polish.
- T018, T028, T034 cùng chạm luồng room: tuần tự. F106 cần T022 (`match.completed`); F107 cần T034 (`JoinTimedQuizRoom`) và luật giải T033.

## Definition of Done

- [ ] Mọi tiêu chí F105-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
