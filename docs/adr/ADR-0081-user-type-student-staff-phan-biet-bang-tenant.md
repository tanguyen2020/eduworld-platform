# ADR-0081 — `user_type` chỉ gồm `student` và `staff`; phân biệt nhân sự trường và platform bằng tenant

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Thay một phần:** ADR-0072 (mục 1, giá trị `user_type`) · **Tài liệu:** [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) · [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

[ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) bỏ `teacher` khỏi `user_type`, để lại `student`, `staff`, `admin`. Khi rà soát, `staff` được hiểu là nhân sự tenant và `admin` là nhân sự platform. Technical Architect xác nhận cách khác: chỉ có hai loại người dùng; nhân sự trường và nhân sự platform đều là `staff`, phân biệt bằng tenant.

## Quyết định

1. `identity.users.user_type` ∈ `student | staff`. Bỏ giá trị `admin` ở mọi spec, schema, CHECK constraint và tài liệu.
2. **Nhân sự platform** là `staff` có `primary_tenant_id = NULL`; **nhân sự trường** là `staff` có `primary_tenant_id` là tenant của trường/đơn vị.
3. Quyền vẫn chỉ đến từ UserRole (ADR-0066): role platform có `tenant_id = NULL` (`scope_type = platform`), role tenant có `tenant_id`. Bất biến: user có `primary_tenant_id = NULL` chỉ được gán role platform; user có `primary_tenant_id` chỉ được gán role tenant. Một người làm cả hai việc dùng hai tài khoản riêng.
4. `Principal` mang `user_type` và `tenant_id` của phiên; middleware `portal_staff` chỉ kiểm tra `user_type = staff` (cùng `azp = eduworld-portal`); phân biệt platform/tenant dựa trên role và `tenant_id`, không dựa trên `user_type`.
5. Game vẫn chỉ cho `user_type = student` (ADR-0031).

## Hệ quả

(+) Mô hình đơn giản, một loại "nhân sự"; không cần đổi `user_type` khi chuyển người giữa các vai trò cùng phạm vi. (−) Phải sửa enum trong OpenAPI, internal spec, DATABASE_DESIGN, GLOSSARY và các tài liệu nhắc `admin` như một `user_type`.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Technical Architect |
