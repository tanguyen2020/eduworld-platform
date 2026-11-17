# Tasks: F107 — Tournament: 6 format, bracket, lịch, thử thách lớp/giải trường từ template

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `challenge`, `tournament`, `tournament/bracket`, `assignment`; đăng ký handler public, hiện thực facade, consumer, job `tournament-scheduler` (profile `worker-jobs`) — file: `src/backend/modules/competition/module.go` (nối vào `src/backend/cmd/core-api`, `src/backend/cmd/worker`)
- [ ] T002 [P] Seed template giải/thử thách + gán `th-demo-a` (local) — file: `src/backend/modules/competition/seed/tournament_templates.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta OpenAPI: chuyển `/lms/challenges`, `/lms/catalog/challenge-templates` sang `competition.v1.yaml` (`x-service: competition`), sửa `ChallengeCreate`/`Challenge`/`Tournament`; thêm `getChallenge`, `cancelChallenge`, `listMyChallenges`, `joinChallenge`, endpoint tournament (contracts §1) — file: `specs/openapi/modules/competition.v1.yaml`, `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`, `make gen MOD=learning`
- [ ] T004 [P] Route ngoại lệ APISIX `/api/v1/lms/challenges*`, `/api/v1/lms/catalog/challenge-templates` → module competition; API_DESIGN §9.14 — file: `deploy/apisix/routes/competition.yaml`, `docs/07-backend/API_DESIGN.md`
- [ ] T005 [P] AsyncAPI `tournament.match.scheduled`, `tournament.bracket.updated`; payload schema `tournament.match_scheduled.v1`, `tournament.completed.v1`, `tournament.rewarded.v1`; catalog EVENT_DESIGN §4.8 — file: `specs/asyncapi/eduworld-realtime.v1.yaml`, `specs/events/payloads/competition/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint`
- [ ] T006 ent schema `challenges`, `tournaments`, `tournament_entries`, `tournament_groups`, `bracket_nodes`, `tournament_results`, `tournament_rewards` (mọi chuỗi `MaxLen`, partial unique) — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f107_tournament`, `make migrate-lint MOD=competition`
- [ ] T007 [P] Cập nhật DATABASE_DESIGN §2.10/§2.16 — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — School Admin tạo thử thách lớp/giải trường từ template (Priority: P1) 🎯 MVP

**Goal**: Tạo/xem/hủy Challenge từ template đã gán.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T008 [P] [US1] Unit test F107-AC-01, F107-AC-02, F107-AC-05: `ParamFilter` (`allowed_params`, `reward` → lỗi), ghim version, kind → Tournament/Timed Quiz, validate lịch/maxLength — file: `src/backend/modules/competition/tests/unit/challenge/challenge_test.go` — phải FAIL trước khi code
- [ ] T009 [P] [US1] Integration test F107-AC-02, F107-AC-03 (TC-CMS-AUTHZ-07, TC-CMS-ASG-01): chưa gán → `403 RESOURCE_NOT_ASSIGNED`; AM/TA → `403`; lớp trường khác → `404`; System Admin thiếu tenant → `422` — file: `src/backend/modules/competition/tests/integration/challenge/create_authz_test.go`
- [ ] T010 [P] [US1] Integration test F107-AC-04, F107-AC-06: idempotent + audit; list/get/cancel đúng phạm vi — file: `src/backend/modules/competition/tests/integration/challenge/lifecycle_test.go`

### Implementation cho User Story 1

- [ ] T011 [P] [US1] Domain `Challenge`, `ParamFilter` — file: `src/backend/modules/competition/internal/domain/challenge/challenge.go`, `src/backend/modules/competition/internal/domain/challenge/params.go`
- [ ] T012 [US1] Port `TemplateReader` (version published + assignment) + command `CreateChallenge`, `CancelChallenge` — file: `src/backend/modules/competition/internal/application/port/template.go`, `src/backend/modules/competition/internal/application/command/challenge/`
- [ ] T013 [US1] Query `ListChallenges`, `GetChallenge`, `ListChallengeTemplates` + handler — file: `src/backend/modules/competition/internal/application/query/`, `src/backend/modules/competition/internal/presentation/http/public/challenge_handler.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Đăng ký, xác nhận, check-in (Priority: P1)

**Goal**: Vòng đời giải, entry, check-in.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T014 [P] [US2] Unit test F107-AC-07: state machine giải (chuyển hợp lệ/không hợp lệ) — file: `src/backend/modules/competition/tests/unit/tournament/lifecycle_test.go` — phải FAIL trước khi code
- [ ] T015 [P] [US2] Integration test F107-AC-08, F107-AC-09: đăng ký (phạm vi, hạn chế, `max_entries`, ngoài cửa sổ, idempotent), đăng ký hộ, confirm/auto-confirm, withdraw — file: `src/backend/modules/competition/tests/integration/tournament/entries_test.go`
- [ ] T016 [P] [US2] Integration test (đồng hồ giả) F107-AC-10: check-in, rút entry, bye, hủy khi dưới tối thiểu — file: `src/backend/modules/competition/tests/integration/tournament/check_in_test.go`

### Implementation cho User Story 2

- [ ] T017 [P] [US2] Domain `Tournament` state machine, `Eligibility` — file: `src/backend/modules/competition/internal/domain/tournament/tournament.go`, `src/backend/modules/competition/internal/domain/tournament/eligibility.go`
- [ ] T018 [US2] Command `Publish`, `Cancel`, `RegisterEntry`, `WithdrawEntry`, `ConfirmEntries`, `CheckIn`, `TransitionByTime` + handler — file: `src/backend/modules/competition/internal/application/command/tournament/`, `src/backend/modules/competition/internal/presentation/http/public/tournament_handler.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Seeding và sinh bracket (Priority: P1)

**Goal**: Bracket single elim/round robin bền vững.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T019 [P] [US3] Property test F107-AC-11, F107-AC-12: single elim và round robin N = 2..64 (slot, bye, hạt giống 1/2, không trùng cặp, standings) — file: `src/backend/modules/competition/tests/unit/tournament/bracket_test.go` — phải FAIL trước khi code
- [ ] T020 [P] [US3] Unit test F107-AC-13: seeding `rating`/`random` (seed tái lập)/`manual`/`separate_class` — file: `src/backend/modules/competition/tests/unit/tournament/seeding_test.go`
- [ ] T021 [P] [US3] Integration test F107-AC-14: một transaction, idempotent, `409` sau khi `scheduled`, `422` dưới tối thiểu, audit — file: `src/backend/modules/competition/tests/integration/tournament/generate_bracket_test.go`

### Implementation cho User Story 3

- [ ] T022 [P] [US3] Domain `bracket/single_elim.go`, `round_robin.go`, `seeding.go`, `standings.go` — file: `src/backend/modules/competition/internal/domain/tournament/bracket/`
- [ ] T023 [US3] Command `GenerateBracket` (dùng `RatingReader` của F106) — file: `src/backend/modules/competition/internal/application/command/tournament/generate_bracket.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Lịch, trận, xác nhận kết quả, trao thưởng (Priority: P1)

**Goal**: Tạo trận đúng giờ, no-show, đẩy người thắng, override, thưởng.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T024 [P] [US4] Integration test (đồng hồ giả) F107-AC-15, F107-AC-16, F107-AC-19: proposal trực tiếp đúng giờ, `tournament.match_scheduled` −10/−1 phút, no-show → forfeit, cả hai vắng, reschedule ≤ 2 — file: `src/backend/modules/competition/tests/integration/tournament/scheduler_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Integration test F107-AC-17, F107-AC-18: `match.completed` lặp → đẩy 1 lần, confirm sau cửa sổ; override lý do/audit/`409` sau khi nút kế bắt đầu — file: `src/backend/modules/competition/tests/integration/tournament/advance_override_test.go`
- [ ] T026 [P] [US4] Integration test F107-AC-20, F107-AC-21: kết quả, points `tournament`, thưởng idempotent `(tournament_id, entry_id, rule_id)`, `tournament.rewarded`, rating không đổi — file: `src/backend/modules/competition/tests/integration/tournament/rewards_test.go`

### Implementation cho User Story 4

- [ ] T027 [P] [US4] Domain `Advance`, `Forfeit`, `Override` — file: `src/backend/modules/competition/internal/domain/tournament/advance.go`, `src/backend/modules/competition/internal/domain/tournament/forfeit.go`, `src/backend/modules/competition/internal/domain/tournament/override.go`
- [ ] T028 [US4] Command `ScheduleDueNodes` (gọi `CreateDirectProposal` F104), `ApplyNoShow`, job `tournament-scheduler` (lock singleton, `FOR UPDATE SKIP LOCKED`) — file: `src/backend/modules/competition/internal/application/command/tournament/schedule_due_nodes.go`, `src/backend/modules/competition/internal/presentation/job/tournament_scheduler.go`
- [ ] T029 [US4] Command `AdvanceOnMatch` + consumer `match.completed`; `ConfirmNode`, `OverrideNode`; `CompleteTournament`, `AwardRewards` + handler — file: `src/backend/modules/competition/internal/application/command/tournament/`, `src/backend/modules/competition/internal/presentation/consumer/match_completed_tournament.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Format mở rộng: double elim, vòng bảng, giải đội, giải trường (Priority: P3)

**Goal**: Format P6 theo PRD, bật theo template.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T030 [P] [US5] Property test F107-AC-22, F107-AC-23: double elim ánh xạ nhánh thua, bracket reset tắt; groups snake 4–6 bảng, top 2 — file: `src/backend/modules/competition/tests/unit/tournament/bracket_extended_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US5] Integration test F107-AC-24: entry đội (cùng trường, đủ điều kiện), thiếu người → forfeit, phạm vi trường/tenant — file: `src/backend/modules/competition/tests/integration/tournament/team_entries_test.go`

### Implementation cho User Story 5

- [ ] T032 [US5] Domain `bracket/double_elim.go`, `groups.go`; nhánh team trong `RegisterEntry`/`ApplyNoShow` — file: `src/backend/modules/competition/internal/domain/tournament/bracket/double_elim.go`, `src/backend/modules/competition/internal/domain/tournament/bracket/groups.go`, `src/backend/modules/competition/internal/application/command/tournament/register_entry.go`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: User Story 6 — Học sinh xem giải, tham gia Class Challenge; gán template (Priority: P1)

**Goal**: View học sinh, join Timed Quiz, phân quyền, gán template.

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [ ] T033 [P] [US6] Test tenant isolation + authorization F107-AC-25, F107-AC-26 (TC-AUTH-014, TC-TEN-001, TC-TEN-011): mọi endpoint mới × {học sinh trong/ngoài phạm vi, tenant khác, nhân sự token game, học sinh gọi route tổ chức, AM, header sai}; chỉ nickname với học sinh — file: `src/backend/modules/competition/tests/integration/tournament/authz_tenant_test.go` — phải FAIL trước khi code
- [ ] T034 [P] [US6] Integration test F107-AC-27: `listMyChallenges`, `joinChallenge` trong/ngoài khung, không thuộc đối tượng — file: `src/backend/modules/competition/tests/integration/challenge/join_test.go`
- [ ] T035 [P] [US6] Test F107-AC-28: quét schema template/`allowed_params` không có khóa tắt chat; log khiếu nại không chứa chat — file: `src/backend/modules/competition/tests/unit/challenge/no_chat_toggle_test.go`
- [ ] T036 [P] [US6] Integration + facade test F107-AC-29 (TC-SVC-006): `assignTenantResources`/`unassignTenantResources`/`listTenantResourceAssignments` idempotent, audit, module ngoài `x-allowed-callers` import facade → `make arch-check` fail — file: `src/backend/modules/competition/tests/integration/assignment/assignment_test.go`

### Implementation cho User Story 6

- [ ] T037 [US6] Query `ListTournaments`, `GetTournament`, `GetTournamentBracket`, `ListMyChallenges`; command `JoinChallenge` (gọi `JoinTimedQuizRoom` F105) + handler — file: `src/backend/modules/competition/internal/application/query/`, `src/backend/modules/competition/internal/application/command/challenge/join_challenge.go`
- [ ] T038 [US6] Command `AssignTemplate`, `UnassignTemplate`; handler internal — file: `src/backend/modules/competition/internal/application/command/assignment/`, `src/backend/modules/competition/internal/presentation/http/internal/assignment_handler.go`

**Checkpoint**: User Story 6 đạt.

## Phase 9: Polish & Cross-Cutting

- [ ] T039 [P] Metric/span theo plan.md §Quan sát — file: `src/backend/modules/competition/internal/infrastructure/observability/tournament_metrics.go`
- [ ] T040 [P] Ghi mâu thuẫn `/lms/challenges` (module sở hữu), Tenant Admin tạo giải liên trường, kiểm tên domain event `tournament.match_scheduled` (RB-412, tài liệu sở hữu đã sửa); CHANGELOG; trạng thái F107 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/107-tournament-challenges/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T003–T007) → US1 → US2 → US3 → US4 → US6 → US5 (P3) → Polish.
- T028 cần F104 `CreateDirectProposal`; T037 cần F105 `JoinTimedQuizRoom`; T023 cần F106 `RatingReader`. T018, T032 cùng sửa `register_entry.go`: tuần tự. F108 dựng UI trên T013, T018, T023, T029.

## Definition of Done

- [ ] Mọi tiêu chí F107-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
