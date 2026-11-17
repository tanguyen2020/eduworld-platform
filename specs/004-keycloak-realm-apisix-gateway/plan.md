# Implementation Plan: F004 — Keycloak realm as code và APISIX gateway (route, `tenant-header`, gateway nội bộ)

**Branch**: `004-keycloak-realm-apisix-gateway` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Viết realm `eduworld` dạng JSON áp bằng `keycloak-config-cli` (bản chung không user/secret + bản dev có Organization, user demo), flow xác thực tùy biến cho game và portal, client scope và audience cho token người dùng, một client service account `svc-identity` cho Keycloak Admin API (ADR-0074 mục 5). Viết cấu hình APISIX hai gateway (public, nội bộ) từ một nguồn route YAML render theo môi trường: standalone `apisix.yaml` cho local, ADC cho dev/staging/prod. Thêm hai Lua plugin (`ew-tenant-header`, `ew-login-key`), các kiểm tra CI (route ↔ spec gộp và upstream `core-api`/`realtime`, không ghi body đăng nhập, không prefix nội bộ ở public, không secret trong realm).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | JSON/YAML cấu hình; Lua (OpenResty, plugin APISIX 3.x); Node.js LTS (script kiểm) |
| Primary Dependencies | Keycloak 26.x, `keycloak-config-cli`, APISIX 3.x (standalone + etcd), ADC, `js-yaml` (đã có trong `scripts/`) |
| Storage | DB `keycloak` (Keycloak tự quản lý); Valkey `ratelimit` cho `limit-count` |
| Testing | Node test (`node --test`) cho script kiểm; test-nginx/`busted` cho plugin Lua; smoke `curl` trên compose; Postman CLI collection gateway |
| Target Platform | Container Keycloak/APISIX (compose local, Swarm sau) |
| Project Type | Hạ tầng (cấu hình) |
| Performance Goals | Gateway thêm ≤ vài ms; đăng nhập theo `login_slo` (đo ở F132) |
| Constraints | AGENTS R3, R4, R7, R20, R30 · ADR-0009, ADR-0040, ADR-0063, ADR-0064 (mục 3, 6, 7), ADR-0065, ADR-0066, ADR-0067, ADR-0068, ADR-0086, ADR-0091 |
| Scale/Scope | ~25 nhóm route public, ~10 nhóm nội bộ; 2 upstream (`core-api`, `realtime`); 5 client Keycloak |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `x-internal` trong task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Upstream theo binary (`core-api`, `realtime`), `x-service` chỉ ghi module sở hữu; không có `/internal/v1` (facade trong tiến trình, ADR-0074) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Gateway chỉ kiểm có mặt/định dạng header; khớp tenant ở `core-api` (F002) |
| IV Server-authoritative | ✅ | Không áp dụng |
| V Xác thực, phân quyền | ✅ | Client/flow đúng ADR-0066/0068; route platform chỉ nội bộ + MFA; không secret trong repo; không giới hạn theo IP đơn lẻ |
| VI An toàn trẻ em | ✅ | Không log mật khẩu/ticket; token không chứa PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Ngưỡng theo tham số CONFIG_DEFAULTS/SECURITY_ARCHITECTURE §14 qua env; route render từ một nguồn |
| VIII UI tách logic, token | ✅ | Theme đăng nhập chỉ khung, giao diện do F051 theo token |
| IX Test có bằng chứng | ✅ | Script CI + smoke + TC-AUTH-060/067, TC-TEN-011a/b, TC-CMS-WS-03 |

## Project Structure

### Documentation (feature này)

```text
specs/004-keycloak-realm-apisix-gateway/
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
deploy/keycloak/
├── realm-eduworld.json            # chung: client, flow, scope, role, policy; ${env.KC_*}
├── realm-eduworld.dev.json        # local: Organization th-demo-a/pgd-demo/common, user demo, secret change-me-local
├── themes/eduworld-ops/login/{theme.properties,messages/messages_vi.properties,messages_en.properties}
└── scripts/{check-realm.mjs,apply.sh}
deploy/apisix/
├── routes/routes.yaml             # nguồn duy nhất: route, gateway, upstream, plugins, priority
├── routes/env/{local,dev,staging,production}.env.example
├── plugins/{ew-tenant-header.lua,ew-login-key.lua}
├── standalone/public/apisix.yaml  standalone/internal/apisix.yaml   # sinh cho local (kết thúc #END)
├── adc/{public,internal}.yaml                                       # sinh cho dev/staging/prod
├── scripts/{render.mjs,check-routes.mjs,check-no-body-logging.mjs}
└── tests/{tenant_header.t,login_key.t}
deploy/compose/docker-compose.yml  # thêm apisix-internal, mount plugins, keycloak-config-cli job
.github/workflows/deploy-config.yml   # thêm job APISIX/Keycloak vào workflow khung của F001
specs/openapi/eduworld-api.v1.yaml # delta x-internal
```

**Structure Decision**: Một file route nguồn render ra hai gateway và hai chế độ triển khai, để CI so được với spec gộp và tránh lệch giữa môi trường (DEPLOYMENT §5).

## Phase 0: Research

→ [research.md](research.md): R1 plugin `ew-tenant-header`; R2 khóa username không ghi body; R3 hành vi khi Valkey lỗi; R4 công cụ áp realm; R5 spike direct grant cho portal (`amr`/`acr`, required action — ADR-0091) [Proposed].

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain / application | — | Không có code Go |
| infrastructure | Realm JSON, flow `eduworld-game-direct-grant` (Username Validation → Password → Condition User Role `student` → Deny), `eduworld-portal-direct-grant` (Username Validation → Password → Allow access only for role `tenant_staff`/`platform_staff` → Conditional OTP theo `platform_staff`; ADR-0091 thay `eduworld-portal-browser`), mapper AMR + LoA | KEYCLOAK_SSO §3.1, §3.3 |
| presentation | Route APISIX public/nội bộ, plugin | API_DESIGN §10 |

**Mô hình route** (`routes.yaml`, mỗi mục): `id`, `uris`, `gateway: public|internal|both`, `upstream: core-api|realtime`, `module: <module sở hữu>` (từ `x-service`), `auth: oidc|none|ticket`, `tenant_header: required|optional|none`, `rate: <khóa tham số>`, `body_max: <khóa tham số>`, `log_body: false` (bắt buộc false cho `/auth/game/*` và `/auth/portal/*`), `host` (route `/auth/portal/*` gắn `EW_PORTAL_HOST` — cùng origin SPA, ADR-0091), `priority`. `render.mjs` thay biến `${EW_*}` từ file env và sinh standalone/ADC.

**Client service account** ([KEYCLOAK_SSO §3.2](../../docs/09-identity/KEYCLOAK_SSO.md), ADR-0074 mục 5): chỉ `svc-identity` (client credentials, secret `${env.KC_SVC_IDENTITY_SECRET}`), gán role `realm-management` tối thiểu (`manage-users`, `view-users`, `manage-organizations` [Proposed]) để module identity gọi Admin API. Không có client scope `internal-<module>` hay audience `svc-<module>-api`: module gọi nhau trong tiến trình qua facade, không dùng token.

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F001 (compose) · Tiêu thụ: F002 (danh sách miễn header, issuer/audience), F012/F013 (dùng client), F051 (theme), F130 (production).
- Không gọi facade, không event.

### Bảo mật, tenant, idempotency, audit

- Secret: `KC_GAME_BACKEND_SECRET`, `KC_PORTAL_CLIENT_SECRET` (ADR-0091, chỉ module identity đọc), `KC_SVC_<NAME>_SECRET`, `KC_SIS_SYNC_SECRET` qua env/Docker secrets; dev dùng `change-me-local`.
- Gateway không audit nghiệp vụ; log truy cập theo MONITORING §8 (bỏ query `/realtime/v1`, `X-Device-Id` rút gọn).
- Header `X-Tenant-Id` chỉ kiểm định dạng; backend kiểm khớp (F002).

### Quan sát

- Plugin `prometheus` (metric `apisix_http_status`, `apisix_http_latency`), `opentelemetry` (không thu body/header nhạy cảm), `request-id` sinh `X-Request-Id`; thuộc tính `tenant.id` khi có header.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
