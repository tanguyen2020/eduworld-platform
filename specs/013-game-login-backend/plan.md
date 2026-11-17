# Implementation Plan: F013 — Đăng nhập game qua backend: login, chọn tenant, refresh, logout, bảo vệ đăng nhập, hàng chờ

**Branch**: `013-game-login-backend` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

module identity thêm command `GameLogin`, `SelectGameLoginTenant`, `RefreshGameToken` (`renew`/`resume`), `GameLogout`, `RevokeGameAuthSessions` (consumer sự kiện). `GameLogin`: validate body (không log) → kiểm `client_version` → rate limit (Valkey) → hàng chờ toàn cục → password grant Keycloak (`eduworld-game-backend`, pool HTTP, timeout, circuit breaker) → xóa mật khẩu → resolve `(iss, sub)` → kiểm `user_type`, trạng thái → membership → `LoginSync` (F011) → quyết định tenant → bootstrap. Nhiều tenant: lưu token set đã mã hóa trong Valkey dưới `login_ticket`. Khi phát token (login/select), ghi aggregate `GameAuthSession` (`sid`, `device_id`, hash refresh token) trong transaction; `RefreshGameToken` tìm phiên theo hash → kiểm thu hồi/dùng lại/thiết bị → refresh Keycloak → cập nhật hash (rotation); `mode = resume` kiểm lại tài khoản/tenant và trả bootstrap (ADR-0086). Middleware `tenant` (khung `src/backend/pkg/httpx` của F002) được hoàn thiện với kiểm tra membership và dùng chung cho mọi service.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, oapi-codegen, `golang.org/x/text/unicode/norm` (NFC), `src/backend/pkg/httpx`, `src/backend/pkg/modcall` (failsafe-go cho Keycloak), Valkey client, `src/backend/pkg/observability` (redaction) |
| Storage | PostgreSQL `eduworld_identity` (`identity.login_events` TA, P tháng; `identity.game_auth_sessions` T, soft-delete — ADR-0086; `users`, `external_identities`); Valkey: `ew:identity:login_ticket:<sha256>` (TTL `game_login_ticket_ttl`), `ew:identity:rl:login:device:<device_id>`, `ew:identity:rl:login:user:<sha256(username)>`, `ew:identity:rl:login:global` |
| Testing | go test + testcontainers (PostgreSQL, Valkey, Keycloak test realm); k6 cho hàng chờ (TC-PERF-016) |
| Target Platform | Linux container |
| Project Type | backend module + platform kit (middleware `tenant`) |
| Performance Goals | `login_slo` (`auth/game/login` p95 gồm Keycloak); resolve sau token ≤ 150 ms p95 ([IDENTITY_MAPPING §3.1](../../docs/09-identity/IDENTITY_MAPPING.md)) |
| Constraints | AGENTS R3, R4, R7, R14, R23, R26, R30; ADR-0031, ADR-0045, ADR-0063, ADR-0064 (mục 3, 6, 7), ADR-0065, ADR-0068, ADR-0069, ADR-0085, ADR-0086, ADR-0087 |
| Scale/Scope | `login_capacity_target` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `GameLoginRequest`/`GameLoginResponse`/`PlayerBootstrap` ở Phase 2; contract refresh ADR-0086 (`device_id`, `mode`, `GameTokenRefreshResponse`, 3 mã `REFRESH_TOKEN_*`) đã áp dụng vào spec gộp |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Không gọi module game khi đăng nhập/refresh; game session ở F014 (ADR-0083); takeover nhận qua event `game_session.replaced`; bootstrap chỉ dữ liệu identity. CQRS theo ADR-0087: command handler ở `internal/application/command/` (port `port/write`, `UnitOfWork`), query (bootstrap) ở `internal/application/query/` (port `port/read`); handler gọi trực tiếp, authz tường minh, không mediator |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Tenant từ membership DB (hàm `identity.memberships_for_user` trong `tenancy.Bypass(ctx, "auth_login")`, không `BYPASSRLS`, có audit — ADR-0089); dòng `login_events` tenant NULL ghi bằng `identity_platform` dưới `PlatformScope`; middleware `tenant` mọi route; `login_events` append-only có `tenant_id`; `game_auth_sessions` có `tenant_id`, chỉ lưu hash token, thu hồi = `revoked_at` (không xóa — R21) |
| IV Server-authoritative | ✅ | Không có điểm/thưởng; logout idempotent; consumer thu hồi idempotent theo `event_id`; audit mọi từ chối/thu hồi quan trọng (reuse, sai thiết bị) |
| V Xác thực, phân quyền | ✅ | Secret Keycloak qua file secret (`EW_KC_GAME_CLIENT_SECRET_FILE`); chỉ học sinh; rate limit không theo IP |
| VI An toàn trẻ em | ✅ | Không log mật khẩu/username thô; bootstrap không PII thừa; thông điệp lỗi không lộ tồn tại tài khoản |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo CONFIG_DEFAULTS; `min_client_version` qua cấu hình (đăng ký CONFIG_DEFAULTS) |
| VIII UI tách logic, token | ✅ | Không có UI (Godot F061) |
| IX Test có bằng chứng | ✅ | Test cho mọi F013-AC-*; Keycloak thật trong testcontainers, không mock trong đường chạy thật |

## Project Structure

### Documentation (feature này)

```text
specs/013-game-login-backend/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/pkg/httpx/tenant.go                    # middleware tenant: header bắt buộc, khớp membership (khung F002, hoàn thiện ở đây)
src/backend/pkg/observability/redact.go            # che field password/refresh_token/login_ticket ở mọi logger/tracer
src/backend/modules/identity/
├── ent/schema/loginevent.go           # identity.login_events (TenantOptionalAppendOnly, ADR-0089)
├── ent/schema/gameauthsession.go      # identity.game_auth_sessions (ADR-0086)
├── internal/domain/gamelogin/         # TenantDecision (1/nhiều/0), LoginTicket, ClientVersion, GameAuthSession (rotate, detect reuse, bind device, revoke)
├── internal/application/command/      # GameLogin, SelectGameLoginTenant, RefreshGameToken (renew/resume), GameLogout, RevokeGameAuthSessions
├── internal/application/query/        # BootstrapQuery (dùng chung login/select/resume)
├── internal/application/port/write/   # GameAuthSessionRepository, UnitOfWork, KeycloakTokenClient, KeycloakSessionRevoker, LoginTicketStore, LoginRateLimiter, LoginSync (F011)
├── internal/application/port/read/    # BootstrapReader
├── internal/presentation/consumer/    # revoke_on_access_revoked.go, revoke_on_student_cleaned_up.go (revoke_on_session_replaced.go thuộc F014)
├── internal/infrastructure/keycloak/token_client.go   # adapter ngoài (Keycloak): password grant / refresh / revoke, pool HTTP, timeout, circuit breaker
├── internal/infrastructure/cache/{login_ticket_store.go, login_rate_limiter.go}
├── internal/presentation/http/public/game_auth.go     # gameLogin, selectGameLoginTenant, refreshGameToken, gameLogout
└── tests/{unit,integration,security}/
```

**Structure Decision**: Theo BACKEND_DESIGN §3, §6.1; middleware dùng chung ở `src/backend/pkg/httpx` để mọi module trong `core-api` áp cùng quy tắc ADR-0065.

## Phase 0: Research

→ [research.md](research.md): lưu token set khi `TENANT_NOT_SELECTED`; nguồn `min_client_version`; game session tách khỏi đăng nhập.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `TenantDecision.From(memberships)`, `LoginTicket` (một lần, `device_id`), `ClientVersion.AtLeast`, `GameAuthSession` (`Rotate(newHash)`, `Classify(hash, deviceID)` → ok/reused/device_mismatch/revoked, `Revoke(reason)`) | Thứ tự lỗi theo IDENTITY_MAPPING §4.1 |
| application | Command: `GameLogin`, `SelectGameLoginTenant`, `RefreshGameToken` (`renew`/`resume`), `GameLogout`, `RevokeGameAuthSessions`; query: `BootstrapQuery` | Public (login, select, refresh — refresh token trong body là credential), học sinh (logout); handler gọi trực tiếp từ presentation, không mediator (ADR-0087) |
| infrastructure | Keycloak token client, ticket store (AES-GCM, khóa từ secret), rate limiter, bootstrap reader (users/students/schools/classes/tenant settings/feature flags), `LoginMembershipReader` (gọi `identity.memberships_for_user(user_id)` trong `Bypass(ctx, "auth_login")`), `LoginEventWriter` (dòng tenant NULL qua client `identity_platform` + `PlatformScope`) | Lý do `auth_login` và hàm chỉ dùng trong package đăng nhập (F013-AC-30, F013-AC-31) |
| presentation | Handler `gameLogin`, `selectGameLoginTenant`, `refreshGameToken`, `gameLogout`; consumer `user.access_revoked`, `user.status_changed`, `student.cleaned_up`; middleware `tenant` (`src/backend/pkg/httpx`) | Không `Idempotency-Key` cho login/refresh (không lưu token vào kho idempotency) |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F011 (`LoginSync`), F012 (Principal, membership, cache), F004 (client Keycloak, APISIX rate limit, không log body).
- Event phát: `user.activated`, `audit.recorded` (`account_not_provisioned`, `game_access_denied_non_student`, `tenant_mismatch`, `auth.game_logout`, `auth.game_session_revoked`, `auth.refresh_token_reused`, `auth.refresh_device_mismatch`).
- Event tiêu thụ (ADR-0086): `user.access_revoked`, `user.status_changed` (F016), `student.cleaned_up` (F016). Port `GameAuthSessionRevoker` được gọi bởi consumer `game_session.replaced` của F014 (takeover) và use case đổi/đặt lại mật khẩu của F016 (cùng service).
- Keycloak: `POST /realms/{realm}/protocol/openid-connect/token` (password, refresh), `/logout` (revoke), Admin `DELETE /sessions/{sid}` (thu hồi phiên game: reuse, takeover, mật khẩu, khóa, dọn).

### Bảo mật, tenant, idempotency, audit

- APISIX + `core-api`: không log body route `/auth/game/*` (`game_login_body_logging`); redaction ở `src/backend/pkg/observability`.
- Rate limit: thiết bị + username + toàn cục (`login_rate`); brute-force do Keycloak; thông điệp chung `INVALID_CREDENTIALS`, cân bằng thời gian phản hồi.
- Refresh (ADR-0086): tìm `GameAuthSession` theo SHA-256 token; hash đã xoay → `REFRESH_TOKEN_REUSED` + thu hồi cả phiên; `device_id` khác → `REFRESH_TOKEN_DEVICE_MISMATCH` (không gọi Keycloak); không thấy/đã thu hồi/Keycloak `invalid_grant` → `REFRESH_TOKEN_INVALID`. Cập nhật hash trong transaction có khóa hàng (`SELECT … FOR UPDATE`) để hai refresh song song không cùng thành công.
- Middleware `tenant`: `X-Tenant-Id` bắt buộc ngoài danh sách miễn; so với membership (`checkMemberships`/cache) và tenant của game session (token game) → `TENANT_MISMATCH` + audit.

### Quan sát

- Metric `eduworld_auth_game_refresh_total{mode,result}`, `eduworld_auth_game_sessions_revoked_total{reason}` (MONITORING), `eduworld_auth_game_login_total{result}`, `eduworld_auth_game_login_duration_seconds`, `eduworld_auth_login_queue_rejected_total`, `eduworld_auth_rate_limited_total{key}`, `eduworld_auth_tenant_mismatch_total`, `eduworld_keycloak_token_call_duration_seconds`; span không chứa body.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
