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

## 5. Bằng chứng

(điền khi hoàn thành)
