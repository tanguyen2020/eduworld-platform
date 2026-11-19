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

## 5. Bằng chứng (2026-11-19, local, T041)

Chạy trong worktree `f020-learning` (Windows, Docker Desktop, `GOMAXPROCS=2`), lệnh tương đương target `make` (máy không có `make`). Bước tay 1–13 ở §2 cần dữ liệu seed identity, loại nội dung thật (F021/F022) và route portal (F045) cùng composition root (Câu hỏi mở F020-3); trước đó chúng được kiểm bằng test tự động dưới đây (fixture chỉ trong `tests/`).

| Tiêu chí | Test (file) | Kết quả |
|---|---|---|
| F020-AC-01 | `TestProjectionTenantSourceVersion` (`tests/integration/projection_tenant_test.go`; lag < 5 s — SC-002) | PASS |
| F020-AC-02 | `TestProjectionEntities` (`projection_entities_test.go`) | PASS |
| F020-AC-03 | `TestProjectionRedeliveryProcessedOnce` (`projection_dedupe_test.go`) | PASS |
| F020-AC-04 | `TestProjectionTenantMismatchDeadLettered` (`projection_dedupe_test.go`, gồm edge case event đến trước `tenant.created`) | PASS |
| F020-AC-05 | `TestOnlyProjectionConsumerWritesRef` (`tests/unit/arch_ref_write_test.go`) | PASS |
| F020-AC-06 | `TestContentAndResourceTypes` (`domain/contentmodel/content_meta_test.go`), `TestApplyTenantAssignmentsInvalid` (`quest` → 422) | PASS |
| F020-AC-07 | `TestContentMetaValidate` | PASS |
| F020-AC-08 | `TestOwnerScopeTenantInvariant`, `TestOwnerCheckAndPublishedImmutability` (CHECK DB) | PASS |
| F020-AC-09 | `TestHybridReadScope` (`hybrid_scope_test.go`) | PASS |
| F020-AC-10 | `TestHybridMissingScopeAndRLS` | PASS |
| F020-AC-11 | `TestScopeOfContentLists` (`scope_principal_test.go`, chuỗi `pkg/httpx`), `TestForRead` | PASS |
| F020-AC-12 | `TestForWrite` (`application/scope`), `TestPlatformScopeWrites` | PASS |
| F020-AC-13 | `TestScopeOfContentLists`, `TestForRead` | PASS |
| F020-AC-14 | `TestStringsAreBounded` (`maxlen_test.go`), `TestRevokeTenantAssignments` (`reason` 1001 ký tự → 422) | PASS |
| F020-AC-15 | `TestApplyTenantAssignments` (`assignment_apply_test.go`) | PASS |
| F020-AC-16 | `TestApplyTenantAssignmentsInvalid` | PASS |
| F020-AC-17 | `TestRevokeTenantAssignments` (`assignment_revoke_test.go`) | PASS |
| F020-AC-18 | `TestAssignedContentReadOnly` (`assignment_readonly_test.go`), `TestSoftDeleteContent` | PASS |
| F020-AC-19 | `TestPlatformOpsOnlyForIdentity`, `TestPlatformOpsAuthorizePrincipal` (`tests/contract/platform_ops_auth_test.go`) — cưỡng chế theo operation lúc chạy (`CALLER_NOT_ALLOWED`); `arch-check` theo package (Câu hỏi mở F020-2) | PASS |
| F020-AC-20 | `TestApplyTenantAssignments`, `TestRevokeTenantAssignments` (audit actor = Principal gốc, cache Valkey) | PASS |
| F020-AC-21 | `TestSoftDeleteContent` (`soft_delete_test.go`) | PASS |
| F020-AC-22 | `TestHardDeleteImpossible` | PASS |
| F020-AC-23 | `TestSoftDeleteContent`, `TestTrashListAndRestore` | PASS |
| F020-AC-24 | `TestTrashListAndRestore` (`trash_test.go`) | PASS |
| F020-AC-25 | `TestTrashListAndRestore` | PASS |
| F020-AC-26 | `TestTrashListAndRestore` | PASS |
| F020-AC-27 | `TestCloneStep` (`clone_step_test.go`) | PASS |
| F020-AC-28 | `TestCloneStep` | PASS |
| F020-AC-29 | `TestCloneStep` | PASS |
| F020-AC-30 | `TestCloneStep` | PASS |
| F020-AC-31 | `TestRLSAndSoftDelete` (`rls_softdelete_test.go`) | PASS |
| F020-AC-32 | `TestPlatformScopeWrites` (`platform_scope_write_test.go`) | PASS |

Lệnh (trong `src/backend`):

```bash
GOMAXPROCS=2 go run ./tools/facadegen --check --oapi=oapi-codegen && GOMAXPROCS=2 go run ./tools/allowedcallers --check   # gen-check
gofmt -l modules/learning pkg; GOMAXPROCS=2 go vet -p 2 -tags integration ./modules/learning/...
golangci-lint run --allow-parallel-runners --concurrency 2 ./modules/learning/...            # 0 issues
golangci-lint run --allow-parallel-runners --concurrency 2 --config .golangci.arch.yml ./modules/learning/...   # 0 issues
GOMAXPROCS=2 go run ./tools/lint/ewlint ./modules/learning/... && go run ./tools/tenancyguard && go run ./tools/entlint && go run ./tools/archgen --check
GOMAXPROCS=2 go test -p 2 ./modules/learning/...                                               # unit + contract
TESTCONTAINERS_RYUK_DISABLED=true GOMAXPROCS=2 go test -p 1 -tags integration -timeout 30m ./modules/learning/tests/integration/
```

Kết quả lần chạy cuối (2026-11-19): integration `modules/learning/tests/integration` **19/19 PASS** (176,9 s, một PostgreSQL 17 dùng chung cho package, NATS/Valkey testcontainers); unit + contract `modules/learning/...` 6 package `ok` (contentmodel, resourceassignment, scope, ent golden, contract, unit); golangci-lint 0 issues (thường và `.golangci.arch.yml`); `ewlint`, `tenancyguard`, `entlint` 0 vi phạm; `archgen --check` 0 violations; `facadegen --check`, `allowedcallers --check` xanh; specs: `check-maxlength` 0 vi phạm, `check-enums` 0 lỗi, `check-features` 0 lỗi, `check-cross-features` 0 lỗi, `validate-schemas` 0 lỗi, `check-links` 0 hỏng.
