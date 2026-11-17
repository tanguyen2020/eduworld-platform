# Tasks: F024 — Question set, lesson, assessment blueprint, worked example, import hàng loạt, tìm kiếm

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package domain `questionset`, `lesson`, `blueprint` và command/query tương ứng — file: `modules/learning/internal/domain/questionset/doc.go`
- [ ] T002 [P] Fixture: set tĩnh/luật, lesson có `worked_example`, blueprint EXAM_ENGINE §3, file import CSV/XLSX/QTI-lite/JSON (có dòng lỗi) — file: `modules/learning/tests/integration/content/fixtures/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Sửa/thêm OpenAPI theo contracts §6 (learning material, delete, pool stats, query `topic_id`/`objective_id`, `kind` import, `LessonAuthoringInput.blocks`) rồi tách spec — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `node scripts/openapi/split-public.mjs`, `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T004 [P] JSON Schema `lesson.schema.json`, `import/questions-row.schema.json`, sửa `worked_example_ref` ở `question.schema.json` và `activities/interactive-exercise.schema.json`, mẫu import — file: `specs/schemas/lesson.schema.json`, `specs/schemas/question.schema.json`, `specs/examples/import/questions.template.csv` — kiểm: `make specs-lint`
- [ ] T005 ent schema 9 entity theo data-model (MaxLen, CHECK jsonb, `search_text`) — file: `modules/learning/ent/schema/` — kiểm: `make gen MOD=learning`
- [ ] T006 Migration: extension `unaccent`, hàm `content.f_unaccent`, cột sinh `search_tsv` + GIN, trigger bất biến 4 bảng version, mở rộng CHECK `import_jobs.kind`, RLS loại H — file: `modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=f024_content_types`, `make migrate-lint MOD=learning`
- [ ] T007 Đăng ký `ResourceHandler` `question_set` (clone theo đồ thị set → câu tenant), `lesson`, `learning_material`, `assessment_blueprint` — file: `modules/learning/internal/application/command/content/resource_handlers.go`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Question set (Priority: P1) 🎯 MVP

**Goal**: Soạn set tĩnh/theo luật hợp lệ và xem pool.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T008 [P] [US1] Unit F024-AC-03 luật (`distribution`, `difficulty`, số luật) — file: `modules/learning/internal/domain/questionset/rules_test.go` — phải FAIL trước khi code
- [ ] T009 [P] [US1] API F024-AC-01 tạo set 3 `kind` + idempotency; F024-AC-02 tham chiếu ngoài phạm vi/platform chưa gán/set platform → câu tenant; F024-AC-05 bất biến + If-Match + một nháp — file: `modules/learning/tests/integration/content/question_set_test.go`
- [ ] T010 [P] [US1] Integration F024-AC-04 pool stats (`available`, `insufficient`, `warning`) theo con trỏ hiện hành và phạm vi tenant — file: `modules/learning/tests/integration/content/pool_stats_test.go`

### Implementation cho User Story 1

- [ ] T011 [P] [US1] Aggregate `QuestionSetVersion` — file: `modules/learning/internal/domain/questionset/question_set.go`
- [ ] T012 [US1] Command/query set (Create, UpdateDraft, NewVersion, Delete, List, Get, PoolStats qua pool query F022) — file: `modules/learning/internal/application/command/questionset/`
- [ ] T013 [US1] Handler `listCmsQuestionSets`, `createCmsQuestionSet`, `getCmsQuestionSet`, `createCmsQuestionSetVersion`, `updateCmsQuestionSetDraft`, `deleteCmsQuestionSet`, `getQuestionSetPoolStats` — file: `modules/learning/internal/presentation/http/public/cms_question_sets.go`

## Phase 4: User Story 2 — Lesson, learning material, worked example (Priority: P1)

### Tests cho User Story 2

- [ ] T014 [P] [US2] Unit F024-AC-06 khối cho phép/`BLOCK_KIND_NOT_ENABLED`/LO/số media, F024-AC-07 `worked_example` (id, 1–8 bước, không lồng câu hỏi) — file: `modules/learning/internal/domain/lesson/blocks_test.go` — phải FAIL trước khi code
- [ ] T015 [P] [US2] Integration F024-AC-08 `worked_example_ref` của câu hỏi (tồn tại, cùng phạm vi, LO chung) — file: `modules/learning/tests/integration/content/worked_example_ref_test.go`
- [ ] T016 [P] [US2] API F024-AC-09 CRUD learning material + lesson (tạo, xem, version mới, sửa nháp, xóa) — file: `modules/learning/tests/integration/content/lesson_material_test.go`

### Implementation cho User Story 2

- [ ] T017 [P] [US2] `LessonVersion`, `Block`, `WorkedExample` (dùng chung cho learning material) — file: `modules/learning/internal/domain/lesson/worked_example.go`
- [ ] T018 [US2] Validator `worked_example_ref` cắm vào registry câu hỏi F022 — file: `modules/learning/internal/application/command/question/worked_example_ref_validator.go`
- [ ] T019 [US2] Command/query + handler lesson và learning material (`listCmsLessons` … `deleteCmsLearningMaterial`) — file: `modules/learning/internal/presentation/http/public/cms_lessons.go`, `modules/learning/internal/presentation/http/public/cms_learning_materials.go`

## Phase 5: User Story 3 — Assessment blueprint (Priority: P2)

### Tests cho User Story 3

- [ ] T020 [P] [US3] Unit F024-AC-10 (tổng điểm, `feedback_timing`, `fixed_question_version_ids`, trùng câu, LO/topic thuộc version), F024-AC-12 mặc định `variant_balance_max_mean_p_diff` — file: `modules/learning/internal/domain/blueprint/validation_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US3] Integration F024-AC-11 pool stats blueprint theo `variants.strategy` — file: `modules/learning/tests/integration/content/blueprint_pool_test.go`

### Implementation cho User Story 3

- [ ] T022 [US3] `BlueprintVersion.Validate` + đọc cấu hình `variant_balance_max_mean_p_diff` — file: `modules/learning/internal/domain/blueprint/validation.go`
- [ ] T023 [US3] Command/query + handler blueprint (`listCmsAssessmentBlueprints` … `deleteCmsAssessmentBlueprint`, `getAssessmentBlueprintPoolStats`) — file: `modules/learning/internal/presentation/http/public/cms_blueprints.go`

## Phase 6: User Story 4 — Import hàng loạt (Priority: P2)

### Tests cho User Story 4

- [ ] T024 [P] [US4] Golden test parser CSV/XLSX/QTI-lite/JSON → DTO câu hỏi/nội dung — file: `modules/learning/internal/infrastructure/importer/importer_golden_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Integration F024-AC-13 job (dry-run, lỗi theo dòng, idempotency), F024-AC-14 công thức thiếu `math_assets` → `MATH_RENDER_FAILED`, F024-AC-15 quá cỡ/sai định dạng/`lessons_docx`, F024-AC-16 audit + tenant của nội dung tạo ra — file: `modules/learning/tests/integration/content/import_test.go`

### Implementation cho User Story 4

- [ ] T026 [P] [US4] Parser `questions_csv`, `questions_xlsx` (excelize stream), `questions_qti_lite`, `content_json` — file: `modules/learning/internal/infrastructure/importer/`
- [ ] T027 [US4] Command `ImportContent` dùng khung job F021 + validator F022/F023/F024 — file: `modules/learning/internal/application/command/content/import_content.go`

## Phase 7: User Story 6 — Phân quyền, Thùng rác, gán, clone (Priority: P2)

### Tests cho User Story 6

- [ ] T028 [P] [US6] API F024-AC-20 phân quyền (`read`/`write`/`import`) + tenant isolation mọi endpoint của feature (gồm endpoint Cần thêm) — file: `modules/learning/tests/integration/content/authz_isolation_test.go` — phải FAIL trước khi code
- [ ] T029 [P] [US6] Integration F024-AC-21 gán set platform (A thấy chỉ đọc, B không thấy) — file: `modules/learning/tests/integration/content/assign_question_set_test.go`
- [ ] T030 [P] [US6] Integration F024-AC-22 clone set A → C (câu tenant sao, câu platform giữ, `id_map`, không dữ liệu học sinh) — file: `modules/learning/tests/integration/content/clone_question_set_test.go`
- [ ] T031 [P] [US6] Integration F024-AC-23 soft-delete + Thùng rác 4 loại — file: `modules/learning/tests/integration/content/trash_test.go`

### Implementation cho User Story 6

- [ ] T032 [US6] Cloner question set (đồ thị phụ thuộc), lesson, learning material, blueprint trong `ResourceHandler` — file: `modules/learning/internal/application/command/content/cloners.go`

## Phase 8: User Story 5 — Tìm kiếm (Priority: P3)

### Tests cho User Story 5

- [ ] T033 [P] [US5] Integration F024-AC-17 "nhan vat" → "nhân vật" ở 5 endpoint + bộ lọc; F024-AC-18 phạm vi tenant/không bản ghi xóa/`tenant_ids`; F024-AC-19 `q` > 200 → 422; EXPLAIN dùng GIN — file: `modules/learning/tests/integration/content/search_test.go` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T034 [US5] Ghép `search_text` khi lưu version (flatten khối) — file: `modules/learning/internal/domain/lesson/search_text.go`
- [ ] T035 [US5] `fulltext` SQL có scope + nối vào 5 query list — file: `modules/learning/internal/infrastructure/persistence/search/fulltext.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T036 [P] Metric/span tìm kiếm, import, pool stats — file: `modules/learning/internal/infrastructure/observability/metrics.go`
- [ ] T037 [P] Cập nhật CONTENT_MODEL §9.2 (shape `worked_example_ref`), QUESTION_BANK §10 (định nghĩa QTI-lite), CHANGELOG — file: `docs/02-learning/CONTENT_MODEL.md`, `docs/02-learning/QUESTION_BANK.md`, `CHANGELOG.md`
- [ ] T038 [P] Benchmark tìm kiếm 100.000 version/tenant — file: `modules/learning/tests/integration/content/search_bench_test.go`
- [ ] T039 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/024-question-set-lesson-blueprint/quickstart.md`
- [ ] T040 [P] Cập nhật DATABASE_DESIGN §2.5 (DDL question set, lesson, learning material, blueprint, `search_tsv`) — file: `docs/07-backend/DATABASE_DESIGN.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 / US4 (song song) → US6 → US5 → Polish.
- US2 bước T018 sửa validator của F022 (cùng module). Clone US6 cần F020 US5 Done.

## Definition of Done

- [ ] Mọi tiêu chí F024-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
