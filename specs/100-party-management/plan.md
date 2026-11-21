# Implementation Plan: F100 — Party: tạo/tham gia/mời/kick/chuyển leader/giải tán, một party active, giới hạn

**Branch**: `100-party-management` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Feature đầu tiên của module competition: dựng skeleton theo BACKEND_DESIGN §3, bounded context `party` trong schema `multiplayer` của `eduworld_competition`. Aggregate `Party` (thành viên, leader, cài đặt) và `PartyInvitation`; mọi command handler (write context) dùng `UnitOfWork`: transaction ent với `FOR UPDATE` trên party + unique partial index (research R4), outbox các event `party.*`. Policy kiểm tra trên projection `ref.*` (học sinh, khối, bạn bè, block, sanction — research R3; không còn lớp — ADR-0095). Sau commit cập nhật Valkey `ew:party:*` và fan-out message WS qua `ew:rt:user:<id>` (research R1). Tên tự đặt kiểm duyệt qua module chat internal có fallback (research R2). Worker: job hết hạn lời mời, consumer projection.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server `competition.v1.yaml`) + facade `chat`, `identity` (`src/backend/modules/<callee>/api`), `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, `src/backend/pkg/idempotency`, `src/backend/pkg/httpx` (`game_player`, `tenant`, rate limit), `src/backend/pkg/modcall`, Valkey client |
| Storage | PostgreSQL `eduworld_competition` (`multiplayer.parties`, `party_members`, `party_invitations`, `party_histories`, `party_policies`, `ref.*`, `infra.*`); Valkey `ew:party:*`, `ew:rl:*`, pub/sub `ew:rt:user:<id>` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); facade test với module chat (`moderateDisplayText`: fake facade phía competition, ví dụ spec phía chat) |
| Target Platform | Linux container (module `competition` nạp trong `core-api` + `worker-events`) |
| Project Type | backend module |
| Performance Goals | Lời mời tới invitee ≤ 1 s p95 (AC-PTY-002/003); route ghi theo `api_rate_default` |
| Constraints | AGENTS R8, R9, R10, R13, R14, R15, R16, R21, R23, R26–R29; ADR-0004, ADR-0031, ADR-0037, ADR-0042, ADR-0050, ADR-0054, ADR-0065 |
| Scale/Scope | `party_max_members`; party ≤ 8 thành viên |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Operation REST/WS có sẵn; delta payload event, `moderateDisplayText`, cột `state_version` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Không đọc DB chat/identity/game; bạn bè/block/sanction qua projection từ event; kiểm duyệt qua facade `chat` |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Mọi bảng `tenant_id` + RLS + `deleted_at`; giải tán bằng `status`; test isolation mọi endpoint |
| IV Server-authoritative | ✅ | Leader/thành viên/giới hạn đọc PG trong transaction, không tin client; `Idempotency-Key` cho tạo/mời/accept/join/transfer; audit |
| V Xác thực, phân quyền | ✅ | `game_player` mọi route; kiểm tra nghiệp vụ leader/member/invitee; không quyền nhân sự |
| VI An toàn trẻ em | ✅ | Block, sanction, cùng trường, grade band; chỉ nickname/avatar; tên qua kiểm duyệt; không log PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; chuỗi có `MaxLen`; mã FR từ PRD |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi F100-AC có task test; test đồng thời; test authz sinh từ OpenAPI |

## Project Structure

### Documentation (feature này)

```text
specs/100-party-management/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/competition/
├── cmd/{api,worker,migrate}/main.go
├── ent/schema/        # party.go, partymember.go, partyinvitation.go, partyhistory.go, partypolicy.go,
│                      # refstudent.go, refenrollment.go, refplayerprofile.go, reffriendship.go, refblock.go, refsanction.go, reftenant.go
├── migrations/
├── internal/domain/party/              # party.go, member.go, invitation.go, policy.go (PolicyEvaluator), errors.go
├── internal/application/command/       # create_party.go, update_party_settings.go, invite_to_party.go, accept_invitation.go,
│                                       # reject_invitation.go, cancel_invitation.go, join_party.go, leave_party.go,
│                                       # kick_member.go, transfer_leader.go, disband_party.go, expire_invitations.go, apply_safety_change.go
├── internal/application/query/         # get_my_party.go, get_party.go, list_my_invitations.go
├── internal/application/port/          # PartyRepository, InvitationRepository, ProjectionReader, ModerationClient,
│                                       # RealtimePublisher, PartyStateCache, RateLimiter, Clock
├── internal/infrastructure/persistence/
├── internal/infrastructure/client/chatclient/        # moderateDisplayText (oapi-codegen + modcall)
├── internal/infrastructure/realtime/valkey_publisher.go
├── internal/infrastructure/cache/{keys.go, party_state.go}
├── internal/presentation/http/public/{parties.go, invitations.go}
├── internal/presentation/consumer/     # projection_identity.go, projection_social.go, projection_game.go, safety_changes.go
├── internal/presentation/job/invitation_expiry.go
└── tests/{unit,integration,contract}/
```

**Structure Decision**: theo BACKEND_DESIGN §3 (D1: `src/backend/modules/competition/`). Bounded context `party` là package domain đầu tiên; matchmaking/battle (F104, F105) thêm package cạnh bên.

## Phase 0: Research

Xem [research.md](research.md): R1 fan-out theo user, R2 kiểm duyệt tên, R3 projection, R4 tuần tự hóa membership.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `Party` (AddMember, RemoveMember(reason), TransferLeader, UpdateSettings, Disband, nextLeader()), `PartyInvitation` (Accept, Reject, Cancel, Expire), `PolicyEvaluator` (trường/tenant unit, khối (`same_grade_only`, ADR-0095), grade band, loại party theo band, block, sanction, `party_policies` siết dần) | Thuần; Clock inject |
| application | Command ở cây trên (mỗi command: nạp aggregate `FOR UPDATE`, gọi domain, lưu, outbox + `party_histories` cùng transaction); query đọc read model | `game_player` + kiểm tra leader/member/invitee trong handler |
| infrastructure | ent repo, projection reader, adapter facade `chat`, Valkey state + publisher, rate limiter `ew:rl:party_invite:<student_id>:<window>` | Key qua `cache/keys.go` |
| presentation | `createParty`, `getMyParty`, `getParty`, `updatePartySettings`, `inviteToParty`, `listMyInvitations`, `acceptPartyInvitation`, `rejectPartyInvitation`, `cancelPartyInvitation`, `joinParty`, `leaveParty`, `kickPartyMember`, `transferPartyLeader`, `disbandParty` · consumer projection/safety · job `party-invitation-expiry` (mỗi 15 s, khóa `ew:lock:job:party-invitation-expiry`) | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F095 (event bạn bè/block), F003 (outbox, idempotency, `UnitOfWork`), F012 (`resolvePrincipal`), F090 (gateway giao `ew:rt:user`). Facade: `moderateDisplayText` (chat, F116), `resolvePrincipal` (identity).
- Event phát: `party.created`, `party.settings_updated`, `party.invitation_sent|_accepted|_rejected|_expired|_cancelled`, `party.member_joined|_left|_kicked`, `party.leader_transferred`, `party.disbanded`, `audit.recorded`.
- Event tiêu thụ: `tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up`, `enrollment.changed` (identity); `friend.added|removed`, `user.blocked|unblocked`, `sanction.applied|lifted` (chat); `game_profile.updated` (game).

### Bảo mật, tenant, idempotency, audit

- `game_player` (`azp = eduworld-game-backend`, `user_type = student`) mọi route; `TenantScope` từ Principal, `X-Tenant-Id` kiểm tra chéo.
- `Idempotency-Key` bắt buộc: `createParty`, `inviteToParty`, `acceptPartyInvitation`, `joinParty`, `transferPartyLeader`; các thao tác khác idempotent tự nhiên (API_DESIGN §9.6 cột Idem).
- Audit action: `party.created`, `party.disbanded`, `party.member_kicked`, `party.leader_transferred`, `party.invitation_policy_denied`, `party.invite_rate_limited`, `party.member_removed_by_policy` (không ghi nội dung chat).

### Quan sát

- Metric `eduworld_party_active`, `eduworld_party_command_total{command,result}`, `eduworld_party_invitation_total{result}`, `eduworld_party_policy_denied_total{rule}`, `eduworld_projection_lag_seconds{table}`; span theo command; log không chứa nickname/tên.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
