# Tasks: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `internal/domain/question/{types,normalize,numeric,render,selection}`; thêm ngoại lệ depguard cho `cockroachdb/apd`, `golang.org/x/text` ở tầng domain — file: `modules/learning/.golangci.yml`
- [ ] T002 [P] Fixture 8 ví dụ QUESTION_BANK §3 và bộ cặp chuẩn hóa tiếng Việt (≥ 200, cột `input,expected,allow_diacritics_missing,match`) — file: `modules/learning/tests/integration/question/fixtures/qb_examples/`, `modules/learning/internal/domain/question/normalize/testdata/normalize_vi_pairs.csv`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Sửa OpenAPI theo contracts §6 (`QuestionAuthoringInput`, `getCmsQuestionVersion`, query `searchQuestions`) rồi tách spec — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `node scripts/openapi/split-public.mjs`, `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T004 ent schema `question`, `question_version` (cột thêm, `grade_levels`), `question_version_objective` (+ `tenant_id`) — file: `modules/learning/ent/schema/question_version.go` — kiểm: `make gen MOD=learning`
- [ ] T005 Migration: cột thêm (expand), trigger `question_versions_immutable`, `questions_release_target`, view `content.question_delivery_v`, unique một nháp mở — file: `modules/learning/migrations/` — kiểm: `make migrate-diff MOD=learning name=f022_question_bank`, `make migrate-lint MOD=learning`
- [ ] T006 Registry `TypeHandler` + port `Delivery`; nạp `question.schema.json` (embed) cho validator — file: `modules/learning/internal/domain/question/registry.go`, `modules/learning/internal/infrastructure/schema/validator.go`
- [ ] T007 Đăng ký `ResourceHandler` `question` (trash, restore, clone câu + version draft, assignable) — file: `modules/learning/internal/application/command/question/resource_handler.go`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Soạn câu hỏi 8 loại (Priority: P1) 🎯 MVP

**Goal**: API CMS tạo/sửa/xem/version mới/xóa/tìm câu hỏi hợp lệ.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T008 [P] [US1] Unit validator ngữ nghĩa 8 loại: F022-AC-03 (ID trùng/sai mẫu, key lạ, ràng buộc theo loại), F022-AC-04 (`partial_method` theo loại, `max_score`), F022-AC-05 (khối cho phép, NFC, số media) — file: `modules/learning/internal/domain/question/types/validator_test.go` — phải FAIL trước khi code
- [ ] T009 [P] [US1] API F022-AC-01 tạo 8 loại + idempotency; F022-AC-02 loại chưa bật → `TYPE_NOT_ENABLED`; F022-AC-06 tách phần chỉ server khi lưu; F022-AC-10 CMS thấy đáp án — file: `modules/learning/tests/integration/question/authoring_test.go`
- [ ] T010 [P] [US1] API F022-AC-07 version mới/không sửa khi không draft; F022-AC-08 If-Match; F022-AC-12 vượt giới hạn → 422 — file: `modules/learning/tests/integration/question/versioning_test.go`
- [ ] T011 [P] [US1] API F022-AC-09 phân quyền + tenant isolation mọi endpoint `/cms/learning/questions*` (tenant A/B, platform gán/không gán, System Admin); F022-AC-13 bộ lọc `searchQuestions` — file: `modules/learning/tests/integration/question/authz_isolation_test.go`
- [ ] T012 [P] [US1] API F022-AC-11 soft-delete + Thùng rác + attempt cũ xem lại (version còn) — file: `modules/learning/tests/integration/question/delete_test.go`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Validator 8 loại + mapper authoring ↔ storage (tách `option_feedback`) — file: `modules/learning/internal/domain/question/types/`
- [ ] T014 [US1] Command `CreateQuestion`, `UpdateQuestionDraft`, `CreateQuestionVersion`, `DeleteQuestion` — file: `modules/learning/internal/application/command/question/`
- [ ] T015 [US1] Query `GetCmsQuestion`, `GetCmsQuestionVersion`, `SearchQuestions` (hybrid scope) — file: `modules/learning/internal/application/query/question/`
- [ ] T016 [US1] Repository ent + handler `searchQuestions`, `createQuestion`, `getCmsQuestion`, `getCmsQuestionVersion`, `updateQuestionDraft`, `createQuestionVersion`, `deleteQuestion` — file: `modules/learning/internal/presentation/http/public/cms_questions.go`

## Phase 4: User Story 2 — Chấm phía server (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Bảng test F022-AC-14 (8 loại, điểm trong `[min_score, max_score]`), F022-AC-15 công thức `multiple_choice`, F022-AC-16 `ordering`/`matching`/`drag_drop`/`true_false`/`fill_in_blank` — file: `modules/learning/internal/domain/question/types/scorer_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Test F022-AC-17 chuẩn hóa tiếng Việt (≥ 200 cặp) + tiếng Anh + `exact` — file: `modules/learning/internal/domain/question/normalize/vietnamese_test.go`
- [ ] T019 [P] [US2] Test F022-AC-18 parse số (`,`/`.`, ngăn nghìn, phân số, dạng tương đương, 3 kiểu dung sai) — file: `modules/learning/internal/domain/question/numeric/parse_test.go`
- [ ] T020 [P] [US2] Test F022-AC-19 response sai shape → `INVALID_RESPONSE` không panic; F022-AC-20 property shuffle; F022-AC-21 không có đầu vào `score`/`is_correct` (API `Grade` chỉ nhận response) — file: `modules/learning/internal/domain/question/scorer_property_test.go`

### Implementation cho User Story 2

- [ ] T021 [P] [US2] `normalize.Vietnamese`, `normalize.English`, bảng dấu thanh — file: `modules/learning/internal/domain/question/normalize/vietnamese.go`
- [ ] T022 [P] [US2] `numeric.Parse` (apd) — file: `modules/learning/internal/domain/question/numeric/parse.go`
- [ ] T023 [US2] Scorer 8 loại + validate response theo `$defs.<type>Response` — file: `modules/learning/internal/domain/question/types/scorers.go`
- [ ] T024 [US2] `Delivery.Grade` (nạp answer key qua cache `ew:learning:key:*`) — file: `modules/learning/internal/infrastructure/persistence/question/delivery.go`

## Phase 5: User Story 3 — Render payload không đáp án (Priority: P1)

### Tests cho User Story 3

- [ ] T025 [P] [US3] Contract test F022-AC-22 quét DTO render 8 loại (danh sách cấm), F022-AC-25 `math_ref` + `speech_text` — file: `modules/learning/tests/contract/question/render_dto_test.go` — phải FAIL trước khi code
- [ ] T026 [P] [US3] Unit F022-AC-23 trộn tái lập, giữ ID, `ordering` khác thứ tự đúng — file: `modules/learning/internal/domain/question/render/shuffle_test.go`
- [ ] T027 [P] [US3] Integration F022-AC-24 delivery reader chỉ dùng view; role đọc view không select được `answer_key` — file: `modules/learning/tests/integration/question/delivery_view_test.go`

### Implementation cho User Story 3

- [ ] T028 [US3] `render.Build` danh sách trắng theo loại + `shuffle` (PCG) — file: `modules/learning/internal/domain/question/render/whitelist.go`
- [ ] T029 [US3] `Delivery.Render` + reader view delivery — file: `modules/learning/internal/infrastructure/persistence/question/delivery_reader.go`

## Phase 6: User Story 4 — Chọn câu ngẫu nhiên (Priority: P2)

### Tests cho User Story 4

- [ ] T030 [P] [US4] Unit F022-AC-26 lọc/phân bổ/seed/loại trừ/`exam_only`, F022-AC-27 nới lỏng + `QUESTION_POOL_INSUFFICIENT` — file: `modules/learning/internal/domain/question/selection/select_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US4] Integration F022-AC-26 phạm vi tenant (chỉ con trỏ hiện hành, platform gán), F022-AC-28 xóa cache khi `content.published`/gán/gỡ gán — file: `modules/learning/tests/integration/question/selection_test.go`

### Implementation cho User Story 4

- [ ] T032 [US4] `selection.Select` + `relax` — file: `modules/learning/internal/domain/question/selection/select.go`
- [ ] T033 [US4] Pool query (index delivery) + cache Valkey + consumer invalidation — file: `modules/learning/internal/presentation/consumer/qb_pool_invalidation.go`

## Phase 7: Polish & Cross-Cutting

- [ ] T034 [P] Cập nhật DATABASE_DESIGN §2.5, §3.3 (cột thêm, `grade_levels`, `topic_id NOT NULL`, view delivery), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `CHANGELOG.md`
- [ ] T035 [P] Benchmark `Grade` và metric/span theo plan — file: `modules/learning/internal/domain/question/types/scorer_bench_test.go`
- [ ] T036 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/022-question-bank-core/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 và US3 (song song) → US4 → Polish.
- US4 cần dữ liệu published có con trỏ: test nạp trực tiếp qua repository; luồng publish thật ở F026.

## Definition of Done

- [ ] Mọi tiêu chí F022-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
