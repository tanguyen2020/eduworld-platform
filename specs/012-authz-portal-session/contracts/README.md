# Contracts: F012 — Phân quyền Permission/Role/UserRole và phiên portal (`/me/*`, `principals:resolve`)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/identity.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `portalLogin` | `POST /api/v1/auth/portal/login` (không token, không `X-Tenant-Id`) | identity.v1.yaml | công khai (rate `portal_login_rate`) | Mới (ADR-0091) — `PortalLoginRequest` → `PortalTokenResponse` + `Set-Cookie`; lỗi `401 INVALID_CREDENTIALS`, `403 ACCOUNT_SETUP_REQUIRED`, `403 PORTAL_ACCESS_STAFF_ONLY`, `403 ACCOUNT_DEACTIVATED`, `429 RATE_LIMITED` |
| `refreshPortalToken` | `POST /api/v1/auth/portal/token:refresh` | identity.v1.yaml | cookie refresh + `X-EW-CSRF` | Mới — `401 REFRESH_TOKEN_INVALID`, `403 CSRF_TOKEN_INVALID` |
| `portalLogout` | `POST /api/v1/auth/portal/logout` | identity.v1.yaml | cookie refresh + `X-EW-CSRF` | Mới — `204`, `403 CSRF_TOKEN_INVALID` |
| `setupPortalPassword` | `POST /api/v1/auth/portal/setup:password` | identity.v1.yaml | `setup_ticket` | Mới — `PortalSetupResult`; `401 SETUP_TICKET_INVALID`, `422 VALIDATION_FAILED` (`PASSWORD_POLICY_VIOLATION`) |
| `forgotPortalPassword` | `POST /api/v1/auth/portal/password:forgot` | identity.v1.yaml | công khai | Mới — luôn `202` |
| `resetPortalPassword` | `POST /api/v1/auth/portal/password:reset` | identity.v1.yaml | token đặt lại | Mới — `204`; `401 PASSWORD_RESET_TOKEN_INVALID`, `422 VALIDATION_FAILED` |
| `createAuthSession` | `POST /api/v1/auth/session` (không `X-Tenant-Id`) | identity.v1.yaml | nhân sự `eduworld-portal` (`portal_staff`) | Có sẵn — **không** thuộc luồng đăng nhập portal (trả `PlayerBootstrap`); portal dùng `listMyTenants` → `getMyPermissions` → `getMe` (RB-115) |
| `getMe` | `GET /api/v1/auth/me` | identity.v1.yaml | đã xác thực | Có sẵn |
| `logout` | `POST /api/v1/auth/logout` | identity.v1.yaml | đã xác thực | Có sẵn |
| `listMyTenants` | `GET /api/v1/me/tenants` (không `X-Tenant-Id`) | identity.v1.yaml | đã xác thực | Có sẵn |
| `getMyPermissions` | `GET /api/v1/me/permissions` | identity.v1.yaml | nhân sự portal | Cần sửa (ADR-0090) — `PermissionSet` thêm `disabled_menus[]`, `disabled_actions[]` (chuỗi ≤ 128) |
| `listTenantPermissions` | `GET /api/v1/tenant/permissions` | identity.v1.yaml | `tenant.role.read` | Có sẵn |
| `listTenantRoles` · `getTenantRole` | `GET /api/v1/tenant/roles[/{role_id}]` | identity.v1.yaml | `tenant.role.read` | Có sẵn |
| `createTenantRole` · `updateTenantRole` · `deleteTenantRole` | `POST /api/v1/tenant/roles` · `PATCH/DELETE /api/v1/tenant/roles/{role_id}` | identity.v1.yaml | `tenant.role.write` | Có sẵn |
| `listTenantUserRoles` · `grantTenantUserRole` · `revokeTenantUserRole` | `GET/POST /api/v1/tenant/user-roles` · `DELETE /api/v1/tenant/user-roles/{user_role_id}` | identity.v1.yaml | `tenant.user_role.write` (list: `tenant.user.read`) | Có sẵn |
| `listSchoolUserRoles` · `grantSchoolUserRole` | `GET/POST /api/v1/schools/{school_id}/user-roles` | identity.v1.yaml | `tenant.user_role.write` (scope trường) | Có sẵn |
| `listTenantUsers` · `createTenantUser` · `updateTenantUser` | `GET/POST /api/v1/tenant/users` · `PATCH /api/v1/tenant/users/{user_id}` | identity.v1.yaml | `tenant.user.read` / `tenant.user.write` | Có sẵn |
| `listPlatformRoles` · `getPlatformRole` | `GET /api/v1/admin/roles[/{role_id}]` | identity.v1.yaml | `platform.role.read` | Có sẵn |
| `createPlatformRole` · `updatePlatformRole` · `deletePlatformRole` | `/api/v1/admin/roles*` | identity.v1.yaml | `platform.role.write` | Có sẵn |
| `listAllPermissions` | `GET /api/v1/admin/permissions` | identity.v1.yaml | `platform.role.read` | Có sẵn |
| `listAllUserRoles` · `grantPlatformUserRole` · `revokePlatformUserRole` | `/api/v1/admin/user-roles*` | identity.v1.yaml | `platform.user_role.write` (list: + `platform.cross_tenant.read`) | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `resolvePrincipal` | learning/game/competition/chat → identity (qua adapter `PrincipalResolver` của F003; `realtime` không gọi facade — lệnh WS được `core-api` dựng Principal) | identity.internal.v1.yaml | learning, game, competition, chat | Cần sửa (request/response; F012 sở hữu nội dung, áp sớm ở F002 T005) |
| `getPrincipal` | như trên (deprecated) | identity.internal.v1.yaml | như trên | Có sẵn |
| `checkMemberships` | learning/game/competition/chat → identity | identity.internal.v1.yaml | learning, game, competition, chat | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `role.updated` | identity → mọi module (invalidate), worker identity (`worker-events`) | `specs/events/payloads/identity/role.updated.v1.schema.json` | Có sẵn |
| `user_role.changed` | identity → mọi module, worker Keycloak (F010) | `specs/events/payloads/identity/user_role.changed.v1.schema.json` | Có sẵn |
| `audit.recorded` | identity → audit sink (F015) | `specs/events/payloads/common/audit.recorded.v1.schema.json` | Cần thêm (F003 T004) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- `identity.internal.v1.yaml` `PrincipalResolveRequest`: thêm `issuer` (maxLength 512), `subject` (maxLength 255) là cách định danh chính (oneOf với `user_id`), giữ `client_id`; `tenant_id` nullable khi phạm vi platform. `Principal.tenant_id`, `tenant_type` nullable; thêm `is_system_admin` (boolean); response `404` = không membership tenant yêu cầu (bên gọi ánh xạ `TENANT_MISMATCH`). F012 là owner duy nhất nội dung operation này; F002 T005 áp sớm ở M-C0 đúng nội dung này, F003 (adapter) và F002 (port `authz.PrincipalResolver`) chỉ tiêu thụ. Cập nhật contract test với mọi bên gọi.
- `specs/openapi/eduworld-api.v1.yaml` `ProblemCode`: thêm `ROLE_NOT_FOUND` (đang được API_DESIGN §9.16 và ADMIN_PORTAL §2.3 dùng nhưng chưa có trong enum).
- `createAuthSession` (`identity.v1.yaml`, spec gộp): sửa `summary` "Bootstrap sau đăng nhập portal" thành bootstrap cho client có hồ sơ người chơi; ghi rõ Admin Portal không gọi (API_DESIGN `/auth/session`, RB-115). Task T038.
- Spec gộp `eduworld-api.v1.yaml` (owner contract chung theo ADR-0090/0091, F012 kiểm khớp — task T039): 6 operation `/auth/portal/*` (tag `auth`, `x-service: identity`, `x-portal-shell: true`, `security: []`, mọi chuỗi có `maxLength`, mô tả cookie `__Secure-ew_portal_rt`/`__Host-ew_portal_csrf` và header `X-EW-CSRF`); `PermissionSet.disabled_menus`/`disabled_actions`; `ProblemCode` thêm `ACCOUNT_SETUP_REQUIRED`, `SETUP_TICKET_INVALID`, `PASSWORD_RESET_TOKEN_INVALID`, `CSRF_TOKEN_INVALID`, `FEATURE_DISABLED`; mọi operation portal có `x-portal-action` hoặc `x-portal-shell` (đầu vào bảng route → action của `featuregate`, sinh bằng `make gen`).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity`, `make test-contract`, `node scripts/specs/check-portal-catalog.mjs`.
