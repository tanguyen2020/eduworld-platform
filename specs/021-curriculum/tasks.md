# Tasks: F021 — Curriculum: cấu trúc, LO, DAG tiên quyết, version, mapping, import/export, seed GDPT

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package domain/application/infrastructure cho module curriculum — file: `src/backend/modules/learning/internal/domain/curriculum/doc.go`
- [ ] T002 [P] File seed `GDPT2018_SAMPLE` Toán 1–5 (16 LO theo CURRICULUM_MODEL §7, `yccd_ref` trống) — file: `src/backend/modules/learning/seed/curriculum/gdpt2018_math_g1_5.yaml`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Sửa/thêm OpenAPI theo contracts §6 (schema `Curriculum*`, `CurriculumVersion*`, `Topic*`, `Competency*`, `Textbook*`, `LoMappingList`; operation Cần thêm; `ContentImportRequest.kind`) rồi tách spec — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `node scripts/openapi/split-public.mjs`, `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T004 [P] JSON Schema import/export + example — file: `specs/schemas/curriculum-import.schema.json`, `specs/examples/curriculum.gdpt2018-sample-math.json` — kiểm: `make specs-lint`
- [ ] T005 [P] Payload `curriculum.version_published.v1` — file: `specs/events/payloads/learning/curriculum.version_published.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T006 ent schema curriculum (13 entity theo data-model, MaxLen, CHECK) — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make gen MOD=learning`
- [ ] T007 Migration + trigger bất biến version và bảng con, RLS loại H/G — file: `src/backend/modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=f021_curriculum`, `make migrate-lint MOD=learning`
- [ ] T008 Đăng ký permission `curriculum.read/write/publish` cho handler; đăng ký `ResourceHandler` `curriculum` (trash, restore, clone cây version, assignable) — file: `src/backend/modules/learning/internal/application/command/curriculum/resource_handler.go`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Soạn cấu trúc chương trình (Priority: P1) 🎯 MVP

**Goal**: CRUD curriculum/version/môn/topic/LO/competency đúng quyền, đúng phạm vi.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit F021-AC-03 `LoCode.Parse` (bảng hợp lệ/không hợp lệ, `G<grade>` khớp khối), F021-AC-05 cây topic không vòng — file: `src/backend/modules/learning/internal/domain/curriculum/objective_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] API F021-AC-01 tạo curriculum + version draft (idempotent), F021-AC-02 nhiều version song song, F021-AC-04 `yccd_ref` 65 ký tự / competency sai version → 422 — file: `src/backend/modules/learning/tests/integration/curriculum/authoring_test.go`
- [ ] T011 [P] [US1] API F021-AC-06 phân quyền (school_admin, tenant ghi môn học/curriculum platform → 403 + audit), F021-AC-08 tenant isolation mọi endpoint `/cms/learning/{curricula,subjects,topics,objectives,competencies}` — file: `src/backend/modules/learning/tests/integration/curriculum/authz_isolation_test.go`
- [ ] T012 [P] [US1] API F021-AC-07 sửa version không draft → 409, F021-AC-09 If-Match thiếu/lệch, F021-AC-10 soft-delete + Thùng rác + archive — file: `src/backend/modules/learning/tests/integration/curriculum/editability_test.go`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Aggregate `CurriculumVersion`, entity Topic/Objective/Competency, `LoCode` — file: `src/backend/modules/learning/internal/domain/curriculum/version.go`
- [ ] T014 [US1] Command Create/Update Curriculum, Subject, Topic, Objective, Competency; Delete* — file: `src/backend/modules/learning/internal/application/command/curriculum/authoring.go`
- [ ] T015 [US1] Query `ListCmsCurricula`, `GetCmsCurriculum`, `ListCurriculumVersions`, list topic/objective/competency (hybrid scope F020) — file: `src/backend/modules/learning/internal/application/query/curriculum/cms_queries.go`
- [ ] T016 [US1] Repository ent + handler CMS (operationId theo contracts §1) — file: `src/backend/modules/learning/internal/presentation/http/public/cms_curriculum.go`

## Phase 4: User Story 2 — Đồ thị LO tiên quyết (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Unit F021-AC-11 (khác version, > 20, tự tham chiếu), F021-AC-12 chu trình ngắn/dài ≥ 5 trả đường đi — file: `src/backend/modules/learning/internal/domain/curriculum/prerequisite_dag_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] API F021-AC-12 trả `errors[].code = PREREQUISITE_CYCLE`, không lưu — file: `src/backend/modules/learning/tests/integration/curriculum/prerequisite_test.go`

### Implementation cho User Story 2

- [ ] T019 [US2] `PrerequisiteDAG.DetectCycle` (DFS 3 màu) + gọi trong `SetPrerequisites` — file: `src/backend/modules/learning/internal/domain/curriculum/prerequisite_dag.go`

## Phase 5: User Story 3 — Duyệt và publish CurriculumVersion (Priority: P1)

### Tests cho User Story 3

- [ ] T020 [P] [US3] Unit `PublishPolicy` F021-AC-14 (GDPT2018 thiếu `yccd_ref`) — file: `src/backend/modules/learning/internal/domain/curriculum/publish_policy_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US3] Integration F021-AC-13 workflow (reviewer ≠ author, reject cần note, transition sai → 422), F021-AC-18 thiếu `curriculum.publish` → 403 — file: `src/backend/modules/learning/tests/integration/curriculum/workflow_test.go`
- [ ] T022 [P] [US3] Integration F021-AC-15 publish một tx (version, con trỏ, `content_releases`, audit, outbox) + idempotency; F021-AC-16 trigger chặn UPDATE — file: `src/backend/modules/learning/tests/integration/curriculum/publish_test.go`
- [ ] T023 [P] [US3] Integration F021-AC-17 tạo version mới sao sâu, giữ `code`/`stable_key`, chỉ một draft mở — file: `src/backend/modules/learning/tests/integration/curriculum/new_version_test.go`

### Implementation cho User Story 3

- [ ] T024 [US3] Command `TransitionCurriculumVersion`, `PublishCurriculumVersion`, `CreateCurriculumVersion` (deep copy) — file: `src/backend/modules/learning/internal/application/command/curriculum/workflow.go`
- [ ] T025 [US3] Handler `transitionCurriculumVersion`, `publishCurriculumVersion`, `createCurriculumVersion`, `getCurriculumVersion`, `updateCurriculumVersionDraft` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_curriculum_versions.go`

## Phase 6: User Story 4 — Đọc cây curriculum cho game và portal (Priority: P2)

### Tests cho User Story 4

- [ ] T026 [P] [US4] API F021-AC-19 version áp dụng theo `lms.school_curriculum_settings` (owner F040; fixture dòng cài đặt) và fallback theo hiệu lực; F021-AC-20 chỉ published, không field biên tập — file: `src/backend/modules/learning/tests/integration/curriculum/public_read_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US4] API F021-AC-21 tra LO version cũ sau publish version mới; tenant isolation `/learning/{curricula,subjects,topics,objectives}` — file: `src/backend/modules/learning/tests/integration/curriculum/history_lookup_test.go`

### Implementation cho User Story 4

- [ ] T028 [US4] Resolver version mặc định + read model cây có cache — file: `src/backend/modules/learning/internal/infrastructure/persistence/curriculum/tree_read_model.go`
- [ ] T029 [US4] Query + handler `listCurricula`, `listSubjects`, `listTopics`, `listLearningObjectives`, `getLearningObjective` (Accept-Language, Content-Language) — file: `src/backend/modules/learning/internal/presentation/http/public/learning_curriculum.go`

## Phase 7: User Story 5 — Import/export và seed (Priority: P2)

### Tests cho User Story 5

- [ ] T030 [P] [US5] Integration F021-AC-22 job import (dry-run không ghi, lỗi theo dòng, idempotency), F021-AC-25 file quá `csv_import_max`/sai định dạng → failed — file: `src/backend/modules/learning/tests/integration/curriculum/import_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US5] Integration F021-AC-23 seed chạy hai lần không trùng; F021-AC-24 export → import round-trip — file: `src/backend/modules/learning/tests/integration/curriculum/seed_export_test.go`

### Implementation cho User Story 5

- [ ] T032 [US5] Khung job import (`content.import_jobs`, worker, báo cáo dòng) dùng chung cho F024 — file: `src/backend/modules/learning/internal/presentation/job/import_worker.go`
- [ ] T033 [US5] Parser YAML/CSV curriculum + command `ImportCurriculum` — file: `src/backend/modules/learning/internal/infrastructure/importer/curriculum_parser.go`
- [ ] T034 [US5] Query `ExportCurriculumVersion` + handler `exportCurriculumVersion` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_curriculum_export.go`
- [ ] T035 [US5] `src/backend/cmd/seed` gọi `ImportCurriculum` + publish `GDPT2018_SAMPLE`; nối vào `make seed` — file: `src/backend/modules/learning/cmd/seed/main.go`, `Makefile`

## Phase 8: User Story 6 — Mapping LO giữa version (Priority: P3)

### Tests cho User Story 6

- [ ] T036 [P] [US6] Integration F021-AC-26 thiếu mapping → 422; F021-AC-27 payload event có `mappings`, learning event không đổi — file: `src/backend/modules/learning/tests/integration/curriculum/lo_mapping_test.go` — phải FAIL trước khi code

### Implementation cho User Story 6

- [ ] T037 [US6] `MappingSet.Covers` + command `PutLoMappings` + handler `getCurriculumVersionLoMappings`/`putCurriculumVersionLoMappings` — file: `src/backend/modules/learning/internal/application/command/curriculum/lo_mappings.go`

## Phase 9: User Story 7 — Mapping bộ sách (Priority: P3)

### Tests cho User Story 7

- [ ] T038 [P] [US7] API F021-AC-28 CRUD bộ sách/tập/bài (không có trường nguyên văn), F021-AC-29 lọc LO theo `textbook_section_id` + tenant isolation — file: `src/backend/modules/learning/tests/integration/curriculum/textbook_test.go` — phải FAIL trước khi code

### Implementation cho User Story 7

- [ ] T039 [US7] Command/query/handler textbook (`listTextbookSeries` … `putTextbookSectionObjectives`) — file: `src/backend/modules/learning/internal/presentation/http/public/cms_textbooks.go`

## Phase 10: Polish & Cross-Cutting

- [ ] T040 [P] Cập nhật DATABASE_DESIGN (DDL `curriculum.*`, `content.import_jobs`, khóa `lo_version_mappings`), CURRICULUM_MODEL §7 (đường dẫn seed, mã `GDPT2018_SAMPLE`), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/02-learning/CURRICULUM_MODEL.md`, `CHANGELOG.md`
- [ ] T041 [P] Metric/span publish và import — file: `src/backend/modules/learning/internal/infrastructure/observability/metrics.go`
- [ ] T042 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/021-curriculum/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 / US5 (song song) → US6 → US7 → Polish.
- US4 cần cột projection của F015 (fallback theo hiệu lực chạy được khi chưa có). Import qua portal cần F027; seed không cần.

## Definition of Done

- [ ] Mọi tiêu chí F021-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
