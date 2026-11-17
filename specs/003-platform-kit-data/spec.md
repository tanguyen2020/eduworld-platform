# Feature Specification: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Feature Branch**: `003-platform-kit-data` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `pkg/` · **Mốc**: M-C0 · **Phụ thuộc**: F002
**Input**: Các gói dùng chung cho tầng dữ liệu và giao tiếp: ent mixin + global filter tenant/soft-delete, `UnitOfWork` (transaction + outbox) và phiên đọc, khuôn CQRS tách write/read context không mediator (ADR-0087), helper span use case, idempotency, transactional outbox → NATS JetStream, consumer idempotent, helper gọi facade giữa module trong tiến trình (`pkg/modcall`, ADR-0074), graceful shutdown và testkit — FR-ORG-011, NFR-MAINT-002, NFR-MAINT-004, NFR-MAINT-005, NFR-MAINT-006, NFR-SEC-004, NFR-AVAIL-004, NFR-AVAIL-005.

## Clarifications

### Session 2026-11-16

- Q: Bảng `idempotency_records` ở schema `platform` (API_DESIGN §6) hay `infra` (DATABASE_DESIGN §3.10)? → A: `infra.idempotency_records` trong database của **mỗi** module (D6, DATABASE_DESIGN là nguồn bảng).
- Q: Hook/interceptor ent ở `pkg/tenancy` hay `pkg/entx`? → A: `pkg/entx` (D5); `pkg/entx` dùng `TenantScope`, `Bypass` của `pkg/tenancy` (F002).
- Q: Xung đột optimistic concurrency trả mã nào (BACKEND_DESIGN §5.5 dùng `CONCURRENT_MODIFICATION`, không có trong catalog)? → A: `409 CONFLICT` từ repository; `412 VERSION_MISMATCH` khi request có `If-Match` lệch ETag (API_DESIGN §7). Không thêm mã mới.
- Q: Tên metric lời gọi giữa module? → A: Theo [API_DESIGN §12.3](../../docs/07-backend/API_DESIGN.md) (tài liệu sở hữu quy ước facade): `eduworld_internal_client_requests_total{caller_module,callee_module,operation,outcome}`, `eduworld_internal_client_duration_seconds` (`caller`/`callee` là tên module); không có metric circuit cho lời gọi facade (ADR-0074). Metric `eduworld_external_client_*` thuộc adapter tích hợp ngoài.
- Q: Adapter `resolvePrincipal` dùng chung đặt ở đâu? → A: (sửa 2026-11-17 theo ADR-0074) `pkg/` không được import `modules/`, nên `pkg/authz` chỉ có port `PrincipalResolver`; adapter bọc facade `identity` (`modules/identity/api`, operation `resolvePrincipal`) qua `pkg/modcall` nằm ở composition root `cmd/internal/wiring/principal_resolver.go` [Proposed] ([BACKEND_DESIGN §6](../../docs/07-backend/BACKEND_DESIGN.md)).
- Q: `pkg/modcall` còn token, header, retry, circuit breaker? → A: Không (ADR-0074 mục 3). `modcall.Call` chạy trong tiến trình: span, deadline theo `module_call_deadline`, kiểm `x-context-required` và `x-allowed-callers`, giữ Principal gốc/`TenantScope`/`Idempotency-Key` trong `context`, ánh xạ lỗi sang `problem` ([BACKEND_DESIGN §7.2](../../docs/07-backend/BACKEND_DESIGN.md)). Resilience (`external_api_resilience`) chỉ ở adapter tích hợp ngoài của module sở hữu.

### Session 2026-11-16 (ADR-0087)

- Q: Còn `pkg/mediator` và pipeline behavior không? → A: Không. CQRS tách write context (`application/command`, `port/write`, `persistence/write`, `UnitOfWork` + outbox) và read context (`application/query`, `port/read`, `persistence/read`, `ReadSession`); presentation gọi handler qua interface có kiểu. F003 cung cấp `entx.UnitOfWork`, `entx.ReadSession`, `observability.UseCase`, rule linter read/write và service mẫu minh họa khuôn.
- Q: Logging/tracing/validation/authorization/idempotency/transaction của behavior cũ đi đâu? → A: Xác thực, Principal, tenant, idempotency, Problem Details, recover, log, trace HTTP ở middleware `pkg/httpx` (F002; middleware idempotency do F003 thêm); `authz.Require` gọi tường minh đầu use case (hàm thuộc F002, F003 chỉ dùng); `Validate()` DTO trong handler; transaction bằng `UnitOfWork.Do` (= `InTenantTx` với scope trong context); span use case bằng `observability.UseCase` (F003 thêm vào `pkg/observability`).
- Q: Cache Valkey cho query (behavior Cache cũ)? → A: Không có kit chung trong F003; adapter `persistence/read` của từng module tự cache theo [VALKEY_DESIGN](../../docs/07-backend/VALKEY_DESIGN.md) khi feature sở hữu màn hình cần.

### Session 2026-11-17 (ADR-0089)

- Q: Bộ mixin `pkg/entx` gồm những gì? → A: `Base` (T), `TenantAppendOnly` (TA), `TenantOptional` (T\*), `TenantOptionalAppendOnly` (TA\*), `HybridContent` (H, đổi tên từ `PlatformBase`), `GlobalBase` (G), `AppendOnly` (GA); dòng `tenant_id IS NULL` của T\*/TA\*/H chỉ ghi bằng `<module>_platform` dưới `PlatformScope`; mỗi entity đúng một mixin; điều kiện interceptor theo [MULTITENANCY §4.4](../../docs/06-architecture/MULTITENANCY.md#44-quy-tắc-interceptor-theo-mixin-ba-loại-scope-và-cổng-bypass-adr-0089); `GlobalBase`/`AppendOnly` không cần scope.
- Q: RLS có lọc `deleted_at` không? → A: Không, RLS chỉ lọc tenant; `deleted_at IS NULL` do interceptor bảo đảm, nên soft-delete chạy được dưới RLS (ADR-0089 §1).
- Q: Ai ghi/publish outbox và dọn bảng `infra` thế nào? → A: Bảng `infra` không bật RLS; `<module>_app` `INSERT` outbox trong `UnitOfWork`, `INSERT`/`SELECT` processed/idempotency, `INSERT` dead letters; relay dùng `<module>_relay` (`SELECT, UPDATE (published_at, attempts)`); `<module>_platform` `SELECT`/`UPDATE` cột xử lý dead letters; bốn bảng outbox/processed/idempotency/dead letters partition theo ngày, job `infra-partition-maintenance` (role `<module>_partition_admin`) tạo partition trước tối thiểu 7 ngày và gỡ partition hết hạn (`outbox_retention`, `idempotency_record_ttl`, retention dead letter của EVENT_DESIGN), từ chối gỡ partition còn outbox chưa publish hoặc dead letter chưa xử lý; DDL partition viết tay trong `migrations/partitions/` (ADR-0089 §4–6).
- Q: Chống trùng xuyên ngày khi partition theo ngày? → A: Khóa partition ổn định: `processed_events` partition theo `event_occurred_at` của envelope, PK `(consumer, event_id, event_occurred_at)`; `idempotency_records` partition theo `key_issued_on` lấy từ timestamp của `Idempotency-Key` (bắt buộc UUIDv7), PK `(user_id, route_template, idem_key, key_issued_on)`; key không phải UUIDv7 hoặc lệch quá `idempotency_record_ttl` → `400` (ADR-0089 §4).
- Q: Ghi idempotency thế nào? → A: `INSERT` một lần trong cùng transaction `UnitOfWork` với thay đổi nghiệp vụ, kèm response đã tuần tự hóa; request trùng đồng thời chờ ở unique index tới khi transaction đầu commit rồi replay; không có trạng thái `in_progress`, không khóa Valkey (ADR-0089 §4).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Repository ent an toàn tenant và chỉ soft-delete (Priority: P1)

Backend dev khai báo entity bằng mixin; mọi truy vấn tự lọc tenant và bản ghi đã xóa; "xóa" luôn là soft-delete có audit.

**Why this priority**: R8, R21, R27; mọi module ở M-C1 dùng ngay.

**Independent Test**: Module thử trong testkit với entity mẫu `Widget` (mixin Base): query không scope, xóa, tạo lại cùng mã.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F003-AC-01 | Mỗi entity PHẢI khai báo đúng một mixin `pkg/entx`: `Base` cho T (UUIDv7 `id`, `tenant_id`, `created_at/by`, `updated_at/by`, `deleted_at/by`, `delete_reason` tối đa 64, `version`), `TenantAppendOnly` cho TA, `TenantOptional` cho T\* (`tenant_id` nullable), `TenantOptionalAppendOnly` cho TA\* (append-only, `tenant_id` nullable), `HybridContent` cho H (`tenant_id` nullable + `owner_scope`, tên cũ `PlatformBase`), `GlobalBase` cho G, `AppendOnly` cho GA | [BACKEND_DESIGN §5.2](../../docs/07-backend/BACKEND_DESIGN.md) · ADR-0039 · ADR-0089 §2 | — |
| F003-AC-02 | KHI truy vấn ent, interceptor PHẢI thêm điều kiện theo mixin của entity ([MULTITENANCY §4.4](../../docs/06-architecture/MULTITENANCY.md#44-quy-tắc-interceptor-theo-mixin-ba-loại-scope-và-cổng-bypass-adr-0089)): `Base` → `tenant_id IN (scope) AND deleted_at IS NULL`; `TenantAppendOnly` → `tenant_id IN (scope)`; `TenantOptional`/`TenantOptionalAppendOnly`/`HybridContent` → nhánh `tenant_id IS NULL` chỉ qua `PlatformScope` (H: hoặc được gán cho tenant trong scope); `GlobalBase` → `deleted_at IS NULL`; `AppendOnly` → không điều kiện; NẾU entity cần scope mà context không có scope và không `Bypass`, THÌ PHẢI trả `ErrMissingTenantScope` và không trả dòng nào | AGENTS R27 · [MULTITENANCY §4.1](../../docs/06-architecture/MULTITENANCY.md) · ADR-0089 §2–3 | TC-TEN-021 |
| F003-AC-03 | KHI gọi `Delete()`/`DeleteOne()`, hook PHẢI chuyển thành cập nhật `deleted_at`, `deleted_by` (người thực hiện) và `delete_reason`; NẾU entity không có trường soft-delete, THÌ PHẢI trả `ErrHardDeleteForbidden` | FR-ORG-011 · ADR-0028 | TC-TEN-022 |
| F003-AC-04 | Helper `entx.SoftDelete(ctx, entity, reason)` PHẢI ghi audit và event `<entity>.deleted` vào outbox trong cùng transaction với cập nhật `deleted_at` | FR-ORG-011 · AGENTS R21 | TC-TEN-015 |
| F003-AC-05 | Unique index trên bảng soft-delete PHẢI là partial `WHERE deleted_at IS NULL`; KHI bản ghi đã soft-delete, tạo bản ghi mới cùng khóa nghiệp vụ PHẢI thành công | FR-ORG-011 · DATABASE_DESIGN §1 | TC-TEN-015 |
| F003-AC-06 | `entx.UnitOfWork.InTenantTx` (write) PHẢI mở transaction, chạy `set_config('app.tenant_id', …, true)` và dùng client `<module>_app` cho scope một tenant, `<module>_platform` cho `CrossTenantScope`, `PlatformScope` (`PlatformScope(p)`, `SystemPlatformScope(job)`) và reason `Bypass` có quyền `<module>_platform`; NẾU ghi dòng `tenant_id IS NULL` bằng client `<module>_app`, THÌ RLS PHẢI từ chối; `entx.ReadSession.InTenantRead` (read) PHẢI làm như vậy trong transaction `READ ONLY` và được trỏ client replica; truy vấn tenant-scoped ngoài hai cơ chế này PHẢI lỗi | MULTITENANCY §5.2 · ADR-0005 | TC-TEN-005 |
| F003-AC-34 | KHI `Delete()`/`entx.SoftDelete` chạy qua `UnitOfWork` bằng role `<module>_app` trên bảng T có RLS (chỉ điều kiện tenant), lệnh cập nhật `deleted_at` PHẢI thành công và dòng đó PHẢI biến mất khỏi mọi truy vấn qua interceptor; chỉ `tenancy.Bypass(ctx, "trash")` PHẢI đọc lại được dòng đó | ADR-0089 §1 · ADR-0028 · AGENTS R21 | TC-TEN-022 |
| F003-AC-07 | NẾU `field.String` thiếu `MaxLen`, schema ent không khai báo đúng một mixin trong danh sách ADR-0089 (test liệt kê mọi schema), policy RLS chứa `deleted_at`, SQL tường minh thiếu điều kiện tenant + `deleted_at IS NULL`, có `DELETE FROM`/`TRUNCATE` ngoài package purge, hoặc bảng có `tenant_id` thiếu RLS, THÌ `entlint`/`tenancyguard` PHẢI fail kèm vị trí | NFR-MAINT-005 · ADR-0041 · MULTITENANCY §11 | TC-TEN-017, TC-TEN-024 |

---

### User Story 2 — Command/query tách write/read context, không mediator (Priority: P1)

Backend dev viết use case ghi trong write context (aggregate + `UnitOfWork` + outbox) và use case đọc trong read context (read model, chỉ đọc); presentation gọi handler qua interface có kiểu; authorization, validation, span là lời gọi tường minh; linter chặn query chạm phía ghi.

**Why this priority**: R26, NFR-MAINT-004, ADR-0087; mọi module ở M-C1 dựng use case theo khuôn này.

**Independent Test**: Module mẫu `Widget` trong testkit: command `CreateWidget`/`DeleteWidget` (write) và query `ListWidgets` (read) — lỗi sau khi ghi outbox, query bằng role chỉ đọc, adapter read cố ghi, validate độ dài, thiếu permission, fixture import cấm cho linter.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F003-AC-08 | KHI command handler gọi `UnitOfWork.Do(ctx, fn)`, `entx.UnitOfWork` PHẢI mở một transaction trên primary DB, gắn transaction vào `ctx` để repository ghi và `outbox.Writer` dùng chung, commit khi `fn` trả `nil` và rollback khi `fn` trả lỗi hoặc panic | NFR-MAINT-004 · [BACKEND_DESIGN §4.1](../../docs/07-backend/BACKEND_DESIGN.md) · ADR-0087 | TC-SVC-001 |
| F003-AC-09 | NẾU command handler lỗi sau khi đã ghi nghiệp vụ và outbox, THÌ transaction PHẢI rollback và không còn event nào trong `infra.outbox_events`; query handler PHẢI chạy thành công với role DB chỉ đọc; NẾU adapter read model ghi trong `entx.ReadSession`, THÌ PHẢI lỗi (transaction `READ ONLY`) | NFR-MAINT-004 · ADR-0038 · ADR-0087 | TC-SVC-001 |
| F003-AC-10 | NẾU package `application/query` import `application/port/write`, `application/command`, `pkg/entx` hoặc `pkg/outbox`, hoặc package `infrastructure/persistence/read` import `pkg/outbox` hay `persistence/write`, THÌ `go-arch-lint`/`depguard` (cấu hình nền `tools/lint`) PHẢI fail kèm vị trí | AGENTS R13, R26 · ADR-0087 | — |
| F003-AC-11 | KHI `Validate()` của DTO input phát hiện chuỗi vượt giới hạn, handler mẫu PHẢI trả `422 VALIDATION_FAILED` với `errors[].code = MAX_LENGTH` trước khi mở transaction; KHI thiếu permission, handler PHẢI trả lỗi của `authz.Require` (F002) và PHẢI KHÔNG gọi repository | NFR-MAINT-005 · ADR-0041 · ADR-0087 | TC-TEN-024 |
| F003-AC-12 | NẾU repository lưu aggregate với `version` đã cũ, THÌ PHẢI trả `409 CONFLICT` và không ghi đè dữ liệu | BACKEND_DESIGN §5.5 · API_DESIGN §7 | — |
| F003-AC-33 | KHI handler gọi `observability.UseCase(ctx, "<module>.<kind>.<Name>")` (`kind` = `command`/`query`), HỆ THỐNG PHẢI mở span con của span HTTP/consumer, ghi metric thời gian theo `service`, `kind`, `usecase`, `result` và một dòng log kết quả có `trace_id`, `tenant_id`; log PHẢI KHÔNG chứa payload input | NFR-MAINT-004 · AGENTS R14, R18 · BACKEND_DESIGN §11 | — |

---

### User Story 3 — Idempotency cho thao tác quan trọng (Priority: P1)

Client gửi lại cùng request (mất mạng, retry) mà không tạo kết quả trùng.

**Why this priority**: R10, NFR-SEC-004 — submit, nộp thi, tạo party, lời mời dùng từ M-C2.

**Independent Test**: Command mẫu có `IdempotencyKey`; gửi lặp cùng/khác payload, song song, lỗi 5xx; user khác cùng key.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F003-AC-13 | KHI request tới operation thay đổi trạng thái có `Idempotency-Key` hợp lệ, `pkg/idempotency` PHẢI `INSERT` bản ghi theo `(user_id, route_template, idem_key, key_issued_on)` kèm hash body và response đã tuần tự hóa **trong cùng transaction `UnitOfWork`** với thay đổi nghiệp vụ; KHI gửi lại cùng payload, HỆ THỐNG PHẢI trả nguyên response đã lưu kèm header `Idempotent-Replayed: true` | NFR-SEC-004 · [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md) · ADR-0089 §4 | — |
| F003-AC-14 | NẾU cùng key khác payload, THÌ PHẢI trả `409 IDEMPOTENCY_KEY_REUSED`; KHI hai request cùng key chạy đồng thời, request sau PHẢI chờ ở unique index tới khi transaction đầu commit rồi trả response đã lưu (`Idempotent-Replayed: true`), chỉ trả `409 IDEMPOTENCY_IN_PROGRESS` kèm `Retry-After` khi chờ vượt `lock_timeout`; NẾU operation bắt buộc key mà thiếu, THÌ PHẢI trả `428 IDEMPOTENCY_KEY_REQUIRED` | NFR-SEC-004 · API_DESIGN §4.1, §6 · ADR-0089 §4 | — |
| F003-AC-15 | NẾU transaction nghiệp vụ rollback (lỗi 4xx/5xx), THÌ bản ghi idempotency PHẢI không tồn tại (cho phép retry cùng key); bản ghi quá `idempotency_record_ttl` PHẢI bị bỏ qua khi đọc và PHẢI KHÔNG bị `DELETE` bởi ứng dụng | API_DESIGN §6 · ADR-0028 · ADR-0089 §4 | — |
| F003-AC-37 | NẾU `Idempotency-Key` không phải UUIDv7, hoặc timestamp trong key lệch quá `idempotency_record_ttl` so với giờ server, THÌ middleware PHẢI trả `400 BAD_REQUEST` trước khi mở transaction; KHI key hợp lệ, `key_issued_on` PHẢI được tính từ timestamp của key nên retry sau nửa đêm (UTC) vẫn bị PK chặn và replay | ADR-0089 §4 · API_DESIGN §6 | — |
| F003-AC-16 | Bản ghi idempotency của người dùng A PHẢI KHÔNG được trả cho người dùng B dùng cùng key và route | NFR-SEC-004 · AGENTS R8 | — |

---

### User Story 4 — Event không mất qua outbox → NATS, consumer idempotent, audit tập trung (Priority: P1)

Module phát domain event cùng transaction nghiệp vụ; module khác tiêu thụ đúng một lần theo nghĩa nghiệp vụ; mọi audit đi qua outbox.

**Why this priority**: R10, R26, ADR-0042; projection `ref.*` và audit tập trung ở M-C1 phụ thuộc.

**Independent Test**: testkit với NATS JetStream: commit, giết relay, chạy lại; giao trùng; event lỗi; event tenant lệch.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F003-AC-17 | `outbox.Writer.Publish` PHẢI ghi `infra.outbox_events` trong cùng ent transaction của command, với envelope hợp lệ theo `specs/events/envelope.schema.json` và `traceparent` | ADR-0042 · [EVENT_DESIGN §2, §3](../../docs/07-backend/EVENT_DESIGN.md) | TC-SVC-001 |
| F003-AC-18 | Relay PHẢI publish lên `ew.evt.<module>.<event_type>` với header `Nats-Msg-Id = event_id` và chỉ đặt `published_at` sau khi JetStream ack; KHI relay bị dừng giữa chừng rồi chạy lại, event PHẢI được publish và PHẢI KHÔNG trùng phía consumer | ADR-0042 · EVENT_DESIGN §3 | TC-SVC-002 |
| F003-AC-19 | Consumer PHẢI ghi `infra.processed_events(consumer, event_id, event_occurred_at)` (lấy `occurred_at` từ envelope) trong cùng transaction với xử lý; KHI cùng event được giao lại, consumer PHẢI bỏ qua và ack | NFR-MAINT-004 · EVENT_DESIGN §7 · ADR-0089 §4 | TC-SVC-003 |
| F003-AC-38 | KHI cùng event được giao lại sau nửa đêm (UTC) so với lần xử lý đầu, consumer PHẢI vẫn phát hiện trùng (PK theo `event_occurred_at` bất biến), bỏ qua và ack, không xử lý lần hai | ADR-0089 §4 · EVENT_DESIGN §7 | TC-SVC-003 |
| F003-AC-20 | Consumer PHẢI dựng `TenantScope` từ `tenant_id` của envelope; event `tenant_id = null` PHẢI chỉ được consumer đăng ký phạm vi platform xử lý; NẾU payload tham chiếu tài nguyên của tenant khác, THÌ PHẢI chuyển DLQ | MULTITENANCY §4.1 mục 7 · EVENT_DESIGN §9 | — |
| F003-AC-21 | NẾU xử lý thất bại quá số lần giao của EVENT_DESIGN §7 hoặc payload sai schema, THÌ consumer PHẢI publish `ew.dlq.<module>.<consumer>`, ghi `infra.dead_letters` và `Term` message gốc mà không chặn message sau | EVENT_DESIGN §7 | — |
| F003-AC-22 | `audit.Recorder` hiện thực bằng outbox PHẢI phát `audit.recorded` với payload `actor_id`, `actor_type`, `action`, `target_type`, `target_id`, `result`, `reason`, `metadata` và PHẢI KHÔNG chứa PII hay nội dung chat | AGENTS R10, R14 · EVENT_DESIGN §4.1 | — |
| F003-AC-23 | Consumer `natsx` PHẢI vô hiệu cache Principal khi nhận `user_role.changed`, `role.updated`, `tenant_membership.changed`, `user.access_revoked`, `enrollment.changed`, `tenant.updated` | [API_DESIGN §12.2](../../docs/07-backend/API_DESIGN.md) · ADR-0066 | — |
| F003-AC-35 | Bảng `infra.outbox_events`, `infra.processed_events`, `infra.idempotency_records`, `infra.dead_letters` PHẢI KHÔNG bật RLS; `outbox.Writer` PHẢI ghi bằng role `<module>_app` (chỉ `INSERT` trên outbox), `outbox.Relay` PHẢI chạy bằng role `<module>_relay` (chỉ `SELECT, UPDATE (published_at, attempts)`); consumer PHẢI ghi `infra.dead_letters` bằng `<module>_app` (chỉ `INSERT`), màn xử lý event lỗi cập nhật cột xử lý bằng `<module>_platform`; NẾU bất kỳ role ứng dụng nào chạy `DELETE`/`TRUNCATE` trên `infra.*`, THÌ PHẢI lỗi quyền | ADR-0089 §4 · [DATABASE_DESIGN §3.9](../../docs/07-backend/DATABASE_DESIGN.md#39-outbox--processed-events) | — |
| F003-AC-36 | Job `infra-partition-maintenance` (role `<module>_partition_admin`) PHẢI tạo partition ngày của `infra.outbox_events`, `infra.processed_events`, `infra.idempotency_records`, `infra.dead_letters` trước tối thiểu 7 ngày (thiếu → cảnh báo P2) và gỡ partition quá `outbox_retention` (outbox đã publish, processed events), `idempotency_record_ttl` hoặc retention dead letter (EVENT_DESIGN §8); NẾU partition còn dòng outbox chưa publish hoặc dead letter chưa xử lý, THÌ job PHẢI từ chối gỡ và phát cảnh báo; job PHẢI KHÔNG chạm bảng nghiệp vụ partition (ADR-0070) | ADR-0089 §5–6 · ADR-0070 | — |
| F003-AC-39 | KHI vận hành chạy `worker projection rebuild --module=<module nhận> --source=<module nguồn> [--table=ref.<bảng>]`, HỆ THỐNG PHẢI dựng lại projection `ref.*` của module nhận bằng operation facade phân trang của module nguồn (vd. `listProjectionSnapshot` của identity, F010) gọi trong tiến trình qua `pkg/modcall`, upsert theo `source_version` (PHẢI KHÔNG ghi đè bản mới hơn do consumer ghi song song), idempotent khi chạy lại; NẾU facade lỗi hoặc vượt deadline, THÌ lệnh PHẢI dừng với exit ≠ 0 và không để projection ở trạng thái xóa trắng; module sở hữu projection đăng ký bộ rebuild từng bảng vào khung `pkg/projection` | DATABASE_DESIGN §2.17 (quy tắc projection 3) · [BACKUP_RECOVERY](../../docs/11-operations/BACKUP_RECOVERY.md) · ADR-0074 | TC-SVC-014 |

---

### User Story 5 — Gọi module khác an toàn qua facade trong tiến trình (Priority: P2)

Module gọi operation facade của module khác (sinh từ spec internal) qua `pkg/modcall`, mang đủ ngữ cảnh trong `context`; lỗi hoặc chậm của module đích không làm bên gọi fail-open hay treo.

**Why this priority**: R29, NFR-MAINT-006, NFR-AVAIL-005; cần từ M-C1 (Principal) và M-C2.

**Independent Test**: Facade giả (sinh từ interface của spec internal thử) trả lỗi Problem, chậm quá deadline, panic; gọi qua `modcall.Call` từ module có và không có trong `x-allowed-callers`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F003-AC-24 | `modcall.Call` PHẢI mở span `<caller>→<callee>.<operationId>`, ghi metric `eduworld_internal_client_*{caller,callee,operation,outcome}` và đặt deadline theo `module_call_deadline` (đọc/ghi/bước saga) khi context chưa có deadline ngắn hơn; PHẢI KHÔNG mở kết nối mạng, lấy token hay gửi header HTTP | NFR-MAINT-006 · ADR-0074 · [API_DESIGN §12.3](../../docs/07-backend/API_DESIGN.md) | — |
| F003-AC-25 | Mọi lời gọi facade PHẢI giữ trong `context` span cha, `TenantScope` (hoặc `CrossTenantScope`/`PlatformScope`), Principal gốc (on-behalf) và module gọi; NẾU operation khai báo `x-context-required` có `tenant_scope` hoặc `idempotency_key` mà context thiếu, THÌ `modcall` PHẢI trả lỗi lập trình `500` và PHẢI KHÔNG gọi facade; NẾU module gọi không có trong `x-allowed-callers` của operation, THÌ PHẢI trả `403 CALLER_NOT_ALLOWED` | NFR-MAINT-006 · API_DESIGN §12.1 · ADR-0074 | TC-SVC-006 |
| F003-AC-26 | `modcall` PHẢI KHÔNG retry, mở circuit breaker hay giới hạn bulkhead cho lời gọi facade; retry, circuit breaker, bulkhead theo `external_api_resilience` PHẢI chỉ nằm trong adapter tích hợp ngoài (Keycloak, SIS, object storage, SMTP) của module sở hữu | NFR-AVAIL-005 · ADR-0074 · API_DESIGN §12.3 | — |
| F003-AC-27 | KHI deadline vượt hoặc database của module đích không sẵn sàng, `modcall` PHẢI trả `503 UNAVAILABLE` có `retryable`; KHI facade panic, PHẢI recover thành `500 INTERNAL`; KHI facade trả lỗi Problem, PHẢI giữ nguyên `code`; PHẢI KHÔNG fail-open | NFR-AVAIL-005 · BACKEND_DESIGN §7 | TC-SVC-011 |
| F003-AC-28 | Adapter `authz.PrincipalResolver` ở composition root PHẢI gọi facade `identity` operation `resolvePrincipal` qua `modcall` và ánh xạ phản hồi không có membership (`404`) thành `authz.ErrNotMember` | AGENTS R29 · ADR-0074 · ADR-0040 | — |

---

### User Story 6 — Dừng có trật tự, không mất việc (Priority: P2)

Khi rolling deploy, binary ngừng nhận việc mới, hoàn tất việc đang làm, không mất event outbox.

**Why this priority**: NFR-AVAIL-004; deploy độc lập (NFR-DEPL-001) cần điều này.

**Independent Test**: Gửi SIGTERM khi có request đang chạy và lô consumer dang dở.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F003-AC-29 | KHI binary `core-api` nhận SIGTERM, HỆ THỐNG PHẢI chuyển `/readyz` sang 503, chờ drain, đóng HTTP public (gồm route gateway nội bộ) trong thời hạn, drain subscription NATS request/reply rồi đóng pool; request đang xử lý PHẢI hoàn tất | NFR-AVAIL-004 · [BACKEND_DESIGN §13](../../docs/07-backend/BACKEND_DESIGN.md) | — |
| F003-AC-30 | KHI binary `worker` (mọi profile) nhận SIGTERM, HỆ THỐNG PHẢI ngừng fetch, chờ handler đang chạy trong giới hạn, commit + ack lô đã xử lý; message chưa ack PHẢI được giao lại; event outbox chưa publish PHẢI được relay lần chạy sau | NFR-AVAIL-004 · ADR-0042 | TC-SVC-002 |

---

### User Story 7 — Testkit và contract-first (Priority: P2)

Mọi module có integration test trên database/NATS/Valkey thật, fixture tenant A/B, contract test với spec public/event và facade test với spec internal.

**Why this priority**: NFR-MAINT-002, constitution IX.

**Independent Test**: Module mẫu dùng testkit chạy integration + contract/facade test trong CI.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F003-AC-31 | `pkg/testkit` PHẢI dựng PostgreSQL 17, NATS 2.10 JetStream và Valkey 8 bằng testcontainers, chạy migration Atlas của module, tạo fixture tenant A/B, Principal theo role và context có `TenantScope` | NFR-MAINT-002 · BACKEND_DESIGN §15 | TC-TEN-005 |
| F003-AC-32 | `pkg/testkit/contract` PHẢI kiểm request/response của handler theo spec OpenAPI public, request/response object của facade (bên nhận) theo ví dụ trong spec internal, payload event theo JSON Schema, và sinh fake facade cho bên gọi (facade test, `make test-contract`); `make specs-breaking` (oasdiff) PHẢI chặn breaking change trong `v1` | NFR-MAINT-002 · ADR-0010 | TC-SVC-015 |

### Edge Cases

- NẾU NATS không truy cập được, THÌ command vẫn commit; outbox tích lũy và relay bù khi NATS lên; `/readyz` của worker báo `nats: fail` (F003-AC-18).
- NẾU hai request cùng `Idempotency-Key` đến đồng thời, THÌ đúng một request xử lý nghiệp vụ, request kia chờ ở unique index rồi nhận response đã lưu; chỉ nhận `IDEMPOTENCY_IN_PROGRESS` khi chờ vượt `lock_timeout` (F003-AC-14).
- NẾU System Admin dùng `CrossTenantScope`, THÌ interceptor lọc theo danh sách tenant đã chọn và `InTenantTx` dùng client `<module>_platform` (F003-AC-06).
- NẾU consumer nhận event có `schema_version` cao hơn version hỗ trợ, THÌ chuyển DLQ, không đoán (F003-AC-21).
- NẾU command handler gọi facade module khác bên trong `UnitOfWork.Do`, THÌ vi phạm BACKEND_DESIGN §7.4; module mẫu gọi trước khi mở transaction (review + test mẫu, không cưỡng chế bằng linter).
- Thùng rác/khôi phục bản ghi soft-delete (đọc qua `tenancy.Bypass(ctx, "trash")`) do F020/F045; F003 chỉ cung cấp `Restore` helper (`Bypass` thuộc F002).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ORG-011 | Hook soft-delete, interceptor `deleted_at IS NULL`, helper `SoftDelete`/`Restore`, `tenancyguard` | ADR-0028, MULTITENANCY §9, DATABASE_DESIGN §3.0 |
| NFR-MAINT-002 | `testkit/contract`, oasdiff trong CI | API_DESIGN §1, BACKEND_DESIGN §15 |
| NFR-MAINT-004 | `entx.UnitOfWork` + `ReadSession`, rule linter read/write, `observability.UseCase`, outbox trong transaction | BACKEND_DESIGN §4, EVENT_DESIGN §3, ADR-0087 |
| NFR-MAINT-005 | `entlint`, `Validate()` DTO trong handler mẫu | ADR-0041, DATABASE_DESIGN §1.4 |
| NFR-MAINT-006 | `pkg/modcall`, adapter `PrincipalResolver` | API_DESIGN §12, BACKEND_DESIGN §7 |
| NFR-SEC-004 | Middleware `httpx.Idempotency` + store `pkg/idempotency` (`infra.idempotency_records`) | API_DESIGN §6 |
| NFR-AVAIL-004 | Graceful shutdown `core-api`/`realtime`/`worker` | BACKEND_DESIGN §13 |
| NFR-AVAIL-005 | Deadline lời gọi facade, không fail-open | API_DESIGN §12.3 |

### Key Entities

- **Outbox event**: event nghiệp vụ chờ publish, theo envelope chuẩn, có khóa sắp xếp.
- **Processed event**: dấu đã xử lý theo (consumer, event).
- **Dead letter**: event xử lý lỗi quá số lần, chờ xử lý tay.
- **Idempotency record**: kết quả request theo (người dùng, method, route, key).
- **Unit of work**: một transaction ghi có ngữ cảnh tenant, gom ghi nghiệp vụ và outbox (write context).
- **Read session**: một transaction chỉ đọc có ngữ cảnh tenant, được dùng replica (read context).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `idempotency_record_ttl` | Hạn bản ghi idempotency |
| `module_call_deadline` | Deadline đọc/ghi/bước saga của `modcall` |
| `jetstream_duplicate_window` | Cửa sổ khử trùng `Nats-Msg-Id` khi tạo stream |
| `event_stream_retention` / `outbox_retention` | Cấu hình stream `EVT_*`; thời hạn giữ partition outbox đã publish và `processed_events` trước khi job `infra-partition-maintenance` gỡ (ADR-0089 §5) |
| `principal_cache` | Phạm vi invalidate (F002) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không I/O ngoài DB trong transaction nghiệp vụ; không gọi facade module khác trong transaction | BACKEND_DESIGN §7, §8 |
| Không event sourcing; không mediator/bus chung, không dispatch theo `any` | AGENTS R26 · ADR-0087 |
| Không auto-migrate; Atlas review | AGENTS R27 · ADR-0039 |
| Integration test không mock database | BACKEND_DESIGN §15 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 event bị mất trong kịch bản giết relay/NATS giữa chừng (100 lần lặp).
- **SC-002**: 0 lần xử lý trùng khi cùng event được giao 2 lần (TC-SVC-003).
- **SC-003**: 100% truy vấn ent không có scope bị từ chối; 0 lệnh `DELETE FROM` ngoài package purge qua `tenancyguard`.
- **SC-004**: Lặp cùng request có `Idempotency-Key` 10 lần tạo đúng 1 kết quả.
- **SC-005**: Khi module đích lỗi hoặc chậm, bên gọi trả 503 trong ≤ `module_call_deadline`; module không phụ thuộc vẫn hoạt động.

## Ngoài phạm vi

- Thùng rác, khôi phục qua API và saga platform → F020, F045; purge chat gameplay (`chat_purger`) → F117.
- `infra.tenant_resource_assignments` → F020 (learning) và các module sở hữu tài nguyên gán được.
- Stream `CHAT_MESSAGES` và đường ghi chat → F115; NATS request/reply cho lệnh realtime → F090.
- Chaos test liên module (TC-SVC-010…013) → F132. Adapter tích hợp ngoài có resilience (`external_api_resilience`) → feature sở hữu (F010/F013 Keycloak, F011/F017 SIS, F027 object storage).

## Assumptions

- Thư viện: ent + pgx/v5, Atlas, nats.go (JetStream API mới), failsafe-go, oapi-codegen client, kin-openapi, testcontainers-go (ADR-0025, ADR-0039, ADR-0040).
- Payload `audit.recorded` và các event identity dùng để invalidate cache chưa có schema → thêm ở Phase 2 (contracts §6).
- D5, D6 theo đề xuất CODE_PLAN §3 [Proposed].
