# ADR-0063 — Đăng nhập Release 1.0: username/password và quy tắc xác định tenant

> **Trạng thái:** Accepted · một phần superseded by [ADR-0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Supersedes một phần:** ADR-0026 (mục 3–4), ADR-0045 (mục 4), ADR-0046 (nhánh IdP) · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) · [STUDENT_GAME_UX_CORE](../05-ux/STUDENT_GAME_UX_CORE.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)** (owner bỏ MFA/TOTP cho đăng nhập Admin Portal, 2026-11-21). Không còn hiệu lực: "+ MFA theo role" ở mục 5 (portal chỉ username/email + mật khẩu). **Còn hiệu lực:** toàn bộ quy tắc tenant khi đăng nhập.

## Bối cảnh

Người duyệt chốt: học sinh **không chọn trường** khi đăng nhập; **đăng nhập bằng username/password**; sau khi đăng nhập thành công thì xử lý theo đúng các trường hợp tenant; người **không thuộc tenant chung và không thuộc tenant SIS nào thì không cho đăng nhập**.

## Quyết định

1. **Phương thức đăng nhập của học sinh trong Release 1.0 [Confirmed]:** chỉ **username + password** (tài khoản do EduWorld cấp trong Keycloak). Các phương thức trong ADR-0045 mục 4 (QR lớp + PIN, thẻ QR cá nhân, mật khẩu hình ảnh, Device Authorization Grant, SSO Google/Microsoft của trường qua tên miền email) **không thuộc Release 1.0**; giữ thiết kế làm điểm mở rộng, bật bằng ADR mới. Màn đăng nhập game không có bước chọn trường, không có ô "nhập mã trường".
2. **Tài khoản chỉ đến từ hai nguồn:**
   - **Tenant SIS:** tạo từ dữ liệu SIS đã đồng bộ (provisioning, ADR-0021/0046/0061).
   - **Tenant chung (`common`):** do platform (System Admin) tạo/import có chủ đích, kèm khối và lớp ảo theo khối (ADR-0060). **Không còn** cơ chế tự đưa người dùng không khớp tenant nào vào tenant chung (bỏ JIT vào tenant chung của ADR-0026 mục 4).
3. **Sau khi đăng nhập thành công, backend xác định tenant theo membership trong DB:**

   | Membership | Kết quả |
   |---|---|
   | Đúng 1 tenant (tenant SIS hoặc tenant chung) | Vào thẳng tenant đó |
   | Nhiều tenant (hiếm) | `403 TENANT_NOT_SELECTED` kèm danh sách; client hiển thị hộp chọn **sau khi đăng nhập**, rồi lấy token với scope `organization:{tenant_alias}` (SSO, không nhập lại mật khẩu) |
   | Không thuộc tenant chung và không thuộc tenant SIS nào | **Không cho đăng nhập**: `403 ACCOUNT_NOT_PROVISIONED`, ghi audit, thu hồi phiên Keycloak vừa tạo |

4. Các kiểm tra khác giữ nguyên: chỉ học sinh vào game (ADR-0031), trạng thái tài khoản, tenant bị tắt, học sinh tenant SIS phải có enrollment active (ADR-0060), login sync (ADR-0061), một thiết bị (ADR-0031).
5. Portal (nhân sự nhà trường, vận hành) vẫn đăng nhập username/password + MFA theo role (ADR-0027); chọn tenant sau đăng nhập khi thuộc nhiều tenant.

## Hệ quả

(+) Luồng đăng nhập đơn giản, dễ hiểu cho học sinh và dễ kiểm thử; không có tài khoản "lạc" vào tenant chung. (−) Học sinh nhỏ tuổi phải nhớ username/password (nhà trường hỗ trợ đặt lại mật khẩu qua portal); các phương thức nhanh hơn phải chờ phiên bản sau. Đăng nhập ở giờ cao điểm dựa hoàn toàn vào băm mật khẩu Keycloak và phiên dài/refresh token (ADR-0045 mục 3, 5, 6 vẫn áp dụng).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
| 2026-11-21 | Một phần superseded by ADR-0094 (bỏ MFA/TOTP cho đăng nhập Admin Portal) | Owner dự án |
