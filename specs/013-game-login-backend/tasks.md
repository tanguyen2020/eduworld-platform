# Tasks: F013 — Đăng nhập game qua backend: login, chọn tenant, refresh, logout, bảo vệ đăng nhập, hàng chờ

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Cấu hình `EW_KC_GAME_CLIENT_ID`, `EW_KC_GAME_CLIENT_SECRET_FILE`, `EW_KC_TOKEN_TIMEOUT`, `EW_GAME_LOGIN_TICKET_TTL`, `EW_GAME_MIN_CLIENT_VERSION`, khóa mã hóa ticket qua `pkg/config` (secret từ file) — file: `modules/identity/internal/config/gamelogin.go`
- [ ] T002 [P] Redaction field `password`, `refresh_token`, `access_token`, `login_ticket` trong slog/OTel/error reporter — file: `pkg/observability/redact.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Kiểm contract đăng nhập đã theo ADR-0083 (không `game_session_id`, `takeover`, `409`, `other_device`) và sửa `PlayerBootstrap` (`game_profile` tùy chọn) ở spec gộp rồi chạy `node scripts/openapi/split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity`
- [ ] T004 [P] Thêm payload schema `user.activated` — file: `specs/events/payloads/identity/user.activated.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T045 [P] Event fixture (envelope hợp lệ theo `specs/events/envelope.schema.json`) cho `user.access_revoked`, `user.status_changed`, `student.cleaned_up` (khóa, xử phạt, thu hồi quyền, dọn) để F013/F014 kiểm consumer; **không** tạo schema payload — owner F011 (`user.*`, T005) và F016 (`student.cleaned_up`, T004); khi file schema của owner có, contract test validate fixture theo schema đó — file: `modules/identity/tests/fixtures/events/` — kiểm: `node scripts/specs/validate-schemas.mjs`
- [ ] T005 ent schema `LoginEvent` (mixin `TenantOptionalAppendOnly` theo ADR-0089; `MaxLen`, CHECK `result`, `tenant_resolution`) + migration partition tháng viết tay trong `migrations/partitions/` (ADR-0089 §6), `REVOKE UPDATE, DELETE` — file: `modules/identity/ent/schema/loginevent.go` — kiểm: `make migrate-diff MOD=identity name=f013_login_events`, `make migrate-lint MOD=identity`
- [ ] T035 ent schema `GameAuthSession` (`MaxLen`, CHECK `revoke_reason`/`platform`, unique partial `keycloak_sid`/`refresh_token_hash`, mixin `entx.Base`, global filter tenant) + migration — file: `modules/identity/ent/schema/gameauthsession.go` — kiểm: `make migrate-diff MOD=identity name=f013_game_auth_sessions`, `make migrate-lint MOD=identity`
- [ ] T006 [P] Adapter Keycloak token (tích hợp ngoài, không dùng `pkg/modcall`; password grant, refresh, revoke, Admin xóa session theo `sid`): pool HTTP, timeout `keycloak_token_call_timeout`, circuit breaker → `ErrUnavailable` — file: `modules/identity/internal/infrastructure/keycloak/token_client.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Học sinh đăng nhập bằng form trong game (Priority: P1) 🎯 MVP

**Goal**: Học sinh một tenant vào thẳng với bootstrap đúng.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Test F013-AC-01, F013-AC-02, F013-AC-07, F013-AC-08: SSO trước tenant sau (sai mật khẩu không truy vấn membership — kiểm span), bootstrap đủ trường, `pending → active` + `user.activated`, nhật ký xác thực, tenant chung; response không có `game_session_id` và không có span gọi module game (ADR-0083) — file: `modules/identity/tests/integration/game_login_success_test.go` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Test F013-AC-03, F013-AC-04, F013-AC-05: `ACCOUNT_NOT_PROVISIONED` + thu hồi `sid` + audit; nhân sự → `GAME_ACCESS_STUDENT_ONLY`; `ACCOUNT_LOCKED`/`ACCOUNT_DEACTIVATED`/`NO_ACTIVE_ENROLLMENT` theo thứ tự — file: `modules/identity/tests/integration/game_login_reject_test.go`
- [ ] T009 [P] [US1] Test F013-AC-06: trường lạ (kể cả `takeover`), username có `@`, chuỗi vượt `maxLength` → 422, không gọi Keycloak — file: `modules/identity/tests/integration/game_login_validation_test.go`
- [ ] T041 [P] [US1] Test F013-AC-30: đọc membership qua `identity.memberships_for_user` bằng `identity_app` (không `BYPASSRLS`) trong `Bypass(ctx, "auth_login")` chỉ trả tenant của user; có audit `cross_tenant.accessed` + metric; test kiến trúc fail khi package ngoài route cho phép dùng `auth_login` hoặc gọi hàm — file: `modules/identity/tests/integration/login_membership_bypass_test.go`
- [ ] T043 [P] [US1] Test F013-AC-31: đăng nhập sai mật khẩu → dòng `login_events` tenant NULL do `identity_platform` ghi; `identity_app` ghi dòng NULL → RLS từ chối; `identity_app` đọc `tenant_memberships_v` trực tiếp khi chưa set `app.tenant_id` → 0 dòng — file: `modules/identity/tests/integration/login_events_null_tenant_test.go`
- [ ] T044 Migration viết tay hàm `identity.memberships_for_user` (`SECURITY DEFINER`, `SET search_path`, `REVOKE ... FROM PUBLIC`, `GRANT EXECUTE ... TO identity_app`) theo DATABASE_DESIGN §4 — file: `modules/identity/migrations/` — kiểm: `make migrate-lint MOD=identity`
- [ ] T046 [P] [US1] Test F013-AC-32 (Clock giả 21:35 và 06:25 ICT): `gameLogin`, `selectGameLoginTenant`, `refreshGameToken` `mode=resume` → `403 CURFEW_ACTIVE` + `reopen_at`; phiên game không bị thu hồi, 06:31 resume thành công; khung trường 21:00 chặn từ 21:00; thứ tự kiểm sau SSO/trạng thái tài khoản — file: `modules/identity/tests/integration/game_login_curfew_test.go` — phải FAIL trước khi code

### Implementation cho User Story 1

- [ ] T010 [P] [US1] Domain `TenantDecision`, `ClientVersion` — file: `modules/identity/internal/domain/gamelogin/`
- [ ] T011 [US1] Command `GameLogin` (validate → version → rate limit → Keycloak → xóa mật khẩu → resolve → `LoginSync` → quyết định tenant → bootstrap → nhật ký) — file: `modules/identity/internal/application/command/game_login.go`
- [ ] T012 [US1] `BootstrapReader` (student, school song ngữ + tỉnh, class, grade band, tenant settings, feature flags) — file: `modules/identity/internal/infrastructure/persistence/bootstrap_reader.go`
- [ ] T042 [US1] `LoginMembershipReader` gọi `identity.memberships_for_user(user_id)` trong `tenancy.Bypass(ctx, "auth_login")` (ADR-0089 §3), dùng chung cho `GameLogin`, `SelectGameLoginTenant`, `RefreshGameToken`; `LoginEventWriter` ghi dòng tenant NULL qua client `identity_platform` + `PlatformScope` — file: `modules/identity/internal/infrastructure/persistence/read/login_membership_reader.go`
- [ ] T013 [US1] Handler `gameLogin` (không log body, không `Idempotency-Key`) — file: `modules/identity/internal/presentation/http/public/game_auth.go`
- [ ] T047 [US1] Bước kiểm giới nghiêm (`EffectiveCurfew` của F015) trong `GameLogin`, `SelectGameLoginTenant`, `RefreshGameToken` (`resume`); contract: thêm `403 CURFEW_ACTIVE` + `Problem.reopen_at` cho ba operation ở spec gộp rồi chạy `node scripts/openapi/split-public.mjs` — file: `modules/identity/internal/application/command/game_login.go`, `specs/openapi/eduworld-api.v1.yaml`

## Phase 4: User Story 2 — Học sinh thuộc nhiều tenant chọn tenant sau đăng nhập (Priority: P1)

### Tests cho User Story 2

- [ ] T014 [P] [US2] Test F013-AC-09, F013-AC-10, F013-AC-11: `TENANT_NOT_SELECTED` + `choices` + ticket; chọn đúng → token + bootstrap; ticket dùng lại/hết hạn/khác thiết bị → `LOGIN_TICKET_INVALID`; tenant ngoài `choices` → `TENANT_MISMATCH` — file: `modules/identity/tests/integration/game_login_tenant_select_test.go` — phải FAIL trước khi code

### Implementation cho User Story 2

- [ ] T015 [P] [US2] `LoginTicketStore` (AES-GCM, `GETDEL`, TTL) — file: `modules/identity/internal/infrastructure/cache/login_ticket_store.go`
- [ ] T016 [US2] Command `SelectGameLoginTenant` + handler `selectGameLoginTenant` — file: `modules/identity/internal/application/command/select_tenant.go`

## Phase 5: User Story 3 — Refresh và đăng xuất khi đang chơi (Priority: P1)

### Tests cho User Story 3

- [ ] T017 [P] [US3] Test F013-AC-12, F013-AC-13: refresh rotation (token mới khác token cũ, hash cập nhật); token đã xoay dùng lại → `401 REFRESH_TOKEN_REUSED` + thu hồi cả phiên (token mới nhất bị từ chối) + audit; idle quá hạn/đã thu hồi/không nhận ra → `401 REFRESH_TOKEN_INVALID`; hai refresh song song chỉ một thành công; header tenant không thuộc membership → `TENANT_MISMATCH` (TC-AUTH-068, TC-AUTH-083, TC-AUTH-085) — file: `modules/identity/tests/integration/game_refresh_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US3] Test F013-AC-27, F013-AC-28: `device_id` khác → `401 REFRESH_TOKEN_DEVICE_MISMATCH`, không gọi Keycloak, phiên gốc vẫn refresh được (TC-AUTH-086); `mode=resume` trả `bootstrap`, kiểm lại tài khoản/tenant/enrollment → `403` tương ứng, `client_version` thấp → `426` giữ phiên (TC-AUTH-082) — file: `modules/identity/tests/integration/game_refresh_resume_test.go`
- [ ] T037 [P] [US3] Test F013-AC-29: (bằng event fixture của T045, không chờ F016) port `GameAuthSessionRevoker` (takeover, đổi mật khẩu), event `user.access_revoked`, `user.status_changed` (locked), `student.cleaned_up`, đổi mật khẩu (port F016) → phiên game liên quan `revoked_at` + Keycloak `sid` bị xóa; event lặp lại không tác dụng phụ; refresh kế tiếp `401 REFRESH_TOKEN_INVALID` (TC-AUTH-087) — file: `modules/identity/tests/integration/game_session_revoke_test.go`
- [ ] T018 [P] [US3] Test F013-AC-14, F013-AC-15: logout `204` idempotent, phiên Keycloak hết, `revoke_reason = logout`, audit, không gọi module game (ADR-0083) (TC-AUTH-088); không bảng/kho nào lưu token thô (chỉ hash); không cấp `offline_access`; client session idle/max theo `game_session_sso` — file: `modules/identity/tests/integration/game_logout_test.go`

### Implementation cho User Story 3

- [ ] T038 [P] [US3] Domain `GameAuthSession` (`Rotate`, `Classify`, `Revoke`, cắt lịch sử hash) + unit test — file: `modules/identity/internal/domain/gamelogin/game_auth_session.go`
- [ ] T039 [US3] Ghi `GameAuthSession` khi `GameLogin`/`SelectGameLoginTenant` phát token (cùng transaction với nhật ký) — file: `modules/identity/internal/application/command/game_login.go`
- [ ] T019 [US3] Command `RefreshGameToken` (`renew`/`resume`: tìm phiên theo hash `FOR UPDATE` → phân loại → Keycloak refresh → rotate; `resume` gọi `BootstrapQuery` + kiểm tài khoản/tenant), `GameLogout` + handler `refreshGameToken`, `gameLogout` (không log body) — file: `modules/identity/internal/application/command/game_token.go`
- [ ] T040 [US3] Command `RevokeGameAuthSessions` (theo user/thiết bị/tenant + lý do; xóa phiên Keycloak theo `sid`; audit) + consumer `user.access_revoked`, `user.status_changed`, `student.cleaned_up` (inbox idempotent) + port `GameAuthSessionRevoker` cho F014 (takeover) và F016 (mật khẩu) — file: `modules/identity/internal/application/command/revoke_game_auth_sessions.go`, `modules/identity/internal/presentation/consumer/`

## Phase 6: User Story 4 — Bảo vệ đăng nhập và hàng chờ (Priority: P1)

### Tests cho User Story 4

- [ ] T020 [P] [US4] Test bảo mật F013-AC-16: gửi mật khẩu mẫu (thành công, sai, 5xx, panic giả lập), quét log, span, error reporter — file: `modules/identity/tests/security/no_password_logging_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US4] Test F013-AC-17: sai mật khẩu / không tồn tại / khóa tạm cùng status, `code`, thông điệp; chênh lệch p50 < 50 ms — file: `modules/identity/tests/security/uniform_error_test.go`
- [ ] T022 [P] [US4] Test F013-AC-18, F013-AC-19: 11 lần/phút theo thiết bị/username → `429 RATE_LIMITED` + `Retry-After`; vượt ngưỡng toàn cục → `429 LOGIN_QUEUE_BUSY`; học sinh khác cùng IP không bị ảnh hưởng — file: `modules/identity/tests/integration/login_rate_limit_test.go`
- [ ] T023 [P] [US4] Test F013-AC-20, F013-AC-21: version thấp → `426` không gọi Keycloak; Keycloak timeout/circuit mở → `503` + `Retry-After` — file: `modules/identity/tests/integration/game_login_resilience_test.go`

### Implementation cho User Story 4

- [ ] T024 [US4] `LoginRateLimiter` (thiết bị, username hash, token bucket toàn cục; fallback trong bộ nhớ khi Valkey lỗi) — file: `modules/identity/internal/infrastructure/cache/login_rate_limiter.go`
- [ ] T025 [US4] Cân bằng thời gian phản hồi lỗi xác thực và ánh xạ lỗi Keycloak → `INVALID_CREDENTIALS`/`GAME_ACCESS_STUDENT_ONLY` — file: `modules/identity/internal/application/command/game_login_errors.go`

## Phase 7: User Story 5 — Header `X-Tenant-Id` và kiểm tra chéo tenant ở mọi request (Priority: P1)

### Tests cho User Story 5

- [ ] T026 [P] [US5] Test F013-AC-22, F013-AC-24: thiếu header → 400 (qua gateway và gọi thẳng); UUID sai → `400 BAD_REQUEST`; route miễn trừ không cần header — file: `pkg/httpx/tenant_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US5] Test F013-AC-23, F013-AC-25: header khác tenant phiên → `TENANT_MISMATCH` + audit + metric; claim nhiều tenant → `TENANT_NOT_SELECTED`; alias lạ/tenant tắt → `TENANT_ACCESS_DISABLED` — file: `modules/identity/tests/integration/tenant_middleware_test.go`
- [ ] T028 [P] [US5] Test F013-AC-26: chữ ký giả, `alg=none`, `aud`/`iss`/`azp` sai → 401 khi gọi thẳng `core-api` — file: `modules/identity/tests/security/token_verify_test.go`
- [ ] T029 [P] [US5] Test tenant isolation + authorization cho `refreshGameToken`, `gameLogout` (refresh token của phiên tenant A gửi kèm `X-Tenant-Id` tenant B → 403 `TENANT_MISMATCH`; token portal/nhân sự → 403; phiên tenant khác không bị thu hồi bởi event của tenant này) — file: `modules/identity/tests/integration/game_auth_authz_test.go`

### Implementation cho User Story 5

- [ ] T030 [US5] Hoàn thiện middleware `tenant` (danh sách miễn từ cấu hình `tenant_header_required`, kiểm membership qua cache/`checkMemberships`, audit + metric) — file: `pkg/httpx/tenant.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T031 [P] Đăng ký `min_client_version` (nguồn cấu hình dùng chung identity/game) trong CONFIG_DEFAULTS §16; CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`
- [ ] T032 [P] Metric `eduworld_auth_game_refresh_total{mode,result}`, `eduworld_auth_game_sessions_revoked_total{reason}`, alert `GameRefreshReuseSpike` (MONITORING), `eduworld_auth_game_login_total`, `eduworld_auth_login_queue_rejected_total`, `eduworld_keycloak_token_call_duration_seconds`; alert `LoginLatencyHigh` theo `alert_thresholds` — file: `modules/identity/internal/application/command/game_login_metrics.go`
- [ ] T033 [P] Kịch bản k6 hàng chờ đăng nhập (TC-PERF-016) — file: `tests/perf/login_queue.js`
- [ ] T034 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Consumer `user.access_revoked`/`user.status_changed`/`student.cleaned_up` (T037, T040) chỉ cần fixture của T045 (schema owner F011/F016), không cần F016 triển khai; kiểm end-to-end ở F016 (F016-AC-24, CODE_PLAN §4.1).

- Setup → Foundational → US1 → US2, US3, US4 song song → US5 (middleware dùng chung, có thể song song với US2–US4) → Polish.
- Cần `LoginSync` (F011 US4) và `resolvePrincipal`/membership cache (F012 US3) trước T011.

## Definition of Done

- [ ] Mọi tiêu chí F013-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh; báo cáo k6 hàng chờ trên staging
- [ ] Constitution Check trong plan.md vẫn đạt; không mock Keycloak trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
