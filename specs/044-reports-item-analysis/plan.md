# Implementation Plan: F044 — Báo cáo, xuất dữ liệu bất đồng bộ, item analysis, chất lượng đề

**Branch**: `044-reports-item-analysis` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Bổ sung vào module `learning`: (1) module `reporting`: bảng `learning.report_jobs`, command tạo job (Idempotency-Key, giới hạn đồng thời), NATS work-queue `LEARNING_REPORT_JOBS`, worker đọc read replica (role `learning_dashboard`) theo lô 10.000 dòng, ghi CSV/XLSX/PDF có watermark lên `ew-exports`, link ký có hạn, audit `report.exported`; (2) query `getCourseResults` từ `lms.assignment_submissions` + điểm hiệu lực; (3) hồ sơ tham khảo `learning.assessment_profiles` + job hằng đêm tính `learning.rm_student_subject_reference_levels`; (4) job item analysis hằng đêm → `content.question_stats`, job hằng tuần → `content.difficulty_calibration_proposals`, command chấp nhận tạo version mới qua workflow F025; (5) hiện thực `VariantBalancer` (interface của F042) và job độ tin cậy khi session official đóng → `assessment.exam_quality_reports`, phục vụ `getExamSessionReport`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go (work-queue), AWS SDK for Go v2 (S3 presign, floci), excelize (XLSX), thư viện PDF server-side [Proposed], `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/authz`, `src/backend/pkg/idempotency`, `src/backend/pkg/outbox` |
| Storage | PostgreSQL `eduworld_learning` (`learning`, `content`, `assessment`); object storage bucket `ew-exports` (ADR-0044); Valkey không dùng cho dữ liệu bền |
| Testing | go test + testcontainers (PostgreSQL, NATS, floci S3); dữ liệu mẫu tính tay cho item analysis/KR-20 |
| Target Platform | Linux container (module `learning` api + worker) |
| Project Type | backend module |
| Performance Goals | [DASHBOARD_DATA §15](../../docs/07-backend/DASHBOARD_DATA.md) (export không làm chậm dashboard > 10%) |
| Constraints | AGENTS R6, R7, R8, R10, R11, R14, R27, R28, R31 · ADR-0027, ADR-0043, ADR-0044, ADR-0062, ADR-0072 |
| Scale/Scope | `dashboard_export_max_rows`; 50.000 học sinh ([DASHBOARD_DATA §11](../../docs/07-backend/DASHBOARD_DATA.md)) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec.md Ready; delta contract (stats, calibration, exam report, `ReportJob` fields) ở Phase 2 tasks |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module `learning`; tạo version mới qua command của workflow nội dung cùng module; worker gọi command handler qua interface có kiểu |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới T có `tenant_id` + `deleted_at` + RLS (dữ liệu platform `owner_scope`); không hard-delete (file hết hạn theo lifecycle bucket) |
| IV Server-authoritative | ✅ | Thống kê, mức tham khảo, độ tin cậy tính ở server; báo cáo đề không lộ đáp án ngoài phạm vi; tạo job/chấp nhận đề xuất có `Idempotency-Key` + audit |
| V Xác thực, phân quyền | ✅ | `report.export`, `lms.report.read`, `analytics.school.read`, `learning_content.read`, `learning_content.review`; route `/admin/reports` gateway nội bộ + MFA |
| VI An toàn trẻ em | ✅ | Không xuất chat; watermark; ẩn ô < 5 cấp tenant/platform; không nhãn thông tư cho học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; ngưỡng ≥ 30 lượt link QUESTION_BANK §9 (đề nghị đăng ký CONFIG_DEFAULTS); chuỗi `MaxLen` |
| VIII UI tách logic, token | ✅ | Không có UI (F054, F056) |
| IX Test có bằng chứng | ✅ | Mỗi AC có task test viết trước; test tenant isolation + authorization mọi endpoint mới |

## Project Structure

### Documentation (feature này)

```text
specs/044-reports-item-analysis/
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
├── ent/schema/                         # report_job.go, assessment_profile.go, reference_level.go, question_stat.go, difficulty_calibration_proposal.go, exam_quality_report.go
├── internal/domain/
│   ├── reporting/                      # report_job.go (trạng thái, giới hạn), watermark.go
│   ├── regulation/                     # profile.go, mapper.go (ánh xạ M_eff → mức, độ phủ)
│   ├── itemanalysis/                   # stats.go (p_value, point-biserial, distractor), calibration.go
│   └── examquality/                    # balancer.go (hiện thực exam.VariantBalancer), reliability.go (KR-20, alpha, SEM)
├── internal/application/
│   ├── command/reporting/              # create_report.go, run_report_job.go
│   ├── command/regulation/             # compute_reference_levels.go, publish_assessment_profile_version.go
│   ├── command/itemanalysis/           # compute_question_stats.go, propose_calibrations.go, accept_calibration.go, dismiss_calibration.go
│   ├── command/examquality/            # compute_exam_quality.go
│   ├── query/reporting/                # get_report.go, get_course_results.go, get_reference_report.go
│   ├── query/itemanalysis/             # get_question_stats.go, list_calibration_proposals.go
│   ├── query/examquality/              # get_exam_session_report.go
│   └── port/                           # ReportStore (S3), ReportRowReader (replica), QuestionVersionCreator (workflow F025)
├── internal/infrastructure/
│   ├── storage/                        # exports_bucket.go (presign GET, khóa exports/<tenant_id>/<report_id>.<ext>)
│   ├── export/                         # csv_writer.go, xlsx_writer.go, pdf_writer.go
│   └── persistence/reporting|itemanalysis/
├── internal/presentation/
│   ├── http/public/                    # createLmsReport, getLmsReport, createTenantReport, getTenantReport, getCourseResults, getQuestionStats, listCalibrationProposals, accept/dismiss, getExamSessionReport
│   ├── http/public/admin/              # createPlatformReport, getPlatformReport
│   ├── consumer/                       # report_job_worker.go (work-queue), exam_closed_consumer.go (exam.closed → quality)
│   └── job/                            # item_analysis_nightly.go, calibration_weekly.go, reference_levels_nightly.go
└── tests/{unit,integration,contract}/reporting|itemanalysis|examquality|regulation/
```

**Structure Decision**: Mọi dữ liệu nguồn (attempt, mastery, nội dung, LMS) nằm trong `eduworld_learning` nên báo cáo và thống kê ở module `learning` (ADR-0037, DASHBOARD_DATA D-6); export đọc replica bằng role dashboard; `VariantBalancer` là interface domain của F042 để F044 hiện thực mà không sửa generator.

## Phase 0: Research

Không có quyết định mở. Công thức KR-20/alpha/SEM, point-biserial và thuật toán cân bằng đã ở EXAM_ENGINE §4.1, §8.1 và QUESTION_BANK §9; chọn thư viện PDF ghi [Proposed] trong Assumptions.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `reporting.ReportJob`, `regulation.Mapper`, `itemanalysis.Stats`, `itemanalysis.Calibration`, `examquality.Balancer`, `examquality.Reliability` | Thuần, seed cố định cho balancer |
| application | Command `CreateReport`, `RunReportJob`, `ComputeReferenceLevels`, `ComputeQuestionStats`, `ProposeCalibrations`, `AcceptCalibration`, `DismissCalibration`, `ComputeExamQuality` · Query `GetReport`, `GetCourseResults`, `GetReferenceReport`, `GetQuestionStats`, `ListCalibrationProposals`, `GetExamSessionReport` | Permission theo contracts §1 |
| infrastructure | Repository ent; reader replica theo lô; S3 `ew-exports` presign; writer CSV/XLSX/PDF; adapter tạo version câu hỏi (port tới command workflow F025) | SQL tường minh có scope + `deleted_at IS NULL` |
| presentation | Handler 13 operationId; work-queue consumer; consumer `exam.closed`; job hằng đêm/tuần | Consumer idempotent |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F042 (`ExamPaper`, `VariantBalancer`, `exam.closed`), F043 (rollup, role `learning_dashboard`), F041 (`lms.assignment_submissions`), F031/F032 (`objective_masteries`, `rapid_guess`, `hints_used`), F025/F026 (workflow tạo/duyệt version).
- Facade: không gọi facade module khác.
- Event phát: `audit.recorded` (`report.exported`, `learning.reference_report_exported`, `content.version_created` khi chấp nhận hiệu chỉnh), `report.completed` [Proposed, thông báo portal]. Event tiêu thụ: `exam.closed`, `attempt.re_evaluated`.

### Bảo mật, tenant, idempotency, audit

- `createLmsReport`/`createTenantReport`/`createPlatformReport`, `acceptCalibration`, `dismissCalibration`: `Idempotency-Key` bắt buộc + audit.
- Export: watermark, phạm vi quyền áp ở reader (scope trường/môn), System Admin `CrossTenantScope` + audit `cross_tenant.accessed`.
- Link: presigned GET `dashboard_export_link_ttl_minutes`, chỉ cấp cho người tạo; khóa file không đoán được.

### Quan sát

- Metric: `eduworld_export_jobs_total{status}`, `eduworld_export_jobs_duration_seconds`, `eduworld_item_analysis_questions_total`, `eduworld_calibration_proposals_total{status}`, `eduworld_exam_reliability_low_total`.
- Log không chứa nội dung file/điểm học sinh; span `report.run`, `itemanalysis.compute`, `examquality.compute`.

Ước lượng: 2–3 sprint (backend 2 dev) [Proposed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
