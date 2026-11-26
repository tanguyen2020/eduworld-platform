# Quickstart: F096 — Student UX pass 1: nhận diện EduWorld và làm mượt hành trình học sinh

> Kiểm nhận diện EduWorld (boot splash, màn chờ Web, icon, giấy phép) và các sửa hành trình học sinh. Không cần backend.

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới); export template 4.7.2 cho bước Web.
- Token: `cd design && pnpm install --frozen-lockfile`; script: `npm ci --prefix scripts`.
- Máy có GPU cho bước chụp ảnh và sinh ảnh nhận diện (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Ảnh nhận diện** (F096-AC-01, AC-03, AC-05): `godot --path src/game -s res://tools/brand_assets.gd` (sinh); `godot --headless --path src/game -s res://tools/brand_assets.gd -- --check` → `brand_assets: OK`.
2. **Chụp hành trình** (F096-AC-19): `godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 -s res://tools/capture_screens.gd -- --out=<dir> --tag=after --sizes=1280x720,1024x600,720x1280 --locales=vi-VN,en-US --band=g1_3`.
3. **Web** (F096-AC-02, AC-03): `godot --headless --path src/game --export-release "Web" <dir>/index.html`; chép `src/game/web/shell_tokens.css` cạnh `index.html`; `node src/game/tools/web/serve_web.mjs <dir>`; mở bằng Chrome: logo, Mây, thanh tiến độ, "Đang chuẩn bị thế giới cho em…"; favicon và `index.apple-touch-icon.png` là icon EduWorld; sau "Chơi" boot splash EduWorld.
4. **Trong game**: đăng nhập → (lần đầu) Mây hướng dẫn 3 bước, "Bỏ qua" được; camera sau lưng nhân vật; thẻ "Việc tiếp theo"; nút HUD có icon; Bản đồ `g1_3` có ô hình + tên; Cài đặt → Thông tin → Giấy phép; làm một bài → dải chúc mừng.
5. **S01**: đăng nhập khi backend chậm → sau 10 s có "Hủy"; backend 503 → "Hệ thống đang bận, em thử lại sau ít phút nhé."

## 3. Lệnh test tự động

```bash
make godot-lint        # check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4 toàn bộ
make design-tokens-check
node --test src/game/web/tests/ scripts/ci/
```

## 4. Bằng chứng cần nộp

- Ảnh trước/sau (`screenshots/`), ảnh shell Web, số test GdUnit4, kết quả lint, log export Web.

## 5. Bằng chứng (2026-11-26, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/096-student-ux-pass-1/screenshots/`; `tools/capture_screens.gd`, renderer Compatibility = bản Web, fixture + fake):

| Nội dung | Trước | Sau |
|---|---|---|
| 19 màn hành trình (splash, login*, resume, curfew, character, hub, hub_onboarding*, hub_offline, journal, activity*, result, world_map, settings, about) 1280×720 + 1024×600 `vi` `g1_3` | `<màn>-before-{1280x720,1024x600}-vi-g1_3.png` | `<màn>-after-{1280x720,1024x600}-vi-g1_3.png` |
| Tiếng Anh 1280×720 | `<màn>-before-1280x720-en-g1_3.png` | `<màn>-after-1280x720-en-g1_3.png` |
| Cửa sổ dọc 720×1280 (splash, login, hub, settings) | `<màn>-before-720x1280-vi-g1_3.png` | `<màn>-after-720x1280-vi-g1_3.png` (+ `hub_onboarding`) |
| Khối `g6_7` (login, hub, journal, world_map, activity, result, settings, character) | `<màn>-before-1280x720-vi-g6_7.png` | `<màn>-after-1280x720-vi-g6_7.png` |
| Shell Web trong Chromium (Playwright) | — (shell F133: panel chữ, `<progress>` mặc định) | `web_shell_loading-after-*`, `web_shell_ready-after-*` (1280×720, 1024×600 vi/en, 720×1280), `web_shell_unsupported-after-1280x720-vi.png`, `web_boot_splash-after-1280x720-*`, `web_after_boot-after-1280x720-*` |

**Ảnh nhận diện** (`src/game/resources/branding/`): `boot_splash.png` 1280×720, `icon_1024.png` (bo góc), `icon_ios_1024.png` (vuông, đục), `icon_android_192.png`, `icon_adaptive_{fg,bg}_432.png`; `brand_assets.gd --check` → `brand_assets: OK (6 file(s))`.

**Web export** (preset `Web` F133): exit 0, `index.pck` 7,31 MB, `index.wasm` 39,5 MB; `index.icon.png`, `index.apple-touch-icon.png` và `index.png` (splash) là ảnh EduWorld; `check_web_export` OK, `scan_artifact` OK; `node --test src/game/web/tests/` 15 test: 14 pass (jsdom 12, gồm 3 test F096; Chromium 2), 1 skip (WebKit chưa cài).

**Gate**: GdUnit4 toàn bộ `-a res://tests` → 882 test case, 0 lỗi, 0 fail, 0 flaky, 10 skip (integration cần backend), 0 orphan; `check_typed` OK (478), `lint_theme_overrides` OK (402), `lint_layers` OK (231), `lint_signals` OK (285), `lint_art` OK (333), `zone_lint` OK (12), `bake_art --check` 0 stale, `brand_assets --check` OK, `extract_pot --check` OK (352; 484 khóa), `pseudo_locale --check` OK, `gen_fixtures --check` OK (62), `gen_greybox_zones --check` 12 vùng, `check_no_admin_ui` OK; `make design-tokens-check` (pnpm test 57/57, check OK); `node --test scripts/ci/` 50/50; check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check` sạch. `gdlint` chưa cài trên máy local (chạy ở CI).
