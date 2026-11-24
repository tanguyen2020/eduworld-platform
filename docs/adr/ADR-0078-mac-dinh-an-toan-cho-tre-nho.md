# ADR-0078 — Mặc định an toàn cho trẻ nhỏ: bước PII/grooming luôn chạy, giờ giới nghiêm platform, siết tenant `common`

> **Trạng thái:** Accepted · một phần supersedes ADR-0030 · Superseded một phần bởi [ADR-0101](ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) (mục 5, phần Windows) · **Ngày:** 2026-11-15 · **OQ:** OQ-17 · **Nếu Accepted sẽ thay một phần:** ADR-0030 (công tắc tắt được bước PII) · **Tài liệu:** [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) · [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Superseded một phần bởi [ADR-0101](ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) (bỏ Windows, 2026-11-24).** Không còn hiệu lực: câu "trên Windows, game client mặc định coi là máy dùng chung … trừ khi bộ cài được chọn máy cá nhân" ở mục 5 (không còn bản Windows hay bộ cài; PC phòng máy chơi bản Web, chỉ giữ refresh token trong RAM). **Còn hiệu lực:** phần còn lại, gồm màn "Tiếp tục là <nickname>?" với nút "Không phải tôi" trên mọi nền tảng tự vào lại bằng refresh token.

## Bối cảnh

ADR-0020 áp một bộ quy tắc chat cho mọi lớp; ADR-0030 cho phép System Admin tắt từng bước kiểm duyệt, kể cả bước PII. Khung giờ truy cập game do trường tự đặt, không có mặc định, nên học sinh chơi ở nhà có thể chat lúc nửa đêm. Tenant `common` ([MULTITENANCY §8.3](../06-architecture/MULTITENANCY.md)) coi mọi học sinh là "cùng trường", tức hàng nghìn người lạ, không có trường thu consent ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) SAF-03, SAF-04).

## Quyết định

1. Bước lọc **PII** và **tín hiệu grooming** luôn chạy, không nằm trong công tắc kiểm duyệt của ADR-0030. Các bước khác giữ công tắc, nhưng mọi lần tắt bắt buộc có `expires_at` (tối đa 24 giờ [Confirmed]) và phê duyệt của hai người.
2. **Giờ giới nghiêm mặc định ở cấp platform** cho game và chat (ví dụ 21:30–06:30 [Confirmed]); trường chỉ được siết thêm. Áp như nhau cho mọi học sinh (không trái R15).
3. **Tenant `common`:** tắt mặc định kết bạn, DM và World chat; consent do platform thu trực tiếp (ADR-0077); giờ giới nghiêm bắt buộc; kiểm duyệt bởi Platform Safety Team (ADR-0072).
4. Cung cấp preset chính sách "chỉ quick message + emote" mà trường có thể chọn khi onboarding; preset áp cho cả trường, không theo khối (giữ R15), trên mọi channel kể cả channel luôn có `match`/`team`/`party`: channel vẫn tự mở và luôn hoạt động (ADR-0020 mục 6), preset chỉ thu hẹp loại tin thành quick message + emote.

5. **Thiết bị dùng chung (bổ sung ADR-0086):** trên Windows, game client mặc định coi là máy dùng chung (`shared_device = true`, không lưu refresh token) trừ khi bộ cài được chọn "máy cá nhân". Trên mọi nền tảng, khi tự vào lại bằng refresh token, game hiện màn "Tiếp tục là <nickname>?" với nút "Không phải tôi" (đăng xuất, xóa token).

## Hệ quả

(+) Mặc định an toàn hơn cho trẻ 6–15 tuổi mà vẫn giữ một bộ quy tắc chung. (−) Thay một phần ADR-0030; cần cập nhật CHAT_SYSTEM, MODERATION, CHILD_SAFETY, chat-policy schema và test.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Accepted; bổ sung mục 5 (Windows mặc định máy dùng chung, màn Tiếp tục là…) | Technical Architect |
| 2026-11-17 | Làm rõ phạm vi preset: áp cả channel luôn có, chỉ thu hẹp loại tin, không tắt channel | Technical Architect |
| 2026-11-24 | Superseded một phần bởi ADR-0101 (mục 5, phần Windows mặc định máy dùng chung) | Owner dự án |
