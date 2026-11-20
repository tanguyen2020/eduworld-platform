# Local Development

> Trạng thái: Draft · Phiên bản: 0.5 · Owner: DevOps Engineer · Cập nhật: 2026-11-17 (ADR-0092: mã nguồn trong `src/`, lệnh Go `go -C src/backend`; floci local chỉ nhận credential `test`/`test`, `sis-mock` ghim prism 4, Atlas chạy bằng image ghim, `make tools` cài vào `./bin`) · 2026-11-17 (modular monolith theo ADR-0074: compose 3 binary, `make dev`, biến DB theo module, bỏ URL/token gọi giữa service; floci chỉ local/CI — ADR-0075; CI GitHub Actions; bảng biến môi trường duy nhất, cổng internal/admin, tài khoản demo bổ sung, tenant QA)
> Liên quan: [DEPLOYMENT](DEPLOYMENT.md) · [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · [ADR-0088](../adr/ADR-0088-ci-cd-bang-github-actions.md) · [ADR-0024, ADR-0037…0042, ADR-0089](../adr/README.md) · [CONTRIBUTING](../../CONTRIBUTING.md) · [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md)

**Mục đích & phạm vi.** Nguồn duy nhất về môi trường local: Docker Compose, biến môi trường local, seed, Makefile, troubleshooting. Topology/sizing môi trường thật → [DEPLOYMENT](DEPLOYMENT.md).

Mục tiêu: developer mới chạy được toàn bộ stack (3 binary backend `core-api`, `realtime`, `worker` theo ADR-0074; Keycloak, APISIX, PostgreSQL, Valkey, NATS, Godot, web) trên máy cá nhân trong **≤ 30 phút** và đăng nhập bằng tài khoản học sinh giả. Khi làm một module domain, developer chạy hạ tầng bằng container và chạy binary chứa module đó native (thường là `core-api`, hoặc cả 3 bằng `make dev`).

> Nền tảng container: **Docker** (ADR-0024). Local/dev dùng Docker Compose; staging/production dùng Docker Swarm với stack file cùng cú pháp Compose (`deploy/swarm/<env>/`), nên service, biến môi trường, healthcheck và network được giữ giống nhau giữa các môi trường. Chưa dùng Kubernetes.

> Trạng thái hiện tại: Phase 0, repository mới có tài liệu. `deploy/compose/*`, `Makefile`, `.env.example`, `src/backend/go.mod` được tạo trong Phase 1 theo đặc tả dưới đây.

## 1. Yêu cầu công cụ

| Công cụ | Phiên bản | Ghi chú |
|---|---|---|
| Git | ≥ 2.40 | Bật `core.autocrlf=input` trên Windows |
| Docker Desktop / Docker Engine + Compose v2 | Compose ≥ 2.24 | RAM cấp cho Docker ≥ **8 GB** (3 binary + hạ tầng; trước đây 12 GB cho 6 service) |
| Go | Stable mới nhất (≥ 1.24) | Một `go.mod` duy nhất ở `src/backend/` (ADR-0074, [ADR-0092](../adr/ADR-0092-to-chuc-ma-nguon-trong-src.md)), gồm `src/backend/{cmd,modules,pkg,tools,test}`; không dùng `go.work`. Lệnh Go chạy trong `src/backend` (`go -C src/backend …`) hoặc qua `make` |
| ent CLI | ghim trong `tools/versions.mk`, `make tools` cài vào `./bin` | Sinh code ORM từ schema (ADR-0039) |
| Atlas CLI | image ghim `arigaio/atlas:0.38.0` (`tools/versions.mk`) | Versioned migrations từng module (ADR-0039). Không cần cài local: `make` chạy Atlas bằng container trên network compose `eduworld_default` (cần `make up` trước) |
| oapi-codegen | ghim trong `tools/versions.mk`, `make tools` cài vào `./bin` | Sinh server stub từ OpenAPI public và facade (interface + DTO trong `src/backend/modules/<module>/api/`) từ spec internal (ADR-0074 mục 3) |
| golangci-lint, depguard, go-arch-lint | ghim trong `tools/versions.mk`, `make tools` cài vào `./bin` | Lint + kiểm tra phụ thuộc tầng Clean Architecture (ADR-0038) và ranh giới module (`x-allowed-callers`, ADR-0074 mục 1) |
| Godot | 4.x (ghim trong `src/game/.godot-version`) | Bản "standard" (ADR-0014) |
| Node.js | LTS hiện hành + pnpm | Cho `src/cms/frontend/` và công cụ spec (Spectral, AsyncAPI, ajv) |
| make | GNU make | Bắt buộc. Windows: `winget install ezwinports.make` (hoặc Git Bash + make, WSL) |
| Postman CLI | mới nhất | Chạy API test collection (`specs/postman/`) |
| Tùy chọn | `k6`, `pgcli`, `valkey-cli`, `nats` CLI, Android SDK + JDK 17 | |

Hệ điều hành hỗ trợ: Windows 11 (khuyến nghị WSL2 cho backend), macOS, Linux.

## 2. Cấu trúc mã nguồn backend

Cây thư mục `src/backend/go.mod`, `src/backend/pkg/`, `src/backend/modules/<module>/`, `src/backend/cmd/{core-api,realtime,worker}` theo Clean Architecture: nguồn duy nhất ở [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) và [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md). Tài liệu này chỉ mô tả cách chạy chúng ở local.

## 3. Dịch vụ docker-compose

File: `deploy/compose/docker-compose.yml` (hạ tầng), `docker-compose.app.yml` (3 binary backend), `docker-compose.observability.yml`.

### 3.1. Hạ tầng

| Service | Image | Port host | Mục đích | Profile |
|---|---|---|---|---|
| `postgres` | `postgres:17` | 5432 | **Một container, nhiều database**: `eduworld_identity`, `eduworld_learning`, `eduworld_game`, `eduworld_competition`, `eduworld_chat`, `keycloak`. Script `deploy/postgres/init/*.sql` tạo database + role theo module `<module>_app/_platform/_readonly/_relay/_migrator/_partition_admin` (+ `chat_purger`, ADR-0089), mỗi role chỉ `CONNECT` database của module mình. Cùng topology với staging/production: một cụm, mỗi module một database (ADR-0074 mục 4) | default |
| `valkey` | `valkey/valkey:8` | 6379 | Cache, presence, queue matchmaking, rate limit, Pub/Sub realtime, seq chat. ACL user theo module (local có thể tắt) | default |
| `nats` | `nats:2.10-alpine` (`-js -sd /data -m 8222`) | 4222, 8222 | JetStream 1 node: stream `EVT_*`, `CHAT_MESSAGES`, `DLQ_*`; request/reply `ew.cmd.>` | default |
| `nats-box` | `natsio/nats-box` | — | CLI debug (`nats stream ls`, `nats consumer info ...`) | `tools` |
| `keycloak` | `quay.io/keycloak/keycloak:26.x` (`start-dev`) | 8180 | OIDC, import realm `eduworld` dev (gồm client `svc-identity` cho Keycloak Admin API) | default |
| `apisix` | `apache/apisix:3.13.0-debian` (standalone) | 9080, 9443 | Gateway **public** local: route render từ `deploy/apisix/routes/routes.yaml` vào `deploy/apisix/standalone/public/apisix.yaml` (`make apisix-render`), plugin Lua `ew-tenant-header`, `ew-login-key` mount từ `deploy/apisix/plugins` (F004) | default |
| `apisix-internal` | `apache/apisix:3.13.0-debian` (standalone) | 9180 | Gateway **internal** local: prefix nền tảng `/api/v1/admin`, `/sis`, `/game-admin`, `/game-media*` + route portal, `ip-restriction` theo `EW_ADMIN_CIDRS` (F004) | default |
| `mailpit` | `axllent/mailpit` | 8025, 1025 | Bắt email của Keycloak và email nghiệp vụ của module identity (`EW_SMTP_*`); xem tại `http://localhost:8025` | default |
| `floci` | `floci/floci:<release-tag>` (ghim tag release cụ thể; tra tag mới nhất trên GitHub Releases/Docker Hub, không commit `latest`) | 4566 | **S3 local giả lập bằng [floci](https://github.com/floci-io/floci)** (ADR-0044) — **chỉ local và CI**; staging/production dùng S3 managed tại Việt Nam hoặc Garage/SeaweedFS ([ADR-0075](../adr/ADR-0075-object-storage-production-khong-dung-floci.md)): `FLOCI_STORAGE_MODE=persistent`, `FLOCI_SERVICES_S3_ENFORCE_AUTH=true` (bắt buộc ký request như S3 thật; khi bật, floci chỉ chấp nhận credential tài khoản mặc định `test` / `test`, khóa khác bị `InvalidAccessKeyId`), volume `./data/floci:/app/data`. Cùng công cụ với staging/production (ADR-0071), khác cấu hình | default |
| `s3-init` | `floci/floci:<release-tag>-compat` (có AWS CLI; hoặc `amazon/aws-cli`) | — | Job một lần, credential `AWS_ACCESS_KEY_ID=test` / `AWS_SECRET_ACCESS_KEY=test` (chỉ local/CI): `aws --endpoint-url http://floci:4566 s3 mb s3://<bucket>` cho 5 bucket `ew-media-quarantine`, `ew-media-public`, `ew-media-private`, `ew-game-bundles`, `ew-exports` (không có bucket bài nộp của học sinh: ngoài version này, ADR-0082); bật versioning cho public/private/bundles bằng `s3api put-bucket-versioning`. Lifecycle 7 ngày của quarantine và bucket policy **không** giả lập ở local (floci chưa ghi rõ hỗ trợ) mà kiểm thử trên staging | default |
| `cdn` | `nginx:alpine` | 8090 | **CDN giả lập**: reverse proxy tới floci (path-style `http://floci:4566/<bucket>/<key>`), thêm `Cache-Control: immutable` cho đường dẫn có hash, kiểm tra chữ ký signed URL (HMAC đơn giản, cùng định dạng production) cho `ew-media-private` | default |
| `clamav` | `clamav/clamav:stable` | 3310 | Quét virus cho media worker; tắt được bằng `EW_MEDIA_SCAN=disabled` để khởi động nhanh (CI và staging luôn bật) | `media` |
| `sis-mock` | `stoplight/prism:4` (mock `specs/sis/eduworld-sis-contract.v1.yaml`; không dùng prism 5: lỗi khởi động `Cannot read properties of undefined (reading 'isPrimary')`) | 4010 | SIS giả lập cho provider `contract_v1` | default |
| `api-mock` | `stoplight/prism:4` (`--dynamic=false`, mock `specs/openapi/eduworld-api.v1.yaml`, chỉ trả `examples`; khởi động ~1 phút do spec lớn) | 4011 | **MOCK** API cho game client khi backend chưa sẵn sàng (F060, R17): game dùng `res://config/env/mock.json` (`is_mock: true`) và hiện nhãn "MOCK" trên mọi màn; bản phát hành không chứa cấu hình này. `make up-mock` | `mock` |

### 3.2. Service ứng dụng (profile `app`)

| Service compose | Build | Port HTTP public | Port admin host (`EW_ADMIN_ADDR=:9090` trong container) | Module nạp | Database |
|---|---|---|---|---|---|
| `core-api` | `src/backend/cmd/core-api` | 8100 | 9100 | `identity`, `learning`, `game`, `competition`, `chat` | 5 database module (mỗi module một pool, role riêng) |
| `realtime` | `src/backend/cmd/realtime` | 8106 (WebSocket) | 9106 | `realtime` | — (Valkey, NATS) |
| `worker` | `src/backend/cmd/worker --profile=all` (local gộp `events` + `chat` + `jobs` trong một tiến trình; staging/production tách 3 Swarm service, [DEPLOYMENT §3.1](DEPLOYMENT.md#31-binary-và-swarm-service-backend-adr-0074)) | — | 9200 | 5 module domain | 5 database module |
| `web` | Vite dev server (native) | 5173 | — | — | — |

Không có cổng internal API: module gọi nhau trong tiến trình qua facade (ADR-0074 mục 3).

### 3.3. Observability (profile `obs`)

`otel-collector` (4317/4318), `prometheus` (9095), `grafana` (3000, dashboard theo binary và module), `loki` (3100), `tempo` (3200).

**Luồng mạng local:** Godot/web → `http://localhost:9080` (APISIX) → `/api/v1/*` tới `core-api`, `/realtime/v1` tới `realtime` (DEPLOYMENT §5). `realtime` gửi lệnh tới `core-api` qua NATS request/reply; event đi qua NATS JetStream. Keycloak truy cập trực tiếp `http://localhost:8180`.

**Chạy lai (hybrid):** `make up-app EXCEPT=core-api` chạy `realtime` và `worker` trong container, developer chạy `core-api` native với `make run-core-api` (tương tự `EXCEPT=worker` + `make run-worker PROFILE=chat`). `make dev` chạy cả 3 binary native trong một lệnh (worker `--profile=all`), chỉ hạ tầng chạy container. APISIX standalone local đọc upstream từ biến `EW_UPSTREAM_CORE_API`, `EW_UPSTREAM_REALTIME`, mặc định trỏ container, đổi sang `host.docker.internal` khi chạy native.

## 4. Biến môi trường

Mục này là **bảng biến môi trường duy nhất** của mọi binary; [BACKEND_DESIGN §12](../07-backend/BACKEND_DESIGN.md) và [DEPLOYMENT](DEPLOYMENT.md) chỉ link về đây. Biến chung đặt ở `.env.example` gốc; biến riêng của từng module ở `src/backend/modules/<module>/.env.example` (commit) và `.env` (không commit); binary nạp module nào thì đọc biến của module đó. Giá trị ví dụ chỉ dùng cho local.

Quy ước tên: tiền tố `EW_`; kết nối database theo module `EW_DB_<MODULE>_*` (vì `core-api`/`worker` nạp nhiều module cùng lúc); object storage `EW_S3_*`; OpenTelemetry dùng biến chuẩn `OTEL_*`. Ở local, secret và chuỗi kết nối truyền giá trị trực tiếp; ở staging/production cùng biến đó đọc từ Docker secret qua biến có hậu tố `_FILE` (bảng dưới).

| Biến local (giá trị trực tiếp) | Biến staging/production (Docker secret) | Dùng cho |
|---|---|---|
| `EW_HTTP_ADDR` | như local (`:8100` cho `core-api`, `:8106` cho `realtime`) | API public / WebSocket; `worker` không mở cổng HTTP public |
| `EW_WORKER_PROFILE` | như local (`events`, `chat`, `jobs`; local `all`) | Nhóm việc của binary `worker`, tương đương cờ `--profile` (ADR-0074 mục 2) |
| `EW_ADMIN_ADDR` | như local, cố định `:9090` trong container | Một cổng quản trị: `/livez`, `/readyz`, `/healthz`, `/metrics`, `/version`; không publish, không route qua APISIX. Chạy native dùng cổng host 9100/9106/9200 ở §3.2 để không trùng |
| `EW_DB_<MODULE>_DSN` | `EW_DB_<MODULE>_URL_FILE` | Role `<module>_app` của từng module (không BYPASSRLS, không DELETE); write context và read context khi chưa có replica |
| `EW_DB_<MODULE>_READ_DSN` | `EW_DB_<MODULE>_READ_URL_FILE` | Read context của CQRS (query handler, `port/read`, ADR-0087): kết nối chỉ đọc tới replica; để trống → dùng primary |
| `EW_DB_<MODULE>_PLATFORM_DSN` | `EW_DB_<MODULE>_PLATFORM_URL_FILE` | Chỉ worker/API liên tenant của System Admin (BYPASSRLS, có audit) |
| `EW_DB_<MODULE>_RELAY_DSN` | `EW_DB_<MODULE>_RELAY_URL_FILE` | Chỉ outbox relay (profile `events`, role `<module>_relay`, ADR-0089) |
| `EW_DB_<MODULE>_MIGRATOR_DSN` | `EW_DB_<MODULE>_MIGRATOR_URL_FILE` | Chỉ job `migrate --module=<module>` (Atlas) |
| `EW_DB_CHAT_PURGER_DSN` | `EW_DB_CHAT_PURGER_URL_FILE` | Chỉ job purge chat gameplay trong profile `chat` (ADR-0033) |
| `EW_DB_<MODULE>_PARTITION_ADMIN_DSN` | `EW_DB_<MODULE>_PARTITION_ADMIN_URL_FILE` | Chỉ job `infra-partition-maintenance` (profile `jobs`, role `<module>_partition_admin`: chỉ `EXECUTE` hàm partition, F003) |
| `EW_VALKEY_<MODULE>_USER` / `EW_VALKEY_<MODULE>_PASSWORD` | như local, mật khẩu qua `EW_VALKEY_<MODULE>_PASSWORD_FILE` | ACL user Valkey của từng module; binary mở một client cho mỗi module nạp |
| `EW_KC_ADMIN_CLIENT_ID` / `EW_KC_ADMIN_CLIENT_SECRET` | `EW_KC_ADMIN_CLIENT_ID` / `EW_KC_ADMIN_CLIENT_SECRET_FILE` | Client `svc-identity`, chỉ để module identity gọi Keycloak Admin API (ADR-0074 mục 5); không có token gọi giữa module |
| `EW_S3_ACCESS_KEY` / `EW_S3_SECRET_KEY` | `EW_S3_ACCESS_KEY_FILE` / `EW_S3_SECRET_KEY_FILE` | Object storage (local/CI: floci, giá trị cố định `test` / `test`; staging/prod: backend theo ADR-0075) |
| `EW_SMTP_HOST`, `EW_SMTP_PORT`, `EW_SMTP_FROM`, `EW_SMTP_USERNAME`, `EW_SMTP_PASSWORD` | như local, mật khẩu qua `EW_SMTP_PASSWORD_FILE` | Gửi email nghiệp vụ của module identity (thông báo vụ việc an toàn cho nhà trường, khóa tài khoản, yêu cầu break-glass); local gửi vào `mailpit` (`localhost:1025`, không auth) |
| `EW_NODE_ID` | như local | Mã node realtime (định tuyến, presence); mặc định hostname container |
| `EW_WS_MAX_CONNECTIONS`, `EW_WS_HEARTBEAT`, `EW_WS_TIMEOUT` | như local | Giới hạn kết nối WebSocket mỗi replica và nhịp heartbeat/timeout của realtime; giá trị theo `ws_*` trong [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md) |
| `EW_WS_PING`, `EW_WS_MAX_FRAME`, `EW_WS_RESUME_WINDOW`, `EW_WS_RESUME_MAX`, `EW_WS_ACCEPT_RATE`, `EW_WS_FRAME_RATE`, `EW_WS_FRAME_BURST`, `EW_WS_MOVE_RATE`, `EW_WS_RATE_VIOLATIONS`, `EW_WS_PROTOCOL_ERRORS`, `EW_WS_INFLIGHT_COMMANDS`, `EW_WS_SEND_QUEUE`, `EW_WS_SEND_STALL`, `EW_WS_WRITE_TIMEOUT`, `EW_WS_DRAIN_WINDOW`, `EW_PRESENCE_TTL` | như local | Tham số còn lại của gateway realtime (F090); mặc định = `ws_*`, `resume_window`, `presence_ttl` của CONFIG_DEFAULTS, chỉ đặt khi cần khác |
| `EW_CMD_TIMEOUT` | như local | Timeout request/reply NATS `ew.cmd.*` từ `realtime` tới handler trong `core-api` |
| `EW_CORS_ALLOWED_ORIGINS`, `EW_REALTIME_PUBLIC_URL` | như local | Allowlist Origin của WebSocket (Web export) và URL WebSocket công khai trả trong `client-config` |
| `EW_APISIX_VALKEY_PASSWORD` | như local, từ Docker secret của gateway | Mật khẩu user ACL `apisix_ratelimit` cho plugin `ew-api-rate` (`api_rate_default`); local trống (Valkey không ACL) |
| `EW_APISIX_LOGIN_KEY_SECRET` | như local, từ Docker secret của gateway | Khóa HMAC của plugin `ew-login-key` (băm username chuẩn hóa làm khóa đếm giới hạn đăng nhập); biến môi trường của container APISIX, không nằm trong file route. Local mặc định `change-me-local` |
| Biến route gateway (`EW_UPSTREAM_CORE_API`, `EW_UPSTREAM_REALTIME`, `EW_OIDC_DISCOVERY`, `EW_PORTAL_HOST`, `EW_ADMIN_CIDRS`, `EW_RATE_*`, `EW_RATELIMIT_VALKEY_HOST`/`_PORT`, `EW_REALTIME_CONN_PER_IP`, `EW_TLS`, `EW_OTEL_COLLECTOR`) | `deploy/apisix/routes/env/<env>.env` của pipeline (mẫu `staging.env.example`) | Không phải biến runtime: `render.mjs` thay `${VAR}` trong `routes.yaml` khi render file standalone. `EW_ADMIN_CIDRS` là allowlist của gateway internal; ngưỡng `EW_RATE_*` theo [SECURITY_ARCHITECTURE §14](../10-security/SECURITY_ARCHITECTURE.md) (đăng nhập: đếm theo từng node gateway; `EW_RATE_API_*`, `EW_RATE_SESSION_PER_MIN`: đếm chung trong Valkey) |

```dotenv
# ---- Chung (gốc) ----
EW_ENV=local
EW_LOG_LEVEL=debug
EW_LOG_FORMAT=text                          # json ở dev/staging/prod
EW_VALKEY_ADDR=localhost:6379
EW_NATS_URLS=nats://localhost:4222
EW_OIDC_ISSUERS=http://localhost:8180/realms/eduworld
EW_OIDC_AUDIENCE=eduworld-api
EW_OIDC_ALLOWED_AZP=eduworld-game-backend,eduworld-portal
EW_OIDC_ALLOWED_CLOCK_SKEW=30s
EW_OIDC_PLATFORM_REQUIRED_AMR=otp                 # MFA platform: amr chứa otp (direct grant không nâng acr — research F004 R5)
EW_OIDC_PLATFORM_REQUIRED_ACR=                    # tùy chọn: acr chấp nhận thêm (trống = chỉ xét amr)
EW_OIDC_PLATFORM_MAX_AUTH_AGE=8h
EW_KC_PORTAL_CLIENT_ID=eduworld-portal            # azp của portal (guard portal_staff, F002)
EW_KC_GAME_CLIENT_ID=eduworld-game-backend        # azp của game (guard game_player, F002)
EW_GATEWAY_PUBLIC_BASE_URL=http://localhost:9080/api/v1   # tùy chọn: chỉ hiển thị ở màn Tích hợp (getIntegrationOverview, F059-2); trống = null
EW_GATEWAY_INTERNAL_BASE_URL=                     # tùy chọn: URL gateway nội bộ (không chứa user/password, query)
EW_SHUTDOWN_DRAIN_DELAY=0s                  # chờ LB thấy readyz 503 (mặc định 5s khi có HTTP public; local 0s)
EW_HTTP_SHUTDOWN_TIMEOUT=20s                # srv.Shutdown (BACKEND_DESIGN §13)
EW_WORKER_SHUTDOWN_TIMEOUT=60s              # chờ handler worker đang chạy
# ---- Object storage / CDN / media (ADR-0043, ADR-0044) ----
EW_S3_ENDPOINT=http://localhost:4566        # floci chỉ local/CI; staging/prod: endpoint S3 managed hoặc Garage/SeaweedFS (ADR-0075)
EW_S3_REGION=us-east-1                     # floci chấp nhận region bất kỳ; dùng giá trị chuẩn để SDK ký đúng
EW_S3_FORCE_PATH_STYLE=true               # bắt buộc với floci; staging/prod theo backend được chọn
EW_S3_ACCESS_KEY=test                     # chỉ local/CI: floci (ENFORCE_AUTH=true) chỉ nhận tài khoản mặc định test/test; staging/prod lấy từ Docker secrets (ADR-0075)
EW_S3_SECRET_KEY=test
EW_CDN_BASE_URL=http://localhost:8090
EW_CDN_SIGNING_KEY=change-me-local        # ký signed URL cho ew-media-private
EW_MEDIA_SCAN=clamd://localhost:3310      # hoặc disabled
EW_CONTENT_SNAPSHOT_TTL=168h
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318   # OTLP/HTTP (exporter otlptracehttp, F002)
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=1.0

# ---- Tích hợp bên ngoài (Keycloak Admin, SIS, object storage) — ADR-0074 mục 5 ----
# Không có URL/timeout/retry cho gọi giữa module: module gọi nhau trong tiến trình (src/backend/pkg/modcall)
EW_EXT_CLIENT_TIMEOUT=2s
EW_EXT_CLIENT_RETRY_MAX=2                   # chỉ thao tác idempotent
EW_EXT_CLIENT_CB_FAILURE_RATIO=0.5          # mở circuit breaker khi ≥ 50% lỗi trong cửa sổ
```

Ví dụ `src/backend/modules/chat/.env.example` (được `core-api` và `worker` đọc khi nạp module chat):

```dotenv
EW_DB_CHAT_DSN=postgres://chat_app:change-me-local@localhost:5432/eduworld_chat?sslmode=disable
EW_DB_CHAT_READ_DSN=                         # trống = đọc từ primary (local không có replica)
EW_DB_CHAT_PLATFORM_DSN=postgres://chat_platform:change-me-local@localhost:5432/eduworld_chat?sslmode=disable
EW_DB_CHAT_RELAY_DSN=postgres://chat_relay:change-me-local@localhost:5432/eduworld_chat?sslmode=disable
EW_DB_CHAT_MIGRATOR_DSN=postgres://chat_migrator:change-me-local@localhost:5432/eduworld_chat?sslmode=disable
EW_DB_CHAT_PURGER_DSN=postgres://chat_purger:change-me-local@localhost:5432/eduworld_chat?sslmode=disable   # chỉ job purge (ADR-0033)
EW_DB_CHAT_MAX_CONNS=20
EW_VALKEY_CHAT_USER=chat
EW_CHAT_STREAM=CHAT_MESSAGES
EW_CHAT_STREAM_REPLICAS=1                    # prod: 3
EW_CHAT_PUBLISH_TIMEOUT=2s
EW_CHAT_PERSIST_BATCH=500
EW_JOB_GAMEPLAY_CHAT_PURGE_CRON=*/15 * * * *
```

Ví dụ biến của binary (`src/backend/cmd/<binary>/.env.example`): `OTEL_SERVICE_NAME=core-api`, `EW_HTTP_ADDR=:8100`, `EW_ADMIN_ADDR=:9100` (chạy native; trong container luôn `:9090`), `EW_NATS_CREDS_FILE=` (local: không auth; staging/prod: creds của user NATS theo binary).

Biến riêng các module khác (rút gọn):

| Module | Biến đặc thù |
|---|---|
| identity | `EW_KC_ADMIN_BASE_URL=http://localhost:8180/admin/realms/eduworld` (Keycloak Admin API qua client `svc-identity`: `EW_KC_ADMIN_CLIENT_ID`/`EW_KC_ADMIN_CLIENT_SECRET` ở bảng trên; timeout/retry/circuit breaker theo `EW_EXT_CLIENT_*`), `EW_SIS_DEFAULT_PROVIDER=csv_import`, `EW_SIS_IMPORT_DIR=./deploy/seed/sis`, `EW_SIS_MOCK_BASE_URL=http://localhost:4010`, `EW_JOB_STUDENT_CLEANUP_CRON=*/10 * * * *`, `EW_JOB_STUDENT_CLEANUP_DRY_RUN=true`, `EW_GAME_MIN_CLIENT_VERSION=0.1.0` (dùng chung với module game, `min_client_version`), `EW_SMTP_HOST=localhost`, `EW_SMTP_PORT=1025`, `EW_SMTP_FROM=no-reply@eduworld.local`, `EW_PORTAL_BASE_URL=http://localhost:5173` (link đặt lại mật khẩu portal), `EW_KC_PORTAL_CLIENT_SECRET_FILE` (client confidential `eduworld-portal`, ADR-0091), `EW_KC_GAME_CLIENT_ID=eduworld-game-backend` + `EW_KC_GAME_CLIENT_SECRET_FILE` (client confidential đăng nhập game, ADR-0068), `EW_KC_TOKEN_TIMEOUT=3s` (`keycloak_token_call_timeout`), `EW_GAME_LOGIN_TICKET_KEY_FILE` (base64 32 byte, AES-256-GCM của `login_ticket`), `EW_GAME_LOGIN_TICKET_TTL=2m` (`game_login_ticket_ttl`), `EW_LOGIN_IP_HASH_KEY_FILE` (HMAC `login_events.ip_hash`), `EW_PLATFORM_CURFEW=21:30-06:30` (`platform_curfew` tới khi F015 đọc platform settings), `EW_CURFEW_WARNING_MINUTES=10`, `EW_PORTAL_MENU_CATALOG` (đường dẫn `specs/portal/menu-catalog.yaml` cho `cmd/migrate`, mặc định theo repo), mặc định cài đặt khi chưa cấu hình: `EW_STUDENT_CLEANUP_DEFAULT_DAYS=180`, `EW_STUDENT_CLEANUP_NOTICE_DAYS=14`, `EW_STUDENT_CLEANUP_APPROVAL_PERCENT=10`, `EW_GAMEPLAY_CHAT_RETENTION_DAYS=7`, `EW_GAMEPLAY_CHAT_RETENTION_MIN_DAYS=3`, `EW_EPHEMERAL_CHAT_RETENTION_HOURS=24`, `EW_PLATFORM_TIMEZONE=Asia/Ho_Chi_Minh` |
| learning | `EW_S3_ENDPOINT`, `EW_S3_BUCKET_MEDIA`, `EW_S3_ACCESS_KEY/SECRET_KEY`, `EW_MATH_MAX_LATEX_LENGTH=2000` (ADR-0041), `EW_MATH_MAX_SVG_BYTES=262144` (256 KB), `EW_MATH_MAX_DEPTH=20`, `EW_MATH_ALLOWED_EXTENSIONS=ams,mhchem,color` (ADR-0055) |
| game | `EW_GAME_SESSION_HEARTBEAT_TIMEOUT=45s` (`game_session_heartbeat_timeout`), `EW_GAME_SESSION_TIMEOUT_SCAN=15s`, `EW_RT_TICKET_TTL=60s` (`realtime_ticket_ttl`), `EW_WS_TICKET_RATE=10` (`ws_ticket_rate`/phút/học sinh), `EW_REALTIME_PUBLIC_URL=ws://localhost:9080/realtime/v1` (`url` của `createRealtimeTicket`; trống → 503, F090), `EW_GAME_MIN_CLIENT_VERSION=0.1.0` (dùng chung module identity), `EW_GAME_LATEST_CLIENT_VERSION` (mặc định = tối thiểu), `EW_GAME_MAINTENANCE=false` + `EW_GAME_MAINTENANCE_MESSAGE`/`EW_GAME_MAINTENANCE_UNTIL` (RFC 3339), `EW_PLATFORM_CURFEW=21:30-06:30` (`platform_curfew`: chỉ là mặc định ban đầu; sau `platform_settings.changed` module game dùng projection `ref.platform_settings` — OQ F014-2), `EW_GAME_LOGIN_URL=http://localhost:9080/api/v1/auth/game/login` (`login.login_url` của client-config), `EW_GAME_REMEMBER_USERNAME_DEFAULT=true`, `EW_SUPPORTED_LOCALES=vi-VN,en-US`, `EW_GAME_IP_HASH_KEY_FILE` (HMAC `game.login_events.ip_hash`, ≥ 32 ký tự) — F014 |
| competition | `EW_MM_TICK=500ms` (matchmaker trong profile `jobs`), `EW_BATTLE_ROOM_CHECKPOINT=5s`, `EW_HUB_TICK_HZ=10` (`hub_sync_rate`), `EW_HUB_INSTANCE_CAPACITY=40` (`hub_instance_capacity`), `EW_HUB_IDLE_CLOSE=5m` (`hub_instance_idle_close`), `EW_HUB_DISCONNECT_GRACE=30s` (`disconnect_grace` hub), `EW_INSTANCE_ID` (id owner trong lease room, mặc định hostname) — room host hub của module competition chạy trong `core-api` (F091); chỉ ghi đè cho môi trường/load test, giá trị gốc ở CONFIG_DEFAULTS |
| chat | `EW_SOCIAL_MAX_PENDING_REQUESTS=10`, `EW_SOCIAL_MAX_REQUESTS_PER_DAY=20`, `EW_SOCIAL_MAX_FRIENDS=100` (`friend_limits`), `EW_SOCIAL_FRIEND_REQUEST_TTL=168h` (`friend_request_ttl`), `EW_SOCIAL_FRIEND_CODE_TTL=10m` (`friend_code_ttl`), `EW_PRESENCE_TTL=90s` (`presence_ttl`, dùng chung gateway), `EW_SOCIAL_EXPIRY_INTERVAL=1m` (job `expire-friend-requests`), `EW_PLATFORM_TIMEZONE=Asia/Ho_Chi_Minh` (ngày của giới hạn lời mời/ngày) — F095; route `/social/*` và fan-out presence chạy trong `core-api`, consumer projection trong `worker --profile=events`, job trong `--profile=jobs`; chỉ ghi đè cho môi trường/test, giá trị gốc ở CONFIG_DEFAULTS |
| realtime | `EW_CORS_ALLOWED_ORIGINS=http://localhost:5173` |


Biến của composition root `core-api`/`worker` (nối binary, OQ F010-2 — giá trị local trong `deploy/compose/docker-compose.app.yml`, anchor `x-module-env`; thiếu biến bắt buộc thì binary dừng và báo **tên** biến):

| Biến | Binary | Dùng cho |
|---|---|---|
| `EW_DB_<MODULE>_PLATFORM_DSN`, `EW_DB_<MODULE>_RELAY_DSN` (bắt buộc), `EW_DB_<MODULE>_READ_DSN` (tùy chọn) | `core-api`, `worker` | Pool theo role của `pkg/module.Registry.SQL` (bảng trên); relay chỉ mở ở profile `events`/`all` |
| `EW_DB_IDENTITY_PARTITION_ADMIN_DSN` | `worker` (`jobs`/`all`) | Job `partition-maintainer` của module identity (`audit.audit_logs`, `identity.login_events`) |
| `EW_OPENAPI_SPEC` | `core-api` | Đường dẫn `specs/openapi/eduworld-api.v1.yaml` để validate request (image: `/app/specs/openapi/…`) |
| `EW_PORTAL_MENU_CATALOG` | `core-api`, `worker` | Danh mục menu portal (workspace của `/me/permissions`, featuregate) — cũng dùng cho `cmd/migrate` |
| `EW_PAGE_CURSOR_KEY` / `_FILE` | `core-api`, `worker` | Khóa HMAC cursor phân trang (≥ 32 byte, `pkg/page`), giống nhau ở mọi replica |
| `EW_PRINCIPAL_CACHE_TTL` (60s), `EW_PRINCIPAL_CACHE_L1_TTL` (15s), `EW_PRINCIPAL_CACHE_L1_SIZE` (10000) | `core-api` | Cache Principal (`principal_cache`, CONFIG_DEFAULTS) |
| `EW_FEATURE_TOGGLE_CACHE_TTL` (60s) | `core-api` | Cache featuregate (`feature_toggle_cache_ttl`) |
| `EW_OIDC_JWKS_REFRESH` (10m), `EW_OIDC_JWKS_MIN_UNKNOWN_KID_INTERVAL` (1m) | `core-api` | Làm mới JWKS theo nền và khi gặp `kid` lạ [Confirmed] |
| `EW_NATS_STREAM_REPLICAS` (1) | `worker` | Số replica của `EVT_*`/`DLQ_*`/`IDENTITY_SIS_JOBS` khi worker tạo stream (prod: 3) |
| `EW_VALKEY_USERNAME`, `EW_VALKEY_PASSWORD` / `_FILE` | `core-api`, `worker` | Một client Valkey cho mỗi binary tới khi có ACL user theo module [Confirmed] |
| `EW_S3_TIMEOUT` | `core-api`, `worker` | Timeout mỗi request object storage (mặc định adapter) |
| `EW_SIS_IMPORT_BUCKET` (`ew-sis-imports`) | `core-api`, `worker` | Bucket file CSV SIS (F011) |
| `EW_SIS_SECRETS_DIR` | `core-api`, `worker` | Thư mục secret SIS `<dir>/<secret_ref>` (OQ F017-3; compose mount `deploy/compose/sis-secrets.example`, đổi bằng `EW_SIS_SECRETS_HOST_DIR`) |
| `EW_SIS_ALLOW_HTTP_BASE_URL`, `EW_SIS_EGRESS_PROXY_URL`, `EW_SIS_ID_HASH_KEY` | `core-api`, `worker` | Client `contract_v1` (http chỉ cho `sis-mock`), proxy egress, khóa băm định danh khi khớp chuyển trường |
| `EW_CLAMAV_ADDR` | `worker` | clamd quét file CSV SIS (compose profile `media`); learning dùng `EW_MEDIA_SCAN` |
| `EW_MEDIA_WORK_DIR` | `worker` | Thư mục tạm của media worker (mặc định thư mục tạm của hệ điều hành) |

Issuer trong container là `http://keycloak:8180/realms/eduworld`: token chỉ được cấp qua backend (`/auth/game/login`, `/auth/portal/login`), nên mọi token mà `core-api` kiểm đều mang issuer đó; chạy `core-api` native thì đặt `EW_OIDC_ISSUERS=http://localhost:8180/realms/eduworld` và `EW_KC_ADMIN_BASE_URL=http://localhost:8180/admin/realms/eduworld`. `make up-app` sinh `EW_GAME_LOGIN_TICKET_KEY` ngẫu nhiên mỗi lần khởi động nếu shell chưa đặt.

Giá trị nghiệp vụ (cleanup_days, retention_days…) nằm trong bảng settings của module sở hữu, không nằm trong env; giá trị mặc định tra ở [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md).

Godot đọc cấu hình môi trường từ `res://config/env/<env>.json` (sinh lúc build theo `--env`; không chứa secret, không có khóa OIDC — ADR-0068); phần còn lại lấy từ `GET /game/client-config`. Ví dụ `res://config/env/local.json`:

```json
{
  "env": "local",
  "api_base_url": "http://localhost:9080/api/v1",
  "realtime_url": "ws://localhost:9080/realtime/v1",
  "cdn_base_url": "http://localhost:8090"
}
```

## 5. Dữ liệu seed

Toàn bộ dữ liệu seed là **giả**, không dùng thông tin học sinh thật.

| Seed | Vị trí | Nội dung |
|---|---|---|
| Realm Keycloak | `deploy/keycloak/realm-eduworld.dev.json` | Realm `eduworld`; client `eduworld-game-backend` (confidential, chỉ Direct Access Grants, secret local `change-me-local` chỉ dùng trong `core-api` cho module identity — ADR-0068), `eduworld-portal` (confidential, chỉ Direct Access Grants, flow `eduworld-portal-direct-grant` với OTP có điều kiện cho `platform_staff`, secret local `change-me-local` chỉ dùng trong `core-api` cho module identity — ADR-0091; theme `eduworld-ops` chỉ cho trang action-token đăng ký OTP; tài khoản dev platform có OTP seed sẵn), `eduworld-api`; client `svc-identity` (chỉ Keycloak Admin API, secret local `change-me-local` — ADR-0074 mục 5) và client provisioning của module identity; realm roles; mỗi tenant seed là một Keycloak Organization: `th-demo-a` (trường `DEMOA1`), `pgd-demo` (2 trường `DEMOB1`, `DEMOC1`), `common`, `qa-probe` (tenant QA `is_qa = true`, chỉ chứa tài khoản synthetic) |
| module identity | `src/backend/modules/identity/seed/` (`dev.yaml` + `sis/<mã trường>/{classes,students,enrollments}.csv`, cột = tên trường DTO của EduWorld SIS Contract v1) | System Admin bootstrap `sa.admin` + nhân sự platform; tenant `th-demo-a` (trường `DEMOA1`), `pgd-demo` (trường `DEMOB1`, `DEMOC1`); 1 `sis_connection` `csv_import` mỗi tenant; lớp/năm học/học sinh/enrollment qua import CSV; nhân sự trường/đơn vị + `user_roles`; khóa `hs.locked`. Tenant chung `common` + trường ảo "EduWorld chung" có sẵn từ migration |
| module learning | `src/backend/modules/learning/seed/` (theo `specs/schemas`) | Curriculum `GDPT2018_SAMPLE` (GDPT 2018 Toán lớp 1–5 rút gọn, file `seed/curriculum/gdpt2018_math_g1_5.yaml`; publish được dù thiếu `yccd_ref`), ~200 câu hỏi, assignment nội dung platform cho 3 tenant (ADR-0029) |
| module game | `src/backend/modules/game/seed/` | 3 quest, 2 mini-game config, item cosmetic, reward rule |
| module competition | `src/backend/modules/competition/seed/` | Cấu hình matchmaking/ranking mặc định, 1 season, tournament template |
| module chat | `src/backend/modules/chat/seed/` | Chat policy mặc định platform, quick messages, danh mục từ nhạy cảm placeholder, 1 sticker pack mẫu |

Projection `ref_*` (tenant, học sinh, lớp) ở các module được nạp **qua event** sau khi module identity sync SIS. Không seed trực tiếp vào bảng `ref_*`, để kiểm thử luôn đường event thật.

Tài khoản demo (mật khẩu chung `Demo@12345`, **chỉ local**):

| Username | Vai trò | Tenant / trường / lớp |
|---|---|---|
| `hs.an.3a`, `hs.binh.3a` | student | `th-demo-a` / TH Demo A / 3A |
| `hs.chi.3b` | student | `th-demo-a` / 3B |
| `hs.dung.7a` | student | `pgd-demo` / THCS Demo B / 7A |
| `hs.giang.6a`, `hs.hoa.6a` | student | `pgd-demo` / THCS Demo B / 6A; trận đội, party `competition_team` lớp 6–7, ghép ±1 khối với `hs.dung.7a` |
| `hs.khanh.8a`, `hs.lan.9a` | student | `pgd-demo` / THCS Demo B / 8A, 9A; leaderboard `g6_9` (top 50 + ±3 quanh mình), bonus tốc độ từ lớp 6 |
| `hs.trung.a1`, `hs.trung.a2` | student | `th-demo-a` / 3A; hai tài khoản trùng một học sinh để thử gộp tài khoản |
| `hs.qa.probe` | student synthetic | `qa-probe` (tenant QA `is_qa = true`); probe đăng nhập định kỳ, smoke; bị loại khỏi mọi analytics và dashboard |
| `hs.em.4a` | student | `pgd-demo` / TH Demo C / 4A, thử chuyển trường trong cùng tenant `unit` |
| `hs.tudo` | student tenant chung (do platform tạo, khối 4) | Vào thẳng `common`, lớp ảo `common-g4` (ADR-0063) |
| `hs.locked` | student bị khóa | `th-demo-a` |
| `hs.nosis` | Keycloak user không thuộc tenant chung lẫn tenant SIS nào | `ACCOUNT_NOT_PROVISIONED`, phiên Keycloak bị thu hồi (ADR-0063) |
| `hs.lau.khongvao` / `hs.sapbidon` | student | Ứng viên dọn / "sắp bị dọn" (ADR-0032) |
| `ad.truong.a` | school_admin | `th-demo-a` / TH Demo A; giao bài, thử thách lớp, School Learning Dashboard, nhận thông báo vụ việc an toàn |
| `ad.truong.b` | school_admin (trường thứ hai) | `pgd-demo` / THCS Demo B; dùng cho test cô lập: không thấy dữ liệu TH Demo A (khác tenant) và TH Demo C (cùng tenant `unit`, khác trường) |
| `qlcm.truong.a` | academic_manager (2 tenant) | `th-demo-a` (scope môn Toán) và `pgd-demo`; thử chọn tenant sau đăng nhập portal, scope `subject`, `GAME_ACCESS_STUDENT_ONLY` |
| `ad.donvi` | tenant_admin | `pgd-demo` |
| `bt.noidung.dv`, `bt.duyet.dv` | tenant_content_editor, tenant_content_reviewer | `pgd-demo`; lane duyệt `tenant`: soạn và duyệt nội dung riêng tenant, reviewer khác tác giả, không thấy task lane platform hay tenant khác |
| `cm.noidung`, `cm.duyet` | content_admin, content_reviewer | Platform (không có tenant gốc); `cm.duyet` cũng duyệt nội dung game (`game.content.review`) |
| `gd.thietke` | game_designer | Platform; Game Operations Console: soạn quest, reward rule, sticker/GIF, bản dịch game; không publish, không điều chỉnh kinh tế |
| `lo.vanhanh` | liveops | Platform; publish/rollback nội dung game, Event Builder, season track, báo cáo game analytics |
| `tc.thidau` | competition_admin | Platform; cấu hình matchmaking, ranking, season, phần thưởng thi đấu (`competition.config.*`) |
| `sa.admin` | system_admin | Platform (không có tenant gốc), liên tenant; MFA: OTP seed sẵn trong realm import (mã 6 số: `totp()` của `deploy/keycloak/scripts/totp.mjs` với secret trong `dev-constants.mjs`); System Admin bootstrap của seed identity |
| `sa.admin2` | system_admin thứ hai, kiêm `game.economy.adjust` | Platform; người duyệt thứ hai cho điều chỉnh kinh tế (tạo bởi `sa.admin`) và yêu cầu break-glass (người duyệt khác người tạo) |
| `sm.antoan` | safety_moderator | Platform (không có tenant gốc); hàng đợi moderation mọi tenant, đọc tin nhắn chỉ trong case (`tenancy.Bypass(ctx, "moderation_case")`); MFA như mọi role platform (tắt ở local) — ADR-0072 |
| `sm.truongca` | safety_moderator + `moderation.case.lead` | Platform; trưởng ca Safety Team (phân case, lịch trực) |

Mọi tài khoản không phải học sinh có `user_type = staff` (ADR-0081): tài khoản trường/đơn vị có `primary_tenant_id` và chỉ có role tenant; tài khoản platform (`cm.*`, `gd.*`, `lo.*`, `tc.*`, `sa.*`, `sm.*`) có `primary_tenant_id = NULL` và chỉ có role platform. Seed vi phạm bất biến này bị trigger `user_roles_scope_chk` chặn. Không có tài khoản giáo viên: Release 1.0 không có vai trò giáo viên (ADR-0072).

Kịch bản tenant ở local (đăng nhập username/password, không chọn trường — ADR-0063): `hs.an.3a` → vào thẳng `th-demo-a`; `qlcm.truong.a` đăng nhập portal → hộp chọn 2 tenant sau đăng nhập, mở game → `GAME_ACCESS_STUDENT_ONLY`; `ad.truong.b` gọi dữ liệu của `th-demo-a` → 404; `hs.tudo` → tenant chung; `hs.nosis` → `ACCOUNT_NOT_PROVISIONED`; thoát rồi mở lại game trên máy dev (thiết bị cá nhân) → tự vào lại bằng refresh token, không hiện form (ADR-0086); chạy với `device.cfg` `shared_device: true` hoặc bản Web → phải nhập lại mật khẩu; bấm Đăng xuất → token bị xóa, mở lại phải đăng nhập; `sa.admin` thấy cả 4 tenant với dropdown lọc (`qa-probe` gắn nhãn "QA"). Thử một thiết bị: chạy hai instance Godot với `--device-id` khác nhau cùng tài khoản.

Lệnh seed: `make seed` = import realm (`make keycloak-import`) → `make migrate` (mọi module) → seed từng module: `make seed-identity` (`go -C src/backend run ./modules/identity/cmd/seed`: tạo System Admin đầu tiên `sa.admin` qua use case bootstrap, rồi thay mặt `sa.admin` gọi các use case `createTenant`, `createAdminTenantSchool`, `createSisConnection`, import CSV của SIS bằng đúng pipeline `SisRun` của job import, `ProvisionStudentAccount`, `createTenantUser`, `grantTenantUserRole`, `lockTenantUser`; tài khoản Keycloak **không** được tạo mới mà gắn với user cùng username của realm import — `keycloak.DevAdopter`, chỉ khi `EW_ENV=local`; chạy lại không thay đổi gì) → `make seed-learning` → `make seed-game` (`go -C src/backend run ./modules/game/cmd/seed`: danh mục tạo nhân vật v1 và item mặc định qua use case publish dưới PlatformScope, F070; chạy lại không tạo version mới) → sau `make up-app`, `make seed-check` chờ worker phát event và projection `ref.students` của learning, game có đủ học sinh đã seed (không cần `make sis-sync`: import CSV chạy đồng bộ trong seed). Chưa seed [Confirmed]: `qa-probe`/`hs.qa.probe` (`createTenant` chưa đặt được `is_qa`), `hs.tudo` (`createCommonStudents` sinh mật khẩu mới), `sa.moi`; `hs.nosis` cố ý không có tài khoản; `qlcm.truong.a` giữ `academic_manager` toàn tenant (chưa có scope môn vì môn học thuộc module learning).

## 6. Chạy từng phần

### 6.1. Hạ tầng và toàn bộ backend
```bash
cp .env.example .env && make env           # sao chép .env.example gốc, của từng module và từng binary
make up                                     # hạ tầng: postgres valkey nats keycloak apisix apisix-internal mailpit floci s3-init sis-mock
make migrate                                # atlas migrate apply cho 5 database module
make seed
make up-app                                 # 3 binary core-api, realtime, worker (--profile=all) trong container
```

### 6.2. Phát triển một module (native)
```bash
make up-app EXCEPT=core-api                 # realtime + worker chạy container
make gen MOD=learning                       # ent + oapi-codegen (server stub public + facade api/ từ spec internal)
make run-core-api                           # go -C src/backend run ./cmd/core-api (nạp cả 5 module domain)
make run-worker PROFILE=events              # khi sửa consumer/job: go -C src/backend run ./cmd/worker --profile=events
# hoặc: make dev                            # cả 3 binary native, worker --profile=all
```
Kiểm tra: `curl http://localhost:9100/readyz` (trạng thái từng module trong body); qua gateway: `curl http://localhost:9080/api/v1/game/client-config`.

### 6.3. Godot
1. Mở `src/game/project.godot` bằng Godot 4.x đúng phiên bản ghim (`src/game/.godot-version`); lần đầu chạy `make godot-import`. Mọi target Godot đọc binary từ biến `GODOT` (mặc định `godot`), ví dụ `make godot-test GODOT="/c/.../Godot_v4.7.2-stable_win64_console.exe"`.
2. Chọn profile môi trường `local` (`-- --env=local`; chỉ build debug đọc `--env`). Chưa có backend: `make up-mock` (Prism profile `mock`, cổng 4011) rồi chạy với `-- --env=mock` — mọi màn có nhãn "MOCK" (R17).
3. Chạy `res://scenes/bootstrap/Boot.tscn` (scene chính) → Splash; từ F061: form đăng nhập trong game (S01) → nhập `hs.an.3a` + mật khẩu seed (game gọi `/api/v1/auth/game/login`, ADR-0068). `res://scenes/ui/debug/UiGallery.tscn` (chỉ build debug) xem component ở 4 density, vi/en/`xx-PS`, tương phản cao.
4. Kiểm tra: `make godot-lint` (gdtoolkit `gdlint`, typed check, lint theme/lớp/signal, khóa dịch, fixture, không Admin UI), `make godot-test` (GdUnit4 headless + quét artifact), `make godot-smoke`, `make godot-screenshots` (cần renderer), `make godot-export-check` (PCK phát hành không cần export template). Đổi chuỗi UI hoặc spec: `make godot-fixtures`.
5. `SecureStorage` (ADR-0086): adapter DPAPI/Keystore cần plugin native build bởi CI (`godot.yml` job `platform-windows`, `platform-android`); checkout chưa build plugin thì game dùng `MemoryOnly` (phải đăng nhập lại mỗi lần mở).

### 6.4. Web CMS/LMS
```bash
cd src/cms/frontend && pnpm install
pnpm gen:api      # sinh type TS (schema.gen.d.ts) + metadata operation (operation-meta.gen.ts) từ specs/openapi/eduworld-api.v1.yaml và danh mục menu (src/generated/menu-catalog.ts, = make portal-catalog / pnpm gen:catalog) từ specs/portal/menu-catalog.yaml; pnpm gen:check báo lệch
pnpm dev          # http://localhost:5173 — Admin Portal (src/cms/frontend/apps/portal), mọi workspace; gọi APISIX local (public + internal cùng máy)
                  # MathJax 4 (mathjax@4.x) + MathLive (mathlive) ghim phiên bản cố định trong src/cms/frontend/apps/portal/package.json (ADR-0055)
pnpm storybook    # http://localhost:6006 — ops-ui + component khung portal (F051) + theme Ant Design (sáng/tối/compact)
pnpm lint && pnpm test   # ESLint + stylelint (chặn hardcode màu/font/khoảng cách, import game, URL môi trường, chuỗi UI literal) · Vitest 1 worker
```
Cấu hình runtime của portal: `/config.json` (`apiBaseUrl`, `apiInternalBaseUrl`, `env`, `principalCacheTtlSeconds` = `principal_cache_ttl`, `sessionIdleTimeoutSeconds` = idle của `portal_session_sso` — CONFIG_DEFAULTS) sinh lúc deploy; không có khóa OIDC vì portal đăng nhập bằng form riêng qua `/api/v1/auth/portal/*` (ADR-0091, F051). Khi dev, Vite phục vụ `apps/portal/public/config.json` nếu có (gitignore), nếu không thì `public/config.example.json` (gateway public `:9080`, nội bộ `:9180`); khi `/config.json` không đọc được (chỉ ở dev) thì dùng biến `VITE_API_BASE_URL`, `VITE_API_INTERNAL_BASE_URL`, `VITE_APP_ENV`, `VITE_PRINCIPAL_CACHE_TTL_SECONDS`, `VITE_SESSION_IDLE_TIMEOUT_SECONDS`. Cookie refresh/CSRF của phiên portal là first-party khi API phục vụ cùng origin với portal (APISIX, [Confirmed] — F004). Trình duyệt Playwright cho axe/visual: `pnpm exec playwright install --only-shell chromium`.

### 6.5. API test bằng Postman CLI
```bash
postman collection run specs/postman/eduworld-api.postman_collection.json \
  -e specs/postman/env.local.postman_environment.json
```

## 7. Makefile targets đề xuất

| Target | Mô tả |
|---|---|
| `make tools` | Cài công cụ Go ghim version trong `tools/versions.mk` (ent, oapi-codegen, golangci-lint, go-arch-lint, govulncheck) vào `./bin` và kéo image Atlas `arigaio/atlas:0.38.0` |
| `make up` / `make down` / `make reset` | Khởi động / dừng / xóa volume hạ tầng |
| `make keycloak-import` / `make keycloak-check` / `make keycloak-test` | Áp realm `eduworld` bằng `keycloak-config-cli` + post-apply (local thêm user/organization dev) / kiểm tĩnh realm as code / kiểm hành vi realm trên Keycloak đang chạy (F004) |
| `make apisix-render [ENV=local]` / `make apisix-check` / `make apisix-smoke` | Render file standalone hai gateway từ `routes.yaml` / kiểm file render mới, route ↔ spec gộp, không log body, không hardcode / smoke trên stack đang chạy (F004) |
| `make up-app [EXCEPT=<binary>]` | Chạy 3 binary `core-api`, `realtime`, `worker` trong container (trừ binary chỉ định) |
| `make up-obs` | Bật stack observability |
| `make run-core-api` / `make run-realtime` / `make run-worker [PROFILE=events\|chat\|jobs\|all]` | Chạy native một binary (`go -C src/backend run ./cmd/<binary>`) |
| `make dev` | Chạy native cả 3 binary trong một lệnh (worker `--profile=all`), hạ tầng chạy container |
| `make seed-identity` / `make seed-learning` / `make seed-game` | Chạy riêng seed của một module (idempotent) |
| `make gen [MOD=<module>]` | `go generate` ent + oapi-codegen (server stub public, facade `src/backend/modules/<module>/api/` từ spec internal) cho một hoặc mọi module |
| `make gen-check` | Báo lỗi nếu code sinh lệch với schema/spec (dùng trong CI GitHub Actions — workflow theo path filter ở [DEPLOYMENT §10](DEPLOYMENT.md#10-cicd-github-actions); mọi target `make` ở bảng này là lệnh CI gọi, chạy được y hệt trên máy dev) |
| `make migrate-<module>` / `make migrate` | `atlas migrate apply` cho một / mọi module |
| `make migrate-diff MOD=<module> name=<x>` | Atlas sinh migration mới từ ent schema |
| `make migrate-lint MOD=<module>` | `atlas migrate lint` (chặn thao tác phá hủy, string thiếu giới hạn) |
| `make seed` / `make seed-check` | Nạp dữ liệu giả / kiểm tra projection đã đồng bộ |
| `make lint [MOD=<module>]` | gofmt, golangci-lint, go-arch-lint (tầng Clean Architecture), gdlint, eslint, spectral |
| `make arch-check` | Ranh giới module trên toàn repository: không import `internal/` của module khác, chỉ import `src/backend/modules/<khác>/api` khi có trong `x-allowed-callers`, `src/backend/cmd/` chỉ nối module qua `Register` (ADR-0074 mục 1) |
| `make test [MOD=<module>]` | Unit test |
| `make test-integration [MOD=<module>]` | Integration (testcontainers: PostgreSQL, NATS, Valkey) |
| `make test-contract` | Facade test: module đích hiện thực đúng interface sinh từ spec internal (lỗi ánh xạ sang `problem`, `maxLength`, ngữ cảnh `src/backend/pkg/modcall`), kèm test của mọi module trong `x-allowed-callers` dùng facade đó; chạy trong tiến trình, không HTTP |
| `make test-e2e` | E2E toàn hệ thống trên compose |
| `make api-test` | Postman CLI collection môi trường local |
| `make godot-lint` / `make godot-test` / `make godot-smoke` / `make godot-screenshots` / `make godot-export-check` / `make godot-export-<windows\|android>` (biến `GODOT=<binary>`) | Godot headless: lint, GdUnit4, smoke boot, ảnh UiGallery so baseline, export + quét artifact (F060) |
| `make up-mock` | Prism MOCK API cho game (profile `mock`, cổng 4011) |
| `make design-tokens` / `make design-tokens-check` | Style Dictionary sinh Theme Godot, `UiTokens.gd`, resource 3D, theme Ant Design + CSS variables từ `design/`; bản `-check` báo lỗi nếu file sinh lệch nguồn (ADR-0047) |
| `make storybook` | Chạy Storybook của `src/cms/frontend/packages/ops-ui` |
| `make web-lint` / `make web-test` / `make web-build` | `src/cms/frontend`: ESLint + stylelint + typecheck + `gen:check` / Vitest 1 worker / build portal + kiểm 5 chunk workspace (F050) |
| `make web-storybook-check` / `make web-visual` | Build Storybook + axe mọi story (lỗi serious/critical fail) / Playwright visual regression 100% và 200% zoom (F050) |
| `make docs-check` | `npm ci --prefix scripts` rồi chạy `node scripts/docs/check-mermaid.mjs` (cú pháp mọi sơ đồ Mermaid trong `docs/` + README) và `node scripts/docs/check-links.mjs` (link tương đối + anchor kiểu GitHub), `node scripts/docs/build-rtm.mjs --check` (ma trận truy vết khớp nguồn). Exit ≠ 0 khi có lỗi; cùng lệnh với job CI `docs:check`. Sinh lại RTM: `node scripts/docs/build-rtm.mjs` |
| `make render-thumbnails [ASSET=<path>]` | Godot headless render thumbnail/turntable cho asset 3D, upload lên floci local |
| `make specs-lint` | Spectral (+ rule `maxLength`), AsyncAPI, ajv JSON Schema, oasdiff, `check-enums`, `check-problem-codes`, `check-portal-catalog` (danh mục menu/action portal ↔ `x-portal-action`, ADR-0090) |
| `make portal-catalog` | Sinh `src/cms/frontend/apps/portal/src/generated/menu-catalog.ts` (`menuRegistry`/`workspaceRegistry`) từ `specs/portal/menu-catalog.yaml`; `make gen` sinh bảng route → action cho `featuregate` từ `x-portal-action` (ADR-0090) |
| `make docs-lint` | markdownlint + kiểm tra link |

## 8. Troubleshooting

| Triệu chứng | Nguyên nhân thường gặp | Cách xử lý |
|---|---|---|
| 401 `invalid issuer` | `iss` là `http://keycloak:8080/...` khác `EW_OIDC_ISSUERS` | Đặt `KC_HOSTNAME=http://localhost:8180`; client luôn gọi Keycloak qua `localhost:8180` |
| Build hoặc `make arch-check` báo `use of internal package` / import không được phép | Module import `internal/` của module khác, hoặc gọi facade mà module chưa có trong `x-allowed-callers` | Gọi qua `src/backend/modules/<khác>/api`; nếu lời gọi hợp lệ, thêm module vào `x-allowed-callers` của operation trong spec internal rồi `make gen` |
| `core-api` không lên, log `missing DSN for module <x>` | Thiếu `EW_DB_<MODULE>_DSN` của một module | `make env`; kiểm tra `.env` của module đó |
| Dữ liệu tenant/học sinh thiếu ở learning/game/chat | Projection `ref_*` chưa nhận event (NATS chưa chạy, `worker` chưa chạy hoặc consumer lỗi) | `make seed-check`; `nats consumer info EVT_IDENTITY <module>-ref`; xem bảng `processed_events` |
| Godot đăng nhập báo lỗi `503`/`UNAVAILABLE` | module identity không gọi được Keycloak (sai `EW_KC_GAME_CLIENT_SECRET_FILE` hoặc Keycloak chưa sẵn sàng) | Kiểm tra secret `kc_game_backend` và log module identity (log không có body đăng nhập) |
| Mọi request game trả `400 TENANT_HEADER_REQUIRED` | Client chưa gắn `X-Tenant-Id` sau bootstrap (ADR-0065) | Kiểm tra `AuthService.tenant_id` được gán từ response `/auth/game/login` |
| `TENANT_NOT_SELECTED` / `TENANT_MISMATCH` | Thuộc nhiều tenant chưa chọn, chọn sai mã, hoặc membership chưa sync | Chọn tenant; kiểm tra tenant `th-demo-a`/`pgd-demo` (trường `DEMOA1`/`DEMOB1`); chạy lại `make seed-identity` |
| `atlas migrate apply` lỗi `permission denied` | Dùng DSN app thay vì migrator, hoặc sai database | Dùng `EW_DB_<MODULE>_MIGRATOR_DSN` của đúng module |
| Query trả rỗng dù có dữ liệu | Thiếu tenant trong context → global filter ent + RLS không trả dòng nào (đúng thiết kế) | Kiểm tra middleware tenant; test phải tạo context có `TenantScope` |
| Testcontainers lỗi trên Windows | Docker Desktop chưa bật WSL integration, hoặc container Ryuk không khởi động được trên Docker Desktop for Windows | Chạy test trong WSL2, hoặc đặt `TESTCONTAINERS_RYUK_DISABLED=true` (chỉ máy dev; tự dọn container còn sót bằng `docker ps`) |
| `make: command not found` trên Windows | Chưa cài GNU make | `winget install ezwinports.make` hoặc dùng Git Bash có make / WSL |
| `sis-mock` thoát ngay, log `Cannot read properties of undefined (reading 'isPrimary')` | Dùng image prism 5 | Ghim `stoplight/prism:4` |
| Upload/`s3-init` lỗi `InvalidAccessKeyId` | floci bật `FLOCI_SERVICES_S3_ENFORCE_AUTH=true` chỉ nhận credential mặc định | Đặt `EW_S3_ACCESS_KEY=test`, `EW_S3_SECRET_KEY=test` (và `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY` của `s3-init`), chỉ local/CI |
| `make migrate`/`migrate-lint` không kết nối được PostgreSQL | Atlas chạy trong container trên network `eduworld_default`, hạ tầng chưa chạy | `make up` trước; kiểm tra `docker network ls` có `eduworld_default` |
| APISIX 404 `Route Not Found` | File standalone cũ hoặc chưa reload; prefix nội bộ gọi vào gateway public (đúng thiết kế) | `make apisix-render` rồi `docker compose restart apisix apisix-internal`; prefix `/api/v1/admin`, `/sis`, `/game-admin`, `/game-media*` gọi qua `:9180` |
| APISIX 400 `TENANT_HEADER_REQUIRED` | Route cần `X-Tenant-Id` (plugin `ew-tenant-header`) | Gửi header UUID tenant đang chọn; `/me/tenants`, `/auth/session` không cần |
| WebSocket đóng ngay với mã 4401 | Ticket hết hạn (60 s) hoặc đã dùng | Xin ticket mới mỗi lần kết nối |
| Lệnh WS timeout | `core-api` chưa chạy nên không có handler subscribe `ew.cmd.<module>.>` | Chạy `core-api`; `nats sub 'ew.cmd.>'` trong `nats-box` để xem |
| Gửi chat báo `CHAT_TEMPORARILY_UNAVAILABLE` | `nats` chưa chạy hoặc stream chưa tạo | `docker compose up -d nats`; chạy `core-api` một lần để tạo stream |
| Tin gửi được nhưng lịch sử thiếu | `chat-persister` (profile `chat` của `worker`) không chạy | `nats consumer info CHAT_MESSAGES persister`; chạy `make run-worker PROFILE=chat` |
| Ký tự tiếng Việt lỗi trong CSV seed | Encoding không phải UTF-8 | Lưu UTF-8 không BOM |
