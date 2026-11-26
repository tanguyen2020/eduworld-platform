# Quickstart: F094 — Art pass 4: sáu vùng còn lại theo "Chibi anime cartoon 3D" và các điểm mở của F093

> Kiểm đồ họa sáu vùng còn lại, ngân sách trên hai renderer, hợp đồng điểm neo, license, Web và ba điểm mở của F093. Không cần backend.

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile` (cho `make design-tokens(-check)`); portal: `cd src/cms/frontend && pnpm install --frozen-lockfile`.
- Máy có GPU cho bước chụp ảnh và benchmark có renderer (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Bake** (khi sửa `src/game/tools/art/*`): `make godot-art-bake`; `godot --headless --path src/game -s res://tools/art/bake_art.gd -- --check` → `0 resource(s) stale` (F094-AC-05).
2. **Điểm neo**: `node src/game/tools/gen_greybox_zones.mjs --check` → `12 zones checked`, không vùng nào lệch (F094-AC-07, F094-AC-10).
3. **Lint art**: `godot --headless --path src/game -s res://tools/lint_art.gd` → `lint_art: OK` (F094-AC-02, F094-AC-08); `zone_lint` OK (F094-AC-03).
4. **Chụp vùng** (F094-AC-09): `godot --path src/game --rendering-method mobile -s res://tools/capture_zone.gd -- --area=arena --out=<dir> --tag=after`; Web: `--rendering-method gl_compatibility --tag=web --shots=overview,player,north`. Góc chụp theo bố cục từng vùng (`capture_zone.gd` `AREA_SHOTS`).
5. **Benchmark có renderer** (F094-AC-04): `godot --path src/game --rendering-method mobile --resolution 1280x720 res://tests/perf/ZoneBenchmark.tscn -- --area=<area_id> --seconds=12` (và `gl_compatibility`) → `max_primitives` ≤ 120000, `max_draw_calls` ≤ 90.
6. **Trong game**: từ Hub đi cổng gió Thư viện Gió, Vườn Bạn Bè, Đại dương, Núi, Tháp, Đấu trường; đi bộ tới mọi điểm neo và cổng gió về (biểu tượng ngôi nhà). Ở Hub: ba vật thể quest F072 (đèn lồng đông/tây, biển chào) có mặt; cây cau không còn mặt đen khi nhìn từ dưới/sau.
7. **Token** (F094-AC-11): `make design-tokens-check`; portal `pnpm lint`, `pnpm typecheck`, Vitest `DomainTag`, `DashboardWidget`, theme.
8. **Web export** (F094-AC-06): `godot --headless --path src/game --export-release "Web" <scratch>/index.html` → `index.pck` chứa mesh/scene sáu vùng.

## 3. Lệnh test tự động

```bash
make godot-lint        # check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4: test_zone_art_pass_4, test_zone_benchmark, test_interactable, test_subject_zone_art
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Ảnh trước/sau (`screenshots/`), số liệu benchmark có renderer, số test GdUnit4, kết quả lint, log export Web, kết quả portal.

## 5. Bằng chứng (2026-11-26, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/094-art-pass-4-remaining-zones/screenshots/`, 1280×720; `<vùng>` = `practice_zone`, `social_zone`, `quest_zone_discovery_ocean`, `challenge_mountain`, `exam_zone`, `arena`):

| Nội dung | Trước (greybox F071) | Sau — Mobile, preset Trung bình | Web (Compatibility) |
|---|---|---|---|
| Toàn cảnh / sau lưng người chơi (biển chỉ đường) | `<vùng>_before_{overview,player}.jpg` | `<vùng>_after_{overview,player}.jpg` | `<vùng>_web_{overview,player}.jpg` |
| Set piece cạnh điểm neo (đông / tây theo bố cục vùng) | — | `<vùng>_after_{east,west}.jpg` | — |
| Cổng gió về, landmark | — | `<vùng>_after_north.jpg` | `<vùng>_web_north.jpg` |
| Main Hub sau F093-1/3 (vật thể quest F072, marker mới, cảnh xa tách) | — | `hub_after_{overview,player,plaza_east}.png` | `hub_web_{overview,player}.png` |

**Benchmark có renderer** (`ZoneBenchmark.tscn --seconds=10`, preset Trung bình, vsync 60, có người chơi; FPS p10 = 60 mọi lần chạy):

| Vùng | Mobile (Vulkan) draw call / primitive | Compatibility (OpenGL 3.3) draw call / primitive | Proxy tĩnh headless (draw call / tam giác) | Load |
|---|---|---|---|---|
| Thư viện Gió | 29 / 106 224 | 34 / 61 544 | 32 / 35 724 | 136–285 ms |
| Vườn Bạn Bè | 29 / 105 722 | 34 / 62 742 | 34 / 37 438 | 138–180 ms |
| Đại dương Khám Phá | 27 / 103 920 | 34 / 58 556 | 34 / 33 608 | 157–204 ms |
| Núi Thử Thách | 24 / 59 022 | 29 / 35 924 | 28 / 21 090 | 150–161 ms |
| Tháp Thử Thách | 27 / 85 316 | 33 / 49 456 | 31 / 28 114 | 152–199 ms |
| Đấu trường Nhà Vô Địch | 29 / 115 484 | 35 / 67 618 | 32 / 38 250 | 153–187 ms |
| Main Hub (`MainHubBenchmark.tscn`, `scene_budget_hub` 150k / 100) | 66 / 148 630 | 84 / 87 270 | — | — |

Lần đo đầu: Đấu trường 124 126 primitive (Mobile) → giảm bóng bay (khối cầu bậc chia 0), dây cờ, hoa/cỏ rải; Main Hub 155 498 (thêm 3 marker quest F072 + marker lớn) → marker nhẹ (102 tam giác, không đổ bóng), cảnh xa Hub tách `env_hub_far_a` (không viền/bóng) → 148 630.

**Asset** (`bake_art`, tam giác): terrain gần 1 332–1 990, cảnh xa 910–5 148 (gần + xa ≤ 7,2k mỗi vùng); landmark (+ nhà) 1 504–4 116; set piece 220–2 728 (lớn nhất: cổng xếp hàng 2 728, bục vinh danh 1 822, bảng lịch giải 1 602, tượng bạch tuộc 1 612); đồ nhỏ gộp 2 332–5 484; `veg_pine_a` 75; marker `prop_spark_marker_a` 102; `veg_palm_cau_a` 52 (khối kín); `env_hub_far_a` 1 516. Tổng 215 resource.

**Web export** (preset `Web` của F133): exit 0, `index.pck` 7,0 MB (F093: 4,99 MB), `index.wasm` 39,5 MB; gói chứa terrain/mesh/scene của sáu vùng, `veg_pine_a`, `env_hub_far_a`.

**Gate**: GdUnit4 toàn bộ `-a res://tests` → 859 test case, 0 lỗi, 0 fail, 0 flaky, 10 skip (integration cần backend), 0 orphan; `check_typed` OK (463), `lint_theme_overrides` OK (394), `lint_layers` OK (225), `lint_signals` OK (276), `lint_art` OK (295), `zone_lint` OK (12), `extract_pot --check` OK (344), `pseudo_locale --check` OK, `gen_fixtures --check` OK (62), `check_no_admin_ui` OK, `bake_art --check` 0 stale, `gen_greybox_zones.mjs --check` 12 vùng khớp; `make design-tokens-check` (pnpm test 57/57, check 336/336 cặp tương phản, OK); portal `pnpm lint` OK, `pnpm typecheck` OK, Vitest DomainTag/dashboard/theme/auth 14 file 92 test xanh (mã "không biết" của test DomainTag đổi từ `history` sang `music`); check-features 96 feature 0 lỗi, check-cross-features 0 lỗi, check-links 0 hỏng, check-mermaid 0 lỗi, build-rtm `--check` khớp. `gdlint` chưa cài trên máy local (chạy ở CI).
