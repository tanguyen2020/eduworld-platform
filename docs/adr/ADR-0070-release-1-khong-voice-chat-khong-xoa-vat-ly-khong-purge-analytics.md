# ADR-0070 — Release 1.0: không voice chat, không xóa vật lý theo luật, không purge dữ liệu analytics thô

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-19, OQ-20 · **Bổ sung:** ADR-0028, ADR-0033, ADR-0051, ADR-0058, ADR-0059, ADR-0062 · **Tài liệu:** [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Ba câu hỏi còn mở trước khi chốt phạm vi Release 1.0: voice chat (OQ-19), xóa vật lý khi luật yêu cầu hoặc hết hạn lưu trữ (OQ-20), và có cần ngoại lệ purge cho `analytics.events_raw` giống ADR-0033 hay không. Người duyệt trả lời: **không làm trong version này** cho cả ba.

## Quyết định

1. **Voice chat không có trong Release 1.0 [Confirmed]** (OQ-19). Chat chỉ là văn bản, emoji, sticker, GIF (ADR-0034). Không xây API, kênh realtime, UI hay hạ tầng media cho giọng nói. Extension point `ChatPolicy.voice` trong tài liệu giữ ở mức thiết kế, không hiện thực. Thống nhất với ADR-0059 (không AI giọng nói). Muốn làm phải có ADR mới, kèm đánh giá an toàn trẻ em và đồng ý của phụ huynh.
2. **Không xóa vật lý theo luật trong Release 1.0 [Confirmed]** (OQ-20).
   - Yêu cầu xóa của chủ thể dữ liệu và dữ liệu hết hạn lưu trữ được đáp ứng bằng **soft-delete + ẩn danh hóa** (ADR-0028): xóa/thay giá trị định danh (tên, username, email, mã học sinh, `player_key`…), giữ dòng để không vỡ khóa ngoại và số liệu tổng hợp.
   - Không có quy trình vận hành xóa vật lý hay gỡ partition trong Release 1.0.
   - **Ngoại lệ duy nhất vẫn giữ:** purge chat gameplay theo ADR-0033.
   - Backup: dữ liệu đã ẩn danh hóa ở DB sẽ dần thay thế bản sao trong backup theo vòng quay backup ([BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md)); không sửa backup.
3. **Không có ngoại lệ purge cho dữ liệu analytics thô [Confirmed].**
   - `analytics.events_raw` và `player_days` theo ADR-0028: hết hạn giữ định danh (raw 90 ngày, `player_days` 400 ngày) thì job **ẩn danh hóa** (`player_key`, `session_key` = NULL); partition **được giữ lại**, không gỡ.
   - Dashboard vẫn chỉ đọc bảng tổng hợp (ADR-0062), nên dung lượng raw không ảnh hưởng tốc độ dashboard.
   - Theo dõi dung lượng: metric kích thước schema `analytics` và số dòng `events_raw`; cảnh báo theo ngưỡng chuyển kho phân tích riêng của ADR-0058 (500 triệu dòng). Khi chạm ngưỡng, quyết định bằng ADR mới (chuyển kho cột, lưu trữ lạnh hoặc purge).

## Hệ quả

(+) Phạm vi Release 1.0 gọn; không thêm rủi ro an toàn trẻ em từ giọng nói; không có đường xóa dữ liệu nào ngoài ADR-0033. (−) Dung lượng PostgreSQL tăng liên tục ở schema `analytics` và các bảng soft-delete, cần theo dõi và dự trù ổ đĩa; việc đáp ứng yêu cầu xóa bằng ẩn danh hóa cần tư vấn pháp lý xác nhận là đủ theo quy định bảo vệ dữ liệu cá nhân hiện hành [TBD — rủi ro pháp lý, xem RISKS].

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR; đóng OQ-19, OQ-20; không làm ngoại lệ purge analytics | Stakeholder (người duyệt) |
