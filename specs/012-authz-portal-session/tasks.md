# Tasks: F012 — Phân quyền Permission/Role/UserRole và phiên portal (`/me/*`, `principals:resolve`)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Khởi tạo package `internal/domain/authz`, `internal/infrastructure/cache` trong module identity — file: `src/backend/modules/identity/internal/`
- [ ] T002 [P] Bổ sung permission nhóm `tenant.*`, `school.*`, `platform.role.*`, `platform.user_role.write`, `platform.tenant.manage`, `platform.catalog.write`, `platform.sis_connection.manage`, `platform.user.manage`, `platform.cross_tenant.read` và 12 role hệ thống theo ADMIN_PORTAL §2.2–§2.3 — file: `src/backend/pkg/authz/catalog.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Owner nội dung delta `resolvePrincipal` (contracts §6): `PrincipalResolveRequest` (`issuer`, `subject`, oneOf `user_id`), `Principal.tenant_id`/`tenant_type` nullable, `is_system_admin`, `404` = không membership — đã áp sớm ở F002 T005 (M-C0) thì chỉ kiểm khớp, chưa áp thì áp ở đây — file: `specs/openapi/modules/identity.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); áp qua F002 T005
- [x] T004 [P] Thêm `ROLE_NOT_FOUND` vào `ProblemCode` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); `ROLE_NOT_FOUND` đã có trong `ProblemCode` và API_DESIGN §4.1
- [ ] T005 ent schema `Permission`, `Role`, `RolePermission`, `UserRole` (`MaxLen` theo data-model) — file: `src/backend/modules/identity/ent/schema/` — kiểm: `make migrate-diff MOD=identity name=f012_authz`, `make migrate-lint MOD=identity`
- [ ] T006 Migration trigger `role_permissions_scope_chk`, `user_roles_scope_chk`, trigger chặn sửa `is_system`, `REVOKE` ghi `identity.permissions`, view `tenant_memberships_v` thêm nhánh `user_roles`, RLS — file: `src/backend/modules/identity/migrations/`
- [ ] T007 `src/backend/cmd/migrate`: đồng bộ catalog → `identity.permissions`, seed role hệ thống + `role_permissions` (idempotent) — file: `src/backend/modules/identity/cmd/migrate/seed_authz.go`
- [ ] T008 [P] Principal cache L1 + Valkey `ew:identity:principal:<user_id>:<tenant_id|_platform>`, pub/sub invalidate, degraded mode có bulkhead — file: `src/backend/modules/identity/internal/infrastructure/cache/principal_cache.go`
- [x] T038 [P] Delta contract `createAuthSession`: sửa `summary`/`description` (không còn "Bootstrap sau đăng nhập portal"; trả `PlayerBootstrap` cho client có hồ sơ người chơi, portal không gọi — RB-115) — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)

- [ ] T039 [P] Kiểm khớp contract ADR-0090/0091 trong spec gộp (6 operation `/auth/portal/*`, `PortalLoginRequest`, `PortalTokenResponse`, `PortalSetupResult`, cookie, `X-EW-CSRF`, `PermissionSet.disabled_menus`/`disabled_actions`, ProblemCode mới) rồi `make gen MOD=identity` và sinh bảng route → action từ `x-portal-action` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `node scripts/specs/check-portal-catalog.mjs`, `make gen-check` — **2026-11-17:** đã kiểm khớp spec gộp (6 operation `/auth/portal/*` có `x-portal-shell: true`, `security: []`, schema `Portal*`, `PermissionSet.disabled_*`, ProblemCode mới; `check-portal-catalog` 0 lỗi); còn `make gen MOD=identity` và bảng route → action
- [ ] T040 ent schema `PortalPasswordResetToken` (`MaxLen`, mixin soft-delete, partial unique token đang hiệu lực theo user) — file: `src/backend/modules/identity/ent/schema/portalpasswordresettoken.go` — kiểm: `make migrate-diff MOD=identity name=f012_portal_reset`, `make migrate-lint MOD=identity`
- [x] T041 Adapter Keycloak cho portal (chờ F004 T039): direct grant client `eduworld-portal` (secret `EW_KC_PORTAL_CLIENT_SECRET_FILE`, kèm `totp`), ánh xạ lỗi (sai/khóa/thiếu OTP → `INVALID_CREDENTIALS`; chặn role → `PORTAL_ACCESS_STAFF_ONLY`; "not fully set up" → `ACCOUNT_SETUP_REQUIRED`), refresh/revoke/logout theo `sid`, Admin API (`svc-identity`): đặt mật khẩu, gỡ `UPDATE_PASSWORD`, đọc required action, `execute-actions-email` `CONFIGURE_TOTP`, đăng xuất mọi phiên portal của user — file: `src/backend/modules/identity/internal/infrastructure/keycloak/portal_auth.go` — **xong 2026-11-19 — `keycloak/portal_token.go` (token/refresh/logout của client confidential, timeout + circuit breaker, chỉ logout retry; ánh xạ lỗi theo F004 R5: `invalid_grant` → INVALID_CREDENTIALS, `not fully set up` → ACCOUNT_SETUP_REQUIRED, 401 `text/html` → PORTAL_ACCESS_STAFF_ONLY, `invalid_client` → 503; tham số OTP là `otp`), `keycloak/admin_portal.go` (tìm user theo username/email, required actions, đặt mật khẩu → `ErrKCPasswordPolicy`, gỡ `UPDATE_PASSWORD`, `execute-actions-email` không retry; đăng xuất mọi phiên dùng `LogoutUser` có sẵn); test `portal_token_test.go`, `admin_portal_test.go` với Keycloak giả `httptest` — `go test ./modules/identity/internal/infrastructure/keycloak` ok**

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Permission catalog và role hệ thống (Priority: P1) 🎯 MVP

**Goal**: Catalog hợp lệ, role seed, bất biến ba lớp.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Test F012-AC-01, F012-AC-02: `ValidateCatalog` từ chối role tenant chứa permission platform-only; danh sách role hệ thống đúng, không có `teacher`/`tenant_moderator` — file: `src/backend/pkg/authz/catalog_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Test F012-AC-03: `src/backend/cmd/migrate` đồng bộ đúng `scope_kind`; INSERT thẳng `role_permissions` (role tenant + permission platform) bị trigger chặn — file: `src/backend/modules/identity/tests/integration/authz_catalog_sync_test.go`

### Implementation cho User Story 1

- [ ] T011 [US1] Gọi `authz.ValidateCatalog` lúc khởi động api/worker; dừng nếu lỗi — file: `src/backend/modules/identity/cmd/api/main.go`

## Phase 4: User Story 2 — Phiên Admin Portal và chọn tenant sau đăng nhập (Priority: P1)

**Goal**: Nhân sự vào portal đúng tenant/phạm vi; học sinh bị chặn.

**Independent Test**: Như spec.md US2.

### Tests cho User Story 2

- [ ] T012 [P] [US2] Test F012-AC-04, F012-AC-08: `listMyTenants` 1/2 tenant/chỉ platform; luồng portal `listMyTenants` → `getMyPermissions` → `getMe` không PII thừa, không cần `createAuthSession`; không ghi `auth.portal_login` ở `listMyTenants` (audit ở `portalLogin`) — file: `src/backend/modules/identity/tests/integration/portal_session_test.go` — phải FAIL trước khi code
- [ ] T013 [P] [US2] Test F012-AC-05, F012-AC-06: token học sinh `eduworld-portal` → `PORTAL_ACCESS_STAFF_ONLY` mọi route portal (sinh từ OpenAPI); thiếu `acr` MFA / `amr` không có `otp` / `auth_time` quá 8 giờ → `MFA_REQUIRED` + `authz.denied` (TC-AUTH-101) — file: `src/backend/modules/identity/tests/integration/portal_guard_test.go`
- [ ] T014 [P] [US2] Test tenant isolation F012-AC-07, F012-AC-09, F012-AC-10: cùng access token đổi `X-Tenant-Id` sang tenant còn membership → dữ liệu tenant mới; tenant đã thu hồi membership/UserRole → `TENANT_MISMATCH` + audit (TC-AUTH-101); `tenant_ids` bị bỏ qua với người dùng tenant; System Admin `tenant_ids` + audit `cross_tenant.accessed`; `getMyPermissions` không trả permission platform cho người dùng tenant — file: `src/backend/modules/identity/tests/integration/portal_tenant_test.go`

### Implementation cho User Story 2

- [ ] T015 [US2] Command `CreateAuthSession` (resolve `(iss, sub)` → User, membership, Principal; không JIT; không gọi module khác); query `ListMyTenants`, `GetMyPermissions` (workspace), `GetMe` — file: `src/backend/modules/identity/internal/application/command/auth_session.go`
- [ ] T016 [US2] Handler `createAuthSession`, `getMe`, `logout`, `listMyTenants`, `getMyPermissions` — file: `src/backend/modules/identity/internal/presentation/http/public/auth_portal.go`

## Phase 5: User Story 3 — Principal cho mọi module (`principals:resolve`) (Priority: P1)

**Goal**: Module khác authorize bằng Principal tin cậy.

**Independent Test**: Như spec.md US3.

### Tests cho User Story 3

- [ ] T017 [P] [US3] Test F012-AC-11, F012-AC-12: Principal đúng permission/scope/version; tenant ngoài membership → `TENANT_MISMATCH` + security event — file: `src/backend/modules/identity/tests/integration/resolve_principal_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US3] Test authorization internal F012-AC-13: token người dùng → 401; caller ngoài allowlist → `403 CALLER_NOT_ALLOWED` + audit — file: `src/backend/modules/identity/tests/integration/internal_auth_test.go`
- [ ] T019 [P] [US3] Test F012-AC-14, F012-AC-15: mỗi event invalidate đúng key, quyền mới ≤ `principal_cache_ttl`; facade `identity` lỗi + không cache → bên gọi `503` (test qua adapter facade + `src/backend/pkg/modcall`) — file: `src/backend/modules/identity/tests/integration/principal_cache_test.go`
- [ ] T020 [P] [US3] Facade test `resolvePrincipal` (`make test-contract`): hiện thực facade `identity` khớp ví dụ trong spec internal; bên gọi learning, game, competition, chat test với fake facade; handler NATS request/reply cho realtime; kèm F012-AC-13 qua `make arch-check` — file: `src/backend/modules/identity/tests/contract/resolve_principal_contract_test.go`

### Implementation cho User Story 3

- [ ] T021 [US3] Query `ResolvePrincipal` (issuer/subject → user, membership, UserRole hiệu lực, scope, SIS link) — file: `src/backend/modules/identity/internal/application/query/resolve_principal.go`
- [ ] T022 [US3] Hiện thực facade `resolvePrincipal`, `getPrincipal`, `checkMemberships` của `src/backend/modules/identity/api` (interface sinh từ spec internal; không route HTTP, không `serviceauth`; `x-allowed-callers` do `make arch-check` và `src/backend/pkg/modcall` kiểm) — file: `src/backend/modules/identity/internal/presentation/facade/principals.go`
- [ ] T023 [US3] Consumer `principal-invalidate` cho `user_role.changed`, `role.updated`, `tenant_membership.changed`, `user.status_changed`, `user.access_revoked`, `tenant.updated`, `enrollment.changed` — file: `src/backend/modules/identity/internal/presentation/consumer/principal_invalidate.go`

## Phase 6: User Story 4 — Quản lý role tùy biến và role platform (Priority: P2)

### Tests cho User Story 4

- [ ] T024 [P] [US4] Test F012-AC-16, F012-AC-17, F012-AC-18: 422 `PERMISSION_NOT_ALLOWED_FOR_SCOPE`; `is_system` → 403; xóa role còn dùng → 409; sửa role phát `role.updated` có `permissions_removed[]`, cần `If-Match` + `reason` — file: `src/backend/modules/identity/tests/integration/roles_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Test tenant isolation + authorization F012-AC-19 cho `/tenant/roles*`, `/admin/roles*`, `listTenantPermissions`, `listAllPermissions` — file: `src/backend/modules/identity/tests/integration/roles_authz_test.go`

### Implementation cho User Story 4

- [ ] T026 [P] [US4] Aggregate `Role` — file: `src/backend/modules/identity/internal/domain/authz/role.go`
- [ ] T027 [US4] Command/Query role tenant + platform; handler — file: `src/backend/modules/identity/internal/application/command/roles.go`, `src/backend/modules/identity/internal/presentation/http/public/roles.go`

## Phase 7: User Story 5 — Gán/thu hồi UserRole theo phạm vi (Priority: P2)

### Tests cho User Story 5

- [ ] T028 [P] [US5] Test F012-AC-20, F012-AC-21: scope `class` → 422; mã role lạ → `ROLE_NOT_FOUND`; role portal cho học sinh / role platform cho nhân sự có tenant gốc → 422 (use case + trigger) — file: `src/backend/modules/identity/tests/integration/user_roles_rules_test.go`
- [ ] T029 [P] [US5] Test F012-AC-22, F012-AC-23, F012-AC-24: School Admin ngoài trường → `PERMISSION_DENIED`; gán/thu hồi có `reason`, `Idempotency-Key`, audit, `user_role.changed`, mất quyền ≤ TTL; `safety_moderator` thiếu `training_completed_at` → 422 — file: `src/backend/modules/identity/tests/integration/user_roles_test.go` — phải FAIL trước khi code
- [ ] T030 [P] [US5] Test tenant isolation + authorization cho `/tenant/user-roles*`, `/schools/{school_id}/user-roles`, `/admin/user-roles*` — file: `src/backend/modules/identity/tests/integration/user_roles_authz_test.go`

### Implementation cho User Story 5

- [ ] T031 [P] [US5] Aggregate `UserRole` + `ScopeRule` — file: `src/backend/modules/identity/internal/domain/authz/user_role.go`
- [ ] T032 [US5] Command `GrantUserRole`, `RevokeUserRole`, `ExpireUserRoles` (job); handler — file: `src/backend/modules/identity/internal/application/command/user_roles.go`, `src/backend/modules/identity/internal/presentation/job/user_role_expiry.go`

## Phase 8: User Story 6 — Nhân sự tạo thủ công trong tenant (Priority: P3)

### Tests cho User Story 6

- [ ] T033 [P] [US6] Test F012-AC-25, F012-AC-26: tạo nhân sự kèm UserRole ban đầu + Keycloak user; `USERNAME_TAKEN`/`EMAIL_TAKEN` (khác hoa thường, tenant khác, tài khoản soft-delete); thiếu email → 422; tenant isolation `listTenantUsers` — file: `src/backend/modules/identity/tests/integration/tenant_users_test.go` — phải FAIL trước khi code

### Implementation cho User Story 6

- [ ] T034 [US6] Command `CreateTenantUser`, `UpdateTenantUser`; query `ListTenantUsers`; handler — file: `src/backend/modules/identity/internal/application/command/tenant_users.go`, `src/backend/modules/identity/internal/presentation/http/public/tenant_users.go`

## Phase 9a: User Story 7 — Đăng nhập portal bằng form riêng qua backend (Priority: P1)

**Goal**: `/auth/portal/login`, `token:refresh`, `logout` với cookie HttpOnly + CSRF, lỗi chung, không lộ mật khẩu.

**Independent Test**: Như spec.md US7.

### Tests cho User Story 7

- [x] T042 [P] [US7] Test F012-AC-27, F012-AC-28, F012-AC-29: `ad.truong.a` không OTP và `sa.admin` có OTP → `200`, thuộc tính cookie đúng, `Cache-Control: no-store`, token có `amr`/`acr` đúng; ma trận sai username/mật khẩu/OTP/thiếu OTP/disabled/khóa tạm → cùng `401 INVALID_CREDENTIALS`, chênh lệch p50 < 50 ms; học sinh → `403 PORTAL_ACCESS_STAFF_ONLY`; User `deactivated` → `403 ACCOUNT_DEACTIVATED` + phiên Keycloak bị thu hồi (TC-AUTH-093, TC-AUTH-094, TC-AUTH-095) — file: `src/backend/modules/identity/tests/integration/portal_login_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — unit `modules/identity/internal/presentation/http/auth_portal_test.go` (`TestPortalLoginSuccess`, `TestPortalLoginGenericFailure`, `TestPortalLoginRefusals`: chuỗi `pkg/httpx` thật + spec gộp, use case thật, port giả có nhãn TEST FAKE) và integration `tests/integration/portal_auth_integration_test.go` (`TestPortalLoginIntegration`: PostgreSQL + Valkey thật, adapter Keycloak thật trước Keycloak giả `httptest`); `amr`/`acr` của Keycloak thật đã đo ở F004 R5, ở đây kiểm ánh xạ `amr ∋ otp` → `mfa`**
- [x] T043 [P] [US7] Test F012-AC-30, F012-AC-31: refresh/logout thiếu/lệch `X-EW-CSRF` → `403 CSRF_TOKEN_INVALID` không gọi Keycloak; refresh xoay vòng token + CSRF; token cũ → `401 REFRESH_TOKEN_INVALID` + xóa cookie; logout lần hai → `204`; quét log/trace/audit không có mật khẩu/OTP/refresh token mẫu (TC-AUTH-098, TC-AUTH-100) — file: `src/backend/modules/identity/tests/integration/portal_session_cookie_test.go` — **xong 2026-11-19 — `TestPortalRefreshAndLogout`, `TestPortalRefreshMaxAuthAge`, `TestPortalNoSecretsInLogsOrAudit` (quét log slog + audit với mật khẩu/OTP/refresh token/CSRF/username mẫu) trong `presentation/http/auth_portal_test.go`; phần refresh/logout/audit outbox trong `TestPortalLoginIntegration`**
- [x] T044 [P] [US7] Test F012-AC-32: vượt `portal_login_rate` theo username (nhiều thiết bị), theo `device_id`, toàn cục → `429 RATE_LIMITED` + `Retry-After`; 50 username khác nhau từ một IP vẫn qua (TC-AUTH-099) — file: `src/backend/modules/identity/tests/integration/portal_login_rate_test.go` — **xong 2026-11-19 — `TestPortalLoginRate` (username nhiều thiết bị, `device_id`, toàn cục 20/s, 50 username một client không bị chặn, refresh theo phiên) + giới hạn username qua Valkey thật và khóa Valkey chỉ chứa băm trong `TestPortalLoginIntegration`**

### Implementation cho User Story 7

- [x] T045 [US7] Command `PortalLogin`, `RefreshPortalToken`, `PortalLogout` (gọi adapter T041, kiểm `user_type = staff`/trạng thái/UserRole portal, audit, xóa mật khẩu khỏi bộ nhớ) — file: `src/backend/modules/identity/internal/application/command/portal_auth.go` — **xong 2026-11-19 — `command/portal_auth.go` (`PortalAuth.Login/Refresh/Logout`), `command/portal_common.go` (rate limit `portal_login_rate`, cấu hình), `domain/portalauth`, repo `persistence/write/portal_account_repo.go` (kiểm UserRole hiệu lực ở tenant active hoặc role platform, scope platform `portal-auth`), session store Valkey `cache/portal_store.go` (giữ thời điểm nhập mật khẩu theo `sid` để chặn refresh MFA quá `EW_OIDC_PLATFORM_MAX_AUTH_AGE`), metric `infrastructure/portalmetrics`**
- [x] T046 [US7] Handler `portalLogin`, `refreshPortalToken`, `portalLogout`: đặt/xóa cookie `__Secure-ew_portal_rt`, `__Host-ew_portal_csrf`, kiểm `X-EW-CSRF` (so sánh hằng thời gian), không log body/cookie, rate limiter lớp hai `portal_login_rate` (Valkey, khóa băm username + `device_id` + toàn cục) — file: `src/backend/modules/identity/internal/presentation/http/public/auth_portal_login.go` — **xong 2026-11-19 — thực tế ở `modules/identity/internal/presentation/http/auth_portal.go` (`PortalAuthHandlers`, `PortalAuthOperations`, `RegisterPortalAuth`); CSRF chạy trước validate OpenAPI qua `httpx.Operation.PreValidate` (mới, chỉ cho operation public); contract test `tests/contract/portal_auth_routes_test.go`; rate limit nằm ở use case (lớp application), không ở handler**

## Phase 9b: User Story 8 — Thiết lập tài khoản lần đầu và quên mật khẩu (Priority: P2)

**Goal**: `ACCOUNT_SETUP_REQUIRED` + `setup:password`, email đăng ký OTP, `password:forgot`/`password:reset`.

**Independent Test**: Như spec.md US8.

### Tests cho User Story 8

- [ ] T047 [P] [US8] Test F012-AC-33, F012-AC-34, F012-AC-35: tài khoản còn `UPDATE_PASSWORD`/`CONFIGURE_TOTP` → `403 ACCOUNT_SETUP_REQUIRED` + ticket + `required_steps`; `setupPortalPassword` thành công rồi đăng nhập được; ticket dùng lại / quá `portal_setup_ticket_ttl` → `401 SETUP_TICKET_INVALID`; mật khẩu yếu → `422` `PASSWORD_POLICY_VIOLATION`; email `CONFIGURE_TOTP` gửi tối đa một lần mỗi cửa sổ (TC-AUTH-096) — file: `src/backend/modules/identity/tests/integration/portal_setup_test.go` — phải FAIL trước khi code
- [ ] T048 [P] [US8] Test F012-AC-36, F012-AC-37: `forgotPortalPassword` với nhân sự / học sinh / không tồn tại → đều `202`, thời gian tương đương, chỉ nhân sự có email; DB chỉ có băm; token dùng lần hai / hết `portal_password_reset_ttl` / bị thay → `401 PASSWORD_RESET_TOKEN_INVALID`; reset thu hồi mọi phiên portal; tenant isolation không áp (bảng theo user) nhưng role ứng dụng không `DELETE` (TC-AUTH-097) — file: `src/backend/modules/identity/tests/integration/portal_password_reset_test.go`

### Implementation cho User Story 8

- [ ] T049 [US8] `setup_ticket` store (Valkey `GETDEL`, TTL `portal_setup_ticket_ttl`, hủy khi `user.status_changed`); command `SetupPortalPassword` + gửi `execute-actions-email` `CONFIGURE_TOTP`; handler — file: `src/backend/modules/identity/internal/application/command/portal_setup.go`, `src/backend/modules/identity/internal/infrastructure/cache/setup_ticket_store.go`
- [ ] T050 [US8] Command `ForgotPortalPassword` (luôn `202`, tạo token băm, soft-delete token cũ, email qua outbox/job — nội dung theo file dịch), `ResetPortalPassword` (dùng nguyên tử, đặt mật khẩu Admin API, thu hồi phiên); handler — file: `src/backend/modules/identity/internal/application/command/portal_password_reset.go`

## Phase 9c: User Story 9 — Chặn action đang tắt ở backend (`featuregate`) (Priority: P2)

**Goal**: Middleware `featuregate` + `disabled_menus`/`disabled_actions`.

**Independent Test**: Như spec.md US9.

### Tests cho User Story 9

- [ ] T051 [P] [US9] Test F012-AC-40, F012-AC-41 (sinh từ OpenAPI cho mọi operation có `x-portal-action`): action/menu cha tắt → `403 FEATURE_DISABLED` + `feature_key`; token game, operation `x-portal-shell`, action `can_disable = false` không bị chặn; chưa xác thực → `401` trước `featuregate`; tắt + thiếu permission → `FEATURE_DISABLED`; System Admin không header → mặc định platform; Source lỗi + cache trống → `503 UNAVAILABLE`; đổi trạng thái có hiệu lực ≤ `feature_toggle_cache_ttl` (TC-CMS-MENU-02, TC-CMS-MENU-04) — file: `src/backend/pkg/httpx/featuregate/featuregate_test.go`, `src/backend/modules/identity/tests/integration/featuregate_test.go` — phải FAIL trước khi code — **phần unit `pkg/httpx/featuregate` xong 2026-11-19 — `featuregate_test.go`, `middleware_test.go` (chuỗi httpx thật + `httpxtest`, fake Source `TEST FAKE`), `spec_test.go` (sinh từ spec gộp: 618 cặp operation/action, 62 `can_disable = false`); FAIL (build) trước khi code, `go test ./pkg/httpx/featuregate` ok; còn mở: test tích hợp `modules/identity/tests/integration/featuregate_test.go` chờ F015 (Source thật)**
- [ ] T052 [P] [US9] Test F012-AC-39: `getMyPermissions` trả `disabled_menus`/`disabled_actions` đúng phạm vi phiên, gồm action có menu cha tắt (TC-CMS-MENU-06) — file: `src/backend/modules/identity/tests/integration/my_permissions_disabled_test.go`

### Implementation cho User Story 9

- [ ] T053 [US9] `src/backend/pkg/httpx/featuregate`: port `Source` (`DisabledFor(ctx, tenantID *uuid) (Set, error)`), middleware (đọc bảng route → action sinh ở T039, chỉ `azp = eduworld-portal`, Problem `FEATURE_DISABLED` có `feature_key`, `503` khi Source lỗi), metric; gắn vào chuỗi middleware của `src/backend/cmd/core-api` sau `portal_staff` với Source của module identity (F015) — file: `src/backend/pkg/httpx/featuregate/featuregate.go`, `src/backend/cmd/core-api/main.go` — **phần `pkg/httpx` xong 2026-11-19 — port `Source`/`Set`, `Catalog` (đọc `specs/portal/menu-catalog.yaml`: menu cha, `can_disable`), `Gate` (cache trong tiến trình theo phạm vi, TTL `feature_toggle_cache_ttl` ≤ `principal_cache_ttl` qua `Options`, `Invalidate` cho `platform_settings.changed` scope `feature_flag`, single-flight, `503 UNAVAILABLE` khi Source lỗi và không có cache hợp lệ), bảng route → action đọc `x-portal-action` của spec mà `httpx.Server` đã nạp (không thêm generator), metric `eduworld_featuregate_blocked_total{feature_key}`, `eduworld_featuregate_source_errors_total`; `httpx.Deps.FeatureGate` tùy chọn (nil = tắt), chạy sau guards, trước idempotency/validation/handler; còn mở: gắn vào `cmd/core-api` (chưa dùng `httpx.Server`) với Source của module identity (F015), consumer `platform_settings.changed` gọi `Gate.Invalidate` (F015)**
- [ ] T054 [US9] Query `GetMyPermissions` bổ sung `disabled_menus`/`disabled_actions` qua `featuregate.Source` — file: `src/backend/modules/identity/internal/application/query/my_permissions.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T035 [P] Cập nhật ADMIN_PORTAL §2.1 và DATABASE_DESIGN §2.1 vị trí catalog `src/backend/pkg/authz/catalog.go` (D4), CHANGELOG — file: `docs/04-admin-portal/ADMIN_PORTAL.md`
- [ ] T036 [P] Metric `eduworld_identity_principal_cache_hit_ratio`, `eduworld_auth_tenant_mismatch_total`, `eduworld_authz_denied_total` — file: `src/backend/modules/identity/internal/infrastructure/cache/metrics.go`
- [ ] T037 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2, US3 song song → US4 → US5 → US6 → Polish.
- T003 phải merge trước khi các module khác sinh facade `identity` (`make gen MOD=identity`).
- F004 T039 (spike Keycloak) và F004 T041 (client `eduworld-portal` confidential) trước T041–T050; T039 trước T046, T053; US9 cần F015 (hiện thực `featuregate.Source`, bảng catalog) cho test tích hợp — unit test dùng fake Source có nhãn `// TEST FAKE` (R17).
- US7 trước F051 (trang `/login`); US9 trước các workspace portal F052+.

## Definition of Done

- [ ] Mọi tiêu chí F012-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
