# Tasks: F103 — Boss battle nhiều phase và quest co-op (shared progress, bước `interdependent`)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Adapter facade `learning` (`createQuestionPlan`, `scorePlanAnswer`, `getLearningMaterialRenders`) qua `src/backend/pkg/modcall` — file: `src/backend/modules/game/internal/infrastructure/modules/learning.go`
- [ ] T002 [P] Key builder `ew:rt:room:<encounter_id>*`, `ew:game:clue:*`; đăng ký room type `boss` với `src/backend/pkg/roomhost` — file: `src/backend/modules/game/internal/infrastructure/cache/keys.go`, `src/backend/modules/game/module.go`
- [ ] T003 [P] Đăng ký handler NATS `ew.cmd.game.boss.ready`, `ew.cmd.game.boss.answer` qua `src/backend/pkg/natsx` — file: `src/backend/modules/game/internal/presentation/natscmd/boss.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta `game.v1` (createBossEncounter chỉ solo, `BossEncounter` cột mới, `QuestDetail.step_type`, `QuestProgress.party_progress`, 3 operation clue/role) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [ ] T005 [P] Ba operation party-activity + enum `reason_code` + `x-allowed-callers: [competition]` — file: `specs/openapi/modules/game.internal.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game`
- [ ] T006 [P] Delta `learning.internal` (callers `game`, `QuestionPlanRequest` mở rộng, `getLearningMaterialRenders`) — phối hợp nhóm D — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T007 [P] AsyncAPI `boss.question.assigned`, `boss.answer.evaluated`, `quest.roles.assigned`, `quest.clue.shared`, `quest.progress.updated.party_progress`, `BossEncounterEnded.outcome` — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T008 [P] Payload schema `boss.encounter_started`, `boss.phase_completed`, `boss.encounter_completed`, `quest.party_completed` — file: `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [ ] T009 ent schema cột mới `BossEncounter`, `BossEncounterMember`; mới `BossPhasePlan`, `BossAnswerResult` (AppendOnly), `PartyQuestSession`, `PartyQuestContribution` với `MaxLen` — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f103_boss_coop`, `make migrate-lint MOD=game`
- [ ] T010 Migration: unique `party_activity_id`, unique `(plan_id, item_index, student_id)`, CHECK jsonb size/enum, RLS, `REVOKE UPDATE, DELETE, TRUNCATE` bảng TA — file: `src/backend/modules/game/migrations/`
- [ ] T011 [P] Test RLS/global filter bảng mới (tenant A không đọc B; thiếu scope → 0 dòng) — file: `src/backend/modules/game/tests/integration/rls_boss_coop_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Boss nhiều phase, solo và party (Priority: P1) 🎯 MVP

**Goal**: Encounter server-authoritative, đóng góp và thưởng đúng.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T012 [P] [US1] Unit test domain `BossEncounter`/`PhaseRule` (F103-AC-07, F103-AC-08): `all`/`any_n`/`collective_count` + `min_contribution`, retry giữ phase trước, vượt retry/hết giờ → `ended` — file: `src/backend/modules/game/tests/unit/boss_encounter_test.go` — phải FAIL trước khi code
- [ ] T013 [P] [US1] Test F103-AC-01: solo hợp lệ/không đủ điều kiện/`mode = party` → `422`; ghim version, hạn giờ server — file: `src/backend/modules/game/tests/integration/create_boss_encounter_test.go`
- [ ] T014 [P] [US1] Facade test phía bên nhận F103-AC-02, F103-AC-03 (ví dụ trong spec, bên gọi competition): candidates, eligibility (không lộ khối), `startPartyActivity` idempotent — file: `src/backend/modules/game/tests/contract/party_activities_provider_test.go`
- [ ] T015 [P] [US1] Test F103-AC-04, F103-AC-05: ready → phase, plan riêng theo `per_grade`, câu hỏi không có đáp án; answer lấy `student_id` từ header, gửi lặp tính một lần, learning record đúng người (stub learning MOCK + contract consumer) — file: `src/backend/modules/game/tests/integration/boss_phase_answer_test.go`
- [ ] T016 [P] [US1] Test F103-AC-06, F103-AC-09: `BOSS_PHASE_CLOSED`, `ROOM_NOT_MEMBER`; message tới người khác không có đúng/sai/số câu đúng — file: `src/backend/modules/game/tests/integration/boss_privacy_test.go`
- [ ] T017 [P] [US1] Test F103-AC-10: kết thúc một transaction, `eligible`, thưởng idempotent khi replay 3 lần (TC-CHL-006), `boss.encounter_completed` có `party_activity_id` — file: `src/backend/modules/game/tests/integration/complete_boss_encounter_test.go`
- [ ] T018 [P] [US1] Test F103-AC-11, F103-AC-12: grace, takeover thiết bị khác nhận lại trạng thái (checkpoint), dưới `party_size.min` → `abandoned`; leave/party event giữ đóng góp; encounter mồ côi → `abandoned` — file: `src/backend/modules/game/tests/integration/boss_disconnect_leave_test.go`

### Implementation cho User Story 1

- [ ] T019 [P] [US1] Domain `boss` (encounter, phase, contribution, eligibility) — file: `src/backend/modules/game/internal/domain/boss/`
- [ ] T020 [US1] Command `CreateBossEncounter`, `StartPartyActivity` (nhánh boss), `MarkEncounterReady`, `SubmitBossAnswer`, `AdvanceBossPhase`, `CompleteBossEncounter`, `LeaveBossEncounter`; query `ListPartyActivityCandidates`, `CheckPartyActivityEligibility`, `GetBossEncounter` — file: `src/backend/modules/game/internal/application/command/`, `src/backend/modules/game/internal/application/query/`
- [ ] T021 [US1] Roomhost adapter (lease, checkpoint, timer phase/grace) + publisher `ew:rt:room:<id>:out`, `ew:rt:user:<id>` — file: `src/backend/modules/game/internal/infrastructure/realtime/boss_room.go`
- [ ] T022 [US1] Handler public `createBossEncounter`, `getBossEncounter`, `leaveBossEncounter`; internal `party_activities.go`; NATS `boss.go`; consumer `party_membership.go` — file: `src/backend/modules/game/internal/presentation/http/public/bosses.go`, `src/backend/modules/game/internal/presentation/http/internal/party_activities.go`, `src/backend/modules/game/internal/presentation/consumer/party_membership.go`

**Checkpoint**: boss solo/party chạy độc lập.

## Phase 4: User Story 2 — Quest co-op với tiến độ chung (Priority: P1)

### Tests cho User Story 2

- [ ] T023 [P] [US2] Unit test domain `PartyQuestSession` (F103-AC-14, F103-AC-15): ba policy, bước khung chia sẻ chỉ cùng instance, bước học tập không chia sẻ, `collective_target` + `min_contribution` — file: `src/backend/modules/game/tests/unit/party_quest_session_test.go` — phải FAIL trước khi code
- [ ] T024 [P] [US2] Test F103-AC-13: `startPartyActivity` quest tạo/gắn `QuestProgress` ghim version + `party_id`, một phiên chung, idempotent — file: `src/backend/modules/game/tests/integration/start_party_quest_test.go`
- [ ] T025 [P] [US2] Test F103-AC-16: không sao chép attempt/điểm/mastery; mỗi attempt gắn đúng `student_id` (TC-PTY-010) — file: `src/backend/modules/game/tests/integration/party_quest_individual_learning_test.go`
- [ ] T026 [P] [US2] Test F103-AC-17, F103-AC-18, F103-AC-19: `quest.completed` từng người + một `quest.party_completed`; bước `boss` chỉ cho `eligible`; rời/disconnect giữ tiến độ; `party.disbanded` áp `reset_step`/`fail_quest` — file: `src/backend/modules/game/tests/integration/party_quest_completion_test.go`

### Implementation cho User Story 2

- [ ] T027 [P] [US2] Domain `partyquest` (session, shared_progress) — file: `src/backend/modules/game/internal/domain/partyquest/session.go`, `src/backend/modules/game/internal/domain/partyquest/shared_progress.go`
- [ ] T028 [US2] Command `StartPartyActivity` (nhánh quest), `ApplySharedStep` (gọi từ `ReportQuestStep` của F072), `RecordCollectiveContribution` (từ consumer `activity.graded`), `CompletePartyQuest`, `ApplyPartyMembershipChange`; consumer `boss_quest_step.go` — file: `src/backend/modules/game/internal/application/command/`, `src/backend/modules/game/internal/presentation/consumer/boss_quest_step.go`

## Phase 5: User Story 3 — Bước `interdependent` (Priority: P2)

### Tests cho User Story 3

- [ ] T029 [P] [US3] Unit test F103-AC-20, F103-AC-24: chia role `auto`, `leader_assigns`, nhiều role khi thiếu người, chuyển role khi disconnect, solo giữ mọi role — file: `src/backend/modules/game/tests/unit/interdependent_roles_test.go` — phải FAIL trước khi code
- [ ] T030 [P] [US3] Test F103-AC-21, F103-AC-22: clue chỉ của role mình + đã chia sẻ, không đáp án, không lộ thẻ chưa chia sẻ trong mọi response/message; share sai role → `403`; share lặp không trùng — file: `src/backend/modules/game/tests/integration/quest_clues_test.go`
- [ ] T031 [P] [US3] Test F103-AC-23: câu cuối tạo attempt riêng; `completion` `all_members`/`min_members`; không đạt `pass_rule` không thưởng — file: `src/backend/modules/game/tests/integration/interdependent_final_question_test.go`

### Implementation cho User Story 3

- [ ] T032 [P] [US3] Domain `roles.go`, `clues.go` — file: `src/backend/modules/game/internal/domain/partyquest/roles.go`, `src/backend/modules/game/internal/domain/partyquest/clues.go`
- [ ] T033 [US3] Command `AssignStepRoles`, `ShareStepClue`; query `GetStepClues` + clue cache; handler `getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles` — file: `src/backend/modules/game/internal/application/command/share_step_clue.go`, `src/backend/modules/game/internal/infrastructure/cache/clue_cache.go`, `src/backend/modules/game/internal/presentation/http/public/quest_clues.go`

## Phase 6: User Story 4 — Phân quyền, cô lập tenant, giới hạn đầu vào (Priority: P1)

### Tests cho User Story 4

- [ ] T034 [P] [US4] Test tenant isolation + authorization F103-AC-25, F103-AC-27: route người chơi × token nhân sự → `403`; encounter/phiên/thẻ tenant khác hoặc không là thành viên → `404`; lệnh WS `boss.*` tenant khác → `PERMISSION_DENIED` (sinh từ OpenAPI/AsyncAPI) — file: `src/backend/modules/game/tests/integration/boss_coop_authz_tenant_test.go` — phải FAIL trước khi code
- [ ] T035 [P] [US4] Test F103-AC-26: fixture module `chat` import facade party-activity → `make arch-check` fail (TC-SVC-005), thành viên khác tenant của `TenantScope` → `422` — file: `src/backend/modules/game/tests/integration/party_activities_facade_test.go`
- [ ] T036 [P] [US4] Test F103-AC-28: thiếu `Idempotency-Key` → `428`; chuỗi vượt `maxLength` → `422` (TC-TEN-024) — file: `src/backend/modules/game/tests/integration/boss_coop_validation_test.go`

### Implementation cho User Story 4

- [ ] T037 [US4] Hiện thực facade party-activity kiểm tenant thành viên theo TenantScope trong context; Principal cho NATS command từ header gateway — file: `src/backend/modules/game/internal/presentation/facade/party_activities.go`, `src/backend/modules/game/internal/presentation/natscmd/principal.go`

## Phase 7: Polish & Cross-Cutting

- [ ] T038 [P] Sửa CHALLENGE_BOSS_SYSTEM §7, §8; EVENT_DESIGN §4 (`quest.party_completed`, consumer competition của `boss.encounter_completed`); REALTIME_ARCHITECTURE §4.1; VALKEY_DESIGN (room `boss`, `ew:game:clue:*`); DATABASE_DESIGN §2.8a (cột/bảng mới); MULTIPLAYER_GAMEPLAY §9.2 (`allow_continue_on_disconnect` → quy tắc QUEST_SYSTEM §6); CHANGELOG — file: `docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md`, `docs/07-backend/EVENT_DESIGN.md`, `docs/06-architecture/REALTIME_ARCHITECTURE.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T039 [P] Metric `eduworld_boss_*`, `eduworld_party_quest_*` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`
- [ ] T040 [P] E2E boss party 2 client headless: ready → 2 phase → hạ boss → thưởng; ngắt kết nối + takeover (TC-CHL-004, TC-CHL-007) — file: `tests/e2e/boss_party_test.go`
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T006 cần nhóm D merge trước T015) → US1 → US2 → US3; US4 song song sau US1 → Polish.
- Thứ tự liên module (R29: module nhận trước): T005 + T014 (provider) trước khi F101 bật `quest`/`boss`; F072, F074, F076 phải Done.

## Definition of Done

- [ ] Mọi tiêu chí F103-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` xanh cho module `game`; `make test-contract`, `make test-e2e` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
