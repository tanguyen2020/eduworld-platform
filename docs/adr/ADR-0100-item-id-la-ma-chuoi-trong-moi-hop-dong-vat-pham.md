# ADR-0100 — `item_id` là mã chuỗi trong mọi hợp đồng vật phẩm

> **Trạng thái:** Accepted · **Ngày:** 2026-11-23 · **OQ:** — · **Tài liệu:** [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) · [API_DESIGN](../07-backend/API_DESIGN.md) · [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Vật phẩm game được định danh bằng mã ổn định người đọc được (`hair_ponytail_01`): `game.item_definitions.item_id varchar(64)` (F070), `avatar_config`, `shop-item.schema.json`, `reward-rule.schema.json` và INVENTORY_ECONOMY §2.2 đều dùng mã. OpenAPI lại khai báo `item_id` của `InventoryItem`, `ShopItem`, `ItemLedgerEntry`, `CollectionDetail.items`, `RewardGrantSummary` và `EconomyAdjustmentRequest` là `Uuid`. Backlog đối chiếu RB-308 và Clarifications Session 2026-11-17 của F075 đã chốt dùng mã, nhưng chưa có ADR nên `make specs-breaking` (chỉ nhận thay đổi phá vỡ có ADR, ADR-0095 mục 7) chặn việc đổi kiểu. Các route liên quan chưa được hiện thực trước F075 (inventory, shop, collection, điều chỉnh kinh tế) hoặc chỉ trả phần thưởng chưa có vật phẩm (`RewardGrantSummary` của F074), chưa client nào gửi hay đọc `item_id` dạng UUID.

## Quyết định

Mọi `item_id` vật phẩm trong OpenAPI (spec gộp và `game.v1.yaml`) dùng `CatalogCode` (`string`, 1–64 ký tự, pattern `^[a-z0-9_]+$`), cùng kiểu với `avatar_config` và cột DB; JSON Schema nội dung (`item`, `shop-item`, `collection`, `reward-rule`, `achievement`) và payload event (`item.granted`, `shop.item_purchased`, `collection.*`, `currency.*`, `economy.adjusted`) dùng cùng pattern. `badge_id` và `collection_item_id` của phần thưởng cũng là mã vật phẩm (huy hiệu là vật phẩm `kind = badge`). Không giữ alias UUID. Mã ổn định suốt vòng đời vật phẩm; đổi nội dung bằng version mới (ADR-0043).

## Hệ quả

(+) Một kiểu định danh vật phẩm từ DB tới client; không cần bảng ánh xạ UUID ↔ mã. (+) F070 `updateAvatar` và F075 inventory/shop dùng chung mã.
(−) oasdiff báo `response-property-type-changed` (format `uuid` bị bỏ) cho mọi operation trả `RewardGrantSummary`, `InventoryItem`, `ShopItem`, `ItemLedgerEntry`, `CollectionDetail`, `EconomyAdjustment` và `request-property-*` cho `createEconomyAdjustment`: chấp nhận qua `specs/breaking-allowlist.yaml` dẫn ADR này.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-23 | Tạo ADR (F075 T004, RB-308) | F075 |
