# Tasks: F100 — Party: tạo/tham gia/mời/kick/chuyển leader/giải tán, một party active, giới hạn

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Skeleton module competition (`module.go` đăng ký vào `src/backend/cmd/core-api` và `src/backend/cmd/worker`, `api/`, `internal/*`, `migrations/`, `src/backend/pkg/config`, outbox relay, rule depguard/arch-check) — file: `src/backend/modules/competition/` — xong: `module.go` đăng ký route party (core-api), consumer/job (worker), `Imports{Chat}`/`Wire`; `cmd/internal/wiring` compose + compose_test (job `party-invitation-expiry`)
- [x] T002 [P] Adapter facade `identity` (`resolvePrincipal`, cache Principal) và `chat` (`moderateDisplayText`) qua `src/backend/pkg/modcall` (TenantScope, Principal gốc trong context) — file: `src/backend/modules/competition/internal/infrastructure/modules/{identity,chat}.go` — xong phần chat: `internal/infrastructure/modules/chat.go` (`Moderator`, `LateModerator` qua `pkg/modcall`); `resolvePrincipal` không cần adapter riêng (Principal từ chuỗi `pkg/httpx`, Clarifications 2026-11-21)
- [x] T003 [P] Cấu hình party đọc từ platform settings: `party_max_members`, `party_invite_ttl`, `party_invite_rate`, `party_kick_reinvite_block`, mặc định `party_policies` cấp platform — file: `src/backend/modules/competition/internal/config/party.go` — xong: `internal/infrastructure/config/party.go` (biến `EW_PARTY_*`, `EW_PRESENCE_TTL`, mặc định CONFIG_DEFAULTS); policy platform `domain/party/policy.go` (`PlatformRules`)
- [x] T004 [P] Key builder Valkey `ew:party:<pid>`, `ew:party:<pid>:members`, `ew:party:user:<uid>`, `ew:party:invite:<id>`, `ew:party:invites:user:<uid>`, `ew:rl:party_invite:<uid>:<window>` — file: `src/backend/modules/competition/internal/infrastructure/cache/keys.go` — xong: `internal/infrastructure/cache/party.go` (`ew:party:<pid>`, `:members`, `ew:party:user:<uid>`, `ew:party:presence:<uid>`); rate limit không dùng `ew:rl:*` (T028)

## Phase 2: Foundational (chặn mọi user story)

- [x] T005 Delta OpenAPI party (`state_version`, `created_at`, `name` maxLength, response `403/404/409/422` theo contracts §6) rồi sinh file theo module — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition` — xong: OpenAPI party (`state_version`, `PartyInvitation.created_at`, `name` ≤ 128, 401/403/404/409/422/429/503, bỏ `max_members default`), `split-public.mjs`; contract `TestPartyOperationsMatchSpec`
- [x] T006 [P] Thêm `moderateDisplayText` + `x-allowed-callers: [competition]` — file: `specs/openapi/modules/chat.internal.v1.yaml` — kiểm: `make specs-lint` — xong (spec + facade sinh `modules/chat/api`; module chat hiện thực ở F116)
- [x] T007 [P] Payload schema 14 event party — file: `specs/events/payloads/competition/party.*.v1.schema.json` — kiểm: `make specs-lint` — xong: 12 schema `specs/events/payloads/competition/party.*.v1` (12 event của contracts §4; `party.activity_*` thuộc F101); `validate-schemas` 0 lỗi; event outbox kiểm schema trong integration (`checkEventSchemas`)
- [x] T008 ent schema `Party` (thêm `StateVersion`), `PartyMember`, `PartyInvitation`, `PartyHistory` (AppendOnly), `PartyPolicy`, `Ref*` với `MaxLen` — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f100_party`, `make migrate-lint MOD=competition` — xong: ent `Party` (`state_version`), `PartyMember`, `PartyInvitation`, `PartyPolicy`; `party_histories` viết tay (phân vùng); `TestMigrationMatchesEnt` pass
- [x] T009 Migration: unique partial (một party active, một leader, lời mời pending), partition tháng `party_histories`, RLS mọi bảng T/TA, `REVOKE DELETE, TRUNCATE`, `infra.*` — file: `src/backend/modules/competition/migrations/` — xong: `migrations/20261229000001_f100_party.sql` (unique partial, RLS FORCE, REVOKE DELETE/TRUNCATE, trigger leave-once, `party_histories` phân vùng append-only, `ref.enrollments`, `ref.sanctions`, view `_live`), `atlas.sum`
- [x] T010 [P] Consumer projection `ref.tenants`, `ref.students`, `ref.classes`, `ref.enrollments`, `ref.player_profiles`, `ref.sanctions` (`source_version`; `ref.friendships`, `ref.blocks` dùng bảng và consumer của F091) + test bắt kịp (TC-SVC-003) — file: `src/backend/modules/competition/internal/presentation/consumer/projection_*.go` — xong: `enrollment.changed` → `ref.enrollments` (consumer `competition.projection_identity`), `ref.sanctions` (consumer `competition.party_safety_chat`); không tạo `ref.classes` (Clarifications); contract `TestConsumersAcceptOwnerExamples`
- [x] T011 [P] Test RLS/global filter bảng `multiplayer.*` (tenant A không đọc B; thiếu scope → 0 dòng; DELETE bị từ chối) — file: `src/backend/modules/competition/tests/integration/rls_test.go` — xong: `tests/integration/rls_party_test.go` (`TestPartyRLS` pass)
- [x] T012 [P] `RealtimePublisher` fan-out `ew:rt:user:<id>` sau commit + `PartyStateCache` (script Lua VALKEY_DESIGN §4.3) — file: `src/backend/modules/competition/internal/infrastructure/realtime/valkey_publisher.go`, `src/backend/modules/competition/internal/infrastructure/cache/party_state.go` — xong: `realtime/party.go` (`PartyPublisher`), `cache/party.go` (`PartyState` Lua sync/drop, `PartyPresence`); `TestPartyRealtimeValkey` pass (invitee nhận `party.invitation.received` ≤ 1 s)

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tạo party và xem party của mình (Priority: P1) 🎯 MVP

**Goal**: Học sinh tạo party, là leader, một party active.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T013 [P] [US1] Test F100-AC-01, F100-AC-03, F100-AC-04: tạo đủ trường từ hồ sơ phiên, mặc định từ cấu hình, outbox `party.created` + `party_histories`; thiếu/lặp `Idempotency-Key`; `422` tên/`max_members`/loại theo band — file: `src/backend/modules/competition/tests/integration/create_party_test.go` — phải FAIL trước khi code — `TestCreateParty`, `TestCreatePartyValidation`, `TestCreatePartyUnprojected`, `TestCreatePartyRate` pass
- [x] T014 [P] [US1] Test F100-AC-02: 100 lượt tạo + accept + join đồng thời → không ai có hai membership active — file: `src/backend/modules/competition/tests/integration/one_active_party_concurrency_test.go` — `TestOneActivePartyUnderConcurrency` pass (102 học sinh × 3 lệnh đồng thời, 0 membership trùng, 0 party hai leader)
- [x] T015 [P] [US1] Test F100-AC-05 (contract + integration với chat giả lập: `blocked` → `422` không lộ từ; timeout → tên mặc định) — file: `src/backend/modules/competition/tests/integration/party_name_moderation_test.go` — `TestPartyNameModeration` pass (moderator MOCK; contract phía chat chờ F116)
- [x] T016 [P] [US1] Test F100-AC-06: response chỉ nickname/avatar/vai trò; người ngoài/tenant khác → `404` — file: `src/backend/modules/competition/tests/integration/get_party_test.go` — `TestGetParty` pass

### Implementation cho User Story 1

- [x] T017 [P] [US1] Domain `Party`, `PartyMember`, lỗi domain — file: `src/backend/modules/competition/internal/domain/party/party.go`, `src/backend/modules/competition/internal/domain/party/member.go`, `src/backend/modules/competition/internal/domain/party/errors.go` — xong: `internal/domain/party/{party,invitation,policy,errors}.go` (Member trong `party.go`); unit `TestPartyAggregate`
- [x] T018 [US1] Command `CreateParty` (kiểm duyệt tên, idempotency, outbox, history) — file: `src/backend/modules/competition/internal/application/command/create_party.go` — xong
- [x] T019 [US1] Query `GetMyParty`, `GetParty` + repository ent — file: `src/backend/modules/competition/internal/application/query/get_party.go`, `src/backend/modules/competition/internal/infrastructure/persistence/party_repository.go` — xong: `query/party.go`, `persistence/party.go` (`Parties`, `PartyDirectory`, SQL tường minh)
- [x] T020 [US1] Handler `createParty`, `getMyParty`, `getParty` (middleware `game_player`, `tenant`) — file: `src/backend/modules/competition/internal/presentation/http/public/parties.go` — xong: `presentation/http/routes.go` (14 operation, `game_player` + `X-Tenant-Id`)

**Checkpoint**: tạo và xem party chạy độc lập.

## Phase 4: User Story 2 — Mời bạn, nhận/từ chối/hủy lời mời (Priority: P1)

### Tests cho User Story 2

- [x] T021 [P] [US2] Test F100-AC-07, F100-AC-11: lời mời `pending` + `expires_at`, `party.invitation.received` tới invitee; accept kiểm lại policy, không trùng, `party.member.joined` tới mọi thành viên — file: `src/backend/modules/competition/tests/integration/invitation_flow_test.go` — phải FAIL trước khi code — `TestInvitationFlow` pass
- [x] T022 [P] [US2] Test F100-AC-08, F100-AC-09: `invitation_policy`, block hai chiều → `PARTY_INVITE_NOT_ALLOWED`; khác trường/tenant/sanction/lớp 1–3 khác lớp/`party_policies` → `PARTY_POLICY_VIOLATION` + audit — file: `src/backend/modules/competition/tests/unit/policy_evaluator_test.go`, `src/backend/modules/competition/tests/integration/invite_policy_test.go` — `TestInviteCheck` (unit `party_policy_test.go`), `TestInviteNotAllowed`, `TestInvitePolicyViolations` pass
- [x] T023 [P] [US2] Test F100-AC-10: lời mời thứ 11/phút → `429` + audit; mời lại sau reject; lời mời trùng → `409` — file: `src/backend/modules/competition/tests/integration/invite_rate_limit_test.go` — `TestInviteRateLimits`, `TestInviteRatePerDay` pass
- [x] T024 [P] [US2] Test F100-AC-12: accept đồng thời vượt `max_members` → `PARTY_FULL`; quá hạn → `410`; sai invitee → `403` — file: `src/backend/modules/competition/tests/integration/accept_invitation_test.go` — `TestAcceptInvitationLimits` pass (trong `invitation_flow_test.go`)
- [x] T025 [P] [US2] Test F100-AC-13, F100-AC-14, F100-AC-15: reject/cancel/quyền cancel; job hết hạn 4 nguyên nhân; `listMyInvitations` chỉ của mình — file: `src/backend/modules/competition/tests/integration/invitation_lifecycle_test.go` — `TestRejectAndCancelInvitation`, `TestInvitationExpiry`, `TestListMyInvitations` pass

### Implementation cho User Story 2

- [x] T026 [P] [US2] Domain `PartyInvitation`, `PolicyEvaluator` — file: `src/backend/modules/competition/internal/domain/party/invitation.go`, `src/backend/modules/competition/internal/domain/party/policy.go` — xong
- [x] T027 [US2] Command `InviteToParty`, `AcceptInvitation`, `RejectInvitation`, `CancelInvitation`, `ExpireInvitations`; query `ListMyInvitations` — file: `src/backend/modules/competition/internal/application/command/invite_to_party.go`, `src/backend/modules/competition/internal/application/command/accept_invitation.go`, `src/backend/modules/competition/internal/application/query/list_my_invitations.go` — xong: `command/invitations.go`, `query/party.go`
- [x] T028 [US2] Rate limiter sliding window `ew:rl:*` — file: `src/backend/modules/competition/internal/infrastructure/cache/rate_limiter.go` — thay bằng đếm PostgreSQL dưới khóa người gửi (Clarifications 2026-11-21); không tạo `rate_limiter.go`
- [x] T029 [US2] Handler lời mời + job `party-invitation-expiry` — file: `src/backend/modules/competition/internal/presentation/http/public/invitations.go`, `src/backend/modules/competition/internal/presentation/job/invitation_expiry.go` — xong: handler trong `presentation/http/routes.go`, job `party-invitation-expiry` (`presentation/job/jobs.go`)

## Phase 5: User Story 3 — Tham gia, rời, kick, chuyển leader, giải tán, sửa cài đặt (Priority: P1)

### Tests cho User Story 3

- [x] T030 [P] [US3] Test F100-AC-16, F100-AC-17: join class/friends; private không lời mời → `403`; leader rời → leader mới đúng thứ tự; người cuối rời → `all_left` — file: `src/backend/modules/competition/tests/integration/join_leave_test.go` — phải FAIL trước khi code — `TestJoinParty`, `TestLeaveParty` pass
- [x] T031 [P] [US3] Test F100-AC-18, F100-AC-19: kick (chặn mời lại, hủy lời mời của người bị kick, audit); transfer tới thành viên offline → `409`; không phải leader → `403 PARTY_NOT_LEADER` — file: `src/backend/modules/competition/tests/integration/kick_transfer_test.go` — `TestKickPartyMember`, `TestKickRate`, `TestTransferPartyLeader` pass
- [x] T032 [P] [US3] Test F100-AC-20, F100-AC-21, F100-AC-22: disband idempotent, lời mời `expired`, một event; sửa cài đặt (`max_members` < số thành viên → `422`), `party.state.snapshot`; mỗi thay đổi có `party_histories` — file: `src/backend/modules/competition/tests/integration/disband_settings_test.go` — `TestDisbandParty`, `TestPartySettingsAndHistory` pass

### Implementation cho User Story 3

- [x] T033 [US3] Command `JoinParty`, `LeaveParty`, `KickMember`, `TransferLeader`, `DisbandParty`, `UpdatePartySettings` — file: `src/backend/modules/competition/internal/application/command/` — xong: `command/membership.go`, `command/create_party.go` (settings)
- [x] T034 [US3] Handler `joinParty`, `leaveParty`, `kickPartyMember`, `transferPartyLeader`, `disbandParty`, `updatePartySettings` — file: `src/backend/modules/competition/internal/presentation/http/public/parties.go` — xong

## Phase 6: User Story 4 — Chỉ học sinh, đúng tenant, an toàn (Priority: P1)

### Tests cho User Story 4

- [x] T035 [P] [US4] Test tenant isolation + authorization F100-AC-23, F100-AC-24: mọi operation party × token nhân sự → `403 GAME_ACCESS_STUDENT_ONLY`; `X-Tenant-Id` khác → `403 TENANT_MISMATCH`; ID tenant khác → `404` (sinh từ OpenAPI) — file: `src/backend/modules/competition/tests/integration/party_authz_tenant_test.go` — phải FAIL trước khi code — `TestPartyRoutesStaffDenied`, `TestPartyTenantIsolation` pass; `routematrix check` 0 problem
- [x] T036 [P] [US4] Test F100-AC-25: `user.blocked` hủy lời mời giữa hai bên; `sanction.applied restrict_multiplayer` loại thành viên (`policy`) + hủy lời mời; `student.synced` đổi trường → loại — file: `src/backend/modules/competition/tests/integration/safety_changes_test.go` — `TestSafetyChanges`, `TestSafetyIdentityChanges` pass; contract `TestPartySafetyConsumers`

### Implementation cho User Story 4

- [x] T037 [US4] Command `ApplySafetyChange` + consumer `safety_changes` (`user.blocked`, `sanction.applied`, `student.synced`, `user.status_changed`) — file: `src/backend/modules/competition/internal/application/command/apply_safety_change.go`, `src/backend/modules/competition/internal/presentation/consumer/safety_changes.go` — xong: `command/apply_safety_change.go`, `presentation/consumer/party_safety.go` (consumer `competition.party_safety_chat`, `competition.party_safety_identity`)

## Phase 7: Polish & Cross-Cutting

- [x] T038 [P] Đăng ký `party_invite_repeat_rules`, `party_pending_invites_max`, `party_create_rate`, `party_kick_rate` trong CONFIG_DEFAULTS + PARTY_SYSTEM §10; kiểm API_DESIGN §5 (block → `PARTY_INVITE_NOT_ALLOWED`), §9.6 và REALTIME §6 (50/ngày, RB-405), REALTIME §5 + VALKEY_DESIGN §3.2 (fan-out `ew:rt:user`), PARTY_SYSTEM §2 (channel `party` tạo bất đồng bộ), §4 (`club_id` theo ADR-0084); CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/03-social-multiplayer/PARTY_SYSTEM.md`, `docs/07-backend/API_DESIGN.md`, `docs/06-architecture/REALTIME_ARCHITECTURE.md`, `docs/07-backend/VALKEY_DESIGN.md` — xong: CONFIG_DEFAULTS đã có 4 tham số; PARTY_SYSTEM §2 (mã lỗi), §10 (đếm PostgreSQL); API_DESIGN §5 (block → `PARTY_INVITE_NOT_ALLOWED`, phạm vi → `PARTY_POLICY_VIOLATION`); REALTIME §5 đã ghi fan-out `ew:rt:user`; VALKEY_DESIGN §3.2 (`ew:party:presence`, `ew:rt:party` không dùng R1.0); CHANGELOG
- [x] T039 [P] Cập nhật DATABASE_DESIGN (§2.9 `state_version`, DDL `party_invitations`, §2.17 projection mới) và EVENT_DESIGN §3 (consumer competition) — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md` — xong: DATABASE_DESIGN §2.9 (`state_version`, cột lời mời), §2.17 (`ref.enrollments` competition, `ref.sanctions`); EVENT_DESIGN (`enrollment.changed` → competition)
- [x] T040 [P] Metric `eduworld_party_*`, `eduworld_projection_lag_seconds` — file: `src/backend/modules/competition/internal/infrastructure/observability/metrics.go` — xong: `observability/party.go` (`eduworld_party_command_total`, `eduworld_party_invitation_total`, `eduworld_party_policy_denied_total`); lag `eduworld_competition_projection_lag_seconds` (có từ F091); gauge `eduworld_party_active` chưa làm (Clarifications)
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [x] T042 **OQ chốt 2026-11-20** (OQ F016-2, F045-3) Facade vòng đời/gộp của module competition; facade test với identity — file: `src/backend/modules/competition/internal/presentation/facade/` — kiểm: `make test-contract`, integration — cỡ M — xong: `command/student_lifecycle.go` (`PartyLifecycle`: cleanup → rời party + kết thúc lời mời; restore → `nothing_to_do`; anonymize → rời party + đặt lại tên party đã lập/làm leader; merge → chuyển membership sang user đích giữ vai trò, hoặc rời khi đích đã có party; idempotent), `presentation/facade/facade.go`, `Module.Facade()`; identity: `participants/competition.go`, `Gateway.CompetitionMerge`, `wire.go` (`Imports.Competition`), compose nối facade; test `TestPartyLifecycleFacade` (integration), `TestOwnerGatewayCompetition` (contract bên gọi)

## Nhận từ F121 (owner chốt 2026-11-21)

- [ ] T043 Consumer `competition.party_safety_chat` xử lý `sanction.applied`/`sanction.lifted` có `reason_code = case_hold` (hold tạm khi case p1 mở): người bị hold không gửi được lời mời party mới và không được mời; gỡ khi `sanction.lifted`; fixture test contract competition validate theo `specs/events/payloads/chat/sanction.*.v1.schema.json` (EVENT_DESIGN §4.9: `sanction_type`, `until`, `reason_code`) — kiểm: `make test-integration MOD=competition`, `make test-contract` — cỡ S

## ADR-0095 — bỏ khái niệm lớp (WP-C1, 2026-11-21)

- [x] T044 Luật party không lớp (D2): `Rules.SameGradeOnly` thay `CrossClass` (lớp 1–3 luôn bật; cấp trường chỉ bật), rule `grade` thay `class`; bỏ `visibility = class`, cấp `class` của `party_policies`; đọc dòng cũ (`study_group`, `class`) qua `Party.Legacy()` — file: `internal/domain/party/policy.go`, `party.go`, `errors.go`; test `TestGrades13SameGrade`, `TestPartyLegacyValues` (unit), `TestGrades13SameGradeParty` (integration: khác khối / khác trường khi tenant tắt liên trường / khác tenant → 403 + audit; tenant `unit` bật liên trường cùng khối → 201) pass
- [x] T045 D7 `study_group` → `study_party` (code, OpenAPI, AsyncAPI, `party.created.v2`); migration `20261231000002_adr0095_noclass.sql` (CHECK mở rộng, đổi dữ liệu, `visibility class` → `private`, soft-delete policy cấp lớp, gỡ `cross_class`) — test `TestCreatePartyValidation`, `TestJoinParty` (`visibility = class` → 422) pass
- [x] T046 Projection không lớp: Subject đọc `ref.students`; consumer `student.placement_changed` (owner F010) vào `ref.students` + `competition.party_safety_identity`; `enrollment.changed` chỉ ack — test `TestConsumersAcceptOwnerExamples`, `TestPartySafetyConsumers`, `TestIdentityProjectionWithoutClasses` (contract), `TestPlacementChangedFeedsGradeRule` (integration) pass
- [ ] T047 (WP-R) DROP `ref.enrollments`; bỏ `study_group`, `class` khỏi `parties_type_chk`/`parties_visibility_chk` và `class` khỏi `party_policies_scope_chk` khi 0 dòng mang giá trị cũ (kể cả đã soft-delete)

## Dependencies & Execution Order

- Setup → Foundational (T010 projection trước mọi story) → US1 → US2, US3 song song → US4 → Polish.
- T006/T015 cần F116 hiện thực `moderateDisplayText`; trước đó test dùng stub contract có nhãn MOCK (R17), không dùng trong build chạy thật.

## Definition of Done

- [x] Mọi tiêu chí F100-AC-* có test pass (ghi mã test/TC) — 25/25, mã test ở từng task
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` xanh cho module `competition`; `make test-contract` xanh — tương đương đã chạy: facadegen/allowedcallers/archgen --check, golangci-lint (+arch), tenancyguard, ewlint, entlint, routematrix, unit + integration competition (tc-lock), TestMigrationMatchesEnt; `atlas migrate validate` chạy trong testkit khi migrate DB test
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — moderator/fake chỉ trong test (nhãn TEST DOUBLE/MOCK); đường chạy thật dùng facade chat (rỗng tới F116 → tên mặc định)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
