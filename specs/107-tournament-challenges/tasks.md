# Tasks: F107 — Tournament: 6 format, bracket, lịch, thử thách khối/giải trường từ template

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `challenge`, `tournament`, `tournament/bracket`, `assignment`; đăng ký handler public, hiện thực facade, consumer, job `tournament-scheduler` (profile `worker-jobs`) — file: `src/backend/modules/competition/module.go` (nối vào `src/backend/cmd/core-api`, `src/backend/cmd/worker`) — ✅ `module.go` (Tournaments, TemplateAssignments, facade, consumer `competition.tournament_match`, job `tournament-scheduler`), `cmd/internal/wiring/compose.go` (`ChatScreen`), test `TestWorkerProfilesRegisterRelaysConsumersAndJobs`
- [x] T002 [P] Seed template giải/thử thách + gán `th-demo-a` (local) — file: `src/backend/modules/competition/seed/tournament_templates.json` — ✅ `seed/tournament_templates.json` (5 template: grade single elim, grade round robin, grade Timed Quiz, school team groups, school double elim) = literal của migration (`TestF107SeedMatchesMigration`); gán `th-demo-a` ở local qua saga F045 (`/api/v1/admin/tenant-assignments` → facade `applyTenantAssignments`), không seed dữ liệu tenant trong migration

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Delta OpenAPI: chuyển `/lms/challenges`, `/lms/catalog/challenge-templates` sang `competition.v1.yaml` (`x-service: competition`), sửa `ChallengeCreate`/`Challenge`/`Tournament` (gồm delta ADR-0095: `school_id`, `targets` khối ± học sinh, `exclude_student_ids`, `kind` `grade_challenge`, `listChallenges` lọc `grade_level` — sau khi WP-L1 merge, contracts §6); thêm `getChallenge`, `cancelChallenge`, `listMyChallenges`, `joinChallenge`, endpoint tournament (contracts §1) — file: `specs/openapi/modules/competition.v1.yaml`, `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`, `make gen MOD=learning` — ✅ spec gộp + `split-public` (19 op, `x-service: competition` cho `/lms/challenges*`, `/lms/catalog/challenge-templates`), `make gen` (facadegen/allowedcallers/routematrix), `specs-breaking` 3 ngoại lệ ADR-0074 ở `specs/breaking-allowlist.yaml`
- [x] T004 [P] Route ngoại lệ APISIX `/api/v1/lms/challenges*`, `/api/v1/lms/catalog/challenge-templates` → module competition; API_DESIGN §9.14 — file: `deploy/apisix/routes/competition.yaml`, `docs/07-backend/API_DESIGN.md` — ✅ không cần route APISIX mới: route `portal-tenant` (`/api/v1/lms/*`) và `player` (`/api/v1/competition/*`) đã liệt kê module competition trên upstream duy nhất `core-api` (ADR-0074); API_DESIGN §9.14 + bảng route ngoại lệ cập nhật
- [x] T005 [P] AsyncAPI `tournament.match.scheduled`, `tournament.bracket.updated`; payload schema `tournament.match_scheduled.v1`, `tournament.completed.v1`, `tournament.rewarded.v1`; catalog EVENT_DESIGN §4.8 — file: `specs/asyncapi/eduworld-realtime.v1.yaml`, `specs/events/payloads/competition/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint` — ✅ AsyncAPI `tournament.match.scheduled`, `tournament.bracket.updated` (`sendTournamentEvents`); payload `tournament.match_scheduled.v1`, `tournament.completed.v1`, `tournament.rewarded.v1`; EVENT_DESIGN §4.8
- [x] T006 ent schema `challenges`, `tournaments`, `tournament_entries`, `tournament_groups`, `bracket_nodes`, `tournament_results`, `tournament_rewards` (mọi chuỗi `MaxLen`, partial unique) — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f107_tournament`, `make migrate-lint MOD=competition` — ✅ `ent/schema/tournament.go` (8 entity, mọi chuỗi `MaxLen`) + `migrations/20261231000006_f107_tournament.sql` (entddl + RLS/CHECK/FK/GRANT/view `_live`/seed) — `TestMigrationMatchesEnt` PASS, `atlas.sum` cập nhật
- [x] T007 [P] Cập nhật DATABASE_DESIGN §2.10/§2.16 — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ DATABASE_DESIGN §2.10 (`challenges`, `tournaments`, `tournament_entries`, `tournament_groups`, `bracket_nodes`, `tournament_results`, `tournament_rewards`), §3.0 assignment competition

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — School Admin tạo thử thách khối/giải trường từ template (Priority: P1) 🎯 MVP

**Goal**: Tạo/xem/hủy Challenge từ template đã gán.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T008 [P] [US1] Unit test F107-AC-01, F107-AC-02, F107-AC-05: `ParamFilter` (`allowed_params`, `reward` → lỗi), ghim version, kind `grade_challenge`/`school_tournament` → Tournament/Timed Quiz (từ chối `class_*`), validate lịch/maxLength, `TargetResolver` (khối ± `student_ids` − `exclude_student_ids`, exclude thắng, ≤ 500, học sinh khác trường → `422`) — file: `src/backend/modules/competition/tests/unit/challenge/challenge_test.go` — phải FAIL trước khi code — ✅ `tests/unit/trn_challenge_test.go` `TestChallengeTemplateAndRequest` PASS
- [x] T009 [P] [US1] Integration test F107-AC-02, F107-AC-03 (TC-CMS-AUTHZ-07, TC-CMS-ASG-01): chưa gán → `403 RESOURCE_NOT_ASSIGNED`; AM/TA → `403`; `school_id` trường khác → `404`; System Admin thiếu tenant → `422` — file: `src/backend/modules/competition/tests/integration/challenge/create_authz_test.go` — ✅ `tests/integration/trn_challenge_test.go` `TestF107CreateChallenge`, `TestF107ChallengeAuthz` PASS
- [x] T010 [P] [US1] Integration test F107-AC-04, F107-AC-06: idempotent + audit; list/get/cancel đúng phạm vi — file: `src/backend/modules/competition/tests/integration/challenge/lifecycle_test.go` — ✅ `TestF107CreateChallenge` (Idempotency-Key + 1 audit), `TestF107ChallengeLifecycle` PASS

### Implementation cho User Story 1

- [x] T011 [P] [US1] Domain `Challenge`, `ParamFilter`, `TargetResolver` (đối tượng khối ± học sinh, ADR-0095; dữ liệu học sinh qua port `StudentReader` đọc `ref.students`) — file: `src/backend/modules/competition/internal/domain/challenge/challenge.go`, `src/backend/modules/competition/internal/domain/challenge/params.go` — ✅ `internal/domain/challenge/template.go`, `request.go` (ParamFilter = `FilterParams`, TargetResolver = `Audience`/`InAudience`; học sinh đọc qua `Students` của repo trên `ref.students`)
- [x] T012 [US1] Port `TemplateReader` (version published + assignment) + command `CreateChallenge`, `CancelChallenge` — file: `src/backend/modules/competition/internal/application/port/template.go`, `src/backend/modules/competition/internal/application/command/challenge/` — ✅ `port/write/tournament.go` (`Template`, `TemplateVersion`), `command/tournament.go` `CreateChallenge`, `CancelChallenge`
- [x] T013 [US1] Query `ListChallenges`, `GetChallenge`, `ListChallengeTemplates` + handler — file: `src/backend/modules/competition/internal/application/query/`, `src/backend/modules/competition/internal/presentation/http/public/challenge_handler.go` — ✅ `query/tournament.go` `ListChallenges`, `GetChallenge`, `ListChallengeTemplates` (keyset trong SQL) + `presentation/http/tournament.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Đăng ký, xác nhận, check-in (Priority: P1)

**Goal**: Vòng đời giải, entry, check-in.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T014 [P] [US2] Unit test F107-AC-07: state machine giải (chuyển hợp lệ/không hợp lệ) — file: `src/backend/modules/competition/tests/unit/tournament/lifecycle_test.go` — phải FAIL trước khi code — ✅ `tests/unit/trn_rules_test.go` `TestTournamentLifecycle` PASS
- [x] T015 [P] [US2] Integration test F107-AC-08, F107-AC-09: đăng ký (phạm vi, hạn chế, `max_entries`, ngoài cửa sổ, idempotent), đăng ký hộ, confirm/auto-confirm, withdraw — file: `src/backend/modules/competition/tests/integration/tournament/entries_test.go` — ✅ `tests/integration/trn_entries_test.go` `TestF107Entries` PASS
- [x] T016 [P] [US2] Integration test (đồng hồ giả) F107-AC-10: check-in, rút entry, bye, hủy khi dưới tối thiểu — file: `src/backend/modules/competition/tests/integration/tournament/check_in_test.go` — ✅ `TestF107CheckIn` PASS

### Implementation cho User Story 2

- [x] T017 [P] [US2] Domain `Tournament` state machine, `Eligibility` — file: `src/backend/modules/competition/internal/domain/tournament/tournament.go`, `src/backend/modules/competition/internal/domain/tournament/eligibility.go` — ✅ `internal/domain/tournament/tournament.go` (vòng đời, tối thiểu format, timers); eligibility = `visible` (command) + `challenge.InAudience`
- [x] T018 [US2] Command `Publish`, `Cancel`, `RegisterEntry`, `WithdrawEntry`, `ConfirmEntries`, `CheckIn`, `TransitionByTime` + handler — file: `src/backend/modules/competition/internal/application/command/tournament/`, `src/backend/modules/competition/internal/presentation/http/public/tournament_handler.go` — ✅ `command/tournament_entries.go` (Register/Withdraw/Confirm/CheckIn), `command/tournament.go` (Publish/Cancel), `command/tournament_jobs.go` (mở/đóng đăng ký, auto-confirm, đóng check-in)

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Seeding và sinh bracket (Priority: P1)

**Goal**: Bracket single elim/round robin bền vững.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T019 [P] [US3] Property test F107-AC-11, F107-AC-12: single elim và round robin N = 2..64 (slot, bye, hạt giống 1/2, không trùng cặp, standings) — file: `src/backend/modules/competition/tests/unit/tournament/bracket_test.go` — phải FAIL trước khi code — ✅ `tests/unit/trn_bracket_test.go` `TestTournamentSingleElimN2to64`, `TestTournamentSingleElimFive`, `TestTournamentRoundRobinN2to64`, `TestTournamentStandings` PASS
- [x] T020 [P] [US3] Unit test F107-AC-13: seeding `rating`/`random` (seed tái lập)/`manual`; `separate_class` bị từ chối (ADR-0095) — file: `src/backend/modules/competition/tests/unit/tournament/seeding_test.go` — ✅ `TestTournamentSeeding` PASS
- [x] T021 [P] [US3] Integration test F107-AC-14: một transaction, idempotent, `409` sau khi `scheduled`, `422` dưới tối thiểu, audit — file: `src/backend/modules/competition/tests/integration/tournament/generate_bracket_test.go` — ✅ `TestF107GenerateBracket` PASS

### Implementation cho User Story 3

- [x] T022 [P] [US3] Domain `bracket/single_elim.go`, `round_robin.go`, `seeding.go`, `standings.go` — file: `src/backend/modules/competition/internal/domain/tournament/bracket/` — ✅ `internal/domain/tournament/bracket/bracket.go`, `seeding.go`; standings/ranks ở `internal/domain/tournament/standings.go`; dựng graph `build.go`
- [x] T023 [US3] Command `GenerateBracket` (dùng `RatingReader` của F106) — file: `src/backend/modules/competition/internal/application/command/tournament/generate_bracket.go` — ✅ `command/tournament_bracket.go` `GenerateTournamentBracket` (RatingReader + Ranking Rules của F106 cho seeding `rating`)

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Lịch, trận, xác nhận kết quả, trao thưởng (Priority: P1)

**Goal**: Tạo trận đúng giờ, no-show, đẩy người thắng, override, thưởng.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T024 [P] [US4] Integration test (đồng hồ giả) F107-AC-15, F107-AC-16, F107-AC-19: proposal trực tiếp đúng giờ, `tournament.match_scheduled` −10/−1 phút, no-show → forfeit, cả hai vắng, reschedule ≤ 2 — file: `src/backend/modules/competition/tests/integration/tournament/scheduler_test.go` — phải FAIL trước khi code — ✅ `tests/integration/trn_match_test.go` `TestF107Scheduler`, `TestF107Reschedule` PASS
- [x] T025 [P] [US4] Integration test F107-AC-17, F107-AC-18: `match.completed` lặp → đẩy 1 lần, confirm sau cửa sổ; override lý do/audit/`409` sau khi nút kế bắt đầu — file: `src/backend/modules/competition/tests/integration/tournament/advance_override_test.go` — ✅ `TestF107AdvanceOverride` PASS
- [x] T026 [P] [US4] Integration test F107-AC-20, F107-AC-21: kết quả, points `tournament`, thưởng idempotent `(tournament_id, entry_id, rule_id)`, `tournament.rewarded`, rating không đổi — file: `src/backend/modules/competition/tests/integration/tournament/rewards_test.go` — ✅ `TestF107Rewards` PASS

### Implementation cho User Story 4

- [x] T027 [P] [US4] Domain `Advance`, `Forfeit`, `Override` — file: `src/backend/modules/competition/internal/domain/tournament/advance.go`, `src/backend/modules/competition/internal/domain/tournament/forfeit.go`, `src/backend/modules/competition/internal/domain/tournament/override.go` — ✅ `internal/domain/tournament/graph.go` (`Apply`, `Withdraw`, `Override`, `Resolve`, `Confirm`, `RequireDecision`)
- [x] T028 [US4] Command `ScheduleDueNodes` (gọi `CreateDirectProposal` F104), `ApplyNoShow`, job `tournament-scheduler` (lock singleton, `FOR UPDATE SKIP LOCKED`) — file: `src/backend/modules/competition/internal/application/command/tournament/schedule_due_nodes.go`, `src/backend/modules/competition/internal/presentation/job/tournament_scheduler.go` — ✅ `command/tournament_jobs.go` (`Tick`, `startNode` → `CreateDirectProposal` với `Attempt`/`ReadyCheck`, `noShow`, `forfeit`), job `tournament-scheduler`; không dùng lock singleton Valkey: các replica tuần tự hóa bằng `FOR UPDATE` trên giải + id proposal xác định (leader election job toàn nền tảng còn [Proposed] ở pkg/module)
- [x] T029 [US4] Command `AdvanceOnMatch` + consumer `match.completed`; `ConfirmNode`, `OverrideNode`; `CompleteTournament`, `AwardRewards` + handler — file: `src/backend/modules/competition/internal/application/command/tournament/`, `src/backend/modules/competition/internal/presentation/consumer/match_completed_tournament.go` — ✅ `command/tournament_bracket.go` (`AdvanceOnMatch`, `MatchAborted`, `ConfirmBracketNode`, `OverrideBracketNode`, `complete`, `awardRewards`) + consumer `presentation/consumer/tournament.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Format mở rộng: double elim, vòng bảng, giải đội, giải trường (Priority: P3)

**Goal**: Format P6 theo PRD, bật theo template.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T030 [P] [US5] Property test F107-AC-22, F107-AC-23: double elim ánh xạ nhánh thua, bracket reset tắt; groups snake 4–6 bảng, top 2 — file: `src/backend/modules/competition/tests/unit/tournament/bracket_extended_test.go` — phải FAIL trước khi code — ✅ `TestTournamentDoubleElimMapping`, `TestTournamentGroupsSnake` PASS
- [x] T031 [P] [US5] Integration test F107-AC-24: entry đội (cùng trường, đủ điều kiện), thiếu người → forfeit, phạm vi trường/tenant — file: `src/backend/modules/competition/tests/integration/tournament/team_entries_test.go` — ✅ `tests/integration/trn_team_test.go` `TestF107TeamTournament` PASS

### Implementation cho User Story 5

- [x] T032 [US5] Domain `bracket/double_elim.go`, `groups.go`; nhánh team trong `RegisterEntry`/`ApplyNoShow` — file: `src/backend/modules/competition/internal/domain/tournament/bracket/double_elim.go`, `src/backend/modules/competition/internal/domain/tournament/bracket/groups.go`, `src/backend/modules/competition/internal/application/command/tournament/register_entry.go` — ✅ `bracket.DoubleElim`, `SnakeGroups`/`GroupStage`/`KnockoutSeeds`/`Knockout` (trong `bracket.go`); nhánh đội ở `registerSelf`, `startNode`, `noShow`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: User Story 6 — Học sinh xem giải, tham gia thử thách khối; gán template (Priority: P1)

**Goal**: View học sinh, join Timed Quiz, phân quyền, gán template.

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [x] T033 [P] [US6] Test tenant isolation + authorization F107-AC-25, F107-AC-26 (TC-AUTH-014, TC-TEN-001, TC-TEN-011): mọi endpoint mới × {học sinh trong/ngoài phạm vi, tenant khác, nhân sự token game, học sinh gọi route tổ chức, AM, header sai}; chỉ nickname với học sinh — file: `src/backend/modules/competition/tests/integration/tournament/authz_tenant_test.go` — phải FAIL trước khi code — ✅ `tests/integration/trn_access_test.go` `TestF107TournamentAccess` PASS (+ `authztest.Cover` cho cả 19 op, routematrix 0 vấn đề)
- [x] T034 [P] [US6] Integration test F107-AC-27: `listMyChallenges`, `joinChallenge` trong/ngoài khung, không thuộc đối tượng — file: `src/backend/modules/competition/tests/integration/challenge/join_test.go` — ✅ `TestF107JoinChallenge` PASS
- [x] T035 [P] [US6] Test F107-AC-28: quét schema template/`allowed_params` không có khóa tắt chat; log khiếu nại không chứa chat — file: `src/backend/modules/competition/tests/unit/challenge/no_chat_toggle_test.go` — ✅ `tests/unit/trn_challenge_test.go` `TestChallengeNoChatToggle` PASS
- [x] T036 [P] [US6] Integration + facade test F107-AC-29 (TC-SVC-006): `assignTenantResources`/`unassignTenantResources`/`listTenantResourceAssignments` idempotent, audit, module ngoài `x-allowed-callers` import facade → `make arch-check` fail — file: `src/backend/modules/competition/tests/integration/assignment/assignment_test.go` — ✅ `TestF107Assignments` PASS (facade thật + idempotency store); `api.AllowedCallers` = [identity], archgen/depguard chặn module khác

### Implementation cho User Story 6

- [x] T037 [US6] Query `ListTournaments`, `GetTournament`, `GetTournamentBracket`, `ListMyChallenges`; command `JoinChallenge` (gọi `JoinTimedQuizRoom` F105) + handler — file: `src/backend/modules/competition/internal/application/query/`, `src/backend/modules/competition/internal/application/command/challenge/join_challenge.go` — ✅ `query/tournament.go` `ListTournaments`, `GetTournament`, `GetTournamentBracket`, `ListMyChallenges`; `command/tournament.go` `JoinChallenge` (phòng Timed Quiz qua `Matchmaking.CreateChallengeMatch`, F105 T034)
- [x] T038 [US6] Command `AssignTemplate`, `UnassignTemplate`; handler internal — file: `src/backend/modules/competition/internal/application/command/assignment/`, `src/backend/modules/competition/internal/presentation/http/internal/assignment_handler.go` — ✅ `command/tournament_assign.go`, `query.Assignments`, `presentation/facade/assignments.go`; module identity: `platformadmin` định tuyến `challenge_template` → competition, `owner_gateway.go` adapter competition

**Checkpoint**: User Story 6 đạt.

## Phase 9: Polish & Cross-Cutting

- [x] T039 [P] Metric/span theo plan.md §Quan sát — file: `src/backend/modules/competition/internal/infrastructure/observability/tournament_metrics.go` — ✅ `infrastructure/observability/tournament.go` (4 metric của plan); span `observability.UseCase` mọi use case
- [x] T040 [P] Ghi mâu thuẫn `/lms/challenges` (module sở hữu), Tenant Admin tạo giải liên trường, kiểm tên domain event `tournament.match_scheduled` (RB-412, tài liệu sở hữu đã sửa); CHANGELOG; trạng thái F107 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — ✅ CHANGELOG, CODE_PLAN (F107 Done), spec Clarifications 2026-11-24; mâu thuẫn Tenant Admin/giải liên trường giữ theo ADMIN_PORTAL (Assumptions); RB-412 tên domain event `tournament.match_scheduled` / message WS `tournament.match.scheduled` đã áp
- [x] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/107-tournament-challenges/quickstart.md` — ✅ quickstart chạy bằng test integration testcontainers (PostgreSQL + Valkey thật, đồng hồ giả) — ánh xạ kịch bản → test ở bảng dưới; chạy compose end-to-end với Godot thuộc F109

## Dependencies & Execution Order

- Setup → Foundational (T003–T007) → US1 → US2 → US3 → US4 → US6 → US5 (P3) → Polish.
- T028 cần F104 `CreateDirectProposal`; T037 cần F105 `JoinTimedQuizRoom`; T023 cần F106 `RatingReader`. T018, T032 cùng sửa `register_entry.go`: tuần tự. F108 dựng UI trên T013, T018, T023, T029.

## Definition of Done

- [x] Mọi tiêu chí F107-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Bằng chứng AC → test (2026-11-24)

| Tiêu chí | Test (PASS) |
|---|---|
| F107-AC-01 | `TestF107CreateChallenge` (template đã gán → Challenge + Tournament draft ghim version; Timed Quiz scheduled; template class → 422), `TestChallengeTemplateAndRequest` |
| F107-AC-02 | `TestF107CreateChallenge` (`params.reward` → 422 nêu khóa; chưa gán → 403 RESOURCE_NOT_ASSIGNED), `TestF107Assignments` (gỡ gán → 403), `TestChallengeTemplateAndRequest` |
| F107-AC-03 | `TestF107ChallengeAuthz` (AM/TA 403, trường khác/tenant khác 404, System Admin thiếu tenant 422, có tenant 201, header lệch 403) |
| F107-AC-04 | `TestF107CreateChallenge` (Idempotency-Key replay, 1 audit `challenge.created` có version template), `TestF107ChallengeLifecycle` (`challenge.cancelled`, `tournament.cancelled`), `TestF107Entries` (`tournament.published`) |
| F107-AC-05 | `TestF107CreateChallenge` (name > 80, ends ≤ starts, lịch ngoài template, targets rỗng, học sinh trường khác, khối 10, > 500 loại trừ, study_group, giờ truy cập trường), `TestChallengeTemplateAndRequest` (đối tượng = khối ∪ danh sách − loại trừ) |
| F107-AC-06 | `TestF107ChallengeLifecycle` (list keyset + lọc khối, get theo scope, cancel soft, cancel lại 422, không event học tập) |
| F107-AC-07 | `TestTournamentLifecycle`, `TestF107Entries` (generate trước seeding → 422) |
| F107-AC-08 | `TestF107Entries` (đăng ký + replay, 409 trùng, 404 ngoài phạm vi/tenant khác, 403 POLICY_DENIED, 409 đủ chỗ) |
| F107-AC-09 | `TestF107Entries` (đăng ký hộ theo đối tượng, không entry cho học sinh loại trừ, confirm, auto-confirm, withdraw soft) |
| F107-AC-10 | `TestF107CheckIn` (ngoài cửa sổ 422, entry người khác 404, không check-in → withdrawn + bye, dưới tối thiểu → cancelled + audit) |
| F107-AC-11 | `TestTournamentSingleElimN2to64`, `TestTournamentSingleElimFive`, `TestF107GenerateBracket` |
| F107-AC-12 | `TestTournamentRoundRobinN2to64`, `TestTournamentStandings`, `TestF107GenerateBracket` (vòng tròn 5 → 10 trận) |
| F107-AC-13 | `TestTournamentSeeding` (rating/provisional, random tái lập, manual, separate_class → 422), `TestF107GenerateBracket` (seed lưu) |
| F107-AC-14 | `TestF107GenerateBracket` (một transaction, replay cùng key, sau scheduled 422, dưới tối thiểu 422, audit) |
| F107-AC-15 | `TestF107Scheduler` (notice −10/−1 mỗi mốc một lần + push, proposal tới giờ, chạy lại không tạo thêm, trận có `tournament_node_id`) |
| F107-AC-16 | `TestF107Scheduler` (một bên vắng → forfeit, hai bên vắng → void), `TestTournamentByesAndForfeits` |
| F107-AC-17 | `TestF107AdvanceOverride` (match.completed lặp → đẩy một lần, hết cửa sổ → confirmed), `TestF107Rewards` (chung kết → completed + results) |
| F107-AC-18 | `TestF107AdvanceOverride` (override có lý do + audit trước/sau, thiếu lý do 422, nút kế đã bắt đầu 422), `TestTournamentOverride` |
| F107-AC-19 | `TestF107Reschedule` (abort → lên lịch lại 2 lần với ready check mới, lần 3 chờ người tổ chức, override quyết) |
| F107-AC-20 | `TestF107Rewards` (8 thưởng + 8 `tournament.rewarded`, điểm `source_type = tournament`, chạy lại không cấp thêm, schema event) |
| F107-AC-21 | `TestF107Rewards` (không rating), `TestF107Scheduler` (trận `ranked = false`), bảng PostgreSQL |
| F107-AC-22 | `TestTournamentDoubleElimMapping` |
| F107-AC-23 | `TestTournamentGroupsSnake`, `TestF107TeamTournament` (2 bảng → knockout 4) |
| F107-AC-24 | `TestF107TeamTournament` (leader đăng ký, thành viên khác trường 422, không leader 403, đội thiếu người → forfeit) |
| F107-AC-25 | `TestF107TournamentAccess` (học sinh trong phạm vi chỉ nickname, ngoài phạm vi/draft/tenant khác 404, người tổ chức thấy giải + seed) |
| F107-AC-26 | `TestF107TournamentAccess` (nhân sự token game 403 GAME_ACCESS_STUDENT_ONLY, học sinh gọi route tổ chức 403, header lệch 403 TENANT_MISMATCH) |
| F107-AC-27 | `TestF107JoinChallenge` (list, join trong/ngoài khung, ngoài đối tượng 404, phòng ≤ 40 lúc starts_at, completed lúc ends_at) |
| F107-AC-28 | `TestChallengeNoChatToggle` |
| F107-AC-29 | `TestF107Assignments`, `TestRouting` (identity: `challenge_template` → competition) |
