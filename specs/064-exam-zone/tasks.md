# Tasks: F064 — Exam Zone: thi có giờ theo server, tự nộp, resume, thông báo bài mới

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 [P] Khóa dịch `ui.exam.*` (`rules_start`, `flag`, `time_left`, `auto_submitted`, `results_pending`, cảnh báo 5/1 phút, xác nhận nộp), `ui.notification.assignment_new`, `ui.error.EXAM_*` — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — bằng chứng: `extract_pot --check` OK; khóa `ui.exam.*`, `ui.hub.board.exam_sessions`, `ui.error.EXAM_*` (vi/en, pot, xx_PS)
- [x] T002 [P] Fixture từ example OpenAPI (`ExamSession` hai kiểu `navigation`, `Attempt` exam có `saved_responses`, `AttemptReview`, Problem `EXAM_*`) — file: `src/game/tests/fixtures/exam/*.json` — bằng chứng: `gen_fixtures.mjs` thêm `ExamSession` (example mới) + `AttemptResult`; biến thể trong `tests/fakes/exam_fixtures.gd` (TEST-ONLY); `--check` OK
- [x] T003 [P] Typed model — file: `src/game/scripts/learning/models/exam_session.gd`, `src/game/scripts/learning/models/exam_result.gd` — bằng chứng: `ExamSessionModel`, `ExamResultModel` (+ `scripts/learning/exam_runner_state.gd`)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: `ExamSession.navigation`, body `{takeover}` cho `startExamAttempt` (và xác nhận `MyAssignment.exam_session_id` từ F062 T004) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — bằng chứng: `navigation` trong bundle + `split-public.mjs`; backend learning (migration `20261228000001`, `TestNavigationOf`, `TestExamSessionNavigation` integration xanh); `take_over` có sẵn từ F042, `MyAssignment.exam_session_id` có sẵn (F062) — Clarifications 2026-11-21
- [x] T005 `ExamCountdown` thuần (`remaining_ms`, mốc cảnh báo, dạng hiển thị theo grade band) — file: `src/game/scripts/learning/exam_countdown.gd` — bằng chứng: `scripts/learning/exam_countdown.gd`, test T006

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Làm bài kiểm tra có giờ (Priority: P1) 🎯 MVP

**Goal**: làm và nộp đề theo đồng hồ server.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T006 [P] [US1] Unit test F064-AC-03, F064-AC-04 với đồng hồ giả (còn lại theo offset, sai lệch ≤ 1 s, toast mỗi mốc một lần, `g1_3` dạng cát chảy) — file: `src/game/tests/unit/exam/test_exam_countdown.gd` — phải FAIL trước khi code — bằng chứng: FAIL (Parse Error: `ExamCountdown` chưa khai báo) → PASS 11/11
- [x] T007 [P] [US1] Unit test `ExamService` F064-AC-02, F064-AC-05, F064-AC-07, F064-AC-08 (`Idempotency-Key` start/submit, `saveExamAnswer` body, 409 → đã nộp) — file: `src/game/tests/unit/exam/test_exam_service.gd` — bằng chứng: FAIL (Parse Error) → PASS 11/11
- [x] T008 [P] [US1] Scene test F064-AC-01, F064-AC-06 (màn quy chế; `sequential_no_return` ẩn "Câu trước") — file: `src/game/tests/scene/exam/test_exam_lobby_runner.gd` — bằng chứng: FAIL (Parse Error) → PASS 10/10
- [x] T009 [P] [US1] Scene test F064-AC-05, F064-AC-11 (renderer chế độ exam không hiện đúng/sai/gợi ý; giới hạn độ dài; không gửi điểm) — file: `src/game/tests/scene/exam/test_exam_runner_items.gd` — bằng chứng: FAIL (Parse Error) → PASS 5/5
- [x] T010 [P] [US1] Unit test F064-AC-09, F064-AC-10 (map `EXAM_NOT_ASSIGNED`/`EXAM_NOT_OPEN`/`EXAM_WINDOW_CLOSED`/`EXAM_MAX_ATTEMPTS`; 404/`GAME_ACCESS_STUDENT_ONLY` không hiển thị dữ liệu) — file: `src/game/tests/unit/exam/test_exam_errors.gd` — bằng chứng: FAIL (Parse Error) → PASS 6/6
- [ ] T011 [US1] Integration headless với compose: `hs.an.3a` làm và nộp; `hs.chi.3b` bị `EXAM_NOT_ASSIGNED`; `hs.dung.7a` nhận 404 với attempt của th-demo-a (tenant isolation + authorization) — file: `src/game/tests/integration/exam/test_exam_flow.gd`  — đã viết `tests/integration/exam/test_exam_flow.gd` (skip khi không có `EDUWORLD_IT_API_BASE_URL`); chờ backend compose + seed đề mở cho 3A

### Implementation cho User Story 1

- [x] T012 [US1] `ExamService` (load/list/start/save/submit/result) — file: `src/game/scripts/learning/exam_service.gd` — bằng chứng: `scripts/learning/exam_service.gd` (+ `PendingWrites.enqueue_latest`, header phát lại), test T007/T010
- [x] T013 [US1] `ExamLobby` (danh sách `listExamSessions` + đề từ bài giao, màn quy chế), đăng ký màn `exam_lobby` cho intent F062 — file: `src/game/scenes/exam/ExamLobby.tscn`, `src/game/scenes/exam/exam_lobby.gd` — bằng chứng: `scenes/exam/ExamLobby.tscn` + `exam_lobby.gd`, `exam_lobby` trong `UiScreenRegistry`, test T008
- [x] T014 [US1] `ExamRunner`, `QuestionGrid`, `SubmitConfirm`, `ExamCountdownLabel` (renderer câu hỏi F063 ở chế độ exam, `suspend_world("exam")`) — file: `src/game/scenes/exam/ExamRunner.tscn`, `src/game/scenes/exam/exam_runner.gd`, `src/game/scenes/exam/QuestionGrid.tscn`, `src/game/scenes/exam/SubmitConfirm.tscn` — bằng chứng: `ExamRunner`, `QuestionGrid`, `SubmitConfirm`, `ExamCountdownLabel` (+ token `ExamClockWarningLabel`), test T008/T009
- [x] T015 [P] [US1] `Interactable` `board.exam_sessions` trong Main Hub greybox — file: `src/game/scenes/world/zones/main_hub/Zone.tscn` — bằng chứng: `Interactables/ExamBoard` (`kind = exam_board` → `exam_lobby`), `tests/scene/exam/test_exam_board.gd` PASS (test viết sau phần cảnh — ghi nhận)

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Tự nộp khi hết giờ và làm tiếp sau rớt mạng (Priority: P1)

**Goal**: không mất bài, không kéo dài giờ.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T016 [P] [US2] Unit test F064-AC-12, F064-AC-13 (đến `deadline_at` → khóa, flush, submit một lần; `EXAM_TIME_OVER` → đóng, không retry) — file: `src/game/tests/unit/exam/test_exam_time_up.gd` — phải FAIL trước khi code
- [ ] T017 [P] [US2] Unit test F064-AC-14 (mất mạng → `PendingWrites` gộp theo `position`, đồng hồ vẫn chạy, không ghi `user://`) — file: `src/game/tests/unit/exam/test_exam_offline.gd`
- [ ] T018 [P] [US2] Unit test F064-AC-15, F064-AC-16 (mở lại → `startExamAttempt` dựng từ `saved_responses`, `deadline_at` giữ; `EXAM_ACTIVE_ON_OTHER_DEVICE` → thông báo) — file: `src/game/tests/unit/exam/test_exam_resume.gd`
- [ ] T019 [US2] Integration qua toxiproxy: cắt mạng 2 phút rồi nối; tắt app và mở lại; để hết giờ (TC-EXM-002, TC-EXM-003) — file: `src/game/tests/integration/exam/test_exam_offline_resume.gd`

### Implementation cho User Story 2

- [ ] T020 [US2] Xử lý hết giờ, offline, resume trong `ExamService`/`ExamRunner` — file: `src/game/scripts/learning/exam_service.gd`, `src/game/scenes/exam/exam_runner.gd`

## Phase 5: User Story 3 — Xem kết quả và phân tích câu sai (Priority: P2)

**Goal**: kết quả theo công bố, ôn tập ngay.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T021 [P] [US3] Scene test F064-AC-17, F064-AC-18 (`EXAM_RESULTS_NOT_RELEASED` → pending; kết quả theo LO; trường đáp án chỉ khi có; không xếp hạng) — file: `src/game/tests/scene/exam/test_exam_result.gd` — phải FAIL trước khi code
- [ ] T022 [P] [US3] Scene test F064-AC-19 ("Luyện LO này" → `startAdHocAttempt` `selection` + `objective_id`) — file: `src/game/tests/scene/exam/test_exam_result.gd`

### Implementation cho User Story 3

- [ ] T023 [US3] `ExamResult` — file: `src/game/scenes/exam/ExamResult.tscn`, `src/game/scenes/exam/exam_result.gd`

## Phase 6: User Story 4 — Thông báo bài mới được giao (Priority: P2)

**Goal**: toast + badge khi có bài mới.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T024 [P] [US4] Unit test F064-AC-20, F064-AC-21 (diff `assignment_id` mới `not_started` → notice; lần đầu phiên → 1 toast gộp; tối đa 3 toast) — file: `src/game/tests/unit/notifications/test_notification_service.gd` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Scene test F064-AC-22, F064-AC-23 (chạm → mở tab, xóa badge; không làm mới khi app ở nền) — file: `src/game/tests/scene/notifications/test_notification_bell.gd`
- [ ] T026 [US4] Integration: `ad.truong.a` giao bài khi `hs.an.3a` đang ở Hub → toast ở lần quay lại Hub (TC-LMS-002) — file: `src/game/tests/integration/exam/test_assignment_notice.gd`

### Implementation cho User Story 4

- [ ] T027 [US4] `NotificationService` (nghe `AssignmentService.assignments_updated`) + `NotificationBell` trong HUD + kích hoạt làm mới khi đóng màn học tập/thi — file: `src/game/scripts/notifications/notification_service.gd`, `src/game/scenes/ui/hud/NotificationBell.tscn`

## Phase 7: Polish & Cross-Cutting

- [ ] T028 [P] Đo FPS runner + lệch đồng hồ trên DEV-AND-MID1 — file: `src/game/tests/perf/exam_runner_benchmark.gd`
- [ ] T029 [P] Visual `vi`/`en`/`xx-PS` cho lobby/runner/result (TC-I18N-006) — file: `src/game/tests/visual/exam_screens.gd`
- [ ] T030 [P] Đề xuất sửa EXAM_ENGINE §6 (lưu đệm chỉ RAM) và đăng ký tham số ân hạn offline vào CONFIG_DEFAULTS; CHANGELOG — file: `docs/02-learning/EXAM_ENGINE.md`, `docs/00-overview/CONFIG_DEFAULTS.md`, `CHANGELOG.md`
- [ ] T031 Sau khi delta `takeover` merge: nút "Chuyển sang thiết bị này" cho `EXAM_ACTIVE_ON_OTHER_DEVICE` + test — file: `src/game/scenes/exam/exam_lobby.gd`, `src/game/tests/unit/exam/test_exam_resume.gd`
- [ ] T032 Chạy [quickstart.md](quickstart.md) trên backend thật và ghi bằng chứng — file: `specs/064-exam-zone/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → (US3 ∥ US4) → Polish.
- Cần F063 T005/T006/T019 (hợp đồng renderer, `PendingWrites`, renderer câu hỏi) và F062 T019 (`AssignmentService`).

## Definition of Done

- [ ] Mọi tiêu chí F064-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test`, `make design-tokens-check` xanh; integration với F042 thật pass
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
