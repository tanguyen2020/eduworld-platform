# Data Model: F012 — Phân quyền Permission/Role/UserRole và phiên portal (`/me/*`, `principals:resolve`)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§2.1, §3.1)

Database `eduworld_identity`.

## 1. Entity

### Permission — `identity.permissions` (Có sẵn, G, bản sao đọc-chỉ)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `permission_code` | `varchar(64)` | PK | vd. `tenant.user_role.write` |
| `scope_kind` | `varchar(64)` | CHECK `tenant`/`platform`; tiền tố `game.`, `competition.config.`, `platform.` bắt buộc `platform` | |
| `group_code` | `varchar(64)` | | nhóm theo workspace |
| `name_vi`, `name_en` | `varchar(255)` | | |
| `catalog_version` | `varchar(64)` | | |
| `synced_at` | timestamptz | | `REVOKE INSERT, UPDATE, DELETE` khỏi `identity_app` |

### Role — `identity.roles` (Có sẵn, G/T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `role_id` | UUIDv7 | PK | |
| `code` | `varchar(64)` | UNIQUE `(code) WHERE tenant_id IS NULL`; UNIQUE `(tenant_id, code) WHERE tenant_id IS NOT NULL` (partial `deleted_at IS NULL`) | |
| `name_vi`, `name_en` | `varchar(255)` | | |
| `description` | `varchar(1000)` | | |
| `role_scope_kind` | `varchar(64)` | CHECK `tenant`/`platform`; `tenant` hoặc `tenant_id IS NULL` | |
| `is_system` | boolean | CHECK `NOT is_system OR tenant_id IS NULL`; trigger chặn UPDATE/DELETE bởi `identity_app` | |
| `tenant_id` | uuid NULL | RLS: dòng có `tenant_id` chỉ thấy trong tenant đó | role tùy biến [Confirmed] |
| `version` | integer | `If-Match` | |
| `created_by`, `updated_by`, `deleted_at`, `deleted_by` | | | |

### RolePermission — `identity.role_permissions` (Có sẵn)

`role_id`, `permission_code` (FK `permissions`), `tenant_id` (= `roles.tenant_id`); PK `(role_id, permission_code)`; trigger `role_permissions_scope_chk`.

### UserRole — `identity.user_roles` (Có sẵn, T*)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `user_role_id` | UUIDv7 | PK |
| `user_id`, `role_id` | uuid | FK |
| `tenant_id` | uuid NULL | CHECK `(scope_type='platform') = (tenant_id IS NULL)` |
| `scope_type` | `varchar(64)` | CHECK `platform`/`tenant`/`school`/`subject` |
| `scope_id`, `school_id` | uuid | |
| `valid_from`, `valid_to` | timestamptz | CHECK `valid_to > valid_from` |
| `granted_by` | uuid | |
| `source` | `varchar(64)` | `sis`/`manual` (Release 1.0 chỉ `manual`) |
| `training_completed_at` | timestamptz | bắt buộc với `safety_moderator` (kiểm ở use case) |
| `deleted_at`, `deleted_by` | | |

**Index**: `user_roles_active_uq (user_id, role_id, scope_type, scope_id) WHERE deleted_at IS NULL AND valid_to IS NULL`; `(user_id, tenant_id)`; `(tenant_id, role_id)`; trigger `user_roles_scope_chk` (DDL §3.1).

### TenantMembership — view `identity.tenant_memberships_v` (Có sẵn, sửa)

F012 bổ sung nhánh nhân sự: user có UserRole hiệu lực (`deleted_at IS NULL`, `valid_from ≤ now()`, `valid_to IS NULL OR valid_to > now()`) trong `tenant_id`. Đúng một dòng mỗi `(tenant_id, user_id)`.

### PortalPasswordResetToken — `identity.portal_password_reset_tokens` (Mới, ADR-0091; bảng do [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) sở hữu — nếu khác, theo DATABASE_DESIGN)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `reset_token_id` | UUIDv7 | PK | |
| `user_id` | uuid | FK `identity.users`, chỉ `user_type = staff` (kiểm ở use case) | |
| `token_hash` | `char(64)` | UNIQUE | SHA-256 hex của token ngẫu nhiên ≥ 32 byte; không lưu token thô |
| `expires_at` | timestamptz | NOT NULL | = `created_at` + `portal_password_reset_ttl` |
| `used_at` | timestamptz NULL | | đặt nguyên tử: `UPDATE … SET used_at = now() WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now() AND deleted_at IS NULL` |
| `requested_device_hash` | `char(64)` NULL | | băm `device_id`, phục vụ điều tra (không IP thô) |
| `created_at` | timestamptz | | |
| `deleted_at`, `deleted_by`, `delete_reason` | | | soft-delete token chưa dùng khi phát token mới (`delete_reason = superseded`) hoặc khi user bị khóa |

**Index**: `portal_password_reset_tokens_active_uq (user_id) WHERE used_at IS NULL AND deleted_at IS NULL`; `(expires_at)`. Không xóa vật lý (ADR-0028); hết hạn thì giữ dòng (không còn dùng được).

### SetupTicket (không lưu bền) — Valkey `ew:identity:portal_setup:<sha256(ticket)>`

`{user_id, required_steps[], otp_email_sent_at?}`; TTL `portal_setup_ticket_ttl`; xóa bằng `GETDEL` khi dùng (một lần); không chứa mật khẩu; hủy khi `user.status_changed`.

### Phiên portal trên trình duyệt (không lưu bền ở EduWorld)

Refresh token do Keycloak quản lý (client session `eduworld-portal`, `portal_session_sso`), chỉ nằm trong cookie `__Secure-ew_portal_rt`; CSRF ngẫu nhiên 32 byte trong cookie `__Host-ew_portal_csrf`, xoay vòng khi đăng nhập/refresh. Module identity không lưu bảng phiên portal.

### Trạng thái bật/tắt hiệu lực (đọc, không sở hữu)

Đọc qua port `featuregate.Source` từ `platform.feature_flags` + `platform.portal_menu_catalog` (F015, [data-model F015](../015-audit-settings/data-model.md)); cache Valkey `ew:fg:{tenant_id|platform}`, TTL `feature_toggle_cache_ttl`.

### Principal (không lưu bền) — cache Valkey `ew:identity:principal:<user_id>:<tenant_id|_platform>`

Nội dung theo schema `Principal` của `identity.internal.v1.yaml`; TTL `principal_cache_ttl`; L1 in-process theo `principal_cache`. Valkey lỗi → đọc DB (bulkhead).

## 2. Vòng đời / trạng thái

| Entity | Chuyển trạng thái |
|---|---|
| Role tùy biến | tạo → sửa (version++) → soft-delete (chỉ khi không còn UserRole hiệu lực) |
| UserRole | hiệu lực (`valid_from`) → hết hạn (`valid_to`, job phát `op = expired`) / thu hồi (soft-delete, `op = revoked`) |
| PortalPasswordResetToken | tạo → dùng (`used_at`) / hết hạn (`expires_at`) / bị thay (soft-delete `superseded`) |
| SetupTicket | phát (Valkey) → dùng một lần (`GETDEL`) / hết TTL |

## 3. Quy tắc kiểm tra dữ liệu

- Bất biến ba lớp: `ValidateCatalog` lúc khởi động · use case `CreateRole`/`UpdateRole` · trigger ([ADMIN_PORTAL §2.1](../../docs/04-admin-portal/ADMIN_PORTAL.md)).
- ADR-0081: `primary_tenant_id IS NULL` ⇔ chỉ role platform; học sinh không nhận role portal.
- Không scope `class` (ADR-0072).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `role.updated` | Tạo/sửa/xóa role | `specs/events/payloads/identity/role.updated.v1.schema.json` (có sẵn) |
| `user_role.changed` | Gán/thu hồi/hết hạn/sửa UserRole | `specs/events/payloads/identity/user_role.changed.v1.schema.json` (có sẵn) |
| `audit.recorded` | Mọi thao tác ghi, đăng nhập portal (`auth.portal_login` ở `portalLogin`), đặt/đặt lại mật khẩu, `authz.denied`, `cross_tenant.accessed` | Schema chung `specs/events/payloads/common/audit.recorded.v1.schema.json` (owner F003) |
