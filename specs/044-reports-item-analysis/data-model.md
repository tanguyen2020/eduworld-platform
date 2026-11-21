# Data Model: F044 — Báo cáo, xuất dữ liệu bất đồng bộ, item analysis, chất lượng đề

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_learning`. Bảng T dùng mixin `Base` (`tenant_id`, `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason varchar(64)`, `version`) + RLS; bảng H (nội dung lai) theo quy ước `owner_scope` (DATABASE_DESIGN §2). Mọi bảng dưới là **Mới** trừ khi ghi khác; task T008 cập nhật DATABASE_DESIGN §2.6, §2.7, §2.5.

## 1. Entity

### ReportJob — `learning.report_jobs` (Mới)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `report_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NULL = báo cáo liên tenant của System Admin | RLS; platform report chỉ role `learning_platform` |
| `scope_level` | `varchar(64)` | CHECK `IN ('lms','tenant','platform')` | Route tạo |
| `report_type` | `varchar(64)` | CHECK `IN ('assignment_results','course_results','regulation_reference','skill_map','school_monthly','tenant_learning_summary','platform_learning_summary')` — ADR-0095: thêm `grade_progress` (migration 20261229000006), `course_results`, `class_progress` chỉ còn cho job cũ | |
| `format` | `varchar(16)` | CHECK `IN ('csv','xlsx','pdf')` | |
| `params` | `jsonb` | CHECK `octet_length ≤ 4096` | `school_id`, `grade_level` (ADR-0095 thay `class_id`, `course_id`), `assignment_id`, `subject_id`, `from`, `to`, `tenant_ids[]` |
| `requested_by` | uuid | NOT NULL | Chỉ người này lấy link |
| `status` | `varchar(64)` | CHECK `IN ('queued','running','succeeded','failed','expired')` | |
| `row_count` | `int` | NULL | |
| `storage_key` | `varchar(512)` | NULL | `exports/<tenant_id|platform>/<report_id>.<ext>` |
| `error` | `varchar(1000)` | NULL | Không PII |
| `idempotency_key` | `varchar(64)` | NOT NULL | UNIQUE `(requested_by, idempotency_key) WHERE deleted_at IS NULL` |
| `started_at`, `finished_at`, `file_expires_at` | `timestamptz` | | `file_expires_at = finished_at + dashboard_export_file_retention_days` |

**Index**: `(tenant_id, requested_by, status) WHERE deleted_at IS NULL` (đếm job đang chạy).

### AssessmentProfile — `learning.assessment_profiles` (Mới, loại H)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `profile_id` | UUIDv7 | PK | |
| `tenant_id`, `owner_scope` | uuid NULL, `varchar(64)` | CHECK `(owner_scope = 'platform') = (tenant_id IS NULL)` | |
| `profile_code` | `varchar(64)` | CHECK `IN ('primary_tt27','lower_secondary_tt22')` | |
| `version` | `int` | UNIQUE `(coalesce(tenant_id, uuid_nil), profile_code, version) WHERE deleted_at IS NULL` | Version mới khi đổi ngưỡng |
| `grade_levels` | `smallint[]` | CHECK `cardinality ≤ 9` | |
| `subject_ids` | `uuid[]` | CHECK `cardinality ≤ 50` | |
| `rules` | `jsonb` | CHECK `octet_length ≤ 16384` | Ngưỡng theo `assessment_reference_thresholds` |
| `disclaimer` | `jsonb` | CHECK `octet_length ≤ 2048` | i18n vi/en, mỗi bản ≤ 500 ký tự |
| `status` | `varchar(64)` | CHECK `IN ('active','superseded')` | |

### ReferenceLevel — `learning.rm_student_subject_reference_levels` (Mới, read model)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `tenant_id`, `school_id`, `class_id`, `student_id`, `subject_id` | uuid | NOT NULL | ADR-0095: `class_id` đã DROP (`20261230000001_adr0095_drop_class_course`); thêm `grade_level` = khối lúc tính |
| `academic_year_id` | uuid | NOT NULL | |
| `period` | `varchar(64)` | CHECK `IN ('mid_term_1','end_term_1','mid_term_2','end_term_2')` | |
| `profile_id`, `profile_version` | uuid, `int` | NOT NULL | Giữ version đã dùng |
| `level_code` | `varchar(64)` | CHECK `IN ('excellent','completed','not_completed','pass','not_pass','insufficient_data')` | Hiển thị bằng nhãn i18n |
| `coverage_ratio`, `proficient_ratio`, `mastered_ratio` | `numeric(5,4)` | | |
| `official_score` | `numeric(4,1)` | NULL | Trung bình đề `is_official` thang 10 |
| `low_reliability_exam_ids` | `uuid[]` | CHECK `cardinality ≤ 20` | |
| `rapid_guess_ratio` | `numeric(5,4)` | | |
| `computed_at` | `timestamptz` | NOT NULL | |

PK `(tenant_id, student_id, subject_id, academic_year_id, period, profile_version)`. Bảng dẫn xuất, không `deleted_at` (như rollup, DASHBOARD_DATA §3.1).

### QuestionStats — `content.question_stats` (Mới; tên theo QUESTION_BANK §9)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `tenant_id` | uuid NULL | | NULL = câu platform; thống kê platform gộp mọi tenant được gán |
| `question_id`, `question_version_id` | uuid | NOT NULL | |
| `grade_level` | `smallint` | NOT NULL | Tách theo khối |
| `first_attempt_count`, `excluded_rapid_guess_count`, `with_hint_count` | `int` | ≥ 0 | |
| `p_value`, `p_value_with_hint`, `discrimination` | `numeric(5,4)` | NULL khi chưa đủ lượt | |
| `distractor_rates` | `jsonb` | CHECK `octet_length ≤ 4096` | `{option_id: rate}` |
| `avg_time_ms` | `int` | | |
| `rapid_guess_rate`, `hint_usage_rate` | `numeric(5,4)` | | |
| `flags` | `varchar(64)[]` | CHECK `cardinality ≤ 8` | `needs_calibration`, `unused_option`, `suggest_option_feedback`, `low_discrimination` |
| `computed_at` | `timestamptz` | NOT NULL | |

PK `(coalesce(tenant_id, uuid_nil), question_version_id, grade_level)`. Bảng dẫn xuất (không `deleted_at`).

### DifficultyCalibrationProposal — `content.difficulty_calibration_proposals` (Mới, loại H)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `proposal_id` | UUIDv7 | PK | |
| `tenant_id`, `owner_scope` | | theo câu | |
| `question_id`, `based_on_version_id` | uuid | NOT NULL | |
| `current_difficulty`, `proposed_difficulty` | `smallint` | 1–5, khác nhau | |
| `p_value` | `numeric(5,4)` | NOT NULL | |
| `sample_size` | `int` | ≥ ngưỡng `difficulty_calibration` | |
| `grade_distribution` | `jsonb` | CHECK `octet_length ≤ 2048` | |
| `status` | `varchar(64)` | CHECK `IN ('pending','accepted','dismissed')` | |
| `decided_by`, `decided_at` | uuid, `timestamptz` | NULL | |
| `created_version_id` | uuid | NULL | Version mới khi `accepted` |
| `consecutive_dismissals` | `smallint` | NOT NULL DEFAULT 0 | 2 lần → không đề xuất lại 90 ngày |
| `suppressed_until` | `date` | NULL | |

UNIQUE `(question_id) WHERE status = 'pending' AND deleted_at IS NULL`.

### ExamQualityReport — `assessment.exam_quality_reports` (Mới)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `exam_session_id` | uuid | PK (cùng `tenant_id`) | |
| `tenant_id`, `school_id` | uuid | NOT NULL | |
| `variants` | `jsonb` | CHECK `octet_length ≤ 32768` | Mỗi biến thể: `n`, `mean`, `sd`, `reliability`, `method` (`kr20`/`alpha`), `sem`, `expected_p` |
| `variant_gap_flag` | `boolean` | | Chênh ≥ 1 điểm thang 10 |
| `problem_items` | `jsonb` | CHECK `octet_length ≤ 32768` | `question_version_id`, `p_value`, `discrimination`, gợi ý `drop_item` |
| `score_distribution` | `int[]` | CHECK `cardinality ≤ 21` | Phổ điểm |
| `reliability_warning` | `boolean` | | Theo `exam_reliability_warn` |
| `computed_at` | `timestamptz` | NOT NULL | |

### ExamPaper — `assessment.exam_papers` (Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `expected_p` | `numeric(5,4)` | NULL | Ghi bởi `VariantBalancer` (EXAM_ENGINE §4.1) |
| `balance_warning` | `boolean` | NOT NULL DEFAULT false | Không đạt ngưỡng sau 200 lần lặp |

## 2. Vòng đời / trạng thái

| Thực thể | Chuyển trạng thái |
|---|---|
| ReportJob | `queued` → `running` → `succeeded` (link cấp theo yêu cầu, hết hạn `dashboard_export_link_ttl_minutes`) → `expired` (sau `file_expires_at`) · `running` → `failed` |
| DifficultyCalibrationProposal | `pending` → `accepted` (tạo version mới) · `pending` → `dismissed` (`consecutive_dismissals++`) |
| AssessmentProfile | `active` → `superseded` khi tạo version mới |

## 3. Quy tắc kiểm tra dữ liệu

- Item analysis chỉ dùng lần trả lời đầu `is_scored = true`, loại `rapid_guess = true`, câu đủ lượt theo QUESTION_BANK §9.
- Đề xuất hiệu chỉnh chỉ khi số lượt ≥ ngưỡng `difficulty_calibration` và lệch ≥ 1 mức.
- Mức tham khảo "Chưa đủ dữ liệu" khi độ phủ dưới ngưỡng hồ sơ (ASSESSMENT_REGULATION_PROFILE §4).
- Export: số dòng ≤ `dashboard_export_max_rows`; job đang chạy/người ≤ `dashboard_export_max_concurrent_per_user`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `audit.recorded` (`report.exported`, `learning.reference_report_exported`, `content.calibration_accepted`) | Xuất file, chấp nhận đề xuất | Envelope chung `specs/events/envelope.schema.json` |
| `report.completed` | Job xong (thông báo portal) | Cần thêm `specs/events/payloads/learning/report.completed.v1.schema.json` [Confirmed] |
| `exam.closed` (tiêu thụ) | Session đóng → tính chất lượng đề | Cần thêm `specs/events/payloads/learning/exam.closed.v1.schema.json` (thuộc F042) |
