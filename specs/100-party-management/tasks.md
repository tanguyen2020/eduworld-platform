# Tasks: F100 — Party: tạo/tham gia/mời/kick/chuyển leader/giải tán, một party active, giới hạn

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Skeleton module competition (`module.go` đăng ký vào `src/backend/cmd/core-api` và `src/backend/cmd/worker`, `api/`, `internal/*`, `migrations/`, `src/backend/pkg/config`, outbox relay, rule depguard/arch-check) — file: `src/backend/modules/competition/`
- [ ] T002 [P] Adapter facade `identity` (`resolvePrincipal`, cache Principal) và `chat` (`moderateDisplayText`) qua `src/backend/pkg/modcall` (TenantScope, Principal gốc trong context) — file: `src/backend/modules/competition/internal/infrastructure/modules/{identity,chat}.go`
- [ ] T003 [P] Cấu hình party đọc từ platform settings: `party_max_members`, `party_invite_ttl`, `party_invite_rate`, `party_kick_reinvite_block`, mặc định `party_policies` cấp platform — file: `src/backend/modules/competition/internal/config/party.go`
- [ ] T004 [P] Key builder Valkey `ew:party:<pid>`, `ew:party:<pid>:members`, `ew:party:user:<uid>`, `ew:party:invite:<id>`, `ew:party:invites:user:<uid>`, `ew:rl:party_invite:<uid>:<window>` — file: `src/backend/modules/competition/internal/infrastructure/cache/keys.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T005 Delta OpenAPI party (`state_version`, `created_at`, `name` maxLength, response `403/404/409/422` theo contracts §6) rồi sinh file theo module — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`
- [ ] T006 [P] Thêm `moderateDisplayText` + `x-allowed-callers: [competition]` — file: `specs/openapi/modules/chat.internal.v1.yaml` — kiểm: `make specs-lint`
- [ ] T007 [P] Payload schema 14 event party — file: `specs/events/payloads/competition/party.*.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T008 ent schema `Party` (thêm `StateVersion`), `PartyMember`, `PartyInvitation`, `PartyHistory` (AppendOnly), `PartyPolicy`, `Ref*` với `MaxLen` — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f100_party`, `make migrate-lint MOD=competition`
- [ ] T009 Migration: unique partial (một party active, một leader, lời mời pending), partition tháng `party_histories`, RLS mọi bảng T/TA, `REVOKE DELETE, TRUNCATE`, `infra.*` — file: `src/backend/modules/competition/migrations/`
- [ ] T010 [P] Consumer projection `ref.tenants`, `ref.students`, `ref.classes`, `ref.enrollments`, `ref.player_profiles`, `ref.sanctions` (`source_version`; `ref.friendships`, `ref.blocks` dùng bảng và consumer của F091) + test bắt kịp (TC-SVC-003) — file: `src/backend/modules/competition/internal/presentation/consumer/projection_*.go`
- [ ] T011 [P] Test RLS/global filter bảng `multiplayer.*` (tenant A không đọc B; thiếu scope → 0 dòng; DELETE bị từ chối) — file: `src/backend/modules/competition/tests/integration/rls_test.go`
- [ ] T012 [P] `RealtimePublisher` fan-out `ew:rt:user:<id>` sau commit + `PartyStateCache` (script Lua VALKEY_DESIGN §4.3) — file: `src/backend/modules/competition/internal/infrastructure/realtime/valkey_publisher.go`, `src/backend/modules/competition/internal/infrastructure/cache/party_state.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tạo party và xem party của mình (Priority: P1) 🎯 MVP

**Goal**: Học sinh tạo party, là leader, một party active.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T013 [P] [US1] Test F100-AC-01, F100-AC-03, F100-AC-04: tạo đủ trường từ hồ sơ phiên, mặc định từ cấu hình, outbox `party.created` + `party_histories`; thiếu/lặp `Idempotency-Key`; `422` tên/`max_members`/loại theo band — file: `src/backend/modules/competition/tests/integration/create_party_test.go` — phải FAIL trước khi code
- [ ] T014 [P] [US1] Test F100-AC-02: 100 lượt tạo + accept + join đồng thời → không ai có hai membership active — file: `src/backend/modules/competition/tests/integration/one_active_party_concurrency_test.go`
- [ ] T015 [P] [US1] Test F100-AC-05 (contract + integration với chat giả lập: `blocked` → `422` không lộ từ; timeout → tên mặc định) — file: `src/backend/modules/competition/tests/integration/party_name_moderation_test.go`
- [ ] T016 [P] [US1] Test F100-AC-06: response chỉ nickname/avatar/vai trò; người ngoài/tenant khác → `404` — file: `src/backend/modules/competition/tests/integration/get_party_test.go`

### Implementation cho User Story 1

- [ ] T017 [P] [US1] Domain `Party`, `PartyMember`, lỗi domain — file: `src/backend/modules/competition/internal/domain/party/party.go`, `src/backend/modules/competition/internal/domain/party/member.go`, `src/backend/modules/competition/internal/domain/party/errors.go`
- [ ] T018 [US1] Command `CreateParty` (kiểm duyệt tên, idempotency, outbox, history) — file: `src/backend/modules/competition/internal/application/command/create_party.go`
- [ ] T019 [US1] Query `GetMyParty`, `GetParty` + repository ent — file: `src/backend/modules/competition/internal/application/query/get_party.go`, `src/backend/modules/competition/internal/infrastructure/persistence/party_repository.go`
- [ ] T020 [US1] Handler `createParty`, `getMyParty`, `getParty` (middleware `game_player`, `tenant`) — file: `src/backend/modules/competition/internal/presentation/http/public/parties.go`

**Checkpoint**: tạo và xem party chạy độc lập.

## Phase 4: User Story 2 — Mời bạn, nhận/từ chối/hủy lời mời (Priority: P1)

### Tests cho User Story 2

- [ ] T021 [P] [US2] Test F100-AC-07, F100-AC-11: lời mời `pending` + `expires_at`, `party.invitation.received` tới invitee; accept kiểm lại policy, không trùng, `party.member.joined` tới mọi thành viên — file: `src/backend/modules/competition/tests/integration/invitation_flow_test.go` — phải FAIL trước khi code
- [ ] T022 [P] [US2] Test F100-AC-08, F100-AC-09: `invitation_policy`, block hai chiều → `PARTY_INVITE_NOT_ALLOWED`; khác trường/tenant/sanction/lớp 1–3 khác lớp/`party_policies` → `PARTY_POLICY_VIOLATION` + audit — file: `src/backend/modules/competition/tests/unit/policy_evaluator_test.go`, `src/backend/modules/competition/tests/integration/invite_policy_test.go`
- [ ] T023 [P] [US2] Test F100-AC-10: lời mời thứ 11/phút → `429` + audit; mời lại sau reject; lời mời trùng → `409` — file: `src/backend/modules/competition/tests/integration/invite_rate_limit_test.go`
- [ ] T024 [P] [US2] Test F100-AC-12: accept đồng thời vượt `max_members` → `PARTY_FULL`; quá hạn → `410`; sai invitee → `403` — file: `src/backend/modules/competition/tests/integration/accept_invitation_test.go`
- [ ] T025 [P] [US2] Test F100-AC-13, F100-AC-14, F100-AC-15: reject/cancel/quyền cancel; job hết hạn 4 nguyên nhân; `listMyInvitations` chỉ của mình — file: `src/backend/modules/competition/tests/integration/invitation_lifecycle_test.go`

### Implementation cho User Story 2

- [ ] T026 [P] [US2] Domain `PartyInvitation`, `PolicyEvaluator` — file: `src/backend/modules/competition/internal/domain/party/invitation.go`, `src/backend/modules/competition/internal/domain/party/policy.go`
- [ ] T027 [US2] Command `InviteToParty`, `AcceptInvitation`, `RejectInvitation`, `CancelInvitation`, `ExpireInvitations`; query `ListMyInvitations` — file: `src/backend/modules/competition/internal/application/command/invite_to_party.go`, `src/backend/modules/competition/internal/application/command/accept_invitation.go`, `src/backend/modules/competition/internal/application/query/list_my_invitations.go`
- [ ] T028 [US2] Rate limiter sliding window `ew:rl:*` — file: `src/backend/modules/competition/internal/infrastructure/cache/rate_limiter.go`
- [ ] T029 [US2] Handler lời mời + job `party-invitation-expiry` — file: `src/backend/modules/competition/internal/presentation/http/public/invitations.go`, `src/backend/modules/competition/internal/presentation/job/invitation_expiry.go`

## Phase 5: User Story 3 — Tham gia, rời, kick, chuyển leader, giải tán, sửa cài đặt (Priority: P1)

### Tests cho User Story 3

- [ ] T030 [P] [US3] Test F100-AC-16, F100-AC-17: join class/friends; private không lời mời → `403`; leader rời → leader mới đúng thứ tự; người cuối rời → `all_left` — file: `src/backend/modules/competition/tests/integration/join_leave_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US3] Test F100-AC-18, F100-AC-19: kick (chặn mời lại, hủy lời mời của người bị kick, audit); transfer tới thành viên offline → `409`; không phải leader → `403 PARTY_NOT_LEADER` — file: `src/backend/modules/competition/tests/integration/kick_transfer_test.go`
- [ ] T032 [P] [US3] Test F100-AC-20, F100-AC-21, F100-AC-22: disband idempotent, lời mời `expired`, một event; sửa cài đặt (`max_members` < số thành viên → `422`), `party.state.snapshot`; mỗi thay đổi có `party_histories` — file: `src/backend/modules/competition/tests/integration/disband_settings_test.go`

### Implementation cho User Story 3

- [ ] T033 [US3] Command `JoinParty`, `LeaveParty`, `KickMember`, `TransferLeader`, `DisbandParty`, `UpdatePartySettings` — file: `src/backend/modules/competition/internal/application/command/`
- [ ] T034 [US3] Handler `joinParty`, `leaveParty`, `kickPartyMember`, `transferPartyLeader`, `disbandParty`, `updatePartySettings` — file: `src/backend/modules/competition/internal/presentation/http/public/parties.go`

## Phase 6: User Story 4 — Chỉ học sinh, đúng tenant, an toàn (Priority: P1)

### Tests cho User Story 4

- [ ] T035 [P] [US4] Test tenant isolation + authorization F100-AC-23, F100-AC-24: mọi operation party × token nhân sự → `403 GAME_ACCESS_STUDENT_ONLY`; `X-Tenant-Id` khác → `403 TENANT_MISMATCH`; ID tenant khác → `404` (sinh từ OpenAPI) — file: `src/backend/modules/competition/tests/integration/party_authz_tenant_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US4] Test F100-AC-25: `user.blocked` hủy lời mời giữa hai bên; `sanction.applied restrict_multiplayer` loại thành viên (`policy`) + hủy lời mời; `student.synced` đổi trường → loại — file: `src/backend/modules/competition/tests/integration/safety_changes_test.go`

### Implementation cho User Story 4

- [ ] T037 [US4] Command `ApplySafetyChange` + consumer `safety_changes` (`user.blocked`, `sanction.applied`, `student.synced`, `user.status_changed`) — file: `src/backend/modules/competition/internal/application/command/apply_safety_change.go`, `src/backend/modules/competition/internal/presentation/consumer/safety_changes.go`

## Phase 7: Polish & Cross-Cutting

- [ ] T038 [P] Đăng ký `party_invite_repeat_rules`, `party_pending_invites_max`, `party_create_rate`, `party_kick_rate` trong CONFIG_DEFAULTS + PARTY_SYSTEM §10; kiểm API_DESIGN §5 (block → `PARTY_INVITE_NOT_ALLOWED`), §9.6 và REALTIME §6 (50/ngày, RB-405), REALTIME §5 + VALKEY_DESIGN §3.2 (fan-out `ew:rt:user`), PARTY_SYSTEM §2 (channel `party` tạo bất đồng bộ), §4 (`club_id` theo ADR-0084); CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/03-social-multiplayer/PARTY_SYSTEM.md`, `docs/07-backend/API_DESIGN.md`, `docs/06-architecture/REALTIME_ARCHITECTURE.md`, `docs/07-backend/VALKEY_DESIGN.md`
- [ ] T039 [P] Cập nhật DATABASE_DESIGN (§2.9 `state_version`, DDL `party_invitations`, §2.17 projection mới) và EVENT_DESIGN §3 (consumer competition) — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`
- [ ] T040 [P] Metric `eduworld_party_*`, `eduworld_projection_lag_seconds` — file: `src/backend/modules/competition/internal/infrastructure/observability/metrics.go`
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T042 **OQ chốt 2026-11-20** (OQ F016-2, F045-3) Facade vòng đời/gộp của module competition; facade test với identity — file: `src/backend/modules/competition/internal/presentation/facade/` — kiểm: `make test-contract`, integration — cỡ M

## Dependencies & Execution Order

- Setup → Foundational (T010 projection trước mọi story) → US1 → US2, US3 song song → US4 → Polish.
- T006/T015 cần F116 hiện thực `moderateDisplayText`; trước đó test dùng stub contract có nhãn MOCK (R17), không dùng trong build chạy thật.

## Definition of Done

- [ ] Mọi tiêu chí F100-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` xanh cho module `competition`; `make test-contract` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
