# Implementation Plan: F056 — Workspace C — School Learning Dashboard và báo cáo

**Branch**: `056-portal-school-dashboard` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Xây module `dashboard` của workspace C: trang tổng quan trường (5 widget theo 5 câu hỏi), drill-down lớp với heatmap LO, hồ sơ học sinh, tab kết quả course, tab phân tích đề/bài, trung tâm xuất báo cáo bất đồng bộ, tổng hợp tenant (Tenant Admin) và platform (System Admin). Mọi số liệu đến từ API dashboard đọc rollup (ADR-0062, F043/F044); Portal áp quy tắc UI chung [ADMIN_PORTAL §3.14.1](../../docs/04-admin-portal/ADMIN_PORTAL.md): bộ chọn khoảng ≤ 2 năm, nhãn độ hạt, "Cập nhật lúc", "Tạm tính", widget tải độc lập có giới hạn song song, xử lý `DASHBOARD_*`. Component dùng chung (`DashboardRangePicker`, `DashboardWidget`, `MasteryHeatmap`, `ExportButton`) đặt ở `ops-ui` để F080 dùng lại.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) |
| Primary Dependencies | Ant Design 5 + Pro (StatisticCard, ProTable, Descriptions, Tabs, Timeline), ECharts (heatmap, chuỗi thời gian), TanStack Query, openapi-typescript, i18next, dayjs `vi` |
| Storage | Không; per-viewer `localStorage` chỉ nhớ bộ lọc khối/môn/khoảng gần nhất |
| Testing | Vitest + Testing Library + MSW (test-only, nhãn `MOCK`), Playwright E2E với backend compose (golden dataset F043), axe, visual regression |
| Target Platform | Trình duyệt desktop/tablet đầy đủ; điện thoại xem thẻ tóm tắt ([ADMIN_PORTAL §1.5.1](../../docs/04-admin-portal/ADMIN_PORTAL.md)) |
| Project Type | web app (workspace C) + component `web/packages/ops-ui` |
| Performance Goals | NFR-PERF-017, NFR-PERF-018; ≤ `dashboard_max_concurrent_per_user` truy vấn đồng thời mỗi trang |
| Constraints | AGENTS R1, R7, R8, R14, R16, R32; ADR-0047, ADR-0062, ADR-0065, ADR-0072 |
| Scale/Scope | Trường 1.500 học sinh, lớp 45 học sinh, khoảng ≤ `dashboard_max_range_days` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec Ready; 3 delta contract (hồ sơ học sinh, phân tích đề/bài, widget tự học) là task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ gọi query API của module learning qua APISIX |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id`; cache client khóa theo tenant; test F056-AC-21 |
| IV Server-authoritative | ✅ | Không tự tính chỉ số/ngưỡng; hiển thị số server trả |
| V Xác thực, phân quyền | ✅ | Menu theo `analytics.*`, `report.export`; 403 xử lý; test ma trận |
| VI An toàn trẻ em | ✅ | Không chat, không Game Rating/bạn bè/IP; ngôn ngữ trung tính, không xếp hạng |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn khoảng/độ hạt theo tên tham số; server trả `grain` |
| VIII UI tách logic, token | ✅ | Hook gọi API; chart/heatmap màu từ `admin-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F056-AC có test trước khi code; MSW chỉ trong test |

## Project Structure

### Documentation (feature này)

```text
specs/056-portal-school-dashboard/
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
web/packages/ops-ui/src/dashboard/
├── DashboardRangePicker.tsx     # preset + giới hạn dashboard_max_range_days, độ hạt thô hơn
├── DashboardWidget.tsx          # skeleton, "Cập nhật lúc", "Tạm tính", "Đang tính…", lỗi DASHBOARD_*
├── MasteryHeatmap.tsx           # ECharts + ký hiệu ○◔◑◕●, aria
├── SuppressedCell.tsx           # "< 5" + tooltip — component dùng chung duy nhất (DESIGN_SYSTEM §4), F080 dùng lại (gồm màn "Hoạt động game" workspace C)
├── ExportButton.tsx             # tạo job, theo dõi, link + hạn
├── useDashboardQueue.ts         # giới hạn truy vấn song song/trang, retry DASHBOARD_BUSY 2 lần
└── *.stories.tsx
web/apps/portal/src/workspaces/school-lms/dashboard/
├── routes.tsx                   # /portal/school-lms/dashboard/{overview,classes/:id,students/:id,courses/:id/results,analysis/:id,reports,tenant,platform}
├── api/{schoolDashboard,classDashboard,studentProfile,courseResults,itemAnalysis,reports,summaries}.ts
├── overview/OverviewPage.tsx + widgets/{AssignmentsWidget,WeakObjectivesWidget,AttentionWidget,SelfStudyWidget,SafetyWidget}.tsx
├── class/ClassDashboardPage.tsx
├── student/StudentProfilePage.tsx (+ tabs: Mastery, History, Recommendations, LoginActivity)
├── results/CourseResultsTab.tsx · analysis/ItemAnalysisTab.tsx
├── reports/ReportsCenterPage.tsx · RegulationDisclaimer.tsx
├── summary/{TenantSummaryPage,PlatformSummaryPage}.tsx
├── i18n/{vi,en}.json
└── __tests__/
web/apps/portal/e2e/dashboard/*.spec.ts
```

**Structure Decision**: Component dashboard dùng chung đặt ở `ops-ui` (R32, F080 tái dùng); màn nghiệp vụ trong chunk workspace C.

## Phase 0: Research

Không có. Chọn ECharts theo [SCHOOL_LMS §8.7](../../docs/04-admin-portal/SCHOOL_LMS.md); quy tắc dữ liệu đã chốt ở [DASHBOARD_DATA](../../docs/07-backend/DASHBOARD_DATA.md).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — | Không có |
| application | Hook `api/*.ts` qua `useDashboardQueue` | Query key `[tenantId, queryId, scope, from, to, grain]` |
| infrastructure | `apiClient` (F050), client sinh | |
| presentation | Page/widget gọi `getSchoolDashboard`, `getClassDashboard`, `getSchoolAnalyticsSeries`, `getCourseResults`, `listTenantStudentLoginEvents`, `createLmsReport`, `getLmsReport`, `getTenantLearningSummary`, `createTenantReport`, `getTenantReport`, `getPlatformLearningSummary`, `createPlatformReport`, `getPlatformReport`; sau delta: `getStudentLearningProfile`, `getExamSessionReport`, `getAssignmentItemAnalysis` | |

### Ánh xạ widget → dữ liệu

| Widget | Trường `SchoolDashboard` / API |
|---|---|
| ① Bài giao đang mở/sắp hạn | `assignments_open`, `assignments_due_soon`, `assignment_completion_rate`, `classes[].assignment_completion_rate` |
| ② LO cần củng cố | `weak_objectives[]` (+ nút Bản đồ kỹ năng → `getClassDashboard.objective_distribution`) |
| ③ Cần chú ý | `students_needing_attention[]` (`reason`) |
| ④ Tự học & gợi ý | `active_minutes_series` + trường delta `self_study` |
| ⑤ An toàn | `safety_notifications_unacknowledged` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F043 (dashboard API), F044 (export, item analysis), F052 (khung workspace C, login events), F055 (wizard nhận tham số), F057 (bộ lọc tenant), F050/F051.
- Event: không (tiến độ gần realtime do server; Portal làm mới khi focus và theo `school_dashboard_refresh`).

### Bảo mật, tenant, idempotency, audit

- Permission: `analytics.school.read`, `analytics.class.read`, `analytics.tenant.read`, `report.export`, `lms.student_activity.read` (tab đăng nhập), `lms.assignment.write` (nút giao ôn tập); System Admin: platform summary (gateway nội bộ, MFA).
- Idempotency-Key cho `createLmsReport`, `createTenantReport`, `createPlatformReport`; audit `report.exported`, `cross_tenant.accessed` ghi ở backend.

### Quan sát

- Log client: `query_id`, `code`, `trace_id`, thời gian tải widget (không họ tên học sinh); Web Vitals trang tổng quan gửi theo cơ chế chung của portal.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.school_lms.school_dashboard`, `menu.school_lms.results`, `menu.school_lms.reports`, `menu.platform_admin.reports` |
| Action dùng | `action.school_lms.school_dashboard.view`, `action.school_lms.results.view`, `action.school_lms.reports.view`, `action.school_lms.reports.export`, `action.platform_admin.reports.view`, `action.platform_admin.reports.export`, `action.school_lms.assignments.view`, `action.school_lms.student_activity.view` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `getStudentLearningProfile`, `getExamSessionReport`, `getAssignmentItemAnalysis` (Cần thêm) → cần `x-portal-action` (đề xuất `action.school_lms.school_dashboard.view`, `action.school_lms.results.view`)

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
