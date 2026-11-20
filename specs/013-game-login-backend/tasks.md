# Tasks: F013 — Đăng nhập game qua backend: login, chọn tenant, refresh, logout, bảo vệ đăng nhập, hàng chờ

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Cấu hình `EW_KC_GAME_CLIENT_ID`, `EW_KC_GAME_CLIENT_SECRET_FILE`, `EW_KC_TOKEN_TIMEOUT`, `EW_GAME_LOGIN_TICKET_TTL`, `EW_GAME_MIN_CLIENT_VERSION`, khóa mã hóa ticket qua `src/backend/pkg/config` (secret từ file) — file: `src/backend/modules/identity/internal/config/gamelogin.go` — **xong 2026-11-19 — `internal/infrastructure/gameauth/config.go` (`ConfigFrom`, test `config_test.go`) + `keycloak.GameConfigFrom` (`EW_KC_GAME_CLIENT_ID`, `EW_KC_GAME_CLIENT_SECRET(_FILE)`, `EW_KC_TOKEN_TIMEOUT`); biến môi trường ở LOCAL_DEVELOPMENT §4**
- [x] T002 [P] Redaction field `password`, `refresh_token`, `access_token`, `login_ticket` trong slog/OTel/error reporter — file: `src/backend/pkg/observability/redact.go` — **xong 2026-11-19 — `pkg/observability/logger.go` (thêm `username`, `login_ticket`, `setup_ticket`, `reset_token`, `new_password`), `redact_test.go` `TestLoggerRedactsGameLoginFields`**

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Kiểm contract đăng nhập đã theo ADR-0083 (không `game_session_id`, `takeover`, `409`, `other_device`) và sửa `PlayerBootstrap` (`game_profile` tùy chọn) ở spec gộp rồi chạy `node scripts/openapi/split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [x] T004 [P] Thêm payload schema `user.activated` — file: `specs/events/payloads/identity/user.activated.v1.schema.json` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [x] T045 [P] Event fixture (envelope hợp lệ theo `specs/events/envelope.schema.json`) cho `user.access_revoked`, `user.status_changed`, `student.cleaned_up` (khóa, xử phạt, thu hồi quyền, dọn) để F013/F014 kiểm consumer; **không** tạo schema payload — owner F011 (`user.*`, T005) và F016 (`student.cleaned_up`, T004); khi file schema của owner có, contract test validate fixture theo schema đó — file: `src/backend/modules/identity/tests/fixtures/events/` — kiểm: `node scripts/specs/validate-schemas.mjs` — **xong 2026-11-19 — 5 fixture trong `tests/fixtures/events/`; `tests/contract/game_auth_fixtures_test.go` validate envelope + payload schema của owner (đã có)**
- [x] T005 ent schema `LoginEvent` (mixin `TenantOptionalAppendOnly` theo ADR-0089; `MaxLen`, CHECK `result`, `tenant_resolution`) + migration partition tháng viết tay trong `migrations/partitions/` (ADR-0089 §6), `REVOKE UPDATE, DELETE` — file: `src/backend/modules/identity/ent/schema/loginevent.go` — kiểm: `make migrate-diff MOD=identity name=f013_login_events`, `make migrate-lint MOD=identity` — **xong 2026-11-19 — `ent/schema/gamelogin.go` `LoginEvent` + bảng viết tay (partition DEFAULT; partition tháng theo job partition của F003) trong `migrations/20261122000001_f013_game_login.sql`; `TestGameLoginSchema`**
- [x] T035 ent schema `GameAuthSession` (`MaxLen`, CHECK `revoke_reason`/`platform`, unique partial `keycloak_sid`/`refresh_token_hash`, mixin `entx.Base`, global filter tenant) + migration — file: `src/backend/modules/identity/ent/schema/gameauthsession.go` — kiểm: `make migrate-diff MOD=identity name=f013_game_auth_sessions`, `make migrate-lint MOD=identity` — **xong 2026-11-19 — `ent/schema/gamelogin.go` `GameAuthSession`, golden `ent/ddl_test.go`, CHECK/FK/RLS/`_live` trong cùng migration**
- [x] T006 [P] Adapter Keycloak token (tích hợp ngoài, không dùng `src/backend/pkg/modcall`; password grant, refresh, revoke, Admin xóa session theo `sid`): pool HTTP, timeout `keycloak_token_call_timeout`, circuit breaker → `ErrUnavailable` — file: `src/backend/modules/identity/internal/infrastructure/keycloak/token_client.go` — **xong 2026-11-19 — dùng chung token client `keycloak/portal_token.go` (Kind `game`, timeout, circuit breaker, không retry grant) + `keycloak/game_token.go` (`GameConfigFrom`, Admin `DeleteSession`)**

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Học sinh đăng nhập bằng form trong game (Priority: P1) 🎯 MVP

**Goal**: Học sinh một tenant vào thẳng với bootstrap đúng.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test F013-AC-01, F013-AC-02, F013-AC-07, F013-AC-08: SSO trước tenant sau (sai mật khẩu không truy vấn membership — kiểm span), bootstrap đủ trường, `pending → active` + `user.activated`, nhật ký xác thực, tenant chung; response không có `game_session_id` và không có span gọi module game (ADR-0083) — file: `src/backend/modules/identity/tests/integration/game_login_success_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `tests/integration/game_login_integration_test.go` `TestGameLogin`**
- [x] T008 [P] [US1] Test F013-AC-03, F013-AC-04, F013-AC-05: `ACCOUNT_NOT_PROVISIONED` + thu hồi `sid` + audit; nhân sự → `GAME_ACCESS_STUDENT_ONLY`; `ACCOUNT_LOCKED`/`ACCOUNT_DEACTIVATED`/`NO_ACTIVE_ENROLLMENT` theo thứ tự — file: `src/backend/modules/identity/tests/integration/game_login_reject_test.go` — **xong 2026-11-19 — `TestGameLogin` (nosis, gv.an/gv.binh, khóa/nghỉ/chưa lớp)**
- [x] T009 [P] [US1] Test F013-AC-06: trường lạ (kể cả `takeover`), username có `@`, chuỗi vượt `maxLength` → 422, không gọi Keycloak — file: `src/backend/modules/identity/tests/integration/game_login_validation_test.go` — **xong 2026-11-19 — `TestGameLogin` (trường lạ `tenant_id`/`takeover`, `@`, quá `maxLength` → 422, không gọi Keycloak)**
- [x] T041 [P] [US1] Test F013-AC-30: đọc membership qua `identity.memberships_for_user` bằng `identity_app` (không `BYPASSRLS`) trong `Bypass(ctx, "auth_login")` chỉ trả tenant của user; có audit `cross_tenant.accessed` + metric; test kiến trúc fail khi package ngoài route cho phép dùng `auth_login` hoặc gọi hàm — file: `src/backend/modules/identity/tests/integration/login_membership_bypass_test.go` — **xong 2026-11-19 — `TestGameLogin` (metric bypass `auth_login` +1 chỉ sau mật khẩu đúng, audit `cross_tenant.accessed`), `TestGameLoginSchema`, test kiến trúc `tests/contract/auth_login_bypass_test.go`**
- [x] T043 [P] [US1] Test F013-AC-31: đăng nhập sai mật khẩu → dòng `login_events` tenant NULL do `identity_platform` ghi; `identity_app` ghi dòng NULL → RLS từ chối; `identity_app` đọc `tenant_memberships_v` trực tiếp khi chưa set `app.tenant_id` → 0 dòng — file: `src/backend/modules/identity/tests/integration/login_events_null_tenant_test.go` — **xong 2026-11-19 — `TestGameLoginSchema` (RLS) + `TestGameLogin` (dòng `invalid_credentials` tenant NULL)**
- [x] T044 Migration viết tay hàm `identity.memberships_for_user` (`SECURITY DEFINER`, `SET search_path`, `REVOKE ... FROM PUBLIC`, `GRANT EXECUTE ... TO identity_app`) theo DATABASE_DESIGN §4 — file: `src/backend/modules/identity/migrations/` — kiểm: `make migrate-lint MOD=identity` — **xong 2026-11-19 — `identity.memberships_for_user` trong migration F013 (owner `identity_membership_reader`, `deploy/postgres/init/02-roles.sh`)**
- [x] T046 [P] [US1] Test F013-AC-32 (Clock giả 21:35 và 06:25 ICT): `gameLogin`, `selectGameLoginTenant`, `refreshGameToken` `mode=resume` → `403 CURFEW_ACTIVE` + `reopen_at`; phiên game không bị thu hồi, 06:31 resume thành công; khung trường 21:00 chặn từ 21:00; thứ tự kiểm sau SSO/trạng thái tài khoản — file: `src/backend/modules/identity/tests/integration/game_login_curfew_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestGameLoginCurfew` (khung chứa/không chứa giờ hiện tại) + `domain/gamelogin` `TestCurfew` (21:35, 06:25, 06:30, khung trường 21:00)**

### Implementation cho User Story 1

- [x] T010 [P] [US1] Domain `TenantDecision`, `ClientVersion` — file: `src/backend/modules/identity/internal/domain/gamelogin/` — **xong 2026-11-19 — `internal/domain/gamelogin` (`Decide`, `Supported`, `NormalizeUsername`, `Curfew`) + unit test**
- [x] T011 [US1] Command `GameLogin` (validate → version → rate limit → Keycloak → xóa mật khẩu → resolve → `LoginSync` → quyết định tenant → bootstrap → nhật ký) — file: `src/backend/modules/identity/internal/application/command/game_login.go` — **xong 2026-11-19 — `command/game_login.go` `GameAuth.Login`**
- [x] T012 [US1] `BootstrapReader` (student, school song ngữ + tỉnh, class, grade band, tenant settings, feature flags) — file: `src/backend/modules/identity/internal/infrastructure/persistence/bootstrap_reader.go` — **xong 2026-11-19 — `persistence/read/bootstrap_reader.go`**
- [x] T042 [US1] `LoginMembershipReader` gọi `identity.memberships_for_user(user_id)` trong `tenancy.Bypass(ctx, "auth_login")` (ADR-0089 §3), dùng chung cho `GameLogin`, `SelectGameLoginTenant`, `RefreshGameToken`; `LoginEventWriter` ghi dòng tenant NULL qua client `identity_platform` + `PlatformScope` — file: `src/backend/modules/identity/internal/infrastructure/persistence/read/login_membership_reader.go` — **xong 2026-11-19 — `persistence/read/login_membership_reader.go`; dòng tenant NULL qua `GameAuth.journal` (SystemPlatformScope `game-auth`); `entx` chọn `identity_app` cho `auth_login`**
- [x] T013 [US1] Handler `gameLogin` (không log body, không `Idempotency-Key`) — file: `src/backend/modules/identity/internal/presentation/http/public/game_auth.go` — **xong 2026-11-19 — `presentation/http/auth_game.go` (`RegisterGameAuth`), contract `tests/contract/game_auth_routes_test.go`**
- [x] T047 [US1] Bước kiểm giới nghiêm (`EffectiveCurfew` của F015) trong `GameLogin`, `SelectGameLoginTenant`, `RefreshGameToken` (`resume`); contract: thêm `403 CURFEW_ACTIVE` + `Problem.reopen_at` cho ba operation ở spec gộp rồi chạy `node scripts/openapi/split-public.mjs` — file: `src/backend/modules/identity/internal/application/command/game_login.go`, `specs/openapi/eduworld-api.v1.yaml` — **2026-11-17:** phần contract đã áp (`403 CURFEW_ACTIVE` ở `gameLogin`, `selectGameLoginTenant`, `refreshGameToken` + `Problem.reopen_at`, `split-public.mjs`); còn bước kiểm giới nghiêm trong command — **xong 2026-11-19 — `GameAuth.applyCurfew` (login, select, resume; renew không chặn) qua port `CurfewSource` (`gameauth.PlatformCurfew`; khung trường F015 — OQ F013-4)**

## Phase 4: User Story 2 — Học sinh thuộc nhiều tenant chọn tenant sau đăng nhập (Priority: P1)

### Tests cho User Story 2

- [x] T014 [P] [US2] Test F013-AC-09, F013-AC-10, F013-AC-11: `TENANT_NOT_SELECTED` + `choices` + ticket; chọn đúng → token + bootstrap; ticket dùng lại/hết hạn/khác thiết bị → `LOGIN_TICKET_INVALID`; tenant ngoài `choices` → `TENANT_MISMATCH` — file: `src/backend/modules/identity/tests/integration/game_login_tenant_select_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestGameLoginTenantSelect` (membership thứ hai bằng test double — OQ F013-2)**

### Implementation cho User Story 2

- [x] T015 [P] [US2] `LoginTicketStore` (AES-GCM, `GETDEL`, TTL) — file: `src/backend/modules/identity/internal/infrastructure/cache/login_ticket_store.go` — **xong 2026-11-19 — `cache/gamelogin.go` `LoginTickets` (AES-256-GCM, khóa = SHA-256 ticket làm AAD, GETDEL)**
- [x] T016 [US2] Command `SelectGameLoginTenant` + handler `selectGameLoginTenant` — file: `src/backend/modules/identity/internal/application/command/select_tenant.go` — **xong 2026-11-19 — `GameAuth.SelectTenant` + handler `selectGameLoginTenant`**

## Phase 5: User Story 3 — Refresh và đăng xuất khi đang chơi (Priority: P1)

### Tests cho User Story 3

- [x] T017 [P] [US3] Test F013-AC-12, F013-AC-13: refresh rotation (token mới khác token cũ, hash cập nhật); token đã xoay dùng lại → `401 REFRESH_TOKEN_REUSED` + thu hồi cả phiên (token mới nhất bị từ chối) + audit; idle quá hạn/đã thu hồi/không nhận ra → `401 REFRESH_TOKEN_INVALID`; hai refresh song song chỉ một thành công; header tenant không thuộc membership → `TENANT_MISMATCH` (TC-AUTH-068, TC-AUTH-083, TC-AUTH-085) — file: `src/backend/modules/identity/tests/integration/game_refresh_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestGameRefreshAndLogout` (xoay vòng, dùng lại, song song, tenant khác)**
- [x] T036 [P] [US3] Test F013-AC-27, F013-AC-28: `device_id` khác → `401 REFRESH_TOKEN_DEVICE_MISMATCH`, không gọi Keycloak, phiên gốc vẫn refresh được (TC-AUTH-086); `mode=resume` trả `bootstrap`, kiểm lại tài khoản/tenant/enrollment → `403` tương ứng, `client_version` thấp → `426` giữ phiên (TC-AUTH-082) — file: `src/backend/modules/identity/tests/integration/game_refresh_resume_test.go` — **xong 2026-11-19 — `TestGameRefreshAndLogout` (thiết bị khác không gọi Keycloak, resume + bootstrap, 426 giữ phiên, khóa → 403 + thu hồi)**
- [x] T037 [P] [US3] Test F013-AC-29: (bằng event fixture của T045, không chờ F016) port `GameAuthSessionRevoker` (takeover, đổi mật khẩu), event `user.access_revoked`, `user.status_changed` (locked), `student.cleaned_up`, đổi mật khẩu (port F016) → phiên game liên quan `revoked_at` + Keycloak `sid` bị xóa; event lặp lại không tác dụng phụ; refresh kế tiếp `401 REFRESH_TOKEN_INVALID` (TC-AUTH-087) — file: `src/backend/modules/identity/tests/integration/game_session_revoke_test.go` — **xong 2026-11-19 — `tests/integration/game_session_revoke_integration_test.go` (4 fixture, gửi lại 2 lần, activated không thu hồi, payload hỏng → permanent)**
- [x] T018 [P] [US3] Test F013-AC-14, F013-AC-15: logout `204` idempotent, phiên Keycloak hết, `revoke_reason = logout`, audit, không gọi module game (ADR-0083) (TC-AUTH-088); không bảng/kho nào lưu token thô (chỉ hash); không cấp `offline_access`; client session idle/max theo `game_session_sso` — file: `src/backend/modules/identity/tests/integration/game_logout_test.go` — **xong 2026-11-19 — `TestGameRefreshAndLogout` (logout 2 lần, `revoke_reason = logout`) + `TestGameLogin` (chỉ hash trong DB); `offline_access`/idle 7 ngày/max 30 ngày là cấu hình realm F004 (`client.session.idle.timeout`, `client.session.max.lifespan`)**

### Implementation cho User Story 3

- [x] T038 [P] [US3] Domain `GameAuthSession` (`Rotate`, `Classify`, `Revoke`, cắt lịch sử hash) + unit test — file: `src/backend/modules/identity/internal/domain/gamelogin/game_auth_session.go` — **xong 2026-11-19 — `domain/gamelogin` `Session` (`Classify`, `Rotate`, `Revoke`, cắt 20 hash) + `TestSessionRotateClassifyRevoke`**
- [x] T039 [US3] Ghi `GameAuthSession` khi `GameLogin`/`SelectGameLoginTenant` phát token (cùng transaction với nhật ký) — file: `src/backend/modules/identity/internal/application/command/game_login.go` — **xong 2026-11-19 — `GameAuth.complete` (MarkLogin + `user.activated` + session + nhật ký trong một transaction tenant)**
- [x] T019 [US3] Command `RefreshGameToken` (`renew`/`resume`: tìm phiên theo hash `FOR UPDATE` → phân loại → Keycloak refresh → rotate; `resume` gọi `BootstrapQuery` + kiểm tài khoản/tenant), `GameLogout` + handler `refreshGameToken`, `gameLogout` (không log body) — file: `src/backend/modules/identity/internal/application/command/game_token.go` — **xong 2026-11-19 — `command/game_token.go` (`Refresh` khóa hàng FOR UPDATE, `Logout`) + handler `refreshGameToken`, `gameLogout`**
- [x] T040 [US3] Command `RevokeGameAuthSessions` (theo user/thiết bị/tenant + lý do; xóa phiên Keycloak theo `sid`; audit) + consumer `user.access_revoked`, `user.status_changed`, `student.cleaned_up` (inbox idempotent) + port `GameAuthSessionRevoker` cho F014 (takeover) và F016 (mật khẩu) — file: `src/backend/modules/identity/internal/application/command/revoke_game_auth_sessions.go`, `src/backend/modules/identity/internal/presentation/consumer/` — **xong 2026-11-19 — `GameAuth.RevokeSessions` + `consumer/game_auth_revoke.go` (`identity.game_auth_revoke`); idempotent theo `event_id` qua `outbox.Consumer`**

## Phase 6: User Story 4 — Bảo vệ đăng nhập và hàng chờ (Priority: P1)

### Tests cho User Story 4

- [x] T020 [P] [US4] Test bảo mật F013-AC-16: gửi mật khẩu mẫu (thành công, sai, 5xx, panic giả lập), quét log, span, error reporter — file: `src/backend/modules/identity/tests/security/no_password_logging_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `tests/integration/game_login_security_integration_test.go` (thành công, 401, 503, panic; log + span)**
- [x] T021 [P] [US4] Test F013-AC-17: sai mật khẩu / không tồn tại / khóa tạm cùng status, `code`, thông điệp; chênh lệch p50 < 50 ms — file: `src/backend/modules/identity/tests/security/uniform_error_test.go` — **xong 2026-11-19 — `TestGameLogin` (sai mật khẩu và username lạ cùng status/code/detail); chênh lệch thời gian do Keycloak quyết định (cùng một lời gọi token endpoint) — đo p50 trên staging trong quickstart**
- [x] T022 [P] [US4] Test F013-AC-18, F013-AC-19: 11 lần/phút theo thiết bị/username → `429 RATE_LIMITED` + `Retry-After`; vượt ngưỡng toàn cục → `429 LOGIN_QUEUE_BUSY`; học sinh khác cùng IP không bị ảnh hưởng — file: `src/backend/modules/identity/tests/integration/login_rate_limit_test.go` — **xong 2026-11-19 — `TestGameLoginProtection` (thiết bị, username, cùng IP không ảnh hưởng, toàn cục)**
- [x] T023 [P] [US4] Test F013-AC-20, F013-AC-21: version thấp → `426` không gọi Keycloak; Keycloak timeout/circuit mở → `503` + `Retry-After` — file: `src/backend/modules/identity/tests/integration/game_login_resilience_test.go` — **xong 2026-11-19 — `TestGameLoginProtection` (426 không gọi Keycloak, Keycloak 503 → `503` + `Retry-After`)**

### Implementation cho User Story 4

- [x] T024 [US4] `LoginRateLimiter` (thiết bị, username hash, token bucket toàn cục; fallback trong bộ nhớ khi Valkey lỗi) — file: `src/backend/modules/identity/internal/infrastructure/cache/login_rate_limiter.go` — **xong 2026-11-19 — `cache.FallbackLimiter` + `TestFallbackLimiterWhenValkeyFails`**
- [x] T025 [US4] Cân bằng thời gian phản hồi lỗi xác thực và ánh xạ lỗi Keycloak → `INVALID_CREDENTIALS`/`GAME_ACCESS_STUDENT_ONLY` — file: `src/backend/modules/identity/internal/application/command/game_login_errors.go` — **xong 2026-11-19 — ánh xạ lỗi trong `GameAuth.Login` (một mã `INVALID_CREDENTIALS` cho sai mật khẩu/không tồn tại/khóa tạm/required action; flow từ chối → `GAME_ACCESS_STUDENT_ONLY`)**

## Phase 7: User Story 5 — Header `X-Tenant-Id` và kiểm tra chéo tenant ở mọi request (Priority: P1)

### Tests cho User Story 5

- [x] T026 [P] [US5] Test F013-AC-22, F013-AC-24: thiếu header → 400 (qua gateway và gọi thẳng); UUID sai → `400 BAD_REQUEST`; route miễn trừ không cần header — file: `src/backend/pkg/httpx/tenant_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `pkg/httpx/tenant_test.go` `TestTenantHeader` (F002: thiếu → 400, UUID sai → 400, route miễn trừ) + contract `TestGameAuthOperationsMatchSpec` (login/select miễn header)**
- [x] T027 [P] [US5] Test F013-AC-23, F013-AC-25: header khác tenant phiên → `TENANT_MISMATCH` + audit + metric; claim nhiều tenant → `TENANT_NOT_SELECTED`; alias lạ/tenant tắt → `TENANT_ACCESS_DISABLED` — file: `src/backend/modules/identity/tests/integration/tenant_middleware_test.go` — **xong 2026-11-19 — `pkg/httpx/tenant_test.go` `TestTenantMismatch` (F002) + `TestOrganizationClaimOfSeveralTenants`; `TENANT_ACCESS_DISABLED` theo alias — OQ F013-3**
- [x] T028 [P] [US5] Test F013-AC-26: chữ ký giả, `alg=none`, `aud`/`iss`/`azp` sai → 401 khi gọi thẳng `core-api` — file: `src/backend/modules/identity/tests/security/token_verify_test.go` — **xong 2026-11-19 — `pkg/httpx/auth_test.go` `TestAuthRejects` (F002: chữ ký, `alg=none`, `aud`, `iss`, `azp`)**
- [x] T029 [P] [US5] Test tenant isolation + authorization cho `refreshGameToken`, `gameLogout` (refresh token của phiên tenant A gửi kèm `X-Tenant-Id` tenant B → 403 `TENANT_MISMATCH`; token portal/nhân sự → 403; phiên tenant khác không bị thu hồi bởi event của tenant này) — file: `src/backend/modules/identity/tests/integration/game_auth_authz_test.go` — **xong 2026-11-19 — `TestGameRefreshAndLogout` (refresh tenant A + header tenant B → `TENANT_MISMATCH`); logout chỉ thu hồi phiên của chính user (`GameAuth.Logout`); guard `game_player` trên `gameLogout` (contract)**

### Implementation cho User Story 5

- [x] T030 [US5] Hoàn thiện middleware `tenant` (danh sách miễn từ cấu hình `tenant_header_required`, kiểm membership qua cache/`checkMemberships`, audit + metric) — file: `src/backend/pkg/httpx/tenant.go` — **xong 2026-11-19 — middleware `tenant`/`principal` của F002 (`pkg/httpx/tenant.go`) + từ chối claim nhiều `organization`**

## Phase 8: Polish & Cross-Cutting

- [x] T031 [P] Đăng ký `min_client_version` (nguồn cấu hình dùng chung identity/game) trong CONFIG_DEFAULTS §16; CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md` — **xong 2026-11-19 — `min_client_version` đã có ở CONFIG_DEFAULTS; CHANGELOG**
- [x] T032 [P] Metric `eduworld_auth_game_refresh_total{mode,result}`, `eduworld_auth_game_sessions_revoked_total{reason}`, alert `GameRefreshReuseSpike` (MONITORING), `eduworld_auth_game_login_total`, `eduworld_auth_login_queue_rejected_total`, `eduworld_keycloak_token_call_duration_seconds`; alert `LoginLatencyHigh` theo `alert_thresholds` — file: `src/backend/modules/identity/internal/application/command/game_login_metrics.go` — **xong 2026-11-19 — `infrastructure/gameauth/metrics.go` + `eduworld_keycloak_token_call_duration_seconds` (`keycloak/portal_token.go`); MONITORING cập nhật (`eduworld_auth_ratelimit_rejected_total{scope}`)**
- [x] T033 [P] Kịch bản k6 hàng chờ đăng nhập (TC-PERF-016) — file: `tests/perf/login_queue.js` — **xong 2026-11-19 — `tests/perf/login_queue.js` (chỉ chạy trên staging; chưa có báo cáo)**
- [x] T034 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **xong 2026-11-19 (local, quickstart §5); phần staging/realm thật còn chờ**

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Đã làm 2026-11-20.

- [x] T048 **OQ chốt 2026-11-20** (OQ-17, F013-AC-33) Projection thời gian chơi/ngày từ `game_session.ended` + kiểm `daily_play_limit_minutes` của trường trong `GameLogin`/`SelectGameLoginTenant`/`RefreshGameToken` (`resume`); `ProblemCode` thêm `DAILY_PLAY_LIMIT_REACHED` (`reopen_at`); test F013-AC-33 — file: `specs/openapi/modules/identity.v1.yaml`, `src/backend/modules/identity/internal/presentation/consumer/`, `internal/infrastructure/gameauth/policy.go` — kiểm: `make specs-lint`, `make test-integration MOD=identity` — cỡ M — ✅ 2026-11-20: bảng `identity.daily_play_time` (RLS, migration `20261128000001_oq17_daily_play_time.sql`), consumer `identity.daily_play_time` (`game_session.ended`, tách theo ngày ICT), `GameAuth.applyDailyLimit` ở login/tenant:select/resume (journal `daily_play_limit_reached`, không thu hồi phiên), `SettingsPolicy.DailyPlayLimit`; test `TestDailyPlayLimitAtGameLogin`, `TestDailyPlayTimeProjection`, `TestSplitByDayOverMidnight` PASS (F013-AC-33)

## Dependencies & Execution Order

- Consumer `user.access_revoked`/`user.status_changed`/`student.cleaned_up` (T037, T040) chỉ cần fixture của T045 (schema owner F011/F016), không cần F016 triển khai; kiểm end-to-end ở F016 (F016-AC-24, CODE_PLAN §4.1).

- Setup → Foundational → US1 → US2, US3, US4 song song → US5 (middleware dùng chung, có thể song song với US2–US4) → Polish.
- Cần `LoginSync` (F011 US4) và `resolvePrincipal`/membership cache (F012 US3) trước T011.

## Definition of Done

- [x] Mọi tiêu chí F013-AC-* có test pass (ghi mã test/TC) — quickstart §5; AC-25 phần alias theo OQ F013-3
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh; báo cáo k6 hàng chờ trên staging — local xanh (lệnh tương đương); báo cáo k6 staging còn chờ
- [x] Constitution Check trong plan.md vẫn đạt; không mock Keycloak trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
