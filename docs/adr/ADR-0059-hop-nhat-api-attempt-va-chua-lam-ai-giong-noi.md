# ADR-0059 — Hợp nhất API attempt và chưa làm AI giọng nói trong Release 1.0

> **Trạng thái:** Accepted · một phần superseded by ADR-0072 · **Ngày:** 2026-11-15 · **OQ:** OQ-22 · **Tài liệu:** [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md) · [API_DESIGN](../07-backend/API_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md).** Không còn hiệu lực: hoạt động `speaking` do giáo viên chấm tay (mục 2) — `speaking` nằm ngoài Release. **Còn hiệu lực:** một họ API attempt; không AI giọng nói trong Release 1.0.

## Bối cảnh

1. Hai họ API cùng phục vụ entity Attempt: `/api/v1/exercises/attempts` (hoạt động dạng câu hỏi) và `/api/v1/learning/activity-attempts` (video, ghi âm, tự luận, mô phỏng, dự án). Chưa có code hay client nào dùng, nên hợp nhất lúc này gần như không tốn chi phí.
2. Stakeholder quyết định: **chưa làm AI giọng nói trong phiên bản này** (trả lời OQ-22).

## Quyết định

1. **Một họ API duy nhất** `/api/v1/learning/activity-attempts` cho mọi `activity_type` (ADR-0048):
   - Bắt đầu: `POST /learning/activities/{activity_id}/attempts` (kể cả quiz/bài tập/question set; bài giao, quest, challenge truyền ngữ cảnh trong body).
   - Trả lời câu hỏi: `POST /learning/activity-attempts/{attempt_id}/responses` (thay `/exercises/attempts/{id}/answers`).
   - Xem lại sau khi nộp: `GET /learning/activity-attempts/{attempt_id}/review` (thay `/exercises/attempts/{id}/review`).
   - Các endpoint riêng theo loại giữ nguyên dưới cùng tiền tố (`/progress`, `/written-response`, `/recordings`, `/submissions`, `/simulation-runs`, `:submit`).
   - **Bỏ** họ `/exercises/attempts`. Nhóm route `/api/v1/exercises` không còn.
2. **AI giọng nói [Confirmed]:** Release 1.0 **không** có nhận dạng giọng nói hay chấm phát âm tự động. Hoạt động `speaking` vẫn có: học sinh ghi âm, **giáo viên nghe và chấm tay** theo rubric. Bỏ endpoint `speech-feedback`. Adapter nhà cung cấp chỉ là điểm mở rộng tương lai (ADR mới khi làm).

## Hệ quả

(+) Client chỉ cần một luồng attempt; server không lặp logic; giảm rủi ro dữ liệu giọng nói trẻ em. (−) Phải sửa spec, tài liệu và test đang tham chiếu `/exercises/attempts`; giáo viên mất thêm thời gian chấm bài nói.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
| 2026-11-15 | Một phần superseded by ADR-0072 (bỏ vai trò giáo viên) | Technical Architect |
