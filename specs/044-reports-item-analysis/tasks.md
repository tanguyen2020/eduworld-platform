# Tasks: F044 — Báo cáo, xuất dữ liệu bất đồng bộ, item analysis, chất lượng đề

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `reporting`, `regulation`, `itemanalysis`, `examquality`; đăng ký work-queue `LEARNING_REPORT_JOBS`, consumer `exam.closed`, job `item-analysis-nightly`, `calibration-weekly`, `reference-levels-nightly` trong composition root — file: `src/backend/modules/learning/cmd/api/main.go`, `src/backend/modules/learning/cmd/worker/main.go` — ✅ package domain `reporting`/`regulation`/`itemanalysis`/`examquality`; job `report-jobs` (thay work-queue NATS, Clarifications 2026-11-21), `item-analysis-nightly`, `calibration-weekly`, `reference-levels-nightly`, consumer `learning.exam_quality` trong `modules/learning/module.go`; `TestWorkerProfilesRegisterRelaysConsumersAndJobs` pass
- [x] T002 [P] Adapter bucket `ew-exports` (presign GET theo `dashboard_export_link_ttl_minutes`, lifecycle `dashboard_export_file_retention_days`) — file: `src/backend/modules/learning/internal/infrastructure/storage/exports_bucket.go`, `deploy/compose/floci/buckets.yaml` — ✅ `storage.ExportBucket.ReportURL` (presign `dashboard_export_link_ttl_minutes`), bucket `ew-exports` có sẵn (F025); link hết hạn → 403 kiểm trong `TestReportExportJob`
- [x] T003 [P] Fixture lượt trả lời (≥ 100 lượt/câu, có `rapid_guess`, có gợi ý) và session official 40 học sinh — file: `src/backend/modules/learning/tests/fixtures/itemanalysis/`, `src/backend/modules/learning/tests/fixtures/examquality/` — ✅ fixture sinh trong test (`seedAnswers`: 42 lượt có rapid_guess + gợi ý; 32 học sinh đề official) — `tests/integration/quality_test.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: `ReportJob`, `LmsReportCreate` mở rộng; thêm `getRegulationReferenceReport`, `getQuestionStats`, `listCalibrationProposals`, `acceptCalibrationProposal`, `dismissCalibrationProposal`, `getExamSessionReport` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — ✅ 6 operation mới + delta `ReportJob`/`LmsReportRequest`/`CourseResults`/`ExamPaper`/`ExamSession.balance`; `split-public`, routematrix gen, portal `gen:api`; check-maxlength/enums/portal-catalog 0 lỗi
- [x] T005 [P] JSON Schema `assessment-profile.schema.json`, payload `report.completed.v1` — file: `specs/schemas/assessment-profile.schema.json`, `specs/events/payloads/learning/report.completed.v1.schema.json` — kiểm: `make specs-lint` — ✅ `specs/schemas/assessment-profile.schema.json`, `specs/events/payloads/learning/report.completed.v1.schema.json`; validate-schemas 0 lỗi
- [x] T006 ent schema bảng mới `learning.report_jobs`, `learning.assessment_profiles`, `learning.rm_student_subject_reference_levels`, `content.question_stats`, `content.difficulty_calibration_proposals`, `assessment.exam_quality_reports`; cột `assessment.exam_papers.expected_p`, `balance_warning` (chuỗi `MaxLen`) — file: `src/backend/modules/learning/ent/schema/report_job.go`, `src/backend/modules/learning/ent/schema/assessment_profile.go`, `src/backend/modules/learning/ent/schema/reference_level.go`, `src/backend/modules/learning/ent/schema/question_stat.go`, `src/backend/modules/learning/ent/schema/difficulty_calibration_proposal.go`, `src/backend/modules/learning/ent/schema/exam_quality_report.go`, `src/backend/modules/learning/ent/schema/exam_paper.go` — kiểm: `make migrate-diff MOD=learning name=f044_reports_quality`, `make migrate-lint MOD=learning` — ✅ bảng viết tay (Clarifications 2026-11-21) trong `migrations/20261226000001_f044_reports_quality.sql`; cột `exam_papers.balance_warning` (`expected_p` có từ F042); `TestMigrationMatchesEnt` pass
- [x] T007 Migration RLS, partial unique, GRANT SELECT cho `learning_dashboard` trên bảng đọc của export; seed hồ sơ `primary_tt27`, `lower_secondary_tt22` version 1 theo `assessment_reference_thresholds` — file: `src/backend/modules/learning/migrations/<ts>_f044_reports_quality.sql` — ✅ cùng migration: RLS T/H, partial unique, view `_live`, REVOKE DELETE/TRUNCATE, GRANT SELECT `learning_dashboard` (reference levels, profiles), seed hồ sơ v1; atlas.sum
- [x] T008 [P] Cập nhật DATABASE_DESIGN §2.5–§2.7 cho bảng/cột mới — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ DATABASE_DESIGN §2.5–§2.7 (+ EVENT_DESIGN `report.completed`)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Xuất báo cáo bất đồng bộ (Priority: P1) 🎯 MVP

**Goal**: Job export có giới hạn, watermark, link có hạn, audit.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Integration test F044-AC-01, F044-AC-02: 202 + cùng `report_id` khi replay key; file trên `ew-exports`; link chỉ người tạo; quá hạn 403 (TC-DASH-008) — file: `src/backend/modules/learning/tests/integration/reporting/export_job_test.go` — phải FAIL trước khi code — ✅ `TestReportExportJob` pass
- [x] T010 [P] [US1] Integration test F044-AC-03: watermark, chỉ dữ liệu trong scope trường/môn, không có cột chat, audit `report.exported` — file: `src/backend/modules/learning/tests/integration/reporting/export_scope_watermark_test.go` — ✅ `TestReportExportJob` (watermark đầu/cuối, chỉ dòng của bài, không cột chat, audit) + `TestReportExportScopeAndLimits` pass
- [x] T011 [P] [US1] API test F044-AC-04, F044-AC-07: `429 EXPORT_LIMIT_REACHED`, vượt `dashboard_export_max_rows` → 422, enum/maxLength/khoảng ngày → 422 (TC-TEN-024, TC-DASH-002) — file: `src/backend/modules/learning/tests/integration/reporting/export_limits_validation_test.go` — ✅ `TestReportExportScopeAndLimits` pass (429 + Retry-After, 422 max rows, 422 enum, 422 DASHBOARD_RANGE_TOO_LARGE)
- [x] T012 [P] [US1] Test tenant isolation + authorization F044-AC-05, F044-AC-06 cho `createLmsReport`, `getLmsReport`, `createTenantReport`, `getTenantReport`, `createPlatformReport`, `getPlatformReport`: không phải người tạo/khác tenant 404, thiếu `report.export` 403, role tenant gọi `/admin/reports` 403, System Admin có audit `cross_tenant.accessed` và ẩn ô < 5 (TC-TEN-001, TC-TEN-004, TC-TEN-018) — file: `src/backend/modules/learning/tests/integration/reporting/export_authz_tenant_test.go` — ✅ `TestReportExportScopeAndLimits` + `TestPlatformReport` pass

### Implementation cho User Story 1

- [x] T013 [P] [US1] Domain `reporting.ReportJob` (trạng thái, giới hạn), `reporting.Watermark` — file: `src/backend/modules/learning/internal/domain/reporting/report_job.go`, `src/backend/modules/learning/internal/domain/reporting/watermark.go` — ✅ `internal/domain/reporting/reporting.go` + `reporting_test.go`
- [x] T014 [US1] Command `CreateReport` (đếm job đang chạy, `Idempotency-Key`, outbox đưa vào work-queue) — file: `src/backend/modules/learning/internal/application/command/reporting/create_report.go` — ✅ `command.Reports.CreateReport` (`internal/application/command/reports.go`)
- [x] T015 [US1] Command `RunReportJob`: reader replica theo lô 10.000 dòng có scope, writer CSV/XLSX/PDF, upload, audit `report.exported`, `report.completed` — file: `src/backend/modules/learning/internal/application/command/reporting/run_report_job.go`, `src/backend/modules/learning/internal/infrastructure/export/xlsx_writer.go`, `src/backend/modules/learning/internal/infrastructure/export/csv_writer.go`, `src/backend/modules/learning/internal/infrastructure/export/pdf_writer.go` — ✅ `command.Reports.RunReportJobs` + `internal/infrastructure/export/writers.go` (CSV/XLSX/PDF, `writers_test.go`)
- [x] T016 [US1] Query `GetReport` (presign khi `succeeded`) + handler 6 operationId + work-queue consumer — file: `src/backend/modules/learning/internal/application/query/reporting/get_report.go`, `src/backend/modules/learning/internal/presentation/http/public/report_handler.go`, `src/backend/modules/learning/internal/presentation/http/public/admin/report_handler.go`, `src/backend/modules/learning/internal/presentation/consumer/report_job_worker.go` — ✅ `query.Reports.GetReport`, `presentation/http/reports.go` (13 operation), job `presentation/job/reports.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Bảng kết quả course theo điểm tự chấm (Priority: P1)

**Goal**: `getCourseResults` đúng `score_policy`, không XP/rating.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T017 [P] [US2] Integration test F044-AC-08, F044-AC-09: điểm theo `best/last/first`, thang `score_scale`, điểm hiệu lực sau re-evaluate, không cột XP/rating — file: `src/backend/modules/learning/tests/integration/reporting/course_results_test.go` — phải FAIL trước khi code — ✅ `TestCourseResults` pass
- [x] T018 [P] [US2] Test authorization F044-AC-10: môn ngoài scope `PERMISSION_DENIED`, trường/tenant khác 404 (TC-TEN-002, TC-TEN-003) — file: `src/backend/modules/learning/tests/integration/reporting/course_results_authz_test.go` — ✅ `TestCourseResults` pass

### Implementation cho User Story 2

- [x] T019 [US2] Query `GetCourseResults` (SQL đọc `lms.assignment_submissions` + `assessment.attempt_score_revisions`, scope) + handler — file: `src/backend/modules/learning/internal/application/query/reporting/get_course_results.go`, `src/backend/modules/learning/internal/presentation/http/public/report_handler.go` — ✅ `query.Reports.GetCourseResults` + `read.QualityModel` (điểm hiệu lực = `assignment_submissions.score` do F041/re-evaluate cập nhật)

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Báo cáo tham khảo theo TT27/2020 và TT22/2021 (Priority: P2)

**Goal**: Mức tham khảo có version hồ sơ và disclaimer.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T020 [P] [US3] Unit test F044-AC-11: ánh xạ AC-ARP-001 (Hoàn thành tốt), AC-ARP-002 (Chưa đủ dữ liệu), TT22 Đạt/Chưa đạt, điểm tham khảo đề official (TC-LRN-018) — file: `src/backend/modules/learning/tests/unit/regulation/mapper_test.go` — phải FAIL trước khi code — ✅ `internal/domain/regulation/regulation_test.go` pass
- [x] T021 [P] [US3] Integration test F044-AC-12, F044-AC-14: disclaimer ở response và đầu/chân file, audit `learning.reference_report_exported`; đổi ngưỡng → version mới, kỳ trước giữ nguyên — file: `src/backend/modules/learning/tests/integration/regulation/reference_report_test.go` — ✅ `TestReferenceLevels` pass
- [x] T022 [P] [US3] Contract test F044-AC-13: quét response mọi operation học sinh không có `level_code`/nhãn thông tư — file: `src/backend/modules/learning/tests/contract/regulation/no_student_labels_test.go` — ✅ `TestNoCircularLevelForStudents` pass (quét contract các route học sinh)
- [x] T023 [P] [US3] Test authorization F044-AC-15: `academic_manager` môn ngoài scope 403; báo cáo có `rapid_guess_ratio`, đánh dấu đề độ tin cậy thấp (TC-TEN-002) — file: `src/backend/modules/learning/tests/integration/regulation/reference_report_authz_test.go` — ✅ `TestReferenceLevels` pass

### Implementation cho User Story 3

- [x] T024 [P] [US3] Domain `regulation.Profile`, `regulation.Mapper` — file: `src/backend/modules/learning/internal/domain/regulation/mapper.go` — ✅ `internal/domain/regulation/regulation.go`
- [x] T025 [US3] Command `ComputeReferenceLevels` + job `reference-levels-nightly`; command `PublishAssessmentProfileVersion` — file: `src/backend/modules/learning/internal/application/command/regulation/compute_reference_levels.go`, `src/backend/modules/learning/internal/presentation/job/reference_levels_nightly.go` — ✅ `command.Quality.ComputeReferenceLevels`/`ReferenceLevelsFor`/`PublishProfileVersion` + job `reference-levels-nightly`
- [x] T026 [US3] Query `GetReferenceReport` + handler `getRegulationReferenceReport`; đưa `regulation_reference` vào writer export — file: `src/backend/modules/learning/internal/application/query/reporting/get_reference_report.go`, `src/backend/modules/learning/internal/presentation/http/public/report_handler.go` — ✅ `query.Reports.GetReferenceReport` + handler; export `regulation_reference` có disclaimer đầu/cuối

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Thống kê chất lượng câu hỏi (item analysis) (Priority: P2)

**Goal**: `content.question_stats` hằng đêm, API CMS.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T027 [P] [US4] Unit test F044-AC-16: `p_value`, point-biserial, `distractor_rate`, loại `rapid_guess`, chỉ lần đầu `is_scored`, tách gợi ý, ngưỡng lượt — file: `src/backend/modules/learning/tests/unit/itemanalysis/stats_test.go` — phải FAIL trước khi code — ✅ `internal/domain/itemanalysis/itemanalysis_test.go` pass
- [x] T028 [P] [US4] Integration test F044-AC-17: thống kê theo version + cờ; checksum version publish không đổi sau job — file: `src/backend/modules/learning/tests/integration/itemanalysis/question_stats_test.go` — ✅ `TestItemAnalysisAndCalibration` pass
- [x] T029 [P] [US4] Test tenant isolation F044-AC-18: câu tenant khác 404, câu platform được gán đọc được (TC-TEN-001) — file: `src/backend/modules/learning/tests/integration/itemanalysis/question_stats_tenant_test.go` — ✅ `TestItemAnalysisAndCalibration` pass

### Implementation cho User Story 4

- [x] T030 [P] [US4] Domain `itemanalysis.Stats` — file: `src/backend/modules/learning/internal/domain/itemanalysis/stats.go` — ✅ `internal/domain/itemanalysis/stats.go`
- [x] T031 [US4] Command `ComputeQuestionStats` (đọc `attempt_answers` lần đầu theo partition, ghi stats) + job `item-analysis-nightly` — file: `src/backend/modules/learning/internal/application/command/itemanalysis/compute_question_stats.go`, `src/backend/modules/learning/internal/presentation/job/item_analysis_nightly.go` — ✅ `command.Quality.ComputeQuestionStats` + job `item-analysis-nightly`
- [x] T032 [US4] Query `GetQuestionStats` + handler — file: `src/backend/modules/learning/internal/application/query/itemanalysis/get_question_stats.go`, `src/backend/modules/learning/internal/presentation/http/public/cms_quality_handler.go` — ✅ `query.Reports.GetQuestionStats` + handler

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Đề xuất hiệu chỉnh độ khó có người duyệt (Priority: P3)

**Goal**: Hàng đợi đề xuất, chấp nhận tạo version mới.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T033 [P] [US5] Unit test F044-AC-19: bảng ánh xạ `difficulty_calibration`, lệch ≥ 1 mức, chặn đề xuất lại 90 ngày sau 2 lần bỏ qua — file: `src/backend/modules/learning/tests/unit/itemanalysis/calibration_test.go` — phải FAIL trước khi code — ✅ `TestPropose`, `TestCalibrationTable` pass
- [x] T034 [P] [US5] Integration test F044-AC-20: accept tạo version mới `change_note = difficulty_calibration` qua workflow, không sửa version publish, replay key không tạo version thứ hai, audit — file: `src/backend/modules/learning/tests/integration/itemanalysis/accept_calibration_test.go` — ✅ `TestItemAnalysisAndCalibration` pass
- [x] T035 [P] [US5] Test authorization F044-AC-21: thiếu `learning_content.review`, tác giả, role tenant với câu platform → 403 (TC-CMS-ASG-02) — file: `src/backend/modules/learning/tests/integration/itemanalysis/calibration_authz_test.go` — ✅ `TestItemAnalysisAndCalibration` pass (thiếu quyền, câu platform với role tenant)

### Implementation cho User Story 5

- [x] T036 [P] [US5] Domain `itemanalysis.Calibration` — file: `src/backend/modules/learning/internal/domain/itemanalysis/calibration.go` — ✅ `internal/domain/itemanalysis/calibration.go`
- [x] T037 [US5] Command `ProposeCalibrations` + job `calibration-weekly`; `AcceptCalibration` (gọi port `QuestionVersionCreator` của workflow F025), `DismissCalibration` — file: `src/backend/modules/learning/internal/application/command/itemanalysis/propose_calibrations.go`, `src/backend/modules/learning/internal/application/command/itemanalysis/accept_calibration.go`, `src/backend/modules/learning/internal/presentation/job/calibration_weekly.go` — ✅ `command.Quality.ProposeCalibrations`/`AcceptCalibration`/`DismissCalibration` (version nháp mới qua store câu hỏi của F022/F025)
- [x] T038 [US5] Query `ListCalibrationProposals` + handler list/accept/dismiss — file: `src/backend/modules/learning/internal/application/query/itemanalysis/list_calibration_proposals.go`, `src/backend/modules/learning/internal/presentation/http/public/cms_quality_handler.go` — ✅ `query.Reports.ListCalibrationProposals` + handler list/accept/dismiss

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: User Story 6 — Chất lượng đề: cân bằng biến thể, độ tin cậy, phân tích đề (Priority: P2)

**Goal**: `VariantBalancer`, độ tin cậy, `getExamSessionReport`.

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [x] T039 [P] [US6] Unit test F044-AC-22: `P_v` từ `p_value`/`p̂`, ≤ 200 lần lặp seed cố định, đạt ngưỡng `variant_balance_max_mean_p_diff` hoặc cảnh báo, tái lập được (TC-EXM-001) — file: `src/backend/modules/learning/tests/unit/examquality/balancer_test.go` — phải FAIL trước khi code — ✅ `internal/domain/examquality/examquality_test.go` (`TestBalancer*`) pass
- [x] T040 [P] [US6] Unit test F044-AC-23: KR-20, alpha, SEM khớp tính tay; < số học sinh tối thiểu → không tính; cờ chênh biến thể — file: `src/backend/modules/learning/tests/unit/examquality/reliability_test.go` — ✅ `TestKR20AndSEM`, `TestAlphaGapAndProblemItems` pass
- [x] T041 [P] [US6] Integration test F044-AC-24: report có phổ điểm, p-value loại `rapid_guess`, `misconception_tag`, cảnh báo độ tin cậy (TC-EXM-004) — file: `src/backend/modules/learning/tests/integration/examquality/exam_session_report_test.go` — ✅ `TestExamQualityReport` pass
- [x] T042 [P] [US6] Test tenant isolation + authorization F044-AC-25 cho `getExamSessionReport`: trường/tenant khác 404, học sinh 403 (TC-TEN-001) — file: `src/backend/modules/learning/tests/integration/examquality/exam_report_authz_test.go` — ✅ `TestExamQualityReport` pass

### Implementation cho User Story 6

- [x] T043 [P] [US6] Domain `examquality.Balancer` hiện thực interface `VariantBalancer` của F042; `examquality.Reliability` — file: `src/backend/modules/learning/internal/domain/examquality/balancer.go`, `src/backend/modules/learning/internal/domain/examquality/reliability.go` — ✅ `internal/domain/examquality/balancer.go`, `reliability.go`
- [x] T044 [US6] Đăng ký balancer vào paper generator của F042 ở composition root — file: `src/backend/modules/learning/cmd/api/main.go` — ✅ `exams.Balancer = examquality.Balancer{}` + `PValues` trong `module.go`
- [x] T045 [US6] Command `ComputeExamQuality` + consumer `exam.closed` — file: `src/backend/modules/learning/internal/application/command/examquality/compute_exam_quality.go`, `src/backend/modules/learning/internal/presentation/consumer/exam_closed_consumer.go` — ✅ `command.Quality.ComputeExamQuality` + `presentation/consumer/exam_quality.go`
- [x] T046 [US6] Query `GetExamSessionReport` + handler — file: `src/backend/modules/learning/internal/application/query/examquality/get_exam_session_report.go`, `src/backend/modules/learning/internal/presentation/http/public/exam_report_handler.go` — ✅ `query.Reports.GetExamSessionReport` + handler

**Checkpoint**: User Story 6 chạy độc lập.

## Phase 9: Polish & Cross-Cutting

- [x] T047 [P] Metric + span theo plan.md §Quan sát; kiểm log không chứa điểm/nội dung file — file: `src/backend/modules/learning/internal/infrastructure/observability/reporting_metrics.go` — ✅ `internal/infrastructure/observability/reporting_metrics.go`; log chỉ mã lỗi/id job
- [x] T048 [P] Đề nghị đăng ký ngưỡng item analysis vào CONFIG_DEFAULTS (PR riêng với QUESTION_BANK §9); cập nhật CHANGELOG và trạng thái F044 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — ✅ CHANGELOG, CODE_PLAN; `item_analysis_min_responses` đã có trong CONFIG_DEFAULTS
- [x] T049 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/044-reports-item-analysis/quickstart.md` — ✅ kịch bản quickstart §2 tự động hóa bằng 7 integration test F044 (bước 1–14) + unit domain; chạy với tài khoản demo trên môi trường dùng chung chờ staging

## Dependencies & Execution Order

- Setup → Foundational (T004–T008) → US1, US2 (song song) → US3 → US4 → US5 (cần US4) ; US6 cần US4 cho `p_value` (dùng `p̂` khi chưa đủ dữ liệu) và F042 interface `VariantBalancer`.
- T016, T019, T026 cùng sửa `report_handler.go`; T032, T038 cùng sửa `cms_quality_handler.go`: làm tuần tự.

## Definition of Done

- [x] Mọi tiêu chí F044-AC-* có test pass (ghi mã test/TC) — 25/25 AC có test pass local; SC-001 (export 500.000 dòng, 20 export đồng thời) chờ kiểm thử tải trên staging
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — ✅ 2026-11-21: gofmt, vet -tags integration, golangci-lint (+arch) 0 issue, tenancyguard/ewlint/entlint/archgen/routematrix/allowedcallers/facadegen OK; unit learning + cmd pass; toàn bộ integration learning pass (646 s)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
