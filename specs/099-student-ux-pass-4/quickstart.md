# Quickstart: F099 — Student UX pass 4: màu trạng thái rõ ràng, bố cục cân, trình phát media thân thiện

> Kiểm màu trạng thái, bố cục cân và trình phát media trên client. Không cần backend (fixture + fake).

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile`; script: `npm ci --prefix scripts`.
- Máy có GPU cho bước chụp ảnh (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Màu trạng thái** (F099-AC-01…AC-03): chụp `godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 -s res://tools/capture_screens.gd -- --out=<dir> --tag=after --screens=activity_tap_incorrect --sizes=1280x720 --locales=vi-VN --band=<khối>` — chỉ thẻ đã chọn (cam ✗) và thẻ "Đáp án đúng" (xanh ✓) có màu; thẻ khác trắng viền xám, giữ số. `make godot-test` — `test_answer_state_colours.gd`, `test_answer_palette.gd`.
2. **Bố cục cân** (F099-AC-05…AC-07): chụp `activity`, `activity_true_false`, `activity_tap_incorrect` ở `--sizes=1280x720,720x1280 --band=g8_9` — không khoảng trống lớn; dọc: một cột, Mây liền dưới đáp án. `make godot-test` — `test_balanced_layout.gd`, `test_grade_band_matrix.gd`.
3. **Media** (F099-AC-08…AC-13): chụp `activity_video` (đang phát, phụ đề, Mây), `activity_video_cue` (câu chèn), `activity_audio`, `activity_simulation` ở 4 khối. `make godot-test` — `test_media_player_ux.gd` + test F065/F066.
4. **Ảnh 4 khối** (F099-AC-16): chạy bước 1–3 với `--tag=before` trên mã trước F099 và `--tag=after` sau, `--band=g1_3|g4_5|g6_7|g8_9`, `--sizes=1280x720,1024x600`; thêm `--locales=en-US` cho `g4_5`; `--sizes=720x1280` cho `activity`, `activity_tap_incorrect`.

## 3. Lệnh test tự động

```bash
make godot-lint        # check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, brand_assets --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4 toàn bộ
make design-tokens-check
node src/game/tools/gen_greybox_zones.mjs --check
```

## 4. Bằng chứng cần nộp

- Ảnh trước/sau 4 khối (`screenshots/`), số test GdUnit4, kết quả lint, log export Web, bảng khối × màn.

## 5. Bằng chứng

(điền sau khi chạy)
