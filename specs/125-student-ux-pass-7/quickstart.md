# Quickstart: F125 — Student UX pass 7: phóng to giao diện trên điện thoại và nút đóng đỏ cho mọi cửa sổ

> Kiểm chế độ `compact` trên điện thoại và nút đóng đỏ chung của mọi cửa sổ. Không cần backend (fixture + fake).

## 1. Chuẩn bị

- Godot 4.7.2 (`.godot-version`), `make godot-import` (chạy `--import` hai lần ở worktree mới).
- Token: `cd design && pnpm install --frozen-lockfile`; script: `npm ci --prefix scripts`.
- Máy có GPU cho bước chụp ảnh (không `--headless`).

## 2. Kịch bản kiểm tra

1. **Token và Theme `compact`** (F125-AC-03, AC-05): `make design-tokens-check` (generated khớp nguồn, 4 Theme `eduworld_theme_<band>_compact.tres`, tương phản AA, semver `0.21.0`).
2. **Chính sách và hệ số** (F125-AC-01, AC-02): `tests/unit/core/test_compact_policy.gd`, `tests/scene/test_compact_scale.gd`, `tests/scene/test_orientation_guard.gd`. Chơi thử bản Web: mở trên điện thoại cầm ngang (chữ to, nút to), kéo cửa sổ trình duyệt máy tính xuống thấp hơn 500 px (vào `compact` không tải lại), xoay dọc → "Xoay ngang", xoay lại → hệ số `compact` trở về.
3. **Ma trận khối** (F125-AC-04): `tests/scene/test_grade_band_matrix.gd` — mọi màn × 4 khối × 1280×720 / 1024×600 / 844×390 / 740×360 (hai điện thoại `compact`, probe vật lý 12 / 40 CSS px), kiểm ngược `test_physical_probe_flags_the_unscaled_phone_layout`.
4. **Nút đóng đỏ** (F125-AC-06, AC-07): `tests/scene/ui/test_window_close_button.gd`; chơi thử: mở từng cửa sổ từ HUD/menu "Thêm", bấm "✕ Đóng" và Esc.
5. **Ảnh** (F125-AC-08): `godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 -s res://tools/capture_screens.gd -- --out=<dir> --tag=after --screens=<màn> --sizes=844x390,740x360 --locales=vi-VN --band=<khối>`; thêm `--locales=en-US --band=g4_5 --sizes=844x390` và `--sizes=1280x720` cho các cửa sổ đổi nút đóng.

## 3. Lệnh test tự động

```bash
make godot-lint        # check_typed, lint_theme_overrides, lint_layers, lint_signals, lint_art, extract_pot, pseudo_locale, gen_fixtures, check_no_admin_ui
make godot-test        # GdUnit4 toàn bộ
make design-tokens-check
make godot-export-web   # preset "Web" + tools/ci/check_web_export.sh
```

## 4. Bằng chứng cần nộp

- Ảnh (`screenshots/`), số test GdUnit4, kết quả lint, `design-tokens-check`, log export Web, bảng màn × khối.

## 5. Bằng chứng (2026-11-27, local Windows 11, Godot 4.7.2 console, renderer Compatibility = bản Web)

**Kiểm ngược (F125-AC-04, chạy trên bố cục hệ số 1 trước khi xếp lại)** — `test_physical_probe_flags_the_unscaled_phone_layout`, 844×390 hệ số 1 (logic 1558×720, 0,54 CSS px / px logic), HUD + Cài đặt: chữ nhỏ nhất / vùng chạm nhỏ nhất `g1_3` 13,0 / 34,7 CSS px · `g4_5` 9,8 / 30,3 · `g6_7` 8,7 / 26,0 · `g8_9` 8,7 / 23,8 → probe báo lỗi ở cả 4 khối (test xanh = probe đỏ đúng chỗ). Lần chạy ma trận đầu tiên với hai cửa sổ `compact` (trước khi xếp lại): 8/38 test đỏ (đăng nhập tràn ngang, nút "Về nhiệm vụ" ngoài khung, ≥ 3 tin chat 102–160 px < 194–242 px, từ bị cắt ở thẻ đội, sheet chat, màn trận pseudo).

**Sau F125** — ma trận khối (4 cửa sổ × 4 khối, `vi`/`en`/pseudo): ở 844×390 chữ nhỏ nhất ≥ 13,0 CSS px, ở 740×360 ≥ 12,0 CSS px; vùng chạm nhỏ nhất ≥ 43,3 CSS px (844×390) / ≥ 40,0 CSS px (740×360) mọi khối.

**Ảnh** (`screenshots/`, 287 ảnh; `tools/capture_screens.gd`, fixture + fake):

- **Trước** (mã F109/F114 hiện tại, hệ số 1): `{login,hub,settings_access,journal,shop,inventory,world_map,challenge_board,activity,result,notifications,party_chat,arena,arena_battle_question}-before-{844x390,740x360}-vi-{g1_3,g8_9}` (56 ảnh).
- **Sau**: 23 màn (`login, hub, settings, settings_{graphics,sound,access,info}, journal, shop, shop_confirm, inventory, world_map, challenge_board, activity, result, notifications, party, party_chat, arena, arena_battle_question, game_menu, achievements, journey`) × `{844x390,740x360}` × 4 khối `vi` (184 ảnh), × `844x390-en-g4_5` (23 ảnh), và 1280×720 `vi` `g1_3`/`g8_9` cho 12 cửa sổ đổi nút đóng (24 ảnh).

**Lỗi phát hiện khi xem ảnh và đã sửa**: (1) icon nét xanh (Nhật ký, Bản đồ, Cửa hàng, menu Thêm) biến mất trên thanh tiêu đề xanh → ô trắng `TitleIconBadge`; (2) menu "Thêm" tràn trên/dưới ở 740×360 → 3 cột trong `compact`; (3) màn làm bài: thẻ đáp án bị bong bóng Mây che → ≤ 4 đáp án một hàng, bỏ câu cổ vũ chung, Mây cỡ bong bóng; (4) màn trận `g1_3`/`g4_5`: dải chat 2 dòng dưới câu hỏi che hàng đáp án → thu về thanh tiêu đề chat ("Mở chat"), dải chỉ lên cạnh header khi đủ chỗ cho cả cột chat; (5) Cửa hàng/Tủ đồ: lưới món đồ chỉ còn một vệt → chip Tủ đồ/Cửa hàng/Lịch sử lên thanh tiêu đề, bỏ câu mô tả trong thẻ món; (6) kết quả bài học: Mây chúc mừng cỡ huấn luyện viên để thấy kết quả không cần cuộn.

**Bảng màn × khối (xem từng ảnh 844×390 / 740×360 `vi`)**

| Màn | `g1_3` | `g4_5` | `g6_7` | `g8_9` |
|---|---|---|---|---|
| Đăng nhập | chữ to, thẻ co vừa giữa hai cột, Mây hiện | như `g1_3` | Mây + bong bóng, thẻ rộng | như `g6_7` |
| HUD | nút góc to (≈ 43 px), chip Xu/Thêm/Nhóm rõ | rõ | rõ, thêm nút Bản đồ | rõ |
| Cài đặt (5 tab) | ✕ Đóng đỏ phải trên, tab + Đăng xuất đủ | đủ | đủ | đủ |
| Nhật ký | thanh tiêu đề xanh + icon ô trắng + ✕ Đóng | như | như | như |
| Cửa hàng / hộp mua / Tủ đồ | chip trên thanh tiêu đề, lưới thấy hàng món, hộp mua rõ "Hủy" trung tính | như | như | như |
| Bản đồ | lưới 4 ô to | lưới | danh sách + "Đến đây" | danh sách |
| Thử thách | ✕ Đóng, độ khó + "Vào" đủ | đủ | đủ | đủ |
| Làm bài (chọn một) | 4 thẻ một hàng, Gợi ý cạnh Mây | như | như | như |
| Kết quả | cuộn nhẹ, "Về nhiệm vụ" ghim | như | như | như |
| Hộp thông báo | ✕ Đóng, hàng + "Xem" đủ | đủ | đủ | đủ |
| Nhóm / Nhóm + chat | "Rời nhóm" + ✕ Đóng; chat cao hết màn hình, ≥ 2–3 tin + tin nhanh + ô nhập | như | như | như |
| Đấu trường | ✕ Đóng + Hạng; thẻ chế độ cuộn, nút Tìm trận ghim | như | như | như |
| Trận (câu hỏi) | 4 đáp án một hàng, chat thu về thanh "Mở chat" (chỉ khi câu hỏi hiện) | như | như | như |
| Menu Thêm | 3 cột 6 ô, không tràn | như | như | như |
| Thành tựu / Hành trình | ✕ Đóng, danh sách cuộn | như | như | như |

`en` `g4_5` 844×390: không tràn (chip "Wardrobe / Shop / History" trên thanh tiêu đề, "Read aloud", "Achievements & Collections" xuống dòng giữa các từ). 1280×720: giống trước F125 trừ icon ô trắng và nút đỏ ✕ Đóng cuối thanh tiêu đề (Nhóm bỏ nút "Thu nhỏ").

**Còn lại (chấp nhận, không lỗi probe)**: ở 740×360 thẻ chế độ Đấu trường, phần "Hoạt động" của màn Nhóm và lưới món Cửa hàng `g1_3` cần cuộn để thấy hết (có thanh cuộn).
