# Tasks: F080 — Game Analytics và Live Ops dashboard (workspace E)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Backend gốc `modules/game/`; frontend gốc `web/apps/portal/`. MSW chỉ trong test (`// MOCK (test-only)`, R17).

## Phase 1: Setup

- [ ] T001 Đăng ký permission `game.analytics.read`, `game.liveops.read`, `game.event.operate`, `analytics.tenant.read`, `analytics.school.read` vào kiểm tra catalog — file: `modules/game/internal/bootstrap/authz.go`
- [ ] T002 [P] Cấu hình: durable consumer `analytics`, lịch `rollup-hourly`, `dashboard-day-close`, `analytics-retention`, `export-worker`; env `EW_PROMETHEUS_URL`, secret khóa HMAC (chỉ `.env.example`) — file: `modules/game/config/worker.yaml`, `modules/game/.env.example`
- [ ] T003 [P] Golden dataset + script phát event — file: `modules/game/tests/fixtures/analytics/golden/`
- [ ] T004 [P] Route lazy `/live-ops/{dashboard,analytics,events}` + `RequirePermission` — file: `src/workspaces/live-ops/routes.tsx`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T005 Delta contract: 7 operation `/admin/analytics/game/*`, `getLiveOpsOverview`, schema response — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [ ] T006 [P] Delta contract: `TelemetryBatch`/`TelemetryEvent` `additionalProperties: false`; `ClientConfig.product_telemetry_enabled` (owner F014); `TenantSettings.product_telemetry_enabled` (owner F015) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T007 [P] Xác nhận payload schema event nguồn (GAME_ANALYTICS §3.1, `tenant.updated`) đã có; thiếu → consumer bỏ qua event đó và ghi metric, không tự định nghĩa — file: `specs/events/payloads/` (chỉ đọc)
- [ ] T008 [P] Cập nhật API_DESIGN §9 (`getGameAnalyticsExport`, `getLiveOpsOverview`), GAME_ANALYTICS §6 theo DATABASE_DESIGN — file: `docs/07-backend/API_DESIGN.md`, `docs/11-operations/GAME_ANALYTICS.md`
- [ ] T009 ent schema + migration: cột mới raw/player_days/rollup, `rollup_monthly`, `export_jobs`, `ref.tenants.product_telemetry_enabled`; partition; role `game_dashboard` (SELECT rollup) — file: `ent/schema/analytics/*.go`, `deploy/postgres/roles/game.sql` — kiểm: `make migrate-diff MOD=game`, `make migrate-lint MOD=game`
- [ ] T010 [P] Domain: `EventMapping`, `PlayerKey`, `MetricCatalog`, `Suppress`, `GrainFor` + unit test bảng — file: `internal/domain/analytics/`
- [ ] T011 [P] `ops-ui` `GrainLabel`, `DataAsOf` + story; dùng lại `DashboardWidget`, `SuppressedCell` của F056 (không tạo component ẩn ô thứ hai, DESIGN_SYSTEM §4) — file: `web/packages/ops-ui/src/dashboard/`
- [ ] T012 Sinh lại client TS — file: `src/api/schema.d.ts` — kiểm: `pnpm gen:api`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Telemetry (Priority: P1) 🎯 MVP

**Goal**: Thu telemetry không PII.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T013 [P] [US1] Test F080-AC-01: schema đóng → 422, khử trùng `client_event_id`, 202 (TC-ANL-003) — file: `tests/integration/analytics/telemetry_ingest_test.go` — phải FAIL trước khi code
- [ ] T014 [P] [US1] Test F080-AC-02: rate limit 429 + `Retry-After`; token không phải học sinh → 403 — file: `tests/integration/analytics/telemetry_authz_test.go`
- [ ] T015 [P] [US1] Test F080-AC-03: tenant tắt → bỏ `screen_view`/`tutorial_step`, giữ perf/crash; client-config trả cờ (TC-ANL-007) — file: `tests/integration/analytics/telemetry_tenant_toggle_test.go`
- [ ] T064 [P] [US1] Test F080-AC-34: `break_reminder_shown` hợp lệ → raw `source = client` với `session_key` của token; khóa lạ/`reminder_no` 0 hoặc 21 → 422; trùng `client_event_id` → 1 dòng; tenant tắt telemetry sản phẩm vẫn ghi (TC-ANL-003) — file: `tests/integration/analytics/telemetry_break_reminder_test.go`
- [ ] T016 [P] [US1] Test F080-AC-04: crash → error tracking, log không PII (TC-ANL-003) — file: `tests/unit/analytics/telemetry_logging_test.go`

### Implementation cho User Story 1

- [ ] T017 [US1] Command `IngestTelemetry` (gồm `break_reminder_shown`, không lọc bởi cờ telemetry sản phẩm, `session_key` từ game session); handler `postTelemetry` mở rộng; consumer `tenant.updated` → `ref.tenants` — file: `internal/application/command/analytics/ingest_telemetry.go`, `internal/presentation/http/public/telemetry.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Consumer và raw (Priority: P1)

### Tests cho User Story 2

- [ ] T018 [P] [US2] Test F080-AC-05: ánh xạ event, `player_key` HMAC, chiều chuẩn, bỏ khóa lạ, idempotent (TC-ANL-002, TC-SVC-003) — file: `tests/integration/analytics/consumer_test.go` — phải FAIL trước khi code
- [ ] T019 [P] [US2] Test F080-AC-06: quét schema `analytics` + `props` không định danh (TC-ANL-003) — file: `tests/integration/analytics/schema_scan_test.go`
- [ ] T020 [P] [US2] Test F080-AC-07: ẩn danh `player_key`, rollup không đổi, không DELETE (TC-ANL-010) — file: `tests/integration/analytics/anonymize_test.go`

### Implementation cho User Story 2

- [ ] T021 [US2] Command `IngestDomainEvent`, `AnonymizePlayer`; consumer `analytics_consumer.go`; `HmacKeyProvider` — file: `internal/application/command/analytics/`, `internal/presentation/consumer/analytics_consumer.go`, `internal/infrastructure/secrets/hmac_key_provider.go`

## Phase 5: User Story 3 — Rollup (Priority: P1)

### Tests cho User Story 3

- [ ] T022 [P] [US3] Test F080-AC-08: golden dataset → rollup/cohort/funnel/content/perf khớp, `is_final` (TC-ANL-001, TC-DASH-004) — file: `tests/integration/analytics/rollup_golden_test.go` — phải FAIL trước khi code
- [ ] T023 [P] [US3] Test F080-AC-09: trễ 30 h tính lại + tăng phiên bản cache; 50 h chỉ metric (TC-DASH-005) — file: `tests/integration/analytics/late_events_test.go`
- [ ] T065 [P] [US3] Test F080-AC-35: golden dataset `break_reminder_shown` (phiên 0/1/2 lần nhắc, sự kiện trùng) → `player_days.reminders_shown`, `sessions_over_reminder`, `sessions_over_second_reminder` khớp tính tay (TC-ANL-001) — file: `tests/integration/analytics/wellbeing_rollup_test.go`
- [ ] T024 [P] [US3] Test F080-AC-10: chỉ số chơi lành mạnh; catalog không có chỉ số tổng thời gian chơi làm mục tiêu (TC-ANL-001) — file: `internal/domain/analytics/metric_catalog_test.go`

### Implementation cho User Story 3

- [ ] T025 [US3] Command `RunHourlyRollup`, `CloseDay`, `ReprocessLate`; job `rollup_hourly.go`, `day_close.go`, `analytics_retention.go` — file: `internal/application/command/analytics/`, `internal/presentation/job/`

## Phase 6: User Story 4 — API platform (Priority: P1)

### Tests cho User Story 4

- [ ] T026 [P] [US4] Test F080-AC-11: 5 query đọc replica role `game_dashboard`, `tenant_ids` + audit, `grain`/`data_as_of`/`is_partial`, ≤ 120 điểm (TC-DASH-003, TC-DASH-001) — file: `tests/integration/analytics/admin_queries_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US4] Test F080-AC-12: 731 ngày → 422; đồng thời → 429; timeout → 504 (TC-DASH-002, TC-DASH-006) — file: `tests/integration/analytics/query_limits_test.go`
- [ ] T028 [P] [US4] Test F080-AC-13: export job, link ký, giới hạn đồng thời, audit (TC-DASH-008) — file: `tests/integration/analytics/export_test.go`
- [ ] T029 [P] [US4] Test F080-AC-14 authorization: role tenant → 403, thiếu MFA → `MFA_REQUIRED`, gateway public không route (TC-ANL-006) — file: `tests/integration/analytics/admin_authz_test.go`

### Implementation cho User Story 4

- [ ] T030 [US4] Query catalog + `GetMetrics`, `GetRetention`, `GetFunnel`, `GetContentStats`, `GetPerformance`, `GetExport`; `DashboardCache` — file: `internal/application/query/analytics/`, `internal/infrastructure/cache/dashboard_cache.go`
- [ ] T031 [US4] Command `CreateExport`, job `export_worker.go`, `ExportStorage`; handler admin analytics — file: `internal/application/command/analytics/create_export.go`, `internal/presentation/http/admin/analytics.go`

## Phase 7: User Story 5 — Báo cáo tenant (Priority: P2)

### Tests cho User Story 5

- [ ] T032 [P] [US5] Test F080-AC-15 tenant isolation: chỉ tenant phiên, School Admin chỉ trường mình (TC-ANL-004, TC-DASH-009) — file: `tests/integration/analytics/tenant_summary_scope_test.go` — phải FAIL trước khi code
- [ ] T033 [P] [US5] Test F080-AC-16: ô < `analytics_min_cell_size` null, không suy ra được (TC-ANL-005) — file: `tests/integration/analytics/suppression_test.go`

### Implementation cho User Story 5

- [ ] T034 [US5] Query `GetTenantSummary` + handler `getTenantGameSummary` — file: `internal/application/query/analytics/get_tenant_summary.go`, `internal/presentation/http/public/tenant_analytics.go`

### Báo cáo hoạt động game cho nhà trường (chuyển từ F081 US4, F080-AC-27…32)

- [ ] T055 [US5] Contract + setup: mở rộng `TenantGameSummary` (field ở [data-model §5](data-model.md)) + `404` cho `getTenantGameSummary`; mục "Hoạt động game" trong `menu.school_lms.reports` của catalog (`action.school_lms.reports.view`; không đăng ký menu tay — ADR-0090) + route lazy `/school-lms/reports/game-activity`, khóa i18n `portal.school_lms.game_report.*`; seed rollup 60 ngày `pgd-demo` có nhóm 3 học sinh (chuyển từ F081 T001–T003, T005) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml`, `web/apps/portal/src/workspaces/school-lms/routes.tsx`, `web/apps/portal/src/i18n/locales/{vi,en}/portal.school_lms.json`, `deploy/seed/analytics/rollup_pgd_demo.sql` — kiểm: `make specs-lint`, `make specs-breaking`, `pnpm gen:api`
- [ ] T056 [P] [US5] Integration test F080-AC-31: field mới chỉ đọc `analytics.rollup_*`/`funnel_daily`/`perf_daily` qua role `game_dashboard`, ẩn ô theo `analytics_min_cell_size`, `is_partial`/`data_as_of` (TC-DASH-001, TC-ANL-005) — file: `tests/integration/analytics/tenant_summary_extended_test.go` — phải FAIL trước khi code
- [ ] T057 [P] [US5] Integration test authorization + tenant isolation F080-AC-32: tenant khác `TENANT_MISMATCH`, trường ngoài scope / scope `subject` `PERMISSION_DENIED`, thiếu permission `PERMISSION_DENIED` (TC-ANL-004) — file: `tests/integration/analytics/tenant_summary_authz_test.go`
- [ ] T058 [P] [US5] Unit test UI F080-AC-27, F080-AC-28: bộ lọc, widget độc lập (≤ 4 song song), đúng nhóm chỉ số, không Xu/danh sách học sinh, thời lượng chỉ trung vị — file: `web/apps/portal/tests/unit/school-lms/game-report/GameActivityReportPage.test.tsx`
- [ ] T059 [P] [US5] Unit test UI F080-AC-29, F080-AC-30: "< 5", `suppressed_cells`, không tự cộng ô; `DASHBOARD_RANGE_TOO_LARGE`/`DASHBOARD_BUSY` (retry 2)/`DASHBOARD_QUERY_TIMEOUT`, "Tạm tính"; E2E điện thoại chỉ xem thẻ Tổng quan/Chơi lành mạnh — file: `web/apps/portal/tests/unit/school-lms/game-report/dashboardStates.test.tsx`, `web/apps/portal/tests/e2e/school-lms/game-report.spec.ts`
- [ ] T060 [US5] Mở rộng query catalog + `GetTenantSummary` (field mới, scope `subject` → `PERMISSION_DENIED`) và DTO handler — file: `internal/application/query/analytics/get_tenant_summary.go`, `internal/infrastructure/persistence/analytics/tenant_summary_repo.go`
- [ ] T061 [US5] `GameActivityReportPage` + 5 widget (`OverviewWidget`, `ProgressionWidget`, `SocialWidget`, `WellbeingWidget`, `DeviceWidget`) dùng `DashboardRangePicker`, `DashboardWidget`, `SuppressedCell`, `useDashboardQueue` của `ops-ui`; cập nhật GAME_ANALYTICS §8 (Academic Manager) — file: `web/apps/portal/src/workspaces/school-lms/game-report/GameActivityReportPage.tsx`, `docs/11-operations/GAME_ANALYTICS.md`

## Phase 8: User Story 6 — Live dashboard, giám sát sự kiện (Priority: P1)

### Tests cho User Story 6

- [ ] T035 [P] [US6] Test F080-AC-17: `getLiveOpsOverview` từ Prometheus (testcontainer), ngưỡng trạng thái, 503 khi nguồn lỗi — file: `tests/integration/liveops/overview_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US6] Test F080-AC-18: quét `/metrics` đủ metric FR-OPS-003, nhãn không user id — file: `tests/contract/metrics_catalog_test.go`
- [ ] T037 [P] [US6] Test F080-AC-19: giám sát + tạm dừng/tiếp tục/hủy lý do bắt buộc, xác nhận 2 bước, `Idempotency-Key` (TC-EVT-003) — file: `src/workspaces/live-ops/__tests__/EventMonitorPage.test.tsx`
- [ ] T038 [P] [US6] Test F080-AC-20: làm mới 15–30 s, "Cập nhật lúc", link sang Kiểm duyệt (TC-UI-004) — file: `src/workspaces/live-ops/__tests__/LiveDashboardPage.test.tsx`

### Implementation cho User Story 6

- [ ] T039 [US6] `MetricsSource` Prometheus + query `GetLiveOpsOverview` + handler — file: `internal/infrastructure/prometheus/metrics_source.go`, `internal/application/query/liveops/get_overview.go`, `internal/presentation/http/admin/liveops.go`
- [ ] T040 [US6] `LiveDashboardPage`, `KpiCards`, `EventMonitorPage`, `EventRunActions` — file: `src/workspaces/live-ops/dashboard/`, `src/workspaces/live-ops/events/`

## Phase 9: User Story 7 — UI Game Analytics (Priority: P1)

### Tests cho User Story 7

- [ ] T041 [P] [US7] Test F080-AC-21: 11 panel, bộ lọc, so sánh kỳ, ≤ 4 yêu cầu song song (TC-ANL-008) — file: `src/workspaces/live-ops/__tests__/GameAnalyticsPage.test.tsx` — phải FAIL trước khi code
- [ ] T042 [P] [US7] Test F080-AC-22: quy tắc UI dashboard, thử lại `DASHBOARD_BUSY` ≤ 2 (TC-DASH-003) — file: `web/packages/ops-ui/src/dashboard/__tests__/DashboardWidget.test.tsx`
- [ ] T043 [P] [US7] Test F080-AC-23: link sang workspace A, Xuất bất đồng bộ, vị trí chỉ số thời lượng (TC-DASH-008) — file: `src/workspaces/live-ops/__tests__/AnalyticsActions.test.tsx`

### Implementation cho User Story 7

- [ ] T044 [US7] Hook `analytics.ts`; `GameAnalyticsPage`, `AnalyticsFilterBar`, 11 panel — file: `src/workspaces/live-ops/analytics/`

## Phase 10: User Story 8 — Bảo vệ workspace E (Priority: P1)

### Tests cho User Story 8

- [ ] T045 [P] [US8] E2E F080-AC-24: `sm.antoan`, `ad.donvi` → 403; role tenant không tải chunk (TC-CMS-WS-01) — file: `tests/e2e/live-ops/authz.spec.ts` — phải FAIL trước khi code
- [ ] T046 [P] [US8] Test F080-AC-25: step-up MFA, VPN (TC-CMS-WS-02) — file: `src/workspaces/live-ops/__tests__/errors.test.tsx`
- [ ] T047 [P] [US8] Test F080-AC-26: `maxLength`/`maxItems` tham số → 422 (TC-TEN-024) — file: `tests/integration/analytics/param_limits_test.go`

### Implementation cho User Story 8

- [ ] T048 [US8] Nối `RequirePermission`, `MfaStepUp`, `VpnRequiredNotice` cho route live-ops (trừ Kiểm duyệt của F122) — file: `src/workspaces/live-ops/routes.tsx`

## Phase N: Polish & Cross-Cutting

- [ ] T049 [P] Tải dashboard 2 năm (TC-ANL-009, TC-DASH-010) — file: `tests/perf/analytics/dashboard_load.js`
- [ ] T050 [P] Alert `AnalyticsRawRowsHigh`, `RollupLagHigh`, `DayCloseNotDone` — file: `deploy/observability/prometheus/rules/analytics.yaml`
- [ ] T051 [P] axe + visual regression Live dashboard, Game Analytics (TC-UI-004, TC-UI-006) — file: `tests/e2e/live-ops/a11y-visual.spec.ts`
- [ ] T052 [P] Cập nhật DATABASE_DESIGN §2.8b (`rollup_monthly`, `export_jobs`, cột mới) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T053 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T054 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng
- [ ] T062 [P] Test F080-AC-33: tắt `action.live_ops.game_analytics.export` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `web/apps/portal/tests/e2e/live-ops/analytics-feature-toggle.spec.ts`
- [ ] T063 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F080-AC-33) — file: `web/apps/portal/src/workspaces/live-ops/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T005–T012) → US1 → US2 → US3 → US4, US5 (song song) → US6 → US7 → US8 (song song từ US6) → Polish.
- US2 chỉ xử lý event đã có payload schema (T007); US6 cần F077 cho event run.
- T055–T061 (màn "Hoạt động game" chuyển từ F081): T060 sau T034; T061 sau T060 và cần F052 (chunk `school-lms`), F056 (component `ops-ui/src/dashboard/`). Không phụ thuộc F081.

## Definition of Done

- [ ] Mọi tiêu chí F080-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` (MOD=game), `make web-lint`, `make web-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
