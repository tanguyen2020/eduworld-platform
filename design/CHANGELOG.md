# Changelog — design tokens

Phiên bản token theo semver (`design/package.json`, DESIGN_SYSTEM §3, §8). Đổi tên hoặc xóa token cần tăng MAJOR; token bị xóa phải mang `$deprecated` ít nhất một phiên bản trước (kiểm bởi `design/checks/semver.mjs`).

## tokens@0.1.0

- 2026-11-19 — Phiên bản đầu (F005): `shared-tokens` (brand, môn học/khu vực, độ hiếm, trạng thái nội dung — mỗi màu có biến thể `light`/`dark`/`game`), `game-tokens` (primitive, semantic, component, art3d, density `g1_3`…`g8_9`, chế độ tương phản cao), `admin-tokens` (seed Ant Design, semantic, mode `light`/`dark`/`compact`); font Be Vietnam Pro + Baloo 2 (OFL). Giá trị khởi tạo [Proposed] theo DESIGN_SYSTEM §4.1, §5.2, §5.5, §6; `color.brand.primary` (light) đổi `#2F6BFF` → `#2A62F0` để chữ trắng đạt 4.5:1.
