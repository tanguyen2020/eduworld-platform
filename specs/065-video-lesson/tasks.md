# Tasks: F065 — Bài học video: plugin `video_lesson` và renderer Godot

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 [P] Fixture nội dung video mẫu (config hợp lệ/không hợp lệ, 2 nhánh) theo `video-lesson.schema.json` — file: `modules/learning/tests/fixtures/videolesson/*.json`
- [ ] T002 [P] Khóa dịch `ui.video.*` (`seek_locked`, `not_ready`, `subtitles`, `resume_prompt`, `chapters`, `completed`) — file: `godot/locale/vi.po`, `godot/locale/en.po`
- [ ] T003 [P] Fixture client (`Attempt` video có cue/nhánh, `ActivityProgressState`, file `.vtt` mẫu vi/en) — file: `godot/tests/fixtures/video/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: `$defs.render` trong `video-lesson.schema.json`; `ActivityRender.media[].variant/language/segment_id`; `ActivityProgressState.watched_ranges` — file: `specs/schemas/activities/video-lesson.schema.json`, `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T005 Contract test client↔server cho payload video (sinh từ example OpenAPI) — file: `modules/learning/tests/contract/videolesson_render_test.go`
- [ ] T006 ent schema `VideoProgress` (PK `attempt_id`, `watched_ranges` jsonb CHECK ≤ 64, `milestones_emitted`) và `VideoBranchChoice` (`MaxLen(64)`, partial unique) với mixin tenant + soft-delete — file: `modules/learning/ent/schema/video_progress.go`, `modules/learning/ent/schema/video_branch_choice.go`
- [ ] T007 Migration Atlas + RLS policy viết tay — file: `modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=video_lesson`, `make migrate-lint MOD=learning`
- [ ] T008 Nếu `modules/learning/ent/schema/integrity_signal.go` chưa có (F030): tạo ent schema `assessment.integrity_signals` theo DATABASE_DESIGN §2.16 — file: `modules/learning/ent/schema/integrity_signal.go`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Server phục vụ và chấm bài học video (Priority: P1) 🎯 MVP

**Goal**: plugin `video_lesson` server-authoritative.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test domain F065-AC-01 (`Config.Validate`: `at_ms`, chương, segment target) — file: `modules/learning/tests/unit/videolesson/config_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Unit test domain F065-AC-05, F065-AC-06 (`Merge`, `AcceptRanges` cắt phần bất hợp lý, `CheckSeek` theo `free`/`backward_only`/`watched_only`) — file: `modules/learning/tests/unit/videolesson/ranges_test.go`
- [ ] T011 [P] [US1] Unit test domain F065-AC-09, F065-AC-10 (`Evaluate`: `min_watched_ratio`, `require_required_cues`, `min_score_ratio`, skip sau `attempts_per_question` lần sai) — file: `modules/learning/tests/unit/videolesson/completion_test.go`
- [ ] T012 [P] [US1] Test hợp đồng plugin chung F065-AC-03 (payload không đáp án, `Evaluate` idempotent, evidence đúng schema — TC-ACT-001) — file: `modules/learning/tests/contract/plugins/videolesson_contract_test.go`
- [ ] T013 [P] [US1] Integration F065-AC-02, F065-AC-04, F065-AC-12 (ghim version + item theo `at_ms`, signed URL, `VIDEO_NOT_READY`, resume trả `resume_position_ms`/`watched_ranges`; publish version mới giữa chừng — TC-VID-005) — file: `modules/learning/tests/integration/videolesson/start_test.go`
- [ ] T014 [P] [US1] Integration F065-AC-05, F065-AC-06, F065-AC-07 (`client_seq` cũ bỏ qua, integrity signal, `VIDEO_SEEK_NOT_ALLOWED` không đổi state, `video.progressed` mỗi mốc một lần — TC-VID-002) — file: `modules/learning/tests/integration/videolesson/progress_test.go`
- [ ] T015 [P] [US1] Integration F065-AC-08, F065-AC-10, F065-AC-11 (cue chấm server, `attempt_answers` có `hints_used`, `video.cue_answered`; `CUE_ANSWER_REQUIRED`; submit idempotent + audit + outbox `activity.submitted`) — file: `modules/learning/tests/integration/videolesson/respond_submit_test.go`
- [ ] T016 [P] [US1] Integration tenant isolation + authorization F065-AC-13 (học sinh khác/tenant khác → 404 ở start/get/progress/responses/submit; token nhân sự → `GAME_ACCESS_STUDENT_ONLY`; ent global filter) — file: `modules/learning/tests/integration/videolesson/isolation_test.go`
- [ ] T017 [P] [US1] API test F065-AC-14 (`cue_id` 65 ký tự, 65 khoảng, `playback_rate` 3 → `422 VALIDATION_FAILED`) — file: `modules/learning/tests/integration/videolesson/validation_test.go`
- [ ] T018 [P] [US1] Test F065-AC-01 qua API CMS (`updateCmsActivityDraft` config sai → `ACTIVITY_CONFIG_INVALID`, TC-ACT-003) — file: `modules/learning/tests/integration/videolesson/cms_validate_test.go`

### Implementation cho User Story 1

- [ ] T019 [P] [US1] Domain thuần `config.go`, `ranges.go`, `plausibility.go`, `seek.go`, `completion.go` — file: `modules/learning/internal/domain/activity/videolesson/`
- [ ] T020 [P] [US1] Port `VideoProgressRepository`, `MediaVariantResolver`, `IntegritySignalWriter` — file: `modules/learning/internal/application/port/`
- [ ] T021 [US1] Plugin `videolesson.Plugin` (Type, ConfigSchema, ValidateConfig, BuildRenderPayload, Respond, Progress, Evaluate, CompletionRule, EmitEvidence) + ghi learning event qua port F031 — file: `modules/learning/internal/application/activityplugin/videolesson/plugin.go`
- [ ] T022 [US1] Repository ent (`FOR UPDATE` theo `attempt_id`, TenantScope) — file: `modules/learning/internal/infrastructure/persistence/video_progress_repo.go`, `video_branch_choice_repo.go`, `integrity_signal_repo.go`
- [ ] T023 [US1] Đăng ký plugin vào Registry, `listActivityTypes` trả `video_lesson` (`min_client_version`, `evaluation_strategies = [completion, auto]`) — file: `modules/learning/cmd/api/main.go`
- [ ] T024 [US1] Metric/span/log theo plan (không log câu trả lời) — file: `modules/learning/internal/application/activityplugin/videolesson/plugin.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được (qua API).

## Phase 4: User Story 2 — Học sinh xem video trong game (Priority: P1)

**Goal**: renderer `video_lesson` trong activity player.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T025 [P] [US2] Unit test `VttParser` + chọn phụ đề F065-AC-16 (cue timing, nhiều dòng, fallback `is_default`) — file: `godot/tests/unit/video/test_vtt_parser.gd` — phải FAIL trước khi code
- [ ] T026 [P] [US2] Unit test `WatchedRanges` + `VideoProgressReporter` F065-AC-17, F065-AC-19 (khóa tua theo mép, `client_seq` tăng, gửi khi pause/exit, gộp khi offline) — file: `godot/tests/unit/video/test_video_progress.gd`
- [ ] T027 [P] [US2] Scene test F065-AC-15, F065-AC-23 (biến thể theo preset, hạ 480p sau 2 lần đệm > 3 s, `suspend_world("video")`) — file: `godot/tests/scene/video/test_video_player_quality.gd`
- [ ] T028 [P] [US2] Scene test F065-AC-18, F065-AC-21 (dừng đúng `at_ms`, overlay renderer câu hỏi, không bỏ qua cue bắt buộc, hết video → tua về cue còn thiếu hoặc submit) — file: `godot/tests/scene/video/test_cue_overlay.gd`
- [ ] T029 [P] [US2] Scene test F065-AC-20, F065-AC-22 (hỏi xem tiếp; `VIDEO_NOT_READY`; `VIDEO_SEEK_NOT_ALLOWED` → tua về mép) — file: `godot/tests/scene/video/test_video_resume_errors.gd`
- [ ] T030 [US2] Integration headless với compose: `hs.an.3a` xem video có cue 01:30 tới hoàn thành (TC-VID-001, TC-VID-003, TC-VID-004) — file: `godot/tests/integration/video/test_video_flow.gd`

### Implementation cho User Story 2

- [ ] T031 [P] [US2] `VttParser`, `WatchedRanges`, `VideoProgressReporter` — file: `godot/scripts/learning/vtt_parser.gd`, `godot/scripts/learning/watched_ranges.gd`, `godot/scripts/learning/video_progress_reporter.gd`
- [ ] T032 [US2] `VideoLessonPlayer` (VideoStreamPlayer, SubtitleLayer, CueOverlay, ChapterList, SeekBar, CC vi/en/tắt) — file: `godot/scenes/learning/video/VideoLessonPlayer.tscn`, `godot/scenes/learning/video/video_lesson_player.gd`
- [ ] T033 [US2] `VideoLessonRenderer` kế thừa `ActivityRendererBase`, đăng ký `ActivityRendererRegistry` — file: `godot/scenes/learning/activity_renderers/video_lesson/VideoLessonRenderer.tscn`, `godot/scenes/learning/activity_renderers/video_lesson/video_lesson_renderer.gd`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Video tương tác có nhánh (Priority: P3)

**Goal**: chọn nhánh, phát đoạn tiếp, hoàn thành ở đoạn kết.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T034 [P] [US3] Integration F065-AC-25 (ghi `video_branch_choices`, một lựa chọn mỗi điểm rẽ, idempotent, trả `next_segment_id`, hoàn thành ở `is_ending`) — file: `modules/learning/tests/integration/videolesson/branch_test.go` — phải FAIL trước khi code
- [ ] T035 [P] [US3] Scene test F065-AC-24 (dừng tại điểm rẽ, gửi `branch_choice`, phát đoạn theo `next_segment_id`) — file: `godot/tests/scene/video/test_branch_choice.gd`

### Implementation cho User Story 3

- [ ] T036 [US3] `Respond(branch_choice)` + điều kiện hoàn thành theo đoạn — file: `modules/learning/internal/application/activityplugin/videolesson/plugin.go`, `modules/learning/internal/domain/activity/videolesson/completion.go`
- [ ] T037 [US3] `BranchPrompt` trong player + chuyển đoạn — file: `godot/scenes/learning/video/BranchPrompt.tscn`, `godot/scenes/learning/video/video_lesson_player.gd`

## Phase 6: Polish & Cross-Cutting

- [ ] T038 [P] Đo 720p@30 trên DEV-AND-MID1, 480p Web (TC-PERF-020) — file: `godot/tests/perf/video_player_benchmark.gd`
- [ ] T039 [P] Postman request cho start/progress/responses/submit video — file: `specs/postman/` (khi thư mục được tạo ở Bước 0)
- [ ] T040 [P] Cập nhật tài liệu: VIDEO_LEARNING §8 (cue là item, bảng `assessment.*`), DATABASE_DESIGN §2.6a (`milestones_emitted`, PK `video_branch_choices`), CHANGELOG — file: `docs/02-learning/VIDEO_LEARNING.md`, `docs/07-backend/DATABASE_DESIGN.md`, `CHANGELOG.md`
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/065-video-lesson/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T004 → T005; T006 → T007; T008) → US1 → US2 → US3 → Polish.
- US2 cần F063 (hợp đồng renderer, renderer câu hỏi, `PendingWrites`) và US1 (hoặc Prism mock từ spec đã có delta T004 — gắn nhãn MOCK).

## Definition of Done

- [ ] Mọi tiêu chí F065-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` (MOD=learning) và `make godot-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
