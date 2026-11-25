# Contracts: F093 — Art pass 3: năm vùng môn học theo "Chibi anime cartoon 3D"

> Feature chỉ đổi đồ họa phía client; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Bố cục vùng (spawn point, interactable, cổng) vẫn là seed thế giới của module `game` (`src/backend/modules/game/seed/world/areas.v1.json`, F071); client chỉ hiển thị.

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ của client: seed F071 ↔ khối neo sinh trong `Zone.tscn` (`gen_greybox_zones.mjs`, `test_anchors_equal_the_world_seed_layout`); token `art.palette.*` (slot) ↔ UV mesh ↔ uniform `palette[64]`; `ArtCatalog.ZONES` ↔ file `.res` (`bake_art --check`).

## 6. Delta cần thực hiện

- Không có delta contract. Delta token: `tokens@0.11.0` (`design/CHANGELOG.md`).
