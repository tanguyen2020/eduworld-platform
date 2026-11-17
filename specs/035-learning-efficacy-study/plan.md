# Implementation Plan: F035 — Nghiên cứu hiệu quả học tập

**Branch**: `035-learning-efficacy-study` · **Date**: 2026-11-17 · **Spec**: [spec.md](spec.md)

## Summary

Hai module mới trong module `learning`: `efficacy` (thỏa thuận, study, lớp/cặp, đợt, participant, rút, snapshot, báo cáo, xuất giả danh) và `experiment` (A/B `mastery_model`, `recommendation_rule`). Cờ `efficacy_instrument` trên question/blueprint version được chặn ở một guard domain dùng chung cho mọi đường chọn/tham chiếu câu. Đợt kiểm tra gọi command `CreateExamSession` của F042 với cài đặt cố định và `efficacy_wave_id`; F042 bỏ outbox `exam.*` cho các session này nên không có evidence mastery/XP/dashboard. Consumer mastery F031 dùng `ModelResolver` để tính thêm trạng thái theo arm; đọc phía trải nghiệm học sinh qua `ExperienceMasteryReader`. Báo cáo tính bằng domain thuần `efficacy/stats` ([research.md](research.md)); xuất dùng hạ tầng `report_jobs` của F044 với `report_type = efficacy_dataset`. Portal: trang workspace D và ô cờ trong Content Studio.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 / TypeScript (React, Vite) |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go, `src/backend/pkg/{authz,tenancy,entx,outbox,idempotency,problem,clock,ids,config}`; Ant Design Pro, TanStack Query, `ops-ui` |
| Storage | PostgreSQL `eduworld_learning` (`learning.*`, `content.*`, `assessment.exam_sessions`); object storage `ew-exports` (F044) |
| Testing | go test (golden stats), testcontainers (PostgreSQL, NATS); Vitest + Playwright |
| Target Platform | Linux container (module `learning` trong `core-api`, `worker-jobs`); trình duyệt |
| Project Type | backend module + web app |
| Performance Goals | Báo cáo một study ≤ 2.000 participant chạy nền ≤ 5 phút [Proposed]; consumer mastery thêm ≤ 20% thời gian xử lý khi có arm shadow (SC-002 của F031) |
| Constraints | R5, R6, R8, R10, R13, R14, R16, R19, R20, R21, R26, R27, R28; ADR-0016, ADR-0017, ADR-0027, ADR-0043, ADR-0044, ADR-0062, ADR-0070, ADR-0080, ADR-0087 |
| Scale/Scope | Pilot 6–8 lớp mỗi nhóm (~200 học sinh/nhóm, [LEARNING_EFFICACY_PLAN §4](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md)) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta OpenAPI, learning event `context`, `ReportRequest` ở Phase 2 (T004–T006); bổ sung ADR-0080 phần A/B trước US5 (T008); không chạm nội dung deferred (consent) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Mọi dữ liệu nguồn trong `eduworld_learning`; không gọi module khác; command ở `application/command/{efficacy,experiment}` + `UnitOfWork` + outbox `audit.recorded`; query ở `application/query/...` chỉ đọc `port/read` (replica); không mediator (ADR-0087) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng T có `tenant_id` + RLS + `deleted_at`; bảng G platform; liên tenant chỉ `CrossTenantScope` của System Admin; rút = soft-delete dữ liệu dẫn xuất |
| IV Server-authoritative | ✅ | Chấm bởi grader F042; không đáp án/kết quả xuống học sinh; thao tác ghi có `Idempotency-Key` + audit |
| V Xác thực, phân quyền | ✅ | `platform.efficacy_study.manage`/`.export`, `platform.learning_experiment.manage` (platform-only, MFA, gateway nội bộ); `authz.Require` đầu mỗi handler |
| VI An toàn trẻ em | ✅ | Không xếp hạng, không công bố điểm, không XP theo điểm; mã giả danh; ô nhỏ bị ẩn; rào chắn R19 cho A/B |
| VII Một nguồn sự thật, cấu hình | ✅ | Ngưỡng từ CONFIG_DEFAULTS (tham số mới đăng ký ở T007); không FR tự tạo; mọi chuỗi có `MaxLen`/`maxLength` |
| VIII UI tách logic, token | ✅ | Portal chỉ gọi API, theme `ops-ui` |
| IX Test có bằng chứng | ✅ | Mỗi AC có test viết trước; golden stats; tenant isolation + authorization cho mọi endpoint mới |

## Project Structure

### Documentation (feature này)

```text
specs/035-learning-efficacy-study/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/modules/learning/
├── ent/schema/{research_agreement,efficacy_study,efficacy_study_class,efficacy_participant,efficacy_wave,efficacy_wave_session,efficacy_withdrawal,efficacy_mastery_snapshot,efficacy_report,learning_experiment,experiment_assignment,experiment_mastery_state,experiment_result}.go
├── ent/schema/{question_version,assessment_blueprint_version,exam_session}.go      # thêm cột efficacy_instrument / efficacy_wave_id
├── internal/domain/question/efficacy_guard.go       # chặn tham chiếu/chọn câu efficacy_instrument
├── internal/domain/efficacy/                        # study.go, pairing.go, wave.go, participant.go, withdrawal.go, pseudonym.go
├── internal/domain/efficacy/stats/                  # gain.go, hedges.go, icc.go, spearman.go, auc.go, calibration.go
├── internal/domain/experiment/                      # experiment.go, assignment.go, arm_params.go, guardrail.go
├── internal/application/command/efficacy/           # create_research_agreement.go, revoke_research_agreement.go, create_study.go, update_study.go, add_study_class.go, remove_study_class.go, activate_study.go, schedule_wave.go, open_wave.go, withdraw_participants.go, complete_study.go, generate_report.go, export_dataset.go
├── internal/application/command/experiment/         # create_experiment.go, update_experiment.go, start_experiment.go, stop_experiment.go, run_guardrail.go, compute_results.go
├── internal/application/query/efficacy/             # list_agreements.go, list_studies.go, get_study.go, list_study_classes.go, list_waves.go, get_report.go
├── internal/application/query/experiment/           # list_experiments.go, get_experiment.go, get_results.go
├── internal/application/port/write/                 # EfficacyStudyRepo, ExperimentRepo, ExamSessionCreator (F042 command), SessionCanceller (F041)
├── internal/application/port/read/                  # EfficacyReadModel, ExperimentReadModel, ExperienceMasteryReader, ArmResolver
├── internal/infrastructure/persistence/write/       # ent repo efficacy_*, experiment_*
├── internal/infrastructure/persistence/read/        # SQL tường minh (TenantScope/CrossTenantScope + deleted_at)
├── internal/infrastructure/secret/pseudonym_key.go  # đọc EW_EFFICACY_PSEUDONYM_KEY_FILE
├── internal/presentation/http/public/admin/         # efficacy_handler.go, experiment_handler.go (/admin/learning/*)
├── internal/presentation/job/                       # efficacy_wave_monitor.go, experiment_guardrail.go, experiment_lifecycle.go
└── tests/{unit,integration,contract}/efficacy|experiment/
src/cms/frontend/apps/portal/src/workspaces/platform-admin/research/
├── routes.tsx                                       # /portal/platform-admin/research/{agreements,studies,studies/:id,experiments,experiments/:id}
├── api/{agreements,studies,waves,reports,experiments}.ts
├── agreements/AgreementListPage.tsx · studies/{StudyListPage,StudyForm,StudyDetailPage,ClassPairTable,WaveScheduler,WithdrawDialog,ReportView}.tsx
├── experiments/{ExperimentListPage,ExperimentForm,ExperimentDetailPage}.tsx
└── i18n/{vi,en}.json · __tests__/
src/cms/frontend/apps/portal/src/workspaces/content-studio/…/EfficacyInstrumentField.tsx      # ô cờ trong editor câu/blueprint (F053/F054)
```

**Structure Decision**: Cùng module với exam, mastery, gợi ý và report job nên không cần facade (ADR-0074); `efficacy/stats` thuần để golden test không cần DB; arm resolution là port đọc để F031/F032 dùng mà không phụ thuộc module `experiment`.

## Phase 0: Research

[research.md](research.md): R1 chặn evidence tại nguồn, R2 công thức thống kê, R3 mã giả danh, R4 phân arm ổn định.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `question.EfficacyGuard`; `efficacy.Study` (trạng thái, kiểm phủ LO/số câu), `efficacy.Pairing` (random theo hash, `form_order` xen kẽ), `efficacy.Wave` (thứ tự pre/post, khoảng cách tuần), `efficacy.Participant`, `efficacy.Pseudonymizer`; `stats.*`; `experiment.Experiment` (arm, allocation, trạng thái), `experiment.Assign`, `experiment.Guardrail` | Ngưỡng truyền vào từ config, không đọc config trong domain |
| application | Command: `CreateResearchAgreement`, `RevokeResearchAgreement`, `CreateEfficacyStudy`, `UpdateEfficacyStudy`, `AddEfficacyStudyClass`, `RemoveEfficacyStudyClass`, `ActivateEfficacyStudy`, `ScheduleEfficacyWave`, `OpenEfficacyWave` (chụp participant / `M_eff`), `WithdrawEfficacyParticipants`, `CompleteEfficacyStudy`, `GenerateEfficacyReport`, `ExportEfficacyDataset` (handler `report_type` của F044), `CreateLearningExperiment`, `UpdateLearningExperiment`, `StartLearningExperiment`, `StopLearningExperiment`, `RunExperimentGuardrail`, `ComputeExperimentResults` · Query: `ListResearchAgreements`, `ListEfficacyStudies`, `GetEfficacyStudy`, `ListEfficacyStudyClasses`, `ListEfficacyWaves`, `GetEfficacyStudyReport`, `ListLearningExperiments`, `GetLearningExperiment`, `GetLearningExperimentResults` | `authz.Require(ctx, p, "platform.efficacy_study.manage" / "platform.efficacy_study.export" / "platform.learning_experiment.manage", resource)` |
| infrastructure | ent repo; SQL đọc replica; `ExamSessionCreator` gọi trực tiếp command F042 trong cùng process; `PseudonymKeyProvider`; outbox `audit.recorded` | Không HTTP nội bộ |
| presentation | Handler 24 operationId mới (contracts §1) dưới `/api/v1/admin/learning/*`; job `efficacy-wave-monitor` (mỗi phút: mở/đóng đợt, chụp), `experiment-guardrail` (hằng ngày), `experiment-lifecycle` (`end_at`) | `keycloakAdminBearer`, `x-internal: true` |

### Tích hợp và phụ thuộc

- **F022/F024/F030/F032**: `EfficacyGuard` gắn vào `selection.Select`, lưu question set/lesson/activity/blueprint; `createQuestionPlan` (internal) dùng cùng `Select`.
- **F026/F020**: snapshot CDN coi `efficacy_instrument` như `exam_only`; `applyTenantAssignments` từ chối.
- **F042**: `CreateExamSession` nhận `EfficacySettings` (chỉ F035 gọi); `GradeExamAttempt`, `TransitionExamWindows`, `StartExamAttempt`, `SubmitExamAttempt` bỏ outbox `exam.*` khi `efficacy_wave_id` khác NULL; `ReleaseExamResults`, `ListExamSessionPapers`, `GetExamResult`, `GetExamAttemptAnalysis` kiểm chặn; R5 không đăng ký. `ExcludeExamSessionStudents` (mới, nội bộ) cho session `scheduled`.
- **F041**: `CancelExamSession` khi rút cả lớp/tenant.
- **F031**: `ApplyMasteryEvidence` bỏ evidence có `efficacy_wave_id`; thêm `ModelResolver` → ghi `experiment_mastery_states` cho arm (shadow/live); recompute bỏ event đo hiệu quả.
- **F032**: rule engine và `selection.Policy` đọc `ExperienceMasteryReader` + `ArmResolver` (tham số arm `recommendation_rule`); learning event thêm `experiment_arms`.
- **F044**: `examquality.KR20`; `report_jobs` + worker thêm handler `efficacy_dataset`; `getExamSessionReport` chặn session đo hiệu quả.
- **F016**: `anonymizeUserData` (internal) ẩn danh/soft-delete dữ liệu efficacy, experiment của học sinh.
- Event tiêu thụ mới: không. Event phát: `audit.recorded`. Event `exam.*` bị bỏ cho session đo hiệu quả (R1 research).

### Bảo mật, tenant, idempotency, audit

- Route `/api/v1/admin/learning/*`: gateway nội bộ, `azp = eduworld-portal`, MFA, role platform; ghi dữ liệu tenant (agreement, class, withdrawal) bắt buộc `X-Tenant-Id` tenant đích; list dùng `tenant_ids[]` + `CrossTenantScope` + audit `cross_tenant.accessed`; ghi platform (study, wave, experiment) không header.
- `Idempotency-Key` cho mọi POST tạo/chuyển trạng thái.
- Audit: `research_agreement.created|revoked`, `efficacy_study.created|updated|class_added|class_removed|activated|completed`, `efficacy_wave.scheduled|opened|closed`, `efficacy.participants_withdrawn`, `efficacy_report.generated`, `efficacy.dataset_exported`, `learning_experiment.created|updated|started|stopped`.
- Job `efficacy-wave-monitor` tạo/đọc dữ liệu nhiều tenant qua `tenancy.Bypass(ctx, "efficacy_study")` (reason trong danh sách đóng ADR-0089 §3, client `learning_platform`) có metric `eduworld_tenant_filter_bypass_total{module,reason}` và audit `cross_tenant.accessed` theo tenant.

### Quan sát

- Metric: `eduworld_efficacy_instrument_blocked_total{path}`, `eduworld_efficacy_wave_sessions_created_total`, `eduworld_efficacy_participants{status}`, `eduworld_efficacy_report_duration_seconds`, `eduworld_experiment_assignments_total{kind}`, `eduworld_experiment_guardrail_breaches_total`, `eduworld_experiment_shadow_updates_total`.
- Span: `efficacy.schedule_wave`, `efficacy.open_wave`, `efficacy.report`, `experiment.assign`, `mastery.apply_arm`.
- Log không chứa `student_id` ở INFO, không chứa khóa giả danh hay câu trả lời.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.platform_admin.efficacy_studies` (action `view`, `manage_agreements`, `edit`, `schedule_waves`, `withdraw`, `report`), `menu.platform_admin.learning_experiments` (action `view`, `edit`, `operate`) |
| Action dùng | `action.content_studio.question_bank.view`, `action.content_studio.question_bank.edit`, `action.content_studio.assessments.view`, `action.content_studio.assessments.edit`, `action.platform_admin.reports.export`, `action.school_lms.assignments.view`, `action.school_lms.assignments.edit`, `action.school_lms.assignments.release_results` |

Khoảng trống catalog/contract — đã đóng:

- ~~Catalog chưa có menu cho màn quản trị nghiên cứu ở workspace D~~ — đã thêm `menu.platform_admin.efficacy_studies` (`/platform-admin/efficacy-studies`, permission `platform.efficacy_study.manage`) và `menu.platform_admin.learning_experiments` (`/platform-admin/learning-experiments`, permission `platform.learning_experiment.manage`), `can_disable: true`; 24 operation `/admin/learning/*` có `x-portal-action` (`createPlatformReport` `efficacy_dataset` vẫn thuộc `action.platform_admin.reports.export`)
- ~~Operation `/exams/*` thiếu `x-portal-action`~~ — đã đóng: `listExamSessions`, `getExamSession` → `action.school_lms.assignments.view`; `createExamSession` → `action.school_lms.assignments.edit`; `releaseExamResults` → `action.school_lms.assignments.release_results`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
