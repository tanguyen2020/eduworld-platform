# ADR-0026 — Đăng nhập theo tenant: xác định tenant, kiểm tra chéo, mặc định tenant chung

> **Trạng thái:** Accepted · supersedes phần ánh xạ Organization của [ADR-0006](ADR-0006-keycloak-mot-realm-dung-organizations-identity-brokering.md) · một phần superseded by ADR-0095 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md), [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md), [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi ADR-0063** (2026-11-15): Release 1.0 học sinh chỉ đăng nhập username/password; không tự đưa người dùng vào tenant chung (bỏ JIT vào tenant chung); không thuộc tenant chung và không thuộc tenant SIS nào thì không cho đăng nhập. Nhận diện SSO theo tên miền email không thuộc Release 1.0. **Còn hiệu lực:** Tenant = Keycloak Organization, kiểm tra chéo membership, một tenant mỗi phiên, chọn tenant sau đăng nhập khi thuộc nhiều tenant.

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md):** membership tenant của học sinh (mục 2) suy ra từ hồ sơ học sinh hiệu lực ở trường (`org.students`), không từ enrollment; "học sinh tenant thường phải có enrollment active" đổi thành "phải có hồ sơ học sinh hiệu lực ở trường của tenant". **Còn hiệu lực:** phần còn lại (mục 3–4 đã thay bởi ADR-0063).

## Bối cảnh

Stakeholder yêu cầu phân biệt được học sinh thuộc tenant nào để kiểm tra đúng khi đăng nhập; học sinh không thuộc tenant nào dùng tenant chung. Mô hình tenant theo ADR-0005.

## Quyết định

1. **Mỗi Tenant là một Keycloak Organization** (`platform.tenants.keycloak_org_id`, `keycloak_org_alias` = mã tenant). Tenant loại `unit` có một Organization chung cho mọi trường; IdP của đơn vị/trường gắn vào Organization đó. Tenant `common` cũng là một Organization.
2. **Membership do dữ liệu EduWorld quyết định.** identity-service đồng bộ membership Keycloak từ dữ liệu SIS/EduWorld: học sinh là member của đúng một tenant (tenant chứa trường có enrollment active); nhân sự là member của mọi tenant có UserRole. Chuyển sang trường thuộc tenant khác → gỡ org cũ, thêm org mới, thu hồi session cũ. Chuyển trường trong cùng tenant `unit` không đổi membership.
3. **Xác định tenant khi đăng nhập — học sinh KHÔNG phải chọn trường [Confirmed, người duyệt 2026-11-15]:**
   - **Mặc định (game):** màn đăng nhập chỉ có nút "Đăng nhập" và các cách đăng nhập nhanh (ADR-0045); không có bước chọn trường. Sau khi xác thực, backend xác định tenant theo membership trong DB (dữ liệu SIS, ADR-0046/0061): đúng 1 tenant → dùng tenant đó; nhiều tenant (hiếm, ví dụ nhân sự làm việc ở nhiều trường) → `403 TENANT_NOT_SELECTED`, client hiển thị danh sách **sau khi đăng nhập** để chọn; không thuộc tenant nào → **tenant chung `common`**.
   - **Trường dùng SSO riêng (Google/Microsoft của trường):** Keycloak nhận diện tổ chức theo **tên miền email** học sinh nhập ở bước đầu (identity-first login của Keycloak Organizations, mỗi Organization khai báo domain) rồi chuyển sang IdP của trường; học sinh không chọn trường.
   - **Nguồn gợi ý tenant không cần học sinh thao tác:** QR lớp (chứa tenant), thiết bị phòng máy cấu hình sẵn (`device.cfg`). Khi có gợi ý, client gửi scope `organization:{tenant_alias}`.
   - **Lối dự phòng:** "Nhập mã trường" chỉ nằm trong mục "Cách đăng nhập khác" (cho trường hợp SSO riêng mà học sinh không có email trường). Portal vẫn dùng URL `/t/{tenant_alias}` hoặc chọn sau đăng nhập.
4. **Backend luôn kiểm tra chéo** tại `POST /api/v1/auth/session` và mọi request sau đó:
   - `(iss, sub)` → User → danh sách tenant mà user là thành viên.
   - Token có claim `organization` thì tenant đó phải nằm trong danh sách; không khớp → `403 TENANT_MISMATCH`, audit + security event, không tạo session.
   - Tenant bị tắt → `TENANT_ACCESS_DISABLED`.
   - Học sinh tenant thường phải có enrollment active (SIS); học sinh tenant chung không cần SIS.
   - **JIT provisioning:** khớp bản ghi SIS của một tenant → tạo trong tenant đó (cách tìm SIS nguồn: ADR-0046); không khớp tenant nào và người dùng không chọn tenant → tạo trong **tenant chung**, School ảo "EduWorld chung" [Confirmed].
5. **Một tenant hoạt động cho mỗi phiên.** `Principal.tenant_id` lấy từ bước 4; repository dùng `TenantScope`; RLS `SET LOCAL app.tenant_id`. Đổi tenant = xin token mới (SSO). System Admin không gắn tenant, dùng chế độ liên tenant của ADR-0027.
6. **Username duy nhất toàn realm** (`hs.<school_code>.<student_code>`) nhưng **không** dùng để suy ra tenant; tenant chỉ lấy từ claim đã ký và membership trong DB.

## Hệ quả

(+) Học sinh không thể vào ngữ cảnh của tenant khác; học sinh không thuộc tenant nào vẫn chơi và học được. (−) Số Keycloak Organization bằng số tenant, cần kiểm thử tải; hành vi scope `organization:{alias}` phải kiểm chứng trên bản Keycloak triển khai (backend vẫn chặn đúng nhờ bước 4); tenant chung cần nội dung được gán (ADR-0029).

## Phương án đã loại

Realm riêng cho mỗi tenant (vận hành nặng, khó SSO cho nhân sự nhiều tenant); suy ra tenant từ username/email (dễ giả mạo); từ chối học sinh không thuộc tenant (stakeholder thay bằng tenant chung).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR: tenant = School, School là Keycloak Organization | Stakeholder |
| 2026-11-15 | Đổi sang Tenant (trường/đơn vị/chung) là Organization; thêm tenant chung | Stakeholder |
| 2026-11-15 | `TENANT_NOT_SELECTED` thống nhất HTTP 403 | Vòng đồng bộ tài liệu |
| 2026-11-15 | Học sinh không phải chọn trường khi đăng nhập; trường SSO riêng nhận diện theo tên miền email; nhập mã trường chỉ là lối dự phòng | Stakeholder (người duyệt) |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
| 2026-11-21 | Một phần superseded by ADR-0095 (bỏ khái niệm lớp — học sinh theo khối) | Owner |
