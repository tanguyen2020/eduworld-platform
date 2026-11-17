# Tasks: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Cấu hình oapi-codegen client mode cho `specs/sis/eduworld-sis-contract.v1.yaml` (package `contractv1`, chỉ `components.schemas` + client `paths`) và target `make gen MOD=identity` — file: `modules/identity/internal/infrastructure/sis/contractv1/oapi-codegen.yaml`, `gen.go`
- [ ] T002 [P] Biến môi trường worker: `EW_SIS_EGRESS_PROXY_URL`, `EW_SIS_SECRETS_DIR`, `EW_SIS_ALLOW_HTTP_BASE_URL` (chỉ compose dev, mặc định false) — file: `modules/identity/.env.example`, `modules/identity/internal/config/sis.go`
- [ ] T003 [P] Secret local mẫu cho connection demo (chỉ `.example`, thư mục thật trong `.gitignore`) — file: `deploy/compose/secrets/sis-demo-a.example`, `deploy/compose/secrets/sis-demo-a-webhook.example`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: mở rộng `SisConnectionCreate`/`SisConnection` cho `contract_v1`, thêm `updateSisConnection`, `checkSisConnectionHealth`, schema `SisConnectionHealth` — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T005 Delta contract: sửa `triggerSisSync` (`dry_run`, 401/404/409/422/428) và `SyncRun` (`kind` + `csv_import`, `status` `partially_failed`, `dry_run`, `trigger`, `failure_code`); thêm ProblemCode `SIS_SYNC_ALREADY_RUNNING`, `SIS_CONNECTION_NOT_ACTIVE`, `SIS_CONTRACT_VERSION_UNSUPPORTED`, `SIS_UNAVAILABLE`; cập nhật API_DESIGN §4.1 — file: `specs/openapi/modules/identity.v1.yaml`, `docs/07-backend/API_DESIGN.md` — kiểm: `make specs-lint` — **2026-11-17:** phần `SyncRun` đã áp (F011 dùng ở M-C1); còn `triggerSisSync`, ProblemCode SIS, API_DESIGN §4.1
- [ ] T006 [P] Contract SIS 1.2.1: `examples` dữ liệu demo TH Demo A cho mọi response 200; webhook thêm 404/503 — file: `specs/sis/eduworld-sis-contract.v1.yaml` — kiểm: `make specs-lint`, `prism mock` khởi động không cảnh báo
- [ ] T007 [P] Delta tương thích schema `sis.sync_completed` của owner F011 (thêm `mode`, `status`, `dry_run`, `connection_status`, `failure_code`) — file: `specs/events/payloads/identity/sis.sync_completed.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T008 ent: thêm cột `webhook_secret_ref`, `capabilities`, `last_health_at`, `last_success_at`, `consecutive_failed_runs`, `status_reason` (sis_connections); `dry_run`, `trigger`, `requested_by`, `delivery_id`, `failure_code` (sync_runs) + index — file: `modules/identity/ent/schema/sisconnection.go`, `syncrun.go` — kiểm: `make migrate-diff MOD=identity`, `make migrate-lint MOD=identity`
- [ ] T009 [P] Cập nhật DATABASE_DESIGN §2.3 (cột mới; ghi chú retention staging theo `sis_staging_retention`) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T010 Sinh code: `make gen MOD=identity` (server + client `contractv1`) — file: `modules/identity/internal/presentation/http/gen/`, `internal/infrastructure/sis/contractv1/` — kiểm: `make gen-check MOD=identity`
- [ ] T011 [P] Fake SIS `httptest` (phân trang, 401/404/410/429/5xx, chậm, trường lạ, version 2.0.0) gắn nhãn `MOCK` chỉ dùng trong test — file: `modules/identity/tests/fakesis/server.go`
- [ ] T012 [P] Port `SecretStore` + hiện thực file (Swarm secret), port `RunLock`, `WebhookNonceStore` + Valkey — file: `modules/identity/internal/application/port/write/secret_store.go`, `run_lock.go`, `webhook_nonce.go`, `internal/infrastructure/secrets/filestore.go`, `internal/infrastructure/sis/valkey/run_lock.go`, `webhook_nonce.go`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Đăng ký connection `contract_v1` (Priority: P1) 🎯 MVP

**Goal**: System Admin tạo, kiểm tra, dry-run connection `contract_v1`.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T013 [P] [US1] Test F017-AC-01: validate `config` (https, `auth_type`, `token_url` khi OAuth2, `secret_ref`, major 1), không field-mapping, audit — file: `modules/identity/tests/unit/domain/sis_connection_test.go` — phải FAIL trước khi code
- [ ] T014 [P] [US1] Test F017-AC-02: health check lưu `capabilities`; version 2.0.0 → `contract_error` + `422 SIS_CONTRACT_VERSION_UNSUPPORTED` (TC-AUTH-011) — file: `modules/identity/tests/integration/sis_health_check_test.go`
- [ ] T015 [P] [US1] Test F017-AC-03: dry-run full sync có `stats`, không apply, không event nghiệp vụ trong outbox — file: `modules/identity/tests/integration/sis_pull_dryrun_test.go`
- [ ] T016 [P] [US1] Test F017-AC-04: response/log/audit không chứa `secret_ref`, `webhook_secret_ref` (quét chuỗi) — file: `modules/identity/tests/integration/sis_secret_leak_test.go`

### Implementation cho User Story 1

- [ ] T017 [P] [US1] Domain `SISConnection` (ValidateConfig, chuyển trạng thái), `ContractVersion.CompatibleMajor` — file: `modules/identity/internal/domain/sis/connection.go`, `contract_version.go`
- [ ] T018 [US1] Command `CreateSisConnection` nhánh `contract_v1`, `CheckSisConnectionHealth` — file: `modules/identity/internal/application/command/sis_connection_contract.go`
- [ ] T019 [US1] `contractv1client`: auth OAuth2 client credentials/API key/mTLS từ `SecretStore`, `Health` — file: `modules/identity/internal/infrastructure/sis/contractv1client/client.go`, `auth.go`
- [ ] T020 [US1] Handler `createSisConnection` (nhánh contract_v1), `checkSisConnectionHealth` — file: `modules/identity/internal/presentation/http/public/sis_connections_contract.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Full sync (Priority: P1)

### Tests cho User Story 2

- [ ] T021 [P] [US2] Test F017-AC-05: full sync duyệt đủ 5 operation theo cursor cho mọi trường ánh xạ; hai connection cùng `HS-0001` vào staging theo khóa nguồn (TC-AUTH-030) — file: `modules/identity/tests/integration/sis_full_sync_test.go` — phải FAIL trước khi code
- [ ] T022 [P] [US2] Test F017-AC-06: lỗi bản ghi `SIS_CONTRACT_VIOLATION`, trường lạ không lưu + metric, vượt ngưỡng → `contract_error` (TC-AUTH-011) — file: `modules/identity/tests/integration/sis_contract_violation_test.go`
- [ ] T023 [P] [US2] Test F017-AC-07: run thứ hai cùng connection → `409 SIS_SYNC_ALREADY_RUNNING`; lịch bỏ qua + metric — file: `modules/identity/tests/integration/sis_run_lock_test.go`
- [ ] T024 [P] [US2] Test F017-AC-08, F017-AC-09: `missing_in_source` sau full sync, `sis.sync_completed` payload, `unmapped_school` — file: `modules/identity/tests/integration/sis_full_sync_complete_test.go`

### Implementation cho User Story 2

- [ ] T025 [P] [US2] Domain `SyncRun` (mode, dry_run, `ViolationRatioExceeded`) — file: `modules/identity/internal/domain/sis/syncrun.go`
- [ ] T026 [US2] `contractv1client` hiện thực `SISProvider` (List*, phân trang, kiểm cursor lặp) + validate response bằng validator F011 — file: `modules/identity/internal/infrastructure/sis/contractv1client/client.go`, `validate.go`
- [ ] T027 [US2] Command `TriggerSisSync`, `RunPullSync` (lock, health, pull → staging → reconcile/apply F011, outbox) — file: `modules/identity/internal/application/command/sis_sync.go`
- [ ] T028 [US2] Scheduler theo `sis_sync_schedule`/`schedule` + consumer kind `scheduled_sync` — file: `modules/identity/internal/application/scheduler/sis_schedule.go`, `internal/presentation/job/sis_scheduler.go`, `internal/presentation/consumer/sis_jobs.go`
- [ ] T029 [US2] Handler `triggerSisSync` — file: `modules/identity/internal/presentation/http/public/sis_connections_contract.go`

## Phase 5: User Story 3 — Lỗi, retry, trạng thái connection (Priority: P1)

### Tests cho User Story 3

- [ ] T030 [P] [US3] Test F017-AC-10, F017-AC-11: backoff + jitter, `Retry-After`, giảm tốc khi 429, giới hạn req/s — file: `modules/identity/tests/unit/infrastructure/contractv1_retry_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US3] Test F017-AC-12: 401/403/lỗi token → `auth_error`, không retry, event `connection_status` — file: `modules/identity/tests/integration/sis_auth_error_test.go`
- [ ] T032 [P] [US3] Test F017-AC-13, F017-AC-14: circuit → `paused`; trigger/webhook bị từ chối `409 SIS_CONNECTION_NOT_ACTIVE`; kích hoạt lại có audit — file: `modules/identity/tests/integration/sis_circuit_test.go`

### Implementation cho User Story 3

- [ ] T033 [P] [US3] Retry/backoff, rate limiter per connection, phân loại lỗi — file: `modules/identity/internal/infrastructure/sis/contractv1client/retry.go`, `ratelimit.go`
- [ ] T034 [US3] `RecordRunResult` (đếm lỗi liên tiếp, circuit), command `UpdateSisConnection` + handler `updateSisConnection` — file: `modules/identity/internal/domain/sis/connection.go`, `internal/application/command/sis_connection_contract.go`, `internal/presentation/http/public/sis_connections_contract.go`

## Phase 6: User Story 4 — Delta sync (Priority: P2)

### Tests cho User Story 4

- [ ] T035 [P] [US4] Test F017-AC-15, F017-AC-16: delta theo `sequence`, `record` bắt buộc khi upsert, watermark chỉ lưu sau apply, 410 → full `fallback_full` — file: `modules/identity/tests/integration/sis_delta_sync_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US4] Test F017-AC-17: `delete`/`suspended` có hiệu lực trong cùng run (TC-AUTH-036) — file: `modules/identity/tests/integration/sis_delta_status_test.go`

### Implementation cho User Story 4

- [ ] T037 [US4] `ListChanges` trong client + nhánh delta của `RunPullSync`, `ProcessChangeBatch` dùng chung với webhook — file: `modules/identity/internal/infrastructure/sis/contractv1client/client.go`, `internal/application/command/sis_sync.go`

## Phase 7: User Story 5 — Webhook (Priority: P2)

### Tests cho User Story 5

- [ ] T038 [P] [US5] Test F017-AC-18: HMAC đúng/sai, so sánh thời gian hằng, lệch `sis_webhook_skew` → 401 — file: `modules/identity/tests/unit/infrastructure/webhook_verify_test.go` — phải FAIL trước khi code
- [ ] T039 [P] [US5] Test F017-AC-19: replay `delivery_id` → 409; batch hợp lệ → job `Nats-Msg-Id`, 202 sau ack; Valkey lỗi → 503 — file: `modules/identity/tests/integration/sis_webhook_test.go`
- [ ] T040 [P] [US5] Test F017-AC-20: connection không tồn tại/không active/thiếu capability/thiếu secret → 404 — file: `modules/identity/tests/integration/sis_webhook_notfound_test.go`
- [ ] T041 [P] [US5] Test F017-AC-21: job lỗi → redeliver → `DLQ_IDENTITY`; SyncRun `mode=webhook` — file: `modules/identity/tests/integration/sis_webhook_job_test.go`

### Implementation cho User Story 5

- [ ] T042 [P] [US5] `VerifyAndParseWebhook` — file: `modules/identity/internal/infrastructure/sis/webhook/verify.go`
- [ ] T043 [US5] Command `ReceiveSisWebhook`, handler `/internal/sis/webhooks/{connection_id}`, consumer kind `webhook_batch` — file: `modules/identity/internal/application/command/sis_sync.go`, `internal/presentation/http/sisinternal/webhook.go`, `internal/presentation/consumer/sis_jobs.go`
- [ ] T044 [US5] Route APISIX internal/partner + IP allowlist cho webhook — file: `deploy/apisix/routes/internal-sis-webhook.yaml`

## Phase 8: User Story 6 — Adapter `lookupStudent` (Priority: P1)

### Tests cho User Story 6

- [ ] T045 [P] [US6] Test F017-AC-22: lookup timeout `sis_login_sync_timeout_ms`, không retry, bỏ trường lạ + metric (TC-SIS-102, TC-SIS-107) — file: `modules/identity/tests/integration/sis_lookup_adapter_test.go` — phải FAIL trước khi code
- [ ] T046 [P] [US6] Test F017-AC-23: không capability → `ErrUnsupported` 0 request; 404 → không tìm thấy; timeout/5xx → lỗi tạm thời (TC-SIS-104) — file: `modules/identity/tests/unit/infrastructure/contractv1_lookup_test.go`
- [ ] T047 [P] [US6] Test F017-AC-24: bulkhead `sis_login_sync_concurrency`, breaker riêng connection (TC-SIS-106) — file: `modules/identity/tests/integration/sis_lookup_bulkhead_test.go`

### Implementation cho User Story 6

- [ ] T048 [US6] `contractV1Lookup` hiện thực `SisLookupPort` (F011) + đăng ký theo `provider_kind` — file: `modules/identity/internal/infrastructure/sis/lookup/contractv1_lookup.go`, `internal/application/port/write/sis_provider_factory.go`

## Phase 9: User Story 7 — Conformance kit và `sis-mock` (Priority: P1)

### Tests cho User Story 7

- [ ] T049 [P] [US7] Collection conformance F017-AC-25: test mọi endpoint bắt buộc (status, JSON Schema, cursor, `limit`, problem+json, 401) — file: `specs/postman/sis-conformance/sis-conformance.postman_collection.json` — phải FAIL trước khi `sis-mock` có `examples` (TC-AUTH-012)
- [ ] T050 [P] [US7] Thư mục capability tùy chọn F017-AC-26 (`listChanges`, `lookupStudent`, webhook mẫu ký HMAC; skip khi không khai báo) — file: `specs/postman/sis-conformance/sis-conformance.postman_collection.json`
- [ ] T051 [US7] Job CI F017-AC-27: dựng `sis-mock`, chạy Postman CLI khi đổi `specs/sis/**`, `specs/postman/sis-conformance/**` (TC-AUTH-012) — file: `.github/workflows/sis-conformance.yml`

### Implementation cho User Story 7

- [ ] T052 [P] [US7] Env `sis-mock` + env mẫu cho trường, README hướng dẫn trường (tiếng Việt) — file: `specs/postman/sis-conformance/env.sis-mock.postman_environment.json`, `env.template.postman_environment.json`, `README.md`
- [ ] T053 [US7] Service `sis-mock` trong compose: lệnh Prism static, healthcheck, mount contract — file: `deploy/compose/docker-compose.yml`

## Phase 10: User Story 8 — Bảo vệ API (Priority: P1)

### Tests cho User Story 8

- [ ] T054 [P] [US8] Test tenant isolation + authorization F017-AC-28 cho `createSisConnection` (contract_v1), `updateSisConnection`, `checkSisConnectionHealth`, `triggerSisSync`: ma trận role, trường/tenant khác → 404 (TC-TEN-001) — file: `modules/identity/tests/integration/sis_contract_authz_test.go` — phải FAIL trước khi code
- [ ] T055 [P] [US8] Test F017-AC-29: thiếu `Idempotency-Key` → 428, lặp key trả cùng kết quả, audit actor `system:sis-sync:<id>` — file: `modules/identity/tests/integration/sis_contract_idempotency_test.go`
- [ ] T056 [P] [US8] Test F017-AC-30: `maxLength`/pattern/https → 422; route `/api/v1/sis/*` không có ở gateway public — file: `modules/identity/tests/contract/sis_connection_validation_test.go`, `deploy/apisix/tests/sis_routes_test.sh`

### Implementation cho User Story 8

- [ ] T057 [US8] Gọi tường minh `authz.Require(ctx, p, perm, resource)` ở đầu command handler, scope trường cho `tenant.sis.import` khi trigger — file: `modules/identity/internal/application/command/sis_sync.go`, `internal/presentation/http/public/sis_connections_contract.go`

## Phase N: Polish & Cross-Cutting

- [ ] T058 [P] Đăng ký tham số ngưỡng vi phạm, lịch retry, rate limit, ngưỡng circuit breaker; kiểm `sis_lookup_timeout` đã bỏ, chỉ còn `sis_login_sync_timeout_ms` (RB-111) — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/09-identity/SIS_INTEGRATION.md`
- [ ] T059 [P] Đồng bộ tên metric SIS giữa SIS_INTEGRATION §8 và MONITORING; dashboard SIS — file: `docs/11-operations/MONITORING.md`
- [ ] T060 [P] Cập nhật SIS_INTEGRATION §2.6 (conformance kit ở `specs/postman/sis-conformance/`), §10 (path `sync-runs`), CHANGELOG — file: `docs/09-identity/SIS_INTEGRATION.md`, `CHANGELOG.md`
- [ ] T061 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 (cần client US2) → US6 (cần client + capability US1) → US4 → US5 (dùng `ProcessChangeBatch` US4) → US7 (song song từ sau T006) → US8 (test authz viết sớm, chạy lại cuối) → Polish.
- Cần F011 Done: port `SISProvider`, `SisLookupPort`, validator, reconcile/apply, consumer `sis-jobs`, `createSisConnection`.

## Definition of Done

- [ ] Mọi tiêu chí F017-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (MOD=identity) và conformance Postman với `sis-mock` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; fake SIS chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
