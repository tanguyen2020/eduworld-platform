# Research: F116 — Pipeline kiểm tra tin nhắn, danh mục từ nhạy cảm, công tắc kiểm duyệt

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Biên dịch và hoán đổi danh mục

- **Decision**: Worker phát `ew:chat:sensitive:version {scope, tenant_id?, version}` sau commit publish (consumer outbox `sensitive_catalog.published`). Mỗi instance api biên dịch ở goroutine nền: Aho–Corasick trên từ khóa đã sinh biến thể (bỏ dấu, teencode ngược, leetspeak, chen ký tự, lặp) + danh sách RE2 đã `regexp.MustCompile` có giới hạn; lưu bản tuần tự hóa + checksum vào `ew:chat:sensitive:compiled:<scope>:<version>`; hoán đổi bằng `atomic.Pointer[CompiledCatalog]` theo khóa `(tenant_id, v_platform, v_tenant)`. Đối chiếu `ew:chat:sensitive:current` mỗi 60 s.
- **Rationale**: MODERATION §11.5–§11.6; tin đang xử lý giữ con trỏ cũ (không lock trên đường gửi).
- **Alternatives**: đọc danh mục từ PG mỗi tin — loại (độ trễ); biên dịch tập trung rồi phân phối file — loại (thêm hạ tầng).
- **RE2 timeout**: Go `regexp` là RE2 (thời gian tuyến tính) nhưng không có timeout; giới hạn bằng độ dài input (≤ `chat_max_length` trần DB) + độ dài pattern ≤ 200 + đo thời gian mỗi pattern, vượt 5 ms đánh dấu pattern "chậm" và bỏ qua ở lần sau (metric).

## R2. Decision log ghi ở đâu

- **Decision**: `pipeline.Validator` trả `Decision` kèm metadata. Tin được publish (`allow/unchecked/mask/held`): metadata nằm trong envelope (`moderation.decision`, `catalog_version`, `steps_skipped`) và `chat-persister` ghi `chat.moderation_decisions` cùng lô với tin. Tin `block` (không vào stream): `DecisionLogWriter` ghi `chat.moderation_decisions` bằng buffer + batch insert bất đồng bộ (role `chat_app`, `message_id = NULL`).
- **Rationale**: R25 cấm ghi **tin chat** vào PG từ đường realtime; decision log là bảng TA chỉ có metadata, không nội dung (MODERATION §12.4).
- **Alternatives**: đẩy decision của tin `block` vào JetStream — loại vì stream `CHAT_MESSAGES` chỉ chứa tin đã được chấp nhận.

## R3. Lưu tin `held`

- **Decision**: Tin `hold_for_review`/`block_and_alert` vẫn cấp `seq` và publish JetStream với `moderation.decision = hold_for_review|block_and_alert` nhưng không fan-out; persister ghi `moderation_state = held`. Lịch sử chỉ trả tin held cho người gửi. Việc duyệt/gỡ tin held do F121 định nghĩa (`chat.message_removed` đã có trong EVENT_DESIGN).
- **Rationale**: CHAT_SYSTEM §5.2 (opt hold), DATABASE_DESIGN §3.8 ("tin held có dòng").
- **Ghi chú**: khoảng trống `seq` mà người nhận thấy do tin held là bình thường (giống tin gửi lỗi, F115).
