# Quickstart: F098 — Student UX pass 3: chạm là trả lời, ma trận kiểm tra 4 khối

> Kiểm chạm-là-trả-lời và bố cục 4 khối trên client. Không cần backend (fixture + fake).

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile`; script: `npm ci --prefix scripts`.
- Máy có GPU cho bước chụp ảnh (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Chạm là trả lời** (F098-AC-01, AC-03…AC-05, AC-07): chụp `godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 -s res://tools/capture_screens.gd -- --out=<dir> --tag=after --screens=<màn> --sizes=1280x720,1024x600 --locales=vi-VN --band=<khối>` với `<màn>` ∈ `activity_tap_grading` (thẻ "Đang chấm…", thẻ khác khóa), `activity_tap_correct` (✓ "Đúng rồi!", Mây vui), `activity_tap_incorrect` (✗ "Chưa đúng, không sao!" + thẻ "Đáp án đúng", giải thích, Mây động viên), `activity_tap_confirm` (hai chạm: ✓ + "Chạm lần nữa…"). Một màn mỗi lần chạy.
2. **Không đổi ở exam/diagnostic/competition** (F098-AC-02): `make godot-test` — `test_tap_to_answer.gd`.
3. **Ma trận khối** (F098-AC-09): `make godot-test` — `test_grade_band_matrix.gd` (4 khối × 3 cửa sổ).
4. **Ảnh 4 khối** (F098-AC-10): chạy bước 1 cho `login`, `hub`, `activity`, `activity_true_false`, `activity_matching`, `activity_numeric`, `activity_tap_*`, `result`, `character`, `shop`, `inventory`, `level_up`, `world_map` với `--band=g1_3|g4_5|g6_7|g8_9`; thêm `--locales=en-US` cho `g4_5` và `--sizes=720x1280` cho `activity_tap_correct`, `hub`.
5. **Hai chạm trong game**: đặt `eduworld/learning/tap_confirm_bands = PackedStringArray("g1_3")` trong `project.godot` (bản build thử), mở luyện tập ở khối `g1_3`.

## 3. Lệnh test tự động

```bash
make godot-lint        # check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, brand_assets --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4 toàn bộ
make design-tokens-check
node src/game/tools/gen_greybox_zones.mjs --check
```

## 4. Bằng chứng cần nộp

- Ảnh 4 khối (`screenshots/`), số test GdUnit4, kết quả lint, log export Web, bảng khối × màn.

## 5. Bằng chứng (2026-11-26, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/098-student-ux-pass-3/screenshots/`, 170 ảnh; `tools/capture_screens.gd`, renderer Compatibility = bản Web, fixture + fake, một màn mỗi tiến trình): 16 màn (`login`, `hub`, `activity`, `activity_true_false`, `activity_matching`, `activity_numeric`, `activity_tap_grading`, `activity_tap_correct`, `activity_tap_incorrect`, `activity_tap_confirm`, `result`, `character`, `shop`, `inventory`, `level_up`, `world_map`) × 4 khối × 1280×720 + 1024×600 `vi`; `en` cho `g4_5` (cả hai cỡ); 720×1280 cho `hub`, `activity_tap_correct`. Tên: `<màn>-after-<cỡ>-<vi|en>-<khối>.png`.

**Bảng khối × màn** (test ma trận `test_grade_band_matrix.gd` ở 3 cửa sổ + xem ảnh; "pass" = đạt ngay, "fixed" = sửa trong lượt này rồi đạt):

| Màn | g1_3 | g4_5 | g6_7 | g8_9 | Ghi chú |
|---|---|---|---|---|---|
| S01 đăng nhập | pass | pass | pass | pass | |
| HUD Hub | fixed | fixed | fixed | fixed | Chip Xu + nút "Thêm" co thành ô chỉ có icon (autowrap trong cột phải) → rộng tối thiểu theo nhãn (`UiBuild.fit_wrapping`) |
| Làm bài — chọn một | pass | pass | pass | pass | Hướng dẫn "Chạm vào đáp án em chọn" |
| Làm bài — đúng/sai | pass | pass | pass | pass | |
| Làm bài — ghép cặp | pass | pass | pass | pass | Giữ "Kiểm tra" |
| Làm bài — nhập số | fixed | fixed | fixed | fixed | Bàn phím 3 cột dưới ô phải cuộn (g1_3 1024×600) → hai hàng ngang cạnh ô nhập |
| Phản hồi — đang chấm | pass | pass | pass | pass | Mới (F098) |
| Phản hồi — đúng | pass | pass | pass | pass | Mới (F098) |
| Phản hồi — chưa đúng + đáp án đúng | pass | pass | pass | pass | Mới (F098) |
| Hai chạm (ảnh) | pass | pass | pass | pass | Mới (F098) |
| Kết quả | pass | pass | pass | pass | |
| S02 tạo nhân vật | pass | pass | pass | pass | |
| Tủ đồ | fixed | fixed | pass | pass | Tên dài ("Đồng phục học sinh") tràn sang ô hàng dưới → ô cao theo chữ |
| Cửa hàng | fixed | fixed | pass | pass | Như Tủ đồ (dòng giá bị che) |
| Lên cấp | pass | pass | pass | pass | |
| Bản đồ thế giới | pass | pass | pass | pass | |

Probe được kiểm ngược: với mã cũ (chip Xu autowrap, ô lưới cũ) `test_grade_band_matrix.gd` báo "word cut across lines" ở `hud` (8 dòng, 4 khối × 2 cửa sổ) và "Cell0 and Cell4 overlap" ở `wardrobe`; với mã mới xanh.

**Gate** (sau rebase lên origin/main 210593d1): GdUnit4 toàn bộ `-a res://tests` → 1035 test case, 0 lỗi, 0 fail, 0 flaky, 11 skip (integration cần backend), 0 orphan (mới: `test_instant_answer_policy` 9, `test_tap_to_answer` 22, `test_grade_band_matrix` 16; bao gồm `test_pseudo_locale` — đóng điểm F069 nút "Thêm" chưa chạy lại suite); `test_exam_board` pass trong cả hai lần chạy toàn bộ và khi chạy sau `scene/learning` + `scene/world` + ma trận (199 test) — không tái hiện phụ thuộc thứ tự; `check_typed` OK (547), `lint_theme_overrides` OK (457), `lint_layers` OK (262), `lint_signals` OK (331), `lint_art` OK (356), `zone_lint` OK (12), `bake_art --check` 0 stale, `brand_assets --check` OK (6), `extract_pot --check` OK (407), `pseudo_locale --check` OK, `gen_fixtures --check` OK (78; sinh lại `ClientConfig.json` theo trường `product_telemetry_enabled` của F080), `gen_greybox_zones --check` 12 vùng, `check_no_admin_ui` OK; `make design-tokens-check` (`pnpm test` 57/57, tương phản 488/488 cặp AA, generated 94 file, semver OK); export Web preset F133 exit 0 (`index.pck` 7,7 MB, `index.wasm` 39,5 MB, `check_web_export` + `scan_artifact` OK); check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check` sạch. `gdlint` chưa cài trên máy local (chạy ở CI).
