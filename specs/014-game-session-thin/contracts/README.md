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
| `resolvePrincipal` | game → identity | identity.internal.v1.yaml | gồm game | Có sẵn (áp 2026-11-17) (F012) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `session.session.replaced` | server → thiết bị cũ (qua Valkey `ew:rt:user:<id>` → realtime, close 4409) | Có sẵn (`specs/asyncapi/eduworld-realtime.v1.yaml`) |
| `session.session.ended` | server → client khi game session kết thúc phía server (qua Valkey `ew:rt:user:<id>` → realtime): giới nghiêm F014-AC-26 → close 4410; lý do thu hồi → close 4403 | Có sẵn (áp 2026-11-17, owner F014) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `student.game_logged_in` | game → identity (`last_login_at`), learning (`ref.login_activity`), AUD | `specs/events/payloads/game/student.game_logged_in.v1.schema.json` | Có sẵn (áp 2026-11-17) |
| `student.game_logged_out` | game → learning, AUD | `student.game_logged_out.v1.schema.json` | Có sẵn (áp 2026-11-17) |
| `game_session.started`, `game_session.ended` | game → learning, analytics, AUD | `game_session.started.v1.schema.json`, `game_session.ended.v1.schema.json` | Có sẵn (áp 2026-11-17) |
| `game_session.replaced` | game → identity (thu hồi `sid`), competition, AUD | `game_session.replaced.v1.schema.json` (thêm `old_keycloak_sid`, `old_device_id` — ADR-0086) | Có sẵn (áp 2026-11-17) |
| `tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up`, `student.restored` | identity → game (projection) | `specs/events/payloads/identity/` | Có sẵn (áp 2026-11-17), owner F010/F011/F016 |
| `user.access_revoked`, `user.status_changed`, `student.cleaned_up` | identity → game (kết thúc session, F014-AC-24) | `specs/events/payloads/identity/` (owner F011 `user.*`, owner F016 `student.cleaned_up`; fixture F013 T045) | Có sẵn (áp 2026-11-17), owner F011/F016 |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- **Đã áp 2026-11-17 (T004).** Event payload JSON Schema thư mục mới `specs/events/payloads/game/`: `student.game_logged_in`, `student.game_logged_out`, `game_session.started`, `game_session.ended`, `game_session.replaced` (thêm `old_keycloak_sid` so với EVENT_DESIGN §4.5 — cập nhật EVENT_DESIGN cùng MR).
- Đã áp dụng theo ADR-0083 (PR chốt D15): `createGameSession` là nơi duy nhất trả `409 ACTIVE_SESSION_ON_OTHER_DEVICE` + nhận `takeover`; đăng nhập (F013) không còn `game_session_id`/`takeover`.
- **Đã áp 2026-11-17 (T005).** Spec gộp `eduworld-api.v1.yaml` (rồi `split-public.mjs`, không sửa tay `game.v1.yaml`) `createGameSession`: thêm response `426` (`CLIENT_VERSION_UNSUPPORTED`) và `503` (`MAINTENANCE`) vào khai báo; `heartbeatGameSession`/`endGameSession`: bổ sung `403` `GAME_ACCESS_STUDENT_ONLY`.
- Kiểm: `make specs-lint`, `make gen MOD=game`, `make test-contract`.
- **Đã áp 2026-11-17 (phần contract của T041, T043):** `createGameSession` khai báo `403 CURFEW_ACTIVE` (mô tả + `Problem.reopen_at`); `end_reason` (`LoginEndReason`, payload `game_session.ended`/`student.game_logged_out`) thêm `curfew`. AsyncAPI `session.session.ended` (server → client; `reason` `curfew`/`account_locked`/`cleanup`/`access_revoked`/`admin`/`password_changed`, `reopen_at` khi `curfew`, `ended_at`, `game_session_id`) + close code `4410` (chỉ `curfew`; lý do thu hồi đóng `4403`) đã thêm theo quyết định Technical Architect 2026-11-17 (owner F014, F090 giao/đóng; REALTIME_ARCHITECTURE §3.1, GAME_NETWORKING §4.4). Nguyên văn: ADR-0078: `createGameSession` khai báo `403 CURFEW_ACTIVE` (+ `Problem.reopen_at`); `GameSession.end_reason` thêm `curfew`; AsyncAPI `session.session.ended` có `reason = curfew` [Proposed] (task T041, T043).
