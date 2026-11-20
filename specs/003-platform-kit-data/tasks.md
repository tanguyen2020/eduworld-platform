# Tasks: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Thêm dependency (ent, pgx/v5, nats.go jetstream, failsafe-go, oapi-codegen runtime, jsonschema, testcontainers, oauth2) — file: `src/backend/go.mod` (Go module duy nhất, ADR-0074, ADR-0092)
- [x] T002 [P] `src/backend/pkg/testkit/containers.go`: khởi động postgres:17, nats:2.10-alpine `-js`, valkey:8; helper DSN theo role `_app/_platform/_migrator/_relay/_partition_admin/_readonly` — file: `src/backend/pkg/testkit/containers.go`
- [x] T003 [P] Module mẫu `src/backend/pkg/testkit/sample` (ent schema `Widget`, migration Atlas) dùng làm đối tượng test của mọi story — file: `src/backend/pkg/testkit/sample/ent/schema/widget.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Thêm schema payload `audit.recorded` v1 và cập nhật bảng payload — file: `specs/events/payloads/common/audit.recorded.v1.schema.json` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); payload có thêm `school_id` tùy chọn (cột `audit.audit_logs.school_id`)
- [x] T005 [P] Fixture envelope (chỉ trường cần để vô hiệu cache: `user_id`, `tenant_id`, `role_id?`) cho `tenant_membership.changed`, `user.access_revoked`, `enrollment.changed`, `tenant.updated` — **không** tạo schema payload (consumer chỉ tham chiếu: owner F010 cho `tenant_membership.changed`, `enrollment.changed`, `tenant.updated`; owner F011 cho `user.access_revoked`); contract test với schema của owner bật khi file có — file: `src/backend/pkg/natsx/testdata/events/` — kiểm: `go -C src/backend test ./pkg/natsx/...`
- [x] T006 [P] Khai báo header `Idempotent-Replayed` và gắn vào response 2xx của operation có `IdempotencyKey`; chạy `split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); header `components.headers.IdempotentReplayed` gắn vào 170 response 2xx của 168 operation
- [x] T007 Schema ent bảng `infra` (`OutboxEvent`, `ProcessedEvent`, `DeadLetter`, `IdempotencyRecord`) theo data-model §1 — file: `src/backend/pkg/entx/infraschema/outbox_event.go`
- [x] T008 Nhúng `infraschema` vào 5 module, sinh migration Atlas + `REVOKE DELETE, TRUNCATE ON infra.*`, role `<module>_relay`/`<module>_partition_admin` và `GRANT` theo ADR-0089 §4 (không bật RLS trên `infra.*`) — file: `src/backend/modules/identity/ent/schema/infra.go` — kiểm: `make migrate-diff MOD=identity`, `make migrate-lint MOD=identity`
- [x] T066 Migration partition viết tay (ADR-0089 §6): `PARTITION BY RANGE` theo ngày cho `infra.outbox_events` (`created_at`), `infra.processed_events` (`event_occurred_at`), `infra.idempotency_records` (`key_issued_on`), `infra.dead_letters` (`created_at`) + partition 7 ngày đầu — file: `src/backend/modules/identity/migrations/partitions/` — kiểm: `make migrate-lint MOD=identity`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Repository ent an toàn tenant và chỉ soft-delete (Priority: P1) 🎯 MVP

**Goal**: Mixin, interceptor, hook soft-delete, unit of work, lint.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Test F003-AC-01: schema sinh từ mixin có đủ cột/kiểu (so migration Atlas của `Widget`) — file: `src/backend/pkg/entx/base_test.go` — phải FAIL trước khi code
- [x] T010 [P] [US1] Integration test F003-AC-02: query không scope → `ErrMissingTenantScope`; scope A không thấy dòng B (TC-TEN-021) — file: `src/backend/pkg/entx/interceptor_integration_test.go`
- [x] T011 [P] [US1] Integration test F003-AC-03, F003-AC-04, F003-AC-05: `Delete()` thành update; entity không soft-delete → `ErrHardDeleteForbidden`; outbox có `<entity>.deleted` + audit; tạo lại cùng mã (TC-TEN-022, TC-TEN-015) — file: `src/backend/pkg/entx/softdelete_integration_test.go`
- [x] T012 [P] [US1] Integration test F003-AC-06: `InTenantTx` set `app.tenant_id`, chọn client `_app`/`_platform`; truy vấn ngoài UoW lỗi (TC-TEN-005) — file: `src/backend/pkg/entx/uow_integration_test.go`
- [x] T013 [P] [US1] Test F003-AC-07: fixture vi phạm (String thiếu MaxLen, schema không mixin hoặc hai mixin, policy RLS có `deleted_at`, SQL thiếu tenant/deleted_at, `DELETE FROM`, bảng tenant thiếu RLS) → `entlint`/`tenancyguard`/`mixin_coverage_test` fail có vị trí (TC-TEN-017, TC-TEN-024 phần DB) — file: `src/backend/tools/tenancyguard/guard_test.go`
- [x] T067 [P] [US1] Integration test F003-AC-34: `DeleteWidget` qua `UnitOfWork` bằng role `_app` dưới RLS thành công; `ListWidgets` không còn dòng; `Bypass(ctx, "trash")` đọc lại được — file: `src/backend/pkg/entx/softdelete_rls_integration_test.go`

### Implementation cho User Story 1

- [x] T014 [P] [US1] Mixin `Base`, `TenantAppendOnly`, `TenantOptional`, `HybridContent` (đổi tên từ `PlatformBase`), `GlobalBase`, `AppendOnly` + test `mixin_coverage_test.go` liệt kê mọi schema ent (ADR-0089 §2) — file: `src/backend/pkg/entx/base.go`
- [x] T015 [US1] `TenantFilterInterceptor` theo mixin (bảng MULTITENANCY §4.4; `PlatformScope` chỉ `tenant_id IS NULL`; `GlobalBase`/`AppendOnly` không cần scope; bỏ qua khi `Bypassed`) — file: `src/backend/pkg/entx/interceptor.go`
- [x] T016 [US1] `SoftDeleteHook`, `SoftDelete(ctx, e, reason)`, `Restore(ctx, e)` (chỉ khi `Bypassed` lý do `trash`) — file: `src/backend/pkg/entx/softdelete.go`
- [x] T017 [US1] `UnitOfWork.InTenantTx` + chọn client theo scope — file: `src/backend/pkg/entx/uow.go`
- [x] T018 [P] [US1] `src/backend/tools/entlint` (MaxLen, annotation `maxlen-justified`) — file: `src/backend/tools/entlint/main.go`
- [x] T019 [P] [US1] `src/backend/tools/tenancyguard` (5 kiểm tra MULTITENANCY §11), thêm vào `make lint` và CI — file: `src/backend/tools/tenancyguard/main.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Command/query tách write/read context, không mediator (Priority: P1)

**Goal**: `UnitOfWork.Do` + `ReadSession`, `observability.UseCase`, rule linter read/write, module mẫu theo khuôn ADR-0087 (không `src/backend/pkg/mediator`).

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T020 [P] [US2] Integration test F003-AC-08: `UnitOfWork.Do` commit khi nil, rollback khi lỗi/panic; repository và `outbox.Writer` dùng cùng tx từ context; `Do` lồng dùng lại tx ngoài — file: `src/backend/pkg/entx/uow_do_integration_test.go` — phải FAIL trước khi code
- [x] T021 [P] [US2] Integration test F003-AC-09: `CreateWidget` lỗi sau khi ghi outbox → rollback, outbox trống; `ListWidgets` chạy với role chỉ đọc; adapter read ghi trong `ReadSession` → lỗi (TC-SVC-001) — file: `src/backend/pkg/testkit/sample/cqrs_integration_test.go`
- [x] T022 [P] [US2] Test F003-AC-10: fixture vi phạm (query import `port/write`, `src/backend/pkg/entx`, `src/backend/pkg/outbox`, `application/command`; `persistence/read` import `src/backend/pkg/outbox`) → `go-arch-lint`/`depguard` fail có vị trí — file: `src/backend/tools/lint/testdata/badquery/internal/application/query/bad.go`
- [x] T023 [P] [US2] Integration test F003-AC-12: `Save` version cũ → 409 `CONFLICT` — file: `src/backend/pkg/testkit/sample/repo_integration_test.go`
- [x] T060 [P] [US2] Test F003-AC-11: tên widget vượt MaxLen → 422 `MAX_LENGTH`, `UnitOfWork` (spy) không được gọi; thiếu permission → lỗi `authz.Require`, repository không được gọi — file: `src/backend/pkg/testkit/sample/internal/application/command/create_widget_test.go`
- [x] T061 [P] [US2] Test F003-AC-33: exporter in-memory có span con `sample.command.CreateWidget`, metric `eduworld_usecase_duration_seconds` đủ nhãn, log không chứa payload — file: `src/backend/pkg/observability/usecase_test.go`

### Implementation cho User Story 2

- [x] T024 [US2] `UnitOfWork.Do` (scope từ context, tx trong context, `Publisher()` trả `outbox.Writer` gắn tx) và `ReadSession.InTenantRead` (tx `READ ONLY`, client đọc) — file: `src/backend/pkg/entx/readsession.go`
- [x] T025 [US2] `observability.UseCase(ctx, name) (context.Context, func(error))` — span, metric, log kết quả theo `problem.Kind` — file: `src/backend/pkg/observability/usecase.go`
- [x] T062 [US2] Thêm rule read/write context vào cấu hình lint nền (go-arch-lint components `command`/`query`/`port-write`/`port-read`/`persist-write`/`persist-read`; depguard cho `query`, `persistence/read`) theo BACKEND_DESIGN §3 — file: `src/backend/tools/lint/go-arch-lint.base.yml` — kiểm: `make lint MOD=pkg`
- [x] T063 [US2] Module mẫu theo khuôn: command `CreateWidget`/`DeleteWidget`, query `ListWidgets`, port `write`/`read`, persistence `write`/`read`, HTTP handler giữ interface có kiểu, nối ở composition root — file: `src/backend/pkg/testkit/sample/internal/application/command/create_widget.go`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Idempotency cho thao tác quan trọng (Priority: P1)

**Goal**: Middleware `httpx.Idempotency` + store idempotency.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T026 [P] [US3] Integration test F003-AC-13, F003-AC-14: bản ghi `INSERT` cùng tx nghiệp vụ; replay + header; khác payload 409; hai request song song → request sau chờ rồi replay (không chạy nghiệp vụ hai lần), vượt `lock_timeout` → 409 + `Retry-After`; thiếu key 428 — file: `src/backend/pkg/idempotency/store_integration_test.go` — phải FAIL trước khi code
- [x] T027 [P] [US3] Integration test F003-AC-15: command lỗi (4xx/5xx) rollback → không có bản ghi, retry cùng key được xử lý; bản ghi hết hạn bị bỏ qua, không có lệnh DELETE — file: `src/backend/pkg/idempotency/expiry_integration_test.go`
- [x] T072 [P] [US3] Test F003-AC-37: key UUIDv4/chuỗi lạ → 400 trước khi mở tx; key UUIDv7 lệch quá `idempotency_record_ttl` → 400; key tạo 23:59 UTC, retry 00:01 → replay (cùng `key_issued_on`) — file: `src/backend/pkg/idempotency/key_test.go`
- [x] T028 [P] [US3] Integration test F003-AC-16: user B cùng key/route không nhận response của A — file: `src/backend/pkg/idempotency/isolation_integration_test.go`

### Implementation cho User Story 3

- [x] T029 [US3] Store (ent `IdempotencyRecord`, hash SHA-256 body chuẩn hóa, `Pending` trong context, `INSERT` trong tx của `UnitOfWork.Do` trước commit, xử lý lỗi unique → đọc và replay; không khóa Valkey) + `key.go` (UUIDv7, `key_issued_on`, kiểm lệch giờ) — file: `src/backend/pkg/idempotency/store.go`
- [x] T030 [US3] Middleware `httpx.Idempotency` (chỉ operation khai báo `IdempotencyKey` trong spec): kiểm key, đọc bản ghi để replay/409, đặt `Pending`, ghi `Idempotent-Replayed`, đọc `idempotency_record_ttl` — file: `src/backend/pkg/httpx/idempotency.go`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Event không mất qua outbox → NATS, consumer idempotent, audit tập trung (Priority: P1)

**Goal**: Writer, Relay, Consumer, DLQ, AuditRecorder, PrincipalInvalidator.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T031 [P] [US4] Test F003-AC-17: Writer ghi trong tx, envelope validate schema, có `traceparent` — file: `src/backend/pkg/outbox/writer_test.go` — phải FAIL trước khi code
- [x] T032 [P] [US4] Integration test F003-AC-18: giết relay giữa chừng, chạy lại, không trùng (TC-SVC-002) — file: `src/backend/pkg/outbox/relay_integration_test.go`
- [x] T033 [P] [US4] Integration test F003-AC-19: giao trùng → xử lý một lần; `processed_events.event_occurred_at` = `occurred_at` của envelope (TC-SVC-003) — file: `src/backend/pkg/outbox/consumer_integration_test.go`
- [x] T073 [P] [US4] Integration test F003-AC-38: event `occurred_at` 23:59 UTC xử lý lúc 23:59, giao lại lúc 00:05 hôm sau (clock giả) → bỏ qua, ack, nghiệp vụ chạy đúng một lần — file: `src/backend/pkg/outbox/consumer_cross_midnight_integration_test.go`
- [x] T034 [P] [US4] Integration test F003-AC-20: scope từ envelope; event platform chỉ consumer platform; tham chiếu tenant khác → DLQ — file: `src/backend/pkg/outbox/consumer_tenant_integration_test.go`
- [x] T035 [P] [US4] Integration test F003-AC-21: quá ngưỡng giao/sai schema → DLQ subject + `infra.dead_letters` + Term — file: `src/backend/pkg/outbox/dlq_integration_test.go`
- [x] T036 [P] [US4] Test F003-AC-22: payload `audit.recorded` validate schema, không khóa PII — file: `src/backend/pkg/outbox/auditrecorder_test.go`
- [x] T037 [P] [US4] Integration test F003-AC-23: publish từng event invalidate → cache Principal bị xóa — file: `src/backend/pkg/natsx/principal_invalidator_integration_test.go`
- [x] T068 [P] [US4] Integration test F003-AC-35: `infra.*` không bật RLS; Writer bằng `_app` chỉ INSERT; Relay bằng `_relay` chỉ `SELECT, UPDATE (published_at, attempts)`; dead letters `_app` chỉ INSERT, `_platform` chỉ cập nhật cột xử lý; `DELETE`/`TRUNCATE` trên `infra.*` bằng mọi role ứng dụng → lỗi quyền — file: `src/backend/pkg/outbox/grants_integration_test.go`
- [x] T069 [P] [US4] Integration test F003-AC-36: job tạo partition trước 7 ngày cho bốn bảng; gỡ partition quá `outbox_retention`/`idempotency_record_ttl`/retention dead letter; partition còn outbox chưa publish hoặc dead letter chưa xử lý → không gỡ + cảnh báo; thiếu partition → cảnh báo; không chạm bảng nghiệp vụ — file: `src/backend/pkg/entx/partitionmaint/job_integration_test.go`
- [x] T075 [P] [US4] Integration test F003-AC-39: module mẫu có projection `ref.*` lệch nguồn → `worker projection rebuild --module=<m> --source=<n>` khớp lại; chạy lại không đổi; bản `source_version` mới hơn không bị ghi đè; facade lỗi → exit ≠ 0, projection giữ nguyên (TC-SVC-014) — file: `src/backend/pkg/projection/rebuild_integration_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [x] T038 [P] [US4] `natsx`: kết nối, `CreateOrUpdateStream` `EVT_<SVC>`/`DLQ_<SVC>` (`event_stream_retention`, `jetstream_duplicate_window`), header `traceparent`/`Ew-Tenant-Id` — file: `src/backend/pkg/natsx/streams.go`
- [x] T039 [US4] `outbox.Writer` + envelope builder (producer `<module>/<bounded-context>`, `idempotency_key`) — file: `src/backend/pkg/outbox/writer.go`
- [x] T040 [US4] `outbox.Relay` (role `<module>_relay`, `FOR UPDATE SKIP LOCKED`, lô 500, poll 200 ms + `LISTEN ew_outbox`) — file: `src/backend/pkg/outbox/relay.go`
- [x] T070 [US4] Job `infra-partition-maintenance` (role `<module>_partition_admin`, chạy trong `src/backend/cmd/worker`) — file: `src/backend/pkg/entx/partitionmaint/job.go`
- [x] T041 [US4] `outbox.Consumer` (durable, BackOff, processed_events, scope từ envelope, DLQ) — file: `src/backend/pkg/outbox/consumer.go`
- [x] T042 [US4] `outbox.AuditRecorder` hiện thực `audit.Recorder` — file: `src/backend/pkg/outbox/auditrecorder.go`
- [x] T043 [US4] `natsx.PrincipalInvalidator` gọi `authz.Cache.Invalidate` — file: `src/backend/pkg/natsx/principal_invalidator.go`
- [x] T076 [US4] Khung `src/backend/pkg/projection` (đăng ký bộ rebuild theo bảng `ref.*`, đọc trang facade qua `src/backend/pkg/modcall`, upsert theo `source_version`) + lệnh con `projection rebuild` của `src/backend/cmd/worker` (BACKEND_DESIGN §3, BACKUP_RECOVERY §3.5) — file: `src/backend/pkg/projection/rebuild.go`, `src/backend/cmd/worker/projection.go`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Gọi module khác an toàn qua facade trong tiến trình (Priority: P2)

**Goal**: `modcall` + adapter `PrincipalResolver`.

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [x] T044 [P] [US5] Test F003-AC-24, F003-AC-25: span + metric + deadline theo `Kind`; scope/Principal gốc/caller/`Idempotency-Key` có trong context bên nhận; thiếu `tenant_scope`/`idempotency_key` khi `x-context-required` → 500 không gọi facade; caller ngoài allowlist → `403 CALLER_NOT_ALLOWED` (TC-SVC-006) — file: `src/backend/pkg/modcall/call_test.go` — phải FAIL trước khi code
- [x] T045 [P] [US5] Test F003-AC-26, F003-AC-27: facade giả chậm quá deadline → 503 `UNAVAILABLE` sau đúng một lần gọi (không retry); facade panic → 500; Problem 4xx giữ `code`; không có policy circuit/bulkhead nào được đăng ký (TC-SVC-011) — file: `src/backend/pkg/modcall/errors_test.go`
- [x] T046 [P] [US5] Test F003-AC-28: adapter `PrincipalResolver` với facade identity giả: thành công → Principal, không membership → `ErrNotMember`, deadline → `UNAVAILABLE` — file: `src/backend/cmd/internal/wiring/principal_resolver_test.go`

### Implementation cho User Story 5

- [x] T047 [US5] `modcall.Call` (span, deadline theo `module_call_deadline`, kiểm `x-context-required`/`x-allowed-callers`, context on-behalf, recover, `problem.Map`, metric API_DESIGN §12.3) — file: `src/backend/pkg/modcall/call.go`
- [x] T048 [US5] Adapter `authz.PrincipalResolver` bọc facade `identity` (`resolvePrincipal`) qua `modcall`, nối ở composition root `core-api`/`worker` — file: `src/backend/cmd/internal/wiring/principal_resolver.go`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase 8: User Story 6 — Dừng có trật tự, không mất việc (Priority: P2)

**Goal**: `src/backend/pkg/lifecycle`.

**Independent Test**: Như spec.md User Story 6.

### Tests cho User Story 6

- [x] T049 [P] [US6] Integration test F003-AC-29: SIGTERM khi request đang chạy → readiness 503, request hoàn tất, server đóng — file: `src/backend/pkg/lifecycle/api_shutdown_integration_test.go` — phải FAIL trước khi code
- [x] T050 [P] [US6] Integration test F003-AC-30: SIGTERM worker giữa lô → ack phần xong, phần còn lại giao lại, outbox relay lần sau (TC-SVC-002) — file: `src/backend/pkg/lifecycle/worker_shutdown_integration_test.go`

### Implementation cho User Story 6

- [x] T051 [US6] `lifecycle.RunAPI`/`RunWorker` theo BACKEND_DESIGN §13 (`EW_HTTP_SHUTDOWN_TIMEOUT`), gắn vào `src/backend/cmd/core-api`, `src/backend/cmd/realtime`, `src/backend/cmd/worker` — file: `src/backend/pkg/lifecycle/shutdown.go`

**Checkpoint**: User Story 6 chạy và kiểm độc lập được.

## Phase 9: User Story 7 — Testkit và contract-first (Priority: P2)

**Goal**: Fixture + contract helper + oasdiff.

**Independent Test**: Như spec.md User Story 7.

### Tests cho User Story 7

- [x] T052 [P] [US7] Test F003-AC-31: testkit dựng đủ container, migrate module mẫu, fixture tenant A/B, context scope — file: `src/backend/pkg/testkit/testkit_integration_test.go` — phải FAIL trước khi code
- [x] T053 [P] [US7] Test F003-AC-32: contract helper phát hiện response lệch spec public, response object facade lệch ví dụ spec internal và payload event lệch schema; fake facade sinh được cho bên gọi; `make specs-breaking` fail với thay đổi breaking mẫu (TC-SVC-015) — file: `src/backend/pkg/testkit/contract/openapi_test.go`

### Implementation cho User Story 7

- [x] T054 [US7] `testkit.New(t, module)`, `Fixtures` (tenant A/B, Principal theo role từ `authztest`), `migrate.go` — file: `src/backend/pkg/testkit/fixtures.go`
- [x] T055 [US7] `testkit/contract` (kin-openapi validator public, kiểm facade theo ví dụ spec internal, fake facade cho bên gọi, jsonschema cho event; target `make test-contract`) + target `make specs-breaking` (oasdiff spec gộp + mọi internal) — file: `src/backend/pkg/testkit/contract/openapi.go`

**Checkpoint**: User Story 7 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [x] T056 [P] Kiểm DATABASE_DESIGN §3.9–3.10 (DDL `infra.dead_letters`, CHECK kích thước `idempotency_records` đã có theo ADR-0089) khớp ent `infraschema`; API_DESIGN §6 đổi `platform.idempotency_records` → `infra.idempotency_records` (D6) nếu còn sót — file: `docs/07-backend/DATABASE_DESIGN.md`
- [x] T074 [P] Delta contract: mô tả `Idempotency-Key` là UUIDv7 (`format: uuid` + mô tả, `400 BAD_REQUEST` khi sai) ở parameter dùng chung của spec gộp rồi chạy `split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [x] T057 [P] BACKEND_DESIGN §5.5 thay `CONCURRENT_MODIFICATION` bằng `CONFLICT`; §11 và TC-SVC-004 dùng tên metric `eduworld_internal_client_*` — file: `docs/07-backend/BACKEND_DESIGN.md`
- [x] T058 [P] CHANGELOG mục F003 — file: `CHANGELOG.md`
- [x] T064 [P] Đăng ký metric `eduworld_usecase_duration_seconds` ở MONITORING; thêm biến `EW_DB_READ_URL_FILE` (client đọc của `ReadSession`) vào BACKEND_DESIGN §12 — file: `docs/11-operations/MONITORING.md`
- [x] T065 [P] Sửa mô tả TC-SVC-001 (bỏ chữ "Mediator CQRS", dùng write/read context theo ADR-0087) — file: `docs/12-project/TEST_STRATEGY.md`
- [x] T071 [P] Đăng ký cảnh báo P2 "thiếu partition `infra`" và metric của job `infra-partition-maintenance` ở MONITORING — file: `docs/11-operations/MONITORING.md`
- [x] T059 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/003-platform-kit-data/quickstart.md`

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T077 **OQ chốt 2026-11-20** (OQ F010-3) `tenancy.Bypass` reason `platform_job`: audit `cross_tenant.accessed` chỉ khi job báo ≥ 1 dòng (kèm số dòng), metric vẫn đếm mỗi lần; cập nhật job learning/identity báo số dòng — file: `src/backend/pkg/tenancy/bypass.go`, các `presentation/job/*.go` — kiểm: unit + integration (job rỗng không sinh audit) — cỡ M
- [ ] T078 **OQ chốt 2026-11-20** (OQ F020-4) Lint `forbidigo` cấm `UpdateOne`/`UpdateOneID` trong module — file: `src/backend/.golangci.yml` — kiểm: `make lint` — cỡ S
- [ ] T079 **OQ chốt 2026-11-20** (OQ F014-6) Helper metric `eduworld_projection_lag_seconds{module, projection}` trong `pkg/`; learning (F020) và game (F014) chuyển sang, gỡ metric riêng — file: `src/backend/pkg/outbox/` (hoặc `pkg/observability`), `src/backend/modules/learning/internal/infrastructure/observability/`, `src/backend/modules/game/internal/infrastructure/observability/metrics.go` — kiểm: `make test` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 (cần `InTenantTx` của T017) → US3, US4 (song song, cần US2) → US5 → US6 → US7 → Polish.
- T062 cần cấu hình lint nền của F002 (T003); T063 cần T003, T024, T025.
- T004, T005, T006 trước T036, T037, T030; T007 trước T008, T029, T039; T066 sau T008, trước T069, T070.
- Mọi feature module M-C1 (F010…F015) chỉ bắt đầu khi F003 Done.

## Definition of Done

- [x] Mọi tiêu chí F003-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — **2026-11-18:** lệnh tương đương đã chạy xanh (generators `--check`, `go vet`, `golangci-lint`, `ewlint`, `archgen`, `routematrix`, `entlint`, `tenancyguard`, `go test ./...`, `go test -tags integration` cho các gói F003); chưa qua GNU make — chờ CI
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Ghi chú triển khai (2026-11-18)

- T007/T008/T066: bốn bảng `infra` partition theo ngày nên toàn bộ DDL viết tay, sinh từ một template bằng `src/backend/tools/infragen` cho 5 module (`modules/<m>/migrations/20261118000001_infra.sql`, `make gen`/`gen-check`); ent không dùng cho bảng `infra` — `outbox`/`idempotency` truy cập bằng SQL tường minh trong transaction của `UnitOfWork` (bảng không có `tenant_id` nghiệp vụ, không RLS). Partition tạo/gỡ qua hàm `SECURITY DEFINER` `infra.ensure_partitions`, `infra.partition_days_ahead`, `infra.drop_expired_partitions` (owner `<m>_migrator`); `<m>_partition_admin` chỉ có `EXECUTE`; partition mới không có grant; có partition `DEFAULT` để ghi không lỗi khi thiếu partition ngày (cảnh báo khi có dòng).
- T014: mixin mang luôn interceptor (truy vấn) và hook (create gán/kiểm `tenant_id`, update/delete thêm cùng điều kiện, delete → soft-delete hoặc `ErrHardDeleteForbidden`); `entx` không import code ent sinh (gọi `Where`/`Client().Mutate` qua reflection). Bảng H cần `HybridContent{ResourceType: "<loại>"}`.
- T017/T024: `UnitOfWork` giữ `*sql.Tx` trong context; module dựng client `ent.NewClient(ent.Driver(entx.Driver(ctx)))`; ngoài transaction mọi lệnh trả `ErrNoTransaction`. `CrossTenantScope` ghi audit `cross_tenant.accessed` khi mở transaction.
- T003: module mẫu ở `src/backend/pkg/testkit/sample` (ent sinh trong `sample/ent`, DDL fixture `sample/fixture/sample.sql` áp bằng `learning_migrator` trong test, schema `sample` của `eduworld_learning`).
- T022/T062: luật read/write context (F003-AC-10) nằm trong `tools/archgen` (`make arch-check`) cho mọi đường dẫn `*/internal/application/query` và `*/internal/infrastructure/persistence/read` (cả module mẫu), fixture thật trong `scripts/ci/test-arch-fixtures.sh` (14/14) thay cho `go-arch-lint.base.yml`.
- US3: command ghi kết quả bằng `idempotency.SetResult(ctx, result)` bên trong `UnitOfWork.Do` (đăng ký `entx.BeforeCommit`); middleware `httpx.Idempotency` bọc response, nếu handler lỗi do request trùng đã commit trước thì đọc lại và replay; `lock_timeout` 5 s → `409 IDEMPOTENCY_IN_PROGRESS` + `Retry-After`. Test integration đặt ở `src/backend/pkg/testkit/sample/idempotency_integration_test.go` (gói `internal` của module mẫu).
- US4: thêm `authz.EventScope(tenantID)` (scope của consumer lấy từ envelope — scope chỉ tạo trong `authz`; golden API cập nhật, là bổ sung không đổi chữ ký cũ). Consumer tự quyết DLQ theo `NumDelivered` (durable `MaxDeliver: -1`), lỗi `outbox.Permanent` vào DLQ ngay. Relay chỉ poll (200 ms); `pg_notify('ew_outbox')` đã phát khi commit để nối `LISTEN` sau [Confirmed]. Test gộp: `pkg/outbox/outbox_integration_test.go` (relay crash, consumer trùng/qua nửa đêm/scope, DLQ, quyền), `writer_test.go` (envelope + `audit.recorded` theo JSON Schema).
- T043/T037: `natsx.PrincipalInvalidator` dùng consumer ordered (tạm thời, `DeliverNew`) để mọi replica nhận mọi event và xóa L1 của chính mình; thêm `CachedResolver.InvalidateTenant` (`tenant.updated`) và `InvalidateAll` (`role.updated`, SCAN + UNLINK prefix module); `enrollment.changed` dùng `student_id`.
- US5: facade sinh bằng `tools/facadegen` (bundle spec internal + oapi-codegen v2.8.2: models + strict-server) vào `modules/<m>/api/facade_gen.go`; phần HTTP server sinh kèm không bao giờ được mount — analyzer `nointernalhttp` (trong `ewlint`) chặn mọi lời gọi `Handler*`/`NewStrictHandler*`. `allowedcallers` sinh thêm `ContextRequired`. `cmd/internal/wiring` được phép import `modules/<m>/api` (archgen). Caller của `modcall` = module của operation (`httpx` đặt `tenancy.WithModule`).
- Delta hợp đồng (bảo mật): `Principal.grants[]` (`PrincipalGrant`: `role_code`, `role_scope_kind`, `school_id`, `subject_id`, `permissions`) trong `identity.internal.v1.yaml` — `permissions`/`scopes` gộp không cho biết permission nào chỉ áp trong trường/môn nào; adapter từ chối Principal thiếu `grants` (503, fail-closed). Nội dung thuộc F012 [Confirmed].
- US6: `pkg/lifecycle` (Plan + Group) dùng trong `bootstrap` cho mọi binary: readiness 503 (`health.Registry.SetDraining`), `EW_SHUTDOWN_DRAIN_DELAY` (5 s khi có HTTP public), dừng worker (`EW_WORKER_SHUTDOWN_TIMEOUT` 60 s), `EW_HTTP_SHUTDOWN_TIMEOUT` (20 s), drain NATS, đóng pool. Consumer xử lý message đã đệm khi drain bằng context không bị hủy; `MaxInFlight` giới hạn số message đệm.
- US7: `testkit.New(t, module)` (Postgres migrated + NATS streams + Valkey + pools + UnitOfWork/ReadSession/outbox, `Kit.As(principal)` với fixture `authztest`); `testkit/contract` (response public theo spec gộp, response facade theo spec internal + ví dụ, envelope/payload theo JSON Schema); `facadegen` sinh `api.Fake` cho test bên gọi; `make test-contract` chạy contract helper + test bên gọi. `make specs-breaking` (oasdiff) có từ F001.
- T013/T018/T019: `tools/entlint` (MaxLen, `field.Text` cần `// maxlen-justified:`, đúng một mixin `entx` mỗi schema — thay cho `mixin_coverage_test.go`) và `tools/tenancyguard` (policy có `deleted_at`, bảng có `tenant_id` thiếu RLS trừ `infra`, `DELETE`/`TRUNCATE` ngoài `purge`, SQL tường minh thiếu `tenant_id` + `deleted_at`, miễn trừ bằng `tenancyguard:ok <lý do>`) trong `make arch-check`. T005: fixture event vô hiệu cache nằm trong `natsx/principal_invalidator_integration_test.go` (không tạo schema payload).
