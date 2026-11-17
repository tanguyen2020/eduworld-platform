# Tasks: F043 — Rollup analytics và School Learning Dashboard API

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `analytics` (domain/application/infrastructure/presentation) và đăng ký consumer `learning.rollup_dashboard`, job `dashboard-day-close`, `read-model-refresh`, `mastery-snapshot-weekly` trong composition root — file: `modules/learning/cmd/api/main.go`, `modules/learning/cmd/worker/main.go`
- [ ] T002 [P] Cấu hình pool replica riêng cho dashboard (DSN role `learning_dashboard`, `dashboard_db_pool_per_api_replica`) và đọc tham số `dashboard_*`, `analytics_min_cell_size`, `school_dashboard_refresh`, `dashboard_attention_thresholds` từ cấu hình — file: `modules/learning/internal/infrastructure/persistence/read/dashboard/replica_pool.go`, `modules/learning/internal/application/port/read/dashboard_config.go`
- [ ] T003 [P] Golden dataset học tập (event + kết quả tính tay) — file: `modules/learning/tests/fixtures/analytics/golden/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: `SchoolDashboard.self_study`, enum `students_needing_attention[].reasons`, thêm `getStudentLearningProfile` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T005 [P] Xác nhận payload `assignment.submission_updated.v1` (owner F041 T004) đã có; thiếu → chặn consumer, không tự tạo — file: `specs/events/payloads/learning/assignment.submission_updated.v1.schema.json` (chỉ đọc)
- [ ] T006 ent schema rollup chuỗi thời gian (`progress_daily` khóa mới, `rollup_class_*`, `rollup_class_objective_weekly`, `rollup_school_*`, `rollup_tenant_*`) và read model (`rm_class_objective_stats`, `rm_class_subject_summary`, `rm_school_grade_summary`, `rm_student_attention`, `mastery_snapshots_weekly`) — file: `modules/learning/ent/schema/rollup_*.go`, `modules/learning/ent/schema/rm_*.go`, `modules/learning/ent/schema/mastery_snapshot_weekly.go` — kiểm: `make migrate-diff MOD=learning name=f043_dashboard_rollups`, `make migrate-lint MOD=learning`
- [ ] T007 Migration: partition tháng/năm, BRIN `bucket_date`, RLS, role `learning_dashboard` (GRANT SELECT chỉ bảng rollup/read model + `lms.assignment_submissions`, `statement_timeout`, `idle_in_transaction_session_timeout`, `work_mem`), partition trước 3 tháng + `DEFAULT` — file: `modules/learning/migrations/<ts>_f043_dashboard_rollups.sql`
- [ ] T008 [P] Cập nhật DATABASE_DESIGN §1.1, §2.7 (thay `progress_daily(date…)`, `class_objective_rollups` bằng bảng theo DASHBOARD_DATA) — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Pipeline rollup gia tăng và chốt ngày (Priority: P1) 🎯 MVP

**Goal**: Rollup đúng, idempotent, chốt ngày và xử lý event muộn.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Integration test F043-AC-01, F043-AC-03, F043-AC-05: golden dataset qua consumer + day-close khớp tính tay (TC-DASH-004) — file: `modules/learning/tests/integration/analytics/rollup_golden_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Integration test F043-AC-02, F043-AC-04: trùng `event_id`; muộn 30 giờ chốt lại, 50 giờ chỉ tăng metric (TC-DASH-005) — file: `modules/learning/tests/integration/analytics/rollup_late_dup_test.go`
- [ ] T011 [P] [US1] Integration test F043-AC-06: read model làm mới trong `school_dashboard_refresh`, snapshot tuần — file: `modules/learning/tests/integration/analytics/read_model_refresh_test.go`

### Implementation cho User Story 1

- [ ] T012 [P] [US1] Domain `analytics.Ratio`, `Histogram`, `MetricVersion` — file: `modules/learning/internal/domain/analytics/metric.go`
- [ ] T013 [US1] Command `ApplyRollupBatch` + upsert `ON CONFLICT` cộng dồn mọi cấp, HLL hôm nay — file: `modules/learning/internal/application/command/analytics/apply_rollup_batch.go`, `modules/learning/internal/infrastructure/persistence/write/rollup/upsert_batch.go`, `modules/learning/internal/infrastructure/cache/hll_today.go`
- [ ] T014 [US1] Consumer `learning.rollup_dashboard` (gom lô `dashboard_ingest_batch`, `processed_events`) — file: `modules/learning/internal/presentation/consumer/rollup_dashboard_consumer.go`
- [ ] T015 [US1] Command `CloseDay`, `RecloseLateDays` + job `dashboard-day-close` theo tenant, tăng `ew:learning:dash:ver:<tenant_id>` — file: `modules/learning/internal/application/command/analytics/close_day.go`, `modules/learning/internal/presentation/job/dashboard_day_close.go`
- [ ] T016 [US1] Command `RefreshReadModels`, `SnapshotMasteryWeekly` + job — file: `modules/learning/internal/application/command/analytics/refresh_read_models.go`, `modules/learning/internal/presentation/job/read_model_refresh.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — School Learning Dashboard cấp trường và lớp (Priority: P1)

**Goal**: `getSchoolDashboard`, `getClassDashboard`, `getSchoolAnalyticsSeries`.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T017 [P] [US2] API test F043-AC-07, F043-AC-09, F043-AC-11: đủ khối widget, khớp truy vấn đối soát, không có XP/rating/chat (TC-LMS-003) — file: `modules/learning/tests/integration/analytics/school_dashboard_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Unit test F043-AC-08: quy tắc "cần chú ý" theo `dashboard_attention_thresholds` — file: `modules/learning/tests/unit/analytics/attention_test.go`
- [ ] T019 [P] [US2] API test F043-AC-10, F043-AC-12: heatmap lớp × LO, phân trang cursor, chuỗi ≤ 120 điểm — file: `modules/learning/tests/integration/analytics/class_dashboard_series_test.go`
- [ ] T020 [P] [US2] Test tenant isolation + authorization F043-AC-13, F043-AC-14, F043-AC-15 cho `getSchoolDashboard`, `getClassDashboard`, `getSchoolAnalyticsSeries`: Academic Manager môn khác `PERMISSION_DENIED`, School Admin trường khác 404, `sm.antoan`/học sinh 403, tenant khác 404, khóa cache có `tenant_id` (TC-TEN-002, TC-TEN-003, TC-DASH-009) — file: `modules/learning/tests/integration/analytics/dashboard_authz_tenant_test.go`

### Implementation cho User Story 2

- [ ] T021 [P] [US2] Domain `analytics.AttentionRule` — file: `modules/learning/internal/domain/analytics/attention.go`
- [ ] T022 [US2] Query catalog SQL (mỗi widget một `query_id`, tham số scope + khoảng) — file: `modules/learning/internal/infrastructure/persistence/read/dashboard/catalog.go`
- [ ] T023 [US2] Query `GetSchoolDashboard`, `GetClassDashboard`, `GetSchoolSeries` + scope trường/môn — file: `modules/learning/internal/application/query/analytics/get_school_dashboard.go`, `modules/learning/internal/application/query/analytics/get_class_dashboard.go`, `modules/learning/internal/application/query/analytics/get_school_series.go`
- [ ] T024 [US2] Handler 3 operationId — file: `modules/learning/internal/presentation/http/public/dashboard_handler.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Quy tắc truy vấn và bảo vệ database (Priority: P1)

**Goal**: Giới hạn khoảng/độ hạt, timeout, đồng thời, cache, replica.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T025 [P] [US3] Unit + API test F043-AC-16, F043-AC-17: 731 ngày, thiếu `from`, grain mịn hơn, `page_size` (TC-DASH-002, TC-DASH-003) — file: `modules/learning/tests/unit/analytics/grain_test.go` — phải FAIL trước khi code
- [ ] T026 [P] [US3] Integration test F043-AC-18, F043-AC-22: role `learning_dashboard` không đọc được `learning.learning_events`/`assessment.attempts`; EXPLAIN mọi `query_id` không Seq Scan trên rollup ngày (TC-DASH-001) — file: `modules/learning/tests/integration/analytics/query_catalog_explain_test.go`
- [ ] T027 [P] [US3] Integration test F043-AC-19: truy vấn thứ 5 → 429, tenant > giới hạn → 429, truy vấn > timeout → 504 (TC-DASH-006) — file: `modules/learning/tests/integration/analytics/dashboard_limits_test.go`
- [ ] T028 [P] [US3] Chaos test F043-AC-20, F043-AC-21: mất Valkey vẫn đúng; sau chốt ngày không trả cache cũ; replica dừng → 503, không dùng primary (TC-DASH-007) — file: `modules/learning/tests/integration/analytics/dashboard_resilience_test.go`
- [ ] T029 [P] [US3] Kiểm thử tải F043-AC-22 trên dữ liệu 2 năm (TC-DASH-010, TC-PERF-025) — file: `modules/learning/tests/perf/analytics/dashboard_load.js`

### Implementation cho User Story 3

- [ ] T030 [P] [US3] Domain `analytics.ChooseGrain` và validate khoảng — file: `modules/learning/internal/domain/analytics/grain.go`
- [ ] T031 [US3] Decorator có kiểu cho query handler dashboard giới hạn đồng thời (`ew:learning:dash:inflight:<user_id>`, theo tenant) + map lỗi `DASHBOARD_BUSY`/`DASHBOARD_QUERY_TIMEOUT` — file: `modules/learning/internal/infrastructure/cache/inflight_limiter.go`, `modules/learning/internal/application/query/analytics/dashboard_limits.go`
- [ ] T032 [US3] Cache kết quả theo phiên bản tenant + `SET NX` chống dồn, fallback không cache — file: `modules/learning/internal/infrastructure/cache/dashboard_cache.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Hồ sơ học tập của một học sinh (Priority: P2)

**Goal**: `getStudentLearningProfile` có audit.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T033 [P] [US4] API test F043-AC-23, F043-AC-24: dữ liệu hồ sơ, audit `learning.profile_viewed`, ngoài phạm vi 404, không Game Rating/chat — file: `modules/learning/tests/integration/analytics/student_profile_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T034 [US4] Query `GetStudentProfile` (đọc `progress_daily`, `mastery_snapshots_weekly`, `objective_masteries`, `rm_student_recommendations`) + outbox `audit.recorded` + handler — file: `modules/learning/internal/application/query/analytics/get_student_profile.go`, `modules/learning/internal/presentation/http/public/dashboard_handler.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Tổng hợp cấp tenant và liên tenant (Priority: P2)

**Goal**: `getTenantLearningSummary`, `getPlatformLearningSummary` có k-anonymity.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T035 [P] [US5] Unit test F043-AC-25: ẩn ô < `analytics_min_cell_size`, tổng không suy ra được ô ẩn — file: `modules/learning/tests/unit/analytics/kanon_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US5] API test F043-AC-26, F043-AC-27: System Admin `tenant_ids` có `tenant_id`/dòng + audit `cross_tenant.accessed`; người dùng tenant gửi `tenant_ids` bị bỏ qua; role tenant gọi `/admin/analytics/*` → 403 (TC-TEN-004, TC-TEN-011, TC-TEN-018) — file: `modules/learning/tests/integration/analytics/tenant_platform_summary_test.go`

### Implementation cho User Story 5

- [ ] T037 [P] [US5] Domain `analytics.KAnonymity` — file: `modules/learning/internal/domain/analytics/kanon.go`
- [ ] T038 [US5] Query `GetTenantSummary`, `GetPlatformSummary` (`CrossTenantScope`, role `learning_dashboard` với `tenant_id = ANY`) + handler — file: `modules/learning/internal/application/query/analytics/get_tenant_summary.go`, `modules/learning/internal/application/query/analytics/get_platform_summary.go`, `modules/learning/internal/presentation/http/public/admin/learning_summary_handler.go`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: Polish & Cross-Cutting

- [ ] T039 [P] Metric DASHBOARD_DATA §13 + cảnh báo `DashboardSlow`, `RollupLagHigh`, `DayCloseNotDone` — file: `modules/learning/internal/infrastructure/observability/dashboard_metrics.go`, `deploy/observability/alerts/learning-dashboard.yaml`
- [ ] T040 [P] Cập nhật CHANGELOG và trạng thái F043 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T041 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/043-school-learning-dashboard/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T004–T008) → US1 → US2 và US3 (song song sau US1) → US4, US5 → Polish.
- T024, T034 cùng sửa `dashboard_handler.go`: làm tuần tự.
- Khối thông báo an toàn trả `null` tới khi F122 có operation facade (không mock, R17).

## Definition of Done

- [ ] Mọi tiêu chí F043-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
