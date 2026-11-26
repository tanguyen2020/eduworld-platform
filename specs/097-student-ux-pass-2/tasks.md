# Tasks: F097 — Student UX pass 2: màn làm bài như một trò chơi, tạo nhân vật bằng hình, marker không che nhân vật

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F063, F065, F066, F070, F071, F094, F096 · ADR-0047, ADR-0103

## Phase 1: Setup

- [x] T001 `tools/lib/screen_capture.gd`: thêm màn `activity_selected`, `activity_incorrect`, `activity_hint`, `activity_multiple`, `activity_true_false`, `activity_fill_blank`, `activity_numeric`, `activity_matching`, `activity_ordering`, `activity_drag_drop`; chọn đáp án ở `activity_feedback`; ảnh "trước" trên main b307207f → `screenshots/*-before-*` — F097-AC-12
- [x] T002 Bảng kiểm UX trong spec.md (điểm R/D/T/P/A, vấn đề) — F097-AC-12

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Token `tokens@0.14.0`: primitive, density `font.stem`/`answer.min_height`, component `answer_tile`/`question_card`/`drop_zone`/`feedback`/`progress_trail`/`activity_board`/`coach`; generator `styleBox` (`borderBottom`, `padL`, `padT`), biến thể `AnswerTile1…6`, `QuestionCard`, `StemLabel`, `InstructionLabel`, `DropZone`, `ActivityBoard`, `PictureTile`, `AnswerGrid/Box/Flow`, bảng kết quả nền nhạt; `UiTokens.DENSITY_FONT_STEM`/`DENSITY_ANSWER_MIN_HEIGHT`; icon mới; `design/CHANGELOG.md`; `make design-tokens` — F097-AC-01, F097-AC-02, F097-AC-13
- [x] T004 Component `AnswerBadge`, `ProgressTrail`, `AvatarThumb`, `ColorDot`; `CloudyMascot` `compact`/`side_tail`/`mood` — F097-AC-01, F097-AC-05, F097-AC-08, F097-AC-09

## Phase 3: User Story 1 — Làm bài như chơi một trò chơi (P1) 🎯 MVP

### Tests

- [x] T005 [P] [US1] `tests/scene/learning/test_activity_player_ux.gd`: thẻ + huy hiệu, cỡ thẻ/chữ 4 khối, mọi dạng câu dùng thẻ, ✓ khi chọn, đề một lần (mọi khối, tắt tiếng), "Đọc to", đường tiến độ — F097-AC-01, F097-AC-02, F097-AC-03, F097-AC-04, F097-AC-05
- [x] T006 [P] [US1] `test_question_renderers.gd` (huy hiệu thay tiền tố "1.", thẻ khóa giữ dáng), `test_voice_over.gd` (`stem_on_screen`) — F097-AC-01, F097-AC-04

### Implementation

- [x] T007 [US1] `QuestionRendererBase`: `_add_stem` (thẻ câu hỏi + "Đọc to"), `_style_tile`, `_lock_tile`, `_tile_height`; `RichContentView.label_variation` — F097-AC-01, F097-AC-03
- [x] T008 [US1] 11 scene câu hỏi: `InstructionLabel`, `AnswerGrid/Box/Flow`; script chọn một/nhiều, đúng/sai, kéo thả (`DropZone`), ghép cặp, sắp xếp (icon mũi tên), nhập số, nhãn ảnh — F097-AC-01, F097-AC-02
- [x] T009 [US1] `ActivityRendererBase.wire_question`/`shows_stem` + `read_aloud_requested` ở quiz/video/audio/simulation; `VoiceOverPlayer.stem_on_screen` — F097-AC-03, F097-AC-04
- [x] T010 [US1] `ActivityHost.tscn` + `activity_host.gd`: nền trời + bảng, header Thoát/tiêu đề/đường tiến độ/"n/N", không lặp "Chọn một đáp án" — F097-AC-05, F097-AC-10

## Phase 4: User Story 2 — Phản hồi tử tế, có Mây (P1)

### Tests

- [x] T011 [P] [US2] `test_activity_player_ux.gd` (icon + chữ, lời tử tế không từ phạt, Mây vui/động viên, câu động viên, nút có icon + vùng chạm 4 khối), `test_feedback_strip.gd` (icon đĩa, nền nhạt, không đỏ) — F097-AC-06, F097-AC-07, F097-AC-08, F097-AC-10

### Implementation

- [x] T012 [US2] Hàng Mây + bong bóng (`CoachLine`, `FeedbackStrip`, `HintPanel`, lỗi), nút chính có icon theo hành động, gợi ý ẩn sau khi chấm; `FeedbackStrip` icon `verdict_*`, "Xem ví dụ" cạnh giải thích; `.po` `coach_cheer*`, `ui.feedback.*` — F097-AC-06, F097-AC-07, F097-AC-08, F097-AC-10

## Phase 5: User Story 3 — Tạo nhân vật bằng hình (P1)

- [x] T013 [P] [US3] `tests/scene/character/test_character_creation_ux.gd` (sân khấu lớn, bàn xoay + dừng sau thao tác, giảm chuyển động, ô hình + ✓, chấm màu, vùng chạm 4 khối); `test_character_creation_scene.gd` (F070) giữ nguyên xanh — F097-AC-09
- [x] T014 [US3] `CharacterCreation.tscn` (nền trời, `Stage`, camera, icon nút), `character_creation_screen.gd` (bàn xoay, `PictureTile` + `AvatarThumb`, `ColorDot`, chip); Tủ đồ dùng `ColorDot` — F097-AC-09

## Phase 6: User Story 4 — Marker không che nhân vật (P2)

- [x] T015 [P] [US4] `tests/unit/world/test_marker_fader.gd` (cài đặt mặc định, ẩn/hiện + trễ, marker cạnh/sau nhân vật giữ, nhóm, spawn Hub) — F097-AC-11
- [x] T016 [US4] `MarkerFader` + ZoneRoot, nhóm `camera_fade` (`WorldAnchor/Marker`, `QuestMarker3D`), `project.godot` `eduworld/world/marker_near_fade_m` — F097-AC-11

## Phase 7: User Story 5 — Bằng chứng & Polish

- [x] T017 [US5] Nhật ký: tab `ChoiceChip` + ✓, Đóng/Bắt đầu có icon — F097-AC-10
- [x] T018 [US5] Ảnh "sau" mọi màn đã sửa (1280×720, 1024×600, 720×1280; vi/en; g1_3/g6_7; Compatibility) — F097-AC-12
- [x] T019 [US5] `extract_pot`, `pseudo_locale`, `test_font_coverage` — F097-AC-13
- [x] T020 Tài liệu sở hữu: STUDENT_GAME_UX_LEARNING (§3.3, §3.7, §4.1, §7), STUDENT_GAME_UX_CORE (S02), DESIGN_SYSTEM §5.1–§5.2, WORLD_RENDERING §5.1, CONFIG_DEFAULTS, CODE_PLAN, TRACEABILITY_MATRIX, CHANGELOG, `design/CHANGELOG.md`, `assets/LICENSES.md`
- [x] T021 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `zone_lint`, `bake_art --check`, `brand_assets --check`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `gen_greybox_zones --check`, `check_no_admin_ui`, `make design-tokens-check`, export Web, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Dependencies & Execution Order

- Phase 1 → Phase 2 (token, component) → US1 → US2 (dùng khung US1) → US3, US4 (độc lập) → Phase 7.

## Definition of Done

- Mọi F097-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh trước/sau trong `screenshots/`.
- Gate T021 pass; không literal màu; icon mới có dòng license.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
