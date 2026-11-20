# Tasks: F065 — Bài học video: plugin `video_lesson` và renderer Godot

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 [P] Fixture nội dung video mẫu (config hợp lệ/không hợp lệ, 2 nhánh) theo `video-lesson.schema.json` — file: `src/backend/modules/learning/tests/fixtures/videolesson/*.json` — ✅ config mẫu hợp lệ/không hợp lệ và 2 nhánh dựng trong test (`videolesson_test.go`, `tests/contract/videolesson_render_test.go`, `tests/integration/videolesson_test.go`) thay cho file JSON riêng
- [x] T002 [P] Khóa dịch `ui.video.*` (`seek_locked`, `not_ready`, `subtitles`, `resume_prompt`, `chapters`, `completed`) — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — ✅ `ui.video.{seek_locked, not_ready, subtitles, resume_prompt, resume, restart, play_pause}` vi/en (khóa dùng thực tế; chương dùng tên chương của nội dung, hoàn thành dùng màn kết quả F063); `messages.pot` 264 khóa, `xx_PS.po` sinh lại
- [x] T003 [P] Fixture client (`Attempt` video có cue/nhánh, `ActivityProgressState`, file `.vtt` mẫu vi/en) — file: `src/game/tests/fixtures/video/` — ✅ `tests/fakes/video_fixtures.gd` (Attempt video dựng từ fixture hợp đồng `Attempt` + `$defs/render`, media biến thể/phụ đề, `.vtt` vi/en mẫu), `fake_video_playback.gd`, `video_renderer_harness.gd` (TEST-ONLY)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: `$defs.render` trong `video-lesson.schema.json`; `ActivityRender.media[].variant/language/segment_id`; `ActivityProgressState.watched_ranges` — file: `specs/schemas/activities/video-lesson.schema.json`, `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — ✅ `$defs.render` (đóng chặt) trong `video-lesson.schema.json`; `ActivityRender.media[].variant/language/segment_id/expires_at`; `ActivityProgressState.watched_ranges` + `segment_id`; `ActivityResponseSubmission.response` bỏ `required` (branch_choice, Clarifications 2026-11-20); `split-public`, portal `pnpm gen:api`; `make specs-breaking` (docker) không chạy local — chỉ thêm trường tùy chọn/nới ràng buộc request
- [x] T005 Contract test client↔server cho payload video (sinh từ example OpenAPI) — file: `src/backend/modules/learning/tests/contract/videolesson_render_test.go` — ✅ `tests/contract/videolesson_render_test.go` — payload của plugin khớp `video-lesson.schema.json#/$defs/render`, không lộ `target_segment_id`/`question_version_id`/asset id/đáp án
- [x] T006 ent schema `VideoProgress` (PK `attempt_id`, `watched_ranges` jsonb CHECK ≤ 64, `milestones_emitted`) và `VideoBranchChoice` (`MaxLen(64)`, partial unique) với mixin tenant + soft-delete — file: `src/backend/modules/learning/ent/schema/video_progress.go`, `src/backend/modules/learning/ent/schema/video_branch_choice.go` — ✅ bảng viết tay (cùng mẫu F031: repository SQL tường minh, không ent) — `assessment.video_progress` (PK `attempt_id`, CHECK ≤ 64 khoảng, `milestones_emitted`, `processed_cue_ids`, `segment_id`, `reached_ending`), `assessment.video_branch_choices` (`varchar(64)`, partial UNIQUE) có `tenant_id` + soft-delete
- [x] T007 Migration Atlas + RLS policy viết tay — file: `src/backend/modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=video_lesson`, `make migrate-lint MOD=learning` — ✅ `migrations/20261215000001_f065_video_lesson.sql` (RLS, view `_live`, grant), `atlas.sum` cập nhật bằng `scripts/db/atlas-sum.mjs`; tenancyguard OK; `make migrate-lint` (Atlas dev DB qua docker) không chạy local
- [x] T008 Nếu `src/backend/modules/learning/ent/schema/integrity_signal.go` chưa có (F030): tạo ent schema `assessment.integrity_signals` theo DATABASE_DESIGN §2.16 — file: `src/backend/modules/learning/ent/schema/integrity_signal.go` — ✅ `assessment.integrity_signals` (append-only, chưa partition [Confirmed]) trong cùng migration

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Server phục vụ và chấm bài học video (Priority: P1) 🎯 MVP

**Goal**: plugin `video_lesson` server-authoritative.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit test domain F065-AC-01 (`Config.Validate`: `at_ms`, chương, segment target) — file: `src/backend/modules/learning/tests/unit/videolesson/config_test.go` — phải FAIL trước khi code — ✅ `internal/domain/activity/videolesson/videolesson_test.go::TestValidate`, `TestCueOrderAndDefaults`
- [x] T010 [P] [US1] Unit test domain F065-AC-05, F065-AC-06 (`Merge`, `AcceptRanges` cắt phần bất hợp lý, `CheckSeek` theo `free`/`backward_only`/`watched_only`) — file: `src/backend/modules/learning/tests/unit/videolesson/ranges_test.go` — ✅ `videolesson_test.go::TestMergeAndAccept`, `TestCheckSeek`, `TestMilestones`
- [x] T011 [P] [US1] Unit test domain F065-AC-09, F065-AC-10 (`Evaluate`: `min_watched_ratio`, `require_required_cues`, `min_score_ratio`, skip sau `attempts_per_question` lần sai) — file: `src/backend/modules/learning/tests/unit/videolesson/completion_test.go` — ✅ `videolesson_test.go::TestCompletion` (min_watched_ratio, require_required_cues, skip sau `attempts_per_question` lần sai, video nhánh)
- [x] T012 [P] [US1] Test hợp đồng plugin chung F065-AC-03 (payload không đáp án, `Evaluate` idempotent, evidence đúng schema — TC-ACT-001) — file: `src/backend/modules/learning/tests/contract/plugins/videolesson_contract_test.go` — ✅ `tests/contract/plugin_contract_test.go` chạy cả `video_lesson` (payload không đáp án, Evaluate idempotent) + `videolesson_render_test.go`
- [x] T013 [P] [US1] Integration F065-AC-02, F065-AC-04, F065-AC-12 (ghim version + item theo `at_ms`, signed URL, `VIDEO_NOT_READY`, resume trả `resume_position_ms`/`watched_ranges`; publish version mới giữa chừng — TC-VID-005) — file: `src/backend/modules/learning/tests/integration/videolesson/start_test.go` — ✅ `tests/integration/videolesson_test.go::TestVideoLessonAttempt` (cue thành item 1, biến thể `ogv_*` ký URL, phụ đề, resume `resume_position_ms`/`watched_ranges`) + `TestVideoLessonConfigAndReadiness` (`VIDEO_NOT_READY`, 0 attempt)
- [x] T014 [P] [US1] Integration F065-AC-05, F065-AC-06, F065-AC-07 (`client_seq` cũ bỏ qua, integrity signal, `VIDEO_SEEK_NOT_ALLOWED` không đổi state, `video.progressed` mỗi mốc một lần — TC-VID-002) — file: `src/backend/modules/learning/tests/integration/videolesson/progress_test.go` — ✅ `TestVideoLessonAttempt` (client_seq cũ trả trạng thái, cắt + integrity signal, `VIDEO_SEEK_NOT_ALLOWED` không đổi trạng thái, `video.progressed` đúng 4 lần)
- [x] T015 [P] [US1] Integration F065-AC-08, F065-AC-10, F065-AC-11 (cue chấm server, `attempt_answers` có `hints_used`, `video.cue_answered`; `CUE_ANSWER_REQUIRED`; submit idempotent + audit + outbox `activity.submitted`) — file: `src/backend/modules/learning/tests/integration/videolesson/respond_submit_test.go` — ✅ `TestVideoLessonAttempt` (cue chấm server → `attempt_answers`, `video.cue_answered`; `CUE_ANSWER_REQUIRED`; nộp một lần, cùng key replay, khác key 409, outbox `activity.submitted`, `activity.completed`)
- [x] T016 [P] [US1] Integration tenant isolation + authorization F065-AC-13 (học sinh khác/tenant khác → 404 ở start/get/progress/responses/submit; token nhân sự → `GAME_ACCESS_STUDENT_ONLY`; ent global filter) — file: `src/backend/modules/learning/tests/integration/videolesson/isolation_test.go` — ✅ `TestVideoLessonIsolationAndValidation` (hs.binh.3a/hs.dung.7a → 404 trên progress/get/responses/submit; nhân sự → `GAME_ACCESS_STUDENT_ONLY`); `authztest.Cover(reportActivityProgress)` cho routematrix
- [x] T017 [P] [US1] API test F065-AC-14 (`cue_id` 65 ký tự, 65 khoảng, `playback_rate` 3 → `422 VALIDATION_FAILED`) — file: `src/backend/modules/learning/tests/integration/videolesson/validation_test.go` — ✅ `TestVideoLessonIsolationAndValidation` (`cue_id` 65 ký tự, 65 khoảng, `playback_rate` 3 → `422 VALIDATION_FAILED`)
- [x] T018 [P] [US1] Test F065-AC-01 qua API CMS (`updateCmsActivityDraft` config sai → `ACTIVITY_CONFIG_INVALID`, TC-ACT-003) — file: `src/backend/modules/learning/tests/integration/videolesson/cms_validate_test.go` — ✅ `TestVideoLessonConfigAndReadiness` (`at_ms` sau cuối video, câu chèn không publish → `422 ACTIVITY_CONFIG_INVALID` khi lưu)

### Implementation cho User Story 1

- [x] T019 [P] [US1] Domain thuần `config.go`, `ranges.go`, `plausibility.go`, `seek.go`, `completion.go` — file: `src/backend/modules/learning/internal/domain/activity/videolesson/` — ✅ `internal/domain/activity/videolesson/{config,ranges,completion,render}.go` (plausibility + seek trong `ranges.go`)
- [x] T020 [P] [US1] Port `VideoProgressRepository`, `MediaVariantResolver`, `IntegritySignalWriter` — file: `src/backend/modules/learning/internal/application/port/` — ✅ port `write.MediaProgressStore` (progress FOR UPDATE, branch choice, integrity signal), `read.MediaProgressReadModel`; media qua `write.MediaStore` + `port.URLSigner` có sẵn của F027
- [x] T021 [US1] Plugin `videolesson.Plugin` (Type, ConfigSchema, ValidateConfig, BuildRenderPayload, Respond, Progress, Evaluate, CompletionRule, EmitEvidence) + ghi learning event qua port F031 — file: `src/backend/modules/learning/internal/application/activityplugin/videolesson/plugin.go` — ✅ plugin `internal/domain/activity/plugins/video` (activity.Plugin + `activity.Timeline`); engine: `command/attempt_media.go` (`ReportProgress`, `Branch`, cue, completion), start/respond/submit/view của F030 gọi các hook này; route `reportActivityProgress`
- [x] T022 [US1] Repository ent (`FOR UPDATE` theo `attempt_id`, TenantScope) — file: `src/backend/modules/learning/internal/infrastructure/persistence/video_progress_repo.go`, `video_branch_choice_repo.go`, `integrity_signal_repo.go` — ✅ `persistence/write/media_progress_repo.go`, `persistence/read/media_progress.go` (SQL tường minh, `tenancyguard:ok`)
- [x] T023 [US1] Đăng ký plugin vào Registry, `listActivityTypes` trả `video_lesson` (`min_client_version`, `evaluation_strategies = [completion, auto]`) — file: `src/backend/modules/learning/cmd/api/main.go` — ✅ `plugins.All()` có `video.New()` (module.go dùng `plugins.Registry()`); `listActivityTypes` trả `video_lesson` (`min_client_version` 0.1.0, strategies `completion, auto`) — `TestActivityRegistryAndConfig`
- [x] T024 [US1] Metric/span/log theo plan (không log câu trả lời) — file: `src/backend/modules/learning/internal/application/activityplugin/videolesson/plugin.go` — ✅ metric `eduworld_video_progress_trimmed_total`, `eduworld_video_seek_rejected_total`, `eduworld_video_completion_total{completed}`; span qua `observability.UseCase` (`ReportActivityProgress`, `ChooseVideoBranch`); log không có câu trả lời

**Checkpoint**: User Story 1 chạy và kiểm độc lập được (qua API).

## Phase 4: User Story 2 — Học sinh xem video trong game (Priority: P1)

**Goal**: renderer `video_lesson` trong activity player.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T025 [P] [US2] Unit test `VttParser` + chọn phụ đề F065-AC-16 (cue timing, nhiều dòng, fallback `is_default`) — file: `src/game/tests/unit/video/test_vtt_parser.gd` — phải FAIL trước khi code — ✅ `tests/unit/video/test_vtt_parser.gd` 4/4 pass
- [x] T026 [P] [US2] Unit test `WatchedRanges` + `VideoProgressReporter` F065-AC-17, F065-AC-19 (khóa tua theo mép, `client_seq` tăng, gửi khi pause/exit, gộp khi offline) — file: `src/game/tests/unit/video/test_video_progress.gd` — ✅ `tests/unit/video/test_video_progress.gd` 4/4 pass (khóa tua, `client_seq` tăng theo unix ms, gửi khi pause, khoảng cộng dồn thay hàng đợi khi mất mạng; heartbeat không key; body `cue_answer`/`branch_choice`)
- [x] T027 [P] [US2] Scene test F065-AC-15, F065-AC-23 (biến thể theo preset, hạ 480p sau 2 lần đệm > 3 s, `suspend_world("video")`) — file: `src/game/tests/scene/video/test_video_player_quality.gd` — ✅ `tests/scene/video/test_video_player_quality.gd` 3/3 pass (Low/Web → `ogv_480`, Trung bình/Cao → `ogv_720`, 2 lần đứng > 3 s → 480p giữ vị trí, `suspend_world("video")`/resume khi teardown, phụ đề theo ngôn ngữ UI, vi → en → tắt, panel `VoiceCaption`)
- [x] T028 [P] [US2] Scene test F065-AC-18, F065-AC-21 (dừng đúng `at_ms`, overlay renderer câu hỏi, không bỏ qua cue bắt buộc, hết video → tua về cue còn thiếu hoặc submit) — file: `src/game/tests/scene/video/test_cue_overlay.gd` — ✅ `tests/scene/video/test_cue_overlay.gd` 4/4 pass (dừng ở `at_ms`, overlay renderer câu hỏi F063, không có nút bỏ qua, `cue_answer`, phát tiếp sau phản hồi; heartbeat khi phát/pause; khóa tua + toast; hết video → tua về cue bắt buộc còn thiếu, không còn → `finish_requested`)
- [x] T029 [P] [US2] Scene test F065-AC-20, F065-AC-22 (hỏi xem tiếp; `VIDEO_NOT_READY`; `VIDEO_SEEK_NOT_ALLOWED` → tua về mép) — file: `src/game/tests/scene/video/test_video_resume_errors.gd` — ✅ `tests/scene/video/test_video_resume_errors.gd` 3/3 pass ("Xem tiếp từ 03:12?" → tua; `VIDEO_SEEK_NOT_ALLOWED` → về mép; `VIDEO_NOT_READY` → `ui.video.not_ready`)
- [ ] T030 [US2] Integration headless với compose: `hs.an.3a` xem video có cue 01:30 tới hoàn thành (TC-VID-001, TC-VID-003, TC-VID-004) — file: `src/game/tests/integration/video/test_video_flow.gd` — viết sẵn `tests/integration/video/test_video_flow.gd` (skip khi thiếu `EDUWORLD_IT_API_BASE_URL`/`EDUWORLD_IT_VIDEO_ACTIVITY_ID`); **chưa chạy**: cần backend compose + video seed đã transcode (F027) — không khởi động container trong phiên này

### Implementation cho User Story 2

- [x] T031 [P] [US2] `VttParser`, `WatchedRanges`, `VideoProgressReporter` — file: `src/game/scripts/learning/vtt_parser.gd`, `src/game/scripts/learning/watched_ranges.gd`, `src/game/scripts/learning/video_progress_reporter.gd` — ✅ `scripts/learning/{vtt_parser,watched_ranges,video_progress_reporter}.gd` + `video_playback.gd` (bọc VideoStreamPlayer), `video_file_cache.gd` (`user://cache/media/<sha256>/<variant>.ogv`, không lưu query ký)
- [x] T032 [US2] `VideoLessonPlayer` (VideoStreamPlayer, SubtitleLayer, CueOverlay, ChapterList, SeekBar, CC vi/en/tắt) — file: `src/game/scenes/learning/video/VideoLessonPlayer.tscn`, `src/game/scenes/learning/video/video_lesson_player.gd` — ✅ gộp vào `VideoLessonRenderer.tscn` (VideoStreamPlayer, SubtitlePanel `VoiceCaption`, Overlay câu chèn, Branch, Resume, SeekBar, CC, chương) thay cho scene `VideoLessonPlayer` riêng
- [x] T033 [US2] `VideoLessonRenderer` kế thừa `ActivityRendererBase`, đăng ký `ActivityRendererRegistry` — file: `src/game/scenes/learning/activity_renderers/video_lesson/VideoLessonRenderer.tscn`, `src/game/scenes/learning/activity_renderers/video_lesson/video_lesson_renderer.gd` — ✅ `video_lesson_renderer.gd` (`VideoLessonRenderer` kế thừa `ActivityRendererBase`), đăng ký `ActivityRendererRegistry`; hợp đồng renderer thêm `wants_action_bar`, `apply_progress`, `progress_refused`, `final_report`; `ActivityHost` nối `progress_reported`/`finish_requested`, gửi heartbeat cuối trước `:pause`; LearningService `report_progress`, `choose_branch`, `respond(..., {kind, cue_id})`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Video tương tác có nhánh (Priority: P3)

**Goal**: chọn nhánh, phát đoạn tiếp, hoàn thành ở đoạn kết.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T034 [P] [US3] Integration F065-AC-25 (ghi `video_branch_choices`, một lựa chọn mỗi điểm rẽ, idempotent, trả `next_segment_id`, hoàn thành ở `is_ending`) — file: `src/backend/modules/learning/tests/integration/videolesson/branch_test.go` — phải FAIL trước khi code — ✅ `tests/integration/videolesson_test.go::TestVideoLessonBranches` (một lựa chọn mỗi điểm rẽ, cùng option trả lại, khác option 409, không chấm đúng/sai, `next_segment_id`, hoàn thành ở đoạn `is_ending` + `activity.completed`)
- [x] T035 [P] [US3] Scene test F065-AC-24 (dừng tại điểm rẽ, gửi `branch_choice`, phát đoạn theo `next_segment_id`) — file: `src/game/tests/scene/video/test_branch_choice.gd` — ✅ `tests/scene/video/test_branch_choice.gd` 1/1 pass (dừng ở điểm rẽ, prompt + lựa chọn, `branch_choice`, phát đoạn `water` từ đầu, hết đoạn kết → `finish_requested`)

### Implementation cho User Story 3

- [x] T036 [US3] `Respond(branch_choice)` + điều kiện hoàn thành theo đoạn — file: `src/backend/modules/learning/internal/application/activityplugin/videolesson/plugin.go`, `src/backend/modules/learning/internal/domain/activity/videolesson/completion.go` — ✅ `Attempts.Branch` + `videolesson.Evaluate` (đoạn kết thay `watched_ratio` cho video nhánh) — cùng commit US1
- [x] T037 [US3] `BranchPrompt` trong player + chuyển đoạn — file: `src/game/scenes/learning/video/BranchPrompt.tscn`, `src/game/scenes/learning/video/video_lesson_player.gd` — ✅ prompt nhánh nằm trong `VideoLessonRenderer.tscn` (`%Branch`, `open_branch_prompt`, `choose_branch`, `_branch_chosen`) thay cho scene `BranchPrompt` riêng; host gửi `choose_branch` và trả `next_segment_id` cho renderer

## Phase 6: Polish & Cross-Cutting

- [ ] T038 [P] Đo 720p@30 trên DEV-AND-MID1, 480p Web (TC-PERF-020) — file: `src/game/tests/perf/video_player_benchmark.gd` — **chưa làm**: cần thiết bị DEV-AND-MID1 (F133); headless chỉ kiểm logic, không đo FPS giải mã Theora
- [ ] T039 [P] Postman request cho start/progress/responses/submit video — file: `specs/postman/` (khi thư mục được tạo ở Bước 0) — **chưa làm**: thư mục `specs/postman/` chưa có (Bước 0)
- [x] T040 [P] Cập nhật tài liệu: VIDEO_LEARNING §8 (cue là item, bảng `assessment.*`), DATABASE_DESIGN §2.6a (`milestones_emitted`, PK `video_branch_choices`), CHANGELOG — file: `docs/02-learning/VIDEO_LEARNING.md`, `docs/07-backend/DATABASE_DESIGN.md`, `CHANGELOG.md` — ✅ VIDEO_LEARNING §8 (cue là item theo `at_ms`, cột mới), DATABASE_DESIGN §2.6a/§2.16, CHANGELOG
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/065-video-lesson/quickstart.md` — **chưa làm**: cần backend compose + video seed đã transcode Theora (F027)

## Dependencies & Execution Order

- Setup → Foundational (T004 → T005; T006 → T007; T008) → US1 → US2 → US3 → Polish.
- US2 cần F063 (hợp đồng renderer, renderer câu hỏi, `PendingWrites`) và US1 (hoặc Prism mock từ spec đã có delta T004 — gắn nhãn MOCK).

## Definition of Done

- [x] Mọi tiêu chí F065-AC-* có test pass (ghi mã test/TC) — 25/25 (backend: `videolesson_test.go` domain, contract, 4 integration testcontainers; client: 6 suite `tests/{unit,scene}/video`); AC-23 FPS 720p trên thiết bị tầm trung chờ T038
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` (MOD=learning) và `make godot-test` xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — fake chỉ ở `tests/`
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — VIDEO_LEARNING, DATABASE_DESIGN, CHANGELOG (CONFIG_DEFAULTS không đổi)
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — `In progress`
