# Tasks: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Thêm dependency (ent, pgx/v5, nats.go jetstream, failsafe-go, oapi-codegen runtime, jsonschema, testcontainers, oauth2) — file: `src/backend/go.mod` (Go module duy nhất, ADR-0074, ADR-0092)
- [ ] T002 [P] `src/backend/pkg/testkit/containers.go`: khởi động postgres:17, nats:2.10-alpine `-js`, valkey:8; helper DSN theo role `_app/_platform/_migrator/_relay/_partition_admin/_readonly` — file: `src/backend/pkg/testkit/containers.go`
- [ ] T003 [P] Module mẫu `src/backend/pkg/testkit/sample` (ent schema `Widget`, migration Atlas) dùng làm đối tượng test của mọi story — file: `src/backend/pkg/testkit/sample/ent/schema/widget.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Thêm schema payload `audit.recorded` v1 và cập nhật bảng payload — file: `specs/events/payloads/common/audit.recorded.v1.schema.json` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); payload có thêm `school_id` tùy chọn (cột `audit.audit_logs.school_id`)
- [ ] T005 [P] Fixture envelope (chỉ trường cần để vô hiệu cache: `user_id`, `tenant_id`, `role_id?`) cho `tenant_membership.changed`, `user.access_revoked`, `enrollment.changed`, `tenant.updated` — **không** tạo schema payload (consumer chỉ tham chiếu: owner F010 cho `tenant_membership.changed`, `enrollment.changed`, `tenant.updated`; owner F011 cho `user.access_revoked`); contract test với schema của owner bật khi file có — file: `src/backend/pkg/natsx/testdata/events/` — kiểm: `go -C src/backend test ./pkg/natsx/...`
- [x] T006 [P] Khai báo header `Idempotent-Replayed` và gắn vào response 2xx của operation có `IdempotencyKey`; chạy `split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); header `components.headers.IdempotentReplayed` gắn vào 170 response 2xx của 168 operation
- [ ] T007 Schema ent bảng `infra` (`OutboxEvent`, `ProcessedEvent`, `DeadLetter`, `IdempotencyRecord`) theo data-model §1 — file: `src/backend/pkg/entx/infraschema/outbox_event.go`
- [ ] T008 Nhúng `infraschema` vào 5 module, sinh migration Atlas + `REVOKE DELETE, TRUNCATE ON infra.*`, role `<module>_relay`/`<module>_partition_admin` và `GRANT` theo ADR-0089 §4 (không bật RLS trên `infra.*`) — file: `src/backend/modules/identity/ent/schema/infra.go` — kiểm: `make migrate-diff MOD=identity`, `make migrate-lint MOD=identity`
- [ ] T066 Migration partition viết tay (ADR-0089 §6): `PARTITION BY RANGE` theo ngày cho `infra.outbox_events` (`created_at`), `infra.processed_events` (`event_occurred_at`), `infra.idempotency_records` (`key_issued_on`), `infra.dead_letters` (`created_at`) + partition 7 ngày đầu — file: `src/backend/modules/identity/migrations/partitions/` — kiểm: `make migrate-lint MOD=identity`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Repository ent an toàn tenant và chỉ soft-delete (Priority: P1) 🎯 MVP

**Goal**: Mixin, interceptor, hook soft-delete, unit of work, lint.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Test F003-AC-01: schema sinh từ mixin có đủ cột/kiểu (so migration Atlas của `Widget`) — file: `src/backend/pkg/entx/base_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Integration test F003-AC-02: query không scope → `ErrMissingTenantScope`; scope A không thấy dòng B (TC-TEN-021) — file: `src/backend/pkg/entx/interceptor_integration_test.go`
- [ ] T011 [P] [US1] Integration test F003-AC-03, F003-AC-04, F003-AC-05: `Delete()` thành update; entity không soft-delete → `ErrHardDeleteForbidden`; outbox có `<entity>.deleted` + audit; tạo lại cùng mã (TC-TEN-022, TC-TEN-015) — file: `src/backend/pkg/entx/softdelete_integration_test.go`
- [ ] T012 [P] [US1] Integration test F003-AC-06: `InTenantTx` set `app.tenant_id`, chọn client `_app`/`_platform`; truy vấn ngoài UoW lỗi (TC-TEN-005) — file: `src/backend/pkg/entx/uow_integration_test.go`
- [ ] T013 [P] [US1] Test F003-AC-07: fixture vi phạm (String thiếu MaxLen, schema không mixin hoặc hai mixin, policy RLS có `deleted_at`, SQL thiếu tenant/deleted_at, `DELETE FROM`, bảng tenant thiếu RLS) → `entlint`/`tenancyguard`/`mixin_coverage_test` fail có vị trí (TC-TEN-017, TC-TEN-024 phần DB) — file: `src/backend/tools/tenancyguard/guard_test.go`
- [ ] T067 [P] [US1] Integration test F003-AC-34: `DeleteWidget` qua `UnitOfWork` bằng role `_app` dưới RLS thành công; `ListWidgets` không còn dòng; `Bypass(ctx, "trash")` đọc lại được — file: `src/backend/pkg/entx/softdelete_rls_integration_test.go`

### Implementation cho User Story 1

- [ ] T014 [P] [US1] Mixin `Base`, `TenantAppendOnly`, `TenantOptional`, `HybridContent` (đổi tên từ `PlatformBase`), `GlobalBase`, `AppendOnly` + test `mixin_coverage_test.go` liệt kê mọi schema ent (ADR-0089 §2) — file: `src/backend/pkg/entx/base.go`
- [ ] T015 [US1] `TenantFilterInterceptor` theo mixin (bảng MULTITENANCY §4.4; `PlatformScope` chỉ `tenant_id IS NULL`; `GlobalBase`/`AppendOnly` không cần scope; bỏ qua khi `Bypassed`) — file: `src/backend/pkg/entx/interceptor.go`
- [ ] T016 [US1] `SoftDeleteHook`, `SoftDelete(ctx, e, reason)`, `Restore(ctx, e)` (chỉ khi `Bypassed` lý do `trash`) — file: `src/backend/pkg/entx/softdelete.go`
- [ ] T017 [US1] `UnitOfWork.InTenantTx` + chọn client theo scope — file: `src/backend/pkg/entx/uow.go`
- [ ] T018 [P] [US1] `src/backend/tools/entlint` (MaxLen, annotation `maxlen-justified`) — file: `src/backend/tools/entlint/main.go`
- [ ] T019 [P] [US1] `src/backend/tools/tenancyguard` (5 kiểm tra MULTITENANCY §11), thêm vào `make lint` và CI — file: `src/backend/tools/tenancyguard/main.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Command/query tách write/read context, không mediator (Priority: P1)

**Goal**: `UnitOfWork.Do` + `ReadSession`, `observability.UseCase`, rule linter read/write, module mẫu theo khuôn ADR-0087 (không `src/backend/pkg/mediator`).

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [ ] T020 [P] [US2] Integration test F003-AC-08: `UnitOfWork.Do` commit khi nil, rollback khi lỗi/panic; repository và `outbox.Writer` dùng cùng tx từ context; `Do` lồng dùng lại tx ngoài — file: `src/backend/pkg/entx/uow_do_integration_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Integration test F003-AC-09: `CreateWidget` lỗi sau khi ghi outbox → rollback, outbox trống; `ListWidgets` chạy với role chỉ đọc; adapter read ghi trong `ReadSession` → lỗi (TC-SVC-001) — file: `src/backend/pkg/testkit/sample/cqrs_integration_test.go`
- [ ] T022 [P] [US2] Test F003-AC-10: fixture vi phạm (query import `port/write`, `src/backend/pkg/entx`, `src/backend/pkg/outbox`, `application/command`; `persistence/read` import `src/backend/pkg/outbox`) → `go-arch-lint`/`depguard` fail có vị trí — file: `src/backend/tools/lint/testdata/badquery/internal/application/query/bad.go`
- [ ] T023 [P] [US2] Integration test F003-AC-12: `Save` version cũ → 409 `CONFLICT` — file: `src/backend/pkg/testkit/sample/repo_integration_test.go`
- [ ] T060 [P] [US2] Test F003-AC-11: tên widget vượt MaxLen → 422 `MAX_LENGTH`, `UnitOfWork` (spy) không được gọi; thiếu permission → lỗi `authz.Require`, repository không được gọi — file: `src/backend/pkg/testkit/sample/internal/application/command/create_widget_test.go`
- [ ] T061 [P] [US2] Test F003-AC-33: exporter in-memory có span con `sample.command.CreateWidget`, metric `eduworld_usecase_duration_seconds` đủ nhãn, log không chứa payload — file: `src/backend/pkg/observability/usecase_test.go`

### Implementation cho User Story 2

- [ ] T024 [US2] `UnitOfWork.Do` (scope từ context, tx trong context, `Publisher()` trả `outbox.Writer` gắn tx) và `ReadSession.InTenantRead` (tx `READ ONLY`, client đọc) — file: `src/backend/pkg/entx/readsession.go`
- [ ] T025 [US2] `observability.UseCase(ctx, name) (context.Context, func(error))` — span, metric, log kết quả theo `problem.Kind` — file: `src/backend/pkg/observability/usecase.go`
- [ ] T062 [US2] Thêm rule read/write context vào cấu hình lint nền (go-arch-lint components `command`/`query`/`port-write`/`port-read`/`persist-write`/`persist-read`; depguard cho `query`, `persistence/read`) theo BACKEND_DESIGN §3 — file: `src/backend/tools/lint/go-arch-lint.base.yml` — kiểm: `make lint MOD=pkg`
- [ ] T063 [US2] Module mẫu theo khuôn: command `CreateWidget`/`DeleteWidget`, query `ListWidgets`, port `write`/`read`, persistence `write`/`read`, HTTP handler giữ interface có kiểu, nối ở composition root — file: `src/backend/pkg/testkit/sample/internal/application/command/create_widget.go`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Idempotency cho thao tác quan trọng (Priority: P1)

**Goal**: Middleware `httpx.Idempotency` + store idempotency.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [ ] T026 [P] [US3] Integration test F003-AC-13, F003-AC-14: bản ghi `INSERT` cùng tx nghiệp vụ; replay + header; khác payload 409; hai request song song → request sau chờ rồi replay (không chạy nghiệp vụ hai lần), vượt `lock_timeout` → 409 + `Retry-After`; thiếu key 428 — file: `src/backend/pkg/idempotency/store_integration_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US3] Integration test F003-AC-15: command lỗi (4xx/5xx) rollback → không có bản ghi, retry cùng key được xử lý; bản ghi hết hạn bị bỏ qua, không có lệnh DELETE — file: `src/backend/pkg/idempotency/expiry_integration_test.go`
- [ ] T072 [P] [US3] Test F003-AC-37: key UUIDv4/chuỗi lạ → 400 trước khi mở tx; key UUIDv7 lệch quá `idempotency_record_ttl` → 400; key tạo 23:59 UTC, retry 00:01 → replay (cùng `key_issued_on`) — file: `src/backend/pkg/idempotency/key_test.go`
- [ ] T028 [P] [US3] Integration test F003-AC-16: user B cùng key/route không nhận response của A — file: `src/backend/pkg/idempotency/isolation_integration_test.go`

### Implementation cho User Story 3

- [ ] T029 [US3] Store (ent `IdempotencyRecord`, hash SHA-256 body chuẩn hóa, `Pending` trong context, `INSERT` trong tx của `UnitOfWork.Do` trước commit, xử lý lỗi unique → đọc và replay; không khóa Valkey) + `key.go` (UUIDv7, `key_issued_on`, kiểm lệch giờ) — file: `src/backend/pkg/idempotency/store.go`
- [ ] T030 [US3] Middleware `httpx.Idempotency` (chỉ operation khai báo `IdempotencyKey` trong spec): kiểm key, đọc bản ghi để replay/409, đặt `Pending`, ghi `Idempotent-Replayed`, đọc `idempotency_record_ttl` — file: `src/backend/pkg/httpx/idempotency.go`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Event không mất qua outbox → NATS, consumer idempotent, audit tập trung (Priority: P1)

**Goal**: Writer, Relay, Consumer, DLQ, AuditRecorder, PrincipalInvalidator.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [ ] T031 [P] [US4] Test F003-AC-17: Writer ghi trong tx, envelope validate schema, có `traceparent` — file: `src/backend/pkg/outbox/writer_test.go` — phải FAIL trước khi code
- [ ] T032 [P] [US4] Integration test F003-AC-18: giết relay giữa chừng, chạy lại, không trùng (TC-SVC-002) — file: `src/backend/pkg/outbox/relay_integration_test.go`
- [ ] T033 [P] [US4] Integration test F003-AC-19: giao trùng → xử lý một lần; `processed_events.event_occurred_at` = `occurred_at` của envelope (TC-SVC-003) — file: `src/backend/pkg/outbox/consumer_integration_test.go`
- [ ] T073 [P] [US4] Integration test F003-AC-38: event `occurred_at` 23:59 UTC xử lý lúc 23:59, giao lại lúc 00:05 hôm sau (clock giả) → bỏ qua, ack, nghiệp vụ chạy đúng một lần — file: `src/backend/pkg/outbox/consumer_cross_midnight_integration_test.go`
- [ ] T034 [P] [US4] Integration test F003-AC-20: scope từ envelope; event platform chỉ consumer platform; tham chiếu tenant khác → DLQ — file: `src/backend/pkg/outbox/consumer_tenant_integration_test.go`
- [ ] T035 [P] [US4] Integration test F003-AC-21: quá ngưỡng giao/sai schema → DLQ subject + `infra.dead_letters` + Term — file: `src/backend/pkg/outbox/dlq_integration_test.go`
- [ ] T036 [P] [US4] Test F003-AC-22: payload `audit.recorded` validate schema, không khóa PII — file: `src/backend/pkg/outbox/auditrecorder_test.go`
- [ ] T037 [P] [US4] Integration test F003-AC-23: publish từng event invalidate → cache Principal bị xóa — file: `src/backend/pkg/natsx/principal_invalidator_integration_test.go`
- [ ] T068 [P] [US4] Integration test F003-AC-35: `infra.*` không bật RLS; Writer bằng `_app` chỉ INSERT; Relay bằng `_relay` chỉ `SELECT, UPDATE (published_at, attempts)`; dead letters `_app` chỉ INSERT, `_platform` chỉ cập nhật cột xử lý; `DELETE`/`TRUNCATE` trên `infra.*` bằng mọi role ứng dụng → lỗi quyền — file: `src/backend/pkg/outbox/grants_integration_test.go`
- [ ] T069 [P] [US4] Integration test F003-AC-36: job tạo partition trước 7 ngày cho bốn bảng; gỡ partition quá `outbox_retention`/`idempotency_record_ttl`/retention dead letter; partition còn outbox chưa publish hoặc dead letter chưa xử lý → không gỡ + cảnh báo; thiếu partition → cảnh báo; không chạm bảng nghiệp vụ — file: `src/backend/pkg/entx/partitionmaint/job_integration_test.go`
- [ ] T075 [P] [US4] Integration test F003-AC-39: module mẫu có projection `ref.*` lệch nguồn → `worker projection rebuild --module=<m> --source=<n>` khớp lại; chạy lại không đổi; bản `source_version` mới hơn không bị ghi đè; facade lỗi → exit ≠ 0, projection giữ nguyên (TC-SVC-014) — file: `src/backend/pkg/projection/rebuild_integration_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T038 [P] [US4] `natsx`: kết nối, `CreateOrUpdateStream` `EVT_<SVC>`/`DLQ_<SVC>` (`event_stream_retention`, `jetstream_duplicate_window`), header `traceparent`/`Ew-Tenant-Id` — file: `src/backend/pkg/natsx/streams.go`
- [ ] T039 [US4] `outbox.Writer` + envelope builder (producer `<module>/<bounded-context>`, `idempotency_key`) — file: `src/backend/pkg/outbox/writer.go`
- [ ] T040 [US4] `outbox.Relay` (role `<module>_relay`, `FOR UPDATE SKIP LOCKED`, lô 500, poll 200 ms + `LISTEN ew_outbox`) — file: `src/backend/pkg/outbox/relay.go`
- [ ] T070 [US4] Job `infra-partition-maintenance` (role `<module>_partition_admin`, chạy trong `src/backend/cmd/worker`) — file: `src/backend/pkg/entx/partitionmaint/job.go`
- [ ] T041 [US4] `outbox.Consumer` (durable, BackOff, processed_events, scope từ envelope, DLQ) — file: `src/backend/pkg/outbox/consumer.go`
- [ ] T042 [US4] `outbox.AuditRecorder` hiện thực `audit.Recorder` — file: `src/backend/pkg/outbox/auditrecorder.go`
- [ ] T043 [US4] `natsx.PrincipalInvalidator` gọi `authz.Cache.Invalidate` — file: `src/backend/pkg/natsx/principal_invalidator.go`
- [ ] T076 [US4] Khung `src/backend/pkg/projection` (đăng ký bộ rebuild theo bảng `ref.*`, đọc trang facade qua `src/backend/pkg/modcall`, upsert theo `source_version`) + lệnh con `projection rebuild` của `src/backend/cmd/worker` (BACKEND_DESIGN §3, BACKUP_RECOVERY §3.5) — file: `src/backend/pkg/projection/rebuild.go`, `src/backend/cmd/worker/projection.go`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Gọi module khác an toàn qua facade trong tiến trình (Priority: P2)

**Goal**: `modcall` + adapter `PrincipalResolver`.

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [ ] T044 [P] [US5] Test F003-AC-24, F003-AC-25: span + metric + deadline theo `Kind`; scope/Principal gốc/caller/`Idempotency-Key` có trong context bên nhận; thiếu `tenant_scope`/`idempotency_key` khi `x-context-required` → 500 không gọi facade; caller ngoài allowlist → `403 CALLER_NOT_ALLOWED` (TC-SVC-006) — file: `src/backend/pkg/modcall/call_test.go` — phải FAIL trước khi code
- [ ] T045 [P] [US5] Test F003-AC-26, F003-AC-27: facade giả chậm quá deadline → 503 `UNAVAILABLE` sau đúng một lần gọi (không retry); facade panic → 500; Problem 4xx giữ `code`; không có policy circuit/bulkhead nào được đăng ký (TC-SVC-011) — file: `src/backend/pkg/modcall/errors_test.go`
- [ ] T046 [P] [US5] Test F003-AC-28: adapter `PrincipalResolver` với facade identity giả: thành công → Principal, không membership → `ErrNotMember`, deadline → `UNAVAILABLE` — file: `src/backend/cmd/internal/wiring/principal_resolver_test.go`

### Implementation cho User Story 5

- [ ] T047 [US5] `modcall.Call` (span, deadline theo `module_call_deadline`, kiểm `x-context-required`/`x-allowed-callers`, context on-behalf, recover, `problem.Map`, metric API_DESIGN §12.3) — file: `src/backend/pkg/modcall/call.go`
- [ ] T048 [US5] Adapter `authz.PrincipalResolver` bọc facade `identity` (`resolvePrincipal`) qua `modcall`, nối ở composition root `core-api`/`worker` — file: `src/backend/cmd/internal/wiring/principal_resolver.go`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase 8: User Story 6 — Dừng có trật tự, không mất việc (Priority: P2)

**Goal**: `src/backend/pkg/lifecycle`.

**Independent Test**: Như spec.md User Story 6.

### Tests cho User Story 6

- [ ] T049 [P] [US6] Integration test F003-AC-29: SIGTERM khi request đang chạy → readiness 503, request hoàn tất, server đóng — file: `src/backend/pkg/lifecycle/api_shutdown_integration_test.go` — phải FAIL trước khi code
- [ ] T050 [P] [US6] Integration test F003-AC-30: SIGTERM worker giữa lô → ack phần xong, phần còn lại giao lại, outbox relay lần sau (TC-SVC-002) — file: `src/backend/pkg/lifecycle/worker_shutdown_integration_test.go`

### Implementation cho User Story 6

- [ ] T051 [US6] `lifecycle.RunAPI`/`RunWorker` theo BACKEND_DESIGN §13 (`EW_HTTP_SHUTDOWN_TIMEOUT`), gắn vào `src/backend/cmd/core-api`, `src/backend/cmd/realtime`, `src/backend/cmd/worker` — file: `src/backend/pkg/lifecycle/shutdown.go`

**Checkpoint**: User Story 6 chạy và kiểm độc lập được.

## Phase 9: User Story 7 — Testkit và contract-first (Priority: P2)

**Goal**: Fixture + contract helper + oasdiff.

**Independent Test**: Như spec.md User Story 7.

### Tests cho User Story 7

- [ ] T052 [P] [US7] Test F003-AC-31: testkit dựng đủ container, migrate module mẫu, fixture tenant A/B, context scope — file: `src/backend/pkg/testkit/testkit_integration_test.go` — phải FAIL trước khi code
- [ ] T053 [P] [US7] Test F003-AC-32: contract helper phát hiện response lệch spec public, response object facade lệch ví dụ spec internal và payload event lệch schema; fake facade sinh được cho bên gọi; `make specs-breaking` fail với thay đổi breaking mẫu (TC-SVC-015) — file: `src/backend/pkg/testkit/contract/openapi_test.go`

### Implementation cho User Story 7

- [ ] T054 [US7] `testkit.New(t, module)`, `Fixtures` (tenant A/B, Principal theo role từ `authztest`), `migrate.go` — file: `src/backend/pkg/testkit/fixtures.go`
- [ ] T055 [US7] `testkit/contract` (kin-openapi validator public, kiểm facade theo ví dụ spec internal, fake facade cho bên gọi, jsonschema cho event; target `make test-contract`) + target `make specs-breaking` (oasdiff spec gộp + mọi internal) — file: `src/backend/pkg/testkit/contract/openapi.go`

**Checkpoint**: User Story 7 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [ ] T056 [P] Kiểm DATABASE_DESIGN §3.9–3.10 (DDL `infra.dead_letters`, CHECK kích thước `idempotency_records` đã có theo ADR-0089) khớp ent `infraschema`; API_DESIGN §6 đổi `platform.idempotency_records` → `infra.idempotency_records` (D6) nếu còn sót — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T074 [P] Delta contract: mô tả `Idempotency-Key` là UUIDv7 (`format: uuid` + mô tả, `400 BAD_REQUEST` khi sai) ở parameter dùng chung của spec gộp rồi chạy `split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T057 [P] BACKEND_DESIGN §5.5 thay `CONCURRENT_MODIFICATION` bằng `CONFLICT`; §11 và TC-SVC-004 dùng tên metric `eduworld_internal_client_*` — file: `docs/07-backend/BACKEND_DESIGN.md`
- [ ] T058 [P] CHANGELOG mục F003 — file: `CHANGELOG.md`
- [ ] T064 [P] Đăng ký metric `eduworld_usecase_duration_seconds` ở MONITORING; thêm biến `EW_DB_READ_URL_FILE` (client đọc của `ReadSession`) vào BACKEND_DESIGN §12 — file: `docs/11-operations/MONITORING.md`
- [ ] T065 [P] Sửa mô tả TC-SVC-001 (bỏ chữ "Mediator CQRS", dùng write/read context theo ADR-0087) — file: `docs/12-project/TEST_STRATEGY.md`
- [ ] T071 [P] Đăng ký cảnh báo P2 "thiếu partition `infra`" và metric của job `infra-partition-maintenance` ở MONITORING — file: `docs/11-operations/MONITORING.md`
- [ ] T059 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/003-platform-kit-data/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 (cần `InTenantTx` của T017) → US3, US4 (song song, cần US2) → US5 → US6 → US7 → Polish.
- T062 cần cấu hình lint nền của F002 (T003); T063 cần T003, T024, T025.
- T004, T005, T006 trước T036, T037, T030; T007 trước T008, T029, T039; T066 sau T008, trước T069, T070.
- Mọi feature module M-C1 (F010…F015) chỉ bắt đầu khi F003 Done.

## Definition of Done

- [ ] Mọi tiêu chí F003-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
