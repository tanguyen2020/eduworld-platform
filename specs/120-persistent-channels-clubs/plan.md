# Implementation Plan: F120 — Channel bền vững (world, area, class, study group, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Branch**: `120-persistent-channels-clubs` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Mở rộng module chat (đã có lõi F115–F118) với: (1) consumer đồng bộ channel `class` (`class.synced`, `class.created`, `enrollment.changed`) và `study_group` (`study_group.*`, ADR-0084); (2) aggregate `ChatPolicy` 4 cấp với API `get/putChatPolicy`, validator schema v5 + luật siết chặt/always-on, effective policy cache Valkey; (3) channel `world`/`area` (ephemeral) và `direct` (bạn bè, đóng băng khi block); (4) module club (`social.*`) dùng channel `team` cố định; (5) tìm kiếm full-text trên `body_display`; (6) job `persistent-chat-retention` ẩn danh hóa. Ghi trạng thái theo CQRS + outbox; tin nhắn vẫn đi JetStream → `chat-persister` (ADR-0035).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent + Atlas, oapi-codegen, nats.go (JetStream), go-redis (Valkey), `src/backend/pkg/outbox`, `src/backend/pkg/tenancy`, `src/backend/pkg/entx`, `src/backend/pkg/authz`, `santhosh-tekuri/jsonschema` (chat-policy v5) |
| Storage | PostgreSQL `eduworld_chat` (schema `chat`, `social`, `ref`, `infra`); Valkey (cache policy, seq — tạm, ADR-0004) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); contract test consumer event (`study_group.*`, `enrollment.changed`) |
| Target Platform | Linux container (module `chat` nạp trong `core-api`; profile `worker-chat` gồm `chat-persister`, consumer và job) |
| Project Type | backend module |
| Performance Goals | Pipeline p95 `moderation_pipeline_latency`; tìm kiếm theo [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md); consumer membership ≤ 5 s p95 |
| Constraints | AGENTS R8, R13–R16, R21, R25–R29; ADR-0020, ADR-0033, ADR-0034, ADR-0035, ADR-0050, ADR-0054, ADR-0070, ADR-0084 |
| Scale/Scope | `club_max_members`, `clubs_per_student_max`; channel lớp ≈ số lớp; NFR-SCAL theo CONFIG_DEFAULTS |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `getChatPolicy`/`putChatPolicy`, `ClubType`, event payload là task Phase 2 (T004–T008) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Study group/enrollment qua event; không gọi facade đồng bộ; command handler (`UnitOfWork`) + outbox (ADR-0087) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Mọi bảng `tenant_id` + RLS + `deleted_at`; retention ẩn danh hóa, không DELETE; test RLS (T015) |
| IV Server-authoritative | ✅ | Tiến độ club goal chỉ từ event server đã kiểm chứng; không nhận số từ client |
| V Xác thực, phân quyền | ✅ | `platform.settings.write`, `tenant.settings.write`, `school.settings.write`, `social.club.approve`; route học sinh `game_player` |
| VI An toàn trẻ em | ✅ | Một bộ rule mọi lớp (schema chặn `grade_*`); tên club qua pipeline; nickname với người ngoài trường; không voice |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; `MaxLen` mọi chuỗi; tên `persistent_chat_retention_days` đăng ký ở Polish |
| VIII UI tách logic, token | ✅ | Không có UI trong feature (UI ở F119, F122) |
| IX Test có bằng chứng | ✅ | Mỗi F120-AC có test viết trước; test tenant isolation + authz cho endpoint mới |

## Project Structure

### Documentation (feature này)

```text
specs/120-persistent-channels-clubs/
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
src/backend/modules/chat/
├── ent/schema/{channel.go,channel_member.go,chat_policy.go,persistent_retention_run.go,
│               club.go,club_member.go,club_join_request.go,club_invitation.go,club_goal.go,club_goal_progress.go,club_event.go}
├── internal/domain/
│   ├── channel/{channel.go,membership.go,kind.go}            # trạng thái open/read_only/frozen/archived
│   ├── policy/{rule.go,validate.go,intersect.go,effective.go} # siết chặt, always-on, không grade
│   ├── club/{club.go,member.go,goal.go}
│   └── retention/plan.go
├── internal/application/
│   ├── command/{channel/sync_class_channel.go,channel/sync_study_group_channel.go,channel/freeze_direct.go,
│   │            policy/put_chat_policy.go,club/*.go,retention/run_persistent_retention.go}
│   ├── query/{policy/get_chat_policy.go,policy/effective_policy.go,search/search_messages.go,club/list_clubs.go}
│   └── port/{policy_cache.go,school_settings_reader.go,moderation_checker.go,evidence_copier.go}
├── internal/infrastructure/{persistence/*,cache/policy_cache.go,messaging/outbox_publisher.go,search/tsquery.go}
├── internal/presentation/
│   ├── http/public/{chat_policy.go,clubs.go,search.go}
│   ├── consumer/{class_synced.go,enrollment_changed.go,study_group.go,user_blocked.go,club_goal_sources.go}
│   └── job/persistent_retention.go                            # chạy trong profile worker-chat
└── tests/{unit,integration,contract}/
```

**Structure Decision**: Một module `chat` (chat, social) theo ADR-0074; consumer và job chạy ở profile `worker-chat` (`src/backend/cmd/worker --profile=chat`), tin nhắn vẫn qua `chat-persister` (ADR-0035). Policy là aggregate riêng vì có version và luật hợp lệ phức tạp.

## Phase 0: Research

Không có. Thuật toán policy ở [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md), tìm kiếm §8.4, retention §6, club [SOCIAL_SYSTEM §6](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md), study group ADR-0084.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `channel.Channel`, `policy.Rule`/`Intersect`/`ValidateLowerLevel`, `club.Club` | Invariant: không `grade_*`; always-on; cấp dưới ⊆ cấp trên; club ≤ `max_members`; staff không là member |
| application | Command `SyncClassChannel`, `SyncStudyGroupChannel`, `FreezeDirectChannel`, `PutChatPolicy`, `CreateClub`, `ApproveClub`, `JoinClub`, `LeaveClub`, `ArchiveClub`, `RecordClubGoalProgress`, `RunPersistentRetention` · Query `GetChatPolicy`, `GetEffectivePolicy`, `SearchMessages`, `ListClubs`, `ListMyChannels` | Permission theo spec US2/US4; authz tường minh đầu handler + idempotency middleware `src/backend/pkg/httpx` |
| infrastructure | Repository ent (global filter tenant + soft-delete), `PolicyCache` Valkey `ew:chat:policy:<tenant>:<scope>`, outbox publisher, port `ModerationChecker` (pipeline in-process F116) | Không đọc DB module khác |
| presentation | Handler `getChatPolicy`, `putChatPolicy`, `listClubs`, `createClub`, `getClub`, `updateClub`, `approveClub`, `leaveClub`, `archiveClub`, `listClubMembers`, `updateClubMemberRole`, `removeClubMember`, `listClubJoinRequests`, `requestJoinClub`, `acceptClubJoinRequest`, `rejectClubJoinRequest`, `inviteToClub`, `acceptClubInvitation`, `declineClubInvitation`, `listClubGoals`, `createClubGoal`, `listClubEvents`, `createClubEvent`, `searchMessages`, `listMyChannels`, `listChannelMessages` · consumer `class.synced`, `class.created`, `enrollment.changed`, `study_group.*`, `user.blocked`/`user.unblocked`, `school.updated`, `activity.graded`, `quest.completed`, `boss.encounter_completed`, `world.discovery_found` · job `persistent-chat-retention` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F115 (channel, JetStream, persister, `ref.*`), F116 (pipeline in-process), F117 (ephemeral purge, `moderation_evidence`), F118 (tính năng tin), F095 (`friendships`, `blocks`, `user.blocked`), F040 (`study_group.*`), F010/F011 (`class.synced`, `enrollment.changed`), F015 (`school.updated` mang `student_club_creation`), F121 (case mở → giữ bằng chứng).
- Facade: không gọi. Event phát: `chat_policy.updated`, `club.created`, `club.approved`, `club.archived`, `club.member_joined`, `club.member_left`, `club.goal_completed`.

### Bảo mật, tenant, idempotency, audit

- Route học sinh (`/chat/*`, `/social/clubs*` khi người gọi là học sinh) qua `game_player`; route nhân sự (`approveClub`, `createClub` loại trường, `putChatPolicy`) yêu cầu `azp = eduworld-portal`; cấp platform chỉ gateway nội bộ + MFA.
- `TenantScope` mọi query; không `CrossTenantScope`. Idempotency-Key: `createClub`, `approveClub`, `requestJoinClub`, `inviteToClub`, `accept*`, `createClubGoal`, `createClubEvent`; `putChatPolicy` dùng `If-Match`.
- Audit: `chat_policy.updated` (before/after), `club.created/approved/archived/member_removed`, `chat.retention_applied` (tổng hợp, không nội dung).

### Quan sát

- Metric: `eduworld_chat_active_channels{channel_type}`, `eduworld_chat_search_latency_seconds`, `eduworld_chat_membership_sync_lag_seconds` (mới), `eduworld_chat_retention_anonymized_total{tenant}` (mới), `eduworld_chat_policy_rejected_total{code}` (mới). Log không chứa nội dung tin/tên học sinh; span `chat.policy.effective`, `chat.consumer.<event>`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
