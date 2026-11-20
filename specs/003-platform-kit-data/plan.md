# Implementation Plan: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Branch**: `003-platform-kit-data` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm vào module `pkg` các gói `entx` (mixin, interceptor, hook, `UnitOfWork` cho write context, `ReadSession` cho read context, schema bảng `infra`), helper `observability.UseCase` (span + metric + log theo use case), middleware `httpx.Idempotency` + store `idempotency`, `outbox` (Writer, Relay, Consumer, DLQ, recorder audit), `natsx` (JetStream, header ngữ cảnh, consumer invalidate Principal), `modcall` (gọi facade trong tiến trình: span, deadline, ngữ cảnh, ánh xạ lỗi — ADR-0074), adapter `PrincipalResolver` ở composition root, `lifecycle` (graceful shutdown) và `testkit`; công cụ `src/backend/tools/entlint`, `src/backend/tools/tenancyguard` và rule read/write context trong cấu hình lint nền `src/backend/tools/lint` (F002). **Không có `src/backend/pkg/mediator`** (ADR-0087): CQRS là khuôn package (`command`/`query`, `port/write`/`port/read`, `persistence/write`/`persistence/read`) do module mẫu `src/backend/pkg/testkit/sample` minh họa và linter cưỡng chế. Mỗi module nhúng schema `infra` của `src/backend/pkg/entx/infraschema` vào `ent/schema` và sinh migration bằng Atlas.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | `entgo.io/ent`, `jackc/pgx/v5`, Atlas, `nats-io/nats.go` (jetstream), `oapi-codegen/runtime`, `getkin/kin-openapi`, `santhosh-tekuri/jsonschema/v6`, `testcontainers-go` |
| Storage | PostgreSQL `eduworld_<module>` schema `infra`; NATS JetStream stream `EVT_<MODULE>`, `DLQ_<MODULE>`; Valkey (cache Principal; idempotency không dùng khóa Valkey — ADR-0089 §4) |
| Testing | `go test` + testcontainers (postgres:17, nats:2.10-alpine `-js`, valkey:8); facade giả sinh từ interface spec internal thử |
| Target Platform | Thư viện Go trong Linux container |
| Project Type | Platform kit |
| Performance Goals | Outbox lag cảnh báo > 60 s (EVENT_DESIGN §10); relay lô 500, poll 200 ms + `pg_notify` |
| Constraints | AGENTS R10, R12, R13, R21, R26, R27, R28, R29 · ADR-0028, ADR-0038, ADR-0087, ADR-0039, ADR-0040, ADR-0041, ADR-0042 |
| Scale/Scope | 5 database trên một cụm, 6 module, 3 binary |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Schema `audit.recorded` + event identity thiếu → task Phase 2 trước code |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Không mediator; write context dùng `UnitOfWork` + outbox, query chỉ đọc qua `ReadSession`, linter chặn `query` import `port/write`/`UnitOfWork`/publisher (F003-AC-10); facade sinh từ spec internal gọi qua `src/backend/pkg/modcall`; event qua outbox |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Interceptor theo mixin (ADR-0089), `InTenantTx` set `app.tenant_id`, RLS chỉ lọc tenant, `tenancyguard`, không hard-delete; gỡ partition chỉ cho ba bảng `infra` (ngoại lệ có giới hạn ADR-0070 theo ADR-0089 §5) |
| IV Server-authoritative | ✅ | Idempotency + audit cho thao tác quan trọng (hạ tầng cho R10) |
| V Xác thực, phân quyền | ✅ | `authz.Require` (F002) gọi tường minh đầu mỗi use case, test thiếu quyền bắt buộc (F003-AC-11); không token giữa module, bên nhận áp quyền của Principal gốc (ADR-0074) |
| VI An toàn trẻ em | ✅ | Payload event/audit không PII; log `observability.UseCase` không payload (F003-AC-33) |
| VII Một nguồn sự thật, cấu hình | ✅ | `entlint` (MaxLen), tham số theo tên CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Không áp dụng |
| IX Test có bằng chứng | ✅ | Integration thật, không mock DB; TC-SVC-001…004, TC-TEN-021/022 |

## Project Structure

### Documentation (feature này)

```text
specs/003-platform-kit-data/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/pkg/
├── entx/          base.go tenant_append_only.go tenant_optional.go hybrid_content.go global_base.go append_only.go
│                  mixin_coverage_test.go interceptor.go softdelete.go uow.go readsession.go
│                  infraschema/{outbox_event.go,processed_event.go,dead_letter.go,idempotency_record.go}
│                  partitionmaint/ (job infra-partition-maintenance, role <module>_partition_admin)
├── observability/ usecase.go (UseCase span + metric + log; thêm vào gói của F002)
├── httpx/         idempotency.go (middleware; thêm vào gói của F002)
├── idempotency/   store.go (ent, INSERT trong tx của UnitOfWork, replay) key.go (UUIDv7, key_issued_on, kiểm lệch giờ) hash.go
├── outbox/        writer.go relay.go consumer.go dlq.go auditrecorder.go envelope.go
├── natsx/         conn.go streams.go headers.go principal_invalidator.go
├── modcall/       call.go (Call, Op, Kind) context.go (Principal gốc, caller, scope, Idempotency-Key) deadline.go mapproblem.go metrics.go callers.go (đọc allowed_callers_gen)
src/backend/cmd/internal/wiring/principal_resolver.go   # adapter authz.PrincipalResolver bọc facade identity [Confirmed]
├── lifecycle/     shutdown.go (core-api, realtime, worker)
└── testkit/       containers.go migrate.go fixtures.go contract/{openapi.go,events.go}
    └── sample/    module mẫu Widget theo khuôn ADR-0087:
                   internal/application/{command,query,port/{write,read},dto}/
                   internal/infrastructure/persistence/{write,read}/  internal/presentation/http/
src/backend/tools/entlint/  src/backend/tools/tenancyguard/
src/backend/tools/lint/        golangci.base.yml go-arch-lint.base.yml (thêm rule read/write context; file của F002)
                   testdata/badquery/ (fixture vi phạm)
src/backend/modules/<module>/ent/schema/infra.go       # nhúng infraschema
src/backend/modules/<module>/migrations/<ts>_infra.sql # Atlas sinh + REVOKE + GRANT/role _relay, _partition_admin
src/backend/modules/<module>/migrations/partitions/<ts>_infra_partitions.sql # viết tay (ADR-0089 §6), atlas migrate lint
specs/events/payloads/common/audit.recorded.v1.schema.json            # Cần thêm
specs/events/payloads/identity/{tenant_membership.changed,user.access_revoked,enrollment.changed,tenant.updated}.v1.schema.json  # Cần thêm
```

**Structure Decision**: BACKEND_DESIGN §2, §3, §4 + D5/D6 + ADR-0087. Schema `infra` định nghĩa một lần trong `src/backend/pkg/entx/infraschema`, mỗi module import để ent sinh code trong module của mình (không chia sẻ ent client giữa service).

## Phase 0: Research

Không có. Thư viện và luồng đã chốt ở ADR-0039, ADR-0040, ADR-0042, EVENT_DESIGN §3, §7.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `entx` KHÔNG dùng trong domain; aggregate tự `PullEvents()` | Domain không biết ent |
| application/command (write) | Mỗi use case một kiểu handler + interface có kiểu; port `write.UnitOfWork` (`Do(ctx, fn)`), `write.<Aggregate>Repository`, `write.EventPublisher`; đầu handler: `observability.UseCase` → `in.Validate()` → `authz.Require` (F002) → `UoW.Do` | Không bus/registry; port không import ent |
| application/query (read) | Handler + interface có kiểu; port `read.<View>ReadModel`; đầu handler: `UseCase` → `Validate` → `Require` → đọc read model | Không import `port/write`, `src/backend/pkg/entx`, `src/backend/pkg/outbox` |
| infrastructure/persistence/write | `entx.UnitOfWork` (hiện thực `write.UnitOfWork`, `Publisher()` trả `outbox.Writer` gắn tx), repository ent | `Do(ctx, fn)` = `InTenantTx(ctx, tenancy.ScopeFrom(ctx), fn)` |
| infrastructure/persistence/read | `entx.ReadSession.InTenantRead(ctx, scope, fn)` (tx `READ ONLY`, client replica nếu cấu hình) | Không `outbox` |
| infrastructure (khác) | `outbox.Relay`, `outbox.Consumer`, adapter `infrastructure/modules/<callee>.go` dùng `modcall` | |
| presentation | HTTP handler giữ interface command/query; consumer handler `func(ctx, outbox.Envelope) error` gọi trực tiếp command handler; `lifecycle.RunAPI`, `RunWorker` | `src/backend/cmd/worker --profile=events` chạy relay + consumers; nối port ↔ adapter trong `module.go`, facade qua `Wire` ở `src/backend/cmd/` |

**Hành vi chính**:

- `UnitOfWork.Do`: lấy scope từ context (do middleware `principal`/`tenant`, adapter NATS hoặc `outbox.Consumer` đặt) → `InTenantTx` → gắn `*ent.Tx` vào context; repository và `outbox.Writer` lấy tx từ context; commit/rollback (cả khi panic). Không lồng tx: gọi `Do` khi đã có tx trong context dùng lại tx đó.
- `ReadSession.InTenantRead`: `BEGIN READ ONLY` + `set_config('app.tenant_id')` trên client đọc (`EW_DB_READ_URL_FILE` nếu có, mặc định primary) [Confirmed — tên biến, thêm vào BACKEND_DESIGN §12 khi triển khai].
- `observability.UseCase(ctx, name) (context.Context, func(error))`: span `name`, metric `eduworld_usecase_duration_seconds{module,kind,usecase,result}`, log một dòng (không payload); `result` ∈ `ok`, `client_error`, `server_error` theo `problem.Kind`.
- Middleware `httpx.Idempotency` (chỉ operation khai báo `IdempotencyKey` trong spec): kiểm key là UUIDv7 và timestamp lệch ≤ `idempotency_record_ttl` (nếu không → `400 BAD_REQUEST`), tính `key_issued_on`; đọc `infra.idempotency_records` theo PK (bỏ `expires_at < now()`) → có bản ghi: so `request_hash` (SHA-256 body chuẩn hóa) rồi replay hoặc `409 IDEMPOTENCY_KEY_REUSED`; chưa có: đặt `idempotency.Pending` vào context và chạy handler. `UnitOfWork.Do` thấy `Pending` thì `INSERT` bản ghi (hash + response đã tuần tự hóa do handler đăng ký qua `idempotency.SetResponse(ctx, status, body)`) ngay trước commit, cùng transaction nghiệp vụ. Request trùng đồng thời chờ ở unique index; khi transaction đầu commit, `INSERT` của request sau lỗi unique → đọc lại và replay; chờ vượt `lock_timeout` → `409 IDEMPOTENCY_IN_PROGRESS` + `Retry-After`. Không khóa Valkey, không trạng thái `in_progress` (ADR-0089 §4). Lớp cuối vẫn là unique constraint nghiệp vụ (API_DESIGN §6).
- Relay: `SELECT … WHERE published_at IS NULL ORDER BY event_id LIMIT 500 FOR UPDATE SKIP LOCKED` (dùng role `<module>_relay`, bảng không RLS, ADR-0089 §4) → `js.Publish(subject, data, nats.MsgId(event_id))` → `UPDATE published_at, attempts`.
- `partitionmaint`: chạy trong `src/backend/cmd/worker` (profile `worker-jobs`, ADR-0074) theo lịch; tạo partition ngày tiếp theo của bốn bảng `infra` trước tối thiểu 7 ngày (thiếu → cảnh báo P2); `DETACH PARTITION … CONCURRENTLY` + `DROP` partition quá `outbox_retention` (outbox, processed), `idempotency_record_ttl` hoặc retention dead letter (EVENT_DESIGN §8); kiểm `published_at IS NULL` (outbox) và `resolved_at IS NULL` (dead letters) trước khi gỡ, còn dòng → bỏ qua + cảnh báo/metric.
- Consumer: durable `<module>.<handler>`, `FilterSubjects`, `AckWait` 60 s, `BackOff` 1′/5′/30′, `MaxDeliver` theo EVENT_DESIGN §7; xử lý trong `InTenantTx` với scope từ envelope; `INSERT … processed_events ON CONFLICT DO NOTHING` → nếu 0 dòng thì ack bỏ qua.
- modcall (BACKEND_DESIGN §7.2): `Call(ctx, Op{Caller, Callee, Operation, Kind, IdempotencyKey}, fn)` → span + metric → deadline theo `module_call_deadline` → kiểm `x-context-required` (scope, `Idempotency-Key`) và `x-allowed-callers` (allowlist sinh, F002 T006) → gọi `fn` (facade) → recover panic → `problem.Map` (giữ `code`; deadline/DB đích lỗi → `503 UNAVAILABLE`). Không HTTP, token, retry, circuit breaker, bulkhead (ADR-0074 mục 3).

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F002 (`authz`, `tenancy`, `problem`, `audit`, `observability`) · Facade gọi: `resolvePrincipal` (facade `identity`, qua adapter ở composition root) · Event phát: `audit.recorded`, `<entity>.deleted` (qua helper) · Event tiêu thụ: `user_role.changed`, `role.updated`, `tenant_membership.changed`, `user.access_revoked`, `enrollment.changed`, `tenant.updated` (invalidate Principal).

### Bảo mật, tenant, idempotency, audit

- Interceptor (theo mixin) + RLS (chỉ tenant) hai lớp; view `_live` cho `<module>_readonly`; `Bypass` chỉ qua `tenancy.Bypass(ctx, reason)` với lý do thuộc danh sách đóng (F002, ADR-0089 §3).
- Idempotency theo user; `Idempotent-Replayed`; bản ghi chỉ tồn tại khi transaction nghiệp vụ commit (lỗi → rollback, retry được).
- Audit: `audit.recorded` qua outbox; `SoftDelete` ghi `action = <entity>.deleted`.

### Quan sát

- Metric: `eduworld_outbox_unpublished_count{module}`, `eduworld_outbox_lag_seconds{module}`, `eduworld_events_consumed_total{module,consumer,event_type,result}`, `eduworld_internal_client_requests_total{caller_module,callee_module,operation,outcome}`, `eduworld_internal_client_duration_seconds`, `eduworld_idempotency_replays_total{route}`, `eduworld_usecase_duration_seconds{module,kind,usecase,result}` (đăng ký vào MONITORING — task Polish).
- Span: `<module>.command.<Name>`, `<module>.query.<Name>`, `<caller>→<callee>.<operationId>`, `outbox.publish`, `consumer.<name>` (link tới span publish qua `traceparent` header NATS).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
