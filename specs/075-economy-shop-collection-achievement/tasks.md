# Tasks: F075 — Kinh tế Xu, Shop cosmetic, inventory, collection, achievement, badge

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 (kết quả: `internal/domain/economy`, `domain/collection`, `domain/achievement`; ranh giới tầng do `.golangci.arch.yml` sinh sẵn (archgen 0 vi phạm), không cần file depguard riêng) Tạo package `internal/domain/economy`, `internal/domain/collection`, `internal/domain/achievement` + depguard — file: `src/backend/modules/game/internal/domain/economy/doc.go`, `src/backend/modules/game/.golangci.yml`
- [x] T002 (kết quả: `internal/infrastructure/config/economy.go` (env `EW_GAME_DUPLICATE_ITEM_XU`, `EW_GAME_SHOP_PURCHASE_LIMIT_DEFAULT`); `xu_daily_cap` = `gamification.currencies.daily_cap`; `badges_displayed_max` = 3 slot `badge_showcase` của CHARACTER_SYSTEM (F070)) [P] Cấu hình `xu_daily_cap`, `duplicate_item_xu`, `shop_purchase_limit_default`, `badges_displayed_max` từ platform settings — file: `src/backend/modules/game/internal/config/economy.go`
- [x] T003 (kết quả: `src/backend/modules/game/seed/economy/` (cùng chỗ seed F074), nạp bằng `cmd/seed`) [P] Seed config: currency `xu`, item, shop item (một gán riêng `th-demo-a`), collection knowledge card, achievement (một ẩn) — file: `deploy/seed/game-config/items/`, `deploy/seed/game-config/shop-items/`, `deploy/seed/game-config/collections/`, `deploy/seed/game-config/achievements/`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 (kết quả: ADR-0100 + `specs/breaking-allowlist.yaml` (48 mục)) Sửa `shop-item.schema.json`, `reward-rule.schema.json` (grant `currency`/`collection_item`); thêm `item.schema.json`, `collection.schema.json`; OpenAPI `item_id` uuid → mã `maxLength: 64` (`InventoryItem`, `ShopItem`, `EconomyAdjustmentRequest`) — file: `specs/schemas/`, `specs/openapi/modules/game.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [x] T005 [P] OpenAPI: `listInventory` query `kind`; `purchaseShopItem` `422`/`428`; `approveEconomyAdjustment` header + `422`; thêm `listEconomyAdjustments`, `rejectEconomyAdjustment` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [x] T006 [P] Payload event `currency.credited/debited`, `shop.item_purchased`, `item.granted`, `collection.item_added/completed`, `achievement.progressed/unlocked`, `economy.adjusted` — file: `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [x] T007 (kết quả: `ent/schema/economy.go` (+ cột phát hành `item_definition_versions`)) ent schema theo data-model (ledger AppendOnly, `inventory_items.quantity`, `shop_item_versions.event_id/season_id`, `economy_adjustments` Mới, `student_achievements.progress_state`) — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f075_economy`, `make migrate-lint MOD=game`
- [x] T008 (kết quả: `migrations/20261229000001_f075_economy.sql` (chưa partition — Clarifications)) Migration: partition tháng ledger, CHECK `balance >= 0`, unique, RLS, `REVOKE UPDATE, DELETE, TRUNCATE` ledger — file: `src/backend/modules/game/migrations/`
- [x] T009 (kết quả: `TestWalletReconcileAndAppendOnly` (economy_wallet_test.go)) [P] Test RLS + append-only F075-AC-05 cho `wallets`, `currency_ledger`, `item_ledger`, `inventory_items`, `economy_adjustments` — file: `src/backend/modules/game/tests/integration/economy_rls_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Ví Xu và sổ cái (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T010 (kết quả: `internal/domain/economy/wallet_test.go`) [P] [US1] Unit test `economy.Wallet` (credit/debit, không âm, cap còn lại) cho F075-AC-01, F075-AC-02, F075-AC-03 — file: `src/backend/modules/game/internal/domain/economy/wallet_test.go` — phải FAIL trước khi code
- [x] T011 (kết quả: `economy_wallet_test.go` (`TestCurrencyGrantInRewardTransaction`, `TestCurrencyDailyCapAndDiminishing`, `TestWalletReconcileAndAppendOnly`)) [P] [US1] Integration test applier `currency` F075-AC-01, F075-AC-02 (cùng transaction RewardGrant, cap, lặp event) + job đối soát F075-AC-03 (cố ý lệch → cảnh báo, không sửa) — file: `src/backend/modules/game/tests/integration/currency_applier_test.go`
- [x] T012 (kết quả: `TestWalletAPI` (economy_wallet_test.go)) [P] [US1] API test F075-AC-04: `getWallet`, `listWalletLedger` chỉ của mình, không id kỹ thuật — file: `src/backend/modules/game/tests/integration/wallet_api_test.go`

### Implementation cho User Story 1

- [x] T013 [P] [US1] Domain `economy.Wallet` — file: `src/backend/modules/game/internal/domain/economy/wallet.go`
- [x] T014 (kết quả: `command.Economy.GrantCurrency` + adapter `infrastructure/reward` (`WithEconomy`); hệ số giảm dần dùng `reward.Award.Factor`) [US1] Applier `currency` (bucket cap `currency:xu`, giảm dần) đăng ký `GrantApplierRegistry` — file: `src/backend/modules/game/internal/infrastructure/reward/currency.go`
- [x] T015 (kết quả: `query/economy.go`, `presentation/http/economy.go`, `presentation/job/economy.go`) [US1] Query `GetWallet`, `ListWalletLedger` + handler; job `wallet-reconcile` — file: `src/backend/modules/game/internal/application/query/get_wallet.go`, `src/backend/modules/game/internal/presentation/http/public/wallet.go`, `src/backend/modules/game/internal/presentation/job/wallet_reconcile.go`

## Phase 4: User Story 2 — Shop cosmetic (Priority: P1)

### Tests cho User Story 2

- [x] T016 (kết quả: trong `domain/economy/wallet_test.go` (`TestListingAvailability`, `TestPurchaseCheck`)) [P] [US2] Unit test `economy.PurchasePolicy` cho F075-AC-10 (limit, sở hữu, thời gian bán, sự kiện/mùa không chạy, sai khối) — file: `src/backend/modules/game/internal/domain/economy/purchase_test.go` — phải FAIL trước khi code
- [x] T017 (kết quả: `economy_shop_test.go` (`TestShopListing`, `TestShopPurchase`)) [P] [US2] Integration test F075-AC-06, F075-AC-07, F075-AC-08: lọc theo tenant/khối/thời gian; một transaction đủ bút toán + outbox; cùng key → cùng response; thiếu key 428; key khác body 422; audit — file: `src/backend/modules/game/tests/integration/shop_purchase_test.go`
- [x] T018 (kết quả: `TestShopPurchaseRace` 50 cặp đồng thời (1.000 cặp của SC-001 là thử tải, chưa chạy)) [P] [US2] Test race F075-AC-09: 1.000 cặp request đồng thời — file: `src/backend/modules/game/tests/integration/shop_purchase_race_test.go`
- [x] T019 (kết quả: `tests/contract/no_real_money_test.go`, `economyschema_test.go`, `wallet_test.go` `TestContentValidation`) [P] [US2] Contract/static test F075-AC-11: quét spec không giá tiền thật/random; module `shop-items`/`items` từ chối `currency_code ≠ xu`, giá ≤ 0, `tradable = true` (TC-ECO-003) — file: `src/backend/modules/game/tests/contract/no_real_money_test.go`
- [x] T020 (kết quả: `TestEconomyPlayerAuthz` + `TestPlayerRoutesStaffDenied` (mọi player route)) [P] [US2] Test authorization + tenant isolation F075-AC-12: mọi route học sinh × token nhân sự → 403; header tenant khác → `TENANT_MISMATCH`; mặt hàng tenant khác → `SHOP_ITEM_UNAVAILABLE` — file: `src/backend/modules/game/tests/integration/economy_player_authz_test.go`

### Implementation cho User Story 2

- [x] T021 [P] [US2] Domain `economy.PurchasePolicy` — file: `src/backend/modules/game/internal/domain/economy/purchase.go`
- [x] T022 (kết quả: `command/economy_ops.go` (`Purchase`), idempotency bằng middleware `pkg/httpx` + `idempotency.SetResult`) [US2] Command `PurchaseShopItem` (idempotency middleware `src/backend/pkg/httpx`, khóa ví + `(student_id, item_id)`, outbox) + query `ListShopItems` + handler — file: `src/backend/modules/game/internal/application/command/purchase_shop_item.go`, `src/backend/modules/game/internal/presentation/http/public/shop.go`
- [x] T023 (kết quả: lõi phát hành F071 (`command/publish_economy.go`, item type `item`/`shop_item`; config type HTTP ở F078)) [US2] Module config `items`, `currencies`, `shop-items` (validate schema + invariant) — file: `src/backend/modules/game/internal/infrastructure/gameconfig/item_module.go`, `src/backend/modules/game/internal/infrastructure/gameconfig/shop_item_module.go`

## Phase 5: User Story 3 — Inventory, vật phẩm trùng, trang bị (Priority: P1)

### Tests cho User Story 3

- [x] T024 (kết quả: `economy_inventory_test.go` `TestItemGrantsAndDuplicates`) [P] [US3] Integration test F075-AC-13, F075-AC-14: grant item/badge, xếp chồng, grant trùng → quy đổi `duplicate_item_xu` — file: `src/backend/modules/game/tests/integration/item_applier_test.go` — phải FAIL trước khi code
- [x] T025 (kết quả: `TestInventoryAndEquip` + contract `no_real_money_test.go`) [P] [US3] API test F075-AC-15, F075-AC-16, F075-AC-17: lọc `kind`; `updateAvatar` không sở hữu/sai slot; không có route ghi inventory hay trao đổi (quét spec) — file: `src/backend/modules/game/tests/integration/inventory_api_test.go`

### Implementation cho User Story 3

- [x] T026 [US3] Applier `item`, `badge` + `DuplicatePolicy` — file: `src/backend/modules/game/internal/infrastructure/reward/item.go`, `src/backend/modules/game/internal/domain/economy/duplicate.go`
- [x] T027 [US3] Query `ListInventory`, `ListInventoryHistory`, `InventoryOwnership` (dùng bởi `updateAvatar` của F070) + handler — file: `src/backend/modules/game/internal/application/query/list_inventory.go`, `src/backend/modules/game/internal/presentation/http/public/inventory.go`

## Phase 6: User Story 4 — Collection và knowledge card (Priority: P2)

### Tests cho User Story 4

- [x] T028 (kết quả: `economy_collection_test.go` `TestKnowledgeCardsAndCollection`) [P] [US4] Integration test F075-AC-18, F075-AC-19: tiến độ collection, hoàn thành thưởng một lần; card cấp khi `proficient`, không thu hồi khi giảm — file: `src/backend/modules/game/tests/integration/collection_test.go` — phải FAIL trước khi code
- [x] T029 (kết quả: gộp vào `TestKnowledgeCardsAndCollection`) [P] [US4] API test F075-AC-20: collection ẩn tới món đầu; "cách nhận" song ngữ — file: `src/backend/modules/game/tests/integration/collection_api_test.go`

### Implementation cho User Story 4

- [x] T030 [US4] Domain `collection` + applier `collection_item` + consumer `game.collection_on_mastery` + module `collections` — file: `src/backend/modules/game/internal/domain/collection/`, `src/backend/modules/game/internal/presentation/consumer/collection_on_mastery.go`, `src/backend/modules/game/internal/infrastructure/gameconfig/collection_module.go`
- [x] T031 [US4] Query `ListCollections`, `GetCollection` + handler — file: `src/backend/modules/game/internal/presentation/http/public/collections.go`

## Phase 7: User Story 5 — Achievement và badge (Priority: P2)

### Tests cho User Story 5

- [x] T032 (kết quả: `internal/domain/achievement/achievement_test.go`) [P] [US5] Unit test `achievement.Evaluator` F075-AC-21, F075-AC-24 (filters, `count`, `distinct_days`, tier, version mới không giảm) — file: `src/backend/modules/game/internal/domain/achievement/evaluator_test.go` — phải FAIL trước khi code
- [x] T033 (kết quả: `economy_achievement_test.go` `TestAchievements`) [P] [US5] Integration test F075-AC-21, F075-AC-22: event lặp mở khóa một lần; `listAchievements` ẩn điều kiện — file: `src/backend/modules/game/tests/integration/achievement_test.go`
- [x] T034 (kết quả: `TestAchievementPublishChecks` (integration, qua lõi phát hành) + domain `TestValidateRefusesUnsafeAchievements`) [P] [US5] Test module `achievements` F075-AC-23: trigger lạ, điều kiện thời gian chơi/đăng nhập/tin nhắn/điểm chính thức → 422 — file: `src/backend/modules/game/tests/unit/achievement_module_test.go`

### Implementation cho User Story 5

- [x] T035 [US5] Domain `achievement`, command `EvaluateAchievements`, consumer `game.achievement_evaluator`, module `achievements`, query `ListAchievements` + handler — file: `src/backend/modules/game/internal/domain/achievement/`, `src/backend/modules/game/internal/presentation/consumer/achievement_evaluator.go`, `src/backend/modules/game/internal/presentation/http/public/achievements.go`

## Phase 8: User Story 6 — Bút toán bù có người duyệt thứ hai (Priority: P2)

### Tests cho User Story 6

- [x] T036 (kết quả: `economy_adjustment_test.go` `TestEconomyAdjustments` (đã xử lý → 422 theo AC-26)) [P] [US6] Integration test F075-AC-25, F075-AC-26: tạo pending, tự duyệt 403, duyệt ghi bút toán + event + audit, âm số dư 409, duyệt lại 409 — file: `src/backend/modules/game/tests/integration/economy_adjustment_test.go` — phải FAIL trước khi code
- [x] T037 (kết quả: `TestEconomyAdjustmentAuthz` (không MFA — ADR-0094)) [P] [US6] Test authorization F075-AC-27: role tenant/platform thiếu quyền, thiếu MFA, `azp` sai → 403 + `authz.denied`; `reason` > 1000 ký tự → 422 — file: `src/backend/modules/game/tests/integration/economy_adjustment_authz_test.go`

### Implementation cho User Story 6

- [x] T038 [US6] Domain `economy.Adjustment`, command `CreateEconomyAdjustment`/`ApproveEconomyAdjustment`/`RejectEconomyAdjustment`, query `ListEconomyAdjustments` + handler (`/admin/economy/*`) — file: `src/backend/modules/game/internal/application/command/approve_economy_adjustment.go`, `src/backend/modules/game/internal/presentation/http/public/economy_admin.go`

## Phase 9: Polish & Cross-Cutting

- [x] T039 [P] Cập nhật DATABASE_DESIGN §2.8a (`quantity`, `economy_adjustments`, `event_id`/`season_id`, `progress_state`, `name_en`), INVENTORY_ECONOMY §8–9, EVENT_DESIGN §4.6, CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/01-game-design/INVENTORY_ECONOMY.md`, `docs/07-backend/EVENT_DESIGN.md`, `CHANGELOG.md`
- [ ] T040 (chưa làm — realm local không đổi; four-eyes local dùng hai tài khoản `system_admin`, Câu hỏi mở F075-1) [P] Thêm tài khoản demo platform thứ hai có `game.economy.adjust` vào realm local và LOCAL_DEVELOPMENT — file: `deploy/keycloak/realm-eduworld.dev.json`, `docs/11-operations/LOCAL_DEVELOPMENT.md`
- [x] T041 (kết quả: `infrastructure/observability/economy_metrics.go`) [P] Metric `eduworld_game_currency_*`, `eduworld_game_shop_purchase_total`, `eduworld_game_ledger_mismatch_total{ledger="currency"}` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`
- [x] T042 (kết quả: kịch bản quickstart được phủ bằng test tích hợp — bảng AC → test dưới đây; chạy tay trên stack local chưa làm) Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2, US3 (song song sau T014) → US4, US5 → US6 → Polish.
- Phụ thuộc F074 (`GrantApplierRegistry`, cap ngày), F070 (`updateAvatar`), F031 (`learning_objective.progressed`).

## Definition of Done

- [ ] Mọi tiêu chí F075-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game`; `make test-contract` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Bảng tiêu chí → test (bằng chứng 2026-11-23)

| Tiêu chí | Test (`src/backend/modules/game/…`) |
|---|---|
| F075-AC-01 | `tests/integration/economy_wallet_test.go` `TestCurrencyGrantInRewardTransaction`; `internal/domain/economy/wallet_test.go` `TestWalletNeverNegative` |
| F075-AC-02 | `TestCurrencyDailyCapAndDiminishing`; domain `TestEarnableUnderDailyCap` |
| F075-AC-03 | `TestWalletReconcileAndAppendOnly` (CHECK số dư, job `wallet-reconcile`: metric + audit, không sửa); domain `TestWalletNeverNegative` |
| F075-AC-04 | `TestWalletAPI` |
| F075-AC-05 | `TestWalletReconcileAndAppendOnly` (`game_app`/`game_platform` UPDATE/DELETE bị từ chối, RLS tenant) |
| F075-AC-06 | `economy_shop_test.go` `TestShopListing` |
| F075-AC-07 | `TestShopPurchase` |
| F075-AC-08 | `TestShopPurchase` (replay, 428, key khác body → 409 theo API_DESIGN — Clarifications, audit) |
| F075-AC-09 | `TestShopPurchaseRace` (50 cặp đồng thời); domain `TestWalletNeverNegative` |
| F075-AC-10 | `TestShopPurchase`, `TestShopListing`; domain `TestPurchaseCheck`, `TestListingAvailability` |
| F075-AC-11 | `tests/contract/no_real_money_test.go`; `internal/infrastructure/economyschema/economyschema_test.go` `TestValidate`; domain `TestContentValidation` |
| F075-AC-12 | `TestEconomyPlayerAuthz`, `player_routes_authz_test.go` `TestPlayerRoutesStaffDenied`, `TestShopListing` (409 SHOP_ITEM_UNAVAILABLE) |
| F075-AC-13 | `economy_inventory_test.go` `TestItemGrantsAndDuplicates` |
| F075-AC-14 | `TestItemGrantsAndDuplicates` (quy đổi theo độ hiếm, `duplicate_xu` trong `reward.granted`, grant đồng thời) |
| F075-AC-15 | `TestInventoryAndEquip` |
| F075-AC-16 | `TestInventoryAndEquip` (`updateAvatar`: `ITEM_NOT_OWNED`, `EQUIP_SLOT_INVALID`); contract `no_real_money_test.go` (không route ghi inventory) |
| F075-AC-17 | contract `no_real_money_test.go` (không route trao đổi), domain `TestContentValidation` (`tradable`) |
| F075-AC-18 | `economy_collection_test.go` `TestKnowledgeCardsAndCollection`; domain `collection_test.go` `TestProgressCompletesOnce` |
| F075-AC-19 | `TestKnowledgeCardsAndCollection`; domain `TestEarnsCard` |
| F075-AC-20 | `TestKnowledgeCardsAndCollection` (ẩn tới món đầu, 404 chi tiết ẩn, cách nhận vi/en) |
| F075-AC-21 | `economy_achievement_test.go` `TestAchievements`; domain `achievement_test.go` `TestCountTiersAndFilters` |
| F075-AC-22 | `TestAchievements` (thành tựu ẩn không lộ trước khi mở) |
| F075-AC-23 | `TestAchievementPublishChecks`; domain `TestValidateRefusesUnsafeAchievements` |
| F075-AC-24 | `TestAchievements` (version 2), domain `TestNewVersionNeverLowers` |
| F075-AC-25 | `economy_adjustment_test.go` `TestEconomyAdjustments`; domain `TestAdjustment` |
| F075-AC-26 | `TestEconomyAdjustments` (tự duyệt 403, âm số dư 409, đã xử lý 422, Xu/vật phẩm/XP); domain `TestAdjustment` |
| F075-AC-27 | `TestEconomyAdjustmentAuthz` |
