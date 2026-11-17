# Data Model: F015 — Audit log tập trung và cài đặt platform/tenant/trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§1.3, §2.0, §2.15, §3.11, §5)

Database `eduworld_identity`.

## 1. Entity

### AuditLog — `audit.audit_logs` (Có sẵn, TA, P tháng theo `occurred_at`)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `audit_id` | UUIDv7 | PK `(audit_id, occurred_at)` | = `event_id` của `audit.recorded` (idempotent) |
| `occurred_at` | timestamptz | NOT NULL | |
| `actor_id` | uuid NULL | | NULL = hệ thống |
| `actor_type` | `varchar(64)` | | DB hiện ghi `user`/`service`/`system`; đề xuất `student`/`staff`/`service`/`system` (Thêm giá trị, khớp ADR-0081 và OpenAPI) |
| `tenant_id` | uuid NULL | RLS TA | NULL cho thao tác platform |
| `school_id` | uuid NULL | | |
| `action` | `varchar(64)` | NOT NULL | OpenAPI `AuditLogEntry.action` đang `maxLength 128` → sửa về 64 |
| `target_type` | `varchar(64)` | NOT NULL | OpenAPI dùng tên `resource_type` → thống nhất |
| `target_id` | `varchar(128)` | NOT NULL | OpenAPI `resource_id` kiểu uuid → sửa thành chuỗi ≤ 128 |
| `result` | `varchar(64)` | CHECK `success`/`denied`/`error` | |
| `reason` | `varchar(1000)` | | |
| `metadata` | jsonb | NOT NULL `{}`, CHECK `octet_length <= 65536` | đã lọc PII |
| `trace_id` | `varchar(64)` | | |
| `ip_hash` | bytea | | |

**Index**: `(tenant_id, occurred_at DESC)`, `(target_type, target_id)`; `REVOKE UPDATE, DELETE, TRUNCATE` khỏi `identity_app`, `identity_platform`. Job `audit-retention` dùng role riêng `identity_retention` (Mới, [Proposed]) chỉ có `UPDATE (actor_id, ip_hash, metadata)` — ẩn danh hóa, không xóa; cần ghi vào DATABASE_DESIGN §1.1 (task Polish).

### PlatformSetting — `platform.platform_settings` (Có sẵn, G)

`key varchar(64)` PK (`student_cleanup`, `chat_retention`), `value jsonb`, `version int`, `updated_by`, `updated_at`. Schema `value`: `student_cleanup = {default_days, notice_days, allow_tenant_override, approval_threshold_percent}`; `chat_retention = {gameplay_chat_retention_days, minimum_days, ephemeral_retention_hours, allow_tenant_override}`.

### TenantSettings — `platform.tenants.settings` (Có sẵn, jsonb)

Khóa: `allowed_hours`, `inactive_student_cleanup_days`, `gameplay_chat_retention_days`, `default_locale`; thêm `version` (Thêm cột `settings_version int` trên `platform.tenants` cho `If-Match`).

### SchoolSettings — `org.schools.settings` (Có sẵn, jsonb)

Khóa (Mới, schema): `allowed_hours {timezone varchar(64), windows[{days[], from "HH:MM", to "HH:MM"}] ≤ 14}`, `student_club_creation` (`disabled`/`enabled`), `disabled_leaderboard_definitions varchar(64)[] ≤ 50`; thêm `settings_version int` (Thêm cột).

### FeatureFlag — `platform.feature_flags` (Có sẵn tên bảng, cột **Mới**, G; ADR-0090)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `flag_key` | `varchar(128)` | PK, CHECK `^[a-z0-9_.]{3,128}$` (khóa menu `menu.<workspace>.<menu>`, action `action.<workspace>.<menu>.<action>`) |
| `flag_kind` | `varchar(16)` | NOT NULL DEFAULT `feature`, CHECK `feature`/`menu`/`action` |
| `parent_key` | `varchar(128)` NULL | CHECK `(flag_kind = 'action') = (parent_key IS NOT NULL)`; FK → `platform.portal_menu_catalog(item_key)` |
| `catalog_key` | `varchar(128)` GENERATED (`CASE WHEN flag_kind <> 'feature' THEN flag_key END`) STORED | FK → `platform.portal_menu_catalog(item_key)` (NULL với `feature` nên không kiểm) |
| `description` | `varchar(1000)` | |
| `default_enabled` | boolean | NOT NULL |
| `tenant_overrides` | jsonb | `{tenant_id: bool}`, CHECK `octet_length <= 65536`; `inherit` = xóa khóa tenant khỏi object |
| `version` | int | `If-Match` |
| `updated_by`, `updated_at`, `deleted_at`, `deleted_by` | | |

### PortalMenuCatalogItem — `platform.portal_menu_catalog` (**Mới**, G, bản sao đọc-chỉ — ADR-0090)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `item_key` | `varchar(128)` | PK | `school_lms`, `menu.school_lms.assignments`, `action.school_lms.assignments.publish`… |
| `item_kind` | `varchar(16)` | CHECK `workspace`/`menu`/`action` | |
| `workspace_key` | `varchar(32)` | CHECK 5 workspace | |
| `parent_key` | `varchar(128)` NULL | FK tự tham chiếu | menu → workspace, action → menu |
| `path` | `varchar(256)` NULL | | chỉ menu |
| `label_key` | `varchar(128)` | NOT NULL | khóa i18n |
| `required_permissions` | `varchar(64)[]` | | any-of |
| `operation_ids` | `varchar(128)[]` | | `operationId` hoặc `operationId[param=value]` |
| `sort_order` | integer | | |
| `can_disable` | boolean | NOT NULL | `false` = mục được bảo vệ |
| `catalog_version` | `varchar(64)` | NOT NULL | |
| `synced_at` | timestamptz | | |
| `deleted_at`, `deleted_by`, `delete_reason` | | | mục bị gỡ khỏi file → soft-delete |

Chỉ `src/backend/cmd/migrate` (role migrate) ghi; `REVOKE INSERT, UPDATE, DELETE` khỏi `identity_app` (cùng mẫu `identity.permissions`).

### Trạng thái hiệu lực (không lưu bền) — Valkey `ew:fg:{tenant_id|platform}`

Tập `disabled_menus`, `disabled_actions` đã tính sẵn; TTL `feature_toggle_cache_ttl`; xóa khi `platform_settings.changed` scope `feature_flag`. Đọc bởi `featuregate.Source` (F012).

## 2. Vòng đời / trạng thái

| Entity | Chuyển trạng thái |
|---|---|
| AuditLog | ghi một lần → (hết retention) ẩn danh hóa; không xóa |
| Settings / FeatureFlag | version tăng mỗi lần ghi; soft-delete flag (kể cả khi mục catalog bị gỡ) |
| PortalMenuCatalogItem | `src/backend/cmd/migrate` upsert theo `catalog_version` → soft-delete khi bị gỡ khỏi file |

## 3. Quy tắc kiểm tra dữ liệu

- Khóa cấm trong `metadata`: `password`, `refresh_token`, `access_token`, `login_ticket`, `body`, `message`, `full_name`, `date_of_birth`, `email`, `username` (giá trị thô).
- Giá trị hiệu lực: tenant nếu `allow_tenant_override`, ngược lại platform ([DATABASE_DESIGN §2.0](../../docs/07-backend/DATABASE_DESIGN.md)).
- Cài đặt cấp dưới chỉ siết chặt (ADR-0020); `gameplay_chat_retention_days` ≥ tối thiểu.
- Flag menu/action: khóa có trong catalog; `can_disable = false` không tắt được (`422 FEATURE_NOT_DISABLEABLE`); `inherit` chỉ với `scope = tenant`; action hiệu lực = flag action AND flag menu cha; không dòng = bật.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `audit.recorded` | Mọi module, mọi thao tác cần audit | `specs/events/payloads/common/audit.recorded.v1.schema.json` — Có sẵn (áp 2026-11-17) (owner F003 T004) |
| `platform_settings.changed` | Ghi cài đặt platform/tenant/trường/flag (flag menu/action: `scope = feature_flag`, `flag_key`, `tenant_id?`) | Có sẵn (áp 2026-11-17) (thêm `scope = school`, `school_id?`, `feature_flag`) — schema do đợt cập nhật event sở hữu |
| `tenant.updated`, `school.updated` | Đổi cài đặt tenant/trường | Schema owner F010 (T004) — F015 phát cùng schema |
