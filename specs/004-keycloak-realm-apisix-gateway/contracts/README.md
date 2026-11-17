# Contracts: F004 — Keycloak realm as code và APISIX gateway (route, `tenant-header`, gateway nội bộ)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| mọi operation | `x-service` (module sở hữu, không còn quyết định upstream), `x-service-routing`, `x-internal`, tham số `X-Tenant-Id` | `specs/openapi/eduworld-api.v1.yaml` | — (gateway không kiểm permission) | Có sẵn — đầu vào `render.mjs`/`check-routes.mjs` (upstream luôn `core-api`, `/realtime/v1` → `realtime`); `x-internal` Có sẵn (áp 2026-11-17) (§6) |
| `gameLogin` và nhóm `/auth/game/*` | `POST /api/v1/auth/game/login`, `tenant:select`, `token:refresh`, `logout` | `specs/openapi/eduworld-api.v1.yaml` | Không token (login/select) | Có sẵn — route không ghi body |
| `portalLogin` và nhóm `/auth/portal/*` | `POST /api/v1/auth/portal/login`, `token:refresh`, `logout`, `setup:password`, `password:forgot`, `password:reset` | `specs/openapi/eduworld-api.v1.yaml` | Không token (cookie refresh + `X-EW-CSRF` cho `token:refresh`/`logout`) | Mới (ADR-0091, chủ sở hữu F012) — route public dưới host portal, không ghi body/cookie, rate `portal_login_rate`, miễn header tenant |
| `getClientConfig` | `GET /api/v1/game/client-config` | `specs/openapi/eduworld-api.v1.yaml` | Không token | Có sẵn — route không `openid-connect`, không header tenant |
| `listMyTenants` | `GET /api/v1/me/tenants` | `specs/openapi/eduworld-api.v1.yaml` | Token | Có sẵn — miễn header tenant |
| — (component) | `ProblemCode`: `TENANT_HEADER_REQUIRED`, `BAD_REQUEST`, `RATE_LIMITED`, `LOGIN_QUEUE_BUSY`, `UNAUTHENTICATED`, `UNAVAILABLE` | `specs/openapi/eduworld-api.v1.yaml` | — | Có sẵn — body Problem do plugin trả |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| mọi operation internal | Không dùng: facade gọi trong tiến trình, không có token hay client scope cho module (ADR-0074) | `specs/openapi/modules/*.internal.v1.yaml` | — | Không áp dụng |
| — | `/internal/v1/*` không khai báo trên APISIX | — | — | Không áp dụng |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Route `/realtime/v1` chỉ chuyển tiếp WebSocket; không xử lý message | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không dùng | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 6. Delta cần thực hiện

- **Đã áp 2026-11-17 (T005).** Spec gộp: thêm `x-internal: true` cho 20 operation `/admin/*` đang thiếu — `listTenantSensitiveTerms`, `createTenantSensitiveTerm`, `updateTenantSensitiveTerm`, `deleteTenantSensitiveTerm`, `publishTenantSensitiveCatalog`, `toggleTenantSensitiveCategory`, `listModerationCases`, `getModerationCase`, `listModerationCaseMessages`, `assignModerationCase`, `actOnModerationCase`, `notifySchoolForCase`, `listSanctions`, `createSanction`, `revokeSanction`, `listSchoolSafetyNotificationsPlatform`, `listPlatformAuditLogs`, `listDataRequests`, `createDataRequest`, `getDataRequest`; chạy `split-public.mjs`. Task T005.
