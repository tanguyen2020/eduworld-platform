# Tasks: F103 — Boss battle nhiều phase và quest co-op (shared progress, bước `interdependent`)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Adapter facade `learning` (`createQuestionPlan`, `scorePlanAnswer`, `getLearningMaterialRenders`) qua `src/backend/pkg/modcall` — file: `src/backend/modules/game/internal/infrastructure/modules/learning.go`
- [x] T002 [P] Key builder `ew:rt:room:<encounter_id>*`, `ew:game:clue:*`; đăng ký room type `boss` với `src/backend/pkg/roomhost` — file: `src/backend/modules/game/internal/infrastructure/cache/keys.go`, `src/backend/modules/game/module.go`
- [x] T003 [P] Đăng ký handler NATS `ew.cmd.game.boss.ready`, `ew.cmd.game.boss.answer` qua `src/backend/pkg/natsx` — file: `src/backend/modules/game/internal/presentation/natscmd/boss.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta `game.v1` (createBossEncounter chỉ solo, `BossEncounter` cột mới, `QuestDetail.step_type`, `QuestProgress.party_progress`, 3 operation clue/role) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [x] T005 [P] Ba operation party-activity + enum `reason_code` + `x-allowed-callers: [competition]` — file: `specs/openapi/modules/game.internal.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game`
- [x] T006 [P] Delta `learning.internal` (callers `game`, `QuestionPlanRequest` mở rộng, `getLearningMaterialRenders`) — phối hợp nhóm D — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [x] T007 [P] AsyncAPI `boss.question.assigned`, `boss.answer.evaluated`, `quest.roles.assigned`, `quest.clue.shared`, `quest.progress.updated.party_progress`, `BossEncounterEnded.outcome` — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [x] T008 [P] Payload schema `boss.encounter_started`, `boss.phase_completed`, `boss.encounter_completed`, `quest.party_completed` — file: `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [x] T009 ent schema cột mới `BossEncounter`, `BossEncounterMember`; mới `BossPhasePlan`, `BossAnswerResult` (AppendOnly), `PartyQuestSession`, `PartyQuestContribution` với `MaxLen` — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f103_boss_coop`, `make migrate-lint MOD=game`
- [x] T010 Migration: unique `party_activity_id`, unique `(plan_id, item_index, student_id)`, CHECK jsonb size/enum, RLS, `REVOKE UPDATE, DELETE, TRUNCATE` bảng TA — file: `src/backend/modules/game/migrations/`
- [x] T011 [P] Test RLS/global filter bảng mới (tenant A không đọc B; thiếu scope → 0 dòng) — file: `src/backend/modules/game/tests/integration/rls_boss_coop_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Boss nhiều phase, solo và party (Priority: P1) 🎯 MVP

**Goal**: Encounter server-authoritative, đóng góp và thưởng đúng.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T012 [P] [US1] Unit test domain `BossEncounter`/`PhaseRule` (F103-AC-07, F103-AC-08): `all`/`any_n`/`collective_count` + `min_contribution`, retry giữ phase trước, vượt retry/hết giờ → `ended` — file: `src/backend/modules/game/tests/unit/boss_encounter_test.go` — phải FAIL trước khi code
- [x] T013 [P] [US1] Test F103-AC-01: solo hợp lệ/không đủ điều kiện/`mode = party` → `422`; ghim version, hạn giờ server — file: `src/backend/modules/game/tests/integration/create_boss_encounter_test.go`
- [x] T014 [P] [US1] Facade test phía bên nhận F103-AC-02, F103-AC-03 (ví dụ trong spec, bên gọi competition): candidates, eligibility (không lộ khối), `startPartyActivity` idempotent — file: `src/backend/modules/game/tests/contract/party_activities_provider_test.go`
- [x] T015 [P] [US1] Test F103-AC-04, F103-AC-05: ready → phase, plan riêng theo `per_grade`, câu hỏi không có đáp án; answer lấy `student_id` từ header, gửi lặp tính một lần, learning record đúng người (stub learning MOCK + contract consumer) — file: `src/backend/modules/game/tests/integration/boss_phase_answer_test.go`
- [x] T016 [P] [US1] Test F103-AC-06, F103-AC-09: `BOSS_PHASE_CLOSED`, `ROOM_NOT_MEMBER`; message tới người khác không có đúng/sai/số câu đúng — file: `src/backend/modules/game/tests/integration/boss_privacy_test.go`
- [x] T017 [P] [US1] Test F103-AC-10: kết thúc một transaction, `eligible`, thưởng idempotent khi replay 3 lần (TC-CHL-006), `boss.encounter_completed` có `party_activity_id` — file: `src/backend/modules/game/tests/integration/complete_boss_encounter_test.go`
- [x] T018 [P] [US1] Test F103-AC-11, F103-AC-12: grace, takeover thiết bị khác nhận lại trạng thái (checkpoint), dưới `party_size.min` → `abandoned`; leave/party event giữ đóng góp; encounter mồ côi → `abandoned` — file: `src/backend/modules/game/tests/integration/boss_disconnect_leave_test.go`

### Implementation cho User Story 1

- [x] T019 [P] [US1] Domain `boss` (encounter, phase, contribution, eligibility) — file: `src/backend/modules/game/internal/domain/boss/`
- [x] T020 [US1] Command `CreateBossEncounter`, `StartPartyActivity` (nhánh boss), `MarkEncounterReady`, `SubmitBossAnswer`, `AdvanceBossPhase`, `CompleteBossEncounter`, `LeaveBossEncounter`; query `ListPartyActivityCandidates`, `CheckPartyActivityEligibility`, `GetBossEncounter` — file: `src/backend/modules/game/internal/application/command/`, `src/backend/modules/game/internal/application/query/`
- [x] T021 [US1] Roomhost adapter (lease, checkpoint, timer phase/grace) + publisher `ew:rt:room:<id>:out`, `ew:rt:user:<id>` — file: `src/backend/modules/game/internal/infrastructure/realtime/boss_room.go`
- [x] T022 [US1] Handler public `createBossEncounter`, `getBossEncounter`, `leaveBossEncounter`; internal `party_activities.go`; NATS `boss.go`; consumer `party_membership.go` — file: `src/backend/modules/game/internal/presentation/http/public/bosses.go`, `src/backend/modules/game/internal/presentation/http/internal/party_activities.go`, `src/backend/modules/game/internal/presentation/consumer/party_membership.go`

**Checkpoint**: boss solo/party chạy độc lập.

## Phase 4: User Story 2 — Quest co-op với tiến độ chung (Priority: P1)

### Tests cho User Story 2

- [x] T023 [P] [US2] Unit test domain `PartyQuestSession` (F103-AC-14, F103-AC-15): ba policy, bước khung chia sẻ chỉ cùng instance, bước học tập không chia sẻ, `collective_target` + `min_contribution` — file: `src/backend/modules/game/tests/unit/party_quest_session_test.go` — phải FAIL trước khi code
- [x] T024 [P] [US2] Test F103-AC-13: `startPartyActivity` quest tạo/gắn `QuestProgress` ghim version + `party_id`, một phiên chung, idempotent — file: `src/backend/modules/game/tests/integration/start_party_quest_test.go`
- [x] T025 [P] [US2] Test F103-AC-16: không sao chép attempt/điểm/mastery; mỗi attempt gắn đúng `student_id` (TC-PTY-010) — file: `src/backend/modules/game/tests/integration/party_quest_individual_learning_test.go`
- [x] T026 [P] [US2] Test F103-AC-17, F103-AC-18, F103-AC-19: `quest.completed` từng người + một `quest.party_completed`; bước `boss` chỉ cho `eligible`; rời/disconnect giữ tiến độ; `party.disbanded` áp `reset_step`/`fail_quest` — file: `src/backend/modules/game/tests/integration/party_quest_completion_test.go`

### Implementation cho User Story 2

- [x] T027 [P] [US2] Domain `partyquest` (session, shared_progress) — file: `src/backend/modules/game/internal/domain/partyquest/session.go`, `src/backend/modules/game/internal/domain/partyquest/shared_progress.go`
- [x] T028 [US2] Command `StartPartyActivity` (nhánh quest), `ApplySharedStep` (gọi từ `ReportQuestStep` của F072), `RecordCollectiveContribution` (từ consumer `activity.graded`), `CompletePartyQuest`, `ApplyPartyMembershipChange`; consumer `boss_quest_step.go` — file: `src/backend/modules/game/internal/application/command/`, `src/backend/modules/game/internal/presentation/consumer/boss_quest_step.go`

## Phase 5: User Story 3 — Bước `interdependent` (Priority: P2)

### Tests cho User Story 3

- [x] T029 [P] [US3] Unit test F103-AC-20, F103-AC-24: chia role `auto`, `leader_assigns`, nhiều role khi thiếu người, chuyển role khi disconnect, solo giữ mọi role — file: `src/backend/modules/game/tests/unit/interdependent_roles_test.go` — phải FAIL trước khi code
- [x] T030 [P] [US3] Test F103-AC-21, F103-AC-22: clue chỉ của role mình + đã chia sẻ, không đáp án, không lộ thẻ chưa chia sẻ trong mọi response/message; share sai role → `403`; share lặp không trùng — file: `src/backend/modules/game/tests/integration/quest_clues_test.go`
- [x] T031 [P] [US3] Test F103-AC-23: câu cuối tạo attempt riêng; `completion` `all_members`/`min_members`; không đạt `pass_rule` không thưởng — file: `src/backend/modules/game/tests/integration/interdependent_final_question_test.go`

### Implementation cho User Story 3

- [x] T032 [P] [US3] Domain `roles.go`, `clues.go` — file: `src/backend/modules/game/internal/domain/partyquest/roles.go`, `src/backend/modules/game/internal/domain/partyquest/clues.go`
- [x] T033 [US3] Command `AssignStepRoles`, `ShareStepClue`; query `GetStepClues` + clue cache; handler `getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles` — file: `src/backend/modules/game/internal/application/command/share_step_clue.go`, `src/backend/modules/game/internal/infrastructure/cache/clue_cache.go`, `src/backend/modules/game/internal/presentation/http/public/quest_clues.go`

## Phase 6: User Story 4 — Phân quyền, cô lập tenant, giới hạn đầu vào (Priority: P1)

### Tests cho User Story 4

- [x] T034 [P] [US4] Test tenant isolation + authorization F103-AC-25, F103-AC-27: route người chơi × token nhân sự → `403`; encounter/phiên/thẻ tenant khác hoặc không là thành viên → `404`; lệnh WS `boss.*` tenant khác → `PERMISSION_DENIED` (sinh từ OpenAPI/AsyncAPI) — file: `src/backend/modules/game/tests/integration/boss_coop_authz_tenant_test.go` — phải FAIL trước khi code
- [x] T035 [P] [US4] Test F103-AC-26: fixture module `chat` import facade party-activity → `make arch-check` fail (TC-SVC-005), thành viên khác tenant của `TenantScope` → `422` — file: `src/backend/modules/game/tests/integration/party_activities_facade_test.go`
- [x] T036 [P] [US4] Test F103-AC-28: thiếu `Idempotency-Key` → `428`; chuỗi vượt `maxLength` → `422` (TC-TEN-024) — file: `src/backend/modules/game/tests/integration/boss_coop_validation_test.go`

### Implementation cho User Story 4

- [x] T037 [US4] Hiện thực facade party-activity kiểm tenant thành viên theo TenantScope trong context; Principal cho NATS command từ header gateway — file: `src/backend/modules/game/internal/presentation/facade/party_activities.go`, `src/backend/modules/game/internal/presentation/natscmd/principal.go`

## Phase 7: Polish & Cross-Cutting

- [x] T038 [P] Sửa CHALLENGE_BOSS_SYSTEM §7, §8; EVENT_DESIGN §4 (`quest.party_completed`, consumer competition của `boss.encounter_completed`); REALTIME_ARCHITECTURE §4.1; VALKEY_DESIGN (room `boss`, `ew:game:clue:*`); DATABASE_DESIGN §2.8a (cột/bảng mới); MULTIPLAYER_GAMEPLAY §9.2 (`allow_continue_on_disconnect` → quy tắc QUEST_SYSTEM §6); CHANGELOG — file: `docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md`, `docs/07-backend/EVENT_DESIGN.md`, `docs/06-architecture/REALTIME_ARCHITECTURE.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/DATABASE_DESIGN.md`
- [x] T039 [P] Metric `eduworld_boss_*`, `eduworld_party_quest_*` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`
- [ ] T040 [P] E2E boss party 2 client headless: ready → 2 phase → hạ boss → thưởng; ngắt kết nối + takeover (TC-CHL-004, TC-CHL-007) — file: `tests/e2e/boss_party_test.go`
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T006 cần nhóm D merge trước T015) → US1 → US2 → US3; US4 song song sau US1 → Polish.
- Thứ tự liên module (R29: module nhận trước): T005 + T014 (provider) trước khi F101 bật `quest`/`boss`; F072, F074, F076 phải Done.

## Definition of Done

- [x] Mọi tiêu chí F103-AC-* có test pass (ghi mã test/TC) — bảng dưới
- [x] gen-check, lint, test, test-integration, migrate (golden DDL) xanh cho module `game` (+ learning, competition); `make test-e2e` chưa có harness (T040)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — double chỉ trong test, gắn nhãn TEST DOUBLE
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Ghi chú triển khai (2026-11-24)

- Vị trí file khác plan: domain test cạnh package (`internal/domain/boss/encounter_test.go`, `internal/domain/partyquest/session_test.go`); integration `tests/integration/boss_us1_test.go`, `boss_party_test.go`, `party_quest_test.go`, `rls_boss_coop_test.go`; contract `tests/contract/boss_coop_test.go`; persistence `persistence/bosssql`, `persistence/partyquestsql`, `persistence/questsql/party.go`; room `infrastructure/room/boss_room.go`; facade `presentation/facade/party_activities.go`; consumer `presentation/consumer/boss_coop.go`; learning `application/query/material_renders.go`, `tests/integration/question_plan_boss_test.go`.
- Quyết định khi triển khai: [spec.md Clarifications 2026-11-24 (triển khai)](spec.md#clarifications).
- T040 (E2E 2 client headless) chưa có harness E2E realtime trong repo — luồng được phủ bằng integration (lệnh NATS gọi qua `natscmd`, room timer gọi trực tiếp, Reward Engine/consumer nhận envelope thật). T041 (quickstart tay trên compose) chưa chạy.

## AC → test

| AC | Test (pass) |
|---|---|
| F103-AC-01 | `TestBossCreateSolo` |
| F103-AC-02 | `TestBossPartyFacade` |
| F103-AC-03 | `TestBossPartyFacade` |
| F103-AC-04 | `TestBossSoloPhasesAndRewards`, `TestBossPartyFlow`; unit `TestReadyStartsPhase`, `TestReadyTimeout`, `TestPerGradeActivity` |
| F103-AC-05 | `TestBossSoloPhasesAndRewards`, `TestBossPartyFlow`; learning `TestQuestionPlanBossAndMaterials` |
| F103-AC-06 | `TestBossSoloPhasesAndRewards` (BOSS_PHASE_CLOSED), `TestBossRetryTimeouts` (lượt cũ), `TestBossIsolationValidation` (ROOM_NOT_MEMBER) |
| F103-AC-07 | `TestBossSoloPhasesAndRewards`, `TestBossPartyFlow`; unit `TestCollectiveCountAndDefeat`, `TestAnyN`, `TestValidate` |
| F103-AC-08 | `TestBossRetryTimeouts`; unit `TestTimeoutRetry` |
| F103-AC-09 | `TestBossPartyFlow` (không đúng/sai của người khác, chỉ `contribution_met`), `TestBossSoloPhasesAndRewards` (câu hỏi không đáp án) |
| F103-AC-10 | `TestBossSoloPhasesAndRewards` (một kết quả, Reward Engine giao lặp 3 lần → thưởng một lần), `TestBossPartyFlow` (`eligible`, `party_activity_id`); unit `TestEligibility` |
| F103-AC-11 | `TestBossDisconnectLeave`; unit `TestPresenceAndLeave`; `pkg/roomhost` `TestHostReloadDueAfterTimer` |
| F103-AC-12 | `TestBossDisconnectLeave` (leave, party.member_left, party.disbanded), `TestBossRetryTimeouts` (lobby mồ côi) |
| F103-AC-13 | `TestPartyQuestShared` |
| F103-AC-14 | `TestPartyQuestShared`; unit `TestSharesStep` |
| F103-AC-15 | `TestPartyQuestCollective`; unit `TestCollective` |
| F103-AC-16 | `TestPartyQuestShared` |
| F103-AC-17 | `TestPartyQuestShared`, `TestPartyQuestCollective` |
| F103-AC-18 | `TestBossPartyFlow` |
| F103-AC-19 | `TestPartyQuestLeaveDisband`; unit `TestReassignAndContinue` |
| F103-AC-20 | `TestInterdependentStep`; unit `TestRoles` |
| F103-AC-21 | `TestInterdependentStep`; unit `TestClues`; learning `TestQuestionPlanBossAndMaterials` |
| F103-AC-22 | `TestInterdependentStep`; unit `TestClues` |
| F103-AC-23 | `TestInterdependentStep`; unit `TestInterdependentCompletion` |
| F103-AC-24 | `TestInterdependentStep`; unit `TestReassignAndContinue`, `TestRoles` |
| F103-AC-25 | `TestPlayerRoutesStaffDenied`; contract `TestNoClientBossResult` |
| F103-AC-26 | `TestBossPartyFacade` (thành viên tenant khác → 422); contract `TestPartyActivityCallers` (chat → CALLER_NOT_ALLOWED) |
| F103-AC-27 | `TestBossIsolationValidation`, `TestInterdependentStep` (tenant khác 404), `TestRLSBossCoopTables` |
| F103-AC-28 | `TestBossCreateSolo`, `TestBossPartyFacade`, `TestBossIsolationValidation` (428, 422 `maxLength`) |
