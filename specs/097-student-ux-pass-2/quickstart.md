# Quickstart: F097 — Student UX pass 2: màn làm bài như một trò chơi, tạo nhân vật bằng hình, marker không che nhân vật

> Kiểm màn làm bài, S02 và marker gần camera trên client. Không cần backend (fixture + fake).

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile`; script: `npm ci --prefix scripts`.
- Máy có GPU cho bước chụp ảnh (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Màn làm bài** (F097-AC-01…08, AC-10): chụp `godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 -s res://tools/capture_screens.gd -- --out=<dir> --tag=after --screens=<màn> --sizes=1280x720,1024x600 --locales=vi-VN,en-US --band=g1_3` với `<màn>` ∈ `activity`, `activity_selected`, `activity_feedback`, `activity_incorrect`, `activity_hint`, `activity_multiple`, `activity_true_false`, `activity_fill_blank`, `activity_numeric`, `activity_matching`, `activity_ordering`, `activity_drag_drop` (**một màn mỗi lần chạy**: chạy nhiều màn trong một tiến trình có lúc treo sau vài màn có sign-in). Kiểm: bảng trên nền trời, đề **một lần** trong thẻ trắng + "Đọc to", 4 thẻ màu có số, chọn = nền nhấn + ✓, đường sao/vòng/chấm + "1/11", Mây + bong bóng; sai → bảng cam nhạt + ✗ + lời tử tế, Mây động viên.
2. **Khối** (F097-AC-02): chạy lại với `--band=g6_7` — chữ và thẻ nhỏ hơn (Theme theo khối được gắn như Boot).
3. **S02** (F097-AC-09): `--screens=character` — sân khấu lớn, ô hình chân dung + tên, ✓ trên ô đang chọn. Trong game: nhân vật quay chậm, kéo/bấm "Xoay" thì dừng 3 s; bật "Giảm chuyển động" thì đứng yên.
4. **Marker** (F097-AC-11): `--screens=hub` — ở spawn Hub không còn đá Tia Sáng che chân nhân vật; đi tới biển chào thì đá hiện lại ở phía trước.
5. **Test tự động**: `make godot-test` (GdUnit4), `make godot-lint`, `make design-tokens-check`.

## 3. Lệnh test tự động

```bash
make godot-lint        # check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, bake_art --check, brand_assets --check, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4 toàn bộ
make design-tokens-check
node src/game/tools/gen_greybox_zones.mjs --check
```

## 4. Bằng chứng cần nộp

- Ảnh trước/sau (`screenshots/`), số test GdUnit4, kết quả lint, log export Web.

## 5. Bằng chứng (2026-11-26, local Windows 11, Intel Iris Xe, Godot 4.7.2 console)

**Ảnh** (`specs/097-student-ux-pass-2/screenshots/`; `tools/capture_screens.gd`, renderer Compatibility = bản Web, fixture + fake; "trước" chụp trên main b307207f):

| Nội dung | Trước | Sau |
|---|---|---|
| 12 màn làm bài + S02 + Hub + Nhật ký, 1280×720 và 1024×600, `vi`, `g1_3` | `<màn>-before-{1280x720,1024x600}-vi-g1_3.png` | `<màn>-after-{1280x720,1024x600}-vi-g1_3.png` |
| Tiếng Anh 1280×720 (`activity`, `activity_feedback`, `character`, `hub`; sau thêm `activity_incorrect`) | `<màn>-before-1280x720-en-g1_3.png` | `<màn>-after-1280x720-en-g1_3.png` |
| Khối `g6_7` 1280×720 (`activity`, `activity_feedback`, `activity_incorrect`, `character`) | `<màn>-before-1280x720-vi-g6_7.png` | `<màn>-after-1280x720-vi-g6_7.png` |
| Cửa sổ dọc 720×1280 (`activity`, `hub`) | `<màn>-before-720x1280-vi-g1_3.png` | `<màn>-after-720x1280-vi-g1_3.png` |

Ghi chú: ảnh "trước" `g6_7` chụp bằng công cụ F096 (chưa gắn Theme theo khối nên chữ ở cỡ `g1_3`); ảnh "sau" đã gắn Theme khối như Boot. Công thức trong đề (fixture không trả SVG) hiện chữ thay thế; lượt này sửa chữ thay thế bị xuống dòng từng ký tự (`MathInline`).

**Gate**: GdUnit4 toàn bộ `-a res://tests` → 907 test case, 0 lỗi, 0 fail, 0 flaky, 10 skip (integration cần backend), 0 orphan (mới: `test_activity_player_ux` 13, `test_character_creation_ux` 5, `test_marker_fader` 6, `test_voice_over` +1); `check_typed` OK (486), `lint_theme_overrides` OK (407), `lint_layers` OK (236), `lint_signals` OK (290), `lint_art` OK (343), `zone_lint` OK (12), `bake_art --check` 0 stale, `brand_assets --check` OK (6), `extract_pot --check` OK (357), `pseudo_locale --check` OK, `gen_fixtures --check` OK (62), `gen_greybox_zones --check` 12 vùng, `check_no_admin_ui` OK; `make design-tokens-check` (`pnpm test` 57/57, tương phản 100 % cặp AA, check OK); export Web preset F133 exit 0 (`index.pck` 7,4 MB, `index.wasm` 39,5 MB); check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check` sạch. `gdlint` chưa cài trên máy local (chạy ở CI).
