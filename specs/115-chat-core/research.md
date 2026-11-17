# Research: F115 — module chat lõi: channel, policy deny-by-default, JetStream, `chat-persister`, gửi/nhận

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Persister ghi vào bảng nào theo loại channel

- **Decision**: Persister chọn bảng theo `channel_type` + `is_gameplay` trong envelope: `is_gameplay = true` (`match`, `party`, `team` gắn trận) → `chat.gameplay_messages` (partition ngày `created_on`, điền `created_on = created_at::date` và `INSERT … ON CONFLICT DO NOTHING` vào `chat.gameplay_partition_channels`); `world`/`area` → `chat.ephemeral_messages` (F117/F120); còn lại → `chat.messages`.
- **Rationale**: DATABASE_DESIGN §3.8, §3.8b tách bảng để purge vật lý không chạm bảng bền vững (ADR-0033); persister là nơi duy nhất ghi tin (R25).
- **Alternatives**: một bảng + cột `is_gameplay` — loại vì purge phải `DELETE` trên bảng bền vững, vi phạm phạm vi role `chat_purger`.

## R2. Kiểm tra policy trên đường gửi

- **Decision**: `PolicyResolver` đọc rule 4 cấp từ `chat.chat_policies` (theo `tenant_id`, `school_id`, `class_id` của channel), giao bằng `policy.Intersect`, cache kết quả theo `(tenant_id, school_id, class_id?, channel_type)` ở Valkey với TTL `chat_policy_cache`; invalidate bằng event `chat_policy.updated` (F120). Thiếu rule ở bất kỳ cấp → `ErrDenied`.
- **Rationale**: ngân sách độ trễ gửi (send p95 < 300 ms) không cho phép đọc PG mỗi tin; CHAT_SYSTEM §3 (cache) và CONFIG_DEFAULTS `chat_policy_cache`.
- **Alternatives**: resolve trong gateway — loại vì gateway không có logic nghiệp vụ (ADR-0074).
- **Ghi chú**: CHAT_SYSTEM §3 ghi "cache 60 s" còn VALKEY_DESIGN/CONFIG_DEFAULTS ghi 5 phút → dùng tên `chat_policy_cache` (giá trị CONFIG_DEFAULTS); báo cáo mâu thuẫn.

## R3. Hợp nhất lịch sử PostgreSQL + bộ đệm gần nhất

- **Decision**: Query `ListChannelMessages` đọc trang từ PG theo `(channel_id, seq)`, rồi trộn tin trong `ew:chat:recent:<channel_id>` có `seq` lớn hơn `MAX(seq)` của trang (chưa kịp persist), loại trùng theo `message_id`, sắp theo `seq`. Tin `held` chỉ trả cho người gửi; tin `removed`/`recalled` trả placeholder (`body = null`).
- **Rationale**: ADR-0035 mục 6; persister có độ trễ p95 ≤ 500 ms.
- **Alternatives**: chờ persister trước khi trả — loại vì PG ngừng thì lịch sử ngừng (vi phạm F115-AC-16).
