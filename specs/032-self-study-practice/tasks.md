# Tasks: F032 — Chế độ luyện tập, hỗ trợ tự học (gợi ý, chẩn đoán, `rapid_guess`), gợi ý ôn tập tự động

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)


## Phase 1: Setup

- [x] T001 Tạo package rỗng và đăng ký route/consumer/job trong composition root — file: `src/backend/modules/learning/cmd/api/main.go`, `src/backend/modules/learning/cmd/worker/main.go` — ✅ nối trong `src/backend/modules/learning/module.go` (`command.SelfStudy`, `command.Recommendations`, handler, job `recommendation-nightly`); `go test ./cmd/internal/wiring/` ok
- [x] T002 [P] Thêm khóa cấu hình đọc từ platform settings: `attempts_per_question`, `rapid_guess_min_response_ms`, `w_hint`, `guess_coefficient`, `retry_wrong_schedule`, `interleave_mix`, `diagnostic_prior` — file: `src/backend/modules/learning/internal/application/port/learning_config.go`, `src/backend/modules/learning/internal/infrastructure/persistence/learning_config_reader.go` — ✅ [Confirmed] `command.PracticeConfig`/`DefaultPracticeConfig` (giá trị CONFIG_DEFAULTS) thay cho đọc platform settings — Clarifications 2026-11-20
- [x] T003 [P] Thêm khóa bản dịch lý do gợi ý (vi/en) theo `reason_code` — file: `src/backend/modules/learning/internal/infrastructure/i18n/recommendation_reasons.{vi,en}.json` — ✅ `internal/infrastructure/i18n/recommendation_reasons.{vi,en}.json` + `reasons.go`; TestReasons

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: `RecommendationList` (recommendation_id, rule_code, reason_code, reason_text, priority, enum mở rộng), `AdHocAttemptStart.source` (`grade_level`, `topic_ids[]`), thêm `getMyMastery`, `ProblemCode.ITEM_POSITION_INVALID` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning` — ✅ merged `eduworld-api.v1.yaml` → `split-public.mjs`; `getMyMastery`, `MyMasteryList`, `RecommendationList`, `AdHocAttemptStart.source.grade_level/topic_ids`, `icon_code`; `ITEM_POSITION_INVALID` không thêm (RB-206, Clarifications)
- [x] T005 [P] Payload schema event mới `learning.recommendation_generated.v1`; trường `recommendation_id?` của `activity.started.v1` do F030 T052 sở hữu (kiểm đã có) — file: `specs/events/payloads/learning/learning.recommendation_generated.v1.schema.json` — kiểm: `make specs-lint` — ✅ `learning.recommendation_generated.v1.schema.json` (validate-schemas 0 lỗi; integration kiểm payload); `activity.started.recommendation_id` đã có (F030)
- [x] T006 ent schema: bảng mới `learning.wrong_question_entries`, `learning.rm_student_recommendations`; cột `attempt_items.hints_revealed`, `attempt_items.presented_at`, `attempts.recommendation_id`, `objective_masteries.prior_value/prior_source/prior_diagnostic_attempt_id` (mọi chuỗi `MaxLen`) — file: `src/backend/modules/learning/ent/schema/wrong_question_entry.go`, `src/backend/modules/learning/ent/schema/student_recommendation.go`, `src/backend/modules/learning/ent/schema/attempt.go`, `src/backend/modules/learning/ent/schema/attempt_item.go`, `src/backend/modules/learning/ent/schema/objective_mastery.go` — kiểm: `make migrate-diff MOD=learning name=f032_self_study`, `make migrate-lint MOD=learning` — ✅ bảng viết tay (repository SQL tường minh như F031): migration `20261205000001_f032_self_study.sql`; `presented_at` không thêm [Confirmed]
- [x] T007 RLS + partial unique index cho bảng mới trong migration — file: `src/backend/modules/learning/migrations/<ts>_f032_self_study.sql` — ✅ RLS + partial unique + view `_live` trong cùng migration; `atlas-sum.mjs`
- [x] T008 Seed migration thêm `model_version` cấu hình mastery mới (`w_hint`, `guess_coefficient`, loại `rapid_guess`) — file: `src/backend/modules/learning/migrations/<ts>_f032_mastery_config.sql` — ✅ [Confirmed] không cần dòng mới: `we-1.0` đã có `w_hint`, `guess_coefficient`, bộ lọc loại `rapid_guess` (Clarifications)
- [x] T009 [P] Cập nhật DATABASE_DESIGN (§2.6, §2.7) cho bảng/cột mới — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ `docs/07-backend/DATABASE_DESIGN.md` §2.7

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Tự luyện có phản hồi ngay (Priority: P1) 🎯 MVP

**Goal**: Học sinh tự luyện theo chủ đề/LO, được chấm và giải thích ngay.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Contract test F032-AC-01: payload `startAdHocAttempt` không chứa `answer_key`, `scoring_rule`, `explanation`, `feedback`, `hints` — file: `src/backend/modules/learning/tests/contract/practice/adhoc_render_payload_test.go` — phải FAIL trước khi code — ✅ TestSelfStudyPracticeHintsRapidGuess (`answerKeysIn` trên payload start, không có gợi ý trong render)
- [x] T011 [P] [US1] Integration test F032-AC-02, F032-AC-03: phản hồi `immediate` có `explanation`/`feedback`; thử lại `is_scored=false`; quá lượt `ANSWER_LIMIT_REACHED` — file: `src/backend/modules/learning/tests/integration/practice/immediate_feedback_test.go` — ✅ TestSelfStudyPracticeHintsRapidGuess (explanation/feedback phương án, 2 lần thử lại `is_scored=false`, lần 4 `409 ANSWER_LIMIT_REACHED`)
- [x] T012 [P] [US1] Unit test F032-AC-04, F032-AC-05, F032-AC-06: không `deadline_at`; độ khó theo level; seed tái lập; nới lỏng rồi `QUESTION_POOL_INSUFFICIENT` — file: `src/backend/modules/learning/tests/unit/selection/adaptive_difficulty_test.go` — ✅ `domain/question/selection` TestDifficultyForLevel; TestSelfStudyPracticeHintsRapidGuess (không `deadline_at`); `QUESTION_POOL_INSUFFICIENT` ở TestAttemptAdHoc/TestSelfStudyReviewInterleaveDiagnostic
- [x] T013 [P] [US1] API test F032-AC-07, F032-AC-08: `422 VALIDATION_FAILED` (maxLength, `question_count`), bỏ qua `score`; idempotent start và resume trong `attempt_resume_window` — file: `src/backend/modules/learning/tests/integration/practice/start_adhoc_validation_test.go` — ✅ TestSelfStudyPracticeHintsRapidGuess (`question_count: 99`, enum sai, interleave + diagnostic → 422; replay cùng key; resume khóa khác)
- [x] T014 [P] [US1] Test tenant isolation + authorization F032-AC-09 cho `startAdHocAttempt`, `revealNextHint`, `listMyWrongQuestions`, `getMyRecommendations`, `getMyProgress`, `getMyMastery`: staff → 403, header tenant khác → `TENANT_MISMATCH`, attempt người khác → 404 — file: `src/backend/modules/learning/tests/integration/practice/authz_tenant_test.go` — ✅ TestSelfStudyPracticeHintsRapidGuess (nhân sự 403, `TENANT_MISMATCH`, attempt học sinh khác/tenant B 404; `authztest.Cover` 4 operation mới)

### Implementation cho User Story 1

- [x] T015 [P] [US1] Domain `selection.Policy` (adaptive difficulty, avoid_recent, nới lỏng, PRNG seed) — file: `src/backend/modules/learning/internal/domain/selection/adaptive_difficulty.go` — ✅ `domain/question/selection/practice.go` (`DifficultyForLevel`; nới `avoid_recent`/±1 của `selection.Select`)
- [x] T016 [US1] Port + SQL đọc pool câu published khả dụng cho tenant (riêng + platform được gán), cache `ew:learning:qb:pool:<tenant_id>:<filter_hash>` — file: `src/backend/modules/learning/internal/application/port/question_pool.go`, `src/backend/modules/learning/internal/infrastructure/persistence/question_pool_reader.go`, `src/backend/modules/learning/internal/infrastructure/cache/question_pool_cache.go` — ✅ `query.Delivery.Pool` (pool tenant + gán, cache Valkey `ew:learning:qb:pool:*`), port `QuestionDelivery.Pool`
- [x] T017 [US1] Command `StartAdHocAttempt` (mode practice, source selection/question_set; ghim version; `presented_at`) — file: `src/backend/modules/learning/internal/application/command/practice/start_adhoc_attempt.go` — ✅ `command/attempt_adhoc.go` (`StartAdHoc` practice/retry_wrong/diagnostic, ghim version, recent 7 ngày)
- [x] T018 [US1] Mở rộng phản hồi theo `FeedbackTiming = immediate` và số lượt thử `attempts_per_question` trong chấm câu của F030 — file: `src/backend/modules/learning/internal/domain/attempt/feedback_policy.go` — ✅ đã có ở F030 (`FeedbackTiming`, `attempts_per_question`); sửa khoảng đo khi pause trong `attempt.Respond`
- [x] T019 [US1] Handler `startAdHocAttempt` + middleware học sinh self-only — file: `src/backend/modules/learning/internal/presentation/http/public/practice_handler.go` — ✅ handler `startAdHocAttempt` (F030) dùng `command.StartAdHocInput` mới

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Gợi ý nhiều cấp trong attempt (Priority: P1)

**Goal**: Mở gợi ý tuần tự, ghi `hints_used`.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T020 [P] [US2] Integration test F032-AC-10, F032-AC-11, F032-AC-12, F032-AC-13: thứ tự cấp, `HINTS_NOT_ALLOWED` (exam/diagnostic/competition/không immediate), `HINTS_EXHAUSTED`, `ATTEMPT_NOT_IN_PROGRESS`, attempt người khác 404 — file: `src/backend/modules/learning/tests/integration/practice/reveal_hint_test.go` — phải FAIL trước khi code — ✅ TestSelfStudyPracticeHintsRapidGuess + TestSelfStudyReviewInterleaveDiagnostic (`HINTS_NOT_ALLOWED` trong diagnostic); domain TestRevealHint
- [x] T021 [P] [US2] Test F032-AC-14, F032-AC-15: `hints_used` = số cấp trước lần trả lời có tính điểm; gợi ý sau đó không đổi evidence; replay cùng key không tăng cấp — file: `src/backend/modules/learning/tests/integration/practice/hints_used_test.go` — ✅ TestSelfStudyPracticeHintsRapidGuess (`hints_used = 2` trong `attempt_answers` và `question.answered`; gợi ý sau lần có tính điểm không đổi; replay cùng key cùng cấp, `hints_revealed = 2`)

### Implementation cho User Story 2

- [x] T022 [P] [US2] Domain `attempt.HintPolicy` (mode/FeedbackTiming được phép, cấp kế tiếp, hết cấp) — file: `src/backend/modules/learning/internal/domain/attempt/hint.go` — ✅ `domain/attempt/hint.go` (`HintsAllowed`, `RevealHint`)
- [x] T023 [US2] Command `RevealNextHint` (đọc `hints[]` từ version ghim, tăng `hints_revealed` có điều kiện version) — file: `src/backend/modules/learning/internal/application/command/practice/reveal_next_hint.go` — ✅ `command/self_study.go` `Attempts.RevealHint` (gợi ý của version ghim, `UpdateItem` có điều kiện version)
- [x] T024 [US2] Ghi `hints_used` khi lưu `attempt_answers` và vào `question.answered` — file: `src/backend/modules/learning/internal/application/command/attempt/submit_response.go` — ✅ `command/attempt_commands.go` (`HintsUsed` vào `attempt_answers`, learning event, response)
- [x] T025 [US2] Handler `revealNextHint` — file: `src/backend/modules/learning/internal/presentation/http/public/practice_handler.go` — ✅ handler `revealNextHint` (`presentation/http/self_study.go`)

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Lọc đoán bừa và trọng số evidence (Priority: P1)

**Goal**: `rapid_guess` ở server, mastery loại đoán bừa, áp `w_hint`, `w_guess`.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T026 [P] [US3] Unit test F032-AC-16, F032-AC-19: `response_ms = min(client, server − pause)`; ngưỡng theo loại câu, ×hệ số `g1_3`, ưu tiên `min_response_ms`; minigame/competition không dùng ngưỡng mặc định — file: `src/backend/modules/learning/tests/unit/attempt/rapid_guess_test.go` — phải FAIL trước khi code — ✅ `domain/attempt` TestRapidGuess, TestAnswerTimingAndHints
- [x] T027 [P] [US3] Golden test F032-AC-18 theo LEARNING_ANALYTICS §3.3 (ví dụ bổ sung `true_false` + 1 gợi ý; 900 ms không đổi `M`) — file: `src/backend/modules/learning/tests/unit/mastery/evidence_weight_golden_test.go` — ✅ golden `we-1.0` đã có (`domain/mastery` TestGoldenAdditional, F031) + TestDiagnosticPriors; integration: evidence 2/3 khi có `rapid_guess`
- [x] T028 [P] [US3] Integration test F032-AC-17, F032-AC-20: điểm attempt giữ nguyên; response học sinh không có `rapid_guess` — file: `src/backend/modules/learning/tests/integration/practice/rapid_guess_score_test.go` — ✅ TestSelfStudyPracticeHintsRapidGuess (điểm 2 gồm câu đoán bừa và câu dùng gợi ý; response không có `rapid`)

### Implementation cho User Story 3

- [x] T029 [P] [US3] Domain `attempt.RapidGuessDetector` — file: `src/backend/modules/learning/internal/domain/attempt/rapid_guess.go` — ✅ `domain/attempt/rapid_guess.go`
- [x] T030 [P] [US3] Domain `mastery.EvidenceWeight` (`w_hint`, `w_guess`, loại `rapid_guess`) cho `model_version` mới — file: `src/backend/modules/learning/internal/domain/mastery/evidence_weight.go` — ✅ đã có trong `mastery.WeightedEvidence.Weight` (F031); thêm `PriorValue`/`propagated`
- [x] T031 [US3] Gắn detector vào lưu câu trả lời; ghi `response_ms`, `rapid_guess` vào `attempt_answers` và `question.answered` — file: `src/backend/modules/learning/internal/application/command/attempt/submit_response.go` — ✅ `Attempts.Respond` + `SelfStudy.rapidGuess`; metric `eduworld_learning_rapid_guess_total`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Ôn câu sai và luyện trộn chủ đề (Priority: P2)

**Goal**: Danh sách ôn theo lịch, câu anh em, interleave.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T032 [P] [US4] Integration test F032-AC-21: sai lượt đầu vào danh sách; đúng 2 lần ở 2 ngày khác nhau → `cleared`; consumer idempotent — file: `src/backend/modules/learning/tests/integration/retrywrong/wrong_entry_lifecycle_test.go` — phải FAIL trước khi code — ✅ TestSelfStudyReviewInterleaveDiagnostic (sai lượt đầu → entry; đúng cùng ngày tính một lần; ngày khác → `cleared`); domain TestEntryLifecycle
- [x] T033 [P] [US4] Unit + integration test F032-AC-22: câu anh em, nới ±1, cuối cùng câu gốc — file: `src/backend/modules/learning/tests/unit/selection/sibling_test.go` — ✅ `domain/question/selection` TestSiblings; integration: `retry_wrong` nhận câu khác `question_id`
- [x] T034 [P] [US4] API test F032-AC-23: `listMyWrongQuestions` chỉ của chính học sinh, không đáp án — file: `src/backend/modules/learning/tests/integration/retrywrong/list_wrong_questions_test.go` — ✅ TestSelfStudyPracticeHintsRapidGuess (`listMyWrongQuestions` chỉ của mình, không đáp án)
- [x] T035 [P] [US4] Unit test F032-AC-24: chia theo `interleave_mix`, không quá số câu liên tiếp cùng topic; từ chối với exam/diagnostic — file: `src/backend/modules/learning/tests/unit/selection/interleave_test.go` — ✅ `domain/question/selection` TestInterleave; integration: 4 câu 2 topic, không 3 câu liên tiếp; 1 topic đã học → 422

### Implementation cho User Story 4

- [x] T036 [P] [US4] Domain `retrywrong.Entry` (lịch, chuỗi đúng) — file: `src/backend/modules/learning/internal/domain/retrywrong/entry.go` — ✅ `domain/retrywrong/entry.go`
- [x] T037 [P] [US4] Domain `selection` câu anh em và interleave — file: `src/backend/modules/learning/internal/domain/selection/sibling.go`, `src/backend/modules/learning/internal/domain/selection/interleave.go` — ✅ `domain/question/selection/practice.go` (`Siblings`, `Mix.Split`, `Interleave`, `Diagnostic`)
- [x] T038 [US4] Command `RecordWrongAnswer`, `RecordRetryResult` + consumer `question.answered`, `attempt.re_evaluated` — file: `src/backend/modules/learning/internal/application/command/retrywrong/record_wrong_answer.go`, `src/backend/modules/learning/internal/presentation/consumer/wrong_question_consumer.go` — ✅ [Confirmed] ghi trong transaction trả lời (`SelfStudy.recordAnswer`); re-evaluate qua `Recommendations.clearReEvaluated` trong consumer `learning.mastery_on_graded`
- [x] T039 [US4] Mở rộng `StartAdHocAttempt` cho `retry_wrong` và `interleave`; query `ListMyWrongQuestions` + handler — file: `src/backend/modules/learning/internal/application/command/practice/start_adhoc_attempt.go`, `src/backend/modules/learning/internal/application/query/practice/list_my_wrong_questions.go`, `src/backend/modules/learning/internal/presentation/http/public/practice_handler.go` — ✅ `StartAdHoc` (`retryPlan`, `interleavePlan`, `bindReviews`), `query.SelfStudy.WrongQuestions`, handler `listMyWrongQuestions`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Bài chẩn đoán đặt mastery ban đầu (Priority: P2)

**Goal**: Attempt `diagnostic` và đặt `M0`.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T040 [P] [US5] Integration test F032-AC-25: phạm vi LO (trọng tâm + tiên quyết), số câu theo `diagnostic_prior`, `after_submit`, không gợi ý — file: `src/backend/modules/learning/tests/integration/mastery/diagnostic_start_test.go` — phải FAIL trước khi code — ✅ TestSelfStudyReviewInterleaveDiagnostic (LO trọng tâm + tiên quyết, 5 câu, `after_submit`, gợi ý 403)
- [x] T041 [P] [US5] Unit test F032-AC-26, F032-AC-28: công thức `M0`, LO < 2 câu giữ mặc định, lan tiên quyết, chỉ khi `n = 0`, làm lại không ghi đè — file: `src/backend/modules/learning/tests/unit/mastery/diagnostic_prior_test.go` — ✅ `domain/mastery` TestDiagnosticPriors; integration AC-26/AC-28 (M0 0,75, lan `propagated`, LO có evidence giữ nguyên)
- [x] T042 [P] [US5] Contract test F032-AC-27: event attempt diagnostic có `context.mode = diagnostic` — file: `src/backend/modules/learning/tests/contract/practice/diagnostic_event_test.go` — ✅ TestSelfStudyReviewInterleaveDiagnostic (`activity.graded` mode diagnostic, learning event context diagnostic)

### Implementation cho User Story 5

- [x] T043 [P] [US5] Domain `mastery.DiagnosticPrior` — file: `src/backend/modules/learning/internal/domain/mastery/diagnostic_prior.go` — ✅ `domain/mastery/diagnostic_prior.go`
- [x] T044 [US5] `StartAdHocAttempt` nhánh `diagnostic` (chọn LO qua đồ thị tiên quyết của curriculum) — file: `src/backend/modules/learning/internal/application/command/practice/start_adhoc_attempt.go` — ✅ `diagnosticPlan` (LO + tiên quyết từ curriculum published của tenant)
- [x] T045 [US5] Command `ApplyDiagnosticPrior` + consumer `activity.graded` mode diagnostic — file: `src/backend/modules/learning/internal/application/command/mastery/apply_diagnostic_prior.go`, `src/backend/modules/learning/internal/presentation/consumer/diagnostic_prior_consumer.go` — ✅ [Confirmed] nhánh diagnostic trong `Mastery.ApplyGraded` (`DiagnosticPriors.apply`), cùng consumer `learning.mastery_on_graded`

**Checkpoint**: User Story 5 chạy độc lập.

## Phase 8: User Story 6 — Gợi ý ôn tập tự động có giải thích (Priority: P1)

**Goal**: Rule engine R2, R3, R4, R7, R8, R9 và API gợi ý.

**Independent Test**: như spec.md US6.

### Tests cho User Story 6

- [x] T046 [P] [US6] Unit test F032-AC-30, F032-AC-33: từng rule R2/R3/R4/R7/R8/R9 theo bảng LEARNING_ANALYTICS §6; đăng ký rule giả lập R1 không sửa engine — file: `src/backend/modules/learning/tests/unit/recommendation/rules_test.go` — phải FAIL trước khi code — ✅ `domain/recommendation` TestRules, TestEngine (rule giả R1 đăng ký không sửa engine)
- [x] T047 [P] [US6] Integration test F032-AC-29, F032-AC-31: ≤ 3 gợi ý, lý do vi/en, chỉ nội dung published khả dụng cho tenant, bỏ qua 3 lần không gợi ý lại — file: `src/backend/modules/learning/tests/integration/recommendation/get_my_recommendations_test.go` — ✅ TestSelfStudyRecommendations (≤ 3, lý do vi/en, `Content-Language`, học sinh khác không thấy, bỏ qua 3 lần không gợi ý lại)
- [x] T048 [P] [US6] Integration test F032-AC-32: `recommendation_id` hợp lệ → `started`, có trong `activity.started`; id lạ bị bỏ qua — file: `src/backend/modules/learning/tests/integration/recommendation/recommendation_started_test.go` — ✅ TestSelfStudyRecommendations (`started` → `completed`, `attempts.recommendation_id`, `activity.started.recommendation_id`; id lạ bỏ qua)

### Implementation cho User Story 6

- [x] T049 [P] [US6] Domain `recommendation.Engine`, interface `Rule`, rule R2, R3, R4, R7, R8, R9 — file: `src/backend/modules/learning/internal/domain/recommendation/engine.go`, `src/backend/modules/learning/internal/domain/recommendation/r2_weak.go` (và các file rule tương ứng) — ✅ `domain/recommendation/engine.go`, `rules.go` (R2, R3, R4, R7, R8, R9)
- [x] T050 [US6] Command `RecomputeStudentRecommendations` (outbox `learning.recommendation_generated`), `MarkRecommendationStarted` — file: `src/backend/modules/learning/internal/application/command/recommendation/recompute_student_recommendations.go`, `src/backend/modules/learning/internal/application/command/recommendation/mark_recommendation_started.go` — ✅ `command/recommendations.go` (`Recompute`, `OnAttempt`, outbox `learning.recommendation_generated`); `MarkRecommendationStarted` = `Attempts.startRecommendation`
- [x] T051 [US6] Consumer recompute + job `recommendation-nightly` (R4 decay, R7) — file: `src/backend/modules/learning/internal/presentation/consumer/recommendation_consumer.go`, `src/backend/modules/learning/internal/presentation/job/recommendation_nightly.go` — ✅ [Confirmed] tính lại trong consumer `learning.mastery_on_graded` (`Mastery.Recommend`) + job `recommendation-nightly` (`presentation/job/recommendation_nightly.go`)
- [x] T052 [US6] Query `GetMyRecommendations` + handler `getMyRecommendations` — file: `src/backend/modules/learning/internal/application/query/recommendation/get_my_recommendations.go`, `src/backend/modules/learning/internal/presentation/http/public/recommendation_handler.go` — ✅ `query.SelfStudy.Recommendations` + handler `getMyRecommendations`

**Checkpoint**: User Story 6 chạy độc lập.

## Phase 9: User Story 7 — Học sinh xem tiến độ dạng thân thiện (Priority: P3)

**Goal**: `getMyProgress`, `getMyMastery` theo grade band.

**Independent Test**: như spec.md US7.

### Tests cho User Story 7

- [x] T053 [P] [US7] API test F032-AC-34: lớp 3 không có `value`, lớp 7 có; chỉ dữ liệu của mình; không nhãn TT27/TT22 — file: `src/backend/modules/learning/tests/integration/progress/my_mastery_test.go` — phải FAIL trước khi code — ✅ TestSelfStudyPracticeHintsRapidGuess (lớp 3 `value = null`, `g1_3.<level>`; lớp 7 `value = 0.64`, `g6_7.proficient`; không TT27/TT22; `getMyProgress.icon_code`)

### Implementation cho User Story 7

- [x] T054 [US7] Query `GetMyProgress`, `GetMyMastery` + handler — file: `src/backend/modules/learning/internal/application/query/progress/get_my_mastery.go`, `src/backend/modules/learning/internal/presentation/http/public/progress_handler.go` — ✅ `query.Progress.MyMastery` + handler `getMyMastery`; `icon_code` trong `getMyProgress`

**Checkpoint**: User Story 7 chạy độc lập.

## Phase 10: Polish & Cross-Cutting

- [x] T055 [P] Metric + span theo plan.md §Quan sát; kiểm log không chứa `response`/gợi ý (R14) — file: `src/backend/modules/learning/internal/infrastructure/observability/practice_metrics.go` — ✅ `infrastructure/observability/practice_metrics.go`; log usecase không chứa response/gợi ý (chỉ mã kết quả)
- [x] T056 [P] Cập nhật CHANGELOG và trạng thái F032 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — ✅ CHANGELOG, CODE_PLAN
- [x] T057 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/032-self-study-practice/quickstart.md` — ✅ quickstart §5

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Đã làm 2026-11-20.

- [x] T058 **OQ chốt 2026-11-20** (OQ F030) `startAdHocAttempt` với question set theo luật: chọn câu bằng `selection.Select`, ghim version từng câu; test đủ câu/thiếu câu — file: `src/backend/modules/learning/internal/application/command/attempt_commands.go` — kiểm: `make test-integration MOD=learning` — cỡ S — ✅ `Attempts.questionSetItems` chọn câu theo từng `selection_rules` (`selection.Select`, seed attempt + chỉ số rule, loại câu đã chọn); thiếu → 422; TestSelfStudyQuestionSetRules
- [x] T059 (F063-AC-24, contract b058746) `AttemptReview.recommendations[]` có `recommendation_id`: gợi ý đang active của học sinh (≤ 3, `target_type` lesson/question_set/quest) có LO giao với LO yếu của attempt [Confirmed] — file: `src/backend/modules/learning/internal/application/query/attempts.go` — kiểm: `make test-integration MOD=learning` — bằng chứng: `TestRenderHintCountAndReviewRecommendations` PASS

## Dependencies & Execution Order

- Setup → Foundational (T004–T009) → US1 → US2, US3 (song song) → US4, US5 (song song) → US6 (cần US3–US5 cho dữ liệu rule) → US7 → Polish.
- T024 và T031 cùng sửa `submit_response.go`: làm tuần tự.
- R1/R5/R6 được F041/F042/F040 đăng ký sau; T049 chỉ cần interface ổn định.

## Definition of Done

- [x] Mọi tiêu chí F032-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
