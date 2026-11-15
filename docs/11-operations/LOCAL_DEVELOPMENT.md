# Local Development

> Trạng thái: Draft · Phiên bản: 0.3 · Owner: DevOps Engineer · Cập nhật: 2026-11-15
> Liên quan: [DEPLOYMENT](DEPLOYMENT.md) · [ADR-0024, ADR-0036…0042](../adr/README.md) · [CONTRIBUTING](../../CONTRIBUTING.md) · [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md)

**Mục đích & phạm vi.** Nguồn duy nhất về môi trường local: Docker Compose, biến môi trường local, seed, Makefile, troubleshooting. Topology/sizing môi trường thật → [DEPLOYMENT](DEPLOYMENT.md).

Mục tiêu: developer mới chạy được toàn bộ stack (6 microservice, Keycloak, APISIX, PostgreSQL, Valkey, NATS, Godot, web) trên máy cá nhân trong **≤ 30 phút** và đăng nhập bằng tài khoản học sinh giả. Khi chỉ làm một service, developer chạy hạ tầng + service đó native, các service còn lại chạy bằng container.

> Nền tảng container: **Docker** (ADR-0024). Local/dev dùng Docker Compose; staging/production dùng Docker Swarm với stack file cùng cú pháp Compose (`deploy/swarm/<env>/`), nên service, biến môi trường, healthcheck và network được giữ giống nhau giữa các môi trường. Chưa dùng Kubernetes.

> Trạng thái hiện tại: Phase 0, repository mới có tài liệu. `deploy/compose/*`, `Makefile`, `.env.example`, `go.work` được tạo trong Phase 1 theo đặc tả dưới đây.

## 1. Yêu cầu công cụ

| Công cụ | Phiên bản | Ghi chú |
|---|---|---|
| Git | ≥ 2.40 | Bật `core.autocrlf=input` trên Windows |
| Docker Desktop / Docker Engine + Compose v2 | Compose ≥ 2.24 | RAM cấp cho Docker ≥ **12 GB** (6 service + hạ tầng) |
| Go | Stable mới nhất (≥ 1.24) | Dùng `go.work` gom các module `services/*` và `pkg/` |
| ent CLI | ghim trong `tools/tools.go` | Sinh code ORM từ schema (ADR-0039) |
| Atlas CLI | ghim version | Versioned migrations từng service (ADR-0039) |
| oapi-codegen | ghim version | Sinh server stub + typed client từ OpenAPI public/internal (ADR-0040) |
| golangci-lint, go-arch-lint | ghim version | Lint + kiểm tra phụ thuộc tầng Clean Architecture (ADR-0038) |
| Godot | 4.x (ghim trong `godot/.godot-version`) | Bản "standard" (ADR-0014) |
| Node.js | LTS hiện hành + pnpm | Cho `web/` và công cụ spec (Spectral, AsyncAPI, ajv) |
| make | GNU make (Windows: Git Bash/WSL) | |
| Postman CLI | mới nhất | Chạy API test collection (`specs/postman/`) |
| Tùy chọn | `k6`, `pgcli`, `valkey-cli`, `nats` CLI, Android SDK + JDK 17 | |

Hệ điều hành hỗ trợ: Windows 11 (khuyến nghị WSL2 cho backend), macOS, Linux.

## 2. Cấu trúc mã nguồn backend

Cây thư mục `go.work`, `pkg/`, `services/<service>/` theo Clean Architecture: nguồn duy nhất ở [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) và [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md). Tài liệu này chỉ mô tả cách chạy chúng ở local.

## 3. Dịch vụ docker-compose

File: `deploy/compose/docker-compose.yml` (hạ tầng), `docker-compose.services.yml` (6 service), `docker-compose.observability.yml`.

### 3.1. Hạ tầng

| Service | Image | Port host | Mục đích | Profile |
|---|---|---|---|---|
| `postgres` | `postgres:17` | 5432 | **Một container, nhiều database**: `eduworld_identity`, `eduworld_learning`, `eduworld_game`, `eduworld_competition`, `eduworld_chat`, `keycloak`. Script `deploy/postgres/init/*.sql` tạo database + role `<svc>_app/_platform/_migrator` (+ `chat_purger`), mỗi role chỉ `CONNECT` database của mình | default |
| `valkey` | `valkey/valkey:8` | 6379 | Cache, presence, queue matchmaking, rate limit, Pub/Sub realtime, seq chat. ACL user theo service (local có thể tắt) | default |
| `nats` | `nats:2.10-alpine` (`-js -sd /data -m 8222`) | 4222, 8222 | JetStream 1 node: stream `EVT_*`, `CHAT_MESSAGES`, `DLQ_*`; request/reply `ew.cmd.>` | default |
| `nats-box` | `natsio/nats-box` | — | CLI debug (`nats stream ls`, `nats consumer info ...`) | `tools` |
| `keycloak` | `quay.io/keycloak/keycloak:26.x` (`start-dev`) | 8180 | OIDC, import realm `eduworld` dev (gồm client `svc-*`) | default |
| `apisix` | `apache/apisix:3.x` (standalone) | 9080, 9443 | Gateway local, upstream theo service | default |
| `mailpit` | `axllent/mailpit` | 8025, 1025 | Bắt email Keycloak | default |
| `floci` | `floci/floci:<release-tag>` (ghim tag release cụ thể; tra tag mới nhất trên GitHub Releases/Docker Hub, không commit `latest`) | 4566 | **S3 local giả lập bằng [floci](https://github.com/floci-io/floci)** (ADR-0044): `FLOCI_STORAGE_MODE=persistent`, `FLOCI_SERVICES_S3_ENFORCE_AUTH=true` (bắt buộc ký request như S3 thật), volume `./data/floci:/app/data`. Cùng công cụ với staging/production (ADR-0071), khác cấu hình | default |
| `s3-init` | `floci/floci:<release-tag>-compat` (có AWS CLI; hoặc `amazon/aws-cli`) | — | Job một lần: `aws --endpoint-url http://floci:4566 s3 mb s3://<bucket>` cho 6 bucket `ew-media-quarantine`, `ew-media-public`, `ew-media-private`, `ew-game-bundles`, `ew-exports`, `ew-student-submissions`; bật versioning cho public/private/bundles bằng `s3api put-bucket-versioning`. Lifecycle 7 ngày của quarantine và bucket policy **không** giả lập ở local (floci chưa ghi rõ hỗ trợ) mà kiểm thử trên staging | default |
| `cdn` | `nginx:alpine` | 8090 | **CDN giả lập**: reverse proxy tới floci (path-style `http://floci:4566/<bucket>/<key>`), thêm `Cache-Control: immutable` cho đường dẫn có hash, kiểm tra chữ ký signed URL (HMAC đơn giản, cùng định dạng production) cho `ew-media-private` | default |
| `clamav` | `clamav/clamav:stable` | 3310 | Quét virus cho media worker; tắt được bằng `EW_MEDIA_SCAN=disabled` để khởi động nhanh (CI và staging luôn bật) | `media` |
| `sis-mock` | `stoplight/prism` (mock `specs/sis/eduworld-sis-contract.v1.yaml`) | 4010 | SIS giả lập cho provider `contract_v1` | default |

### 3.2. Service ứng dụng (profile `app`)

| Service | Build | Port HTTP | Port admin (`/readyz`, `/metrics`) | Database |
|---|---|---|---|---|
| `identity-service` (+ `identity-worker`) | `services/identity-service` | 8101 | 9101 / 9201 | `eduworld_identity` |
| `learning-service` (+ `learning-worker`) | `services/learning-service` | 8102 | 9102 / 9202 | `eduworld_learning` |
| `game-service` (+ `game-worker`) | `services/game-service` | 8103 | 9103 / 9203 | `eduworld_game` |
| `competition-service` (+ `competition-worker`) | `services/competition-service` | 8104 | 9104 / 9204 | `eduworld_competition` |
| `chat-service` (+ `chat-worker`) | `services/chat-service` | 8105 | 9105 / 9205 | `eduworld_chat` |
| `realtime-gateway` | `services/realtime-gateway` | 8106 (WebSocket) | 9106 | — |
| `web` | Vite dev server (native) | 5173 | — | — |

### 3.3. Observability (profile `obs`)

`otel-collector` (4317/4318), `prometheus` (9095), `grafana` (3000, dashboard theo service), `loki` (3100), `tempo` (3200).

**Luồng mạng local:** Godot/web → `http://localhost:9080` (APISIX) → upstream theo prefix (DEPLOYMENT §5). Service gọi nhau qua `EW_SVC_<NAME>_URL` (container: `http://learning-service:8102`; native: `http://localhost:8102`). Keycloak truy cập trực tiếp `http://localhost:8180`.

**Chạy lai (hybrid):** `make up-app EXCEPT=chat-service` chạy mọi service trong container trừ service đang phát triển; developer chạy service đó native với `make svc-chat`. APISIX standalone local đọc upstream từ biến `EW_UPSTREAM_<SVC>`, mặc định trỏ container, đổi sang `host.docker.internal` khi chạy native.

## 4. Biến môi trường

Mỗi service có `services/<svc>/.env.example` (commit) và `.env` (không commit). Biến chung đặt ở `.env.example` gốc. Giá trị chỉ dùng cho local.

```dotenv
# ---- Chung (gốc) ----
EW_ENV=local
EW_LOG_LEVEL=debug
EW_LOG_FORMAT=text                          # json ở dev/staging/prod
EW_VALKEY_ADDR=localhost:6379
EW_NATS_URLS=nats://localhost:4222
EW_OIDC_ISSUERS=http://localhost:8180/realms/eduworld
EW_OIDC_AUDIENCE=eduworld-api
# ---- Object storage / CDN / media (ADR-0043, ADR-0044) ----
EW_S3_ENDPOINT=http://localhost:4566        # floci ở mọi môi trường (ADR-0071); staging/prod: endpoint floci nội bộ
EW_S3_REGION=us-east-1                     # floci chấp nhận region bất kỳ; dùng giá trị chuẩn để SDK ký đúng
EW_S3_FORCE_PATH_STYLE=true               # bắt buộc với floci (mọi môi trường)
EW_S3_ACCESS_KEY=change-me-local          # chỉ local (floci chấp nhận giá trị bất kỳ khác rỗng); staging/prod lấy từ Docker secrets
EW_S3_SECRET_KEY=change-me-local
EW_CDN_BASE_URL=http://localhost:8090
EW_CDN_SIGNING_KEY=change-me-local        # ký signed URL cho ew-media-private
EW_MEDIA_SCAN=clamd://localhost:3310      # hoặc disabled
EW_CONTENT_SNAPSHOT_TTL=168h
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=1.0

# ---- URL service khác (typed client, ADR-0040) ----
EW_SVC_IDENTITY_URL=http://localhost:8101
EW_SVC_LEARNING_URL=http://localhost:8102
EW_SVC_GAME_URL=http://localhost:8103
EW_SVC_COMPETITION_URL=http://localhost:8104
EW_SVC_CHAT_URL=http://localhost:8105
EW_SVC_CLIENT_TIMEOUT=2s
EW_SVC_CLIENT_RETRY_MAX=2
EW_SVC_CLIENT_CB_FAILURE_RATIO=0.5          # mở circuit breaker khi ≥ 50% lỗi trong cửa sổ
```

Ví dụ `services/chat-service/.env.example`:

```dotenv
OTEL_SERVICE_NAME=chat-service
EW_HTTP_ADDR=:8105
EW_ADMIN_ADDR=:9105
EW_DB_DSN=postgres://chat_app:change-me-local@localhost:5432/eduworld_chat?sslmode=disable
EW_DB_PLATFORM_DSN=postgres://chat_platform:change-me-local@localhost:5432/eduworld_chat?sslmode=disable
EW_DB_MIGRATOR_DSN=postgres://chat_migrator:change-me-local@localhost:5432/eduworld_chat?sslmode=disable
EW_DB_PURGER_DSN=postgres://chat_purger:change-me-local@localhost:5432/eduworld_chat?sslmode=disable   # chỉ job purge (ADR-0033)
EW_DB_MAX_CONNS=20
EW_VALKEY_USER=chat
EW_NATS_CREDS_FILE=                          # local: không auth; staging/prod: creds của user `chat`
EW_SVC_CLIENT_ID=svc-chat                    # Keycloak client credentials (ADR-0040)
EW_SVC_CLIENT_SECRET=change-me-local
EW_CHAT_STREAM=CHAT_MESSAGES
EW_CHAT_STREAM_REPLICAS=1                    # prod: 3
EW_CHAT_PUBLISH_TIMEOUT=2s
EW_CHAT_PERSIST_BATCH=500
EW_JOB_GAMEPLAY_CHAT_PURGE_CRON=*/15 * * * *
```

Biến riêng các service khác (rút gọn):

| Service | Biến đặc thù |
|---|---|
| identity-service | `EW_KEYCLOAK_ADMIN_URL`, `EW_KEYCLOAK_PROVISIONING_CLIENT_ID/SECRET`, `EW_SIS_DEFAULT_PROVIDER=csv_import`, `EW_SIS_IMPORT_DIR=./deploy/seed/sis`, `EW_SIS_MOCK_BASE_URL=http://localhost:4010`, `EW_JOB_STUDENT_CLEANUP_CRON=*/10 * * * *`, `EW_JOB_STUDENT_CLEANUP_DRY_RUN=true` |
| learning-service | `EW_S3_ENDPOINT`, `EW_S3_BUCKET_MEDIA`, `EW_S3_ACCESS_KEY/SECRET_KEY`, `EW_MATH_MAX_LATEX_LENGTH=2000` (ADR-0041), `EW_MATH_MAX_SVG_BYTES=262144` (256 KB), `EW_MATH_MAX_DEPTH=20`, `EW_MATH_ALLOWED_EXTENSIONS=ams,mhchem,color` (ADR-0055) |
| game-service | `EW_GAME_SESSION_HEARTBEAT_TIMEOUT=45s`, `EW_RT_TICKET_TTL=60s` |
| competition-service | `EW_MM_TICK=500ms`, `EW_BATTLE_ROOM_CHECKPOINT=5s` |
| realtime-gateway | `EW_RT_HUB_TICK_HZ=10`, `EW_RT_HUB_INSTANCE_CAPACITY=40`, `EW_CORS_ALLOWED_ORIGINS=http://localhost:5173` |

Giá trị nghiệp vụ (cleanup_days, retention_days…) nằm trong bảng settings của service sở hữu, không nằm trong env.

Godot đọc cấu hình từ `godot/config/env/local.cfg` (không chứa secret):

```ini
[api]
base_url="http://localhost:9080/api/v1"
realtime_url="ws://localhost:9080/realtime/v1"
# Đăng nhập game qua backend (ADR-0068): game không có cấu hình OIDC/client secret
login_path="/api/v1/auth/game/login"
```

## 5. Dữ liệu seed

Toàn bộ dữ liệu seed là **giả**, không dùng thông tin học sinh thật.

| Seed | Vị trí | Nội dung |
|---|---|---|
| Realm Keycloak | `deploy/keycloak/realm-eduworld.dev.json` | Realm `eduworld`; client `eduworld-game-backend` (confidential, chỉ Direct Access Grants, secret local `change-me-local` chỉ mount vào identity-service — ADR-0068), `eduworld-portal` (theme `eduworld-ops`, conditional OTP cho `platform_staff`; tài khoản dev platform có OTP seed sẵn), `eduworld-api`; client service `svc-identity`, `svc-learning`, `svc-game`, `svc-competition`, `svc-chat`, `svc-realtime` (secret local `change-me-local`) và client provisioning của identity-service; realm roles; mỗi tenant seed là một Keycloak Organization: `th-demo-a` (mã `DEMOA1`), `pgd-demo` (mã `PGDDM2`, 2 trường), `common` |
| identity-service | `services/identity-service/seed/` | `tenants`, School ảo "EduWorld chung", 2 `sis_connections` `csv_import`; SIS CSV trong `deploy/seed/sis/*.csv` (cột = tên trường DTO của EduWorld SIS Contract v1) |
| learning-service | `services/learning-service/seed/` (theo `specs/schemas`) | Khung GDPT 2018 Toán lớp 1–5 rút gọn, ~200 câu hỏi, assignment nội dung platform cho 3 tenant (ADR-0029) |
| game-service | `services/game-service/seed/` | 3 quest, 2 mini-game config, item cosmetic, reward rule |
| competition-service | `services/competition-service/seed/` | Cấu hình matchmaking/ranking mặc định, 1 season, tournament template |
| chat-service | `services/chat-service/seed/` | Chat policy mặc định platform, quick messages, danh mục từ nhạy cảm placeholder, 1 sticker pack mẫu |

Projection `ref_*` (tenant, học sinh, lớp) ở các service được nạp **qua event** sau khi identity-service sync SIS. Không seed trực tiếp vào bảng `ref_*`, để kiểm thử luôn đường event thật.

Tài khoản demo (mật khẩu chung `Demo@12345`, **chỉ local**):

| Username | Vai trò | Tenant / trường / lớp |
|---|---|---|
| `hs.an.3a`, `hs.binh.3a` | student | `th-demo-a` / TH Demo A / 3A |
| `hs.chi.3b` | student | `th-demo-a` / 3B |
| `hs.dung.7a` | student | `pgd-demo` / THCS Demo B / 7A |
| `hs.em.4a` | student | `pgd-demo` / TH Demo C / 4A, thử chuyển trường trong cùng tenant `unit` |
| `hs.tudo` | student tenant chung (do platform tạo, khối 4) | Vào thẳng `common`, lớp ảo `common-g4` (ADR-0063) |
| `hs.locked` | student bị khóa | `th-demo-a` |
| `hs.nosis` | Keycloak user không thuộc tenant chung lẫn tenant SIS nào | `ACCOUNT_NOT_PROVISIONED`, phiên Keycloak bị thu hồi (ADR-0063) |
| `hs.lau.khongvao` / `hs.sapbidon` | student | Ứng viên dọn / "sắp bị dọn" (ADR-0032) |
| `gv.lan` | teacher | `th-demo-a`, chủ nhiệm 3A |
| `gv.minh` | teacher (2 tenant) | `th-demo-a` và `pgd-demo`; thử `TENANT_NOT_SELECTED`, `GAME_ACCESS_STUDENT_ONLY` |
| `ad.truong.a` | school_admin | `th-demo-a` |
| `ad.donvi` | tenant_admin | `pgd-demo` |
| `cm.noidung`, `cm.duyet` | content_admin, content_reviewer | Platform |
| `sa.admin` | system_admin | Platform, liên tenant (MFA tắt ở local) |

Kịch bản tenant ở local (đăng nhập username/password, không chọn trường — ADR-0063): `hs.an.3a` → vào thẳng `th-demo-a`; `gv.minh` → `TENANT_NOT_SELECTED` với 2 lựa chọn (chọn sau đăng nhập, ở portal); `hs.tudo` → tenant chung; `hs.nosis` → `ACCOUNT_NOT_PROVISIONED`; thoát rồi mở lại game → phải nhập lại mật khẩu (ADR-0064); `sa.admin` thấy cả 3 tenant với dropdown lọc. Thử một thiết bị: chạy hai instance Godot với `--device-id` khác nhau cùng tài khoản.

Lệnh seed: `make seed` = import realm → `make migrate` (mọi service) → seed từng service → `identity-worker sis-sync --once` → chờ projection `ref_*` ở các service đủ (lệnh kiểm tra `make seed-check`).

## 6. Chạy từng phần

### 6.1. Hạ tầng và toàn bộ service
```bash
cp .env.example .env && make env           # sao chép .env.example của từng service
make up                                     # hạ tầng: postgres valkey nats keycloak apisix mailpit floci s3-init sis-mock
make migrate                                # atlas migrate apply cho 5 service
make seed
make up-app                                 # 6 service + worker trong container
```

### 6.2. Phát triển một service (native)
```bash
make up-app EXCEPT=learning-service         # các service khác chạy container
make gen SVC=learning-service               # ent + oapi-codegen (server + client của service nó gọi)
make svc-learning                           # go run ./services/learning-service/cmd/api
make svc-learning-worker                    # go run ./services/learning-service/cmd/worker
```
Kiểm tra: `curl http://localhost:9102/readyz`; qua gateway: `curl http://localhost:9080/api/v1/game/client-config`.

### 6.3. Godot
1. Mở `godot/project.godot` bằng Godot 4.x đúng phiên bản ghim.
2. Chọn profile môi trường `local` (`-- --env=local`).
3. Chạy `res://scenes/bootstrap/bootstrap.tscn` → form đăng nhập trong game (S01) → nhập `hs.an.3a` + mật khẩu seed (game gọi `/api/v1/auth/game/login`, ADR-0068).
4. Test: `godot --headless --path godot -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests`.

### 6.4. Web CMS/LMS
```bash
cd web && pnpm install
pnpm gen:api      # sinh client TS từ specs/openapi (bundle public)
pnpm dev          # http://localhost:5173 — Admin Portal (web/apps/portal), mọi workspace; gọi APISIX local (public + internal cùng máy)
                  # MathJax 4 (mathjax@4.x) + MathLive (mathlive) ghim phiên bản cố định trong web/apps/portal/package.json (ADR-0055)
pnpm storybook    # http://localhost:6006 — ops-ui + theme Ant Design (sáng/tối/compact)
```

### 6.5. API test bằng Postman CLI
```bash
postman collection run specs/postman/eduworld-api.postman_collection.json \
  -e specs/postman/env.local.postman_environment.json
```

## 7. Makefile targets đề xuất

| Target | Mô tả |
|---|---|
| `make tools` | Cài công cụ ghim version (ent, atlas, oapi-codegen, golangci-lint, go-arch-lint) |
| `make up` / `make down` / `make reset` | Khởi động / dừng / xóa volume hạ tầng |
| `make up-app [EXCEPT=<svc>]` | Chạy 6 service + worker trong container (trừ service chỉ định) |
| `make up-obs` | Bật stack observability |
| `make svc-<name>` / `make svc-<name>-worker` | Chạy native một service (`svc-identity`, `svc-learning`, `svc-game`, `svc-competition`, `svc-chat`, `svc-realtime`) |
| `make gen [SVC=<svc>]` | `go generate` ent + oapi-codegen (server stub, typed client internal) cho một hoặc mọi service |
| `make gen-check` | Báo lỗi nếu code sinh lệch với schema/spec (dùng trong CI) |
| `make migrate-<svc>` / `make migrate` | `atlas migrate apply` cho một / mọi service |
| `make migrate-diff SVC=<svc> name=<x>` | Atlas sinh migration mới từ ent schema |
| `make migrate-lint SVC=<svc>` | `atlas migrate lint` (chặn thao tác phá hủy, string thiếu giới hạn) |
| `make seed` / `make seed-check` | Nạp dữ liệu giả / kiểm tra projection đã đồng bộ |
| `make lint [SVC=<svc>]` | gofmt, golangci-lint, go-arch-lint (tầng Clean Architecture), gdlint, eslint, spectral |
| `make test [SVC=<svc>]` | Unit test |
| `make test-integration [SVC=<svc>]` | Integration (testcontainers: PostgreSQL, NATS, Valkey) |
| `make test-contract` | Contract test consumer-driven giữa service |
| `make test-e2e` | E2E liên service trên compose |
| `make api-test` | Postman CLI collection môi trường local |
| `make godot-test` / `make godot-export-<target>` | Godot headless |
| `make design-tokens` / `make design-tokens-check` | Style Dictionary sinh Theme Godot, `UiTokens.gd`, resource 3D, theme Ant Design + CSS variables từ `design/`; bản `-check` báo lỗi nếu file sinh lệch nguồn (ADR-0047) |
| `make storybook` | Chạy Storybook của `web/packages/ops-ui` |
| `make docs-check` | `npm ci --prefix scripts` rồi chạy `node scripts/docs/check-mermaid.mjs` (cú pháp mọi sơ đồ Mermaid trong `docs/` + README) và `node scripts/docs/check-links.mjs` (link tương đối + anchor kiểu GitHub), `node scripts/docs/build-rtm.mjs --check` (ma trận truy vết khớp nguồn). Exit ≠ 0 khi có lỗi; cùng lệnh với job CI `docs:check`. Sinh lại RTM: `node scripts/docs/build-rtm.mjs` |
| `make render-thumbnails [ASSET=<path>]` | Godot headless render thumbnail/turntable cho asset 3D, upload lên floci local |
| `make specs-lint` | Spectral (+ rule `maxLength`), AsyncAPI, ajv JSON Schema, oasdiff |
| `make docs-lint` | markdownlint + kiểm tra link |

## 8. Troubleshooting

| Triệu chứng | Nguyên nhân thường gặp | Cách xử lý |
|---|---|---|
| 401 `invalid issuer` | `iss` là `http://keycloak:8080/...` khác `EW_OIDC_ISSUERS` | Đặt `KC_HOSTNAME=http://localhost:8180`; client luôn gọi Keycloak qua `localhost:8180` |
| Service A gọi service B bị 401 | Sai `EW_SVC_CLIENT_ID/SECRET` hoặc `azp` của A chưa có trong allowlist endpoint internal của B | Kiểm tra realm dev có client `svc-*`; xem log authz của B |
| Service gọi nhau báo `circuit open` | Service đích chưa chạy hoặc sai `EW_SVC_<NAME>_URL` (container vs native) | `make up-app`; dùng `localhost:<port>` khi chạy native, tên service khi trong container |
| Dữ liệu tenant/học sinh thiếu ở learning/game/chat | Projection `ref_*` chưa nhận event (NATS chưa chạy hoặc consumer lỗi) | `make seed-check`; `nats consumer info EVT_IDENTITY <svc>-ref`; xem bảng `processed_events` |
| Godot đăng nhập báo lỗi `503`/`UNAVAILABLE` | identity-service không gọi được Keycloak (sai `EW_KC_GAME_CLIENT_SECRET_FILE` hoặc Keycloak chưa sẵn sàng) | Kiểm tra secret `kc_game_backend` và log identity-service (log không có body đăng nhập) |
| Mọi request game trả `400 TENANT_HEADER_REQUIRED` | Client chưa gắn `X-Tenant-Id` sau bootstrap (ADR-0065) | Kiểm tra `AuthService.tenant_id` được gán từ response `/auth/game/login` |
| `TENANT_NOT_SELECTED` / `TENANT_MISMATCH` | Thuộc nhiều tenant chưa chọn, chọn sai mã, hoặc membership chưa sync | Chọn tenant; kiểm tra `DEMOA1`/`PGDDM2`; chạy `identity-worker sis-sync --once` |
| `atlas migrate apply` lỗi `permission denied` | Dùng DSN app thay vì migrator, hoặc sai database | Dùng `EW_DB_MIGRATOR_DSN` của đúng service |
| Query trả rỗng dù có dữ liệu | Thiếu tenant trong context → global filter ent + RLS không trả dòng nào (đúng thiết kế) | Kiểm tra middleware tenant; test phải tạo context có `TenantScope` |
| Testcontainers lỗi trên Windows | Docker Desktop chưa bật WSL integration | Chạy test trong WSL2 |
| APISIX 404 | Route standalone chưa reload / upstream sai | `docker compose restart apisix`; kiểm tra `apisix.yaml` kết thúc bằng `#END` |
| WebSocket đóng ngay với mã 4401 | Ticket hết hạn (60 s) hoặc đã dùng | Xin ticket mới mỗi lần kết nối |
| Lệnh WS timeout | Service đích không subscribe `ew.cmd.<svc>.>` (chưa chạy) | Chạy service; `nats sub 'ew.cmd.>'` trong `nats-box` để xem |
| Gửi chat báo `CHAT_TEMPORARILY_UNAVAILABLE` | `nats` chưa chạy hoặc stream chưa tạo | `docker compose up -d nats`; chạy `chat-service` một lần để tạo stream |
| Tin gửi được nhưng lịch sử thiếu | `chat-persister` (chat-worker) không chạy | `nats consumer info CHAT_MESSAGES persister`; chạy `make svc-chat-worker` |
| Ký tự tiếng Việt lỗi trong CSV seed | Encoding không phải UTF-8 | Lưu UTF-8 không BOM |
