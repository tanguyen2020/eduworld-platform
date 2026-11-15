# ADR-0051 — Phạm vi voice chat

> **Trạng thái:** Accepted (xác nhận bởi ADR-0070) · **Ngày:** 2026-11-15 · **OQ:** OQ-19 · **Tài liệu:** [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) · [Chỉ mục ADR](README.md)

> [!NOTE]
> **Đã được xác nhận bởi [ADR-0070](ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md)** (2026-11-15): Release 1.0 không có voice chat; OQ-19 đã đóng. Điểm mở rộng chỉ ở mức thiết kế.

## Bối cảnh

Master Prompt gốc §6.2 và Master Prompt "Re-architecture" §13.5: voice chat không bật mặc định. OQ-19 chưa trả lời.

## Quyết định (đề xuất)

- Release 1.0 **không có voice chat**.
- Giữ điểm mở rộng: channel có `media_capabilities` (hiện chỉ `text`), permission `chat.voice.*` dự phòng, UI chat có chỗ cho nút voice nhưng ẩn.
- Nếu OQ-19 quyết định bật: ADR mới về hạ tầng (WebRTC SFU), ghi âm/kiểm duyệt giọng nói, đồng ý của phụ huynh/nhà trường, chỉ party/team, mặc định tắt theo tenant.

## Hệ quả

(+) Tránh rủi ro an toàn trẻ em lớn nhất trong giao tiếp thời gian thực. (−) Phối hợp trong trận chỉ bằng chữ, quick message, emote.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Audit theo Master Prompt mới |
