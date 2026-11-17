# Tasks: F056 — Workspace C — School Learning Dashboard và báo cáo

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo module `dashboard` trong workspace C, route lazy `/portal/school-lms/dashboard/*` có guard permission — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/routes.tsx`
- [ ] T002 [P] Thêm ECharts (ghim phiên bản) vào `ops-ui`, khung thư mục `src/dashboard` — file: `src/cms/frontend/packages/ops-ui/package.json`
- [ ] T003 [P] File dịch vi/en (`dashboard.*`, `error.DASHBOARD_*`, `error.EXPORT_LIMIT_REACHED`) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/i18n/vi.json`, `en.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract (owner F043): `getStudentLearningProfile` + schema `StudentLearningProfile`; sửa `SchoolDashboard.widgets.self_study` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T005 Delta contract (owner F044): `getExamSessionReport`, `getAssignmentItemAnalysis` + schema `ItemAnalysis`; cập nhật EXAM_ENGINE §11, DASHBOARD_DATA §14 — file: `specs/openapi/eduworld-api.v1.yaml`, `docs/02-learning/EXAM_ENGINE.md`, `docs/07-backend/DASHBOARD_DATA.md` — kiểm: `make specs-lint`
- [ ] T006 Sinh lại client TS — file: `src/cms/frontend/apps/portal/src/api/generated/eduworld-api.d.ts` — kiểm: `pnpm gen:api` + CI diff
- [ ] T007 [P] Test `useDashboardQueue`: không vượt `dashboard_max_concurrent_per_user` song song, retry `DASHBOARD_BUSY` 2–5 s tối đa 2 lần — file: `src/cms/frontend/packages/ops-ui/src/dashboard/useDashboardQueue.test.ts` — phải FAIL trước khi code
- [ ] T008 [P] `useDashboardQueue` + `DashboardWidget` (skeleton, "Cập nhật lúc", "Tạm tính", "Đang tính…", lỗi theo `code`) + story — file: `src/cms/frontend/packages/ops-ui/src/dashboard/useDashboardQueue.ts`, `DashboardWidget.tsx`
- [ ] T009 [P] `DashboardRangePicker` (preset, giới hạn `dashboard_max_range_days`, độ hạt thô hơn) + story — file: `src/cms/frontend/packages/ops-ui/src/dashboard/DashboardRangePicker.tsx`
- [ ] T010 [P] MSW handler test-only (nhãn `MOCK`) từ OpenAPI + golden fixtures — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/__tests__/msw/handlers.ts`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Trang tổng quan trường (Priority: P1) 🎯 MVP

**Goal**: 5 widget trả lời 5 câu hỏi tuần.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T011 [P] [US1] Test F056-AC-01: gọi `getSchoolDashboard` đúng tham số, thứ tự 5 widget — file: `…/dashboard/__tests__/OverviewPage.test.tsx` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Test F056-AC-02: nhãn `data_as_of`, `grain`, `is_partial`, "Đang tính số liệu…" — file: `src/cms/frontend/packages/ops-ui/src/dashboard/DashboardWidget.test.tsx`
- [ ] T013 [P] [US1] Test F056-AC-03: preset, ngày vượt giới hạn bị vô hiệu, không "toàn bộ thời gian", độ hạt thô hơn (TC-DASH-002) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/DashboardRangePicker.test.tsx`
- [ ] T014 [P] [US1] Test F056-AC-04: `DASHBOARD_RANGE_TOO_LARGE`/`DASHBOARD_QUERY_TIMEOUT` gợi ý thu hẹp; widget độc lập (TC-DASH-006) — file: `…/dashboard/__tests__/OverviewPage.errors.test.tsx`
- [ ] T015 [P] [US1] Test F056-AC-05: nhãn `reason`, "làm quá nhanh", không xếp hạng — file: `…/dashboard/__tests__/AttentionWidget.test.tsx`
- [ ] T016 [P] [US1] Test F056-AC-06: `suppressed` → "< 5" — file: `src/cms/frontend/packages/ops-ui/src/dashboard/SuppressedCell.test.tsx`
- [ ] T017 [P] [US1] Test F056-AC-07: widget an toàn chỉ số lượng + link; quét DOM không có trường nội dung chat — file: `…/dashboard/__tests__/SafetyWidget.test.tsx`

### Implementation cho User Story 1

- [ ] T018 [P] [US1] Hook `useSchoolDashboard`, `useSchoolSeries` — file: `…/dashboard/api/schoolDashboard.ts`
- [ ] T019 [P] [US1] `SuppressedCell` (chữ "< 5" + tooltip, không hiện số thật) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/SuppressedCell.tsx`
- [ ] T020 [US1] Trang tổng quan + 5 widget, bố cục mobile thẻ dọc — file: `…/dashboard/overview/OverviewPage.tsx`, `…/overview/widgets/*.tsx`
- [ ] T021 [US1] E2E tổng quan với golden dataset (TC-LMS-003) — file: `src/cms/frontend/apps/portal/e2e/dashboard/overview.spec.ts`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Drill-down lớp, bản đồ kỹ năng, hồ sơ học sinh (Priority: P1)

### Tests cho User Story 2

- [ ] T022 [P] [US2] Test F056-AC-08: drill-down giữ bộ lọc, breadcrumb — file: `…/dashboard/__tests__/ClassDashboardPage.test.tsx` — phải FAIL trước khi code
- [ ] T023 [P] [US2] Test F056-AC-09: `MasteryHeatmap` có ký hiệu, aria-label, tooltip; axe (TC-UI-006) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/MasteryHeatmap.test.tsx`
- [ ] T024 [P] [US2] Test F056-AC-10: nút "Giao ôn tập theo LO" điều hướng wizard F055 với `mode=practice`, LO, lớp, học sinh — file: `…/dashboard/__tests__/AssignReviewShortcut.test.tsx`
- [ ] T025 [P] [US2] Test F056-AC-11: hồ sơ học sinh có các tab, không trường IP/bạn bè/chat/Game Rating — file: `…/dashboard/__tests__/StudentProfilePage.test.tsx`

### Implementation cho User Story 2

- [ ] T026 [P] [US2] `MasteryHeatmap` (ECharts, màu token) + story — file: `src/cms/frontend/packages/ops-ui/src/dashboard/MasteryHeatmap.tsx`
- [ ] T027 [P] [US2] Hook `useClassDashboard`, `useStudentProfile` (sau T004), `useLoginEvents` — file: `…/dashboard/api/classDashboard.ts`, `studentProfile.ts`
- [ ] T028 [US2] Trang lớp + hồ sơ học sinh — file: `…/dashboard/class/ClassDashboardPage.tsx`, `…/dashboard/student/StudentProfilePage.tsx`

## Phase 5: User Story 3 — Bảng kết quả và phân tích đề/bài (Priority: P2)

### Tests cho User Story 3

- [ ] T029 [P] [US3] Test F056-AC-12: cột bài graded, `score_10`, phân trang `dashboard_page_size_max`, không cột XP/rating/nhập tay — file: `…/dashboard/__tests__/CourseResultsTab.test.tsx` — phải FAIL trước khi code
- [ ] T030 [P] [US3] Test F056-AC-13: phân tích theo câu, `rapid_guess` loại khỏi p-value, cảnh báo độ tin cậy, nút giao ôn tập (TC-EXM-004) — file: `…/dashboard/__tests__/ItemAnalysisTab.test.tsx`

### Implementation cho User Story 3

- [ ] T031 [P] [US3] Hook `useCourseResults`, `useItemAnalysis` (sau T005) — file: `…/dashboard/api/courseResults.ts`, `itemAnalysis.ts`
- [ ] T032 [US3] Tab Kết quả và tab Phân tích — file: `…/dashboard/results/CourseResultsTab.tsx`, `…/dashboard/analysis/ItemAnalysisTab.tsx`

## Phase 6: User Story 4 — Xuất báo cáo bất đồng bộ (Priority: P2)

### Tests cho User Story 4

- [ ] T033 [P] [US4] Test F056-AC-14: `createLmsReport` có `Idempotency-Key`, theo dõi `getLmsReport`, link + hạn, không tải trực tiếp (TC-DASH-008) — file: `src/cms/frontend/packages/ops-ui/src/dashboard/ExportButton.test.tsx` — phải FAIL trước khi code
- [ ] T034 [P] [US4] Test F056-AC-15: `EXPORT_LIMIT_REACHED` và link hết hạn — file: `…/dashboard/__tests__/ReportsCenter.test.tsx`
- [ ] T035 [P] [US4] Test F056-AC-16: nhãn "Tham khảo — không thay đánh giá chính thức", "Chưa đủ dữ liệu" — file: `…/dashboard/__tests__/RegulationDisclaimer.test.tsx`

### Implementation cho User Story 4

- [ ] T036 [P] [US4] `ExportButton` + hook `useReportJob` — file: `src/cms/frontend/packages/ops-ui/src/dashboard/ExportButton.tsx`, `…/dashboard/api/reports.ts`
- [ ] T037 [US4] Trung tâm báo cáo + nhãn tham khảo — file: `…/dashboard/reports/ReportsCenterPage.tsx`, `RegulationDisclaimer.tsx`

## Phase 7: User Story 5 — Tổng hợp tenant và platform (Priority: P3)

### Tests cho User Story 5

- [ ] T038 [P] [US5] Test F056-AC-17: `getTenantLearningSummary`, "< 5", export tenant (TC-ANL-005) — file: `…/dashboard/__tests__/TenantSummaryPage.test.tsx` — phải FAIL trước khi code
- [ ] T039 [P] [US5] Test F056-AC-18: System Admin gửi `tenant_ids[]` không gửi `X-Tenant-Id`, cột Tenant (TC-CMS-TEN-02) — file: `…/dashboard/__tests__/PlatformSummaryPage.test.tsx`

### Implementation cho User Story 5

- [ ] T040 [US5] Trang tổng hợp tenant và platform — file: `…/dashboard/api/summaries.ts`, `…/dashboard/summary/TenantSummaryPage.tsx`, `PlatformSummaryPage.tsx`

## Phase 8: User Story 6 — Phân quyền và cô lập tenant (Priority: P1)

### Tests cho User Story 6

- [ ] T041 [P] [US6] Test F056-AC-19: menu theo `analytics.*`; bộ lọc môn theo scope; `403` → "Ngoài phạm vi quyền" (TC-TEN-002) — file: `…/dashboard/__tests__/permissions.matrix.test.tsx` — phải FAIL trước khi code
- [ ] T042 [P] [US6] E2E F056-AC-20: `sm.antoan`, `cm.noidung` mở URL dashboard → 403, không request dashboard (TC-CMS-WS-01) — file: `src/cms/frontend/apps/portal/e2e/dashboard/authz.spec.ts`
- [ ] T043 [P] [US6] E2E F056-AC-21: `ad.truong.b` mở lớp TH Demo A → "Không tìm thấy"; cache khóa theo tenant (TC-DASH-009, TC-TEN-001) — file: `src/cms/frontend/apps/portal/e2e/dashboard/tenant-isolation.spec.ts`
- [ ] T044 [P] [US6] Test F056-AC-22: ô tìm kiếm/bộ lọc theo `maxLength` (TC-TEN-024) — file: `…/dashboard/__tests__/filters.test.tsx`

### Implementation cho User Story 6

- [ ] T045 [US6] Guard route/menu và xử lý `403`/`404`/`TENANT_MISMATCH` chung cho module — file: `…/dashboard/routes.tsx`

## Phase N: Polish & Cross-Cutting

- [ ] T046 [P] Visual regression tổng quan/lớp/heatmap sáng/tối/compact/mobile (TC-UI-004) — file: `src/cms/frontend/apps/portal/e2e/dashboard/visual.spec.ts`
- [ ] T047 [P] Đo thời gian tải tổng quan với dữ liệu 2 năm (TC-DASH-010) — file: `src/cms/frontend/apps/portal/e2e/dashboard/perf.spec.ts`
- [ ] T048 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md`
- [ ] T049 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng
- [ ] T050 [P] Test F056-AC-23: tắt `action.school_lms.reports.export` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/dashboard-feature-toggle.spec.ts`
- [ ] T051 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F056-AC-23) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004–T006 trước T027/T031) → US1 → US2, US6 (song song) → US3, US4 → US5 → Polish.
- F056-AC-10 cần wizard F055 nhận query điền sẵn.

## Definition of Done

- [ ] Mọi tiêu chí F056-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, `make storybook`, Playwright E2E xanh với backend thật
- [ ] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
