# Tasks: F080 — Game Analytics và Live Ops dashboard (workspace E)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Backend gốc `src/backend/modules/game/`; frontend gốc `src/cms/frontend/apps/portal/`. MSW chỉ trong test (`// MOCK (test-only)`, R17).

## Phase 1: Setup

- [x] T001 Đăng ký permission `game.analytics.read`, `game.liveops.read`, `game.event.operate`, `analytics.tenant.read`, `analytics.school.read` vào kiểm tra catalog — file: `src/backend/modules/game/internal/bootstrap/authz.go`
- [x] T002 [P] Cấu hình: durable consumer `analytics`, lịch `rollup-hourly`, `dashboard-day-close`, `analytics-retention`, `export-worker`; env `EW_PROMETHEUS_URL`, secret khóa HMAC (chỉ `.env.example`) — file: `src/backend/modules/game/config/worker.yaml`, `src/backend/modules/game/.env.example`
- [x] T003 [P] Golden dataset + script phát event — file: `src/backend/modules/game/tests/fixtures/analytics/golden/`
- [x] T004 [P] Route lazy `/live-ops/{dashboard,analytics,events}` + `RequirePermission` — file: `src/workspaces/live-ops/routes.tsx`

## Phase 2: Foundational (chặn mọi user story)

- [x] T005 Delta contract: 7 operation `/admin/analytics/game/*`, `getLiveOpsOverview`, schema response — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [x] T006 [P] Delta contract: `TelemetryBatch`/`TelemetryEvent` `additionalProperties: false`; `ClientConfig.product_telemetry_enabled` (owner F014); `TenantSettings.product_telemetry_enabled` (owner F015) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [x] T007 [P] Xác nhận payload schema event nguồn (GAME_ANALYTICS §3.1, `tenant.updated`) đã có; thiếu → consumer bỏ qua event đó và ghi metric, không tự định nghĩa — file: `specs/events/payloads/` (chỉ đọc)
- [x] T008 [P] Cập nhật API_DESIGN §9 (`getGameAnalyticsExport`, `getLiveOpsOverview`), GAME_ANALYTICS §6 theo DATABASE_DESIGN — file: `docs/07-backend/API_DESIGN.md`, `docs/11-operations/GAME_ANALYTICS.md`
- [x] T009 ent schema + migration: cột mới raw/player_days/rollup, `rollup_monthly`, `export_jobs`, `ref.tenants.product_telemetry_enabled`; partition; role `game_dashboard` (SELECT rollup) — file: `ent/schema/analytics/*.go`, `deploy/postgres/roles/game.sql` — kiểm: `make migrate-diff MOD=game`, `make migrate-lint MOD=game`
- [x] T010 [P] Domain: `EventMapping`, `PlayerKey`, `MetricCatalog`, `Suppress`, `GrainFor` + unit test bảng — file: `internal/domain/analytics/`
- [x] T011 [P] `ops-ui` `GrainLabel`, `DataAsOf` + story; dùng lại `DashboardWidget`, `SuppressedCell` của F056 (không tạo component ẩn ô thứ hai, DESIGN_SYSTEM §4) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/`
- [x] T012 Sinh lại client TS — file: `src/api/schema.d.ts` — kiểm: `pnpm gen:api`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Telemetry (Priority: P1) 🎯 MVP

**Goal**: Thu telemetry không PII.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T013 [P] [US1] Test F080-AC-01: schema đóng → 422, khử trùng `client_event_id`, 202 (TC-ANL-003) — file: `tests/integration/analytics/telemetry_ingest_test.go` — phải FAIL trước khi code
- [x] T014 [P] [US1] Test F080-AC-02: rate limit 429 + `Retry-After`; token không phải học sinh → 403 — file: `tests/integration/analytics/telemetry_authz_test.go`
- [x] T015 [P] [US1] Test F080-AC-03: tenant tắt → bỏ `screen_view`/`tutorial_step`, giữ perf/crash; client-config trả cờ (TC-ANL-007) — file: `tests/integration/analytics/telemetry_tenant_toggle_test.go`
- [x] T064 [P] [US1] Test F080-AC-34: `break_reminder_shown` hợp lệ → raw `source = client` với `session_key` của token; khóa lạ/`reminder_no` 0 hoặc 21 → 422; trùng `client_event_id` → 1 dòng; tenant tắt telemetry sản phẩm vẫn ghi (TC-ANL-003) — file: `tests/integration/analytics/telemetry_break_reminder_test.go`
- [x] T016 [P] [US1] Test F080-AC-04: crash → error tracking, log không PII (TC-ANL-003) — file: `tests/unit/analytics/telemetry_logging_test.go`

### Implementation cho User Story 1

- [x] T017 [US1] Command `IngestTelemetry` (gồm `break_reminder_shown`, không lọc bởi cờ telemetry sản phẩm, `session_key` từ game session); handler `postTelemetry` mở rộng; consumer `tenant.updated` → `ref.tenants` — file: `internal/application/command/analytics/ingest_telemetry.go`, `internal/presentation/http/public/telemetry.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Consumer và raw (Priority: P1)

### Tests cho User Story 2

- [x] T018 [P] [US2] Test F080-AC-05: ánh xạ event, `player_key` HMAC, chiều chuẩn, bỏ khóa lạ, idempotent (TC-ANL-002, TC-SVC-003) — file: `tests/integration/analytics/consumer_test.go` — phải FAIL trước khi code
- [x] T019 [P] [US2] Test F080-AC-06: quét schema `analytics` + `props` không định danh (TC-ANL-003) — file: `tests/integration/analytics/schema_scan_test.go`
- [x] T020 [P] [US2] Test F080-AC-07: ẩn danh `player_key`, rollup không đổi, không DELETE (TC-ANL-010) — file: `tests/integration/analytics/anonymize_test.go`

### Implementation cho User Story 2

- [x] T021 [US2] Command `IngestDomainEvent`, `AnonymizePlayer`; consumer `analytics_consumer.go`; `HmacKeyProvider` — file: `internal/application/command/analytics/`, `internal/presentation/consumer/analytics_consumer.go`, `internal/infrastructure/secrets/hmac_key_provider.go`

## Phase 5: User Story 3 — Rollup (Priority: P1)

### Tests cho User Story 3

- [x] T022 [P] [US3] Test F080-AC-08: golden dataset → rollup/cohort/funnel/content/perf khớp, `is_final` (TC-ANL-001, TC-DASH-004) — file: `tests/integration/analytics/rollup_golden_test.go` — phải FAIL trước khi code
- [x] T023 [P] [US3] Test F080-AC-09: trễ 30 h tính lại + tăng phiên bản cache; 50 h chỉ metric (TC-DASH-005) — file: `tests/integration/analytics/late_events_test.go`
- [x] T065 [P] [US3] Test F080-AC-35: golden dataset `break_reminder_shown` (phiên 0/1/2 lần nhắc, sự kiện trùng) → `player_days.reminders_shown`, `sessions_over_reminder`, `sessions_over_second_reminder` khớp tính tay (TC-ANL-001) — file: `tests/integration/analytics/wellbeing_rollup_test.go`
- [x] T024 [P] [US3] Test F080-AC-10: chỉ số chơi lành mạnh; catalog không có chỉ số tổng thời gian chơi làm mục tiêu (TC-ANL-001) — file: `internal/domain/analytics/metric_catalog_test.go`

### Implementation cho User Story 3

- [x] T025 [US3] Command `RunHourlyRollup`, `CloseDay`, `ReprocessLate`; job `rollup_hourly.go`, `day_close.go`, `analytics_retention.go` — file: `internal/application/command/analytics/`, `internal/presentation/job/`

## Phase 6: User Story 4 — API platform (Priority: P1)

### Tests cho User Story 4

- [x] T026 [P] [US4] Test F080-AC-11: 5 query đọc replica role `game_dashboard`, `tenant_ids` + audit, `grain`/`data_as_of`/`is_partial`, ≤ 120 điểm (TC-DASH-003, TC-DASH-001) — file: `tests/integration/analytics/admin_queries_test.go` — phải FAIL trước khi code
- [x] T027 [P] [US4] Test F080-AC-12: 731 ngày → 422; đồng thời → 429; timeout → 504 (TC-DASH-002, TC-DASH-006) — file: `tests/integration/analytics/query_limits_test.go`
- [x] T028 [P] [US4] Test F080-AC-13: export job, link ký, giới hạn đồng thời, audit (TC-DASH-008) — file: `tests/integration/analytics/export_test.go`
- [x] T029 [P] [US4] Test F080-AC-14 authorization: role tenant → 403, thiếu MFA → `MFA_REQUIRED`, gateway public không route (TC-ANL-006) — file: `tests/integration/analytics/admin_authz_test.go`

### Implementation cho User Story 4

- [x] T030 [US4] Query catalog + `GetMetrics`, `GetRetention`, `GetFunnel`, `GetContentStats`, `GetPerformance`, `GetExport`; `DashboardCache` — file: `internal/application/query/analytics/`, `internal/infrastructure/cache/dashboard_cache.go`
- [x] T031 [US4] Command `CreateExport`, job `export_worker.go`, `ExportStorage`; handler admin analytics — file: `internal/application/command/analytics/create_export.go`, `internal/presentation/http/admin/analytics.go`

## Phase 7: User Story 5 — Báo cáo tenant (Priority: P2)

### Tests cho User Story 5

- [x] T032 [P] [US5] Test F080-AC-15 tenant isolation: chỉ tenant phiên, School Admin chỉ trường mình (TC-ANL-004, TC-DASH-009) — file: `tests/integration/analytics/tenant_summary_scope_test.go` — phải FAIL trước khi code
- [x] T033 [P] [US5] Test F080-AC-16: ô < `analytics_min_cell_size` null, không suy ra được (TC-ANL-005) — file: `tests/integration/analytics/suppression_test.go`

### Implementation cho User Story 5

- [x] T034 [US5] Query `GetTenantSummary` + handler `getTenantGameSummary` — file: `internal/application/query/analytics/get_tenant_summary.go`, `internal/presentation/http/public/tenant_analytics.go`

### Báo cáo hoạt động game cho nhà trường (chuyển từ F081 US4, F080-AC-27…32)

- [x] T055 [US5] Contract + setup: mở rộng `TenantGameSummary` (field ở [data-model §5](data-model.md)) + `404` cho `getTenantGameSummary`; mục "Hoạt động game" trong `menu.school_lms.reports` của catalog (`action.school_lms.reports.view`; không đăng ký menu tay — ADR-0090) + route lazy `/school-lms/reports/game-activity`, khóa i18n `portal.school_lms.game_report.*`; seed rollup 60 ngày `pgd-demo` có nhóm 3 học sinh (chuyển từ F081 T001–T003, T005) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx`, `src/cms/frontend/apps/portal/src/i18n/locales/{vi,en}/portal.school_lms.json`, `deploy/seed/analytics/rollup_pgd_demo.sql` — kiểm: `make specs-lint`, `make specs-breaking`, `pnpm gen:api`
- [x] T056 [P] [US5] Integration test F080-AC-31: field mới chỉ đọc `analytics.rollup_*`/`funnel_daily`/`perf_daily` qua role `game_dashboard`, ẩn ô theo `analytics_min_cell_size`, `is_partial`/`data_as_of` (TC-DASH-001, TC-ANL-005) — file: `tests/integration/analytics/tenant_summary_extended_test.go` — phải FAIL trước khi code
- [x] T057 [P] [US5] Integration test authorization + tenant isolation F080-AC-32: tenant khác `TENANT_MISMATCH`, trường ngoài scope / scope `subject` `PERMISSION_DENIED`, thiếu permission `PERMISSION_DENIED` (TC-ANL-004) — file: `tests/integration/analytics/tenant_summary_authz_test.go`
- [x] T058 [P] [US5] Unit test UI F080-AC-27, F080-AC-28: bộ lọc, widget độc lập (≤ 4 song song), đúng nhóm chỉ số, không Xu/danh sách học sinh, thời lượng chỉ trung vị — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/game-report/GameActivityReportPage.test.tsx`
- [x] T059 [P] [US5] Unit test UI F080-AC-29, F080-AC-30: "< 5", `suppressed_cells`, không tự cộng ô; `DASHBOARD_RANGE_TOO_LARGE`/`DASHBOARD_BUSY` (retry 2)/`DASHBOARD_QUERY_TIMEOUT`, "Tạm tính"; E2E điện thoại chỉ xem thẻ Tổng quan/Chơi lành mạnh — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/game-report/dashboardStates.test.tsx`, `src/cms/frontend/apps/portal/tests/e2e/school-lms/game-report.spec.ts`
- [x] T060 [US5] Mở rộng query catalog + `GetTenantSummary` (field mới, scope `subject` → `PERMISSION_DENIED`) và DTO handler — file: `internal/application/query/analytics/get_tenant_summary.go`, `internal/infrastructure/persistence/analytics/tenant_summary_repo.go`
- [x] T061 [US5] `GameActivityReportPage` + 5 widget (`OverviewWidget`, `ProgressionWidget`, `SocialWidget`, `WellbeingWidget`, `DeviceWidget`) dùng `DashboardRangePicker`, `DashboardWidget`, `SuppressedCell`, `useDashboardQueue` của `ops-ui`; cập nhật GAME_ANALYTICS §8 (Academic Manager) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/game-report/GameActivityReportPage.tsx`, `docs/11-operations/GAME_ANALYTICS.md`

## Phase 8: User Story 6 — Live dashboard, giám sát sự kiện (Priority: P1)

### Tests cho User Story 6

- [x] T035 [P] [US6] Test F080-AC-17: `getLiveOpsOverview` từ Prometheus (testcontainer), ngưỡng trạng thái, 503 khi nguồn lỗi — file: `tests/integration/liveops/overview_test.go` — phải FAIL trước khi code
- [x] T036 [P] [US6] Test F080-AC-18: quét `/metrics` đủ metric FR-OPS-003, nhãn không user id — file: `tests/contract/metrics_catalog_test.go`
- [x] T037 [P] [US6] Test F080-AC-19: giám sát + tạm dừng/tiếp tục/hủy lý do bắt buộc, xác nhận 2 bước, `Idempotency-Key` (TC-EVT-003) — file: `src/workspaces/live-ops/__tests__/EventMonitorPage.test.tsx`
- [x] T038 [P] [US6] Test F080-AC-20: làm mới 15–30 s, "Cập nhật lúc", link sang Kiểm duyệt (TC-UI-004) — file: `src/workspaces/live-ops/__tests__/LiveDashboardPage.test.tsx`

### Implementation cho User Story 6

- [x] T039 [US6] `MetricsSource` Prometheus + query `GetLiveOpsOverview` + handler — file: `internal/infrastructure/prometheus/metrics_source.go`, `internal/application/query/liveops/get_overview.go`, `internal/presentation/http/admin/liveops.go`
- [x] T040 [US6] `LiveDashboardPage`, `KpiCards`, `EventMonitorPage`, `EventRunActions` — file: `src/workspaces/live-ops/dashboard/`, `src/workspaces/live-ops/events/`

## Phase 9: User Story 7 — UI Game Analytics (Priority: P1)

### Tests cho User Story 7

- [x] T041 [P] [US7] Test F080-AC-21: 11 panel, bộ lọc, so sánh kỳ, ≤ 4 yêu cầu song song (TC-ANL-008) — file: `src/workspaces/live-ops/__tests__/GameAnalyticsPage.test.tsx` — phải FAIL trước khi code
- [x] T042 [P] [US7] Test F080-AC-22: quy tắc UI dashboard, thử lại `DASHBOARD_BUSY` ≤ 2 (TC-DASH-003) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/__tests__/DashboardWidget.test.tsx`
- [x] T043 [P] [US7] Test F080-AC-23: link sang workspace A, Xuất bất đồng bộ, vị trí chỉ số thời lượng (TC-DASH-008) — file: `src/workspaces/live-ops/__tests__/AnalyticsActions.test.tsx`

### Implementation cho User Story 7

- [x] T044 [US7] Hook `analytics.ts`; `GameAnalyticsPage`, `AnalyticsFilterBar`, 11 panel — file: `src/workspaces/live-ops/analytics/`

## Phase 10: User Story 8 — Bảo vệ workspace E (Priority: P1)

### Tests cho User Story 8

- [ ] T045 [P] [US8] E2E F080-AC-24: `sm.antoan`, `ad.donvi` → 403; role tenant không tải chunk (TC-CMS-WS-01) — file: `tests/e2e/live-ops/authz.spec.ts` — phải FAIL trước khi code
- [x] T046 [P] [US8] Test F080-AC-25: step-up MFA, VPN (TC-CMS-WS-02) — file: `src/workspaces/live-ops/__tests__/errors.test.tsx`
- [x] T047 [P] [US8] Test F080-AC-26: `maxLength`/`maxItems` tham số → 422 (TC-TEN-024) — file: `tests/integration/analytics/param_limits_test.go`

### Implementation cho User Story 8

- [x] T048 [US8] Nối `RequirePermission`, `MfaStepUp`, `VpnRequiredNotice` cho route live-ops (trừ Kiểm duyệt của F122) — file: `src/workspaces/live-ops/routes.tsx`

## Phase N: Polish & Cross-Cutting

- [ ] T049 [P] Tải dashboard 2 năm (TC-ANL-009, TC-DASH-010) — file: `tests/perf/analytics/dashboard_load.js`
- [x] T050 [P] Alert `AnalyticsRawRowsHigh`, `RollupLagHigh`, `DayCloseNotDone` — file: `deploy/observability/prometheus/rules/analytics.yaml`
- [ ] T051 [P] axe + visual regression Live dashboard, Game Analytics (TC-UI-004, TC-UI-006) — file: `tests/e2e/live-ops/a11y-visual.spec.ts`
- [x] T052 [P] Cập nhật DATABASE_DESIGN §2.8b (`rollup_monthly`, `export_jobs`, cột mới) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [x] T053 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T054 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng
- [ ] T062 [P] Test F080-AC-33: tắt `action.live_ops.game_analytics.export` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/live-ops/analytics-feature-toggle.spec.ts`
- [x] T063 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F080-AC-33) — file: `src/cms/frontend/apps/portal/src/workspaces/live-ops/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T005–T012) → US1 → US2 → US3 → US4, US5 (song song) → US6 → US7 → US8 (song song từ US6) → Polish.
- US2 chỉ xử lý event đã có payload schema (T007); US6 cần F077 cho event run.
- T055–T061 (màn "Hoạt động game" chuyển từ F081): T060 sau T034; T061 sau T060 và cần F052 (chunk `school-lms`), F056 (component `ops-ui/src/dashboard/`). Không phụ thuộc F081.

## Definition of Done

- [x] Mọi tiêu chí F080-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` (MOD=game), `make web-lint`, `make web-test` xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Triển khai (2026-11-26)

- Backend: migration `src/backend/modules/game/migrations/20270102000080_f080_analytics.sql` (bảng `analytics.*` viết tay, RLS; role `game_dashboard` ở `deploy/postgres/init/02-roles.sh`); domain `internal/domain/analytics`; command `analytics_ingest.go`, `analytics_rollup.go`, `analytics_export.go`; query `analytics.go`, `analytics_tenant.go`, `analytics_export.go`, `analytics_flag.go`, `liveops_overview.go`, `ListEventRuns`; hạ tầng `persistence/analyticssql`, `prometheus`, `cache/analytics_dashboard.go`, `storage/exports.go`, `config/analytics.go`, `observability/analytics_metrics.go`; HTTP `http/analytics.go`, consumer `analytics.go` (4 durable `game.analytics*`), job `analytics.go`; alert `deploy/observability/prometheus/alerts/game.yml`.
- Portal: workspace E `live-ops/{dashboard,events,analytics,api,shared}`, workspace C `school-lms/game-report` (tab "Hoạt động game"), i18n `live-ops.{vi,en}.json`, `portal.school_lms.json` (`game_report`).
- **T003:** golden dataset sinh trong `tests/integration/analytics_test.go` (`seedGolden`, giá trị tính tay trong test) thay thư mục fixture; `make seed-analytics` chưa có (cần compose).
- **T011:** không thêm `GrainLabel`/`DataAsOf` — `DashboardWidget` của F056 đã hiển thị độ hạt, "Cập nhật lúc", "Tạm tính", "Chưa có số liệu trước ngày …"; `SuppressedCell` dùng lại.
- **Chưa làm (cần stack compose / Playwright / k6):** T045, T062 (E2E), phần E2E điện thoại của T059, T049 (tải k6), T051 (axe + visual), T054 (quickstart tay). Tiêu chí liên quan có test Vitest toàn portal và integration test module game (bảng dưới).

## AC → test

| Tiêu chí | Test |
|---|---|
| F080-AC-01 | Go `TestAnalyticsTelemetry` (202, lưu một lần theo `client_event_id`, thuộc tính lạ / thuộc tính của loại khác → 422); unit `TestValidateTelemetry` |
| F080-AC-02 | Go `TestAnalyticsTelemetry` (429 `RATE_LIMITED` + `Retry-After`; token nhân sự → 403 `GAME_ACCESS_STUDENT_ONLY`) |
| F080-AC-03 | Go `TestAnalyticsTelemetry` (tenant tắt → bỏ `screen_view`/`tutorial_step`, giữ perf/nhắc nghỉ; `getClientConfig?tenant_id=` trả `false`); unit `TestDropTelemetry` |
| F080-AC-04 | Go `TestAnalyticsTelemetry` (log lỗi có `crash_signature`, không student id/token; `crash_signature` không vào analytics); unit `TestValidateTelemetry` |
| F080-AC-05 | Go `TestAnalyticsConsumerAndAnonymize` (HMAC `player_key`/`session_key`, chiều từ projection, một dòng/event_id, một dòng/học sinh của trận, học sinh lạ bỏ); unit `TestMap` |
| F080-AC-06 | Go `TestAnalyticsConsumerAndAnonymize` (quét `information_schema` + `props`); contract `TestAnalyticsSchemaNoIdentity` |
| F080-AC-07 | Go `TestAnalyticsConsumerAndAnonymize` (khóa NULL ở raw/`player_days`, số dòng và rollup không đổi) |
| F080-AC-08 | Go `TestAnalyticsRollupGolden` (dau, phiên/người, wau/mau, retention d1, funnel, tuần, nội dung, hiệu năng, `is_final`) |
| F080-AC-09 | Go `TestAnalyticsLateEvents` (trễ 30 h → chốt lại + tăng phiên bản cache; trễ 50 h → bỏ); unit `TestLateAndWindow` |
| F080-AC-10 | Go `TestAnalyticsRollupGolden` (`late_night_play_rate`, `sessions_over_*`); unit `TestCatalogWellbeingAndNoTotalPlaytime` |
| F080-AC-11 | Go `TestAnalyticsAdminQueries` (grain/data_as_of/is_partial, ≤ 120 điểm, audit `cross_tenant.accessed`, `game_dashboard` không đọc `events_raw`); contract `TestAnalyticsDashboardCatalogTables` |
| F080-AC-12 | Go `TestAnalyticsAdminQueries` (731 ngày → 422 `DASHBOARD_RANGE_TOO_LARGE`, thiếu `from` → 422, 429 `DASHBOARD_BUSY` + `Retry-After`, 504 `DASHBOARD_QUERY_TIMEOUT`); unit `TestRangeGrain` |
| F080-AC-13 | Go `TestAnalyticsExport` (202, worker CSV, link ký 15 phút, 429 `EXPORT_LIMIT_REACHED`, người khác 404, `report.exported`) |
| F080-AC-14 | Go `TestAnalyticsAdminQueries`, `TestAnalyticsExport` (role tenant, Safety Moderator → 403 `PERMISSION_DENIED`) |
| F080-AC-15 | Go `TestTenantGameSummary` (chỉ tenant phiên, School Admin chỉ trường mình, trường tenant khác 404) |
| F080-AC-16 | Go `TestTenantGameSummary`, `TestAnalyticsAdminQueries` (ô < 5 null + ẩn bổ sung, `suppressed_cells`, funnel bước nhỏ ẩn); unit `TestSuppressComplementary` |
| F080-AC-17 | Go `TestLiveOpsOverview` (catalog PromQL, trạng thái ngưỡng, không chuỗi → null/unknown, Prometheus lỗi → 503, chỉ `game.liveops.read`) |
| F080-AC-18 | contract `TestLiveOpsMetricsCatalog` (đủ KPI FR-OPS-003, metric đã đăng ký, nhãn không định danh) |
| F080-AC-19 | Go `TestListEventRuns`; Vitest `EventMonitorPage.test.tsx` (lý do, xác nhận 2 bước, `Idempotency-Key` mỗi thao tác, 409 tải lại, ẩn nút khi thiếu quyền) |
| F080-AC-20 | Vitest `LiveDashboardPage.test.tsx` (làm mới 20 s, "Cập nhật lúc", link Kiểm duyệt khi có quyền, 503 không số 0) |
| F080-AC-21 | Vitest `GameAnalyticsPage.test.tsx` (11 panel, ≤ 4 yêu cầu song song, so sánh kỳ, một chiều cắt); `analytics/model.test.ts` |
| F080-AC-22 | Vitest `GameAnalyticsPage.test.tsx` (độ hạt, "Cập nhật lúc", "Tạm tính", "Chưa có số liệu trước ngày …", BUSY thử lại ≤ 2, RANGE/TIMEOUT) |
| F080-AC-23 | Vitest `GameAnalyticsPage.test.tsx` (link workspace A, xuất bất đồng bộ + thông báo, chỉ số thời lượng chỉ ở Chơi lành mạnh/trung vị) |
| F080-AC-24 | Vitest `LiveDashboardPage.test.tsx` (Safety Moderator → 403, role tenant không tải chunk) |
| F080-AC-25 | Vitest `LiveDashboardPage.test.tsx` (403 kèm `trace_id`, gateway nội bộ không tới → thông báo VPN) |
| F080-AC-26 | Go `TestAnalyticsAdminQueries` (21 `client_versions` → 422), `TestAnalyticsTelemetry` (giới hạn schema → 422); Vitest `limits.test.ts` |
| F080-AC-27 | Vitest `GameActivityReport.test.tsx` (khoảng, trường của mọi trang chỉ với tenant `unit`, khối, widget độc lập) |
| F080-AC-28 | Vitest `GameActivityReport.test.tsx` (nhóm §7.2, không Xu/danh sách/tổng thời gian chơi, điện thoại chỉ Tổng quan + Chơi lành mạnh); Go `TestTenantGameSummary` (không trường định danh/kinh tế) |
| F080-AC-29 | Vitest `GameActivityReport.test.tsx` ("< 5" + chú thích `suppressed_cells`, không tự cộng) |
| F080-AC-30 | Vitest `GameActivityReport.test.tsx` (độ hạt, "Cập nhật lúc", "Tạm tính", RANGE/BUSY/TIMEOUT); Go `TestTenantGameSummary` (731 ngày → 422) |
| F080-AC-31 | Go `TestTenantGameSummary` (raw chưa rollup không đổi kết quả); contract `TestAnalyticsDashboardCatalogTables` |
| F080-AC-32 | Go `TestTenantGameSummary` (`TENANT_MISMATCH`, trường ngoài scope / scope `subject` / thiếu quyền → `PERMISSION_DENIED`); Vitest `GameActivityReport.test.tsx` (không tab khi thiếu quyền) |
| F080-AC-33 | Vitest `LiveDashboardPage.test.tsx`, `GameAnalyticsPage.test.tsx`, `GameActivityReport.test.tsx` (menu/nút tắt ẩn, `403 FEATURE_DISABLED` → thông báo + tải lại quyền) |
| F080-AC-34 | Go `TestAnalyticsTelemetry` (`session_key` từ game session, 422 ngoài 1–20 / thiếu thuộc tính, lưu một lần, vẫn ghi khi tắt telemetry sản phẩm); unit `TestValidateTelemetry` |
| F080-AC-35 | Go `TestAnalyticsRollupGolden` (`reminders_shown` = 2 dù sự kiện trùng, `sessions_over_reminder` 2/10, `sessions_over_second_reminder` 1/10) |
