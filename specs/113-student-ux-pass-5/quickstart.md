# Quickstart: F113 — Student UX pass 5: game luôn chơi ngang (khóa ngang Android/iOS, màn "Xoay ngang" trên Web)

> Kiểm khóa ngang native, màn "Xoay ngang" của game và shell Web. Không cần backend (fixture + fake).

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile`; script: `npm ci --prefix scripts`; Playwright: `pnpm install` trong `src/cms/frontend`.
- Máy có GPU cho bước chụp ảnh (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Native luôn ngang** (F113-AC-01, AC-02): `make godot-test` — `test_orientation_config.gd`; export Android/iOS (CI F133) dùng cùng `project.godot`.
2. **Màn "Xoay ngang" trong game** (F113-AC-03…AC-05): chụp `godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 -s res://tools/capture_screens.gd -- --out=<dir> --tag=after --screens=rotate_prompt --sizes=390x844 --locales=vi-VN,en-US --band=<khối>`; `make godot-test` — `test_orientation_policy.gd`, `test_orientation_guard.gd`, `test_grade_band_matrix.gd` (`test_rotate_prompt_fits_every_band`).
3. **Shell Web** (F113-AC-06, AC-07): `node --test src/game/web/tests/`; `make godot-export-web`, mở bản Web trong Chromium ở 390×844 (dọc) → lời nhắc; 844×390 → "Chơi"; chơi rồi đổi khung nhìn sang dọc → màn nhắc của game; đổi lại → game chạy tiếp.
4. **Ngang không đổi** (F113-AC-09, AC-10): chụp màn game ở `--sizes=1280x720,844x390` cho 4 khối; `make godot-test` toàn bộ.

## 3. Lệnh test tự động

```bash
make godot-lint        # check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, brand_assets --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4 toàn bộ
make design-tokens-check
node --test src/game/web/tests/
node src/game/tools/gen_greybox_zones.mjs --check
```

## 4. Bằng chứng cần nộp

- Ảnh (`screenshots/`), số test GdUnit4 và test shell, kết quả lint, log export Web, bảng khối × màn.

## 5. Bằng chứng (2026-11-26, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/113-student-ux-pass-5/screenshots/`, 82 ảnh; `tools/capture_screens.gd`, renderer Compatibility = bản Web, fixture + fake, một màn mỗi tiến trình; ảnh Web bằng Chromium headless của Playwright, SwiftShader, `deviceScaleFactor` 2):

- **Trước** (mã F099, công cụ chụp chỉ thêm cỡ `390x844`): `login`, `activity`, `hub` `-before-390x844-vi-g1_3` — game co về canvas 1280×2770, chữ ≈ 30% (điện thoại cầm dọc).
- **Màn "Xoay ngang" native**: `rotate_prompt-after-390x844-vi-{g1_3,g4_5,g6_7,g8_9}`, `rotate_prompt-after-390x844-en-g4_5`.
- **Game ngang**: `{login,hub,activity,activity_matching,activity_tap_incorrect,result,shop,world_map}-after-{1280x720,844x390}-vi-{g1_3,g4_5,g6_7,g8_9}` (64 ảnh).
- **Web export (preset "Web") trong Chromium**: `web-shell-rotate-390x844-{vi,en}` (lời nhắc của shell), `web-shell-ready-844x390-{vi,en}` (xoay ngang → "Chơi"), `web-game-844x390-{vi,en}` (game chạy), `web-game-rotate-390x844-{vi,en}` (xoay dọc khi đang chơi → màn nhắc của game; game mặc định `vi-VN` nên chữ trong game là tiếng Việt cả khi trình duyệt tiếng Anh — bản en của màn nhắc trong game: ảnh native `rotate_prompt-after-390x844-en-g4_5`), `web-game-back-844x390-{vi,en}` (xoay lại → đúng màn cũ, 0 lỗi trang).

**Lỗi phát hiện khi xem ảnh và đã sửa**: (1) màn nhắc phóng bằng `CanvasLayer.scale` làm chữ nhỏ nhòe ở `g8_9` → đổi sang `content_scale_factor` của cửa sổ + Theme mật độ `g1_3`; (2) icon điện thoại mờ trên Web → import SVG 6×; (3) màn chờ Web ở 844×390 cắt logo và nút "Chơi" → Mây nhỏ khi cửa sổ ≥ 2:1; (4) câu tiếng Anh xuống dòng còn một chữ → rút gọn "Turn sideways to keep playing.".

**Bảng khối × màn — cửa sổ dọc** (ma trận `test_grade_band_matrix.gd` + xem ảnh; chỉ màn "Xoay ngang" — bố cục game dọc không còn là mục tiêu):

| Màn (cửa sổ dọc) | g1_3 | g4_5 | g6_7 | g8_9 | Ghi chú |
|---|---|---|---|---|---|
| Màn "Xoay ngang" 720×1280 | pass | pass | pass | pass | 3 chữ, phủ khung, chữ ≥ `font.small` |
| Màn "Xoay ngang" 390×844 | pass + ảnh | pass + ảnh (vi, en) | pass + ảnh | pass + ảnh | Chữ cỡ `g1_3` ở mọi khối, nét (content scale 16/9) |
| Shell Web 390×844 (trước khi chạy) | — | — | — | — | Không theo khối; ảnh vi/en, test jsdom + Chromium |
| Game bất kỳ ở cửa sổ dọc | thay bằng màn nhắc | thay bằng màn nhắc | thay bằng màn nhắc | thay bằng màn nhắc | Game dừng phía sau, chạy tiếp khi xoay lại (`test_orientation_guard.gd`) |

**Ma trận ngang** (cửa sổ 1280×720 / 1024×600 / 844×390, 4 khối, mọi màn của ma trận pass; dải trống lớn nhất trong bảng làm bài, % khung nhìn, 1280×720 / 844×390): chọn một 4/4, 7/7, 11/11, 13/13 · đúng/sai 13/13, 15/15, 18/18, 20/20 · đang chấm 5/5, 8/8, 11/11, 13/13 · đúng 4/4, 8/8, 11/11, 13/13 · chưa đúng 6/6, 6/6, 10/10, 13/13 (g1_3, g4_5, g6_7, g8_9; ngưỡng 25%).

**Gate** (local): GdUnit4 toàn bộ `-a res://tests` → 1091 test case, 0 lỗi, 0 fail, 0 flaky, 11 skip (integration cần backend), 0 orphan (195/195 suite); test mới: `test_orientation_config` 5, `test_orientation_policy` 4, `test_orientation_guard` 8, `test_grade_band_matrix` 22 (+ `test_rotate_prompt_fits_every_band`; cửa sổ 844×390 thay 720×1280), `test_boot` 4 (autoload #14). `check_typed` OK (560), `lint_theme_overrides` OK (463), `lint_layers` OK (266), `lint_signals` OK (336), `lint_art` OK (361), `zone_lint` OK (12), `bake_art --check` 0 stale, `brand_assets --check` OK (6), `extract_pot --check` OK (413), `pseudo_locale --check` OK, `gen_fixtures --check` OK (78), `gen_greybox_zones --check` 12 vùng, `check_no_admin_ui` OK, `scan_artifact_test` 22/22; `make design-tokens-check` (`pnpm test` 57/57, tương phản 544/544 AA, generated 99 file, semver OK); export Web preset "Web" exit 0 (`index.pck` 7,8 MB, `index.wasm` 39,5 MB), `check_web_export` + `scan_artifact` OK; `node --test src/game/web/tests/` 21 test: 20 pass, 1 skip (WebKit chưa cài), gồm Chromium 390×844 (lời nhắc, xoay ngang → "Chơi", toàn màn hình được yêu cầu khi bấm "Chơi", xoay dọc/ngang khi đang chơi không lỗi trang); check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`. `gdlint` chưa cài trên máy local (chạy ở CI). Không kiểm trên điện thoại Android/iOS thật (cấu hình kiểm bằng test; export native thuộc CI F133).
