# Data Model: F067 — Art pass 1: Main Hub low-poly stylized và nhân vật chibi (asset MIT tự làm)

> Feature chỉ có dữ liệu hiển thị phía client; không bảng, không event, không dữ liệu học sinh.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| Bảng màu mỹ thuật | token `art.palette.<tên>` (`design/game-tokens/art3d.json`, `$extensions.eduworld.index`) | tên, màu (alias `palette.art.*`, `color.subject.*`, `palette.avatar.*`), slot | sinh: `Art3dTokens.ART_PALETTE`, `ART_PALETTE_COLORS`; mảng `palette` của `mat_toon*.tres` |
| Preset toon | token `toon.*`, `outline.*`, bảng màu | `palette[64]`, `palette_size`, `albedo`, `toon_steps`, `shadow_tint`, `rim_strength`; `next_pass` viền: `outline_color`, `outline_width`, `smooth_normals_in_color` | `res://resources/art/generated/mat_toon.tres`, `mat_toon_no_outline.tres` |
| Ánh sáng zone | token `light.*`, `env.<zone>.*` | màu/năng lượng nắng, ambient phẳng, sương, exposure, bầu trời | `env_<zone>.tres`, `Art3dTokens.LIGHT_*` |
| Mesh bake | công thức `tools/art/*_art.gd` | VERTEX, NORMAL phẳng, COLOR = pháp tuyến mượt × 0,5 + 0,5, TEX_UV = (slot + 0,5, 0,5) hoặc (0,5, 1,5) = `albedo` | `res://assets/models/<category>/*.res` |
| `ScatterSet` | `HubArt.placements(kind)` (seed cố định) | `mesh`, `transforms: Array[Transform3D]` | `res://assets/models/env/env_hub_scatter_<kind>_a.res` |
| `ItemVisual.tint_token` | tên slot bảng màu | màu đồ/phụ kiện của avatar | `res://resources/items/*.tres` |
| Preset chất lượng (mở rộng) | `QualityPolicy.PRESETS` | `outline_distance` (12 / 30 / 0 m), `decor_density` (0,25 / 0,6 / 1,0) | code (CONFIG_DEFAULTS `quality_presets`) |
| Dòng license | `assets/LICENSES.md` | glob, nguồn, tác giả, license, ngày | repo |

## 2. Vòng đời / trạng thái

- Đổi màu: sửa token → `make design-tokens` → preset/Environment sinh lại; mesh không đổi (slot cố định).
- Đổi hình: sửa công thức → `bake_art.gd` → commit `.res` + ảnh trước/sau; test `test_committed_art_matches_the_recipes` bảo đảm khớp.
- Thêm asset ngoài (CC0/MIT, owner tải): thêm dòng `LICENSES.md` trước khi commit file.

## 3. Quy tắc kiểm tra dữ liệu

- Slot chỉ thêm cuối, không đánh số lại (`artPaletteSlots` báo lỗi khi thiếu/trùng chỉ số; tối đa 64).
- `AreaPortal.gate_mesh` chỉ nhận id khớp `^[a-z0-9_.]{1,64}$`; id lạ → cổng mặc định.
- License: chỉ MIT, CC0-1.0, Public-Domain; OFL-1.1 cho font (ADR-0102).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Không có event | — |
