# Tasks: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Thêm input map `move_forward/back/left/right`, `jump`, `interact`, `open_journal`, `camera_rotate`, `camera_zoom` — file: `src/game/project.godot`
- [ ] T002 [P] Thêm khóa dịch `ui.hub.*`, `ui.quest.title`, `ui.quest.tab.assigned`, `ui.quest.start`, `ui.quest.due`, `ui.quest.overdue`, `ui.quest.empty`, `ui.quality.*`, `ui.common.break_reminder`, `ui.common.outside_access_window`, `ui.error.ASSIGNMENT_NOT_OPEN` — file: `src/game/locale/vi.po`, `src/game/locale/en.po`
- [ ] T003 [P] Sinh fixture từ example OpenAPI `MyAssignment` (rỗng, 2 bài, đủ trạng thái, `content_kind` lạ) — file: `src/game/tests/fixtures/learning/my_assignments_*.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: thêm `exam_session_id`, `result_release`, `late_policy` vào `MyAssignment` (chung PR với F064) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T005 Typed model `MyAssignmentModel.from_dict()` bỏ qua trường lạ — file: `src/game/scripts/learning/models/my_assignment.gd`
- [ ] T006 Zone greybox theo cây node chuẩn, `SpawnPoint` `main_hub.default`, `Interactable` `board.school_assignments` (kind `quest_board`) — file: `src/game/scenes/world/zones/main_hub/Zone.tscn`, `src/game/scripts/world/zone_root.gd`, `src/game/scripts/world/spawn_point.gd`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Vào Main Hub và điều khiển nhân vật (Priority: P1) 🎯 MVP

**Goal**: học sinh đi lại trong Hub, tương tác với điểm neo.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Scene test F062-AC-01 (spawn theo id, fallback `main_hub.default`) — file: `src/game/tests/scene/test_main_hub_zone.gd` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Unit test F062-AC-02, F062-AC-03 (vận tốc → trạng thái animation; tap-to-move chỉ bật ở `g1_3`) — file: `src/game/tests/unit/test_player_controller.gd`
- [ ] T009 [P] [US1] Scene test F062-AC-04 (vào vùng → nút hành động, phát `interacted(interactable_id)`, không gọi API) — file: `src/game/tests/scene/test_interactable.gd`
- [ ] T010 [P] [US1] Scene test F062-AC-05, F062-AC-06 (biến thể Theme theo grade band, đổi locale cập nhật HUD) + chạy `lint_theme_overrides.gd` — file: `src/game/tests/scene/test_main_hud.gd`

### Implementation cho User Story 1

- [ ] T011 [P] [US1] `PlayerController` (CharacterBody3D), `CameraRig`, `AnimDriver` (AnimationTree state machine), `TapToMove` — file: `src/game/scripts/player/player_controller.gd`, `src/game/scripts/player/camera_rig.gd`, `src/game/scripts/player/anim_driver.gd`, `src/game/scripts/player/tap_to_move.gd`, `src/game/scenes/character/Player.tscn`
- [ ] T012 [P] [US1] `Interactable` (Area3D, `interactable_id`, `kind`, `prompt_key`, signal `interacted`) — file: `src/game/scripts/world/interactable.gd`
- [ ] T013 [US1] `MainHud` tối thiểu (PlayerBadge, chip bài giao, Cài đặt, Về Quảng trường, slot ẩn), `ContextActionButton`, `Joystick` — file: `src/game/scenes/ui/hud/MainHud.tscn`, `src/game/scenes/ui/hud/main_hud.gd`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Xem và mở bài nhà trường giao (Priority: P1)

**Goal**: tab "Nhà trường giao" đọc API thật/mock, mở bài qua intent.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T014 [P] [US2] Unit test F062-AC-07, F062-AC-12 với `FakeApiClient` (gắn `X-Tenant-Id`, 401 → refresh 1 lần, 403 → `session_expired`, 5xx → giữ dữ liệu cũ) — file: `src/game/tests/unit/test_assignment_service.gd` — phải FAIL trước khi code
- [ ] T015 [P] [US2] Scene test F062-AC-08, F062-AC-11 (cột hiển thị, định dạng hạn theo locale, không có điểm/tên người khác, danh sách ảo hóa) — file: `src/game/tests/scene/test_quest_journal_assigned.gd`
- [ ] T016 [P] [US2] Scene test F062-AC-09, F062-AC-10 (intent `activity_host`/`exam_lobby` đúng tham số; `content_kind` lạ → `update_required` + telemetry) — file: `src/game/tests/scene/test_quest_journal_assigned.gd`
- [ ] T017 [P] [US2] Test F062-AC-13 (profile `mock` hiện nhãn MOCK; export preset release không chứa `config/env/mock.cfg`) — file: `src/game/tests/unit/test_config_mock_label.gd`
- [ ] T018 [US2] Integration headless với backend compose: `hs.an.3a` chỉ thấy bài 3A, `hs.dung.7a` không thấy bài th-demo-a (tenant isolation, TC-LMS-002) — file: `src/game/tests/integration/test_my_assignments_isolation.gd`

### Implementation cho User Story 2

- [ ] T019 [US2] `AssignmentService` (`refresh()`, `start(assignment)`, signal `assignments_updated`) — file: `src/game/scripts/learning/assignment_service.gd`
- [ ] T020 [US2] `QuestJournal` (tab `assigned`), `AssignmentRow`, mở từ phím J/nút 📜/bảng Hub — file: `src/game/scenes/quest/QuestJournal.tscn`, `src/game/scenes/quest/quest_journal.gd`, `src/game/scenes/quest/AssignmentRow.tscn`
- [ ] T021 [US2] Đăng ký làm mới khi vào/quay lại Hub (`SceneRouter.zone_entered`, `EventBus.ui_closed`) — file: `src/game/scripts/world/zone_root.gd`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Chạy mượt trên máy tầm trung (Priority: P1)

**Goal**: `QualityManager` đủ chức năng, Hub trong ngân sách.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T022 [P] [US3] Unit test F062-AC-14, F062-AC-15, F062-AC-17 (chọn preset từ số đo giả; device profile ghi đè; cờ lỗi renderer → Compatibility) — file: `src/game/tests/unit/test_quality_manager.gd` — phải FAIL trước khi code
- [ ] T023 [P] [US3] Unit test F062-AC-16 (bộ điều khiển render scale theo `dynamic_resolution`, chặn trong giới hạn preset, không đổi preset) — file: `src/game/tests/unit/test_quality_manager.gd`
- [ ] T024 [P] [US3] Scene test F062-AC-18 (`suspend_world` dừng physics/animation, ảnh chụp mờ; đứng yên > 10 s → 15 FPS) — file: `src/game/tests/scene/test_suspend_world.gd`
- [ ] T025 [US3] Benchmark headless F062-AC-19 (TC-PERF-018) và đo F062-AC-20 (TC-PERF-020) — file: `src/game/tests/perf/MainHubBenchmark.tscn`, `src/game/tests/perf/main_hub_benchmark.gd`

### Implementation cho User Story 3

- [ ] T026 [US3] `QualityManager` (benchmark, preset, device profile, render scale, trần FPS, thermal, renderer fallback, `suspend_world/resume_world`, telemetry) — file: `src/game/scripts/core/quality_manager.gd`, `src/game/scenes/bootstrap/QualityBenchmark.tscn`
- [ ] T027 [US3] Màn chọn chất lượng trong Cài đặt (preset + từng thiết lập) — file: `src/game/scenes/ui/settings/QualitySettings.tscn`, `src/game/scenes/ui/settings/quality_settings.gd`

## Phase 6: User Story 4 — Nhắc nghỉ (Priority: P2)

**Goal**: nhắc nghỉ theo policy, xử lý ngoài giờ.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T028 [P] [US4] Unit test F062-AC-21, F062-AC-22 với đồng hồ giả (đến ngưỡng → signal; đang exam → hoãn) — file: `src/game/tests/unit/test_play_time_service.gd` — phải FAIL trước khi code
- [ ] T034 [P] [US4] Unit test F062-AC-24 với đồng hồ giả và `TelemetryQueue` giả: hộp nhắc hiện → một sự kiện `break_reminder_shown` (`reminder_no` 1, 2…, `minutes_played`), nhắc bị hoãn chưa gửi, mất mạng giữ `client_event_id`, vẫn gửi khi `product_telemetry_enabled = false`, không có trường định danh — file: `src/game/tests/unit/test_break_reminder_telemetry.gd` — phải FAIL trước khi code
- [ ] T029 [P] [US4] Unit test F062-AC-23 (`403 ACCESS_OUTSIDE_ALLOWED_HOURS` → thông điệp + Login) — file: `src/game/tests/unit/test_assignment_service.gd`

### Implementation cho User Story 4

- [ ] T030 [US4] `PlayTimeService` + `BreakReminderDialog` (Mây, không chặn) — file: `src/game/scripts/core/play_time_service.gd`, `src/game/scenes/ui/hud/BreakReminderDialog.tscn`
- [ ] T035 [US4] `PlayTimeService` phát signal `break_reminder_shown` khi hộp nhắc hiện; `TelemetryService` xếp sự kiện `break_reminder_shown` vào lô `postTelemetry` (hàng đợi bền khi offline, không lọc bởi cờ telemetry sản phẩm) — file: `src/game/scripts/core/play_time_service.gd`, `src/game/scripts/networking/telemetry_service.gd`

## Phase 7: Polish & Cross-Cutting

- [ ] T031 [P] Chụp UI Gallery/visual `vi`, `en`, `xx-PS` cho HUD và tab (TC-I18N-006) — file: `src/game/tests/visual/hub_screens.gd`
- [ ] T032 [P] Đề xuất sửa GODOT_ARCHITECTURE §10 (nhắc nghỉ dùng ngưỡng bootstrap ở M-C2) và CHANGELOG — file: `docs/08-game-client/GODOT_ARCHITECTURE.md`, `CHANGELOG.md`
- [ ] T033 Chạy [quickstart.md](quickstart.md) trên backend thật (sau F041 Done) và ghi bằng chứng — file: `specs/062-main-hub-greybox/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → (US2 ∥ US3) → US4 → Polish.
- T004 phải merge trước T018 (integration với backend thật); trong lúc chờ dùng Prism mock (T017).
- F063/F064 đăng ký màn `activity_host`/`exam_lobby` cho intent ở T016.

## Definition of Done

- [ ] Mọi tiêu chí F062-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test`, `make design-tokens-check`, benchmark CI `main_hub` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — profile `mock` không có trong build release
- [ ] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
