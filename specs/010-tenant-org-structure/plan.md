# Implementation Plan: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Branch**: `010-tenant-org-structure` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

module identity sở hữu tenant, danh mục (`catalog`), tổ chức (`org`) trong database `eduworld_identity`. Command handler (write context, `UnitOfWork` + outbox) cho tạo/sửa tenant, trường, lớp, enrollment (chuyển lớp/trường) và tạo học sinh tenant chung; query handler (read context, chỉ đọc) cho danh mục và danh sách. Worker có consumer `keycloak-membership` (Organization + membership + realm role) và job `org-membership-reconcile` hằng đêm. Event qua outbox → `EVT_IDENTITY`. Adapter Keycloak Admin (`internal/infrastructure/keycloak`) do feature này tạo, các feature identity sau dùng lại.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go, `pkg/entx`, `pkg/tenancy`, `pkg/outbox`, `pkg/authz`, `pkg/problem` |
| Storage | PostgreSQL `eduworld_identity` (schema `platform`, `catalog`, `org`, `identity`, `infra`); Valkey cache tenant (`ew:identity:tenant:<tenant_id>`) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Keycloak test realm) |
| Target Platform | Linux container (module `identity` api + worker) |
| Project Type | backend module |
| Performance Goals | Danh mục có `ETag`; list cursor; xem [API_DESIGN](../../docs/07-backend/API_DESIGN.md) |
| Constraints | AGENTS R7, R8, R10, R13, R21, R26–R28; ADR-0005, ADR-0026, ADR-0028, ADR-0060, ADR-0063, ADR-0081 |
| Scale/Scope | Số tenant ≤ số trường; Organization = tenant (OQ-03) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta OpenAPI (tenant, trường thuộc tenant, học sinh tenant chung) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module identity; module khác nhận projection qua event `tenant.*`, `school.*`, `class.*`, `enrollment.changed` |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng T có `tenant_id` + RLS + soft-delete; `catalog.*`, `platform.tenants` là G; test isolation mọi endpoint |
| IV Server-authoritative | ✅ | Không có điểm/phần thưởng; ghi quan trọng có `Idempotency-Key` + audit |
| V Xác thực, phân quyền | ✅ | `platform.tenant.manage`, `platform.catalog.write`, `tenant.school.*`, `tenant.class.read`, `lms.class.read`, `school.people.read` |
| VI An toàn trẻ em | ✅ | Tên thật/trường chỉ trả cho chính học sinh và nhân sự trong scope (R16); không log PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Bảng/cột theo DATABASE_DESIGN; danh mục từ `specs/reference-data`; mọi chuỗi `MaxLen` |
| VIII UI tách logic, token | ✅ | Không có UI (portal ở F052/F057) |
| IX Test có bằng chứng | ✅ | Mỗi F010-AC-* có task test; test RLS/isolation/authorization |

## Project Structure

### Documentation (feature này)

```text
specs/010-tenant-org-structure/
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
modules/identity/
├── ent/schema/                      # tenant.go, country.go, province.go, provincesuccession.go,
│                                    # school.go, academicyear.go, class.go, student.go, enrollment.go, user.go
├── migrations/                      # Atlas: bảng + RLS + view identity.tenant_memberships_v + seed catalog/common
├── internal/domain/{tenant,catalog,school,enrollment,user}/
├── internal/application/command/    # CreateTenant, UpdateTenant, CreateTenantSchool, UpdateTenantSchool,
│                                    # ApproveTenantSchool, ImportSchools, TransferStudent, CreateCommonTenantStudents,
│                                    # CreateCountry/UpdateCountry/CreateProvince/UpdateProvince/CreateProvinceSuccession
├── internal/application/query/      # ListTenants, ListCountries, ListProvinces, ListCatalogSchools, ListTenantSchools,
│                                    # GetTenantSchool, ListClasses, ListUnassignedStudents, GetMyStudentProfile, GetStudentProfile
├── internal/application/port/       # TenantRepository, SchoolRepository, EnrollmentRepository, KeycloakAdmin, MembershipReader
├── internal/infrastructure/persistence/  # ent repo, read model SQL (scope + deleted_at)
├── internal/infrastructure/keycloak/     # adapter Admin REST: organizations, members, users, realm roles, sessions
├── internal/presentation/http/public/    # handler sinh từ identity.v1.yaml
├── internal/presentation/consumer/       # keycloak_membership.go
├── internal/presentation/job/            # org_membership_reconcile.go
└── tests/{unit,integration,contract}/
```

**Structure Decision**: Theo BACKEND_DESIGN §3 với tên thư mục `modules/identity/` (CODE_PLAN Bước 0 D1).

## Phase 0: Research

Không có — quyết định đã có trong ADR-0005, ADR-0026, ADR-0060, ADR-0063 và DATABASE_DESIGN. Spike Keycloak Organizations (KEYCLOAK_SSO §2.1 "Cần kiểm chứng") thuộc F004; backend luôn đối chiếu DB nên không chặn feature.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `tenant.Tenant` (loại, trạng thái, alias), `school.School` (tỉnh active, `pending_review`), `enrollment.Enrollment` (một active, đóng/mở cùng lúc), `user.User` (`student`/`staff`) | Invariant: tenant `school` một trường; `common` duy nhất; lớp cùng trường |
| application | Command ở cây trên · Query ở cây trên | Permission theo [contracts](contracts/README.md) §1 |
| infrastructure | ent repository, read model SQL, adapter Keycloak, outbox writer | Client `eduworld-sis-sync` (client credentials) |
| presentation | Handler `listTenants`, `createTenant`*, `updateTenant`*, `listCountries`, `listProvinces`, `listCatalogSchools`, catalog write, `importSchools`, `getSchoolImport`, `listTenantSchools`, `getTenantSchool`, `updateTenantSchool`, `approveTenantSchool`, `listTenantSchoolClasses`, `listUnassignedStudents`, `listClasses`, `getMyStudentProfile`, `getStudentProfile`, `createCommonTenantStudents`* · consumer `keycloak-membership` · job `org-membership-reconcile` | * = Cần thêm |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F003 (entx/`UnitOfWork`, outbox, idempotency), F004 (realm, client service account).
- Event phát: `tenant.created`, `tenant.updated`, `school.updated`, `enrollment.changed`, `student.transferred`, `tenant_membership.changed`, `province.updated`, `class.created`/`class.updated`, `<entity>.deleted`, `audit.recorded`.
- Event tiêu thụ (worker): `tenant.created/updated`, `tenant_membership.changed`, `user_role.changed` (phát bởi F012), `user.status_changed` (F016).

### Bảo mật, tenant, idempotency, audit

- Ghi platform (`/admin/*`): `azp = eduworld-portal` + MFA + role platform, gateway nội bộ; ghi tenant đích qua `X-Tenant-Id`.
- `TenantScope` từ `authz.ScopeFor`; System Admin dùng `CrossTenantScope` + audit `cross_tenant.accessed`.
- `Idempotency-Key` bắt buộc cho `createTenant`, `importSchools`, `createCommonTenantStudents`; audit action `tenant.created`, `tenant.updated`, `school.updated`, `school.approved`, `enrollment.changed`, `user.provisioned`.

### Quan sát

- Metric `eduworld_identity_org_sync_total{op,result}`, `eduworld_auth_common_tenant_total`; span cho mỗi lời gọi Keycloak Admin; log không chứa tên/ngày sinh học sinh.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
