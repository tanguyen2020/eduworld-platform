# Tasks: F113 — Student UX pass 5: game luôn chơi ngang (khóa ngang Android/iOS, màn "Xoay ngang" trên Web)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F096, F097, F098, F099, F133 · ADR-0047, ADR-0086, ADR-0101

## Phase 1: Setup

- [x] T001 Worktree `feat/ux-pass-5`, `npm ci --prefix scripts`, `pnpm install` trong `design/`, Godot `--import` hai lần; chụp ảnh "trước" (màn game ở 390×844 dọc — canvas co 1280×2770) — F113-AC-11

## Phase 2: Foundational (chặn mọi user story)

- [x] T002 Icon `design/icons/game/rotate_phone.svg` (MIT, `assets/LICENSES.md`, `design/CHANGELOG.md`), `make design-tokens`; khóa dịch `ui.rotate.{title,hint,desktop}` vi/en — F113-AC-08

## Phase 3: User Story 1 — Native luôn ngang (P1) 🎯 MVP

- [x] T003 [P] [US1] `tests/unit/core/test_orientation_config.gd`: `project.godot` orientation = `SCREEN_SENSOR_LANDSCAPE`; preset Android không có khóa hướng; iOS plist thêm không có hướng — F113-AC-01, F113-AC-02
- [x] T004 [US1] `project.godot` `window/handheld/orientation=4` — F113-AC-01

## Phase 4: User Story 2 — Màn "Xoay ngang" (P1)

- [x] T005 [P] [US2] `tests/unit/core/test_orientation_policy.gd`: dọc/ngang/vuông, hệ số phóng — F113-AC-03, F113-AC-05
- [x] T006 [P] [US2] `tests/scene/test_orientation_guard.gd`: bật/tắt theo cỡ (xoay, đổi cỡ), dừng/bỏ dừng, không phá dừng nơi khác, scene giữ trạng thái, màn phủ khung, tín hiệu, `ApiClient`/`RealtimeClient` `PROCESS_MODE_ALWAYS` — F113-AC-03, F113-AC-04, F113-AC-05
- [x] T007 [US2] `OrientationPolicy`, autoload `OrientationGuard` (#14), `RotatePrompt.tscn` + `rotate_prompt.gd`; `ApiClient`/`RealtimeClient` `PROCESS_MODE_ALWAYS`; `test_boot.gd` thứ tự autoload — F113-AC-03…F113-AC-05, F113-AC-07, F113-AC-08
- [x] T008 [US2] Shell: `#ew-rotate`, `isPortrait`, `data-portrait`, `tryLockLandscape` trong "Chơi"; `shell.test.mjs` (dọc ở loading/ready, không che unsupported, khóa hướng im lặng, chuỗi vi/en), `shell.browser.test.mjs` (Chromium 390×844) — F113-AC-06, F113-AC-07

## Phase 5: User Story 3 — Dọn phần dọc (P2)

- [x] T009 [US3] `BandLayoutProbe.SIZES` ngang (`844x390`), `PORTRAIT_SIZES`; `test_grade_band_matrix.gd` + `test_rotate_prompt_fits_every_band`; `screen_capture.gd` cỡ `844x390`/`390x844`, màn `rotate_prompt`, bỏ `720x1280` — F113-AC-05, F113-AC-09
- [x] T010 [US3] GdUnit4 toàn bộ xanh (F096–F099, F133) — F113-AC-10

## Phase 6: Bằng chứng

- [x] T011 Ảnh: `rotate_prompt` 390×844 4 khối; game 844×390 + 1280×720 4 khối; Web export + Chromium 390×844 `vi`/`en` (shell, trong game sau khi xoay); xem ảnh, sửa lỗi; bảng khối × màn ở quickstart §5 — F113-AC-11

## Phase 7: Polish

- [x] T012 `.po` vi/en, `extract_pot`, `pseudo_locale`, `test_font_coverage` — F113-AC-08
- [x] T013 Tài liệu: STUDENT_GAME_UX §2 (link màn nhắc), GODOT_ARCHITECTURE §4 (autoload #14), §10.2 (hiển thị/hướng), §11 (shell), DESIGN_SYSTEM §9 (ma trận/cỡ chụp), STUDENT_GAME_UX_LEARNING (ảnh dọc), F099 spec (superseded), CODE_PLAN, TRACEABILITY_MATRIX, CHANGELOG, `design/CHANGELOG.md`, `assets/LICENSES.md`
- [x] T014 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `zone_lint`, `bake_art --check`, `brand_assets --check`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `gen_greybox_zones --check`, `check_no_admin_ui`, `make design-tokens-check`, export Web + `check_web_export`, `node --test src/game/web/tests/`, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Dependencies & Execution Order

- Phase 1 (ảnh "trước") → Phase 2 (icon, chuỗi) → US1 → US2 → US3 → Phase 6 → Phase 7.

## Definition of Done

- Mọi F113-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh trong `screenshots/`.
- Gate T014 pass; không literal màu; không đổi contract.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
