# Tasks: F063 — Activity player: renderer registry, renderer 11 loại câu hỏi, gợi ý, kết quả, công thức SVG

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 [P] Khóa dịch `ui.activity_player.*`, `ui.question.*`, `ui.feedback.worked_example`, `ui.result.*`, `ui.learning.no_translation`, `ui.error.<CODE>` cho mã lỗi attempt — file: `godot/locale/vi.po`, `godot/locale/en.po`
- [ ] T002 [P] Script sinh fixture từ example OpenAPI (`Attempt` 11 loại câu, `ActivityResponseResult` có/không feedback, `HintResult`, `AttemptReview`) — file: `godot/tests/fixtures/learning/`, `tools/godot/gen_fixtures.gd`
- [ ] T003 [P] Typed model parse JSON — file: `godot/scripts/learning/models/attempt.gd`, `question_render.gd`, `response_result.gd`, `attempt_review.gd`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: `QuestionRender.hint_count`, `ContentManifest.activity_min_client_versions` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/modules/game.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T005 Hợp đồng `ActivityRendererBase`, `QuestionRendererBase`, `ActivityRendererRegistry` (autoload), `QuestionRendererRegistry` — file: `godot/scripts/learning/activity_renderer_base.gd`, `question_renderer_base.gd`, `activity_renderer_registry.gd`, `question_renderer_registry.gd`
- [ ] T006 `PendingWrites` (hàng đợi RAM, flush theo thứ tự, giữ `Idempotency-Key`) — file: `godot/scripts/networking/pending_writes.gd`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Làm hoạt động câu hỏi trong khung activity player (Priority: P1) 🎯 MVP

**Goal**: làm và nộp hoạt động `quiz`/`interactive_exercise`, resume được.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Unit test `LearningService` F063-AC-01, F063-AC-05, F063-AC-08 (body start, `Idempotency-Key` mới mỗi lần trả lời, giữ khi retry, 409 → thành công) — file: `godot/tests/unit/learning/test_learning_service.gd` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Scene test 11 renderer câu hỏi F063-AC-02, F063-AC-03 (payload fixture → `response_ready` đúng shape `QuestionResponse`; không có trường điểm) — file: `godot/tests/scene/learning/test_question_renderers.gd`
- [ ] T009 [P] [US1] Contract test F063-AC-03 (parse mọi fixture; quét không có `answer_key`, `correct_option_id(s)`, `feedback`, `hints`, `explanation` trước chấm) — file: `godot/tests/contract/learning/test_attempt_fixtures.gd`
- [ ] T010 [P] [US1] Scene test F063-AC-04 (dấu tiếng Việt, ảnh, audio; `g1_3` tự phát lồng tiếng, tắt tiếng → phụ đề + icon) — file: `godot/tests/scene/learning/test_voice_over.gd`
- [ ] T011 [P] [US1] Unit test F063-AC-06 (mất mạng → `PendingWrites`, banner, flush cùng key; không ghi `user://`) — file: `godot/tests/unit/learning/test_pending_writes.gd`
- [ ] T012 [P] [US1] Unit test F063-AC-07 (thoát → `:pause`; resume dựng đúng position, `saved_responses`, `hints_revealed`) — file: `godot/tests/unit/learning/test_learning_service_resume.gd`
- [ ] T013 [P] [US1] Scene test F063-AC-09 (mở → `suspend_world("activity")`, đóng → `resume_world`, quay đúng cảnh) — file: `godot/tests/scene/learning/test_activity_host.gd`
- [ ] T014 [P] [US1] Unit test F063-AC-10, F063-AC-11 (map `ATTEMPT_EXPIRED`, `ASSIGNMENT_NOT_OPEN`, `ASSIGNMENT_MAX_ATTEMPTS`, `ATTEMPT_NOT_IN_PROGRESS`, `INVALID_RESPONSE` → `ui.error.<CODE>`; 404/`GAME_ACCESS_STUDENT_ONLY` → không hiển thị dữ liệu) — file: `godot/tests/unit/learning/test_learning_errors.gd`
- [ ] T015 [P] [US1] Scene test F063-AC-12, F063-AC-13 (giới hạn độ dài ô nhập; `translation_missing` → nhãn; đổi locale giữ ngôn ngữ câu đang làm) — file: `godot/tests/scene/learning/test_question_input_limits.gd`
- [ ] T016 [US1] Integration headless với compose: `hs.an.3a` làm hết activity; truy cập attempt của `hs.dung.7a` → 404 (tenant isolation + authorization, TC-TEN-001) — file: `godot/tests/integration/learning/test_activity_flow.gd`

### Implementation cho User Story 1

- [ ] T017 [US1] `LearningService` (start/resume/respond/pause/submit, signal) — file: `godot/scripts/learning/learning_service.gd`
- [ ] T018 [P] [US1] 11 answer model — file: `godot/scripts/learning/answer_models/*_answer.gd`
- [ ] T019 [P] [US1] 11 scene câu hỏi theo bố cục §4.3 + skin grade band — file: `godot/scenes/learning/questions/*.tscn`
- [ ] T020 [US1] `QuizRenderer`, `InteractiveExerciseRenderer`, đăng ký registry — file: `godot/scenes/learning/activity_renderers/quiz/QuizRenderer.tscn`, `godot/scenes/learning/activity_renderers/interactive_exercise/InteractiveExerciseRenderer.tscn`
- [ ] T021 [US1] `ActivityHost` (header, tiến độ, đồng hồ theo `Clock`, action bar, hộp thoát, phím tắt `Enter/Esc/H/R/1–6/Tab`), `VoiceOverPlayer`, đăng ký màn `activity_host` cho intent F062 — file: `godot/scenes/learning/ActivityHost.tscn`, `godot/scenes/learning/activity_host.gd`, `godot/scenes/learning/VoiceOverPlayer.tscn`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Phản hồi sau chấm và gợi ý nhiều cấp (Priority: P1)

**Goal**: phản hồi theo FeedbackTiming, gợi ý từng cấp.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T022 [P] [US2] Scene test F063-AC-14 (có/không `is_correct`, feedback theo phương án, icon + chữ) — file: `godot/tests/scene/learning/test_feedback_strip.gd` — phải FAIL trước khi code
- [ ] T023 [P] [US2] Unit + scene test F063-AC-15, F063-AC-16 (`revealNextHint` với key; `HINTS_EXHAUSTED` → mờ; `HINTS_NOT_ALLOWED`/mode exam → ẩn; không tự đếm) — file: `godot/tests/scene/learning/test_hint_panel.gd`
- [ ] T024 [P] [US2] Scene test F063-AC-17 (nút ví dụ mẫu → overlay từ `getLesson`) — file: `godot/tests/scene/learning/test_worked_example.gd`

### Implementation cho User Story 2

- [ ] T025 [US2] `FeedbackStrip`, `HintPanel`, `WorkedExampleOverlay`, `LearningService.request_next_hint` — file: `godot/scenes/learning/FeedbackStrip.tscn`, `godot/scenes/learning/HintPanel.tscn`, `godot/scenes/learning/WorkedExampleOverlay.tscn`, `godot/scripts/learning/learning_service.gd`

## Phase 5: User Story 3 — Hiển thị công thức Toán/Lý/Hóa (Priority: P1)

**Goal**: SVG công thức cache bất biến, đúng Theme.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T026 [P] [US3] Unit test F063-AC-18, F063-AC-20 (kiểm `sha256`, cache path, khóa texture `(sha256, scale, màu)`, lỗi/vượt `math_svg_max` → `speech_text`) — file: `godot/tests/unit/learning/test_math_svg_cache.gd` — phải FAIL trước khi code
- [ ] T027 [P] [US3] Visual test F063-AC-19 (bộ công thức mẫu, TC-MATH-001) — file: `godot/tests/visual/test_math_inline.gd`

### Implementation cho User Story 3

- [ ] T028 [US3] `MathSvgCache`, `MathInline` (inline/block, màu Theme) — file: `godot/scripts/learning/math_svg_cache.gd`, `godot/scenes/learning/math/MathInline.tscn`

## Phase 6: User Story 4 — Màn kết quả học tập (Priority: P2)

**Goal**: kết quả học tập tách phần thưởng, theo grade band.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T029 [P] [US4] Scene test F063-AC-21, F063-AC-22 (2 vùng; rewards rỗng → ẩn; `g1_3` không thang 10; không có so sánh/nhận xét) — file: `godot/tests/scene/learning/test_activity_result.gd` — phải FAIL trước khi code
- [ ] T030 [P] [US4] Scene test F063-AC-23, F063-AC-24 (`results_later`; gợi ý → `startAdHocAttempt` có `recommendation_id`) — file: `godot/tests/scene/learning/test_activity_result.gd`

### Implementation cho User Story 4

- [ ] T031 [US4] `ActivityResult` + `LearningService.review()` — file: `godot/scenes/learning/ActivityResult.tscn`, `godot/scenes/learning/activity_result.gd`

## Phase 7: User Story 5 — Phiên bản client và loại chưa hỗ trợ (Priority: P2)

**Goal**: không crash với loại/dạng chưa hỗ trợ.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T032 [P] [US5] Scene test F063-AC-25, F063-AC-26 (registry thiếu loại, `min_client_version` cao, `422 ACTIVITY_TYPE_NOT_SUPPORTED`, item `type` lạ → màn cập nhật + telemetry) — file: `godot/tests/scene/learning/test_update_required.gd` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T033 [US5] `UpdateRequired` + kiểm phiên bản theo `ActivityRender.min_client_version` và manifest — file: `godot/scenes/learning/UpdateRequired.tscn`, `godot/scripts/learning/activity_renderer_registry.gd`

## Phase 8: Polish & Cross-Cutting

- [ ] T034 [P] Benchmark/đo activity player trên DEV-AND-MID1 (FPS, mở ≤ `load_time`, TC-PERF-020) — file: `godot/tests/perf/activity_player_benchmark.gd`
- [ ] T035 [P] Visual `vi`/`en`/`xx-PS` cho player, câu hỏi, kết quả (TC-I18N-006) — file: `godot/tests/visual/learning_screens.gd`
- [ ] T036 Chạy [quickstart.md](quickstart.md) trên backend thật (sau F030, F032 Done) và ghi bằng chứng — file: `specs/063-activity-player/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → (US2 ∥ US3) → US4 → US5 → Polish.
- T004 cần trước khi bỏ fallback ẩn/hiện nút Gợi ý theo lỗi; Bước 0 S1/S4 cần trước T018.
- F064, F065 dùng T005 (hợp đồng renderer) và T019 (renderer câu hỏi).

## Definition of Done

- [ ] Mọi tiêu chí F063-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test`, `make design-tokens-check` xanh; integration với F030/F032 thật pass
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
