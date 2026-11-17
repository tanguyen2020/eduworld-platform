# Contracts: F035 — Nghiên cứu hiệu quả học tập

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`, `x-service: learning`). Route `/admin/learning/*`: `security: keycloakAdminBearer`, `x-internal: true` (gateway nội bộ, MFA). Permission mới [Proposed]: `platform.efficacy_study.manage` (viết tắt **ESM**), `platform.efficacy_study.export`, `platform.learning_experiment.manage` (**LEM**).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createResearchAgreement` | `POST /api/v1/admin/learning/research-agreements` (`X-Tenant-Id` đích) | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `listResearchAgreements` | `GET /api/v1/admin/learning/research-agreements?tenant_ids=&status=` | learning.v1 | ESM | Cần thêm |
| `revokeResearchAgreement` | `POST /api/v1/admin/learning/research-agreements/{agreement_id}:revoke` (`X-Tenant-Id`) | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `createEfficacyStudy` | `POST /api/v1/admin/learning/efficacy-studies` | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `listEfficacyStudies` | `GET /api/v1/admin/learning/efficacy-studies?status=&tenant_ids=` | learning.v1 | ESM | Cần thêm |
| `getEfficacyStudy` | `GET /api/v1/admin/learning/efficacy-studies/{study_id}` | learning.v1 | ESM | Cần thêm |
| `updateEfficacyStudy` | `PATCH /api/v1/admin/learning/efficacy-studies/{study_id}` (`If-Match`, chỉ `draft`) | learning.v1 | ESM | Cần thêm |
| `addEfficacyStudyClass` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}/classes` (`X-Tenant-Id`) | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `listEfficacyStudyClasses` | `GET /api/v1/admin/learning/efficacy-studies/{study_id}/classes?tenant_ids=` | learning.v1 | ESM | Cần thêm |
| `removeEfficacyStudyClass` | `DELETE /api/v1/admin/learning/efficacy-studies/{study_id}/classes/{class_id}` (`X-Tenant-Id`, soft-delete, chỉ `draft`) | learning.v1 | ESM | Cần thêm |
| `activateEfficacyStudy` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}:activate` | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `scheduleEfficacyWave` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}/waves` | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `listEfficacyWaves` | `GET /api/v1/admin/learning/efficacy-studies/{study_id}/waves` (số mời/đã làm/vắng theo lớp, không điểm) | learning.v1 | ESM | Cần thêm |
| `withdrawEfficacyParticipants` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}:withdraw` (`X-Tenant-Id`) | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `completeEfficacyStudy` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}:complete` | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `createEfficacyStudyReport` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}/reports` → 202 | learning.v1 | ESM · `Idempotency-Key` | Cần thêm |
| `getEfficacyStudyReport` | `GET /api/v1/admin/learning/efficacy-studies/{study_id}/reports/{report_id}` | learning.v1 | ESM | Cần thêm |
| `createLearningExperiment` | `POST /api/v1/admin/learning/experiments` | learning.v1 | LEM · `Idempotency-Key` | Cần thêm |
| `listLearningExperiments` | `GET /api/v1/admin/learning/experiments?status=&kind=` | learning.v1 | LEM | Cần thêm |
| `getLearningExperiment` | `GET /api/v1/admin/learning/experiments/{experiment_id}` | learning.v1 | LEM | Cần thêm |
| `updateLearningExperiment` | `PATCH /api/v1/admin/learning/experiments/{experiment_id}` (`If-Match`, chỉ `draft`) | learning.v1 | LEM | Cần thêm |
| `startLearningExperiment` | `POST /api/v1/admin/learning/experiments/{experiment_id}:start` (`{exposure: shadow|live}`) | learning.v1 | LEM · `Idempotency-Key` | Cần thêm |
| `stopLearningExperiment` | `POST /api/v1/admin/learning/experiments/{experiment_id}:stop` (`reason` 10–1.000) | learning.v1 | LEM · `Idempotency-Key` | Cần thêm |
| `getLearningExperimentResults` | `GET /api/v1/admin/learning/experiments/{experiment_id}/results` | learning.v1 | LEM | Cần thêm |
| `createPlatformReport` | `POST /api/v1/admin/reports` (`report_type = efficacy_dataset`, `params.study_id`) | learning.v1 (`ReportRequest`) | `report.export` + `platform.efficacy_study.export` | Cần sửa |
| `getPlatformReport` | `GET /api/v1/admin/reports/{report_id}` | learning.v1 (`ReportJob`) | người tạo | Có sẵn |
| `createQuestion`, `createQuestionVersion`, `updateQuestionDraft`, `getCmsQuestion`, `searchQuestions` | `/api/v1/cms/learning/questions*` | learning.v1 (`QuestionAuthoringInput`) | `learning_content.write`/`.read` | Cần sửa (`efficacy_instrument`, lọc) |
| `createCmsAssessmentBlueprint`, `createCmsAssessmentBlueprintVersion`, `updateCmsAssessmentBlueprintDraft`, `getCmsAssessmentBlueprint` | `/api/v1/cms/learning/assessment-blueprints*` | learning.v1 | `learning_content.write`/`.read` | Cần sửa (`efficacy_instrument`) |
| `createExamSession` | `POST /api/v1/exams/sessions` | learning.v1 | `lms.assignment.write` | Cần sửa (mô tả `422` blueprint đo hiệu quả) |
| `getExamSession`, `listExamSessions` | `/api/v1/exams/sessions*` | learning.v1 (`ExamSession`) | học sinh / `lms.assignment.read` | Cần sửa (`is_efficacy` chỉ cho nhân sự) |
| `releaseExamResults`, `getExamResult` | `/api/v1/exams/...` | learning.v1 | như F042 | Cần sửa (mô tả chặn) |
| `listExamSessionPapers`, `getExamAttemptAnalysis`, `getExamSessionReport` | `/api/v1/exams/...` | — (F042/F044 Cần thêm) | như F042/F044 | Mô tả chặn khi F042/F044 thêm |
| `getMyProgress`, `getMyMastery`, `getMyRecommendations`, `startAdHocAttempt` | `/api/v1/learning/*` | learning.v1 (F031/F032) | `game_player` | Có sẵn / F032 thêm — không đổi schema; hành vi theo arm (F035-AC-24) |

ProblemCode dùng: `PERMISSION_DENIED`, `MFA_REQUIRED`, `TENANT_HEADER_REQUIRED`, `TENANT_MISMATCH`, `NOT_FOUND`, `CONFLICT`, `INVALID_STATE_TRANSITION`, `VALIDATION_FAILED`, `VERSION_MISMATCH`, `IDEMPOTENCY_KEY_REQUIRED`, `EXAM_RESULTS_NOT_RELEASED`, `EXPORT_LIMIT_REACHED` (F044). Không thêm mã mới.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `createQuestionPlan` | game/competition → learning | `learning.internal.v1.yaml` | có sẵn | Có sẵn — hành vi: loại `efficacy_instrument` (F035-AC-02) |
| `applyTenantAssignments` | identity → learning | `common/internal-common.v1.yaml` (F020) | `identity` | Cần thêm ở F020 — mô tả `422` cho tài nguyên `efficacy_instrument` |
| `anonymizeUserData` | identity → learning | `learning.internal.v1.yaml` | `identity` | Có sẵn — hành vi: ẩn danh dữ liệu efficacy/experiment |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `audit.recorded` | learning phát / identity (audit) | envelope có sẵn | Có sẵn |
| `exam.started`, `exam.submitted`, `exam.graded`, `exam.results_released`, `exam.regraded`, `exam.opened`, `exam.closed` | **không phát** cho ExamSession đo hiệu quả | `specs/events/payloads/learning/` (F042) | Mô tả trong EVENT_DESIGN (Cần sửa) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/learning-event.schema.json` `$defs/context` | Thêm `efficacy_wave_id` (uuid), `experiment_arms` (mảng ≤ 10, mỗi phần tử `pattern ^[a-z0-9_-]{1,64}:[a-z0-9_]{1,32}$`) | Cần sửa |
| `specs/schemas/question.schema.json` | Thêm `efficacy_instrument` (boolean, mặc định false; mô tả ràng buộc `exam_only`, không `params`) | Cần sửa |
| `specs/schemas/assessment-blueprint.schema.json` | Thêm `efficacy_instrument` (boolean) | Cần sửa |

## 6. Delta cần thực hiện

- `learning.v1.yaml`: 24 operationId `/admin/learning/*` ở §1 với schema mới `ResearchAgreement(Create)`, `EfficacyStudy(Create|Update)`, `EfficacyStudyClass(Create)`, `EfficacyWave(Create)` (`kind`, `form`, `window_start`, `window_end`, `duration_seconds`), `EfficacyWithdrawalCreate` (`scope`, `school_id`, `class_id`, `student_ids` ≤ 500, `reason` 10–1000, `school_request_ref` ≤ 128), `EfficacyReport` (`status`, `descriptive_only`, `results`), `LearningExperiment(Create|Update)`, `ExperimentArm`, `ExperimentResults`; mọi string có `maxLength` (ADR-0041) — task T004.
- `ReportRequest.report_type` thêm `efficacy_dataset`; `params.study_id`; mô tả permission bổ sung — task T004.
- `QuestionAuthoringInput`, blueprint input/output thêm `efficacy_instrument`; `ExamSession` thêm `is_efficacy` (chỉ trả cho nhân sự); mô tả chặn ở `createExamSession`, `releaseExamResults`, `getExamResult` — task T004.
- `learning-event.schema.json`, `question.schema.json`, `assessment-blueprint.schema.json` như §5 — task T005.
- EVENT_DESIGN: ghi rõ ExamSession đo hiệu quả không phát `exam.*`; `experiment_arms`, `efficacy_wave_id` trong context — task T009.
- Permission catalog `pkg/authz/catalog.go` + [ADMIN_PORTAL §2.2](../../../docs/04-admin-portal/ADMIN_PORTAL.md) — task T006.
