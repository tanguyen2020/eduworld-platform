# Data Model: F093 — Art pass 3: năm vùng môn học theo "Chibi anime cartoon 3D"

> Feature chỉ có dữ liệu hiển thị phía client; không bảng, không event, không dữ liệu học sinh mới.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| Prefab vùng chung | code (`tools/art/zone_kit.gd`) | `SPAWN`, `NPC`, `EAST`, `WEST`, `NORTH`, `PORTAL` (= seed F071), `PLAZA_CENTER/RADIUS`, `WALK` 23,5 m, `BACKDROP` (0, 0, −31), `THEME` (vai trò → slot palette) | code |
| Công thức vùng | `tools/art/zone_<môn>_art.gd` | `SLUG`, `THEME_OVERRIDES`, tâm set piece, `BLOCKED` (vùng cấm rải cây), `meshes()` (đường dẫn → Callable), `SCATTER` (loại → mesh cây), `placements(loại)` | code; đăng ký `ArtCatalog.ZONES` |
| Mesh vùng | bake | `env_<môn>_terrain_a` (gần, có viền), `env_<môn>_far_a` (xa, không viền/bóng), `bld_<môn>_set_a` (landmark + nhà), `bld_/prop_<môn>_<set piece>_a`, `prop_<môn>_set_a` (đồ nhỏ), `veg_tree_{autumn,blossom}_a`, `veg_palm_coconut_a` | `assets/models/{env,bld,prop,veg}/` |
| `ScatterSet` vùng | bake | mesh cây + transform | `assets/models/env/env_<môn>_scatter_<loại>_a.res` |
| Cổng gió về Thị trấn | `hub_art.wind_gate("main_hub")` | biểu tượng ngôi nhà, màu `roof` | `assets/models/prop/prop_wind_gate_main_hub.res` |
| Slot palette mới | token `art.palette.*` | `tile_blue` 52, `tile_blue_dark` 53, `leaf_autumn` 54, `bronze` 55, `bronze_patina` 56, `metal` 57, `metal_dark` 58, `blossom_violet` 59 | `design/game-tokens/art3d.json` → `Art3dTokens`, preset `mat_toon*` |
| Scene vùng | tay + sinh | `Terrain/{Collision,Ground,Far}`, `Static/{Boundary*,…Collision,Buildings,Props,<set piece>}`, `Ambient/<scatter>`; khối neo giữa marker `BEGIN/END F071 generated anchors` | `scenes/world/zones/learning_zone_<môn>/Zone.tscn` |

## 2. Vòng đời / trạng thái

- Đổi hình: sửa công thức → `make godot-art-bake` → commit `.res` + ảnh (`capture_zone.gd`); `bake_art --check` và `test_committed_art_matches_the_recipes` bảo đảm khớp.
- Đổi bố cục điểm neo: sửa seed backend → `node src/game/tools/gen_greybox_zones.mjs` (chỉ khối neo của scene đã có art được sinh lại) → kiểm set piece không chắn điểm neo (`test_subject_zone_art.gd`).

## 3. Quy tắc kiểm tra dữ liệu

- Slot bảng màu chỉ thêm cuối (52–59 mới, 60/64 đã dùng).
- Mọi điểm neo của vùng = seed (id, loại, vị trí); collision mới không chắn spawn, điểm neo, lối chính, lối ngang.
- Mesh trong ngân sách PERFORMANCE_BUDGET §8.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Không có event | — |
