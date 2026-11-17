# Tasks: F025 — Workflow duyệt nội dung, song ngữ, lồng tiếng `g1_3`

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `domain/workflow`, `domain/readiness`, `domain/i18ntext` và đăng ký vào composition root — file: `src/backend/modules/learning/cmd/api/main.go`
- [ ] T002 [P] Đăng ký permission dùng (`learning_content.write/review/translate`, `learning_content.activity.write/review`) kiểm tra với catalog lúc khởi động — file: `src/backend/modules/learning/internal/application/authz_requirements.go`
- [ ] T003 [P] Cấu hình bucket `ew-exports` và client S3 (env `EW_S3_*`) cho job dịch — file: `src/backend/modules/learning/internal/infrastructure/storage/export_bucket.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta OpenAPI: `revise`, `audio_exemption_acknowledged`, enum `lane` = `tenant`/`platform_learning`, `ProblemCode.AUDIO_REQUIRED`, response 409/428 cho `transitionContentVersion` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml`, `docs/07-backend/API_DESIGN.md` — kiểm: `make specs-lint`, `make gen MOD=learning`
- [ ] T005 [P] Payload schema `content.version_submitted/approved/rejected.v1`; `audio_exempt_reason` trong `question.schema.json`, `activity-base.schema.json` — file: `specs/events/payloads/learning/`, `specs/schemas/` — kiểm: `make specs-lint`
- [ ] T006 ent schema `ReviewTask`, `ContentTransition` (AppendOnly), `TranslationJob`; mixin `WorkflowFields` thêm vào mọi schema `*_version` — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f025_workflow`, `make migrate-lint MOD=learning`
- [ ] T007 Cập nhật DATABASE_DESIGN §2.5 (cột `review_tasks`, `content_transitions`, `translation_jobs`, cột version mới) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T008 Port `ContentVersionRepository` (adapter theo `content_type`) và `MediaStatusReader` — file: `src/backend/modules/learning/internal/application/port/content_version_repo.go`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Gửi duyệt, duyệt, từ chối nội dung (Priority: P1) 🎯 MVP

**Goal**: version đi `draft → in_review → approved/rejected → draft` đúng quy tắc.

**Independent Test**: như spec US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test bảng chuyển trạng thái + reviewer ≠ author cho F025-AC-01, F025-AC-04, F025-AC-05, F025-AC-07, F025-AC-08 — file: `src/backend/modules/learning/internal/domain/workflow/workflow_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Integration test submit_review thiếu schema/metadata và media chưa ready (F025-AC-02, F025-AC-03) — file: `src/backend/modules/learning/tests/integration/workflow/submit_review_test.go`
- [ ] T011 [P] [US1] Integration test reject cần note, audit + outbox + `content_transitions` cùng transaction (F025-AC-06, F025-AC-01) — file: `src/backend/modules/learning/tests/integration/workflow/reject_test.go`
- [ ] T012 [P] [US1] Integration test `If-Match`/`Idempotency-Key` (F025-AC-09) — file: `src/backend/modules/learning/tests/integration/workflow/idempotency_test.go`
- [ ] T013 [P] [US1] Test tenant isolation + authorization `transitionContentVersion` (tenant khác → 404, platform được gán → 403, thiếu permission → 403) cho F025-AC-10 — file: `src/backend/modules/learning/tests/integration/workflow/authz_tenant_test.go`
- [ ] T014 [P] [US1] Integration test transition không chạm attempt/evaluation/learning event (F025-AC-11) — file: `src/backend/modules/learning/tests/integration/workflow/history_protection_test.go`

### Implementation cho User Story 1

- [ ] T015 [P] [US1] Domain `ContentWorkflow.Apply` + lỗi domain (`ErrSelfReview`, `ErrInvalidTransition`) — file: `src/backend/modules/learning/internal/domain/workflow/workflow.go`
- [ ] T016 [P] [US1] Domain `readiness.Guard` với `SchemaRule`, `MediaRule` — file: `src/backend/modules/learning/internal/domain/readiness/guard.go`
- [ ] T017 [US1] Command `TransitionContentVersion` (permission theo action, outbox `content.version_*`, audit, review task) — file: `src/backend/modules/learning/internal/application/command/transition_content_version.go`
- [ ] T018 [US1] Repository ent workflow/review task + adapter version 7 loại — file: `src/backend/modules/learning/internal/infrastructure/persistence/workflow_repo.go`
- [ ] T019 [US1] Handler `transitionContentVersion` + ánh xạ lỗi Problem Details — file: `src/backend/modules/learning/internal/presentation/http/public/cms_workflow_handler.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Hàng đợi duyệt theo lane (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Integration test lọc lane/tenant/scope trường, bỏ qua `tenant_ids` với user tenant (F025-AC-12) — file: `src/backend/modules/learning/tests/integration/workflow/review_queue_test.go`
- [ ] T021 [P] [US2] Test `system_admin` `tenant_ids` + audit `cross_tenant.accessed`, `content_reviewer` chỉ lane platform (F025-AC-13) — file: `src/backend/modules/learning/tests/integration/workflow/review_queue_cross_tenant_test.go`
- [ ] T022 [P] [US2] Test authorization thiếu `learning_content.review` (F025-AC-14) — file: `src/backend/modules/learning/tests/integration/workflow/review_queue_authz_test.go`

### Implementation cho User Story 2

- [ ] T023 [US2] Query `ListContentReviewTasks` (read model, cursor) — file: `src/backend/modules/learning/internal/application/query/list_review_tasks.go`
- [ ] T024 [US2] Handler `listContentReviewTasks` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_workflow_handler.go`

## Phase 5: User Story 3 — Audio bắt buộc cho nội dung lớp 1–3 (Priority: P1)

### Tests cho User Story 3

- [ ] T025 [P] [US3] Unit test `AudioRule` (question/activity/lesson, môn Tiếng Anh, `grade_levels` giao {1,2,3}) cho F025-AC-15 — file: `src/backend/modules/learning/internal/domain/readiness/audio_rule_test.go` — phải FAIL trước khi code
- [ ] T026 [P] [US3] Integration test miễn trừ + `audio_exemption_acknowledged` (F025-AC-16) — file: `src/backend/modules/learning/tests/integration/workflow/audio_exemption_test.go`
- [ ] T027 [P] [US3] Contract test `PublishReadinessGuard` trả cùng mã khi gọi từ đường publish (F025-AC-17) — file: `src/backend/modules/learning/tests/contract/readiness_guard_test.go`

### Implementation cho User Story 3

- [ ] T028 [US3] `AudioRule` + ghi `audio_exempt_approved_by` khi approve — file: `src/backend/modules/learning/internal/domain/readiness/audio_rule.go`
- [ ] T029 [US3] Xuất `PublishReadinessGuard` qua port cho F026 — file: `src/backend/modules/learning/internal/application/port/readiness.go`

## Phase 6: User Story 4 — Ngôn ngữ nội dung và trạng thái bản dịch (Priority: P2)

### Tests cho User Story 4

- [ ] T030 [P] [US4] Unit test `LanguageRule` → `TRANSLATION_INCOMPLETE` (F025-AC-18) — file: `src/backend/modules/learning/internal/domain/readiness/language_rule_test.go`
- [ ] T031 [P] [US4] Unit test `ComputeStatus` complete/partial/outdated (F025-AC-19) — file: `src/backend/modules/learning/internal/domain/i18ntext/status_test.go`
- [ ] T032 [P] [US4] Unit test `Resolve` fallback + `Content-Language` (F025-AC-20) — file: `src/backend/modules/learning/internal/domain/i18ntext/resolve_test.go`
- [ ] T033 [P] [US4] Test NFC + `maxLength` → `422 VALIDATION_FAILED` (F025-AC-21) — file: `src/backend/modules/learning/tests/integration/workflow/text_limits_test.go`

### Implementation cho User Story 4

- [ ] T034 [US4] `LanguageRule`, `ComputeStatus`, `Resolve`, chuẩn hóa NFC — file: `src/backend/modules/learning/internal/domain/i18ntext/i18ntext.go`
- [ ] T035 [US4] Gắn `ComputeStatus` vào command lưu draft (hook chung) — file: `src/backend/modules/learning/internal/application/command/save_draft_hook.go`

## Phase 7: User Story 5 — Xuất/nhập file dịch (Priority: P3)

### Tests cho User Story 5

- [ ] T036 [P] [US5] Integration test export XLIFF/CSV + `download_url` (F025-AC-22) — file: `src/backend/modules/learning/tests/integration/translation/export_test.go`
- [ ] T037 [P] [US5] Integration test import tạo draft mới, published không đổi, báo khóa lỗi (F025-AC-23) — file: `src/backend/modules/learning/tests/integration/translation/import_test.go`
- [ ] T038 [P] [US5] Test authorization + tenant isolation export/import/get job (F025-AC-24) — file: `src/backend/modules/learning/tests/integration/translation/authz_tenant_test.go`

### Implementation cho User Story 5

- [ ] T039 [US5] Command `ExportLearningTranslations`, `ImportLearningTranslations`; query `GetLearningTranslationJob` — file: `src/backend/modules/learning/internal/application/command/export_translations.go`
- [ ] T040 [US5] Codec XLIFF 2.1/CSV (khóa `<content_type>/<id>/<version>/<pointer>`) — file: `src/backend/modules/learning/internal/infrastructure/translation/codec.go`
- [ ] T041 [US5] Job runner `translation-jobs` trong worker + handler 3 endpoint — file: `src/backend/modules/learning/internal/presentation/job/translation_job_runner.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T042 [P] Metric `eduworld_content_transitions_total`, `eduworld_review_tasks_open`, span `workflow.transition` — file: `src/backend/modules/learning/internal/infrastructure/observability/workflow_metrics.go`
- [ ] T043 [P] Đăng ký `content_review_sla_days` vào CONFIG_DEFAULTS + ADMIN_PORTAL §4; ghi CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`
- [ ] T044 [P] Đồng bộ tên event `content.version_*` trong CONTENT_MODEL §6 (D11) — file: `docs/02-learning/CONTENT_MODEL.md`
- [ ] T045 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/025-content-review-workflow/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → (US2, US3 song song) → US4 → US5 → Polish.
- US3 dùng `readiness.Guard` từ T016; F026 dùng T029. T008 cần bảng `content.media_assets` (F027) cho `MediaRule`.

## Definition of Done

- [ ] Mọi tiêu chí F025-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `learning`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
