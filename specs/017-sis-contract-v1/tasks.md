# Tasks: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Cấu hình oapi-codegen client mode cho `specs/sis/eduworld-sis-contract.v1.yaml` (package `contractv1`, chỉ `components.schemas` + client `paths`) và target `make gen MOD=identity` — file: `src/backend/modules/identity/internal/infrastructure/sis/contractv1/oapi-codegen.yaml`, `gen.go` — **Done 2026-11-19:** thay oapi-codegen bằng client viết tay + DTO kiểm bằng JSON Schema trích từ contract (`tools/sisgen`, OQ F017-2): `src/backend/modules/identity/internal/infrastructure/sis/contractv1client/`; `go run ./tools/sisgen --check` xanh
- [x] T002 [P] Biến môi trường worker: `EW_SIS_EGRESS_PROXY_URL`, `EW_SIS_SECRETS_DIR`, `EW_SIS_ALLOW_HTTP_BASE_URL` (chỉ compose dev, mặc định false) — file: `src/backend/modules/identity/.env.example`, `src/backend/modules/identity/internal/config/sis.go` — **Done 2026-11-19:** `contractv1client.ConfigFrom` (`EW_SIS_SECRETS_DIR`, `EW_SIS_EGRESS_PROXY_URL`, `EW_SIS_ALLOW_HTTP_BASE_URL`) ở `internal/infrastructure/sis/contractv1client/config.go` (không có `internal/config` trong module); `.env.example` thuộc deny-list của agent nên chưa sửa (OQ F017-9)
- [x] T003 [P] Secret local mẫu cho connection demo (chỉ `.example`, thư mục thật trong `.gitignore`) — file: `deploy/compose/secrets/sis-demo-a.example`, `deploy/compose/secrets/sis-demo-a-webhook.example` — **Done 2026-11-19:** `deploy/compose/sis-secrets.example/` (thư mục `secrets/` bị `.gitignore` nên mẫu nằm ngoài, README hướng dẫn chép)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: mở rộng `SisConnectionCreate`/`SisConnection` cho `contract_v1`, thêm `updateSisConnection`, `checkSisConnectionHealth`, schema `SisConnectionHealth` — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **Done 2026-11-19:** spec gộp `eduworld-api.v1.yaml` (nguồn sự thật) → `SisConnectionCreate` oneOf + discriminator, `SisContractConfig`, `SisSecretRef`, `SisConnectionUpdate`, `SisConnectionHealth`, `SisCapability`, `updateSisConnection`, `checkSisConnectionHealth`, response `BadGateway`; `node scripts/openapi/split-public.mjs`; menu catalog thêm 2 operation; `check-maxlength`/`check-enums`/`check-portal-catalog`/`validate-schemas` 0 lỗi
- [x] T005 Delta contract: sửa `triggerSisSync` (`dry_run`, 401/404/409/422/428) và `SyncRun` (`kind` + `csv_import`, `status` `partially_failed`, `dry_run`, `trigger`, `failure_code`); thêm ProblemCode `SIS_SYNC_ALREADY_RUNNING`, `SIS_CONNECTION_NOT_ACTIVE`, `SIS_CONTRACT_VERSION_UNSUPPORTED`, `SIS_UNAVAILABLE`; cập nhật API_DESIGN §4.1 — file: `specs/openapi/modules/identity.v1.yaml`, `docs/07-backend/API_DESIGN.md` — kiểm: `make specs-lint` — **2026-11-17:** phần `SyncRun` đã áp (F011 dùng ở M-C1); còn `triggerSisSync`, ProblemCode SIS, API_DESIGN §4.1 — **Done 2026-11-19:** `triggerSisSync` (`dry_run`, `additionalProperties: false`, 401/404/409/422/428); ProblemCode `SIS_SYNC_ALREADY_RUNNING`, `SIS_CONNECTION_NOT_ACTIVE`, `SIS_CONTRACT_VERSION_UNSUPPORTED`, `SIS_UNAVAILABLE` + API_DESIGN §4.1 + `go run ./pkg/problem/gen`; `check-problem-codes` 167/167
- [x] T006 [P] Contract SIS 1.2.1: `examples` dữ liệu demo TH Demo A cho mọi response 200; webhook thêm 404/503 — file: `specs/sis/eduworld-sis-contract.v1.yaml` — kiểm: `make specs-lint`, `prism mock` khởi động không cảnh báo — **Done 2026-11-19:** contract 1.2.1: `examples` TH Demo A cho mọi 200, webhook 404/503; Prism 4 khởi động "Prism is listening", conformance 22/22 assertion pass
- [x] T007 [P] Delta tương thích schema `sis.sync_completed` của owner F011 (thêm `mode`, `status`, `dry_run`, `connection_status`, `failure_code`) — file: `specs/events/payloads/identity/sis.sync_completed.v1.schema.json` — kiểm: `make specs-lint` — **Done 2026-11-19:** thêm `sis_connection_id`, `mode`, `status`, `dry_run`, `connection_status`, `failure_code` (tùy chọn) + ví dụ; `validate-schemas` 0 lỗi
- [x] T008 ent: thêm cột `webhook_secret_ref`, `capabilities`, `last_health_at`, `last_success_at`, `consecutive_failed_runs`, `status_reason` (sis_connections); `dry_run`, `trigger`, `requested_by`, `delivery_id`, `failure_code` (sync_runs) + index — file: `src/backend/modules/identity/ent/schema/sisconnection.go`, `syncrun.go` — kiểm: `make migrate-diff MOD=identity`, `make migrate-lint MOD=identity` — **Done 2026-11-19:** ent `sisconnection` (+6 cột, index `sis_connections_schedule_idx`), `syncrun` (+`requested_by`, `delivery_id`; `dry_run`/`trigger`/`failure_code` đã có từ F011); migration `20261124000001_f017_sis_contract.sql` (ALTER + CHECK + chép `config.capabilities` + view `_live`), `entx.AddedDDL`/`entx.Without` + golden test dạng ALTER; `go test ./modules/identity/ent/` ok; `atlas.sum` sinh lại
- [x] T009 [P] Cập nhật DATABASE_DESIGN §2.3 (cột mới; ghi chú retention staging theo `sis_staging_retention`) — file: `docs/07-backend/DATABASE_DESIGN.md` — **Done 2026-11-19:** DATABASE_DESIGN §2.3 (cột mới, retention staging)
- [x] T010 Sinh code: `make gen MOD=identity` (server + client `contractv1`) — file: `src/backend/modules/identity/internal/presentation/http/gen/`, `internal/infrastructure/sis/contractv1/` — kiểm: `make gen-check MOD=identity` — **Done 2026-11-19:** không có bước sinh server/client (OQ F017-2); `pkg/problem/codes_gen.go`, `pnpm gen:api` (portal) và `tools/sisgen` sinh lại, `gen:check` khớp
- [x] T011 [P] Fake SIS `httptest` (phân trang, 401/404/410/429/5xx, chậm, trường lạ, version 2.0.0) gắn nhãn `MOCK` chỉ dùng trong test — file: `src/backend/modules/identity/tests/fakesis/server.go` — **Done 2026-11-19:** `src/backend/modules/identity/tests/fakesis/server.go` (nhãn `MOCK`: trang cursor, 401/404/410/429/5xx, `Retry-After`, chậm, trường lạ, version bất kỳ, OAuth2 token, cursor lặp)
- [x] T012 [P] Port `SecretStore` + hiện thực file (Swarm secret), port `RunLock`, `WebhookNonceStore` + Valkey — file: `src/backend/modules/identity/internal/application/port/write/secret_store.go`, `run_lock.go`, `webhook_nonce.go`, `internal/infrastructure/secrets/filestore.go`, `internal/infrastructure/sis/valkey/run_lock.go`, `webhook_nonce.go` — **Done 2026-11-19:** port `SecretStore`, `RunLock`, `WebhookNonces` trong `internal/application/port/sis_contract.go`; `internal/infrastructure/secrets/filestore.go` (TestFileStore), `internal/infrastructure/cache/sis.go` (Valkey SET NX PX + heartbeat Lua; dùng thật trong TestSisContractWebhook)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Đăng ký connection `contract_v1` (Priority: P1) 🎯 MVP

**Goal**: System Admin tạo, kiểm tra, dry-run connection `contract_v1`.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T013 [P] [US1] Test F017-AC-01: validate `config` (https, `auth_type`, `token_url` khi OAuth2, `secret_ref`, major 1), không field-mapping, audit — file: `src/backend/modules/identity/tests/unit/domain/sis_connection_test.go` — phải FAIL trước khi code — **Done 2026-11-19:** `internal/domain/sis/contract_test.go` TestValidateContract (OQ F017-11) + TestSisContractConnection (audit `sis_connection.created`)
- [x] T014 [P] [US1] Test F017-AC-02: health check lưu `capabilities`; version 2.0.0 → `contract_error` + `422 SIS_CONTRACT_VERSION_UNSUPPORTED` (TC-AUTH-011) — file: `src/backend/modules/identity/tests/integration/sis_health_check_test.go` — **Done 2026-11-19:** TestSisContractConnection (capabilities + `last_health_at`, 2.0.0 → `contract_error`/`version_mismatch` + `SIS_CONTRACT_VERSION_UNSUPPORTED`), TestCompatibleMajor, TestClientPagesAndValidation
- [x] T015 [P] [US1] Test F017-AC-03: dry-run full sync có `stats`, không apply, không event nghiệp vụ trong outbox — file: `src/backend/modules/identity/tests/integration/sis_pull_dryrun_test.go` — **Done 2026-11-19:** TestSisContractPull (dry run: 9 staging, 0 user, 0 `student.synced`)
- [x] T016 [P] [US1] Test F017-AC-04: response/log/audit không chứa `secret_ref`, `webhook_secret_ref` (quét chuỗi) — file: `src/backend/modules/identity/tests/integration/sis_secret_leak_test.go` — **Done 2026-11-19:** TestSisContractConnection (outbox/audit không chứa `sis/th-demo-a`), TestSisContractHTTP (response không có `secret_ref`), tests/contract TestSisConnectionSchemas, TestFileStore (lỗi không lộ tham chiếu)

### Implementation cho User Story 1

- [x] T017 [P] [US1] Domain `SISConnection` (ValidateConfig, chuyển trạng thái), `ContractVersion.CompatibleMajor` — file: `src/backend/modules/identity/internal/domain/sis/connection.go`, `contract_version.go` — **Done 2026-11-19:** `internal/domain/sis/contract.go` (ValidateContract, ConnectionState, CompatibleMajor, ViolationRatioExceeded), `cron.go`
- [x] T018 [US1] Command `CreateSisConnection` nhánh `contract_v1`, `CheckSisConnectionHealth` — file: `src/backend/modules/identity/internal/application/command/sis_connection_contract.go` — **Done 2026-11-19:** `internal/application/command/sis_connection_contract.go` (`SisContract.Create/Update/CheckHealth`), `SisConnections.CreateContract`
- [x] T019 [US1] `contractv1client`: auth OAuth2 client credentials/API key/mTLS từ `SecretStore`, `Health` — file: `src/backend/modules/identity/internal/infrastructure/sis/contractv1client/client.go`, `auth.go` — **Done 2026-11-19:** `contractv1client/client.go` (API key, OAuth2 client credentials + làm mới một lần, mTLS PEM, TLS 1.2+, không theo redirect), `ops.go` Health; TestClientOAuth2
- [x] T020 [US1] Handler `createSisConnection` (nhánh contract_v1), `checkSisConnectionHealth` — file: `src/backend/modules/identity/internal/presentation/http/public/sis_connections_contract.go` — **Done 2026-11-19:** `internal/presentation/http/sis_admin.go` (nhánh `contract_v1` của `createSisConnection`), `sis_contract.go` (`checkSisConnectionHealth`); TestSisContractHTTP

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Full sync (Priority: P1)

### Tests cho User Story 2

- [x] T021 [P] [US2] Test F017-AC-05: full sync duyệt đủ 5 operation theo cursor cho mọi trường ánh xạ; hai connection cùng `HS-0001` vào staging theo khóa nguồn (TC-AUTH-030) — file: `src/backend/modules/identity/tests/integration/sis_full_sync_test.go` — phải FAIL trước khi code — **Done 2026-11-19:** TestSisContractPull (5 operation theo cursor, PageSize 2), TestClientPagesAndValidation
- [x] T022 [P] [US2] Test F017-AC-06: lỗi bản ghi `SIS_CONTRACT_VIOLATION`, trường lạ không lưu + metric, vượt ngưỡng → `contract_error` (TC-AUTH-011) — file: `src/backend/modules/identity/tests/integration/sis_contract_violation_test.go` — **Done 2026-11-19:** TestSisContractPull (1 lỗi `SIS_CONTRACT_VIOLATION` cột `status`, `phone` không có trong staging, metric tăng; > 10% → `SIS_CONTRACT_VIOLATION_THRESHOLD` + `contract_error`), TestViolationRatio
- [x] T023 [P] [US2] Test F017-AC-07: run thứ hai cùng connection → `409 SIS_SYNC_ALREADY_RUNNING`; lịch bỏ qua + metric — file: `src/backend/modules/identity/tests/integration/sis_run_lock_test.go` — **Done 2026-11-19:** TestSisContractPull (trigger thứ hai → `SIS_SYNC_ALREADY_RUNNING`; lịch bỏ qua + `eduworld_sis_sync_runs_skipped_total{reason=locked}`; run thủ công chờ lock → `ErrRunLockBusy`), TestSisContractHTTP (409 qua HTTP)
- [x] T024 [P] [US2] Test F017-AC-08, F017-AC-09: `missing_in_source` sau full sync, `sis.sync_completed` payload, `unmapped_school` — file: `src/backend/modules/identity/tests/integration/sis_full_sync_complete_test.go` — **Done 2026-11-19:** TestSisContractPull (`missing_runs = 1`, `sis.sync_completed` có `mode`/`connection_status`, `unmapped_school` TH-LA-999, không tạo trường)

### Implementation cho User Story 2

- [x] T025 [P] [US2] Domain `SyncRun` (mode, dry_run, `ViolationRatioExceeded`) — file: `src/backend/modules/identity/internal/domain/sis/syncrun.go` — **Done 2026-11-19:** `ConnectionState.RecordRun`, `ViolationRatioExceeded`, hằng mode/trigger/failure code trong `internal/domain/sis/contract.go`
- [x] T026 [US2] `contractv1client` hiện thực `SISProvider` (List*, phân trang, kiểm cursor lặp) + validate response bằng validator F011 — file: `src/backend/modules/identity/internal/infrastructure/sis/contractv1client/client.go`, `validate.go` — **Done 2026-11-19:** `contractv1client/ops.go` (List*, kiểm cursor lặp → `ErrSisPagination`, `Strip` + `Validate` của validator F011)
- [x] T027 [US2] Command `TriggerSisSync`, `RunPullSync` (lock, health, pull → staging → reconcile/apply F011, outbox) — file: `src/backend/modules/identity/internal/application/command/sis_sync.go` — **Done 2026-11-19:** `internal/application/command/sis_sync.go` (`Trigger`, `Execute`, full/delta/changes, `finish`), `sis_run_pull.go` (`SisRun.ProcessPulled`, `RaiseRunIssue`, delete), F011 `SisRun.execute` tách `process`
- [x] T028 [US2] Scheduler theo `sis_sync_schedule`/`schedule` + consumer kind `scheduled_sync` — file: `src/backend/modules/identity/internal/application/scheduler/sis_schedule.go`, `internal/presentation/job/sis_scheduler.go`, `internal/presentation/consumer/sis_jobs.go` — **Done 2026-11-19:** `command.ScheduleSisSync` + `job.SisSchedulerInterval` (30 s), consumer `messaging.SisJobHandlers.ScheduledSync`; TestSisContractDelta (scheduler delta 09:30 ICT, full 01:00 ICT, Nats-Msg-Id `sched:…`)
- [x] T029 [US2] Handler `triggerSisSync` — file: `src/backend/modules/identity/internal/presentation/http/public/sis_connections_contract.go` — **Done 2026-11-19:** `internal/presentation/http/sis_contract.go` (`RegisterSisContract`); tests/contract TestSisContractOperationsMatchSpec

## Phase 5: User Story 3 — Lỗi, retry, trạng thái connection (Priority: P1)

### Tests cho User Story 3

- [x] T030 [P] [US3] Test F017-AC-10, F017-AC-11: backoff + jitter, `Retry-After`, giảm tốc khi 429, giới hạn req/s — file: `src/backend/modules/identity/tests/unit/infrastructure/contractv1_retry_test.go` — phải FAIL trước khi code — **Done 2026-11-19:** TestClientRetries (503×2 → 1 s, 2 s + jitter; `Retry-After: 2`; hết 6 lần → `ErrSisUnavailable`; 401 không retry; 410), TestClientRateLimit (10 req/s, chậm một nửa sau 429)
- [x] T031 [P] [US3] Test F017-AC-12: 401/403/lỗi token → `auth_error`, không retry, event `connection_status` — file: `src/backend/modules/identity/tests/integration/sis_auth_error_test.go` — **Done 2026-11-19:** TestSisContractPull (401 → `failed` `SIS_AUTH_FAILED`, `auth_error`, 1 lời gọi, event `connection_status = auth_error`), TestClientOAuth2
- [x] T032 [P] [US3] Test F017-AC-13, F017-AC-14: circuit → `paused`; trigger/webhook bị từ chối `409 SIS_CONNECTION_NOT_ACTIVE`; kích hoạt lại có audit — file: `src/backend/modules/identity/tests/integration/sis_circuit_test.go` — **Done 2026-11-19:** TestSisContractPull (5 run lỗi → `paused`/`circuit_open` + audit; trigger → `SIS_CONNECTION_NOT_ACTIVE`; kích hoạt lại → `active`, đếm 0), TestCircuitBreaker, TestSisContractWebhook (paused → 404)

### Implementation cho User Story 3

- [x] T033 [P] [US3] Retry/backoff, rate limiter per connection, phân loại lỗi — file: `src/backend/modules/identity/internal/infrastructure/sis/contractv1client/retry.go`, `ratelimit.go` — **Done 2026-11-19:** `contractv1client/client.go` (`pacer`, backoff + jitter, `Retry-After` ≤ 5 phút, phân loại lỗi)
- [x] T034 [US3] `RecordRunResult` (đếm lỗi liên tiếp, circuit), command `UpdateSisConnection` + handler `updateSisConnection` — file: `src/backend/modules/identity/internal/domain/sis/connection.go`, `internal/application/command/sis_connection_contract.go`, `internal/presentation/http/public/sis_connections_contract.go` — **Done 2026-11-19:** `ConnectionState.RecordRun/Activate`, `SisContract.Update` (If-Match, `reason`, health check khi kích hoạt lại), handler `updateSisConnection`

## Phase 6: User Story 4 — Delta sync (Priority: P2)

### Tests cho User Story 4

- [x] T035 [P] [US4] Test F017-AC-15, F017-AC-16: delta theo `sequence`, `record` bắt buộc khi upsert, watermark chỉ lưu sau apply, 410 → full `fallback_full` — file: `src/backend/modules/identity/tests/integration/sis_delta_sync_test.go` — phải FAIL trước khi code — **Done 2026-11-19:** TestSisContractDelta (2 trang, watermark `wm-3` sau apply, lần sau gửi `since=wm-3`; 410 → `SIS_WATERMARK_EXPIRED`, watermark giữ, job `fallback_full`), TestParseChanges
- [x] T036 [P] [US4] Test F017-AC-17: `delete`/`suspended` có hiệu lực trong cùng run (TC-AUTH-036) — file: `src/backend/modules/identity/tests/integration/sis_delta_status_test.go` — **Done 2026-11-19:** TestSisContractDelta (`suspended` → user `locked`, delete enrollment → `ended` trong cùng run)

### Implementation cho User Story 4

- [x] T037 [US4] `ListChanges` trong client + nhánh delta của `RunPullSync`, `ProcessChangeBatch` dùng chung với webhook — file: `src/backend/modules/identity/internal/infrastructure/sis/contractv1client/client.go`, `internal/application/command/sis_sync.go` — **Done 2026-11-19:** `Client.ListChanges`, `SisSync.delta`/`changes` dùng chung với webhook (OQ F017-5)

## Phase 7: User Story 5 — Webhook (Priority: P2)

### Tests cho User Story 5

- [x] T038 [P] [US5] Test F017-AC-18: HMAC đúng/sai, so sánh thời gian hằng, lệch `sis_webhook_skew` → 401 — file: `src/backend/modules/identity/tests/unit/infrastructure/webhook_verify_test.go` — phải FAIL trước khi code — **Done 2026-11-19:** `internal/infrastructure/sis/webhook/verify_test.go` TestVerify (khóa sai, body sửa, ±6 phút, thiếu tiền tố, > 500 thay đổi, major 2)
- [x] T039 [P] [US5] Test F017-AC-19: replay `delivery_id` → 409; batch hợp lệ → job `Nats-Msg-Id`, 202 sau ack; Valkey lỗi → 503 — file: `src/backend/modules/identity/tests/integration/sis_webhook_test.go` — **Done 2026-11-19:** TestSisContractWebhook (202 sau ack, 1 job; replay → 409; Valkey lỗi → 503)
- [x] T040 [P] [US5] Test F017-AC-20: connection không tồn tại/không active/thiếu capability/thiếu secret → 404 — file: `src/backend/modules/identity/tests/integration/sis_webhook_notfound_test.go` — **Done 2026-11-19:** TestSisContractWebhook (connection lạ, connection thiếu webhook secret, connection paused → 404)
- [x] T041 [P] [US5] Test F017-AC-21: job lỗi → redeliver → `DLQ_IDENTITY`; SyncRun `mode=webhook` — file: `src/backend/modules/identity/tests/integration/sis_webhook_job_test.go` — **Done 2026-11-19:** TestSisContractWebhook (run `mode = webhook` `delivery_id` succeeded; lỗi 5 lần → `DLQ_IDENTITY`)

### Implementation cho User Story 5

- [x] T042 [P] [US5] `VerifyAndParseWebhook` — file: `src/backend/modules/identity/internal/infrastructure/sis/webhook/verify.go` — **Done 2026-11-19:** `internal/infrastructure/sis/webhook/verify.go` (`Verifier.Verify`, `Sign`, `Parse`)
- [x] T043 [US5] Command `ReceiveSisWebhook`, handler `/internal/sis/webhooks/{connection_id}`, consumer kind `webhook_batch` — file: `src/backend/modules/identity/internal/application/command/sis_sync.go`, `internal/presentation/http/sisinternal/webhook.go`, `internal/presentation/consumer/sis_jobs.go` — **Done 2026-11-19:** `command.ReceiveSisWebhook` (`sis_webhook.go`), `internal/presentation/sisinternal/webhook.go`, `messaging.SisJobHandlers.WebhookBatch` (5 lần, 5 phút, DLQ `ew.dlq.identity.sis_jobs`, OQ F017-6)
- [x] T044 [US5] Route APISIX internal/partner + IP allowlist cho webhook — file: `deploy/apisix/routes/internal-sis-webhook.yaml` — **Done 2026-11-19:** route `sis-webhook` trong `deploy/apisix/routes/routes.yaml` (nguồn duy nhất, gateway internal, `ip_allow: EW_SIS_WEBHOOK_CIDRS`, không OIDC, body 8 MiB, `sensitive`); `render --check`, `check-routes` 0 vấn đề, `deploy/apisix/scripts/sis-routes.test.mjs` 2/2

## Phase 8: User Story 6 — Adapter `lookupStudent` (Priority: P1)

### Tests cho User Story 6

- [x] T045 [P] [US6] Test F017-AC-22: lookup timeout `sis_login_sync_timeout_ms`, không retry, bỏ trường lạ + metric (TC-SIS-102, TC-SIS-107) — file: `src/backend/modules/identity/tests/integration/sis_lookup_adapter_test.go` — phải FAIL trước khi code — **Done 2026-11-19:** TestLookup (`internal/infrastructure/sis/lookup/contractv1_lookup_test.go`: 1 lời gọi, bỏ `parent_phone`, timeout < 900 ms), TestClientLookup (không retry)
- [x] T046 [P] [US6] Test F017-AC-23: không capability → `ErrUnsupported` 0 request; 404 → không tìm thấy; timeout/5xx → lỗi tạm thời (TC-SIS-104) — file: `src/backend/modules/identity/tests/unit/infrastructure/contractv1_lookup_test.go` — **Done 2026-11-19:** TestLookup (không capability / paused → `ErrSisLookupUnsupported` 0 request; 404 → `ErrSisLookupNotFound`; timeout/5xx → lỗi tạm thời)
- [x] T047 [P] [US6] Test F017-AC-24: bulkhead `sis_login_sync_concurrency`, breaker riêng connection (TC-SIS-106) — file: `src/backend/modules/identity/tests/integration/sis_lookup_bulkhead_test.go` — **Done 2026-11-19:** TestLookupBulkheadAndBreaker (concurrency 2: 2 ok/4 throttled; breaker 3 lỗi chỉ mở connection treo; connection kia < 200 ms)

### Implementation cho User Story 6

- [x] T048 [US6] `contractV1Lookup` hiện thực `SisLookupPort` (F011) + đăng ký theo `provider_kind` — file: `src/backend/modules/identity/internal/infrastructure/sis/lookup/contractv1_lookup.go`, `internal/application/port/write/sis_provider_factory.go` — **Done 2026-11-19:** `internal/infrastructure/sis/lookup/contractv1_lookup.go` (failsafe-go bulkhead + circuit breaker theo connection); `ConnectionLoader` thay `sis_provider_factory.go` (factory là `port.SisProviderFactory` ở `port/sis_contract.go`); login sync đọc cột `capabilities`

## Phase 9: User Story 7 — Conformance kit và `sis-mock` (Priority: P1)

### Tests cho User Story 7

- [x] T049 [P] [US7] Collection conformance F017-AC-25: test mọi endpoint bắt buộc (status, JSON Schema, cursor, `limit`, problem+json, 401) — file: `specs/postman/sis-conformance/sis-conformance.postman_collection.json` — phải FAIL trước khi `sis-mock` có `examples` (TC-AUTH-012) — **Done 2026-11-19:** `specs/postman/sis-conformance/generate.mjs` sinh collection từ contract (`--check` cho CI); `postman collection run` vào sis-mock: 13 request, 22/22 assertion pass; vào facade không có → 8 request bắt buộc fail, 5 tùy chọn bỏ qua
- [x] T050 [P] [US7] Thư mục capability tùy chọn F017-AC-26 (`listChanges`, `lookupStudent`, webhook mẫu ký HMAC; skip khi không khai báo) — file: `specs/postman/sis-conformance/sis-conformance.postman_collection.json` — **Done 2026-11-19:** thư mục "Tùy chọn theo capability" (`listChanges` + 410, `lookupStudent` 200/404, vector HMAC webhook), `pm.execution.skipRequest()` khi capability vắng
- [x] T051 [US7] Job CI F017-AC-27: dựng `sis-mock`, chạy Postman CLI khi đổi `specs/sis/**`, `specs/postman/sis-conformance/**` (TC-AUTH-012) — file: `.github/workflows/sis-conformance.yml` — **Done 2026-11-19:** `.github/workflows/sis-conformance.yml` (generate --check, Prism 4, Postman CLI); `scripts/ci/check-workflows.mjs` 0 lỗi

### Implementation cho User Story 7

- [x] T052 [P] [US7] Env `sis-mock` + env mẫu cho trường, README hướng dẫn trường (tiếng Việt) — file: `specs/postman/sis-conformance/env.sis-mock.postman_environment.json`, `env.template.postman_environment.json`, `README.md` — **Done 2026-11-19:** `env.sis-mock.postman_environment.json` (base `http://localhost:4010`: Prism phục vụ path không kèm `/eduworld/v1`), `env.template.postman_environment.json`, `README.md`
- [x] T053 [US7] Service `sis-mock` trong compose: lệnh Prism static, healthcheck, mount contract — file: `deploy/compose/docker-compose.yml` — **Done 2026-11-19:** service `sis-mock` có sẵn từ F001 (Prism 4 static, mount `../../specs`, healthcheck) — đã xác nhận chạy bằng `docker run stoplight/prism:4 mock …`; không đổi compose

## Phase 10: User Story 8 — Bảo vệ API (Priority: P1)

### Tests cho User Story 8

- [x] T054 [P] [US8] Test tenant isolation + authorization F017-AC-28 cho `createSisConnection` (contract_v1), `updateSisConnection`, `checkSisConnectionHealth`, `triggerSisSync`: ma trận role, trường/tenant khác → 404 (TC-TEN-001) — file: `src/backend/modules/identity/tests/integration/sis_contract_authz_test.go` — phải FAIL trước khi code — **Done 2026-11-19:** TestSisContractHTTP (sa.admin/ad.truong.a/ad.truong.b/qlcm.truong.a: tạo 403 cho tenant staff, trigger 202 trường mình, 404 tenant khác, 403 qlcm; health 403), TestSisContractConnection (school admin → PERMISSION_DENIED); `authztest.Cover`
- [x] T055 [P] [US8] Test F017-AC-29: thiếu `Idempotency-Key` → 428, lặp key trả cùng kết quả, audit actor `system:sis-sync:<id>` — file: `src/backend/modules/identity/tests/integration/sis_contract_idempotency_test.go` — **Done 2026-11-19:** TestSisContractHTTP (428 không key; replay cùng key trả cùng connection), TestSisContractPull (audit `sis.sync_run.completed` actor `system:sis-sync:<id>`)
- [x] T056 [P] [US8] Test F017-AC-30: `maxLength`/pattern/https → 422; route `/api/v1/sis/*` không có ở gateway public — file: `src/backend/modules/identity/tests/contract/sis_connection_validation_test.go`, `deploy/apisix/tests/sis_routes_test.sh` — **Done 2026-11-19:** TestSisContractHTTP (base_url 3000 ký tự, ftp → 422), tests/contract TestSisConnectionSchemas, `deploy/apisix/scripts/sis-routes.test.mjs` (thay `sis_routes_test.sh`)

### Implementation cho User Story 8

- [x] T057 [US8] Gọi tường minh `authz.Require(ctx, p, perm, resource)` ở đầu command handler, scope trường cho `tenant.sis.import` khi trigger — file: `src/backend/modules/identity/internal/application/command/sis_sync.go`, `internal/presentation/http/public/sis_connections_contract.go` — **Done 2026-11-19:** `SisSync.authorizeTrigger` (`authz.Require` platform hoặc `tenant.sis.import` với `SchoolID`), `platformTarget` cho connection ops

## Phase N: Polish & Cross-Cutting

- [x] T058 [P] Đăng ký tham số ngưỡng vi phạm, lịch retry, rate limit, ngưỡng circuit breaker; kiểm `sis_lookup_timeout` đã bỏ, chỉ còn `sis_login_sync_timeout_ms` (RB-111) — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/09-identity/SIS_INTEGRATION.md` — **Done 2026-11-19:** CONFIG_DEFAULTS: `sis_sync_schedule` chi tiết, `sis_webhook_retry`, `sis_run_lock_ttl`/`sis_run_stale_after`; ngưỡng vi phạm, retry, rate, circuit đã có; `sis_lookup_timeout` chỉ còn ở bảng xung đột C-8
- [x] T059 [P] Đồng bộ tên metric SIS giữa SIS_INTEGRATION §8 và MONITORING; dashboard SIS — file: `docs/11-operations/MONITORING.md` — **Done 2026-11-19:** MONITORING: metric F017 (`eduworld_sis_sync_duration_seconds`, `_http_requests_total`, `_webhook_total`, `_sync_runs_skipped_total`, nhãn `connection,entity` của unknown fields)
- [x] T060 [P] Cập nhật SIS_INTEGRATION §2.6 (conformance kit ở `specs/postman/sis-conformance/`), §10 (path `sync-runs`), CHANGELOG — file: `docs/09-identity/SIS_INTEGRATION.md`, `CHANGELOG.md` — **Done 2026-11-19:** SIS_INTEGRATION §2.6, §4.2, §7, §8, §10; CHANGELOG
- [x] T061 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng — **Done 2026-11-19:** bằng chứng ở [quickstart.md §5](quickstart.md); backend thật chưa nối binary (OQ F010-2) → kịch bản chạy bằng test tích hợp + Prism/Postman

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 (cần client US2) → US6 (cần client + capability US1) → US4 → US5 (dùng `ProcessChangeBatch` US4) → US7 (song song từ sau T006) → US8 (test authz viết sớm, chạy lại cuối) → Polish.
- Cần F011 Done: port `SISProvider`, `SisLookupPort`, validator, reconcile/apply, consumer `sis-jobs`, `createSisConnection`.

## Definition of Done

- [x] Mọi tiêu chí F017-AC-* có test pass (ghi mã test/TC) — xem quickstart.md §5
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (MOD=identity) và conformance Postman với `sis-mock` xanh
- [x] Constitution Check trong plan.md vẫn đạt; fake SIS chỉ trong test (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
