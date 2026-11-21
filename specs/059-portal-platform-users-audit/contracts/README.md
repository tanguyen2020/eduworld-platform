# Contracts: F059 — Workspace D — người dùng/role platform, catalog permission, audit toàn hệ thống, login events liên tenant, feature flags, màn tích hợp

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (nguồn: `specs/openapi/modules/identity.v1.yaml`, `game.v1.yaml`). Route `/admin/*` chỉ gateway nội bộ, `azp = eduworld-portal` + role platform.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listAllPermissions` | `GET /api/v1/admin/permissions` | identity | `platform.role.read` | Có sẵn |
| `listPlatformRoles` · `getPlatformRole` | `GET /api/v1/admin/roles[/{role_id}]` | identity | `platform.role.read` | Có sẵn |
| `createPlatformRole` · `updatePlatformRole` · `deletePlatformRole` | `POST /api/v1/admin/roles` · `PATCH`/`DELETE …/{role_id}` | identity | `platform.role.write` | Có sẵn |
| `listAllUserRoles` · `grantPlatformUserRole` | `GET`/`POST /api/v1/admin/user-roles` | identity | `platform.user_role.write` (list: + `platform.cross_tenant.read`) | Có sẵn; request cần sửa (§6) |
| `revokePlatformUserRole` | `DELETE /api/v1/admin/user-roles/{user_role_id}` | identity | `platform.user_role.write` | Có sẵn |
| `listPlatformUsers` | `GET /api/v1/admin/users` | identity | `platform.user.manage` | Có sẵn (owner F012 áp 2026-11-20) |
| `createPlatformUser` | `POST /api/v1/admin/users` | identity | `platform.user.manage` (+ `platform.user_role.write`) | Có sẵn (owner F012 áp 2026-11-20) |
| `createTenantUser` | `POST /api/v1/tenant/users` (`X-Tenant-Id` = tenant đích) | identity | `tenant.user.write` (System Admin theo tenant đích) | Có sẵn |
| `lockUser` | `POST /api/v1/admin/users/{user_id}:lock` | identity | `platform.user.manage` | Có sẵn |
| `unlockUser` · `deactivateUser` | `POST /api/v1/admin/users/{user_id}:unlock` · `:deactivate` | identity | `platform.user.manage` | Cần thêm (owner F016) |
| `listPlatformAuditLogs` | `GET /api/v1/admin/audit-logs` | identity | `platform.audit.read` | Có sẵn (`action_prefix[]` áp 2026-11-20, owner F015) |
| `listLoginEventsCrossTenant` | `GET /api/v1/admin/login-events` | game | `platform.cross_tenant.read` | Có sẵn |
| `listFeatureFlags` · `putFeatureFlag` | `GET /api/v1/admin/feature-flags` · `PUT …/{flag_key}` | identity | `platform.feature_flag.write` | Cần thêm (owner F015) |
| `getIntegrationOverview` | `GET /api/v1/admin/integrations` | identity | `platform.sis_connection.manage` hoặc `platform.tenant.manage` | Có sẵn (owner F012 áp 2026-11-20; chưa có `route_groups[]` — OQ F059-12) |
| `listSisConnections` | `GET /api/v1/sis/connections` | identity | `platform.sis_connection.manage` | Có sẵn (áp 2026-11-17, owner F011) |
| `listSisSyncRuns` | `GET /api/v1/sis/connections/{connection_id}/sync-runs` | identity | `platform.sis_connection.manage` | Có sẵn (áp 2026-11-17, owner F011) |
| `listSisConnectionSchools` | `GET /api/v1/admin/sis/connections/{connection_id}/schools` | identity | `platform.sis_connection.manage` | Có sẵn |
| `listTenants` | `GET /api/v1/admin/tenants` | identity | `platform.cross_tenant.read` | Có sẵn |
| `getMyPermissions` | `GET /api/v1/me/permissions` | identity | đã đăng nhập | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (frontend) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Portal không phát/tiêu thụ; `role.updated`, `user_role.changed`, `platform_settings.changed` do module identity phát (F012, F015) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không | — |

## 6. Delta cần thực hiện

Phase 2 task T004–T010. Mọi chuỗi có `maxLength` (ADR-0041); mọi operation `/admin/*` có `x-internal: true` + `security: keycloakAdminBearer`.

- **Cần thêm** (owner F012) `listPlatformUsers` `GET /admin/users` — query `q` (minLength 2, maxLength 100), `user_type` (`student`/`staff`), `status` (`pending`/`active`/`locked`/`deactivated`), `platform_only` boolean, `role_code` `RoleCode`, `TenantIds`, `Limit`, `Cursor` → `UserPage` (mỗi `User` có `primary_tenant_id`, `tenant_ids[]` maxItems 50 membership). Không `X-Tenant-Id`; audit `cross_tenant.accessed`.
- **Cần thêm** (owner F012) `createPlatformUser` `POST /admin/users` — `Idempotency-Key`, body `PlatformUserCreateRequest` {`username` pattern `^[A-Za-z0-9._-]{3,128}$`, `email` format email maxLength 320, `display_name` maxLength 128, `locale` `Locale`, `initial_role_ids[]` (minItems 1, maxItems 20, role `platform`), `reason` maxLength 1000} → 201 `User` (`primary_tenant_id = null`), 409 `USERNAME_TAKEN`/`EMAIL_TAKEN`, 422.
- **Cần sửa** (owner F012) `UserRoleGrantRequest`: thêm `training_completed_at` (`Timestamp`, bắt buộc khi role `safety_moderator` → `422 TRAINING_REQUIRED`); bỏ `class_id` khỏi mô tả `scope_id`; `TenantUserCreateRequest.initial_user_roles[].scope_type` bỏ `class` (ADR-0072).
- **Cần thêm** (owner F016) `unlockUser`, `deactivateUser` `POST /admin/users/{user_id}:unlock|:deactivate` — body `{reason}` maxLength 1000, `Idempotency-Key`, 204.
- **Cần thêm** (owner F015) `listFeatureFlags` `GET /admin/feature-flags` → `FeatureFlagList` {`items[]` maxItems 500 `FeatureFlag` {`flag_key` pattern `^[a-z0-9_.]{3,64}$`, `description` maxLength 1000, `default_enabled`, `tenant_overrides[]` maxItems 1000 {`tenant_id`, `enabled`}, `version`, `updated_by`, `updated_at`}}; `putFeatureFlag` `PUT /admin/feature-flags/{flag_key}` body `FeatureFlagUpdate` {`default_enabled`, `tenant_overrides[]`, `reason` maxLength 1000} + `If-Match` → 200 `FeatureFlag`, 404, 412.
- **Cần sửa** (owner F015) `listPlatformAuditLogs`: thêm `x-internal: true`, `security: keycloakAdminBearer`; tham số `action` chấp nhận tiền tố (`pattern ^[a-z_.]+\*?$`, ví dụ `role.*`) hoặc thêm `action_prefix[]` maxItems 10 để lọc nhật ký phân quyền.
- **Đã áp bởi owner F011 (2026-11-17)** — dùng contract của owner: `SisConnectionPage {data[], page}`, `SisConnection` {`sis_connection_id`, `tenant_id`, `provider_kind` (`contract_v1`/`csv_import`), `contract_version`, `status`, `school_count`, `last_sync_run`, `created_at`, `updated_at`} — không có `base_url`/`credential_ref` (F011-AC-27 không trả credential/tham chiếu secret; `capabilities` do F017 thêm); `SyncRunPage {data[], page}`. Mô tả gốc của F059 (`connection_id`, `mode`, `base_url`, `credential_ref`) được thay bằng bản của owner. ~~**Cần thêm** (owner F011) `listSisConnections` `GET /sis/connections` (`TenantIds`, `Limit`, `Cursor`) → `SisConnectionPage` {`connection_id`, `tenant_id`, `mode` (`contract_v1`/`csv_import`), `base_url` maxLength 2048, `credential_ref` maxLength 128 (chỉ tham chiếu, không secret), `capabilities[]` maxItems 10, `status`, `last_sync_run` `SyncRun`}; `listSisSyncRuns` `GET /sis/connections/{connection_id}/sync-runs` → `SyncRunPage` (schema `SyncRun` có sẵn).~~
- **Cần thêm** (owner identity, đi cùng F012) `getIntegrationOverview` `GET /admin/integrations` → `IntegrationOverview` {`keycloak` {`issuer_url` maxLength 2048, `realm` maxLength 64, `clients[]` maxItems 20 {`client_id` maxLength 64, `purpose` maxLength 255}, `platform_auth_max_age_seconds`} (`platform_mfa_required` đã gỡ — ADR-0094), `apisix` {`public_base_url`, `internal_base_url` maxLength 2048, `route_groups[]` maxItems 50 {`prefix` maxLength 128, `gateway` enum `public`/`internal`, `service` maxLength 64}}, `sis` {`connections_by_status` object maxProperties 10}}; giá trị từ cấu hình server, không có secret ([ADMIN_PORTAL §1.2](../../../docs/04-admin-portal/ADMIN_PORTAL.md)).
