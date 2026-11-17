# Research: F051 — Portal shell: đăng nhập, chọn tenant sau đăng nhập, menu theo quyền, bộ lọc tenant

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Lưu token và làm mới im lặng — **thay bởi R5 (ADR-0091, 2026-11-17)**

- **Decision**: `oidc-client-ts` `UserManager` với `userStore = new WebStorageStateStore({ store: new InMemoryWebStorage() })`, `automaticSilentRenew = true` bằng refresh token (rotation), `response_type=code`, PKCE S256. Tải lại trang (F5) → phiên SSO của Keycloak (cookie của Keycloak, không phải của portal) cho phép `signinSilent`/redirect không nhập lại mật khẩu trong `portal_session_sso`.
- **Rationale**: ADR-0022 cấm `localStorage`; F051-AC-02. Cookie SSO thuộc domain Keycloak, portal không ghi.
- **Alternatives**: BFF giữ token phía server — loại vì ADR-0022 chọn SPA PKCE, thêm thành phần backend không có trong kiến trúc backend (ADR-0074).
- **Bổ sung ADR-0086 mục 7 (2026-11-16)**: realm SSO Session Idle/Max nâng lên 7 ngày/30 ngày cho client game, nên cookie SSO tồn tại lâu hơn phiên portal. Portal cưỡng chế chính sách của mình: `max_age` = 10 giờ trên mọi auth request; client session `eduworld-portal` idle 30 phút làm refresh token hết hạn; `IdleGuard` ghi `lastActivityAt` và quá 30 phút gọi end-session hoặc `prompt=login` khi khởi tạo. Phương án đã loại: realm riêng cho nhân sự (đổi ADR-0006, ảnh hưởng Organizations và username duy nhất toàn realm).

## R2. Xin token cho tenant đã chọn — **bỏ (ADR-0091 mục 4: tenant bằng `X-Tenant-Id` + membership, không token mới)**

- **Decision**: Sau khi chọn tenant, gọi `userManager.signinRedirect({ scope: "openid profile organization:" + alias, state: { returnUrl } })` (SSO có sẵn nên không hỏi mật khẩu); alias lấy từ `MyTenant.keycloak_org_alias` (delta T004). Phạm vi platform và nhân sự 1 tenant không thêm scope `organization` (backend chọn theo membership, [KEYCLOAK_SSO §4.4](../../docs/09-identity/KEYCLOAK_SSO.md)).
- **Rationale**: [KEYCLOAK_SSO §3.3, §4.3](../../docs/09-identity/KEYCLOAK_SSO.md); không gửi scope `organization` trống.
- **Alternatives**: `signinSilent` trong iframe — bị trình duyệt chặn cookie bên thứ ba khi Keycloak khác domain; dùng làm tối ưu khi cùng site [Proposed].

## R3. Chu kỳ làm mới permission

- **Decision**: `useMyPermissions` đặt `staleTime` và `refetchInterval` bằng `principal_cache_ttl` (đọc từ cấu hình portal, không hardcode), `refetchOnWindowFocus = true`, `invalidateQueries` khi nhận `403 PERMISSION_DENIED`.
- **Rationale**: [ADMIN_PORTAL §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md) "người được gán thấy menu mới trong ≤ 60 s"; TC-CMS-RBAC-03.
- **Alternatives**: Đẩy realtime qua WebSocket — loại, portal không dùng realtime trong Release 1.0.

## R5. Phiên portal bằng cookie HttpOnly + CSRF (ADR-0091)

- **Decision**: Không dùng `oidc-client-ts`. `authApi` gọi `/api/v1/auth/portal/*` cùng origin với `credentials: "include"`; access token (body) giữ trong `tokenStore` (RAM); refresh token chỉ nằm trong cookie `__Secure-ew_portal_rt` HttpOnly do backend đặt; header `X-EW-CSRF` lấy từ cookie `__Host-ew_portal_csrf` (double-submit). `refreshScheduler` làm mới trước `expires_in` khi người dùng còn tương tác và khi nhận `401 TOKEN_EXPIRED`, single-flight bằng một Promise dùng chung; giữa các tab dùng `BroadcastChannel` để tab nhận `401 REFRESH_TOKEN_INVALID` đọc lại cookie CSRF và thử lại một lần. Tải lại trang = `refreshPortalToken` lúc khởi động.
- **Rationale**: ADR-0091 mục 4; JS không đọc được refresh token (chống XSS lấy token dài hạn); `SameSite=Strict` + CSRF header chống CSRF; không cần cookie bên thứ ba của Keycloak.
- **Alternatives**: Giữ refresh token trong RAM (mất phiên khi F5 — trái AC-28); lưu `sessionStorage` (lộ cho XSS — trái AC-02); BFF giữ access token (thêm thành phần, ADR-0091 đã chọn mô hình này).

## R6. Sinh menu từ catalog (ADR-0090)

- **Decision**: `web/apps/portal/scripts/gen-menu-catalog.mjs` đọc `specs/portal/menu-catalog.yaml` (đã kiểm bằng `scripts/specs/check-portal-catalog.mjs`) và sinh `src/generated/menu-catalog.ts`: mảng workspace/menu/action (`key`, `path`, `labelKey`, `requiredPermissions`, `order`), union type `MenuKey`, `ActionKey`. `make portal-catalog` chạy script; CI chạy lại và `git diff --exit-code`. Test kiểm mỗi `MenuKey` có component trong `routeComponents`.
- **Rationale**: Một nguồn sự thật (R33), khóa action có kiểu nên `ActionGate` không nhận khóa sai; trùng khớp với `platform.portal_menu_catalog` và `x-portal-action` mà backend dùng.
- **Alternatives**: Đọc catalog lúc chạy từ API (thêm round-trip, menu không có code tương ứng vô nghĩa — ADR-0090 phương án đã loại); khai báo tay (lệch catalog).

## R4. Tách chunk theo workspace

- **Decision**: `workspaceRegistry` ánh xạ mã `PermissionSet.workspaces` → `React.lazy(() => import("../workspaces/<slug>/index.tsx"))`; route chỉ được đăng ký khi workspace có trong `workspaces[]`; Vite `build.rollupOptions.output.manualChunks` không gộp `src/workspaces/*` vào chunk chung. Kiểm bằng Playwright đếm request chunk (TC-CMS-WS-01).
- **Rationale**: ADR-0027 mục 3, 7 (tối ưu tải, không phải lớp bảo mật).
- **Alternatives**: Micro-frontend riêng từng workspace — loại vì ADR-0027 chọn một ứng dụng.
