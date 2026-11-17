# Contracts: F014 — Game session lát mỏng: một thiết bị, login events, client-config, manifest tối thiểu

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/game.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createGameSession` | `POST /api/v1/game/sessions` (`X-Tenant-Id`, `X-Device-Id`, `Idempotency-Key`; body `GameSessionCreate`; `409` `ActiveSessionConflict`) | game.v1.yaml | `game_player` (học sinh) | Có sẵn |
| `endGameSession` | `POST /api/v1/game/sessions/{game_session_id}:end` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `heartbeatGameSession` | `POST /api/v1/game/sessions/{game_session_id}:heartbeat` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `getClientConfig` | `GET /api/v1/game/client-config?platform=&version=` (public, không `X-Tenant-Id`) | game.v1.yaml | public | Có sẵn |
| `getContentManifest` | `GET /api/v1/game/content-manifest` (`ETag`, `If-None-Match`) | game.v1.yaml | `game_player` | Có sẵn |
| `listTenantStudentLoginEvents` | `GET /api/v1/tenant/students/{student_id}/login-events` | game.v1.yaml | `lms.student_activity.read` (scope) | Có sẵn |
| `listLoginEventsCrossTenant` | `GET /api/v1/admin/login-events?tenant_ids=` | game.v1.yaml | `platform.cross_tenant.read` (MFA, gateway nội bộ) | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `resolvePrincipal` | game → identity | identity.internal.v1.yaml | gồm game | Cần sửa (F012) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `session.session.replaced` | server → thiết bị cũ (qua Valkey `ew:rt:user:<id>` → realtime, close 4409) | Có sẵn (`specs/asyncapi/eduworld-realtime.v1.yaml`) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `student.game_logged_in` | game → identity (`last_login_at`), learning (`ref.login_activity`), AUD | `specs/events/payloads/game/student.game_logged_in.v1.schema.json` | Cần thêm |
| `student.game_logged_out` | game → learning, AUD | `student.game_logged_out.v1.schema.json` | Cần thêm |
| `game_session.started`, `game_session.ended` | game → learning, analytics, AUD | `game_session.started.v1.schema.json`, `game_session.ended.v1.schema.json` | Cần thêm |
| `game_session.replaced` | game → identity (thu hồi `sid`), competition, AUD | `game_session.replaced.v1.schema.json` (thêm `old_keycloak_sid`, `old_device_id` — ADR-0086) | Cần thêm |
| `tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up`, `student.restored` | identity → game (projection) | `specs/events/payloads/identity/` | Cần thêm (F010, F011, F016) |
| `user.access_revoked`, `user.status_changed`, `student.cleaned_up` | identity → game (kết thúc session, F014-AC-24) | `specs/events/payloads/identity/` (owner F011 `user.*`, owner F016 `student.cleaned_up`; fixture F013 T045) | Cần thêm (F011 T005, F016 T004) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- Event payload JSON Schema thư mục mới `specs/events/payloads/game/`: `student.game_logged_in`, `student.game_logged_out`, `game_session.started`, `game_session.ended`, `game_session.replaced` (thêm `old_keycloak_sid` so với EVENT_DESIGN §4.5 — cập nhật EVENT_DESIGN cùng MR).
- Đã áp dụng theo ADR-0083 (PR chốt D15): `createGameSession` là nơi duy nhất trả `409 ACTIVE_SESSION_ON_OTHER_DEVICE` + nhận `takeover`; đăng nhập (F013) không còn `game_session_id`/`takeover`.
- Spec gộp `eduworld-api.v1.yaml` (rồi `split-public.mjs`, không sửa tay `game.v1.yaml`) `createGameSession`: thêm response `426` (`CLIENT_VERSION_UNSUPPORTED`) và `503` (`MAINTENANCE`) vào khai báo; `heartbeatGameSession`/`endGameSession`: bổ sung `403` `GAME_ACCESS_STUDENT_ONLY`.
- Kiểm: `make specs-lint`, `make gen MOD=game`, `make test-contract`.
- ADR-0078: `createGameSession` khai báo `403 CURFEW_ACTIVE` (+ `Problem.reopen_at`); `GameSession.end_reason` thêm `curfew`; AsyncAPI `session.session.ended` có `reason = curfew` [Proposed] (task T041, T043).
