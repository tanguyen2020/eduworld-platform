# Tasks: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `internal/domain/question/{types,normalize,numeric,render,selection}`; thêm ngoại lệ depguard cho `cockroachdb/apd`, `golang.org/x/text` ở tầng domain — file: `src/backend/modules/learning/.golangci.yml` — **Bằng chứng:** package `internal/domain/question` (+ `normalize`, `numeric`, `selection`); không cần ngoại lệ depguard: `big.Rat` thay `apd` (OQ F022-1), `golang.org/x/text` đã được phép; bố cục gộp `types`/`render` (OQ F022-2)
- [x] T002 [P] Fixture 8 ví dụ QUESTION_BANK §3 và bộ cặp chuẩn hóa tiếng Việt (≥ 200, cột `input,expected,allow_diacritics_missing,match`) — file: `src/backend/modules/learning/tests/integration/question/fixtures/qb_examples/`, `src/backend/modules/learning/internal/domain/question/normalize/testdata/normalize_vi_pairs.csv` — **Bằng chứng:** `internal/domain/question/testdata/qb_examples/*.json` (8 ví dụ), `normalize/testdata/normalize_vi_pairs.csv` (314 cặp)

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Sửa OpenAPI theo contracts §6 (`QuestionAuthoringInput`, `getCmsQuestionVersion`, query `searchQuestions`) rồi tách spec — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `node scripts/openapi/split-public.mjs`, `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — **Bằng chứng:** `QuestionAuthoringInput` viết lại, `getCmsQuestionVersion`, query `searchQuestions`; split 509 operation; `check-maxlength`/`check-enums`/`check-portal-catalog` 0 lỗi; `routematrix gen` + `check` 0 problems
- [x] T004 ent schema `question`, `question_version` (cột thêm, `grade_levels`), `question_version_objective` (+ `tenant_id`) — file: `src/backend/modules/learning/ent/schema/question_version.go` — kiểm: `make gen MOD=learning` — **Bằng chứng:** `ent/schema/question.go` (Question, QuestionVersion, QuestionVersionObjective — OQ F022-3); `ent/ddl_test.go` golden pass
- [x] T005 Migration: cột thêm (expand), trigger `question_versions_immutable`, `questions_release_target`, view `content.question_delivery_v`, unique một nháp mở — file: `src/backend/modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=f022_question_bank`, `make migrate-lint MOD=learning` — **Bằng chứng:** `migrations/20261125000001_f022_question_bank.sql` (CHECK, FK, RLS H, `_live`, view `content.question_delivery_v`, trigger `question_versions_immutable`, `questions_release_target`, một nháp mở) + `atlas.sum`; áp dụng trong mọi test tích hợp
- [x] T006 Registry `TypeHandler` + port `Delivery`; nạp `question.schema.json` (embed) cho validator — file: `src/backend/modules/learning/internal/domain/question/registry.go`, `src/backend/modules/learning/internal/infrastructure/schema/validator.go` — **Bằng chứng:** `internal/domain/question/registry.go`; `infrastructure/qbschema/validator.go` (`TestEmbeddedSchemas` đồng bộ với `specs/schemas`)
- [x] T007 Đăng ký `ResourceHandler` `question` (trash, restore, clone câu + version draft, assignable) — file: `src/backend/modules/learning/internal/application/command/question/resource_handler.go` — **Bằng chứng:** `command.QuestionResource` (`application/command/question.go`); trash/restore qua use case F020 (`TestQuestionAuthzSearchDelete`)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Soạn câu hỏi 8 loại (Priority: P1) 🎯 MVP

**Goal**: API CMS tạo/sửa/xem/version mới/xóa/tìm câu hỏi hợp lệ.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T008 [P] [US1] Unit validator ngữ nghĩa 8 loại: F022-AC-03 (ID trùng/sai mẫu, key lạ, ràng buộc theo loại), F022-AC-04 (`partial_method` theo loại, `max_score`), F022-AC-05 (khối cho phép, NFC, số media) — file: `src/backend/modules/learning/internal/domain/question/types/validator_test.go` — phải FAIL trước khi code — **Bằng chứng:** `TestValidate`, `TestSplitFeedback` (`internal/domain/question/question_test.go`) pass
- [x] T009 [P] [US1] API F022-AC-01 tạo 8 loại + idempotency; F022-AC-02 loại chưa bật → `TYPE_NOT_ENABLED`; F022-AC-06 tách phần chỉ server khi lưu; F022-AC-10 CMS thấy đáp án — file: `src/backend/modules/learning/tests/integration/question/authoring_test.go` — **Bằng chứng:** `TestQuestionAuthoring` (`tests/integration/question_test.go`) pass
- [x] T010 [P] [US1] API F022-AC-07 version mới/không sửa khi không draft; F022-AC-08 If-Match; F022-AC-12 vượt giới hạn → 422 — file: `src/backend/modules/learning/tests/integration/question/versioning_test.go` — **Bằng chứng:** `TestQuestionVersioning` + phần AC-12 của `TestQuestionAuthoring` pass
- [x] T011 [P] [US1] API F022-AC-09 phân quyền + tenant isolation mọi endpoint `/cms/learning/questions*` (tenant A/B, platform gán/không gán, System Admin); F022-AC-13 bộ lọc `searchQuestions` — file: `src/backend/modules/learning/tests/integration/question/authz_isolation_test.go` — **Bằng chứng:** `TestQuestionAuthzSearchDelete` (11 bộ lọc) + `TestQuestionRouteMatrix` (7 operation, `authztest.Cover`) pass
- [x] T012 [P] [US1] API F022-AC-11 soft-delete + Thùng rác + attempt cũ xem lại (version còn) — file: `src/backend/modules/learning/tests/integration/question/delete_test.go` — **Bằng chứng:** `TestQuestionAuthzSearchDelete` (Thùng rác, audit `content.deleted`, event `question.deleted`, version còn, khôi phục, câu đã phát hành → 409) pass

### Implementation cho User Story 1

- [x] T013 [P] [US1] Validator 8 loại + mapper authoring ↔ storage (tách `option_feedback`) — file: `src/backend/modules/learning/internal/domain/question/types/` — **Bằng chứng:** `internal/domain/question/types.go`, `split.go` (tách `option_feedback`)
- [x] T014 [US1] Command `CreateQuestion`, `UpdateQuestionDraft`, `CreateQuestionVersion`, `DeleteQuestion` — file: `src/backend/modules/learning/internal/application/command/question/` — **Bằng chứng:** `internal/application/command/question.go` (`CreateQuestion`, `UpdateQuestionDraft`, `CreateQuestionVersion`, `DeleteQuestion`)
- [x] T015 [US1] Query `GetCmsQuestion`, `GetCmsQuestionVersion`, `SearchQuestions` (hybrid scope) — file: `src/backend/modules/learning/internal/application/query/question/` — **Bằng chứng:** `internal/application/query/question.go` (`GetQuestion`, `GetQuestionVersion`, `Search`) + `persistence/read/question.go`
- [x] T016 [US1] Repository ent + handler `searchQuestions`, `createQuestion`, `getCmsQuestion`, `getCmsQuestionVersion`, `updateQuestionDraft`, `createQuestionVersion`, `deleteQuestion` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_questions.go` — **Bằng chứng:** `persistence/write/question_repo.go`, `presentation/http/cms_questions.go`; contract `TestCurriculumOperationsMatchSpec` phủ 7 operation

## Phase 4: User Story 2 — Chấm phía server (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Bảng test F022-AC-14 (8 loại, điểm trong `[min_score, max_score]`), F022-AC-15 công thức `multiple_choice`, F022-AC-16 `ordering`/`matching`/`drag_drop`/`true_false`/`fill_in_blank` — file: `src/backend/modules/learning/internal/domain/question/types/scorer_test.go` — phải FAIL trước khi code — **Bằng chứng:** `TestGradeTable` (8 loại, điểm trong `[min_score, max_score]`) pass
- [x] T018 [P] [US2] Test F022-AC-17 chuẩn hóa tiếng Việt (≥ 200 cặp) + tiếng Anh + `exact` — file: `src/backend/modules/learning/internal/domain/question/normalize/vietnamese_test.go` — **Bằng chứng:** `TestNormalizationPairs` (314 cặp), `TestNormalizationRules` pass; `normalize` coverage 100%
- [x] T019 [P] [US2] Test F022-AC-18 parse số (`,`/`.`, ngăn nghìn, phân số, dạng tương đương, 3 kiểu dung sai) — file: `src/backend/modules/learning/internal/domain/question/numeric/parse_test.go` — **Bằng chứng:** `TestParse`, `TestWithin` pass; `numeric` coverage 95,5%
- [x] T020 [P] [US2] Test F022-AC-19 response sai shape → `INVALID_RESPONSE` không panic; F022-AC-20 property shuffle; F022-AC-21 không có đầu vào `score`/`is_correct` (API `Grade` chỉ nhận response) — file: `src/backend/modules/learning/internal/domain/question/scorer_property_test.go` — **Bằng chứng:** `TestGradeInvalidResponses`, `TestGradeIndependentOfShuffle` pass; `Delivery.Grade` chỉ nhận response

### Implementation cho User Story 2

- [x] T021 [P] [US2] `normalize.Vietnamese`, `normalize.English`, bảng dấu thanh — file: `src/backend/modules/learning/internal/domain/question/normalize/vietnamese.go` — **Bằng chứng:** `normalize/normalize.go`, `normalize/tone_table.go`
- [x] T022 [P] [US2] `numeric.Parse` (apd) — file: `src/backend/modules/learning/internal/domain/question/numeric/parse.go` — **Bằng chứng:** `numeric/parse.go` (`big.Rat`, OQ F022-1)
- [x] T023 [US2] Scorer 8 loại + validate response theo `$defs.<type>Response` — file: `src/backend/modules/learning/internal/domain/question/types/scorers.go` — **Bằng chứng:** `types.go` (8 scorer) + `qbschema.ValidateResponse` (`INVALID_RESPONSE`, kiểm trong `TestQuestionDelivery`)
- [x] T024 [US2] `Delivery.Grade` (nạp answer key qua cache `ew:learning:key:*`) — file: `src/backend/modules/learning/internal/infrastructure/persistence/question/delivery.go` — **Bằng chứng:** `query.Delivery.Grade` + `read.Delivery.Grading`; cache key `ew:learning:key:*` chưa làm (OQ F022-12)

## Phase 5: User Story 3 — Render payload không đáp án (Priority: P1)

### Tests cho User Story 3

- [x] T025 [P] [US3] Contract test F022-AC-22 quét DTO render 8 loại (danh sách cấm), F022-AC-25 `math_ref` + `speech_text` — file: `src/backend/modules/learning/tests/contract/question/render_dto_test.go` — phải FAIL trước khi code — **Bằng chứng:** `tests/contract/question_render_test.go` `TestRenderDTOHasNoAnswerData` (8 loại, khóa cấm, id giữ nguyên, `math_ref` + `speech_text`) pass
- [x] T026 [P] [US3] Unit F022-AC-23 trộn tái lập, giữ ID, `ordering` khác thứ tự đúng — file: `src/backend/modules/learning/internal/domain/question/render/shuffle_test.go` — **Bằng chứng:** `TestRender` (trộn tái lập, giữ ID, `ordering` khác thứ tự đúng) pass
- [x] T027 [P] [US3] Integration F022-AC-24 delivery reader chỉ dùng view; role đọc view không select được `answer_key` — file: `src/backend/modules/learning/tests/integration/question/delivery_view_test.go` — **Bằng chứng:** `TestQuestionDelivery`: `learning_readonly` không select được `answer_key` từ view lẫn bảng; view không có 6 cột đáp án

### Implementation cho User Story 3

- [x] T028 [US3] `render.Build` danh sách trắng theo loại + `shuffle` (PCG) — file: `src/backend/modules/learning/internal/domain/question/render/whitelist.go` — **Bằng chứng:** `registry.go` `Render` (PCG theo seed + version, danh sách trắng theo payload, `math_ref`)
- [x] T029 [US3] `Delivery.Render` + reader view delivery — file: `src/backend/modules/learning/internal/infrastructure/persistence/question/delivery_reader.go` — **Bằng chứng:** `query.Delivery.Render` + `read.Delivery.Deliverable` (đọc view delivery); OQ F022-4

## Phase 6: User Story 4 — Chọn câu ngẫu nhiên (Priority: P2)

### Tests cho User Story 4

- [x] T030 [P] [US4] Unit F022-AC-26 lọc/phân bổ/seed/loại trừ/`exam_only`, F022-AC-27 nới lỏng + `QUESTION_POOL_INSUFFICIENT` — file: `src/backend/modules/learning/internal/domain/question/selection/select_test.go` — phải FAIL trước khi code — **Bằng chứng:** `TestSelect`, `TestRelaxation` (`selection/select_test.go`) pass
- [x] T031 [P] [US4] Integration F022-AC-26 phạm vi tenant (chỉ con trỏ hiện hành, platform gán), F022-AC-28 xóa cache khi `content.published`/gán/gỡ gán — file: `src/backend/modules/learning/tests/integration/question/selection_test.go` — **Bằng chứng:** `TestQuestionDelivery` (chỉ con trỏ hiện hành, platform không gán/nháp/exam_only/efficacy bị loại, tenant B không thấy, cache bị xóa theo tenant/toàn bộ) pass

### Implementation cho User Story 4

- [x] T032 [US4] `selection.Select` + `relax` — file: `src/backend/modules/learning/internal/domain/question/selection/select.go` — **Bằng chứng:** `selection/select.go`
- [x] T033 [US4] Pool query (index delivery) + cache Valkey + consumer invalidation — file: `src/backend/modules/learning/internal/presentation/consumer/qb_pool_invalidation.go` — **Bằng chứng:** `read.Delivery.Pool`, `cache/pool.go` (TTL 5 phút, OQ F022-14), `presentation/consumer/qb_pool_invalidation.go` (chưa nối binary — OQ F022-8)

## Phase 7: Polish & Cross-Cutting

- [x] T034 [P] Cập nhật DATABASE_DESIGN §2.5, §3.3 (cột thêm, `grade_levels`, `topic_id NOT NULL`, view delivery), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `CHANGELOG.md` — **Bằng chứng:** DATABASE_DESIGN §2.5, §3.3; CHANGELOG (Added)
- [x] T035 [P] Benchmark `Grade` và metric/span theo plan — file: `src/backend/modules/learning/internal/domain/question/types/scorer_bench_test.go` — **Bằng chứng:** `BenchmarkGrade` (`scorer_test.go`); metric `eduworld_qb_grade_total`, `eduworld_qb_select_pool_insufficient_total`, `eduworld_qb_pool_cache_requests_total` (`observability.QuestionCounter`, kiểm trong `TestQuestionDelivery`); span `learning.qb.*`
- [x] T036 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/022-question-bank-core/quickstart.md` — **Bằng chứng:** [quickstart.md §5](quickstart.md#5-bằng-chứng-2026-11-19)

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 và US3 (song song) → US4 → Polish.
- US4 cần dữ liệu published có con trỏ: test nạp trực tiếp qua repository; luồng publish thật ở F026.

## Definition of Done

- [x] Mọi tiêu chí F022-AC-* có test pass (ghi mã test/TC) — bảng ánh xạ ở [quickstart.md §5](quickstart.md#5-bằng-chứng-2026-11-19)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — chạy lệnh gốc (máy không có `make`), xem quickstart §5
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — helper test (`publishQuestion`, `memAssets`) gắn nhãn TEST
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — DATABASE_DESIGN §2.5/§3.3, CHANGELOG; CONFIG_DEFAULTS không đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
