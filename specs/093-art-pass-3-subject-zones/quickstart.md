# Quickstart: F093 — Art pass 3: năm vùng môn học theo "Chibi anime cartoon 3D"

> Kiểm đồ họa năm vùng môn học (prefab vùng chung + set piece từng môn), ngân sách trên hai renderer, hợp đồng điểm neo, license, Web. Không cần backend.

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile` (cho `make design-tokens(-check)`).
- Máy có GPU cho bước chụp ảnh và benchmark có renderer (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Bake** (khi sửa `src/game/tools/art/*`): `make godot-art-bake`; `godot --headless --path src/game -s res://tools/art/bake_art.gd -- --check` → `0 resource(s) stale` (F093-AC-04).
2. **Điểm neo**: `node src/game/tools/gen_greybox_zones.mjs --check` → chỉ `main_hub` còn lệch (có từ trước F093, Câu hỏi mở F093-1); năm vùng môn học khớp seed (F093-AC-05).
3. **Lint art**: `godot --headless --path src/game -s res://tools/lint_art.gd` → `lint_art: OK` (F093-AC-02, F093-AC-06).
4. **Chụp vùng** (F093-AC-08): `godot --path src/game --rendering-method mobile -s res://tools/capture_zone.gd -- --area=learning_zone.math --out=<dir> --tag=after`; Web: `--rendering-method gl_compatibility --tag=web --shots=overview,player,north`. Năm góc: `overview` (toàn cảnh từ phía nam), `player` (sau lưng người chơi ở spawn), `east`/`west` (điểm khám phá), `north` (vật sưu tầm, cổng gió, landmark).
5. **Benchmark có renderer** (F093-AC-03): `godot --path src/game --rendering-method mobile --resolution 1280x720 res://tests/perf/ZoneBenchmark.tscn -- --area=learning_zone.math --seconds=12` (và `gl_compatibility`) → `max_primitives` ≤ 120000, `max_draw_calls` ≤ 90.
6. **Trong game**: từ Hub đi cổng gió Toán/Tiếng Việt/Tiếng Anh/Khoa học/Lịch sử; đi bộ tới hai điểm khám phá, vật sưu tầm và cổng gió về (biểu tượng ngôi nhà); ở Vương quốc Số đi qua cầu cong (sàn có collision dốc).
7. **Web export** (F093-AC-07): `godot --headless --path src/game --export-release "Web" <scratch>/index.html` → `index.pck` chứa `env_<môn>_*`, `bld_<môn>_*`, `prop_<môn>_*`, `veg_palm_coconut_a`, `prop_wind_gate_main_hub`, `learning_zone_<môn>/Zone.tscn`.

## 3. Lệnh test tự động

```bash
make godot-lint        # gdlint, check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4: test_subject_zone_art, test_zone_benchmark, test_baked_art, test_zone_lint, F062/F068/F071 giữ xanh
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Ảnh trước/sau (`screenshots/`), số liệu benchmark có renderer, số test GdUnit4, kết quả lint, log export Web.

## 5. Bằng chứng (2026-11-25, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/093-art-pass-3-subject-zones/screenshots/`, 1280×720 JPEG; `<môn>` = `math`, `vietnamese`, `english`, `science`, `history`):

| Nội dung | Trước (greybox F071) | Sau — Mobile, preset Trung bình | Web (Compatibility) |
|---|---|---|---|
| Toàn cảnh / sau lưng người chơi | `<môn>_before_{overview,player}.jpg` | `<môn>_after_{overview,player}.jpg` | `<môn>_web_{overview,player}.jpg` |
| Điểm khám phá đông / tây | — | `<môn>_after_{east,west}.jpg` | — |
| Vật sưu tầm, cổng gió về, landmark | — | `<môn>_after_north.jpg` | `<môn>_web_north.jpg` (có blob shadow) |

**Benchmark có renderer** (`ZoneBenchmark.tscn --seconds=12`, preset Trung bình, vsync 60, có người chơi):

| Vùng | Mobile (Vulkan) draw call / primitive | Compatibility (OpenGL 3.3) draw call / primitive | FPS p10 | Proxy tĩnh headless (draw call / tam giác, không người chơi) |
|---|---|---|---|---|
| Vương quốc Số | 40 / 114 646 | 44 / 67 246 | 60 / 60 | 42 / 35 686 |
| Rừng Ngôn Ngữ — Tiếng Việt | 33 / 111 640 | 38 / 64 972 | 60 / 60 | 34 / 34 298 |
| Rừng Ngôn Ngữ — Tiếng Anh | 31 / 111 312 | 36 / 64 904 | 60 / 60 | 32 / 34 558 |
| Đảo Khoa Học | 31 / 106 292 | 36 / 60 884 | 60 / 60 | 32 / 30 960 |
| Vương quốc Lịch Sử | 32 / 103 224 | 36 / 59 568 | 60 / 60 | 34 / 32 790 |

Lần đo đầu (trước khi tách cảnh xa và giảm rải cây): Vương quốc Số 131k, Rừng Ngôn Ngữ — Tiếng Việt 125k, Đảo Khoa Học 142k primitive (Mobile) → đã giảm theo plan Phase 0. Load vùng 97–268 ms.

**Asset** (`bake_art`, tam giác): terrain gần 1 376–2 264, cảnh xa 2 368–5 148 (gần + xa ≤ 7,5k mỗi vùng); landmark + nhà 2 206–3 446; set piece 656–2 268 (lớn nhất: Cây Chữ 2 268, Hang Vọng 2 032, trống đồng 1 992, cổng Bàn Tính 1 986); đồ nhỏ gộp 2 764–5 962; cây `veg_tree_autumn_a` 160, `veg_tree_blossom_a` 224, `veg_palm_coconut_a` 304; cổng gió về Thị trấn 682. Tổng 138 resource (mesh + ScatterSet + clip), `src/game/assets/models` 3,5 MB.

**Web export** (preset `Web` của F133): exit 0, `index.pck` 4,99 MB (F068: 3,0 MB), `index.wasm` 39,5 MB; gói chứa mesh, `ScatterSet` và scene của năm vùng.

**Gate**: GdUnit4 toàn bộ `-a res://tests` → 851 test case, 0 lỗi, 0 fail, 0 flaky, 10 skip (integration cần backend), 0 orphan, 161 suite (2 phút 14 giây); `check_typed` OK (456), `lint_theme_overrides` OK (389), `lint_layers` OK (225), `lint_signals` OK (270), `lint_art` OK (212), `zone_lint` OK (12), `extract_pot --check` OK (344), `pseudo_locale --check` OK, `gen_fixtures --check` OK (62), `check_no_admin_ui` OK, `bake_art --check` 0 stale; `make design-tokens-check`: 57/57 test token, 336/336 cặp tương phản, `design-tokens-check: OK`. check-features 95 feature, 329/329 FR Release được phủ, 0 lỗi, 0 cảnh báo; check-cross-features 0 lỗi (6 cảnh báo có sẵn); check-links 0 hỏng (10 556 link); check-mermaid 0 lỗi (126 sơ đồ); build-rtm `--check` khớp. `gdlint` chưa cài trên máy local (chạy ở CI `godot.yml`).

## 6. Asset ngoài đề xuất (owner tự tải, chưa dùng)

Lượt này **không** dùng asset ngoài (phân quyền chặn tải); danh sách bộ CC0 hợp hướng chibi/cartoon giữ ở [F068 quickstart §6](../068-art-pass-2-chibi-anime/quickstart.md#6-asset-ngoài-đề-xuất-owner-tự-tải-chưa-dùng).
