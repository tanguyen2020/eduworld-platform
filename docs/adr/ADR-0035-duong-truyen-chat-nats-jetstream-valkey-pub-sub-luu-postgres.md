# ADR-0035 — Đường truyền chat: NATS JetStream + Valkey Pub/Sub, lưu PostgreSQL

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md), [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu chat dùng Pub/Sub (Valkey) + NATS để gửi, nhận và lưu ở PostgreSQL.

## Quyết định

1. **Phân vai:**

   | Thành phần | Vai trò | Không làm |
   |---|---|---|
   | **NATS JetStream** | Đường ghi **bền vững**: mọi tin nhắn và thay đổi tin (sửa, thu hồi, reaction, pin, gỡ) vào stream trước khi xác nhận "Sent"; bộ đệm cho persister và nguồn cho consumer bất đồng bộ | Không lưu lâu dài (tối đa 72 giờ) |
   | **Valkey Pub/Sub** | **Fan-out realtime** tới các node `realtime-gateway`; mang tín hiệu tạm: typing, receipts, unread, presence | Không đảm bảo giao, không lưu |
   | **PostgreSQL** | **Nguồn sự thật**: tin nhắn, revision, reaction, pin, con trỏ đọc, lịch sử, tìm kiếm | Không nằm trên đường trễ gửi/nhận |

2. **Luồng gửi** (trong **chat-service**; gateway chỉ chuyển lệnh qua NATS request/reply, ADR-0036): kiểm tra membership/policy/rate limit → pipeline kiểm duyệt (ADR-0020/0030) → cấp `seq` bằng Valkey `INCR` (khởi tạo từ `MAX(seq)` trong PostgreSQL khi key mất, script Lua nguyên tử) → envelope (`message_id` UUIDv7, `tenant_id`, `channel_id`, `seq`, `client_msg_id`, nội dung đã kiểm duyệt) → **publish JetStream** `ew.chat.<tenant_id>.msg.<channel_id>` với `Nats-Msg-Id = message_id` → ack → trả `chat.message.acked` (Sent) → **publish Valkey** `ew:chat:fanout:<channel_id>` → gateway giao `chat.message.created` → thiết bị nhận ack `delivered`.
3. **Luồng lưu:** worker `chat-persister` (JetStream durable pull consumer, nhiều instance) ghi theo lô vào PostgreSQL với `ON CONFLICT (message_id) DO NOTHING`, cập nhật `channels.last_seq`; độ trễ lưu p95 ≤ 500 ms [Proposed]; ack JetStream sau khi commit; lỗi lặp lại → DLQ + alert.
4. **Consumer khác:** `chat-notifier` (mention/tin mới cho người offline), `chat-moderation-async` (bước `async_analysis`), `chat-stats` (rollup analytics, ADR-0033). Con trỏ đọc gửi gộp tối đa 1 lần/2 giây/thành viên.
5. **Tín hiệu tạm chỉ qua Valkey Pub/Sub:** typing, delivered/read broadcast, `chat.unread.updated`.
6. **Độ tin cậy:**
   - Mất gói Pub/Sub: client phát hiện **khoảng trống `seq`** và gọi lịch sử (PostgreSQL + bộ đệm 200 tin gần nhất trong Valkey, TTL 10 phút).
   - NATS không sẵn sàng: từ chối gửi `CHAT_TEMPORARILY_UNAVAILABLE`; **không** phát tin chưa vào JetStream.
   - PostgreSQL không sẵn sàng: JetStream giữ tin (≤ 72 giờ), persister ghi bù.
   - Thứ tự hiển thị theo `seq`.
7. **JetStream [Proposed]:** stream `CHAT_MESSAGES` (subjects `ew.chat.*.msg.*`, `ew.chat.*.evt.*`, `ew.chat.*.cursor.*`), file storage, 3 replicas, `max_age` 72 h, `duplicate_window` 2 phút; consumer `persister`, `notifier`, `moderation`, `stats`.
8. **Cô lập và bảo mật:** subject chứa `tenant_id`; consumer đối chiếu `tenant_id` với channel; NATS account/user riêng cho từng service, quyền tối thiểu, TLS, chỉ mạng private. Game client và portal **không** kết nối NATS/Valkey.
9. **Hạ tầng:** NATS cluster 3 node có JetStream (local: 1 node); metric qua `prometheus-nats-exporter`; không cần backup NATS.
10. **Phạm vi:** party/lobby/hub/battle dùng Valkey Pub/Sub (ADR-0011); domain event chung dùng NATS JetStream (ADR-0042) — hệ thống chỉ có một message bus bền vững.

## Hệ quả

(+) Gửi/nhận nhanh, không mất tin khi DB gián đoạn, dễ thêm consumer. (−) Thêm NATS phải vận hành; lịch sử phải ghép với bộ đệm gần nhất; cần test mất Valkey/NATS/PostgreSQL.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted (xử lý trong realtime node) | Stakeholder |
| 2026-11-15 | Xử lý chuyển vào chat-service; gateway chỉ định tuyến | ADR-0036 |
| 2026-11-15 | Domain event chung cũng dùng NATS | ADR-0042 |
