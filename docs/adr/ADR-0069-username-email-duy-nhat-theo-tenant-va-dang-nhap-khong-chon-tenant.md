# ADR-0069 — Đăng nhập xác thực SSO trước, kiểm tra tenant sau; username/email không trùng trong tenant

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Làm rõ:** ADR-0067 mục 2 · **Bổ sung:** ADR-0063, ADR-0066, ADR-0068 · **Tài liệu:** [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) · [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Người duyệt yêu cầu username/email **không trùng trong cùng một `tenant_id`**, đồng thời đăng nhập game và portal **không chọn tenant** (ADR-0063, ADR-0067). Người duyệt chốt luồng: đăng nhập bằng username + password, **SSO xác thực trước**, sau đó **identity-service kiểm tra thông tin tenant**.

Bản nháp đầu của ADR này cho phép hai người khác nhau ở hai tenant dùng cùng username, nên phải đối chiếu nhiều tài khoản khi đăng nhập (giới hạn ứng viên, authenticator Keycloak tùy biến). Người duyệt đã so sánh với phương án "chọn tenant khi đăng nhập" và chọn giữ **không chọn tenant theo luồng SSO trước**; bản nháp đó bị loại.

## Quyết định

1. **Một người, một tài khoản SSO [Confirmed]:** trên realm Keycloak, username và email duy nhất toàn realm (mặc định của Keycloak; "Duplicate emails" tắt). Người làm việc ở nhiều trường/tenant dùng **một** tài khoản, thuộc nhiều tenant qua membership và UserRole (ADR-0066).
2. **Ràng buộc trong identity-service [Confirmed]:**
   - `identity.users` (một dòng cho mỗi tài khoản, liên kết `keycloak_sub`): unique `lower(username)` và `lower(email) WHERE email IS NOT NULL AND deleted_at IS NULL`, khớp với Keycloak.
   - Dữ liệu theo tenant có `tenant_id NOT NULL` (ADR-0065): `tenant_memberships` unique `(tenant_id, user_id) WHERE deleted_at IS NULL`; bảng hồ sơ học sinh/nhân sự theo tenant; bảng nào lưu bản sao định danh đăng nhập theo tenant (ví dụ staging import) có unique `(tenant_id, lower(username))`, `(tenant_id, lower(email))`. Như vậy username/email **không bao giờ trùng trong cùng tenant**.
3. **Luồng đăng nhập (game và portal giống nhau về thứ tự):**
   1. Người dùng nhập username (game) hoặc username/email (portal) + mật khẩu, **không chọn tenant**.
   2. **SSO xác thực:** game qua `POST /api/v1/auth/game/login` → identity-service → Keycloak (ADR-0068); portal qua trang đăng nhập Keycloak chuẩn (ADR-0066). Sai username hoặc mật khẩu → `401 INVALID_CREDENTIALS`, thông điệp chung.
   3. **identity-service kiểm tra thông tin** của tài khoản (theo `keycloak_sub`): loại người dùng (học sinh cho game, nhân sự cho portal), các tenant còn hiệu lực, enrollment/UserRole, đồng bộ SIS khi đăng nhập (ADR-0061).
   4. Quyết định tenant theo ADR-0063 (game) / ADR-0067 mục 1 (portal): 1 tenant → vào thẳng; nhiều tenant → chọn **sau khi đăng nhập** (`403 TENANT_NOT_SELECTED` + `choices[]` + `login_ticket`); chỉ tenant chung → tenant chung; không có tenant hợp lệ → từ chối.
   5. Client gắn `X-Tenant-Id` ở các request sau (ADR-0065).
4. **Không dùng** đối chiếu nhiều tài khoản cùng tên, không giới hạn số ứng viên, không authenticator Keycloak tùy biến cho bước tìm tài khoản, không username nội bộ dạng `<tenant_code>~<username>`.
5. **Tránh trùng giữa các trường:** username học sinh SIS mặc định `hs.<school_code>.<student_code>`; nhân sự `gv.<school_code>.<staff_code>` hoặc đăng nhập portal bằng email cá nhân. Username do trường tự đặt phải qua kiểm tra trùng toàn realm khi tạo.
6. **Khi tạo/sửa/đồng bộ gặp trùng:**
   - Trùng trong cùng tenant → `409 USERNAME_TAKEN` / `409 EMAIL_TAKEN` (tạo tay) hoặc `reconciliation_issue` loại `duplicate_identifier` (SIS/import).
   - Trùng với tài khoản ở tenant khác: nếu xác định là **cùng một người** (theo liên kết định danh SIS hoặc Tenant Admin xác nhận theo [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md)) → thêm membership vào tài khoản sẵn có; nếu là người khác → không tạo, mở `reconciliation_issue` để trường đổi username.
7. Soft-delete (ADR-0028) không giải phóng username/email cho đến khi tài khoản được ẩn danh hóa [Confirmed] (giữ từ ADR-0067 mục 4).
8. Chống brute-force giữ như ADR-0068 mục 3 (rate limit theo thiết bị và theo username, brute-force detection Keycloak, thông điệp lỗi chung).

## Hệ quả

(+) Dùng đúng cơ chế chuẩn của Keycloak, không plugin tùy biến; mỗi lần đăng nhập chỉ một lần kiểm tra mật khẩu; học sinh không phải chọn trường. (−) Hai người khác nhau ở hai trường không được dùng cùng username; cần quy tắc đặt tên có mã trường và quy trình xử lý trùng khi đồng bộ.

## Phương án đã loại

- **Cho trùng username giữa tenant + đối chiếu nhiều tài khoản khi đăng nhập:** nhiều lần kiểm tra mật khẩu mỗi lần đăng nhập, cần authenticator Keycloak tùy biến cho portal, rủi ro tải và dò mật khẩu.
- **Chọn tenant khi đăng nhập:** cho trùng username tự do nhưng học sinh nhỏ phải tìm trường trong danh sách rất dài, dễ chọn nhầm; trái quyết định "không chọn trường" (ADR-0063).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Bản nháp: duy nhất theo tenant + đối chiếu ứng viên | Stakeholder (người duyệt) |
| 2026-11-15 | Chốt: SSO xác thực trước, kiểm tra tenant sau; một người một tài khoản SSO; ràng buộc theo tenant ở identity; loại phương án đối chiếu ứng viên và chọn tenant | Stakeholder (người duyệt) |
