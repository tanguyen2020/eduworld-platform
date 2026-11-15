# ADR-0039 — ORM ent cho Go (tương đương Entity Framework), migration bằng Atlas

> **Trạng thái:** Accepted · supersedes phần ORM/migration của [ADR-0025](ADR-0025-thu-vien-nen-tang-cho-go-backend.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md), [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu Entity Framework; vì giữ Golang nên dùng ORM Go tương đương.

## Quyết định

- **`ent` (entgo.io)** làm ORM: schema khai báo bằng code Go (code-first như EF Core), query builder type-safe, transaction, eager loading, hook/interceptor.
- **Global query filter** (tương đương `HasQueryFilter`): interceptor bắt buộc thêm `tenant_id = ?` và `deleted_at IS NULL`; bỏ filter (System Admin liên tenant, Thùng rác) chỉ qua API riêng có audit. **Soft-delete** bằng hook đổi delete thành update `deleted_at` (ADR-0028). RLS vẫn bật ở DB.
- **Mixin chung:** `id` (UUIDv7), `tenant_id`, `created_at/by`, `updated_at/by`, `deleted_at/by`, `version` (optimistic concurrency, tương đương RowVersion).
- **Migration:** Atlas versioned migrations sinh từ ent schema, review SQL trong merge request, chạy bằng job migrate riêng từng service; không auto-migrate khi khởi động.
- Truy vấn phức tạp phía Query (báo cáo, full-text, partition) viết SQL tường minh qua `entsql`/`pgx` ở tầng infrastructure, có test, vẫn bắt buộc tenant + soft-delete.
- Driver `pgx/v5`.

## Hệ quả

(+) Code-first, an toàn kiểu, filter tenant/soft-delete tự động. (−) Phụ thuộc codegen của ent; SQL phức tạp vẫn cần viết tay.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted, supersede ORM/migration của ADR-0025 | Stakeholder |
