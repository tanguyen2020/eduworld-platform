# Tasks: F024 — Question set, lesson, assessment blueprint, worked example, import hàng loạt, tìm kiếm

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package domain `questionset`, `lesson`, `blueprint` và command/query tương ứng — file: `src/backend/modules/learning/internal/domain/questionset/doc.go` — ✅ 2026-11-19: `internal/domain/{questionset,lesson,blueprint}`, `command/docs.go`, `query/docs.go`
- [x] T002 [P] Fixture: set tĩnh/luật, lesson có `worked_example`, blueprint EXAM_ENGINE §3, file import CSV/XLSX/QTI-lite/JSON (có dòng lỗi) — file: `src/backend/modules/learning/tests/integration/content/fixtures/` — ✅ 2026-11-19: fixture trong test (`content_env_test.go`, `questionsCSV`, gói QTI-lite dựng trong `TestQTILite`, mẫu `specs/examples/import/questions.template.csv`) — đặt phẳng, OQ F024-11

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Sửa/thêm OpenAPI theo contracts §6 (learning material, delete, pool stats, query `topic_id`/`objective_id`, `kind` import, `LessonAuthoringInput.blocks`) rồi tách spec — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `node scripts/openapi/split-public.mjs`, `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — ✅ 2026-11-19: learning material CRUD, 4 delete, 2 pool-stats, `topic_id`/`objective_id`, `AuthoringPatch`, `kind` import, khối lesson; `split-public`, `check-maxlength` 0, `check-portal-catalog` 0 lỗi, portal `gen:api`
- [x] T004 [P] JSON Schema `lesson.schema.json`, `import/questions-row.schema.json`, sửa `worked_example_ref` ở `question.schema.json` và `activities/interactive-exercise.schema.json`, mẫu import — file: `specs/schemas/lesson.schema.json`, `specs/schemas/question.schema.json`, `specs/examples/import/questions.template.csv` — kiểm: `make specs-lint` — ✅ 2026-11-19: `lesson.schema.json`, `import/questions-row.schema.json`, `worked_example_ref` object (question + interactive-exercise), mẫu CSV; `validate-schemas` 0 lỗi
- [x] T005 ent schema 9 entity theo data-model (MaxLen, CHECK jsonb, `search_text`) — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make gen MOD=learning` — ✅ 2026-11-19: `ent/schema/content_docs.go` (9 entity, cột chung — OQ F024-1), `TestMigrationMatchesEnt` ok
- [x] T006 Migration: extension `unaccent`, hàm `content.f_unaccent`, cột sinh `search_tsv` + GIN, trigger bất biến 4 bảng version, mở rộng CHECK `import_jobs.kind`, RLS loại H — file: `src/backend/modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=f024_content_types`, `make migrate-lint MOD=learning` — ✅ 2026-11-19: `20261127000001_f024_content_types.sql` (unaccent trong schema `content`, `f_unaccent`, `f_search_text`, `search_tsv` + GIN 5 bảng, RLS H, `_live`, trigger bất biến/con trỏ, CHECK `import_jobs.kind`)
- [x] T007 Đăng ký `ResourceHandler` `question_set` (clone theo đồ thị set → câu tenant), `lesson`, `learning_material`, `assessment_blueprint` — file: `src/backend/modules/learning/internal/application/command/content/resource_handlers.go` — ✅ 2026-11-19: `command/doc_resource.go` (`DocResources`, clone đồ thị set → câu tenant)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Question set (Priority: P1) 🎯 MVP

**Goal**: Soạn set tĩnh/theo luật hợp lệ và xem pool.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T008 [P] [US1] Unit F024-AC-03 luật (`distribution`, `difficulty`, số luật) — file: `src/backend/modules/learning/internal/domain/questionset/rules_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `questionset/rules_test.go` `TestRules`, `TestStats` ok
- [x] T009 [P] [US1] API F024-AC-01 tạo set 3 `kind` + idempotency; F024-AC-02 tham chiếu ngoài phạm vi/platform chưa gán/set platform → câu tenant; F024-AC-05 bất biến + If-Match + một nháp — file: `src/backend/modules/learning/tests/integration/content/question_set_test.go` — ✅ 2026-11-19: `TestQuestionSetAuthoring` ok (`tests/integration/content_question_set_test.go`)
- [x] T010 [P] [US1] Integration F024-AC-04 pool stats (`available`, `insufficient`, `warning`) theo con trỏ hiện hành và phạm vi tenant — file: `src/backend/modules/learning/tests/integration/content/pool_stats_test.go` — ✅ 2026-11-19: `TestQuestionSetPoolStats` ok

### Implementation cho User Story 1

- [x] T011 [P] [US1] Aggregate `QuestionSetVersion` — file: `src/backend/modules/learning/internal/domain/questionset/question_set.go` — ✅ 2026-11-19: `internal/domain/questionset/question_set.go`
- [x] T012 [US1] Command/query set (Create, UpdateDraft, NewVersion, Delete, List, Get, PoolStats qua pool query F022) — file: `src/backend/modules/learning/internal/application/command/questionset/` — ✅ 2026-11-19: `command/docs.go`, `query/docs.go` (chung cho 4 loại, OQ F024-1)
- [x] T013 [US1] Handler `listCmsQuestionSets`, `createCmsQuestionSet`, `getCmsQuestionSet`, `createCmsQuestionSetVersion`, `updateCmsQuestionSetDraft`, `deleteCmsQuestionSet`, `getQuestionSetPoolStats` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_question_sets.go` — ✅ 2026-11-19: `presentation/http/cms_docs.go` (route sinh theo loại)

## Phase 4: User Story 2 — Lesson, learning material, worked example (Priority: P1)

### Tests cho User Story 2

- [x] T014 [P] [US2] Unit F024-AC-06 khối cho phép/`BLOCK_KIND_NOT_ENABLED`/LO/số media, F024-AC-07 `worked_example` (id, 1–8 bước, không lồng câu hỏi) — file: `src/backend/modules/learning/internal/domain/lesson/blocks_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `lesson/blocks_test.go` `TestBlocks` ok
- [x] T015 [P] [US2] Integration F024-AC-08 `worked_example_ref` của câu hỏi (tồn tại, cùng phạm vi, LO chung) — file: `src/backend/modules/learning/tests/integration/content/worked_example_ref_test.go` — ✅ 2026-11-19: `TestLessonsMaterialsWorkedExamples` (ref đúng/sai block/không chung LO/tenant khác) ok
- [x] T016 [P] [US2] API F024-AC-09 CRUD learning material + lesson (tạo, xem, version mới, sửa nháp, xóa) — file: `src/backend/modules/learning/tests/integration/content/lesson_material_test.go` — ✅ 2026-11-19: `TestLessonsMaterialsWorkedExamples` (CRUD material + lesson, xóa, 428/409) ok

### Implementation cho User Story 2

- [x] T017 [P] [US2] `LessonVersion`, `Block`, `WorkedExample` (dùng chung cho learning material) — file: `src/backend/modules/learning/internal/domain/lesson/worked_example.go` — ✅ 2026-11-19: `internal/domain/lesson/blocks.go`
- [x] T018 [US2] Validator `worked_example_ref` cắm vào registry câu hỏi F022 — file: `src/backend/modules/learning/internal/application/command/question/worked_example_ref_validator.go` — ✅ 2026-11-19: `command/worked_example_ref_validator.go` (gắn vào `Questions.prepare`)
- [x] T019 [US2] Command/query + handler lesson và learning material (`listCmsLessons` … `deleteCmsLearningMaterial`) — file: `src/backend/modules/learning/internal/presentation/http/public/cms_lessons.go`, `src/backend/modules/learning/internal/presentation/http/public/cms_learning_materials.go` — ✅ 2026-11-19: `cms_docs.go` (lesson, learning material)

## Phase 5: User Story 3 — Assessment blueprint (Priority: P2)

### Tests cho User Story 3

- [x] T020 [P] [US3] Unit F024-AC-10 (tổng điểm, `feedback_timing`, `fixed_question_version_ids`, trùng câu, LO/topic thuộc version), F024-AC-12 mặc định `variant_balance_max_mean_p_diff` — file: `src/backend/modules/learning/internal/domain/blueprint/validation_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `blueprint/validation_test.go` `TestValidate`, `TestWithDefaults`, `TestStats` ok
- [x] T021 [P] [US3] Integration F024-AC-11 pool stats blueprint theo `variants.strategy` — file: `src/backend/modules/learning/tests/integration/content/blueprint_pool_test.go` — ✅ 2026-11-19: `TestAssessmentBlueprints` ok

### Implementation cho User Story 3

- [x] T022 [US3] `BlueprintVersion.Validate` + đọc cấu hình `variant_balance_max_mean_p_diff` — file: `src/backend/modules/learning/internal/domain/blueprint/validation.go` — ✅ 2026-11-19: `internal/domain/blueprint/blueprint.go` (mặc định `DefaultMaxMeanPDiff`, OQ F024-6)
- [x] T023 [US3] Command/query + handler blueprint (`listCmsAssessmentBlueprints` … `deleteCmsAssessmentBlueprint`, `getAssessmentBlueprintPoolStats`) — file: `src/backend/modules/learning/internal/presentation/http/public/cms_blueprints.go` — ✅ 2026-11-19: `cms_docs.go` (blueprint + pool stats)

## Phase 6: User Story 4 — Import hàng loạt (Priority: P2)

### Tests cho User Story 4

- [x] T024 [P] [US4] Golden test parser CSV/XLSX/QTI-lite/JSON → DTO câu hỏi/nội dung — file: `src/backend/modules/learning/internal/infrastructure/importer/importer_golden_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `importer/questions_test.go` `TestQuestionParsers`, `TestQTILite` ok
- [x] T025 [P] [US4] Integration F024-AC-13 job (dry-run, lỗi theo dòng, idempotency), F024-AC-14 công thức thiếu `math_assets` → `MATH_RENDER_FAILED`, F024-AC-15 quá cỡ/sai định dạng/`lessons_docx`, F024-AC-16 audit + tenant của nội dung tạo ra — file: `src/backend/modules/learning/tests/integration/content/import_test.go` — ✅ 2026-11-19: `TestQuestionImport` ok

### Implementation cho User Story 4

- [x] T026 [P] [US4] Parser `questions_csv`, `questions_xlsx` (excelize stream), `questions_qti_lite`, `content_json` — file: `src/backend/modules/learning/internal/infrastructure/importer/` — ✅ 2026-11-19: `infrastructure/importer/questions.go` (CSV, excelize, QTI-lite, JSON)
- [x] T027 [US4] Command `ImportContent` dùng khung job F021 + validator F022/F023/F024 — file: `src/backend/modules/learning/internal/application/command/content/import_content.go` — ✅ 2026-11-19: `command/import_content.go` + `Curriculum.runContentImport`

## Phase 7: User Story 6 — Phân quyền, Thùng rác, gán, clone (Priority: P2)

### Tests cho User Story 6

- [x] T028 [P] [US6] API F024-AC-20 phân quyền (`read`/`write`/`import`) + tenant isolation mọi endpoint của feature (gồm endpoint Cần thêm) — file: `src/backend/modules/learning/tests/integration/content/authz_isolation_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `authztest.Cover` cho 23 operation trong các test content_*; 403/404 trong `TestQuestionSetAuthoring`, `TestLessonsMaterialsWorkedExamples`, `TestQuestionImport`, `TestAssessmentBlueprints`
- [x] T029 [P] [US6] Integration F024-AC-21 gán set platform (A thấy chỉ đọc, B không thấy) — file: `src/backend/modules/learning/tests/integration/content/assign_question_set_test.go` — ✅ 2026-11-19: `TestContentAssignCloneTrash` (gán chỉ đọc, tenant B không thấy) ok
- [x] T030 [P] [US6] Integration F024-AC-22 clone set A → C (câu tenant sao, câu platform giữ, `id_map`, không dữ liệu học sinh) — file: `src/backend/modules/learning/tests/integration/content/clone_question_set_test.go` — ✅ 2026-11-19: `TestContentAssignCloneTrash` (clone set + câu tenant, giữ câu platform) ok
- [x] T031 [P] [US6] Integration F024-AC-23 soft-delete + Thùng rác 4 loại — file: `src/backend/modules/learning/tests/integration/content/trash_test.go` — ✅ 2026-11-19: `TestContentAssignCloneTrash` (4 loại vào Thùng rác, khôi phục lesson) ok

### Implementation cho User Story 6

- [x] T032 [US6] Cloner question set (đồ thị phụ thuộc), lesson, learning material, blueprint trong `ResourceHandler` — file: `src/backend/modules/learning/internal/application/command/content/cloners.go` — ✅ 2026-11-19: `DocResource.Clone`/`cloneItems`

## Phase 8: User Story 5 — Tìm kiếm (Priority: P3)

### Tests cho User Story 5

- [x] T033 [P] [US5] Integration F024-AC-17 "nhan vat" → "nhân vật" ở 5 endpoint + bộ lọc; F024-AC-18 phạm vi tenant/không bản ghi xóa/`tenant_ids`; F024-AC-19 `q` > 200 → 422; EXPLAIN dùng GIN — file: `src/backend/modules/learning/tests/integration/content/search_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestContentSearch` ok (EXPLAIN chưa ghi — OQ F024-11)

### Implementation cho User Story 5

- [x] T034 [US5] Ghép `search_text` khi lưu version (flatten khối) — file: `src/backend/modules/learning/internal/domain/lesson/search_text.go` — ✅ 2026-11-19: không cần ghép ở ứng dụng — `search_tsv` là cột sinh (OQ F024-2)
- [x] T035 [US5] `fulltext` SQL có scope + nối vào 5 query list — file: `src/backend/modules/learning/internal/infrastructure/persistence/search/fulltext.go` — ✅ 2026-11-19: `docFilters` (ent predicate `search_tsv @@ websearch_to_tsquery`) cho 4 list + `searchQuestions`

## Phase 9: Polish & Cross-Cutting

- [x] T036 [P] Metric/span tìm kiếm, import, pool stats — file: `src/backend/modules/learning/internal/infrastructure/observability/metrics.go` — ✅ 2026-11-19: `eduworld_pool_stats_insufficient_total`; span use case `learning.cms.search.*`, import dùng `eduworld_import_jobs_total`
- [x] T037 [P] Cập nhật CONTENT_MODEL §9.2 (shape `worked_example_ref`), QUESTION_BANK §10 (định nghĩa QTI-lite), CHANGELOG — file: `docs/02-learning/CONTENT_MODEL.md`, `docs/02-learning/QUESTION_BANK.md`, `CHANGELOG.md` — ✅ 2026-11-19: CONTENT_MODEL §9.2, QUESTION_BANK §10 (QTI-lite), CHANGELOG
- [ ] T038 [P] Benchmark tìm kiếm 100.000 version/tenant — file: `src/backend/modules/learning/tests/integration/content/search_bench_test.go` — ⏳ chưa chạy (OQ F024-11)
- [x] T039 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/024-question-set-lesson-blueprint/quickstart.md` — ✅ 2026-11-19: quickstart §5
- [x] T040 [P] Cập nhật DATABASE_DESIGN §2.5 (DDL question set, lesson, learning material, blueprint, `search_tsv`) — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ 2026-11-19: DATABASE_DESIGN §2.5

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T041 **OQ chốt 2026-11-20** (OQ F054-2) Tham số `content_language`, `translation_status` cho `searchQuestions` và list bốn loại F024 (contract + read model + index nếu cần); test lọc + tenant isolation — file: `specs/openapi/modules/learning.v1.yaml`, `src/backend/modules/learning/internal/infrastructure/persistence/content/` — kiểm: `make specs-breaking`, `make test-integration MOD=learning` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 / US4 (song song) → US6 → US5 → Polish.
- US2 bước T018 sửa validator của F022 (cùng module). Clone US6 cần F020 US5 Done.

## Definition of Done

- [x] Mọi tiêu chí F024-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
