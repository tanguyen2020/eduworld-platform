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

## 5. Bằng chứng

(điền sau khi chạy)
