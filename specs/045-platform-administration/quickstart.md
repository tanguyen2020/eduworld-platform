# Quickstart: F045 — Platform Administration backend: saga gán/clone, thùng rác mọi tenant, đọc liên tenant

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F010, F012, F015, F020 (module learning có platform ops và `infra.tenant_resource_assignments`).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=jobs` (saga), `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); gọi route `/admin/*` qua gateway nội bộ của compose.
- Tài khoản demo: `sa.admin` (system_admin, MFA tắt ở local), `cm.noidung` (content_admin), `sm.antoan` (safety_moderator), `ad.donvi` (tenant_admin `pgd-demo`), `ad.truong.a` (school_admin `th-demo-a`), `ad.truong.b` (school_admin `pgd-demo`).
- Dữ liệu: một question set platform đã publish (`cm.noidung` tạo), một câu hỏi tenant `th-demo-a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `sa.admin` gọi `listTenants?q=DEMO` | Trả `th-demo-a`, `pgd-demo` (không phân biệt hoa thường), có `tenant_type`; `q` 101 ký tự → `422` | F045-AC-05, F045-AC-26 |
| 2 | `sa.admin` gọi `listTrash?resource_type=question` (không `X-Tenant-Id`, không `tenant_ids`) | Dữ liệu mọi tenant, mỗi dòng có `tenant_id`; audit `cross_tenant.accessed`; metric bypass tăng | F045-AC-01, F045-AC-06, F045-AC-19 |
| 3 | `ad.donvi` gọi `listTenantTrash?resource_type=question&tenant_ids=<th-demo-a>` | Chỉ dữ liệu `pgd-demo` (tham số bị bỏ qua) | F045-AC-02, F045-AC-22 |
| 4 | `cm.noidung`, `sm.antoan` gọi `listTrash`; `ad.truong.a` gọi `/admin/trash` qua gateway public; token `sa.admin` thiếu MFA (bật MFA trong test) | `403 PERMISSION_DENIED`; không có route; `403 MFA_REQUIRED` + audit `authz.denied` | F045-AC-03, F045-AC-04 |
| 5 | `sa.admin` `assignToTenants` question set platform → `th-demo-a`; gửi lại cùng key; cùng key khác payload | `ad.truong.a` thấy chỉ đọc, `ad.truong.b` không thấy; replay trả kết quả cũ; `409 IDEMPOTENCY_KEY_REUSED` | F045-AC-07, F045-AC-09, F045-AC-12 |
| 6 | Gán câu hỏi của tenant `th-demo-a` (không phải platform); gán id không tồn tại | `422 VALIDATION_FAILED`; `404 NOT_FOUND` | F045-AC-08 |
| 7 | `ad.truong.a` sửa question set được gán; `ad.truong.b` tạo bài giao tham chiếu nó | `403`; `403 RESOURCE_NOT_ASSIGNED` | F045-AC-09 |
| 8 | `sa.admin` `unassignFromTenants` | `th-demo-a` không còn thấy; attempt cũ vẫn xem được; event `tenant_data.unassigned` | F045-AC-10 |
| 9 | `assignToTenants` với `apply_to_new_tenants=true`, rồi tạo tenant mới | Tenant mới có assignment | F045-AC-11 |
| 10 | `sa.admin` `createTenantClone` question set `th-demo-a` → `pgd-demo`; poll `getTenantClone` | `202`; `succeeded`; bản sao `draft`, `cloned_from_*`, ID mới; không có attempt/học sinh mới ở `pgd-demo`; audit ở cả 2 tenant | F045-AC-13, F045-AC-14, F045-AC-15, F045-AC-18 |
| 11 | Clone với nguồn = đích; `resource_type` không cho phép | `422 VALIDATION_FAILED`, không tạo job | F045-AC-15, F045-AC-16 |
| 12 | Cho facade `learning` trả lỗi tạm thời (fault injection) giữa job clone, rồi bỏ lỗi | Bước retry, job hội tụ, số bản sao không đổi khi chạy lại | F045-AC-17 |
| 13 | Xóa câu hỏi ở `th-demo-a` → `restoreTrashItem` có `reason`; tạo câu cùng mã rồi khôi phục bản cũ | Khôi phục, xuất hiện lại; `409 CONFLICT` | F045-AC-20, F045-AC-21 |
| 14 | Tìm route xóa vĩnh viễn; đọc danh sách câu hỏi thường | Không có; bản ghi đã xóa không xuất hiện | F045-AC-23 |
| 15 | Tắt module learning rồi gọi `listTrash?resource_type=question` | `503 UNAVAILABLE` | F045-AC-24, F045-AC-25 |
| 16 | Seed `hs.trung.a1`, `hs.trung.a2` (cùng `th-demo-a`) + issue `duplicate_identifier`; `ad.truong.a` đề nghị gộp; `sa.admin` gọi `mergeUsers` (from a2, into a1) | `202` + `merge_id`; `getUserMerge` → `succeeded`; `identity.user_merges` một dòng; attempt/XP của a2 thấy ở a1; a2 trong Thùng rác `restorable = false`; audit `user.merged` | F045-AC-27, F045-AC-28, F045-AC-32, F045-AC-33 |
| 17 | `ad.truong.a` gọi trực tiếp `mergeUsers`; `sa.admin` gộp a1 vào a1, gộp lại a2, `reason` 2 ký tự; user ở `pgd-demo` | `403`; `409 CONFLICT`; `409 CONFLICT`; `422`; `404` | F045-AC-30, F045-AC-31 |
| 18 | Giữ game session active cho a2 rồi gộp | module game `blocked`; saga `failed`, alert `SagaStuck`; đóng session, chạy lại → `succeeded` | F045-AC-29 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=identity
make lint MOD=identity
make test MOD=identity
make test-integration MOD=identity
make test-contract
make migrate-lint MOD=identity
```

## 4. Bằng chứng cần nộp

- Số test pass/fail (`platformadmin`), kết quả facade test identity ↔ learning, trace_id kịch bản 10 và 12, bản ghi audit truy vấn được cho kịch bản 2, 5, 10, 13.
