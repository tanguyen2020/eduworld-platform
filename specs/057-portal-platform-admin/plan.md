# Implementation Plan: F057 — Workspace D — Platform Administration: tenant, gán/clone, thùng rác, danh mục, cấu hình hệ thống

**Branch**: `057-portal-platform-admin` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Xây workspace D (chunk lazy-load `src/workspaces/platform-admin`) chỉ cho System Admin, gọi API `/api/v1/admin/*` qua gateway nội bộ với token MFA: Tenant, Gán cho tenant (hộp thoại + tab dùng chung), Clone giữa tenant (wizard + theo dõi job), Thùng rác mọi tenant, Danh mục (quốc gia, tỉnh/thành, kế thừa, trường, import CSV), Cấu hình hệ thống (dọn học sinh, lần dọn chờ duyệt, thời hạn chat gameplay). Thêm `TenantFilter` (ops-ui) và hook `useCrossTenantScope` để mọi workspace áp quy ước liên tenant (FR-CMS-013): list gửi `tenant_ids[]` không có `X-Tenant-Id`, ghi gửi `X-Tenant-Id` tenant đích. Bốn delta contract đưa vào Phase 2.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) |
| Primary Dependencies | Ant Design 5 + Pro (ProTable, ProForm, StepsForm, Upload, Result), TanStack Query, openapi-typescript, React Hook Form + Zod, i18next |
| Storage | Không; `sessionStorage` cho lựa chọn bộ lọc tenant (per-viewer) |
| Testing | Vitest + Testing Library + MSW (test-only, nhãn `MOCK`), Playwright E2E với backend compose (gateway nội bộ local) |
| Target Platform | Trình duyệt desktop/tablet ≥ 1024 px cho gán/clone/tenant; mobile chỉ đọc |
| Project Type | web app (workspace D) + component `web/packages/ops-ui` |
| Performance Goals | `TenantFilter` tìm kiếm debounce 300 ms, phân trang cursor |
| Constraints | AGENTS R3, R7, R8, R10, R20, R21, R22, R28, R32; ADR-0027, ADR-0028, ADR-0029, ADR-0032, ADR-0060, ADR-0065 |
| Scale/Scope | Hàng trăm tenant; Thùng rác/danh mục phân trang cursor |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `createTenant`, `updateTenant`, `previewTenantClone`, `cancelStudentCleanupRun` là task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Gọi API identity/learning qua APISIX nội bộ; không logic nghiệp vụ ở client |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Quy ước `tenant_ids`/`X-Tenant-Id`; không xóa vĩnh viễn; clone không dữ liệu học sinh |
| IV Server-authoritative | ✅ | Kết quả clone/restore/import do server; Portal hiển thị |
| V Xác thực, phân quyền | ✅ | Chỉ `system_admin`; MFA step-up; "Cần VPN"; test ma trận |
| VI An toàn trẻ em | ✅ | Không hiển thị dữ liệu học sinh ngoài danh sách cần thiết; clone loại trừ dữ liệu học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn từ schema; URL gateway từ env; tham số theo tên |
| VIII UI tách logic, token | ✅ | Hook gọi API; theme `admin-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F057-AC có test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/057-portal-platform-admin/
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
web/packages/ops-ui/src/tenant/
├── TenantFilter.tsx              # Select tìm kiếm, chọn nhiều, "Tất cả tenant", cursor
├── TenantColumn.tsx              # cột Tenant (tên + loại, Trường với unit)
├── TargetTenantField.tsx         # trường bắt buộc "Tenant đích" / "Platform" trong form ghi
└── *.stories.tsx
web/apps/portal/src/shared/tenancy/useCrossTenantScope.ts   # đọc permission + lựa chọn → tham số list / header ghi
web/apps/portal/src/workspaces/platform-admin/
├── routes.tsx                    # /portal/platform-admin/{tenants,assignments,clones,trash,catalog/*,settings/*}
├── api/{tenants,assignments,clones,trash,catalog,schoolImports,cleanup,chatRetention}.ts
├── tenants/{TenantListPage,TenantForm}.tsx
├── assignments/{AssignToTenantsDialog,AssignedTenantsTab}.tsx       # export cho workspace A/B
├── clones/{CloneWizard,CloneJobPage}.tsx
├── trash/TrashPage.tsx
├── catalog/{CountriesPage,ProvincesPage,ProvinceSuccessionDialog,SchoolCatalogPage,SchoolImportWizard}.tsx
├── settings/{StudentCleanupSettingsPage,CleanupRunsPage,ChatRetentionPage,PlatformCurfewSettingsCard}.tsx
├── errors/{VpnRequiredBanner,MfaStepUp}.tsx
├── i18n/{vi,en}.json
└── __tests__/
web/apps/portal/e2e/platform/*.spec.ts
```

**Structure Decision**: `TenantFilter`, `TenantColumn`, `TargetTenantField` ở `ops-ui` vì mọi workspace dùng; `useCrossTenantScope` ở `src/shared` để `apiClient` (F050) và các workspace áp chung quy ước. Màn platform-only trong chunk riêng (ADR-0027 mục 7).

## Phase 0: Research

Không có. Quy ước liên tenant ở [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md), gán/clone/thùng rác ở §3.7–§3.8, danh mục ở §3.16, cấu hình ở §3.10.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — | Không có |
| application | Hook `api/*.ts`, `useCrossTenantScope` | Mutation sinh `Idempotency-Key` một lần/thao tác; job polling (clone, import) dừng khi trạng thái kết thúc |
| infrastructure | `apiClient` (F050) với base URL gateway nội bộ từ env `VITE_EW_INTERNAL_API_BASE_URL` | Không hardcode URL (R3); lỗi mạng → `VpnRequiredBanner` |
| presentation | Page/dialog gọi `listTenants`, `assignToTenants`, `listTenantAssignments`, `unassignFromTenants`, `createTenantClone`, `getTenantClone`, `listTrash`, `restoreTrashItem`, `listCountries`, `createCountry`, `updateCountry`, `listProvinces`, `createProvince`, `updateProvince`, `createProvinceSuccession`, `listCatalogSchools`, `importSchools`, `getSchoolImport`, `getStudentCleanupSettings`, `putStudentCleanupSettings`, `listStudentCleanupRuns`, `approveStudentCleanupRun`, `getChatRetentionSettings`, `putChatRetentionSettings`, `getPlatformCurfewSettings`, `putPlatformCurfewSettings`; sau delta: `createTenant`, `updateTenant`, `previewTenantClone`, `cancelStudentCleanupRun` | |

### Quy ước liên tenant (`useCrossTenantScope`)

| Trường hợp | Request |
|---|---|
| Người dùng tenant | Header `X-Tenant-Id` = tenant phiên; không thêm `tenant_ids` |
| System Admin, list, "Tất cả tenant" | Không `X-Tenant-Id`, không `tenant_ids` |
| System Admin, list, đã chọn tenant | Không `X-Tenant-Id`; `tenant_ids[]` = lựa chọn |
| System Admin, ghi | `X-Tenant-Id` = tenant đích từ `TargetTenantField` (hoặc không có với phạm vi Platform); thiếu → chặn submit |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F045 (gán/clone/thùng rác/đọc liên tenant), F010 (tenant, catalog), F016 (dọn học sinh), F117/F122 (chat retention API hiện có ở identity), F051 (shell, topbar gắn `TenantFilter`), F050.
- Dùng bởi: F053/F054, F055, F056, F079 (hộp gán, `TenantFilter`).

### Bảo mật, tenant, idempotency, audit

- Permission: `platform.cross_tenant.read`, `platform.tenant.manage`, `platform.tenant_data.assign`, `platform.tenant_data.clone`, `platform.trash.read`, `platform.trash.restore`, `platform.catalog.write`, `platform.settings.write`, `platform.student_cleanup.approve`.
- Token `azp = eduworld-portal` + MFA (`acr`); `403 MFA_REQUIRED` → shell F051 mở hộp đăng nhập lại (mật khẩu + OTP qua `portalLogin`, ADR-0091).
- Idempotency-Key: `assignToTenants`, `unassignFromTenants`, `createTenantClone`, `restoreTrashItem`, `importSchools`, `approveStudentCleanupRun`, `createTenant`; audit `tenant_data.assigned/unassigned/cloned`, `content.restored`, `cross_tenant.accessed` ghi ở backend.

### Quan sát

- Log client: `code`, `trace_id`, route; không lý do nhập tay, không tên học sinh; đếm lỗi "Cần VPN" theo phiên.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.platform_admin.tenants`, `menu.platform_admin.tenant_data`, `menu.platform_admin.trash`, `menu.platform_admin.catalog`, `menu.platform_admin.system_settings` |
| Action dùng | `action.platform_admin.tenants.view`, `action.platform_admin.tenant_data.view`, `action.platform_admin.tenant_data.assign`, `action.platform_admin.tenant_data.clone`, `action.platform_admin.trash.view`, `action.platform_admin.trash.restore`, `action.platform_admin.catalog.edit`, `action.platform_admin.catalog.import_schools`, `action.platform_admin.system_settings.view`, `action.platform_admin.system_settings.edit`, `action.platform_admin.system_settings.approve_cleanup`, `action.platform_admin.tenants.edit` |
| Không tắt được | `menu.platform_admin.tenants`, `menu.platform_admin.trash` không tắt được (ADR-0090 mục 2) |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `createTenant`, `updateTenant` (Cần thêm trong OpenAPI) → gắn `x-portal-action: action.platform_admin.tenants.edit` khi thêm (action đã có trong catalog, UI-only tới khi có operation)
- `previewTenantClone`, `cancelStudentCleanupRun` (Cần thêm) → đề xuất `action.platform_admin.tenant_data.clone`, `action.platform_admin.system_settings.approve_cleanup`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
