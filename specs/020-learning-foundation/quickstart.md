# Quickstart: F020 — Nền module learning: projection, mô hình nội dung, gán platform→tenant, thùng rác tenant

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F003, F010 (F045 Done để chạy bước 6–9 qua portal; nếu chưa, chạy bằng facade test).
- Lệnh: `make up`, `make migrate`, `make seed`, `make seed-check`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT §6](../../docs/11-operations/LOCAL_DEVELOPMENT.md#6-chạy-từng-phần)).
- Tài khoản demo: `sa.admin` (system_admin), `ad.truong.a` (school_admin, tenant `th-demo-a`), `ad.truong.b` (school_admin, tenant `pgd-demo`), `ad.donvi` (tenant_admin, `pgd-demo`).
- Nội dung mẫu: F020 chưa có loại nội dung thật; bước 4–11 dùng loại đầu tiên có sẵn khi chạy (curriculum từ F021 hoặc question từ F022). Trước khi có, chạy bằng `make test-integration MOD=learning` (fixture).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `make seed-check` sau `make seed` | `ref.tenants` có `th-demo-a`, `pgd-demo`, `common`; `ref.schools`, `ref.classes`, `ref.students` khớp seed identity | F020-AC-01, F020-AC-02 |
| 2 | Phát lại event `tenant.updated` cũ (source_version nhỏ hơn) bằng `nats pub` | `ref.tenants` không đổi; `processed_events` có bản ghi | F020-AC-01, F020-AC-03 |
| 3 | Phát `class.synced` có `tenant_id` sai so với `ref.schools` | Event vào `DLQ_LEARNING`, metric `eduworld_consumer_rejected_total` tăng | F020-AC-04 |
| 4 | `ad.truong.a` tạo nội dung tenant; `ad.truong.b` GET đúng ID đó | `404 NOT_FOUND` | F020-AC-09 |
| 5 | `ad.truong.a` gọi danh sách với `?tenant_ids=<pgd-demo>` | Chỉ dữ liệu `th-demo-a`; gửi `X-Tenant-Id: <pgd-demo>` → `403 TENANT_MISMATCH` | F020-AC-11 |
| 6 | `sa.admin` tạo nội dung `owner_scope=tenant` không có tenant đích | `422 VALIDATION_FAILED` | F020-AC-12 |
| 7 | `sa.admin` gán nội dung platform cho `th-demo-a` (`POST /api/v1/admin/tenant-assignments`) | `ad.truong.a` thấy (cờ `assigned=true`), `ad.truong.b` không thấy; audit `tenant_data.assigned` | F020-AC-15, F020-AC-20 |
| 8 | `ad.truong.a` PATCH nội dung được gán | `403 PERMISSION_DENIED` | F020-AC-18 |
| 9 | `sa.admin` gỡ gán | `ad.truong.a` không còn thấy ở lần đọc kế | F020-AC-17 |
| 10 | `ad.truong.a` xóa nội dung draft; `sa.admin` mở Thùng rác (`GET /api/v1/admin/trash?resource_type=…`) | Nội dung biến khỏi danh sách thường, có trong Thùng rác; audit `content.deleted` | F020-AC-21, F020-AC-23, F020-AC-24 |
| 11 | Tạo bản ghi mới trùng mã rồi khôi phục bản đã xóa | `409 CONFLICT`; xóa bản mới rồi khôi phục → thành công, audit `content.restored` | F020-AC-25, F020-AC-26 |
| 12 | `sa.admin` đọc danh sách với `tenant_ids` rỗng | Có cột tenant, audit `cross_tenant.accessed` | F020-AC-13 |
| 13 | `SELECT qual FROM pg_policies WHERE schemaname IN ('content','curriculum')`; xóa item bằng `learning_app`; đọc view `_live` bằng `learning_readonly` | Không policy nào chứa `deleted_at`; soft-delete thành công; item không còn trong `_live`; `learning_readonly` đọc bảng gốc bị từ chối | F020-AC-31 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make test-contract
make migrate-lint MOD=learning
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail) theo mã F020-AC-*, trace_id của bước 7 và 10, bản ghi audit tương ứng, output `make migrate-lint` (RLS + REVOKE).
