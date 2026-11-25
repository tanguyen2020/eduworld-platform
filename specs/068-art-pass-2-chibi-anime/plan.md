# Implementation Plan: F068 — Art pass 2: Chibi anime cartoon 3D cho Main Hub và nhân vật

**Branch**: `068-art-pass-2-chibi-anime` · **Date**: 2026-11-25 · **Spec**: [spec.md](spec.md)

## Summary

Giữ pipeline F067 (công thức trong `src/game/tools/art/` → bake `.res`, màu là slot token `art.palette.*`, `lint_art`, `bake_art --check`, benchmark) và đổi hướng hình theo [ADR-0103](../../docs/adr/ADR-0103-art-direction-chibi-anime-cartoon-3d-thay-stylized-low-poly.md):

- **Builder** `lowpoly_mesh.gd`: pháp tuyến mượt trong góc gấp 62° (trọng số diện tích → mặt lớn vẫn phẳng, cạnh vát thành bo tròn), hộp vát cạnh (`bevel_for`), trụ có nắp bo vành, tối thiểu 8 cạnh, thêm một bậc chia cho khối cầu; `set_flat()` cho trang trí dày; skinning (`skinned`, `bone`, `skin_fn` → BONES/WEIGHTS).
- **Token + shader**: `toon.steps` 2, `toon.band_softness` 0,06 (cạnh dải cel hẹp), `outline.color` mực tím ấm; preset mới `mat_toon_skinned.tres` (viền theo NORMAL đã skin); `light.blob_shadow_*`; slot `eye_iris`, `eye_white`, `ash`, `ash_dark`; `tokens@0.10.0`.
- **Nhân vật**: `KidSkeleton` (8 xương) dùng chung cho công thức, clip và `AvatarRig.tscn`; thân chibi (đầu 0,58 m / 1,2 m) và trang phục skinned; mặt theo biểu cảm (decal chiếu lên ellipsoid đầu, chia nhỏ tam giác); tóc lọn khối (`_lock`); clip keyframe `character_anim.gd` bake ra `anim_kid_*.res`; `AvatarBuilder.set_expression()` + chớp mắt; `AnimDriver` nạp clip rig, chọn biểu cảm theo trạng thái/emote; emote `.tres` trỏ clip.
- **Hub**: cùng công thức F067 với builder mới; vật thể lặp/xa/NPC giữ bậc chia gốc, khối < 0,15 m không vát, mặt đất và bụi không đổ bóng (ngân sách); Cây Tia Sáng 5 cấp (`spark_tree(stage)`), `SparkTree` + `SparkTreeModel`.
- **Blob shadow**: `BlobShadow` (+ `blob_shadow.gdshader`) trong `AvatarRig.tscn`, `NPC.tscn`; `QualityPolicy.blob_shadow_visible(renderer, sun_shadows)`; `ZoneArtQuality` cập nhật; người chơi bám mặt đất.
- **Công cụ**: `capture_avatars.gd` (mới), `capture_hub.gd` (giữ).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2), static typing; Godot shading language; Node 20 (pipeline token) |
| Primary Dependencies | GdUnit4; pipeline token F005 (`design/`, Style Dictionary 4) |
| Storage | Không có dữ liệu nghiệp vụ; mesh/clip `.res` nén trong repo |
| Testing | GdUnit4 headless; benchmark `MainHubBenchmark.tscn` có renderer (Mobile + Compatibility); `capture_hub.gd`, `capture_avatars.gd`; export Web (preset F133) |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | `scene_budget_hub`, `character_asset_budget` (PERFORMANCE_BUDGET §7, §8) |
| Constraints | AGENTS R1, R2, R17, R32, R33 · ADR-0047, ADR-0057, ADR-0101, ADR-0102, ADR-0103 |
| Scale/Scope | Main Hub + nhân vật/NPC/cổng gió dùng chung mọi zone; các vùng khác → F093 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec từ quyết định owner + ADR-0103; không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client + design token; không backend, không contract |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu tenant/học sinh mới; Cây Tia Sáng chưa gọi API (F068-1) |
| IV Server-authoritative | ✅ | Biểu cảm, clip, cấp cây chỉ là hiển thị; không đổi luật chơi |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Không vũ khí/bạo lực; emote không chế giễu (CHARACTER_SYSTEM §7) |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc ở WORLD_DESIGN §11 / CHARACTER_SYSTEM §4.1 / DESIGN_SYSTEM §5.3 / WORLD_RENDERING §7, §8.1; tham số ở CONFIG_DEFAULTS; màu ở token |
| VIII UI tách logic, token | ✅ | `SparkTreeModel` thuần; scene chỉ hiển thị; file sinh không sửa tay |
| IX Test có bằng chứng | ✅ | Mỗi F068-AC có test hoặc bằng chứng công cụ (quickstart §5) |

## Project Structure

```text
design/game-tokens/{primitive,art3d}.json, design/lib/pipeline.mjs, design/sd/formats/godot-tres.mjs   # tokens@0.10.0
src/game/resources/art/shaders/{toon,blob_shadow}.gdshader                    # cel shader, blob shadow (MIT)
src/game/resources/art/generated/mat_toon_skinned.tres, art3d_tokens.gd       # sinh
src/game/tools/art/{lowpoly_mesh,character_art,character_anim,hub_art,art_catalog,bake_art}.gd
src/game/tools/capture_avatars.gd                                             # mới
src/game/assets/models/chr/{chr_face_*,anim_kid_*}.res, assets/models/veg/veg_banyan_spark_s*_a.res
src/game/scripts/player/{kid_skeleton,avatar_builder,anim_driver,item_visual}.gd
src/game/scripts/world/{blob_shadow,spark_tree,models/spark_tree_model,zone_art_quality,npc_anchor}.gd, scripts/core/quality_policy.gd
src/game/scenes/character/{AvatarRig,Player}.tscn, scenes/npc/NPC.tscn, scenes/world/zones/main_hub/Zone.tscn
src/game/resources/items/emote_*.tres
src/game/tests/unit/art/test_art_pass_2.gd, tests/scene/character/test_kid_rig.gd, tests/unit/world/{test_spark_tree,test_blob_shadow}.gd
```

## Phase 0: Research

- Skinned mesh và outline inverted hull: COLOR (pháp tuyến mượt tư thế nghỉ) không đi theo xương → viền lệch khi tay giơ cao; dùng NORMAL đã skin cho preset nhân vật (mọi phần nhân vật đã mượt nên viền không nứt).
- AnimationTree lấy track theo root của AnimationPlayer (`AvatarRig`) → track `Bob/Skeleton3D:<xương>` chạy cả ở Player, RemotePlayer, S02, Tủ đồ (đo trong `test_kid_rig.gd`).
- Chân tay chibi ngắn hơn đầu: tay giơ thẳng 160–175° bị đầu che → góc ăn mừng/vẫy 105–135° (ảnh `avatars_after_poses_emotes.png`).
- Ngân sách: pháp tuyến mượt không thêm tam giác; vát cạnh mọi hộp và khối cầu chi tiết hơn làm Hub tăng từ 38k lên 91k tam giác tĩnh (Mobile 323k primitive) → giới hạn vát (≥ 0,15 m ở Hub), giữ bậc chia gốc cho vật lặp/xa/NPC, tắt đổ bóng mặt đất và bụi → 48k tĩnh, Mobile 149,6k, Compatibility 88k.
- `IN_SHADOW_PASS` hay outline trong suốt không giảm primitive đếm được (đếm lúc gửi), nên giảm bằng hình học/đổ bóng.

## Phase 1: Design & Contracts

- Không có contract API/event mới (contracts/README.md).
- Model hiển thị: data-model.md. Quy trình duyệt: quickstart.md.
