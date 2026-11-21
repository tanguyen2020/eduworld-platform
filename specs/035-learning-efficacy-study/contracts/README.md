# Contracts: F035 — Nghiên cứu hiệu quả học tập

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`, `x-service: learning`). Route `/admin/learning/*`: `security: keycloakAdminBearer`, `x-internal: true` (gateway nội bộ, MFA). Permission (catalog [ADMIN_PORTAL §2.2](../../../docs/04-admin-portal/ADMIN_PORTAL.md), platform-only, System Admin): `platform.efficacy_study.manage` (viết tắt **ESM**), `platform.efficacy_study.export`, `platform.learning_experiment.manage` (**LEM**). Mọi operation `/admin/learning/*` có `x-portal-action` thuộc menu `menu.platform_admin.efficacy_studies` (action `view`, `manage_agreements`, `edit`, `schedule_waves`, `withdraw`, `report`) hoặc `menu.platform_admin.learning_experiments` (action `view`, `edit`, `operate`) trong [`specs/portal/menu-catalog.yaml`](../../portal/menu-catalog.yaml).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createResearchAgreement` | `POST /api/v1/admin/learning/research-agreements` (`X-Tenant-Id` đích) | learning.v1 | ESM · `Idempotency-Key` | Có sẵn |
| `listResearchAgreements` | `GET /api/v1/admin/learning/research-agreements?tenant_ids=&status=` | learning.v1 | ESM | Có sẵn |
| `revokeResearchAgreement` | `POST /api/v1/admin/learning/research-agreements/{agreement_id}:revoke` (`X-Tenant-Id`) | learning.v1 | ESM · `Idempotency-Key` | Có sẵn |
| `createEfficacyStudy` | `POST /api/v1/admin/learning/efficacy-studies` | learning.v1 | ESM · `Idempotency-Key` | Có sẵn |
| `listEfficacyStudies` | `GET /api/v1/admin/learning/efficacy-studies?status=&tenant_ids=` | learning.v1 | ESM | Có sẵn |
| `getEfficacyStudy` | `GET /api/v1/admin/learning/efficacy-studies/{study_id}` | learning.v1 | ESM | Có sẵn |
| `updateEfficacyStudy` | `PATCH /api/v1/admin/learning/efficacy-studies/{study_id}` (`If-Match`, chỉ `draft`) | learning.v1 | ESM | Có sẵn |
| `addEfficacyStudyCohort` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}/cohorts` (`X-Tenant-Id`, body `school_id`) | learning.v1 | ESM · `Idempotency-Key` | ADR-0095 (WP-L2): thay `addEfficacyStudyClass` |
| `listEfficacyStudyCohorts` | `GET /api/v1/admin/learning/efficacy-studies/{study_id}/cohorts?tenant_ids=` | learning.v1 | ESM | ADR-0095: thay `listEfficacyStudyClasses` (item có `school_id`, `grade_level`, `conflict`) |
| `removeEfficacyStudyCohort` | `DELETE /api/v1/admin/learning/efficacy-studies/{study_id}/cohorts/{school_id}` (ADR-0095, thay removeEfficacyStudyClass) (`X-Tenant-Id`, soft-delete, chỉ `draft`) | learning.v1 | ESM | Có sẵn |
| `activateEfficacyStudy` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}:activate` | learning.v1 | ESM · `Idempotency-Key` | Có sẵn |
| `scheduleEfficacyWave` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}/waves` | learning.v1 | ESM · `Idempotency-Key` | Có sẵn |
| `listEfficacyWaves` | `GET /api/v1/admin/learning/efficacy-studies/{study_id}/waves` (số mời/đã làm/vắng theo lớp, không điểm) | learning.v1 | ESM | Có sẵn |
| `withdrawEfficacyParticipants` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}:withdraw` (`X-Tenant-Id`) | learning.v1 | ESM · `Idempotency-Key` | Có sẵn |
| `completeEfficacyStudy` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}:complete` | learning.v1 | ESM · `Idempotency-Key` | Có sẵn |
| `createEfficacyStudyReport` | `POST /api/v1/admin/learning/efficacy-studies/{study_id}/reports` → 202 | learning.v1 | ESM · `Idempotency-Key` | Có sẵn |
| `getEfficacyStudyReport` | `GET /api/v1/admin/learning/efficacy-studies/{study_id}/reports/{report_id}` | learning.v1 | ESM | Có sẵn |
| `createLearningExperiment` | `POST /api/v1/admin/learning/experiments` | learning.v1 | LEM · `Idempotency-Key` | Có sẵn |
| `listLearningExperiments` | `GET /api/v1/admin/learning/experiments?status=&kind=` | learning.v1 | LEM | Có sẵn |
| `getLearningExperiment` | `GET /api/v1/admin/learning/experiments/{experiment_id}` | learning.v1 | LEM | Có sẵn |
| `updateLearningExperiment` | `PATCH /api/v1/admin/learning/experiments/{experiment_id}` (`If-Match`, chỉ `draft`) | learning.v1 | LEM | Có sẵn |
| `startLearningExperiment` | `POST /api/v1/admin/learning/experiments/{experiment_id}:start` (`{exposure: shadow|live}`) | learning.v1 | LEM · `Idempotency-Key` | Có sẵn |
| `stopLearningExperiment` | `POST /api/v1/admin/learning/experiments/{experiment_id}:stop` (`reason` 10–1.000) | learning.v1 | LEM · `Idempotency-Key` | Có sẵn |
| `getLearningExperimentResults` | `GET /api/v1/admin/learning/experiments/{experiment_id}/results` | learning.v1 | LEM | Có sẵn |
| `createPlatformReport` | `POST /api/v1/admin/reports` (`report_type = efficacy_dataset`, `params.study_id`) | learning.v1 (`ReportRequest`) | `report.export` + `platform.efficacy_study.export` | Có sẵn (`report_type = efficacy_dataset`, `params.study_id`) |
| `getPlatformReport` | `GET /api/v1/admin/reports/{report_id}` | learning.v1 (`ReportJob`) | người tạo | Có sẵn |
| `createQuestion`, `createQuestionVersion`, `updateQuestionDraft`, `getCmsQuestion`, `searchQuestions` | `/api/v1/cms/learning/questions*` | learning.v1 (`QuestionAuthoringInput`) | `learning_content.write`/`.read` | Có sẵn (`QuestionAuthoringInput.efficacy_instrument`, `exam_only`; lọc `efficacy_instrument` ở `searchQuestions`) |
| `createCmsAssessmentBlueprint`, `createCmsAssessmentBlueprintVersion`, `updateCmsAssessmentBlueprintDraft`, `getCmsAssessmentBlueprint` | `/api/v1/cms/learning/assessment-blueprints*` | learning.v1 | `learning_content.write`/`.read` | Có sẵn (`efficacy_instrument`) |
| `createExamSession` | `POST /api/v1/exams/sessions` | learning.v1 | `lms.assignment.write` | Có sẵn (mô tả `422` blueprint đo hiệu quả) |
| `getExamSession`, `listExamSessions` | `/api/v1/exams/sessions*` | learning.v1 (`ExamSession`) | học sinh / `lms.assignment.read` | Có sẵn (`is_efficacy` chỉ cho nhân sự) |
| `releaseExamResults`, `getExamResult` | `/api/v1/exams/...` | learning.v1 | như F042 | Có sẵn (mô tả chặn; `releaseExamResults` thêm `409`) |
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
| `specs/schemas/learning-event.schema.json` `$defs/context` | Thêm `efficacy_wave_id` (uuid), `experiment_arms` (mảng ≤ 10, mỗi phần tử `pattern ^[a-z0-9_-]{1,64}:[a-z0-9_]{1,32}$`) | Có sẵn |
| `specs/schemas/question.schema.json` | Thêm `efficacy_instrument` (boolean, mặc định false; mô tả ràng buộc `exam_only`, không `params`) | Có sẵn |
| `specs/schemas/assessment-blueprint.schema.json` | Thêm `efficacy_instrument` (boolean) | Có sẵn |

## 6. Delta cần thực hiện

Đã thực hiện trong contract (task T004, T005): 24 operationId `/admin/learning/*` ở §1 với schema `ResearchAgreement(Create|Revoke|Page)`, `EfficacyStudy(Create|Update|Page)`, `EfficacyStudyClass(Create|Page)`, `EfficacyWave(Create|List)`, `EfficacyWithdrawal(Create)`, `EfficacyReport(Create)`, `LearningExperiment(Create|Update|Start|Stop|Page)`, `ExperimentArm`, `ExperimentResults` (mọi string có `maxLength`, ADR-0041); `ReportRequest.report_type = efficacy_dataset` + `params.study_id`; `efficacy_instrument` trong `QuestionAuthoringInput`/`AssessmentBlueprintAuthoringInput` và bộ lọc `searchQuestions`; `ExamSession.is_efficacy` và mô tả chặn ở `createExamSession`, `releaseExamResults` (`409`), `getExamResult`; JSON Schema §5; permission ở [ADMIN_PORTAL §2.2](../../../docs/04-admin-portal/ADMIN_PORTAL.md); menu/action ở `specs/portal/menu-catalog.yaml`; endpoint ở [API_DESIGN §9.16](../../../docs/07-backend/API_DESIGN.md#916-admin-internal--apiv1admin).

Còn lại:

- EVENT_DESIGN: ghi rõ ExamSession đo hiệu quả không phát `exam.*`; `experiment_arms`, `efficacy_wave_id` trong context — task T009.
- Permission catalog code `src/backend/pkg/authz/catalog.go` + seed role `system_admin` — task T006.
- `applyTenantAssignments` (F020) mô tả `422` cho tài nguyên `efficacy_instrument`; `listExamSessionPapers`, `getExamAttemptAnalysis`, `getExamSessionReport` mô tả chặn khi F042/F044 thêm operation.
