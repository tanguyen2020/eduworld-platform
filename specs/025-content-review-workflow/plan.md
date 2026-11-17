# Implementation Plan: F025 — Workflow duyệt nội dung, song ngữ, lồng tiếng `g1_3`

**Branch**: `025-content-review-workflow` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm aggregate `ContentWorkflow` dùng chung cho mọi `LearningContentType` trong module `learning`: state machine `draft → in_review → approved` (+ `rejected`, `revise`), review task theo lane, nhật ký `content_transitions`, audit và outbox. Bộ `PublishReadinessGuard` (schema, media `ready`, audio `g1_3`, bản `content_language`) chạy khi `submit_review` và được F026 gọi lại khi publish. `translation_status` tính khi lưu draft bằng `translation_meta` (hash bản gốc theo JSON pointer). Job xuất/nhập XLIFF/CSV chạy trong `learning-worker`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go, `pkg/authz`, `pkg/tenancy`, `pkg/entx`, `pkg/outbox`, `pkg/problem`, AWS SDK v2 (S3) |
| Storage | PostgreSQL `eduworld_learning` (schema `content`); floci S3 bucket `ew-exports` |
| Testing | go test + testcontainers (PostgreSQL, NATS) |
| Target Platform | Linux container (module `learning` trong `core-api`, `worker-events`) |
| Project Type | backend module |
| Performance Goals | Transition p95 < 300 ms (NFR chung API CMS, [API_DESIGN](../../docs/07-backend/API_DESIGN.md)) |
| Constraints | R7, R8, R10, R14, R26–R28; ADR-0043, ADR-0056, ADR-0072 |
| Scale/Scope | Hàng đợi duyệt ≤ vài nghìn task/tenant; export ≤ 500 item/job (`TranslationExportRequest.content_ids`) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta contract (action `revise`, lane enum, `AUDIO_REQUIRED`, event payload) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ trong module `learning`; command (write context) / query (read context) tách (ADR-0087); không gọi module khác |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới có `tenant_id` (NULL = platform) + `deleted_at`; `content_transitions` append-only; test TC-TEN-001 |
| IV Server-authoritative | ✅ | Không chạm attempt/điểm (FR-CMS-007); transition có Idempotency-Key + audit |
| V Xác thực, phân quyền | ✅ | Permission theo action; `portal_staff`; role tenant không duyệt dữ liệu platform |
| VI An toàn trẻ em | ✅ | Không có dữ liệu học sinh; không log nội dung câu hỏi |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc link CONTENT_MODEL §6, §8; tham số theo tên CONFIG_DEFAULTS; chuỗi có MaxLen |
| VIII UI tách logic, token | ✅ | Không có UI (portal ở F053/F054) |
| IX Test có bằng chứng | ✅ | Mỗi F025-AC có test viết trước; test tenant + authz cho mọi endpoint |

## Project Structure

### Documentation (feature này)

```text
specs/025-content-review-workflow/
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
modules/learning/
├── ent/schema/{review_task.go,content_transition.go,translation_job.go}
├── ent/schema/mixin/workflow_fields.go          # translation_status, translation_meta, audio_exempt_*
├── internal/domain/workflow/                     # ContentWorkflow, Transition, ReviewLane, errors
├── internal/domain/readiness/                    # PublishReadinessGuard: AudioRule, LanguageRule, MediaRule, SchemaRule
├── internal/domain/i18ntext/                     # TranslationStatus, Resolve(locale) fallback, NFC
├── internal/application/command/{transition_content_version.go,export_translations.go,import_translations.go}
├── internal/application/query/{list_review_tasks.go,get_translation_job.go}
├── internal/application/port/{content_version_repo.go,media_status_reader.go,export_storage.go}
├── internal/infrastructure/persistence/{workflow_repo.go,review_task_repo.go}
├── internal/infrastructure/storage/export_bucket.go
├── internal/presentation/http/public/{cms_workflow_handler.go,cms_translation_handler.go}
├── internal/presentation/job/translation_job_runner.go
└── tests/{unit,integration,contract}/workflow/
```

**Structure Decision**: Workflow là domain riêng (`domain/workflow`) vì áp cho mọi loại nội dung; repository version chung qua port `ContentVersionRepository` có adapter theo `content_type` (question, question_set, assessment_blueprint, lesson, learning_material, learning_path, activity).

## Phase 0: Research

Không có — quy tắc đủ trong CONTENT_MODEL §6, §8, §8.1, ADMIN_PORTAL §1.5, §4 và ADR-0043, ADR-0056.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `workflow.ContentWorkflow` (Apply(action, actor) → event), `readiness.Guard` (rule chain trả `[]Violation{code, pointer}`), `i18ntext.ComputeStatus`, `i18ntext.Resolve` | Bảng chuyển trạng thái = CONTENT_MODEL §6; reviewer ≠ author trong domain |
| application | Command `TransitionContentVersion`, `ExportLearningTranslations`, `ImportLearningTranslations` · Query `ListContentReviewTasks`, `GetLearningTranslationJob` | Permission: `submit_review`/`revise` → `learning_content.write` (activity `.activity.write`); `approve`/`reject` → `learning_content.review` (activity `.activity.review`); translate → `learning_content.translate`; queue → `learning_content.review` |
| infrastructure | ent repo version theo loại, `ReviewTaskRepo`, `MediaStatusReader` (đọc `content.media_assets`), `ExportBucket` (S3 `ew-exports`, presign GET), outbox writer | |
| presentation | `transitionContentVersion`, `listContentReviewTasks`, `exportLearningTranslations`, `importLearningTranslations`, `getLearningTranslationJob`; job `translation-jobs` | Hook lưu draft (F022/F024) gọi `i18ntext.ComputeStatus` |

### Tích hợp và phụ thuộc

- Phụ thuộc: F022 (question version + validator), F020 (owner_scope, assignment); đọc media status từ bảng của F027.
- Event phát: `content.version_submitted`, `content.version_approved`, `content.version_rejected`, `audit.recorded` (outbox). Không tiêu thụ event.
- Không gọi facade của module khác.

### Bảo mật, tenant, idempotency, audit

- Middleware `portal_staff` (azp `eduworld-portal`, `user_type = staff`); `TenantScope` qua middleware `pkg/httpx` + interceptor ent; dữ liệu platform chỉ role platform ghi; System Admin ghi phải có `X-Tenant-Id` đích.
- `Idempotency-Key` + `If-Match` (ETag = version ent) cho transition; job export/import có Idempotency-Key.
- Audit action: `content.submitted`, `content.approved`, `content.rejected`, `content.version_created` (import), `authz.denied`, `cross_tenant.accessed`.

### Quan sát

- Log `slog` có `tenant_id`, `content_type`, `content_id`, `version`, `action` (không nội dung).
- Metric `eduworld_content_transitions_total{content_type,action,result}`, `eduworld_review_tasks_open{lane}`; span `workflow.transition`, `readiness.check`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
