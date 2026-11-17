# Tasks: F002 — Platform kit lõi: config, observability, problem, httpx, authz, tenancy

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Thêm dependency (chi, go-oidc, go-jose, koanf, otel, prometheus, kin-openapi, valkey-go, golang-lru) vào Go module `src/backend` — file: `src/backend/go.mod` (Go module duy nhất, ADR-0074, ADR-0092)
- [x] T002 [P] `src/backend/pkg/clock` (Clock, Fake) và `src/backend/pkg/ids` (UUIDv7) — file: `src/backend/pkg/ids/ids.go`
- [ ] T003 [P] Cấu hình lint nền: depguard theo tầng BACKEND_DESIGN §3, go-arch-lint, rule cấm `src/backend/pkg/*` import `src/backend/modules/*` — file: `src/backend/tools/lint/golangci.base.yml`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Generator mã lỗi từ enum `ProblemCode` (spec gộp) → `codes_gen.go`; hook vào `make gen` / `make gen-check` — file: `src/backend/pkg/problem/gen/main.go` — kiểm: `make gen-check`
- [x] T005 Áp sớm (M-C0, contract-first để F003 sinh facade) delta `resolvePrincipal` mà F012 sở hữu nội dung (F012 contracts §6, task F012 T003): `PrincipalResolveRequest` định danh bằng `issuer` + `subject` (oneOf `user_id`), `tenant_id` nullable, `Principal.tenant_id`/`tenant_type` nullable, `is_system_admin`, mô tả `404` = không membership; không tự định nghĩa thêm trường — file: `specs/openapi/modules/identity.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [x] T006 Generator allowlist `x-allowed-callers` → `api/allowed_callers_gen.go` của từng module (dùng bởi `src/backend/pkg/modcall` kiểm lúc chạy ở F003) — file: `src/backend/tools/allowedcallers/main.go` — kiểm: `make gen MOD=learning`
- [x] T007 `src/backend/pkg/audit` (owner duy nhất của interface; hiện thực outbox ở F003 T042, F015 chỉ tiêu thụ): interface `Recorder.Record(ctx, Event) error`, `Event{Action, TargetType, TargetID, Result, Reason, Metadata}` (độ dài theo DATABASE_DESIGN §3.11) — file: `src/backend/pkg/audit/recorder.go`
- [x] T055 Delta contract `ProblemCode`: gỡ `FORBIDDEN_SCOPE` khỏi enum và mô tả `403` (spec gộp + spec theo module) rồi sinh lại `codes_gen.go` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/*.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen-check` — **2026-11-17:** phần contract đã áp (gỡ `FORBIDDEN_SCOPE` khỏi `ProblemCode`, mô tả response `Forbidden`, API_DESIGN §4.1; `split-public.mjs`); còn sinh lại `codes_gen.go` (cần generator T004)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Service tự xác thực token, không tin APISIX (Priority: P1) 🎯 MVP

**Goal**: Middleware `auth` và JWKS cache.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T008 [P] [US1] Test F002-AC-01, F002-AC-02: bảng ca token hợp lệ/sai aud/sai iss/chữ ký giả/alg none/thiếu/`typ` sai/`azp` ngoài danh sách — file: `src/backend/pkg/httpx/auth_test.go` — phải FAIL trước khi code
- [x] T009 [P] [US1] Test F002-AC-03: token hết hạn → `TOKEN_EXPIRED`; `nbf` tương lai ngoài skew → 401 — file: `src/backend/pkg/httpx/auth_expiry_test.go`
- [x] T010 [P] [US1] Test F002-AC-04: JWKS refresh khi gặp kid lạ, giới hạn tần suất, không gọi introspection — file: `src/backend/pkg/httpx/jwks_test.go`
- [x] T011 [P] [US1] Test F002-AC-05: token client credentials (`svc-identity`) tới route ứng dụng → 401; mọi request `/internal/v1/*` → 404 vì server không đăng ký tiền tố — file: `src/backend/pkg/httpx/no_internal_route_test.go`

### Implementation cho User Story 1

- [x] T012 [US1] JWKS cache (go-oidc RemoteKeySet bọc refresh nền + rate limit kid lạ) — file: `src/backend/pkg/httpx/jwks.go`
- [x] T013 [US1] Middleware `auth` dựng `authz.Claims` (sub, azp, sid, acr, auth_time, roles) vào context — file: `src/backend/pkg/httpx/auth.go`
- [x] T014 [US1] `auth` từ chối token không có người dùng (client credentials); `httpx.NewServer` chặn đăng ký route tiền tố `/internal/` — file: `src/backend/pkg/httpx/auth.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Tenant của phiên được kiểm tra chéo ở mọi request (Priority: P1)

**Goal**: Middleware `tenant`/`principal`, `TenantScope`, `Bypass`, RLS helper, cache Principal.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T015 [P] [US2] Test F002-AC-06: thiếu header, sai UUID, route miễn trừ (`tenant_header_required`), header tùy chọn — file: `src/backend/pkg/httpx/tenant_test.go` — phải FAIL trước khi code
- [x] T016 [P] [US2] Test F002-AC-07, F002-AC-08: membership lệch → `TENANT_MISMATCH` + audit + metric; `TenantID` lấy từ resolver; `tenant_ids` bị bỏ qua với người dùng tenant — file: `src/backend/pkg/httpx/principal_test.go`
- [x] T017 [P] [US2] Test F002-AC-09: resolver lỗi + cache trống → 503 retryable; cache còn hạn → thành công — file: `src/backend/pkg/authz/cache_failclosed_test.go`
- [x] T018 [P] [US2] Test F002-AC-10: L1 + Valkey hit/miss, `Invalidate` xóa cả hai (testcontainers valkey:8) — file: `src/backend/pkg/authz/cache_test.go`
- [x] T019 [P] [US2] Test F002-AC-11: `ScopeFor` một tenant; `CrossTenantScope` cần quyền + MFA + lý do; `PlatformScope(p)`/`SystemPlatformScope(job)` chỉ thấy dòng `tenant_id IS NULL`; depguard chặn gói khác dùng `scopector` — file: `src/backend/pkg/authz/scope_test.go`
- [x] T020 [P] [US2] Test F002-AC-12: `Bypass` lý do rỗng hoặc ngoài danh sách đóng lỗi; lý do hợp lệ ghi audit `cross_tenant.accessed` + tăng `eduworld_tenant_filter_bypass_total{module,reason}` — file: `src/backend/pkg/tenancy/bypass_test.go`
- [x] T021 [P] [US2] Integration test F002-AC-13: áp SQL sinh cho bảng T/TA/T\*/H trên postgres:17, kiểm 0 dòng khi không set/tenant khác (TC-TEN-005); assert không policy nào chứa `deleted_at` — file: `src/backend/pkg/tenancy/rls_integration_test.go`
- [x] T056 [P] [US2] Integration test F002-AC-29: bảng T có policy + view `_live` do helper sinh; `UPDATE ... SET deleted_at = now() ... RETURNING` bằng role `_app` thành công; dòng không còn trong truy vấn `deleted_at IS NULL` và view `_live`; role `_readonly` đọc bảng gốc bị từ chối — file: `src/backend/pkg/tenancy/rls_softdelete_integration_test.go`
- [x] T057 [P] [US2] Test F002-AC-30: fixture gọi `tenancy.Bypass` với lý do ngoài danh sách → analyzer `bypassreason` fail; test API surface (`apidiff`/golden) của `src/backend/pkg/tenancy`, `src/backend/pkg/authz` — file: `src/backend/tools/lint/bypassreason/analyzer_test.go`

### Implementation cho User Story 2

- [x] T022 [P] [US2] `tenancy.TenantScope`, `CrossTenantScope`, `PlatformScope` (người hoặc job), context helpers, constructor nội bộ `tenancy/internal/scopector` — file: `src/backend/pkg/tenancy/scope.go`
- [x] T023 [P] [US2] `tenancy.Bypass`/`Bypassed`, kiểu `BypassReason` danh sách đóng (ADR-0089 §3), audit `cross_tenant.accessed` + metric `eduworld_tenant_filter_bypass_total{module,reason}` — file: `src/backend/pkg/tenancy/bypass.go`
- [x] T024 [P] [US2] `tenancy.RLSPolicies(table, kind)` (T/TA/T\*/TA\*/H, chỉ điều kiện tenant) và `tenancy.LiveView(table)` theo DATABASE_DESIGN §4 — file: `src/backend/pkg/tenancy/rls.go`
- [x] T058 [P] [US2] Analyzer `bypassreason` (go/analysis) chặn lý do `Bypass` ngoài danh sách, đăng ký vào `golangci.base.yml` — file: `src/backend/tools/lint/bypassreason/analyzer.go`
- [x] T025 [US2] `authz.Principal`, `PrincipalResolver`, `ErrNotMember`, `ScopeFor`, `CrossTenantScope`, `PlatformScope(p)`, `SystemPlatformScope(job)` — file: `src/backend/pkg/authz/principal.go`
- [x] T026 [US2] Cache hai lớp `principal_cache`/`principal_cache_ttl`, key `ew:<module>:principal:<user>:<tenant>`, `Invalidate` — file: `src/backend/pkg/authz/cache.go`
- [x] T027 [US2] Middleware `tenant` (parse, miễn trừ) và `principal` (resolve, ánh xạ lỗi, audit) — file: `src/backend/pkg/httpx/tenant.go`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Permission catalog trong code và chốt chặn theo loại client (Priority: P1)

**Goal**: Catalog + `ValidateCatalog`, guard `portal_staff`, `game_player`, platform MFA, `Require`.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T028 [P] [US3] Test F002-AC-14, F002-AC-15: catalog hợp lệ qua; role tenant chứa từng loại permission platform-only (game, competition.config, platform, moderation, chat platform) → lỗi (TC-CMS-AUTHZ-04) — file: `src/backend/pkg/authz/validate_test.go` — phải FAIL trước khi code
- [x] T029 [P] [US3] Test F002-AC-16, F002-AC-17: ma trận `azp` × `user_type` × nhóm route — file: `src/backend/pkg/httpx/guards_test.go`
- [x] T030 [P] [US3] Test F002-AC-18: thiếu MFA (`acr`), `auth_time` quá hạn, không role platform — file: `src/backend/pkg/httpx/platform_guard_test.go`
- [x] T031 [P] [US3] Test F002-AC-19: `Require` trả 404 ngoài tenant, `PERMISSION_DENIED` cả khi ngoài school/subject lẫn khi thiếu quyền (TC-TEN-002, TC-TEN-003) — file: `src/backend/pkg/authz/require_test.go`

### Implementation cho User Story 3

- [x] T032 [US3] Hằng permission theo ADMIN_PORTAL §2.2 + danh sách platform-only — file: `src/backend/pkg/authz/catalog.go`
- [x] T033 [US3] Role hệ thống theo ADMIN_PORTAL §2.3 (`RoleDef{Name, ScopeKind, Permissions}`) + `ValidateCatalog`; hook `authz.MustValidateAtStartup()` gọi trong `src/backend/cmd/*` của mọi binary — file: `src/backend/pkg/authz/roles.go`
- [x] T034 [US3] Guard `portal_staff`, `game_player`, `platform_mfa` (đọc `EW_OIDC_PLATFORM_REQUIRED_ACR`, `EW_OIDC_PLATFORM_MAX_AUTH_AGE`) — file: `src/backend/pkg/httpx/guards.go`
- [x] T035 [US3] `authz.Require(ctx, p, perm, resource)` + `Resource{TenantID, SchoolID, SubjectID}` — file: `src/backend/pkg/authz/require.go`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Lỗi, cấu hình và quan sát thống nhất (Priority: P2)

**Goal**: `problem`, `config`, `observability`, validator request.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T036 [P] [US4] Test F002-AC-20: `problem.Map` theo Kind, lỗi lạ → 500 không lộ chi tiết, có `trace_id` — file: `src/backend/pkg/problem/problem_test.go` — phải FAIL trước khi code
- [x] T037 [P] [US4] Test F002-AC-21: `gen-check` phát hiện mã lệch spec — file: `src/backend/pkg/problem/gen/gen_test.go`
- [x] T038 [P] [US4] Test F002-AC-22: thiếu biến → exit 1 + tên biến, không giá trị; đọc `*_FILE` — file: `src/backend/pkg/config/load_test.go`
- [x] T039 [P] [US4] Test F002-AC-23: log capture với header Authorization, body có `password`, email → không xuất hiện (TC-AUTH-060 phần backend) — file: `src/backend/pkg/observability/redact_test.go`
- [x] T040 [P] [US4] Test F002-AC-24: exporter in-memory có span server + `traceparent` lan truyền; metric có nhãn chuẩn, không user_id — file: `src/backend/pkg/observability/otel_test.go`
- [x] T041 [P] [US4] Test F002-AC-25: validator kin-openapi trả 422 `MAX_LENGTH` với tên trường (TC-TEN-024 phần API) — file: `src/backend/pkg/httpx/validate_test.go`

### Implementation cho User Story 4

- [x] T042 [P] [US4] `src/backend/pkg/problem` (RFC 9457, `Kind`, `Map`, `Invalid(field, code)`) — file: `src/backend/pkg/problem/problem.go`
- [x] T043 [P] [US4] `src/backend/pkg/config` (koanf env, `*_FILE`, `Required`) — file: `src/backend/pkg/config/load.go`
- [x] T044 [P] [US4] `src/backend/pkg/observability` (slog JSON + redaction, OTel tracer/meter, Prometheus registry nhãn `binary`, `module`, metric HTTP + DB pool) — file: `src/backend/pkg/observability/logger.go`
- [x] T045 [US4] `httpx.NewServer` ráp chuỗi middleware theo plan, validator kin-openapi theo spec của module, timeout biên — file: `src/backend/pkg/httpx/server.go`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Kiến trúc được kiểm bằng máy và test cô lập tenant sinh tự động (Priority: P2)

**Goal**: Lint tầng + `routematrix`.

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [x] T046 [P] [US5] Test F002-AC-26, F002-AC-27: fixture module có import cấm → `golangci-lint`/`go-arch-lint` fail — file: `src/backend/tools/lint/testdata/badsvc/internal/domain/bad.go` — phải FAIL trước khi code
- [x] T047 [P] [US5] Test F002-AC-28: `routematrix check` fail khi operation thiếu ca tenant A/B hoặc ca role thiếu quyền — file: `src/backend/tools/routematrix/check_test.go`

### Implementation cho User Story 5

- [ ] T048 [US5] `.golangci.yml` + `.go-arch-lint.yml` cho 6 module kế thừa base — file: `src/backend/modules/learning/.golangci.yml`
- [x] T049 [US5] `routematrix`: đọc spec gộp (`x-service`, `X-Tenant-Id`, path param ID) + internal, xuất `src/backend/test/authz/matrix_gen.json`, lệnh `check` so với test đã đăng ký qua `authztest.Register` — file: `src/backend/tools/routematrix/main.go`
- [x] T050 [US5] Harness `authztest`: fixture Principal theo role (tenant A/B, platform, học sinh), helper gọi handler và kỳ vọng 404/403 — file: `src/backend/pkg/authz/authztest/harness.go`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [ ] T051 [P] Sửa tài liệu: SECURITY_ARCHITECTURE §5.2 đường dẫn `src/backend/pkg/authz/catalog.go`; MULTITENANCY §4.1/§4.3 key cache `ew:<module>:principal:`; TEST_STRATEGY TC-SVC-006 kỳ vọng `CALLER_NOT_ALLOWED` — file: `docs/10-security/SECURITY_ARCHITECTURE.md`
- [ ] T052 [P] Ghi danh sách permission platform-only mở rộng (moderation/chat) vào SECURITY_ARCHITECTURE §5.2 (bỏ nhãn [Proposed]) — file: `docs/10-security/SECURITY_ARCHITECTURE.md`
- [ ] T053 [P] CHANGELOG mục F002 — file: `CHANGELOG.md`
- [ ] T054 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/002-platform-kit-core/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 (cần claims của US1) → US3 (cần Principal của US2) → US4 (song song US2/US3 được) → US5 → Polish.
- T005 cần review chủ sở hữu identity (F012); F003 bắt đầu sau US2 + US4.

## Definition of Done

- [ ] Mọi tiêu chí F002-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Ghi chú triển khai (2026-11-17)

- Test gộp file: T009 (`auth_expiry_test.go`) và T011 (`no_internal_route_test.go`) nằm trong `src/backend/pkg/httpx/auth_test.go`; T016 (`principal_test.go`) trong `tenant_test.go`; T030 (`platform_guard_test.go`) trong `guards_test.go`.
- `authz.Grant` mang `ScopeKind` (role_scope_kind) để guard `platform_mfa` nhận ra role platform kể cả role platform tùy biến; `Principal` = `PrincipalData` (dữ liệu resolve) + tập permission tính sẵn; `ClientID`, `SessionID`, `Channel`, `MFA` lấy từ token mỗi request, không cache.
- Client id portal/game đọc từ `EW_KC_PORTAL_CLIENT_ID`/`EW_KC_GAME_CLIENT_ID` (R3), không hardcode.
- Cache Principal: `ew:<module>:principal:<user_id>:<tenant_id|platform>` (VALKEY_DESIGN) + khóa phụ `…:principal:sub:<hash(iss,sub)>` → `user_id` (token không mang `user_id`) + `…:principal:idx:<user_id>` (SET) cho invalidate toàn user.
- T045: `httpx.NewServer` + validator kin-openapi (spec gộp OpenAPI 3.1, mọi operation đăng ký phải có trong spec) xong; `bootstrap` dùng logger/OTel/metric của `pkg/observability` cho mọi binary. Nối `httpx.Server` + `PrincipalResolver` thật vào `src/backend/cmd/core-api` thuộc F003 T048 (adapter facade identity) — trước đó core-api không có route ứng dụng.
- Compose app xuất trace OTLP/HTTP (`otel-collector:4318`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`).
- Sửa lỗi YAML trong spec gộp: `TenantCreate.keycloak_org_alias.description` có dấu phẩy không quote trong flow mapping tạo key thừa (kin-openapi `doc.Validate` phát hiện); sinh lại `identity.v1.yaml`.
- T046/T003: luật F002-AC-26/27 được kiểm bởi `tools/archgen --check` (`make arch-check`, chạy local không cần golangci-lint) và fixture trong `scripts/ci/test-arch-fixtures.sh` (12/12); depguard sinh trong `.golangci.arch.yml` vẫn là lớp phụ cho CI.
- T057/T058: analyzer `bypassreason` + `scopeguard` chạy bằng `go run ./tools/lint/ewlint` (multichecker, trong `make lint` và `make arch-check`) thay vì plugin golangci; API surface `pkg/tenancy`, `pkg/authz` khóa bằng golden `tools/lint/apisurface/testdata/*.api`.
- T049/T050: `routematrix gen` sinh `src/backend/test/authz/matrix_gen.json` (484 operation, ca bắt buộc `role_denied` cho mọi operation có xác thực, thêm `cross_tenant` khi có `X-Tenant-Id` hoặc id trong path); `routematrix check` quét AST: operation "đã cài đặt" = literal `httpx.Operation{ID: …}` trong file không phải test dưới `modules/`, `cmd/`; ca được phủ = lời gọi `authztest.Cover(t, "<op>", authztest.CrossTenant|RoleDenied)` với literal (thay cho `authztest.Register` lúc chạy, để CI kiểm tĩnh). Operation facade (spec internal) do facade test (`make test-contract`) phủ.
