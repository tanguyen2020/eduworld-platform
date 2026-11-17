# Quickstart: F002 — Platform kit lõi: config, observability, problem, httpx, authz, tenancy

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F001
- Lệnh: `make up`, `make up-obs`, `make test MOD=pkg`, `make test-integration MOD=pkg` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md))
- Service mẫu kiểm end-to-end: `modules/learning` gắn `httpx` với route thử `GET /api/v1/_kit/echo` (chỉ build tag `kitdemo`, không vào image phát hành) và `PrincipalResolver` giả (fixture tenant A = `th-demo-a`, B = `pgd-demo`).
- Tài khoản demo (token phát bởi JWKS giả trong test; với Keycloak thật cần F004): `ad.truong.a` (tenant A), `ad.truong.b` (tenant B), `qlcm.truong.a` (2 tenant), `sa.admin` (platform), `hs.an.3a` (học sinh).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Gọi `/api/v1/_kit/echo` với token hợp lệ của `ad.truong.a` + `X-Tenant-Id` = A | 200, body có `tenant_id = A` | F002-AC-01, F002-AC-07 |
| 2 | Token sai `aud`, chữ ký giả, `alg=none`, không token (gọi thẳng `:8102`) | 401 `UNAUTHENTICATED` | F002-AC-02 |
| 3 | Token hết hạn | 401 `TOKEN_EXPIRED` | F002-AC-03 |
| 4 | Xoay khóa JWKS (kid mới) rồi gọi lại | 200; log `jwks_refresh` một lần | F002-AC-04 |
| 5 | Gọi `/api/v1/_kit/echo` bằng token client credentials `svc-identity`; gọi `/internal/v1/_kit/echo` bằng token người dùng | 401 `UNAUTHENTICATED`; 404 | F002-AC-05 |
| 6 | Bỏ `X-Tenant-Id`; header `abc` | 400 `TENANT_HEADER_REQUIRED`; 400 `BAD_REQUEST` | F002-AC-06 |
| 7 | `ad.truong.b` gửi `X-Tenant-Id` = A | 403 `TENANT_MISMATCH`, audit `authz.denied`, metric tăng | F002-AC-07 |
| 8 | `ad.truong.a` gửi `?tenant_ids=B` | Bị bỏ qua, chỉ dữ liệu A | F002-AC-08 |
| 9 | Resolver trả timeout, cache trống | 503 `UNAVAILABLE` `retryable: true` | F002-AC-09 |
| 10 | Gọi 2 lần liên tiếp rồi `Invalidate` | Lần 2 hit cache; sau invalidate gọi resolver | F002-AC-10 |
| 11 | `sa.admin` (MFA) dùng `CrossTenantScope` với lý do rỗng / có lý do | Lỗi / thành công + audit `cross_tenant.accessed` | F002-AC-11, F002-AC-12 |
| 12 | Chạy test RLS (testcontainers) | 0 dòng khi không set hoặc set tenant khác | F002-AC-13 |
| 13 | Sửa catalog thêm `game.quest.write` vào role `school_admin`, chạy binary | Exit 1, log tên role/permission | F002-AC-14, F002-AC-15 |
| 14 | `hs.an.3a` gọi `/api/v1/cms/_kit/echo`; `ad.truong.a` gọi route game thử | 403 `PORTAL_ACCESS_STAFF_ONLY`; 403 `GAME_ACCESS_STUDENT_ONLY` | F002-AC-16, F002-AC-17 |
| 15 | `sa.admin` không MFA gọi route platform thử; `ad.truong.a` có MFA | 403 `MFA_REQUIRED`; 403 `PERMISSION_DENIED` | F002-AC-18 |
| 16 | `qlcm.truong.a` (scope Toán) gọi tài nguyên môn khác; tài nguyên tenant B | 403 `PERMISSION_DENIED`; 404 | F002-AC-19 |
| 17 | Panic giả trong handler; lỗi lạ | 500 `INTERNAL`, có `trace_id`, không stack trace | F002-AC-20 |
| 18 | Thêm mã vào `ProblemCode` không chạy gen → `make gen-check` | Fail | F002-AC-21 |
| 19 | Bỏ `EW_OIDC_ISSUERS` rồi chạy binary | Exit 1, log tên biến | F002-AC-22 |
| 20 | Gửi body chứa `password` mẫu; quét log | 0 lần xuất hiện | F002-AC-23 |
| 21 | Mở Tempo bằng `trace_id` của bước 17; xem `/metrics` | Thấy span; metric có nhãn `binary`, `module` | F002-AC-24 |
| 22 | Gửi chuỗi dài hơn `maxLength` | 422 `VALIDATION_FAILED`, `MAX_LENGTH` | F002-AC-25 |
| 23 | Thêm import `entgo.io/ent` vào `internal/domain` mẫu | `make lint` fail | F002-AC-26, F002-AC-27 |
| 24 | Thêm operation mới vào spec không có test | `routematrix check` fail | F002-AC-28 |
| 25 | Chạy test soft-delete dưới RLS (testcontainers): `UPDATE ... SET deleted_at` bằng role `_app`, đọc lại qua `deleted_at IS NULL` và view `_live` | Update thành công; dòng không còn; `_readonly` đọc bảng gốc bị từ chối | F002-AC-29 |
| 26 | Thêm `tenancy.Bypass(ctx, "debug")` vào fixture | `make lint` fail (analyzer `bypassreason`) | F002-AC-30 |

## 3. Lệnh test tự động

```bash
make test MOD=pkg
make test-integration MOD=pkg          # RLS (postgres:17), cache (valkey:8)
make lint MOD=learning
go run ./tools/routematrix check --spec specs/openapi --module learning
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo gói, `trace_id` của kịch bản 17 trong Tempo, kết quả quét log (bước 20), output `routematrix check`.
