# Contracts: F075 — Kinh tế Xu, Shop cosmetic, inventory, collection, achievement, badge

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/game.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getWallet` | `GET /api/v1/game/wallet` → `Wallet` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `listWalletLedger` | `GET /api/v1/game/wallet/ledger` → `CurrencyLedgerEntry[]` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `listShopItems` | `GET /api/v1/game/shop/items?kind=` → `ShopItem[]` | game.v1.yaml | `game_player` | Có sẵn |
| `purchaseShopItem` | `POST /api/v1/game/shop/items/{shop_item_id}:purchase` (`Idempotency-Key`) → `PurchaseResult` | game.v1.yaml | `game_player` | Cần sửa (khai báo `422`, `428`) |
| `listInventory` | `GET /api/v1/game/inventory` → `InventoryItem[]` | game.v1.yaml | `game_player` (self) | Cần sửa (thêm query `kind`) |
| `listInventoryHistory` | `GET /api/v1/game/inventory/history` → `ItemLedgerEntry[]` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `updateAvatar` | `PUT /api/v1/game/profile/avatar` (`ITEM_NOT_OWNED`, `EQUIP_SLOT_INVALID`) | game.v1.yaml | `game_player` | Có sẵn (handler F070; F075 cấp kiểm tra sở hữu) |
| `listCollections` | `GET /api/v1/game/collections` → `CollectionProgress[]` | game.v1.yaml | `game_player` | Có sẵn |
| `getCollection` | `GET /api/v1/game/collections/{collection_id}` → `CollectionDetail` | game.v1.yaml | `game_player` | Có sẵn |
| `listAchievements` | `GET /api/v1/game/achievements?category=` → `AchievementProgress[]` | game.v1.yaml | `game_player` | Có sẵn |
| `createEconomyAdjustment` | `POST /api/v1/admin/economy/adjustments` (`X-Tenant-Id` đích, `Idempotency-Key`) → `202 EconomyAdjustment` | game.v1.yaml | `game.economy.adjust` (MFA, gateway nội bộ) | Có sẵn |
| `approveEconomyAdjustment` | `POST /api/v1/admin/economy/adjustments/{adjustment_id}:approve` | game.v1.yaml | `game.economy.adjust` (≠ người tạo) | Cần sửa (thêm `IdempotencyKey`, `TenantIdHeader`, `422`) |
| `listEconomyAdjustments` | `GET /api/v1/admin/economy/adjustments?status=&tenant_ids=` | game.v1.yaml | `game.economy.adjust` | Cần thêm |
| `rejectEconomyAdjustment` | `POST /api/v1/admin/economy/adjustments/{adjustment_id}:reject` (`ReasonRequest`) | game.v1.yaml | `game.economy.adjust` | Cần thêm |
| `/game-admin/{config_type}` (`items`, `currencies`, `shop-items`, `collections`, `achievements`) | workflow ở F078 | game.v1.yaml | `game.item.write` (`items`), `game.economy.write` (`currencies`), `game.shop.write` (`shop-items`), `game.collection.write` (`collections`), `game.achievement.write` | Có sẵn — F075 đăng ký module |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không gọi đồng bộ module khác | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| Thông báo trong game kind `reward`, `achievement` (`RewardList` kind `currency`/`item`/`badge`) | server → học sinh | Có sẵn (`specs/asyncapi/eduworld-realtime.v1.yaml`; giao bởi F123) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `currency.credited`, `currency.debited` | game → RT, AUD, ANA | `specs/events/payloads/game/currency.credited.v1.schema.json`, `currency.debited.v1.schema.json` | Cần thêm |
| `shop.item_purchased` | game → RT, achievement, AUD | `shop.item_purchased.v1.schema.json` | Cần thêm |
| `item.granted` | game → RT, AUD | `item.granted.v1.schema.json` | Cần thêm |
| `collection.item_added`, `collection.completed` | game → RT, reward, achievement | `collection.item_added.v1.schema.json`, `collection.completed.v1.schema.json` | Cần thêm |
| `achievement.progressed`, `achievement.unlocked` | game → RT, NTF, reward | `achievement.progressed.v1.schema.json`, `achievement.unlocked.v1.schema.json` | Cần thêm |
| `economy.adjusted` | game → AUD | `economy.adjusted.v1.schema.json` | Cần thêm |
| `learning_objective.progressed` | learning → game (`game.collection_on_mastery`) | `specs/events/payloads/learning/learning_objective.progressed.v1.schema.json` | Cần thêm (owner F031) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/shop-item.schema.json` | Version `shop-items` | Cần sửa (`event_id`/`season_id`) |
| `specs/schemas/achievement.schema.json` | Version `achievements` | Có sẵn (F077 thêm category `event`) |
| `specs/schemas/reward-rule.schema.json` | Grant `currency`, `collection_item` | Cần sửa |
| `specs/schemas/item.schema.json` | Version `items` | Cần thêm |
| `specs/schemas/collection.schema.json` | Version `collections` | Cần thêm |

## 6. Delta cần thực hiện

- `item_id` ở mọi schema/contract là mã `type: string, maxLength: 64` (pattern mã) — OpenAPI `InventoryItem`, `ShopItem`, `EconomyAdjustmentRequest` và path/query tham chiếu item đổi từ uuid sang mã (RB-308); `shop-item.schema.json`, `reward-rule.schema.json` giữ chuỗi 64.
- `shop-item.schema.json`: thêm `event_id`, `season_id` (uuid, tùy chọn).
- `reward-rule.schema.json`: `grants[].type` thêm `currency` (`currency_code` const `xu`, `amount`), `collection_item`; `badge_id` → uuid.
- Thêm `item.schema.json` (`kind` theo `ItemKind`, `rarity`, `equip_slot`, `stackable`, `max_stack`, `tradable` const false, `name`/`description` song ngữ `maxLength`, `asset_refs`, `sources[]`) và `collection.schema.json` (`collection_type`, `visibility`, `items[]` với `source_hint` song ngữ, `objective_id`, `completion_rewards`).
- OpenAPI: `listInventory` thêm query `kind` (`ItemKind`); `purchaseShopItem` khai báo `422` (`IDEMPOTENCY_KEY_REUSED`, `VALIDATION_FAILED`), `428`; `approveEconomyAdjustment` thêm `IdempotencyKey`, `TenantIdHeader`, `422`; thêm `listEconomyAdjustments`, `rejectEconomyAdjustment` (tag `admin`, `x-internal: true`, `keycloakAdminBearer`).
- Payload event mới ở `specs/events/payloads/game/` (bảng §4).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make test-contract`.
