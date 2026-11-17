# Data Model: F056 — Workspace C — School Learning Dashboard và báo cáo

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F056 là frontend, không tạo hay sửa bảng. Rollup (`learning.rollup_school_*`, `learning.rollup_class_daily`, `learning.rm_class_objective_stats`, `learning.progress_daily`, `learning.mastery_snapshots_weekly`) và job export thuộc module learning (F043, F044, [DASHBOARD_DATA §2](../../docs/07-backend/DASHBOARD_DATA.md)).

Trạng thái phía client (không bền vững):

| Trạng thái | Nơi giữ | Ghi chú |
|---|---|---|
| Cache truy vấn dashboard | RAM (TanStack Query) | Khóa `[tenantId, queryId, scope, from, to, grain]`; `staleTime` theo `school_dashboard_refresh`; xóa khi đổi tenant |
| Bộ lọc gần nhất (khối, môn, preset khoảng) | `localStorage` khóa `ew.portal.dashboard.filters.<userId>.<tenantId>` | Per-viewer, bọc try/catch; không chứa dữ liệu học sinh |
| Job export đang theo dõi | RAM | Danh sách `report_id` của phiên; bỏ khi đổi tenant |

Schema dùng (chỉ đọc định nghĩa): `SchoolDashboard`, `DashboardMeta`, `AnalyticsPoint`, `AnalyticsSeries`, `ClassDashboard`, `CourseResults`, `LmsReportRequest`, `ReportJob`, `AssignmentSubmission` trong `specs/openapi/eduworld-api.v1.yaml`.
