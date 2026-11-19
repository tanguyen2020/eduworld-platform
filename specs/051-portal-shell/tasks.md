# Tasks: F051 — Portal shell: đăng nhập, chọn tenant sau đăng nhập, menu theo quyền, bộ lọc tenant

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn tương đối gốc `src/cms/frontend/apps/portal/` trừ khi ghi rõ. Mock MSW chỉ trong test, gắn nhãn `// MOCK (test-only)` (R17); E2E chạy với backend local thật.

## Phase 1: Setup

- [x] T001 Tạo cây thư mục shell theo plan.md và `.env.example` (không còn `VITE_OIDC_*` — ADR-0091; `VITE_API_PUBLIC_BASE_URL` cùng origin portal, `VITE_API_INTERNAL_BASE_URL`, `VITE_PORTAL_BASE_PATH`, `VITE_PRINCIPAL_CACHE_TTL_SECONDS`) — file: `src/cms/frontend/apps/portal/.env.example` — **2026-11-19:** theo F050 (cấu hình runtime): `public/config.example.json` thay `.env.example`, bỏ khóa OIDC, thêm `principalCacheTtlSeconds`, `sessionIdleTimeoutSeconds` (OQ F051-2); cây `src/{auth,session,permissions,shell,generated}`.
- [x] T002 [P] Đọc cấu hình env, báo lỗi khi thiếu biến, không giá trị mặc định cho URL/client id (R3) — file: `src/cms/frontend/apps/portal/src/config/env.ts` — **2026-11-19:** `src/app/runtime-config.ts` (thay `config/env.ts`, F050): thiếu/sai khóa → `RuntimeConfigError`, không mặc định URL; test `runtime-config.test.ts` (6 pass).
- [ ] T003 [P] Playwright project `@F051`, fixture đăng nhập tài khoản demo (`ad.truong.a`, `qlcm.truong.a`, `sa.admin`, `cm.noidung`, `hs.an.3a`) — file: `src/cms/frontend/apps/portal/tests/e2e/fixtures/auth.ts` — **2026-11-19:** chưa làm — E2E với backend thật chờ F004/F012 (OQ F051-9); tài khoản demo có ở MOCK `src/test/mock-backend.ts`.

## Phase 2: Foundational (chặn mọi user story)

- [x] ~~T004 Contract: thêm `keycloak_org_alias` vào `MyTenant`~~ — hủy theo ADR-0091 mục 4 (không scope `organization:{alias}`)
- [x] ~~T005 Contract: `logout` dùng `TenantIdHeaderOptional`~~ — hủy theo ADR-0091 (portal dùng `portalLogout`)
- [x] T006 Sinh lại kiểu client từ bundle — file: `src/cms/frontend/apps/portal/src/api/schema.d.ts` — kiểm: `pnpm gen:api` không diff trong CI — **2026-11-19:** `pnpm gen:check` → `gen:check: 2 file(s) match` + `menu-catalog.ts matches` (kiểu `schema.gen.d.ts` F050 đã có `/auth/portal/*`, `PermissionSet.disabled_*`).
- [x] T007 [P] Khởi tạo i18n `vi` (mặc định) / `en`, khóa `portal.shell.*` — file: `src/cms/frontend/apps/portal/src/i18n/index.ts`, `src/cms/frontend/apps/portal/src/i18n/locales/{vi,en}/portal.shell.json` — **2026-11-19:** `src/i18n/{vi,en}.json` (`shell.*`), `catalog.{vi,en}.json` (nhãn workspace/menu); test `i18n/keys.test.ts`.
- [x] T008 [P] Providers: `QueryClientProvider`, `ConfigProvider` (theme `ops-ui` sáng/tối/compact, locale `vi_VN`/`en_US`, dayjs) — file: `src/cms/frontend/apps/portal/src/app/providers.tsx` — **2026-11-19:** `src/app/PreferencesProvider.tsx` (theme sáng/tối/compact, locale, dayjs qua `OpsThemeProvider`), `src/app/runtime.ts` (QueryClient, apiClient); test `shell/layout.test.tsx`.
- [x] T009 [P] `ProblemMessage`: ánh xạ `code` → khóa i18n, hiển thị `trace_id`, mặc định cho code lạ — file: `src/cms/frontend/apps/portal/src/shell/ProblemMessage.tsx` — **2026-11-19:** `src/shell/ProblemMessage.tsx`; test `shell/errors.test.tsx` › ProblemMessage.
- [x] T010 `workspaceRegistry` (mã `PermissionSet.workspaces` → slug + `React.lazy`) và `menuRegistry` (đọc `src/generated/menu-catalog.ts` sinh ở T054, lọc theo `workspaces`/`permissions` any-of/`disabled_menus`; **không** có API `register` khai báo menu tay — ADR-0090) dạng hàm thuần — file: `src/cms/frontend/apps/portal/src/permissions/workspaceRegistry.ts`, `src/cms/frontend/apps/portal/src/permissions/menuRegistry.ts` — **2026-11-19:** `src/permissions/{workspaceRegistry,menuRegistry}.ts` (hàm thuần, không API register); test `menuRegistry.test.ts` (7 pass).

- [x] T050 [P] Tiện ích dùng chung cho F052–F057: `useIdempotentMutation` (một `Idempotency-Key` cho mỗi ý định người dùng, giữ nguyên khi retry/bấm lại, đổi khi người dùng sửa dữ liệu), `ReasonDialog` (lý do bắt buộc, `maxLength` theo OpenAPI, bộ đếm), `mapValidationErrors` (`422 VALIDATION_FAILED` `errors[].field` → field form) kèm unit test — file: `src/cms/frontend/apps/portal/src/api/useIdempotentMutation.ts`, `src/cms/frontend/apps/portal/src/shell/ReasonDialog.tsx`, `src/cms/frontend/apps/portal/src/api/mapValidationErrors.ts`, `src/cms/frontend/apps/portal/tests/unit/api/shared.test.ts` — **2026-11-19:** `src/shared/api/{useIdempotentMutation,mapValidationErrors}.ts`, `src/shell/ReasonDialog.tsx`; test `src/shared/api/shared.test.tsx` (4 pass).

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Đăng nhập portal và vào đúng phạm vi (Priority: P1) 🎯 MVP

**Goal**: Đăng nhập bằng form `/login` của portal qua `/api/v1/auth/portal/*` (ADR-0091, thay PKCE), định tuyến sau đăng nhập, chặn học sinh, hết phiên, đăng xuất.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T011 [P] [US1] Unit test `sessionMachine` định tuyến platform / 1 tenant / nhiều tenant / rỗng (F051-AC-03) — file: `src/cms/frontend/apps/portal/tests/unit/session/sessionMachine.test.ts` — phải FAIL trước khi code — **2026-11-19:** `src/session/sessionMachine.test.ts` (9 pass).
- [x] T012 [P] [US1] Unit test `tokenStore`/`authApi`: access token chỉ trong RAM, không ghi `localStorage`/`sessionStorage`/IndexedDB/cookie, mật khẩu/OTP/ticket không lưu (F051-AC-02) — file: `src/cms/frontend/apps/portal/tests/unit/auth/tokenStore.test.ts` — **2026-11-19:** `src/auth/tokenStore.test.ts` (7 pass: không token/mật khẩu/mã/ticket trong storage, chỉ ghi `ew.portal.deviceId`).
- [x] T013 [P] [US1] Unit test interceptor: `403 PORTAL_ACCESS_STAFF_ONLY` → `no_access` + đăng xuất (F051-AC-04); refresh thất bại → `expired` giữ `returnUrl` (F051-AC-07) — file: `src/cms/frontend/apps/portal/tests/unit/session/interceptors.test.ts` — **2026-11-19:** `src/auth/sessionLifecycle.test.tsx` › staff only, session expired.
- [x] T014 [P] [US1] Unit test `tenantSelection`: không xin token mới, cùng access token, header `X-Tenant-Id` sau chọn, nhớ lựa chọn (F051-AC-05, F051-AC-06) — file: `src/cms/frontend/apps/portal/tests/unit/session/tenantSelection.test.ts` — **2026-11-19:** `src/session/tenantSession.test.tsx` › tenant selection after sign-in (3 pass).
- [ ] T015 [US1] E2E đăng nhập 4 tài khoản + học sinh qua trang `/login` tự vẽ (không request tới Keycloak từ trình duyệt), không URL theo tenant (F051-AC-01, F051-AC-03, F051-AC-04, F051-AC-05; TC-AUTH-070, TC-AUTH-071, TC-AUTH-072, TC-AUTH-095, TC-CMS-LOGIN-01, TC-CMS-LOGIN-02) — file: `src/cms/frontend/apps/portal/tests/e2e/login.spec.ts` — **2026-11-19:** phần unit/integration xong (`auth/LoginPage.test.tsx` 8 pass với MOCK backend); E2E backend thật chờ OQ F051-9.
- [ ] T016 [US1] E2E hết phiên và đăng xuất xóa cache (F051-AC-07, F051-AC-08) — file: `src/cms/frontend/apps/portal/tests/e2e/logout.spec.ts` — **2026-11-19:** phần unit/integration xong (`auth/sessionLifecycle.test.tsx` › sign-out, session expired); E2E chờ OQ F051-9.

### Implementation cho User Story 1

- [x] T017 [P] [US1] `authApi` + `tokenStore` + `csrf` + `deviceId` (thay `oidcClient` — ADR-0091): gọi `/auth/portal/*` `credentials: "include"`, access token RAM, `X-EW-CSRF` từ cookie, `device_id` per-trình duyệt — file: `src/cms/frontend/apps/portal/src/auth/authApi.ts`, `src/cms/frontend/apps/portal/src/auth/tokenStore.ts`, `src/cms/frontend/apps/portal/src/auth/csrf.ts`, `src/cms/frontend/apps/portal/src/auth/deviceId.ts` — **2026-11-19:** `src/auth/{authApi,tokenStore,csrf,deviceId}.ts`.
- [x] T018 [P] [US1] `sessionMachine` (trạng thái theo data-model.md §2) — file: `src/cms/frontend/apps/portal/src/session/sessionMachine.ts` — **2026-11-19:** `src/session/sessionMachine.ts`.
- [x] T019 [US1] `AuthProvider`, `SessionProvider` (không còn `CallbackPage`) (gọi `listMyTenants`, cấp `tenantHeaderProvider` cho `apiClient`) — file: `src/cms/frontend/apps/portal/src/auth/AuthProvider.tsx`, `src/cms/frontend/apps/portal/src/session/SessionProvider.tsx` — **2026-11-19:** `src/session/SessionProvider.tsx` (thay `AuthProvider`; `WorkingTenant` là `TenantSource` của apiClient).
- [x] T020 [US1] `TenantSelectPage` (tìm theo tên/mã, chọn sẵn tenant nhớ, "Nhớ lựa chọn", Đăng xuất) + `tenantSelection.ts` — file: `src/cms/frontend/apps/portal/src/session/TenantSelectPage.tsx`, `src/cms/frontend/apps/portal/src/session/tenantSelection.ts` — **2026-11-19:** `src/session/SessionPages.tsx` (`TenantSelectPage`), `tenantSelection.ts`.
- [x] T021 [US1] `NoAccessPage` (thông điệp học sinh / chưa cấp quyền) và `SessionExpiredModal` — file: `src/cms/frontend/apps/portal/src/session/NoAccessPage.tsx`, `src/cms/frontend/apps/portal/src/session/SessionExpiredModal.tsx` — **2026-11-19:** `src/shell/StatusPages.tsx` (`NoAccessPage`), `SessionPages.tsx` (`SessionExpiredModal`, `RequireSession`).
- [x] T022 [US1] Hook `useMyTenants`, `useMe` và interceptor phiên trong `apiClient` — file: `src/cms/frontend/apps/portal/src/api/hooks/useMyTenants.ts`, `src/cms/frontend/apps/portal/src/session/interceptors.ts` — **2026-11-19:** `src/shared/api/hooks/{useMe,useMyPermissions}.ts`; interceptor = sự kiện apiClient (`staff_only`, `tenant_mismatch`, `feature_disabled`, `permission_denied`, `mfa_required`) xử lý trong `SessionProvider`.
- [x] T023 [US1] Đăng xuất: `portalLogout` (kèm `X-EW-CSRF`) → xóa access token, tenant phiên → `queryClient.clear()` → `/login` — file: `src/cms/frontend/apps/portal/src/auth/useAuth.ts` — **2026-11-19:** `SessionProvider.logout` (kể cả lỗi mạng); test `sessionLifecycle.test.tsx` › sign-out (2 pass).

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Shell theo quyền: workspace, menu, lỗi quyền (Priority: P1)

**Goal**: Menu/workspace từ permission, chunk lazy, 403, step-up, VPN, i18n, theme, responsive, a11y.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T024 [P] [US2] Unit test `menuRegistry` lọc mục sinh từ catalog theo `permissions[]` (any-of)/`workspaces[]`/`disabled_menus[]` (F051-AC-09, F051-AC-35) — file: `src/cms/frontend/apps/portal/tests/unit/permissions/menuRegistry.test.ts` — phải FAIL trước khi code — **2026-11-19:** `src/permissions/menuRegistry.test.ts`.
- [x] T025 [P] [US2] Unit test `useMyPermissions`: `refetchInterval` = `principal_cache_ttl`, refetch khi focus và sau 403 (F051-AC-12) — file: `src/cms/frontend/apps/portal/tests/unit/permissions/useMyPermissions.test.ts` — **2026-11-19:** `src/permissions/catalogMenus.test.tsx` › refresh period, PERMISSION_DENIED.
- [x] T026 [P] [US2] Unit test interceptor `MFA_REQUIRED` → `ReLoginDialog` (mật khẩu + mã, `portalLogin`, thử lại request) (F051-AC-13), lỗi mạng base nội bộ → `VpnRequiredNotice` (F051-AC-14), `ProblemMessage` code lạ (F051-AC-15) — file: `src/cms/frontend/apps/portal/tests/unit/shell/errors.test.tsx` — **2026-11-19:** `src/shell/errors.test.tsx` (MFA, VPN, ProblemMessage).
- [ ] T027 [US2] E2E authorization: `qlcm.truong.a` chỉ tải chunk `school-lms`/`content-studio`, `/game-ops` → 403 không gọi API; `ad.truong.a` không có `game.*`/`platform.*` trong menu (F051-AC-09, F051-AC-10, F051-AC-11; TC-CMS-WS-01, TC-CMS-AUTHZ-05, TC-CMS-AUTHZ-06) — file: `src/cms/frontend/apps/portal/tests/e2e/workspaces.spec.ts` — **2026-11-19:** phần unit/integration xong (`app/router.test.tsx`: chỉ chunk `school-lms`, `/game-ops` → 403 không tải chunk, không gọi API màn); E2E chờ OQ F051-9.
- [ ] T028 [P] [US2] E2E đổi ngôn ngữ/theme/compact, 3 breakpoint + visual regression sáng/tối (F051-AC-16, F051-AC-17, F051-AC-19; TC-I18N-006, TC-UI-004) — file: `src/cms/frontend/apps/portal/tests/e2e/visual.spec.ts` — **2026-11-19:** phần unit xong (`shell/layout.test.tsx`: 3 breakpoint, ngôn ngữ/theme/compact không tải lại) + visual Storybook 18 story × 2 zoom; E2E toàn trang chờ OQ F051-9.
- [ ] T029 [P] [US2] E2E axe shell 3 breakpoint (F051-AC-18; TC-UI-006) — file: `src/cms/frontend/apps/portal/tests/e2e/a11y.spec.ts` — **2026-11-19:** axe trên Storybook cho 6 component khung (0 serious/critical); E2E toàn trang chờ OQ F051-9.
- [x] T030 [P] [US2] Lint: `no-restricted-imports` chặn import từ `src/game/`, `design/game-tokens`; stylelint/ESLint chặn hardcode (F051-AC-17; TC-UI-003, TC-UI-010) — file: `src/cms/frontend/apps/portal/eslint.config.js` — **2026-11-19:** ESLint flat config F050 đã chặn (`ew/no-game-imports`, `ew/no-hardcoded-style`, stylelint); `eslint.config.mjs` bỏ qua `src/generated/**`; `pnpm lint` 0 lỗi.

### Implementation cho User Story 2

- [x] T031 [US2] `PortalLayout` (ProLayout: topbar, workspace switcher, sidebar, breadcrumb, `BannerSlot`; drawer < 1280 px, menu ☰ < 768 px) — file: `src/cms/frontend/apps/portal/src/shell/PortalLayout.tsx`, `src/cms/frontend/apps/portal/src/shell/WorkspaceSwitcher.tsx`, `src/cms/frontend/apps/portal/src/shell/BannerSlot.tsx` — **2026-11-19:** `src/shell/PortalLayout.tsx` (Ant Design Layout/Menu/Drawer thay ProLayout — OQ F051-5; `BannerSlot`, `WorkspaceSwitcher`).
- [x] T032 [US2] Router: route workspace lazy chỉ khi có trong `workspaces[]`; `RequirePermission` + `ForbiddenPage` — file: `src/cms/frontend/apps/portal/src/app/router.tsx`, `src/cms/frontend/apps/portal/src/permissions/RequirePermission.tsx`, `src/cms/frontend/apps/portal/src/shell/ForbiddenPage.tsx` — **2026-11-19:** `src/app/router.tsx`, `src/permissions/RequirePermission.tsx` (`WorkspaceGate`, `MenuGate`, `WorkspaceRoutes`), `ForbiddenPage` trong `StatusPages.tsx`.
- [x] T033 [US2] `useMyPermissions` (query key theo `scopeKey`, làm mới theo AC-12) — file: `src/cms/frontend/apps/portal/src/api/hooks/useMyPermissions.ts` — **2026-11-19:** `src/shared/api/hooks/useMyPermissions.ts`.
- [x] T034 [US2] `ReLoginDialog` (thay `stepUp.ts`) và `VpnRequiredNotice` — file: `src/cms/frontend/apps/portal/src/auth/ReLoginDialog.tsx`, `src/cms/frontend/apps/portal/src/shell/VpnRequiredNotice.tsx` — **2026-11-19:** `src/auth/ReLoginDialog.tsx` (`ReLoginHost` + `reauthenticate` của apiClient), `VpnRequiredNotice` trong `StatusPages.tsx`.
- [x] T035 [US2] `UserMenu`: ngôn ngữ, sáng/tối, compact, đăng xuất; lưu prefs per-viewer (try/catch) — file: `src/cms/frontend/apps/portal/src/shell/UserMenu.tsx` — **2026-11-19:** `UserMenu` trong `PortalLayout.tsx` (Popover: ngôn ngữ, sáng/tối, compact, đăng xuất).
- [x] T036 [P] [US2] Entry lazy mỗi workspace (trang tổng quan rỗng, chưa có dữ liệu) và `LargeScreenRequired` dùng chung — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/index.tsx`, `src/cms/frontend/apps/portal/src/shell/LargeScreenRequired.tsx` — **2026-11-19:** 5 `src/workspaces/*/index.tsx` gắn `placeholderScreen` cho mọi khóa menu (`satisfies MenuScreens<W>`), `LargeScreenRequired`.

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Đổi đơn vị làm việc trên topbar (Priority: P2)

**Goal**: Đổi tenant an toàn, không trộn dữ liệu.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T037 [P] [US3] Unit test đổi tenant: cảnh báo bản nháp, không gọi `portalLogin`/`refreshPortalToken`, đổi `X-Tenant-Id`, `queryClient.clear()`, tải lại permission (F051-AC-20) — file: `src/cms/frontend/apps/portal/tests/unit/session/switchTenant.test.ts` — phải FAIL trước khi code — **2026-11-19:** `src/session/tenantSession.test.tsx` › switching tenant on the topbar.
- [x] T038 [P] [US3] Unit test `TENANT_MISMATCH`/`TENANT_ACCESS_DISABLED` → hủy query → màn chọn đơn vị (F051-AC-21) — file: `src/cms/frontend/apps/portal/tests/unit/session/tenantMismatch.test.ts` — **2026-11-19:** `src/session/tenantSession.test.tsx` › tenant access changed (2 pass).
- [ ] T039 [US3] E2E tenant isolation: `qlcm.truong.a` đổi `th-demo-a` → `pgd-demo`, gửi lại request header cũ → `403 TENANT_MISMATCH`, không còn dữ liệu cũ trên màn (F051-AC-20, F051-AC-21; TC-TEN-008, TC-CMS-LOGIN-03) — file: `src/cms/frontend/apps/portal/tests/e2e/tenant-switch.spec.ts` — **2026-11-19:** phần unit xong (T037, T038: không request nào mang header cũ sau khi đổi); E2E chờ OQ F051-9.

### Implementation cho User Story 3

- [x] T040 [US3] `TenantSwitcher` (topbar; mobile trong ☰) + `unsavedChangesGuard` (registry để editor đăng ký) — file: `src/cms/frontend/apps/portal/src/shell/TenantSwitcher.tsx`, `src/cms/frontend/apps/portal/src/session/unsavedChangesGuard.ts` — **2026-11-19:** `TenantSwitcher`/`TenantSwitcherView` trong `src/shell/TenantControls.tsx`, `src/session/unsavedChangesGuard.ts`.
- [x] T041 [US3] Interceptor `TENANT_MISMATCH`/`TENANT_ACCESS_DISABLED` — file: `src/cms/frontend/apps/portal/src/session/interceptors.ts` — **2026-11-19:** apiClient phát `tenant_mismatch` cho `TENANT_MISMATCH`/`TENANT_ACCESS_DISABLED`, `SessionProvider` hủy query, xóa cache, về màn chọn đơn vị.

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Bộ lọc tenant của System Admin (Priority: P2)

**Goal**: Dropdown tenant nhiều lựa chọn, `tenant_ids`, tenant đích khi ghi.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T042 [P] [US4] Unit test `TenantFilter`: chỉ hiện khi có `platform.cross_tenant.read`, mặc định tất cả, lưu `sessionStorage`, ô tìm giới hạn `maxLength` (F051-AC-22, F051-AC-26, F051-AC-27) — file: `src/cms/frontend/apps/portal/tests/unit/shell/TenantFilter.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `src/shell/TenantFilter.test.tsx` (4 pass).
- [x] T043 [P] [US4] Unit test `withTenantScope`: System Admin đọc → `tenant_ids[]` không header; người dùng tenant → header, bỏ `tenant_ids`; `requireSingleTenant`; `TenantTargetField` bắt buộc, không có "tất cả" (F051-AC-23, F051-AC-24, F051-AC-25, F051-AC-26) — file: `src/cms/frontend/apps/portal/tests/unit/shell/withTenantScope.test.ts` — **2026-11-19:** `src/shared/api/withTenantScope.test.ts` (7 pass).
- [ ] T044 [US4] E2E authorization + tenant: `sa.admin` lọc 2 tenant thấy cột Tenant; `cm.noidung` và `ad.truong.a` không có bộ lọc (F051-AC-22, F051-AC-23, F051-AC-24, F051-AC-25, F051-AC-26; TC-TEN-018, TC-TEN-004, TC-CMS-TEN-01, TC-CMS-TEN-02, TC-CMS-TEN-03) — file: `src/cms/frontend/apps/portal/tests/e2e/tenant-filter.spec.ts` — **2026-11-19:** phần unit xong (T042, T043); E2E chờ OQ F051-9.

### Implementation cho User Story 4

- [x] T045 [US4] `TenantFilter`, `useTenantOptions` (`listTenants`, base nội bộ, debounce), `TenantTargetField`, helper `withTenantScope`/`requireSingleTenant`, cột Tenant chuẩn cho `ResponsiveTable` — file: `src/cms/frontend/apps/portal/src/shell/TenantFilter.tsx`, `src/cms/frontend/apps/portal/src/shell/TenantTargetField.tsx`, `src/cms/frontend/apps/portal/src/api/withTenantScope.ts` — **2026-11-19:** `TenantFilter`, `TenantTargetField`, `useTenantColumn` (`src/shell/TenantControls.tsx`), `tenantFilterStore.ts`, `useTenantOptions.ts`, `src/shared/api/withTenantScope.ts`; apiClient giữ header đích của System Admin và bỏ `tenant_ids` khi phiên có tenant.

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: Polish & Cross-Cutting

- [x] T050 [P] [US1] Test F051-AC-28, F051-AC-29: tải lại trang khi phiên còn hạn → `refreshPortalToken` khôi phục URL + tenant, không có token trong kho trình duyệt; quá idle 30 phút / 10 giờ → về `/login`, giữ `returnUrl`; `localStorage` bị chặn không làm hỏng đăng nhập (TC-AUTH-089) — file: `src/cms/frontend/apps/portal/tests/unit/auth/sessionRestore.test.ts`, `src/cms/frontend/apps/portal/tests/e2e/session-reload.spec.ts` — phải FAIL trước khi code — **2026-11-19:** `src/auth/sessionLifecycle.test.tsx` › reload, idle limit (5 pass); E2E `session-reload.spec.ts` chờ OQ F051-9.
- [x] T051 [US1] `IdleGuard` (ghi `ew.portal.lastActivityAt` có try/catch, hết 30 phút → ngừng làm mới + `portalLogout`) và khởi tạo phiên khi tải lại (`refreshPortalToken`, `401`/`CSRF_TOKEN_INVALID` → `/login?returnUrl=`) — file: `src/cms/frontend/apps/portal/src/auth/IdleGuard.ts`, `src/cms/frontend/apps/portal/src/auth/restoreSession.ts` — **2026-11-19:** `src/auth/IdleGuard.ts`; khôi phục phiên trong `SessionProvider` (thay `restoreSession.ts`).
- [ ] T052 [P] Kiểm cấu hình client `eduworld-portal` trong realm test (confidential, direct grant, client session idle 30 phút / max 10 giờ, refresh token rotation) và ghi kết quả kiểm chứng KEYCLOAK_SSO §5.2 [Proposed] — file: `specs/051-portal-shell/quickstart.md` — **2026-11-19:** chờ F004 T039/realm test (OQ F051-9).
- [x] T046 [P] Cập nhật ADMIN_PORTAL §2.1 mục 6 và §8 (`getMyPermissions` dùng header `X-Tenant-Id`), ghi CHANGELOG — file: `docs/04-admin-portal/ADMIN_PORTAL.md`, `CHANGELOG.md` — **2026-11-19:** ADMIN_PORTAL §2.1 mục 6 và §8 đã ghi header `X-Tenant-Id` (không cần sửa); CHANGELOG (Added), LOCAL_DEVELOPMENT §6.4, README frontend cập nhật.
- [x] T047 [P] Story Storybook sáng/tối cho `TenantFilter`, `TenantSwitcher`, `ForbiddenPage`, `NoAccessPage`, `VpnRequiredNotice` (TC-UI-005) — file: `src/cms/frontend/apps/portal/src/shell/shell.stories.tsx` — **2026-11-19:** 6 file `src/shell/*.stories.tsx` (ForbiddenPage, FeatureDisabledPage, NoAccessPage, VpnRequiredNotice, TenantSwitcher, TenantFilter) × Light/Dark/Compact; Storybook gom `apps/portal/src/**/*.stories.tsx`; axe + visual win32.
- [x] T048 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/051-portal-shell/quickstart.md` — **2026-11-19:** bằng chứng trong [quickstart.md](quickstart.md) §5.
- [x] T049 Cập nhật trạng thái F051 — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — **2026-11-19:** CODE_PLAN: Done (2026-11-19).

### Bổ sung ADR-0090/0091 — đăng nhập form riêng, menu sinh từ catalog

- [x] T053 Kiểm khớp contract: `/auth/portal/*`, `PortalLoginRequest`/`PortalTokenResponse`/`PortalSetupResult`, `PermissionSet.disabled_menus`/`disabled_actions`, ProblemCode mới; `pnpm gen:api` — file: `src/cms/frontend/apps/portal/src/api/schema.d.ts` — kiểm: `pnpm gen:api` không diff, `node scripts/specs/check-portal-catalog.mjs` — **2026-11-19:** `pnpm gen:check` không lệch; `node scripts/specs/check-portal-catalog.mjs` 0 lỗi.
- [x] T054 Generator `gen-menu-catalog.mjs` (catalog YAML → `src/generated/menu-catalog.ts`, union `MenuKey`/`ActionKey`), target `make portal-catalog`, bước CI `git diff --exit-code` — file: `src/cms/frontend/apps/portal/scripts/gen-menu-catalog.mjs`, `Makefile` — **2026-11-19:** `scripts/gen-menu-catalog.mjs` (`pnpm gen:catalog`, `--check` trong `pnpm gen:check` của CI), target `make portal-catalog`, web.yml thêm `specs/portal/**`; test `scripts/gen-menu-catalog.test.mjs`.
- [x] T055 [P] [US1] Unit + E2E F051-AC-01, F051-AC-30: `/login` đủ ô, một request `portalLogin` kèm `otp` khi nhập, `401 INVALID_CREDENTIALS` một thông điệp chung + xóa ô mật khẩu/mã, `429` hiện thời gian chờ, cờ `otpHint`; `ad.truong.a`, `sa.admin` (mã TOTP seed) vào được (TC-AUTH-093, TC-AUTH-094) — file: `src/cms/frontend/apps/portal/tests/unit/auth/LoginPage.test.tsx`, `src/cms/frontend/apps/portal/tests/e2e/login.spec.ts` — phải FAIL trước khi code — **2026-11-19:** `src/auth/LoginPage.test.tsx` (8 pass, MOCK backend); E2E với mã TOTP seed chờ OQ F051-9.
- [x] T056 [P] [US1] Test F051-AC-31, F051-AC-32: màn thiết lập (đổi mật khẩu, lỗi policy theo field, màn "kiểm tra email", ticket hết hạn → `/login`); quên/đặt lại mật khẩu (thông điệp như nhau, token xóa khỏi URL, `Referrer-Policy`, link hết hiệu lực) (TC-AUTH-096, TC-AUTH-097) — file: `src/cms/frontend/apps/portal/tests/unit/auth/accountRecovery.test.tsx`, `src/cms/frontend/apps/portal/tests/e2e/account-setup.spec.ts` — **2026-11-19:** `src/auth/accountRecovery.test.tsx` (11 pass); E2E Mailpit chờ OQ F051-9.
- [x] T057 [P] [US1] Test F051-AC-02, F051-AC-08, F051-AC-33: single-flight refresh khi nhiều request nhận `401 TOKEN_EXPIRED`, gửi `X-EW-CSRF`, thử lại một lần; hai tab xoay vòng không mất phiên; `document.cookie` không thấy refresh token; đăng xuất xóa trạng thái kể cả lỗi mạng (TC-AUTH-098) — file: `src/cms/frontend/apps/portal/tests/unit/auth/refreshScheduler.test.ts`, `src/cms/frontend/apps/portal/tests/e2e/session-cookie.spec.ts` — **2026-11-19:** `src/auth/refreshScheduler.test.ts` (6 pass), `tokenStore.test.ts`, `sessionLifecycle.test.tsx` › sign-out; E2E cookie chờ OQ F051-9.
- [x] T058 [US1] `LoginPage`, `AccountSetupPage`, `ForgotPasswordPage`, `ResetPasswordPage` (Ant Design, theme `ops-ui`, i18n `portal.shell.login.*`, `maxLength` theo OpenAPI) — file: `src/cms/frontend/apps/portal/src/auth/LoginPage.tsx`, `src/cms/frontend/apps/portal/src/auth/AccountSetupPage.tsx`, `src/cms/frontend/apps/portal/src/auth/ForgotPasswordPage.tsx`, `src/cms/frontend/apps/portal/src/auth/ResetPasswordPage.tsx` — **2026-11-19:** `src/auth/{LoginPage,AccountRecoveryPages,AuthLayout}.tsx`.
- [x] T059 [US1] `refreshScheduler` (làm mới trước `expires_in` khi còn tương tác, single-flight, `BroadcastChannel` giữa tab) + interceptor `TOKEN_EXPIRED` trong `apiClient` — file: `src/cms/frontend/apps/portal/src/auth/refreshScheduler.ts`, `src/cms/frontend/apps/portal/src/session/interceptors.ts` — **2026-11-19:** `src/auth/refreshScheduler.ts` (single-flight, Web Locks thay BroadcastChannel cho làm mới — OQ F051-7); `TOKEN_EXPIRED` → refresh + thử lại trong apiClient F050.
- [x] T060 [P] [US2] Test F051-AC-34, F051-AC-35, F051-AC-36, F051-AC-37: file sinh khớp catalog, mỗi `MenuKey` có component; menu ẩn khi thiếu permission/`disabled_menus`, URL menu tắt → `FeatureDisabledPage` không gọi API; `ActionGate` ẩn/khóa theo `disabled_actions`; `403 FEATURE_DISABLED` → thông báo + refetch quyền (TC-CMS-MENU-02, TC-CMS-MENU-05, TC-CMS-MENU-06) — file: `src/cms/frontend/apps/portal/tests/unit/permissions/catalogMenus.test.tsx`, `src/cms/frontend/apps/portal/tests/e2e/feature-toggles.spec.ts` — phải FAIL trước khi code — **2026-11-19:** `src/permissions/catalogMenus.test.tsx`; E2E `feature-toggles.spec.ts` chờ OQ F051-9.
- [x] T061 [US2] `ActionGate`, `FeatureDisabledPage`, `routeComponents` (khóa menu → component lazy), interceptor `FEATURE_DISABLED` — file: `src/cms/frontend/apps/portal/src/permissions/ActionGate.tsx`, `src/cms/frontend/apps/portal/src/permissions/FeatureDisabledPage.tsx`, `src/cms/frontend/apps/portal/src/permissions/routeComponents.ts` — **2026-11-19:** `src/permissions/ActionGate.tsx`, `FeatureDisabledPage` (`StatusPages.tsx`), `RequirePermission.tsx` (thay `routeComponents.ts`: `screens` theo workspace), sự kiện `feature_disabled`.

## Dependencies & Execution Order

- Setup → Foundational (T053, T054 trước T010; T004–T005 đã hủy) → US1 → US2 → US3, US4 (song song được) → Polish.
- F004 T039 (spike) và F012 US7–US8 trước T055–T059; F012 US9 + F015 US5 trước E2E T060.
- T019 cần T017, T018; T032 cần T010; T045 cần T033 (permission để biết System Admin).
- Backend F012 (US7 đăng nhập, US9 `featuregate`) phải chạy trước T015, T039, T055–T060.

## Definition of Done

- [x] Mọi tiêu chí F051-AC-* có test pass (ghi mã test/TC) — mức unit/integration với MOCK backend; E2E backend thật còn mở (OQ F051-9)
- [x] `make web-lint`, `make web-test`, `make storybook` xanh (lệnh pnpm tương đương, quickstart §5); Playwright `@F051` chờ OQ F051-9
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — MOCK chỉ ở `src/test/`, story
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
