# Implementation Plan: F011 — Đồng bộ SIS bằng CSV và provisioning tài khoản

**Branch**: `011-sis-sync-provisioning` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Pipeline SIS của module identity chạy trong `src/backend/cmd/worker` (profile `worker-jobs` cho lịch/import, consumer ở `worker-events`): port `SISProvider` với hiện thực `csvImportReader` (hiện thực `contractV1Client` → F017) dùng DTO sinh từ contract; validator JSON Schema → `sis.staging_records` → `Reconciler` → apply qua command của F010 (school/class/student/enrollment) trong lô 500, outbox event. Job vào JetStream work-queue `IDENTITY_SIS_JOBS`. Consumer `keycloak-provisioner` tạo Keycloak user + ExternalIdentity + `sis_identity_links`. `LoginSync` (port application, tra cứu qua `SisLookupPort`) phục vụ F013. API: import CSV (public), connection `csv_import`/run/issue (gateway nội bộ). Pull `contract_v1`, webhook, conformance kit, `sis-mock` → F017.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | ent, Atlas, oapi-codegen (server; models từ `specs/sis` cho DTO), santhosh-tekuri/jsonschema hoặc tương đương cho validator, nats.go JetStream, AWS SDK v2 (S3 floci), failsafe-go (`src/backend/pkg/modcall`) |
| Storage | PostgreSQL `eduworld_identity` (`sis`, `identity`, `org`); object storage (CSV, mã hóa at-rest, `sis_staging_retention`); Valkey lock `ew:identity:sis:lock:<connection_id>` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey, Keycloak); fake `SisLookupPort` (chỉ test) |
| Target Platform | Linux container (module `identity` api + worker) |
| Project Type | backend module |
| Performance Goals | Import CSV 1.000 học sinh ≤ 10 phút (SC-001) [Proposed] ([SIS_INTEGRATION §11](../../docs/09-identity/SIS_INTEGRATION.md)) |
| Constraints | AGENTS R3, R4, R13, R14, R21, R28, R30; ADR-0021, ADR-0046, ADR-0061, ADR-0069 |
| Scale/Scope | `sis_login_sync_concurrency`, `csv_import_max` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta OpenAPI (connection, run, issue), event payload ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Toàn bộ trong module identity; module khác nhận `student.synced`/`enrollment.changed`/`class.synced`/`school.synced` |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng `sis.*` T có `tenant_id` + RLS; job platform dùng role `identity_platform` với `tenant_id` tường minh; staging ẩn danh hóa, không xóa |
| IV Server-authoritative | ✅ | Không có điểm/phần thưởng; import có `Idempotency-Key`, audit mọi run |
| V Xác thực, phân quyền | ✅ | `tenant.sis.import`, `tenant.sis.read`, `platform.sis_connection.manage`; connection `csv_import` không có credential |
| VI An toàn trẻ em | ✅ | Data minimization theo DTO; không log `full_name`, `date_of_birth`; `national_student_id` chỉ lưu HMAC |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số SIS theo tên CONFIG_DEFAULTS; enum theo contract |
| VIII UI tách logic, token | ✅ | Không có UI (F052) |
| IX Test có bằng chứng | ✅ | Test cho mọi F011-AC-*; fake `SisLookupPort` không dùng trong đường chạy thật (R17) |

## Project Structure

### Documentation (feature này)

```text
specs/011-sis-sync-provisioning/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/identity/
├── ent/schema/                     # sisconnection.go, sisconnectionschool.go, syncrun.go, stagingrecord.go,
│                                   # syncrecorderror.go, sisimport.go, sislink.go, syncedstudent.go,
│                                   # studentloginkey.go, reconciliationissue.go, externalidentity.go, sisidentitylink.go
├── internal/domain/sis/            # SyncRun, ReconciliationIssue, ConflictRule, mapping DTO → domain
├── internal/domain/identity/       # ExternalIdentity, SisIdentityLink, Identifier (chuẩn hóa)
├── internal/application/command/   # CreateSisImport, ApplyStagingBatch, ProvisionStudentAccount,
│                                   # ResolveReconciliationIssue, ConfirmSamePerson, CreateSisConnection, SetSisConnectionSchools
├── internal/application/query/     # GetSisImport, ListSisConnections, ListSyncRuns, ListReconciliationIssues
├── internal/application/port/      # SISProvider, SisLookupPort, ContractValidator, ObjectStorage, KeycloakAdmin (F010), LoginSync
├── internal/application/loginsync/ # LoginSync: nhánh A/B/C (dùng bởi F013)
├── internal/infrastructure/sis/dto/          # models sinh từ specs/sis (client contract_v1 → F017)
├── internal/infrastructure/sis/csvreader/    # csvImportReader
├── internal/infrastructure/sis/validator/    # JSON Schema trích từ spec
├── internal/presentation/http/public/sis_imports.go
├── internal/presentation/http/public/sis_connections.go
├── internal/presentation/consumer/{sis_jobs.go, keycloak_provisioner.go}
├── internal/presentation/job/staging_anonymizer.go
└── tests/{unit,integration,contract}/
```

**Structure Decision**: Theo BACKEND_DESIGN §3; port `SISProvider` theo [SIS_INTEGRATION §4.1](../../docs/09-identity/SIS_INTEGRATION.md).

## Phase 0: Research

Không có quyết định mới. Lựa chọn thư viện JSON Schema validator theo ADR-0025 khi code (không đổi kiến trúc).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `SyncRun` (trạng thái, stats), `ConflictRule` (matrix §6.3), `Identifier.Normalize` (trim, NFC, lower), `SisIdentityLink` (một active/user, một active/khóa nguồn) | Thuần, không I/O |
| application | Command/Query ở cây trên; `LoginSync.Ensure(ctx, userID) (Branch, error)` | Permission: [contracts](contracts/README.md) §1 |
| infrastructure | ent repo, CSV reader, validator, S3, Keycloak Admin (F010) | Client `contract_v1` → F017 |
| presentation | `createSisImport`, `getSisImport`, `setSisConnectionSchools`, `listSisConnectionSchools`, Cần thêm: `listSisConnections`, `createSisConnection`, `listSisSyncRuns`, `getSisSyncRun`, `listSisSyncRunErrors`, `listSisReconciliationIssues`, `resolveSisReconciliationIssue` · consumer `sis-jobs`, `keycloak-provisioner` · job `sis-staging-anonymizer` | `triggerSisSync`, `receiveSisWebhook`, job `sis-scheduler` → F017 |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F010 (domain tổ chức, `TransferStudent`, adapter Keycloak, tenant/school).
- Event phát: `school.synced`, `class.synced`, `student.synced`, `enrollment.changed`, `student.provisioning_requested`, `user.provisioned`, `user.activated` (F013), `student.sis_link_created`, `student.sis_link_superseded`, `user.status_changed`, `user.access_revoked`, `sis.sync_completed`, `sis.reconciliation_issue_raised`, `audit.recorded`.
- Event/job tiêu thụ: `student.provisioning_requested` (keycloak-provisioner), `ew.job.identity.sis.>` (`csv_import`, `login_refresh`; `scheduled_sync`, `webhook_batch` → F017).
- Contract ra ngoài: chỉ dùng `components.schemas` của `specs/sis/eduworld-sis-contract.v1.yaml` làm DTO; operation provider và webhook → F017.

### Bảo mật, tenant, idempotency, audit

- Import: `tenant.sis.import` scope trường; read: `tenant.sis.read`; connection: `platform.sis_connection.manage` (gateway nội bộ + MFA).
- Job platform chạy `TenantScope` theo `tenant_id` của connection; audit mỗi run và mỗi thay đổi (`actor = system:sis-sync:<connection_id>`, diff trường, không PII đầy đủ).
- `Idempotency-Key` cho `createSisImport`, `createSisConnection`, `resolveSisReconciliationIssue`.
- CSV: quét malware, mã hóa at-rest, chống CSV injection khi xuất lại.

### Quan sát

- Metric `eduworld_sis_sync_runs_total`, `eduworld_sis_sync_duration_seconds`, `eduworld_sis_records_total`, `eduworld_sis_record_errors_total`, `eduworld_sis_reconciliation_issues_open`, `eduworld_sis_last_success_timestamp`, `eduworld_sis_contract_unknown_fields_total`, `eduworld_sis_login_sync_total{branch,result}`, `eduworld_sis_login_sync_duration_seconds`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
