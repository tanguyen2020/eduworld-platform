# ADR-0066 — Đăng nhập riêng cho Admin Portal (CMS/LMS) và phân quyền Role / UserRole

> **Trạng thái:** Accepted · một phần superseded by [ADR-0091](ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) · một phần superseded by ADR-0072 · một phần superseded by [ADR-0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md) · một phần superseded by [ADR-0096](ADR-0096-chan-hoc-sinh-o-portal-khong-tinh-la-dang-nhap-sai.md) · **Ngày:** 2026-11-15 · **OQ:** OQ-07 · **Bổ sung:** ADR-0027 · **Tài liệu:** [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0096](ADR-0096-chan-hoc-sinh-o-portal-khong-tinh-la-dang-nhap-sai.md)** (2026-11-22, Proposed). Không còn hiệu lực: lớp chặn trong Keycloak ("điều kiện role trong authentication flow", mục 1) — Keycloak từ chối được tính là một lần đăng nhập sai và khóa tạm cả tài khoản game của học sinh. **Còn hiệu lực:** phần còn lại; học sinh vẫn bị chặn ở portal bằng hai lớp backend (`portalLogin` của module `identity` sau khi Keycloak cấp token, thu hồi phiên; guard `portal_staff` ở mọi route portal).

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md).** Không còn hiệu lực: role `teacher` và `tenant_moderator` trong danh sách role seed (mục 5); thêm role platform `safety_moderator`. **Còn hiệu lực:** đăng nhập portal riêng, mô hình Permission → Role → UserRole.

> [!WARNING]
> **Một phần được thay bởi ADR-0091.** Không còn hiệu lực: mục 1 "trang đăng nhập bằng theme Keycloak". **Còn hiệu lực:** đăng nhập riêng cho portal, chặn học sinh hai lớp, mô hình Role/UserRole.

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)** (owner bỏ MFA/TOTP cho đăng nhập Admin Portal, 2026-11-21). Không còn hiệu lực: "MFA bắt buộc với role platform" ở mục 1. **Còn hiệu lực:** màn đăng nhập riêng (theo ADR-0091), học sinh không vào được, mô hình Permission → Role → UserRole.

## Bối cảnh

Người duyệt yêu cầu: CMS/LMS có **màn hình đăng nhập riêng**; **học sinh không được phép đăng nhập** vào CMS/LMS; CMS/LMS **phân quyền theo Role và UserRole**.

## Quyết định

1. **Màn đăng nhập riêng:** Admin Portal (`web/apps/portal`, client `eduworld-portal`) có trang đăng nhập riêng (theme Keycloak dành cho vận hành, khác giao diện game), tại tên miền portal. Đăng nhập username + password; MFA bắt buộc với role platform (ADR-0027). Thuộc nhiều tenant → chọn tenant sau đăng nhập.
2. **Học sinh không được đăng nhập portal [Confirmed]:**
   - Keycloak: client `eduworld-portal` chỉ cho người có realm role `tenant_staff` hoặc `platform_staff` hoàn tất đăng nhập (điều kiện role trong authentication flow); học sinh (realm role `student`) bị từ chối ngay ở màn đăng nhập portal.
   - Backend: mọi route portal kiểm tra `azp = eduworld-portal` và `user_type ≠ student`; vi phạm → `403 PORTAL_ACCESS_STAFF_ONLY`, ghi audit. Hai lớp kiểm tra độc lập (Keycloak có thể cấu hình sai).
   - Đối xứng với ADR-0031 (game chỉ cho học sinh).
3. **Mô hình phân quyền Role / UserRole:**
   - **Permission**: quyền nguyên tử, **định nghĩa trong code** (catalog cố định, ví dụ `lms.assignment.write`, `game.quest.write`), có thuộc tính `scope_kind` (`tenant` hoặc `platform`).
   - **Role**: tập permission, lưu trong DB (`identity.roles`, `identity.role_permissions`): `role_id`, `code`, tên song ngữ, mô tả, `role_scope_kind` (`tenant` | `platform`), `is_system` (role hệ thống không sửa/xóa được), `tenant_id` (NULL = role dùng chung toàn platform; có giá trị = role tùy biến của tenant) [Confirmed: cho phép tenant tạo role tùy biến từ permission tenant].
   - **UserRole**: gán role cho người dùng trong một phạm vi (`identity.user_roles`): `user_id`, `role_id`, `tenant_id`, `scope_type` (`platform` | `tenant` | `school` | `class` | `subject`), `scope_id`, `valid_from`, `valid_to`, `granted_by`, `source` (`sis` | `manual`), `deleted_at`. Thay cho tên `role_assignments` trước đây.
   - **Ràng buộc bắt buộc:** role `tenant` không được chứa permission `platform`/`game.*`/`competition.config.*` (kiểm tra khi lưu role và khi khởi động); UserRole của role `platform` chỉ có `scope_type = platform`; học sinh không được gán role portal.
   - **Đánh giá quyền:** `Principal` = hợp các permission của mọi UserRole còn hiệu lực trong tenant hiện tại (header `X-Tenant-Id`, ADR-0065) và phạm vi tài nguyên; cache 60 giây; mọi thay đổi role/UserRole phát event `role.updated` / `user_role.changed` để làm mới cache và menu.
4. **Màn quản trị trong portal:** quản lý Role (danh sách, tạo/sửa role tùy biến, chọn permission theo nhóm, xem role hệ thống chỉ đọc), quản lý UserRole (gán/thu hồi theo người dùng, theo trường/lớp/môn, thời hạn), nhật ký thay đổi phân quyền. Tenant Admin quản lý role/UserRole trong tenant mình; System Admin quản lý role platform và mọi tenant.
5. Role hệ thống seed sẵn (giữ danh sách hiện có): tenant — `tenant_admin`, `school_admin`, `academic_manager`, `teacher`, `tenant_content_editor`, `tenant_content_reviewer`, `tenant_moderator`; platform — `system_admin`, `content_admin`, `content_reviewer`, `game_designer`, `liveops`, `competition_admin`.

## Hệ quả

(+) Tách bạch hoàn toàn người dùng game và người vận hành; phân quyền quản lý được trên giao diện, linh hoạt theo trường. (−) Role tùy biến làm tăng số tổ hợp cần kiểm thử; phải có kiểm tra chặt để không lọt permission platform vào role tenant.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
| 2026-11-15 | Một phần superseded by ADR-0072 (bỏ vai trò giáo viên) | Technical Architect |
| 2026-11-17 | Một phần superseded by ADR-0091 (portal đăng nhập bằng form riêng qua backend) | Technical Architect |
| 2026-11-21 | Một phần superseded by ADR-0094 (bỏ MFA/TOTP cho đăng nhập Admin Portal) | Owner dự án |
| 2026-11-22 | Một phần superseded by ADR-0096 (Proposed): chặn học sinh ở portal không còn nằm trong flow Keycloak để không bị brute-force tính là đăng nhập sai | E2E Portal v2 |
