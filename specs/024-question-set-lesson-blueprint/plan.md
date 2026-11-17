# Implementation Plan: F024 — Question set, lesson, assessment blueprint, worked example, import hàng loạt, tìm kiếm

**Branch**: `024-question-set-lesson-blueprint` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm bốn aggregate nội dung có version vào module `learning` — `QuestionSet`, `Lesson`, `LearningMaterial`, `AssessmentBlueprint` — dùng mixin/lifecycle của F020 và port `Delivery`/registry câu hỏi của F022 (kiểm tham chiếu, pool stats). Khối `worked_example` là value object dùng chung cho lesson/material; validator `worked_example_ref` cắm vào validator câu hỏi của F022. Import mở rộng khung job của F021 với parser CSV/Excel/QTI-lite/JSON. Tìm kiếm full-text theo [research.md](research.md). Đăng ký `ResourceHandler` cho bốn loại (trash, gán, clone có đồ thị phụ thuộc question set → question).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | ent + Atlas, oapi-codegen, `santhosh-tekuri/jsonschema/v6`, `xuri/excelize/v2`, `encoding/xml` (QTI), registry/`Delivery` F022, khung import F021 |
| Storage | PostgreSQL `eduworld_learning` schema `content` (extension `unaccent`); Valkey pool cache của F022 |
| Testing | go test, testcontainers, API test, golden file cho parser import |
| Target Platform | Linux container (`core-api` + `worker` chạy job import) |
| Project Type | backend module |
| Performance Goals | Tìm kiếm CMS p95 ≤ 500 ms với 100.000 version/tenant (EXPLAIN trong integration test); import 500 dòng ≤ 60 s |
| Constraints | ADR-0015, ADR-0028, ADR-0029, ADR-0041, ADR-0043, ADR-0055; AGENTS R11, R22, R31 |
| Scale/Scope | `csv_import_max`, `assets_per_version_max`, `variant_balance_max_mean_p_diff` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta OpenAPI (learning material, delete, pool stats, bộ lọc, `kind` import), `lesson.schema.json`, sửa `worked_example_ref` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ trong module learning; pool stats là query đọc |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng loại H, RLS, soft-delete, test isolation; SQL tìm kiếm có scope |
| IV Server-authoritative | ✅ | Blueprint/set chỉ tham chiếu version bất biến; không CDN cho đề |
| V Xác thực, phân quyền | ✅ | `learning_content.read/write/import` |
| VI An toàn trẻ em | ✅ | Không có dữ liệu học sinh; clone không sao dữ liệu học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Ràng buộc blueprint link EXAM_ENGINE; tham số theo tên |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi F024-AC có task test |

## Project Structure

### Documentation (feature này)

```text
specs/024-question-set-lesson-blueprint/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
modules/learning/
├── ent/schema/question_set.go question_set_version.go question_set_item.go lesson.go lesson_version.go
│   learning_material.go learning_material_version.go assessment_blueprint.go assessment_blueprint_version.go
├── migrations/                                   # unaccent, content.f_unaccent, search_tsv + GIN, trigger bất biến, kind import mới
├── internal/domain/questionset/{question_set.go,rules.go}
├── internal/domain/lesson/{lesson.go,blocks.go,worked_example.go}      # dùng chung cho learning material
├── internal/domain/blueprint/{blueprint.go,validation.go}
├── internal/application/command/{questionset,lesson,learningmaterial,blueprint}/   # Create, UpdateDraft, NewVersion, Delete
├── internal/application/query/{questionset,lesson,learningmaterial,blueprint}/     # List (q + bộ lọc), Get, PoolStats
├── internal/application/command/question/worked_example_ref_validator.go          # cắm vào validator F022
├── internal/infrastructure/importer/{questions_csv.go,questions_xlsx.go,qti_lite.go,content_json.go}
├── internal/infrastructure/persistence/search/fulltext.go
├── internal/presentation/http/public/{cms_question_sets.go,cms_lessons.go,cms_learning_materials.go,cms_blueprints.go}
└── tests/{unit,integration,contract}/content/
```

**Structure Decision**: Mỗi loại một aggregate (bất biến kiểm riêng); khối nội dung và `worked_example` là package `lesson` dùng chung cho learning material để một nguồn quy tắc.

## Phase 0: Research

Xem [research.md](research.md): R1 full-text không dấu, R2 QTI-lite, R3 Excel.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `QuestionSetVersion` (ValidateItems, ValidateRules), `LessonVersion`/`LearningMaterialVersion` (blocks, `WorkedExample`), `BlueprintVersion.Validate(points, feedback, rules)` | Thuần Go |
| application | Command Create/UpdateDraft/NewVersion/Delete cho 4 loại; `ImportContent` (kind mới); Query List/Get/PoolStats | `learning_content.read/write/import` |
| infrastructure | ent repo, `fulltext` SQL (scope + `deleted_at IS NULL`), parser import, pool stats qua `Delivery`/pool query F022 | |
| presentation | `listCmsQuestionSets`, `createCmsQuestionSet`, `getCmsQuestionSet`, `createCmsQuestionSetVersion`, `updateCmsQuestionSetDraft`, `listCmsLessons`, `createCmsLesson`, `getCmsLesson`, `createCmsLessonVersion`, `updateCmsLessonDraft`, `listCmsAssessmentBlueprints`, `createCmsAssessmentBlueprint`, `getCmsAssessmentBlueprint`, `createCmsAssessmentBlueprintVersion`, `updateCmsAssessmentBlueprintDraft`, `createContentImport`, `getContentImport`, `searchQuestions` (`q`) + operation Cần thêm | |

### Tích hợp và phụ thuộc

- F020 (mixin, hybrid scope, `ResourceRegistry`), F021 (LO/topic, khung import), F022 (`Delivery`, validator câu hỏi, pool query). Bên dùng: F026 (publish, kiểm tham chiếu con), F030 (khối `activity`, attempt từ question set), F042 (sinh đề từ blueprint), F053/F054 (UI).
- Event: `question_set.deleted`, `lesson.deleted`, `learning_material.deleted`, `assessment_blueprint.deleted` (+ `.restored`) qua F020; `audit.recorded`.

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key`: create*, `createContentImport`; `If-Match`: update*, delete*.
- Clone question set: bypass `tenant_clone`; đồ thị: set → câu hỏi riêng tenant nguồn (clone) / câu platform (giữ); `id_map` trả cho bước sau (vd. quest ở module game).
- Audit: `content.version_created`, `content.deleted`, `content.restored`, `tenant_data.cloned`, import (`content.imported` dạng `action` của `audit.recorded`).

### Quan sát

- Metric `eduworld_cms_search_duration_seconds{type}`, `eduworld_import_jobs_total{kind,status}`, `eduworld_pool_stats_insufficient_total`; span `learning.cms.search`, `learning.import.run`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
