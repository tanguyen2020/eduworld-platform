# ADR-0005 — Mô hình multi-tenancy: shared database + `tenant_id` + RLS

> **Trạng thái:** Accepted · một phần superseded by ADR-0095 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [MULTITENANCY](../06-architecture/MULTITENANCY.md), [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md):** phân cấp ở mục 1 là `Platform → Tenant → School`; học sinh thuộc trường với một khối (`grade_level`), không còn `Class`. **Còn hiệu lực:** toàn bộ mô hình multi-tenancy.

## Bối cảnh

Khách hàng có thể là một trường đơn lẻ hoặc một đơn vị quản lý nhiều trường (Sở/Phòng GD, hệ thống trường). Có học sinh không thuộc trường/đơn vị nào. Dữ liệu phải tách theo tenant và mỗi người dùng tenant chỉ làm việc trên một tenant tại một thời điểm. Liên quan: đăng nhập theo tenant (ADR-0026), phân quyền Admin Portal (ADR-0027), soft-delete (ADR-0028), gán/clone dữ liệu (ADR-0029).

## Quyết định

1. **Phân cấp** `Platform → Tenant → School → Class`. Entity **Tenant** (`tenant_id`, `code`, `name`, `tenant_type ∈ {school, unit, common}`, `status`, `keycloak_org_id`):
   - `school`: một trường độc lập, chứa đúng 1 School.
   - `unit`: một đơn vị chứa nhiều School; dữ liệu mọi trường trong đơn vị cùng `tenant_id`, `school_id` là chiều phân loại bên trong tenant.
   - `common`: **tenant chung** duy nhất (`code = common`) cho học sinh không thuộc tenant nào (ADR-0026), có một School ảo "EduWorld chung" [Confirmed].
2. **Khóa cô lập là `tenant_id`.** Mọi bảng nghiệp vụ có `tenant_id NOT NULL`. Dữ liệu do platform sở hữu (cấu hình game, nội dung dùng chung) có `tenant_id = NULL` + `owner_scope = platform`; tenant chỉ thấy khi được gán (ADR-0029).
3. **Mọi truy vấn lọc theo `tenant_id`**, hai lớp bảo vệ:
   - **Ứng dụng (bắt buộc):** use case nhận `Principal` có `tenant_id` hoạt động; repository bắt buộc nhận `TenantScope` và thêm `tenant_id = $tenant` cùng điều kiện soft-delete (ADR-0028). Global filter của ent thực thi điều này (ADR-0039).
   - **Database (defense-in-depth):** RLS trên mọi bảng tenant-scoped, policy `tenant_id = current_setting('app.tenant_id')::uuid`, set bằng `SET LOCAL` mỗi transaction; không set thì không trả dòng nào. Phiên System Admin xem liên tenant dùng role DB `<svc>_platform` (BYPASSRLS) và vẫn có điều kiện `tenant_id` tường minh, có audit.
4. **Một tenant hoạt động tại một thời điểm** cho mọi người dùng thuộc tenant; chỉ System Admin có chế độ xem liên tenant (ADR-0027).

## Hệ quả

(+) Cô lập dữ liệu rõ ràng với chi phí vận hành thấp. (−) Mọi truy vấn phải mang tenant scope; cần test tự động phát hiện truy vấn thiếu điều kiện.

## Phương án đã loại

DB-per-tenant (chi phí vận hành với hàng trăm trường); schema-per-tenant (migration phức tạp). Khách hàng lớn yêu cầu cô lập vật lý → ADR mới.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (tenant = School, khóa `school_id`) | Phase 0 |
| 2026-11-15 | Accepted; tenant = trường/đơn vị/chung, khóa đổi sang `tenant_id`, thay khái niệm Organization | Stakeholder |
| 2026-11-15 | Áp dụng trong từng database của service; role DB theo service | ADR-0037 |
| 2026-11-21 | Một phần superseded by ADR-0095 (bỏ khái niệm lớp — học sinh theo khối) | Owner |
