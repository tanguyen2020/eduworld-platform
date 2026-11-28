# Research: F137 — Âm thanh game: kế hoạch nguồn asset CC0/MIT

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Quy tắc: [ADR-0106](../../docs/adr/ADR-0106-am-thanh-game-phu-hop-giao-duc-va-nguon-cc0-mit.md), [WORLD_RENDERING §8.4](../../docs/08-game-client/WORLD_RENDERING.md#84-âm-thanh-nguồn-manifest-lint-f137)

> **Ứng viên, chưa xác minh.** Agent không tải asset (ADR-0102 mục 6). Mọi mục dưới đây là nơi để người được owner giao tìm; license phải được kiểm **trên trang của từng file/pack lúc tải** và ghi vào `audio_manifest.json`. Không mục nào được coi là đã đạt CC0/MIT chỉ vì có trong danh sách này.

## R1. Âm thanh UI, phản hồi học tập, stinger

| Ứng viên | Nơi tìm | License cần thấy | Dùng cho | Ghi chú |
|---|---|---|---|---|
| Kenney — "Interface Sounds", "UI Audio" | `https://kenney.nl/assets` (lọc Audio) | CC0-1.0 (ghi trên trang pack và `License.txt` trong gói) | `ui_tap`, `ui_open`, `ui_close`, `ui_tab`, `ui_error`, `ui_success`, `ui_notify` | Chọn tiếng mềm; bỏ tiếng "buzz"/còi |
| Kenney — "Digital Audio", "Impact Sounds" | như trên | CC0-1.0 | `boss_hit` (tiếng "bụp" mềm), `saved`, `xp_tick` | Bỏ tiếng va chạm mạnh, tiếng vũ khí |
| Kenney — "Music Jingles" | như trên | CC0-1.0 | `quest_complete`, `reward`, `level_up`, `match_win`, `match_lose_gentle`, `streak` | Chọn jingle ≤ 3 s, giọng điệu vui/nhẹ; không dùng các pack chủ đề casino |
| Tự tổng hợp (first-party) | Công thức trong `src/game/tools/audio/synth/` (giống `minigame_feedback_sfx.gd`) | MIT | Dự phòng cho mọi tiếng UI/phản hồi | Tái lập, không phụ thuộc nguồn ngoài |

## R2. Nhạc nền không lời và ambience

| Ứng viên | Nơi tìm | License cần thấy | Dùng cho | Ghi chú |
|---|---|---|---|---|
| OpenGameArt — Music, lọc license CC0 | `https://opengameart.org/art-search-advanced` (Art Type = Music, License = CC0) | CC0-1.0 trên trang của **từng** bài | `menu`, `hub`, `zone.*`, `arena`, `boss`, `learning` | Chọn bài 70–100 BPM, nhạc cụ mộc, không lời, có điểm lặp; nhiều tác giả trên OGA dùng CC-BY → loại |
| OpenGameArt — Sound Effects/ambience, lọc CC0 | như trên (Art Type = Sound Effect) | CC0-1.0 | Ambience vùng (lá, suối, sóng, gió, chim, chợ xa, chuông xa) | Ưu tiên bản ghi dài ≥ 30 s lặp được |
| Freesound — chỉ mục gắn "Creative Commons 0" | `https://freesound.org` (bộ lọc License = Creative Commons 0) | CC0-1.0 | Ambience, bước chân | Mục "Attribution"/"Noncommercial" → loại |
| Kenney — "RPG Audio" | `https://kenney.nl/assets` | CC0-1.0 | Bước chân (đất, gỗ, cỏ), sột soạt khi mặc đồ | |
| Tự sáng tác/tổng hợp | Công cụ do người được giao chọn; file xuất phát hành MIT | MIT | Bù khi thiếu bài CC0 phù hợp | Ghi `author: EduWorld`, `source_url: first-party` |

## R3. Nguồn đã loại

| Nguồn | Lý do loại |
|---|---|
| Nhạc/âm thanh CC-BY (gồm nhiều bài phổ biến cho game indie) | ADR-0106 chỉ CC0-1.0/MIT |
| Thư viện "royalty-free"/license riêng của nền tảng (không phải CC0) | Không thuộc danh sách; thường cấm phân phối lại file |
| Bundle tặng kèm sự kiện (license độc quyền) | Pack trả phí/EULA bị cấm (ADR-0102 mục 2) |
| Bản ghi "public domain" không có văn bản CC0 | ADR-0106 mục 4 |

## R4. Kỹ thuật

- **Ogg Vorbis cho mọi file**: Godot 4 phát `AudioStreamOggVorbis` stream trên Web/Android/iOS; một định dạng giúp lint và ngân sách đơn giản. SFX ngắn import với `loop = false`, nhạc/ambience `loop = true` có `loop_offset` từ catalog.
- **Loudness**: ffmpeg `loudnorm` hai lượt (đo → áp), đo lại `ebur128`; mục tiêu theo loại (`audio_loudness_targets`): nhạc thấp hơn SFX để tiếng phản hồi nghe rõ, ambience thấp nhất.
- **Ngân sách** (ước lượng, kiểm ở T040): nhạc 96 kbps ≈ 0,72 MB/phút; `menu` + `hub` mỗi bài ~2 phút ≈ 3 MB; 25 tiếng UI/phản hồi/stinger mono ≈ 0,5–1 MB → lõi ≈ 4 MB < `audio_core_budget` (6 MB). Mỗi vùng: một bài 2–3 phút (≈ 2 MB) + ambience 1 phút (≈ 0,7 MB) ≪ `audio_zone_budget` (8 MB).
- **Web**: AudioContext chỉ mở sau tương tác đầu tiên; shell F133 đã khởi động engine sau tương tác nên không cần nút bật âm thanh riêng.
