# Contracts: F056 — Workspace C — School Learning Dashboard và báo cáo

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml`. Route `/lms/*`, `/tenant/*` gửi `X-Tenant-Id`; route `/admin/*` chỉ gateway nội bộ + MFA, list dùng `tenant_ids[]`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getSchoolDashboard` | `GET /api/v1/lms/dashboards/school` | eduworld-api | `analytics.school.read` | Có sẵn; ADR-0095 (WP-L2): bỏ `class_id`, lọc `grade_level`; `widgets.grades` (thay `classes`), `weak_objectives[].grade_levels`, `assignments[].by_grade` (ô có `suppressed`, số đếm null), `students_needing_attention[].grade_level`, `suppressed` cấp trả lời, `widgets.active_students` nullable |
| `getGradeDashboard` | `GET /api/v1/lms/schools/{school_id}/grades/{grade_level}/dashboard` | eduworld-api | `analytics.school.read` | Mới (ADR-0095, WP-L2) — thay `getClassDashboard` (đã xóa): heatmap LO của khối, học sinh của khối (cursor), tiến độ bài giao; `suppressed` khi khối < `analytics_min_cell_size` |
| `getSchoolAnalyticsSeries` | `GET /api/v1/lms/schools/{school_id}/analytics/series` | eduworld-api | `analytics.school.read` | Có sẵn; ADR-0095: chuỗi khối bị che → điểm `value: null`, `suppressed: true` |
| ~~`getCourseResults`~~ | ~~`GET /api/v1/lms/courses/{course_id}/results`~~ | — | — | **Đã xóa** (ADR-0095, WP-L2): kết quả theo bài giao = export `assignment_results` (cột `grade_level`); `skill_map` theo khối; `/tenant/reports` `grade_progress` thay `class_progress` |
| `listAssignmentSubmissions` | `GET /api/v1/lms/assignments/{assignment_id}/submissions` | eduworld-api | `lms.assignment.read` | Có sẵn |
| `listTenantStudentLoginEvents` | `GET /api/v1/tenant/students/{student_id}/login-events` | eduworld-api | `lms.student_activity.read` | Có sẵn |
| `createLmsReport` · `getLmsReport` | `POST /api/v1/lms/reports` · `GET …/{report_id}` | eduworld-api | `report.export` | Có sẵn |
| `getTenantLearningSummary` | `GET /api/v1/tenant/analytics/learning/summary` | eduworld-api | `analytics.tenant.read` | Có sẵn |
| `createTenantReport` · `getTenantReport` | `POST /api/v1/tenant/reports` · `GET …/{report_id}` | eduworld-api | `analytics.tenant.read` + `report.export` | Có sẵn |
| `getPlatformLearningSummary` | `GET /api/v1/admin/analytics/learning/summary` | eduworld-api | `platform.cross_tenant.read` | Có sẵn |
| `createPlatformReport` · `getPlatformReport` | `POST /api/v1/admin/reports` · `GET …/{report_id}` | eduworld-api | `platform.cross_tenant.read` + `report.export` | Có sẵn |
| `getStudentLearningProfile` | `GET /api/v1/lms/students/{student_id}/analytics` | eduworld-api | `analytics.school.read` | Cần thêm |
| `getExamSessionReport` | `GET /api/v1/exams/sessions/{exam_session_id}/report` | eduworld-api | `lms.report.read` | Cần thêm |
| `getAssignmentItemAnalysis` | `GET /api/v1/lms/assignments/{assignment_id}/analysis` | eduworld-api | `lms.report.read` | Cần thêm |
| `getMyPermissions` | `GET /api/v1/me/permissions` | eduworld-api | đã đăng nhập | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (frontend) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Portal không phát/tiêu thụ; `assignment.submission_updated` tiêu thụ ở module learning (F043) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không | — |

## 6. Delta cần thực hiện

Owner backend: F043 (dashboard), F044 (phân tích đề). Phase 2 task T004–T006.

- **Cần thêm** `getStudentLearningProfile` — `GET /lms/students/{student_id}/analytics?subject_id=&from=&to=` → `StudentLearningProfile` (allOf `DashboardMeta`; `student_id`, `display_name` maxLength 128, `class_id`, `objectives[]` {`objective_id`, `code` maxLength 64, `mastery_level` `MasteryLevel`, `last_evidence_at`} maxItems 500, `weekly_trend[]` `AnalyticsPoint` maxItems 120, `recent_assignments[]` `AssignmentSubmission` maxItems 50, `active_recommendations[]` {`objective_id`, `reason_code` maxLength 64, `done`} maxItems 20); 403/404/422/429/504. Đường dẫn đã nêu ở [DASHBOARD_DATA §14](../../../docs/07-backend/DASHBOARD_DATA.md) nhưng chưa có trong spec.
- **Cần thêm** `getExamSessionReport` — `GET /exams/sessions/{exam_session_id}/report` ([EXAM_ENGINE §11](../../../docs/02-learning/EXAM_ENGINE.md)) và `getAssignmentItemAnalysis` — `GET /lms/assignments/{assignment_id}/analysis`, cùng schema `ItemAnalysis` (`score_histogram[]` maxItems 20, `median`, `pass_rate`, `items[]` {`position`, `question_version_id`, `p_value`, `rapid_guess_ratio`, `common_wrong_options[]` {`option_id` maxLength 64, `ratio`, `misconception_tag` maxLength 64} maxItems 5} maxItems 200, `reliability` {`kr20`, `sem`, `warning`}).
- **Cần sửa** `SchoolDashboard.widgets`: thêm `self_study` {`avg_active_minutes_per_student`, `recommendation_completion_rate`, `objectives_improved_after_recommendation`} cho câu hỏi 4 ([SCHOOL_LMS §8.1](../../../docs/04-admin-portal/SCHOOL_LMS.md)).
