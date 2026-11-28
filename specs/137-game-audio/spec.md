# Feature Specification: F137 — Âm thanh game: nhạc theo vùng/màn, âm thanh UI và sự kiện, ambience, phù hợp giáo dục, asset CC0/MIT

**Feature Branch**: `137-game-audio` · **Created**: 2026-11-28 · **Status**: Ready · **Module**: `src/game/` (+ `src/game/tools/audio/`, `.github/workflows/godot.yml`) · **Mốc**: M3 · **Phụ thuộc**: F060, F062, F063, F064, F071, F073, F114, F133 (và F066 bài nghe `duck_music`, F097 "Đọc to", F110/F109 màn boss/Arena, F123 hộp thông báo)
**Input**: Owner 2026-11-28: "thêm âm thanh cho mọi phần chưa có, phù hợp giáo dục". Hiện trạng: bus `Music`/`SFX`/`Voice` (`resources/audio/bus_layout.tres`), `AudioDirector` chỉ ánh xạ âm lượng và tắt nhạc khi bài nghe chạy; tiếng đúng/sai mini-game sinh bằng code (`minigame_feedback_sfx.gd`); có lồng tiếng và bài nghe; zone có `music_id` nhưng không có file nhạc, không có chỗ phát; không có âm thanh UI, thế giới, sự kiện. Quyết định: [ADR-0106](../../docs/adr/ADR-0106-am-thanh-game-phu-hop-giao-duc-va-nguon-cc0-mit.md). FR nguồn: FR-GAME-020, FR-GAME-021, FR-GAME-010; NFR-A11Y-001, NFR-COMP-001, NFR-COMP-004.

## Clarifications

### Session 2026-11-28 (quyết định owner và orchestrator)

- Q: Phạm vi âm thanh? → A: Nhạc nền theo vùng/màn (Main Hub, từng vùng, Arena/thi đấu, boss, màn học tập nhẹ hoặc tắt khi có giọng, menu) có crossfade; âm thanh UI (chạm, mở/đóng cửa sổ, tab, lỗi, thông báo); âm thanh học tập/sự kiện (đúng/sai dùng lại hoặc nâng cấp, chuỗi đúng, quest xong, thưởng/XP, lên cấp, boss trúng đòn/bị hạ, bắt đầu/thắng/thua trận giọng điệu động viên); ambience theo vùng; bước chân tùy chọn. [Confirmed — owner 2026-11-28]
- Q: Quy tắc giáo dục? → A: Nhịp chậm, không lời; giảm nhạc dưới lồng tiếng/đọc to; tắt nhạc trong bài kiểm tra và câu có giờ (quy tắc ở [LEARNING_EXPERIENCE §9.1](../../docs/02-learning/LEARNING_EXPERIENCE.md#91-âm-thanh-khi-học-và-khi-kiểm-tra)); giảm chuyển động làm dịu stinger lớn; thanh âm lượng riêng và tắt tiếng được tôn trọng; mọi âm thanh có tín hiệu hình (NFR-A11Y-001); không gắt/dọa/áp lực/gây nghiện (R19). [Confirmed — owner 2026-11-28]
- Q: License asset? → A: Chỉ `CC0-1.0` hoặc `MIT`; manifest từng file (URL nguồn, tác giả, license, sha256) kiểm bằng lint CI; chuẩn hóa loudness; Ogg Vorbis; ngân sách Web tải đầu ≤ 40 MB, nhạc stream/tải theo vùng; tham số mới đăng ký CONFIG_DEFAULTS. [Confirmed — owner 2026-11-28]
- Q: `Public-Domain` (không ghi CC0) có được nhận như đồ họa (ADR-0102)? → A: Không — owner nói "chỉ CC0 hoặc MIT"; chọn đúng chữ và an toàn hơn; ADR-0106 thay câu âm thanh ở ADR-0102 mục 5 và câu trả lời F067-2. [Confirmed — owner "chốt hết"]
- Q: Ai tải asset? → A: Không phải agent (ADR-0102 mục 6 giữ nguyên): [research.md](research.md) liệt kê ứng viên; người được owner giao tải, kiểm license từng file, rồi đưa vào repo kèm manifest (task T012). Ứng viên chưa là license đã xác minh. [Confirmed — owner "chốt hết"]
- Q: Manifest đặt ở đâu? → A: `src/game/assets/audio/audio_manifest.json` (mỗi file một bản ghi có `sha256`) + một dòng glob `assets/audio/**` trong `src/game/assets/LICENSES.md` trỏ tới manifest (để `lint_art.gd` vốn quét `.ogg`/`.wav` vẫn đạt); lint riêng `tools/audio/lint_audio.mjs` kiểm sâu (license, sha256, định dạng, loudness, ngân sách). Chọn JSON vì cần trường máy đọc (sha256, LUFS) mà bảng Markdown của LICENSES.md không có. [Confirmed — owner "chốt hết"]
- Q: Thông báo và chat có âm thanh? → A: Tin chat **không** có âm thanh mặc định (giữ F112-AC-05); thông báo mới trong hộp thư (F123) một tiếng chuông nhẹ **một lần** khi app đang mở và không trong bài kiểm tra, không nhắc lại, không phát khi app ở nền. [Confirmed — owner "chốt hết", an toàn nhất theo R19]
- Q: Bước chân bật hay tắt? → A: Chỉ bước chân của chính nhân vật, rất nhỏ; mặc định bật ở preset Trung bình/Cao, tắt ở Thấp; có công tắc "Tiếng bước chân" ở Cài đặt → Âm thanh (lưu cục bộ như các thiết lập F114). Người chơi khác không phát bước chân (giảm tiếng ồn, tiết kiệm CPU trên Hub đông người). [Confirmed — owner "chốt hết"]
- Q: Có đổi backend/contract? → A: Không. Id nhạc/ambience là asset trình bày (không phải nội dung học tập, R2): zone scene khai `music_id`/`ambience_id`; mini-game dùng `MiniGameConfig.presentation.background_music_id` đã có trong contract; client ánh xạ id → file qua `AudioCatalog` (`resources/audio/audio_catalog.json`); id lạ → giữ nhạc vùng. [Confirmed — owner "chốt hết"]
- Q: Tiếng đúng/sai mini-game hiện sinh bằng code? → A: Thay bằng file trong catalog khi có, giữ `MiniGameFeedbackSfx` làm dự phòng (thiếu file → vẫn có tiếng). Mọi activity (không chỉ mini-game) dùng chung sự kiện `correct`/`try_again`/`saved`. [Confirmed — owner "chốt hết"]
- Q: Thời điểm "có giờ" xác định thế nào? → A: `AppState.blocking_activity` là bài kiểm tra/thi (F064) **hoặc** activity/thử thách đang mở có `time_limit_seconds` (contract hiện có, ví dụ `timed_challenge`); khi đó tắt nhạc và ambience, chỉ giữ âm thanh UI tối thiểu, không phát đúng/sai trong bài kiểm tra (bài kiểm tra không phản hồi đúng/sai). [Confirmed — owner "chốt hết"]
- Q: Web chặn tự phát âm thanh trước tương tác? → A: Nhạc `menu` chỉ bắt đầu sau lần chạm/nhấn đầu tiên (shell F133 đã chạy engine sau tương tác đầu); không có lớp "bấm để bật âm thanh" riêng. [Confirmed — owner "chốt hết"]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Nhạc nền theo vùng và màn, chuyển mượt (Priority: P1) 🎯 MVP

An (lớp 3) vào Quảng trường Tia Sáng nghe nhạc Hub nhẹ nhàng; đi qua cổng gió sang Vương quốc Số, nhạc chuyển dần sang bài của vùng Toán, không bị ngắt đột ngột; mở World map nhạc vẫn chạy.

**Why this priority**: Phần "không có tiếng" lớn nhất của game; mọi âm thanh khác dựa trên `AudioDirector` + `AudioCatalog` của story này.

**Independent Test**: Scene test với `AudioCatalog` fixture (file test MIT ngắn): vào `main_hub` → nhạc `hub`; `set_context("zone.learning_zone.math")` → crossfade trong `audio_music_crossfade`; vào lại cùng vùng → không phát lại từ đầu; id lạ → giữ nhạc cũ + WARN.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F137-AC-01 | game client PHẢI có `AudioCatalog` thuần đọc `res://resources/audio/audio_catalog.json` (id → file, `kind`, bus, `bundle_id`, `loop`, `visual_cue`) và PHẢI trả rỗng kèm log WARN (không lỗi, không crash) với id không có hoặc file chưa tải | FR-GAME-020 · [GODOT_ARCHITECTURE §10.3](../../docs/08-game-client/GODOT_ARCHITECTURE.md#103-âm-thanh-audiodirector-f137) · AGENTS R2 | TC-UI-015 |
| F137-AC-02 | KHI vào một zone, `AudioDirector` PHẢI phát nhạc theo `music_id` của zone scene (vùng chưa có bài → nhạc `hub`), và KHI đổi sang ngữ cảnh khác (zone, `arena`, `boss`, `learning`, `menu`, `login`) PHẢI crossfade giữa hai player trên bus `Music` trong `audio_music_crossfade`; cùng id PHẢI không khởi động lại bài | FR-GAME-020 · [STUDENT_GAME_UX §8a](../../docs/05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) | TC-UI-015 |
| F137-AC-03 | Bảng âm thanh theo màn S01–S18 của STUDENT_GAME_UX §8a PHẢI được hiện thực đúng ngữ cảnh: S01/S02 `menu` (chỉ bắt đầu sau tương tác đầu tiên trên Web), S03–S05/S08–S12/S14/S15 giữ nhạc vùng, S06 `boss`, S13 `arena`, S16/S18 `learning` rất nhẹ, S17 tắt khi video/bài nghe phát | FR-GAME-020 · [STUDENT_GAME_UX §8a](../../docs/05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) | TC-UI-015 |
| F137-AC-04 | KHI mini-game có `presentation.background_music_id` có trong catalog, `AudioDirector` PHẢI phát bài đó trong lúc chơi rồi trả về nhạc trước đó khi thoát; id không có trong catalog PHẢI giữ nhạc hiện tại | FR-GAME-020 · FR-GAME-006 | TC-UI-015 |
| F137-AC-05 | Nhạc và ambience PHẢI phát dạng stream (`AudioStreamOggVorbis`), nạp khi vào vùng và giải phóng khi rời; KHI file thuộc gói vùng chưa tải hoặc giải mã lỗi, game PHẢI chạy tiếp không tiếng cho nguồn đó (log WARN, telemetry đếm không PII) | FR-GAME-020 · NFR-COMP-004 · [PERFORMANCE_BUDGET §8](../../docs/08-game-client/PERFORMANCE_BUDGET.md#8-ngân-sách-asset) | TC-PERF-026 |
| F137-AC-06 | Scene PHẢI chỉ khai báo dữ liệu âm thanh (`music_id`, `ambience_id`) và phát intent (`EventBus.audio_requested`/service); PHẢI không có `AudioStreamPlayer` riêng trong scene UI hay zone (trừ `FootstepEmitter` của nhân vật) và không có quyết định phát/tắt trong scene | AGENTS R1 · [GODOT_ARCHITECTURE §10.3](../../docs/08-game-client/GODOT_ARCHITECTURE.md#103-âm-thanh-audiodirector-f137) | — |

---

### User Story 2 — Âm thanh không làm phiền việc học (Priority: P1)

Bình (lớp 2) bấm "Đọc to", nhạc nhỏ hẳn để em nghe rõ đề rồi to lại; Dũng (lớp 7) vào bài kiểm tra giữa kỳ, nhạc và tiếng khung cảnh tắt, không có tiếng tích tắc; kéo thanh "Nhạc nền" về 0% thì không còn nhạc ở bất kỳ màn nào.

**Why this priority**: Quy tắc giáo dục của owner và LEARNING_EXPERIENCE §9.1; âm thanh sai chỗ còn tệ hơn không có âm thanh.

**Independent Test**: `FakeVoicePlayer` phát giọng → bus nhạc giảm `audio_voice_duck_db`, trả lại sau; `AppState.blocking_activity = exam` → nhạc/ambience tắt; activity có `time_limit_seconds` → tắt; lồng nhiều lý do → chỉ trả nhạc khi hết lý do; thanh âm lượng 0% → không player nào phát; app vào nền → tạm dừng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F137-AC-07 | KHI lồng tiếng đề bài, "Đọc to", thoại NPC hoặc hướng dẫn của Mây phát trên bus `Voice`, `AudioDirector` PHẢI giảm bus `Music` (và ambience) `audio_voice_duck_db` trong ≤ 0,3 s và trả lại khi hết giọng; bài nghe/video PHẢI tắt nhạc như `duck_music` hiện có (F066) | FR-GAME-021 · [LEARNING_EXPERIENCE §9.1](../../docs/02-learning/LEARNING_EXPERIENCE.md#91-âm-thanh-khi-học-và-khi-kiểm-tra) | TC-UI-015 |
| F137-AC-08 | TRONG KHI `AppState.blocking_activity` là bài kiểm tra/thi hoặc activity/thử thách đang mở có `time_limit_seconds`, game client PHẢI tắt nhạc và ambience, PHẢI không phát âm thanh đúng/sai, tích tắc, cảnh báo hết giờ hay chuông thông báo, và chỉ giữ âm thanh UI tối thiểu (chạm, đã lưu đáp án) | FR-GAME-021 · [LEARNING_EXPERIENCE §9.1](../../docs/02-learning/LEARNING_EXPERIENCE.md#91-âm-thanh-khi-học-và-khi-kiểm-tra) · [STUDENT_GAME_UX_LEARNING §9](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md#9-bài-kiểm-tra-có-giờ) | TC-UI-015 |
| F137-AC-09 | Các lý do giảm/tắt nhạc (`voice`, `listening`, `exam`, stinger) PHẢI cộng dồn theo ngăn xếp: nhạc chỉ trở lại mức của Settings khi không còn lý do nào; mức giảm PHẢI là mức sâu nhất trong các lý do đang có | FR-GAME-021 | TC-UI-015 |
| F137-AC-10 | Mọi âm thanh PHẢI phát qua bus `Music`, `SFX` hoặc `Voice` theo `kind` của catalog; thanh âm lượng và tắt tiếng của Settings PHẢI có hiệu lực ngay (≤ 100 ms) với mọi nguồn đang phát; bus ở 0% PHẢI không phát và không giải mã nguồn mới | FR-GAME-021 · FR-GAME-010 · [STUDENT_GAME_UX_SOCIAL §10](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md#10-cài-đặt) | TC-UI-015 |
| F137-AC-11 | KHI app vào nền (tab Web ẩn, Android/iOS pause), game client PHẢI tạm dừng nhạc và ambience và PHẢI không phát âm thanh nào (kể cả chuông thông báo); KHI quay lại PHẢI tiếp tục bài đang dở bằng fade-in | FR-GAME-021 · AGENTS R19 | TC-UI-016 |
| F137-AC-12 | Mọi nhạc PHẢI không lời, nhịp chậm theo STUDENT_GAME_UX §8a; danh sách asset PHẢI được người review đánh dấu "không lời, không gắt/dọa" trong manifest (`review.calm = true`) trước khi vào catalog | FR-GAME-021 · AGENTS R19 · [STUDENT_GAME_UX §8a](../../docs/05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) | TC-UI-016 |

---

### User Story 3 — Asset chỉ CC0/MIT, có manifest, kiểm tự động (Priority: P1)

Người phụ trách tải pack âm thanh CC0 của Kenney, chạy script chuẩn hóa, thêm bản ghi manifest; CI báo đỏ khi một file `.ogg` thiếu bản ghi hay ghi license CC-BY.

**Why this priority**: Không có lint thì không được đưa asset ngoài vào repo (ADR-0106 mục 4–6); chặn trước mọi story có file âm thanh.

**Independent Test**: Thư mục fixture trong `tools/audio/tests/`: file thiếu bản ghi, license `CC-BY-4.0`, `Public-Domain`, sha256 lệch, `.wav`, loudness −10 LUFS, SFX 2 s → `lint_audio` exit 1 với từng file; bộ hợp lệ → exit 0.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F137-AC-13 | Mỗi file âm thanh dưới `src/game/` (trừ `tests/`, `addons/`) PHẢI có đúng một bản ghi trong `src/game/assets/audio/audio_manifest.json` gồm `path`, `source_url`, `author`, `license`, `sha256`, `source_sha256`, `retrieved`, `kind`, `loudness_lufs`, `true_peak_dbtp`, `duration_s`, `channels`, `review.calm`; `src/game/assets/LICENSES.md` PHẢI có dòng `assets/audio/**` trỏ manifest để `lint_art.gd` vẫn đạt | FR-GAME-021 · ADR-0106 · [WORLD_RENDERING §8.4](../../docs/08-game-client/WORLD_RENDERING.md#84-âm-thanh-nguồn-manifest-lint-f137) | TC-UI-017 |
| F137-AC-14 | `lint_audio` (CI job `godot.yml`) PHẢI fail và chỉ rõ file khi: thiếu bản ghi, `license` ngoài `CC0-1.0`/`MIT`, `sha256` không khớp, định dạng khác Ogg Vorbis, SFX/UI > 1 s hoặc stinger > 3 s, loudness lệch `audio_loudness_targets` của `kind` quá ±1 LU, true peak > −1 dBTP, catalog trỏ file không có trong manifest, hoặc `music_id`/`ambience_id` của zone scene không có trong catalog | FR-GAME-021 · ADR-0106 | TC-UI-017 |
| F137-AC-15 | Script `tools/audio/normalize.sh` PHẢI chuẩn hóa tái lập (ffmpeg ghim phiên bản trong image CI, `loudnorm` hai lượt theo `kind`, `libvorbis`, SFX/UI mono, nhạc/ambience stereo `audio_music_bitrate`) và ghi số đo vào manifest; chạy lại trên cùng file nguồn PHẢI ra cùng `sha256` | FR-GAME-021 · [WORLD_RENDERING §8.4](../../docs/08-game-client/WORLD_RENDERING.md#84-âm-thanh-nguồn-manifest-lint-f137) | TC-UI-017 |
| F137-AC-16 | Không file âm thanh ngoài nào được commit nếu chưa có bằng chứng kiểm license từng file (link trang license, văn bản license gốc khi pack kèm theo trong `assets/audio/licenses/`); agent PHẢI không tải asset — việc tải do người được owner giao (ADR-0102 mục 6) | ADR-0106 · ADR-0102 | TC-UI-017 |
| F137-AC-17 | Trang "Giấy phép" trong Cài đặt (F096) PHẢI có mục "Âm thanh" sinh từ manifest khi build (tác giả, pack, license), không gõ tay | ADR-0106 · NFR-MAINT-007 | — |

---

### User Story 4 — Âm thanh giao diện (Priority: P2)

Chi (lớp 5) chạm nút nghe tiếng "tách" nhẹ, mở Kho đồ có tiếng mở nhẹ, đóng có tiếng ngược lại; nhập sai mật khẩu nghe hai nốt trầm nhẹ chứ không phải tiếng còi.

**Why this priority**: Làm game "có hồn" và phản hồi thao tác rõ hơn, nhất là trên điện thoại; không chặn học tập.

**Independent Test**: Scene test `UiBuild` button/`Window` mở/đóng/tab/lỗi → spy `AudioDirector.play_ui` đúng sự kiện; nút disabled → không tiếng; 20 chạm liên tiếp → không vượt `audio_sfx_max_voices`, không chồng trong 80 ms.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F137-AC-18 | KHI học sinh chạm nút (tầng 1 của F114), mở/đóng cửa sổ, đổi tab, gặp lỗi (thông điệp lỗi) hoặc lưu thành công, game client PHẢI phát âm thanh UI tương ứng (`ui_tap`, `ui_open`, `ui_close`, `ui_tab`, `ui_error`, `ui_success`) qua thành phần dùng chung (`UiBuild`/component F114), không qua từng scene; nút bị mờ PHẢI không phát | FR-GAME-020 · [STUDENT_GAME_UX §8a](../../docs/05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) | TC-UI-015 |
| F137-AC-19 | Tiếng lỗi PHẢI là âm nhẹ (không buzzer, không còi) và luôn đi kèm thông điệp chữ; tiếng UI PHẢI ≤ 0,3 s, không chồng cùng sự kiện trong 80 ms, và SFX pool PHẢI giới hạn `audio_sfx_max_voices` (bỏ tiếng UI trước tiếng phản hồi học tập khi đầy) | FR-GAME-021 · AGENTS R19 · NFR-A11Y-001 | TC-UI-016 |
| F137-AC-20 | Tin chat PHẢI không có âm thanh mặc định (F112-AC-05); KHI có thông báo mới trong hộp thư (F123) trong lúc app mở và không trong bài kiểm tra, game client PHẢI phát `ui_notify` **một lần** cùng badge/toast và PHẢI không lặp, không nhắc lại; KHI công tắc "Hiện thông báo nhỏ khi đang chơi" tắt, PHẢI không phát | FR-GAME-021 · AGENTS R19 · [STUDENT_GAME_UX_SOCIAL §10](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md#10-cài-đặt) | TC-UI-016 |

---

### User Story 5 — Âm thanh học tập và sự kiện, luôn có tín hiệu hình (Priority: P2)

Em (lớp 4) trả lời đúng nghe "ting" vui và thấy ✓; sai nghe tiếng nhẹ "thử lại nhé" và thấy gợi ý; lên cấp có đoạn nhạc ngắn; thua trận Arena nghe giai điệu nhẹ cùng lời "Lần sau mình thử lại nhé!". Bạn khiếm thính chơi tắt tiếng vẫn thấy đủ thông tin.

**Why this priority**: Phản hồi học tập bằng âm thanh tăng hứng thú (owner), nhưng phải an toàn (R19) và không là tín hiệu duy nhất.

**Independent Test**: Ma trận sự kiện (`correct`, `try_again`, `saved`, `streak`, `quest_complete`, `reward`, `xp_tick`, `level_up`, `boss_hit`, `boss_defeated`, `match_start`, `match_win`, `match_lose_gentle`) → spy âm thanh + kiểm `visual_cue` hiện; bật giảm chuyển động → gain stinger giảm; tắt cả 3 bus → màn vẫn hiện đủ tín hiệu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F137-AC-21 | KHI server trả kết quả chấm một câu (practice/quest/challenge/mini-game, không phải bài kiểm tra), game client PHẢI phát `correct` hoặc `try_again` (nhẹ, không trầm nặng, không mang nghĩa phạt) cùng ✓/✗ + chữ; kết quả chỉ dựa trên phản hồi server (R5); KHI ≥ 3 câu đúng liên tiếp PHẢI phát `streak` một lần cho chuỗi đó | FR-GAME-021 · FR-EXR-004 · [LEARNING_EXPERIENCE §4](../../docs/02-learning/LEARNING_EXPERIENCE.md#4-phản-hồi-đúngsai-thân-thiện) · AGENTS R5 | TC-UI-016 |
| F137-AC-22 | KHI hoàn thành quest, nhận thưởng/XP, lên cấp, boss trúng đòn/bị hạ, bắt đầu/thắng/thua trận, game client PHẢI phát stinger tương ứng (`quest_complete`, `reward`, `xp_tick` ≤ 1 s, `level_up` ≤ 2 s, `boss_hit`, `boss_defeated`, `match_start`, `match_win`, `match_lose_gentle`) và nhạc PHẢI giảm tạm trong lúc stinger; `match_start` PHẢI không có tiếng đếm ngược tích tắc; `match_lose_gentle` PHẢI đi cùng lời động viên | FR-GAME-020 · FR-GAME-021 · AGENTS R19 · [STUDENT_GAME_UX §8a](../../docs/05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) | TC-UI-016 |
| F137-AC-23 | Mỗi sự kiện trong `audio_catalog.json` PHẢI có `visual_cue` (icon/chữ/hiệu ứng đã có trên màn) và test kiến trúc PHẢI fail nếu sự kiện thiếu; KHI cả ba bus ở 0% hoặc thiết bị không có đầu ra âm thanh, mọi màn PHẢI hiển thị đủ thông tin như khi có tiếng | NFR-A11Y-001 · FR-GAME-021 · [STUDENT_GAME_UX §8](../../docs/05-ux/STUDENT_GAME_UX.md#8-trợ-năng) | TC-UI-016 |
| F137-AC-24 | TRONG KHI "Giảm chuyển động" bật, `AudioDirector` PHẢI phát stinger lớn (`level_up`, `boss_defeated`, `match_win`, `quest_complete`, `reward`) với gain `audio_reduced_motion_stinger_gain` và chỉ phần nốt ngắn (bỏ lớp fanfare dài) | FR-GAME-021 · [STUDENT_GAME_UX §8a](../../docs/05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) | TC-UI-016 |
| F137-AC-25 | Âm thanh mua/nhận Xu PHẢI là nốt nhẹ; PHẢI không có âm thanh kiểu máy đánh bạc, quay thưởng, "jackpot", tiếng tăng dần thúc giục làm nhanh hơn hay âm thanh lặp gọi học sinh quay lại | AGENTS R19 · NFR-SAFE-003 · [CHILD_SAFETY §5](../../docs/10-security/CHILD_SAFETY.md#5-không-dark-pattern-không-cơ-chế-cờ-bạc) | TC-UI-016 |
| F137-AC-26 | KHI catalog chưa có file cho `correct`/`try_again`/`saved`, game client PHẢI dùng `MiniGameFeedbackSfx` (F073) làm dự phòng để phản hồi học tập luôn có tiếng | FR-GAME-020 · F073-AC-24 | TC-UI-015 |

---

### User Story 6 — Ambience theo vùng và tiếng bước chân (Priority: P3)

Ở Rừng Ngôn Ngữ có tiếng lá và suối rất nhỏ; ở Đảo Khoa Học có tiếng sóng; khi chạy trên đường làng nghe bước chân nhẹ của chính mình.

**Why this priority**: Tăng cảm giác thế giới sống; ít quan trọng hơn nhạc/phản hồi; có thể làm sau.

**Independent Test**: Zone fixture có `ambience_id` → ambience lặp trên bus `SFX` theo `audio_loudness_targets` ambience; đổi vùng → crossfade ambience; preset Thấp → bước chân tắt; công tắc "Tiếng bước chân" tắt → không phát; nhân vật người khác → không bước chân.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F137-AC-27 | KHI vào zone có `ambience_id`, `AudioDirector` PHẢI phát ambience lặp (stream) và crossfade khi đổi vùng; ambience PHẢI tắt trong bài kiểm tra/có giờ (F137-AC-08) và giảm dưới giọng (F137-AC-07) | FR-GAME-020 · [STUDENT_GAME_UX §8a](../../docs/05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) | TC-UI-015 |
| F137-AC-28 | `FootstepEmitter` PHẢI chỉ phát bước chân của nhân vật người chơi đang điều khiển theo nhịp clip `walk`/`run` (không người chơi khác, không NPC), rất nhỏ; mặc định bật ở preset Trung bình/Cao, tắt ở Thấp; công tắc "Tiếng bước chân" ở Cài đặt → Âm thanh PHẢI có hiệu lực ngay và lưu cục bộ | FR-GAME-020 · [STUDENT_GAME_UX_SOCIAL §10](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md#10-cài-đặt) · [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-UI-015 |

---

### User Story 7 — Ngân sách, hiệu năng, Cài đặt, ranh giới (Priority: P2)

**Why this priority**: Bắt buộc theo constitution (VII, VIII, IX) và ngân sách Web/máy tầm trung; kiểm sau luồng chính.

**Independent Test**: `lint_audio` + `scripts/godot/size` trên export Web; profiler Android tầm trung 10 phút Hub; ma trận khối màn Cài đặt có công tắc mới; quét mã.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F137-AC-29 | Âm thanh trong lõi tải đầu Web (UI, phản hồi học tập, stinger, nhạc `menu` + `hub`) PHẢI ≤ `audio_core_budget` và tải đầu Web PHẢI vẫn ≤ 40 MB; nhạc + ambience mỗi gói vùng PHẢI ≤ `audio_zone_budget`; CI PHẢI chặn khi vượt | NFR-COMP-004 · [PERFORMANCE_BUDGET §11](../../docs/08-game-client/PERFORMANCE_BUDGET.md#11-thời-gian-tải-và-dung-lượng) | TC-PERF-026 |
| F137-AC-30 | Với nhạc + ambience + SFX chạy, Main Hub PHẢI giữ ≥ 30 FPS trên Android tầm trung preset Trung bình (TC-PERF-009) và crossfade/đổi vùng PHẢI không gây khung > 50 ms do âm thanh | NFR-COMP-001 · [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-026, TC-PERF-009 |
| F137-AC-31 | Tab Âm thanh của Cài đặt PHẢI thêm công tắc "Tiếng bước chân" (`ToggleSwitch` có chữ Bật/Tắt) và phát tiếng mẫu của bus khi chỉnh thanh; chuỗi mới qua khóa `ui.settings.sound.*` (vi/en), Theme/token F114 (không hardcode style), bố cục chịu pseudo-locale và chế độ `compact` (F125) | FR-GAME-010 · NFR-I18N-001 · AGENTS R32 · F125-AC-03 | TC-I18N-006, TC-UI-003 |
| F137-AC-32 | Logic chọn/giảm/tắt âm thanh PHẢI nằm trong `AudioDirector`/`AudioCatalog` (test được không cần Node âm thanh thật); client PHẢI không gửi telemetry chứa tên file theo người, chỉ số đếm (lỗi giải mã, file thiếu) không PII; mock/fixture PHẢI gắn nhãn TEST DOUBLE và file âm thanh test PHẢI là file tự sinh MIT trong `tests/` | AGENTS R1 · AGENTS R14 · AGENTS R17 · FR-GAME-011 | — |

### Edge Cases

- NẾU học sinh đổi vùng trong lúc lồng tiếng phát, THÌ crossfade vẫn chạy nhưng ở mức đã giảm (F137-AC-09).
- NẾU bài kiểm tra bắt đầu khi stinger đang phát, THÌ cắt stinger bằng fade 0,2 s rồi tắt nhạc (F137-AC-08).
- NẾU gói vùng tải xong khi học sinh đã ở vùng đó, THÌ nhạc vùng fade-in thay nhạc `hub` dự phòng.
- NẾU thiết bị không có đầu ra âm thanh (phòng máy), THÌ không báo lỗi, chỉ dùng tín hiệu hình (LEARNING_EXPERIENCE §5.1).
- NẾU người chơi bấm liên tục một nút, THÌ chỉ một tiếng mỗi 80 ms (F137-AC-19).
- NẾU tab Web bị ẩn trong lúc nhận thông báo, THÌ không phát chuông khi quay lại (F137-AC-11, F137-AC-20).
- NẾU `background_music_id` của mini-game trùng bài đang phát, THÌ không khởi động lại (F137-AC-02).
- NẾU manifest có file đã xóa khỏi repo, THÌ lint báo bản ghi thừa (F137-AC-14).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-020 | Nhạc theo vùng/màn có crossfade, âm thanh UI, âm thanh học tập/sự kiện, ambience, bước chân | [STUDENT_GAME_UX §8a](../../docs/05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game), [GODOT_ARCHITECTURE §10.3](../../docs/08-game-client/GODOT_ARCHITECTURE.md#103-âm-thanh-audiodirector-f137) |
| FR-GAME-021 | Quy tắc giáo dục (không lời, giảm dưới giọng, tắt khi kiểm tra/có giờ, giảm chuyển động, âm lượng/tắt tiếng, tín hiệu hình), asset CC0/MIT + manifest + lint + loudness + ngân sách | [LEARNING_EXPERIENCE §9.1](../../docs/02-learning/LEARNING_EXPERIENCE.md#91-âm-thanh-khi-học-và-khi-kiểm-tra), [WORLD_RENDERING §8.4](../../docs/08-game-client/WORLD_RENDERING.md#84-âm-thanh-nguồn-manifest-lint-f137), [PERFORMANCE_BUDGET §8, §11](../../docs/08-game-client/PERFORMANCE_BUDGET.md#8-ngân-sách-asset) |
| FR-GAME-010 (phần âm thanh) | Thiết lập âm thanh theo nhóm tuổi trong Cài đặt (công tắc bước chân, tiếng mẫu) | [STUDENT_GAME_UX_SOCIAL §10](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md#10-cài-đặt) |

NFR: NFR-A11Y-001, NFR-COMP-001, NFR-COMP-004, NFR-I18N-001, NFR-SAFE-003, NFR-MAINT-007.

### Key Entities

- **AudioCatalogEntry**: id ngữ cảnh/sự kiện → file, `kind`, bus, `bundle_id`, `loop`, `visual_cue`.
- **AudioManifestRecord**: một file âm thanh trong repo với nguồn, tác giả, license, sha256, số đo loudness, cờ review.
- **AudioContext**: ngữ cảnh nhạc hiện hành (`login`, `menu`, `hub`, `zone.<area_id>`, `arena`, `boss`, `learning`, `exam`).
- **DuckStack**: tập lý do giảm/tắt nhạc đang hiệu lực (`voice`, `listening`, `exam`, `stinger`).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `audio_music_crossfade` | Thời gian chuyển nhạc giữa ngữ cảnh |
| `audio_voice_duck_db` | Mức giảm nhạc/ambience dưới giọng |
| `audio_loudness_targets` | Mục tiêu LUFS/true peak theo `kind` (chuẩn hóa + lint) |
| `audio_core_budget`, `audio_zone_budget` | Ngân sách dung lượng âm thanh trong lõi Web và mỗi gói vùng |
| `audio_music_bitrate` | Bitrate mã hóa nhạc/ambience |
| `audio_sfx_max_voices` | Số tiếng SFX đồng thời |
| `audio_reduced_motion_stinger_gain` | Gain stinger khi giảm chuyển động |
| `quality_presets` | Bật/tắt bước chân mặc định theo preset |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không logic trong scene; scene chỉ khai dữ liệu và phát intent | AGENTS R1 |
| Id âm thanh là asset trình bày, không nội dung học tập; không đổi contract backend | AGENTS R2 · ADR-0106 mục 8 |
| Không âm thanh áp lực/gây nghiện/cờ bạc; chat không đổ chuông | AGENTS R19 · NFR-SAFE-003 |
| Asset chỉ CC0-1.0/MIT, manifest + lint; agent không tải asset | ADR-0106 · ADR-0102 mục 6 |
| Mọi âm thanh có tín hiệu hình | NFR-A11Y-001 |
| Style lấy từ Theme/token | AGENTS R32 · ADR-0047 |
| GDScript static typing, test GdUnit4 | AGENTS §7 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% màn S01–S18 và bài kiểm tra có giờ khớp bảng âm thanh của STUDENT_GAME_UX §8a trong scene test.
- **SC-002**: 0 file âm thanh trong repo thiếu bản ghi manifest hoặc license ngoài CC0-1.0/MIT (lint CI xanh).
- **SC-003**: 0 sự kiện âm thanh thiếu tín hiệu hình (test kiến trúc).
- **SC-004**: Tải đầu Web ≤ 40 MB và âm thanh lõi ≤ `audio_core_budget` trên bản export.
- **SC-005**: Main Hub ≥ 30 FPS trên Android tầm trung tham chiếu preset Trung bình với âm thanh đầy đủ trong 10 phút.
- **SC-006**: Owner nghe thử và duyệt danh sách nhạc/âm thanh (bản ghi `review.calm`) trước khi chuyển Done.

## Ngoài phạm vi

- Lồng tiếng nội dung học tập, TTS, bài nghe → đã có (F025, F066, F097); F137 chỉ giảm/tắt nhạc quanh chúng.
- Voice chat: không có (ADR-0070). Âm thanh trong Admin Portal: không có.
- Nhạc theo sự kiện/season do Game Studio cấu hình (thêm trường contract) → chưa làm; nếu cần, feature sau sửa contract `game` (câu hỏi mở F137-2).
- Âm thanh 3D theo vị trí cho người chơi khác, emote có tiếng → chưa làm (câu hỏi mở F137-3).

## Assumptions

- F060, F062, F063, F064, F071, F073, F114 đã có `AudioDirector`, `Settings`, zone scene, activity player, Exam Zone, mini-game và tab Âm thanh; F133 có `scripts/godot/size` và job export Web để đo ngân sách.
- Có người được owner giao để tải và kiểm license pack âm thanh (T012); nếu chưa có, story US1–US2, US4–US7 vẫn kiểm được bằng file test MIT tự sinh và tiếng dự phòng, nhưng F137 chưa Done tới khi có asset thật được owner duyệt (SC-006).
- Câu hỏi mở cho owner: F137-1 duyệt danh sách nhạc cuối cùng (nghe thử); F137-2 nhạc riêng cho sự kiện/season; F137-3 âm thanh 3D của người chơi khác/emote.
