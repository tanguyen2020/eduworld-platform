# Audio & Language Learning

> Trạng thái: Draft · Phiên bản: 0.4 · Owner: EdTech Specialist (Tiếng Anh, Tiếng Việt) + Identity & Security Engineer · Cập nhật: 2026-11-16
> Liên quan: [ACTIVITY_ENGINE](ACTIVITY_ENGINE.md) · [VIDEO_LEARNING](VIDEO_LEARNING.md) · [CONTENT_MODEL §8.1](CONTENT_MODEL.md#81-audio-đọc-đề-và-hướng-dẫn-cho-nhóm-g1_3-confirmed) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [ADR-0044](../adr/ADR-0044-luu-tru-hinh-anh-va-media-object-storage-s3-compatible-cdn.md) · [ADR-0059](../adr/ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md) · [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) · [ADR-0082](../adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md) · Schema: [`audio-lesson.schema.json`](../../specs/schemas/activities/audio-lesson.schema.json)

## 1. Mục đích và phạm vi

| `activity_type` | Gồm | Nhãn |
|---|---|---|
| `audio_lesson` | Bài nghe, nghe hiểu (listening) có câu hỏi tự chấm, nghe chép (dictation) chấm bằng đáp án chấp nhận | [Confirmed] Master Prompt mới §15.1 |

Version này chỉ có hoạt động nghe hiểu **server tự chấm**. Không có ghi âm giọng nói học sinh, không có AI giọng nói (speech-to-text, chấm phát âm tự động) [Confirmed, ADR-0059, ADR-0072]. Bài nói (`speaking`) ngoài version này (ADR-0082).

Audio đọc đề và hướng dẫn cho lớp 1–3 (không phải hoạt động nghe hiểu) là yêu cầu nội dung, quy định ở [CONTENT_MODEL §8.1](CONTENT_MODEL.md#81-audio-đọc-đề-và-hướng-dẫn-cho-nhóm-g1_3-confirmed).

## 2. `audio_lesson`

| Trường (`content`) | Mô tả |
|---|---|
| `audio_asset_id` | Ogg Vorbis cho Godot, MP3 cho web (ADR-0044) |
| `transcript_asset_id` | Bản ghi lời do người soạn cung cấp: bản nháp được để trống, **bắt buộc khi publish** (trợ năng khi phòng máy không có tai nghe); thiếu khi publish → `422 ACTIVITY_CONFIG_INVALID`. Học sinh thấy transcript theo `show_transcript` |
| `segments[]` | `{segment_id, start_ms, end_ms, title (≤ 128)}` |
| `questions[]` | `{at_ms? , question_version_id}`, có thể chèn giữa bài như video; mọi câu là loại tự chấm ([QUESTION_BANK](QUESTION_BANK.md)) |
| `interaction` | `max_plays` (giới hạn số lần nghe cho bài luyện thi, mặc định không giới hạn), `allow_speed_change`, `show_transcript: never/after_complete/always` |

Tiến độ, resume, câu chèn, quy tắc hoàn thành dùng chung cơ chế với [VIDEO_LEARNING §4–§7](VIDEO_LEARNING.md#4-tiến-độ-và-resume) (thay video bằng audio). Phát trong Godot bằng `AudioStreamPlayer` (Ogg Vorbis).

**Lượt nghe và timeline (F066):** một lượt nghe = phát từ đầu hoặc phát lại sau khi hết bài; client gửi `play_started = true` trong `reportActivityProgress`, server tăng `plays_used` (cột của `assessment.video_progress`) một lần cho mỗi `client_seq` mới, đủ `max_plays` → `422 AUDIO_PLAY_LIMIT_REACHED` và tiến độ không đổi; tạm dừng/tua trong lượt không tính. Câu hỏi là item theo `at_ms` (câu không có `at_ms` hỏi sau khi nghe hết), **không** bắt buộc như câu chèn video: câu bỏ trống được 0 điểm, không chặn nộp. Tốc độ 0,75×/1× khi `allow_speed_change`. Biến thể phát là `ogg` (thiếu → `422 MEDIA_NOT_READY` khi bắt đầu). Transcript là `media[]` `kind = transcript` (signed URL) khi `show_transcript = always`, hoặc `after_complete` sau khi nộp.

**Dạng bài nghe tự chấm:**

| Dạng | Câu hỏi dùng | Ví dụ |
|---|---|---|
| Nghe hiểu ý chính/chi tiết | `single_choice`, `multiple_choice`, `true_false` | Nghe đoạn hội thoại, chọn nơi các bạn đi chơi |
| Nghe và chọn hình | `image_based`, `single_choice` có ảnh | Nghe "apple", chạm vào hình quả táo |
| Nghe và sắp xếp | `ordering`, `matching` | Sắp xếp các bước làm bánh theo đoạn nghe |
| Nghe chép | `fill_in_blank`, `short_answer` (chuẩn hóa theo [QUESTION_BANK §4](QUESTION_BANK.md#4-chuẩn-hóa-văn-bản-tiếng-việt-match--normalized)) | Nghe và điền từ còn thiếu |

Kỹ năng **nói** được luyện gián tiếp qua nghe hiểu và nhận diện (ví dụ nghe và chọn cách phát âm đúng); không có đánh giá kỹ năng nói.

## 3. Dữ liệu (module learning)

| Bảng | Cột chính |
|---|---|
| `assessment.video_progress` | Dùng chung cho audio (media progress, gồm số lượt nghe đã dùng) |
| `assessment.attempt_items` / `assessment.attempt_answers` | Câu hỏi nghe hiểu là item của attempt; câu trả lời ghi như câu hỏi thường ([ACTIVITY_ENGINE §6.4](ACTIVITY_ENGINE.md#64-dữ-liệu-lưu-trữ-confirmed)) |

## 4. API

| Method | Path | Mô tả |
|---|---|---|
| POST | `/api/v1/learning/activities/{activity_id}/attempts` | Bắt đầu bài nghe (ghim version) |
| POST | `/api/v1/learning/activity-attempts/{attempt_id}/progress` | Tiến độ nghe (chung với video) |
| POST | `/api/v1/learning/activity-attempts/{attempt_id}/responses` | Trả lời câu hỏi nghe hiểu |
| POST | `/api/v1/learning/activity-attempts/{attempt_id}:submit` | Nộp, tính hoàn thành và điểm |

Event: `activity.started`, `question.answered`, `activity.completed`.

## 5. Nghiệm thu

| AC | Kịch bản |
|---|---|
| AC-AUD-001 | Bài nghe có `max_plays = 2`: lần thứ 3 không phát được; câu hỏi nghe hiểu chấm server |
| AC-AUD-002 | Bài nghe chép `fill_in_blank`: câu trả lời chuẩn hóa đúng quy tắc QUESTION_BANK §4, chấm server |
| AC-AUD-003 | Không có endpoint ghi âm, upload giọng nói hay phản hồi phát âm; activity `speaking` không publish được (`ACTIVITY_TYPE_NOT_SUPPORTED`) |
| AC-AUD-004 | Resume bài nghe trong thời hạn: tiếp tục đúng vị trí, câu đã trả lời giữ nguyên |

Test: TC-AUD-001…004 trong [TEST_STRATEGY](../12-project/TEST_STRATEGY.md).
