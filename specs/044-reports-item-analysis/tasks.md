# Tasks: F044 — Báo cáo, xuất dữ liệu bất đồng bộ, item analysis, chất lượng đề

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `reporting`, `regulation`, `itemanalysis`, `examquality`; đăng ký work-queue `LEARNING_REPORT_JOBS`, consumer `exam.closed`, job `item-analysis-nightly`, `calibration-weekly`, `reference-levels-nightly` trong composition root — file: `modules/learning/cmd/api/main.go`, `modules/learning/cmd/worker/main.go`
- [ ] T002 [P] Adapter bucket `ew-exports` (presign GET theo `dashboard_export_link_ttl_minutes`, lifecycle `dashboard_export_file_retention_days`) — file: `modules/learning/internal/infrastructure/storage/exports_bucket.go`, `deploy/compose/floci/buckets.yaml`
- [ ] T003 [P] Fixture lượt trả lời (≥ 100 lượt/câu, có `rapid_guess`, có gợi ý) và session official 40 học sinh — file: `modules/learning/tests/fixtures/itemanalysis/`, `modules/learning/tests/fixtures/examquality/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: `ReportJob`, `LmsReportCreate` mở rộng; thêm `getRegulationReferenceReport`, `getQuestionStats`, `listCalibrationProposals`, `acceptCalibrationProposal`, `dismissCalibrationProposal`, `getExamSessionReport` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T005 [P] JSON Schema `assessment-profile.schema.json`, payload `report.completed.v1` — file: `specs/schemas/assessment-profile.schema.json`, `specs/events/payloads/learning/report.completed.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T006 ent schema bảng mới `learning.report_jobs`, `learning.assessment_profiles`, `learning.rm_student_subject_reference_levels`, `content.question_stats`, `content.difficulty_calibration_proposals`, `assessment.exam_quality_reports`; cột `assessment.exam_papers.expected_p`, `balance_warning` (chuỗi `MaxLen`) — file: `modules/learning/ent/schema/report_job.go`, `modules/learning/ent/schema/assessment_profile.go`, `modules/learning/ent/schema/reference_level.go`, `modules/learning/ent/schema/question_stat.go`, `modules/learning/ent/schema/difficulty_calibration_proposal.go`, `modules/learning/ent/schema/exam_quality_report.go`, `modules/learning/ent/schema/exam_paper.go` — kiểm: `make migrate-diff MOD=learning name=f044_reports_quality`, `make migrate-lint MOD=learning`
- [ ] T007 Migration RLS, partial unique, GRANT SELECT cho `learning_dashboard` trên bảng đọc của export; seed hồ sơ `primary_tt27`, `lower_secondary_tt22` version 1 theo `assessment_reference_thresholds` — file: `modules/learning/migrations/<ts>_f044_reports_quality.sql`
- [ ] T008 [P] Cập nhật DATABASE_DESIGN §2.5–§2.7 cho bảng/cột mới — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Xuất báo cáo bất đồng bộ (Priority: P1) 🎯 MVP

**Goal**: Job export có giới hạn, watermark, link có hạn, audit.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Integration test F044-AC-01, F044-AC-02: 202 + cùng `report_id` khi replay key; file trên `ew-exports`; link chỉ người tạo; quá hạn 403 (TC-DASH-008) — file: `modules/learning/tests/integration/reporting/export_job_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Integration test F044-AC-03: watermark, chỉ dữ liệu trong scope trường/môn, không có cột chat, audit `report.exported` — file: `modules/learning/tests/integration/reporting/export_scope_watermark_test.go`
- [ ] T011 [P] [US1] API test F044-AC-04, F044-AC-07: `429 EXPORT_LIMIT_REACHED`, vượt `dashboard_export_max_rows` → 422, enum/maxLength/khoảng ngày → 422 (TC-TEN-024, TC-DASH-002) — file: `modules/learning/tests/integration/reporting/export_limits_validation_test.go`
- [ ] T012 [P] [US1] Test tenant isolation + authorization F044-AC-05, F044-AC-06 cho `createLmsReport`, `getLmsReport`, `createTenantReport`, `getTenantReport`, `createPlatformReport`, `getPlatformReport`: không phải người tạo/khác tenant 404, thiếu `report.export` 403, role tenant gọi `/admin/reports` 403, System Admin có audit `cross_tenant.accessed` và ẩn ô < 5 (TC-TEN-001, TC-TEN-004, TC-TEN-018) — file: `modules/learning/tests/integration/reporting/export_authz_tenant_test.go`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Domain `reporting.ReportJob` (trạng thái, giới hạn), `reporting.Watermark` — file: `modules/learning/internal/domain/reporting/report_job.go`, `modules/learning/internal/domain/reporting/watermark.go`
- [ ] T014 [US1] Command `CreateReport` (đếm job đang chạy, `Idempotency-Key`, outbox đưa vào work-queue) — file: `modules/learning/internal/application/command/reporting/create_report.go`
- [ ] T015 [US1] Command `RunReportJob`: reader replica theo lô 10.000 dòng có scope, writer CSV/XLSX/PDF, upload, audit `report.exported`, `report.completed` — file: `modules/learning/internal/application/command/reporting/run_report_job.go`, `modules/learning/internal/infrastructure/export/xlsx_writer.go`, `modules/learning/internal/infrastructure/export/csv_writer.go`, `modules/learning/internal/infrastructure/export/pdf_writer.go`
- [ ] T016 [US1] Query `GetReport` (presign khi `succeeded`) + handler 6 operationId + work-queue consumer — file: `modules/learning/internal/application/query/reporting/get_report.go`, `modules/learning/internal/presentation/http/public/report_handler.go`, `modules/learning/internal/presentation/http/public/admin/report_handler.go`, `modules/learning/internal/presentation/consumer/report_job_worker.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Bảng kết quả course theo điểm tự chấm (Priority: P1)

**Goal**: `getCourseResults` đúng `score_policy`, không XP/rating.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T017 [P] [US2] Integration test F044-AC-08, F044-AC-09: điểm theo `best/last/first`, thang `score_scale`, điểm hiệu lực sau re-evaluate, không cột XP/rating — file: `modules/learning/tests/integration/reporting/course_results_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Test authorization F044-AC-10: môn ngoài scope `PERMISSION_DENIED`, trường/tenant khác 404 (TC-TEN-002, TC-TEN-003) — file: `modules/learning/tests/integration/reporting/course_results_authz_test.go`

### Implementation cho User Story 2

- [ ] T019 [US2] Query `GetCourseResults` (SQL đọc `lms.assignment_submissions` + `assessment.attempt_score_revisions`, scope) + handler — file: `modules/learning/internal/application/query/reporting/get_course_results.go`, `modules/learning/internal/presentation/http/public/report_handler.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Báo cáo tham khảo theo TT27/2020 và TT22/2021 (Priority: P2)

**Goal**: Mức tham khảo có version hồ sơ và disclaimer.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T020 [P] [US3] Unit test F044-AC-11: ánh xạ AC-ARP-001 (Hoàn thành tốt), AC-ARP-002 (Chưa đủ dữ liệu), TT22 Đạt/Chưa đạt, điểm tham khảo đề official (TC-LRN-018) — file: `modules/learning/tests/unit/regulation/mapper_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US3] Integration test F044-AC-12, F044-AC-14: disclaimer ở response và đầu/chân file, audit `learning.reference_report_exported`; đổi ngưỡng → version mới, kỳ trước giữ nguyên — file: `modules/learning/tests/integration/regulation/reference_report_test.go`
- [ ] T022 [P] [US3] Contract test F044-AC-13: quét response mọi operation học sinh không có `level_code`/nhãn thông tư — file: `modules/learning/tests/contract/regulation/no_student_labels_test.go`
- [ ] T023 [P] [US3] Test authorization F044-AC-15: `academic_manager` môn ngoài scope 403; báo cáo có `rapid_guess_ratio`, đánh dấu đề độ tin cậy thấp (TC-TEN-002) — file: `modules/learning/tests/integration/regulation/reference_report_authz_test.go`

### Implementation cho User Story 3

- [ ] T024 [P] [US3] Domain `regulation.Profile`, `regulation.Mapper` — file: `modules/learning/internal/domain/regulation/mapper.go`
- [ ] T025 [US3] Command `ComputeReferenceLevels` + job `reference-levels-nightly`; command `PublishAssessmentProfileVersion` — file: `modules/learning/internal/application/command/regulation/compute_reference_levels.go`, `modules/learning/internal/presentation/job/reference_levels_nightly.go`
- [ ] T026 [US3] Query `GetReferenceReport` + handler `getRegulationReferenceReport`; đưa `regulation_reference` vào writer export — file: `modules/learning/internal/application/query/reporting/get_reference_report.go`, `modules/learning/internal/presentation/http/public/report_handler.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Thống kê chất lượng câu hỏi (item analysis) (Priority: P2)

**Goal**: `content.question_stats` hằng đêm, API CMS.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T027 [P] [US4] Unit test F044-AC-16: `p_value`, point-biserial, `distractor_rate`, loại `rapid_guess`, chỉ lần đầu `is_scored`, tách gợi ý, ngưỡng lượt — file: `modules/learning/tests/unit/itemanalysis/stats_test.go` — phải FAIL trước khi code
- [ ] T028 [P] [US4] Integration test F044-AC-17: thống kê theo version + cờ; checksum version publish không đổi sau job — file: `modules/learning/tests/integration/itemanalysis/question_stats_test.go`
- [ ] T029 [P] [US4] Test tenant isolation F044-AC-18: câu tenant khác 404, câu platform được gán đọc được (TC-TEN-001) — file: `modules/learning/tests/integration/itemanalysis/question_stats_tenant_test.go`

### Implementation cho User Story 4

- [ ] T030 [P] [US4] Domain `itemanalysis.Stats` — file: `modules/learning/internal/domain/itemanalysis/stats.go`
- [ ] T031 [US4] Command `ComputeQuestionStats` (đọc `attempt_answers` lần đầu theo partition, ghi stats) + job `item-analysis-nightly` — file: `modules/learning/internal/application/command/itemanalysis/compute_question_stats.go`, `modules/learning/internal/presentation/job/item_analysis_nightly.go`
- [ ] T032 [US4] Query `GetQuestionStats` + handler — file: `modules/learning/internal/application/query/itemanalysis/get_question_stats.go`, `modules/learning/internal/presentation/http/public/cms_quality_handler.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Đề xuất hiệu chỉnh độ khó có người duyệt (Priority: P3)

**Goal**: Hàng đợi đề xuất, chấp nhận tạo version mới.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T033 [P] [US5] Unit test F044-AC-19: bảng ánh xạ `difficulty_calibration`, lệch ≥ 1 mức, chặn đề xuất lại 90 ngày sau 2 lần bỏ qua — file: `modules/learning/tests/unit/itemanalysis/calibration_test.go` — phải FAIL trước khi code
- [ ] T034 [P] [US5] Integration test F044-AC-20: accept tạo version mới `change_note = difficulty_calibration` qua workflow, không sửa version publish, replay key không tạo version thứ hai, audit — file: `modules/learning/tests/integration/itemanalysis/accept_calibration_test.go`
- [ ] T035 [P] [US5] Test authorization F044-AC-21: thiếu `learning_content.review`, tác giả, role tenant với câu platform → 403 (TC-CMS-ASG-02) — file: `modules/learning/tests/integration/itemanalysis/calibration_authz_test.go`

### Implementation cho User Story 5

- [ ] T036 [P] [US5] Domain `itemanalysis.Calibration` — file: `modules/learning/internal/domain/itemanalysis/calibration.go`
- [ ] T037 [US5] Command `ProposeCalibrations` + job `calibration-weekly`; `AcceptCalibration` (gọi port `QuestionVersionCreator` của workflow F025), `DismissCalibration` — file: `modules/learning/internal/application/command/itemanalysis/propose_calibrations.go`, `modules/learning/internal/application/command/itemanalysis/accept_calibration.go`, `modules/learning/internal/presentation/job/calibration_weekly.go`
- [ ] T038 [US5] Query `ListCalibrationProposals` + handler list/accept/dismiss — file: `modules/learning/internal/application/query/itemanalysis/list_calibration_proposals.go`, `modules/learning/internal/presentation/http/public/cms_quality_handler.go`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: User Story 6 — Chất lượng đề: cân bằng biến thể, độ tin cậy, phân tích đề (Priority: P2)

**Goal**: `VariantBalancer`, độ tin cậy, `getExamSessionReport`.

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [ ] T039 [P] [US6] Unit test F044-AC-22: `P_v` từ `p_value`/`p̂`, ≤ 200 lần lặp seed cố định, đạt ngưỡng `variant_balance_max_mean_p_diff` hoặc cảnh báo, tái lập được (TC-EXM-001) — file: `modules/learning/tests/unit/examquality/balancer_test.go` — phải FAIL trước khi code
- [ ] T040 [P] [US6] Unit test F044-AC-23: KR-20, alpha, SEM khớp tính tay; < số học sinh tối thiểu → không tính; cờ chênh biến thể — file: `modules/learning/tests/unit/examquality/reliability_test.go`
- [ ] T041 [P] [US6] Integration test F044-AC-24: report có phổ điểm, p-value loại `rapid_guess`, `misconception_tag`, cảnh báo độ tin cậy (TC-EXM-004) — file: `modules/learning/tests/integration/examquality/exam_session_report_test.go`
- [ ] T042 [P] [US6] Test tenant isolation + authorization F044-AC-25 cho `getExamSessionReport`: trường/tenant khác 404, học sinh 403 (TC-TEN-001) — file: `modules/learning/tests/integration/examquality/exam_report_authz_test.go`

### Implementation cho User Story 6

- [ ] T043 [P] [US6] Domain `examquality.Balancer` hiện thực interface `VariantBalancer` của F042; `examquality.Reliability` — file: `modules/learning/internal/domain/examquality/balancer.go`, `modules/learning/internal/domain/examquality/reliability.go`
- [ ] T044 [US6] Đăng ký balancer vào paper generator của F042 ở composition root — file: `modules/learning/cmd/api/main.go`
- [ ] T045 [US6] Command `ComputeExamQuality` + consumer `exam.closed` — file: `modules/learning/internal/application/command/examquality/compute_exam_quality.go`, `modules/learning/internal/presentation/consumer/exam_closed_consumer.go`
- [ ] T046 [US6] Query `GetExamSessionReport` + handler — file: `modules/learning/internal/application/query/examquality/get_exam_session_report.go`, `modules/learning/internal/presentation/http/public/exam_report_handler.go`

**Checkpoint**: User Story 6 chạy độc lập.

## Phase 9: Polish & Cross-Cutting

- [ ] T047 [P] Metric + span theo plan.md §Quan sát; kiểm log không chứa điểm/nội dung file — file: `modules/learning/internal/infrastructure/observability/reporting_metrics.go`
- [ ] T048 [P] Đề nghị đăng ký ngưỡng item analysis vào CONFIG_DEFAULTS (PR riêng với QUESTION_BANK §9); cập nhật CHANGELOG và trạng thái F044 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T049 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/044-reports-item-analysis/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T004–T008) → US1, US2 (song song) → US3 → US4 → US5 (cần US4) ; US6 cần US4 cho `p_value` (dùng `p̂` khi chưa đủ dữ liệu) và F042 interface `VariantBalancer`.
- T016, T019, T026 cùng sửa `report_handler.go`; T032, T038 cùng sửa `cms_quality_handler.go`: làm tuần tự.

## Definition of Done

- [ ] Mọi tiêu chí F044-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
