# Contracts: F013 — Đăng nhập game qua backend: login, chọn tenant, refresh, logout, bảo vệ đăng nhập, hàng chờ

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/identity.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `gameLogin` | `POST /api/v1/auth/game/login` (public, không `X-Tenant-Id`) | identity.v1.yaml | public; kết quả chỉ cho `user_type = student` | Có sẵn (đã áp dụng ADR-0083); còn `PlayerBootstrap` cần sửa |
| `selectGameLoginTenant` | `POST /api/v1/auth/game/tenant:select` (public, `login_ticket`) | identity.v1.yaml | public (ticket) | Có sẵn (response `GameLoginResponse`, không `409` — ADR-0083) |
| `refreshGameToken` | `POST /api/v1/auth/game/token:refresh` (`X-Tenant-Id`; body `refresh_token`, `device_id`, `mode` `renew`/`resume`, `client_version`) | identity.v1.yaml | public — refresh token trong body là credential, phiên gắn `device_id`; chỉ phiên của client `eduworld-game-backend` | Đã cập nhật theo ADR-0086 (`GameTokenRefreshResponse` có `bootstrap` khi `resume`; `401` `REFRESH_TOKEN_*`; `403`, `422`, `426`, `503`) |
| `gameLogout` | `POST /api/v1/auth/game/logout` | identity.v1.yaml | học sinh; `204` idempotent; không đóng game session (ADR-0083) | Có sẵn |

Schema liên quan: `GameLoginRequest`, `GameLoginResponse`, `GameTokenSet`, `GameTokenRefreshRequest`, `GameTokenRefreshResponse`, `PlayerBootstrap`, `ProblemCode` (`INVALID_CREDENTIALS`, `LOGIN_TICKET_INVALID`, `REFRESH_TOKEN_INVALID`, `REFRESH_TOKEN_REUSED`, `REFRESH_TOKEN_DEVICE_MISMATCH` — mới ADR-0086, `CLIENT_VERSION_UNSUPPORTED`, `LOGIN_QUEUE_BUSY`, `RATE_LIMITED`, `ACCOUNT_NOT_PROVISIONED`, `GAME_ACCESS_STUDENT_ONLY`, `TENANT_NOT_SELECTED`, `TENANT_MISMATCH`, `TENANT_HEADER_REQUIRED`, `TENANT_ACCESS_DISABLED`, `ACCOUNT_LOCKED`, `ACCOUNT_DEACTIVATED`, `NO_ACTIVE_ENROLLMENT`, `ACCESS_OUTSIDE_ALLOWED_HOURS`, `UNAUTHENTICATED`, `UNAVAILABLE`) — có sẵn.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `checkMemberships` / `resolvePrincipal` | middleware `tenant` của `core-api` → facade `identity` | identity.internal.v1.yaml | xem F012 | Có sẵn / Cần sửa (F012) |

Keycloak (ra ngoài): token endpoint (password, refresh_token), revoke/logout, Admin API thu hồi session theo `sid`.

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có (đóng WS khi logout do client/F014/F090) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `user.activated` | identity → AUD, game (F070) | `specs/events/payloads/identity/user.activated.v1.schema.json` | Cần thêm |
| `audit.recorded` | identity → audit sink (F015) | `specs/events/payloads/common/audit.recorded.v1.schema.json` | Cần thêm (F003 T004) |
| `game_session.replaced` | game (F014) → identity (consumer F014 gọi `GameAuthSessionRevoker` của F013): thu hồi phiên game thiết bị cũ | `specs/events/payloads/game/game_session.replaced.v1.schema.json` (`old_device_id`, `old_keycloak_sid`) | Cần thêm (F014) |
| `user.access_revoked`, `user.status_changed`, `student.cleaned_up` | Tiêu thụ (identity, consumer thu hồi phiên game) | `specs/events/payloads/identity/<event>.v1.schema.json` — owner F011 (`user.*`, T005), owner F016 (`student.cleaned_up`, T004); F013 không tạo schema, chỉ dùng fixture (T045) | Cần thêm (F011, F016) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- **Đã áp dụng theo ADR-0083** (PR chốt D15, sửa `specs/openapi/eduworld-api.v1.yaml` + `split-public.mjs`): `GameLoginRequest`/`GameTenantSelectRequest` bỏ `takeover`; `GameLoginResponse` bỏ `game_session_id`; `gameLogin`/`selectGameLoginTenant` bỏ `409` và mô tả "tạo game session"; `GameLoginProblem` bỏ `other_device`; `gameLogout` không đóng game session. Task T003 chỉ kiểm lại.
- `PlayerBootstrap`: `game_profile` không còn trong `required` (module identity không đọc dữ liệu game); thêm `school`, `class`, `grade_band` vào `required` khi `is_common_tenant = false` (mô tả).
- Event payload `user.activated`.
- **Đã áp dụng theo ADR-0086** (spec gộp + `split-public.mjs`): `GameTokenRefreshRequest` thêm `device_id` (bắt buộc), `mode`, `client_version`; response `GameTokenRefreshResponse` (= `GameTokenSet` + `bootstrap` khi `resume`); `401` dùng `REFRESH_TOKEN_INVALID`/`REFRESH_TOKEN_REUSED`/`REFRESH_TOKEN_DEVICE_MISMATCH` thay `UNAUTHENTICATED`; thêm `422`, `426`, `503`; mô tả `gameLogout`, `GameTokenSet` theo ADR-0086; API_DESIGN §4.1 đồng bộ (`check-problem-codes` 153 = 153).
- Tài liệu (API_DESIGN §9.1, KEYCLOAK_SSO §6.2, GODOT_ARCHITECTURE §5) được sửa trong PR ADR-0083.
- Sửa ở spec gộp `specs/openapi/eduworld-api.v1.yaml` rồi `node scripts/openapi/split-public.mjs` (không sửa tay `identity.v1.yaml`). Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity`.
- ADR-0078: `gameLogin`, `selectGameLoginTenant`, `refreshGameToken` khai báo `403 CURFEW_ACTIVE`; `Problem` thêm `reopen_at` (date-time) cho mã này (task T047).
