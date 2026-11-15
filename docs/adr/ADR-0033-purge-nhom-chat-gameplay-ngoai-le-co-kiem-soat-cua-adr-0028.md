# ADR-0033 — Purge nhóm chat gameplay (ngoại lệ có kiểm soát của ADR-0028)

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-20 · **Tài liệu:** [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md), [MODERATION](../03-social-multiplayer/MODERATION.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu clear nhóm chat vì nếu không xóa sẽ tạo nhiều dữ liệu rác. Mỗi trận 1v1, trận đội và party đều tự tạo channel (ADR-0020); soft-delete (ADR-0028) không giảm dung lượng.

## Quyết định

1. **Phạm vi purge (xóa vật lý)** chỉ dữ liệu chat **ngắn hạn gắn với phiên chơi**: channel `match`, `team` gắn với một trận, `party` (sau khi giải tán), cùng thành viên channel, tin nhắn, reaction, read state; tin nhắn ephemeral của World/Area. **Không** áp dụng cho Class, Study Group, Team cố định/guild, Direct Message.
2. **Thời điểm:** channel đóng khi trận kết thúc hoặc party giải tán (`closed_at`); purge sau `gameplay_chat_retention_days` kể từ `closed_at`. Mặc định 7 ngày [Proposed], **tối thiểu 3 ngày** để kịp báo cáo; cấu hình cấp platform, tenant được điều chỉnh trong giới hạn [Proposed]. World/Area ephemeral: 24 giờ.
3. **Giữ bằng chứng:** tin thuộc report, moderation case mở hoặc sanction được **sao chép** sang `chat.moderation_evidence` trước khi purge; channel có case mở thì hoãn purge.
4. **Thực hiện (ứng dụng vẫn không hard-delete):**
   - Không API/use case nào xóa vật lý; chỉ job `gameplay-chat-purge` trong worker của chat-service, dùng role DB `chat_purger` chỉ có `DELETE` trên các bảng chat gameplay.
   - Tin nhắn gameplay ở bảng riêng `chat.gameplay_messages` **partition theo ngày tạo**; purge theo từng channel đã hết hạn (xóa theo lô); chỉ `DROP PARTITION` khi mọi channel trong partition đã purge.
   - Không purge bản ghi còn tham chiếu từ moderation/evidence.
   - Mỗi lần chạy ghi audit tổng hợp (số channel, số tin, tenant), không ghi nội dung; metric `eduworld_chat_purge_total{tenant,kind}`; alert khi lỗi hoặc tồn đọng quá 2 ngày.
5. **Chỉ số tổng hợp** (số tin mỗi trận, số tin bị chặn…) được rollup vào analytics trước khi purge.

## Hệ quả

(+) Dung lượng chat ổn định, đúng tinh thần tối thiểu hóa dữ liệu trẻ em. (−) Sau thời hạn không xem lại được chat trận cũ (trừ phần lưu làm bằng chứng). Đây là ngoại lệ duy nhất của ADR-0028.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted (partition theo ngày `closed_at`, role `eduworld_purger`) | Stakeholder |
| 2026-11-15 | Partition theo ngày tạo + purge theo channel (tránh di chuyển dòng giữa partition); role đổi tên `chat_purger` | DATABASE_DESIGN (thiết kế chat) |
