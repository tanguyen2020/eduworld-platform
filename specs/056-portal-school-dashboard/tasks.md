# Tasks: F056 — Workspace C — School Learning Dashboard và báo cáo

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo module `dashboard` trong workspace C, route lazy `/portal/school-lms/dashboard/*` có guard permission — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/routes.tsx` — ✅ menu `menu.school_lms.school_dashboard` (path từ catalog `/school-lms/school-dashboard/*`, MenuGate) → `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/SchoolDashboardScreen.tsx` (index, `classes/:id`, `students/:id`, `reference`); `results`, `reports`, `platform_admin.reports` gắn vào `index.tsx` của workspace
- [x] T002 [P] Thêm ECharts (ghim phiên bản) vào `ops-ui`, khung thư mục `src/dashboard` — file: `src/cms/frontend/packages/ops-ui/package.json` — ✅ `echarts` 5.6.0 ghim trong `packages/ops-ui/package.json` (lockfile), sub-path `@eduworld/ops-ui/dashboard`, tải theo nhu cầu (chunk `charts`/`renderers` riêng, `check-chunks` 5 workspace)
- [x] T003 [P] File dịch vi/en (`dashboard.*`, `error.DASHBOARD_*`, `error.EXPORT_LIMIT_REACHED`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/i18n/vi.json`, `en.json` — ✅ `apps/portal/src/i18n/dashboard.{vi,en}.json` (`dashboard.*`, gộp trong `i18n/index.ts`) + `packages/ops-ui/src/i18n` (`dashboard.*`); test cùng khóa vi/en (`model/filters.test.ts`); lỗi `DASHBOARD_*`/`EXPORT_LIMIT_REACHED` theo `code`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract (owner F043): `getStudentLearningProfile` + schema `StudentLearningProfile`; sửa `SchoolDashboard.widgets.self_study` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — ✅ đã có trên main từ F043 T004 (`getStudentLearningProfile`, `SchoolDashboard.widgets.self_study`) — không cần delta
- [x] T005 Delta contract (owner F044): `getExamSessionReport`, `getAssignmentItemAnalysis` + schema `ItemAnalysis`; cập nhật EXAM_ENGINE §11, DASHBOARD_DATA §14 — file: `specs/openapi/eduworld-api.v1.yaml`, `docs/02-learning/EXAM_ENGINE.md`, `docs/07-backend/DASHBOARD_DATA.md` — kiểm: `make specs-lint` — ✅ F044 T004 đã thêm `getExamSessionReport`; `getAssignmentItemAnalysis` không có — không thêm endpoint, dùng `Assignment.exam_session_id` (Clarifications 2026-11-21, câu hỏi mở F056-1)
- [x] T006 Sinh lại client TS — file: `src/cms/frontend/apps/portal/src/api/generated/eduworld-api.d.ts` — kiểm: `pnpm gen:api` + CI diff — ✅ không có delta → `pnpm gen:check` khớp spec (op-meta, menu-catalog, 8 schema)
- [x] T007 [P] Test `useDashboardQueue`: không vượt `dashboard_max_concurrent_per_user` song song, retry `DASHBOARD_BUSY` 2–5 s tối đa 2 lần — file: `src/cms/frontend/packages/ops-ui/src/dashboard/useDashboardQueue.test.ts` — phải FAIL trước khi code — ✅ `packages/ops-ui/src/dashboard/dashboardQueue.test.ts` (2 test: ≤ 4 song song FIFO, lỗi giải phóng slot) + `apps/portal/src/shared/api/middleware/retry.test.ts` (`DASHBOARD_BUSY` chờ 2000/5000 ms tối đa 2 lần, `DASHBOARD_QUERY_TIMEOUT` không lặp)
- [x] T008 [P] `useDashboardQueue` + `DashboardWidget` (skeleton, "Cập nhật lúc", "Tạm tính", "Đang tính…", lỗi theo `code`) + story — file: `src/cms/frontend/packages/ops-ui/src/dashboard/useDashboardQueue.ts`, `DashboardWidget.tsx` — ✅ `dashboardQueue.tsx` (FIFO semaphore + provider), `DashboardWidget.tsx` (`DashboardWidget`, `DashboardMetric`) + stories Light/Dark/Compact; retry `DASHBOARD_BUSY` nằm ở apiClient (`middleware/retry.ts` `wait429`, `NOT_REPEATED_CODES`)
- [x] T009 [P] `DashboardRangePicker` (preset, giới hạn `dashboard_max_range_days`, độ hạt thô hơn) + story — file: `src/cms/frontend/packages/ops-ui/src/dashboard/DashboardRangePicker.tsx` — ✅ `DashboardRangePicker.tsx` + story; preset học kỳ/năm học nhận lịch qua prop (chưa có API → vô hiệu, câu hỏi mở F056-2)
- [x] T010 [P] MSW handler test-only (nhãn `MOCK`) từ OpenAPI + golden fixtures — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/msw/handlers.ts` — ✅ theo mẫu repo: `apps/portal/src/test/dashboard-mock.ts` + `dashboard-harness.tsx` (nhãn MOCK, R17) thay MSW

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Trang tổng quan trường (Priority: P1) 🎯 MVP

**Goal**: 5 widget trả lời 5 câu hỏi tuần.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T011 [P] [US1] Test F056-AC-01: gọi `getSchoolDashboard` đúng tham số, thứ tự 5 widget — file: `…/dashboard/__tests__/OverviewPage.test.tsx` — phải FAIL trước khi code — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/overview.test.tsx` › AC-01/AC-02 pass
- [x] T012 [P] [US1] Test F056-AC-02: nhãn `data_as_of`, `grain`, `is_partial`, "Đang tính số liệu…" — file: `src/cms/frontend/packages/ops-ui/src/dashboard/DashboardWidget.test.tsx` — ✅ `packages/ops-ui/src/dashboard/DashboardWidget.test.tsx` (5 test) + overview AC-02 "Đang tính số liệu…"
- [x] T013 [P] [US1] Test F056-AC-03: preset, ngày vượt giới hạn bị vô hiệu, không "toàn bộ thời gian", độ hạt thô hơn (TC-DASH-002) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/DashboardRangePicker.test.tsx` — ✅ `packages/ops-ui/src/dashboard/DashboardRangePicker.test.tsx` (6 test)
- [x] T014 [P] [US1] Test F056-AC-04: `DASHBOARD_RANGE_TOO_LARGE`/`DASHBOARD_QUERY_TIMEOUT` gợi ý thu hẹp; widget độc lập (TC-DASH-006) — file: `…/dashboard/__tests__/OverviewPage.errors.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/overview.test.tsx` › AC-04 (504/422 gợi ý ở 5 widget, 504 không lặp; 429 BUSY thử lại ≥ 2 s rồi tải)
- [x] T015 [P] [US1] Test F056-AC-05: nhãn `reason`, "làm quá nhanh", không xếp hạng — file: `…/dashboard/__tests__/AttentionWidget.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/overview.test.tsx` › AC-05
- [x] T016 [P] [US1] Test F056-AC-06: `suppressed` → "< 5" — file: `src/cms/frontend/packages/ops-ui/src/dashboard/SuppressedCell.test.tsx` — ✅ `packages/ops-ui/src/dashboard/SuppressedCell.test.tsx` (2 test)
- [x] T017 [P] [US1] Test F056-AC-07: widget an toàn chỉ số lượng + link; quét DOM không có trường nội dung chat — file: `…/dashboard/__tests__/SafetyWidget.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/overview.test.tsx` › AC-07 (chỉ số lượng + link, không request chat)

### Implementation cho User Story 1

- [x] T018 [P] [US1] Hook `useSchoolDashboard`, `useSchoolSeries` — file: `…/dashboard/api/schoolDashboard.ts` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/api/dashboardApi.ts` (`useSchoolDashboard`, `useClassDashboard`, `useStudentProfile`, `useSubjects`, `useRegulationReference`; khóa `['school-lms','dashboard',name,tenant,…]`, qua DashboardQueue) + `model/filters.ts`, `DashboardFilterBar.tsx`; widget ④ dùng `active_minutes_series` của `SchoolDashboard` nên không gọi `getSchoolAnalyticsSeries`
- [x] T019 [P] [US1] `SuppressedCell` (chữ "< 5" + tooltip, không hiện số thật) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/SuppressedCell.tsx` — ✅ `packages/ops-ui/src/dashboard/SuppressedCell.tsx` + story
- [x] T020 [US1] Trang tổng quan + 5 widget, bố cục mobile thẻ dọc — file: `…/dashboard/overview/OverviewPage.tsx`, `…/overview/widgets/*.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/overview/OverviewPage.tsx` (5 widget, `Col xs=24` thẻ dọc trên điện thoại)
- [ ] T021 [US1] E2E tổng quan với golden dataset (TC-LMS-003) — file: `src/cms/frontend/apps/portal/e2e/dashboard/overview.spec.ts` — chờ backend compose + golden dataset F043 (logic đã có Vitest `overview.test.tsx`)

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Drill-down lớp, bản đồ kỹ năng, hồ sơ học sinh (Priority: P1)

### Tests cho User Story 2

- [x] T022 [P] [US2] Test F056-AC-08: drill-down giữ bộ lọc, breadcrumb — file: `…/dashboard/__tests__/ClassDashboardPage.test.tsx` — phải FAIL trước khi code — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/class-student.test.tsx` › AC-08/AC-09
- [x] T023 [P] [US2] Test F056-AC-09: `MasteryHeatmap` có ký hiệu, aria-label, tooltip; axe (TC-UI-006) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/MasteryHeatmap.test.tsx` — ✅ `packages/ops-ui/src/dashboard/MasteryHeatmap.test.tsx` (2 test: bảng, ký hiệu, aria-label, tooltip, focus, chú giải); axe ở Storybook test-runner
- [x] T024 [P] [US2] Test F056-AC-10: nút "Giao ôn tập theo LO" điều hướng wizard F055 với `mode=practice`, LO, lớp, học sinh — file: `…/dashboard/__tests__/AssignReviewShortcut.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/class-student.test.tsx` › AC-10
- [x] T025 [P] [US2] Test F056-AC-11: hồ sơ học sinh có các tab, không trường IP/bạn bè/chat/Game Rating — file: `…/dashboard/__tests__/StudentProfilePage.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/class-student.test.tsx` › AC-11 (+ học sinh ngoài phạm vi → "Không tìm thấy")

### Implementation cho User Story 2

- [x] T026 [P] [US2] `MasteryHeatmap` (ECharts, màu token) + story — file: `src/cms/frontend/packages/ops-ui/src/dashboard/MasteryHeatmap.tsx` — ✅ `packages/ops-ui/src/dashboard/MasteryHeatmap.tsx` (bảng HTML + token, Clarifications 2026-11-21) + `DashboardChart.tsx` (ECharts) + stories
- [x] T027 [P] [US2] Hook `useClassDashboard`, `useStudentProfile` (sau T004), `useLoginEvents` — file: `…/dashboard/api/classDashboard.ts`, `studentProfile.ts` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/api/dashboardApi.ts`; tab đăng nhập dùng `StudentLoginActivityPanel` của F052 (`listTenantStudentLoginEvents`)
- [x] T028 [US2] Trang lớp + hồ sơ học sinh — file: `…/dashboard/class/ClassDashboardPage.tsx`, `…/dashboard/student/StudentProfilePage.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/class/ClassDashboardPage.tsx`, `student/StudentProfilePage.tsx`, `reports/ReferenceLevelsPage.tsx`

## Phase 5: User Story 3 — Bảng kết quả và phân tích đề/bài (Priority: P2)

### Tests cho User Story 3

- [x] T029 [P] [US3] Test F056-AC-12: cột bài graded, `score_10`, phân trang `dashboard_page_size_max`, không cột XP/rating/nhập tay — file: `…/dashboard/__tests__/CourseResultsTab.test.tsx` — phải FAIL trước khi code — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/results-reports.test.tsx` › AC-12
- [x] T030 [P] [US3] Test F056-AC-13: phân tích theo câu, `rapid_guess` loại khỏi p-value, cảnh báo độ tin cậy, nút giao ôn tập (TC-EXM-004) — file: `…/dashboard/__tests__/ItemAnalysisTab.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/results-reports.test.tsx` › AC-13 (2 test)

### Implementation cho User Story 3

- [x] T031 [P] [US3] Hook `useCourseResults`, `useItemAnalysis` (sau T005) — file: `…/dashboard/api/courseResults.ts`, `itemAnalysis.ts` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/api/resultsApi.ts` (`useCourseResults`, `useExamSessionReport`)
- [x] T032 [US3] Tab Kết quả và tab Phân tích — file: `…/dashboard/results/CourseResultsTab.tsx`, `…/dashboard/analysis/ItemAnalysisTab.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/results/ResultsScreen.tsx`, `analysis/ItemAnalysisPage.tsx`

## Phase 6: User Story 4 — Xuất báo cáo bất đồng bộ (Priority: P2)

### Tests cho User Story 4

- [x] T033 [P] [US4] Test F056-AC-14: `createLmsReport` có `Idempotency-Key`, theo dõi `getLmsReport`, link + hạn, không tải trực tiếp (TC-DASH-008) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/ExportButton.test.tsx` — phải FAIL trước khi code — ✅ `packages/ops-ui/src/dashboard/ExportButton.test.tsx` (4 test) + `results-reports.test.tsx` › AC-14
- [x] T034 [P] [US4] Test F056-AC-15: `EXPORT_LIMIT_REACHED` và link hết hạn — file: `…/dashboard/__tests__/ReportsCenter.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/results-reports.test.tsx` › AC-15
- [x] T035 [P] [US4] Test F056-AC-16: nhãn "Tham khảo — không thay đánh giá chính thức", "Chưa đủ dữ liệu" — file: `…/dashboard/__tests__/RegulationDisclaimer.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/results-reports.test.tsx` › AC-16

### Implementation cho User Story 4

- [x] T036 [P] [US4] `ExportButton` + hook `useReportJob` — file: `src/cms/frontend/packages/ops-ui/src/dashboard/ExportButton.tsx`, `…/dashboard/api/reports.ts` — ✅ `packages/ops-ui/src/dashboard/ExportButton.tsx` + `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/api/reportsApi.ts` (`useReportExport`), `reports/ReportExport.tsx`
- [x] T037 [US4] Trung tâm báo cáo + nhãn tham khảo — file: `…/dashboard/reports/ReportsCenterPage.tsx`, `RegulationDisclaimer.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/reports/ReportsScreen.tsx`, `reports/RegulationDisclaimer.tsx`

## Phase 7: User Story 5 — Tổng hợp tenant và platform (Priority: P3)

### Tests cho User Story 5

- [x] T038 [P] [US5] Test F056-AC-17: `getTenantLearningSummary`, "< 5", export tenant (TC-ANL-005) — file: `…/dashboard/__tests__/TenantSummaryPage.test.tsx` — phải FAIL trước khi code — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/results-reports.test.tsx` › AC-17
- [x] T039 [P] [US5] Test F056-AC-18: System Admin gửi `tenant_ids[]` không gửi `X-Tenant-Id`, cột Tenant (TC-CMS-TEN-02) — file: `…/dashboard/__tests__/PlatformSummaryPage.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/results-reports.test.tsx` › AC-18

### Implementation cho User Story 5

- [x] T040 [US5] Trang tổng hợp tenant và platform — file: `…/dashboard/api/summaries.ts`, `…/dashboard/summary/TenantSummaryPage.tsx`, `PlatformSummaryPage.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/api/reportsApi.ts` (`useTenantLearningSummary`), tab "Tổng hợp đơn vị" trong `reports/ReportsScreen.tsx`, `apps/portal/src/workspaces/platform-admin/reports/PlatformReportsPage.tsx`

## Phase 8: User Story 6 — Phân quyền và cô lập tenant (Priority: P1)

### Tests cho User Story 6

- [x] T041 [P] [US6] Test F056-AC-19: menu theo `analytics.*`; bộ lọc môn theo scope; `403` → "Ngoài phạm vi quyền" (TC-TEN-002) — file: `…/dashboard/__tests__/permissions.matrix.test.tsx` — phải FAIL trước khi code — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/permissions.test.tsx` › AC-19 (2 test)
- [ ] T042 [P] [US6] E2E F056-AC-20: `sm.antoan`, `cm.noidung` mở URL dashboard → 403, không request dashboard (TC-CMS-WS-01) — file: `src/cms/frontend/apps/portal/e2e/dashboard/authz.spec.ts` — chờ backend compose; logic đã có Vitest `permissions.test.tsx` › AC-20 (trang 403, không request dashboard)
- [ ] T043 [P] [US6] E2E F056-AC-21: `ad.truong.b` mở lớp TH Demo A → "Không tìm thấy"; cache khóa theo tenant (TC-DASH-009, TC-TEN-001) — file: `src/cms/frontend/apps/portal/e2e/dashboard/tenant-isolation.spec.ts` — chờ backend compose; logic đã có Vitest `permissions.test.tsx` › AC-21 (404 → "Không tìm thấy", khóa cache theo tenant)
- [x] T044 [P] [US6] Test F056-AC-22: ô tìm kiếm/bộ lọc theo `maxLength` (TC-TEN-024) — file: `…/dashboard/__tests__/filters.test.tsx` — ✅ `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/class-student.test.tsx` › AC-22 (ô tìm học sinh `maxlength=100` = `listTenantClassStudents.q`)

### Implementation cho User Story 6

- [x] T045 [US6] Guard route/menu và xử lý `403`/`404`/`TENANT_MISMATCH` chung cho module — file: `…/dashboard/routes.tsx` — ✅ MenuGate của catalog (403/tắt) + `DashboardUi.tsx` (`DashboardPageError`, `useWidgetError`: 403 → "Ngoài phạm vi quyền", 404 → "Không tìm thấy")

## Phase N: Polish & Cross-Cutting

- [ ] T046 [P] Visual regression tổng quan/lớp/heatmap sáng/tối/compact/mobile (TC-UI-004) — file: `src/cms/frontend/apps/portal/e2e/dashboard/visual.spec.ts` — chờ Playwright visual với backend compose
- [ ] T047 [P] Đo thời gian tải tổng quan với dữ liệu 2 năm (TC-DASH-010) — file: `src/cms/frontend/apps/portal/e2e/dashboard/perf.spec.ts` — chờ dữ liệu 2 năm trên backend compose
- [x] T048 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md` — ✅ `CHANGELOG.md`
- [ ] T049 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng — chờ backend compose
- [x] T050 [P] Test F056-AC-23: tắt `action.school_lms.reports.export` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/dashboard-feature-toggle.spec.ts` — ✅ Vitest `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/permissions.test.tsx` › AC-23 (2 test: action/menu tắt; `403 FEATURE_DISABLED` → thông báo + tải lại `getMyPermissions`); Playwright cùng kịch bản chờ backend compose
- [x] T051 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F056-AC-23) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx` — ✅ `src/workspaces/school-lms/index.tsx` + `platform-admin/index.tsx` gắn màn vào khóa menu catalog; nút bọc `<ActionGate>` (`reports.export`, `assignments.edit`), tab theo `reports.view`/`reports.export`, tab đăng nhập theo `student_activity.view`; `check-portal-catalog` 0 lỗi

## Việc nhận sau khi chốt câu hỏi mở (owner chốt 2026-11-21)

- [x] T052 Truyền lịch năm học/học kỳ từ `listSchoolAcademicYears` (F010 T047) vào `DashboardRangePicker` để bật preset "Học kỳ này"/"Năm học này"; hiện `available_from` ("Chưa có số liệu trước ngày …") và cảnh báo trễ replica từ `DashboardMeta` (F043 T042) trong `DashboardWidget` — chờ F010 T047, F043 T042 — kiểm: `pnpm lint`, `pnpm typecheck`, vitest dashboard — cỡ S — ✅ 2026-11-21: `useSchoolAcademicYears` + `model/calendar.ts` (`schoolCalendar`) → `DashboardRangePicker calendar`; `DashboardMetaView` thêm `from`, `available_from`, `replica_lag_warning` và dòng meta "Chưa có số liệu trước ngày …" / "Số liệu có thể chậm do đồng bộ trễ" (ops-ui vi/en); "Học kỳ này" vẫn vô hiệu tới khi có học kỳ (OQ F010-4); test: `DashboardWidget.test` (ops-ui, FAIL → PASS), `calendar.test.ts`, `overview.test.tsx` T052 ×2 (FAIL → PASS), vitest dashboard 7 file/41 test PASS; eslint, typecheck PASS

## Giao diện owner chốt 2026-11-21 — hướng B "Sổ tay giáo viên"

- [x] T053 Áp hướng B qua token (`tokens@0.6.0`: chữ 15 px, bo 8/12, nền trang/viền thẻ, component token `Layout`/`Card`/`Menu`/`Statistic`) cho shell và tổng quan dashboard: lời chào theo buổi, nút chính "+ Giao bài mới" cỡ lớn (vẫn qua `ActionGate`), `DashboardWidget accent` viền trên 4 px (`dashboard.accent.width`) theo màu ngữ nghĩa; dữ liệu/hành vi không đổi — file: `design/admin-tokens/*`, `src/cms/frontend/packages/ops-ui/src/{theme/accent.ts,dashboard/DashboardWidget.tsx}`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/overview/OverviewPage.tsx`, `src/cms/frontend/apps/portal/src/shell/PortalLayout.tsx` — kiểm: ✅ 2026-11-21 Vitest `DashboardWidget.test` 8/8 (accent ngữ nghĩa/nghiệp vụ, sáng/tối), dashboard `__tests__` PASS, `layout.test` 5/5 (mật độ mặc định thường); ảnh `ad.truong.a-12-school-lms-school-dashboard.png` (thư mục ảnh ngoài repo)

## Câu hỏi mở (Clarifications 2026-11-21)

- **F056-1** — `getAssignmentItemAnalysis` (phân tích theo câu cho bài không có phiên thi) và danh sách học sinh sai từng câu chưa có trong contract; owner F044 quyết định có thêm hay không. Hiện: chỉ bài có `exam_session_id`. **Chốt 2026-11-21:** Release 1 giữ cách hiện tại (phân tích theo câu chỉ cho bài có phiên thi), không thêm `getAssignmentItemAnalysis`.
- **F056-2** — Ngày năm học/học kỳ của trường cho preset "Học kỳ này"/"Năm học này" (đọc `academic_years` qua API công khai). Hiện: hai preset bị vô hiệu kèm giải thích. **Chốt 2026-11-21:** thêm operation đọc năm học (F010 T047) rồi bật preset (T052).
- **F056-3** — `DashboardMeta` thiếu `available_from` ("Chưa có số liệu trước ngày …") và cờ cảnh báo trễ replica (Edge Cases); owner F043 bổ sung contract. **Chốt 2026-11-21:** F043 T042 thêm hai trường, F056 T052 hiển thị.

## ADR-0095 — dashboard trường → khối → học sinh, kết quả theo bài giao (WP-P1, 2026-11-21)

Nguồn: Clarifications "Session 2026-11-21 (ADR-0095 — [Confirmed])" của spec; CODE_PLAN §4.2 WP-P1.

- [x] T054 Gỡ trang lớp (`dashboard/class/ClassDashboardPage.tsx`, route `classes/:classId`, `useClassDashboard`), bộ lọc lớp, danh sách/liên kết lớp ở widget bài giao, LO yếu, "Cần chú ý"; khối 1–9 theo trường; breadcrumb trường → khối → học sinh; tên học sinh qua trạng thái điều hướng (F056-AC-01, AC-08, AC-10, AC-11; AC-22 Removed) — file: `dashboard/{SchoolDashboardScreen,DashboardFilterBar,DashboardUi}.tsx`, `dashboard/model/filters.ts`, `dashboard/overview/OverviewPage.tsx`, `dashboard/student/StudentProfilePage.tsx` — kiểm: `__tests__/grade-student.test.tsx`, `overview.test.tsx`, `permissions.test.tsx`
- [x] T055 "Kết quả bài/đề" theo bài giao (`listAssignments` lọc trường/môn/khối, đối tượng, đã nộp, điểm trung bình) → Phân tích (`getExamSessionReport`, xuất `assignment_results`); nút "Giao ôn tập…" điền trường/môn/khối của bài; bỏ `getCourseResults` (F056-AC-12, AC-13, AC-14, AC-15) — file: `dashboard/results/ResultsScreen.tsx`, `dashboard/analysis/ItemAnalysisPage.tsx`, `dashboard/api/resultsApi.ts` — kiểm: `__tests__/results-reports.test.tsx`
- [x] T056 Báo cáo: bỏ `course_results` và `skill_map` lớp khỏi màn xuất, `regulation_reference` không lọc lớp; mức tham khảo đánh số học sinh (F056-AC-14, AC-16) — file: `dashboard/reports/{ReportsScreen,ReferenceLevelsPage}.tsx`, MOCK `dashboard-mock.ts` (giữ trường lớp trong fixture để chứng minh không hiển thị)
- [x] T057 Sau WP-L2: trang khối `grades/:grade` (`getGradeDashboard`: heatmap LO, học sinh cursor, "< 5" khi `suppressed`), tổng quan `widgets.grades`/`by_grade`/`grade_levels`/`grade_level`, `skill_map` theo khối, mức tham khảo lọc khối và tên từ `listTenantGradeStudents` — file: `dashboard/grade/GradeDashboardPage.tsx`, `dashboard/overview/OverviewPage.tsx`, `dashboard/reports/*`, `test/dashboard-mock.ts` — kiểm: `grade-student.test.tsx`, `results-reports.test.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004–T006 trước T027/T031) → US1 → US2, US6 (song song) → US3, US4 → US5 → Polish.
- F056-AC-10 cần wizard F055 nhận query điền sẵn.

## Definition of Done

- [x] Mọi tiêu chí F056-AC-* có test pass (ghi mã test/TC) — Vitest (ops-ui + portal, MOCK) cho AC-01…AC-23; E2E Playwright T021, T042, T043 chờ backend compose
- [ ] `make web-lint`, `make web-test`, `make storybook`, Playwright E2E xanh với backend thật — lint, typecheck, Vitest, `check-modes` xanh; Playwright chờ backend compose
- [x] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17) — mock chỉ ở `src/test/` (nhãn MOCK)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — SCHOOL_LMS §8.7 (heatmap), CHANGELOG; CONFIG_DEFAULTS không đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — In progress (E2E chờ backend compose)
