# ADR-0034 — Bộ tính năng chat đầy đủ

> **Trạng thái:** Accepted · một phần superseded by ADR-0072 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md).** Không còn hiệu lực: quyền của giáo viên trong chat (gỡ tin qua moderation, `@all` và ghim tin ở channel lớp); gỡ tin do Platform Safety Team, `@all`/ghim ở channel lớp không còn vai trò giáo viên. **Còn hiệu lực:** toàn bộ bộ tính năng chat.

## Bối cảnh

Stakeholder yêu cầu chat có: gửi, nhận, sửa, thu hồi; reply, quote, forward; emoji, sticker, GIF; reactions; mention; pin; copy; typing indicator; sent/delivered/read; unread count; lịch sử và tìm kiếm.

## Quyết định

| Tính năng | Thiết kế | Ràng buộc an toàn / kỹ thuật |
|---|---|---|
| **Gửi / nhận** | Lệnh WS `chat.message.send` (`client_msg_id` để idempotent), server cấp `seq` theo channel, phát `chat.message.created` | Qua pipeline kiểm tra (ADR-0020/0030) |
| **Sửa** | Trong **15 phút** [Confirmed], nhãn "đã chỉnh sửa", lưu bản cũ trong `chat.message_revisions` | Nội dung mới **chạy lại pipeline**; bị chặn thì giữ bản cũ; bản cũ chỉ moderator xem trong case |
| **Thu hồi** | Trong **24 giờ** [Confirmed], mọi người thấy "Tin nhắn đã được thu hồi"; giáo viên/moderator gỡ tin của người khác bằng chức năng moderation | Thu hồi = ẩn (soft); nội dung gốc **giữ làm bằng chứng** đến hết thời hạn lưu trữ; vẫn báo cáo được |
| **Reply** | `reply_to_message_id` (cùng channel), hiển thị trích ngắn | Tin gốc không còn → "Tin nhắn gốc không còn" |
| **Quote** | `quote: {message_id, excerpt}` | Trích từ bản **đã kiểm duyệt**, không tự gõ nội dung trích |
| **Forward** | Sang channel người dùng được gửi, **chỉ trong cùng tenant**, nhãn "Đã chuyển tiếp" + nickname người gửi gốc | Chạy lại pipeline và policy channel đích; không forward tin đã thu hồi/che/gỡ; tối đa 5 channel đích [Confirmed] |
| **Emoji** | Bộ emoji Unicode cho phép | — |
| **Sticker** | Sticker pack do platform quản lý (Game Operations Console); có thể mở khóa làm phần thưởng | **Không có upload của người dùng** |
| **GIF** | **Thư viện GIF nội bộ đã duyệt**, tìm theo tag, phát từ CDN EduWorld | **Không** dùng Giphy/Tenor…; dùng nhà cung cấp ngoài → ADR mới |
| **Reactions** | Bộ reaction cho phép, tối đa 3 reaction/người/tin [Confirmed] | — |
| **Mention** | `@nickname` (không dùng username đăng nhập vì chứa mã học sinh), gợi ý trong thành viên channel, bộ đếm mention riêng | Tối đa 5 mention/tin; `@all` chỉ giáo viên ở channel lớp [Confirmed]; không mention người đã block mình |
| **Pin** | Tối đa 3 tin/channel [Confirmed]; quyền: leader party, giáo viên ở channel lớp, cả hai người trong DM, vai trò trong study group; không pin ở World/Area | Audit; tin bị thu hồi/gỡ tự bỏ ghim |
| **Copy** | Tính năng client | Copy đúng bản đã che |
| **Typing** | `chat.typing.start/stop`, throttle 3 s, hết sau 5 s, **không lưu DB** | Không gửi tới người đã block; không có ở World/Area |
| **Sent / Delivered / Read** | Theo **con trỏ** mỗi thành viên (`last_delivered_seq`, `last_read_seq`) | Chi tiết từng người ở DM và nhóm ≤ 8 người; Class/Study Group hiển thị "N người đã xem"; World/Area không có |
| **Unread count** | `channel.last_seq − member.last_read_seq` + bộ đếm mention; đồng bộ qua `chat.unread.updated` | Cache Valkey, nguồn sự thật PostgreSQL |
| **Lịch sử & tìm kiếm** | Phân trang theo `seq`; full-text (`tsvector` + `unaccent`) trong channel người dùng là thành viên | Luôn lọc `tenant_id`, membership, `deleted_at IS NULL`; không trả tin thu hồi/bị chặn; không tìm được dữ liệu đã purge (ADR-0033); moderator tìm trong phạm vi case |

**Theo loại channel:** mặc định đủ tính năng, trừ World/Area không có pin, receipts, typing, forward ra ngoài. Bật/tắt từng tính năng là cấu hình **platform** trong chat policy (`features`); tenant không nới được. Channel gameplay (ADR-0020) luôn đủ: gửi/nhận, reply, emoji, sticker, GIF, reaction, mention, typing, receipts, unread.

## Hệ quả

(+) Trải nghiệm như ứng dụng nhắn tin quen thuộc mà vẫn an toàn: nội dung mới (kể cả sửa, forward) đều qua kiểm tra; thu hồi không xóa được bằng chứng. (−) Nhiều sự kiện realtime (typing, receipts) cần throttle và đo tải; index full-text tăng kích thước.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
| 2026-11-15 | Một phần superseded by ADR-0072 (bỏ vai trò giáo viên) | Technical Architect |
