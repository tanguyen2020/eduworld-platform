# Tasks: F095 — Bạn bè, block, mute (module social)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Skeleton module chat (`module.go` đăng ký vào `src/backend/cmd/core-api` và `src/backend/cmd/worker --profile=chat`, `api/`, `internal/*`, `migrations/`, `src/backend/pkg/config`, outbox relay, rule depguard/arch-check, prefix Valkey `ew:chat:*` và quyền subscribe `ew.rt.presence.>` trong ACL của binary nạp module chat) — file: `src/backend/modules/chat/`
- [ ] T002 [P] Cấu hình social: đọc `friend_limits`, `friend_request_ttl`, `friend_code_ttl` từ settings platform/tenant (không hardcode) — file: `src/backend/modules/chat/internal/config/social.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta OpenAPI social (lỗi `404/409/422/403`, `redeemFriendCode` `404 FRIEND_CODE_INVALID`, mô tả từ chối lặng lẽ, `maxLength` nickname) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/chat.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=chat`
- [ ] T004 [P] Payload schema `friend_request.sent`, `friend.added`, `friend.removed`, `user.blocked`, `user.unblocked`, `user.muted`, `user.unmuted` — file: `specs/events/payloads/chat/`, `specs/events/README.md` — kiểm: `make specs-lint`
- [ ] T005 ent schema `FriendRequest` (+`source`, `responded_at`), `Friendship` (PK UUID), `FriendCode`, `Block`, `Mute`, `RefTenant`, `RefStudent`, `RefPlayerProfile` (+`avatar_config`) với `MaxLen` — file: `src/backend/modules/chat/ent/schema/` — kiểm: `make migrate-diff MOD=chat name=f095_social`, `make migrate-lint MOD=chat`
- [ ] T006 Migration: unique partial, RLS, `REVOKE DELETE, TRUNCATE`, `infra.*` — file: `src/backend/modules/chat/migrations/`
- [ ] T007 [P] Consumer projection identity + `game_profile.updated` (`source_version`, `processed_events`, TC-SVC-003) — file: `src/backend/modules/chat/internal/presentation/consumer/`
- [ ] T008 [P] Test RLS bảng `social.*` (TC-TEN-005, TC-TEN-016) — file: `src/backend/modules/chat/tests/integration/rls_social_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Gửi và nhận lời mời kết bạn trong phạm vi trường (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit `ScopePolicy`, `FriendLimits` F095-AC-02, F095-AC-03: cùng trường; liên trường chỉ khi tenant `unit` bật; khác tenant; block hai chiều; giới hạn pending/bạn/ngày — file: `src/backend/modules/chat/tests/unit/social_scope_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Integration F095-AC-01, F095-AC-04, F095-AC-05: tạo lời mời + outbox + audit cùng tx; trùng/đã là bạn/chính mình; idempotency; accept tạo đúng một tình bạn (đua 2 accept ngược chiều), lời mời ngược `cancelled` — file: `src/backend/modules/chat/tests/integration/friend_request_test.go`
- [ ] T011 [P] [US1] Integration F095-AC-06, F095-AC-07: reject lặng lẽ (không event, outgoing vẫn `pending`); job hết hạn với Clock giả — file: `src/backend/modules/chat/tests/integration/friend_request_reject_expire_test.go`
- [ ] T012 [P] [US1] Integration F095-AC-08, F095-AC-09: friend code định dạng, mã mới thay mã cũ, redeem kiểm phạm vi, mã sai/hết hạn → `FRIEND_CODE_INVALID`, sai định dạng → 422; quét OpenAPI không có route tìm người chơi — file: `src/backend/modules/chat/tests/integration/friend_code_test.go`
- [ ] T013 [P] [US1] Test authorization + tenant isolation mọi route social F095-AC-10 (5 role nhân sự → `GAME_ACCESS_STUDENT_ONLY`; ID của tenant khác → 404; sinh từ OpenAPI, TC-AUTH-014, TC-TEN-001) — file: `src/backend/modules/chat/tests/integration/social_authz_test.go`
- [ ] T028 [P] [US1] Test F095-AC-23: học sinh tenant `common` gửi lời mời, tạo mã bạn bè, thêm bằng mã → `403 SOCIAL_SCOPE_VIOLATION`, không ghi `friend_requests`; sau khi System Admin bật lại (audit) gửi được — file: `src/backend/modules/chat/tests/integration/common_tenant_friends_test.go` — phải FAIL trước khi code

### Implementation cho User Story 1

- [ ] T014 [P] [US1] Domain `FriendRequest`, `Friendship`, `FriendCode`, `ScopePolicy`, `FriendLimits` — file: `src/backend/modules/chat/internal/domain/social/`
- [ ] T015 [US1] Command `SendFriendRequest`, `AcceptFriendRequest`, `RejectFriendRequest`, `CreateFriendCode`, `RedeemFriendCode`, `ExpireFriendRequests` + repo ent — file: `src/backend/modules/chat/internal/application/command/`, `src/backend/modules/chat/internal/infrastructure/persistence/social/`
- [ ] T016 [US1] Handler HTTP (middleware `game_player`, `tenant`, `idempotency`) + job hết hạn — file: `src/backend/modules/chat/internal/presentation/http/public/social.go`, `src/backend/modules/chat/internal/presentation/job/expire_friend_requests.go`
- [ ] T029 [US1] Kiểm `common_tenant_social_defaults` (projection `ref.tenants.kind` + cờ bật lại của F120 T059) trong `SendFriendRequest`, `CreateFriendCode`, `AddFriendByCode` — file: `src/backend/modules/chat/internal/application/command/social_scope_guard.go`

**Checkpoint**: kết bạn chạy độc lập.

## Phase 4: User Story 2 — Danh sách bạn bè và hủy kết bạn (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Integration F095-AC-11, F095-AC-12, F095-AC-13: `listFriends` chỉ bạn của mình, `PlayerCard` không tên thật/trường/lớp/điểm; `removeFriend` soft-delete + `friend.removed`, không thông báo, người lạ → 404; `listFriendRequests` hai chiều chỉ của mình — file: `src/backend/modules/chat/tests/integration/friends_list_test.go` — phải FAIL trước khi code

### Implementation cho User Story 2

- [ ] T018 [US2] Query `ListFriends`, `ListFriendRequests`; command `RemoveFriend`; cache `ew:chat:social:friends:<id>` — file: `src/backend/modules/chat/internal/application/query/friends.go`, `src/backend/modules/chat/internal/application/command/remove_friend.go`, `src/backend/modules/chat/internal/infrastructure/cache/friend_cache.go`

## Phase 5: User Story 3 — Chặn và tắt tiếng (Priority: P1)

### Tests cho User Story 3

- [ ] T019 [P] [US3] Integration F095-AC-14, F095-AC-15, F095-AC-16: chặn idempotent, kết thúc tình bạn, hủy lời mời hai chiều, `user.blocked` + `friend.removed` + audit cùng tx; lời mời hai chiều bị từ chối; bỏ chặn không khôi phục bạn (TC-CHAT-005) — file: `src/backend/modules/chat/tests/integration/block_test.go` — phải FAIL trước khi code
- [ ] T020 [P] [US3] Integration F095-AC-17, F095-AC-18, F095-AC-19: mute/unmute idempotent + event; tự chặn → 422; đích tenant khác → 404; `listBlocks`/`listMutes` chỉ của mình, không có API "ai chặn tôi" (TC-SOC-003) — file: `src/backend/modules/chat/tests/integration/mute_list_test.go`
- [ ] T021 [P] [US3] Contract test: payload `user.blocked`/`friend.added` khớp schema mà competition (F091) tiêu thụ — file: `src/backend/modules/chat/tests/contract/social_events_test.go`

### Implementation cho User Story 3

- [ ] T022 [US3] Domain `Block`, `Mute`; command `BlockUser`, `UnblockUser`, `MuteUser`, `UnmuteUser`; query `ListBlocks`, `ListMutes` + handler — file: `src/backend/modules/chat/internal/domain/social/block.go`, `src/backend/modules/chat/internal/application/command/block_user.go`, `src/backend/modules/chat/internal/presentation/http/public/social.go`

## Phase 6: User Story 4 — Bạn bè thấy trạng thái online (Priority: P2)

### Tests cho User Story 4

- [ ] T023 [P] [US4] Integration F095-AC-20, F095-AC-21, F095-AC-22: tín hiệu presence → `presence.friend.updated` chỉ tới bạn online không chặn; ẩn → `offline`; thi → activity chung; `listFriends.presence` theo cache, mặc định `offline` — file: `src/backend/modules/chat/tests/integration/presence_fanout_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T024 [US4] Subscriber `ew.rt.presence.>` + `PresenceCache` + publish `ew:rt:user:<friend>` qua `src/backend/pkg/realtime` — file: `src/backend/modules/chat/internal/presentation/subscriber/presence_fanout.go`, `src/backend/modules/chat/internal/infrastructure/cache/presence_cache.go`

## Phase 7: Polish & Cross-Cutting

- [ ] T025 [P] Cập nhật DATABASE_DESIGN §2.11 (PK UUID + unique partial, cột mới), §2.17 (`avatar_config`); VALKEY_DESIGN §3.1 (key `social:*`); API_DESIGN §8 (rate lời mời theo `friend_limits`); đăng ký `cross_school_social_enabled` (CONFIG_DEFAULTS, MULTITENANCY); AGENTS R23 nêu `/social/*` phía người chơi (đề xuất trong PR); CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/API_DESIGN.md`, `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/06-architecture/MULTITENANCY.md`, `CHANGELOG.md`
- [ ] T026 [P] Metric `eduworld_social_*` — file: `src/backend/modules/chat/internal/infrastructure/observability/metrics.go`
- [ ] T027 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T007 projection trước mọi story) → US1 → US2, US3 song song → US4 (cần F091) → Polish.

## Definition of Done

- [ ] Mọi tiêu chí F095-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` xanh cho module `chat`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
