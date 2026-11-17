# Implementation Plan: F095 — Bạn bè, block, mute (module social)

**Branch**: `095-friends-block-mute` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Feature đầu tiên của module chat: dựng skeleton theo [BACKEND_DESIGN §3](../../docs/07-backend/BACKEND_DESIGN.md), projection `ref.*` cần cho phạm vi và hiển thị, module `social` với aggregate `FriendRequest`, `Friendship`, `FriendCode`, `Block`, `Mute`. Command handler (write context, `UnitOfWork`) ghi `eduworld_chat` + outbox (`friend_request.sent`, `friend.added`, `friend.removed`, `user.blocked`, `user.unblocked`, `user.muted`, `user.unmuted`, `audit.recorded`) trong một transaction. `ScopePolicy` kiểm cùng trường/liên trường theo `ref.tenants.settings_subset`, block hai chiều. Worker: job hết hạn lời mời, consumer projection, subscriber NATS core `ew.rt.presence.*` gửi `presence.friend.updated` cho bạn bè qua `pkg/realtime`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `pkg/entx` (`UnitOfWork`), `pkg/httpx` (`game_player`, `tenant`, `idempotency`), `pkg/outbox`, `pkg/natsx`, `pkg/realtime`, `valkey-go` |
| Storage | PostgreSQL `eduworld_chat` (`social.friend_requests`, `social.friendships`, `social.friend_codes`, `social.blocks`, `social.mutes`, `ref.*`, `infra.*`); Valkey cache bạn bè + presence (tạm) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); contract test event với competition |
| Target Platform | Linux container (module `chat` nạp trong `core-api` + `worker-chat`) |
| Project Type | backend module |
| Performance Goals | Route đọc p95 theo `api_rate_default`/NFR chung; fan-out presence ≤ 2 s |
| Constraints | AGENTS R8, R10, R13, R14, R16, R21, R23, R26–R29 · ADR-0028, ADR-0074, ADR-0037, ADR-0042, ADR-0054 |
| Scale/Scope | ≤ 100 bạn/học sinh (`friend_limits`), 10.000 CCU |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta OpenAPI (lỗi, mô tả), payload 7 event ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Quan hệ xã hội chỉ ở module chat; module khác nhận qua event; hồ sơ/học sinh qua projection |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Mọi bảng `tenant_id` + RLS + soft-delete; test isolation mọi endpoint |
| IV Server-authoritative | ✅ | `Idempotency-Key` + audit cho gửi/chấp nhận lời mời; hạn theo giờ server |
| V Xác thực, phân quyền | ✅ | `game_player` (chỉ học sinh); chỉ thao tác trên quan hệ của chính mình |
| VI An toàn trẻ em | ✅ | Không tìm kiếm tự do; từ chối lặng lẽ; không lộ ai chặn mình; chỉ nickname + avatar |
| VII Một nguồn sự thật, cấu hình | ✅ | `friend_limits`, `friend_request_ttl`, `friend_code_ttl` từ cấu hình; `cross_school_social_enabled` đã đăng ký CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mọi F095-AC-*, test isolation + authorization từng route |

## Project Structure

### Documentation (feature này)

```text
specs/095-friends-block-mute/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
modules/chat/
├── cmd/{api,worker,migrate}/main.go
├── ent/schema/                         # friendrequest.go, friendship.go, friendcode.go, block.go, mute.go, reftenant.go, refstudent.go, refplayerprofile.go
├── migrations/
├── internal/domain/social/             # FriendRequest, Friendship, FriendCode, Block, Mute, ScopePolicy, FriendLimits
├── internal/application/command/       # SendFriendRequest, AcceptFriendRequest, RejectFriendRequest, RemoveFriend, CreateFriendCode,
│                                       # RedeemFriendCode, BlockUser, UnblockUser, MuteUser, UnmuteUser, ExpireFriendRequests
├── internal/application/query/         # ListFriends, ListFriendRequests, ListBlocks, ListMutes
├── internal/application/port/          # SocialRepository, ProfileReader, PresenceCache, RealtimePublisher, Clock, CodeGenerator
├── internal/infrastructure/persistence/social/
├── internal/infrastructure/cache/      # keys.go, presence_cache.go, friend_cache.go
├── internal/presentation/http/public/social.go
├── internal/presentation/consumer/     # projection_identity.go, projection_player_profiles.go
├── internal/presentation/subscriber/presence_fanout.go   # NATS core ew.rt.presence.*
├── internal/presentation/job/expire_friend_requests.go
└── tests/{unit,integration,contract}/
```

**Structure Decision**: Module social thuộc module chat (DOMAIN_MODEL, SOCIAL_SYSTEM §1, AGENTS §6); thư mục `modules/chat` theo LOCAL_DEVELOPMENT §3.

## Phase 0: Research

Không có — SOCIAL_SYSTEM §4–§5, ADR-0054, DATABASE_DESIGN §2.11 đủ; quyết định nhỏ ([Proposed]) ghi ở spec Clarifications.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `FriendRequest` (Accept, Reject, Cancel, Expire), `Friendship` (cặp chuẩn hóa), `ScopePolicy` (cùng tenant, cùng trường hoặc liên trường khi bật, không block), `FriendLimits` | Thuần, Clock inject |
| application | Command/Query ở cây trên; mọi command kiểm `user_type = student` từ Principal | Không permission catalog riêng (route `game_player`) |
| infrastructure | ent repo (`SELECT … FOR UPDATE` theo cặp khi accept), code generator `crypto/rand` A–Z0–9, cache Valkey, publisher | |
| presentation | `sendFriendRequest`, `acceptFriendRequest`, `rejectFriendRequest`, `listFriendRequests`, `listFriends`, `removeFriend`, `createFriendCode`, `redeemFriendCode`, `blockUser`, `unblockUser`, `listBlocks`, `muteUser`, `unmuteUser`, `listMutes` · consumer · subscriber presence · job | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F090 (gateway, `pkg/realtime`), F091 (tín hiệu presence), F070 (`game_profile.updated`), F010/F011 (`tenant.*`, `student.synced`), F016 (`student.cleaned_up`, `user.access_revoked`).
- Event phát: `friend_request.sent`, `friend.added`, `friend.removed`, `user.blocked`, `user.unblocked`, `user.muted`, `user.unmuted`, `audit.recorded`. Consumer: competition (F091, F100, F104), chat nội bộ (F115), notification (F123).

### Bảo mật, tenant, idempotency, audit

- `game_player` + `tenant` middleware; mọi query lọc tenant phiên + chủ thể là người gọi.
- `Idempotency-Key` bắt buộc: `sendFriendRequest`, `acceptFriendRequest`, `createFriendCode`, `redeemFriendCode`; PUT/DELETE block/mute idempotent theo trạng thái.
- Audit: `social.friend_request_sent`, `social.friend_added`, `social.user_blocked`, `social.user_unblocked`, `social.user_muted` (không PII).
- Mã bạn bè: 8 ký tự A–Z0–9 [Proposed], so khớp hằng thời gian, rate limit redeem theo `api_rate_default`.

### Quan sát

- Metric: `eduworld_social_friend_requests_total{result}`, `eduworld_social_blocks_total`, `eduworld_social_presence_fanout_total`, `eduworld_projection_lag_seconds{table}`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
