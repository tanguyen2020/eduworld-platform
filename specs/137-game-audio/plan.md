# Implementation Plan: F137 — Âm thanh game: nhạc theo vùng/màn, âm thanh UI và sự kiện, ambience, phù hợp giáo dục, asset CC0/MIT

**Branch**: `137-game-audio` · **Date**: 2026-11-28 · **Spec**: [spec.md](spec.md)

## Summary

Mở rộng autoload `AudioDirector` từ "chỉ ánh xạ âm lượng" thành bộ điều phối âm thanh theo **ngữ cảnh**: `AudioCatalog` thuần đọc `resources/audio/audio_catalog.json` (id → file, `kind`, bus, `bundle_id`, `visual_cue`); hai player nhạc luân phiên crossfade theo `audio_music_crossfade`; ambience stream theo `ambience_id` của zone; `DuckStack` thuần gom lý do giảm/tắt (`voice`, `listening`, `exam`, `stinger`); SFX pool giới hạn `audio_sfx_max_voices`; stinger sự kiện có biến thể giảm chuyển động. Màn và component dùng chung (F114 `UiBuild`, activity player F063, Exam Zone F064, quest/boss/Arena, hộp thông báo F123) chỉ phát intent. Asset ngoài chỉ CC0-1.0/MIT, vào repo qua `audio_manifest.json` (sha256, loudness) được `tools/audio/lint_audio.mjs` kiểm trong CI; chuẩn hóa bằng `tools/audio/normalize.sh` (ffmpeg ghim phiên bản). Không đổi contract backend.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x của repo), static typing; Node 20 (lint), bash + ffmpeg (chuẩn hóa) |
| Primary Dependencies | `AudioDirector`, `Settings`, `AppState`, `EventBus`, `QualityManager` (F060, F062, F114); activity player + `MiniGameFeedbackSfx` (F063, F073); Exam Zone (F064); `VoiceOverPlayer`/"Đọc to" (F097); bài nghe `duck_music` (F066); hộp thông báo (F123); `scripts/godot/size`, job export Web (F133) |
| Storage | Không có dữ liệu bền mới; công tắc "Tiếng bước chân" lưu `user://settings.cfg` như thiết lập F114 |
| Testing | GdUnit4 headless: unit `AudioCatalog`, `DuckStack`, `AudioContextResolver`, `SfxPool`; scene test màn S01–S18 với spy `AudioDirector` + `FakeVoicePlayer`; test kiến trúc (`visual_cue`, không `AudioStreamPlayer` trong scene); `node --test` cho `lint_audio.mjs` với fixture; đo ngân sách trên export Web; perf thiết bị lab |
| Target Platform | Web, Android, iOS (ADR-0101) |
| Project Type | game client (`src/game/`) + công cụ asset (`src/game/tools/audio/`) |
| Performance Goals | ≥ 30 FPS tầm trung preset Trung bình (NFR-COMP-001, TC-PERF-009); tải đầu Web ≤ 40 MB (NFR-COMP-004) |
| Constraints | AGENTS R1, R2, R5, R14, R17, R19, R32; ADR-0102, ADR-0106, ADR-0047, ADR-0057 |
| Scale/Scope | ~25 sự kiện UI/phản hồi/stinger, ~16 bài nhạc (menu, hub, 12 vùng, arena, boss, learning — có thể dùng chung), ~12 ambience |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec Ready; ADR-0106 Accepted; không delta contract backend (id âm thanh là asset trình bày, `background_music_id` đã có trong `MiniGameConfig`); không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client; tầng client: `AudioCatalog`/`DuckStack`/`AudioContextResolver` thuần, `AudioDirector` là service, scene chỉ phát intent |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu tenant/học sinh mới; không gọi API mới |
| IV Server-authoritative | ✅ | Âm thanh đúng/sai, thưởng, lên cấp chỉ phát theo phản hồi server (F137-AC-21); không tính điểm ở client |
| V Xác thực, phân quyền | ✅ | Không endpoint mới; không đổi luồng xác thực |
| VI An toàn trẻ em | ✅ | R19: không âm thanh áp lực/cờ bạc/chuông lặp, chat không đổ chuông, không âm thanh khi app ở nền; nhạc không lời; bài kiểm tra im lặng; R14: telemetry chỉ số đếm |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc ở STUDENT_GAME_UX §8a, LEARNING_EXPERIENCE §9.1, GODOT_ARCHITECTURE §10.3, WORLD_RENDERING §8.4, PERFORMANCE_BUDGET §8/§11; tham số theo tên CONFIG_DEFAULTS (8 tham số mới); id âm thanh ở catalog dữ liệu, không trong scene |
| VIII UI tách logic, token | ✅ | Công tắc mới dùng `ToggleSwitch`/Theme F114; không token mới (nếu cần icon `footsteps` thì sửa `design/icons/game/` + `make design-tokens`) |
| IX Test có bằng chứng | ✅ | Mỗi F137-AC có test (bảng dưới), test viết trước và FAIL trước khi code; fixture/fake gắn nhãn TEST DOUBLE; file âm thanh test tự sinh MIT |

### AC → test

| AC | Test |
|---|---|
| F137-AC-01 | `tests/unit/audio/test_audio_catalog.gd` |
| F137-AC-02, AC-04 | `tests/unit/audio/test_audio_director_music.gd` |
| F137-AC-03 | `tests/scene/audio/test_screen_sound_table.gd` (bảng S01–S18 dạng dữ liệu, so với §8a) |
| F137-AC-05 | `tests/unit/audio/test_audio_director_streaming.gd` |
| F137-AC-06, AC-32 | `tests/unit/audio/test_audio_boundaries.gd` (quét `.tscn`/`.gd` không `AudioStreamPlayer` ngoài allowlist, không telemetry tên file) |
| F137-AC-07, AC-09 | `tests/unit/audio/test_duck_stack.gd`, `tests/scene/audio/test_voice_ducking.gd` |
| F137-AC-08 | `tests/scene/audio/test_exam_silence.gd` |
| F137-AC-10, AC-11 | `tests/unit/audio/test_audio_director_buses.gd` |
| F137-AC-12, AC-13…AC-16 | `src/game/tools/audio/tests/lint_audio.test.mjs` (fixture hợp lệ/không hợp lệ), job CI `audio-lint` |
| F137-AC-15 | `src/game/tools/audio/tests/normalize.test.sh` (chạy hai lần cùng sha256) |
| F137-AC-17 | `tests/scene/test_license_page.gd` (mở rộng F096) |
| F137-AC-18, AC-19 | `tests/scene/audio/test_ui_sounds.gd`, `tests/unit/audio/test_sfx_pool.gd` |
| F137-AC-20 | `tests/scene/audio/test_notification_chime.gd` |
| F137-AC-21, AC-26 | `tests/scene/audio/test_learning_feedback_sounds.gd` |
| F137-AC-22, AC-24, AC-25 | `tests/scene/audio/test_event_stingers.gd` |
| F137-AC-23 | `tests/unit/audio/test_visual_cue_coverage.gd` + `tests/scene/audio/test_muted_matrix.gd` |
| F137-AC-27, AC-28 | `tests/scene/audio/test_ambience_footsteps.gd` |
| F137-AC-29 | `scripts/godot/size` + `lint_audio.mjs --budget` trong job export Web |
| F137-AC-30 | `tests/perf/test_audio_hub_perf.gd` + đo thiết bị lab |
| F137-AC-31 | `tests/scene/test_grade_band_matrix.gd` (tab Âm thanh), `tests/scene/test_pseudo_locale.gd`, `tools/lint_theme_overrides.gd` |

## Project Structure

### Documentation (feature này)

```text
specs/137-game-audio/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
├── checklists/requirements.md
└── screenshots/                      # ảnh tab Âm thanh theo khối (T043)
```

### Source Code (repository)

```text
src/game/
├── scripts/core/
│   ├── audio_director.gd             # SỬA: ngữ cảnh nhạc, crossfade, ambience, DuckStack, SFX pool, stinger, nền/tiền cảnh
│   ├── audio_catalog.gd              # thuần: đọc audio_catalog.json, tra id, visual_cue
│   ├── audio_duck_stack.gd           # thuần: lý do giảm/tắt, mức sâu nhất
│   ├── audio_context_resolver.gd     # thuần: zone/màn/blocking_activity/time_limit → AudioContext
│   └── settings.gd                   # SỬA: footsteps_enabled (mặc định theo preset)
├── scripts/player/footstep_emitter.gd    # chỉ nhân vật người chơi, nhịp theo clip walk/run
├── scripts/learning/minigame_feedback_sfx.gd  # GIỮ: dự phòng khi catalog thiếu file
├── scenes/ui/components/             # SỬA: UiBuild/nút/Window/tab phát play_ui
├── scenes/ui/settings/settings_panel.gd  # SỬA: công tắc "Tiếng bước chân", tiếng mẫu khi chỉnh thanh
├── scenes/world/*.tscn               # SỬA dữ liệu: music_id, ambience_id
├── resources/audio/audio_catalog.json
├── assets/audio/{music,ambience,sfx,ui}/*.ogg, audio_manifest.json, licenses/
├── assets/LICENSES.md                # SỬA: dòng assets/audio/** → manifest
├── tools/audio/{normalize.sh,lint_audio.mjs,synth/*.gd,tests/}
├── locale/vi.po, en.po               # ui.settings.sound.*
└── tests/{unit/audio,scene/audio,perf,fakes}/
.github/workflows/godot.yml           # SỬA: job audio-lint (ffmpeg ghim phiên bản)
```

**Structure Decision**: Giữ một autoload (`AudioDirector`, #10 trong GODOT_ARCHITECTURE §4) làm điểm phát duy nhất để bảo đảm âm lượng/tắt tiếng/giảm nhạc áp mọi nơi; phần quyết định thuần (`AudioCatalog`, `DuckStack`, `AudioContextResolver`) tách ra để unit test không cần audio server. Catalog JSON (không `.tres`) để lint Node đọc được.

## Phase 0: Research

[research.md](research.md): nguồn ứng viên CC0/MIT (Kenney, OpenGameArt lọc CC0, Freesound CC0, tự tổng hợp MIT), nguồn đã loại, kỹ thuật Ogg/loudness/ngân sách ước lượng.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `AudioCatalog`, `DuckStack`, `AudioContextResolver`, `SfxPool` (chọn slot) | Thuần, không Node |
| application (client) | `AudioDirector` (autoload) | Signal thì quá khứ: `music_changed`, `ducking_changed` |
| infrastructure | `AudioStreamPlayer` ×2 (nhạc), ×1 (ambience), pool SFX; `AudioServer` bus | Chỉ trong `AudioDirector` và `FootstepEmitter` |
| presentation | Component F114, activity player, Exam Zone, quest/boss/Arena, hộp thông báo, Cài đặt | Chỉ phát intent (`EventBus.audio_requested(event)`), không giữ player |

### Tích hợp và phụ thuộc

- F066 `duck_music(reason, on)` giữ API, nội bộ chuyển sang `DuckStack` (lý do `listening`). F097 "Đọc to" và `VoiceOverPlayer` báo bắt đầu/kết thúc giọng (lý do `voice`). F064 `AppState.blocking_activity` + `time_limit_seconds` của activity/challenge → lý do `exam`. F073 `MiniGameFeedbackSfx` thành dự phòng. F123 hộp thông báo phát `ui_notify` qua `AudioDirector` theo F137-AC-20. F096 trang Giấy phép thêm mục Âm thanh. F133 job export đo ngân sách.
- Không cần thay đổi backend.

### Bảo mật, tenant, idempotency, audit

- Không thao tác ghi server, không dữ liệu học sinh mới. Telemetry chỉ số đếm (`audio_decode_failed`, `audio_missing`) theo cơ chế chung, không tên file theo người, không PII (R14).

### Quan sát

- Telemetry client: số lần thiếu file/giải mã lỗi theo `kind`; thời gian nạp nhạc vùng (p95) — không PII.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
