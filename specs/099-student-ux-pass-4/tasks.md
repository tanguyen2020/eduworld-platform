# Tasks: F099 — Student UX pass 4: màu trạng thái rõ ràng, bố cục cân, trình phát media thân thiện

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F065, F066, F096, F097, F098 · ADR-0016, ADR-0047, ADR-0103

## Phase 1: Setup

- [x] T001 Worktree `feat/ux-pass-4`, `npm ci --prefix scripts`, `pnpm install` trong `design/`, Godot `--import` hai lần — F099-AC-17
- [x] T002 `tools/lib/screen_capture.gd`: màn `activity_video`, `activity_video_cue`, `activity_audio`, `activity_simulation` (playback giả TEST-ONLY); chụp ảnh "trước" 4 khối cho mọi màn sẽ đổi — F099-AC-16

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Token `tokens@0.16.0`: primitive `palette.{cyan,plum,indigo}.{50,700}`, `palette.ink.200`, `number.ratio.{150,400}`; bộ màu `component.answer_tile.{bg,accent}_1…6` mới; `component.answer_tile.{rest_bg,rest_border,rest_accent,rest_text,grow_max,grow_max_tall}`; `component.caption.*`, `component.media_track.*`, `component.media_stage.bg` (cặp chữ có `contrastWith`); Theme `AnswerTileRest`, `CaptionCard`, `CaptionLabel`, `MediaStage`; icon `play_light`, `pause_light`, `rewind`, `captions`; `design/CHANGELOG.md`; `make design-tokens` — F099-AC-01, F099-AC-04

## Phase 3: User Story 1 — Màu trạng thái (P1) 🎯 MVP

- [x] T004 [P] [US1] `tests/unit/learning/test_answer_palette.gd`: khoảng cách sắc độ ≥ 30° giữa mọi màu nhấn và màu đúng/chưa đúng — F099-AC-01
- [x] T005 [P] [US1] `tests/scene/learning/test_answer_state_colours.gd`: sau đúng/chưa đúng (có/không `correct_response`), thẻ khác là `AnswerTileRest`, StyleBox + huy hiệu không dùng màu trạng thái, chữ/số giữ; `is_correct` rỗng không đổi; câu đúng/sai; câu chèn video — F099-AC-02, F099-AC-03, F099-AC-12
- [x] T006 [US1] `AnswerBadge.Verdict.REST`, `QuestionRendererBase.show_verdict()` + `_all_tiles()` (single choice, true/false) — F099-AC-02, F099-AC-03

## Phase 4: User Story 2 — Bố cục cân (P1)

- [x] T007 [P] [US2] `BandLayoutProbe.empty_bands()` + kiểm ngược; `test_grade_band_matrix.gd` gọi cho màn một bước — F099-AC-07
- [x] T008 [P] [US2] `tests/scene/learning/test_balanced_layout.gd`: thẻ cao lên trong trần, một cột khi dọc, Mây liền dưới đáp án, không co dưới min, nội dung dài không căn giữa — F099-AC-05, F099-AC-06
- [x] T009 [US2] `QuestionRendererBase.fit_height()`; `single_choice`/`multiple_choice`/`true_false` `_fit_tiles()`/`_fit_rows()`/`_apply_tall()`; `QuizActivityRenderer.fit_layout()`; `ActivityHost._balance()` + spacer — F099-AC-05, F099-AC-06

## Phase 5: User Story 3 — Media (P2)

- [x] T010 [P] [US3] `tests/scene/video/test_media_player_ux.gd`: nút icon + chữ đổi theo trạng thái, nút giữa khung, "Lùi 10 giây" qua luật tua, thanh (vạch chương, dấu ?/✓, cao ≥ vùng chạm), `CaptionCard` + tên người nói, nút phụ đề bằng chữ, lời Mây; bài nghe; mô phỏng (mục tiêu ✓/○, bước đánh số, lượt thử) — F099-AC-08, F099-AC-09, F099-AC-10, F099-AC-11, F099-AC-12, F099-AC-13
- [x] T011 [US3] `MediaTrack`; `VttParser.voice_at`; `ActivityRendererBase.coach_hint()`/`coach_hint_changed`; host đọc lời gợi ý — F099-AC-09, F099-AC-11
- [x] T012 [US3] Video: `VideoLessonRenderer.tscn` + script (nút, nút giữa khung, thanh, phụ đề, Mây, overlay `ActivityBoard`, `show_verdict`) — F099-AC-08…F099-AC-12
- [x] T013 [US3] Bài nghe: thẻ bài nghe, nút, thanh có dấu câu hỏi, Mây, overlay — F099-AC-08, F099-AC-09, F099-AC-11, F099-AC-12
- [x] T014 [US3] Mô phỏng: thẻ mục tiêu ✓/○, bước đánh số, nút icon + chữ, lượt thử, Mây, overlay — F099-AC-11…F099-AC-13
- [x] T015 [US3] Test F065/F066 hiện có xanh (cập nhật theo F099 nơi kiểu phụ đề đổi) — F099-AC-14
- [x] T016 [US3] Ma trận: video, video câu chèn, bài nghe, mô phỏng × 4 khối × 3 cửa sổ — F099-AC-15

## Phase 5b: Chip lọc cuộn được (P3)

- [x] T021 [P3] `ChipScroller` (`UiBuild.chip_scroller`): mũi tên hai bên khi hàng chip tràn; `tests/scene/test_chip_scroller.gd`; ảnh `shop` 1024×600 `vi`/`en` `g4_5` trước/sau — F099-AC-18

## Phase 6: User Story 4 — Bằng chứng (P1)

- [x] T017 [US4] Ảnh "sau" 4 khối, xem ảnh, sửa lỗi; bảng khối × màn ở quickstart §5 — F099-AC-16

## Phase 7: Polish

- [x] T018 `.po` vi/en, `extract_pot`, `pseudo_locale`, `test_font_coverage` — F099-AC-17
- [x] T019 Tài liệu sở hữu: STUDENT_GAME_UX_LEARNING (§3.3a, §4.1a, §4.1b, §5, §6, §8), DESIGN_SYSTEM (§5.1, §9), VIDEO_LEARNING/AUDIO_LANGUAGE_LEARNING/SIMULATION_ENGINE (ghi chú hiển thị client), CODE_PLAN, TRACEABILITY_MATRIX, CHANGELOG, `design/CHANGELOG.md`, `assets/LICENSES.md`
- [x] T020 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `zone_lint`, `bake_art --check`, `brand_assets --check`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `gen_greybox_zones --check`, `check_no_admin_ui`, `make design-tokens-check`, export Web, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Dependencies & Execution Order

- Phase 1 (ảnh "trước" trên mã cũ) → Phase 2 (token) → US1 → US2 (dùng thẻ US1) → US3 → US4 → Phase 7.

## Definition of Done

- Mọi F099-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh trước/sau 4 khối trong `screenshots/`.
- Gate T020 pass; không literal màu; không đổi contract.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
