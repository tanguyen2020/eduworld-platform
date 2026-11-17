# Contracts: F043 — Rollup analytics và School Learning Dashboard API

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`, `x-service: learning`). Mọi endpoint theo quy ước dashboard [API_DESIGN §9.14](../../../docs/07-backend/API_DESIGN.md): `from`/`to` bắt buộc, response có `grain`, `data_as_of`, `is_partial` (`DashboardMeta`), lỗi `DASHBOARD_RANGE_TOO_LARGE`, `DASHBOARD_BUSY`, `DASHBOARD_QUERY_TIMEOUT`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getSchoolDashboard` | `GET /api/v1/lms/dashboards/school` | `learning.v1.yaml` (`SchoolDashboard`) | `analytics.school.read` (scope trường/môn) | Cần sửa (thêm khối `self_study`) |
| `getClassDashboard` | `GET /api/v1/lms/classes/{class_id}/dashboard` | `learning.v1.yaml` | `analytics.school.read` | Có sẵn |
| `getSchoolAnalyticsSeries` | `GET /api/v1/lms/schools/{school_id}/analytics/series` | `learning.v1.yaml` | `analytics.school.read` | Có sẵn |
| `getStudentLearningProfile` | `GET /api/v1/lms/students/{student_id}/analytics` | — | `analytics.school.read` (trong phạm vi) · audit `learning.profile_viewed` | Cần thêm |
| `getTenantLearningSummary` | `GET /api/v1/tenant/analytics/learning/summary` | `learning.v1.yaml` | `analytics.tenant.read` (TA); `analytics.school.read` (SA, trường mình) | Có sẵn |
| `getPlatformLearningSummary` | `GET /api/v1/admin/analytics/learning/summary` | `learning.v1.yaml` | `platform.cross_tenant.read` (System Admin, gateway nội bộ, MFA) | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| Đếm thông báo an toàn chưa xác nhận theo trường | learning → chat | `specs/openapi/modules/chat.internal.v1.yaml` | `learning` | Cần thêm (khi F122 làm `school-notifications`; trước đó trả `null`) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có (portal đọc REST) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `question.answered` (learning event) | Tiêu thụ | `specs/schemas/learning-event.schema.json` | Có sẵn |
| `activity.submitted`, `activity.graded` | Tiêu thụ | `specs/events/payloads/learning/activity.*.v1.schema.json` | Có sẵn |
| `assignment.created` | Tiêu thụ | `specs/events/payloads/learning/assignment.created.v1.schema.json` | Có sẵn |
| `exam.graded`, `attempt.re_evaluated`, `assignment.published`, `assignment.completed`, `assignment.results_released`, `learning_objective.progressed`, `learning_objective.mastered`, `learning.recommendation_generated` | Tiêu thụ | `specs/events/payloads/learning/<event>.v1.schema.json` | Cần thêm (thuộc F031/F032/F041/F042; F043 chỉ tiêu thụ) |
| `assignment.submission_updated` | Tiêu thụ | `specs/events/payloads/learning/assignment.submission_updated.v1.schema.json` | Cần thêm (owner F041) |
| `student.game_logged_in`, `student.game_logged_out` | Tiêu thụ (`ref.login_activity`) | `specs/events/payloads/game/<event>.v1.schema.json` | Cần thêm (thuộc F014) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- `SchoolDashboard`: thêm khối `self_study {avg_active_minutes_per_student (number), recommendation_started_rate (Ratio), objectives_improved_after_recommendation (integer)}`; kiểm `students_needing_attention[].reasons[]` là enum (`missed_assignments`, `beginning_objectives`, `inactive_game`, `rapid_guess`, `scheduled_cleanup`, `maxLength` 64) và không có trường XP/rating.
- Thêm `GET /lms/students/{student_id}/analytics` (`getStudentLearningProfile`): query `subject_id`, `from`, `to`; response `subjects[] {subject_id, accuracy, active_minutes, mastery_by_topic[] {topic_id, topic_name (maxLength 255), level_dist}}`, `trend_weeks[]` (≤ 8), `recent_attempts[]` (≤ 20, không có câu trả lời), `active_recommendations[]` (≤ 3, `rule_code`, `reason_code`), `DashboardMeta`; lỗi 403/404.
- Event payload schema `assignment.submission_updated.v1` và bổ sung dòng vào EVENT_DESIGN §4.3 (đồng thời ghi nhận `assignment.closed` của SCHOOL_LMS §12).
- Internal chat: endpoint đếm thông báo an toàn theo `school_id` (`x-allowed-callers: [learning]`) — phối hợp F122.
