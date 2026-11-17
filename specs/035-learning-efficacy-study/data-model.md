# Data Model: F035 — Nghiên cứu hiệu quả học tập

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_learning`. Bảng **T** dùng mixin `Base` (`tenant_id`, `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason varchar(64)`, `version`) + RLS; bảng **G** là dữ liệu platform (không `tenant_id`, có `deleted_at`, `deleted_by`, `version`). Mọi bảng `learning.*` dưới là **Mới**; task T009 cập nhật DATABASE_DESIGN §2.5, §2.6, §2.7.

## 1. Entity

### Cột thêm vào bảng có sẵn

| Bảng | Cột | Kiểu | Ràng buộc |
|---|---|---|---|
| `content.question_versions` (F022) | `efficacy_instrument` | bool | NOT NULL DEFAULT false; CHECK `NOT efficacy_instrument OR (exam_only AND owner_scope = 'platform')` |
| `content.assessment_blueprint_versions` (F024) | `efficacy_instrument` | bool | NOT NULL DEFAULT false; CHECK `NOT efficacy_instrument OR owner_scope = 'platform'` |
| `content.question_delivery_v` (view) | `efficacy_instrument` | | thêm cột cho bộ lọc chọn câu |
| `assessment.exam_sessions` (F042) | `efficacy_wave_id` | uuid NULL | idx `(tenant_id, efficacy_wave_id) WHERE efficacy_wave_id IS NOT NULL AND deleted_at IS NULL` |

### ResearchAgreement — `learning.research_agreements` (Mới, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `agreement_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NOT NULL, RLS | tenant đích |
| `agreement_ref` | `varchar(128)` | NOT NULL | mã văn bản thỏa thuận pilot |
| `covers` | `varchar(64)[]` | CHECK `cardinality BETWEEN 1 AND 2`, phần tử ∈ {`efficacy_study`,`experiment`} | |
| `agreed_on` | date | NOT NULL | |
| `status` | `varchar(64)` | `active`/`revoked` | |
| `revoked_at`, `revoked_by` | timestamptz, uuid | NULL | |
| `revoke_reason` | `varchar(1000)` | NULL | |
| `recorded_by` | uuid | NOT NULL | |

Partial unique `(tenant_id, agreement_ref) WHERE deleted_at IS NULL`.

### EfficacyStudy — `learning.efficacy_studies` (Mới, G)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `study_id` | UUIDv7 | PK | |
| `code` | `varchar(64)` | partial unique `WHERE deleted_at IS NULL` | |
| `title` | `varchar(255)` | NOT NULL | |
| `subject_id` | uuid | NOT NULL | |
| `grade_level` | smallint | 1–9 | |
| `objective_ids` | uuid[] | CHECK `cardinality BETWEEN 1 AND 100` | |
| `form_a_blueprint_version_id`, `form_b_blueprint_version_id` | uuid | NOT NULL, khác nhau | ghim (ADR-0043) |
| `min_intervention_weeks` | smallint | CHECK ≥ 1 | |
| `status` | `varchar(64)` | `draft`/`active`/`completed`/`cancelled` | |
| `created_by` | uuid | NOT NULL | |
| `activated_at`, `completed_at` | timestamptz | NULL | |
| `idempotency_key` | `varchar(64)` | UNIQUE `(created_by, idempotency_key)` | |

### EfficacyStudyClass — `learning.efficacy_study_classes` (Mới, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `study_id`, `class_id` | uuid | PK `(study_id, class_id)` | |
| `tenant_id`, `school_id` | uuid | NOT NULL, RLS | |
| `pair_no` | smallint | NULL với `validation`; 1–200 | |
| `arm` | `varchar(64)` | NULL trước kích hoạt; `intervention`/`comparison`/`validation` | |
| `assignment_method` | `varchar(64)` | `manual`/`random` | |
| `form_order` | `varchar(64)` | NULL trước kích hoạt; `a_then_b`/`b_then_a` | |
| `status` | `varchar(64)` | `enrolled`/`withdrawn` | |
| `withdrawn_at` | timestamptz | NULL | |
| `withdrawal_id` | uuid | NULL | |

Partial unique `(class_id) WHERE status = 'enrolled' AND deleted_at IS NULL` (một lớp một study đang chạy); idx `(study_id, pair_no)`.

### EfficacyParticipant — `learning.efficacy_participants` (Mới, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `study_id`, `student_id` | uuid | PK `(study_id, student_id)` | |
| `tenant_id`, `school_id`, `class_id` | uuid | NOT NULL, RLS | `class_id` lúc chụp |
| `arm`, `form_order` | `varchar(64)` | NOT NULL | sao từ lớp |
| `status` | `varchar(64)` | `active`/`withdrawn`/`excluded` | |
| `exclusion_reason` | `varchar(64)` | NULL; `withdrawn`/`anonymized`/`processing_restricted` | |
| `enrolled_at` | timestamptz | NOT NULL | thời điểm mở đợt `pre` |
| `withdrawn_at`, `withdrawal_id` | | NULL | |

### EfficacyWave — `learning.efficacy_waves` (Mới, G) · EfficacyWaveSession — `learning.efficacy_wave_sessions` (Mới, T)

`efficacy_waves`: `wave_id` PK, `study_id`, `kind varchar(64)` (`validation`/`pre`/`post`), `form varchar(16)` NULL (`a`/`b`, chỉ `validation`), `window_start`, `window_end` (CHECK `>`), `duration_sec int` (> 0, mặc định từ blueprint), `status varchar(64)` (`scheduled`/`open`/`closed`/`cancelled`), `opened_at`, `closed_at`, `created_by`, `idempotency_key varchar(64)`; partial unique `(study_id, kind) WHERE kind IN ('pre','post') AND status <> 'cancelled' AND deleted_at IS NULL`.

`efficacy_wave_sessions`: PK `(wave_id, class_id)`, `tenant_id`, `school_id`, `exam_session_id uuid NOT NULL UNIQUE`, `form varchar(16)` (`a`/`b`), `status varchar(64)` (`active`/`cancelled`).

### EfficacyWithdrawal — `learning.efficacy_withdrawals` (Mới, T)

`withdrawal_id` PK, `tenant_id` (RLS), `study_id` NULL (NULL = mọi study, do thu hồi thỏa thuận), `scope varchar(64)` (`tenant`/`school`/`class`/`students`), `school_id`, `class_id` NULL, `student_ids uuid[]` CHECK `cardinality ≤ 500`, `reason varchar(1000)` NOT NULL (≥ 10), `school_request_ref varchar(128)` NULL, `affected_count int`, `requested_by`, `idempotency_key varchar(64)` UNIQUE `(requested_by, idempotency_key)`. Append-only (không cập nhật sau tạo).

### EfficacyMasterySnapshot — `learning.efficacy_mastery_snapshots` (Mới, T)

PK `(study_id, student_id, objective_id)`; `tenant_id` (RLS), `m_eff numeric(4,3)`, `level varchar(64)` (`MasteryLevel`), `model_version varchar(64)`, `evidence_count int`, `taken_at timestamptz`; soft-delete khi rút (`delete_reason = 'efficacy_withdrawn'`).

### EfficacyReport — `learning.efficacy_reports` (Mới, G)

`report_id` PK, `study_id`, `status varchar(64)` (`queued`/`running`/`succeeded`/`failed`), `params jsonb` (CHECK ≤ 8192 B: `excluded_question_version_ids` ≤ 100), `results jsonb` (CHECK ≤ 262144 B; đã ẩn ô nhỏ), `inputs_hash bytea` (CHECK `octet_length = 32`), `descriptive_only bool`, `error varchar(1000)`, `requested_by`, `idempotency_key varchar(64)` UNIQUE `(requested_by, idempotency_key)`, `started_at`, `finished_at`.

### LearningExperiment — `learning.experiments` (Mới, G; tên theo LEARNING_EFFICACY_PLAN §6)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `experiment_id` | UUIDv7 | PK | |
| `code` | `varchar(64)` | partial unique `WHERE deleted_at IS NULL`; `^[a-z0-9_-]{1,64}$` | |
| `kind` | `varchar(64)` | `mastery_model`/`recommendation_rule` | |
| `unit` | `varchar(64)` | `class`/`student`, DEFAULT `class` | |
| `arms` | jsonb | CHECK ≤ 16384 B | `[{code ≤ 32, allocation_pct, is_control, model_version?, rule_params?}]` 2–5 phần tử |
| `tenant_ids` | uuid[] | CHECK `cardinality BETWEEN 1 AND 200` | |
| `study_id` | uuid | NULL | giới hạn đơn vị vào lớp `intervention` |
| `status` | `varchar(64)` | `draft`/`shadow`/`live`/`stopped`/`completed` | |
| `start_at`, `end_at` | timestamptz | CHECK `end_at > start_at` | |
| `stop_reason` | `varchar(64)` | NULL; `manual`/`guardrail_breach`/`agreement_revoked`/`ended` | |
| `revision` | int | NOT NULL DEFAULT 1 | tăng mỗi lần sửa (chỉ `draft`) |
| `owner_id` | uuid | NOT NULL | |

### ExperimentAssignment — `learning.experiment_assignments` (Mới, T)

PK `(experiment_id, student_id)`; `tenant_id` (RLS), `unit_id uuid` (class hoặc student), `bucket smallint` (0–99), `arm_code varchar(32)`, `assigned_at`; `status varchar(64)` (`active`/`removed`).

### ExperimentMasteryState — `learning.experiment_mastery_states` (Mới, T)

PK `(experiment_id, arm_code, tenant_id, student_id, objective_id)`; cột trạng thái như `objective_masteries` (F031): `value numeric(4,3)`, `level varchar(64)`, `evidence_count`, `distinct_days`, `last_evidence_at`, `last_evidence_date`, `model_version varchar(64)`, `version bigint`; với arm control ở `live` lưu bản shadow của mô hình `active` để so sánh.

### ExperimentResult — `learning.experiment_results` (Mới, G)

`result_id` PK, `experiment_id`, `computed_at`, `results jsonb` (CHECK ≤ 65536 B; theo arm, đã ẩn ô nhỏ), `guardrail_breached bool`.

## 2. Vòng đời / trạng thái

| Entity | Chuyển trạng thái |
|---|---|
| EfficacyStudy | `draft → active` (`activateEfficacyStudy`) `→ completed` (`completeEfficacyStudy`, sau đợt `post` đóng) · `draft|active → cancelled` |
| EfficacyWave | `scheduled → open` (job tại `window_start`) `→ closed` (tại `window_end`) · `scheduled → cancelled` |
| EfficacyParticipant | `active → withdrawn` (rút) · `active → excluded` (ẩn danh, hạn chế xử lý) |
| ResearchAgreement | `active → revoked` |
| LearningExperiment | `draft → shadow → live → stopped | completed` · `shadow → stopped | completed` |

## 3. Quy tắc kiểm tra dữ liệu

- Dạng A/B: blueprint `efficacy_instrument`, `variants.strategy = fixed`, cùng môn/khối với study, phủ đúng `objective_ids`, đủ số câu mỗi LO theo [LEARNING_EFFICACY_PLAN §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md).
- Mỗi `pair_no` có đúng 2 lớp `intervention` + `comparison` khi kích hoạt; `validation` không có cặp và không vào đợt `pre`/`post`.
- `post.window_start ≥ pre.window_end + min_intervention_weeks`.
- Arm thử nghiệm: đúng một control; tổng `allocation_pct` = 100; `rule_params` theo danh mục cho phép của `recommendation.Engine` (F032).
- Mọi truy vấn đọc có `tenant_id` (T) và `deleted_at IS NULL` (R27).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `audit.recorded` | Mọi thao tác ghi, xem liên tenant, xuất dữ liệu | envelope có sẵn |
| learning event (mọi `event_type`) | Attempt đo hiệu quả, học sinh trong thử nghiệm | `specs/schemas/learning-event.schema.json` — Cần sửa `$defs/context`: `efficacy_wave_id`, `experiment_arms` |
| `exam.*` | **Không phát** cho ExamSession có `efficacy_wave_id` | — |
