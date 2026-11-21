# Implementation Plan: F051 — Portal shell: đăng nhập, chọn tenant sau đăng nhập, menu theo quyền, bộ lọc tenant

> **ADR-0094 (2026-11-21, owner bỏ MFA/TOTP cho đăng nhập Admin Portal):** mọi nội dung về OTP/TOTP, mapper AMR/LoA, `acr`/`amr`, `MFA_REQUIRED`, step-up, `configure_otp`, `otp_setup_email_sent`, guard `platform_mfa` trong file này là **lịch sử, không còn hiệu lực**. Hiện hành: [spec.md — Clarifications, Session 2026-11-21](spec.md) và [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md).

**Branch**: `051-portal-shell` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Dựng khung ứng dụng `src/cms/frontend/apps/portal`: trang `/login` tự vẽ + màn thiết lập tài khoản, quên/đặt lại mật khẩu gọi `/api/v1/auth/portal/*` (ADR-0091; access token trong bộ nhớ, refresh token trong cookie HttpOnly do backend đặt, CSRF double-submit `X-EW-CSRF`, làm mới single-flight), máy trạng thái phiên (`restoring → unauthenticated → resolving_scope → selecting_tenant → ready | no_access`), định tuyến sau đăng nhập theo `listMyTenants`, `ProLayout` dựng workspace switcher/sidebar từ danh mục sinh từ `specs/portal/menu-catalog.yaml` lọc theo `getMyPermissions` (`permissions`, `workspaces`, `disabled_menus`, `disabled_actions` — ADR-0090), `ActionGate`, route workspace lazy (`React.lazy` + `import()` theo `workspaces[]`), trang 403/"Tính năng đang tắt"/VPN/hết phiên, đổi đơn vị bằng `X-Tenant-Id` (không token mới), bộ lọc tenant System Admin (`listTenants`, lưu `sessionStorage`). Shell không chứa business logic: mọi quyết định quyền do backend; shell chỉ ẩn/hiện (R1, R20).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript (strict) · React 18 · Vite |
| Primary Dependencies | React Router (không dùng `oidc-client-ts` — ADR-0091), TanStack Query, Ant Design 5 + `@ant-design/pro-components` (ProLayout), `react-i18next`, `openapi-typescript` (kiểu từ F050), `src/cms/frontend/packages/ops-ui` |
| Storage | Không có dữ liệu bền vững; access token trong RAM; refresh token trong cookie HttpOnly (backend đặt); `localStorage`/`sessionStorage` chỉ cho tùy chọn hiển thị, `device_id`, tenant phiên ([data-model.md](data-model.md)) |
| Testing | Vitest + Testing Library + MSW (mock **chỉ trong test**, gắn nhãn `// MOCK (test-only)`); Playwright E2E với backend local thật; axe-core; Playwright screenshot |
| Target Platform | Trình duyệt desktop/tablet/mobile (Chromium, Firefox, Safari mới) |
| Project Type | Web app (SPA) |
| Performance Goals | Bundle khởi đầu không chứa mã workspace; shell tương tác < 2 s trên mạng trường [Confirmed] |
| Constraints | AGENTS R1, R3, R4, R14, R20, R32 · ADR-0022, ADR-0027, ADR-0047, ADR-0065, ADR-0066, ADR-0067, ADR-0081, ADR-0090, ADR-0091 |
| Scale/Scope | 5 workspace, ~12 màn shell; tham số `portal_session_sso`, `principal_cache_ttl` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; contract `/auth/portal/*`, `PermissionSet.disabled_*`, `specs/portal/menu-catalog.yaml` đã có trong spec gộp (ADR-0090/0091) — kiểm khớp ở T053; delta cũ T004–T005 bỏ theo ADR-0091 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public/nội bộ qua `apiClient`; không gọi Keycloak Admin API, không NATS/Valkey (R25) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Một tenant mỗi phiên; xóa cache khi đổi tenant; `tenant_ids` chỉ cho System Admin; test E2E cô lập (T039, T044) |
| IV Server-authoritative | ✅ | Không có điểm/kết quả; quyền do backend quyết định |
| V Xác thực, phân quyền | ✅ | Form gửi backend (SPA không biết secret), access token RAM, refresh cookie HttpOnly + CSRF, lỗi đăng nhập chung, đăng nhập lại khi `MFA_REQUIRED`, xử lý `PORTAL_ACCESS_STAFF_ONLY`/`TENANT_MISMATCH`/`FEATURE_DISABLED`; ẩn menu/action không phải lớp bảo mật (backend `featuregate`); URL từ env |
| VI An toàn trẻ em | ✅ | Không hiển thị dữ liệu học sinh trong shell; không log token/PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; `maxLength` ô tìm theo OpenAPI |
| VIII UI tách logic, token | ✅ | Theme từ `ops-ui` sinh từ `design/admin-tokens`; không component game |
| IX Test có bằng chứng | ✅ | Mỗi F051-AC-* có test (tasks.md); test viết trước, FAIL trước khi code |

## Project Structure

### Documentation (feature này)

```text
specs/051-portal-shell/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/cms/frontend/apps/portal/
├── .env.example                     # VITE_API_PUBLIC_BASE_URL (cùng origin portal), VITE_API_INTERNAL_BASE_URL, VITE_PORTAL_BASE_PATH (không còn VITE_OIDC_*)
├── scripts/gen-menu-catalog.mjs     # specs/portal/menu-catalog.yaml → src/generated/menu-catalog.ts (make portal-catalog)
├── src/
│   ├── main.tsx
│   ├── config/env.ts                # đọc env, không hardcode (R3)
│   ├── app/{App.tsx,router.tsx,providers.tsx,queryClient.ts}
│   ├── generated/menu-catalog.ts    # SINH TỰ ĐỘNG từ catalog — không sửa tay (ADR-0090)
│   ├── auth/{authApi.ts,tokenStore.ts,refreshScheduler.ts,csrf.ts,deviceId.ts,AuthProvider.tsx,useAuth.ts,
│   │         IdleGuard.ts,restoreSession.ts,ReLoginDialog.tsx,LoginPage.tsx,AccountSetupPage.tsx,
│   │         ForgotPasswordPage.tsx,ResetPasswordPage.tsx}
│   ├── session/{sessionMachine.ts,SessionProvider.tsx,useSession.ts,tenantSelection.ts,
│   │            TenantSelectPage.tsx,NoAccessPage.tsx,SessionExpiredModal.tsx,unsavedChangesGuard.ts}
│   ├── permissions/{usePermissions.ts,RequirePermission.tsx,workspaceRegistry.ts,menuRegistry.ts,ActionGate.tsx,
│   │                FeatureDisabledPage.tsx,routeComponents.ts}   # registry đọc generated/menu-catalog.ts
│   ├── shell/{PortalLayout.tsx,WorkspaceSwitcher.tsx,TenantSwitcher.tsx,TenantFilter.tsx,
│   │          TenantTargetField.tsx,UserMenu.tsx,BannerSlot.tsx,ForbiddenPage.tsx,
│   │          VpnRequiredNotice.tsx,LargeScreenRequired.tsx,ProblemMessage.tsx}
│   ├── api/                         # từ F050: client.ts, schema.d.ts (openapi-typescript), problem.ts
│   │   └── hooks/{useMyTenants.ts,useMyPermissions.ts,useMe.ts,useTenantOptions.ts}
│   ├── i18n/{index.ts,locales/vi/portal.shell.json,locales/en/portal.shell.json}
│   └── workspaces/{school-lms,content-studio,game-ops,live-ops,platform-admin}/index.tsx  # entry lazy
└── tests/
    ├── unit/{session,permissions,shell,auth}/*.test.ts(x)
    └── e2e/{login.spec.ts,workspaces.spec.ts,tenant-switch.spec.ts,tenant-filter.spec.ts,a11y.spec.ts,visual.spec.ts}
```

**Structure Decision**: Một SPA theo ADR-0022/ADR-0027; mã workspace tách thư mục `src/workspaces/<slug>` để Vite tách chunk; shell (auth/session/permissions/shell) dùng chung. `TenantTargetField`, `LargeScreenRequired`, `BannerSlot` đặt ở shell để F052–F057 dùng lại.

## Phase 0: Research

Xem [research.md](research.md): R1 (thay bởi R5) lưu token và làm mới im lặng; R2 (bỏ) xin token tenant `organization:{alias}`; R3 chu kỳ làm mới permission; R4 tách chunk theo workspace; R5 phiên bằng cookie HttpOnly + CSRF (ADR-0091); R6 sinh menu từ catalog (ADR-0090).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md) (chỉ trạng thái client)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `sessionMachine.ts` (trạng thái phiên, quyết định định tuyến thuần từ `MyTenantsResponse`), `menuRegistry.ts` (lọc mục menu sinh từ catalog theo `workspaces`, `permissions` any-of, `disabled_menus`), `actionAllowed(actionKey, permissionSet)` | Hàm thuần, test unit; không gọi API |
| application (client) | Hook TanStack Query: `useMyTenants` (`listMyTenants`), `useMyPermissions` (`getMyPermissions`, `refetchInterval` ≤ `principal_cache_ttl`), `useMe` (`getMe`), `useTenantOptions` (`listTenants`) | Query key có tenant phiên ([data-model.md](data-model.md)) |
| infrastructure (client) | `authApi.ts` (`portalLogin`, `refreshPortalToken`, `portalLogout`, `setupPortalPassword`, `forgotPortalPassword`, `resetPortalPassword`, `credentials: "include"`), `tokenStore.ts` (access token trong RAM), `csrf.ts` (đọc cookie `__Host-ew_portal_csrf` → header `X-EW-CSRF`), `refreshScheduler.ts` (single-flight), `apiClient` (F050) với interceptor `TOKEN_EXPIRED` → refresh + thử lại, `MFA_REQUIRED` → `ReLoginDialog`, `TENANT_MISMATCH` → reset phiên, `FEATURE_DISABLED` → thông báo + refetch quyền, network error ở base nội bộ → `VpnRequiredNotice` | Base URL public/nội bộ theo workspace |
| presentation | `LoginPage`, `AccountSetupPage`, `ForgotPasswordPage`, `ResetPasswordPage`, `ReLoginDialog`, `PortalLayout` (ProLayout), `TenantSelectPage`, `NoAccessPage`, `ForbiddenPage`, `FeatureDisabledPage`, `ActionGate`, `SessionExpiredModal`, `TenantFilter`, `TenantSwitcher`, `UserMenu` | Không chứa business logic (R1) |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F012 (`/auth/portal/*`, `listMyTenants`, `getMyPermissions` có `disabled_*`, `getMe`, `featuregate`, kiểm tra `PORTAL_ACCESS_STAFF_ONLY`), F015 (dữ liệu bật/tắt), F004 (client `eduworld-portal` confidential + direct grant, route `/auth/portal/*` cùng origin, trang action-token `eduworld-ops`), F050 (`apiClient`, theme, `ops-ui`).
- Không tiêu thụ event NATS (R25). `listTenants` gọi qua base nội bộ (x-internal).
- Workspace khác **không** khai báo menu: chỉ gắn component vào khóa menu sinh từ catalog (`routeComponents['menu.school_lms.assignments'] = lazy(...)`) và bọc thao tác bằng `<ActionGate actionKey="action.…">`; thêm menu/action mới = sửa `specs/portal/menu-catalog.yaml` (+ `x-portal-action` trong OpenAPI) rồi `make portal-catalog` (ADR-0090).

### Bảo mật, tenant, idempotency, audit

- Access token chỉ trong RAM; refresh token chỉ trong cookie `__Secure-ew_portal_rt` (HttpOnly, `SameSite=Strict`, `Path=/api/v1/auth/portal`) do backend đặt và xoay vòng; `refreshPortalToken`/`portalLogout` gửi `X-EW-CSRF` = cookie `__Host-ew_portal_csrf`; mọi gọi `/auth/portal/*` dùng `credentials: "include"` cùng origin; không ghi token, mật khẩu, OTP vào log/telemetry (ADR-0091).
- Phiên khi tải lại trang: `restoreSession` gọi `refreshPortalToken`, giữ `returnUrl`/tenant (`sessionStorage`); `IdleGuard` ghi `ew.portal.lastActivityAt` (dấu thời gian, `localStorage`, try/catch), quá 30 phút không tương tác → ngừng làm mới và gọi `portalLogout`; tối đa 10 giờ do client session Keycloak cưỡng chế (`portal_session_sso`).
- `/reset-password`: xóa token khỏi URL ngay, `Referrer-Policy: no-referrer`, không đưa token vào router state lưu bền.
- `X-Tenant-Id`: tenant phiên cho người dùng tenant; System Admin: đọc liên tenant không gửi header + `tenant_ids[]`, ghi gửi tenant đích (`TenantTargetField`).
- Không có thao tác ghi nghiệp vụ trong F051 → không Idempotency-Key; audit `cross_tenant.accessed`, `authz.denied` do backend.
- Bảng permission dùng trong shell: `platform.cross_tenant.read` (hiện bộ lọc tenant); permission của menu/action lấy từ catalog sinh tự động (không khai báo tay).

### Quan sát

- Error boundary ghi `trace_id` từ Problem Details vào thông điệp người dùng; không log body, token, email, tên học sinh ra console ở build production (R14).
- Đếm sự kiện client (không PII) `portal_login_route{result}` gửi qua kênh telemetry của F050 nếu có [Confirmed].

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
