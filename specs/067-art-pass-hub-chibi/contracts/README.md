# Contracts: F067 — Art pass 1: Main Hub low-poly stylized và nhân vật chibi (asset MIT tự làm)

> Feature chỉ đổi đồ họa phía client; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Cổng gió đọc `target_area_id` đã có của layout vùng (F071, `getWorldAreas`) chỉ để chọn mesh hiển thị.

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có. Ngoại hình người chơi khác vẫn theo `hub.player.appearance_changed` (F092) và `AvatarBuilder`.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ của client: token `art.palette.*` (slot) ↔ UV của mesh bake ↔ uniform `palette[64]` của `toon.gdshader` (data-model.md §1).

## 6. Delta cần thực hiện

- Không có delta contract. Delta token: `tokens@0.8.0` (`design/CHANGELOG.md`): `palette.art.*`, `art.palette.*`, `light.{sun_energy,ambient_energy,ambient_color,fog_density,exposure}`, `number.light.*`; preset `mat_toon*.tres`; Environment có ambient phẳng, tonemap, sương.
