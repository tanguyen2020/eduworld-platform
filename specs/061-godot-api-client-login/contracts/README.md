# Contracts: F061 — `ApiClient`, đăng nhập game trong form, chọn tenant, takeover, buộc cập nhật, đăng xuất

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml`. Game client không có permission portal; route người chơi yêu cầu token học sinh (middleware `game_player`, R23).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getClientConfig` | `GET /api/v1/game/client-config?platform=&version=` | eduworld-api (game) | Public, không `X-Tenant-Id` | Có sẵn |
| `gameLogin` | `POST /api/v1/auth/game/login` | eduworld-api (identity) | Public; body `GameLoginRequest`; 200 `GameLoginResponse` (`tokens`, `bootstrap`, không `game_session_id`); 401/403/422/426/429 `GameLoginProblem`/`ClientVersionProblem` | Có sẵn (ADR-0083) |
| `selectGameLoginTenant` | `POST /api/v1/auth/game/tenant:select` | eduworld-api (identity) | Public; body `GameTenantSelectRequest` | Có sẵn |
| `refreshGameToken` | `POST /api/v1/auth/game/token:refresh` | eduworld-api (identity) | `X-Tenant-Id` (đã lưu cùng refresh token); body `GameTokenRefreshRequest` (`refresh_token`, `device_id`, `client_version`, `mode` = `renew`/`resume`); 200 `GameTokenRefreshResponse` (token xoay vòng + `bootstrap` khi `resume`); `401 REFRESH_TOKEN_INVALID`/`REFRESH_TOKEN_REUSED`/`REFRESH_TOKEN_DEVICE_MISMATCH`; 403 tài khoản/tenant; 426; 429 | Cập nhật theo ADR-0086 (F013 sửa OpenAPI) |
| `gameLogout` | `POST /api/v1/auth/game/logout` | eduworld-api (identity) | Bearer + `X-Tenant-Id`; body `GameLogoutRequest`; 204; không đóng game session — gọi sau `endGameSession` | Có sẵn |
| `heartbeatGameSession` | `POST /api/v1/game/sessions/{game_session_id}:heartbeat` | eduworld-api (game) | Học sinh; `X-Tenant-Id`, `X-Device-Id`; 204/404/409 | Có sẵn |
| `endGameSession` | `POST /api/v1/game/sessions/{game_session_id}:end` | eduworld-api (game) | Học sinh; 204/404 | Có sẵn |
| `createGameSession` | `POST /api/v1/game/sessions` | eduworld-api (game) | Học sinh; `X-Tenant-Id`, `X-Device-Id`, `Idempotency-Key`; body `GameSessionCreate` (`takeover`); 201 `GameSession`; 409 `ActiveSessionConflict` (`other_device`) — gọi ngay sau đăng nhập (ADR-0083) | Có sẵn |

Schema dùng: `GameLoginRequest`, `GameLoginResponse`, `GameSessionCreate`, `GameSession`, `ActiveSessionConflict`, `OtherDevice`, `GameTokenSet`, `GameTokenRefreshRequest`, `GameTokenRefreshResponse`, `GameTenantSelectRequest`, `GameLoginTenantChoice`, `GameLoginProblem`, `ClientVersionProblem`, `PlayerBootstrap`, `ClientConfig`, `Problem`, `ProblemCode`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (client) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `session.session.replaced` | server → client | Có sẵn trong `specs/asyncapi/eduworld-realtime.v1.yaml`; F061 không dùng (F092 xử lý qua WS) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không kết nối NATS (R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không | — |

## 6. Delta cần thực hiện

- F061 không sở hữu delta contract. Contract đăng nhập đã theo ADR-0083 (bỏ `game_session_id`, `takeover`, `409` khỏi `gameLogin`/`selectGameLoginTenant`).
- Phụ thuộc delta ADR-0086 do F013 thực hiện trên `refreshGameToken`: `device_id` (bắt buộc), `mode`, response `GameTokenRefreshResponse` có `bootstrap`, `ProblemCode` mới `REFRESH_TOKEN_INVALID`, `REFRESH_TOKEN_REUSED`, `REFRESH_TOKEN_DEVICE_MISMATCH`. Client sinh lại fixture `tests/fixtures/auth/*.json` sau khi spec gộp cập nhật.
- Hợp đồng nội bộ client: `SecureStorage` (F060) — `SessionStore` chỉ dùng khóa `eduworld.game.session`.
- Tài liệu được sửa trong PR ADR-0083; task T004 chỉ kiểm lại: GODOT_ARCHITECTURE §4 (bỏ `oidc_issuer`/`oidc_client_id`, bỏ "xin token mới với scope `organization:`"), §5 (SessionStart gọi `createGameSession` sau login), §11 (bỏ intent filter OAuth, redirect OIDC Web), §13.2 (đọc `tokens.*`); GAME_NETWORKING §4.5 (409 ở `createGameSession`); STUDENT_GAME_UX_CORE S01 (bỏ "nhớ lựa chọn trường trên thiết bị"; tự vào lại, Đăng xuất, máy dùng chung theo ADR-0086); KEYCLOAK_SSO §6.2 (gọi `POST /game/sessions` sau đăng nhập).
- ADR-0078: client xử lý `403 CURFEW_ACTIVE` (`Problem.reopen_at`, owner F013/F014) và khung giới nghiêm trong `PlayerBootstrap.policies.play_time` (owner F015).
