# Feature Specification: F004 — Keycloak realm as code và APISIX gateway (route, `tenant-header`, gateway nội bộ)

**Feature Branch**: `004-keycloak-realm-apisix-gateway` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: hạ tầng (`deploy/keycloak`, `deploy/apisix`) · **Mốc**: M-C0 · **Phụ thuộc**: F001
**Input**: Cấu hình realm `eduworld` dạng code (client, flow, scope, role, chính sách phiên/mật khẩu) và hai gateway APISIX (public, nội bộ) với route tới binary `core-api`/`realtime` (ADR-0074 mục 5), plugin `tenant-header`, rate limit, không ghi body đăng nhập; client `eduworld-portal` confidential + direct grant (ADR-0091) — FR-AUTH-003, FR-AUTH-012, FR-AUTH-013, FR-AUTH-018, FR-AUTH-029, FR-SIS-007, NFR-SEC-002, NFR-SEC-003.

## Clarifications

### Session 2026-11-16

- Q: Plugin `tenant-header` hiện thực bằng Lua plugin riêng hay `serverless-pre-function`? → A: Lua plugin riêng `deploy/apisix/plugins/ew-tenant-header.lua` (xem [research.md](research.md)) [Confirmed].
- Q: Theme đăng nhập `eduworld-ops` có trong feature này không? → A: Chỉ khung theme kế thừa theme mặc định của Keycloak + file dịch vi/en để realm import được; giao diện hoàn chỉnh theo token admin do F051.
- Q: Ngưỡng rate limit cụ thể? → A: Theo tên tham số `login_rate`, `api_rate_default` và bảng [SECURITY_ARCHITECTURE §14](../../docs/10-security/SECURITY_ARCHITECTURE.md) (nguồn duy nhất); cấu hình APISIX đọc giá trị từ biến môi trường, không chép số vào spec.
- Q: 20 operation `/admin/*` trong spec gộp thiếu `x-internal: true` (moderation, chat tenant-terms, audit-logs, data-requests)? → A: Gateway định tuyến theo prefix (API_DESIGN §10) nên `/admin/*` luôn chỉ ở gateway nội bộ; bổ sung `x-internal: true` cho đồng bộ (contracts §6).

### Session 2026-11-17

- Q: Portal còn dùng Authorization Code + PKCE và browser flow `eduworld-portal-browser`? → A: Không. [ADR-0091](../../docs/adr/ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md): client `eduworld-portal` (giữ client id) chuyển **confidential**, tắt Standard Flow/Implicit, bật Direct Access Grants, secret chỉ module identity giữ (`${env.KC_PORTAL_CLIENT_SECRET}`); không redirect URI, web origin hay PKCE. Direct grant flow `eduworld-portal-direct-grant`: Username Validation → Password → "Allow access only for role" (`tenant_staff`/`platform_staff`, thông điệp `portalStudentDenied`) → Conditional OTP (bắt buộc với `platform_staff`, tùy chọn khi đã đăng ký). Chỉ TOTP, bỏ WebAuthn với portal (F004-AC-24…AC-26).
- Q: Theme `eduworld-ops` còn dùng ở đâu? → A: Chỉ cho trang action-token của Keycloak mở từ email (`CONFIGURE_TOTP` do `execute-actions-email`) — ngoại lệ duy nhất dùng trang Keycloak (ADR-0091 mục 3); không còn trang đăng nhập Keycloak cho portal (F004-AC-27).
- Q: `amr`/`acr` có xuất hiện trong token direct grant không, và direct grant trả gì khi tài khoản còn required action? → A: Chưa chắc chắn → **spike** T039 [Confirmed] phải xong trước khi F012/F051 làm phần đăng nhập portal: xác nhận mapper AMR (`amr` chứa `otp` khi bước OTP chạy, `pwd` khi không) và LoA (`acr` `1`/`2`), hành vi "Account is not fully set up" và việc Keycloak tự thêm `CONFIGURE_TOTP` cho `platform_staff` chưa có OTP (F004-AC-29).
- Q: Route `/api/v1/auth/portal/*` ở gateway nào? → A: APISIX public, phục vụ dưới origin của portal (`https://portal.<domain>/api/v1/...`) để cookie refresh/CSRF là first-party [Confirmed]; không `openid-connect`, không `ew-tenant-header`, không ghi body, rate limit theo `portal_login_rate` (username, `device_id`, toàn cục; không theo IP — R30) (F004-AC-28).

### Session 2026-11-18 (khi triển khai)

- Q: Cú pháp biến của realm as code? → A: `keycloak-config-cli` dùng `$(env:KC_…)` (bật `IMPORT_VARSUBSTITUTION_ENABLED`), không phải `${env.KC_…}`; `check-realm.mjs` và `check-no-hardcode.mjs` chấp nhận cú pháp này. `organizations` và User Profile không áp được qua `keycloak-config-cli` → Admin API trong `post-apply.mjs`/`seed-dev-orgs.mjs` (User Profile qua component `declarative-user-profile`).
- Q: Kết quả spike R5 ảnh hưởng tiêu chí MFA thế nào? → A: Direct grant luôn `acr = 1` và không có `auth_time`; MFA platform xác định bằng `amr` chứa `otp` (`EW_OIDC_PLATFORM_REQUIRED_AMR`, backend F002), tuổi xác thực lấy `iat` khi thiếu `auth_time`. OTP chỉ yêu cầu khi tài khoản đã cấu hình OTP (OTP REQUIRED với user chưa có OTP trả 500); `platform_staff` chưa có OTP bị chặn ở backend (F012) chứ không ở flow.
- Q: Giới hạn đăng nhập đếm ở đâu? → A: Plugin Lua `ew-login-key` đếm theo thiết bị/username (HMAC)/toàn cục trong shared dict `ew-login-limit` **của từng node gateway** (APISIX standalone không có `limit-count` policy `redis` cho khóa tính trong plugin); ngưỡng `EW_RATE_*` là ngưỡng mỗi node. `api_rate_default` (T029) do plugin Lua `ew-api-rate` đếm theo `sub` trong Valkey (đọc/ghi riêng, Problem Details `429 RATE_LIMITED`), không dùng `limit-count` có sẵn vì body lỗi không phải Problem Details. Ngưỡng riêng `/cms/*` (120/phút), `/sis/imports`, telemetry, chat report… của §14 chưa tách profile — đã chốt 2026-11-20 (profile theo route trong route-as-code, Session owner chốt).
- Q: Gateway internal có yêu cầu `X-Tenant-Id`? → A: Route `platform-internal` chế độ `optional` (System Admin chọn tenant đích theo use case); route portal dùng chung (`portal-tenant`) giữ `required` trên cả hai gateway.
- Q: `/auth/game/logout` có `openid-connect`? → A: Có. Contract không đặt `security: []` (trả `401`), nên route riêng `auth-game-logout` có `openid-connect` + `ew-tenant-header`; vẫn `sensitive` (không ghi body). `check-routes.mjs` bắt lệch này.
- Q: Đầu ra ADC và `adc diff`? → A: Chưa làm trong F004 (cần APISIX etcd của dev/staging, F130); F004 sinh standalone, job CI boot từng file render trong image APISIX và đưa diff render staging (so nhánh base) vào job summary. `keycloak-config-cli --dry-run` trên staging cũng chuyển F130.

### Session 2026-11-20 (owner chốt)

- [TBD] ngưỡng riêng theo nhóm route (Session triển khai, `api_rate_default`) → **Chốt:** plugin `ew-api-rate` nhận `profile` khai báo theo route trong route-as-code (`deploy/apisix/routes/routes.yaml`); bảng nhóm route → ngưỡng → khóa lấy từ API_DESIGN §8 (tài liệu sở hữu, không chép số vào plugin; số đọc từ cấu hình); route không khai báo dùng "Đọc chung"/"Ghi chung" theo method. F004 làm cơ chế profile và profile cho mọi dòng mức gateway hiện có trong bảng (`/cms/media:presign-upload`, `/schools/*/sis-imports`, `/learning/activity-attempts/*/responses`, `/game/telemetry`, `/api/v1/auth/*` còn lại); dòng ghi "nghiệp vụ, Valkey" và rate limit chat do module sở hữu đếm. Feature thêm route mới khai báo profile trong cùng MR.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Realm Keycloak được quản lý bằng code (Priority: P1)

DevOps áp realm `eduworld` từ Git cho mọi môi trường; secret chỉ đến từ secret manager; developer có realm dev với Organization và tài khoản demo.

**Why this priority**: M-C1 (đăng nhập portal/game) phụ thuộc client, flow và role đúng; R3, R4.

**Independent Test**: Chạy `keycloak-config-cli` hai lần trên Keycloak sạch; kiểm client, flow, scope bằng Admin API; thử password grant với từng client.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F004-AC-01 | Realm `eduworld` dạng code PHẢI khai báo client `eduworld-game-backend`, `eduworld-portal`, `eduworld-api`, `eduworld-sis-sync`, `svc-identity` theo [KEYCLOAK_SSO §3](../../docs/09-identity/KEYCLOAK_SSO.md), PHẢI KHÔNG chứa secret thật (chỉ placeholder `${env.KC_…}`) và PHẢI KHÔNG chứa user ở file dùng cho staging/production | FR-AUTH-013 · FR-AUTH-012 | — |
| F004-AC-02 | Chỉ hai client confidential `eduworld-game-backend` và `eduworld-portal` PHẢI bật Direct Access Grants; NẾU gọi password grant bằng client khác hoặc thiếu/sai secret, THÌ Keycloak PHẢI từ chối (`unauthorized_client`/`invalid_client`) | FR-AUTH-013 · ADR-0068 · ADR-0091 | TC-AUTH-067 |
| F004-AC-03 | Direct grant flow `eduworld-game-direct-grant` PHẢI từ chối người dùng không có realm role `student`; direct grant flow `eduworld-portal-direct-grant` PHẢI từ chối người dùng có role `student` (sau bước mật khẩu) và PHẢI bắt OTP khi người dùng có realm role `platform_staff` (chi tiết F004-AC-25) | FR-AUTH-013 · FR-AUTH-029 · ADR-0066 · ADR-0091 · KEYCLOAK_SSO §3.1b, §3.3 | TC-AUTH-064, TC-AUTH-095 |
| F004-AC-04 | Client `svc-identity` PHẢI là client service account duy nhất của backend (chỉ client credentials, tắt standard/implicit/direct grant) và chỉ được gán role `realm-management` cần cho Keycloak Admin API (thu hồi session, membership Organization, cập nhật user); realm PHẢI KHÔNG có client `svc-learning`, `svc-game`, `svc-competition`, `svc-chat`, `svc-realtime` hay client scope `internal-<module>` | FR-AUTH-013 · ADR-0074 · KEYCLOAK_SSO §3.2 | — |
| F004-AC-05 | Client scope `eduworld-api-audience` PHẢI thêm `aud = eduworld-api` và `eduworld-roles` thêm claim `roles` cho `eduworld-game-backend`, `eduworld-portal`; scope `organization:{alias}` PHẢI KHÔNG được gán cho `eduworld-portal` (tenant của phiên portal đi bằng `X-Tenant-Id` + membership — ADR-0091 mục 4); access token PHẢI KHÔNG chứa họ tên, email, ngày sinh; `offline_access` PHẢI KHÔNG được cấp | FR-AUTH-013 · KEYCLOAK_SSO §4.3 | — |
| F004-AC-06 | Realm PHẢI đặt `registrationAllowed=false`, `duplicateEmailsAllowed=false`, `loginWithEmailAllowed=true`, `editUsernameAllowed=false`, brute-force theo tài khoản theo `brute_force_lockout` (không permanent lockout), chính sách mật khẩu KEYCLOAK_SSO §10, token và phiên theo `access_token_ttl`, `game_session_sso` (client session `eduworld-game-backend` idle 7 ngày / tối đa 30 ngày, realm SSO Session ≥ giá trị đó — ADR-0086), `portal_session_sso` (client session `eduworld-portal`), refresh rotation bật (reuse 0) | FR-AUTH-013 · ADR-0064, ADR-0067, ADR-0086 · KEYCLOAK_SSO §5, §10 | TC-AUTH-061 |
| F004-AC-07 | KHI áp realm bằng `keycloak-config-cli` lần thứ hai không đổi nguồn, kết quả PHẢI không có thay đổi; realm dev PHẢI có Organization `th-demo-a`, `pgd-demo`, `common` và tài khoản demo của [LOCAL_DEVELOPMENT §5](../../docs/11-operations/LOCAL_DEVELOPMENT.md) | FR-AUTH-012 · LOCAL_DEVELOPMENT §5 | — |
| F004-AC-08 | NẾU file realm commit chứa secret thật hoặc file realm không phải dev chứa user, THÌ CI PHẢI fail | FR-AUTH-012 · FR-OPS-004 · AGENTS R4 | — |
| F004-AC-24 | Client `eduworld-portal` PHẢI là confidential (secret `${env.KC_PORTAL_CLIENT_SECRET}`), `standardFlowEnabled=false`, `implicitFlowEnabled=false`, `directAccessGrantsEnabled=true`, `serviceAccountsEnabled=false`, không `redirectUris`/`webOrigins`/thuộc tính PKCE, giữ nguyên client id; `check-realm.mjs` PHẢI fail nếu client này có redirect URI hoặc bật Standard Flow | FR-AUTH-013 · FR-AUTH-029 · ADR-0091 mục 2 | TC-AUTH-067 |
| F004-AC-25 | Direct grant flow `eduworld-portal-direct-grant` (gắn override cho `eduworld-portal`) PHẢI chạy theo thứ tự Username Validation → Password → Allow access only for role (`tenant_staff` hoặc `platform_staff`, thông điệp `portalStudentDenied`) → sub-flow Conditional OTP (điều kiện realm role `platform_staff` → OTP bắt buộc; người dùng khác → OTP khi đã cấu hình); OTP sai hoặc thiếu PHẢI trả cùng lỗi `invalid_grant` như sai mật khẩu; brute-force detection PHẢI giữ nguyên theo `brute_force_lockout` | FR-AUTH-029 · ADR-0091 mục 2 · KEYCLOAK_SSO §3.3 | TC-AUTH-093, TC-AUTH-094, TC-AUTH-095 |
| F004-AC-26 | KHI bước OTP của `eduworld-portal-direct-grant` chạy thành công, access token PHẢI có `amr` chứa `otp` và `acr` ở mức MFA; KHI bước OTP không chạy, `amr` PHẢI chỉ có `pwd` và `acr` ở mức mật khẩu (mapper AMR + ánh xạ LoA, reference value `pwd`/`otp` trên authenticator) [Confirmed — xác nhận bằng spike F004-AC-29] | FR-AUTH-029 · ADR-0091 mục 2 · ADR-0027 mục 3 | TC-AUTH-093 |
| F004-AC-27 | Theme `eduworld-ops` PHẢI chỉ phục vụ trang action-token của Keycloak (đăng ký TOTP từ link `execute-actions-email`, thông điệp vi/en) và email hành động; realm PHẢI KHÔNG gán browser flow hay login theme cho đăng nhập của `eduworld-portal`; WebAuthn PHẢI KHÔNG có trong flow của portal | FR-AUTH-029 · ADR-0091 mục 2, 3 | TC-AUTH-096 |
| F004-AC-29 | Spike [Confirmed] PHẢI ghi vào [research.md](research.md) (R5) kết quả đo trên Keycloak 26.x: claim `amr`/`acr` của token direct grant có/không OTP; lỗi Keycloak khi tài khoản còn required action (`UPDATE_PASSWORD`, `CONFIGURE_TOTP`) sau khi mật khẩu đúng; việc conditional OTP tự thêm `CONFIGURE_TOTP` cho `platform_staff` chưa có OTP; thông điệp khi bị "Allow access only for role" chặn; và phải xong trước task đăng nhập portal của F012/F051 | FR-AUTH-029 · ADR-0091 mục 2 | — |

---

### User Story 2 — Gateway định tuyến tới `core-api`/`realtime` và chặn route nội bộ (Priority: P1)

Mọi request từ game/portal đi qua APISIX public tới đúng binary (`/api/v1/*` → `core-api`, `/realtime/v1` → `realtime`); API platform chỉ có ở gateway nội bộ; thiếu header tenant bị chặn sớm.

**Why this priority**: ADR-0009, ADR-0027, ADR-0065; R7, R20.

**Independent Test**: Chạy APISIX standalone local, gọi từng prefix qua public/internal; gửi request thiếu/sai `X-Tenant-Id`; chạy script so route ↔ spec.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F004-AC-09 | APISIX public PHẢI định tuyến mọi prefix `/api/v1/*` của [API_DESIGN §10](../../docs/07-backend/API_DESIGN.md) tới upstream `core-api` và `/realtime/v1` tới upstream `realtime`, và PHẢI KHÔNG khai báo `/api/v1/admin/*`, `/api/v1/sis/*`, `/api/v1/game-admin/*`, `/api/v1/game-media*`, `/internal/*`, `/livez`, `/readyz`, `/metrics`; gọi các prefix này qua public PHẢI nhận 404 mà không tới backend | FR-SIS-007 · ADR-0009 · ADR-0027 · ADR-0074 | TC-CMS-WS-03, TC-CMS-AUTHZ-01 |
| F004-AC-10 | APISIX nội bộ PHẢI phục vụ prefix nội bộ với `openid-connect` (bearer, `aud = eduworld-api`) và `ip-restriction` theo CIDR quản trị lấy từ biến môi trường; request ngoài CIDR PHẢI bị từ chối 403 | FR-SIS-007 · API_DESIGN §9.17, §10 | TC-CMS-AUTHZ-01 |
| F004-AC-11 | NẾU route tenant-scoped nhận request thiếu `X-Tenant-Id`, THÌ plugin `ew-tenant-header` PHẢI trả `400 TENANT_HEADER_REQUIRED` dạng Problem Details; header sai UUID PHẢI nhận `400 BAD_REQUEST`; route ngoại lệ theo `tenant_header_required` PHẢI KHÔNG gắn plugin; route chế độ tùy chọn chỉ kiểm định dạng khi có header | FR-AUTH-018 · ADR-0065 | TC-TEN-011a, TC-TEN-011b |
| F004-AC-12 | KHI header hợp lệ, plugin PHẢI gắn `tenant.id` vào log/trace và làm khóa `limit-count` theo tenant; APISIX PHẢI KHÔNG quyết định quyền nghiệp vụ hay khớp tenant | FR-AUTH-018 · ADR-0009 | — |
| F004-AC-13 | NẾU token sai `aud`/`iss`/chữ ký, THÌ `openid-connect` PHẢI trả 401 trước khi tới backend; route không xác thực PHẢI chỉ gồm danh sách [DEPLOYMENT §5](../../docs/11-operations/DEPLOYMENT.md) | FR-AUTH-004 · ADR-0009 | TC-AUTH-009 |
| F004-AC-14 | Script kiểm route PHẢI so mọi route với spec gộp và fail khi route `/api/v1/*` (public hoặc nội bộ, gồm `/api/v1/game-admin/*`) có upstream khác `core-api`, khi `/realtime/v1` có upstream khác `realtime`, khi một operation của spec gộp không có route, khi prefix nội bộ xuất hiện trên gateway public, hoặc khi operation `/admin/*` thiếu `x-internal: true` | API_DESIGN §2.1, §10 | — |

---

### User Story 3 — Không lộ mật khẩu, TLS và rate limit ở biên (Priority: P1)

Mật khẩu học sinh đi qua `/auth/game/login` không bao giờ vào log/trace của gateway; mọi kết nối ngoài dùng TLS; lạm dụng bị giới hạn mà không chặn cả trường dùng chung NAT.

**Why this priority**: FR-AUTH-003, ADR-0068; R30 (không giới hạn đăng nhập theo IP đơn lẻ).

**Independent Test**: Gửi mật khẩu mẫu duy nhất qua APISIX rồi quét log/trace; gửi 11 request/phút từ một thiết bị; kiểm cấu hình TLS ở compose staging-like.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F004-AC-15 | Route `/api/v1/auth/game/*` PHẢI không gắn `openid-connect`, không gắn `ew-tenant-header` (trừ token:refresh/logout theo API_DESIGN §10), mọi logger PHẢI không bật `include_req_body`, `opentelemetry` PHẢI không thu body hay header nhạy cảm, body giới hạn theo API_DESIGN §10; NẾU cấu hình bật ghi body trên route này, THÌ CI PHẢI fail | FR-AUTH-003 · ADR-0068 | TC-AUTH-060 |
| F004-AC-16 | KHI gửi mật khẩu mẫu tới `/api/v1/auth/game/login` qua APISIX (thành công, sai, upstream 5xx), access log, error log và trace của APISIX PHẢI không chứa chuỗi mật khẩu | FR-AUTH-003 · NFR-SEC-002 | TC-AUTH-060 |
| F004-AC-17 | APISIX PHẢI giới hạn `/auth/game/login`, `/auth/game/tenant:select` theo thiết bị (`X-Device-Id`), theo username đã chuẩn hóa và ngưỡng toàn cục theo `login_rate` (vượt toàn cục → `429 LOGIN_QUEUE_BUSY` + `Retry-After` có jitter; vượt thiết bị/username → `429 RATE_LIMITED`), PHẢI KHÔNG giới hạn đăng nhập theo IP đơn lẻ, và áp `api_rate_default` cho route khác với bộ đếm lưu Valkey | NFR-SEC-003 · ADR-0045 · AGENTS R30 | TC-AUTH-063, TC-AUTH-043 |
| F004-AC-18 | Response bị giới hạn PHẢI có `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `Retry-After` và body Problem Details với `code` thuộc `ProblemCode` | NFR-SEC-003 · API_DESIGN §8 | — |
| F004-AC-19 | Ở NƠI môi trường là dev, staging hoặc production, APISIX public PHẢI chỉ phục vụ HTTPS TLS ≥ 1.2 với HSTS; route đăng nhập game PHẢI từ chối HTTP | NFR-SEC-002 · DEPLOYMENT §5 | — |
| F004-AC-20 | Route `/realtime/v1` PHẢI bật WebSocket, không `openid-connect`, có `limit-conn`, và access log PHẢI bỏ query string (chứa ticket) | NFR-SEC-002 · [MONITORING §8](../../docs/11-operations/MONITORING.md) | — |
| F004-AC-21 | Mọi route public PHẢI gắn `request-id`, `prometheus`, `opentelemetry` và `cors` với danh sách origin từ biến môi trường | FR-AUTH-012 · ADR-0023 | — |
| F004-AC-28 | Route `/api/v1/auth/portal/*` PHẢI ở APISIX public, phục vụ dưới host của portal (`EW_PORTAL_HOST`, cùng origin với SPA) [Confirmed], chỉ HTTPS, không `openid-connect`, không `ew-tenant-header`, logger và `opentelemetry` không thu body/cookie/header `X-EW-CSRF`, giữ nguyên `Set-Cookie`/`Cookie` (không cache), giới hạn theo `portal_login_rate` (theo username chuẩn hóa qua `ew-login-key`, theo `device_id`, toàn cục; vượt → `429 RATE_LIMITED`) và PHẢI KHÔNG giới hạn theo IP đơn lẻ; `check-no-body-logging.mjs` PHẢI bao cả prefix này | FR-AUTH-029 · NFR-SEC-002 · NFR-SEC-003 · ADR-0091 mục 5 · AGENTS R30 | TC-AUTH-099, TC-AUTH-100 |

---

### User Story 4 — Cấu hình theo môi trường, thay đổi có kiểm soát (Priority: P2)

DevOps áp cùng cấu hình route cho local (standalone) và dev/staging/prod (ADC + etcd), chỉ khác biến môi trường; mọi thay đổi có diff trước khi áp.

**Why this priority**: FR-AUTH-012, R3; cần trước F130 nhưng không chặn M-C1 local.

**Independent Test**: Render cấu hình cho `local` và `staging` từ cùng nguồn; `adc diff` trên etcd thử.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F004-AC-22 | Cấu hình route và realm PHẢI lấy upstream, hostname, CIDR, origin, issuer từ biến môi trường; PHẢI KHÔNG hardcode URL môi trường | FR-AUTH-012 · AGENTS R3 | — |
| F004-AC-23 | KHI PR thay đổi `deploy/apisix/**` hoặc `deploy/keycloak/**`, workflow `deploy-config.yml` PHẢI validate cấu hình standalone, chạy `adc diff` (dev/staging) và diff realm, đăng kết quả lên PR (job summary/comment) trước khi áp | FR-AUTH-012 · DEPLOYMENT §5, §10.2 · ADR-0088 | — |

### Edge Cases

- NẾU `X-Tenant-Id` có ở route miễn trừ (vd. `/auth/game/login`), THÌ gateway bỏ qua header, không lỗi (F004-AC-11).
- NẾU Valkey (bộ đếm rate limit) lỗi, THÌ APISIX dùng chế độ fail-open có giới hạn `limit-conn` toàn cục và ghi cảnh báo; quyền vẫn do backend kiểm (research.md, F004-AC-17).
- NẾU Keycloak JWKS tạm không truy cập được, THÌ `openid-connect` dùng JWKS đã cache; hết cache → 503 (F004-AC-13).
- NẾU spike F004-AC-29 cho thấy direct grant không phát `amr`/`acr` như mong đợi, THÌ dừng phần đăng nhập portal của F012/F051 và ghi "Câu hỏi mở" kèm ADR bổ sung; không tự kiểm OTP trong module identity (ADR-0091 phương án đã loại).
- NẾU request `/api/v1/auth/portal/*` mang `X-Tenant-Id`, THÌ gateway bỏ qua header (route miễn trừ, F004-AC-28).
- NẾU route ngoại lệ `/lms/event-templates`, `/tenant/students/*/login-events` khai báo sau route chung, THÌ thứ tự ưu tiên `priority` phải đưa route cụ thể lên trước (F004-AC-14 kiểm).
- Idempotency, audit nghiệp vụ, giới hạn độ dài field: không áp dụng ở gateway (backend F002/F003); gateway chỉ giới hạn kích thước body (F004-AC-15).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-003 | Route `/auth/game/*` không log/trace body ở APISIX + kiểm CI | SECURITY_ARCHITECTURE §3.1b, DEPLOYMENT §5 |
| FR-AUTH-012 | Realm + route cấu hình qua env, không hardcode | KEYCLOAK_SSO §11 |
| FR-AUTH-013 | Khai báo client, flow, scope, role, chính sách realm | KEYCLOAK_SSO §3–§5, §10 |
| FR-AUTH-029 | Client `eduworld-portal` confidential + direct grant flow `eduworld-portal-direct-grant`, mapper AMR/LoA, theme chỉ cho trang action-token, route `/api/v1/auth/portal/*` (ADR-0091) | KEYCLOAK_SSO §3.3 · SECURITY_ARCHITECTURE §14 |
| FR-AUTH-018 | Plugin `ew-tenant-header` (lớp gateway của kiểm tra header) | API_DESIGN §2, §10 · MULTITENANCY §8 |
| FR-SIS-007 | `/api/v1/sis` (và mọi prefix nội bộ) chỉ ở gateway nội bộ | API_DESIGN §9.15, §10 |
| NFR-SEC-002 | TLS ≥ 1.2, HSTS ở gateway public; không log mật khẩu | DEPLOYMENT §5 |
| NFR-SEC-003 | Rate limit APISIX theo thiết bị/username/toàn cục/route | SECURITY_ARCHITECTURE §14 |

### Key Entities

- **Realm `eduworld`**: client, flow xác thực, client scope, realm role (`student`, `tenant_staff`, `platform_staff`), Organization theo tenant.
- **Route gateway**: prefix, gateway (public/nội bộ), upstream (`core-api` hoặc `realtime`), module sở hữu (`x-service`, chỉ để kiểm tra/quan sát), bộ plugin.
- **Chính sách giới hạn**: khóa (thiết bị, username, tenant, user, toàn cục), ngưỡng, mã lỗi trả về.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `login_rate` | Giới hạn `/auth/game/login`, `tenant:select` |
| `portal_login_rate` | Giới hạn `/auth/portal/*` theo username, `device_id`, toàn cục (ADR-0091) |
| `api_rate_default` | Giới hạn GET/ghi/refresh/`auth/session` |
| `request_body_max` / `csv_import_max` | `client-control.max_body_size` |
| `tenant_header_required` | Danh sách route miễn header |
| `brute_force_lockout` | Brute-force Keycloak |
| `access_token_ttl`, `game_session_sso`, `portal_session_sso` | Token/phiên client |
| `game_login_body_logging` | Luôn tắt ghi body đăng nhập |
| `secret_rotation` | Chu kỳ xoay secret client (vận hành) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| APISIX không thay thế authorization trong Go | AGENTS R7 · ADR-0009 |
| Không giới hạn đăng nhập theo IP đơn lẻ (NAT trường) | AGENTS R30 · ADR-0045 |
| Không commit secret; secret client qua Docker secrets | AGENTS R4 · DEPLOYMENT §13 |
| Đăng nhập game qua backend, game không biết secret | ADR-0068 |
| Đăng nhập portal qua backend, SPA không biết secret `eduworld-portal` | ADR-0091 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 lần chuỗi mật khẩu mẫu xuất hiện trong log/trace của APISIX qua bộ test TC-AUTH-060.
- **SC-002**: 100% prefix nội bộ trả 404 khi gọi qua gateway public.
- **SC-003**: 100% request tenant-scoped thiếu header bị chặn ở gateway với `TENANT_HEADER_REQUIRED`.
- **SC-004**: Áp realm lặp lại tạo 0 thay đổi; 0 secret thật trong repository (gitleaks).
- **SC-005**: Học sinh khác cùng IP vẫn đăng nhập được khi một tài khoản bị giới hạn (TC-AUTH-043).

## Ngoài phạm vi

- Endpoint `/auth/game/*`, `/auth/session`, `/me/*`, xác định tenant và kiểm khớp header ↔ membership trong `core-api` → F012, F013; middleware tenant → F002.
- Giao diện trang action-token `eduworld-ops` hoàn chỉnh, màn đăng nhập portal tự vẽ → F051; endpoint `/auth/portal/*` → F012; provider đăng nhập nhanh, IdP brokering → ngoài Release 1.0 (ADR-0063).
- APISIX/Keycloak cluster production, cert manager, warm-up theo lịch → F130; load test đăng nhập → F132.

## Assumptions

- Keycloak 26.x (Organizations GA) và APISIX 3.x; `keycloak-config-cli` và ADC chạy trong workflow GitHub Actions `deploy-config.yml` (DEPLOYMENT §5, §6, §10; ADR-0088).
- Kết quả spike KEYCLOAK_SSO §2.1 (scope `organization` khi không là member) không ảnh hưởng feature này vì backend luôn kiểm chéo.
- Bộ đếm giới hạn đăng nhập nằm trong shared dict của từng node gateway (Clarifications 2026-11-18); Valkey `ratelimit` dành cho `api_rate_default` (DEPLOYMENT §5); ngưỡng từ biến môi trường theo SECURITY_ARCHITECTURE §14.
