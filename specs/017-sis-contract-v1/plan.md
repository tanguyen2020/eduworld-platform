# Implementation Plan: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Branch**: `017-sis-contract-v1` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm hiện thực thứ hai của port `SISProvider` (F011): `contractV1Client` sinh bằng oapi-codegen (client mode) từ `specs/sis/eduworld-sis-contract.v1.yaml`, bọc rate limit, retry/backoff, `Retry-After`, OAuth2 client credentials/API key/mTLS với credential đọc từ secret manager theo `secret_ref`. `identity-worker` có scheduler (full/delta theo `sis_sync_schedule`), orchestrator `PullSyncRun` (lock một run/connection, version check, ngưỡng vi phạm, circuit breaker) đưa bản ghi vào pipeline staging → reconcile → apply của F011. Webhook `/internal/sis/webhooks/{connection_id}` xác thực HMAC, chống replay (Valkey nonce), publish job `webhook_batch` vào `IDENTITY_SIS_JOBS`. Adapter `contractV1Lookup` hiện thực `SisLookupPort` cho login sync. Conformance kit = Postman collection `specs/postman/sis-conformance/`; `sis-mock` = Prism với `examples` demo.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | oapi-codegen (client từ `specs/sis`, server từ `identity.v1.yaml`), ent, Atlas, nats.go JetStream, go-redis (Valkey), golang.org/x/oauth2/clientcredentials, golang.org/x/time/rate, failsafe-go (retry, circuit breaker, bulkhead cho adapter tích hợp ngoài; `src/backend/pkg/modcall` không có resilience — ADR-0074), robfig/cron v3; Postman CLI; stoplight/prism |
| Storage | PostgreSQL `eduworld_identity` (`sis.sis_connections`, `sis.sync_runs` — thêm cột); Valkey (lock run, nonce webhook — tạm, không phải nguồn sự thật, R9); secret manager (Swarm secret/file local) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); fake SIS `httptest` (chỉ test, nhãn `MOCK`); Postman CLI + `sis-mock` trong CI |
| Target Platform | Linux container (module `identity` api + worker); `sis-mock` trong `deploy/compose` |
| Project Type | backend module + công cụ kiểm thử |
| Performance Goals | Full sync 50.000 học sinh < 15 phút [Proposed] ([SIS_INTEGRATION §11](../../docs/09-identity/SIS_INTEGRATION.md)) |
| Constraints | AGENTS R3, R4, R9, R10, R13, R14, R28, R29 (không áp dụng: SIS là hệ thống ngoài, không phải service EduWorld), R30; ADR-0009, ADR-0021, ADR-0042, ADR-0045, ADR-0046, ADR-0061 |
| Scale/Scope | `sis_login_sync_concurrency`, `sis_sync_schedule`; hàng trăm connection |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `identity.v1.yaml` (connection `contract_v1`, health check, update, `triggerSisSync`), `examples` trong contract SIS, event payload, collection conformance ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Toàn bộ trong module identity; client SIS ở `infrastructure`, sau port `SISProvider`/`SisLookupPort`; command handler (write context) gọi qua interface có kiểu (ADR-0087) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Connection T có `tenant_id`; job chạy `TenantScope` theo tenant của connection; không hard-delete; Valkey chỉ giữ lock/nonce |
| IV Server-authoritative | ✅ | Không điểm/phần thưởng; `Idempotency-Key` tạo connection/trigger; webhook idempotent theo `delivery_id`; audit mỗi run |
| V Xác thực, phân quyền | ✅ | `platform.sis_connection.manage`, `tenant.sis.import` (scope trường); `/api/v1/sis/*` và webhook chỉ gateway nội bộ; credential chỉ qua `secret_ref` |
| VI An toàn trẻ em | ✅ | Data minimization theo DTO; không log `full_name`, `date_of_birth`, `national_student_id`; trường lạ không lưu |
| VII Một nguồn sự thật, cấu hình | ✅ | DTO sinh từ contract; tham số theo tên CONFIG_DEFAULTS; giá trị chưa đăng ký có task Polish |
| VIII UI tách logic, token | ✅ | Không có UI (F052/F059) |
| IX Test có bằng chứng | ✅ | Mỗi F017-AC có test viết trước; fake SIS chỉ trong test; conformance chạy CI |

## Project Structure

### Documentation (feature này)

```text
specs/017-sis-contract-v1/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/identity/
├── ent/schema/sisconnection.go, syncrun.go           # thêm cột (data-model.md)
├── internal/domain/sis/
│   ├── connection.go        # SISConnection: ValidateConfig, Activate/Pause/MarkAuthError/MarkContractError, RecordRunResult (circuit)
│   ├── contract_version.go  # CompatibleMajor
│   └── syncrun.go           # mode full/delta/webhook, dry_run, ngưỡng vi phạm
├── internal/application/
│   ├── command/sis_connection_contract.go   # CreateSisConnection (nhánh contract_v1), UpdateSisConnection, CheckSisConnectionHealth
│   ├── command/sis_sync.go                  # TriggerSisSync, RunPullSync, ProcessChangeBatch, ReceiveSisWebhook
│   ├── port/{secret_store.go, run_lock.go, webhook_nonce.go, sis_provider_factory.go}
│   └── scheduler/sis_schedule.go            # tính lịch từ sis_sync_schedule
├── internal/infrastructure/sis/
│   ├── contractv1/          # sinh bởi oapi-codegen từ specs/sis (gen.go + oapi-codegen.yaml)
│   ├── contractv1client/    # client.go (SISProvider), auth.go, ratelimit.go, retry.go, validate.go
│   ├── lookup/contractv1_lookup.go          # SisLookupPort cho contract_v1 (bulkhead + breaker/connection)
│   ├── webhook/verify.go    # VerifyAndParseWebhook (HMAC, skew)
│   └── valkey/{run_lock.go, webhook_nonce.go}
├── internal/infrastructure/secrets/filestore.go     # đọc secret theo secret_ref (Swarm secret / file local)
├── internal/presentation/http/public/sis_connections_contract.go   # create/update/health-check, triggerSisSync
├── internal/presentation/http/sisinternal/webhook.go               # POST /internal/sis/webhooks/{connection_id}
├── internal/presentation/job/sis_scheduler.go
├── internal/presentation/consumer/sis_jobs.go       # thêm kind scheduled_sync, webhook_batch (file của F011)
└── tests/{unit,integration,contract}/sis_contractv1_*.go, tests/fakesis/server.go
specs/postman/sis-conformance/
├── sis-conformance.postman_collection.json
├── env.sis-mock.postman_environment.json · env.template.postman_environment.json
└── README.md                              # hướng dẫn cho trường
deploy/compose/docker-compose.yml          # service sis-mock có sẵn (F001): lệnh `prism mock -h 0.0.0.0 -p 4010` (static examples), healthcheck
deploy/apisix/routes/internal-sis-webhook.yaml
```

**Structure Decision**: Theo BACKEND_DESIGN §3 và port `SISProvider` của [SIS_INTEGRATION §4.1](../../docs/09-identity/SIS_INTEGRATION.md). Webhook ở package presentation riêng `sisinternal` vì xác thực HMAC, không Bearer. Conformance kit nằm dưới `specs/postman/` theo SIS_INTEGRATION §2.6.

## Phase 0: Research

→ [research.md](research.md): R1 `sis-mock` Prism static vs server tự viết; R2 nơi lưu nonce/lock; R3 client sinh từ OpenAPI 3.1 có `webhooks`; R4 đọc secret theo `secret_ref`.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `SISConnection` (trạng thái `active`/`auth_error`/`contract_error`/`paused`, `consecutive_failed_runs`, `capabilities`), `ContractVersion.CompatibleMajor`, `SyncRun` (`mode`, `dry_run`, `ViolationRatioExceeded`) | Thuần, không I/O |
| application | Command `CreateSisConnection` (contract_v1), `UpdateSisConnection`, `CheckSisConnectionHealth`, `TriggerSisSync`, `RunPullSync`, `ProcessChangeBatch`, `ReceiveSisWebhook` | Permission ở [contracts](contracts/README.md) §1; webhook không qua authz người, qua HMAC |
| infrastructure | `contractv1client` (SISProvider), `contractV1Lookup` (SisLookupPort), secret store, Valkey lock/nonce, ent repo | Dùng validator của F011 (`internal/infrastructure/sis/validator`) |
| presentation | `createSisConnection`, `updateSisConnection`, `checkSisConnectionHealth`, `triggerSisSync`; webhook `receiveSisChanges`; job `sis-scheduler`; consumer `sis-jobs` (kind `scheduled_sync`, `webhook_batch`) | |

### Luồng `RunPullSync`

1. Lấy lock `ew:identity:sis:lock:<connection_id>` (heartbeat); không được → `SIS_SYNC_ALREADY_RUNNING`/bỏ qua.
2. Đọc secret theo `secret_ref`; `getHealth` → kiểm major, cập nhật `capabilities`.
3. `full`: duyệt `schools[]` của `config` ∩ trường đã ánh xạ → năm học hiện hành (`is_current`) → lớp/học sinh/enrollment theo cursor; `delta`: `listChanges(since)`.
4. Mỗi trang: validate (F011) → staging; vượt ngưỡng vi phạm → dừng, `contract_error`.
5. Gọi `Reconciler`/apply của F011 (bỏ qua apply khi `dry_run`); full: gửi tập khóa đã thấy để đánh dấu `missing_in_source`.
6. Kết thúc: cập nhật `delta_watermark` (chỉ khi apply thành công), `last_success_at`, `consecutive_failed_runs`; outbox `sis.sync_completed` + audit.

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F011 (port `SISProvider`, `SisLookupPort`, validator, staging/reconcile/apply, consumer `sis-jobs`, `createSisConnection`/`setSisConnectionSchools`), F004 (APISIX internal), F003 (outbox, `UnitOfWork`, idempotency), F001 (`sis-mock` trong compose).
- Event phát: `sis.sync_completed` (mở rộng payload), `audit.recorded`; event nghiệp vụ do apply F011 phát.
- Job: publish `ew.job.identity.sis.scheduled_sync`, `ew.job.identity.sis.webhook_batch` vào `IDENTITY_SIS_JOBS`.

### Bảo mật, tenant, idempotency, audit

- Permission `platform.sis_connection.manage` (create/update/health-check/trigger mọi trường), `tenant.sis.import` scope trường (trigger cho trường mình). Route `/api/v1/sis/*` chỉ APISIX internal + MFA với role platform.
- Webhook: HMAC-SHA256, `sis_webhook_skew`, nonce `ew:identity:sis:wh:nonce:<delivery_id>` TTL `sis_webhook_nonce_ttl`; Valkey lỗi → `503` (fail closed); APISIX IP allowlist theo connection.
- Idempotency-Key: `createSisConnection`, `triggerSisSync`; audit `sis_connection.created/updated/status_changed`, `sis.sync_run.*` actor `system:sis-sync:<connection_id>`.
- Egress cố định, TLS 1.2+, không tắt kiểm cert; không log body response SIS.

### Quan sát

- Metric: `eduworld_sis_sync_runs_total{connection,mode,status}`, `eduworld_sis_sync_duration_seconds`, `eduworld_sis_records_total`, `eduworld_sis_record_errors_total`, `eduworld_sis_contract_unknown_fields_total`, `eduworld_sis_last_success_timestamp`, `eduworld_sis_http_requests_total{connection,operation,status}`, `eduworld_sis_webhook_total{result}`, `eduworld_sis_login_sync_total` (adapter góp nhãn `result`).
- Span `identity.command.RunPullSync`, client span mỗi operation SIS (không gắn query `login_username`).
- Alert `SISSyncFailed`, `SisLookupTimeouts` ([MONITORING](../../docs/11-operations/MONITORING.md)).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
