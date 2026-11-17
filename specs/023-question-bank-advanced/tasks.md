# Tasks: F023 — Question Bank nâng cao: short_answer, image_based, interactive, câu tham số hóa

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `geometry`, `widget`, `expr`, `params` trong domain question — file: `modules/learning/internal/domain/question/expr/doc.go`
- [ ] T002 [P] Fixture câu `short_answer`, `image_based` (hotspot, label), `interactive` (`balance_scale`), tham số hóa (`single_choice`, `numeric`) — file: `modules/learning/tests/integration/question/advanced/fixtures/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Sửa `question.schema.json` (giới hạn mảng/biểu thức, `params`) và thêm `question-widgets/balance_scale.v1.schema.json`, cập nhật `specs/README.md` — file: `specs/schemas/question.schema.json`, `specs/schemas/question-widgets/balance_scale.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T004 Thêm operation `listQuestionWidgets` + schema `QuestionWidgetList`, tách spec — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `node scripts/openapi/split-public.mjs`, `make specs-lint`, `make gen MOD=learning`
- [ ] T005 Không có thay đổi ent/migration (xác nhận bằng `make migrate-diff MOD=learning name=f023_noop` không sinh file) — file: `modules/learning/migrations/`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Câu trả lời ngắn (Priority: P1) 🎯 MVP

**Goal**: Tạo và chấm `short_answer` đúng chính sách chuẩn hóa.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T006 [P] [US1] Unit validator F023-AC-01 (`accepted`, fuzzy biên), F023-AC-03 chặn fuzzy môn `vietnamese`/`english` — file: `modules/learning/internal/domain/question/types/short_answer_validator_test.go` — phải FAIL trước khi code
- [ ] T007 [P] [US1] Bảng chấm F023-AC-02 (normalized/exact, dấu, fuzzy theo `min_length`), F023-AC-04 tiếng Anh — file: `modules/learning/internal/domain/question/types/short_answer_scorer_test.go`

### Implementation cho User Story 1

- [ ] T008 [P] [US1] Levenshtein có giới hạn — file: `modules/learning/internal/domain/question/normalize/levenshtein.go`
- [ ] T009 [US1] `ShortAnswerHandler` (validator, scorer, renderer) + đăng ký registry (bỏ `TYPE_NOT_ENABLED` cho loại này) — file: `modules/learning/internal/domain/question/types/short_answer.go`

## Phase 4: User Story 5 — Phân quyền và giới hạn cho loại mới (Priority: P1)

### Tests cho User Story 5

- [ ] T010 [P] [US5] API F023-AC-19 chạy lại bộ authz/isolation F022 với 3 loại mới + `listQuestionWidgets` (tenant A/B, thiếu `learning_content.read` → 403) — file: `modules/learning/tests/integration/question/advanced/authz_isolation_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US5] API F023-AC-20 vượt giới hạn (đáp án 256 ký tự, polygon 65 đỉnh, 11 biến, ràng buộc 256 ký tự) → 422 — file: `modules/learning/tests/integration/question/advanced/limits_test.go`

### Implementation cho User Story 5

- [ ] T012 [US5] Kiểm giới hạn ở validator domain (song song với schema) — file: `modules/learning/internal/domain/question/types/limits.go`

## Phase 5: User Story 2 — Câu dựa trên hình (Priority: P2)

### Tests cho User Story 2

- [ ] T013 [P] [US2] Unit F023-AC-05 validator hotspot/label — file: `modules/learning/internal/domain/question/types/image_based_validator_test.go` — phải FAIL trước khi code
- [ ] T014 [P] [US2] Unit F023-AC-07 điểm trong tròn/chữ nhật/đa giác (biên, lõm), chấm hotspot/label; F023-AC-08 response lỗi → `INVALID_RESPONSE` — file: `modules/learning/internal/domain/question/geometry/contains_test.go`
- [ ] T015 [P] [US2] Contract F023-AC-06 DTO render không có `regions`/`correct_region_ids`/`placements` — file: `modules/learning/tests/contract/question/render_image_based_test.go`

### Implementation cho User Story 2

- [ ] T016 [P] [US2] `geometry` (circle, rect, polygon even-odd, kiểm tự cắt) — file: `modules/learning/internal/domain/question/geometry/polygon.go`
- [ ] T017 [US2] `ImageBasedHandler` + đăng ký — file: `modules/learning/internal/domain/question/types/image_based.go`

## Phase 6: User Story 3 — Câu tương tác (Priority: P2)

### Tests cho User Story 3

- [ ] T018 [P] [US3] Unit F023-AC-09 widget/validator chưa đăng ký, config sai schema — file: `modules/learning/internal/domain/question/widget/registry_test.go` — phải FAIL trước khi code
- [ ] T019 [P] [US3] Unit + fuzz F023-AC-10 không thực thi mã người soạn (validator chỉ từ registry), F023-AC-11 replay hợp lệ/giả/vượt `max_moves`, F023-AC-12 log > 500 → `INVALID_RESPONSE` — file: `modules/learning/internal/domain/question/widget/balance_scale_v1_test.go`
- [ ] T020 [P] [US3] API F023-AC-13 `listQuestionWidgets` trả widget/version/schema/validator — file: `modules/learning/tests/integration/question/advanced/widgets_api_test.go`

### Implementation cho User Story 3

- [ ] T021 [P] [US3] `widget.Registry` + `balance_scale@1.0` (Replay, `balance_scale.equal`) — file: `modules/learning/internal/domain/question/widget/balance_scale_v1.go`
- [ ] T022 [US3] `InteractiveHandler` (cờ `integrity_mismatch` trong `GradeResult`) + đăng ký — file: `modules/learning/internal/domain/question/types/interactive.go`
- [ ] T023 [US3] Query + handler `listQuestionWidgets` — file: `modules/learning/internal/presentation/http/public/cms_question_widgets.go`

## Phase 7: User Story 4 — Câu tham số hóa (Priority: P2)

### Tests cho User Story 4

- [ ] T024 [P] [US4] Unit F023-AC-14 validator `params` (tên, kiểu, miền, placeholder, khả thi), F023-AC-15 evaluator từ chối định danh/hàm lạ — file: `modules/learning/internal/domain/question/params/spec_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Fuzz test evaluator (không panic, không vượt độ sâu/số mũ) cho F023-AC-15 — file: `modules/learning/internal/domain/question/expr/eval_fuzz_test.go`
- [ ] T026 [P] [US4] Property F023-AC-16 (1.000 seed thỏa ràng buộc, tái lập, đề đã thay số theo `content_language`), F023-AC-17 chấm theo `presentation.params` — file: `modules/learning/internal/domain/question/params/sampler_test.go`
- [ ] T027 [P] [US4] Contract F023-AC-18 DTO không chứa `params`/ràng buộc/biểu thức — file: `modules/learning/tests/contract/question/render_params_test.go`

### Implementation cho User Story 4

- [ ] T028 [P] [US4] `expr` lexer/parser Pratt/evaluator decimal — file: `modules/learning/internal/domain/question/expr/eval.go`
- [ ] T029 [US4] `params.Sample` (rejection sampling, giới hạn số lần thử), `params.Expand` (thay placeholder) — file: `modules/learning/internal/domain/question/params/sampler.go`
- [ ] T030 [US4] Nối `params` vào validator/scorer/renderer `single_choice`, `numeric` và `Delivery.Render/Grade` — file: `modules/learning/internal/domain/question/types/param_hooks.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T031 [P] Benchmark chấm `interactive` và sinh tham số — file: `modules/learning/internal/domain/question/widget/replay_bench_test.go`
- [ ] T032 [P] Metric `eduworld_qb_interactive_integrity_mismatch_total`, `eduworld_qb_param_sampling_failures_total` — file: `modules/learning/internal/infrastructure/observability/metrics.go`
- [ ] T033 [P] Cập nhật QUESTION_BANK §3.5 (chặn fuzzy theo môn), §3.10 (giới hạn mảng), §5 (toán tử evaluator), CHANGELOG — file: `docs/02-learning/QUESTION_BANK.md`, `CHANGELOG.md`
- [ ] T034 Đăng ký tham số mới số lần thử sinh tham số (`param_sampling_max_tries`, phạm vi `code`) ở QUESTION_BANK §5 và CONFIG_DEFAULTS §13, đọc từ config — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/02-learning/QUESTION_BANK.md`
- [ ] T035 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/023-question-bank-advanced/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US5 → US2, US3, US4 (song song, khác package) → Polish.
- US4 dùng lại scorer `numeric`/`single_choice` của F022.

## Definition of Done

- [ ] Mọi tiêu chí F023-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
