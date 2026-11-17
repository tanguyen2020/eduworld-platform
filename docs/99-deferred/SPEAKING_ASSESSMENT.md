# Speaking Assessment — bài nói và ghi âm giọng nói

> Trạng thái: Draft · Phiên bản: 0.1 · Owner: EdTech Specialist (Tiếng Anh, Tiếng Việt) + Identity & Security Engineer · Cập nhật: 2026-11-15
> Liên quan: [ADR-0082](../adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md) · [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) · [ADR-0059](../adr/ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md) · [ADR-0053](../adr/ADR-0053-cham-diem-co-ho-tro-ai-ai-assisted-grading.md) · [ADR-0044](../adr/ADR-0044-luu-tru-hinh-anh-va-media-object-storage-s3-compatible-cdn.md) · [LEGAL_COMPLIANCE](LEGAL_COMPLIANCE.md) · [README](README.md) · Schema: [`speaking.schema.json`](../../specs/_deferred/schemas/activities/speaking.schema.json)

> [!WARNING]
> **Deferred — không triển khai (ADR-0082).** Version này không có activity `speaking`, không ghi âm giọng nói học sinh, không có AI giọng nói (speech-to-text, chấm phát âm). AI coding agent không đọc tài liệu này để triển khai. Bài nghe tự chấm (`audio_lesson`) ở [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md). Mở lại phải có ADR mới, chuyển tài liệu về cây chính và viết feature spec.

## 1. Mục đích và phạm vi

Lưu thiết kế tham khảo của activity `speaking` (đọc to, nhắc lại theo mẫu, trả lời bằng lời có ghi âm), tách khỏi AUDIO_LANGUAGE_LEARNING ngày 2026-11-15. Câu hỏi mở liên quan: OQ-22 (nhà cung cấp nhận dạng giọng nói, nơi xử lý dữ liệu giọng nói trẻ em) — Hoãn (ADR-0082).

## 2. Dạng bài

| `speaking_mode` | Mô tả |
|---|---|
| `read_aloud` | Đọc to câu/đoạn cho sẵn |
| `repeat_after` | Nghe audio mẫu rồi nhắc lại |
| `free_response` | Trả lời câu hỏi bằng lời |
| `pronunciation_drill` | Luyện từ/âm khó: nghe mẫu, ghi âm, nghe lại bản của mình để tự so sánh |

## 3. Luồng của học sinh trong game

1. Mở hoạt động, xem yêu cầu, nghe audio mẫu (nếu có).
2. Bấm nút ghi âm (biểu tượng "đang ghi âm" hiển thị rõ), dừng, **nghe lại**; được ghi lại tối đa `max_retakes` lần.
3. Chọn bản muốn nộp → nộp. Cách đánh giá phụ thuộc ADR mở lại (version này không có người chấm).

## 4. Ghi âm trên Godot

Ràng buộc kỹ thuật bắt buộc nếu mở lại:

- Dùng bus âm thanh có `AudioEffectRecord`, chỉ bật khi học sinh bấm và giữ/bấm nút ghi âm.
- Giới hạn độ dài mỗi bản ghi `max_record_seconds` (30 giây cho lớp 1–5, 90 giây cho lớp 6–9) và số lần ghi lại `max_retakes` (3) [Proposed — chỉ áp dụng khi mở lại].
- Mã hóa Ogg Vorbis mono 16 kHz trên client.
- Upload qua **presigned PUT** lên bucket riêng `ew-student-submissions` (prefix `recordings/`, ADR-0044 mục 3); file không đi qua CDN công khai.
- Quyền micro: xin quyền hệ điều hành khi lần đầu dùng, có màn hình giải thích thân thiện.

## 5. Bảo vệ dữ liệu giọng nói trẻ em

Giọng nói là dữ liệu cá nhân nhạy cảm của trẻ. Nếu mở lại `speaking`, các biện pháp sau là điều kiện tối thiểu:

| Biện pháp | Chi tiết | Nhãn |
|---|---|---|
| Tối thiểu hóa | Chỉ ghi khi học sinh chủ động bấm; không ghi nền | [Confirmed] |
| Đồng ý | Tenant bật tính năng, có xác nhận đồng ý của nhà trường/phụ huynh theo [LEGAL_COMPLIANCE](LEGAL_COMPLIANCE.md) | [Proposed], OQ-12 (Hoãn) |
| Lưu trữ | Bucket private, mã hóa at-rest, signed URL ngắn hạn chỉ cho học sinh đó và người được giao xử lý theo ADR mở lại | [Proposed] |
| Thời hạn | Ẩn file khỏi API sau thời hạn lưu; Release 1.0 không xóa vật lý (ADR-0070) nên cần quyết định ngoại lệ trước khi mở lại | [TBD] |
| Truy cập | Không chia sẻ bản ghi cho học sinh khác; không gửi ra dịch vụ bên ngoài khi chưa có ADR; không dùng huấn luyện mô hình | [Confirmed] |

## 6. Dữ liệu dự kiến

Bảng `assessment.audio_recordings` (metadata bản ghi: `attempt_id`, `object_key`, `duration_ms`, `retake_no`, `submitted`) và object trong `ew-student-submissions/recordings/` — chỉ tạo khi mở lại.

## 7. Điều kiện mở lại

ADR mới chọn cách đánh giá (tự động có kiểm định độ chính xác với giọng trẻ em), nhà cung cấp/mô hình, nơi xử lý dữ liệu (trong nước, OQ-12) và quy trình đồng ý ([LEGAL_COMPLIANCE](LEGAL_COMPLIANCE.md)); cập nhật DATA_PRIVACY, CHILD_SAFETY, schema `speaking` và feature spec.

## 8. Acceptance criteria

Không có trong version này. Nghiệm thu version hiện tại chỉ kiểm tra **không** có endpoint ghi âm (AC-AUD-003 trong [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md#5-nghiệm-thu)).
