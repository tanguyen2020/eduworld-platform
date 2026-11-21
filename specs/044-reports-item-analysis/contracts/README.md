# Contracts: F044 — Báo cáo, xuất dữ liệu bất đồng bộ, item analysis, chất lượng đề

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`, `x-service: learning`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createLmsReport` | `POST /api/v1/lms/reports` → 202 | `learning.v1.yaml` | `report.export` · `Idempotency-Key` | Đã sửa (F044: thêm `subject_id`, `class_id`, `period`, `from`/`to`; response `ReportJob`); ADR-0095: bỏ `class_id`, `course_id`, loại `course_results`; thêm `grade_level` (bắt buộc với `skill_map`); `/tenant/reports` thay `class_progress` bằng `grade_progress` |
| `getLmsReport` | `GET /api/v1/lms/reports/{report_id}` | `learning.v1.yaml` | người tạo | Đã sửa (F044: `status`, `row_count`, `download_url`, `expires_at`, `error`) |
| `createTenantReport` | `POST /api/v1/tenant/reports` → 202 | `learning.v1.yaml` | `report.export` (TA, SA) · `Idempotency-Key` | Có sẵn, đã hiện thực (F044) |
| `getTenantReport` | `GET /api/v1/tenant/reports/{report_id}` | `learning.v1.yaml` | người tạo | Có sẵn, đã hiện thực (F044) |
| `createPlatformReport` | `POST /api/v1/admin/reports` → 202 | `learning.v1.yaml` | System Admin · `report.export` · gateway nội bộ + MFA | Có sẵn, đã hiện thực (F044) |
| `getPlatformReport` | `GET /api/v1/admin/reports/{report_id}` | `learning.v1.yaml` | người tạo (System Admin) | Có sẵn, đã hiện thực (F044) |
| ~~`getCourseResults`~~ | ~~`GET /api/v1/lms/courses/{course_id}/results`~~ | — | — | **Đã xóa** (ADR-0095: không còn course — kết quả theo bài giao `assignment_results`) |
| `getRegulationReferenceReport` | `GET /api/v1/lms/schools/{school_id}/reference-levels?subject_id=&period=&grade_level=` | — | `analytics.school.read` | Đã thêm (F044); ADR-0095: lọc `grade_level` thay `class_id`, item có `grade_level` |
| `getQuestionStats` | `GET /api/v1/cms/learning/questions/{question_id}/stats` | — | `learning_content.read` | Đã thêm (F044) |
| `listCalibrationProposals` | `GET /api/v1/cms/learning/calibration-proposals?status=` | — | `learning_content.review` | Đã thêm (F044) |
| `acceptCalibrationProposal` | `POST /api/v1/cms/learning/calibration-proposals/{proposal_id}:accept` | — | `learning_content.review` · `Idempotency-Key` · audit | Đã thêm (F044) |
| `dismissCalibrationProposal` | `POST /api/v1/cms/learning/calibration-proposals/{proposal_id}:dismiss` | — | `learning_content.review` · `Idempotency-Key` · audit | Đã thêm (F044) |
| `getExamSessionReport` | `GET /api/v1/exams/sessions/{exam_session_id}/report` | — | `lms.report.read` (SA trường, AM scope) | Đã thêm (F044) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có (portal poll `getLmsReport`) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `audit.recorded` | Phát (`report.exported`, `learning.reference_report_exported`, audit chấp nhận/bỏ qua đề xuất) | `specs/events/envelope.schema.json` | Có sẵn, đã hiện thực (F044) |
| `report.completed` | Phát (thông báo portal) | `specs/events/payloads/learning/report.completed.v1.schema.json` | Đã thêm (F044) |
| `exam.closed` | Tiêu thụ | `specs/events/payloads/learning/exam.closed.v1.schema.json` | Cần thêm (thuộc F042; EXAM_ENGINE §5 có, EVENT_DESIGN §4.3 thiếu) |
| `attempt.re_evaluated` | Tiêu thụ (tính lại bảng kết quả, thống kê) | `specs/events/payloads/learning/attempt.re_evaluated.v1.schema.json` | Cần thêm (thuộc F031) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/assessment-blueprint.schema.json` (`variants.balance`) | Đọc `max_mean_p_diff`, `strategy` cho bộ cân bằng | Có sẵn, đã hiện thực (F044) |
| `specs/schemas/assessment-profile.schema.json` | `rules`, `disclaimer` của hồ sơ TT27/TT22 (mọi chuỗi `maxLength`) | Đã thêm (F044) |

## 6. Delta cần thực hiện

- Schema `ReportJob` dùng chung cho `/lms/reports`, `/tenant/reports`, `/admin/reports`: `report_id`, `report_type`, `format`, `status` (enum `queued/running/succeeded/failed/expired`), `row_count`, `download_url` (`format: uri`, maxLength 2048, chỉ khi `succeeded`), `expires_at`, `error` (maxLength 1000); `LmsReportCreate` thêm `subject_id`, `class_id`, `period` (enum 4 kỳ), `from`, `to`; lỗi `429 EXPORT_LIMIT_REACHED`, `422 VALIDATION_FAILED`.
- Thêm 6 endpoint "Cần thêm" ở §1 với schema `RegulationReferenceReport` (có `disclaimer` maxLength 500, `profile_version`, `items[] {student_id, level_code, coverage_ratio, official_score, rapid_guess_ratio}`), `QuestionStats`, `CalibrationProposal`, `ExamSessionReport` (`variants[]`, `reliability_warning`, `score_distribution`, `items[] {question_version_id, p_value, discrimination, top_misconceptions[] (maxLength 64), avg_time_ms, rapid_guess_rate}`).
- Thêm JSON Schema `assessment-profile.schema.json`; payload `report.completed.v1`.
- Đề nghị đăng ký ngưỡng item analysis (≥ 30 lượt, QUESTION_BANK §9) vào CONFIG_DEFAULTS §13 (không sửa trong đợt này).
