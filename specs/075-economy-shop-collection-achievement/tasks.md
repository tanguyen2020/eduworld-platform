# Tasks: F075 — Kinh tế Xu, Shop cosmetic, inventory, collection, achievement, badge

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `internal/domain/economy`, `internal/domain/collection`, `internal/domain/achievement` + depguard — file: `modules/game/internal/domain/economy/doc.go`, `modules/game/.golangci.yml`
- [ ] T002 [P] Cấu hình `xu_daily_cap`, `duplicate_item_xu`, `shop_purchase_limit_default`, `badges_displayed_max` từ platform settings — file: `modules/game/internal/config/economy.go`
- [ ] T003 [P] Seed config: currency `xu`, item, shop item (một gán riêng `th-demo-a`), collection knowledge card, achievement (một ẩn) — file: `deploy/seed/game-config/items/`, `deploy/seed/game-config/shop-items/`, `deploy/seed/game-config/collections/`, `deploy/seed/game-config/achievements/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Sửa `shop-item.schema.json`, `reward-rule.schema.json` (grant `currency`/`collection_item`); thêm `item.schema.json`, `collection.schema.json`; OpenAPI `item_id` uuid → mã `maxLength: 64` (`InventoryItem`, `ShopItem`, `EconomyAdjustmentRequest`) — file: `specs/schemas/`, `specs/openapi/modules/game.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T005 [P] OpenAPI: `listInventory` query `kind`; `purchaseShopItem` `422`/`428`; `approveEconomyAdjustment` header + `422`; thêm `listEconomyAdjustments`, `rejectEconomyAdjustment` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [ ] T006 [P] Payload event `currency.credited/debited`, `shop.item_purchased`, `item.granted`, `collection.item_added/completed`, `achievement.progressed/unlocked`, `economy.adjusted` — file: `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [ ] T007 ent schema theo data-model (ledger AppendOnly, `inventory_items.quantity`, `shop_item_versions.event_id/season_id`, `economy_adjustments` Mới, `student_achievements.progress_state`) — file: `modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f075_economy`, `make migrate-lint MOD=game`
- [ ] T008 Migration: partition tháng ledger, CHECK `balance >= 0`, unique, RLS, `REVOKE UPDATE, DELETE, TRUNCATE` ledger — file: `modules/game/migrations/`
- [ ] T009 [P] Test RLS + append-only F075-AC-05 cho `wallets`, `currency_ledger`, `item_ledger`, `inventory_items`, `economy_adjustments` — file: `modules/game/tests/integration/economy_rls_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Ví Xu và sổ cái (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T010 [P] [US1] Unit test `economy.Wallet` (credit/debit, không âm, cap còn lại) cho F075-AC-01, F075-AC-02, F075-AC-03 — file: `modules/game/internal/domain/economy/wallet_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Integration test applier `currency` F075-AC-01, F075-AC-02 (cùng transaction RewardGrant, cap, lặp event) + job đối soát F075-AC-03 (cố ý lệch → cảnh báo, không sửa) — file: `modules/game/tests/integration/currency_applier_test.go`
- [ ] T012 [P] [US1] API test F075-AC-04: `getWallet`, `listWalletLedger` chỉ của mình, không id kỹ thuật — file: `modules/game/tests/integration/wallet_api_test.go`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Domain `economy.Wallet` — file: `modules/game/internal/domain/economy/wallet.go`
- [ ] T014 [US1] Applier `currency` (bucket cap `currency:xu`, giảm dần) đăng ký `GrantApplierRegistry` — file: `modules/game/internal/infrastructure/reward/currency.go`
- [ ] T015 [US1] Query `GetWallet`, `ListWalletLedger` + handler; job `wallet-reconcile` — file: `modules/game/internal/application/query/get_wallet.go`, `modules/game/internal/presentation/http/public/wallet.go`, `modules/game/internal/presentation/job/wallet_reconcile.go`

## Phase 4: User Story 2 — Shop cosmetic (Priority: P1)

### Tests cho User Story 2

- [ ] T016 [P] [US2] Unit test `economy.PurchasePolicy` cho F075-AC-10 (limit, sở hữu, thời gian bán, sự kiện/mùa không chạy, sai khối) — file: `modules/game/internal/domain/economy/purchase_test.go` — phải FAIL trước khi code
- [ ] T017 [P] [US2] Integration test F075-AC-06, F075-AC-07, F075-AC-08: lọc theo tenant/khối/thời gian; một transaction đủ bút toán + outbox; cùng key → cùng response; thiếu key 428; key khác body 422; audit — file: `modules/game/tests/integration/shop_purchase_test.go`
- [ ] T018 [P] [US2] Test race F075-AC-09: 1.000 cặp request đồng thời — file: `modules/game/tests/integration/shop_purchase_race_test.go`
- [ ] T019 [P] [US2] Contract/static test F075-AC-11: quét spec không giá tiền thật/random; module `shop-items`/`items` từ chối `currency_code ≠ xu`, giá ≤ 0, `tradable = true` (TC-ECO-003) — file: `modules/game/tests/contract/no_real_money_test.go`
- [ ] T020 [P] [US2] Test authorization + tenant isolation F075-AC-12: mọi route học sinh × token nhân sự → 403; header tenant khác → `TENANT_MISMATCH`; mặt hàng tenant khác → `SHOP_ITEM_UNAVAILABLE` — file: `modules/game/tests/integration/economy_player_authz_test.go`

### Implementation cho User Story 2

- [ ] T021 [P] [US2] Domain `economy.PurchasePolicy` — file: `modules/game/internal/domain/economy/purchase.go`
- [ ] T022 [US2] Command `PurchaseShopItem` (idempotency middleware `pkg/httpx`, khóa ví + `(student_id, item_id)`, outbox) + query `ListShopItems` + handler — file: `modules/game/internal/application/command/purchase_shop_item.go`, `modules/game/internal/presentation/http/public/shop.go`
- [ ] T023 [US2] Module config `items`, `currencies`, `shop-items` (validate schema + invariant) — file: `modules/game/internal/infrastructure/gameconfig/item_module.go`, `modules/game/internal/infrastructure/gameconfig/shop_item_module.go`

## Phase 5: User Story 3 — Inventory, vật phẩm trùng, trang bị (Priority: P1)

### Tests cho User Story 3

- [ ] T024 [P] [US3] Integration test F075-AC-13, F075-AC-14: grant item/badge, xếp chồng, grant trùng → quy đổi `duplicate_item_xu` — file: `modules/game/tests/integration/item_applier_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US3] API test F075-AC-15, F075-AC-16, F075-AC-17: lọc `kind`; `updateAvatar` không sở hữu/sai slot; không có route ghi inventory hay trao đổi (quét spec) — file: `modules/game/tests/integration/inventory_api_test.go`

### Implementation cho User Story 3

- [ ] T026 [US3] Applier `item`, `badge` + `DuplicatePolicy` — file: `modules/game/internal/infrastructure/reward/item.go`, `modules/game/internal/domain/economy/duplicate.go`
- [ ] T027 [US3] Query `ListInventory`, `ListInventoryHistory`, `InventoryOwnership` (dùng bởi `updateAvatar` của F070) + handler — file: `modules/game/internal/application/query/list_inventory.go`, `modules/game/internal/presentation/http/public/inventory.go`

## Phase 6: User Story 4 — Collection và knowledge card (Priority: P2)

### Tests cho User Story 4

- [ ] T028 [P] [US4] Integration test F075-AC-18, F075-AC-19: tiến độ collection, hoàn thành thưởng một lần; card cấp khi `proficient`, không thu hồi khi giảm — file: `modules/game/tests/integration/collection_test.go` — phải FAIL trước khi code
- [ ] T029 [P] [US4] API test F075-AC-20: collection ẩn tới món đầu; "cách nhận" song ngữ — file: `modules/game/tests/integration/collection_api_test.go`

### Implementation cho User Story 4

- [ ] T030 [US4] Domain `collection` + applier `collection_item` + consumer `game.collection_on_mastery` + module `collections` — file: `modules/game/internal/domain/collection/`, `modules/game/internal/presentation/consumer/collection_on_mastery.go`, `modules/game/internal/infrastructure/gameconfig/collection_module.go`
- [ ] T031 [US4] Query `ListCollections`, `GetCollection` + handler — file: `modules/game/internal/presentation/http/public/collections.go`

## Phase 7: User Story 5 — Achievement và badge (Priority: P2)

### Tests cho User Story 5

- [ ] T032 [P] [US5] Unit test `achievement.Evaluator` F075-AC-21, F075-AC-24 (filters, `count`, `distinct_days`, tier, version mới không giảm) — file: `modules/game/internal/domain/achievement/evaluator_test.go` — phải FAIL trước khi code
- [ ] T033 [P] [US5] Integration test F075-AC-21, F075-AC-22: event lặp mở khóa một lần; `listAchievements` ẩn điều kiện — file: `modules/game/tests/integration/achievement_test.go`
- [ ] T034 [P] [US5] Test module `achievements` F075-AC-23: trigger lạ, điều kiện thời gian chơi/đăng nhập/tin nhắn/điểm chính thức → 422 — file: `modules/game/tests/unit/achievement_module_test.go`

### Implementation cho User Story 5

- [ ] T035 [US5] Domain `achievement`, command `EvaluateAchievements`, consumer `game.achievement_evaluator`, module `achievements`, query `ListAchievements` + handler — file: `modules/game/internal/domain/achievement/`, `modules/game/internal/presentation/consumer/achievement_evaluator.go`, `modules/game/internal/presentation/http/public/achievements.go`

## Phase 8: User Story 6 — Bút toán bù có người duyệt thứ hai (Priority: P2)

### Tests cho User Story 6

- [ ] T036 [P] [US6] Integration test F075-AC-25, F075-AC-26: tạo pending, tự duyệt 403, duyệt ghi bút toán + event + audit, âm số dư 409, duyệt lại 409 — file: `modules/game/tests/integration/economy_adjustment_test.go` — phải FAIL trước khi code
- [ ] T037 [P] [US6] Test authorization F075-AC-27: role tenant/platform thiếu quyền, thiếu MFA, `azp` sai → 403 + `authz.denied`; `reason` > 1000 ký tự → 422 — file: `modules/game/tests/integration/economy_adjustment_authz_test.go`

### Implementation cho User Story 6

- [ ] T038 [US6] Domain `economy.Adjustment`, command `CreateEconomyAdjustment`/`ApproveEconomyAdjustment`/`RejectEconomyAdjustment`, query `ListEconomyAdjustments` + handler (`/admin/economy/*`) — file: `modules/game/internal/application/command/approve_economy_adjustment.go`, `modules/game/internal/presentation/http/public/economy_admin.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T039 [P] Cập nhật DATABASE_DESIGN §2.8a (`quantity`, `economy_adjustments`, `event_id`/`season_id`, `progress_state`, `name_en`), INVENTORY_ECONOMY §8–9, EVENT_DESIGN §4.6, CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/01-game-design/INVENTORY_ECONOMY.md`, `docs/07-backend/EVENT_DESIGN.md`, `CHANGELOG.md`
- [ ] T040 [P] Thêm tài khoản demo platform thứ hai có `game.economy.adjust` vào realm local và LOCAL_DEVELOPMENT — file: `deploy/keycloak/realm-eduworld.dev.json`, `docs/11-operations/LOCAL_DEVELOPMENT.md`
- [ ] T041 [P] Metric `eduworld_game_currency_*`, `eduworld_game_shop_purchase_total`, `eduworld_game_ledger_mismatch_total{ledger="currency"}` — file: `modules/game/internal/infrastructure/observability/metrics.go`
- [ ] T042 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2, US3 (song song sau T014) → US4, US5 → US6 → Polish.
- Phụ thuộc F074 (`GrantApplierRegistry`, cap ngày), F070 (`updateAvatar`), F031 (`learning_objective.progressed`).

## Definition of Done

- [ ] Mọi tiêu chí F075-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game`; `make test-contract` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
