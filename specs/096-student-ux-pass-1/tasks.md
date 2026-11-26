# Tasks: F096 — Student UX pass 1: nhận diện EduWorld và làm mượt hành trình học sinh

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F060, F061, F062, F063, F064, F070, F071, F133 · ADR-0047, ADR-0101, ADR-0102, ADR-0103

## Phase 1: Setup

- [ ] T001 Công cụ chụp hành trình `tools/capture_screens.gd` + `tools/lib/screen_capture.gd` (fixture/fake, Compatibility); ảnh "trước" 13 màn ở 1280×720, 1024×600, 720×1280, `vi`/`en`, `g1_3`/`g6_7` → `screenshots/*-before-*` — F096-AC-19
- [ ] T002 Bảng kiểm UX trong spec.md (điểm R/D/T/P/A, vấn đề, ưu tiên) — F096-AC-19

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Icon game mới `design/icons/game/*.svg` (HUD, vùng World Map, sao, Mây vui, loa, gợi ý, cờ, lửa, play/pause); token shell `component.{logo,backdrop,mascot,progress}.*` trong `game-shell-css.mjs`; `tokens@0.13.0`, `design/CHANGELOG.md`; `make design-tokens` — F096-AC-02, F096-AC-09, F096-AC-15
- [ ] T004 Component `BrandLockup` (Tia Sáng + wordmark + Mây) — F096-AC-01, F096-AC-06

## Phase 3: User Story 1 — Nhận diện EduWorld (P1) 🎯 MVP

### Tests

- [ ] T005 [P] [US1] `tests/unit/core/test_branding.gd`: boot splash, icon project/Android/iOS, tiêu đề, `LICENSES.md`, `brand_assets.gd --check` — F096-AC-01, F096-AC-03, F096-AC-05
- [ ] T006 [P] [US1] `src/game/web/tests/shell.test.mjs` + `shell.browser.test.mjs`: logo, Mây, thanh tiến độ, câu vi/en, không literal màu, giữ WebGL 2/heap/audio — F096-AC-02
- [ ] T007 [P] [US1] `tests/scene/settings/test_settings_panel.gd`: trang Giấy phép (MIT Godot, thành phần bên thứ ba, OFL, MIT) — F096-AC-04

### Implementation

- [ ] T008 [US1] `tools/brand_assets.gd` sinh `resources/branding/*.png`, ghi `project.godot`/`export_presets.cfg`; `--check` — F096-AC-01, F096-AC-03, F096-AC-05
- [ ] T009 [US1] Shell Web: logo + Mây SVG, thanh tiến độ bo tròn, câu thân thiện — F096-AC-02
- [ ] T010 [US1] Splash, ResumeSplash, CurfewNotice, LoginWaiting dùng `SkyBackdrop` + `BrandLockup` — F096-AC-06
- [ ] T011 [US1] Cài đặt → Thông tin → Giấy phép — F096-AC-04

## Phase 4: User Story 2 — Vào Hub biết đi đâu (P1)

### Tests

- [ ] T012 [P] [US2] `tests/unit/player/test_camera_rig.gd`: pitch ban đầu trong khoảng, camera trên + sau người chơi — F096-AC-07
- [ ] T013 [P] [US2] `tests/scene/hud/test_toast_stack.gd`: toast hiện chữ — F096-AC-08
- [ ] T014 [P] [US2] `tests/scene/hud/test_main_hud.gd`: icon + nhãn đã dịch, không "(J)", vùng chạm — F096-AC-09
- [ ] T015 [P] [US2] `tests/unit/core/test_onboarding_guide.gd`, `tests/scene/hud/test_onboarding_hint.gd` — F096-AC-10
- [ ] T016 [P] [US2] `tests/unit/core/test_next_step_hint.gd`, thẻ Việc tiếp theo trong `test_main_hud.gd` — F096-AC-11

### Implementation

- [ ] T017 [US2] `Player.tscn` SpringArm3D −20° — F096-AC-07
- [ ] T018 [US2] `ToastStack.push` đặt `body_key` trước khi add — F096-AC-08
- [ ] T019 [US2] Icon nút HUD, nhãn, "Emote" dịch — F096-AC-09
- [ ] T020 [US2] `OnboardingGuide` + `OnboardingHint` + cờ thiết bị + "Xem lại hướng dẫn" — F096-AC-10
- [ ] T021 [US2] `NextStepHint` + `NextStepCard` — F096-AC-11

## Phase 5: User Story 3 — Đọc được, bấm được trên Web (P1)

- [ ] T022 [P] [US3] `tests/scene/world/test_world_map_scene.gd` — F096-AC-12
- [ ] T023 [P] [US3] `tests/unit/core/test_font_coverage.gd` — F096-AC-15
- [ ] T024 [P] [US3] `tests/scene/test_grade_band_touch_targets.gd` — F096-AC-18
- [ ] T025 [US3] World Map: ô hình icon + tên (`g1_3`), hàng danh sách cột cố định — F096-AC-12
- [ ] T026 [US3] Cài đặt: `ChoiceChip` + ✓ cho ngôn ngữ/preset — F096-AC-13
- [ ] T027 [US3] Thay emoji trong `.po`/scene bằng icon SVG — F096-AC-15

## Phase 6: User Story 4 — Phần thưởng vui, không áp lực (P1)

- [ ] T028 [P] [US4] `tests/scene/learning/test_activity_result.gd`: dải chúc mừng, sao `g1_3`, giảm chuyển động, không từ cấm — F096-AC-14
- [ ] T029 [US4] `CelebrationBanner` + màn kết quả (bỏ "1/11" ở header) — F096-AC-14

## Phase 7: User Story 5 — Đăng nhập không bị kẹt (P1)

- [ ] T030 [P] [US5] `tests/scene/auth/test_login_scene.gd` (Hủy sau 10 s), `tests/unit/core/test_l10n.gd` (503) — F096-AC-16, F096-AC-17
- [ ] T031 [US5] Nút Hủy S01; `L10n` ánh xạ HTTP 502/503/504 → `UNAVAILABLE`, chuỗi mới vi/en — F096-AC-16, F096-AC-17

## Phase 8: User Story 6 — Bằng chứng & Polish

- [ ] T032 [US6] Ảnh "sau" mọi màn đã sửa (Compatibility) + ảnh shell Web (Chromium) → `screenshots/` — F096-AC-19
- [ ] T033 [US6] `.po` vi/en, `extract_pot`, `pseudo_locale`, ảnh pseudo — F096-AC-20
- [ ] T034 Tài liệu sở hữu: STUDENT_GAME_UX (§3, §7), STUDENT_GAME_UX_CORE (S01, S03, S04), STUDENT_GAME_UX_SOCIAL §10, STUDENT_GAME_UX_LEARNING §10, DESIGN_SYSTEM §5.1, GODOT_ARCHITECTURE §5/§11, DEPLOYMENT/F133 (shell), CODE_PLAN, TRACEABILITY_MATRIX, CHANGELOG, `assets/LICENSES.md`, `design/CHANGELOG.md`
- [ ] T035 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `zone_lint`, `extract_pot --check`, `pseudo_locale --check`, `check_no_admin_ui`, `gen_fixtures --check`, `bake_art --check`, `gen_greybox_zones --check`, `brand_assets --check`, `make design-tokens-check`, `node --test scripts/ci/`, test shell, export Web, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Dependencies & Execution Order

- Phase 1 → Phase 2 (icon, token, BrandLockup) → US1 → US2 → US3 → US4 → US5 → Phase 8. US2–US5 độc lập với nhau sau Phase 2.

## Definition of Done

- Mọi F096-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh trước/sau trong `screenshots/`.
- Gate T035 pass; không literal màu; mọi asset có dòng license được phép.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
