# Feature Specification: F002 — Platform kit lõi: config, observability, problem, httpx, authz, tenancy

**Feature Branch**: `002-platform-kit-core` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/backend/pkg/` · **Mốc**: M-C0 · **Phụ thuộc**: F001
**Input**: Các gói dùng chung để mọi module (trong `core-api`, `realtime`, `worker`) xác thực token độc lập APISIX, dựng Principal, kiểm tra tenant và permission, trả lỗi Problem Details, ghi log/trace/metric thống nhất — FR-AUTH-004, FR-ORG-006, FR-CMS-012, FR-OPS-003, NFR-OBS-002, NFR-SEC-006, NFR-MAINT-001.

## Clarifications

### Session 2026-11-16

- Q: Permission catalog đặt ở `internal/authorization` (SECURITY_ARCHITECTURE §5.2) hay `src/backend/pkg/authz`? → A: `src/backend/pkg/authz/catalog.go` (D4, AGENTS R20); mọi binary gọi `ValidateCatalog` lúc khởi động.
- Q: `Bypass`/`TenantScope` ở `src/backend/pkg/tenancy` hay `src/backend/pkg/entx`? → A: `TenantScope`, `Bypass` ở `src/backend/pkg/tenancy`; interceptor/hook ent ở `src/backend/pkg/entx` (F003) — D5 [Confirmed].
- Q: Key cache Principal? → A: Mỗi module dùng prefix của mình `ew:<module>:principal:<user_id>:<tenant_id>` (VALKEY_DESIGN §1, ACL theo module); D7 chỉ chốt endpoint `principals:resolve` [Confirmed].
- Q: Internal API gọi sai `azp` trả `CALLER_NOT_ALLOWED` (API_DESIGN §12.1) hay `PERMISSION_DENIED` (TC-SVC-006)? → A: `403 CALLER_NOT_ALLOWED` theo catalog mã lỗi (API_DESIGN §4.1 là nguồn); TC-SVC-006 cần sửa kỳ vọng.

### Session 2026-11-17

- Q: Còn middleware `serviceauth` và token client credentials giữa module không? → A: Không (ADR-0074 mục 3, 5). Module gọi nhau trong tiến trình qua facade; `x-allowed-callers` được kiểm bởi linter (`make arch-check`, F001) và bởi `src/backend/pkg/modcall` lúc chạy (F003, `403 CALLER_NOT_ALLOWED`). F002 chỉ còn xác thực token người dùng trên HTTP public và route gateway nội bộ của `core-api`.
- Q: Permission platform-only gồm những gì ngoài tiền tố `game.`, `competition.config.`, `platform.`? → A: Thêm `moderation.case.*`, `moderation.sanction.*`, `moderation.school_notification.send`, `chat.sensitive_terms.write`, `chat.policy.toggle`, `chat.tenant_terms.write`, `chat.tenant_category.toggle` theo [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Policy RLS có lọc `deleted_at` không? → A: Không. RLS chỉ lọc tenant (PostgreSQL buộc dòng mới của `UPDATE` thỏa policy `SELECT`, nên `deleted_at` trong policy chặn soft-delete); lớp thứ hai cho bản ghi đã xóa là view `<table>_live` cho `<module>_readonly` (ADR-0089 §1).
- Q: Có những loại scope nào và `Bypass` nhận lý do gì? → A: `TenantScope` (`authz.ScopeFor(p)`), `CrossTenantScope` (`authz.CrossTenantScope(p, ids, reason)`), `PlatformScope` (`authz.PlatformScope(p)` cho nhân sự platform, `authz.SystemPlatformScope(job)` cho job platform); reason là hằng số kiểu `tenancy.BypassReason`, mỗi reason gắn một quyền DB ([MULTITENANCY §4.4](../../docs/06-architecture/MULTITENANCY.md#44-quy-tắc-interceptor-theo-mixin-ba-loại-scope-và-cổng-bypass-adr-0089)); `tenancy.Bypass(ctx, reason)` chỉ nhận lý do thuộc danh sách đóng `system_admin_cross_tenant`, `moderation_case`, `break_glass`, `trash`, `auth_login`, `tenant_clone`, `inactive_cleanup`, `re_evaluation`, `efficacy_study`, `platform_job`, ghi audit `cross_tenant.accessed` + metric `eduworld_tenant_filter_bypass_total{module,reason}`; API công khai `src/backend/pkg/tenancy`/`src/backend/pkg/authz` khóa từ F002, đổi chữ ký cần ADR (ADR-0089 §3).

### Session 2026-11-18 (quyết định khi triển khai, [Confirmed] — Technical Architect duyệt)

- Q: Danh sách lý do `Bypass` có bao nhiêu mục? → A: 11 — thêm `platform_analytics` theo ADR-0089 §3 (nguồn sở hữu); câu trả lời phiên 2026-11-17 liệt kê 10 mục là thiếu.
- Q: Resolve Principal bằng gì khi token không mang `user_id` EduWorld? → A: `PrincipalResolver.Resolve(ctx, {Subject{Issuer, Subject}, TenantID, ClientID})`; cache Valkey giữ đúng khóa `ew:<module>:principal:<user_id>:<tenant_id|platform>` của VALKEY_DESIGN, thêm khóa phụ `…:principal:sub:<hash(iss,sub)>` → `user_id` và `…:principal:idx:<user_id>` để `Invalidate(user_id, nil)` xóa mọi tenant; `ClientID`, `sid`, `Channel`, MFA lấy từ token mỗi request, không cache.
- Q: Phạm vi school/subject của UserRole biểu diễn thế nào? → A: `Principal` = `PrincipalData{…, Grants []Grant{Role, ScopeKind, Permissions, SchoolID, SubjectID}}` + tập permission hợp; `Grant.ScopeKind` để guard `platform_mfa` nhận ra role platform (kể cả role platform tùy biến). Thay cho `Scopes []Scope` trong BACKEND_DESIGN §6 (tránh trùng tên với `tenancy.Scope`).
- Q: Client id portal/game so sánh với gì? → A: `EW_KC_PORTAL_CLIENT_ID`, `EW_KC_GAME_CLIENT_ID` (R3), ánh xạ `azp` → `authz.Channel`.
- Q: Role hệ thống lệch chỗ nào so với ADMIN_PORTAL §2.3? → A: `report.export` cho `tenant_admin`, `school_admin`, `academic_manager`; `school_admin` thêm `social.club.approve`, `tenant.audit.read`; `tenant_admin` không có `learning_content.re_evaluate` (ma trận §2.4); `content_reviewer` thêm `learning_content.read`; `system_admin` = toàn bộ catalog (ma trận §2.4 ✔ mọi dòng); `moderation.case.lead` không nằm trong role `safety_moderator` (gán thêm cho trưởng ca).
- Q: `LiveView` dùng `security_invoker`? → A: Không — `security_barrier` + quyền chủ view, vì `_readonly` không có quyền trên bảng gốc (F002-AC-29); RLS vẫn lọc do `FORCE ROW LEVEL SECURITY`.
- Q: `tenant_header_required` cấu hình ở đâu? → A: Theo operation (`httpx.Operation.Tenant` = Required/Optional/None), sinh từ tham số `TenantIdHeader` của spec; route portal tự gắn `portal_staff` theo tiền tố, `/game-admin/*`, `/admin/*` thêm `platform_mfa`.
- Q: Linter `bypassreason`/`scopeguard` chạy thế nào? → A: Lệnh `tools/lint/ewlint` (go/analysis multichecker) trong `make lint` và `make arch-check`, không qua plugin golangci; F002-AC-26/27 kiểm bởi `tools/archgen`.
- Q: `routematrix` biết test nào phủ operation bằng cách nào? → A: Quét tĩnh lời gọi `authztest.Cover(t, "<operationId>", authztest.CrossTenant, authztest.RoleDenied)` (literal), thay cho đăng ký lúc chạy; operation "đã cài đặt" = literal `httpx.Operation{ID: …}` ngoài test.

### Session 2026-11-20 (owner chốt — việc nhận từ OQ của feature khác)

- OQ F027-2 (`media.upload` chỉ có ở `tenant_content_editor`, `system_admin`) → **Chốt:** thêm `media.upload` vào mọi role hệ thống có `learning_content.write`: `tenant_admin`, `school_admin`, `academic_manager`, `content_admin` (ADMIN_PORTAL §2.2 xếp `media.upload` cùng nhóm soạn nội dung; người soạn được phải tải được ảnh/âm thanh). Không thêm permission mới, chỉ sửa danh sách role trong `pkg/authz/roles.go`.
- OQ F011-3 (`routematrix` không nhận route viết bằng helper `op(...)`) → **Chốt:** sửa tool: `routematrix` lấy danh sách operation từ `pkg/module.Registry` (route thật đã đăng ký) trong test thay vì phân tích literal; không đổi route sang literal. Ma trận role/tenant bắt buộc cho mọi operation của mọi module.
- OQ F013-5 (IP client sau APISIX) → **Chốt:** `pkg/httpx` thêm `ClientIP(r)`: chỉ tin `X-Forwarded-For` (lấy địa chỉ không tin cậy ngoài cùng bên phải) khi `RemoteAddr` thuộc `EW_TRUSTED_PROXY_CIDRS` (node APISIX); ngoài ra dùng `RemoteAddr`. Module identity (`ip_hash` đăng nhập) và game (`login_events`) dùng chung hàm này; chỉ hash, không lưu IP thô (R23).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Service tự xác thực token, không tin APISIX (Priority: P1)

Backend dev gắn `src/backend/pkg/httpx` vào `core-api`; mọi request có token được kiểm tra đầy đủ ngay cả khi gọi thẳng binary bỏ qua gateway.

**Why this priority**: AGENTS R7, ADR-0009; mọi endpoint ở M-C1 phụ thuộc.

**Independent Test**: Server test dựng bằng `httpx`, JWKS giả, gửi token hợp lệ/sai `aud`/sai chữ ký/`alg=none`/hết hạn/token client credentials.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F002-AC-01 | KHI request tới route có xác thực, middleware `auth` PHẢI kiểm tra chữ ký theo JWKS, `iss` thuộc `EW_OIDC_ISSUERS`, `aud` chứa `EW_OIDC_AUDIENCE`, `exp`/`nbf` với độ lệch `EW_OIDC_ALLOWED_CLOCK_SKEW`, `azp` thuộc `EW_OIDC_ALLOWED_AZP`, `typ = Bearer` mà không phụ thuộc APISIX | FR-AUTH-004 · [KEYCLOAK_SSO §4.4](../../docs/09-identity/KEYCLOAK_SSO.md) | TC-AUTH-009 |
| F002-AC-02 | NẾU token thiếu, sai `aud`/`iss`, chữ ký giả hoặc `alg=none`, THÌ HỆ THỐNG PHẢI trả `401 UNAUTHENTICATED` dạng Problem Details, kể cả khi gọi thẳng `core-api` | FR-AUTH-004 · ADR-0009 | TC-AUTH-009 |
| F002-AC-03 | NẾU token hết hạn, THÌ HỆ THỐNG PHẢI trả `401 TOKEN_EXPIRED` | FR-AUTH-004 · [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md) | — |
| F002-AC-04 | HỆ THỐNG PHẢI giữ JWKS trong bộ nhớ, làm mới nền định kỳ và làm mới ngay khi gặp `kid` lạ (có giới hạn tần suất), PHẢI KHÔNG gọi introspection Keycloak | ADR-0045 · [BACKEND_DESIGN §6.1](../../docs/07-backend/BACKEND_DESIGN.md) | — |
| F002-AC-05 | NẾU request tới route ứng dụng mang token client credentials (không có người dùng, vd. client `svc-identity`), THÌ middleware `auth` PHẢI trả `401 UNAUTHENTICATED`; `httpx.NewServer` PHẢI KHÔNG đăng ký route hay listener cho tiền tố `/internal/v1/*` (request tới tiền tố này PHẢI nhận `404`) | ADR-0074 · API_DESIGN §12.1 | — |

---

### User Story 2 — Tenant của phiên được kiểm tra chéo ở mọi request (Priority: P1)

Mọi request tenant-scoped mang `X-Tenant-Id`; `core-api` dựng Principal theo tenant đó và tạo `TenantScope` duy nhất cho repository.

**Why this priority**: R8, ADR-0005, ADR-0065 — rò dữ liệu giữa trường là lỗi nghiêm trọng nhất.

**Independent Test**: Server test với `PrincipalResolver` giả (membership tenant A), gửi header A, B, thiếu, sai định dạng; test RLS bằng testcontainers.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F002-AC-06 | NẾU route tenant-scoped nhận request thiếu `X-Tenant-Id`, THÌ middleware `tenant` PHẢI trả `400 TENANT_HEADER_REQUIRED`; header sai UUID PHẢI nhận `400 BAD_REQUEST`; route miễn trừ theo `tenant_header_required` PHẢI KHÔNG yêu cầu header | FR-AUTH-018 · FR-ORG-006 · ADR-0065 | TC-TEN-011a, TC-TEN-011b |
| F002-AC-07 | KHI header hợp lệ, HỆ THỐNG PHẢI resolve Principal theo `(user_id, tenant_id, client_id)`; NẾU người dùng không có membership tenant đó, THÌ PHẢI trả `403 TENANT_MISMATCH`, ghi audit `authz.denied` (`reason = TENANT_MISMATCH`) và tăng `eduworld_auth_tenant_mismatch_total` | FR-ORG-006 · [MULTITENANCY §4.1](../../docs/06-architecture/MULTITENANCY.md) | TC-TEN-006, TC-TEN-011 |
| F002-AC-08 | HỆ THỐNG PHẢI lấy `Principal.TenantID` từ kết quả resolve, PHẢI KHÔNG lấy thẳng từ header; tham số `tenant_ids` của người dùng không có `platform.cross_tenant.read` PHẢI bị bỏ qua | FR-ORG-006 · ADR-0065 | TC-TEN-011, TC-CMS-TEN-01 |
| F002-AC-09 | NẾU module identity lỗi hoặc timeout và không có Principal hợp lệ trong cache, THÌ HỆ THỐNG PHẢI trả `503 UNAVAILABLE` có `retryable` và PHẢI KHÔNG cho qua | BACKEND_DESIGN §6 · NFR-AVAIL-005 | TC-SVC-011 |
| F002-AC-10 | HỆ THỐNG PHẢI cache Principal hai lớp (L1 trong tiến trình + Valkey) theo `principal_cache` và `principal_cache_ttl`; KHI `Invalidate(user_id, tenant_id)` được gọi, cả hai lớp PHẢI bị xóa ngay | [IDENTITY_MAPPING](../../docs/09-identity/IDENTITY_MAPPING.md) · ADR-0066 | — |
| F002-AC-11 | Scope PHẢI chỉ được tạo qua `authz.ScopeFor(p)` (`TenantScope`, đúng một tenant), `authz.CrossTenantScope(p, ids, reason)` khi Principal có `platform.cross_tenant.read`, `azp = eduworld-portal` và MFA, hoặc `authz.PlatformScope(p)` (nhân sự platform) / `authz.SystemPlatformScope(job)` (job platform) (chỉ dòng `tenant_id IS NULL`, không đọc được dòng có `tenant_id`); linter PHẢI fail khi code ngoài `src/backend/pkg/authz` tạo scope | FR-ORG-006 · MULTITENANCY §4.1, §4.4 · AGENTS R8 · ADR-0089 | TC-TEN-004, TC-TEN-018 |
| F002-AC-12 | KHI `tenancy.Bypass(ctx, reason)` được gọi, HỆ THỐNG PHẢI từ chối lý do rỗng hoặc ngoài danh sách đóng (`system_admin_cross_tenant`, `moderation_case`, `break_glass`, `trash`, `auth_login`, `tenant_clone`, `inactive_cleanup`, `re_evaluation`, `efficacy_study`, `platform_job`), ghi audit `cross_tenant.accessed` (trường `reason`, `tenant_ids`, `mode` = `read`/`write`) và tăng `eduworld_tenant_filter_bypass_total{module,reason}`; mỗi lần dùng `CrossTenantScope` PHẢI ghi cùng audit đó | AGENTS R27 · ADR-0039 · ADR-0089 | TC-TEN-023 |
| F002-AC-13 | Helper RLS PHẢI sinh policy cho bảng loại T, TA, T\*, TA\*, H khớp [DATABASE_DESIGN §4](../../docs/07-backend/DATABASE_DESIGN.md): T/TA/T\*/TA\* chỉ có điều kiện tenant, H có điều kiện tenant + gán, không policy nào chứa `deleted_at`; NẾU `app.tenant_id` không được đặt hoặc đặt tenant A, THÌ truy vấn không điều kiện trên bảng có dòng tenant B PHẢI trả 0 dòng | FR-ORG-006 · ADR-0005 · ADR-0089 | TC-TEN-005 |
| F002-AC-29 | KHI lệnh soft-delete (`UPDATE ... SET deleted_at = now() ... RETURNING`) chạy bằng role `<module>_app` trên bảng loại T có policy do helper sinh, HỆ THỐNG PHẢI thực hiện thành công (không lỗi "new row violates row-level security policy") và dòng đó PHẢI biến mất khỏi truy vấn có `deleted_at IS NULL` và khỏi view `<table>_live` do helper sinh; role `<module>_readonly` PHẢI KHÔNG đọc được bảng gốc | ADR-0089 §1 · ADR-0028 · [DATABASE_DESIGN §4](../../docs/07-backend/DATABASE_DESIGN.md) | — |
| F002-AC-30 | NẾU code gọi `tenancy.Bypass` với lý do không thuộc danh sách đóng của `src/backend/pkg/tenancy`, THÌ linter (`make lint`) PHẢI fail; NẾU chữ ký công khai của `TenantScope`, `CrossTenantScope`, `PlatformScope`, `SystemPlatformScope`, `Bypass`, `BypassReason`, `Principal` thay đổi, THÌ test API surface PHẢI fail | AGENTS R27 · ADR-0089 §3 | — |

---

### User Story 3 — Permission catalog trong code và chốt chặn theo loại client (Priority: P1)

Mọi module dùng chung catalog; role tenant không bao giờ chứa quyền platform; route portal chỉ cho nhân sự, route game chỉ cho học sinh, route platform cần MFA.

**Why this priority**: R20, R23, ADR-0027, ADR-0031, ADR-0066.

**Independent Test**: Unit test catalog; server test từng guard với token giả theo `azp`, `user_type`, `acr`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F002-AC-14 | `src/backend/pkg/authz` PHẢI chứa catalog permission theo [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) và định nghĩa role hệ thống theo ADMIN_PORTAL §2.3 trong code; mọi binary PHẢI gọi `authz.ValidateCatalog` khi khởi động | FR-CMS-012 · AGENTS R20 | TC-CMS-AUTHZ-04 |
| F002-AC-15 | NẾU role phạm vi `tenant` chứa permission tiền tố `game.`, `competition.config.`, `platform.` hoặc permission platform-only của moderation/chat (Clarifications), THÌ tiến trình PHẢI thoát mã 1 và log tên role + permission | FR-CMS-012 · ADR-0027 | TC-CMS-AUTHZ-04 |
| F002-AC-16 | KHI request tới `/cms/*`, `/lms/*`, `/tenant/*`, `/schools/*`, `/game-admin/*`, `/admin/*`, middleware `portal_staff` PHẢI yêu cầu `azp = eduworld-portal` và `user_type = staff`; nếu không PHẢI trả `403 PORTAL_ACCESS_STAFF_ONLY` kèm audit | ADR-0066 · ADR-0081 · [API_DESIGN §11](../../docs/07-backend/API_DESIGN.md) | TC-AUTH-070 |
| F002-AC-17 | KHI request tới route người chơi (API_DESIGN §11 mục 1b), middleware `game_player` PHẢI yêu cầu `azp = eduworld-game-backend` và `user_type = student`; nếu không PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` kèm audit | ADR-0031 · AGENTS R23 | TC-AUTH-014 |
| F002-AC-18 | Ở NƠI operation yêu cầu permission platform-only, HỆ THỐNG PHẢI yêu cầu `acr` đạt `EW_OIDC_PLATFORM_REQUIRED_ACR` và `auth_time` trong `EW_OIDC_PLATFORM_MAX_AUTH_AGE` (nếu không: `403 MFA_REQUIRED`) và role phạm vi platform (nếu không: `403 PERMISSION_DENIED` + audit `authz.denied`) | ADR-0027 · KEYCLOAK_SSO §3.3 | TC-CMS-WS-02, TC-CMS-AUTHZ-02 |
| F002-AC-19 | `authz.Require(ctx, p, perm, resource)` PHẢI trả `404 NOT_FOUND` khi tài nguyên ngoài tenant của phiên, `403 PERMISSION_DENIED` khi vượt phạm vi school/subject của UserRole hoặc khi trong phạm vi nhưng thiếu permission (không có mã scope riêng, RB-113) | API_DESIGN §4.1, §11 · ADR-0072 | TC-TEN-001, TC-TEN-002, TC-TEN-003 |

---

### User Story 4 — Lỗi, cấu hình và quan sát thống nhất (Priority: P2)

Vận hành tra một lỗi bằng `trace_id`, thấy log/trace/metric cùng nhãn `binary`, `module`; không có PII hay token trong log.

**Why this priority**: NFR-OBS-002, R14, R18; cần trước khi có nhiều module.

**Independent Test**: Unit test `problem`, `config`; test log capture quét chuỗi nhạy cảm; test exporter OTel in-memory.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F002-AC-20 | Mọi lỗi HTTP PHẢI trả `application/problem+json` RFC 9457 có `code` thuộc `ProblemCode` và `trace_id`; lỗi không nhận diện PHẢI trả `500 INTERNAL` không lộ chi tiết nội bộ | [API_DESIGN §4](../../docs/07-backend/API_DESIGN.md) · ADR-0010 | — |
| F002-AC-21 | Hằng mã lỗi trong `src/backend/pkg/problem` PHẢI được sinh từ enum `ProblemCode` của spec gộp; NẾU lệch, THÌ `make gen-check` PHẢI fail | API_DESIGN §4.1 · NFR-MAINT-002 | — |
| F002-AC-22 | NẾU thiếu biến môi trường bắt buộc, THÌ binary PHẢI thoát mã 1 và log tên biến, PHẢI KHÔNG in giá trị; biến `*_FILE` PHẢI được đọc từ file secret | FR-AUTH-012 · [BACKEND_DESIGN §12](../../docs/07-backend/BACKEND_DESIGN.md) | — |
| F002-AC-23 | Log PHẢI là JSON `slog` có các trường BACKEND_DESIGN §11 và `binary`, `module`; PHẢI KHÔNG chứa token, header `Authorization`, cookie, mật khẩu, nội dung chat, câu trả lời, họ tên, email, IP đầy đủ | NFR-OBS-002 · AGENTS R14 · [MONITORING §8](../../docs/11-operations/MONITORING.md) | TC-AUTH-060 |
| F002-AC-24 | HỆ THỐNG PHẢI tạo span server cho mọi request, tiếp nhận và lan truyền `traceparent`, và xuất `eduworld_http_server_duration_seconds`, `eduworld_http_server_requests_total` với nhãn MONITORING §2 (không có user_id, IP) | NFR-OBS-002 · FR-OPS-003 · ADR-0023 | — |
| F002-AC-25 | NẾU trường chuỗi trong request vượt `maxLength` của spec, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` với `errors[].code = MAX_LENGTH` và tên trường, trước khi vào application | ADR-0041 · API_DESIGN §13 | TC-TEN-024 |

---

### User Story 5 — Kiến trúc được kiểm bằng máy và test cô lập tenant sinh tự động (Priority: P2)

Tech Lead muốn CI chặn vi phạm tầng trong module và bảo đảm mọi route tenant-scoped có test tenant A/B.

**Why this priority**: NFR-MAINT-001, NFR-SEC-006 yêu cầu 100% endpoint tenant-scoped.

**Independent Test**: Thêm import cấm vào module mẫu → `make lint` đỏ; thêm route mới không có test → CI đỏ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F002-AC-26 | NẾU `internal/domain` import ent, pgx, `net/http`, nats, valkey hoặc `application` import `infrastructure`/`presentation`, THÌ `make lint` PHẢI fail (import chéo module do `make arch-check` chặn theo F001-AC-21…23) | NFR-MAINT-001 · AGENTS R13 · [BACKEND_DESIGN §3](../../docs/07-backend/BACKEND_DESIGN.md) | — |
| F002-AC-27 | Gói `src/backend/pkg/*` PHẢI KHÔNG import `src/backend/modules/*`; `src/backend/pkg/authz`, `src/backend/pkg/problem`, `src/backend/pkg/tenancy` PHẢI KHÔNG import ent hay pgx | NFR-MAINT-001 · ADR-0038 | — |
| F002-AC-28 | Công cụ `routematrix` PHẢI sinh từ spec public và internal danh sách operation tenant-scoped và operation có ID trong path; NẾU một operation của module thiếu ca test tenant A/B và ca role không đủ quyền, THÌ CI PHẢI fail | NFR-SEC-006 · [BACKEND_DESIGN §15](../../docs/07-backend/BACKEND_DESIGN.md) | TC-TEN-001 |

### Edge Cases

- NẾU JWKS không tải được lúc khởi động, THÌ `/readyz` PHẢI trả 503 (check `jwks`) và request có token PHẢI nhận `503 UNAVAILABLE` (F002-AC-04, F002-AC-09).
- NẾU System Admin gửi `X-Tenant-Id` của tenant `disabled` khi ghi, THÌ PHẢI trả `403 TENANT_ACCESS_DISABLED` (MULTITENANCY §4.1 mục 6).
- NẾU người dùng tenant gửi cả `X-Tenant-Id` hợp lệ và `tenant_ids` của tenant khác, THÌ `tenant_ids` bị bỏ qua (F002-AC-08).
- NẾU token game (`eduworld-game-backend`) gọi `/cms/*`, THÌ trả `403 PORTAL_ACCESS_STAFF_ONLY` (F002-AC-16).
- Idempotency và audit của thao tác ghi nghiệp vụ: do middleware idempotency (`src/backend/pkg/httpx`), `UnitOfWork` (`src/backend/pkg/entx`) và recorder outbox của F003 (ADR-0087); F002 định nghĩa interface `audit.Recorder` dùng cho các audit ở trên.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-004 | Middleware `auth`, JWKS cache | KEYCLOAK_SSO §4.4, BACKEND_DESIGN §6 |
| FR-ORG-006 | Middleware `tenant`, `TenantScope`, `Bypass`, helper RLS | MULTITENANCY §4–§5, DATABASE_DESIGN §4 |
| FR-CMS-012 | Catalog permission + role hệ thống + `ValidateCatalog`; guard platform/MFA | ADMIN_PORTAL §2, SECURITY_ARCHITECTURE §5.2 |
| FR-OPS-003 | Metric HTTP/DB pool chuẩn (API latency, error rate, DB performance); metric nghiệp vụ khác do feature sở hữu | MONITORING §3 |
| NFR-OBS-002 | `src/backend/pkg/observability`: slog JSON, OTel, Prometheus, nhãn `binary`/`module` | MONITORING §1, §7, §8 |
| NFR-SEC-006 | `routematrix` + harness test tenant/role | TEST_STRATEGY §6.7 |
| NFR-MAINT-001 | Cấu hình depguard + go-arch-lint mẫu | BACKEND_DESIGN §3 |

### Key Entities

- **Principal**: người dùng (hoặc job hệ thống qua `SystemPlatformScope`) đã xác thực trong một tenant: user, loại người dùng, tenant, quyền hợp nhất từ UserRole, phạm vi school/subject, client, phiên.
- **TenantScope / CrossTenantScope / PlatformScope**: phạm vi cho truy vấn: một tenant (người dùng tenant, consumer event, lệnh realtime), danh sách tenant có lý do (System Admin), hoặc chỉ dữ liệu `tenant_id IS NULL` (nhân sự platform qua `PlatformScope(p)`, job platform qua `SystemPlatformScope(job)`) (ADR-0089 §3).
- **Permission catalog**: danh sách permission và role hệ thống, kèm bất biến phạm vi platform/tenant.
- **Problem**: lỗi chuẩn RFC 9457 với mã ổn định.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `principal_cache` | TTL L1 + Valkey của Principal |
| `principal_cache_ttl` | TTL Valkey, làm mới khi role đổi |
| `tenant_header_required` | Danh sách route miễn header |
| `access_token_ttl` | Kiểm thử hết hạn token (không cấu hình trong code) |
| `portal_session_sso` | Giới hạn `auth_time` cho route platform |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Verify JWT < 1 ms; resolve Principal cache hit ≥ 95% | BACKEND_DESIGN §6.1 · ADR-0045 |
| Không fail-open khi không xác định quyền | API_DESIGN §12.3 · TC-SVC-011 |
| Không log PII/token | AGENTS R14 · MONITORING §8 |
| Không hardcode issuer/audience/URL | AGENTS R3 · FR-AUTH-012 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% token giả mạo/sai `aud`/`iss`/hết hạn bị từ chối khi gọi thẳng `core-api` (bộ test TC-AUTH-009).
- **SC-002**: 100% operation tenant-scoped trong spec có ca test tenant A/B do `routematrix` sinh; 0 rò dữ liệu chéo tenant.
- **SC-003**: Catalog có role tenant chứa quyền platform → 100% lần khởi động thất bại.
- **SC-004**: Quét log của bộ test tích hợp không tìm thấy token/mật khẩu mẫu (0 lần xuất hiện).
- **SC-005**: Mọi lỗi API có `trace_id` tra được trong Tempo.

## Ngoài phạm vi

- `src/backend/pkg/entx` (interceptor, hook soft-delete, unit of work `InTenantTx`), idempotency, outbox, `natsx`, `modcall`, client `principals:resolve` thật và consumer vô hiệu cache, `testkit` → F003.
- Endpoint `principals:resolve`, `GET /me/permissions`, bảng Role/UserRole → F012; middleware so tenant với game session → F014.
- Cấu hình APISIX (`tenant-header` ở gateway) → F004; dashboard/alert → F131.

## Assumptions

- D4, D5, D7 của CODE_PLAN §3 theo đề xuất [Confirmed].
- Thư viện theo ADR-0025: chi, `coreos/go-oidc` + `go-jose`, `koanf`, `log/slog`, OpenTelemetry Go, `prometheus/client_golang`, `kin-openapi` (validator request).
- `resolvePrincipal` trả `404` khi người dùng không có membership tenant yêu cầu → ánh xạ `TENANT_MISMATCH` (contracts §6 ghi cần làm rõ trong spec internal).
