# ADR-0029 — System Admin gán dữ liệu cho tenant và clone dữ liệu giữa tenant

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [MULTITENANCY](../06-architecture/MULTITENANCY.md), [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu: dữ liệu System Admin tạo có thể gán cho các tenant khác sử dụng; System Admin có thể clone dữ liệu từ tenant này sang tenant khác.

## Quyết định

1. **Dữ liệu dùng chung** do System Admin tạo có `tenant_id = NULL`, `owner_scope = platform`. Tenant **chỉ thấy** khi được gán trong bảng `tenant_resource_assignments(resource_type, resource_id, tenant_id, assigned_by, assigned_at, deleted_at)` nằm ở database của service sở hữu tài nguyên; identity-service điều phối.
   - Truy vấn của tenant = dữ liệu `tenant_id = $tenant` **hoặc** dữ liệu platform có assignment active; cả hai lọc `deleted_at IS NULL`.
   - Dữ liệu được gán **chỉ đọc** với tenant; muốn sửa thì System Admin clone về tenant.
   - Gỡ gán = soft-delete assignment; lịch sử học tập đã tham chiếu version giữ nguyên.
   - Tùy chọn "gán cho mọi tenant hiện tại và tenant mới" [Confirmed].
2. **Clone giữa tenant (chỉ System Admin):** chọn tenant nguồn, bản ghi, tenant đích → bản sao **sâu** với ID mới, `tenant_id = đích`, trạng thái `draft` cho nội dung có workflow, cột nguồn gốc `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by`.
   - Clone kéo theo phụ thuộc và ánh xạ lại tham chiếu; phụ thuộc là dữ liệu platform thì giữ tham chiếu.
   - **Không bao giờ clone dữ liệu cá nhân/học tập của học sinh**.
   - Chạy bất đồng bộ như **saga** do identity-service điều phối qua internal platform API của từng service, có `Idempotency-Key`, báo cáo kết quả, audit cho cả hai tenant.
3. **Quyền:** chỉ System Admin có `platform.tenant_data.assign` / `platform.tenant_data.clone`, qua workspace Platform Administration (MFA, gateway nội bộ — ADR-0027).

## Hệ quả

(+) Nội dung chuẩn tạo một lần, phân phối nhiều tenant; tenant mới khởi tạo nhanh. (−) Truy vấn tenant có thêm nhánh assignment (cần index); clone sâu cần đồ thị phụ thuộc cho từng loại tài nguyên.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
| 2026-11-15 | Assignment nằm ở DB service sở hữu; clone là saga | ADR-0036, ADR-0037 |
| 2026-11-15 | Client `eduworld-portal` + MFA thay `eduworld-admin` | ADR-0027 |
