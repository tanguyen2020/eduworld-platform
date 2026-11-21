# Contracts: F051 — Portal shell: đăng nhập, chọn tenant sau đăng nhập, menu theo quyền, bộ lọc tenant

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Nguồn contract: [`specs/openapi/eduworld-api.v1.yaml`](../../openapi/eduworld-api.v1.yaml) (bundle; `specs/openapi/modules/*.yaml` sinh từ bundle).

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `portalLogin` · `refreshPortalToken` · `portalLogout` | `POST /api/v1/auth/portal/login` · `/token:refresh` · `/logout` (cùng origin, `credentials: "include"`; refresh/logout kèm `X-EW-CSRF`) | `specs/openapi/eduworld-api.v1.yaml` | công khai / cookie refresh | Có trong spec gộp (ADR-0091, backend F012) |
| `setupPortalPassword` · `forgotPortalPassword` · `resetPortalPassword` | `POST /api/v1/auth/portal/setup:password` · `/password:forgot` · `/password:reset` | như trên | `setup_ticket` / công khai / token đặt lại | Có trong spec gộp (ADR-0091) |
| `listMyTenants` | `GET /api/v1/me/tenants` (không `X-Tenant-Id`) | `specs/openapi/eduworld-api.v1.yaml` | đã đăng nhập, `azp = eduworld-portal`, `user_type = staff` | Có sẵn (delta `keycloak_org_alias` bỏ theo ADR-0091) |
| `getMyPermissions` | `GET /api/v1/me/permissions` (`X-Tenant-Id` tùy chọn) | như trên | đã đăng nhập | Có trong spec gộp: thêm `disabled_menus[]`, `disabled_actions[]` (ADR-0090) |
| — (danh mục) | `specs/portal/menu-catalog.yaml` → `make portal-catalog` → `src/generated/menu-catalog.ts` | `specs/schemas/portal-menu-catalog.schema.json` | — | Có (ADR-0090); kiểm `node scripts/specs/check-portal-catalog.mjs` |
| `getMe` | `GET /api/v1/auth/me` | như trên | đã đăng nhập | Có sẵn |
| `logout` | `POST /api/v1/auth/logout` | như trên | đã đăng nhập | Không dùng cho portal (thay bằng `portalLogout` — ADR-0091) |
| `listTenants` | `GET /api/v1/admin/tenants` (gateway nội bộ) | như trên | `platform.cross_tenant.read` | Có sẵn |
| `createAuthSession` | `POST /api/v1/auth/session` | như trên | — | Không dùng (trả `PlayerBootstrap` của game) |
| OIDC | `/realms/{realm}/protocol/openid-connect/*` | Keycloak | — | **Không dùng** từ trình duyệt (ADR-0091); chỉ trang action-token `UPDATE_PASSWORD` mở từ email (đăng ký OTP bỏ — ADR-0094) |

Mã lỗi Problem Details shell xử lý (enum `ProblemCode`, có sẵn): `UNAUTHENTICATED`, `TOKEN_EXPIRED`, `PORTAL_ACCESS_STAFF_ONLY`, `PERMISSION_DENIED`, `TENANT_HEADER_REQUIRED`, `TENANT_MISMATCH`, `TENANT_NOT_SELECTED`, `TENANT_ACCESS_DISABLED`, `ACCOUNT_NOT_PROVISIONED`, `ACCOUNT_LOCKED`, `ACCOUNT_DEACTIVATED`, `RATE_LIMITED`, `UNAVAILABLE`, `MAINTENANCE`; thêm theo ADR-0090/0091: `INVALID_CREDENTIALS`, `ACCOUNT_SETUP_REQUIRED`, `SETUP_TICKET_INVALID`, `PASSWORD_RESET_TOKEN_INVALID`, `CSRF_TOKEN_INVALID`, `REFRESH_TOKEN_INVALID`, `FEATURE_DISABLED`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không áp dụng (client chỉ gọi API public; không gọi facade nội bộ giữa module) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không áp dụng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không áp dụng (R25: client không kết nối NATS) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không áp dụng | — |

## 6. Delta cần thực hiện

- ~~**Cần sửa** `MyTenant` thêm `keycloak_org_alias`~~ — **bỏ** theo ADR-0091 mục 4 (không xin token theo scope `organization:{alias}`); task T004 hủy.
- ~~**Cần sửa** `logout` dùng `TenantIdHeaderOptional`~~ — **bỏ**: portal dùng `portalLogout` (cookie + `X-EW-CSRF`); task T005 hủy.
- ADR-0090/0091: kiểm khớp `/auth/portal/*`, `PermissionSet.disabled_*`, catalog (task T053).
- **Đã chốt (RB-115)**: portal không gọi `createAuthSession` (trả `PlayerBootstrap`); `getMyPermissions` dùng header `X-Tenant-Id` (tài liệu sở hữu đã sửa). Sửa `summary` của `createAuthSession` trong contract do F012 làm (task T038 của F012).
