# Tasks: F137 — Âm thanh game: nhạc theo vùng/màn, âm thanh UI và sự kiện, ambience, phù hợp giáo dục, asset CC0/MIT

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Quy ước: test viết trước và phải FAIL trước khi code của story đó; fake/fixture gắn nhãn TEST DOUBLE (R17); file âm thanh test tự sinh MIT trong `tests/`; mọi đường dẫn dưới `src/game/` trừ khi ghi khác. **Agent không tải asset** (ADR-0102 mục 6) — T012 do người được owner giao.

## Phase 1: Setup

- [ ] T001 [P] Khóa dịch `ui.settings.sound.footsteps`, `ui.settings.sound.sample_*`, mục "Âm thanh" trang Giấy phép (vi/en), sinh lại `messages.pot`, `xx_PS.po` — file: `src/game/locale/vi.po`, `src/game/locale/en.po`
- [ ] T002 [P] Bộ sinh file âm thanh test MIT tất định (sine/noise ngắn, đủ `kind`) + fixture catalog/manifest hợp lệ và không hợp lệ — file: `src/game/tests/fixtures/audio/`, `src/game/tools/audio/tests/fixtures/`
- [ ] T003 [P] Image CI ghim ffmpeg (phiên bản cố định) cho chuẩn hóa/lint; lệnh chạy local bằng container dùng một lần — file: `src/game/tools/audio/Dockerfile`, `.github/workflows/godot.yml`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 JSON Schema `audio-catalog.schema.json`, `audio-manifest.schema.json` (mọi chuỗi có `maxLength`/`pattern`/`enum`, R28) — file: `src/game/tools/audio/schemas/`
- [ ] T005 Dòng `assets/audio/**` trỏ manifest trong `LICENSES.md`; `audio_manifest.json` rỗng hợp lệ; `audio_catalog.json` khung (id ngữ cảnh/sự kiện theo STUDENT_GAME_UX §8a, chưa có file) — file: `src/game/assets/LICENSES.md`, `src/game/assets/audio/audio_manifest.json`, `src/game/resources/audio/audio_catalog.json`
- [ ] T006 `AudioCatalog`, `DuckStack`, `AudioContextResolver` (thuần, class rỗng có chữ ký) và đăng ký trong `AudioDirector` — file: `src/game/scripts/core/audio_catalog.gd`, `src/game/scripts/core/audio_duck_stack.gd`, `src/game/scripts/core/audio_context_resolver.gd`, `src/game/scripts/core/audio_director.gd`
- [ ] T007 Intent `EventBus.audio_requested(event: StringName)` và allowlist `AudioStreamPlayer` (chỉ `AudioDirector`, `FootstepEmitter`, player media/giọng sẵn có) — file: `src/game/scripts/core/event_bus.gd`, `src/game/tools/audio/player_allowlist.txt`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Nhạc nền theo vùng và màn, chuyển mượt (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T008 [P] [US1] Unit F137-AC-01, F137-AC-02, F137-AC-04, F137-AC-05: tra id, id lạ → rỗng + WARN; crossfade đúng `audio_music_crossfade`; cùng id không khởi động lại; `background_music_id` rồi trả nhạc cũ; stream nạp/giải phóng, gói chưa tải → im lặng (TC-UI-015, TC-PERF-026) — file: `src/game/tests/unit/audio/test_audio_catalog.gd`, `src/game/tests/unit/audio/test_audio_director_music.gd`, `src/game/tests/unit/audio/test_audio_director_streaming.gd` — phải FAIL trước khi code
- [ ] T009 [P] [US1] Scene F137-AC-03: bảng S01–S18 (dữ liệu) so với ngữ cảnh thực tế từng màn; nhạc `menu` chỉ sau tương tác đầu trên Web (TC-UI-015) — file: `src/game/tests/scene/audio/test_screen_sound_table.gd` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Kiến trúc F137-AC-06: không `AudioStreamPlayer` ngoài allowlist trong `.tscn`/`.gd`, scene không gọi `play()` âm thanh — file: `src/game/tests/unit/audio/test_audio_boundaries.gd` — phải FAIL trước khi code

### Implementation cho User Story 1

- [ ] T011 [US1] `AudioDirector`: `set_context`, hai player nhạc luân phiên + crossfade, stream theo `bundle_id`, nối `SceneRouter`/zone `music_id`, mini-game `background_music_id` — file: `src/game/scripts/core/audio_director.gd`, `src/game/scripts/core/audio_context_resolver.gd`, `src/game/scripts/world/zone_root.gd`
- [ ] T012 [US1] **(Người được owner giao, không phải agent)** Tải pack/bài ứng viên theo [research.md](research.md), kiểm license trên trang **từng file**, lưu văn bản license gốc, ghi `source_url`/`author`/`license`/`source_sha256`/`retrieved` vào manifest — file: `src/game/assets/audio/`, `src/game/assets/audio/licenses/`, `src/game/assets/audio/audio_manifest.json`
- [ ] T013 [US1] Gán `music_id`/`ambience_id` cho mọi zone scene và id ngữ cảnh menu/login/arena/boss/learning trong catalog — file: `src/game/scenes/world/`, `src/game/resources/audio/audio_catalog.json`

**Checkpoint**: US1 chạy và kiểm độc lập được (với file test hoặc asset thật).

## Phase 4: User Story 2 — Âm thanh không làm phiền việc học (Priority: P1)

### Tests cho User Story 2

- [ ] T014 [P] [US2] Unit F137-AC-07, F137-AC-09: `DuckStack` cộng dồn lý do, mức sâu nhất, trả lại khi hết lý do; `duck_music` F066 vẫn chạy (TC-UI-015) — file: `src/game/tests/unit/audio/test_duck_stack.gd`, `src/game/tests/scene/audio/test_voice_ducking.gd` — phải FAIL trước khi code
- [ ] T015 [P] [US2] Scene F137-AC-08: bài kiểm tra F064 và activity có `time_limit_seconds` → nhạc/ambience tắt, không đúng/sai/tích tắc/chuông, UI tối thiểu (TC-UI-015) — file: `src/game/tests/scene/audio/test_exam_silence.gd` — phải FAIL trước khi code
- [ ] T016 [P] [US2] Unit F137-AC-10, F137-AC-11: mọi nguồn qua bus, âm lượng/tắt tiếng ≤ 100 ms, bus 0% không giải mã; app vào nền tạm dừng, không chuông, quay lại fade-in (TC-UI-015, TC-UI-016) — file: `src/game/tests/unit/audio/test_audio_director_buses.gd` — phải FAIL trước khi code

### Implementation cho User Story 2

- [ ] T017 [US2] `DuckStack` trong `AudioDirector`; nối `VoiceOverPlayer`, "Đọc to" (F097), thoại NPC, hướng dẫn Mây (lý do `voice`), bài nghe/video (`listening`), `AppState.blocking_activity` + `time_limit_seconds` (`exam`) — file: `src/game/scripts/core/audio_director.gd`, `src/game/scripts/learning/`, `src/game/scenes/learning/`
- [ ] T018 [US2] Tạm dừng/tiếp tục theo `NOTIFICATION_APPLICATION_PAUSED`/`RESUMED` và tab Web ẩn — file: `src/game/scripts/core/audio_director.gd`
- [ ] T019 [US2] Rà nghe và đánh dấu `review.calm` cho từng file (không lời, không gắt/dọa) — F137-AC-12; owner nghe duyệt danh sách (câu hỏi mở F137-1) — file: `src/game/assets/audio/audio_manifest.json`

## Phase 5: User Story 3 — Asset chỉ CC0/MIT, có manifest, kiểm tự động (Priority: P1)

### Tests cho User Story 3

- [ ] T020 [P] [US3] `node --test` F137-AC-13, F137-AC-14, F137-AC-16: fixture thiếu bản ghi, `CC-BY-4.0`, `Public-Domain`, sha256 lệch, `.wav`, SFX 2 s, loudness −10 LUFS, true peak 0 dBTP, catalog trỏ file lạ, zone `music_id` lạ, bản ghi thừa → exit 1 đúng file; bộ hợp lệ → exit 0 (TC-UI-017) — file: `src/game/tools/audio/tests/lint_audio.test.mjs` — phải FAIL trước khi code
- [ ] T021 [P] [US3] Test F137-AC-15: chạy `normalize.sh` hai lần → cùng `sha256`, số đo ghi vào manifest (TC-UI-017) — file: `src/game/tools/audio/tests/normalize.test.sh` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T022 [US3] `lint_audio.mjs` (ajv schema, sha256, định dạng, độ dài, ebur128 qua ffmpeg, ngân sách `--budget`) — file: `src/game/tools/audio/lint_audio.mjs`
- [ ] T023 [US3] `normalize.sh` (loudnorm hai lượt theo `kind`, libvorbis, mono SFX/UI, cắt im lặng, ghi manifest) — file: `src/game/tools/audio/normalize.sh`
- [ ] T024 [US3] Job CI `audio-lint` trong `godot.yml` (path filter `src/game/assets/audio/**`, `src/game/resources/audio/**`, `src/game/tools/audio/**`, zone scene) — file: `.github/workflows/godot.yml`
- [ ] T025 [US3] Trang Giấy phép: mục "Âm thanh" sinh từ manifest khi build (F137-AC-17) + test — file: `src/game/scenes/ui/settings/`, `src/game/tests/scene/test_license_page.gd`

## Phase 6: User Story 4 — Âm thanh giao diện (Priority: P2)

### Tests cho User Story 4

- [ ] T026 [P] [US4] Scene F137-AC-18, F137-AC-19: nút/Window/tab/lỗi/lưu phát đúng `ui_*` qua component dùng chung; nút mờ im; 20 chạm liên tiếp không chồng 80 ms, không vượt `audio_sfx_max_voices`, ưu tiên phản hồi học tập (TC-UI-015, TC-UI-016) — file: `src/game/tests/scene/audio/test_ui_sounds.gd`, `src/game/tests/unit/audio/test_sfx_pool.gd` — phải FAIL trước khi code
- [ ] T027 [P] [US4] Scene F137-AC-20: chat không tiếng; thông báo F123 một tiếng một lần khi app mở, không trong bài kiểm tra, không khi công tắc tắt (TC-UI-016) — file: `src/game/tests/scene/audio/test_notification_chime.gd` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T028 [US4] SFX pool + `play_ui` trong `AudioDirector`; gắn vào `UiBuild`/nút tầng 1/`Window`/tab/toast lỗi F114 — file: `src/game/scripts/core/audio_director.gd`, `src/game/scenes/ui/components/`
- [ ] T029 [US4] Hộp thông báo F123 phát `ui_notify` theo F137-AC-20 — file: `src/game/scenes/ui/hud/`, `src/game/scripts/notifications/`

## Phase 7: User Story 5 — Âm thanh học tập và sự kiện, luôn có tín hiệu hình (Priority: P2)

### Tests cho User Story 5

- [ ] T030 [P] [US5] Scene F137-AC-21, F137-AC-26: kết quả chấm server → `correct`/`try_again` + ✓/✗, `streak` một lần; catalog thiếu file → `MiniGameFeedbackSfx` (TC-UI-016, TC-UI-015) — file: `src/game/tests/scene/audio/test_learning_feedback_sounds.gd` — phải FAIL trước khi code
- [ ] T031 [P] [US5] Scene F137-AC-22, F137-AC-24, F137-AC-25: stinger quest/thưởng/XP/level-up/boss/trận, nhạc giảm tạm, không tích tắc, thua kèm lời động viên; giảm chuyển động → gain + bản ngắn; không âm thanh cờ bạc (TC-UI-016) — file: `src/game/tests/scene/audio/test_event_stingers.gd` — phải FAIL trước khi code
- [ ] T032 [P] [US5] Unit + scene F137-AC-23: mọi sự kiện catalog có `visual_cue`; ma trận ba bus 0% / không đầu ra âm thanh vẫn đủ tín hiệu hình (TC-UI-016) — file: `src/game/tests/unit/audio/test_visual_cue_coverage.gd`, `src/game/tests/scene/audio/test_muted_matrix.gd` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T033 [US5] `play_event` + nối activity player (F063), mini-game (F073), quest (F072), thưởng/level-up (F069), boss (F110), Arena (F109) — file: `src/game/scripts/core/audio_director.gd`, `src/game/scenes/learning/`, `src/game/scenes/quest/`, `src/game/scenes/challenge/`, `src/game/scenes/competition/`
- [ ] T034 [US5] Biến thể giảm chuyển động (`reduced_motion_file`, gain) và chuỗi đúng — file: `src/game/scripts/core/audio_director.gd`, `src/game/resources/audio/audio_catalog.json`
- [ ] T035 [US5] Công thức SFX tự tổng hợp MIT cho tiếng còn thiếu (dự phòng, tái lập) — file: `src/game/tools/audio/synth/`

## Phase 8: User Story 6 — Ambience theo vùng và tiếng bước chân (Priority: P3)

### Tests cho User Story 6

- [ ] T036 [P] [US6] Scene F137-AC-27, F137-AC-28: ambience theo `ambience_id`, crossfade, tắt khi kiểm tra, giảm dưới giọng; bước chân chỉ nhân vật của em, theo preset, công tắc (TC-UI-015) — file: `src/game/tests/scene/audio/test_ambience_footsteps.gd` — phải FAIL trước khi code

### Implementation cho User Story 6

- [ ] T037 [US6] Ambience trong `AudioDirector`; `FootstepEmitter` theo nhịp clip `walk`/`run`; `Settings.footsteps_enabled` mặc định theo preset — file: `src/game/scripts/core/audio_director.gd`, `src/game/scripts/player/footstep_emitter.gd`, `src/game/scripts/core/settings.gd`

## Phase 9: User Story 7 — Ngân sách, hiệu năng, Cài đặt, ranh giới (Priority: P2)

### Tests cho User Story 7

- [ ] T038 [P] [US7] Ngân sách F137-AC-29: `lint_audio.mjs --budget` + `scripts/godot/size` trên export Web chặn khi vượt `audio_core_budget`/`audio_zone_budget`/40 MB (TC-PERF-026) — file: `src/game/tools/audio/tests/lint_audio.test.mjs`, `scripts/godot/` — phải FAIL trước khi code
- [ ] T039 [P] [US7] Perf F137-AC-30: Hub có nhạc + ambience + SFX, đổi vùng, không khung > 50 ms do âm thanh (TC-PERF-026, TC-PERF-009) — file: `src/game/tests/perf/test_audio_hub_perf.gd` — phải FAIL trước khi code
- [ ] T040 [P] [US7] Ma trận khối + pseudo-locale + lint theme cho tab Âm thanh (F137-AC-31); kiến trúc F137-AC-32 (logic thuần, telemetry không tên file/PII) (TC-I18N-006, TC-UI-003) — file: `src/game/tests/scene/test_grade_band_matrix.gd`, `src/game/tests/unit/audio/test_audio_boundaries.gd` — phải FAIL trước khi code

### Implementation cho User Story 7

- [ ] T041 [US7] Cài đặt → Âm thanh: công tắc "Tiếng bước chân", tiếng mẫu khi chỉnh thanh (Theme F114, `compact` F125) — file: `src/game/scenes/ui/settings/settings_panel.gd`
- [ ] T042 [US7] Telemetry đếm `audio_missing`/`audio_decode_failed` theo `kind` (không PII) — file: `src/game/scripts/core/audio_director.gd`

## Phase 10: Polish & Cross-Cutting

- [ ] T043 [P] Ảnh tab Âm thanh 4 khối × 1280×720/844×390 `vi` (+ `en` `g4_5`), xem từng ảnh — file: `specs/137-game-audio/screenshots/`
- [ ] T044 [P] Cập nhật tài liệu sở hữu nếu triển khai đổi chi tiết (STUDENT_GAME_UX §8a, GODOT_ARCHITECTURE §10.3, WORLD_RENDERING §8.4, PERFORMANCE_BUDGET §8/§11, LEARNING_EXPERIENCE §9.1), CHANGELOG — file: `docs/05-ux/STUDENT_GAME_UX.md`, `CHANGELOG.md`
- [ ] T045 Chạy `make godot-test`, `node --test src/game/tools/audio/tests/`, `make godot-export-check` — file: `src/game/`
- [ ] T046 Chạy [quickstart.md](quickstart.md) §2 với backend thật, đo FPS lab (SC-005), ghi bảng ngân sách — file: `specs/137-game-audio/quickstart.md`
- [ ] T047 Owner nghe duyệt danh sách âm thanh (SC-006, F137-1); ghi kết quả vào manifest (`review`) và quickstart §5 — file: `specs/137-game-audio/quickstart.md`, `src/game/assets/audio/audio_manifest.json`
- [ ] T048 Cập nhật dòng F137 trong CODE_PLAN và ghi bằng chứng (lệnh, số pass/fail) vào tasks.md — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`, `specs/137-game-audio/tasks.md`

## Dependencies & Execution Order

- F060/F062/F063/F064/F071/F073/F114 đã có; F133 cho đo ngân sách Web.
- Setup (T001–T003) → Foundational (T004–T007) → US1 (MVP) và US3 song song (US3 chặn việc commit asset ngoài: T012 chỉ commit sau khi T022–T024 xanh) → US2 (P1) → US4, US5 (P2, độc lập nhau) → US6 (P3) → US7 → Polish.
- T012 (người thật tải asset) chạy song song với code; mọi story kiểm được bằng file test MIT + tiếng dự phòng trước khi có asset thật.
- Trong mỗi story: test (FAIL) → code → test pass.

## Definition of Done

- [ ] Mọi tiêu chí F137-AC-01…F137-AC-32 có test pass (bảng AC → test ở plan.md) và ghi mã test/TC
- [ ] `make godot-test`, `node --test src/game/tools/audio/tests/`, job `audio-lint` và `make godot-export-check` xanh; ngân sách đạt
- [ ] Mọi file âm thanh có bản ghi manifest CC0-1.0/MIT với sha256 khớp và `review.calm`; owner đã nghe duyệt (SC-006)
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17); kịch bản quickstart §2 chạy với backend thật
- [ ] Tài liệu sở hữu, CHANGELOG cập nhật; trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
