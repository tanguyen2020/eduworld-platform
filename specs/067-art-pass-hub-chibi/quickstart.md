# Quickstart: F067 — Art pass 1: Main Hub low-poly stylized và nhân vật chibi (asset MIT tự làm)

> Kiểm đồ họa Main Hub và nhân vật, license asset, ngân sách, preset, renderer Web. Không cần backend.

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` một lần sau checkout.
- Token: `cd design && pnpm install --frozen-lockfile` (cho `make design-tokens(-check)`).
- Máy có GPU cho bước chụp ảnh và benchmark có renderer (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Bake lại art** (khi sửa công thức `src/game/tools/art/*_art.gd`): `make godot-art-bake` → 52 resource; `godot --headless --path src/game -s res://tools/art/bake_art.gd -- --check` → `0 resource(s) stale` (F067-AC-09).
2. **Lint art**: `godot --headless --path src/game -s res://tools/lint_art.gd` → `lint_art: OK` (F067-AC-03, AC-04). Thử thêm một PNG không có dòng trong `src/game/assets/LICENSES.md` → lint báo `has no licence row`.
3. **Chụp ảnh** (F067-AC-12): `godot --path src/game --rendering-method mobile -s res://tools/capture_hub.gd -- --out=<dir> --tag=after` (5 góc: overview, player, plaza_east, characters, portals); Web: thêm `--rendering-method gl_compatibility --tag=web`; preset: `--preset=low|high`. So với `screenshots/hub_before_*.png`: phải thấy cây đa, mái ngói, ruộng bậc thang, thuyền thúng, đèn lồng, cổng gió có biểu tượng, bảng tin (F067-AC-10), nhân vật chibi có mặt (F067-AC-08), viền mềm và dải toon (F067-AC-05), Web cùng màu với Mobile (F067-AC-07).
4. **Benchmark có renderer** (F067-AC-02): `godot --path src/game --rendering-method mobile res://tests/perf/MainHubBenchmark.tscn -- --seconds=8` (và `gl_compatibility`) → dòng JSON `max_draw_calls` ≤ 100, `max_primitives` ≤ 150000.
5. **Preset** (F067-AC-06): trong game mở Cài đặt → Chất lượng, đổi Thấp/Trung bình/Cao: cỏ/hoa/lúa thưa dần, viền mất ở xa (Thấp 12 m, Trung bình 30 m), bóng nắng tắt ở Thấp.
6. **Tạo nhân vật** (F067-AC-08): S02 đổi dáng người, màu da, tóc, màu tóc, trang phục, phụ kiện → preview đổi ngay bằng mesh chibi.
7. **Web export** (F067-AC-07): preset Web chính thức thuộc F133 T002; kiểm tạm bằng preset cục bộ (không commit) `--export-release "<Web preset>" <scratch>/index.html` → `index.pck` chứa `assets/models/*`, `toon*.gdshader`.

## 3. Lệnh test tự động

```bash
make godot-lint        # gdlint, check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4: tests/unit/art/*, tests/unit/world/test_zone_art_quality.gd, tests/scene/world/test_hub_art.gd, tests/perf/test_main_hub_benchmark.gd, F062/F070/F071 giữ xanh
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Ảnh trước/sau (`screenshots/`), số liệu benchmark có renderer, số test GdUnit4, kết quả lint, log export Web.

## 5. Bằng chứng (2026-11-24, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/067-art-pass-hub-chibi/screenshots/`, 1280×720):

| Góc | Trước (greybox) | Sau — Mobile, preset Trung bình | Web (Compatibility) / preset |
|---|---|---|---|
| Toàn cảnh | `hub_before_overview.png` | `hub_after_overview.png` | `hub_web_overview.png` |
| Sau lưng người chơi | `hub_before_player.png` | `hub_after_player.png` | — |
| Phía đông quảng trường | `hub_before_plaza_east.png` | `hub_after_plaza_east.png` | `hub_low_plaza_east.png` (Thấp), `hub_high_plaza_east.png` (Cao) |
| Nhân vật cận cảnh | `hub_before_characters.png` | `hub_after_characters.png` | `hub_web_characters.png` |
| Hàng cổng gió | `hub_before_portals.png` | `hub_after_portals.png` | — |
| Preview tạo nhân vật (S02) | — | `avatar_preview_after.png` (tóc đuôi ngựa, mũ sao, kính tròn, áo ngôi sao) | — |

**Benchmark có renderer** (`MainHubBenchmark.tscn --seconds=8`, preset Trung bình, vsync 60): Mobile (Vulkan) `max_draw_calls` 65, `max_primitives` 123450, FPS p10 60; Compatibility (OpenGL 3.3) 73, 69162, FPS p10 60. Proxy headless (CI): 57 draw call (mesh × pass vật liệu + MultiMesh + Label3D), 38166 tam giác, 1 body động. Load Hub 352–573 ms.

**Asset** (`bake_art`): 52 resource (45 mesh + 7 ScatterSet), 27,5k tam giác mesh; thân chibi 1074, NPC 1134–1462, cổng gió 558–638, cây đa 1,5k, địa hình 3,5k; `src/game/assets/models` 0,6 MB.

**Web export** (preset cục bộ tạm, không commit): `index.pck` 2,1 MB, `index.wasm` 39,5 MB, exit 0; 52 file `assets/models/*` và 2 shader toon trong gói; `export_presets.cfg` khôi phục nguyên trạng.

**Gate**: GdUnit4 toàn bộ `-a res://tests` → 812 test case, 802 chạy, 0 lỗi, 0 fail, 10 skip (integration cần backend), 0 orphan, 154 suite; `check_typed` OK (430), `lint_theme_overrides` OK (380), `lint_layers` OK (217), `lint_signals` OK (253), `lint_art` OK (108), `zone_lint` OK (12), `extract_pot --check` OK (336), `pseudo_locale --check` OK, `gen_fixtures --check` OK (62), `check_no_admin_ui` OK, `bake_art --check` 0 stale; `make design-tokens-check` OK (57 test token, 216/216 cặp tương phản, semver so với origin/main); check-features 93 feature 0 lỗi; check-cross-features 0 lỗi; check-links 0 hỏng; check-mermaid 0 lỗi; build-rtm `--check` khớp. `gen_greybox_zones.mjs --check` báo `main_hub` và `learning_zone_math` lệch seed F072 (quest object mới) — có sẵn trên main trước F067, không thuộc phạm vi; khối anchor F071 của Hub giữ nguyên.

## 6. Asset ngoài đề xuất (owner tự tải, chưa dùng)

Lượt này **không** dùng asset ngoài. Nếu muốn bổ sung, các bộ CC0 sau hợp phong cách low-poly. URL và license ghi theo hiểu biết hiện có, **chưa mở trang trong phiên này** (không truy cập mạng): owner kiểm lại license trên trang trước khi tải, rồi thêm dòng `LICENSES.md`:

| Bộ | URL | License ghi trên trang | Dùng cho |
|---|---|---|---|
| Kenney — Nature Kit | https://kenney.nl/assets/nature-kit | CC0 1.0 | Cây, đá, hoa bổ sung cho các vùng |
| Kenney — Fantasy Town Kit | https://kenney.nl/assets/fantasy-town-kit | CC0 1.0 | Nhà, tường, đường cho Thị trấn Trung Tâm |
| Quaternius — Ultimate Nature / Stylized Nature MegaKit | https://quaternius.com (mục Packs) | CC0 1.0 | Cây, bụi stylized cho Rừng Ngôn Ngữ |
| Quaternius — Ultimate Modular Characters / Animated Characters | https://quaternius.com | CC0 1.0 | Tham khảo rig + animation cho skeleton `kid_base` |
| Poly Haven (texture/HDRI) | https://polyhaven.com | CC0 1.0 | Không cần cho toon; chỉ nếu cần bầu trời HDRI |
