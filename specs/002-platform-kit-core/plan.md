# Implementation Plan: F002 — Platform kit lõi: config, observability, problem, httpx, authz, tenancy

**Branch**: `002-platform-kit-core` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Bổ sung vào module `pkg` (tạo ở F001) các gói `config`, `observability`, `problem`, `httpx`, `authz`, `tenancy`, `audit` (interface), `clock`, `ids` và công cụ `tools/routematrix`, cấu hình lint kiến trúc. `httpx` cung cấp server chi với chuỗi middleware chuẩn; `authz` sở hữu catalog permission, Principal, guard và tạo `TenantScope`; `tenancy` giữ scope trong context, `Bypass` có audit + metric và helper sinh SQL RLS. Không có ent/pgx trong các gói này (ent ở F003).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, `coreos/go-oidc/v3` + `go-jose/v4`, `knadh/koanf`, `log/slog`, `go.opentelemetry.io/otel` (+ `otelhttp`), `prometheus/client_golang`, `getkin/kin-openapi` (openapi3filter), `valkey-io/valkey-go`, `hashicorp/golang-lru/v2` |
| Storage | Valkey (cache Principal, ADR-0004); không có bảng |
| Testing | `go test`, `testify`, `httptest`, JWKS giả (`go-jose`), OTel in-memory exporter, testcontainers postgres:17 (helper RLS), valkey:8 |
| Target Platform | Thư viện Go dùng trong Linux container |
| Project Type | Platform kit |
| Performance Goals | Verify JWT < 1 ms; Principal cache hit ≥ 95% ([BACKEND_DESIGN §6.1](../../docs/07-backend/BACKEND_DESIGN.md)) |
| Constraints | AGENTS R3, R7, R8, R13, R14, R18, R20, R23, R27 · ADR-0009, ADR-0023, ADR-0027, ADR-0038, ADR-0041, ADR-0065, ADR-0066 |
| Scale/Scope | 6 module trong 3 binary dùng chung; 10.000 CCU (`target_ccu`) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta contract `resolvePrincipal` ghi ở contracts §6, task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | `pkg/` không import `modules/`; `authz`/`problem`/`tenancy` không biết ent/pgx; lint tầng do feature này cung cấp |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `TenantScope` chỉ từ `authz`; `Bypass` có audit; helper RLS; test TC-TEN-005 |
| IV Server-authoritative | ✅ | Không áp dụng |
| V Xác thực, phân quyền | ✅ | Verify JWT độc lập gateway; guard `portal_staff`, `game_player`, platform MFA; catalog chặn quyền platform cho role tenant |
| VI An toàn trẻ em | ✅ | Redaction log, không PII trong span/metric label |
| VII Một nguồn sự thật, cấu hình | ✅ | Mã lỗi sinh từ `ProblemCode`; tham số theo tên CONFIG_DEFAULTS; issuer/audience từ env |
| VIII UI tách logic, token | ✅ | Không áp dụng |
| IX Test có bằng chứng | ✅ | Mỗi F002-AC-* có test; `routematrix` bắt buộc test tenant/role cho route mới |

## Project Structure

### Documentation (feature này)

```text
specs/002-platform-kit-core/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
pkg/
├── config/          load.go (koanf env + *_FILE), required.go
├── observability/   logger.go (slog JSON, redaction), otel.go, metrics.go, redact.go
├── problem/         problem.go (RFC 9457, Kind→HTTP), codes_gen.go (sinh từ ProblemCode), map.go
├── clock/           clock.go          ids/ ids.go (UUIDv7)
├── audit/           recorder.go (interface Recorder, Event)
├── authz/           principal.go permission.go catalog.go roles.go validate.go scope.go require.go
│                    resolver.go (port PrincipalResolver) cache.go (L1 + Valkey) authztest/
├── tenancy/         scope.go (TenantScope, context) bypass.go rls.go (sinh SQL policy)
└── httpx/           server.go chain.go auth.go tenant.go principal.go guards.go
                     validate.go (kin-openapi) problem.go recover.go requestid.go log.go otel.go
tools/routematrix/   main.go (đọc specs/openapi → testdata matrix + kiểm thiếu test)
tools/lint/          golangci.base.yml  go-arch-lint.base.yml  (module include)
modules/<module>/.golangci.yml  .go-arch-lint.yml  (kế thừa base)
```

**Structure Decision**: Theo BACKEND_DESIGN §2 và D4/D5: catalog ở `pkg/authz`, scope/bypass ở `pkg/tenancy`. `audit.Recorder` là interface để `httpx`/`authz` ghi audit mà không phụ thuộc outbox (hiện thực ở F003).

## Phase 0: Research

Không có. Thư viện đã chốt ở ADR-0025; chuỗi middleware ở BACKEND_DESIGN §6, API_DESIGN §11, MULTITENANCY §4.3.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — | `pkg/clock`, `pkg/ids` là gói thuần được domain phép dùng |
| application | `authz.Require`, `authz.Permission*` hằng, `problem.*` | Gói application được phép import (BACKEND_DESIGN §3) |
| infrastructure | `authz.ValkeyPrincipalCache`, JWKS cache | Không ent |
| presentation | `httpx.NewServer(cfg, opts...)`, middleware | Dùng ở `cmd/core-api` |

**Chuỗi middleware public** (thứ tự): `requestid` → `recover` → `otel` → `log` → `problem` → `auth` → `tenant` (parse header, miễn trừ) → `principal` (resolve, mismatch → `TENANT_MISMATCH`) → guard route (`portal_staff` | `game_player` | `platform_mfa`) → `validate` (kin-openapi, maxLength) → handler. Không có chuỗi middleware internal: lời gọi giữa module đi qua facade trong tiến trình (`pkg/modcall`, F003), không qua HTTP (ADR-0074 mục 3). Route gateway nội bộ (`/admin/*`, `/sis/*`, `/game-admin/*`) dùng chung chuỗi trên với guard `portal_staff`/`platform_mfa`.

**API chính**:

- `authz.Principal` theo BACKEND_DESIGN §6 (thêm `ACR`, `AuthTime`); `authz.PrincipalResolver` interface `Resolve(ctx, ref authz.SubjectRef, tenantID *uuid.UUID, clientID string) (Principal, error)` với `SubjectRef{Issuer, Subject}` (khớp `PrincipalResolveRequest` do F012 sở hữu: `issuer` + `subject`, `tenant_id` nullable, `client_id`); `authz.ErrNotMember` → `TENANT_MISMATCH`.
- `authz.Catalog` (permission constants sinh từ bảng ADMIN_PORTAL §2.2, `PlatformOnlyPrefixes = {"game.","competition.config.","platform."}` + danh sách platform-only tường minh), `RoleDefs` (§2.3), `ValidateCatalog(RoleDefs) error`.
- `authz.ScopeFor(p) (tenancy.TenantScope, error)`, `authz.CrossTenantScope(p, ids, reason)`, `authz.PlatformScope(p)` và `authz.SystemPlatformScope(job)` (chỉ dòng `tenant_id IS NULL`); `tenancy.newScope` không export, `authz` gọi qua `tenancy/internal/scopector` (depguard chỉ cho `pkg/authz`). API công khai `TenantScope`, `CrossTenantScope`, `PlatformScope`, `SystemPlatformScope`, `Bypass`, `BypassReason`, `Principal` khóa từ F002 (test API surface), đổi chữ ký cần ADR (ADR-0089 §3).
- `tenancy.Bypass(ctx, reason BypassReason) (context.Context, error)` với `BypassReason` là danh sách đóng `system_admin_cross_tenant`, `moderation_case`, `break_glass`, `trash`, `auth_login`, `tenant_clone`, `inactive_cleanup`, `re_evaluation`, `efficacy_study`, `platform_job` (mỗi hằng số khai báo kèm quyền DB theo bảng MULTITENANCY §4.4; ghi `audit.Recorder` action `cross_tenant.accessed` với `reason`, `tenant_ids`, `mode` + metric `eduworld_tenant_filter_bypass_total{module,reason}`; analyzer trong `tools/lint` cấm giá trị ngoài danh sách); `tenancy.Bypassed(ctx) bool`; `tenancy.RLSPolicies(table string, kind TableKind) string` sinh SQL theo DATABASE_DESIGN §4 (`TableKind` = T, TA, T\*, H; policy chỉ điều kiện tenant, không `deleted_at`); `tenancy.LiveView(table string) string` sinh view `<table>_live` + `GRANT SELECT` cho `<module>_readonly` (ADR-0089 §1).

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F001 · Feature tiêu thụ: F003 (entx dùng `tenancy`, recorder audit), F004 (cùng danh sách miễn header), mọi module.
- Facade: không gọi (adapter `resolvePrincipal` bọc facade identity ở F003) · Event: không trực tiếp (invalidate qua F003).

### Bảo mật, tenant, idempotency, audit

- Permission: catalog đầy đủ ADMIN_PORTAL §2.2 · `CrossTenantScope` chỉ với `platform.cross_tenant.read` + MFA.
- Audit action: `authz.denied` (reason `TENANT_MISMATCH` | `PORTAL_ACCESS_STAFF_ONLY` | `GAME_ACCESS_STUDENT_ONLY` | `PERMISSION_DENIED` | `MFA_REQUIRED`), `cross_tenant.accessed` (mọi lần `Bypass` hoặc `CrossTenantScope`, metadata `reason`, `tenant_ids`, `mode`, ADR-0089) — qua `audit.Recorder`.
- Không lưu/log token; Problem không chứa chi tiết nội bộ.

### Quan sát

- Log: trường BACKEND_DESIGN §11 + `layer`; redaction header `Authorization`, `Cookie`, `X-Device-Id` đầy đủ, khóa JSON `password|token|refresh_token|body`.
- Metric: `eduworld_http_server_duration_seconds`, `eduworld_http_server_requests_total{binary,module,route,method,status_class,code}`, `eduworld_auth_tenant_mismatch_total`, `eduworld_tenant_filter_bypass_total{module,reason}`, `eduworld_principal_cache_hits_total{layer,result}`, `eduworld_jwks_refresh_total{result}`.
- Span: `http.server` (otelhttp), `authz.resolve_principal`; thuộc tính `enduser.pseudo_id` thay user_id (MONITORING §7).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
