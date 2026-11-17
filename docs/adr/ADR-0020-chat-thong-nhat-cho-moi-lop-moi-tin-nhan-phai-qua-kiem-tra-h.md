# ADR-0020 — Chat thống nhất cho mọi lớp, mọi tin nhắn phải qua kiểm tra hợp lệ

> **Trạng thái:** Accepted · một phần superseded by ADR-0072 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](../03-social-multiplayer/MODERATION.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md).** Không còn hiệu lực: giáo viên là thành viên channel Class (mục 3) và quyền giáo viên mute/khóa channel lớp (mục 4); channel Class chỉ gồm học sinh của lớp, kiểm duyệt do Platform Safety Team. **Còn hiệu lực:** toàn bộ phần còn lại, gồm một bộ quy tắc cho mọi lớp và chat gameplay luôn có.

## Bối cảnh

Stakeholder quyết định: chat **không phân biệt theo lớp**, mọi tin nhắn phải **hợp lệ** trước khi phát đi, và chat **luôn có** khi chơi cùng nhau. Quyết định này thay mục "Age/Grade-based Policy" của Master Prompt gốc §6.3.

## Quyết định

1. **Một bộ quy tắc chat cho học sinh lớp 1–9.** `ChatPolicy` không có điều kiện theo `grade_level`/`grade_band`. Mọi học sinh có cùng loại channel, cùng loại tin nhắn (text, emoji, sticker, GIF, quick message, reaction) và cùng rate limit. Tính năng chi tiết: ADR-0034.
2. **8 loại channel**, phạm vi theo **quan hệ**, không theo lớp:
   - World: người chơi cùng trường (mặc định); tenant loại đơn vị có thể mở rộng ra mọi trường trong tenant. Người ngoài trường chỉ thấy nickname.
   - Area: người chơi trong cùng instance/khu vực.
   - Match: người chơi trong một trận 1v1/đội.
   - Party / Team / Study Group: thành viên nhóm.
   - Class: học sinh của lớp + giáo viên được phân công.
   - Direct Message: chỉ giữa hai người **đã là bạn bè** và không block nhau.
3. **Kiểm tra hợp lệ đồng bộ, phía server, trước khi phát** cho mọi tin nhắn (kể cả quick message có tham số và tên nhóm/channel): normalize (NFC, bỏ ký tự ẩn, gộp ký tự lặp) → định dạng/độ dài → từ nhạy cảm (tiếng Việt có/không dấu, teencode, leetspeak, tiếng Anh; danh mục theo ADR-0030) → thông tin cá nhân (SĐT, email, địa chỉ, số định danh, link, tài khoản mạng xã hội) → spam/flood/trùng lặp → bắt nạt/xúc phạm (rule + classifier) → quyết định `allow | mask | block | hold_for_review`. Chỉ tin `allow`/`mask` được phát; tin `block` không lưu nội dung, chỉ metadata. Phân tích bất đồng bộ (ngữ cảnh hội thoại, grooming heuristic) chạy sau trong worker và có thể gỡ tin, mở moderation case. Pipeline **bật mặc định**; System Admin có thể tắt từng bước hoặc toàn bộ theo ADR-0030.
4. **Kiểm soát của tenant/trường/lớp áp dụng như nhau cho mọi học sinh:** bật/tắt loại channel (trừ channel gameplay ở mục 5), khung giờ chat, slow mode, giáo viên mute/khóa channel lớp, mức nghiêm của bộ lọc. Cấp dưới chỉ được **siết chặt hơn** cấp trên (platform → tenant → school → class).
5. **Chat luôn có khi chơi cùng nhau.** Khi vào trận **1v1**, **chơi theo đội** hoặc **tạo party**, hệ thống tự tạo và tự cho người chơi tham gia channel: 1v1 → `match`; theo đội → `team` cho từng đội + `match` chung; party → `party` từ lúc tạo đến khi giải tán. Ba loại này **không tắt được** bằng cài đặt trường/lớp; chỉ mute/block cá nhân hoặc sanction của moderator ngăn được một người gửi tin. Channel `match` đóng khi trận kết thúc; dữ liệu chat gameplay bị purge theo ADR-0033.
6. Block, mute, report, moderation queue, audit theo MODERATION.

## Hệ quả

(+) Trải nghiệm chat đồng nhất, dễ hiểu, dễ kiểm thử. (−) Học sinh nhỏ tuổi nhận cùng mức tự do như học sinh lớn, nên bộ lọc, moderation và thời gian xử lý báo cáo phải đạt ngưỡng trước khi phát hành (recall ≥ 95%, false positive ≤ 3% [Proposed], xem TEST_STRATEGY); cần công cụ moderation đầy đủ ngay khi bật chat.

## Phương án đã loại

Policy theo grade band (quick message cho lớp 1–3, DM chỉ lớp 6–9): stakeholder loại; giữ nguyên khi đối chiếu Master Prompt "Re-architecture" (DOCUMENTATION_AUDIT C2).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR: default-deny, policy theo grade band | Phase 0 |
| 2026-11-15 | Accepted: chat như nhau mọi lớp, mọi tin phải hợp lệ | Stakeholder |
| 2026-11-15 | Pipeline có thể tắt theo ADR-0030 | Stakeholder |
| 2026-11-15 | Thêm channel `match`; chat gameplay luôn có | Stakeholder |
| 2026-11-15 | Giữ quyết định khi audit theo Master Prompt mới | DOCUMENTATION_AUDIT §4.1 |
| 2026-11-15 | Một phần superseded by ADR-0072 (bỏ vai trò giáo viên) | Technical Architect |
