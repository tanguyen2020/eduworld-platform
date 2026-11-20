# Tasks: F061 — `ApiClient`, đăng nhập game trong form, chọn tenant, takeover, buộc cập nhật, đăng xuất

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Khai báo autoload `ApiClient`, `AuthService` đúng thứ tự GODOT_ARCHITECTURE §4 (sau `AppState`), xóa khóa `oidc_issuer`/`oidc_client_id` khỏi cấu hình — file: `src/game/project.godot`, `src/game/config/local.json`, `src/game/config/staging.json`, `src/game/config/production.json` — ✅ autoload `AuthService` (#5, `class_name GameAuthService`) và `ApiClient` (#6, `RestApiClient`) sau `AppState`; cấu hình môi trường thật ở `config/env/*.json` (F060) không có khóa `oidc_*` (Config chặn) — `test_boot.gd` thứ tự 9 autoload
- [x] T002 [P] Test double `FakeHttp`, `FakeClock` (nhãn TEST-ONLY) và fixture sinh từ example OpenAPI cho `gameLogin` (200, 401, 403 × 7 code, 422, 426, 429), `selectGameLoginTenant`, `refreshGameToken` (200 renew, 200 resume có `bootstrap`, `401 REFRESH_TOKEN_INVALID`/`REFRESH_TOKEN_REUSED`/`REFRESH_TOKEN_DEVICE_MISMATCH`, 403, 426), `getClientConfig`, `createGameSession` (201, 403, 409 `ActiveSessionConflict`, 409 `IDEMPOTENCY_KEY_REUSED`, 426, 503), `endGameSession`, `gameLogout` — file: `src/game/tests/fakes/fake_http.gd`, `src/game/tests/fakes/fake_clock.gd`, `src/game/tests/fixtures/auth/*.json` — ✅ `tests/fakes/fake_http.gd`, `fake_clock.gd`, `auth_harness.gd` (TEST-ONLY); fixture sinh từ OpenAPI bằng `tools/gen_fixtures.mjs` vào `tests/fixtures/contract/openapi/` (GameLoginResponse, GameTokenRefreshResponse, GameLoginProblem, ClientVersionProblem, GameSession, ActiveSessionConflict) — các mã lỗi dựng từ fixture `Problem` trong harness [Confirmed, Clarifications 2026-11-20]
- [x] T003 [P] Khóa dịch vi/en `ui.login.*` (theo S01 §11, gồm `ui.login.resuming`, `ui.login.use_other_account`, `ui.login.session_expired`, `ui.settings.logout_confirm`), `ui.error.<CODE>` cho mọi code ở spec — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — ✅ 65 khóa mới vi/en (`ui.login.*`, `ui.settings.logout*`, `ui.world.placeholder`, `ui.error.VALIDATION_FAILED`); `extract_pot --check` 143 khóa, `pseudo_locale --check` OK

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Kiểm tài liệu đã cập nhật theo ADR-0083 (D15), D16 và ADR-0086 (PR ADR-0086: GODOT_ARCHITECTURE §5.2 SecureStorage, GAME_NETWORKING §3, STUDENT_GAME_UX_CORE S01) (login → `createGameSession`, takeover ở module game, không `oidc_*`); sửa phần còn lệch: GODOT_ARCHITECTURE §4, §5, §11, §13.2; GAME_NETWORKING §4.5; STUDENT_GAME_UX_CORE S01 (bỏ nhớ lựa chọn trường); KEYCLOAK_SSO §6.2; ghi CHANGELOG — file: `docs/08-game-client/GODOT_ARCHITECTURE.md`, `docs/08-game-client/GAME_NETWORKING.md`, `docs/05-ux/STUDENT_GAME_UX_CORE.md`, `docs/09-identity/KEYCLOAK_SSO.md` — kiểm: `make docs-check` — ✅ GODOT_ARCHITECTURE §4/§5/§5.1/§5.2/§9/§11/§13.2, GAME_NETWORKING §2–§4.5, STUDENT_GAME_UX_CORE S01, KEYCLOAK_SSO §6.2 đã theo ADR-0083/D16/ADR-0086 (không `oidc_*`, login → `createGameSession`, không nhớ trường); chỉ sửa tên `SessionService` → `GameSessionService` và cây thư mục §3 theo mã thật; `check-links` OK (`build-rtm --check` đã lệch sẵn trên `origin/main` trước F061)
- [x] T005 [P] Test `Problem`/`ApiResult` parse Problem Details và trường mở rộng (F061-AC-20) — file: `src/game/tests/unit/networking/test_problem.gd` — phải FAIL trước khi code — ✅ `tests/unit/networking/test_problem.gd` 5/5
- [x] T006 [P] `Problem`, `ApiResult`, `TraceContext`, `Backoff` (theo `client_reconnect_backoff`) — file: `src/game/scripts/networking/problem.gd`, `api_result.gd`, `trace_context.gd`, `backoff.gd` — ✅ `api_result.gd`, `trace_context.gd`, `backoff.gd`, `api_request.gd`, `client_clock.gd`; Problem Details = `ApiProblem` của F060 mở rộng trường `errors`, `choices`, `login_ticket`, `other_device`, `min_client_version`, `store_url`, `reopen_at` [Confirmed]
- [x] T007 [P] `HttpTransport` + `HttpRequestPool` (tối đa 4 song song) — file: `src/game/scripts/networking/http_transport.gd`, `http_request_pool.gd` — ✅ `http_transport.gd` + `http_request_pool.gd` (≤ 4 `HTTPRequest`); chạy thật với Prism MOCK :4011 (client-config 200, login 200, sessions 201, heartbeat 204, :end 204, logout 204)
- [x] T008 [P] `DeviceIdentity` (`user://device_id`, Web localStorage) + test — file: `src/game/scripts/core/device_identity.gd`, `src/game/tests/unit/core/test_device_identity.gd` — ✅ `device_identity.gd` (+ `--device-id=` chỉ debug), `uuid.gd` (v4/v7); `test_device_identity.gd` 4/4
- [x] T009 [P] `TokenStore` (access token chỉ RAM, `clear()`), `SessionStore` (bản ghi `eduworld.game.session` qua `SecureStorage` F060: `save`/`load`/`erase`, bản ghi hỏng → xóa), `PasswordBuffer` (`wipe()`), `FakeSecureStorage` (TEST-ONLY) + test access token/mật khẩu không có API ghi đĩa — file: `src/game/scripts/authentication/token_store.gd`, `session_store.gd`, `password_buffer.gd`, `src/game/tests/fakes/fake_secure_storage.gd`, `src/game/tests/unit/authentication/test_token_store.gd` — ✅ `token_store.gd`, `session_store.gd` (`load_record`/`save`/`erase`, bản ghi hỏng → xóa), `password_buffer.gd`; dùng `FakeSecureStorage` có sẵn của F060 (`tests/fixtures/`); `test_token_store.gd` 6/6

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 3 — `ApiClient` chuẩn (Priority: P1)

**Goal**: Lớp REST dùng chung. (Làm trước US1 vì US1 dùng `ApiClient`.)

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T010 [P] [US3] Test F061-AC-16: header chuẩn; không `X-Tenant-Id` ở `getClientConfig`/`gameLogin`/`selectGameLoginTenant`; `traceparent` mới mỗi request; `Idempotency-Key` giữ khi retry (TC-TEN-011) — file: `src/game/tests/unit/networking/test_api_client_headers.gd` — phải FAIL trước khi code — ✅ `test_api_client_headers.gd` 5/5
- [x] T011 [P] [US3] Test F061-AC-17: retry chỉ GET/có key, backoff tối đa 3 lần, `429` tối đa 2 lần, POST không key không retry — file: `src/game/tests/unit/networking/test_api_client_retry.gd` — ✅ `test_api_client_retry.gd` 7/7
- [x] T012 [P] [US3] Test F061-AC-18: single-flight refresh khi 5 request gặp `401` (body có `device_id`, `mode: "renew"`), rotate refresh token ghi đè `SessionStore`, refresh `401 REFRESH_TOKEN_*` → xóa RAM + `SessionStore`, `session_expired` (TC-AUTH-002, TC-AUTH-003, TC-AUTH-068) — file: `src/game/tests/unit/authentication/test_refresh_single_flight.gd` — ✅ `test_refresh_single_flight.gd` 4/4
- [x] T013 [P] [US3] Test F061-AC-19: `400 TENANT_HEADER_REQUIRED` → telemetry `client_bug` + xóa token; `403 TENANT_MISMATCH` → về form — file: `src/game/tests/unit/networking/test_api_client_tenant_errors.gd` — ✅ `test_api_client_tenant_errors.gd` 3/3

### Implementation cho User Story 3

- [x] T014 [US3] `ApiClient` (`request()`, `post_public()`, header, retry, hook refresh qua `AuthService`, tắt log body `/auth/game/*`) — file: `src/game/scripts/networking/api_client.gd` — ✅ `api_client.gd` (`send(ApiRequest)`, `as_public()` thay `post_public()`; release chỉ HTTPS; log chỉ method/path/status/code/trace_id)
- [x] T015 [US3] `AuthService.refresh()` single-flight gọi `refreshGameToken` (`device_id`, `mode: "renew"`), ghi đè `SessionStore` trước khi trả token — file: `src/game/scripts/authentication/auth_service.gd` — ✅ `AuthService.refresh()` single-flight (`refresh_finished`), ghi đè SessionStore trong `_call_refresh` trước khi trả

**Checkpoint**: `ApiClient` dùng được cho service khác.

## Phase 4: User Story 1 — Đăng nhập bằng form trong game (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T016 [P] [US1] Test F061-AC-01 và F061-AC-27: `getClientConfig` trước form; `update_required`/SemVer < `min_client_version`/`426` → ForceUpdate; `maintenance` → kiểm lại định kỳ (TC-AUTH-066) — file: `src/game/tests/unit/core/test_version_check.gd` — phải FAIL trước khi code — ✅ `tests/unit/core/test_version_check.gd` 6/6
- [x] T017 [P] [US1] Scene test F061-AC-02: thành phần form S01, nút tắt khi ô trống, không node WebView/OS.shell_open (TC-AUTH-048, TC-TEN-014) — file: `src/game/tests/scene/auth/test_login_scene.gd` — ✅ `tests/scene/auth/test_login_scene.gd` 5/5
- [x] T018 [P] [US1] Test F061-AC-03: body `GameLoginRequest` đủ trường, username trim, ô nhập theo `maxLength` (TC-AUTH-001) — file: `src/game/tests/unit/authentication/test_auth_service_login.gd` — ✅ `test_auth_service_login.gd` 3/3
- [x] T019 [P] [US1] Test F061-AC-04, F061-AC-28: 200 → access token trong `TokenStore`, refresh token + `tenant_id` trong `SessionStore` (FakeSecureStorage), bootstrap trong `AppState`, bộ đệm mật khẩu đã wipe, rồi `createGameSession` có `X-Device-Id`/`X-Tenant-Id`/`Idempotency-Key`; `201` → `game_session_id` trong `AppState`; lỗi khác `409` → `gameLogout` + xóa RAM + `SessionStore` + về form (TC-AUTH-082, TC-AUTH-046) — file: `src/game/tests/unit/authentication/test_auth_service_success.gd` — ✅ `test_auth_service_success.gd` 4/4
- [x] T020 [P] [US1] Test F061-AC-05, F061-AC-07, F061-AC-08, F061-AC-09: map `INVALID_CREDENTIALS`, `GAME_ACCESS_STUDENT_ONLY`, `ACCOUNT_NOT_PROVISIONED`, các `403` khác sang trạng thái/khóa i18n, xóa token, không auto retry (TC-AUTH-062, TC-AUTH-064, TC-AUTH-013, TC-AUTH-032) — file: `src/game/tests/unit/authentication/test_login_form_model.gd` — ✅ `test_login_form_model.gd` 6/6
- [x] T021 [P] [US1] Test F061-AC-06: `LOGIN_QUEUE_BUSY` tự thử lại theo `Retry-After` + jitter tới khi Hủy; `RATE_LIMITED` đếm ngược, không tự gửi (TC-AUTH-044, TC-AUTH-063) — file: `src/game/tests/unit/authentication/test_login_queue.gd` — ✅ `test_login_queue.gd` 3/3
- [x] T022 [P] [US1] Test F061-AC-10: `Log`/telemetry/crash buffer không chứa mật khẩu, token, ticket sau mọi nhánh (TC-AUTH-060) — file: `src/game/tests/unit/authentication/test_no_secret_logging.gd` — ✅ `test_no_secret_logging.gd` 3/3
- [x] T023 [P] [US1] Script CI F061-AC-11: quét artifact export tìm secret, `/protocol/openid-connect/token`, `oidc_`, intent filter OAuth (TC-AUTH-010) — file: `src/game/tools/ci/scan_export_secrets.sh` — ✅ `tools/ci/scan_export_secrets.sh` (gọi `tools/scan_artifact.sh` — một nguồn mẫu, thêm `oidc_*`, `redirect_uri`, `oauth2redirect`, AppAuth, `/protocol/openid-connect/token`); `scan_artifact_test.sh` 20/20; `make godot-export-check` OK (13 mẫu)
- [x] T024 [P] [US1] Test F061-AC-12: nhớ username chỉ `last_username` trong `settings.cfg`; `shared_device` ẩn tùy chọn, không lưu (TC-AUTH-045) — file: `src/game/tests/unit/authentication/test_remember_username.gd` — ✅ `test_remember_username.gd` 3/3 (Settings thêm mục `[login]`)

### Implementation cho User Story 1

- [x] T025 [P] [US1] `VersionCheck` + scene Boot/VersionCheck/ForceUpdate/Maintenance — file: `src/game/scripts/core/version_check.gd`, `src/game/scenes/bootstrap/Boot.tscn`, `VersionCheck.tscn`, `ForceUpdate.tscn`, `Maintenance.tscn` — ✅ `version_check.gd`; Splash là màn VersionCheck (giữ smoke `splash_shown`), `ForceUpdate.tscn`, `Maintenance.tscn`; Boot không đổi
- [x] T026 [US1] `AuthService.login()` + `LoginFormModel` (map code → khóa i18n, đếm lần sai trên thiết bị) + `GameSessionService.create(takeover: bool)` gọi `createGameSession` ngay sau `200` (ADR-0083) — file: `src/game/scripts/authentication/auth_service.gd`, `src/game/scripts/authentication/login_form.gd`, `src/game/scripts/authentication/game_session_service.gd` — ✅ `AuthService.login()`, `LoginFormModel` (`login_form.gd`), `GameSessionService.create(takeover)` (`game_session_service.gd`)
- [x] T027 [US1] Scene Login, LoginWaiting, NotStudent, AccountNotProvisioned, AccountBlocked (theme sinh, không override) — file: `src/game/scenes/auth/Login.tscn`, `LoginWaiting.tscn`, `NotStudent.tscn`, `AccountNotProvisioned.tscn`, `AccountBlocked.tscn` — ✅ `Login.tscn`, `LoginWaiting.tscn`, `NotStudent.tscn`, `AccountNotProvisioned.tscn`, `AccountBlocked.tscn` (script chung `account_notice.gd`); `lint_theme_overrides` 0 override; `test_pseudo_locale.gd` gồm 14 màn F061
- [x] T028 [US1] Bộ lọc khóa nhạy cảm trong `Log` và error reporter — file: `src/game/scripts/core/logger.gd` — ✅ `Log` (F060) đã che `password`/`token`/`login_ticket`/`Authorization`; ApiClient/AuthService không bao giờ log body/header — kiểm bằng `test_no_secret_logging.gd`, không cần sửa `logger.gd`

**Checkpoint**: học sinh 1 tenant vào được game.

## Phase 5: User Story 2 — Chọn tenant sau đăng nhập (Priority: P1)

### Tests cho User Story 2

- [x] T029 [P] [US2] Test F061-AC-13: `TENANT_NOT_SELECTED` → `tenant_choice_required(choices)`; `select_tenant` gửi `login_ticket`, `tenant_id`, `device_id`, không mật khẩu (TC-TEN-014) — file: `src/game/tests/unit/authentication/test_select_tenant.gd` — phải FAIL trước khi code — ✅ `test_select_tenant.gd` 2/2
- [x] T030 [P] [US2] Test F061-AC-14: `LOGIN_TICKET_INVALID`/`TENANT_MISMATCH` → xóa ticket, về form (TC-AUTH-065) — file: `src/game/tests/unit/authentication/test_select_tenant_errors.gd` — ✅ `test_select_tenant_errors.gd` 2/2
- [x] T031 [P] [US2] Test F061-AC-15: tenant chỉ nằm trong bản ghi `SessionStore` (không có dạng rõ trong `user://` sau chọn); `TenantChoice` không chọn sẵn — file: `src/game/tests/unit/authentication/test_tenant_not_persisted.gd` — ✅ `test_tenant_not_persisted.gd` 4/4

### Implementation cho User Story 2

- [x] T032 [US2] `AuthService.select_tenant()` + scene `TenantChoice` (thẻ lớn, đọc được bằng trình đọc màn hình) — file: `src/game/scripts/authentication/auth_service.gd`, `src/game/scenes/auth/TenantChoice.tscn` — ✅ `AuthService.select_tenant()` + `TenantChoice.tscn` (thẻ toggle, không chọn sẵn, cuộn được; logo chưa tải [Confirmed])

## Phase 6: User Story 4 — Một thiết bị và takeover (Priority: P1)

### Tests cho User Story 4

- [x] T033 [P] [US4] Test F061-AC-21: `409 ACTIVE_SESSION_ON_OTHER_DEVICE` từ `createGameSession` → `takeover_required(other_device)`; "Tiếp tục" gọi lại `createGameSession` với `takeover: true` + `Idempotency-Key` mới, không có mật khẩu trong RAM/body; "Ở lại" → `gameLogout` + về form — file: `src/game/tests/unit/authentication/test_takeover.gd` — phải FAIL trước khi code — ✅ `test_takeover.gd` 3/3
- [x] T034 [P] [US4] Test F061-AC-22: heartbeat `409`/`404` → `session_replaced`, xóa token RAM + `SessionStore`, không reconnect — file: `src/game/tests/unit/authentication/test_game_session_replaced.gd` — ✅ `test_game_session_replaced.gd` 1/1 (409 và 404)
- [x] T035 [US4] Integration hai client headless (`--device-id` khác nhau) với backend compose (TC-AUTH-015, TC-AUTH-016) — file: `src/game/tests/integration/auth/test_takeover_two_devices.gd` — ✅ viết `tests/integration/auth/test_takeover_two_devices.gd` (skip khi thiếu `EDUWORLD_IT_API_BASE_URL`); **chưa chạy** với backend thật (binary chưa nối, OQ F014-7)

### Implementation cho User Story 4

- [x] T036 [US4] `AuthService.confirm_takeover()` gọi `GameSessionService.create(true)` với key mới; `decline_takeover()` gọi `gameLogout` (research R1, ADR-0083) — file: `src/game/scripts/authentication/auth_service.gd` — ✅ `confirm_takeover()` (create `takeover: true`, key mới) / `decline_takeover()` (`gameLogout`)
- [x] T037 [US4] Dialog `TakeoverConfirm`, `ReplacedNotice` (không tự đóng, nút an toàn bên trái) — file: `src/game/scenes/ui/dialogs/TakeoverConfirm.tscn`, `src/game/scenes/ui/dialogs/ReplacedNotice.tscn` — ✅ `TakeoverConfirm.tscn` (nút "Ở lại máy kia" bên trái, focus mặc định), `ReplacedNotice.tscn`

## Phase 7: User Story 5 — Đăng xuất, thoát và vòng đời phiên (Priority: P1)

### Tests cho User Story 5

- [x] T038 [P] [US5] Test F061-AC-23: nút Đăng xuất (và `WM_CLOSE_REQUEST` khi `shared_device`) → `endGameSession` trước rồi `gameLogout` (refresh token) + xóa RAM + `SessionStore`; `endGameSession` lỗi vẫn gọi `gameLogout` (TC-AUTH-004, TC-AUTH-088) — file: `src/game/tests/unit/authentication/test_logout.gd` — phải FAIL trước khi code — ✅ `test_logout.gd` 3/3
- [x] T039 [P] [US5] Test F061-AC-24: heartbeat mỗi `ws_heartbeat` có `X-Device-Id`, dừng khi logout (TC-AUTH-017) — file: `src/game/tests/unit/authentication/test_heartbeat.gd` — ✅ `test_heartbeat.gd` 2/2
- [x] T040 [P] [US5] Test F061-AC-25: idle `shared_device_idle_logout` → logout; mobile quay lại từ nền / mở lại sau khi bị OS đóng → tiếp tục bằng refresh token, không hỏi mật khẩu (TC-AUTH-047, TC-AUTH-045) — file: `src/game/tests/unit/core/test_lifecycle_watcher.gd` — ✅ `tests/unit/core/test_lifecycle_watcher.gd` 5/5
- [x] T041 [P] [US5] Test F061-AC-26: `WM_CLOSE_REQUEST` trên thiết bị cá nhân chỉ gọi `endGameSession`, giữ `SessionStore`; Web/`shared_device` khởi động luôn vào form, ô mật khẩu trống (TC-AUTH-084) — file: `src/game/tests/scene/auth/test_cold_start.gd` — ✅ `tests/scene/auth/test_cold_start.gd` 2/2

### Implementation cho User Story 5

- [x] T042 [P] [US5] `GameSessionService` bổ sung heartbeat, end (tạo session ở T026) — file: `src/game/scripts/authentication/game_session_service.gd` — ✅ heartbeat loop (`ws_heartbeat` 15 s), `beat()`, `end()`, `clear()`
- [x] T043 [US5] `LifecycleWatcher` (đóng cửa sổ theo loại thiết bị) + `AuthService.logout()` (xóa `SessionStore`) + nút Đăng xuất trong Cài đặt → Tài khoản với `LogoutConfirm` — file: `src/game/scripts/core/lifecycle_watcher.gd`, `src/game/scripts/authentication/auth_service.gd` — ✅ `lifecycle_watcher.gd` (WM_CLOSE, PAUSED/RESUMED, idle 30 phút), `AuthService.logout()`/`close_game()`, `LogoutConfirm.tscn` trên màn tạm `WorldPlaceholder` (Cài đặt → Tài khoản có khi có màn Cài đặt) [Confirmed]

## Phase 8: User Story 6 — Tự vào lại game khi phiên còn hạn (Priority: P1)

**Goal**: Resume bằng refresh token đã lưu (ADR-0086).

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [x] T048 [P] [US6] Test F061-AC-29: có bản ghi → `ResumeSplash`, `refreshGameToken` `mode: "resume"` với `device_id`, `client_version`, `X-Tenant-Id` đã lưu; `200` → ghi đè `SessionStore` trước request khác, áp `bootstrap`, `createGameSession` (409 → takeover) (TC-AUTH-082) — file: `src/game/tests/unit/authentication/test_resume.gd` — phải FAIL trước khi code — ✅ `test_resume.gd` 4/4
- [x] T049 [P] [US6] Test F061-AC-30: `401 REFRESH_TOKEN_INVALID`/`REFRESH_TOKEN_REUSED`/`REFRESH_TOKEN_DEVICE_MISMATCH` và các `403` → xóa `SessionStore` + RAM, form với `ui.login.session_expired`/`ui.error.<CODE>`, username điền sẵn (TC-AUTH-083, TC-AUTH-086) — file: `src/game/tests/unit/authentication/test_resume_failures.gd` — ✅ `test_resume_failures.gd` 3/3
- [x] T050 [P] [US6] Test F061-AC-31: lỗi mạng/5xx/429 → giữ bản ghi, thử lại theo backoff, nút "Đăng nhập bằng tài khoản khác"; `426` → ForceUpdate giữ bản ghi — file: `src/game/tests/unit/authentication/test_resume_transient.gd` — ✅ `test_resume_transient.gd` 3/3
- [x] T051 [P] [US6] Test F061-AC-32: `shared_device`/Web → `MemoryOnly`, không còn bản ghi sau thoát, đóng cửa sổ = Đăng xuất (TC-AUTH-084) — file: `src/game/tests/unit/authentication/test_shared_device_session.gd` — ✅ `test_shared_device_session.gd` 3/3
- [x] T052 [P] [US6] Test F061-AC-33: không bao giờ gửi lại refresh token đã bị thay; `REFRESH_TOKEN_REUSED` → telemetry `auth_refresh_reused` không kèm token (TC-AUTH-085) — file: `src/game/tests/unit/authentication/test_refresh_rotation.gd` — ✅ `test_refresh_rotation.gd` 3/3
- [x] T053 [US6] Integration với backend compose: đăng nhập → thoát → mở lại tiến trình → vào hub không form; realm local idle ngắn → form; School Admin đặt lại mật khẩu → resume về form không vòng lặp (F061-AC-34; TC-AUTH-082, TC-AUTH-083, TC-AUTH-087) — file: `src/game/tests/integration/auth/test_resume_flow.gd` — ✅ viết `tests/integration/auth/test_resume_flow.gd` (skip khi thiếu backend; biến thể realm idle/đặt lại mật khẩu là thủ công quickstart 29, 34); **chưa chạy** với backend thật
- [x] T056 [P] [US6] Unit test F061-AC-35 (`FakeHttp` + `FakeSecureStorage`): resume `200` → `ResumeConfirm` trước `createGameSession`; "Không phải tôi" → `gameLogout`, bản ghi bị xóa, form trống — file: `src/game/tests/unit/test_resume_confirm.gd` — phải FAIL trước khi code — ✅ `tests/unit/test_resume_confirm.gd` 3/3
- [x] T057 [P] [US6] Unit test F061-AC-36: `403 CURFEW_ACTIVE` ở login/resume/session → `CurfewNotice` với `reopen_at`, giữ `SecureStorage`; cảnh báo 10 phút trước theo giờ server; `session.session.ended` (`curfew`) → `CurfewNotice` — file: `src/game/tests/unit/test_curfew_flow.gd` — ✅ `tests/unit/test_curfew_flow.gd` 6/6

### Implementation cho User Story 6

- [x] T054 [US6] `AuthService.resume()` + phân loại kết quả (xác định/tạm thời/426) + trạng thái `Resume` trong `Boot`/`SceneRouter` (research R4) — file: `src/game/scripts/authentication/auth_service.gd` — ✅ `AuthService._resume()` + phân loại (xác định/tạm thời/426/giới nghiêm) trong `start()`
- [x] T055 [US6] Scene `ResumeSplash` ("Đang vào lớp…", "Đang kết nối…", nút "Đăng nhập bằng tài khoản khác" sau 5 s) và `InfoBanner` phiên hết trên form — file: `src/game/scenes/auth/ResumeSplash.tscn` — ✅ `ResumeSplash.tscn` ("Đang vào lớp…"/"Đang kết nối…", nút sau 5 s); banner "Phiên đăng nhập đã hết" trên `Login.tscn`
- [x] T058 [US6] Trạng thái `ResumeConfirm` (S01b) và `CurfewNotice` trong `GameStateMachine`; `CurfewWatcher` đọc khung giới nghiêm từ `bootstrap`; khóa dịch `ui.login.resume_confirm.*`, `ui.curfew.*` (vi/en) — file: `src/game/scripts/core/game_state_machine.gd`, `src/game/scenes/ui/login/resume_confirm.tscn`, `src/game/scripts/services/curfew_watcher.gd` — ✅ `scripts/core/game_state_machine.gd` (trạng thái + scene), `scenes/auth/ResumeConfirm.tscn`, `scenes/auth/CurfewNotice.tscn`, `scripts/services/curfew_watcher.gd`; khóa `ui.login.resume_confirm.*`, `ui.login.curfew*` [Confirmed: đường dẫn scene]

**Checkpoint**: thiết bị cá nhân tự vào lại khi phiên còn hạn; Web/máy dùng chung luôn hiện form.

## Phase N: Polish & Cross-Cutting

- [x] T044 [P] Integration end-to-end đăng nhập `hs.an.3a` → mọi request sau có `X-Tenant-Id`; `ad.truong.a` bị chặn (TC-AUTH-001, TC-AUTH-014) — file: `src/game/tests/integration/auth/test_login_flow.gd` — ✅ viết `tests/integration/auth/test_login_flow.gd` (skip khi thiếu backend); **chưa chạy** với backend thật
- [x] T045 [P] Lint theme override và typed check cho scene mới (TC-UI-003) — kiểm: `make godot-test` — ✅ `make godot-lint` (check_typed 136 file, lint theme 98, lint lớp 42, lint signal 72, khóa dịch, pseudo, fixture, no Admin UI) — gdlint chỉ chạy ở CI
- [x] T046 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md` — ✅ `CHANGELOG.md`
- [ ] T047 Chạy [quickstart.md](quickstart.md) trên Windows + Android với backend thật và ghi bằng chứng — **chưa làm**: backend compose chưa nối binary (OQ F014-7) và chưa có thiết bị/emulator Android (lane F133); bằng chứng local thay thế ở quickstart.md mục Bằng chứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T059 **OQ chốt 2026-11-20** (pinning) `ApiClient`/`HttpRequestPool` dùng `TLSOptions.client(trusted_chain)` với CA bundle từ cấu hình build (release, Windows/Android/iOS); test chứng chỉ ngoài bundle bị từ chối, không fallback — file: `src/game/scripts/networking/api_client.gd`, `src/game/tests/unit/networking/` — kiểm: `make godot-test` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational → US3 (`ApiClient`) → US1 → US2, US4, US5, US6 (song song sau US1) → Polish.
- US6 và T009 cần F060 US5 (`SecureStorage`); adapter iOS Keychain đến từ F133.
- Có thể chạy US1/US3 trên Prism mock (F060, nhãn MOCK) khi F013/F014 chưa Done; T035, T044, T047, T053 cần backend thật (F013 có rotation, `device_id`, `REFRESH_TOKEN_*`).

## Definition of Done

- [x] Mọi tiêu chí F061-AC-* có test pass (ghi mã test/TC) — bảng trong quickstart.md mục Bằng chứng (GdUnit4 FakeHttp/FakeClock/FakeSecureStorage)
- [ ] `make godot-test`, `make godot-export-check`, quét artifact (T023) xanh ✅; integration với backend thật pass ❌ (T035/T044/T053 skip — chờ backend nối binary)
- [x] Constitution Check trong plan.md vẫn đạt; FakeHttp/Prism mock không có trong build phát hành (R17) — `tests/*` bị loại khỏi preset, `scan_artifact_test.sh` kiểm PCK release
- [x] Tài liệu sở hữu (T004), CHANGELOG cập nhật
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
