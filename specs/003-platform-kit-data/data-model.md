# Data Model: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Bốn bảng `infra` có trong **mỗi** database module (DATABASE_DESIGN §1 nguyên tắc 13, §2.17). Định nghĩa ent một lần ở `pkg/entx/infraschema`, mỗi module nhúng và sinh migration Atlas riêng. Bảng hạ tầng: **không** `tenant_id` bắt buộc, **không** RLS, append-only/không soft-delete; không role ứng dụng nào có `DELETE`/`TRUNCATE`. Quyền (ADR-0089 §4): `<module>_app` `INSERT` trên outbox, `INSERT`/`SELECT` trên processed/idempotency, `INSERT` dead letters; `<module>_relay` `SELECT, UPDATE (published_at, attempts)` trên outbox; `<module>_platform` `SELECT, UPDATE (resolved_at, resolved_by)` dead letters. Retention (ADR-0089 §5): cả bốn bảng partition theo ngày; job `infra-partition-maintenance` (role `<module>_partition_admin`) gỡ partition hết hạn, từ chối partition còn outbox chưa publish hoặc dead letter chưa xử lý.

## 1. Entity

### OutboxEvent — `infra.outbox_events` (Có sẵn, DATABASE_DESIGN §3.9)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `event_id` | UUID / `uuid` | PK (1) | UUIDv7, = `Nats-Msg-Id` |
| `event_type` | `MaxLen(64)` / `varchar(64)` | NOT NULL | `<entity>.<past_verb>` |
| `subject` | `MaxLen(128)` / `varchar(128)` | NOT NULL | `ew.evt.<module>.<event_type>` |
| `tenant_id` | UUID nullable | | NULL = event platform |
| `partition_key` | `MaxLen(128)` / `varchar(128)` | NOT NULL | khóa sắp xếp |
| `envelope` | JSON / `jsonb` | NOT NULL, CHECK `octet_length(envelope::text) <= 65536` | EVENT_DESIGN §2 |
| `created_at` | time / `timestamptz` | PK (2), NOT NULL default now() | cột partition (`PARTITION BY RANGE`, theo ngày) |
| `published_at` | time nullable | | đặt sau ack |
| `attempts` | int | NOT NULL default 0 | |

**Index**: `outbox_unpublished_idx (event_id) WHERE published_at IS NULL`. **Partition**: theo ngày trên `created_at`, DDL viết tay trong `migrations/partitions/` (DATABASE_DESIGN §7.1).

### ProcessedEvent — `infra.processed_events` (Có sẵn, DATABASE_DESIGN §3.9)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `consumer` | `MaxLen(128)` / `varchar(128)` | PK (1) | `<module>.<handler>` |
| `event_id` | `uuid` | PK (2) | |
| `event_occurred_at` | `timestamptz` | PK (3), NOT NULL | `occurred_at` của envelope, bất biến với mỗi event; cột partition theo ngày (event giao lại luôn vào cùng partition) |
| `processed_at` | `timestamptz` | NOT NULL default now() | |

### DeadLetter — `infra.dead_letters` (Có sẵn, DDL DATABASE_DESIGN §3.9)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `dead_letter_id` | UUID | PK (1) | UUIDv7 |
| `stream` | `MaxLen(64)` / `varchar(64)` | NOT NULL | `EVT_<SVC>` |
| `consumer` | `MaxLen(128)` / `varchar(128)` | NOT NULL, UNIQUE (`consumer`, `event_id`, `created_at`) | |
| `event_id` | `uuid` | NOT NULL | |
| `event_type` | `MaxLen(64)` / `varchar(64)` | NOT NULL | |
| `payload` | `jsonb` | NOT NULL, CHECK `octet_length(payload::text) <= 65536` | envelope gốc |
| `error` | `MaxLen(1000)` / `varchar(1000)` | NOT NULL | không PII |
| `delivery_count` | int | NOT NULL | |
| `created_at` | `timestamptz` | PK (2), NOT NULL default now() | cột partition theo ngày |
| `resolved_at` | `timestamptz` nullable | | |
| `resolved_by` | `uuid` nullable | | |

### IdempotencyRecord — `infra.idempotency_records` (Có sẵn, DATABASE_DESIGN §3.10)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `user_id` | `uuid` | PK (1) | Principal (người dùng hoặc job hệ thống) |
| `method` | `MaxLen(16)` / `varchar(16)` | NOT NULL | không thuộc PK (ADR-0089 §4) |
| `route_template` | `MaxLen(255)` / `varchar(255)` | PK (2) | template, không path thật |
| `idem_key` | `uuid` | PK (3) | header `Idempotency-Key`, bắt buộc UUIDv7 |
| `key_issued_on` | `date` | PK (4) | ngày UTC lấy từ timestamp trong UUIDv7 của key; cột partition theo ngày |
| `request_hash` | `bytea` | NOT NULL, CHECK `octet_length = 32` (đề xuất) | SHA-256 |
| `response_status` | smallint | NOT NULL | |
| `response_body` | `jsonb` nullable | CHECK `octet_length(response_body::text) <= 65536` | response đã tuần tự hóa |
| `created_at` | `timestamptz` | NOT NULL default now() | |
| `expires_at` | `timestamptz` | NOT NULL | `created_at + idempotency_record_ttl` |

**Index**: `idempotency_expires_idx (expires_at)`.

### Mixin ent (không phải bảng)

| Mixin | Trường | Ghi chú |
|---|---|---|
| `entx.Base` (T) | `id`, `tenant_id` (immutable), `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`, `deleted_by`, `delete_reason` `MaxLen(64)`, `version` int64 | BACKEND_DESIGN §5.2; interceptor `tenant_id IN (scope) AND deleted_at IS NULL` + hook |
| `entx.TenantAppendOnly` (TA) | `id`, `tenant_id`, `created_at`, `created_by` | không `deleted_at`; interceptor chỉ lọc tenant |
| `entx.TenantOptional` (T\*) | như Base nhưng `tenant_id` nullable | phiên tenant: như Base; dòng `tenant_id IS NULL` chỉ qua `PlatformScope` (`user_roles`, `account_locks`) |
| `entx.HybridContent` (H, tên cũ `PlatformBase`) | như Base nhưng `tenant_id` nullable + `owner_scope` `MaxLen(64)` (`platform`/`tenant`) | dữ liệu platform/gán (ADR-0029, D8); interceptor có nhánh gán qua `infra.tenant_resource_assignments` |
| `entx.GlobalBase` (G) | `id`, `created_at/by`, `updated_at/by`, `deleted_at/by`, `delete_reason`, `version` | không `tenant_id`; interceptor chỉ `deleted_at IS NULL`, không cần scope |
| `entx.AppendOnly` (GA) | `id`, `created_at`, `created_by` | không `tenant_id`, không `deleted_at`; không filter, không cần scope |

Mỗi entity khai báo đúng một mixin; test `entx/mixin_coverage_test.go` liệt kê mọi schema ent và fail nếu thiếu (ADR-0089 §2). Bảng `infra.*` dùng `infraschema` riêng, không mixin nghiệp vụ.

`entx.UnitOfWork` (write context) và `entx.ReadSession` (read context, ADR-0087) không có bảng riêng: chỉ mở transaction trên các bảng trên và bảng nghiệp vụ của module.

## 2. Vòng đời / trạng thái

| Bảng | Chuyển trạng thái |
|---|---|
| `outbox_events` | `published_at IS NULL` → (relay ack) → `published_at` đặt; `attempts` tăng mỗi lần lỗi |
| `idempotency_records` | Không có vòng đời: `INSERT` một lần trong cùng transaction `UnitOfWork` với thay đổi nghiệp vụ (kèm response); rollback → không có bản ghi; không trạng thái `in_progress`, không `UPDATE` (ADR-0089 §4) |
| `dead_letters` | `resolved_at IS NULL` → `resolved_at` đặt khi xử lý tay (role `<module>_platform`) |

## 3. Quy tắc kiểm tra dữ liệu

- Envelope hợp lệ theo `specs/events/envelope.schema.json` trước khi ghi outbox (F003-AC-17).
- `Idempotency-Key` phải là UUIDv7, timestamp lệch ≤ `idempotency_record_ttl` so với giờ server; nếu không → `400 BAD_REQUEST` (F003-AC-37).
- Không `DELETE` trên bảng `infra` từ ứng dụng; retention bằng gỡ partition ngày (job `infra-partition-maintenance`, `outbox_retention`, `idempotency_record_ttl`, retention dead letter của EVENT_DESIGN §8), từ chối gỡ partition còn outbox chưa publish hoặc dead letter chưa xử lý; ngoại lệ có giới hạn của ADR-0070 (ADR-0089 §5).
- Migration thêm `REVOKE DELETE, TRUNCATE ON infra.* FROM <module>_app, <module>_platform, <module>_relay` (kế thừa baseline F001), `GRANT` theo ADR-0089 §4, tạo role `<module>_relay`, `<module>_partition_admin`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `audit.recorded` | Mọi lần `audit.Recorder.Record` | `specs/events/payloads/common/audit.recorded.v1.schema.json` (Cần thêm, T004) — owner F003; mọi module phát cùng schema |
| `<entity>.deleted` | `entx.SoftDelete` | Theo feature sở hữu entity (`specs/events/payloads/<module>/…`) |
