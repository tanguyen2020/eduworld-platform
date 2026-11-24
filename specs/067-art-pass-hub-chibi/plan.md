# Implementation Plan: F067 — Art pass 1: Main Hub low-poly stylized và nhân vật chibi (asset MIT tự làm)

**Branch**: `067-art-pass-hub-chibi` · **Date**: 2026-11-24 · **Spec**: [spec.md](spec.md)

## Summary

Toàn bộ đồ họa là asset **tự làm bằng code** (ADR-0102, MIT): công thức low-poly trong `src/game/tools/art/` (`lowpoly_mesh.gd` builder, `hub_art.gd`, `character_art.gd`) bake ra mesh `res://assets/models/<category>/*.res` bằng `tools/art/bake_art.gd` (`--check` so lại công thức). Mỗi mặt chỉ lưu **slot** của bảng màu token `art.palette.*` trong UV; shader toon chung (`resources/art/shaders/toon.gdshader`) đọc mảng màu do preset sinh từ token (`resources/art/generated/mat_toon*.tres`), viền inverted hull là `next_pass` (`toon_outline.gdshader`, pháp tuyến mượt trong COLOR). Scene Hub giữ nguyên cây node, spawn, interactable, collision; thay mesh greybox bằng mesh bake + `ScatterInstance` (MultiMesh) cho cây/cỏ/lúa. `ZoneArtQuality` áp preset (khoảng cách viền, mật độ trang trí, tầm nhìn); Compatibility tắt bóng nắng thời gian thực để giữ renderer parity. Nhân vật: thân chibi bake (mặt trong mesh), tóc/đồ/phụ kiện bake, `AvatarBuilder` đặt `albedo` theo avatar từ token. Lint `lint_art.gd` kiểm license (`assets/LICENSES.md`) và literal màu.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2, `.godot-version`), static typing; Godot shading language; Node 20 (design token pipeline) |
| Primary Dependencies | GdUnit4; pipeline token F005 (`design/`, Style Dictionary 4) |
| Storage | Không có dữ liệu nghiệp vụ; mesh `.res` nén trong repo (~0,6 MB) |
| Testing | GdUnit4 headless (unit + scene), benchmark `MainHubBenchmark.tscn` có renderer, `tools/capture_hub.gd` chụp ảnh, export Web tạm |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | `scene_budget_hub`, `character_asset_budget` (PERFORMANCE_BUDGET §7, §8) |
| Constraints | AGENTS R1, R2, R17, R32 · ADR-0047, ADR-0057, ADR-0101, ADR-0102 |
| Scale/Scope | 1 zone (Main Hub) + nhân vật/NPC/cổng gió/marker dùng chung mọi zone |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec từ quyết định owner + ADR-0102; không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client + design token; không backend, không contract |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu tenant/học sinh mới |
| IV Server-authoritative | ✅ | Không đổi luật chơi; cổng gió chỉ chọn mesh theo `target_area_id` (id hợp lệ) |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Không vũ khí/bạo lực (zone lint F071 giữ); biểu tượng không chỉ dùng màu |
| VII Một nguồn sự thật, cấu hình | ✅ | Màu/ánh sáng từ token; preset trong `QualityPolicy` + CONFIG_DEFAULTS `quality_presets`; license ở ADR-0102 + `LICENSES.md` |
| VIII UI tách logic, token | ✅ | Script mới chỉ hiển thị (`ZoneArtQuality`, `ScatterInstance`, chọn mesh); file sinh không sửa tay |
| IX Test có bằng chứng | ✅ | Mỗi F067-AC có test/công cụ; ảnh trước/sau ở `screenshots/` |

## Project Structure

```text
design/game-tokens/{primitive,art3d}.json        # palette.art.*, art.palette.* (slot), light.*, number.light.*
design/sd/formats/{godot-tres,gdscript-tokens}.mjs, design/lib/pipeline.mjs   # mat_toon*.tres, Environment, Art3dTokens
src/game/resources/art/shaders/toon.gdshader, toon_outline.gdshader           # MIT, viết tay
src/game/resources/art/generated/mat_toon.tres, mat_toon_no_outline.tres, env_*.tres, art3d_tokens.gd  # sinh
src/game/assets/LICENSES.md                                                   # ADR-0102
src/game/assets/models/{env,bld,prop,veg,chr}/*.res                           # mesh bake (MIT)
src/game/tools/art/{lowpoly_mesh,hub_art,character_art,art_catalog,bake_art}.gd
src/game/tools/{lint_art.gd,lib/art_rules.gd,capture_hub.gd}
src/game/scripts/world/{zone_art_quality,scatter_set,scatter_instance}.gd     # mới
src/game/scripts/world/{zone_root,area_portal,npc_anchor}.gd, scripts/core/quality_policy.gd, scripts/player/{avatar_builder,item_visual}.gd  # sửa
src/game/scenes/world/{zones/main_hub/Zone.tscn,AreaPortal.tscn,WorldAnchor.tscn}, scenes/npc/NPC.tscn, scenes/character/AvatarRig.tscn
src/game/resources/items/{hair,outfit,acc}_*.tres                             # mesh bake + tint_token
src/game/tests/unit/art/*, tests/unit/world/test_zone_art_quality.gd, tests/scene/world/test_hub_art.gd, tests/perf/main_hub_benchmark.gd
```

## Phase 0: Research

- Toon trong Godot 4: `light()` tùy biến chạy được ở Mobile và Compatibility; `source_color` trên texture sinh (GradientTexture1D) không được giải mã giống nhau giữa hai renderer → bảng màu là **mảng uniform** `vec4 palette[64] : source_color` (chuyển sRGB trên CPU như mọi uniform).
- Compatibility: DirectionalLight3D có bóng được vẽ thêm một pass cộng; shader `light()` tùy biến nhận nắng hai lần → tắt bóng nắng trên Compatibility (đo bằng ảnh `hub_web_*` so `hub_after_*`).
- MultiMesh lưu transform trong RenderingServer: bake headless không lưu được → `ScatterSet` (mesh + `Array[Transform3D]`) dựng MultiMesh lúc chạy.
- Viền inverted hull nứt ở cạnh phẳng → lưu pháp tuyến mượt (hàn theo vị trí) trong COLOR.

## Phase 1: Design & Contracts

- Không có contract API/event mới (contracts/README.md).
- Model hiển thị: data-model.md.
- Quy trình duyệt: quickstart.md (bake, lint, benchmark, chụp ảnh, export Web).
