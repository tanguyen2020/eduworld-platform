# Data Model: F137 — Âm thanh game: nhạc theo vùng/màn, âm thanh UI và sự kiện, ambience, phù hợp giáo dục, asset CC0/MIT

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng cho database — F137 không thêm bảng, cột hay contract server. Dữ liệu là file trong repo client và trạng thái trong RAM:

| Thực thể | Nơi lưu | Trường chính | Ghi chú |
|---|---|---|---|
| `AudioCatalogEntry` | `src/game/resources/audio/audio_catalog.json` | `id` (`^[a-z0-9_.]{1,64}$`, ví dụ `hub`, `zone.learning_zone.math`, `ui_tap`, `level_up`), `kind` (`music`/`ambience`/`sfx`/`ui`/`stinger`), `file` (`res://assets/audio/...ogg`), `bus` (`Music`/`SFX`/`Voice`), `bundle_id` (`core` hoặc `zone.<area_id>`), `loop`, `loop_offset_s`, `priority` (`feedback` > `stinger` > `ui`), `reduced_motion_file?` (biến thể ngắn), `visual_cue` (khóa mô tả tín hiệu hình, bắt buộc với `sfx`/`ui`/`stinger`) | Dữ liệu trình bày, không nội dung học tập (R2) |
| `AudioManifestRecord` | `src/game/assets/audio/audio_manifest.json` | `path`, `source_url`, `author`, `license` (`CC0-1.0`/`MIT`), `sha256`, `source_sha256`, `retrieved` (YYYY-MM-DD), `kind`, `loudness_lufs`, `true_peak_dbtp`, `duration_s`, `channels`, `review` (`calm`, `reviewer`, `date`) | Mọi chuỗi có giới hạn độ dài trong JSON Schema của lint (`maxLength`, R28) |
| `AudioContext` | RAM (`AudioDirector`) | `context_id`, `music_id`, `ambience_id` | Từ `AudioContextResolver` |
| `DuckStack` | RAM | tập `{reason → level_db}` (`voice` = `audio_voice_duck_db`, `listening`/`exam` = tắt, `stinger` = `audio_voice_duck_db` trong lúc stinger phát) | Mức hiệu lực = sâu nhất |
| `footsteps_enabled` | `user://settings.cfg` (khóa `audio.footsteps`) | bool; vắng → theo preset (`low` tắt) | Như các thiết lập cục bộ F114 |

## 2. Vòng đời / trạng thái

- Nhạc: `idle` → `playing(A)` → `crossfading(A→B)` (`audio_music_crossfade`) → `playing(B)`; `muted` khi DuckStack có `listening`/`exam` hoặc bus 0%; `paused` khi app ở nền → `fading_in` khi quay lại.
- Stream vùng: `not_loaded` (gói vùng chưa tải) → `loaded` khi vào vùng → `released` khi rời (trừ `core`).
- SFX pool: slot `free` → `busy` (tới hết clip) → `free`; đầy → thay slot có `priority` thấp nhất.

## 3. Quy tắc kiểm tra dữ liệu

- Mọi `file` của catalog có trong manifest; mọi file âm thanh trong repo có đúng một bản ghi manifest; `sha256` khớp; `license` ∈ {`CC0-1.0`, `MIT`}; định dạng Ogg Vorbis.
- `kind = sfx`/`ui`: ≤ 1 s, mono; `stinger`: ≤ 3 s; `music`/`ambience`: stereo, `loop = true`.
- Loudness theo `audio_loudness_targets` (±1 LU), true peak ≤ −1 dBTP.
- Mọi `music_id`/`ambience_id` trong zone scene có trong catalog; mọi sự kiện `sfx`/`ui`/`stinger` có `visual_cue`.
- `review.calm = true` bắt buộc trước khi file được catalog tham chiếu.

## 4. Event / outbox

| Event | Vai trò | Ghi chú |
|---|---|---|
| — | Không áp dụng | Client không phát domain event; intent nội bộ `EventBus.audio_requested` không phải event NATS (R25) |
