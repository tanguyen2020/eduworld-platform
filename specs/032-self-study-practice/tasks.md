# Tasks: F032 — Chế độ luyện tập, hỗ trợ tự học (gợi ý, chẩn đoán, `rapid_guess`), gợi ý ôn tập tự động

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)


## Phase 1: Setup

- [ ] T001 Tạo package rỗng và đăng ký route/consumer/job trong composition root — file: `modules/learning/cmd/api/main.go`, `modules/learning/cmd/worker/main.go`
- [ ] T002 [P] Thêm khóa cấu hình đọc từ platform settings: `attempts_per_question`, `rapid_guess_min_response_ms`, `w_hint`, `guess_coefficient`, `retry_wrong_schedule`, `interleave_mix`, `diagnostic_prior` — file: `modules/learning/internal/application/port/learning_config.go`, `modules/learning/internal/infrastructure/persistence/learning_config_reader.go`
- [ ] T003 [P] Thêm khóa bản dịch lý do gợi ý (vi/en) theo `reason_code` — file: `modules/learning/internal/infrastructure/i18n/recommendation_reasons.{vi,en}.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: `RecommendationList` (recommendation_id, rule_code, reason_code, reason_text, priority, enum mở rộng), `AdHocAttemptStart.source` (`grade_level`, `topic_ids[]`), thêm `getMyMastery`, `ProblemCode.ITEM_POSITION_INVALID` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [ ] T005 [P] Payload schema event mới `learning.recommendation_generated.v1`; trường `recommendation_id?` của `activity.started.v1` do F030 T052 sở hữu (kiểm đã có) — file: `specs/events/payloads/learning/learning.recommendation_generated.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T006 ent schema: bảng mới `learning.wrong_question_entries`, `learning.rm_student_recommendations`; cột `attempt_items.hints_revealed`, `attempt_items.presented_at`, `attempts.recommendation_id`, `objective_masteries.prior_value/prior_source/prior_diagnostic_attempt_id` (mọi chuỗi `MaxLen`) — file: `modules/learning/ent/schema/wrong_question_entry.go`, `modules/learning/ent/schema/student_recommendation.go`, `modules/learning/ent/schema/attempt.go`, `modules/learning/ent/schema/attempt_item.go`, `modules/learning/ent/schema/objective_mastery.go` — kiểm: `make migrate-diff MOD=learning name=f032_self_study`, `make migrate-lint MOD=learning`
- [ ] T007 RLS + partial unique index cho bảng mới trong migration — file: `modules/learning/migrations/<ts>_f032_self_study.sql`
- [ ] T008 Seed migration thêm `model_version` cấu hình mastery mới (`w_hint`, `guess_coefficient`, loại `rapid_guess`) — file: `modules/learning/migrations/<ts>_f032_mastery_config.sql`
- [ ] T009 [P] Cập nhật DATABASE_DESIGN (§2.6, §2.7) cho bảng/cột mới — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Tự luyện có phản hồi ngay (Priority: P1) 🎯 MVP

**Goal**: Học sinh tự luyện theo chủ đề/LO, được chấm và giải thích ngay.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Contract test F032-AC-01: payload `startAdHocAttempt` không chứa `answer_key`, `scoring_rule`, `explanation`, `feedback`, `hints` — file: `modules/learning/tests/contract/practice/adhoc_render_payload_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Integration test F032-AC-02, F032-AC-03: phản hồi `immediate` có `explanation`/`feedback`; thử lại `is_scored=false`; quá lượt `ANSWER_LIMIT_REACHED` — file: `modules/learning/tests/integration/practice/immediate_feedback_test.go`
- [ ] T012 [P] [US1] Unit test F032-AC-04, F032-AC-05, F032-AC-06: không `deadline_at`; độ khó theo level; seed tái lập; nới lỏng rồi `QUESTION_POOL_INSUFFICIENT` — file: `modules/learning/tests/unit/selection/adaptive_difficulty_test.go`
- [ ] T013 [P] [US1] API test F032-AC-07, F032-AC-08: `422 VALIDATION_FAILED` (maxLength, `question_count`), bỏ qua `score`; idempotent start và resume trong `attempt_resume_window` — file: `modules/learning/tests/integration/practice/start_adhoc_validation_test.go`
- [ ] T014 [P] [US1] Test tenant isolation + authorization F032-AC-09 cho `startAdHocAttempt`, `revealNextHint`, `listMyWrongQuestions`, `getMyRecommendations`, `getMyProgress`, `getMyMastery`: staff → 403, header tenant khác → `TENANT_MISMATCH`, attempt người khác → 404 — file: `modules/learning/tests/integration/practice/authz_tenant_test.go`

### Implementation cho User Story 1

- [ ] T015 [P] [US1] Domain `selection.Policy` (adaptive difficulty, avoid_recent, nới lỏng, PRNG seed) — file: `modules/learning/internal/domain/selection/adaptive_difficulty.go`
- [ ] T016 [US1] Port + SQL đọc pool câu published khả dụng cho tenant (riêng + platform được gán), cache `ew:learning:qb:pool:<tenant_id>:<filter_hash>` — file: `modules/learning/internal/application/port/question_pool.go`, `modules/learning/internal/infrastructure/persistence/question_pool_reader.go`, `modules/learning/internal/infrastructure/cache/question_pool_cache.go`
- [ ] T017 [US1] Command `StartAdHocAttempt` (mode practice, source selection/question_set; ghim version; `presented_at`) — file: `modules/learning/internal/application/command/practice/start_adhoc_attempt.go`
- [ ] T018 [US1] Mở rộng phản hồi theo `FeedbackTiming = immediate` và số lượt thử `attempts_per_question` trong chấm câu của F030 — file: `modules/learning/internal/domain/attempt/feedback_policy.go`
- [ ] T019 [US1] Handler `startAdHocAttempt` + middleware học sinh self-only — file: `modules/learning/internal/presentation/http/public/practice_handler.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Gợi ý nhiều cấp trong attempt (Priority: P1)

**Goal**: Mở gợi ý tuần tự, ghi `hints_used`.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T020 [P] [US2] Integration test F032-AC-10, F032-AC-11, F032-AC-12, F032-AC-13: thứ tự cấp, `HINTS_NOT_ALLOWED` (exam/diagnostic/competition/không immediate), `HINTS_EXHAUSTED`, `ATTEMPT_NOT_IN_PROGRESS`, attempt người khác 404 — file: `modules/learning/tests/integration/practice/reveal_hint_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Test F032-AC-14, F032-AC-15: `hints_used` = số cấp trước lần trả lời có tính điểm; gợi ý sau đó không đổi evidence; replay cùng key không tăng cấp — file: `modules/learning/tests/integration/practice/hints_used_test.go`

### Implementation cho User Story 2

- [ ] T022 [P] [US2] Domain `attempt.HintPolicy` (mode/FeedbackTiming được phép, cấp kế tiếp, hết cấp) — file: `modules/learning/internal/domain/attempt/hint.go`
- [ ] T023 [US2] Command `RevealNextHint` (đọc `hints[]` từ version ghim, tăng `hints_revealed` có điều kiện version) — file: `modules/learning/internal/application/command/practice/reveal_next_hint.go`
- [ ] T024 [US2] Ghi `hints_used` khi lưu `attempt_answers` và vào `question.answered` — file: `modules/learning/internal/application/command/attempt/submit_response.go`
- [ ] T025 [US2] Handler `revealNextHint` — file: `modules/learning/internal/presentation/http/public/practice_handler.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Lọc đoán bừa và trọng số evidence (Priority: P1)

**Goal**: `rapid_guess` ở server, mastery loại đoán bừa, áp `w_hint`, `w_guess`.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T026 [P] [US3] Unit test F032-AC-16, F032-AC-19: `response_ms = min(client, server − pause)`; ngưỡng theo loại câu, ×hệ số `g1_3`, ưu tiên `min_response_ms`; minigame/competition không dùng ngưỡng mặc định — file: `modules/learning/tests/unit/attempt/rapid_guess_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US3] Golden test F032-AC-18 theo LEARNING_ANALYTICS §3.3 (ví dụ bổ sung `true_false` + 1 gợi ý; 900 ms không đổi `M`) — file: `modules/learning/tests/unit/mastery/evidence_weight_golden_test.go`
- [ ] T028 [P] [US3] Integration test F032-AC-17, F032-AC-20: điểm attempt giữ nguyên; response học sinh không có `rapid_guess` — file: `modules/learning/tests/integration/practice/rapid_guess_score_test.go`

### Implementation cho User Story 3

- [ ] T029 [P] [US3] Domain `attempt.RapidGuessDetector` — file: `modules/learning/internal/domain/attempt/rapid_guess.go`
- [ ] T030 [P] [US3] Domain `mastery.EvidenceWeight` (`w_hint`, `w_guess`, loại `rapid_guess`) cho `model_version` mới — file: `modules/learning/internal/domain/mastery/evidence_weight.go`
- [ ] T031 [US3] Gắn detector vào lưu câu trả lời; ghi `response_ms`, `rapid_guess` vào `attempt_answers` và `question.answered` — file: `modules/learning/internal/application/command/attempt/submit_response.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Ôn câu sai và luyện trộn chủ đề (Priority: P2)

**Goal**: Danh sách ôn theo lịch, câu anh em, interleave.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T032 [P] [US4] Integration test F032-AC-21: sai lượt đầu vào danh sách; đúng 2 lần ở 2 ngày khác nhau → `cleared`; consumer idempotent — file: `modules/learning/tests/integration/retrywrong/wrong_entry_lifecycle_test.go` — phải FAIL trước khi code
- [ ] T033 [P] [US4] Unit + integration test F032-AC-22: câu anh em, nới ±1, cuối cùng câu gốc — file: `modules/learning/tests/unit/selection/sibling_test.go`
- [ ] T034 [P] [US4] API test F032-AC-23: `listMyWrongQuestions` chỉ của chính học sinh, không đáp án — file: `modules/learning/tests/integration/retrywrong/list_wrong_questions_test.go`
- [ ] T035 [P] [US4] Unit test F032-AC-24: chia theo `interleave_mix`, không quá số câu liên tiếp cùng topic; từ chối với exam/diagnostic — file: `modules/learning/tests/unit/selection/interleave_test.go`

### Implementation cho User Story 4

- [ ] T036 [P] [US4] Domain `retrywrong.Entry` (lịch, chuỗi đúng) — file: `modules/learning/internal/domain/retrywrong/entry.go`
- [ ] T037 [P] [US4] Domain `selection` câu anh em và interleave — file: `modules/learning/internal/domain/selection/sibling.go`, `modules/learning/internal/domain/selection/interleave.go`
- [ ] T038 [US4] Command `RecordWrongAnswer`, `RecordRetryResult` + consumer `question.answered`, `attempt.re_evaluated` — file: `modules/learning/internal/application/command/retrywrong/record_wrong_answer.go`, `modules/learning/internal/presentation/consumer/wrong_question_consumer.go`
- [ ] T039 [US4] Mở rộng `StartAdHocAttempt` cho `retry_wrong` và `interleave`; query `ListMyWrongQuestions` + handler — file: `modules/learning/internal/application/command/practice/start_adhoc_attempt.go`, `modules/learning/internal/application/query/practice/list_my_wrong_questions.go`, `modules/learning/internal/presentation/http/public/practice_handler.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Bài chẩn đoán đặt mastery ban đầu (Priority: P2)

**Goal**: Attempt `diagnostic` và đặt `M0`.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T040 [P] [US5] Integration test F032-AC-25: phạm vi LO (trọng tâm + tiên quyết), số câu theo `diagnostic_prior`, `after_submit`, không gợi ý — file: `modules/learning/tests/integration/mastery/diagnostic_start_test.go` — phải FAIL trước khi code
- [ ] T041 [P] [US5] Unit test F032-AC-26, F032-AC-28: công thức `M0`, LO < 2 câu giữ mặc định, lan tiên quyết, chỉ khi `n = 0`, làm lại không ghi đè — file: `modules/learning/tests/unit/mastery/diagnostic_prior_test.go`
- [ ] T042 [P] [US5] Contract test F032-AC-27: event attempt diagnostic có `context.mode = diagnostic` — file: `modules/learning/tests/contract/practice/diagnostic_event_test.go`

### Implementation cho User Story 5

- [ ] T043 [P] [US5] Domain `mastery.DiagnosticPrior` — file: `modules/learning/internal/domain/mastery/diagnostic_prior.go`
- [ ] T044 [US5] `StartAdHocAttempt` nhánh `diagnostic` (chọn LO qua đồ thị tiên quyết của curriculum) — file: `modules/learning/internal/application/command/practice/start_adhoc_attempt.go`
- [ ] T045 [US5] Command `ApplyDiagnosticPrior` + consumer `activity.graded` mode diagnostic — file: `modules/learning/internal/application/command/mastery/apply_diagnostic_prior.go`, `modules/learning/internal/presentation/consumer/diagnostic_prior_consumer.go`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: User Story 6 — Gợi ý ôn tập tự động có giải thích (Priority: P1)

**Goal**: Rule engine R2, R3, R4, R7, R8, R9 và API gợi ý.

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [ ] T046 [P] [US6] Unit test F032-AC-30, F032-AC-33: từng rule R2/R3/R4/R7/R8/R9 theo bảng LEARNING_ANALYTICS §6; đăng ký rule giả lập R1 không sửa engine — file: `modules/learning/tests/unit/recommendation/rules_test.go` — phải FAIL trước khi code
- [ ] T047 [P] [US6] Integration test F032-AC-29, F032-AC-31: ≤ 3 gợi ý, lý do vi/en, chỉ nội dung published khả dụng cho tenant, bỏ qua 3 lần không gợi ý lại — file: `modules/learning/tests/integration/recommendation/get_my_recommendations_test.go`
- [ ] T048 [P] [US6] Integration test F032-AC-32: `recommendation_id` hợp lệ → `started`, có trong `activity.started`; id lạ bị bỏ qua — file: `modules/learning/tests/integration/recommendation/recommendation_started_test.go`

### Implementation cho User Story 6

- [ ] T049 [P] [US6] Domain `recommendation.Engine`, interface `Rule`, rule R2, R3, R4, R7, R8, R9 — file: `modules/learning/internal/domain/recommendation/engine.go`, `modules/learning/internal/domain/recommendation/r2_weak.go` (và các file rule tương ứng)
- [ ] T050 [US6] Command `RecomputeStudentRecommendations` (outbox `learning.recommendation_generated`), `MarkRecommendationStarted` — file: `modules/learning/internal/application/command/recommendation/recompute_student_recommendations.go`, `modules/learning/internal/application/command/recommendation/mark_recommendation_started.go`
- [ ] T051 [US6] Consumer recompute + job `recommendation-nightly` (R4 decay, R7) — file: `modules/learning/internal/presentation/consumer/recommendation_consumer.go`, `modules/learning/internal/presentation/job/recommendation_nightly.go`
- [ ] T052 [US6] Query `GetMyRecommendations` + handler `getMyRecommendations` — file: `modules/learning/internal/application/query/recommendation/get_my_recommendations.go`, `modules/learning/internal/presentation/http/public/recommendation_handler.go`

**Checkpoint**: User Story 6 chạy độc lập.

## Phase 9: User Story 7 — Học sinh xem tiến độ dạng thân thiện (Priority: P3)

**Goal**: `getMyProgress`, `getMyMastery` theo grade band.

**Independent Test**: như spec.md US7.

### Tests cho User Story 7

- [ ] T053 [P] [US7] API test F032-AC-34: lớp 3 không có `value`, lớp 7 có; chỉ dữ liệu của mình; không nhãn TT27/TT22 — file: `modules/learning/tests/integration/progress/my_mastery_test.go` — phải FAIL trước khi code

### Implementation cho User Story 7

- [ ] T054 [US7] Query `GetMyProgress`, `GetMyMastery` + handler — file: `modules/learning/internal/application/query/progress/get_my_mastery.go`, `modules/learning/internal/presentation/http/public/progress_handler.go`

**Checkpoint**: User Story 7 chạy độc lập.

## Phase 10: Polish & Cross-Cutting

- [ ] T055 [P] Metric + span theo plan.md §Quan sát; kiểm log không chứa `response`/gợi ý (R14) — file: `modules/learning/internal/infrastructure/observability/practice_metrics.go`
- [ ] T056 [P] Cập nhật CHANGELOG và trạng thái F032 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T057 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/032-self-study-practice/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T004–T009) → US1 → US2, US3 (song song) → US4, US5 (song song) → US6 (cần US3–US5 cho dữ liệu rule) → US7 → Polish.
- T024 và T031 cùng sửa `submit_response.go`: làm tuần tự.
- R1/R5/R6 được F041/F042/F040 đăng ký sau; T049 chỉ cần interface ổn định.

## Definition of Done

- [ ] Mọi tiêu chí F032-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
