# Contracts: F011 — Đồng bộ SIS bằng CSV và provisioning tài khoản

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/identity.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createSisImport` | `POST /api/v1/schools/{school_id}/sis-imports` (`dry_run`) | identity.v1.yaml | `tenant.sis.import` (scope trường) | Có sẵn |
| `getSisImport` | `GET /api/v1/schools/{school_id}/sis-imports/{import_id}` | identity.v1.yaml | `tenant.sis.read` | Có sẵn |
| `listSisConnectionSchools` · `setSisConnectionSchools` | `GET/PUT /api/v1/admin/sis/connections/{connection_id}/schools` | identity.v1.yaml | `platform.sis_connection.manage` | Có sẵn |
| `listSisConnections` · `createSisConnection` | `GET/POST /api/v1/sis/connections` (F011 chỉ `provider_kind = csv_import`) | identity.v1.yaml | `platform.sis_connection.manage` (list: + `tenant.sis.read` scope) | Cần thêm |
| `listSisSyncRuns` | `GET /api/v1/sis/connections/{connection_id}/sync-runs` | identity.v1.yaml | `tenant.sis.read` / platform | Cần thêm |
| `getSisSyncRun` · `listSisSyncRunErrors` | `GET /api/v1/sis/runs/{run_id}` · `/errors` | identity.v1.yaml | `tenant.sis.read` | Cần thêm |
| `listSisReconciliationIssues` | `GET /api/v1/sis/reconciliation-issues` | identity.v1.yaml | `tenant.sis.read` | Cần thêm |
| `resolveSisReconciliationIssue` | `POST /api/v1/sis/reconciliation-issues/{issue_id}:resolve` | identity.v1.yaml | `tenant.sis.import` (scope trường) | Cần thêm |
| `createPlatformSisImport` | `POST /api/v1/sis/imports` (System Admin, `X-Tenant-Id` đích) | identity.v1.yaml | `platform.sis_connection.manage` | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có internal API mới. `triggerSisSync`, webhook `receiveSisWebhook`/`receiveSisChanges` và các operation provider của `specs/sis/eduworld-sis-contract.v1.yaml` (`getHealth`, `listSchools`, …, `lookupStudent`) thuộc F017 | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `class.synced` | identity → learning, game, competition, chat | `specs/events/payloads/identity/class.synced.v1.schema.json` | Có sẵn |
| `school.synced` | identity → mọi module (`ref.schools`) | `school.synced.v1.schema.json` | Cần thêm |
| `student.synced` | identity → mọi module (`ref.students`) | `student.synced.v1.schema.json` | Cần thêm |
| `student.provisioning_requested` | identity → identity worker | `student.provisioning_requested.v1.schema.json` | Cần thêm |
| `student.sis_link_created`, `student.sis_link_superseded` | identity → AUD, learning | `student.sis_link_created.v1.schema.json`, `student.sis_link_superseded.v1.schema.json` | Cần thêm |
| `user.status_changed`, `user.access_revoked` | identity → mọi module, realtime | `user.status_changed.v1.schema.json`, `user.access_revoked.v1.schema.json` | Cần thêm |
| `sis.sync_completed`, `sis.reconciliation_issue_raised` | identity → AUD, NTF | `sis.sync_completed.v1.schema.json`, `sis.reconciliation_issue_raised.v1.schema.json` | Cần thêm |
| Job `ew.job.identity.sis.<kind>` | identity (`core-api`/job lịch) → `cmd/worker` profile `worker-jobs` (work-queue `IDENTITY_SIS_JOBS`) | nội bộ module (struct Go) | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `components.schemas` của `specs/sis/eduworld-sis-contract.v1.yaml` (`School`, `Class`, `Student`, `Enrollment`) | Định nghĩa cột CSV và validator bản ghi | Có sẵn |

## 6. Delta cần thực hiện

- `identity.v1.yaml`: thêm `listSisConnections`, `createSisConnection`, `listSisSyncRuns`, `getSisSyncRun`, `listSisSyncRunErrors`, `listSisReconciliationIssues`, `resolveSisReconciliationIssue`, `createPlatformSisImport` theo [API_DESIGN §9.15](../../../docs/07-backend/API_DESIGN.md) (`x-internal: true`, mọi chuỗi có `maxLength`, không trả credential).
- Event payload JSON Schema: `school.synced`, `student.synced`, `student.provisioning_requested`, `student.sis_link_created`, `student.sis_link_superseded`, `user.status_changed`, `user.access_revoked`, `sis.sync_completed`, `sis.reconciliation_issue_raised`.
- Webhook `receiveSisWebhook` và `specs/postman/sis-conformance/` → F017.
