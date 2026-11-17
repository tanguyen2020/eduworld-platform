# Implementation Plan: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Branch**: `022-question-bank-core` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `question` trong module `learning`: aggregate `Question`/`QuestionVersion` dùng mixin nội dung của F020; registry theo `QuestionType` với ba vai trò cho mỗi loại — `Validator` (schema + ngữ nghĩa), `Scorer` (chấm thuần Go, partial credit), `Renderer` (danh sách trắng + trộn theo seed). Port `Delivery` (`Select`, `Render`, `Grade`, `Explanation`) phục vụ F030 và question-plan của thi đấu. API CMS: tạo/sửa/xem/version mới/xóa/tìm kiếm. View `content.question_delivery_v` cho đường giao học sinh. Quyết định kỹ thuật ở [research.md](research.md).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | ent + Atlas, oapi-codegen, `santhosh-tekuri/jsonschema/v6` (validate `question.schema.json`), `cockroachdb/apd/v3`, `golang.org/x/text`, `math/rand/v2` |
| Storage | PostgreSQL `eduworld_learning` schema `content`; Valkey `ew:learning:qb:pool:<tenant_id>:<filter_hash>` (5 phút), `ew:learning:key:<question_version_id>` (chỉ server) |
| Testing | Bảng test + property test (`testing/quick`) cho scorer; contract test DTO; testcontainers |
| Target Platform | Linux container |
| Project Type | backend module |
| Performance Goals | `Grade` thuần CPU < 1 ms/câu p99 (benchmark); `Select` đọc cache pool |
| Constraints | ADR-0015, ADR-0016, ADR-0041, ADR-0043, ADR-0055; AGENTS R5, R6, R11 |
| Scale/Scope | `assets_per_version_max`, `request_body_max` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `QuestionAuthoringInput`, `getCmsQuestionVersion`, bộ lọc `searchQuestions` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Scorer/Renderer ở domain thuần; `Delivery` là port nội bộ module |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Loại H qua F020; soft-delete; test isolation mọi endpoint |
| IV Server-authoritative | ✅ | Chấm chỉ server; render danh sách trắng; view không cột đáp án |
| V Xác thực, phân quyền | ✅ | `learning_content.read/write`; tenant không ghi dữ liệu platform |
| VI An toàn trẻ em | ✅ | Không có dữ liệu học sinh; không log đáp án ở INFO (R14) |
| VII Một nguồn sự thật, cấu hình | ✅ | Công thức chấm link QUESTION_BANK; tham số theo tên |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi F022-AC có task test |

## Project Structure

### Documentation (feature này)

```text
specs/022-question-bank-core/
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
src/backend/modules/learning/
├── ent/schema/question.go question_version.go question_version_objective.go
├── migrations/                                   # trigger question_versions_immutable, view content.question_delivery_v, CHECK octet_length
├── internal/domain/question/
│   ├── question.go version.go presentation.go grade_result.go
│   ├── registry.go                               # TypeHandler{Validator, Scorer, Renderer} theo QuestionType
│   ├── types/{single_choice,multiple_choice,true_false,fill_in_blank,numeric,matching,ordering,drag_drop}.go
│   ├── normalize/{vietnamese.go,english.go,tone_table.go}
│   ├── numeric/parse.go
│   ├── render/whitelist.go shuffle.go
│   └── selection/select.go relax.go
├── internal/application/
│   ├── port/delivery.go question_repository.go pool_cache.go
│   ├── command/question/{create.go,update_draft.go,new_version.go,delete.go}
│   └── query/question/{get.go,get_version.go,search.go}
├── internal/infrastructure/persistence/question/ # ent repo, delivery reader (view), pool cache
├── internal/infrastructure/schema/validator.go   # nạp specs/schemas/question.schema.json (embed khi build)
├── internal/presentation/http/public/cms_questions.go
├── internal/presentation/consumer/qb_pool_invalidation.go  # content.published/rolled_back/unpublished (item_type=question)
└── tests/{unit,integration,contract}/question/   # fixtures/qb_examples/*.json, normalize_vi_pairs.csv (≥ 200)
```

**Structure Decision**: Một registry cho mọi loại để F023 thêm loại bằng đăng ký, không sửa code lõi (open/closed); domain không import ent/http.

## Phase 0: Research

Xem [research.md](research.md): R1 decimal, R2 chuẩn hóa tiếng Việt, R3 trộn tái lập, R4 tách phần chỉ server.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `QuestionVersion` (Edit chỉ khi draft), `TypeHandler` 8 loại, `normalize.Vietnamese`, `numeric.Parse`, `render.Build(version, seed)`, `selection.Select(pool, rule, seed, exclude)` | Thuần Go |
| application | Command `CreateQuestion`, `UpdateQuestionDraft`, `CreateQuestionVersion`, `DeleteQuestion` · Query `GetCmsQuestion`, `GetCmsQuestionVersion`, `SearchQuestions` · Port `Delivery` | `learning_content.write` (ghi), `learning_content.read` (đọc CMS) |
| infrastructure | ent repo (mixin F020), delivery reader trên view, pool cache Valkey, JSON Schema validator | SQL tường minh có scope + `deleted_at IS NULL` |
| presentation | `searchQuestions`, `createQuestion`, `getCmsQuestion`, `getCmsQuestionVersion`, `updateQuestionDraft`, `createQuestionVersion`, `deleteQuestion`; consumer invalidation | |

### Tích hợp và phụ thuộc

- F020 (mixin, hybrid scope, `ResourceHandler` `question`: trash/restore/clone câu + version draft), F021 (LO, topic, môn, `curriculum_version_id` phải tồn tại trong phạm vi).
- Bên dùng port `Delivery`: F024 (pool stats), F030 (attempt), F105 (question-plan qua facade `learning`: `createQuestionPlan`/`scorePlanAnswer`).
- Event tiêu thụ: `content.published`, `content.rolled_back`, `content.unpublished` (`item_type = question`), `tenant_data.assigned/unassigned` → xóa pool cache.

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key`: `createQuestion`, `createQuestionVersion`; `If-Match`: `updateQuestionDraft`, `deleteQuestion`.
- Audit: `content.version_created`, `content.deleted`, `content.restored` (`target_type = question`). Không ghi nội dung đáp án vào log/audit (R14).
- Tenant không ghi câu platform; System Admin ghi cần tenant đích hoặc `owner_scope = platform` (F020-AC-12).

### Quan sát

- Metric `eduworld_qb_grade_total{type,result}`, `eduworld_qb_grade_invalid_response_total{type}`, `eduworld_qb_select_pool_insufficient_total`, `eduworld_qb_pool_cache_hit_ratio`; span `learning.qb.grade`, `learning.qb.select`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
