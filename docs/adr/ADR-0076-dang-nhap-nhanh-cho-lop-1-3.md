# ADR-0076 — Đăng nhập nhanh cho lớp 1–3 trong bản phát hành đầu: thẻ QR cá nhân + PIN hoặc mật khẩu hình

> **Trạng thái:** Rejected · **Ngày:** 2026-11-15 · **OQ:** OQ-03 · **Nếu Accepted sẽ thay một phần:** ADR-0063 (chỉ username/password), ADR-0045 mục 4 · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [STUDENT_GAME_UX_CORE](../05-ux/STUDENT_GAME_UX_CORE.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Release 1.0 chỉ cho đăng nhập username + mật khẩu ([ADR-0063](ADR-0063-dang-nhap-release-1-0-username-password-va-quy-tac-tenant.md)), nhập lại mỗi lần mở game ([ADR-0064](ADR-0064-khong-tu-dang-nhap-khi-mo-lai-game.md)); khóa tạm sau 5 lần sai. Học sinh lớp 1–2 đọc và gõ còn chậm; một phòng máy 35–40 em có thể mất nhiều phút cho riêng việc đăng nhập, và bạn cùng lớp có thể vô tình hoặc cố ý làm khóa tài khoản nhau ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) CLS-02). Không còn giáo viên mở "mã lớp" (ADR-0072).

## Quyết định (đề xuất)

1. Học sinh lớp 1–3 có thêm cách đăng nhập **thẻ QR cá nhân** (in từ portal bởi `school_admin`) kèm **PIN 4 số** hoặc **mật khẩu hình** (chọn 3 hình theo thứ tự); quét bằng camera thiết bị hoặc nhập mã ngắn in trên thẻ.
2. Xác thực vẫn qua Keycloak bằng authenticator tùy biến, gọi qua `/auth/game/login` của identity-service (giữ ADR-0068); không lưu token trên đĩa (giữ ADR-0064).
3. Khóa tạm theo cặp (tài khoản, thiết bị) thay vì chỉ theo tài khoản; giới hạn tốc độ theo thiết bị.
4. Thu hồi/in lại thẻ trên portal, có audit. Thẻ chỉ chứa mã định danh không đoán được, không chứa tên đầy đủ.
5. Lớp 4–9 giữ username/password; trường có thể bật thẻ QR cho các khối khác.

## Hệ quả

(+) Trẻ nhỏ tự đăng nhập được; giảm thời gian đầu tiết và yêu cầu hỗ trợ. (−) Phát triển authenticator Keycloak, quy trình in/thu hồi thẻ, pen-test bổ sung (đã có trong kế hoạch Phase 6 của ADR-0045).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Từ chối: không làm đăng nhập thẻ QR + PIN/mật khẩu hình | Technical Architect |
