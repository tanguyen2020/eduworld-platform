# Implementation Plan: F043 — Rollup analytics và School Learning Dashboard API

**Branch**: `043-school-learning-dashboard` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Dựng module `learning_analytics` trong module `learning` theo ADR-0062: (1) consumer `learning.rollup_dashboard` gom lô event học tập/LMS và upsert gia tăng vào `progress_daily`, `rollup_class_*`, `rollup_class_objective_weekly`, `rollup_school_*`, `rollup_tenant_*`; (2) job `dashboard-day-close` chốt ngày, dựng tuần/tháng, chốt lại ngày bị event muộn; (3) job read model 15 phút (`rm_class_objective_stats`, `rm_class_subject_summary`, `rm_school_grade_summary`) và snapshot tuần; (4) query handler dashboard chạy trên replica bằng role `learning_dashboard`, có query catalog, giới hạn khoảng/độ hạt/đồng thời, cache Valkey theo phiên bản tenant; (5) API `getSchoolDashboard`, `getClassDashboard`, `getSchoolAnalyticsSeries`, `getStudentLearningProfile`, `getTenantLearningSummary`, `getPlatformLearningSummary`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go (JetStream durable consumer), pgx (SQL đọc query catalog), `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/authz`, `src/backend/pkg/tenancy`, `src/backend/pkg/outbox` |
| Storage | PostgreSQL `eduworld_learning` schema `learning` (primary ghi rollup; read replica đọc); Valkey: cache `ew:learning:dash:*`, HLL hôm nay, bộ đếm đồng thời |
| Testing | go test + testcontainers (PostgreSQL primary + replica giả lập bằng role, NATS, Valkey); golden dataset; EXPLAIN test; k6 cho TC-PERF-025 |
| Target Platform | Linux container (module `learning` api + worker) |
| Project Type | backend module |
| Performance Goals | NFR-PERF-017, NFR-PERF-018, [DASHBOARD_DATA §15](../../docs/07-backend/DASHBOARD_DATA.md) |
| Constraints | AGENTS R8, R9, R14, R16, R20, R26, R27, R28 · ADR-0004, ADR-0005, ADR-0027, ADR-0037, ADR-0042, ADR-0062, ADR-0072 |
| Scale/Scope | 50.000 học sinh × 2 năm ([DASHBOARD_DATA §11](../../docs/07-backend/DASHBOARD_DATA.md)) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec.md Ready; delta contract (`getStudentLearningProfile`, `SchoolDashboard.self_study`) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Rollup chỉ từ event (không đọc DB module khác); query handler chỉ đọc; consumer/job gọi command handler qua interface có kiểu |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng rollup có `tenant_id` + RLS (bảng dẫn xuất không `deleted_at` theo DASHBOARD_DATA §3.1); SQL query catalog nhận `TenantScope`; test tenant isolation |
| IV Server-authoritative | ✅ | Chỉ đọc số liệu server tính; không endpoint ghi từ client |
| V Xác thực, phân quyền | ✅ | `analytics.school.read`, `analytics.tenant.read`, `platform.cross_tenant.read`; scope trường/môn; route `/admin/*` gateway nội bộ + role platform |
| VI An toàn trẻ em | ✅ | Không nội dung chat, không Game Rating; audit xem hồ sơ; ẩn ô < 5 ở cấp tenant/platform |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số `dashboard_*`, `analytics_min_cell_size` từ cấu hình; chuỗi `MaxLen` |
| VIII UI tách logic, token | ✅ | Không có UI (F056) |
| IX Test có bằng chứng | ✅ | Golden dataset, EXPLAIN, chaos Valkey/replica, tải TC-PERF-025; mỗi AC có task test |

## Project Structure

### Documentation (feature này)

```text
specs/043-school-learning-dashboard/
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
src/backend/modules/learning/
├── ent/schema/                              # rollup_*.go, rm_*.go, mastery_snapshot_weekly.go (mixin AppendOnly/Derived, không soft-delete)
├── migrations/                              # partition tháng/năm, BRIN, role learning_dashboard, GRANT SELECT
├── internal/domain/analytics/               # metric.go (MetricVersion, Ratio, Histogram), grain.go (ChooseGrain), attention.go, kanon.go
├── internal/application/
│   ├── command/analytics/                   # apply_rollup_batch.go, close_day.go, reclose_late_days.go, refresh_read_models.go, snapshot_mastery_weekly.go
│   ├── query/analytics/                     # get_school_dashboard.go, get_class_dashboard.go, get_school_series.go, get_student_profile.go, get_tenant_summary.go, get_platform_summary.go
│   └── port/                                # RollupWriter, DashboardReader (query catalog), DashboardCache, ConcurrencyLimiter, SafetyNotificationCounter
├── internal/infrastructure/
│   ├── persistence/dashboard/               # catalog.go (query_id → SQL), replica_pool.go (role learning_dashboard, statement_timeout)
│   ├── persistence/rollup/                  # upsert_batch.go (ON CONFLICT cộng dồn)
│   └── cache/                               # dashboard_cache.go (ver tenant, SET NX chống dồn), hll_today.go, inflight_limiter.go
├── internal/presentation/
│   ├── http/public/                         # handler getSchoolDashboard, getClassDashboard, getSchoolAnalyticsSeries, getStudentLearningProfile, getTenantLearningSummary
│   ├── http/public/admin/                   # handler getPlatformLearningSummary (gateway nội bộ)
│   ├── consumer/rollup_dashboard_consumer.go
│   └── job/                                 # dashboard_day_close.go, read_model_refresh.go, mastery_snapshot_weekly.go
└── tests/{unit,integration,contract,perf}/analytics/
```

**Structure Decision**: Rollup nằm trong database của module sở hữu dữ liệu (DASHBOARD_DATA D-6); pool replica và role riêng tách khỏi pool nghiệp vụ; query catalog trong infrastructure để kiểm EXPLAIN trong CI.

## Phase 0: Research

Không có. Cơ chế đã chốt ở ADR-0062 và DASHBOARD_DATA §3–§9.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `analytics.ChooseGrain`, `Ratio`, `Histogram`, `AttentionRule`, `KAnonymity` | Thuần |
| application | Command `ApplyRollupBatch`, `CloseDay`, `RecloseLateDays`, `RefreshReadModels`, `SnapshotMasteryWeekly` · Query `GetSchoolDashboard`, `GetClassDashboard`, `GetSchoolSeries`, `GetStudentProfile`, `GetTenantSummary`, `GetPlatformSummary` | Permission `analytics.school.read` (scope trường/môn), `analytics.tenant.read`, `platform.cross_tenant.read` |
| infrastructure | Upsert batch primary; query catalog replica; cache + limiter Valkey; adapter facade `chat` (đếm thông báo an toàn, khi có) | SQL tường minh có `tenant_id` + phạm vi + khoảng ngày (R27) |
| presentation | Handler 6 operationId; consumer `learning.rollup_dashboard`; job `dashboard-day-close`, `read-model-refresh`, `mastery-snapshot-weekly` | Consumer idempotent `processed_events` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F031 (learning events, mastery), F041 (`assignment.*`, `lms.assignment_submissions`), F032 (`recommendation_id`, `rm_student_recommendations`), F014 (event đăng nhập → `ref.login_activity`).
- Facade: `chat` đếm `school-notifications` [Confirmed, khi F122 Done], gọi trong tiến trình qua `src/backend/pkg/modcall` (R29).
- Event tiêu thụ: `question.answered`, `activity.submitted`, `activity.graded`, `exam.graded`, `attempt.re_evaluated`, `assignment.created/published/completed/results_released`, `assignment.submission_updated`, `learning_objective.progressed/mastered`, `learning.recommendation_generated`, `student.game_logged_in/out`. Event phát: không.

### Bảo mật, tenant, idempotency, audit

- `TenantScope` từ phiên; System Admin dùng `CrossTenantScope(tenant_ids, reason)` → audit `cross_tenant.accessed`; người dùng tenant gửi `tenant_ids` bị bỏ qua.
- Scope `school_admin` theo trường; `academic_manager` theo trường/môn (scope `subject`); ngoài trường → 404, ngoài môn → `403 PERMISSION_DENIED`.
- Audit `learning.profile_viewed` cho hồ sơ học sinh. Không có thao tác ghi từ API → không cần `Idempotency-Key`.

### Quan sát

- Metric DASHBOARD_DATA §13: `eduworld_dashboard_query_duration_seconds`, `eduworld_dashboard_cache_requests_total`, `eduworld_dashboard_rejected_total`, `eduworld_rollup_lag_seconds`, `eduworld_rollup_day_close_duration_seconds`, `eduworld_rollup_late_events_total`.
- Log không PII (chỉ `query_id`, `tenant_id`, phạm vi id); span `dashboard.query`, `rollup.apply_batch`, `rollup.close_day`.

Ước lượng: 3 sprint (backend 2 dev) [Confirmed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
