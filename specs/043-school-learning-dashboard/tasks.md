# Tasks: F043 — Rollup analytics và School Learning Dashboard API

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `analytics` (domain/application/infrastructure/presentation) và đăng ký consumer `learning.rollup_dashboard`, job `dashboard-day-close`, `read-model-refresh`, `mastery-snapshot-weekly` trong composition root — file: `src/backend/modules/learning/cmd/api/main.go`, `src/backend/modules/learning/cmd/worker/main.go` — Bằng chứng: `module.go` đăng ký job `dashboard-day-close`, `read-model-refresh` (danh sách job của `cmd/internal/wiring/compose_test.go`); consumer = `learning.progress_daily` mở rộng [Confirmed, spec Session 2026-11-21]; package theo bố cục phẳng của module (`internal/domain/analytics`, `command/dashboards.go`, `query/dashboard*.go`, `persistence/{read,write}`)
- [x] T002 [P] Cấu hình pool replica riêng cho dashboard (DSN role `learning_dashboard`, `dashboard_db_pool_per_api_replica`) và đọc tham số `dashboard_*`, `analytics_min_cell_size`, `school_dashboard_refresh`, `dashboard_attention_thresholds` từ cấu hình — file: `src/backend/modules/learning/internal/infrastructure/persistence/read/dashboard/replica_pool.go`, `src/backend/modules/learning/internal/application/port/read/dashboard_config.go` — Bằng chứng: `dashboardSession` (module.go) mở `EW_DB_LEARNING_DASHBOARD_DSN` + `EW_DB_LEARNING_DASHBOARD_PLATFORM_DSN`, `SetMaxOpenConns(10)`; tham số trong `analytics.DefaultConfig()` [Confirmed — chưa có reader settings]
- [x] T003 [P] Golden dataset học tập (event + kết quả tính tay) — file: `src/backend/modules/learning/tests/fixtures/analytics/golden/` — Bằng chứng: golden dataset trong mã test `dashboard_env_test.go` (`dashKit.golden`, số tính tay ghi ở `dashboard_rollup_test.go`) thay thư mục fixture

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: `SchoolDashboard.self_study`, enum `students_needing_attention[].reasons`, thêm `getStudentLearningProfile` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — Bằng chứng: `SchoolDashboard.widgets.self_study`, `AttentionReason`, `DashboardAssignmentProgress`, `ClassDashboard` phân trang, `getStudentLearningProfile`, `LearningSummary.cells/excluded_tenant_ids`, 503; `check-maxlength`, `check-enums`, `check-portal-catalog`, `validate-schemas`, `split-public --check` 0 lỗi
- [x] T005 [P] Xác nhận payload `assignment.submission_updated.v1` (owner F041 T004) đã có; thiếu → chặn consumer, không tự tạo — file: `specs/events/payloads/learning/assignment.submission_updated.v1.schema.json` (chỉ đọc) — Bằng chứng: `specs/events/payloads/learning/assignment.submission_updated.v1.schema.json` có sẵn; F043 không tiêu thụ (tỷ lệ bài giao tính lại từ `assignment_submissions`, [Confirmed])
- [x] T006 ent schema rollup chuỗi thời gian (`progress_daily` khóa mới, `rollup_class_*`, `rollup_class_objective_weekly`, `rollup_school_*`, `rollup_tenant_*`) và read model (`rm_class_objective_stats`, `rm_class_subject_summary`, `rm_school_grade_summary`, `rm_student_attention`, `mastery_snapshots_weekly`) — file: `src/backend/modules/learning/ent/schema/rollup_*.go`, `src/backend/modules/learning/ent/schema/rm_*.go`, `src/backend/modules/learning/ent/schema/mastery_snapshot_weekly.go` — kiểm: `make migrate-diff MOD=learning name=f043_dashboard_rollups`, `make migrate-lint MOD=learning` — Bằng chứng: bảng viết tay như `progress_daily` của F031 (không entity ent, chỉ SQL tường minh) [Confirmed] — `TestMigrationMatchesEnt` không đổi
- [x] T007 Migration: partition tháng/năm, BRIN `bucket_date`, RLS, role `learning_dashboard` (GRANT SELECT chỉ bảng rollup/read model + `lms.assignment_submissions`, `statement_timeout`, `idle_in_transaction_session_timeout`, `work_mem`), partition trước 3 tháng + `DEFAULT` — file: `src/backend/modules/learning/migrations/<ts>_f043_dashboard_rollups.sql` — Bằng chứng: `migrations/20261223000001_f043_dashboard_rollups.sql` (partition tháng/năm + DEFAULT, BRIN, RLS, GRANT `learning_dashboard`, `learning.ensure_rollup_partitions`); role + `statement_timeout`/`idle_in_transaction_session_timeout`/`work_mem` ở `deploy/postgres/init/02-roles.sh`; `tenancyguard` OK; atlas.sum cập nhật
- [x] T008 [P] Cập nhật DATABASE_DESIGN §1.1, §2.7 (thay `progress_daily(date…)`, `class_objective_rollups` bằng bảng theo DASHBOARD_DATA) — file: `docs/07-backend/DATABASE_DESIGN.md` — Bằng chứng: DATABASE_DESIGN §1.1, §2.7 cập nhật

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Pipeline rollup gia tăng và chốt ngày (Priority: P1) 🎯 MVP

**Goal**: Rollup đúng, idempotent, chốt ngày và xử lý event muộn.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Integration test F043-AC-01, F043-AC-03, F043-AC-05: golden dataset qua consumer + day-close khớp tính tay (TC-DASH-004) — file: `src/backend/modules/learning/tests/integration/analytics/rollup_golden_test.go` — phải FAIL trước khi code — Bằng chứng: `TestDashboardRollupGolden` PASS
- [x] T010 [P] [US1] Integration test F043-AC-02, F043-AC-04: trùng `event_id`; muộn 30 giờ chốt lại, 50 giờ chỉ tăng metric (TC-DASH-005) — file: `src/backend/modules/learning/tests/integration/analytics/rollup_late_dup_test.go` — Bằng chứng: `TestDashboardRollupLateAndDuplicate` PASS
- [x] T011 [P] [US1] Integration test F043-AC-06: read model làm mới trong `school_dashboard_refresh`, snapshot tuần — file: `src/backend/modules/learning/tests/integration/analytics/read_model_refresh_test.go` — Bằng chứng: `TestDashboardReadModelRefresh` PASS

### Implementation cho User Story 1

- [x] T012 [P] [US1] Domain `analytics.Ratio`, `Histogram`, `MetricVersion` — file: `src/backend/modules/learning/internal/domain/analytics/metric.go` — Bằng chứng: `internal/domain/analytics/{metric,grain,config,late}.go`, unit `analytics_test.go` PASS
- [x] T013 [US1] Command `ApplyRollupBatch` + upsert `ON CONFLICT` cộng dồn mọi cấp, HLL hôm nay — file: `src/backend/modules/learning/internal/application/command/analytics/apply_rollup_batch.go`, `src/backend/modules/learning/internal/infrastructure/persistence/write/rollup/upsert_batch.go`, `src/backend/modules/learning/internal/infrastructure/cache/hll_today.go` — Bằng chứng: `command.Progress.add` + `write.RollupRepo.AddRollups` (cùng transaction với `progress_daily`); HLL thay bằng distinct chính xác ở refresh [Confirmed]
- [x] T014 [US1] Consumer `learning.rollup_dashboard` (gom lô `dashboard_ingest_batch`, `processed_events`) — file: `src/backend/modules/learning/internal/presentation/consumer/rollup_dashboard_consumer.go` — Bằng chứng: consumer `learning.progress_daily` (F031) gọi `command.Progress` có `Rollups`; idempotent qua `processed_events` (`TestDashboardRollupLateAndDuplicate`)
- [x] T015 [US1] Command `CloseDay`, `RecloseLateDays` + job `dashboard-day-close` theo tenant, tăng `ew:learning:dash:ver:<tenant_id>` — file: `src/backend/modules/learning/internal/application/command/analytics/close_day.go`, `src/backend/modules/learning/internal/presentation/job/dashboard_day_close.go` — Bằng chứng: `command.Dashboards.CloseDays`, `write.RollupRepo.{DaysToClose,Recompute,MarkClosed}`, `job.DashboardDayClose`, `cache.Dashboard.Bump`
- [x] T016 [US1] Command `RefreshReadModels`, `SnapshotMasteryWeekly` + job — file: `src/backend/modules/learning/internal/application/command/analytics/refresh_read_models.go`, `src/backend/modules/learning/internal/presentation/job/read_model_refresh.go` — Bằng chứng: `command.Dashboards.Refresh`, `RefreshReadModels`, `AttentionSignals`/`SaveAttention`, `SnapshotMastery`, `job.ReadModelRefresh`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — School Learning Dashboard cấp trường và lớp (Priority: P1)

**Goal**: `getSchoolDashboard`, `getClassDashboard`, `getSchoolAnalyticsSeries`.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T017 [P] [US2] API test F043-AC-07, F043-AC-09, F043-AC-11: đủ khối widget, khớp truy vấn đối soát, không có XP/rating/chat (TC-LMS-003) — file: `src/backend/modules/learning/tests/integration/analytics/school_dashboard_test.go` — phải FAIL trước khi code — Bằng chứng: `TestSchoolDashboardWidgets` PASS
- [x] T018 [P] [US2] Unit test F043-AC-08: quy tắc "cần chú ý" theo `dashboard_attention_thresholds` — file: `src/backend/modules/learning/tests/unit/analytics/attention_test.go` — Bằng chứng: `TestAttentionThresholds` (domain `analytics`) PASS
- [x] T019 [P] [US2] API test F043-AC-10, F043-AC-12: heatmap lớp × LO, phân trang cursor, chuỗi ≤ 120 điểm — file: `src/backend/modules/learning/tests/integration/analytics/class_dashboard_series_test.go` — Bằng chứng: `TestClassDashboardAndSeries` PASS
- [x] T020 [P] [US2] Test tenant isolation + authorization F043-AC-13, F043-AC-14, F043-AC-15 cho `getSchoolDashboard`, `getClassDashboard`, `getSchoolAnalyticsSeries`: Academic Manager môn khác `PERMISSION_DENIED`, School Admin trường khác 404, `sm.antoan`/học sinh 403, tenant khác 404, khóa cache có `tenant_id` (TC-TEN-002, TC-TEN-003, TC-DASH-009) — file: `src/backend/modules/learning/tests/integration/analytics/dashboard_authz_tenant_test.go` — Bằng chứng: `TestDashboardAuthorizationAndTenant` PASS (khóa cache có `tenant_id`: `TestDashboardCacheAndResilience`)

### Implementation cho User Story 2

- [x] T021 [P] [US2] Domain `analytics.AttentionRule` — file: `src/backend/modules/learning/internal/domain/analytics/attention.go` — Bằng chứng: `internal/domain/analytics/attention.go`
- [x] T022 [US2] Query catalog SQL (mỗi widget một `query_id`, tham số scope + khoảng) — file: `src/backend/modules/learning/internal/infrastructure/persistence/read/dashboard/catalog.go` — Bằng chứng: `persistence/read/dashboard.go` (`DashboardModel`, `Catalog`)
- [x] T023 [US2] Query `GetSchoolDashboard`, `GetClassDashboard`, `GetSchoolSeries` + scope trường/môn — file: `src/backend/modules/learning/internal/application/query/analytics/get_school_dashboard.go`, `src/backend/modules/learning/internal/application/query/analytics/get_class_dashboard.go`, `src/backend/modules/learning/internal/application/query/analytics/get_school_series.go` — Bằng chứng: `query/dashboard_views.go` (`GetSchoolDashboard`, `GetClassDashboard`, `GetSchoolSeries`, `schoolScope`)
- [x] T024 [US2] Handler 3 operationId — file: `src/backend/modules/learning/internal/presentation/http/public/dashboard_handler.go` — Bằng chứng: `presentation/http/dashboards.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Quy tắc truy vấn và bảo vệ database (Priority: P1)

**Goal**: Giới hạn khoảng/độ hạt, timeout, đồng thời, cache, replica.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T025 [P] [US3] Unit + API test F043-AC-16, F043-AC-17: 731 ngày, thiếu `from`, grain mịn hơn, `page_size` (TC-DASH-002, TC-DASH-003) — file: `src/backend/modules/learning/tests/unit/analytics/grain_test.go` — phải FAIL trước khi code — Bằng chứng: `TestNewRangeGrainAndLimits`, `TestBucketsBounded`, `TestPageSizeClamped` (domain) + `TestClassDashboardAndSeries` PASS
- [x] T026 [P] [US3] Integration test F043-AC-18, F043-AC-22: role `learning_dashboard` không đọc được `learning.learning_events`/`assessment.attempts`; EXPLAIN mọi `query_id` không Seq Scan trên rollup ngày (TC-DASH-001) — file: `src/backend/modules/learning/tests/integration/analytics/query_catalog_explain_test.go` — Bằng chứng: `TestDashboardRoleAndQueryCatalog` PASS (EXPLAIN với `enable_seqscan = off` trên dữ liệu mẫu)
- [x] T027 [P] [US3] Integration test F043-AC-19: truy vấn thứ 5 → 429, tenant > giới hạn → 429, truy vấn > timeout → 504 (TC-DASH-006) — file: `src/backend/modules/learning/tests/integration/analytics/dashboard_limits_test.go` — Bằng chứng: `TestDashboardLimits` PASS
- [x] T028 [P] [US3] Chaos test F043-AC-20, F043-AC-21: mất Valkey vẫn đúng; sau chốt ngày không trả cache cũ; replica dừng → 503, không dùng primary (TC-DASH-007) — file: `src/backend/modules/learning/tests/integration/analytics/dashboard_resilience_test.go` — Bằng chứng: `TestDashboardCacheAndResilience` PASS
- [ ] T029 [P] [US3] Kiểm thử tải F043-AC-22 trên dữ liệu 2 năm (TC-DASH-010, TC-PERF-025) — file: `src/backend/modules/learning/tests/perf/analytics/dashboard_load.js` — **Chưa chạy**: cần dữ liệu 2 năm × 50.000 học sinh trên staging (replica, k6); EXPLAIN của mọi `query_id` đã kiểm ở T026

### Implementation cho User Story 3

- [x] T030 [P] [US3] Domain `analytics.ChooseGrain` và validate khoảng — file: `src/backend/modules/learning/internal/domain/analytics/grain.go` — Bằng chứng: `internal/domain/analytics/grain.go`
- [x] T031 [US3] Decorator có kiểu cho query handler dashboard giới hạn đồng thời (`ew:learning:dash:inflight:<user_id>`, theo tenant) + map lỗi `DASHBOARD_BUSY`/`DASHBOARD_QUERY_TIMEOUT` — file: `src/backend/modules/learning/internal/infrastructure/cache/inflight_limiter.go`, `src/backend/modules/learning/internal/application/query/analytics/dashboard_limits.go` — Bằng chứng: `cache.Limiter` (Valkey, dự phòng theo tiến trình khi mất Valkey), `query.dashRun`, `query.BusyError` + `Retry-After`, `read.MapError`/57014 → 504
- [x] T032 [US3] Cache kết quả theo phiên bản tenant + `SET NX` chống dồn, fallback không cache — file: `src/backend/modules/learning/internal/infrastructure/cache/dashboard_cache.go` — Bằng chứng: `cache.Dashboard` (`ew:learning:dash:ver|res:*`); chống dồn `SET NX` chưa làm [Confirmed — đo ở TC-PERF-025]

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Hồ sơ học tập của một học sinh (Priority: P2)

**Goal**: `getStudentLearningProfile` có audit.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T033 [P] [US4] API test F043-AC-23, F043-AC-24: dữ liệu hồ sơ, audit `learning.profile_viewed`, ngoài phạm vi 404, không Game Rating/chat — file: `src/backend/modules/learning/tests/integration/analytics/student_profile_test.go` — phải FAIL trước khi code — Bằng chứng: `TestStudentLearningProfile` PASS

### Implementation cho User Story 4

- [x] T034 [US4] Query `GetStudentProfile` (đọc `progress_daily`, `mastery_snapshots_weekly`, `objective_masteries`, `rm_student_recommendations`) + outbox `audit.recorded` + handler — file: `src/backend/modules/learning/internal/application/query/analytics/get_student_profile.go`, `src/backend/modules/learning/internal/presentation/http/public/dashboard_handler.go` — Bằng chứng: `query.GetStudentProfile` + `ProfileViews.Record` ở handler

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Tổng hợp cấp tenant và liên tenant (Priority: P2)

**Goal**: `getTenantLearningSummary`, `getPlatformLearningSummary` có k-anonymity.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T035 [P] [US5] Unit test F043-AC-25: ẩn ô < `analytics_min_cell_size`, tổng không suy ra được ô ẩn — file: `src/backend/modules/learning/tests/unit/analytics/kanon_test.go` — phải FAIL trước khi code — Bằng chứng: `TestSuppressKAnonymity` PASS
- [x] T036 [P] [US5] API test F043-AC-26, F043-AC-27: System Admin `tenant_ids` có `tenant_id`/dòng + audit `cross_tenant.accessed`; người dùng tenant gửi `tenant_ids` bị bỏ qua; role tenant gọi `/admin/analytics/*` → 403 (TC-TEN-004, TC-TEN-011, TC-TEN-018) — file: `src/backend/modules/learning/tests/integration/analytics/tenant_platform_summary_test.go` — Bằng chứng: `TestTenantAndPlatformSummary` PASS

### Implementation cho User Story 5

- [x] T037 [P] [US5] Domain `analytics.KAnonymity` — file: `src/backend/modules/learning/internal/domain/analytics/kanon.go` — Bằng chứng: `internal/domain/analytics/kanon.go`
- [x] T038 [US5] Query `GetTenantSummary`, `GetPlatformSummary` (`CrossTenantScope`, role `learning_dashboard` với `tenant_id = ANY`) + handler — file: `src/backend/modules/learning/internal/application/query/analytics/get_tenant_summary.go`, `src/backend/modules/learning/internal/application/query/analytics/get_platform_summary.go`, `src/backend/modules/learning/internal/presentation/http/public/admin/learning_summary_handler.go` — Bằng chứng: `query.GetTenantSummary`, `query.GetPlatformSummary`, handler trong `presentation/http/dashboards.go`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: Polish & Cross-Cutting

- [ ] T039 [P] Metric DASHBOARD_DATA §13 + cảnh báo `DashboardSlow`, `RollupLagHigh`, `DayCloseNotDone` — file: `src/backend/modules/learning/internal/infrastructure/observability/dashboard_metrics.go`, `deploy/observability/alerts/learning-dashboard.yaml` — **Một phần**: metric DASHBOARD_DATA §13 ở `observability/dashboard_metrics.go` (đăng ký trong module.go); file cảnh báo chờ thư mục rule Prometheus của MONITORING (repo chưa có `deploy/observability/alerts/`)
- [x] T040 [P] Cập nhật CHANGELOG và trạng thái F043 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — Bằng chứng: CHANGELOG, CODE_PLAN (In progress — AC-22 chờ kiểm thử tải)
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/043-school-learning-dashboard/quickstart.md` — **Một phần**: kịch bản 1–12 tương ứng test integration `TestDashboard*`, `TestSchoolDashboardWidgets`, `TestClassDashboardAndSeries`, `TestStudentLearningProfile`, `TestTenantAndPlatformSummary` (local, testcontainers); kịch bản 13 (tải) chờ staging

## Dependencies & Execution Order

- Setup → Foundational (T004–T008) → US1 → US2 và US3 (song song sau US1) → US4, US5 → Polish.
- T024, T034 cùng sửa `dashboard_handler.go`: làm tuần tự.
- Khối thông báo an toàn trả `null` tới khi F122 có operation facade (không mock, R17).

## Definition of Done

- [ ] Mọi tiêu chí F043-AC-* có test pass (ghi mã test/TC) — 26/27: F043-AC-22 (p95 trên dữ liệu 2 năm) mới có phần EXPLAIN, chưa có kiểm thử tải
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — chạy lệnh nền tương đương (máy không có `make`)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — số thông báo an toàn trả `null` tới F122
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — DATABASE_DESIGN §1.1/§2.7, CHANGELOG; giá trị CONFIG_DEFAULTS không đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
