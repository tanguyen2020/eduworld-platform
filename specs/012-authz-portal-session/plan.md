# Implementation Plan: F012 — Phân quyền Permission/Role/UserRole và phiên portal (`/me/*`, `principals:resolve`)

**Branch**: `012-authz-portal-session` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Bổ sung catalog permission nhóm identity vào `src/backend/pkg/authz/catalog.go` (khung của F002), seed role hệ thống và đồng bộ `identity.permissions` bằng `src/backend/cmd/migrate`. module identity có aggregate `Role`, `UserRole`; command quản lý role/UserRole/nhân sự; query `ListMyTenants`, `GetMyPermissions`, `GetMe`; command `CreateAuthSession` (bootstrap portal). Internal `resolvePrincipal` dựng Principal từ DB, cache L1 + Valkey `ew:identity:principal:<user_id>:<tenant_id|_platform>`, invalidate bằng event. Trigger DB `role_permissions_scope_chk`, `user_roles_scope_chk` là lớp 2. Theo ADR-0091: command `PortalLogin`, `RefreshPortalToken`, `PortalLogout`, `SetupPortalPassword`, `ForgotPortalPassword`, `ResetPortalPassword` gọi Keycloak (Direct Access Grant bằng client confidential `eduworld-portal`; Admin API bằng `svc-identity`), cookie refresh HttpOnly + CSRF double-submit, `setup_ticket` trong Valkey, bảng `identity.portal_password_reset_tokens`. Theo ADR-0090: middleware `featuregate` (`src/backend/pkg/httpx`) + port `featuregate.Source` (hiện thực F015) và `disabled_menus`/`disabled_actions` trong `getMyPermissions`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `src/backend/pkg/authz`, `src/backend/pkg/httpx`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, Valkey client, JWKS cache (F002) |
| Storage | PostgreSQL `eduworld_identity` (`identity.permissions`, `roles`, `role_permissions`, `user_roles`, view `tenant_memberships_v`); Valkey cache Principal/membership |
| Testing | go test + testcontainers (PostgreSQL, Valkey, NATS, Keycloak test realm); test sinh từ OpenAPI cho authorization matrix |
| Target Platform | Linux container |
| Project Type | backend module + platform kit (`src/backend/pkg/authz` catalog) |
| Performance Goals | Đường đăng nhập portal (`listMyTenants` → `getMyPermissions` → `getMe`) p95 theo `login_slo`; `resolvePrincipal` hit cache ≥ 95% [Proposed] ([BACKEND_DESIGN §6.1](../../docs/07-backend/BACKEND_DESIGN.md)) |
| Constraints | AGENTS R7, R8, R14, R20, R27, R29, R30; ADR-0027, ADR-0065, ADR-0066, ADR-0067, ADR-0072, ADR-0081, ADR-0090, ADR-0091 |
| Scale/Scope | `principal_cache`, `principal_cache_ttl`, `portal_login_rate`, `portal_setup_ticket_ttl`, `portal_password_reset_ttl`, `feature_toggle_cache_ttl` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `PrincipalResolveRequest`, `Principal.tenant_id` nullable, `ROLE_NOT_FOUND` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Module khác chỉ gọi `resolvePrincipal`/`checkMemberships` qua facade `identity` (R29, ADR-0074) và nghe event; realtime gọi qua NATS request/reply do `core-api` xử lý |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `user_roles` có `tenant_id` (NULL chỉ scope platform) + RLS; role tùy biến RLS theo tenant; soft-delete UserRole/role |
| IV Server-authoritative | ✅ | Không có điểm/thưởng; gán/thu hồi có `Idempotency-Key` + audit |
| V Xác thực, phân quyền | ✅ | Feature này định nghĩa permission; không cấp permission platform cho role tenant (ba lớp); mật khẩu/OTP chỉ Keycloak xác thực (R30), lỗi đăng nhập chung, cookie HttpOnly + CSRF; `featuregate` chạy trước `authz.Require`, không thay authorization |
| VI An toàn trẻ em | ✅ | Học sinh không có role portal; bootstrap không PII thừa |
| VII Một nguồn sự thật, cấu hình | ✅ | Permission chỉ trong code; DB là bản sao; tham số theo CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | `/me/permissions` chỉ để dựng menu; quyết định ở backend |
| IX Test có bằng chứng | ✅ | Ma trận authorization sinh từ OpenAPI; test mọi F012-AC-* |

## Project Structure

### Documentation (feature này)

```text
specs/012-authz-portal-session/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/pkg/authz/catalog.go                  # bổ sung permission nhóm identity + role hệ thống (khung F002)
src/backend/modules/identity/
├── cmd/migrate/                                  # đồng bộ identity.permissions + seed role từ catalog
├── ent/schema/                       # permission.go, role.go, rolepermission.go, userrole.go
├── migrations/                       # trigger role_permissions_scope_chk, user_roles_scope_chk; view tenant_memberships_v (thêm nhánh user_roles)
├── internal/domain/authz/            # Role, UserRole, ScopeRule (ADR-0081, ADR-0072)
├── internal/application/command/     # PortalLogin, RefreshPortalToken, PortalLogout, SetupPortalPassword,
│                                     # ForgotPortalPassword, ResetPortalPassword (ADR-0091),
│                                     # CreateAuthSession, CreateTenantRole, UpdateTenantRole, DeleteTenantRole,
│                                     # CreatePlatformRole, UpdatePlatformRole, DeletePlatformRole, GrantUserRole,
│                                     # RevokeUserRole, ExpireUserRoles, CreateTenantUser, UpdateTenantUser
├── internal/application/query/       # ResolvePrincipal, ListMyTenants, GetMyPermissions, GetMe, ListTenantRoles,
│                                     # ListTenantPermissions, ListTenantUserRoles, ListSchoolUserRoles, ListAllUserRoles, ListTenantUsers
├── internal/infrastructure/cache/principal_cache.go   # L1 LRU + Valkey, pub/sub ew:identity:invalidate
├── internal/infrastructure/cache/setup_ticket_store.go # setup_ticket một lần (Valkey, TTL portal_setup_ticket_ttl)
├── internal/infrastructure/keycloak/portal_auth.go    # direct grant eduworld-portal, Admin API (reset, required action, execute-actions-email, logout)
├── internal/presentation/http/public/{auth_portal.go, auth_portal_login.go, me.go, roles.go, user_roles.go, tenant_users.go}
├── internal/presentation/http/internal/principals.go
├── internal/presentation/consumer/principal_invalidate.go
├── internal/presentation/job/user_role_expiry.go
└── tests/{unit,integration,contract}/
src/backend/pkg/httpx/featuregate/                # middleware featuregate + port Source + bảng route→action sinh từ x-portal-action (ADR-0090)
src/backend/cmd/core-api/                         # gắn featuregate sau portal_staff, Source = module identity (F015)
```

**Structure Decision**: Theo BACKEND_DESIGN §3, §6; D4 (catalog ở `src/backend/pkg/authz/catalog.go`), D7 (endpoint/cache key).

## Phase 0: Research

Không có research riêng — ADR-0066/0067/0081/0090/0091 và DATABASE_DESIGN §3.1 đã đủ; hành vi Keycloak direct grant (`amr`/`acr`, required action) lấy từ spike [F004 research R5](../004-keycloak-realm-apisix-gateway/research.md) [Proposed].

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `Role` (system chỉ đọc, scope kind, permission set chỉ tenant nếu role tenant), `UserRole` (scope hợp lệ, thời hạn, không `class`), `ScopeRule` (student không role portal; `primary_tenant_id` NULL ⇔ chỉ role platform) | Thuần |
| application | Command/Query ở cây trên; mỗi handler gọi tường minh `authz.Require` ở đầu use case | Permission: [contracts](contracts/README.md) §1 |
| infrastructure | ent repo, read model membership, principal cache, Keycloak (tạo user nhân sự — adapter F010), outbox | |
| presentation | `portalLogin`, `refreshPortalToken`, `portalLogout`, `setupPortalPassword`, `forgotPortalPassword`, `resetPortalPassword` (ADR-0091), `createAuthSession`, `getMe`, `logout`, `listMyTenants`, `getMyPermissions`, `listTenantPermissions`, `listTenantRoles`, `createTenantRole`, `getTenantRole`, `updateTenantRole`, `deleteTenantRole`, `listTenantUserRoles`, `grantTenantUserRole`, `revokeTenantUserRole`, `listSchoolUserRoles`, `grantSchoolUserRole`, `listPlatformRoles`, `createPlatformRole`, `getPlatformRole`, `updatePlatformRole`, `deletePlatformRole`, `listAllPermissions`, `listAllUserRoles`, `grantPlatformUserRole`, `revokePlatformUserRole`, `listTenantUsers`, `createTenantUser`, `updateTenantUser`; internal `resolvePrincipal`, `getPrincipal` (deprecated), `checkMemberships` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F010 (user, tenant, membership view, adapter Keycloak); F002 (`src/backend/pkg/authz`, middleware).
- Event phát: `role.updated`, `user_role.changed` (payload có sẵn), `audit.recorded`.
- Event tiêu thụ (invalidate cache): `user_role.changed`, `role.updated`, `tenant_membership.changed`, `user.status_changed`, `user.access_revoked`, `tenant.updated`, `enrollment.changed`.
- Facade `identity` (`src/backend/modules/identity/api`) sinh từ `identity.internal.v1.yaml` bằng `make gen`; bên gọi trong tiến trình: learning, game, competition, chat. Realtime (binary riêng, không DB) lấy Principal qua NATS request/reply do `core-api` xử lý (ADR-0074).

### Bảo mật, tenant, idempotency, audit

- Route portal: middleware `portal_staff` (`azp = eduworld-portal`, `user_type = staff`); `/admin/*`: + MFA (`acr` + `amr` chứa `otp`, `auth_time` ≤ 8 giờ) + role platform, gateway nội bộ.
- Thứ tự middleware của `core-api` (ADR-0090 mục 3): xác thực JWT → `tenant` (`X-Tenant-Id` + membership; portal không dùng claim `organization`) → `portal_staff` → `featuregate` (chỉ `azp = eduworld-portal`, operation có `x-portal-action`, bỏ qua `x-portal-shell`/`can_disable = false`) → handler (`authz.Require`). Lỗi `403 FEATURE_DISABLED` (`feature_key`); không đọc được trạng thái → `503 UNAVAILABLE`.
- `/auth/portal/*`: không token, không `X-Tenant-Id`; cookie `__Secure-ew_portal_rt` (HttpOnly, SameSite=Strict, Path `/api/v1/auth/portal`) và `__Host-ew_portal_csrf` (đọc được, Path `/`); `token:refresh`/`logout` kiểm `X-EW-CSRF` (so sánh hằng thời gian) → `403 CSRF_TOKEN_INVALID`; không `Idempotency-Key`; không log body/cookie; rate limit lớp hai `portal_login_rate` (khóa băm username + `device_id` + toàn cục, không IP). Ánh xạ lỗi Keycloak: sai mật khẩu/OTP/thiếu OTP/disabled/khóa tạm → `401 INVALID_CREDENTIALS`; bị chặn ở bước role → `403 PORTAL_ACCESS_STAFF_ONLY`; "Account is not fully set up" → `403 ACCOUNT_SETUP_REQUIRED` + `setup_ticket`.
- Secret client `eduworld-portal` chỉ ở module identity (`EW_KC_PORTAL_CLIENT_SECRET_FILE`, Docker secret `kc_portal`).
- `TenantScope` từ Principal; System Admin `CrossTenantScope` cho `listAllUserRoles`, `listPlatformRoles?tenant_ids=` + audit `cross_tenant.accessed`.
- `Idempotency-Key`: `createTenantRole`, `createPlatformRole`, `grant*UserRole`, `createTenantUser`. `If-Match` + `reason`: update/delete role, revoke UserRole.
- Audit action: `auth.portal_login` (ở `portalLogin`, thành công/thất bại), `auth.portal_logout`, `auth.portal_setup_password`, `auth.portal_password_reset_requested`, `auth.portal_password_reset`, `auth.portal_otp_setup_email_sent`, `role.created/updated/deleted`, `user_role.granted/revoked/expired`, `user.created`, `authz.denied`, `cross_tenant.accessed`.

### Quan sát

- Metric `eduworld_portal_login_total{result}`, `eduworld_portal_csrf_rejected_total`, `eduworld_featuregate_blocked_total{feature_key}`, `eduworld_featuregate_source_errors_total`, `eduworld_identity_principal_cache_hit_ratio`, `eduworld_identity_principal_resolve_duration_seconds`, `eduworld_auth_tenant_mismatch_total`, `eduworld_authz_denied_total{route}`; log không chứa token.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
