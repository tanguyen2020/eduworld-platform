# Tasks: F098 — Student UX pass 3: chạm là trả lời, ma trận kiểm tra 4 khối

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F063, F069, F096, F097 · ADR-0016, ADR-0047, ADR-0103

## Phase 1: Setup

- [x] T001 Worktree `feat/ux-pass-3`, `npm ci --prefix scripts`, `pnpm install` trong `design/`, Godot `--import` hai lần — F098-AC-11
- [x] T002 `tools/lib/screen_capture.gd`: màn `activity_tap_grading`, `activity_tap_correct`, `activity_tap_incorrect`, `activity_tap_confirm` — F098-AC-10

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Token `tokens@0.15.0`: `component.answer_tile.{correct_bg,correct_border,try_bg,try_border,tag_correct_bg,tag_try_bg,tag_pending_bg,tag_pending_border,tag_text,tag_pending_text}` (cặp chữ có `contrastWith`); Theme `AnswerTileCorrect`, `AnswerTileTry`, `TileTagCorrect`, `TileTagTry`, `TileTagPending`; `design/CHANGELOG.md`; `make design-tokens` — F098-AC-04, F098-AC-11
- [x] T004 `InstantAnswerPolicy` + `ProjectSettings eduworld/learning/tap_confirm_bands`; `ResponseResultModel.correct_response` — F098-AC-01, F098-AC-02, F098-AC-08

## Phase 3: User Story 1 — Chạm là trả lời (P1) 🎯 MVP

### Tests

- [x] T005 [P] [US1] `tests/unit/learning/test_instant_answer_policy.gd`: mode/timing/release/loại câu/nhiều mệnh đề/cài đặt khối — F098-AC-01, F098-AC-02, F098-AC-08
- [x] T006 [P] [US1] `tests/scene/learning/test_tap_to_answer.gd`: chạm gửi ngay (body đúng hình), phím = chạm, không gửi ở exam/diagnostic/competition/after_submit/nhiều bước, khóa + "Đang chấm…" + không gửi hai lần + cùng khóa khi mất mạng, thẻ đúng/chưa đúng có icon + chữ, "Đáp án đúng" chỉ khi có `correct_response`, không verdict khi `is_correct` rỗng, Mây, nút "Tiếp", lỗi 4xx → "Gửi lại", resume không tự gửi, hướng dẫn chạm, vòng focus — F098-AC-01, F098-AC-02, F098-AC-03, F098-AC-04, F098-AC-05, F098-AC-06, F098-AC-07

### Implementation

- [x] T007 [US1] `QuestionRendererBase`: `choice_tapped`, `set_instant_answer`, `set_grading`, `show_verdict`, `_mark_tile`; `AnswerBadge.verdict` — F098-AC-03, F098-AC-04, F098-AC-07
- [x] T008 [US1] `single_choice_question.gd`, `true_false_question.gd`: chạm/phím phát `choice_tapped`, thẻ verdict, `correct_response` → thẻ đáp án đúng — F098-AC-01, F098-AC-04
- [x] T009 [US1] `ActivityRendererBase`/`QuizActivityRenderer`: chuyển tiếp `choice_tapped`, `supports_instant_answer`, `set_instant_answer`, `set_grading`; `ActivityHost`: `_instant`/`_double_tap`, gửi khi chạm, khóa khi chấm, ẩn "Kiểm tra", "Gửi lại", Mây — F098-AC-01, F098-AC-02, F098-AC-03, F098-AC-04, F098-AC-05, F098-AC-06, F098-AC-07

## Phase 4: User Story 2 — Chạm hai lần (P3)

- [x] T010 [US2] Hai chạm: chạm lần 1 chọn + Mây "Chạm lần nữa để chắc chắn nhé!", chạm lại cùng thẻ/phím/nút chính gửi; test trong `test_tap_to_answer.gd` — F098-AC-08

## Phase 5: User Story 3 — Ma trận 4 khối (P1)

- [x] T011 [P] [US3] `tests/fakes/band_layout_probe.gd` + `tests/scene/test_grade_band_matrix.gd`: 4 khối × 3 cửa sổ × S01, HUD, làm bài (chọn một, đúng/sai, ghép cặp, nhập số), phản hồi (đang chấm, đúng, chưa đúng), kết quả, S02, cửa hàng/tủ đồ, lên cấp, bản đồ — F098-AC-09
- [x] T012 [US3] Sửa lỗi bố cục/cỡ phát hiện bởi ma trận/ảnh: ô `VirtualItemGrid` cao theo chữ dài nhất (chữ tràn sang ô dưới ở `g1_3`/`g4_5`), chip Xu + nút "Thêm" của HUD `UiBuild.fit_wrapping` (cột phải co thành ô chỉ có icon), bàn phím số hai hàng cạnh ô nhập (phải cuộn ở `g1_3` 1024×600); probe thêm luật "word cut", "overlap", "wrapped text cut" — F098-AC-09
- [x] T013 [US3] Ảnh "sau" 4 khối (1280×720 + 1024×600 `vi`; `en` `g4_5`; 720×1280 chạm-đúng + HUD), xem ảnh, sửa lỗi; bảng khối × màn trong quickstart §5 — F098-AC-10

## Phase 6: Polish

- [x] T014 `.po` vi/en, `extract_pot`, `pseudo_locale`, `test_font_coverage` — F098-AC-11
- [x] T015 Test F063/F097 bị đổi hành vi (chọn một/đúng-sai ở practice nay gửi khi chạm) cập nhật theo F098; `test_exam_board` chạy lặp để kiểm phụ thuộc thứ tự
- [x] T016 Tài liệu sở hữu: STUDENT_GAME_UX_LEARNING (§3.6, §4.1b, §4.5), ACTIVITY_ENGINE (§8.1 ghi chú hành vi client, không đổi luật), DESIGN_SYSTEM (§5.1, §9), CONFIG_DEFAULTS (§13 `tap_confirm_bands`), CODE_PLAN, TRACEABILITY_MATRIX, CHANGELOG, `design/CHANGELOG.md`
- [x] T017 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `zone_lint`, `bake_art --check`, `brand_assets --check`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `gen_greybox_zones --check`, `check_no_admin_ui`, `make design-tokens-check`, export Web, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Dependencies & Execution Order

- Phase 1 → Phase 2 (token, policy) → US1 → US2 (dùng khung US1) → US3 (cần màn chạm của US1 cho ảnh/test phản hồi) → Phase 6.

## Definition of Done

- Mọi F098-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh 4 khối trong `screenshots/`.
- Gate T017 pass; không literal màu; không chính sách backend mới.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
