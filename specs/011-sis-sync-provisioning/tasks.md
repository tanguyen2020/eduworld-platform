# Tasks: F011 — Đồng bộ SIS bằng CSV và provisioning tài khoản

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Sinh kiểu DTO (chỉ models, không client) từ `components.schemas` của contract SIS cho CSV reader + validator, thêm vào `make gen` — file: `src/backend/modules/identity/internal/infrastructure/sis/dto/gen.go`
- [ ] T002 [P] Tạo stream work-queue `IDENTITY_SIS_JOBS` (`ew.job.identity.sis.>`) idempotent khi worker khởi động — file: `src/backend/modules/identity/internal/infrastructure/messaging/streams.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Thêm operation `listSisConnections`, `createSisConnection`, `listSisSyncRuns`, `getSisSyncRun`, `listSisSyncRunErrors`, `listSisReconciliationIssues`, `resolveSisReconciliationIssue`, `createPlatformSisImport` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=identity` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); `SyncRun` thống nhất với data-model theo delta F017 T005
- [ ] T004 [P] Port `SisLookupPort` (application) + fake đếm lời gọi chỉ dùng trong test (gắn nhãn mock, R17); adapter `contract_v1` thuộc F017 — file: `src/backend/modules/identity/internal/application/port/sis_lookup.go`
- [x] T005 [P] Thêm payload schema `school.synced`, `student.synced`, `student.provisioning_requested`, `student.sis_link_created`, `student.sis_link_superseded`, `user.status_changed`, `user.access_revoked`, `sis.sync_completed`, `sis.reconciliation_issue_raised` — file: `specs/events/payloads/identity/` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [ ] T006 ent schema bảng `sis.*`, `identity.external_identities`, `identity.sis_identity_links` theo data-model (bỏ `school_ids`, thêm cột issue/run/error) — file: `src/backend/modules/identity/ent/schema/` — kiểm: `make migrate-diff MOD=identity name=f011_sis`, `make migrate-lint MOD=identity`
- [ ] T007 Migration partition tháng cho `staging_records`, `sync_record_errors`; RLS bảng T — file: `src/backend/modules/identity/migrations/`
- [ ] T008 [P] Validator JSON Schema trích từ `components.schemas` của contract (bỏ qua trường lạ, đếm metric) — file: `src/backend/modules/identity/internal/infrastructure/sis/validator/validator.go`
- [ ] T009 [P] Domain `Identifier.Normalize` (trim, NFC, lower) + test unit — file: `src/backend/modules/identity/internal/domain/identity/identifier.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — School Admin nhập CSV theo contract (Priority: P1) 🎯 MVP

**Goal**: Import CSV dry-run/áp dụng tạo dữ liệu tổ chức.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Test F011-AC-01, F011-AC-02, F011-AC-03: dry-run không ghi, dòng vi phạm `SIS_CONTRACT_VIOLATION` + all-or-nothing, cột lạ bị bỏ qua — file: `src/backend/modules/identity/tests/integration/sis_csv_validate_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Test F011-AC-04, F011-AC-05: thứ tự apply, lô 500 idempotent, event outbox, import lại 0 update — file: `src/backend/modules/identity/tests/integration/sis_apply_test.go`
- [ ] T012 [P] [US1] Test authorization + tenant isolation F011-AC-06, F011-AC-07: thiếu `tenant.sis.import`, trường khác, tenant khác → 403/404; file vượt `csv_import_max` → 422; thiếu `Idempotency-Key` → 428; audit diff không PII — file: `src/backend/modules/identity/tests/integration/sis_import_authz_test.go`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] `csvImportReader` (UTF-8/BOM, header bắt buộc, tách `login_*`) hiện thực port `SISProvider` (hiện thực `contract_v1` → F017) — file: `src/backend/modules/identity/internal/infrastructure/sis/csvreader/reader.go`
- [ ] T014 [US1] Command `CreateSisImport` (presigned upload S3, quét malware, xếp job `csv_import`), `ApplyStagingBatch`; query `GetSisImport` — file: `src/backend/modules/identity/internal/application/command/sis_import.go`
- [ ] T015 [US1] Orchestrator SyncRun: validate → staging → reconcile → apply (gọi command F010) — file: `src/backend/modules/identity/internal/application/command/sis_run.go`
- [ ] T016 [US1] Handler `createSisImport`, `getSisImport`; consumer `sis-jobs` — file: `src/backend/modules/identity/internal/presentation/http/public/sis_imports.go`, `src/backend/modules/identity/internal/presentation/consumer/sis_jobs.go`

## Phase 4: User Story 2 — Provisioning tài khoản học sinh và định danh duy nhất (Priority: P1)

**Goal**: Học sinh mới có tài khoản đăng nhập được.

**Independent Test**: Như spec.md US2.

### Tests cho User Story 2

- [ ] T017 [P] [US2] Test F011-AC-08, F011-AC-13: provisioning tạo Keycloak user + `external_identities` + `sis_identity_links`; hai SIS cùng `student_external_id` → hai link (Keycloak testcontainer) — file: `src/backend/modules/identity/tests/integration/provisioning_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Test F011-AC-09, F011-AC-10, F011-AC-11: 409 Keycloak → link theo `eduworld_user_id`; trùng người khác → `duplicate_identifier`; cùng người → thêm membership — file: `src/backend/modules/identity/tests/integration/identifier_uniqueness_test.go`
- [ ] T019 [P] [US2] Test F011-AC-12: `issuer` ngoài allowlist bị từ chối; khóa chỉ `(issuer, subject)` — file: `src/backend/modules/identity/tests/unit/external_identity_test.go`

### Implementation cho User Story 2

- [ ] T020 [P] [US2] Domain `ExternalIdentity`, `SisIdentityLink` (invariant một active) — file: `src/backend/modules/identity/internal/domain/identity/`
- [ ] T021 [US2] Command `ProvisionStudentAccount`, `ConfirmSamePerson` — file: `src/backend/modules/identity/internal/application/command/provisioning.go`
- [ ] T022 [US2] Consumer `keycloak-provisioner` (`student.provisioning_requested`) — file: `src/backend/modules/identity/internal/presentation/consumer/keycloak_provisioner.go`

## Phase 5: User Story 3 — Xử lý reconciliation issue (Priority: P1)

**Goal**: Xung đột có người duyệt.

**Independent Test**: Như spec.md US3.

### Tests cho User Story 3

- [ ] T023 [P] [US3] Test F011-AC-14, F011-AC-15, F011-AC-16: mỗi loại trong conflict matrix tạo đúng issue; `missing_class`; chuyển SIS cùng tenant `unit` → link `superseded` — file: `src/backend/modules/identity/tests/unit/conflict_rules_test.go` — phải FAIL trước khi code
- [ ] T024 [P] [US3] Test F011-AC-17 + tenant isolation/authorization cho `listSisReconciliationIssues`, `resolveSisReconciliationIssue` — file: `src/backend/modules/identity/tests/integration/reconciliation_api_test.go`

### Implementation cho User Story 3

- [ ] T025 [P] [US3] Domain `ConflictRule`, `ReconciliationIssue` — file: `src/backend/modules/identity/internal/domain/sis/reconciliation.go`
- [ ] T026 [US3] Command `ResolveReconciliationIssue`; query `ListReconciliationIssues`; handler — file: `src/backend/modules/identity/internal/application/command/reconciliation.go`, `src/backend/modules/identity/internal/presentation/http/public/sis_issues.go`

## Phase 6: User Story 4 — Đồng bộ khi đăng nhập (login sync) (Priority: P1)

**Goal**: `LoginSync` cho F013.

**Independent Test**: Như spec.md US4.

### Tests cho User Story 4

- [ ] T027 [P] [US4] Test F011-AC-18, F011-AC-19, F011-AC-20, F011-AC-21 với fake `SisLookupPort` đếm lời gọi; connection `csv_import` không gọi lookup — file: `src/backend/modules/identity/tests/integration/login_sync_test.go` — phải FAIL trước khi code
- [ ] T028 [P] [US4] Test tải F011-AC-22 (200 lookup, giới hạn concurrency, circuit breaker, metric `throttled`) — file: `src/backend/modules/identity/tests/integration/login_sync_load_test.go`

### Implementation cho User Story 4

- [ ] T029 [US4] `LoginSync.Ensure` nhánh A/B/C qua `SisLookupPort` (bỏ qua khi connection không có `student_lookup`), bulkhead theo connection, job `login_refresh` có `Nats-Msg-Id` — file: `src/backend/modules/identity/internal/application/loginsync/loginsync.go`

## Phase 7: User Story 5 — Trạng thái tài khoản từ SIS (Priority: P2)

### Tests cho User Story 5

- [ ] T030 [P] [US5] Test F011-AC-23, F011-AC-24, F011-AC-25: ánh xạ status, `missing_in_source` hai lần, không khôi phục học sinh đã dọn — file: `src/backend/modules/identity/tests/integration/sis_status_test.go` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T031 [US5] Áp trạng thái trong `ApplyStagingBatch` (outbox `user.status_changed` + `user.access_revoked` cùng transaction) — file: `src/backend/modules/identity/internal/application/command/sis_apply_status.go`

## Phase 8: User Story 6 — Connection `csv_import` và ranh giới truy cập SIS (Priority: P2)

### Tests cho User Story 6

- [ ] T032 [P] [US6] Test tenant isolation + authorization F011-AC-26: `createSisConnection` (`csv_import`), `setSisConnectionSchools`, `listSisConnections` — thiếu `platform.sis_connection.manage` → 403, tenant khác → 404, thiếu `Idempotency-Key` → 428, trường đã gán → `409 SIS_SCHOOL_ALREADY_CONNECTED`, audit — file: `src/backend/modules/identity/tests/integration/sis_connection_authz_test.go` — phải FAIL trước khi code
- [ ] T033 [P] [US6] Test F011-AC-27: response không chứa `secret_ref`/credential; route `/api/v1/sis/*` không có trên gateway public (route test APISIX của F004) — file: `src/backend/modules/identity/tests/integration/sis_boundary_test.go`

### Implementation cho User Story 6

- [ ] T034 [US6] Command `CreateSisConnection` (chỉ `csv_import` trong F011), `SetSisConnectionSchools`; query `ListSisConnections`; handler — file: `src/backend/modules/identity/internal/application/command/sis_connection.go`, `src/backend/modules/identity/internal/presentation/http/public/sis_connections.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T035 [P] Cập nhật DATABASE_DESIGN §2.3 (bỏ `sis_connections.school_ids`, cột mới của `reconciliation_issues`, `sync_runs.tenant_id`, `sync_record_errors.column_name`; thống nhất retention staging theo `sis_staging_retention`) và CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T036 [P] Job `sis-staging-anonymizer` ẩn danh hóa payload staging quá `sis_staging_retention` (không DELETE) — file: `src/backend/modules/identity/internal/presentation/job/staging_anonymizer.go`
- [ ] T037 [P] Metric và alert `eduworld_sis_last_success_timestamp` theo SIS_INTEGRATION §8 — file: `src/backend/modules/identity/internal/infrastructure/sis/metrics.go`
- [ ] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US6 (connection `csv_import` cần trước import thật) → US1 → US2 (cần provisioning sau apply) → US3, US4, US5 song song → Polish.
- F017 (SIS `contract_v1`) dùng lại port `SISProvider`, `SisLookupPort`, validator và pipeline staging → apply của F011.
- US4 (`LoginSync`) phải xong trước khi F013 tích hợp.

## Definition of Done

- [ ] Mọi tiêu chí F011-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `identity`
- [ ] Constitution Check trong plan.md vẫn đạt; fake `SisLookupPort` chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
