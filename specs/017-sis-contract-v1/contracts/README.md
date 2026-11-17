# Contracts: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/identity.v1.yaml` (gộp vào `specs/openapi/eduworld-api.v1.yaml`). Mọi route `x-internal: true` (APISIX internal + MFA với role platform).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createSisConnection` | `POST /api/v1/sis/connections` | identity.v1.yaml | `platform.sis_connection.manage` | Có sẵn (F011, chỉ `csv_import`, áp 2026-11-17) → F017 mở rộng `SisConnectionCreate` cho `contract_v1` (Cần sửa) |
| `listSisConnections` | `GET /api/v1/sis/connections` | identity.v1.yaml | `platform.sis_connection.manage` / `tenant.sis.read` (scope) | Có sẵn (F011, áp 2026-11-17) → F017 thêm trường `capabilities`, `last_health_at`, `last_success_at`, `status_reason`, `has_secret`, `has_webhook_secret` vào `SisConnection` (Cần sửa) |
| `updateSisConnection` | `PATCH /api/v1/sis/connections/{connection_id}` | identity.v1.yaml | `platform.sis_connection.manage` | Cần thêm |
| `checkSisConnectionHealth` | `POST /api/v1/sis/connections/{connection_id}:health-check` | identity.v1.yaml | `platform.sis_connection.manage` | Cần thêm |
| `triggerSisSync` | `POST /api/v1/sis/connections/{connection_id}/sync-runs` | identity.v1.yaml | `platform.sis_connection.manage`; `tenant.sis.import` (scope trường, `school_id` bắt buộc) | Có sẵn → Cần sửa |
| `listSisSyncRuns` · `getSisSyncRun` · `listSisSyncRunErrors` | `GET /api/v1/sis/…` | identity.v1.yaml | `tenant.sis.read` / platform | Có sẵn (F011, áp 2026-11-17) — F017 chỉ dùng |
| `setSisConnectionSchools` | `PUT /api/v1/admin/sis/connections/{connection_id}/schools` | identity.v1.yaml | `platform.sis_connection.manage` | Có sẵn (F011) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `receiveSisChanges` (webhook `sisChanges`) | SIS của trường → module identity `POST /internal/sis/webhooks/{connection_id}` (HMAC, APISIX internal/partner + IP allowlist) | `specs/sis/eduworld-sis-contract.v1.yaml` | Không áp dụng (hệ thống ngoài, không token `svc-*`) | Có sẵn → Cần sửa |
| `getHealth`, `listSchools`, `listAcademicYears`, `listClasses`, `listStudents`, `listEnrollments`, `listChanges`, `lookupStudent` | module identity (`src/backend/cmd/worker`, profile `worker-jobs`) → facade SIS của trường (outbound) | `specs/sis/eduworld-sis-contract.v1.yaml` | Không áp dụng | Có sẵn (thêm `examples`) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `sis.sync_completed` | identity → AUD, NTF | `specs/events/payloads/identity/sis.sync_completed.v1.schema.json` | Có sẵn (owner F011, áp 2026-11-17) → F017 thêm trường tương thích (Cần sửa, T007) |
| Job `ew.job.identity.sis.scheduled_sync`, `ew.job.identity.sis.webhook_batch` | identity (job lịch/webhook trong `core-api`) → `src/backend/cmd/worker` profile `worker-jobs` (`IDENTITY_SIS_JOBS`) | nội bộ module (struct Go) | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `components.schemas` của `specs/sis/eduworld-sis-contract.v1.yaml` (`Health`, `School`, `AcademicYear`, `Class`, `Student`, `Enrollment`, `Change`, `ChangeBatch`, `*Page`) | Kiểu client sinh + validator response/webhook | Có sẵn |

## 6. Delta cần thực hiện

Phase 2 task T004–T009.

- **Cần sửa** `createSisConnection` (owner F011, F017 mở rộng): `SisConnectionCreate` dạng `oneOf` theo `provider_kind`; nhánh `contract_v1`: `contract_version` (maxLength 64, pattern `^1\.[0-9]+\.[0-9]+$`), `config` {`base_url` format uri maxLength 2048 pattern `^https://`, `auth_type` enum `oauth2_client_credentials`/`api_key`/`mtls`, `token_url` maxLength 2048, `oauth_scope` maxLength 255, `schools[]` (`ExternalId` maxLength 64, maxItems 500)}, `secret_ref` maxLength 512 pattern, `webhook_secret_ref` maxLength 512, `schedule` maxLength 64; response `SisConnection` không có `secret_ref`/`webhook_secret_ref`, có `has_secret`, `has_webhook_secret`, `capabilities`, `status_reason`, `last_health_at`, `last_success_at`.
- **Cần thêm** `updateSisConnection` `PATCH /sis/connections/{connection_id}` (JSON Merge Patch: `config`, `secret_ref`, `webhook_secret_ref`, `schedule`, `status` enum `active`/`paused`, `reason` maxLength 500 bắt buộc khi đổi `status`; `If-Match`), 200 `SisConnection`, 403, 404, 409 `SIS_CONNECTION_NOT_ACTIVE`/`VERSION_MISMATCH`, 422.
- **Cần thêm** `checkSisConnectionHealth` `POST /sis/connections/{connection_id}:health-check` → 200 `SisConnectionHealth` {`status` enum `ok`/`degraded`/`unreachable`, `contract_version` maxLength 64, `compatible` boolean, `capabilities[]`, `checked_at`}, 422 `SIS_CONTRACT_VERSION_UNSUPPORTED`, 502 `SIS_UNAVAILABLE`.
- **Áp một phần 2026-11-17:** phần `SyncRun` (`kind` + `csv_import`, `status` `queued`/`running`/`succeeded`/`partially_failed`/`failed`, `dry_run`, `trigger`, `failure_code`, `sis_connection_id`) đã áp vì F011 (M-C1) trả `SyncRun`; phần `triggerSisSync` còn Cần sửa. **Cần sửa** `triggerSisSync`: body thêm `dry_run` boolean (mặc định false), `additionalProperties: false`; thêm response 401, 404, 409 (`SIS_SYNC_ALREADY_RUNNING`, `SIS_CONNECTION_NOT_ACTIVE`), 422, 428; `SyncRun` thống nhất với DATABASE_DESIGN: `kind` thêm `csv_import`, `status` = `queued`/`running`/`succeeded`/`partially_failed`/`failed` (bỏ `partial`), thêm `dry_run`, `trigger`, `failure_code` (maxLength 64), `sis_connection_id`.
- **Cần thêm** ProblemCode: `SIS_SYNC_ALREADY_RUNNING`, `SIS_CONNECTION_NOT_ACTIVE`, `SIS_CONTRACT_VERSION_UNSUPPORTED`, `SIS_UNAVAILABLE` vào `ProblemCode` (`identity.v1.yaml`, `eduworld-api.v1.yaml`) + API_DESIGN §4.1.
- **Cần sửa** `specs/sis/eduworld-sis-contract.v1.yaml` (bump 1.2.1, chỉ thêm): `examples` cho mọi response 200 (dữ liệu demo TH Demo A khớp seed: trường `TH-DEMO-001`, lớp 3A/3B, `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b`); webhook `receiveSisChanges` thêm response `404` và `503`, mô tả `delivery_id` dùng làm `Nats-Msg-Id`.
- **F011 đã tạo 2026-11-17** (owner) với `sync_run_id`, `stats`, `issues_count`; F017 T007 thêm trường tương thích. ~~**Cần thêm** `sis.sync_completed.v1.schema.json` (nếu F011 chưa tạo)~~ với `sync_run_id`, `sis_connection_id`, `mode`, `status`, `dry_run`, `stats`, `issues_count`, `connection_status`, `failure_code` (maxLength 64).
- **Cần thêm** `specs/postman/sis-conformance/` (collection + env mẫu) và route APISIX `/internal/sis/webhooks/*` (internal/partner, IP allowlist).
