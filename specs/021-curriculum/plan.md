# Implementation Plan: F021 — Curriculum: cấu trúc, LO, DAG tiên quyết, version, mapping, import/export, seed GDPT

**Branch**: `021-curriculum` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `curriculum` trong module `learning` (schema PostgreSQL `curriculum`): aggregate `Curriculum` với `CurriculumVersion` là gốc bất biến sau publish, chứa SubjectGrade, Topic, LearningObjective, Competency. Dùng `contentmodel.Lifecycle` và `content.content_releases` của F020; publish phát `curriculum.version_published` qua outbox. Kiểm DAG tiên quyết bằng DFS trong domain. Import/export qua khung job `content.import_jobs` (F021 dựng khung, F024 thêm loại câu hỏi/bài học). Seed GDPT chạy bằng `cmd/seed` gọi cùng command import. Đăng ký `ResourceHandler` `curriculum` cho Thùng rác/gán/clone của F020.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent + Atlas, oapi-codegen, `gopkg.in/yaml.v3`, `encoding/csv`, `pkg/{entx,tenancy,authz,outbox,httpx,problem}` |
| Storage | PostgreSQL `eduworld_learning` schema `curriculum`, `content` (import_jobs, releases); Valkey cache read model cây `ew:learning:cur:tree:<curriculum_version_id>:<subject_id>:<grade>` (bất biến theo version published) |
| Testing | go test (unit domain), testcontainers (integration), API test |
| Target Platform | Linux container |
| Project Type | backend module |
| Performance Goals | Đọc cây từ cache; p95 theo [SYSTEM_ARCHITECTURE §7](../../docs/06-architecture/SYSTEM_ARCHITECTURE.md#7-non-functional-baseline) |
| Constraints | ADR-0015, ADR-0028, ADR-0029, ADR-0041, ADR-0043; AGENTS R8, R10, R11, R21, R26 |
| Scale/Scope | `csv_import_max`, `request_body_max` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta OpenAPI (endpoint version/workflow/export/mapping/textbook/competency, sửa `Curriculum`) + payload `curriculum.version_published` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Cấu hình trường đọc từ projection `ref.schools`, không gọi identity đồng bộ |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng loại H/G có `deleted_at`; RLS; soft-delete; test isolation |
| IV Server-authoritative | ✅ | Version published bất biến (trigger + use case); không có điểm |
| V Xác thực, phân quyền | ✅ | `curriculum.read/write/publish`, môn học chỉ role platform |
| VI An toàn trẻ em | ✅ | Không có dữ liệu học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc mã LO/`yccd_ref` link CURRICULUM_MODEL; tham số theo tên |
| VIII UI tách logic, token | ✅ | Không có UI (F053) |
| IX Test có bằng chứng | ✅ | Mỗi F021-AC có task test |

## Project Structure

### Documentation (feature này)

```text
specs/021-curriculum/
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
├── ent/schema/curriculum.go curriculum_version.go subject.go subject_grade.go topic.go learning_objective.go
│   competency.go lo_version_mapping.go textbook_series.go textbook.go textbook_section.go textbook_section_objective.go import_job.go
├── migrations/                                  # + trigger curriculum_versions_immutable, CHECK cardinality
├── seed/curriculum/gdpt2018_math_g1_5.yaml       # mã GDPT2018_SAMPLE, yccd_ref trống
├── cmd/seed/main.go                             # gọi command ImportCurriculum với file cục bộ (System Admin principal seed)
├── internal/domain/curriculum/                  # curriculum.go, version.go, topic_tree.go, objective.go (LoCode), prerequisite_dag.go, mapping.go, textbook.go
├── internal/application/command/curriculum/     # CreateCurriculum, UpdateCurriculum, CreateCurriculumVersion, UpdateCurriculumVersionDraft,
│                                                # Create/Update/DeleteSubject|Topic|Objective|Competency, TransitionCurriculumVersion,
│                                                # PublishCurriculumVersion, PutLoMappings, ImportCurriculum, Textbook*
├── internal/application/query/curriculum/       # ListCmsCurricula, GetCmsCurriculum, ListCurriculumVersions, ExportCurriculumVersion,
│                                                # ListCurricula (public), ListSubjects, ListTopics, ListLearningObjectives, GetLearningObjective
├── internal/infrastructure/persistence/curriculum/  # ent repo, read model cây, resolver version mặc định (ref.schools)
├── internal/infrastructure/importer/            # parser YAML/CSV curriculum → DTO; khung job import (worker)
├── internal/presentation/http/public/cms_curriculum.go learning_curriculum.go
├── internal/presentation/job/import_worker.go
└── tests/{unit,integration,contract}/curriculum/
```

**Structure Decision**: Một aggregate gốc `CurriculumVersion` (topic/LO/competency là entity con) để bất biến kiểm ở một chỗ; `Curriculum` giữ con trỏ release.

## Phase 0: Research

Không có. Thuật toán DAG (DFS 3 màu) và định dạng mã LO đã xác định ở CURRICULUM_MODEL §3.3, §4.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `CurriculumVersion` (AddTopic, AddObjective, SetPrerequisites, Submit/Approve/Reject/Publish/Archive qua `contentmodel.Lifecycle`), `LoCode.Parse`, `PrerequisiteDAG.DetectCycle`, `MappingSet.Covers(oldLOs)`, `PublishPolicy` (GDPT2018 cần `yccd_ref`) | Thuần, test bảng |
| application | Command như cây trên · Query như cây trên | `curriculum.write` (soạn, submit_review), `curriculum.publish` (approve/reject/publish/archive), `curriculum.read` (CMS đọc); môn học: chỉ principal platform; `/learning/*`: mọi principal đã đăng nhập trong tenant |
| infrastructure | ent repo, read model cây (SQL + cache), importer, resolver version mặc định | SQL tường minh có scope + `deleted_at IS NULL` |
| presentation | `listCmsCurricula`, `createCurriculum`, `getCmsCurriculum`, `updateCurriculum`, `listCmsSubjects`, `createSubject`, `updateSubject`, `listCmsTopics`, `createTopic`, `updateTopic`, `listCmsObjectives`, `createObjective`, `updateObjective`, `listCurricula`, `listSubjects`, `listTopics`, `listLearningObjectives`, `createContentImport`, `getContentImport` + operation Cần thêm ở contracts | |

### Tích hợp và phụ thuộc

- F020: `ContentItemMixin`, `Lifecycle`, `content_releases`, `content_transitions`, `ResourceRegistry` (đăng ký `curriculum`: trash, restore, clone sâu cả cây version, assignable).
- Version mặc định năm học đọc từ `lms.school_curriculum_settings` (cùng module, owner F040); không có projection/event cài đặt trường.
- Event phát: `curriculum.version_published`, `audit.recorded`, `curriculum.deleted`/`curriculum.restored` (qua F020).
- Consumer của F021 event: F031 (mastery projection), F024/F026 (kiểm LO của version published).

### Bảo mật, tenant, idempotency, audit

- Phạm vi: curriculum `owner_scope = tenant` chỉ do Academic Manager/Tenant Admin của tenant (theo permission) soạn; platform do Content Admin/System Admin.
- `Idempotency-Key` bắt buộc: `createCurriculum`, `createCurriculumVersion`, `transitionCurriculumVersion`, `publishCurriculumVersion`, `createContentImport`; `If-Match` cho mọi PATCH/transition/publish.
- Audit: `content.version_created`, `content.submitted`, `content.approved`, `content.rejected`, `content.published`, `content.archived`, `content.deleted`, `content.restored` với `target_type = curriculum`.

### Quan sát

- Metric `eduworld_curriculum_publish_total{result}`, `eduworld_import_jobs_total{kind,status}`, `eduworld_import_rows_rejected_total{kind}`; span `learning.curriculum.publish`, `learning.import.run`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
