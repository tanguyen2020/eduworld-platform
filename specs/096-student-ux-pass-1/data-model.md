# Data Model: F096 — Student UX pass 1: nhận diện EduWorld và làm mượt hành trình học sinh

> Feature chỉ có dữ liệu hiển thị phía client, một cờ thiết bị và token/icon; không bảng, không event, không dữ liệu học sinh mới.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| `OnboardingGuide` | code (`scripts/core/onboarding_guide.gd`) | `STEPS` (≤ 3: `{key, icon, target}`), `should_show(seen, just_created)`, `step`, `next()`, `skip()` | RAM; cờ `onboarding_hub_seen` (bool) trong `user://settings.cfg` |
| `NextStepHint` | code (`scripts/core/next_step_hint.gd`) | `for_assignments(count) → {kind: "assigned" \| "explore", label_key, icon, screen_id, args}` | RAM |
| `BrandLockup` | scene (`scenes/ui/components/BrandLockup.tscn`) | `SparkMark` + wordmark `LogoLabel` + `CloudyMascot` (tuỳ chọn), `compact` | scene |
| Ảnh nhận diện | `tools/brand_assets.gd` | `boot_splash.png` (1280×720, nền trong suốt), `icon_1024.png`, `icon_192.png`, `icon_adaptive_fg_432.png`, `icon_adaptive_bg_432.png` | `src/game/resources/branding/` |
| Icon game mới | `design/icons/game/*.svg` (màu `{token}`) | nút HUD, vùng World Map (`area_<icon_key>`), `star`, `sound_on`, `sound_off`, `hint`, `flag`, `party`, `medal`, `fire`, `play`, `pause` | → `src/game/resources/ui/icons/` (pipeline) |
| Token shell | `design/game-tokens/component.json` | `component.logo.*`, `component.backdrop.*`, `component.mascot.*`, `component.progress.*` → `--ew-component-*` | `src/game/web/shell_tokens.css` |

## 2. Vòng đời / trạng thái

- Hướng dẫn: `hidden` → (vào Hub, `should_show`) `step 1..n` → `done` (Tiếp ở bước cuối hoặc Bỏ qua) → ghi cờ thiết bị. "Xem lại hướng dẫn" (Cài đặt) xoá cờ và mở lại ở bước 1.
- S01: `busy` → (10 s) nút Hủy hiện → `cancel_login()` → `LOGIN` (form dùng được, mật khẩu đã xoá).
- Ảnh nhận diện: sửa thành phần S01/token → `godot --path src/game -s res://tools/brand_assets.gd` → commit PNG; `--check` bảo đảm cài đặt khớp.

## 3. Quy tắc kiểm tra dữ liệu

- Mọi ký tự trong `vi.po`/`en.po` có trong font Theme (`test_font_coverage.gd`).
- Ảnh nhận diện đúng kích thước; `project.godot`/`export_presets.cfg` trỏ đúng file; màu nền boot splash = token; có dòng `LICENSES.md`.
- Vùng chạm ≥ `DENSITY_BUTTON_MIN_HEIGHT[band]`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Không có event | — |
