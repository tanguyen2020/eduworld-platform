# Data Model: F068 — Art pass 2: Chibi anime cartoon 3D cho Main Hub và nhân vật

> Feature chỉ có dữ liệu hiển thị phía client; không bảng, không event, không dữ liệu học sinh mới.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| `KidSkeleton` | code (`scripts/player/kid_skeleton.gd`) | `BONE_NAMES` (root, hips, chest, head, arm_l, arm_r, leg_l, leg_r), `BONE_PARENTS`, `BONE_REST`, `HEAD_CENTER`/`HEAD_RADII`, `ANCHORS` (slot → xương + offset), `EXPRESSIONS`, `face_path()`, `clip_path()` | code; Skeleton3D của `AvatarRig.tscn` khớp (test) |
| Mesh thân/trang phục skinned | công thức `character_art.gd` | VERTEX, NORMAL (mượt theo góc), COLOR (pháp tuyến hàn), TEX_UV (slot), BONES/WEIGHTS (4/đỉnh, tổng 1) | `assets/models/chr/chr_body_kid_a.res`, `chr_outfit_*_a.res` |
| Mesh mặt | `character_art.face(biểu cảm)` | decal chiếu lên ellipsoid đầu, không gian xương đầu; slot `eye`, `eye_white`, `eye_iris`, `eye_light`, `mouth`, `cheek`, `flower_pink`, `wind` | `chr_face_{neutral,happy,surprised,trying,blink}_a.res` |
| Clip | `character_anim.clip(tên)` | track rotation mọi xương, position + scale xương gốc, scale ngực; độ dài, lặp | `anim_kid_{idle,walk,run,jump,wave,celebrate,clap,dance,thank,apologize}_a.res` |
| `ItemVisual` (mở rộng) | `resources/items/emote_*.tres` | `emote_clip` → clip bake; `expression` (mới, rỗng = `happy`) | repo |
| Cây Tia Sáng | `hub_art.spark_tree(cấp)` | 5 mesh cấp 0–4 | `assets/models/veg/veg_banyan_spark_s<0–4>_a.res` |
| `SparkTreeModel` | code | `LEVELS` (= enum `MasteryLevel` learning.v1), `DEFAULT_STAGE` = `spark_tree_default_stage`, `stage_for(levels)` | code |
| Preset cel | token `toon.*`, `outline.*`, bảng màu | thêm `band_softness`; preset mới `toon_skinned` (`smooth_normals_in_color = false`) | `resources/art/generated/mat_toon*.tres` |
| Blob shadow | token `light.blob_shadow_color`, `light.blob_shadow_opacity` | `ShaderMaterial` dùng chung của `BlobShadow` | code + shader `blob_shadow.gdshader` |

## 2. Vòng đời / trạng thái

- Đổi hình hoặc clip: sửa công thức → `make godot-art-bake` → commit `.res` + ảnh; `bake_art --check` và `test_committed_art_matches_the_recipes` bảo đảm khớp (mesh, xương, trọng số, key clip).
- Biểu cảm: `neutral` ⇄ `blink` (đồng hồ `avatar_blink`); `set_expression()` đổi sang biểu cảm khác (dừng chớp mắt).
- Cây: `stage` 0–4, mặc định 3; `show_mastery(levels)` / `set_stage(n)` (kẹp 0–4).

## 3. Quy tắc kiểm tra dữ liệu

- Slot bảng màu chỉ thêm cuối (48–51 mới), tối đa 64.
- Biểu cảm ngoài danh sách bị từ chối (`set_expression` → false).
- Giá trị mastery lạ bị bỏ qua.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Không có event | — |
