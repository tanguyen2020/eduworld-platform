# Implementation Plan: F081 — Workspace C — sự kiện trường từ template

**Branch**: `081-school-events-game-report` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Frontend: thêm module `school-events/` vào chunk workspace C (`src/cms/frontend/apps/portal/src/workspaces/school-lms/`): danh sách, wizard tạo từ template, sửa `If-Match`, hủy có lý do, tiến độ. Mọi quyết định quyền, `allowed_params`, giới hạn do server; client chỉ ẩn/hiện và map lỗi theo `code`. Backend nhỏ trong module game: query `GetSchoolEventProgress` (bounded context `liveops` của F077, đọc PostgreSQL). Delta contract ở Phase 2. Màn "Hoạt động game" và mở rộng `GetTenantSummary` đã chuyển sang F080 US5 (tách F081, CODE_PLAN §4.1).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) · Go ≥ 1.24 (module game) |
| Primary Dependencies | Ant Design 5 + Pro (ProTable, StepsForm, StatisticCard), TanStack Query, openapi-fetch + `apiClient` (F050), `withTenantScope`/`TenantTargetField` (F051), chart theo DESIGN_SYSTEM §4.4, i18next · ent, `src/backend/pkg/tenancy`, `src/backend/pkg/authz` |
| Storage | Không bảng mới. Đọc `eduworld_game`: `game.event_runs`, `game.event_participation`, `game.community_goals`, `game.community_goal_progress`, `game.community_goal_contributions` (không `ref.classes` — ADR-0095) |
| Testing | Vitest + Testing Library + MSW (chỉ trong test, nhãn `MOCK`), Playwright E2E với backend compose, axe · go test + testcontainers (PostgreSQL) |
| Target Platform | Trình duyệt desktop/tablet (form), điện thoại chỉ xem · Linux container |
| Project Type | web app (workspace C) + mở rộng backend module |
| Performance Goals | Trang danh sách/tiến độ theo budget API portal; không có truy vấn dashboard |
| Constraints | AGENTS R1, R7, R8, R10, R15, R16, R19, R27, R32; ADR-0027, ADR-0029 |
| Scale/Scope | ≤ `school_events_concurrent_per_tenant` sự kiện chạy/tenant; ≤ 9 khối/sự kiện (`SchoolEventCreate.audience.grade_levels`) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `getSchoolEventProgress`, `EventTemplate.*_option_items`, `SchoolEvent.status` là task Phase 2 (T004, T006) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Query mới nằm trong module game (chủ dữ liệu); chỉ đọc (read context, `port/read`, ADR-0087); không gọi module khác |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Query qua ent global filter / SQL tường minh có `tenant_id` (+ `deleted_at IS NULL` cho bảng có cột); test cô lập trường/tenant |
| IV Server-authoritative | ✅ | `allowed_params`, thời lượng, giới hạn đều ở server; portal không tính số |
| V Xác thực, phân quyền | ✅ | `lms.school_event.write` + scope UserRole; middleware `portal_staff` |
| VI An toàn trẻ em | ✅ | Không `student_id`/tên/xếp hạng trong response; từ nhạy cảm không hiển thị |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; `maxLength` từ OpenAPI |
| VIII UI tách logic, token | ✅ | Admin/Ops theme; màu trạng thái từ `shared-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F081-AC có task test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/081-school-events-game-report/
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
src/cms/frontend/apps/portal/src/workspaces/school-lms/
├── school-events/
│   ├── routes.tsx                         # /school-lms/school-events, /new, /:eventId, /:eventId/edit
│   ├── SchoolEventListPage.tsx
│   ├── CreateSchoolEventWizard.tsx        # bước: chọn template → thông tin → đối tượng → nội dung/thưởng → lịch → xác nhận
│   ├── fields/{AllowedParamField.tsx,AudiencePicker.tsx,TemplateOptionPicker.tsx}
│   ├── SchoolEventDetailPage.tsx · EditSchoolEventForm.tsx · CancelSchoolEventDialog.tsx
│   ├── progress/{SchoolEventProgressPanel.tsx,GradeContributionTable.tsx}
│   ├── api/{eventTemplates.ts,schoolEvents.ts,schoolEventProgress.ts}
│   └── mapProblem.ts                      # code → khóa i18n, errors[].field → field form
├── routes.tsx                             # route cho `menu.school_lms.school_events` của catalog (ADR-0090)
src/cms/frontend/apps/portal/src/i18n/locales/{vi,en}/portal.school_lms.json
src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/*.test.tsx
src/cms/frontend/apps/portal/tests/e2e/school-lms/{school-events,school-events-common}.spec.ts
src/backend/modules/game/
├── internal/application/query/liveops/get_school_event_progress.go
├── internal/infrastructure/persistence/liveops/school_event_progress_repo.go
├── internal/presentation/http/public/school_events.go                  # thêm handler getSchoolEventProgress
└── tests/{unit,integration,contract}/liveops/
```

**Structure Decision**: Module theo sitemap workspace C ([ADMIN_PORTAL §10](../../docs/04-admin-portal/ADMIN_PORTAL.md)); thư mục `school-lms` theo F051/F052 (F056 dùng `school/` — mâu thuẫn báo cáo). Backend đặt cạnh code F077 để giữ một chủ cho mỗi bảng.

## Phase 0: Research

Không có — ADR và tài liệu sở hữu đủ; delta contract đã xác định ([contracts/README.md](contracts/README.md)).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md) (không bảng mới; read model)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `liveops.GradeBreakdownPolicy` (dải khối được xem đóng góp theo khối + ẩn khối < 5 học sinh — EVENT_SEASON_SYSTEM §6, ADR-0095) | Hàm thuần, test unit |
| application | Query `GetSchoolEventProgress` (permission `lms.school_event.write`, scope trường) | authz `authz.Require` tường minh đầu handler; tracing `src/backend/pkg/observability`; validation trong DTO/handler |
| infrastructure | `SchoolEventProgressRepo` (ent + SQL tường minh `GROUP BY grade_level` có `tenant_id`) | R27 |
| presentation | Handler `getSchoolEventProgress` · Portal gọi `listEventTemplates`, `listSchoolEvents`, `createSchoolEvent`, `getSchoolEvent`, `updateSchoolEvent`, `cancelSchoolEvent`, `getSchoolEventProgress`, `getMyPermissions` | |

**Màn → operationId → permission (ẩn/hiện; server vẫn kiểm):**

| Màn | Đọc | Ghi | Permission |
|---|---|---|---|
| Sự kiện trường | `listSchoolEvents`, `getSchoolEvent`, `listEventTemplates` | `createSchoolEvent`, `updateSchoolEvent`, `cancelSchoolEvent` | `lms.school_event.write` |
| Tiến độ sự kiện | `getSchoolEventProgress` | — | `lms.school_event.write` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F052 (chunk `school-lms`, menu), F051 (`withTenantScope`, `TenantTargetField`, 403), F050 (`apiClient`, theme), F077 (API sự kiện trường). Không phụ thuộc F080 (màn "Hoạt động game" đã chuyển sang F080 US5).
- Facade: không. Event phát/tiêu thụ: không.

### Bảo mật, tenant, idempotency, audit

- Người dùng tenant: `X-Tenant-Id` = tenant phiên; System Admin: đọc `tenant_ids[]`, ghi qua `TenantTargetField`.
- `Idempotency-Key` (UUIDv7 sinh một lần/ý định, giữ khi retry): `createSchoolEvent`, `cancelSchoolEvent`; `If-Match`: `updateSchoolEvent`. Audit sự kiện trường ở F077; query đọc không ghi audit (trừ System Admin liên tenant: `cross_tenant.accessed` theo F080).

### Quan sát

- Backend: span `query.GetSchoolEventProgress`, không log `student_id`.
- Portal: log `code`, `trace_id`, route; không log nội dung form.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.school_lms.school_events` |
| Action dùng | `action.school_lms.school_events.view`, `action.school_lms.school_events.edit`, `action.school_lms.school_events.cancel` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `getSchoolEventProgress` (Cần thêm) → đề xuất `action.school_lms.school_events.view`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
