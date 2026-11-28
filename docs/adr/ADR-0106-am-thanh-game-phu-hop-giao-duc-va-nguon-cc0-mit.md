# ADR-0106 — Âm thanh game phù hợp giáo dục, nguồn asset âm thanh chỉ CC0 hoặc MIT

> **Trạng thái:** Accepted · Supersedes một phần [ADR-0102](ADR-0102-nguon-asset-do-hoa-theo-license-mit-hoac-de-dai-hon.md) (câu "**Âm thanh**: áp dụng cùng danh sách được phép…" ở mục 5 và câu trả lời F067-2) · **Ngày:** 2026-11-28 · **OQ:** — · **Tài liệu:** [STUDENT_GAME_UX §8a](../05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) · [GODOT_ARCHITECTURE §10.3](../08-game-client/GODOT_ARCHITECTURE.md#103-âm-thanh-audiodirector-f137) · [WORLD_RENDERING §8.4](../08-game-client/WORLD_RENDERING.md#84-âm-thanh-nguồn-manifest-lint-f137) · [PERFORMANCE_BUDGET §8](../08-game-client/PERFORMANCE_BUDGET.md#8-ngân-sách-asset) · [LEARNING_EXPERIENCE §9.1](../02-learning/LEARNING_EXPERIENCE.md#91-âm-thanh-khi-học-và-khi-kiểm-tra) · [Chỉ mục ADR](README.md)

## Bối cảnh

Đến 2026-11-28 game gần như không có tiếng: `AudioDirector` (`src/game/scripts/core/audio_director.gd`) chỉ ánh xạ ba thanh âm lượng Nhạc nền / Hiệu ứng / Giọng nói vào bus `Music`/`SFX`/`Voice` và tắt bus nhạc khi bài nghe chạy (F066); tiếng đúng/sai của mini-game là PCM sinh bằng code (`minigame_feedback_sfx.gd`, F073); lồng tiếng `g1_3` và bài nghe có sẵn. Zone có `music_id` (`zone_root.gd`), `MiniGameConfig.presentation.background_music_id` có trong contract, nhưng repo không có file nhạc nào và không có chỗ phát. Không có âm thanh UI, âm thanh thế giới hay sự kiện.

Ngày 2026-11-28 owner yêu cầu: thêm âm thanh cho **mọi phần chưa có tiếng**, phù hợp giáo dục; asset **chỉ CC0 hoặc MIT**. ADR-0102 mục 5 trước đó áp danh sách license đồ họa (MIT, CC0-1.0, Public-Domain) cho âm thanh ở mức [Proposed] (Câu hỏi mở F067-2, chốt theo lệnh "chốt hết" ngày 2026-11-24).

## Quyết định

1. **Phạm vi âm thanh** [Confirmed — owner 2026-11-28]: nhạc nền theo vùng/màn (Main Hub, từng vùng đất, Arena/thi đấu, boss, menu; màn học tập nhạc nhẹ hoặc tắt khi có lồng tiếng) chuyển mượt khi đổi vùng; âm thanh UI (chạm, mở/đóng cửa sổ, đổi tab, lỗi, thông báo); âm thanh học tập và sự kiện (đúng/sai, chuỗi đúng, hoàn thành quest, nhận thưởng/XP, lên cấp, boss trúng đòn/bị hạ, bắt đầu/thắng/thua trận với giọng điệu động viên); ambience theo vùng; tiếng bước chân tùy chọn. Bảng âm thanh theo từng màn S01–S18 do [STUDENT_GAME_UX §8a](../05-ux/STUDENT_GAME_UX.md#8a-âm-thanh-trong-game) sở hữu.
2. **Quy tắc phù hợp giáo dục** [Confirmed — owner 2026-11-28]: nhạc nhịp chậm, **không lời**; không âm thanh gắt, đáng sợ, còi báo động hay đếm nhịp tạo áp lực; không vòng lặp gây chú ý, không âm thanh "jackpot" thưởng ngẫu nhiên (AGENTS R19); thua trận dùng giai điệu nhẹ kèm lời động viên. Nhạc **giảm** dưới lồng tiếng/đọc to và **tắt** trong bài kiểm tra có giờ, câu/thử thách có giờ — quy tắc sư phạm do [LEARNING_EXPERIENCE §9.1](../02-learning/LEARNING_EXPERIENCE.md#91-âm-thanh-khi-học-và-khi-kiểm-tra) sở hữu. "Giảm chuyển động" làm dịu stinger lớn. Ba thanh âm lượng và tắt tiếng của người chơi luôn được tôn trọng (không âm thanh nào bỏ qua bus). Mọi âm thanh có **tín hiệu hình tương đương** (icon, chữ hoặc hiệu ứng) để học sinh không có tai nghe/khiếm thính không thiếu thông tin (NFR-A11Y-001).
3. **Thông báo và chat không đổ chuông lặp** [Confirmed — owner "chốt hết", an toàn nhất theo R19]: tin chat không có âm thanh mặc định (giữ F112-AC-05); thông báo trong hộp thư game chỉ phát một tiếng chuông nhẹ **một lần** khi app đang mở và học sinh không ở bài kiểm tra, không nhắc lại; không phát âm thanh khi app ở nền.
4. **Danh sách license cho âm thanh** [Confirmed — owner 2026-11-28]: **chỉ `CC0-1.0` hoặc `MIT`**. `Public-Domain` không ghi CC0 rõ ràng **không** được nhận cho âm thanh (khó kiểm chứng nguồn của bản ghi âm); CC-BY, CC-BY-SA, NC, ND, GPL và pack trả phí bị cấm như ADR-0102 mục 2. Âm thanh tự làm (sinh bằng code, tổng hợp) phát hành MIT. Thay câu về âm thanh ở ADR-0102 mục 5 và câu trả lời F067-2.
5. **Không tải asset trong phiên của agent** (giữ ADR-0102 mục 6) [Confirmed]: kế hoạch nguồn liệt kê pack/URL ứng viên (ví dụ pack âm thanh CC0 của Kenney, mục OpenGameArt lọc theo CC0); **owner hoặc người được giao** tải, kiểm license **từng file**, rồi mới đưa vào repo kèm dòng manifest. Ứng viên trong tài liệu chưa phải license đã xác minh.
6. **Manifest và lint** [Confirmed — owner 2026-11-28]: mỗi file âm thanh có một bản ghi trong `src/game/assets/audio/audio_manifest.json` (đường dẫn, URL nguồn chính xác, tác giả, license, sha256, ngày tải, loudness đo được) và thư mục được phủ bởi một dòng trong `src/game/assets/LICENSES.md` (lint đồ họa `lint_art.gd` đã quét `.ogg`/`.wav`). Lint `lint_audio` trong CI chặn file thiếu bản ghi, license ngoài danh sách, sha256 lệch, sai định dạng, loudness/true peak ngoài mục tiêu. Chi tiết: [WORLD_RENDERING §8.4](../08-game-client/WORLD_RENDERING.md#84-âm-thanh-nguồn-manifest-lint-f137).
7. **Định dạng, loudness, ngân sách** [Confirmed — owner 2026-11-28; giá trị: owner "chốt hết"]: mọi file Ogg Vorbis (SFX mono, nhạc/ambience stereo); chuẩn hóa loudness bằng script tái lập (ffmpeg `loudnorm`) theo `audio_loudness_targets`; nhạc và ambience **stream**, nằm trong gói nội dung của vùng (tải theo vùng), lõi tải đầu Web chỉ chứa âm thanh UI, phản hồi học tập và nhạc Main Hub/menu trong `audio_core_budget`; mỗi gói vùng ≤ `audio_zone_budget`; tải đầu Web vẫn ≤ 40 MB ([PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md)).
8. **Dữ liệu, không logic trong scene** [Confirmed]: id nhạc/ambience là asset trình bày (không phải nội dung học tập, R2) khai báo ở zone scene (`music_id`, `ambience_id`) và `MiniGameConfig.presentation.background_music_id`; client ánh xạ id → file qua `AudioCatalog`; id lạ → giữ nhạc vùng (không lỗi). Không đổi contract backend. Scene chỉ phát intent âm thanh; `AudioDirector` quyết định phát (R1).

## Hệ quả

- (+) Game có tiếng ở mọi màn mà vẫn an toàn cho trẻ: không áp lực, không chuông lặp, không làm phiền khi kiểm tra, không mất thông tin khi tắt tiếng.
- (+) License kiểm được tự động (manifest + sha256), không phụ thuộc trí nhớ người review; loudness đồng đều giữa nguồn khác nhau.
- (+) Ngân sách Web giữ nguyên nhờ nhạc theo vùng và stream.
- (−) Nhạc CC0 không lời chất lượng tốt ít; có thể phải dùng ít bài, lặp dài, hoặc tự tổng hợp (MIT). Việc tải và kiểm license từng file cần người thật.
- (−) Việc phải làm: [F137](../../specs/137-game-audio/spec.md); cập nhật tài liệu sở hữu ở dòng Tài liệu; ADR-0102 ghi superseded một phần.

## Phương án đã loại

1. **Giữ danh sách đồ họa (gồm Public-Domain) cho âm thanh (F067-2):** owner chỉ cho CC0 hoặc MIT; bản ghi "public domain" không có văn bản CC0 khó kiểm chứng.
2. **Nhạc có lời / nhạc thịnh hành:** trái quy tắc giáo dục và gần như không có license CC0/MIT.
3. **Âm thanh thông báo chat mặc định bật:** tạo áp lực phản hồi (R19), trái F112-AC-05.
4. **Gộp toàn bộ nhạc vào bản tải đầu Web:** vượt ngân sách 40 MB.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-28 | Tạo ADR (Accepted) | Owner 2026-11-28: "thêm âm thanh cho mọi phần chưa có, phù hợp giáo dục; chỉ CC0 hoặc MIT"; F137 |
