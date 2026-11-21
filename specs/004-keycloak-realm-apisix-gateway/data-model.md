# Data Model: F004 — Keycloak realm as code và APISIX gateway (route, `tenant-header`, gateway nội bộ)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F004 không tạo bảng trong database của module EduWorld. Database `keycloak` do Keycloak tự quản lý (tạo ở F001); trạng thái APISIX nằm trong file Git (local) hoặc etcd (dev/staging/prod); bộ đếm rate limit trong Valkey là dữ liệu tạm (ADR-0004).

Cấu trúc cấu hình để tham chiếu khi code:

| Cấu trúc | Vị trí | Trường chính |
|---|---|---|
| Client Keycloak | `deploy/keycloak/realm-eduworld.json` → `clients[]` | `clientId`, `publicClient`, `directAccessGrantsEnabled`, `standardFlowEnabled`, `serviceAccountsEnabled`, `defaultClientScopes`, `optionalClientScopes`, `authenticationFlowBindingOverrides`, `attributes.login_theme`, `secret: ${env.KC_…}` |
| Client scope | `clientScopes[]` | `eduworld-api-audience`, `eduworld-roles`, `organization` (optional, không gán cho `eduworld-portal` — ADR-0091); không có scope `internal-<module>` (ADR-0074) |
| Client `eduworld-portal` (ADR-0091) | `clients[]` | `publicClient=false`, `secret: ${env.KC_PORTAL_CLIENT_SECRET}`, `standardFlowEnabled=false`, `implicitFlowEnabled=false`, `directAccessGrantsEnabled=true`, không `redirectUris`/`webOrigins`, `authenticationFlowBindingOverrides.direct_grant = eduworld-portal-direct-grant`, protocol mapper AMR + ánh xạ ACR→LoA |
| Flow `eduworld-portal-direct-grant` | `authenticationFlows[]` | `direct-grant-validate-username` → `direct-grant-validate-password` → allow-access-for-role (`tenant_staff`/`platform_staff`) (không có bước OTP — ADR-0094) |
| Theme action-token | `deploy/keycloak/themes/eduworld-ops/login/` | chỉ trang required action (`UPDATE_PASSWORD`; `CONFIGURE_TOTP` bỏ theo ADR-0094) và email; không gán browser flow cho portal |
| Realm role | `roles.realm[]` | `student`, `tenant_staff`, `platform_staff` |
| Organization (dev) | `realm-eduworld.dev.json` → `organizations[]` | alias `th-demo-a`, `pgd-demo`, `common`; attribute `tenant_id`, `tenant_type`, `login_code` (`DEMOA1`, `PGDDM2`) |
| Route APISIX | `deploy/apisix/routes/routes.yaml` | `id`, `uris`, `gateway`, `upstream` (`core-api`/`realtime`), `module`, `auth`, `tenant_header`, `rate`, `body_max`, `log_body`, `priority` |
| Khóa Valkey rate limit | do `limit-count` tạo, prefix `ew:apisix:rl:` (ACL user `ratelimit`) | khóa thiết bị, hash username, tenant, `sub`, toàn cục |

Đồng bộ với `platform.tenants.keycloak_org_id`/`keycloak_org_alias` (DATABASE_DESIGN §3.0) do module identity (F010) thực hiện; realm dev seed alias khớp seed identity của LOCAL_DEVELOPMENT §5.
