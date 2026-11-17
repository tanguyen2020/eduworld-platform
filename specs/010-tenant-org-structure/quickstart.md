# Quickstart: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F003, F004.
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `sa.admin` (System Admin, MFA tắt ở local), `ad.donvi` (Tenant Admin `pgd-demo`), `ad.truong.a`, `ad.truong.b` (School Admin), `hs.an.3a`, `hs.em.4a`, `hs.tudo`.
- Token portal lấy bằng `POST /api/v1/auth/portal/login` (mật khẩu + OTP, Keycloak local qua module identity — ADR-0091); gọi API qua APISIX nội bộ cho `/admin/*`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `sa.admin` gọi `createTenant` (`th-demo-x`, `school`) với `Idempotency-Key` | `201`; Keycloak có Organization alias `th-demo-x`, attribute `tenant_id`; audit `tenant.created` | F010-AC-01, F010-AC-05 |
| 2 | Gọi lại `createTenant` loại `common` | `409 CONFLICT` | F010-AC-02 |
| 3 | `ad.donvi` gọi `createTenant` | `403 PERMISSION_DENIED` (gateway nội bộ chặn từ public: 404) | F010-AC-04 |
| 4 | `updateTenant` `th-demo-x` → `disabled` | Organization `enabled=false`; resolve thành viên → `TENANT_ACCESS_DISABLED` | F010-AC-03 |
| 5 | `GET /catalog/provinces?country=VN` | 34 đơn vị `active` | F010-AC-06 |
| 6 | `importSchools` vào `pgd-demo` CSV có `vn.old.ha-giang` và một dòng thiếu tỉnh | Dòng 1 ánh xạ `vn.tuyen-quang` + cảnh báo; dòng 2 `pending_review` | F010-AC-08 |
| 7 | `ad.donvi` `updateTenantSchool` với tỉnh `inactive`; rồi `approveTenantSchool` với tỉnh `active` | `422`; sau đó `active`, audit | F010-AC-07, F010-AC-09 |
| 8 | `ad.truong.b` gọi `listClasses` của TH Demo A (tenant khác) và TH Demo C (cùng tenant) | `404` / `403 PERMISSION_DENIED` | F010-AC-17, F010-AC-18 |
| 9 | Chuyển `hs.an.3a` sang 3B; chuyển `hs.em.4a` sang THCS Demo B (cùng `pgd-demo`) | Một enrollment active; `enrollment.changed`; `hs.em.4a` giữ tenant và membership | F010-AC-11, F010-AC-13, F010-AC-14 |
| 10 | Thêm tay `hs.an.3a` vào Organization `pgd-demo`, chạy `org-membership-reconcile --once` | Member bị gỡ, issue `unexpected_org_member` | F010-AC-22 |
| 11 | `sa.admin` gọi `createCommonTenantStudents` khối 4 | User + enrollment `common-g4`, member Organization `common` | F010-AC-24 |
| 11b | `sa.admin` gọi `createCommonTenantStudents` lô 3 dòng (`hs.tudo1` khối 4, `hs.tudo2` khối 7, dòng 3 trùng `hs.an.3a`), rồi gọi lại cùng `Idempotency-Key` | `200`: 2 `created` có `initial_password` (khối 7 phải đổi mật khẩu lần đầu), 1 `failed` `USERNAME_TAKEN`; lần gọi lại: cùng kết quả, không có `initial_password`, `password_redacted = true` | F010-AC-28, F010-AC-29 |
| 12 | `hs.an.3a` gọi `getMyStudentProfile` | Trường/lớp/tỉnh song ngữ của chính mình | F010-AC-19 |
| 13 | Facade test: module learning gọi `listProjectionSnapshot` `entity_type=class`, `tenant_id` của `pgd-demo`, `limit=2` rồi theo `next_cursor` | Đủ lớp của tenant, có `source_version`; không lớp của tenant khác | F010-AC-27 |

## 3. Lệnh test tự động

```bash
make test MOD=identity
make test-integration MOD=identity
make migrate-lint MOD=identity
node scripts/specs/validate-reference-data.mjs
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), trace_id của kịch bản 1 và 9, ảnh Organization trong Keycloak admin console (local), log reconciliation.
