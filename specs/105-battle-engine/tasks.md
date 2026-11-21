# Tasks: F105 — Battle engine server-authoritative và 8 chế độ thi đấu

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `battle` (domain/application/presentation) và đăng ký NATS command `ew.cmd.competition.battle.*`, `coop.*`, consumer `match.created` trong composition root — file: `src/backend/modules/competition/module.go` (nối vào `src/backend/cmd/core-api`, `src/backend/cmd/worker`) — **Xong 2026-11-21:** `module.go`: room host `competition.battlehost` + lệnh `battle.answer.submit`, `battle.state.request`, `coop.answer.submit`, `coop.action.perform` (core-api), consumer `competition.battle_open` (worker events); `cmd/internal/wiring/compose.go` nối facade learning
- [x] T002 [P] Biến `EW_BATTLE_ROOM_CHECKPOINT`, deadline gọi facade `learning` — file: `src/backend/modules/competition/internal/infrastructure/config/config.go` — **Xong 2026-11-21:** `internal/infrastructure/config/battle.go`: `EW_BATTLE_TICK_HZ`, `EW_BATTLE_ROOM_CHECKPOINT`, `EW_BATTLE_PLAN_DEADLINE`, `EW_BATTLE_SCORE_DEADLINE`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Delta internal learning: thêm `revealPlanItem` + `PlanItemReveal` — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`, `make gen MOD=competition` — **Xong 2026-11-21:** `revealPlanItem` + `PlanItemReveal` (thêm `question` render payload), `QuestionPlanRequest.subject_id`, `participants` ≤ 40, `QuestionPlan.plan_version`; facadegen/allowedcallers/archgen sinh lại
- [x] T004 [P] Delta public: `Match` (`end_reason`, `rated`, `review[]`, bỏ `rating_delta`, outcome mới) — file: `specs/openapi/modules/competition.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=competition` — **Xong 2026-11-21:** `Match`: `end_reason`, `rated`, `question_count`, `review[]`, outcome `forfeit`/`no_contest`, bỏ `rating_delta`, `getMatch` 401/403; split-public + portal `pnpm gen:api`/`gen:check` khớp
- [x] T005 [P] Delta AsyncAPI `battle.match.ended` (outcome `abandoned`/`forfeit`, `end_reason`, `top[]`, `my_rank`, `grade_goal` thay `class_goal` — ADR-0095) — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint` — **Xong 2026-11-21:** `battle.match.ended`: outcome `abandoned`/`forfeit`, `end_reason`, `top[]`, `my_rank`, `grade_goal {current, target, reached, contributors_count ≥ 5}`
- [x] T006 [P] Payload schema `match.started.v1`, `match.completed.v1`, `match.aborted.v1`; cập nhật EVENT_DESIGN §4.8 — file: `specs/events/payloads/competition/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint` — **Xong 2026-11-21:** `match.started.v1`, `match.completed.v1`, `match.aborted.v1` (validate-schemas 0 lỗi, check-maxlength 0 vi phạm; outbox kiểm schema trong `TestBattleFlow1v1`, `TestBattleFailover`); EVENT_DESIGN §4.8
- [x] T007 ent schema learning `question_plans` (RLS, unique `match_id`) — file: `src/backend/modules/learning/ent/schema/question_plan.go` — kiểm: `make migrate-diff MOD=learning name=f105_question_plans`, `make migrate-lint MOD=learning` — **Xong 2026-11-21:** `ent/schema/question_plan.go`, migration `20261229000003_f105_question_plans.sql` (RLS, unique `match_id` live, CHECK); `TestMigrationMatchesEnt` pass; atlas.sum qua `scripts/db/atlas-sum.mjs`
- [x] T008 ent schema competition: cột mới `matches`, `match_participants`; bảng `match_results`, `integrity_signals` (partition tháng); cột `rooms`, `session_memberships` — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f105_battle`, `make migrate-lint MOD=competition` — **Xong 2026-11-21:** `match_results` (ent, `tenant_ids` tay), cột `matches`/`match_participants`/`rooms`, CHECK `integrity_signals` mới; migration `20261231000003_f105_battle.sql`; `TestMigrationMatchesEnt` pass
- [x] T009 [P] Cập nhật DATABASE_DESIGN §2.6, §2.9, §2.10 — file: `docs/07-backend/DATABASE_DESIGN.md` — **Xong 2026-11-21:** DATABASE_DESIGN: `question_plans`, `matches`, `match_participants`, `match_results`, `rooms`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Trận 1v1 Knowledge Battle do server điều khiển (Priority: P1) 🎯 MVP

**Goal**: Plan server, câu không đáp án, nhận/chấm câu trả lời, reveal đồng thời.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Integration test learning F105-AC-02: `createQuestionPlan` chọn câu publish/gán tenant, đúng khối/môn, loại câu đã gặp 7 ngày, payload không đáp án; idempotent theo `match_id`; module ngoài `x-allowed-callers` import facade → `make arch-check` fail — file: `src/backend/modules/learning/tests/integration/questionplan/create_plan_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `modules/learning/tests/integration/question_plan_test.go` `TestQuestionPlanCreate` (khối/môn, không câu nháp/khối khác, tránh câu đã gặp, không đáp án, idempotent, caller chat bị chặn, tenant B, pool thiếu) — pass
- [x] T011 [P] [US1] Facade test competition ↔ learning (fake facade phía competition; hiện thực facade với ví dụ spec phía learning) (`createQuestionPlan`, `scorePlanAnswer`, `revealPlanItem`) — file: `src/backend/modules/competition/tests/contract/learningclient_test.go`, `src/backend/modules/learning/tests/contract/questionplan_provider_test.go` — **Xong 2026-11-21:** bên gọi `internal/infrastructure/modules/learning_facade_test.go` `TestLearningPlansFacade` (fake `learningapi.Fake`); bên nhận: facade thật trong `TestQuestionPlanCreate`/`TestQuestionPlanScoreAndReveal` — pass
- [x] T012 [P] [US1] Unit test F105-AC-04, F105-AC-06: điểm 100/0, câu cuối ×2, bonus chỉ khi `grade_level ≥ speed_bonus_min_grade` (bảng khối 1–9), thắng/hòa — file: `src/backend/modules/competition/tests/unit/battle/scoring_test.go` — **Xong 2026-11-21:** `tests/unit/battle_scoring_test.go` `TestBattlePointsByGrade`, `TestBattleWinnerAndDraw` — FAIL (package chưa có) rồi PASS
- [x] T013 [P] [US1] Unit test F105-AC-03: state machine từ chối trả lời trễ, trùng, câu chưa phát/đã đóng, không thuộc plan; ghi signal — file: `src/backend/modules/competition/tests/unit/battle/room_answer_test.go` — **Xong 2026-11-21:** `tests/unit/battle_room_answer_test.go` `TestBattleAnswerRules`, `TestBattleScoreFailureReplaysThenAborts` — FAIL rồi PASS
- [x] T014 [P] [US1] Integration test (bot WS) F105-AC-01, F105-AC-05: cùng câu, cùng `shown_at`; plan ghim version; reveal đồng thời; không đáp án trước reveal — file: `src/backend/modules/competition/tests/integration/battle/battle_1v1_flow_test.go` — **Xong 2026-11-21:** `tests/integration/battle_flow_test.go` `TestBattleFlow1v1` (fake learning có nhãn TEST DOUBLE; room host thật + Valkey + NATS) — pass

### Implementation cho User Story 1

- [x] T015 [US1] learning: command `CreateQuestionPlan`, `ScorePlanAnswer` (ghi attempt answer `competition`), query `RevealPlanItem` + handler internal — file: `src/backend/modules/learning/internal/application/command/questionplan/`, `src/backend/modules/learning/internal/application/query/questionplan/reveal_plan_item.go`, `src/backend/modules/learning/internal/presentation/http/internal/question_plan_handler.go` — **Xong 2026-11-21:** `command/question_plans.go` (`CreatePlan`, `ScoreAnswer`), `query/question_plans.go` (`Reveal`), `presentation/facade/question_plans.go`, repo `QuestionPlanRepo`, read `QuestionPlanModel`
- [x] T016 [P] [US1] Domain `MatchRoom`, `ModeRules` 1v1, `Scoring` — file: `src/backend/modules/competition/internal/domain/battle/room.go`, `src/backend/modules/competition/internal/domain/battle/rules_1v1.go`, `src/backend/modules/competition/internal/domain/battle/scoring.go` — **Xong 2026-11-21:** `internal/domain/battle/` (`room.go`, `config.go`, `scoring.go`, `response.go`) — 13 unit test pass
- [x] T017 [US1] Adapter facade `learning` qua `src/backend/pkg/modcall` (port `QuestionPlanClient`) — file: `src/backend/modules/competition/internal/infrastructure/modules/learning.go` — **Xong 2026-11-21:** `internal/infrastructure/modules/learning.go` (`Learning`, `LateLearning`)
- [x] T018 [US1] Command `OpenMatchRoom` (gọi `createQuestionPlan` khi tạo trận), `StartCountdown`, `SubmitAnswer`, `CloseQuestion`; consumer `match.created`; NATS handler `battle.answer.submit`, `battle.state.request`; eventmap → `ew:rt:room:<id>:out` — file: `src/backend/modules/competition/internal/application/command/battle/`, `src/backend/modules/competition/internal/presentation/realtime/battle_cmd.go`, `src/backend/modules/competition/internal/presentation/consumer/match_created_consumer.go` — **Xong 2026-11-21:** `command/battle.go` (`Open`, `Started`, `Score`, `Reveal`), `infrastructure/battlehost/host.go`, `presentation/natscmd/battle.go`, `presentation/consumer/battle_open.go`, `infrastructure/realtime/battle.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Kết quả ghi một lần và bằng chứng học tập (Priority: P1)

**Goal**: `match_results` duy nhất, event, audit, attempt answer `competition`.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T019 [P] [US2] Integration test F105-AC-07, F105-AC-08 (TC-CMP-009): ghi kết quả 2 lần/2 node → 1 dòng, 1 event; audit; không có REST ghi kết quả (quét OpenAPI) — file: `src/backend/modules/competition/tests/integration/battle/record_result_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `TestBattleRecordOnce` (4 lần đồng thời → 1 dòng, 1 event, 1 audit, 3 × `MATCH_RESULT_ALREADY_RECORDED`; không REST ghi kết quả) — pass
- [x] T020 [P] [US2] Integration test learning F105-AC-09: `scorePlanAnswer` idempotent `(plan_id, index, student_id)`, `late` không điểm, attempt `mode = competition` — file: `src/backend/modules/learning/tests/integration/questionplan/score_answer_test.go` — **Xong 2026-11-21:** `TestQuestionPlanScoreAndReveal` (idempotent, 4 lời gọi đồng thời → 1 dòng, `late` không điểm, attempt `competition`, `question.answered` context competition) — pass
- [x] T021 [P] [US2] Integration test F105-AC-10: kết quả còn sau `FLUSHALL` Valkey — file: `src/backend/modules/competition/tests/integration/battle/result_durability_test.go` — **Xong 2026-11-21:** `TestBattleFlow1v1` (FLUSHALL Valkey rồi `getMatch` vẫn đủ kết quả)

### Implementation cho User Story 2

- [x] T022 [US2] Command `RecordMatchResult` (tx: `match_results` + participant + outbox `match.completed` + audit) — file: `src/backend/modules/competition/internal/application/command/battle/record_match_result.go`, `src/backend/modules/competition/internal/infrastructure/persistence/match_result_repo.go` — **Xong 2026-11-21:** `command/battle.go` `Finish` + `persistence/battle.go` `RecordResult`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Mất kết nối, kết nối lại, đổi thiết bị, sự cố server (Priority: P1)

**Goal**: Grace theo mode, abandon/forfeit, takeover, checkpoint/failover, hạn chế bỏ trận.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T023 [P] [US3] Unit test F105-AC-11, F105-AC-12, F105-AC-13: `DisconnectPolicy` theo `disconnect_grace` cho 1v1/đội/timed quiz/giải; cả hai rời → `cancelled` — file: `src/backend/modules/competition/tests/unit/battle/disconnect_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `tests/unit/battle_disconnect_test.go` `TestBattleDisconnect1v1`, `TestBattleDisconnectTeamAndTimedQuiz`, `TestBattleCheckpointResume` — FAIL rồi PASS
- [x] T024 [P] [US3] Integration test F105-AC-11, F105-AC-14: ngắt/nối lại → `status_changed` + `state.snapshot`; takeover vào lại bằng principal, `leave_reason = replaced_by_other_device` — file: `src/backend/modules/competition/tests/integration/battle/reconnect_takeover_test.go` — **Xong 2026-11-21:** `tests/integration/battle_resilience_test.go` `TestBattleReconnectTakeover` — pass
- [x] T025 [P] [US3] Integration test F105-AC-15: kill room host → khôi phục checkpoint, replay câu; không checkpoint → `aborted`, không rating/abandon — file: `src/backend/modules/competition/tests/integration/battle/failover_test.go` — **Xong 2026-11-21:** `TestBattleFailover` (crash owner → replica khác khôi phục, chơi lại câu; xóa checkpoint → `aborted` `server_failure`, không kết quả) — pass
- [x] T026 [P] [US3] Integration test F105-AC-16: 3 `abandoned`/24 h → `competition_restrictions` + audit; trận `aborted` không tính — file: `src/backend/modules/competition/tests/integration/battle/abandon_penalty_test.go` — **Xong 2026-11-21:** `TestBattleAbandonPenalty` (plan lỗi → aborted không tính; 3 lần bỏ ranked → hạn chế 15 phút + audit) — pass

### Implementation cho User Story 3

- [x] T027 [P] [US3] Domain `DisconnectPolicy` — file: `src/backend/modules/competition/internal/domain/battle/disconnect.go` — **Xong 2026-11-21:** `domain/battle/room.go` (`Disconnect`, `expire`, `leave`)
- [x] T028 [US3] Command `HandleDisconnect`, `HandleReconnect`, `AbortMatch`, `ApplyAbandonPenalty`; checkpoint store; consumer `game_session.replaced` — file: `src/backend/modules/competition/internal/application/command/battle/handle_disconnect.go`, `src/backend/modules/competition/internal/application/command/battle/abort_match.go`, `src/backend/modules/competition/internal/application/command/battle/apply_abandon_penalty.go`, `src/backend/modules/competition/internal/infrastructure/cache/room_checkpoint.go` — **Xong 2026-11-21:** `Disconnect` qua tín hiệu presence của gateway, reconnect/takeover qua `battle.state.request` (phiên game mới → `replaced_by_other_device`), `AbortUnhosted`, `abandonPenalties`; checkpoint `cache/battle.go` — không cần consumer `game_session.replaced` (thiết bị mới tự vào lại bằng principal)

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Team Battle, Co-op Challenge, Timed Quiz, bot và thử thách khối (Priority: P2)

**Goal**: Luật các mode còn lại.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T029 [P] [US4] Unit test F105-AC-17: xoay vòng lượt chính, lượt hỗ trợ theo `team_battle_format`, khối ≥ 4 — file: `src/backend/modules/competition/tests/unit/battle/rules_team_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `tests/unit/battle_modes_test.go` `TestBattleTeamRotation` — FAIL rồi PASS
- [x] T030 [P] [US4] Integration test F105-AC-18: room `coop`, attempt riêng từng người, không thứ hạng — file: `src/backend/modules/competition/tests/integration/battle/coop_test.go` — **Xong 2026-11-21:** `tests/integration/battle_access_test.go` `TestBattleModes` (room `coop`, mỗi câu trả lời chấm riêng, `no_contest`, không rating) + unit `TestBattleCoop` — pass
- [x] T031 [P] [US4] Unit + integration test F105-AC-19: timed quiz 40 người, top 5 + `my_rank`, lớp 1–3 chỉ `grade_goal` (không tên, ô < 5 ẩn), lớp 1–5 không bonus — file: `src/backend/modules/competition/tests/integration/battle/timed_quiz_test.go` — **Xong 2026-11-21:** unit `TestBattleTimedQuiz` (41 người → ROOM_FULL; 40 người; lớp 4 top 5 + `my_rank`; lớp 2 chỉ `grade_goal`; không bonus) — pass; tích hợp Timed Quiz đầy đủ chờ F107 tạo trận
- [x] T032 [P] [US4] Unit test F105-AC-20, F105-AC-21: bot (nhãn, `rated = false`, độ chính xác cấu hình); trận giải grace 2 lần → `forfeit`, `rated = false` — file: `src/backend/modules/competition/tests/unit/battle/bot_tournament_rules_test.go` — **Xong 2026-11-21:** unit `TestBattleBot`, `TestBattleTournament`; tích hợp bot trong `TestBattleModes` — pass

### Implementation cho User Story 4

- [x] T033 [P] [US4] Domain `rules_team.go`, `rules_coop.go`, `rules_timed_quiz.go`, `rules_tournament.go`, `bot.go` — file: `src/backend/modules/competition/internal/domain/battle/` — **Xong 2026-11-21:** luật mode trong `domain/battle/config.go` (`RulesFor`), `scoring.go` (`Points`, lượt hỗ trợ), `room.go` (`MainTurn`), `BotAnswer`
- [x] T034 [US4] NATS handler `coop.answer.submit`, `coop.action.perform`; join timed quiz cho thử thách khối (command `JoinTimedQuizRoom` dùng bởi F107) — file: `src/backend/modules/competition/internal/presentation/realtime/coop_cmd.go`, `src/backend/modules/competition/internal/application/command/battle/join_timed_quiz_room.go` — **Xong 2026-11-21:** `natscmd.CoopAnswer`, `natscmd.CoopAction`; vào Timed Quiz bằng `battle.state.request` (Clarifications 2026-11-21, không command riêng)

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Room, lịch sử trận, phân quyền và chống gian lận (Priority: P1)

**Goal**: Room/session membership, xem trận, authz, signal, event cho chat.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T035 [P] [US5] Integration test F105-AC-22: `rooms`, `session_memberships` đầy đủ; non-member → `ROOM_NOT_MEMBER`; ticket tenant khác → `PERMISSION_DENIED` (TC-TEN-012) — file: `src/backend/modules/competition/tests/integration/battle/room_membership_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `TestBattleFlow1v1` (rooms, session_memberships, `ROOM_NOT_MEMBER`, `PERMISSION_DENIED`) — pass
- [x] T036 [P] [US5] Test tenant isolation + authorization F105-AC-23, F105-AC-25: `getMatch`/`listMyMatches` participant/không participant/nhân sự trường khác/tenant khác; staff token game → `403 GAME_ACCESS_STUDENT_ONLY`; header sai → `403 TENANT_MISMATCH`; không trả số rating — file: `src/backend/modules/competition/tests/integration/battle/match_authz_tenant_test.go` — **Xong 2026-11-21:** `TestBattleMatchAuthzTenant` (participant 200, học sinh khác 404, nhân sự trường 200, trường khác 404, token game 403 `GAME_ACCESS_STUDENT_ONLY`, `TENANT_MISMATCH`, không rating) + `authztest.Cover` — pass
- [x] T037 [P] [US5] Unit test F105-AC-24: `FastAnswerDetector` theo `cheat_fast_answer_flag`, bỏ bonus, không đổi kết quả — file: `src/backend/modules/competition/tests/unit/battle/integrity_test.go` — **Xong 2026-11-21:** unit `TestBattleFastAnswerSignal` — FAIL rồi PASS
- [x] T038 [P] [US5] Integration test F105-AC-26, F105-AC-27: event `match.started/completed/aborted` có participants/đội, không cờ tắt chat (quét cấu hình); lệnh sai schema/maxLength → `VALIDATION_FAILED`, room không đổi — file: `src/backend/modules/competition/tests/integration/battle/events_validation_test.go` — **Xong 2026-11-21:** `TestBattleCommandValidation` (trường `score`, instance > 64, sai loại response → `VALIDATION_FAILED`, room không đổi; signal `answer_before_shown`; `match.started` có đội) — pass

### Implementation cho User Story 5

- [x] T039 [US5] Query `GetMatch` (review sau trận qua `revealPlanItem`), `ListMyMatches` + handler — file: `src/backend/modules/competition/internal/application/query/battle/get_match.go`, `src/backend/modules/competition/internal/presentation/http/public/match_handler.go` — **Xong 2026-11-21:** `query/battle.go` (`GetMatch` + review qua `revealPlanItem`, `ListMyMatches`), `presentation/http/battle.go`
- [x] T040 [US5] Repo `rooms`/`session_memberships`; `FastAnswerDetector` + ghi `integrity_signals` — file: `src/backend/modules/competition/internal/infrastructure/persistence/room_repo.go`, `src/backend/modules/competition/internal/domain/battle/integrity.go` — **Xong 2026-11-21:** `persistence/battle.go` (`JoinRoom`, `LeaveRoom`, `Disconnected`, `InsertSignal`), `persistence/battle_read.go`

**Checkpoint**: User Story 5 đạt.

## Phase 8: Polish & Cross-Cutting

- [x] T041 [P] Metric/span theo plan.md §Quan sát; kiểm log không chứa `response` (R14) — file: `src/backend/modules/competition/internal/infrastructure/observability/battle_metrics.go` — **Xong 2026-11-21:** `observability/battle.go` (`eduworld_battle_answer_rejected_total`, `eduworld_battle_score_call_seconds`, `eduworld_room_abandon_total`, `eduworld_matches_aborted_total`, `eduworld_reconnect_total`, `eduworld_integrity_signals_total`, `eduworld_rooms_active`); log không chứa response (R14); span qua `modcall`
- [ ] T042 [P] Kịch bản tải battle → learning (TC-PERF-013) — file: `tests/perf/k6/battle_learning.js` — **Chờ:** chạy trên staging (TC-PERF-013); local chỉ có histogram `eduworld_battle_score_call_seconds`
- [x] T043 [P] Ghi lệch COMPETITION_SYSTEM §3.1/MULTIPLAYER_GAMEPLAY §9.2 và REALTIME §7.1 để cập nhật; CHANGELOG; trạng thái F105 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — **Xong 2026-11-21:** Clarifications 2026-11-21 (triển khai); link ở COMPETITION_SYSTEM §4, REALTIME_ARCHITECTURE §7.1; CONFIG_DEFAULTS; CHANGELOG; CODE_PLAN
- [ ] T044 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/105-battle-engine/quickstart.md` — **Chờ:** quickstart tay cần stack compose riêng (stack local đang phục vụ E2E của agent khác); kịch bản 1–24 đã có test tự động tương ứng

## Dependencies & Execution Order

- Setup → Foundational (T003–T009) → US1 (T015 learning trước T017/T018) → US2 → US3 → US4, US5 (song song) → Polish.
- T018, T028, T034 cùng chạm luồng room: tuần tự. F106 cần T022 (`match.completed`); F107 cần T034 (`JoinTimedQuizRoom`) và luật giải T033.

## Definition of Done

- [x] Mọi tiêu chí F105-AC-* có test pass (ghi mã test/TC) — 27/27: unit `battle_*_test.go` 13 pass; integration competition `TestBattle*` 8 pass (cả suite competition 88/88 pass); learning `TestQuestionPlan*` 2 pass; facade test hai phía pass
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — chạy tay tương đương (không có make trên Windows): gofmt, go vet -tags integration, golangci-lint (0 issues) + arch config, archgen, ewlint, entlint, tenancyguard, allowedcallers/facadegen --check, routematrix, gen-routes --check, `TestMigrationMatchesEnt`, unit competition/learning/cmd/tools, integration qua tc-lock; specs + docs check; portal `pnpm gen:check`
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — fake learning/output chỉ trong test, gắn nhãn TEST DOUBLE
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
