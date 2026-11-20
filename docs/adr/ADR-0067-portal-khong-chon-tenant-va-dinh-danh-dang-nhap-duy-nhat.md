# ADR-0067 — Portal đăng nhập không chọn tenant; username và email là định danh duy nhất

> **Trạng thái:** Accepted · một phần superseded by [ADR-0091](ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Bổ sung:** ADR-0063, ADR-0065, ADR-0066 · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) · [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Mục 2–3 được làm rõ bởi ADR-0069** (2026-11-15): một người một tài khoản SSO (username/email duy nhất trên Keycloak); identity-service có thêm ràng buộc duy nhất theo `tenant_id` trên dữ liệu theo tenant; đăng nhập xác thực SSO trước rồi kiểm tra tenant; trùng với tenant khác mà là cùng một người thì thêm membership. Toàn bộ ADR này vẫn còn hiệu lực.

> [!WARNING]
> **Một phần được thay bởi ADR-0091.** Không còn hiệu lực: chọn/đổi tenant của portal bằng token mới với scope `organization:{alias}` (nay dùng `X-Tenant-Id` + membership). **Còn hiệu lực:** quy tắc username/email, chọn tenant sau đăng nhập.

## Bối cảnh

Người duyệt yêu cầu: (1) đăng nhập CMS/LMS cũng **không cần chọn tenant**, xử lý giống các trường hợp đăng nhập game; (2) học sinh và nhân sự có **username và email là duy nhất**, không được trùng nhau trong cùng một tenant.

Khi đăng nhập không chọn tenant, hệ thống chỉ nhận username (hoặc email) và mật khẩu. Nếu cùng một username tồn tại ở hai tenant thì không xác định được tài khoản. Vì vậy định danh đăng nhập phải duy nhất trên toàn hệ thống, điều kiện "không trùng trong cùng tenant" là hệ quả.

## Quyết định

1. **Portal đăng nhập không chọn tenant [Confirmed]:** màn đăng nhập portal chỉ có username/email + mật khẩu (+ MFA cho role platform). Bỏ yêu cầu URL `/t/{tenant_alias}`. Sau khi đăng nhập:

   | Trường hợp | Kết quả |
   |---|---|
   | Có role platform (System Admin, game designer, live-ops…) | Vào phạm vi platform; chọn tenant đích khi thao tác ghi (ADR-0027, ADR-0065) |
   | Nhân sự thuộc đúng 1 tenant | Vào thẳng tenant đó |
   | Nhân sự thuộc nhiều tenant | Chọn tenant **sau khi đăng nhập**; đổi tenant ở bộ chọn trên topbar (lấy token mới, đổi `X-Tenant-Id`) |
   | Không có role portal nào còn hiệu lực ở bất kỳ tenant nào và không có role platform | **Không cho đăng nhập**: `403 PORTAL_ACCESS_STAFF_ONLY` |
   | Tài khoản học sinh | **Không cho đăng nhập** (ADR-0066) |

2. **Định danh đăng nhập duy nhất toàn hệ thống [Confirmed về yêu cầu; phạm vi toàn hệ thống là hệ quả kỹ thuật]:**
   - **Username:** duy nhất trên toàn realm Keycloak và toàn bảng `identity.users`, so sánh không phân biệt hoa thường, sau khi chuẩn hóa (trim, NFC, chữ thường). Học sinh SIS dùng mẫu `hs.<school_code>.<student_code>`, nhân sự `gv.<school_code>.<staff_code>` hoặc username do trường cấp đã kiểm tra trùng.
   - **Email:** nếu có thì duy nhất toàn hệ thống (không phân biệt hoa thường). Nhân sự **bắt buộc** có email; học sinh không bắt buộc. Keycloak tắt "Duplicate emails", bật "Login with email" cho portal; game chỉ dùng username (ADR-0063).
   - **DB:** unique index `lower(username)` và `lower(email) WHERE email IS NOT NULL AND deleted_at IS NULL`; thêm ràng buộc theo tenant trên bảng membership `(tenant_id, user_id)` để mỗi người chỉ có một membership mỗi tenant.
   - **Một người, một tài khoản:** người làm việc ở nhiều trường/tenant dùng một tài khoản với nhiều membership và UserRole (ADR-0066), không tạo tài khoản trùng.
3. **Khi đồng bộ SIS hoặc import gặp trùng:** username/email trùng với tài khoản khác → không tạo/không ghi đè, tạo `reconciliation_issue` loại `duplicate_identifier` để Tenant Admin xử lý (đổi username do trường cấp, hoặc gộp tài khoản theo quy trình ACCOUNT_PROVISIONING). Mã lỗi khi tạo/sửa thủ công: `409 USERNAME_TAKEN`, `409 EMAIL_TAKEN`.
4. Soft-delete (ADR-0028) không giải phóng username/email cho đến khi tài khoản được ẩn danh hóa [Confirmed], để tránh nhầm danh tính.

## Hệ quả

(+) Đăng nhập không cần chọn tenant mà vẫn xác định đúng một tài khoản ở cả game và portal; không có tài khoản trùng. (−) Trường không tự đặt được username trùng với trường khác; cần quy tắc đặt tên có mã trường và quy trình xử lý trùng khi đồng bộ.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
| 2026-11-17 | Một phần superseded by ADR-0091 (portal đăng nhập bằng form riêng qua backend) | Technical Architect |
