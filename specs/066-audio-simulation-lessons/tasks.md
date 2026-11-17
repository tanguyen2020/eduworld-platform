# Tasks: F066 — Bài nghe và mô phỏng: plugin `audio_lesson`, `simulation` + checker Go, template Godot

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 [P] Fixture nội dung bài nghe (hợp lệ, đoạn chồng, thiếu transcript, câu không tự chấm) và mô phỏng (hợp lệ, tham số ngoài khoảng, trường `script`) — file: `src/backend/modules/learning/tests/fixtures/{audiolesson,simulation}/*.json`
- [ ] T002 [P] Khóa dịch `ui.audio.*` (`plays_left`, `plays_exhausted`, `not_ready`, `speed`, `transcript`) và `ui.simulation.*` (`loading`, `goal_reached`, `log_invalid`, `runs_exhausted`, `reset_camera`) — file: `src/game/locale/vi.po`, `src/game/locale/en.po`
- [ ] T003 [P] Fixture client (`Attempt` bài nghe/mô phỏng, `SimulationRunResult` các trạng thái, file `.ogg` ngắn) — file: `src/game/tests/fixtures/{audio,simulation}/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta OpenAPI: `ActivityProgressReport.play_started`, `ActivityProgressState.plays_used/max_plays`, ProblemCode `AUDIO_PLAY_LIMIT_REACHED`, `SIMULATION_RUN_LIMIT_REACHED`; `submitSimulationRun` thêm `401/403/404/428`; `SimulationRunResult` (`verification_status`, `goals_met[]`, `runs_used`, `max_runs`); `SimulationTemplate` (`simulation_kind` đúng enum, `actions`, `metrics`, `checker_available`, `status`) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` (+ enum ProblemCode ở `specs/openapi/modules/*.v1.yaml`) — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T005 [P] `$defs.render` đóng chặt cho `audio-lesson.schema.json` và `simulation.schema.json` (không `check`) — file: `specs/schemas/activities/audio-lesson.schema.json`, `specs/schemas/activities/simulation.schema.json` — kiểm: `make specs-lint`
- [ ] T006 [P] Event payload `simulation.run_verified.v1`, `simulation.run_rejected.v1` — file: `specs/events/payloads/learning/simulation.run_verified.v1.schema.json`, `specs/events/payloads/learning/simulation.run_rejected.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T007 [P] Vector chung `physics.lever` v1 (≥ 20 vector) và example activity — file: `specs/examples/simulation/physics.lever/v1/vectors.json`, `specs/examples/activity.audio-lesson.english-g3-my-name.json`, `specs/examples/activity.simulation.physics-lever.json`
- [ ] T008 ent schema `SimulationTemplate` (PK kép, `MaxLen`, cột mới `title`, `actions`, `metrics`, `min_client_version`), `SimulationRun` (mixin tenant + soft-delete, CHECK ≤ 64 KB, partial unique `idempotency_key`, `run_no`), thêm `plays_used` vào `VideoProgress` — file: `src/backend/modules/learning/ent/schema/simulation_template.go`, `simulation_run.go`, `video_progress.go`
- [ ] T009 Migration Atlas + RLS cho `assessment.simulation_runs` — file: `src/backend/modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=audio_simulation`, `make migrate-lint MOD=learning`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Server phục vụ và chấm bài nghe (Priority: P1) 🎯 MVP

**Goal**: plugin `audio_lesson` server-authoritative.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Unit test domain F066-AC-01 (`audiolesson.Config.Validate`: đoạn, `at_ms`, câu tự chấm, transcript khi publish) — file: `src/backend/modules/learning/tests/unit/audiolesson/config_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Unit test F066-AC-04 (`Plays.Accept`: tăng một lần theo `client_seq`, chặn khi đủ `max_plays`, không giới hạn khi null) — file: `src/backend/modules/learning/tests/unit/audiolesson/plays_test.go`
- [ ] T012 [P] [US1] Test hợp đồng plugin chung F066-AC-03 cho `audio_lesson` (payload không đáp án, `Evaluate` idempotent, evidence đúng schema — TC-ACT-001) — file: `src/backend/modules/learning/tests/contract/plugins/audiolesson_contract_test.go`
- [ ] T013 [P] [US1] Integration F066-AC-02, F066-AC-03, F066-AC-07 (ghim version + item, signed URL, `MEDIA_NOT_READY`, resume giữ `plays_used`/câu trả lời — TC-ACT-005, TC-LRN-009) — file: `src/backend/modules/learning/tests/integration/audiolesson/start_resume_test.go`
- [ ] T014 [P] [US1] Integration F066-AC-04 (`AUDIO_PLAY_LIMIT_REACHED` không đổi state, cắt tiến độ bất hợp lý + integrity signal — TC-AUD-001) — file: `src/backend/modules/learning/tests/integration/audiolesson/progress_test.go`
- [ ] T015 [P] [US1] Integration F066-AC-05, F066-AC-06 (chấm nghe chép chuẩn hóa, `question.answered`, trả lời sau hết lượt; submit idempotent + audit + outbox — TC-LRN-003, TC-LRN-008) — file: `src/backend/modules/learning/tests/integration/audiolesson/respond_submit_test.go`
- [ ] T016 [P] [US1] Test bảo mật F066-AC-08 (quét OpenAPI/AsyncAPI không có route ghi âm; `speaking` → `ACTIVITY_TYPE_NOT_SUPPORTED`; `listActivityTypes` không có `speaking` — TC-AUD-004) — file: `src/backend/modules/learning/tests/integration/audiolesson/no_voice_test.go`

### Implementation cho User Story 1

- [ ] T017 [P] [US1] Domain `config.go`, `plays.go`, `completion.go` (dùng `ranges`/`plausibility` của F065) — file: `src/backend/modules/learning/internal/domain/activity/audiolesson/`
- [ ] T018 [US1] Plugin `audiolesson.Plugin` (ValidateConfig, BuildRenderPayload, Respond, Progress, Evaluate, CompletionRule, EmitEvidence) — file: `src/backend/modules/learning/internal/application/activityplugin/audiolesson/plugin.go`
- [ ] T019 [US1] Repository `video_progress` cập nhật `plays_used` trong `SELECT … FOR UPDATE` — file: `src/backend/modules/learning/internal/infrastructure/persistence/video_progress_repo.go`
- [ ] T020 [US1] Đăng ký plugin, `listActivityTypes` trả `audio_lesson` (`evaluation_strategies = [completion, auto]`, `min_client_version`) — file: `src/backend/modules/learning/cmd/api/main.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được (qua API).

## Phase 4: User Story 2 — Server kiểm chứng mô phỏng bằng checker Go (Priority: P1)

**Goal**: plugin `simulation`, khung checker, template `physics.lever` v1 phía server.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T021 [P] [US2] Unit test vector chung F066-AC-17 (`lever.Checker` khớp `vectors.json`; benchmark p95 — TC-SIM-003) — file: `src/backend/modules/learning/tests/unit/simulation/lever_vectors_test.go` — phải FAIL trước khi code
- [ ] T022 [P] [US2] Unit test F066-AC-13 (`ValidateLog`: 501 hành động, > 64 KB, `t_ms` giảm, action lạ, đối số ngoài phạm vi) và `EvaluateGoals` — file: `src/backend/modules/learning/tests/unit/simulation/actionlog_goals_test.go`
- [ ] T023 [P] [US2] Unit test F066-AC-09 (`simulation.Config.Validate` theo `parameter_schema`, metric ∈ template, template `retired`) — file: `src/backend/modules/learning/tests/unit/simulation/config_test.go`
- [ ] T024 [P] [US2] Test hợp đồng plugin F066-AC-10 (payload không `check`, không đáp án — TC-ACT-001) — file: `src/backend/modules/learning/tests/contract/plugins/simulation_contract_test.go`
- [ ] T025 [P] [US2] Integration F066-AC-11, F066-AC-12 (run verified, `simulation.goal_met`, outbox `simulation.run_verified`; `client_outcome` lệch → integrity signal — TC-SIM-001, TC-SIM-002) — file: `src/backend/modules/learning/tests/integration/simulation/run_verify_test.go`
- [ ] T026 [P] [US2] Integration F066-AC-13, F066-AC-14 (`SIMULATION_LOG_INVALID` lưu `rejected` + `simulation.run_rejected`; seed/tham số lệch; `SIMULATION_RUN_LIMIT_REACHED`; cùng key → cùng run — TC-LRN-008) — file: `src/backend/modules/learning/tests/integration/simulation/run_reject_limit_test.go`
- [ ] T027 [P] [US2] Integration F066-AC-15, F066-AC-16 (template quan sát `not_verifiable`, không evidence; submit điểm theo run verified cuối + câu `simulation_question`, `goals_met` completion) — file: `src/backend/modules/learning/tests/integration/simulation/submit_test.go`
- [ ] T028 [P] [US2] Integration CMS F066-AC-09 (`updateCmsActivityDraft` tham số sai/trường `script` → `ACTIVITY_CONFIG_INVALID` — TC-SIM-004); `listSimulationTemplates` trả descriptor đã sync — file: `src/backend/modules/learning/tests/integration/simulation/cms_validate_test.go`

### Implementation cho User Story 2

- [ ] T029 [P] [US2] Domain `checker.go`, `actionlog.go`, `goals.go`, `seed.go`, `config.go` — file: `src/backend/modules/learning/internal/domain/activity/simulation/`
- [ ] T030 [P] [US2] Descriptor + checker `physics.lever` v1 (mô hình mô-men, hành động `place_mass`/`remove_mass`/`release`) — file: `src/backend/modules/learning/internal/domain/activity/simulation/templates/physics.lever/v1/descriptor.json`, `checker.go`
- [ ] T031 [US2] Port + repository `SimulationRunRepository`, `SimulationTemplateRepository` (TenantScope cho run) — file: `src/backend/modules/learning/internal/application/port/`, `src/backend/modules/learning/internal/infrastructure/persistence/simulation_run_repo.go`, `simulation_template_repo.go`
- [ ] T032 [US2] Plugin `simulation.Plugin` + command `SubmitSimulationRun` (transaction: run + integrity signal + outbox + learning event) — file: `src/backend/modules/learning/internal/application/activityplugin/simulation/plugin.go`, `src/backend/modules/learning/internal/application/command/submit_simulation_run.go`
- [ ] T033 [US2] Handler `submitSimulationRun` (`game_player` + owner, Idempotency-Key) và job `simtemplates.Sync`; đăng ký plugin + `CheckerRegistry`, readiness khi thiếu checker — file: `src/backend/modules/learning/internal/presentation/http/public/simulation_run_handler.go`, `src/backend/modules/learning/internal/presentation/job/simtemplates_sync.go`, `src/backend/modules/learning/cmd/api/main.go`
- [ ] T034 [US2] Metric/span/log (không log action log) — file: `src/backend/modules/learning/internal/application/command/submit_simulation_run.go`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được (qua API).

## Phase 5: User Story 3 — Học sinh nghe bài và làm mô phỏng trong game (Priority: P1)

**Goal**: renderer `audio_lesson`, `simulation`, template Godot `physics.lever` v1.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T035 [P] [US3] Scene test F066-AC-18, F066-AC-19 (bộ đếm lượt, `play_started` trước khi phát, nút mờ khi hết lượt, tốc độ, transcript theo cấu hình) — file: `src/game/tests/scene/audio/test_audio_lesson_player.gd` — phải FAIL trước khi code
- [ ] T036 [P] [US3] Test F066-AC-20 (quét `project.godot`/export preset không có quyền micro, không node `AudioEffectRecord` trong `scenes/learning` — TC-AUD-004) — file: `src/game/tests/unit/audio/test_no_recording.gd`
- [ ] T037 [P] [US3] GdUnit4 vector chung F066-AC-24 (`lever_model.gd` khớp `vectors.json` — TC-SIM-003) — file: `src/game/tests/unit/simulation/test_lever_vectors.gd`
- [ ] T038 [P] [US3] Unit test F066-AC-22 (`SimulationActionLog`: `t_ms` tương đối, giới hạn, giữ `Idempotency-Key` khi retry qua `PendingWrites`) — file: `src/game/tests/unit/simulation/test_action_log.gd`
- [ ] T039 [P] [US3] Scene test F066-AC-21, F066-AC-23 (nạp template, kiểm `sha256`, thiếu template → update_required; "Đạt mục tiêu" chỉ sau kết quả server; `log_invalid`, `runs_exhausted` — TC-ACT-004) — file: `src/game/tests/scene/simulation/test_simulation_host.gd`
- [ ] T040 [US3] Integration headless với compose: `hs.an.3a` làm bài nghe và mô phỏng tới hoàn thành (TC-AUD-001, TC-SIM-001) — file: `src/game/tests/integration/test_audio_simulation_flow.gd`

### Implementation cho User Story 3

- [ ] T041 [P] [US3] `AudioLessonPlayer` + `AudioLessonRenderer` (kế thừa `ActivityRendererBase`, đăng ký registry) + `audio_play_tracker.gd` — file: `src/game/scenes/learning/audio/AudioLessonPlayer.tscn`, `src/game/scenes/learning/activity_renderers/audio_lesson/AudioLessonRenderer.tscn`, `src/game/scripts/learning/audio_play_tracker.gd`
- [ ] T042 [P] [US3] Template `physics.lever` v1 (bản đầy đủ + `lever_lite.tscn`, `lever_model.gd` cùng công thức checker) — file: `src/game/scenes/learning/simulation/templates/physics_lever/v1/`
- [ ] T043 [US3] `SimulationHost`, `SimulationRenderer`, `simulation_action_log.gd`, `simulation_template_loader.gd` (build hoặc PCK theo manifest, `suspend_world("simulation")`) — file: `src/game/scenes/learning/simulation/SimulationHost.tscn`, `src/game/scenes/learning/activity_renderers/simulation/SimulationRenderer.tscn`, `src/game/scripts/learning/simulation_action_log.gd`, `src/game/scripts/learning/simulation_template_loader.gd`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Editor bài nghe và mô phỏng (Priority: P2)

**Goal**: editor `audio_lesson`, `simulation` trong Activity Builder (F058).

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T044 [P] [US4] Test F066-AC-25 (chọn audio/transcript, đoạn, câu theo mốc chỉ loại tự chấm, ajv theo schema, lỗi `ACTIVITY_CONFIG_INVALID` theo trường) và F066-AC-26 (form từ `parameter_schema`, metric từ template, không ô script, ẩn nút khi thiếu `learning_content.activity.write`) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/activities/editors/__tests__/AudioLessonEditor.test.tsx`, `SimulationEditor.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T045 [US4] `AudioLessonEditor`, `SimulationEditor` + đăng ký editor registry F058 — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/activities/editors/audio_lesson/AudioLessonEditor.tsx`, `src/cms/frontend/apps/portal/src/workspaces/content-studio/activities/editors/simulation/SimulationEditor.tsx`

## Phase 7: User Story 5 — Bảo vệ, cô lập tenant, giới hạn input (Priority: P1)

### Tests cho User Story 5

- [ ] T046 [P] [US5] Integration tenant isolation + authorization F066-AC-27 (học sinh/tenant khác → 404 ở start/get/progress/responses/simulation-runs/submit; token nhân sự → `GAME_ACCESS_STUDENT_ONLY`; ent global filter — TC-TEN-001, TC-AUTH-014) — file: `src/backend/modules/learning/tests/integration/simulation/isolation_test.go`, `src/backend/modules/learning/tests/integration/audiolesson/isolation_test.go` — phải FAIL trước khi code
- [ ] T047 [P] [US5] API test F066-AC-28 (`action` 65 ký tự, `args` 21 thuộc tính, `parameters` 51 thuộc tính → `422 VALIDATION_FAILED` — TC-TEN-024) — file: `src/backend/modules/learning/tests/integration/simulation/validation_test.go`

## Phase N: Polish & Cross-Cutting

- [ ] T048 [P] Đo template trên Web export và DEV-AND-MID1 (FPS, kích thước, thời gian nạp — TC-SIM-005, TC-PERF-020) — file: `src/game/tests/perf/simulation_lever_benchmark.gd`
- [ ] T049 [P] Cập nhật tài liệu: SIMULATION_ENGINE §7 (schema `content.`/`assessment.`, `actions`/`metrics`, descriptor), AUDIO_LANGUAGE_LEARNING §2–§3 (`plays_used`, transcript bắt buộc khi publish, bảng `assessment.video_progress`), DATABASE_DESIGN §2.6a (cột mới), CONFIG_DEFAULTS (đăng ký giới hạn action log + hiệu năng template), CHANGELOG — file: `docs/02-learning/SIMULATION_ENGINE.md`, `docs/02-learning/AUDIO_LANGUAGE_LEARNING.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/00-overview/CONFIG_DEFAULTS.md`, `CHANGELOG.md`
- [ ] T050 [P] Gỡ `recording_uploader.gd` khỏi cây thư mục GODOT_ARCHITECTURE §3 (trái ADR-0072) — file: `docs/08-game-client/GODOT_ARCHITECTURE.md`
- [ ] T051 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/066-audio-simulation-lessons/quickstart.md`
- [ ] T052 [P] Test F066-AC-29: tắt `menu.content_studio.simulations` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/audio-simulation-feature-toggle.spec.ts`
- [ ] T053 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F066-AC-29) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/activities/editors/`

## Dependencies & Execution Order

- Setup → Foundational (T004 → `make gen`; T005–T007 song song; T008 → T009) → US1 và US2 (song song, khác thư mục) → US5 → US3 (cần US1/US2 hoặc Prism mock từ spec đã có delta, gắn nhãn MOCK) → US4 (cần F058) → Polish.
- T030 (checker) và T042 (scene) cùng đọc T007 (vector); sửa công thức phải sửa cả hai trong một PR.

## Definition of Done

- [ ] Mọi tiêu chí F066-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` (MOD=learning), `make godot-test`, `make web-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
