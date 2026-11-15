# Audio & Language Learning

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: EdTech Specialist (Tiếng Anh, Tiếng Việt) + Identity & Security Engineer · Cập nhật: 2026-11-15
> Liên quan: [ACTIVITY_ENGINE](ACTIVITY_ENGINE.md) · [VIDEO_LEARNING](VIDEO_LEARNING.md) · [WRITTEN_RESPONSE_RUBRIC](WRITTEN_RESPONSE_RUBRIC.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [ADR-0044](../adr/ADR-0044-luu-tru-hinh-anh-va-media-object-storage-s3-compatible-cdn.md) · [ADR-0059](../adr/ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md) · Schema: [`audio-lesson.schema.json`](../../specs/schemas/activities/audio-lesson.schema.json), [`speaking.schema.json`](../../specs/schemas/activities/speaking.schema.json)

## 1. Mục đích và phạm vi

Hai loại hoạt động:

| `activity_type` | Gồm | Nhãn |
|---|---|---|
| `audio_lesson` | Bài nghe, nghe hiểu (listening) có câu hỏi, nghe chép (dictation) | [Confirmed] Master Prompt mới §15.1 |
| `speaking` | Đọc to, nhắc lại theo mẫu, trả lời bằng lời; học sinh **ghi âm**, **giáo viên nghe và chấm tay** theo rubric | [Confirmed] (ADR-0059) |

**Release 1.0 không có AI giọng nói** [Confirmed, ADR-0059]: không nhận dạng giọng nói (speech-to-text), không chấm phát âm tự động, không gửi giọng nói học sinh tới dịch vụ bên ngoài. Xem §7.

## 2. `audio_lesson`

| Trường (`content`) | Mô tả |
|---|---|
| `audio_asset_id` | Ogg Vorbis cho Godot, MP3 cho web (ADR-0044) |
| `transcript_asset_id?` | Bản ghi lời do người soạn cung cấp, hiện sau khi hoàn thành hoặc khi bật trợ năng |
| `segments[]` | `{segment_id, start_ms, end_ms, title (≤ 128)}` |
| `questions[]` | `{at_ms? , question_version_id}`, có thể chèn giữa bài như video |
| `interaction` | `max_plays` (giới hạn số lần nghe cho bài luyện thi, mặc định không giới hạn), `allow_speed_change`, `show_transcript: never/after_complete/always` |

Tiến độ, resume, câu chèn, quy tắc hoàn thành dùng chung cơ chế với [VIDEO_LEARNING §4–§7](VIDEO_LEARNING.md#4-tiến-độ-và-resume) (thay video bằng audio). Phát trong Godot bằng `AudioStreamPlayer` (Ogg Vorbis).

## 3. `speaking`

### 3.1. Dạng bài

| `speaking_mode` | Mô tả | Chấm |
|---|---|---|
| `read_aloud` | Đọc to câu/đoạn cho sẵn | Giáo viên chấm theo rubric (`teacher_review`) |
| `repeat_after` | Nghe audio mẫu rồi nhắc lại | Giáo viên chấm theo rubric |
| `free_response` | Trả lời câu hỏi bằng lời (ví dụ giới thiệu bản thân bằng tiếng Anh) | Giáo viên chấm theo rubric |
| `pronunciation_drill` | Luyện từ/âm khó | **Tự luyện**: học sinh nghe mẫu, ghi âm, nghe lại bản của mình để tự so sánh; hoàn thành khi ghi đủ số từ. Không có điểm tự động; giáo viên có thể nghe và nhận xét nếu bài được giao |

### 3.2. Luồng của học sinh trong game

1. Mở hoạt động (từ quest, bài giao, khu Rừng Ngôn Ngữ): xem yêu cầu, nghe audio mẫu (nếu có).
2. Bấm nút ghi âm (biểu tượng "đang ghi âm" hiển thị rõ), dừng, **nghe lại**; được ghi lại tối đa `max_retakes` lần.
3. Chọn bản muốn nộp → nộp (`:submit`). Attempt chuyển `pending_review` (trừ `pronunciation_drill` không giao chấm → hoàn thành ngay).
4. Nhận thông báo khi giáo viên chấm xong; xem điểm rubric và nhận xét trong màn Kết quả.

### 3.3. Ghi âm trên Godot

- Dùng bus âm thanh có `AudioEffectRecord`, chỉ bật khi học sinh bấm và giữ/bấm nút ghi âm.
- Giới hạn độ dài mỗi bản ghi `max_record_seconds` (mặc định 30 giây cho lớp 1–5, 90 giây cho lớp 6–9 [Proposed]) và số lần ghi lại `max_retakes` (mặc định 3) — giá trị sở hữu tại tài liệu này, chỉ mục ở [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md).
- Mã hóa Ogg Vorbis mono 16 kHz trên client (giảm dung lượng, phù hợp máy tầm trung và mạng chậm).
- Upload: client xin **presigned PUT** (`POST /api/v1/learning/activity-attempts/{attempt_id}/recordings:presign`) → upload thẳng lên bucket riêng `ew-student-submissions` (prefix `recordings/`, ADR-0044 mục 3) → gọi `:complete` để server xác nhận (kích thước, định dạng, thời lượng). File không đi qua CDN công khai.
- Quyền micro: xin quyền hệ điều hành khi lần đầu dùng, có màn hình giải thích thân thiện. Từ chối quyền → hoạt động hiển thị lựa chọn thay thế (nếu giáo viên cho phép) hoặc báo cần micro.

### 3.4. Giáo viên chấm bài nói

- Bài nộp vào **hàng chờ chấm** của giáo viên được phân công lớp/môn trong Admin Portal ([SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)), cùng hàng chờ với bài tự luận ([WRITTEN_RESPONSE_RUBRIC](WRITTEN_RESPONSE_RUBRIC.md)).
- Màn chấm: trình phát bản ghi (tua, tốc độ 0,75×/1×), văn bản tham chiếu (với `read_aloud`/`repeat_after`), rubric, ô nhận xét; chấm xong → Evaluation `source = teacher`, attempt `graded`, học sinh nhận thông báo.
- Hạn chấm khuyến nghị và quy tắc chấm lại dùng chung với tự luận.

## 4. Bảo vệ dữ liệu giọng nói trẻ em

Giọng nói là dữ liệu cá nhân nhạy cảm của trẻ.

| Biện pháp | Chi tiết | Nhãn |
|---|---|---|
| Tối thiểu hóa | Chỉ ghi khi học sinh chủ động bấm; không ghi nền; **không xử lý tự động nội dung giọng nói** (không speech-to-text, không phân tích giọng) | [Confirmed] |
| Đồng ý | Tính năng `speaking` cần tenant bật, có xác nhận đồng ý của nhà trường/phụ huynh theo DATA_PRIVACY | [Proposed], OQ-12 |
| Lưu trữ | Bucket private, mã hóa at-rest, signed URL ngắn hạn chỉ cho học sinh đó, giáo viên được phân công lớp/môn, moderator trong case | [Proposed] |
| Thời hạn | Sau **90 ngày** kể từ khi chấm xong [Proposed], file ghi âm bị ẩn khỏi API (không cấp signed URL nữa); giữ điểm và nhận xét. Release 1.0 không xóa vật lý nên file vẫn nằm trong bucket private (ADR-0070); cần người duyệt xác nhận hoặc thêm ngoại lệ xóa file ghi âm | [TBD] |
| Truy cập | Không chia sẻ bản ghi cho học sinh khác; không gửi ra dịch vụ bên ngoài; không dùng huấn luyện mô hình | [Confirmed] |

## 5. Dữ liệu (learning-service)

| Bảng | Cột chính |
|---|---|
| `learning.audio_recordings` | `recording_id`, `tenant_id`, `attempt_id`, `student_id`, `storage_key` varchar(512), `duration_ms`, `size_bytes`, `status` (`uploading/ready/submitted/expired`), `retention_until`, `created_at`, `deleted_at` |
| `learning.video_progress` | Dùng chung cho audio (media progress) |

Điểm và nhận xét của giáo viên nằm trong `Evaluation` ([ACTIVITY_ENGINE §7](ACTIVITY_ENGINE.md)).

## 6. API

| Method | Path | Mô tả |
|---|---|---|
| POST | `/api/v1/learning/activity-attempts/{attempt_id}/recordings:presign` | Xin URL upload bản ghi |
| POST | `/api/v1/learning/activity-attempts/{attempt_id}/recordings/{recording_id}:complete` | Xác nhận upload xong |
| GET | `/api/v1/learning/activity-attempts/{attempt_id}/recordings/{recording_id}` | Signed URL nghe lại (chủ sở hữu, giáo viên được phân công) |
| POST | `/api/v1/learning/activity-attempts/{attempt_id}/progress` | Tiến độ nghe (chung với video) |
| POST | `/api/v1/learning/activity-attempts/{attempt_id}:submit` | Nộp bài nói → `pending_review` |
| POST | `/api/v1/lms/reviews/{review_id}:grade` | Giáo viên chấm (xem SCHOOL_LMS) |

Event: `speaking.recording_submitted`, `activity.completed`, `evaluation.completed`.

## 7. Ngoài phạm vi Release 1.0 — mở rộng sau

Nhận dạng giọng nói và chấm phát âm tự động **không làm** trong Release 1.0 (ADR-0059; OQ-22 đã trả lời). Thiết kế giữ điểm mở rộng: hoạt động `speaking` có trường `evaluation_strategy` (hiện chỉ `teacher_review`/`self_practice`); khi làm sau cần ADR mới chọn nhà cung cấp/mô hình, nơi xử lý dữ liệu (trong nước, OQ-12), đánh giá độ chính xác với giọng trẻ em và quy trình đồng ý.

## 8. Nghiệm thu

| AC | Kịch bản |
|---|---|
| AC-AUD-001 | Bài nghe có `max_plays = 2`: lần thứ 3 không phát được; câu hỏi nghe hiểu chấm server |
| AC-AUD-002 | Học sinh ghi âm 3 lần, nghe lại, nộp lần cuối; bản ghi nằm ở bucket private, URL nghe lại hết hạn sau 15 phút |
| AC-AUD-003 | Bài `read_aloud` nộp xong ở trạng thái `pending_review`, xuất hiện trong hàng chờ chấm của giáo viên được phân công; giáo viên nghe và chấm theo rubric, học sinh nhận điểm và nhận xét |
| AC-AUD-004 | Không có request nào gửi dữ liệu giọng nói ra ngoài hệ thống; không có endpoint phản hồi phát âm tự động |
| AC-AUD-005 | Tenant chưa bật `speaking`: hoạt động không cho bắt đầu, portal cảnh báo khi gán |

Test: TC-AUD-001…005 trong [TEST_STRATEGY](../12-project/TEST_STRATEGY.md).
