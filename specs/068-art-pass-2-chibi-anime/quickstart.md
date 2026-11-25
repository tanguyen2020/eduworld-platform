# Quickstart: F068 — Art pass 2: Chibi anime cartoon 3D cho Main Hub và nhân vật

> Kiểm hướng "Chibi anime cartoon 3D" (ADR-0103) trên Hub và nhân vật: hình khối, cel shading, rig/clip, biểu cảm, blob shadow, Cây Tia Sáng, ngân sách, license, Web. Không cần backend.

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile` (cho `make design-tokens(-check)`).
- Máy có GPU cho bước chụp ảnh và benchmark có renderer (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Bake** (khi sửa `src/game/tools/art/*`): `make godot-art-bake` → 71 resource (mesh, ScatterSet, clip); `godot --headless --path src/game -s res://tools/art/bake_art.gd -- --check` → `0 resource(s) stale` (F068-AC-13).
2. **Lint art**: `godot --headless --path src/game -s res://tools/lint_art.gd` → `lint_art: OK` (F068-AC-11).
3. **Chụp Hub** (F068-AC-15): `godot --path src/game --rendering-method mobile -s res://tools/capture_hub.gd -- --out=<dir> --tag=after`; Web: `--rendering-method gl_compatibility --tag=web`. So với `hub_before_*`: khối tròn, cạnh vát, dải cel rõ, viền mực ấm (AC-01, AC-02); trên Web có bóng tròn mờ dưới nhân vật, Mobile có bóng nắng (AC-08).
4. **Chụp nhân vật** (F068-AC-15): `godot --path src/game --rendering-method mobile -s res://tools/capture_avatars.gd -- --out=<dir> --tag=after` → `lineup` (6 kiểu tóc/trang phục/màu da/màu tóc), `accessories`, `body_types`, `expressions` (neutral/happy/surprised/trying), `poses_move` (idle/walk/run/jump), `poses_emotes` (6 emote), `closeup`, `spark_stages` (5 cấp cây); Web: `--rendering-method gl_compatibility --tag=web`.
5. **Benchmark có renderer** (F068-AC-10): `godot --path src/game --rendering-method mobile --resolution 1280x720 res://tests/perf/MainHubBenchmark.tscn -- --seconds=12` (và `gl_compatibility`) → `max_primitives` ≤ 150000, `max_draw_calls` ≤ 100.
6. **Trong game**: đi/chạy/nhảy — clip đổi và mặt đổi (chạy → cố gắng, nhảy → ngạc nhiên); bánh xe emote — vẫy, ăn mừng…; S02/Tủ đồ đổi tóc, đồ, phụ kiện, màu (AC-07, AC-12).
7. **Web export** (F068-AC-14): `godot --headless --path src/game --export-release "Web" <scratch>/index.html` → `index.pck` chứa `anim_kid_*`, `chr_face_*`, `veg_banyan_spark_s*`, `blob_shadow.gdshader`, `mat_toon_skinned`.

## 3. Lệnh test tự động

```bash
make godot-lint        # gdlint, check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4: test_art_pass_2, test_kid_rig, test_spark_tree, test_blob_shadow, test_baked_art, test_hub_art, F062/F070/F071 giữ xanh
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Ảnh trước/sau (`screenshots/`), số liệu benchmark có renderer, số test GdUnit4, kết quả lint, log export Web.

## 5. Bằng chứng (2026-11-25, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/068-art-pass-2-chibi-anime/screenshots/`, 1280×720; ảnh nhân vật "trước" là ảnh F067 `specs/067-art-pass-hub-chibi/screenshots/hub_after_characters.png`, `avatar_preview_after.png`):

| Nội dung | Trước (F067 low-poly) | Sau — Mobile, preset Trung bình | Web (Compatibility) |
|---|---|---|---|
| Hub toàn cảnh / sau lưng / phía đông / nhân vật / cổng gió | `hub_before_{overview,player,plaza_east,characters,portals}.png` | `hub_after_*.png` | `hub_web_*.png` |
| 6 kiểu tóc, trang phục, màu da, màu tóc | — | `avatars_after_lineup.png` | `avatars_web_lineup.png` |
| Phụ kiện (mũ sao, kính tròn, ba lô cú, cờ) | — | `avatars_after_accessories.png`, `avatars_after_closeup.png` | — |
| Dáng người kid_a…kid_d | — | `avatars_after_body_types.png` | — |
| Biểu cảm neutral / happy / surprised / trying | — | `avatars_after_expressions.png` | `avatars_web_expressions.png` |
| Tư thế idle / walk / run / jump | — | `avatars_after_poses_move.png` | `avatars_web_poses_move.png` |
| Tư thế emote wave / celebrate / clap / dance / thank / apologize | — | `avatars_after_poses_emotes.png` | `avatars_web_poses_emotes.png` (có blob shadow) |
| Cây Tia Sáng cấp 0–4 | — | `avatars_after_spark_stages.png` | — |

**Benchmark có renderer** (`MainHubBenchmark.tscn --seconds=12`, preset Trung bình, vsync 60): Mobile (Vulkan) `max_draw_calls` 64, `max_primitives` 149614, FPS p10 60; Compatibility (OpenGL 3.3) 77, 88010, FPS p10 60. Proxy headless: 58 (Mobile) / 62 draw call, 48,4k tam giác, 1 body động. Load Hub 221–223 ms. Lần đo đầu (mọi khối vát, khối cầu chi tiết, mặt đất đổ bóng): Mobile 323k → đã giảm theo plan Phase 0.

**Asset** (`bake_art`): 71 resource (54 mesh + 7 ScatterSet + 10 clip), 57,1k tam giác mesh; thân chibi 2 132, mặt 166–668, tóc 1 676–2 880, trang phục 346–976, phụ kiện 112–774 (nhân vật nặng nhất 7 856 ≤ 8k); NPC 1 870–2 032; Cây Tia Sáng 888 / 888 / 2 490 / 2 768 / 2 988 (cấp 0–4, ≤ 3k); `src/game/assets/models` 1,5 MB.

**Web export** (preset `Web` của F133): exit 0, `index.pck` 3,0 MB, `index.wasm` 39,5 MB; gói chứa `anim_kid_*`, `chr_face_*`, `veg_banyan_spark_s*`, `blob_shadow.gdshader`, `mat_toon_skinned.tres`.

**Gate**: GdUnit4 toàn bộ `-a res://tests` → 843 test case, 833 chạy, 0 lỗi, 0 fail, 10 skip (integration cần backend), 0 orphan, 159 suite (3 phút 56 giây); `check_typed` OK (446), `lint_theme_overrides` OK (389), `lint_layers` OK (225), `lint_signals` OK (263), `lint_art` OK (139), `zone_lint` OK (12), `extract_pot --check` OK (344), `pseudo_locale --check` OK, `gen_fixtures --check` OK (62), `check_no_admin_ui` OK, `bake_art --check` 0 stale; `make design-tokens-check`: 57/57 test token, 336/336 cặp tương phản, `design-tokens-check: OK` (semver so với origin/main); check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`: xem báo cáo push.

## 6. Asset ngoài đề xuất (owner tự tải, chưa dùng)

Lượt này **không** dùng asset ngoài (phân quyền chặn tải). Nếu muốn nâng chất lượng, các bộ sau hợp hướng chibi/cartoon; URL và license theo hiểu biết hiện có, **chưa mở trang trong phiên này**: owner kiểm lại license trước khi tải, rồi thêm dòng `LICENSES.md` (chỉ CC0/MIT/public domain — ADR-0102):

| Bộ | URL | License ghi trên trang | Dùng cho |
|---|---|---|---|
| Quaternius — Ultimate Animated Character Pack / Universal Animation Library | https://quaternius.com | CC0 1.0 | Tham khảo/retarget clip người (đi, chạy, vẫy) lên `kid_base` |
| Kenney — Mini Characters / Blocky Characters | https://kenney.nl/assets | CC0 1.0 | Tham khảo tỉ lệ chibi, phụ kiện |
| Kenney — Nature Kit, Fantasy Town Kit | https://kenney.nl/assets/nature-kit · https://kenney.nl/assets/fantasy-town-kit | CC0 1.0 | Đạo cụ, cây, nhà cho F093 (cần bo tròn/đổ bóng theo ADR-0103) |
| Quaternius — Stylized Nature MegaKit | https://quaternius.com | CC0 1.0 | Cây, bụi stylized cho Rừng Ngôn Ngữ (F093) |
| Poly Haven | https://polyhaven.com | CC0 1.0 | Chỉ nếu cần HDRI trời; không cần cho cel shading |
