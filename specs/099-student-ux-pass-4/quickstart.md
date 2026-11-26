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

## 5. Bằng chứng (2026-11-26, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/099-student-ux-pass-4/screenshots/`, 200 ảnh = 100 trước + 100 sau; `tools/capture_screens.gd`, renderer Compatibility = bản Web, fixture + fake, một màn mỗi tiến trình). "Trước" chụp từ worktree sạch ở commit spec (mã F098, chỉ thêm builder màn media vào công cụ chụp). Màn: `activity`, `activity_true_false`, `activity_tap_grading`, `activity_tap_correct`, `activity_tap_incorrect`, `activity_video`, `activity_video_cue`, `activity_audio`, `activity_simulation` × 4 khối × 1280×720 + 1024×600 `vi`; `en` cho `g4_5` (cả hai cỡ); 720×1280 cho `activity`, `activity_tap_incorrect` ở 4 khối; `shop` 1024×600 `vi`/`en` `g4_5`. Tên: `<màn>-{before,after}-<cỡ>-<vi|en>-<khối>.png`.

**Bảng khối × màn** (ma trận `test_grade_band_matrix.gd` ở 3 cửa sổ + xem ảnh; số = dải trống lớn nhất trong bảng làm bài, % chiều cao khung nhìn, 1280×720 / 1024×600 / 720×1280; ngưỡng 25%. "fixed" = lỗi của lượt này đã sửa; "new" = màn mới vào ma trận):

| Màn | g1_3 | g4_5 | g6_7 | g8_9 | Ghi chú |
|---|---|---|---|---|---|
| Chọn một | fixed 4/6/9 | fixed 7/9/13 | fixed 11/13/17 | fixed 13/15/20 | Trước: `g8_9` 1280×720 ≈ 32%, 720×1280 ≈ 75% (ước từ ảnh, probe mới); thẻ cao lên ≤ 1,5× (ngang) / 4× (dọc), dọc một cột, Mây liền dưới |
| Đúng/sai | fixed 13/14/15 | fixed 15/17/18 | fixed 18/19/21 | fixed 20/21/23 | Bộ 2 đổi cam → mận; dọc xếp hai thẻ chồng |
| Đang chấm | fixed 5/6/10 | fixed 8/9/13 | fixed 11/13/17 | fixed 13/15/20 | |
| Đúng | fixed 4/6/10 | fixed 8/9/13 | fixed 11/13/17 | fixed 13/15/20 | Thẻ khác về thẻ nghỉ |
| Chưa đúng + đáp án đúng | fixed 6/5/8 | fixed 6/6/12 | fixed 10/12/16 | fixed 13/14/19 | Hết "hai thẻ xanh": thẻ số 3/4 trắng viền xám giữ số |
| Video (đang phát + phụ đề) | new | new | new | new | Nút icon + chữ, "Lùi 10 giây", `MediaTrack` (vạch chương, đĩa ?), `CaptionCard` "Cô: …", Mây gợi ý |
| Video — câu chèn | new | new | new | new | Câu hỏi chiếm khung trên `ActivityBoard`, thanh/nút ẩn; trước: overlay chật, thẻ bị cắt |
| Bài nghe | new | new | new | new | Thẻ bài nghe (loa + tên đoạn), "Nghe lại", Mây "Muốn nghe thêm…"; trước: vùng trống + toast |
| Mô phỏng | new | new | new | new | Thẻ mục tiêu ô + chữ (không mờ), bước "1.", "Thả"/"Làm lại" có icon, "Lượt thử: 0/3" |
| Cửa hàng 1024×600 `en` (chip) | — | fixed | — | — | Mũi tên hai bên khi hàng chip tràn (đóng điểm F098) |

Màn video/video câu chèn/bài nghe/mô phỏng: ma trận kiểm nút ≥ vùng chạm, chữ ≥ `font.small`, thanh tiến độ ≥ vùng chạm, không tràn — pass ở cả 4 khối × 3 cửa sổ.

**Gate** (local, trước rebase): GdUnit4 toàn bộ `-a res://tests` → 1073 test case, 0 lỗi, 1 fail → `test_extract_pot` (`messages.pot` chưa sinh lại cho khóa mới) → sinh lại `messages.pot` + `xx_PS.po`, chạy lại `test_extract_pot` + `test_font_coverage` + `test_pseudo_locale` → 10/10 pass; 11 skip (integration cần backend), 0 orphan. Test mới: `test_answer_palette` 3, `test_answer_state_colours` 7, `test_balanced_layout` 6, `test_media_player_ux` 14, `test_chip_scroller` 3, `test_grade_band_matrix` 21 (thêm dải trống + 4 màn media + kiểm ngược probe); cập nhật `test_tap_to_answer` (thẻ khác = nghỉ), `test_video_player_quality` (`CaptionCard`), `test_simulation_host` ("Lượt thử"). `check_typed` OK (554), `lint_theme_overrides` OK (459), `lint_layers` OK (264), `lint_signals` OK (333), `lint_art` OK (360), `zone_lint` OK (12), `bake_art --check` 0 stale, `brand_assets --check` OK (6), `extract_pot --check` OK (409), `pseudo_locale --check` OK, `gen_fixtures --check` OK (78), `gen_greybox_zones --check` 12 vùng, `check_no_admin_ui` OK; `make design-tokens-check` (`pnpm test` 57/57, tương phản 544/544 cặp AA, generated 98 file, semver OK); export Web preset "Web" exit 0 (`index.pck` 7,7 MB, `index.wasm` 39,5 MB, `check_web_export` + `scan_artifact` OK); check-features, check-cross-features, check-links (0 hỏng), check-mermaid (0 lỗi), build-rtm `--check` khớp. Sau rebase lên origin/main `2b4c1336` (F082, không chạm `src/game`/`design`): GdUnit4 toàn bộ → 1073 test case, 0 lỗi, 0 fail, 0 flaky, 11 skip, 0 orphan (192/192 suite); check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`, `make design-tokens-check`, `gen_fixtures --check` chạy lại xanh. `gdlint` chưa cài trên máy local (chạy ở CI).
