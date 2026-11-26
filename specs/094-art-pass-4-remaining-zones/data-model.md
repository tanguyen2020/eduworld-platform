# Data Model: F094 — Art pass 4: sáu vùng còn lại theo "Chibi anime cartoon 3D" và các điểm mở của F093

> Feature chỉ có dữ liệu hiển thị phía client và một token màu nghiệp vụ dùng chung; không bảng, không event, không dữ liệu học sinh mới.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| Bố cục vùng | code (`zone_<vùng>_art.gd`) | `LAYOUT`: `plaza` [tâm, bán kính], `lanes` (hình chữ nhật [tâm, nửa cạnh]), `pads` (bệ tròn dưới điểm neo), `BLOCKED` (vùng cấm rải cây) | code |
| Prefab vùng chung | `tools/art/zone_kit.gd` | `lawn()`, `plaza()`, `pad()`, `path_strip()`, `boundary()`, `far()`, `house()`, `lamp()`, `flower_bed()`, `clear_of()`, `scatter_in()`, `pennant()`, `bunting()`, `balloons()`, `cairn()`, `rock()`, `pine()` | code |
| Công thức vùng | `tools/art/zone_<vùng>_art.gd` (`practice`, `social`, `ocean`, `mountain`, `exam`, `arena`) | `SLUG`, `THEME_OVERRIDES`, `meshes()`, `SCATTER`, `placements()` | code; `ArtCatalog.ZONES` |
| Mesh vùng | bake | `env_<vùng>_terrain_a`, `env_<vùng>_far_a`, `bld_<vùng>_set_a`, `bld_/prop_<vùng>_<set piece>_a`, `prop_<vùng>_set_a`, `veg_pine_a` | `assets/models/{env,bld,prop,veg}/` |
| `ScatterSet` vùng | bake | mesh cây + transform | `assets/models/env/env_<vùng>_scatter_<loại>_a.res` |
| Màu môn Lịch sử | `design/shared-tokens/domain.json` | `color.subject.history` (light, dark, game) | → `antd-theme.ts`, `tokens.css`, `Art3dTokens`, palette slot 60 `subject_history` |
| Environment | `design/game-tokens/art3d.json` `env.*` | `history`, `ocean`, `mountain`, `exam`, `arena` (sky, fog) | `resources/art/generated/env_<zone>.tres` |
| Scene vùng | tay + sinh | `Terrain/{Collision,Ground,Far}`, `Static/{Boundary*,…Collision,Buildings,Props,<set piece>}`, `Ambient/<scatter>`; khối neo giữa marker | `scenes/world/zones/<area>/Zone.tscn` |

## 2. Vòng đời / trạng thái

- Đổi hình: sửa công thức → `make godot-art-bake` → commit `.res` + ảnh (`capture_zone.gd`); `bake_art --check` và test bảo đảm khớp.
- Đổi điểm neo: sửa seed backend → `node src/game/tools/gen_greybox_zones.mjs` (chỉ khối neo được sinh lại) → kiểm set piece không chắn điểm neo/lối đi (`test_zone_art_pass_4.gd`).

## 3. Quy tắc kiểm tra dữ liệu

- Slot palette chỉ thêm cuối (60 mới; 61–63 trống; tối đa 64).
- Mọi điểm neo = seed (id, loại, vị trí); collision mới không chắn spawn, điểm neo, lối đi của bố cục.
- Mesh trong ngân sách PERFORMANCE_BUDGET §8; mesh có viền không có mặt hai mặt dày 0.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Không có event | — |
