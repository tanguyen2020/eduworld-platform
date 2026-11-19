# Changelog — design tokens

Phiên bản token theo semver (`design/package.json`, DESIGN_SYSTEM §3, §8). Đổi tên hoặc xóa token cần tăng MAJOR; token bị xóa phải mang `$deprecated` ít nhất một phiên bản trước (kiểm bởi `design/checks/semver.mjs`).

## tokens@0.3.0

- 2026-11-19 — Thêm seed Ant Design `antd.colorTextPlaceholder` (alias `ops.color.text.secondary`): mặc định `#bfbfbf` chỉ đạt 1,83:1, axe chặn placeholder "Tất cả tenant" của bộ lọc tenant (F051-AC-18, F051-AC-22). MINOR vì chỉ thêm token.

## tokens@0.2.0

- 2026-11-19 — Thêm seed Ant Design `antd.colorText`, `colorTextSecondary`, `colorTextTertiary`, `colorTextDescription` (alias `ops.color.text.*`): giá trị mặc định của Ant Design (độ mờ 0,45) chỉ đạt 3,36:1, bị axe chặn trong Storybook (F050-AC-19). MINOR vì chỉ thêm token.

## tokens@0.1.0

- 2026-11-19 — Phiên bản đầu (F005): `shared-tokens` (brand, môn học/khu vực, độ hiếm, trạng thái nội dung — mỗi màu có biến thể `light`/`dark`/`game`), `game-tokens` (primitive, semantic, component, art3d, density `g1_3`…`g8_9`, chế độ tương phản cao), `admin-tokens` (seed Ant Design, semantic, mode `light`/`dark`/`compact`); font Be Vietnam Pro + Baloo 2 (OFL). Giá trị khởi tạo [Proposed] theo DESIGN_SYSTEM §4.1, §5.2, §5.5, §6; `color.brand.primary` (light) đổi `#2F6BFF` → `#2A62F0` để chữ trắng đạt 4.5:1.
