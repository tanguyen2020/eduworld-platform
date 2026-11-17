# Data Model: F057 — Workspace D — Platform Administration: tenant, gán/clone, thùng rác, danh mục, cấu hình hệ thống

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F057 là frontend, không tạo hay sửa bảng. Tenant, danh mục, gán (`infra.tenant_resource_assignments`), job clone, Thùng rác, cài đặt platform thuộc module identity và module sở hữu tài nguyên (F010, F045, F016).

Trạng thái phía client (không bền vững):

| Trạng thái | Nơi giữ | Ghi chú |
|---|---|---|
| Lựa chọn bộ lọc tenant | `sessionStorage` khóa `ew.portal.tenantFilter.<userId>` (mảng `tenant_id`, rỗng = "Tất cả") | Per-viewer, theo phiên làm việc; bọc try/catch; xóa khi đăng xuất |
| Job clone/import đang theo dõi | RAM | Dừng polling khi `succeeded`/`partially_failed`/`failed`/`cancelled` |
| Cache truy vấn | RAM (TanStack Query) | Khóa chứa danh sách `tenant_ids` đang lọc |

Schema dùng (chỉ đọc định nghĩa): `TenantSummary`, `TenantType`, `ResourceType`, `TenantAssignmentRequest`, `TenantAssignment`, `TenantCloneRequest`, `TenantCloneJob`, `TrashItem`, `Country`, `CountryInput`, `Province`, `ProvinceInput`, `ProvinceSuccessionInput`, `SchoolImportJob`, `StudentCleanupSettings`, `StudentCleanupRun`, `ChatRetentionSettings` trong `specs/openapi/eduworld-api.v1.yaml`.
