# Contracts: F094 — Art pass 4: sáu vùng còn lại theo "Chibi anime cartoon 3D" và các điểm mở của F093

> Feature chỉ đổi đồ họa phía client và thêm một màu nghiệp vụ dùng chung; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Bố cục vùng (spawn point, interactable, cổng) vẫn là seed thế giới của module `game` (`src/backend/modules/game/seed/world/areas.v1.json`, F071/F072); client chỉ hiển thị. Mã môn `history` đã có trong seed (`subject_code`).

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ: seed F071 ↔ khối neo sinh trong `Zone.tscn` (`gen_greybox_zones.mjs --check`, test điểm neo); token `art.palette.*` (slot) ↔ UV mesh ↔ uniform `palette[64]`; `ArtCatalog.ZONES` ↔ file `.res` (`bake_art --check`); `color.subject.*` (shared-tokens) ↔ `domainColors` của portal ↔ `Art3dTokens` (kiểm `domain-variants`).

## 6. Delta cần thực hiện

- Không có delta contract API. Delta token: `tokens@0.12.0` (`design/CHANGELOG.md`): `color.subject.history`, `art.palette.subject_history` (slot 60), `env.{history,ocean,mountain,exam,arena}`.
