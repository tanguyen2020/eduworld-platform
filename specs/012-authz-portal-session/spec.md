# Feature Specification: F012 — Phân quyền Permission/Role/UserRole và phiên portal (`/me/*`, `principals:resolve`)

**Feature Branch**: `012-authz-portal-session` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `identity` · **Mốc**: M-C1 · **Phụ thuộc**: F010
**Input**: Permission (trong code) → Role (DB) → UserRole (DB, scope platform/tenant/school/subject); phiên Admin Portal chỉ cho nhân sự, đăng nhập bằng form riêng của portal qua module identity (Keycloak Direct Access Grant, ADR-0091), không chọn tenant trước, chọn/đổi tenant bằng `X-Tenant-Id` + membership; menu từ `/me/permissions` (kèm `disabled_menus`/`disabled_actions`, ADR-0090) và middleware `featuregate`; Principal cho mọi module qua `principals:resolve`. FR nguồn: FR-AUTH-016, FR-AUTH-019, FR-AUTH-029, FR-AUTH-030, FR-AUTH-032, FR-CMS-001, FR-CMS-012, FR-CMS-018, FR-LMS-005, FR-ORG-008.

## Clarifications

### Session 2026-11-16

- Q: Khung `src/backend/pkg/authz` (Principal, `ValidateCatalog`) thuộc F002 hay F012? → A: F002 tạo khung; F012 điền nội dung catalog nhóm identity (`tenant.*`, `school.*`, `platform.*` liên quan), seed role hệ thống, đồng bộ bản sao `identity.permissions`.
- Q: `principals:resolve` nhận `user_id` nhưng module khác chỉ có `sub` trong token? → A: Delta contract: `PrincipalResolveRequest` nhận `issuer` + `subject` (thay thế `user_id`), `Principal.tenant_id` nullable cho phạm vi platform ([IDENTITY_MAPPING §0](../../docs/09-identity/IDENTITY_MAPPING.md)); xem [contracts](contracts/README.md) §6.
- Q: Key cache Principal? → A: `ew:identity:principal:<user_id>:<tenant_id|_platform>` (CODE_PLAN Bước 0 D7).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Portal có dùng `createAuthSession` làm bootstrap? → A: Không (RB-115): luồng portal là `listMyTenants` → `getMyPermissions` (header `X-Tenant-Id`, không query `tenant_id`) → `getMe`; ~~audit `auth.portal_login` chuyển sang lần `listMyTenants` đầu tiên của phiên~~ — thay bởi câu trả lời ADR-0091 bên dưới (audit ở `portalLogin`).
- Q: Portal đăng nhập thế nào sau [ADR-0091](../../docs/adr/ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md)? → A: Form riêng của portal gọi `portalLogin` (`POST /api/v1/auth/portal/login`, mật khẩu + OTP trong một request); module identity gọi token endpoint Keycloak bằng client confidential `eduworld-portal` (Direct Access Grant). Access token trả trong body (SPA chỉ giữ trong bộ nhớ); refresh token trong cookie `__Secure-ew_portal_rt` (`HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/portal`); `refreshPortalToken`/`portalLogout` bắt buộc header `X-EW-CSRF` khớp cookie `__Host-ew_portal_csrf` (double-submit). Mọi lỗi xác thực là `401 INVALID_CREDENTIALS` một thông điệp chung (US7).
- Q: Tài khoản còn mật khẩu tạm/chưa có OTP? → A: `403 ACCOUNT_SETUP_REQUIRED` + `setup_ticket` (một lần, `portal_setup_ticket_ttl`) + `required_steps[]`; đổi mật khẩu bằng `setupPortalPassword`; đăng ký OTP qua email `execute-actions-email` `CONFIGURE_TOTP` (trang action-token Keycloak — ngoại lệ duy nhất); quên mật khẩu bằng `forgotPortalPassword` (luôn `202`) + `resetPortalPassword` (token hash, một lần, `portal_password_reset_ttl`) (US8).
- Q: Đổi tenant còn xin token mới với scope `organization:{alias}`? → A: Không; tenant của request do `X-Tenant-Id` + kiểm tra membership quyết định (ADR-0065, ADR-0091 mục 4), cùng access token (F012-AC-07).
- Q: Middleware `featuregate` (ADR-0090) thuộc feature nào? → A: F012 sở hữu middleware trong `src/backend/pkg/httpx` và port `featuregate.Source` (thứ tự: xác thực → `tenant` → `portal_staff` → `featuregate` → handler → `authz.Require`) và trường `disabled_menus`/`disabled_actions` của `getMyPermissions`; F015 sở hữu dữ liệu (`platform.feature_flags`, `platform.portal_menu_catalog`), API System Admin và hiện thực `featuregate.Source` trong module identity (US9).
- Q: Phần đăng nhập portal có phụ thuộc kết quả spike Keycloak không? → A: Có — F004 T039 (claim `amr`/`acr`, lỗi khi còn required action) phải xong trước T041–T050 của feature này [Confirmed].

### Session 2026-11-19 (triển khai)

- Q: Loại bảng `roles`, `role_permissions`, `user_roles`? → A: T* (mixin `TenantOptional`): dòng `tenant_id` NULL là role hệ thống/dùng chung và UserRole platform. RLS cho phép mọi tenant **đọc** dòng dùng chung của `roles`/`role_permissions` (cần để dựng Principal), ghi chỉ trong tenant của phiên; dòng dùng chung do `identity_migrator` (seed) hoặc `identity_platform` ghi. Khóa ghép `role_permissions` thành id surrogate + unique index; gỡ permission khỏi role = soft-delete.
- Q: Tên hiển thị permission/role trong `identity.permissions`/`roles`? → A: [Confirmed] Bằng chính mã (`name_vi = name_en = code`), `group_code` = tiền tố trước dấu chấm; portal dịch mã qua file dịch. Chưa có bảng tên trong ADMIN_PORTAL §2.2.
- Q: Đồng bộ catalog chạy ở đâu? → A: `cmd/migrate` gọi `identity.SyncAuthzCatalog` ngay sau Atlas của module identity, một transaction, idempotent: id role hệ thống tất định (UUID v5 theo mã), permission/quyền rời catalog bị soft-delete, `catalog_version` = băm catalog. Trigger chặn `identity_app`/`identity_platform` sửa role hệ thống và quyền của nó (lớp 2 của F012-AC-17).
- Q: Membership nhân sự? → A: `identity.tenant_memberships_v` thêm nhánh `member_kind = staff` từ UserRole tenant còn hiệu lực (một dòng mỗi tenant/user); consumer `keycloak-membership` của F010 dùng lại view này.
- Q: `version` của Principal? → A: `users.version`; gán/thu hồi/hết hạn UserRole và đổi trạng thái tăng version trong cùng transaction.
- Q: Kiểm tra "role vượt quyền người gán" (F012-AC-22) áp cho ai? → A: Chỉ người gán có `tenant.user_role.write` giới hạn theo trường (School Admin); grant toàn tenant (`tenant_admin`) gán mọi role tenant của ADMIN_PORTAL §2.3 (role này không có đủ `lms.*` của `school_admin`).
- Q: UserRole hết hạn? → A: [Confirmed] Job `user-role-expiry` soft-delete dòng quá `valid_to` (`delete_reason = expired`) và phát `user_role.changed` op `expired` (`actor_id` null).
- Q: Tạo nhân sự đặt mật khẩu thế nào? → A: [Confirmed] Không đặt mật khẩu; sau khi tạo, gửi email action Keycloak `UPDATE_PASSWORD`; lỗi gửi chỉ ghi log (đặt lại qua quên mật khẩu). `updateTenantUser` chưa đổi username/email (cần đồng bộ Keycloak) — OQ F012-1.
- Q: `createAuthSession`, `logout` (T015/T016)? → A: Không làm trong F012: portal không dùng `createAuthSession` (F012-AC-08) và `PlayerBootstrap` cần hồ sơ người chơi của module game — OQ F012-2 [Confirmed: F013/F014].
- Q: `featuregate.Source`? → A: Port và middleware ở `pkg/httpx/featuregate` (F012); hiện thực Source, consumer `platform_settings.changed` → `Gate.Invalidate` và cache Valkey `ew:fg:*` do F015; `getMyPermissions` trả `503` khi Source lỗi.
- Q: Đăng nhập portal (US7/US8) — điểm lệch? → A: `auth_time` của phiên MFA lưu Valkey theo `sid` (token direct grant không có `auth_time`); rate limit fail-open khi Valkey lỗi (APISIX + brute-force Keycloak vẫn chặn); email quên mật khẩu gửi từ goroutine giới hạn (chưa có hạ tầng thông báo dùng outbox) [Confirmed]; tên audit theo plan (`auth.portal_logout`…), mô tả OpenAPI cần thống nhất.
- Q: Gắn binary? → A: Vẫn theo OQ F010-2: F012 cung cấp `Register`, `RegisterPortalAuth`, facade, job và adapter; composition root `cmd/core-api`/`cmd/worker` chưa gắn.
- Q: Song song hóa F012? → A: Data model (Pha 0) làm trước; `featuregate` (`pkg/httpx`) và đăng nhập portal (US7/US8) làm song song trên nhánh riêng, merge sau gate.

### Session 2026-11-20 (follow-up F059, Workspace D)

- Q: `listPlatformUsers`, `createPlatformUser` (OQ F059-1, T004 của F059)? → A: Thêm vào contract và module identity. `GET /admin/users` (`platform.user.manage`): vắng `tenant_ids` = chỉ nhân sự platform (`primary_tenant_id = null`); có `tenant_ids` (CrossTenantScope, `platform.cross_tenant.read` + MFA, audit `cross_tenant.accessed`) = người có membership trong các tenant đó, `User.tenant_ids[]` (maxItems 50) chỉ gồm tenant trong phạm vi; không có truy vấn mọi người dùng mọi tenant (AGENTS R8) nên bỏ cờ `platform_only` của đề xuất F059 (vắng `tenant_ids` đã là "chỉ nhân sự platform"); `user_roles` để trống (dùng `listAllUserRoles?user_id=`). `POST /admin/users` (`platform.user.manage` + `platform.user_role.write`, `Idempotency-Key`, `PlatformUserCreateRequest` {username, email, display_name, locale, `initial_role_ids[]` 1–20 role `platform`, reason}): cùng đường Keycloak Admin của `createTenantUser` (user không mật khẩu + email `UPDATE_PASSWORD`, realm role `platform_staff` để OTP bắt buộc), role kiểm trước khi gọi Keycloak (role `tenant`, `safety_moderator` thiếu `training_completed_at` → `422`, gán sau bằng `grantPlatformUserRole`), lỗi DB vô hiệu hóa user Keycloak; phát `user.provisioned` (`tenant_id = null`), audit `user.created`.
- Q: `getIntegrationOverview` (OQ F059-2, T009 của F059)? → A: `GET /admin/integrations`, chỉ đọc, any-of `platform.sis_connection.manage`/`platform.tenant.manage` (khớp `action.platform_admin.integrations.view`, ADR-0090). Keycloak: issuer (phần tử đầu `EW_OIDC_ISSUERS`), realm, client id + mục đích (`portal`, `game`, `admin_api`), `platform_mfa_required = true`, `platform_auth_max_age_seconds`; APISIX: `EW_GATEWAY_PUBLIC_BASE_URL`/`EW_GATEWAY_INTERNAL_BASE_URL` tùy chọn (null khi trống; URL có user/password, query, fragment bị từ chối lúc khởi động); SIS: số connection theo `status` mọi tenant (bypass `system_admin_cross_tenant` có audit). Không bao giờ đọc biến secret. [Confirmed] `route_groups[]` (nhóm prefix → gateway → service) của đề xuất F059 chưa làm: nguồn là `deploy/apisix/routes/routes.yaml` (route-as-code) không có trong image core-api — xem OQ F059-12.
- Q: `TenantUserCreateRequest.initial_user_roles[].scope_type` còn `class` (OQ F059-4)? → A: Đã bỏ trước đó (enum `tenant`/`school`/`subject`, F052); backend vẫn từ chối `class` (ADR-0072). Không đổi thêm.
- Q: `platform.audit.read` hay `platform.audit.read_all` (OQ F059-11)? → A: `platform.audit.read` — đúng catalog `src/backend/pkg/authz/catalog.go`, OpenAPI, `specs/portal/menu-catalog.yaml` và ADMIN_PORTAL §2.2 (F015 đã gỡ `read_all`); không có permission `platform.audit.read_all`.

### Session 2026-11-20 (owner chốt)

- OQ F012-1 (đổi username/email) → **Chốt:** `updateTenantUser` (và `updatePlatformUser` nếu có) nhận đổi `email`: kiểm trùng `lower(email)` toàn hệ thống (ADR-0067, ADR-0069), ghi trong transaction + outbox, worker `keycloak-provisioner` cập nhật email Keycloak (đặt `emailVerified = false`); **username không đổi qua API** (realm `editUsernameAllowed = false`, KEYCLOAK_SSO) — cần username khác thì tạo tài khoản mới. Thống nhất với OQ F016-10.
- OQ F012-2 → đã [Confirmed] (F013/F014), không còn việc.
- OQ F051-3 (`getMe` bắt buộc `X-Tenant-Id`) → **Chốt:** `getMe` dùng `TenantIdHeaderOptional`: nhân sự platform không gửi header nhận hồ sơ ở phạm vi platform; người dùng tenant thiếu header vẫn `400 TENANT_HEADER_REQUIRED` (ADR-0065).
- OQ F059-12 (`route_groups[]` của màn Tích hợp) → **Chốt:** thêm `IntegrationOverview.apisix.route_groups[]` (`prefix` ≤ 256, `gateway` ∈ {`public`, `internal`}, `service` ≤ 64; maxItems 100). Danh mục **sinh lúc build** từ `deploy/apisix/routes/routes.yaml` (`make gen` → file nhúng `go:embed` trong module identity, `gen-check` chặn lệch) — vẫn một nguồn route-as-code, `core-api` không đọc file triển khai lúc chạy. F059-AC-22 yêu cầu nhóm prefix → service.
- Triển khai T055–T057 (2026-11-20): email đổi được phát bằng event mới `user.email_changed` (chỉ `user_id`, `email_verified = false`; không chứa địa chỉ — R14) và do consumer sẵn có `identity.keycloak_access` (worker đồng bộ tài khoản sang Keycloak) chép sang Keycloak thay cho tên `keycloak-provisioner` ở trên (consumer đó chỉ tạo tài khoản học sinh SIS); Keycloak báo trùng email → DLQ cho vận hành. `route_groups`: một mục cho mỗi URI × gateway (`both` → public + internal), `service` = upstream (`core-api`, `realtime`), bỏ `*` cuối. Route APISIX `tenant-optional` nhận `/api/v1/auth/me` để khớp `getMe` header tùy chọn.

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F012-AC-06 sửa: không còn `403 MFA_REQUIRED`; người không có role platform → `403 PERMISSION_DENIED`; phiên `platform_staff` quá 8 giờ từ lúc nhập mật khẩu → `refreshPortalToken` trả `401 REFRESH_TOKEN_INVALID`. F012-AC-27, AC-28, AC-31, AC-33, AC-34 sửa: `portalLogin` không có `otp`, `PortalTokenResponse` không có `mfa`, thông điệp lỗi chung "Tên đăng nhập hoặc mật khẩu chưa đúng", `required_steps` chỉ `update_password`, `PortalSetupResult` không có `otp_setup_email_sent`. F012-AC-35 bỏ (không gửi email `CONFIGURE_TOTP`, bỏ khóa Valkey `portal_otp_email`, audit `auth.portal_otp_setup_email_sent`). Bản ghi phiên Valkey đổi `{user_id, auth_time, mfa}` → `{user_id, auth_time, platform}` (platform = realm role `platform_staff` trong token). `getIntegrationOverview` bỏ `platform_mfa_required`, giữ `platform_auth_max_age_seconds`. Nhắc OTP/MFA trong plan, tasks (T004, T009, T013, T039–T050, T058, T060), quickstart, Clarifications cũ là lịch sử; test hiện hành: `sa.admin` đăng nhập chỉ bằng mật khẩu vào được route platform. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-21 (sửa lỗi E2E portal trên stack local)

- Q: Keycloak 26.4 trả gì ở bước "Allow access only for role" khi module identity gửi `Accept: application/json`? → A: `401 application/json` với `{"error": "<thông điệp denyErrorMessage đã dịch>"}` (không phải mã OAuth, không có `error_description`); chỉ request không đòi JSON mới nhận trang `text/html` như spike R5 của F004 (research (d)). Module identity coi `401` có `error` không phải mã OAuth (RFC 6749 §5.2: chữ thường + `_`) hoặc `error = access_denied` là flow từ chối → `403 PORTAL_ACCESS_STAFF_ONLY` (portal) / `403 GAME_ACCESS_STUDENT_ONLY` (game) theo F012-AC-29; các mã OAuth khác (`invalid_client`, `invalid_request`…) vẫn là lỗi cấu hình (`503`). Không lộ thêm gì: bước này chỉ chạy sau khi mật khẩu đúng, sai mật khẩu vẫn `401 INVALID_CREDENTIALS` (ADR-0091). Timeout gọi token endpoint của client portal dùng `keycloak_token_call_timeout` (`EW_KC_TOKEN_TIMEOUT`, 3 s) như client game, không dùng `EW_EXT_CLIENT_TIMEOUT`.
- Q: `school_admin` (`ad.truong.a`) và `academic_manager` (`qlcm.truong.a`) nhận `403` ở `listTenantSchools` nên bộ chọn trường của School Learning Dashboard, SIS sync, hoạt động học sinh, cài đặt, nhật ký trống — role nào đọc trường? → A: Thêm `tenant.school.read` vào role hệ thống `school_admin` và `academic_manager` (ADMIN_PORTAL §2.3; `tenant_admin` đã có qua `tenant.*`); đồng bộ vào `identity.role_permissions` bằng `SyncAuthzCatalog` của `cmd/migrate`. `listTenantSchools` trả **chỉ** các trường của grant giới hạn trường (grant không giới hạn trường — scope tenant hoặc môn — thấy mọi trường của tenant phiên); `getTenantSchool` một trường ngoài grant → `403 PERMISSION_DENIED`; tenant khác → `404` (R8). Người không có permission vẫn `403` + `authz.denied`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Permission catalog và role hệ thống (Priority: P1)

Đội phát triển khai báo permission trong code; role hệ thống được seed; role tenant không bao giờ chứa permission platform.

**Why this priority**: Mọi endpoint của mọi module cần permission cụ thể (R7, R20).

**Independent Test**: Khởi động module identity với catalog hợp lệ → `identity.permissions` khớp catalog; catalog có role tenant chứa `game.quest.write` → tiến trình không start.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-01 | HỆ THỐNG PHẢI có role hệ thống tenant `tenant_admin`, `school_admin`, `academic_manager`, `tenant_content_editor`, `tenant_content_reviewer` và platform `system_admin`, `content_admin`, `content_reviewer`, `game_designer`, `liveops`, `competition_admin`, `safety_moderator`; không có `teacher`, `tenant_moderator` | FR-CMS-001 · ADR-0072 | TC-CMS-AUTHZ-06 |
| F012-AC-02 | NẾU catalog có role `tenant` chứa permission `game.*`, `competition.config.*`, `platform.*`, `moderation.case.*`, `moderation.sanction.*`, `chat.sensitive_terms.write`, `chat.policy.toggle`, `chat.tenant_terms.write`, `chat.tenant_category.toggle`, THÌ mọi binary (`core-api`, `realtime`, `worker`) PHẢI từ chối khởi động | FR-CMS-012 · FR-AUTH-016 · ADR-0027 | TC-CMS-AUTHZ-04 |
| F012-AC-03 | KHI deploy, module identity PHẢI đồng bộ bản sao đọc-chỉ của catalog vào kho permission với `scope_kind`; ghi trực tiếp liên kết role tenant ↔ permission platform PHẢI bị lớp DB chặn | FR-AUTH-016 · ADR-0066 | TC-AUTH-076 |

---

### User Story 2 — Phiên Admin Portal và chọn tenant sau đăng nhập (Priority: P1)

Nhân sự đăng nhập portal (username hoặc email + mật khẩu, không MFA với mọi role — ADR-0094), không chọn tenant trước; 1 tenant vào thẳng, nhiều tenant chọn sau, đổi trên topbar; học sinh bị chặn.

**Why this priority**: Mốc M-C1 yêu cầu `ad.truong.a` đăng nhập portal thấy đúng workspace.

**Independent Test**: `ad.truong.a` (1 tenant) → `listMyTenants` 1 dòng; `qlcm.truong.a` (2 tenant) → 2 dòng, đổi tenant không nhập lại mật khẩu; token học sinh → `403 PORTAL_ACCESS_STAFF_ONLY`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-04 | KHI nhân sự gọi `listMyTenants` (không cần `X-Tenant-Id`), module identity PHẢI trả các tenant có membership và UserRole portal hiệu lực kèm `roles[]` tóm tắt và cờ `has_platform_role` | FR-AUTH-030 · ADR-0067 | TC-AUTH-072 |
| F012-AC-05 | NẾU token `azp = eduworld-portal` thuộc `user_type = student`, hoặc nhân sự không có UserRole portal hiệu lực ở tenant nào và không có role platform, THÌ HỆ THỐNG PHẢI trả `403 PORTAL_ACCESS_STAFF_ONLY` ở mọi route portal và ghi audit | FR-AUTH-029 · AC-AUTH-025 | TC-AUTH-070 |
| F012-AC-06 | NẾU người không có role platform gọi route platform (`/admin/*`, `/game-admin/*`), THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` và audit `authz.denied`; NẾU phiên của người có realm role `platform_staff` đã quá `EW_OIDC_PLATFORM_MAX_AUTH_AGE` (8 giờ) từ lúc nhập mật khẩu, THÌ `refreshPortalToken` PHẢI trả `401 REFRESH_TOKEN_INVALID` (người dùng nhập lại mật khẩu); HỆ THỐNG PHẢI KHÔNG yêu cầu `acr`/`amr` hay trả `MFA_REQUIRED` (sửa theo ADR-0094) | FR-AUTH-029 · ADR-0066 · ADR-0091 mục 2, 4 · ADR-0094 | TC-CMS-WS-02, TC-AUTH-101 |
| F012-AC-07 | KHI nhân sự thuộc nhiều tenant đổi tenant, HỆ THỐNG PHẢI chấp nhận cùng access token với `X-Tenant-Id` mới và xác định tenant của request chỉ bằng header + kiểm tra membership/UserRole portal hiệu lực (không dùng claim `organization`, không xin token mới); NẾU header là tenant không còn membership hoặc UserRole portal hiệu lực, THÌ HỆ THỐNG PHẢI trả `403 TENANT_MISMATCH` + audit; một request PHẢI không trả dữ liệu của hai tenant | FR-AUTH-019 · FR-AUTH-030 · FR-ORG-008 · AC-AUTH-012 · ADR-0091 mục 4 | TC-CMS-LOGIN-03, TC-AUTH-101 |
| F012-AC-08 | KHI nhân sự đăng nhập portal, HỆ THỐNG PHẢI dựng phiên bằng `listMyTenants` → (chọn tenant) → `getMyPermissions` (header `X-Tenant-Id`) → `getMe`, **không** qua `createAuthSession` (endpoint này trả `PlayerBootstrap` của hồ sơ người chơi); các response PHẢI không có PII thừa và audit đăng nhập portal `auth.portal_login` PHẢI được ghi ở `portalLogin` (F012-AC-27), không ở `listMyTenants` | FR-AUTH-029 · FR-AUD-001 · [API_DESIGN `/auth/session`](../../docs/07-backend/API_DESIGN.md) | TC-CMS-LOGIN-02 |
| F012-AC-09 | TRONG KHI người dùng là người dùng tenant, HỆ THỐNG PHẢI bỏ qua tham số `tenant_ids`; chỉ `system_admin` dùng `tenant_ids` để đọc liên tenant và mỗi lần PHẢI ghi audit `cross_tenant.accessed` | FR-ORG-008 · ADR-0065 | TC-CMS-TEN-01 |
| F012-AC-10 | KHI gọi `getMyPermissions`, module identity PHẢI trả role, permission hiệu lực và danh sách workspace của tenant trong header (System Admin bỏ header = phạm vi platform); người dùng tenant không bao giờ nhận `game.*`, `competition.config.*`, `platform.*` | FR-CMS-012 | TC-CMS-AUTHZ-06 |
| F012-AC-39 | KHI gọi `getMyPermissions`, module identity PHẢI trả thêm `disabled_menus[]` và `disabled_actions[]` hiệu lực cho phạm vi phiên (tenant của header; System Admin không header = mặc định platform), trong đó `disabled_actions` gồm cả action có menu cha đang tắt; giá trị PHẢI lấy từ cùng nguồn với `featuregate` (`featuregate.Source`) | FR-CMS-018 · FR-CMS-012 · ADR-0090 mục 3 | TC-CMS-MENU-06 |

---

### User Story 3 — Principal cho mọi module (`principals:resolve`) (Priority: P1)

Module khác (learning, game, competition, chat; realtime qua NATS request/reply) lấy Principal (role, permission, scope, tenant) theo token người dùng và tenant của request.

**Why this priority**: Mọi module cần authorization nghiệp vụ trong Go (R7).

**Independent Test**: Module `learning` gọi `resolvePrincipal` qua facade `identity` cho `qlcm.truong.a` tenant `th-demo-a` → scope `subject` Toán; thu hồi UserRole → Principal mới mất permission trong ≤ 60 s.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-11 | KHI module có trong `x-allowed-callers` gọi `resolvePrincipal` qua facade `identity` với tenant thuộc membership của user, module identity PHẢI trả Principal gồm `user_type`, `status`, `roles`, `permissions` (hợp UserRole hiệu lực `valid_from ≤ now < valid_to`, chưa xóa), `scopes` (`tenant`/`school`/`subject`), `version` | FR-AUTH-016 · FR-LMS-005 | TC-TEN-002 |
| F012-AC-12 | NẾU tenant yêu cầu không thuộc membership của user, THÌ module identity PHẢI trả `TENANT_MISMATCH` và ghi security event | FR-AUTH-019 · ADR-0065 | TC-TEN-011 |
| F012-AC-13 | NẾU một module không có trong `x-allowed-callers` của `resolvePrincipal` import `src/backend/modules/identity/api`, THÌ `make arch-check` PHẢI fail; `resolvePrincipal` PHẢI KHÔNG được mở thành route HTTP (public hay nội bộ) | FR-AUTH-016 · ADR-0074 | TC-SVC-006 |
| F012-AC-14 | KHI có `user_role.changed`, `role.updated`, `tenant_membership.changed`, `user.status_changed`, `user.access_revoked` hoặc `tenant.updated`, HỆ THỐNG PHẢI xóa cache Principal liên quan để quyền mới có hiệu lực trong ≤ `principal_cache_ttl` | FR-AUTH-016 · ADR-0066 | TC-AUTH-078 |
| F012-AC-15 | NẾU module identity không trả lời và bên gọi không còn cache hợp lệ, THÌ bên gọi PHẢI trả `503 UNAVAILABLE` (không fail-open) | FR-AUTH-016 · NFR-AVAIL-005 | TC-SVC-011 |

---

### User Story 4 — Quản lý role tùy biến và role platform (Priority: P2)

Tenant Admin tạo role tùy biến chỉ từ permission tenant [Confirmed]; System Admin quản lý role platform; role hệ thống chỉ đọc.

**Why this priority**: Nhà trường cần điều chỉnh quyền; không chặn đăng nhập.

**Independent Test**: `ad.donvi` tạo role "Tổ trưởng Toán" từ `lms.assignment.write` → thành công; thêm `platform.cross_tenant.read` → 422.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-16 | NẾU role tùy biến tenant chứa permission `scope_kind = platform`, THÌ module identity PHẢI trả `422 VALIDATION_FAILED` với `errors[].code = PERMISSION_NOT_ALLOWED_FOR_SCOPE` và audit | FR-AUTH-032 · FR-AUTH-016 | TC-CMS-RBAC-01 |
| F012-AC-17 | NẾU sửa/xóa role `is_system`, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED`; xóa role còn UserRole hiệu lực PHẢI trả `409 CONFLICT` | FR-AUTH-032 | TC-CMS-RBAC-05 |
| F012-AC-18 | KHI sửa role (bắt buộc `If-Match` và `reason`), module identity PHẢI phát `role.updated` kèm `permissions_added[]`, `permissions_removed[]` và mọi người có role mất permission trong ≤ `principal_cache_ttl` | FR-AUTH-032 · ADR-0066 | TC-CMS-RBAC-04 |
| F012-AC-19 | HỆ THỐNG PHẢI chỉ cho `tenant.role.write` (tenant của phiên) và `platform.role.write` (role platform/dùng chung) tạo/sửa role; người dùng tenant khác nhận `404` | FR-AUTH-032 · NFR-SEC-006 | TC-TEN-001 |

---

### User Story 5 — Gán/thu hồi UserRole theo phạm vi (Priority: P2)

Tenant Admin gán UserRole trong tenant; School Admin gán trong trường mình; System Admin gán role platform; phạm vi `tenant`/`school`/`subject`, có thời hạn.

**Why this priority**: Cần để có Academic Manager, School Admin thật trên môi trường pilot.

**Independent Test**: `ad.truong.a` gán `academic_manager` scope môn Toán cho `qlcm.truong.a` → menu cập nhật ≤ 60 s; gán ở trường B → `403 PERMISSION_DENIED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-20 | KHI gán UserRole, module identity PHẢI chỉ chấp nhận `scope_type ∈ {tenant, school, subject}` với role tenant và `platform` với role platform; scope `class` hoặc mã role không tồn tại PHẢI trả `422 VALIDATION_FAILED` / `422 ROLE_NOT_FOUND` | FR-AUTH-016 · FR-LMS-005 · ADR-0072 | TC-AUTH-077 |
| F012-AC-21 | NẾU gán role portal cho học sinh, role platform cho nhân sự có tenant gốc, hoặc role tenant cho nhân sự platform, THÌ HỆ THỐNG PHẢI từ chối `422` ở use case và lớp DB | FR-AUTH-016 · ADR-0081 | TC-CMS-RBAC-02 |
| F012-AC-22 | NẾU School Admin gán UserRole ngoài trường mình hoặc role có permission vượt quyền của chính mình, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | FR-AUTH-032 | TC-CMS-RBAC-06 |
| F012-AC-23 | KHI gán/thu hồi UserRole (bắt buộc `reason`, `Idempotency-Key` khi gán), module identity PHẢI ghi `source = manual`, `granted_by`, audit, phát `user_role.changed`, và quyền bị thu hồi PHẢI mất trong ≤ `principal_cache_ttl` | FR-AUTH-032 · FR-AUTH-016 | TC-CMS-RBAC-03 |
| F012-AC-24 | KHI gán role `safety_moderator`, module identity PHẢI yêu cầu `training_completed_at` | FR-AUTH-016 · ADR-0072 | — |

---

### User Story 6 — Nhân sự tạo thủ công trong tenant (Priority: P3)

Tenant Admin thêm nhân sự (họ tên, username, email bắt buộc, role ban đầu); trùng định danh bị chặn.

**Why this priority**: SIS không gửi nhân sự (ADR-0072); trước khi có màn này dùng seed.

**Independent Test**: `ad.donvi` tạo nhân sự email đã dùng ở tenant khác (khác hoa thường) → `409 EMAIL_TAKEN`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-25 | KHI tạo nhân sự, module identity PHẢI tạo Keycloak user + `identity.users` (`staff`, `primary_tenant_id` = tenant phiên) + ít nhất một UserRole ban đầu trong một use case, có `Idempotency-Key` và audit | FR-AUTH-032 · FR-AUTH-029 | — |
| F012-AC-26 | NẾU username/email đã thuộc tài khoản khác (không phân biệt hoa thường, trên SSO hoặc trong tenant), THÌ HỆ THỐNG PHẢI trả `409 USERNAME_TAKEN`/`409 EMAIL_TAKEN`; thiếu email PHẢI trả `422 VALIDATION_FAILED` | FR-AUTH-031 · ADR-0069 | TC-AUTH-081 |

---

### User Story 7 — Đăng nhập portal bằng form riêng qua backend (Priority: P1)

Nhân sự nhập tên đăng nhập/email và mật khẩu trên trang `/login` của portal (không mã xác thực — ADR-0094); một request, một thông điệp lỗi chung; phiên giữ bằng cookie refresh HttpOnly và CSRF double-submit.

**Why this priority**: ADR-0091 thay đăng nhập OIDC chuyển hướng; M-C1 cần `ad.truong.a` và `sa.admin` vào được portal.

**Independent Test**: `ad.truong.a` và `sa.admin` đăng nhập chỉ bằng mật khẩu → `200` + cookie (ADR-0094); mật khẩu sai / username không tồn tại → cùng `401 INVALID_CREDENTIALS`; tải lại trang → `refreshPortalToken` có `X-EW-CSRF` → token mới; thiếu header → `403 CSRF_TOKEN_INVALID`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-27 | KHI `portalLogin` nhận `username` (username hoặc email), `password`, `device_id`, `locale` hợp lệ (không có `otp` — ADR-0094), module identity PHẢI gọi token endpoint Keycloak bằng client confidential `eduworld-portal` (Direct Access Grant), kiểm tra `user_type = staff`, trạng thái User và có UserRole portal hoặc role platform, rồi trả `200` `PortalTokenResponse` (`access_token`, `expires_in`, `refresh_expires_in`, `auth_time`) với `Cache-Control: no-store`, đặt cookie `__Secure-ew_portal_rt` (`HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/portal`) và `__Host-ew_portal_csrf` (không HttpOnly, `Secure; SameSite=Strict; Path=/`), ghi audit `auth.portal_login` (`result = success`) | FR-AUTH-029 · AC-AUTH-033 · ADR-0091 mục 1, 4 · ADR-0094 | TC-AUTH-093 |
| F012-AC-28 | NẾU username không tồn tại, mật khẩu sai, tài khoản bị Keycloak vô hiệu hóa hoặc đang khóa tạm do brute-force, THÌ module identity PHẢI trả `401 INVALID_CREDENTIALS` cùng status, cùng `code`, cùng thông điệp "Tên đăng nhập hoặc mật khẩu chưa đúng", chênh lệch thời gian phản hồi p50 < 50 ms, và ghi audit thất bại không chứa username thô | FR-AUTH-029 · AC-AUTH-033 · ADR-0091 mục 1 | TC-AUTH-094 |
| F012-AC-29 | NẾU Keycloak chặn ở bước "Allow access only for role" (tài khoản học sinh, mật khẩu đúng), THÌ module identity PHẢI trả `403 PORTAL_ACCESS_STAFF_ONLY` và audit; NẾU Keycloak xác thực thành công nhưng User EduWorld `deactivated`, THÌ PHẢI trả `403 ACCOUNT_DEACTIVATED` và thu hồi phiên Keycloak vừa tạo | FR-AUTH-029 · AC-AUTH-025 · ADR-0091 mục 5 | TC-AUTH-095 |
| F012-AC-30 | KHI `refreshPortalToken` hoặc `portalLogout` được gọi, module identity PHẢI yêu cầu cookie `__Secure-ew_portal_rt` và header `X-EW-CSRF` bằng giá trị cookie `__Host-ew_portal_csrf`; NẾU thiếu hoặc lệch, THÌ PHẢI trả `403 CSRF_TOKEN_INVALID` mà không gọi Keycloak; refresh thành công PHẢI xoay vòng refresh token và CSRF; refresh token hết hạn, đã thu hồi hoặc bị dùng lại PHẢI trả `401 REFRESH_TOKEN_INVALID` và xóa cả hai cookie; logout PHẢI thu hồi refresh token và phiên Keycloak (`sid`), xóa cookie, trả `204` kể cả khi phiên đã kết thúc | FR-AUTH-029 · AC-AUTH-036 · ADR-0091 mục 4 | TC-AUTH-098 |
| F012-AC-31 | HỆ THỐNG PHẢI không ghi body, cookie, header `X-EW-CSRF` của `/auth/portal/*` vào log, trace hay audit; PHẢI xóa mật khẩu khỏi bộ nhớ sau khi gọi Keycloak (best effort); PHẢI không lưu refresh token thô ở phía server và không dùng `Idempotency-Key` cho các operation này | FR-AUTH-029 · FR-AUTH-003 · ADR-0091 mục 5 · AGENTS R14 | TC-AUTH-100 |
| F012-AC-32 | NẾU một username (đã chuẩn hóa, băm), một `device_id` hoặc toàn hệ thống vượt `portal_login_rate` ở `portalLogin`, `forgotPortalPassword`, `setupPortalPassword`, `resetPortalPassword` hay `refreshPortalToken`, THÌ module identity (lớp hai sau APISIX) PHẢI trả `429 RATE_LIMITED` + `Retry-After`; HỆ THỐNG PHẢI KHÔNG giới hạn theo IP đơn lẻ | FR-AUTH-029 · NFR-SEC-003 · ADR-0091 mục 5 · AGENTS R30 | TC-AUTH-099 |

---

### User Story 8 — Thiết lập tài khoản lần đầu và quên mật khẩu (Priority: P2)

Nhân sự mới đổi mật khẩu tạm trên màn riêng của portal, đăng ký ứng dụng xác thực qua link email (trang Keycloak duy nhất còn dùng), và tự đặt lại mật khẩu khi quên.

**Why this priority**: Tài khoản nhân sự do Tenant Admin/System Admin tạo kèm mật khẩu tạm (F012-AC-25); không có màn Keycloak để đổi.

**Independent Test**: Tài khoản còn `UPDATE_PASSWORD` đăng nhập → `403 ACCOUNT_SETUP_REQUIRED` + ticket → `setupPortalPassword` → đăng nhập lại được; `forgotPortalPassword` với email nhân sự, email học sinh, email không tồn tại → đều `202`, chỉ nhân sự nhận email; link dùng lần hai → `401 PASSWORD_RESET_TOKEN_INVALID`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-33 | NẾU Keycloak báo tài khoản chưa thiết lập xong (còn required action) sau khi mật khẩu đúng, THÌ module identity PHẢI trả `403 ACCOUNT_SETUP_REQUIRED` kèm `setup_ticket` (ngẫu nhiên, dùng một lần, hết hạn theo `portal_setup_ticket_ttl`, gắn `user_id`, lưu Valkey, không chứa mật khẩu) và `required_steps[]` (chỉ `update_password`; `configure_otp` bỏ theo ADR-0094), không cấp token và ghi audit | FR-AUTH-029 · AC-AUTH-034 · ADR-0091 mục 3 | TC-AUTH-096 |
| F012-AC-34 | KHI `setupPortalPassword` nhận `setup_ticket` hợp lệ và `new_password`, module identity PHẢI đặt mật khẩu qua Keycloak Admin API (client `svc-identity`, áp password policy của realm), gỡ required action `UPDATE_PASSWORD`, tiêu hủy ticket, audit và trả `PortalSetupResult` (`remaining_steps[]`; `otp_setup_email_sent` bỏ theo ADR-0094); NẾU ticket hết hạn, đã dùng hoặc không tồn tại, THÌ PHẢI trả `401 SETUP_TICKET_INVALID`; NẾU mật khẩu vi phạm policy, THÌ PHẢI trả `422 VALIDATION_FAILED` với `errors[].code = PASSWORD_POLICY_VIOLATION` và ticket còn hiệu lực đến hết hạn | FR-AUTH-029 · AC-AUTH-034 · ADR-0091 mục 3 | TC-AUTH-096 |
| F012-AC-35 | Module identity PHẢI KHÔNG gửi email `execute-actions-email` `CONFIGURE_TOTP`, PHẢI KHÔNG trả `configure_otp`/`otp_setup_email_sent`, và PHẢI KHÔNG tự sinh, lưu hay kiểm secret OTP (bỏ luồng đăng ký OTP theo ADR-0094) | FR-AUTH-029 · AC-AUTH-034 · ADR-0091 mục 3 · ADR-0094 · AGENTS R30 | TC-AUTH-096 |
| F012-AC-36 | KHI `forgotPortalPassword` nhận `identifier`, module identity PHẢI luôn trả `202` trong thời gian không phụ thuộc tài khoản có tồn tại; chỉ khi `identifier` khớp tài khoản `staff` đang hoạt động, module identity PHẢI tạo token ngẫu nhiên (≥ 32 byte), lưu băm SHA-256 với `expires_at` = thời điểm tạo + `portal_password_reset_ttl`, soft-delete token chưa dùng trước đó của user, và gửi email (bất đồng bộ) chứa link `https://portal.<domain>/reset-password?token=…`; tài khoản học sinh PHẢI không bao giờ nhận email này | FR-AUTH-029 · AC-AUTH-035 · ADR-0091 mục 3 | TC-AUTH-097 |
| F012-AC-37 | KHI `resetPortalPassword` nhận token khớp băm, chưa dùng, chưa hết hạn, module identity PHẢI đánh dấu `used_at` nguyên tử (một lần), đặt mật khẩu qua Keycloak Admin API, thu hồi mọi phiên portal của user và audit; NẾU token sai, đã dùng hoặc hết hạn, THÌ PHẢI trả `401 PASSWORD_RESET_TOKEN_INVALID`; vi phạm policy PHẢI trả `422 VALIDATION_FAILED` (`PASSWORD_POLICY_VIOLATION`) | FR-AUTH-029 · AC-AUTH-035 · ADR-0091 mục 3 | TC-AUTH-097 |

---

### User Story 9 — Chặn action đang tắt ở backend (`featuregate`) (Priority: P2)

Khi System Admin tắt một menu hoặc action (F015), mọi request của Admin Portal tới operation thuộc action đó bị chặn ở backend, không chỉ bị ẩn trên giao diện.

**Why this priority**: ADR-0090 mục 3 — tắt menu là tắt thật ở API; cần trước khi các workspace portal (F052+) dùng catalog.

**Independent Test**: Tắt `menu.school_lms.assignments` cho `th-demo-a` → `ad.truong.a` gọi `listAssignments` → `403 FEATURE_DISABLED` (`feature_key`); token game gọi API học sinh không bị ảnh hưởng; `ad.truong.b` (tenant khác) vẫn gọi được.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F012-AC-40 | KHI request có token `azp = eduworld-portal` khớp operation có `x-portal-action` (bảng route → action sinh từ OpenAPI bằng `make gen`), middleware `featuregate` của `core-api` PHẢI chạy sau xác thực, `tenant`, `portal_staff` và trước handler (`authz.Require`), tra trạng thái hiệu lực của action và menu cha theo tenant của phiên (System Admin không `X-Tenant-Id` = mặc định platform), và NẾU action hoặc menu cha tắt THÌ PHẢI trả `403 FEATURE_DISABLED` với extension `feature_key`; `featuregate` PHẢI KHÔNG áp cho token khác `eduworld-portal`, operation `x-portal-shell` hay action `can_disable = false`, và PHẢI KHÔNG thay authorization | FR-CMS-018 · ADR-0090 mục 3 | TC-CMS-MENU-04 |
| F012-AC-41 | `featuregate` PHẢI đọc trạng thái qua port `featuregate.Source` (`src/backend/pkg/httpx`, hiện thực ở module identity — F015), cache theo `feature_toggle_cache_ttl` (không vượt `principal_cache_ttl`) và làm mới khi nhận `platform_settings.changed` scope `feature_flag`; NẾU không đọc được trạng thái (cache trống và database identity không sẵn sàng), THÌ PHẢI trả `503 UNAVAILABLE` cho operation có thể tắt (không fail-open) | FR-CMS-018 · ADR-0090 mục 3 · NFR-AVAIL-005 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU `portalLogin` gửi kèm `X-Tenant-Id`, THÌ bỏ qua header (route miễn trừ); tenant được chọn sau đăng nhập.
- NẾU hai tab cùng gọi `refreshPortalToken` gần như đồng thời, THÌ lần thứ hai dùng refresh token đã xoay vòng nhận `401 REFRESH_TOKEN_INVALID`; portal làm single-flight trong một tab và tab còn lại đọc lại cookie CSRF mới rồi thử lại một lần (F051).
- NẾU người dùng có `setup_ticket` nhưng quản trị khóa tài khoản trước khi dùng, THÌ `setupPortalPassword` trả `401 SETUP_TICKET_INVALID` (ticket bị hủy khi `user.status_changed`).
- NẾU người dùng yêu cầu `forgotPortalPassword` nhiều lần, THÌ chỉ token mới nhất còn hiệu lực (token cũ soft-delete) và giới hạn `portal_login_rate` áp dụng.
- NẾU menu bị tắt trong lúc người dùng đang ở màn đó, THÌ request kế tiếp nhận `403 FEATURE_DISABLED`; menu biến mất sau lần tải `getMyPermissions` kế tiếp (F051).
- NẾU role tùy biến bị soft-delete, THÌ mọi UserRole của nó không còn được tính vào Principal.
- NẾU token có realm role mà DB không có UserRole tương ứng, THÌ bỏ qua role trong token (DB thắng).
- NẾU UserRole hết hạn (`valid_to` qua), THÌ job/consumer phát `user_role.changed` `op = expired` và quyền mất.
- NẾU System Admin ghi dữ liệu tenant mà không gửi `X-Tenant-Id` đích, THÌ `400 TENANT_HEADER_REQUIRED` (TC-CMS-TEN-03).
- NẾU Content Admin (platform, không phải System Admin) đọc dữ liệu tenant, THÌ `403 PERMISSION_DENIED` (TC-CMS-TEN-04).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-016 | Permission → Role → UserRole, kiểm tra ở mọi use case, bất biến phân vùng, event | [ADMIN_PORTAL §2](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUTH-019 | Một tenant mỗi request; portal đổi tenant bằng header `X-Tenant-Id` mới + membership (không token mới — ADR-0091); System Admin `tenant_ids` | [MULTITENANCY §8](../../docs/06-architecture/MULTITENANCY.md) |
| FR-AUTH-029 | Đăng nhập portal bằng form riêng qua `/auth/portal/*` (username/email + mật khẩu một request, lỗi chung, cookie refresh + CSRF, thiết lập lần đầu, quên/đặt lại mật khẩu); chỉ nhân sự (`PORTAL_ACCESS_STAFF_ONLY`); không MFA (ADR-0094) | [KEYCLOAK_SSO §3.3](../../docs/09-identity/KEYCLOAK_SSO.md) · [SECURITY_ARCHITECTURE](../../docs/10-security/SECURITY_ARCHITECTURE.md) |
| FR-AUTH-030 | Không chọn tenant trước; `/me/tenants`; chọn/đổi tenant sau đăng nhập bằng `X-Tenant-Id` + membership | [ADMIN_PORTAL §1.3b](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUTH-032 | Quản lý Role/UserRole tenant, trường, platform; nhật ký phân quyền | [ADMIN_PORTAL §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-001 | Danh mục role tenant/platform | [ADMIN_PORTAL §2.3](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-012 | `/me/permissions`, catalog trong code, cấm permission platform trong role tenant | [ADMIN_PORTAL §2.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-018 | Phần backend chặn: middleware `featuregate` + `disabled_menus`/`disabled_actions` trong `getMyPermissions` (dữ liệu và API bật/tắt ở F015, UI ở F051) | [API_DESIGN §11](../../docs/07-backend/API_DESIGN.md) · [ADMIN_PORTAL](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-LMS-005 | Scope `school`, `subject`; không scope lớp | [SECURITY_ARCHITECTURE §5](../../docs/10-security/SECURITY_ARCHITECTURE.md) |
| FR-ORG-008 | Một tenant mỗi phiên; liên tenant chỉ System Admin + audit (phần API) | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Permission**: quyền nguyên tử trong code, `scope_kind` `tenant`/`platform`, nhóm, tên song ngữ.
- **Role**: tập permission; hệ thống (chỉ đọc) hoặc tùy biến của tenant; `role_scope_kind`.
- **UserRole**: gán role cho user theo scope và thời hạn, nguồn `manual`, người gán.
- **Principal**: ngữ cảnh quyền của user trong một tenant (hoặc platform) — permission, scope, version.
- **Portal session bootstrap**: kết quả đăng nhập portal (tenant của phiên hoặc phạm vi platform).
- **Phiên portal trên trình duyệt**: access token (bộ nhớ SPA), refresh token (cookie HttpOnly), giá trị CSRF (cookie đọc được + header).
- **Setup ticket**: vé dùng một lần cho bước thiết lập tài khoản lần đầu (Valkey, không lưu bền).
- **PortalPasswordResetToken**: token đặt lại mật khẩu (chỉ lưu băm), hạn dùng, dùng một lần.
- **Trạng thái bật/tắt hiệu lực**: tập menu/action đang tắt theo tenant hoặc platform (dữ liệu F015), dùng cho `featuregate` và `getMyPermissions`.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `principal_cache`, `principal_cache_ttl` | Cache Principal L1 + Valkey và độ trễ thu hồi quyền |
| `portal_session_sso` | Idle/max phiên portal, `auth_time` tối đa cho route platform, vòng đời cookie refresh |
| `portal_setup_ticket_ttl` | Hạn `setup_ticket` |
| `portal_password_reset_ttl` | Hạn token đặt lại mật khẩu |
| `portal_login_rate` | Giới hạn `/auth/portal/*` theo username, `device_id`, toàn cục (không theo IP) |
| `brute_force_lockout` | Khóa tạm của Keycloak (không đổi, lỗi vẫn là `INVALID_CREDENTIALS`) |
| `feature_toggle_cache_ttl` | Độ trễ áp trạng thái bật/tắt menu/action vào `featuregate` và `getMyPermissions` |
| `tenant_header_required` | Route miễn `X-Tenant-Id` (`/auth/session`, `/me/tenants`, `/auth/portal/*`) |
| `identifier_uniqueness` | Trùng username/email khi tạo nhân sự |
| `api_rate_default` | Rate limit `auth/session` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Đường đăng nhập portal (`portalLogin`, `listMyTenants`, `getMyPermissions`, `getMe`) không gọi đồng bộ module khác (chỉ Keycloak); p95 theo `login_slo`; chỉ Keycloak xác thực mật khẩu | AGENTS R30 · ADR-0045 · ADR-0091 |
| Mật khẩu/refresh token không vào log, trace, audit | AGENTS R14 · ADR-0091 mục 5 |
| Route platform chỉ gateway nội bộ, `azp = eduworld-portal` + role platform (không MFA) | AGENTS R20 · ADR-0027 · ADR-0094 |
| Authorization ở backend; ẩn menu không phải lớp bảo mật | AGENTS R7, R20 |
| Test isolation + authorization cho 100% endpoint mới | NFR-SEC-006 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Thu hồi UserRole có hiệu lực ở mọi module trong ≤ 60 giây (đo bằng test tích hợp).
- **SC-002**: 0 role tenant chứa permission platform ở mọi môi trường (kiểm khi khởi động và bằng trigger).
- **SC-003**: Nhân sự thuộc 2 tenant đổi tenant không phải nhập lại mật khẩu trong 100% lượt thử E2E.
- **SC-004**: 100% route portal trả `403 PORTAL_ACCESS_STAFF_ONLY` với token học sinh trong test sinh từ OpenAPI.
- **SC-005**: 0 lần mật khẩu mẫu xuất hiện trong log/trace sau bộ test TC-AUTH-100; 100% biến thể lỗi đăng nhập trả cùng `401 INVALID_CREDENTIALS` (TC-AUTH-094).
- **SC-006**: 100% operation có `x-portal-action` trả `403 FEATURE_DISABLED` khi action/menu cha tắt, trong ≤ `feature_toggle_cache_ttl` sau khi đổi (test sinh từ OpenAPI).

## Ngoài phạm vi

- Màn đăng nhập portal (`/login`, thiết lập, quên/đặt lại mật khẩu), bộ chọn tenant, menu UI sinh từ catalog → F051; màn người dùng/role → F052, F057.
- Cấu hình Keycloak realm, client `eduworld-portal` confidential, direct grant flow chặn học sinh (không OTP — ADR-0094), route APISIX `/auth/portal/*` → F004.
- Dữ liệu bật/tắt (`platform.feature_flags`, `platform.portal_menu_catalog`), API System Admin, hiện thực `featuregate.Source` → F015; danh mục `specs/portal/menu-catalog.yaml` và `x-portal-action` → contract chung (ADR-0090).
- Gửi email (adapter SMTP/outbox thông báo) dùng hạ tầng thông báo chung; nội dung email theo file dịch.
- Đồng bộ membership Organization và realm role theo `user_role.changed` → F010 (consumer).
- Đăng nhập game → F013; audit sink và truy vấn audit → F015.
- Đọc liên tenant đầy đủ (Thùng rác, gán/clone) → F045.

## Assumptions

- Role tùy biến của tenant [Confirmed] (ADR-0066); triển khai nhưng có thể tắt bằng feature flag nếu chưa duyệt.
- F002 cung cấp middleware `auth`, `principal`, `tenant`, `portal_staff`, `game_player` trong `src/backend/pkg/httpx`; F012 cung cấp dữ liệu (Principal) và test.
- `auth_time` tối đa 8 giờ cho route platform [Confirmed] theo KEYCLOAK_SSO §3.3.
- Lỗi Keycloak khi còn required action theo giả định của ADR-0091; xác nhận bằng spike F004 T039 [Confirmed]. Claim `amr`/`acr` không còn dùng (ADR-0094).
- APISIX phục vụ `/api/v1/*` dưới origin của portal để cookie là first-party [Confirmed] (F004-AC-28).
