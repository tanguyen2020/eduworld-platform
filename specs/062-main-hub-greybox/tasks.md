# Tasks: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Thêm input map `move_forward/back/left/right`, `jump`, `interact`, `open_journal`, `camera_rotate`, `camera_zoom` — file: `src/game/project.godot` — ✅ 10 action (`move_*` WASD + mũi tên, `jump` Space, `interact` E, `open_journal` J, `camera_rotate` chuột phải, `camera_zoom_in/out` con lăn — tách `camera_zoom` thành hai chiều) + autoload `QualityManager` #12 + `application/config/project_settings_override` — `test_player_controller.gd::test_keyboard_actions_and_joystick_feed_the_input`
- [x] T002 [P] Thêm khóa dịch `ui.hub.*`, `ui.quest.title`, `ui.quest.tab.assigned`, `ui.quest.start`, `ui.quest.due`, `ui.quest.overdue`, `ui.quest.empty`, `ui.quality.*`, `ui.common.break_reminder`, `ui.common.outside_access_window`, `ui.error.ASSIGNMENT_NOT_OPEN` — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — ✅ 53 khóa mới vi/en (`ui.hub.*`, `ui.quest.*` gồm `status.*`, `ui.quality.*`, `ui.common.*`, `ui.activity_player.update_required`, `ui.error.ASSIGNMENT_NOT_OPEN`, `ui.settings.*`), bỏ `ui.world.placeholder`; `messages.pot` (195 khóa) và `xx_PS.po` sinh lại; `extract_pot --check` OK
- [x] T003 [P] Sinh fixture từ example OpenAPI `MyAssignment` (rỗng, 2 bài, đủ trạng thái, `content_kind` lạ) — file: `src/game/tests/fixtures/learning/my_assignments_*.json` — ✅ `gen_fixtures.mjs` sinh thêm `openapi/MyAssignment.json`, `openapi/TelemetryBatch.json` và `openapi/listMyAssignments.200.json` (example của response); biến thể rỗng / đủ trạng thái / kind lạ / danh sách dài dựng trong `tests/fakes/assignment_fixtures.gd` từ chính fixture hợp đồng (không chép tay JSON nên không lệch contract) — `gen_fixtures --check` OK (22 file)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: thêm `exam_session_id`, `result_release`, `late_policy` vào `MyAssignment` (chung PR với F064) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — ✅ `MyAssignment` thêm `exam_session_id`, `result_release`, `late_policy`; `listMyAssignments` thêm `example` 200 (Prism MOCK trả 2 bài) và `403`; `learning.v1.yaml` sinh lại bằng `split-public.mjs` — `check-maxlength`, `check-enums`, `validate-schemas`, `check-features`, `check-cross-features`, `check-portal-catalog`, `check-problem-codes`, `split-public --check` OK; `make specs-breaking` không chạy được: oasdiff không parse được spec gốc origin/main (`items: false`, lỗi có sẵn, mọi file đều báo) — delta chỉ thêm trường tùy chọn/response nên không breaking. F064 dùng chung delta này
- [x] T005 Typed model `MyAssignmentModel.from_dict()` bỏ qua trường lạ — file: `src/game/scripts/learning/models/my_assignment.gd` — ✅ `scripts/learning/models/my_assignment.gd` (`MyAssignmentModel.from_dict/list_from`, bỏ trường lạ, kind/status lạ giữ dạng chữ) — `test_assignment_service.gd::test_model_parses_contract_and_ignores_unknown_fields`
- [x] T006 Zone greybox theo cây node chuẩn, `SpawnPoint` `main_hub.default`, `Interactable` `board.school_assignments` (kind `quest_board`) — file: `src/game/scenes/world/zones/main_hub/Zone.tscn`, `src/game/scripts/world/zone_root.gd`, `src/game/scripts/world/spawn_point.gd` — ✅ `Zone.tscn` đủ cây chuẩn WORLD_RENDERING §2 (Environment dùng `env_hub.tres` sinh từ token, Terrain, Static, Navigation, Occluders, SpawnPoints `main_hub.default`/`main_hub.from_arena`, Interactables `board.school_assignments` kind `quest_board`, Portals, Players, Ambient) + `zone_root.gd`, `spawn_point.gd` — `test_main_hub_zone.gd::test_zone_has_the_standard_tree_and_the_assignments_board`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Vào Main Hub và điều khiển nhân vật (Priority: P1) 🎯 MVP

**Goal**: học sinh đi lại trong Hub, tương tác với điểm neo.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Scene test F062-AC-01 (spawn theo id, fallback `main_hub.default`) — file: `src/game/tests/scene/test_main_hub_zone.gd` — phải FAIL trước khi code — ✅ `tests/scene/world/test_main_hub_zone.gd` (5 test spawn/fallback/cảnh báo/về Quảng trường) — pass. Ghi chú: test được viết song song với code (không có lần chạy FAIL riêng)
- [x] T008 [P] [US1] Unit test F062-AC-02, F062-AC-03 (vận tốc → trạng thái animation; tap-to-move chỉ bật ở `g1_3`) — file: `src/game/tests/unit/test_player_controller.gd` — ✅ `tests/unit/player/test_player_controller.gd` (6 test) — pass
- [x] T009 [P] [US1] Scene test F062-AC-04 (vào vùng → nút hành động, phát `interacted(interactable_id)`, không gọi API) — file: `src/game/tests/scene/test_interactable.gd` — ✅ `tests/scene/world/test_interactable.gd` (3 test: vào vùng → focus + prompt, `interacted(id)` → intent `quest_journal {tab: assigned}`, script không gọi service/mạng) — pass
- [x] T010 [P] [US1] Scene test F062-AC-05, F062-AC-06 (biến thể Theme theo grade band, đổi locale cập nhật HUD) + chạy `lint_theme_overrides.gd` — file: `src/game/tests/scene/test_main_hud.gd` — ✅ `tests/scene/hud/test_main_hud.gd` (5 test: vùng chạm ≥ `DENSITY_BUTTON_MIN_HEIGHT` mọi band, `g1_3` không minimap, slot tương lai ẩn, đổi locale cập nhật ngay, rule `lint_theme_overrides` + khóa dịch trên 13 scene F062) — pass; `lint_theme_overrides: OK (136 files)`

### Implementation cho User Story 1

- [x] T011 [P] [US1] `PlayerController` (CharacterBody3D), `CameraRig`, `AnimDriver` (AnimationTree state machine), `TapToMove` — file: `src/game/scripts/player/player_controller.gd`, `src/game/scripts/player/camera_rig.gd`, `src/game/scripts/player/anim_driver.gd`, `src/game/scripts/player/tap_to_move.gd`, `src/game/scenes/character/Player.tscn` — ✅ `player_controller.gd`, `camera_rig.gd` (SpringArm, chuột phải/vuốt, zoom), `anim_driver.gd` (một AnimationTree state machine idle/walk/run/jump dựng bằng code; clip bob PLACEHOLDER R17 tới F070), `tap_to_move.gd` (chỉ `g1_3`), `Player.tscn`
- [x] T012 [P] [US1] `Interactable` (Area3D, `interactable_id`, `kind`, `prompt_key`, signal `interacted`) — file: `src/game/scripts/world/interactable.gd` — ✅ `scripts/world/interactable.gd` (Area3D, `interacted`/`player_entered`/`player_exited`, nhóm `local_player`)
- [x] T013 [US1] `MainHud` tối thiểu (PlayerBadge, chip bài giao, Cài đặt, Về Quảng trường, slot ẩn), `ContextActionButton`, `Joystick` — file: `src/game/scenes/ui/hud/MainHud.tscn`, `src/game/scenes/ui/hud/main_hud.gd` — ✅ `MainHud.tscn`/`main_hud.gd` (PlayerBadge nickname + grade band, chip "Nhà trường giao (n)", Nhiệm vụ (J), Cài đặt, Về Quảng trường, banner hệ thống; slot XpBar/CurrencyChip/QuestTracker/Minimap/ChatDock/PartyChip ẩn), `ContextActionButton.tscn`, `Joystick.tscn` (chỉ màn cảm ứng); thêm `WorldRoot.tscn` thay `WorldPlaceholder` (đã xóa) — Đăng xuất chuyển vào Cài đặt → Tài khoản, cảnh báo giờ giới nghiêm thành banner HUD

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Xem và mở bài nhà trường giao (Priority: P1)

**Goal**: tab "Nhà trường giao" đọc API thật/mock, mở bài qua intent.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T014 [P] [US2] Unit test F062-AC-07, F062-AC-12 với `FakeApiClient` (gắn `X-Tenant-Id`, 401 → refresh 1 lần, 403 → `session_expired`, 5xx → giữ dữ liệu cũ) — file: `src/game/tests/unit/test_assignment_service.gd` — phải FAIL trước khi code — ✅ `tests/unit/learning/test_assignment_service.gd` (10 test: `X-Tenant-Id` + Bearer, phân trang cursor, single-flight, 401 → refresh 1 lần rồi gửi lại, 401 lần 2 không lặp, `TENANT_MISMATCH` → Login + xóa phiên, `GAME_ACCESS_STUDENT_ONLY` → NotStudent + `session_expired`, mạng/503 giữ danh sách cũ) — pass
- [x] T015 [P] [US2] Scene test F062-AC-08, F062-AC-11 (cột hiển thị, định dạng hạn theo locale, không có điểm/tên người khác, danh sách ảo hóa) — file: `src/game/tests/scene/test_quest_journal_assigned.gd` — ✅ `tests/scene/quest/test_quest_journal_assigned.gd` (hàng: tên nguyên văn không auto-translate, icon + chữ trạng thái, hạn theo `LocaleFormat`, đổi locale; ảo hóa 100 bài ≤ 12 node; không điểm/tên bạn/nhận xét) — pass
- [x] T016 [P] [US2] Scene test F062-AC-09, F062-AC-10 (intent `activity_host`/`exam_lobby` đúng tham số; `content_kind` lạ → `update_required` + telemetry) — file: `src/game/tests/scene/test_quest_journal_assigned.gd` — ✅ cùng file: intent `activity_host {activity_id, mode, context_type, context_id}`, `exam_lobby {exam_session_id, assignment_id}`; `quest`/`lesson`/kind lạ → `ui.activity_player.update_required` + 3 telemetry `client_error`/`client_unsupported_assignment_kind`; trạng thái lạ → "Chưa hỗ trợ" — pass
- [x] T017 [P] [US2] Test F062-AC-13 (profile `mock` hiện nhãn MOCK; export preset release không chứa `config/env/mock.cfg`) — file: `src/game/tests/unit/test_config_mock_label.gd` — ✅ `tests/unit/core/test_config_mock_label.gd` (nhãn MOCK layer 128 trên HUD layer 2 ở profile `mock`; mọi preset export loại `config/env/mock.json`, `tests/*`, `tools/*`, `scenes/ui/debug/*`) — pass; `godot-export-check`: `scan_artifact: OK (1 artifact(s), 13 patterns)`
- [x] T018 [US2] Integration headless với backend compose: `hs.an.3a` chỉ thấy bài 3A, `hs.dung.7a` không thấy bài th-demo-a (tenant isolation, TC-LMS-002) — file: `src/game/tests/integration/test_my_assignments_isolation.gd` — ⏳ **đã chạy với backend thật 2026-11-24, chưa pass**: test đổi theo ADR-0095 (`test_students_only_see_their_own_school_grade`: DEMOA1 khối 3 vs DEMOB1 khối 7, kiểm `X-Tenant-Id`, ánh xạ intent theo kind, ghi trace id); lượt 06:32 đăng nhập/phiên/`listMyAssignments` 200 cho cả hai học sinh nhưng **0 bài** mỗi bên → fail ở `is_not_empty` (stack local chưa có bài giao; phạm vi chỉ đọc/chơi nên không tạo bài). Cần giao ≥ 1 bài cho DEMOA1 khối 3 rồi chạy lại lệnh ở quickstart.md §6. 2026-11-24 (lượt 2): seed DEV bài giao `seed/assignments/demoa1_g3.dev.json` + `devseed.RunAssignments` (unit test idempotent pass) đã thêm vào `make seed-content`; **chưa nạp vào stack của owner** — lệnh ghi vào stack bị hệ thống phân quyền chặn, cần owner chạy `EW_SEED_PASSWORD=… make seed-content` rồi chạy lại T018 — ✅ pass 2026-11-24 17:10 ICT với backend thật sau khi nạp seed bài giao (`seed-content`: 2 bài DEMOA1 khối 3): `hs.an.3a` 2 bài (`activity` 1, `question_set` 1, đủ `subject_name`, 2 intent; trace `06eedef3b5aac210ef03da4d3f0bf21e`), `hs.dung.7a` 0 bài (trace `74aa4fe6a5e3f964457c5d5aa76d6aac`) — 1/1 test case, 0 failures (quickstart.md §6.2)

### Implementation cho User Story 2

- [x] T019 [US2] `AssignmentService` (`refresh()`, `start(assignment)`, signal `assignments_updated`) — file: `src/game/scripts/learning/assignment_service.gd` — ✅ `scripts/learning/assignment_service.gd` (`refresh()` single-flight + phân trang ≤ 10 trang, `start()`, signal `assignments_updated`/`state_changed`/`access_denied`) + `ServiceLocator` trong `AppState.services` (GODOT_ARCHITECTURE §4) + `AuthService.handle_access_denied()`
- [x] T020 [US2] `QuestJournal` (tab `assigned`), `AssignmentRow`, mở từ phím J/nút 📜/bảng Hub — file: `src/game/scenes/quest/QuestJournal.tscn`, `src/game/scenes/quest/quest_journal.gd`, `src/game/scenes/quest/AssignmentRow.tscn` — ✅ `QuestJournal.tscn`/`quest_journal.gd` (tab "Nhà trường giao", loading/rỗng/lỗi + Thử lại, ảo hóa ≤ 12 hàng), `AssignmentRow.tscn`; mở bằng phím J, nút "Nhiệm vụ (J)", chip HUD và bảng trong Hub (`ui_requested("quest_journal")`)
- [x] T021 [US2] Đăng ký làm mới khi vào/quay lại Hub (`SceneRouter.zone_entered`, `EventBus.ui_closed`) — file: `src/game/scripts/world/zone_root.gd` — ✅ `zone_root.gd` làm mới khi vào zone (`_ready`) và khi `EventBus.ui_closed` (signal toàn cục mới, thay cho `SceneRouter.zone_entered` chưa có) — `test_main_hub_zone.gd::test_entering_and_returning_to_the_hub_refreshes_assignments`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Chạy mượt trên máy tầm trung (Priority: P1)

**Goal**: `QualityManager` đủ chức năng, Hub trong ngân sách.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T022 [P] [US3] Unit test F062-AC-14, F062-AC-15, F062-AC-17 (chọn preset từ số đo giả; device profile ghi đè; cờ lỗi renderer → Compatibility) — file: `src/game/tests/unit/test_quality_manager.gd` — phải FAIL trước khi code — ✅ `tests/unit/core/test_quality_manager.gd` (benchmark lần đầu ≤ 5 s lưu preset, ánh xạ p10 → preset, bỏ qua chọn tay, device profile ép preset + renderer, profile không khớp/regex sai/`os_version_max`, cờ boot renderer → Compatibility + telemetry, engine tự fallback, boot khỏe không đổi) — pass
- [x] T023 [P] [US3] Unit test F062-AC-16 (bộ điều khiển render scale theo `dynamic_resolution`, chặn trong giới hạn preset, không đổi preset) — file: `src/game/tests/unit/test_quality_manager.gd` — ✅ cùng file: bộ điều khiển render scale giảm/tăng trong khoảng preset, chờ 10 s mới tăng, ở mức tối thiểu 30 s chỉ phát `preset_suggested` (preset không đổi), UI không bị scale, trần FPS nhiệt/idle — pass
- [x] T024 [P] [US3] Scene test F062-AC-18 (`suspend_world` dừng physics/animation, ảnh chụp mờ; đứng yên > 10 s → 15 FPS) — file: `src/game/tests/scene/test_suspend_world.gd` — ✅ `tests/scene/world/test_suspend_world.gd` (2 test: `suspend_world` tắt process/physics của zone, `disable_3d`, lớp ảnh chụp mờ dưới HUD, nhiều lý do; đứng yên > 10 s → 15 FPS, có input → trả trần cũ) — pass
- [x] T025 [US3] Benchmark headless F062-AC-19 (TC-PERF-018) và đo F062-AC-20 (TC-PERF-020) — file: `src/game/tests/perf/MainHubBenchmark.tscn`, `src/game/tests/perf/main_hub_benchmark.gd` — ✅ CI headless: `tests/perf/MainHubBenchmark.tscn` + `main_hub_benchmark.gd` + `test_main_hub_benchmark.gd` — Hub greybox 8 496 tam giác / 8 draw call (proxy) / 1 vật thể động ≤ `scene_budget_hub` Mobile (150k/100/60, +5 %); HUD 24 Control, 0,03 ms/lần cập nhật ≤ `ui_cpu_budget`; nạp Hub 27 ms. ⏳ Đo FPS p10 và khởi động lạnh → Hub trên DEV-PC-MID/DEV-AND-MID1 (F062-AC-20, SC-002; từ 2026-11-24 DEV-WEB-MID thay DEV-PC-MID — ADR-0101) chưa làm: cần thiết bị lab (chạy `MainHubBenchmark.tscn` với renderer)

### Implementation cho User Story 3

- [x] T026 [US3] `QualityManager` (benchmark, preset, device profile, render scale, trần FPS, thermal, renderer fallback, `suspend_world/resume_world`, telemetry) — file: `src/game/scripts/core/quality_manager.gd`, `src/game/scenes/bootstrap/QualityBenchmark.tscn` — ✅ `scripts/core/quality_manager.gd` (autoload #12) + `quality_policy.gd` + `dynamic_resolution.gd` + `QualityBenchmark.tscn`/`quality_benchmark.gd`; telemetry `perf_sample` mỗi 60 s, `client_error renderer_fallback`. Nguồn nhiệt Android (plugin `PowerManager`) chưa có: API `set_thermal_throttled()` sẵn, nối plugin ở F133
- [x] T027 [US3] Màn chọn chất lượng trong Cài đặt (preset + từng thiết lập) — file: `src/game/scenes/ui/settings/QualitySettings.tscn`, `src/game/scenes/ui/settings/quality_settings.gd` — ✅ `scenes/ui/settings/QualitySettings.tscn` (preset Thấp/Trung bình/Cao, Bóng đổ, Khử răng cưa, gợi ý hạ preset) trong `SettingsPanel.tscn` (ngôn ngữ vi/en, Đồ họa, Tài khoản → Đăng xuất)

## Phase 6: User Story 4 — Nhắc nghỉ (Priority: P2)

**Goal**: nhắc nghỉ theo policy, xử lý ngoài giờ.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T028 [P] [US4] Unit test F062-AC-21, F062-AC-22 với đồng hồ giả (đến ngưỡng → signal; đang exam → hoãn) — file: `src/game/tests/unit/test_play_time_service.gd` — phải FAIL trước khi code — ✅ `tests/unit/core/test_play_time_service.gd` (7 test: đúng ngưỡng server, lặp mỗi khoảng, không policy → không nhắc, hoãn khi thi/hộp thoại và hiện khi nhả hết, trần 20 lần, phiên mới đếm lại) — pass
- [x] T034 [P] [US4] Unit test F062-AC-24 với đồng hồ giả và `TelemetryQueue` giả: hộp nhắc hiện → một sự kiện `break_reminder_shown` (`reminder_no` 1, 2…, `minutes_played`), nhắc bị hoãn chưa gửi, mất mạng giữ `client_event_id`, vẫn gửi khi `product_telemetry_enabled = false`, không có trường định danh — file: `src/game/tests/unit/test_break_reminder_telemetry.gd` — phải FAIL trước khi code — ✅ `tests/unit/networking/test_break_reminder_telemetry.gd` (7 test: lô `postTelemetry` có `reminder_no` 1, 2 + `minutes_played`, `client_event_id` UUID mới, nhắc hoãn chưa gửi, mất mạng giữ và gửi lại cùng id, vẫn gửi khi tắt telemetry sản phẩm, không trường định danh, giới hạn 30 s/lô, hết phiên xóa hàng đợi) — pass
- [x] T029 [P] [US4] Unit test F062-AC-23 (`403 ACCESS_OUTSIDE_ALLOWED_HOURS` → thông điệp + Login) — file: `src/game/tests/unit/test_assignment_service.gd` — ✅ `test_assignment_service.gd::test_outside_access_window_returns_to_login_with_the_window` (`403 ACCESS_OUTSIDE_ALLOWED_HOURS` → Login, banner `ui.common.outside_access_window` kèm `access_window` của bootstrap, xóa phiên) — pass

### Implementation cho User Story 4

- [x] T030 [US4] `PlayTimeService` + `BreakReminderDialog` (Mây, không chặn) — file: `src/game/scripts/core/play_time_service.gd`, `src/game/scenes/ui/hud/BreakReminderDialog.tscn` — ✅ `scripts/core/play_time_service.gd` + `BreakReminderDialog.tscn` (Mây, không modal, đóng một chạm) — `test_world_root.gd::test_break_reminder_dialog_is_gentle`
- [x] T035 [US4] `PlayTimeService` phát signal `break_reminder_shown` khi hộp nhắc hiện; `TelemetryService` xếp sự kiện `break_reminder_shown` vào lô `postTelemetry` (hàng đợi bền khi offline, không lọc bởi cờ telemetry sản phẩm) — file: `src/game/scripts/core/play_time_service.gd`, `src/game/scripts/networking/telemetry_service.gd` — ✅ `PlayTimeService.break_reminder_shown` → `ServiceLocator` → `TelemetryService.queue_break_reminder()`; `scripts/networking/telemetry_service.gd` (hàng đợi RAM ≤ 300, lô ≤ 100, `flush()` mỗi ≥ 30 s theo MONITORING §10, xóa chỉ sau 2xx, lọc `screen_view`/`tutorial_step` khi tắt telemetry sản phẩm, chuyển hàng đợi cũ `AppState.pending_telemetry` thành `client_error`); `WorldRoot` flush định kỳ

## Phase 7: Polish & Cross-Cutting

- [x] T031 [P] Chụp UI Gallery/visual `vi`, `en`, `xx-PS` cho HUD và tab (TC-I18N-006) — file: `src/game/tests/visual/hub_screens.gd` — ✅ `tests/visual/hub_screens.gd` chụp 12 ảnh (HUD + tab Nhà trường giao × `g1_3`/`g6_7` × vi/en/xx-PS) với renderer `gl_compatibility` trên Windows; đã xem và sửa tràn chữ (PlayerBadge, nút HUD) — ảnh là artefact review, không commit baseline (3D phụ thuộc GPU)
- [x] T032 [P] Đề xuất sửa GODOT_ARCHITECTURE §10 (nhắc nghỉ dùng ngưỡng bootstrap ở M-C2) và CHANGELOG — file: `docs/08-game-client/GODOT_ARCHITECTURE.md`, `CHANGELOG.md` — ✅ GODOT_ARCHITECTURE §10 đã mô tả nhắc nghỉ theo ngưỡng bootstrap (không cần đề xuất thêm); §4 cập nhật danh sách `EventBus` (`ui_requested`, `ui_closed`); CHANGELOG
- [x] T033 Chạy [quickstart.md](quickstart.md) trên backend thật (sau F041 Done) và ghi bằng chứng — file: `specs/062-main-hub-greybox/quickstart.md` — ⏳ **một phần (2026-11-24)**: quickstart.md §6 — đường đăng nhập → phiên → profile → `listMyAssignments` trên backend thật OK (trace id ghi lại); bước cần bài giao (3–7) không có dữ liệu, bước cần màn hình/thiết bị (2, 8–10, 12) chưa chạy (agent headless). Phát hiện: `config/env/local.json` `client_version = 0.0.0-dev` < `EW_GAME_MIN_CLIENT_VERSION` (0.1.0) nên client `--env=local` bị `426` — đã sửa thành `0.1.0` (owner chốt 2026-11-24); boot headless `--env=local` → `getClientConfig` 200, không ForceUpdate — ✅ 2026-11-24: bước chạy được headless xong (§6, §6.2); bước giao diện 2, 8, 9, 10 → owner kiểm tay (cửa sổ game `--env=local` đã mở cho owner 2026-11-24); bước 12 (AC-20/SC-002) → F133 (đã thống nhất)

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Đã làm 2026-11-24 (Clarifications "Session 2026-11-24").

- [x] T036 **OQ chốt 2026-11-20** (F062-1) Hàng bài giao hiện `subject_name`; test scene với fixture có/không có field — file: `src/game/scripts/learning/`, `src/game/tests/scene/test_quest_journal_assigned.gd` — kiểm: `make godot-test` — cỡ S — ✅ `MyAssignmentModel.subject_name` + `subject_text()` (`L10n.pick`, bỏ giá trị sai kiểu, cắt 255), nhãn `Subject` trong `AssignmentRow.tscn` (ẩn khi không có); fixture `AssignmentFixtures.subjects()` (vi+en / không có field / chỉ vi) — `test_quest_journal_assigned.gd::test_row_shows_the_subject_name_only_when_the_server_sends_it`, `test_assignment_service.gd::test_model_reads_subject_name_and_ignores_a_malformed_one` — pass
- [x] T037 **OQ chốt 2026-11-20** (F062-2) Ánh xạ `question_set` → intent `activity_host {question_set_id, mode: "assignment", context_type, context_id}` thay `update_required`; test scene — file: `src/game/scripts/learning/`, `src/game/tests/scene/test_quest_journal_assigned.gd` — kiểm: `make godot-test` — cỡ S — ✅ `AssignmentService.start()` nhánh `KIND_QUESTION_SET`; `test_quest_journal_assigned.gd::test_question_set_opens_the_activity_host_with_the_set_id` (intent đúng tham số, không notice, không telemetry, `LearningService.start_request` → `startAdHocAttempt` `source.kind = question_set`) — pass

## Nhận từ ADR-0095 (owner chốt 2026-11-21)

- [x] T038 Đổi chuỗi grade band "Lớp 1–3 / 4–5 / 6–9" trong `src/game/locale/*.po` (HUD, F062) thành "Khối 1–3 / 4–5 / 6–9"; `extract_pot --check`, `pseudo_locale --check`, GdUnit4 — cỡ S — ✅ `vi.po`: `ui.hub.grade_band.g1_3|g4_5|g6_7|g8_9` và `ui.gallery.density.*` → "Khối 1–3/4–5/6–7/8–9" (band thật là 4 nhóm, Clarifications 2026-11-24); `en.po`/`xx_PS.po` không đổi — `extract_pot: OK (334 files)`, `pseudo_locale: OK`

## Dependencies & Execution Order

- Setup → Foundational → US1 → (US2 ∥ US3) → US4 → Polish.
- T004 phải merge trước T018 (integration với backend thật); trong lúc chờ dùng Prism mock (T017).
- F063/F064 đăng ký màn `activity_host`/`exam_lobby` cho intent ở T016.

## Definition of Done

- [x] Mọi tiêu chí F062-AC-* có test pass (ghi mã test/TC) — ✅ 24/24 AC có test GdUnit4 pass (quickstart.md §5, §6); T018 (TC-LMS-002) pass với backend thật 2026-11-24 (§6.2); AC-20/SC-002 đo thiết bị hoãn sang F133
- [x] `make godot-test`, `make design-tokens-check`, benchmark CI `main_hub` xanh — GdUnit4 313 test (308 pass, 5 skip integration thiếu backend), `scan_artifact_test` 20/20, design 56/56 + `design-tokens-check: OK`, `test_main_hub_benchmark.gd` 3/3 (lệnh chạy trực tiếp bằng Godot 4.7.2 console, Windows không có make; gdlint chạy ở CI)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — profile `mock` không có trong build release — `godot-export-check` + `test_config_mock_label.gd`; PLACEHOLDER có nhãn: clip animation avatar (tới F070), Hub greybox (art F071)
- [x] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi — GODOT_ARCHITECTURE §4, CHANGELOG, spec Clarifications 2026-11-20
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — `Done` (2026-11-24: T018 pass với backend thật; bước giao diện của quickstart do owner kiểm tay; AC-20/SC-002 → F133)
