# Data Model: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§2.3)

Database `eduworld_identity`, schema `sis`. F011 tạo các bảng; F017 chỉ **thêm cột** và dùng các cột F011 đã tạo sẵn cho `contract_v1` (`config`, `secret_ref`, `delta_watermark`, `schedule`). Mọi cột mới cần task cập nhật DATABASE_DESIGN §2.3.

## 1. Entity

### SISConnection — `sis.sis_connections` (Có sẵn từ F011, T; Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `provider_kind` | `varchar(64)` | `contract_v1` (F017) / `csv_import` (F011) | có sẵn |
| `contract_version` | `varchar(64)` (`MaxLen(64)`) | pattern `^1\.[0-9]+\.[0-9]+$` khi `contract_v1` | có sẵn |
| `config` | jsonb | CHECK `octet_length(config::text) <= 65536`; bắt buộc khi `contract_v1` | có sẵn; khóa: `base_url` (≤ 2048, https), `auth_type` (`oauth2_client_credentials`/`api_key`/`mtls`), `token_url` (≤ 2048), `oauth_scope` (≤ 255, mặc định `eduworld.sis.read`), `schools[]` (`school_external_id` ≤ 64, maxItems 500) — không secret, không field-mapping |
| `secret_ref` | `varchar(512)` | NOT NULL khi `contract_v1` | có sẵn |
| `webhook_secret_ref` | `varchar(512)` (`MaxLen(512)`) | NULL | **Thêm cột**; bắt buộc để nhận webhook |
| `capabilities` | `varchar(64)[]` | CHECK `cardinality <= 4`, phần tử ∈ `full_sync`/`delta`/`webhook`/`student_lookup` | **Thêm cột**; cập nhật từ `getHealth` |
| `last_health_at` | timestamptz | NULL | **Thêm cột** |
| `last_success_at` | timestamptz | NULL | **Thêm cột**; nguồn metric `eduworld_sis_last_success_timestamp` |
| `consecutive_failed_runs` | int | NOT NULL DEFAULT 0, ≥ 0 | **Thêm cột**; circuit breaker |
| `status_reason` | `varchar(64)` | NULL | **Thêm cột**; `version_mismatch`/`violation_threshold`/`credential_rejected`/`secret_unavailable`/`circuit_open`/`manual` |
| `status` | `varchar(64)` | `active`/`auth_error`/`contract_error`/`paused` | có sẵn |
| `delta_watermark` | `varchar(512)` | NULL | có sẵn; chỉ ghi sau apply thành công |
| `schedule` | `varchar(64)` | NULL = theo `sis_sync_schedule` của tenant | có sẵn; cron 5 trường |

**Index**: có sẵn từ F011; thêm idx `(status, provider_kind) WHERE deleted_at IS NULL` cho scheduler.

### SyncRun — `sis.sync_runs` (Có sẵn từ F011, G; Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `mode` | `varchar(64)` | `full`/`delta`/`webhook`/`csv_import` | có sẵn |
| `status` | `varchar(64)` | `running`/`succeeded`/`partially_failed`/`failed` | có sẵn |
| `dry_run` | boolean | NOT NULL DEFAULT false | **Thêm cột** |
| `trigger` | `varchar(64)` | `schedule`/`manual`/`webhook`/`fallback_full` | **Thêm cột** |
| `requested_by` | uuid | NULL (lịch/webhook) | **Thêm cột** |
| `delivery_id` | uuid | NULL; chỉ `mode = webhook` | **Thêm cột** |
| `failure_code` | `varchar(64)` | NULL | **Thêm cột**; `SIS_AUTH_FAILED`, `SIS_CONTRACT_VERSION_UNSUPPORTED`, `SIS_CONTRACT_VIOLATION_THRESHOLD`, `SIS_UNAVAILABLE`, `SIS_PAGINATION_INVALID`, `SIS_WATERMARK_EXPIRED` |
| `stats` | jsonb | | có sẵn; thêm khóa `http_requests`, `retries`, `unknown_fields`, `health_status` |

**Index**: thêm idx `(sis_connection_id, started_at DESC)`.

### Không tạo bảng mới

Nonce webhook và lock run ở Valkey (research R2); job ở JetStream `IDENTITY_SIS_JOBS`; staging/lỗi/issue ở bảng F011.

## 2. Vòng đời / trạng thái

| Từ | Sự kiện | Tới |
|---|---|---|
| `active` | 401/403 hoặc không đọc được secret | `auth_error` |
| `active` | major version khác / vượt ngưỡng vi phạm | `contract_error` |
| `active` | `consecutive_failed_runs` đạt ngưỡng | `paused` (`status_reason = circuit_open`) |
| `active` | System Admin tạm dừng | `paused` (`manual`) |
| `auth_error`/`contract_error`/`paused` | System Admin kích hoạt lại (`updateSisConnection status = active`) + health check đạt | `active`, `consecutive_failed_runs = 0` |

SyncRun: `running` → `succeeded` / `partially_failed` (có lỗi bản ghi, dưới ngưỡng) / `failed`.

## 3. Quy tắc kiểm tra dữ liệu

- `contract_v1` ⇒ `config.base_url` https, `secret_ref` NOT NULL; `oauth2_client_credentials` ⇒ `token_url` NOT NULL.
- `config.schools[]` chỉ chứa `school_external_id`; ánh xạ sang trường nội bộ ở `sis_connection_schools` + `sis_links` (F011); không suy ra tenant từ `organization_external_id`.
- Watermark chỉ tiến khi mọi lô apply thành công; 410 không đổi watermark.
- Một connection một run `running` (lock Valkey; kiểm thêm trạng thái DB khi lấy lock).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `sis.sync_completed` | Kết thúc run full/delta/webhook | `specs/events/payloads/identity/sis.sync_completed.v1.schema.json` — owner F011 (T005); F017 thêm trường tương thích `mode`, `status`, `dry_run`, `connection_status`, `failure_code` |
| `audit.recorded` | Tạo/sửa connection, đổi trạng thái, mỗi run | theo F015 (audit tập trung) |
