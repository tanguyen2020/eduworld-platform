# Implementation Plan: F075 — Kinh tế Xu, Shop cosmetic, inventory, collection, achievement, badge

**Branch**: `075-economy-shop-collection-achievement` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm bounded context `economy` (ví, sổ cái Xu/vật phẩm, shop, điều chỉnh) và `collection`/`achievement` vào module game, schema PostgreSQL `gamification` (+ `game.item_definitions`, `game.inventory_items`). Đăng ký applier `currency`, `item`, `badge`, `collection_item` vào `GrantApplierRegistry` của F074 nên mọi phần thưởng (rule, level, achievement, collection, sự kiện) đi chung một transaction sổ cái. `PurchaseShopItem` là command có `Idempotency-Key` (`src/backend/pkg/idempotency`), khóa `wallets` `FOR UPDATE` theo `(student_id, 'xu')` và `(student_id, item_id)`. Consumer `game.achievement_evaluator` và `game.collection_on_mastery` xử lý event. `CreateEconomyAdjustment`/`ApproveEconomyAdjustment` qua gateway nội bộ, four-eyes. Job `wallet-reconcile` hằng đêm.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/idempotency`, `src/backend/pkg/outbox`, `src/backend/pkg/authz`, `src/backend/pkg/tenancy`, jsonschema validator |
| Storage | PostgreSQL `eduworld_game`: `gamification.currencies` G, `wallets` T, `currency_ledger` TA P, `item_ledger` TA P, `shop_items`/`shop_item_versions` G, `shop_purchases` T, `collections`/`collection_versions`/`collection_items` G, `collection_progress` T, `achievements`/`achievement_versions` G, `student_achievements` T, `economy_adjustments` T (Mới); `game.item_definitions`/`item_definition_versions` G, `game.inventory_items` T; `infra.tenant_resource_assignments`, `infra.idempotency_records` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); test race mua đồng thời |
| Target Platform | Linux container (`core-api` + `worker`, module `game`) |
| Project Type | backend module |
| Performance Goals | `purchaseShopItem` p95 ≤ 300 ms (SC-004) |
| Constraints | AGENTS R5, R8, R10, R19, R20, R21, R26, R27, R28; ADR-0027, ADR-0029, ADR-0043, ADR-0049 |
| Scale/Scope | Ledger partition tháng (DATABASE_DESIGN §2.8a) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta schema item/collection, sửa `shop-item`/`reward-rule`, endpoint điều chỉnh, payload event ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ trong module game; mastery nhận qua event; không facade mới |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng T/TA + RLS; ledger chỉ INSERT; test isolation mọi endpoint |
| IV Server-authoritative | ✅ | Giá/số dư/cấp thưởng ở server; `Idempotency-Key` + audit cho mua và điều chỉnh |
| V Xác thực, phân quyền | ✅ | `game_player` cho học sinh; `game.economy.adjust` + role platform + `eduworld-portal` cho điều chỉnh; config qua F078 (`game.item.write`, `game.economy.write`, `game.shop.write`, `game.collection.write`, `game.achievement.write`) |
| VI An toàn trẻ em | ✅ | Không tiền thật/loot box/trading; achievement không dựa thời gian chơi (R19) |
| VII Một nguồn sự thật, cấu hình | ✅ | `xu_daily_cap`, `duplicate_item_xu` từ cấu hình |
| VIII UI tách logic, token | ✅ | Không có UI trong feature (màu độ hiếm là shared token ở client) |
| IX Test có bằng chứng | ✅ | Test mỗi F075-AC-*, race, RLS, authorization |

## Project Structure

### Documentation (feature này)

```text
specs/075-economy-shop-collection-achievement/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/        # currency.go, wallet.go, currencyledger.go, itemledger.go, itemdefinition.go, itemdefinitionversion.go,
│                      # inventoryitem.go, shopitem.go, shopitemversion.go, shoppurchase.go, collection*.go,
│                      # achievement.go, achievementversion.go, studentachievement.go, economyadjustment.go
├── internal/domain/economy/       # Wallet (Credit/Debit, không âm), Purchase (limit, availability), DuplicatePolicy, Adjustment (four-eyes)
├── internal/domain/collection/    # Collection, Progress, KnowledgeCardRule
├── internal/domain/achievement/   # Definition, Evaluator (filters, metric, tiers), VisibilityPolicy
├── internal/application/command/  # purchase_shop_item.go, create_economy_adjustment.go, approve_economy_adjustment.go,
│                                  # evaluate_achievements.go, grant_knowledge_card.go, reconcile_wallets.go
├── internal/application/query/    # get_wallet.go, list_wallet_ledger.go, list_shop_items.go, list_inventory.go,
│                                  # list_inventory_history.go, list_collections.go, get_collection.go, list_achievements.go,
│                                  # list_economy_adjustments.go, inventory_ownership.go
├── internal/infrastructure/reward/        # appliers: currency.go, item.go, badge.go, collection_item.go
├── internal/infrastructure/persistence/   # wallet_repo.go, inventory_repo.go, shop_repo.go, achievement_repo.go
├── internal/infrastructure/gameconfig/    # item_module.go, currency_module.go, shop_item_module.go, collection_module.go, achievement_module.go
├── internal/presentation/http/public/     # wallet.go, shop.go, inventory.go, collections.go, achievements.go, economy_admin.go
├── internal/presentation/consumer/        # achievement_evaluator.go, collection_on_mastery.go
├── internal/presentation/job/             # wallet_reconcile.go
└── tests/{unit,integration,contract}/
```

**Structure Decision**: Applier nằm ở infrastructure (ghi ent trong transaction của F074) để Reward Engine giữ một transaction cho mọi loại grant; domain economy thuần để test invariant không âm/four-eyes.

## Phase 0: Research

Không có — INVENTORY_ECONOMY, ADR-0049, DATABASE_DESIGN §2.8a đủ. Kiểu `item_id` đã chốt mã `varchar(64)` (spec Clarifications Session 2026-11-17, RB-308).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `economy.Wallet`, `economy.PurchasePolicy` (giá, limit, sở hữu, availability), `economy.DuplicatePolicy`, `economy.Adjustment` (creator ≠ approver), `achievement.Evaluator`, `collection.Progress` | Thuần; Clock inject |
| application | Command `PurchaseShopItem` (permission: học sinh self), `CreateEconomyAdjustment`/`ApproveEconomyAdjustment`/`RejectEconomyAdjustment` (`game.economy.adjust`), `EvaluateAchievements`, `GrantKnowledgeCard`, `ReconcileWallets` · Query như cây trên | idempotency middleware `src/backend/pkg/httpx` cho command HTTP; authz `authz.Require` tường minh đầu handler |
| infrastructure | ent repo, applier theo loại grant, module config | |
| presentation | `getWallet`, `listWalletLedger`, `listShopItems`, `purchaseShopItem`, `listInventory`, `listInventoryHistory`, `listCollections`, `getCollection`, `listAchievements`, `createEconomyAdjustment`, `approveEconomyAdjustment`, `listEconomyAdjustments` (Cần thêm), `rejectEconomyAdjustment` (Cần thêm) · consumer · job | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F074 (`GrantApplierRegistry`, cap ngày, `reward_grants`), F070 (`updateAvatar`, slot), F071 (nguồn vật sưu tầm), F031 (`learning_objective.progressed`).
- Event tiêu thụ: trigger achievement (`quest.completed`, `learning_objective.mastered`, `challenge.completed`, `world.*`, `level.reached`, `collection.completed`, `shop.item_purchased`, `party.activity_completed`, `match.completed`…), `learning_objective.progressed`, `content.published` (item_type `shop_item`, `achievement`, `collection`).
- Event phát: `currency.credited`, `currency.debited`, `shop.item_purchased`, `item.granted`, `collection.item_added`, `collection.completed`, `achievement.progressed`, `achievement.unlocked`, `economy.adjusted`, `audit.recorded`.

### Bảo mật, tenant, idempotency, audit

- Học sinh: `game_player`, `TenantScope` từ Principal, chỉ `student_id = principal.user_id`; shop lọc `infra.tenant_resource_assignments` (`resource_type = shop_item`) hoặc mặt hàng gán mọi tenant.
- Điều chỉnh: route `/admin/economy/*` chỉ gateway nội bộ; `azp = eduworld-portal`, role platform, permission `game.economy.adjust`; ghi theo `X-Tenant-Id` tenant đích.
- `Idempotency-Key` bắt buộc: `purchaseShopItem`, `createEconomyAdjustment`, `approveEconomyAdjustment`. Audit: `shop.item_purchased`, `economy_adjustment.created/approved/rejected`, `authz.denied`.

### Quan sát

- Metric `eduworld_game_currency_delta_total{reason_code}`, `eduworld_game_shop_purchase_total{result}`, `eduworld_game_ledger_mismatch_total{ledger="currency"}`, `eduworld_game_achievement_unlocked_total`, `eduworld_game_economy_adjustment_total{status}`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
