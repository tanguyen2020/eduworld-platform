# Implementation Plan: F061 — `ApiClient`, đăng nhập game trong form, chọn tenant, takeover, buộc cập nhật, đăng xuất

**Branch**: `061-godot-api-client-login` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Hiện thực tầng mạng REST và luồng vào game của Godot theo GODOT_ARCHITECTURE §4–§5, §9 và GAME_NETWORKING §2–§3: autoload `ApiClient` (pool `HTTPRequest`, header chuẩn, Problem Details, retry an toàn, single-flight refresh), `AuthService` (gameLogin, tenant:select, resume, refresh single-flight, logout; access token chỉ RAM, refresh token + `tenant_id` qua `SessionStore` bọc `SecureStorage` của F060 — ADR-0086), `GameSessionService` (`createGameSession` ngay sau đăng nhập + takeover, heartbeat REST, end, phát hiện bị thay), `VersionCheck` (client-config), `LifecycleWatcher` (đóng cửa sổ, nền mobile, idle máy dùng chung) và các scene S01: Boot/VersionCheck/ForceUpdate/Maintenance, ResumeSplash, Login, LoginWaiting, TenantChoice, TakeoverConfirm, ReplacedNotice, AccountNotProvisioned, NotStudent, AccountBlocked, LogoutConfirm. Theo ADR-0083 (D15): đăng nhập chỉ trả token + bootstrap, client gọi `createGameSession` riêng; takeover không giữ mật khẩu; không còn cấu hình OIDC (D16). Theo ADR-0086: khởi động có phiên đã lưu → `refreshGameToken` `mode=resume` → vào game không hiện form; thoát game trên máy cá nhân giữ phiên, nút Đăng xuất xóa phiên; Web/`shared_device` chỉ RAM.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim trong `project.godot`), static typing, `untyped_declaration = error` |
| Primary Dependencies | GdUnit4; autoload từ F060 (`Config`, `Log`, `EventBus`, `AppState`, `Settings`, `SceneRouter`) và `SecureStorage`/`SecureStorageFactory` (F060 US5) |
| Storage | RAM cho access token/bootstrap/session; `SecureStorage` khóa `eduworld.game.session` (`refresh_token`, `tenant_id`, `saved_at`) trên thiết bị cá nhân, `MemoryOnly` trên Web/`shared_device`; `user://device_id`, `user://settings.cfg` (`last_username`); `device.cfg` (`shared_device`) chỉ đọc |
| Testing | GdUnit4 unit (FakeHttp/FakeClock/FakeSecureStorage), scene test S01, integration headless với backend compose (kể cả mở lại tiến trình để kiểm resume), quét artifact |
| Target Platform | Android (M-C1); iOS, Web cùng mã (ADR-0101: không Windows; chạy editor trên máy dev Windows vẫn được) |
| Project Type | game client |
| Performance Goals | `login_slo`; form hiện ≤ 3 s từ mở app (S01 §13) |
| Constraints | AGENTS R1, R3, R4, R14, R23, R30, R32; ADR-0014, ADR-0031, ADR-0045, ADR-0063, ADR-0064 (mục 3, 6, 7), ADR-0065, ADR-0068, ADR-0083, ADR-0086 |
| Scale/Scope | Hàng chờ đăng nhập 10.000 CCU (`login_rate`, ADR-0045) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Contract đủ (identity, game) theo ADR-0083; kiểm tài liệu D15/D16 là task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public qua APISIX; tách Service ← UI (G-4) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` từ bootstrap; `tenant_id` chỉ lưu cùng refresh token trong `SecureStorage` để resume, server kiểm membership lại khi resume; không đổi tenant trong phiên |
| IV Server-authoritative | ✅ | Client không quyết định quyền/tenant/phiên; chỉ hiển thị theo `code` |
| V Xác thực, phân quyền | ✅ | Không secret/URL Keycloak trong build; nhân sự bị chặn bởi server, client hiện màn hướng dẫn; refresh token chỉ trong kho bảo mật hệ điều hành, gắn `device_id`, rotation single-flight; thu hồi/hết hạn do server quyết định (ADR-0086) |
| VI An toàn trẻ em | ✅ | Không log mật khẩu/token; không hiển thị tên trước khi đăng nhập xong; thông điệp không tiết lộ tài khoản |
| VII Một nguồn sự thật, cấu hình | ✅ | URL API từ `Config`; tham số theo tên; giới hạn ô nhập từ schema |
| VIII UI tách logic, token | ✅ | Scene chỉ phát intent/nhận signal; theme sinh từ `design/game-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F061-AC có test GdUnit4/integration viết trước; FakeHttp chỉ trong test, Prism mock gắn nhãn MOCK |

## Project Structure

### Documentation (feature này)

```text
specs/061-godot-api-client-login/
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
src/game/
├── config/{local,staging,production}.json   # api_base_url (không oidc_*); mock.json nhãn MOCK do F060
├── scripts/networking/
│   ├── api_client.gd            # autoload ApiClient: request(), post_public(), header, retry, refresh hook
│   ├── http_transport.gd        # class_name HttpTransport — bọc HTTPRequest pool (thay bằng FakeHttp trong test)
│   ├── http_request_pool.gd · backoff.gd · problem.gd (class_name Problem) · api_result.gd (class_name ApiResult)
│   └── trace_context.gd         # sinh traceparent
├── scripts/authentication/
│   ├── auth_service.gd          # autoload AuthService: resume(), login(), select_tenant(), confirm_takeover(), decline_takeover(), refresh() (single-flight), logout()
│   ├── token_store.gd           # class_name TokenStore — access token RAM; refresh token RAM + SessionStore
│   ├── session_store.gd         # class_name SessionStore — đọc/ghi/xóa bản ghi `eduworld.game.session` qua SecureStorage (F060)
│   ├── password_buffer.gd       # class_name PasswordBuffer — PackedByteArray, wipe() khi gameLogin có phản hồi cuối
│   ├── login_form.gd            # class_name LoginFormModel — trạng thái form, map code → khóa i18n
│   └── game_session_service.gd  # create(takeover) + Idempotency-Key, heartbeat REST, end, signal session_replaced
├── scripts/core/
│   ├── version_check.gd         # getClientConfig, so SemVer
│   ├── device_identity.gd       # user://device_id (Web: localStorage)
│   └── lifecycle_watcher.gd     # WM_CLOSE_REQUEST, APPLICATION_PAUSED/RESUMED, idle shared_device
├── scenes/bootstrap/{Boot,VersionCheck,ForceUpdate,Maintenance}.tscn + .gd
├── scenes/auth/{ResumeSplash,Login,LoginWaiting,TenantChoice,AccountNotProvisioned,NotStudent,AccountBlocked}.tscn + .gd
├── scenes/ui/dialogs/{TakeoverConfirm,ReplacedNotice,LogoutConfirm}.tscn + .gd
├── locale/{vi,en}.po            # ui.login.*, ui.error.<CODE>
└── tests/
    ├── fakes/{fake_http.gd,fake_clock.gd,fake_secure_storage.gd}     # test double, nhãn TEST-ONLY
    ├── fixtures/auth/*.json                    # sinh từ examples OpenAPI
    ├── unit/networking/*.gd · unit/authentication/*.gd · unit/core/*.gd
    ├── scene/auth/*.gd
    └── integration/auth/*.gd                   # backend compose
```

**Structure Decision**: Theo GODOT_ARCHITECTURE §3–§4; thêm `game_session_service.gd`, `version_check.gd`, `device_identity.gd`, `lifecycle_watcher.gd` và các scene ForceUpdate/Maintenance/NotStudent/AccountBlocked/ReplacedNotice mà tài liệu đã nêu trong vòng đời nhưng chưa có trong cây thư mục.

## Phase 0: Research

→ [research.md](research.md): R1 takeover ở `createGameSession`, không giữ mật khẩu (ADR-0083); R2 transport HTTP; R3 đồng hồ hạn token; R4 giữ phiên và tự vào lại (ADR-0086).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `LoginFormModel`, `Problem`, `ApiResult`, `PasswordBuffer` (RefCounted, thuần) | Không Node, test unit |
| application | `AuthService`, `GameSessionService`, `VersionCheck` | Signal thì quá khứ: `session_resumed`, `resume_failed(code)`, `login_succeeded`, `login_failed(code)`, `tenant_choice_required(choices)`, `takeover_required(other_device)`, `session_replaced`, `session_expired` (EventBus), `logged_out` |
| infrastructure | `ApiClient`, `HttpTransport`, `TokenStore`, `SessionStore` (bọc `SecureStorage`), `DeviceIdentity` | Gọi `getClientConfig`, `gameLogin`, `selectGameLoginTenant`, `refreshGameToken`, `gameLogout`, `createGameSession`, `heartbeatGameSession`, `endGameSession` |
| presentation | Scene S01 và dialog | Chỉ gọi intent của service, không HTTP |

### Luồng trạng thái (khớp GODOT_ARCHITECTURE §5 theo ADR-0083)

`Boot → VersionCheck (getClientConfig) → [ForceUpdate | Maintenance | Resume (có bản ghi SecureStorage) | Login]`; `Resume (refreshGameToken mode=resume, device_id, X-Tenant-Id đã lưu) → { 200: ghi đè SessionStore, áp bootstrap → SessionStart | 401 REFRESH_TOKEN_* / 403: xóa SessionStore → Login + thông báo | mạng/5xx/429: giữ bản ghi, thử lại | 426: ForceUpdate }`; `Login → SigningIn (gameLogin) → { 200 (tokens, bootstrap; xóa mật khẩu): SessionStart | 403 TENANT_NOT_SELECTED: TenantChoice → selectGameLoginTenant → 200: SessionStart | 429 LOGIN_QUEUE_BUSY: LoginWaiting | lỗi: Login/NotStudent/AccountNotProvisioned/AccountBlocked/ForceUpdate }` → `SessionStart (createGameSession, Idempotency-Key mới) → { 201: InWorld-ready (game_session_id) | 409 ACTIVE_SESSION_ON_OTHER_DEVICE: TakeoverConfirm → Tiếp tục: createGameSession takeover=true (key mới, không mật khẩu) / Ở lại: gameLogout → Login | lỗi khác: gameLogout → Login/ForceUpdate }`. Đăng xuất (nút ở Cài đặt → Tài khoản, `LogoutConfirm`): `endGameSession` → `gameLogout` → xóa RAM + `SessionStore` → Login. Đóng cửa sổ: thiết bị cá nhân → `endGameSession` (best effort), giữ `SessionStore`; `shared_device` → như Đăng xuất. Sau vào game: heartbeat mỗi `ws_heartbeat`; `409`/`404` heartbeat → ReplacedNotice → xóa `SessionStore` → Login.

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F013 (`gameLogin`, `selectGameLoginTenant`, `refreshGameToken` có `device_id`/`mode`/rotation/`REFRESH_TOKEN_*`, `gameLogout`), F014 (`getClientConfig`, `createGameSession` + `409`/takeover, `heartbeatGameSession`, `endGameSession`), F060 (autoload, theme, i18n, CI, Prism mock, `SecureStorage`). F133 thêm adapter iOS Keychain.
- Dùng bởi: F062+ (mọi service domain nhận `ApiClient` qua constructor).

### Bảo mật, tenant, idempotency, audit

- Không secret, không URL Keycloak; `ApiClient` tắt ghi body/header nhạy cảm cho `/auth/game/*` và mọi `Authorization`; `Log` lọc khóa `password`, `access_token`, `refresh_token`, `login_ticket`.
- `X-Tenant-Id` gắn theo bảng ngoại lệ `tenant_header_required`; không đổi trong phiên.
- Refresh token: ghi đè `SessionStore` ngay khi nhận token mới (login, tenant:select, refresh); refresh single-flight (`_refresh_in_flight`), không bao giờ gửi lại token cũ; xóa `SessionStore` ở mọi lỗi xác định (`401 REFRESH_TOKEN_*`, `403` tài khoản/tenant, 4409/heartbeat `409`, `ACCOUNT_NOT_PROVISIONED`, Đăng xuất). Không đọc/ghi `SecureStorage` từ scene (R1).
- `gameLogin`/`selectGameLoginTenant` không dùng `Idempotency-Key` (spec); `createGameSession` dùng `Idempotency-Key` do `GameSessionService` sinh, giữ khi retry mạng, sinh key mới cho lời gọi `takeover: true`. Audit login do backend.

### Quan sát

- Telemetry (không PII): thời gian đăng nhập, số lần vào hàng chờ, `client_bug` khi `TENANT_HEADER_REQUIRED`, kết quả resume (`resumed`/`expired`/`network`), `auth_refresh_reused`, `secure_storage_write_failed`; `traceparent` mỗi request để liên kết log server; log client chỉ `code` + `trace_id`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
