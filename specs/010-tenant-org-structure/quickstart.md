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

## 5. Bằng chứng (2026-11-19, Windows 11 + Docker Desktop 27.1; postgres:17 testcontainers với mọi migration Atlas)

Kịch bản §2 được tự động hóa bằng test (gói `src/backend/modules/identity/...`); kịch bản thủ công trên stack chờ phần nối binary (OQ F010-2 trong spec).

| Kịch bản / tiêu chí | Test | Kết quả |
|---|---|---|
| 1, 2, 4 — AC-01, AC-02, AC-03 (Organization), AC-05 | `TestTenantLifecycle`, `command/tenant_test.go`, `command/org_sync_test.go`, `keycloak/admin_test.go` | pass |
| 3 — AC-04, AC-05 qua HTTP (403 thiếu MFA/role tenant, 428 thiếu key, replay, 422 `maxLength`, 428/412 `If-Match`) | `TestIdentityHTTP` (chuỗi `httpx` thật + `httpxtest`) | pass |
| 5 — AC-06 | `TestOrgSchema/CatalogSeed` | pass |
| 6 — AC-08 | `TestImportSchools` (tỉnh cũ → kế thừa + `province_outdated`, thiếu tỉnh → `pending_review`, mã tỉnh sai, trùng mã, dry-run) | pass |
| 7 — AC-07, AC-09 | `TestSchoolLifecycle`, `command/school_test.go` | pass |
| 8 — AC-10, AC-17, AC-18, AC-19 | `TestOrgQueries` | pass |
| 9 — AC-11…AC-15 | `TestTransferStudent`, `TestOrgSchema/EnrollmentInvariants` | pass |
| AC-16 | `TestOrgSchema/UserChecks` | pass |
| AC-20 | `TestMembershipSync` | pass |
| 10 — AC-22 | `TestReconcileOrganizations` | pass |
| AC-23 | `keycloak/admin_test.go` (`TestRetries`), consumer trả lỗi → Nak/DLQ (`pkg/outbox`) | pass |
| 11, 11b — AC-24, AC-25, AC-28, AC-29 | `TestCreateCommonStudents`, `TestIdentityHTTP` (`no-store`, replay `password_redacted`) | pass |
| AC-26 | `tests/contract/no_self_registration_test.go` | pass |
| 13 — AC-27 | `TestListProjectionSnapshot` | pass |
| Catalog write (ADMIN_PORTAL §3.16) | `TestCatalogWrites` | pass |
| Bảng route ↔ spec | `tests/contract/routes_test.go` (24 operation) | pass |

Chưa đủ: AC-03 phần resolve thành viên → `TENANT_ACCESS_DISABLED` và AC-21 (realm role nhân sự theo UserRole) cần resolver/`user_roles` của F012; `importSchools` bất đồng bộ (OQ F010-1, F011); kịch bản 4/10/12 trên stack thật (OQ F010-2).

Lệnh: `go test ./modules/identity/...` (unit + contract) và `TESTCONTAINERS_RYUK_DISABLED=true go test -tags integration -timeout 45m ./modules/identity/...` — 11/11 + 2 test tích hợp mới pass; gate local (vet, golangci-lint, ewlint, archgen, entlint, tenancyguard, gofmt, unit) `CHECK OK`.
