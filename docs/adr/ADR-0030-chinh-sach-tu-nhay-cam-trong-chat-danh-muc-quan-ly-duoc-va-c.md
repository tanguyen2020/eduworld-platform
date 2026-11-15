# ADR-0030 — Chính sách từ nhạy cảm trong chat: danh mục quản lý được và công tắc bật/tắt kiểm duyệt

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [MODERATION](../03-social-multiplayer/MODERATION.md), [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu: chat có policy khi gặp từ nhạy cảm; danh mục được định nghĩa và có thể bật/tắt; công tắc **tắt được cả các kiểm tra nền**. Bổ sung ADR-0020.

## Quyết định

1. **Danh mục từ nhạy cảm** là dữ liệu quản lý trong Admin Portal, không hardcode:
   - **Nhóm (category):** tục tĩu, xúc phạm/miệt thị, bạo lực, tình dục, chất cấm, tự gây hại, lừa đảo/dụ dỗ, chính trị/tôn giáo nhạy cảm, mẫu thông tin cá nhân… Mỗi nhóm có `code`, tên, mô tả, `severity` mặc định (`low | medium | high | critical`), `action` mặc định, `enabled`.
   - **Mục từ (term):** `category_id`, `term`, `match_type` (`word | phrase | contains | pattern` RE2 giới hạn độ dài), `locale` (vi/en), biến thể tự sinh (không dấu, teencode, leetspeak, ký tự chen giữa) + biến thể khai báo, `severity`/`action` ghi đè, `enabled`.
   - **Ngoại lệ (allowlist)** để tránh chặn nhầm (địa danh, từ ghép).
   - Mỗi thay đổi tạo **phiên bản danh mục** (`catalog_version`); mỗi quyết định chặn/che ghi version đã dùng.
2. **Hành động theo mức:**

   | Action | Hành vi | Mặc định cho severity |
   |---|---|---|
   | `mask` | Thay từ bằng `***`, tin vẫn gửi, nhắc nhở nhẹ | low |
   | `block` | Không gửi, không lưu nội dung, báo lý do thân thiện | medium |
   | `hold_for_review` | Không gửi, vào moderation queue | high |
   | `block_and_alert` | Không gửi, mở case ưu tiên cao, báo moderator của tenant | critical (tự hại, dụ dỗ) |

3. **Phạm vi danh mục:** danh mục **platform** do System Admin định nghĩa (workspace Platform Administration/Game Live Operations), áp dụng mọi tenant; danh mục **bổ sung của tenant** [Proposed]: Tenant Admin/moderator thêm mục từ (chỉ thêm/siết chặt, không xóa/nới mục platform) trong mục An toàn & Kiểm duyệt của workspace School & Learning Management.
4. **Công tắc kiểm duyệt:**
   - Cấp **platform**, **tenant**, và **từng category**; tắt ở cấp trên thì cấp dưới không bật lại được. **Mặc định: BẬT.**
   - Chỉ System Admin bật/tắt cấp platform và tenant; Tenant Admin chỉ bật/tắt category của tenant khi được cho phép (`allow_tenant_toggle`) [Proposed].
   - **Tắt = tắt toàn bộ pipeline kiểm tra nội dung**: so khớp từ nhạy cảm **và** kiểm tra nền của ADR-0020 (thông tin cá nhân, spam/trùng lặp, bắt nạt/xúc phạm, phân tích sau khi gửi). Tắt được từng bước (`sensitive_terms`, `pii`, `spam`, `bullying`, `async_analysis`) hoặc tất cả bằng công tắc tổng.
   - **Vẫn giữ khi tắt** (giới hạn kỹ thuật/tính năng người dùng): độ dài tối đa, rate limit chống flood, quyền vào channel, block/mute/report, moderation thủ công khi có report.
   - Kiểm soát khi tắt: bắt buộc lý do; audit; event `chat_policy.moderation_switch_changed`; tùy chọn `expires_at` tự bật lại [Proposed]; banner "Đang tắt kiểm duyệt" trong portal; metric `eduworld_chat_policy_disabled{tenant}` có alert.
5. **Thực thi:** chat-service nạp danh mục đã biên dịch (Aho-Corasick cho `word/phrase/contains`, RE2 cho `pattern`) và nhận thông báo đổi version qua Valkey Pub/Sub. Tin nhắn được chuẩn hóa (NFC, chữ thường, bỏ dấu song song, gộp ký tự lặp, bỏ ký tự chen giữa) trước khi so khớp. Ngân sách p95 ≤ 50 ms cho toàn pipeline. Áp dụng cho tin nhắn, tên party/nhóm/channel và tham số quick message.
6. **Danh mục là dữ liệu nhạy cảm:** chỉ người có `chat.sensitive_terms.read` xem được; không gửi xuống client game; khi chặn chỉ trả mã nhóm, không trả từ khớp.

## Hệ quả

(+) Cập nhật danh mục không cần deploy; tenant có công cụ theo ngữ cảnh địa phương; bật/tắt có kiểm soát và truy vết. (−) Khi tắt, tin nhắn của học sinh 6–15 tuổi không được kiểm tra nội dung (kể cả thông tin cá nhân); trách nhiệm thuộc bên quyết định tắt (RISKS). Cần đội duy trì danh mục tiếng Việt và bộ test hồi quy chặn nhầm/bỏ sót (recall ≥ 95%, false positive ≤ 3% [Proposed]).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted: danh mục + công tắc chỉ cho bước so khớp từ | Stakeholder |
| 2026-11-15 | Công tắc tắt được toàn bộ pipeline kiểm tra nội dung | Stakeholder |
| 2026-11-15 | Thực thi trong chat-service; workspace theo portal mới | ADR-0036, ADR-0027 |
