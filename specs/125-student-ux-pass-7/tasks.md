# Tasks: F125 — Student UX pass 7: phóng to giao diện trên điện thoại và nút đóng đỏ cho mọi cửa sổ

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F113, F114, F119, F123, F102 · ADR-0047, ADR-0057

## Phase 1: Setup

- [x] T001 Rebase `feat/f125` lên `origin/main` (F109, F114, F113); `pnpm install` trong `design/`, `npm ci --prefix scripts`, Godot `--import` hai lần; công cụ chụp thêm cửa sổ `740x360`; chụp ảnh "trước" (mã hiện tại: đăng nhập, HUD, Cài đặt Trợ năng, Nhật ký, Cửa hàng, Tủ đồ, Bản đồ, Thử thách, câu chọn một, kết quả, Hộp thông báo, Nhóm + chat, Đấu trường, câu hỏi trận đấu; `g1_3`, `g8_9` × 844×390, 740×360) — F125-AC-08

## Phase 2: Foundational (chặn mọi user story)

- [x] T002 Token `tokens@0.21.0`: primitive `size.control.60`, `size.layout.540`; component `component.compact.logical_height`; `density/compact.json` (`button.min_height` 60, `font.small` 18); bộ sinh `compactMap` + 4 Theme `eduworld_theme_<band>_compact.tres`, `UiTokens.COMPACT_THEME_PATHS`; `checks/structure.mjs`; `design/CHANGELOG.md`; `pnpm build` — F125-AC-03, F125-AC-05
- [x] T003 Cài đặt dự án `eduworld/ui/compact_short_side` = 500; `CompactPolicy` (thuần) — F125-AC-01

## Phase 3: User Story 1 — Chơi trên điện thoại vẫn đọc rõ, bấm trúng (P1) 🎯 MVP

- [x] T004 [P] [US1] Test trước: `tests/unit/core/test_compact_policy.gd` (ngưỡng độc lập mật độ, bàn phím ảo, canvas 540, hệ số 1 ngoài compact, chặn 16:9, sàn không hạ khối, sàn vật lý ở 844×390/740×360 + kiểm ngược hệ số 1, `SIZES` của ma trận khớp policy) — F125-AC-01, F125-AC-03, F125-AC-05
- [x] T005 [P] [US1] Test trước: `tests/scene/test_compact_scale.gd` (guard bật/tắt compact theo cửa sổ, ghi hệ số một lần, mật độ, bàn phím, "Xoay ngang" ưu tiên rồi trả hệ số chế độ, `Window` thật cao 540, Theme compact sàn + không hạ, `ThemeService`/`UiBuild` theo chế độ không tải lại) — F125-AC-01, F125-AC-02, F125-AC-03, F125-AC-05
- [x] T006 [US1] `OrientationGuard` (compact + hệ số chế độ, seam mật độ/bàn phím/`set_compact`), `AppState.compact`/`set_compact`/`compact_changed`, `ThemeService.set_compact`; `test_orientation_guard.gd` cập nhật (hệ số chế độ thay hệ số trước đó) — F125-AC-01, F125-AC-02
- [x] T007 [US1] `UiBuild.target()`/`target_for()` theo chế độ; HUD, chuông, vòng biểu cảm, thanh media, tạo nhân vật, phối đồ dùng helper; HUD xếp lại khi `compact_changed` — F125-AC-03
- [x] T008 [US1] Ma trận khối: `BandLayoutProbe` (2 điện thoại compact, Theme compact, ngưỡng `CompactPolicy`, probe vật lý 12/40 CSS px, `LEGACY_PHONE`), `_matrix` bật compact theo cửa sổ; `test_physical_probe_flags_the_unscaled_phone_layout` (kiểm ngược, chạy đỏ trên bố cục cũ trước khi sửa) — F125-AC-04
- [x] T009 [US1] Xếp lại mọi màn không vừa canvas 540 (cuộn/cột/gom, không thu chữ) đến khi ma trận xanh ở 4 cửa sổ × 4 khối — F125-AC-04, F125-AC-05

## Phase 4: User Story 2 — Mọi cửa sổ đóng cùng một cách (P1)

- [x] T010 [P] [US2] Test trước: `tests/scene/ui/test_window_close_button.gd` (16 cửa sổ × thường/compact: đúng 1 nút đỏ ✕ Đóng, cuối thanh tiêu đề ở đỉnh cửa sổ, vùng chạm; Esc và nút đóng như nhau; hộp xác nhận/màn làm bài/đăng nhập không có nút đỏ) — F125-AC-06, F125-AC-07
- [x] T011 [US2] `UiBuild.title_bar`/`close_button`/`fit_icon`; `UiBuild.window`, `ArenaUi.window` dùng chung; Nhật ký, Bản đồ, Phối đồ, sảnh đề thi (`.tscn`) đổi sang thanh tiêu đề chung; Nhóm bỏ "Thu nhỏ" (nút đỏ luôn hiện); Esc cho kết quả thử thách — F125-AC-06, F125-AC-07

## Phase 5: User Story 3 — Bằng chứng (P2)

- [x] T012 [US3] Ảnh "sau": mọi màn chính × 4 khối × 844×390 + 740×360 `vi`, `en` `g4_5` 844×390, 1280×720 `vi` các cửa sổ đổi nút đóng; xem từng ảnh, sửa lỗi; bảng màn × khối ở quickstart §5 — F125-AC-08
- [x] T013 [US3] GdUnit4 toàn bộ, lint Godot, `design-tokens-check`, export Web + `check_web_export`, kiểm spec/tài liệu xanh — F125-AC-08

## Phase 6: Polish

- [x] T014 Tài liệu: STUDENT_GAME_UX §2 (breakpoint compact), DESIGN_SYSTEM §5.1 (nút đóng chung), §5.2 (token compact), §9 (sàn vật lý, ảnh 740×360), CONFIG_DEFAULTS, STUDENT_GAME_UX_SOCIAL (Nhóm: bỏ "Thu nhỏ"), GODOT_ARCHITECTURE §10.2, F114 spec F114-3 → đã đóng, CODE_PLAN, CHANGELOG, `design/CHANGELOG.md`

## Dependencies & Execution Order

- T001 → T002/T003 → (T004, T005, T010 song song) → T006 → T007 → T008 → T009; T010 → T011; T009 + T011 → T012 → T013 → T014.

## Definition of Done

- Mọi F125-AC-* có test pass (bảng AC ↔ test ở plan.md) và bằng chứng ảnh ở quickstart §5; ảnh trong `screenshots/`.
- Gate T013 pass (GdUnit4 toàn bộ, lint Godot, `design-tokens-check` + WCAG AA, export Web, kiểm spec/tài liệu); không literal màu/cỡ; không đổi contract.
- Tài liệu sở hữu, CODE_PLAN, CHANGELOG, `design/CHANGELOG.md` cập nhật trong cùng push.
