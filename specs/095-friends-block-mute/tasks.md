# Tasks: F095 — Bạn bè, block, mute (module social)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Skeleton module chat (`module.go` đăng ký vào `src/backend/cmd/core-api` và `src/backend/cmd/worker --profile=chat`, `api/`, `internal/*`, `migrations/`, `src/backend/pkg/config`, outbox relay, rule depguard/arch-check, prefix Valkey `ew:chat:*` và quyền subscribe `ew.rt.presence.>` trong ACL của binary nạp module chat) — file: `src/backend/modules/chat/` — bằng chứng: `modules/chat/module.go` (core-api: route `/social/*` + subscriber presence; worker events: relay + 2 consumer; worker jobs: `expire-friend-requests`), `cmd/internal/wiring` `TestCoreAPIMountsEveryImplementedOperation`, `TestWorkerProfilesRegisterRelaysConsumersAndJobs`, `TestWorkerChatProfileLoadsChatOnly` pass; key Valkey `ew:chat:social:*`. ACL NATS/Valkey theo binary chưa có cấu hình trong `deploy/` (OQ F090-2)
- [x] T002 [P] Cấu hình social: đọc `friend_limits`, `friend_request_ttl`, `friend_code_ttl` từ settings platform/tenant (không hardcode) — file: `src/backend/modules/chat/internal/infrastructure/config/social.go` (biến `EW_SOCIAL_*`, mặc định CONFIG_DEFAULTS; nền platform settings cho `friend_limits` chưa có ở module chat)

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Delta OpenAPI social (lỗi `404/409/422/403`, `redeemFriendCode` `404 FRIEND_CODE_INVALID`, mô tả từ chối lặng lẽ, `maxLength` nickname) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/chat.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=chat` — bằng chứng: `split-public.mjs --check`, `check-maxlength`, `check-problem-codes` 0 lỗi; routematrix 561 operation 0 problem; portal `pnpm gen:api`
- [x] T004 [P] Payload schema `friend_request.sent`, `friend.added`, `friend.removed`, `user.blocked`, `user.unblocked`, `user.muted`, `user.unmuted` — file: `specs/events/payloads/chat/`, `specs/events/README.md` — kiểm: `make specs-lint` — bằng chứng: `validate-schemas` 134 schema 0 lỗi; outbox kiểm schema trong integration (`checkEventSchemas`)
- [x] T005 ent schema `FriendRequest` (+`source`, `responded_at`), `Friendship` (PK UUID), `FriendCode`, `Block`, `Mute`, `RefTenant`, `RefStudent`, `RefPlayerProfile` (+`avatar_config`) với `MaxLen` — file: `src/backend/modules/chat/ent/schema/` — kiểm: `make migrate-diff MOD=chat name=f095_social`, `make migrate-lint MOD=chat` — bằng chứng: migration `20261228000001_f095_social.sql` (khối entddl), `go test ./modules/chat/ent` (`TestMigrationMatchesEnt`) pass; `entlint` 0 lỗi
- [x] T006 Migration: unique partial, RLS, `REVOKE DELETE, TRUNCATE`, `infra.*` — file: `src/backend/modules/chat/migrations/` — bằng chứng: `tenancyguard` 0 lỗi; `TestSocialRLS`
- [x] T007 [P] Consumer projection identity + `game_profile.updated` (`source_version`, `processed_events`, TC-SVC-003) — file: `src/backend/modules/chat/internal/presentation/consumer/` — bằng chứng: `TestProjections` (integration)
- [x] T008 [P] Test RLS bảng `social.*` (TC-TEN-005, TC-TEN-016) — file: `src/backend/modules/chat/tests/integration/rls_social_test.go` — bằng chứng: `TestSocialRLS`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Gửi và nhận lời mời kết bạn trong phạm vi trường (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit `ScopePolicy`, `FriendLimits` F095-AC-02, F095-AC-03: cùng trường; liên trường chỉ khi tenant `unit` bật; khác tenant; block hai chiều; giới hạn pending/bạn/ngày — file: `src/backend/modules/chat/tests/unit/social_scope_test.go` — phải FAIL trước khi code — bằng chứng: `TestScopePolicy`, `TestFriendLimits`, `TestFriendRequestLifecycle`, `TestPairOrder`, `TestPresenceVisible`, `TestFriendCodes` (unit)
- [x] T010 [P] [US1] Integration F095-AC-01, F095-AC-04, F095-AC-05: tạo lời mời + outbox + audit cùng tx; trùng/đã là bạn/chính mình; idempotency; accept tạo đúng một tình bạn (đua 2 accept ngược chiều), lời mời ngược `cancelled` — file: `src/backend/modules/chat/tests/integration/friend_request_test.go` — bằng chứng: `TestSendFriendRequest`, `TestSendFriendRequestScope`, `TestFriendLimits`, `TestAcceptFriendRequest` (5 vòng đua chấp nhận chéo)
- [x] T011 [P] [US1] Integration F095-AC-06, F095-AC-07: reject lặng lẽ (không event, outgoing vẫn `pending`); job hết hạn với Clock giả — file: `src/backend/modules/chat/tests/integration/friend_request_reject_expire_test.go` — bằng chứng: `TestRejectIsSilent`, `TestExpireJob`
- [x] T012 [P] [US1] Integration F095-AC-08, F095-AC-09: friend code định dạng, mã mới thay mã cũ, redeem kiểm phạm vi, mã sai/hết hạn → `FRIEND_CODE_INVALID`, sai định dạng → 422; quét OpenAPI không có route tìm người chơi — file: `src/backend/modules/chat/tests/integration/friend_code_test.go` — bằng chứng: `TestFriendCodes` (integration), `TestNoPlayerSearch` (contract, quét OpenAPI)
- [x] T013 [P] [US1] Test authorization + tenant isolation mọi route social F095-AC-10 (5 role nhân sự → `GAME_ACCESS_STUDENT_ONLY`; ID của tenant khác → 404; sinh từ OpenAPI, TC-AUTH-014, TC-TEN-001) — file: `src/backend/modules/chat/tests/integration/social_authz_test.go` — bằng chứng: `TestSocialRoutesStaffDenied` (14 route × 5 role), `TestSocialTenantIsolation`; routematrix 0 problem
- [x] T028 [P] [US1] Test F095-AC-23: học sinh tenant `common` gửi lời mời, tạo mã bạn bè, thêm bằng mã → `403 SOCIAL_SCOPE_VIOLATION`, không ghi `friend_requests`; sau khi System Admin bật lại (audit) gửi được — file: `src/backend/modules/chat/tests/integration/common_tenant_friends_test.go` — phải FAIL trước khi code — bằng chứng: `TestCommonTenantFriendsOff`; bootstrap `social.friends_enabled` trong identity `TestGameLogin`, `TestFeatureFlagsAndMenuCatalog`

### Implementation cho User Story 1

- [x] T014 [P] [US1] Domain `FriendRequest`, `Friendship`, `FriendCode`, `ScopePolicy`, `FriendLimits` — file: `src/backend/modules/chat/internal/domain/social/`
- [x] T015 [US1] Command `SendFriendRequest`, `AcceptFriendRequest`, `RejectFriendRequest`, `CreateFriendCode`, `RedeemFriendCode`, `ExpireFriendRequests` + repo ent — file: `src/backend/modules/chat/internal/application/command/`, `src/backend/modules/chat/internal/infrastructure/persistence/social/`
- [x] T016 [US1] Handler HTTP (middleware `game_player`, `tenant`, `idempotency`) + job hết hạn — file: `src/backend/modules/chat/internal/presentation/http/public/social.go`, `src/backend/modules/chat/internal/presentation/job/expire_friend_requests.go` (`presentation/http/routes.go`, `presentation/job/jobs.go`)
- [x] T029 [US1] Kiểm `common_tenant_social_defaults` (projection `ref.tenants.kind` + cờ bật lại của F120 T059) trong `SendFriendRequest`, `CreateFriendCode`, `AddFriendByCode` — file: `src/backend/modules/chat/internal/application/command/social_scope_guard.go` (hiện thực trong `Tenant.FriendsEnabled` + `command.Social` — tên `AddFriendByCode` = `RedeemFriendCode`; cờ bật lại `common_social_enabled` [Confirmed] tới khi có F120 T059)

**Checkpoint**: kết bạn chạy độc lập.

## Phase 4: User Story 2 — Danh sách bạn bè và hủy kết bạn (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Integration F095-AC-11, F095-AC-12, F095-AC-13: `listFriends` chỉ bạn của mình, `PlayerCard` không tên thật/trường/lớp/điểm; `removeFriend` soft-delete + `friend.removed`, không thông báo, người lạ → 404; `listFriendRequests` hai chiều chỉ của mình — file: `src/backend/modules/chat/tests/integration/friends_list_test.go` — phải FAIL trước khi code — bằng chứng: `TestListFriends`, `TestRemoveFriend`, `TestListFriendRequests`

### Implementation cho User Story 2

- [x] T018 [US2] Query `ListFriends`, `ListFriendRequests`; command `RemoveFriend`; cache `ew:chat:social:friends:<id>` — file: `src/backend/modules/chat/internal/application/query/friends.go`, `src/backend/modules/chat/internal/application/command/remove_friend.go`, `src/backend/modules/chat/internal/infrastructure/cache/friend_cache.go` (`application/query/social.go`, `application/command/relations.go`, `infrastructure/cache/social.go`)

## Phase 5: User Story 3 — Chặn và tắt tiếng (Priority: P1)

### Tests cho User Story 3

- [x] T019 [P] [US3] Integration F095-AC-14, F095-AC-15, F095-AC-16: chặn idempotent, kết thúc tình bạn, hủy lời mời hai chiều, `user.blocked` + `friend.removed` + audit cùng tx; lời mời hai chiều bị từ chối; bỏ chặn không khôi phục bạn (TC-CHAT-005) — file: `src/backend/modules/chat/tests/integration/block_test.go` — phải FAIL trước khi code — bằng chứng: `TestBlockUser`
- [x] T020 [P] [US3] Integration F095-AC-17, F095-AC-18, F095-AC-19: mute/unmute idempotent + event; tự chặn → 422; đích tenant khác → 404; `listBlocks`/`listMutes` chỉ của mình, không có API "ai chặn tôi" (TC-SOC-003) — file: `src/backend/modules/chat/tests/integration/mute_list_test.go` — bằng chứng: `TestMuteUser`, `TestRelationTargetsAndLists`
- [x] T021 [P] [US3] Contract test: payload `user.blocked`/`friend.added` khớp schema mà competition (F091) tiêu thụ — file: `src/backend/modules/chat/tests/contract/social_events_test.go` — đặt ở phía bên nhận vì module chat không được import `competition/internal`: `modules/competition/tests/contract/events_test.go` (`TestConsumersAcceptOwnerExamples` với ví dụ schema chat, `TestSocialProjectionRefusesInvalidPayloads`), integration `TestSocialProjection` (competition); phía chat: `checkEventSchemas` trên outbox

### Implementation cho User Story 3

- [x] T022 [US3] Domain `Block`, `Mute`; command `BlockUser`, `UnblockUser`, `MuteUser`, `UnmuteUser`; query `ListBlocks`, `ListMutes` + handler — file: `src/backend/modules/chat/internal/domain/social/block.go`, `src/backend/modules/chat/internal/application/command/block_user.go`, `src/backend/modules/chat/internal/presentation/http/public/social.go` (`domain/social/social.go`, `application/command/relations.go`)

## Phase 6: User Story 4 — Bạn bè thấy trạng thái online (Priority: P2)

### Tests cho User Story 4

- [x] T023 [P] [US4] Integration F095-AC-20, F095-AC-21, F095-AC-22: tín hiệu presence → `presence.friend.updated` chỉ tới bạn online không chặn; ẩn → `offline`; thi → activity chung; `listFriends.presence` theo cache, mặc định `offline` — file: `src/backend/modules/chat/tests/integration/presence_fanout_test.go` — phải FAIL trước khi code — bằng chứng: `TestPresenceFanout` (Valkey + NATS thật, ≤ 2 s), gateway `TestPresenceRefreshSignal`

### Implementation cho User Story 4

- [x] T024 [US4] Subscriber `ew.rt.presence.>` + `PresenceCache` + publish `ew:rt:user:<friend>` qua `src/backend/pkg/realtime` — file: `src/backend/modules/chat/internal/presentation/subscriber/presence_fanout.go`, `src/backend/modules/chat/internal/infrastructure/cache/presence_cache.go` (`presentation/subscriber/presence_fanout.go`, `infrastructure/cache/social.go`, `application/command/presence.go`)

## Phase 7: Polish & Cross-Cutting

- [x] T025 [P] Cập nhật DATABASE_DESIGN §2.11 (PK UUID + unique partial, cột mới), §2.17 (`avatar_config`); VALKEY_DESIGN §3.1 (key `social:*`); API_DESIGN §8 (rate lời mời theo `friend_limits`); đăng ký `cross_school_social_enabled` (CONFIG_DEFAULTS, MULTITENANCY); AGENTS R23 nêu `/social/*` phía người chơi (đề xuất trong PR); CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/API_DESIGN.md`, `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/06-architecture/MULTITENANCY.md`, `CHANGELOG.md` — đã sửa DATABASE_DESIGN §2.11/§2.17, VALKEY_DESIGN §3.1, API_DESIGN §8, LOCAL_DEVELOPMENT, CHANGELOG; `cross_school_social_enabled` đã có ở CONFIG_DEFAULTS/MULTITENANCY; AGENTS R23 chỉ đề xuất (spec A-2)
- [x] T026 [P] Metric `eduworld_social_*` — file: `src/backend/modules/chat/internal/infrastructure/observability/metrics.go`
- [ ] T027 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — chưa chạy tay trên stack đầy đủ (`make up`); 15 kịch bản quickstart được phủ bởi test integration ở trên

## Dependencies & Execution Order

- Setup → Foundational (T007 projection trước mọi story) → US1 → US2, US3 song song → US4 (cần F091) → Polish.

## Definition of Done

- [x] Mọi tiêu chí F095-AC-* có test pass (ghi mã test/TC) — AC-01, 04: `TestSendFriendRequest`; AC-02: `TestSendFriendRequestScope`, `TestSocialTenantIsolation`; AC-03: `TestFriendLimits` (unit + integration); AC-05: `TestAcceptFriendRequest`; AC-06: `TestRejectIsSilent`; AC-07: `TestExpireJob`; AC-08: `TestFriendCodes`; AC-09: `TestNoPlayerSearch`; AC-10: `TestSocialRoutesStaffDenied`; AC-11: `TestListFriends`; AC-12: `TestRemoveFriend`; AC-13: `TestListFriendRequests`; AC-14, 15, 16: `TestBlockUser`; AC-17: `TestMuteUser`; AC-18, 19: `TestRelationTargetsAndLists`; AC-20, 21, 22: `TestPresenceFanout`, `TestPresenceVisible`, `TestPresenceRefreshSignal`; AC-23: `TestCommonTenantFriendsOff`, identity `TestGameLogin`, `TestFeatureFlagsAndMenuCatalog`
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` xanh cho module `chat` (lệnh gốc của Makefile, chạy tay trên Windows)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
