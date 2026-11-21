# Tasks: F021 — Curriculum: cấu trúc, LO, DAG tiên quyết, version, mapping, import/export, seed GDPT

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package domain/application/infrastructure cho module curriculum — file: `src/backend/modules/learning/internal/domain/curriculum/doc.go` — ✅ 2026-11-19: package `internal/domain/curriculum` (codes, objective, prerequisite_dag, topic_tree, version, mapping); command/query trong package `command`/`query` phẳng (Clarifications 2026-11-19)
- [x] T002 [P] File seed `GDPT2018_SAMPLE` Toán 1–5 (16 LO theo CURRICULUM_MODEL §7, `yccd_ref` trống) — file: `src/backend/modules/learning/seed/curriculum/gdpt2018_math_g1_5.yaml` — ✅ 2026-11-19: 5 khối, 15 topic, 16 LO, `yccd_ref` trống, có tiên quyết mẫu

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Sửa/thêm OpenAPI theo contracts §6 (schema `Curriculum*`, `CurriculumVersion*`, `Topic*`, `Competency*`, `Textbook*`, `LoMappingList`; operation Cần thêm; `ContentImportRequest.kind`) rồi tách spec — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `node scripts/openapi/split-public.mjs`, `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — ✅ 2026-11-19: schema `Curriculum*`, `CurriculumVersion*`, `*Patch`, `Competency*`, `Textbook*`, `LoMapping*`, `BloomLevel`; 24 operation Cần thêm + `getLearningObjective`; split, check-maxlength/enums/portal-catalog 0 lỗi
- [x] T004 [P] JSON Schema import/export + example — file: `specs/schemas/curriculum-import.schema.json`, `specs/examples/curriculum.gdpt2018-sample-math.json` — kiểm: `make specs-lint` — ✅ 2026-11-19: `validate-schemas` 0 lỗi (example đăng ký trong scripts/specs/validate-schemas.mjs)
- [x] T005 [P] Payload `curriculum.version_published.v1` — file: `specs/events/payloads/learning/curriculum.version_published.v1.schema.json` — kiểm: `make specs-lint` — ✅ 2026-11-19: có `examples`, `validate-schemas` 0 lỗi
- [x] T006 ent schema curriculum (13 entity theo data-model, MaxLen, CHECK) — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make gen MOD=learning` — ✅ 2026-11-19: `ent/schema/curriculum.go` 13 entity, `go run ./tools/entlint` sạch
- [x] T007 Migration + trigger bất biến version và bảng con, RLS loại H/G — file: `src/backend/modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=f021_curriculum`, `make migrate-lint MOD=learning` — ✅ 2026-11-19: `20261124000001_f021_curriculum.sql` (entddl khớp `TestMigrationMatchesEnt`, CHECK, RLS H/T*, `_live`, trigger `curriculum_versions_immutable` + `*_draft_only`), áp dụng trong `TestStringsAreBounded` (testcontainers) pass
- [x] T008 Đăng ký permission `curriculum.read/write/publish` cho handler; đăng ký `ResourceHandler` `curriculum` (trash, restore, clone cây version, assignable) — file: `src/backend/modules/learning/internal/application/command/curriculum/resource_handler.go` — ✅ 2026-11-19: `command.CurriculumResource` (`internal/application/command/curriculum_resource.go`, cần port `write.CurriculumTrashStore`) — trash/restore curriculum + topic + LO, clone sâu version mới nhất, owners; permission `curriculum.read/write/publish` gọi `authz.Require` đầu mỗi use case; Thùng rác kiểm trong `TestCurriculumEditability`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Soạn cấu trúc chương trình (Priority: P1) 🎯 MVP

**Goal**: CRUD curriculum/version/môn/topic/LO/competency đúng quyền, đúng phạm vi.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit F021-AC-03 `LoCode.Parse` (bảng hợp lệ/không hợp lệ, `G<grade>` khớp khối), F021-AC-05 cây topic không vòng — file: `src/backend/modules/learning/internal/domain/curriculum/objective_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestLoCode`, `TestObjectiveValidate`, `TestTopicParent`, `TestCodesAndKinds` (`go test ./modules/learning/internal/domain/curriculum` ok)
- [x] T010 [P] [US1] API F021-AC-01 tạo curriculum + version draft (idempotent), F021-AC-02 nhiều version song song, F021-AC-04 `yccd_ref` 65 ký tự / competency sai version → 422 — file: `src/backend/modules/learning/tests/integration/curriculum/authoring_test.go` — ✅ 2026-11-19: `TestCurriculumAuthoring` (file `tests/integration/curriculum_authoring_test.go`, cùng package integration để dùng chung TestMain/fixture) pass
- [x] T011 [P] [US1] API F021-AC-06 phân quyền (school_admin, tenant ghi môn học/curriculum platform → 403 + audit), F021-AC-08 tenant isolation mọi endpoint `/cms/learning/{curricula,subjects,topics,objectives,competencies}` — file: `src/backend/modules/learning/tests/integration/curriculum/authz_isolation_test.go` — ✅ 2026-11-19: `TestCurriculumAuthzIsolation` + `TestCurriculumRouteMatrix` (43 operation × role_denied/cross_tenant, `authztest.Cover`) pass; `go run ./tools/routematrix check` 0 problems
- [x] T012 [P] [US1] API F021-AC-07 sửa version không draft → 409, F021-AC-09 If-Match thiếu/lệch, F021-AC-10 soft-delete + Thùng rác + archive — file: `src/backend/modules/learning/tests/integration/curriculum/editability_test.go` — ✅ 2026-11-19: `TestCurriculumEditability` pass (409 CONTENT_NOT_EDITABLE use case + trigger, 428/412, soft-delete topic/curriculum vào Thùng rác, curriculum đã publish → 409)

### Implementation cho User Story 1

- [x] T013 [P] [US1] Aggregate `CurriculumVersion`, entity Topic/Objective/Competency, `LoCode` — file: `src/backend/modules/learning/internal/domain/curriculum/version.go` — ✅ 2026-11-19: domain `curriculum` (`codes.go`, `objective.go`, `topic_tree.go`, `version.go`); dữ liệu aggregate ở `application/dto`
- [x] T014 [US1] Command Create/Update Curriculum, Subject, Topic, Objective, Competency; Delete* — file: `src/backend/modules/learning/internal/application/command/curriculum/authoring.go` — ✅ 2026-11-19: `command/curriculum_authoring.go`, `curriculum_structure.go` (package `command` phẳng như F020)
- [x] T015 [US1] Query `ListCmsCurricula`, `GetCmsCurriculum`, `ListCurriculumVersions`, list topic/objective/competency (hybrid scope F020) — file: `src/backend/modules/learning/internal/application/query/curriculum/cms_queries.go` — ✅ 2026-11-19: `query/curriculum.go` + read model `persistence/read/curriculum.go` (ent trong ReadSession, phân trang theo id)
- [x] T016 [US1] Repository ent + handler CMS (operationId theo contracts §1) — file: `src/backend/modules/learning/internal/presentation/http/public/cms_curriculum.go` — ✅ 2026-11-19: repo `persistence/write/curriculum_repo.go`, `catalog_repo.go`; handler `presentation/http/cms_curriculum.go`, `cms_structure.go`; `TestCurriculumOperationsMatchSpec` (contract) pass

## Phase 4: User Story 2 — Đồ thị LO tiên quyết (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Unit F021-AC-11 (khác version, > 20, tự tham chiếu), F021-AC-12 chu trình ngắn/dài ≥ 5 trả đường đi — file: `src/backend/modules/learning/internal/domain/curriculum/prerequisite_dag_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestDetectCycle` (ngắn, dài 6 LO, tự tham chiếu, kim cương) + `TestObjectiveValidate` (21 phần tử, tự tham chiếu) pass
- [x] T018 [P] [US2] API F021-AC-12 trả `errors[].code = PREREQUISITE_CYCLE`, không lưu — file: `src/backend/modules/learning/tests/integration/curriculum/prerequisite_test.go` — ✅ 2026-11-19: trong `TestCurriculumAuthoring` (A→B→C rồi C→A → 422 `prerequisite_ids:PREREQUISITE_CYCLE`, `cycle` liệt kê 3 mã, không lưu) pass

### Implementation cho User Story 2

- [x] T019 [US2] `PrerequisiteDAG.DetectCycle` (DFS 3 màu) + gọi trong `SetPrerequisites` — file: `src/backend/modules/learning/internal/domain/curriculum/prerequisite_dag.go` — ✅ 2026-11-19: `prerequisite_dag.go` (DFS 3 màu, trả đường đi), gọi trong `command.checkObjective`

## Phase 5: User Story 3 — Duyệt và publish CurriculumVersion (Priority: P1)

### Tests cho User Story 3

- [x] T020 [P] [US3] Unit `PublishPolicy` F021-AC-14 (GDPT2018 thiếu `yccd_ref`) — file: `src/backend/modules/learning/internal/domain/curriculum/publish_policy_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestMissingYccd`, `TestMappingCoverage`, `TestVersionRules` pass
- [x] T021 [P] [US3] Integration F021-AC-13 workflow (reviewer ≠ author, reject cần note, transition sai → 422), F021-AC-18 thiếu `curriculum.publish` → 403 — file: `src/backend/modules/learning/tests/integration/curriculum/workflow_test.go` — ✅ 2026-11-19: `TestCurriculumWorkflow` pass (tự duyệt 422 INVALID_STATE_TRANSITION, reject thiếu note 422, content_reviewer 403, 5 dòng content_transitions, audit)
- [x] T022 [P] [US3] Integration F021-AC-15 publish một tx (version, con trỏ, `content_releases`, audit, outbox) + idempotency; F021-AC-16 trigger chặn UPDATE — file: `src/backend/modules/learning/tests/integration/curriculum/publish_test.go` — ✅ 2026-11-19: `TestCurriculumWorkflow` pass (một content_releases, một outbox `curriculum.version_published`, replay `Idempotent-Replayed`, trigger chặn UPDATE version/topic)
- [x] T023 [P] [US3] Integration F021-AC-17 tạo version mới sao sâu, giữ `code`/`stable_key`, chỉ một draft mở — file: `src/backend/modules/learning/tests/integration/curriculum/new_version_test.go` — ✅ 2026-11-19: `TestCurriculumWorkflow` pass (version 2 draft sao sâu giữ code/stable_key, tiên quyết ánh xạ lại, lần hai 409 CONFLICT)

### Implementation cho User Story 3

- [x] T024 [US3] Command `TransitionCurriculumVersion`, `PublishCurriculumVersion`, `CreateCurriculumVersion` (deep copy) — file: `src/backend/modules/learning/internal/application/command/curriculum/workflow.go` — ✅ 2026-11-19: `command/curriculum_workflow.go` (CreateVersion, Transition, Publish)
- [x] T025 [US3] Handler `transitionCurriculumVersion`, `publishCurriculumVersion`, `createCurriculumVersion`, `getCurriculumVersion`, `updateCurriculumVersionDraft` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_curriculum_versions.go` — ✅ 2026-11-19: handler trong `presentation/http/cms_curriculum.go`

## Phase 6: User Story 4 — Đọc cây curriculum cho game và portal (Priority: P2)

### Tests cho User Story 4

- [x] T026 [P] [US4] API F021-AC-19 version áp dụng theo `lms.school_curriculum_settings` (owner F040; fixture dòng cài đặt) và fallback theo hiệu lực; F021-AC-20 chỉ published, không field biên tập — file: `src/backend/modules/learning/tests/integration/curriculum/public_read_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestCurriculumPublicRead` pass (fallback theo hiệu lực, cài đặt trường qua fixture `lms.school_curriculum_settings` gắn nhãn TEST FIXTURE, không field biên tập, Content-Language)
- [x] T027 [P] [US4] API F021-AC-21 tra LO version cũ sau publish version mới; tenant isolation `/learning/{curricula,subjects,topics,objectives}` — file: `src/backend/modules/learning/tests/integration/curriculum/history_lookup_test.go` — ✅ 2026-11-19: `TestCurriculumPublicRead` (LO version cũ sau khi version 2 publish, tenant B 404) + `TestCurriculumRouteMatrix` pass

### Implementation cho User Story 4

- [x] T028 [US4] Resolver version mặc định + read model cây có cache — file: `src/backend/modules/learning/internal/infrastructure/persistence/curriculum/tree_read_model.go` — ✅ 2026-11-19: resolver `read.SchoolSettings` + `query.Curriculum.applied`; cache Valkey của cây chưa làm (đọc thẳng DB, OQ F021-10)
- [x] T029 [US4] Query + handler `listCurricula`, `listSubjects`, `listTopics`, `listLearningObjectives`, `getLearningObjective` (Accept-Language, Content-Language) — file: `src/backend/modules/learning/internal/presentation/http/public/learning_curriculum.go` — ✅ 2026-11-19: `presentation/http/public.go`, query `PublicCurricula/Topics/Objectives/Objective`

## Phase 7: User Story 5 — Import/export và seed (Priority: P2)

### Tests cho User Story 5

- [x] T030 [P] [US5] Integration F021-AC-22 job import (dry-run không ghi, lỗi theo dòng, idempotency), F021-AC-25 file quá `csv_import_max`/sai định dạng → failed — file: `src/backend/modules/learning/tests/integration/curriculum/import_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestCurriculumImportSeedExport` pass (dry-run không ghi, lỗi theo dòng có row/field/code, replay Idempotency-Key, quá csv_import_max → FILE_TOO_LARGE, YAML/CSV hỏng → INVALID_FORMAT)
- [x] T031 [P] [US5] Integration F021-AC-23 seed chạy hai lần không trùng; F021-AC-24 export → import round-trip — file: `src/backend/modules/learning/tests/integration/curriculum/seed_export_test.go` — ✅ 2026-11-19: `TestCurriculumImportSeedExport` pass (seed 2 lần: 16 LO, lần 2 bỏ qua; export YAML/CSV → import lại cùng cấu trúc)

### Implementation cho User Story 5

- [x] T032 [US5] Khung job import (`content.import_jobs`, worker, báo cáo dòng) dùng chung cho F024 — file: `src/backend/modules/learning/internal/presentation/job/import_worker.go` — ✅ 2026-11-19: `content.import_jobs` + `command.CreateImport/RunPendingImports/RunImportJob`, runner `presentation/job/curriculum.go` (`ImportWorker`); nối vào worker profile jobs cùng F020-3
- [x] T033 [US5] Parser YAML/CSV curriculum + command `ImportCurriculum` — file: `src/backend/modules/learning/internal/infrastructure/importer/curriculum_parser.go` — ✅ 2026-11-19: `infrastructure/importer/curriculum.go` (YAML theo node có số dòng, CSV) + `command.ImportCurriculum` (savepoint từng dòng, rollback khi có lỗi hoặc dry-run)
- [x] T034 [US5] Query `ExportCurriculumVersion` + handler `exportCurriculumVersion` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_curriculum_export.go` — ✅ 2026-11-19: `query.Curriculum.Export` + handler `exportCurriculumVersion`
- [x] T035 [US5] `src/backend/cmd/seed` gọi `ImportCurriculum` + publish `GDPT2018_SAMPLE`; nối vào `make seed` — file: `src/backend/modules/learning/cmd/seed/main.go`, `Makefile` — ✅ 2026-11-19: `modules/learning/cmd/seed/main.go` → `job.Seed` → `SeedSubjects` + `SeedCurriculum` (import + publish); `make seed`; `seed/curriculum/subjects.yaml`

## Phase 8: User Story 6 — Mapping LO giữa version (Priority: P3)

### Tests cho User Story 6

- [x] T036 [P] [US6] Integration F021-AC-26 thiếu mapping → 422; F021-AC-27 payload event có `mappings`, learning event không đổi — file: `src/backend/modules/learning/tests/integration/curriculum/lo_mapping_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestCurriculumLoMappings` pass (thiếu mapping → 422 liệt kê mã, payload 3 mapping có `removed`, version 1 không đổi)

### Implementation cho User Story 6

- [x] T037 [US6] `MappingSet.Covers` + command `PutLoMappings` + handler `getCurriculumVersionLoMappings`/`putCurriculumVersionLoMappings` — file: `src/backend/modules/learning/internal/application/command/curriculum/lo_mappings.go` — ✅ 2026-11-19: `MappingSet.Validate/Uncovered` + `command.PutLoMappings` + handler GET/PUT

## Phase 9: User Story 7 — Mapping bộ sách (Priority: P3)

### Tests cho User Story 7

- [x] T038 [P] [US7] API F021-AC-28 CRUD bộ sách/tập/bài (không có trường nguyên văn), F021-AC-29 lọc LO theo `textbook_section_id` + tenant isolation — file: `src/backend/modules/learning/tests/integration/curriculum/textbook_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestCurriculumTextbooks` pass (trường nguyên văn bị contract từ chối, lọc LO theo bài: platform 2, tenant A được gán 2, tenant B 0)

### Implementation cho User Story 7

- [x] T039 [US7] Command/query/handler textbook (`listTextbookSeries` … `putTextbookSectionObjectives`) — file: `src/backend/modules/learning/internal/presentation/http/public/cms_textbooks.go` — ✅ 2026-11-19: `command/curriculum_textbook.go` + handler trong `cms_structure.go`

## Phase 10: Polish & Cross-Cutting

- [x] T040 [P] Cập nhật DATABASE_DESIGN (DDL `curriculum.*`, `content.import_jobs`, khóa `lo_version_mappings`), CURRICULUM_MODEL §7 (đường dẫn seed, mã `GDPT2018_SAMPLE`), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/02-learning/CURRICULUM_MODEL.md`, `CHANGELOG.md` — ✅ 2026-11-19: DATABASE_DESIGN §2.4 (DDL curriculum), §2.5 (`import_jobs`), §2.16 (khóa mapping/bộ sách); CURRICULUM_MODEL §9; CHANGELOG
- [x] T041 [P] Metric/span publish và import — file: `src/backend/modules/learning/internal/infrastructure/observability/metrics.go` — ✅ 2026-11-19: `eduworld_curriculum_publish_total{result}`, `eduworld_import_jobs_total{kind,status}`, `eduworld_import_rows_rejected_total{kind}` (`observability.CurriculumCounter`); span theo `observability.UseCase` (`learning.command.PublishCurriculumVersion`, `learning.command.ImportCurriculum`)
- [x] T042 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/021-curriculum/quickstart.md` — ✅ 2026-11-19: quickstart §5 bằng chứng (kịch bản 1–16 bằng test tự động)

## ADR-0095 — nhận FR-CUR-008 từ F040 (WP-L1, 2026-11-21)

- [x] T043 Chuyển `lms.school_curriculum_settings` (command `SchoolCurriculum`, query `SchoolCurriculum`, repository `CurriculumSettingsRepo`/`CurriculumSettingsModel`, handler `get/putSchoolCurriculumSettings`) ra khỏi code F040 trước khi xóa F040; integration test `TestSchoolCurriculumSettings` (F021-AC-30) — file: `src/backend/modules/learning/internal/`, `src/backend/modules/learning/tests/integration/curriculum_settings_test.go`
- [x] T044 (WP-I1) Nguồn năm học hiện tại của trường cho resolver thay `ref.classes.academic_year_id`: `ref.students.academic_year_id` từ `student.placement_changed` (migration `20261229000004_adr0095_student_academic_year`, `read.SchoolSettings.DefaultVersion`)

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 / US5 (song song) → US6 → US7 → Polish.
- US4 cần cột projection của F015 (fallback theo hiệu lực chạy được khi chưa có). Import qua portal cần F027; seed không cần.

## Definition of Done

- [x] Mọi tiêu chí F021-AC-* có test pass (ghi mã test/TC) — 29/29 AC, bảng quickstart §5
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — lệnh tương đương (không có `make`): gofmt, vet, golangci-lint (2 config), tenancyguard, ewlint, entlint, archgen, routematrix, unit + contract, integration 28/28
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — test double `memAssets` và fixture `lms.school_curriculum_settings` chỉ trong test, có nhãn
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — DATABASE_DESIGN, CURRICULUM_MODEL §9, specs/events README, CHANGELOG; CONFIG_DEFAULTS không đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
