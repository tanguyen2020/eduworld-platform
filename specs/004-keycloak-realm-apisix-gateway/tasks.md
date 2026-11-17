# Tasks: F004 — Keycloak realm as code và APISIX gateway (route, `tenant-header`, gateway nội bộ)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Thêm `apisix-internal` (cổng local `:9180`), mount `deploy/apisix/plugins`, job `keycloak-config-cli` vào compose — file: `deploy/compose/docker-compose.yml` — **xong 2026-11-18**
- [x] T002 [P] File env mẫu theo môi trường (upstream `EW_UPSTREAM_CORE_API`, `EW_UPSTREAM_REALTIME`, `EW_ADMIN_CIDRS`, `EW_CORS_ALLOWED_ORIGINS`, `EW_OIDC_ISSUERS`, ngưỡng rate từ SECURITY_ARCHITECTURE §14, `EW_APISIX_LOGIN_KEY_SECRET`) — file: `deploy/apisix/routes/env/local.env.example` — **xong 2026-11-18 — `routes/env/local.env` (không có secret: khóa HMAC là biến môi trường của container gateway) + `staging.env.example`**
- [x] T003 [P] Target `make keycloak-import`, `make apisix-render ENV=`, `make apisix-plugin-test` — file: `Makefile` — **xong 2026-11-18 — `keycloak-import/-check/-test`, `apisix-render/-check/-smoke`; test-nginx (`apisix-plugin-test`) thay bằng smoke trên gateway thật**

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Nguồn route `routes.yaml` theo API_DESIGN §10 (mọi prefix, gateway, upstream, auth, tenant_header, rate, body_max, log_body, priority) — file: `deploy/apisix/routes/routes.yaml` — **xong 2026-11-18**
- [x] T005 Delta contract: thêm `x-internal: true` cho 20 operation `/admin/*` (contracts §6), chạy `split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [x] T006 `render.mjs`: thay biến env, sinh `standalone/{public,internal}/apisix.yaml` (kết thúc `#END`) và `adc/{public,internal}.yaml` — file: `deploy/apisix/scripts/render.mjs` — **xong 2026-11-18 cho standalone (`--check`, `--out`); đầu ra ADC chuyển F130 (Clarifications 2026-11-18)**

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Realm Keycloak được quản lý bằng code (Priority: P1) 🎯 MVP

**Goal**: Realm chung + dev áp được, idempotent, không secret.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test F004-AC-01, F004-AC-08: `check-realm.mjs` kiểm danh sách client, secret chỉ dạng `${env.KC_…}` (dev cho phép `change-me-local`), file chung không có `users` — file: `deploy/keycloak/scripts/check-realm.test.mjs` — phải FAIL trước khi code
- [x] T008 [P] [US1] Test tích hợp F004-AC-02, F004-AC-03: password grant theo client (chỉ `eduworld-game-backend`, `eduworld-portal` có secret); user không `student` bị chặn ở game direct grant; `student` bị chặn ở `eduworld-portal-direct-grant` sau bước mật khẩu (TC-AUTH-067, TC-AUTH-064, TC-AUTH-095 phần Keycloak) — file: `deploy/keycloak/tests/flows.test.mjs`
- [x] T009 [P] [US1] Test tích hợp F004-AC-04, F004-AC-05: `svc-identity` là client service account duy nhất, chỉ có role `realm-management` tối thiểu; client `svc-<module>` khác và scope `internal-*` không tồn tại; token người dùng có `aud`, `roles`, không PII, không `offline_access` — file: `deploy/keycloak/tests/tokens.test.mjs`
- [x] T010 [P] [US1] Test tích hợp F004-AC-06, F004-AC-07: đọc realm qua Admin API so tham số; áp lần 2 không thay đổi (TC-AUTH-061 phần Keycloak) — file: `deploy/keycloak/tests/realm_settings.test.mjs`

### Implementation cho User Story 1

- [x] T011 [US1] Realm chung: realm settings, realm role, client scope (`eduworld-api-audience`, `eduworld-roles`, `organization` — không gán cho `eduworld-portal`), 5 client theo KEYCLOAK_SSO §3 (gồm `svc-identity` cho Admin API; `eduworld-portal` confidential theo T041) — file: `deploy/keycloak/realm-eduworld.json`
- [x] T012 [US1] Flow `eduworld-game-direct-grant` (role condition `student`) gắn override vào client; flow portal xem T041 (thay `eduworld-portal-browser`, ADR-0091) — file: `deploy/keycloak/realm-eduworld.json`
- [x] T013 [P] [US1] Realm dev: Organization `th-demo-a`/`pgd-demo`/`common`, user demo LOCAL_DEVELOPMENT §5 (realm role đúng loại tài khoản, OTP tắt local), secret `change-me-local` — file: `deploy/keycloak/realm-eduworld.dev.json`
- [x] T014 [P] [US1] Khung theme `eduworld-ops` chỉ cho trang action-token/required action và email (kế thừa theme mặc định, `messages_vi`, `messages_en`, không CSS hardcode; không trang đăng nhập — ADR-0091, F004-AC-27) — file: `deploy/keycloak/themes/eduworld-ops/login/theme.properties`
- [x] T015 [US1] `apply.sh` gọi `keycloak-config-cli` (var substitution) cho realm chung rồi dev (chỉ `EW_ENV=local`) — file: `deploy/keycloak/scripts/apply.sh`

### Bổ sung ADR-0091 — client `eduworld-portal` dùng direct grant

- [x] T039 [US1] SPIKE [Proposed] F004-AC-29: đo trên Keycloak 26.x claim `amr`/`acr` của direct grant có/không OTP, lỗi khi còn required action (`UPDATE_PASSWORD`, `CONFIGURE_TOTP`), conditional OTP tự thêm `CONFIGURE_TOTP`, thông điệp chặn `student`, trang action-token từ `execute-actions-email`; ghi kết quả vào research R5 — **chặn** phần đăng nhập portal của F012 và F051 — file: `specs/004-keycloak-realm-apisix-gateway/research.md`
- [x] T040 [P] [US1] Test F004-AC-24, F004-AC-25, F004-AC-26, F004-AC-27: `check-realm.mjs` fail khi `eduworld-portal` có redirect URI/Standard Flow/public; flow portal đúng thứ tự; OTP sai/thiếu cùng lỗi với sai mật khẩu; token có `amr`/`acr` theo bước OTP; không browser flow/login theme cho portal, không WebAuthn (TC-AUTH-067, TC-AUTH-093, TC-AUTH-094, TC-AUTH-095 phần Keycloak) — file: `deploy/keycloak/tests/portal_direct_grant.test.mjs` — phải FAIL trước khi code
- [x] T041 [US1] Client `eduworld-portal` confidential (`${env.KC_PORTAL_CLIENT_SECRET}`, Standard Flow/Implicit tắt, Direct Access Grants bật, bỏ redirect URI/web origin) + flow `eduworld-portal-direct-grant` (Username Validation → Password → allow-access-for-role `tenant_staff`/`platform_staff` → Conditional OTP theo `platform_staff`), xóa `eduworld-portal-browser` — file: `deploy/keycloak/realm-eduworld.json`
- [x] T042 [US1] Mapper AMR (reference value `pwd`/`otp` trên authenticator) và ánh xạ ACR→LoA cho `eduworld-portal` theo kết quả T039 — file: `deploy/keycloak/realm-eduworld.json`
- [x] T043 [P] [US1] Realm dev: secret `change-me-local` cho `eduworld-portal`; tài khoản `sa.admin` có OTP seed (local tắt bắt buộc theo LOCAL_DEVELOPMENT §5) và một tài khoản nhân sự còn `UPDATE_PASSWORD` để thử luồng thiết lập — file: `deploy/keycloak/realm-eduworld.dev.json`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Gateway định tuyến tới `core-api`/`realtime` và chặn route nội bộ (Priority: P1)

**Goal**: Hai gateway, plugin `ew-tenant-header`, kiểm route ↔ spec.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T016 [P] [US2] Smoke F004-AC-09, F004-AC-10: prefix nội bộ 404 ở public (không log upstream), 403 ngoài CIDR ở internal (TC-CMS-WS-03, TC-CMS-AUTHZ-01) — file: `deploy/apisix/tests/smoke_internal_prefix.sh` — phải FAIL trước khi code — **xong 2026-11-18 — gộp vào `deploy/apisix/tests/smoke.test.mjs`; 403 ngoài CIDR chỉ kiểm ở mức render (`render.test.mjs`) vì máy dev nằm trong CIDR**
- [x] T017 [P] [US2] Test plugin F004-AC-11, F004-AC-12: thiếu header 400 `TENANT_HEADER_REQUIRED`, sai UUID 400 `BAD_REQUEST`, mode optional, route miễn trừ, set `ew_tenant_id` (TC-TEN-011a, TC-TEN-011b) — file: `deploy/apisix/tests/tenant_header.t` — **xong 2026-11-18 — gộp vào `smoke.test.mjs` (gateway thật thay test-nginx)**
- [x] T018 [P] [US2] Smoke F004-AC-13: token sai `aud`/`iss`/chữ ký → 401 ở APISIX; route không auth đúng danh sách DEPLOYMENT §5 (TC-AUTH-009 phần gateway) — file: `deploy/apisix/tests/smoke_oidc.sh` — **xong 2026-11-18 — gộp vào `smoke.test.mjs` (thiếu token, token sửa chữ ký, token realm `master`)**
- [x] T019 [P] [US2] Test F004-AC-14: `check-routes.mjs` fail khi route `/api/v1/*` có upstream khác `core-api`, `/realtime/v1` khác `realtime`, operation thiếu route, prefix nội bộ ở public, `/admin/*` thiếu `x-internal` — file: `deploy/apisix/scripts/check-routes.test.mjs` — **xong 2026-11-18 — 11 test**

### Implementation cho User Story 2

- [x] T020 [US2] Plugin `ew-tenant-header.lua` (schema `mode`, Problem JSON, `ctx.var.ew_tenant_id`) — file: `deploy/apisix/plugins/ew-tenant-header.lua` — **xong 2026-11-18**
- [x] T021 [US2] Plugin chung route public: `openid-connect` (bearer_only, discovery từ `EW_OIDC_ISSUERS`, `aud`), `request-id`, `prometheus`, `opentelemetry`, `cors`; internal thêm `ip-restriction` — file: `deploy/apisix/routes/routes.yaml` — **xong 2026-11-18**
- [x] T022 [US2] `check-routes.mjs` (so `routes.yaml` + file render với spec gộp) và thêm vào CI — file: `deploy/apisix/scripts/check-routes.mjs` — **xong 2026-11-18 — 484 operation, 0 lỗi; CI job `apisix`**

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Không lộ mật khẩu, TLS và rate limit ở biên (Priority: P1)

**Goal**: Route đăng nhập không ghi body, rate limit theo thiết bị/username/toàn cục, TLS.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T023 [P] [US3] Test F004-AC-15: `check-no-body-logging.mjs` fail khi `include_req_body`/OTel thu body trên `/auth/game/*` — file: `deploy/apisix/scripts/check-no-body-logging.test.mjs` — phải FAIL trước khi code — **xong 2026-11-18 — 8 test**
- [x] T024 [P] [US3] Smoke F004-AC-16: mật khẩu mẫu duy nhất (thành công/sai/upstream 5xx giả) → quét access/error log và Tempo không thấy (TC-AUTH-060 phần gateway) — file: `deploy/apisix/tests/smoke_no_password_log.sh` — **xong 2026-11-18 — gộp vào `smoke.test.mjs` (log gateway game + portal); quét Tempo và ca upstream 5xx chưa có (stack obs chưa chạy trong smoke)**
- [x] T025 [P] [US3] Test plugin + smoke F004-AC-17, F004-AC-18: 11 lần/phút theo thiết bị/username → 429 `RATE_LIMITED`; ngưỡng toàn cục → 429 `LOGIN_QUEUE_BUSY` + `Retry-After`; IP chung không bị chặn; header RateLimit (TC-AUTH-063, TC-AUTH-043) — file: `deploy/apisix/tests/login_key.t` — **xong 2026-11-18 — smoke thiết bị/username/IP chung + header; ngưỡng toàn cục `LOGIN_QUEUE_BUSY` chỉ kiểm qua cấu hình (cần tải ≥ 120 req/s)**
- [x] T026 [P] [US3] Test render F004-AC-19, F004-AC-20, F004-AC-21: staging có TLS ≥ 1.2 + HSTS; `/realtime/v1` websocket + `limit-conn` + log bỏ query; plugin chung trên mọi route public — file: `deploy/apisix/scripts/render.test.mjs` — **xong 2026-11-18 — 6 test**

### Implementation cho User Story 3

- [x] T027 [US3] Route `/auth/game/*`: không `openid-connect`, `client-control` body theo API_DESIGN §10, logger không body, OTel không body — file: `deploy/apisix/routes/routes.yaml` — **xong 2026-11-18**
- [x] T028 [US3] Plugin `ew-login-key.lua` (HMAC username chuẩn hóa) + ba `limit-count` (thiết bị, username, toàn cục với `rejected_code` và body Problem tương ứng) — file: `deploy/apisix/plugins/ew-login-key.lua` — **xong 2026-11-18**
- [x] T029 [US3] `limit-count`/`limit-req` theo `api_rate_default` và bảng SECURITY_ARCHITECTURE §14, policy `redis` (Valkey `ratelimit`, `allow_degradation` theo research R3) — file: `deploy/apisix/routes/routes.yaml` — **xong 2026-11-18 — plugin Lua `ew-api-rate` (Valkey, theo `sub`, đọc 300/ghi 60, `auth/session` 10/phút); ngưỡng riêng theo route của §14 (`/cms/*`, telemetry…) chuyển feature sở hữu route**
- [x] T030 [P] [US3] Cấu hình TLS/HSTS cho render dev/staging/prod; access log format MONITORING §8 — file: `deploy/apisix/routes/env/staging.env.example` — **xong 2026-11-18**
- [x] T031 [US3] `check-no-body-logging.mjs` và thêm vào CI — file: `deploy/apisix/scripts/check-no-body-logging.mjs` — **xong 2026-11-18**

### Bổ sung ADR-0091 — route `/api/v1/auth/portal/*`

- [ ] T044 [P] [US3] Test F004-AC-28: route `/auth/portal/*` không `openid-connect`/`ew-tenant-header`, chỉ host portal, `check-no-body-logging.mjs` bao prefix này, 11 lần/phút theo username/`device_id` → 429 `RATE_LIMITED`, ngưỡng toàn cục, IP chung không bị chặn, `Set-Cookie` đi qua nguyên vẹn (TC-AUTH-099, TC-AUTH-100 phần gateway) — file: `deploy/apisix/tests/portal_auth_routes.t` — phải FAIL trước khi code — **một phần** — đã có: route không `openid-connect`/`ew-tenant-header`, host portal, `check-no-body-logging` bao prefix, mật khẩu portal không vào log (smoke); còn lại (`Set-Cookie` đi qua nguyên vẹn, 429 theo username portal trên stack) cần backend F012
- [x] T045 [US3] Route `/api/v1/auth/portal/*` trên gateway public với `host = ${EW_PORTAL_HOST}` (cùng origin SPA [Proposed]), `ew-login-key` đọc `username`/`identifier`, `limit-count` theo `portal_login_rate`, logger/OTel không body/cookie/`X-EW-CSRF`, chỉ HTTPS — file: `deploy/apisix/routes/routes.yaml` — **xong 2026-11-18**

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Cấu hình theo môi trường, thay đổi có kiểm soát (Priority: P2)

**Goal**: Env-driven, diff trong PR.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T032 [P] [US4] Test F004-AC-22: quét `routes.yaml`, realm không có URL/hostname literal ngoài biến — file: `deploy/apisix/scripts/check-no-hardcode.test.mjs` — phải FAIL trước khi code — **xong 2026-11-18 — 3 test**
- [ ] T033 [P] [US4] Test F004-AC-23: workflow `deploy-config.yml` chạy trên PR mẫu, có job summary diff — file: `scripts/ci/test-deploy-config.sh` — **chưa làm** — cần chạy workflow trên PR GitHub thật

### Implementation cho User Story 4

- [x] T034 [US4] Job trong `deploy-config.yml` (action ghim SHA, `permissions: contents: read, pull-requests: write`): validate standalone (container APISIX `-t`), `adc diff`, diff realm (`keycloak-config-cli --dry-run`), compose validate; secret dev/staging chỉ từ GitHub Environment, không chạy diff có secret cho PR từ fork — file: `.github/workflows/deploy-config.yml` — **xong 2026-11-18 — job `apisix` (check + boot standalone trong image + diff staging vào job summary) và `keycloak` (kiểm tĩnh); `adc diff`/`keycloak-config-cli --dry-run` staging chuyển F130; chỉ `contents: read` vì job summary không cần `pull-requests: write`**

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [x] T035 [P] Cập nhật LOCAL_DEVELOPMENT §3.1 (`apisix-internal :9180`), §4 (biến `EW_APISIX_LOGIN_KEY_SECRET`, `EW_ADMIN_CIDRS`); DEPLOYMENT §5 ghi chốt plugin Lua `ew-tenant-header` (bỏ [Proposed]) — file: `docs/11-operations/LOCAL_DEVELOPMENT.md` — **xong 2026-11-18 — LOCAL_DEVELOPMENT §3.1, §4, §7, §8; DEPLOYMENT §5**
- [ ] T036 [P] Postman collection gateway (prefix nội bộ 404, thiếu header, rate limit) — file: `specs/postman/gateway.postman_collection.json` — **chưa làm** (tùy chọn) — các kịch bản đã có trong `smoke.test.mjs`
- [x] T037 [P] CHANGELOG mục F004 — file: `CHANGELOG.md` — **xong 2026-11-18**
- [x] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/004-keycloak-realm-apisix-gateway/quickstart.md` — **xong 2026-11-18**

## Dependencies & Execution Order

- Setup → Foundational → US1 và US2 (song song) → US3 (cần route của US2) → US4 → Polish.
- T004, T006 trước mọi task route; T011 trước T012, T013.
- F012/F013 cần US1; F051 cần client `eduworld-portal` (US1) và gateway (US2).
- T039 (spike) trước T041, T042 và trước phần đăng nhập portal của F012/F051; T041 trước T043; T045 cần T028 (`ew-login-key`).

## Definition of Done

- [ ] Mọi tiêu chí F004-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Ghi chú triển khai (2026-11-18)

- Realm chung áp bằng `adorsys/keycloak-config-cli:6.4.0-26.1.0` (job compose `keycloak-config`, profile `tools`); placeholder secret dùng cú pháp thay biến của công cụ `$(env:KC_…)` (tương đương `${env.KC_…}` trong tài liệu). Phần công cụ không biểu diễn được làm bằng Admin API idempotent: `post-apply.mjs` (bỏ `offline_access`/`uma_authorization` khỏi `default-roles-eduworld`) cho mọi môi trường; `seed-dev.mjs` (chỉ `EW_ENV=local`: `partialImport` người dùng với `SKIP` — áp lại không đặt lại mật khẩu nên không vi phạm `passwordHistory(3)` — và Organization + thành viên vì 6.4 không import organizations). `realm-eduworld.dev.json` sinh bằng `gen-dev-realm.mjs` (33 tài khoản LOCAL_DEVELOPMENT §5 + `sa.moi` cho required action; OTP cố định cho `sa.admin`, chỉ local).
- Kết quả spike R5 (research.md) thay đổi thiết kế: OTP chỉ chạy khi người dùng đã cấu hình (OTP REQUIRED với người chưa có → 500); MFA của token portal suy ra từ `amr` chứa `otp` (`acr` luôn `1` trong direct grant) — `httpx` nhận thêm `EW_OIDC_PLATFORM_REQUIRED_AMR` (mặc định `otp`); token direct grant không có `auth_time` → `httpx` dùng `iat`, F012 cưỡng chế tuổi xác thực khi refresh; User Profile (component `declarative-user-profile`) bỏ bắt buộc email/họ/tên; mỗi execution có authenticator config riêng (Keycloak giữ một config cho mỗi alias).
- `svc-identity` có `manage-users`, `view-users`, `query-users`, `manage-realm` (`manage-realm` cần cho membership Organization trên Keycloak 26 [Proposed]).
